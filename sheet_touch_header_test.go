package bibletext

// NO SHEET ON A PHONE OR TABLET STARTS PARTWAY DOWN A HEADER CONTROL.
//
// These lay the real window out as a phone's driver does — inside the safe
// area its insets leave, less the padding of a mobile window — open each
// sheet over it as the reader opens it, and compare where the sheet's box
// begins with the header's controls as they are drawn: the title's letters,
// the translation line's, the Go to chip's outline, the sparkle and the gear.
// The drawn parts are read off a rendering of the header, not taken from
// layout boxes, so a sheet counts as starting partway down a control only
// where pixels of that control show above its edge. The harness and the
// checks that are cheap under the race detector are here; the whole matrix
// is in sheet_touch_header_matrix_test.go.

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// touchScreen is a device's canvas in Fyne units and the safe insets its
// driver reports.
type touchScreen struct {
	name                     string
	w, h                     float32
	top, bottom, left, right float32
}

// touchScreens are the phones and tablets the sheets are measured on. The
// Android phone is a 1080x2410 screen at its default 420 dpi, which the
// driver lays out at three pixels to the unit, with a 63-pixel status bar
// and a 63-pixel gesture navigation bar, or the 126-pixel bar of three-button
// navigation, and the same phone at 2:1 (1080x2160); the
// iPhones' insets are the status bar or Dynamic Island and the home
// indicator, and on their sides the sensor housing's edge.
var touchScreens = []touchScreen{
	{"320x568", 320, 568, 20, 0, 0, 0},
	{"360x720", 360, 720, 21, 21, 0, 0},
	{"360x803", 360, 803, 21, 21, 0, 0},
	{"360x803, 3-button navigation", 360, 803, 21, 42, 0, 0},
	{"375x667", 375, 667, 20, 0, 0, 0},
	{"393x852", 393, 852, 59, 34, 0, 0},
	{"440x956", 440, 956, 62, 34, 0, 0},
	{"568x320", 568, 320, 0, 0, 0, 0},
	{"667x375", 667, 375, 0, 0, 0, 0},
	{"803x360", 803, 360, 21, 0, 0, 0},
	{"852x393", 852, 393, 0, 21, 59, 59},
	{"956x440", 956, 440, 0, 21, 62, 62},
	{"834x1194", 834, 1194, 24, 20, 0, 0},
	{"1194x834", 1194, 834, 24, 20, 0, 0},
	{"1024x1366", 1024, 1366, 24, 20, 0, 0},
}

// onItsSide reports whether s is a phone on its side, where the Read tab
// reads full-screen and the header shows on Books and Search.
func (s touchScreen) onItsSide() bool { return s.w > s.h && s.h < 500 }

// safeInsetLayout places the window's content as the mobile driver does
// (sizeContent): inside the device's safe area, less the theme's padding on
// every edge, a mobile window being padded.
type safeInsetLayout struct {
	s   touchScreen
	pad float32
}

func (safeInsetLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (l safeInsetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.s.left+l.pad, l.s.top+l.pad))
		o.Resize(size.SubtractWidthHeight(l.s.left+l.s.right+2*l.pad, l.s.top+l.s.bottom+2*l.pad))
	}
}

// touchAppWindow lays the real window out on s, on the Read tab, or on Books
// for a phone on its side, inside a root widget like the layout watcher a
// phone's window holds (CreateMainUI), and gives the sheets the safe area s
// reports (canvasArea).
func touchAppWindow(t *testing.T, s touchScreen) (*AppState, fyne.Window) {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	fyne.SetCurrentApp(phoneTestApp{app})
	prevArea, prevFoot := canvasArea, keyboardFreeFoot
	keyboardFreeFoot.known, keyboardFreeFoot.foot = false, 0
	canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
		return fyne.NewPos(s.left, s.top), c.Size().SubtractWidthHeight(s.left+s.right, s.top+s.bottom)
	}
	t.Cleanup(func() { canvasArea, keyboardFreeFoot = prevArea, prevFoot })
	softKeyboard(t, false)

	st := sampleState()
	w := app.NewWindow("phone")
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(s.w, s.h))
	st.window, st.app, st.theme = w, app, th
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	if s.onItsSide() {
		st.CurrentTab = 1
	}
	w.SetPadded(false)
	root := &wrappingRoot{phoneRoot{content: buildCompactUI(st)}}
	root.ExtendBaseWidget(root)
	w.SetContent(container.New(safeInsetLayout{s, th.Size(theme.SizeNamePadding)}, root))
	w.Resize(fyne.NewSize(s.w, s.h))
	if st.header == nil {
		t.Fatal("the window's header was not recorded")
	}
	return st, w
}

// onCanvasAt is where o lies on its canvas: the test driver, like the
// mobile one, reports positions from the interactive area's origin.
func onCanvasAt(o fyne.CanvasObject) fyne.Position {
	return canvasPosition(fyne.CurrentApp().Driver().CanvasForObject(o), o)
}

// sheetSpan is the box a popup occupies, in canvas units, and, where they
// were read, the least height it can be (its MinSize) and the height of its
// scrolling part (scrollBody).
type sheetSpan struct{ x0, x1, top, bottom, least, body float32 }

func (s sheetSpan) String() string {
	return fmt.Sprintf("x %.1f..%.1f, y %.1f..%.1f", s.x0, s.x1, s.top, s.bottom)
}

func sheetSpanOf(t *testing.T, popup *widget.PopUp) sheetSpan {
	t.Helper()
	top, bottom := sheetBox(t, popup)
	pad := popup.Theme().Size(theme.SizeNameInnerPadding)
	x0 := popup.Content.Position().X - pad/2
	return sheetSpan{x0: x0, x1: x0 + popup.Content.Size().Width + pad, top: top, bottom: bottom}
}

type touchSheet struct {
	name string
	open func(*testing.T, *AppState)
}

// touchSheets is every sheet a phone or tablet opens over the page on its
// own, each opened as the reader opens it, in its tallest and shortest forms
// where its height is data. The notes questions and the model list are not
// among them: they open only from Settings, over it. The two that change the
// reader's state to open as they do — every notice at once, and Matthew 5 —
// come last, so a window can open the others first as they open on a fresh
// one.
func touchSheets(t *testing.T) []touchSheet {
	t.Helper()
	votdSynchronousRemeasure(t)
	holdSheetTimers(t)
	holdNoteSheetTimers(t)
	studies := stubAIActionParked(t)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsLoad = func() error { return nil }
	return []touchSheet{
		{"Settings", func(_ *testing.T, s *AppState) { showAISettings(s) }},
		{"Go to", func(_ *testing.T, s *AppState) { showGotoPicker(s) }},
		{"chapter picker", func(_ *testing.T, s *AppState) { showChapterPicker(s) }},
		{"translation picker", func(_ *testing.T, s *AppState) { showVersionPicker(s) }},
		{"verse of the day, one verse", func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, oneDayVerse()) }},
		{"verse of the day, long", func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, longDayPassage()) }},
		{"audio source menu", func(_ *testing.T, s *AppState) { showAudioSourceMenu(s) }},
		{"note composer", func(_ *testing.T, s *AppState) { promptShareNote(s, "For God so loved the world", selSpan{}) }},
		{"Ask", func(_ *testing.T, s *AppState) { promptAskQuestion(s, "For God so loved the world") }},
		{"AI answer, waiting", func(t *testing.T, s *AppState) {
			showAIPanel(s, aiActionExplain, "For God so loved the world", "")
			waitParked(t, studies, "the study request")
		}},
		{"cross-references, waiting", func(_ *testing.T, s *AppState) {
			crossRefsRun = func(func()) {} // the load never lands
			showCrossRefs(s, "For God so loved the world", selSpan{})
		}},
		{"share image preview", func(_ *testing.T, s *AppState) {
			showShareImagePreview(s, "For God so loved the world", "John 3:16", "WEB")
		}},
		{"note link offer", func(_ *testing.T, s *AppState) {
			offerNoteLinkChoice(s, "https://example.invalid/web/john/3/16",
				ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16})
		}},
		{"link notice", func(_ *testing.T, s *AppState) {
			showLinkNotice(s, "This passage isn't available", "John 3:16",
				"The link names a translation this reader does not have.")
		}},
		{"translation downloading", func(_ *testing.T, s *AppState) { showVersionLoading(s, "Sample Translation") }},
		{"translation download failed", func(_ *testing.T, s *AppState) { showVersionLoadError(s, "Sample Translation") }},
		{"translation picker, more translations", func(t *testing.T, s *AppState) {
			withMoreTranslations(t)
			withEveryNotice(t, s)
			showVersionPicker(s)
		}},
		{"cross-references, listed", func(t *testing.T, s *AppState) {
			crossRefsRun = func(work func()) { work() }
			withBeatitudes(s)
			showCrossRefs(s, "", selSpan{lo: 3, hi: 3})
			if list := findScroll(s.window.Canvas().Overlays().Top()); list == nil || !list.Visible() {
				t.Fatal("control: the panel should be showing its list")
			}
		}},
	}
}

// oneDayVerse is a verse of the day one verse long, the card's shortest
// ordinary form.
func oneDayVerse() dayVerse {
	return dayVerse{Book: "John", Chapter: 3, Lo: 16, Hi: 16, Verses: []Verse{{BookName: "John", Chapter: 3, Verse: 16,
		Text: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish, but have eternal life."}}}
}

// withTouchRule sets whether the sheets keep clear of the header's controls
// on a touch device, for the rest of the test.
func withTouchRule(t *testing.T, on bool) {
	t.Helper()
	prev := touchSheetsKeepClearOfHeader
	touchSheetsKeepClearOfHeader = on
	t.Cleanup(func() { touchSheetsKeepClearOfHeader = prev })
}

// openSheetAs opens sh on st's window with the rule on or off, reads where
// it lies and the least height it can be, and closes it.
func openSheetAs(t *testing.T, st *AppState, sh touchSheet, on bool) sheetSpan {
	t.Helper()
	withTouchRule(t, on)
	popup := pickerPopup(t, st, func(a *AppState) { sh.open(t, a) })
	defer popup.Hide()
	sp := sheetSpanOf(t, popup)
	sp.least = popup.MinSize().Height
	sp.body = scrollBody(popup)
	return sp
}

// scrollBody is the height of the tallest scroll showing in o, what a sheet
// shows of the part of it that scrolls, or zero when nothing in it scrolls
// (a card, a spinner, the image preview).
func scrollBody(o fyne.CanvasObject) float32 {
	var tallest float32
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *container.Scroll:
			tallest = max(tallest, v.Size().Height)
			walk(v.Content)
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		case *container.ThemeOverride:
			walk(v.Content)
		case *widget.PopUp:
			walk(v.Content)
		}
	}
	walk(o)
	return tallest
}

// headerEdges is where st's header begins and ends on the canvas.
func headerEdges(st *AppState) (top, bottom float32) {
	at := onCanvasAt(st.header)
	return at.Y, at.Y + st.header.Size().Height
}

// ON A DESKTOP WINDOW THE RULE HAS NOTHING TO DO. Its sheets keep below the
// header by headerClearance, and every one opens as the same box with the
// rule off and on, on the short window where they are tightest.
func TestTouchHeaderRuleLeavesDesktopSheetsAlone(t *testing.T) {
	sheets := touchSheets(t)
	for _, size := range []fyne.Size{{Width: 1280, Height: 440}} {
		st, _ := desktopWindow(t, size)
		if _, ok := touchHeaderBand(st); ok {
			t.Fatalf("%.0fx%.0f: a desktop window has a header band for the touch rule", size.Width, size.Height)
		}
		for _, sh := range sheets {
			if before, after := openSheetAs(t, st, sh, false), openSheetAs(t, st, sh, true); !sameSpan(before, after) {
				t.Errorf("%.0fx%.0f, %s: the sheet spanned %v; it now spans %v", size.Width, size.Height, sh.name, before, after)
			}
		}
	}
}

func sameSpan(a, b sheetSpan) bool {
	near := func(x, y float32) bool { return math.Abs(float64(x-y)) <= 0.5 }
	return near(a.x0, b.x0) && near(a.x1, b.x1) && near(a.top, b.top) && near(a.bottom, b.bottom)
}

// THE TRANSLATION PICKER ON A 1080-PIXEL ANDROID PHONE AT 420 DPI. On its
// full 1080x2410 screen and at 2:1 (1080x2160), with the translations this
// build compiles in and with more than any build has and every notice at
// once, the picker's top edge is at or below the header's bottom, or at or
// above its top: never between. With more translations it started 12pt
// down the header, partway down the title.
func TestTranslationPickerOnA420DpiAndroidPhoneIsClearOfTheHeader(t *testing.T) {
	sheets := touchSheets(t)
	for _, s := range touchScreens {
		if s.w != 360 {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			st, _ := touchAppWindow(t, s)
			top, bottom := headerEdges(st)
			for _, sh := range sheets {
				if !strings.HasPrefix(sh.name, "translation picker") {
					continue
				}
				popup := pickerPopup(t, st, func(a *AppState) { sh.open(t, a) })
				if sp := sheetSpanOf(t, popup); sp.top > top+0.5 && sp.top < bottom-0.5 {
					t.Errorf("%s: the picker starts at %.1f, inside the header (%.1f..%.1f)", sh.name, sp.top, top, bottom)
				}
				popup.Hide()
			}
		})
	}
}

// WHAT A SHEET THAT SCROLLS KEEPS TO OPEN BELOW THE HEADER. Two thirds of
// its scrolling part, and never less than 120 units of it, or all of it if
// it had less: shortening is refused as soon as either would be broken. The
// matrix's sheets do not reach the 120-unit floor at the text size the tests
// use; a larger text size, whose chrome is deeper, would.
func TestTouchSheetKeepsTwoThirdsAndAHundredAndTwenty(t *testing.T) {
	for _, tc := range []struct {
		body, short float32
		keeps       bool
	}{
		{600, 200, true},  // 400 left, two thirds exactly
		{600, 201, false}, // under two thirds
		{180, 60, true},   // 120 left, both floors exactly
		{150, 30, true},   // 120 left, over two thirds
		{150, 40, false},  // 110 left: over two thirds, under 120
		{100, 1, false},   // under 120 already: it keeps all of it
		{100, 0, true},
	} {
		if got := touchSheetKeeps(tc.body, tc.short); got != tc.keeps {
			t.Errorf("a scroll %.0f tall made %.0f shorter: keeps enough %v, want %v", tc.body, tc.short, got, tc.keeps)
		}
	}
}
