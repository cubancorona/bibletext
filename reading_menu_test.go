package bibletext

// Tests for the Fyne reading pane's selection study menu (the Win/Linux twin of
// the native selection menus), the selection normalization it feeds the study
// actions, and the text-size setting finally applying to the Fyne pane.
// Untagged: chapterText is untagged, so these run on every platform's CI.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func menuLabels(m *fyne.Menu) []string {
	labels := make([]string, 0, len(m.Items))
	for _, it := range m.Items {
		if it.IsSeparator {
			labels = append(labels, "---")
			continue
		}
		labels = append(labels, it.Label)
	}
	return labels
}

func TestPlainSelection(t *testing.T) {
	in := "¹⁶ For God so\nloved the world,\n\n¹⁷ For God did not send"
	want := "16 For God so loved the world,\n\n17 For God did not send"
	if got := plainSelection(in); got != want {
		t.Fatalf("plainSelection = %q, want %q", got, want)
	}
}

func TestSelectionMenuMirrorsNativeLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	verses := state.Bible.GetChapter("John", 1)
	c := newChapterText(state, verses)

	// AI on (default): Copy, Select all, ---, Study with AI, Share, Cross-references.
	m := c.menuForSelection("For God so loved the world")
	want := []string{"Copy", "Select all", "---", "Study with AI", "Share", "Cross-references"}
	if got := menuLabels(m); len(got) != len(want) {
		t.Fatalf("AI-on menu = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("AI-on menu = %v, want %v", got, want)
			}
		}
	}
	// The AI submenu carries the three study actions; Share carries all four
	// forms — with citation, as image, as a link, and as a link with a note.
	ai := m.Items[3]
	if ai.ChildMenu == nil || len(ai.ChildMenu.Items) != 3 {
		t.Fatalf("Study with AI submenu wrong: %+v", ai.ChildMenu)
	}
	if share := m.Items[4]; share.ChildMenu == nil || len(share.ChildMenu.Items) != 4 {
		t.Fatalf("Share submenu wrong: %+v", share.ChildMenu)
	}

	// AI off ("None"): Cross-references takes the study slot, ahead of Share.
	state.aiKeys.setAIEnabled(false)
	m = c.menuForSelection("For God so loved the world")
	want = []string{"Copy", "Select all", "---", "Cross-references", "Share"}
	got := menuLabels(m)
	if len(got) != len(want) {
		t.Fatalf("AI-off menu = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AI-off menu = %v, want %v", got, want)
		}
	}

	// No selection: just Copy (disabled) + Select all — no study group.
	m = c.menuForSelection("")
	if got := menuLabels(m); len(got) != 2 || got[0] != "Copy" || got[1] != "Select all" {
		t.Fatalf("empty-selection menu = %v", got)
	}
	if !m.Items[0].Disabled {
		t.Fatal("Copy must be disabled with no selection")
	}
}

func TestReadingTextSizeApplies(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	verses := state.Bible.GetChapter("John", 1)

	setReadingTextSizeID("normal")
	normal := newChapterText(state, verses)
	if diff := normal.textSize - theme.TextSize(); diff < -0.01 || diff > 0.01 {
		t.Fatalf("normal textSize = %v, want theme default %v", normal.textSize, theme.TextSize())
	}
	normal.rewrap(300)

	setReadingTextSizeID("xl")
	defer setReadingTextSizeID("normal")
	xl := newChapterText(state, verses)
	wantXL := theme.TextSize() * 1.3
	if diff := xl.textSize - wantXL; diff < -0.01 || diff > 0.01 {
		t.Fatalf("xl textSize = %v, want %v", xl.textSize, wantXL)
	}
	xl.rewrap(300)

	// Bigger text at the same width must wrap onto more lines — the visible
	// proof the setting reaches the pane (it changed nothing before).
	if xl.totalLines <= normal.totalLines {
		t.Fatalf("xl must wrap more lines than normal: xl=%d normal=%d", xl.totalLines, normal.totalLines)
	}
}
