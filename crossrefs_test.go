package bibletext

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"unsafe"
)

func TestParseOSISTarget(t *testing.T) {
	single, ok := parseOSISTarget("Ps.90.2")
	if !ok || single.Book != "Psalms" || single.Chapter != 90 || single.Verse != 2 || single.EndV != 0 {
		t.Errorf("single: %+v ok=%v", single, ok)
	}
	if single.label() != "Psalms 90:2" {
		t.Errorf("single label = %q", single.label())
	}

	rng, ok := parseOSISTarget("Rom.1.19-Rom.1.20")
	if !ok || rng.Book != "Romans" || rng.Verse != 19 || rng.EndV != 20 {
		t.Errorf("range: %+v ok=%v", rng, ok)
	}
	if rng.label() != "Romans 1:19-20" {
		t.Errorf("range label = %q", rng.label())
	}

	if _, ok := parseOSISTarget("Zzz.1.1"); ok {
		t.Error("unknown book should not parse")
	}
}

func makeCrossRefZip(t *testing.T, rows string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("cross_references.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("From Verse\tTo Verse\tVotes\n" + rows)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseCrossRefZipAndRank(t *testing.T) {
	rows := "Gen.1.1\tHeb.1.2\t64\n" +
		"Gen.1.1\tJohn.1.1-John.1.3\t369\n" +
		"Gen.1.1\tPs.90.2\t61\n"
	idx, _, err := parseCrossRefZip(makeCrossRefZip(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	got := idx[crossRefKey("Genesis", 1, 1)]
	if len(got) != 3 {
		t.Fatalf("want 3 refs, got %d", len(got))
	}
	// Highest votes first: John 1:1-3 (369).
	if top := got[0].crossRef(); top.Book != "John" || top.Votes != 369 {
		t.Errorf("top ref = %+v, want John 1:1-3 (369)", top)
	}
}

// The cross-reference dataset (OpenBible's TSK) numbers the Romans doxology
// 16:25-27, as the BSB and the ESV do. The app's reference versification is
// the WEB, which numbers it 14:24-26. Nothing normalised between the two, so
// both halves of the feature failed on that passage:
//
//   - a reader on ANY translation selecting the doxology looked the dataset up
//     under the reference number and found none of its 92 rows;
//   - a row POINTING at the doxology kept its dataset number, which in the WEB
//     names a verse that does not exist — a labelled row with a blank preview
//     and a tap that goes nowhere.
//
// Both are fixed by normalising the dataset into the reference numbering as it
// is parsed, so everything downstream sees one numbering.
func TestCrossRefDatasetNumberingIsNormalised(t *testing.T) {
	// One TSK row in the dataset's own numbering: Romans 16:25 -> Eph 3:20.
	const tsv = "From Verse\tTo Verse\tVotes\n" +
		"Rom.16.25\tEph.3.20\t30\n" +
		"Eph.3.20\tRom.16.25-Rom.16.27\t70\n"

	idx, _, err := parseCrossRefRows(strings.NewReader(tsv))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// CONTROL: the parser must produce something at all, or the absences below
	// prove nothing.
	if len(idx) == 0 {
		t.Fatal("control: the parser produced no rows; the assertions below are vacuous")
	}

	// (1) The SOURCE key is the reference number, not the dataset's.
	if got := idx[crossRefKey("Romans", 14, 24)]; len(got) != 1 {
		t.Errorf("the doxology's row must be keyed at the reference number "+
			"Romans 14:24, got %d rows there", len(got))
	}
	if got := idx[crossRefKey("Romans", 16, 25)]; len(got) != 0 {
		t.Errorf("nothing may remain keyed at the dataset's Romans 16:25: %+v", got)
	}

	// (2) The TARGET is rewritten too, span end included, so the panel can
	// preview and navigate to text that exists.
	rows := idx[crossRefKey("Ephesians", 3, 20)]
	if len(rows) != 1 {
		t.Fatalf("expected one row from Ephesians 3:20, got %+v", rows)
	}
	tgt := rows[0].crossRef()
	if tgt.Book != "Romans" || tgt.Chapter != 14 || tgt.Verse != 24 {
		t.Errorf("target start must be the reference number Romans 14:24, got %s %d:%d",
			tgt.Book, tgt.Chapter, tgt.Verse)
	}
	if tgt.EndV != 26 || (tgt.EndCh != 0 && tgt.EndCh != 14) {
		t.Errorf("target span end must map to 14:26, got EndCh=%d EndV=%d", tgt.EndCh, tgt.EndV)
	}
}

// The dataset's 3 John runs to verse 15, as the ESV's does; every shipped
// text ends the letter at 14, with the closing greeting inside it. The
// dataset's one row from 3 John 1:15 — the friends greeted "by name", to John
// 10:3 — was keyed to a verse no translation has, so no reader ever saw it.
func TestTheDatasetsThirdJohnFifteenIsTheReferencesFourteen(t *testing.T) {
	const tsv = "From Verse\tTo Verse\tVotes\n" +
		"3John.1.15\tJohn.10.3\t1\n" +
		"John.10.3\t3John.1.15\t1\n"
	idx, _, err := parseCrossRefRows(strings.NewReader(tsv))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) == 0 {
		t.Fatal("control: the parser produced no rows; the assertions below are vacuous")
	}
	if got := idx[crossRefKey("3 John", 1, 14)]; len(got) != 1 || got[0].crossRef().Book != "John" {
		t.Errorf("3 John 1:15's row must be keyed at 3 John 1:14, got %+v", got)
	}
	if got := idx[crossRefKey("3 John", 1, 15)]; len(got) != 0 {
		t.Errorf("nothing may stay keyed at a 3 John 1:15 no translation has: %+v", got)
	}
	if got := idx[crossRefKey("John", 10, 3)]; len(got) != 1 || got[0].crossRef().label() != "3 John 1:14" {
		t.Errorf("a row pointing at 3 John 1:15 must point at 1:14, got %+v", got)
	}
}

// withCrossRefIndex installs a Treasury index parsed from a synthetic TSV for
// the test's duration, in place of the downloaded one.
func withCrossRefIndex(t *testing.T, rows string) {
	t.Helper()
	idx, _, err := parseCrossRefRows(strings.NewReader("From Verse\tTo Verse\tVotes\n" + rows))
	if err != nil {
		t.Fatal(err)
	}
	crossRefMu.Lock()
	was := crossRefIndex
	crossRefIndex = idx
	crossRefMu.Unlock()
	t.Cleanup(func() {
		crossRefMu.Lock()
		crossRefIndex = was
		crossRefMu.Unlock()
	})
}

// xrefBible is a synthetic text holding the named chapters, each with verses
// 1..n, so a panel can be built without a downloaded translation.
func xrefBible(chapters map[string]map[int]int) *BibleData {
	bd := &BibleData{Verses: map[string]map[int][]Verse{}}
	var books []string
	for book := range chapters {
		books = append(books, book)
	}
	sort.Strings(books)
	for _, book := range books {
		chs := chapters[book]
		bd.Books = append(bd.Books, book)
		bd.Verses[book] = map[int][]Verse{}
		for ch, n := range chs {
			for v := 1; v <= n; v++ {
				bd.Verses[book][ch] = append(bd.Verses[book][ch], Verse{
					BookName: book, Book: book, Chapter: ch, Verse: v,
					Text: fmt.Sprintf("Synthetic text of %s %d:%d.", book, ch, v),
				})
			}
		}
	}
	return bd
}

// A RANGE THAT RUNS INTO THE NEXT BOOK. Eighteen Treasury rows end in a
// different book from the one they start in, and the end's book was thrown
// away, so the end was read as a verse of the START book: "Leviticus
// 27:34-1:1" ran backwards, and "2 John 1:1-15" named a verse 2 John does not
// have — the dataset means 2 John 1:1 to 3 John 1:15, the greeting every
// shipped text prints at 3 John 1:14.
func TestACrossBookRangeKeepsItsEndBook(t *testing.T) {
	r, ok := parseOSISTarget("Lev.27.34-Num.1.1")
	if !ok || r.Book != "Leviticus" || r.EndBook != "Numbers" || r.EndCh != 1 || r.EndV != 1 {
		t.Fatalf("parsed %+v (ok=%v)", r, ok)
	}
	if got := r.label(); got != "Leviticus 27:34-Numbers 1:1" {
		t.Errorf("label %q, want \"Leviticus 27:34-Numbers 1:1\"", got)
	}

	withCrossRefIndex(t, "Num.3.1\tLev.27.34-Num.1.1\t5\n"+
		"Acts.11.30\t2John.1.1-3John.1.15\t4\n")
	bd := xrefBible(map[string]map[int]int{
		"Leviticus": {27: 34}, "Numbers": {1: 3, 3: 1},
		"Acts": {11: 30}, "2 John": {1: 13}, "3 John": {1: 14},
	})
	for _, tc := range []struct {
		book    string
		ch, v   int
		want    string
		version string
	}{
		{"Numbers", 3, 1, "Leviticus 27:34-Numbers 1:1", "web"},
		{"Numbers", 3, 1, "Leviticus 27:34-Numbers 1:1", "bsb"},
		{"Acts", 11, 30, "2 John 1:1-3 John 1:14", "web"},
		{"Acts", 11, 30, "2 John 1:1-3 John 1:14", "nkjv"},
	} {
		st := &AppState{Bible: bd, CurrentBook: tc.book, CurrentChapter: tc.ch, CurrentVersion: tc.version}
		refs := crossRefsForSelection(st, "", selSpan{lo: tc.v, hi: tc.v})
		var labels []string
		for _, c := range refs {
			labels = append(labels, c.label())
		}
		// CONTROL: the panel was built at all, or a missing label proves nothing.
		if len(refs) == 0 {
			t.Fatalf("%s %s %d:%d: the panel is empty", tc.version, tc.book, tc.ch, tc.v)
		}
		if labels[0] != tc.want {
			t.Errorf("%s %s %d:%d lists %q, want %q first", tc.version, tc.book, tc.ch, tc.v, labels, tc.want)
		}
	}
}

// THE CAP COUNTS ROWS A READER CAN SEE. A verse shows its sixteen best
// Treasury rows. The sixteen used to be chosen when the dataset was read,
// before the rows this translation cannot show were dropped and the rows a
// parallel already shows were hidden, so each of those left an empty place
// instead of handing it to the seventeenth: WEB Catholic's Genesis 41:42
// showed 10, and Matthew 10:1 showed 14 in every translation.
func TestTheCapCountsOnlyRowsTheReaderCanSee(t *testing.T) {
	var rows strings.Builder
	// The two best rows point into Esther, which WEB Catholic's Greek Esther
	// cannot receive (versification: incommensurable).
	rows.WriteString("Gen.41.42\tEsth.6.8\t90\nGen.41.42\tEsth.8.15\t80\n")
	for v := 1; v <= 16; v++ {
		fmt.Fprintf(&rows, "Gen.41.42\tDan.5.%d\t%d\n", v, 40-v)
	}
	withCrossRefIndex(t, rows.String())
	bd := xrefBible(map[string]map[int]int{"Genesis": {41: 57}, "Esther": {6: 14, 8: 17}, "Daniel": {5: 31}})
	count := func(vid string) (n int, last string) {
		st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 41, CurrentVersion: vid}
		for _, c := range crossRefsForSelection(st, "", selSpan{lo: 42, hi: 42}) {
			n, last = n+1, c.label()
		}
		return n, last
	}
	// CONTROL: the WEB has Esther, so its sixteen are the top sixteen, and the
	// sixteenth is Daniel 5:14.
	if n, last := count("web"); n != maxCrossRefsPerVerse || last != "Daniel 5:14" {
		t.Fatalf("control: the WEB lists %d rows ending %q, want 16 ending \"Daniel 5:14\"", n, last)
	}
	if n, last := count("webc"); n != maxCrossRefsPerVerse || last != "Daniel 5:16" {
		t.Errorf("WEB Catholic lists %d rows ending %q, want 16 ending \"Daniel 5:16\": the two "+
			"rows into Greek Esther must hand their places to the next two", n, last)
	}

	// The other half: a row the panel hides because a Gospel parallel listed
	// above it already shows that passage. Matthew 10:1's two best rows are
	// Luke 9:1-6 and Mark 6:7-13, which are its parallels (the commissioning
	// of the Twelve), so its sixteen Treasury rows are the sixteen after them.
	rows.Reset()
	rows.WriteString("Matt.10.1\tLuke.9.1-Luke.9.6\t90\nMatt.10.1\tMark.6.7-Mark.6.13\t80\n")
	for v := 1; v <= 16; v++ {
		fmt.Fprintf(&rows, "Matt.10.1\tDan.5.%d\t%d\n", v, 40-v)
	}
	withCrossRefIndex(t, rows.String())
	gospel := xrefBible(map[string]map[int]int{"Matthew": {10: 42}, "Mark": {6: 56}, "Luke": {9: 62}, "Daniel": {5: 31}})
	st := &AppState{Bible: gospel, CurrentBook: "Matthew", CurrentChapter: 10, CurrentVersion: "web"}
	var parallels, treasury []string
	for _, c := range crossRefsForSelection(st, "", selSpan{lo: 1, hi: 1}) {
		if c.Parallel {
			parallels = append(parallels, c.label())
		} else {
			treasury = append(treasury, c.label())
		}
	}
	// CONTROL: the two rows really are hidden behind the parallels, or the
	// count below is not about hiding at all.
	if strings.Join(parallels, "; ") != "Mark 6:7-13; Luke 9:1-6" {
		t.Fatalf("control: Matthew 10:1's parallels are %q, want Mark 6:7-13 and Luke 9:1-6", parallels)
	}
	for _, lbl := range treasury {
		if lbl == "Mark 6:7-13" || lbl == "Luke 9:1-6" {
			t.Fatalf("control: the Treasury rows repeat the parallel %q", lbl)
		}
	}
	if len(treasury) == 0 {
		t.Fatal("Matthew 10:1 lists no Treasury rows")
	}
	if n, last := len(treasury), treasury[len(treasury)-1]; n != maxCrossRefsPerVerse || last != "Daniel 5:16" {
		t.Errorf("Matthew 10:1 lists %d Treasury rows ending %q, want 16 ending \"Daniel 5:16\": "+
			"the two rows its parallels already show must hand their places to the next two", n, last)
	}
}

// THE INDEX KEEPS EACH VERSE'S BEST maxCrossRefsKept ROWS, EIGHT BYTES EACH.
// It is held for as long as the app runs, on every platform. As crossRefs,
// every row kept, it came to some 53 MB; the panel never reads past a
// verse's twentieth row (crossRefDeepestRead), so the rest need not be kept,
// and a row needs only its books, numbers and votes.
func TestTheIndexKeepsEachVersesBestRows(t *testing.T) {
	var rows strings.Builder
	n := maxCrossRefsKept + 8
	for v := 1; v <= n; v++ {
		// Votes fall in pairs, so ties must keep the dataset's order.
		fmt.Fprintf(&rows, "Gen.1.1\tPs.119.%d\t%d\n", v, (n-v)/2)
	}
	rows.WriteString("Gen.1.2\tPs.104.30\t9\nGen.1.2\tJob.26.13\t8\n")
	tsv := "From Verse\tTo Verse\tVotes\n" + rows.String()
	// CONTROL: read without the cap, every row is there.
	every, _, err := readCrossRefRows(strings.NewReader(tsv), 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(every[crossRefKey("Genesis", 1, 1)]); got != n {
		t.Fatalf("control: the uncapped index holds %d rows, want %d", got, n)
	}
	idx, _, err := parseCrossRefRows(strings.NewReader(tsv))
	if err != nil {
		t.Fatal(err)
	}
	got := idx[crossRefKey("Genesis", 1, 1)]
	if len(got) != maxCrossRefsKept {
		t.Fatalf("the index keeps %d of the verse's %d rows, want its best %d", len(got), n, maxCrossRefsKept)
	}
	for i, r := range got {
		if want := fmt.Sprintf("Psalms 119:%d", i+1); r.crossRef().label() != want {
			t.Fatalf("row %d is %q, want %q: the best rows, ties in the dataset's order", i, r.crossRef().label(), want)
		}
	}
	for key, rows := range idx {
		if cap(rows) != len(rows) {
			t.Errorf("%s's %d rows have capacity %d: an append to them would write over another verse's", key, len(rows), cap(rows))
		}
	}
	if size := unsafe.Sizeof(tskRow{}); size > 8 {
		t.Errorf("an index row takes %d bytes, want at most 8", size)
	}
	// The cap must clear the measured worst case with room to spare.
	if maxCrossRefsKept < crossRefDeepestRead+maxCrossRefsPerVerse/2 {
		t.Errorf("the index keeps %d rows a verse, too close to the %d a panel has been measured to read",
			maxCrossRefsKept, crossRefDeepestRead)
	}
}

// A row's books, numbers and votes survive packing, and a row naming a
// number past any verse is not kept.
func TestAnIndexRowUnpacksToTheRowParsed(t *testing.T) {
	for _, c := range []crossRef{
		{Book: "Genesis", Chapter: 1, Verse: 1, Votes: 369},
		{Book: "Psalms", Chapter: 150, Verse: 6, Votes: -86},
		{Book: "Psalms", Chapter: 119, Verse: 1, EndV: 176, Votes: 1290},
		{Book: "Romans", Chapter: 14, Verse: 24, EndCh: 0, EndV: 26},
		{Book: "Ruth", Chapter: 1, Verse: 22, EndCh: 2, EndV: 3},
		{Book: "2 John", Chapter: 1, Verse: 1, EndBook: "3 John", EndCh: 1, EndV: 14, Votes: 4},
	} {
		r, ok := packTSKRow(c)
		if !ok {
			t.Errorf("%s was not packed", c.label())
			continue
		}
		if got := r.crossRef(); got != c {
			t.Errorf("%+v unpacked as %+v", c, got)
		}
	}
	if _, ok := packTSKRow(crossRef{Book: "Genesis", Chapter: 1, Verse: 300}); ok {
		t.Error("a row naming verse 300 was kept")
	}
	if r, _ := packTSKRow(crossRef{Book: "Genesis", Chapter: 1, Verse: 1, Votes: 1 << 20}); r.votes != 1<<15-1 {
		t.Errorf("votes past an int16 packed as %d, want them held at %d", r.votes, 1<<15-1)
	}
}

// THE CORRECTIONS WERE MADE TO ONE COPY OF THE DATASET. The app downloads
// whatever copy OpenBible serves, with no version or checksum to pin it, and
// the hand-made corrections — Philippians 2:3 re-filed under 1:16, Matthew
// 23:13's rows filed under the reference's 23:14 — are right only while the
// rows they correct are as they were in the 2026-08-31 copy. Each checks
// that first. Where the rows have changed it corrects nothing, which leaves
// them where the dataset files them rather than somewhere a guess put them,
// and says so; the numbering moves (the doxology, 3 John 1:15) say so when
// the dataset stops naming the verse they move.
func TestACorrectionLeavesRowsThatHaveChangedAlone(t *testing.T) {
	const asMade = "Phil.1.16\t2Cor.2.17\t9\n" +
		"Phil.1.17\tActs.22.1\t9\n" +
		"Phil.1.17\tPhil.2.3\t4\n" +
		"Matt.23.13\tLuke.11.52\t20\n" +
		"Matt.23.13\tMatt.23.23\t9\n" +
		"Rom.16.25\tEph.3.20\t30\n" +
		"Rom.16.26\tRom.1.5\t10\n" +
		"Rom.16.27\tJude.1.25\t10\n" +
		"3John.1.15\tJohn.10.3\t1\n"
	read := func(rows string) (map[string][]string, []string) {
		t.Helper()
		idx, drift, err := parseCrossRefRows(strings.NewReader("From Verse\tTo Verse\tVotes\n" + rows))
		if err != nil {
			t.Fatal(err)
		}
		labels := map[string][]string{}
		for key, rs := range idx {
			for _, r := range rs {
				labels[key] = append(labels[key], r.crossRef().label())
			}
		}
		return labels, drift
	}
	has := func(labels map[string][]string, key, label string) bool {
		return slices.Contains(labels[key], label)
	}

	// CONTROL: the copy they were made for takes every correction, and
	// reports nothing.
	labels, drift := read(asMade)
	if len(drift) != 0 {
		t.Fatalf("control: the copy the corrections were made for reports %q", drift)
	}
	if !has(labels, "Philippians|1|16", "Philippians 2:3") || has(labels, "Philippians|1|17", "Philippians 2:3") {
		t.Fatalf("control: Philippians 2:3 is not re-filed under 1:16: %q", labels)
	}
	if !has(labels, "Matthew|23|14", "Luke 11:52") || len(labels["Matthew|23|13"]) != 0 {
		t.Fatalf("control: Matthew 23:13's rows are not filed under 23:14: %q", labels)
	}

	for _, tc := range []struct {
		name, rows, report string
		left               func(map[string][]string) bool
	}{
		{
			"1:16 has its own row to 2:3",
			asMade + "Phil.1.16\tPhil.2.3\t2\n", "Philippians 2:3 under 1:16",
			func(l map[string][]string) bool {
				return has(l, "Philippians|1|17", "Philippians 2:3") && slices.Equal(l["Philippians|1|16"], []string{"2 Corinthians 2:17", "Philippians 2:3"})
			},
		},
		{
			"Philippians 1:16-17 in the ESV's order, so 1:17 is no longer the defence",
			strings.Replace(asMade, "Phil.1.17\tActs.22.1\t9\n", "Phil.1.16\tActs.22.1\t9\n", 1), "Philippians 2:3 under 1:16",
			func(l map[string][]string) bool {
				return has(l, "Philippians|1|17", "Philippians 2:3") && !has(l, "Philippians|1|16", "Philippians 2:3")
			},
		},
		{
			"a Matthew 23:14 of the dataset's own",
			asMade + "Matt.23.14\tMark.12.40\t5\n", "Matthew 23:13 as a source",
			func(l map[string][]string) bool {
				return has(l, "Matthew|23|13", "Luke 11:52") && slices.Equal(l["Matthew|23|14"], []string{"Mark 12:40"})
			},
		},
		{
			"23:13's rows no longer the kingdom woe's",
			strings.Replace(asMade, "Matt.23.13\tLuke.11.52\t20\n", "Matt.23.13\tMark.12.40\t20\n", 1), "Matthew 23:13 as a source",
			func(l map[string][]string) bool {
				return has(l, "Matthew|23|13", "Mark 12:40") && len(l["Matthew|23|14"]) == 0
			},
		},
		{
			"3 John ends at 1:14",
			strings.Replace(asMade, "3John.1.15\tJohn.10.3\t1\n", "", 1), "3 John 1:15",
			func(l map[string][]string) bool { return len(l["3 John|1|14"]) == 0 },
		},
		{
			"the doxology without its 16:26",
			strings.Replace(asMade, "Rom.16.26\tRom.1.5\t10\n", "", 1), "Romans 16:26",
			func(l map[string][]string) bool { return has(l, "Romans|14|24", "Ephesians 3:20") },
		},
	} {
		labels, drift := read(tc.rows)
		if len(drift) != 1 || !strings.Contains(drift[0], tc.report) {
			t.Errorf("%s: reported %q, want one report naming %q", tc.name, drift, tc.report)
		}
		if !tc.left(labels) {
			t.Errorf("%s: the rows were not left as the dataset files them: %q", tc.name, labels)
		}
	}
}

// A correction that no longer applies is logged when the app builds the
// index, once: the index is built once a run.
func TestACorrectionThatNoLongerAppliesIsLoggedOnce(t *testing.T) {
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), cacheFileName))
	rows := "Phil.1.16\tPhil.2.3\t2\nPhil.1.17\tPhil.2.3\t4\nPhil.1.17\tActs.22.1\t9\n" +
		"Matt.23.13\tLuke.11.52\t20\nRom.16.25\tEph.3.20\t30\nRom.16.26\tRom.1.5\t10\n" +
		"Rom.16.27\tJude.1.25\t10\n3John.1.15\tJohn.10.3\t1\n"
	if err := os.WriteFile(crossRefCachePath(), makeCrossRefZip(t, rows), 0o644); err != nil {
		t.Fatal(err)
	}
	var logged []string
	crossRefMu.Lock()
	was := struct {
		loaded bool
		err    error
		index  map[string][]tskRow
		logf   func(string, ...any)
	}{crossRefLoaded, crossRefLoadErr, crossRefIndex, crossRefLogf}
	crossRefLoaded, crossRefLoadErr, crossRefIndex = false, nil, nil
	crossRefLogf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	crossRefMu.Unlock()
	t.Cleanup(func() {
		crossRefMu.Lock()
		crossRefLoaded, crossRefLoadErr, crossRefIndex, crossRefLogf = was.loaded, was.err, was.index, was.logf
		crossRefMu.Unlock()
	})

	for range 2 {
		if err := ensureCrossRefs(); err != nil {
			t.Fatal(err)
		}
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "Philippians 2:3 under 1:16") {
		t.Errorf("logged %q, want one line about Philippians 2:3", logged)
	}
	// CONTROL: the index was built from this copy, so the log is about it.
	if got := crossRefIndex[crossRefKey("Matthew", 23, 14)]; len(got) != 1 {
		t.Errorf("control: the index holds %d rows at Matthew 23:14, want the one filed at 23:13", len(got))
	}
}
