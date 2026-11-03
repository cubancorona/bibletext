package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func composerViewportOf(t *testing.T, popup fyne.CanvasObject) *composeViewport {
	t.Helper()
	var result *composeViewport
	walkTree(popup, func(o fyne.CanvasObject) {
		if c, ok := o.(*fyne.Container); ok {
			if v, ok := c.Layout.(*composeViewport); ok {
				result = v
			}
		}
	})
	if result == nil {
		t.Fatal("missing compose viewport")
	}
	return result
}

// The API 35 landscape measurement leaves106.667 units above the IME after a
// 24.667unit safe inset. Releasing that IME must make actual passage rows
// readable, rather than merely setting the expanded flag above a zero-height
// scroll. This models native insets; device acceptance is a separate check.
func TestAndroidShowPassageReleasesKeyboardRoomForNoteAndAsk(t *testing.T) {
	for _, action := range []string{"note", "ask"} {
		t.Run(action, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, 800, 360)
			held := holdNoteSheetTimers(t)
			softKeyboard(t, false)
			prevArea, prevFoot, prevKeyboard := canvasArea, keyboardFreeFoot, noteEntryKeyboard
			keyboardFreeFoot.known = false
			const top, keyboardRoom = float32(24.667), float32(106.667)
			raised := false
			canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
				height := c.Size().Height - top - 20
				if raised {
					height = keyboardRoom
				}
				return fyne.NewPos(0, top), fyne.NewSize(c.Size().Width, height)
			}
			t.Cleanup(func() { canvasArea, keyboardFreeFoot, noteEntryKeyboard = prevArea, prevFoot, prevKeyboard })
			typed, shows, hides, _ := nativeAskEditor(t, win)
			openComposer(st, action)
			popup := topPopup(t, win)
			owner := noteEntryOwner
			const draft = "A synthetic draft.\nA second line."
			*typed = draft
			if noteEntryOnChanged != nil {
				noteEntryOnChanged()
			}
			full := composerActions(popup)
			focusCalls := []bool{}
			noteEntryKeyboard = func(expected uint64, focus bool) {
				if expected != owner {
					t.Errorf("keyboard request names owner %d, want %d", expected, owner)
				}
				focusCalls = append(focusCalls, focus)
				raised = focus
				if raised {
					noteSoftKeyboard(360 - top - keyboardRoom)
				} else {
					noteSoftKeyboard(0)
				}
			}
			raised = true
			noteSoftKeyboard(360 - top - keyboardRoom)
			runHeld(held)
			v := composerViewportOf(t, popup)
			if !v.dense {
				t.Fatal("control: measured keyboard geometry did not produce a dense composer")
			}
			test.Tap(findTreeButton(popup, "Show passage"))
			runHeld(held)
			if raised || len(focusCalls) != 1 || focusCalls[0] {
				t.Fatalf("Show passage retained the keyboard: raised %v, focus requests %v", raised, focusCalls)
			}
			renderer := test.WidgetRenderer(v.excerpt).(*noteExcerptRenderer)
			if renderer.rowH <= 0 || len(renderer.drawn()) == 0 {
				t.Fatal("control: the expanded quotation has no measured text rows")
			}
			if !v.expanded || !v.scroll.Visible() || v.scroll.Size().Height < renderer.rowH {
				t.Errorf("expanded passage is not readable: expanded %v, visible %v, height %v, row %v", v.expanded, v.scroll.Visible(), v.scroll.Size().Height, renderer.rowH)
			}
			slot := composerSlot(popup)
			if slot == nil || !shownTree(popup)[slot] || slot.Size().Height <= 0 {
				t.Fatal("keyboard dismissal also hid the native editor")
			}
			for label, before := range full {
				if after := composerActions(popup)[label]; after.sz != before.sz {
					t.Errorf("%s changed visual size: %v→%v", label, before.sz, after.sz)
				}
			}
			if *typed != draft || *shows != 1 || *hides != 0 {
				t.Errorf("Show passage replaced/cleared the native field: draft %q, shows %d, hides %d", *typed, *shows, *hides)
			}
			test.Tap(findTreeButton(popup, "Hide passage"))
			runHeld(held)
			if !raised || len(focusCalls) != 2 || !focusCalls[1] || *typed != draft {
				t.Errorf("returning to editing did not retain the draft and keyboard: raised %v, requests %v, draft %q", raised, focusCalls, *typed)
			}
			test.Tap(findTreeButton(popup, "Cancel"))
			runHeld(held)
		})
	}
}

func TestNativeComposerDisclosureCannotFocusAReplacementOrClosedEditor(t *testing.T) {
	for _, action := range []string{"note", "ask"} {
		t.Run(action, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, 800, 360)
			held := holdNoteSheetTimers(t)
			_, _, _, frames := nativeAskEditor(t, win)
			oldKeyboard := noteEntryKeyboard
			t.Cleanup(func() { noteEntryKeyboard = oldKeyboard })
			calls := 0
			noteEntryKeyboard = func(uint64, bool) { calls++ }
			openComposer(st, action)
			first := topPopup(t, win)
			stale := composerViewportOf(t, first).disclosure.OnTapped
			first.Hide()
			openComposer(st, action)
			second := topPopup(t, win)
			current := composerViewportOf(t, second)
			beforeFrames := len(*frames)
			stale()
			if calls != 0 || len(*frames) != beforeFrames || current.expanded {
				t.Error("a superseded disclosure changed the replacement editor")
			}
			closed := current.disclosure.OnTapped
			test.Tap(findTreeButton(second, "Cancel"))
			runHeld(held)
			beforeFrames = len(*frames)
			closed()
			if calls != 0 || len(*frames) != beforeFrames || current.expanded {
				t.Error("a closed disclosure focused or laid out its former editor")
			}
		})
	}
}

func TestNativeComposerKeyboardPolicyFollowsTallShortTallRoom(t *testing.T) {
	for _, action := range []string{"note", "ask"} {
		t.Run(action, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, 800, 900)
			held := holdNoteSheetTimers(t)
			softKeyboard(t, false)
			previousArea, previousFoot, previousKeyboard := canvasArea, keyboardFreeFoot, noteEntryKeyboard
			keyboardFreeFoot.known = false
			const top, keyboardOverlap = float32(24.667), float32(228.666)
			canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
				return fyne.NewPos(0, top), fyne.NewSize(c.Size().Width, c.Size().Height-top-keyboardOverlap)
			}
			t.Cleanup(func() { canvasArea, keyboardFreeFoot, noteEntryKeyboard = previousArea, previousFoot, previousKeyboard })
			typed, shows, hides, _ := nativeAskEditor(t, win)
			openComposer(st, action)
			popup := topPopup(t, win)
			v := composerViewportOf(t, popup)
			const draft = "A retained synthetic draft."
			*typed = draft
			focusCalls := []bool{}
			noteEntryKeyboard = func(owner uint64, focus bool) {
				if owner != noteEntryOwner {
					t.Fatal("keyboard policy acted for a different editor")
				}
				focusCalls = append(focusCalls, focus)
			}
			win.Resize(fyne.NewSize(800, 360))
			popup.Refresh()
			runHeld(held)
			if !v.dense {
				t.Fatal("control: the short intermediate layout is not dense")
			}
			win.Resize(fyne.NewSize(800, 900))
			popup.Refresh()
			runHeld(held)
			if v.dense {
				t.Fatal("the tall form retained the previous short-layout keyboard policy")
			}
			// Retained callbacks still use the current measured room even when
			// the full form no longer needs its compact disclosure row.
			v.disclosure.OnTapped()
			runHeld(held)
			if len(focusCalls) != 1 || !focusCalls[0] {
				t.Errorf("roomy expansion unnecessarily dismissed the keyboard: %v", focusCalls)
			}
			if *typed != draft || *shows != 1 || *hides != 0 {
				t.Error("reflow changed the native editor or draft")
			}
			test.Tap(findTreeButton(popup, "Cancel"))
			runHeld(held)
		})
	}
}
