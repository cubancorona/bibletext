package bibletext

// The NKJV style census — docs/SCRIPTURE_WORKLIST.md, S7.
//
// S7 asked for three things: extend apiBibleSkipPara to the full USFM heading
// families, deny character styles that are not Scripture, and census what
// arrives. The census shipped first because the other two could not be decided
// without it — adding a style to apiBibleSkipPara moves any block of that style
// off the prose path, so its words leave Verse.Text and it becomes a Heading
// that every surface draws, which is a decoded-text change, a cacheEpoch bump,
// and a re-download of a LICENSED edition against a metered quota. Doing that
// blind, to guard against styles that might not exist, would have been an
// expensive way to change nothing.
//
// THE CENSUS THEN ANSWERED IT. Run over the whole canon on 9 September 2026,
// the NKJV feed sends exactly seven paragraph styles and six character styles:
//
//	paragraph  q2 21,543 · p 8,668 · q1 3,318 · s 2,822 · d 116 · qa 66 · pc 18
//	character  it 18,375 · sc 7,085 · wj 3,539 · qs 74 · bd 47 · sls 4
//
// Every one of them already has a considered answer, and no block of any style
// arrived empty. Nothing in the heading families the skip list would have
// gained — ms*, mr, sr, r, sp, cl, cd, s1-s5, sd, mt, the introduction and
// back-matter families — occurs at all. Neither does a single non-Scripture
// character style: no fig, no xt/xo, no rq, no va/vp. So both extensions would
// have been no-ops, and the right change was no change: no epoch, no
// re-download, and the census left in place as the standing guard for the day
// the feed does send one.
//
// Re-measure with BIBLETEXT_STYLE_INVENTORY=1 (see report) rather than trusting
// this list; it is a record of one run, not a property of the format.
//
// Nothing in this file is consulted by the decoder's routing. Every function
// here decides which COUNTER to increment and nothing else.

import (
	"os"
	"sort"
	"strings"
)

// apiBibleKnownParaStyles are the paragraph styles the decoder has a considered
// answer for: the ones it skips (apiBibleSkipPara), the superscription, and the
// prose and poetry families it deliberately walks into verse text.
//
// Wider than what the canon actually sends: the full-canon run found only q2,
// p, q1, s, d, qa and pc. The rest are here because they are ordinary USFM
// prose, poetry and list styles that this decoder would handle correctly if the
// feed began sending them, and a census that cried about them would be noise.
// A style NOT in this map is one nobody has thought about, which is the only
// thing worth a log line. Do not widen it to silence a report — re-measure with
// BIBLETEXT_STYLE_INVENTORY=1 and decide what the new style should do.
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
	// sls marks a passage in a SECONDARY LANGUAGE — the Aramaic of Daniel
	// 2:4b-7:28, and one span in Zechariah. This census found it on its first
	// full-canon run (Daniel ×3, Zechariah ×1), which is the check doing
	// exactly what it was built for. Walking it transparently is right: those
	// words ARE the text, whatever language the translators rendered them
	// from, and the app has nowhere to show the distinction.
	"sls": true,
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
//
// BIBLETEXT_STYLE_INVENTORY=1 asks for the whole list instead. That is how the
// question "which styles does the canon ACTUALLY send" gets answered — the
// unknown-only report cannot answer it, because a style being known says
// nothing about whether it occurs. Deciding what may be added to
// apiBibleSkipPara needs the occurrence list, not the unknown list.
func (c *apiBibleStyleCensus) report() {
	if c == nil {
		return
	}
	if os.Getenv("BIBLETEXT_STYLE_INVENTORY") == "1" {
		c.inventory()
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

// inventory logs every style seen, known or not, with the text split. Env-gated
// because it is a page of output per book and is only ever wanted deliberately.
func (c *apiBibleStyleCensus) inventory() {
	line := func(kind string, m map[string]int, suffix string) {
		if len(m) == 0 {
			return
		}
		var all []string
		for s, n := range m {
			all = append(all, formatCount(s, n))
		}
		sort.Strings(all)
		c.logf("bibletext: NKJV style inventory: %s: %s%s: %s",
			c.label, kind, suffix, strings.Join(all, ", "))
	}
	line("paragraph", c.paraWithText, " (with text)")
	line("paragraph", c.paraEmpty, " (no text)")
	line("character", c.charStyles, "")
}
