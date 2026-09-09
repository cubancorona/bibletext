package bibletext

// The decode-time feed census — docs/SCRIPTURE_WORKLIST.md, S6.
//
// The decoder names four chapter-level node kinds and four verse-content item
// shapes. Everything else it drops: the chapter loop falls through its type
// tests, and the verse-content switch in bsbVerseTextMarkedLevels has no
// default at all, so an item shape nobody anticipated contributes nothing and
// leaves no trace. Nothing is lost today — measured against the live feeds on
// 9 September 2026, every node and every item matches something we handle —
// but that is a fact about today's feed, not a property of the code. A feed
// that starts sending a new kind would be invisible.
//
// So: count what arrives, name what matched nothing, and say it ONCE per fetch.
//
// What this is not: a validator. It never fails a fetch and never changes a
// decode. An edition that grows a node kind we do not understand is still an
// edition a reader can read, and the words we already understand are still the
// words. The census only ensures nobody has to guess whether that happened.
//
// Cost: one map increment per chapter-level node (~35,000 per edition, against
// an 8 MB JSON parse — immaterial). The expensive part, unmarshalling an item
// again to report its key set, runs ONLY on the unmatched path, which no
// current edition reaches.

import (
	"encoding/json"
	"sort"
	"strings"
)

// helloAOKnownNodeTypes is what the chapter loop in decodeHelloAOChapters
// actually handles. Pinned in helloao_census_test.go against the captured
// feeds, so adding a branch without adding it here is a test failure.
var helloAOKnownNodeTypes = map[string]bool{
	"verse":           true,
	"line_break":      true,
	"heading":         true,
	"hebrew_subtitle": true,
}

// helloAOCensus accumulates one edition's shape report across all its books.
//
// Every method tolerates a nil receiver, so the decoder can be handed nil in a
// test that does not care and the call sites stay free of guards.
type helloAOCensus struct {
	edition string
	logf    func(format string, v ...any)

	nodeTypes  map[string]int // every chapter-level type seen, known or not
	itemShapes map[string]int // key sets of verse items that matched no case
	badNodes   int            // chapter-level nodes that would not unmarshal
	badItems   int            // verse-content items that would not unmarshal
}

func newHelloAOCensus(edition string, logf func(string, ...any)) *helloAOCensus {
	if edition == "" {
		edition = "helloao"
	}
	return &helloAOCensus{
		edition:    edition,
		logf:       logf,
		nodeTypes:  make(map[string]int, 8),
		itemShapes: make(map[string]int, 4),
	}
}

// node records one chapter-level node's type. An empty type is a node that
// carries none, which is itself worth knowing.
func (c *helloAOCensus) node(t string) {
	if c == nil {
		return
	}
	if t == "" {
		t = "(no type)"
	}
	c.nodeTypes[t]++
}

// badNode records a chapter-level node that would not unmarshal at all.
func (c *helloAOCensus) badNode() {
	if c == nil {
		return
	}
	c.badNodes++
}

// unmatchedItem records a verse-content item that reached no case in the
// content switch — the silent drop this whole check exists for.
//
// It reports the item's KEY SET rather than a bare count, because a count says
// only "something changed" while a key set says WHAT changed, which is the
// difference between a line worth reading and a line worth ignoring. This is
// the only place that unmarshals an item twice, and it runs on a path no
// current edition reaches.
func (c *helloAOCensus) unmatchedItem(raw json.RawMessage) {
	if c == nil {
		return
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keyed); err != nil {
		c.badItems++
		return
	}
	keys := make([]string, 0, len(keyed))
	for k := range keyed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	shape := "{" + strings.Join(keys, ",") + "}"
	if len(keys) == 0 {
		shape = "{} (empty object)"
	}
	c.itemShapes[shape]++
}

// report says once what the whole edition sent. It is silent when everything
// matched, because a line printed on every successful download is a line its
// reader learns to skip.
func (c *helloAOCensus) report() {
	if c == nil {
		return
	}
	var unknown []string
	for t, n := range c.nodeTypes {
		if !helloAOKnownNodeTypes[t] {
			unknown = append(unknown, formatCount(t, n))
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		c.logf("bibletext: feed census: %s: chapter-level node types the decoder does not handle: %s",
			c.edition, strings.Join(unknown, ", "))
	}
	if len(c.itemShapes) > 0 {
		var shapes []string
		for s, n := range c.itemShapes {
			shapes = append(shapes, formatCount(s, n))
		}
		sort.Strings(shapes)
		c.logf("bibletext: feed census: %s: verse-content item shapes that reached no case: %s",
			c.edition, strings.Join(shapes, ", "))
	}
	if c.badNodes > 0 || c.badItems > 0 {
		c.logf("bibletext: feed census: %s: %d chapter nodes and %d verse items would not parse",
			c.edition, c.badNodes, c.badItems)
	}
}

// seen is the set of chapter-level node types this census recorded. For tests.
func (c *helloAOCensus) seen() map[string]int {
	if c == nil {
		return nil
	}
	return c.nodeTypes
}

func formatCount(name string, n int) string {
	return name + "×" + itoa(n)
}

// itoa avoids pulling strconv into this file for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
