//go:build !race

package bibletext

// The rules of a note's ground, margin and rule (crossref_section.go) that
// TestEachNoteStandsOnAGroundOfItsOwn does not reach: the two grounds in
// both themes, the rule's contrast, the measures as figures, every shape of
// note that is not a row over pieces, a note keyed to no word, the width
// with a group open, and what the Treasury keeps. Each fails when its rule
// is broken.

import (
	"encoding/json"
	"image/color"
	"os"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// THE THREE GROUNDS, IN BOTH THEMES. A note takes the palette's ground
// further in lightness from the panel's and an opened group the other, so
// the three differ whichever theme is on: the page's near-white for a note
// in the light theme, the window's near-black in the dark.
func TestANotesGroundAndAGroupsDifferFromThePanelsInBothThemes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		pal         palette
		note, group color.NRGBA
	}{
		{"light", lightPalette, lightPalette.Surface, lightPalette.Background},
		{"dark", darkPalette, darkPalette.Background, darkPalette.Surface},
	} {
		note, group := crossRefGrounds(tc.pal)
		if note != tc.note || group != tc.group {
			t.Errorf("%s: a note stands on %v and an opened group on %v; want %v and %v", tc.name, note, group, tc.note, tc.group)
		}
		panel := tc.pal.SurfaceAlt
		if note == panel || group == panel || note == group {
			t.Errorf("%s: the panel %v, a note %v and an opened group %v are not three grounds", tc.name, panel, note, group)
		}
		away := func(c color.NRGBA) float64 {
			d := crossRefLightness(c) - crossRefLightness(panel)
			if d < 0 {
				d = -d
			}
			return d
		}
		if away(note) <= away(group) {
			t.Errorf("%s: a note's ground is %.1f from the panel's in lightness and an opened group's %.1f; the note takes the further", tc.name, away(note), away(group))
		}
	}
}

// THE RULE UNDER A LETTER is the letter's colour, quieter than the letter,
// and better than 4:1 against the note's ground in both themes: it is the
// one device that still shows where a ground's own edge is faint.
func TestTheRuleUnderALetterReadsOnItsNotesGround(t *testing.T) {
	for _, tc := range []struct {
		name string
		pal  palette
	}{{"light", lightPalette}, {"dark", darkPalette}} {
		rule, ground := crossRefStemColour(tc.pal), crossRefSectionGround(tc.pal)
		if rule.R != tc.pal.Accent.R || rule.G != tc.pal.Accent.G || rule.B != tc.pal.Accent.B || rule.A >= tc.pal.Accent.A {
			t.Errorf("%s: the rule is %v; want the letter's %v, quieter", tc.name, rule, tc.pal.Accent)
		}
		a := float64(rule.A) / 255
		mix := func(f, b uint8) uint8 { return uint8(float64(f)*a + float64(b)*(1-a) + 0.5) }
		drawn := color.NRGBA{R: mix(rule.R, ground.R), G: mix(rule.G, ground.G), B: mix(rule.B, ground.B), A: 255}
		if got := contrastRatio(drawn, ground); got < 4 {
			t.Errorf("%s: the rule reads %.2f:1 against the note's ground, want better than 4:1", tc.name, got)
		}
	}
}

// AN OPENED GROUP'S EDGE AND THE POINTER'S WASH, IN BOTH THEMES. An opened
// group's ground is the faintest step of the three (1.09:1 from a note's in
// the dark theme), so it has an edge that reads against the note's ground
// whatever its fill; and the wash under a desktop's pointer is a tint that
// shows alike on the panel's ground, a note's and an opened group's, where
// the panel's own colour showed on a note's ground and not on the panel's.
func TestAnOpenedGroupsEdgeAndThePointersWashShowOnEveryGround(t *testing.T) {
	over := func(f, b color.NRGBA) color.NRGBA {
		a := float64(f.A) / 255
		mix := func(f, b uint8) uint8 { return uint8(float64(f)*a + float64(b)*(1-a) + 0.5) }
		return color.NRGBA{R: mix(f.R, b.R), G: mix(f.G, b.G), B: mix(f.B, b.B), A: 255}
	}
	for _, tc := range []struct {
		name string
		pal  palette
	}{{"light", lightPalette}, {"dark", darkPalette}} {
		note, group := crossRefGrounds(tc.pal)
		edge := crossRefGroupEdge(tc.pal)
		if got := contrastRatio(edge, note); got < 1.4 {
			t.Errorf("%s: an opened group's edge reads %.2f:1 against the note's ground, want 1.4:1 or better", tc.name, got)
		}
		if got := contrastRatio(edge, group); got < 1.3 {
			t.Errorf("%s: an opened group's edge reads %.2f:1 against its own ground, want 1.3:1 or better", tc.name, got)
		}
		wash := crossRefHover(tc.pal)
		least, most := 0.0, 0.0
		for i, ground := range []color.NRGBA{tc.pal.SurfaceAlt, note, group} {
			got := contrastRatio(over(wash, ground), ground)
			if got < 1.15 {
				t.Errorf("%s: the pointer's wash reads %.3f:1 on ground %d of the panel's, a note's and a group's; want 1.15:1 or better", tc.name, got, i)
			}
			if i == 0 || got < least {
				least = got
			}
			if got > most {
				most = got
			}
		}
		if most/least > 1.05 {
			t.Errorf("%s: the pointer's wash reads from %.3f:1 to %.3f:1 across the three grounds; it should show alike on each", tc.name, least, most)
		}
	}
	// In the panel: every row and card that takes a tap has that wash, the
	// Treasury's cards and the publisher's alike, and the opened group's
	// ground has that edge.
	st, _ := groupListState(t)
	p := mountSectionPanel(t, st, 16, "John.3.16\tRom.5.1\t10\n")
	var taps []*tapCard
	walkTree(p.top, func(o fyne.CanvasObject) {
		if c, ok := o.(*tapCard); ok {
			taps = append(taps, c)
		}
	})
	// The first note's row, the two group rows, the first note's three
	// cards, and the Treasury's one (the lone group's card is made when it
	// first opens).
	if len(taps) != 7 || len(p.groups) != 2 {
		t.Fatalf("%d rows and cards take a tap in the panel (%d groups), want 7 and 2", len(taps), len(p.groups))
	}
	for i, c := range taps {
		if c.hoverBg != crossRefHover(p.pal) {
			t.Errorf("tap %d washes with %v under the pointer; the list's wash is %v", i, c.hoverBg, crossRefHover(p.pal))
		}
	}
	for gi, g := range p.groups {
		if g.shade.StrokeColor != crossRefGroupEdge(p.pal) || g.shade.StrokeWidth != 1 || g.shade.FillColor != crossRefGroupGround(p.pal) {
			t.Errorf("group %d: the opened group's ground is %v with an edge %v, %v wide; want %v with %v, 1 wide", gi, g.shade.FillColor, g.shade.StrokeColor, g.shade.StrokeWidth, crossRefGroupGround(p.pal), crossRefGroupEdge(p.pal))
		}
	}
}

// sectionPanel is the cross-references panel for one verse, open on the
// narrowest phone, and what these tests measure in it.
type sectionPanel struct {
	top      *widget.PopUp
	scroll   *container.Scroll
	sections []*fyne.Container
	notes    []*crossRefNoteRow
	groups   []*crossRefGroupRow
	pal      palette
}

func mountSectionPanel(t *testing.T, st *AppState, verse int, index string) sectionPanel {
	t.Helper()
	return mountSectionPanelAt(t, st, verse, index, fyne.NewSize(375, 667))
}

func mountSectionPanelAt(t *testing.T, st *AppState, verse int, index string, window fyne.Size) sectionPanel {
	t.Helper()
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, index)
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	w := test.NewWindow(widget.NewLabel("Reading"))
	t.Cleanup(w.Close)
	w.Resize(window)
	st.theme, st.window = th, w
	showCrossRefs(st, "", selSpan{lo: verse, hi: verse})
	top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok {
		t.Fatal("the cross-references panel did not open")
	}
	p := sectionPanel{top: top, pal: th.palette()}
	walkTree(top, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *container.Scroll:
			if p.scroll == nil {
				p.scroll = x
			}
		case *fyne.Container:
			if _, section := x.Layout.(crossRefSectionLayout); section {
				p.sections = append(p.sections, x)
			}
		case *crossRefNoteRow:
			p.notes = append(p.notes, x)
		case *crossRefGroupRow:
			p.groups = append(p.groups, x)
		}
	})
	return p
}

func atX(o fyne.CanvasObject) float32 {
	return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).X
}
func atY(o fyne.CanvasObject) float32 {
	return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).Y
}
func atRight(o fyne.CanvasObject) float32 { return atX(o) + o.Size().Width }
func atFoot(o fyne.CanvasObject) float32  { return atY(o) + o.Size().Height }
func standsOn(o, g fyne.CanvasObject) bool {
	return atX(o) >= atX(g) && atRight(o) <= atRight(g) && atY(o) >= atY(g) && atFoot(o) <= atFoot(g)
}

// within compares two drawn positions to half a point: a line's middle and a
// row's fall on different halves of a point.
func within(a, b float32) bool { d := a - b; return d > -0.5 && d < 0.5 }

// THE MEASURES, AS FIGURES AND AS WHAT THEY ARE FOR. The panel's other
// tests compare the layout with the constants it was laid out by, which
// holds whatever the constants are; here they are the figures decided, the
// letter stands alone in its margin at a card reference's size, the rule
// hangs from the letter inside the note's ground, and a note closed and
// opened again has its rule and its mark back.
func TestTheMeasuresOfANotesGroundAreTheOnesDecided(t *testing.T) {
	st, _ := groupListState(t)
	p := mountSectionPanel(t, st, 16, "John.3.16\tRom.5.8\t10\n")
	if len(p.sections) != 3 || len(p.notes) != 1 || len(p.groups) != 2 {
		t.Fatalf("%d grounds, %d note rows, %d group rows", len(p.sections), len(p.notes), len(p.groups))
	}
	// The figures every comparison in the panel's tests is written against.
	if crossRefSectionGap != 10 || crossRefSectionPadX != 8 || crossRefSectionPadY != 2 || crossRefSectionRadius != 10 ||
		crossRefLetterMargin != 18 || crossRefLetterSize != 16 || crossRefRowPad != 6 ||
		crossRefStemAt != 3 || crossRefStemWidth != 3 || crossRefStemGap != 5 || crossRefStemFoot != 6 {
		t.Errorf("the measures are gap %v, padding %v by %v, radius %v, margin %v, letter %v, row air %v, rule at %v, %v wide, %v under the line, %v above the foot; decided 11 November 2026: 10, 8 by 2, 10, 18, 16, 6, 3, 3, 5, 6",
			crossRefSectionGap, crossRefSectionPadX, crossRefSectionPadY, crossRefSectionRadius, crossRefLetterMargin, crossRefLetterSize, crossRefRowPad,
			crossRefStemAt, crossRefStemWidth, crossRefStemGap, crossRefStemFoot)
	}
	var reference *canvas.Text
	walkTree(p.top, func(o fyne.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok && tx.Text == "Romans 5:8" {
			reference = tx
		}
	})
	if reference == nil {
		t.Fatal("no card reference to hold the letter's size to")
	}
	drv := fyne.CurrentApp().Driver()
	a, b := p.notes[0], p.groups[1]
	ground := p.sections[0].Objects[0]
	// The letter alone in its margin: the printed note starts clear of it,
	// and it is the size of a card's reference, for a note's row and a lone
	// group's row alike.
	for _, pair := range []struct {
		letter *canvas.Text
		label  *crossRefPrinted
	}{{a.letter, a.label}, {b.letter, b.label}} {
		if atRight(pair.letter) > atX(pair.label) {
			t.Errorf("the letter %q runs to %v and the printed note starts at %v: the letter does not stand alone in its margin", pair.letter.Text, atRight(pair.letter), atX(pair.label))
		}
		if pair.letter.TextSize != reference.TextSize {
			t.Errorf("the letter %q is %v points; a card's reference is %v", pair.letter.Text, pair.letter.TextSize, reference.TextSize)
		}
	}
	// The rule hangs from the letter and stops short of the rows' foot,
	// inside the note's ground.
	_, own := drv.RenderedTextSize(a.letter.Text, a.letter.TextSize, a.letter.TextStyle, nil)
	last := a.pieces[len(a.pieces)-1]
	if atY(a.stem) <= atY(a.letter)+own || atFoot(a.stem) >= atFoot(last) || !standsOn(a.stem, ground) {
		t.Errorf("the rule under %q runs from %v to %v; the letter's line is at %v, the rows' foot at %v, the ground %v to %v", a.letter.Text, atY(a.stem), atFoot(a.stem), atY(a.letter)+own, atFoot(last), atY(ground), atFoot(ground))
	}
	// The letter and the printed line at the middle of the row's height, as
	// the icon is (a one-line lone group shows it: the second note).
	if mid, row := atY(b.head)+b.head.Size().Height/2, atY(b.header)+b.header.Size().Height/2; !within(mid, row) {
		t.Errorf("the second note's letter and printed line stand at %v in a row whose middle is %v", mid, row)
	}
	// Closed and opened again, the rule is back where it was.
	wasY, wasFoot, openIcon := atY(a.stem), atFoot(a.stem), a.icon.Resource.Name()
	test.Tap(a.header)
	closedIcon := a.icon.Resource.Name()
	// Closed, the letter and the note as printed are still on the row: read
	// from what is drawn, since a hidden line still holds its words.
	if !a.letter.Visible() || !a.label.Visible() || a.label.Size().Height <= 0 || !standsOn(a.label, p.sections[0].Objects[0]) {
		t.Errorf("closed: the letter visible %v, the printed note visible %v and %v tall", a.letter.Visible(), a.label.Visible(), a.label.Size().Height)
	}
	test.Tap(a.header)
	if !a.open || !a.stem.Visible() || atY(a.stem) != wasY || atFoot(a.stem) != wasFoot {
		t.Errorf("reopened: open %v, the rule visible %v from %v to %v; it ran from %v to %v", a.open, a.stem.Visible(), atY(a.stem), atFoot(a.stem), wasY, wasFoot)
	}
	if closedIcon == openIcon || a.icon.Resource.Name() != openIcon {
		t.Errorf("the note's mark is %q open, %q closed and %q reopened", openIcon, closedIcon, a.icon.Resource.Name())
	}
}

// noteShapesState is one verse whose five notes are the five shapes that
// are not a row over pieces: one citation, the reader's own verse, words the
// loaded text cannot follow, a group nothing of which can be followed, and a
// group that is the whole of its note.
func noteShapesState(t *testing.T) *AppState {
	t.Helper()
	bd := xrefBible(map[string]map[int]int{"John": {3: 36}, "Romans": {5: 21}})
	x := footnoteKindCrossref
	bd.Verses["John"][3][15].Footnotes = []Footnote{
		{Anchor: 0, Text: "Rom. 5:8", Kind: x, Caller: "-"},
		{Anchor: 10, Text: "John 3:16", Kind: x, Caller: "-"},
		{Anchor: 20, Text: "Ezek. 8:21", Kind: x, Caller: "-"},
		{Anchor: 30, Text: "(Hab. 2:4)", Kind: x, Caller: "-"},
		{Anchor: 35, Text: "(Rom. 5:8)", Kind: x, Caller: "-"},
	}
	return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
}

// EVERY SHAPE OF NOTE stands on a note's ground with its letter in the
// margin, on the line of its first row of text — not only a row over
// pieces.
func TestEveryShapeOfNoteStandsBesideItsLetterOnItsGround(t *testing.T) {
	p := mountSectionPanel(t, noteShapesState(t), 16, "John.3.16\tRom.5.8\t10\n")
	if len(p.sections) != 5 || len(p.notes) != 0 || len(p.groups) != 1 {
		t.Fatalf("%d grounds, %d note rows, %d group rows; want a ground for each of the five notes, no row over pieces, and the one lone group", len(p.sections), len(p.notes), len(p.groups))
	}
	drv := fyne.CurrentApp().Driver()
	for i, want := range []struct {
		shape, letter, first string
		size                 float32
		style                fyne.TextStyle
	}{
		{"one citation", "a", "Romans 5:8", 16, fyne.TextStyle{Bold: true}},
		{"the reader's own verse", "b", "John 3:16", crossRefGroupTextSize, fyne.TextStyle{}},
		{"words", "c", "Ezek. 8:21", crossRefGroupTextSize, fyne.TextStyle{}},
		{"a group nothing of which is followed", "d", "(Hab. 2:4)", crossRefGroupTextSize, fyne.TextStyle{}},
		{"a group that is its whole note", "e", "(Rom. 5:8)", crossRefGroupTextSize, fyne.TextStyle{}},
	} {
		section := p.sections[i]
		ground := section.Objects[0]
		var letter *canvas.Text
		var first fyne.CanvasObject
		var preview *widget.Label
		walkTree(section.Objects[1], func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *canvas.Text:
				if x.TextStyle.Bold && x.TextStyle.Italic && letter == nil {
					letter = x
				} else if x.Text == want.first {
					first = x
				}
			case *crossRefPrinted:
				if x.Text == want.first {
					first = x
				}
			case *widget.Label:
				preview = x
			}
		})
		if letter == nil || letter.Text != want.letter || first == nil {
			t.Errorf("%s: its letter %v, its first line found %v; want the letter %q and %q", want.shape, letter, first != nil, want.letter, want.first)
			continue
		}
		if got := atX(letter) - atX(ground); got != crossRefSectionPadX {
			t.Errorf("%s: the letter stands %v in from its ground's edge, want %v", want.shape, got, crossRefSectionPadX)
		}
		if got := atX(first) - atX(letter); got != crossRefLetterMargin {
			t.Errorf("%s: its first line stands %v beside the letter, want %v", want.shape, got, crossRefLetterMargin)
		}
		if preview != nil && atX(preview) != atX(first) {
			t.Errorf("%s: its preview starts at %v and its reference at %v: the preview does not stand beside the margin", want.shape, atX(preview), atX(first))
		}
		_, own := drv.RenderedTextSize(letter.Text, letter.TextSize, letter.TextStyle, nil)
		_, line := drv.RenderedTextSize("Ag", want.size, want.style, nil)
		if d := (atY(letter) + own) - (atY(first) + line); d < -0.5 || d > 0.5 {
			t.Errorf("%s: the letter stands %v points off the line of %q", want.shape, d, want.first)
		}
		if !standsOn(section.Objects[1], ground) || ground.(*canvas.Rectangle).FillColor != crossRefSectionGround(p.pal) {
			t.Errorf("%s does not stand on a note's ground", want.shape)
		}
	}
}

// EVERY LETTER SITS ON ITS FIRST LINE OUTSIDE THE PANEL'S THEME SCOPE. Every
// lettered shape sets its first row on a canvas.Text or crossRefPrinted, so
// crossRefLettered does not depend on passageCitationTheme's zero inner
// padding; only a card's widget.Label preview depends on that scope to align
// its inner text with the reference's left edge.
func TestEveryShapeOfNoteKeepsItsLetterOnTheLineOutsideTheListScope(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st := noteShapesState(t)
	st.theme = th
	st.Bible.Verses["John"][3][15].Footnotes = append(st.Bible.Verses["John"][3][15].Footnotes,
		Footnote{Anchor: 40, Text: "Rom. 5:8; (Rom. 5:9)", Kind: footnoteKindCrossref, Caller: "-"},
	)
	notes := publisherCrossRefsFor(st.Bible, "John", 3, versesBetween(st.Bible, "John", 3, 16, 16))
	if len(notes) != 6 {
		t.Fatalf("control: %d notes, want all six shapes", len(notes))
	}
	var blocks []fyne.CanvasObject
	for _, n := range notes {
		b, _, _, _ := publisherNoteBlock(st, n, st.pal(), func(crossRef) {}, &crossRefRelayout{})
		blocks = append(blocks, b)
	}
	w := test.NewWindow(container.New(squeezeWidthLayout{}, container.NewVBox(blocks...)))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(300, 600))
	drv := fyne.CurrentApp().Driver()
	for i, want := range []struct {
		shape, letter, first string
		size                 float32
		style                fyne.TextStyle
	}{
		{"one citation", "a", "Romans 5:8", 16, fyne.TextStyle{Bold: true}},
		{"the reader's own verse", "b", "John 3:16", crossRefGroupTextSize, fyne.TextStyle{}},
		{"words", "c", "Ezek. 8:21", crossRefGroupTextSize, fyne.TextStyle{}},
		{"a group nothing of which is followed", "d", "(Hab. 2:4)", crossRefGroupTextSize, fyne.TextStyle{}},
		{"a group that is its whole note", "e", "(Rom. 5:8)", crossRefGroupTextSize, fyne.TextStyle{}},
		{"several pieces", "f", "Rom. 5:8; (Rom. 5:9)", crossRefGroupTextSize, fyne.TextStyle{}},
	} {
		var letter *canvas.Text
		var first fyne.CanvasObject
		walkTree(blocks[i], func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *canvas.Text:
				if x.TextStyle.Bold && x.TextStyle.Italic && letter == nil {
					letter = x
				} else if x.Text == want.first {
					first = x
				}
			case *crossRefPrinted:
				if x.Text == want.first {
					first = x
				}
			}
		})
		if letter == nil || letter.Text != want.letter || first == nil {
			t.Fatalf("%s: letter %v, first %v; want %q and %q", want.shape, letter, first != nil, want.letter, want.first)
		}
		_, own := drv.RenderedTextSize(letter.Text, letter.TextSize, letter.TextStyle, nil)
		_, line := drv.RenderedTextSize("Ag", want.size, want.style, nil)
		if d := (atY(letter) + own) - (atY(first) + line); d < -0.5 || d > 0.5 {
			t.Errorf("%s outside listScope: the letter stands %v points off the line of %q", want.shape, d, want.first)
		}
	}
}

// A NOTE KEYED TO NO WORD has no letter. Its rows stand beside an empty
// margin, in line with the other notes', and its rule starts with its
// pieces.
func TestANoteKeyedToNoWordStandsBesideAnEmptyMargin(t *testing.T) {
	st := &AppState{Bible: displacedFixture(), CurrentBook: "Ruth", CurrentChapter: 3, CurrentVersion: "nkjv"}
	p := mountSectionPanel(t, st, 11, "")
	if len(p.sections) != 1 || len(p.notes) != 1 || p.notes[0].letter.Text != "" || len(p.notes[0].pieces) != 2 {
		t.Fatalf("%d grounds, %d note rows; want the one note keyed to no word, a row over its two pieces", len(p.sections), len(p.notes))
	}
	n, ground := p.notes[0], p.sections[0].Objects[0]
	beside := atX(ground) + crossRefSectionPadX + crossRefLetterMargin
	if atX(n.label) != beside {
		t.Errorf("the printed note starts at %v; beside an empty margin it starts at %v", atX(n.label), beside)
	}
	for i, piece := range n.pieces {
		if atX(piece) != beside {
			t.Errorf("row %d starts at %v; beside an empty margin it starts at %v", i, atX(piece), beside)
		}
	}
	first, last := n.pieces[0], n.pieces[len(n.pieces)-1]
	if !n.stem.Visible() || atY(n.stem) != atY(first) || !within(atFoot(n.stem), atFoot(last)-crossRefStemFoot) || atX(n.stem)-atX(ground) != crossRefSectionPadX+crossRefStemAt {
		t.Errorf("the rule: visible %v, from %v to %v at %v; its pieces run from %v to %v", n.stem.Visible(), atY(n.stem), atFoot(n.stem), atX(n.stem)-atX(ground), atY(first), atFoot(last))
	}
}

// THE ONE CARD OF A NOTE KEYED TO NO WORD stands in line with a lettered
// note's card. No note of the canon fetched on 6 November 2026 is this
// shape, so it is built from a note that is one citation.
func TestTheCardOfANoteKeyedToNoWordStandsInLineWithALetteredOnes(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st := noteShapesState(t)
	st.theme = th
	notes := publisherCrossRefsFor(st.Bible, "John", 3, versesBetween(st.Bible, "John", 3, 16, 16))
	lettered := notes[0]
	if _, alone := lettered.alone(); !alone {
		t.Fatal("control: the first note is not one citation")
	}
	unkeyed := lettered
	unkeyed.Index, unkeyed.Unkeyed = -1, true
	nop := func(crossRef) {}
	a, _, _, _ := publisherNoteBlock(st, lettered, st.pal(), nop, &crossRefRelayout{})
	b, _, _, _ := publisherNoteBlock(st, unkeyed, st.pal(), nop, &crossRefRelayout{})
	w := test.NewWindow(container.New(squeezeWidthLayout{}, container.NewVBox(a, b)))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(300, 400))
	var refs []*canvas.Text
	letters := 0
	walkTree(w.Content(), func(o fyne.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok {
			switch {
			case tx.Text == "Romans 5:8":
				refs = append(refs, tx)
			case tx.TextStyle.Bold && tx.TextStyle.Italic && tx.Text != "":
				letters++
			}
		}
	})
	if len(refs) != 2 || letters != 1 {
		t.Fatalf("%d references and %d letters; want the two cards, one lettered", len(refs), letters)
	}
	if atX(refs[0]) != atX(refs[1]) {
		t.Errorf("the lettered note's reference starts at %v and the unlettered one's at %v: not in line", atX(refs[0]), atX(refs[1]))
	}
}

// A NOTE THAT IS ONE PARENTHESIZED GROUP IS DRAWN AS A GROUP ROW. Decided
// 11 November 2026: a note that is one parenthesised group and nothing else
// starts closed as the same group row, at the same level and with the same
// formatting, as a parenthesised group inside a note of several pieces, while
// its letter stands in its margin and its tap spans the margin as a note's own
// row does.
func TestANoteThatIsOneGroupIsDrawnAsAGroupRow(t *testing.T) {
	st, _ := groupListState(t)
	p := mountSectionPanel(t, st, 16, "John.3.16\tRom.5.8\t10\n")
	if len(p.sections) != 3 || len(p.notes) != 1 || len(p.groups) != 2 {
		t.Fatalf("%d grounds, %d note rows, %d group rows", len(p.sections), len(p.notes), len(p.groups))
	}
	inner, b := p.groups[0], p.groups[1]
	ground := p.sections[1].Objects[0]
	if inner.open || b.open || inner.inset.Visible() || b.inset.Visible() || b.stem == nil || b.stem.Visible() {
		t.Fatalf("as the panel opens: inner open %v (inset %v), lone open %v (inset %v, stem %v)", inner.open, inner.inset.Visible(), b.open, b.inset.Visible(), b.stem != nil && b.stem.Visible())
	}
	// Labels at the same x and right edge, at 14 points in the muted colour.
	if atX(b.label) != atX(inner.label) || atRight(b.label) != atRight(inner.label) {
		t.Errorf("the lone group's label spans %v to %v; the inner group's %v to %v", atX(b.label), atRight(b.label), atX(inner.label), atRight(inner.label))
	}
	for name, lbl := range map[string]*crossRefPrinted{"inner": inner.label, "lone": b.label} {
		rows := test.WidgetRenderer(lbl).(*crossRefPrintedRenderer).rows
		if len(rows) == 0 {
			t.Fatalf("%s group draws no row", name)
		}
		for _, row := range rows {
			if row.TextSize != crossRefGroupTextSize || crossRefGroupTextSize != 14 || row.Color != p.pal.TextMuted {
				t.Errorf("%s group label row is %v points in %v; want 14 in %v", name, row.TextSize, row.Color, p.pal.TextMuted)
			}
		}
	}
	// Fold marks in the same box at the same right edge.
	box := fyne.NewSize(crossRefFoldIconSize, crossRefFoldIconSize)
	if b.icon.Size() != box || inner.icon.Size() != box || atX(b.icon) != atX(inner.icon) || atRight(b.icon) != atRight(inner.icon) {
		t.Errorf("the lone mark is %v at %v..%v; the inner mark is %v at %v..%v", b.icon.Size(), atX(b.icon), atRight(b.icon), inner.icon.Size(), atX(inner.icon), atRight(inner.icon))
	}
	// Taps at least 44 points tall; the lone group's tap spans the letter's
	// margin as a note's own row does.
	if b.header.Size().Height < 44 || inner.header.Size().Height < 44 {
		t.Errorf("taps are %v (lone) and %v (inner) tall; want at least 44", b.header.Size().Height, inner.header.Size().Height)
	}
	if atX(b.header) != atX(p.notes[0].header) || atRight(b.header) != atRight(inner.header) {
		t.Errorf("the lone group's tap spans %v..%v; the note's row starts at %v and the inner tap ends at %v", atX(b.header), atRight(b.header), atX(p.notes[0].header), atRight(inner.header))
	}
	// The lone group's letter stands crossRefSectionPadX in from its ground,
	// on the label's first line.
	if b.letter == nil || b.letter.Text != "b" {
		t.Fatalf("the lone group's letter is %v, want b", b.letter)
	}
	if got := atX(b.letter) - atX(ground); got != crossRefSectionPadX {
		t.Errorf("the lone group's letter stands %v in from its ground, want %v", got, crossRefSectionPadX)
	}
	drv := fyne.CurrentApp().Driver()
	_, own := drv.RenderedTextSize(b.letter.Text, b.letter.TextSize, b.letter.TextStyle, nil)
	_, line := drv.RenderedTextSize("Ag", crossRefGroupTextSize, fyne.TextStyle{}, nil)
	if d := (atY(b.letter) + own) - (atY(b.label) + line); !within(d, 0) {
		t.Errorf("the lone group's letter stands %v points off the label's first line", d)
	}
	// Once opened, wells at the same x and right edge, with the same fill and edge.
	test.Tap(inner.header)
	test.Tap(b.header)
	if !inner.open || !b.open || !inner.inset.Visible() || !b.inset.Visible() {
		t.Fatalf("after tapping both: inner open %v, lone open %v", inner.open, b.open)
	}
	if atX(b.shade) != atX(inner.shade) || atRight(b.shade) != atRight(inner.shade) {
		t.Errorf("the lone well spans %v to %v; the inner well %v to %v", atX(b.shade), atRight(b.shade), atX(inner.shade), atRight(inner.shade))
	}
	if b.shade.FillColor != inner.shade.FillColor || b.shade.FillColor != crossRefGroupGround(p.pal) ||
		b.shade.StrokeColor != inner.shade.StrokeColor || b.shade.StrokeColor != crossRefGroupEdge(p.pal) ||
		b.shade.StrokeWidth != 1 || inner.shade.StrokeWidth != 1 {
		t.Errorf("the lone well is %v (edge %v, %v) and the inner well %v (edge %v, %v)", b.shade.FillColor, b.shade.StrokeColor, b.shade.StrokeWidth, inner.shade.FillColor, inner.shade.StrokeColor, inner.shade.StrokeWidth)
	}
}

// THE WIDTH WITH A GROUP OPEN INSIDE A NOTE. The fixture holds one, the
// loop that opens every group is shown to have opened it, and the three
// layouts of a note report the width they take: a layout that under-reports
// would let a row past the list's edge without any object's MinSize saying
// so.
func TestTheListFitsTheNarrowestPhoneWithAGroupOpenInsideANote(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	limit := narrowestPhoneBody(t)
	st, sel := groupListState(t)
	st.theme = th
	lst := buildCrossRefList(st, sel, nil, nil, st.pal(), func(crossRef) {})
	opened := 0
	for _, g := range groupRowsIn(lst.Objects) {
		if g.header != nil {
			g.toggle()
			opened++
		}
	}
	if opened != 2 {
		t.Fatalf("control: %d groups opened; want the one inside the first note and the lone group", opened)
	}
	if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
		t.Errorf("groups open: %v", problems)
	}
	// The measure itself: each layout wants what it holds and what it takes.
	for _, o := range lst.Objects {
		section, ok := o.(*fyne.Container)
		if !ok {
			continue
		}
		if _, is := section.Layout.(crossRefSectionLayout); !is {
			continue
		}
		if got, want := section.MinSize().Width, section.Objects[1].MinSize().Width+2*crossRefSectionPadX; got != want {
			t.Errorf("a note's ground wants %v points for a block that wants %v and %v of padding", got, section.Objects[1].MinSize().Width, 2*crossRefSectionPadX)
		}
	}
	for _, n := range noteRowsIn(lst.Objects) {
		if got, want := n.head.MinSize().Width, n.label.MinSize().Width+crossRefLetterMargin; got != want {
			t.Errorf("the letter and the printed note want %v points; the printed note wants %v and the margin %v", got, n.label.MinSize().Width, crossRefLetterMargin)
		}
		body, head := n.body.MinSize().Width+crossRefLetterMargin, n.header.MinSize().Width
		if body <= head {
			t.Fatalf("control: the rows under %q want %v with their margin, no more than its own row's %v", n.letter.Text, body, head)
		}
		if got := n.MinSize().Width; got != body {
			t.Errorf("the note under %q wants %v points; its rows want %v beside a margin of %v", n.letter.Text, got, n.body.MinSize().Width, crossRefLetterMargin)
		}
	}
	lone := groupRowsIn(lst.Objects)[1]
	if got, want := lone.head.MinSize().Width, lone.label.MinSize().Width+crossRefLetterMargin; got != want {
		t.Errorf("the lone group's letter and label want %v points; the label wants %v and the margin %v", got, lone.label.MinSize().Width, crossRefLetterMargin)
	}
	if got, want := lone.inset.MinSize().Width, lone.inset.Objects[0].MinSize().Width+crossRefLetterMargin; got != want {
		t.Errorf("the lone group's inset wants %v points; its well wants %v and the margin %v", got, lone.inset.Objects[0].MinSize().Width, crossRefLetterMargin)
	}
	if inset, head := lone.inset.MinSize().Width, lone.header.MinSize().Width; inset <= head || lone.MinSize().Width != inset {
		t.Errorf("the open lone group wants %v (inset %v, header %v)", lone.MinSize().Width, inset, head)
	}
}

// A GROUP'S FIRST OPENING LEAVES THE LIST AS IT WANTS TO BE. A group's cards
// are made at the tap, after the list's theme scope took in the rows that
// stood there, so a layout pass that ran before the scope took them in
// measured them at the app's sizes and left the heights of that measure
// standing: a note's ground taller than its rows with its rule past their
// foot where the list is shorter than the scroll, and the scroll's content
// longer than the list where it is taller. Both are held here as the tap
// leaves them, with no further frame.
func TestAGroupsFirstOpeningLeavesTheListAsItWants(t *testing.T) {
	short := func(t *testing.T) *AppState {
		bd := xrefBible(map[string]map[int]int{"John": {3: 36}, "Romans": {5: 21}, "1 John": {4: 21}})
		text := "Rom. 5:8; (1 John 4:9)"
		bd.Verses["John"][3][15].Footnotes = []Footnote{{Anchor: 0, Text: text, Kind: footnoteKindCrossref, Caller: "-",
			Refs: []NoteRef{noteRefAt(t, text, "Rom. 5:8", "ROM.5.8")}}}
		return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
	}
	long := func(t *testing.T) *AppState {
		st, _ := groupListState(t)
		return st
	}
	for _, tc := range []struct {
		name      string
		state     func(*testing.T) *AppState
		window    fyne.Size
		groupIdx  int
		sectionAt int
		taller    bool // the list, than the scroll, once the group is open
	}{
		{"a list shorter than the scroll", short, fyne.NewSize(402, 874), 0, 0, false},
		{"a list taller than the scroll", long, fyne.NewSize(375, 667), 0, 0, true},
		{"a lone group", long, fyne.NewSize(375, 667), 1, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := mountSectionPanelAt(t, tc.state(t), 16, "", tc.window)
			if len(p.groups) <= tc.groupIdx || p.groups[tc.groupIdx].header == nil || p.scroll == nil {
				t.Fatalf("%d group rows; want group %d and the list's scroll", len(p.groups), tc.groupIdx)
			}
			g := p.groups[tc.groupIdx]
			test.Tap(g.header)
			if !g.open || len(g.members.Objects) == 0 {
				t.Fatal("the tap did not open the group")
			}
			wants, has := p.scroll.Content.MinSize().Height, p.scroll.Content.Size().Height
			if got := wants > p.scroll.Size().Height; got != tc.taller {
				t.Fatalf("control: the list wants %v in a scroll %v tall; this case is %s", wants, p.scroll.Size().Height, tc.name)
			}
			if tc.taller && !within(has, wants) {
				t.Errorf("the scroll's content is %v tall and the list wants %v", has, wants)
			}
			section := p.sections[tc.sectionAt]
			if got, want := section.Size().Height, section.MinSize().Height; !within(got, want) {
				t.Errorf("section %d's ground and gap are %v tall and the note wants %v", tc.sectionAt, got, want)
			}
			if tc.sectionAt == 0 {
				n := p.notes[0]
				last := n.pieces[len(n.pieces)-1]
				if got, want := atFoot(n.stem), atFoot(last)-crossRefStemFoot; !within(got, want) {
					t.Errorf("the rule ends at %v; %v above the foot of the note's rows is %v", got, crossRefStemFoot, want)
				}
			} else if g.stem != nil {
				if got, want := atFoot(g.stem), atFoot(g.shade)-crossRefStemFoot; !g.stem.Visible() || !within(got, want) {
					t.Errorf("the lone group's rule: visible %v, ends at %v; %v above the well's foot is %v", g.stem.Visible(), got, crossRefStemFoot, want)
				}
			}
			// The cards the tap made are drawn at the list's sizes.
			var preview *widget.Label
			walkTree(g.members, func(o fyne.CanvasObject) {
				if l, ok := o.(*widget.Label); ok && preview == nil {
					preview = l
				}
			})
			if preview == nil {
				t.Fatal("no preview among the group's cards")
			}
			if got := preview.Theme().Size(theme.SizeNameText); got != 17 {
				t.Errorf("a card the tap made reads its text size as %v; the list's is 17", got)
			}
		})
	}
}

// THE TREASURY KEEPS EACH SOURCE RELATIONSHIP. Direct citations and closed
// publisher groups do not remove the Treasury's own references.
func TestTheTreasuryKeepsWhatAClosedGroupCites(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := groupListState(t)
	refs := []crossRef{
		{Book: "Romans", Chapter: 5, Verse: 8, Votes: 9}, // the first note's direct citation: stays
		{Book: "1 John", Chapter: 4, Verse: 9, Votes: 5}, // inside the first note's closed group: stays
		{Book: "Isaiah", Chapter: 9, Verse: 6, Votes: 3}, // inside the second note's closed lone group: stays
	}
	lst := buildCrossRefList(st, sel, refs, nil, lightPalette, func(crossRef) {})
	if got := cardLabelsIn(lst.Objects); lst.TSKRows != 3 || joined(got) != "Romans 5:8|Ephesians 2:4|2 Thessalonians 2:16|Romans 5:8|1 John 4:9|Isaiah 9:6" {
		t.Errorf("%d Treasury rows, cards %q; want all three Treasury destinations after the first note's three cards", lst.TSKRows, got)
	}
}

// THE CANON, EVERY GROUP OPEN. Every verse of a fetched canon that has a
// note, built as a selection of its own with every group open, wants no more
// than the narrowest phone gives the list. It runs when BIBLETEXT_RENDER_XREFS
// names a canon, as the render harness does; over the one fetched on
// 6 November 2026 it measures 21,266 selections and opens 4,850 groups.
func TestTheCanonFitsTheNarrowestPhoneWithEveryGroupOpen(t *testing.T) {
	src := os.Getenv("BIBLETEXT_RENDER_XREFS")
	if src == "" {
		t.Skip("BIBLETEXT_RENDER_XREFS not set — canon width skipped")
	}
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var bd BibleData
	if err := json.Unmarshal(body, &bd); err != nil {
		t.Fatal(err)
	}
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	limit := narrowestPhoneBody(t)
	st := &AppState{Bible: &bd, CurrentVersion: "nkjv", theme: th}
	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		selections, opened, misfits := 0, 0, 0
		for _, book := range bd.Books {
			for _, ch := range bd.GetChapterNumbersForBook(book) {
				for _, v := range bd.GetChapter(book, ch) {
					sel := []Verse{v}
					if len(publisherCrossRefsFor(&bd, book, ch, sel)) == 0 {
						continue
					}
					selections++
					st.CurrentBook, st.CurrentChapter = book, ch
					lst := buildCrossRefList(st, sel, nil, nil, st.pal(), func(crossRef) {})
					for _, g := range groupRowsIn(lst.Objects) {
						if g.header != nil {
							g.toggle()
							opened++
						}
					}
					if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
						misfits++
						if misfits <= 5 {
							t.Errorf("%s %s %d:%d, groups open: %s", sizeOpt.ID, book, ch, v.Verse, problems[0])
						}
					}
					if lst.Passage && len(lst.Objects) > 0 {
						if c, ok := lst.Objects[0].(*fyne.Container); ok && len(c.Objects) > 0 {
							if p, ok := c.Objects[0].(*readingParagraph); ok {
								p.Resize(fyne.NewSize(limit, 10))
								for _, tx := range test.WidgetRenderer(p).(*readingParagraphRenderer).objs {
									if edge := tx.Position().X + tx.Size().Width; edge > limit+0.5 {
										misfits++
										if misfits <= 5 {
											t.Errorf("%s %s %d:%d: passage row reaches %.1f points, over %.0f", sizeOpt.ID, book, ch, v.Verse, edge, limit)
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
		if selections == 0 || opened == 0 {
			t.Fatalf("control (%s): %d selections with a note, %d groups opened", sizeOpt.ID, selections, opened)
		}
		t.Logf("%s: %d selections with a note, %d groups opened, %d over %.0f points", sizeOpt.ID, selections, opened, misfits, limit)
	}

	// When the Treasury zip is also cached locally, build full three-source lists
	// (publisher + Gospel parallels + Treasury, including cross-book Treasury
	// spans and cross-book overlap cases) and verify both width and cross-book
	// publisher filtering against the real canon.
	if zipBytes, err := os.ReadFile(realCachePath(crossRefCachePath())); err == nil {
		if idx, _, err := parseCrossRefZip(zipBytes); err == nil {
			crossRefMu.Lock()
			was := crossRefIndex
			crossRefIndex = idx
			crossRefMu.Unlock()
			t.Cleanup(func() {
				crossRefMu.Lock()
				crossRefIndex = was
				crossRefMu.Unlock()
			})
			for _, sizeOpt := range textSizeOptions {
				setReadingTextSizeID(sizeOpt.ID)
				for _, tc := range []struct {
					book        string
					ch, v       int
					presentCard string
					absentCard  string
				}{
					{"Ezekiel", 36, 3, "Jeremiah 52:1–Lamentations 1:5", ""}, // 30-char cross-book Treasury row (wraps at Extra large)
					{"Numbers", 3, 1, "Leviticus 27:34–Numbers 1:1", ""},     // 27-char cross-book Treasury row
					{"Matthew", 3, 1, "", ""},                                // publisher + Gospel parallels + Treasury
					{"Ezra", 4, 3, "", "2 Chronicles 36:22–Ezra 1:3"},        // overlapped by publisher Ezra 1:1–4
					{"Ezra", 6, 3, "", "2 Chronicles 36:22–Ezra 1:4"},        // overlapped by publisher Ezra 1:1
					{"Daniel", 6, 28, "", "2 Chronicles 36:22–Ezra 1:2"},     // overlapped by publisher Ezra 1:1, 2
				} {
					st.CurrentBook, st.CurrentChapter = tc.book, tc.ch
					sel := versesBetween(&bd, tc.book, tc.ch, tc.v, tc.v)
					refs := crossRefsForSelection(st, "", selSpan{lo: tc.v, hi: tc.v})
					lst := buildCrossRefList(st, sel, refs, nil, st.pal(), func(crossRef) {})
					for _, g := range groupRowsIn(lst.Objects) {
						if g.header != nil {
							g.toggle()
						}
					}
					if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
						t.Errorf("%s full list %s %d:%d: %s", sizeOpt.ID, tc.book, tc.ch, tc.v, strings.Join(problems, "; "))
					}
					cards := cardLabelsIn(lst.Objects)
					if tc.presentCard != "" && !slices.Contains(cards, tc.presentCard) {
						t.Errorf("%s %s %d:%d missing expected cross-book Treasury card %q",
							sizeOpt.ID, tc.book, tc.ch, tc.v, tc.presentCard)
					}
					if tc.absentCard != "" && slices.Contains(cards, tc.absentCard) {
						t.Errorf("%s %s %d:%d retained overlapping cross-book Treasury card %q",
							sizeOpt.ID, tc.book, tc.ch, tc.v, tc.absentCard)
					}
				}
			}
		}
	}
}

// FLUSH FRAGMENTS IN THE PASSAGE STAY INSIDE THE LIST'S WIDTH. A quotation
// mark before an italic supplied word, a word split by a raised letter, a
// letter before a quoted supplied word, and a colour change before closing
// punctuation are drawn flush with no space between them, so a wrap must break
// before the whole flush chain rather than leave its tail past the right edge.
// The verse-of-the-day card keeps its existing wrap (holdFlush == false).
func TestThePassageWrapsFlushFragmentsWithinTheListsWidth(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	pal := th.palette()

	// Store-build verse-of-the-day paragraphs leave holdFlush off.
	votd := newReadingParagraph([]cardRun{{Text: "Synthetic card words."}}, 17, pal.Text, pal.RedLetter, styledPaneFont(), cardItalicFont())
	if votd.holdFlush {
		t.Fatal("newReadingParagraph turned holdFlush on for the verse-of-the-day card")
	}

	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 2}, "Romans": {5: 21}})
	v := &bd.Verses["Genesis"][1][0]
	// Synthetic words only:
	//   - ‘suppliedword,’ has an upright opening quote flush before an italic word
	//   - firstword+mark+secondword has a raised letter flush inside a run-together pair
	//   - mark+‘anotheritalic’ has a raised letter flush before an upright quote and italic word
	v.Text = "Alpha beta gamma delta ‘suppliedword,’ epsilon zeta firstwordsecondword eta ‘anotheritalic’ theta."
	q1 := len([]rune("Alpha beta gamma delta ‘"))
	q1End := q1 + len([]rune("suppliedword,"))
	midMark := len([]rune("Alpha beta gamma delta ‘suppliedword,’ epsilon zeta firstword"))
	q2Mark := len([]rune("Alpha beta gamma delta ‘suppliedword,’ epsilon zeta firstwordsecondword eta "))
	q2 := q2Mark + len([]rune("‘"))
	q2End := q2 + len([]rune("anotheritalic"))
	v.Supplied = []TextSpan{{Start: q1, End: q1End}, {Start: q2, End: q2End}}
	v.Footnotes = []Footnote{
		{Anchor: midMark, Text: "Rom. 5:8", Kind: footnoteKindCrossref, Caller: "-"},
		{Anchor: q2Mark, Text: "Rom. 5:9", Kind: footnoteKindCrossref, Caller: "-"},
	}
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "nkjv", theme: th}
	sel := []Verse{*v}
	notes := publisherCrossRefsFor(bd, "Genesis", 1, sel)
	row := newCrossRefPassage(st, "Genesis", 1, sel, notes, pal)
	p := row.(*fyne.Container).Objects[0].(*readingParagraph)
	if !p.holdFlush {
		t.Fatal("newCrossRefPassage did not set holdFlush on the passage")
	}
	if got := passagePlainText(p.runs); got != v.Text {
		t.Fatalf("passagePlainText = %q, want %q", got, v.Text)
	}

	// Also exercise a red-to-black flush boundary before closing punctuation,
	// using synthetic runs with holdFlush enabled as newCrossRefPassage sets it.
	redRuns := passageRuns([]passageUnit{{
		Runs: []verseRun{
			{Text: "Alpha beta gamma delta spokenword", Red: true},
			{Text: "; epsilon zeta.", Red: false},
		},
	}})
	redPara := newReadingParagraph(redRuns, readingGlyphSize(crossRefPassageTextSize), pal.Text, pal.RedLetter, styledPaneFont(), cardItalicFont())
	redPara.holdFlush = true

	for _, tc := range []struct {
		name string
		para *readingParagraph
	}{
		{"passage", p},
		{"red-to-black", redPara},
	} {
		for w := float32(200); w <= 283; w += 2 {
			tc.para.Resize(fyne.NewSize(w, 10))
			for _, tx := range test.WidgetRenderer(tc.para).(*readingParagraphRenderer).objs {
				if edge := tx.Position().X + tx.Size().Width; edge > w+0.5 {
					t.Fatalf("%s at width %.0f: %q reaches %.1f, over %.0f", tc.name, w, tx.(*canvas.Text).Text, edge, w)
				}
			}
		}
	}
}

// REOPENING ACROSS APPEARANCE AND LAYOUT REBUILDS KEEPS FOLDS AND PLACE. Both
// a light/dark switch (observeAppearance) and a rotation or window-layout
// rebuild (rebuildForLayoutChange) replace the panel with one in the new
// palette or size while keeping the notes and groups the reader had opened or
// closed and clamping the scroll offset to the new list's range.
func TestThePanelKeepsItsOpenRowsAndPlaceAcrossARebuild(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "John.3.16\tRom.5.1\t12\nJohn.3.16\tRom.5.2\t11\nJohn.3.16\tRom.5.3\t10\nJohn.3.16\tRom.5.4\t9\nJohn.3.16\tRom.5.5\t8\nJohn.3.16\tRom.5.6\t7\n")
	previousRun, previousLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = previousRun, previousLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }

	liveVariant := theme.VariantLight
	previousVariant := appearanceVariant
	appearanceVariant = func(*AppState) fyne.ThemeVariant { return liveVariant }
	t.Cleanup(func() { appearanceVariant = previousVariant })

	st, _ := groupListState(t)
	st.app = fyne.CurrentApp()
	st.theme = th
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(375, 667))
	st.window = w
	w.SetContent(CreateMainUI(st.app, st, w))
	st.appearance.built = theme.VariantLight

	inspect := func() (top *widget.PopUp, scroll *container.Scroll, notes []*crossRefNoteRow, groups []*crossRefGroupRow, section *fyne.Container) {
		t.Helper()
		var ok bool
		top, ok = w.Canvas().Overlays().Top().(*widget.PopUp)
		if !ok || top == nil {
			t.Fatal("the cross-references panel is not on top of the canvas")
		}
		walkTree(top, func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *container.Scroll:
				if scroll == nil {
					scroll = x
				}
			case *fyne.Container:
				if section == nil {
					if _, s := x.Layout.(crossRefSectionLayout); s {
						section = x
					}
				}
			case *crossRefNoteRow:
				notes = append(notes, x)
			case *crossRefGroupRow:
				groups = append(groups, x)
			}
		})
		if scroll == nil || len(notes) != 1 || len(groups) != 2 || section == nil {
			t.Fatalf("panel holds scroll=%v, %d notes, %d groups, section=%v; want 1 note and 2 groups", scroll != nil, len(notes), len(groups), section != nil)
		}
		return top, scroll, notes, groups, section
	}

	showCrossRefs(st, "", selSpan{lo: 16, hi: 16})
	first, scroll, _, groups, _ := inspect()
	// Open both the inner group inside note a and the lone group row of note b,
	// then scroll down into the list.
	test.Tap(groups[0].header)
	test.Tap(groups[1].header)
	scroll.Offset = fyne.NewPos(0, 140)
	scroll.Refresh()

	// 1. Light -> dark switch rebuilds in the dark palette and keeps both
	// groups open and the scroll at 140.
	liveVariant = theme.VariantDark
	observeAppearance(st, appearanceChanged)
	second, scroll, notes, groups, section := inspect()
	if second == first || first.Visible() || len(w.Canvas().Overlays().List()) != 1 {
		t.Fatal("the appearance rebuild did not replace the panel cleanly")
	}
	if got, want := section.Objects[0].(*canvas.Rectangle).FillColor, crossRefSectionGround(darkPalette); got != want {
		t.Errorf("reopened note ground is %v, want dark palette's %v", got, want)
	}
	if !notes[0].open || !groups[0].open || !groups[1].open || len(groups[0].members.Objects) != 3 || len(groups[1].members.Objects) != 1 {
		t.Errorf("after appearance rebuild: note open=%v, groups open=(%v with %d, %v with %d); want true, (true with 3, true with 1)",
			notes[0].open, groups[0].open, len(groups[0].members.Objects), groups[1].open, len(groups[1].members.Objects))
	}
	if scroll.Offset.Y != 140 {
		t.Errorf("after appearance rebuild: scroll offset Y = %v, want 140", scroll.Offset.Y)
	}
	if notes[0].pin == nil || !notes[0].pin.Visible() || notes[0].letter.Visible() {
		t.Errorf("after appearance rebuild at scroll 140: note a pin visible=%v, own letter visible=%v; want pinned immediately",
			notes[0].pin != nil && notes[0].pin.Visible(), notes[0].letter.Visible())
	}

	// 2. Close note a, set scroll to 95, and rotate/resize via rebuildForLayoutChange.
	test.Tap(notes[0].header)
	scroll.Offset = fyne.NewPos(0, 95)
	scroll.Refresh()
	w.Resize(fyne.NewSize(667, 375))
	rebuildForLayoutChange(st)
	third, scroll, notes, groups, _ := inspect()
	if third == second || second.Visible() || len(w.Canvas().Overlays().List()) != 1 {
		t.Fatal("the layout rebuild did not replace the panel cleanly")
	}
	if notes[0].open || !groups[0].open || !groups[1].open {
		t.Errorf("after layout rebuild: note open=%v, groups open=(%v, %v); want false, (true, true)",
			notes[0].open, groups[0].open, groups[1].open)
	}
	if scroll.Offset.Y != 95 {
		t.Errorf("after layout rebuild: scroll offset Y = %v, want 95", scroll.Offset.Y)
	}

	// 3. Capture an offset beyond what a taller viewport needs: scroll.Refresh
	// after restoring Offset clamps it to the new content's range rather than
	// stranding past the foot.
	w.Resize(fyne.NewSize(402, 874))
	scroll.Offset = fyne.NewPos(0, 900)
	rebuildForLayoutChange(st)
	_, scroll, _, _, _ = inspect()
	maxY := max(float32(0), scroll.Content.MinSize().Height-scroll.Size().Height)
	if scroll.Offset.Y != maxY || scroll.Offset.Y >= 900 {
		t.Errorf("clamped scroll offset Y = %v, want %v (< 900)", scroll.Offset.Y, maxY)
	}
}

// A LETTER STAYS IN VIEW WHILE ITS NOTE OR OPENED LONE GROUP RUNS PAST THE TOP
// OF THE LIST. A multi-piece note or an opened lone parenthesised group can be
// taller than the list; with its own row scrolled out above, its letter stands
// at crossRefPinTop under the top of the view, in the margin, with the rule
// hanging from it there, leaves with the foot of the rows, returns to its own
// row at the top, and is restored immediately when a scrolled panel reopens.
func TestALetterStaysInViewWhileItsNoteRunsPastTheTopOfTheList(t *testing.T) {
	st, _ := groupListState(t)
	// Give the lone group (note b) several citations so that when tapped open
	// it is taller than the view and exercises lone-group letter pinning too.
	v := &st.Bible.Verses["John"][3][15]
	v.Footnotes[1].Text = "(Is. 9:6; Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; 1 John 4:9, 10; Rev. 1:5)"
	// Add trailing Treasury rows so the scroll has ample travel past note b's foot.
	tsk := "John.3.16\tJohn.3.1\t9\nJohn.3.16\tJohn.3.2\t8\nJohn.3.16\tJohn.3.3\t7\nJohn.3.16\tJohn.3.4\t6\nJohn.3.16\tJohn.3.5\t5\nJohn.3.16\tJohn.3.6\t4\n"
	p := mountSectionPanel(t, st, 16, tsk)
	if len(p.notes) != 1 || len(p.groups) != 2 || p.scroll == nil {
		t.Fatalf("%d note rows, %d group rows, scroll=%v; want 1 note, 2 groups, and scroll", len(p.notes), len(p.groups), p.scroll != nil)
	}
	a := p.notes[0]
	inner, b := p.groups[0], p.groups[1]
	if crossRefPinTop != 4 {
		t.Errorf("a letter kept in view stands %v under the top of the view; built at 4", crossRefPinTop)
	}
	line := a.letterLine()
	to := func(y float32) {
		t.Helper()
		p.scroll.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.Delta{DY: p.scroll.Offset.Y - y}})
		if !within(p.scroll.Offset.Y, y) {
			t.Fatalf("control: the list is scrolled to %v, not %v (it runs to %v)", p.scroll.Offset.Y, y, p.scroll.Content.Size().Height-p.scroll.Size().Height)
		}
	}
	view := atY(p.scroll)
	own := atY(a.letter) - view + p.scroll.Offset.Y
	ownX := atX(a.letter)
	rule, foot := atY(a.stem), atFoot(a.stem)

	// 1. As the panel opens, the letter stands on its own row.
	if a.pinned != 0 || a.pin != nil || !a.letter.Visible() {
		t.Fatalf("as the panel opens: kept at %v, pin=%v, own visible=%v", a.pinned, a.pin != nil, a.letter.Visible())
	}
	// 2. Scrolled while the letter's own line is still below the view's top.
	to(own - crossRefPinTop - 2)
	if a.pinned != 0 || !a.letter.Visible() {
		t.Errorf("with own row still in view: kept at %v, own visible=%v", a.pinned, a.letter.Visible())
	}
	// 3. Scrolled into note a: pinned at crossRefPinTop with the rule beneath it.
	to(own + 60)
	if a.pin == nil || !a.pin.Visible() || a.letter.Visible() {
		t.Fatalf("scrolled into note a: pin visible=%v, own visible=%v", a.pin != nil && a.pin.Visible(), a.letter.Visible())
	}
	if a.pin.Text != "a" || a.pin.TextSize != a.letter.TextSize || a.pin.TextStyle != a.letter.TextStyle || a.pin.Color != a.letter.Color {
		t.Errorf("pin is %q at %v in %v; own is %q at %v in %v", a.pin.Text, a.pin.TextSize, a.pin.Color, a.letter.Text, a.letter.TextSize, a.letter.Color)
	}
	if got := atY(a.pin) - view; !within(got, crossRefPinTop) {
		t.Errorf("pin stands %v under view top, want %v", got, crossRefPinTop)
	}
	if atX(a.pin) != ownX {
		t.Errorf("pin X = %v, own X = %v", atX(a.pin), ownX)
	}
	if got, want := atY(a.stem), atY(a.pin)+line+crossRefStemGap; !within(got, want) {
		t.Errorf("rule starts at %v, want %v", got, want)
	}
	if got, want := atFoot(a.stem), foot-(own+60); !within(got, want) {
		t.Errorf("rule ends at %v, want %v", got, want)
	}
	// Opening an inner group updates the note's height and keeps the letter pinned.
	inner.toggle()
	if got := atY(a.pin) - view; !a.pin.Visible() || !within(got, crossRefPinTop) {
		t.Errorf("with inner group open: pin at %v under view top, visible=%v", got, a.pin.Visible())
	}
	inner.toggle()
	// 4. Scrolled to note a's foot: clamped above the rule's foot.
	last := a.pieces[len(a.pieces)-1]
	to(p.scroll.Offset.Y + atFoot(last) - view - 8)
	if got, want := atY(a.pin)+line+crossRefStemGap, atFoot(last)-crossRefStemFoot; !within(got, want) {
		t.Errorf("at note a's foot: pin+line+gap = %v, rule foot = %v", got, want)
	}
	if atY(a.pin) >= view+crossRefPinTop {
		t.Errorf("at note a's foot: pin stands at %v, did not leave with rows (%v)", atY(a.pin), view+crossRefPinTop)
	}
	// 5. Back at top: unpinned, rule restored, and closing note a keeps it unpinned.
	to(0)
	if a.pinned != 0 || a.pin.Visible() || !a.letter.Visible() {
		t.Errorf("back at top: pinned=%v, pin visible=%v, own visible=%v", a.pinned, a.pin.Visible(), a.letter.Visible())
	}
	if !within(atY(a.stem), rule) || !within(atFoot(a.stem), foot) {
		t.Errorf("back at top rule = %v..%v, want %v..%v", atY(a.stem), atFoot(a.stem), rule, foot)
	}
	test.Tap(a.header)
	to(own + 20)
	if a.pinned != 0 || a.pin.Visible() || !a.letter.Visible() {
		t.Errorf("closed note a: pinned=%v, pin visible=%v, own visible=%v", a.pinned, a.pin.Visible(), a.letter.Visible())
	}
	test.Tap(a.header)
	to(0)

	// 6. Lone parenthesised group (note b): closed has nothing pinned; opened
	// and scrolled into pins b's letter at crossRefPinTop above b's stem and
	// clamps at b's well foot.
	bOwn := atY(b.letter) - view + p.scroll.Offset.Y
	to(bOwn + 20)
	if b.pinned != 0 || b.pin != nil || !b.letter.Visible() {
		t.Fatalf("closed lone group b: pinned=%v, pin=%v, own visible=%v", b.pinned, b.pin != nil, b.letter.Visible())
	}
	to(0)
	test.Tap(b.header)
	bOwn = atY(b.letter) - view + p.scroll.Offset.Y
	bOwnX := atX(b.letter)
	bLine := b.letterLine()
	to(bOwn + 80)
	if b.pin == nil || !b.pin.Visible() || b.letter.Visible() {
		t.Fatalf("scrolled into opened lone group b: pin visible=%v, own visible=%v", b.pin != nil && b.pin.Visible(), b.letter.Visible())
	}
	if got := atY(b.pin) - view; !within(got, crossRefPinTop) {
		t.Errorf("lone group b pin stands %v under view top, want %v", got, crossRefPinTop)
	}
	if atX(b.pin) != bOwnX {
		t.Errorf("lone group b pin X = %v, own X = %v", atX(b.pin), bOwnX)
	}
	if got, want := atY(b.stem), atY(b.pin)+bLine+crossRefStemGap; !within(got, want) {
		t.Errorf("lone group b rule starts at %v, want %v", got, want)
	}
	to(p.scroll.Offset.Y + atFoot(b.shade) - view - 8)
	if got, want := atY(b.pin)+bLine+crossRefStemGap, atFoot(b.shade)-crossRefStemFoot; !within(got, want) {
		t.Errorf("at lone group b's well foot: pin+line+gap = %v, rule foot = %v", got, want)
	}
	// Closing opened lone group b resets its pinned state.
	test.Tap(b.header)
	if b.pinned != 0 || (b.pin != nil && b.pin.Visible()) || !b.letter.Visible() {
		t.Errorf("after closing lone group b: pinned=%v, pin visible=%v, own visible=%v", b.pinned, b.pin != nil && b.pin.Visible(), b.letter.Visible())
	}

	// Standalone note row (no panel relayout wired) also resets pin on toggle().
	solo := newCrossRefNoteRow(st, a.note, st.pal(), func(crossRef) {}, &crossRefRelayout{})
	solo.Resize(fyne.NewSize(260, 400))
	solo.keepLetterInView(80)
	if solo.pinned == 0 || solo.pin == nil || !solo.pin.Visible() {
		t.Fatalf("control: solo note did not pin at into=80")
	}
	solo.toggle()
	if solo.pinned != 0 || solo.pin.Visible() || !solo.letter.Visible() {
		t.Errorf("after closing standalone note: pinned=%v, pin visible=%v, own visible=%v", solo.pinned, solo.pin.Visible(), solo.letter.Visible())
	}
}
