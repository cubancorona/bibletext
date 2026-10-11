package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

type answerRefreshCounter struct {
	widget.TextSegment
	updates int
}

func (s *answerRefreshCounter) Update(object fyne.CanvasObject) {
	s.updates++
	s.TextSegment.Update(object)
}

func TestAnswerDeferredFitStopsAfterDismissal(t *testing.T) {
	for _, dismissal := range []string{"visible", "close", "evicted"} {
		t.Run(dismissal, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			window := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(window.Close)
			window.Resize(fyne.NewSize(900, 760))
			state := studyPanelState(t, window)
			requests := stubCapturedStudyRequests(t)
			var deferredFit func()
			studyRemeasure = func(fit func()) { deferredFit = fit }
			showAIPanelForPassage(state, aiActionAsk, askComposerFixture(), "What does this passage say?", selSpan{})
			request := nextCapturedStudyRequest(t, requests, "deferred answer fit")
			finishCapturedStudyRequest(t, request, "A synthetic answer paragraph describes the field.", nil)
			if deferredFit == nil {
				t.Fatal("the answer did not schedule its final fit")
			}
			popup := topPopup(t, window)
			answer := askAnswerRichText(t, popup)
			counter := &answerRefreshCounter{TextSegment: widget.TextSegment{
				Text: "A synthetic answer remains available.", Style: widget.RichTextStyleParagraph,
			}}
			answer.Segments = []widget.RichTextSegment{counter}
			answer.Refresh()
			switch dismissal {
			case "close":
				test.Tap(findTreeButton(popup, "Close"))
			case "evicted":
				popup.Hide()
			}
			counter.updates = 0
			deferredFit()
			if dismissal == "visible" && counter.updates == 0 {
				t.Error("a visible answer lost its final layout pass")
			}
			if dismissal != "visible" && counter.updates != 0 {
				t.Error("the delayed fit refreshed an answer after its popup left the canvas")
			}
		})
	}
}
