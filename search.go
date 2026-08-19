package bibletext

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// trackSearchScroll remembers a results list's scroll position in state and restores
// it once the list has laid out — so returning to the Search tab lands where you left
// off. A new search resets state.searchScrollY to 0, so live/typed results start at top.
func trackSearchScroll(state *AppState, scroll *container.Scroll) {
	scroll.OnScrolled = func(p fyne.Position) { state.searchScrollY = p.Y }
	if state.searchScrollY > 0 {
		target := state.searchScrollY
		time.AfterFunc(60*time.Millisecond, func() {
			fyne.Do(func() { scroll.ScrollToOffset(fyne.NewPos(0, target)) })
		})
	}
}

func buildSearchResultsView(state *AppState) fyne.CanvasObject {
	// Notes mode is its own corpus, so it short-circuits before any of the verse
	// paths below. searchModeOf validates the flag against the feature switch, so
	// a mode left over from before notes were turned off cannot land here.
	if searchModeOf(state) == modeNotes {
		return buildNotesBrowseView(state)
	}
	// When the current results context is the AI search, render the matching state — the
	// passages, or (driven from state, for desktop where results replace the reading pane)
	// the in-progress / no-key / error / prompt states. This also powers "back to results"
	// and the Read-tab inline results for AI Find.
	// The aiFeaturesEnabled guard keeps a leftover aiSearchActive (set before the
	// reader switched the assistant to "None") from routing into the AI result
	// views — it falls through to plain keyword results below.
	if aiFeaturesEnabled(state) && state.aiSearchActive {
		switch {
		case state.aiSearchLoading:
			return aiSearchingView(state)
		case !hasAIKey(state):
			return aiNoKeyView(state)
		case state.aiSearchErr != nil:
			return aiSearchMessageView(friendlyAIError(state.aiSearchErr), "Try again", func() {
				if state.retryAISearch != nil {
					state.retryAISearch()
				}
			})
		case state.aiSearchCancelled && len(state.aiSearchResults) == 0:
			// Before the empty-results case below: the reader stopped this
			// search, so the pane must not claim the AI found nothing. Gated on
			// having NO results: the flag exists only to suppress a false
			// zero-result message, so it must never hide real, paid-for
			// passages if some path leaves it stale.
			return aiSearchMessageView("Search cancelled.", "Try again", func() {
				if state.retryAISearch != nil {
					state.retryAISearch()
				}
			})
		case len(state.aiSearchResults) == 0 && strings.TrimSpace(state.aiSearchQuery) == "":
			return aiSearchPromptView(state)
		default:
			return aiResultsView(state, state.aiSearchQuery, state.aiSearchResults)
		}
	}
	pal := state.pal()
	trimmed := strings.TrimSpace(state.ActiveSearchQuery)

	// Before a real query is entered, show a calm, centred prompt rather than an
	// empty `Results for ""` heading sitting over an empty bordered box.
	if len([]rune(trimmed)) < 2 {
		return searchPromptView(state)
	}

	// The query already shows in the search field above, so the results header is just
	// a compact muted count line (no big "Results for …" heading) — keeps the results
	// taking most of the pane.
	var sub string
	switch {
	case len(state.SearchResults) == 0:
		sub = "No verses matched your search."
	case state.SearchTruncated:
		sub = fmt.Sprintf("Showing the first %d matches — refine your search to narrow it down.", len(state.SearchResults))
	default:
		sub = fmt.Sprintf("%d matches", len(state.SearchResults))
	}
	subLabel := canvas.NewText(sub, pal.TextMuted)
	subLabel.TextSize = subheadingTextSize

	terms := strings.Fields(strings.ToLower(trimmed))

	rows := make([]fyne.CanvasObject, 0, len(state.SearchResults))
	for _, verse := range state.SearchResults {
		rows = append(rows, searchResultRow(state, verse, terms, pal))
	}

	column := container.New(&readingColumn{maxWidth: 820}, container.NewVBox(rows...))
	scroll := container.NewVScroll(column)
	trackSearchScroll(state, scroll)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	head := container.NewVBox(subLabel, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

// searchPromptView is the calm, centred empty state shown before a query is
// entered — a muted search glyph and a one-line invitation. Clearer than echoing
// `Results for ""` over an empty results box.
func searchPromptView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.SearchIcon(), colorNameMuted))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(44, 44))

	title := canvas.NewText("Search the Bible", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter

	hint := canvas.NewText("A word or phrase, or a reference like John 3:16.", pal.TextMuted)
	hint.TextSize = subheadingTextSize
	hint.Alignment = fyne.TextAlignCenter

	col := container.NewVBox(
		container.NewCenter(icon),
		spacer(12),
		container.NewCenter(title),
		spacer(4),
		container.NewCenter(hint),
	)
	return container.NewCenter(col)
}

// aiResultsView renders AI-found passages as the same tappable cards as keyword
// search (no term highlight), with an honesty note — the passages are AI-suggested,
// but the text shown is the real verse from our Bible.
func aiResultsView(state *AppState, query string, verses []Verse) fyne.CanvasObject {
	pal := state.pal()
	_ = query // the request shows in the Find field above; no big heading here

	sub := fmt.Sprintf("%d passages found by AI", len(verses))
	switch len(verses) {
	case 0:
		sub = "AI didn’t find matching passages — try rephrasing."
	case 1:
		sub = "1 passage found by AI"
	}
	subLabel := canvas.NewText(sub, pal.TextMuted)
	subLabel.TextSize = subheadingTextSize

	rows := make([]fyne.CanvasObject, 0, len(verses))
	for _, v := range verses {
		rows = append(rows, searchResultRow(state, v, nil, pal))
	}
	column := container.New(&readingColumn{maxWidth: 820}, container.NewVBox(rows...))
	scroll := container.NewVScroll(column)
	trackSearchScroll(state, scroll)
	paper := surface(container.NewPadded(scroll), pal.Surface, pal.Border, fyne.Size{})

	note := canvas.NewText("AI-suggested passages — read each in context.", pal.TextMuted)
	note.TextSize = 11
	head := container.NewVBox(subLabel, note, widget.NewSeparator())
	return container.NewPadded(container.NewBorder(head, nil, nil, nil, paper))
}

// aiSearchPromptView is the calm empty state for the AI Find (passage search)
// mode, shown before a request is entered.
func aiSearchPromptView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.SearchIcon(), colorNameMuted))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(44, 44))

	title := canvas.NewText("Find passages by meaning", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20
	title.Alignment = fyne.TextAlignCenter

	hint := canvas.NewText(`e.g. "what did God say to Jonah?"`, pal.TextMuted)
	hint.TextSize = subheadingTextSize
	hint.Alignment = fyne.TextAlignCenter

	col := container.NewVBox(
		container.NewCenter(icon), spacer(12),
		container.NewCenter(title), spacer(4),
		container.NewCenter(hint),
	)
	return container.NewCenter(col)
}

// aiNoKeyView is the clean, non-intrusive explanation shown in AI Find mode when no
// provider key is set: what it does, that it needs the reader's own key, and a quiet
// route into settings. No error styling.
func aiNoKeyView(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("Find needs your own AI key", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	title.Alignment = fyne.TextAlignCenter

	body := widget.NewLabel("Describe what you're looking for and AI finds the passages. It uses your own AI provider key, stored only on this device.")
	body.Wrapping = fyne.TextWrapWord
	body.Alignment = fyne.TextAlignCenter

	setup := widget.NewButton("Set up AI", func() { showAISettings(state) })
	setup.Importance = widget.HighImportance

	col := container.NewVBox(
		container.NewCenter(title), spacer(6),
		wrappedParagraph(body, 300), spacer(14),
		container.NewCenter(setup),
	)
	return container.NewCenter(col)
}

// aiSearchMessageView centres a short message with an optional action button —
// used for AI-search errors (with a Try again retry).
func aiSearchMessageView(msg, action string, onAction func()) fyne.CanvasObject {
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Alignment = fyne.TextAlignCenter

	items := []fyne.CanvasObject{wrappedParagraph(lbl, 300)}
	if action != "" && onAction != nil {
		btn := widget.NewButton(action, onAction)
		items = append(items, spacer(12), container.NewCenter(btn))
	}
	return container.NewCenter(container.NewVBox(items...))
}

// aiSearchingView is the in-progress state while an AI passage search runs (used on
// desktop, where results replace the reading pane). A plain centered line — no animated
// ProgressBarInfinite, which would pin the whole canvas dirty (see the scroll-lag note).
func aiSearchingView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	msg := canvas.NewText("Searching with AI…", pal.TextMuted)
	msg.Alignment = fyne.TextAlignCenter
	msg.TextSize = 16

	// No ticking elapsed counter on purpose: a per-second repaint pins the
	// canvas dirty (the same reason there is no ProgressBarInfinite here). One
	// honest line sets the expectation instead — a high-capability model can
	// legitimately think for a minute or more (aiRequestBudget) — and Cancel
	// keeps the wait the reader's choice, not a constant's.
	// Width-bounded caption (the app's muted style) so the column has a stable
	// measure and every element centres against the same axis.
	hint := container.NewGridWrap(fyne.NewSize(260, captionHeightFor(2)),
		centeredCaption("Capable models can take a minute or more."))

	// The handler is a WRAPPER, not state.cancelAISearch itself: a Button stores
	// the func VALUE it is given, so binding the field directly would pin
	// whichever search was in flight when this view was built — from the second
	// Find on, Cancel would abandon the PREVIOUS request and leave the current
	// one running. Reading the field at tap time always hits the live one.
	items := []fyne.CanvasObject{container.NewCenter(msg), spacer(10), container.NewCenter(hint)}

	cancelBtn := widget.NewButton("Cancel", func() {
		abandonAISearch(state)
		state.aiSearchCancelled = true
		state.refresh() // → the cancelled state (never a false "found nothing")
	})
	// inputFrame: the theme fills buttons with SurfaceAlt, which is near-equal
	// to the page ground here (and IS the card fill on the study panel), so a
	// bare button reads as floating text (observed in practice). The outline gives it
	// the same visible box the app's cards and fields carry.
	items = append(items, spacer(4), container.NewCenter(inputFrame(cancelBtn, state.pal().Border)))

	// Below Cancel, and quieter than it (fasterModelControl): an offer for the
	// impatient, not a competing action — shown only when there IS something
	// faster. Switching re-asks the same question straight away, since the
	// reader is sitting here waiting for exactly that answer.
	if pid, model, label, ok := fasterModelOffer(state); ok {
		items = append(items, spacer(6), fasterModelControl(label, func() {
			q := state.aiSearchQuery
			abandonAISearch(state)
			applyFasterModel(state, pid, model)
			if state.retryAISearch != nil && strings.TrimSpace(q) != "" {
				state.retryAISearch()
				return
			}
			state.aiSearchCancelled = true
			state.refresh()
		}))
	}
	return container.NewCenter(container.NewVBox(items...))
}

// buildSearchModeControls builds the whole mode row; see its definition below.
// This older comment describes the two-segment Search/Find pair it contains: the
// active half is filled, it is shared by the mobile Search tab and the desktop
// sidebar, and on desktop it renders compact and quieter
// (smaller text/padding, flat inactive half) — touch-sized buttons are too intrusive for
// a mouse UI. (The narrative-answer "Ask" lives on the reading selection menu, not here.)
// searchMode is which corpus the Search tab is pointed at.
type searchMode int

const (
	modeKeyword searchMode = iota // the Bible, by word
	modeFind                      // the Bible, by natural-language AI Find
	modeNotes                     // the notes people have shared with you
)

// searchModeOf resolves the two mode flags into one answer, and is the ONLY
// place that knows the precedence. Both flags are validated against whether
// their feature is switched on, so a mode left over from before the reader
// turned the assistant to "None" (or turned notes off) can never strand the tab
// in a mode whose control is no longer on screen.
func searchModeOf(state *AppState) searchMode {
	switch {
	case state.NotesMode && notesFeatureOn(state):
		return modeNotes
	case state.aiSearchMode && aiFeaturesEnabled(state):
		return modeFind
	}
	return modeKeyword
}

// buildSearchModeControls builds the whole mode row: the Search / Find pair —
// the two ways of looking through SCRIPTURE — and, set apart on the right, the
// shared-notes bubble.
//
// ONE builder for all three, and this is not tidiness. The fill that marks the
// active mode has to move BETWEEN them, so whatever owns it must be able to
// reach all three at once. Built as two independent widgets, tapping Find lit
// Find while the notes bubble stayed lit as well — two modes claiming to be
// active, because the pair's apply() had never heard of the bubble.
//
// Notes is not a third segment inside the pair: it searches a different corpus,
// and a third text segment implied the three were the same kind of thing. It is
// an icon — the note bubble a note is actually drawn as — so the control and
// what it opens read as one object, and so it cannot be mistaken for a third way
// to search the Bible.
//
// Each half disappears with its feature (no assistant → no pair; notes off → no
// bubble), and with neither left there is nothing to choose and the row collapses
// to a zero-height spacer. Callers lay it out unconditionally and rely on that.
func buildSearchModeControls(state *AppState, onSelect func(mode searchMode)) fyne.CanvasObject {
	aiOn := aiFeaturesEnabled(state)
	notesOn := notesFeatureOn(state)
	if !aiOn && !notesOn {
		return spacer(0)
	}

	compact := !fyne.CurrentDevice().IsMobile()
	idle := widget.MediumImportance
	if compact {
		idle = widget.LowImportance // flat inactive → only the active one is filled
	}

	var kwBtn, aiBtn, notesBtn *widget.Button
	// apply is the single place the fill lives. Every control the row owns is set
	// on every change, so exactly one of them can ever look active.
	apply := func(m searchMode) {
		if kwBtn != nil {
			kwBtn.Importance = idle
			aiBtn.Importance = idle
			switch m {
			case modeKeyword:
				kwBtn.Importance = widget.HighImportance
			case modeFind:
				aiBtn.Importance = widget.HighImportance
			}
			kwBtn.Refresh()
			aiBtn.Refresh()
		}
		if notesBtn != nil {
			notesBtn.Importance = widget.LowImportance
			if m == modeNotes {
				notesBtn.Importance = widget.HighImportance
			}
			notesBtn.Refresh()
		}
	}

	var pair fyne.CanvasObject = spacer(0)
	if aiOn {
		kwBtn = widget.NewButton("Search", func() { apply(modeKeyword); onSelect(modeKeyword) })
		aiBtn = widget.NewButton("Find", func() { apply(modeFind); onSelect(modeFind) })
		grid := container.NewGridWithColumns(2, kwBtn, aiBtn)
		pair = grid
		if compact {
			// Shrink the text + padding so the pair reads as a small, elegant control.
			var base fyne.Theme = theme.DefaultTheme()
			if state.theme != nil {
				base = state.theme
			}
			pair = container.NewThemeOverride(grid, smallChipTheme{Theme: base})
		}
	}

	var right fyne.CanvasObject = spacer(0)
	if notesOn {
		notesBtn = widget.NewButtonWithIcon("", iconNoteBubble, func() {
			// Tapping the bubble while already in notes goes back to scripture, so
			// the control is a way out as well as a way in — otherwise the only exit
			// is a segment that looks unrelated to it.
			//
			// BUT ONLY WHEN THE LIST IS ACTUALLY ON SCREEN. On desktop the mode
			// survives opening a note (openNote sets IsSearching=false and the
			// reading pane takes over while NotesMode stays true — the row's
			// highlight honestly says which mode the pane belongs to), so the
			// mode alone cannot decide. Deciding on it alone made the tap a
			// silent exit: the reader pressed the lit Notes bubble expecting
			// the list back and got a mode flip to keyword instead — nothing

			// mimic/desktop desync). Mobile always shows the list whenever the
			// row is visible (the row lives inside the Search tab), so there
			// the toggle-out is unconditional, as before.
			m := modeNotes
			if searchModeOf(state) == modeNotes && (state.surfaceSearch != nil || state.IsSearching) {
				m = modeKeyword
			}
			apply(m)
			onSelect(m)
		})
		right = notesBtn
	}

	apply(searchModeOf(state))
	return container.NewBorder(nil, nil, nil, right, pair)
}

func searchResultRow(state *AppState, verse Verse, terms []string, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(fmt.Sprintf("%s %d:%d", verse.BookName, verse.Chapter, verse.Verse), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 18

	// Cards are one-snippet UI: flatten authored poem lines here (the reading
	// pane and text shares keep them — this is presentation, not content).
	cardText := strings.Join(strings.Fields(verse.Text), " ")
	segs := termHighlightSegments(cardText, terms, colorNameVerseText, colorNameHighlightHi)
	text := widget.NewRichText(segs...)
	text.Wrapping = fyne.TextWrapWord

	// The whole card is one tap target — reference, verse text, and surrounding
	// padding — not just the reference heading.
	inner := container.NewPadded(container.NewVBox(ref, text))
	card := newSearchResultCard(state, verse, inner, pal)

	return container.NewVBox(card, widget.NewSeparator())
}

// searchResultCard makes an entire result row tappable. Previously only the
// small "Book C:V" heading opened the verse, which is an awkward target —
// especially on touch, where the rest of the row looks tappable but isn't.
// Tapping anywhere on the card jumps to that verse; on desktop it also shows a
// pointer cursor and a faint hover wash so the row reads as clickable.
type searchResultCard struct {
	widget.BaseWidget
	state   *AppState
	verse   Verse
	content fyne.CanvasObject
	hoverBg color.NRGBA
	bg      *canvas.Rectangle
	// onTap overrides the default "open this verse". The Notes mode reuses this
	// card so a note row is the same tap target, the same hover, and the same
	// shape as the search hits it sits beside.
	onTap func()
}

func newSearchResultCard(state *AppState, verse Verse, content fyne.CanvasObject, pal palette) *searchResultCard {
	c := &searchResultCard{state: state, verse: verse, content: content, hoverBg: pal.SurfaceAlt}
	c.ExtendBaseWidget(c)
	return c
}

func (c *searchResultCard) CreateRenderer() fyne.WidgetRenderer {
	c.bg = canvas.NewRectangle(color.Transparent)
	c.bg.CornerRadius = 8
	return widget.NewSimpleRenderer(container.NewStack(c.bg, c.content))
}

func (c *searchResultCard) Tapped(*fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
		return
	}
	openSearchResult(c.state, c.verse)
}

func (c *searchResultCard) MouseIn(*desktop.MouseEvent) {
	if c.bg != nil {
		c.bg.FillColor = c.hoverBg
		c.bg.Refresh()
	}
}

func (c *searchResultCard) MouseMoved(*desktop.MouseEvent) {}

func (c *searchResultCard) MouseOut() {
	if c.bg != nil {
		c.bg.FillColor = color.Transparent
		c.bg.Refresh()
	}
}

func (c *searchResultCard) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

var (
	_ fyne.Tappable      = (*searchResultCard)(nil)
	_ desktop.Hoverable  = (*searchResultCard)(nil)
	_ desktop.Cursorable = (*searchResultCard)(nil)
)

type matchRange struct {
	start int
	end   int
}

// termHighlightSegments splits text into RichText segments, emphasising every
// occurrence of the search terms. Matching is case-insensitive.
func termHighlightSegments(text string, terms []string, base, highlight fyne.ThemeColorName) []widget.RichTextSegment {
	ranges := matchRanges(text, terms)
	if len(ranges) == 0 {
		return []widget.RichTextSegment{resultSegment(text, base, false)}
	}

	segs := make([]widget.RichTextSegment, 0, len(ranges)*2+1)
	pos := 0
	for _, r := range ranges {
		if r.start > pos {
			segs = append(segs, resultSegment(text[pos:r.start], base, false))
		}
		segs = append(segs, resultSegment(text[r.start:r.end], highlight, true))
		pos = r.end
	}
	if pos < len(text) {
		segs = append(segs, resultSegment(text[pos:], base, false))
	}
	return segs
}

// matchRanges returns merged, ordered byte ranges where any term occurs. It
// bails out when lowercasing CHANGES the byte length, which is what would put
// the offsets out of step with the original. That is rarer than "multi-byte":
// the curly quotes and apostrophes the decoders emit lowercase to the same
// length, so ordinary scripture text highlights normally.
func matchRanges(text string, terms []string) []matchRange {
	lower := strings.ToLower(text)
	if len(lower) != len(text) {
		return nil
	}

	var ranges []matchRange
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if len([]rune(term)) < 2 {
			continue
		}
		from := 0
		for {
			i := strings.Index(lower[from:], term)
			if i < 0 {
				break
			}
			start := from + i
			ranges = append(ranges, matchRange{start: start, end: start + len(term)})
			from = start + len(term)
		}
	}
	if len(ranges) == 0 {
		return nil
	}

	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.start <= last.end {
			if r.end > last.end {
				last.end = r.end
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

func resultSegment(s string, colorName fyne.ThemeColorName, bold bool) widget.RichTextSegment {
	return &widget.TextSegment{
		Text: s,
		Style: widget.RichTextStyle{
			Inline:    true,
			SizeName:  theme.SizeNameText,
			ColorName: colorName,
			TextStyle: fyne.TextStyle{Bold: bold},
		},
	}
}
