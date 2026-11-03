package bibletext

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func reflowPositionState() *AppState {
	words := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu ", 45)
	verses := []Verse{
		{Book: "Synthetic", BookName: "Synthetic", Chapter: 1, Verse: 1, Text: words},
		{Book: "Synthetic", BookName: "Synthetic", Chapter: 1, Verse: 2, Text: words},
	}
	return &AppState{Bible: &BibleData{Books: []string{"Synthetic"}, Verses: map[string]map[int][]Verse{"Synthetic": {1: verses}}}, CurrentBook: "Synthetic", CurrentChapter: 1}
}

// Reflow preserves the character that occupied the first visible line, and
// its fraction through that line. Verse-relative pixel distances instead
// land among different words when that verse gains or loses wrapped lines.
func TestStyledWidthReflowKeepsTheSameReadingCharacter(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	t.Cleanup(resetStyledWiring)
	state := reflowPositionState()
	area := styledReadingScrollArea(state, state.Bible.GetChapter("Synthetic", 1), lightPalette)
	window := test.NewWindow(area)
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(820, 300))
	for range 4 {
		area.Refresh()
	}
	initial := styledPane.lay.Lines[8]
	character := initial.StartOffset
	selectionStart, selectionEnd := character+2, character+24
	styledPane.selAnchor, styledPane.selStart, styledPane.selEnd = selectionStart, selectionStart, selectionEnd
	fraction := float32(0.37)
	initialY := initial.Y + fraction*initial.H
	styledScroll.Offset = fyne.NewPos(0, initialY)
	styledScroll.OnScrolled(styledScroll.Offset)
	widths := []float32{440, 300, 670, 360, 820, 490, 820}
	for _, width := range widths {
		window.Resize(fyne.NewSize(width, 300))
		for range 4 {
			area.Refresh()
		}
		var target *styledLine
		for j := range styledPane.lay.Lines {
			line := &styledPane.lay.Lines[j]
			if line.StartOffset <= character && character < line.EndOffset {
				target = line
				break
			}
		}
		if target == nil {
			t.Fatalf("character %d disappeared after width %.0f", character, width)
		}
		if styledPane.selStart != selectionStart || styledPane.selEnd != selectionEnd {
			t.Fatalf("width %.0f changed selection [%d,%d) to [%d,%d)", width, selectionStart, selectionEnd, styledPane.selStart, styledPane.selEnd)
		}
		want := target.Y + fraction*target.H
		if got := styledScroll.Offset.Y; math.Abs(float64(got-want)) > 0.5 {
			t.Fatalf("width %.0f: scroll %.3f, want %.3f for reading character %d (line [%d,%d))", width, got, want, character, target.StartOffset, target.EndOffset)
		}
	}
	if got := styledScroll.Offset.Y; math.Abs(float64(got-initialY)) > 0.5 {
		t.Errorf("round trip drifted from %.3f to %.3f", initialY, got)
	}
}

func TestStyledTextReflowRejectsChangedWords(t *testing.T) {
	before := &chapterLayout{Text: "alpha beta gamma", Lines: []styledLine{{StartOffset: 0, EndOffset: 16, Y: 10, H: 20}}}
	anchor := captureStyledReflow(before, 17)
	changed := &chapterLayout{Text: "alpha zeta gamma", Lines: []styledLine{{StartOffset: 0, EndOffset: 16, Y: 15, H: 20}}}
	if _, ok := anchor.y(changed); ok {
		t.Fatal("changed words reused a reading-character anchor")
	}
	wrapped := &chapterLayout{Text: "alpha\nbeta gamma", Lines: []styledLine{{StartOffset: 0, EndOffset: 6, Y: 15, H: 20}, {StartOffset: 6, EndOffset: 16, Y: 35, H: 20}}}
	if y, ok := anchor.y(wrapped); !ok || math.Abs(float64(y-22)) > .001 {
		t.Fatalf("presentation wrap rejected or moved anchor: %.3f,%v", y, ok)
	}
}

func TestStyledTextReflowPreservesWrappedHeading(t *testing.T) {
	before := &chapterLayout{Text: "body", Lines: []styledLine{
		{Heading: "A synthetic", StartOffset: 0, EndOffset: 0, Y: 10, H: 20},
		{Heading: "heading continues", StartOffset: 0, EndOffset: 0, Y: 30, H: 20},
		{StartOffset: 0, EndOffset: 4, Y: 60, H: 20},
	}}
	anchor := captureStyledReflow(before, 36)
	after := &chapterLayout{Text: "body", Lines: []styledLine{
		{Heading: "A synthetic heading", StartOffset: 0, EndOffset: 0, Y: 15, H: 20},
		{Heading: "continues", StartOffset: 0, EndOffset: 0, Y: 35, H: 20},
		{StartOffset: 0, EndOffset: 4, Y: 65, H: 20},
	}}
	if y, ok := anchor.y(after); !ok || math.Abs(float64(y-21)) > .001 {
		t.Fatalf("heading character did not survive rewrap: %.3f,%v", y, ok)
	}
}

func TestStyledWidthReflowFollowsTheReadersNewPosition(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	t.Cleanup(resetStyledWiring)
	state := reflowPositionState()
	area := styledReadingScrollArea(state, state.Bible.GetChapter("Synthetic", 1), lightPalette)
	window := test.NewWindow(area)
	t.Cleanup(window.Close)
	settle := func(width float32) {
		window.Resize(fyne.NewSize(width, 300))
		for range 4 {
			area.Refresh()
		}
	}
	settle(820)
	line := styledPane.lay.Lines[8]
	styledScroll.Offset = fyne.NewPos(0, line.Y+line.H*.37)
	styledScroll.OnScrolled(styledScroll.Offset)
	settle(440)
	line = styledPane.lay.Lines[20]
	next := line.StartOffset
	styledScroll.Offset = fyne.NewPos(0, line.Y+line.H*.21)
	styledScroll.OnScrolled(styledScroll.Offset)
	settle(700)
	for _, line = range styledPane.lay.Lines {
		if line.StartOffset <= next && next < line.EndOffset {
			want := line.Y + line.H*.21
			if math.Abs(float64(styledScroll.Offset.Y-want)) > .5 {
				t.Fatalf("new user position lost: %.3f,want%.3f", styledScroll.Offset.Y, want)
			}
			return
		}
	}
	t.Fatal("new reading character disappeared")
}

// These assertions read the laid-out line ranges directly, independently of
// the transient anchor's capture/resolve implementation.
func styledReadingCharacterAtTop(t *testing.T) (int, float32) {
	t.Helper()
	y := styledScroll.Offset.Y
	for _, line := range styledPane.lay.Lines {
		if line.Heading == "" && y < line.Y+line.H {
			return line.StartOffset, (y - line.Y) / line.H
		}
	}
	t.Fatal("viewport has no reading line")
	return 0, 0
}

func assertStyledReadingCharacter(t *testing.T, character int, fraction float32) {
	t.Helper()
	for _, line := range styledPane.lay.Lines {
		if line.Heading == "" && line.StartOffset <= character && character < line.EndOffset {
			want := line.Y + fraction*line.H
			if math.Abs(float64(styledScroll.Offset.Y-want)) > .5 {
				t.Fatalf("reading character %d moved: scroll%.3f,want%.3f", character, styledScroll.Offset.Y, want)
			}
			return
		}
	}
	t.Fatalf("reading character %d disappeared", character)
}

func TestStyledWidthReflowKeepsTheSameFootnoteCharacter(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	t.Cleanup(resetStyledWiring)
	setFootnotesEnabled(true)
	t.Cleanup(func() { setFootnotesEnabled(false) })
	state := reflowPositionState()
	verses := state.Bible.Verses["Synthetic"][1]
	verses[0].Footnotes = []Footnote{{Anchor: 0, Text: strings.Repeat("first second third fourth fifth sixth seventh eighth ninth tenth ", 60)}}
	area := styledReadingScrollArea(state, verses, lightPalette)
	window := test.NewWindow(area)
	t.Cleanup(window.Close)
	settle := func(width float32) {
		window.Resize(fyne.NewSize(width, 260))
		for range 4 {
			area.Refresh()
		}
	}
	settle(820)
	var character int
	var original float32
	fraction := float32(.31)
	for index, line := range styledFootnoteReflowLayout(styledPane).Lines {
		if index == 5 {
			character = line.StartOffset
			original = line.Y + fraction*line.H
			break
		}
	}
	if original == 0 {
		t.Fatal("footnote fixture did not wrap")
	}
	styledScroll.Offset = fyne.NewPos(0, original)
	styledScroll.OnScrolled(styledScroll.Offset)
	for _, width := range []float32{440, 300, 670, 360, 820} {
		settle(width)
		found := false
		for _, line := range styledFootnoteReflowLayout(styledPane).Lines {
			if line.StartOffset <= character && character < line.EndOffset {
				found = true
				want := line.Y + fraction*line.H
				if math.Abs(float64(styledScroll.Offset.Y-want)) > .5 {
					t.Fatalf("width%.0f lost footnote character%d:%.3f,want%.3f", width, character, styledScroll.Offset.Y, want)
				}
				break
			}
		}
		if !found {
			t.Fatalf("footnote character%d disappeared", character)
		}
	}
	if math.Abs(float64(styledScroll.Offset.Y-original)) > .5 {
		t.Fatal("footnote roundtrip drifted")
	}
}
