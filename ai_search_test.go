package bibletext

import "testing"

func TestAIListMarkerPreservesBookNumber(t *testing.T) {
	cases := map[string]string{
		"1 John 4:8":    "1 John 4:8", // a leading book number must NOT be stripped
		"2 Peter 1:21":  "2 Peter 1:21",
		"1. John 3:16":  "John 3:16", // a list number ("1.") is stripped
		"2) Mark 1:1":   "Mark 1:1",
		"- Genesis 1:1": "Genesis 1:1",
		"* Psalms 23:1": "Psalms 23:1",
	}
	for in, want := range cases {
		if got := aiListMarkerPattern.ReplaceAllString(in, ""); got != want {
			t.Errorf("aiListMarkerPattern(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveReferenceListParsesAIReply(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	if bd.GetVerse("John", 3, 16) == nil {
		t.Skip("sample data lacks John 3:16")
	}

	// A realistic, messy model reply: list markers, a duplicate, trailing
	// commentary, a non-existent book, and a prose line with no reference.
	reply := "1. John 3:16\n" +
		"- John 3:16\n" + // duplicate of the above
		"John 3:16 — God's love for the world\n" + // trailing commentary
		"Hobbiton 9:9\n" + // not a real book
		"some reflection with no reference at all\n"

	got := resolveReferenceList(bd, reply)
	if len(got) != 1 {
		t.Fatalf("expected exactly one de-duplicated verse, got %d: %+v", len(got), got)
	}
	if got[0].BookName != "John" || got[0].Chapter != 3 || got[0].Verse != 16 {
		t.Fatalf("expected John 3:16, got %s %d:%d", got[0].BookName, got[0].Chapter, got[0].Verse)
	}
}

func TestExtractReferenceHandlesMultiWordBookAndJunk(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	if bd.GetVerse("1 Corinthians", 13, 4) == nil {
		t.Skip("sample data lacks 1 Corinthians 13:4")
	}
	if v, ok := extractReference(bd, "1 Corinthians 13:4 is about love"); !ok ||
		v.BookName != "1 Corinthians" || v.Chapter != 13 || v.Verse != 4 {
		t.Fatalf("expected 1 Corinthians 13:4 from a commented line, got %+v ok=%v", v, ok)
	}
	if _, ok := extractReference(bd, "no reference here"); ok {
		t.Fatal("a prose line should not yield a reference")
	}
}
