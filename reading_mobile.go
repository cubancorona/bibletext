//go:build android

package holybible

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// afterRebuild is a no-op on Android (no native text overlay to re-pin).
func afterRebuild(*AppState) {}

// buildReadingViewMobile is the mobile reading pane.
//
// Each paragraph is rendered as one widget.RichText (verses flow inline with
// superscript verse numbers, exactly like the desktop chapterText), then wrapped
// in a selectableParagraph widget that detects long-press and shows a context
// menu with the standard reading actions — Copy / Look Up / Share — much like
// long-pressing a paragraph in Mail.
//
// We do NOT use widget.Entry per-paragraph or chapterText here: on iOS those
// claim a huge hit area (Entry sized to its full content) and intercept taps
// destined for the bottom tab bar, plus they pop the soft keyboard on touch.
// RichText is not a Tappable, so plain taps fall through to the parent scroll
// (preserving native iOS scroll) and only long-press triggers our menu.
func buildReadingViewMobile(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	var content fyne.CanvasObject
	if len(verses) == 0 {
		msg := widget.NewLabel("No verses are available for this chapter yet.")
		msg.Wrapping = fyne.TextWrapWord
		content = msg
	} else {
		content = buildChapterSelectable(state, verses)
	}

	scroll := container.NewVScroll(container.NewPadded(content))
	paper := surface(scroll, pal.Surface, pal.Border, fyne.Size{})

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(bar)
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeader(state, chapterNumbers))

	return container.NewPadded(container.NewBorder(top, nil, nil, nil, paper))
}

// buildChapterSelectable produces one selectableParagraph per paragraph
// (verses flowing inline inside it), separated by visual spacers.
func buildChapterSelectable(state *AppState, verses []Verse) fyne.CanvasObject {
	paragraphs := groupVersesIntoParagraphs(verses)
	rows := make([]fyne.CanvasObject, 0, len(paragraphs)*2)
	for i, para := range paragraphs {
		rows = append(rows, newSelectableParagraph(state, para))
		if i < len(paragraphs)-1 {
			rows = append(rows, spacer(10))
		}
	}
	return container.NewVBox(rows...)
}

// longPressDuration is how long the user must hold a paragraph before the
// context menu appears. iOS standard is ~500 ms.
const longPressDuration = 500 * time.Millisecond

// selectableParagraph renders one paragraph as inline RichText and intercepts
// long-press to show a context menu. Short taps and drags pass through so the
// parent VScroll keeps native scrolling.
type selectableParagraph struct {
	widget.BaseWidget
	state  *AppState
	verses []Verse
	rt     *widget.RichText

	pressTimer *time.Timer
	pressPos   fyne.Position
}

func newSelectableParagraph(state *AppState, verses []Verse) *selectableParagraph {
	sp := &selectableParagraph{state: state, verses: verses}
	sp.ExtendBaseWidget(sp)

	segs := make([]widget.RichTextSegment, 0, len(verses)*3)
	verseNumStyle := widget.RichTextStyle{
		Inline:    true,
		ColorName: colorNameVerseNumber,
		SizeName:  theme.SizeNameCaptionText,
		TextStyle: fyne.TextStyle{Bold: true},
	}
	for i, v := range verses {
		if i > 0 {
			segs = append(segs, &widget.TextSegment{
				Text:  " ",
				Style: widget.RichTextStyle{Inline: true, ColorName: colorNameVerseText},
			})
		}
		segs = append(segs, &widget.TextSegment{
			Text:  superscriptNumber(v.Verse) + " ",
			Style: verseNumStyle,
		})
		bodyColor := colorNameVerseText
		bodyStyle := fyne.TextStyle{}
		if isVerseHighlighted(state, v) {
			bodyColor = colorNameHighlightHi
			bodyStyle = fyne.TextStyle{Bold: true}
		}
		segs = append(segs, &widget.TextSegment{
			Text: strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")),
			Style: widget.RichTextStyle{
				Inline: true, ColorName: bodyColor, TextStyle: bodyStyle,
			},
		})
	}
	rt := widget.NewRichText(segs...)
	rt.Wrapping = fyne.TextWrapWord
	sp.rt = rt
	return sp
}

func (s *selectableParagraph) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(s.rt)
}

// TappedSecondary opens the menu on right-click (desktop drivers / tests).
func (s *selectableParagraph) TappedSecondary(ev *fyne.PointEvent) {
	s.showMenu(ev.AbsolutePosition)
}

// Tapped is a no-op so a plain tap doesn't open the menu and doesn't take
// focus — scrolling stays a native gesture handled by the parent scroll.
func (s *selectableParagraph) Tapped(*fyne.PointEvent) {}

// TouchDown starts a one-shot long-press timer; if the timer fires before
// TouchUp/TouchCancel, the context menu appears at the touch position.
func (s *selectableParagraph) TouchDown(ev *mobile.TouchEvent) {
	s.pressPos = ev.Position
	if s.pressTimer != nil {
		s.pressTimer.Stop()
	}
	s.pressTimer = time.AfterFunc(longPressDuration, func() {
		fyne.Do(func() { s.showMenu(s.absoluteTouchPos()) })
	})
}

// TouchUp / TouchCancel cancel a pending long-press, so a quick tap or a
// drag (which the parent scroll consumes) won't summon the menu.
func (s *selectableParagraph) TouchUp(*mobile.TouchEvent)     { s.cancelTimer() }
func (s *selectableParagraph) TouchCancel(*mobile.TouchEvent) { s.cancelTimer() }

func (s *selectableParagraph) cancelTimer() {
	if s.pressTimer != nil {
		s.pressTimer.Stop()
		s.pressTimer = nil
	}
}

// absoluteTouchPos projects the widget-local touch position into canvas
// coordinates so the popover can anchor where the finger landed.
func (s *selectableParagraph) absoluteTouchPos() fyne.Position {
	base := fyne.CurrentApp().Driver().AbsolutePositionForObject(s)
	return base.Add(s.pressPos)
}

// touchedVerse returns the verse closest to the touch Y inside the paragraph.
// We slice the paragraph's height into len(verses) equal bands as a cheap
// approximation. Verses tend to be similar lengths within a paragraph, so the
// resulting "which verse did I press?" guess is usually right — and the menu
// always also offers paragraph- and chapter-wide actions if the guess is off.
func (s *selectableParagraph) touchedVerse() Verse {
	if len(s.verses) == 1 {
		return s.verses[0]
	}
	h := s.Size().Height
	if h <= 0 {
		return s.verses[0]
	}
	band := h / float32(len(s.verses))
	idx := int(s.pressPos.Y / band)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.verses) {
		idx = len(s.verses) - 1
	}
	return s.verses[idx]
}

// showMenu opens the standard reading-action popover. The "guessed" verse from
// touchedVerse() drives Copy-verse / Copy-with-reference; broader actions
// (Copy paragraph / Copy chapter) are unambiguous and always available.
func (s *selectableParagraph) showMenu(at fyne.Position) {
	if s.state == nil || s.state.window == nil {
		return
	}
	cnv := s.state.window.Canvas()
	v := s.touchedVerse()
	verseRef := fmt.Sprintf("%s %d:%d", v.BookName, v.Chapter, v.Verse)
	verseBody := strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " "))

	paraText := s.plainText()
	paraRef := fmt.Sprintf("%s %d:%d-%d",
		s.verses[0].BookName,
		s.verses[0].Chapter,
		s.verses[0].Verse,
		s.verses[len(s.verses)-1].Verse,
	)

	clip := s.state.window.Clipboard()
	set := func(text string) {
		if clip != nil {
			clip.SetContent(text)
		}
	}

	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Copy verse ("+verseRef+")", func() { set(verseBody) }),
		fyne.NewMenuItem("Copy verse with reference", func() { set(verseBody + " — " + verseRef) }),
		fyne.NewMenuItem("Copy paragraph ("+paraRef+")", func() { set(paraText) }),
		fyne.NewMenuItem("Copy chapter", func() { copyChapter(s.state) }),
		fyne.NewMenuItem("Look up", func() { openLookup(verseRef + " " + verseBody) }),
		fyne.NewMenuItem("Share…", func() { openShare(verseRef, verseBody) }),
	)
	widget.ShowPopUpMenuAtPosition(menu, cnv, at)
}

// plainText returns the paragraph's verses joined as one space-separated
// string, with superscript verse numbers stripped — i.e. the clean text the
// user expects when they "Copy paragraph".
func (s *selectableParagraph) plainText() string {
	var b strings.Builder
	for i, v := range s.verses {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
	}
	return b.String()
}

// openLookup pops Safari with a search for the query. Native dictionary
// lookup (UIReferenceLibraryViewController) would require CGO; Safari is the
// simplest universal alternative.
func openLookup(query string) {
	u, _ := url.Parse("https://www.google.com/search?q=" + url.QueryEscape(query))
	_ = fyne.CurrentApp().OpenURL(u)
}

// openShare pops the iOS Mail composer pre-filled with the verse. Without CGO
// we can't show UIActivityViewController; mailto: is universal and the user
// can route the text from there to any sharing app.
func openShare(ref, body string) {
	u, _ := url.Parse(fmt.Sprintf(
		"mailto:?subject=%s&body=%s",
		url.QueryEscape(ref),
		url.QueryEscape(body+"\n\n— "+ref),
	))
	_ = fyne.CurrentApp().OpenURL(u)
}

// Interface assertions — make sure long-press dispatch keeps working as we
// refactor.
var (
	_ fyne.Tappable          = (*selectableParagraph)(nil)
	_ fyne.SecondaryTappable = (*selectableParagraph)(nil)
	_ mobile.Touchable       = (*selectableParagraph)(nil)
)
