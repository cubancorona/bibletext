package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// seedState is the shape a FIRST INSTALL is in: the embedded four-book Gospels
// seed on screen, the complete Bible still downloading. Every link naming a book
// outside Matthew–John lands in that gap, and it is the single most likely state
// for a shared link to arrive in — someone is sent a verse, installs the app from
// the App Store, and comes back to the message to tap it.
func seedState(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"John": {3: {
			{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world."},
		}},
	}
	bd.Books = []string{"John"} // the seed carries the Gospels only
	bd.PrepareSearchIndex()
	return &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
		fullPending:    true, // the full download is in flight
		seedOnly:       true, // and what is on screen really is the seed
	}
}

// fullBible is what triggerFullDownload eventually swaps in.
//
// Chapters must be CONTIGUOUS from 1: applyShareTarget clamps against
// GetChaptersForBook, which is a COUNT (bible.go:266), so a book holding only
// chapter 23 reports one chapter and a link to 23 clamps to 1 — an artefact of
// the fixture, not of the app, but one that quietly turns a real assertion into
// a false failure.
func fullBible() *BibleData {
	bd := NewBibleData()
	chapters := func(book string, upTo int, text string) map[int][]Verse {
		m := map[int][]Verse{}
		for c := 1; c <= upTo; c++ {
			m[c] = []Verse{{BookName: book, Chapter: c, Verse: 1, Text: text}}
		}
		return m
	}
	bd.Verses = map[string]map[int][]Verse{
		"John":   chapters("John", 3, "For God so loved the world."),
		"Psalms": chapters("Psalms", 23, "Yahweh is my shepherd."),
		"Romans": chapters("Romans", 8, "All things work together for good."),
	}
	bd.Books = []string{"Psalms", "Romans", "John"}
	bd.PrepareSearchIndex()
	return bd
}

// A link for a book the seed does not carry must be HELD, not dropped. Before
// the fix applyShareTarget returned on chapters == 0 and the tap did nothing at
// all — no passage, no message, nothing parked, and the sender's note gone.
func TestSeedInstallParksLinksForBooksNotYetDownloaded(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	for _, tc := range []struct {
		name     string
		target   ShareTarget
		wantPark bool
	}{
		{"in the seed", ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16}, false},
		{"outside the seed", ShareTarget{VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1}, true},
		{"outside the seed, with a note", ShareTarget{VersionID: "web", Book: "Romans", Chapter: 8, VerseLo: 28, Note: "synthetic note"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := seedState(t)
			applyShareTarget(st, tc.target)
			if tc.wantPark {
				if st.pendingLink == nil {
					t.Fatalf("link for %s %d was claimed and then dropped: nothing parked, still on %s %d",
						tc.target.Book, tc.target.Chapter, st.CurrentBook, st.CurrentChapter)
				}
				if st.pendingLink.Book != tc.target.Book {
					t.Errorf("parked %s, want %s", st.pendingLink.Book, tc.target.Book)
				}
				// The version slot means "waiting for a translation"; using it here
				// would let the next version load consume or bin this target.
				if st.pendingLinkVersion != "" {
					t.Errorf("pendingLinkVersion = %q, want empty — this park belongs to the full download",
						st.pendingLinkVersion)
				}
			} else if st.CurrentBook != tc.target.Book {
				t.Errorf("a book the seed HAS should open immediately; on %s %d", st.CurrentBook, st.CurrentChapter)
			}
		})
	}
}

// ...and when the full text lands, the held link must actually open. Parking
// without a consumer would just be a slower way to lose it.
func TestParkedSeedLinkOpensWhenTheFullBibleLands(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := seedState(t)
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "Psalms", Chapter: 23, VerseLo: 1, Note: "for you"})
	if st.pendingLink == nil {
		t.Fatal("precondition: the link should have been parked")
	}

	// The success tail of triggerFullDownload, in the order the real one runs.
	st.Bible = fullBible()
	st.fullPending = false
	st.seedOnly = false
	consumePendingLink(st)

	if st.CurrentBook != "Psalms" || st.CurrentChapter != 23 {
		t.Errorf("after the download the reader is on %s %d, want Psalms 23", st.CurrentBook, st.CurrentChapter)
	}
	if !st.hlOn() || st.hlLo() != 1 {
		t.Errorf("the shared verse is not highlighted (has=%v verse=%d)", st.hlOn(), st.hlLo())
	}
	if st.ActiveNote != "for you" {
		t.Errorf("the sender's note did not survive the wait: %q", st.ActiveNote)
	}
	if st.pendingLink != nil {
		t.Error("the parked link was not cleared after being applied")
	}
}

// A book that genuinely does not exist must still be a no-op — the park is for
// text that has not arrived, not for bad payloads.
func TestUnknownBookIsStillIgnoredOnceTheBibleIsComplete(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := seedState(t)
	st.Bible = fullBible()
	st.fullPending, st.seedOnly = false, false

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "Nonexistent", Chapter: 1})
	if st.pendingLink != nil {
		t.Error("a link for a book that does not exist was parked; it will never arrive")
	}
	if st.CurrentBook != "John" {
		t.Errorf("the reader was moved to %s by a bad payload", st.CurrentBook)
	}
}
