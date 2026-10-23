package bibletext

import (
	"strings"
	"testing"
)

// Removing an unfinished word must not let the share fallback put it back.
// Menus and dispatch read that same quotation, so a punctuation remainder
// offers no study while a complete word on either side still does.
func TestAnUnfinishedWordAcrossAHeadingLeavesNoStudyableFragment(t *testing.T) {
	for _, c := range []struct {
		name, words, end, quote string
		study                   bool
	}{
		{"one letter", "Harbour lights welcomed the travellers.", "H", ".", false},
		{"several letters", "Harbour lights welcomed the travellers.", "Harbou", ".", false},
		{"apostrophe", "Harbour’s lights welcomed the travellers.", "Harbour’", ".", false},
		{"hyphen", "Harbour-lights welcomed the travellers.", "Harbour-", ".", false},
		{"opening quotation mark", "“Harbour lights welcomed the travellers.”", "“H", ".", false},
		{"opening ellipsis", "… Harbour lights welcomed the travellers.", "… H", ". …", false},
		{"whole first word", "Harbour lights welcomed the travellers.", "Harbour", "Harbour", true},
		{"whole word before an unfinished second", "Harbour lights welcomed the travellers.", "Harbour l", "Harbour", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := headingFixture("Genesis", 1, map[int]string{
				1: "The traveller waited.",
				2: c.words,
			}, Heading{Text: "The Return", BeforeVerse: 2})
			from, end := f.pvs[0].end-1, f.pvs[1].body+len(c.end)
			raw := f.page[from:end]
			span := paneHeadingSpan(f.page, f.pvs, from, end)
			if got := paneHoldsScripture(f.page, f.pvs, from, end); got != c.study {
				t.Errorf("native panes' rule = %v, want %v", got, c.study)
			}
			for _, lead := range []bool{true, false} {
				placed := span
				placed.hasLead = lead
				if study, _, _, _ := studyMenuItems(f.st, raw, placed); study != c.study {
					t.Errorf("lead %v: menu study = %v, want %v", lead, study, c.study)
				}
				quote, _, _ := selectionQuotationIn(f.st, "Genesis", 1, raw, placed)
				if collapseSpaces(quote) != c.quote {
					t.Errorf("lead %v: quoted %q, want %q", lead, quote, c.quote)
				}
				if passage := aiPassageFor(f.st, raw, placed); collapseSpaces(passage.Text) != c.quote {
					t.Errorf("lead %v: prompt passage %q, want %q", lead, passage.Text, c.quote)
				}
			}
		})
	}
}

func TestACompleteWordAboveAHeadingKeepsStudyAfterTheBoundaryTrim(t *testing.T) {
	f := headingFixture("Genesis", 1, map[int]string{
		1: "The traveller waited.",
		2: "Harbour lights welcomed the travellers.",
	}, Heading{Text: "The Return", BeforeVerse: 2})
	from := f.pvs[0].body + strings.Index(f.page[f.pvs[0].body:f.pvs[0].end], "waited")
	end := f.pvs[1].body + len("H")
	raw, span := f.page[from:end], paneHeadingSpan(f.page, f.pvs, from, end)
	if !paneHoldsScripture(f.page, f.pvs, from, end) || !aiStudyOffered(f.st, raw, span) {
		t.Fatal("a complete word above the heading lost study")
	}
	if passage := aiPassageFor(f.st, raw, span); passage.Text != "waited." || passage.Ref != "Genesis 1:1" {
		t.Errorf("prompt passage = (%q, %q), want (waited., Genesis 1:1)", passage.Text, passage.Ref)
	}
}
