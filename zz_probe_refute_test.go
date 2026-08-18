package bibletext

import (
	"encoding/json"
	"testing"
)

// Probe: an old-build record (no "ar") round-trips byte-identically through
// the new build's codec, and a record with runs round-trips its runs.
func TestProbeOldShapeByteIdentity(t *testing.T) {
	old := []byte(`{"id":7,"k":"received","v":"web","b":"John","c":3,"lo":16,"t":"hello","ts":1700000000,"zz_future":"kept"}`)
	var n StoredNote
	if err := json.Unmarshal(old, &n); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("old bytes:  %s", old)
	t.Logf("new bytes:  %s", out)
	var n2 StoredNote
	if err := json.Unmarshal(out, &n2); err != nil {
		t.Fatal(err)
	}
	if n2.Extra["zz_future"] == nil {
		t.Error("future field lost")
	}

	withRuns := n
	withRuns.AnchorRuns = []anchorRun{{Chapter: 3, Lo: 16}, {Chapter: 3, Lo: 18, Hi: 19}}
	b, _ := json.Marshal(withRuns)
	t.Logf("with runs:  %s", b)
	var n3 StoredNote
	if err := json.Unmarshal(b, &n3); err != nil {
		t.Fatal(err)
	}
	if len(n3.AnchorRuns) != 2 || n3.AnchorRuns[1].Hi != 19 {
		t.Errorf("runs did not round trip: %+v", n3.AnchorRuns)
	}
	if n3.Extra["ar"] != nil {
		t.Error("ar leaked into Extra")
	}
}

// Probe: hostile spelling strings through noteRunsFromSpelling.
func TestProbeHostileSpellings(t *testing.T) {
	for _, s := range []string{"", "0", "-1", "5-3", "1,2,3", "abc", "1,,2", "1-", "4294967295", "1-4294967295"} {
		t.Logf("%q => %+v", s, noteRunsFromSpelling(s))
	}
}
