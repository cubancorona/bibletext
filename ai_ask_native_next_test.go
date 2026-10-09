//go:build next

package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// nativeAskEditor models the singleton UIKit field without a keyboard or
// provider. Show replaces its text, Hide clears it, and reads observe edits.
func nativeAskEditor(t *testing.T, win fyne.Window) (*string, *int, *int, *[]noteFrame) {
	t.Helper()
	frames := nativeNoteField(t, win)
	typed, shows, hides := "", 0, 0
	prevRead, prevShow, prevHide := noteEntryTyped, noteEntryShow, noteEntryHide
	noteEntryTyped = func() string { return typed }
	noteEntryShow = func(initial, _ string, _ palette) { typed = initial; shows++ }
	noteEntryHide = func() { typed = ""; hides++ }
	t.Cleanup(func() { noteEntryTyped, noteEntryShow, noteEntryHide = prevRead, prevShow, prevHide })
	return &typed, &shows, &hides, frames
}

func TestAskNativeComposerSubmitsMultilineTextAndCapturedPassage(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	holdNoteSheetTimers(t)
	typed, shows, hides, frames := nativeAskEditor(t, win)
	reqs := stubCapturedStudyRequests(t)
	passage := askComposerFixture()
	promptAskQuestionForPassage(st, passage, "", verseTwo)
	p := topPopup(t, win)
	if composerField(p) != nil || *shows != 1 || len(*frames) == 0 {
		t.Fatalf("the iOS composer must use the positioned native field; shows %d, frames %d", *shows, len(*frames))
	}
	test.Tap(findTreeButton(p, "Ask"))
	select {
	case <-reqs:
		t.Fatal("an empty question started a request")
	default:
	}
	if !p.Visible() {
		t.Fatal("an empty question dismissed the composer")
	}
	question := strings.TrimSpace("How do these fixture verses connect?\n" + strings.Repeat("A further question. ", 24))
	*typed = "  " + question + "  "
	st.CurrentBook, st.CurrentChapter = "Another fixture book", 8
	test.Tap(findTreeButton(p, "Ask"))
	r := nextCapturedStudyRequest(t, reqs, "native Ask")
	if r.question != question || r.passage != passage || !r.captured || r.span != verseTwo {
		t.Errorf("the request lost its text or passage: question %q, passage %+v, captured %v, span %+v", r.question, r.passage, r.captured, r.span)
	}
	if *hides != 1 || p.Visible() {
		t.Errorf("Ask must dismiss the editor and its sheet; hides %d, visible %v", *hides, p.Visible())
	}
	finishCapturedStudyRequest(t, r, "A fixture answer.", nil)
	panel := topPopup(t, win)
	if !sheetHas(panel, passage.Ref) || !sheetHas(panel, passage.Version) {
		t.Errorf("the answer card must retain the passage and translation; texts %v", sheetTexts(panel))
	}
	test.Tap(findTreeButton(panel, "Close"))
}

func TestAskNativeComposerReopenLeavesTheNewEditorAlone(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	held := holdNoteSheetTimers(t)
	typed, shows, hides, frames := nativeAskEditor(t, win)
	passage := askComposerFixture()
	restores := 0
	st.showReadingOverlay = func() { restores++ }
	promptAskQuestionForPassage(st, passage, "", verseTwo)
	first := topPopup(t, win)
	*typed = "First fixture line.\nSecond fixture line."
	want := *typed
	oldCallbacks := append([]func(){}, (*held)...)
	reopen := takeTopSheetReopen(st)
	if reopen == nil {
		t.Fatal("the native composer did not register a reopen")
	}
	first.Hide()
	st.CurrentBook, st.CurrentChapter = "Another fixture book", 8
	reopen()
	again := topPopup(t, win)
	if *shows != 2 || *typed != want || !sheetHas(again, passage.Ref) {
		t.Fatalf("reopen lost the question or passage; shows %d, text %q, card %v", *shows, *typed, sheetTexts(again))
	}
	before := len(*frames)
	for _, f := range oldCallbacks {
		f()
	}
	if *hides != 0 || restores != 0 || len(*frames) != before || *typed != want {
		t.Errorf("old callbacks changed the new editor; hides %d, restores %d, frames %d→%d, text %q", *hides, restores, before, len(*frames), *typed)
	}
	test.Tap(findTreeButton(again, "Cancel"))
	if *hides != 1 || restores != 1 || *typed != "" {
		t.Errorf("Cancel must clean up exactly once; hides %d, restores %d, text %q", *hides, restores, *typed)
	}
}

func TestAskNativeComposerFollowsANarrowedIPadCanvas(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 834, 1024)
	held := holdNoteSheetTimers(t)
	_, _, _, frames := nativeAskEditor(t, win)
	passage := askComposerFixture()
	promptAskQuestionForPassage(st, passage, "", selSpan{})
	p := topPopup(t, win)
	win.Resize(fyne.NewSize(320, 1024))
	p.Refresh()
	for _, f := range append([]func(){}, (*held)...) {
		f()
	}
	ex, renderer := noteSheetExcerpt(t, p)
	bottom := fyne.CurrentApp().Driver().AbsolutePositionForObject(ex).Y + float32(len(renderer.drawn()))*renderer.rowH
	if len(*frames) == 0 {
		t.Fatal("the native field received no frame")
	}
	last := (*frames)[len(*frames)-1]
	if last.pos.Y < bottom || last.pos.X+last.sz.Width > 320 {
		t.Errorf("the field must follow below the wrapped excerpt inside the narrowed canvas; frame %+v, excerpt bottom %v", last, bottom)
	}
	test.Tap(findTreeButton(p, "Cancel"))
}

func TestSupersededAskLeavesTheNoteComposerEditorAlone(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	held := holdNoteSheetTimers(t)
	typed, _, hides, frames := nativeAskEditor(t, win)
	promptAskQuestionForPassage(st, askComposerFixture(), "", selSpan{})
	first := topPopup(t, win)
	oldCallbacks := append([]func(){}, (*held)...)
	first.Hide()
	promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
	note := topPopup(t, win)
	if noteEntryOnChanged == nil {
		t.Fatal("control: the note composer did not install its counter hook")
	}
	*typed = "A synthetic note draft."
	before := len(*frames)
	for _, f := range oldCallbacks {
		f()
	}
	if noteEntryOnChanged == nil || *hides != 0 || len(*frames) != before || *typed != "A synthetic note draft." {
		t.Error("the superseded Ask sheet changed the note editor or its counter hook")
	}
	test.Tap(findTreeButton(note, "Cancel"))
}
