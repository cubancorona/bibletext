package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// A Go-to and its followed reference can both land without any user scroll.
// Their history entries still need a place to return to; a measured scroll
// after arrival must supersede that fallback.
func TestVerseArrivalReturnsThroughCrossReferenceHistory(t *testing.T) {
	for _, scrolled := range []bool{false, true} {
		name := "without_scroll"
		if scrolled {
			name = "newer_scroll"
		}
		t.Run(name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			st := verseArrivalHistoryState()
			capture, touch := captureAnchorFn, touchResumeEnabled
			captures, measured := 0, false
			touchResumeEnabled = false
			captureAnchorFn = func() (int, float64, float64, bool) {
				captures++
				if measured {
					return 42, 7.5, 0.6, true
				}
				return 2, 11, 0.1, true // a pane still showing the outgoing chapter
			}
			t.Cleanup(func() { captureAnchorFn, touchResumeEnabled = capture, touch })

			goToVerseRange(st, "Matthew", 5, 43, 43)
			want := ChapterVisit{Book: "Matthew", Chapter: 5, Verse: 43}
			if saved, ok := readReadingState(appPrefs()); !ok || saved.Version != "web" ||
				saved.Book != "Matthew" || saved.Chapter != 5 || saved.AnchorVerse != 43 ||
				saved.AnchorDelta != 0 || saved.ScrollFrac != 0 ||
				len(saved.Recent) == 0 || saved.Recent[0] != want {
				t.Fatalf("arrival did not save its identity and history position: %+v, present=%v", saved, ok)
			}
			wantCaptures := 0
			if scrolled {
				// Exercise the successful capture path used by a completed scroll,
				// without a native view or an asynchronous preference write.
				measured = true
				snapshot := captureSnapshot(st, appPrefs())
				measured = false
				wantCaptures = 1
				want.Verse, want.Delta, want.Frac = 42, 7.5, 0.6
				if snapshot.Version != "web" || snapshot.Book != "Matthew" || snapshot.Chapter != 5 ||
					snapshot.AnchorVerse != want.Verse || snapshot.AnchorDelta != want.Delta || snapshot.ScrollFrac != want.Frac ||
					len(snapshot.Recent) == 0 || snapshot.Recent[0] != want {
					t.Fatalf("successful capture did not replace the arrival fallback: %+v", snapshot)
				}
			}
			followCrossRef(st, crossRef{Book: "Leviticus", Chapter: 19, Verse: 18})
			back := verseArrivalHistoryVisit(t, st, "Matthew", 5)
			navigateToVisit(st, back)
			if back != want || st.restore == nil || st.restore.Verse != want.Verse ||
				st.restore.Delta != want.Delta || st.restore.Frac != want.Frac {
				t.Fatalf("history returned visit %+v with restore %+v, want %+v", back, st.restore, want)
			}
			if st.hasMark() {
				t.Error("history must restore the reading place without reviving the arrival highlight")
			}
			if saved, ok := readReadingState(appPrefs()); !ok || saved.AnchorVerse != want.Verse ||
				saved.AnchorDelta != want.Delta || saved.ScrollFrac != want.Frac {
				t.Fatalf("returned place was not persisted: %+v, present=%v", saved, ok)
			}
			// The followed target is itself an explicit verse arrival. Returning
			// there without scrolling must retain verse 18 as well.
			navigateToVisit(st, verseArrivalHistoryVisit(t, st, "Leviticus", 19))
			if st.restore == nil || st.restore.Verse != 18 {
				t.Fatalf("followed target lost its return position: %+v", st.restore)
			}
			if captures != wantCaptures {
				t.Errorf("capture calls=%d, want only the %d explicit successful capture", captures, wantCaptures)
			}
		})
	}
}

func TestRepeatedVerseArrivalReplacesHistoryFallback(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	st := verseArrivalHistoryState()
	goToVerseRange(st, "Matthew", 5, 43, 43)
	updateCurrentVisitAnchor(st, 42, 7.5, 0.6)
	goToVerseRange(st, "Matthew", 5, 38, 40)
	if st.restore != nil || !st.forceReposition {
		t.Fatalf("arrival changed placement priority: restore=%+v force=%v", st.restore, st.forceReposition)
	}
	if saved, ok := readReadingState(appPrefs()); !ok || saved.Version != "web" || saved.Book != "Matthew" ||
		saved.Chapter != 5 || saved.AnchorVerse != 38 || saved.AnchorDelta != 0 || saved.ScrollFrac != 0 ||
		len(saved.Recent) == 0 || saved.Recent[0] != (ChapterVisit{Book: "Matthew", Chapter: 5, Verse: 38}) {
		t.Fatalf("unscrolled arrival would reopen at %+v, present=%v", saved, ok)
	}
	followCrossRef(st, crossRef{Book: "Leviticus", Chapter: 19, Verse: 18})
	navigateToVisit(st, verseArrivalHistoryVisit(t, st, "Matthew", 5))
	if st.restore == nil || st.restore.Verse != 38 || st.restore.Delta != 0 || st.restore.Frac != 0 {
		t.Fatalf("new same-chapter arrival retained an older place: %+v", st.restore)
	}
}

func TestChapterAndInvalidVerseArrivalsKeepAnchorlessHistory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		arrive func(*AppState)
	}{
		{"chapter", func(st *AppState) { navigateToReference(st, "Matthew", 5) }},
		{"title", func(st *AppState) { goToChapterTitle(st, "Matthew", 5) }},
		{"missing_verse", func(st *AppState) { goToVerseRange(st, "Matthew", 5, 99, 99) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			st := verseArrivalHistoryState()
			tc.arrive(st)
			followCrossRef(st, crossRef{Book: "Leviticus", Chapter: 19, Verse: 18})
			back := verseArrivalHistoryVisit(t, st, "Matthew", 5)
			navigateToVisit(st, back)
			if back.Verse != 0 || back.Delta != 0 || back.Frac != 0 || st.restore != nil {
				t.Fatalf("anchorless visit invented a position: visit=%+v restore=%+v", back, st.restore)
			}
		})
	}
}

func verseArrivalHistoryState() *AppState {
	return &AppState{
		Bible:          xrefBible(map[string]map[int]int{"Matthew": {5: 48}, "Leviticus": {19: 37}}),
		CurrentVersion: "web", CurrentBook: "Leviticus", CurrentChapter: 19,
		RecentChapters: []ChapterVisit{{Book: "Leviticus", Chapter: 19}},
	}
}

func verseArrivalHistoryVisit(t *testing.T, st *AppState, book string, chapter int) ChapterVisit {
	t.Helper()
	for _, visit := range recentJumpTargets(st, maxRecent-1) {
		if visit.Book == book && visit.Chapter == chapter {
			return visit
		}
	}
	t.Fatalf("history omitted %s %d", book, chapter)
	return ChapterVisit{}
}
