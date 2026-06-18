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
		testBtn := widget.NewButton("Test key", func() {
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
		testBtn.Importance = widget.LowImportance

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
				entry,
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

	// --- Chrome.
	title := canvas.NewText("Settings", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	intro := widget.NewLabel("Reading options, and optional AI study with your own key. Everything stays on this device.")
	intro.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(title, intro, widget.NewSeparator())

	// Reading options. Auto-save: toggle writes the pref immediately; the chapter
	// re-renders when the panel closes (Done).
	redLetter := widget.NewCheck("Show the words of Christ in red", nil)
	redLetter.SetChecked(redLetterEnabled())
	redLetter.OnChanged = func(b bool) { setRedLetterEnabled(b) }

	// Assistant + key first so the key field sits high in the sheet — on a phone
	// the soft keyboard covers the lower half, and this keeps the field above it.
	form := container.NewVBox(
		sectionLabel("ASSISTANT", pal),
		active,
		keyArea,
		widget.NewSeparator(),
		sectionLabel("READING", pal),
		redLetter,
	)
	body := container.NewVScroll(container.NewPadded(form))

	// No Save/Cancel — every change is already saved. Done just closes the panel
	// and re-renders the chapter so a red-letter toggle takes effect.
	var popup *widget.PopUp
	done := func() {
		if popup != nil {
			popup.Hide()
		}
		restore()
		state.refreshReadingOnly()
	}
	doneBtn := widget.NewButton("Done", done)
	doneBtn.Importance = widget.HighImportance

	note := canvas.NewText("Changes save automatically.", pal.TextMuted)
	note.TextSize = 11
	footer := container.NewVBox(
		widget.NewSeparator(),
		note,
		container.NewBorder(nil, nil, nil, doneBtn),
	)

	content := container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.Surface, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	// Size to the form's natural height so there's no empty space below it,
	// capped to the screen (the body scrolls if a small window forces it).
	ps := aiPanelSize(cnv.Size())
	h := header.MinSize().Height + form.MinSize().Height + footer.MinSize().Height + 84
	if h > ps.Height {
		h = ps.Height
	}
	// Phone: ride tall so the sheet sits near the top of the screen, keeping the
	// key field above the soft keyboard (the form scrolls inside if it overflows).
	if fyne.CurrentDevice().IsMobile() {
		h = ps.Height
	}
	popup.Resize(fyne.NewSize(ps.Width, h))
}
