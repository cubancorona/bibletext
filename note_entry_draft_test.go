package bibletext

import (
	"strings"
	"testing"
)

func TestNativeComposeDraftRetainsUnicodeBeforeUIUpdates(t *testing.T) {
	var draft nativeComposeDraft
	initial := "A fixture: e\u0301 אבג 👩🏽‍💻\nSecond line.\x00"
	draft.begin(1, initial)
	if text, _, open := draft.read(1); !open || text != initial {
		t.Fatalf("initial text: %q, open %v", text, open)
	}
	typed := initial + strings.Repeat(" Long fixture text 🕊️", 60)
	if !draft.update(1, typed) {
		t.Fatal("current native edit rejected")
	}
	if text, _, _ := draft.read(1); text != typed {
		t.Fatal("a submit before the queued UI counter would lose the latest text")
	}
}

func TestNativeComposeDraftRejectsSupersededEditsAndKeyboard(t *testing.T) {
	var draft nativeComposeDraft
	draft.begin(1, "First draft")
	draft.begin(2, "Replacement")
	if draft.update(1, "Late edit") || draft.keyboard(1, 400) {
		t.Fatal("superseded callback changed the current draft")
	}
	draft.end(1)
	if text, px, open := draft.read(2); text != "Replacement" || px != 0 || !open {
		t.Fatal("old close changed the replacement")
	}
	if !draft.keyboard(2, 320) {
		t.Fatal("current keyboard update rejected")
	}
	if _, px, _ := draft.read(2); px != 320 {
		t.Fatal("keyboard geometry lost")
	}
}

func TestNativeComposeDraftCloseRetainsTextForReopen(t *testing.T) {
	var draft nativeComposeDraft
	draft.begin(7, "Retained fixture\n🕊️")
	draft.keyboard(7, 300)
	draft.end(7)
	text, px, open := draft.read(7)
	if text != "Retained fixture\n🕊️" || px != 0 || open {
		t.Fatal("close must retain text for the sheet's reopen capture")
	}
	if draft.update(7, "Late") || draft.keyboard(7, 400) {
		t.Fatal("closed native callback was accepted")
	}
	draft.begin(8, text)
	if again, _, open := draft.read(8); !open || again != text {
		t.Fatal("reopen lost the retained draft")
	}
}

func TestNativeComposePrefilledNoteRefreshesCounterAfterOwnerChanges(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	holdNoteSheetTimers(t)
	nativeNoteField(t, win)
	var draft nativeComposeDraft
	read, show, hide := noteEntryTyped, noteEntryShow, noteEntryHide
	noteEntryTyped = func() string { text, _, _ := draft.read(noteEntryOwner); return text }
	noteEntryShow = func(initial, _ string, _ palette) { draft.begin(noteEntryOwner, initial) }
	noteEntryHide = func() { draft.end(noteEntryOwner) }
	t.Cleanup(func() { noteEntryTyped, noteEntryShow, noteEntryHide = read, show, hide })
	text := strings.Repeat("Long fixture draft. ", 200)
	promptShareNoteWith(st, "The fixture passage", selSpanFromNative(1, 1), text, selectionPassage(st, selSpanFromNative(1, 1)))
	p := topPopup(t, win)
	labels := strings.Join(sheetTexts(p), "\n")
	if !strings.Contains(labels, "Will be shortened") && !strings.Contains(labels, "Too long") {
		t.Fatal("the visible counter did not refresh after the native owner started")
	}
	findTreeButton(p, "Cancel").OnTapped()
}
