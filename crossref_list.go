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
// note as printed and folds, its pieces under it; a parenthesised group —
// inside a note or as the whole of its note — is a row that starts closed and
// opens into its citations (crossref_group.go). This replaces the one
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
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
// monospace, muted). One label per source, never per verse, title or heading:
// the publisher's names the SET, as its two neighbours do (a bare edition
// abbreviation beside verse text is the app's idiom for the previews'
// translation, which every card's preview already is).
const (
	parallelsLabel       = "GOSPEL PARALLELS"
	treasuryLabel        = "TREASURY OF SCRIPTURE KNOWLEDGE"
	publisherLabelSuffix = " REFERENCES" // after the edition's abbreviation: "NKJV REFERENCES"
)

// publisherEmptyLine is the one line under the publisher's label when the
// selection has no note of its own; the Treasury stands open beneath it. The
// label scopes the sentence: under a bare line the Treasury's cards would
// stand a few points below a sentence saying there are none. Hyphenated as
// the panel's title is. publisherMissingLine replaces it where the edition has
// a caller whose note is missing at the end of a displaced stretch in the feed.
const (
	publisherEmptyLine   = "No %s cross-references for this selection."
	publisherMissingLine = "The %s has a cross-reference here that isn't in our source."
)

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
// shows (publisher_xrefs.go) answers from its run wherever it has a note for
// the selection, and needs no network; where it has none and the Treasury
// cannot cover the selection at all, the answer cannot change with the load,
// as in every other edition.
func panelCoverage(state *AppState, book string, verses []Verse) (note string, none bool) {
	v := state.currentVersion()
	if v.PublisherCrossRefs {
		_, chapter := selectedChapter(state, verses)
		if len(publisherCrossRefsFor(state.Bible, book, chapter, verses)) > 0 ||
			hasSilentDisplacedCaller(state.Bible, book, chapter, verses) {
			return "", false
		}
		if note, none := crossRefCoverage(v.ID, book, verses); none {
			return note, true
		}
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
	// when the panel opens: the notes' direct citations. PublisherGroups is
	// the parenthesised groups among the notes, inside a note or as the whole
	// of its note, each closed until tapped (crossref_group.go).
	// PublisherLines is the pieces at a note's own level that are shown and
	// not followed — a citation of the reader's own passage, words that are
	// no citation the loaded text has — and the line that says the edition
	// has no note for the selection. A lone group counts as a group and never
	// as a line, even one with nothing to follow, which is drawn as its words.
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

// crossRefTreasuryState describes the Treasury section's state when composing
// the cross-references list: settled (both loading false and err nil), loading
// (an inline caption and progress bar under the Treasury heading while the
// dataset downloads), or failed (err != nil, with an inline retry tap target
// when mounted in the live panel).
type crossRefTreasuryState struct {
	loading bool
	bar     *widget.ProgressBarInfinite
	err     error
	onRetry func()
}

// buildCrossRefList composes the list for one selection. refs is
// crossRefsForSelection's answer (parallels first, then the Treasury rows);
// tskErr is the Treasury's load error when it has none, which a list with a
// publisher run reports in the Treasury's own place rather than instead of
// everything else. The same-occasion parallels lead and the other-occasion
// rows follow them, whatever order refs hands them over in.
func buildCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	return buildCrossRefPanelList(state, selected, refs, crossRefTreasuryState{err: tskErr}, pal, onTap)
}

// buildCrossRefPanelList is buildCrossRefList with the live panel's Treasury
// loading and inline retry state.
func buildCrossRefPanelList(state *AppState, selected []Verse, refs []crossRef, tskState crossRefTreasuryState, pal palette, onTap func(crossRef)) crossRefList {
	out := composeCrossRefList(state, selected, refs, tskState, pal, onTap)
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
func composeCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskState crossRefTreasuryState, pal palette, onTap func(crossRef)) crossRefList {
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
	missing := len(notes) == 0 && hasSilentDisplacedCaller(state.Bible, book, chapter, selected)
	// Nothing at all to list is the panel's message state, not a run with
	// one line over an empty Treasury: with the Treasury unloaded, the
	// connection error; with it loaded and empty, the sentence every empty
	// panel shows. The publisher's empty line is not content; a missing
	// displaced note is.
	if len(notes) == 0 && !missing && len(parallels)+len(others) == 0 && len(tsk) == 0 {
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

	// The publisher's run stands directly under the lettered passage, before
	// the Gospel parallels, under one section label for the source: sections
	// are for the sources only, never for verses, a Psalm's title or a
	// heading. Each note is a block of its own under its letter, in the order
	// the notes stand in the selection (the title's, a heading's, then the
	// verses'), and holds everything the note prints in the order it prints
	// it (crossref_note.go); no card is withheld because an earlier note has
	// its passage — the note is the edition's unit and is kept whole.
	out.Sections.Publisher = true
	label(publisherRunLabel(v.Abbrev))
	if len(notes) == 0 {
		line := publisherEmptyLine
		if missing {
			line = publisherMissingLine
		}
		out.Objects = append(out.Objects, crossRefNoteCaption(fmt.Sprintf(line, v.Abbrev)))
		out.PublisherLines++
	} else {
		// A little air between the label and the first note's ground.
		out.Objects = append(out.Objects, spacer(4))
	}
	var carded []crossRef // the passages the publisher's cards on screen follow, for the Treasury filter
	for _, n := range notes {
		row, cards, groups, lines := publisherNoteBlock(state, n, pal, onTap, out.Relayout)
		out.Objects = append(out.Objects, row)
		carded = append(carded, cards...)
		out.PublisherNotes++
		out.PublisherCards += len(cards)
		out.PublisherGroups += groups
		out.PublisherLines += lines
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

	// The Treasury, open under its label, less the passages the publisher's
	// cards already carry. While the dataset downloads, its label holds an
	// inline loading caption and progress bar while the local sections stand
	// open above it; when a load fails with local content on screen, its label
	// holds the offline line (and an inline retry tap target in the live
	// panel, which reloads the Treasury section in place). Nothing for the
	// selection once loaded: no section, no credit. A row that lies wholly
	// inside the selection is muted with no tap, as in the publisher's run.
	kept := treasuryMinusPublisher(tsk, carded)
	switch {
	case tskState.loading:
		label(treasuryLabel)
		out.Objects = append(out.Objects, crossRefTreasuryLoading(tskState.bar))
	case tskState.err != nil:
		label(treasuryLabel)
		out.Objects = append(out.Objects, crossRefNoteCaption(crossRefOfflineLine))
		if tskState.onRetry != nil {
			out.Objects = append(out.Objects, crossRefTreasuryRetry(pal, tskState.onRetry))
		}
	case len(kept) > 0:
		label(treasuryLabel)
		for _, c := range kept {
			if isWithinSelection(c, book, chapter, selected) {
				out.Objects = append(out.Objects, crossRefSelfRow(c.label(), "", pal, true))
				continue
			}
			out.Objects = append(out.Objects, crossRefRow(state, passageCard(c, false), pal, onTap))
		}
		out.TSKRows = len(kept)
		out.Sections.Treasury = true
	}
	return out
}

// publisherRunLabel is the publisher's section label ("NKJV REFERENCES"): one
// over every note of the selection, naming the source as its neighbours do.
func publisherRunLabel(abbrev string) string {
	return strings.ToUpper(abbrev) + publisherLabelSuffix
}

// treasuryMinusPublisher drops the Treasury rows whose passage a publisher's
// card already carries: for an ordinary verse or range card, any row whose
// passage overlaps the card's — containment, not label equality, so "2 Samuel
// 15:1–18" yields to the title note's "2 Samuel 15:13–17", "2 Corinthians
// 5:19–21" to its "2 Cor. 5:19", and a cross-book range such as "2 Chronicles
// 36:22–Ezra 1:3" to a card in either book it spans ("Ezra 1:1–4" or "Ezra
// 1:1"). A citation of a chapter's superscription (TitleRef) previews the
// title rather than verse 1 and never drops a Treasury verse row; a citation
// of whole chapters (WholeChapters) previews only verse 1 under a chapter
// heading and drops a Treasury row only when the row spans those exact whole
// chapters, keeping specific verse and sub-range Treasury rows inside the
// chapter. Removal is in the licence-safe direction: nothing is added to the
// publisher's run, and a passage appears once, under the source that stands
// above it.
func treasuryMinusPublisher(rows, cards []crossRef) []crossRef {
	var kept []crossRef
	for _, r := range rows {
		carried := false
		for _, c := range cards {
			if c.TitleRef {
				continue
			}
			if c.WholeChapters {
				if !r.crossBook() && c.Book == r.Book {
					rs, re := r.bounds()
					cs, ce := c.bounds()
					if rs == cs && re == ce {
						carried = true
						break
					}
				}
				continue
			}
			if c.containsVerse(r.Book, r.Chapter, r.Verse) || r.containsVerse(c.Book, c.Chapter, c.Verse) {
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

// crossRefTreasuryLoading draws the inline waiting state under the Treasury
// heading while the dataset loads behind already-rendered local sections.
func crossRefTreasuryLoading(bar *widget.ProgressBarInfinite) fyne.CanvasObject {
	if bar == nil {
		bar = widget.NewProgressBarInfinite()
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(2),
		crossRefNoteCaption("Finding related passages…"),
		container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), bar),
	)
}

// crossRefTreasuryRetry draws the inline retry action under the Treasury
// heading as a bounded tapCard rather than a widget.Button (bannedIn).
func crossRefTreasuryRetry(pal palette, onRetry func()) fyne.CanvasObject {
	lbl := canvas.NewText("Try again", pal.Accent)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.TextSize = crossRefPanelSizes().printed
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = pal.Border
	border.StrokeWidth = 1
	border.CornerRadius = 8
	pill := container.NewStack(
		border,
		container.New(layout.NewCustomPaddedLayout(0, 0, 14, 14),
			container.NewVBox(layout.NewSpacer(), lbl, layout.NewSpacer())),
	)
	return container.New(layout.NewCustomPaddedLayout(2, 6, 0, 0),
		container.NewHBox(newTapCard(pill, crossRefHover(pal), onRetry)))
}
