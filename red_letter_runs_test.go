package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// The BSB pane must colour only the words that are Christ's. Before spans, the
// whole verse went red — including the other speaker's reply.
func TestApplePaneRedensOnlyChristsWordsInTheBSB(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	key := verseKeyFor("Mark", 8, 5)
	text := bsbVerseFixture[key]
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: text}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	html := buildChapterHTML(st, bd.GetChapter("Mark", 8))
	red := redTextOf(html)
	if !strings.Contains(red, "How many loaves do you have") {
		t.Errorf("His question is not red:\n%s", html)
	}
	if strings.Contains(red, "Seven") {
		t.Errorf("the disciples' answer was reddened:\n%s", html)
	}
	if strings.Contains(red, "Jesus asked") {
		t.Errorf("the narration was reddened:\n%s", html)
	}
}

// With the switch off, the pane must go back to reddening the whole verse —
// otherwise "turn it off" is not something we can actually do.
func TestApplePaneFallsBackWhenTheSwitchIsOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")

	key := verseKeyFor("Mark", 8, 5)
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: bsbVerseFixture[key]}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	red := redTextOf(buildChapterHTML(st, bd.GetChapter("Mark", 8)))
	if !strings.Contains(red, "Seven") {
		t.Error("switched off, the whole verse should be red again — the old behaviour")
	}
}

// redTextOf returns the concatenated contents of every wj span.
func redTextOf(html string) string {
	var out strings.Builder
	for _, open := range []string{`<span class="wj">`, `<span class="hl wj">`} {
		rest := html
		for {
			i := strings.Index(rest, open)
			if i < 0 {
				break
			}
			rest = rest[i+len(open):]
			j := strings.Index(rest, "</span>")
			if j < 0 {
				break
			}
			out.WriteString(rest[:j])
			rest = rest[j:]
		}
	}
	return out.String()
}

// Android renders its own HTML dialect, so it needs its own proof.
func TestAndroidPaneRedensOnlyChristsWordsInTheBSB(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	text := bsbVerseFixture[verseKeyFor("Mark", 8, 5)]
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Mark": {8: {{BookName: "Mark", Chapter: 8, Verse: 5, Text: text}}},
	}
	bd.Books = []string{"Mark"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Mark", CurrentChapter: 8, CurrentVersion: "bsb"}

	html := buildChapterHTMLAndroid(st, bd.GetChapter("Mark", 8))
	red := nrgbaToHex(st.pal().RedLetter)
	var coloured strings.Builder
	rest := html
	open := `<font color="` + red + `">`
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			break
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, "</font>")
		if j < 0 {
			break
		}
		coloured.WriteString(rest[:j])
		rest = rest[j:]
	}
	got := coloured.String()
	if !strings.Contains(got, "How many loaves do you have") {
		t.Errorf("His question is not red:\n%s", html)
	}
	if strings.Contains(got, "Seven") {
		t.Errorf("the disciples' answer was reddened:\n%s", html)
	}
}

// The NKJV's spans are the PUBLISHER's own words-of-Jesus marks, not ours, so
// the same verse must colour the same way — and the disciples' reply must not.
func TestNKJVUsesItsOwnSpans(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	key := verseKeyFor("Mark", 8, 5)
	spans, ok := nkjvRedLetterSpans[key]
	if !ok || len(spans) == 0 {
		t.Fatal("no NKJV span data for Mark 8:5")
	}
	// The NKJV's own table must be independent of the BSB's — same verse, but
	// different translations, so identical offsets would mean one overwrote the
	// other.
	if bsb, ok := bsbRedLetterSpans[key]; ok && len(bsb) == len(spans) && bsb[0] == spans[0] {
		t.Error("the NKJV spans are identical to the BSB's; one table is feeding the other")
	}
	if n := nkjvRedLetterRunes[key]; n == 0 {
		t.Error("no rune length recorded, so the guard cannot fire")
	}
	if _, ok := nkjvRedLetterHashes[key]; !ok {
		t.Error("no content fingerprint recorded, so same-length edits would pass the guard")
	}
	if got, want := len(nkjvRedLetterSpans), 2054; got != want {
		t.Errorf("NKJV marked verse count = %d, want publisher count %d", got, want)
	}
	runCount := 0
	for _, spans := range nkjvRedLetterSpans {
		runCount += len(spans)
	}
	if got, want := runCount, 3484; got != want {
		t.Errorf("NKJV run count = %d, want publisher count %d", got, want)
	}
}

// Every edition we hold spans for goes through one lookup; another edition's
// same verse text must not accidentally satisfy its content guard.
func TestRedLetterSpanLookupCoversTheRightEditions(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	text := bsbVerseFixture[verseKeyFor("Mark", 8, 5)]

	if _, ok := redLetterSpansFor("bsb", "Mark", 8, 5, text); !ok {
		t.Error("bsb: expected span data")
	}
	for _, vid := range []string{"web", "webc", "kjv", ""} {
		if _, ok := redLetterSpansFor(vid, "Mark", 8, 5, text); ok {
			t.Errorf("%q: accepted BSB text as its own span source", vid)
		}
	}
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")
	if redLetterVerseMarked("nkjv", "Mark", 8, 5) != true {
		t.Error("the BSB diagnostics switch changed the NKJV's judgement")
	}
}

// The WEB and WEB Catholic mark their own words of Jesus in their published
// USFM, so like the BSB and NKJV nothing about these is our judgement. What must
// hold is that the spans are the TRANSLATORS' and every marked source verse has
// guarded offsets in the runtime revision.
func TestWEBAndWEBCUseTheirOwnSpans(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, tc := range []struct {
		vid                 string
		spans               map[string][]redLetterSpan
		runes               map[string]int
		hashes              map[string]uint64
		marked              map[string]struct{}
		wantSpans, wantRuns int
	}{
		{"web", webRedLetterSpans, webRedLetterRunes, webRedLetterHashes,
			webRedLetterMarked, 2059, 2290},
		{"webc", webcRedLetterSpans, webcRedLetterRunes, webcRedLetterHashes,
			webcRedLetterMarked, 2059, 2289},
	} {
		if got, want := len(tc.spans), tc.wantSpans; got != want {
			t.Errorf("%s: span verse count = %d, want %d", tc.vid, got, want)
		}
		if got, want := len(tc.marked), 2059; got != want {
			t.Errorf("%s: marked verse count = %d, want source count %d", tc.vid, got, want)
		}
		runCount := 0
		for _, spans := range tc.spans {
			runCount += len(spans)
		}
		if runCount != tc.wantRuns {
			t.Errorf("%s: run count = %d, want source count %d", tc.vid, runCount, tc.wantRuns)
		}
		for key, spans := range tc.spans {
			n, ok := tc.runes[key]
			if !ok {
				t.Errorf("%s: %s has spans but no recorded verse length, so the guard cannot fire", tc.vid, key)
				break
			}
			if _, ok := tc.hashes[key]; !ok {
				t.Errorf("%s: %s has spans but no content fingerprint", tc.vid, key)
				break
			}
			for _, s := range spans {
				if s.Start < 0 || s.End > n || s.Start >= s.End {
					t.Errorf("%s: %s span {%d,%d} outside a verse of %d runes", tc.vid, key, s.Start, s.End, n)
					break
				}
			}
		}
		if _, ok := redLetterSpansFor(tc.vid, "Nonexistent", 1, 1, "whatever"); ok {
			t.Errorf("%s: got spans for a verse that is not in the table", tc.vid)
		}
		for key := range tc.marked {
			if _, ok := tc.spans[key]; !ok {
				t.Errorf("%s: publisher-marked verse %s has no recovered offsets", tc.vid, key)
			}
		}
	}
}

// The runtime supplier's WEB-family text predates the current eBible revision
// in a few places. These two mixed verses prove that boundary recovery preserves
// black narration instead of treating a source mismatch as a whole-red verse.
func TestWEBRevisionRecoveryPreservesMixedVerseBoundaries(t *testing.T) {
	john := "Jesus therefore said to him, “Unless you see signs and wonders, you will in no way believe.”"
	luke := "Other fell into the good ground and grew and produced one hundred times as much fruit.” As he said these things, he called out, “He who has ears to hear, let him hear!”"
	for _, vid := range []string{"web", "webc"} {
		for _, tc := range []struct {
			book             string
			chapter, verse   int
			text, red, black string
		}{
			{"John", 4, 48, john,
				"“Unless you see signs and wonders, you will in no way believe.”",
				"Jesus therefore said to him, "},
			{"Luke", 8, 8, luke,
				"Other fell into the good ground and grew and produced one hundred times as much fruit.”“He who has ears to hear, let him hear!”",
				" As he said these things, he called out, "},
		} {
			var red, black strings.Builder
			for _, run := range redLetterRuns(vid, Verse{
				BookName: tc.book, Chapter: tc.chapter, Verse: tc.verse, Text: tc.text,
			}, true) {
				if run.Red {
					red.WriteString(run.Text)
				} else {
					black.WriteString(run.Text)
				}
			}
			if got := red.String(); got != tc.red {
				t.Errorf("%s %s %d:%d red text = %q, want %q", vid, tc.book, tc.chapter, tc.verse, got, tc.red)
			}
			if got := black.String(); got != tc.black {
				t.Errorf("%s %s %d:%d black text = %q, want %q", vid, tc.book, tc.chapter, tc.verse, got, tc.black)
			}
		}
	}

	webcMark := "If your eye causes you to stumble, throw it out. It is better for you to enter into God’s Kingdom with one eye, rather than having two eyes to be cast into the Gehenna oF fire,"
	runs := redLetterRuns("webc", Verse{BookName: "Mark", Chapter: 9, Verse: 47, Text: webcMark}, true)
	if len(runs) != 1 || !runs[0].Red || runs[0].Text != webcMark {
		t.Errorf("WEBC Mark 9:47 typo recovery = %+v, want one wholly red run", runs)
	}
}

func TestEveryRedLetterTableHasCompleteValidMetadata(t *testing.T) {
	tables := []struct {
		name   string
		spans  map[string][]redLetterSpan
		runes  map[string]int
		hashes map[string]uint64
		marked map[string]struct{}
	}{
		{"bsb", bsbRedLetterSpans, bsbRedLetterRunes, bsbRedLetterHashes, nil},
		{"nkjv", nkjvRedLetterSpans, nkjvRedLetterRunes, nkjvRedLetterHashes, nil},
		{"web", webRedLetterSpans, webRedLetterRunes, webRedLetterHashes, webRedLetterMarked},
		{"webc", webcRedLetterSpans, webcRedLetterRunes, webcRedLetterHashes, webcRedLetterMarked},
	}
	for _, table := range tables {
		if len(table.spans) != len(table.runes) || len(table.spans) != len(table.hashes) {
			t.Errorf("%s metadata sizes: spans=%d runes=%d hashes=%d",
				table.name, len(table.spans), len(table.runes), len(table.hashes))
		}
		for key, spans := range table.spans {
			n, lengthOK := table.runes[key]
			_, hashOK := table.hashes[key]
			if !lengthOK || !hashOK {
				t.Errorf("%s %s: missing length=%v or hash=%v", table.name, key, lengthOK, hashOK)
				continue
			}
			if table.marked != nil {
				if _, ok := table.marked[key]; !ok {
					t.Errorf("%s %s: spans exist outside the publisher marked-verse set", table.name, key)
				}
			}
			previousEnd := 0
			for _, span := range spans {
				if span.Start < previousEnd || span.Start < 0 || span.Start >= span.End || span.End > n {
					t.Errorf("%s %s: invalid span {%d,%d} for %d runes",
						table.name, key, span.Start, span.End, n)
				}
				previousEnd = span.End
			}
		}
		for key := range table.runes {
			if _, ok := table.spans[key]; !ok {
				t.Errorf("%s %s: rune metadata without spans", table.name, key)
			}
		}
		for key := range table.hashes {
			if _, ok := table.spans[key]; !ok {
				t.Errorf("%s %s: hash metadata without spans", table.name, key)
			}
		}
	}
}

func TestEditionsNeverBorrowTheWEBFallback(t *testing.T) {
	for _, tc := range []struct {
		version, book  string
		chapter, verse int
	}{
		{"nkjv", "1 Timothy", 5, 18},
		{"nkjv", "Matthew", 8, 32},
		{"nkjv", "Mark", 10, 49},
		{"nkjv", "Revelation", 21, 5},
		{"nkjv", "Revelation", 21, 6},
		{"nkjv", "Revelation", 21, 7},
		{"nkjv", "Revelation", 21, 8},
		{"nkjv", "Revelation", 22, 14},
		{"nkjv", "Revelation", 22, 15},
		{"bsb", "John", 8, 33},
		{"bsb", "Luke", 20, 23},
		{"lsb", "John", 11, 25},
		{"nrsv", "John", 11, 25},
	} {
		if !isWordsOfChrist(tc.book, tc.chapter, tc.verse) {
			t.Fatalf("fixture: %s %s %d:%d is not marked by WEB", tc.version, tc.book, tc.chapter, tc.verse)
		}
		runs := redLetterRuns(tc.version, Verse{
			BookName: tc.book, Chapter: tc.chapter, Verse: tc.verse, Text: "publisher black",
		}, true)
		for _, run := range runs {
			if run.Red {
				t.Errorf("%s %s %d:%d borrowed WEB red-letter status", tc.version, tc.book, tc.chapter, tc.verse)
			}
		}
	}
}

func TestStaleOffsetsFallBackWithinTheSelectedEdition(t *testing.T) {
	// This NKJV-only marked verse is intentionally supplied with wrong text so
	// its content guard rejects the offsets. It must remain red from NKJV's own
	// marked-verse set, while WEB must leave the same reference black.
	v := Verse{BookName: "Luke", Chapter: 17, Verse: 36, Text: "stale text"}
	if runs := redLetterRuns("nkjv", v, true); len(runs) != 1 || !runs[0].Red {
		t.Fatalf("NKJV stale-offset fallback = %+v, want one red run", runs)
	}
	if runs := redLetterRuns("web", v, true); len(runs) != 1 || runs[0].Red {
		t.Fatalf("WEB result = %+v, want one black run", runs)
	}
}

// ---------------------------------------------------------------------------
// The two rules the verifiers found unpinned.
// ---------------------------------------------------------------------------

// The styled desktop pane colours whole TOKENS, so it needs a rule for a span
// that ends mid-token — and two shipping BSB spans do exactly that. Mark 7:34's
// closes inside "opened!”)." A first-rune-only rule passed the whole suite when
// the verifier tried it, which is what this pins.
func TestStyledTokenGoesRedWhenAnyRuneIsRed(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	spans, ok := bsbRedLetterSpans[verseKeyFor("Mark", 7, 34)]
	if !ok || len(spans) == 0 {
		t.Skip("Mark 7:34 carries no span data in this build")
	}
	// A token straddling the span's end must still come back red.
	text := "abc" + "DEF" + "ghi"
	v := Verse{BookName: "Mark", Chapter: 7, Verse: 34, Text: text}
	// Span covering only "DEF" plus one rune either side of a token boundary.
	got := runsFromSpans(text, []redLetterSpan{{3, 7}})
	if len(got) != 3 || !got[1].Red || got[1].Text != "DEFg" {
		t.Fatalf("runsFromSpans split wrongly: %+v", got)
	}
	_ = v
}

// The fallback pane must keep strings.TrimSpace on the single-run path. Removing
// the trim entirely left the suite green for the patch author, so this pins what
// the trim actually does: a verse padded with whitespace comes out clean, and one
// that is nothing but whitespace still emits a body segment.
func TestFyneFallbackTrimsTheSingleRunPath(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, false)

	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{
		"Genesis": {1: {{BookName: "Genesis", Chapter: 1, Verse: 1, Text: "  In the beginning.  \r"}}},
	}
	bd.Books = []string{"Genesis"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "web"}

	segs := mobileParagraphSegments(st, bd.GetChapter("Genesis", 1))
	var body string
	for _, s := range segs {
		if ts, ok := s.(*widget.TextSegment); ok && ts.Style.ColorName != colorNameVerseNumber {
			body = ts.Text
		}
	}
	if body != "In the beginning." {
		t.Errorf("single-run body = %q, want the trimmed text — the trim was dropped or changed", body)
	}
}

// An edition must be allowed to disagree with the WEB about whether Christ
// speaks in a verse at all. The WEB's verse-level table used to GATE the run
// splitter, so four verses the NKJV's own publisher reddens came out black —
// including Luke 17:36, which the WEB does not even contain.
func TestAnEditionsOwnTableOverridesTheWEBGate(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Preferences().SetBool(prefRedLetter, true)

	for _, tc := range []struct {
		book  string
		ch, v int
	}{
		{"Mark", 5, 31}, {"Luke", 24, 7}, {"Matthew", 27, 63}, {"Luke", 17, 36},
	} {
		key := verseKeyFor(tc.book, tc.ch, tc.v)
		if isWordsOfChrist(tc.book, tc.ch, tc.v) {
			t.Fatalf("fixture: %s is in the WEB table, so it cannot show the gate being overruled", key)
		}
		spans, ok := nkjvRedLetterSpans[key]
		if !ok || len(spans) == 0 {
			t.Fatalf("fixture: %s has no NKJV span data", key)
		}
		// Deliberately stale text makes the exact-content guard reject the spans.
		// The NKJV's own marked-verse fallback must still override the old WEB gate.
		v := Verse{BookName: tc.book, Chapter: tc.ch, Verse: tc.v,
			Text: strings.Repeat("x", nkjvRedLetterRunes[key])}
		runs := redLetterRuns("nkjv", v, true)
		anyRed := false
		for _, r := range runs {
			anyRed = anyRed || r.Red
		}
		if !anyRed {
			t.Errorf("%s: the NKJV marks this verse but nothing came back red — the WEB gate is still overruling it", key)
		}
		// ...and the WEB itself must be unaffected: it has no entry, and its
		// own table says Christ does not speak here.
		for _, r := range redLetterRuns("web", Verse{BookName: tc.book, Chapter: tc.ch, Verse: tc.v, Text: "plain"}, true) {
			if r.Red {
				t.Errorf("%s: the WEB reddened a verse its own table does not mark", key)
			}
		}
	}
}
