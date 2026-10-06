//go:build next

package bibletext

// THE GREEK ESTHER IN THE NEXT MAJOR RELEASE (docs/NEXT.md). WEB Catholic
// prints the Greek Esther, translated from a different text from the Hebrew
// Esther the other editions print, and it keeps the Hebrew book's verse
// numbers, so the next release's versification table
// (versification_data_next.go) maps it verse for verse: references, notes,
// highlights and cross-references cross between the two wherever both have
// the verse. The shipping build's behaviour is pinned beside this file, in
// greek_esther_current_test.go; on the day the next release ships that file
// is deleted and this one loses its constraint.

import (
	"reflect"
	"slices"
	"testing"

	"fyne.io/fyne/v2/test"
)

// greekAdditionUnplacedSentence is the line under a note on one of the Greek
// Esther's additions read in the WEB, and greekAdditionUnplacedSeen the start
// of it a banner shows, as seenText reads it. Here the book maps verse for
// verse, and the addition is a verse the Hebrew Esther does not have.
const (
	greekAdditionUnplacedSentence = "These verses are not in the translation being read."
	greekAdditionUnplacedSeen     = "these verses are not in the trans"
)

// crossRefDeepestRead is the furthest down a verse's rows any panel reads,
// measured by the walk in crossrefs_realdata_test.go over the 2026-08-31
// dataset in all four translations, for every selection that can hold the
// verse (crossRefDeepestReadable): the eighteenth row, first at the WEB's
// Matthew 10:1, whose two best rows are its own parallels. maxCrossRefsKept
// is set well above it, and the walk fails if the dataset ever reads deeper,
// so the margin is re-judged rather than assumed.
const crossRefDeepestRead = 18

// The next release's tables are the ones in force: each generated file's init
// puts its table in place of the shipping one before any test runs.
func TestTheTablesInForceAreTheNextReleases(t *testing.T) {
	if reflect.ValueOf(versificationDeltas).Pointer() != reflect.ValueOf(nextVersificationDeltas).Pointer() {
		t.Error("versificationDeltas is not versification_data_next.go's table")
	}
	if reflect.ValueOf(omittedVerses).Pointer() != reflect.ValueOf(nextOmittedVerses).Pointer() {
		t.Error("omittedVerses is not omitted_verses_data_next.go's table")
	}
}

// THE GREEK ESTHER, VERSE FOR VERSE. WEB Catholic prints the Greek Esther,
// translated from a different text from the Hebrew Esther the WEB prints, and
// keeps the Hebrew book's verse numbers: 164 of the WEB's 167 verses are there
// under their own numbers, the three it has nothing at are absent, and its
// additions are numbered after the Hebrew verses or set inside one. Pinned
// whole, because the table is generated (scripts/gen-versification.py --next)
// and a regeneration must not be able to change it unnoticed.
func TestTheGreekEstherMapsVerseForVerse(t *testing.T) {
	d := versificationDeltas["webc"]
	var absent, extra []verseRef
	for _, a := range d.absent {
		if a.Book == "Esther" {
			absent = append(absent, a)
		}
	}
	for _, e := range d.extra {
		if e.Book == "Esther" {
			extra = append(extra, e)
		}
	}
	wantAbsent := []verseRef{{"Esther", 4, 6}, {"Esther", 9, 5}, {"Esther", 9, 30}}
	if !slices.Equal(absent, wantAbsent) {
		t.Errorf("the Greek Esther lacks %v, want %v", absent, wantAbsent)
	}
	var wantExtra []verseRef
	for v := 18; v <= 47; v++ {
		wantExtra = append(wantExtra, verseRef{"Esther", 4, v})
	}
	for v := 4; v <= 14; v++ {
		wantExtra = append(wantExtra, verseRef{"Esther", 10, v})
	}
	if !slices.Equal(extra, wantExtra) {
		t.Errorf("the Greek Esther's own verses are %v, want 4:18-47 and 10:4-14", extra)
	}
	for _, m := range d.moved {
		if m.Book == "Esther" {
			t.Errorf("the table moves Esther %d:%d to %d:%d; the Greek Esther keeps the Hebrew numbers",
				m.Chapter, m.Verse, m.ToChapter, m.ToVerse)
		}
	}
	if why := IncommensurableBook("webc", "Esther"); why != "" {
		t.Errorf("the table still calls the Greek Esther incommensurable: %q", why)
	}
	// The verses the additions are set inside keep their numbers both ways:
	// Addition A opens 1:1, B closes 3:13, D is 5:1-2, E is inside 8:13.
	for _, at := range []verseRef{{"Esther", 1, 1}, {"Esther", 3, 13}, {"Esther", 5, 1}, {"Esther", 5, 2}, {"Esther", 8, 13}} {
		for _, pair := range [][2]string{{"web", "webc"}, {"webc", "web"}} {
			if ch, v, res := MapVerse(pair[0], pair[1], at.Book, at.Chapter, at.Verse); res != verseMapExact ||
				ch != at.Chapter || v != at.Verse {
				t.Errorf("%s->%s Esther %d:%d = %d:%d (%s), want itself", pair[0], pair[1], at.Chapter, at.Verse, ch, v, res)
			}
		}
	}
}

// The Greek Esther is translated from a different text and keeps the Hebrew
// book's verse numbers. A number both have names the same passage, the
// additions set inside a verse included; a number only one has is a verse only
// one has.
func TestMapVerseTheGreekEsther(t *testing.T) {
	for _, tc := range []struct {
		name           string
		from, to, book string
		chapter, verse int
		wantCh, wantV  int
		want           verseMapResult
	}{
		{"the Greek Esther keeps the Hebrew numbers", "web", "webc", "Esther", 4, 1, 4, 1, verseMapExact},
		{"and back out of it", "webc", "web", "Esther", 1, 1, 1, 1, verseMapExact},
		{"an addition set inside a verse keeps the verse", "webc", "web", "Esther", 8, 13, 8, 13, verseMapExact},
		{"the NKJV's Esther is the WEB's", "nkjv", "webc", "Esther", 10, 3, 10, 3, verseMapExact},
		{"Esther 4:6 is not in the Greek Esther", "web", "webc", "Esther", 4, 6, 0, 0, verseMapAbsent},
		{"nor 9:30", "bsb", "webc", "Esther", 9, 30, 0, 0, verseMapAbsent},
		{"Mordecai's prayer has nowhere to go in the WEB", "webc", "web", "Esther", 4, 20, 0, 0, verseMapAbsent},
		{"nor the reading of his dream", "webc", "nkjv", "Esther", 10, 5, 0, 0, verseMapAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch, v, res := MapVerse(tc.from, tc.to, tc.book, tc.chapter, tc.verse)
			if res != tc.want || ch != tc.wantCh || v != tc.wantV {
				t.Errorf("%s %s %d:%d -> %s = %d:%d (%s), want %d:%d (%s)",
					tc.from, tc.book, tc.chapter, tc.verse, tc.to, ch, v, res, tc.wantCh, tc.wantV, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		ch, v int
		want  bool
	}{
		{4, 1, true},
		{4, 6, false}, // the Greek Esther has nothing at 4:6
	} {
		if got := VerseExistsIn("webc", "Esther", tc.ch, tc.v); got != tc.want {
			t.Errorf("VerseExistsIn(webc, Esther %d:%d) = %v, want %v", tc.ch, tc.v, got, tc.want)
		}
	}
}

// A chapter the Greek Esther adds nothing to and lacks nothing of is the same
// chapter verse for verse — the additions inside 1:1 included — and so is
// chapter 10, whose added verses follow the last the NKJV has. Chapters 4 and
// 9, where it has nothing at 4:6, 9:5 and 9:30, differ, and the kind is a
// verse missing.
func TestTheGreekEstherNumbersItsChaptersAsTheHebrewDoes(t *testing.T) {
	for _, tc := range []struct{ chapter, span int }{{1, 22}, {8, 17}, {10, 3}} {
		if !ChapterNumberingAgrees("nkjv", "webc", "Esther", tc.chapter, tc.span) {
			t.Errorf("nkjv->webc Esther %d maps verse for verse; the link should carry the verse", tc.chapter)
		}
	}
	for _, tc := range []struct {
		chapter, span int
		why           string
	}{
		{4, 17, "the Greek Esther has nothing at 4:6"},
		{9, 32, "the Greek Esther has nothing at 9:5 or 9:30"},
	} {
		if ChapterNumberingAgrees("nkjv", "webc", "Esther", tc.chapter, tc.span) {
			t.Errorf("nkjv->webc Esther %d agrees, but %s", tc.chapter, tc.why)
		}
	}
	for _, tc := range []struct {
		chapter, span int
		want          string
	}{
		{1, 22, NumberingSame},
		{4, 17, NumberingAbsent}, // the Greek Esther has nothing at 4:6
	} {
		if got := ChapterNumberingDifference("nkjv", "webc", "Esther", tc.chapter, tc.span); got != tc.want {
			t.Errorf("nkjv->webc Esther %d: got %q, want %q", tc.chapter, got, tc.want)
		}
	}
}

// The Greek Esther keeps the Hebrew book's verse numbers, so the three it
// skips are holes like any other: the numbers between verses it prints that
// it has nothing at, which versification_data_next.go records as absent. Its
// additions are numbered after the Hebrew verses (4:18-47, 10:4-14), so a
// chapter's last Hebrew verse is not a hole either.
func TestGreekEsthersGapsAreHoles(t *testing.T) {
	if got := omittedVersesIn("webc", "Esther", 4); !slices.Equal(got, []int{6}) {
		t.Errorf("the Greek Esther's chapter 4 records %v as omitted, want [6]", got)
	}
	if got := omittedVersesIn("webc", "Esther", 9); !slices.Equal(got, []int{5, 30}) {
		t.Errorf("the Greek Esther's chapter 9 records %v as omitted, want [5 30]", got)
	}
	for _, ch := range []int{1, 2, 3, 5, 6, 7, 8, 10} {
		if got := omittedVersesIn("webc", "Esther", ch); got != nil {
			t.Errorf("the Greek Esther's chapter %d records %v as omitted; it skips no number", ch, got)
		}
	}
	// CONTROL: the Hebrew Esther the WEB and the BSB print skips nothing.
	for _, edition := range []string{"web", "bsb"} {
		if got := omittedVersesIn(edition, "Esther", 4); got != nil {
			t.Errorf("%s's Esther 4 records %v as omitted", edition, got)
		}
	}
}

// A note on the Hebrew Esther lands on the Greek Esther's verse of the same
// number, except at the three it has nothing at; and a note on one of its
// additions has no Hebrew verse to land on.
func TestANoteOnTheGreekEstherLandsVerseForVerse(t *testing.T) {
	bible := anchorTestBible()
	note := func(vid string, verse int) StoredNote {
		return StoredNote{Kind: noteKindReceived, VersionID: vid, Book: "Esther", Chapter: 4, VerseLo: verse, Text: "x"}
	}
	for _, tc := range []struct {
		name    string
		note    StoredNote
		reading string
		want    placement
	}{
		{
			// The Greek Esther keeps the Hebrew numbers: a note on the
			// Hebrew Esther's 4:1 lands on the Greek Esther's 4:1.
			name: "Greek Esther follows verse for verse",
			note: note("web", 1), reading: "webc",
			want: placement{Kind: placedExact, Here: []anchorRun{{Chapter: 4, Lo: 1, Hi: 1}}},
		},
		{
			// ...except at the three verses the Greek text has nothing at.
			name: "a verse the Greek Esther lacks",
			note: note("web", 6), reading: "webc",
			want: placement{Kind: unplacedAbsent},
		},
		{
			// And an addition — Mordecai's prayer, 4:18-47 — is in no
			// Hebrew Esther.
			name: "a Greek addition read in the Hebrew Esther",
			note: note("webc", 20), reading: "web",
			want: placement{Kind: unplacedAbsent},
		},
	} {
		if got := resolveNoteAnchor(tc.note, tc.reading, bible); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got %v %+v\nwant %v %+v", tc.name, got.Kind, got, tc.want.Kind, tc.want)
		}
	}
}

// ...but only where the passage genuinely corresponds. The Greek Esther keeps
// the Hebrew book's verse numbers, so a note on the Hebrew Esther's 4:1 goes
// to the Greek Esther's 4:1 — and a note on 4:6, which the Greek text has
// nothing at, stays where it is rather than being planted on a neighbour.
func TestANoteFollowsIntoTheGreekEstherVerseForVerse(t *testing.T) {
	p := newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 1,
		Text: "fixture translation message alpha"})
	if got, ok := noteForChapter(p, "webc", "Esther", 4, nil); !ok || got.VerseLo != 1 {
		t.Errorf("a note on Esther 4:1 did not follow to the Greek Esther's 4:1: %+v (%v)", got, ok)
	}

	p = newNotePrefs()
	addNote(p, StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Esther", Chapter: 4, VerseLo: 6,
		Text: "fixture translation message beta"})
	if got, ok := noteForChapter(p, "webc", "Esther", 4, nil); ok {
		t.Errorf("a note on Esther 4:6 crossed into the Greek Esther, which has nothing there: %+v", got)
	}
}

// A highlight on the Greek Esther crosses into the Hebrew Esther at a verse
// both number alike, and clears on one of its additions.
func TestAHighlightFollowsTheGreekEstherVerseForVerse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	t.Run("the Greek Esther keeps the Hebrew numbers", func(t *testing.T) {
		st := markedInGreekEsther(t, 17)
		applyVersionSwitchForTest(t, st, "web", estherFourBible(17, 0))
		if sp, ok := st.markSpan(); !ok || sp.Chapter != 4 || sp.Lo != 17 {
			t.Errorf("the Greek Esther's 4:17 is the Hebrew Esther's 4:17; the mark is %+v (%v)", sp, ok)
		}
	})

	t.Run("a Greek addition clears", func(t *testing.T) {
		if _, _, res := MapVerse("webc", "web", "Esther", 4, 20); res != verseMapAbsent {
			t.Fatalf("precondition: webc Esther 4:20 should be ABSENT from the web, got %v", res)
		}
		st := markedInGreekEsther(t, 20)
		applyVersionSwitchForTest(t, st, "web", estherFourBible(17, 0))
		if sp, ok := st.markSpan(); ok {
			t.Errorf("a mark on a verse only the Greek Esther has must clear, still lights %d:%d", sp.Chapter, sp.Lo)
		}
	})
}

// The sentence for a note on a book whose numbering does not correspond,
// word for word. A draft awaiting approval (docs/NEXT.md, decision C): no
// translation pair in the next release has such a book, so no reader would
// see it; it is pinned so that its wording changes only on purpose.
func TestTheIncommensurableSentenceIsPinned(t *testing.T) {
	const want = "The translation being read numbers the whole of this book differently."
	if placementCopyIncommensurable != want || placementCopy(unplacedIncommensurable) != want {
		t.Errorf("the incommensurable sentence is %q, want %q", placementCopy(unplacedIncommensurable), want)
	}
}

// The web reader's completeness check expects the Greek Esther's numbers: the
// Hebrew chapter's less the verse it has nothing at, then its additions.
func TestExpectedVerseNumbersMapTheGreekEsther(t *testing.T) {
	numbered := func(ch, last int) []Verse {
		var out []Verse
		for v := 1; v <= last; v++ {
			out = append(out, Verse{BookName: "Esther", Chapter: ch, Verse: v, Text: "x"})
		}
		return out
	}
	ref := &BibleData{
		Books:  []string{"Esther"},
		Verses: map[string]map[int][]Verse{"Esther": {1: numbered(1, 22), 4: numbered(4, 17), 10: numbered(10, 3)}},
	}
	got, err := ExpectedVerseNumbers("webc", ref)
	if err != nil {
		t.Fatal(err)
	}
	span := func(lo, hi int, skip ...int) []int {
		var out []int
		for v := lo; v <= hi; v++ {
			if !slices.Contains(skip, v) {
				out = append(out, v)
			}
		}
		return out
	}
	for _, tc := range []struct {
		ch   int
		want []int
	}{
		{1, span(1, 22)},
		{4, span(1, 47, 6)},
		{10, span(1, 14)},
	} {
		if !slices.Equal(got["Esther"][tc.ch], tc.want) {
			t.Errorf("WEBC's Esther %d is expected as %v, want %v", tc.ch, got["Esther"][tc.ch], tc.want)
		}
	}
}
