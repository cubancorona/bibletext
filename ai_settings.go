package holybible

// The AI settings panel (bring-your-own-key): pick the active AI and paste an
// API key per provider, with a link out to each provider's key page and a "Test"
// button. Keys are saved to the on-device key store (ai_keystore.go).

import (
	"context"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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

	// Active-provider chooser.
	names := make([]string, len(providers))
	nameToID := map[string]string{}
	idToName := map[string]string{}
	for i, p := range providers {
		names[i] = p.Name
		nameToID[p.Name] = p.ID
		idToName[p.ID] = p.Name
	}
	active := widget.NewRadioGroup(names, nil)
	active.SetSelected(idToName[store.activeProvider()])

	// One key field per provider.
	entries := map[string]*widget.Entry{}
	keyRows := container.NewVBox()
	for _, p := range providers {
		entry := widget.NewPasswordEntry()
		entry.SetPlaceHolder("Paste " + p.Name + " key")
		entry.SetText(store.apiKey(p.ID))
		entries[p.ID] = entry

		name := canvas.NewText(p.Name, pal.Text)
		name.TextStyle = fyne.TextStyle{Bold: true}

		var getLink fyne.CanvasObject
		if u, err := url.Parse(p.KeyURL); err == nil {
			getLink = widget.NewHyperlink("Get a key ↗", u)
		} else {
			getLink = layout.NewSpacer()
		}

		hint := canvas.NewText(p.KeyHint, pal.TextMuted)
		hint.TextSize = 11

		keyRows.Add(container.NewVBox(
			container.NewBorder(nil, nil, name, getLink),
			entry,
			hint,
			widget.NewSeparator(),
		))
	}

	// Test the selected provider's key with a tiny request.
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	testBtn := widget.NewButton("Test selected", func() {
		id := nameToID[active.Selected]
		info, ok := providerByID(id)
		if !ok {
			return
		}
		key := strings.TrimSpace(entries[id].Text)
		if key == "" {
			status.SetText("Paste a " + info.Name + " key first.")
			return
		}
		status.SetText("Testing " + info.Name + "…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, err := info.New(key).generate(ctx, "Reply with the single word: OK")
			fyne.Do(func() {
				if err != nil {
					status.SetText("✗ " + friendlyAIError(err))
				} else {
					status.SetText("✓ " + info.Name + " is working.")
				}
			})
		}()
	})

	var popup *widget.PopUp
	saveBtn := widget.NewButton("Save", func() {
		for id, entry := range entries {
			store.setAPIKey(id, entry.Text)
		}
		if active.Selected != "" {
			store.setActiveProvider(nameToID[active.Selected])
		}
		if popup != nil {
			popup.Hide()
		}
		restore()
	})
	saveBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", func() {
		if popup != nil {
			popup.Hide()
		}
		restore()
	})

	title := canvas.NewText("AI study settings", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	intro := widget.NewLabel("Bring your own key. Choose an AI and paste its API key — keys stay on this device.")
	intro.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(title, intro, widget.NewSeparator())

	form := container.NewVBox(
		sectionLabel("ACTIVE AI", pal),
		active,
		widget.NewSeparator(),
		sectionLabel("API KEYS", pal),
		keyRows,
		container.NewHBox(testBtn),
		status,
	)
	body := container.NewVScroll(container.NewPadded(form))

	note := canvas.NewText("Keys are stored on this device.", pal.TextMuted)
	note.TextSize = 11
	footer := container.NewVBox(
		widget.NewSeparator(),
		note,
		container.NewHBox(layout.NewSpacer(), cancelBtn, saveBtn),
	)

	content := container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.Surface, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	popup.Resize(aiPanelSize(cnv.Size()))
}
