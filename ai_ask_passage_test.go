package bibletext

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

type capturedStudyResult struct {
	text string
	err  error
}

type capturedStudyRequest struct {
	ctx      context.Context
	passage  aiPassage
	captured bool
	action   string
	text     string
	question string
	span     selSpan
	answer   chan capturedStudyResult
}

// stubCapturedStudyRequests observes the same seam as the cancellation tests,
// including the immutable passage carried by its context. Finish each request
// before inspecting its panel so the test driver's inline completion has ended.
func stubCapturedStudyRequests(t *testing.T) chan capturedStudyRequest {
	t.Helper()
	previousRun, previousFit := aiActionRun, studyRemeasure
	studyRemeasure = func(fit func()) { fit() }
	reqs := make(chan capturedStudyRequest, 8)
	stop := make(chan struct{})
	var mu sync.Mutex
	var contexts []context.Context
	aiActionRun = func(ctx context.Context, _ *AppState, action, text, question string, span selSpan) (string, error) {
		p, captured := aiPassageFromContext(ctx)
		r := capturedStudyRequest{
			ctx: ctx, passage: p, captured: captured, action: action,
			text: text, question: question, span: span,
			answer: make(chan capturedStudyResult, 1),
		}
		mu.Lock()
		contexts = append(contexts, ctx)
		mu.Unlock()
		select {
		case reqs <- r:
		case <-stop:
			return "", context.Canceled
		}
		select {
		case result := <-r.answer:
			return result.text, result.err
		case <-stop:
			return "", context.Canceled
		}
	}
	t.Cleanup(func() {
		close(stop)
		mu.Lock()
		pending := append([]context.Context(nil), contexts...)
		mu.Unlock()
		for _, ctx := range pending {
			<-ctx.Done()
		}
		aiActionRun, studyRemeasure = previousRun, previousFit
	})
	return reqs
}

func nextCapturedStudyRequest(t *testing.T, reqs chan capturedStudyRequest, label string) capturedStudyRequest {
	t.Helper()
	select {
	case r := <-reqs:
		return r
	case <-time.After(2 * time.Second):
		t.Fatalf("%s never reached the study request seam", label)
		return capturedStudyRequest{}
	}
}

func finishCapturedStudyRequest(t *testing.T, r capturedStudyRequest, text string, err error) {
	t.Helper()
	r.answer <- capturedStudyResult{text: text, err: err}
	<-r.ctx.Done()
}

func syntheticAskPassage() aiPassage {
	p := aiPassage{
		Book: "Mark", Chapter: 1, Lo: 2, Hi: 2, Ref: "Mark 1:2",
		Text: "Synthetic passage selected on the first page.", Version: "Synthetic edition A",
	}
	if nextRelease {
		p.VersionAbbrev = "SYN-A"
		p.PreviewText = "A distinct captured preview names the Lᴏʀᴅ."
		p.HasPreview = true
	}
	return p
}

// The legacy wrapper changes only gated Ask. Fixed actions retain their
// text-and-span request seam in both build states; explicit ForPassage calls
// are covered separately by the captured-passage lifecycle controls.
func TestStudyPanelWrapperCapturesOnlyGatedAsk(t *testing.T) {
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionCompare, aiActionAsk} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			reqs := stubCapturedStudyRequests(t)
			state := studyPanelState(t, w)
			const text = "Synthetic selection passed through the legacy wrapper."
			const question = "What does this line mean?"
			span := selSpanFromNative(2, 2)
			passage := aiPassageFor(state, text, span)
			showAIPanel(state, action, text, question, span)
			r := nextCapturedStudyRequest(t, reqs, action+" wrapper")
			wantCaptured := action == aiActionAsk && askQuestionEnabled
			if r.captured != wantCaptured {
				t.Errorf("captured passage = %v, want %v", r.captured, wantCaptured)
			}
			if wantCaptured && r.passage != passage {
				t.Errorf("Ask wrapper changed the passage: got %+v, want %+v", r.passage, passage)
			}
			if r.action != action || r.text != text || r.question != question || r.span != span {
				t.Errorf("wrapper changed the legacy request arguments: %+v", r)
			}
			finishCapturedStudyRequest(t, r, "A synthetic wrapper answer.", nil)
		})
	}
}

// Equal words and references may occur in different translations; neither
// that change nor a different question may serve the first cached answer.
func TestAskCacheSeparatesCapturedTranslationAndQuestion(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	p := syntheticAskPassage()
	const question = "What does this line mean?"
	scope := aiRequestScope(providerGemini, geminiModel, aiActionAsk, question, p)
	key := aiCacheKey(scope, p.Book, p.Chapter, p.Text)
	aiCacheSet(key, "answer from the captured edition")
	t.Cleanup(func() {
		aiCacheMu.Lock()
		delete(aiCache, key)
		aiCacheMu.Unlock()
	})

	state := &AppState{CurrentBook: "Acts", CurrentChapter: 7, CurrentVersion: "bsb"}
	got, err := runAIAction(withAIPassage(context.Background(), p), state, aiActionAsk,
		"replacement selection on a different page", question, selSpanFromNative(5, 6))
	if err != nil || got != "answer from the captured edition" {
		t.Fatalf("captured request re-read the live page: (%q, %v)", got, err)
	}

	otherEdition := p
	otherEdition.Version = "Synthetic edition B"
	otherPreview := p
	otherPreview.VersionAbbrev = "DISPLAY"
	otherPreview.PreviewText = "A different display-only preview."
	otherPreview.HasPreview = !p.HasPreview
	wantPrompt, err := aiStudyPrompt(aiActionAsk, p, question)
	if err != nil {
		t.Fatal(err)
	}
	if gotPrompt, err := aiStudyPrompt(aiActionAsk, otherPreview, question); err != nil || gotPrompt != wantPrompt {
		t.Errorf("preview metadata changed the provider prompt: got (%q, %v), want %q", gotPrompt, err, wantPrompt)
	}
	for _, tc := range []struct {
		name     string
		passage  aiPassage
		question string
		cached   bool
	}{
		{"same passage and question", p, question, true},
		{"same request with different preview metadata", otherPreview, question, true},
		{"same words reference and question in another edition", otherEdition, question, false},
		{"same passage with another question", p, "Why is this line here?", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := runAIActionForPassage(context.Background(), state, aiActionAsk, tc.passage, tc.question)
			if tc.cached {
				if err != nil || got != "answer from the captured edition" {
					t.Errorf("matching control: (%q, %v)", got, err)
				}
			} else if !isNoKeyError(err) {
				t.Errorf("changed input received the cached answer: (%q, %v)", got, err)
			}
		})
	}
}

// All three ways to ask again retain the original passage despite navigation
// and translation changes. The fresh panel's header and Report carry it too.
func TestAskPanelRetainsCapturedPassage(t *testing.T) {
	for _, transition := range []string{"retry", "answered rebuild", "in-flight eviction"} {
		t.Run(transition, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			reqs := stubCapturedStudyRequests(t)
			state := studyPanelState(t, w)
			p := syntheticAskPassage()
			const question = "What does this line mean?"
			span := selSpanFromNative(2, 2)
			showAIPanelForPassage(state, aiActionAsk, p, question, span)
			first := nextCapturedStudyRequest(t, reqs, "first request")
			checkCapturedAskRequest(t, first, p, question, span)
			panel := studyOverlay(t, w)
			assertAskPanelPassage(t, panel, p)

			state.Bible = nil
			state.CurrentBook, state.CurrentChapter, state.CurrentVersion = "Acts", 7, "bsb"
			switch transition {
			case "retry":
				finishCapturedStudyRequest(t, first, "", errors.New("synthetic request failure"))
				retry := findTreeButton(panel, "Try again")
				if retry == nil {
					t.Fatal("failed request has no retry")
				}
				test.Tap(retry)
			case "answered rebuild":
				finishCapturedStudyRequest(t, first, "first synthetic answer", nil)
				var reopen func()
				for _, registration := range state.sheetReopens {
					if registration.popup == panel {
						reopen = registration.capture()
					}
				}
				if reopen == nil {
					t.Fatal("answered panel registered no reopen")
				}
				panel.Hide()
				reopen()
			case "in-flight eviction":
				panel.Hide()
				first.answer <- capturedStudyResult{text: "first synthetic answer"}
			}

			second := nextCapturedStudyRequest(t, reqs, "request after "+transition)
			if transition == "in-flight eviction" {
				<-first.ctx.Done()
			}
			checkCapturedAskRequest(t, second, p, question, span)
			finishCapturedStudyRequest(t, second, "final synthetic answer", nil)
			again := studyOverlay(t, w)
			assertAskPanelPassage(t, again, p)
			if !treeHasText(again, question) {
				t.Errorf("panel lost exact question %q; texts %v", question, treeTexts(again))
			}
			openedInTests()
			report := findTreeButton(again, "Report")
			if report == nil || report.Disabled() {
				t.Fatal("answered panel has no enabled Report button")
			}
			test.Tap(report)
			opened := openedInTests()
			if len(opened) != 1 {
				t.Fatalf("Report opened %d links, want one", len(opened))
			}
			mail, err := url.Parse(opened[0])
			if err != nil || mail.Scheme != "mailto" {
				t.Fatalf("Report did not open a mail link: %v", err)
			}
			body := mail.Query().Get("body")
			wantBody := "I'd like to report AI-generated content shown in BibleText.\n\n" +
				"Reference: " + p.Ref + "\nStudy action: Answer\nTranslation: " + p.Version +
				"\nQuestion: " + question + "\n\nResponse:\nfinal synthetic answer\n\nMy concern:\n"
			if body != wantBody {
				t.Errorf("Report changed the exact captured reference, full translation, question or answer: got %q, want %q", body, wantBody)
			}
		})
	}
}

func checkCapturedAskRequest(t *testing.T, r capturedStudyRequest, p aiPassage, question string, span selSpan) {
	t.Helper()
	if !r.captured || r.passage != p || r.action != aiActionAsk || r.text != p.Text || r.question != question || r.span != span {
		t.Errorf("request changed the captured passage or question: %+v", r)
	}
	prompt, err := aiStudyPrompt(r.action, r.passage, r.question)
	if err != nil {
		t.Fatalf("captured request has no prompt: %v", err)
	}
	for _, want := range []string{p.Ref, p.Version, p.Text, question} {
		if !strings.Contains(prompt, want) {
			t.Errorf("request prompt lost %q", want)
		}
	}
}
