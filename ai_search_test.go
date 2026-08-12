package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

// TestAISearchPromptContract locks the Find prompt's load-bearing properties:
// the result bound comes from aiSearchResultCap (one source of truth with the
// parser, never a re-hardcoded number), the model is told not to pad, and the
// reply-format instructions come AFTER the reader's request — instruction-last
// ordering, so text inside the request can't easily override the contract.
func TestAISearchPromptContract(t *testing.T) {
	const query = "what did God say to Jonah?"
	p := buildAISearchPrompt("  " + query + "  ")

	if !strings.Contains(p, fmt.Sprintf("up to %d", aiSearchResultCap)) {
		t.Errorf("prompt must carry the shared cap %d, got:\n%s", aiSearchResultCap, p)
	}
	if !strings.Contains(p, "never pad") {
		t.Error("prompt must forbid padding toward the cap")
	}
	qAt := strings.Index(p, query)
	instrAt := strings.Index(p, "Reply with ONLY")
	if qAt < 0 || instrAt < 0 || instrAt < qAt {
		t.Errorf("format instructions must follow the reader's request (request at %d, instructions at %d)", qAt, instrAt)
	}
	if strings.Contains(p, "  "+query) {
		t.Error("the request must be trimmed into the prompt")
	}
}

// TestResolveReferenceListHonorsCap locks the parser half of the shared bound:
// a model that ignores the instruction cannot flood the results past
// aiSearchResultCap.
func TestResolveReferenceListHonorsCap(t *testing.T) {
	// A corpus BIGGER than the cap, built for the purpose. The previous version
	// drew its references from PopulateWithSampleVerses — 62 verses against a cap
	// of 120 — so `len(got) > cap` could never hold and the `distinct > cap`
	// branch was guarded by a condition that was always false. It passed
	// unconditionally while appearing to police the bound.
	const verses = aiSearchResultCap + 40
	bd := NewBibleData()
	bd.Books = []string{"Psalms"}
	chapter := make([]Verse, 0, verses)
	for i := 1; i <= verses; i++ {
		chapter = append(chapter, Verse{BookName: "Psalms", Chapter: 119, Verse: i,
			Text: fmt.Sprintf("Line %d of the long psalm.", i)})
	}
	bd.Verses = map[string]map[int][]Verse{"Psalms": {119: chapter}}
	bd.PrepareSearchIndex()

	var b strings.Builder
	for i := 1; i <= verses; i++ {
		fmt.Fprintf(&b, "Psalms 119:%d\n", i)
	}

	got := resolveReferenceList(bd, b.String())
	if len(got) > aiSearchResultCap {
		t.Fatalf("resolver must cap results at %d, got %d", aiSearchResultCap, len(got))
	}
	if len(got) != aiSearchResultCap {
		t.Fatalf("with %d resolvable references the resolver should FILL the cap %d, got %d",
			verses, aiSearchResultCap, len(got))
	}
}

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
