package bibletext

import (
	"strings"
	"testing"
)

func androidSavedRestoreTestState() *AppState {
	return &AppState{CurrentVersion: "web", CurrentBook: "Example", CurrentChapter: 3,
		restore: &restoreAnchor{Book: "Example", Chapter: 3, Verse: 4, Delta: .375, Frac: .2}}
}

func TestAndroidRestoreRepetitionRequiresTheExactSuccessfulDispatch(t *testing.T) {
	s := androidSavedRestoreTestState()
	r := s.restore
	var d androidReadingDispatch
	intent, id := d.prepare(s)
	if d.repeatsSavedRestore(s) {
		t.Fatal("planning a first request is not a successful native dispatch")
	}
	d.dispatched(intent, id+1)
	if d.repeatsSavedRestore(s) {
		t.Fatal("a different dispatch generation promoted the restore")
	}
	d.dispatched(intent, id)
	if !d.repeatsSavedRestore(s) {
		t.Fatal("the successfully dispatched identical request must allow live carry")
	}
	_, repeatedID := d.prepare(s)
	if repeatedID == id || s.restore != r || r.Reflow || r.Verse != 4 || r.Delta != .375 || r.Frac != .2 {
		t.Fatal("repetition must retain the original request and placement fallback")
	}
	for _, change := range []struct {
		name  string
		apply func(*AppState) *AppState
	}{
		{"new state", func(s *AppState) *AppState {
			return &AppState{CurrentVersion: s.CurrentVersion, CurrentBook: s.CurrentBook,
				CurrentChapter: s.CurrentChapter, restore: s.restore}
		}},
		{"new equal-coordinate history", func(s *AppState) *AppState { other := *s.restore; s.restore = &other; return s }},
		{"new version", func(s *AppState) *AppState { s.CurrentVersion = "other"; return s }},
		{"new book", func(s *AppState) *AppState { s.CurrentBook = "Other"; return s }},
		{"new chapter", func(s *AppState) *AppState { s.CurrentChapter++; return s }},
		{"no restore", func(s *AppState) *AppState { s.restore = nil; return s }},
		{"explicit arrival", func(s *AppState) *AppState { s.forceReposition = true; return s }},
		{"transient carry", func(s *AppState) *AppState { s.restore.Reflow = true; return s }},
	} {
		t.Run(change.name, func(t *testing.T) {
			s := androidSavedRestoreTestState()
			var owner androidReadingDispatch
			intent, id := owner.prepare(s)
			owner.dispatched(intent, id)
			if owner.repeatsSavedRestore(change.apply(s)) {
				t.Fatal("a new intent inherited the old saved request's live-carry permission")
			}
		})
	}
}

func TestAndroidScrollEventCannotRetireANewerReadingRequest(t *testing.T) {
	for _, when := range []string{"before replacement dispatch", "after replacement dispatch"} {
		t.Run(when, func(t *testing.T) {
			s := androidSavedRestoreTestState()
			var d androidReadingDispatch
			old, oldID := d.prepare(s)
			d.dispatched(old, oldID)
			// This is the actual queued Fyne operation: only the source event's
			// generation is captured on receipt; current state is read at execution.
			queued := func() bool { return d.readerScrolled(s, oldID) }
			newRestore := *s.restore
			s.restore = &newRestore
			if when == "after replacement dispatch" {
				fresh, freshID := d.prepare(s)
				if freshID == oldID {
					t.Fatal("equal coordinates reused the old history event identity")
				}
				d.dispatched(fresh, freshID)
			}
			if queued() || s.restore != &newRestore {
				t.Fatal("a queued old scroll retired the fresh equal-coordinate history request")
			}
		})
	}
	t.Run("failed repeat keeps its successfully issued saved request", func(t *testing.T) {
		s := androidSavedRestoreTestState()
		r := s.restore
		var d androidReadingDispatch
		first, firstID := d.prepare(s)
		d.dispatched(first, firstID)
		_, failedID := d.prepare(s) // This later native dispatch fails.
		if failedID == firstID || !d.repeatsSavedRestore(s) || s.restore != r || r.Reflow {
			t.Fatal("a failed live repeat lost its exact prior successful carry owner")
		}
		if d.readerScrolled(s, firstID) || d.readerScrolled(s, failedID) || s.restore != r {
			t.Fatal("repeat carry relaxed the strict scroll-event generation fence")
		}
	})
	t.Run("replacement native page with the same request", func(t *testing.T) {
		s := androidSavedRestoreTestState()
		var d androidReadingDispatch
		first, firstID := d.prepare(s)
		d.dispatched(first, firstID)
		if !d.repeatsSavedRestore(s) {
			t.Fatal("same request lost repeat classification")
		}
		second, secondID := d.prepare(s)
		d.dispatched(second, secondID)
		if firstID == secondID || d.readerScrolled(s, firstID) || s.restore == nil {
			t.Fatal("an old native page retired the replacement page's restore")
		}
	})
	t.Run("new navigation without restore", func(t *testing.T) {
		s := androidSavedRestoreTestState()
		var d androidReadingDispatch
		old, id := d.prepare(s)
		d.dispatched(old, id)
		s.restore = nil
		s.CurrentChapter++
		if d.readerScrolled(s, id) {
			t.Fatal("an old page's event was accepted for new plain navigation")
		}
	})
	t.Run("new arrival before failed dispatch", func(t *testing.T) {
		s := androidSavedRestoreTestState()
		var d androidReadingDispatch
		old, id := d.prepare(s)
		d.dispatched(old, id)
		s.forceReposition = true
		if d.readerScrolled(s, id) {
			t.Fatal("an event consumed a fresh explicit arrival before dispatch")
		}
		_, freshID := d.prepare(s)
		s.forceReposition = false
		if freshID == id || d.readerScrolled(s, id) || d.repeatsSavedRestore(s) {
			t.Fatal("a planned but failed new arrival fell back to the prior dispatch owner")
		}
	})
	t.Run("valid ordinary scrolls keep saving on the installed page", func(t *testing.T) {
		s := androidSavedRestoreTestState()
		var d androidReadingDispatch
		intent, id := d.prepare(s)
		d.dispatched(intent, id)
		if !d.readerScrolled(s, id) || s.restore != nil {
			t.Fatal("a valid reader scroll did not retire its own restore")
		}
		if !d.readerScrolled(s, id) {
			t.Fatal("later genuine scrolls on the same page require no intervening render")
		}
	})
}

func TestAndroidRestoreIntentAndCallbackWiringKeepsQueueOwnership(t *testing.T) {
	ordered := func(source string, parts ...string) bool {
		for _, part := range parts {
			at := strings.Index(source, part)
			if at < 0 {
				return false
			}
			source = source[at+len(part):]
		}
		return true
	}
	source := readNativeSource(t, "reading_android.go")
	start := strings.Index(source, "func pushChapterHTML(")
	if start < 0 {
		t.Fatal("Android chapter push missing")
	}
	end := strings.Index(source[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("Android chapter push boundary is missing")
	}
	push := source[start : start+end]
	if !ordered(push,
		"pushWanted := androidReadingPushWanted(state, fp == lastPushedChapterFP, explicitArrival)",
		"repeatedRestore := (sameChapterPush || androidRecreatedReadingCarry) && !explicitArrival && readingAndroidDispatch.repeatsSavedRestore(state)",
		"if !pushWanted {", "intent, intentID := readingAndroidDispatch.prepare(state)", "armPendingRestore(state)") {
		t.Fatal("restore classification/planning must preserve no-op and cold-placement order")
	}
	final := strings.Index(push, "issued := false")
	if final < 0 || !ordered(push[final:],
		"runBta(func(env uintptr) {", "C.btaSetReadingIntent(C.uintptr_t(env), C.longlong(intentID))",
		"C.btaSetStyle(C.uintptr_t(env),", "issued = C.btaSetHtml(", "if issued {", "readingAndroidDispatch.dispatched(intent, intentID)") {
		t.Fatal("the intent and installed page must be dispatched under one native context before recording ownership")
	}
	if strings.Count(push, "C.btaSetReadingIntent(") != 1 ||
		!strings.Contains(push, "if androidReadingCarryWanted(sameChapterPush || androidRecreatedReadingCarry, explicitArrival, state.restore, repeatedRestore) {") ||
		!strings.Contains(push, "C.int(restoreVerse), C.float(restoreWithin), carry, carry)") ||
		strings.Contains(push, "pageChanged") {
		t.Fatal("same-grammar imports need the same position/selection carry request and one final intent dispatch")
	}
	export := readNativeSource(t, "reading_android_export.go")
	callback := javaBlockAfter(t, strings.ReplaceAll(export, "func btaScrolled", "CALLBACK btaScrolled"), "CALLBACK btaScrolled")
	if !ordered(callback, "fyne.Do(func() {", "state := activeAIState", "readingAndroidDispatch.readerScrolled(state, id)", "clearAndroidDataSwapArrival(state)") {
		t.Fatal("the native callback must compare current state only on the Fyne goroutine")
	}
}

func TestAndroidRecreationRetainsTheExistingRestoreRequest(t *testing.T) {
	s := androidSavedRestoreTestState()
	r := s.restore
	if !androidExistingRestoreCanRecover(s) || s.restore != r || r.Reflow {
		t.Fatal("pending saved request was replaced on recreation")
	}
	r.Reflow = true
	if !androidExistingRestoreCanRecover(s) {
		t.Fatal("existing transient fallback was discarded on recreation")
	}
	s.forceReposition = true
	if androidExistingRestoreCanRecover(s) {
		t.Fatal("recovery took over an explicit arrival")
	}
	s.forceReposition = false
	s.CurrentChapter++
	if androidExistingRestoreCanRecover(s) {
		t.Fatal("recovery kept another chapter's restore")
	}
	source := readNativeSource(t, "reading_android.go")
	keep := strings.Index(source, "if androidExistingRestoreCanRecover(state) {")
	capture := strings.Index(source, "} else if v, d, f, ok := captureReadingAnchor(); ok {")
	if keep < 0 || capture <= keep {
		t.Fatal("recreation captures a possibly unmeasured anchor before preserving its pending target")
	}
}

// This tests Go's request eligibility. Native text identity and selected-range
// validation execute separately in the Java reflow mapper harness.
func TestAndroidReadingCarryWithinEitherPage(t *testing.T) {
	const reference = 21.0
	cutoff := reporterMeasureEm*reference + 2*readingPageSideMin
	for _, tc := range []struct {
		name          string
		before, after float64
		kind          readingPageKind
	}{
		{"phone to phone", cutoff - 100, cutoff - 20, readingPagePhone},
		{"book to book", cutoff + 20, cutoff + 200, readingPageBook},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if readingPageFor(tc.before, reference).Kind != tc.kind || readingPageFor(tc.after, reference).Kind != tc.kind {
				t.Fatal("control: both widths must retain the same page grammar")
			}
			if !androidReadingCarryWanted(true, false, nil, false) {
				t.Fatal("a live same-chapter import lost carry because its page grammar stayed the same")
			}
		})
	}
}

func TestAndroidReadingCarryRetainsPlacementPriority(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		same, arrival, repeated bool
		restore                 *restoreAnchor
		want                    bool
	}{
		{"live same chapter", true, false, false, nil, true},
		{"transient layout restore", true, false, false, &restoreAnchor{Reflow: true}, true},
		{"first saved restore", true, false, false, &restoreAnchor{Verse: 4}, false},
		{"same-coordinate history top", true, false, false, &restoreAnchor{}, false},
		{"successfully repeated saved restore", true, false, true, &restoreAnchor{Verse: 4}, true},
		{"explicit arrival", true, true, false, nil, false},
		{"explicit arrival with restore", true, true, false, &restoreAnchor{Reflow: true}, false},
		{"different chapter", false, false, false, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := androidReadingCarryWanted(tc.same, tc.arrival, tc.restore, tc.repeated); got != tc.want {
				t.Fatalf("carry request = %v, want %v", got, tc.want)
			}
		})
	}
}
