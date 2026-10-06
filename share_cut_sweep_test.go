package bibletext

// The RAGGED-CUT sweep: every bug in the share pipeline's field history came
// from a drag boundary landing somewhere unexpected — inside a word, inside a
// verse-number marker, one rune short of a period. So instead of enumerating
// boundary cases by hand, this sweep runs prepareShareQuote over EVERY
// possible end-cut position (and every start-cut position) through whole
// chapters of the real embedded WEB Gospels, with and without the reading
// view's verse-number markers, and asserts the settled invariants on each:
//
//	C1  the shared text is a substring of the chapter's marker-free prose
//	    (nothing invented, no marker ever leaks) — allowing only the single
//	    restored terminal punctuation mark;
//	C2  both ends of the shared text sit on WORD boundaries in the source
//	    (the mid-word repair always lands);
//	C3  the citation is exactly the verses the shared words overlap
//	    (provenance: any surviving word cites its verse, nothing else);
//	C4  ELLIPSIS HONESTY, both directions: the formatted quote carries the
//	    four-dot mark IFF at least one word stands between the cut and the
//	    source sentence's terminal (a drag that stopped short of only the
//	    punctuation shares clean without a false ellipsis);
//	C5  the standing formatter invariants (balance, no "…", no bare
//	    three-dot ending) via rwCheckInvariants.
//
// The window is anchored at a real verse start so every iteration is a
// plausible drag; go test -short samples every 7th cut.

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// cutSweepChapters: variety on purpose — narrative + dialogue (John 1),
// discourse (Matthew 5), parables with nested speech (Luke 15).
var cutSweepChapters = []struct {
	book    string
	chapter int
}{
	{"John", 1},
	{"Matthew", 5},
	{"Luke", 15},
}

// parseCiteRange extracts lo–hi from "Book C:lo–hi" / "Book C:lo" / "Book C".
func parseCiteRange(t *testing.T, cite string) (lo, hi int, ok bool) {
	t.Helper()
	i := strings.LastIndexByte(cite, ':')
	if i < 0 {
		return 0, 0, false
	}
	span := cite[i+1:]
	if j := strings.Index(span, "–"); j >= 0 {
		lo, _ = strconv.Atoi(span[:j])
		hi, _ = strconv.Atoi(span[j+len("–"):])
	} else {
		lo, _ = strconv.Atoi(span)
		hi = lo
	}
	return lo, hi, lo > 0 && hi >= lo
}

// sentenceHasWordBeforeTerminal reports whether a word rune stands between
// corpus position end and the terminal of the sentence in progress there —
// the ground truth for whether an end omission actually omits words.
func sentenceHasWordBeforeTerminal(corpus string, end int) bool {
	for _, r := range corpus[end:] {
		switch {
		case r == '.' || r == '!' || r == '?' || r == '…':
			return false
		case isWordRune(r):
			return true
		}
	}
	return false
}

// isSubWordDrag reports a drag with no complete word in it. Such drags are
// degenerate: the mid-word repair rightly leaves nothing, and the pipeline
// falls back to sharing the fragment verbatim — pinned separately in
// TestShareSelectionSubWordFragment, excluded from the invariant sweep.
func isSubWordDrag(raw string) bool {
	return !strings.Contains(strings.TrimSpace(raw), " ")
}

// assertCutInvariants runs one drag through prepareShareQuote and asserts
// C1–C6 on what it shares. corpus and spans are chapterProse(st), which the
// sweeps build once per chapter: the chapter never changes during a sweep, so
// rebuilding it for every cut would only repeat the same result. It reports
// whether the cut was checked (false for a sub-word drag or an empty share).
func assertCutInvariants(t *testing.T, st *AppState, corpus string, spans []verseSpan, label, raw string) bool {
	t.Helper()
	if isSubWordDrag(raw) {
		return false
	}
	text, cite, _, _ := prepareShareQuote(st, raw, selSpan{})
	return assertSharedCutInvariants(t, st, corpus, spans, label, text, cite)
}

// assertSharedCutInvariants asserts C1–C6 on prepareShareQuote's text and
// citation for one drag, for a caller that already holds them.
func assertSharedCutInvariants(t *testing.T, st *AppState, corpus string, spans []verseSpan, label, text, cite string) bool {
	t.Helper()
	if text == "" {
		return false
	}

	// C1: substring of the prose, allowing one restored terminal at the end.
	idx := strings.Index(corpus, text)
	body := text
	if idx < 0 {
		trimmed := strings.TrimRight(text, ".!?…")
		if trimmed == text || len(text)-len(trimmed) > len("…") {
			t.Fatalf("%s: shared text is not chapter prose: %q", label, text)
		}
		body = trimmed
		idx = strings.Index(corpus, body)
		if idx < 0 {
			t.Fatalf("%s: shared text (sans restored terminal) not in prose: %q", label, text)
		}
	}
	end := idx + len(body)

	// C2: word boundaries at both ends.
	if idx > 0 {
		if r, _ := utf8.DecodeLastRuneInString(corpus[:idx]); isWordRune(r) {
			t.Fatalf("%s: share starts mid-word: …%q + %q", label, string(r), firstRunes(text, 20))
		}
	}
	if end < len(corpus) {
		if r, _ := utf8.DecodeRuneInString(corpus[end:]); isWordRune(r) {
			t.Fatalf("%s: share ends mid-word before %q: …%q", label, string(r), text[maxInt(0, len(text)-20):])
		}
	}

	// C3: provenance citation.
	wantLo, wantHi := 0, 0
	for _, sp := range spans {
		if sp.start < end && idx < sp.end {
			if wantLo == 0 {
				wantLo = sp.verse
			}
			wantHi = sp.verse
		}
	}
	if lo, hi, ok := parseCiteRange(t, cite); ok {
		if lo != wantLo || hi != wantHi {
			t.Fatalf("%s: citation %q but shared words span %d–%d (text …%q)",
				label, cite, wantLo, wantHi, text[maxInt(0, len(text)-40):])
		}
	} else if wantLo != 0 {
		t.Fatalf("%s: chapter-only citation %q though words located at %d–%d", label, cite, wantLo, wantHi)
	}

	// C4 + C5: format and check ellipsis honesty + standing invariants.
	// A text ending ON its sentence terminal (inside any closing marks) is a
	// COMPLETE quotation — material after the final punctuation is never
	// marked (BB 5.3(b)(iv) / Indigo R39.9) — so words in FOLLOWING sentences
	// don't count as omissions; only an unterminated text can omit words.
	quote := formatBibleQuote(text, originalSentenceTerminal(st, text, -1, 0))
	hasMark := strings.Contains(quote, " . . . ")
	core := strings.TrimRight(text, " \u201d\u2019\"'")
	completed := false
	if r, _ := utf8.DecodeLastRuneInString(core); r == '.' || r == '!' || r == '?' || r == '…' {
		completed = true
	}
	omitsWords := !completed && sentenceHasWordBeforeTerminal(corpus, end)
	if hasMark && !omitsWords {
		t.Fatalf("%s: four-dot mark with no omitted words (…%q → %q)",
			label, text[maxInt(0, len(text)-30):], quote[maxInt(0, len(quote)-40):])
	}
	if !hasMark && omitsWords {
		t.Fatalf("%s: words omitted but no mark (…%q)", label, quote[maxInt(0, len(quote)-40):])
	}
	rwCheckInvariants(t, label, text, quote, len(strings.Fields(text)))

	// C6: the cited-text restore pass. Whatever structure
	// restoreShareLineBreaks re-inserts must be EXACTLY the flattened spaces —
	// collapsing it back must reproduce the pipeline text (content-preserving),
	// and any inserted break must land between words, never inside one.
	restored := restoreShareLineBreaks(st, text, -1, 0)
	if collapseSpaces(strings.ReplaceAll(restored, "\n", " ")) != collapseSpaces(text) {
		t.Fatalf("%s: restore pass altered content:\n text %q\n rest %q", label, text, restored)
	}
	for i := 0; i < len(restored); i++ {
		if restored[i] != '\n' {
			continue
		}
		if i == 0 || i == len(restored)-1 {
			t.Fatalf("%s: restored break at text edge: %q", label, restored)
		}
		prev, _ := utf8.DecodeLastRuneInString(restored[:i])
		switch {
		case isWordRune(prev):
		case prev == '\n': // second half of a paragraph (blank-line) break
		case prev == '.' || prev == ',' || prev == ';' || prev == ':' ||
			prev == '”' || prev == '’' || prev == '?' || prev == '!' || prev == ')':
		default:
			t.Fatalf("%s: break follows unexpected rune %q: %q", label, prev, restored)
		}
	}
	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestRaggedCutSweep drags the END of a selection across every rune boundary
// of three real chapters (window anchored at a verse start ~320 bytes back).
func TestRaggedCutSweep(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	for _, cc := range cutSweepChapters {
		st := &AppState{Bible: bd, CurrentBook: cc.book, CurrentChapter: cc.chapter}
		corpus, spans := chapterProse(st)
		if corpus == "" {
			t.Fatalf("no prose for %s %d", cc.book, cc.chapter)
		}
		cuts, checked := 0, 0
		for e := 1; e <= len(corpus); e += step {
			if e < len(corpus) && !utf8.RuneStart(corpus[e]) {
				continue // never cut inside a multibyte rune — real drags can't
			}
			wStart := 0
			for _, sp := range spans {
				if sp.start <= maxInt(0, e-320) {
					wStart = sp.start
				}
			}
			if wStart >= e {
				continue
			}
			raw := corpus[wStart:e]
			cuts++
			if assertCutInvariants(t, st, corpus, spans,
				fmt.Sprintf("%s %d end-cut@%d", cc.book, cc.chapter, e), raw) {
				checked++
			}
		}
		t.Logf("%s %d: %d end-cuts, %d checked", cc.book, cc.chapter, cuts, checked)
	}
}

// TestRaggedStartSweep drags the START across every boundary of John 1
// (fixed window end), covering the symmetric mid-word start repair.
func TestRaggedStartSweep(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	st := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 1}
	corpus, spans := chapterProse(st)
	step := 1
	if testing.Short() {
		step = 7
	}
	cuts, checked := 0, 0
	for s := 0; s < len(corpus)-1; s += step {
		if !utf8.RuneStart(corpus[s]) {
			continue
		}
		e := s + 300
		if e > len(corpus) {
			e = len(corpus)
		}
		for e > s && e < len(corpus) && !utf8.RuneStart(corpus[e]) {
			e--
		}
		cuts++
		if assertCutInvariants(t, st, corpus, spans, fmt.Sprintf("John 1 start-cut@%d", s), corpus[s:e]) {
			checked++
		}
	}
	t.Logf("John 1: %d start-cuts, %d checked", cuts, checked)
}

// TestRaggedMarkerCutSweep rebuilds the READING-VIEW text (verse-number
// markers included, exactly as a selection arrives from the overlay) and cuts
// inside and around every marker: "…God. 2", "…God. 21 ", "…God. 21 A" — the
// shapes that leaked a marker into a real shared card.
func TestRaggedMarkerCutSweep(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, cc := range cutSweepChapters {
		st := &AppState{Bible: bd, CurrentBook: cc.book, CurrentChapter: cc.chapter}
		corpus, spans := chapterProse(st)
		verses := bd.GetChapter(cc.book, cc.chapter)

		// The overlay's rendition: "1 text 2 text 3 text…"
		var b strings.Builder
		type vpos struct{ mark, text int }
		var marks []vpos
		for _, v := range verses {
			vt := collapseSpaces(v.Text)
			if vt == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			marks = append(marks, vpos{mark: b.Len()})
			b.WriteString(strconv.Itoa(v.Verse))
			b.WriteByte(' ')
			marks[len(marks)-1].text = b.Len()
			b.WriteString(vt)
		}
		rendered := b.String()

		// Cut at every boundary within [marker start, marker start+40) for
		// every verse — straddling the digits, the space, and the first words.
		cuts, shared, checked := 0, 0, 0
		for _, m := range marks[1:] { // skip verse 1 (nothing before it to anchor)
			wStart := maxInt(0, m.mark-260)
			for wStart > 0 && rendered[wStart-1] != ' ' {
				wStart--
			}
			limit := m.mark + 40
			if limit > len(rendered) {
				limit = len(rendered)
			}
			for e := m.mark + 1; e <= limit; e++ {
				if e < len(rendered) && !utf8.RuneStart(rendered[e]) {
					continue
				}
				raw := rendered[wStart:e]
				label := fmt.Sprintf("%s %d marker-cut@%d", cc.book, cc.chapter, e)
				cuts++
				text, cite, _, _ := prepareShareQuote(st, raw, selSpan{})
				if text == "" {
					continue
				}
				shared++
				// The one non-negotiable: a verse-number token NEVER survives.
				bodyText := strings.TrimRight(text, ".!?…")
				if strings.Index(corpus, bodyText) < 0 {
					t.Fatalf("%s: output is not chapter prose (marker leak?): %q", label, text)
				}
				// The same share, checked against C1–C6. prepareShareQuote is a
				// pure function of the chapter and the drag, so the result above
				// is the one a second call would return.
				if isSubWordDrag(raw) {
					continue
				}
				if assertSharedCutInvariants(t, st, corpus, spans, label, text, cite) {
					checked++
				}
			}
		}
		t.Logf("%s %d: %d marker-cuts, %d shared, %d checked", cc.book, cc.chapter, cuts, shared, checked)
	}
}

// poetrySweepState is a full poetic chapter for the sweep — BSB Psalm 23 with
// authored poem-line breaks in every verse (representative line placement), so
// every cut position exercises the restore pass (C6) against real poetic
// structure: within-verse breaks AND poetic verse joins.
func poetrySweepState() *AppState {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1,
				Text: "The LORD is my shepherd;\nI shall not want."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2,
				Text: "He makes me lie down in green pastures;\nHe leads me beside quiet waters."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 3,
				Text: "He restores my soul;\nHe guides me in the paths of righteousness\nfor the sake of His name."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 4,
				Text: "Even though I walk through the valley of the shadow of death,\nI will fear no evil,\nfor You are with me;\nYour rod and Your staff,\nthey comfort me."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 5,
				Text: "You prepare a table before me\nin the presence of my enemies.\nYou anoint my head with oil;\nmy cup overflows."},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 6,
				Text: "Surely goodness and mercy will follow me\nall the days of my life,\nand I will dwell in the house of the LORD\nforever."},
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

// TestRaggedCutSweepPoetry drags the END of a selection across every rune
// boundary of a wholly poetic chapter — the C1-C6 invariants (including the
// restore pass's content preservation and break placement) must hold on
// poetry exactly as on the prose Gospels.
func TestRaggedCutSweepPoetry(t *testing.T) {
	st := poetrySweepState()
	corpus, spans := chapterProse(st)
	if corpus == "" {
		t.Fatal("no prose for the poetry sweep")
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	for e := 1; e <= len(corpus); e += step {
		if e < len(corpus) && !utf8.RuneStart(corpus[e]) {
			continue
		}
		wStart := 0
		for _, sp := range spans {
			if sp.start <= maxInt(0, e-320) {
				wStart = sp.start
			}
		}
		if wStart >= e {
			continue
		}
		raw := corpus[wStart:e]
		assertCutInvariants(t, st, corpus, spans,
			fmt.Sprintf("Psalms 23 poetry end-cut@%d", e), raw)
	}
}

// THE SPANNED SWEEP. The sweeps above hand the pipeline no span, which no
// shipped pane does: the Apple and Android panes and the Windows and Linux
// pane all report the verses a selection touches, read off the page by
// position (selection_span.go). This one drags the way a reader does across
// every verse boundary of all 89 chapters of the embedded WEB Gospels, with
// the verse numbers in the selected text and the span the panes would
// report, and asks of each drag the one question a citation must answer
// right: are the verses it names the verses the shared words came from.
//
// The page is rebuilt as the panes draw it, "1 text 2 text …", and a drag's
// span is what the panes compute: the verse whose number is the last at or
// before the drag's first character, and the same for its last character
// (btIOSVerseAtIndex, btMacVerseSpanForRange, BtBridge.verseAtOffset; the
// styled pane's runs give the same answer). Three kinds of drag start at each
// boundary between verse N−1 and verse N, and each runs on to the end of the
// k-th word from the start of verse N:
//
//	punct  begins on a mark that ends verse N−1, its full stop or the closing
//	       quotation marks with it. The span is honestly (N−1, …), the mark is
//	       rightly dropped from the quotation, and the shared words are all
//	       verse N's. k runs to 12;
//	cross  begins on verse N−1's last word, so the shared words run across
//	       the boundary. k runs to 3;
//	number begins on verse N's own number, the control: the span starts at N,
//	       and nothing is cut from the front. k runs to 3.
//
// The truth for a drag is where its shared words stand inside the drag: the
// verses that stretch of the chapter overlaps. A shared text that does not
// occur inside the drag at all is wrong whatever it cites.
type spannedPage struct {
	st       *AppState
	corpus   string
	spans    []verseSpan
	page     string // the chapter as the panes draw it, numbers included
	number   []int  // page offset of each verse's number, by index into spans
	text     []int  // page offset of each verse's text
	verseAt  []int  // per page offset: the verse whose number is last at or before it
	toCorpus []int  // per page offset: the chapter-prose offset (a number maps to its verse's start)
}

func newSpannedPage(bd *BibleData, book string, chapter int) *spannedPage {
	st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter}
	corpus, spans := chapterProse(st)
	p := &spannedPage{st: st, corpus: corpus, spans: spans}
	var b strings.Builder
	for _, sp := range spans {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		p.number = append(p.number, b.Len())
		b.WriteString(strconv.Itoa(sp.verse))
		b.WriteByte(' ')
		p.text = append(p.text, b.Len())
		b.WriteString(corpus[sp.start:sp.end])
	}
	p.page = b.String()
	p.verseAt = make([]int, len(p.page)+1)
	p.toCorpus = make([]int, len(p.page)+1)
	vi := 0
	for i := 0; i <= len(p.page); i++ {
		for vi+1 < len(p.number) && p.number[vi+1] <= i {
			vi++
		}
		p.verseAt[i] = spans[vi].verse
		if i < p.text[vi] {
			p.toCorpus[i] = spans[vi].start
		} else {
			p.toCorpus[i] = spans[vi].start + i - p.text[vi]
		}
	}
	return p
}

// words is the page offset of the end of each space-delimited word of verse
// text, in chapter order, and the index of each verse's first word.
func (p *spannedPage) words() (ends []int, first []int) {
	for vi, sp := range p.spans {
		first = append(first, len(ends))
		body := p.corpus[sp.start:sp.end]
		for i := 0; i < len(body); {
			j := strings.IndexByte(body[i:], ' ')
			if j < 0 {
				j = len(body) - i
			}
			if j > 0 {
				ends = append(ends, p.text[vi]+i+j)
			}
			i += j + 1
		}
	}
	return ends, first
}

// citeOf is the citation for the chapter-prose stretch [at, at+n).
func (p *spannedPage) citeOf(at, n int) string {
	lo, hi := 0, 0
	for _, sp := range p.spans {
		if sp.start < at+n && at < sp.end {
			if lo == 0 {
				lo = sp.verse
			}
			hi = sp.verse
		}
	}
	return verseRangeCitation(p.st.CurrentBook, p.st.CurrentChapter, lo, hi)
}

// spannedDrag is one drag's outcome: whether its citation names the verses
// its shared words came from, and the detail to print when it does not.
type spannedDrag struct {
	right  bool
	detail string
}

func (p *spannedPage) drag(t *testing.T, from, to int) spannedDrag {
	t.Helper()
	raw := p.page[from:to]
	span := selSpanFromNative(p.verseAt[from], p.verseAt[to-1])
	text, cite, at, baseLen := prepareShareQuote(p.st, raw, span)
	if text == "" {
		t.Fatalf("%s %d: the drag %q span %+v shared nothing", p.st.CurrentBook, p.st.CurrentChapter, raw, span)
	}
	// The located words, without the terminal the completion may append.
	body := strings.TrimRight(text, ".!?…")
	if at >= 0 {
		body = text[:baseLen]
	}
	lo, hi := p.toCorpus[from], p.toCorpus[to]
	inDrag := p.corpus[lo:hi]
	truth := ""
	for i := 0; ; {
		j := strings.Index(inDrag[i:], body)
		if j < 0 {
			break
		}
		c := p.citeOf(lo+i+j, len(body))
		if truth != "" && c != truth {
			t.Fatalf("%s %d: the shared words %q stand twice in the drag %q with different verses; the sweep cannot say which was meant",
				p.st.CurrentBook, p.st.CurrentChapter, body, raw)
		}
		truth = c
		i += j + 1
	}
	detail := fmt.Sprintf("%s %d: drag %q span %d–%d shared %q cited %s", p.st.CurrentBook, p.st.CurrentChapter,
		raw, span.lo, span.hi, text, cite)
	if truth == "" {
		return spannedDrag{false, detail + ", words that are not in the drag"}
	}
	return spannedDrag{cite == truth, detail + ", want " + truth}
}

func TestSpannedBoundaryDragSweep(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	type family struct {
		name     string
		maxWords int
	}
	families := []family{{"punct", 12}, {"cross", 3}, {"number", 3}}
	drags := map[string][]int{}
	wrong := map[string][]int{}
	for _, f := range families {
		drags[f.name] = make([]int, f.maxWords+1)
		wrong[f.name] = make([]int, f.maxWords+1)
	}
	chapters, shown := 0, 0
	for _, book := range bd.Books {
		for _, ch := range bd.GetChapterNumbersForBook(book) {
			chapters++
			p := newSpannedPage(bd, book, ch)
			ends, first := p.words()
			for vi := 1; vi < len(p.spans); vi += step {
				// The marks that end the verse before: the last characters
				// before the space ahead of this verse's number, back to the
				// first letter or digit.
				var marks []int
				for at := p.number[vi] - 1; at > 0; {
					r, size := utf8.DecodeLastRuneInString(p.page[:at])
					if isWordRune(r) || r == ' ' {
						break
					}
					at -= size
					marks = append(marks, at)
				}
				lastWord := ends[first[vi]-1] - 1
				for lastWord > 0 && p.page[lastWord-1] != ' ' {
					lastWord--
				}
				for _, f := range families {
					var starts []int
					switch f.name {
					case "punct":
						starts = marks
					case "cross":
						starts = []int{lastWord}
					case "number":
						starts = []int{p.number[vi]}
					}
					for k := 1; k <= f.maxWords && first[vi]+k-1 < len(ends); k++ {
						for _, from := range starts {
							d := p.drag(t, from, ends[first[vi]+k-1])
							drags[f.name][k]++
							if !d.right {
								wrong[f.name][k]++
								if shown < 5 {
									shown++
									t.Logf("%s", d.detail)
								}
							}
						}
					}
				}
			}
		}
	}
	if chapters != 89 {
		t.Fatalf("swept %d chapters, want the 89 of the embedded Gospels", chapters)
	}
	total, totalWrong := 0, map[string]int{}
	for _, f := range families {
		var line []string
		n := 0
		for k := 1; k <= f.maxWords; k++ {
			n += drags[f.name][k]
			totalWrong[f.name] += wrong[f.name][k]
			line = append(line, fmt.Sprintf("%d words %d/%d", k, wrong[f.name][k], drags[f.name][k]))
		}
		total += n
		t.Logf("%s: %d of %d drags cite the wrong verses (%s)", f.name, totalWrong[f.name], n, strings.Join(line, ", "))
	}
	t.Logf("%d drags", total)

	// The rates as they stand, held exactly so that any change shows. A drag
	// that opens on the previous verse's punctuation is cited from an earlier
	// copy of its words in 974 of 59,199 drags: one in seven at one word, 2.8%
	// at two, none from ten. Five drags that begin on the previous verse's
	// last word are located at an earlier copy of the same words that does
	// not cross the boundary, and cite the one verse. Every drag that begins
	// on the verse's number is right.
	if testing.Short() {
		return
	}
	if totalWrong["punct"] != 974 || totalWrong["cross"] != 5 || totalWrong["number"] != 0 {
		t.Errorf("wrong citations: punct %d, cross %d, number %d; recorded 974, 5 and 0",
			totalWrong["punct"], totalWrong["cross"], totalWrong["number"])
	}
}
