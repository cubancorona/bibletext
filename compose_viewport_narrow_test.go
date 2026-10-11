package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func walkVisibleComposer(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if object == nil || !object.Visible() {
		return
	}
	visit(object)
	var children []fyne.CanvasObject
	switch value := object.(type) {
	case *fyne.Container:
		children = value.Objects
	case *container.Scroll:
		children = []fyne.CanvasObject{value.Content}
	case *container.ThemeOverride:
		children = []fyne.CanvasObject{value.Content}
	case *widget.PopUp:
		children = []fyne.CanvasObject{value.Content}
	}
	for _, child := range children {
		walkVisibleComposer(child, visit)
	}
}

// The smallest portrait phone still needs room for its draft and controls
// beside a wrapping citation. The geometry is read from the actual composer,
// including native editor frames, after its keyboard and disclosure callbacks.
func TestMobileComposerFitsNarrowPhoneKeyboard(t *testing.T) {
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
				state := psalm23NoteState()
				win := noteSheetWindow(t, state, true, 320, 568)
				held := holdNoteSheetTimers(t)
				softKeyboard(t, false)
				previousArea, previousFoot := canvasArea, keyboardFreeFoot
				keyboardFreeFoot.known = false
				up := false
				canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
					bottom := float32(548)
					if up {
						bottom = 300
					}
					return fyne.NewPos(0, 20), fyne.NewSize(c.Size().Width, bottom-20)
				}
				t.Cleanup(func() { canvasArea, keyboardFreeFoot = previousArea, previousFoot })
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
				passage := askComposerFixture()
				passage.Book = "An Exceptionally Long Synthetic Book Name for a Narrow Passage Card"
				passage.Chapter, passage.Lo, passage.Hi = 123, 45, 67
				passage.Ref = passage.Book + " 123:45–67"
				passage.Text = strings.Repeat("A synthetic selected passage follows a path beside the field. ", 16) + "Final selected words."
				passage.PreviewText, passage.HasPreview = passage.Text, true
				if ask {
					promptAskQuestionForPassage(state, passage, "", verseTwo)
				} else {
					promptShareNote(state, drawnSelection(t, state, 1, 2), verseTwo)
				}
				popup := topPopup(t, win)
				draft := "A synthetic question draft."
				if !ask {
					draft = strings.Repeat("x", NoteMaxRunes+1)
				}
				if native {
					typed = draft
					if noteEntryOnChanged != nil {
						noteEntryOnChanged()
					}
				} else {
					composerField(popup).SetText(draft)
				}
				up = true
				noteSoftKeyboard(268)
				runHeld(held)
				primary := "Share"
				if ask {
					primary = "Ask"
				}
				checkRect := func(label string, position fyne.Position, size fyne.Size) {
					t.Helper()
					if position.X < -0.5 || position.X+size.Width > 320.5 || position.Y < 19.5 || position.Y+size.Height > 300.5 {
						t.Errorf("%s outside narrow keyboard-free area: %v %v", label, position, size)
					}
				}
				check := func() noteFrame {
					t.Helper()
					driver := fyne.CurrentApp().Driver()
					buttons := []*widget.Button{}
					for _, label := range []string{"Cancel", primary, "Show passage"} {
						button := findTreeButton(popup, label)
						if label == "Show passage" && button == nil {
							button = findTreeButton(popup, "Hide passage")
						}
						if label == "Show passage" && button == nil && !ask {
							continue // the ordinary Fyne Share form can fit in full
						}
						if button == nil {
							t.Fatalf("missing %s", label)
						}
						checkRect(label, driver.AbsolutePositionForObject(button), button.Size())
						buttons = append(buttons, button)
					}
					// A Border layout can keep both boxes on-screen while the
					// title paints over a button on a narrow toolbar.
					walkVisibleComposer(popup, func(object fyne.CanvasObject) {
						text, ok := object.(*canvas.Text)
						if !ok || (text.Text != "Ask a question" && text.Text != "Will be shortened") {
							return
						}
						position := driver.AbsolutePositionForObject(text)
						checkRect("compact heading", position, text.Size())
						for _, button := range buttons[:2] {
							b := driver.AbsolutePositionForObject(button)
							if position.X < b.X+button.Size().Width && b.X < position.X+text.Size().Width && position.Y < b.Y+button.Size().Height && b.Y < position.Y+text.Size().Height {
								t.Errorf("compact heading overlaps %s: heading %v %v, button %v %v", button.Text, position, text.Size(), b, button.Size())
							}
						}
					})
					var position fyne.Position
					var size fyne.Size
					if native {
						if len(*frames) == 0 {
							t.Fatal("missing native editor frame")
						}
						last := (*frames)[len(*frames)-1]
						position, size = last.pos, last.sz
						if typed != draft {
							t.Error("passage disclosure changed the native draft")
						}
					} else {
						entry := composerField(popup)
						position, size = driver.AbsolutePositionForObject(entry), entry.Size()
						if entry.Text != draft || win.Canvas().Focused() != entry {
							t.Error("passage disclosure changed the draft or its focus")
						}
					}
					checkRect("editor", position, size)
					if size.Height < 30 {
						t.Errorf("editor has no usable text line: %v", size)
					}
					return noteFrame{position, size}
				}
				editor := check()
				if ask {
					var citation *widget.RichText
					walkVisibleComposer(popup, func(object fyne.CanvasObject) {
						if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == passagePreviewCitation(passage.Ref, passage.previewVersion()) {
							citation = rich
						}
					})
					if citation == nil || citation.Size().Height < 30 {
						t.Fatal("the long collapsed citation did not wrap")
					}
					position := fyne.CurrentApp().Driver().AbsolutePositionForObject(citation)
					checkRect("wrapping citation", position, citation.Size())
					if position.Y+citation.Size().Height > editor.pos.Y+0.5 {
						t.Error("wrapping citation overlaps the editor")
					}
				}
				show := findTreeButton(popup, "Show passage")
				if show == nil {
					test.Tap(findTreeButton(popup, "Cancel"))
					runHeld(held)
					return
				}
				test.Tap(show)
				runHeld(held)
				check()
				if ask {
					excerpt, renderer := noteSheetExcerpt(t, popup)
					rows := renderer.drawn()
					if excerpt.text != passage.PreviewText || len(rows) <= 3 || !strings.HasSuffix(strings.Join(rows, " "), "Final selected words.") {
						t.Errorf("expanded passage lost its full selected text: %v", rows)
					}
					scroll := scrollHolding(popup, excerpt)
					if scroll == nil {
						t.Fatal("the expanded selection has no scroll")
					}
					scroll.ScrollToBottom()
					if scroll.Offset.Y <= 0 || scroll.Content.Size().Height-scroll.Offset.Y > scroll.Size().Height+0.5 {
						t.Error("expanded passage ending is outside the scroll's reach")
					}
				}
				test.Tap(findTreeButton(popup, "Hide passage"))
				runHeld(held)
				check()
				test.Tap(findTreeButton(popup, "Cancel"))
				runHeld(held)
			})
		}
	}
}

func TestDenseAskComposerDismissesKeyboardForPassage(t *testing.T) {
	state := psalm23NoteState()
	win := noteSheetWindow(t, state, true, 800, 360)
	held := holdNoteSheetTimers(t)
	softKeyboard(t, false)
	previousArea, previousFoot := canvasArea, keyboardFreeFoot
	keyboardFreeFoot.known = false
	up := false
	const top, keyboardRoom = float32(24.667), float32(106.667)
	canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
		height := c.Size().Height - top - 20
		if up {
			height = keyboardRoom
		}
		return fyne.NewPos(0, top), fyne.NewSize(c.Size().Width, height)
	}
	t.Cleanup(func() { canvasArea, keyboardFreeFoot = previousArea, previousFoot })
	passage := askComposerFixture()
	passage.Text = strings.Repeat("A synthetic selected passage follows a path beside the field. ", 16) + "Final selected words."
	passage.PreviewText, passage.HasPreview = passage.Text, true
	promptAskQuestionForPassage(state, passage, "", verseTwo)
	popup := topPopup(t, win)
	entry := composerField(popup)
	if entry == nil {
		t.Fatal("the Android composer lost its Fyne editor")
	}
	const draft = "A synthetic draft on the dense landscape phone."
	entry.SetText(draft)
	up = true
	noteSoftKeyboard(360 - top - keyboardRoom)
	runHeld(held)
	checkTyping := func() {
		t.Helper()
		driver := fyne.CurrentApp().Driver()
		var citation *widget.RichText
		walkVisibleComposer(popup, func(object fyne.CanvasObject) {
			if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == passagePreviewCitation(passage.Ref, passage.previewVersion()) {
				if citation != nil {
					t.Error("the dense toolbar repeats its citation")
				}
				citation = rich
			}
		})
		if citation == nil {
			t.Fatal("the dense toolbar lost its citation")
		}
		objects := []fyne.CanvasObject{citation}
		for _, label := range []string{"Show passage", "Cancel", "Ask"} {
			button := findTreeButton(popup, label)
			if button == nil {
				t.Fatalf("the dense toolbar lost %s", label)
			}
			objects = append(objects, button)
		}
		objects = append(objects, entry)
		for index, object := range objects {
			position, size := driver.AbsolutePositionForObject(object), object.Size()
			if position.X < -0.5 || position.X+size.Width > 800.5 || position.Y < top-0.5 || position.Y+size.Height > top+keyboardRoom+0.5 {
				t.Errorf("dense control %d outside keyboard-free area: %v %v", index, position, size)
			}
			for _, other := range objects[:index] {
				otherPosition, otherSize := driver.AbsolutePositionForObject(other), other.Size()
				if position.X < otherPosition.X+otherSize.Width-0.5 && otherPosition.X < position.X+size.Width-0.5 && position.Y < otherPosition.Y+otherSize.Height-0.5 && otherPosition.Y < position.Y+size.Height-0.5 {
					t.Errorf("dense controls overlap: %T %v %v, %T %v %v", object, position, size, other, otherPosition, otherSize)
				}
			}
		}
		if entry.Size().Height < 24 || entry.Text != draft || win.Canvas().Focused() != entry {
			t.Errorf("dense editor lost a usable line, draft or focus: %v, %q", entry.Size(), entry.Text)
		}
	}
	checkTyping()
	test.Tap(findTreeButton(popup, "Show passage"))
	if win.Canvas().Focused() != nil || entry.Text != draft {
		t.Error("dense passage disclosure did not dismiss typing while retaining the draft")
	}
	up = false
	noteSoftKeyboard(0)
	runHeld(held)
	if findTreeButton(popup, "Hide passage") == nil || entry.Text != draft {
		t.Fatal("keyboard dismissal lost the expanded passage or draft")
	}
	excerpt, renderer := noteSheetExcerpt(t, popup)
	rows := renderer.drawn()
	if excerpt.text != passage.PreviewText || len(rows) <= 3 || !strings.HasSuffix(strings.Join(rows, " "), "Final selected words.") {
		t.Errorf("the expanded passage lost its full selected text: %v", rows)
	}
	scroll := scrollHolding(popup, excerpt)
	if scroll == nil {
		t.Fatal("the expanded passage has no scroll")
	}
	scroll.ScrollToBottom()
	if scroll.Offset.Y <= 0 || scroll.Content.Size().Height-scroll.Offset.Y > scroll.Size().Height+0.5 {
		t.Error("the full selected passage is outside the scroll's reach")
	}
	win.Canvas().Focus(entry)
	up = true
	noteSoftKeyboard(360 - top - keyboardRoom)
	runHeld(held)
	checkTyping()
	test.Tap(findTreeButton(popup, "Cancel"))
	runHeld(held)
}
