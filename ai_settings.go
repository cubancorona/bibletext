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

	// The only sheet control that affects the reading pane is the red-letter
	// toggle. Capture it now so closing the sheet re-renders ONLY when it actually
	// changed — refreshing unconditionally rebuilds the reading pane (re-pinning
	// the native text overlay) and flickers the screen for an AI-key-only change.
	redLetterAtOpen := redLetterEnabled()

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
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.Name
		nameToID[p.Name] = p.ID
		idToName[p.ID] = p.Name
	}

	// keyArea shows only the selected provider's key + status; it rebuilds when the
	// picker changes. Everything auto-saves straight to the on-device store — there
	// is no Save/Cancel — so there's no pending-edits buffer to flush.
	keyArea := container.NewStack()
	var renderKey func(id string)
	renderKey = func(id string) {
		info, ok := providerByID(id)
		if !ok {
			return
		}

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
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				_, err := info.New(key).generate(ctx, "Reply with the single word: OK")
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

		refreshStatus := func() {
			if strings.TrimSpace(entry.Text) != "" {
				status.Text = "✓ Saved on this device."
				status.Color = pal.Accent
				clearBtn.Enable()
			} else {
				status.Text = info.KeyHint
				status.Color = pal.TextMuted
				clearBtn.Disable()
			}
			status.Refresh()
		}
		// Auto-save: every edit writes straight to the on-device key store.
		entry.OnChanged = func(s string) {
			store.setAPIKey(id, strings.TrimSpace(s))
			refreshStatus()
		}
		refreshStatus()

		keyArea.Objects = []fyne.CanvasObject{
			container.NewVBox(
				container.NewBorder(nil, nil, heading, link),
				withCaret(state, entry),
				status,
				// Paste + Clear + Test sit on the left; the result label fills the
				// rest, so showing it never grows the sheet.
				container.NewBorder(nil, nil, container.NewHBox(pasteBtn, clearBtn, testBtn), nil, result),
			),
		}
		keyArea.Refresh()
	}

	active := widget.NewRadioGroup(names, func(name string) {
		if id, ok := nameToID[name]; ok {
			store.setActiveProvider(id) // auto-save
			renderKey(id)
		}
	})
	active.Required = true
	active.SetSelected(idToName[store.activeProvider()])
	renderKey(store.activeProvider())

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
		if redLetterEnabled() != redLetterAtOpen {
			state.refreshReadingOnly() // red-letter changed → re-render the verses
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

	// In-app disclosure of where AI prompts go, shown right under the key field
	// (Guideline 5.1.2 — be transparent before user content leaves the device). It
	// mirrors the privacy policy and links to it.
	aiNote := widget.NewRichText(&widget.TextSegment{
		Text:  "When you use AI study, the passage you select and your question are sent to the AI provider you choose, using your key.",
		Style: widget.RichTextStyle{ColorName: colorNameMuted, SizeName: theme.SizeNameCaptionText},
	})
	aiNote.Wrapping = fyne.TextWrapWord
	aiDisclosure := container.NewVBox(aiNote)
	if u, err := url.Parse("https://cubancorona.github.io/bibletext/"); err == nil {
		aiDisclosure.Add(container.NewHBox(widget.NewHyperlink("Privacy Policy ↗", u), layout.NewSpacer()))
	}

	// Assistant + key first so the key field sits high in the sheet — on a phone
	// the soft keyboard covers the lower half, and this keeps the field above it.
	// Section labels (not separators) divide the groups, keeping the sheet tight.
	form := container.NewVBox(
		sectionLabel("ASSISTANT", pal),
		active,
		keyArea,
		aiDisclosure,
		sectionLabel("READING", pal),
		redLetter,
	)

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
	card := container.New(fixedWidthLayout{width: ps.Width},
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
	var watchDismiss func()
	watchDismiss = func() {
		if popup == nil || !popup.Visible() {
			done()
			return
		}
		time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watchDismiss) })
	}
	time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watchDismiss) })
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
