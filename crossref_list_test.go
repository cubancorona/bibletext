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

// The panel's composition: with the edition's block off, the list is
// parallels then Treasury rows and nothing of the publisher's; with it on,
// the publisher's notes lead under their own heading and credit, the Treasury
// waits in a disclosure under its own, and the two never share a line.

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
// rich-text segments (a hyperlink's words included) and accordion titles.
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
			// One paragraph: the segments of a note re-join as the reader
			// sees them, links included.
			var b strings.Builder
			for _, s := range x.Segments {
				b.WriteString(s.Textual())
			}
			out = append(out, b.String())
		case *widget.Accordion:
			for _, it := range x.Items {
				out = append(out, it.Title)
				walk(it.Detail)
			}
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

func hyperlinksIn(objs []fyne.CanvasObject) (links []string) {
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.RichText:
			for _, s := range x.Segments {
				if h, ok := s.(*widget.HyperlinkSegment); ok {
					links = append(links, h.Text)
				}
			}
		case *widget.Accordion:
			for _, it := range x.Items {
				walk(it.Detail)
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
	return links
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

func TestCrossRefListWithoutThePublisherIsWhatItAlwaysWas(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", false)
	st, sel := listFixtureState()

	lst := buildCrossRefList(st, sel, listFixtureRefs(), nil, lightPalette, func(crossRef) {})
	if lst.PublisherBlock || lst.PublisherRows != 0 || lst.Disclosure != nil {
		t.Errorf("the publisher's block appeared with the flag off: %+v", lst)
	}
	if lst.Parallels != 1 || lst.TSKRows != 2 || len(lst.Objects) != 3 {
		t.Errorf("list = %d parallels, %d Treasury rows, %d objects; want 1, 2, 3", lst.Parallels, lst.TSKRows, len(lst.Objects))
	}
	all := joinedTexts(lst.Objects)
	if strings.Contains(all, "cross references") || strings.Contains(all, "Thomas Nelson") || strings.Contains(all, "John 7:50; 19:39") {
		t.Errorf("a publisher's note or credit reached the plain list:\n%s", all)
	}
	// The Treasury rows stand in the open, so the footer names the Treasury;
	// and the edition is licensed, so the footer carries its notice under the
	// previews of its verse text.
	sources, notice := crossRefFooterCredit(st)
	if !strings.Contains(sources, "OpenBible.info") || !strings.Contains(sources, "Gospel parallels") {
		t.Errorf("footer sources = %q", sources)
	}
	if !strings.Contains(notice, "Thomas Nelson") {
		t.Errorf("a licensed edition's previews are shown without its notice: %q", notice)
	}
	// CONTROL: a public-domain edition carries no notice.
	st.CurrentVersion = "web"
	if _, notice := crossRefFooterCredit(st); notice != "" {
		t.Errorf("a public-domain edition grew a notice: %q", notice)
	}
}

func TestCrossRefListWithThePublisherLeadsAndSeparates(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, sel := listFixtureState()
	var followed []crossRef
	lst := buildCrossRefList(st, sel, listFixtureRefs(), nil, lightPalette, func(c crossRef) { followed = append(followed, c) })

	if !lst.PublisherBlock || lst.PublisherRows != 4 || lst.Parallels != 1 || lst.TSKRows != 2 {
		t.Fatalf("list = %+v; want the publisher's four notes, one parallel and two Treasury rows", lst)
	}
	if lst.Disclosure == nil || len(lst.Disclosure.Items) != 1 || !strings.Contains(lst.Disclosure.Items[0].Title, "Treasury") {
		t.Fatalf("the Treasury is not in a disclosure of its own: %+v", lst.Disclosure)
	}
	if lst.Disclosure.Items[0].Open {
		t.Error("the disclosure opened although the publisher's block has rows")
	}
	// Order: the parallel, then the publisher's block, then the disclosure last.
	if _, ok := lst.Objects[len(lst.Objects)-1].(*widget.Accordion); !ok {
		t.Error("the disclosure is not the last object")
	}
	all := joinedTexts(lst.Objects)
	if !strings.Contains(all, "NKJV cross references") || !strings.Contains(all, "John 7:50; 19:39") ||
		!strings.Contains(all, "(John 1:13; Gal. 6:15; 1 John 3:9)") {
		t.Errorf("the publisher's notes are not in the list verbatim:\n%s", all)
	}
	if !strings.Contains(all, "compare") {
		t.Error("a parenthesised citation is shown without the legend")
	}
	// The two credits never share a block: the publisher's block (everything
	// before the disclosure) never says OpenBible; the disclosure never says
	// Thomas Nelson.
	open := joinedTexts(lst.Objects[:len(lst.Objects)-1])
	if strings.Contains(open, "OpenBible") {
		t.Errorf("the Treasury's credit reached the publisher's block:\n%s", open)
	}
	if !strings.Contains(open, "Thomas Nelson") {
		t.Errorf("the publisher's block carries no notice:\n%s", open)
	}
	inside := strings.Join(textsIn(lst.Disclosure), "\n")
	if !strings.Contains(inside, "OpenBible") || strings.Contains(inside, "Thomas Nelson") {
		t.Errorf("the disclosure's credit is wrong:\n%s", inside)
	}
	// The footer no longer names the Treasury while its rows are folded away.
	if sources, _ := crossRefFooterCredit(st); strings.Contains(sources, "OpenBible") {
		t.Errorf("the footer credits the Treasury while it sits in the disclosure: %q", sources)
	}
	// Only the resolved citations are links; the untagged one and the one
	// the loaded text lacks are words; tapping a link follows the passage.
	links := hyperlinksIn(lst.Objects[:len(lst.Objects)-1])
	if len(links) != 2 || links[0] != "John 7:50" || links[1] != "Gal. 6:15" {
		t.Fatalf("links = %q, want John 7:50 and Gal. 6:15 only", links)
	}
	for _, o := range lst.Objects {
		for _, s := range richSegmentsIn(o) {
			if h, ok := s.(*widget.HyperlinkSegment); ok && h.Text == "John 7:50" {
				h.OnTapped()
			}
		}
	}
	if len(followed) != 1 || followed[0] != (crossRef{Book: "John", Chapter: 7, Verse: 50}) {
		t.Errorf("tapping the citation followed %+v", followed)
	}
	// The self-range on 3:3 is bold words, not a link.
	if strings.Contains(strings.Join(links, " "), "John 3:1") {
		t.Error("a citation of the passage on screen became a link")
	}
}

func TestCrossRefListEmptyPublisherBlockOpensTheTreasury(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	st, _ := listFixtureState()
	// Psalm 3:2 has no notes.
	st.CurrentBook, st.CurrentChapter = "Psalms", 3
	sel := st.Bible.GetChapter("Psalms", 3)[1:]

	lst := buildCrossRefList(st, sel, []crossRef{{Book: "John", Chapter: 7, Verse: 50, Votes: 3}}, nil, lightPalette, func(crossRef) {})
	if lst.PublisherRows != 0 || !lst.PublisherBlock {
		t.Fatalf("list = %+v", lst)
	}
	all := joinedTexts(lst.Objects)
	if !strings.Contains(all, "The NKJV gives no cross references for this selection.") {
		t.Errorf("the empty state does not name the edition:\n%s", all)
	}
	if !lst.Disclosure.Items[0].Open {
		t.Error("with nothing from the publisher, the Treasury must open by default")
	}
	// A Treasury that failed to load says so in its own place; the block
	// above still stands.
	lst = buildCrossRefList(st, sel, nil, errors.New("offline"), lightPalette, func(crossRef) {})
	inside := strings.Join(textsIn(lst.Disclosure), "\n")
	if !strings.Contains(inside, "Couldn't load the Treasury") || !lst.PublisherBlock {
		t.Errorf("an offline Treasury is not reported in the disclosure:\n%s", inside)
	}
	// CONTROL: with the flag off the same failure and no rows is an empty
	// list, which the panel turns into its message.
	withPublisherCrossRefs(t, "nkjv", false)
	if lst := buildCrossRefList(st, sel, nil, errors.New("offline"), lightPalette, func(crossRef) {}); len(lst.Objects) != 0 {
		t.Errorf("flag off, nothing loaded, yet %d objects", len(lst.Objects))
	}
}

func richSegmentsIn(o fyne.CanvasObject) (segs []widget.RichTextSegment) {
	switch x := o.(type) {
	case *widget.RichText:
		segs = append(segs, x.Segments...)
	case *fyne.Container:
		for _, c := range x.Objects {
			segs = append(segs, richSegmentsIn(c)...)
		}
	}
	return segs
}

// The row renders the note as ONE paragraph: the words between and around the
// citations are kept, in order, so the reader sees the print entry.
func TestPublisherCrossRefRowIsOneParagraph(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	row := publisherCrossRef{Verse: 2, Text: "(John 1:13; Gal. 6:15; 1 John 3:9)",
		Targets: []publisherTarget{{Start: 12, End: 21, Ref: crossRef{Book: "Galatians", Chapter: 6, Verse: 15}}}}
	obj := publisherCrossRefRow(row, lightPalette, func(crossRef) {})
	var texts []string
	for _, s := range richSegmentsIn(obj) {
		texts = append(texts, s.Textual())
	}
	if strings.Join(texts, "") != row.Text {
		t.Errorf("segments %q do not re-join to the note %q", texts, row.Text)
	}
	if len(texts) != 3 || texts[1] != "Gal. 6:15" {
		t.Errorf("segments = %q, want words, link, words", texts)
	}
	// The origin key.
	if all := strings.Join(textsIn(obj), "\n"); !strings.HasPrefix(all, "2\n") {
		t.Errorf("the row does not lead with its verse key:\n%s", all)
	}
	title := publisherCrossRefRow(publisherCrossRef{Verse: 0, Text: "2 Sam. 15:13–17"}, lightPalette, nil)
	if all := strings.Join(textsIn(title), "\n"); !strings.HasPrefix(all, "Title\n") {
		t.Errorf("a title note's row does not lead with Title:\n%s", all)
	}
	_ = container.NewVBox
}

// A ROW'S TAP WASHES WHAT THE ROW CITES. It used to wash the first verse
// only — "John 1:1-3" lit John 1:1 — though Go to and the verse of the day
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
