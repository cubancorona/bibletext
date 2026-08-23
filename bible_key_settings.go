package bibletext

// The Settings → Translations section: the reader's own API.Bible key (BYOK)
// unlocking licensed translations — today the NKJV. Mirrors the AI key row's
// conventions exactly (auto-save on change, Paste/Clear with visible button
// chrome, a keychain-aware saved status, an async Test), so the two key
// surfaces feel like one design.

import (
	"context"
	"net/http"
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
// WHY NOT SAY BOTH: measured at the app's 18pt body size, "[Included with
// BibleText]" is 195pt and fits the ~199pt a 320pt phone gives the box — and
// was rendered at 320pt to confirm it clears the reveal icon, since that budget
// is a conservative estimate rather than a measured limit. Anything that also
// names the action runs past it —
// anything that also names the action runs past it — "Included — paste to
// replace" is 223pt, and the fuller sentence tried once before was 428pt. One
// idea is what fits.
//
// PURE, and taking the state rather than reading it, so the three call sites —
// construction, Clear, and emptying the field by hand — cannot drift. That last
// one is why this is a function at all: OnChanged clears the bundled key for
// good, so a placeholder set only at construction would go on claiming the key
// was included after the reader deleted it.
func bibleKeyPlaceholder(usingBundled bool) string {
	if usingBundled {
		// BRACKETED because it is a DESCRIPTION, not a value and not an
		// instruction. This is a password field: text sitting where masked
		// characters would be can read as content, and the brackets say at a
		// glance that it is neither something typed nor something to type.
		// The other branch stays unbracketed — an imperative already reads as a
		// hint, and bracketing an instruction would only muddle the distinction
		// this draws.
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

	status := canvas.NewText("", pal.TextMuted)
	status.TextSize = 12

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
	// force — the stored one. Everything below asks THIS rather than reading
	// entry.Text, so hiding the bundled key's characters cannot quietly turn
	// Test into "paste a key first", disable Clear, or make the status claim
	// there is no key.
	keyInUse := func() string {
		if t := strings.TrimSpace(entry.Text); t != "" {
			return t
		}
		return strings.TrimSpace(store.bibleAPIKey())
	}

	testBtn := widget.NewButtonWithIcon("Test key", theme.MediaPlayIcon(), func() {
		key := keyInUse()
		result.Show()
		remeasure() // the result line just appeared — the sheet grew
		if key == "" {
			setResult("Paste a key first.")
			return
		}
		setResult("Testing…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), bibleKeyProbeBudget)
			defer cancel()
			client := &http.Client{Timeout: bibleKeyProbeBudget}
			var meta struct {
				Data struct {
					Name string `json:"name"`
				} `json:"data"`
			}
			err := apiBibleGet(ctx, client, key, "/bibles/"+nkjvProviderBibleID, &meta)
			fyne.Do(func() {
				switch {
				case err != nil:
					setResult("✗ " + friendlyBibleKeyError(err))
				case meta.Data.Name == "":
					setResult("✗ The key works, but the NKJV isn't on it — add the New King James Version to your API.Bible app.")
				default:
					setResult("✓ Key works.\n" + meta.Data.Name + " is now available in the translation picker.")
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
	refreshStatus := func() {
		savedLabel := "✓ Saved on this device."
		if store.keyInSecureStore(bibleKeyID) {
			savedLabel = "✓ Saved in the Keychain."
		}
		// Say plainly when the key in use is the one that shipped with the
		// app rather than one the reader supplied — it is a shared key with a
		// shared quota, and a reader deciding whether to add their own
		// deserves to know which they are looking at.
		if store.usingBundledBibleKey() {
			savedLabel = "✓ Included with BibleText — or paste your own."
		}
		if keyInUse() != "" {
			if saveOK {
				status.Text = savedLabel
				status.Color = pal.Accent
			} else {
				status.Text = "Couldn't save this key securely. Please try again."
				status.Color = theme.Color(theme.ColorNameError)
			}
			clearBtn.Enable()
		} else if !saveOK {
			status.Text = "Couldn't remove the stored key. Please try again."
			status.Color = theme.Color(theme.ColorNameError)
			clearBtn.Enable()
		} else {
			status.Text = "Free for personal use — no card, no charge."
			status.Color = pal.TextMuted
			clearBtn.Disable()
		}
		status.Refresh()
	}
	hadKey := store.bibleAPIKey() != ""
	// Clearing has to reach the STORE directly. With the bundled key in force the
	// field is already empty, so emptying it again fires no OnChanged and the key
	// would have survived a tap on Clear — the one control whose whole job is to
	// remove it (README: "Clear removes it for good").
	clearBibleKey = func() {
		entry.SetText("")
		saveOK = store.setBibleAPIKey("")
		store.noteBibleKeyCleared(true)
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
		saveOK = store.setBibleAPIKey(s)
		// Emptying the field is a decision, not an accident of state: record
		// it so the bundled key is not quietly re-seeded next launch. Typing
		// a key again cancels that.
		store.noteBibleKeyCleared(s == "")
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
		link = widget.NewHyperlink("Get a key ↗", u)
	}

	rows = container.NewVBox(
		// Named for the same reason the assistant's row is: the label must
		// still identify the key when read on its own — and the link rides the
		// label row, top-right of the box, matching the assistant section.
		container.NewBorder(nil, nil, container.NewCenter(widget.NewLabel("API.Bible key")), link),
		// Field on its own full-width line: an API key is long.
		entry,
		container.NewHBox(pasteBtn, testBtn, clearBtn, layout.NewSpacer()),
		// Status has the row to itself; the link is up on the label row.
		status,
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
