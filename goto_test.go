package bibletext

import (
	"fmt"
	"strconv"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// pickerPopup opens a picker on the state's window and returns its popup overlay.
func pickerPopup(t *testing.T, state *AppState, open func(*AppState)) *widget.PopUp {
	t.Helper()
	open(state)
	popup, ok := state.window.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || popup == nil {
		t.Fatalf("expected a picker popup overlay, got %T", state.window.Canvas().Overlays().Top())
	}
	return popup
}

// findNumberEntry walks the constructed tree for a numberEntry by placeholder.
func findNumberEntry(o fyne.CanvasObject, placeholder string) *numberEntry {
	var found *numberEntry
	walkTree(o, func(n fyne.CanvasObject) {
		if e, ok := n.(*numberEntry); ok && found == nil && e.PlaceHolder == placeholder {
			found = e
		}
	})
	return found
}

func TestShowChapterPicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Chapter Picker Test")
	state := sampleState()
	state.window = win

	popup := pickerPopup(t, state, showChapterPicker)

	if !treeHasText(popup, "Go to") {
		t.Error("picker missing its title")
	}

	// The chapter grid shows the reader's current book: heading, one button per
	// chapter, the current chapter highlighted.
	nums := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	if len(nums) < 2 {
		t.Fatal("sample data must give the current book several chapters")
	}
	if want := fmt.Sprintf("%s · %d chapters", state.CurrentBook, len(nums)); !treeHasText(popup, want) {
		t.Errorf("picker missing the chapter-grid heading %q; texts: %v", want, treeTexts(popup))
	}
	var current *widget.Button
	for _, n := range nums {
		btn := findTreeButton(popup, strconv.Itoa(n))
		if btn == nil {
			t.Fatalf("no grid button for chapter %d", n)
		}
		if n == state.CurrentChapter {
			current = btn
		}
	}
	if current == nil || current.Importance != widget.HighImportance {
		t.Error("the current chapter's grid button must be highlighted")
	}

	// The left pane is the full book list in Bible order — and this flavour has
	// no verse-range row.
	var list *widget.List
	walkTree(popup, func(n fyne.CanvasObject) {
		if l, ok := n.(*widget.List); ok && list == nil {
			list = l
		}
	})
	if list == nil {
		t.Fatal("chapter picker missing its book list")
	}
	if got, want := list.Length(), len(state.Bible.Books); got != want {
		t.Errorf("book list has %d rows, want %d", got, want)
	}
	row := list.CreateItem()
	list.UpdateItem(0, row)
	if lbl, ok := row.(*widget.Label); !ok || lbl.Text != state.Bible.Books[0] {
		t.Errorf("book row 0 renders %#v, want a label reading %q", row, state.Bible.Books[0])
	}
	if findNumberEntry(popup, "verse") != nil {
		t.Error("the chapter picker must not carry the verse-range row")
	}

	// Tapping a chapter navigates immediately and closes the picker.
	target := nums[len(nums)-1]
	if target == state.CurrentChapter {
		target = nums[0]
	}
	test.Tap(findTreeButton(popup, strconv.Itoa(target)))
	if state.CurrentBook != "John" || state.CurrentChapter != target {
		t.Fatalf("chapter tap landed on %s %d, want John %d", state.CurrentBook, state.CurrentChapter, target)
	}
	if win.Canvas().Overlays().Top() != nil {
		t.Fatal("the picker must close after a chapter tap")
	}
}

func TestShowGotoPicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Goto Picker Test")
	state := sampleState() // John 1; sample data includes John 3:16
	state.window = win

	popup := pickerPopup(t, state, showGotoPicker)

	if !treeHasText(popup, "Go to") {
		t.Error("picker missing its title")
	}

	// The verse flavour's left pane is the alphabet navigator: one key per
	// distinct book initial, the current book's letter highlighted.
	letters := bookLetters(alphabeticalBooks(state.Bible.Books))
	if len(letters) < 2 {
		t.Fatal("sample data must span several book initials")
	}
	for _, r := range letters {
		if findTreeButton(popup, string(r)) == nil {
			t.Errorf("no alphabet key for %q", string(r))
		}
	}
	if cur := findTreeButton(popup, string(firstLetter(state.CurrentBook))); cur == nil || cur.Importance != widget.HighImportance {
		t.Error("the current book's letter key must be highlighted")
	}

	// The verse-range row: start + end number fields and the Go commit button.
	start := findNumberEntry(popup, "verse")
	end := findNumberEntry(popup, "end")
	goBtn := findTreeButton(popup, "Go")
	if start == nil || end == nil || goBtn == nil {
		t.Fatalf("verse row incomplete: start=%v end=%v go=%v", start != nil, end != nil, goBtn != nil)
	}

	// A chapter tap only SELECTS in this flavour: it re-highlights in place and
	// neither navigates nor closes the picker.
	chBtn := findTreeButton(popup, "3") // John 3 — the chapter holding 3:16
	if chBtn == nil {
		t.Fatal("no grid button for John chapter 3")
	}
	test.Tap(chBtn)
	if state.CurrentChapter != 1 {
		t.Fatalf("selecting a chapter must not navigate, reader moved to chapter %d", state.CurrentChapter)
	}
	if win.Canvas().Overlays().Top() == nil {
		t.Fatal("selecting a chapter must not close the Go-to picker")
	}
	if chBtn.Importance != widget.HighImportance {
		t.Error("the tapped chapter must gain the in-place highlight")
	}
	if b1 := findTreeButton(popup, "1"); b1 != nil && b1.Importance == widget.HighImportance {
		t.Error("the previously selected chapter must lose its highlight")
	}

	// Go commits the selection with the typed verse: John 3:16, picker closed.
	start.Text = "16" // set directly; commit reads the field
	test.Tap(goBtn)
	if win.Canvas().Overlays().Top() != nil {
		t.Fatal("Go must close the picker")
	}
	if state.CurrentBook != "John" || state.CurrentChapter != 3 {
		t.Fatalf("Go landed on %s %d, want John 3", state.CurrentBook, state.CurrentChapter)
	}
	if !state.HasHighlightedVerse || state.HighlightedVerse != 16 {
		t.Fatalf("Go must highlight verse 16, got hv=%v v=%d", state.HasHighlightedVerse, state.HighlightedVerse)
	}
}
