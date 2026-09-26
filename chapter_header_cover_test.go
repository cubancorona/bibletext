package bibletext

// The open narration card is a pop-up. On a phone it lies over the
// next-chapter arrow and the full-screen button, and with a long heading over
// the copy icon and the heading's end as well; on a narrow desktop pane, or the
// Android fallback pane, it lies over the chapter block. It stays there until
// its ✕ closes it. These tests hold the open card to covering what it lies
// over — drawn above it, and claiming every tap across the card's rectangle —
// and to staying on the screen, ✕ included, so that it can be closed; and they
// hold the closed speaker to what it always was: in the same place, beneath
// its neighbours on a phone, and taking no tap but its own. (A small control
// whose glyph the open card touches is hidden rather than covered, so no part
// of it shows beside the card; chapter_header_hide_test.go holds that.)
//
// They lay out the REAL headers (chapterHeaderMobile, chapterHeader) in a test
// window and tap them through the canvas with test.TapCanvas, which resolves a
// tap with Fyne's own hit test (internal/driver FindObjectAtPositionMatching).
// That call asks one question, "which Tappable is here?"; the drivers ask four
// (a mobile tap-down, move and tap-up, and a desktop click), each with its own
// set of interfaces, so hitAt below replays the same walk with each of them.

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// coverBooks are the chapters the headers are laid out for: the longest
// headings and chapter lines in the canon (the widest chapter block, so the
// most the card lies over), a book of one chapter, a chapter with both arrows
// live, and an ordinary first chapter.
var coverBooks = []struct {
	book              string
	chapter, chapters int
}{
	{"Deuteronomy", 34, 34},
	{"Song of Solomon", 8, 8},
	{"2 Thessalonians", 3, 3},
	{"Psalms", 119, 150},
	{"Jude", 1, 1},
	{"John", 1, 21},
}

// phoneCanvasWidths are canvas widths in Fyne units. The mobile driver divides
// a screen's pixels by a scale it buckets by density: the iPhones meant here
// get a canvas their width in points (320 the first iPhone SE, 375 the later
// one, 390, 393 and 402 current phones), and a 1080-pixel Android phone at
// 420 dpi, a Pixel 7 among them, gets one 360 wide, where Android itself
// counts 411dp. 700 and 1024 are tablets', where the same header has room for
// the card.
var phoneCanvasWidths = []float32{320, 360, 375, 390, 393, 402, 700, 1024}

// phoneTestApp is the test app with a phone for its device. The closed audio
// control is arranged differently on a touch device (audioControlContent asks
// fyne.CurrentDevice().IsMobile()), and the test driver's device is a desktop
// with no setter, so the phone header is laid out under this wrapper to get
// the arrangement a phone gets.
type phoneTestApp struct{ fyne.App }

func (a phoneTestApp) Driver() fyne.Driver { return phoneTestDriver{a.App.Driver()} }

type phoneTestDriver struct{ fyne.Driver }

func (d phoneTestDriver) Device() fyne.Device { return phoneTestDevice{d.Driver.Device()} }

type phoneTestDevice struct{ fyne.Device }

func (phoneTestDevice) IsMobile() bool { return true }

// coverApp starts a test app in the reader's own theme, so the headers are
// measured in the typefaces they ship with, with read-aloud available, so every
// chapter has an audio control as on the phones (Windows and Linux have none
// for a chapter without a recording). phone makes the device a phone.
func coverApp(t *testing.T, phone bool) *bibleTheme {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	if phone {
		fyne.SetCurrentApp(phoneTestApp{app})
	}
	savedTTS, savedOpen := ttsSupported, audioPanelOpen
	ttsSupported = func() bool { return true }
	t.Cleanup(func() {
		ttsSupported, audioPanelOpen = savedTTS, savedOpen
		gAudio.setOnChange(nil)
	})
	return th
}

// coverState is a reader on book's chapter, in a Bible holding one verse in
// each of the book's chapters: all the header reads, and all the chapter
// arrows and the copy button need to act.
func coverState(th *bibleTheme, book string, chapter, chapters int) (*AppState, []int) {
	verses := make(map[int][]Verse, chapters)
	nums := make([]int, 0, chapters)
	for c := 1; c <= chapters; c++ {
		verses[c] = []Verse{{BookName: book, Chapter: c, Verse: 1, Text: "In the beginning was the Word."}}
		nums = append(nums, c)
	}
	st := &AppState{
		Bible:          &BibleData{Books: []string{book}, Verses: map[string]map[int][]Verse{book: verses}},
		CurrentBook:    book,
		CurrentChapter: chapter,
		theme:          th,
	}
	return st, nums
}

// phonePage places a phone header as both native reading panes do
// (reading_ios.go, reading_android.go): inset one pad on the left, in a body
// padded one pad on the right, in a window that pads its canvas, so the header
// gets the canvas less four pads, as on the device.
func phonePage(header fyne.CanvasObject) fyne.CanvasObject {
	pad := theme.Padding()
	band := container.New(layout.NewCustomPaddedLayout(0, 0, pad, 0), container.NewVBox(header))
	body := container.NewBorder(band, nil, nil, nil, canvas.NewRectangle(color.Transparent))
	return container.New(layout.NewCustomPaddedLayout(0, pad, 0, pad), body)
}

// toolbarPage places the desktop toolbar as buildReadingView and the Android
// fallback pane (reading_mobile.go) do: padded, at the top of the page.
func toolbarPage(header fyne.CanvasObject) fyne.CanvasObject {
	return container.NewPadded(container.NewBorder(container.NewVBox(header), nil, nil, nil,
		canvas.NewRectangle(color.Transparent)))
}

const coverClipboard = "nothing copied"

// coverTestWindow keeps one clipboard, so a copy can be seen: the test
// window hands out a fresh one on every call.
type coverTestWindow struct {
	fyne.Window
	clip *fakeClipboard
}

func (w coverTestWindow) Clipboard() fyne.Clipboard { return w.clip }

// coverWindow shows page in a window width wide, and makes it the reader's
// window, so the header's copy button and chapter picker act on it.
func coverWindow(t *testing.T, st *AppState, page fyne.CanvasObject, width float32) fyne.Window {
	t.Helper()
	win := coverTestWindow{Window: test.NewTempWindow(t, page), clip: &fakeClipboard{content: coverClipboard}}
	win.Resize(fyne.NewSize(width, 640))
	st.window = win
	// The size probe audioControl measures and never shows keeps its
	// renderers in Fyne's cache after the test, and through its state the
	// whole window: let the state go of the window when the test ends.
	t.Cleanup(func() { st.window = nil })
	return win
}

// showPage makes page the window's content at width. The test canvas grows
// to its content's minimum size on SetContent, where a phone's screen stays
// the width it is, and the open card's reserved cell is wider than a phone's
// header has room for, so without this the page would be laid out wider than
// the phone.
func showPage(win fyne.Window, page fyne.CanvasObject, width float32) {
	win.SetContent(page)
	win.Resize(fyne.NewSize(width, 640))
}

// coverWatch sees what a tap did to the reader: moved the chapter (an arrow),
// entered full screen (the full-screen or focus button), opened the chapter
// picker (the heading or the chapter line), or copied the chapter.
type coverWatch struct {
	st      *AppState
	win     fyne.Window
	chapter int
	pickers int
}

// pickerStopped is what the watch's hideReadingOverlay panics with, to stop
// the chapter picker before it is built: gotoPickerModal hides the reading
// overlay first, before it builds anything (goto.go). The sweeps reach the
// picker hundreds of times, and a built one leaves its chapter grid's
// renderers in Fyne's cache, which only a painting canvas cleans — about 90
// MB a run. (The other modals that hide the overlay first are opened by
// nothing a test here taps.)
type pickerStopped struct{}

func watchCover(st *AppState, win fyne.Window) *coverWatch {
	w := &coverWatch{st: st, win: win, chapter: st.CurrentChapter}
	st.hideReadingOverlay = func() { panic(pickerStopped{}) }
	return w
}

// tap taps the canvas at p, with Fyne's own hit test, and counts a tap that
// reached the chapter picker.
func (w *coverWatch) tap(p fyne.Position) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(pickerStopped); !ok {
				panic(r)
			}
			w.pickers++
		}
	}()
	test.TapCanvas(w.win.Canvas(), p)
}

// reached names every header control the taps since the last call reached,
// and puts the reader back as it was.
func (w *coverWatch) reached() []string {
	var got []string
	if w.st.CurrentChapter != w.chapter {
		got = append(got, fmt.Sprintf("a chapter arrow (chapter %d became %d)", w.chapter, w.st.CurrentChapter))
		w.st.CurrentChapter = w.chapter
	}
	if w.st.IsFullScreen {
		got = append(got, "the full-screen button")
		w.st.IsFullScreen = false
	}
	if w.pickers > 0 {
		got = append(got, "the chapter picker")
		w.pickers = 0
	}
	for top := w.win.Canvas().Overlays().Top(); top != nil; top = w.win.Canvas().Overlays().Top() {
		got = append(got, fmt.Sprintf("a pop-up (%T)", top))
		top.Hide()
		w.win.Canvas().Overlays().Remove(top)
	}
	if w.win.Clipboard().Content() != coverClipboard {
		got = append(got, "the copy button")
		w.win.Clipboard().SetContent(coverClipboard)
	}
	return got
}

// drawnObject is one visible object in the order Fyne draws the tree: a
// container before its children, children in slice order, a widget's renderer
// objects as its children. Fyne's painter and its hit test (internal/driver
// walkObjectTree) walk in this order, and the hit test gives an event to the
// LAST matching object under the point. The pages here hold no clipping
// container, so there is no clip rectangle to carry.
type drawnObject struct {
	obj   fyne.CanvasObject
	pos   fyne.Position // on the canvas
	order int
	path  []fyne.CanvasObject // obj's ancestors, the canvas content first
}

func drawnTree(t *testing.T, root fyne.CanvasObject) []drawnObject {
	t.Helper()
	var out []drawnObject
	var walk func(o fyne.CanvasObject, offset fyne.Position, path []fyne.CanvasObject)
	walk = func(o fyne.CanvasObject, offset fyne.Position, path []fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		pos := o.Position().Add(offset)
		out = append(out, drawnObject{obj: o, pos: pos, order: len(out), path: path})
		var children []fyne.CanvasObject
		switch v := o.(type) {
		case *fyne.Container:
			children = v.Objects
		case fyne.Widget:
			// The renderer the canvas made goes when the test does.
			children = test.TempWidgetRenderer(t, v).Objects()
		}
		below := append(append([]fyne.CanvasObject(nil), path...), o)
		for _, c := range children {
			walk(c, pos, below)
		}
	}
	walk(root, fyne.Position{}, nil)
	return out
}

func (d drawnObject) contains(p fyne.Position) bool {
	s := d.obj.Size()
	return p.X >= d.pos.X && p.Y >= d.pos.Y && p.X < d.pos.X+s.Width && p.Y < d.pos.Y+s.Height
}

func (d drawnObject) overlaps(o drawnObject) bool {
	a, b := d.obj.Size(), o.obj.Size()
	return d.pos.X < o.pos.X+b.Width && o.pos.X < d.pos.X+a.Width &&
		d.pos.Y < o.pos.Y+b.Height && o.pos.Y < d.pos.Y+a.Height
}

// under reports whether d is root or lies inside it.
func (d drawnObject) under(root fyne.CanvasObject) bool {
	if d.obj == root {
		return true
	}
	for _, a := range d.path {
		if a == root {
			return true
		}
	}
	return false
}

// drawsSomething leaves out what paints nothing and takes no event of its
// own: containers and spacers.
func (d drawnObject) drawsSomething() bool {
	switch d.obj.(type) {
	case *fyne.Container, *layout.Spacer:
		return false
	}
	return true
}

func (d drawnObject) String() string {
	s := d.obj.Size()
	name := fmt.Sprintf("%T", d.obj)
	switch v := d.obj.(type) {
	case *iconTapButton:
		name = "icon button " + v.icon.Name()
	case *tapText:
		name = fmt.Sprintf("tap text %q", v.text)
	case *referenceButton:
		name = fmt.Sprintf("heading %q", v.text)
	case *widget.Button:
		name = "button " + v.Icon.Name()
	case *coverButton:
		name = "button " + v.Icon.Name()
	}
	return fmt.Sprintf("%s at %.0f,%.0f %.0fx%.0f", name, d.pos.X, d.pos.Y, s.Width, s.Height)
}

// driverMatchers are the questions Fyne's drivers ask of the object under a
// point: internal/driver/mobile/canvas.go (tapDown, tapMove, tapUp) and
// internal/driver/glfw/window.go (mouseClicked).
var driverMatchers = []struct {
	event string
	match func(fyne.CanvasObject) bool
}{
	{"a touch down", func(o fyne.CanvasObject) bool {
		switch o.(type) {
		case mobile.Touchable, fyne.Focusable:
			return true
		}
		return false
	}},
	{"a touch move", func(o fyne.CanvasObject) bool {
		switch o.(type) {
		case fyne.Draggable, mobile.Touchable:
			return true
		}
		return false
	}},
	{"a touch up (tap, long press, double tap)", func(o fyne.CanvasObject) bool {
		switch o.(type) {
		case fyne.Tappable, fyne.SecondaryTappable, mobile.Touchable, fyne.DoubleTappable:
			return true
		}
		return false
	}},
	{"a mouse click", func(o fyne.CanvasObject) bool {
		switch o.(type) {
		case fyne.Tappable, fyne.SecondaryTappable, fyne.DoubleTappable, fyne.Focusable, desktop.Mouseable:
			return true
		}
		return false
	}},
}

// hitAt is the object a driver hands an event at p to: the last object in
// draw order under p that matches.
func hitAt(tree []drawnObject, p fyne.Position, match func(fyne.CanvasObject) bool) (drawnObject, bool) {
	var found drawnObject
	ok := false
	for _, d := range tree {
		if d.contains(p) && match(d.obj) {
			found, ok = d, true
		}
	}
	return found, ok
}

func findDrawn(tree []drawnObject, match func(fyne.CanvasObject) bool) []drawnObject {
	var out []drawnObject
	for _, d := range tree {
		if match(d.obj) {
			out = append(out, d)
		}
	}
	return out
}

func oneDrawn(t *testing.T, tree []drawnObject, what string, match func(fyne.CanvasObject) bool) drawnObject {
	t.Helper()
	got := findDrawn(tree, match)
	if len(got) != 1 {
		t.Fatalf("found %d of %s in the header, want exactly one", len(got), what)
	}
	return got[0]
}

func isIconButton(icon fyne.Resource) func(fyne.CanvasObject) bool {
	return func(o fyne.CanvasObject) bool {
		b, ok := o.(*iconTapButton)
		return ok && b.icon.Name() == icon.Name()
	}
}

func isType[T fyne.CanvasObject](o fyne.CanvasObject) bool {
	_, ok := o.(T)
	return ok
}

// branchBelow finds fork, the lowest object holding both a and b, and branch,
// fork's child on the way to a. For the speaker and the full-screen button
// they are the header row and the part of it holding the audio control; for
// the card's source chip and its ✕, fork is the card.
func branchBelow(tree []drawnObject, a, b drawnObject) (fork, branch drawnObject) {
	pa := append(append([]fyne.CanvasObject(nil), a.path...), a.obj)
	pb := append(append([]fyne.CanvasObject(nil), b.path...), b.obj)
	n := 0
	for n < len(pa) && n < len(pb) && pa[n] == pb[n] {
		n++
	}
	for _, d := range tree {
		if d.obj == pa[n-1] {
			fork = d
		}
		if n < len(pa) && d.obj == pa[n] {
			branch = d
		}
	}
	return fork, branch
}

// openCard finds the open card: the object holding both its source chip and
// its ✕, which is buildAudioCard's stack, and the host the control swaps it
// into. It is found by what the card shows rather than by its shield, so a
// card built without one is found all the same.
func openCard(t *testing.T, tree []drawnObject) (card drawnObject, host *fyne.Container) {
	t.Helper()
	chip := oneDrawn(t, tree, "the card's source chip", isType[*labeledTapChip])
	closeX := oneDrawn(t, tree, "the card's ✕", isType[*tappableArea])
	card, _ = branchBelow(tree, chip, closeX)
	host, ok := card.path[len(card.path)-1].(*fyne.Container)
	if !ok {
		t.Fatalf("the open card's parent is a %T, not the control's host", card.path[len(card.path)-1])
	}
	return card, host
}

// cardTaps records the card's own controls' taps, so they can be tapped
// without starting narration, opening the source menu or closing the card.
type cardTaps struct{ fired []string }

func (c *cardTaps) callbacks() audioCardCallbacks {
	rec := func(name string) func() { return func() { c.fired = append(c.fired, name) } }
	return audioCardCallbacks{onSrc: rec("source"), onBack: rec("skip back"), onPlay: rec("play"),
		onFwd: rec("skip forward"), onClose: rec("close")}
}

func (c *cardTaps) take() []string {
	got := c.fired
	c.fired = nil
	return got
}

// swapInCard puts a card with recorded callbacks into the open control's host,
// the way the control's own expand path puts the real one there (a host.Objects
// swap, no relayout of the header), and returns the tree as it now stands.
// The builder is the one the control calls, so the card is the real card in
// every respect but where its taps go.
func swapInCard(t *testing.T, win fyne.Window, st *AppState, kind audioKind, taps *cardTaps) []drawnObject {
	t.Helper()
	_, host := openCard(t, drawnTree(t, win.Canvas().Content()))
	host.Objects = []fyne.CanvasObject{buildAudioCard(st, kind, false, false, false, kind == audioRecorded, taps.callbacks())}
	host.Refresh()
	return drawnTree(t, win.Canvas().Content())
}

// gridOver is every point of a dense grid across a rectangle, with its edges:
// the rounded corners, the padding, the gaps between the controls.
func gridOver(pos fyne.Position, size fyne.Size) []fyne.Position {
	const step = 4
	axis := func(from, length float32) []float32 {
		var out []float32
		for v := from; v < from+length; v += step {
			out = append(out, v)
		}
		return append(out, from+length-0.25)
	}
	var pts []fyne.Position
	for _, y := range axis(pos.Y, size.Height) {
		for _, x := range axis(pos.X, size.Width) {
			pts = append(pts, fyne.NewPos(x, y))
		}
	}
	return pts
}

// onCanvas reports whether p is on the canvas, where a finger can reach it.
// test.TapCanvas does not ask: it hands a tap at a point past the canvas's edge
// to whatever is laid out there all the same.
func onCanvas(win fyne.Window, p fyne.Position) bool {
	s := win.Canvas().Size()
	return p.X >= 0 && p.Y >= 0 && p.X < s.Width && p.Y < s.Height
}

// drawnAs finds obj in the tree.
func drawnAs(t *testing.T, tree []drawnObject, obj fyne.CanvasObject) drawnObject {
	t.Helper()
	for _, d := range tree {
		if d.obj == obj {
			return d
		}
	}
	t.Fatalf("%T is not drawn in the header", obj)
	return drawnObject{}
}

// near compares two layout coordinates, which float arithmetic may leave a
// hair apart.
func near(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }

// phoneAudioCell checks where the phone header puts its audio control: the
// cell reserved at the open card's size, centred on the gap the row's
// BorderLayout leaves between the chapter block and the full-screen button,
// exactly where container.NewCenter put it — save that an open card which
// would run past the header's right edge is moved left until its right edge
// is the header's, and no further. inner is the speaker or the open card. It
// returns the cell, and whether the card had to be moved. (The row is found
// through the heading, which the open card never hides. The row's part that
// holds the control is one of its two places for it, which holds the gap,
// which holds the cell.)
func phoneAudioCell(t *testing.T, tree []drawnObject, inner drawnObject, open bool) (cell drawnObject, moved bool) {
	t.Helper()
	heading := oneDrawn(t, tree, "the heading", isType[*referenceButton])
	row, place := branchBelow(tree, inner, heading)
	holds := func(d drawnObject, what string) drawnObject {
		c, ok := d.obj.(*fyne.Container)
		if !ok || len(c.Objects) != 1 {
			t.Fatalf("%s is %s, not a container holding one object", what, d)
		}
		return drawnAs(t, tree, c.Objects[0])
	}
	part := holds(place, "the header row's place for the audio control")
	if part.pos != place.pos || part.obj.Size() != place.obj.Size() {
		t.Errorf("the audio control's gap (%s) does not fill its place in the row (%s)", part, place)
	}
	cell = holds(part, "the audio control's gap")
	cs, gs := cell.obj.Size(), part.obj.Size()
	want := fyne.NewPos(part.pos.X+(gs.Width-cs.Width)/2, part.pos.Y+(gs.Height-cs.Height)/2)
	edge := row.pos.X + row.obj.Size().Width
	if open && want.X+cs.Width > edge {
		want.X, moved = edge-cs.Width, true
	}
	if !near(cell.pos.X, want.X) || !near(cell.pos.Y, want.Y) {
		t.Errorf("the audio control's cell is at %.2f,%.2f, want %.2f,%.2f (card open %v; gap %s; header's right edge %.2f)",
			cell.pos.X, cell.pos.Y, want.X, want.Y, open, part, edge)
	}
	if open {
		if inner.pos != cell.pos || inner.obj.Size() != cs {
			t.Errorf("the open card (%s) does not fill its cell (%s)", inner, cell)
		}
		if inner.pos.X < row.pos.X || inner.pos.X+cs.Width > edge+0.01 {
			t.Errorf("the open card (%s) lies outside the header row (%s)", inner, row)
		}
	}
	return cell, moved
}

// hitName names what hitAt found: d, or nothing.
func hitName(d drawnObject, ok bool) string {
	if !ok {
		return "nothing"
	}
	return d.String()
}

// assertClosedControlTakesOnlyTheSpeaker holds the closed audio control to
// taking no event but its speaker's. Its cell is reserved at the open card's
// size, and wherever the cell overlaps a neighbour a stray tappable object in
// it would take that neighbour's taps. So at every point of the cell each
// driver must hand its event to the object it would hand it to if the cell
// held nothing but the speaker: the speaker where it is on top, and otherwise
// the neighbour, or nothing.
func assertClosedControlTakesOnlyTheSpeaker(t *testing.T, tree []drawnObject, cell, speaker drawnObject) {
	t.Helper()
	var bare []drawnObject
	for _, d := range tree {
		if d.under(cell.obj) && !d.under(speaker.obj) {
			continue
		}
		bare = append(bare, d)
	}
	missed := 0
	for _, p := range gridOver(cell.pos, cell.obj.Size()) {
		for _, m := range driverMatchers {
			got, ok := hitAt(tree, p, m.match)
			want, wantOK := hitAt(bare, p, m.match)
			if ok == wantOK && (!ok || got.obj == want.obj) {
				continue
			}
			if missed++; missed <= 3 {
				t.Errorf("with the card closed, %s at %.1f,%.1f goes to %s, where the speaker alone would leave it to %s",
					m.event, p.X, p.Y, hitName(got, ok), hitName(want, wantOK))
			}
		}
	}
	if missed > 3 {
		t.Errorf("... %d misses in all across the closed control's cell", missed)
	}
}

// assertCardCovers is the check the open card must pass wherever it is drawn:
//
//   - it lies wholly on the canvas, so its ✕ and its transport can be reached;
//   - it is drawn after everything it lies over, so nothing is painted on it;
//   - its first paint is its frame, spanning the card and opaque, so nothing
//     beneath shows through it;
//   - its shield spans exactly the card;
//   - every driver hands an event anywhere in the card's rectangle to the card
//     (to its own control, or to the shield), never to what lies beneath;
//   - a tap anywhere on it but its own controls does nothing at all, to the
//     reader or to the card.
//
// It returns the objects outside the card that the card lies over.
func assertCardCovers(t *testing.T, win fyne.Window, tree []drawnObject, watch *coverWatch, taps *cardTaps) []drawnObject {
	t.Helper()
	card, _ := openCard(t, tree)
	cs := card.obj.Size()

	if w := win.Canvas().Size(); !onCanvas(win, card.pos) || card.pos.X+cs.Width > w.Width || card.pos.Y+cs.Height > w.Height {
		t.Errorf("the open card (%s) runs off the %.0f-wide canvas, where no finger can reach what it holds", card, w.Width)
	}

	var covered, above []drawnObject
	for _, d := range tree {
		if d.under(card.obj) || !d.drawsSomething() || !d.overlaps(card) {
			continue
		}
		covered = append(covered, d)
		if d.order > card.order {
			above = append(above, d)
		}
	}
	for i, d := range above {
		if i == 3 {
			t.Errorf("... and %d more drawn over the open card", len(above)-3)
			break
		}
		t.Errorf("%s is drawn over the open card (%s)", d, card)
	}

	var frame *canvas.Rectangle
	for _, d := range tree {
		if !d.under(card.obj) || d.obj == card.obj || !d.drawsSomething() || isType[*tapShield](d.obj) {
			continue
		}
		r, ok := d.obj.(*canvas.Rectangle)
		if !ok || d.pos != card.pos || d.obj.Size() != cs {
			t.Errorf("the card's first paint is %s, not a frame spanning the card (%s)", d, card)
		} else {
			frame = r
		}
		break
	}
	if frame != nil {
		if _, _, _, a := frame.FillColor.RGBA(); a != 0xffff {
			t.Errorf("the card's frame is not opaque (alpha %d of 65535): covered controls show through it", a)
		}
	}

	shields := findDrawn(tree, isType[*tapShield])
	switch {
	case len(shields) != 1:
		t.Errorf("found %d tap shields in the header with the card open, want one", len(shields))
	case !shields[0].under(card.obj) || shields[0].pos != card.pos || shields[0].obj.Size() != cs:
		t.Errorf("the tap shield is %s, not the card's own rectangle %s", shields[0], card)
	}

	missed := 0
	for _, p := range gridOver(card.pos, cs) {
		for _, m := range driverMatchers {
			got, ok := hitAt(tree, p, m.match)
			if ok && got.under(card.obj) {
				continue
			}
			if missed++; missed <= 3 {
				if ok {
					t.Errorf("%s at %.1f,%.1f on the open card goes to %s, which the card covers", m.event, p.X, p.Y, got)
				} else {
					t.Errorf("%s at %.1f,%.1f on the open card goes to nothing: the card does not claim it", m.event, p.X, p.Y)
				}
			}
		}
		// A tap on one of the card's own controls is the card's; those are
		// tapped by name below. Everywhere else, tap through the canvas.
		if got, ok := hitAt(tree, p, driverMatchers[2].match); ok && got.under(card.obj) && !isType[*tapShield](got.obj) {
			continue
		}
		watch.tap(p)
		reached, fired := watch.reached(), taps.take()
		if len(reached) == 0 && len(fired) == 0 {
			continue
		}
		if missed++; missed <= 3 {
			t.Errorf("a tap at %.1f,%.1f on the open card's background reached %s",
				p.X, p.Y, strings.Join(append(reached, fired...), " and "))
		}
	}
	if missed > 3 {
		t.Errorf("... %d misses in all across the open card", missed)
	}
	return covered
}

// assertCardControlsWork taps each of the open card's own controls at its
// centre and wants exactly that control's callback, and nothing beneath. The
// centre must be on the canvas: a control past the screen's edge does not work,
// whatever a tap test.TapCanvas delivers there does.
func assertCardControlsWork(t *testing.T, win fyne.Window, tree []drawnObject, watch *coverWatch, taps *cardTaps, canSeek bool) {
	t.Helper()
	card, _ := openCard(t, tree)
	inCard := func(m func(fyne.CanvasObject) bool) func(fyne.CanvasObject) bool {
		return func(o fyne.CanvasObject) bool {
			for _, d := range tree {
				if d.obj == o {
					return m(o) && d.under(card.obj)
				}
			}
			return false
		}
	}
	controls := []struct {
		name  string
		match func(fyne.CanvasObject) bool
		fires bool
	}{
		{"play", isIconButton(theme.MediaPlayIcon()), true},
		{"skip back", isIconButton(iconSkipBack15), canSeek},
		{"skip forward", isIconButton(iconSkipFwd15), canSeek},
		{"source", isType[*labeledTapChip], true},
		{"close", isType[*tappableArea], true},
	}
	for _, c := range controls {
		d := oneDrawn(t, tree, "the card's "+c.name, inCard(c.match))
		s := d.obj.Size()
		centre := d.pos.Add(fyne.NewPos(s.Width/2, s.Height/2))
		if !onCanvas(win, centre) {
			t.Errorf("the centre of the card's %s, at %.1f,%.1f, is off the %.0f-wide canvas", c.name, centre.X, centre.Y, win.Canvas().Size().Width)
			continue
		}
		watch.tap(centre)
		want := []string{}
		if c.fires {
			want = []string{c.name}
		}
		fired := taps.take()
		if fmt.Sprint(fired) != fmt.Sprint(want) {
			t.Errorf("a tap at the centre of the card's %s fired %v, want %v", c.name, fired, want)
		}
		if reached := watch.reached(); len(reached) > 0 {
			t.Errorf("a tap at the centre of the card's %s reached %s beneath the card", c.name, strings.Join(reached, " and "))
		}
	}
}

// With the card open, the phone header is as tall as with it closed, the card
// sits where the speaker's cell is centred unless that would run it past the
// header's right edge, it lies wholly on the screen, it is drawn over
// everything it lies over, and it takes every tap across its rectangle: its
// own controls, the ✕ first among them, still work, and nothing it covers — the
// heading, the chapter line, the edge of a control's tap box — can be reached
// through it, by any of the events the drivers deliver; and each control whose
// glyph it touches is hidden (chapter_header_hide_test.go holds that across
// the canon). The header is built with the card already open, as it is when
// the reader moves to another chapter with the card open. The card the header
// built is checked as it stands, and then cards with recorded callbacks are
// swapped in so that their controls can be tapped.
func TestOpenPhoneCardCoversWhatItLiesOver(t *testing.T) {
	th := coverApp(t, true)
	coveredSomewhere := map[string]bool{}
	hiddenSomewhere := map[string]bool{}
	var moved, stayed int
	for _, b := range coverBooks {
		for _, w := range phoneCanvasWidths {
			t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
				st, nums := coverState(th, b.book, b.chapter, b.chapters)
				audioPanelOpen = false
				closedHeader := chapterHeaderMobile(st, nums)
				win := coverWindow(t, st, phonePage(closedHeader), w)
				drawnTree(t, win.Canvas().Content())
				closedH := closedHeader.MinSize().Height

				audioPanelOpen = true
				header := chapterHeaderMobile(st, nums)
				showPage(win, phonePage(header), w)
				if h := header.MinSize().Height; h != closedH {
					t.Errorf("the header is %.1f tall with the card open and %.1f with it closed", h, closedH)
				}
				if h := header.Size().Height; h != closedH {
					t.Errorf("the open header is laid out %.1f tall, the closed one needs %.1f", h, closedH)
				}

				watch := watchCover(st, win)
				view := readHeader(t, newLayoutWalker(t), win.Canvas().Content(), phoneHeaderSurface)
				for _, c := range assertCoverage(t, view, nil, nil) {
					hiddenSomewhere[c] = true
				}
				tree := drawnTree(t, win.Canvas().Content())
				card, _ := openCard(t, tree)
				if _, m := phoneAudioCell(t, tree, card, true); m {
					moved++
				} else {
					stayed++
				}
				assertCardCovers(t, win, tree, watch, &cardTaps{})

				for _, k := range []struct {
					kind    audioKind
					canSeek bool
				}{{audioTTS, false}, {audioRecorded, true}} {
					taps := &cardTaps{}
					tree := swapInCard(t, win, st, k.kind, taps)
					for _, d := range assertCardCovers(t, win, tree, watch, taps) {
						if isType[*referenceButton](d.obj) {
							coveredSomewhere["heading"] = true
						}
					}
					assertCardControlsWork(t, win, tree, watch, taps, k.canSeek)
				}
			})
		}
	}
	// The sweep must include the cases the card exists for: hiding the
	// next-chapter arrow and the full-screen button, whose glyphs it touches,
	// and lying over the heading, which it covers and never hides.
	for _, want := range []string{"next arrow", "full-screen button"} {
		if !hiddenSomewhere[want] {
			t.Errorf("at no width does the open card hide the %s, so the checks above tested no hiding (hidden: %v)", want, hiddenSomewhere)
		}
	}
	if !coveredSomewhere["heading"] {
		t.Errorf("at no width does the open card lie over the heading, so the checks above tested no covering")
	}
	// And both places: a card centred where the speaker is, and one moved in
	// from past the header's edge.
	if moved == 0 || stayed == 0 {
		t.Errorf("the open card was moved in from the header's edge in %d layouts and left centred in %d: the sweep must reach both", moved, stayed)
	}
}

// With the card closed, the phone header is what it always was. The speaker's
// cell is centred on the gap; the speaker is drawn beneath its neighbours, so
// where its box reaches under the copy icon or the full-screen button (on the
// narrow phones) those controls keep the tap; no shield exists, and nothing in
// the cell but the speaker takes an event; and a tap at the centre of each of
// the chapter arrows, the full-screen button, the chapter line and the heading
// reaches that control and does what it does.
func TestClosedPhoneSpeakerStaysBeneathItsNeighbours(t *testing.T) {
	th := coverApp(t, true)
	for _, b := range coverBooks {
		for _, w := range phoneCanvasWidths {
			t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
				st, nums := coverState(th, b.book, b.chapter, b.chapters)
				audioPanelOpen = false
				win := coverWindow(t, st, phonePage(chapterHeaderMobile(st, nums)), w)
				tree := drawnTree(t, win.Canvas().Content())
				watch := watchCover(st, win)

				if n := len(findDrawn(tree, isType[*tapShield])); n != 0 {
					t.Errorf("found %d tap shields with the card closed: the speaker's cell would block the controls around it", n)
				}
				speaker := oneDrawn(t, tree, "the speaker", isIconButton(theme.VolumeUpIcon()))
				full := oneDrawn(t, tree, "the full-screen button", isType[*coverButton])
				cell, _ := phoneAudioCell(t, tree, speaker, false)
				assertClosedControlTakesOnlyTheSpeaker(t, tree, cell, speaker)
				_, audio := branchBelow(tree, speaker, full)
				last := audio.order
				for _, d := range tree {
					if d.under(audio.obj) && d.order > last {
						last = d.order
					}
				}
				for _, d := range tree {
					if !d.under(audio.obj) && d.drawsSomething() && d.overlaps(speaker) && d.order < last {
						t.Errorf("%s is drawn beneath the closed speaker (%s), which it used to be drawn over", d, speaker)
					}
				}

				idx := indexOf(nums, st.CurrentChapter)
				targets := []struct {
					name  string
					match func(fyne.CanvasObject) bool
					want  string
				}{
					{"previous arrow", isIconButton(theme.NavigateBackIcon()), "a chapter arrow"},
					{"next arrow", isIconButton(theme.NavigateNextIcon()), "a chapter arrow"},
					{"full-screen button", isType[*coverButton], "the full-screen button"},
					{"chapter line", func(o fyne.CanvasObject) bool {
						l, ok := o.(*tapText)
						return ok && strings.HasPrefix(l.text, "Chapter")
					}, "the chapter picker"},
					{"heading", isType[*referenceButton], "the chapter picker"},
				}
				for _, tg := range targets {
					d := oneDrawn(t, tree, "the "+tg.name, tg.match)
					s := d.obj.Size()
					centre := d.pos.Add(fyne.NewPos(s.Width/2, s.Height/2))
					for _, m := range driverMatchers[2:] {
						if got, ok := hitAt(tree, centre, m.match); !ok || got.obj != d.obj {
							t.Errorf("%s at the centre of the %s goes to %s, not to it", m.event, tg.name, hitName(got, ok))
						}
					}
					watch.tap(centre)
					want := tg.want
					if (tg.name == "previous arrow" && idx <= 0) || (tg.name == "next arrow" && idx >= len(nums)-1) {
						want = "" // no chapter to move to: the arrow takes the tap and does nothing
					}
					reached := strings.Join(watch.reached(), " and ")
					if !strings.HasPrefix(reached, want) || (want == "" && reached != "") {
						t.Errorf("a tap at the centre of the %s reached %q, want %q", tg.name, reached, want)
					}
				}
			})
		}
	}
}

// The live path: tapping the speaker opens the card in place, the header keeps
// its height, and the card is lifted over everything it lies over, moved in
// from the header's edge where it would run past it, and covers what it lies
// over as the card a rebuilt header shows does; tapping its ✕, on the screen,
// closes it and puts the speaker back beneath its neighbours, where it was.
// The row lifts the control by moving it from its first part, a place drawn
// before the chapter block and the full-screen button, to its last, a place
// drawn after them, and back: its parts and their order never change, which a
// painting canvas would answer by laying the whole row out again
// (TestOpeningTheCardLaysOutOnlyTheControlOnTheNextFrame).
func TestPhoneHeaderLiftsTheCardOnlyWhileItIsOpen(t *testing.T) {
	th := coverApp(t, true)
	for _, b := range coverBooks {
		for _, w := range phoneCanvasWidths {
			t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
				st, nums := coverState(th, b.book, b.chapter, b.chapters)
				audioPanelOpen = false
				header := chapterHeaderMobile(st, nums)
				win := coverWindow(t, st, phonePage(header), w)
				watch := watchCover(st, win)
				height := header.Size().Height
				tree := drawnTree(t, win.Canvas().Content())
				speaker := oneDrawn(t, tree, "the speaker", isIconButton(theme.VolumeUpIcon()))
				heading := oneDrawn(t, tree, "the heading", isType[*referenceButton])
				row, audio := branchBelow(tree, speaker, heading)
				rowObjects := row.obj.(*fyne.Container)
				if rowObjects.Objects[0] != audio.obj {
					t.Fatalf("with the card closed the audio control is not drawn first in the header row")
				}
				parts := append([]fyne.CanvasObject(nil), rowObjects.Objects...)
				sameParts := func(when string) {
					same := len(rowObjects.Objects) == len(parts)
					for i := 0; same && i < len(parts); i++ {
						same = rowObjects.Objects[i] == parts[i]
					}
					if !same {
						t.Errorf("%s, the header row's parts are not the ones it had, in the order it had them: a painting canvas lays a row out again when they change", when)
					}
				}

				// Tap the speaker where it, and not a neighbour over it, takes the tap.
				var at fyne.Position
				found := false
				for _, p := range gridOver(speaker.pos, speaker.obj.Size()) {
					if got, ok := hitAt(tree, p, driverMatchers[2].match); ok && got.obj == speaker.obj {
						at, found = p, true
						break
					}
				}
				if !found {
					t.Fatalf("no point of the speaker (%s) takes a tap", speaker)
				}
				watch.tap(at)
				if !audioPanelOpen {
					t.Fatalf("a tap on the speaker at %.1f,%.1f did not open the card", at.X, at.Y)
				}
				if h := header.Size().Height; h != height || header.MinSize().Height != height {
					t.Errorf("opening the card changed the header's height from %.1f to %.1f", height, header.MinSize().Height)
				}
				tree = drawnTree(t, win.Canvas().Content())
				card, _ := openCard(t, tree)
				if _, place := branchBelow(tree, card, heading); place.obj != rowObjects.Objects[len(rowObjects.Objects)-1] {
					t.Errorf("with the card open the audio control is not drawn last in the header row")
				}
				sameParts("with the card open")
				phoneAudioCell(t, tree, card, true)
				for _, d := range tree {
					if !d.under(card.obj) && d.drawsSomething() && d.overlaps(card) && d.order > card.order {
						t.Errorf("%s is drawn over the card the speaker opened", d)
					}
				}
				assertCardCovers(t, win, tree, watch, &cardTaps{})

				closeX := oneDrawn(t, tree, "the card's ✕", isType[*tappableArea])
				s := closeX.obj.Size()
				at = closeX.pos.Add(fyne.NewPos(s.Width/2, s.Height/2))
				if !onCanvas(win, at) {
					t.Fatalf("the card's ✕ (%s) is centred off the %.0f-wide canvas: the reader cannot close the card", closeX, win.Canvas().Size().Width)
				}
				watch.tap(at)
				if audioPanelOpen {
					t.Fatal("a tap on the card's ✕ did not close it")
				}
				tree = drawnTree(t, win.Canvas().Content())
				if n := len(findDrawn(tree, isType[*tapShield])); n != 0 {
					t.Errorf("closing the card left %d tap shields behind", n)
				}
				speaker = oneDrawn(t, tree, "the speaker", isIconButton(theme.VolumeUpIcon()))
				if _, place := branchBelow(tree, speaker, heading); place.obj != rowObjects.Objects[0] {
					t.Errorf("with the card closed again the audio control is not drawn first in the header row")
				}
				sameParts("with the card closed again")
				phoneAudioCell(t, tree, speaker, false)
			})
		}
	}
}

// toolbarSurfaces are the two places the desktop toolbar is shown: a desktop
// window, down to the narrowest reading pane, and the Android fallback pane,
// which uses the toolbar on a phone.
var toolbarSurfaces = []struct {
	name   string
	phone  bool
	widths []float32
}{
	{"desktop", false, []float32{320, 360, 402, 480, 560, 700, 1000}},
	{"Android fallback", true, phoneCanvasWidths},
}

// The desktop toolbar (reading.go's chapterHeader) carries the same card: on a
// narrow pane, and on the Android fallback pane, which uses this toolbar on a
// phone, the open card lies over the chapter block. It must cover it as the
// phone header's card covers what it lies over, hiding the copy icon and the
// arrows whose glyphs it touches, and its controls must work.
//
// On the desktop the pointer can also hover, which the mobile driver never
// delivers. The shield does not take hover or the cursor (audio_button.go
// says why): nothing the desktop card can lie over reacts to the pointer. This
// checks that premise, so a control that does react arriving under the card
// fails here rather than lighting up, or changing the cursor, through it.
func TestOpenToolbarCardCoversTheChapterBlock(t *testing.T) {
	for _, surface := range toolbarSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			th := coverApp(t, surface.phone)
			overlapped := 0
			for _, b := range coverBooks {
				for _, w := range surface.widths {
					t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
						st, nums := coverState(th, b.book, b.chapter, b.chapters)
						audioPanelOpen = false
						closedHeader := chapterHeader(st, nums)
						win := coverWindow(t, st, toolbarPage(closedHeader), w)
						drawnTree(t, win.Canvas().Content())
						closedH := closedHeader.MinSize().Height

						audioPanelOpen = true
						header := chapterHeader(st, nums)
						showPage(win, toolbarPage(header), w)
						if h := header.MinSize().Height; h != closedH {
							t.Errorf("the toolbar is %.1f tall with the card open and %.1f with it closed", h, closedH)
						}

						hs := desktopToolbarSurface
						if surface.phone {
							hs = fallbackToolbarSurface
						}
						assertCoverage(t, readHeader(t, newLayoutWalker(t), win.Canvas().Content(), hs), nil, nil)

						watch := watchCover(st, win)
						taps := &cardTaps{}
						tree := swapInCard(t, win, st, audioRecorded, taps)
						covered := assertCardCovers(t, win, tree, watch, taps)
						assertCardControlsWork(t, win, tree, watch, taps, true)
						for _, d := range covered {
							switch d.obj.(type) {
							case *iconTapButton, *referenceButton, *tapText:
								overlapped++
							}
							if surface.phone {
								continue
							}
							_, hover := d.obj.(desktop.Hoverable)
							_, cursor := d.obj.(desktop.Cursorable)
							if hover || cursor {
								t.Errorf("the open card lies over %s, which reacts to the desktop pointer: give tapShield desktop.Hoverable and desktop.Cursorable", d)
							}
						}
					})
				}
			}
			if overlapped == 0 {
				t.Errorf("at no width does the open card lie over the chapter block, so the checks above tested no covering")
			}
		})
	}
}

// The card's fill is opaque in both palettes: what the card lies over cannot
// show through it in the light theme or the dark one. (The test app runs in the
// light variant, so assertCardCovers checks the frame it draws; this checks the
// colour the dark theme would draw it in.)
func TestAudioCardFillIsOpaqueInBothPalettes(t *testing.T) {
	for name, p := range map[string]palette{"light": lightPalette, "dark": darkPalette} {
		if p.SurfaceAlt.A != 255 {
			t.Errorf("the %s palette's SurfaceAlt, the audio card's fill, has alpha %d: what the card covers shows through it", name, p.SurfaceAlt.A)
		}
	}
}

// With the card closed, the desktop toolbar and the Android fallback pane's
// are what they always were. No shield exists, and nothing in the audio
// control's reserved cell but the speaker takes an event, so where that cell
// reaches over the chapter block on a narrow pane the block keeps its taps;
// and a tap on each of the heading, the chapter line, the chapter arrows and
// the focus toggle reaches that control and does what it does. (The copy icon
// is left to the hit test: its tap starts a timer that would outlive the test.)
func TestClosedToolbarSpeakerTakesOnlyItsOwnTaps(t *testing.T) {
	for _, surface := range toolbarSurfaces {
		t.Run(surface.name, func(t *testing.T) {
			th := coverApp(t, surface.phone)
			overlapped := 0
			for _, b := range coverBooks {
				for _, w := range surface.widths {
					t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
						st, nums := coverState(th, b.book, b.chapter, b.chapters)
						audioPanelOpen = false
						win := coverWindow(t, st, toolbarPage(chapterHeader(st, nums)), w)
						tree := drawnTree(t, win.Canvas().Content())
						watch := watchCover(st, win)

						if n := len(findDrawn(tree, isType[*tapShield])); n != 0 {
							t.Errorf("found %d tap shields with the card closed: the speaker's cell would block the controls around it", n)
						}
						speaker := oneDrawn(t, tree, "the speaker", isIconButton(theme.VolumeUpIcon()))
						focus := oneDrawn(t, tree, "the focus toggle", isType[*widget.Button])
						_, cell := branchBelow(tree, speaker, focus)
						assertClosedControlTakesOnlyTheSpeaker(t, tree, cell, speaker)
						for _, d := range tree {
							switch d.obj.(type) {
							case *iconTapButton, *referenceButton, *tapText:
								if !d.under(cell.obj) && d.overlaps(cell) {
									overlapped++
								}
							}
						}

						idx := indexOf(nums, st.CurrentChapter)
						for _, tg := range []struct {
							name  string
							match func(fyne.CanvasObject) bool
							want  string
						}{
							{"heading", isType[*referenceButton], "the chapter picker"},
							{"chapter line", func(o fyne.CanvasObject) bool {
								l, ok := o.(*tapText)
								return ok && strings.HasPrefix(l.text, "Chapter")
							}, "the chapter picker"},
							{"previous arrow", isIconButton(theme.NavigateBackIcon()), "a chapter arrow"},
							{"next arrow", isIconButton(theme.NavigateNextIcon()), "a chapter arrow"},
							{"focus toggle", isType[*widget.Button], "the full-screen button"},
						} {
							d := oneDrawn(t, tree, "the "+tg.name, tg.match)
							var at fyne.Position
							found := false
							for _, p := range gridOver(d.pos, d.obj.Size()) {
								if got, ok := hitAt(tree, p, driverMatchers[2].match); ok && got.obj == d.obj && onCanvas(win, p) {
									at, found = p, true
									break
								}
							}
							if !found {
								t.Errorf("no point of the %s (%s) takes a tap with the card closed", tg.name, d)
								continue
							}
							watch.tap(at)
							want := tg.want
							if (tg.name == "previous arrow" && idx <= 0) || (tg.name == "next arrow" && idx >= len(nums)-1) {
								want = "" // no chapter to move to: the arrow takes the tap and does nothing
							}
							reached := strings.Join(watch.reached(), " and ")
							if !strings.HasPrefix(reached, want) || (want == "" && reached != "") {
								t.Errorf("a tap on the %s at %.1f,%.1f reached %q, want %q", tg.name, at.X, at.Y, reached, want)
							}
							if audioPanelOpen {
								t.Errorf("a tap on the %s at %.1f,%.1f opened the narration card", tg.name, at.X, at.Y)
								audioPanelOpen = false
							}
						}
					})
				}
			}
			if overlapped == 0 {
				t.Errorf("at no width does the closed control's cell reach over the chapter block, so the checks above tested no overlap")
			}
		})
	}
}
