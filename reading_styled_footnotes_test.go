package bibletext

// The styled pane's chapter-bottom footnote section is GEOMETRY-ONLY
// (reading_styled_footnotes.go): these tests pin the purity-by-construction
// properties — the selection model never contains the apparatus, presses in
// it are inert, washes cannot reach it — plus the sizing contract MinSize
// carries for the scroll machinery.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

func styledFnState() *AppState {
	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	st.Bible.OrphanFootnotes = map[string]map[int][]OrphanFootnote{
		"John": {3: {{Verse: 17, Text: "Some copies omit verse 17.", Caller: "+"}}},
	}
	return st
}

// The selection model is byte-identical with the section on or off —
// select-all, copy, share and verse attribution exclude the apparatus by
// construction, the styled pane's twin of the Apple content-end clamps.
func TestStyledFootnoteSectionStaysOutOfTheTextModel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := styledFnState()
	setFootnotesEnabled(false)
	off := newTestPane(t, st, 420)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := newTestPane(t, st, 420)

	if off.lay.Text != on.lay.Text {
		t.Fatalf("lay.Text must be byte-identical with the section on:\n off: %q\n  on: %q", off.lay.Text, on.lay.Text)
	}
	on.selectAll()
	clip := &fakeClipboard{}
	on.clipboard = clip
	on.copyToClipboard()
	for _, probe := range []string{"loved the world in this way", "omit verse 17"} {
		if strings.Contains(clip.content, probe) {
			t.Errorf("select-all copy carried apparatus text: %q", probe)
		}
	}
}

// MinSize grows only when the toggle is on AND the chapter has notes; the
// section hangs strictly below the chapter's own height.
func TestStyledFootnoteSectionSizing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := styledFnState()
	setFootnotesEnabled(false)
	base := newTestPane(t, st, 420).MinSize().Height

	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := newTestPane(t, st, 420)
	if on.MinSize().Height <= base {
		t.Errorf("MinSize must grow with the section on: %v <= %v", on.MinSize().Height, base)
	}
	if !on.fnGeom.present || on.fnGeom.rect.Y < on.lay.Height {
		t.Errorf("the section must hang below the chapter: rect.Y=%v lay.Height=%v", on.fnGeom.rect.Y, on.lay.Height)
	}
	// Toggle on, no notes: nothing changes.
	plain := footnoteFixtureState(stripFootnotes(footnoteFixtureVerses()))
	if got := newTestPane(t, plain, 420); got.fnGeom.present {
		t.Error("a note-free chapter must grow no section")
	}
	// The orphan-only chapter still gets one.
	orphanOnly := footnoteFixtureState(stripFootnotes(footnoteFixtureVerses()))
	orphanOnly.Bible.OrphanFootnotes = map[string]map[int][]OrphanFootnote{
		"John": {3: {{Verse: 17, Text: "Some copies omit verse 17."}}},
	}
	if got := newTestPane(t, orphanOnly, 420); !got.fnGeom.present {
		t.Error("an orphan-only chapter must still render the section")
	}
}

// A press inside the section starts no selection, clears nothing, selects no
// word, opens no study menu — the sticker's "not text" discipline.
func TestStyledFootnoteSectionPressesAreInert(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	p := newTestPane(t, styledFnState(), 420)
	if !p.fnGeom.present {
		t.Fatal("fixture must render a section")
	}
	inSection := fyne.NewPos(p.fnGeom.rect.X+4, p.fnGeom.rect.Y+p.fnGeom.rect.H/2)
	p.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: inSection},
		Button:     desktop.MouseButtonPrimary,
	})
	if p.selStart != -1 || p.selAnchor != -1 {
		t.Errorf("press in the section started a selection: anchor=%d start=%d", p.selAnchor, p.selStart)
	}
	// An existing selection survives a tap on the section.
	p.selectAll()
	before := p.selectedRaw()
	p.Tapped(&fyne.PointEvent{Position: inSection})
	if got := p.selectedRaw(); got != before {
		t.Errorf("tap in the section changed the selection: %q -> %q", before, got)
	}
	p.DoubleTapped(&fyne.PointEvent{Position: inSection})
	if got := p.selectedRaw(); got != before {
		t.Errorf("double-tap in the section changed the selection: %q -> %q", before, got)
	}
}

// A DRAG that begins in the section is as inert as the press: without the
// fnGrab latch, Dragged planted an anchor via lineAtY's clamp onto the last
// scripture line (fresh pane), or extended a STALE anchor left behind by
// clearSelection (after any earlier click) and selected whole verses while
// the pointer never left the apparatus.
func TestStyledFootnoteSectionDragsAreInert(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	p := newTestPane(t, styledFnState(), 420)
	inSection := fyne.NewPos(p.fnGeom.rect.X+4, p.fnGeom.rect.Y+p.fnGeom.rect.H/2)
	inText := fyne.NewPos(p.insetX()+10, p.lay.Lines[0].Y+2)

	// Fresh pane: press in the section, drag across it and up into the text.
	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: inSection}, Button: desktop.MouseButtonPrimary})
	p.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: inText}})
	p.DragEnd()
	if got := p.selectedRaw(); got != "" {
		t.Errorf("drag from the section selected text: %q", got)
	}

	// Stale-anchor shape: a click in the text (leaves selAnchor behind after
	// the clear), then press-and-drag inside the section.
	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: inText}, Button: desktop.MouseButtonPrimary})
	p.DragEnd()
	p.Tapped(&fyne.PointEvent{Position: inText}) // clears selection, may leave the anchor
	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: inSection}, Button: desktop.MouseButtonPrimary})
	p.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: inSection}})
	p.DragEnd()
	if got := p.selectedRaw(); got != "" {
		t.Errorf("drag within the section after an earlier click selected text: %q", got)
	}
}

// Narrower width re-wraps the section with the chapter: more lines, taller
// geometry — measured inside relayout, so Resize drives it for free.
func TestStyledFootnoteSectionRewrapsOnResize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	p := newTestPane(t, styledFnState(), 520)
	wide := p.fnGeom.height
	p.Resize(fyne.NewSize(220, 400))
	if p.fnGeom.height <= wide {
		t.Errorf("narrow width must re-wrap the section taller: %v <= %v", p.fnGeom.height, wide)
	}
}

func TestStyledFnWrap(t *testing.T) {
	meas := func(s string) float32 { return float32(len(s)) }
	lines := styledFnWrap("aa bb cc dd", 6, 8, meas)
	// First line wraps at 6 ("aa bb"), continuations at 8 ("cc dd").
	if len(lines) != 2 || lines[0] != "aa bb" || lines[1] != "cc dd" {
		t.Errorf("wrap wrong: %q", lines)
	}
	if got := styledFnWrap("   ", 10, 10, meas); got != nil {
		t.Errorf("blank text must wrap to nothing: %q", got)
	}
}

// --- superscription on the styled pane ----------------------------------------

// The title occupies a reserved advance ABOVE the chapter: lay.Text is
// byte-identical with and without it, lines start below its height, presses
// on it are inert, and a drag beginning on it selects nothing.
func TestStyledSuperscriptionIsGeometryOnly(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := footnoteFixtureVerses()
	plain := newTestPane(t, footnoteFixtureState(verses), 420)

	st := footnoteFixtureState(verses)
	st.Bible.Superscriptions = map[string]map[int]Superscription{
		"John": {3: {Text: "A test title, according to Gittith."}},
	}
	p := newTestPane(t, st, 420)

	if !p.superGeom.present {
		t.Fatal("fixture must produce a title")
	}
	if p.lay.Text != plain.lay.Text {
		t.Fatal("the title must never enter lay.Text")
	}
	if first := p.lay.Lines[0].Y; first < p.superGeom.height {
		t.Errorf("first line (Y=%v) must clear the title advance (%v)", first, p.superGeom.height)
	}
	if plain.MinSize().Height >= p.MinSize().Height {
		t.Error("MinSize must grow by the title's advance")
	}

	inTitle := fyne.NewPos(p.superGeom.rect.X+4, p.superGeom.rect.Y+p.superGeom.rect.H/3)
	p.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: inTitle}, Button: desktop.MouseButtonPrimary})
	p.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(p.insetX()+10, p.lay.Lines[0].Y+2)}})
	p.DragEnd()
	if got := p.selectedRaw(); got != "" {
		t.Errorf("press-and-drag from the title selected text: %q", got)
	}
}

// A superscription note keys as "Title" on this pane too — the shared key
// helper reaches all three section renderers.
func TestStyledFootnoteSectionKeysTitleNotes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	st := styledFnState()
	st.Bible.Superscriptions = map[string]map[int]Superscription{
		"John": {3: {Text: "A test title.",
			Footnotes: []Footnote{{Anchor: 0, Text: "A title gloss.", Caller: "+"}}}},
	}
	p := newTestPane(t, st, 420)
	if !p.fnGeom.present || len(p.fnGeom.texts) == 0 {
		t.Fatal("fixture must render a section")
	}
	if p.fnGeom.texts[0].Text != "Title" || !p.fnGeom.texts[0].Key {
		t.Errorf("first section key = %q (key=%v), want Title", p.fnGeom.texts[0].Text, p.fnGeom.texts[0].Key)
	}
	for _, ft := range p.fnGeom.texts {
		if ft.Key && ft.Text == "0" {
			t.Error("a verse-0 key leaked — title notes must key as Title")
		}
	}
}
