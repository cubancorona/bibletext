package bibletext

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

// Source groups preserve the relationship to the selection independently of
// the destination. The same passage may belong to several verses or events.
type crossRefSourceSet struct {
	id, title, origin string
	verse             int
	refs              []crossRef
}

func crossRefGospelSets(rows []crossRef) []crossRefSourceSet {
	var sets []crossRefSourceSet
	seen := map[string]int{}
	for _, c := range rows {
		id := c.SourceGroup
		if id == "" {
			// A caller without source metadata cannot establish a shared event.
			id = fmt.Sprintf("unkeyed:%t:%s:%s", c.OtherOccasion, c.Title, c.label())
		}
		if i, ok := seen[id]; ok {
			sets[i].refs = append(sets[i].refs, c)
			continue
		}
		origin := c.SourceLabel
		if c.otherOccasion() {
			if origin != "" {
				origin += " · "
			}
			origin += otherOccasionLabel
		}
		seen[id] = len(sets)
		sets = append(sets, crossRefSourceSet{id: id, title: c.Title, origin: origin, refs: []crossRef{c}})
	}
	return sets
}

func crossRefTreasurySets(rows []crossRef) (sets []crossRefSourceSet, unkeyed []crossRef) {
	seen := map[int]int{}
	for _, c := range rows {
		if c.SourceVerse <= 0 {
			unkeyed = append(unkeyed, c)
			continue
		}
		if i, ok := seen[c.SourceVerse]; ok {
			sets[i].refs = append(sets[i].refs, c)
			continue
		}
		seen[c.SourceVerse] = len(sets)
		sets = append(sets, crossRefSourceSet{
			id: "treasury:" + strconv.Itoa(c.SourceVerse), verse: c.SourceVerse, refs: []crossRef{c},
		})
	}
	sort.SliceStable(sets, func(i, j int) bool { return sets[i].verse < sets[j].verse })
	return sets, unkeyed
}

// Summaries abbreviate book names only; every destination and discontinuous
// range remains visible. Publisher note summaries keep their original text.
func crossRefSourceSummary(refs []crossRef) string {
	keys := make([]string, 0, len(shorthandBooks))
	for short := range shorthandBooks {
		keys = append(keys, short)
	}
	sort.Strings(keys)
	shortBook := func(book string) string {
		for _, short := range keys {
			id := shorthandBooks[short]
			if name := usfmToCatholicName[id]; name != "" {
				if _, ok := resolveBookName([]string{book}, name); ok {
					return short
				}
			}
		}
		return book
	}
	labels := make([]string, 0, len(refs))
	for _, c := range refs {
		c.Book = shortBook(c.Book)
		if c.EndBook != "" {
			c.EndBook = shortBook(c.EndBook)
		}
		labels = append(labels, c.label())
	}
	return strings.Join(labels, "; ")
}

func crossRefSourceHeading(title, origin string, pal palette) []fyne.CanvasObject {
	sz := crossRefPanelSizes()
	var rows []fyne.CanvasObject
	if title != "" {
		t := newCrossRefTitle(title, sz.printed, pal)
		t.style, t.color = fyne.TextStyle{Bold: true}, pal.Text
		rows = append(rows, t)
	}
	if origin != "" {
		t := newCrossRefTitle(origin, sz.title, pal)
		t.style = fyne.TextStyle{}
		rows = append(rows, t)
	}
	return rows
}

func crossRefSourceCards(state *AppState, refs []crossRef, selected []Verse, pal palette, onTap func(crossRef)) []fyne.CanvasObject {
	book, chapter := selectedChapter(state, selected)
	rows := make([]fyne.CanvasObject, 0, len(refs))
	for i, c := range refs {
		rule := i < len(refs)-1
		if isWithinSelection(c, book, chapter, selected) {
			rows = append(rows, crossRefSelfRow(c.label(), "", pal, rule))
			continue
		}
		card := passageCard(c, false)
		card.Grouped = true
		rows = append(rows, crossRefRowRuled(state, card, pal, onTap, rule))
	}
	return rows
}

func crossRefGospelBlock(state *AppState, set crossRefSourceSet, selected []Verse, pal palette, onTap func(crossRef), relayout *crossRefRelayout) fyne.CanvasObject {
	build := func() []fyne.CanvasObject { return crossRefSourceCards(state, set.refs, selected, pal, onTap) }
	if len(set.refs) == 1 {
		rows := crossRefSourceHeading(set.title, set.origin, pal)
		rows = append(rows, build()...)
		return crossRefSection(container.New(layout.NewCustomPaddedVBoxLayout(3), rows...), pal)
	}
	return crossRefSection(newCrossRefSourceGroup(set.id, "", set.title, set.origin,
		crossRefSourceSummary(set.refs), true, pal, relayout, build), pal)
}

// Heading-only BSB notes have no word anchor. Their actual heading and its
// placement identify the note without repeating the whole selected passage.
func crossRefHeadingNoteBlock(state *AppState, n publisherCrossRef, pal palette, onTap func(crossRef), relayout *crossRefRelayout) (fyne.CanvasObject, int, int, int) {
	cards, groups, lines := 0, 0, 0
	for _, p := range n.Pieces {
		switch {
		case p.Group != nil:
			groups++
		case p.Target != nil && !p.Target.Current:
			cards++
		default:
			lines++
		}
	}
	origin := fmt.Sprintf("Heading before verse %d", n.Verse)
	build := func() []fyne.CanvasObject { return publisherPieceRows(state, n.Pieces, pal, onTap, relayout) }
	if len(n.Pieces) <= 1 {
		rows := crossRefSourceHeading(n.HeadingText, origin, pal)
		rows = append(rows, build()...)
		return crossRefSection(container.New(layout.NewCustomPaddedVBoxLayout(3), rows...), pal), cards, groups, lines
	}
	id := fmt.Sprintf("heading:%d:%d", n.Verse, n.Index)
	return crossRefSection(newCrossRefSourceGroup(id, "", n.HeadingText, origin, n.Text, true, pal, relayout, build), pal), cards, groups, lines
}

func crossRefSourceRows(objects []fyne.CanvasObject) []*crossRefSourceGroup {
	var rows []*crossRefSourceGroup
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefSourceGroup:
			rows = append(rows, x)
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objects {
		walk(o)
	}
	return rows
}
