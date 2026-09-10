package bibletext

// The verse-of-the-day card shares a passage the reader is NOT on. Every stage
// of the selection route reads the reader's current chapter, so pushing the
// card's verse through it with the reader on Matthew 5 located nothing there
// and cited "Matthew 5" for Psalm 23:1; the bare alternative, formatBibleQuote
// of the verse text, skips normalizeShareSelection's trailing clause-mark strip
// and ships "…God; . . . ." for a verse ending in ";". The passage route
// (shareQuoteForPassage) must give, for every verse and every short range,
// exactly what selecting it in its own chapter gives — quote and citation —
// while the reader stays where they are. Each test names the mutation it was
// written against and was watched to fail under it.

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// selectionRouteRaw is the text a native pane hands the selection route for
// verses lo..hi: each verse's superscript number as a leading token, its text
// with the source newlines, one verse per line. An empty verse renders nothing
// and is skipped, as the pane skips it.
func selectionRouteRaw(verses []Verse, lo, hi int) string {
	var parts []string
	for _, v := range verses {
		if v.Verse < lo || v.Verse > hi || strings.TrimSpace(v.Text) == "" {
			continue
		}
		parts = append(parts, strconv.Itoa(v.Verse)+" "+v.Text)
	}
	return strings.Join(parts, "\n")
}

// selectionRouteShare is shareVerse's text route for verses lo..hi of
// book/chapter with the reader ON that chapter, run through the bare
// (reader's-chapter) spellings every existing share test holds to — the
// reference the passage route is measured against, computed without it.
// ok=false is a selection of nothing, which dispatchSelectionAction drops.
func selectionRouteShare(bd *BibleData, book string, chapter, lo, hi int) (quote, cite string, ok bool) {
	raw := selectionRouteRaw(bd.GetChapter(book, chapter), lo, hi)
	if raw == "" {
		return "", "", false
	}
	st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter}
	span := selSpanFromNative(lo, hi)
	cleaned, cite, at, baseLen := prepareShareQuote(st, raw, span)
	terminal := originalSentenceTerminal(st, cleaned, at, baseLen)
	cleaned = restoreShareLineBreaks(st, cleaned, at, baseLen)
	return formatBibleQuote(cleaned, terminal), cite, true
}

// readerElsewhere picks a chapter of bd other than book/chapter for the card
// state's reader to be on — a real one where the fixture has a second, so a
// route that read the reader's chapter would find prose there and cite it.
func readerElsewhere(bd *BibleData, book string, chapter int) (string, int) {
	for _, b := range bd.Books {
		for _, c := range bd.GetChapterNumbersForBook(b) {
			if b != book || c != chapter {
				return b, c
			}
		}
	}
	return "Elsewhere", 1
}

// fragmentShapesBible reproduces, in one chapter, the verse shapes the
// pipeline treats differently from a bare formatBibleQuote: a verse ending in
// ";" (v1) and in "," (v2); one that stops mid-sentence with no punctuation
// (v3), whose sentence completes in the two-line poem verse after it (v4);
// a sentence that completes in the next verse (v5, v6); a paragraph opening
// at v7, so a range across it carries a paragraph break; and a question cut
// short (v9), whose terminal sits in v10.
func fragmentShapesBible() *BibleData {
	mk := func(verse int, text string, para bool) Verse {
		return Verse{BookName: "Isaiah", Book: "Isaiah", Chapter: 40, Verse: verse, Text: text, ParaStart: para}
	}
	return &BibleData{
		Books: []string{"Isaiah"},
		Verses: map[string]map[int][]Verse{"Isaiah": {40: {
			mk(1, "All flesh is grass, and all its glory is like the flower of the field;", false),
			mk(2, "The grass withers, the flower fades,", false),
			mk(3, "when the breath of the LORD blows on it", false),
			mk(4, "surely the people are grass.\nThe grass withers, the flower fades.", false),
			mk(5, "He will feed his flock like a shepherd", false),
			mk(6, "and gather the lambs in his arms.", false),
			mk(7, "Who has measured the waters in the hollow of his hand?", true),
			mk(8, "Do you not know? Have you not heard?", false),
			mk(9, "Have you not heard that the everlasting God", false),
			mk(10, "does not faint or grow weary?", false),
		}}},
	}
}

// passageFixtures is every Bible the suite can build without a download: the
// sample verses, the cache fixture, the edge chapter with an empty verse, the
// two repeated-wording chapters (the span must pick the passage's copy), and
// the fragment shapes.
func passageFixtures() map[string]*BibleData {
	return map[string]*BibleData{
		"sample":         sampleState().Bible,
		"testBibleData":  testBibleData(),
		"john3Edge":      john3EdgeState().Bible,
		"twoCopy":        twoCopyState().Bible,
		"refrain":        refrainChapterState().Bible,
		"fragmentShapes": fragmentShapesBible(),
	}
}

// comparePassageRoutes runs both routes over every verse of every chapter of
// bd, as a single verse and as ranges of two to five verses (the rotation
// carries passages of up to five), with the card state's
// reader on a different chapter. It reports the first few mismatches in full
// and returns the totals so a caller can fail on the rest.
func comparePassageRoutes(t *testing.T, name string, bd *BibleData) (compared, mismatched int) {
	t.Helper()
	const report = 12
	for _, book := range bd.Books {
		for _, chapter := range bd.GetChapterNumbersForBook(book) {
			verses := bd.GetChapter(book, chapter)
			rb, rc := readerElsewhere(bd, book, chapter)
			for i := range verses {
				for width := 1; width <= 5 && i+width <= len(verses); width++ {
					lo, hi := verses[i].Verse, verses[i+width-1].Verse
					if hi < lo {
						continue
					}
					wantQuote, wantCite, wantOK := selectionRouteShare(bd, book, chapter, lo, hi)
					card := &AppState{Bible: bd, CurrentBook: rb, CurrentChapter: rc}
					quote, cite, ok := shareQuoteForPassage(card, book, chapter, lo, hi)
					if card.CurrentBook != rb || card.CurrentChapter != rc {
						t.Fatalf("%s: sharing %s %d:%d–%d moved the reader from %s %d to %s %d",
							name, book, chapter, lo, hi, rb, rc, card.CurrentBook, card.CurrentChapter)
					}
					compared++
					if ok == wantOK && quote == wantQuote && cite == wantCite {
						continue
					}
					mismatched++
					if mismatched <= report {
						t.Errorf("%s: %s %d:%d–%d (reader on %s %d)\n card: ok=%v %q — %q\n  sel: ok=%v %q — %q",
							name, book, chapter, lo, hi, rb, rc, ok, quote, cite, wantOK, wantQuote, wantCite)
					}
				}
			}
		}
	}
	if mismatched > report {
		t.Errorf("%s: %d of %d passages differ between the routes (first %d shown)", name, mismatched, compared, report)
	}
	return compared, mismatched
}

// The decisive test: over every fixture the CI can load, the card route equals
// the selection route — same quote, same citation — for a single verse and for
// a 2- and 3-verse range, with the reader on another chapter. Mutation:
// shareQuoteForPassage reading state.CurrentBook/CurrentChapter in place of
// book/chapter (the defect): the locate fails in the reader's chapter and the
// legacy citation names it. The same walk fails under a stage falling back to
// its bare spelling (restoreShareLineBreaksIn through chapterShareStructure:
// Isaiah 40:4 loses its poem line) and under a bare formatBibleQuote of the
// verse text (the ";" and "," verses keep their clause mark ahead of the
// four-dot mark — the control below proves that difference is visible here).
func TestPassageShareEqualsSelectionRoute(t *testing.T) {
	total := 0
	for name, bd := range passageFixtures() {
		compared, _ := comparePassageRoutes(t, name, bd)
		if compared == 0 {
			t.Errorf("%s: no passages compared — the fixture is empty", name)
		}
		total += compared
	}
	t.Logf("%d passages compared across %d fixtures", total, len(passageFixtures()))
}

// The control that proves the decisive test can see the defect it exists for:
// on a verse ending in ";" the bare route — formatBibleQuote of the trimmed
// verse text, which skips normalizeShareSelection's trailing clause-mark strip
// (the orphan-punctuation contract in share_edge_test.go) — DIFFERS from the
// selection route, carrying the ";" into the four-dot mark. Mutation:
// shareQuoteForPassage returning formatBibleQuote(strings.TrimSpace(text)).
func TestPassageShareControlBareFormatKeepsTheClauseMark(t *testing.T) {
	for _, tc := range []struct {
		bd             *BibleData
		book           string
		chapter, verse int
	}{
		{fragmentShapesBible(), "Isaiah", 40, 1},
		{sampleState().Bible, "Romans", 3, 23}, // "…fall short of the glory of God;"
	} {
		v := tc.bd.GetVerse(tc.book, tc.chapter, tc.verse)
		if v == nil {
			t.Fatalf("%s %d:%d missing from the fixture", tc.book, tc.chapter, tc.verse)
		}
		bare := formatBibleQuote(strings.TrimSpace(v.Text))
		want, _, ok := selectionRouteShare(tc.bd, tc.book, tc.chapter, tc.verse, tc.verse)
		if !ok {
			t.Fatalf("%s %d:%d: the selection route shares nothing", tc.book, tc.chapter, tc.verse)
		}
		if bare == want {
			t.Errorf("%s %d:%d: control failed — the bare route agrees with the selection route (%q), so the decisive test could not see the defect",
				tc.book, tc.chapter, tc.verse, bare)
		}
		if !strings.Contains(bare, "; . . . .") {
			t.Errorf("%s %d:%d: the bare route should carry the clause mark into the ellipsis: %q", tc.book, tc.chapter, tc.verse, bare)
		}
		if strings.Contains(want, ";") {
			t.Errorf("%s %d:%d: the selection route should have dropped the clause mark: %q", tc.book, tc.chapter, tc.verse, want)
		}
		card := &AppState{Bible: tc.bd, CurrentBook: "Elsewhere", CurrentChapter: 1}
		got, _, ok := shareQuoteForPassage(card, tc.book, tc.chapter, tc.verse, tc.verse)
		if !ok || got != want {
			t.Errorf("%s %d:%d: card route = %q (ok=%v), want the selection route's %q", tc.book, tc.chapter, tc.verse, got, ok, want)
		}
	}
}

// The defect, pinned in its reported shape: the reader on Matthew 5, Psalm
// 23:1 on the card. Mutation: shareQuoteForPassage reading the reader's
// chapter — the locate fails in Matthew 5 and the legacy citation names
// "Matthew 5". The fragment shapes are pinned by value beside it so the
// decisive test's equality cannot be satisfied by both routes being wrong the
// same way: the clause mark dropped, the poem line kept, the question's own
// terminal in the four-dot slot, the range across the paragraph opening.
func TestPassageShareCitesThePassageNotTheReadersChapter(t *testing.T) {
	st := sampleState()
	st.CurrentBook, st.CurrentChapter = "Matthew", 5
	quote, cite, ok := shareQuoteForPassage(st, "Psalms", 23, 1, 1)
	if !ok || cite != "Psalms 23:1" {
		t.Errorf("cite = %q (ok=%v), want Psalms 23:1", cite, ok)
	}
	if want := "“Yahweh is my shepherd: I shall lack nothing.”"; quote != want {
		t.Errorf("quote = %q, want %q", quote, want)
	}
	if st.CurrentBook != "Matthew" || st.CurrentChapter != 5 {
		t.Errorf("the share moved the reader to %s %d", st.CurrentBook, st.CurrentChapter)
	}

	fx := &AppState{Bible: fragmentShapesBible(), CurrentBook: "Elsewhere", CurrentChapter: 1}
	for _, tc := range []struct {
		lo, hi      int
		quote, cite string
	}{
		{1, 1, "“All flesh is grass, and all its glory is like the flower of the field . . . .”", "Isaiah 40:1"},
		{2, 2, "“The grass withers, the flower fades . . . .”", "Isaiah 40:2"},
		{3, 3, "“[W]hen the breath of the LORD blows on it . . . .”", "Isaiah 40:3"},
		{4, 4, "“[S]urely the people are grass.\nThe grass withers, the flower fades.”", "Isaiah 40:4"},
		{5, 5, "“He will feed his flock like a shepherd . . . .”", "Isaiah 40:5"},
		{5, 6, "“He will feed his flock like a shepherd and gather the lambs in his arms.”", "Isaiah 40:5–6"},
		{6, 7, "“[A]nd gather the lambs in his arms.\n\nWho has measured the waters in the hollow of his hand?”", "Isaiah 40:6–7"},
		{9, 9, "“Have you not heard that the everlasting God . . . ?”", "Isaiah 40:9"},
	} {
		quote, cite, ok := shareQuoteForPassage(fx, "Isaiah", 40, tc.lo, tc.hi)
		if !ok || quote != tc.quote || cite != tc.cite {
			t.Errorf("Isaiah 40:%d–%d: got (%q, %q, ok=%v), want (%q, %q)", tc.lo, tc.hi, quote, cite, ok, tc.quote, tc.cite)
		}
	}
	if fx.CurrentBook != "Elsewhere" || fx.CurrentChapter != 1 {
		t.Errorf("the share moved the reader to %s %d", fx.CurrentBook, fx.CurrentChapter)
	}
}

// sharePassageText hands over composeShareText(quote, cite, version) — the
// selection route's message byte for byte, the translation named in full —
// and hands over nothing when there is nothing to share. Mutation:
// sharePassageMessage composing without the blank line, or naming the
// version by its abbreviation.
func TestPassageShareMessageIsTheSelectionRoutesMessage(t *testing.T) {
	st := sampleState()
	st.CurrentBook, st.CurrentChapter = "Matthew", 5
	msg, ok := sharePassageMessage(st, "Psalms", 23, 1, 2)
	wantQuote, wantCite, _ := selectionRouteShare(st.Bible, "Psalms", 23, 1, 2)
	want := composeShareText(wantQuote, wantCite, st.currentVersion().Name)
	if !ok || msg != want {
		t.Errorf("message:\n got %q (ok=%v)\nwant %q", msg, ok, want)
	}
	if !strings.HasSuffix(msg, "\n\n— Psalms 23:1–2 (World English Bible)") {
		t.Errorf("message must end on the blank line and the full citation line: %q", msg)
	}
	if msg, ok := sharePassageMessage(st, "Psalms", 23, 40, 41); ok || msg != "" {
		t.Errorf("verses the chapter lacks: got (%q, ok=%v), want nothing", msg, ok)
	}
	if msg, ok := sharePassageMessage(st, "Psalms", 23, 2, 1); ok || msg != "" {
		t.Errorf("inverted range: got (%q, ok=%v), want nothing", msg, ok)
	}
	if _, ok := sharePassageMessage(nil, "Psalms", 23, 1, 1); ok {
		t.Error("nil state must share nothing")
	}
}

// TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly is the
// decisive walk over every verse of every chapter of the three public-domain
// translations whose caches the app keeps in the user cache directory (web,
// bsb, webc — found the way the app's own migration finds them: the current
// epoch's file, else the newest superseded one). LOCAL-ONLY, and the name says
// so: the caches are downloaded translation data, not repository content, so a
// CI checkout has none of them. A translation with no cache present is
// skipped; one whose cache is present but will not load is an error, since
// that is not absence. Opt-in by BIBLETEXT_EXHAUSTIVE=1: it adds a minute to
// every run, and the CI-runnable tests above already carry the fixtures that
// reproduce every fragment shape it would find.
func TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: walks three ~30k-verse translation caches (a minute); " +
			"run with BIBLETEXT_EXHAUSTIVE=1")
	}
	walked := 0
	for _, id := range []string{"web", "bsb", "webc", "nkjv"} {
		v, known := versionByID(id)
		if !known {
			t.Fatalf("version %q is not in the catalogue", id)
		}
		var bd *BibleData
		for _, path := range append([]string{cachePathForVersion(id)}, supersededCachePaths(v)...) {
			if _, err := os.Stat(path); err != nil {
				continue
			}
			loaded, err := loadBibleFromCache(path)
			if err != nil {
				t.Errorf("%s: a cache is present but will not load: %v", id, err)
				continue
			}
			bd = loaded
			break
		}
		if bd == nil {
			t.Logf("%s: no cache present — skipped", id)
			continue
		}
		walked++
		compared, mismatched := comparePassageRoutes(t, id, bd)
		t.Logf("%s: %d passages compared, %d differ", id, compared, mismatched)
	}
	if walked == 0 {
		t.Skip("local-only: no translation cache present")
	}
}

// sharePassageText's own guard: an empty or inverted range hands NOTHING to
// the platform, rather than an empty message. Mutation: drop the ok check
// before shareTextOut.
func TestSharePassageTextDeliversNothingForAnEmptyRange(t *testing.T) {
	calls := 0
	prev := shareTextOut
	shareTextOut = func(string) { calls++ }
	t.Cleanup(func() { shareTextOut = prev })
	st := &AppState{Bible: fragmentShapesBible()}
	book := st.Bible.Books[0]
	chapter := st.Bible.GetChapterNumbersForBook(book)[0]
	first := st.Bible.GetChapter(book, chapter)[0].Verse
	sharePassageText(st, book, chapter, first+1, first) // inverted
	sharePassageText(st, book, chapter, 999, 999)       // absent
	if calls != 0 {
		t.Errorf("an unshareable range still reached the platform %d time(s)", calls)
	}
	sharePassageText(st, book, chapter, first, first)
	if calls != 1 {
		t.Errorf("a shareable passage reached the platform %d time(s), not once — the control", calls)
	}
}
