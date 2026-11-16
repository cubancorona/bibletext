//go:build !race

package bibletext

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func selectionExcerptState(version, book string) (*AppState, []Verse) {
	bd := xrefBible(map[string]map[int]int{book: {1: 4}, "Exodus": {1: 3}, "Luke": {1: 3}})
	for i := range bd.Verses[book][1] {
		bd.Verses[book][1][i].Text = fmt.Sprintf("Selected passage number %d keeps its own words.", i+1)
	}
	st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: 1, CurrentVersion: version}
	return st, versesBetween(bd, book, 1, 1, 4)
}

func requireUnletteredSelectionExcerpt(t *testing.T, list crossRefList, selected []Verse) {
	t.Helper()
	p := passageIn(list.Objects)
	if !list.Passage || p == nil {
		t.Fatal("the reference list has no selected-passage excerpt")
	}
	var wantText, wantNumbers, numbers []string
	for _, v := range selected {
		wantText = append(wantText, v.Text)
		wantNumbers = append(wantNumbers, fmt.Sprint(v.Verse))
	}
	if got := passagePlainText(p.runs); got != strings.Join(wantText, "\n") {
		t.Fatalf("excerpt changed or omitted selected text: %q", got)
	}
	for _, run := range p.runs {
		if !run.Mark {
			continue
		}
		if !run.Dim {
			t.Fatalf("excerpt invented a word-anchor letter %q", run.Text)
		}
		numbers = append(numbers, run.Text)
	}
	if !slices.Equal(numbers, wantNumbers) {
		t.Fatalf("excerpt verse numbers = %v, want the complete selection %v", numbers, wantNumbers)
	}
}

func TestCrossRefSelectionExcerptIncludesVersesWithoutTreasuryRows(t *testing.T) {
	th := appThemeForTest(t)
	setReadingTextSizeID("normal")
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	refs := []crossRef{
		{Book: "Exodus", Chapter: 1, Verse: 1, SourceVerse: 2},
		{Book: "Exodus", Chapter: 1, Verse: 2, SourceVerse: 3},
		{Book: "Exodus", Chapter: 1, Verse: 3, SourceVerse: 4},
	}
	for _, version := range []string{"web", "webc", "bsb"} {
		t.Run(version, func(t *testing.T) {
			st, selected := selectionExcerptState(version, "Genesis")
			list := buildCrossRefList(st, selected, refs, nil, lightPalette, func(crossRef) {})
			requireUnletteredSelectionExcerpt(t, list, selected)
			groups := crossRefSourceRows(list.Objects)
			if list.TSKRows != 3 || !list.Sections.Treasury || list.Sections.Publisher || list.Sections.Parallels || len(groups) != 3 {
				t.Fatalf("the excerpt changed the Treasury-only grouping: rows=%d groups=%d sections=%+v", list.TSKRows, len(groups), list.Sections)
			}
			for i, group := range groups {
				if group.id != fmt.Sprintf("treasury:%d", i+2) || group.key == nil || group.key.Text != fmt.Sprint(i+2) || group.open {
					t.Fatalf("group %d lost its actual source verse or closed state", i)
				}
				group.setOpen(true)
			}
			if got := originListCardLabels(list.Objects); !slices.Equal(got, []string{"Exodus 1:1", "Exodus 1:2", "Exodus 1:3"}) {
				t.Fatalf("Treasury destinations changed: %v", got)
			}
			if out := os.Getenv("BIBLETEXT_SELECTION_EXCERPT_RENDER_DIR"); out != "" {
				if err := os.MkdirAll(out, 0o755); err != nil {
					t.Fatal(err)
				}
				withCrossRefIndex(t, "Gen.1.2\tExod.1.1\t9\nGen.1.3\tExod.1.2\t8\nGen.1.4\tExod.1.3\t7\n")
				oldRun, oldLoad := crossRefsRun, crossRefsLoad
				crossRefsRun = func(fn func()) { fn() }
				crossRefsLoad = func() error { return nil }
				t.Cleanup(func() { crossRefsRun, crossRefsLoad = oldRun, oldLoad })
				w := test.NewWindow(widget.NewLabel("Synthetic reading page"))
				t.Cleanup(w.Close)
				w.Resize(fyne.NewSize(320, 568))
				st.theme, st.window = th, w
				showCrossRefs(st, "", selSpan{lo: 1, hi: 4})
				top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
				if !ok {
					t.Fatal("cross-reference modal did not open")
				}
				w.Canvas().Capture()
				_, box, _ := panelListParts(top)
				if box == nil {
					t.Fatal("modal has no composed list")
				}
				requireUnletteredSelectionExcerpt(t, crossRefList{Objects: box.Objects, Passage: true}, selected)
				if issues := crossRefListMisfits(box.Objects, box.Size().Width); len(issues) != 0 {
					t.Fatal(strings.Join(issues, "; "))
				}
				writeRenderPNG(t, filepath.Join(out, "selected-1-4-treasury-2-4-"+version+"-320.png"), w.Canvas().Capture())
			}
		})
	}
}

func TestCrossRefSelectionExcerptKeepsGospelEventIdentity(t *testing.T) {
	appThemeForTest(t)
	st, selected := selectionExcerptState("web", "Matthew")
	refs := []crossRef{
		{Book: "Luke", Chapter: 1, Verse: 1, EndV: 2, Parallel: true,
			Title: "A synthetic journey", SourceGroup: "gospel:fixture", SourceLabel: "Matthew 1:2–4"},
		{Book: "Luke", Chapter: 1, Verse: 3, Parallel: true,
			Title: "A synthetic journey", SourceGroup: "gospel:fixture", SourceLabel: "Matthew 1:2–4"},
	}
	list := buildCrossRefList(st, selected, refs, nil, lightPalette, func(crossRef) {})
	requireUnletteredSelectionExcerpt(t, list, selected)
	groups := crossRefSourceRows(list.Objects)
	if len(groups) != 1 || groups[0].id != "gospel:fixture" || groups[0].key != nil || !groups[0].open ||
		!list.Sections.Parallels || list.Sections.Treasury || list.Sections.Publisher {
		t.Fatal("the excerpt changed the Gospel event group")
	}
	if got := joinedTexts(list.Objects); !strings.Contains(got, "A synthetic journey") || !strings.Contains(got, "Matthew 1:2–4") {
		t.Fatal("Gospel title or actual source span is missing")
	}
	if got := originListCardLabels(list.Objects); !slices.Equal(got, []string{"Luke 1:1–2", "Luke 1:3"}) {
		t.Fatalf("Gospel destination changed: %v", got)
	}
}

func TestCrossRefSelectionExcerptDoesNotDuplicateBSBHeadings(t *testing.T) {
	appThemeForTest(t)
	st, selected := selectionExcerptState("bsb", "Genesis")
	const heading = "A synthetic heading before the third verse"
	st.Bible.Headings = map[string]map[int][]Heading{"Genesis": {1: {{Text: heading, Style: "heading", BeforeVerse: 3,
		Footnotes: []Footnote{{Text: "Exodus 1:1; 1:2", Kind: footnoteKindCrossref, Caller: "-",
			Refs: []NoteRef{{"EXO.1.1", 0, 10}, {"EXO.1.2", 12, 15}}}},
	}}}}
	list := buildCrossRefList(st, selected, nil, nil, lightPalette, func(crossRef) {})
	requireUnletteredSelectionExcerpt(t, list, selected)
	groups := crossRefSourceRows(list.Objects)
	if len(groups) != 1 || groups[0].key != nil || !groups[0].open || list.PublisherNotes != 1 || list.PublisherCards != 2 ||
		!list.Sections.Publisher || list.Sections.Treasury || list.Sections.Parallels {
		t.Fatal("the excerpt changed the BSB heading group")
	}
	if got := joinedTexts(list.Objects); strings.Count(got, heading) != 1 || !strings.Contains(got, "Heading before verse 3") {
		t.Fatal("BSB heading was duplicated or lost its source position")
	}
	if got := originListCardLabels(list.Objects); !slices.Equal(got, []string{"Exodus 1:1", "Exodus 1:2"}) {
		t.Fatalf("BSB heading destinations changed: %v", got)
	}
}

func TestCrossRefSelectionExcerptLeavesEmptyAndUnavailableListsEmpty(t *testing.T) {
	appThemeForTest(t)
	st, selected := selectionExcerptState("web", "Genesis")
	for _, state := range []crossRefTreasuryState{{}, {loading: true}, {err: errOfflineForTest}} {
		list := buildCrossRefPanelList(st, selected, nil, state, lightPalette, func(crossRef) {})
		if list.Passage || len(list.Objects) != 0 {
			t.Fatal("an empty, loading or unavailable list gained a passage instead of retaining its message state")
		}
	}
}
