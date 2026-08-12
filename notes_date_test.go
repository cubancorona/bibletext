package bibletext

import (
	"testing"
	"time"
)

// The browser's date line is the only place a reader can see WHEN a note
// arrived, and "Newest first" is the default sort — so a wrong label here
// quietly contradicts the order the list is in.
func TestNoteDateLabel(t *testing.T) {
	// A fixed "now" so the boundaries are exact rather than whenever the suite
	// happens to run. Mid-afternoon on purpose: a note from 23:50 last night is
	// "Yesterday" even though it is only ~15 hours ago, because the label counts
	// CALENDAR days, which is how a reader reads it.
	now := time.Date(2026, time.August, 12, 15, 0, 0, 0, time.UTC)
	at := func(y int, m time.Month, d, h int) int64 {
		return time.Date(y, m, d, h, 0, 0, 0, time.UTC).Unix()
	}

	for _, tc := range []struct {
		name string
		ts   int64
		want string
	}{
		{"no timestamp at all", 0, ""},
		{"negative timestamp", -1, ""},
		{"this morning", at(2026, time.August, 12, 8), "Today"},
		{"one minute ago", now.Add(-time.Minute).Unix(), "Today"},
		{"later today", at(2026, time.August, 12, 23), "Today"},
		{"clock skew, arrives tomorrow", at(2026, time.August, 13, 9), "Today"},
		{"late last night", at(2026, time.August, 11, 23), "Yesterday"},
		{"two days", at(2026, time.August, 10, 12), "2 days ago"},
		{"six days is the last relative one", at(2026, time.August, 6, 12), "6 days ago"},
		{"seven days becomes a date", at(2026, time.August, 5, 12), "5 Aug"},
		{"earlier this year", at(2026, time.February, 3, 12), "3 Feb"},
		{"last year carries the year", at(2025, time.December, 31, 12), "31 Dec 2025"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := noteDateLabel(tc.ts, now); got != tc.want {
				t.Errorf("noteDateLabel(%d) = %q, want %q", tc.ts, got, tc.want)
			}
		})
	}
}

// The switch from "N days ago" to a date must land on the same day whichever
// direction you approach it from — an off-by-one here shows up as a note that
// says "7 days ago" one refresh and "5 Aug" the next.
func TestNoteDateLabelBoundaryIsStable(t *testing.T) {
	now := time.Date(2026, time.August, 12, 0, 30, 0, 0, time.UTC) // just past midnight
	sixDays := time.Date(2026, time.August, 6, 23, 59, 0, 0, time.UTC).Unix()
	if got := noteDateLabel(sixDays, now); got != "6 days ago" {
		t.Errorf("a note from six calendar days back = %q, want %q", got, "6 days ago")
	}
	sevenDays := time.Date(2026, time.August, 5, 0, 1, 0, 0, time.UTC).Unix()
	if got := noteDateLabel(sevenDays, now); got != "5 Aug" {
		t.Errorf("a note from seven calendar days back = %q, want %q", got, "5 Aug")
	}
}
