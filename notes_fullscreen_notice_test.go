package bibletext

// The presented reading mode is chrome-free by design, and for a long time that
// included the one piece of chrome a reader cannot do without: the notice that a
// shared link's payload could not be read. Both native panes returned their
// full-screen tree BEFORE the banner call, and the presented mode is the default
// for a phone held sideways — so on the two platforms where links actually
// arrive, a reader who tapped a bad link in landscape was handed the passage and
// told nothing at all.
//
// These tests hold the seam that fixed it. They exercise fullScreenTop, which is
// what both panes now put above the text, rather than either pane's own tree:
// the panes are cgo behind build tags and no host test can reach them, but this
// is the whole of what they were dropping.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestThePresentedModeStillTellsAReaderTheNoteCouldNotBeRead(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	defer setNotesEnabled(true)

	st := noticeState()
	st.NoteNotice = noteDamagedMessage

	top := fullScreenTop(st)
	if top == nil {
		t.Fatal("the presented mode offered nothing above the text at all")
	}
	found := false
	for _, s := range treeTexts(top) {
		if strings.Contains(s, noteDamagedMessage) {
			found = true
		}
	}
	if !found {
		t.Errorf("the notice is missing from the presented mode; the reader is told nothing.\n"+
			"got: %v", treeTexts(top))
	}

	// The control. If the row alone carried this text the check above would pass
	// for a pane that still drops the banner, so prove the row does not.
	for _, s := range treeTexts(fullScreenExitRow(st)) {
		if strings.Contains(s, noteDamagedMessage) {
			t.Fatal("the exit row already carries the notice, so this test cannot tell " +
				"a pane that shows the banner from one that drops it")
		}
	}
}

// And it must cost an ordinary read nothing: no notice, no extra row.
func TestThePresentedModeAddsNothingWhenThereIsNothingToSay(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	defer setNotesEnabled(true)

	st := noticeState()
	st.NoteNotice = ""

	if buildNoteBanner(st) != nil {
		t.Skip("this state raises a banner of its own; the check below would prove nothing")
	}
	plain := len(treeTexts(fullScreenExitRow(st)))
	got := len(treeTexts(fullScreenTop(st)))
	if got != plain {
		t.Errorf("the presented mode grew from %d text nodes to %d with nothing to report — "+
			"the reading mode is meant to be bare", plain, got)
	}
}
