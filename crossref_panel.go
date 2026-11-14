package bibletext

// The cross-references panel: a modal that lists the passages related to the
// selected verse(s). It mirrors the AI panel (spinner while the dataset loads on
// first use, then content, or one message), and reuses the chapter-picker
// overlay hide/restore dance. Each card is a full tap target that jumps to the
// passage in context; the list is composed in crossref_list.go.

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// crossRefPanelHeight is the panel's height on a window with room to spare,
// whatever its body holds.
const crossRefPanelHeight = 460

// crossRefListChrome is the width of the panel's chrome beside its list: the
// list is the panel's width less this.
const crossRefListChrome = 44

// crossRefScrollLane is how far the list's scroll reaches past the list's
// right edge, into the panel's padding: the lane its indicator runs in. The
// panel's padding beside the list is half of crossRefListChrome, so the lane
// stays inside the panel.
const crossRefScrollLane float32 = 7

// crossRefScrollLaneLayout gives the scroll the list's place and the lane
// beside it. Its one object is the scroll.
type crossRefScrollLaneLayout struct{}

func (crossRefScrollLaneLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return objs[0].MinSize()
}

func (crossRefScrollLaneLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(fyne.NewSize(size.Width+crossRefScrollLane, size.Height))
}

// crossRefPanelPlace is the reader's place in an open cross-references list,
// captured before a light/dark or layout rebuild drains the panel and applied
// when the replacement list lands.
type crossRefPanelPlace struct {
	notesOpen  []bool
	groupsOpen []bool
	scrollY    float32
}

// crossRefFoldRows returns the list's note rows and group rows in order.
func crossRefFoldRows(objs []fyne.CanvasObject) (notes []*crossRefNoteRow, groups []*crossRefGroupRow) {
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *crossRefNoteRow:
			notes = append(notes, x)
			for _, p := range x.pieces {
				walk(p)
			}
		case *crossRefGroupRow:
			groups = append(groups, x)
			for _, m := range x.members.Objects {
				walk(m)
			}
		case *fyne.Container:
			for _, c := range x.Objects {
				walk(c)
			}
		}
	}
	for _, o := range objs {
		walk(o)
	}
	return notes, groups
}

func showCrossRefs(state *AppState, text string, span selSpan) {
	showCrossRefsAt(state, text, span, nil)
}

func showCrossRefsAt(state *AppState, text string, span selSpan, place *crossRefPanelPlace) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	pal := state.pal()

	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	title := canvas.NewText("Cross-references", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 22
	src := canvas.NewText(citationForSelection(state, text, span), pal.Accent)
	src.TextStyle = fyne.TextStyle{Bold: true}
	src.TextSize = subheadingTextSize
	src.FontSource = crossRefZeroFace(src.TextStyle)
	header := container.NewVBox(title, src, widget.NewSeparator())

	ps := sheetPanelSize(state, cnv)
	bodyW := ps.Width - crossRefListChrome
	listBox := container.New(layout.NewCustomPaddedVBoxLayout(0))
	// One scope for the whole list keeps previews compact and removes the
	// label's inner inset, so references and wrapped text share a left edge.
	sz := crossRefPanelSizes()
	listTheme := crossRefScopeTheme{Theme: passageCitationTheme{fyne.CurrentApp().Settings().Theme()}, text: sz.preview, caption: sz.caption}
	listScope := container.NewThemeOverride(listBox, listTheme)
	scrollBody := container.New(layout.NewCustomPaddedVBoxLayout(0), listScope)
	// squeezeWidthLayout, as the AI panel's columns have: a vertical scroll
	// widens its content to the content's own minimum width and clips the
	// excess sideways with no bar to reach it, so a child that wants more
	// width than the panel has can only clip itself (sheet_fit.go).
	// The list keeps the width it has always had, and the scroll that holds
	// it reaches a few points further to the right, into the panel's own
	// padding, so that its indicator runs in a lane beside the list and not
	// over the right edge of a note's ground (crossRefScrollLane).
	scroll := container.NewVScroll(container.New(squeezeWidthLayout{},
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, crossRefScrollLane), scrollBody)))
	list := container.New(crossRefScrollLaneLayout{}, scroll)
	body := container.NewStack(list)

	var popup *widget.PopUp
	// Stop the spinner on every exit from the thinking state — a running
	// ProgressBarInfinite pins the canvas dirty and repaints the whole tree every
	// frame, which lingers past dismissal and competes with scrolling.
	var thinkingBar *widget.ProgressBarInfinite
	stopThinking := func() {
		if thinkingBar != nil {
			thinkingBar.Stop()
			thinkingBar = nil
		}
	}
	closePanel := func() {
		stopThinking()
		if popup != nil {
			popup.Hide()
		}
		restore()
	}
	closeBtn := widget.NewButton("Close", closePanel)
	footer, footerRule, captionBox, closeRow, sources := crossRefFooterParts(state, closeBtn)
	listRule := widget.NewSeparator()
	listFoot := container.New(layout.NewCustomPaddedLayout(8, 8, 0, 0),
		container.New(layout.NewCustomPaddedVBoxLayout(0), listRule, captionBox))
	// On portrait phones and roomy desktop windows the credits and edition
	// notice stay pinned in the footer; on landscape phones and short desktop
	// windows (ps.Height < crossRefPanelHeight) they follow the rows inside
	// the scroll so the pinned chrome reduces to Close alone.
	placeCaptions := func(pinned bool) {
		if pinned || captionBox.MinSize().Height <= 0 {
			scrollBody.Objects = []fyne.CanvasObject{listScope}
			footer.Objects = []fyne.CanvasObject{footerRule, captionBox, closeRow}
			return
		}
		scrollBody.Objects = []fyne.CanvasObject{listScope, listFoot}
		footer.Objects = []fyne.CanvasObject{footerRule, closeRow}
	}

	var sourcesText string
	var settingScroll, userTouched bool
	keep := func() { keepCrossRefLettersInView(listBox, scroll.Offset.Y) }
	scroll.OnScrolled = func(fyne.Position) {
		if !settingScroll {
			userTouched = true
		}
		keep()
	}

	// fit sizes the panel to its cap (below); every change of body is
	// followed by it, since the body decides whether the panel's least
	// height leaves room below the header.
	var fit func()
	setCentered := func(o fyne.CanvasObject) {
		body.Objects = []fyne.CanvasObject{container.NewVBox(layout.NewSpacer(), o, layout.NewSpacer())}
		body.Refresh()
		fit()
	}
	setMessage := func(msg string) {
		stopThinking()
		lbl := widget.NewLabel(msg)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		setCentered(lbl)
	}
	setThinking := func() {
		// Stop any spinner already running before replacing it, as both sibling
		// transitions do. A ProgressBarInfinite left assigned-over keeps its
		// RepeatForever animation, repainting the whole canvas at ~20fps until
		// GC reclaims it; ui.go documents that as a real defect it once had.
		// Try again re-enters this state from the error, so it is called
		// more than once in a panel's life.
		stopThinking()
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		msg := widget.NewLabel("Finding related passages…")
		msg.Alignment = fyne.TextAlignCenter
		setCentered(container.NewVBox(msg, bar))
	}
	// load asks for the Treasury and lands the list; set below, since the
	// error state's Try again runs it again (ensureCrossRefs forgets a
	// failure, so the retry is a real fetch).
	var load func()
	setOffline := func() {
		stopThinking()
		lbl := widget.NewLabel(crossRefOfflineMessage)
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		// The AI panel's verb for the same state, under the sentence.
		retry := widget.NewButton("Try again", func() { load() })
		setCentered(container.NewVBox(lbl, spacer(12), container.NewCenter(retry)))
	}
	follow := func(cc crossRef) {
		closePanel()
		followCrossRef(state, cc)
	}
	// fitList sizes the list so the panel is exactly as tall as its cap, the
	// list scrolling within it. The chrome around the list (header, footer,
	// padding, border) is MEASURED, as the AI panel's fitBody measures it: a
	// 150pt guess stood here, 25pt short of the chrome the panel has, and
	// the list stood the panel that much over its cap, which on a desktop
	// window put its top edge inside the header. Only while the list is the
	// body: the chrome is what the panel needs besides it.
	listShowing := func() bool {
		return len(body.Objects) == 1 && body.Objects[0] == fyne.CanvasObject(list) && len(listBox.Objects) > 0
	}
	capturePlace := func() *crossRefPanelPlace {
		if !listShowing() {
			return nil
		}
		notes, groups := crossRefFoldRows(listBox.Objects)
		saved := &crossRefPanelPlace{
			notesOpen:  make([]bool, len(notes)),
			groupsOpen: make([]bool, len(groups)),
			scrollY:    scroll.Offset.Y,
		}
		for i, n := range notes {
			saved.notesOpen[i] = n.open
		}
		for i, g := range groups {
			saved.groupsOpen[i] = g.open
		}
		return saved
	}
	fitList := func() {
		pinned := ps.Height >= crossRefPanelHeight
		setCrossRefSources(sources, sourcesText, bodyW)
		placeCaptions(pinned)
		footer.Refresh()
		scrollBody.Refresh()
		scroll.SetMinSize(fyne.NewSize(bodyW, 1))
		chrome := popup.MinSize().Height - scroll.MinSize().Height
		h := ps.Height - chrome
		if h < 1 {
			h = 1
		}
		scroll.SetMinSize(fyne.NewSize(bodyW, h))
	}
	mountList := func(lst crossRefList, keepPlace bool) {
		listBox.Objects = lst.Objects
		if place != nil {
			notes, groups := crossRefFoldRows(lst.Objects)
			for i, n := range notes {
				if i < len(place.notesOpen) && n.open != place.notesOpen[i] {
					n.toggle()
				}
			}
			for i, g := range groups {
				if i < len(place.groupsOpen) && g.header != nil && g.open != place.groupsOpen[i] {
					g.toggle()
				}
			}
		}
		listScope.Refresh()
		// A group that opens or closes changes the list's height inside the
		// scroll; the panel keeps its size and its place (crossref_group.go).
		// The scope is refreshed, not the list inside it: a group's cards
		// are made at the tap, and the scope has to take them in before
		// anything measures them, or they are measured at the app's text
		// size and padding and the heights of that measure are left
		// standing — a note's ground taller than its rows, or the scroll's
		// content longer than the list — until the next frame.
		// The letter of a note or opened lone group the view stands inside is
		// kept in view (crossref_note.go), as the list is scrolled and when a
		// row's height changes under it.
		lst.Relayout.fn = func() {
			userTouched = true
			listScope.Refresh()
			scroll.Refresh()
			keep()
		}
		sourcesText, _ = crossRefFooterCredit(state, lst.Sections)
		body.Objects = []fyne.CanvasObject{list}
		body.Refresh()
		settingScroll = true
		fit()
		if place != nil {
			scroll.Refresh()
			scroll.Offset = fyne.NewPos(0, place.scrollY)
			scroll.Refresh()
			keep()
			if !keepPlace {
				place = nil
			}
		} else {
			scroll.ScrollToTop()
		}
		settingScroll = false
		userTouched = false
	}
	// setLoading shows the embedded publisher notes and Gospel parallels
	// immediately when the loaded edition displays its own apparatus and has
	// local content for the selection, putting the spinner only under the
	// Treasury heading while the dataset downloads; otherwise it shows the
	// whole-panel spinner.
	setLoading := func() {
		if !state.currentVersion().PublisherCrossRefs {
			setThinking()
			return
		}
		if (place == nil || userTouched) && listShowing() {
			place = capturePlace()
		}
		stopThinking()
		bar := widget.NewProgressBarInfinite()
		thinkingBar = bar
		localRefs := localCrossRefsForSelection(state, text, span)
		lst := buildCrossRefPanelList(state, selectionVerses(state, text, span), localRefs,
			crossRefTreasuryState{loading: true, bar: bar}, pal, follow)
		if len(lst.Objects) == 0 || !lst.Sections.Publisher {
			setThinking()
			return
		}
		mountList(lst, true)
	}
	// showRefs lands the load: the list when it holds anything, else one
	// message — the connection error when the Treasury did not load and
	// nothing else answered for the selection, the empty sentence otherwise.
	// The footer's sources line is set from the list that was built, so it
	// names only the sets on screen, and is laid out at the footer's width
	// before the list is fitted, so the chrome measured counts its wrapped
	// height.
	showRefs := func(refs []crossRef, tskErr error) {
		stopThinking()
		if popup != nil && !popup.Visible() {
			return
		}
		if (place == nil || userTouched) && listShowing() {
			place = capturePlace()
		}
		lst := buildCrossRefPanelList(state, selectionVerses(state, text, span), refs,
			crossRefTreasuryState{err: tskErr, onRetry: func() { load() }}, pal, follow)
		if len(lst.Objects) == 0 {
			if tskErr != nil {
				setOffline()
				return
			}
			msg := crossRefNoneLine
			if lst.CoverageNote != "" {
				msg += "\n" + lst.CoverageNote
			}
			setMessage(msg)
			return
		}
		mountList(lst, false)
	}

	content := container.NewBorder(header, footer, nil, nil, body)
	popup = widget.NewModalPopUp(
		surface(container.NewPadded(content), pal.SurfaceAlt, pal.Border, fyne.Size{}),
		cnv,
	)
	popup.Show()
	// Capped by the window, and on a short desktop window covering the header
	// where the body's state leaves the panel too tall to clear it: the
	// waiting state's message and bar do not scroll. Covering, it is the
	// panel it is on a window with room to spare, crossRefPanelHeight tall
	// with the body centred in it, capped at the room.
	fit = func() {
		if listShowing() {
			fitList()
		} else {
			placeCaptions(true)
			footer.Refresh()
			scrollBody.Refresh()
		}
		popup.Resize(fyne.NewSize(ps.Width, minF(ps.Height, crossRefPanelHeight)))
		coverHeaderUnlessClear(state, popup, crossRefPanelHeight)
		if listShowing() {
			keep()
		}
	}
	fit()
	// Sized again from the height a desktop window now gives it whenever the
	// window changes size, in whichever state it is (sheet_refit.go).
	registerSheetRefit(state, popup, func() {
		ps = sheetPanelSize(state, cnv)
		bodyW = ps.Width - crossRefListChrome
		fit()
	})
	// The same selection again after a light/dark or layout rebuild, keeping
	// what the reader had opened or closed and where the list stood. The
	// dataset loads once and is guarded, so a reopen mid-load waits on the
	// same load rather than starting another (sheet_reopen.go).
	registerSheetReopenCapture(state, popup, func() func() {
		stopThinking()
		saved := place
		if saved == nil || userTouched {
			saved = capturePlace()
		}
		return func() { showCrossRefsAt(state, text, span, saved) }
	})

	// A selection the cross-references cannot cover at all is answered
	// before anything loads: the answer cannot change, and a reader who is
	// offline must not be told to check the connection for it
	// (panelCoverage).
	book, _ := selectionChapter(state, span)
	if note, none := panelCoverage(state, book, selectionVerses(state, text, span)); none {
		setMessage(crossRefNoneLine + "\n" + note)
		return
	}

	load = func() {
		setLoading()
		crossRefsRun(func() {
			err := crossRefsLoad()
			// crossRefsForSelection always returns the embedded Gospel
			// parallels (offline), plus the Treasury rows when they loaded,
			// and the publisher's run needs no network: a Treasury that
			// failed to load is reported in its own place when anything else
			// answers for the selection, and as the panel's one message when
			// nothing does (buildCrossRefList, showRefs).
			refs := crossRefsForSelection(state, text, span)
			fyne.Do(func() { showRefs(refs, err) })
		})
	}
	load()
}

// crossRefFooter is the panel's footer: a rule, the sources line, the
// edition's notice once whenever the loaded edition is licensed — the preview
// under every card is its verse text — and Close. The sources line is empty
// and hidden until the list is built, since it names only the sets on screen
// (setCrossRefSources). It is a caption, which wraps: as a canvas.Text it
// wanted 333 points against the 299 a 375-point phone gives the footer, and
// clipped.
func crossRefFooter(state *AppState, closeBtn fyne.CanvasObject) (footer *fyne.Container, sources *widget.RichText) {
	footer, _, _, _, sources = crossRefFooterParts(state, closeBtn)
	return footer, sources
}

func crossRefFooterParts(state *AppState, closeBtn fyne.CanvasObject) (footer *fyne.Container, rule *widget.Separator, captionBox, closeRow *fyne.Container, sources *widget.RichText) {
	sources = crossRefCaption("")
	sources.Hide()
	lines := []fyne.CanvasObject{sources}
	if _, notice := crossRefFooterCredit(state, crossRefSections{}); notice != "" {
		lines = append(lines, crossRefCaption(notice))
	}
	captions := container.New(layout.NewCustomPaddedVBoxLayout(6), lines...)
	captionScope := container.NewThemeOverride(captions, passageCitationTheme{fyne.CurrentApp().Settings().Theme()})
	rule = widget.NewSeparator()
	captionBox = container.New(captionPadLayout{}, captionScope)
	closeRow = container.New(layout.NewCustomPaddedLayout(7, 0, 0, 0), container.NewHBox(layout.NewSpacer(), closeBtn))
	footer = container.New(layout.NewCustomPaddedVBoxLayout(0), rule, captionBox, closeRow)
	return footer, rule, captionBox, closeRow, sources
}

// captionPadLayout gives the footer's captions an 8pt gap below the rule and a
// 2pt gap above the Close row's 7pt top inset when at least one caption is
// visible, and collapses to zero height when every caption is hidden.
type captionPadLayout struct{}

func (captionPadLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 8))
		o.Resize(fyne.NewSize(size.Width, max(float32(0), size.Height-3)))
	}
}

func (captionPadLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var min fyne.Size
	for _, o := range objects {
		if o.Visible() {
			min = min.Max(o.MinSize())
		}
	}
	if min.Height <= 0 {
		return fyne.Size{}
	}
	return fyne.NewSize(min.Width, min.Height+3)
}

// setCrossRefSources puts text on the footer's sources line, or hides the line
// when there is nothing to name. The line is laid out at width first so its
// MinSize is its wrapped height before the panel measures its chrome.
func setCrossRefSources(sources *widget.RichText, text string, width float32) {
	if text == "" {
		sources.Hide()
		return
	}
	sources.Segments = []widget.RichTextSegment{&widget.TextSegment{Text: text, Style: crossRefCaptionStyle}}
	sources.Show()
	sources.Resize(fyne.NewSize(width, sources.MinSize().Height))
	sources.Refresh()
}

// crossRefsRun runs the panel's load and its landing, on a goroutine of their
// own, and crossRefsLoad is the load: ensureCrossRefs, which may fetch the
// Treasury. Seams for tests, which keep the panel in the state it opened in by
// never running the work, or run it where they stand with the load answered,
// so nothing touches the panel from another goroutine while they read it.
var (
	crossRefsRun  = func(work func()) { go work() }
	crossRefsLoad = ensureCrossRefs
)

// followCrossRef takes the reader to a row's passage and washes what the row
// cites: the whole range, as Go to and the verse of the day wash theirs, not
// only its first verse. A wash lies within one chapter (VerseSpan), so a range
// that runs on into the next chapter or book is washed from its start to the
// end of the chapter it starts in.
func followCrossRef(state *AppState, c crossRef) {
	v := state.Bible.GetVerse(c.Book, c.Chapter, c.Verse)
	if v == nil {
		return
	}
	if c.TitleRef {
		goToChapterTitle(state, v.BookName, v.Chapter)
		return
	}
	end := v.Verse
	switch {
	case c.EndV == 0:
	case !c.crossBook() && (c.EndCh == 0 || c.EndCh == c.Chapter):
		end = c.EndV
	default:
		if ch := state.Bible.GetChapter(v.BookName, v.Chapter); len(ch) > 0 {
			end = ch[len(ch)-1].Verse
		}
	}
	goToVerseRange(state, v.BookName, v.Chapter, v.Verse, end)
}

// crossRefPreviewRunes is how much of a row's first verse its preview shows.
const crossRefPreviewRunes = 90

// crossRefPreview is the opening of a row's verse, cut at a word: the cut used
// to fall wherever the 90th character did, mid-word in about half of all rows
// and on a space in another seventh, which put a space before the ellipsis.
func crossRefPreview(text string) string {
	r := []rune(collapseSpaces(text))
	if len(r) <= crossRefPreviewRunes {
		return string(r)
	}
	cut := string(r[:crossRefPreviewRunes])
	if r[crossRefPreviewRunes] != ' ' {
		// Back to the last whole word, unless that would leave too little.
		if i := strings.LastIndexByte(cut, ' '); i >= len(cut)/2 {
			cut = cut[:i]
		}
	}
	return strings.TrimRight(cut, " ,;:") + "…"
}

// crossRefCard is one card of the list: the passage it follows and previews,
// and how it is labelled and tagged. One constructor (crossRefRow) draws every
// passage in the panel — a Treasury row, a Gospel parallel, a publisher's
// citation — so the list has one card idiom.
type crossRefCard struct {
	Ref   crossRef // the passage the card follows and previews
	Label string   // the reference line: Ref.label() for a Treasury or parallel row, the chapter for a citation of one
	// Tag is the muted word beside the reference, in the slot the parallel
	// pill uses, saying which passage this is: "Title" for a citation of a
	// Psalm's title, a basis ("LXX", "LXX, DSS") for a citation the edition
	// qualifies — search draws exactly this for a title hit. "" for none.
	Tag string
	// Title marks a card whose passage is a Psalm's title: the preview is the
	// superscription, not the first verse, which is where the tap lands.
	Title bool
	// Letter is the note's letter, set in the margin beside the reference
	// of a card that is the whole of its note (crossref_section.go); "" for
	// every other card.
	Letter string
	// Badge draws the PARALLEL pill on a same-occasion parallel: only where no
	// section label carries the row's provenance, the builds without the
	// publisher's run. Under a GOSPEL PARALLELS label the pill would say the
	// label's word again, in the most saturated object on the screen.
	Badge bool
}

// passageCard is the card for a row crossRefsForSelection hands over — a
// Treasury row or a Gospel parallel — labelled as the row labels itself.
func passageCard(c crossRef, badge bool) crossRefCard {
	return crossRefCard{Ref: c, Label: c.label(), Badge: badge}
}

func crossRefRow(state *AppState, card crossRefCard, pal palette, onTap func(crossRef)) fyne.CanvasObject {
	return crossRefRowRuled(state, card, pal, onTap, true)
}

// crossRefRowRuled is crossRefRow with or without the rule that closes it.
// It has none as the last card on an open group's ground, as the last row
// of a note, or as a card that is the whole of its note: the ground it
// stands on ends under it (crossref_group.go, crossref_section.go).
func crossRefRowRuled(state *AppState, card crossRefCard, pal palette, onTap func(crossRef), rule bool) fyne.CanvasObject {
	sz := crossRefPanelSizes()
	c := card.Ref
	refStyle := fyne.TextStyle{Bold: true}
	ref := newCrossRefHeadingText(card.Label, sz.reference, refStyle, pal.Accent)

	// The reference line, with a "Parallel" tag for Gospel-synopsis entries
	// where the list has no label to say so, or the card's own muted tag. A
	// row of the same saying on another occasion is not the same event and
	// does not wear the pill: it carries its own label, on a line of its own
	// because the label is longer than the tag and a phone has no room for it
	// beside a long reference.
	refLine := ref
	switch {
	case card.Badge && c.Parallel && !c.otherOccasion():
		refLine = container.New(&crossRefHeadingFlow{gap: 7}, ref, parallelBadge(pal))
	case card.Tag != "":
		// The idiom search.go draws for a title hit: the tag beside the
		// reference rather than under it, so the two read as one heading, and
		// dropping under it only when a narrow well at Extra large leaves no
		// room beside a long book name.
		tag := canvas.NewText(card.Tag, pal.TextMuted)
		tag.TextSize = sz.title
		refLine = container.New(&crossRefHeadingFlow{gap: 7}, ref, tag)
	}
	lines := []fyne.CanvasObject{refLine}
	if c.otherOccasion() {
		lines = append(lines, container.NewHBox(otherOccasionBadge(pal)))
	}

	// For a parallel, the pericope title ("The Beatitudes") is the useful context,
	// so show it above the verse preview; for another occasion, the saying's.
	if c.Parallel && c.Title != "" {
		lines = append(lines, newCrossRefTitle(c.Title, sz.title, pal))
	}

	// The preview carries the divine name's small capitals as the page draws
	// them (small_caps_draw.go): in the NKJV they are the whole difference
	// between "Lᴏʀᴅ", the Tetragrammaton, and "Lord", Adonai, and the raw text
	// the preview used to show has the same letters for both. They are
	// characters, so the 90-character cut is unchanged; the chrome face has no
	// small capitals and the toolkit draws them from its own fallback face, as
	// a shared verse's are drawn wherever it is pasted. A title card previews
	// the superscription, drawn the same way; a chapter whose title the loaded
	// text lacks previews its first verse, where the tap lands.
	snippet := ""
	if card.Title {
		if s := state.Bible.SuperscriptionFor(c.Book, c.Chapter); strings.TrimSpace(s.Text) != "" {
			snippet = crossRefPreview(drawSmallCaps(s.Text, s.SmallCaps))
		}
	}
	if snippet == "" {
		if v := state.Bible.GetVerse(c.Book, c.Chapter, c.Verse); v != nil {
			snippet = crossRefPreview(smallCapsText(*v))
		}
	}
	snip := widget.NewLabel(snippet)
	snip.Wrapping = fyne.TextWrapWord
	lines = append(lines, snip)

	var body fyne.CanvasObject = container.New(layout.NewCustomPaddedVBoxLayout(3), lines...)
	if card.Letter != "" {
		// A card that is the whole of a publisher's note: its letter in the
		// margin, on the reference's line, inside the tap
		// (crossref_section.go).
		body = crossRefLettered(card.Letter, pal, body, 0, sz.reference, refStyle)
	}
	inner := container.New(layout.NewCustomPaddedLayout(6, 6, 0, 0), body)
	tap := newTapCard(inner, crossRefHover(pal), func() { onTap(c) })
	block := []fyne.CanvasObject{tap}
	if rule {
		block = append(block, widget.NewSeparator())
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(0), block...)
}

// parallelBadge is the small accent pill that marks a Gospel-synopsis parallel row.
func parallelBadge(pal palette) fyne.CanvasObject {
	t := canvas.NewText("PARALLEL", pal.AccentText)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 9
	bg := canvas.NewRectangle(pal.Accent)
	bg.CornerRadius = 5
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 7, 7), t))
}

// otherOccasionLabel is the label on a row of the second kind of Gospel
// parallel (parallels.go): the same saying, recorded on an occasion other
// than the verse's. Reader-facing wording, in one place. The next major
// release's (docs/NEXT.md), where its wording is an open decision.
const otherOccasionLabel = "Same saying, another occasion"

// otherOccasionBadge marks such a row: the accent's outline where the
// parallel tag is filled, so the two kinds read as related and different.
func otherOccasionBadge(pal palette) fyne.CanvasObject {
	t := canvas.NewText(otherOccasionLabel, pal.Accent)
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = 10
	bg := canvas.NewRectangle(color.Transparent)
	bg.StrokeColor = pal.Accent
	bg.StrokeWidth = 1
	bg.CornerRadius = 5
	return container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(2, 2, 7, 7), t))
}

// tapCard makes an arbitrary content block one tap target, with a desktop hover
// wash and pointer cursor — the generic form of search.go's result card.
type tapCard struct {
	widget.BaseWidget
	content fyne.CanvasObject
	hoverBg color.NRGBA
	onTap   func()
	bg      *canvas.Rectangle
}

func newTapCard(content fyne.CanvasObject, hoverBg color.NRGBA, onTap func()) *tapCard {
	c := &tapCard{content: content, hoverBg: hoverBg, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *tapCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(color.Transparent)
	c.bg.CornerRadius = 8
	c.bg.SetMinSize(fyne.NewSize(0, 44))
	return widget.NewSimpleRenderer(container.NewStack(c.bg, c.content))
}

func (c *tapCard) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *tapCard) MouseIn(*desktop.MouseEvent) {
	if c.bg != nil {
		c.bg.FillColor = c.hoverBg
		c.bg.Refresh()
	}
}
func (c *tapCard) MouseMoved(*desktop.MouseEvent) {}
func (c *tapCard) MouseOut() {
	if c.bg != nil {
		c.bg.FillColor = color.Transparent
		c.bg.Refresh()
	}
}
func (c *tapCard) Cursor() desktop.Cursor { return desktop.PointerCursor }

var (
	_ fyne.Tappable      = (*tapCard)(nil)
	_ desktop.Hoverable  = (*tapCard)(nil)
	_ desktop.Cursorable = (*tapCard)(nil)
)
