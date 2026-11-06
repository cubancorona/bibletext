package bibletext

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The panel's composition. With the publisher's run off, the list is the
// badged parallels then the Treasury rows and nothing of the publisher's;
// with it on, one list of one card under section labels: the parallels
// without their pill, the publisher's citations as cards with the words that
// did not become one as captions, and the Treasury open beneath, less the
// passages the publisher's cards already carry. Every check carries a control.

// withPublisherCrossRefs flips the registry flag for one edition for the
// test's duration — what the nkjvxrefs build tag does for the NKJV at init.
func withPublisherCrossRefs(t *testing.T, id string, on bool) {
	t.Helper()
	for i := range registeredVersions {
		if registeredVersions[i].ID == id {
			was := registeredVersions[i].PublisherCrossRefs
			registeredVersions[i].PublisherCrossRefs = on
			t.Cleanup(func() { registeredVersions[i].PublisherCrossRefs = was })
			return
		}
	}
	t.Fatalf("no registered version %q", id)
}

// textsIn collects every visible string in a tree: canvas texts, labels,
// rich-text segments and what a card holds.
func textsIn(o fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			out = append(out, x.Text)
		case *widget.Label:
			out = append(out, x.Text)
		case *widget.RichText:
			var b strings.Builder
			for _, s := range x.Segments {
				b.WriteString(s.Textual())
			}
			out = append(out, b.String())
		case *tapCard:
			walk(x.content)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	walk(o)
	return out
}

func joinedTexts(objs []fyne.CanvasObject) string {
	var b strings.Builder
	for _, o := range objs {
		for _, s := range textsIn(o) {
			b.WriteString(s)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// cardsIn is every card in objs, in list order.
func cardsIn(objs []fyne.CanvasObject) []*tapCard {
	var out []*tapCard
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *tapCard:
			out = append(out, x)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// cardLabelsIn is the reference line of every card in objs, in list order:
// the first text a card holds.
func cardLabelsIn(objs []fyne.CanvasObject) []string {
	var out []string
	for _, c := range cardsIn(objs) {
		if ts := textsIn(c.content); len(ts) > 0 {
			out = append(out, ts[0])
		}
	}
	return out
}

// sectionLabelsIn is every section label in objs, in list order: the bold
// monospace texts, which nothing else in the list is.
func sectionLabelsIn(objs []fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *canvas.Text:
			if x.TextStyle.Monospace && x.TextStyle.Bold {
				out = append(out, x.Text)
			}
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// bannedIn names every object in objs of a kind the list may not hold: the
// widgets whose width cannot wrap, which widened the scroll's content past
// the panel (crossref_list.go), and the override that clears the font cache.
func bannedIn(objs []fyne.CanvasObject) []string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.Accordion:
			out = append(out, "Accordion")
		case *widget.Button:
			out = append(out, "Button")
		case *widget.Hyperlink:
			out = append(out, "Hyperlink")
		case *container.ThemeOverride:
			out = append(out, "ThemeOverride")
		case *widget.RichText:
			for _, s := range x.Segments {
				if _, ok := s.(*widget.HyperlinkSegment); ok {
					out = append(out, "HyperlinkSegment")
				}
			}
		case *tapCard:
			walk(x.content)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return out
}

// licenseNKJVForTest makes the NKJV's licensed source available, as the
// integration tests do, so the edition counts as licensed and serving text.
func licenseNKJVForTest(t *testing.T) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")
}

func listFixtureState() (*AppState, []Verse) {
	bd := publisherFixture()
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3, CurrentVersion: "nkjv"}
	return st, bd.GetChapter("John", 3)
}

// Treasury rows and a parallel row as crossRefsForSelection would hand them
// over: parallels first, then the ranked Treasury rows.
func listFixtureRefs() []crossRef {
	return []crossRef{
		{Book: "Matthew", Chapter: 3, Verse: 1, Parallel: true, Title: "A synopsis probe"},
		{Book: "John", Chapter: 7, Verse: 50, Votes: 40},
		{Book: "Galatians", Chapter: 6, Verse: 15, Votes: 12},
	}
}

func joined(ss []string) string { return strings.Join(ss, "|") }

// hasLine reports whether some text in objs is exactly s.
func hasLine(objs []fyne.CanvasObject, s string) bool {
	for _, o := range objs {
		for _, line := range textsIn(o) {
			if line == s {
				return true
			}
		}
	}
	return false
}

func TestCrossRefListWithoutThePublisherIsWhatItAlwaysWas(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", false)
	st, sel := listFixtureState()

	lst := buildCrossRefList(st, sel, listFixtureRefs(), nil, lightPalette, func(crossRef) {})
	if lst.PublisherCards != 0 || lst.PublisherLines != 0 || lst.Sections.Publisher {
		t.Errorf("the publisher's run appeared with the flag off: %+v", lst)
	}
	if lst.Parallels != 1 || lst.TSKRows != 2 || len(lst.Objects) != 3 {
		t.Errorf("list = %d parallels, %d Treasury rows, %d objects; want 1, 2, 3", lst.Parallels, lst.TSKRows, len(lst.Objects))
	}
	if got := cardLabelsIn(lst.Objects); joined(got) != "Matthew 3:1|John 7:50|Galatians 6:15" {
		t.Errorf("cards = %q", got)
	}
	if labels := sectionLabelsIn(lst.Objects); len(labels) != 0 {
		t.Errorf("section labels in the plain list: %q", labels)
	}
	all := joinedTexts(lst.Objects)
	if !strings.Contains(all, "PARALLEL") {
		t.Error("the parallel row lost its pill, the one thing that says where it is from in a list with no labels")
	}
	for _, banned := range []string{"REFERENCES", "Thomas Nelson", "John 7:50; 19:39", "(John 1:13", "No NKJV"} {
		if strings.Contains(all, banned) {
			t.Errorf("a publisher's note, label or credit reached the plain list (%q):\n%s", banned, all)
		}
	}
	// The footer names the Treasury in the words it has always had, and the
	// synopsis beside it while a parallel row is on screen; the edition is
	// licensed, so the footer carries its notice under the previews of its
	// verse text.
	sources, notice := crossRefFooterCredit(st, lst.Sections)
	if sources != "Cross-references: OpenBible.info (CC-BY) · Gospel parallels: synopsis" {
		t.Errorf("footer sources = %q", sources)
	}
	if !strings.Contains(notice, "Thomas Nelson") {
		t.Errorf("a licensed edition's previews are shown without its notice: %q", notice)
	}
	// Without a parallel row the synopsis is not named: the one repair the
	// plain list takes, since a synopsis was credited on every Psalm.
	lst = buildCrossRefList(st, sel, listFixtureRefs()[1:], nil, lightPalette, func(crossRef) {})
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != tskCredit {
		t.Errorf("footer sources without a parallel row = %q, want the Treasury alone", sources)
	}
	// CONTROL: a public-domain edition carries no notice.
	st.CurrentVersion = "web"
	if _, notice := crossRefFooterCredit(st, lst.Sections); notice != "" {
		t.Errorf("a public-domain edition grew a notice: %q", notice)
	}
}

func TestCrossRefListWithThePublisherIsOneListOfCards(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := listFixtureState()
	var followed []crossRef
	lst := buildCrossRefList(st, sel, listFixtureRefs(), nil, lightPalette, func(c crossRef) { followed = append(followed, c) })

	if lst.Parallels != 1 || lst.PublisherCards != 1 || lst.PublisherLines != 4 || lst.TSKRows != 1 {
		t.Fatalf("list = %+v; want one parallel, one publisher card, four caption lines and one Treasury card", lst)
	}
	if !lst.Sections.Parallels || !lst.Sections.Publisher || !lst.Sections.Treasury {
		t.Errorf("sections = %+v; want all three on screen", lst.Sections)
	}
	// One heading idiom: the three runs' labels, the publisher's keyed per
	// verse with notes since the selection spans three verses.
	want := "GOSPEL PARALLELS|NKJV REFERENCES · VERSE 1|NKJV REFERENCES · VERSE 2|NKJV REFERENCES · VERSE 3|TREASURY OF SCRIPTURE KNOWLEDGE"
	if got := sectionLabelsIn(lst.Objects); joined(got) != want {
		t.Errorf("section labels = %q\nwant %q", got, want)
	}
	// One card idiom: the parallel, the publisher's card, then the Treasury's.
	// The Treasury's John 7:50 is the publisher's card's passage and goes;
	// its Galatians 6:15 stays, since the tagged span inside the group did
	// not become a card.
	if got := cardLabelsIn(lst.Objects); joined(got) != "Matthew 3:1|John 7:50|Galatians 6:15" {
		t.Errorf("cards = %q", got)
	}
	all := joinedTexts(lst.Objects)
	// The words that did not become cards stand verbatim: the untagged note,
	// the continuation the text lacks, the group whole, the self-range.
	for _, want := range []string{"(Acts 10:38)", "19:39", "(John 1:13; Gal. 6:15; 1 John 3:9)", "John 3:1–21", "A synopsis probe"} {
		if !strings.Contains(all, want) {
			t.Errorf("the list lacks %q:\n%s", want, all)
		}
	}
	// No pill under the parallels' label (the pill is a text of its own; the
	// label's word is not it), no legend, no notice or credit in the list, no
	// note re-joined as one paragraph.
	if hasLine(lst.Objects, "PARALLEL") {
		t.Errorf("the parallel card wears its pill under the GOSPEL PARALLELS label:\n%s", all)
	}
	for _, banned := range []string{"compare", "Thomas Nelson", "OpenBible", "cross references", "John 7:50; 19:39"} {
		if strings.Contains(all, banned) {
			t.Errorf("%q is in the list:\n%s", banned, all)
		}
	}
	if b := bannedIn(lst.Objects); len(b) != 0 {
		t.Errorf("the list holds %v", b)
	}
	// A card's tap follows its passage.
	cards := cardsIn(lst.Objects)
	cards[1].Tapped(nil)
	if len(followed) != 1 || followed[0] != (crossRef{Book: "John", Chapter: 7, Verse: 50}) {
		t.Errorf("tapping the publisher's card followed %+v", followed)
	}
	// The note's remainder sits inside the card's block, before its rule.
	var block *fyne.Container
	for _, o := range lst.Objects {
		if c, ok := o.(*fyne.Container); ok && len(c.Objects) > 0 && c.Objects[0] == cards[1] {
			block = c
		}
	}
	if block == nil || len(block.Objects) != 3 {
		t.Fatalf("the publisher's card does not stand in a block of card, caption and rule: %+v", block)
	}
	if _, ok := block.Objects[2].(*widget.Separator); !ok || !strings.Contains(strings.Join(textsIn(block.Objects[1]), ""), "19:39") {
		t.Errorf("the card's block is not card, caption, rule: %v / %q", block.Objects[2], textsIn(block.Objects[1]))
	}
	// The footer names both sets, the Treasury in its label's words, and the
	// notice once.
	sources, notice := crossRefFooterCredit(st, lst.Sections)
	if sources != treasuryCredit+" · "+parallelsCredit {
		t.Errorf("footer sources = %q", sources)
	}
	if !strings.Contains(notice, "Thomas Nelson") {
		t.Errorf("the footer carries no notice: %q", notice)
	}
}

// The run's label carries a verse key only where the key informs.
func TestCrossRefListKeysTheRunOnlyWhereItInforms(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := listFixtureState()
	nop := func(crossRef) {}

	// One verse: no key anywhere.
	lst := buildCrossRefList(st, sel[1:2], nil, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES" {
		t.Errorf("one verse: labels = %q", got)
	}
	// Two verses with notes: one label per verse, keyed.
	lst = buildCrossRefList(st, sel[:2], nil, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES · VERSE 1|NKJV REFERENCES · VERSE 2" {
		t.Errorf("two verses: labels = %q", got)
	}
	// The title's notes are keyed TITLE, and a verse with no note is silently
	// absent: Psalm 3's title has a note, its verses 1 and 2 have none.
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	ps := st.Bible.GetChapter("Psalms", 3)
	for _, verses := range [][]Verse{ps, ps[:1]} {
		lst = buildCrossRefList(st, verses, nil, nil, lightPalette, nop)
		if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES · TITLE" {
			t.Errorf("%d verses of Psalm 3: labels = %q", len(verses), got)
		}
	}
	// CONTROL: a verse without a note and no title note: the bare label over
	// the empty line (a Treasury row keeps the list from being the message).
	lst = buildCrossRefList(st, ps[1:], []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 3}}, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("Psalm 3:2: labels = %q", got)
	}
}

// A title note's card, and the Treasury rows the publisher's cards already
// carry: dropped by containment, in both directions, and the section goes
// when nothing of it is left.
func TestCrossRefListDropsTheTreasuryRowsThePublisherCarries(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	st.Bible.Verses["2 Samuel"] = map[int][]Verse{
		15: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 15, Verse: 13, Text: "A fixture messenger came."}},
		16: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 16, Verse: 1, Text: "Over the fixture hill."}},
	}
	ps := st.Bible.GetChapter("Psalms", 3)
	nop := func(crossRef) {}
	refs := []crossRef{
		{Book: "2 Samuel", Chapter: 15, Verse: 1, EndV: 18, Votes: 20}, // contains the card's start: goes
		{Book: "2 Samuel", Chapter: 15, Verse: 14, Votes: 9},           // inside the card's passage: goes
		{Book: "2 Samuel", Chapter: 16, Verse: 1, Votes: 5},            // another passage: stays
		{Book: "John", Chapter: 7, Verse: 50, Votes: 3},
	}
	lst := buildCrossRefList(st, ps[:1], refs, nil, lightPalette, nop)
	if got := cardLabelsIn(lst.Objects); joined(got) != "2 Samuel 15:13–17|2 Samuel 16:1|John 7:50" {
		t.Errorf("cards = %q", got)
	}
	if lst.PublisherCards != 1 || lst.TSKRows != 2 || !lst.Sections.Treasury {
		t.Errorf("list = %+v", lst)
	}
	// CONTROL: with the flag off every Treasury row shows.
	withPublisherCrossRefs(t, "nkjv", false)
	if lst := buildCrossRefList(st, ps[:1], refs, nil, lightPalette, nop); lst.TSKRows != 4 {
		t.Errorf("flag off: %d Treasury rows, want all 4", lst.TSKRows)
	}
	withPublisherCrossRefs(t, "nkjv", true)
	// Every Treasury row carried: no Treasury section, no credit.
	lst = buildCrossRefList(st, ps[:1], refs[:2], nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES · TITLE" {
		t.Errorf("labels with the Treasury wholly carried = %q", got)
	}
	if lst.TSKRows != 0 || lst.Sections.Treasury {
		t.Errorf("a Treasury with nothing left is still on screen: %+v", lst)
	}
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != "" {
		t.Errorf("the footer credits a set with nothing on screen: %q", sources)
	}
}

func TestCrossRefListEmptyPublisherRunShowsTheLineOverTheTreasury(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, john3 := listFixtureState()
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	ps := st.Bible.GetChapter("Psalms", 3)
	nop := func(crossRef) {}

	// Psalm 3:2 has no note: the label, one line, the Treasury open beneath.
	lst := buildCrossRefList(st, ps[1:], []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 3}}, nil, lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("labels = %q", got)
	}
	if lst.PublisherCards != 0 || lst.PublisherLines != 1 || lst.TSKRows != 1 {
		t.Errorf("list = %+v", lst)
	}
	all := joinedTexts(lst.Objects)
	if !strings.Contains(all, "No NKJV cross-references for this selection.") || strings.Contains(all, "Thomas Nelson") {
		t.Errorf("the empty run is not one scoped line:\n%s", all)
	}
	// Nothing at all for the selection is the panel's message state, not a
	// run of one line over an empty Treasury.
	if lst := buildCrossRefList(st, ps[1:], nil, nil, lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("nothing to list, yet %d objects: %q", len(lst.Objects), joinedTexts(lst.Objects))
	}
	// Offline with nothing of the publisher's: the message state, which the
	// panel turns into its connection error.
	if lst := buildCrossRefList(st, ps[1:], nil, errors.New("offline"), lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("offline and no notes, yet %d objects: %q", len(lst.Objects), joinedTexts(lst.Objects))
	}
	// Offline with the publisher's notes: the run, then the Treasury's label
	// and one line, no verb, no credit.
	st.CurrentBook, st.CurrentChapter = "John", 3
	lst = buildCrossRefList(st, john3[1:2], nil, errors.New("offline"), lightPalette, nop)
	if got := sectionLabelsIn(lst.Objects); joined(got) != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("offline labels = %q", got)
	}
	all = joinedTexts(lst.Objects)
	if !strings.Contains(all, "Couldn't load cross-references.") || strings.Contains(all, "try again") {
		t.Errorf("an offline Treasury is not one line in its place:\n%s", all)
	}
	if lst.Sections.Treasury || lst.PublisherCards != 1 {
		t.Errorf("offline list = %+v", lst)
	}
	if sources, _ := crossRefFooterCredit(st, lst.Sections); sources != "" {
		t.Errorf("the footer credits a Treasury that did not load: %q", sources)
	}
	if b := bannedIn(lst.Objects); len(b) != 0 {
		t.Errorf("the offline list holds %v", b)
	}
	// CONTROL: with the flag off the same failure and no rows is an empty
	// list, which the panel turns into its message.
	withPublisherCrossRefs(t, "nkjv", false)
	if lst := buildCrossRefList(st, john3[1:2], nil, errors.New("offline"), lightPalette, nop); len(lst.Objects) != 0 {
		t.Errorf("flag off, nothing loaded, yet %d objects", len(lst.Objects))
	}
}

// The footer's sources line names the sets with rows on screen, and no other.
func TestTheFooterNamesOnlyTheSetsOnScreen(t *testing.T) {
	licenseNKJVForTest(t)
	st, _ := listFixtureState()
	for _, tc := range []struct {
		flag     bool
		sections crossRefSections
		want     string
	}{
		{false, crossRefSections{Treasury: true, Parallels: true}, "Cross-references: OpenBible.info (CC-BY) · Gospel parallels: synopsis"},
		{false, crossRefSections{Treasury: true}, "Cross-references: OpenBible.info (CC-BY)"},
		{false, crossRefSections{Parallels: true}, "Gospel parallels: synopsis"},
		{false, crossRefSections{}, ""},
		{true, crossRefSections{Treasury: true, Parallels: true, Publisher: true}, "Treasury of Scripture Knowledge: OpenBible.info (CC-BY) · Gospel parallels: synopsis"},
		{true, crossRefSections{Treasury: true, Publisher: true}, "Treasury of Scripture Knowledge: OpenBible.info (CC-BY)"},
		{true, crossRefSections{Publisher: true}, ""},
	} {
		withPublisherCrossRefs(t, "nkjv", tc.flag)
		if sources, _ := crossRefFooterCredit(st, tc.sections); sources != tc.want {
			t.Errorf("flag %v, %+v: sources = %q, want %q", tc.flag, tc.sections, sources, tc.want)
		}
	}
}

// A title card previews the chapter's superscription and wears the Title tag;
// the same passage as an ordinary card previews the verse.
func TestATitleCardPreviewsTheSuperscription(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := &AppState{Bible: publisherFixture(), CurrentBook: "1 Samuel", CurrentChapter: 19, CurrentVersion: "nkjv"}
	ref := crossRef{Book: "Psalms", Chapter: 59, Verse: 1}
	nop := func(crossRef) {}
	title := textsIn(crossRefRow(st, crossRefCard{Ref: ref, Label: "Psalms 59", Tag: "Title", Title: true}, lightPalette, nop))
	got := strings.Join(title, "\n")
	if len(title) < 3 || title[0] != "Psalms 59" || title[1] != "Title" || !strings.Contains(got, "A fixture song of David") || strings.Contains(got, "Rescue me") {
		t.Errorf("the title card reads %q; want the chapter, the Title tag and the superscription", title)
	}
	// CONTROL: the ordinary card of the same verse.
	plain := textsIn(crossRefRow(st, passageCard(ref, false), lightPalette, nop))
	if got := strings.Join(plain, "\n"); plain[0] != "Psalms 59:1" || strings.Contains(got, "Title") || !strings.Contains(got, "Rescue me") {
		t.Errorf("the plain card reads %q", plain)
	}
	// A chapter whose title the loaded text lacks previews its first verse,
	// where the tap lands.
	bare := textsIn(crossRefRow(st, crossRefCard{Ref: crossRef{Book: "Psalms", Chapter: 45, Verse: 4}, Label: "Psalms 45", Tag: "Title", Title: true}, lightPalette, nop))
	if got := strings.Join(bare, "\n"); !strings.Contains(got, "Ride on") {
		t.Errorf("a title card over an untitled chapter reads %q", bare)
	}
	// A basis tag stands in the same slot.
	basis := textsIn(crossRefRow(st, crossRefCard{Ref: crossRef{Book: "Job", Chapter: 13, Verse: 16}, Label: "Job 13:16", Tag: "LXX"}, lightPalette, nop))
	if len(basis) < 2 || basis[0] != "Job 13:16" || basis[1] != "LXX" {
		t.Errorf("the qualified card reads %q", basis)
	}
}

// A ROW'S TAP WASHES WHAT THE ROW CITES. It used to wash the first verse
// only — "John 1:1–3" lit John 1:1 — though Go to and the verse of the day
// already wash a whole range. A wash is one chapter, so a range that runs on
// is washed to the end of the chapter it starts in.
func TestFollowingARowWashesTheRangeItCites(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := xrefBible(map[string]map[int]int{"John": {1: 51}, "Mark": {8: 38, 9: 50}, "Ruth": {4: 22}, "1 Samuel": {1: 28}})
	for _, tc := range []struct {
		c      crossRef
		lo, hi int
	}{
		{crossRef{Book: "John", Chapter: 1, Verse: 1}, 1, 1},
		{crossRef{Book: "John", Chapter: 1, Verse: 1, EndV: 3}, 1, 3},
		{crossRef{Book: "Mark", Chapter: 8, Verse: 34, EndCh: 9, EndV: 1}, 34, 38},
		{crossRef{Book: "Ruth", Chapter: 4, Verse: 18, EndBook: "1 Samuel", EndCh: 1, EndV: 2}, 18, 22},
	} {
		st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1, CurrentVersion: "web"}
		followCrossRef(st, tc.c)
		at := st.mark.At
		if st.CurrentBook != tc.c.Book || st.CurrentChapter != tc.c.Chapter {
			t.Errorf("%s: landed on %s %d", tc.c.label(), st.CurrentBook, st.CurrentChapter)
		}
		if at.Book != tc.c.Book || at.Chapter != tc.c.Chapter || at.Lo != tc.lo || at.Hi != tc.hi {
			t.Errorf("%s: washed %s %d:%d-%d, want %d-%d", tc.c.label(), at.Book, at.Chapter, at.Lo, at.Hi, tc.lo, tc.hi)
		}
	}
}

// A preview ends on a whole word, and never with a space before its ellipsis.
func TestACrossReferencePreviewEndsOnAWord(t *testing.T) {
	short := "In the beginning, God created the heavens and the earth."
	if got := crossRefPreview(short); got != short {
		t.Errorf("a verse shorter than the preview is shown whole, got %q", got)
	}
	// 89 runes of words, then a space at rune 90: the old cut ended "… ".
	long := strings.Repeat("word ", 18) + "andmore text follows here"
	got := crossRefPreview(long)
	if strings.HasSuffix(got, " …") || !strings.HasSuffix(got, "…") {
		t.Errorf("preview %q must end with an ellipsis straight after a word", got)
	}
	// A cut that falls inside a word backs off to the word before it.
	mid := strings.Repeat("abcdefghi ", 8) + "unbreakableword and the rest of the verse"
	if got := crossRefPreview(mid); strings.Contains(got, "unbreak") || !strings.HasSuffix(got, "abcdefghi…") {
		t.Errorf("preview %q must not end inside a word", got)
	}
	for _, s := range []string{long, mid} {
		if n := len([]rune(crossRefPreview(s))); n > crossRefPreviewRunes+1 {
			t.Errorf("preview of %d runes is longer than the %d-rune cut", n, crossRefPreviewRunes)
		}
	}
}

// THE PREVIEW KEEPS THE DIVINE NAME'S SMALL CAPITALS. In the NKJV they are the
// only difference between the Tetragrammaton and Adonai, and the preview showed
// the raw text, which spells both "Lord".
func TestACrossReferencePreviewKeepsTheSmallCapitals(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	const text = "Thus says the Lord God: the Lord is my shepherd."
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {23: {{
		BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: text,
		// "Lord God" is Adonai YHWH: the second word alone is in small capitals.
		SmallCaps: []TextSpan{{Start: 19, End: 22}, {Start: 28, End: 32}},
	}}}}}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23, CurrentVersion: "nkjv"}
	row := crossRefRow(st, passageCard(crossRef{Book: "Psalms", Chapter: 23, Verse: 1}, false), st.pal(), func(crossRef) {})
	got := strings.Join(textsIn(row), "\n")
	if !strings.Contains(got, "the Lord Gᴏᴅ: the Lᴏʀᴅ is my shepherd.") {
		t.Errorf("the preview reads %q; it must keep the small capitals the page draws", got)
	}
}
