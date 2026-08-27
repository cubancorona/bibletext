package bibletext

import "testing"

func TestVerseOfDayRefsAreWellFormed(t *testing.T) {
	if len(verseOfDayRefs) == 0 {
		t.Fatal("verseOfDayRefs is empty")
	}
	for _, r := range verseOfDayRefs {
		if r.Book == "" || r.Chapter < 1 || r.Verse < 1 {
			t.Errorf("malformed reference %+v", r)
		}
	}
}

func TestVerseOfDayRefsAreUniqueAndAmple(t *testing.T) {
	seen := make(map[dayVerseRef]bool, len(verseOfDayRefs))
	for _, r := range verseOfDayRefs {
		if seen[r] {
			t.Errorf("duplicate reference %+v — a repeat wastes a slot in the daily rotation", r)
		}
		seen[r] = true
	}
	// A year's worth is the whole point; guard against an accidental truncation.
	if n := len(verseOfDayRefs); n < 300 {
		t.Errorf("verseOfDayRefs shrank to %d; expected a full-year rotation", n)
	}
}

func TestVerseOfTheDayNilOrEmpty(t *testing.T) {
	if _, ok := verseOfTheDay(nil); ok {
		t.Error("nil state should yield no verse")
	}
	if _, ok := verseOfTheDay(&AppState{Bible: NewBibleData()}); ok {
		t.Error("empty bible should yield no verse")
	}
}

func TestVerseOfTheDayResolvesAndIsStable(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{
		3: {{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world."}},
	}
	state := &AppState{Bible: bd}

	v, ok := verseOfTheDay(state)
	if !ok {
		t.Fatal("expected a resolvable verse")
	}
	if v.BookName != "John" || v.Chapter != 3 || v.Verse != 16 {
		t.Errorf("unexpected verse %+v", v)
	}
	// Same day -> same verse. (Identity fields — Verse carries a slice now.)
	if v2, _ := verseOfTheDay(state); v2.BookName != v.BookName ||
		v2.Chapter != v.Chapter || v2.Verse != v.Verse || v2.Text != v.Text {
		t.Error("verse of the day should be stable within a day")
	}
}
