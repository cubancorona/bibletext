package bibletext

import (
	"fmt"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// armAISpeedClock installs a deterministic synthetic clock whose steps exceed
// aiSpeedArmDelay so non-timing tests can tap freshly rendered choice rows
// immediately without sleeping. Timing tests install their own manual clock.
func armAISpeedClock(t *testing.T) {
	t.Helper()
	prev := aiSpeedNow
	now := time.Unix(1_700_000_000, 0)
	aiSpeedNow = func() time.Time {
		now = now.Add(time.Second)
		return now
	}
	t.Cleanup(func() { aiSpeedNow = prev })
}

// Only return controls the reader can activate from the current view, and fail
// loudly if duplicate visible rows ever share the same title.
func waitingAction(root fyne.CanvasObject, label string) *aiSwitchActionRow {
	var matches []*aiSwitchActionRow
	walkVisibleControls(root, func(o fyne.CanvasObject) {
		if r, ok := o.(*aiSwitchActionRow); ok && r.titleText == label {
			matches = append(matches, r)
		}
	})
	if len(matches) > 1 {
		panic(fmt.Sprintf("found %d visible speed actions titled %q", len(matches), label))
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func waitingDisclosure(root fyne.CanvasObject) *aiSwitchDisclosure {
	var matches []*aiSwitchDisclosure
	walkVisibleControls(root, func(o fyne.CanvasObject) {
		if r, ok := o.(*aiSwitchDisclosure); ok {
			matches = append(matches, r)
		}
	})
	if len(matches) > 1 {
		panic(fmt.Sprintf("found %d visible speed disclosures", len(matches)))
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return nil
}

func TestAISpeedControlOffersAndValidatesEachRetry(t *testing.T) {
	armAISpeedClock(t)
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	store := newTestKeyStore()
	state.aiKeys = store
	store.setActiveProvider(providerOpenAI)
	store.setAPIKey(providerOpenAI, "synthetic")
	const source = "gpt-5.4-pro"
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
	if toggle == nil || toggle.title.Text != "Faster options" || toggle.expanded || waitingAction(c.box, "Use less reasoning this time") != nil {
		t.Fatal("missing speed-options control")
	}
	w.Canvas().Focus(toggle)
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if sw := waitingAction(c.box, "Switch to a faster model"); sw == nil || sw.detail != openAIFastModel+" · saves in Settings" {
		t.Fatalf("switch should show the destination scope note: %v", treeTexts(c.box))
	}
	if lowerRow := waitingAction(c.box, "Use less reasoning this time"); lowerRow == nil || lowerRow.detail != "Medium effort" {
		t.Fatalf("lower-effort row should show Medium effort: %v", treeTexts(c.box))
	}
	old := tap("Use less reasoning this time")
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if waitingDisclosure(c.box) != toggle || w.Canvas().Focused() != toggle {
		t.Fatal("collapsing the choices lost keyboard focus")
	}
	old()
	if len(calls) != 0 || waitingAction(c.box, "Use less reasoning this time") != nil {
		t.Fatalf("a dismissed offer was still active: %v", calls)
	}
	toggle.Tapped(nil)
	lower := tap("Use less reasoning this time")
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
	if lowerRow := waitingAction(c.box, "Use less reasoning this time"); lowerRow == nil || lowerRow.detail != "Low effort" {
		t.Fatalf("expected Low effort note: %v", treeTexts(c.box))
	}
	tap("Use less reasoning this time")()
	if len(calls) != 2 || calls[1].effort != "low" || aiReasoningEffort(store.prefs, providerOpenAI, source) != "high" {
		t.Fatalf("wrong second step: %v", calls)
	}

	// A stale offer cannot pin a different effort from the one active when
	// options() built it. The refreshed direct control exposes the updated
	// offer before applying it.
	newWait("")
	firstSwitch := tap("Switch to a faster model")
	setAIReasoningEffort(store.prefs, providerOpenAI, openAIFastModel, "low")
	firstSwitch()
	sw := waitingAction(c.box, "Switch to a faster model")
	if len(calls) != 2 || activeModelFor(store, providerOpenAI) != source ||
		sw == nil || sw.detail != openAIFastModel+" · saves in Settings" {
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
	armAISpeedClock(t)
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
			budget := waitingAction(c.box, "Use less reasoning this time")
			if budget == nil || budget.detail != "4,096 tokens" || waitingDisclosure(c.box) != nil || waitingAction(c.box, "Switch to a faster model") != nil {
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
