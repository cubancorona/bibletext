package bibletext

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func originListTreasuryFixture(t *testing.T) *AppState {
	t.Helper()
	withCrossRefIndex(t, "Gen.1.1\tExod.1.1\t9\n"+
		"Gen.1.1\tExod.1.2\t5\n"+
		"Gen.1.3\tExod.1.1\t7\n")
	return &AppState{
		Bible:       xrefBible(map[string]map[int]int{"Genesis": {1: 3}, "Exodus": {1: 3}}),
		CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web",
	}
}

// originListCards walks mounted destination cards, excluding disclosure
// headers and closed bodies even after their children have been created.
func originListCards(objects []fyne.CanvasObject) []*tapCard {
	var cards []*tapCard
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefSourceGroup:
			if x.open {
				walk(x.body)
			}
		case *crossRefNoteRow:
			if x.open {
				walk(x.body)
			}
		case *crossRefGroupRow:
			if x.open {
				walk(x.members)
			}
		case *tapCard:
			if cardReference(x) != "" {
				cards = append(cards, x)
			}
		case *fyne.Container:
			for _, child := range x.Objects {
				walk(child)
			}
		}
	}
	for _, object := range objects {
		walk(object)
	}
	return cards
}

func originListCardLabels(objects []fyne.CanvasObject) []string {
	var labels []string
	for _, card := range originListCards(objects) {
		labels = append(labels, cardReference(card))
	}
	return labels
}

func TestOriginListTreasuryGroupsAreIndependentAndInitiallyCollapsed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := originListTreasuryFixture(t)
	selected := versesBetween(state.Bible, "Genesis", 1, 1, 3)
	refs := crossRefsForSelection(state, "", selSpan{lo: 1, hi: 3})
	lst := buildCrossRefList(state, selected, refs, nil, lightPalette, func(crossRef) {})
	groups := crossRefSourceRows(lst.Objects)
	if len(groups) != 2 || lst.TSKRows != 3 || !lst.Sections.Treasury {
		t.Fatalf("got %d groups and %d Treasury rows, want two origins with all three relationships", len(groups), lst.TSKRows)
	}
	for i, want := range []string{"treasury:1", "treasury:3"} {
		g := groups[i]
		if g.id != want || g.open || len(g.body.Objects) != 0 || g.body.Visible() {
			t.Errorf("group %d: id=%q open=%v mounted=%d visible=%v; want %q collapsed and unmounted", i, g.id, g.open, len(g.body.Objects), g.body.Visible(), want)
		}
	}
	if got := originListCardLabels(lst.Objects); len(got) != 0 {
		t.Fatalf("collapsed groups exposed cards: %v", got)
	}
	groups[0].toggle()
	if !groups[0].open || groups[1].open || len(groups[1].body.Objects) != 0 {
		t.Fatal("opening the first origin also opened or mounted the second")
	}
	if got := originListCardLabels(groups[0].body.Objects); !slices.Equal(got, []string{"Exodus 1:1", "Exodus 1:2"}) {
		t.Errorf("first origin lost its ranked references: %v", got)
	}
	groups[1].toggle()
	groups[0].toggle()
	if groups[0].open || !groups[1].open {
		t.Fatal("closing the first origin changed the second origin's state")
	}
	if got := originListCardLabels(lst.Objects); !slices.Equal(got, []string{"Exodus 1:1"}) {
		t.Errorf("the shared destination did not remain in the second origin: %v", got)
	}
	if sources, notice := crossRefFooterCredit(state, lst.Sections); sources != treasuryCredit || notice != "" {
		t.Errorf("collapsed-source credit = %q / %q, want Treasury credit only", sources, notice)
	}
}

func TestOriginListSoleTreasuryContributorOpensAndSingleVerseStaysFlat(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	state := originListTreasuryFixture(t)
	for _, tc := range []struct {
		name       string
		lo, hi     int
		wantGroups int
		wantCards  []string
	}{
		{"one contributor in a range", 2, 3, 1, []string{"Exodus 1:1"}},
		{"one selected verse", 1, 1, 0, []string{"Exodus 1:1", "Exodus 1:2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := versesBetween(state.Bible, "Genesis", 1, tc.lo, tc.hi)
			refs := crossRefsForSelection(state, "", selSpan{lo: tc.lo, hi: tc.hi})
			lst := buildCrossRefList(state, selected, refs, nil, lightPalette, func(crossRef) {})
			groups := crossRefSourceRows(lst.Objects)
			if len(groups) != tc.wantGroups {
				t.Fatalf("got %d groups, want %d", len(groups), tc.wantGroups)
			}
			if len(groups) == 1 && (!groups[0].open || groups[0].id != "treasury:3") {
				t.Errorf("sole contributor = %q, open=%v", groups[0].id, groups[0].open)
			}
			if got := originListCardLabels(lst.Objects); !slices.Equal(got, tc.wantCards) {
				t.Errorf("visible destinations = %v, want %v", got, tc.wantCards)
			}
		})
	}
}

func TestOriginListRetainsOneDestinationAcrossAllContributingSources(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	withCrossRefIndex(t, "Matt.5.43\tLuke.6.27-Luke.6.28\t9\n"+
		"Matt.5.44\tLuke.6.27-Luke.6.28\t8\n")
	bd := xrefBible(map[string]map[int]int{"Matthew": {5: 48}, "Luke": {6: 49}})
	bd.Verses["Matthew"][5][42].Footnotes = []Footnote{
		{Text: "Luke 6:27–28", Anchor: 5, Kind: footnoteKindCrossref, Caller: "+"},
	}
	state := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web"}
	selected := versesBetween(bd, "Matthew", 5, 43, 44)
	refs := crossRefsForSelection(state, "", selSpan{lo: 43, hi: 44})
	parallel, treasury := 0, 0
	for _, ref := range refs {
		if ref.label() != "Luke 6:27–28" {
			continue
		}
		if ref.Parallel {
			parallel++
		} else {
			treasury++
		}
	}
	if parallel != 1 || treasury != 2 {
		t.Fatalf("control: source lookup returned %d Gospel and %d Treasury relationships, want 1 and 2", parallel, treasury)
	}
	lst := buildCrossRefList(state, selected, refs, nil, lightPalette, func(crossRef) {})
	for _, g := range crossRefSourceRows(lst.Objects) {
		g.setOpen(true)
	}
	count := 0
	for _, label := range originListCardLabels(lst.Objects) {
		if label == "Luke 6:27–28" {
			count++
		}
	}
	if count != 4 || lst.PublisherCards != 1 || lst.TSKRows != 2 {
		t.Errorf("shared target appears %d times, with publisher=%d Treasury=%d; want four relationships", count, lst.PublisherCards, lst.TSKRows)
	}
	if sources, _ := crossRefFooterCredit(state, lst.Sections); sources != treasuryCredit+"\n"+parallelsCredit {
		t.Errorf("mixed-source credits = %q", sources)
	}
}

func TestOriginListBSBUsesActualHeadingsWithoutLetteredPassage(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 3}, "Exodus": {1: 3}})
	bd.Headings = map[string]map[int][]Heading{"Genesis": {1: {
		{Text: "A fixture journey", Style: "heading", BeforeVerse: 1, Footnotes: []Footnote{
			{Text: "Exodus 1:1; 1:2", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"EXO.1.1", 0, 10}, {"EXO.1.2", 12, 15}}},
		}},
		{Text: "A fixture arrival", Style: "heading", BeforeVerse: 3, Footnotes: []Footnote{
			{Text: "Exodus 1:3", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"EXO.1.3", 0, 10}}},
		}},
	}}}
	state := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "bsb"}
	selected := versesBetween(bd, "Genesis", 1, 1, 3)
	lst := buildCrossRefList(state, selected, nil, nil, lightPalette, func(crossRef) {})
	if !lst.Passage || !lst.Sections.Publisher || lst.PublisherNotes != 2 || lst.PublisherCards != 3 {
		t.Fatalf("heading-only BSB list = passage %v, publisher %v, notes %d, cards %d", lst.Passage, lst.Sections.Publisher, lst.PublisherNotes, lst.PublisherCards)
	}
	requirePlainCrossRefExcerpt(t, passageIn(lst.Objects), selected)
	groups := crossRefSourceRows(lst.Objects)
	if len(groups) != 1 {
		t.Fatalf("got %d disclosure groups, want only the two-target heading", len(groups))
	}
	g := groups[0]
	if g.key != nil || len(g.labels) < 2 || g.labels[0].Text != "A fixture journey" || g.labels[1].Text != "Heading before verse 1" {
		t.Fatal("the group did not retain the actual heading and its before-verse position without an arbitrary key")
	}
	g.setOpen(true)
	if got := joinedTexts(lst.Objects); !strings.Contains(got, "A fixture arrival") || !strings.Contains(got, "Heading before verse 3") {
		t.Errorf("direct heading note lost its origin: %q", got)
	}
	if got := originListCardLabels(lst.Objects); !slices.Equal(got, []string{"Exodus 1:1", "Exodus 1:2", "Exodus 1:3"}) {
		t.Errorf("heading destinations = %v", got)
	}
	for _, card := range originListCards(lst.Objects) {
		if letter := cardLetter(card); letter != "" {
			t.Errorf("heading card acquired arbitrary letter %q", letter)
		}
	}
}

func TestOriginListKeepsWEBFamilyExactLettersAndDirectSingleTargets(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, version := range []string{"web", "webc"} {
		t.Run(version, func(t *testing.T) {
			bd := xrefBible(map[string]map[int]int{"Genesis": {1: 2}, "Exodus": {1: 3}})
			for i := range bd.Verses["Genesis"][1] {
				bd.Verses["Genesis"][1][i].Text = "A fixture phrase followed by another phrase."
			}
			bd.Verses["Genesis"][1][0].Footnotes = []Footnote{
				{Text: "Exodus 1:1", Anchor: len([]rune("A fixture phrase")), Kind: footnoteKindCrossref, Caller: "+", Refs: []NoteRef{{"EXO.1.1", 0, 10}}},
			}
			bd.Verses["Genesis"][1][1].Footnotes = []Footnote{
				{Text: "Exodus 1:2; 1:3", Anchor: len([]rune(bd.Verses["Genesis"][1][1].Text)), Kind: footnoteKindCrossref, Caller: "+", Refs: []NoteRef{{"EXO.1.2", 0, 10}, {"EXO.1.3", 12, 15}}},
			}
			state := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: version}
			selected := versesBetween(bd, "Genesis", 1, 1, 2)
			notes := publisherCrossRefsFor(bd, "Genesis", 1, selected)
			units := publisherPassageUnits(state, "Genesis", 1, selected, notes)
			if len(units) != 2 {
				t.Fatalf("got %d passage units, want two numbered verses", len(units))
			}
			for i, u := range units {
				want := passageMark{At: selected[i].Footnotes[0].Anchor, Letter: noteLetter(i)}
				if u.Number != i+1 || u.text() != selected[i].Text || len(u.Marks) != 1 || u.Marks[0] != want {
					t.Errorf("verse %d lost its exact text, number or source mark: %+v", i+1, u)
				}
			}
			lst := buildCrossRefList(state, selected, nil, nil, lightPalette, func(crossRef) {})
			if !lst.Passage || passageIn(lst.Objects) == nil || lst.PublisherNotes != 2 || lst.PublisherCards != 3 || len(crossRefSourceRows(lst.Objects)) != 0 {
				t.Fatalf("lettered publisher list was replaced by source groups: %+v", lst)
			}
			rows := noteRowsIn(lst.Objects)
			if len(rows) != 1 || rows[0].note.Index != 1 {
				t.Fatalf("got %d note disclosures, want only the multi-target b note", len(rows))
			}
			direct := false
			for _, card := range originListCards(lst.Objects) {
				if cardReference(card) == "Exodus 1:1" && cardLetter(card) == "a" {
					direct = true
				}
			}
			if !direct {
				t.Error("the single-target a note is no longer a directly available card")
			}
		})
	}
}

func TestOriginListGospelContentAndCreditsSurviveTreasuryLoadingAndFailure(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := xrefBible(map[string]map[int]int{"Matthew": {5: 48}, "Luke": {6: 49}})
	state := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web"}
	selected := versesBetween(bd, "Matthew", 5, 43, 44)
	local := localCrossRefsForSelection(state, "", selSpan{lo: 43, hi: 44})
	if len(local) == 0 {
		t.Fatal("control: the local Gospel fixture has no relationships")
	}
	bar := widget.NewProgressBarInfinite()
	defer bar.Stop()
	for _, tc := range []struct {
		name  string
		state crossRefTreasuryState
		want  string
	}{
		{"loading", crossRefTreasuryState{loading: true, bar: bar}, "Finding related passages…"},
		{"failure", crossRefTreasuryState{err: errors.New("synthetic unavailable source"), onRetry: func() {}}, crossRefOfflineLine},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lst := buildCrossRefPanelList(state, selected, local, tc.state, lightPalette, func(crossRef) {})
			if lst.Sections.Publisher || lst.Sections.Treasury || !lst.Sections.Parallels || lst.Parallels == 0 {
				t.Fatalf("local-only source flags changed: %+v", lst.Sections)
			}
			if got := sectionLabelsIn(lst.Objects); !slices.Equal(got, []string{parallelsLabel, treasuryLabel}) {
				t.Errorf("section labels = %v", got)
			}
			if got := joinedTexts(lst.Objects); !strings.Contains(got, tc.want) {
				t.Errorf("missing scoped Treasury status %q in %q", tc.want, got)
			}
			for _, g := range crossRefSourceRows(lst.Objects) {
				g.setOpen(true)
			}
			if len(originListCards(lst.Objects)) == 0 {
				t.Error("Treasury status replaced the available Gospel destinations")
			}
			if sources, notice := crossRefFooterCredit(state, lst.Sections); sources != parallelsCredit || notice != "" {
				t.Errorf("credits = %q / %q, want only the displayed Gospel source", sources, notice)
			}
		})
	}
}
