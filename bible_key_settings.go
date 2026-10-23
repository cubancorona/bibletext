package bibletext

// The Settings → Translations section: the reader's own API.Bible key (BYOK)
// unlocking licensed translations — today the NKJV. Mirrors the AI key row's
// conventions exactly (auto-save on change, Paste/Clear with visible button
// chrome, a keychain-aware saved status, an async Test), so the two key
// surfaces feel like one design.

import (
	"context"
	"image/color"
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

// bibleKeyProbeBudget bounds the Test call — one metadata request.
const bibleKeyProbeBudget = 12 * time.Second

// keyNotRestoredLine describes the same definitive absence under each empty
// key field: a saved mark remains, the credential store holds no key and no
// copy remains in Preferences (keyLeftOnOldDevice). It does not infer a
// restore or explain why the key is missing. A store error is not absence.
// The line replaces the field's hint until dismissed with its ✕.
const keyNotRestoredLine = "Your saved key isn't available on this device. Paste it again here."

// keyStatusRow is a key field's status line with, to its right while the
// line says keyNotRestoredLine, the ✕ that dismisses it (decision 13 in
// docs/ROADMAP.md): a reader with no key to paste can put the line away, and
// it stays away for that restore (keyNotRestoredDismissed). The same row
// serves the API.Bible key's field and each AI provider's, so the rule is the
// same wherever the line is said.
//
// The ✕ is an icon button of low importance with no words, as the Settings
// header's close control is; the toolkit gives a control no accessibility
// label, so there is none to name. It is small and level with the line's
// first row (decision 13): its glyph is lineCloseGlyph, smaller than the
// header's, and lineCloseLayout puts the glyph's centre on the centre of the
// line's first row, however many rows the line breaks into, on every
// platform and at every text size. The column keeps the previous button's
// minimum size; only touch devices use its spare space for an invisible tap
// target. Hidden, the column gives its width back to the line.
type keyStatusRow struct {
	row   *fyne.Container
	close *fyne.Container
}

func newKeyStatusRow(status *statusLine, onDismiss func()) *keyStatusRow {
	btn := widget.NewButtonWithIcon("", theme.CancelIcon(), onDismiss)
	btn.Importance = widget.LowImportance
	minimum := btn.MinSize()
	small := container.NewThemeOverride(btn, lineCloseTheme{fyne.CurrentApp().Settings().Theme()})
	target := newTappableArea(canvas.NewRectangle(color.Transparent), onDismiss)
	if !fyne.CurrentDevice().IsMobile() {
		target.Hide()
	}
	closeCol := container.New(lineCloseLayout{line: status, minimum: minimum}, target, small)
	closeCol.Hide()
	return &keyStatusRow{row: container.NewBorder(nil, nil, nil, closeCol, status), close: closeCol}
}

// lineCloseGlyph is the size the ✕ beside a key field's line draws its glyph
// at: the size of the app's other small inline icon buttons, the results
// trail's ✕ and the AI panel's passage disclosure (trailChipTheme,
// aiPassageDisclosureTheme), where the Settings header's ✕ draws the theme's
// inline icon, 20pt.
const lineCloseGlyph float32 = 14

// lineCloseBox is the side of the button drawn around that glyph, where its
// hover and its press show: the glyph and the inner padding the results
// trail's ✕ has, 5pt on each side.
const lineCloseBox = lineCloseGlyph + 2*5

// lineCloseTheme draws the ✕'s glyph at lineCloseGlyph. The button has no
// words, so its subtree holds no text for an override's own font caches to
// measure differently (showAISettings).
type lineCloseTheme struct{ fyne.Theme }

func (t lineCloseTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInlineIcon:
		return lineCloseGlyph
	case theme.SizeNameInnerPadding:
		return (lineCloseBox - lineCloseGlyph) / 2
	}
	return t.Theme.Size(name)
}

// lineCloseLayout keeps the status row's original allocation while drawing
// the smaller button level with its first line. The touch-only target starts
// at the row's top and fits wholly inside its column, capped at minTapTarget.
// A short line therefore keeps its original height rather than growing to
// make room for a target, and no target extends into Clear above the line.
//
// On iOS and Android the toolkit's mobile driver moves each touch 8pt up
// before it finds what is under it (tapYOffset), as it does for every
// control. This layout does not shift its target towards another control to
// compensate for that offset.
type lineCloseLayout struct {
	line    *statusLine
	minimum fyne.Size
}

func (l lineCloseLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return l.minimum
}

func (l lineCloseLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) != 2 {
		return
	}
	target, button := objs[0], objs[1]
	targetWidth := fyne.Min(size.Width, minTapTarget)
	target.Move(fyne.NewPos((size.Width-targetWidth)/2, 0))
	target.Resize(fyne.NewSize(targetWidth, fyne.Min(size.Height, minTapTarget)))
	centre := l.line.rowHeight() / 2
	button.Move(fyne.NewPos((size.Width-lineCloseBox)/2, centre-lineCloseBox/2))
	button.Resize(fyne.NewSquareSize(lineCloseBox))
}

// showClose shows or hides the ✕ and reports whether that changed anything:
// the width the line breaks in changes with it, and so can the sheet's
// height.
func (r *keyStatusRow) showClose(show bool) bool {
	if r.close.Visible() == show {
		return false
	}
	if show {
		r.close.Show()
	} else {
		r.close.Hide()
	}
	r.row.Refresh()
	return true
}

// bibleKeySection builds the Translations settings group: the rows that live
// on the group card, and the footnote that sits below it. onKeyPresence is
// called whenever the stored key appears or disappears (or the area grows),
// so the sheet can re-measure itself.
// bibleKeyPlaceholder is what the EMPTY field should say, for the state it is
// empty in.
//
// The bundled key's characters are withheld (see the note in bibleKeySection),
// so with it in force the box is empty — and an empty box reading "Paste your
// API.Bible key" says there is no key when one is working: the field reads as
// unset and asks to be filled while a working key is already in force. It now
// describes the state instead, and the Paste button beside it plus the status
// line under it carry "how do I change this".
//
// At the app's 18pt body size, "[Included with BibleText]" is 195pt and fits
// the approximately 199pt available at 320pt width without touching the reveal
// icon. Copy that also describes the replacement action exceeds that budget;
// the adjacent Paste button and status line carry the action and state details.
//
// The helper takes the state rather than reading it, so the three call sites —
// construction, Clear, and emptying the field by hand — cannot drift. That last
// one is why this is a function at all: OnChanged clears the bundled key for
// good, so a placeholder set only at construction would go on claiming the key
// was included after the reader deleted it.
func bibleKeyPlaceholder(usingBundled bool) string {
	if usingBundled {
		// Brackets distinguish this state description from masked field content
		// and from the unbracketed instruction used when no key is available.
		// The other branch stays unbracketed — an imperative already reads as a
		// hint.
		return "[Included with BibleText]"
	}
	return "Paste your API.Bible key"
}

func bibleKeySection(state *AppState, pal palette, onKeyPresence func()) (rows, footer fyne.CanvasObject) {
	store := state.keys()

	footer = caption("The New King James Version downloads with your own free API.Bible key — " +
		"create a key, add the NKJV to it, then choose NKJV from the translation picker.")

	// The BUNDLED key never reaches this field. A PasswordEntry hides its
	// characters but keeps them — one tap on the reveal eye prints the project's
	// production credential on screen, and a stored key is the reader's to see
	// only when it is THEIRS. Ours is a shared credential on a shared quota, and
	// showing it invites a reader to lift it, which costs every other reader.
	//
	// So a bundled key shows as an empty field. Nothing is hidden about the STATE
	// — refreshStatus below says plainly that the included key is in use — only
	// the characters are withheld. A key the reader pasted is shown exactly as
	// before: it is theirs, and being able to check it is how they spot a
	// truncated paste.
	//
	// The placeholder says only what TYPING here does; the status line under the
	// box is what says the included key is already working. It used to try to say
	// both ("Included with BibleText — paste your own to replace it") and ran off
	// the end of the field: measured 458pt of text at the app's 18pt body size in
	// a box with ~281pt of usable width on a Pro Max, and ~199pt on a 320pt phone.
	// Anything set here must be measured at 18 — theme.TextSize() is the stock 14
	// and flatters every string by 29%.
	entry := widget.NewPasswordEntry()
	usingBundled := store.usingBundledBibleKey()
	entry.SetPlaceHolder(bibleKeyPlaceholder(usingBundled))
	if !usingBundled {
		entry.SetText(store.bibleAPIKey())
	}

	// A statusLine rather than a canvas.Text: it breaks between words where
	// the row is narrower than the text (status_line.go).
	status := newStatusLine(12)

	// The test result speaks in the status voice (caption size), not
	// headline size — and a wrapping RichText re-flows dependably when its
	// text changes after a Show.
	result := widget.NewRichText()
	result.Wrapping = fyne.TextWrapWord
	result.Hide()

	remeasure := func() {
		if onKeyPresence != nil {
			onKeyPresence()
		}
	}
	setResult := func(s string) {
		result.Segments = []widget.RichTextSegment{&widget.TextSegment{
			Text:  s,
			Style: widget.RichTextStyle{SizeName: theme.SizeNameCaptionText},
		}}
		result.Refresh()
		remeasure()
	}

	// keyInUse is what the app would actually send: what the reader has typed,
	// or — when the field is deliberately empty because the bundled key is in
	// force — the effective runtime fallback. Everything below asks THIS rather
	// than reading entry.Text. Hiding the bundled key's characters therefore cannot
	// quietly turn Test into "paste a key first", disable Clear, or make the status claim
	// there is no key.
	keyInUse := func() string {
		if t := strings.TrimSpace(entry.Text); t != "" {
			return t
		}
		return strings.TrimSpace(store.bibleAPIKey())
	}

	// The line is painted from the test's progress on state
	// (key_test_progress.go), so a sheet a light/dark reopen brings back shows
	// the wait of a test still running and the verdict of one that landed
	// meanwhile.
	slot := state.keyTest(keyTestBible)
	slot.attach(func() {
		t := slot.test
		if t == nil {
			result.Hide()
			return
		}
		result.Show()
		if t.running {
			setResult("Testing…")
		} else {
			setResult(t.text)
		}
	})
	testBtn := widget.NewButtonWithIcon("Test key", theme.MediaPlayIcon(), func() {
		key := keyInUse()
		if key == "" {
			slot.begin("", false, "Paste a key first.")
			return
		}
		t := slot.begin("", true, "")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), bibleKeyProbeBudget)
			defer cancel()
			client := newHTTPClient()
			client.Timeout = bibleKeyProbeBudget
			var meta struct {
				Data struct {
					Name string `json:"name"`
				} `json:"data"`
			}
			err := apiBibleGet(ctx, client, key, "/bibles/"+nkjvProviderBibleID, &meta)
			fyne.Do(func() {
				switch {
				case err != nil:
					slot.finish(t, "✗ "+friendlyBibleKeyError(err))
				case meta.Data.Name == "":
					slot.finish(t, "✗ The key works, but the NKJV isn't on it — add the New King James Version to your API.Bible app.")
				default:
					slot.finish(t, "✓ Key works.\n"+meta.Data.Name+" is now available in the translation picker.")
				}
			})
		}()
	})

	pasteBtn := widget.NewButtonWithIcon("Paste", theme.ContentPasteIcon(), func() {
		if state.window == nil {
			return
		}
		clip := state.window.Clipboard()
		if clip == nil {
			return
		}
		if v := strings.TrimSpace(clip.Content()); v != "" {
			entry.SetText(v) // fires OnChanged, which auto-saves
		}
	})
	// Assigned below, once refreshStatus and the presence hook exist.
	var clearBibleKey func()
	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		if clearBibleKey != nil {
			clearBibleKey()
		}
	})

	saveOK := true
	var refreshStatus func()
	statusRow := newKeyStatusRow(status, func() {
		store.dismissKeyNotRestored(bibleKeyID)
		refreshStatus()
	})
	refreshStatus = func() {
		savedLabel := "✓ Saved on this device."
		if store.keyInSecureStore(bibleKeyID) {
			savedLabel = "✓ Saved in the " + store.secureStoreName() + "."
		}
		// Say plainly when the key in use is the one that shipped with the
		// app rather than one the reader supplied — it is a shared key with a
		// shared quota, and a reader deciding whether to add their own
		// deserves to know which they are looking at.
		if store.usingBundledBibleKey() {
			savedLabel = "✓ Included with BibleText — or paste your own."
		}
		var text string
		var col color.Color
		if keyInUse() != "" {
			if saveOK {
				text, col = savedLabel, pal.Accent
			} else {
				text, col = "Couldn't save this key securely. Please try again.", theme.Color(theme.ColorNameError)
			}
			clearBtn.Enable()
		} else if !saveOK {
			text, col = "Couldn't remove the stored key. Please try again.", theme.Color(theme.ColorNameError)
			clearBtn.Enable()
		} else {
			text, col = "Free for personal use — no card, no charge.", pal.TextMuted
			clearBtn.Disable()
		}
		// An empty field whose key a restore did not bring back says so,
		// whether or not the included key is working meanwhile: the field's
		// placeholder already says that, and this reader has a key of their
		// own to paste. Until they dismiss it with its ✕.
		notRestored := saveOK && strings.TrimSpace(entry.Text) == "" && store.keyNotRestoredShown(bibleKeyID)
		if notRestored {
			text, col = keyNotRestoredLine, pal.Text
		}
		// A status that now takes a line more or less changes the section's
		// height, and the sheet's with it; so can the ✕ coming or going.
		changed := status.set(text, col)
		if statusRow.showClose(notRestored) {
			changed = true
		}
		if changed {
			remeasure()
		}
	}
	hadKey := store.bibleAPIKey() != ""
	// Clearing has to reach the STORE directly. With the bundled key in force the
	// field is already empty, so emptying it again fires no OnChanged and the key
	// would have survived a tap on Clear — the one control whose whole job is to
	// remove it (README: "Clear removes it for good").
	clearBibleKey = func() {
		entry.SetText("")
		// The record says which key went: the included one when it was in
		// force, the field then being empty already (removeBibleKey).
		saveOK = store.removeBibleKey()
		giveUpKeyedTranslation(state)
		entry.SetPlaceHolder(bibleKeyPlaceholder(store.usingBundledBibleKey()))
		entry.Refresh()
		refreshStatus()
		if hadKey {
			hadKey = false
			if onKeyPresence != nil {
				onKeyPresence()
			}
		}
	}
	entry.OnChanged = func(s string) {
		s = strings.TrimSpace(s)
		// Emptying the field is a decision, not an accident of state: record
		// it, and which key it removed, so the bundled key is not quietly
		// re-seeded next launch. Typing a key again cancels that.
		if s == "" {
			saveOK = store.removeBibleKey()
			giveUpKeyedTranslation(state)
		} else {
			saveOK = store.setBibleAPIKey(s)
			store.noteBibleKeyCleared(false)
			takeBackKeyedTranslation(state)
		}
		// Emptying the field by hand clears the bundled key for good, so the
		// placeholder must stop advertising it — the Clear button is not the
		// only way to reach that state.
		entry.SetPlaceHolder(bibleKeyPlaceholder(store.usingBundledBibleKey()))
		refreshStatus()
		if has := store.bibleAPIKey() != ""; has != hadKey {
			hadKey = has
			if onKeyPresence != nil {
				onKeyPresence()
			}
		}
	}
	refreshStatus()

	var link fyne.CanvasObject = layout.NewSpacer()
	if u, err := url.Parse("https://api.bible/sign-up/starter"); err == nil {
		link = newOutboundLink("Get a key", u)
	}

	rows = container.NewVBox(
		// Named for the same reason the assistant's row is: the label must
		// still identify the key when read on its own — and the link rides the
		// label row, top-right of the box, matching the assistant section.
		container.NewBorder(nil, nil, container.NewCenter(widget.NewLabel("API.Bible key")), link),
		// Field on its own full-width line: an API key is long.
		entry,
		// The same row as the assistant's (key_actions_row.go).
		keyActionsRow(pasteBtn, testBtn, clearBtn),
		// Status has the row to itself, but for the ✕ of a key a restore did
		// not bring back; the link is up on the label row.
		statusRow.row,
		result,
	)
	return rows, footer
}

// friendlyBibleKeyError keeps provider errors readable at caption length —
// never a raw Go error on the sheet.
func friendlyBibleKeyError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "rejected the key"):
		return "API.Bible rejected this key — check it was copied whole."
	case strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "Client.Timeout"),
		strings.Contains(msg, "no such host"),
		strings.Contains(msg, "connection"):
		return "Couldn't reach api.bible — check your connection and try again."
	}
	if len(msg) > 160 {
		msg = msg[:160] + "…"
	}
	return msg
}
