package bibletext

// SAVED NOTES STAY READABLE ACROSS BUILDS. The note store is the one place a
// reader's notes, and notes other people sent, exist, and it is read and
// rewritten by whichever build is installed, older or newer. These hold the
// stored record's codec (StoredNote's MarshalJSON and UnmarshalJSON,
// notes_model.go) to what that needs: a record an earlier build wrote comes
// back out byte for byte, so an unchanged store writes unchanged, and a field
// a later build added survives this build's rewrite; and a note's anchor runs,
// the field added after the first stores were written, round-trip without
// leaking into the unknown fields. They began as a probe of the anchor-runs
// change, and are kept because the property is permanent.
//
// The last test holds the parser of a link's anchor spelling, which is where
// a received note's runs come from, to giving no runs for a malformed one.

import (
	"bytes"
	"encoding/json"
	"testing"
)

// A record written before anchor runs existed ("ar" absent), with a field no
// build knows yet, comes back out byte for byte. Mutation guarded: "ar"
// emitted when empty, which writes "ar":null into every old record.
func TestAnOldSavedNoteRoundTripsByteForByte(t *testing.T) {
	old := []byte(`{"id":7,"k":"received","v":"web","b":"John","c":3,"lo":16,"t":"hello","ts":1700000000,"zz_future":"kept"}`)
	var n StoredNote
	if err := json.Unmarshal(old, &n); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, old) {
		t.Errorf("an old record changed on its way through this build's codec:\n was %s\n now %s", old, out)
	}
	if string(n.Extra["zz_future"]) != `"kept"` {
		t.Errorf("a later build's field was not kept: Extra = %v", n.Extra)
	}
}

// Anchor runs round-trip, are written where the codec puts them, never land
// in Extra, and a second pass writes the same bytes as the first.
func TestASavedNotesAnchorRunsRoundTrip(t *testing.T) {
	var n StoredNote
	if err := json.Unmarshal([]byte(`{"id":7,"k":"received","v":"web","b":"John","c":3,"lo":16,"t":"hello","ts":1700000000,"zz_future":"kept"}`), &n); err != nil {
		t.Fatal(err)
	}
	n.AnchorRuns = []anchorRun{{Chapter: 3, Lo: 16}, {Chapter: 3, Lo: 18, Hi: 19}}
	first, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"id":7,"k":"received","v":"web","b":"John","c":3,"lo":16,"ar":[{"c":3,"lo":16},{"c":3,"lo":18,"hi":19}],"t":"hello","ts":1700000000,"zz_future":"kept"}`
	if string(first) != want {
		t.Errorf("a note with anchor runs is written as\n %s\nwant\n %s", first, want)
	}
	var back StoredNote
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.AnchorRuns) != 2 || back.AnchorRuns[0] != n.AnchorRuns[0] || back.AnchorRuns[1] != n.AnchorRuns[1] {
		t.Errorf("the anchor runs did not round-trip: %+v", back.AnchorRuns)
	}
	if _, leaked := back.Extra["ar"]; leaked {
		t.Error(`"ar" was kept as an unknown field as well as read as the anchor runs`)
	}
	second, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(second, first) {
		t.Errorf("a second pass changed the record:\n first %s\nsecond %s", first, second)
	}
}

// A malformed anchor spelling in a link gives no runs at all, rather than a
// partial or inverted set, and a well-formed one gives each verse its run.
func TestAMalformedAnchorSpellingGivesNoRuns(t *testing.T) {
	for _, s := range []string{"", "0", "-1", "5-3", "abc", "1,,2", "1-", "1,x", "3-x", ",1"} {
		if runs := noteRunsFromSpelling(s); len(runs) != 0 {
			t.Errorf("noteRunsFromSpelling(%q) = %+v, want no runs", s, runs)
		}
	}
	if runs := noteRunsFromSpelling("1,2-4,9"); len(runs) != 3 ||
		runs[0] != (NoteVerseRun{Lo: 1, Hi: 1}) || runs[1] != (NoteVerseRun{Lo: 2, Hi: 4}) ||
		runs[2] != (NoteVerseRun{Lo: 9, Hi: 9}) {
		t.Errorf(`noteRunsFromSpelling("1,2-4,9") = %+v, want 1, 2-4 and 9`, runs)
	}
	// Whatever a spelling holds, a run it gives starts at verse 1 or later
	// and never ends before it starts.
	for _, s := range []string{"4294967295", "1-4294967295", "7-7", "2,1"} {
		for _, r := range noteRunsFromSpelling(s) {
			if r.Lo < 1 || r.Hi < r.Lo {
				t.Errorf("noteRunsFromSpelling(%q) gave the run %+v", s, r)
			}
		}
	}
}
