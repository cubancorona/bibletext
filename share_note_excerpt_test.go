package bibletext

// The selected words on the Add a note sheet (share_note_excerpt.go): what they
// say, how many rows they take, that they never widen the card, that the iOS
// field's slot sits below them, and that they come back with a light/dark
// reopen. Each sheet is opened for real, through promptShareNote, on the
// desktop path and on the phone path (the phone device from
// chapter_header_cover_test.go), and the phone path's timers are held and run
// here rather than on a timer's goroutine (noteSheetAfter).

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// psalm23NoteState is Psalm 23 as an edition that marks the divine name stores
// it: the publisher's "Lord", with a small-capitals span over it, which the
// page draws as "Lᴏʀᴅ".
func psalm23NoteState() *AppState {
	verses := []Verse{
		{Verse: 1, Text: "The Lord is my shepherd; I shall not want.", SmallCaps: []TextSpan{{Start: 4, End: 8}}},
		{Verse: 2, Text: "He maketh me to lie down in green pastures: he leadeth me beside the still waters."},
		{Verse: 3, Text: "He restoreth my soul: he leadeth me in the paths of righteousness for his name's sake."},
		{Verse: 4, Text: "Yea, though I walk through the valley of the shadow of death, I will fear no evil: for thou art with me; thy rod and thy staff they comfort me."},
		{Verse: 5, Text: "Thou preparest a table before me in the presence of mine enemies: thou anointest my head with oil; my cup runneth over."},
		{Verse: 6, Text: "Surely goodness and mercy shall follow me all the days of my life: and I will dwell in the house of the Lord for ever.",
			SmallCaps: []TextSpan{{Start: 104, End: 108}}},
	}
	for i := range verses {
		verses[i].BookName, verses[i].Book, verses[i].Chapter = "Psalms", "Psalms", 23
	}
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {23: verses}}}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
}

// drawnSelection is verses lo..hi of the state's chapter as a selection hands
// them over: each verse drawn as the page draws it, preceded by its number.
func drawnSelection(t *testing.T, st *AppState, lo, hi int) string {
	t.Helper()
	var parts []string
	for _, v := range st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter) {
		if v.Verse >= lo && v.Verse <= hi {
			parts = append(parts, fmt.Sprintf("%d %s", v.Verse, drawnVerse(t, v)))
		}
	}
	return strings.Join(parts, " ")
}

// noteSheetWindow opens a window of the given size in the reader's own theme,
// on a phone when phone is set, holding st.
func noteSheetWindow(t *testing.T, st *AppState, phone bool, w, h float32) fyne.Window {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	if phone {
		fyne.SetCurrentApp(phoneTestApp{app})
	}
	win := app.NewWindow("note")
	t.Cleanup(win.Close)
	win.Resize(fyne.NewSize(w, h))
	st.window = win
	st.theme = th
	return win
}

// holdNoteSheetTimers keeps what the composer schedules (the slot's second
// push, the phone sheet's watchdog) for the test to run on its own goroutine.
func holdNoteSheetTimers(t *testing.T) *[]func() {
	t.Helper()
	held := &[]func(){}
	prev := noteSheetAfter
	noteSheetAfter = func(_ time.Duration, f func()) { *held = append(*held, f) }
	t.Cleanup(func() { noteSheetAfter = prev })
	return held
}

// noteFrame is a rect the native field was told to take.
type noteFrame struct {
	pos fyne.Position
	sz  fyne.Size
}

// nativeNoteField makes this platform float the native field, as iOS does,
// and records every frame it would be given while its slot is on win's
// canvas. The pushes a layout makes before the popup is on the canvas are not
// recorded: there is no position to give them yet, and on a device they reach
// the main queue ahead of the view the sheet then creates.
func nativeNoteField(t *testing.T, win fyne.Window) *[]noteFrame {
	t.Helper()
	frames := &[]noteFrame{}
	prevN, prevF := noteEntryNative, noteEntryFrameTo
	noteEntryNative = func() bool { return true }
	noteEntryFrameTo = func(o fyne.CanvasObject) {
		showing := false
		for _, ov := range win.Canvas().Overlays().List() {
			walkTree(ov, func(n fyne.CanvasObject) { showing = showing || n == o })
		}
		if !showing {
			return
		}
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
		*frames = append(*frames, noteFrame{pos, o.Size()})
	}
	t.Cleanup(func() { noteEntryNative, noteEntryFrameTo = prevN, prevF })
	return frames
}

// noteSheetExcerpt finds the excerpt on the sheet, laid out as it stands.
func noteSheetExcerpt(t *testing.T, p *widget.PopUp) (*noteExcerpt, *noteExcerptRenderer) {
	t.Helper()
	var ex *noteExcerpt
	if p != nil {
		walkTree(p, func(o fyne.CanvasObject) {
			if e, ok := o.(*noteExcerpt); ok && ex == nil {
				ex = e
			}
		})
	}
	if ex == nil {
		t.Fatalf("the sheet shows no excerpt; texts %v", sheetTexts(p))
	}
	return ex, test.WidgetRenderer(ex).(*noteExcerptRenderer)
}

// THE EXCERPT IS THE SHARE'S QUOTE. A selection of two verses as the page hands
// it over, verse numbers and all, with the divine name drawn in small
// capitals: the sheet shows the words prepareShareQuote makes of it, on the
// desktop and on a phone, the name still in small capitals, no verse number,
// drawn in the reading face (the chrome face has no small capitals) in the
// muted ink. A verse number selected on its own is no words at all: the share
// quotes nothing for it (TestAVerseNumberSelectedOnItsOwnQuotesNothing), so
// the sheet shows the reference alone. Mutations guarded: the raw selection
// shown (the numbers stay); the words sent through outboundText (the name in
// capitals); the rows drawn in the chrome face; a lone verse number quoted.
func TestTheNoteSheetShowsTheSharesQuote(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phone bool
		w, h  float32
	}{
		{"desktop", false, 900, 760},
		{"tablet", true, 768, 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, tc.phone, tc.w, tc.h)
			holdNoteSheetTimers(t)
			sel := drawnSelection(t, st, 1, 2)
			span := selSpanFromNative(1, 2)
			want, cite, _, _ := prepareShareQuote(st, sel, span)
			// The premise: the selection carries what the sheet must not show,
			// and the share's quote carries what it must.
			if !strings.HasPrefix(sel, "1 The Lᴏʀᴅ") || !strings.Contains(sel, " 2 He maketh") {
				t.Fatalf("control: the selection must carry the verse numbers and the drawn name: %q", sel)
			}
			if !strings.Contains(want, "Lᴏʀᴅ") || strings.Contains(want, "2 He") || cite != "Psalms 23:1–2" {
				t.Fatalf("control: the share's quote is %q (%s)", want, cite)
			}

			promptShareNote(st, sel, span)
			p := topPopup(t, win)
			test.WidgetRenderer(p).Layout(p.Size())
			ex, r := noteSheetExcerpt(t, p)
			got := strings.Join(r.drawn(), " ")
			if got != want {
				t.Errorf("the excerpt reads %q, want the share's quote %q", got, want)
			}
			if !strings.Contains(got, "Lᴏʀᴅ") || strings.Contains(got, "LORD") || strings.Contains(got, "Lord") {
				t.Errorf("the divine name must be drawn in small capitals as the page draws it: %q", got)
			}
			if strings.HasPrefix(got, "1 ") || strings.Contains(got, " 2 ") {
				t.Errorf("the excerpt carries a verse number: %q", got)
			}
			if !sheetHas(p, cite) {
				t.Errorf("the reference must be the share's citation %q; texts %v", cite, sheetTexts(p))
			}
			muted := st.pal().TextMuted
			for i, o := range r.Objects() {
				row, ok := o.(*canvas.Text)
				if !ok {
					t.Fatalf("row %d is a %T", i, o)
				}
				if row.FontSource != styledPaneFont() {
					t.Errorf("row %d is not set in the reading face, which alone has the small capitals", i)
				}
				if row.Color != muted {
					t.Errorf("row %d is drawn in %v, want the muted ink %v", i, row.Color, muted)
				}
				if typed := theme.Size(theme.SizeNameText); row.TextSize >= typed {
					t.Errorf("row %d is %vpt: the excerpt is a reminder, smaller than the %vpt note", i, row.TextSize, typed)
				}
			}
			if ex.Size().Width <= 0 {
				t.Error("the excerpt was never laid out")
			}
			p.Hide()

			// A verse number on its own: the reference to its verse, no words.
			promptShareNote(st, "2", selSpanFromNative(2, 2))
			p = topPopup(t, win)
			if !sheetHas(p, "Psalms 23:2") {
				t.Fatalf("control: a verse number selected alone must cite its verse; texts %v", sheetTexts(p))
			}
			if ex, r := noteSheetExcerpt(t, p); ex.Visible() {
				t.Errorf("a verse number selected alone shows as the selected words: %q", r.drawn())
			}
		})
	}
}

// A HEADING NEVER REACHES THE EXCERPT, because a heading never reaches the
// share's quote. A selection led by a publisher's heading shows the verses'
// words without the heading's; a heading selected on its own quotes nothing,
// so the sheet shows the reference to the verse beneath it and no excerpt at
// all, not an empty one holding a gap open. That holds for a heading in the
// middle of a chapter and for one at its top, which has no verse above it and
// comes with no span (TestAHeadingAtTheTopOfAChapterSelectedOnItsOwnQuotesNothing).
// Mutations guarded: the excerpt left showing, empty, for a heading selected
// alone; the top heading's words quoted.
func TestAHeadingNeverReachesTheNoteExcerpt(t *testing.T) {
	st := headingChapterState()
	win := noteSheetWindow(t, st, false, 900, 760)

	promptShareNote(st, headingText+" "+headingUnderVerses, selSpanFromNative(43, 47))
	p := topPopup(t, win)
	ex, r := noteSheetExcerpt(t, p)
	if got := strings.Join(r.drawn(), " "); !ex.Visible() || !strings.HasPrefix(got, "While Peter was still speaking") ||
		strings.Contains(got, "Gentiles") {
		t.Errorf("a selection led by a heading shows %q: the verses' words, not the heading's", got)
	}
	p.Hide()

	promptShareNote(st, headingText, selSpanFromNative(43, 43))
	p = topPopup(t, win)
	if !sheetHas(p, "Acts 10:44") {
		t.Fatalf("control: a heading selected alone must cite the verse beneath it; texts %v", sheetTexts(p))
	}
	ex, r = noteSheetExcerpt(t, p)
	if ex.Visible() {
		t.Errorf("a heading selected alone quotes nothing, but the sheet shows an excerpt of %d rows", len(r.drawn()))
	}
	p.Hide()

	// A heading at the top of a chapter, as the native panes report it.
	top := openingHeadingState()
	top.window, top.theme = st.window, st.theme
	for _, span := range []selSpan{{}, selSpanFromNative(0, 1)} {
		promptShareNote(top, "The Making of All Things", span)
		p = topPopup(t, win)
		if !sheetHas(p, "Genesis 1:1") {
			t.Fatalf("span %+v: a chapter's opening heading selected alone must cite the verse beneath it; texts %v",
				span, sheetTexts(p))
		}
		if ex, r := noteSheetExcerpt(t, p); ex.Visible() {
			t.Errorf("span %+v: a chapter's opening heading shows as the selected words: %q", span, r.drawn())
		}
		p.Hide()
	}
}

// A LONG SELECTION IS CUT, WITH AN ELLIPSIS, AFTER THE ROWS IT MAY TAKE: three
// on the desktop and a phone in portrait, one on a phone in landscape, where
// every row is a row of the note field pushed under the keyboard. A desktop
// window as short as a phone in landscape keeps three: no soft keyboard covers
// its sheet. The rows it does draw are the start of the share's quote, and none
// is wider than the excerpt. Mutations guarded: no row cap; a cut row without
// its ellipsis; the short budget dropped (three rows on a phone in landscape);
// the short budget given to a short desktop window.
func TestTheNoteSheetCutsALongSelection(t *testing.T) {
	for _, tc := range []struct {
		name  string
		phone bool
		w, h  float32
		rows  int
	}{
		{"desktop", false, 900, 760, noteExcerptMaxLines},
		{"desktop, a short window", false, 900, 400, noteExcerptMaxLines},
		{"phone portrait", true, 375, 667, noteExcerptMaxLines},
		{"phone landscape", true, 852, 393, noteExcerptShortMaxLines},
		{"small phone landscape", true, 667, 375, noteExcerptShortMaxLines},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, tc.phone, tc.w, tc.h)
			holdNoteSheetTimers(t)
			sel := drawnSelection(t, st, 1, 6)
			span := selSpanFromNative(1, 6)
			want, _, _, _ := prepareShareQuote(st, sel, span)

			promptShareNote(st, sel, span)
			p := topPopup(t, win)
			test.WidgetRenderer(p).Layout(p.Size())
			ex, r := noteSheetExcerpt(t, p)
			rows := r.drawn()
			if len(rows) != tc.rows {
				t.Fatalf("the excerpt takes %d rows, want %d: %q", len(rows), tc.rows, rows)
			}
			last := rows[len(rows)-1]
			if !strings.HasSuffix(last, noteExcerptEllipsis) {
				t.Errorf("the cut row must end with an ellipsis: %q", last)
			}
			shown := strings.TrimSuffix(strings.Join(rows, " "), noteExcerptEllipsis)
			if !strings.HasPrefix(want, shown) || len(shown) >= len(want) {
				t.Errorf("the rows must be the start of the share's quote:\n rows  %q\n quote %q", shown, want)
			}
			for i, w := range r.rowWidths() {
				if w > ex.Size().Width+0.5 {
					t.Errorf("row %d is %vpt wide in a %vpt excerpt", i, w, ex.Size().Width)
				}
			}
			if h := ex.Size().Height; h != float32(tc.rows)*r.rowH {
				t.Errorf("the excerpt is %vpt tall for %d rows of %vpt", h, tc.rows, r.rowH)
			}
		})
	}
}

// THE DESKTOP CARD IS AS TALL AS WHAT IT HOLDS. Its height is its content's
// minimum, and the excerpt and the counter wrap, so the height must be read at
// the width they are drawn at; read from the layout Show makes at the form's
// narrow minimum, it left the card a counter's row too tall, with the excerpt
// adding its own. At a wide window and a narrow one, with a selection that
// fills the excerpt. Mutation guarded: one Resize pass instead of two.
func TestTheDesktopNoteCardIsAsTallAsItsContent(t *testing.T) {
	for _, w := range []float32{900, 375} {
		t.Run(fmt.Sprint(w), func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, false, w, 760)
			promptShareNote(st, drawnSelection(t, st, 1, 6), selSpanFromNative(1, 6))
			p := topPopup(t, win)
			if _, r := noteSheetExcerpt(t, p); len(r.drawn()) != noteExcerptMaxLines {
				t.Fatalf("control: the excerpt must fill its rows: %q", r.drawn())
			}
			if got, min := p.Content.Size().Height, p.Content.MinSize().Height; got > min+0.5 || got < min-0.5 {
				t.Errorf("the card is %vpt tall for %vpt of content", got, min)
			}
			d := fyne.CurrentApp().Driver()
			share := findTreeButton(p.Content, "Share")
			if share == nil {
				t.Fatal("no Share button on the sheet")
			}
			sb := d.AbsolutePositionForObject(share).Y + share.Size().Height
			cb := d.AbsolutePositionForObject(p.Content).Y + p.Content.Size().Height
			if sb > cb {
				t.Errorf("Share ends at y=%v, below the card's bottom at %v", sb, cb)
			}
		})
	}
}

// The wrapping itself, measured one unit per letter so the rows can be written
// out: words stay whole where they fit, a word wider than a row is broken
// between letters, the rows stop at the cap, and the cut is marked on the last
// row, which gives up its trailing punctuation and then whole words to make
// room. Text that fits is drawn as it is, with no ellipsis.
func TestNoteExcerptLines(t *testing.T) {
	perLetter := func(s string) float32 { return float32(len([]rune(s))) }
	for _, tc := range []struct {
		text  string
		width float32
		max   int
		want  []string
	}{
		{"aaaa bbbb", 10, 3, []string{"aaaa bbbb"}},
		{"aaaa bbbb cccc", 10, 3, []string{"aaaa bbbb", "cccc"}},
		{"aaaa bbbb cccc dddd eeee", 10, 2, []string{"aaaa bbbb", "cccc dddd…"}},
		{"aaaa bbbb, cccc dddd", 10, 1, []string{"aaaa bbbb…"}},
		{"aaa bbb, ccccccc", 9, 1, []string{"aaa bbb…"}},
		{"aaaa bbbbb cccc", 10, 1, []string{"aaaa…"}},
		{"xxxxxxxxxxxxxxxxxxxxxxxxx", 10, 3, []string{"xxxxxxxxxx", "xxxxxxxxxx", "xxxxx"}},
		{"xxxxxxxxxxxxxxxxxxxxxxxxx yy", 10, 2, []string{"xxxxxxxxxx", "xxxxxxxxx…"}},
		{"ab xxxxxxxxxxxxxx", 10, 3, []string{"ab", "xxxxxxxxxx", "xxxx"}},
		{"Lᴏʀᴅ Lᴏʀᴅ", 9, 3, []string{"Lᴏʀᴅ Lᴏʀᴅ"}},
	} {
		got := noteExcerptLines(tc.text, tc.width, tc.max, perLetter)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("noteExcerptLines(%q, %v, %d) = %q, want %q", tc.text, tc.width, tc.max, got, tc.want)
		}
		for _, row := range got {
			if perLetter(row) > tc.width {
				t.Errorf("row %q is wider than %v", row, tc.width)
			}
		}
	}
}

// NOTHING IN THE SHEET MAKES THE CARD WIDER THAN THE CANVAS, the excerpt
// included, on a 375-wide canvas with a selection that is one unbroken word
// far wider than the screen: the card stays inside the canvas, Share with it,
// and the word is broken across rows none of which runs past the excerpt's
// edge. Mutations guarded: the excerpt asking for the width of its words
// unwrapped; the word left whole on a row of its own.
func TestTheNoteSheetIsNoWiderThanTheCanvas(t *testing.T) {
	word := strings.Repeat("Mahershalalhashbaz", 12)
	for _, tc := range []struct {
		name  string
		phone bool
	}{
		{"phone", true},
		{"desktop", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23NoteState()
			st.Bible.Verses["Psalms"][23][2].Text = word
			win := noteSheetWindow(t, st, tc.phone, 375, 667)
			holdNoteSheetTimers(t)
			sel := drawnSelection(t, st, 3, 3)

			promptShareNote(st, sel, selSpanFromNative(3, 3))
			p := topPopup(t, win)
			test.WidgetRenderer(p).Layout(p.Size())
			cw := win.Canvas().Size().Width
			if wd := p.Content.Size().Width; wd > cw {
				t.Errorf("the card is %vpt wide on a %vpt canvas", wd, cw)
			}
			share := findTreeButton(p.Content, "Share")
			if share == nil {
				t.Fatal("no Share button on the sheet")
			}
			if right := fyne.CurrentApp().Driver().AbsolutePositionForObject(share).X + share.Size().Width; right > cw {
				t.Errorf("Share ends at x=%v on a %vpt canvas", right, cw)
			}
			ex, r := noteSheetExcerpt(t, p)
			if r.measure(word).Width <= cw {
				t.Fatalf("control: the word must be wider than the canvas (%vpt)", r.measure(word).Width)
			}
			rows := r.drawn()
			if len(rows) < 2 {
				t.Errorf("the word must be broken across rows: %q", rows)
			}
			if ex.Size().Width <= 0 || ex.Size().Width > cw {
				t.Errorf("the excerpt is %vpt wide on a %vpt canvas", ex.Size().Width, cw)
			}
			for i, w := range r.rowWidths() {
				if w > ex.Size().Width+0.5 {
					t.Errorf("row %d is %vpt wide in a %vpt excerpt", i, w, ex.Size().Width)
				}
			}
			if !strings.HasSuffix(rows[len(rows)-1], noteExcerptEllipsis) {
				t.Errorf("a word longer than the rows allow is cut too: %q", rows)
			}
		})
	}
}

// THE FIELD'S SLOT SITS BELOW THE EXCERPT, so the native text view iOS parks
// over the slot cannot cover the words. The field is made native here as on
// iOS, and every frame it would be given while the sheet is showing is read:
// at open, after a relayout of the kind the phone canvas makes when the
// keyboard's inset changes (a Refresh of every popup), and at the slot's
// second push. In portrait and landscape, each with its own row budget; and
// the slot's distance below the excerpt's last row is the same whether the
// excerpt takes one row or three, so it moves down by exactly the excerpt's
// height.
//
// And on a 320pt canvas, the width of iPad Slide Over and of a third of an
// 11-inch iPad in Split View, where the form is narrower than the width an
// unsized excerpt wraps for (noteExcerptGuessWidth), with a verse that takes a
// row more at the form's width than at that one. Every other case wraps to as
// many rows at either width, so only this one tells a height read at the width
// the excerpt is drawn at from a height read at some other width. The bottom
// is measured from the rows drawn, never from the excerpt's size, which is
// derived from the height being tested.
//
// Mutations guarded: the excerpt reporting the height of one row whatever it
// draws; the excerpt placed after the slot; the excerpt's height taken from
// its rows at the guess width rather than at the width it was drawn at.
func TestTheNoteFieldSlotSitsBelowTheExcerpt(t *testing.T) {
	// A synthetic verse that wraps to three rows at a 320pt canvas's form
	// width and to two at the guess width (asserted below as a control).
	const narrowVerse = "The shepherd went out early to the far hills to look for the one sheep that had wandered."
	gaps := map[int]float32{}
	for _, tc := range []struct {
		name   string
		w, h   float32
		lo, hi int
		rows   int
		gap    bool   // measure the slot's distance below the last row
		verse3 string // verse 3's text, when the case needs its own
	}{
		{"portrait, three rows", 393, 852, 1, 6, noteExcerptMaxLines, true, ""},
		{"portrait, one row", 393, 852, 1, 1, 1, true, ""},
		{"small phone portrait, three rows", 320, 568, 1, 6, noteExcerptMaxLines, false, ""},
		{"landscape, the short budget", 852, 393, 1, 6, noteExcerptShortMaxLines, false, ""},
		{"a 320pt iPad window, a row more than at the guess width", 320, 1024, 3, 3, 3, false, narrowVerse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23NoteState()
			if tc.verse3 != "" {
				st.Bible.Verses["Psalms"][23][2].Text = tc.verse3
			}
			win := noteSheetWindow(t, st, true, tc.w, tc.h)
			held := holdNoteSheetTimers(t)
			frames := nativeNoteField(t, win)

			promptShareNote(st, drawnSelection(t, st, tc.lo, tc.hi), selSpanFromNative(tc.lo, tc.hi))
			p := topPopup(t, win)
			ex, r := noteSheetExcerpt(t, p)
			if n := len(r.drawn()); n != tc.rows {
				t.Fatalf("control: the excerpt takes %d rows, want %d", n, tc.rows)
			}
			if tc.verse3 != "" {
				width := func(s string) float32 { return r.measure(s).Width }
				drawn := len(noteExcerptLines(ex.text, ex.Size().Width, ex.maxLines, width))
				guess := len(noteExcerptLines(ex.text, noteExcerptGuessWidth, ex.maxLines, width))
				if ex.Size().Width >= noteExcerptGuessWidth || drawn == guess {
					t.Fatalf("control: the excerpt is %vpt wide and wraps to %d rows there and %d at the %vpt guess width; "+
						"the case needs a form narrower than the guess and a row count that differs",
						ex.Size().Width, drawn, guess, noteExcerptGuessWidth)
				}
			}
			if len(*frames) == 0 {
				t.Fatal("the slot pushed no frame while the sheet was showing")
			}
			top := fyne.CurrentApp().Driver().AbsolutePositionForObject(ex).Y
			bottom := top + float32(len(r.drawn()))*r.rowH
			check := func(when string) {
				t.Helper()
				for i, f := range *frames {
					if f.pos.Y < bottom {
						t.Errorf("%s: frame %d puts the field at y=%v, over the excerpt's rows (%v..%v)", when, i, f.pos.Y, top, bottom)
					}
					if f.pos.X+f.sz.Width > tc.w || f.sz.Height <= 0 {
						t.Errorf("%s: frame %d is %v at %v on a %vpt canvas", when, i, f.sz, f.pos, tc.w)
					}
				}
			}
			check("at open")
			if tc.gap {
				gaps[tc.rows] = (*frames)[len(*frames)-1].pos.Y - bottom
			}

			// The keyboard rises: the phone canvas refreshes every popup.
			n := len(*frames)
			p.Refresh()
			if len(*frames) == n {
				t.Error("control: the relayout pushed no frame")
			}
			check("with the keyboard up")

			// The slot's second push, and one pass of the watchdog.
			for _, f := range append([]func(){}, *held...) {
				f()
			}
			check("at the second push")
		})
	}
	if len(gaps) != 2 {
		t.Fatalf("control: both portrait gaps must be measured, got %v", gaps)
	}
	if gaps[1] != gaps[noteExcerptMaxLines] {
		t.Errorf("the field sits %vpt below a one-row excerpt and %vpt below a three-row one: it must follow the excerpt's height exactly",
			gaps[1], gaps[noteExcerptMaxLines])
	}
}

// THE EXCERPT COMES BACK WITH THE SHEET after a light/dark change: the rebuild
// drains the composer and the reopen builds it again, with the same words, in
// the new palette's muted ink. On the desktop, with the note written so far;
// on a phone, with the field native as on iOS, and the reopened sheet's field
// below the reopened excerpt. Mutations guarded: the excerpt built only on a
// first open (the reopen is not one); the composer registering no reopen.
func TestTheNoteExcerptComesBackWithTheSheet(t *testing.T) {
	sel := "1 In the beginning was the Word, and the Word was with God, and the Word was God. 2 The same was in the beginning with God."
	span := selSpanFromNative(1, 2)
	for _, phone := range []bool{false, true} {
		name := "desktop"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			h := newAppearanceHarness(t, phone)
			var frames *[]noteFrame
			if phone {
				app := fyne.CurrentApp()
				fyne.SetCurrentApp(phoneTestApp{app})
				t.Cleanup(func() { fyne.SetCurrentApp(app) })
				holdNoteSheetTimers(t)
				frames = nativeNoteField(t, h.state.window)
			}
			want, _, _, _ := prepareShareQuote(h.state, sel, span)
			if strings.Contains(want, " 2 ") || !strings.HasPrefix(want, "In the beginning") {
				t.Fatalf("control: the share's quote is %q", want)
			}
			promptShareNote(h.state, sel, span)
			first := h.top()
			_, r1 := noteSheetExcerpt(t, first)
			if got := strings.Join(r1.drawn(), " "); got != want {
				t.Fatalf("control: the excerpt reads %q, want %q", got, want)
			}
			before := h.state.pal().TextMuted
			if !phone {
				var entry *searchKeyEntry
				walkTree(first, func(o fyne.CanvasObject) {
					if e, ok := o.(*searchKeyEntry); ok && entry == nil {
						entry = e
					}
				})
				if entry == nil {
					t.Fatal("control: the composer has no field")
				}
				entry.SetText("Read this before Sunday")
			} else {
				*frames = (*frames)[:0] // from here, only the reopened field's
			}

			h.flip()
			again := h.top()
			if again == nil || again == first || !sheetHas(again, "Add a note") {
				t.Fatalf("the composer must come back, with its excerpt; texts %v", sheetTexts(again))
			}
			after := h.state.pal().TextMuted
			if after == before {
				t.Fatal("control: the two palettes' muted inks must differ")
			}
			ex, r2 := noteSheetExcerpt(t, again)
			// Shown and laid out, not merely present: a hidden excerpt keeps
			// its wrapped rows, so the rows alone cannot tell.
			if !ex.Visible() || ex.Size().Width <= 0 || ex.Size().Height != float32(len(r2.drawn()))*r2.rowH {
				t.Fatalf("the reopened excerpt must be shown at its rows' height; visible=%v size=%v rows=%d of %vpt",
					ex.Visible(), ex.Size(), len(r2.drawn()), r2.rowH)
			}
			if got := strings.Join(r2.drawn(), " "); got != want {
				t.Errorf("the reopened excerpt reads %q, want %q", got, want)
			}
			for i, o := range r2.Objects() {
				if row := o.(*canvas.Text); row.Color != after {
					t.Errorf("row %d came back in %v, want the new palette's muted ink %v", i, row.Color, after)
				}
			}
			if phone {
				// From the rows drawn, not the excerpt's size: the size is
				// derived from the height a field placement would be wrong by.
				bottom := fyne.CurrentApp().Driver().AbsolutePositionForObject(ex).Y + float32(len(r2.drawn()))*r2.rowH
				if len(*frames) == 0 {
					t.Fatal("the reopened sheet's field was given no frame")
				}
				for i, f := range *frames {
					if f.pos.Y < bottom {
						t.Errorf("frame %d puts the reopened field at y=%v, over the excerpt ending at %v", i, f.pos.Y, bottom)
					}
				}
			}
		})
	}
}

// THE PHONE SHEET REFITS WHEN THE CANVAS CHANGES SIZE. Narrowing an iPad's
// Split View or Slide Over window with the composer open changes neither the
// layout class nor the rail, so nothing rebuilds the window, and the toolkit
// only lays a popup out again at the size it was given: the card, the excerpt
// and the slot kept their old width, and the native field was parked past the
// canvas's right edge with Share beyond it. The watchdog now notices the new
// canvas size and sizes the card for it, so the excerpt re-wraps and the
// field follows the slot. The canvas is changed here as the mobile driver
// changes it (a new size, then a Refresh of every popup), and the watchdog's
// pass is run by hand (noteSheetAfter).
//
// The sheet opens with safe insets, as on a device (noteSheetArea), and it
// keeps the top and the foot gap it opened with. One case changes the canvas
// with the keyboard up, which the mobile driver counts as a bottom inset: the
// card still reaches the foot gap it opened with, since the phone sheet is not
// resized for the keyboard and a card fitted to the keyboard's top would stay
// short once it went down. A canvas shorter than the phone-landscape threshold
// gives the excerpt the one row a sheet opened there would take. And Android,
// whose field is the Fyne entry in the same slot, refits the same way.
//
// Mutations guarded: no refit; the refit's height taken from the interactive
// area (the keyboard's top); the refit's height taken from the canvas without
// the top and foot gap; the row budget left as the sheet opened with it; the
// native field told the rects the refit's layout passes through on the way
// (settle); no refit where the field is the Fyne entry.
func TestTheNoteSheetRefitsWhenTheCanvasChanges(t *testing.T) {
	const safeTop, safeFoot, keyboardH = 47, 34, 300
	for _, tc := range []struct {
		name     string
		from, to fyne.Size
		keyboard bool // the keyboard is up when the canvas changes
		rows     int  // the excerpt's rows once refitted
		fyneOnly bool // the field is the Fyne entry, as on Android
	}{
		{"Split View narrowed to a third", fyne.NewSize(590, 820), fyne.NewSize(320, 820), false, noteExcerptMaxLines, false},
		{"Split View widened again", fyne.NewSize(320, 820), fyne.NewSize(590, 820), false, noteExcerptMaxLines, false},
		{"narrowed with the keyboard up", fyne.NewSize(590, 820), fyne.NewSize(320, 820), true, noteExcerptMaxLines, false},
		{"a window made shorter than a phone in landscape", fyne.NewSize(400, 820), fyne.NewSize(400, 440), false, noteExcerptShortMaxLines, false},
		{"Android, a window narrowed", fyne.NewSize(590, 820), fyne.NewSize(320, 820), false, noteExcerptMaxLines, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, tc.from.Width, tc.from.Height)
			held := holdNoteSheetTimers(t)
			frames := &[]noteFrame{}
			if !tc.fyneOnly {
				frames = nativeNoteField(t, win)
			}
			keyboardUp := false
			prevArea := noteSheetArea
			noteSheetArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
				h := c.Size().Height - safeTop - safeFoot
				if keyboardUp {
					h = c.Size().Height - safeTop - keyboardH
				}
				return fyne.NewPos(0, safeTop), fyne.NewSize(c.Size().Width, h)
			}
			t.Cleanup(func() { noteSheetArea = prevArea })
			// One pass of what is held: the watchdog re-arms itself on every
			// pass, so the held list never empties by itself.
			run := func() {
				fs := *held
				*held = nil
				for _, f := range fs {
					f()
				}
			}

			promptShareNote(st, drawnSelection(t, st, 1, 6), selSpanFromNative(1, 6))
			p := topPopup(t, win)
			pad := p.Theme().Size(theme.SizeNameInnerPadding)
			card := func(canvas fyne.Size) fyne.Size {
				return fyne.NewSize(canvas.Width-pad, canvas.Height-safeTop-safeFoot-pad)
			}
			if got, want := p.Content.Size(), card(tc.from); got != want {
				t.Fatalf("control: the card opened at %v on a %v canvas, want %v", got, tc.from, want)
			}
			run()

			// The canvas changes, as the mobile driver changes it.
			keyboardUp = tc.keyboard
			win.Resize(tc.to)
			p.Refresh()
			if tc.from.Width != tc.to.Width && p.Content.Size().Width == card(tc.to).Width {
				t.Fatal("control: the toolkit refitted the card by itself, so this test proves nothing about the refit")
			}
			*frames = (*frames)[:0]
			run() // the watchdog's pass, which refits
			run() // the second pushes the refit's layout armed

			if got, want := p.Content.Size(), card(tc.to); got != want {
				t.Errorf("the card is %v on a %v canvas, want %v: the width of the canvas, from the top it opened at "+
					"to the foot gap it opened with", got, tc.to, want)
			}
			if y := p.Content.Position().Y - pad/2; y != safeTop {
				t.Errorf("the card starts at y=%v, want the top it opened at, %v", y, float32(safeTop))
			}
			d := fyne.CurrentApp().Driver()
			share := findTreeButton(p.Content, "Share")
			if share == nil {
				t.Fatal("no Share button on the sheet")
			}
			if right := d.AbsolutePositionForObject(share).X + share.Size().Width; right > tc.to.Width {
				t.Errorf("Share ends at x=%v on a %vpt canvas", right, tc.to.Width)
			}
			ex, r := noteSheetExcerpt(t, p)
			if n := len(r.drawn()); n != tc.rows {
				t.Errorf("the excerpt takes %d rows on the new canvas, want %d", n, tc.rows)
			}
			if ex.Size().Width > tc.to.Width {
				t.Errorf("the excerpt is %vpt wide on a %vpt canvas", ex.Size().Width, tc.to.Width)
			}
			for i, w := range r.rowWidths() {
				if w > ex.Size().Width+0.5 {
					t.Errorf("row %d is %vpt wide in a %vpt excerpt: it was wrapped for the old width", i, w, ex.Size().Width)
				}
			}
			bottom := d.AbsolutePositionForObject(ex).Y + float32(len(r.drawn()))*r.rowH
			if tc.fyneOnly {
				var entry *searchKeyEntry
				walkTree(p, func(o fyne.CanvasObject) {
					if e, ok := o.(*searchKeyEntry); ok && entry == nil {
						entry = e
					}
				})
				if entry == nil {
					t.Fatal("control: the sheet has no Fyne field")
				}
				at := d.AbsolutePositionForObject(entry)
				if at.X+entry.Size().Width > tc.to.Width || at.Y < bottom {
					t.Errorf("the Fyne field is %v at %v on a %v canvas, below an excerpt ending at %v",
						entry.Size(), at, tc.to, bottom)
				}
				return
			}
			if len(*frames) == 0 {
				t.Fatal("the refit pushed the native field no frame")
			}
			for i, f := range *frames {
				if f.pos.X < 0 || f.pos.X+f.sz.Width > tc.to.Width {
					t.Errorf("frame %d puts the field at x=%v..%v on a %vpt canvas", i, f.pos.X, f.pos.X+f.sz.Width, tc.to.Width)
				}
				if f.pos.Y < bottom {
					t.Errorf("frame %d puts the field at y=%v, over the excerpt ending at %v", i, f.pos.Y, bottom)
				}
			}
		})
	}
}
