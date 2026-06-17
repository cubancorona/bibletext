package bibletext

import "testing"

// chapterRenderFingerprint is the gate that lets the native reading overlay skip
// re-importing identical chapter HTML. These tests pin the load-bearing property:
// it stays stable for the same render but changes whenever anything that affects
// buildChapterHTML's output changes — otherwise navigation would show stale text.
func TestChapterRenderFingerprintStableForSameRender(t *testing.T) {
	s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	if a, b := chapterRenderFingerprint(s), chapterRenderFingerprint(s); a != b {
		t.Fatalf("fingerprint not stable for identical state: %q vs %q", a, b)
	}
}

func TestChapterRenderFingerprintChangesOnNavigation(t *testing.T) {
	base := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
	baseFP := chapterRenderFingerprint(base)

	cases := []struct {
		name   string
		mutate func(*AppState)
	}{
		{"chapter", func(s *AppState) { s.CurrentChapter = 4 }},
		{"book", func(s *AppState) { s.CurrentBook = "Mark" }},
		{"version", func(s *AppState) { s.CurrentVersion = "nrsv" }},
		{"highlight on", func(s *AppState) {
			s.HasHighlightedVerse = true
			s.HighlightedBook = "John"
			s.HighlightedChapter = 3
			s.HighlightedVerse = 16
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3}
			tc.mutate(s)
			if got := chapterRenderFingerprint(s); got == baseFP {
				t.Fatalf("fingerprint did not change after %s change: still %q", tc.name, got)
			}
		})
	}
}

// Two different highlighted verses in the same chapter must differ, since the
// gate decides whether a search-jump (highlight on a specific verse) re-renders.
func TestChapterRenderFingerprintDistinguishesHighlightedVerse(t *testing.T) {
	mk := func(v int) *AppState {
		return &AppState{
			CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3,
			HasHighlightedVerse: true, HighlightedBook: "John", HighlightedChapter: 3, HighlightedVerse: v,
		}
	}
	if chapterRenderFingerprint(mk(16)) == chapterRenderFingerprint(mk(17)) {
		t.Fatal("fingerprint should differ for different highlighted verses")
	}
}
