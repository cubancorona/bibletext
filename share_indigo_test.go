package bibletext

// The Indigo Book's worked quotation family, asserted VERBATIM. The Indigo
// Book (law.resource.org/pub/us/code/blue/IndigoBook.html) is the public-
// domain, Bluebook-compatible citation manual; its R38/R39 examples quote
// Viacom Int'l, Inc. v. YouTube, Inc., 676 F.3d 19 (2d Cir. 2012) and are the
// most authoritative freely licensed worked set available. Each test names
// its Indigo rule and the Bluebook rule it maps to.
//
// The Viacom sentences, used as a two-"verse" fixture chapter so the
// context-aware pipeline (completeTrailingSentence, provenance citations) can
// be exercised against them exactly as it runs against scripture:

import (
	"strings"
	"testing"
)

const (
	viacom1 = "The difference between actual and red flag knowledge is thus not between specific and generalized knowledge, but instead between a subjective and an objective standard."
	viacom2 = "In other words, the actual knowledge provision turns on whether the provider actually or subjectively knew of specific infringement, while the red flag provision turns on whether the provider was subjectively aware of facts that would have made the specific infringement objectively obvious to a reasonable person."
)

func viacomState() *AppState {
	bd := &BibleData{
		Books: []string{"Viacom"},
		Verses: map[string]map[int][]Verse{"Viacom": {1: {
			{BookName: "Viacom", Book: "Viacom", Chapter: 1, Verse: 1, Text: viacom1},
			{BookName: "Viacom", Book: "Viacom", Chapter: 1, Verse: 2, Text: viacom2},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Viacom", CurrentChapter: 1}
}

// Indigo R39.2 (Bluebook 5.3(b)(i)): beginning of the sentence omitted, quote
// used as a full sentence — bracket-capitalize, never a leading ellipsis:
//
//	"[T]he actual knowledge provision turns on whether the provider …"
func TestIndigoBeginningOmission(t *testing.T) {
	sel := "the actual knowledge provision turns on whether the provider actually or subjectively knew of specific infringement"
	if got := bracketStartCapital(sel); !strings.HasPrefix(got, "[T]he actual knowledge provision") {
		t.Errorf("R39.2 bracketed capital:\n got %q", firstRunes(got, 50))
	}
}

// Indigo R39.7 (Bluebook 5.3(b)(iii)): end of the sentence omitted from a
// quote standing as a full sentence — the four-dot form, space before the
// first period:
//
//	"The difference between actual and red flag knowledge is thus not
//	 between specific and generalized knowledge . . . ."
func TestIndigoEndOmission(t *testing.T) {
	sel := "The difference between actual and red flag knowledge is thus not between specific and generalized knowledge"
	want := "The difference between actual and red flag knowledge is thus not between specific and generalized knowledge . . . ."
	if got := addEndOmission(sel, '.'); got != want {
		t.Errorf("R39.7 four-dot:\n got %q\nwant %q", got, want)
	}
	// And end to end through the real pipeline, with the provenance citation.
	st := viacomState()
	text, cite, _, _ := prepareShareQuote(st, sel, selSpan{})
	quote := formatBibleQuote(text, originalSentenceTerminal(st, text, -1, 0))
	if !strings.HasSuffix(quote, "generalized knowledge . . . .”") {
		t.Errorf("pipeline four-dot: %q", quote[maxInt(0, len(quote)-50):])
	}
	if cite != "Viacom 1:1" {
		t.Errorf("citation = %q, want Viacom 1:1", cite)
	}
}

// Indigo R39.9 (Bluebook 5.3(b)(iv)): "omitting material AFTER a final
// punctuation mark: no ellipsis at all — the quote simply ends with the
// original period." The independent published confirmation of the app's
// completed-sentence rule: quoting sentence 1 in full, with sentence 2
// omitted, carries NO mark — even when the drag stopped one rune short of
// the period (completeTrailingSentence restores it first).
func TestIndigoNoMarkAfterFinalPunctuation(t *testing.T) {
	st := viacomState()
	for _, sel := range []string{
		viacom1,                          // full sentence, period included
		strings.TrimSuffix(viacom1, "."), // drag stopped before the period
	} {
		text, cite, _, _ := prepareShareQuote(st, sel, selSpan{})
		quote := formatBibleQuote(text, originalSentenceTerminal(st, text, -1, 0))
		if strings.Contains(quote, ". . .") {
			t.Errorf("R39.9: no ellipsis after a final punctuation mark; sel ends %q → %q",
				sel[len(sel)-12:], quote[maxInt(0, len(quote)-40):])
		}
		if !strings.HasSuffix(quote, "objective standard.”") {
			t.Errorf("quote must end on the original period: %q", quote[maxInt(0, len(quote)-40):])
		}
		if cite != "Viacom 1:1" {
			t.Errorf("citation = %q, want Viacom 1:1", cite)
		}
	}
}

// CSU (Cleveland State) Law Library's Rule 5.3 teaching pair
// (cmlawlibraryblog.classcaster.net, 2014 & 2017 posts):
//
//	original: "Standard poodles generally look great in chunky winter
//	           sweaters, and can rock the booties, too."
//	quoted:   "[P]oodles generally look great in chunky winter sweaters,
//	           and can rock the booties, too."
func TestCSUPoodleBeginningOmission(t *testing.T) {
	in := "poodles generally look great in chunky winter sweaters, and can rock the booties, too."
	want := "[P]oodles generally look great in chunky winter sweaters, and can rock the booties, too."
	if got := bracketStartCapital(in); got != want {
		t.Errorf("CSU pair:\n got %q\nwant %q", got, want)
	}
}

// Yale Style Rule 5.2's wholly-nested example (via the coursework guides that
// reproduce it): a quotation consisting ENTIRELY of a nested quotation
// collapses to ONE set of marks — quoting a speaker's line
//
//	“Frosted flakes are more than good; they’re great.”
//
// shares as "Frosted flakes are more than good; they’re great." — never
// "'Frosted flakes …'" double-wrapped. This is the published twin of the
// app's Rule 5.2(f)(i) balanced-pair collapse.
func TestYaleWhollyNestedCollapse(t *testing.T) {
	got := formatBibleQuote("“Frosted flakes are more than good; they’re great.”", '.')
	want := "“Frosted flakes are more than good; they’re great.”"
	if got != want {
		t.Errorf("wholly-nested quotation must carry ONE pair:\n got %q\nwant %q", got, want)
	}
}

// John 1:41 (WEB, the real embedded seed text) ends "…(which is, being
// interpreted, Christ)." — a drag stopping after "Christ" omits only the
// closing parenthesis and period, no words. Found by the ragged-cut sweep:
// the completion must traverse the parenthesis (appending it only because
// its opener is inside the selection) and carry no false ellipsis.
func TestParenthesisBeforeTerminal(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1}
	corpus, _ := chapterProse(st)
	i := strings.Index(corpus, "He first found his own brother")
	j := strings.Index(corpus, ", Christ)")
	if i < 0 || j < 0 {
		t.Skip("seed text differs from the expected WEB John 1:41 wording")
	}
	sel := corpus[i : j+len(", Christ")]
	text, _, _, _ := prepareShareQuote(st, sel, selSpan{})
	if !strings.HasSuffix(text, "Christ).") {
		t.Errorf("completion must traverse the parenthesis: …%q", text[maxInt(0, len(text)-25):])
	}
	quote := formatBibleQuote(text, originalSentenceTerminal(st, text, -1, 0))
	if strings.Contains(quote, ". . .") {
		t.Errorf("no words omitted — no mark belongs: %q", quote[maxInt(0, len(quote)-40):])
	}
}
