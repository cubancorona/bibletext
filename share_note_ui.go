package bibletext

// Shared notes in the app: the sheet that writes one, and the card that shows
// one that arrived.
//
// Untagged on purpose — no build tag, no cgo — so the whole flow unit-tests on
// the host and the same code serves iPhone, iPad, macOS, Android, Windows and
// Linux. That matters more here than usual: the reading pane is a native
// overlay on three of those platforms, and a per-platform bubble would have
// been four implementations of the same conversation.
//
// Both surfaces observe the overlay invariant ([redacted-retired-private-reference] → Native text
// overlays): the native reading view floats ABOVE the Fyne canvas, so anything
// Fyne draws must hide it on open and restore it on close, or it renders behind
// the scripture and looks like nothing happened.

import (
	"image/color"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// noteEntrySlot is the transparent box the native compose field parks over. It
// is a widget rather than a bare rectangle so its own Resize/Move push the
// fresh absolute rect across to the native view — the live-tracking pattern
// nativeReadingHost uses for the reading overlay. Without it the field's frame
// was only re-asserted for 600ms after the sheet opened, and any later layout
// change left the UITextView stranded where the slot used to be.
//
// open latches it: a slot from a closed sheet must never push frames, or a
// stale relayout could reposition a native view belonging to nothing.
type noteEntrySlot struct {
	widget.BaseWidget
	open *bool
}

func newNoteEntrySlot(open *bool) *noteEntrySlot {
	s := &noteEntrySlot{open: open}
	s.ExtendBaseWidget(s)
	return s
}

func (s *noteEntrySlot) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(0, 112)) // ~4 comfortable lines at 18px
	return widget.NewSimpleRenderer(r)
}

func (s *noteEntrySlot) Resize(sz fyne.Size) {
	s.BaseWidget.Resize(sz)
	s.push()
}

func (s *noteEntrySlot) Move(p fyne.Position) {
	s.BaseWidget.Move(p)
	s.push()
}

// push projects the slot's rect immediately (responsive) and again a tick later
// (Resize/Move can fire mid-layout, before siblings have their final heights —
// same double-push the reading overlay uses).
func (s *noteEntrySlot) push() {
	if s.open == nil || !*s.open {
		return
	}
	setNativeNoteEntryFrameFromObject(s)
	time.AfterFunc(50*time.Millisecond, func() {
		fyne.Do(func() {
			if s.open != nil && *s.open {
				setNativeNoteEntryFrameFromObject(s)
			}
		})
	})
}

// noteEntryOnChanged is installed by the compose sheet while it is open, and
// fired (via fyne.Do) by the native field's //export callback on every edit —
// it drives the live character counter. nil whenever no sheet is up.
var noteEntryOnChanged func()

// promptShareNote collects an optional note, then shares the link carrying it.
// It is a SECOND verb beside "Share as link", never a step in front of it: the
// plain share stays one tap, because most shares carry no note and a modal in
// everyone's way to serve the minority is the wrong trade.
func promptShareNote(state *AppState, selectedText string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText == "" {
		return
	}
	pal := state.pal()
	mobile := fyne.CurrentDevice().IsMobile()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	// sheetOpen latches the native slot's frame-pushing (see noteEntrySlot): it
	// must drop on EVERY close path, or a stale relayout of this sheet's dead
	// slot could reposition a newer sheet's field.
	sheetOpen := true
	closeSheet := func() {
		if closed {
			return
		}
		closed = true
		sheetOpen = false
		noteEntryOnChanged = nil
		hideNativeNoteEntry() // no-op off iOS, and when the Fyne entry was used
		if popup != nil {
			popup.Hide() // removes it from the overlay stack synchronously
		}
		// Restore the reading overlay only when nothing else owns the canvas —
		// the same rule the keep/delete prompt follows. On the watchdog path a
		// window rebuild may already have re-pinned everything, and another
		// sheet may have opened since.
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	title := canvas.NewText("Add a note", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	ref := canvas.NewText(shareNoteReference(state, selectedText), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	// On iOS the field is a REAL UITextView floated over the sheet (dictation,
	// autocorrect, system selection, undo, VoiceOver, system emoji — see
	// note_entry_ios.go). Everywhere else it is the Fyne entry below. noteText
	// is the one place that knows which is live.
	useNative := mobile && nativeNoteEntrySupported()

	entry := newSearchEntry()
	entry.SetPlaceHolder("Say something about this passage…")
	noteText := func() string {
		if useNative {
			return nativeNoteEntryText()
		}
		return entry.Text
	}

	// A live count, because the cap is real: the note rides inside the link, and
	// a link a messenger truncates is a link that opens nothing.
	//
	// A WRAPPING RichText, not a canvas.Text: a canvas.Text never wraps, so the
	// idle line's ~357pt made it the whole card's minimum width — and on a
	// canvas narrower than that, the non-modal renderer grew the card past the
	// screen's right edge, taking the Share button with it (the implementation requirement).
	left := widget.NewRichText(&widget.TextSegment{
		Style: widget.RichTextStyle{ColorName: colorNameMuted, SizeName: theme.SizeNameCaptionText},
	})
	left.Wrapping = fyne.TextWrapWord
	updateLeft := func() {
		seg := left.Segments[0].(*widget.TextSegment)
		n := utf8.RuneCountInString(strings.TrimSpace(noteText()))
		switch {
		case n == 0:
			seg.Text = "Optional. The note travels inside the link — it is never uploaded."
			seg.Style.ColorName = colorNameMuted
		case n > NoteMaxRunes:
			seg.Text = "Too long by " + strconv.Itoa(n-NoteMaxRunes) + " — it will be shortened"
			seg.Style.ColorName = theme.ColorNameError
		default:
			seg.Text = strconv.Itoa(NoteMaxRunes-n) + " characters left"
			seg.Style.ColorName = colorNameMuted
		}
		left.Refresh()
	}
	entry.OnChanged = func(string) { updateLeft() }
	updateLeft()

	send := func() {
		note := strings.TrimSpace(noteText())
		closeSheet()
		shareVerseLinkWithNote(state, selectedText, note)
	}
	entry.OnSubmitted = func(string) { send() }

	sendBtn := widget.NewButton("Share", send)
	sendBtn.Importance = widget.HighImportance
	cancelBtn := widget.NewButton("Cancel", closeSheet)
	actions := container.NewBorder(nil, nil, nil, container.NewHBox(cancelBtn, sendBtn))

	// The field's slot in the form. With the native view in play the slot is a
	// transparent tracking widget — the UITextView is parked exactly over it and
	// FOLLOWS it through every relayout, so Fyne still owns the layout and the
	// native view just wears it.
	entrySlot := fyne.CanvasObject(inputFrame(withCaret(state, entry), pal.Border))
	if useNative {
		entrySlot = newNoteEntrySlot(&sheetOpen)
	}

	form := container.NewVBox(
		title, ref,
		widget.NewSeparator(),
		entrySlot,
		left,
		actions,
	)

	focusEntry := func() {
		if state.window != nil {
			state.window.Canvas().Focus(entry)
		}
	}

	if !mobile {
		card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		w := float32(460)
		if cw := cnv.Size().Width - 80; cw > 280 && w > cw {
			w = cw
		}
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		focusEntry()
		return
	}

	// Mobile: the same full-canvas, top-anchored, non-modal sheet promptAskQuestion
	// uses, and for the same reason — a centered modal puts the field under the
	// soft keyboard, and a full-canvas card means no tap lands "outside" it and
	// leaves the reading overlay latched hidden.
	body := container.NewVBox(form, layout.NewSpacer())
	card := surface(container.NewPadded(body), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewPopUp(card, cnv)
	cw, ch := cnv.Size().Width, cnv.Size().Height
	topY := float32(0)
	if pos, sz := cnv.InteractiveArea(); sz.Height > 0 {
		topY, ch = pos.Y, sz.Height
	}
	popup.Resize(fyne.NewSize(cw, ch))
	popup.ShowAtPosition(fyne.NewPos(0, topY))

	// A window rebuild (theme flip, rotation, a background data swap) drains
	// every popup WITHOUT running closeSheet — Hide() is all a drain does. For
	// the Fyne entry that was merely untidy; with a native field it would leave
	// an orphaned UITextView floating over whatever the rebuild painted. Poll
	// until the popup is gone by ANY route, then run the (idempotent) teardown —
	// the same 150ms watchdog the ask sheet uses.
	var watch func()
	watch = func() {
		if popup == nil || !popup.Visible() {
			closeSheet() // idempotent; also drops the slot latch
			return
		}
		time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watch) })
	}
	time.AfterFunc(150*time.Millisecond, func() { fyne.Do(watch) })

	if !useNative {
		focusEntry()
		return
	}

	// Native field: create it (keyboard comes up with it) and install the
	// counter hook. Placement is the slot widget's job from here — its
	// Resize/Move fired during the popup's layout pass above and keep firing on
	// every relayout. The view stays hidden until its first real frame arrives,
	// so mis-timing shows nothing rather than a box at (0,0).
	showNativeNoteEntry("", "Say something about this passage…", pal)
	noteEntryOnChanged = updateLeft
}

// showSharedNote presents a note that arrived on a link. It is deliberately a
// card the reader dismisses rather than something drawn into the scripture:
// the note is somebody's message about the passage, and it must never be
// mistaken for the passage. The attribution line is load-bearing — a message
// that could pass for the app's own voice would be a phishing surface.
func showSharedNote(state *AppState, note string) {
	if state == nil || state.window == nil {
		return
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeCard := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	who := canvas.NewText("Note from Friend", pal.TextMuted)
	who.TextSize = 11

	ref := canvas.NewText(state.CurrentBook+" "+strconv.Itoa(state.CurrentChapter), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	// A plain wrapped Label: the note is TEXT. Nothing here interprets markup,
	// and nothing in it becomes tappable.
	body := widget.NewLabel(note)
	body.Wrapping = fyne.TextWrapWord

	okBtn := widget.NewButton("Read the passage", closeCard)
	okBtn.Importance = widget.HighImportance

	// The note is the one thing here that grows — 280 runes of someone's
	// message, possibly with blank lines — so IT scrolls while the attribution
	// and the button stay fixed. The button matters most: this popup is modal
	// and "Read the passage" is its ONLY dismissal, so a note tall enough to
	// push it off a small phone stranded the reader with the reading overlay
	// latched hidden (the implementation requirement). squeezeWidthLayout for the same reason as
	// every scroll here: a bare VScroll widens content to its MinSize and clips
	// it sideways (sheet_fit.go).
	bodyScroll := container.NewVScroll(container.New(squeezeWidthLayout{}, body))
	form := container.NewBorder(
		container.NewVBox(who, ref, widget.NewSeparator()),
		container.NewBorder(nil, nil, nil, container.NewHBox(okBtn)),
		nil, nil,
		bodyScroll,
	)

	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()
	w := float32(460)
	if cw := cnv.Size().Width - 40; cw > 260 && w > cw {
		w = cw
	}
	fit := func() {
		pos, sz := cnv.InteractiveArea()
		maxH := sheetMaxHeight(cnv.Size().Height, pos.Y, sz.Height, pos.Y+16)
		h := scrollingSheetHeight(
			popup.MinSize().Height,
			bodyScroll.MinSize().Height,
			body.MinSize().Height,
			maxH,
		)
		popup.Resize(fyne.NewSize(w, h))
	}
	fit()
	fit() // wrap-accurate only after the first layout pass (the ai_settings lesson)
}

// shareNoteReference is the passage label on the compose sheet — the same
// citation the share itself will carry, so the writer can see what they are
// attaching the note to.
func shareNoteReference(state *AppState, selection string) string {
	if _, cite := prepareShareQuote(state, selection); cite != "" {
		return cite
	}
	return state.CurrentBook + " " + strconv.Itoa(state.CurrentChapter)
}
