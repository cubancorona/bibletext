package bibletext

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// S19 of docs/SCRIPTURE_WORKLIST.md: an omitted verse's hole is marked in the
// text, on every surface, on the footnotes toggle, from the offline table.
//
// The Berean's Matthew 17 is the fixture throughout: it omits verse 21 and
// leaves no note explaining it, which is exactly the case the orphan-driven
// design could not reach and the table can.

func gappedState() (*AppState, []Verse) {
	verses := []Verse{
		{BookName: "Matthew", Book: "Matthew", Chapter: 17, Verse: 20,
			Text: "Because of your unbelief; for truly I say to you, nothing will be impossible for you."},
		{BookName: "Matthew", Book: "Matthew", Chapter: 17, Verse: 22,
			Text: "While they were staying in Galilee, Jesus said to them, The Son of Man is about to be delivered up."},
	}
	bd := &BibleData{
		Books:  []string{"Matthew"},
		Verses: map[string]map[int][]Verse{"Matthew": {17: verses}},
	}
	return &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 17, CurrentVersion: "bsb"}, verses
}

// --- the shared decision -----------------------------------------------------

func TestAGapIsFiledUnderTheVerseItPrecedes(t *testing.T) {
	_, verses := gappedState()
	got := gapsBefore("bsb", "Matthew", 17, verses)
	if len(got[22]) != 1 || got[22][0] != 21 {
		t.Errorf("gaps = %v, want {22: [21]}", got)
	}
}

// CONTROL: the licensed edition has no table and must produce nothing; and a
// hole the table names but nothing rendered stands before is the chapter
// starting late, not a gap.
func TestNoTableAndNoInteriorMeansNoMarks(t *testing.T) {
	_, verses := gappedState()
	if got := gapsBefore("nkjv", "Matthew", 17, verses); got != nil {
		t.Errorf("the licensed edition produced marks: %v", got)
	}
	// Render only verse 22 onward: 21 is now below the first rendered verse.
	if got := gapsBefore("bsb", "Matthew", 17, verses[1:]); got != nil {
		t.Errorf("a hole before the first rendered verse was marked: %v", got)
	}
	if got := gapsBefore("bsb", "Matthew", 17, nil); got != nil {
		t.Errorf("an empty chapter produced marks: %v", got)
	}
}

// --- the Apple dialect -------------------------------------------------------

func TestTheAppleDialectMarksTheHoleOnlyWhenFootnotesAreOn(t *testing.T) {
	st, verses := gappedState()
	setFootnotesEnabled(false)
	off := buildChapterHTML(st, verses)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := buildChapterHTML(st, verses)

	if strings.Contains(off, "[21]") || strings.Contains(off, "vg") {
		t.Errorf("footnotes off, but the mark is on the page:\n%s", off)
	}
	if !strings.Contains(on, `class="vg">[21]</span>`) {
		t.Errorf("footnotes on, but Matthew 17:21's hole is unmarked:\n%s", on)
	}
	// It stands before verse 22's number, after verse 20.
	if strings.Index(on, "[21]") > strings.Index(on, `<sup class="v">22</sup>`) {
		t.Errorf("the mark is not ahead of the verse it precedes:\n%s", on)
	}

	// THE PHANTOM-VERSE GUARD. The iOS and macOS verse indexes read every
	// small run's integerValue: a mark that were a <sup>, or whose small run
	// touched a number's small run, could invent verse 21 or lose verse 22.
	// So: exactly one <sup> per rendered verse, the mark is never a <sup>,
	// and a body-size character separates the mark from the number.
	if got := strings.Count(on, "<sup"); got != len(verses) {
		t.Errorf("%d <sup> runs for %d verses — the mark must not be one", got, len(verses))
	}
	if strings.Contains(on, "</span><sup") {
		t.Errorf("the mark's small run touches the number's small run; they would coalesce:\n%s", on)
	}
	if !strings.Contains(on, `[21]</span> <sup`) {
		t.Errorf("no body-size space between the mark and the number:\n%s", on)
	}
}

// --- the Android dialect -----------------------------------------------------

func TestTheAndroidDialectMarksTheHoleWithoutASup(t *testing.T) {
	st, verses := gappedState()
	setFootnotesEnabled(false)
	off := buildChapterHTMLAndroid(st, verses)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := buildChapterHTMLAndroid(st, verses)

	if strings.Contains(off, "[21]") {
		t.Errorf("footnotes off, but the mark is on the page:\n%s", off)
	}
	if !strings.Contains(on, "[21]") {
		t.Errorf("footnotes on, but the hole is unmarked:\n%s", on)
	}
	// BtBridge sets the chapter's content end at the first non-digit <sup>.
	// A mark written as one would clamp every verb from there down.
	if got := strings.Count(on, "<sup>"); got != len(verses) {
		t.Errorf("%d <sup> for %d verses — the mark must not be a <sup>", got, len(verses))
	}
	if strings.Contains(on, "<sup><small><font") && strings.Contains(on, "[21]</font></small></sup>") {
		t.Errorf("the mark was written inside a <sup>:\n%s", on)
	}
}

// --- the styled pane ---------------------------------------------------------

// The mark is drawn but NEVER in the text model: lay.Text is byte-identical
// with the toggle on or off, so copy, selection offsets and verse attribution
// cannot see it. This is the same guarantee the footnote section already
// carries, extended to the mark by construction rather than by stripping.
func TestTheStyledPaneDrawsTheMarkOutsideTheTextModel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st, _ := gappedState()
	setFootnotesEnabled(false)
	off := newTestPane(t, st, 420)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	on := newTestPane(t, st, 420)

	if off.lay.Text != on.lay.Text {
		t.Fatalf("lay.Text changed with the mark on:\n off: %q\n  on: %q", off.lay.Text, on.lay.Text)
	}
	if strings.Contains(on.lay.Text, "[21]") {
		t.Fatal("the mark entered the text model")
	}

	drawn := 0
	for _, dr := range on.drawRuns {
		if dr.Kind == runVerseGap {
			drawn++
			if dr.Text != "[21]" {
				t.Errorf("gap run text = %q, want [21]", dr.Text)
			}
		}
	}
	if drawn != 1 {
		t.Errorf("%d gap runs drawn, want 1", drawn)
	}
	for _, dr := range off.drawRuns {
		if dr.Kind == runVerseGap {
			t.Error("a gap run was drawn with footnotes off")
		}
	}

	// And a select-all copy carries nothing of it.
	on.selectAll()
	clip := &fakeClipboard{}
	on.clipboard = clip
	on.copyToClipboard()
	if strings.Contains(clip.content, "[21]") || strings.Contains(clip.content, "21") {
		t.Errorf("select-all copy carried the mark: %q", clip.content)
	}
	if !strings.Contains(clip.content, "unbelief") || !strings.Contains(clip.content, "Galilee") {
		t.Errorf("the copy lost scripture: %q", clip.content)
	}
}

// --- the outbound funnels ----------------------------------------------------

func TestOutboundTextStripsTheMarkAndItsSpace(t *testing.T) {
	got := outboundText("the other left. [36] They answered")
	if got != "the other left. They answered" {
		t.Errorf("outboundText = %q", got)
	}
	// Two marks, and one at the very start.
	got = outboundText("[44] where their worm [46] does not die")
	if got != "where their worm does not die" {
		t.Errorf("outboundText = %q", got)
	}
}

// CONTROL: only a bracketed run of DIGITS is the mark. Anything a publisher
// might actually bracket must survive.
func TestOutboundTextKeepsEveryOtherBracket(t *testing.T) {
	for _, s := range []string{
		"[God] is love",
		"a [12a] reference",
		"[ 36 ] with spaces",
		"[] empty",
		"36] and [36",
	} {
		if got := outboundText(s); got != s {
			t.Errorf("outboundText(%q) = %q; it should be untouched", s, got)
		}
	}
}

// The strip is by SHAPE, which is only safe while no publisher text carries
// that shape. Walked against the feeds; skips in CI where they are absent.
func TestNoShippedVerseContainsABracketedNumber(t *testing.T) {
	pat := regexp.MustCompile(`\[\d+\]`)
	checked := 0
	for _, path := range []string{
		"build/biblecache/bsb.json", "build/biblecache/web.json", "build/biblecache/webc.json",
	} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", path)
		}
		var doc struct {
			Books []helloAOBook `json:"books"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatal(err)
		}
		for _, b := range doc.Books {
			for _, w := range b.Chapters {
				for _, node := range w.Chapter.Content {
					var head struct {
						Type    string            `json:"type"`
						Content []json.RawMessage `json:"content"`
					}
					if json.Unmarshal(node, &head) != nil || head.Type != "verse" {
						continue
					}
					text := bsbVerseText(head.Content)
					checked++
					if pat.MatchString(text) {
						t.Errorf("%s %s %d carries a bracketed number, which outboundText would strip: %q",
							path, b.ID, w.Chapter.Number, text)
					}
				}
			}
		}
	}
	if checked < 30000 {
		t.Fatalf("only %d verses checked; this test proves nothing", checked)
	}
}

// --- the native scans the mark relies on ------------------------------------

// The mark is invisible to the three native verse indexes because of how they
// classify a run, and nothing on this host can execute them. So the property
// is pinned in the source: a change that stopped keying on integerValue or on
// SuperscriptSpan would have to delete this test to land.
func TestTheNativeVerseIndexesStillIgnoreABracketedNumber(t *testing.T) {
	for path, wants := range map[string][]string{
		"reading_ios.go":        {"integerValue", "if (v > 0)"},
		"reading_macos.go":      {"integerValue", "if (v <= 0) return;"},
		"android/BtBridge.java": {"SuperscriptSpan.class", "parseLeadingInt"},
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s no longer contains %q — the verse index may have stopped "+
					"skipping a bracketed number, and a gap mark could invent a verse", path, want)
			}
		}
		if strings.Contains(string(src), "this string is not in the file") {
			t.Fatal("the control string matched; this test proves nothing")
		}
	}
}
