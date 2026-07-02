//go:build !race

// Skipped under -race like the other Fyne-render tests (the test app clears its
// font cache on a background goroutine, which races text measurement).

package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// collectIconButtons walks the object tree gathering every *iconTapButton.
func collectIconButtons(o fyne.CanvasObject) []*iconTapButton {
	switch v := o.(type) {
	case *iconTapButton:
		return []*iconTapButton{v}
	case *fyne.Container:
		var out []*iconTapButton
		for _, c := range v.Objects {
			out = append(out, collectIconButtons(c)...)
		}
		return out
	}
	return nil
}

// TestAudioCardHitRegionsMatchLayout locks in the fix for the dead/mis-wired audio
// transport: tapping the canvas at each control's own laid-out centre must fire THAT
// control's callback. On desktop the expanded card is swapped into the header's
// Stack host in place; a construction whose drawn glyphs and tap targets disagree
// made the visible ▶ actually hit skip-back (a silent no-op), so play "did nothing".
// The card is placed exactly the way reading.go's chapterHeader places it.
func TestAudioCardHitRegionsMatchLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()

	var fired []string
	rec := func(name string) func() { return func() { fired = append(fired, name) } }
	card := buildAudioCard(state, audioRecorded, false, true, audioCardCallbacks{
		onSrc: rec("src"), onBack: rec("back"), onPlay: rec("play"),
		onFwd: rec("fwd"), onClose: rec("close"),
	})

	// Mirror audioControl + chapterHeader (audio_button.go / reading.go): a Stack
	// host that began life holding the collapsed speaker, wrapped in the fixed
	// GridWrap cell reserved at the expanded card's MinSize, inside
	// HBox(cell, hgap, focusBtn), vertically centred in the Border's right column.
	// The card is then swapped in via the live expand path (Objects mutation) —
	// the geometry contract under test is that the swap changes nothing.
	speaker := newIconTapButton(state, theme.VolumeUpIcon(), 20, 34, func() {})
	host := container.NewStack(container.NewHBox(layout.NewSpacer(), container.NewCenter(speaker)))
	probe := buildAudioCard(state, audioTTS, false, false, audioCardCallbacks{})
	cell := container.NewGridWrap(probe.MinSize(), host)
	focusBtn := widget.NewButtonWithIcon("", theme.ViewFullScreenIcon(), func() {})
	focusBtn.Importance = widget.LowImportance
	rightControls := container.NewHBox(cell, hgap(8), focusBtn)
	right := container.NewVBox(layout.NewSpacer(), rightControls, layout.NewSpacer())
	title := widget.NewLabel("Romans 8")
	chapterLine := widget.NewLabel("Chapter 8 of 16")
	left := container.NewVBox(title, chapterLine)
	row := container.NewBorder(nil, nil, left, right, nil)

	win := app.NewWindow("hit")
	defer win.Close()
	win.SetContent(container.NewBorder(container.NewVBox(row, widget.NewSeparator()), nil, nil, nil, widget.NewLabel("reading pane")))
	win.Resize(fyne.NewSize(1000, 700))

	// The live expand path: swap the card into the host in place and refresh.
	host.Objects = []fyne.CanvasObject{card}
	host.Refresh()

	buttons := collectIconButtons(card)
	if len(buttons) != 4 {
		t.Fatalf("expected 4 icon buttons in the card, found %d", len(buttons))
	}
	byIcon := func(sub string) *iconTapButton {
		for _, b := range buttons {
			if strings.Contains(b.icon.Name(), sub) {
				return b
			}
		}
		t.Fatalf("no button with icon containing %q", sub)
		return nil
	}
	targets := []struct {
		name string
		obj  fyne.CanvasObject
	}{
		{"back", byIcon("skip_back")},
		{"play", byIcon("play")},
		{"fwd", byIcon("skip_fwd")},
		{"src", byIcon("account")},
	}

	for _, tgt := range targets {
		fired = nil
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(tgt.obj)
		sz := tgt.obj.Size()
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("%s has no laid-out size (%v) — card never got a real layout", tgt.name, sz)
		}
		test.TapCanvas(win.Canvas(), pos.Add(fyne.NewPos(sz.Width/2, sz.Height/2)))
		if len(fired) != 1 || fired[0] != tgt.name {
			t.Errorf("tap at %s's centre (%v+%v/2) fired %v, want [%s]", tgt.name, pos, sz, fired, tgt.name)
		}
	}
}
