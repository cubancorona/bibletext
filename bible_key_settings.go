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
func bibleKeySection(state *AppState, pal palette, onKeyPresence func()) (rows, footer fyne.CanvasObject) {
	store := state.keys()

	footer = caption("The New King James Version downloads with your own free API.Bible key — " +
		"create a key, add the NKJV to it, then choose NKJV from the translation picker.")

	entry := widget.NewPasswordEntry()
	entry.SetPlaceHolder("Paste your API.Bible key")
	entry.SetText(store.bibleAPIKey())

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

	testBtn := widget.NewButtonWithIcon("Test key", theme.MediaPlayIcon(), func() {
		key := strings.TrimSpace(entry.Text)
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
	clearBtn := widget.NewButtonWithIcon("Clear", theme.ContentClearIcon(), func() {
		entry.SetText("") // fires OnChanged → clears the saved key
	})

	saveOK := true
	refreshStatus := func() {
		savedLabel := "✓ Saved on this device."
		if store.keyInSecureStore(bibleKeyID) {
			savedLabel = "✓ Saved in the Keychain."
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
	entry.OnChanged = func(s string) {
		saveOK = store.setBibleAPIKey(strings.TrimSpace(s))
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
		settingsRow("Key", entry),
		container.NewHBox(pasteBtn, testBtn, clearBtn, layout.NewSpacer()),
		// Status left, "Get a key ↗" right — the same row shape as the
		// assistant key area, so the two sections read as one design.
		container.NewBorder(nil, nil, nil, link, status),
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
