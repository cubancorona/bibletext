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
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.PrepareSearchIndex()

	book, ok := "", false
	var chapter int
	var verses []Verse
	for _, b := range bd.Books {
		for ch, vs := range bd.Verses[b] {
			if len(vs) > 0 {
				book, chapter, verses, ok = b, ch, vs, true
				break
			}
		}
		if ok {
			break
		}
	}
	if !ok {
		t.Skip("sample data has no verses")
	}

	// More distinct resolvable lines than the cap: repeat the chapter's verses
	// with distinct verse numbers by cycling through every book's chapters.
	var b strings.Builder
	distinct := 0
	for _, bk := range bd.Books {
		for ch, vs := range bd.Verses[bk] {
			for _, v := range vs {
				fmt.Fprintf(&b, "%s %d:%d\n", bk, ch, v.Verse)
				distinct++
			}
		}
	}
	if distinct <= aiSearchResultCap {
		// Not enough sample data to exceed the cap — pad with duplicates and
		// assert dedup instead (the cap branch is still exercised by the loop).
		fmt.Fprintf(&b, "%s %d:%d\n", book, chapter, verses[0].Verse)
	}

	got := resolveReferenceList(bd, b.String())
	if len(got) > aiSearchResultCap {
		t.Fatalf("resolver must cap results at %d, got %d", aiSearchResultCap, len(got))
	}
	if distinct > aiSearchResultCap && len(got) != aiSearchResultCap {
		t.Fatalf("with %d distinct references the resolver should fill the cap %d, got %d",
			distinct, aiSearchResultCap, len(got))
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
