package bibletext

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"strings"
)

func TestSuperscriptNumber(t *testing.T) {
	cases := map[int]string{0: "", 1: "¹", 16: "¹⁶", 119: "¹¹⁹"}
	for in, want := range cases {
		if got := superscriptNumber(in); got != want {
			t.Errorf("superscriptNumber(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestChapterPickerColumns(t *testing.T) {
	cases := map[int]int{0: 1, 1: 2, 4: 2, 9: 3, 50: 8, 150: 8}
	for in, want := range cases {
		if got := chapterPickerColumns(in); got != want {
			t.Errorf("chapterPickerColumns(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestGroupVersesIntoParagraphsPreservesEveryVerse(t *testing.T) {
	if groupVersesIntoParagraphs(nil) != nil {
		t.Fatal("expected nil for no verses")
	}

	verses := make([]Verse, 0, 12)
	for i := 1; i <= 12; i++ {
		verses = append(verses, Verse{
			BookName: "Test",
			Chapter:  1,
			Verse:    i,
			Text:     "This is a reasonably long sentence used to push the running character count past the paragraph break threshold.",
		})
	}

	paragraphs := groupVersesIntoParagraphs(verses)
	if len(paragraphs) < 2 {
		t.Fatalf("expected long chapters to split into multiple paragraphs, got %d", len(paragraphs))
	}

	seen := 0
	expect := 1
	for _, para := range paragraphs {
		for _, v := range para {
			if v.Verse != expect {
				t.Fatalf("verse order broken: got %d want %d", v.Verse, expect)
			}
			expect++
			seen++
		}
	}
	if seen != len(verses) {
		t.Fatalf("expected all %d verses retained, got %d", len(verses), seen)
	}
}

func TestReadingColumnCentresAndCaps(t *testing.T) {
	child := canvas.NewRectangle(color.Black)
	child.SetMinSize(fyne.NewSize(10, 200))
	objects := []fyne.CanvasObject{child}

	l := &readingColumn{maxWidth: 760}

	// Wide window: width capped at 760 and centred (x = (1200-760)/2).
	l.Layout(objects, fyne.NewSize(1200, 800))
	if child.Size().Width != 760 {
		t.Fatalf("expected capped width 760, got %v", child.Size().Width)
	}
	if child.Position().X != (1200-760)/2 {
		t.Fatalf("expected centred X 220, got %v", child.Position().X)
	}

	// Narrow window: column fills the width and sits at the left edge.
	l.Layout(objects, fyne.NewSize(300, 800))
	if child.Size().Width != 300 || child.Position().X != 0 {
		t.Fatalf("expected responsive full-width column at X 0, got w=%v x=%v", child.Size().Width, child.Position().X)
	}
}

// TestReadingColumnMinSizeWidthIsZero guards the sidebar-fills-the-screen bug: the
// column must not propagate its (wide) text min-width upward, or the enclosing
// HSplit divider can collapse the reading pane and let the sidebar take over.
func TestReadingColumnMinSizeWidthIsZero(t *testing.T) {
	wide := canvas.NewRectangle(color.Black)
	wide.SetMinSize(fyne.NewSize(2000, 400))
	l := &readingColumn{maxWidth: 760}

	ms := l.MinSize([]fyne.CanvasObject{wide})
	if ms.Width != 0 {
		t.Errorf("readingColumn MinSize width must be 0 (decoupled from text width), got %v", ms.Width)
	}
	if ms.Height != 400 {
		t.Errorf("expected height passed through (400), got %v", ms.Height)
	}
}

// A verse that closes reported speech ends on a typographic quotation mark,
// not a typewriter one. The rule already meant to break there — it lists both
// straight quote characters — so the curly forms belong in the same list.
// Without them a fifth of the rule's break points were refused, and always in
// dialogue: Deuteronomy 27's curses ran eleven verses into one paragraph
// because "…let all the people say, 'Amen!'" ends on a curly mark.
func TestParagraphBreaksAfterClosedSpeech(t *testing.T) {
	long := strings.Repeat("a reasonably long clause to carry the paragraph past its threshold, ", 6)
	for _, tc := range []struct {
		name, ending string
		want         bool
	}{
		{"full stop", "the end.", true},
		{"question mark", "who can say?", true},
		{"straight double quote", `he said, "come."`, true},
		{"straight single quote", "they said, 'amen!'", true},
		{"curly double quote", "he said, “come.”", true},
		{"curly single quote", "they said, ‘amen!’", true},
		{"mid-sentence", "and then they", false},
		{"a dash", "and then —", false},
	} {
		if got := shouldBreakParagraph(long+tc.ending, 400); got != tc.want {
			t.Errorf("%s: break after %q = %v, want %v", tc.name, tc.ending, got, tc.want)
		}
	}
	// The length floor still governs: the same ending under the threshold
	// never breaks.
	if shouldBreakParagraph("he said, “come.”", 100) {
		t.Error("a short paragraph must not break, whatever it ends on")
	}
}
