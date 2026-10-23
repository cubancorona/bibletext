package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// The Windows and Linux pane draws a publisher's heading outside its text
// model (TestHeadingsAreDrawnButNotSelectable), and the model offset under a
// heading's line is the end of the verse above it. A double-click on a
// heading's word selected that verse's last word, "sins." of Acts 10:43 here,
// and the verbs quoted it and cited the verse above: the other panes' fault
// in this pane's shape. A word of a heading means the section the heading
// introduces (selSpan.heading); this pane cannot select one, so a
// double-click there selects nothing, anywhere along the heading's line. The
// control is a double-click on a verse's word, which selects that word.
func TestADoubleClickOnAHeadingSelectsNothingOnTheStyledPane(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := newTestPane(t, headingChapterState(), 420)

	off := len([]rune(p.lay.Text[:strings.Index(p.lay.Text, "commanded")]))
	p.DoubleTapped(&fyne.PointEvent{Position: posForOffset(p, off+2)})
	if got := p.selectedRaw(); got != "commanded" {
		t.Fatalf("control: a double-click on a verse's word selected %q, want \"commanded\"", got)
	}
	p.clearSelection()

	lines := 0
	for _, ln := range p.lay.Lines {
		if ln.Heading == "" {
			continue
		}
		lines++
		for _, x := range []float32{2, 40, 120, 200} {
			p.DoubleTapped(&fyne.PointEvent{Position: fyne.NewPos(p.insetX()+x, ln.Y+ln.H/2)})
			if got := p.selectedRaw(); got != "" {
				t.Errorf("a double-click at %v on the heading line %q selected %q, want nothing; span %+v",
					x, ln.Heading, got, p.selectedVerseSpan())
			}
			p.clearSelection()
		}
	}
	if lines == 0 {
		t.Fatal("fixture: the pane laid out no heading line")
	}
}
