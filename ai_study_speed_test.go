package bibletext

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestAskSpeedChoicesFollowTheEffortOfEachAttempt(t *testing.T) {
	armAISpeedClock(t)
	app := test.NewApp()
	defer app.Quit()
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	w.Resize(fyne.NewSize(393, 852))
	st := studyPanelState(t, w)
	const model = "gemini-4-carbon"
	store := st.keys()
	store.setOverrideModel(providerGemini, model)
	setAIReasoningEffort(store.prefs, providerGemini, model, "high")
	sends := routeStudy(t, "")
	showAIPanel(st, aiActionAsk, "A synthetic passage.", "A synthetic question?", selSpan{})
	panel := studyOverlay(t, w)
	for _, effort := range []string{"high", "medium", "low"} {
		if effort != "high" {
			toggle := waitingDisclosure(panel)
			if toggle == nil {
				t.Fatal("missing speed choices")
			}
			toggle.Tapped(nil)
			row := waitingAction(panel, "Use less reasoning this time")
			if row == nil || row.detail != effortNoteLabel(effort) {
				t.Fatalf("no %q effort action after expanding the choices: %v", effort, treeTexts(panel))
			}
			row.Tapped(nil)
		}
		sent := nextSend(t, sends)
		if sent.model != model || sent.effort != effort {
			t.Fatalf("sent %s at %s, want %s at %s", sent.model, sent.effort, model, effort)
		}
		wantModelLine(t, panel, "Gemini (Google) · "+model+" · "+effortDisplayName(effort), "after send")
	}
	if waitingDisclosure(panel) != nil || waitingAction(panel, "Switch to a faster model") == nil {
		t.Fatal("only the direct model switch should remain at Low")
	}
	if got := aiReasoningEffort(store.prefs, providerGemini, model); got != "high" {
		t.Fatalf("lowering effort changed the saved choice to %q", got)
	}
	findTreeButton(panel, "Cancel").OnTapped()
	showAIPanel(st, aiActionAsk, "A synthetic passage.", "Another question?", selSpan{})
	if next := nextSend(t, sends); next.model != model || next.effort != "high" {
		t.Fatalf("the next question used %s at %s", next.model, next.effort)
	}
}

func TestTemporaryStudyEffortSeparatesAnswersAndRejectsStaleChoices(t *testing.T) {
	isolateAICache(t)
	store := newTestKeyStore()
	store.setActiveProvider(providerOpenAI)
	store.setAPIKey(providerOpenAI, "synthetic-key")
	grantAIPermission(store, providerOpenAI)
	const model = "gpt-5-mini"
	store.setOverrideModel(providerOpenAI, model)
	setAIReasoningEffort(store.prefs, providerOpenAI, model, "high")
	state := &AppState{aiKeys: store}
	p := aiPassage{Book: "Acts", Chapter: 2, Text: "A synthetic passage."}
	var sent []string
	prev := aiStudyGenerate
	t.Cleanup(func() { aiStudyGenerate = prev })
	aiStudyGenerate = func(ctx context.Context, info providerInfo, store *keyStore, key, prompt string) (string, error) {
		r := info.New(store, key).(*modelResolver)
		r.build = func(model, effort string) aiClient {
			return clientFunc(func(context.Context, string) (string, error) {
				sent = append(sent, effort)
				return "Answer at " + effort, nil
			})
		}
		return r.generate(ctx, prompt)
	}
	ctx := context.Background()
	temp := withAISpeedChoice(ctx, aiSpeedChoice{provider: providerOpenAI, model: model, effort: "medium"})
	for _, tc := range []struct {
		ctx  context.Context
		want string
	}{{ctx, "high"}, {temp, "medium"}, {temp, "medium"}, {ctx, "high"}} {
		got, err := runAIActionForPassage(tc.ctx, state, aiActionAsk, p, "A synthetic question?")
		if err != nil || got != "Answer at "+tc.want {
			t.Fatalf("wanted %s answer, got %q (%v)", tc.want, got, err)
		}
	}
	store.setOverrideModel(providerOpenAI, "gpt-5")
	if _, err := runAIActionForPassage(temp, state, aiActionAsk, p, "A synthetic question?"); !errors.Is(err, context.Canceled) {
		t.Fatalf("stale model choice returned %v", err)
	}
	store.setOverrideModel(providerOpenAI, model)
	rememberModelEfforts(store.prefs, providerOpenAI, []discoveredModel{{id: model, efforts: []string{"high"}}})
	if _, err := runAIActionForPassage(temp, state, aiActionAsk, p, "A synthetic question?"); !errors.Is(err, context.Canceled) {
		t.Fatalf("removed effort returned %v despite the cached answer", err)
	}
	if !slices.Equal(sent, []string{"high", "medium"}) {
		t.Fatalf("sent efforts %v", sent)
	}
}

func TestOneTimeEffortCannotSwitchModelsAfterARetirement(t *testing.T) {
	store := newTestKeyStore()
	const model = "gpt-5-mini"
	store.setResolvedModel(providerOpenAI, model)
	r := modelResolver{id: providerOpenAI, store: store, build: func(model, effort string) aiClient {
		if effort != "low" {
			t.Fatalf("sent effort %q", effort)
		}
		return clientFunc(func(context.Context, string) (string, error) {
			return "", &apiHTTPError{StatusCode: http.StatusNotFound}
		})
	}, list: func(context.Context, string) ([]discoveredModel, error) {
		t.Fatal("a same-model retry attempted model discovery")
		return nil, nil
	}}
	ctx := withAISpeedChoice(context.Background(), aiSpeedChoice{provider: providerOpenAI, model: model, effort: "low"})
	var gone modelGoneError
	if _, err := r.generate(ctx, "Synthetic question"); !errors.As(err, &gone) || gone.tried != model {
		t.Fatalf("retired model returned %v", err)
	}
}

func TestLowerEffortOfferCannotEnableExtraThinking(t *testing.T) {
	prefs := newFakePrefs()
	const model = "claude-sonnet-4-6"
	if got := speedTargetReport(prefs, providerAnthropic, model); got.lowerEffort != "" {
		t.Fatalf("the proposed retry %q would turn on adaptive thinking", got.lowerEffort)
	}
	setAIReasoningEffort(prefs, providerAnthropic, model, "high")
	if got := speedTargetReport(prefs, providerAnthropic, model); got.lowerEffort != "medium" {
		t.Fatalf("explicit High offered %q", got.lowerEffort)
	}
}
