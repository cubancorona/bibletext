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

// findLabeledChip walks the object tree for the card's *labeledTapChip.
func findLabeledChip(o fyne.CanvasObject) *labeledTapChip {
	switch v := o.(type) {
	case *labeledTapChip:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if got := findLabeledChip(c); got != nil {
				return got
			}
		}
	}
	return nil
}

// findTappableArea walks the object tree for the card's close ✕ cell.
func findTappableArea(o fyne.CanvasObject) *tappableArea {
	switch v := o.(type) {
	case *tappableArea:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if got := findTappableArea(c); got != nil {
				return got
			}
		}
	}
	return nil
}

// TestAudioCardSourceChipClearsCloseButton locks in that the source chip and the
// close ✕ never overlap. The ✕ is overlaid on the card's upper-right corner
// through a Stack, so it contributes nothing to the top row's width; the chip is
// centred in that row. A label wide enough to reach the corner therefore slid
// straight under the ✕ — "Read aloud ▾" (the read-aloud/TTS label) did, while the
// shorter "Narrator ▾" happened to clear it, so the collision only showed on
// chapters without a recording. Both labels are checked at their real laid-out
// positions, since the whole failure was one of geometry rather than of drawing.
func TestAudioCardSourceChipClearsCloseButton(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := sampleState()

	for _, tc := range []struct {
		name string
		kind audioKind
	}{
		{"Read aloud (TTS)", audioTTS},
		{"Narrator (recorded)", audioRecorded},
	} {
		card := buildAudioCard(state, tc.kind, false, false, false, tc.kind == audioRecorded, audioCardCallbacks{})
		win := app.NewWindow("card")
		// The reader reserves exactly the card's MinSize for it (reading.go's
		// chapterHeader), so measure it at that size — the width it really gets.
		win.SetContent(container.NewVBox(container.NewGridWrap(card.MinSize(), card)))
		win.Resize(fyne.NewSize(1000, 700))

		chip := findLabeledChip(card)
		if chip == nil {
			t.Fatalf("%s: no labeledTapChip in the card", tc.name)
		}
		closeCell := findTappableArea(card)
		if closeCell == nil {
			t.Fatalf("%s: no close ✕ cell in the card", tc.name)
		}
		chipPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(chip)
		closePos := fyne.CurrentApp().Driver().AbsolutePositionForObject(closeCell)
		chipSz, closeSz := chip.Size(), closeCell.Size()
		if chipSz.Width <= 0 || closeSz.Width <= 0 {
			t.Fatalf("%s: card never got a real layout (chip %v, close %v)", tc.name, chipSz, closeSz)
		}
		t.Logf("%s: card width %.1f, chip x=%.1f..%.1f (w %.1f), ✕ x=%.1f..%.1f",
			tc.name, card.MinSize().Width, chipPos.X, chipPos.X+chipSz.Width, chipSz.Width, closePos.X, closePos.X+closeSz.Width)
		overlapX := chipPos.X+chipSz.Width > closePos.X && closePos.X+closeSz.Width > chipPos.X
		overlapY := chipPos.Y+chipSz.Height > closePos.Y && closePos.Y+closeSz.Height > chipPos.Y
		if overlapX && overlapY {
			t.Errorf("%s: source chip overlaps the close ✕ — chip x=%.1f..%.1f, ✕ x=%.1f..%.1f (card width %.1f)",
				tc.name, chipPos.X, chipPos.X+chipSz.Width, closePos.X, closePos.X+closeSz.Width, card.MinSize().Width)
		}
		win.Close()
	}
}

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
	card := buildAudioCard(state, audioRecorded, false, false, false, true, audioCardCallbacks{
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
	probe := buildAudioCard(state, audioTTS, false, false, false, false, audioCardCallbacks{})
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
	if len(buttons) != 3 {
		t.Fatalf("expected 3 icon buttons in the card (transport), found %d", len(buttons))
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
	srcChip := findLabeledChip(card)
	if srcChip == nil {
		t.Fatal("no labeledTapChip (the narrator/source selector) in the card")
	}
	targets := []struct {
		name string
		obj  fyne.CanvasObject
	}{
		{"back", byIcon("skip_back")},
		{"play", byIcon("play")},
		{"fwd", byIcon("skip_fwd")},
		{"src", srcChip},
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

// TestCollapsedControlClearsStaleReadAlong locks in the rule that the read-along
// tint only outlives playback while the audio card is open: rendering the
// COLLAPSED control with nothing actively playing must drop the armed timing
// table (and with it the on-screen highlight), while a still-playing collapsed
// control must leave it armed so the highlight keeps following the narration.
// The collapsed render runs on every close and every audio state change, so this
// covers both orders (pause → close, and close → pause from the lock screen).
func TestCollapsedControlClearsStaleReadAlong(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := sampleState()
	fp := chapterAudioFingerprint(state)

	setAudio := func(s audioPlayState, armed bool) {
		gAudio.mu.Lock()
		gAudio.loaded = true
		gAudio.loadedFP = fp
		gAudio.kind = audioRecorded
		gAudio.state = s
		if armed {
			gAudio.readAlong = []verseTiming{{1, 0, 1}}
			gAudio.readAlongVerse = 1
		}
		gAudio.mu.Unlock()
	}
	armedNow := func() bool {
		gAudio.mu.Lock()
		defer gAudio.mu.Unlock()
		return gAudio.readAlong != nil
	}
	defer func() { // reset the package singleton for other tests
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.state = false, "", audioIdle
		gAudio.readAlong, gAudio.readAlongVerse = nil, 0
		gAudio.mu.Unlock()
	}()

	audioPanelOpen = false

	// Paused + collapsed → stale tint must clear.
	setAudio(audioPaused, true)
	_ = audioControlContent(state, 34, func() {})
	if armedNow() {
		t.Fatal("collapsed control with paused audio kept the read-along table armed")
	}

	// Playing + collapsed → the highlight keeps following; table must survive.
	setAudio(audioPlaying, true)
	_ = audioControlContent(state, 34, func() {})
	if !armedNow() {
		t.Fatal("collapsed control cleared the read-along table while audio was still playing")
	}

	// Buffering + collapsed → about to play; the armed table must survive too.
	setAudio(audioBuffering, true)
	_ = audioControlContent(state, 34, func() {})
	if !armedNow() {
		t.Fatal("collapsed control cleared the read-along table while the stream was buffering")
	}
}
