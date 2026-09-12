package main

// The web reader's note chrome comes from the SAME Go functions the app panes
// consume — step 9 of docs/NOTE_CHROME_UNIFICATION.md. These pins hold the
// seam in both directions: the template carries no private spelling of a
// shared value, and the generate-time fill emits exactly what the shared
// functions answer. Divergences that remain (the verse target's
// header-inclusive margin, the own verb arm's absence, minimize's global
// highlight suppress) are stated in the template beside the code they excuse.

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func TestWebReaderNoteChromeComesFromTheSharedFunctions(t *testing.T) {
	for _, ph := range []string{"__NOTE_BYLINE__", "__NOTE_PILL_LABEL__"} {
		if !strings.Contains(readerJSTemplate, ph) {
			t.Errorf("readerJSTemplate lost %s — the value is composed in Go and must be emitted", ph)
		}
	}
	if strings.Contains(readerJSTemplate, "'Note from Friend'") {
		t.Error("readerJSTemplate spells the byline itself again — senderByline is the one author")
	}
	if !strings.Contains(readerCSSTemplate, "__NOTE_LEAD__") {
		t.Error("readerCSSTemplate lost __NOTE_LEAD__ — the arrival lead is the spec's, not this file's")
	}

	js := readerJS(nil)
	wantByline, err := json.Marshal(bibletext.WebNoteByline())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, string(wantByline)) {
		t.Errorf("the generated reader.js does not carry the shared byline %s", wantByline)
	}
	wantPill, err := json.Marshal(bibletext.WebNotePillLabel())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, string(wantPill)) {
		t.Errorf("the generated reader.js does not carry the shared pill label %s", wantPill)
	}
	for _, ph := range []string{"__NOTE_BYLINE__", "__NOTE_PILL_LABEL__"} {
		if strings.Contains(js, ph) {
			t.Errorf("%s survives into the generated reader.js unfilled", ph)
		}
	}

	css := readerCSS(webFonts{uiRegular: "r.woff2", uiBold: "b.woff2", scriptureRegular: "s.woff2", scriptureBold: "sb.woff2"})
	if want := fmt.Sprintf("scroll-margin-top:%dpx", bibletext.WebNoteArrivalLeadPx()); !strings.Contains(css, want) {
		t.Errorf("the generated reader.css does not carry the shared arrival lead %q", want)
	}
	if strings.Contains(css, "__NOTE_LEAD__") {
		t.Error("__NOTE_LEAD__ survives into the generated reader.css unfilled")
	}

	// The band's air is the spec's (noteMetrics), stacked on whatever stands
	// above the card the way every native reserves it. The generated sheet is
	// checked for the filled numbers, and the TEMPLATE's own rules for the
	// placeholder spellings — so a px literal typed into a rule fails even when
	// it happens to equal today's spec — and for the mechanism: an inline-level
	// card, whose margins cannot collapse with the paragraph gap or a heading's
	// tail (block-level, the spec's numbers would stop meaning what they mean
	// everywhere else).
	m := bibletext.WebNoteGapAbovePx()
	below := bibletext.WebNoteTailDepthPx() + bibletext.WebNoteGapBelowPx()
	for _, want := range []string{
		"display:inline-block; width:100%; vertical-align:top;",
		fmt.Sprintf("margin:%dpx 0 %dpx;", m, below),
		fmt.Sprintf(".note.notail{margin-bottom:%dpx}", bibletext.WebNoteGapBelowPx()),
		fmt.Sprintf("margin:calc(%dpx - var(--pgap, 0rem)/2) 0 calc(%dpx + var(--pgap, 0rem)/2);", m, bibletext.WebNoteGapBelowPx()),
		fmt.Sprintf(".text .notechip:first-child, .notechip.notail{margin:%dpx 0 %dpx}", m, bibletext.WebNoteGapBelowPx()),
		fmt.Sprintf(".text .sec + .notechip{margin:calc(%dpx - var(--htail)/2) 0 calc(%dpx + var(--htail)/2)}", m, bibletext.WebNoteGapBelowPx()),
		fmt.Sprintf(".text p.pst + .notechip{margin:calc(%dpx - var(--tgap)/2) 0 calc(%dpx + var(--tgap)/2)}", m, bibletext.WebNoteGapBelowPx()),
		fmt.Sprintf("--htail:calc(%s * ", strconv.FormatFloat(bibletext.ReadingHeadTailEm(), 'f', -1, 64)),
		fmt.Sprintf("--tgap:calc(%s * ", strconv.FormatFloat(bibletext.ReadingTitleGapEm(), 'f', -1, 64)),
		fmt.Sprintf("min-height:%dpx; min-width:%dpx;", bibletext.WebNotePillHPx(), bibletext.WebNotePillMinWPx()),
		fmt.Sprintf("padding:.3rem %dpx;", bibletext.WebNotePillPadXPx()),
		"box-sizing:border-box;\n  bottom:calc(-" + webNoteTailHalf() + "px - 1px);",
		fmt.Sprintf("width:%spx; height:%spx;", webNoteTailSide(), webNoteTailSide()),
	} {
		if !strings.Contains(css, want) {
			t.Errorf("the generated reader.css does not carry the spec's band air: missing %q", want)
		}
	}
	rule := func(sel string) string {
		i := strings.Index(readerCSSTemplate, sel)
		if i < 0 {
			t.Fatalf("readerCSSTemplate lost the %s rule", sel)
		}
		r := readerCSSTemplate[i:]
		return r[:strings.Index(r, "}")+1]
	}
	for sel, wants := range map[string][]string{
		".note{":        {"display:inline-block; width:100%; vertical-align:top;", "margin:__NOTE_GAP_ABOVE__px 0 __NOTE_BAND_BELOW__px;"},
		".note.notail{": {"margin-bottom:__NOTE_GAP_BELOW__px"},
		".note::after{": {"box-sizing:border-box;", "bottom:calc(-__NOTE_TAIL_HALF__px - 1px);", "width:__NOTE_TAIL_SIDE__px; height:__NOTE_TAIL_SIDE__px;"},
		".notechip{":    {"min-height:__NOTE_PILL_H__px; min-width:__NOTE_PILL_MIN_W__px;", "margin:calc(__NOTE_GAP_ABOVE__px - var(--pgap, 0rem)/2) 0 calc(__NOTE_GAP_BELOW__px + var(--pgap, 0rem)/2);", "padding:.3rem __NOTE_PILL_PAD_X__px;"},
		".text .notechip:first-child, .notechip.notail{": {"margin:__NOTE_GAP_ABOVE__px 0 __NOTE_GAP_BELOW__px"},
		".text .sec + .notechip{":                        {"calc(__NOTE_GAP_ABOVE__px - var(--htail)/2) 0 calc(__NOTE_GAP_BELOW__px + var(--htail)/2)"},
		".text p.pst + .notechip{":                       {"calc(__NOTE_GAP_ABOVE__px - var(--tgap)/2) 0 calc(__NOTE_GAP_BELOW__px + var(--tgap)/2)"},
		".text{--pgap:":                                  {"--htail:calc(__HEAD_TAIL_FACTOR__ * __SCRIPTURE_REM__)", "--tgap:calc(__TITLE_GAP_FACTOR__ * __SCRIPTURE_REM__)"},
	} {
		r := rule(sel)
		for _, w := range wants {
			if !strings.Contains(r, w) {
				t.Errorf("the template's %s rule does not spell %q — the number or the mechanism is this file's own again", sel, w)
			}
		}
	}
	for _, ph := range []string{"__NOTE_GAP_ABOVE__", "__NOTE_GAP_BELOW__", "__NOTE_BAND_BELOW__", "__NOTE_TAIL_SIDE__", "__NOTE_TAIL_HALF__", "__NOTE_PILL_H__", "__NOTE_PILL_PAD_X__", "__NOTE_PILL_MIN_W__"} {
		if strings.Contains(css, ph) {
			t.Errorf("%s survives into the generated reader.css unfilled", ph)
		}
	}
	// The card's and the chip's rules carry no margin of the template's own:
	// the notice card (.notenotice) is chrome outside the spec and keeps its.
	for _, sel := range []string{".note{", ".notechip{", ".text .notechip:first-child"} {
		i := strings.Index(readerCSSTemplate, sel)
		if i < 0 {
			t.Fatalf("readerCSSTemplate lost the %s rule", sel)
		}
		rule := readerCSSTemplate[i:]
		rule = rule[:strings.Index(rule, "}")+1]
		if strings.Contains(rule, "rem 0") || strings.Contains(rule, "calc(1.1rem") {
			t.Errorf("the %s rule spells a margin of its own again — the band's air is noteMetrics': %q", sel, rule)
		}
	}
}

// The tail is the spec's shape: a square of side TailWidth/√2 turned 45° with
// its centre on the card's edge hangs TailWidth/2 = TailDepth below it. The
// spec says 9 and 18, so the two must agree or the formula is wrong.
func TestWebReaderTailIsTheSpecsShape(t *testing.T) {
	if got, want := bibletext.WebNoteTailWidthPx(), 2*bibletext.WebNoteTailDepthPx(); got != want {
		t.Fatalf("the turned-square tail hangs TailWidth/2 below the card; the spec's TailWidth %d is not 2 × TailDepth %d", got, bibletext.WebNoteTailDepthPx())
	}
	side, _ := strconv.ParseFloat(webNoteTailSide(), 64)
	half, _ := strconv.ParseFloat(webNoteTailHalf(), 64)
	if d := side / math.Sqrt2; math.Abs(d-float64(bibletext.WebNoteTailDepthPx())) > 0.01 {
		t.Errorf("tail side %.2f hangs %.2f below the edge; the spec's TailDepth is %d", side, d, bibletext.WebNoteTailDepthPx())
	}
	if math.Abs(half*2-side) > 0.011 {
		t.Errorf("tail half %.2f is not half the side %.2f", half, side)
	}
}

// The tail rule: a card has a tail iff its anchor names a passage
// (noteChrome.hasTail). The web's spelling is a class the chapter-top parking
// sets and the stylesheet gates the tail on.
func TestWebReaderTailObeysTheSharedRule(t *testing.T) {
	for _, frag := range []string{
		"el.classList.add('notail');",
		"el.classList.remove('notail');",
	} {
		if !strings.Contains(readerJSTemplate, frag) {
			t.Errorf("readerJSTemplate lost %q — an anchorless card would grow a tail claiming the first paragraph", frag)
		}
	}
	if !strings.Contains(readerCSSTemplate, ".note.notail::after{display:none}") {
		t.Error("readerCSSTemplate lost the notail gate — the class would be set and change nothing")
	}
}

// One verb vocabulary: the bubble and the card offer the same action under the
// same name. Counted over CODE — the template's comments narrate the old
// second vocabulary and must not count as a parser.
func TestWebReaderSpeaksOneVerbVocabulary(t *testing.T) {
	src := stripJSLineComments(readerJSTemplate)
	if strings.Contains(src, "'Hide note'") {
		t.Error("the second vocabulary is back: 'Hide note' and 'Minimize note' are one action")
	}
	if got := strings.Count(src, "'Minimize note'"); got != 2 {
		t.Errorf("'Minimize note' appears %d times, want 2 — the card's button and the bubble's", got)
	}
	if got := strings.Count(src, "'Delete note'"); got != 2 {
		t.Errorf("'Delete note' appears %d times, want 2 — the card's button and the bubble's", got)
	}
}

func stripJSLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
