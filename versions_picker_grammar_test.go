package bibletext

// THE PICKER'S FOOTER SENTENCES AGREE WITH A LIST WHOSE LENGTH IS DATA.
//
// Both were written when the list they described happened to have the length
// the wording assumed. Removing the NRSV left one evaluation version and the

// reasonably amused). The bring-your-own-key line had the mirror-image fault
// waiting: it said "unlocks", so a second BYOK translation would have produced
// "X and Y unlocks".
//
// Registering or removing a translation is a normal thing to do, so the
// sentences are tested at every length that changes the verb.

import (
	"strings"
	"testing"
)

// evaluationNote and byokNote mirror the sentences built in buildVersionPicker.
// They live here as functions of the list so the WORDING can be asserted without
// a canvas — the same reason linkVersionUnavailable is separate from its card.
func evaluationNote(names []string) string {
	return joinNatural(names) + pick(len(names), " is", " are") +
		" under evaluation and not yet selectable; " +
		pick(len(names), "it unlocks", "they unlock") +
		" once licensing is complete."
}

func byokNote(names []string) string {
	return joinNatural(names) + pick(len(names), " unlocks", " unlock") +
		" with your own free API.Bible key — add it in Settings."
}

func TestPickerNotesAgreeWithTheirListLength(t *testing.T) {
	for _, tc := range []struct {
		names    []string
		wantHas  []string
		wantLack []string
	}{
		{[]string{"LSB"}, []string{"LSB is under evaluation", "it unlocks"}, []string{" are ", "they unlock"}},
		{[]string{"LSB", "NKJV"}, []string{"LSB and NKJV are under evaluation", "they unlock"}, []string{" is under", "it unlocks"}},
		{[]string{"A", "B", "C"}, []string{"A, B and C are under evaluation"}, []string{" is under"}},
	} {
		got := evaluationNote(tc.names)
		for _, want := range tc.wantHas {
			if !strings.Contains(got, want) {
				t.Errorf("%d name(s): %q does not contain %q", len(tc.names), got, want)
			}
		}
		for _, bad := range tc.wantLack {
			if strings.Contains(got, bad) {
				t.Errorf("%d name(s): %q still contains %q", len(tc.names), got, bad)
			}
		}
	}

	if got := byokNote([]string{"NKJV"}); !strings.Contains(got, "NKJV unlocks with") {
		t.Errorf("one BYOK name should take the singular verb; got %q", got)
	}
	if got := byokNote([]string{"NKJV", "Other"}); !strings.Contains(got, "NKJV and Other unlock with") {
		t.Errorf("two BYOK names should take the plural verb; got %q", got)
	}
}
