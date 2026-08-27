package bibletext

// The translators' footnotes, rendered — when the reader turns them on — as a
// separated section at the chapter's bottom, in the manner of a Supreme Court
// slip opinion: a short flush-left rule, then the chapter's notes in smaller
// muted type, each keyed by its verse number. No markers are added to the
// Scripture text itself; the page above the rule is byte-identical with the
// section on or off, and every derived surface (copy, share, TTS, search,
// links, AI) reads Verse.Text, which the section never touches (bible.go).
//
// The section renders at 0.85em — a size that exists nowhere else in the
// chapter storage (body text is 1.0em, verse superscripts 0.66em). That is a
// load-bearing constant twice over on the Apple panes:
//
//   - the native verse-number scans capture only runs BELOW 0.8× the largest
//     font (btIOSBuildVerseIndex, btMacVerseSpanForRange), so at 0.85em the
//     section's digit-leading verse keys can never be indexed as phantom
//     verses at any reader text size (em sizes scale with the body);
//   - the native content-end detectors (btIOSFindContentEnd,
//     btMacFindContentEnd) find the apparatus boundary as the FIRST run in the
//     [0.8, 0.95) band of the largest font, which bounds the last verse's
//     read-along/highlight ranges and the selection verbs.
//
// Never restyle the section below 0.8em or up to body size without revisiting
// both. TestFootnoteSectionNativeContract pins the pact from this side.

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
)

const prefFootnotes = "reading.footnotes"

// footnoteSeparator is the short flush-left rule that opens the section: an
// underline drawn over a run of no-break spaces. The underline is drawn by
// the TEXT SYSTEM, not by glyphs, so it is a continuous hairline by
// construction on both Apple importers — every glyph construction measured
// (em dashes, U+2015, U+2500) joins on macOS but gaps on iOS, per-class
// letter-spacing is ignored, <s> is dropped, and the importer also drops
// borders, <hr> styling and margin-top, so a styled div can't draw it either.
var footnoteSeparator = "<u>" + strings.Repeat("\u00a0", 16) + "</u>"

// footnoteSectionSupported reports whether THIS platform's reading pane
// renders the bottom section: the Apple native panes (one shared HTML
// builder), Android's TextView pipeline, and the Windows/Linux styled pane
// (geometry-only, reading_styled_footnotes.go). The Fyne fallback panes are
// documented gaps unreachable in shipping builds. The Settings card hides
// where this is false rather than offering a switch that does nothing.
func footnoteSectionSupported() bool {
	switch runtime.GOOS {
	case "darwin", "ios", "android":
		return true
	case "windows", "linux":
		return styledPaneEnabledOnPlatform
	}
	return false
}

// footnotesEnabled reports the persisted toggle — OFF by default: the
// apparatus is an opt-in, the opposite default from red letter, because a
// footnote section is added matter beneath the text rather than colour on the
// words themselves (docs/FOOTNOTES.md §3). nil-safe for tests.
func footnotesEnabled() bool {
	if app := fyne.CurrentApp(); app != nil {
		return app.Preferences().BoolWithFallback(prefFootnotes, false)
	}
	return false
}

func setFootnotesEnabled(v bool) {
	if app := fyne.CurrentApp(); app != nil {
		app.Preferences().SetBool(prefFootnotes, v)
	}
}

// footnoteEntry is one row of the bottom section: a verse number key and the
// translators' note that belongs to it.
type footnoteEntry struct {
	Verse int
	Text  string
}

// chapterFootnoteEntries collects the chapter's translator footnotes in verse
// order — the notes riding on rendered verses PLUS the orphans whose verse
// the translation omits (Luke 17:36 and kin), merged by verse number so an
// omitted verse's note sorts into its natural place between its neighbours.
// Cross-references (Kind == footnoteKindCrossref — the NKJV feed's entire
// apparatus) are excluded: whether they ever display is an open decision
// (docs/FOOTNOTES.md §8), and a wall of "John 7:50; 19:39" rows is a
// different feature from wording-and-manuscript notes.
func chapterFootnoteEntries(verses []Verse, orphans []OrphanFootnote) []footnoteEntry {
	var entries []footnoteEntry
	for _, v := range verses {
		for _, fn := range v.Footnotes {
			if fn.Kind != "" {
				continue
			}
			text := strings.TrimSpace(fn.Text)
			if text == "" {
				continue
			}
			entries = append(entries, footnoteEntry{Verse: v.Verse, Text: text})
		}
	}
	for _, o := range orphans {
		if o.Kind != "" {
			continue
		}
		text := strings.TrimSpace(o.Text)
		if text == "" {
			continue
		}
		entries = append(entries, footnoteEntry{Verse: o.Verse, Text: text})
	}
	// Stable: keeps a verse's own notes in marker order, and an orphan's
	// verse number cannot collide with a rendered verse's (its verse has no
	// text by definition).
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Verse < entries[j].Verse })
	return entries
}

// chapterHasFootnotes reports whether the current chapter has anything the
// section would show — the gate for the header toggle, mirroring
// chapterAudioAvailable: no chapter gets a dead control. Orphans count: a
// chapter whose ONLY note explains an omitted verse still has a section.
func chapterHasFootnotes(state *AppState) bool {
	if state == nil || state.Bible == nil {
		return false
	}
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
		for _, fn := range v.Footnotes {
			if fn.Kind == "" && strings.TrimSpace(fn.Text) != "" {
				return true
			}
		}
	}
	for _, o := range state.Bible.OrphanNotesFor(state.CurrentBook, state.CurrentChapter) {
		if o.Kind == "" && strings.TrimSpace(o.Text) != "" {
			return true
		}
	}
	return false
}

// The feature's ONE control is the TRANSLATORS' FOOTNOTES card at the top of
// Settings (ai_settings.go) — a deliberate design decision: no toggle in the
// reading-pane chrome for now. A header toggle existed briefly and was
// removed; if it returns, chapterHasFootnotes above is its availability gate
// (the audio-control convention — absent, not disabled), iconFootnote
// (icons_embed.go) is its reserved glyph, and the mobile header must not
// widen its right column (the expanded audio card's reserved centre
// footprint overlaps it on 375pt phones — stack under full-screen in boxH
// cells instead).

// writeFootnoteCSS emits the section's stylesheet rules — only called when
// the section is actually rendered, so a footnotes-off chapter's HTML is
// byte-identical to one built before this feature existed.
//
// 0.85em everywhere: see the file comment — the native scans and the
// content-end detector both key on this size band. line-height tightens
// against the body's airy leading (SCOTUS sets its notes tighter too);
// entries justify like the body prose above them.
func writeFootnoteCSS(b *strings.Builder, mutedHex string) {
	fmt.Fprintf(b, `p.fnsep {
		color: %s;
		font-size: 0.85em;
		line-height: 1.4;
		text-align: left;
		margin: 0 0 8px 0;
	}`, mutedHex)
	fmt.Fprintf(b, `p.fn {
		color: %s;
		font-size: 0.85em;
		line-height: 1.4;
		text-align: justify;
		hyphens: auto;
		-webkit-hyphens: auto;
		margin: 0 0 5px 0;
	}`, mutedHex)
	b.WriteString(`.fnv { font-weight: 600; }`)
}

// writeFootnoteSection appends the separator rule and the entries after the
// last verse paragraph. In reporter layout paragraphs carry no bottom margin
// and the importer zeroes every margin-top, so the air above the rule is a
// blank line inside the separator paragraph itself.
func writeFootnoteSection(b *strings.Builder, entries []footnoteEntry, reporter bool) {
	sep := footnoteSeparator
	if reporter {
		sep = "<br>" + sep
	}
	fmt.Fprintf(b, `<p class="fnsep">%s</p>`, sep)
	for _, e := range entries {
		fmt.Fprintf(b, `<p class="fn"><span class="fnv">%d</span>&nbsp;%s</p>`, e.Verse, htmlEscape(e.Text))
	}
}
