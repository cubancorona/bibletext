package holybible

import "testing"

func TestAnnotationStoreAddAndQuery(t *testing.T) {
	store := NewAnnotationStore()
	jn316 := VerseRef{Book: "John", Chapter: 3, Verse: 16}
	ps23 := VerseRef{Book: "Psalms", Chapter: 23, Verse: 1}

	if store.HasAny(jn316) {
		t.Fatal("new store should be empty")
	}

	store.Add(Annotation{Ref: jn316, Note: "the gospel in one verse"})
	store.Add(Annotation{Ref: jn316, Color: "gold"})
	store.Add(Annotation{Ref: ps23, Note: "comfort"})

	if !store.HasAny(jn316) {
		t.Error("expected John 3:16 to have annotations")
	}
	if got := len(store.ForVerse(jn316)); got != 2 {
		t.Errorf("expected 2 annotations on John 3:16, got %d", got)
	}
	if got := store.Count(); got != 3 {
		t.Errorf("expected 3 annotations total, got %d", got)
	}
	if store.ForVerse(jn316)[0].Created.IsZero() {
		t.Error("Add should stamp Created")
	}
}

func TestAnnotationStoreRefsSortedCanonically(t *testing.T) {
	store := NewAnnotationStore()
	store.Add(Annotation{Ref: VerseRef{Book: "John", Chapter: 3, Verse: 16}})
	store.Add(Annotation{Ref: VerseRef{Book: "Genesis", Chapter: 1, Verse: 1}})
	store.Add(Annotation{Ref: VerseRef{Book: "John", Chapter: 1, Verse: 1}})

	refs := store.Refs(NewBibleData().Books)
	want := []VerseRef{
		{Book: "Genesis", Chapter: 1, Verse: 1},
		{Book: "John", Chapter: 1, Verse: 1},
		{Book: "John", Chapter: 3, Verse: 16},
	}
	if len(refs) != len(want) {
		t.Fatalf("expected %d refs, got %d", len(want), len(refs))
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("ref %d = %v, want %v", i, refs[i], want[i])
		}
	}
}

func TestVerseRefString(t *testing.T) {
	if got := (VerseRef{Book: "1 Corinthians", Chapter: 13, Verse: 4}).String(); got != "1 Corinthians 13:4" {
		t.Errorf("unexpected ref string %q", got)
	}
	if got := refOf(Verse{BookName: "John", Chapter: 3, Verse: 16}); got != (VerseRef{Book: "John", Chapter: 3, Verse: 16}) {
		t.Errorf("refOf mismatch: %v", got)
	}
}
