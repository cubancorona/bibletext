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
// section label; the publisher's citations are the same cards the Treasury's
// rows are, in the publisher's order, never re-sorted or capped; the Treasury
// stands OPEN under its own label, with the passages the publisher's cards
// already carry removed from it; and since the labels carry provenance, the
// parallel cards drop their pill. Without the apparatus — every other
// edition, and every store build until the licensing reply — the list is
// what it always was: the badged parallels, then the Treasury rows, no
// labels and no captions.
//
// Decided 11 November 2026: nothing the edition's notes carry is lost. The
// selected text heads the list, with a letter on each word a note hangs from
// (crossref_passage.go), and the publisher's run is its notes one by one
// under those letters, each whole and in the order it is printed
// (crossref_note.go): a note of several pieces is a row that carries the
// note as printed and folds, its pieces under it, and so is a note that is
// one parenthesised group; a parenthesised group inside a note is a row
// that opens into its citations (crossref_group.go). This replaces the one
// caption that gathered, under a note's last card, whatever of the note was
// not a card, and the rule that two notes citing one passage gave one card:
// both were the list's reading of a note, and the note is the edition's.
// Each note stands on a ground of its own, its letter alone in a margin and
// the rest of it beside the margin (crossref_section.go).
//
// Width, by construction: nothing in the list may want more width than the
// narrowest phone's body gives it (crossRefListChrome; narrowestPhoneBody in
// crossref_width_test.go). Every child is one of
// a few bounded kinds — the passage, which wraps to the width it is given; a
// section label; a card (its reference measured, its preview wrapping); a
// note's or a group's row (a printed line that wraps beside an icon of fixed
// size); a reference that is shown and not followed; a wrapping caption; a
// spacer; a separator — and no Accordion, Button, Hyperlink or per-row theme
// override stands among them. A note's rows are these same kinds, on the
// note's padded ground beside the letter's margin. A
// single theme scope wraps the complete list in the panel. A widget
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
	// PublisherNotes is the publisher's notes on screen, each under its
	// letter (crossref_note.go). PublisherCards is the cards that stand open
	// when the panel opens: the notes' direct citations, and the citations
	// of a note that is one parenthesised group. PublisherGroups is the
	// parenthesised groups among the notes' pieces; one inside a note is a
	// row that opens into its citations (crossref_group.go). PublisherLines
	// is the pieces on screen as the panel opens that are shown and not
	// followed — a citation of the reader's own passage, words that are no
	// citation the loaded text has, at a note's own level or in a note that
	// is one group — and the line that says the edition has no note for the
	// selection.
	PublisherNotes  int
	PublisherCards  int
	PublisherLines  int
	PublisherGroups int
	TSKRows         int // the Treasury cards on screen, after those the publisher's cards already carry are dropped
	Sections        crossRefSections
	// Passage reports that the selected text heads the list, with the
	// publisher's notes' letters on their words (crossref_passage.go).
	Passage bool
	// Relayout is how a row that changes its height asks for the list to be
	// laid out again; the panel sets its fn once the list is mounted.
	Relayout *crossRefRelayout
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
		out.Objects = append([]fyne.CanvasObject{crossRefNoteCaption(out.CoverageNote)}, out.Objects...)
	}
	return out
}

// composeCrossRefList is the list without the coverage note.
func composeCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	out := crossRefList{Relayout: &crossRefRelayout{}}
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

	// The selected text first, with the notes' letters on their words: the
	// notes below are read by those letters (crossref_passage.go). Only where
	// the edition has a note for the selection; a selection without one has
	// no letter to show.
	if len(notes) > 0 {
		out.Objects = append(out.Objects, newCrossRefPassage(state, book, chapter, selected, notes, pal), spacer(4))
		out.Passage = true
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
	// Each note is a block of its own under its letter, in the order the
	// notes stand in the verse, and holds everything the note prints in the
	// order it prints it (crossref_note.go). A verse's label stands over its
	// first note: every note puts something on screen, so no label is held
	// back, and a card is not withheld because an earlier note of the verse
	// has its passage — the note is the edition's unit and is kept whole.
	var carded []crossRef // the passages the publisher's cards on screen follow, for the Treasury filter
	for i, n := range notes {
		if i == 0 || (keyed && n.Verse != notes[i-1].Verse) {
			label(publisherRunLabel(v.Abbrev, n.Verse, keyed))
			// A little air between the label and the first note's ground.
			out.Objects = append(out.Objects, spacer(4))
		}
		row, cards, groups, lines := publisherNoteBlock(state, n, pal, onTap, out.Relayout)
		out.Objects = append(out.Objects, row)
		carded = append(carded, cards...)
		out.PublisherNotes++
		out.PublisherCards += len(cards)
		out.PublisherGroups += groups
		out.PublisherLines += lines
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

// crossRefSectionLabel aligns the section heading with the references of
// the cards that stand at the list's edge, the Treasury's and the
// parallels'. The list supplies the larger gap above each new section.
func crossRefSectionLabel(text string, pal palette) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(0, 4, 0, 0), sectionLabel(text, pal))
}

// crossRefCaptionStyle is the style of every caption in the panel: muted and
// caption-sized, so it reads as annotation rather than as a row.
var crossRefCaptionStyle = widget.RichTextStyle{
	ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText,
}

// crossRefCaption is a wrapped, muted, caption-sized line: the coverage note,
// the footer's credits and the empty states.
func crossRefCaption(s string) *widget.RichText {
	rt := widget.NewRichText(&widget.TextSegment{Text: s, Style: crossRefCaptionStyle})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// crossRefNoteCaption aligns an explanatory caption with the previews and
// separates it from what follows.
func crossRefNoteCaption(s string) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(4, 6, 0, 0), crossRefCaption(s))
}
