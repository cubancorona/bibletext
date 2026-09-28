package bibletext

// Shared notes in the app: the sheet that writes one, and the card that shows
// one that arrived.
//
// Untagged on purpose — no build tag, no cgo — so the whole flow unit-tests on
// the host and the same code serves iPhone, iPad, macOS, Android, Windows and
// Linux. That matters more here than usual: the reading pane is a native
// overlay on three of those platforms, and a per-platform bubble would have
// been four implementations of the same interaction.
//
// Both surfaces observe the native-overlay invariant documented in
// ARCHITECTURE.md: the native reading view floats ABOVE the Fyne canvas, so
// anything Fyne draws must hide it on open and restore it on close, or it
// renders behind the scripture and looks like nothing happened.

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
// stale relayout could reposition a native view belonging to nothing. quiet
// holds its pushes while the sheet is refitted (settle).
type noteEntrySlot struct {
	widget.BaseWidget
	open  *bool
	quiet bool
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
	if s.open == nil || !*s.open || s.quiet {
		return
	}
	noteEntryFrameTo(s)
	noteSheetAfter(50*time.Millisecond, func() {
		if s.open != nil && *s.open {
			noteEntryFrameTo(s)
		}
	})
}

// settle runs relayout, which lays the sheet out again, with the slot's pushes
// held, then pushes the rect the slot ended at. The field is told where the
// slot settles and not the rects a relayout passes through on the way: the
// popup lays its content out before it moves it, so the first pass measures
// from the content's old position, and the excerpt's first pass at a new width
// reports the height of its rows at the old one. Each push reaches the view on
// the main queue, which may draw between two of them.
func (s *noteEntrySlot) settle(relayout func()) {
	s.quiet = true
	relayout()
	s.quiet = false
	s.push()
}

// The composer's platform seams, variables so a host test can lay out the
// sheet iOS gets and read what its native field would be told:
//
//   - noteEntryNative says whether this platform floats a native field over
//     the slot (iOS only; note_entry_ios.go);
//   - noteEntryFrameTo parks the native field over the slot, at the slot's
//     absolute rect;
//   - noteEntryTyped reads what the reader has typed into the native field;
//   - noteSheetAfter runs f on the UI goroutine after d: the slot's second
//     push and the phone sheet's watchdog. Under the test driver fyne.Do runs
//     a closure on the timer's own goroutine, so a test that opens the phone
//     sheet holds these and runs them itself (docs/BACKLOG.md, "Deferred-UI
//     timers under the test driver");
//   - noteSheetArea is the part of the canvas the phone sheet may cover, the
//     canvas's interactive area. On a phone that is the canvas less its safe
//     insets and, while the keyboard is up, less the keyboard too (the mobile
//     driver counts the keyboard as a bottom inset); the test driver has no
//     keyboard and small fixed insets of its own, so a test that needs a
//     device's insets or a keyboard sets them here.
var (
	noteEntryNative  = nativeNoteEntrySupported
	noteEntryFrameTo = setNativeNoteEntryFrameFromObject
	noteEntryTyped   = nativeNoteEntryText
	noteSheetAfter   = func(d time.Duration, f func()) {
		time.AfterFunc(d, func() { fyne.Do(f) })
	}
	noteSheetArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) { return c.InteractiveArea() }
)

// noteEntryOnChanged is installed by the compose sheet while it is open, and
// fired (via fyne.Do) by the native field's //export callback on every edit —
// it drives the live character counter. nil whenever no sheet is up.
var noteEntryOnChanged func()

// noteEntryOwner counts compose sheets, so the native field and its counter
// hook belong to the NEWEST one. There is only one native field (a second show
// replaces the first), and a composer drained by a rebuild runs its teardown
// from a watchdog up to 150ms later — by which time the light/dark reopen has
// already opened a new composer and put its own field up. Torn down
// unconditionally, the old sheet took the new sheet's field with it. So a
// sheet tears down only what it still owns. UI goroutine only.
var noteEntryOwner uint64

// promptShareNote collects an optional note, then shares the link carrying it.
// It is a SECOND verb beside "Share as link", never a step in front of it: the
// plain share stays one tap, because most shares carry no note and a modal in
// everyone's way to serve the minority is the wrong trade.
func promptShareNote(state *AppState, selectedText string, span selSpan) {
	if state == nil {
		return
	}
	promptShareNoteWith(state, selectedText, span, "", readerPassage(state))
}

// promptShareNoteWith opens the composer with note already in the field — ""
// for every ordinary open. The light/dark reopen passes what the reader had
// written when the rebuild drained the sheet (sheet_reopen.go). at is the
// passage the selection was made on, taken when the composer first opened and
// carried through a reopen, so a sent note is shown only there (showSentNote).
func promptShareNoteWith(state *AppState, selectedText string, span selSpan, note string, at notePassage) {
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
	noteEntryOwner++
	owner := noteEntryOwner
	closeSheet := func() {
		if closed {
			return
		}
		closed = true
		sheetOpen = false
		// Only this sheet's own field and hook (noteEntryOwner): a newer
		// composer — the reopen after a rebuild drained this one — has put up
		// its own by the time a watchdog runs this.
		if owner == noteEntryOwner {
			noteEntryOnChanged = nil
			hideNativeNoteEntry() // no-op off iOS, and when the Fyne entry was used
		}
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

	quote, cite := shareNoteQuote(state, selectedText, span, at)
	ref := canvas.NewText(cite, pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = subheadingTextSize

	// The selected words, as the share makes them, under the reference
	// (share_note_excerpt.go). It is built from the selection on every open,
	// the light/dark reopen's included, so it comes back with the sheet.
	excerpt := newNoteExcerpt(quote, noteExcerptMaxLinesFor(mobile, cnv.Size().Height), pal.TextMuted)
	if quote == "" {
		// A heading selected on its own quotes nothing: the share cites the
		// verse beneath it and carries no words (prepareShareQuote). The sheet
		// shows the reference alone, without an empty row's gap under it.
		excerpt.Hide()
	}

	// On iOS the field is a REAL UITextView floated over the sheet (dictation,
	// autocorrect, system selection, undo, VoiceOver, system emoji — see
	// note_entry_ios.go). Everywhere else it is the Fyne entry below. noteText
	// is the one place that knows which is live.
	useNative := mobile && noteEntryNative()

	entry := newSearchEntry()
	entry.SetPlaceHolder("Say something about this passage…")
	entry.SetText(note)
	noteText := func() string {
		if useNative {
			return noteEntryTyped()
		}
		return entry.Text
	}

	// A live count, because the cap is real: the note rides inside the link, and
	// a link a messenger truncates is a link that opens nothing.
	//
	// A WRAPPING RichText, not a canvas.Text: a canvas.Text never wraps, so the
	// idle line's ~357pt made it the whole card's minimum width — and on a
	// canvas narrower than that, the non-modal renderer grew the card past the
	// screen's right edge, taking the Share button with it.
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

	// Share closes the sheet, sends, and shows the note it kept on the
	// passage, the card the notes browser opens. The share sheet is handed
	// its message first: on an iPad and on a Mac it opens beside the
	// selection, and the note's card and the view's placement move the text
	// it would be measured against. The same closure serves the button and
	// Return, the Fyne field and iOS's native one, the desktop card and the
	// phone sheet. It shares and files the note against at, the passage the
	// words were selected on, as the excerpt above shows it: a link arriving
	// while the sheet is open moves the reader and leaves the sheet up.
	send := func() {
		note := strings.TrimSpace(noteText())
		closeSheet()
		if stored, kept := shareVerseLinkWithNote(state, selectedText, note, span, at); kept {
			showSentNote(state, stored, at)
		}
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
		excerpt,
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

	// A light/dark rebuild drains the composer; it comes back with the note as
	// written so far, read when the reopen runs — from the Fyne entry, which a
	// drained sheet still holds, or on iOS from the native field, which is
	// still up until the drained sheet's watchdog finds it no longer owns it
	// (noteEntryOwner). The new sheet takes the caret, as any open does.
	reopen := func() { promptShareNoteWith(state, selectedText, span, noteText(), at) }

	if !mobile {
		card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
		popup = widget.NewModalPopUp(card, cnv)
		popup.Show()
		w := float32(460)
		if cw := cnv.Size().Width - 80; cw > 280 && w > cw {
			w = cw
		}
		// Two passes. The card's height is its content's minimum, and the
		// excerpt and the counter wrap, so their minimum depends on the width
		// they are laid out at: the first pass lays the form out at the card's
		// width, the second takes the height that width gives. With one pass
		// the height was read from Show's layout at the form's narrow minimum
		// width, where both wrap into more rows, and the card stood taller
		// than its content by the difference.
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		popup.Resize(fyne.NewSize(w, card.MinSize().Height))
		registerSheetReopen(state, popup, reopen)
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
	if pos, sz := noteSheetArea(cnv); sz.Height > 0 {
		topY, ch = pos.Y, sz.Height
	}
	// What the card leaves uncovered at the canvas's foot as it opens: the
	// home indicator's inset, the keyboard not being up yet. A refit keeps it
	// (below).
	footGap := cnv.Size().Height - topY - ch
	// The card is the canvas's size whatever its content, so nothing is read
	// from the excerpt's height here. The popup lays the form out as it is
	// resized, where the excerpt wraps at its real width, and again as it is
	// shown, with the height those rows give: the slot, and on iOS the native
	// field parked over it, is below the last row before the sheet is on the
	// canvas (TestTheNoteFieldSlotSitsBelowTheExcerpt).
	popup.Resize(fyne.NewSize(cw, ch))
	popup.ShowAtPosition(fyne.NewPos(0, topY))
	registerSheetReopen(state, popup, reopen)

	// REFIT WHEN THE CANVAS CHANGES SIZE, as the Go to picker does (goto.go).
	// The toolkit never resizes a popup for a new canvas size; it only lays it
	// out again at the size it was given. Narrowing an iPad's Split View or
	// Slide Over window, or an Android window, with the composer open changes
	// neither the layout class nor the rail, so no rebuild drains the sheet:
	// the card, the excerpt wrapped for the old width and the slot kept their
	// old width, the native field was parked past the canvas's right edge,
	// and Share with it. The refit sizes the card for the canvas it now has,
	// which lays the form out again, so the excerpt re-wraps and the slot and
	// its native field follow, and gives the excerpt the row budget a sheet
	// opened on this canvas would take.
	//
	// The card's height comes from the canvas, less what it left at the foot
	// as it opened, and never from the interactive area again: that shrinks
	// while the keyboard is up, and the phone sheet is not resized for the
	// keyboard. A card fitted to it would end at the keyboard's top and stay
	// short, the page showing beneath it, once the keyboard went down.
	lastCanvas := cnv.Size()
	refit := func() {
		now := cnv.Size()
		relayout := func() {
			excerpt.setMaxLines(noteExcerptMaxLinesFor(true, now.Height))
			popup.Resize(fyne.NewSize(now.Width, now.Height-topY-footGap))
		}
		if slot, ok := entrySlot.(*noteEntrySlot); ok {
			slot.settle(relayout) // the native field is told only where the slot ends up
			return
		}
		relayout()
	}

	// A window rebuild (theme flip, rotation, a background data swap) drains
	// every popup WITHOUT running closeSheet — Hide() is all a drain does. For
	// the Fyne entry that was merely untidy; with a native field it would leave
	// an orphaned UITextView floating over whatever the rebuild painted. Poll
	// until the popup is gone by ANY route, then run the (idempotent) teardown —
	// the same 150ms watchdog the ask sheet uses. The same poll notices a new
	// canvas size and refits the sheet to it.
	var watch func()
	watch = func() {
		if popup == nil || !popup.Visible() {
			closeSheet() // idempotent; also drops the slot latch
			return
		}
		if now := cnv.Size(); now != lastCanvas {
			lastCanvas = now
			refit()
		}
		noteSheetAfter(150*time.Millisecond, watch)
	}
	noteSheetAfter(150*time.Millisecond, watch)

	if !useNative {
		focusEntry()
		return
	}

	// Native field: create it (keyboard comes up with it) and install the
	// counter hook. Placement is the slot widget's job from here — its
	// Resize/Move fired during the popup's layout pass above and keep firing on
	// every relayout. The view stays hidden until its first real frame arrives,
	// so mis-timing shows nothing rather than a box at (0,0).
	showNativeNoteEntry(note, "Say something about this passage…", pal)
	noteEntryOnChanged = updateLeft
}

// shareNoteReference is the passage label on the compose sheet for a
// selection on the reader's chapter — the same citation the share itself will
// carry, so the writer can see what they are attaching the note to.
func shareNoteReference(state *AppState, selection string, span selSpan) string {
	_, ref := shareNoteQuote(state, selection, span, readerPassage(state))
	return ref
}

// shareNoteQuote is what the compose sheet shows of the selection: the words
// and the reference, from ONE run of the share's own pipeline
// (prepareShareQuoteIn), so the excerpt, the reference and the citation the
// share sends cannot disagree. Both read the passage the composer recorded,
// at, so a light/dark reopen after the reader was moved still shows what
// Share will send. The words are the quote before its Bluebook
// framing — no quotation marks, bracketed capital or omission dots, which
// belong to a quotation standing on its own, not to a reminder under its
// reference — with the verse numbers stripped and the divine name in the small
// capitals the page draws.
func shareNoteQuote(state *AppState, selection string, span selSpan, at notePassage) (quote, ref string) {
	quote, ref, _, _ = prepareShareQuoteIn(state, at.book, at.chapter, selection, span)
	if ref == "" {
		ref = at.book + " " + strconv.Itoa(at.chapter)
	}
	return quote, ref
}
