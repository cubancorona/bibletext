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

func assertCutInvariants(t *testing.T, st *AppState, corpus string, label, raw string) {
	t.Helper()
	// Sub-word drags (no complete word in the selection) are degenerate: the
	// mid-word repair rightly leaves nothing, and the pipeline falls back to
	// sharing the fragment verbatim — pinned separately in
	// TestShareSelectionSubWordFragment, excluded from the invariant sweep.
	if !strings.Contains(strings.TrimSpace(raw), " ") {
		return
	}
	text, cite, _, _ := prepareShareQuote(st, raw, selSpan{})
	if text == "" {
		return
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
	_, spans := chapterProse(st)
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
			assertCutInvariants(t, st, corpus,
				fmt.Sprintf("%s %d end-cut@%d", cc.book, cc.chapter, e), raw)
		}
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
	corpus, _ := chapterProse(st)
	step := 1
	if testing.Short() {
		step = 7
	}
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
		assertCutInvariants(t, st, corpus, fmt.Sprintf("John 1 start-cut@%d", s), corpus[s:e])
	}
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
		corpus, _ := chapterProse(st)
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
				text, _, _, _ := prepareShareQuote(st, raw, selSpan{})
				if text == "" {
					continue
				}
				// The one non-negotiable: a verse-number token NEVER survives.
				bodyText := strings.TrimRight(text, ".!?…")
				if strings.Index(corpus, bodyText) < 0 {
					t.Fatalf("%s: output is not chapter prose (marker leak?): %q", label, text)
				}
				assertCutInvariants(t, st, corpus, label, raw)
			}
		}
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
		assertCutInvariants(t, st, corpus,
			fmt.Sprintf("Psalms 23 poetry end-cut@%d", e), raw)
	}
}
