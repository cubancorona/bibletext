package bibletext

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// displacedFixture is a text with the marks of the two stretches the NKJV
// feed hangs one caller late: the contents delivered at their first and last
// callers, a verse before each that lacks its caller, and callers between,
// across a chapter, a book and a Psalm's title. Each caller's offset is its
// own, so a test can see that a caller keeps its place when its content
// changes.
func displacedFixture() *BibleData {
	bd := xrefBible(map[string]map[int]int{
		"Ruth": {3: 18, 4: 22}, "1 Samuel": {1: 28}, "2 Samuel": {24: 25}, "1 Kings": {1: 53},
		"Psalms": {119: 176, 120: 7, 150: 6}, "Proverbs": {1: 33},
	})
	bd.Books = []string{"Ruth", "1 Samuel", "2 Samuel", "1 Kings", "Psalms", "Proverbs"} // the feed's order
	note := func(anchor int, text string) Footnote {
		return Footnote{Anchor: anchor, Text: text, Kind: footnoteKindCrossref, Caller: "-"}
	}
	set := func(book string, ch, v int, notes ...Footnote) {
		bd.Verses[book][ch][v-1].Footnotes = notes
	}
	set("Ruth", 3, 10, note(3, "Ruth 2:20"))
	set("Ruth", 3, 12, note(4, "Prov. 12:4; 31:10–31"), note(10, "Ruth 3:9"))
	set("Ruth", 3, 13, note(5, "Ruth 4:1"))
	set("Ruth", 4, 22, note(6, "Matt. 1:4"))
	set("1 Samuel", 1, 1, note(7, "1 Chr. 2:15"), note(12, "Josh. 17:17"))
	set("2 Samuel", 24, 24, note(8, "(Ezek. 20:40, 41)"))
	set("2 Samuel", 24, 25, note(9, "1 Chr. 21:24, 25"), note(14, "2 Sam. 21:14"))
	set("1 Kings", 1, 1, note(2, "1 Chr. 23:1"))
	set("Psalms", 119, 132, note(0, "Ps. 106:4"))
	set("Psalms", 119, 133, note(0, "Ps. 51:1; (2 Thess. 1:6)"), note(11, "Ps. 17:5"))
	bd.Superscriptions = map[string]map[int]Superscription{"Psalms": {120: {
		Text: "A fixture song of ascents.", Footnotes: []Footnote{note(2, "Ps. 119:170")},
	}}}
	set("Psalms", 120, 1, note(3, "Jon. 2:2"))
	set("Psalms", 150, 1, note(6, "Ps. 148:14"))
	set("Psalms", 150, 2, note(5, "Ps. 145:5, 6"))
	set("Proverbs", 1, 1, note(4, "1 Kin. 4:32"))
	return bd
}

// shownNotes names what one verse shows: each note by its letter ("-" for a
// note keyed to no word), its offset and its text, a title's with "title".
func shownNotes(bd *BibleData, book string, ch, v int) string {
	var out []string
	for _, n := range publisherCrossRefsFor(bd, book, ch, versesBetween(bd, book, ch, v, v)) {
		letter := noteLetter(n.Index)
		if n.Unkeyed {
			letter = "-"
		}
		if n.Verse == 0 {
			letter = "title " + letter
		}
		out = append(out, letter+"@"+strconv.Itoa(n.Anchor)+" "+n.Text)
	}
	return strings.Join(out, " | ")
}

// WHERE THE FEED HANGS EVERY NOTE ONE CALLER LATE, A CALLER SHOWS THE NEXT
// CALLER'S CONTENT. The caller keeps its place; the first content of the
// stretch, whose caller the feed lacks, stands with its verse keyed to no
// word; the last caller, whose content the feed lacks, shows nothing; and
// outside the stretch nothing moves.
func TestANoteTheFeedHangsOneCallerLateIsShownOnItsOwn(t *testing.T) {
	bd := displacedFixture()
	for _, c := range []struct {
		book  string
		ch, v int
		want  string
	}{
		{"Ruth", 3, 10, "a@3 Ruth 2:20"},                                     // before the stretch: its own
		{"Ruth", 3, 11, "-@0 Prov. 12:4; 31:10–31"},                          // the stretch's first content, on the verse that lacks its caller
		{"Ruth", 3, 12, "a@4 Ruth 3:9 | b@10 Ruth 4:1"},                      // each caller in its place, with the next one's content
		{"Ruth", 3, 13, "a@5 Matt. 1:4"},                                     // across a chapter
		{"Ruth", 4, 22, "a@6 1 Chr. 2:15"},                                   // across a book
		{"1 Samuel", 1, 1, "a@7 Josh. 17:17 | b@12 (Ezek. 20:40, 41)"},       // within a verse, then on
		{"2 Samuel", 24, 24, "a@8 1 Chr. 21:24, 25"},                         //
		{"2 Samuel", 24, 25, "a@9 2 Sam. 21:14"},                             // the last caller's content is not in the feed
		{"1 Kings", 1, 1, "a@2 1 Chr. 23:1"},                                 // after the stretch: its own
		{"Psalms", 119, 132, "a@0 Ps. 106:4 | -@0 Ps. 51:1; (2 Thess. 1:6)"}, // the verse's own, then the content of its missing second caller
		{"Psalms", 119, 133, "a@0 Ps. 17:5 | b@11 Ps. 119:170"},              // on to the next psalm's title
		{"Psalms", 120, 1, "title a@2 Jon. 2:2 | b@3 Ps. 148:14"},            // a title's caller, then its verse's
		{"Psalms", 150, 1, "a@6 Ps. 145:5, 6"},                               //
		{"Psalms", 150, 2, ""},                                               // the last caller
		{"Proverbs", 1, 1, "a@4 1 Kin. 4:32"},                                // after it
	} {
		if got := shownNotes(bd, c.book, c.ch, c.v); got != c.want {
			t.Errorf("%s %d:%d shows\n got %s\nwant %s", c.book, c.ch, c.v, got, c.want)
		}
	}
	// A content moved from the verse it cited to a verse it does not cite is
	// a card there, not the reader's own passage: Ruth 3:9, delivered on
	// 3:12's second caller, is shown on its first.
	n := publisherCrossRefsFor(bd, "Ruth", 3, versesBetween(bd, "Ruth", 3, 12, 12))[0]
	if got := joined(pieceNames(n.Pieces)); got != "card:Ruth 3:9" {
		t.Errorf("Ruth 3:12's first note is %s", got)
	}

	// CONTROL: a feed that is put right is read as delivered. With a caller
	// on Ruth 3:11 the first stretch no longer matches; the second still does.
	fixed := displacedFixture()
	fixed.Verses["Ruth"][3][10].Footnotes = []Footnote{{Anchor: 1, Text: "Prov. 12:4; 31:10–31", Kind: footnoteKindCrossref, Caller: "-"}}
	for _, c := range []struct {
		book  string
		ch, v int
		want  string
	}{
		{"Ruth", 3, 11, "a@1 Prov. 12:4; 31:10–31"},
		{"Ruth", 3, 12, "a@4 Prov. 12:4; 31:10–31 | b@10 Ruth 3:9"},
		{"2 Samuel", 24, 25, "a@9 1 Chr. 21:24, 25 | b@14 2 Sam. 21:14"},
		{"Psalms", 150, 1, "a@6 Ps. 145:5, 6"},
	} {
		if got := shownNotes(fixed, c.book, c.ch, c.v); got != c.want {
			t.Errorf("control, the first stretch put right: %s %d:%d shows\n got %s\nwant %s", c.book, c.ch, c.v, got, c.want)
		}
	}
	// CONTROL: so is a feed whose first content there is another, or whose
	// last caller has gained a content after it.
	other := displacedFixture()
	other.Verses["Psalms"][119][132].Footnotes[0].Text = "Ps. 17:5"
	if got := shownNotes(other, "Psalms", 150, 2); got != "a@5 Ps. 145:5, 6" {
		t.Errorf("control, another first content: Psalm 150:2 shows %q", got)
	}
	grown := displacedFixture()
	v := &grown.Verses["2 Samuel"][24][24]
	v.Footnotes = append(v.Footnotes, Footnote{Anchor: 20, Text: "2 Sam. 24:21", Kind: footnoteKindCrossref, Caller: "-"})
	if got := shownNotes(grown, "Ruth", 3, 12); got != "a@4 Prov. 12:4; 31:10–31 | b@10 Ruth 3:9" {
		t.Errorf("control, a content after the last caller: Ruth 3:12 shows %q", got)
	}
	// CONTROL: and the switch a test reads the feed as delivered by.
	correctDisplacedNotes = false
	defer func() { correctDisplacedNotes = true }()
	if got := shownNotes(bd, "Ruth", 3, 12); got != "a@4 Prov. 12:4; 31:10–31 | b@10 Ruth 3:9" {
		t.Errorf("control, the correction off: Ruth 3:12 shows %q", got)
	}
	if got := shownNotes(bd, "Ruth", 3, 11); got != "" {
		t.Errorf("control, the correction off: Ruth 3:11 shows %q", got)
	}
}

// A SILENT DISPLACED CALLER WITH NO OTHER NOTE IN SELECTION SAYS SO PLAINLY.
// On Psalm 150:2 the edition prints a cross-reference whose content our feed
// omits; saying "No NKJV cross-references" there would misstate the edition,
// whereas on 2 Samuel 24:25 the verse's first note still shows on its own.
func TestASilentDisplacedCallerNamesTheMissingNote(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	bd := displacedFixture()
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 150, CurrentVersion: "nkjv"}
	nop := func(crossRef) {}
	ps150v2 := versesBetween(bd, "Psalms", 150, 2, 2)
	const wantMissing = "The NKJV has a cross-reference here that isn't in our source."

	// With Treasury rows loaded: NKJV REFERENCES shows the missing-source
	// sentence above the Treasury section.
	tsk := []crossRef{{Book: "Proverbs", Chapter: 1, Verse: 1, Votes: 8}}
	lst := buildCrossRefList(st, ps150v2, tsk, nil, lightPalette, nop)
	if got := joined(sectionLabelsIn(lst.Objects)); got != "NKJV REFERENCES|TREASURY OF SCRIPTURE KNOWLEDGE" {
		t.Errorf("Psalm 150:2 labels = %q", got)
	}
	all := joinedTexts(lst.Objects)
	if !strings.Contains(all, wantMissing) || strings.Contains(all, "No NKJV cross-references") || lst.CoverageNote != "" {
		t.Errorf("Psalm 150:2 with Treasury:\n%s\ncoverage=%q", all, lst.CoverageNote)
	}

	// With no Treasury rows (and even offline): the missing-source sentence
	// still renders under NKJV REFERENCES rather than collapsing into the
	// generic empty or connection-error message.
	for _, tc := range []struct {
		name string
		err  error
	}{{"no treasury", nil}, {"offline", errors.New("offline")}} {
		lst := buildCrossRefList(st, ps150v2, nil, tc.err, lightPalette, nop)
		if len(lst.Objects) == 0 {
			t.Fatalf("%s: Psalm 150:2 returned an empty list", tc.name)
		}
		if got := joinedTexts(lst.Objects); !strings.Contains(got, wantMissing) || lst.CoverageNote != "" {
			t.Errorf("%s: Psalm 150:2 rendered:\n%s\ncoverage=%q", tc.name, got, lst.CoverageNote)
		}
	}

	// Selecting Psalm 150:1–2 shows 150:1's note without the empty/missing line.
	span := buildCrossRefList(st, versesBetween(bd, "Psalms", 150, 1, 2), tsk, nil, lightPalette, nop)
	if got := joinedTexts(span.Objects); strings.Contains(got, wantMissing) || !strings.Contains(got, "Ps. 145:5, 6") {
		t.Errorf("Psalm 150:1–2 rendered:\n%s", got)
	}

	// 2 Samuel 24:25 has its first note present; no missing-source line is added.
	st2Sam := &AppState{Bible: bd, CurrentBook: "2 Samuel", CurrentChapter: 24, CurrentVersion: "nkjv"}
	sam := buildCrossRefList(st2Sam, versesBetween(bd, "2 Samuel", 24, 25, 25), tsk, nil, lightPalette, nop)
	if got := joinedTexts(sam.Objects); strings.Contains(got, wantMissing) || !strings.Contains(got, "2 Sam. 21:14") {
		t.Errorf("2 Samuel 24:25 rendered:\n%s", got)
	}

	// CONTROL: an ordinary verse with no notes omits the publisher section;
	// it must not acquire the genuine missing-source warning.
	stProv := &AppState{Bible: bd, CurrentBook: "Proverbs", CurrentChapter: 1, CurrentVersion: "nkjv"}
	prov := buildCrossRefList(stProv, versesBetween(bd, "Proverbs", 1, 2, 2), tsk, nil, lightPalette, nop)
	if got := joinedTexts(prov.Objects); strings.Contains(got, wantMissing) || strings.Contains(got, "No NKJV cross-references for this selection.") || prov.Sections.Publisher || joined(sectionLabelsIn(prov.Objects)) != treasuryLabel {
		t.Errorf("Proverbs 1:2 rendered:\n%s", got)
	}

	// CONTROL: if the feed stretch is fixed, Psalm 150:2 shows its delivered note.
	other := displacedFixture()
	other.Verses["Psalms"][119][132].Footnotes[0].Text = "Ps. 17:5"
	stFixed := &AppState{Bible: other, CurrentBook: "Psalms", CurrentChapter: 150, CurrentVersion: "nkjv"}
	fixed := buildCrossRefList(stFixed, versesBetween(other, "Psalms", 150, 2, 2), tsk, nil, lightPalette, nop)
	if got := joinedTexts(fixed.Objects); strings.Contains(got, wantMissing) || !strings.Contains(got, "Ps. 145:5, 6") {
		t.Errorf("fixed feed Psalm 150:2 rendered:\n%s", got)
	}
}
