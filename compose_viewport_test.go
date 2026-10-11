package bibletext

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestMobileComposerKeepsEditorAndActionsAboveLandscapeKeyboard(t *testing.T) {
	for _, ask := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			name := "Share/Fyne"
			if ask {
				name = "Ask/Fyne"
			}
			if native {
				name += "/native"
			}
			t.Run(name, func(t *testing.T) {
				st := psalm23NoteState()
				win := noteSheetWindow(t, st, true, 874, 402)
				held := holdNoteSheetTimers(t)
				softKeyboard(t, false)
				prevArea, prevFoot := canvasArea, keyboardFreeFoot
				keyboardFreeFoot.known = false
				up := false
				canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
					h := float32(381)
					if up {
						h = 178
					}
					return fyne.NewPos(62, 0), fyne.NewSize(c.Size().Width-124, h)
				}
				t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })
				var frames *[]noteFrame
				typed := ""
				if native {
					frames = nativeNoteField(t, win)
					read, show, hide := noteEntryTyped, noteEntryShow, noteEntryHide
					noteEntryTyped = func() string { return typed }
					noteEntryShow = func(text, _ string, _ palette) { typed = text }
					noteEntryHide = func() { typed = "" }
					t.Cleanup(func() { noteEntryTyped, noteEntryShow, noteEntryHide = read, show, hide })
				}
				if ask {
					promptAskQuestion(st, "A synthetic selected passage.", verseTwo)
				} else {
					promptShareNote(st, drawnSelection(t, st, 1, 2), verseTwo)
				}
				p := topPopup(t, win)
				_, originalFoot := sheetBox(t, p)
				const draft = "A synthetic draft."
				if native {
					typed = draft
					if noteEntryOnChanged != nil {
						noteEntryOnChanged()
					}
				} else {
					composerField(p).SetText(draft)
				}
				up = true
				noteSoftKeyboard(224)
				runHeld(held)
				check := func() {
					t.Helper()
					primary := "Share"
					if ask {
						primary = "Ask"
					}
					for _, label := range []string{"Cancel", primary, "Show passage"} {
						b := findTreeButton(p, label)
						if label == "Show passage" && b == nil {
							b = findTreeButton(p, "Hide passage")
						}
						if b == nil {
							t.Fatalf("missing %s", label)
						}
						pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b)
						if pos.Y < 0 || pos.Y+b.Size().Height > 178.5 {
							t.Errorf("%s is outside keyboard-free area: %v %v", label, pos, b.Size())
						}
					}
					var pos fyne.Position
					var size fyne.Size
					if native {
						if len(*frames) == 0 {
							t.Fatal("missing native editor frame")
						}
						last := (*frames)[len(*frames)-1]
						pos, size = last.pos, last.sz
					} else {
						e := composerField(p)
						pos, size = fyne.CurrentApp().Driver().AbsolutePositionForObject(e), e.Size()
					}
					if size.Height < 30 || pos.Y+size.Height > 178.5 || pos.X < 62 || pos.X+size.Width > 812.5 {
						t.Errorf("editor outside usable area: %v %v", pos, size)
					}
					_, foot := sheetBox(t, p)
					if math.Abs(float64(foot-originalFoot)) > .5 {
						t.Errorf("keyboard shortened sheet background: %v → %v", originalFoot, foot)
					}
				}
				check()
				// A driver may run only the parent layout during a minimum-size
				// pass, without refreshing the unchanged child containers afterward.
				walkTree(p, func(o fyne.CanvasObject) {
					if c, ok := o.(*fyne.Container); ok {
						if v, ok := c.Layout.(*composeViewport); ok {
							v.Layout(c.Objects, c.Size())
						}
					}
				})
				check()
				test.Tap(findTreeButton(p, "Show passage"))
				check()
				if !native && win.Canvas().Focused() != composerField(p) {
					t.Error("passage disclosure lost the draft's keyboard focus")
				}
				test.Tap(findTreeButton(p, "Hide passage"))
				check()
				up = false
				noteSoftKeyboard(0)
				runHeld(held)
				if findTreeButton(p, "Show passage") != nil {
					t.Error("full-height form did not return after keyboard dismissal")
				}
				if native {
					last := (*frames)[len(*frames)-1]
					if last.sz.Height != 112 || typed != draft {
						t.Errorf("native editor/draft did not restore: %+v, %q", last, typed)
					}
				} else if composerField(p).Text != draft {
					t.Error("Fyne draft lost")
				}
				test.Tap(findTreeButton(p, "Cancel"))
				if p.Visible() || (native && typed != "") {
					t.Error("Cancel left composer/editor up")
				}
			})
		}
	}
}
