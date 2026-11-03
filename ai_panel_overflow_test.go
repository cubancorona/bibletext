package bibletext

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func errorRecoveryScroll(popup *widget.PopUp) *container.Scroll {
	button := findTreeButton(popup, "Try again")
	var found *container.Scroll
	walkTree(popup, func(object fyne.CanvasObject) {
		if scroll, ok := object.(*container.Scroll); ok && findTreeButton(scroll.Content, "Try again") == button && button != nil {
			found = scroll
		}
	})
	return found
}

// The last controls must be reachable by scrolling without moving Close.
func wantAIErrorOverflowReachable(t *testing.T, popup *widget.PopUp) {
	t.Helper()
	scroll := errorRecoveryScroll(popup)
	if scroll == nil || scroll.Size().Height < 32 {
		t.Fatal("exceptional error lacks a usable combined scroll")
	}
	driver := fyne.CurrentApp().Driver()
	closeButton := findTreeButton(popup, "Close")
	before := driver.AbsolutePositionForObject(closeButton)
	for _, name := range []string{"Try again", "Report", "Copy"} {
		button := findTreeButton(scroll.Content, name)
		if button == nil || button.Size().Height < button.MinSize().Height-.5 {
			t.Fatalf("combined scroll lost or shrank %s", name)
		}
	}
	offset := scroll.Offset
	scroll.ScrollToBottom()
	last := findTreeButton(scroll.Content, "Copy")
	lastPosition := driver.AbsolutePositionForObject(last)
	top := driver.AbsolutePositionForObject(scroll)
	if lastPosition.Y < top.Y-.5 || lastPosition.Y+last.Size().Height > top.Y+scroll.Size().Height+.5 {
		t.Error("the combined scroll cannot reach its final controls")
	}
	if driver.AbsolutePositionForObject(closeButton) != before || findTreeButton(scroll.Content, "Close") != nil {
		t.Error("Close moves with the scrolling error contents")
	}
	scroll.ScrollToOffset(offset)
}

func TestAIErrorExceptionalHeightScrollsContextAndRecovery(t *testing.T) {
	for _, size := range []fyne.Size{{Width: 640, Height: 320}, {Width: 732, Height: 240}} {
		for _, action := range []string{aiActionExplain, aiActionCompare, aiActionAsk} {
			for _, timeout := range []bool{false, true} {
				t.Run(fmt.Sprintf("%.0fx%.0f/%s/timeout=%t", size.Width, size.Height, action, timeout), func(t *testing.T) {
					state, w := mobileStudyResizeWindow(t)
					previous := canvasArea
					canvasArea = func(c fyne.Canvas) (fyne.Position, fyne.Size) {
						return fyne.NewPos(0, 24), fyne.NewSize(c.Size().Width, c.Size().Height-24)
					}
					t.Cleanup(func() { canvasArea = previous })
					w.Resize(size)
					requests := stubCapturedStudyRequests(t)
					showAIPanelForPassage(state, action, syntheticAskPassage(), "How does the synthetic passage describe the path?", selSpan{})
					err := syntheticAIAccountError()
					if timeout {
						err = syntheticAITimeout()
					}
					finishCapturedStudyRequest(t, nextCapturedStudyRequest(t, requests, "short error"), "", err)
					popup := topPopup(t, w)
					scroll := errorRecoveryScroll(popup)
					if scroll == nil {
						t.Fatal("exceptionally short error must put its recovery actions in the context/message scroll")
					}
					box := sheetBoxOn(t, popup)
					if box.y0 < 23.5 || box.y1 > size.Height-23.5 {
						t.Errorf("error card %v escapes safe centred bounds on %v", box, size)
					}
					if scroll.Size().Height < 32 || scroll.Content.MinSize().Height <= scroll.Size().Height {
						t.Errorf("overflow body is not a usable scrolling region: %v / %v", scroll.Size(), scroll.Content.MinSize())
					}
					closeButton := findTreeButton(popup, "Close")
					if findTreeButton(scroll.Content, "Close") != nil {
						t.Error("Close must stay outside the scrolling region")
					}
					position := fyne.CurrentApp().Driver().AbsolutePositionForObject(closeButton)
					if position.Y < box.y0 || position.Y+closeButton.Size().Height > box.y1+.5 {
						t.Errorf("Close escaped the card: %v / %v", position, closeButton.Size())
					}
					for _, label := range []string{"Copy", "Report"} {
						button := findTreeButton(scroll.Content, label)
						if button == nil || !button.Disabled() {
							t.Errorf("scroll lost disabled %s", label)
						}
					}
					if !strings.Contains(strings.Join(treeTexts(scroll.Content), " "), "AI-generated") {
						t.Error("overflow discarded the disclosure")
					}
					closePosition := position
					scroll.ScrollToBottom()
					if fyne.CurrentApp().Driver().AbsolutePositionForObject(closeButton) != closePosition {
						t.Error("scrolling moved the fixed Close control")
					}
					copyButton := findTreeButton(scroll.Content, "Copy")
					last := fyne.CurrentApp().Driver().AbsolutePositionForObject(copyButton)
					top := fyne.CurrentApp().Driver().AbsolutePositionForObject(scroll)
					if last.Y < top.Y-.5 || last.Y+copyButton.Size().Height > top.Y+scroll.Size().Height+.5 {
						t.Errorf("last recovery content is unreachable: %.1f..%.1f in %.1f..%.1f", last.Y, last.Y+copyButton.Size().Height, top.Y, top.Y+scroll.Size().Height)
					}
					test.Tap(closeButton)
				})
			}
		}
	}
}

func TestAIErrorExceptionalLayoutRestoresOnResizeAndRetry(t *testing.T) {
	state, w := mobileStudyResizeWindow(t)
	requests := stubCapturedStudyRequests(t)
	showAIPanelForPassage(state, aiActionAsk, syntheticAskPassage(), "What does the path mean?", selSpan{})
	finishCapturedStudyRequest(t, nextCapturedStudyRequest(t, requests, "initial"), "", syntheticAIAccountError())
	popup := topPopup(t, w)
	if errorRecoveryScroll(popup) != nil {
		t.Fatal("normal height must retain the pinned recovery layout")
	}
	w.Resize(fyne.NewSize(732, 240))
	if errorRecoveryScroll(popup) == nil {
		t.Fatal("resize did not enable exceptional-height scrolling")
	}
	w.Resize(fyne.NewSize(744, 1133))
	if errorRecoveryScroll(popup) != nil {
		t.Fatal("a taller resize did not restore the standard error layout")
	}
	w.Resize(fyne.NewSize(732, 240))
	test.Tap(findTreeButton(popup, "Try again"))
	request := nextCapturedStudyRequest(t, requests, "retry")
	finishCapturedStudyRequest(t, request, "A synthetic answer about the path.", nil)
	if errorRecoveryScroll(popup) != nil {
		t.Fatal("answer retained the exceptional error layout")
	}
	if button := findTreeButton(popup, "Copy"); button == nil || button.Disabled() {
		t.Fatal("answer lost the ordinary Copy action")
	}
	test.Tap(findTreeButton(popup, "Close"))
}

func TestAIErrorOverflowAdditionalForms(t *testing.T) {
	for _, form := range []aiErrorForm{
		{name: "generic", action: aiActionExplain, err: errAIUnreachable, button: "Try again"},
		{name: "missing-key", action: aiActionAsk, question: "How does this synthetic passage fit?", err: errNoAPIKey, button: "Open AI settings"},
	} {
		t.Run(form.name, func(t *testing.T) {
			state, w := insetWindow(t, insetScreen{"short synthetic phone", 732, 240, 24, 24, 0, 0}, 0)
			requests := stubCapturedStudyRequests(t)
			openAIError(t, requests, state, form, aiLongSelection)
			popup := topPopup(t, w)
			if !treeHasText(popup, friendlyAIError(form.err)) {
				t.Fatal("the overflow lost the exact error message")
			}
			box := sheetBoxOn(t, popup)
			if box.y0 < 23.5 || box.y1 > 216.5 {
				t.Errorf("error escaped safe bounds: %v", box)
			}
			if findTreeButton(popup, form.button) == nil {
				t.Fatal("the error lost its recovery action")
			}
			if form.button == "Try again" {
				wantAIErrorOverflowReachable(t, popup)
			}
			test.Tap(quoteDisclosure(t, popup))
			if quoteDisclosure(t, popup).Icon != iconDisclosureCollapse {
				t.Error("the exceptional error lost an explicit passage expansion")
			}
			for _, label := range []string{"Copy", "Report"} {
				if button := findTreeButton(popup, label); button == nil || !button.Disabled() {
					t.Errorf("error lost disabled %s", label)
				}
			}
			test.Tap(findTreeButton(popup, "Close"))
		})
	}
	t.Run("narrow-desktop", func(t *testing.T) {
		app := newDesktopApp(t)
		state, w := app.window(t, fyne.NewSize(507, 240))
		requests := stubCapturedStudyRequests(t)
		showAIPanelForPassage(state, aiActionAsk, syntheticAskPassage(), "How does the path fit this passage?", selSpan{})
		finishCapturedStudyRequest(t, nextCapturedStudyRequest(t, requests, "desktop error"), "", syntheticAIAccountError())
		popup := topPopup(t, w)
		wantAIErrorOverflowReachable(t, popup)
		box := sheetBoxOn(t, popup)
		if box.y0 < -.5 || box.y1 > w.Canvas().Size().Height+.5 {
			t.Errorf("desktop error escaped window: %v / %v", box, w.Canvas().Size())
		}
		test.Tap(findTreeButton(popup, "Close"))
	})
}

// The optional output records the normal geometry as well as actual canvas
// renders of both ends of a scrolling short error.
func TestAIErrorLayoutSnapshots(t *testing.T) {
	dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("snapshot output is opt-in")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, size := range []fyne.Size{{Width: 744, Height: 1133}, {Width: 640, Height: 320}, {Width: 732, Height: 240}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size.Width, size.Height), func(t *testing.T) {
			state, w := insetWindow(t, insetScreen{"synthetic snapshot", size.Width, size.Height, 24, 24, 0, 0}, 0)
			requests := stubCapturedStudyRequests(t)
			showAIPanelForPassage(state, aiActionAsk, syntheticAskPassage(), "What does the path mean?", selSpan{})
			finishCapturedStudyRequest(t, nextCapturedStudyRequest(t, requests, "render error"), "", syntheticAIAccountError())
			popup := topPopup(t, w)
			name := fmt.Sprintf("%.0fx%.0f", size.Width, size.Height)
			capture := func(suffix string) {
				t.Helper()
				file, err := os.Create(filepath.Join(dir, name+suffix+".png"))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, w.Canvas().Capture())
				closeErr := file.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
			}
			sheetBox(t, popup)
			capture("-top")
			box := sheetBoxOn(t, popup)
			geometry := map[string]any{"sheet": map[string]float32{"left": box.x0, "top": box.y0, "right": box.x1, "bottom": box.y1}}
			title := aiPanelTitle(t, popup, aiActionAsk)
			geometry["title"] = map[string]any{"position": fyne.CurrentApp().Driver().AbsolutePositionForObject(title), "size": title.Size(), "minimum": title.MinSize()}
			for _, label := range []string{"Try again", "Report", "Copy", "Close"} {
				button := findTreeButton(popup, label)
				geometry[label] = map[string]any{"position": fyne.CurrentApp().Driver().AbsolutePositionForObject(button), "size": button.Size(), "minimum": button.MinSize()}
			}
			file, err := json.MarshalIndent(geometry, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+"-geometry.json"), file, 0600); err != nil {
				t.Fatal(err)
			}
			if scroll := errorRecoveryScroll(popup); scroll != nil {
				scroll.ScrollToBottom()
				capture("-bottom")
			}
			test.Tap(findTreeButton(popup, "Close"))
		})
	}
}
