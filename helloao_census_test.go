package bibletext

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The census exists so a feed that grows a shape cannot do it quietly. What
// these tests hold are the two halves of that: the known sets are what the
// decoder really handles, and an unknown shape really does get named.

// The verse-content item shapes the three editions send today, measured live on
// 9 September 2026. Not asserted as a closed set — the feed is free to add one,
// and that is precisely the event the census reports rather than forbids — but
// recorded so a reader of this test knows what "normal" was.
//
//	bare string                    bsb 31,180  web 23,831  webc 26,579
//	{text, poem}                   bsb 24,507  web 18,872  webc 23,365
//	{noteId}                       bsb  4,853  web  1,226  webc  1,673
//	{lineBreak}                    bsb  3,636  web  2,773  webc  3,493
//	{text, wordsOfJesus}           bsb      0  web  2,178  webc  2,178
//	{text, poem, wordsOfJesus}     bsb      0  web    111  webc    111
//	{text, descriptive}            bsb      1  web     21  webc     21
//
// Every one of them reaches a case in the content switch, so the census reports
// nothing on a healthy feed.

func TestTheCensusNamesAnUnknownChapterNode(t *testing.T) {
	r := &recorder{}
	c := newHelloAOCensus("WEB", r.logf)
	c.node("verse")
	c.node("verse")
	c.node("interlinear_gloss") // not a kind this decoder handles
	c.report()

	got := r.joined()
	if !strings.Contains(got, "interlinear_gloss") {
		t.Errorf("an unhandled node type was not named:\n%s", got)
	}
	if strings.Contains(got, "verse×") {
		t.Errorf("a handled node type was reported as unhandled:\n%s", got)
	}
}

// CONTROL for the whole file: the census must be SILENT on a feed it fully
// understands, or every real report would be lost in noise and the check would
// be worthless in the only situation it exists for.
func TestTheCensusIsSilentOnAFeedItUnderstands(t *testing.T) {
	r := &recorder{}
	c := newHelloAOCensus("BSB", r.logf)
	for _, kind := range []string{"verse", "line_break", "heading", "hebrew_subtitle"} {
		c.node(kind)
	}
	c.report()
	if len(r.lines) != 0 {
		t.Errorf("the census spoke about a feed it entirely handles:\n%s", r.joined())
	}
}

// The item half, and the reason it reports a KEY SET rather than a count: a
// count says only that something changed, while the keys say what.
func TestAnUnmatchedItemIsReportedByItsKeys(t *testing.T) {
	r := &recorder{}
	c := newHelloAOCensus("WEB", r.logf)
	c.unmatchedItem(json.RawMessage(`{"strongs":"H430","lemma":"אֱלֹהִים"}`))
	c.unmatchedItem(json.RawMessage(`{"strongs":"H776","lemma":"אֶרֶץ"}`))
	c.report()

	got := r.joined()
	if !strings.Contains(got, "{lemma,strongs}") {
		t.Errorf("the item's key set was not reported:\n%s", got)
	}
	if !strings.Contains(got, "×2") {
		t.Errorf("repeated shapes should be counted, not repeated:\n%s", got)
	}
	// The keys must be sorted, or the same shape would report under two names
	// depending on Go's map iteration order and the count would split.
	if strings.Contains(got, "{strongs,lemma}") {
		t.Errorf("keys are not sorted, so one shape can report under two names:\n%s", got)
	}
}

// The real hole this item was written for: an item that unmarshals cleanly into
// the decoder's struct and matches no case. Before the census it produced no
// text, no marker, no break and no trace at all.
func TestAnItemThatMatchesNoCaseIsCensusedNotSwallowed(t *testing.T) {
	r := &recorder{}
	c := newHelloAOCensus("WEB", r.logf)

	// Shape the decoder cannot use: no text, no noteId, no lineBreak.
	content := []json.RawMessage{
		json.RawMessage(`{"text":"In the beginning","poem":1}`),
		json.RawMessage(`{"strongs":"H7225"}`),
	}
	text, _, _, _ := bsbVerseTextMarkedLevelsChecked(content, &helloAOChecks{Census: c}, "")

	if text != "In the beginning" {
		t.Errorf("the census changed the decoded text: %q", text)
	}
	c.report()
	if !strings.Contains(r.joined(), "{strongs}") {
		t.Errorf("an item that reached no case was not reported:\n%s", r.joined())
	}
}

// A nil census must be usable everywhere, or every call site grows a guard.
func TestANilCensusIsSafe(t *testing.T) {
	var c *helloAOCensus
	c.node("verse")
	c.badNode()
	c.unmatchedItem(json.RawMessage(`{"anything":1}`))
	c.report()
	if c.seen() != nil {
		t.Error("a nil census should report nothing seen")
	}
}

// Against the real feeds: today every node and every item is handled, so the
// census must find nothing. If this ever fails, the feed changed — which is the
// check working, not the test being wrong. Re-measure before widening anything.
func TestTheLiveFeedsAreFullyAccountedFor(t *testing.T) {
	for _, tc := range []struct {
		path   string
		decode func([]byte) (*BibleData, error)
	}{
		{"build/biblecache/bsb.json", decodeCanonical66},
		{"build/biblecache/web.json", decodeWEB},
		{"build/biblecache/webc.json", decodeHelloAOCatholic},
	} {
		body, err := os.ReadFile(tc.path)
		if err != nil {
			t.Skipf("%s not present (build/ is gitignored); skipping", tc.path)
		}
		var doc struct {
			Books []helloAOBook `json:"books"`
		}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}

		r := &recorder{}
		c := newHelloAOCensus(tc.path, r.logf)
		nodes := 0
		for _, b := range doc.Books {
			decodeHelloAOChapters(b.ID, b, &helloAOChecks{Census: c})
		}
		for _, n := range c.seen() {
			nodes += n
		}
		c.report()

		// Anti-vacuity: a census that saw nothing would pass this test while
		// checking nothing at all.
		if nodes < 30000 {
			t.Fatalf("%s: the census saw only %d chapter nodes; it is not being "+
				"driven and this test proves nothing", tc.path, nodes)
		}
		if len(r.lines) != 0 {
			t.Errorf("%s: the feed sent something the decoder does not handle:\n%s",
				tc.path, r.joined())
		}
		for kind := range c.seen() {
			if !helloAOKnownNodeTypes[kind] {
				t.Errorf("%s: unhandled chapter node type %q", tc.path, kind)
			}
		}
	}
}
