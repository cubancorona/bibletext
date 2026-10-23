package bibletext

// The AI answer's waiting line is whole on every window.
//
// While a study request runs, the panel's body is a scroll holding the
// waiting column: "Reading the passage…", the bar, the model line, the hint
// and Cancel, with Ask's question above them. The scroll was sized from the
// room the panel had left once its title, citation and footer were placed,
// down to the scroll's own 32pt least. On a desktop window too short for the
// whole column but tall enough for the panel to clear the header, that left a
// scroll too short to show the waiting line, which it cut through the middle:
// at 1280x401, the least height a Mac or Linux window lays out, the scroll was
// 39.6pt tall for a line 38.3pt tall that starts 15pt down it. A phone held
// sideways on a 320pt screen cut it the same way. The body is
// now never shorter than the column down to the foot of the waiting line, the
// question above it included; where the panel then cannot clear the header it
// covers it, as every desktop sheet does (coverHeaderUnlessClear), and it is
// never taller than the window gives a sheet (sheetMostHeight).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// aiWaitingLine is the waiting state's first line.
const aiWaitingLine = "Reading the passage…"

// aiWaitingForm is one way the panel waits: a fixed study verb, the
// comparison, and Ask with the reader's question above the waiting column.
type aiWaitingForm struct {
	name string
	open func(*testing.T, *AppState)
}

// aiWaitingForms opens each form on a request parked for the rest of the
// test, so the panel stays waiting and nothing lands on another goroutine.
func aiWaitingForms(studies chan context.Context, question string) []aiWaitingForm {
	const quote = "For God so loved the world"
	form := func(name, action, q string) aiWaitingForm {
		return aiWaitingForm{name, func(t *testing.T, s *AppState) {
			showAIPanel(s, action, quote, q, selSpan{})
			waitParked(t, studies, "the study request")
		}}
	}
	return []aiWaitingForm{
		form("Explain", aiActionExplain, ""),
		form("Compare", aiActionCompare, ""),
		form("Ask", aiActionAsk, question),
	}
}

// aiShortQuestion is a question of one line at every width these tests use.
const aiShortQuestion = "What does the world mean here, and who is meant by it?"

// waitingLineIn finds the waiting line and the scroll that holds it.
func waitingLineIn(t *testing.T, popup *widget.PopUp) (*container.Scroll, *widget.Label) {
	t.Helper()
	var sc *container.Scroll
	var line *widget.Label
	walkTree(popup, func(o fyne.CanvasObject) {
		if s, ok := o.(*container.Scroll); ok {
			walkTree(s.Content, func(n fyne.CanvasObject) {
				if l, ok := n.(*widget.Label); ok && l.Text == aiWaitingLine {
					sc, line = s, l
				}
			})
		}
	})
	if sc == nil || !sc.Visible() {
		t.Fatalf("control: the panel is not waiting; texts %v", treeTexts(popup))
	}
	return sc, line
}

// wantWaitingLineWhole fails unless the waiting line lies wholly inside the
// part of its scroll on screen, with the scroll at its top, as it opens.
func wantWaitingLineWhole(t *testing.T, popup *widget.PopUp) {
	t.Helper()
	sheetBox(t, popup) // lays the panel out where it settles
	sc, line := waitingLineIn(t, popup)
	if sc.Offset.Y != 0 {
		t.Fatalf("control: the waiting column opened scrolled %.1fpt down", sc.Offset.Y)
	}
	drv := fyne.CurrentApp().Driver()
	top := drv.AbsolutePositionForObject(sc).Y
	bottom := top + sc.Size().Height
	y := drv.AbsolutePositionForObject(line).Y
	if y < top-0.5 || y+line.Size().Height > bottom+0.5 {
		t.Errorf("%q spans %.1f..%.1f, where its scroll shows %.1f..%.1f (%.1fpt tall)",
			aiWaitingLine, y, y+line.Size().Height, top, bottom, bottom-top)
	}
}

// On every desktop window from the least the test window allows to 440pt,
// and at the least a real window lays out (401pt), the waiting line is whole
// and the panel clears the header or covers it. The code before cut the line
// on each 1280pt window from 393 to 414pt high, 401 among them, and to 434pt
// for Ask with its one-line question; at 386pt the panel already covered the
// header and showed the line whole.
func TestTheAIWaitingLineIsWholeOnEveryWindow(t *testing.T) {
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	var windows []fyne.Size
	for h := float32(386); h <= 440; h++ {
		windows = append(windows, fyne.NewSize(1280, h))
	}
	windows = append(windows, fyne.NewSize(507, 386), fyne.NewSize(507, 401), fyne.NewSize(507, 420),
		fyne.NewSize(1280, 800))
	for _, win := range windows {
		for _, f := range aiWaitingForms(studies, aiShortQuestion) {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { f.open(t, s) })
				defer popup.Hide()
				wantClearOrCovering(t, st, w, popup)
				wantWaitingLineWhole(t, popup)
			})
		}
	}
}

// Covering the header, the waiting panel is as tall as its whole column
// needs, capped at the room from the header's top, and never so short that
// the waiting line is cut: a covering sheet's natural height
// (coverHeaderUnlessClear's preferred height) and the waiting line's least
// (fitBody's least) hold together. On each 1280pt window from 386 to 440pt
// where the panel covers, its scroll is no taller than the column and the
// 10pt under it, so no blank band stands under Cancel, and where the panel
// ends above the room's foot the scroll shows the whole column; the panel's
// popup, which draws the shade, spans the window (wantClearOrCovering). The
// column is taller than the room wherever the panel covers, to 409pt for
// Explain and Compare and 430pt for Ask here, so covering it fills the room.
// Mutations guarded: the panel covering with no preferred height, as a card
// does, which stopped it at its least, above the room's foot with the column
// cut short; and the body's least left out, which cut the waiting line from
// 393 to 414pt, and to 434pt for Ask.
func TestACoveringWaitingPanelTakesItsNaturalHeightWithItsLineWhole(t *testing.T) {
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	covered := 0
	for h := float32(386); h <= 440; h++ {
		win := fyne.NewSize(1280, h)
		for _, f := range aiWaitingForms(studies, aiShortQuestion) {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { f.open(t, s) })
				defer popup.Hide()
				covering := wantClearOrCovering(t, st, w, popup)
				wantWaitingLineWhole(t, popup)
				if !covering {
					return
				}
				covered++
				sc, _ := waitingLineIn(t, popup)
				headerTop := fyne.CurrentApp().Driver().AbsolutePositionForObject(st.header).Y
				foot := w.Canvas().Size().Height - headerTop
				top, bottom := sheetBox(t, popup)
				natural := sc.Content.MinSize().Height + 10
				t.Logf("covering %.1f..%.1f (room to %.1f), scroll %.1fpt for a column of %.1fpt and 10pt under it",
					top, bottom, foot, sc.Size().Height, sc.Content.MinSize().Height)
				if sc.Size().Height > natural+0.5 {
					t.Errorf("covering the header (%.1f..%.1f), the waiting scroll is %.1fpt tall for %.1fpt of column: a blank band under Cancel",
						top, bottom, sc.Size().Height, natural)
				}
				if bottom < foot-0.5 && sc.Size().Height < natural-0.5 {
					t.Errorf("covering the header, the panel ends at %.1f above the room's foot at %.1f, its scroll %.1fpt for %.1fpt of column",
						bottom, foot, sc.Size().Height, natural)
				}
			})
		}
	}
	if covered == 0 {
		t.Fatal("control: the waiting panel covered the header on none of the windows")
	}
}

// The same over the reading surface Windows and Linux ship, the styled pane,
// which the window here lays out whatever the host: the panel's height comes
// from the header and the window alone, so it is the same over either pane.
// 760pt is the width of a window on an 800x600 screen.
func TestTheAIWaitingLineIsWholeOverTheStyledPane(t *testing.T) {
	prevPane, prevConsume := useStyledPane, sheetConsumeClosure
	useStyledPane = func() bool { return true }
	sheetConsumeClosure = func() bool { return true }
	t.Cleanup(func() { useStyledPane, sheetConsumeClosure = prevPane, prevConsume })
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{{Width: 1280, Height: 401}, {Width: 1280, Height: 413}, {Width: 1280, Height: 414},
		{Width: 760, Height: 401}, {Width: 760, Height: 420}} {
		for _, f := range aiWaitingForms(studies, aiShortQuestion) {
			t.Run(fmt.Sprintf("%.0fx%.0f/%s", win.Width, win.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { f.open(t, s) })
				defer popup.Hide()
				wantClearOrCovering(t, st, w, popup)
				wantWaitingLineWhole(t, popup)
			})
		}
	}
}

// The line stays whole when the window changes size under the waiting panel,
// which is sized again with it (sheet_refit.go).
func TestTheAIWaitingLineStaysWholeAsTheWindowChanges(t *testing.T) {
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	for _, rs := range []struct{ from, to fyne.Size }{
		{fyne.NewSize(1280, 800), fyne.NewSize(1280, 401)},
		{fyne.NewSize(1280, 386), fyne.NewSize(1280, 401)},
		{fyne.NewSize(1280, 440), fyne.NewSize(1280, 405)},
		{fyne.NewSize(1280, 401), fyne.NewSize(1280, 386)},
		{fyne.NewSize(1280, 401), fyne.NewSize(1280, 800)},
	} {
		for _, f := range aiWaitingForms(studies, aiShortQuestion) {
			t.Run(fmt.Sprintf("%.0fx%.0f to %.0fx%.0f/%s", rs.from.Width, rs.from.Height, rs.to.Width, rs.to.Height, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, rs.from)
				popup := pickerPopup(t, st, func(s *AppState) { f.open(t, s) })
				defer popup.Hide()
				w.Resize(rs.to)
				wantClearOrCovering(t, st, w, popup)
				wantWaitingLineWhole(t, popup)
			})
		}
	}
}

// aiLongSelection is a selection several lines long, which the citation
// shows in a quote box the reader can open and close.
var aiLongSelection = strings.TrimSpace(strings.Repeat(
	"In the beginning was the Word, and the Word was with God, and the Word was God. ", 6))

// quoteDisclosure is the citation's button that opens and closes the quote.
func quoteDisclosure(t *testing.T, popup *widget.PopUp) *widget.Button {
	t.Helper()
	var b *widget.Button
	walkTree(popup, func(o fyne.CanvasObject) {
		if bt, ok := o.(*widget.Button); ok && bt.Text == "" &&
			(bt.Icon == iconDisclosureExpand || bt.Icon == iconDisclosureCollapse) {
			b = bt
		}
	})
	if b == nil {
		t.Fatalf("control: the citation has no disclosure; texts %v", treeTexts(popup))
	}
	return b
}

// A long question can make the waiting line need more than the few answer
// rows normally reserved under an open quotation. Automatic expansion uses
// that state's minimum too; the subsequent error recalculates its own need.
func TestTheAIWaitingQuoteClosesWhenItsQuestionNeedsTheRoom(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	app := newDesktopApp(t)
	st, _ := app.window(t, fyne.NewSize(1280, 503))
	question := strings.TrimSpace(strings.Repeat("Who is meant by the world here, and how far does the love reach? ", 18))
	popup := pickerPopup(t, st, func(s *AppState) {
		showAIPanel(s, aiActionAsk, aiLongSelection, question, selSpan{})
	})
	defer popup.Hide()
	request := nextCapturedStudyRequest(t, reqs, "waiting with a long question")
	if quoteDisclosure(t, popup).Icon != iconDisclosureExpand {
		t.Error("the quotation opened while the waiting column needed its room")
	}
	finishCapturedStudyRequest(t, request, "", errAIUnreachable)
	if quoteDisclosure(t, popup).Icon != iconDisclosureExpand {
		t.Error("the quotation reopened while the error column needed its room")
	}
}

// The line stays whole as the reader opens the quote under the citation and
// closes it again while the panel waits: each tap sizes the panel again
// (refitContext), with the least it opened with. Mutation guarded: that
// refit without the least, which cut the line once the quote was closed at
// 401 and 410pt and once it was opened at 430 and 440pt. The code before cut
// it at each of these heights.
func TestTheAIWaitingLineStaysWholeAsTheQuoteOpensAndCloses(t *testing.T) {
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	for _, h := range []float32{401, 410, 430, 440} {
		for _, f := range []struct{ name, action, question string }{
			{"Explain", aiActionExplain, ""},
			{"Ask", aiActionAsk, aiShortQuestion},
		} {
			t.Run(fmt.Sprintf("1280x%.0f/%s", h, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, fyne.NewSize(1280, h))
				popup := pickerPopup(t, st, func(s *AppState) {
					showAIPanel(s, f.action, aiLongSelection, f.question, selSpan{})
					waitParked(t, studies, "the study request")
				})
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
						wantWaitingLineWhole(t, popup)
					})
				}
				if !seen["open"] || !seen["closed"] {
					t.Fatalf("control: two taps showed the quote %v, not both open and closed", seen)
				}
			})
		}
	}
}

// The line stays whole when the request names a model too long for one line,
// which makes the column taller and sizes the panel again
// (modelLine.relayout), with the least it opened with. Mutation guarded: that
// relayout without the least, which cut the line at 401 and 410pt, and for
// Ask at 420pt, as the code before did.
func TestTheAIWaitingLineStaysWholeUnderAWrappedModelName(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	studies := stubAIActionParked(t)
	app := newDesktopApp(t)
	for _, h := range []float32{401, 410, 420} {
		for _, f := range []struct{ name, action, question string }{
			{"Explain", aiActionExplain, ""},
			{"Ask", aiActionAsk, aiShortQuestion},
		} {
			t.Run(fmt.Sprintf("1280x%.0f/%s", h, f.name), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, fyne.NewSize(1280, h))
				var ctx context.Context
				popup := pickerPopup(t, st, func(s *AppState) {
					showAIPanel(s, f.action, "For God so loved the world", f.question, selSpan{})
					ctx = nextCtx(t, studies)
				})
				defer popup.Hide()
				reportAIModel(ctx, providerGemini, long)
				wantModelLine(t, popup, label(providerGemini, long), "the long name reported")
				if box, _ := modelLineBox(popup); box.MinSize().Height < 1.5*captionLine() {
					t.Fatalf("control: the model line is %.1fpt tall: the long name did not wrap", box.MinSize().Height)
				}
				wantClearOrCovering(t, st, w, popup)
				wantWaitingLineWhole(t, popup)
			})
		}
	}
}

// The same rule on a phone held sideways: the line is whole, and the panel
// stays inside the safe area. The code before cut the line on the 320pt-tall
// screen of the smallest iPhone iOS 15 runs on.
func TestTheAIWaitingLineIsWholeOnPhonesHeldSideways(t *testing.T) {
	studies := stubAIActionParked(t)
	for _, s := range []insetScreen{
		proMaxSideways,
		{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
		{"iPhone SE (1st generation), landscape", 568, 320, 0, 0, 0, 0},
		{"Android phone, landscape, cutout on the left", 803, 360, 0, 0, 21, 0},
	} {
		for _, f := range aiWaitingForms(studies, aiShortQuestion) {
			t.Run(s.name+"/"+f.name, func(t *testing.T) {
				st, w := insetWindow(t, s, 0)
				f.open(t, st)
				popup := topPopup(t, w)
				wantInsideTheSafeArea(t, s, popup)
				wantWaitingLineWhole(t, popup)
			})
		}
	}
}

// wantInsideTheSafeArea fails unless the sheet lies between the safe area's
// top and bottom edges.
func wantInsideTheSafeArea(t *testing.T, s insetScreen, popup *widget.PopUp) (top, bottom float32) {
	t.Helper()
	top, bottom = sheetBox(t, popup)
	if top < s.top-0.5 || bottom > s.h-s.bottom+0.5 {
		t.Errorf("the sheet spans %.1f..%.1f, outside the safe area %.1f..%.1f", top, bottom, s.top, s.h-s.bottom)
	}
	return top, bottom
}

// A question too long for the window to show the waiting line under it
// gives the panel all the height a sheet may have there, and no more: on a
// desktop window the panel covers the header and ends as far above the
// window's foot, and on a phone it takes as much of the safe area as a sheet
// centred on the screen may, from the deeper of its top and bottom insets in
// from each edge. Mutations guarded: the body's least height not held to what
// the window gives a sheet, which took the panel on a desktop window to the
// window's top and foot, over the band above the header that a covering sheet
// leaves; and that most height taken from the canvas, or from its top inset
// alone, which took the panel on an iPhone 17 Pro Max held sideways to
// 0..440pt, over the 20pt under the home indicator.
func TestAQuestionTooLongForTheWindowKeepsTheWaitingPanelOnIt(t *testing.T) {
	studies := stubAIActionParked(t)
	long := strings.TrimSpace(strings.Repeat("Who is meant by the world here, and how far does the love reach? ", 18))
	ask := aiWaitingForms(studies, long)[2]
	t.Run("desktop", func(t *testing.T) {
		app := newDesktopApp(t)
		for _, win := range []fyne.Size{{Width: 1280, Height: 386}, {Width: 1280, Height: 401}, {Width: 1280, Height: 440}} {
			t.Run(fmt.Sprintf("%.0fx%.0f", win.Width, win.Height), func(t *testing.T) {
				app.asNew(t)
				st, w := app.window(t, win)
				popup := pickerPopup(t, st, func(s *AppState) { ask.open(t, s) })
				defer popup.Hide()
				if !wantClearOrCovering(t, st, w, popup) {
					t.Error("the panel clears the header with the waiting line out of sight, where covering it would show more")
				}
			})
		}
	})
	t.Run("phone", func(t *testing.T) {
		for _, s := range []insetScreen{
			{"iPhone SE (1st generation), landscape", 568, 320, 0, 0, 0, 0},
			proMaxSideways,
		} {
			t.Run(s.name, func(t *testing.T) {
				st, w := insetWindow(t, s, 0)
				ask.open(t, st)
				popup := topPopup(t, w)
				most := s.h - 2*max(s.top, s.bottom)
				if top, bottom := wantInsideTheSafeArea(t, s, popup); bottom-top < most-0.5 {
					t.Errorf("the panel is %.1fpt tall with the waiting line out of sight, where a sheet centred in the safe area may be %.1fpt",
						bottom-top, most)
				}
			})
		}
	})
}
