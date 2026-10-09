package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func askComposerFixture() aiPassage {
	return aiPassage{
		Book: "Fixture Book", Chapter: 2, Lo: 1, Hi: 2,
		Ref: "Fixture Book 2:1–2", Version: "Fixture translation",
		Text: "The first fixture verse describes a field beside a hill. " +
			"The second fixture verse describes a path from the field to the river.",
	}
}

func TestAskComposerKeepsTheExcerptAndQuestionOnReopen(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		name := "desktop"
		if mobile {
			name = "mobile Fyne field"
		}
		t.Run(name, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, mobile, 393, 852)
			holdNoteSheetTimers(t)
			prevNative := noteEntryNative
			noteEntryNative = func() bool { return false }
			t.Cleanup(func() { noteEntryNative = prevNative })
			passage := askComposerFixture()
			promptAskQuestionForPassage(st, passage, "", verseTwo)
			first := topPopup(t, win)
			ex, renderer := noteSheetExcerpt(t, first)
			if ex.text != passage.Text || len(renderer.drawn()) < 2 {
				t.Fatalf("the selection must be a wrapping excerpt; text %q, rows %v", ex.text, renderer.drawn())
			}
			if !sheetHas(first, passage.Ref) || !sheetHas(first, passage.Version) {
				t.Errorf("the card must name its reference and translation; texts %v", sheetTexts(first))
			}
			field := composerField(first)
			if field == nil {
				t.Fatal("the fallback composer has no question field")
			}
			const question = "What does the fixture passage mean?"
			field.SetText(question)
			reopen := takeTopSheetReopen(st)
			if reopen == nil {
				t.Fatal("the composer did not register a reopen")
			}
			first.Hide()
			st.CurrentBook, st.CurrentChapter = "Another fixture book", 8
			reopen()
			again := topPopup(t, win)
			if again == first || composerField(again).Text != question {
				t.Fatal("the reopened composer lost the question")
			}
			if !sheetHas(again, passage.Ref) || !sheetHas(again, passage.Version) {
				t.Errorf("the reopened composer changed its passage; texts %v", sheetTexts(again))
			}
			if ex, _ := noteSheetExcerpt(t, again); ex.text != passage.Text {
				t.Errorf("the reopened excerpt is %q, want %q", ex.text, passage.Text)
			}
			test.Tap(findTreeButton(again, "Cancel"))
		})
	}
}

func TestNativeEntrySlotStopsWhenAnotherComposerOwnsTheEditor(t *testing.T) {
	held := holdNoteSheetTimers(t)
	prevFrame := noteEntryFrameTo
	frames := 0
	noteEntryFrameTo = func(fyne.CanvasObject) { frames++ }
	t.Cleanup(func() { noteEntryFrameTo = prevFrame })
	noteEntryOwner++
	open := true
	slot := newNoteEntrySlot(&open)
	slot.push()
	if frames != 1 || len(*held) != 1 {
		t.Fatalf("control: the current slot must push and schedule a second push; frames %d, callbacks %d", frames, len(*held))
	}
	noteEntryOwner++
	slot.push()
	(*held)[0]()
	if frames != 1 {
		t.Errorf("the superseded slot repositioned the new editor %d times", frames-1)
	}
	current := newNoteEntrySlot(&open)
	current.push()
	if frames != 2 {
		t.Fatal("control: the new owner's slot stopped pushing frames")
	}
	open = false
	for _, f := range *held {
		f()
	}
	if frames != 2 {
		t.Error("a closed slot pushed a delayed frame")
	}
}

func TestAskComposerExcerptNeverWidensANarrowCard(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 320, 1024)
	holdNoteSheetTimers(t)
	passage := askComposerFixture()
	passage.Text = strings.Repeat("A fixture passage with a deliberately long word. ", 30)
	promptAskQuestionForPassage(st, passage, "", selSpan{})
	p := topPopup(t, win)
	ex, renderer := noteSheetExcerpt(t, p)
	if ex.Size().Width > win.Canvas().Size().Width || len(renderer.drawn()) != noteExcerptMaxLines {
		t.Errorf("the excerpt must remain within the card and its row cap; size %v, rows %v", ex.Size(), renderer.drawn())
	}
	button := findTreeButton(p, "Ask")
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(button)
	if pos.X+button.Size().Width > win.Canvas().Size().Width {
		t.Errorf("the Ask button is outside the canvas: %v at %v", button.Size(), pos)
	}
	test.Tap(findTreeButton(p, "Cancel"))
}
