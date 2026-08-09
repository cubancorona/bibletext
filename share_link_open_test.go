package bibletext

// What happens when someone taps a shared link. The two tests that matter are
// the startup race (a link arrives before the Bible has loaded — the COMMON
// case) and the scroll-restore collision, which is silent and intermittent when
// it goes wrong.

import "testing"

func linkTestState() *AppState {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible:          bd,
		CurrentBook:    "Genesis",
		CurrentChapter: 1,
		loadPhase:      loadReady,
	}
}

// TestHandleShareLinkNavigatesAndHighlights: the whole point — a tapped link
// lands on the passage with the shared verses lit up.
func TestHandleShareLinkNavigatesAndHighlights(t *testing.T) {
	st := linkTestState()
	if !HandleShareLink(st, "https://bibletext.co.uk/web/john/3/#v16-18") {
		t.Fatal("a valid reader link must be handled by the app")
	}
	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("landed on %s %d, want John 3", st.CurrentBook, st.CurrentChapter)
	}
	if !st.HasHighlightedVerse || st.HighlightedVerse != 16 || st.HighlightedVerseEnd != 18 {
		t.Errorf("highlight = %v %d-%d, want 16-18",
			st.HasHighlightedVerse, st.HighlightedVerse, st.HighlightedVerseEnd)
	}
}

// TestHandleShareLinkIgnoresForeignURLs: anything that is not a reader passage
// must be left to the browser — above all privacy.html and support.html, the
// URLs App Store Connect points at.
func TestHandleShareLinkIgnoresForeignURLs(t *testing.T) {
	for _, url := range []string{
		"https://bibletext.co.uk/privacy.html",
		"https://bibletext.co.uk/support.html",
		"https://bibletext.co.uk/",
		"https://example.com/web/john/3/",
		"",
	} {
		st := linkTestState()
		if HandleShareLink(st, url) {
			t.Errorf("%q must NOT be claimed by the app", url)
		}
		if st.CurrentBook != "Genesis" {
			t.Errorf("%q moved the reader to %s — it should have been ignored", url, st.CurrentBook)
		}
	}
}

// TestShareLinkArrivingDuringLoadIsParkedNotDropped: the OS delivers the link
// within milliseconds of launch, while the Bible is still parsing. Dropping it
// would mean a tapped link opens the app at the wrong place — the failure
// people would actually hit.
func TestShareLinkArrivingDuringLoadIsParkedNotDropped(t *testing.T) {
	st := linkTestState()
	st.loadPhase = loadPending

	if !HandleShareLink(st, "https://bibletext.co.uk/web/john/3/#v16") {
		t.Fatal("a link arriving during load must still be accepted")
	}
	if st.pendingLink == nil {
		t.Fatal("the link was dropped instead of parked")
	}
	if st.CurrentBook != "Genesis" {
		t.Error("navigation must wait for the data, not happen mid-load")
	}

	// The data lands.
	st.loadPhase = loadReady
	consumePendingLink(st)
	if st.CurrentBook != "John" || st.CurrentChapter != 3 || st.HighlightedVerse != 16 {
		t.Errorf("after loading, want John 3:16; got %s %d:%d",
			st.CurrentBook, st.CurrentChapter, st.HighlightedVerse)
	}
	if st.pendingLink != nil {
		t.Error("the parked link must be consumed exactly once")
	}
}

// TestShareTargetClearsTheSavedScrollRestore pins the INVARIANT, not one line:
// after opening a shared link the saved scroll target must be gone, or the app
// shows the shared verse and then silently scrolls to wherever the reader was
// last session. Today addRecentChapter clears it and applyShareTarget clears it
// again for good measure — so this test survives either changing, which is the
// point of asserting the outcome rather than the mechanism.
func TestShareTargetClearsTheSavedScrollRestore(t *testing.T) {
	st := linkTestState()
	st.restore = &restoreAnchor{Verse: 42}

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16})

	if st.restore != nil {
		t.Error("the saved scroll target must be cleared, or the reader is scrolled " +
			"away from the verse someone just shared with them")
	}
}

// TestShareTargetClampsOutOfRangeChapters: links live for years and canons
// differ. Landing on the nearest real chapter beats an error page.
func TestShareTargetClampsOutOfRangeChapters(t *testing.T) {
	st := linkTestState()
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 999, VerseLo: 1})
	if st.CurrentBook != "John" {
		t.Fatalf("want John, got %s", st.CurrentBook)
	}
	if max := st.Bible.GetChaptersForBook("John"); st.CurrentChapter != max {
		t.Errorf("chapter clamped to %d, want %d", st.CurrentChapter, max)
	}

	// A book outside this canon leaves the reader where they are.
	before := st.CurrentBook
	applyShareTarget(st, ShareTarget{VersionID: "webc", Book: "Tobit", Chapter: 1})
	if st.CurrentBook != before {
		t.Errorf("a book absent from this translation moved the reader to %s", st.CurrentBook)
	}
}
