package main

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

func segmentText(segs []widget.RichTextSegment) string {
	var b strings.Builder
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func boldText(segs []widget.RichTextSegment) string {
	var b strings.Builder
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Style.TextStyle.Bold {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func TestMatchRangesMergesOverlapsCaseInsensitive(t *testing.T) {
	ranges := matchRanges("Faith and faithful faith", []string{"faith"})
	if len(ranges) != 3 {
		t.Fatalf("expected 3 matches of 'faith', got %d (%v)", len(ranges), ranges)
	}

	// Overlapping terms should merge into one span.
	merged := matchRanges("steadfastness", []string{"stead", "steadfast"})
	if len(merged) != 1 {
		t.Fatalf("expected overlapping terms to merge, got %v", merged)
	}
	if merged[0].start != 0 || merged[0].end != len("steadfast") {
		t.Fatalf("unexpected merged span: %+v", merged[0])
	}

	if got := matchRanges("nothing here", []string{"a"}); got != nil {
		t.Fatalf("single-character terms should be ignored, got %v", got)
	}
}

func TestTermHighlightSegmentsPreserveTextAndEmphasis(t *testing.T) {
	text := "For God so loved the world"
	segs := termHighlightSegments(text, []string{"god", "world"}, colorNameVerseText, colorNameHighlightHi)

	if got := segmentText(segs); got != text {
		t.Fatalf("highlight segments lost text: got %q want %q", got, text)
	}
	if got := boldText(segs); got != "Godworld" {
		t.Fatalf("expected matched terms emphasised, got bold=%q", got)
	}
}

func TestTermHighlightSegmentsNoMatch(t *testing.T) {
	segs := termHighlightSegments("plain text", []string{"zebra"}, colorNameVerseText, colorNameHighlightHi)
	if len(segs) != 1 {
		t.Fatalf("expected a single segment when nothing matches, got %d", len(segs))
	}
	if boldText(segs) != "" {
		t.Fatal("expected no emphasis when nothing matches")
	}
}

func TestResolveBookName(t *testing.T) {
	books := NewBibleData().Books

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"John", "John", true},
		{"john", "John", true},
		{"jn", "John", true},
		{"ps", "Psalms", true},
		{"psalm", "Psalms", true},
		{"1 cor", "1 Corinthians", true},
		{"philipp", "Philippians", true}, // unique prefix
		{"song of songs", "Song of Solomon", true},
		{"zebra", "", false},
		{"j", "", false}, // ambiguous prefix (John, Jude, James, Joshua...)
	}
	for _, c := range cases {
		got, ok := resolveBookName(books, c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("resolveBookName(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseReferenceQueryWithAliases(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()

	book, chapter, verse, hasVerse, ok := bd.parseReferenceQuery("jn 3:16")
	if !ok || book != "John" || chapter != 3 || verse != 16 || !hasVerse {
		t.Fatalf("expected John 3:16, got %s %d:%d (hasVerse=%v ok=%v)", book, chapter, verse, hasVerse, ok)
	}

	book, chapter, _, hasVerse, ok = bd.parseReferenceQuery("Ps 23")
	if !ok || book != "Psalms" || chapter != 23 || hasVerse {
		t.Fatalf("expected Psalms 23 chapter ref, got %s %d (hasVerse=%v ok=%v)", book, chapter, hasVerse, ok)
	}
}
