package bibletext

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

// legacyPaneSelect makes a selection in the legacy Entry pane the way its
// own keyboard model does, cursor at the first byte and shift-right to the
// last, over the first copy of words that stands after the number of verse,
// whether the pane wrapped a line between two of them or not.
func legacyPaneSelect(t *testing.T, c *chapterText, verse int, words string) {
	t.Helper()
	text := c.Entry.Text
	num := strings.Index(text, superscriptNumber(verse)+" ")
	if num < 0 {
		t.Fatalf("verse %d's number is not in the pane's text", verse)
	}
	q := strings.Fields(words)
	for i := range q {
		q[i] = regexp.QuoteMeta(q[i])
	}
	loc := regexp.MustCompile(strings.Join(q, `[ \n]+`)).FindStringIndex(text[num:])
	if loc == nil {
		t.Fatalf("%q does not stand after verse %d's number", words, verse)
	}
	start, end := num+loc[0], num+loc[1]
	c.CursorRow = strings.Count(text[:start], "\n")
	rowStart := strings.LastIndexByte(text[:start], '\n') + 1
	c.CursorColumn = utf8.RuneCountInString(text[rowStart:start])
	c.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	for n := utf8.RuneCountInString(text[start:end]); n > 0; n-- {
		c.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	c.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	if got := c.SelectedText(); got != text[start:end] {
		t.Fatalf("the pane selected %q, want %q", got, text[start:end])
	}
}

// The legacy Entry pane, the Windows and Linux fallback behind the styled
// pane, hands its verbs the verses its selection runs over. It used to hand
// them the zero span, so a refrain selected in verse 3 was placed at its
// first copy and shared as Psalms 136:1. Control: the refrain selected in
// verse 1, where the first copy is the reader's.
func TestLegacyPaneSharesASelectionFromItsOwnVerse(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, c := range []struct {
		verse int
		want  string
	}{
		{3, "Psalms 136:3"},
		{1, "Psalms 136:1"},
	} {
		state := refrainChapterState()
		state.aiKeys = newKeyStoreWith(newFakePrefs())
		pane := newChapterText(state, state.Bible.GetChapter("Psalms", 136))
		w := test.NewWindow(pane)
		w.Resize(fyne.NewSize(720, 900))
		w.Canvas().Focus(pane)
		legacyPaneSelect(t, pane, c.verse, "loving kindness endures forever")

		m := pane.selectionMenu()
		var share *fyne.MenuItem
		for _, it := range m.Items {
			if it.Label == "Share" {
				share = it
			}
		}
		if share == nil || share.ChildMenu == nil || share.ChildMenu.Items[0].Label != "Share with citation" {
			t.Fatalf("the pane's menu has no Share with citation: %v", menuLabels(m))
		}
		var shared []string
		prev := shareTextOut
		shareTextOut = func(s string) { shared = append(shared, s) }
		share.ChildMenu.Items[0].Action()
		shareTextOut = prev
		w.Close()
		if len(shared) != 1 || !strings.Contains(shared[0], c.want+" (") {
			t.Errorf("verse %d's refrain was shared as %q, want it cited %s", c.verse, shared, c.want)
		}
	}
}

// rewrap records where each verse's first token begins in the text it lays
// out, at every width: the number glued to the first word, wherever a soft
// wrap, a poem line, a blank poem line or a paragraph break put it, on the
// line verseLines names. An invented chapter with all four.
func TestLegacyPaneRecordsWhereEachVerseBegins(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	verses := []Verse{
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 1, ParaStart: true, Text: "The people went out to the field at first light and worked until the evening came."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 2, Text: "They ate together by the well."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 3, ParaStart: true, Text: "Sing to the hills,\nsing to the valleys below,\n\nand let the rivers answer."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 4, Text: "The morning comes,\nthe night is gone."},
		{BookName: "Probe", Book: "Probe", Chapter: 1, Verse: 10, ParaStart: true, Text: "At the end of the days they returned to the city with their flocks."},
	}
	state := &AppState{Bible: NewBibleData(), CurrentBook: "Probe", CurrentChapter: 1}
	pane := newChapterText(state, verses)
	for _, width := range []float32{150, 260, 400, 720, 2000} {
		pane.rewrap(width)
		if len(pane.verseStarts) != len(verses) || len(pane.verseLines) != len(verses) {
			t.Fatalf("at %v: %d verse offsets and %d verse lines for %d verses", width, len(pane.verseStarts), len(pane.verseLines), len(verses))
		}
		text := pane.Entry.Text
		for i, off := range pane.verseStarts {
			v := pane.verseLines[i].verse
			if off < 0 || off > len(text) || !strings.HasPrefix(text[off:], superscriptNumber(v)+" ") {
				t.Errorf("at %v: verse %d is recorded at byte %d, where the text reads %q", width, v, off, text[max(0, min(len(text), off)):min(len(text), max(0, off)+12)])
				continue
			}
			if row := strings.Count(text[:off], "\n"); row != pane.verseLines[i].line {
				t.Errorf("at %v: verse %d begins on row %d, recorded on line %d", width, v, row, pane.verseLines[i].line)
			}
		}
	}
}
