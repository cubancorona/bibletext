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
// Ending and letting go are separate. What the share holds — the sheet's
// handler and the package's parts — is freed as it ends, except when the
// sheet has been asked to open and has not asked for the share in time: the
// fallback opens then, but the sheet, which does not know the app gave up,
// may still appear and ask, and a handler kept for it fills the package
// rather than leaving the sheet an empty one. That share lets go when the
// sheet asks, when a newer share starts, or after shareSheetKeep.
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
// the reader is given the fallback. The slowest seen on the Windows 11 VM was
// the sheet's first request after start, 549 ms; five seconds is nine times
// that and still short enough that a reader who tapped Share is not left
// looking at nothing. A slower machine's sheet that asks after it still gets
// the share (shareSheetKeep).
const shareSessionWait = 5 * time.Second

// shareSheetKeep is how long a share that fell back after the sheet was
// asked to open keeps its handler and its parts for a sheet that asks late:
// the 30 s Chromium gives the same sheet, from the same call, before it
// gives up on a share
// (chrome/browser/webshare/win/show_share_ui_for_window_operation.h,
// kMaxExecutionTime). A newer share frees them sooner.
const shareSheetKeep = 30 * time.Second

var (
	// shareSessionOnUI runs f on the UI goroutine, later. fyne.Do queues
	// while the app runs, even from the UI goroutine itself, so what it
	// posts never runs inside the call that posts it.
	shareSessionOnUI = fyne.Do

	// shareSessionAfter arms a timer and returns its stop.
	shareSessionAfter = func(d time.Duration, f func()) (stop func() bool) {
		return time.AfterFunc(d, f).Stop
	}

	// currentShareSession is the share still holding what it made: one not
	// yet ended, or one that fell back and is kept for a late sheet. A newer
	// share replaces it. UI goroutine only.
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
	finished  bool // under mu: delivered, or fallen back
	delivered bool // under mu: the sheet has the share
	late      bool // under mu: fell back while the sheet it asked to open may still ask
	released  bool // under mu: what the share holds is freed, and no request fills from it

	// UI goroutine only.
	releases   []func() // what the share holds, freed newest first as it lets go
	waiting    int      // the watchdog's generation; a timer armed for an older one does nothing
	stopWait   func() bool
	sheetAsked bool  // the sheet has been asked to open and said it would
	result     error // what the share ended with: nil when delivered
}

// startShareSession begins a share whose fallback is the verb's in-app
// sheet, ending the one before it without one and freeing what it holds.
// UI goroutine.
func startShareSession(fallback func()) *shareSession {
	if cur := currentShareSession; cur != nil {
		cur.finish(errShareSuperseded)
		cur.release()
	}
	s := &shareSession{fallback: fallback}
	currentShareSession = s
	return s
}

// hold registers release to run when the share lets go of what it holds
// (release). UI goroutine.
func (s *shareSession) hold(release func()) { s.releases = append(s.releases, release) }

// sheetOpening records that the sheet has been asked to open and has said
// it would: from here a watchdog that fires opens the fallback but keeps
// what the share holds for a late request. UI goroutine.
func (s *shareSession) sheetOpening() { s.sheetAsked = true }

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
// ended gets it again. A sheet that asks after the share fell back, while it
// is kept for one (late), gets it too, and the share then lets go; its
// fallback is already open, so nothing more opens. Any goroutine: the
// sheet's request may arrive on any thread, and inside a call the UI
// goroutine is making.
func (s *shareSession) deliver(fill func() error) {
	s.mu.Lock()
	if s.released || (s.finished && !s.late) {
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
	late := s.finished
	s.mu.Unlock()
	if late {
		if err != nil {
			fyne.LogError("The system share sheet asked for the share after the in-app sheet opened, and it could not be filled", err)
		}
		shareSessionOnUI(s.release)
		return
	}
	shareSessionOnUI(func() { s.finish(err) })
}

// finish ends the share with err: nil when the sheet has it, otherwise the
// step that failed, which is logged, and the verb's fallback opens. Only the
// first call does anything. A share already delivered ignores its
// watchdog, whose finish can be queued behind the delivery's. What the
// share holds is freed, unless the watchdog fired after the sheet was asked
// to open: then it is kept for the sheet until release. UI goroutine.
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
	s.late = s.sheetAsked && errors.As(err, &noAnswer)
	s.mu.Unlock()

	s.result = err
	if s.stopWait != nil {
		s.stopWait()
		s.stopWait = nil
	}
	if s.late {
		s.stopWait = shareSessionAfter(shareSheetKeep, func() { shareSessionOnUI(s.release) })
	} else {
		s.release()
	}
	if err == nil || errors.Is(err, errShareSuperseded) {
		return
	}
	fyne.LogError("The system share sheet did not take the share; opening the in-app sheet", err)
	if s.fallback != nil {
		s.fallback()
	}
}

// release frees what the share holds, newest first, once; no request fills
// a package from it after that. A fill still running, on another thread or
// beneath this call, finishes first. UI goroutine.
func (s *shareSession) release() {
	if !s.mu.TryLock() {
		shareSessionOnUI(s.release)
		return
	}
	if s.released {
		s.mu.Unlock()
		return
	}
	s.released = true
	s.mu.Unlock()

	if s.stopWait != nil {
		s.stopWait()
		s.stopWait = nil
	}
	for i := len(s.releases) - 1; i >= 0; i-- {
		s.releases[i]()
	}
	s.releases = nil
	if currentShareSession == s {
		currentShareSession = nil
	}
}
