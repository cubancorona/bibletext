package bibletext

// The cross-references panel's list, assembled from the three sources that
// can appear in it — the Gospel synopsis parallels, the publisher's own
// apparatus (publisher_xrefs.go: the NKJV's, and only in a build that shows
// it), and the Treasury of Scripture Knowledge — each under its own heading
// and its own credit, never interleaved: a licensed publisher's citations
// mixed into a list credited to OpenBible.info would misattribute both
// (docs/SOURCE_FIELDS_DECISIONS.md). Kept apart from the popup that mounts it
// so the composition can be tested without a window.
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

// crossRefList is the assembled list plus what it holds, for the panel and
// for the tests that pin the composition.
type crossRefList struct {
	Objects        []fyne.CanvasObject
	Parallels      int
	PublisherRows  int
	TSKRows        int
	PublisherBlock bool              // the edition's block was rendered (rows or its empty state)
	Disclosure     *widget.Accordion // the Treasury's disclosure; nil when its rows stand in the open
}

// buildCrossRefList composes the list for one selection. refs is
// crossRefsForSelection's answer (parallels first, then the Treasury rows);
// tskErr is the Treasury's load error when it has none, which a list with a
// publisher block reports in the Treasury's own place rather than instead of
// everything else.
func buildCrossRefList(state *AppState, selected []Verse, refs []crossRef, tskErr error, pal palette, onTap func(crossRef)) crossRefList {
	var out crossRefList
	var tsk []crossRef
	for _, c := range refs {
		if c.Parallel {
			out.Parallels++
			out.Objects = append(out.Objects, crossRefRow(state, c, pal, onTap))
			continue
		}
		tsk = append(tsk, c)
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
