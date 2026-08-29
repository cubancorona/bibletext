package bibletext

// The audio source menu, opened by tapping the source indicator beside the play
// button (iOS, while a chapter is loaded). It explains in words how the chapter is
// being read aloud — a recorded narration vs on-device text-to-speech — and, when
// more than one source is available, lets the reader switch between them. As more
// recordings are added per chapter, they appear here as additional rows.
//
// It uses the same popup scaffolding as the settings sheet: hide the native
// reading overlay while up, restore it on dismiss (a tap outside or the ✕), and a
// poll that catches an outside-tap close.

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func showAudioSourceMenu(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()
	// curKind/curRecID is the source the play button will use — the reader's current
	// choice (or the per-chapter default). The menu only CHANGES this choice; it
	// never starts playback (that's the play button's job).
	curKind, curRecID := gAudio.effectiveSource(state)
	recs := chapterRecordings(state)

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

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
	}

	title := canvas.NewText("Audio source", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), done)
	closeBtn.Importance = widget.LowImportance
	header := container.NewBorder(nil, nil, container.NewCenter(title), container.NewCenter(closeBtn))

	// Selectable sources, the chosen one highlighted — one row per recording the
	// chapter has (named after its narrator), plus read-aloud. Tapping a different
	// one just CHOOSES it (the play button then plays it); tapping the current one
	// closes.
	rows := container.NewVBox()
	addSource := func(kind audioKind, recID string, icon fyne.Resource, label string) {
		k, id := kind, recID
		isCurrent := k == curKind && (k != audioRecorded || id == curRecID)
		btn := widget.NewButtonWithIcon(label, icon, func() {
			if !isCurrent {
				gAudio.selectSource(state, k, id) // set the source; does NOT start playback
			}
			done()
		})
		btn.Alignment = widget.ButtonAlignLeading
		if isCurrent {
			btn.Importance = widget.HighImportance // the chosen source
		}
		rows.Add(btn)
	}
	for _, r := range recs {
		// The person glyph is the app's mark for "a human read this" — a machine
		// voice gets the computer glyph instead, so the row cannot be mistaken for
		// a human narration.
		icon := theme.AccountIcon()
		if r.synthetic {
			icon = theme.ComputerIcon()
		}
		addSource(audioRecorded, r.id, icon, "Recorded · "+r.narrator)
	}
	if ttsSupported() {
		addSource(audioTTS, "", iconAudioWave, "Read aloud · text to speech")
	}

	// The copy describes what is actually on offer. A synthetic recording is named
	// as one: no human read these books in the public domain, and saying "narration"
	// unqualified would imply otherwise.
	alt := " You can switch to your device reading it aloud instead."
	if !ttsSupported() {
		alt = ""
	}
	explain := "Your device is reading this chapter aloud (text to speech). No recorded narration is available for it yet."
	switch {
	case len(recs) > 1:
		explain = "This chapter has several public-domain recordings. Pick a voice, or have your device read it aloud instead."
	case len(recs) == 1 && recs[0].synthetic:
		explain = "This chapter has a public-domain recording read by a synthetic (computer) voice — no human recording of it exists in the public domain." + alt
	case len(recs) == 1:
		explain = "This chapter has a public-domain recorded narration." + alt
	}
	note := widget.NewLabel(explain)
	note.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(header, widget.NewSeparator(), rows, note)

	w := cnv.Size().Width - 48
	if w > 360 {
		w = 360
	}
	if w < 260 {
		w = 260
	}
	card := container.New(fixedWidthLayout{width: w},
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}))

	popup = widget.NewPopUp(card, cnv)
	popup.Resize(card.MinSize())
	x := (cnv.Size().Width - w) / 2
	if x < 0 {
		x = 0
	}
	y := float32(28)
	if pos, _ := cnv.InteractiveArea(); pos.Y > 0 {
		y = pos.Y + 16
	}
	popup.ShowAtPosition(fyne.NewPos(x, y))

	// Catch an outside-tap close (Fyne's PopUp.Hide doesn't call our done()).
	var watch func()
	watch = func() {
		if popup == nil || !popup.Visible() {
			// Skip when another overlay owns the canvas (a sheet reopened
			// within one tick of a rebuild drain) — see goto.go's watchdog.
			if cnv.Overlays().Top() == nil {
				done()
			}
			return
		}
		time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watch) })
	}
	time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watch) })
}
