package bibletext

// The cross-references panel's list, assembled from the three sources that
// can appear in it — the Gospel parallels (the synopsis's same-event rows,
// and in the next major release the same saying on another occasion after
// them), the publisher's own apparatus (publisher_xrefs.go: the NKJV's, and
// only in a build that shows it), and the Treasury of Scripture Knowledge —
// each under its own heading and its own credit, never interleaved: a
// licensed publisher's citations mixed into a list credited to OpenBible.info
// would misattribute both (docs/SOURCE_FIELDS_DECISIONS.md). Kept apart from
// the popup that mounts it so the composition can be tested without a window.
//
// With the publisher's apparatus showing, its block is the panel's primary
// content and the Treasury moves into a disclosure beneath it: closed when
// the publisher has something to say about the selection, open when it has
// nothing, so a reader is never shown an empty edition block and no way on.
// Without it — every other edition, and every store build until the licensing
// reply — the list is what it always was: parallels, then the Treasury rows.

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The Treasury's credit, and the synopsis's. The publisher's block credits
// itself with the edition's own LicenseNotice, so no credit line here ever
// has to know which edition is on screen.
const (
	tskCredit       = "Cross-references: OpenBible.info (CC-BY)"
	parallelsCredit = "Gospel parallels: synopsis"
)

// What the panel says of a selection the cross-references cannot cover, in
// two lines: the first is the one every empty panel shows, the second says
// why. A selection they cover only in part shows its rows under the second
// line alone. Pinned word for word (crossref_coverage_next_test.go). The
// second line is the next major release's (nextRelease, docs/NEXT.md): the
// shipping build shows the first alone, as 1.2.19 does.
const (
	crossRefNoneLine         = "No cross-references for this selection."
	crossRefDeuterocanonLine = "The cross-references don't cover the deuterocanonical books."
	crossRefUncoveredLine    = "Nothing is listed for %s, which the cross-references don't cover."
)

// crossRefNotedAdditions are the books whose added verses crossRefCoverage
// names: verses a translation prints that the reference, whose numbering
// the Treasury is keyed by, does not have, so no row is filed under them.
// Esther's are the WEB Catholic's Greek additions, 4:18-47 and 10:4-14. The
// Song of the Three (Daniel 3:24-90), Susanna and Bel (Daniel 13 and 14) in
// the WEB Catholic, and the four verses the NKJV prints and the reference
// does not (Acts 8:37, 15:34 and 24:7, Luke 17:36), have no rows for the
// same reason and show the first line alone.
var crossRefNotedAdditions = map[string]bool{"Esther": true}

// crossRefCoverage says what of a selection the Treasury cannot cover: note
// is the sentence that says so, "" when it covers all of it, and none
// reports that it covers none of it. A deuterocanonical book is not in the
// dataset at all; a verse a translation adds to a book that is has no number
// in the numbering the dataset is keyed by (crossRefSourceRef). verses lie in
// one chapter of book, as a selection's do.
//
// The panel asks it only in the next major release (nextRelease,
// docs/NEXT.md), where the Greek Esther maps verse for verse and its
// additions are the verses it adds. The shipping build's panel says what
// 1.2.19's does: the first line alone, after the Treasury has loaded.
func crossRefCoverage(versionID, book string, verses []Verse) (note string, none bool) {
	if len(verses) == 0 {
		return "", false
	}
	if !protestantCanonBooks[book] {
		return crossRefDeuterocanonLine, true
	}
	if !crossRefNotedAdditions[book] {
		return "", false
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
// for the tests that pin the composition.
type crossRefList struct {
	Objects        []fyne.CanvasObject
	Parallels      int // same-occasion rows
	OtherOccasions int // the same saying on another occasion (next major release)
	PublisherRows  int
	TSKRows        int
	PublisherBlock bool              // the edition's block was rendered (rows or its empty state)
	Disclosure     *widget.Accordion // the Treasury's disclosure; nil when its rows stand in the open
	// CoverageNote is crossRefCoverage's sentence for the selection, "" when
	// the cross-references cover all of it. It heads the list when the list
	// has anything else in it, and is the empty panel's second line when not.
	CoverageNote string
}

// buildCrossRefList composes the list for one selection. refs is
// crossRefsForSelection's answer (parallels first, then the Treasury rows);
// tskErr is the Treasury's load error when it has none, which a list with a
// publisher block reports in the Treasury's own place rather than instead of
// everything else. The same-occasion parallels lead and the other-occasion
// rows follow them, whatever order refs hands them over in.
func buildCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	out := composeCrossRefList(state, selected, refs, tskErr, pal, onTap)
	if !nextRelease {
		return out
	}
	book, _ := readerChapter(state)
	out.CoverageNote, _ = crossRefCoverage(state.currentVersion().ID, book, selected)
	if out.CoverageNote != "" && len(out.Objects) > 0 {
		out.Objects = append([]fyne.CanvasObject{crossRefCaption(out.CoverageNote)}, out.Objects...)
	}
	return out
}

// composeCrossRefList is the list without the coverage note.
func composeCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	var out crossRefList
	var others, tsk []crossRef
	for _, c := range refs {
		switch {
		case c.otherOccasion():
			others = append(others, c)
		case c.Parallel:
			out.Parallels++
			out.Objects = append(out.Objects, crossRefRow(state, c, pal, onTap))
		default:
			tsk = append(tsk, c)
		}
	}
	out.OtherOccasions = len(others)
	for _, c := range others {
		out.Objects = append(out.Objects, crossRefRow(state, c, pal, onTap))
	}
	out.TSKRows = len(tsk)

	v := state.currentVersion()
	if !v.PublisherCrossRefs {
		for _, c := range tsk {
			out.Objects = append(out.Objects, crossRefRow(state, c, pal, onTap))
		}
		return out
	}

	// The edition's own block: heading, the notes verbatim, the legend the
	// parenthesised citations need, and the edition's own credit.
	out.PublisherBlock = true
	rows := publisherCrossRefsFor(state.Bible, state.CurrentBook, state.CurrentChapter, selected)
	out.PublisherRows = len(rows)
	out.Objects = append(out.Objects, crossRefBlockHeading(fmt.Sprintf("%s cross references", v.Abbrev), pal))
	if len(rows) == 0 {
		out.Objects = append(out.Objects, crossRefCaption(fmt.Sprintf("The %s gives no cross references for this selection.", v.Abbrev)))
	}
	legend := false
	for _, r := range rows {
		out.Objects = append(out.Objects, publisherCrossRefRow(r, pal, onTap))
		if strings.Contains(r.Text, "(") {
			legend = true
		}
	}
	if legend {
		out.Objects = append(out.Objects, crossRefCaption("References in parentheses are the edition's \"compare\" citations, as printed."))
	}
	if v.LicenseNotice != "" {
		out.Objects = append(out.Objects, crossRefCaption(v.LicenseNotice))
	}

	// The Treasury, one tap away, under its own heading and credit.
	var detail fyne.CanvasObject
	switch {
	case len(tsk) == 0 && tskErr != nil:
		detail = crossRefCaption("Couldn't load the Treasury of Scripture Knowledge. Check your connection and try again.")
	case len(tsk) == 0:
		detail = crossRefCaption("No Treasury of Scripture Knowledge references for this selection.")
	default:
		objs := make([]fyne.CanvasObject, 0, len(tsk)+1)
		for _, c := range tsk {
			objs = append(objs, crossRefRow(state, c, pal, onTap))
		}
		objs = append(objs, crossRefCaption(tskCredit))
		detail = container.NewVBox(objs...)
	}
	acc := widget.NewAccordion(widget.NewAccordionItem("More references — Treasury of Scripture Knowledge", detail))
	if len(rows) == 0 {
		acc.Open(0)
	}
	out.Disclosure = acc
	out.Objects = append(out.Objects, acc)
	return out
}

// crossRefFooterCredit is the panel footer's credit: the sources whose rows
// stand in the open, and — whenever the on-screen edition is licensed — that
// edition's own notice, because the preview under every row is its verse
// text. The Treasury is named here only while its rows are in the open; in
// the disclosure it carries its own credit.
func crossRefFooterCredit(state *AppState) (sources, notice string) {
	v := state.currentVersion()
	if v.PublisherCrossRefs {
		sources = parallelsCredit
	} else {
		sources = tskCredit + " · " + parallelsCredit
	}
	if !v.PublicDomain && !v.isTesting() {
		notice = v.LicenseNotice
	}
	return sources, notice
}

// publisherCrossRefRow renders one note verbatim: its origin key ("Title" or
// the verse number) and the citation text as one flowing paragraph, with each
// resolved citation a link that jumps to the passage and a citation that
// names the passage on screen set bold and inert.
func publisherCrossRefRow(row publisherCrossRef, pal palette, onTap func(crossRef)) fyne.CanvasObject {
	key := "Title"
	if row.Verse > 0 {
		key = strconv.Itoa(row.Verse)
	}
	origin := canvas.NewText(key, pal.TextMuted)
	origin.TextSize = 12
	origin.TextStyle = fyne.TextStyle{Bold: true}

	runes := []rune(row.Text)
	var segs []widget.RichTextSegment
	words := func(s string) {
		if s != "" {
			segs = append(segs, &widget.TextSegment{Text: s, Style: widget.RichTextStyleInline})
		}
	}
	pos := 0
	for _, tg := range row.Targets {
		if tg.Start < pos || tg.End > len(runes) {
			continue // nested inside the previous citation, or does not fit
		}
		words(string(runes[pos:tg.Start]))
		cited := string(runes[tg.Start:tg.End])
		if tg.Current {
			segs = append(segs, &widget.TextSegment{Text: cited, Style: widget.RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}}})
		} else {
			segs = append(segs, &widget.HyperlinkSegment{Text: cited, OnTapped: func() { onTap(tg.Ref) }})
		}
		pos = tg.End
	}
	words(string(runes[pos:]))
	text := widget.NewRichText(segs...)
	text.Wrapping = fyne.TextWrapWord

	return container.NewVBox(container.NewPadded(container.NewVBox(origin, text)), widget.NewSeparator())
}

func crossRefBlockHeading(s string, pal palette) fyne.CanvasObject {
	t := canvas.NewText(s, pal.Text)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 14
	return container.NewPadded(t)
}

// crossRefCaption is a wrapped, muted, caption-sized line: legends, credits
// and empty states, which must read as annotation rather than as rows.
func crossRefCaption(s string) fyne.CanvasObject {
	rt := widget.NewRichText(&widget.TextSegment{Text: s, Style: widget.RichTextStyle{
		ColorName: theme.ColorNamePlaceHolder, SizeName: theme.SizeNameCaptionText,
	}})
	rt.Wrapping = fyne.TextWrapWord
	return rt
}
