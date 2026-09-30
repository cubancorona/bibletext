package bibletext

// NO DESKTOP SHEET STARTS PARTWAY DOWN THE HEADER.
//
// With Settings open on a 1280x800 Mac window the top of the header's centred
// Go to chip showed as a grey arc above the sheet: Fyne centres a modal sheet,
// and a sheet capped only by the canvas centred to a top edge inside the
// header. Every desktop sheet now opens below it (headerClearance), and stays
// below it when the window changes size (sheet_refit.go). These lay the real
// window out — header, rail, reading pane — open each sheet over it, and read
// where the sheet's box begins.

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

type desktopSheet struct {
	name string
	open func(*testing.T, *AppState)
}

// desktopSheets is every sheet a desktop window opens over the page, each
// opened as the reader opens it. The AI answer opens in its waiting state and
// stays there: its request is parked for the rest of the test, so nothing
// lands on another goroutine while the test reads the sheet. The
// cross-references open twice: waiting on the load, and with the load
// answered and the list showing, both on the test's own goroutine. So does
// the translation picker: with the translations the build compiles in, and
// with more than any build has and every notice at once, so the sheet's
// longest form is covered whatever tags the suite is built with.
func desktopSheets(t *testing.T) []desktopSheet {
	t.Helper()
	votdSynchronousRemeasure(t)
	holdSheetTimers(t) // the audio source menu's watchdog, and the composer's
	studies := stubAIActionParked(t)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsLoad = func() error { return nil }
	return []desktopSheet{
		{"Settings", func(_ *testing.T, s *AppState) { showAISettings(s) }},
		{"Go to", func(_ *testing.T, s *AppState) { showGotoPicker(s) }},
		{"translation picker", func(_ *testing.T, s *AppState) { showVersionPicker(s) }},
		{"translation picker, more translations", func(t *testing.T, s *AppState) {
			withMoreTranslations(t)
			withEveryNotice(t, s)
			showVersionPicker(s)
		}},
		{"verse of the day", func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, longDayPassage()) }},
		{"audio source menu", func(_ *testing.T, s *AppState) { showAudioSourceMenu(s) }},
		{"note composer", func(_ *testing.T, s *AppState) { promptShareNote(s, "For God so loved the world", selSpan{}) }},
		{"share image preview", func(_ *testing.T, s *AppState) {
			showShareImagePreview(s, "For God so loved the world", "John 3:16", "WEB")
		}},
		{"share confirmation", func(t *testing.T, s *AppState) {
			withoutMailProbe(t)
			showShareCopiedSheet(s, shareDoneForText(strings.Repeat("For God so loved the world, that he gave his one and only Son. ", 12)+
				"\n\n— John 3:16 (World English Bible)"))
		}},
		{"share confirmation, image", func(t *testing.T, s *AppState) {
			withoutMailProbe(t)
			showShareCopiedSheet(s, shareDone{line: shareLineImage, subject: "John 3:16 (World English Bible)", attachment: "card.png"})
		}},
		{"cross-references", func(_ *testing.T, s *AppState) {
			crossRefsRun = func(func()) {} // the load never lands
			showCrossRefs(s, "For God so loved the world", selSpan{})
		}},
		{"cross-references listed", func(t *testing.T, s *AppState) {
			crossRefsRun = func(work func()) { work() }
			withBeatitudes(s)
			showCrossRefs(s, "", selSpan{lo: 3, hi: 3})
			top := s.window.Canvas().Overlays().Top()
			if list := findScroll(top); list == nil || !list.Visible() ||
				len(list.Content.(*fyne.Container).Objects) == 0 || treeHasText(top, "No cross-references for this selection.") {
				t.Fatalf("control: the panel should be showing its list; texts %v", treeTexts(top))
			}
		}},
		{"AI answer", func(t *testing.T, s *AppState) {
			showAIPanel(s, aiActionExplain, "For God so loved the world", "")
			waitParked(t, studies, "the study request")
		}},
	}
}

// moreTranslations is how many translations under evaluation
// withMoreTranslations adds: more than any set of build tags compiles in.
const moreTranslations = 8

// sampleTranslationID is the id of the i-th translation withMoreTranslations
// registers.
func sampleTranslationID(i int) string { return fmt.Sprintf("sample-%c", 'a'+i) }

// withMoreTranslations registers moreTranslations synthetic translations
// under evaluation for the rest of the test, once however often it is called.
// The translation picker's height is data: a row for each translation and a
// sentence naming those under evaluation, which the nrsv and lsb tags
// lengthen. Opened with these, the picker is longer than any build's, so what
// the sheet tests cover does not depend on the tags the suite is built with.
func withMoreTranslations(t *testing.T) {
	t.Helper()
	if _, ok := versionByID(sampleTranslationID(0)); ok {
		return
	}
	for i := 0; i < moreTranslations; i++ {
		id := sampleTranslationID(i)
		withRegisteredVersion(t, BibleVersion{
			ID: id, Name: fmt.Sprintf("Sample Translation %c", 'A'+i), Abbrev: strings.ToUpper(id),
			Publisher: "Sample Publisher — license required",
			source:    newLicensedSource(id),
		})
	}
}

// withEveryNotice puts s where the picker's notice says every fact it can at
// once, one per line: the reader's choice could not be opened, the default
// translation is updating, and every registered translation is on a previous
// edition. The update is already downloading, so opening the picker starts
// no fetch.
func withEveryNotice(t *testing.T, s *AppState) {
	t.Helper()
	vs := bibleVersions()
	s.CurrentVersion = defaultVersionID
	s.preferredVersion = vs[len(vs)-1].ID
	s.fullPending, s.fullDownloading, s.seedOnly = true, true, false
	s.staleVersions = map[string]bool{}
	for _, v := range vs {
		s.staleVersions[v.ID] = true
	}
	if n := fullPendingNotice(s); strings.Count(n, "\n") != 2 {
		t.Fatalf("control: the notice should say three facts, one per line; it reads %q", n)
	}
}

// longDayPassage is a verse-of-the-day passage long enough that the card is
// capped by every window these tests use but the portrait display, so its
// height follows the window's. Today's verse would make the card's height
// depend on the date.
func longDayPassage() dayVerse {
	d := dayVerse{Book: "Psalms", Chapter: 119, Lo: 1, Hi: 16}
	for v := d.Lo; v <= d.Hi; v++ {
		d.Verses = append(d.Verses, Verse{BookName: "Psalms", Chapter: 119, Verse: v,
			Text: "Blessed are those whose ways are blameless, who walk according to Yahweh’s law."})
	}
	return d
}

// withBeatitudes puts the reader on Matthew 5, with Matthew 5:3 and the first
// verse of Luke's parallel to it, so a selection of Matthew 5:3 lists Luke
// 6:20-23. The Gospel parallels are embedded (parallels.go), so no Treasury
// is needed.
func withBeatitudes(s *AppState) {
	s.Bible.Verses["Matthew"] = map[int][]Verse{5: {{BookName: "Matthew", Chapter: 5, Verse: 3,
		Text: "Blessed are the poor in spirit, for theirs is the Kingdom of Heaven."}}}
	s.Bible.Verses["Luke"] = map[int][]Verse{6: {{BookName: "Luke", Chapter: 6, Verse: 20,
		Text: "He lifted up his eyes to his disciples, and said, Blessed are you who are poor."}}}
	s.CurrentBook, s.CurrentChapter = "Matthew", 5
}

// waitParked waits for a parked call to start, which also orders its read of
// the seam before the test puts the seam back.
func waitParked[T any](t *testing.T, started chan T, what string) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s never started", what)
	}
}

// desktopApp is a test app wearing the real theme, for desktop windows to be
// laid out on.
type desktopApp struct {
	app fyne.App
	th  *bibleTheme
	// windows is what the app had open when it started, the one window the
	// test driver opens for itself, and sizes and contents what each had.
	windows  []fyne.Window
	sizes    []fyne.Size
	contents []fyne.CanvasObject
}

// newDesktopApp starts a test app and gives it the real theme.
func newDesktopApp(t *testing.T) *desktopApp {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	d := &desktopApp{app: app, th: th}
	for _, w := range app.Driver().AllWindows() {
		d.windows = append(d.windows, w)
		d.sizes = append(d.sizes, w.Canvas().Size())
		d.contents = append(d.contents, w.Content())
	}
	return d
}

// asNew puts d back as newDesktopApp left it, apart from what the toolkit has
// cached, and fails where it cannot. It takes off the two things a case may
// leave on an app that a new one does not have, preferences set and text on
// the clipboard (opening Settings stores the text size it shows), and fails
// unless the rest is as it started: the same current app, the same theme,
// and no window open but the driver's own, at the size and with the content
// it had.
//
// The sheet sweeps lay every case out on one app rather than starting one
// per case. Starting a test app throws away the toolkit's parsed font faces
// and measured text, and parsing and measuring again from nothing was most of
// what laying a window out cost. What comes back from those caches is what
// measuring afresh gives: a measure is keyed on the text, its size and style
// and the font resource it names, and with no resource named it is made in
// the app's theme, which is the same one for every case. So each case begins
// here, and nothing else a case might leave on the app carries into the next.
func (d *desktopApp) asNew(t *testing.T) {
	t.Helper()
	if fyne.CurrentApp() != d.app {
		t.Fatal("another app has replaced the test app the sweep shares")
	}
	if th := d.app.Settings().Theme(); th != fyne.Theme(d.th) {
		t.Fatalf("the shared app wears %T, not the theme it was given", th)
	}
	ws := d.app.Driver().AllWindows()
	if !slices.Equal(ws, d.windows) {
		t.Fatalf("the shared app has %d windows open, where it started with %d: an earlier case left one open",
			len(ws), len(d.windows))
	}
	for i, w := range ws {
		if w.Canvas().Size() != d.sizes[i] || w.Content() != d.contents[i] {
			t.Fatalf("an earlier case changed the test driver's own window")
		}
	}
	prefs := d.app.Preferences()
	values, ok := prefs.(interface{ ReadValues(func(map[string]any)) })
	if !ok {
		t.Fatalf("the shared app's preferences (%T) cannot be read back", prefs)
	}
	keys := func() []string {
		var set []string
		values.ReadValues(func(m map[string]any) {
			for k := range m {
				set = append(set, k)
			}
		})
		return set
	}
	for _, k := range keys() {
		prefs.RemoveValue(k)
	}
	if left := keys(); len(left) > 0 {
		t.Fatalf("preferences %v stay set on the shared app", left)
	}
	d.app.Clipboard().SetContent("")
	if c := d.app.Clipboard().Content(); c != "" {
		t.Fatalf("the shared app's clipboard still holds %q", c)
	}
}

// desktopWindow lays the real window out at size on a fresh test app.
func desktopWindow(t *testing.T, size fyne.Size) (*AppState, fyne.Window) {
	t.Helper()
	return newDesktopApp(t).window(t, size)
}

// window lays the real window out at size, on a fresh state, on d.
func (d *desktopApp) window(t *testing.T, size fyne.Size) (*AppState, fyne.Window) {
	t.Helper()
	if fyne.CurrentApp() != d.app {
		t.Fatal("another app has replaced the test app this window is for")
	}
	st := sampleState()
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(size)
	st.window, st.app, st.theme = w, d.app, d.th
	// No assistant key: Settings would fetch the provider's model list for
	// one. The study request never reads it; it is parked (desktopSheets).
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	w.SetContent(buildCompactUI(st))
	w.Resize(size)
	if st.header == nil {
		t.Fatal("the window's header was not recorded; nothing can clear it")
	}
	return st, w
}

// wantClearOfHeader fails if the sheet's top edge lies inside the header, or
// partway down the Go to chip in it.
func wantClearOfHeader(t *testing.T, st *AppState, w fyne.Window, popup *widget.PopUp) {
	t.Helper()
	drv := fyne.CurrentApp().Driver()
	headerBottom := drv.AbsolutePositionForObject(st.header).Y + st.header.Size().Height
	var chip *widget.Button
	walkTree(w.Content(), func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Go to" {
			chip = b
		}
	})
	if chip == nil {
		t.Fatal("no Go to chip in the header")
	}
	// The chip's outline is a 1pt stroke centred on its edge.
	chipTop := drv.AbsolutePositionForObject(chip).Y - 0.5
	chipBottom := chipTop + chip.Size().Height + 1

	top, _ := sheetBox(t, popup)
	if top > chipTop && top < chipBottom {
		t.Errorf("the sheet's top edge (%.1f) is partway down the Go to chip (%.1f..%.1f)",
			top, chipTop, chipBottom)
	}
	if top < headerBottom+sheetHeaderGap-0.5 {
		t.Errorf("the sheet starts at %.1f, inside the header (which ends at %.1f; sheets open %dpt below it)",
			top, headerBottom, sheetHeaderGap)
	}
}

func TestDesktopSheetsOpenBelowTheHeader(t *testing.T) {
	sheets := desktopSheets(t)
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{
		{Width: 1280, Height: 800}, // the window the arc was seen in
		{Width: 1280, Height: 860}, // the size the app asks for at launch
		{Width: 1024, Height: 768},
		{Width: 1440, Height: 900},
		{Width: 1280, Height: 600}, // a short window, where the pickers reached the header too
		{Width: 1280, Height: 480}, // shorter: the share preview's image stopped shrinking here
		{Width: 1280, Height: 440},
		{Width: 507, Height: 440}, // as narrow as the window's content allows
	} {
		for _, sh := range sheets {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, sh.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { sh.open(t, s) })
				defer popup.Hide()
				wantClearOfHeader(t, st, w, popup)
			})
		}
	}
}

// A SHEET STAYS CLEAR WHEN THE WINDOW CHANGES SIZE UNDER IT. The toolkit
// re-centres an open popup at the size it opened at, so a sheet sized for a
// taller window rose into the header as the window shrank. Each is now sized
// again, to exactly what opening it at the new size gives, and a sheet opened
// in a short window takes the room a taller one gives it.
func TestDesktopSheetsRefitWhenTheWindowResizes(t *testing.T) {
	sheets := desktopSheets(t)
	app := newDesktopApp(t)
	for _, rs := range []struct {
		why      string
		from, to fyne.Size
	}{
		{"a maximised window restored", fyne.NewSize(1440, 900), fyne.NewSize(1280, 800)},
		{"the launch size shrunk a little", fyne.NewSize(1280, 860), fyne.NewSize(1280, 800)},
		{"an edge dragged up", fyne.NewSize(1280, 800), fyne.NewSize(1280, 720)},
		{"further", fyne.NewSize(1280, 800), fyne.NewSize(1280, 680)},
		{"to a short window", fyne.NewSize(1280, 800), fyne.NewSize(1280, 440)},
		{"grown", fyne.NewSize(1280, 600), fyne.NewSize(1280, 1000)},
		// Room for every sheet at its natural height, the translation
		// picker's with more translations too, so each is sized from what its
		// content measures rather than by the cap.
		{"grown onto a portrait display", fyne.NewSize(1280, 800), fyne.NewSize(1280, 2400)},
		{"narrowed", fyne.NewSize(1280, 800), fyne.NewSize(900, 800)},
	} {
		for _, sh := range sheets {
			t.Run(fmt.Sprintf("%.0fx%.0f to %.0fx%.0f/%s", rs.from.Width, rs.from.Height, rs.to.Width, rs.to.Height, sh.name), func(t *testing.T) {
				app.asNew(t)
				// Where the sheet sits when it opens in a window of the new size.
				fst, _ := app.window(t, rs.to)
				fresh := pickerPopup(t, fst, func(s *AppState) { sh.open(t, s) })
				wantTop, wantBottom := sheetBox(t, fresh)
				fresh.Hide()

				st, w := app.window(t, rs.from)
				popup := pickerPopup(t, st, func(s *AppState) { sh.open(t, s) })
				defer popup.Hide()
				w.Resize(rs.to)
				wantClearOfHeader(t, st, w, popup)
				top, bottom := sheetBox(t, popup)
				if math.Abs(float64(top-wantTop)) > 0.5 || math.Abs(float64(bottom-wantBottom)) > 0.5 {
					t.Errorf("%s: the sheet spans %.1f..%.1f, where one opened at this size spans %.1f..%.1f",
						rs.why, top, bottom, wantTop, wantBottom)
				}
			})
		}
	}
}

// THE PICKER'S SENTENCES SCROLL ONLY WHERE THEY CANNOT BE PINNED. Under its
// rows the translation picker says which translations are locked and why,
// and what is true of the edition on screen, pinned above Close. How long
// those sentences are is data, so where the pinned part alone would be
// taller than the room below the header they follow the rows inside the
// scroll instead: every one, in the same words and order, with every row
// still listed. Given room again they are pinned again.
func TestTranslationPickerPinsItsSentencesWhereTheyFit(t *testing.T) {
	// The window the app most often has, the translations this build
	// compiles in, and the notice a reader most often sees.
	st, _ := desktopWindow(t, fyne.NewSize(1280, 800))
	st.fullPending, st.fullDownloading = true, true
	popup := pickerPopup(t, st, showVersionPicker)
	pinned := pickerSentences(popup)
	if len(pinned) == 0 || pinned[0].Text != fullPendingNotice(st) {
		t.Fatalf("control: the picker should open with the notice first among its sentences; it has %v", labelTexts(pinned))
	}
	wantSentencesPinned(t, popup, pinned, "1280x800")
	popup.Hide()

	// Short, with more translations than any build and every notice at once:
	// pinned, they would put the sheet's top inside the header.
	st, w := desktopWindow(t, fyne.NewSize(1280, 440))
	popup = pickerPopup(t, st, func(s *AppState) {
		withMoreTranslations(t)
		withEveryNotice(t, s)
		showVersionPicker(s)
	})
	defer popup.Hide()
	wantClearOfHeader(t, st, w, popup)
	scrolled := labelTexts(pickerSentences(popup))
	body := findScroll(popup)
	drv := fyne.CurrentApp().Driver()
	var lastRowBottom float32
	for _, v := range versionPickerOrder() {
		name := findTreeStatus(body.Content, v.Name+"  ("+v.Abbrev+")")
		if name == nil {
			t.Fatalf("1280x440: the %s row is not in the list", v.ID)
		}
		lastRowBottom = drv.AbsolutePositionForObject(name).Y + name.Size().Height
	}
	for _, l := range pickerSentences(popup) {
		if !objectUnder(body.Content, l) {
			t.Errorf("1280x440: %q is pinned, where there is no room for it", l.Text)
		} else if top := drv.AbsolutePositionForObject(l).Y; top < lastRowBottom {
			t.Errorf("1280x440: %q starts at %.1f, above the end of the last row (%.1f)", l.Text, top, lastRowBottom)
		}
	}

	// Grown to a window with room for them, the same sentences are pinned
	// again.
	w.Resize(fyne.NewSize(1280, 1600))
	again := pickerSentences(popup)
	wantSentencesPinned(t, popup, again, "grown to 1280x1600")
	if len(scrolled) < 2 {
		t.Fatalf("control: the notice and the evaluation sentence should both show; the sentences are %q", scrolled)
	}
	if got := labelTexts(again); strings.Join(got, "|") != strings.Join(scrolled, "|") {
		t.Errorf("the sentences changed between the short window and the tall one:\n  %q\n  %q", scrolled, got)
	}
}

// pickerSentences returns the translation picker's sentences, in the order it
// shows them: every label but the intro.
func pickerSentences(popup *widget.PopUp) []*widget.Label {
	var out []*widget.Label
	walkTree(popup, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok && l.Text != "Choose a Bible version." {
			out = append(out, l)
		}
	})
	return out
}

// labelTexts is what each label reads, in order.
func labelTexts(ls []*widget.Label) []string {
	var out []string
	for _, l := range ls {
		out = append(out, l.Text)
	}
	return out
}

// wantSentencesPinned fails unless every sentence sits outside the scroll,
// between its bottom edge and the Close button.
func wantSentencesPinned(t *testing.T, popup *widget.PopUp, sentences []*widget.Label, where string) {
	t.Helper()
	sheetBox(t, popup) // lays the popup out
	drv := fyne.CurrentApp().Driver()
	body := findScroll(popup)
	closeBtn := findTreeButton(popup, "Close")
	if body == nil || closeBtn == nil {
		t.Fatalf("%s: the picker has no list or no Close button", where)
	}
	listBottom := drv.AbsolutePositionForObject(body).Y + body.Size().Height
	closeTop := drv.AbsolutePositionForObject(closeBtn).Y
	for _, l := range sentences {
		top := drv.AbsolutePositionForObject(l).Y
		if objectUnder(body.Content, l) || top < listBottom || top+l.Size().Height > closeTop {
			t.Errorf("%s: %q is not pinned between the list (ends %.1f) and Close (starts %.1f); it spans %.1f..%.1f",
				where, l.Text, listBottom, closeTop, top, top+l.Size().Height)
		}
	}
}

// objectUnder reports whether target is root or anywhere under it.
func objectUnder(root, target fyne.CanvasObject) bool {
	found := false
	walkTree(root, func(o fyne.CanvasObject) {
		if o == target {
			found = true
		}
	})
	return found
}

// findTreeStatus returns the first statusLine under o reading text: the
// translation picker's row names, publishers and captions.
func findTreeStatus(o fyne.CanvasObject, text string) *statusLine {
	var found *statusLine
	walkTree(o, func(n fyne.CanvasObject) {
		if s, ok := n.(*statusLine); ok && found == nil && s.text == text {
			found = s
		}
	})
	return found
}

// THE PICKER FITS A 320PT PHONE. The sheet there is 280pt wide, and its
// rows are drawn as statusLines that break between words, its sentences
// wrapped to the list's width by the squeeze the list sits in: nothing in
// the list ends past the list's edge, and the sheet sits inside the screen.
// Opened in the notice states a reader meets — the default translation
// updating on a previous edition, the reader's choice fallen back on a
// previous edition, and every notice at once — with the translations the
// build compiles in. Mutations guarded: the list not squeezed (the sentences
// keep the width they wrapped at pinned, and lose their last words); the
// rows as canvas.Text (the NKJV publisher line, the WEBC name and the key
// caption run past the edge).
func TestTranslationPickerFitsA320Phone(t *testing.T) {
	staleAll := func(s *AppState) {
		s.staleVersions = map[string]bool{}
		for _, v := range bibleVersions() {
			s.staleVersions[v.ID] = true
		}
	}
	for _, tc := range []struct {
		name    string
		arrange func(*testing.T, *AppState)
	}{
		{"updating and stale", func(_ *testing.T, s *AppState) {
			s.CurrentVersion = defaultVersionID
			s.fullPending, s.fullDownloading = true, true
			staleAll(s)
		}},
		{"fallen back and stale", func(_ *testing.T, s *AppState) {
			vs := bibleVersions()
			s.CurrentVersion = defaultVersionID
			s.preferredVersion = vs[len(vs)-1].ID
			staleAll(s)
		}},
		{"every notice", func(t *testing.T, s *AppState) { withEveryNotice(t, s) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := sampleState()
			st.aiKeys = newKeyStoreWith(newFakePrefs())
			win := noteSheetWindow(t, st, true, 320, 568)
			win.SetContent(canvas.NewRectangle(st.pal().Background))
			tc.arrange(t, st)
			if n := fullPendingNotice(st); strings.Count(n, "\n") < 1 {
				t.Fatalf("control: the notice should say at least two facts; it reads %q", n)
			}
			popup := pickerPopup(t, st, showVersionPicker)
			defer popup.Hide()
			// The frames a painting canvas lays out after the open, until
			// nothing asks for another (paintedFrames).
			walk := newLayoutWalker(t)
			frames := newPaintedFrames(walk, popup)
			for i := 0; i < 12 && len(frames.frame()) > 0; i++ {
			}
			sheetBox(t, popup)
			drv := fyne.CurrentApp().Driver()
			if left, width := drv.AbsolutePositionForObject(popup.Content).X, popup.Content.Size().Width; left < 0 || left+width > 320 {
				t.Errorf("the sheet spans x %.1f..%.1f on a 320pt canvas", left, left+width)
			}
			body := findScroll(popup)
			if body == nil {
				t.Fatal("the picker has no list")
			}
			listRight := drv.AbsolutePositionForObject(body).X + body.Size().Width
			sentences := pickerSentences(popup)
			if len(sentences) < 2 {
				t.Fatalf("control: the notice and the key sentence should both show; the sentences are %q", labelTexts(sentences))
			}
			under := func(p placed, root fyne.CanvasObject) bool {
				for _, a := range p.path {
					if a == root {
						return true
					}
				}
				return false
			}
			texts := 0
			for _, p := range walk.walk(popup) {
				tx, ok := p.obj.(*canvas.Text)
				if !ok || !p.shown || tx.Text == "" {
					continue
				}
				texts++
				end := p.pos.X + tx.MinSize().Width
				if end > 320.5 {
					t.Errorf("%q ends at %.1f, off the 320pt canvas", tx.Text, end)
				}
				if under(p, body.Content) && end > listRight+0.5 {
					t.Errorf("%q ends at %.1f, past the list's edge at %.1f", tx.Text, end, listRight)
				}
			}
			if texts < 12 {
				t.Fatalf("control: the walk saw only %d texts; the rows and sentences were not reached", texts)
			}
			for _, l := range sentences {
				if !treeHasText(popup, l.Text) {
					t.Errorf("the sentence %q is not on the sheet", l.Text)
				}
			}
		})
	}
}

// ON A PHONE OR TABLET NOTHING IS RESIZED UNDER THE READER. There the
// content changes height as the soft keyboard comes and goes, and a refit
// would resize Settings while the reader types a key into it. Only a desktop
// window refits its sheets (registerSheetRefit).
func TestSheetsAreNotRefitOnAPhoneOrTablet(t *testing.T) {
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	fyne.SetCurrentApp(phoneTestApp{app})
	st := sampleState()
	w := app.NewWindow("Settings")
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(834, 700))
	st.window, st.app, st.theme = w, app, th
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	w.SetContent(windowRoot(st, canvas.NewRectangle(st.pal().Background)))
	w.Resize(fyne.NewSize(834, 700))

	popup := pickerPopup(t, st, showAISettings)
	top, bottom := sheetBox(t, popup)
	w.Resize(fyne.NewSize(834, 1400))
	nt, nb := sheetBox(t, popup)
	if math.Abs(float64((nb-nt)-(bottom-top))) > 0.5 {
		t.Errorf("the sheet went from %.1fpt to %.1fpt tall as the canvas changed size under it",
			bottom-top, nb-nt)
	}
	popup.Hide()

	// Control: opened on the taller canvas the sheet is taller, so a refit
	// would have shown above.
	again := pickerPopup(t, st, showAISettings)
	defer again.Hide()
	if at, ab := sheetBox(t, again); ab-at < bottom-top+50 {
		t.Fatalf("control: Settings opens %.1fpt tall on a 1400pt canvas and %.1fpt on a 700pt one; "+
			"the check above could not see a refit", ab-at, bottom-top)
	}
}

// With no header on screen there is nothing to clear, and a sheet keeps the
// full height the canvas gives it.
func TestHeaderClearanceNeedsAHeaderOnScreen(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	st := sampleState()
	w := test.NewWindow(nil)
	defer w.Close()
	st.window, st.app = w, app
	w.Resize(fyne.NewSize(1280, 800))

	if c := headerClearance(st); c != 0 {
		t.Errorf("no header recorded, clearance %v, want 0", c)
	}
	// A header built but not on the window — the tree a rebuild replaced.
	buildHeader(st)
	if c := headerClearance(st); c != 0 {
		t.Errorf("a header that is not on the window gives clearance %v, want 0", c)
	}
	w.SetContent(buildCompactUI(st))
	if c := headerClearance(st); c <= 0 {
		t.Errorf("the header on the window gives clearance %v, want its bottom edge plus the gap", c)
	}

	if got := clearOfHeader(700, 800, 0); got != 700 {
		t.Errorf("clearance 0 changed the height to %v", got)
	}
	if got := clearOfHeader(700, 800, 80); got != 640 {
		t.Errorf("a 700pt sheet on an 800pt canvas clearing 80pt: %v, want 640", got)
	}
	if got := clearOfHeader(300, 800, 80); got != 300 {
		t.Errorf("a sheet already clear of the header was changed to %v", got)
	}
	if got := clearOfHeader(300, 250, 80); got != minSheetHeight {
		t.Errorf("a window too short to clear the header: %v, want the %v floor", got, minSheetHeight)
	}
}
