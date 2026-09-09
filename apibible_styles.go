package bibletext

// The NKJV style census — docs/SCRIPTURE_WORKLIST.md, S7, first part only.
//
// WHY ONLY THE CENSUS, AND NOT THE SKIP LIST
//
// S7 asks for three things: extend apiBibleSkipPara to the full USFM heading
// families, deny character styles that are not Scripture, and census what
// arrives. The first two cannot ship in a Stage-2 change, and the reason is
// worth stating so nobody re-derives it.
//
// Adding a style to apiBibleSkipPara moves any block of that style OFF the
// prose path: its words leave Verse.Text where a verse was open, and it becomes
// a BibleData.Headings entry, which chapter_blocks.go draws unconditionally on
// all five surfaces. Adding a character style to a live denylist removes those
// characters from Verse.Text. Either one, if the style occurs even once in the
// canon, is a decoded-text change AND a drawn change — which means a cacheEpoch
// bump and a full re-download of a LICENSED edition against a metered quota.
// Stage 2's whole premise is "no epoch, no reader-visible change".
//
// And nothing on disk can tell us whether those styles occur. The cached
// chapters are New Testament only; the titles-on/titles-off comparison bounds
// only the styles the titles flag strips. So the honest order is: census first,
// learn what the canon actually sends, then extend the lists in a change that
// owns its epoch. A style the census reports as never carrying text can be
// added later with no epoch at all, because for a text-free block the skip path
// and the prose path leave identical state — which is why the census records
// that split rather than a bare count.
//
// Nothing in this file is consulted by the decoder's routing. Every function
// here decides which COUNTER to increment and nothing else.

import (
	"sort"
	"strings"
)

// apiBibleKnownParaStyles are the paragraph styles the decoder has a considered
// answer for: the ones it skips (apiBibleSkipPara), the superscription, and the
// prose and poetry families it deliberately walks into verse text.
//
// Measured against the cached NT chapters, only p, s, q1, q2 and pc occur; the
// Old Testament is unmeasured, so this set is a considered list rather than an
// observation, and the census exists precisely because the first full-canon run
// will probably report styles beyond it. THAT IS THE CHECK WORKING. Re-measure
// from what the run reports; do not widen this to silence a log line.
var apiBibleKnownParaStyles = map[string]bool{
	// skipped as headings or non-scripture (apiBibleSkipPara)
	"qa": true, "cl": true, "cd": true, "mr": true, "sr": true, "r": true, "sp": true,
	"s": true, "s1": true, "s2": true, "s3": true, "s4": true,
	"ms": true, "ms1": true, "ms2": true, "ms3": true,
	// the Psalm superscription, handled on its own path
	"d": true,
	// prose
	"p": true, "m": true, "mi": true, "pi": true, "pi1": true, "pi2": true,
	"nb": true, "pc": true, "pr": true, "pm": true, "po": true, "cls": true,
	// poetry
	"q": true, "q1": true, "q2": true, "q3": true, "q4": true,
	"qc": true, "qr": true, "qm": true, "qm1": true, "qm2": true, "qm3": true,
	"b": true,
	// lists
	"li": true, "li1": true, "li2": true, "li3": true, "li4": true,
	"lh": true, "lf": true, "lim1": true, "lim2": true,
}

// apiBibleKnownCharStyles are the character styles the decoder has an answer
// for: the two it renders as spans, and the ones it walks through transparently.
var apiBibleKnownCharStyles = map[string]bool{
	"nd": true, "sc": true, // the divine name and small capitals — drawn as spans
	"wj": true, // words of Jesus
	"it": true, "bd": true, "bdit": true, "em": true, "no": true,
	"add": true, // supplied words
	"qs":  true, // Selah
	"tl":  true, "pn": true, "k": true, "ord": true, "sig": true, "w": true,
}

// apiBibleStyleCensus records every paragraph and character style one NKJV
// fetch actually saw, and whether a block of that style carried any text.
//
// Nil is a working value.
type apiBibleStyleCensus struct {
	label string // the book being decoded, so a report names where to look
	logf  func(format string, v ...any)

	paraWithText  map[string]int
	paraEmpty     map[string]int
	charStyles    map[string]int
	unnamedBlocks int
}

func newAPIBibleStyleCensus(label string, logf func(string, ...any)) *apiBibleStyleCensus {
	return &apiBibleStyleCensus{
		label:        label,
		logf:         logf,
		paraWithText: make(map[string]int, 32),
		paraEmpty:    make(map[string]int, 8),
		charStyles:   make(map[string]int, 16),
	}
}

// para records one block. hasText separates the two cases that matter for
// deciding, later, whether the style can be skipped without owing an epoch.
func (c *apiBibleStyleCensus) para(style string, hasText bool) {
	if c == nil {
		return
	}
	if style == "" {
		c.unnamedBlocks++
		return
	}
	if hasText {
		c.paraWithText[style]++
		return
	}
	c.paraEmpty[style]++
}

// char records one character span's style.
func (c *apiBibleStyleCensus) char(style string) {
	if c == nil {
		return
	}
	if style == "" {
		style = "(none)"
	}
	c.charStyles[style]++
}

// report names only what the decoder has no considered answer for. A full
// inventory on every fetch would be unreadable; the unknowns are the news.
func (c *apiBibleStyleCensus) report() {
	if c == nil {
		return
	}
	var unknownPara []string
	for s, n := range c.paraWithText {
		if !apiBibleKnownParaStyles[s] {
			unknownPara = append(unknownPara, formatCount(s+" (with text)", n))
		}
	}
	for s, n := range c.paraEmpty {
		if !apiBibleKnownParaStyles[s] {
			unknownPara = append(unknownPara, formatCount(s+" (no text)", n))
		}
	}
	if len(unknownPara) > 0 {
		sort.Strings(unknownPara)
		c.logf("bibletext: NKJV style census: %s: paragraph styles with no considered answer: %s",
			c.label, strings.Join(unknownPara, ", "))
	}

	var unknownChar []string
	for s, n := range c.charStyles {
		if !apiBibleKnownCharStyles[s] {
			unknownChar = append(unknownChar, formatCount(s, n))
		}
	}
	if len(unknownChar) > 0 {
		sort.Strings(unknownChar)
		c.logf("bibletext: NKJV style census: %s: character styles with no considered answer: %s",
			c.label, strings.Join(unknownChar, ", "))
	}
	if c.unnamedBlocks > 0 {
		c.logf("bibletext: NKJV style census: %s: %d blocks carried no style at all", c.label, c.unnamedBlocks)
	}
}

// styles is every paragraph style seen, for tests.
func (c *apiBibleStyleCensus) styles() map[string]int {
	if c == nil {
		return nil
	}
	all := make(map[string]int, len(c.paraWithText)+len(c.paraEmpty))
	for s, n := range c.paraWithText {
		all[s] += n
	}
	for s, n := range c.paraEmpty {
		all[s] += n
	}
	return all
}

// apiBibleBlockHasText reports whether a block contributes any characters at
// all. It is the split that decides, later, whether a style could be added to
// apiBibleSkipPara without owing a cache epoch: for a text-free block the skip
// path and the prose path leave identical state, so skipping it changes nothing
// a reader could see.
func apiBibleBlockHasText(items []apiBibleNode) bool {
	for _, n := range items {
		if strings.TrimSpace(n.Text) != "" {
			return true
		}
		if len(n.Items) > 0 && apiBibleBlockHasText(n.Items) {
			return true
		}
	}
	return false
}
