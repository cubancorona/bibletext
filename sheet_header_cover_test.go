package bibletext

// A sheet that cannot clear the header covers it.
//
// A desktop window can be as short as 386pt, and below about 440pt the parts
// of some sheets that do not scroll are taller than the room below the header
// and its gap. Fyne centres a modal sheet and never sizes it below its
// content, so those sheets started in the gap or inside the header: at 386pt
// the cross-references, while their list loads, 7pt below the Go to chip, and
// a link notice partway down it. Such a sheet now covers the header, as a
// phone or tablet sheet at its cap starts above the header's controls, and
// every other sheet the sweep below opens still opens below the header with
// the gap every sheet keeps (headerClearance). These lay the real window out
// at the heights the window allows, as sheet_header_clearance_test.go does
// above them.

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// shortWindowSheets is every sheet desktopSheets opens; the cards a desktop
// window centres that size themselves to their content: a link notice, the
// note-link offer, Ask, the question turning shared notes off asks, the
// download error and the download's own card; the two sheets Settings
// opens over itself: the Delete all notes question and the model picker, with
// a model list longer than any window has room for; and the question asked
// before the first request to an AI provider, whose words scroll so that it
// always clears the header.
func shortWindowSheets(t *testing.T) []desktopSheet {
	t.Helper()
	target := ShareTarget{VersionID: defaultVersionID, Book: "John", Chapter: 3, VerseLo: 16, Note: "a note"}
	return append(desktopSheets(t),
		desktopSheet{"link notice", func(_ *testing.T, s *AppState) { showLinkVersionUnavailable(s, "New King James Version") }},
		desktopSheet{"note-link offer", func(_ *testing.T, s *AppState) {
			offerNoteLinkChoice(s, "https://bibletext.co.uk/web/john/3/#v16", target)
		}},
		desktopSheet{"Ask", func(_ *testing.T, s *AppState) { promptAskQuestion(s, "For God so loved the world", selSpan{}) }},
		desktopSheet{"notes off", func(_ *testing.T, s *AppState) { promptKeepOrDeleteNotes(s, func() {}, func() {}) }},
		desktopSheet{"download error", func(_ *testing.T, s *AppState) { showVersionLoadError(s, "World English Bible") }},
		desktopSheet{"download", func(_ *testing.T, s *AppState) { showVersionLoading(s, "World English Bible") }},
		desktopSheet{"delete all notes", func(t *testing.T, s *AppState) {
			for v := 16; v <= 18; v++ {
				if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
					Book: "John", Chapter: 3, VerseLo: v, Text: fmt.Sprintf("a note on verse %d", v)}); !ok {
					t.Fatal("control: the note was not stored")
				}
			}
			showAISettings(s)
			del := findTreeButton(s.window.Canvas().Overlays().Top(), "Delete all notes")
			if del == nil || del.Disabled() {
				t.Fatal("control: Settings should offer Delete all notes with notes stored")
			}
			del.OnTapped()
		}},
		desktopSheet{"model picker", func(t *testing.T, s *AppState) {
			settings := settingsWithListedModels(t, s, 20)
			pick := findTreeButton(settings, "Recommended")
			if pick == nil {
				t.Fatalf("control: no model button in Settings; texts %v", treeTexts(settings))
			}
			pick.OnTapped()
		}},
		desktopSheet{"AI permission", func(_ *testing.T, s *AppState) {
			promptAIPermission(s, aiProviders()[0], nil, nil, nil)
		}},
	)
}

// settingsWithListedModels opens Settings with a key saved for the default
// provider, answers the model list the sheet fetches with n models, and waits
// for the list to land. The sheet fetches on a goroutine of its own and lands
// the list from it; the fetch's context is cancelled as that goroutine
// returns, after the landing, so waiting on it orders the landing before
// anything the test reads.
func settingsWithListedModels(t *testing.T, s *AppState, n int) fyne.CanvasObject {
	t.Helper()
	asked := make(chan context.Context, 1)
	answer := make(chan struct{})
	prev := aiSettingsListModels
	aiSettingsListModels = func(ctx context.Context, _ providerInfo, _ string) ([]discoveredModel, error) {
		asked <- ctx
		<-answer
		models := make([]discoveredModel, n)
		for i := range models {
			models[i] = discoveredModel{id: fmt.Sprintf("gemini-sample-%02d", i), rank: int64(i)}
		}
		return models, nil
	}
	t.Cleanup(func() { aiSettingsListModels = prev })
	s.aiKeys.setAPIKey(defaultProviderID, "test-key")
	showAISettings(s)
	settings := s.window.Canvas().Overlays().Top()
	var ctx context.Context
	select {
	case ctx = <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("control: Settings did not fetch the model list for the saved key")
	}
	close(answer)
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the model list never landed")
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("the fetch ended with %v, not with its goroutine returning", ctx.Err())
	}
	return settings
}

// coversAt is, for each window the sweep lays out, every sheet in
// shortWindowSheets that cannot clear the header there with the content
// those tests give it, and so covers it. Each sheet covers on every window up
// to a height and clears it above: the link notice to 440pt, the note-link
// offer to 418pt, the cross-references while they load to 416pt, the share
// confirmation with a long text to 407pt (430pt at 507pt wide), Go to to
// 395pt, the note composer and Ask to 398pt, and the AI answer while it waits
// to 413pt, where the scroll under its waiting line keeps the line whole
// (ai_panel_waiting_line_test.go). The windows either side of each sheet's
// last covering height hold the line in both directions.
var coversAt = map[fyne.Size][]string{
	{Width: 1280, Height: 386}: {"Go to", "note composer", "share confirmation", "cross-references", "AI answer",
		"link notice", "note-link offer", "Ask"},
	{Width: 507, Height: 386}: {"Go to", "note composer", "share confirmation", "cross-references", "AI answer",
		"link notice", "note-link offer", "Ask"},
	{Width: 1280, Height: 392}: {"Go to", "note composer", "share confirmation", "cross-references", "AI answer",
		"link notice", "note-link offer", "Ask"},
	{Width: 1280, Height: 393}: {"Go to", "note composer", "share confirmation", "cross-references", "AI answer",
		"link notice", "note-link offer", "Ask"},
	{Width: 1280, Height: 398}: {"note composer", "share confirmation", "cross-references", "AI answer", "link notice",
		"note-link offer", "Ask"},
	{Width: 1280, Height: 399}: {"share confirmation", "cross-references", "AI answer", "link notice", "note-link offer"},
	{Width: 1280, Height: 401}: {"share confirmation", "cross-references", "AI answer", "link notice", "note-link offer"},
	{Width: 1280, Height: 407}: {"share confirmation", "cross-references", "AI answer", "link notice", "note-link offer"},
	{Width: 1280, Height: 408}: {"cross-references", "AI answer", "link notice", "note-link offer"},
	{Width: 1280, Height: 413}: {"cross-references", "AI answer", "link notice", "note-link offer"},
	{Width: 1280, Height: 414}: {"cross-references", "link notice", "note-link offer"},
	{Width: 1280, Height: 418}: {"link notice", "note-link offer"},
	{Width: 1280, Height: 419}: {"link notice"},
	{Width: 507, Height: 430}:  {"share confirmation", "link notice"},
	{Width: 507, Height: 431}:  {"link notice"},
	{Width: 1280, Height: 440}: {"link notice"},
	{Width: 1280, Height: 442}: nil,
	{Width: 1280, Height: 448}: nil,
}

// wantClearOrCovering fails unless the sheet either starts below the header
// with the gap every sheet keeps, or covers the header: its top edge the
// header's, and its bottom no lower than as far above the window's foot, so
// it is wholly on screen. Either way its popup, which draws the shade, spans
// the whole window, a sheet raised to cover the header (raiseSheet) as well
// as one centred on the window. It reports whether the sheet starts above
// that gap, and so must cover the header.
func wantClearOrCovering(t *testing.T, st *AppState, w fyne.Window, popup *widget.PopUp) bool {
	t.Helper()
	drv := fyne.CurrentApp().Driver()
	headerTop := drv.AbsolutePositionForObject(st.header).Y
	headerBottom := headerTop + st.header.Size().Height
	cs := w.Canvas().Size()
	canvasH := cs.Height
	top, bottom := sheetBox(t, popup)
	if pos, size := popup.Position(), popup.Size(); pos.X > 0.5 || pos.Y > 0.5 ||
		pos.X+size.Width < cs.Width-0.5 || pos.Y+size.Height < canvasH-0.5 {
		t.Errorf("the sheet's popup spans %.1f,%.1f to %.1f,%.1f, not the whole %.0fx%.0f window: its shade leaves part of the page undimmed",
			pos.X, pos.Y, pos.X+size.Width, pos.Y+size.Height, cs.Width, canvasH)
	}
	if top >= headerBottom+sheetHeaderGap-0.5 {
		return false
	}
	if math.Abs(float64(top-headerTop)) > 0.5 {
		t.Errorf("the sheet starts at %.1f: neither below the header (%.1f..%.1f, sheets open %dpt below it) nor covering it from its top edge",
			top, headerTop, headerBottom, sheetHeaderGap)
	}
	if bottom > canvasH-headerTop+0.5 {
		t.Errorf("covering the header, the sheet ends at %.1f, below the %.1f a %.0fpt window gives it",
			bottom, canvasH-headerTop, canvasH)
	}
	return true
}

// coveringFills is every sheet in shortWindowSheets that covers the header
// on some window and whose lists or scroll areas use the room they are
// given: Go to's book and chapter lists; the cross-references, a 460pt panel
// on a window with room to spare whatever its body holds; and the AI answer
// while it waits, whose waiting column scrolls and with the panel's chrome
// is taller than the room on the windows where the panel covers. Covering,
// they fill the room from the header's top. Every other covering sheet is a
// card sized from its content.
var coveringFills = map[string]bool{"Go to": true, "cross-references": true, "AI answer": true}

// landedStudySheets is the study panel with its request answered, with a
// short answer and with an error. Each lands on the request's own goroutine,
// its second fit included, and the sheet is read once that goroutine has
// returned. Neither body fills the room it is given, and each body scrolls,
// so the panel's least height is not its natural height: that is its chrome
// and its whole body, the body as tall as on a window with room to spare.
// The chrome is not compared: the passage's excerpt above the body shows
// fewer lines on a short window.
func landedStudySheets() []desktopSheet {
	land := func(result string, err error) func(*testing.T, *AppState) {
		return func(t *testing.T, s *AppState) {
			prevRun, prevFit := aiActionRun, studyRemeasure
			t.Cleanup(func() { aiActionRun, studyRemeasure = prevRun, prevFit })
			studyRemeasure = func(fit func()) { fit() }
			asked := make(chan context.Context, 1)
			answer := make(chan struct{})
			aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, _ selSpan) (string, error) {
				asked <- ctx
				<-answer
				return result, err
			}
			showAIPanel(s, aiActionExplain, "For God so loved the world", "", selSpan{})
			var ctx context.Context
			select {
			case ctx = <-asked:
			case <-time.After(2 * time.Second):
				t.Fatal("control: the study request never started")
			}
			close(answer)
			// Ended as the request's goroutine returns, once what it got is
			// on the panel.
			<-ctx.Done()
			shown := result
			if err != nil {
				shown = friendlyAIError(err)
			}
			if top := s.window.Canvas().Overlays().Top(); !treeHasText(top, shown) {
				t.Fatalf("control: the panel should show %q; texts %v", shown, treeTexts(top))
			}
		}
	}
	return []desktopSheet{
		{"AI answer, short", land("God gave his Son because he loved the world.", nil)},
		{"AI answer, error", land("", errBudgetExhausted)},
	}
}

// landedCoversAt is, for each window TestACoveringSheetTakesItsNaturalHeight
// lays out, the states in landedStudySheets that cannot clear the header
// there. The error covers it at 401pt as well, where its body is never
// shorter than its message and Try again (ai_panel_error_whole_test.go).
var landedCoversAt = map[fyne.Size][]string{
	{Width: 1280, Height: 386}: {"AI answer, short", "AI answer, error"},
	{Width: 1280, Height: 401}: {"AI answer, error"},
}

// A sheet that covers the header takes its natural height, capped at the
// room from the header's top. A card sized from its content (a link notice,
// the note-link offer, Ask, the note composer, the share confirmation) is as
// tall as its content, with no blank band under its last row; the study panel
// with a short answer or an error shows its whole body, the body as tall as
// on a window with room to spare and no taller; Go to's lists, the
// cross-references and the AI answer's waiting scroll fill the room.
// Grown to the room, at 1280x386 the link notice showed 108.7pt of blank
// under its content and the note-link offer, Ask and the composer 116.7pt;
// at 1280x401, the least height a Mac window's canvas is laid out at, the
// link notice 123.7pt and the note-link offer 131.7pt; the share
// confirmation was 17.3 and 8.3pt taller than its content.
func TestACoveringSheetTakesItsNaturalHeight(t *testing.T) {
	sheets := append(shortWindowSheets(t), landedStudySheets()...)
	landed := map[string]bool{}
	for _, sh := range landedStudySheets() {
		landed[sh.name] = true
	}
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{{Width: 1280, Height: 386}, {Width: 1280, Height: 401}} {
		var covering []string
		for _, sh := range sheets {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, sh.name), func(t *testing.T) {
				app.asNew(t)
				// The height a landed study panel's body takes with room to
				// spare. The body is the panel's first scroll: the border it
				// sits in lists it before the header and the footer.
				var roomyBody float32
				if landed[sh.name] {
					rst, rw := app.window(t, fyne.NewSize(win.Width, 800))
					roomy := pickerPopup(t, rst, func(s *AppState) { sh.open(t, s) })
					if wantClearOrCovering(t, rst, rw, roomy) {
						t.Fatal("control: on a 800pt window the panel should open below the header")
					}
					body := findScroll(roomy)
					if body == nil || body.Offset.Y != 0 || body.Size().Height < body.Content.MinSize().Height {
						t.Fatal("control: with room to spare the panel's body should show all it holds")
					}
					roomyBody = body.Size().Height
					roomy.Hide()
				}
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { sh.open(t, s) })
				defer popup.Hide()
				if !wantClearOrCovering(t, st, w, popup) {
					return
				}
				covering = append(covering, sh.name)
				headerTop := fyne.CurrentApp().Driver().AbsolutePositionForObject(st.header).Y
				foot := w.Canvas().Size().Height - headerTop
				top, bottom := sheetBox(t, popup)
				if landed[sh.name] {
					body := findScroll(popup)
					if body == nil {
						t.Fatal("the panel has no body")
					}
					t.Logf("covering %.1f..%.1f, its body %.1fpt tall, %.1fpt with room to spare; texts %v",
						top, bottom, body.Size().Height, roomyBody, treeTexts(body))
					if math.Abs(float64(body.Size().Height-roomyBody)) > 0.5 {
						t.Errorf("covering the header (%.1f..%.1f), the panel's body is %.1fpt tall, where with room to spare it is %.1fpt",
							top, bottom, body.Size().Height, roomyBody)
					}
					return
				}
				if coveringFills[sh.name] {
					if math.Abs(float64(bottom-foot)) > 0.5 {
						t.Errorf("covering the header from %.1f, the sheet ends at %.1f; its lists should fill the room to %.1f",
							top, bottom, foot)
					}
					return
				}
				blank := popup.Content.Size().Height - popup.Content.MinSize().Height
				t.Logf("covering %.1f..%.1f, %.1fpt beyond its content's height", top, bottom, blank)
				if blank > 1 {
					t.Errorf("covering the header, the card is %.1fpt taller than its content (%.1f..%.1f): a blank band under its last row",
						blank, top, bottom)
				}
			})
		}
		want := append(append([]string{}, coversAt[win]...), landedCoversAt[win]...)
		if got, want := strings.Join(covering, ", "), strings.Join(want, ", "); got != want {
			t.Errorf("%.0fx%.0f: control: the sheets covering the header are [%s], want [%s]", win.Width, win.Height, got, want)
		}
	}
}

// Every sheet clears the header or covers it, and covers it only on the
// windows where it cannot clear it (coversAt). A sheet is judged where it
// settles: a link notice judged before its wrapped paragraph had measured
// itself at the card's width covered the header on 441 to 448pt windows it
// clears, and Ask, the note composer and the share confirmation covered it at
// the one height where they clear it by less than half a point.
func TestDesktopSheetsClearOrCoverTheHeaderOnShortWindows(t *testing.T) {
	sheets := shortWindowSheets(t)
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{
		{Width: 1280, Height: 386}, // the least height the window allows
		{Width: 507, Height: 386},  // and as narrow as its content allows
		{Width: 1280, Height: 392},
		{Width: 1280, Height: 393},
		{Width: 1280, Height: 398},
		{Width: 1280, Height: 399},
		{Width: 1280, Height: 401},
		{Width: 1280, Height: 407},
		{Width: 1280, Height: 408},
		{Width: 1280, Height: 413},
		{Width: 1280, Height: 414},
		{Width: 1280, Height: 418},
		{Width: 1280, Height: 419},
		{Width: 507, Height: 430},
		{Width: 507, Height: 431},
		{Width: 1280, Height: 440},
		{Width: 1280, Height: 442},
		{Width: 1280, Height: 448},
	} {
		want, ok := coversAt[win]
		if !ok {
			t.Fatalf("%.0fx%.0f: no covering sheets listed for this window", win.Width, win.Height)
		}
		var covering []string
		for _, sh := range sheets {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, sh.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { sh.open(t, s) })
				defer popup.Hide()
				if wantClearOrCovering(t, st, w, popup) {
					covering = append(covering, sh.name)
				}
			})
		}
		if got, want := strings.Join(covering, ", "), strings.Join(want, ", "); got != want {
			t.Errorf("%.0fx%.0f: the sheets covering the header are [%s], want [%s]", win.Width, win.Height, got, want)
		}
	}
}

// A sheet covering the header is sized again with the window: shrunk to the
// least height it covers the header, as one opened there does, and given room
// again it opens below the header, as one opened there does. A covering sheet
// the toolkit only re-centred as the window grew would start partway down the
// Go to chip. The windows a link notice clears by less than 4pt are among
// them: one opened there was judged before it settled, and covered the header
// that one sized again there cleared.
func TestASheetCoveringTheHeaderIsSizedAgainWithTheWindow(t *testing.T) {
	sheets := shortWindowSheets(t)
	app := newDesktopApp(t)
	for _, rs := range []struct {
		why      string
		from, to fyne.Size
	}{
		{"shrunk to the least height", fyne.NewSize(1280, 800), fyne.NewSize(1280, 386)},
		{"grown from the least height", fyne.NewSize(1280, 386), fyne.NewSize(1280, 800)},
		{"grown a little", fyne.NewSize(1280, 386), fyne.NewSize(1280, 420)},
		{"shrunk a little", fyne.NewSize(1280, 440), fyne.NewSize(1280, 390)},
		{"grown to just clear", fyne.NewSize(1280, 386), fyne.NewSize(1280, 442)},
		{"shrunk to just clear", fyne.NewSize(1280, 800), fyne.NewSize(1280, 445)},
	} {
		for _, sh := range sheets {
			t.Run(fmt.Sprintf("%.0fx%.0f to %.0fx%.0f/%s", rs.from.Width, rs.from.Height, rs.to.Width, rs.to.Height, sh.name), func(t *testing.T) {
				app.asNew(t)
				fst, _ := app.window(t, rs.to)
				fresh := pickerPopup(t, fst, func(s *AppState) { sh.open(t, s) })
				wantTop, wantBottom := sheetBox(t, fresh)
				fresh.Hide()

				st, w := app.window(t, rs.from)
				popup := pickerPopup(t, st, func(s *AppState) { sh.open(t, s) })
				defer popup.Hide()
				w.Resize(rs.to)
				wantClearOrCovering(t, st, w, popup)
				top, bottom := sheetBox(t, popup)
				if math.Abs(float64(top-wantTop)) > 0.5 || math.Abs(float64(bottom-wantBottom)) > 0.5 {
					t.Errorf("%s: the sheet spans %.1f..%.1f, where one opened at this size spans %.1f..%.1f",
						rs.why, top, bottom, wantTop, wantBottom)
				}
			})
		}
	}
}

// The cross-references stay clear or covering through their load. The waiting
// state's message and bar do not scroll, so on the least window the panel
// covers the header while the list loads; the listed state scrolls, and when
// the list lands the panel is sized for it, below the header, as one opened
// listed is. Nothing but the body changes between the two, so the panel is
// sized again whenever the body changes. Mutation guarded: the landing leaving
// the panel at the covering height the waiting state took.
func TestTheCrossReferencesAreSizedAgainWhenTheirListLands(t *testing.T) {
	desktopSheets(t) // the seams the panel's load runs through
	var land func()
	crossRefsRun = func(work func()) { land = work }
	st, w := desktopWindow(t, fyne.NewSize(1280, 386))
	withBeatitudes(st)
	popup := pickerPopup(t, st, func(s *AppState) { showCrossRefs(s, "", selSpan{lo: 3, hi: 3}) })
	defer popup.Hide()
	if !wantClearOrCovering(t, st, w, popup) {
		t.Fatal("control: waiting on its load, the panel should be too tall to clear the header at 1280x386")
	}
	if land == nil {
		t.Fatal("control: the panel did not start its load")
	}
	land()
	if findScroll(popup) == nil || treeHasText(popup, "Finding related passages…") {
		t.Fatalf("control: the list should have landed; texts %v", treeTexts(popup))
	}
	if wantClearOrCovering(t, st, w, popup) {
		t.Error("with its list, which scrolls, the panel should open below the header, as one opened listed does")
	}
}

// The model picker opens below the header on every window, not on short ones
// alone, and is sized again when the window changes size. Capped by a margin
// alone, a list longer than the window has room for centred the picker to a
// top edge 70pt down the window, 1pt inside the header, whatever the window's
// height.
func TestTheModelPickerOpensBelowTheHeader(t *testing.T) {
	var picker desktopSheet
	for _, sh := range shortWindowSheets(t) {
		if sh.name == "model picker" {
			picker = sh
		}
	}
	if picker.open == nil {
		t.Fatal("control: no model picker among the sheets")
	}
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{{Width: 1280, Height: 860}, {Width: 1280, Height: 800}, {Width: 507, Height: 600}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", win.Width, win.Height), func(t *testing.T) {
			app.asNew(t)
			st, w := app.window(t, win)
			popup := pickerPopup(t, st, func(s *AppState) { picker.open(t, s) })
			defer popup.Hide()
			wantClearOfHeader(t, st, w, popup)
		})
	}
	t.Run("1280x860 to 1280x600", func(t *testing.T) {
		app.asNew(t)
		fst, _ := app.window(t, fyne.NewSize(1280, 600))
		fresh := pickerPopup(t, fst, func(s *AppState) { picker.open(t, s) })
		wantTop, wantBottom := sheetBox(t, fresh)
		fresh.Hide()
		st, w := app.window(t, fyne.NewSize(1280, 860))
		popup := pickerPopup(t, st, func(s *AppState) { picker.open(t, s) })
		defer popup.Hide()
		w.Resize(fyne.NewSize(1280, 600))
		wantClearOfHeader(t, st, w, popup)
		if top, bottom := sheetBox(t, popup); math.Abs(float64(top-wantTop)) > 0.5 || math.Abs(float64(bottom-wantBottom)) > 0.5 {
			t.Errorf("the picker spans %.1f..%.1f, where one opened at this size spans %.1f..%.1f", top, bottom, wantTop, wantBottom)
		}
	})
}
