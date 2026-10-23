package bibletext

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// An omitted verse's mark lights when a link or a note names that verse
// (docs/ROADMAP.md, decision 18). The BSB's Matthew 17 omits verse 21 and
// draws "[21]" before verse 22's number; an /nkjv/ link to 17:21 opened there
// without the NKJV landed beside an unlit mark, so the reader could not see
// where the link pointed. The mark stands for the omitted verse alone: a span
// that names the verse after the hole, and not the hole, leaves it bare.
//
// omittedVerseLinkState (omitted_verse_arrival_test.go) is the fixture: the
// BSB's Matthew 17:19-23 without verse 21, a link mark set from the NKJV.

// markShape is what an HTML page writes around the "[21]" mark: whether the
// joining space before it, the mark itself, the space after it, and verse
// 22's number are inside the wash, read through the model of the Apple
// importer (reading_tint_wash_shape_test.go).
type markShape struct{ join, mark, after, next bool }

func (s markShape) String() string {
	return fmt.Sprintf("join %v, mark %v, space after %v, verse 22 %v", s.join, s.mark, s.after, s.next)
}

func appleMarkShape(t *testing.T, html string) markShape {
	t.Helper()
	doc := importChapterHTML(t, html)
	text := string(doc.text)
	i := strings.Index(text, "[21]")
	if i < 0 {
		t.Fatalf("the page has no [21] mark:\n%s", html)
	}
	at := len([]rune(text[:i]))
	n22 := doc.locOf(22)
	if at < 1 || at+5 >= len(doc.text) || n22 < 0 {
		t.Fatalf("the mark stands at %d, verse 22 at %d, in %q", at, n22, text)
	}
	return markShape{join: doc.hl[at-1], mark: doc.hl[at] && doc.hl[at+3], after: doc.hl[at+4], next: doc.hl[n22]}
}

// androidMarkShape reads the same four facts off the Android dialect, which
// inlines a background span around each washed piece.
func androidMarkShape(t *testing.T, html string) markShape {
	t.Helper()
	bg := `<span style="background-color:#[0-9a-f]{6}">`
	join := regexp.MustCompile(bg + ` </span>` + `(?:` + bg + `)?<btgap>`)
	lit := regexp.MustCompile(bg + `<btgap><font color="#[0-9a-f]{6}">\[21\]</font></btgap>( </span>|</span> )`)
	plain := regexp.MustCompile(`<btgap><font color="#[0-9a-f]{6}">\[21\]</font></btgap> `)
	num := regexp.MustCompile(bg + `<sup><btnum><font color="#[0-9a-f]{6}"><b>22</b>`)
	var s markShape
	s.join = join.MatchString(html)
	if m := lit.FindStringSubmatch(html); m != nil {
		s.mark, s.after = true, m[1] == " </span>"
	} else if !plain.MatchString(html) {
		t.Fatalf("the Android page has no [21] mark of either shape:\n%s", html)
	}
	s.next = num.MatchString(html)
	return s
}

func TestAMarkNamingAnOmittedVerseLightsItsMark(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	for _, c := range []struct {
		name   string
		lo, hi int
		want   markShape
	}{
		// The decision: a span that names the omitted verse lights its mark.
		{"the omitted verse alone", 21, 21, markShape{mark: true}},
		{"a span ending at the hole", 20, 21, markShape{join: true, mark: true}},
		{"a span from the hole", 21, 22, markShape{mark: true, after: true, next: true}},
		{"a span across the hole", 20, 22, markShape{join: true, mark: true, after: true, next: true}},
		// A span that does not name it leaves it bare: the verse after the
		// hole, and the verse before it.
		{"the verse after the hole", 22, 22, markShape{next: true}},
		{"the verse before the hole", 20, 20, markShape{}},
	} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		if got := appleMarkShape(t, buildChapterHTML(st, verses)); got != c.want {
			t.Errorf("Apple page, %s (%d-%d): %v, want %v", c.name, c.lo, c.hi, got, c.want)
		}
		if got := androidMarkShape(t, buildChapterHTMLAndroid(st, verses)); got != c.want {
			t.Errorf("Android page, %s (%d-%d): %v, want %v", c.name, c.lo, c.hi, got, c.want)
		}
	}
}

// The Apple panes paint the wash from runs of verse numbers resolved against
// the imported text (nativeTintRuns), not from the markup. A number the page
// omits stands for its mark, so the run reaches the mark wherever the markup
// lights it: a span on 21 alone was no run at all. A span on 22 alone does not
// name the mark, and its run starts at 22's number.
func TestNativeTintRunsReachTheMarkOfAnOmittedVerse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	type run struct{ lo, hi int }
	runsOf := func(lo, hi int) []run {
		st, verses := omittedVerseLinkState(t, lo, hi)
		var out []run
		for _, r := range nativeTintRuns(st, verses) {
			out = append(out, run{r.Lo, r.Hi})
		}
		return out
	}
	cases := []struct {
		lo, hi   int
		on, off  []run
		describe string
	}{
		{21, 21, []run{{21, 21}}, nil, "the mark alone"},
		{20, 21, []run{{20, 21}}, []run{{20, 20}}, "verse 20 and the mark"},
		{21, 22, []run{{21, 22}}, []run{{22, 22}}, "the mark and verse 22"},
		{22, 22, []run{{22, 22}}, []run{{22, 22}}, "verse 22 alone"},
		{20, 22, []run{{20, 22}}, []run{{20, 20}, {22, 22}}, "one run across the mark"},
		{20, 20, []run{{20, 20}}, []run{{20, 20}}, "verse 20 alone"},
	}
	setFootnotesEnabled(true)
	for _, c := range cases {
		if got := runsOf(c.lo, c.hi); fmt.Sprint(got) != fmt.Sprint(c.on) {
			t.Errorf("footnotes on, a span on %d-%d: runs %v, want %v (%s)", c.lo, c.hi, got, c.on, c.describe)
		}
	}
	// With the footnotes off the page draws no mark, and the hole ends a run.
	setFootnotesEnabled(false)
	for _, c := range cases {
		if got := runsOf(c.lo, c.hi); fmt.Sprint(got) != fmt.Sprint(c.off) {
			t.Errorf("footnotes off, a span on %d-%d: runs %v, want %v", c.lo, c.hi, got, c.off)
		}
	}
}

// The Windows and Linux pane draws the mark as a run of its own, which took
// the tint of the verse after it. It takes the tint of the omitted verse.
func TestTheCanvasPaneLightsTheMarkOfAnOmittedVerse(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	for _, c := range []struct {
		lo, hi int
		lit    bool
	}{{21, 21, true}, {20, 21, true}, {22, 22, false}, {20, 20, false}} {
		st, verses := omittedVerseLinkState(t, c.lo, c.hi)
		p := newStyledReadingPane(st, verses)
		p.Resize(fyne.NewSize(560, 700))
		markLine := -1
		var mark, before styledRun // the mark's run, and verse 20's last run on its line
		for li, ln := range p.lay.Lines {
			for _, r := range ln.Runs {
				if r.Kind == runVerseGap {
					markLine, mark = li, r
					break
				}
				if r.Verse == 20 {
					before = r
				}
			}
			if markLine >= 0 {
				break
			}
			before = styledRun{}
		}
		if markLine < 0 {
			t.Fatalf("a span on %d-%d: the pane drew no mark", c.lo, c.hi)
		}
		if got := mark.Tint == tintHighlight; got != c.lit {
			t.Errorf("a span on %d-%d: the mark's run is lit %v, want %v", c.lo, c.hi, got, c.lit)
		}
		// The wash drawn over the mark: a band covering it when it is lit.
		covered := false
		for _, sp := range p.tintSpans {
			if sp.Line != markLine || sp.Tint != tintHighlight || sp.LineX0 > mark.X || sp.LineX1 < mark.X+mark.W {
				continue
			}
			covered = true
			// A span ending at the hole is one band from verse 20 through
			// the space to the mark, where verse 20 shares the mark's line.
			if c.lo == 20 && c.hi == 21 && before.Text != "" && sp.LineX0 > before.X {
				t.Errorf("a span on 20-21: the band starts at x %.1f, after verse 20's last word at %.1f", sp.LineX0, before.X)
			}
		}
		if covered != c.lit {
			t.Errorf("a span on %d-%d: a band covers the mark %v, want %v", c.lo, c.hi, covered, c.lit)
		}
		if c.lo == 21 && c.hi == 21 {
			// The link lands on the mark's line, the line it lit.
			if got, want := p.highlightY(), p.lay.Lines[markLine].Y; !p.highlightOwnsScroll() || got != want {
				t.Errorf("a link to 21 lands at y %.1f (owns scroll %v), want the mark's line at %.1f", got, p.highlightOwnsScroll(), want)
			}
		}
	}
}

// holeChapterState is a chapter of an edition that omits verses: the verses
// nums, with invented text, and a link's mark on lo-hi, sent from edition
// from. Each case names a chapter whose holes the table records, so the page
// draws their marks with the footnotes on.
func holeChapterState(t *testing.T, from, version, book string, chapter int, nums []int, lo, hi int) (*AppState, []Verse) {
	t.Helper()
	verses := make([]Verse, 0, len(nums))
	for _, n := range nums {
		verses = append(verses, Verse{BookName: book, Book: book, Chapter: chapter, Verse: n,
			Text: fmt.Sprintf("Invented fixture text standing for verse %d.", n)})
	}
	st := &AppState{
		Bible:          &BibleData{Books: []string{book}, Verses: map[string]map[int][]Verse{book: {chapter: verses}}},
		CurrentVersion: version, CurrentBook: book, CurrentChapter: chapter,
	}
	st.setMark(hlLinkSpan, VerseSpan{VersionID: from, Book: book, Chapter: chapter, Lo: lo, Hi: hi})
	if len(pageGaps(st, verses)) == 0 {
		t.Fatalf("fixture: %s's %s %d draws no mark", version, book, chapter)
	}
	return st, verses
}

var (
	androidGapRE = regexp.MustCompile(`(<span style="background-color:#[0-9a-f]{6}">)?<btgap><font color="#[0-9a-f]{6}">\[(\d+)\]</font></btgap>`)
	androidNumRE = regexp.MustCompile(`(<span style="background-color:#[0-9a-f]{6}">)?<sup><btnum><font color="#[0-9a-f]{6}"><b>(\d+)</b>`)
)

// androidLit is the numbers an Android page writes with pattern re, and
// those of them it writes inside a wash.
func androidLit(t *testing.T, re *regexp.Regexp, html string) (all, lit []int) {
	t.Helper()
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, n)
		if m[1] != "" {
			lit = append(lit, n)
		}
	}
	return all, lit
}

// An omitted verse's mark lights only where a link or a note names that
// verse (docs/ROADMAP.md, decision 18), on every surface: the markup both
// HTML dialects write, the wash the Apple panes paint from their runs, and
// the Windows and Linux pane. A span that names the verse after the hole,
// and not the hole, leaves the mark bare. Where holes stand together, as the
// WEB Catholic Edition's nine before Sirach 26:28, a link to that verse lit
// the nine marks "[19]" to "[27]" with it.
func TestAnOmittedVersesMarkLightsOnlyWhereItsVerseIsNamed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	type chapter struct {
		from, version, book string
		ch                  int
		nums                []int
	}
	matthew := chapter{"nkjv", "bsb", "Matthew", 17, []int{19, 20, 22, 23}}
	sirach := chapter{"webc", "webc", "Sirach", 26, []int{17, 18, 28, 29}}
	acts := chapter{"nkjv", "web", "Acts", 8, []int{35, 36, 38, 39}}
	for _, c := range []struct {
		at            chapter
		lo, hi        int
		marks, verses []int // lit
	}{
		{matthew, 21, 21, []int{21}, nil},
		{matthew, 20, 21, []int{21}, []int{20}},
		{matthew, 22, 22, nil, []int{22}},
		{sirach, 28, 28, nil, []int{28}},
		{acts, 37, 37, []int{37}, nil},
		{acts, 38, 38, nil, []int{38}},
	} {
		name := fmt.Sprintf("%s %d:%d-%d in the %s", c.at.book, c.at.ch, c.lo, c.hi, c.at.version)
		st, verses := holeChapterState(t, c.at.from, c.at.version, c.at.book, c.at.ch, c.at.nums, c.lo, c.hi)
		checkLitOnEverySurface(t, name, st, verses, fmt.Sprint(c.marks, c.verses))
	}
}

// checkLitOnEverySurface holds the marks and verses each surface lights for
// the mark on st, written "[marks] [verses]", to want: the Apple dialect as
// the importer reads it, with the wash the Apple panes paint from their runs
// covering what the markup lights and nothing else; the Android dialect; and
// the Windows and Linux pane, where each run's tint and the band drawn over it
// agree.
func checkLitOnEverySurface(t *testing.T, name string, st *AppState, verses []Verse, want string) {
	t.Helper()
	// The Apple dialect, as the importer reads it.
	doc := importChapterHTML(t, buildChapterHTML(st, verses))
	var marks, lit []int
	for _, g := range doc.gaps {
		if doc.hl[g.loc] {
			marks = append(marks, g.n)
		}
	}
	for _, v := range doc.verse {
		if doc.hl[v.loc] {
			lit = append(lit, v.verse)
		}
	}
	if got := fmt.Sprint(marks, lit); got != want {
		t.Errorf("%s: the Apple page lights marks and verses %s, want %s", name, got, want)
	}
	// The wash the Apple panes paint from the runs covers what the markup
	// lights, and nothing else.
	wash := make([]bool, len(doc.text))
	for _, r := range nativeTintRuns(st, verses) {
		for i, on := range doc.runMask(r.Lo, r.Hi, true) {
			wash[i] = wash[i] || on
		}
	}
	if d := maskDiff(doc, wash, doc.hl); d != "" {
		t.Errorf("%s: the Apple panes' wash and the markup disagree:%s", name, d)
	}

	// The Android dialect.
	html := buildChapterHTMLAndroid(st, verses)
	allMarks, marks := androidLit(t, androidGapRE, html)
	if len(allMarks) == 0 {
		t.Fatalf("%s: the Android page draws no mark", name)
	}
	_, lit = androidLit(t, androidNumRE, html)
	if got := fmt.Sprint(marks, lit); got != want {
		t.Errorf("%s: the Android page lights marks and verses %s, want %s", name, got, want)
	}

	// The Windows and Linux pane: a run's tint, and the band drawn over it.
	p := newStyledReadingPane(st, verses)
	p.Resize(fyne.NewSize(560, 700))
	marks, lit = nil, nil
	for li, ln := range p.lay.Lines {
		for _, r := range ln.Runs {
			if r.Kind != runVerseGap && r.Kind != runVerseNum {
				continue
			}
			on := r.Tint == tintHighlight
			banded := false
			for _, sp := range p.tintSpans {
				if sp.Line == li && sp.Tint == tintHighlight && sp.LineX0 <= r.X && sp.LineX1 >= r.X+r.W {
					banded = true
				}
			}
			if banded != on {
				t.Errorf("%s: the canvas pane's %q is lit %v, banded %v", name, r.Text, on, banded)
			}
			switch {
			case !on:
			case r.Kind == runVerseNum:
				lit = append(lit, r.Verse)
			default:
				n, err := strconv.Atoi(strings.Trim(r.Text, "[]"))
				if err != nil {
					t.Fatal(err)
				}
				marks = append(marks, n)
			}
		}
	}
	if got := fmt.Sprint(marks, lit); got != want {
		t.Errorf("%s: the canvas pane lights marks and verses %s, want %s", name, got, want)
	}
}

// A note from another edition is filed by the chapter's plan on the verses
// this edition has (displayCopy): an NKJV note on Matthew 17:21-22 read in
// the BSB is drawn on verse 22. The note still names verse 21, so its mark
// lights "[21]" (docs/ROADMAP.md, decision 18), as a link to the same span
// does in the app and on the website. A note ending at the hole lights verse
// 20 and the mark, and one across it verse 20, the mark and verse 22, the
// band the same span draws from a link. A note on the verse after the hole
// alone leaves the mark bare. Each arrives on a real note link, opened by a
// reader without the NKJV.
func TestANoteFromAnotherEditionLightsTheMarksItNames(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())
	type chapter struct {
		version, path, book string
		ch                  int
		nums                []int
	}
	matthew := chapter{"bsb", "matthew", "Matthew", 17, []int{19, 20, 22, 23}}
	acts := chapter{"web", "acts", "Acts", 8, []int{35, 36, 38, 39}}
	type run struct{ lo, hi int }
	for _, c := range []struct {
		at            chapter
		lo, hi        int
		marks, verses []int // lit
		runs          []run // the Apple panes' runs
		card          int   // the verse the card hangs above
	}{
		{matthew, 21, 22, []int{21}, []int{22}, []run{{21, 22}}, 22},
		{matthew, 20, 21, []int{21}, []int{20}, []run{{20, 21}}, 20},
		{matthew, 20, 22, []int{21}, []int{20, 22}, []run{{20, 22}}, 20},
		{matthew, 22, 22, nil, []int{22}, []run{{22, 22}}, 22},
		{acts, 37, 38, []int{37}, []int{38}, []run{{37, 38}}, 38},
		{acts, 38, 38, nil, []int{38}, []run{{38, 38}}, 38},
	} {
		name := fmt.Sprintf("an NKJV note on %s %d:%d-%d in the %s", c.at.book, c.at.ch, c.lo, c.hi, c.at.version)
		deleteAllNotes(appPrefs())
		st := lockedNKJVState(t)
		var verses []Verse
		bd := &BibleData{Books: []string{c.at.book}, Verses: map[string]map[int][]Verse{c.at.book: {}}}
		for ch := 1; ch <= c.at.ch; ch++ {
			nums := []int{1}
			if ch == c.at.ch {
				nums = c.at.nums
			}
			var vs []Verse
			for _, n := range nums {
				vs = append(vs, Verse{BookName: c.at.book, Book: c.at.book, Chapter: ch, Verse: n,
					Text: fmt.Sprintf("Invented fixture text standing for verse %d.", n)})
			}
			bd.Verses[c.at.book][ch] = vs
			verses = vs
		}
		st.Bible = bd
		st.CurrentVersion, st.CurrentBook, st.CurrentChapter = c.at.version, c.at.book, 1
		st.loadedVersions = map[string]*BibleData{c.at.version: bd}

		frag := fmt.Sprintf("#v%d-%d", c.lo, c.hi)
		if c.hi == c.lo {
			frag = fmt.Sprintf("#v%d", c.lo)
		}
		payload := EncodeNoteWire(NoteWire{Text: "Invented note text.", Version: "nkjv",
			Book: c.at.book, Chapter: c.at.ch, VerseLo: c.lo, VerseHi: c.hi})
		target, ok := ParseShareLink(fmt.Sprintf("https://bibletext.co.uk/nkjv/%s/%d/%s&n=%s", c.at.path, c.at.ch, frag, payload))
		if !ok {
			t.Fatalf("%s: the link did not parse", name)
		}
		applyShareTarget(st, target)
		if st.CurrentVersion != c.at.version || st.CurrentChapter != c.at.ch || st.NoteID == 0 || !st.mark.fromNote() {
			t.Fatalf("%s: the link opened the %s on chapter %d with note %d (the mark the note's %v); want the stored note on chapter %d",
				name, st.CurrentVersion, st.CurrentChapter, st.NoteID, st.mark.fromNote(), c.at.ch)
		}
		checkLitOnEverySurface(t, name, st, verses, fmt.Sprint(c.marks, c.verses))
		var runs []run
		for _, r := range nativeTintRuns(st, verses) {
			runs = append(runs, run{r.Lo, r.Hi})
		}
		if fmt.Sprint(runs) != fmt.Sprint(c.runs) {
			t.Errorf("%s: the Apple panes' runs are %v, want %v", name, runs, c.runs)
		}
		chrome := chapterNoteChrome(st, buildChapterPlan(st, appPrefs(), st.Bible), verses)
		if chrome.Anchor != c.card {
			t.Errorf("%s: the card hangs above verse %d, want %d", name, chrome.Anchor, c.card)
		}
	}

	// The reader's own note from another edition, drawn in its own slot
	// while focus names it, lights the same span.
	deleteAllNotes(appPrefs())
	st, verses := omittedVerseLinkState(t, 19, 19)
	st.clearMark()
	own, ok := addNote(appPrefs(), StoredNote{Kind: noteKindMine, VersionID: "nkjv", Book: "Matthew", Chapter: 17,
		VerseLo: 21, VerseHi: 22, Text: "Invented note text."})
	if !ok {
		t.Fatal("the store refused the reader's own note")
	}
	st.focusNote(own.ID)
	applyNoteForCurrentChapter(st)
	if st.NoteID != own.ID || !st.mark.fromNote() {
		t.Fatalf("the reader's own note: note %d is drawn (the mark the note's %v), want %d", st.NoteID, st.mark.fromNote(), own.ID)
	}
	checkLitOnEverySurface(t, "the reader's own NKJV note on Matthew 17:21-22 in the bsb", st, verses, "[21] [22]")
}

// omittedNoteState is the fixture with a note shown from its link alone: the
// store refused it (NoteID 0), so the chapter's plan does not carry it and
// the note keeps the verse its link named, which the page omits.
func omittedNoteState(t *testing.T, lo, hi int) (*AppState, []Verse) {
	t.Helper()
	st, verses := omittedVerseLinkState(t, lo, hi)
	st.clearMark()
	st.setMark(hlNote, VerseSpan{VersionID: "nkjv", Book: "Matthew", Chapter: 17, Lo: lo, Hi: hi})
	st.ActiveNote = "Invented note text."
	st.NoteVerseLo = lo
	st.NoteID = 0
	return st, verses
}

// A note shown from its link alone on a span that starts at the omitted verse
// hangs its card above the verse after the hole on every surface, where its
// mark stands (decision 18; PLATFORM_MATRIX.md, divergence 34, closed): the
// chrome hands every pane that verse, and the arrival lands on the card's
// band. It used to hand the omitted verse, which the Apple panes resolved
// through the lit range, the website through the fragment, and Android and
// the canvas pane not at all, parking the card at the top; on the omitted
// verse alone every surface parked it there.
func TestANoteOnAnOmittedVerseHangsAboveTheVerseAfterTheHole(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	setNotesEnabled(true)
	for _, c := range []struct{ lo, hi int }{{21, 21}, {21, 22}} {
		st, verses := omittedNoteState(t, c.lo, c.hi)
		chrome := chapterNoteChrome(st, buildChapterPlan(st, appPrefs(), st.Bible), verses)
		if !chrome.present() {
			t.Fatalf("a note on %d-%d: no card on the chrome", c.lo, c.hi)
		}
		if chrome.Anchor != 22 || !chrome.hasTail() {
			t.Errorf("a note on %d-%d: the card hangs from verse %d (tail %v), want verse 22 with a tail",
				c.lo, c.hi, chrome.Anchor, chrome.hasTail())
		}
		if chrome.Arrival != arriveBand || chrome.ArrivalVerse != 22 {
			t.Errorf("a note on %d-%d: the arrival is %v at verse %d, want the band at 22",
				c.lo, c.hi, chrome.Arrival, chrome.ArrivalVerse)
		}
		// The Windows and Linux pane opens the band above verse 22.
		p := newStyledReadingPane(st, verses)
		p.Resize(fyne.NewSize(560, 700))
		if got := p.noteAnchorVerse(); got != 22 {
			t.Errorf("a note on %d-%d: the canvas pane opens the band above verse %d, want 22", c.lo, c.hi, got)
		}
	}
	// The control: a note past the chapter's last verse has nowhere to hang.
	st, verses := omittedNoteState(t, 99, 99)
	if got := chapterNoteChrome(st, buildChapterPlan(st, appPrefs(), st.Bible), verses).Anchor; got != 99 {
		t.Errorf("a note past the chapter's end hangs from verse %d, want 99 as named", got)
	}
}

// Every pane takes the card's verse from the chrome, so the four cannot come
// to disagree about a verse the page omits again. The panes' own source is
// read because three of them sit behind build tags.
func TestEveryPanePushesTheNotesDrawnVerse(t *testing.T) {
	for path, want := range map[string]string{
		"reading_ios.go":     "C.bibleTextSetNote(cText, cWho, min, nx, ownFlag, C.int(c.Anchor),",
		"reading_macos.go":   "C.bibleTextMacSetNote(cText, cWho, min, nx, macOwnFlag(state), C.int(c.Anchor),",
		"reading_android.go": "anchor := c.Anchor\n",
		"notes_chrome.go":    "Anchor: drawnNoteAnchor(verses, state.NoteVerseLo),",
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), want) {
			t.Errorf("%s does not push the note's drawn verse: %q is missing", path, want)
		}
		if strings.Contains(string(src), "C.int(state.NoteVerseLo)") || strings.Contains(string(src), "anchor := state.NoteVerseLo") {
			t.Errorf("%s still pushes the note's verse as named", path)
		}
	}
}

// --- The Apple panes' rule with holes' marks, in Go ---------------------------
//
// The model of the imported page and of the native wash rule is
// reading_tint_wash_shape_test.go's; these add the marks to it.

// gapRange is btIOSGapRange / btMacGapRange: the mark of omitted verse n.
func (d importedChapter) gapRange(n int) (int, int, bool) {
	for _, g := range d.gaps {
		if g.n == n {
			return g.loc, g.end, true
		}
	}
	return 0, 0, false
}

// endOfWithMarks is a verse's end as the native side records it now: endOf,
// clamped at the first hole's mark after the verse's number
// (btIOSBuildVerseIndex; btMacReadAlongRange).
func (d importedChapter) endOfWithMarks(verse int) int {
	end := d.endOf(verse, true)
	start := d.locOf(verse)
	for _, g := range d.gaps {
		if g.loc > start && g.loc < end {
			return g.loc
		}
	}
	return end
}

// elemRange is btIOSRunElementRange / btMacRunElementRange: verse k's span,
// or the mark of omitted k. withMarks false is the rule this replaced, where
// a run resolved verses alone and a verse ran on to the next number.
func (d importedChapter) elemRange(k int, withMarks bool) (int, int, bool) {
	if s := d.locOf(k); s >= 0 {
		if withMarks {
			return s, d.endOfWithMarks(k), true
		}
		return s, d.endOf(k, true), true
	}
	if withMarks {
		return d.gapRange(k)
	}
	return 0, 0, false
}

// runMask is the characters the wash view paints for a run lo..hi:
// btIOSRunSpanRange (from the first number the page draws to the end of the
// last), trimmed of trailing whitespace (btIOSRunWashRange), less the bare
// ranges (btIOSPaintedPieces).
func (d importedChapter) runMask(lo, hi int, withMarks bool) []bool {
	mask := make([]bool, len(d.text))
	first, start, end := lo, 0, 0
	found := false
	for ; first <= hi; first++ {
		if s, e, ok := d.elemRange(first, withMarks); ok {
			start, end, found = s, e, true
			break
		}
	}
	if !found {
		return mask
	}
	for k := hi; k > first; k-- {
		if _, e, ok := d.elemRange(k, withMarks); ok {
			if e > end {
				end = e
			}
			break
		}
	}
	for end > start && unicode.IsSpace(d.text[end-1]) {
		end--
	}
	for _, p := range paintedPieces(d.text, d.heads, start, end, true) {
		for i := p[0]; i < p[1] && i < len(mask); i++ {
			mask[i] = true
		}
	}
	return mask
}

// The wash the Apple panes paint from the runs must cover exactly what the
// markup lights, the mark and the spaces beside it included, or a wash-only
// repaint changes the band's shape (reading_tint_wash_shape_test.go says
// why). Each case also shows that the rule this replaced, a run of verses
// alone and a verse running on to the next number, disagreed with the markup,
// so it cannot pass by accident: it lit nothing for a span on the omitted
// verse, left the mark bare beside a lit verse 22 and the space after it bare
// in a span across the hole, and lit the mark with verse 20 alone.
func TestTheWashCoversAnOmittedVersesMarkAsTheMarkupDoes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	for _, para := range []bool{false, true} {
		for _, c := range []struct {
			lo, hi int
			proves bool // the rule this replaced disagreed here
		}{
			{21, 21, true}, {20, 21, false}, {21, 22, true}, {22, 22, false},
			{20, 22, true}, {20, 20, true}, {19, 20, true}, {23, 23, false},
		} {
			for _, reporter := range []bool{false, true} {
				name := fmt.Sprintf("%d-%d, verse 22 opens a paragraph %v, reporter %v", c.lo, c.hi, para, reporter)
				st, verses := omittedVerseLinkState(t, c.lo, c.hi)
				if para {
					verses[2].ParaStart = true
					st.Bible.Verses["Matthew"][17] = verses
				}
				var html string
				withReporterLayout(reporter, func() { html = buildChapterHTML(st, verses) })
				doc := importChapterHTML(t, html)
				if len(doc.gaps) != 1 {
					t.Fatalf("%s: the model found %d marks, want one", name, len(doc.gaps))
				}
				got := make([]bool, len(doc.text))
				for _, r := range nativeTintRuns(st, verses) {
					for i, on := range doc.runMask(r.Lo, r.Hi, true) {
						got[i] = got[i] || on
					}
				}
				if d := maskDiff(doc, got, doc.hl); d != "" {
					t.Errorf("%s: the wash and the markup disagree:%s", name, d)
				}
				// The old runs for the same span, under the old rule: walked
				// over the verses alone, a hole ending the run.
				oldRuns := nativeTintRunsWithoutMarks(st, verses)
				oldMask := make([]bool, len(doc.text))
				for _, r := range oldRuns {
					for i, on := range doc.runMask(r.Lo, r.Hi, false) {
						oldMask[i] = oldMask[i] || on
					}
				}
				if differs := maskDiff(doc, oldMask, doc.hl) != ""; differs != c.proves {
					t.Errorf("%s: the rule this replaced disagreed with the markup %v, want %v", name, differs, c.proves)
				}
			}
		}
	}
}

// nativeTintRunsWithoutMarks is the flattening nativeTintRuns replaced: the
// verses alone, a gap in the numbering ending a run.
func nativeTintRunsWithoutMarks(state *AppState, verses []Verse) []nativeTintRun {
	tints := chapterTint(state)
	var runs []nativeTintRun
	last := tintNone
	for _, v := range verses {
		tv := tints.of(v)
		if tv == tintNone {
			last = tintNone
			continue
		}
		if tv == last && len(runs) > 0 && runs[len(runs)-1].Hi == v.Verse-1 {
			runs[len(runs)-1].Hi = v.Verse
			continue
		}
		runs = append(runs, nativeTintRun{Lo: v.Verse, Hi: v.Verse})
		last = tv
	}
	return runs
}

// The Apple panes resolve the runs above against the imported text, in
// Objective-C no host test runs. Their half of the rule is held in the
// source: a hole's mark is indexed apart from the verses and ends the verse
// before it, and a run resolves an omitted number to its mark. The model of
// the same rule runs in TestTheWashCoversAnOmittedVersesMarkAsTheMarkupDoes.
func TestTheApplePanesResolveARunToAHolesMark(t *testing.T) {
	for path, wants := range map[string][]string{
		"reading_ios.go": {
			"NSInteger hole = btGapMarkNumber(run);",
			"if (gi < g && gGapIndex[gi].loc < bound) bound = gGapIndex[gi].loc;",
			"return r.location != NSNotFound ? r : btIOSGapRange(ts, k);",
			"a = btIOSRunElementRange(ts, first);",
			"NSRange r = btIOSRunElementRange(ts, k);",
		},
		"reading_macos.go": {
			"if ([run integerValue] > 0 || btMacGapMarkNumber(run) > 0) { nextLoc = r.location; *stop = YES; }",
			"return r.location != NSNotFound ? r : btMacGapRange(ts, k);",
			"a = btMacRunElementRange(ts, first);",
			"NSRange r = btMacRunElementRange(ts, k);",
		},
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s is missing %q", path, want)
			}
		}
	}
}

// --- The narration over a hole's mark ----------------------------------------
//
// No verse narrates an omitted verse's mark. The recording of the BSB's
// Matthew 17 has no verse 21, so the narration of verse 20 stops before "[21]"
// and that of verse 22 starts at 22's number, on every surface. The Apple
// panes end a verse at the mark after it (TestTheApplePanesResolveARunToAHolesMark
// holds their source). The canvas pane filed the mark under verse 22 and lit
// it with 22's narration, and Android ran verse 20 on to 22's number and lit it
// with 20's, so the three gave three answers.

// The Windows and Linux pane: the narration's band is drawn from the runs it
// names (verseSpansForLayout), and the mark's run is no verse's.
func TestTheCanvasPaneNarratesNoOmittedVersesMark(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	st, verses := omittedVerseLinkState(t, 19, 19)
	p := newStyledReadingPane(st, verses)
	p.Resize(fyne.NewSize(560, 700))
	markLine, numLine := -1, -1
	var mark, num styledRun
	for li, ln := range p.lay.Lines {
		for _, r := range ln.Runs {
			switch {
			case r.Kind == runVerseGap:
				markLine, mark = li, r
			case r.Kind == runVerseNum && r.Verse == 22:
				numLine, num = li, r
			}
		}
	}
	if markLine < 0 || numLine < 0 {
		t.Fatalf("the pane drew the mark on line %d and verse 22's number on line %d", markLine, numLine)
	}
	covers := func(spans []tintSpan, line int, r styledRun) bool {
		for _, sp := range spans {
			if sp.Line == line && sp.LineX0 < r.X+r.W && sp.LineX1 > r.X {
				return true
			}
		}
		return false
	}
	for _, verse := range []int{20, 22} {
		p.setReadAlongVerse(verse)
		if len(p.raSpans) == 0 {
			t.Fatalf("the narration of verse %d lights nothing", verse)
		}
		if covers(p.raSpans, markLine, mark) {
			t.Errorf("the narration of verse %d lights the %q mark", verse, mark.Text)
		}
	}
	// The control: the narration of verse 22 still lights its own number,
	// which stands just after the mark.
	p.setReadAlongVerse(22)
	if !covers(p.raSpans, numLine, num) {
		t.Errorf("the narration of verse 22 does not light its own number")
	}
}

// Android narrates from the bridge's verse index, which ends a verse at the
// next verse's number or a heading before it. The mark is no superscript, so
// the index has to be told where it stands: the import gives it a span class
// of its own, and a verse ends at the first mark after its number. The Java
// runs on the device alone, so its source is held here.
func TestAndroidEndsAVerseAtAnOmittedVersesMark(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	handler := java[strings.Index(java, "private static final Html.TagHandler READING_SIZES"):]
	handler = handler[:strings.Index(handler, "\n    };\n")]
	if !strings.Contains(handler, "out.setSpan(new GapMarkSpan(), st, en, flags);") {
		t.Error("READING_SIZES does not size <btgap> with GapMarkSpan, so the verse index cannot find the mark")
	}
	if !strings.Contains(java, "private static final class GapMarkSpan extends android.text.style.RelativeSizeSpan {\n        GapMarkSpan() { super(GAP_MARK_EM); }") {
		t.Error("GapMarkSpan is not the mark's size, GAP_MARK_EM")
	}
	index := javaMethodSource(t, "android/BtBridge.java", "private static void buildVerseIndex(CharSequence cs) {")
	for _, want := range []string{
		"GapMarkSpan[] gaps = sp.getSpans(0, sp.length(), GapMarkSpan.class);",
		"Arrays.sort(marks);",
		"ends[count] = endBeforeHeading(sp, st, endAtMark(marks, st, en));",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("buildVerseIndex lacks %q: a verse runs on over the mark after it", want)
		}
	}
	end := javaMethodSource(t, "android/BtBridge.java", "private static int endAtMark(int[] marks, int st, int en) {")
	for _, want := range []string{"if (m >= en) break;", "if (m > st) return m;", "return en;"} {
		if !strings.Contains(end, want) {
			t.Errorf("endAtMark lacks %q", want)
		}
	}
}
