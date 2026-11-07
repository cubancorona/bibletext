package bibletext

// The cross-references panel's list, assembled from the three sources that
// can appear in it — the Gospel parallels (the synopsis's same-event rows,
// and in the next major release the same saying on another occasion after
// them), the publisher's own apparatus (publisher_xrefs.go: the NKJV's, and
// only in a build that shows it), and the Treasury of Scripture Knowledge —
// as consecutive runs of ONE card (crossRefRow), never interleaved: a
// licensed publisher's citations mixed into a list credited to OpenBible.info
// would misattribute both (docs/SOURCE_FIELDS_DECISIONS.md). Kept apart from
// the popup that mounts it so the composition can be tested without a window.
//
// Decided 6 November 2026: one list, one card. With the publisher's apparatus
// showing, each run stands under the app's one in-list heading, the sidebar's
// section label; the publisher's tagged citations are the same cards the
// Treasury's rows are, in the publisher's order, never re-sorted or capped;
// what the edition prints that cannot be a card stands under the note's cards
// as one caption, verbatim and unexplained; the Treasury stands OPEN under its
// own label, with the passages the publisher's cards already carry removed
// from it; and since the labels carry provenance, the parallel cards drop
// their pill. Without the apparatus — every other edition, and every store
// build until the licensing reply — the list is what it always was: the
// badged parallels, then the Treasury rows, no labels and no captions.
//
// Width, by construction: nothing in the list may want more width than the
// narrowest phone's body gives it (crossRefListWidth). Every child is one of
// five bounded kinds — a section label, a card (its reference measured, its
// preview wrapping), a wrapping caption, a spacer, a separator — and no
// Accordion, Button, Hyperlink or theme override stands among them: a widget
// whose title could not wrap widened the vertical scroll's content past the
// panel and clipped every row sideways, with no bar to reach the rest (the
// fold this list replaced). The panel wraps the list in squeezeWidthLayout as
// a belt, and TestCrossRefListFitsTheNarrowestPhone measures every object.

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Source credits appear with their displayed rows, one credit per line.
// The footer shows the edition's LicenseNotice once for a licensed edition.
const (
	// tskCredit names the Treasury in the builds without the publisher's run,
	// in the words they have always shown.
	tskCredit       = "Cross-references: OpenBible.info (CC-BY)"
	parallelsCredit = "Gospel parallels: synopsis"
	// treasuryCredit uses the section's full name in its source credit.
	treasuryCredit = "Treasury of Scripture Knowledge: OpenBible.info (CC-BY)"
)

// The section labels, set as sectionLabel sets them (uppercase, 11-pt bold
// monospace, muted). The publisher's names the SET, as its two neighbours
// do: a bare edition abbreviation beside verse text is the app's idiom for
// the previews' translation, which every card's preview already is.
const (
	parallelsLabel       = "GOSPEL PARALLELS"
	treasuryLabel        = "TREASURY OF SCRIPTURE KNOWLEDGE"
	publisherLabelSuffix = " REFERENCES" // after the edition's abbreviation: "NKJV REFERENCES"
	publisherKeySep      = " · "         // before a verse key: "NKJV REFERENCES · VERSE 2"
	publisherTitleKey    = "TITLE"       // the key of the Psalm title's notes
	publisherVerseKey    = "VERSE "      // before the number
)

// publisherEmptyLine is the one line under the publisher's label when the
// selection has no note of its own; the Treasury stands open beneath it. The
// label scopes the sentence: under a bare line the Treasury's cards would
// stand a few points below a sentence saying there are none. Hyphenated as
// the panel's title is.
const publisherEmptyLine = "No %s cross-references for this selection."

// What the panel says of a selection the cross-references cannot cover, in
// two lines: the first is the one every empty panel shows, the second says
// why. A selection they cover only in part shows its rows under the second
// line alone. Pinned word for word (crossref_coverage_test.go).
const (
	crossRefNoneLine         = "No cross-references for this selection."
	crossRefDeuterocanonLine = "The cross-references don't cover the deuterocanonical books."
	crossRefUncoveredLine    = "Nothing is listed for %s, which the cross-references don't cover."
)

// crossRefOfflineLine is the first sentence of the panel's connection error
// (crossRefOfflineMessage), and alone the caption under the Treasury's label
// when the Treasury failed to load while the rest of the list has content.
const (
	crossRefOfflineLine    = "Couldn't load cross-references."
	crossRefOfflineMessage = crossRefOfflineLine + "\nCheck your connection and try again."
)

// crossRefCoverage says what of a selection the Treasury cannot cover: note
// is the sentence that says so, "" when it covers all of it, and none
// reports that it covers none of it. A deuterocanonical book is not in the
// dataset at all. A verse a translation adds to a book that is has no number
// in the numbering the dataset is keyed by (crossRefSourceRef), so no row is
// filed under it, and every such verse is named by the one rule: the WEB
// Catholic's Greek Esther additions (4:18-47 and 10:4-14), its Song of the
// Three (Daniel 3:24-90), Susanna and Bel (Daniel 13 and 14), and the four
// verses the NKJV prints and the reference does not (Acts 8:37, 15:34 and
// 24:7, Luke 17:36). verses lie in one chapter of book, as a selection's do.
func crossRefCoverage(versionID, book string, verses []Verse) (note string, none bool) {
	if len(verses) == 0 {
		return "", false
	}
	if !protestantCanonBooks[book] {
		return crossRefDeuterocanonLine, true
	}
	var added []int
	for _, v := range verses {
		if _, _, ok := crossRefSourceRef(versionID, v); !ok {
			added = append(added, v.Verse)
		}
	}
	if len(added) == 0 {
		return "", false
	}
	return fmt.Sprintf(crossRefUncoveredLine, verseRunsPhrase(verses[0].Chapter, added)), len(added) == len(verses)
}

// panelCoverage is crossRefCoverage for the selection the panel is showing,
// in the translation being read. An edition whose own apparatus the panel
// shows (publisher_xrefs.go) is not asked: its run answers for the
// selection whatever the Treasury covers, and needs no network, so neither
// the note nor the answer given before the Treasury loads would be true of
// it.
func panelCoverage(state *AppState, book string, verses []Verse) (note string, none bool) {
	v := state.currentVersion()
	if v.PublisherCrossRefs {
		return "", false
	}
	return crossRefCoverage(v.ID, book, verses)
}

// verseRunsPhrase names ascending verses of one chapter as runs: "verse
// 4:20", "verses 4:18–47", "verses 4:18–20 and 4:25".
func verseRunsPhrase(chapter int, verses []int) string {
	var runs []string
	for i := 0; i < len(verses); {
		j := i
		for j+1 < len(verses) && verses[j+1] == verses[j]+1 {
			j++
		}
		run := fmt.Sprintf("%d:%d", chapter, verses[i])
		if j > i {
			run += fmt.Sprintf("–%d", verses[j])
		}
		runs = append(runs, run)
		i = j + 1
	}
	phrase := runs[len(runs)-1]
	if len(runs) > 1 {
		phrase = strings.Join(runs[:len(runs)-1], ", ") + " and " + phrase
	}
	if len(verses) == 1 {
		return "verse " + phrase
	}
	return "verses " + phrase
}

// crossRefList is the assembled list plus what it holds, for the panel and
// for the tests that pin the composition. An empty Objects is the panel's
// message state: nothing to list, or nothing loaded.
type crossRefList struct {
	Objects        []fyne.CanvasObject
	Parallels      int // same-occasion rows
	OtherOccasions int // the same saying on another occasion (next major release)
	PublisherCards int // the publisher's cards
	PublisherLines int // the publisher's caption lines: the notes' remainders, and the line that says it has no note
	TSKRows        int // the Treasury cards on screen, after those the publisher's cards already carry are dropped
	Sections       crossRefSections
	// CoverageNote is crossRefCoverage's sentence for the selection, "" when
	// the cross-references cover all of it. It heads the list when the list
	// has anything else in it, and is the empty panel's second line when not.
	CoverageNote string
}

// crossRefSections is which sets have rows on screen, for the footer's
// sources line: a set is named only while something of it is shown, so a
// synopsis is not credited on a Psalm, nor the Treasury when it failed to load
// or had nothing for the selection.
type crossRefSections struct {
	Parallels bool
	Publisher bool
	Treasury  bool
}

// buildCrossRefList composes the list for one selection. refs is
// crossRefsForSelection's answer (parallels first, then the Treasury rows);
// tskErr is the Treasury's load error when it has none, which a list with a
// publisher run reports in the Treasury's own place rather than instead of
// everything else. The same-occasion parallels lead and the other-occasion
// rows follow them, whatever order refs hands them over in.
func buildCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	out := composeCrossRefList(state, selected, refs, tskErr, pal, onTap)
	// The selected verses' own book: the selection's chapter, which the
	// reader may have left (selectedChapter).
	book, _ := selectedChapter(state, selected)
	out.CoverageNote, _ = panelCoverage(state, book, selected)
	if out.CoverageNote != "" && len(out.Objects) > 0 {
		out.Objects = append([]fyne.CanvasObject{crossRefCaption(out.CoverageNote)}, out.Objects...)
	}
	return out
}

// composeCrossRefList is the list without the coverage note.
func composeCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	var out crossRefList
	var parallels, others, tsk []crossRef
	for _, c := range refs {
		switch {
		case c.otherOccasion():
			others = append(others, c)
		case c.Parallel:
			parallels = append(parallels, c)
		default:
			tsk = append(tsk, c)
		}
	}
	out.Parallels, out.OtherOccasions = len(parallels), len(others)
	out.Sections.Parallels = len(parallels)+len(others) > 0

	v := state.currentVersion()
	if !v.PublisherCrossRefs {
		// The shipped list: the parallels wear their pill, since no label
		// says where they are from, then the Treasury's rows.
		for _, c := range parallels {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, true), pal, onTap))
		}
		for _, c := range others {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, true), pal, onTap))
		}
		for _, c := range tsk {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, false), pal, onTap))
		}
		out.TSKRows = len(tsk)
		out.Sections.Treasury = len(tsk) > 0
		return out
	}

	// The publisher's notes are the selected verses' chapter's, which the
	// reader may have left (selectedChapter); the title's notes come first
	// when the selection reaches the chapter's first verse.
	book, chapter := selectedChapter(state, selected)
	notes := publisherCrossRefsFor(state.Bible, book, chapter, selected)
	// Nothing at all to list is the panel's message state, not a run with
	// one line over an empty Treasury: with the Treasury unloaded, the
	// connection error; with it loaded and empty, the sentence every empty
	// panel shows. The publisher's empty line is not content.
	if len(notes) == 0 && len(parallels)+len(others) == 0 && len(tsk) == 0 {
		return out
	}

	labels := 0
	label := func(text string) {
		// Air before every label after the first — more above a label than
		// below it, so it opens its run rather than closing the one before.
		if labels > 0 {
			out.Objects = append(out.Objects, spacer(10))
		}
		labels++
		out.Objects = append(out.Objects, crossRefSectionLabel(text, pal))
	}

	if len(parallels)+len(others) > 0 {
		label(parallelsLabel)
		for _, c := range parallels {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, false), pal, onTap))
		}
		for _, c := range others {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, false), pal, onTap))
		}
	}

	// The publisher's run. Its label carries a verse key only where the key
	// informs: a selection of several verses, or a note whose key is not the
	// selected verse (the title's, keyed 0, when verse 1 is selected). Then
	// the label repeats per verse that has notes, and a verse without one is
	// silently absent, as in print.
	out.Sections.Publisher = true
	keyed := len(selected) > 1
	for _, n := range notes {
		if n.Verse != selected[0].Verse {
			keyed = true
		}
	}
	if len(notes) == 0 {
		label(publisherRunLabel(v.Abbrev, 0, false))
		out.Objects = append(out.Objects, crossRefNoteCaption(fmt.Sprintf(publisherEmptyLine, v.Abbrev)))
		out.PublisherLines++
	}
	var carded []crossRef     // the publisher's passages on screen, for the Treasury filter
	seen := map[string]bool{} // two notes citing one passage give one card, the first (publisherTarget.key)
	// A verse's label is held until the verse puts something on screen. A
	// note whose every citation an earlier note already carded leaves no
	// card and no words (split cuts the carded spans out whether or not a
	// card was made of them), and a verse of such notes is silently absent,
	// as a verse with no note is, rather than a label standing over the next
	// label or over the end of the list.
	pending := ""
	flush := func() {
		if pending != "" {
			label(pending)
			pending = ""
		}
	}
	for i, n := range notes {
		if i == 0 || (keyed && n.Verse != notes[i-1].Verse) {
			pending = publisherRunLabel(v.Abbrev, n.Verse, keyed)
		}
		targets, remainder := n.split()
		var cards []crossRefCard
		for _, tg := range targets {
			key := tg.key()
			if seen[key] {
				continue
			}
			seen[key] = true
			carded = append(carded, tg.Ref)
			cards = append(cards, crossRefCard{Ref: tg.Ref, Label: tg.label(), Tag: tg.tag(), Title: tg.Title})
		}
		if len(cards) == 0 {
			// A note with no card: its words stand alone, then a rule, so a
			// note ends in a rule whatever it holds.
			if remainder != "" {
				flush()
				out.Objects = append(out.Objects, container.NewVBox(crossRefNoteCaption(remainder), widget.NewSeparator()))
				out.PublisherLines++
			}
			continue
		}
		flush()
		if remainder != "" {
			cards[len(cards)-1].Caption = remainder
			out.PublisherLines++
		}
		for _, card := range cards {
			out.Objects = append(out.Objects, crossRefRow(state, card, pal, onTap))
			out.PublisherCards++
		}
	}

	// The Treasury, open under its label, less the passages the publisher's
	// cards already carry. Nothing for the selection: no section, no credit.
	// Failed to load while the list has content: the label and one line, no
	// verb — a retry here would put the whole-body spinner over rows that
	// need no network.
	kept := treasuryMinusPublisher(tsk, carded)
	switch {
	case tskErr != nil:
		label(treasuryLabel)
		out.Objects = append(out.Objects, crossRefNoteCaption(crossRefOfflineLine))
	case len(kept) > 0:
		label(treasuryLabel)
		for _, c := range kept {
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, false), pal, onTap))
		}
		out.TSKRows = len(kept)
		out.Sections.Treasury = true
	}
	return out
}

// publisherRunLabel is the publisher's section label: "NKJV REFERENCES", and
// with a key "NKJV REFERENCES · VERSE 2" or, for the title's notes,
// "NKJV REFERENCES · TITLE".
func publisherRunLabel(abbrev string, verse int, keyed bool) string {
	label := strings.ToUpper(abbrev) + publisherLabelSuffix
	switch {
	case !keyed:
		return label
	case verse == 0:
		return label + publisherKeySep + publisherTitleKey
	default:
		return label + publisherKeySep + publisherVerseKey + strconv.Itoa(verse)
	}
}

// treasuryMinusPublisher drops the Treasury rows whose passage a publisher's
// card already carries: a row the card's passage contains, or that contains
// the card's start verse — containment, not label equality, so "2 Samuel
// 15:1–18" yields to the title note's "2 Samuel 15:13–17" and "2 Corinthians
// 5:19–21" to its "2 Cor. 5:19". Removal is in the licence-safe direction:
// nothing is added to the publisher's run, and a passage appears once, under
// the source that stands above it. A row that runs on into another book is
// compared by its start verse alone, as contains reads one book.
func treasuryMinusPublisher(rows, cards []crossRef) []crossRef {
	var kept []crossRef
	for _, r := range rows {
		carried := false
		for _, c := range cards {
			if c.Book != r.Book {
				continue
			}
			if c.contains(r.Chapter, r.Verse) || (!r.crossBook() && r.contains(c.Chapter, c.Verse)) {
				carried = true
				break
			}
		}
		if !carried {
			kept = append(kept, r)
		}
	}
	return kept
}

// crossRefFooterCredit is the panel footer's credit: the sources whose rows
// stand on screen (sections), and — whenever the on-screen edition is licensed
// — that edition's own notice, because the preview under every card is its
// verse text, and in the builds that show its apparatus the apparatus is its
// too. The Treasury is named in the words of its label where the list has
// labels, and in the words every store build has always shown where not.
//
// Two credits stand one to a line. Joined on one line by " · ", the pair was
// wider than a phone's footer, and the toolkit's word wrap, which breaks at
// any space, broke it inside a credit: "… (CC-BY) · Gospel" over "parallels:
// synopsis" at 375 points, "synopsis" alone on the second row at 402. Each
// credit alone fits the narrowest footer, so the break now falls only
// between them, and the words are unchanged.
func crossRefFooterCredit(state *AppState, sections crossRefSections) (sources, notice string) {
	v := state.currentVersion()
	var parts []string
	if sections.Treasury {
		if v.PublisherCrossRefs {
			parts = append(parts, treasuryCredit)
		} else {
			parts = append(parts, tskCredit)
		}
	}
	if sections.Parallels {
		parts = append(parts, parallelsCredit)
	}
	sources = strings.Join(parts, "\n")
	if !v.PublicDomain && !v.isTesting() {
		notice = v.LicenseNotice
	}
	return sources, notice
}

// crossRefSectionLabel is the list's one heading idiom: the sidebar's section
// label, inset by the theme's padding — the card's own inset — so its ink
// stands level with the card references beneath it by construction.
func crossRefSectionLabel(text string, pal palette) fyne.CanvasObject {
	p := theme.Padding()
	return container.New(layout.NewCustomPaddedLayout(0, 0, p, p), sectionLabel(text, pal))
}

// crossRefCaptionStyle is the style of every caption in the panel: muted and
// caption-sized, so it reads as annotation rather than as a row.
var crossRefCaptionStyle = widget.RichTextStyle{
	ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText,
}

// crossRefCaption is a wrapped, muted, caption-sized line: a note's remainder,
// the coverage note, the footer's credits and the empty states.
func crossRefCaption(s string) *widget.RichText {
	rt := widget.NewRichText(&widget.TextSegment{Text: s, Style: crossRefCaptionStyle})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// crossRefNoteCaption is a note's remainder, or the publisher's empty line,
// set where a card's preview stands: inset by the card's padding, so its
// words share the preview's left edge, with the card's air beneath it.
func crossRefNoteCaption(s string) fyne.CanvasObject {
	p := theme.Padding()
	return container.New(layout.NewCustomPaddedLayout(0, p, p, p), crossRefCaption(s))
}
