package bibletext

// The cross-references panel keeps publisher notes, Gospel episodes and
// Treasury origins in separate source sections. Publisher notes preserve
// their exact pieces and word anchors; other sources use their own verse,
// heading or event identities. All sections share cards and bounded folds.

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
		if v.PublisherApparatus {
			if note, none := crossRefCoverage(v.ID, book, verses); none {
				return note, true
			}
			return "", false
		}
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
	TSKRows         int // Treasury relationships, including cards in closed verse groups
	Sections        crossRefSections
	// Passage reports that the selected text heads the list, with the
	// publisher's letters only where exact word anchors exist (crossref_passage.go).
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
	book, chapter := selectedChapter(state, selected)
	var notes []publisherCrossRef
	var missing bool
	if v.PublisherCrossRefs {
		// The publisher's notes are the selected verses' chapter's, which the
		// reader may have left (selectedChapter); the title's notes come first
		// when the selection reaches the chapter's first verse.
		book, chapter = selectedChapter(state, selected)
		notes = publisherCrossRefsFor(state.Bible, book, chapter, selected)
		missing = len(notes) == 0 && hasSilentDisplacedCaller(state.Bible, book, chapter, selected)
	}

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

	// Keep the selected passage above every source. Only exact publisher
	// word anchors receive letters; heading-only notes are identified below
	// by their heading text and before-verse position.
	headingOnly := v.ID == "bsb" && len(notes) > 0
	for _, n := range notes {
		headingOnly = headingOnly && n.Heading && n.HeadingText != ""
	}
	passageNotes := notes
	if headingOnly {
		passageNotes = nil
	}
	if len(selected) > 0 {
		out.Objects = append(out.Objects, newCrossRefPassage(state, book, chapter, selected, passageNotes, pal), spacer(4))
		out.Passage = true
	}

	if len(notes) > 0 || missing {
		out.Sections.Publisher = true
		label(publisherRunLabel(v.Abbrev))
		if missing {
			out.Objects = append(out.Objects, crossRefNoteCaption(fmt.Sprintf(publisherMissingLine, v.Abbrev)))
			out.PublisherLines++
		} else {
			out.Objects = append(out.Objects, spacer(4))
		}
	}
	for _, n := range notes {
		if headingOnly {
			row, cards, groups, lines := crossRefHeadingNoteBlock(state, n, pal, onTap, out.Relayout)
			out.Objects = append(out.Objects, row)
			out.PublisherCards += cards
			out.PublisherGroups += groups
			out.PublisherLines += lines
		} else {
			row, cards, groups, lines := publisherNoteBlock(state, n, pal, onTap, out.Relayout)
			out.Objects = append(out.Objects, row)
			out.PublisherCards += len(cards)
			out.PublisherGroups += groups
			out.PublisherLines += lines
		}
		out.PublisherNotes++
	}

	if len(parallels)+len(others) > 0 {
		label(parallelsLabel)
		for _, set := range crossRefGospelSets(append(parallels, others...)) {
			out.Objects = append(out.Objects, crossRefGospelBlock(state, set, selected, pal, onTap, out.Relayout))
		}
	}

	// An unavailable Treasury never replaces local notes or episodes. Its
	// credit appears only when references are present, including closed groups.
	_, uncovered := crossRefCoverage(v.ID, book, selected)
	switch {
	case uncovered && len(tsk) == 0:
	case tskState.loading:
		label(treasuryLabel)
		out.Objects = append(out.Objects, crossRefTreasuryLoading(tskState.bar))
	case tskState.err != nil:
		label(treasuryLabel)
		out.Objects = append(out.Objects, crossRefNoteCaption(crossRefOfflineLine))
		if tskState.onRetry != nil {
			out.Objects = append(out.Objects, crossRefTreasuryRetry(pal, tskState.onRetry))
		}
	case len(tsk) > 0:
		label(treasuryLabel)
		out.TSKRows = len(tsk)
		out.Sections.Treasury = true
		if len(selected) <= 1 {
			out.Objects = append(out.Objects, crossRefSourceCards(state, tsk, selected, pal, onTap)...)
			break
		}
		sets, unkeyed := crossRefTreasurySets(tsk)
		rows := make([]*crossRefSourceGroup, 0, len(sets))
		var margin float32
		for _, set := range sets {
			build := func() []fyne.CanvasObject { return crossRefSourceCards(state, set.refs, selected, pal, onTap) }
			row := newCrossRefSourceGroup(set.id, fmt.Sprint(set.verse), "", "",
				crossRefSourceSummary(set.refs), len(sets) == 1, pal, out.Relayout, build)
			margin = max(margin, row.margin)
			rows = append(rows, row)
		}
		// Align all origins in a section, including a range crossing 99–100.
		for _, row := range rows {
			row.margin = margin
			out.Objects = append(out.Objects, crossRefSection(row, pal))
		}
		// Older in-memory callers may not supply origins. Keep those rows
		// visible without inventing a verse key for them.
		out.Objects = append(out.Objects, crossRefSourceCards(state, unkeyed, selected, pal, onTap)...)
	}

	return out
}

// publisherRunLabel is the publisher's section label ("NKJV REFERENCES"): one
// over every note of the selection, naming the source as its neighbours do.
func publisherRunLabel(abbrev string) string {
	return strings.ToUpper(abbrev) + publisherLabelSuffix
}

// crossRefFooterCredit is the panel footer's credit: the sources whose rows
// stand on screen (sections), and — whenever the on-screen edition is licensed
// — that edition's own notice, because the preview under every card is its
// verse text, and in the builds that show its apparatus the apparatus is its
// too. Each displayed source keeps its own credit even when another source
// points to the same destination.
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
		parts = append(parts, treasuryCredit)
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
