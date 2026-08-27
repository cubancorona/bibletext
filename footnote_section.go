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
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
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
// renders the bottom section. Prototype scope: the Apple native panes (one
// shared HTML builder). Android's TextView pipeline and the Windows/Linux
// styled pane have documented recipes (docs/FOOTNOTES.md) but no rendering
// yet, so the toggle and settings row hide there rather than offering a
// switch that visibly does nothing.
func footnoteSectionSupported() bool {
	switch runtime.GOOS {
	case "darwin", "ios":
		return true
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
// order. Cross-references (Kind == footnoteKindCrossref — the NKJV feed's
// entire apparatus) are excluded: whether they ever display is an open
// decision (docs/FOOTNOTES.md §8), and a wall of "John 7:50; 19:39" rows is a
// different feature from wording-and-manuscript notes.
func chapterFootnoteEntries(verses []Verse) []footnoteEntry {
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
	return entries
}

// chapterHasFootnotes reports whether the current chapter has anything the
// section would show — the gate for the header toggle, mirroring
// chapterAudioAvailable: no chapter gets a dead control.
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
	return false
}

// footnotesToggleButton is the reading-header toggle: nil when this platform
// doesn't render the section or the chapter has no notes (the audio-control
// convention — absent, not disabled). Importance is the on/off state, the
// same active-fill convention as the Search tab's notes bubble; the tap's
// refreshReadingOnly rebuilds the header, which re-reads the state.
func footnotesToggleButton(state *AppState) fyne.CanvasObject {
	if !footnoteSectionSupported() || !chapterHasFootnotes(state) {
		return nil
	}
	btn := widget.NewButtonWithIcon("", iconFootnote, func() {
		setFootnotesEnabled(!footnotesEnabled())
		state.refreshReadingOnly()
	})
	btn.Importance = widget.LowImportance
	if footnotesEnabled() {
		btn.Importance = widget.HighImportance
	}
	return btn
}

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
