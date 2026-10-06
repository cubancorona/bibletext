package bibletext

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// crossRefDeepestRead, the furthest down a verse's rows any panel reads, is
// measured by the walk below and declared for each state of the next switch:
// greek_esther_current_test.go and greek_esther_next_test.go.

// crossRefDeepestReadable is the furthest down a verse's rows any selection
// holding the verse can read. Every such selection hides at least own, the
// labels of the verse's own parallels, and at most chapter, those of every
// parallel in the verse's chapter, since a selection lies within one chapter.
//
// The panel built with every parallel of the chapter hidden is not the
// deepest. Hiding more reads deeper while sixteen rows are still to be found,
// but where fewer than sixteen are left to show, the deepest row read is the
// last one shown, and hiding a neighbour's parallel can hide exactly that
// row. So a row counts here when it can be shown at all, that is when only
// the verse's own parallels are hidden, and fewer than sixteen rows above it
// are shown when every parallel of the chapter is: no selection shows fewer
// above it, and none hides less.
func crossRefDeepestReadable(rows []tskRow, resolve func(crossRef) []crossRef, own, chapter map[string]bool) int {
	deepest, above := 0, 0
	seen := map[string]bool{} // labels of the rows above, which no selection shows twice
	for i, r := range rows {
		if above >= maxCrossRefsPerVerse {
			break
		}
		shown, counted := false, false
		for _, c := range resolve(r.crossRef()) {
			lbl := c.label()
			if seen[lbl] {
				continue
			}
			seen[lbl] = true
			shown = shown || !own[lbl]
			counted = counted || !chapter[lbl]
		}
		if shown {
			deepest = i + 1
		}
		if counted {
			above++
		}
	}
	return deepest
}

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
// row builds, and no selection may need a row past the cap: the depth is
// measured for every selection that can hold the verse, from the one that
// hides only the verse's own parallels to the one that hides every parallel
// of its chapter (crossRefDeepestReadable).
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
				hidden := map[string]bool{}      // every parallel of the chapter
				own := map[int]map[string]bool{} // each verse's own
				for _, v := range verses {
					own[v.Verse] = map[string]bool{}
					if ch, vs, ok := crossRefSourceRef(id, v); ok {
						for _, p := range gospelParallelsForVerse(v.BookName, ch, vs) {
							for _, c := range resolve(p) {
								hidden[c.label()] = true
								own[v.Verse][c.label()] = true
							}
						}
					}
				}
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
						if endBook == c.Book && !verseBefore(verseRef{c.Book, c.Chapter, c.Verse}, verseRef{c.Book, endCh, c.EndV}) {
							report("%s, a range that does not run forwards", where)
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
						read := crossRefDeepestReadable(every[crossRefKey(v.BookName, ch, vs)], resolve, own[v.Verse], hidden)
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
		t.Errorf("a panel reads row %d of %s, deeper than the %d measured when maxCrossRefsKept was "+
			"chosen: re-judge the margin and update crossRefDeepestRead", deepest, deepestAt, crossRefDeepestRead)
	}
}

// The depth the walk measures holds for every selection, including the one
// the panel with every parallel of the chapter hidden misses: a verse with
// fewer than sixteen rows to show, whose last row is a neighbour's parallel.
// Hidden, as it is when the whole chapter is selected, the panel reads ten
// rows; the verse selected alone shows that row and reads twelve.
func TestTheWalksDepthHoldsForEverySelection(t *testing.T) {
	var rows []tskRow
	for v := 1; v <= 10; v++ {
		r, _ := packTSKRow(crossRef{Book: "Psalms", Chapter: 1, Verse: v})
		rows = append(rows, r)
	}
	for _, v := range []int{11, 12} { // the neighbours' parallels
		r, _ := packTSKRow(crossRef{Book: "Luke", Chapter: 9, Verse: v})
		rows = append(rows, r)
	}
	resolve := func(c crossRef) []crossRef { return []crossRef{c} }
	own := map[string]bool{"Mark 6:1": true}
	chapter := map[string]bool{"Mark 6:1": true, "Luke 9:11": true, "Luke 9:12": true}

	// Every selection hides own and something of the rest of the chapter.
	deepestOf := 0
	for _, extra := range [][]string{nil, {"Luke 9:11"}, {"Luke 9:12"}, {"Luke 9:11", "Luke 9:12"}} {
		hidden := map[string]bool{"Mark 6:1": true}
		for _, lbl := range extra {
			hidden[lbl] = true
		}
		if _, read := treasuryRowsFor(rows, resolve, hidden); read > deepestOf {
			deepestOf = read
		}
	}
	// CONTROL: the chapter-wide panel really does read less than a selection.
	if _, read := treasuryRowsFor(rows, resolve, chapter); read >= deepestOf {
		t.Fatalf("control: the chapter-wide panel reads row %d and the deepest selection row %d", read, deepestOf)
	}
	if got := crossRefDeepestReadable(rows, resolve, own, chapter); got != deepestOf {
		t.Errorf("the walk measures row %d, but a selection reads row %d", got, deepestOf)
	}

	// And where sixteen rows are still to be found, hiding more reads deeper:
	// twenty rows, the first two the neighbours' parallels.
	rows = append(rows[10:12:12], rows[:10]...)
	for v := 11; v <= 18; v++ {
		r, _ := packTSKRow(crossRef{Book: "Psalms", Chapter: 1, Verse: v})
		rows = append(rows, r)
	}
	_, wide := treasuryRowsFor(rows, resolve, chapter)
	if got := crossRefDeepestReadable(rows, resolve, own, chapter); got != wide || wide != 18 {
		t.Errorf("the walk measures row %d; the chapter-wide panel reads row %d, want 18", got, wide)
	}
}
