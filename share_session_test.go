//go:build !ios && !android

package bibletext

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// sessionHarness holds the share session's seams: what it posts to the UI
// goroutine waits in a queue the test drains, and its timers fire when the
// test says. Windows posts from its own threads too, so the queue is locked.
type sessionHarness struct {
	t         *testing.T
	mu        sync.Mutex
	queue     []func() // under mu
	timers    []*sessionTimer
	fallbacks int
	log       []string
}

type sessionTimer struct {
	f       func()
	stopped bool
}

func newSessionHarness(t *testing.T) *sessionHarness {
	h := &sessionHarness{t: t}
	prevOnUI, prevAfter, prevCurrent := shareSessionOnUI, shareSessionAfter, currentShareSession
	shareSessionOnUI = func(f func()) {
		h.mu.Lock()
		h.queue = append(h.queue, f)
		h.mu.Unlock()
	}
	shareSessionAfter = func(d time.Duration, f func()) func() bool {
		if d != shareSessionWait {
			t.Errorf("a timer of %v, want %v", d, shareSessionWait)
		}
		tm := &sessionTimer{f: f}
		h.timers = append(h.timers, tm)
		return func() bool { was := !tm.stopped; tm.stopped = true; return was }
	}
	currentShareSession = nil
	t.Cleanup(func() {
		shareSessionOnUI, shareSessionAfter, currentShareSession = prevOnUI, prevAfter, prevCurrent
	})
	return h
}

// start begins a share whose fallback and releases the harness counts.
func (h *sessionHarness) start(name string) *shareSession {
	s := startShareSession(func() { h.fallbacks++; h.log = append(h.log, name+" fallback") })
	s.hold(func() { h.log = append(h.log, name+" release 1") })
	s.hold(func() { h.log = append(h.log, name+" release 2") })
	return s
}

// drain runs what has been posted, and what that posts, until nothing is left.
func (h *sessionHarness) drain() {
	for i := 0; ; i++ {
		if i > 100 {
			h.t.Fatal("the UI queue does not empty")
		}
		h.mu.Lock()
		if len(h.queue) == 0 {
			h.mu.Unlock()
			return
		}
		f := h.queue[0]
		h.queue = h.queue[1:]
		h.mu.Unlock()
		f()
	}
}

// fire runs the latest timer that has not been stopped, as its time arriving.
func (h *sessionHarness) fire() {
	h.t.Helper()
	for i := len(h.timers) - 1; i >= 0; i-- {
		if tm := h.timers[i]; !tm.stopped {
			tm.stopped = true
			tm.f()
			return
		}
	}
	h.t.Fatal("control: no timer is armed")
}

func (h *sessionHarness) wantLog(want ...string) {
	h.t.Helper()
	if !slices.Equal(h.log, want) {
		h.t.Errorf("the share did %q, want %q", h.log, want)
	}
}

// A STEP THAT FAILS ENDS THE SHARE IN ITS FALLBACK, ONCE: what it held is
// released newest first, and then the in-app sheet opens; the share is no
// longer current, and a second end does nothing. Mutations: the releases
// run oldest first, the fallback skipped for an error, finish not guarded.
func TestAShareThatFailsEndsInItsFallbackOnce(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	s.finish(errors.New("GetForWindow: 0x80004005"))
	s.finish(errors.New("a second failure"))
	h.drain()
	h.wantLog("share release 2", "share release 1", "share fallback")
	if currentShareSession != nil {
		t.Error("an ended share is still current")
	}
	if s.result == nil || !strings.Contains(s.result.Error(), "GetForWindow") {
		t.Errorf("the share ended with %v, want the first failure", s.result)
	}
}

// A SHARE THE SHEET TAKES ENDS WITHOUT A FALLBACK, even when its watchdog
// fires as the sheet asks, so that the watchdog's end is queued ahead of the
// delivery's. Mutations: the delivered check dropped from finish (the
// watchdog falls back), delivered never set.
func TestADeliveredShareEndsWithoutAFallback(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	s.await("the sheet asking")
	filled := 0
	h.fire() // the watchdog's end, queued ahead of the delivery's
	s.deliver(func() error { filled++; return nil })
	h.drain()
	if filled != 1 {
		t.Errorf("the package was filled %d times, want once", filled)
	}
	h.wantLog("share release 2", "share release 1")
	if s.result != nil {
		t.Errorf("the share ended with %v, want delivered", s.result)
	}
	s.deliver(func() error { filled++; return nil })
	if filled != 1 {
		t.Error("a request after the share ended filled the package again")
	}
}

// A SHEET THAT NEVER ASKS ENDS IN THE FALLBACK when the watchdog fires, with
// the step it was waiting on named. Mutations: the watchdog not armed, its
// finish given no error.
func TestASheetThatNeverAsksEndsInTheFallback(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	s.await("the sheet asking")
	h.drain()
	h.wantLog()
	h.fire()
	h.drain()
	h.wantLog("share release 2", "share release 1", "share fallback")
	var noAnswer shareNoAnswer
	if !errors.As(s.result, &noAnswer) || string(noAnswer) != "the sheet asking" {
		t.Errorf("the share ended with %v, want no answer from the sheet asking", s.result)
	}
}

// A WATCHDOG ARMED FOR AN EARLIER STEP DOES NOTHING once the share has moved
// on: the picture resolved, and the sheet is now the step waited on.
// Mutation: the generation check dropped (the first timer ends the share).
func TestAnEarlierStepsWatchdogDoesNothing(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	s.await("the picture resolving")
	first := h.timers[0]
	s.await("the sheet asking")
	if !first.stopped {
		t.Error("moving on did not stop the earlier step's timer")
	}
	first.f() // a timer that fired as it was being stopped
	h.drain()
	h.wantLog()
	h.fire()
	h.drain()
	h.wantLog("share release 2", "share release 1", "share fallback")
}

// A PACKAGE THAT WILL NOT FILL ENDS IN THE FALLBACK, and so does a fill that
// panics, which leaves the share's lock free. Mutations: a fill error taken
// for a delivery, the panic not recovered.
func TestAPackageThatWillNotFillEndsInTheFallback(t *testing.T) {
	for _, c := range []struct {
		name string
		fill func() error
	}{
		{"an error", func() error { return errors.New("put_Title: 0x80070057") }},
		{"a panic", func() error { panic("a fill that panics") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newSessionHarness(t)
			s := h.start("share")
			s.deliver(c.fill)
			h.drain()
			h.wantLog("share release 2", "share release 1", "share fallback")
			if !s.mu.TryLock() {
				t.Fatal("the share's lock was left held")
			}
			s.mu.Unlock()
		})
	}
}

// A NEWER SHARE REPLACES ONE STILL IN FLIGHT, which ends quietly: what it
// held is released, and only the newer share can fall back. A request for
// the old share after that fills nothing. Mutations: the replaced share
// given its fallback, not ended at all.
func TestANewerShareReplacesOneInFlight(t *testing.T) {
	h := newSessionHarness(t)
	old := h.start("old")
	old.await("the sheet asking")
	newer := h.start("newer")
	if currentShareSession != newer {
		t.Error("the newer share is not current")
	}
	h.drain()
	h.wantLog("old release 2", "old release 1")
	old.deliver(func() error { t.Error("the replaced share filled a package"); return nil })
	newer.finish(errors.New("ShowShareUIForWindow: 0x80004005"))
	h.drain()
	h.wantLog("old release 2", "old release 1", "newer release 2", "newer release 1", "newer fallback")
}

// A SHARE ENDED WHILE ITS PACKAGE IS FILLING KEEPS WHAT IT HOLDS until the
// fill is done: a fill that dispatches the reader's click on a new share, on
// the same thread, ends the share it is filling only after it returns.
// Mutation: finish taking the lock unconditionally (the test deadlocks and
// times out) or releasing without it (the fill sees its parts gone).
func TestAShareEndedWhileFillingReleasesAfterTheFill(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	s.deliver(func() error {
		s.finish(errShareSuperseded) // beneath the fill, as a pumped click would
		if len(h.log) != 0 {
			t.Errorf("the share released %q while its package was filling", h.log)
		}
		return nil
	})
	h.drain()
	h.wantLog("share release 2", "share release 1")
	if !errors.Is(s.result, errShareSuperseded) {
		t.Errorf("the share ended with %v, want superseded", s.result)
	}
}

// WHAT IS POSTED FOR A SHARE THAT HAS ENDED DOES NOT RUN: the picture's file
// resolving after the reader has moved on. Mutation: post without the check.
func TestWhatIsPostedForAnEndedShareDoesNotRun(t *testing.T) {
	h := newSessionHarness(t)
	s := h.start("share")
	ran := false
	s.post(func() { ran = true })
	s.finish(errShareSuperseded)
	h.drain()
	if ran {
		t.Error("a step posted before the share ended ran after it")
	}
}
