package bibletext

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// crossRefDeepestRead is the retained read-depth regression ceiling for the
// downloaded-corpus walk. These 18/21-row budgets leave space below the
// 32-row index cap; the walk measures the current source-local pipeline
// directly rather than inferring the effects of other displayed sources.
var crossRefDeepestRead = func() int {
	if nextRelease {
		return 21
	}
	return 18
}()

// THE WHOLE TREASURY AGAINST THE DOWNLOADED TEXTS. The tests beside this one
// pin the mapping rules on synthetic rows; this one walks every verse of every
// translation the machine has downloaded, builds the cross-references panel
// for it, and checks what a reader would be shown: every row opens a verse the
// text has, every range ends on a verse the text has and after its start, and
// no label names a range that is really one verse. It is the check that found
// "Romans 16:25-25", "Leviticus 27:34-1:1" and "2 John 1:1-15", which no
// synthetic fixture had thought to contain.
//
// It fails, first, if a hand-made correction to the dataset no longer
// applies to the copy the machine downloaded (crossRefCorrection). And it
// holds the index's cap to the panels. The index keeps each verse's best
// maxCrossRefsKept rows; every panel must be the one an index keeping every
// row builds, and no selection may need a row past the cap. Each source
// verse contributes independently, so treasuryRowsFor's last contributing
// row measures that origin's depth for every selection containing it.
//
// Opt-in, and read-only: it reads the machine's own caches through
// realCachePath (open the panel once in the app for the Treasury zip), and
// runs only when asked, because the walk builds some 250,000 panels.
//
//	BIBLETEXT_XREF_REALDATA=1 go test -run TestEveryCrossReferenceRowOpensScriptureTheTextHas -v .
//
// It prints references and counts only, never verse text.
func TestEveryCrossReferenceRowOpensScriptureTheTextHas(t *testing.T) {
	if os.Getenv("BIBLETEXT_XREF_REALDATA") == "" {
		t.Skip("set BIBLETEXT_XREF_REALDATA=1 to walk the downloaded texts")
	}
	zipBytes, err := os.ReadFile(realCachePath(crossRefCachePath()))
	if err != nil {
		t.Skipf("no Treasury zip in the machine's cache: %v", err)
	}
	idx, drift, err := parseCrossRefZip(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	// The hand-made corrections were made to one copy of the file, and the
	// app downloads whatever copy OpenBible serves. Each correction checks
	// that the rows it corrects are still as they were, and leaves them alone
	// if not; that is safe for the reader but leaves the rows as the dataset
	// files them, so here it fails.
	for _, d := range drift {
		t.Errorf("a correction to the dataset no longer applies: %s", d)
	}
	tsv, err := crossRefTSV(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	every, _, err := readCrossRefRows(tsv, 0)
	tsv.Close()
	if err != nil {
		t.Fatal(err)
	}
	use := func(index map[string][]tskRow) {
		crossRefMu.Lock()
		crossRefIndex = index
		crossRefMu.Unlock()
	}
	was := crossRefIndex
	use(idx)
	t.Cleanup(func() { use(was) })

	deepest, deepestAt := 0, ""
	walked := 0
	for _, id := range []string{"web", "bsb", "webc", "nkjv"} {
		bd, err := loadBibleFromCache(realCachePath(cachePathForVersion(id)))
		if err != nil {
			t.Logf("%s: not in the machine's cache, skipped (%v)", id, err)
			continue
		}
		walked++
		if id == versificationReference {
			// The index speaks the reference's numbering, so every verse it
			// names must be one the reference prints, or one a move points
			// rows at that it prints only in a footnote, on a verse it prints
			// (crossRefFootnotedVerses).
			for key, rows := range every {
				f := strings.Split(key, "|")
				var ch, v int
				fmt.Sscan(f[1], &ch)
				fmt.Sscan(f[2], &v)
				if bd.GetVerse(f[0], ch, v) == nil {
					t.Errorf("the index is keyed at %s %d:%d, which the reference does not print", f[0], ch, v)
				}
				for _, r := range rows {
					c := r.crossRef()
					if bd.GetVerse(c.Book, c.Chapter, c.Verse) != nil {
						continue
					}
					if home, ok := crossRefFootnotedVerses[verseRef{c.Book, c.Chapter, c.Verse}]; ok && c.EndV == 0 &&
						bd.GetVerse(home.Book, home.Chapter, home.Verse) != nil {
						continue
					}
					t.Errorf("a row from %s %d:%d points at %s, whose start the reference does not print", f[0], ch, v, c.label())
				}
			}
		}
		panels, rows, bad, deepestHere := 0, 0, 0, 0
		report := func(format string, args ...any) {
			bad++
			if bad <= 20 {
				t.Errorf(id+": "+format, args...)
			}
		}
		resolve := crossRefResolver(bd.Books, id)
		for _, book := range bd.Books {
			for _, chapter := range bd.GetChapterNumbersForBook(book) {
				verses := bd.GetChapter(book, chapter)
				for _, v := range verses {
					st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: id}
					panels++
					panel := crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse})
					for _, c := range panel {
						rows++
						where := fmt.Sprintf("%s %d:%d lists %q", book, chapter, v.Verse, c.label())
						if bd.GetVerse(c.Book, c.Chapter, c.Verse) == nil {
							report("%s, whose first verse the text does not have", where)
						}
						if c.EndV == 0 {
							continue
						}
						endBook, endCh := c.Book, c.EndCh
						if c.EndBook != "" {
							endBook = c.EndBook
						}
						if endCh == 0 {
							endCh = c.Chapter
						}
						if bd.GetVerse(endBook, endCh, c.EndV) == nil {
							report("%s, whose last verse the text does not have", where)
						}
						if _, _, ok := c.canonBounds(); !ok {
							report("%s, a range that does not run forwards in canonical order", where)
						}
					}

					use(every)
					full := crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse})
					use(idx)
					if !slices.Equal(panel, full) {
						report("%s %d:%d lists %d rows from the index and %d from every row the dataset has",
							book, chapter, v.Verse, len(panel), len(full))
					}
					if ch, vs, ok := crossRefSourceRef(id, v); ok {
						_, read := treasuryRowsFor(every[crossRefKey(v.BookName, ch, vs)], resolve)
						if read > deepestHere {
							deepestHere = read
						}
						if read > deepest {
							deepest, deepestAt = read, fmt.Sprintf("%s %s %d:%d", id, book, chapter, v.Verse)
						}
					}
				}
			}
		}
		t.Logf("%s: %d panels, %d rows, %d faults; deepest read row %d", id, panels, rows, bad, deepestHere)
		if panels == 0 {
			t.Errorf("%s: no panels were built; the walk proves nothing", id)
		}
	}
	if walked == 0 {
		t.Skip("no downloaded translation in the machine's cache")
	}
	t.Logf("deepest read: row %d, %s; the index keeps %d", deepest, deepestAt, maxCrossRefsKept)
	if deepest > maxCrossRefsKept {
		t.Errorf("a panel reads row %d of %s, past the %d the index keeps: raise maxCrossRefsKept",
			deepest, deepestAt, maxCrossRefsKept)
	}
	if deepest > crossRefDeepestRead {
		t.Errorf("a panel reads row %d of %s, past the %d-row regression ceiling: re-judge the index margin",
			deepest, deepestAt, crossRefDeepestRead)
	}
}

// Read depth is the last source row that contributes a distinct destination,
// not the number of emitted cards or the number of rows examined. Other
// source sections have no role in this per-origin limit.
func TestTreasuryReadDepthTracksContributingSourceRows(t *testing.T) {
	pack := func(refs ...crossRef) []tskRow {
		t.Helper()
		rows := make([]tskRow, 0, len(refs))
		for _, ref := range refs {
			row, ok := packTSKRow(ref)
			if !ok {
				t.Fatalf("invalid synthetic reference: %+v", ref)
			}
			rows = append(rows, row)
		}
		return rows
	}
	identity := func(c crossRef) []crossRef { return []crossRef{c} }
	var refs []crossRef
	for v := 1; v <= 10; v++ {
		refs = append(refs, crossRef{Book: "Psalms", Chapter: 1, Verse: v})
	}
	refs = append(refs, crossRef{Book: "Luke", Chapter: 9, Verse: 11}, crossRef{Book: "Luke", Chapter: 9, Verse: 12})
	rows := pack(refs...)
	got, read := treasuryRowsFor(rows, identity)
	if read != 12 || !slices.Equal(got, refs) {
		t.Fatalf("uncapped source = %d rows at depth %d; want all 12", len(got), read)
	}

	// References that also occur as Gospel parallels still occupy their
	// ranked Treasury places; the sixteenth source row closes the cap.
	refs = append(append([]crossRef(nil), refs[10:12]...), refs[:10]...)
	for v := 11; v <= 18; v++ {
		refs = append(refs, crossRef{Book: "Psalms", Chapter: 1, Verse: v})
	}
	got, read = treasuryRowsFor(pack(refs...), identity)
	if read != 16 || !slices.Equal(got, refs[:16]) {
		t.Fatalf("capped source = %d rows at depth %d; want the first 16", len(got), read)
	}

	// A dropped leading row and a duplicate consume no cap places. A
	// dropped trailing row also cannot inflate the last contributing depth.
	resolve := func(c crossRef) []crossRef {
		if c.Book == "Mark" {
			return nil
		}
		return []crossRef{c}
	}
	filtered := []crossRef{
		{Book: "Mark", Chapter: 1, Verse: 1},
		{Book: "Psalms", Chapter: 1, Verse: 1},
		{Book: "Psalms", Chapter: 1, Verse: 1},
		{Book: "Psalms", Chapter: 1, Verse: 2},
		{Book: "Mark", Chapter: 1, Verse: 2},
	}
	got, read = treasuryRowsFor(pack(filtered...), resolve)
	if read != 4 || !slices.Equal(got, []crossRef{filtered[1], filtered[3]}) {
		t.Fatalf("filtered source = %v at depth %d; want two distinct rows at depth 4", got, read)
	}
}

// EVERY ROW OF THE TREASURY DATASET IS ACCOUNTED FOR. Mirrors
// publisher_xrefs_census_test.go for the OpenBible Treasury zip: counts raw TSV
// lines against parsed rows (zero silent parse/pack drops), pins the kept/capped
// index split, and verifies for every verse across all downloaded editions that
// every dataset row is accounted for as unmappable, duplicate, shown in
// places 1..16, or past the 16-row cap, matching crossRefsForSelection's
// Treasury slice row for row.
func TestEveryTreasuryDatasetRowIsAccountedFor(t *testing.T) {
	if os.Getenv("BIBLETEXT_XREF_REALDATA") == "" {
		t.Skip("set BIBLETEXT_XREF_REALDATA=1 to account for the downloaded Treasury")
	}
	zipBytes, err := os.ReadFile(realCachePath(crossRefCachePath()))
	if err != nil {
		t.Skipf("no Treasury zip in the machine's cache: %v", err)
	}

	rc, err := crossRefTSV(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	rawLines := 0
	sc := bufio.NewScanner(rc)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "From") || strings.HasPrefix(line, "#") {
			continue
		}
		rawLines++
	}
	if err := sc.Err(); err != nil {
		rc.Close()
		t.Fatal(err)
	}
	rc.Close()

	rc2, err := crossRefTSV(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	every, drift, err := readCrossRefRows(rc2, 0)
	rc2.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		t.Errorf("correction drift: %s", d)
	}
	parsedTotal := 0
	for _, rows := range every {
		parsedTotal += len(rows)
	}
	if rawLines != parsedTotal || rawLines != 344799 || len(every) != 29363 {
		t.Fatalf("rawLines=%d parsedTotal=%d keys=%d; want 344,799 rows across 29,363 source keys (0 dropped)",
			rawLines, parsedTotal, len(every))
	}

	idx, _, err := parseCrossRefZip(zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	keptTotal := 0
	for _, rows := range idx {
		keptTotal += len(rows)
	}
	if keptTotal != 337179 || parsedTotal-keptTotal != 7620 {
		t.Errorf("kept=%d past32=%d; want 337,179 kept and 7,620 past maxCrossRefsKept=%d",
			keptTotal, parsedTotal-keptTotal, maxCrossRefsKept)
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

	walked := 0
	for _, id := range []string{"web", "bsb", "webc", "nkjv"} {
		bd, err := loadBibleFromCache(realCachePath(cachePathForVersion(id)))
		if err != nil {
			continue
		}
		walked++
		resolve := crossRefResolver(bd.Books, id)
		var unmappable, duplicate, shownPlaces, pastCap, emittedRows, checkedVerses int
		for _, book := range bd.Books {
			for _, chapter := range bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: id}
				for _, v := range bd.GetChapter(book, chapter) {
					checkedVerses++
					var wantTSK []crossRef
					if srcCh, srcV, ok := crossRefSourceRef(id, v); ok {
						srcRows := every[crossRefKey(v.BookName, srcCh, srcV)]
						vUnmap, vHidDup, vShown, vPast := 0, 0, 0, 0
						given := map[string]bool{}
						for _, r := range srcRows {
							if vShown == maxCrossRefsPerVerse {
								vPast++
								continue
							}
							resolved := resolve(r.crossRef())
							if len(resolved) == 0 {
								vUnmap++
								continue
							}
							took := false
							for _, c := range resolved {
								lbl := c.label()
								if given[lbl] {
									continue
								}
								given[lbl] = true
								c.SourceVerse = v.Verse
								c.SourceLabel = (crossRef{Book: v.BookName, Chapter: v.Chapter, Verse: v.Verse}).label()
								wantTSK = append(wantTSK, c)
								took = true
							}
							if took {
								vShown++
							} else {
								vHidDup++
							}
						}
						if vUnmap+vHidDup+vShown+vPast != len(srcRows) {
							t.Fatalf("%s %s %d:%d accounting mismatch: %d+%d+%d+%d != %d",
								id, book, chapter, v.Verse, vUnmap, vHidDup, vShown, vPast, len(srcRows))
						}
						unmappable += vUnmap
						duplicate += vHidDup
						shownPlaces += vShown
						pastCap += vPast
					}
					var gotTSK []crossRef
					for _, c := range crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse}) {
						if !c.Parallel {
							gotTSK = append(gotTSK, c)
						}
					}
					if !slices.Equal(gotTSK, wantTSK) {
						t.Fatalf("%s %s %d:%d Treasury slice = %+v, want %+v", id, book, chapter, v.Verse, gotTSK, wantTSK)
					}
					emittedRows += len(gotTSK)
				}
			}
		}
		t.Logf("%s (%d verses): shownPlaces=%d (emitted=%d), unmappable=%d, duplicate=%d, pastCap16=%d",
			id, checkedVerses, shownPlaces, emittedRows, unmappable, duplicate, pastCap)
	}
	if walked == 0 {
		t.Skip("no downloaded translation in the machine's cache")
	}
}
