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
	"regexp"
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
// styled pane's runs give the same answer). Four kinds of drag start at each
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
//	       and nothing is cut from the front. k runs to 3;
//	digit  begins on a later digit of verse N's number, the "9" of 29. The span
//	       still starts at N, but what the drag hands over opens "9 He said
//	       to", which reads as verse 9's number wherever verse 9 opens with
//	       the same words. k runs to 3.
//
// The truth for a drag is where its shared words stand inside the drag: the
// verses that stretch of the chapter overlaps. A shared text that does not
// occur inside the drag at all is wrong whatever it cites. And because every
// one of these drags carries a verse number, the pipeline must place its words
// inside the drag too (prepareShareQuote's at): the completion, the original
// terminal and the line-break restore all read the chapter around that place.
// The one exception is a digit drag the pipeline cannot place, which it
// shares as it came, stray digit and all, cited by the span: that is not
// wrong about the verse, and is counted apart.
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
// its shared words came from and its words were placed inside the drag, and
// the detail to print when not. verbatim is a drag the pipeline did not place
// at all (at < 0), shared as it came and cited by exactly the drag's verses.
type spannedDrag struct {
	right    bool
	verbatim bool
	detail   string
}

func (p *spannedPage) drag(t *testing.T, from, to int, lead bool) spannedDrag {
	t.Helper()
	raw := p.page[from:to]
	span := selSpanFromNative(p.verseAt[from], p.verseAt[to-1])
	if lead {
		// What every pane sends with the span: the page from verse lo's
		// number to the drag's first character (selSpan.lead).
		for i, sp := range p.spans {
			if sp.verse == span.lo {
				span = selSpanWithLead(span.lo, span.hi, p.page[p.number[i]:from])
				break
			}
		}
	}
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
	if at < 0 {
		whole := p.citeOf(lo, hi-lo)
		return spannedDrag{verbatim: cite == whole, detail: detail + ", not placed; the drag is " + whole}
	}
	if truth == "" {
		return spannedDrag{detail: detail + ", words that are not in the drag"}
	}
	if at < lo || at+baseLen > hi {
		return spannedDrag{detail: fmt.Sprintf("%s, placed at %d outside the drag's %d–%d", detail, at, lo, hi)}
	}
	return spannedDrag{right: cite == truth, detail: detail + ", want " + truth}
}

// spannedFamily is one kind of drag the spanned sweep makes at each verse
// boundary, run on to at most maxWords words of the verse.
type spannedFamily struct {
	name     string
	maxWords int
}

var spannedFamilies = []spannedFamily{{"punct", 12}, {"cross", 3}, {"number", 3}, {"digit", 3}}

func TestSpannedBoundaryDragSweep(t *testing.T) {
	step := 1
	if testing.Short() {
		step = 7
	}
	drags, wrong, verbatim := spannedBoundarySweep(t, step, false)
	total, totalWrong := reportSpannedSweep(t, drags, wrong)
	t.Logf("%d drags; %d digit drags shared as they came", total, verbatim)

	// None may be wrong. Before the selection was placed by its verse number
	// and kept in place through the repairs (normalizeShareSelection), a drag
	// that opens on the previous verse's punctuation was cited from an earlier
	// copy of its words in 974 of these 59,199 drags: one in seven at one
	// word, 2.8% at two, none from ten. Five that begin on the previous
	// verse's last word were placed at an earlier copy of the same words
	// inside that verse and cited it alone. Those that begin on the verse's
	// number were right.
	//
	// A drag begun on a later digit of a verse's number was placed where the
	// digit says, as if it were a number of its own, in 141 of these 8,931
	// drags when that placing was first made, before the span had to agree:
	// Mark 7's "9 He said to", spanned 29, cited 7:9.
	for _, f := range spannedFamilies {
		if totalWrong[f.name] != 0 {
			t.Errorf("%s: %d drags cite verses their words did not come from, or place the words outside the drag",
				f.name, totalWrong[f.name])
		}
	}
	// The rest of the digit drags are shared as they came, the stray digit
	// in the quotation and the citation right: "0 Hezekiah" for Matthew 1:10.
	// Held exactly so that a change shows; the backlog has it (BACKLOG.md,
	// "A drag begun inside a verse number shares the digit").
	if !testing.Short() && verbatim != 8769 {
		t.Errorf("%d digit drags shared as they came, stray digit and all; recorded 8,769", verbatim)
	}
}

// The same drags, each with the lead every pane sends (selSpan.lead): the
// page from verse lo's number to the drag's first character. None is wrong,
// and no digit drag is shared as it came: the lead says the drag began inside
// the number, so the rest of the number is taken off as the page's apparatus
// instead of being quoted.
func TestSpannedBoundaryDragSweepWithLeads(t *testing.T) {
	step := 1
	if testing.Short() {
		step = 7
	}
	drags, wrong, verbatim := spannedBoundarySweep(t, step, true)
	total, totalWrong := reportSpannedSweep(t, drags, wrong)
	t.Logf("%d drags with their leads; %d digit drags shared as they came", total, verbatim)
	for _, f := range spannedFamilies {
		if totalWrong[f.name] != 0 {
			t.Errorf("%s, with the lead: %d drags cite verses their words did not come from, or place the words outside the drag",
				f.name, totalWrong[f.name])
		}
	}
	if verbatim != 0 {
		t.Errorf("with the lead, %d digit drags were shared as they came, stray digit and all", verbatim)
	}
}

// spannedBoundarySweep makes every drag of the spanned sweep over the
// embedded Gospels, each with the lead the panes send when lead is set, and
// counts by family and word count the drags made and those wrong, and the
// digit drags shared as they came.
func spannedBoundarySweep(t *testing.T, step int, lead bool) (drags, wrong map[string][]int, verbatim int) {
	t.Helper()
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	drags = map[string][]int{}
	wrong = map[string][]int{}
	for _, f := range spannedFamilies {
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
				for _, f := range spannedFamilies {
					var starts []int
					switch f.name {
					case "punct":
						starts = marks
					case "cross":
						starts = []int{lastWord}
					case "number":
						starts = []int{p.number[vi]}
					case "digit":
						starts = nil
						for at := p.number[vi] + 1; at < p.text[vi]-1; at++ {
							starts = append(starts, at)
						}
					}
					for k := 1; k <= f.maxWords && first[vi]+k-1 < len(ends); k++ {
						for _, from := range starts {
							d := p.drag(t, from, ends[first[vi]+k-1], lead)
							drags[f.name][k]++
							if !d.right && f.name == "digit" && d.verbatim {
								verbatim++
								continue
							}
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
	return drags, wrong, verbatim
}

// reportSpannedSweep logs each family's counts and returns the drags made and
// the wrong ones by family.
func reportSpannedSweep(t *testing.T, drags, wrong map[string][]int) (total int, totalWrong map[string]int) {
	t.Helper()
	totalWrong = map[string]int{}
	for _, f := range spannedFamilies {
		var line []string
		n := 0
		for k := 1; k <= f.maxWords; k++ {
			n += drags[f.name][k]
			totalWrong[f.name] += wrong[f.name][k]
			line = append(line, fmt.Sprintf("%d words %d/%d", k, wrong[f.name][k], drags[f.name][k]))
		}
		total += n
		t.Logf("%s: %d of %d drags wrong (%s)", f.name, totalWrong[f.name], n, strings.Join(line, ", "))
	}
	return total, totalWrong
}

// wordToken is a word as a double-click takes it: letters and digits, with
// an apostrophe between two of them ("man’s", "Isn’t") inside the word.
var wordToken = regexp.MustCompile(`[\p{L}\p{Nd}]+(?:['’][\p{L}\p{Nd}]+)*`)

// wholeWordCopy is the first offset of sub in s at which a double-click, or a
// drag from word to word, could have taken it: neither end of the copy falls
// inside a word of s (wordToken). -1 if there is none.
func wholeWordCopy(s, sub string) int {
	inside := make([]bool, len(s)+1)
	for _, m := range wordToken.FindAllStringIndex(s, -1) {
		for i := m[0] + 1; i < m[1]; i++ {
			inside[i] = true
		}
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.HasPrefix(s[i:], sub) && !inside[i] && !inside[i+len(sub)] {
			return i
		}
	}
	return -1
}

// psalmTitlePage is a page that draws a psalm's title, perhaps a heading
// under it, and the psalm, as one text a drag can run through: the title
// ends at titleEnd, and verse i's number and its text begin at number[i]
// and text[i], its text as long as its span of the chapter's prose.
type psalmTitlePage struct {
	page         string
	titleEnd     int
	number, text []int
}

// verseAt is the verse whose number is the last at or before page offset
// at, 0 for none: what a verse lookup on the page gives.
func (p psalmTitlePage) verseAt(at int, spans []verseSpan) int {
	v := 0
	for i := range spans {
		if p.number[i] <= at {
			v = spans[i].verse
		}
	}
	return v
}

// spacedPsalmTitlePage is the page as the Apple and Android panes draw it,
// each piece a space after the one before: the title, the heading, then
// "1 …".
func spacedPsalmTitlePage(st *AppState, withHeading bool) psalmTitlePage {
	corpus, spans := chapterProse(st)
	var b strings.Builder
	b.WriteString(collapseSpaces(st.Bible.SuperscriptionFor(st.CurrentBook, st.CurrentChapter).DrawnText()))
	p := psalmTitlePage{titleEnd: b.Len(), number: make([]int, len(spans)), text: make([]int, len(spans))}
	if withHeading {
		b.WriteString(" " + psalmHeading)
	}
	for i, sp := range spans {
		b.WriteByte(' ')
		p.number[i] = b.Len()
		b.WriteString(strconv.Itoa(sp.verse) + " ")
		p.text[i] = b.Len()
		b.WriteString(corpus[sp.start:sp.end])
	}
	p.page = b.String()
	return p
}

// sweepPsalmTitleDrags drags from every character of the title on p to
// every character boundary through it, the heading under it if there is
// one, verse 1 and into verse 2, with the span spanAt gives for a drag that
// ends at a page offset, and checks every answer against psalmTitleState's
// Psalm 3 (TestPsalmTitleDragSweep says what each must be). It returns how
// many drags it made, and how many of them stayed inside the title and were
// quoted from the psalm.
func sweepPsalmTitleDrags(t *testing.T, st *AppState, p psalmTitlePage, spanAt func(to int) selSpan, name string) (drags, psalmCopies int) {
	t.Helper()
	corpus, spans := chapterProse(st)
	page := p.page
	// toCorpus maps a page offset inside verse text to chapter prose; an
	// offset before verse 1's text took none of it.
	toCorpus := func(at int) int {
		c := 0
		for i, sp := range spans {
			if at >= p.text[i] {
				c = sp.start + at - p.text[i]
				if c > sp.end {
					c = sp.end
				}
			} else if at > p.number[i] {
				c = sp.start
			}
		}
		return c
	}
	verseOf := func(off int) int { // the verse holding corpus offset off
		for _, sp := range spans {
			if off >= sp.start && off < sp.end {
				return sp.verse
			}
		}
		return 0
	}
	title := page[:p.titleEnd]
	limit := p.text[1] + len("Many there are")
	// Every drag begins in the title. One begun in the heading under it is
	// a heading's, whose rules are the heading tests' (a piece of a word
	// there that is also in the psalm is found in the psalm, as anywhere
	// a heading stands).
	for from := 0; from < p.titleEnd; from++ {
		if !utf8.RuneStart(page[from]) {
			continue
		}
		for to := from + 1; to <= limit; to++ {
			if to < len(page) && !utf8.RuneStart(page[to]) {
				continue
			}
			raw := page[from:to]
			if strings.TrimSpace(raw) == "" {
				continue // a space between two words, which no menu acts on
			}
			drags++
			span := spanAt(to)
			quote, cite, at, baseLen := prepareShareQuote(st, raw, span)

			// The psalm's words the drag holds, from verse 1's start, cut
			// back to the last whole word.
			took := corpus[:toCorpus(to)]
			if r, _ := utf8.DecodeRuneInString(corpus[len(took):]); len(took) < len(corpus) && isWordRune(r) {
				cut := strings.LastIndexByte(took, ' ')
				if cut < 0 {
					cut = 0
				}
				took = took[:cut]
			}
			hasWord := strings.IndexFunc(took, isWordRune) >= 0

			label := fmt.Sprintf("%s: drag %q span %d–%d", name, raw, span.lo, span.hi)
			if sel := collapseSpaces(raw); !span.valid() && strings.Contains(title, sel) && strings.IndexFunc(sel, isWordRune) >= 0 {
				if copyAt := wholeWordCopy(corpus, sel); copyAt >= 0 {
					psalmCopies++
					start := copyAt + strings.IndexFunc(sel, isWordRune)
					end := copyAt + len(strings.TrimRight(sel, ",;:—"))
					want := verseRangeCitation("Psalms", 3, verseOf(start), verseOf(end-1))
					if quote == "" || at != start || baseLen != end-start || cite != want {
						t.Fatalf("%s is letters the psalm has as words, but shared (%q, %q) placed at %d; want %q from %d, cited %s",
							label, quote, cite, at, corpus[start:end], start, want)
					}
					continue
				}
			}
			if !hasWord {
				if quote != "" || cite != "Psalms 3:1" {
					t.Fatalf("%s holds no word of the psalm, but shared (%q, %q); want (\"\", Psalms 3:1)", label, quote, cite)
				}
				continue
			}
			if at != 0 || baseLen == 0 || !strings.HasPrefix(corpus, quote[:baseLen]) {
				t.Fatalf("%s shared %q placed at %d; want the psalm's words from verse 1's start", label, quote, at)
			}
			lo, hi := 0, 0
			for _, sp := range spans {
				if sp.start < baseLen {
					if lo == 0 {
						lo = sp.verse
					}
					hi = sp.verse
				}
			}
			if want := verseRangeCitation("Psalms", 3, lo, hi); cite != want {
				t.Fatalf("%s shared %q cited %s, want %s", label, quote, cite, want)
			}
		}
	}
	if psalmCopies == 0 {
		t.Fatalf("%s: no drag inside the title is letters the psalm has as words, so that rule is not swept", name)
	}
	return drags, psalmCopies
}

// TestPsalmTitleDragSweep drags from every character of a psalm's title to
// every character boundary through it, a heading set under it, verse 1 and
// into verse 2, on a page drawn as the Apple and Android panes draw it: the
// title, the heading, then "1 …". Those panes never hand such a drag over
// (their verbs start at verse 1's number, see psalmTitle), and since the
// Apple panes' fallback page carries its verse numbers it does not either;
// TestADragFromATitleWithNoSpanSharesThePsalmsWordsAlone sweeps that page's
// layout with no span, the shape a selection with no position still has.
// The span here is what a verse
// lookup on the page would give (nothing at or before the start, so no span
// until the drag reaches verse 1's number, then 1 to the verse at its end).
// Three answers are possible, and the drag's own verse words say which:
//
//   - it holds at least one whole word of the psalm: those words are shared,
//     from verse 1's start, and cited by the verses they span. No piece of
//     the title or the heading is ever in the quotation;
//   - it stays inside the title, with no span, and the psalm has the same
//     letters as whole words: they are quoted from the first such copy and
//     cited by its verse, since nothing tells them from the same letters
//     selected there. The fixture's verses hold "a" and "is" as words, which
//     the title has inside "David" and "his", so the sweep meets this; a
//     psalm that shares whole words with its title is
//     TestPsalmTitleWordsWithNoSpanSweep's;
//   - otherwise, only the title or the heading, perhaps with verse 1's
//     number or part of its first word: nothing is quoted and verse 1 is
//     cited, as for a heading selected on its own.
func TestPsalmTitleDragSweep(t *testing.T) {
	for _, withHeading := range []bool{false, true} {
		st := psalmTitleState(withHeading)
		_, spans := chapterProse(st)
		p := spacedPsalmTitlePage(st, withHeading)
		spanAt := func(to int) selSpan { return selSpanFromNative(0, p.verseAt(to-1, spans)) }
		drags, copies := sweepPsalmTitleDrags(t, st, p, spanAt, fmt.Sprintf("heading=%v", withHeading))
		t.Logf("heading=%v: %d drags, %d of them inside the title and quoted from the psalm", withHeading, drags, copies)
	}
}

// TestPsalmTitleWordsWithNoSpanSweep selects every piece of a psalm's title,
// from every character to every later boundary, with no span, as a page
// with no verse geometry hands a selection over (the Apple panes' fallback
// page did until it carried its verse numbers). The
// psalm shares words with its title (psalmWordsState), and with no span
// nothing tells the title's letters from the same letters in the psalm. Two
// answers are possible:
//
//   - letters the psalm holds as whole words (wholeWordCopy), whether the
//     title has them as words ("Yahweh") or inside longer ones ("his" in
//     "this"), perhaps with the punctuation beside them: quoted from the
//     first such copy in the psalm, past any punctuation the selection
//     opens with, and cited by the verses that copy spans;
//   - anything else, the title whole, a phrase only the title has, letters
//     the psalm has only inside longer words ("s" and "’s" of "Saul’s"
//     against "king’s"), punctuation alone: nothing quoted and verse 1
//     cited, as for a title selected on its own.
//
// Each of the first kind was cited as verse 1 with nothing quoted while the
// title was answered before the psalm was searched, and those the title has
// inside longer words still were while only whole words of the title were
// looked for there; "’s" was quoted as the "s" of "king’s".
func TestPsalmTitleWordsWithNoSpanSweep(t *testing.T) {
	st := psalmWordsState()
	corpus, spans := chapterProse(st)
	title := collapseSpaces(st.Bible.SuperscriptionFor("Psalms", 18).DrawnText())
	verseAt := func(off int) int {
		for _, sp := range spans {
			if off >= sp.start && off < sp.end {
				return sp.verse
			}
		}
		return 0
	}

	seen := map[string]bool{}
	words, inside := 0, 0
	for from := 0; from < len(title); from++ {
		if !utf8.RuneStart(title[from]) {
			continue
		}
		for to := from + 1; to <= len(title); to++ {
			if to < len(title) && !utf8.RuneStart(title[to]) {
				continue
			}
			sel := strings.TrimSpace(title[from:to])
			if sel == "" || seen[sel] {
				continue
			}
			seen[sel] = true
			quote, cite, at, baseLen := prepareShareQuote(st, sel, selSpan{})

			copyAt := -1
			if strings.IndexFunc(sel, isWordRune) >= 0 {
				copyAt = wholeWordCopy(corpus, sel)
			}
			if copyAt < 0 {
				if quote != "" || cite != "Psalms 18:1" {
					t.Fatalf("%q with no span is the title's alone, but shared (%q, %q); want (\"\", Psalms 18:1)", sel, quote, cite)
				}
				continue
			}
			words++
			if wholeWordCopy(title, sel) < 0 {
				inside++
			}
			start := copyAt + strings.IndexFunc(sel, isWordRune)
			end := copyAt + len(strings.TrimRight(sel, ",;:—"))
			want := verseRangeCitation("Psalms", 18, verseAt(start), verseAt(end-1))
			if quote == "" || at != start || baseLen != end-start || cite != want {
				t.Fatalf("%q with no span shared (%q, %q) placed at %d; want %q from %d, cited %s",
					sel, quote, cite, at, corpus[start:end], start, want)
			}
		}
	}
	if words == 0 || inside == 0 {
		t.Fatalf("the psalm holds %d of the title's selections as words, %d of them inside the title's longer words; the sweep needs both kinds", words, inside)
	}
	t.Logf("%d selections, %d of them letters the psalm holds as words, %d of those inside a longer word of the title", len(seen), words, inside)
}
