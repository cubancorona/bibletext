package bibletext

// Regression tests for search-update races: (1) Find — the reader
// edits the query and resubmits, but a slow completion for the ABANDONED query
// lands late and clobbers the new search (progress bar flashes, old results
// reappear); (2) keyword search — a debounced run whose timer already fired
// lands AFTER an Enter-submitted search and overwrites its results. Both are
// supersession bugs; both mechanisms are pinned here.

import (
	"sync"
	"testing"
	"time"
)

func TestAISearchSessionSupersedes(t *testing.T) {
	var s aiSearchSession

	g1 := s.Start()
	if !s.Current(g1) {
		t.Fatal("a just-started submission must be current")
	}

	// The reader edits the query and resubmits while g1 is still in flight.
	g2 := s.Start()
	if s.Current(g1) {
		t.Error("the abandoned submission must no longer be current (it would clobber the new search)")
	}
	if !s.Current(g2) {
		t.Error("the newest submission must be current")
	}

	// Clearing the field / toggling the mode abandons the in-flight ask outright.
	s.Invalidate()
	if s.Current(g2) {
		t.Error("Invalidate must abandon the in-flight submission")
	}
}

// marshalRecorder stands in for fyne.Do: it either runs closures inline or holds
// them for the test to release, and serializes all cross-goroutine state — the
// role the UI goroutine plays in production.
type marshalRecorder struct {
	mu     sync.Mutex
	hold   bool
	queued []func()
	fired  []string
}

func (m *marshalRecorder) marshal(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hold {
		m.queued = append(m.queued, fn)
		return
	}
	fn() // inline, under the same lock that guards fired
}

func (m *marshalRecorder) record(s string) {
	// Called from within marshal (lock already held on the hold path is NOT the
	// case — released queue runs call this from the test goroutine). Guard anyway.
	m.fired = append(m.fired, s)
}

func (m *marshalRecorder) firedNow() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.fired...)
}

func (m *marshalRecorder) releaseQueued() int {
	m.mu.Lock()
	q := m.queued
	m.queued = nil
	m.mu.Unlock()
	for _, fn := range q {
		fn()
	}
	return len(q)
}

func TestTrailingDebouncerStaleFire(t *testing.T) {
	// Ordinary debounce: rapid keystrokes collapse to the last value.
	rec := &marshalRecorder{}
	onChanged, _ := newTrailingDebouncer(5*time.Millisecond, rec.marshal, rec.record)
	onChanged("g")
	onChanged("gr")
	onChanged("grace")
	time.Sleep(40 * time.Millisecond)
	if got := rec.firedNow(); len(got) != 1 || got[0] != "grace" {
		t.Fatalf("debounce should fire once with the final text; got %v", got)
	}

	// THE BUG: the timer fires (its goroutine queues the marshalled run), and
	// only THEN the reader hits Enter — stop() runs after the timer is already
	// past Stop()'s reach. The generation bump must still drop the queued run.
	rec2 := &marshalRecorder{hold: true}
	onChanged2, stop2 := newTrailingDebouncer(1*time.Millisecond, rec2.marshal, rec2.record)
	onChanged2("grac")
	time.Sleep(30 * time.Millisecond) // timer fired; run is queued in the marshal
	stop2()                           // Enter: submit searches immediately, debounce must die
	if n := rec2.releaseQueued(); n != 1 {
		t.Fatalf("expected exactly one queued run, got %d", n)
	}
	if got := rec2.firedNow(); len(got) != 0 {
		t.Errorf("a stale debounced run landed after stop() and fired: %v", got)
	}

	// A queued run superseded by a NEWER keystroke must drop too.
	rec3 := &marshalRecorder{hold: true}
	onChanged3, _ := newTrailingDebouncer(1*time.Millisecond, rec3.marshal, rec3.record)
	onChanged3("gra")
	time.Sleep(30 * time.Millisecond) // fires; queued
	onChanged3("grace")               // newer keystroke supersedes the queued one
	rec3.releaseQueued()              // stale run lands — must drop itself
	if got := rec3.firedNow(); len(got) != 0 {
		t.Errorf("a superseded debounced run fired: %v", got)
	}
}

// TestAISearchSessionSurvivesRebuild pins the rebuild variant of the same race:
// the supersession guard lives on AppState (state.askSession), so the session a
// pre-rebuild completion closure captured and the session a post-rebuild
// submission bumps are the SAME object. When the guard was a per-build local,
// each rebuildWindow minted a fresh zeroed session and a stale pre-rebuild
// response passed its own (never-bumped) check and repainted the new query's
// results.
func TestAISearchSessionSurvivesRebuild(t *testing.T) {
	state := &AppState{}

	// Build 1's sidebar captures the session the way a builder does.
	build1 := captureSessionLikeABuilder(state)
	genA := build1.Start()

	// The window rebuilds mid-flight (rotation / theme change / version switch)
	// and a NEW builder captures again. Taking &state.askSession twice inline —
	// which this test used to do — is the same address by construction, so the
	// assertions below were a tautology about the type rather than a check that
	// the session is state-held.
	build2 := captureSessionLikeABuilder(state)
	genB := build2.Start()

	// A's slow completion closes over build1 — it must see itself superseded.
	if build1.Current(genA) {
		t.Error("a pre-rebuild submission stayed current after a post-rebuild submission — stale results would repaint the new query")
	}
	if !build2.Current(genB) {
		t.Error("the post-rebuild submission must be current")
	}
}

// captureSessionLikeABuilder mimics what a sidebar builder does: reach the
// session through the AppState it was handed. If the session ever moves back to
// a per-build local, this returns distinct objects and the test above fails.
func captureSessionLikeABuilder(state *AppState) *aiSearchSession {
	return &state.askSession
}
