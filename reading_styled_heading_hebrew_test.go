package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// A HEBREW LETTER IN A HEADING IS SET IN THE HEBREW FACE on the Windows and
// Linux pane, as the Apple panes' cascade and Android's fallback family set it.
// The pane drew each heading line as one text object in the bold cut, which has
// no Hebrew, so Psalm 119's stanza letters (the NKJV sets one at the head of
// each stanza's heading) came from whatever face the platform supplied. The
// control is a heading with no Hebrew: one object, in the bold cut, where it
// always stood.
func TestTheWindowsAndLinuxPaneSetsAHeadingsHebrewInTheHebrewFace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	heb := hebrewReadingFont()
	if heb == nil {
		t.Fatal("the Hebrew face is not embedded")
	}

	build := func(heading string) (*styledReadingPane, *styledPaneRenderer) {
		st := sampleState()
		ch := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
		st.Bible.Headings = map[string]map[int][]Heading{
			st.CurrentBook: {st.CurrentChapter: {{Text: heading, Style: "qa", BeforeVerse: ch[0].Verse}}},
		}
		p := newStyledReadingPane(st, ch)
		w := fyne.CurrentApp().NewWindow("x")
		w.SetContent(p)
		w.Resize(fyne.NewSize(900, 400))
		p.Resize(fyne.NewSize(900, 400))
		p.Refresh()
		r, ok := test.WidgetRenderer(p).(*styledPaneRenderer)
		if !ok {
			t.Fatal("the pane's renderer is not the styled renderer")
		}
		return p, r
	}

	p, r := build("א Fixture")
	if len(r.headTexts) != 2 {
		t.Fatalf("the heading is drawn as %d objects, want 2 (the letter, the words)", len(r.headTexts))
	}
	letter, words := r.headTexts[0], r.headTexts[1]
	name := func(f fyne.Resource) string {
		if f == nil {
			return "no face"
		}
		return f.Name()
	}
	if letter.Text != "\u05d0" || letter.FontSource != heb {
		t.Errorf("the letter is %q in %s, want it in the Hebrew face", letter.Text, name(letter.FontSource))
	}
	if words.Text != " Fixture" || words.FontSource != p.headingFace() {
		t.Errorf("the words are %q in %s, want them in the bold cut", words.Text, name(words.FontSource))
	}
	// Side by side, as measured, and on one baseline.
	if got, want := words.Position().X-letter.Position().X, letter.MinSize().Width; got < want-0.01 || got > want+0.01 {
		t.Errorf("the words start %.2f after the letter, which is %.2f wide", got, want)
	}
	drv := fyne.CurrentApp().Driver()
	baseline := func(o *canvas.Text) float32 {
		_, base := drv.RenderedTextSize(o.Text, o.TextSize, fyne.TextStyle{}, o.FontSource)
		return o.Position().Y + base
	}
	if a, b := baseline(letter), baseline(words); a-b > 0.01 || b-a > 0.01 {
		t.Errorf("the letter's baseline is at %.2f and the words' at %.2f", a, b)
	}
	if got, want := p.measure("א Fixture", runHeading, false), letter.MinSize().Width+words.MinSize().Width; got < want-0.01 || got > want+0.01 {
		t.Errorf("the heading measures %.2f and draws %.2f wide", got, want)
	}

	// The control.
	_, plain := build("A Fixture Heading")
	if len(plain.headTexts) != 1 || plain.headTexts[0].Text != "A Fixture Heading" || plain.headTexts[0].FontSource != p.headingFace() {
		t.Errorf("a heading with no Hebrew is no longer one object in the bold cut: %d objects", len(plain.headTexts))
	}
}

func TestHeadingSegmentsCutWhereTheScriptChanges(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"No Hebrew Here", []string{"No Hebrew Here"}},
		{"\u05d0 Aleph", []string{"\u05d0", " Aleph"}},
		{"Aleph \u05d0", []string{"Aleph ", "\u05d0"}},
		{"The \u05e9\u05c1\u05b8 Mark", []string{"The ", "\u05e9\u05c1\u05b8", " Mark"}},
		// A Hebrew phrase keeps its spaces, and so its own order.
		{"\u05d0\u05d1 \u05d2\u05d3  Aleph", []string{"\u05d0\u05d1 \u05d2\u05d3", "  Aleph"}},
	} {
		got := headingSegments(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%q cut as %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%q cut as %q, want %q", c.in, got, c.want)
				break
			}
		}
	}
}
