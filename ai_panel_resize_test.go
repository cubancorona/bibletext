package bibletext

import (
	"errors"
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func mobileStudyResizeWindow(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	fyne.SetCurrentApp(phoneTestApp{app})
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(744, 1133))
	state := studyPanelState(t, w)
	w.SetContent(windowRoot(state, widget.NewLabel("Synthetic reading pane")))
	return state, w
}

func assertMobileStudyMargins(t *testing.T, w fyne.Window, popup *widget.PopUp) {
	t.Helper()
	box := sheetBoxOn(t, popup)
	limit := aiPanelSize(w.Canvas().Size())
	// At the narrowest width the footer's minimum can add two points, just
	// as it does on a freshly opened card; the canvas margins still hold.
	if math.Abs(float64(box.x1-box.x0-limit.Width)) > 2.5 || box.y1-box.y0 > limit.Height+0.5 {
		t.Errorf("card spans %v, beyond the current canvas's %v reading size", box, limit)
	}
	if box.x0 < 22.5 || box.x1 > w.Canvas().Size().Width-22.5 || box.y0 < 39.5 || box.y1 > w.Canvas().Size().Height-39.5 {
		t.Errorf("card lost its margins in the resized window: %v on %v", box, w.Canvas().Size())
	}
	for _, name := range []string{"Copy", "Report", "Close"} {
		button := findTreeButton(popup, name)
		if button == nil {
			t.Fatalf("the card lost %s", name)
		}
		pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(button)
		if pos.X < box.x0 || pos.Y < box.y0 || pos.X+button.Size().Width > box.x1+0.5 || pos.Y+button.Size().Height > box.y1+0.5 {
			t.Errorf("%s escaped the resized card: %v/%v within %v", name, pos, button.Size(), box)
		}
	}
}

func TestStudyPanelMobileResizeIgnoresKeyboardAndClosedCards(t *testing.T) {
	state, w := mobileStudyResizeWindow(t)
	requests := stubCapturedStudyRequests(t)
	showAIPanelForPassage(state, aiActionAsk, syntheticAskPassage(), "What does the path mean?", selSpan{})
	request := nextCapturedStudyRequest(t, requests, "keyboard-safe answer card")
	finishCapturedStudyRequest(t, request, strings.Repeat("A synthetic answer paragraph describes the path and river.\n\n", 40), nil)
	popup := topPopup(t, w)
	body := askAnswerBody(t, popup)
	answer := askAnswerRichText(t, popup)
	var counter *answerRefreshCounter
	for i, segment := range answer.Segments {
		if text, ok := segment.(*widget.TextSegment); ok {
			counter = &answerRefreshCounter{TextSegment: *text}
			answer.Segments[i] = counter
			break
		}
	}
	if counter == nil {
		t.Fatal("the answer must have a measurable text segment")
	}
	answer.Refresh()
	body.ScrollToOffset(fyne.NewPos(0, 80))
	boxSize, bodySize, offset := popup.Content.Size(), body.Size(), body.Offset
	rawCanvas := w.Canvas().Size()
	counter.updates = 0
	for _, height := range []float32{rawCanvas.Height - 260, rawCanvas.Height} {
		// A keyboard changes the root's usable height without resizing the
		// raw canvas. This still exercises the root's refit dispatch.
		w.Content().Resize(fyne.NewSize(rawCanvas.Width, height))
		if w.Canvas().Size() != rawCanvas || popup.Content.Size() != boxSize || body.Size() != bodySize || body.Offset != offset || counter.updates != 0 {
			t.Error("a keyboard-only layout change refit the answer card")
		}
	}
	test.Tap(findTreeButton(popup, "Close"))
	counter.updates = 0
	w.Resize(fyne.NewSize(375, 602.5))
	if len(state.sheetRefits) != 0 || counter.updates != 0 {
		t.Error("resizing retained or refit a closed answer card")
	}
}

func TestStudyPanelMobileResizePreservesAnswerAndQuotation(t *testing.T) {
	for _, action := range []string{aiActionAsk, aiActionCompare} {
		for _, expanded := range []bool{true, false} {
			name := "collapsed"
			if expanded {
				name = "expanded"
			}
			t.Run(action+"/"+name, func(t *testing.T) {
				state, w := mobileStudyResizeWindow(t)
				requests := stubCapturedStudyRequests(t)
				passage := syntheticAskPassage()
				passage.Ref, passage.VersionAbbrev = "2 Thessalonians 2:1–17", "SYNTHETIC-LONG"
				passage.PreviewText = strings.Repeat("A synthetic quotation follows the path to the river. ", 40)
				passage.Text = passage.PreviewText
				question := "How does this synthetic passage describe the path?"
				showAIPanelForPassage(state, action, passage, question, selSpan{})
				request := nextCapturedStudyRequest(t, requests, "resizable study card")
				answer := strings.Repeat("A synthetic answer paragraph describes the path and the river.\n\n", 40)
				finishCapturedStudyRequest(t, request, answer, nil)
				popup := topPopup(t, w)
				body := askAnswerBody(t, popup)
				rich := studyAnswerContent(t, popup)
				rendered := studyAnswerText(rich)
				excerpt, _ := noteSheetExcerpt(t, popup)
				context := scrollHolding(popup, excerpt)
				if context == nil {
					t.Fatal("the expanded quotation lost its scroll")
				}
				if !expanded {
					test.Tap(studyDisclosureButton(popup, true))
				} else {
					// Make expansion an explicit choice rather than an automatic fit.
					test.Tap(studyDisclosureButton(popup, true))
					test.Tap(studyDisclosureButton(popup, false))
					context.ScrollToOffset(fyne.NewPos(0, 40))
				}
				body.ScrollToOffset(fyne.NewPos(0, 80))
				offset, contextOffset := body.Offset, context.Offset
				if offset.Y == 0 || expanded && contextOffset.Y == 0 {
					t.Fatal("the reading offsets must exercise scrolled content")
				}
				for _, size := range []fyne.Size{{Width: 375, Height: 602.5}, {Width: 540, Height: 850}, {Width: 744, Height: 1133}} {
					w.Resize(size)
					assertMobileStudyMargins(t, w, popup)
					if topPopup(t, w) != popup || askAnswerBody(t, popup) != body || studyAnswerContent(t, popup) != rich {
						t.Fatal("resizing replaced the answer card or its reading area")
					}
					if studyDisclosureButton(popup, expanded) == nil || studyObjectVisible(popup, excerpt) != expanded {
						t.Error("resizing changed the explicit quotation choice")
					}
					if body.Offset != offset || expanded && context.Offset != contextOffset {
						t.Errorf("resizing changed reading offsets: answer %v→%v, quote %v→%v", offset, body.Offset, contextOffset, context.Offset)
					}
					if body.Size().Height < 80 || studyAnswerText(rich) != rendered || !sheetHas(popup, expectedPreviewCitation(passage.Ref, passage.VersionAbbrev)) {
						t.Error("resizing lost reading space, the complete answer, or the long citation")
					}
					body.ScrollToBottom()
					if body.Content.Size().Height-body.Offset.Y > body.Size().Height+0.5 {
						t.Error("the resized answer's last paragraph is unreachable")
					}
					body.ScrollToOffset(offset)
				}
				select {
				case <-requests:
					t.Error("resizing duplicated the AI request")
				default:
				}
				test.Tap(findTreeButton(popup, "Close"))
			})
		}
	}
}

func TestStudyPanelMobileResizeKeepsPendingRequest(t *testing.T) {
	for _, action := range []string{aiActionAsk, aiActionCompare} {
		for _, failure := range []bool{false, true} {
			name := "answer"
			if failure {
				name = "error"
			}
			t.Run(action+"/"+name, func(t *testing.T) {
				state, w := mobileStudyResizeWindow(t)
				requests := stubCapturedStudyRequests(t)
				passage := syntheticAskPassage()
				passage.Ref, passage.VersionAbbrev = "2 Thessalonians 2:1–17", "SYNTHETIC-LONG"
				showAIPanelForPassage(state, action, passage, "What does the path mean?", selSpan{})
				request := nextCapturedStudyRequest(t, requests, "pending resizable card")
				popup := topPopup(t, w)
				for _, size := range []fyne.Size{{Width: 375, Height: 602.5}, {Width: 744, Height: 1133}, {Width: 375, Height: 602.5}} {
					w.Resize(size)
					assertMobileStudyMargins(t, w, popup)
					if topPopup(t, w) != popup || request.ctx.Err() != nil || findTreeButton(popup, "Cancel") == nil {
						t.Fatal("resizing replaced or cancelled the pending request")
					}
				}
				var err error
				if failure {
					err = errors.New("synthetic provider unavailable")
				}
				finishCapturedStudyRequest(t, request, "A synthetic answer paragraph describes the path.", err)
				if topPopup(t, w) != popup {
					t.Fatal("the request completed into a different card")
				}
				assertMobileStudyMargins(t, w, popup)
				if failure {
					if findTreeButton(popup, "Try again") == nil || !findTreeButton(popup, "Copy").Disabled() {
						t.Error("resizing changed the provider error state")
					}
				} else if findTreeButton(popup, "Copy").Disabled() || !sheetHas(popup, "A synthetic answer paragraph describes the path.") {
					t.Error("the resized pending card did not display its answer")
				}
				select {
				case <-requests:
					t.Error("resizing repeated the pending request")
				default:
				}
				test.Tap(findTreeButton(popup, "Close"))
			})
		}
	}
}
