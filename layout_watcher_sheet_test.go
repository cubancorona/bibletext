package bibletext

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestLayoutRebuildPreservesNativeQuestionDraft(t *testing.T) {
	state := psalm23NoteState()
	win := noteSheetWindow(t, state, true, 744, 1133)
	state.app = fyne.CurrentApp()
	win.SetContent(CreateMainUI(state.app, state, win))
	held := holdNoteSheetTimers(t)
	typed, shows, hides, _ := nativeAskEditor(t, win)
	passage := askComposerFixture()
	promptAskQuestionForPassage(state, passage, "", verseTwo)
	first := topPopup(t, win)
	*typed = "A synthetic first question.\nA second question with “words” 🌿."
	want := *typed
	oldCallbacks := append([]func(){}, (*held)...)

	for _, size := range []fyne.Size{{Width: 1133, Height: 744}, {Width: 744, Height: 1133}} {
		win.Resize(size)
		rebuildForLayoutChange(state)
		again := topPopup(t, win)
		if again == first || first.Visible() || len(win.Canvas().Overlays().List()) != 1 {
			t.Fatal("the layout rebuild must replace the old sheet with exactly one showing sheet")
		}
		if *typed != want {
			t.Fatalf("the layout change lost the native draft: %q", *typed)
		}
		assertAskComposerPassage(t, again, passage)
		first = again
	}
	for _, callback := range oldCallbacks {
		callback()
	}
	if *shows != 3 || *hides != 0 || *typed != want {
		t.Fatalf("a drained composer changed the new editor; shows=%d hides=%d draft=%q", *shows, *hides, *typed)
	}
	test.Tap(findTreeButton(first, "Cancel"))
	if *hides != 1 || *typed != "" {
		t.Fatal("Cancel did not remove the current native editor")
	}
}

func TestLayoutRebuildPreservesCachedStudyAnswer(t *testing.T) {
	for _, action := range []string{aiActionAsk, aiActionCompare} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			state := deferredTestState(t, app)
			state.window.Resize(fyne.NewSize(744, 1133))
			holdSheetTimers(t)
			passage := syntheticAskPassage()
			question := "A synthetic question about this passage."
			if action != aiActionAsk {
				question = ""
			}
			store := state.keys()
			provider := store.activeProvider()
			model := activeModelFor(store, provider)
			key := aiCacheKey(aiRequestScope(provider, model, action, question, passage), passage.Book, passage.Chapter, passage.Text)
			const answer = "A synthetic answer paragraph retains the original passage."
			aiCacheSet(key, answer)
			t.Cleanup(func() {
				aiCacheMu.Lock()
				delete(aiCache, key)
				aiCacheRemoveLocked(key)
				aiCacheMu.Unlock()
			})
			previousRun, previousMeasure := aiActionRun, studyRemeasure
			settled := make(chan struct{}, 8)
			aiActionRun = func(ctx context.Context, state *AppState, action, text, question string, span selSpan) (string, error) {
				return runAIAction(ctx, state, action, text, question, span)
			}
			studyRemeasure = func(fit func()) { fit(); settled <- struct{}{} }
			t.Cleanup(func() { aiActionRun, studyRemeasure = previousRun, previousMeasure })
			wait := func() {
				t.Helper()
				select {
				case <-settled:
				case <-time.After(2 * time.Second):
					t.Fatal("the cached answer did not settle")
				}
			}
			showAIPanelForPassage(state, action, passage, question, verseTwo)
			wait()
			first := topPopup(t, state.window)
			for _, size := range []fyne.Size{{Width: 1133, Height: 744}, {Width: 744, Height: 1133}} {
				state.window.Resize(size)
				rebuildForLayoutChange(state)
				wait()
				again := topPopup(t, state.window)
				if again == first || first.Visible() || len(state.window.Canvas().Overlays().List()) != 1 {
					t.Fatal("the layout rebuild lost or stacked the completed answer")
				}
				if !sheetHas(again, answer) || findTreeButton(again, "Copy").Disabled() {
					t.Fatal("the completed cached answer did not remain readable")
				}
				assertAskPanelPassage(t, again, passage)
				if action == aiActionAsk && !sheetHas(again, question) {
					t.Fatal("the answer lost its captured question")
				}
				first = again
			}
			test.Tap(findTreeButton(first, "Close"))
		})
	}
}

func TestLayoutRebuildDoesNotReopenDismissedOrUnregisteredSheet(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := deferredTestState(t, app)
	first := widget.NewModalPopUp(widget.NewLabel("Synthetic registered sheet"), state.window.Canvas())
	first.Show()
	reopened := 0
	registerSheetReopen(state, first, func() { reopened++ })
	first.Hide()
	second := widget.NewModalPopUp(widget.NewLabel("Synthetic unregistered sheet"), state.window.Canvas())
	second.Show()
	rebuildForLayoutChange(state)
	if reopened != 0 || second.Visible() || state.window.Canvas().Overlays().Top() != nil {
		t.Fatal("the responsive rebuild revived a dismissed or unregistered sheet")
	}
}
