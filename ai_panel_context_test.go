package bibletext

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func askAnswerContext(t *testing.T, popup *widget.PopUp, question string) (*widget.RichText, *container.Scroll) {
	t.Helper()
	var rich *widget.RichText
	walkTree(popup, func(object fyne.CanvasObject) {
		if candidate, ok := object.(*widget.RichText); ok && segmentText(candidate.Segments) == question {
			rich = candidate
		}
	})
	if rich == nil {
		t.Fatal("the answer panel lost the complete question")
	}
	return rich, scrollHolding(popup, rich)
}

func askAnswerRichText(t *testing.T, popup *widget.PopUp) *widget.RichText {
	t.Helper()
	var rich *widget.RichText
	walkTree(popup, func(object fyne.CanvasObject) {
		if candidate, ok := object.(*widget.RichText); ok && strings.Contains(segmentText(candidate.Segments), "A synthetic answer paragraph") {
			rich = candidate
		}
	})
	if rich == nil {
		t.Fatal("the synthetic answer never rendered")
	}
	return rich
}

func studyAnswerContent(t *testing.T, popup *widget.PopUp) fyne.CanvasObject {
	t.Helper()
	var blocks *compareAnswer
	walkTree(popup, func(object fyne.CanvasObject) {
		if candidate, ok := object.(*compareAnswer); ok {
			blocks = candidate
		}
	})
	if blocks != nil {
		if !strings.Contains(studyAnswerText(blocks), "A synthetic answer paragraph") {
			t.Fatal("the synthetic answer blocks never rendered")
		}
		return blocks
	}
	return askAnswerRichText(t, popup)
}

func studyAnswerText(object fyne.CanvasObject) string {
	var blocks []string
	walkTree(object, func(child fyne.CanvasObject) {
		if rich, ok := child.(*widget.RichText); ok {
			blocks = append(blocks, segmentText(rich.Segments))
		}
	})
	return strings.Join(blocks, "\n")
}

func askAnswerBody(t *testing.T, popup *widget.PopUp) *container.Scroll {
	t.Helper()
	rich := studyAnswerContent(t, popup)
	scroll := scrollHolding(popup, rich)
	if scroll == nil {
		t.Fatal("the answer lost its scroll")
	}
	return scroll
}

func studyDisclosureButton(root fyne.CanvasObject, expanded bool) *widget.Button {
	icon := iconDisclosureExpand
	if expanded {
		icon = iconDisclosureCollapse
	}
	var found *widget.Button
	walkTree(root, func(object fyne.CanvasObject) {
		if button, ok := object.(*widget.Button); ok && button.Text == "" && button.Icon != nil && button.Icon.Name() == icon.Name() {
			found = button
		}
	})
	return found
}

func studyObjectVisible(root, target fyne.CanvasObject) bool {
	visible := false
	walkTree(root, func(object fyne.CanvasObject) {
		if object == target {
			visible = true
		}
	})
	walkTree(root, func(object fyne.CanvasObject) {
		if !object.Visible() {
			walkTree(object, func(child fyne.CanvasObject) {
				if child == target {
					visible = false
				}
			})
		}
	})
	return visible
}

func TestStudyResultPassageDisclosure(t *testing.T) {
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionCompare, aiActionAsk} {
		for _, screen := range []struct {
			name         string
			size         fyne.Size
			mobile       bool
			longCitation bool
		}{
			{"desktop", fyne.NewSize(900, 760), false, false},
			{"short desktop", fyne.NewSize(529, 505), false, false},
			{"small phone", fyne.NewSize(320, 568), true, false},
			{"small phone long citation", fyne.NewSize(320, 568), true, true},
			{"phone landscape", fyne.NewSize(874, 402), true, false},
		} {
			t.Run(action+"/"+screen.name, func(t *testing.T) {
				app := test.NewApp()
				t.Cleanup(app.Quit)
				if screen.mobile {
					fyne.SetCurrentApp(phoneTestApp{app})
				}
				w := test.NewWindow(widget.NewLabel("reading"))
				t.Cleanup(w.Close)
				w.Resize(screen.size)
				state := studyPanelState(t, w)
				reqs := stubCapturedStudyRequests(t)
				passage := syntheticAskPassage()
				passage.PreviewText = strings.Repeat("A synthetic selection follows the path from the field to the river. ", 30) + "Final selected words."
				passage.Text = passage.PreviewText
				if screen.longCitation {
					passage.Ref = "2 Thessalonians 2:1–17"
					passage.VersionAbbrev = "SYNTHETIC-LONG"
				}
				question := "What does this synthetic passage describe?"
				showAIPanelForPassage(state, action, passage, question, selSpan{})
				request := nextCapturedStudyRequest(t, reqs, "passage disclosure")
				if request.passage != passage || request.action != action || request.question != question {
					t.Fatal("display fitting changed the selected request")
				}
				answer := strings.Repeat("A synthetic answer paragraph describes the field and the path to the river.\n\n", 30)
				finishCapturedStudyRequest(t, request, answer, nil)
				popup := topPopup(t, w)
				body := askAnswerBody(t, popup)
				rendered := studyAnswerText(studyAnswerContent(t, popup))
				if strings.Count(rendered, "A synthetic answer paragraph") != 30 {
					t.Fatal("the rendered answer lost paragraphs")
				}
				excerpt, renderer := noteSheetExcerpt(t, popup)
				var citation *widget.RichText
				walkTree(popup, func(object fyne.CanvasObject) {
					if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == expectedPreviewCitation(passage.Ref, passage.VersionAbbrev) {
						citation = rich
					}
				})
				if citation == nil || scrollHolding(popup, citation) != nil {
					t.Fatal("the citation must remain outside the quotation scroll")
				}
				checkLayout := func(expanded bool) {
					t.Helper()
					label := "collapsed"
					if expanded {
						label = "expanded"
					}
					assertBoxOnScreen(t, popup, w.Canvas(), label)
					if body.Size().Height < 80 || !studyObjectVisible(popup, citation) {
						t.Errorf("%s lost reading room or its visible citation", label)
					}
					for _, text := range []string{"disclosure", "Copy", "Report", "Close"} {
						button := findTreeButton(popup, text)
						if text == "disclosure" {
							button = studyDisclosureButton(popup, expanded)
						}
						if button == nil || button.Disabled() {
							t.Fatalf("missing enabled %s control", text)
						}
						if text == "disclosure" && (button.Size().Height < 44 || button.Size().Width < 44) {
							t.Errorf("disclosure has less than a 44-point touch target: %v", button.Size())
						}
						pos := app.Driver().AbsolutePositionForObject(button)
						if pos.X < 0 || pos.Y < 0 || pos.X+button.Size().Width > w.Canvas().Size().Width+0.5 || pos.Y+button.Size().Height > w.Canvas().Size().Height+0.5 {
							t.Errorf("%s is outside the available screen", text)
						}
					}
				}
				if button := studyDisclosureButton(popup, false); button != nil {
					test.Tap(button)
				}
				checkLayout(true)
				if !studyObjectVisible(popup, excerpt) {
					t.Fatal("the expanded quotation remains hidden")
				}
				expandedHeight := body.Size().Height
				body.ScrollToOffset(fyne.NewPos(0, 80))
				offset := body.Offset
				disclosure := studyDisclosureButton(popup, true)
				if screen.mobile {
					inset, _ := w.Canvas().InteractiveArea()
					position := app.Driver().AbsolutePositionForObject(disclosure).Add(inset)
					test.TapCanvas(w.Canvas(), position.Add(fyne.NewPos(2, 2)))
				} else {
					w.Canvas().Focus(disclosure)
					if w.Canvas().Focused() != disclosure {
						t.Fatal("the disclosure cannot receive keyboard focus")
					}
					disclosure.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
				}
				checkLayout(false)
				if studyObjectVisible(popup, excerpt) || body.Size().Height <= expandedHeight+20 {
					t.Error("hiding the quotation did not reclaim answer reading space")
				}
				if body.Offset != offset {
					t.Error("hiding the quotation changed the answer's reading position")
				}
				if !screen.mobile {
					w.Resize(fyne.NewSize(screen.size.Width, 820))
					refitSheets(state)
					if studyDisclosureButton(popup, false) == nil || studyObjectVisible(popup, excerpt) {
						t.Error("resizing lost the explicit collapsed choice")
					}
					w.Resize(screen.size)
					refitSheets(state)
				}
				disclosure = studyDisclosureButton(popup, false)
				inset, _ := w.Canvas().InteractiveArea()
				position := app.Driver().AbsolutePositionForObject(disclosure).Add(inset)
				test.TapCanvas(w.Canvas(), position.Add(fyne.NewPos(disclosure.Size().Width-2, disclosure.Size().Height-2)))
				checkLayout(true)
				rows := renderer.drawn()
				if !studyObjectVisible(popup, excerpt) || excerpt.text != passage.PreviewText || !strings.HasSuffix(strings.Join(rows, " "), "Final selected words.") {
					t.Error("expanding failed to restore the complete selected quotation")
				}
				if body.Offset != offset || studyAnswerText(studyAnswerContent(t, popup)) != rendered {
					t.Error("expanding changed the answer or reading position")
				}
				body.ScrollToBottom()
				if body.Content.Size().Height-body.Offset.Y > body.Size().Height+0.5 {
					t.Error("the answer's last paragraph is outside the scroll's reach")
				}
				select {
				case <-reqs:
					t.Error("quotation disclosure repeated the AI request")
				default:
				}
				test.Tap(findTreeButton(popup, "Close"))
			})
		}
	}
}

func TestStudyResultWithoutQuotationHasNoDisclosure(t *testing.T) {
	for _, action := range []string{aiActionExplain, aiActionAsk} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(900, 760))
			reqs := stubCapturedStudyRequests(t)
			const question = "What does this passage mean?"
			showAIPanelForPassage(studyPanelState(t, w), action, aiPassage{Ref: "Mark 1:2"}, question, selSpan{})
			request := nextCapturedStudyRequest(t, reqs, "empty quotation")
			finishCapturedStudyRequest(t, request, "A synthetic answer paragraph describes the field.", nil)
			popup := topPopup(t, w)
			if studyDisclosureButton(popup, false) != nil || studyDisclosureButton(popup, true) != nil {
				t.Error("an empty quotation has an inert disclosure control")
			}
			if action == aiActionAsk {
				_, questionScroll := askAnswerContext(t, popup, question)
				if questionScroll != askAnswerBody(t, popup) {
					t.Error("Ask without a quotation lost its question in the reading area")
				}
			}
			test.Tap(findTreeButton(popup, "Close"))
		})
	}
}

func TestStudyResultShortContextUsesItsNaturalHeight(t *testing.T) {
	for _, action := range []string{aiActionCompare, aiActionAsk} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			fyne.SetCurrentApp(phoneTestApp{app})
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(375, 667))
			reqs := stubCapturedStudyRequests(t)
			passage := syntheticAskPassage()
			passage.Text, passage.PreviewText = "Final selected words.", "Final selected words."
			showAIPanelForPassage(studyPanelState(t, w), action, passage, "What does this passage mean?", selSpan{})
			request := nextCapturedStudyRequest(t, reqs, "short context")
			finishCapturedStudyRequest(t, request, "A synthetic answer paragraph describes the field.", nil)
			popup := topPopup(t, w)
			if button := studyDisclosureButton(popup, false); button != nil {
				test.Tap(button)
			}
			excerpt, _ := noteSheetExcerpt(t, popup)
			context := scrollHolding(popup, excerpt)
			if context == nil || !studyObjectVisible(popup, excerpt) {
				t.Fatal("the short expanded quotation is missing")
			}
			if extra := context.Size().Height - context.Content.MinSize().Height; math.Abs(float64(extra)) > 0.5 {
				t.Errorf("short context reserves %vpt beyond its complete text", extra)
			}
			var citation *widget.RichText
			walkTree(popup, func(object fyne.CanvasObject) {
				if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == expectedPreviewCitation(passage.Ref, passage.VersionAbbrev) {
					citation = rich
				}
			})
			if citation == nil {
				t.Fatal("the citation is missing")
			}
			quotePosition := app.Driver().AbsolutePositionForObject(excerpt)
			citationPosition := app.Driver().AbsolutePositionForObject(citation)
			if gap := citationPosition.Y - quotePosition.Y - excerpt.Size().Height; gap < 11.5 || gap > 16 {
				t.Errorf("short quotation has %vpt before its citation, want 12–16pt", gap)
			}
			test.Tap(findTreeButton(popup, "Close"))
		})
	}
}

func TestAskAnswerQuestionFollowsCitationInReadingArea(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(900, 760))
	state := studyPanelState(t, w)
	reqs := stubCapturedStudyRequests(t)
	const question = "How does this synthetic passage describe the field?"
	showAIPanelForPassage(state, aiActionAsk, askComposerFixture(), question, selSpan{})
	request := nextCapturedStudyRequest(t, reqs, "Ask answer")
	finishCapturedStudyRequest(t, request, "A synthetic answer paragraph describes the field.", nil)
	popup := topPopup(t, w)
	questionText, questionScroll := askAnswerContext(t, popup, question)
	body := askAnswerBody(t, popup)
	if questionScroll != body || studyDisclosureButton(popup, true) == nil {
		t.Fatal("the question must scroll with the answer below the expanded passage")
	}
	for _, segment := range questionText.Segments {
		text, ok := segment.(*widget.TextSegment)
		if !ok || text.Style.TextStyle.Bold || text.Style.SizeName != theme.SizeNameCaptionText {
			t.Error("the question must use regular caption-size text")
		}
	}
	var citation *widget.RichText
	walkTree(popup, func(object fyne.CanvasObject) {
		if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == expectedPreviewCitation(askComposerFixture().Ref, askComposerFixture().VersionAbbrev) {
			citation = rich
		}
	})
	if citation == nil {
		t.Fatal("the citation is missing")
	}
	questionPosition := app.Driver().AbsolutePositionForObject(questionText)
	citationPosition := app.Driver().AbsolutePositionForObject(citation)
	answerPosition := app.Driver().AbsolutePositionForObject(studyAnswerContent(t, popup))
	if questionPosition.Y < citationPosition.Y+citation.Size().Height || answerPosition.Y < questionPosition.Y+questionText.Size().Height {
		t.Error("the question must appear after the citation and before the answer")
	}
	test.Tap(studyDisclosureButton(popup, true))
	if !studyObjectVisible(popup, questionText) || segmentText(questionText.Segments) != question {
		t.Error("collapsing the quotation hid or changed the question")
	}
}

func TestStudyAnswerMarkdownRendererScope(t *testing.T) {
	const raw = "## Reading\n\nA synthetic answer paragraph describes **the path**.\n\n- Outer point.\n  - Nested point.\n\n> A synthetic quoted line."
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionCompare, aiActionAsk} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := coverTestWindow{Window: test.NewWindow(widget.NewLabel("reading")), clip: &fakeClipboard{}}
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(900, 760))
			requests := stubCapturedStudyRequests(t)
			showAIPanelForPassage(studyPanelState(t, w), action, syntheticAskPassage(), "What does the path mean?", selSpan{})
			request := nextCapturedStudyRequest(t, requests, "Markdown renderer scope")
			finishCapturedStudyRequest(t, request, raw, nil)
			popup := topPopup(t, w)
			answer := studyAnswerContent(t, popup)
			blocks, usesBlocks := answer.(*compareAnswer)
			if wantBlocks := action == aiActionCompare || action == aiActionAsk; usesBlocks != wantBlocks {
				t.Fatalf("%s block renderer = %v, want %v", action, usesBlocks, wantBlocks)
			}
			if usesBlocks {
				assertStudyAnswerMarkdownBlocks(t, blocks)
			}
			test.Tap(findTreeButton(popup, "Copy"))
			if got := w.Clipboard().Content(); got != raw {
				t.Error("Markdown rendering changed the copied provider answer")
			}
			if !sheetHas(popup, "AI-generated — may be imperfect. Verify important details.") {
				t.Error("Markdown rendering lost the AI disclosure")
			}
			test.Tap(findTreeButton(popup, "Close"))
		})
	}
}

func assertStudyAnswerMarkdownBlocks(t *testing.T, answer *compareAnswer) {
	t.Helper()
	fixture := collectCompareRendered(answer)
	heading := fixture.find(t, "Reading")
	paragraph := fixture.find(t, "A synthetic answer")
	outer := fixture.find(t, "Outer point")
	nested := fixture.find(t, "Nested point")
	quote := fixture.find(t, "A synthetic quoted")
	if !heading.style.Bold || paragraph.pos.Y < heading.pos.Y+heading.size.Height+theme.Padding() {
		t.Error("the Markdown heading is not separate from the following answer paragraph")
	}
	if nested.pos.X <= outer.pos.X || nested.pos.Y < outer.pos.Y+outer.size.Height {
		t.Error("the nested Markdown list lost its indentation or vertical placement")
	}
	if quote.pos.X <= paragraph.pos.X {
		t.Error("the Markdown quotation lost its inset")
	}
	for _, text := range fixture.texts {
		position := text.pos.Subtract(answer.Position())
		if position.X < -0.5 || position.X+text.size.Width > answer.Size().Width+0.5 || position.Y+text.size.Height > answer.MinSize().Height+0.5 {
			t.Errorf("rendered Markdown text %q escaped the measured answer: %v/%v within %v", text.text, position, text.size, answer.Size())
		}
	}
}

func TestAskAnswerLongMobileContextLeavesReadableAnswer(t *testing.T) {
	for _, screen := range []struct {
		name string
		size fyne.Size
	}{
		{"portrait", fyne.NewSize(402, 874)},
		{"landscape", fyne.NewSize(874, 402)},
	} {
		t.Run(screen.name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			fyne.SetCurrentApp(phoneTestApp{app})
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			w.Resize(screen.size)
			state := studyPanelState(t, w)
			restores := 0
			state.showReadingOverlay = func() { restores++ }
			reqs := stubCapturedStudyRequests(t)
			passage := askComposerFixture()
			passage.PreviewText = strings.Repeat("A synthetic chapter follows the path from the field to the river. ", 85) + "Final selected words."
			passage.Text = "3 " + passage.PreviewText
			question := strings.Repeat("How does this synthetic chapter connect its opening words with its ending? ", 11) + "Final question words?"
			showAIPanelForPassage(state, aiActionAsk, passage, question, selSpan{})
			request := nextCapturedStudyRequest(t, reqs, "long mobile Ask")
			if request.question != question || request.passage != passage {
				t.Fatal("display fitting changed the full captured question or selected passage")
			}
			state.CurrentBook, state.CurrentChapter, state.CurrentVersion = "Another fixture book", 8, "bsb"
			answer := strings.Repeat("A synthetic answer paragraph connects the chapter's opening and ending.\n\n", 20)
			finishCapturedStudyRequest(t, request, answer, nil)
			popup := topPopup(t, w)
			body := askAnswerBody(t, popup)
			checkReadingRoom := func() {
				t.Helper()
				assertBoxOnScreen(t, popup, w.Canvas(), "long mobile Ask answer")
				if body.Size().Height < 80 {
					t.Errorf("long mobile context leaves only %vpt for the answer", body.Size().Height)
				}
				bodyPosition := app.Driver().AbsolutePositionForObject(body)
				for _, label := range []string{"Report", "Copy", "Close"} {
					button := findTreeButton(popup, label)
					if button == nil || button.Disabled() {
						t.Fatalf("answered panel has no enabled %s control", label)
					}
					position := app.Driver().AbsolutePositionForObject(button)
					if position.Y < bodyPosition.Y+body.Size().Height || position.Y+button.Size().Height > w.Canvas().Size().Height {
						t.Errorf("%s overlaps the answer or extends below the phone", label)
					}
				}
				var citation *widget.RichText
				walkTree(popup, func(object fyne.CanvasObject) {
					if rich, ok := object.(*widget.RichText); ok && segmentText(rich.Segments) == expectedPreviewCitation(passage.Ref, passage.VersionAbbrev) {
						citation = rich
					}
				})
				if citation == nil || scrollHolding(popup, citation) != nil {
					t.Error("fitting lost the captured citation outside the details scroll")
				} else {
					position := app.Driver().AbsolutePositionForObject(citation)
					if position.Y < 0 || position.Y+citation.Size().Height > bodyPosition.Y+0.5 {
						t.Error("the captured citation overlaps the answer or lies above the phone")
					}
					excerpt, _ := noteSheetExcerpt(t, popup)
					assertPassagePreviewSpacing(t, excerpt, citation)
				}
			}
			checkReadingRoom()
			questionText, questionScroll := askAnswerContext(t, popup, question)
			excerpt, renderer := noteSheetExcerpt(t, popup)
			contextScroll := scrollHolding(popup, excerpt)
			if contextScroll == nil || questionScroll != body {
				t.Fatal("the quotation and question must have separate reachable reading areas")
			}
			if hide := studyDisclosureButton(popup, true); hide != nil {
				test.Tap(hide)
			}
			show := studyDisclosureButton(popup, false)
			if show == nil {
				t.Fatal("collapsed mobile context cannot be expanded")
			}
			checkReadingRoom()
			if !studyObjectVisible(popup, questionText) || segmentText(questionText.Segments) != question {
				t.Error("collapsing the quotation hid or changed the complete question")
			}
			test.Tap(show)
			checkReadingRoom()
			if studyDisclosureButton(popup, true) == nil {
				t.Fatal("the mobile details disclosure did not expand")
			}
			contextScroll.ScrollToBottom()
			if contextScroll.Offset.Y <= 0 || contextScroll.Content.Size().Height-contextScroll.Offset.Y > contextScroll.Size().Height+0.5 {
				t.Error("the details scroll cannot reach the selected passage's final words")
			}
			rows := renderer.drawn()
			if excerpt.text != passage.PreviewText || len(rows) < 10 || !strings.HasSuffix(strings.Join(rows, " "), "Final selected words.") {
				t.Error("expanded details truncate or change the selected passage")
			}
			body.ScrollToBottom()
			if body.Offset.Y <= 0 || body.Content.Size().Height-body.Offset.Y > body.Size().Height+0.5 {
				t.Error("the answer's final paragraph is outside the phone scroll's reach")
			}
			if strings.Count(studyAnswerText(studyAnswerContent(t, popup)), "A synthetic answer paragraph") != 20 {
				t.Error("details disclosure changed the complete answer")
			}
			test.Tap(findTreeButton(popup, "Close"))
			if popup.Visible() || restores != 1 {
				t.Error("Close did not dismiss the answer and restore the reading overlay")
			}
			select {
			case <-reqs:
				t.Error("details disclosure repeated the AI request")
			default:
			}
		})
	}
}

func TestAskAnswerRefitsContextOnShortDesktop(t *testing.T) {
	for _, longQuestion := range []bool{false, true} {
		label := "one-line question"
		question := "What does this passage say?"
		if longQuestion {
			label = "long question"
			question = strings.Repeat("How does this synthetic passage describe the field and its path? ", 30) + "Final question words."
		}
		t.Run(label, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
			app.Settings().SetTheme(th)
			state := deferredTestState(t, app)
			state.theme = th
			state.window.Resize(fyne.NewSize(529, 460))
			reqs := stubCapturedStudyRequests(t)
			passage := askComposerFixture()
			showAIPanelForPassage(state, aiActionAsk, passage, question, selSpan{})
			request := nextCapturedStudyRequest(t, reqs, "short desktop Ask")
			if request.question != question || request.passage != passage {
				t.Fatal("display fitting changed the captured question or passage")
			}
			answer := strings.Repeat("A synthetic answer paragraph follows the path from the field to the river.\n\n", 30)
			finishCapturedStudyRequest(t, request, answer, nil)
			popup := topPopup(t, state.window)
			assertBoxOnScreen(t, popup, state.window.Canvas(), "short Ask answer")
			excerpt, renderer := noteSheetExcerpt(t, popup)
			if excerpt.text != passage.PreviewText {
				t.Error("collapsing details changed the selected passage")
			}
			if !treeHasText(popup, expectedPreviewCitation(passage.Ref, passage.VersionAbbrev)) {
				t.Error("collapsed details lost the passage citation")
			}
			if studyDisclosureButton(popup, false) == nil {
				t.Fatal("a short desktop answer did not collapse its context")
			}
			body := askAnswerBody(t, popup)
			minimumReadingHeight := 2 * fyne.MeasureText("Ag", theme.TextSize(), fyne.TextStyle{}).Height
			if body.Size().Height < 2*minimumReadingHeight {
				t.Errorf("short desktop answer has only %vpt of reading room", body.Size().Height)
			}
			body.ScrollToBottom()
			if body.Offset.Y <= 0 || body.Content.Size().Height-body.Offset.Y > body.Size().Height+0.5 {
				t.Error("the answer's final paragraph is outside the scroll's reach")
			}
			for _, label := range []string{"Copy", "Report", "Close"} {
				button := findTreeButton(popup, label)
				if button == nil || button.Disabled() {
					t.Fatalf("answered panel has no enabled %s control", label)
				}
				position := app.Driver().AbsolutePositionForObject(button)
				if position.Y+button.Size().Height > state.window.Canvas().Size().Height {
					t.Errorf("%s is below the short window", label)
				}
			}

			// With no explicit choice, tall windows expand and short windows
			// collapse. Refitting keeps the same answered panel and request.
			state.window.Resize(fyne.NewSize(1100, 760))
			refitSheets(state)
			assertBoxOnScreen(t, popup, state.window.Canvas(), "expanded Ask answer")
			if studyDisclosureButton(popup, true) == nil {
				t.Fatal("a tall desktop answer did not expand its context")
			}
			if body.Size().Height < 180 {
				t.Errorf("expanded answer did not regain reading room: %vpt", body.Size().Height)
			}
			state.window.Resize(fyne.NewSize(529, 460))
			refitSheets(state)
			show := studyDisclosureButton(popup, false)
			if show == nil {
				t.Fatal("shrinking the window did not collapse its context")
			}
			show.OnTapped()
			assertBoxOnScreen(t, popup, state.window.Canvas(), "disclosed short Ask answer")
			if studyDisclosureButton(popup, true) == nil || body.Size().Height < minimumReadingHeight {
				t.Errorf("expanded short details squeezed out the answer: %vpt", body.Size().Height)
			}
			_, questionScroll := askAnswerContext(t, popup, question)
			if questionScroll != body {
				t.Error("the long question must remain in the answer reading area")
			}
			contextScroll := scrollHolding(popup, excerpt)
			if contextScroll.Content.Size().Height <= contextScroll.Size().Height {
				t.Fatal("the disclosed context did not retain scrollable content")
			}
			contextScroll.ScrollToBottom()
			if contextScroll.Offset.Y <= 0 || contextScroll.Content.Size().Height-contextScroll.Offset.Y > contextScroll.Size().Height+0.5 {
				t.Error("the context's final words are outside the scroll's reach")
			}
			rows := renderer.drawn()
			if len(rows) < 3 || strings.HasSuffix(rows[len(rows)-1], noteExcerptEllipsis) {
				t.Errorf("expanded details truncate the selected words: %v", rows)
			}
			studyDisclosureButton(popup, true).OnTapped()
			state.window.Resize(fyne.NewSize(1100, 760))
			refitSheets(state)
			if studyDisclosureButton(popup, false) == nil {
				t.Fatal("a resize lost the reader's explicit collapsed choice")
			}
			studyDisclosureButton(popup, false).OnTapped()
			state.window.Resize(fyne.NewSize(529, 460))
			refitSheets(state)
			if studyDisclosureButton(popup, true) == nil || body.Size().Height < minimumReadingHeight {
				t.Error("a resize lost the reader's explicit expanded choice or reading room")
			}
			if got := studyAnswerText(studyAnswerContent(t, popup)); strings.Count(got, "A synthetic answer paragraph") != 30 {
				t.Error("context fitting changed the complete rendered answer")
			}
			if top := studyOverlay(t, state.window); top != popup {
				t.Error("refitting reopened the answer instead of keeping its state")
			}
			select {
			case <-reqs:
				t.Error("context fitting repeated the AI request")
			default:
			}
		})
	}
}
