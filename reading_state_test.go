package bibletext

import (
	"testing"
)

// memPrefStore is an in-memory prefStore for exercising the persistence layer
// without a running Fyne app.
type memPrefStore struct{ m map[string]string }

func newMemPrefStore() *memPrefStore { return &memPrefStore{m: map[string]string{}} }

func (p *memPrefStore) String(key string) string { return p.m[key] }
func (p *memPrefStore) StringWithFallback(key, fb string) string {
	if v, ok := p.m[key]; ok {
		return v
	}
	return fb
}
func (p *memPrefStore) SetString(key, value string) { p.m[key] = value }

func TestReadingStateRoundTrip(t *testing.T) {
	p := newMemPrefStore()
	in := readingState{
		Version:     "web",
		Book:        "John",
		Chapter:     3,
		AnchorVerse: 16,
		AnchorDelta: 12.5,
		ScrollFrac:  0.42,
		TouchVerse:  14,
		TouchDelta:  6.5,
		Recent: []ChapterVisit{
			{Book: "John", Chapter: 3},
			{Book: "Genesis", Chapter: 1},
		},
	}
	writeReadingState(p, in)

	got, ok := readReadingState(p)
	if !ok {
		t.Fatal("readReadingState returned not-ok for a freshly written state")
	}
	if got.Version != in.Version || got.Book != in.Book || got.Chapter != in.Chapter {
		t.Errorf("version/book/chapter mismatch: %+v", got)
	}
	if got.AnchorVerse != 16 || got.AnchorDelta != 12.5 || got.ScrollFrac != 0.42 {
		t.Errorf("scroll anchor mismatch: verse=%d delta=%v frac=%v", got.AnchorVerse, got.AnchorDelta, got.ScrollFrac)
	}
	if got.TouchVerse != 14 || got.TouchDelta != 6.5 {
		t.Errorf("touch anchor mismatch: verse=%d delta=%v", got.TouchVerse, got.TouchDelta)
	}
	if len(got.Recent) != 2 || got.Recent[1].Book != "Genesis" {
		t.Errorf("recent mismatch: %+v", got.Recent)
	}
}

func TestReadReadingStateRejectsJunk(t *testing.T) {
	if _, ok := readReadingState(nil); ok {
		t.Error("nil store should report absent")
	}
	if _, ok := readReadingState(newMemPrefStore()); ok {
		t.Error("empty store should report absent")
	}
	p := newMemPrefStore()
	p.SetString(prefReadingState, "{not json")
	if _, ok := readReadingState(p); ok {
		t.Error("invalid JSON should report absent")
	}
	p.SetString(prefReadingState, `{"book":"","chapter":0}`)
	if _, ok := readReadingState(p); ok {
		t.Error("a state with no book/chapter should report absent")
	}
}

func TestApplyRestoredStateValid(t *testing.T) {
	base := baseSampleBible()
	book, ch := firstBookChapter(t, base)

	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
	}
	rs := readingState{
		Version: defaultVersionID, Book: book, Chapter: ch,
		AnchorVerse: 2, AnchorDelta: 8,
		Recent: []ChapterVisit{{Book: book, Chapter: ch}},
	}
	if !applyRestoredState(state, rs, base) {
		t.Fatal("applyRestoredState should succeed for a valid saved state")
	}
	if state.CurrentBook != book || state.CurrentChapter != ch {
		t.Errorf("restored position = %s %d, want %s %d", state.CurrentBook, state.CurrentChapter, book, ch)
	}
	if state.restore == nil || state.restore.Verse != 2 || state.restore.Book != book || state.restore.Chapter != ch {
		t.Errorf("pending restore anchor not set correctly: %+v", state.restore)
	}
	// No touch was recorded, so reopen uses the exact top-visible anchor (delta
	// preserved) and shows NO marker.
	if state.restore.Delta != 8 || state.restore.Marker != 0 {
		t.Errorf("no-touch restore should keep the anchor delta and set no marker: %+v", state.restore)
	}
	if len(state.RecentChapters) == 0 || state.RecentChapters[0].Book != book || state.RecentChapters[0].Chapter != ch {
		t.Errorf("current chapter must be at history head: %+v", state.RecentChapters)
	}
}

// TestApplyRestoredStatePrefersTouch verifies that WHEN THE FEATURE IS ON and the
// last scroll recorded an initial-touch verse, reopen anchors on THAT verse
// (brought to the top, delta 0) and marks it, in preference to the top-visible
// anchor.
func TestApplyRestoredStatePrefersTouch(t *testing.T) {
	touchResumeEnabled = true
	defer func() { touchResumeEnabled = false }()

	base := baseSampleBible()
	book, ch := firstBookChapter(t, base)
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
	}
	rs := readingState{
		Version: defaultVersionID, Book: book, Chapter: ch,
		AnchorVerse: 2, AnchorDelta: 8, // top-visible anchor (should be overridden)
		TouchVerse: 5, TouchDelta: 4, // the verse the finger grabbed
		Recent: []ChapterVisit{{Book: book, Chapter: ch}},
	}
	if !applyRestoredState(state, rs, base) {
		t.Fatal("applyRestoredState should succeed")
	}
	if state.restore == nil {
		t.Fatal("expected a pending restore")
	}
	if state.restore.Verse != 5 || state.restore.Delta != 0 {
		t.Errorf("reopen should anchor on the touched verse at the top: verse=%d delta=%v (want 5, 0)",
			state.restore.Verse, state.restore.Delta)
	}
	if state.restore.Marker != 5 {
		t.Errorf("the touched verse should be marked: marker=%d, want 5", state.restore.Marker)
	}
}

// TestApplyRestoredStateTouchDisabledUsesAnchor verifies that with the feature OFF
// (the default), a recorded touch verse is ignored: reopen uses the top-visible
// anchor and shows no marker.
func TestApplyRestoredStateTouchDisabledUsesAnchor(t *testing.T) {
	if touchResumeEnabled {
		t.Skip("feature is enabled in this build; this test covers the off path")
	}
	base := baseSampleBible()
	book, ch := firstBookChapter(t, base)
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
	}
	rs := readingState{
		Version: defaultVersionID, Book: book, Chapter: ch,
		AnchorVerse: 2, AnchorDelta: 8,
		TouchVerse: 5, TouchDelta: 4, // present, but must be ignored while off
		Recent: []ChapterVisit{{Book: book, Chapter: ch}},
	}
	if !applyRestoredState(state, rs, base) {
		t.Fatal("applyRestoredState should succeed")
	}
	if state.restore == nil {
		t.Fatal("expected a pending restore from the top-visible anchor")
	}
	if state.restore.Verse != 2 || state.restore.Delta != 8 {
		t.Errorf("feature off should use the top-visible anchor: verse=%d delta=%v (want 2, 8)",
			state.restore.Verse, state.restore.Delta)
	}
	if state.restore.Marker != 0 {
		t.Errorf("feature off should set no marker: marker=%d", state.restore.Marker)
	}
}

func TestApplyRestoredStateInvalidBookFallsBack(t *testing.T) {
	base := baseSampleBible()
	state := &AppState{Bible: base, CurrentVersion: defaultVersionID}
	rs := readingState{Version: defaultVersionID, Book: "Nonexiston", Chapter: 1}
	if applyRestoredState(state, rs, base) {
		t.Error("a saved book that no longer exists must not restore (caller falls back)")
	}
}

func TestApplyRestoredStateClampsChapter(t *testing.T) {
	base := baseSampleBible()
	book, _ := firstBookChapter(t, base)
	state := &AppState{Bible: base, CurrentVersion: defaultVersionID}
	rs := readingState{Version: defaultVersionID, Book: book, Chapter: 9999}
	if !applyRestoredState(state, rs, base) {
		t.Fatal("restore should succeed and clamp an out-of-range chapter")
	}
	nums := base.GetChapterNumbersForBook(book)
	if state.CurrentChapter != nums[0] {
		t.Errorf("chapter clamp = %d, want %d", state.CurrentChapter, nums[0])
	}
	// No scroll anchor was saved, so no pending restore should be set.
	if state.restore != nil {
		t.Errorf("no anchor saved → no pending restore, got %+v", state.restore)
	}
}

func TestRestoreRecentDropsInvalidAndDedups(t *testing.T) {
	base := baseSampleBible()
	book, ch := firstBookChapter(t, base)
	saved := []ChapterVisit{
		{Book: book, Chapter: ch},       // duplicate of current → should not double
		{Book: "Ghostbook", Chapter: 1}, // invalid → dropped
		{Book: book, Chapter: ch},       // another duplicate
	}
	out := restoreRecent(saved, base, book, ch)
	if len(out) != 1 {
		t.Fatalf("want only the current entry after dedup/drop, got %+v", out)
	}
	if out[0].Book != book || out[0].Chapter != ch {
		t.Errorf("head must be current chapter, got %+v", out[0])
	}
}

func TestNavigateToReference(t *testing.T) {
	base := baseSampleBible()
	book, ch := firstBookChapter(t, base)
	state := &AppState{
		Bible: base, CurrentVersion: defaultVersionID,
		CurrentBook: "Somewhere", CurrentChapter: 99,
	}
	navigateToReference(state, book, ch)
	if state.CurrentBook != book || state.CurrentChapter != ch {
		t.Fatalf("navigate set %s %d, want %s %d", state.CurrentBook, state.CurrentChapter, book, ch)
	}
	if len(state.RecentChapters) == 0 || state.RecentChapters[0].Book != book || state.RecentChapters[0].Chapter != ch {
		t.Errorf("visit not recorded at history head: %+v", state.RecentChapters)
	}
}

// firstBookChapter returns a real (book, chapter) from the sample Bible.
func firstBookChapter(t *testing.T, bd *BibleData) (string, int) {
	t.Helper()
	for _, b := range bd.Books {
		if nums := bd.GetChapterNumbersForBook(b); len(nums) > 0 {
			return b, nums[0]
		}
	}
	t.Fatal("sample Bible has no books with chapters")
	return "", 0
}
