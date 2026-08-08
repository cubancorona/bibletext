package bibletext

// The AI settings sheet (bring-your-own-key). It stays deliberately calm: choose
// one assistant, see and edit just that assistant's key, test it, save. The key
// area swaps to whichever provider is selected, so there's never a wall of four
// password fields. Reachable any time from the header gear, including after a key
// is already set. Keys live in the on-device key store (ai_keystore.go).

import (
	"context"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func showAISettings(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()
	store := state.keys()

	// The only sheet controls that affect the reading pane are the red-letter
	// toggle and the text-size choice. Capture them now so closing the sheet
	// re-renders ONLY when one actually changed — refreshing unconditionally
	// rebuilds the reading pane (re-pinning the native text overlay) and flickers
	// the screen for an AI-key-only change.
	redLetterAtOpen := redLetterEnabled()
	textSizeAtOpen := readingTextSizeID()
	// Choosing "None" (or leaving it) changes which whole surfaces exist — the
	// Search-tab Find toggle, the native selection menus — so closing the sheet
	// after that change rebuilds the window rather than just re-rendering verses.
	aiOnAtOpen := store.aiEnabled()
	// Whether AI is USABLE — an assistant is selected AND its key is present.
	// aiEnabled alone misses the common case: a provider is already selected
	// (Gemini is the default) but has no key, so Find shows "Find needs your
	// own AI key"; pasting a key, or switching to a provider that already has
	// one, leaves aiEnabled unchanged and used to leave that stale panel up
	// until the reader navigated away and back (field report).
	aiKeyAtOpen := hasAIKey(state)
	// If a full rebuild happens while the sheet is open (theme-variant flip,
	// tablet rotation), the new window was already built from live prefs —
	// done()'s own delta rebuild/refresh would be a duplicate window swap.
	rebuildGenAtOpen := windowRebuildGen

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	providers := aiProviders()
	nameToID := map[string]string{}
	idToName := map[string]string{}
	// "None" leads the assistant picker: choosing it turns every AI feature off
	// (no Study-with-AI selection menu, no Find search, no key fields) while
	// keeping any saved keys, so picking a provider again restores the old setup.
	const noAssistantName = "None"
	names := make([]string, 0, len(providers)+1)
	names = append(names, noAssistantName)
	for _, p := range providers {
		names = append(names, p.Name)
		nameToID[p.Name] = p.ID
		idToName[p.ID] = p.Name
	}

	// keyArea shows only the selected provider's key + status; it rebuilds when the
	// picker changes. Everything auto-saves straight to the on-device store — there
	// is no Save/Cancel — so there's no pending-edits buffer to flush.
	keyArea := container.NewStack()
	// renderGen invalidates in-flight async work (the model-list fetch) when the
	// key area re-renders for another provider — a slow response for Gemini must
	// never repopulate the dropdown now showing Anthropic.
	renderGen := 0
	var renderKey func(id string)
	renderKey = func(id string) {
		info, ok := providerByID(id)
		if !ok {
			return
		}
		renderGen++
		gen := renderGen
		// Assigned below with the model dropdown; the key field calls it so
		// pasting a key populates the model list immediately.
		var fetchModels func()

		heading := canvas.NewText(info.Name+" key", pal.Text)
		heading.TextStyle = fyne.TextStyle{Bold: true}
		heading.TextSize = 18 // match the standard chrome text size

		var link fyne.CanvasObject = layout.NewSpacer()
		if u, err := url.Parse(info.KeyURL); err == nil {
			link = widget.NewHyperlink("Get a key ↗", u)
		}

		entry := widget.NewPasswordEntry()
		entry.SetPlaceHolder("Paste your " + info.Name + " key")
		entry.SetText(store.apiKey(id))

		// status + the Clear button are kept in step with what's in the field and
		// what's saved by refreshStatus (defined below, once the button exists).
		status := canvas.NewText("", pal.TextMuted)
		status.TextSize = 12

		result := widget.NewLabel("")
		result.Wrapping = fyne.TextWrapWord
		result.Hide()
		testBtn := widget.NewButtonWithIcon("Test key", theme.MediaPlayIcon(), func() {
			key := strings.TrimSpace(entry.Text)
			result.Show()
			if key == "" {
				result.SetText("Paste a key first.")
				return
			}
			result.SetText("Testing…")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), aiProbeBudget)
				defer cancel()
				_, err := info.New(store, key).generate(ctx, "Reply with the single word: OK")
				fyne.Do(func() {
					if err != nil {
						result.SetText("✗ " + friendlyAIError(err))
					} else {
						result.SetText("✓ Working")
					}
				})
			}()
		})
		// A normal-weight button with an icon, so it clearly reads as tappable. A
		// low-importance button is borderless — it looks like a plain bold label and
		// hides that it's interactive (and on touch there's no hover state to reveal
		// it). Fyne's principle is that every interaction should be visually hinted, so
		// the icon + button background match the Paste / Clear buttons beside it.

		// API keys are pasted, not typed — a one-tap Paste avoids fighting the
		// on-screen keyboard (which otherwise covers this field on a phone).
		pasteBtn := widget.NewButtonWithIcon("Paste", theme.ContentPasteIcon(), func() {
			if state.window == nil {
				return
			}
			clip := state.window.Clipboard()
			if clip == nil {
				return
			}
			if v := strings.TrimSpace(clip.Content()); v != "" {
				entry.SetText(v) // fires OnChanged, which auto-saves the key
			}
		})

		// Clear empties the field, which (auto-save) removes the stored key. The X
		// icon makes the intent obvious.
		clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
			entry.SetText("") // fires OnChanged → clears the saved key
		})

		saveOK := true
		refreshStatus := func() {
			savedLabel := "✓ Saved on this device."
			if store.keyInSecureStore(info.ID) {
				savedLabel = "✓ Saved securely in the device keychain."
			}
			if strings.TrimSpace(entry.Text) != "" {
				if saveOK {
					status.Text = savedLabel
					status.Color = pal.Accent
				} else {
					status.Text = "Couldn't save this key securely. Please try again."
					status.Color = theme.Color(theme.ColorNameError)
				}
				clearBtn.Enable()
			} else if !saveOK {
				// The Clear tapped but the credential-store delete FAILED: say
				// so — a reader who believes a key is gone when it isn't has a
				// false sense of removal (audit finding).
				status.Text = "Couldn't remove the stored key. Please try again."
				status.Color = theme.Color(theme.ColorNameError)
				clearBtn.Enable()
			} else {
				status.Text = info.KeyHint
				status.Color = pal.TextMuted
				clearBtn.Disable()
			}
			status.Refresh()
		}
		// Auto-save: every edit writes straight to the on-device key store. A new
		// key also (re-)fetches the provider's model list for the dropdown below.
		entry.OnChanged = func(s string) {
			saveOK = store.setAPIKey(id, strings.TrimSpace(s))
			refreshStatus()
			if fetchModels != nil {
				fetchModels()
			}
		}
		refreshStatus()

		// Model picker (a dropdown — nothing to mistype). "Recommended" pins
		// nothing: it uses the model currently in effect — the built-in default,
		// or the self-healed replacement if the provider retired it — and keeps
		// healing itself (ai_model_resolve.go). The other choices are the
		// provider's OWN current model list, fetched live with the user's key
		// whenever this area renders or a key is pasted — so new models appear as
		// soon as the provider publishes them, with no app update.
		effModel := store.resolvedModel(id)
		if effModel == "" {
			effModel = info.Model
		}
		recommended := "Recommended (" + effModel + ")"
		modelCaption := canvas.NewText("Model — Recommended keeps itself current automatically.", pal.TextMuted)
		modelCaption.TextSize = 12

		// Until (or unless) the live list arrives, the choices are Recommended
		// plus any model already pinned, so the control is honest offline too.
		options := []string{recommended}
		selected := recommended
		if ov := store.overrideModel(id); ov != "" {
			options = append(options, ov)
			selected = ov
		}

		// The model control is a button that opens a MODAL picker (title + ✕ +
		// a scrollable list), NOT a raw widget.Select: with ~20 live models the
		// Select's popup fills the entire phone screen, leaving no visible
		// "outside" to tap and no way to back out without picking something
		// (field-reported on iPhone). The modal matches the app's chapter-picker
		// convention and always shows an explicit close.
		var pickerPop *widget.PopUp
		var pickerList *widget.List
		modelBtn := widget.NewButtonWithIcon(selected, theme.MenuDropDownIcon(), nil)
		modelBtn.Alignment = widget.ButtonAlignLeading
		applyChoice := func(sel string) {
			if sel == recommended {
				store.setOverrideModel(id, "")
			} else if sel != "" {
				store.setOverrideModel(id, sel)
			}
			selected = sel
			modelBtn.SetText(sel)
		}
		closePicker := func() {
			if pickerPop != nil {
				pickerPop.Hide()
				pickerPop = nil
			}
			pickerList = nil
		}
		modelBtn.OnTapped = func() {
			const rowHeight = 44 // touch-sized rows, like the mobile book list
			list := widget.NewList(
				func() int { return len(options) },
				func() fyne.CanvasObject {
					l := canvas.NewText("", pal.Text)
					l.TextSize = 14
					return container.NewPadded(l)
				},
				func(i widget.ListItemID, o fyne.CanvasObject) {
					if i < 0 || i >= len(options) {
						return
					}
					l := o.(*fyne.Container).Objects[0].(*canvas.Text)
					l.Text = options[i]
					if options[i] == selected {
						l.Color = pal.Accent
						l.TextStyle = fyne.TextStyle{Bold: true}
					} else {
						l.Color = pal.Text
						l.TextStyle = fyne.TextStyle{}
					}
					l.Refresh()
				})
			for i := 0; i < len(options); i++ {
				list.SetItemHeight(widget.ListItemID(i), rowHeight)
			}
			list.OnSelected = func(i widget.ListItemID) {
				if i < 0 || i >= len(options) {
					return
				}
				applyChoice(options[i])
				closePicker()
			}
			pickerList = list

			title := canvas.NewText("Model", pal.Text)
			title.TextSize = 16
			title.TextStyle = fyne.TextStyle{Bold: true}
			x := widget.NewButtonWithIcon("", theme.CancelIcon(), closePicker)
			x.Importance = widget.LowImportance
			hdr := container.NewBorder(nil, nil, container.NewCenter(title), x)
			card := surface(container.NewBorder(hdr, nil, nil, nil, list), pal.Surface, pal.Border, fyne.Size{})

			pickerPop = widget.NewModalPopUp(card, cnv)
			pickerPop.Show()
			// Size to the content but ALWAYS leave a margin, so it reads as a
			// sheet floating over the settings — never a full-screen takeover.
			cs := cnv.Size()
			w := min(cs.Width-48, 420)
			h := min(cs.Height-140, float32(len(options))*rowHeight+72)
			pickerPop.Resize(fyne.NewSize(w, h))
			for i, o := range options {
				if o == selected {
					list.ScrollTo(widget.ListItemID(i))
					break
				}
			}
		}

		// fetchModels populates the dropdown from the provider's live list. Guards:
		//   • fetchedKey — only refetch when the key CHANGES (set on success, and
		//     reset on failure so a transient blip/rate-limit doesn't latch the
		//     dropdown to [Recommended] with no retry);
		//   • fetchSeq — only the most recently STARTED fetch may apply, so a slow
		//     response for an old key can't overwrite a newer key's list;
		//   • gen — a fetch for a different provider render never applies.
		// A short debounce coalesces the per-keystroke burst when a key is typed
		// (rather than pasted), so we don't fire dozens of doomed partial-key
		// requests that could trip the provider's rate limit.
		fetchedKey, fetchSeq := "", 0
		var debounce *time.Timer
		doFetch := func() {
			key := strings.TrimSpace(providerAPIKey(store, id))
			if key == "" || info.ListModels == nil || key == fetchedKey {
				return
			}
			fetchSeq++
			seq := fetchSeq
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), aiProbeBudget)
				defer cancel()
				models, err := info.ListModels(ctx, key)
				ids := dropdownModelIDs(models, info.ExtraModelExclude, modelFamilyOf(info.Model))
				fyne.Do(func() {
					if gen != renderGen || seq != fetchSeq {
						return // superseded by a newer render or a newer fetch
					}
					if err != nil || len(ids) == 0 {
						fetchedKey = "" // let the next paste/keystroke retry
						return
					}
					fetchedKey = key
					opts := []string{recommended}
					cur := store.overrideModel(id)
					if cur != "" && !containsExact(ids, cur) {
						opts = append(opts, cur) // a pin the provider no longer lists stays visible
					}
					opts = append(opts, ids...)
					options = opts
					if cur != "" {
						selected = cur
					} else {
						selected = recommended
					}
					modelBtn.SetText(selected)
					if pickerList != nil {
						pickerList.Refresh() // live list landed while the picker is open
					}
				})
			}()
		}
		fetchModels = func() {
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(400*time.Millisecond, func() { fyne.Do(doFetch) })
		}
		doFetch() // initial populate immediately (no key change to debounce)

		keyArea.Objects = []fyne.CanvasObject{
			container.NewVBox(
				container.NewBorder(nil, nil, heading, link),
				withCaret(state, entry),
				status,
				// Paste + Clear + Test on one row; the result label gets its OWN
				// full-width row below. It must NOT share the row as a Border
				// center: on a phone the three buttons leave only a sliver of
				// width, and a word-wrapped label in that sliver wraps at a few
				// characters per line and balloons the row into a tall text
				// column (field-reported on iPhone with the long model-gone
				// message). Hidden until a test runs, so the sheet only grows by
				// a line or two when there's something to say.
				container.NewHBox(pasteBtn, clearBtn, testBtn),
				result,
				spacer(8),
				modelCaption,
				modelBtn,
			),
		}
		keyArea.Refresh()
	}

	// applyAssistant re-renders the sheet's key/disclosure area for the current
	// choice and mirrors it into the native selection menus immediately (so the
	// Study-with-AI submenu appears/disappears without waiting for the sheet to
	// close). Assigned below, once the disclosure widget exists; the radio callback
	// only persists the choice and delegates here.
	var applyAssistant func()
	active := widget.NewRadioGroup(names, func(name string) {
		if name == noAssistantName {
			store.setAIEnabled(false) // auto-save; keys are kept
			// Drop any live Find context (an open results pane, an in-flight
			// query) so no stale AI state — IsSearching, the back-to-results
			// label, a late error — survives the switch to "None".
			clearAISearchContext(state)
		} else if id, ok := nameToID[name]; ok {
			store.setAIEnabled(true)
			store.setActiveProvider(id) // auto-save
		}
		if applyAssistant != nil {
			applyAssistant()
		}
	})
	active.Required = true

	// --- Chrome. A compact sheet: a small title + ✕, then the form. There is no
	// Done button — every change auto-saves, so the ✕ or a tap anywhere outside the
	// card dismisses it. done() runs the cleanup either way (re-show the native
	// reading overlay + re-render so a red-letter toggle takes effect).
	var popup *widget.PopUp
	closed := false
	done := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide()
		}
		restore()
		if windowRebuildGen != rebuildGenAtOpen {
			return // a rebuild drained us; it already built from live prefs
		}
		if aiSurfacesChanged(aiOnAtOpen, aiKeyAtOpen, store.aiEnabled(), hasAIKey(state)) {
			rebuildWindow(state)
		} else if redLetterEnabled() != redLetterAtOpen || readingTextSizeID() != textSizeAtOpen {
			state.refreshReadingOnly() // red-letter / text size changed → re-render the verses
		}
	}

	title := canvas.NewText("Settings", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), done)
	closeBtn.Importance = widget.LowImportance
	header := container.NewBorder(nil, nil, container.NewCenter(title), container.NewCenter(closeBtn))

	redLetter := widget.NewCheck("Show the words of Christ in red", nil)
	redLetter.SetChecked(redLetterEnabled())
	redLetter.OnChanged = func(b bool) { setRedLetterEnabled(b) }

	// Scripture text size — the app can't inherit the phone's Larger Text setting
	// (Fyne renders its own canvas), so this is the reader's size control. Radio
	// rows (not a slider): three named steps are easier to hit and to reason about.
	sizeLabels := make([]string, len(textSizeOptions))
	labelToID := map[string]string{}
	currentLabel := ""
	for i, o := range textSizeOptions {
		sizeLabels[i] = o.Label
		labelToID[o.Label] = o.ID
		if o.ID == readingTextSizeID() {
			currentLabel = o.Label
		}
	}
	textSize := widget.NewRadioGroup(sizeLabels, func(sel string) {
		if id, ok := labelToID[sel]; ok && sel != "" {
			setReadingTextSizeID(id)
		}
	})
	textSize.Horizontal = true
	textSize.Required = true
	textSize.SetSelected(currentLabel)
	textSizeRow := container.NewVBox(
		widget.NewLabel("Text size"),
		textSize,
	)

	// In-app disclosure of where AI prompts go, shown right under the key field
	// (Guideline 5.1.2 — be transparent before user content leaves the device). It
	// mirrors the privacy policy and links to it.
	aiNote := widget.NewRichText(&widget.TextSegment{
		Text: "When you use AI study, your Find search or selected passage and study action are sent directly to the provider you choose, authenticated with your key. " +
			"The provider may associate and retain the request under your account terms.",
		Style: widget.RichTextStyle{ColorName: colorNameMuted, SizeName: theme.SizeNameCaptionText},
	})
	aiNote.Wrapping = fyne.TextWrapWord
	aiDisclosure := container.NewVBox(aiNote)
	// Deep-link the policy itself — the site ROOT is the download landing page
	// since 2026-07 (the policy moved to privacy.html when the site gained a
	// download page; keep this in sync with gh-pages).
	if u, err := url.Parse("https://bibletext.co.uk/privacy.html"); err == nil {
		aiDisclosure.Add(container.NewHBox(widget.NewHyperlink("Privacy Policy ↗", u), layout.NewSpacer()))
	}

	var card *fyne.Container // assigned below, before the popup shows
	applyAssistant = func() {
		// Mirror the choice into the native selection menus right away, so the
		// Study-with-AI submenu is gone (or back) on the very next selection.
		syncNativeAIMenu(state)
		if store.aiEnabled() {
			renderKey(store.activeProvider())
			aiDisclosure.Show()
		} else {
			keyArea.Objects = []fyne.CanvasObject{caption(
				"AI features are off — text selection keeps Share and Cross-references, " +
					"and Search keeps keyword search. Choose an assistant to bring them back.")}
			keyArea.Refresh()
			aiDisclosure.Hide()
		}
		// The card's height changes between the one-line hint (None) and the full
		// key+model form — and fyne's PopUp uses its Resize-time innerSize both to
		// paint and as the tap-outside-to-dismiss boundary. Without re-measuring,
		// a mid-sheet flip leaves that boundary stale: taps on the grown card
		// phantom-dismiss the sheet mid-key-entry (grow case), or a dead band
		// below the shrunken card swallows outside-taps (shrink case).
		//
		// Resize twice: a wrapping RichText that was hidden (the disclosure on a
		// None-state open) reports a single-line MinSize until it has been laid
		// out at its real width — which the FIRST Resize's layout pass does — so
		// only the second measurement is wrap-accurate on the grow flip.
		if popup != nil && card != nil {
			popup.Resize(card.MinSize())
			popup.Resize(card.MinSize())
		}
	}
	// Set the radio's visual selection WITHOUT firing its OnChanged (which would
	// call applyAssistant → renderKey → a live model-list fetch), then render once
	// explicitly. SetSelected here would double the initial fetch on every sheet
	// open (the callback's render is discarded by the gen guard, but the HTTP
	// request still fires) — the single most common path, so worth avoiding.
	if store.aiEnabled() {
		active.Selected = idToName[store.activeProvider()]
	} else {
		active.Selected = noAssistantName
	}
	active.Refresh()
	applyAssistant() // single initial render → single model-list fetch

	// Assistant + key first so the key field sits high in the sheet — on a phone
	// the soft keyboard covers the lower half, and this keeps the field above it.
	// Section labels (not separators) divide the groups, keeping the sheet tight.
	form := container.NewVBox(
		sectionLabel("ASSISTANT", pal),
		active,
		keyArea,
		aiDisclosure,
		sectionLabel("READING", pal),
		textSizeRow,
	)
	if redLetterSupported() {
		form.Add(redLetter)
	}

	hint := canvas.NewText("Changes save automatically — tap outside to close.", pal.TextMuted)
	hint.TextSize = 11

	inner := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator()),
		container.NewPadded(hint),
		nil, nil,
		form,
	)
	// Chrome text at the standard 18px (the tighter layout — not a smaller font —
	// does the de-cluttering). compactTheme stays as the one knob if we ever want to
	// nudge just the sheet's text size.
	themed := container.NewThemeOverride(inner, compactTheme{Theme: state.theme, text: 18})

	// A CARD-sized sheet at a fixed width, auto-sizing its height to the content. Two
	// things this buys us: the popup's overlay-background rectangle is only as big as
	// the card (hidden behind the surface fill), so it never shows as a white wall;
	// and there is no scroll view, so a stray scrollbar is impossible.
	ps := aiPanelSize(cnv.Size())
	card = container.New(fixedWidthLayout{width: ps.Width},
		surface(themed, pal.SurfaceAlt, pal.Border, fyne.Size{}))

	// A NON-modal popup: leaves the reading page visible (undimmed) behind it and
	// dismisses on a tap OUTSIDE the card. Resize it to the card's size FIRST — Fyne gates
	// the tap-to-dismiss on PopUp.isInsideContent, which reads innerSize, and without an
	// explicit Resize innerSize stays zero so EVERY tap (even on the card) counts as
	// "outside" and closes the sheet. (Same as the Goto picker's popup.)
	popup = widget.NewPopUp(card, cnv)
	popup.Resize(card.MinSize())
	x := (cnv.Size().Width - ps.Width) / 2
	if x < 0 {
		x = 0
	}
	y := float32(28)
	if pos, _ := cnv.InteractiveArea(); pos.Y > 0 {
		y = pos.Y + 16
	}
	popup.ShowAtPosition(fyne.NewPos(x, y))

	// done() (overlay-restore cleanup) is called directly by the ✕. An outside-tap close
	// goes through Fyne's built-in PopUp.Hide, which does NOT call done() — and a PopUp
	// subclass can't intercept it (PopUp.Show registers the embedded *PopUp, so a Tapped
	// override is never dispatched). So poll until the popup is gone by ANY route, then run
	// done(); its `closed` guard keeps the ✕ path idempotent. (Same approach as Goto.)
	// onOverlayStack: belt-and-braces. rebuildWindow's overlay drain HIDES
	// *widget.PopUp overlays (reading.go), which flips Visible() false — so
	// this membership check is redundant on that path and stays only as
	// defense against any future bare OverlayStack.Remove eviction, which
	// never runs PopUp.Hide and would otherwise leave this watchdog polling
	// for the life of the process.
	onOverlayStack := func() bool {
		for _, o := range cnv.Overlays().List() {
			if o == popup {
				return true
			}
		}
		return false
	}
	var watchDismiss func()
	watchDismiss = func() {
		if popup == nil || !popup.Visible() || !onOverlayStack() {
			// Skip the close-out when ANOTHER overlay owns the canvas by the
			// time this poll fires (the reader reopened a sheet within one
			// tick of a drain): done()'s restore would un-suppress the native
			// reading view OVER the new modal.
			if cnv.Overlays().Top() == nil {
				done()
			}
			return
		}
		time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watchDismiss) })
	}
	time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watchDismiss) })
}

// aiSurfacesChanged reports whether closing the settings sheet must rebuild the
// window, given what was true when it opened and what is true now.
//
// Two independent reasons, and the second is the one that is easy to miss:
//   - the assistant came or went ("None" ↔ a provider), so whole surfaces —
//     the Search-tab Find toggle, the native selection menus — appear or
//     disappear; or
//   - AI became usable or unusable because a KEY arrived or left. A provider is
//     selected by default, so aiEnabled() is already true while Find is showing
//     "Find needs your own AI key". Pasting a key (or switching to a provider
//     that already has one) changes nothing about aiEnabled, so without this
//     half the stale set-up panel survived until the reader navigated away and
//     back — field-reported.
func aiSurfacesChanged(enabledAtOpen, keyAtOpen, enabledNow, keyNow bool) bool {
	return enabledAtOpen != enabledNow || keyAtOpen != keyNow
}

// compactTheme shrinks only the base text size of a subtree (applied via
// container.NewThemeOverride), delegating everything else to the app theme. It
// renders the settings sheet's chrome tighter than the 18px reading text size.
type compactTheme struct {
	fyne.Theme
	text float32
}

func (c compactTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return c.text
	}
	return c.Theme.Size(name)
}
