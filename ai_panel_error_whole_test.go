package bibletext

// The AI answer's error state is whole on short windows.
//
// The message and Ask's question keep their full height wherever a sheet
// can show them. On shorter canvases they scroll, with Try again or Open AI
// settings pinned below the scroll so the action remains visible.

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// aiErrorForm is one way the panel shows an error: the action and question
// asked, the error the request ends with, the button the error offers, and
// the lines its message runs to on a 1280pt window.
type aiErrorForm struct {
	name, action, question string
	err                    error
	button                 string
	lines                  int
}

// errAIUnreachable is a request that never reached the provider, which the
// panel reports in two lines and answers with Try again.
var errAIUnreachable = errors.New("synthetic connection failure")

// aiErrorForms are the fixed study verb, the comparison and Ask failing to
// reach the provider, a two-line message with Try again; Explain with the
// model's allowance spent, a three-line message with Try again; and Explain
// with no key, one line with Open AI settings.
func aiErrorForms() []aiErrorForm {
	return []aiErrorForm{
		{"Explain", aiActionExplain, "", errAIUnreachable, "Try again", 2},
		{"Compare", aiActionCompare, "", errAIUnreachable, "Try again", 2},
		{"Ask", aiActionAsk, aiShortQuestion, errAIUnreachable, "Try again", 2},
		{"Explain, allowance spent", aiActionExplain, "", errBudgetExhausted, "Try again", 3},
		{"Explain, no key", aiActionExplain, "", errNoAPIKey, "Open AI settings", 1},
	}
}

// labelLines is how many lines a label laid out lbl's height tall shows.
func labelLines(lbl *widget.Label) int {
	one := widget.NewLabel("x").MinSize().Height
	line := widget.NewLabel("x\nx").MinSize().Height - one
	return int(math.Round(float64((lbl.Size().Height-one)/line))) + 1
}

// openAIError opens the panel for f on s, about selection, and lands the
// request's error. The panel is read once the request's goroutine has
// returned (finishCapturedStudyRequest).
func openAIError(t *testing.T, reqs chan capturedStudyRequest, s *AppState, f aiErrorForm, selection string) {
	t.Helper()
	showAIPanel(s, f.action, selection, f.question, selSpan{})
	finishCapturedStudyRequest(t, nextCapturedStudyRequest(t, reqs, f.name), "", f.err)
}

// aiErrorIn finds the error's message, its scroll and its pinned button.
func aiErrorIn(t *testing.T, popup *widget.PopUp, f aiErrorForm) (*container.Scroll, *widget.Label, *widget.Button) {
	t.Helper()
	msg := friendlyAIError(f.err)
	var sc *container.Scroll
	var lbl *widget.Label
	var btn *widget.Button
	walkTree(popup, func(o fyne.CanvasObject) {
		s, ok := o.(*container.Scroll)
		if !ok {
			return
		}
		walkTree(s.Content, func(n fyne.CanvasObject) {
			if l, ok := n.(*widget.Label); ok && l.Text == msg {
				sc, lbl = s, l
			}
		})
	})
	btn = findTreeButton(popup, f.button)
	if sc == nil || !sc.Visible() || btn == nil {
		t.Fatalf("control: the panel does not show %q with %s; texts %v", msg, f.button, treeTexts(popup))
	}
	return sc, lbl, btn
}

// aiErrorCuts checks the message against its visible scroll and the pinned
// action against the sheet. On a 1280pt window the message must have the
// form's expected line count.
func aiErrorCuts(t *testing.T, popup *widget.PopUp, f aiErrorForm, wide bool) []string {
	t.Helper()
	sheetTop, sheetBottom := sheetBox(t, popup) // lays the panel out where it settles
	sc, lbl, btn := aiErrorIn(t, popup, f)
	if sc.Offset.Y != 0 {
		t.Fatalf("control: the error column opened scrolled %.1fpt down", sc.Offset.Y)
	}
	if n := labelLines(lbl); wide && n != f.lines {
		t.Fatalf("control: the message runs to %d lines (%.1fpt), not %d", n, lbl.Size().Height, f.lines)
	}
	drv := fyne.CurrentApp().Driver()
	top := drv.AbsolutePositionForObject(sc).Y
	bottom := top + sc.Size().Height
	// What the reader can see of the scroll: its part inside the sheet.
	top, bottom = max(top, sheetTop), min(bottom, sheetBottom)
	var cuts []string
	if y := drv.AbsolutePositionForObject(lbl).Y; y < top-0.5 || y+lbl.Size().Height > bottom+0.5 {
		cuts = append(cuts, fmt.Sprintf("the message spans %.1f..%.1f, where the sheet shows its scroll over %.1f..%.1f (%.1fpt)",
			y, y+lbl.Size().Height, top, bottom, bottom-top))
	}
	cuts = append(cuts, aiErrorActionCuts(t, popup, f, sc, btn)...)
	return cuts
}

// aiErrorActionCuts checks the action independently of scroll position or
// message length, including every footer button below it.
func aiErrorActionCuts(t *testing.T, popup *widget.PopUp, f aiErrorForm, sc *container.Scroll, btn *widget.Button) []string {
	t.Helper()
	sheetTop, sheetBottom := sheetBox(t, popup)
	drv := fyne.CurrentApp().Driver()
	bottom := drv.AbsolutePositionForObject(sc).Y + sc.Size().Height
	var cuts []string
	if findTreeButton(sc.Content, f.button) != nil {
		cuts = append(cuts, f.button+" is inside the message scroll")
	}
	if y := drv.AbsolutePositionForObject(btn).Y; y < sheetTop-0.5 || y+btn.Size().Height > sheetBottom+0.5 || y < bottom-0.5 {
		cuts = append(cuts, fmt.Sprintf("%s spans %.1f..%.1f, outside the sheet's %.1f..%.1f or overlapping its scroll ending %.1f",
			f.button, y, y+btn.Size().Height, sheetTop, sheetBottom, bottom))
	}
	// The footer's buttons stand below the error action, inside the sheet.
	for _, name := range []string{"Report", "Copy", "Close"} {
		b := findTreeButton(popup, name)
		if b == nil {
			t.Fatalf("control: the panel has no %s; texts %v", name, treeTexts(popup))
		}
		if y := drv.AbsolutePositionForObject(b).Y; y < sheetTop-0.5 || y+b.Size().Height > sheetBottom+0.5 {
			cuts = append(cuts, fmt.Sprintf("%s spans %.1f..%.1f, outside the sheet's %.1f..%.1f",
				name, y, y+b.Size().Height, sheetTop, sheetBottom))
		}
		if y := drv.AbsolutePositionForObject(b).Y; y < drv.AbsolutePositionForObject(btn).Y+btn.Size().Height-0.5 {
			cuts = append(cuts, name+" overlaps the pinned error action")
		}
	}
	return cuts
}

// wantAIErrorWhole fails unless the error's message and its button are both
// whole and in sight (aiErrorCuts).
func wantAIErrorWhole(t *testing.T, popup *widget.PopUp, f aiErrorForm, wide bool) {
	t.Helper()
	for _, cut := range aiErrorCuts(t, popup, f, wide) {
		t.Error(cut)
	}
}

// On 1280pt windows 386, 401, 436 and 503pt high, and on those either side of
// where the code before cut the error, the message's lines are whole and its
// button is in sight, and the panel clears the header or covers it. The code
// before cut the two-line message on each 1280pt window from 393 to 436pt (at
// 401pt through its first line) and left Try again out of sight from 393 to
// 503pt, and to 524pt for Ask with a one-line question and 525pt for the
// three-line message; at 386 and 392pt the panel covered the header and
// showed the error whole. Where it covers,
// its body is no taller than the column and the 10pt under it, and where it
// ends above the room's foot its body shows the whole column: a covering
// sheet's natural height. Mutations guarded: the error state given no least,
// as before; a least that stops at the message's foot, which left the button
// out of sight on the windows from 436 to 520pt; and a least that leaves out
// Ask's question, which left Ask's Try again out of sight at 503, 504 and
// 520pt.
func TestTheAIErrorIsWholeOnShortWindows(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	covered := 0
	for _, win := range []fyne.Size{
		{Width: 1280, Height: 386}, {Width: 1280, Height: 392}, {Width: 1280, Height: 393},
		{Width: 1280, Height: 401}, {Width: 1280, Height: 436}, {Width: 1280, Height: 437},
		{Width: 1280, Height: 470}, {Width: 1280, Height: 503}, {Width: 1280, Height: 504},
		{Width: 1280, Height: 520}, {Width: 507, Height: 386}, {Width: 507, Height: 401},
		{Width: 507, Height: 503}, {Width: 1280, Height: 800},
	} {
		for _, f := range aiErrorForms() {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { openAIError(t, reqs, s, f, "For God so loved the world") })
				defer popup.Hide()
				covering := wantClearOrCovering(t, st, w, popup)
				wantAIErrorWhole(t, popup, f, win.Width >= 1280)
				if !covering {
					return
				}
				covered++
				sc, _, _ := aiErrorIn(t, popup, f)
				headerTop := fyne.CurrentApp().Driver().AbsolutePositionForObject(st.header).Y
				foot := w.Canvas().Size().Height - headerTop
				top, bottom := sheetBox(t, popup)
				natural := sc.Content.MinSize().Height + 10
				if sc.Size().Height > natural+0.5 {
					t.Errorf("covering the header (%.1f..%.1f), the error's scroll is %.1fpt tall for %.1fpt of column: a blank band under %s",
						top, bottom, sc.Size().Height, natural, f.button)
				}
				if bottom < foot-0.5 && sc.Size().Height < natural-0.5 {
					t.Errorf("covering the header, the panel ends at %.1f above the room's foot at %.1f, its scroll %.1fpt for %.1fpt of column",
						bottom, foot, sc.Size().Height, natural)
				}
			})
		}
	}
	if covered == 0 {
		t.Fatal("control: the error panel covered the header on none of the windows")
	}
}

// The same over the reading surface Windows and Linux ship, the styled pane:
// the panel's height comes from the header and the window alone, so it is
// the same over either pane. 760pt is the width of a window on an 800x600
// screen.
func TestTheAIErrorIsWholeOverTheStyledPane(t *testing.T) {
	prevPane, prevConsume := useStyledPane, sheetConsumeClosure
	useStyledPane = func() bool { return true }
	sheetConsumeClosure = func() bool { return true }
	t.Cleanup(func() { useStyledPane, sheetConsumeClosure = prevPane, prevConsume })
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{{Width: 1280, Height: 401}, {Width: 1280, Height: 436}, {Width: 1280, Height: 503},
		{Width: 760, Height: 401}, {Width: 760, Height: 480}} {
		for _, f := range aiErrorForms() {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { openAIError(t, reqs, s, f, "For God so loved the world") })
				defer popup.Hide()
				wantClearOrCovering(t, st, w, popup)
				wantAIErrorWhole(t, popup, f, win.Width >= 1280)
			})
		}
	}
}

// The error stays whole when the window changes size under the panel, which
// is sized again with it (sheet_refit.go), with the least it was given.
// Mutation guarded: the refit without the least, which cut the error on each
// resize to 401, 436 and 470pt.
func TestTheAIErrorStaysWholeAsTheWindowChanges(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	for _, rs := range []struct{ from, to fyne.Size }{
		{fyne.NewSize(1280, 800), fyne.NewSize(1280, 401)},
		{fyne.NewSize(1280, 800), fyne.NewSize(1280, 436)},
		{fyne.NewSize(1280, 386), fyne.NewSize(1280, 470)},
		{fyne.NewSize(1280, 401), fyne.NewSize(1280, 800)},
		{fyne.NewSize(1280, 503), fyne.NewSize(1280, 401)},
	} {
		for _, f := range aiErrorForms() {
			t.Run(fmt.Sprintf("%.0fx%.0f to %.0fx%.0f/%s", rs.from.Width, rs.from.Height, rs.to.Width, rs.to.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, rs.from)
				popup := pickerPopup(t, st, func(s *AppState) { openAIError(t, reqs, s, f, "For God so loved the world") })
				defer popup.Hide()
				w.Resize(rs.to)
				wantClearOrCovering(t, st, w, popup)
				wantAIErrorWhole(t, popup, f, true)
			})
		}
	}
}

// The error stays whole as the reader opens the quote under the citation and
// closes it again: each tap sizes the panel again (refitContext), with the
// least the error was given. Mutation guarded: that refit without the least,
// which cut the error at each of these heights, as the code before did.
func TestTheAIErrorStaysWholeAsTheQuoteOpensAndCloses(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	forms := aiErrorForms()
	for _, h := range []float32{401, 436, 470, 503} {
		for _, f := range []aiErrorForm{forms[0], forms[2]} {
			t.Run(fmt.Sprintf("1280x%.0f/%s", h, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, fyne.NewSize(1280, h))
				popup := pickerPopup(t, st, func(s *AppState) { openAIError(t, reqs, s, f, aiLongSelection) })
				defer popup.Hide()
				b := quoteDisclosure(t, popup)
				seen := map[string]bool{}
				for range 2 {
					was := b.Icon
					test.Tap(b)
					if b.Icon == was {
						t.Fatal("control: the tap left the quote as it was")
					}
					step := "closed"
					if b.Icon == iconDisclosureCollapse {
						step = "open"
					}
					seen[step] = true
					t.Run("quote "+step, func(t *testing.T) {
						wantClearOrCovering(t, st, w, popup)
						wantAIErrorWhole(t, popup, f, true)
					})
				}
				if !seen["open"] || !seen["closed"] {
					t.Fatalf("control: two taps showed the quote %v, not both open and closed", seen)
				}
			})
		}
	}
}

// Automatic expansion must leave room for the error's whole column, not
// merely a few answer rows. A long quote previously covered the header at
// 620pt and the no-key form at 480pt even though closing the quote let both
// panels clear it. The disclosure remains available beside the citation.
func TestTheAIErrorClosesTheQuoteBeforeCoveringTheHeader(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	forms := aiErrorForms()
	for _, tc := range []struct {
		height    float32
		form      aiErrorForm
		selection string
	}{
		{620, forms[0], aiLongSelection},
		{620, forms[1], aiLongSelection},
		{620, forms[2], aiLongSelection},
		{620, forms[3], aiLongSelection},
		{480, forms[4], "For God so loved the world"},
	} {
		t.Run(fmt.Sprintf("1280x%.0f/%s", tc.height, tc.form.name), func(t *testing.T) {
			app.asNew(t)
			st, w := app.window(t, fyne.NewSize(1280, tc.height))
			popup := pickerPopup(t, st, func(s *AppState) {
				openAIError(t, reqs, s, tc.form, tc.selection)
			})
			defer popup.Hide()
			if quoteDisclosure(t, popup).Icon != iconDisclosureExpand {
				t.Error("the quotation opened without room for the error column")
			}
			if wantClearOrCovering(t, st, w, popup) {
				t.Error("the error covers the header although closing the quotation leaves room")
			}
			wantAIErrorWhole(t, popup, tc.form, true)
		})
	}
}

// An explicit disclosure choice overrides automatic fitting for this panel.
// Waiting, failure and retry share the same choice, including after refits;
// the reader may still inspect the quotation while an error is shown.
func TestTheAIQuoteChoiceSurvivesFailureAndRetry(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	for _, expanded := range []bool{false, true} {
		t.Run(fmt.Sprintf("expanded=%v", expanded), func(t *testing.T) {
			app.asNew(t)
			st, w := app.window(t, fyne.NewSize(1280, 620))
			popup := pickerPopup(t, st, func(s *AppState) {
				showAIPanel(s, aiActionExplain, aiLongSelection, "", selSpan{})
			})
			defer popup.Hide()
			request := nextCapturedStudyRequest(t, reqs, "explicit quotation choice")
			b := quoteDisclosure(t, popup)
			wantIcon := iconDisclosureExpand
			if expanded {
				wantIcon = iconDisclosureCollapse
			}
			// At least one tap records a choice even when the automatic state
			// already matches it; a second tap restores that starting state.
			test.Tap(b)
			if b.Icon != wantIcon {
				test.Tap(b)
			}
			wantChoice := func() {
				t.Helper()
				if b.Icon != wantIcon {
					t.Errorf("the panel lost its explicit expanded=%v choice", expanded)
				}
			}
			wantChoice()
			wantWaitingLineWhole(t, popup)
			finishCapturedStudyRequest(t, request, "", errAIUnreachable)
			wantChoice()
			wantAIErrorWhole(t, popup, aiErrorForms()[0], true)
			for _, height := range []float32{820, 620} {
				w.Resize(fyne.NewSize(1280, height))
				refitSheets(st)
				wantChoice()
			}
			test.Tap(findTreeButton(popup, "Try again"))
			request = nextCapturedStudyRequest(t, reqs, "retry with quotation choice")
			wantChoice()
			wantWaitingLineWhole(t, popup)
			finishCapturedStudyRequest(t, request, "", errAIUnreachable)
			wantChoice()
			wantAIErrorWhole(t, popup, aiErrorForms()[0], true)
		})
	}
}

// A question too long for the window to show the error under it gives the
// panel all the height a sheet may have there, and no more, as it does the
// waiting line (TestAQuestionTooLongForTheWindowKeepsTheWaitingPanelOnIt):
// the panel covers the header and ends as far above the window's foot. The
// scroll opens at its top, with the head of the question in sight, so the
// error under it can still be out of sight, kept as it is. The quotation
// closes automatically to leave the question and error the available room.
// The code before cleared the header with the error out of sight. Mutations
// guarded: the error state given no least, as before; and the least not held
// to the most a sheet may be, which took the panel to the window's top and
// foot.
func TestAQuestionTooLongForTheWindowKeepsTheErrorPanelOnIt(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	long := strings.TrimSpace(strings.Repeat("Who is meant by the world here, and how far does the love reach? ", 36))
	f := aiErrorForm{"Ask, a long question", aiActionAsk, long, errAIUnreachable, "Try again", 2}
	for _, h := range []float32{401, 503} {
		t.Run(fmt.Sprintf("1280x%.0f", h), func(t *testing.T) {
			app.asNew(t)
			st, w := app.window(t, fyne.NewSize(1280, h))
			popup := pickerPopup(t, st, func(s *AppState) { openAIError(t, reqs, s, f, "For God so loved the world") })
			defer popup.Hide()
			if quoteDisclosure(t, popup).Icon != iconDisclosureExpand {
				t.Error("the quotation opened while the question and error needed its room")
			}
			if !wantClearOrCovering(t, st, w, popup) {
				t.Fatal("the panel clears the header with the error out of sight, where covering it would show more")
			}
			headerTop := fyne.CurrentApp().Driver().AbsolutePositionForObject(st.header).Y
			most := w.Canvas().Size().Height - 2*headerTop
			if top, bottom := sheetBox(t, popup); bottom-top < most-0.5 {
				t.Errorf("covering the header, the panel spans %.1f..%.1f, %.1fpt, where a sheet may take %.1fpt",
					top, bottom, bottom-top, most)
			}
			if h == 401 && len(aiErrorCuts(t, popup, f, true)) == 0 {
				t.Error("control: the error is whole here, so the question is not too long for the window")
			}
		})
	}
}

// aiErrorCutOn is, for each screen TestTheAIErrorKeepsItsRuleOnPhones lays
// out, the forms whose message and question are taller than the message
// viewport the largest permitted sheet leaves. The action remains visible.
// On the iPhone 16 Pro held sideways, automatically closing the quotation
// leaves the error whole for every form.
// The 732x240 canvas is laid out as before the change, the sheet filling it.
var aiErrorCutOn = map[string][]string{
	"iPhone SE (1st generation), landscape": {"Explain", "Compare", "Ask",
		"Explain, allowance spent"},
	"640x320 under a 24pt status bar": {"Explain", "Compare", "Ask", "Explain, allowance spent",
		"Explain, no key"},
	"732x240 split screen": {"Explain", "Compare", "Ask", "Explain, allowance spent",
		"Explain, no key"},
}

// The same rule on a phone held sideways, and on a phone's smallest canvases:
// the panel stays inside the safe area, and the error is whole, or, where
// even the most a sheet may be is too short for its column (aiErrorCutOn),
// the panel takes that most. The code before cut the button on the iPhone 17
// Pro Max for Ask and the three-line message, on the iPhone 16 Pro and the
// Android phone for every form, and on the iPhone SE and the 640x320 screen
// for every form with its message as well, the panel there 240pt tall where a
// sheet may be 320 and 272pt; on the 732x240 canvas it filled the canvas, as
// it does now.
func TestTheAIErrorKeepsItsRuleOnPhones(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	for _, s := range []insetScreen{
		proMaxSideways,
		{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
		{"iPhone SE (1st generation), landscape", 568, 320, 0, 0, 0, 0},
		{"Android phone, landscape, cutout on the left", 803, 360, 0, 0, 21, 0},
		{"640x320 under a 24pt status bar", 640, 320, 24, 0, 0, 0},
		{"732x240 split screen", 732, 240, 0, 0, 0, 0},
	} {
		cut := map[string]bool{}
		for _, name := range aiErrorCutOn[s.name] {
			cut[name] = true
		}
		for _, f := range aiErrorForms() {
			t.Run(s.name+"/"+f.name, func(t *testing.T) {
				st, w := insetWindow(t, s, 0)
				openAIError(t, reqs, st, f, "For God so loved the world")
				popup := topPopup(t, w)
				top, bottom := wantInsideTheSafeArea(t, s, popup)
				if !cut[f.name] {
					wantAIErrorWhole(t, popup, f, false)
					return
				}
				if most := s.h - 2*max(s.top, s.bottom); bottom-top < most-0.5 {
					t.Errorf("the panel is %.1fpt tall with its error cut, where a sheet centred in the safe area may be %.1fpt",
						bottom-top, most)
				}
				cuts := aiErrorCuts(t, popup, f, false)
				if len(cuts) == 0 {
					t.Errorf("control: the error is whole here, where aiErrorCutOn records it cut")
				}
				t.Logf("the panel takes the most a sheet may be, %.1f..%.1f; %v", top, bottom, cuts)
			})
		}
	}
}

// Short mobile canvases keep the error action visible even when a long
// question or an explicitly opened quotation leaves the message scrolling.
func TestTheAIErrorActionStaysVisibleOnLandscapePhones(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	long := strings.TrimSpace(strings.Repeat("What does this passage mean in its chapter? ", 36))
	forms := append(aiErrorForms(), aiErrorForm{"Ask, long question", aiActionAsk, long, errAIUnreachable, "Try again", 2})
	for _, screen := range []insetScreen{
		{"Android QA phone, landscape", 914, 411, 24, 24, 0, 0},
		{"800x360 phone, landscape", 800, 360, 0, 0, 21, 0},
		{"667x375 phone, landscape", 667, 375, 0, 0, 0, 0},
	} {
		for _, size := range []string{"normal", "xl"} {
			for _, f := range forms {
				t.Run(screen.name+"/"+size+"/"+f.name, func(t *testing.T) {
					st, w := insetWindow(t, screen, 0)
					setReadingTextSizeID(size)
					openAIError(t, reqs, st, f, aiLongSelection)
					popup := topPopup(t, w)
					defer popup.Hide()
					wantInsideTheSafeArea(t, screen, popup)
					sc, _, btn := aiErrorIn(t, popup, f)
					wantAction := func() {
						t.Helper()
						for _, cut := range aiErrorActionCuts(t, popup, f, sc, btn) {
							t.Error(cut)
						}
						if btn.Size().Height < btn.MinSize().Height {
							t.Error("the error action was shrunk below its normal control height")
						}
						if sc.Content.MinSize().Height <= sc.Size().Height {
							wantAIErrorWhole(t, popup, f, false)
						}
					}
					wantAction()
					if f.question == long {
						if sc.Content.MinSize().Height <= sc.Size().Height || sc.Direction != fyne.ScrollVerticalOnly {
							t.Fatal("control: the long question does not require vertical scrolling")
						}
						sc.ScrollToBottom()
						if sc.Offset.Y <= 0 {
							t.Fatal("the long question and error cannot be scrolled")
						}
						wantAction()
						sc.ScrollToTop()
					}
					b := quoteDisclosure(t, popup)
					if b.Icon != iconDisclosureExpand {
						t.Error("the long quotation opened automatically on the short screen")
					}
					test.Tap(b)
					wantAction()
				})
			}
		}
	}
}
