//go:build !ios && !android

package bibletext

// A SHARE HANDED TO A SHEET THAT ASKS FOR IT LATER ENDS EXACTLY ONCE.
//
// The Windows Share sheet (share_windows.go) opens first and asks the app for
// the share afterwards: on the Windows 11 VM its request came through the
// window's message loop 140–550 ms after the sheet was asked to open, never
// within that call, and the picture's file resolved on a thread-pool thread.
// Until the sheet has the share, it can still fail: a step Windows refuses,
// a sheet that never asks, a package that will not fill. shareSession is the
// share across that gap, and it ends once — delivered to the sheet, or in
// the verb's fallback, the in-app confirmation sheet
// (share_sheet_desktop.go) — so a share the reader started never ends in
// silence. A reader who cancels the system sheet has made a choice, as on
// macOS, iOS and Android, and is shown nothing more.
//
// It is kept apart from the Windows calls so that the host suite can hold
// its rules on any platform (share_session_test.go).

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// shareSessionWait is how long the share waits on a step that answers later
// — the picture's file resolving, the sheet asking for the share — before
// falling back. The slowest seen was the sheet's first request after start,
// 549 ms.
const shareSessionWait = 5 * time.Second

var (
	// shareSessionOnUI runs f on the UI goroutine, later. fyne.Do queues
	// while the app runs, even from the UI goroutine itself, so what it
	// posts never runs inside the call that posts it.
	shareSessionOnUI = fyne.Do

	// shareSessionAfter arms a timer and returns its stop.
	shareSessionAfter = func(d time.Duration, f func()) (stop func() bool) {
		return time.AfterFunc(d, f).Stop
	}

	// currentShareSession is the share not yet ended; a newer share
	// replaces it. UI goroutine only.
	currentShareSession *shareSession
)

// errShareSuperseded ends a share the reader has replaced with a newer one,
// which will itself end in a sheet; it has no fallback of its own.
var errShareSuperseded = errors.New("a newer share replaced it")

// shareNoAnswer is a step that did not answer within shareSessionWait.
type shareNoAnswer string

func (s shareNoAnswer) Error() string {
	return fmt.Sprintf("no answer within %v: %s", shareSessionWait, string(s))
}

type shareSession struct {
	fallback func() // the verb's in-app sheet; UI goroutine

	mu        sync.Mutex
	finished  bool // under mu
	delivered bool // under mu: the sheet has the share

	// UI goroutine only.
	releases []func() // what the share holds, freed newest first as it ends
	waiting  int      // the watchdog's generation; a timer armed for an older one does nothing
	stopWait func() bool
	result   error // what the share ended with: nil when delivered
}

// startShareSession begins a share whose fallback is the verb's in-app
// sheet, ending the one before it without one. UI goroutine.
func startShareSession(fallback func()) *shareSession {
	if cur := currentShareSession; cur != nil {
		cur.finish(errShareSuperseded)
	}
	s := &shareSession{fallback: fallback}
	currentShareSession = s
	return s
}

// hold registers release to run when the share ends. UI goroutine.
func (s *shareSession) hold(release func()) { s.releases = append(s.releases, release) }

// ended reports whether the share has ended.
func (s *shareSession) ended() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finished
}

// post runs f on the UI goroutine, unless the share has ended by then. Any
// goroutine.
func (s *shareSession) post(f func()) {
	shareSessionOnUI(func() {
		if !s.ended() {
			f()
		}
	})
}

// await arms the watchdog for step, in place of any armed before: unless the
// share moves on to another step or ends within shareSessionWait, it falls
// back. UI goroutine.
func (s *shareSession) await(step string) {
	if s.stopWait != nil {
		s.stopWait()
	}
	s.waiting++
	gen := s.waiting
	s.stopWait = shareSessionAfter(shareSessionWait, func() {
		s.post(func() {
			if s.waiting == gen {
				s.finish(shareNoAnswer(step))
			}
		})
	})
}

// deliver hands the share to the sheet that asked for it: fill puts it into
// the sheet's package, and the share then ends, delivered or, when fill
// failed, in its fallback. A sheet that asks again before the share has
// ended gets it again. Any goroutine: the sheet's request may arrive on any
// thread, and inside a call the UI goroutine is making.
func (s *shareSession) deliver(fill func() error) {
	s.mu.Lock()
	if s.finished {
		s.mu.Unlock()
		return
	}
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("filling the share: %v", r)
			}
		}()
		return fill()
	}()
	if err == nil {
		s.delivered = true
	}
	s.mu.Unlock()
	shareSessionOnUI(func() { s.finish(err) })
}

// finish ends the share with err: nil when the sheet has it, otherwise the
// step that failed, which is logged, and the verb's fallback opens. Only the
// first call does anything. A share already delivered ignores its
// watchdog, whose finish can be queued behind the delivery's. UI goroutine.
func (s *shareSession) finish(err error) {
	if !s.mu.TryLock() {
		// A fill is running, on another thread or beneath this call, and its
		// package still needs what the share holds: end it after.
		shareSessionOnUI(func() { s.finish(err) })
		return
	}
	var noAnswer shareNoAnswer
	if s.finished || (s.delivered && errors.As(err, &noAnswer)) {
		s.mu.Unlock()
		return
	}
	s.finished = true
	s.mu.Unlock()

	s.result = err
	if s.stopWait != nil {
		s.stopWait()
	}
	for i := len(s.releases) - 1; i >= 0; i-- {
		s.releases[i]()
	}
	s.releases = nil
	if currentShareSession == s {
		currentShareSession = nil
	}
	if err == nil || errors.Is(err, errShareSuperseded) {
		return
	}
	fyne.LogError("The system share sheet did not take the share; opening the in-app sheet", err)
	if s.fallback != nil {
		s.fallback()
	}
}
