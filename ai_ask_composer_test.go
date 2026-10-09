package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func expectedPreviewCitation(ref, abbrev string) string {
	if abbrev == "" {
		return ref
	}
	if ref == "" {
		return "(" + abbrev + ")"
	}
	return ref + " (" + abbrev + ")"
}

func askComposerFixture() aiPassage {
	p := aiPassage{
		Book: "Fixture Book", Chapter: 2, Lo: 1, Hi: 2,
		Ref: "Fixture Book 2:1–2", Version: "Fixture translation",
		Text: "The first fixture verse describes a field beside a hill. " +
			"The second fixture verse describes a path from the field to the river.",
	}
	if nextRelease {
		p.VersionAbbrev = "FXT"
		p.PreviewText = "The fixture preview names the Lᴏʀᴅ beside a field and a hill. " +
			"Its second sentence follows the path from that field to the river."
		p.HasPreview = true
	}
	return p
}

func assertAskComposerPassage(t *testing.T, popup *widget.PopUp, passage aiPassage) {
	t.Helper()
	if nextRelease {
		assertCapturedPassagePreview(t, popup, passage)
		return
	}
	if !sheetHas(popup, passage.Ref) || !sheetHas(popup, passage.Version) {
		t.Errorf("current composer changed its exact reference or translation; texts %v", sheetTexts(popup))
	}
	ex, _ := noteSheetExcerpt(t, popup)
	if ex.text != strings.Join(strings.Fields(passage.Text), " ") {
		t.Errorf("current composer excerpt = %q, want raw selected text %q", ex.text, passage.Text)
	}
}

func assertAskPanelPassage(t *testing.T, popup *widget.PopUp, passage aiPassage) {
	t.Helper()
	if nextRelease {
		assertCapturedPassagePreview(t, popup, passage)
		return
	}
	for _, want := range []string{passage.Ref, passage.Version, quotedOneLine(passage.Text, 300)} {
		if !sheetHas(popup, want) {
			t.Errorf("current panel lost exact %q; texts %v", want, sheetTexts(popup))
		}
	}
}

func assertCapturedPassagePreview(t *testing.T, popup *widget.PopUp, passage aiPassage) {
	t.Helper()
	ex, renderer := noteSheetExcerpt(t, popup)
	wantText := passage.Text
	if passage.HasPreview {
		wantText = passage.PreviewText
	}
	wantText = strings.Join(strings.Fields(wantText), " ")
	if ex.text != wantText {
		t.Errorf("preview excerpt = %q, want captured %q", ex.text, wantText)
	}
	for i, row := range renderer.rows {
		if row.FontSource != styledPaneFont() || row.TextStyle.Italic {
			t.Errorf("preview row %d must use the upright reading face", i)
		}
	}
	abbrev := passage.VersionAbbrev
	if abbrev == "" {
		abbrev = passage.Version
	}
	wantCitation := expectedPreviewCitation(passage.Ref, abbrev)
	var citation *widget.RichText
	count := 0
	walkTree(popup, func(o fyne.CanvasObject) {
		if rich, ok := o.(*widget.RichText); ok && segmentText(rich.Segments) == wantCitation {
			citation = rich
			count++
		}
	})
	if count != 1 {
		t.Fatalf("preview has %d exact combined citations %q; texts %v", count, wantCitation, sheetTexts(popup))
	}
	if len(citation.Segments) != 1 {
		t.Fatalf("citation has %d segments, want one exact string", len(citation.Segments))
	}
	segment, ok := citation.Segments[0].(*widget.TextSegment)
	if !ok || segment.Text != wantCitation || segment.Style.Alignment != fyne.TextAlignTrailing || citation.Wrapping != fyne.TextWrapWord {
		t.Errorf("citation must be the exact trailing-aligned wrapping string %q", wantCitation)
	}
	quoteBottom := fyne.CurrentApp().Driver().AbsolutePositionForObject(ex).Y + ex.Size().Height
	if citationY := fyne.CurrentApp().Driver().AbsolutePositionForObject(citation).Y; citationY < quoteBottom {
		t.Errorf("citation is above or over the preview: citation Y %v, quote bottom %v", citationY, quoteBottom)
	}
	if sheetHas(popup, passage.Ref) || sheetHas(popup, passage.Version) {
		t.Errorf("next preview retained a separate reference or full translation; texts %v", sheetTexts(popup))
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
			if len(renderer.drawn()) < 2 {
				t.Fatalf("the selection must be a wrapping excerpt; text %q, rows %v", ex.text, renderer.drawn())
			}
			assertAskComposerPassage(t, first, passage)
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
			st.CurrentBook, st.CurrentChapter, st.CurrentVersion = "Another fixture book", 8, "bsb"
			reopen()
			again := topPopup(t, win)
			if again == first || composerField(again).Text != question {
				t.Fatal("the reopened composer lost the question")
			}
			assertAskComposerPassage(t, again, passage)
			test.Tap(findTreeButton(again, "Cancel"))
		})
	}
}

func TestAskComposerRetainsAnIntentionallyEmptyPreview(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, false, 393, 852)
	holdNoteSheetTimers(t)
	passage := askComposerFixture()
	if nextRelease {
		passage.PreviewText = ""
	}
	promptAskQuestionForPassage(st, passage, "", verseTwo)
	first := topPopup(t, win)
	assertAskComposerPassage(t, first, passage)
	if excerpt, _ := noteSheetExcerpt(t, first); excerpt.Visible() != !nextRelease {
		t.Errorf("empty captured preview visibility = %v, want %v", excerpt.Visible(), !nextRelease)
	}
	reopen := takeTopSheetReopen(st)
	if reopen == nil {
		t.Fatal("the composer did not register a reopen")
	}
	first.Hide()
	st.CurrentBook, st.CurrentChapter, st.CurrentVersion = "Another fixture book", 8, "bsb"
	reopen()
	again := topPopup(t, win)
	assertAskComposerPassage(t, again, passage)
	if excerpt, _ := noteSheetExcerpt(t, again); excerpt.Visible() != !nextRelease {
		t.Errorf("reopened empty preview visibility = %v, want %v", excerpt.Visible(), !nextRelease)
	}
	test.Tap(findTreeButton(again, "Cancel"))
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
	if nextRelease {
		passage.PreviewText = strings.Repeat("A fixture preview with a deliberately long word. ", 30)
	}
	promptAskQuestionForPassage(st, passage, "", selSpan{})
	p := topPopup(t, win)
	ex, renderer := noteSheetExcerpt(t, p)
	assertAskComposerPassage(t, p, passage)
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
