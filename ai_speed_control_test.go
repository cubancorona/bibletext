package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// A hidden action still exists in the disclosure's container. Only return
// controls the reader can activate from the current view.
func waitingAction(root fyne.CanvasObject, label string) *aiSwitchActionRow {
	var action *aiSwitchActionRow
	walkVisibleControls(root, func(o fyne.CanvasObject) {
		if r, ok := o.(*aiSwitchActionRow); ok && r.label.Text == label {
			action = r
		}
	})
	return action
}

func waitingDisclosure(root fyne.CanvasObject) *aiSwitchDisclosure {
	var toggle *aiSwitchDisclosure
	walkVisibleControls(root, func(o fyne.CanvasObject) {
		if r, ok := o.(*aiSwitchDisclosure); ok {
			toggle = r
		}
	})
	return toggle
}

func TestAISpeedControlOffersAndValidatesEachRetry(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	store := newTestKeyStore()
	state.aiKeys = store
	store.setActiveProvider(providerOpenAI)
	store.setAPIKey(providerOpenAI, "synthetic")
	const source = "gpt-synthetic-study"
	store.setOverrideModel(providerOpenAI, source)
	rememberModelEfforts(store.prefs, providerOpenAI, []discoveredModel{
		{id: source, efforts: []string{"low", "medium", "high"}},
		{id: openAIFastModel, efforts: []string{"low", "high"}},
	})
	setAIReasoningEffort(store.prefs, providerOpenAI, source, "high")
	setAIReasoningEffort(store.prefs, providerOpenAI, openAIFastModel, "high")
	var calls []aiSpeedChoice
	var c *aiSpeedControl
	newWait := func(lower string) {
		c = newAISpeedControl(state, func(choice aiSpeedChoice) { calls = append(calls, choice) })
		c.show(aiModelInUse{provider: providerOpenAI, model: source, lowerEffort: lower})
	}
	tap := func(text string) func() {
		t.Helper()
		h := waitingAction(c.box, text)
		if h == nil {
			t.Fatalf("missing action %q in %v", text, treeTexts(c.box))
		}
		return func() { h.Tapped(nil) }
	}

	// Only the chooser appears while collapsed. Opening and closing it
	// never retries, and a callback from the collapsed choices is inert.
	newWait("medium")
	w := test.NewWindow(c.box)
	t.Cleanup(w.Close)
	toggle := waitingDisclosure(c.box)
	if toggle == nil || toggle.title.Text+" "+toggle.second.Text != "Switch to a faster model or reasoning effort" || toggle.expanded || waitingAction(c.box, "Use Medium effort this time") != nil {
		t.Fatal("missing speed-options control")
	}
	w.Canvas().Focus(toggle)
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if sw := waitingAction(c.box, "Switch to a faster model"); sw == nil || sw.detail != "Restart this request and save "+openAIFastModel+" in Settings. Effort: High." {
		t.Fatalf("switch should show the destination's saved effort: %v", treeTexts(c.box))
	}
	old := tap("Use Medium effort this time")
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if waitingDisclosure(c.box) != toggle || w.Canvas().Focused() != toggle {
		t.Fatal("collapsing the choices lost keyboard focus")
	}
	old()
	if len(calls) != 0 || waitingAction(c.box, "Use Medium effort this time") != nil {
		t.Fatalf("a dismissed offer was still active: %v", calls)
	}
	toggle.Tapped(nil)
	lower := tap("Use Medium effort this time")
	lower()
	lower()
	if len(calls) != 1 || calls[0] != (aiSpeedChoice{providerOpenAI, source, "medium"}) ||
		activeModelFor(store, providerOpenAI) != source || aiReasoningEffort(store.prefs, providerOpenAI, source) != "high" {
		t.Fatalf("one-time retry changed settings or repeated: %v", calls)
	}

	// The request just retried at Medium can step down to Low while its
	// saved High setting remains intact.
	newWait("low")
	waitingDisclosure(c.box).Tapped(nil)
	tap("Use Low effort this time")()
	if len(calls) != 2 || calls[1].effort != "low" || aiReasoningEffort(store.prefs, providerOpenAI, source) != "high" {
		t.Fatalf("wrong second step: %v", calls)
	}

	// A stale offer cannot pin a different effort from the one shown.
	// The refreshed direct control exposes the new choice before applying it.
	newWait("")
	firstSwitch := tap("Switch to a faster model")
	setAIReasoningEffort(store.prefs, providerOpenAI, openAIFastModel, "low")
	firstSwitch()
	sw := waitingAction(c.box, "Switch to a faster model")
	if len(calls) != 2 || activeModelFor(store, providerOpenAI) != source ||
		sw == nil || sw.detail != "Restart this request and save "+openAIFastModel+" in Settings. Effort: Low." {
		t.Fatalf("stale destination retried or failed to repaint: %v, %v", calls, treeTexts(c.box))
	}
	tap("Switch to a faster model")()
	if len(calls) != 3 || calls[2] != (aiSpeedChoice{provider: providerOpenAI, model: openAIFastModel}) ||
		activeModelFor(store, providerOpenAI) != openAIFastModel || aiReasoningEffort(store.prefs, providerOpenAI, openAIFastModel) != "low" {
		t.Fatalf("switch did not pin the displayed model and keep its effort: %v", calls)
	}

	c = newAISpeedControl(state, func(aiSpeedChoice) { t.Fatal("unexpected retry") })
	if c.box.Visible() {
		t.Fatal("a fast model at its lowest setting has no speed action")
	}
	store.setOverrideModel(providerOpenAI, source)
	c = newAISpeedControl(state, nil)
	c.show(aiModelInUse{provider: providerOpenAI, model: source, lowerEffort: "medium"})
	if c.box.Visible() {
		t.Fatal("an unbound waiting pane offers no speed actions")
	}
}

func TestAISpeedControlOnlyAValidLowerBudgetCanRetry(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, change := range []string{"none", "key", "model", "provider", "level removed", "permission withdrawn", "closed", "inactive view"} {
		t.Run(change, func(t *testing.T) {
			state := sampleState()
			store := newTestKeyStore()
			state.aiKeys = store
			store.setAPIKey(providerGemini, "synthetic")
			grantAIPermission(store, providerGemini)
			store.setOverrideModel(providerGemini, geminiFastModel)
			rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{
				{id: geminiFastModel, efforts: []string{"2048", "4096", "8192"}},
			})
			var calls []aiSpeedChoice
			c := newAISpeedControl(state, func(choice aiSpeedChoice) { calls = append(calls, choice) })
			c.show(aiModelInUse{provider: providerGemini, model: geminiFastModel, lowerEffort: "4096"})
			budget := waitingAction(c.box, "Use a smaller thinking budget this time")
			if budget == nil || budget.detail != "Restart this request with a limit of 4,096 tokens. Your saved budget stays the same." || waitingDisclosure(c.box) != nil || waitingAction(c.box, "Switch to a faster model") != nil {
				t.Fatalf("only the direct budget action should show: %v", treeTexts(c.box))
			}
			switch change {
			case "key":
				store.setAPIKey(providerGemini, "replaced")
			case "model":
				store.setOverrideModel(providerGemini, "different-model")
			case "provider":
				store.setActiveProvider(providerOpenAI)
			case "permission withdrawn":
				store.setAIEnabled(false)
				store.setAIEnabled(true)
			case "level removed":
				rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{{id: geminiFastModel, efforts: []string{"2048", "8192"}}})
			case "closed":
				c.close()
			case "inactive view":
				c.active = func() bool { return false }
			}
			budget.Tapped(nil)
			if change == "none" {
				if len(calls) != 1 || calls[0] != (aiSpeedChoice{providerGemini, geminiFastModel, "4096"}) {
					t.Fatalf("wrong retry: %v", calls)
				}
			} else if len(calls) != 0 {
				t.Fatalf("%s let a stale retry through: %v", change, calls)
			}
		})
	}
}
