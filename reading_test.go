package main

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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

func TestReadingLayoutStacksCentresAndCaps(t *testing.T) {
	a := canvas.NewRectangle(color.Black)
	a.SetMinSize(fyne.NewSize(10, 100))
	b := canvas.NewRectangle(color.Black)
	b.SetMinSize(fyne.NewSize(10, 50))
	objects := []fyne.CanvasObject{a, b}

	l := &readingLayout{maxWidth: 760, spacing: 20}

	// Wide window: width capped at 760 and centred (x = (1200-760)/2).
	l.Layout(objects, fyne.NewSize(1200, 800))
	if a.Size().Width != 760 {
		t.Fatalf("expected capped width 760, got %v", a.Size().Width)
	}
	if a.Position().X != (1200-760)/2 {
		t.Fatalf("expected centred X 220, got %v", a.Position().X)
	}
	// Second paragraph stacks below the first with spacing (100 + 20).
	if b.Position().Y != 120 {
		t.Fatalf("expected second paragraph at Y 120, got %v", b.Position().Y)
	}

	// Narrow window: column fills the width and sits at the left edge.
	l.Layout(objects, fyne.NewSize(300, 800))
	if a.Size().Width != 300 || a.Position().X != 0 {
		t.Fatalf("expected responsive full-width column at X 0, got w=%v x=%v", a.Size().Width, a.Position().X)
	}

	if ms := l.MinSize(objects); ms.Height != 100+50+20 {
		t.Fatalf("expected MinSize height 170 (heights + spacing), got %v", ms.Height)
	}
}
