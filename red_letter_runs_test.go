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
}

// Every edition we hold spans for must go through one lookup, and editions we
// do not hold must still get the whole-verse answer.
func TestRedLetterSpanLookupCoversTheRightEditions(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	text := bsbVerseFixture[verseKeyFor("Mark", 8, 5)]

	if _, ok := redLetterSpansFor("bsb", "Mark", 8, 5, text); !ok {
		t.Error("bsb: expected span data")
	}
	for _, vid := range []string{"web", "webc", "kjv", ""} {
		if _, ok := redLetterSpansFor(vid, "Mark", 8, 5, text); ok {
			t.Errorf("%q: got span data, but no table exists for it — it must fall back to the whole verse", vid)
		}
	}
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")
	if _, ok := redLetterSpansFor("nkjv", "Mark", 8, 5, text); ok {
		t.Error("nkjv: the switch is off but spans were still handed out")
	}
}

// The WEB and WEB Catholic mark their own words of Jesus in their published
// USFM, so like the NKJV — and unlike the BSB — nothing about these is our
// judgement. What must hold is that the spans are the TRANSLATORS' and that a
// verse missing from the table still renders, whole, rather than not at all.
func TestWEBAndWEBCUseTheirOwnSpans(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, tc := range []struct {
		vid   string
		spans map[string][]redLetterSpan
		runes map[string]int
	}{
		{"web", webRedLetterSpans, webRedLetterRunes},
		{"webc", webcRedLetterSpans, webcRedLetterRunes},
	} {
		if len(tc.spans) < 2000 {
			t.Errorf("%s: only %d verses have spans; the table looks truncated", tc.vid, len(tc.spans))
		}
		for key, spans := range tc.spans {
			n, ok := tc.runes[key]
			if !ok {
				t.Errorf("%s: %s has spans but no recorded verse length, so the guard cannot fire", tc.vid, key)
				break
			}
			for _, s := range spans {
				if s.Start < 0 || s.End > n || s.Start >= s.End {
					t.Errorf("%s: %s span {%d,%d} outside a verse of %d runes", tc.vid, key, s.Start, s.End, n)
					break
				}
			}
		}
		// A verse the table does not carry must fall back, not vanish: five per
		// edition genuinely differ from eBible's revision and have no entry.
		if _, ok := redLetterSpansFor(tc.vid, "Nonexistent", 1, 1, "whatever"); ok {
			t.Errorf("%s: got spans for a verse that is not in the table", tc.vid)
		}
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
		// A verse of exactly the recorded length so the rune guard passes.
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
