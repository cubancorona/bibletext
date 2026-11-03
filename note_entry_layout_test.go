package bibletext

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// iPhoneEditorSlot is the height of the iPhone's native Note and Ask field:
// four comfortable lines of its 18pt text. Every platform that floats a
// native editor over the composer's slot takes it.
const iPhoneEditorSlot = float32(112)

// openComposer opens the Share with note or the Ask composer on st.
func openComposer(st *AppState, action string) {
	if action == "ask" {
		promptAskQuestionForPassage(st, askComposerFixture(), "", selSpanFromNative(1, 1))
	} else {
		promptShareNote(st, "The fixture passage", selSpanFromNative(1, 1))
	}
}

// rectOf is where o stands on the canvas and its size.
func rectOf(o fyne.CanvasObject) noteFrame {
	return noteFrame{pos: fyne.CurrentApp().Driver().AbsolutePositionForObject(o), sz: o.Size()}
}

func (f noteFrame) bottom() float32 { return f.pos.Y + f.sz.Height }

// overlaps reports whether two rects share more than a hairline.
func (f noteFrame) overlaps(g noteFrame) bool {
	const hair = 0.5
	return f.pos.X+f.sz.Width > g.pos.X+hair && g.pos.X+g.sz.Width > f.pos.X+hair &&
		f.bottom() > g.pos.Y+hair && g.bottom() > f.pos.Y+hair
}

// shownTree is every object under root that is visible together with all the
// containers above it.
func shownTree(root fyne.CanvasObject) map[fyne.CanvasObject]bool {
	shown := map[fyne.CanvasObject]bool{}
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		shown[o] = true
		switch v := o.(type) {
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.Scroll:
			walk(v.Content)
		case *container.ThemeOverride:
			walk(v.Content)
		case *widget.PopUp:
			walk(v.Content)
		}
	}
	walk(root)
	return shown
}

// composerSlot is the native editor's slot on an open composer.
func composerSlot(p *widget.PopUp) *noteEntrySlot {
	var slot *noteEntrySlot
	walkTree(p, func(o fyne.CanvasObject) {
		if v, ok := o.(*noteEntrySlot); ok && slot == nil {
			slot = v
		}
	})
	return slot
}

// composerActions are the composer's Cancel and Share or Ask buttons as they
// stand, by label.
func composerActions(p *widget.PopUp) map[string]noteFrame {
	shown := shownTree(p)
	m := map[string]noteFrame{}
	walkTree(p, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && shown[b] && (b.Text == "Cancel" || b.Text == "Ask" || b.Text == "Share") {
			m[b.Text] = rectOf(b)
		}
	})
	return m
}

// composerCounter is the note composer's character counter as it stands: the
// line under the field in the full card, the short count beside Show passage
// in the compact one. ok is false when neither is showing.
func composerCounter(p *widget.PopUp) (frame noteFrame, ok bool) {
	shown := shownTree(p)
	walkTree(p, func(o fyne.CanvasObject) {
		if ok || !shown[o] {
			return
		}
		switch v := o.(type) {
		case *canvas.Text:
			if v.Text == "Optional" || strings.HasSuffix(v.Text, "characters left") {
				frame, ok = rectOf(v), true
			}
		case *widget.RichText:
			if s := segmentText(v.Segments); strings.HasPrefix(s, "Optional. ") || strings.HasSuffix(s, "characters left") {
				frame, ok = rectOf(v), true
			}
		}
	})
	return frame, ok
}

// The native editor's slot is the iPhone's on every platform that floats one:
// 112 units, against the toolkit field's one line. Beside the same card drawn
// with the toolkit's field, the slot starts where the field starts and is as
// wide; the counter, the actions and the Ask line below it stand lower by
// exactly the difference in height, and no control changes size. Phones
// upright and tablets in both orientations, where the card is laid out in
// full.
func TestNativeComposeTakesTheIPhoneEditorSlot(t *testing.T) {
	for _, canvasSize := range []struct {
		name string
		w, h float32
	}{{"phone upright", 393, 852}, {"tablet upright", 800, 1280}, {"tablet sideways", 1280, 800}} {
		for _, action := range []string{"note", "ask"} {
			t.Run(canvasSize.name+"/"+action, func(t *testing.T) {
				st := psalm23NoteState()
				win := noteSheetWindow(t, st, true, canvasSize.w, canvasSize.h)
				holdNoteSheetTimers(t)
				prevNative := noteEntryNative
				t.Cleanup(func() { noteEntryNative = prevNative })

				noteEntryNative = func() bool { return false }
				openComposer(st, action)
				fallback := topPopup(t, win)
				if findTreeButton(fallback, "Show passage") != nil && shownTree(fallback)[findTreeButton(fallback, "Show passage")] {
					t.Fatal("the card is compact on this canvas, so its full layout is not what is measured")
				}
				field := composerField(fallback)
				if field == nil {
					t.Fatal("missing the toolkit field")
				}
				fieldRect := rectOf(field)
				fallbackActions := composerActions(fallback)
				fallbackCounter, counted := composerCounter(fallback)
				findTreeButton(fallback, "Cancel").OnTapped()

				nativeNoteField(t, win)
				openComposer(st, action)
				native := topPopup(t, win)
				slot := composerSlot(native)
				if slot == nil {
					t.Fatal("missing the native editor's slot")
				}
				got := rectOf(slot)
				if got.pos != fieldRect.pos || got.sz.Width != fieldRect.sz.Width {
					t.Errorf("the slot stands at %+v, not where the toolkit field stands, %+v", got, fieldRect)
				}
				if got.sz.Height != iPhoneEditorSlot {
					t.Errorf("the native editor's slot is %v units tall, not the iPhone's %v", got.sz.Height, iPhoneEditorSlot)
				}
				shift := iPhoneEditorSlot - fieldRect.sz.Height
				nativeActions := composerActions(native)
				if len(fallbackActions) != 2 || len(nativeActions) != 2 {
					t.Fatalf("expected two actions on each card, got %v and %v", fallbackActions, nativeActions)
				}
				for label, before := range fallbackActions {
					after := nativeActions[label]
					if after.sz != before.sz {
						t.Errorf("%s is %v beside the native editor, %v beside the toolkit field: a control changed size", label, after.sz, before.sz)
					}
					if after.pos.X != before.pos.X || !nearlyEqual(after.pos.Y, before.pos.Y+shift, 0.01) {
						t.Errorf("%s stands at %v, want %v moved down by %v", label, after.pos, before.pos, shift)
					}
				}
				if counted {
					nativeCounter, ok := composerCounter(native)
					if !ok {
						t.Fatal("the native card shows no counter")
					}
					if nativeCounter.sz != fallbackCounter.sz || !nearlyEqual(nativeCounter.pos.Y, fallbackCounter.pos.Y+shift, 0.01) {
						t.Errorf("the counter stands at %+v, want %+v moved down by %v", nativeCounter, fallbackCounter, shift)
					}
				} else if action == "note" {
					t.Error("the note card shows no counter")
				}
				findTreeButton(native, "Cancel").OnTapped()
			})
		}
	}
}

// A phone held sideways with the keyboard up, the keyboard reported by the
// native editor itself (nativeComposeArea, the path Android's editor takes):
// the card folds to its compact row, Cancel and Share or Ask stand above the
// keyboard at the size they have without it, the count stays beside Show
// passage, and the editor takes the room between that row and the keyboard,
// the iPhone's slot as far as it fits and never less than the 44-unit floor
// where there is room for it, without reaching under the keyboard or over
// any action. Show passage caps the field at 60 units with the passage above
// it, and the editor stays mounted through Show and Hide passage. This test
// models geometry; it does not observe native keyboard focus or text rows.
func TestNativeComposeFitsAPhoneHeldSidewaysWithTheKeyboardUp(t *testing.T) {
	for _, phone := range []struct {
		name              string
		w, h, keyboardTop float32
	}{
		{"Pixel 7 emulator", 800, 360, 132},
		{"iPhone 16 Pro", 874, 402, 178},
	} {
		for _, action := range []string{"note", "ask"} {
			t.Run(phone.name+"/"+action, func(t *testing.T) {
				st := psalm23NoteState()
				win := noteSheetWindow(t, st, true, phone.w, phone.h)
				held := holdNoteSheetTimers(t)
				softKeyboard(t, false)
				prevArea, prevFoot := nativeComposeArea, keyboardFreeFoot
				keyboardFreeFoot.known = false
				raise := false
				nativeComposeArea = func(_ fyne.Canvas, pos fyne.Position, size fyne.Size) (fyne.Position, fyne.Size) {
					if raise {
						size.Height = phone.keyboardTop - pos.Y
					}
					return pos, size
				}
				t.Cleanup(func() { nativeComposeArea, keyboardFreeFoot = prevArea, prevFoot })
				_, shows, hides, frames := nativeAskEditor(t, win)

				openComposer(st, action)
				p := topPopup(t, win)
				upright := composerActions(p)
				raise = true
				noteSoftKeyboard(phone.h - phone.keyboardTop)
				runHeld(held)
				if b := findTreeButton(p, "Show passage"); b == nil || !shownTree(p)[b] {
					t.Fatal("the card did not fold to its compact row above the keyboard")
				}

				check := func(stage string, least, most float32) noteFrame {
					t.Helper()
					actions := composerActions(p)
					if len(actions) != 2 {
						t.Fatalf("%s: expected two actions, got %v", stage, actions)
					}
					slot := composerSlot(p)
					if slot == nil || !shownTree(p)[slot] {
						t.Fatalf("%s: the editor's slot is not showing", stage)
					}
					editor := rectOf(slot)
					for label, a := range actions {
						if a.sz != upright[label].sz {
							t.Errorf("%s: %s is %v with the keyboard up, %v without: a control changed size", stage, label, a.sz, upright[label].sz)
						}
						if a.bottom() > phone.keyboardTop+0.5 {
							t.Errorf("%s: %s ends at %v, under the keyboard at %v", stage, label, a.bottom(), phone.keyboardTop)
						}
						if editor.overlaps(a) {
							t.Errorf("%s: the editor %+v reaches over %s %+v", stage, editor, label, a)
						}
					}
					if editor.bottom() > phone.keyboardTop+0.5 {
						t.Errorf("%s: the editor ends at %v, under the keyboard at %v", stage, editor.bottom(), phone.keyboardTop)
					}
					if room := phone.keyboardTop - editor.pos.Y; room >= least && editor.sz.Height < least {
						t.Errorf("%s: the editor is %v units tall with %v of room, less than %v", stage, editor.sz.Height, room, least)
					}
					if editor.sz.Height > most {
						t.Errorf("%s: the editor is %v units tall, more than %v", stage, editor.sz.Height, most)
					}
					if action == "note" {
						counter, ok := composerCounter(p)
						if !ok {
							t.Errorf("%s: the count is not showing", stage)
						} else if counter.bottom() > phone.keyboardTop+0.5 || counter.overlaps(editor) {
							t.Errorf("%s: the count %+v is under the keyboard or the editor %+v", stage, counter, editor)
						}
					}
					if n := len(*frames); n == 0 || (*frames)[n-1] != editor {
						t.Errorf("%s: the native editor was last told %v, not the slot's %+v", stage, *frames, editor)
					}
					return editor
				}
				check("keyboard up", 44, iPhoneEditorSlot)

				test.Tap(findTreeButton(p, "Show passage"))
				runHeld(held)
				if findTreeButton(p, "Hide passage") == nil {
					t.Fatal("the passage did not expand")
				}
				check("passage shown", 44, 60)
				test.Tap(findTreeButton(p, "Hide passage"))
				runHeld(held)
				check("passage hidden again", 44, iPhoneEditorSlot)
				if *shows != 1 || *hides != 0 {
					t.Errorf("the native editor was shown %d times and hidden %d times through Show and Hide passage; it stays mounted", *shows, *hides)
				}
				findTreeButton(p, "Cancel").OnTapped()
				runHeld(held)
			})
		}
	}
}

// A composer too short for the passage and the editor together (dense): Show
// passage expands the passage above the native editor and Hide passage folds
// it again, and the editor stays mounted through both. The editor keeps a
// slot inside the modeled keyboard room; native keyboard focus and readable
// passage rows are checked separately.
func TestDenseNativeComposerKeepsItsEditorThroughShowPassage(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 800, 360)
	held := holdNoteSheetTimers(t)
	softKeyboard(t, false)
	prevArea, prevFoot := canvasArea, keyboardFreeFoot
	keyboardFreeFoot.known = false
	raise := false
	const top, keyboardRoom = float32(24.667), float32(106.667)
	canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
		height := c.Size().Height - top - 20
		if raise {
			height = keyboardRoom
		}
		return fyne.NewPos(0, top), fyne.NewSize(c.Size().Width, height)
	}
	t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })
	_, shows, hides, _ := nativeAskEditor(t, win)
	passage := askComposerFixture()
	passage.Text = strings.Repeat("A synthetic selected passage follows a path beside the field. ", 16) + "Final selected words."
	passage.PreviewText, passage.HasPreview = passage.Text, true
	promptAskQuestionForPassage(st, passage, "", verseTwo)
	popup := topPopup(t, win)
	raise = true
	noteSoftKeyboard(360 - top - keyboardRoom)
	runHeld(held)
	var viewport *composeViewport
	walkTree(popup, func(o fyne.CanvasObject) {
		if c, ok := o.(*fyne.Container); ok {
			if l, ok := c.Layout.(*composeViewport); ok {
				viewport = l
			}
		}
	})
	if viewport == nil {
		t.Fatal("missing compose viewport")
	}
	if !viewport.dense {
		t.Fatal("the composer is not dense on this canvas, so the test exercises nothing")
	}
	keyboardTop := top + keyboardRoom
	editorAbove := func(stage string) {
		t.Helper()
		slot := composerSlot(popup)
		if slot == nil || !shownTree(popup)[slot] {
			t.Fatalf("%s: the editor's slot is not showing", stage)
		}
		r := rectOf(slot)
		if r.sz.Height <= 0 || r.bottom() > keyboardTop+0.5 {
			t.Errorf("%s: the editor's slot %+v is not inside the room above the keyboard at %v", stage, r, keyboardTop)
		}
	}
	editorAbove("dense")
	test.Tap(findTreeButton(popup, "Show passage"))
	runHeld(held)
	if findTreeButton(popup, "Hide passage") == nil {
		t.Fatal("the passage did not expand")
	}
	editorAbove("passage shown")
	test.Tap(findTreeButton(popup, "Hide passage"))
	runHeld(held)
	editorAbove("passage hidden again")
	if *shows != 1 || *hides != 0 {
		t.Errorf("the native editor was shown %d times and hidden %d times; it stays mounted through Show and Hide passage", *shows, *hides)
	}
	test.Tap(findTreeButton(popup, "Cancel"))
	runHeld(held)
}

func TestNativeComposeDismissTargetsOnlyTheCurrentCard(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	holdNoteSheetTimers(t)
	_, _, hides, _ := nativeAskEditor(t, win)
	promptAskQuestionForPassage(st, askComposerFixture(), "fixture", verseTwo)
	old := noteEntryOwner
	first := topPopup(t, win)
	promptShareNote(st, "The fixture passage", verseTwo)
	second := topPopup(t, win)
	dismissNoteEntry(old)
	if !second.Visible() || *hides != 0 {
		t.Fatal("old Back dismissed the replacement editor")
	}
	dismissNoteEntry(noteEntryOwner)
	if second.Visible() || *hides != 1 {
		t.Fatal("current Back did not close the note exactly once")
	}
	dismissNoteEntry(noteEntryOwner)
	if *hides != 1 {
		t.Fatal("repeated Back closed a former editor again")
	}
	first.Hide()
}

// The Ask composer alone: a native Back closes it once, and a closed
// composer leaves no hook for a late Back to run.
func TestNativeAskBackClosesTheAskCardOnce(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	holdNoteSheetTimers(t)
	_, _, hides, _ := nativeAskEditor(t, win)
	promptAskQuestionForPassage(st, askComposerFixture(), "fixture", verseTwo)
	p := topPopup(t, win)
	if noteEntryOnDismiss == nil {
		t.Fatal("the Ask composer installed no dismiss hook, so a native Back cannot close it")
	}
	dismissNoteEntry(noteEntryOwner)
	if p.Visible() || *hides != 1 {
		t.Fatalf("Back did not close the Ask composer exactly once: visible %v, hides %d", p.Visible(), *hides)
	}
	if noteEntryOnDismiss != nil {
		t.Error("the closed Ask composer left its dismiss hook installed")
	}
	dismissNoteEntry(noteEntryOwner)
	if *hides != 1 {
		t.Errorf("a repeated Back closed the former editor again: hides %d", *hides)
	}
}

// canvasArea consults the native compose seam, the one path by which an
// Android editor's keyboard overlap reaches the sheet fit, and the composer
// refits above the area the seam gives.
func TestCanvasAreaConsultsTheNativeComposeSeam(t *testing.T) {
	prev := nativeComposeArea
	t.Cleanup(func() { nativeComposeArea = prev })
	nativeComposeArea = func(_ fyne.Canvas, pos fyne.Position, size fyne.Size) (fyne.Position, fyne.Size) {
		size.Height -= 100
		return pos, size
	}
	c := test.NewCanvas()
	c.Resize(fyne.NewSize(393, 852))
	_, driver := c.InteractiveArea()
	if _, got := canvasArea(c); got.Height != driver.Height-100 {
		t.Fatalf("canvasArea gives %v of the driver's %v: the native compose seam was not consulted", got.Height, driver.Height)
	}

	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 874, 402)
	held := holdNoteSheetTimers(t)
	softKeyboard(t, false)
	prevFoot := keyboardFreeFoot
	keyboardFreeFoot.known = false
	t.Cleanup(func() { keyboardFreeFoot = prevFoot })
	raise := false
	const keyboardTop = float32(178)
	nativeComposeArea = func(_ fyne.Canvas, pos fyne.Position, size fyne.Size) (fyne.Position, fyne.Size) {
		if raise {
			size.Height = keyboardTop - pos.Y
		}
		return pos, size
	}
	nativeNoteField(t, win)
	promptShareNote(st, "The fixture passage", verseTwo)
	p := topPopup(t, win)
	raise = true
	noteSoftKeyboard(224)
	runHeld(held)
	for _, label := range []string{"Cancel", "Share"} {
		b := findTreeButton(p, label)
		if b == nil {
			t.Fatalf("missing %s", label)
		}
		if bottom := fyne.CurrentApp().Driver().AbsolutePositionForObject(b).Y + b.Size().Height; bottom > keyboardTop+0.5 {
			t.Errorf("%s ends at %v, under the keyboard the seam reports at %v", label, bottom, keyboardTop)
		}
	}
	findTreeButton(p, "Cancel").OnTapped()
	runHeld(held)
}

// Expanded native composers cap the editor at 60 units without growing a
// field whose own minimum is shorter. The real 112-unit slot remains 60.
func TestNativeExpandedComposerDoesNotGrowAShortEditor(t *testing.T) {
	for _, tc := range []struct {
		name          string
		minimum, want float32
	}{
		{"short", 20, 20}, {"single-line", 44, 44}, {"cap", 60, 60}, {"normal-native", 112, 60},
	} {
		for _, dense := range []bool{false, true} {
			name := tc.name + "/roomy"
			if dense {
				name = tc.name + "/dense"
			}
			t.Run(name, func(t *testing.T) {
				toolbar, passage, entry := canvas.NewRectangle(color.Transparent), canvas.NewRectangle(color.Transparent), canvas.NewRectangle(color.Transparent)
				toolbar.SetMinSize(fyne.NewSize(10, 30))
				entry.SetMinSize(fyne.NewSize(10, tc.minimum))
				l := composeCompactLayout{native: true, metadataHeight: 80, expanded: true, dense: dense}
				l.Layout([]fyne.CanvasObject{toolbar, passage, entry}, fyne.NewSize(400, 300))
				if got := entry.Size().Height; got != tc.want {
					t.Errorf("expanded field is %v units tall, want %v for minimum %v", got, tc.want, tc.minimum)
				}
				if passage.Size().Height < 80 {
					t.Error("the short editor consumed the passage's reserved room")
				}
			})
		}
	}
}

// Expanding in short room and then gaining height keeps the expanded state.
// This explicitly reaches the non-dense compact native layout, rather than
// asserting only a range on a field squeezed by the keyboard.
func TestNativeExpandedComposerUsesSixtyUnitsAfterRoomGrows(t *testing.T) {
	for _, action := range []string{"note", "ask"} {
		t.Run(action, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, 800, 360)
			held := holdNoteSheetTimers(t)
			softKeyboard(t, false)
			previousArea, previousFoot, previousKeyboard := canvasArea, keyboardFreeFoot, noteEntryKeyboard
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
			t.Cleanup(func() { canvasArea, keyboardFreeFoot, noteEntryKeyboard = previousArea, previousFoot, previousKeyboard })
			_, shows, hides, _ := nativeAskEditor(t, win)
			// Isolate layout from the separate keyboard-dismissal policy.
			noteEntryKeyboard = func(uint64, bool) {}
			openComposer(st, action)
			popup := topPopup(t, win)
			before := composerActions(popup)
			raised = true
			noteSoftKeyboard(360 - top - keyboardRoom)
			runHeld(held)
			v := composerViewportOf(t, popup)
			if !v.dense {
				t.Fatal("control: the short layout did not become dense")
			}
			show := findTreeButton(popup, "Show passage")
			if show == nil || !shownTree(popup)[show] {
				t.Fatal("control: Show passage is not available")
			}
			test.Tap(show)
			runHeld(held)
			raised = false
			noteSoftKeyboard(0)
			win.Resize(fyne.NewSize(800, 900))
			popup.Refresh()
			runHeld(held)
			if !v.expanded || v.dense {
				t.Fatalf("did not reach expanded non-dense layout: expanded %v, dense %v", v.expanded, v.dense)
			}
			slot := composerSlot(popup)
			if slot == nil || !shownTree(popup)[slot] || slot.Size().Height != 60 {
				t.Fatalf("non-dense expanded native field is not exactly 60 units: %v", slot)
			}
			renderer := test.WidgetRenderer(v.excerpt).(*noteExcerptRenderer)
			if renderer.rowH <= 0 || len(renderer.drawn()) == 0 || !v.scroll.Visible() || v.scroll.Size().Height < renderer.rowH {
				t.Errorf("expanded passage has no complete readable row: scroll %v, row %v", v.scroll.Size(), renderer.rowH)
			}
			for label, old := range before {
				if current := composerActions(popup)[label]; current.sz != old.sz {
					t.Errorf("%s changed visual size: %v to %v", label, old.sz, current.sz)
				}
			}
			if *shows != 1 || *hides != 0 {
				t.Error("reflow replaced the mounted editor")
			}
			test.Tap(findTreeButton(popup, "Cancel"))
			runHeld(held)
		})
	}
}
