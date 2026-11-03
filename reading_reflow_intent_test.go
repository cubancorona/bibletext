package bibletext

import "testing"

func TestReadingReflowYieldsToExplicitRestoreAndArrival(t *testing.T) {
	for _, test := range []struct {
		name          string
		same, arrival bool
		restore       *restoreAnchor
		want          bool
	}{
		{"plain same chapter", true, false, nil, true},
		{"layout carry", true, false, &restoreAnchor{Verse: 4, Reflow: true}, true},
		{"history same chapter", true, false, &restoreAnchor{Verse: 4}, false},
		{"explicit chapter top", true, false, &restoreAnchor{}, false},
		{"explicit arrival", true, true, nil, false},
		{"new chapter", false, false, nil, false},
	} {
		if got := readingReflowWanted(test.same, test.arrival, test.restore); got != test.want {
			t.Errorf("%s: reflow=%v, want %v", test.name, got, test.want)
		}
	}
}
