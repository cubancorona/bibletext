//go:build bibletextdev

package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// The dev build wires the listener in so the handoff can be rehearsed on a
// Mac under BIBLETEXT_MIMIC: a second launch forwards to the first. Every
// directory the record could land in is redirected, so a developer's live
// instance (or the CI runner's) is never touched.
func TestSecondLaunchForwardsToTheFirst(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_RUNTIME_DIR", tmp)
	t.Setenv("XDG_CACHE_HOME", tmp)
	t.Setenv("LocalAppData", tmp)
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
	}
	stop, forwarded := claimSingleInstance(st, "")
	defer stop()
	if forwarded || !singleInstanceListening() {
		t.Fatal("the first launch did not become the primary")
	}
	// The test driver runs fyne.Do inline on the listener goroutine, and the
	// primary answers "ok" only after that delivery returned, so the state is
	// settled once the forward comes back (this suite runs without -race).
	if !forwardToRunningInstance("https://bibletext.co.uk/web/john/3/#v16") {
		t.Fatal("the second launch did not reach the first")
	}
	if st.CurrentBook != "John" || st.hlLo() != 16 {
		t.Fatalf("the forwarded link was not opened: %s verse %d", st.CurrentBook, st.hlLo())
	}
	if _, again := claimSingleInstance(NewLoadingState(), ""); !again {
		t.Fatal("a third launch was not told to forward while the primary runs")
	}
}
