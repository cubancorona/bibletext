package bibletext

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// THE WHOLE TREASURY AGAINST THE DOWNLOADED TEXTS. The tests beside this one
// pin the mapping rules on synthetic rows; this one walks every verse of every
// translation the machine has downloaded, builds the cross-references panel
// for it, and checks what a reader would be shown: every row opens a verse the
// text has, every range ends on a verse the text has and after its start, and
// no label names a range that is really one verse. It is the check that found
// "Romans 16:25-25", "Leviticus 27:34-1:1" and "2 John 1:1-15", which no
// synthetic fixture had thought to contain.
//
// Opt-in, and read-only: it reads the machine's own caches through
// realCachePath (open the panel once in the app for the Treasury zip), and
// runs only when asked, because the walk builds some 120,000 panels.
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
	idx, err := parseCrossRefZip(zipBytes)
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
			// names must be one the reference prints.
			for key, rows := range idx {
				f := strings.Split(key, "|")
				var ch, v int
				fmt.Sscan(f[1], &ch)
				fmt.Sscan(f[2], &v)
				if bd.GetVerse(f[0], ch, v) == nil {
					t.Errorf("the index is keyed at %s %d:%d, which the reference does not print", f[0], ch, v)
				}
				for _, c := range rows {
					if bd.GetVerse(c.Book, c.Chapter, c.Verse) == nil {
						t.Errorf("a row from %s %d:%d points at %s, whose start the reference does not print", f[0], ch, v, c.label())
					}
				}
			}
		}
		panels, rows, bad := 0, 0, 0
		report := func(format string, args ...any) {
			bad++
			if bad <= 20 {
				t.Errorf(id+": "+format, args...)
			}
		}
		for _, book := range bd.Books {
			for _, chapter := range bd.GetChapterNumbersForBook(book) {
				for _, v := range bd.GetChapter(book, chapter) {
					st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter, CurrentVersion: id}
					panels++
					for _, c := range crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse}) {
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
				}
			}
		}
		t.Logf("%s: %d panels, %d rows, %d faults", id, panels, rows, bad)
		if panels == 0 {
			t.Errorf("%s: no panels were built; the walk proves nothing", id)
		}
	}
	if walked == 0 {
		t.Skip("no downloaded translation in the machine's cache")
	}
}
