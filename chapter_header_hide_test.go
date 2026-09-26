package bibletext

// While the narration card is open, each small icon control of a chapter
// header whose glyph the card touches is hidden — not drawn, not tappable
// anywhere on its box, not reached by Tab — and it is back when the card
// closes (cardCover, audio_button.go). So is the full-screen button when the
// card touches the focus highlight it fills its box with. The heading and the
// chapter line, which name the chapter, are only covered; and nothing moves,
// open or closed. These tests hold the phone header and the desktop toolbar,
// which the Android fallback pane also uses, to that. The sweep lays both out
// for the first and last chapters of every book of the canon, at every tenth
// width from 300 to 1020, at 1024, and at 375, 393 and 402, the phones' own
// widths between those steps, and steps Tab round each of those layouts. Taps,
// the keyboard's focus and keys, a change of width while the card is open, and
// what a painting canvas lays out on the frame after the card opens or closes
// are tested on fewer layouts: the chapters coverBooks names and those each
// test adds, at the phone widths, the desktop pane's, and those each test
// adds.
//
// Where a control's glyph and fill are, the tests read off the canvas: the
// canvas.Image and the painted canvas.Rectangles among the control's own
// renderer objects, where the layout put them. The card is found by what it
// shows, its source chip and its ✕. So neither is taken on the header code's
// word, and a header that decided by the tap box, or by a card rectangle of
// its own, would fail here.

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// canonChapters is the 73-book canon, each book with its number of chapters
// in the English Bibles the app carries. A header's widths turn on the book's
// name and on the digits of its chapter numbers, so where an edition numbers a
// book differently the widths barely move: the Catholic edition's Greek Daniel
// runs to 14 chapters, as many digits as 12.
var canonChapters = []struct {
	book     string
	chapters int
}{
	{"Genesis", 50}, {"Exodus", 40}, {"Leviticus", 27}, {"Numbers", 36}, {"Deuteronomy", 34},
	{"Joshua", 24}, {"Judges", 21}, {"Ruth", 4}, {"1 Samuel", 31}, {"2 Samuel", 24},
	{"1 Kings", 22}, {"2 Kings", 25}, {"1 Chronicles", 29}, {"2 Chronicles", 36}, {"Ezra", 10},
	{"Nehemiah", 13}, {"Esther", 10}, {"Job", 42}, {"Psalms", 150}, {"Proverbs", 31},
	{"Ecclesiastes", 12}, {"Song of Solomon", 8}, {"Isaiah", 66}, {"Jeremiah", 52}, {"Lamentations", 5},
	{"Ezekiel", 48}, {"Daniel", 12}, {"Hosea", 14}, {"Joel", 3}, {"Amos", 9},
	{"Obadiah", 1}, {"Jonah", 4}, {"Micah", 7}, {"Nahum", 3}, {"Habakkuk", 3},
	{"Zephaniah", 3}, {"Haggai", 2}, {"Zechariah", 14}, {"Malachi", 4},
	{"Matthew", 28}, {"Mark", 16}, {"Luke", 24}, {"John", 21}, {"Acts", 28},
	{"Romans", 16}, {"1 Corinthians", 16}, {"2 Corinthians", 13}, {"Galatians", 6}, {"Ephesians", 6},
	{"Philippians", 4}, {"Colossians", 4}, {"1 Thessalonians", 5}, {"2 Thessalonians", 3},
	{"1 Timothy", 6}, {"2 Timothy", 4}, {"Titus", 3}, {"Philemon", 1},
	{"Hebrews", 13}, {"James", 5}, {"1 Peter", 5}, {"2 Peter", 3}, {"1 John", 5}, {"2 John", 1}, {"3 John", 1},
	{"Jude", 1}, {"Revelation", 22},
	{"Tobit", 14}, {"Judith", 16}, {"1 Maccabees", 16}, {"2 Maccabees", 15}, {"Wisdom", 19}, {"Sirach", 51}, {"Baruch", 6},
}

// hideWidths are the canvas widths the sweep lays each header out at: every
// width from 300 to 1020 in steps of 10, then 1024, and the phones' own widths
// that fall between the steps (375 an iPhone SE, 393 and 402 current iPhones;
// 320, 360, 390 and 430 fall on them).
func hideWidths() []float32 {
	var ws []float32
	for w := 300; w <= 1020; w += 10 {
		ws = append(ws, float32(w))
	}
	ws = append(ws, 1024, 375, 393, 402)
	sort.Slice(ws, func(i, j int) bool { return ws[i] < ws[j] })
	return ws
}

// coverBook is a chapter of coverBooks.
type coverBook = struct {
	book              string
	chapter, chapters int
}

// hideable is a header control the open card may hide, and how to find it.
type hideable struct {
	name  string
	match func(fyne.CanvasObject) bool
}

// hideSurface is a header as a reader meets it: the header, the page it is
// laid out on, the device it is laid out for, and the controls the card may
// hide in it.
type hideSurface struct {
	name    string
	phone   bool // laid out for a phone: the device says it is mobile
	toolbar bool // the desktop toolbar (reading.go) rather than the phone header
	header  func(*AppState, []int) fyne.CanvasObject
	page    func(fyne.CanvasObject) fyne.CanvasObject
}

// hideables are the controls the open card may hide on the surface: the copy
// icon and the chapter arrows everywhere, and the phone header's full-screen
// button. (The toolbar's focus toggle sits beside the card's cell in the right
// column's row, and is checked as a control the card must never touch.)
func (s hideSurface) hideables() []hideable {
	hs := []hideable{
		{"copy icon", isIconButton(theme.ContentCopyIcon())},
		{"previous arrow", isIconButton(theme.NavigateBackIcon())},
		{"next arrow", isIconButton(theme.NavigateNextIcon())},
	}
	if !s.toolbar {
		hs = append(hs, hideable{"full-screen button", isType[*coverButton]})
	}
	return hs
}

var (
	phoneHeaderSurface     = hideSurface{"phone header", true, false, chapterHeaderMobile, phonePage}
	desktopToolbarSurface  = hideSurface{"desktop toolbar", false, true, chapterHeader, toolbarPage}
	fallbackToolbarSurface = hideSurface{"Android fallback toolbar", true, true, chapterHeader, toolbarPage}
	hideSurfaces           = []hideSurface{phoneHeaderSurface, desktopToolbarSurface, fallbackToolbarSurface}
)

// rect is a rectangle on the canvas.
type rect struct {
	pos  fyne.Position
	size fyne.Size
}

// overlap is how far two rectangles overlap: the lesser of their overlaps
// along the two axes. It is positive where they share an area, zero where they
// only meet, and negative, the gap between them, where they are apart.
func (r rect) overlap(o rect) float32 {
	dx := min(r.pos.X+r.size.Width, o.pos.X+o.size.Width) - max(r.pos.X, o.pos.X)
	dy := min(r.pos.Y+r.size.Height, o.pos.Y+o.size.Height) - max(r.pos.Y, o.pos.Y)
	return min(dx, dy)
}

func (r rect) contains(p fyne.Position) bool {
	return p.X >= r.pos.X && p.Y >= r.pos.Y && p.X < r.pos.X+r.size.Width && p.Y < r.pos.Y+r.size.Height
}

func (r rect) String() string {
	return fmt.Sprintf("%.2f,%.2f %.2fx%.2f", r.pos.X, r.pos.Y, r.size.Width, r.size.Height)
}

// coverNoise is how near an edge two rectangles computed by different sums
// of the same float32 positions may come to disagree about. Where a glyph's
// edge lies this near the card's, the header's decision to hide it or not is
// taken as it stands, and counted.
const coverNoise = 0.01

// placed is one object of a laid-out header, hidden or not: where it lies on
// the canvas, how large it is, whether it is shown (it and everything that
// holds it visible), what holds it (the canvas's content first) and, for a
// container, the MinSize it asked for.
type placed struct {
	obj   fyne.CanvasObject
	pos   fyne.Position
	size  fyne.Size
	shown bool
	path  []fyne.CanvasObject
	min   fyne.Size
}

func (p placed) rect() rect { return rect{p.pos, p.size} }

// under reports whether o holds p.
func (p placed) under(o fyne.CanvasObject) bool {
	for _, a := range p.path {
		if a == o {
			return true
		}
	}
	return false
}

// layoutWalker walks a laid-out tree, the hidden parts included, as drawnTree
// walks what is drawn. Each widget's renderer is let go when the test ends, as
// drawnTree lets it go, but the release is asked for once per widget however
// often the sweep walks it, so the sweep does not pile up a cleanup on every
// walk.
type layoutWalker struct {
	t    *testing.T
	seen map[fyne.Widget]bool
}

func newLayoutWalker(t *testing.T) *layoutWalker {
	return &layoutWalker{t: t, seen: map[fyne.Widget]bool{}}
}

func (w *layoutWalker) renderer(wid fyne.Widget) fyne.WidgetRenderer {
	if w.seen[wid] {
		return test.WidgetRenderer(wid)
	}
	w.seen[wid] = true
	return test.TempWidgetRenderer(w.t, wid)
}

func (w *layoutWalker) walk(root fyne.CanvasObject) []placed {
	var out []placed
	var visit func(o fyne.CanvasObject, offset fyne.Position, shown bool, path []fyne.CanvasObject)
	visit = func(o fyne.CanvasObject, offset fyne.Position, shown bool, path []fyne.CanvasObject) {
		if o == nil {
			return
		}
		p := placed{obj: o, pos: o.Position().Add(offset), size: o.Size(), shown: shown && o.Visible(), path: path}
		var children []fyne.CanvasObject
		switch v := o.(type) {
		case *fyne.Container:
			children = v.Objects
			p.min = v.MinSize()
		case fyne.Widget:
			children = w.renderer(v).Objects()
		}
		out = append(out, p)
		below := append(append([]fyne.CanvasObject(nil), path...), o)
		for _, c := range children {
			visit(c, p.pos, p.shown, below)
		}
	}
	visit(root, fyne.Position{}, true, nil)
	return out
}

// viewControl is one of the header's controls as laid out: its box; the
// glyph it draws, the first canvas.Image among its renderer's objects; and
// the rectangle taking in every canvas.Rectangle among them that paints
// something (fill), where there is one (filled): a focused widget.Button's
// highlight.
type viewControl struct {
	name   string
	at     placed
	glyph  rect
	fill   rect
	filled bool
}

// union is the rectangle that takes in r and o.
func (r rect) union(o rect) rect {
	x0, y0 := min(r.pos.X, o.pos.X), min(r.pos.Y, o.pos.Y)
	x1, y1 := max(r.pos.X+r.size.Width, o.pos.X+o.size.Width), max(r.pos.Y+r.size.Height, o.pos.Y+o.size.Height)
	return rect{fyne.NewPos(x0, y0), fyne.NewSize(x1-x0, y1-y0)}
}

// rectPaints reports whether a rectangle paints anything: a fill, or an
// outline, in a colour that is not wholly transparent.
func rectPaints(r *canvas.Rectangle) bool {
	shows := func(c color.Color) bool {
		if c == nil {
			return false
		}
		_, _, _, a := c.RGBA()
		return a > 0
	}
	return r.Visible() && (shows(r.FillColor) || (r.StrokeWidth > 0 && shows(r.StrokeColor)))
}

// readControl reads a control off a walk of the tree that holds it, starting
// at its own entry: the glyph and the fill its renderer draws.
func readControl(name string, all []placed, at int) (viewControl, bool) {
	p := all[at]
	c := viewControl{name: name, at: p}
	found := false
	for _, d := range all[at+1:] {
		if !d.under(p.obj) {
			break
		}
		switch o := d.obj.(type) {
		case *canvas.Image:
			if !found {
				c.glyph, found = d.rect(), true
			}
		case *canvas.Rectangle:
			if !rectPaints(o) {
				continue
			}
			if c.filled {
				c.fill = c.fill.union(d.rect())
			} else {
				c.fill, c.filled = d.rect(), true
			}
		}
	}
	return c, found
}

// paintedNow reads what a control paints now, off a fresh walk of the control
// alone, placed where the header laid it out.
func paintedNow(w *layoutWalker, c viewControl) viewControl {
	all := w.walk(c.at.obj)
	shift := c.at.pos.Subtract(c.at.obj.Position())
	for i := range all {
		all[i].pos = all[i].pos.Add(shift)
	}
	now, _ := readControl(c.name, all, 0)
	now.at = c.at
	return now
}

// headerView is what the tests read off one laid-out header.
type headerView struct {
	surface     hideSurface
	all         []placed
	index       map[fyne.CanvasObject]int
	controls    []viewControl // the controls the card may hide, in hideables order
	heading     placed
	chapterLine placed
	focus       *viewControl // the toolbar's focus toggle
	speaker     *placed      // the closed control
	card        *placed      // the open card
	chip        *placed      // the open card's source chip
	closeX      *placed      // the open card's ✕
	row         placed       // the header row
	audioPart   placed       // the row's part that holds the audio control, and moves nothing else
}

func readHeader(t *testing.T, w *layoutWalker, root fyne.CanvasObject, s hideSurface) headerView {
	t.Helper()
	v := headerView{surface: s, all: w.walk(root), index: map[fyne.CanvasObject]int{}}
	for i, p := range v.all {
		v.index[p.obj] = i
	}
	one := func(what string, match func(fyne.CanvasObject) bool, shownOnly bool) *placed {
		var got []placed
		for _, p := range v.all {
			if match(p.obj) && (p.shown || !shownOnly) {
				got = append(got, p)
			}
		}
		switch {
		case len(got) == 1:
			return &got[0]
		case len(got) > 1:
			t.Fatalf("found %d of %s in the %s, want at most one", len(got), what, s.name)
		}
		return nil
	}
	control := func(name string, match func(fyne.CanvasObject) bool) viewControl {
		p := one("the "+name, match, false)
		if p == nil {
			t.Fatalf("the %s has no %s", s.name, name)
		}
		c, ok := readControl(name, v.all, v.index[p.obj])
		if !ok {
			t.Fatalf("the %s's %s draws no image", s.name, name)
		}
		return c
	}
	for _, h := range s.hideables() {
		v.controls = append(v.controls, control(h.name, h.match))
	}
	must := func(p *placed, what string) placed {
		if p == nil {
			t.Fatalf("the %s has no %s", s.name, what)
		}
		return *p
	}
	v.heading = must(one("the heading", isType[*referenceButton], false), "heading")
	v.chapterLine = must(one("the chapter line", func(o fyne.CanvasObject) bool {
		l, ok := o.(*tapText)
		return ok && strings.HasPrefix(l.text, "Chapter")
	}, false), "chapter line")
	if s.toolbar {
		f := control("focus toggle", isType[*widget.Button])
		v.focus = &f
	}
	v.speaker = one("the speaker", isIconButton(theme.VolumeUpIcon()), true)
	v.chip = one("the card's source chip", isType[*labeledTapChip], true)
	v.closeX = one("the card's ✕", isType[*tappableArea], true)
	inner := v.speaker
	if v.chip != nil && v.closeX != nil {
		card, _ := v.fork(*v.chip, *v.closeX)
		v.card, inner = &card, &card
	}
	if inner == nil {
		t.Fatalf("the %s shows neither the speaker nor the open card", s.name)
	}
	v.row, _ = v.fork(*inner, v.heading)
	anchor := v.heading
	if s.toolbar {
		anchor = v.focus.at
	}
	_, v.audioPart = v.fork(*inner, anchor)
	return v
}

// fork finds the lowest object holding both a and b, and its child on the way
// to a.
func (v headerView) fork(a, b placed) (fork, branch placed) {
	pa := append(append([]fyne.CanvasObject(nil), a.path...), a.obj)
	pb := append(append([]fyne.CanvasObject(nil), b.path...), b.obj)
	n := 0
	for n < len(pa) && n < len(pb) && pa[n] == pb[n] {
		n++
	}
	fork = v.all[v.index[pa[n-1]]]
	if n < len(pa) {
		branch = v.all[v.index[pa[n]]]
	}
	return fork, branch
}

// cardHost is the audio control's host, which holds the open card.
func (v headerView) cardHost(t *testing.T) *fyne.Container {
	t.Helper()
	host, ok := v.card.path[len(v.card.path)-1].(*fyne.Container)
	if !ok {
		t.Fatalf("the open card's parent is a %T, not the control's host", v.card.path[len(v.card.path)-1])
	}
	return host
}

func (v headerView) control(name string) viewControl {
	for _, c := range v.controls {
		if c.name == name {
			return c
		}
	}
	panic("no control " + name)
}

// coverTally counts what the checks met across a sweep, so that a sweep which
// never met a case can say so.
type coverTally struct {
	hidden         map[string]int // layouts that hid each control
	boxNotGlyph    int            // controls whose box the card overlapped, and not their glyph
	boundary       int            // glyphs whose edge met the card's within coverNoise
	headingCovered int            // layouts in which the card lay over the heading
	lit            int            // chapters in which Tab lit a control the card then hid
	layouts        int
}

func newCoverTally() *coverTally { return &coverTally{hidden: map[string]int{}} }

// assertCoverage checks a header with the card open: each control the card may
// hide is hidden if and only if the card's rectangle overlaps what it draws,
// its glyph and any fill it paints now, or, for a control lit while the card
// has been open (lit, by name: the focus lit its fill across its box, and the
// card hid it), its box; no control shown, the toolbar's focus toggle
// included, has a glyph or a fill the card overlaps; and the heading and the
// chapter line are shown. It returns the names of the controls hidden.
func assertCoverage(t *testing.T, v headerView, tally *coverTally, lit map[string]bool) []string {
	t.Helper()
	if v.card == nil {
		t.Fatalf("the %s shows no open card", v.surface.name)
	}
	card := v.card.rect()
	var hidden []string
	for _, c := range v.controls {
		depth := c.glyph.overlap(card)
		if c.filled {
			depth = max(depth, c.fill.overlap(card))
		}
		if lit[c.name] {
			depth = max(depth, c.at.rect().overlap(card))
		}
		want, got := depth > 0, !c.at.obj.Visible()
		if got {
			hidden = append(hidden, c.name)
		}
		near := depth > -coverNoise && depth < coverNoise
		if got != want && !near {
			verb := "hidden"
			if !got {
				verb = "shown"
			}
			t.Errorf("the %s is %s with the card open, where the card (%s) and what it draws (glyph %s; fill %v %s; lit %v, box %s) overlap by %.2f (less than nothing is a gap): it is to be hidden if and only if they overlap",
				c.name, verb, card, c.glyph, c.filled, c.fill, lit[c.name], c.at.rect(), depth)
		}
		if tally == nil {
			continue
		}
		if near {
			tally.boundary++
		}
		if got {
			tally.hidden[c.name]++
		}
		if c.glyph.overlap(card) <= 0 && c.at.rect().overlap(card) > 0 {
			tally.boxNotGlyph++
		}
	}
	shown := v.controls
	if v.focus != nil {
		shown = append(append([]viewControl(nil), shown...), *v.focus)
	}
	for _, c := range shown {
		if !c.at.shown {
			continue
		}
		if c.glyph.overlap(card) > coverNoise {
			t.Errorf("the %s is shown with its glyph (%s) under the open card (%s)", c.name, c.glyph, card)
		}
		if c.filled && c.fill.overlap(card) > coverNoise {
			t.Errorf("the %s is shown with the fill it paints (%s) under the open card (%s)", c.name, c.fill, card)
		}
	}
	for _, p := range []placed{v.heading, v.chapterLine} {
		if !p.shown {
			t.Errorf("%T at %s is hidden with the card open: the heading and the chapter line are only ever covered", p.obj, p.rect())
		}
	}
	if tally != nil {
		tally.layouts++
		if v.heading.rect().overlap(card) > 0 {
			tally.headingCovered++
		}
	}
	return hidden
}

// assertAllShown checks that every control is shown, with the card closed.
func assertAllShown(t *testing.T, v headerView, when string) {
	t.Helper()
	for _, c := range v.controls {
		if !c.at.shown {
			t.Errorf("%s, the %s is hidden", when, c.name)
		}
	}
	if v.card != nil {
		t.Errorf("%s, the card is open", when)
	}
}

// assertNothingMoved checks that every object of the header outside the part
// of the row that holds the audio control lies exactly where, and is exactly
// as large as, it was in before, that every container among them asks for the
// same MinSize, and that none has come or gone. Within that part the speaker
// gives way to the card, and the phone header may move the card; nothing else
// may move at all.
func assertNothingMoved(t *testing.T, before, after headerView, when string) {
	t.Helper()
	is := map[fyne.CanvasObject]placed{}
	for _, p := range after.all {
		if !p.under(after.audioPart.obj) {
			is[p.obj] = p
		}
	}
	var moved, resized, gone, come int
	report := func(format string, args ...any) {
		if moved+resized+gone+come <= 3 {
			t.Errorf(when+": "+format, args...)
		}
	}
	for _, b := range before.all {
		if b.under(before.audioPart.obj) {
			continue
		}
		a, ok := is[b.obj]
		delete(is, b.obj)
		switch {
		case !ok:
			gone++
			report("%T at %s has gone from the header", b.obj, b.rect())
		case a.pos != b.pos || a.size != b.size:
			moved++
			report("%T moved from %s to %s", b.obj, b.rect(), a.rect())
		case a.min != b.min:
			resized++
			report("%T at %s asks for a MinSize of %v, where it asked for %v", b.obj, a.rect(), a.min, b.min)
		}
	}
	for _, a := range after.all {
		if _, ok := is[a.obj]; ok {
			come++
			report("%T at %s has come into the header", a.obj, a.rect())
		}
	}
	if n := moved + resized + gone + come; n > 3 {
		t.Errorf("%s: %d objects moved or resized, %d containers asked for another MinSize, %d went and %d came, in all",
			when, moved, resized, gone, come)
	}
}

// assertTabSkipsHidden steps the keyboard focus both ways round the whole
// page with the card open and checks that it never lands on a hidden object,
// and that the control it lands on paints nothing under the card, its focus
// highlight included; on the toolbar, that it does reach the focus toggle,
// which the card never hides. A control shown before the steps and hidden
// after them was hidden because the focus lit its fill: its box must reach
// under the card, and it is added to lit, for assertCoverage to expect it
// hidden wherever the card overlaps its box until the card closes.
func assertTabSkipsHidden(t *testing.T, win fyne.Window, walk *layoutWalker, v headerView, lit map[string]bool) {
	t.Helper()
	c := win.Canvas()
	card := v.card.rect()
	focusable := v.controls
	if v.focus != nil {
		focusable = append(append([]viewControl(nil), focusable...), *v.focus)
	}
	reachedToggle := false
	for _, step := range []func(){c.FocusNext, c.FocusNext, c.FocusPrevious, c.FocusPrevious, c.FocusPrevious} {
		step()
		f := c.Focused()
		if f == nil {
			continue
		}
		o := f.(fyne.CanvasObject)
		if !o.Visible() {
			t.Errorf("with the card open, Tab reached %T, which the card hides", o)
		}
		if v.focus != nil && o == v.focus.at.obj {
			reachedToggle = true
		}
		for _, ctl := range focusable {
			if ctl.at.obj != o {
				continue
			}
			if now := paintedNow(walk, ctl); now.filled && now.fill.overlap(card) > coverNoise {
				t.Errorf("with the card open, Tab left the %s focused, with the fill it paints (%s) under the open card (%s)", ctl.name, now.fill, card)
			}
		}
	}
	c.Unfocus()
	if v.focus != nil && !reachedToggle {
		t.Errorf("with the card open, Tab never reaches the toolbar's focus toggle")
	}
	for _, ctl := range v.controls {
		if !ctl.at.shown || ctl.at.obj.Visible() {
			continue
		}
		if ctl.at.rect().overlap(card) <= 0 {
			t.Errorf("Tab hid the %s, whose box (%s) the open card (%s) does not reach", ctl.name, ctl.at.rect(), card)
		}
		lit[ctl.name] = true
	}
}

// assertCardLabel checks that the open card is the one meant.
func assertCardLabel(t *testing.T, v headerView, want string) {
	t.Helper()
	if got := v.chip.obj.(*labeledTapChip).label; got != want {
		t.Fatalf("the open card's source chip says %q, want %q", got, want)
	}
}

// The sweep: every book of the canon, at its first chapter and its last, at
// each of the widths hideWidths gives, on the phone header, the desktop
// toolbar and the Android fallback pane's toolbar. The header is laid out at
// each width with the card closed, and every control is shown. Then the
// speaker is tapped, as a reader opens the card, and the window is taken
// through the widths again, narrowest last, as a reader turning a phone or
// resizing a window would: at each, each control the card may hide is hidden
// if and only if the card overlaps its glyph; no control shown has a glyph
// under the card; the heading and the chapter line are shown; and every object
// outside the audio control lies exactly where, and is as large as, it was at
// that width with the card closed, and every container asks for the MinSize it
// asked for then. Then the "Narrator" card takes the "Read aloud" card's place
// and is taken through the widths, and hides the same controls at each. Then
// the "Read aloud" card is taken through them once more, narrowest last, with
// the keyboard: at each width Tab and Shift-Tab step round the page, and never
// land on a hidden control or leave a focus highlight under the card; where
// the focus lit the full-screen button's highlight over the edge of its box
// that the card overlaps, the button is hidden from then on wherever the card
// overlaps its box, and the other checks expect it so. Then, at 320, the ✕
// closes the card, and every control is back, and nothing has moved.
//
// (Opening and closing the card at each width, rather than once, would
// rebuild the card each time, and the rebuild, which draws its icons afresh,
// costs ten times what the checks do. The other tests open and close the card
// at their widths, and TestCardCoverFollowsTheWidthWhileOpen holds a change of
// width while it is open to the layout the header has with it closed.)
func TestOpenCardHidesExactlyTheControlsItTouches(t *testing.T) {
	widths := hideWidths()
	for _, s := range hideSurfaces {
		t.Run(s.name, func(t *testing.T) {
			th := coverApp(t, s.phone)
			tally := newCoverTally()
			for _, b := range canonChapters {
				chapters := []int{1, b.chapters}
				if b.chapters == 1 {
					chapters = chapters[:1]
				}
				for _, ch := range chapters {
					t.Run(fmt.Sprintf("%s %d", b.book, ch), func(t *testing.T) {
						sweepChapter(t, th, s, b.book, ch, b.chapters, widths, tally)
					})
				}
			}
			t.Logf("%d open layouts: hidden %v; a box but not its glyph under the card %d times; a glyph's edge within %.2f of the card's %d times; the heading covered in %d; Tab lit a control the card then hid in %d chapters",
				tally.layouts, tally.hidden, tally.boxNotGlyph, coverNoise, tally.boundary, tally.headingCovered, tally.lit)
			// The sweep must have met what it checks: each control hidden
			// somewhere, and a card over a control's box that leaves its glyph
			// alone, where a header that decided by the box would hide it; and,
			// on the phone header, Tab lighting the full-screen button's
			// highlight where the card overlaps only the edge of its box.
			for _, h := range s.hideables() {
				if tally.hidden[h.name] == 0 {
					t.Errorf("the open card hides the %s in no layout of the sweep", h.name)
				}
			}
			if tally.boxNotGlyph == 0 {
				t.Errorf("in no layout of the sweep does the card overlap a control's box and not its glyph")
			}
			if !s.toolbar && tally.lit == 0 {
				t.Errorf("in no chapter of the sweep does Tab light a control whose box the card overlaps")
			}
		})
	}
}

func sweepChapter(t *testing.T, th *bibleTheme, s hideSurface, book string, chapter, chapters int, widths []float32, tally *coverTally) {
	st, nums := coverState(th, book, chapter, chapters)
	audioPanelOpen = false
	win := coverWindow(t, st, s.page(s.header(st, nums)), widths[0])
	walk := newLayoutWalker(t)
	read := func(w float32) headerView {
		win.Resize(fyne.NewSize(w, 640))
		return readHeader(t, walk, win.Canvas().Content(), s)
	}
	stop := func() bool {
		return t.Failed() // one width's failures say it; the rest would repeat them
	}

	closed := map[float32]headerView{}
	for _, w := range widths {
		closed[w] = read(w)
		assertAllShown(t, closed[w], fmt.Sprintf("at %.0f with the card closed", w))
	}
	last := closed[widths[len(widths)-1]]
	if last.speaker == nil {
		t.Fatalf("the closed header shows no speaker")
	}
	last.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
	if !audioPanelOpen {
		t.Fatalf("the speaker did not open the card")
	}

	hidden := map[float32]string{}
	var open headerView
	for i := len(widths) - 1; i >= 0 && !stop(); i-- {
		w := widths[i]
		open = read(w)
		assertCardLabel(t, open, "Read aloud ▾")
		hidden[w] = fmt.Sprint(assertCoverage(t, open, tally, nil))
		assertNothingMoved(t, closed[w], open, fmt.Sprintf("at %.0f, the card open", w))
	}
	if stop() {
		return
	}

	host := open.cardHost(t)
	live := host.Objects
	host.Objects = []fyne.CanvasObject{buildAudioCard(st, audioRecorded, false, false, false, true, audioCardCallbacks{})}
	host.Refresh()
	for _, w := range widths {
		narrated := read(w)
		assertCardLabel(t, narrated, "Narrator ▾")
		if got := fmt.Sprint(assertCoverage(t, narrated, nil, nil)); got != hidden[w] {
			t.Errorf("at %.0f the Narrator card hides %s, the Read aloud card %s", w, got, hidden[w])
		}
		assertNothingMoved(t, closed[w], narrated, fmt.Sprintf("at %.0f, the Narrator card open", w))
		if stop() {
			return
		}
	}
	host.Objects = live
	host.Refresh()

	lit := map[string]bool{}
	for i := len(widths) - 1; i >= 0 && !stop(); i-- {
		w := widths[i]
		keyed := read(w)
		assertCoverage(t, keyed, nil, lit)
		assertNothingMoved(t, closed[w], keyed, fmt.Sprintf("at %.0f, the card open, with the keyboard", w))
		assertTabSkipsHidden(t, win, walk, keyed, lit)
	}
	if stop() {
		return
	}
	if len(lit) > 0 {
		tally.lit++
	}

	open = read(320)
	if hidden[320] == "[]" {
		t.Fatalf("the card hides nothing at 320, so closing it there shows nothing")
	}
	open.closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
	if audioPanelOpen {
		t.Fatalf("the card's ✕ did not close it")
	}
	reclosed := read(320)
	assertAllShown(t, reclosed, "at 320 once the card is closed again")
	assertNothingMoved(t, closed[320], reclosed, "at 320, the card closed again")
	if reclosed.speaker == nil || reclosed.speaker.rect() != closed[320].speaker.rect() {
		t.Errorf("at 320 the speaker is not back where it was once the card is closed")
	}
}

// controlActions is what a tap on each control does to the reader, as
// coverWatch names it.
var controlActions = map[string]string{
	"copy icon":          "the copy button",
	"previous arrow":     "a chapter arrow",
	"next arrow":         "a chapter arrow",
	"full-screen button": "the full-screen button",
}

// The live path's taps. With the card open, a tap anywhere on a hidden
// control's box never reaches that control: on the part the card lies over it
// reaches the card or nothing, and on the part beside the card it reaches
// nothing at all, save where another control shown there is drawn over the
// box (on the narrowest toolbars the focus toggle lies over the end of a long
// heading's copy icon, open or closed), which takes the tap as it always has.
// The card's own controls still work. And once the ✕ has closed the card,
// every control is back and works as it did before the card opened, at the
// same point of its box. The layouts are the ones the other tests use, and two
// in which a card drawn over the controls left one showing in part beside it:
// Matthew 27 on a 402-wide iPhone, Psalm 23 on a 360-wide Android phone.
//
// The taps go through Fyne's own hit test (test.TapCanvas), as every driver's
// do, and that hit test passes over an object that is not visible and
// everything in it (internal/driver/util.go, walkObjectTree), for every event
// a driver asks about. That is the premise the hiding stands on, and the taps
// are what would catch a change in it, or a control hidden some other way
// than Hide: they would reach the control. (Replaying each driver's question
// over the tree drawnTree reads cannot: drawnTree leaves out what is not
// visible, exactly as the hit test does, so it could never find a hidden
// control there.)
func TestControlsTheCardHidesTakeNoTap(t *testing.T) {
	books := append(coverBooks[:len(coverBooks):len(coverBooks)], coverBook{"Matthew", 27, 28}, coverBook{"Psalms", 23, 150})
	for _, s := range hideSurfaces {
		t.Run(s.name, func(t *testing.T) {
			th := coverApp(t, s.phone)
			widths := phoneCanvasWidths
			if !s.phone {
				widths = toolbarSurfaces[0].widths
			}
			hiddenSomewhere := map[string]bool{}
			beside, underCard := 0, 0
			for _, b := range books {
				for _, w := range widths {
					t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
						st, nums := coverState(th, b.book, b.chapter, b.chapters)
						audioPanelOpen = false
						win := coverWindow(t, st, s.page(s.header(st, nums)), w)
						walk := newLayoutWalker(t)
						watch := watchCover(st, win)
						closed := readHeader(t, walk, win.Canvas().Content(), s)

						// Where each control takes a tap before the card opens: the
						// first point of its box, on the canvas, where the hit test
						// hands it the tap. (On the narrowest toolbars the focus
						// toggle lies over the whole of a long heading's copy icon,
						// which then takes no tap at all, open or closed.)
						tapAt := map[string]fyne.Position{}
						tree := drawnTree(t, win.Canvas().Content())
						for _, c := range closed.controls {
							for _, p := range gridOver(c.at.pos, c.at.size) {
								if got, ok := hitAt(tree, p, driverMatchers[2].match); ok && got.obj == c.at.obj && onCanvas(win, p) {
									tapAt[c.name] = p
									break
								}
							}
						}

						closed.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
						open := readHeader(t, walk, win.Canvas().Content(), s)
						host := open.cardHost(t)
						live := host.Objects
						taps := &cardTaps{}
						host.Objects = []fyne.CanvasObject{buildAudioCard(st, audioRecorded, false, false, false, true, taps.callbacks())}
						host.Refresh()
						view := readHeader(t, walk, win.Canvas().Content(), s)
						card := view.card.rect()
						tree = drawnTree(t, win.Canvas().Content())
						cardObj, _ := openCard(t, tree)
						missed := 0
						miss := func(format string, args ...any) {
							if missed++; missed <= 3 {
								t.Errorf(format, args...)
							}
						}
						for _, c := range view.controls {
							if c.at.obj.Visible() {
								continue
							}
							hiddenSomewhere[c.name] = true
							for _, p := range gridOver(c.at.pos, c.at.size) {
								if !onCanvas(win, p) {
									continue
								}
								over, ok := hitAt(tree, p, driverMatchers[2].match)
								shownOver := ok && !over.under(cardObj.obj)
								on := card.contains(p)
								watch.tap(p)
								reached, fired := watch.reached(), taps.take()
								for _, r := range reached {
									if strings.HasPrefix(r, controlActions[c.name]) && !shownOver {
										miss("a tap at %.1f,%.1f on the hidden %s's box (the card is at %s) reached %s", p.X, p.Y, c.name, card, r)
									}
								}
								switch {
								case shownOver:
									// a control shown there takes the tap, as it always has
								case on:
									underCard++
									if len(reached) > 0 {
										miss("a tap at %.1f,%.1f on the hidden %s's box, under the card (%s), reached %s beneath it",
											p.X, p.Y, c.name, card, strings.Join(reached, " and "))
									}
								default:
									beside++
									if len(reached) > 0 || len(fired) > 0 {
										miss("a tap at %.1f,%.1f on the hidden %s's box, beside the card (%s), reached %s",
											p.X, p.Y, c.name, card, strings.Join(append(reached, fired...), " and "))
									}
								}
							}
						}
						if missed > 3 {
							t.Errorf("... %d misses in all on hidden controls' boxes", missed)
						}
						assertCardControlsWork(t, win, tree, watch, taps, true)

						host.Objects = live
						host.Refresh()
						x := open.closeX.rect()
						watch.tap(x.pos.Add(fyne.NewPos(x.size.Width/2, x.size.Height/2)))
						watch.reached()
						if audioPanelOpen {
							t.Fatalf("a tap at the centre of the card's ✕ did not close it")
						}
						after := readHeader(t, walk, win.Canvas().Content(), s)
						assertAllShown(t, after, "once the ✕ has closed the card")
						tree = drawnTree(t, win.Canvas().Content())
						idx := indexOf(nums, st.CurrentChapter)
						for _, c := range after.controls {
							p, ok := tapAt[c.name]
							if !ok {
								continue
							}
							if got, ok := hitAt(tree, p, driverMatchers[2].match); !ok || got.obj != c.at.obj {
								t.Errorf("with the card closed again, a tap at %.1f,%.1f goes to %s, where before the card opened it went to the %s",
									p.X, p.Y, hitName(got, ok), c.name)
								continue
							}
							if c.name == "copy icon" {
								continue // its tap starts a timer that would outlive the test
							}
							want := controlActions[c.name]
							if (c.name == "previous arrow" && idx <= 0) || (c.name == "next arrow" && idx >= len(nums)-1) {
								want = "" // no chapter to move to: the arrow takes the tap and does nothing
							}
							watch.tap(p)
							reached := strings.Join(watch.reached(), " and ")
							if !strings.HasPrefix(reached, want) || (want == "" && reached != "") {
								t.Errorf("with the card closed again, a tap on the %s at %.1f,%.1f reached %q, want %q", c.name, p.X, p.Y, reached, want)
							}
						}
					})
				}
			}
			for _, want := range []string{"next arrow", "copy icon"} {
				if !hiddenSomewhere[want] {
					t.Errorf("the card hides the %s in none of these layouts, so its taps were not tested (hidden: %v)", want, hiddenSomewhere)
				}
			}
			if !s.toolbar && !hiddenSomewhere["full-screen button"] {
				t.Errorf("the card hides the full-screen button in none of these layouts, so its taps were not tested")
			}
			if beside == 0 || underCard == 0 {
				t.Errorf("taps landed on hidden controls' boxes %d times beside the card and %d times under it: the test must reach both", beside, underCard)
			}
		})
	}
}

// tabBooks are the chapters the keyboard tests lay the phone header out for:
// coverBooks, and four in which, at tabWidths, the open card overlaps the edge
// of the full-screen button's box and not its glyph, so that the focus
// highlight the button fills its box with would reach under the card.
var tabBooks = append(coverBooks[:len(coverBooks):len(coverBooks)],
	coverBook{"Genesis", 1, 50}, coverBook{"Genesis", 50, 50}, coverBook{"Exodus", 1, 40}, coverBook{"Matthew", 27, 28})

var tabWidths = append(phoneCanvasWidths[:len(phoneCanvasWidths):len(phoneCanvasWidths)], 424, 430, 440)

// The keyboard: with the card closed, Tab reaches the phone header's
// full-screen button, its one focusable control, and the button fills its
// box with its focus highlight. Opened over the focused button, the card hides
// it, and takes the focus from it, wherever it touches the glyph or the
// highlight; where it is clear of the whole box the button keeps the focus,
// its highlight whole beside the card. Closed, the card gives the button back.
// Opened again with nothing focused, Tab and Shift-Tab, round the whole page,
// never land on a hidden control and never leave a highlight under the card:
// where the card overlaps only the edge of the button's box, Tab lands on the
// button, which is then hidden and gives up the focus, and it stays hidden
// through more Tabs and a layout of the row, until the card closes. Once the
// card is closed, Tab reaches it again.
func TestTabNeverReachesAControlTheCardHides(t *testing.T) {
	th := coverApp(t, true)
	byGlyph, byHighlight, kept, tabbedAway := 0, 0, 0, 0
	for _, b := range tabBooks {
		for _, w := range tabWidths {
			t.Run(fmt.Sprintf("%s %d at %.0f", b.book, b.chapter, w), func(t *testing.T) {
				st, nums := coverState(th, b.book, b.chapter, b.chapters)
				audioPanelOpen = false
				win := coverWindow(t, st, phonePage(chapterHeaderMobile(st, nums)), w)
				walk := newLayoutWalker(t)
				c := win.Canvas()
				closed := readHeader(t, walk, c.Content(), phoneHeaderSurface)
				fc := closed.control("full-screen button")
				full := fc.at.obj.(*coverButton)
				for _, step := range []func(){c.FocusNext, c.FocusPrevious} {
					c.Unfocus()
					step()
					if c.Focused() != full {
						t.Fatalf("with the card closed, Tab reaches %T, not the full-screen button", c.Focused())
					}
				}
				// The premise the highlight check stands on: focused, the
				// button paints its whole box.
				if now := paintedNow(walk, fc); !now.filled || now.fill != fc.at.rect() {
					t.Fatalf("the focused full-screen button paints %v %s, not its box %s", now.filled, now.fill, fc.at.rect())
				}

				closed.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
				open := readHeader(t, walk, c.Content(), phoneHeaderSurface)
				card := open.card.rect()
				fc = open.control("full-screen button")
				glyph, box := fc.glyph.overlap(card), fc.at.rect().overlap(card)
				switch {
				case min(abs32(glyph), abs32(box)) < coverNoise:
					// an edge too near the card's to say
				case glyph > 0 || box > 0:
					if glyph > 0 {
						byGlyph++
					} else {
						byHighlight++
					}
					if full.Visible() {
						t.Errorf("the card opened over the focused full-screen button, over its glyph by %.2f and its box by %.2f, and the button is shown", glyph, box)
					}
					if c.Focused() != nil {
						t.Errorf("the card hid the focused full-screen button, and %T still holds the focus", c.Focused())
					}
				default:
					kept++
					if !full.Visible() || c.Focused() != full {
						t.Errorf("the card is clear of the full-screen button's box (by %.2f), and yet the button is hidden (%v) or lost the focus (the focus is %T)", -box, !full.Visible(), c.Focused())
					}
					if now := paintedNow(walk, fc); !now.filled || now.fill.overlap(card) > 0 {
						t.Errorf("the focused full-screen button paints %v %s beside the open card (%s)", now.filled, now.fill, card)
					}
				}

				// Closed, the button is back; opened again with nothing
				// focused, it is what the card's glyph rule makes it.
				open.closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
				if !full.Visible() {
					t.Errorf("once the card is closed, the full-screen button is still hidden")
				}
				c.Unfocus()
				readHeader(t, walk, c.Content(), phoneHeaderSurface).speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
				open = readHeader(t, walk, c.Content(), phoneHeaderSurface)
				fc = open.control("full-screen button")
				shownOverBox := full.Visible() && fc.at.rect().overlap(card) > coverNoise
				lit := map[string]bool{}
				assertTabSkipsHidden(t, win, walk, open, lit)
				if shownOverBox {
					tabbedAway++
					if full.Visible() || !lit["full-screen button"] {
						t.Errorf("Tab landed on the full-screen button, whose box the card overlaps, and the button is still shown")
					}
					assertTabSkipsHidden(t, win, walk, readHeader(t, walk, c.Content(), phoneHeaderSurface), lit)
					row := open.row.obj.(*fyne.Container)
					row.Layout.Layout(row.Objects, row.Size())
					if full.Visible() {
						t.Errorf("once Tab had hidden the full-screen button, a layout of the row brought it back beside the open card")
					}
				}
				assertCoverage(t, readHeader(t, walk, c.Content(), phoneHeaderSurface), nil, lit)

				open.closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
				if !full.Visible() {
					t.Errorf("once the card is closed, the full-screen button is still hidden")
				}
				c.Unfocus()
				c.FocusNext()
				if c.Focused() != full {
					t.Errorf("once the card is closed, Tab reaches %T, not the full-screen button", c.Focused())
				}
				c.Unfocus()
			})
		}
	}
	if byGlyph == 0 || byHighlight == 0 || kept == 0 || tabbedAway == 0 {
		t.Errorf("opened over the focused button, the card hid it by its glyph in %d layouts and by its highlight in %d, and left it in %d; Tab lit it under the card in %d: the test must reach all four",
			byGlyph, byHighlight, kept, tabbedAway)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// A hidden control takes no key. The cover takes the focus from a control it
// hides only through the canvas, and while an overlay is up the canvas's focus
// is the overlay's: so a full-screen button that held the page's focus when
// an overlay opened, and that a change of width hides while the overlay is
// up, keeps the page's focus, and has it again once the overlay has gone,
// when the drivers hand it every key. Hiding it must leave the overlay's own
// focus alone; Space must not press it while it is hidden, and it gives up
// the focus instead; and once the card closes it is back, and Space presses
// it.
func TestAHiddenFullScreenButtonTakesNoKey(t *testing.T) {
	th := coverApp(t, true)
	for _, b := range coverBooks {
		t.Run(fmt.Sprintf("%s %d", b.book, b.chapter), func(t *testing.T) {
			st, nums := coverState(th, b.book, b.chapter, b.chapters)
			audioPanelOpen = false
			win := coverWindow(t, st, phonePage(chapterHeaderMobile(st, nums)), 1024)
			walk := newLayoutWalker(t)
			c := win.Canvas()
			v := readHeader(t, walk, c.Content(), phoneHeaderSurface)
			full := v.control("full-screen button").at.obj.(*coverButton)
			v.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
			if !full.Visible() {
				t.Fatalf("the card hides the full-screen button at 1024")
			}
			c.Unfocus()
			c.FocusNext()
			if c.Focused() != full {
				t.Fatalf("with the card open at 1024, Tab reaches %T, not the full-screen button", c.Focused())
			}

			other := widget.NewButton("in the overlay", func() {})
			pop := widget.NewPopUp(other, c)
			pop.Show()
			drawnTree(t, pop) // its renderers go when the test does
			c.Focus(other)
			if c.Focused() != other {
				t.Fatalf("the overlay's button did not take the focus")
			}
			win.Resize(fyne.NewSize(320, 640))
			if full.Visible() {
				t.Fatalf("the card does not hide the full-screen button at 320")
			}
			if c.Focused() != other {
				t.Errorf("hiding the full-screen button took the focus from the overlay's own button (the focus is %T)", c.Focused())
			}
			pop.Hide()

			key := func() {
				if f := c.Focused(); f != nil {
					f.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace}) // as the drivers hand the focused object a key
				}
			}
			heldIt := c.Focused() == full
			key()
			if st.IsFullScreen {
				st.IsFullScreen = false
				t.Errorf("Space pressed the full-screen button while the card hid it")
			}
			if c.Focused() == full {
				t.Errorf("a key reached the hidden full-screen button, and it kept the focus")
			}
			if !heldIt {
				t.Errorf("once the overlay had gone the page's focus was %T, not the hidden full-screen button: the test no longer tests a hidden control with the focus", c.Focused())
			}

			readHeader(t, walk, c.Content(), phoneHeaderSurface).closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
			if !full.Visible() {
				t.Fatalf("once the card is closed, the full-screen button is still hidden")
			}
			c.Unfocus()
			c.FocusNext()
			key()
			if !st.IsFullScreen {
				t.Errorf("once the card is closed, Space on the focused full-screen button does not press it")
			}
			st.IsFullScreen = false
			c.Unfocus()
		})
	}
}

// countingLayout counts the layouts of the row it stands in for.
type countingLayout struct {
	inner   fyne.Layout
	layouts int
}

func (c *countingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	c.layouts++
	c.inner.Layout(objects, size)
}

func (c *countingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return c.inner.MinSize(objects)
}

// A change of width while the card is open — a rotation, a split view, a
// resized window — lays the row out once, and what the card hides is worked
// out again for the new width: it is what the card hides at that width, and
// every object outside the audio control lies exactly where it lies in a
// header laid out at that width with the card closed, and every container
// asks for the MinSize it asks for there. Opening and closing the card lay the
// row out not at all, and once the card is closed nothing has moved.
func TestCardCoverFollowsTheWidthWhileOpen(t *testing.T) {
	pairs := [][2]float32{{1024, 320}, {320, 1024}, {402, 360}, {360, 402}, {375, 430}, {430, 375}, {700, 393}, {393, 700}, {320, 300}, {560, 402}}
	for _, s := range []hideSurface{phoneHeaderSurface, desktopToolbarSurface} {
		t.Run(s.name, func(t *testing.T) {
			th := coverApp(t, s.phone)
			changed := 0
			for _, b := range coverBooks {
				for _, pr := range pairs {
					t.Run(fmt.Sprintf("%s %d from %.0f to %.0f", b.book, b.chapter, pr[0], pr[1]), func(t *testing.T) {
						st, nums := coverState(th, b.book, b.chapter, b.chapters)
						audioPanelOpen = false
						win := coverWindow(t, st, s.page(s.header(st, nums)), pr[1])
						walk := newLayoutWalker(t)
						read := func() headerView { return readHeader(t, walk, win.Canvas().Content(), s) }
						closed := read() // at the width the card will be taken to
						win.Resize(fyne.NewSize(pr[0], 640))
						start := read()
						row, ok := start.row.obj.(*fyne.Container)
						if !ok {
							t.Fatalf("the header row is a %T", start.row.obj)
						}
						count := &countingLayout{inner: row.Layout}
						row.Layout = count

						start.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
						if count.layouts != 0 {
							t.Errorf("opening the card laid the header row out %d times", count.layouts)
						}
						before := assertCoverage(t, read(), nil, nil)

						count.layouts = 0
						win.Resize(fyne.NewSize(pr[1], 640))
						if count.layouts != 1 {
							t.Errorf("a change of width laid the header row out %d times, want once", count.layouts)
						}
						open := read()
						after := assertCoverage(t, open, nil, nil)
						if fmt.Sprint(before) != fmt.Sprint(after) {
							changed++
						}
						assertNothingMoved(t, closed, open, "the card open at the new width against the header closed there")

						count.layouts = 0
						open.closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
						if count.layouts != 0 {
							t.Errorf("closing the card laid the header row out %d times", count.layouts)
						}
						reclosed := read()
						assertAllShown(t, reclosed, "once the card is closed at the new width")
						assertNothingMoved(t, closed, reclosed, "the card closed again at the new width")
					})
				}
			}
			if changed == 0 {
				t.Errorf("no change of width changed what the card hides, so the test tested no recomputing")
			}
		})
	}
}

// frameNode is a painting canvas's record of one object it draws: its parent's,
// its first child's and its next sibling's records, and the MinSize the
// object asked for when it was last drawn.
type frameNode struct {
	parent, firstChild, nextSibling *frameNode
	obj                             fyne.CanvasObject
	minSize                         fyne.Size
}

// paintedFrames replays what Fyne's painting canvases (the glfw and mobile
// drivers) do before each frame, and the test canvas does not: a port of
// internal/driver/common's Canvas.walkTree and the ensureMinSize pass of
// Canvas.EnsureMinSize (fyne v2.7.4, canvas.go). The canvas keeps a record of
// every object it draws from frame to frame, matched to a container's or a
// renderer's objects by identity and in order, so an object met where a
// different one was drawn before is recorded afresh, and so is every object
// after it among its siblings. Before each frame it asks every visible object
// for its MinSize, and lays out again the parent of each whose MinSize is not
// the one recorded, a fresh record's included. frame returns the objects that
// laid out.
type paintedFrames struct {
	root *frameNode
	walk *layoutWalker // the renderers, let go when the test ends
}

func newPaintedFrames(walk *layoutWalker, content fyne.CanvasObject) *paintedFrames {
	return &paintedFrames{root: &frameNode{obj: content}, walk: walk}
}

func (f *paintedFrames) frame() []fyne.CanvasObject {
	var laid []fyne.CanvasObject
	layOut := func(o fyne.CanvasObject) {
		laid = append(laid, o)
		switch v := o.(type) {
		case *fyne.Container:
			if v.Layout != nil {
				v.Layout.Layout(v.Objects, v.Size())
			}
		case fyne.Widget:
			f.walk.renderer(v).Layout(v.Size())
		}
	}
	var node, parent, prev, needsLayout *frameNode
	node = f.root
	before := func(obj fyne.CanvasObject) {
		if node != nil && node.obj != obj {
			if parent.firstChild == node {
				parent.firstChild = nil
			}
			node = nil
		}
		if node == nil {
			node = &frameNode{parent: parent, obj: obj}
			if parent.firstChild == nil {
				parent.firstChild = node
			} else {
				prev.nextSibling = node
			}
		}
		if prev != nil && prev.parent != parent {
			prev = nil
		}
		parent = node
		node = parent.firstChild
	}
	after := func() {
		node = parent
		parent = node.parent
		if prev != nil && prev.parent != parent {
			prev.nextSibling = nil
		}
		if needsLayout == node {
			layOut(node.obj)
			needsLayout = nil
		}
		if min := node.obj.MinSize(); min != node.minSize {
			node.minSize = min
			if node.parent != nil {
				needsLayout = node.parent
			} else {
				layOut(node.obj)
			}
		}
		prev = node
		node = node.nextSibling
	}
	var visit func(o fyne.CanvasObject)
	visit = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		before(o)
		switch v := o.(type) {
		case *fyne.Container:
			for _, c := range v.Objects {
				visit(c)
			}
		case fyne.Widget:
			for _, c := range f.walk.renderer(v).Objects() {
				visit(c)
			}
		}
		after()
	}
	visit(f.root.obj)
	return laid
}

// On the drivers that paint, as on the test canvas, opening and closing the
// card is a repaint, never a relayout of the header. On the frame after the
// card opens, and after it closes, a painting canvas lays out the audio control
// and the part of the row that holds it, whose objects it draws afresh, and a
// control drawn for the first time (one the card has hidden since the header
// was built), within its slot: never the row, nor the boxes that place the
// heading, the chapter line and the controls. (A phone row that reordered its
// parts to lift the card was laid out again on each such frame.) The frame
// after that lays out nothing. A change of width while the card is open lays
// the row out there and then (TestCardCoverFollowsTheWidthWhileOpen), and the
// frame after it lays out no more than opening the card does. The phone header
// is checked built with the card closed and built with it open, as after a
// change of chapter with the card open; the toolbar the same way.
func TestOpeningTheCardLaysOutOnlyTheControlOnTheNextFrame(t *testing.T) {
	books := append(coverBooks[:len(coverBooks):len(coverBooks)], coverBook{"Matthew", 27, 28})
	for _, s := range []hideSurface{phoneHeaderSurface, desktopToolbarSurface} {
		t.Run(s.name, func(t *testing.T) {
			th := coverApp(t, s.phone)
			for _, b := range books {
				for _, w := range []float32{320, 402, 1024} {
					for _, builtOpen := range []bool{false, true} {
						t.Run(fmt.Sprintf("%s %d at %.0f built open %v", b.book, b.chapter, w, builtOpen), func(t *testing.T) {
							st, nums := coverState(th, b.book, b.chapter, b.chapters)
							audioPanelOpen = builtOpen
							win := coverWindow(t, st, s.page(s.header(st, nums)), w)
							walk := newLayoutWalker(t)
							read := func() headerView { return readHeader(t, walk, win.Canvas().Content(), s) }
							frames := newPaintedFrames(walk, win.Canvas().Content())
							first, v := frames.frame(), read()
							laidRow := false
							for _, o := range first {
								laidRow = laidRow || o == v.row.obj
							}
							if !laidRow {
								t.Fatalf("the first frame did not lay the header row out: the replay is not recording what it draws")
							}
							if again := frames.frame(); len(again) != 0 {
								t.Fatalf("a frame with nothing changed laid out %d objects", len(again))
							}
							next := func(when string) {
								t.Helper()
								v := read()
								allowed := map[fyne.CanvasObject]bool{}
								for _, p := range v.all {
									if p.obj == v.audioPart.obj || p.under(v.audioPart.obj) {
										allowed[p.obj] = true
									}
									for _, c := range v.controls {
										if p.obj == c.at.obj || p.under(c.at.obj) {
											allowed[p.obj] = true
										}
									}
								}
								for _, c := range v.controls {
									allowed[c.at.path[len(c.at.path)-1]] = true // its slot
								}
								for _, o := range frames.frame() {
									if !allowed[o] {
										at := "(not in the header)"
										if i, ok := v.index[o]; ok {
											at = v.all[i].rect().String()
										}
										t.Errorf("%s, the next frame laid out %T at %s", when, o, at)
									}
								}
								if again := frames.frame(); len(again) != 0 {
									t.Errorf("%s, the frame after the next laid out %d objects more", when, len(again))
								}
							}
							toggle := func(when string) {
								v := read()
								if v.card != nil {
									v.closeX.obj.(*tappableArea).Tapped(&fyne.PointEvent{})
								} else {
									v.speaker.obj.(*iconTapButton).Tapped(&fyne.PointEvent{})
								}
								next(when)
							}
							if builtOpen {
								toggle("closing the card the header was built with open")
								toggle("opening it again")
							} else {
								toggle("opening the card")
								toggle("closing it")
								toggle("opening it again")
							}
							if !audioPanelOpen {
								t.Fatalf("the card is not open at the end")
							}
							win.Resize(fyne.NewSize(w+57, 640))
							next("after a change of width with the card open")
						})
					}
				}
			}
		})
	}
}
