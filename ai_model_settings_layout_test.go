package bibletext

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestSettingsEffortPickerFitsChoices(t *testing.T) {
	for _, tc := range []struct {
		name, provider, model, last string
		size                        fyne.Size
		mobile, needsScroll         bool
	}{
		{"Carbon on iPhone", providerGemini, "gemini-4-carbon", "high", fyne.NewSize(393, 852), true, false},
		{"Carbon on landscape iPhone", providerGemini, "gemini-4-carbon", "high", fyne.NewSize(667, 375), true, false},
		{"seven options on iPhone", providerOpenAI, "gpt-5.6-sol", "max", fyne.NewSize(390, 844), true, false},
		{"seven options on desktop", providerOpenAI, "gpt-5.6-sol", "max", fyne.NewSize(900, 700), false, false},
		{"seven options on short landscape", providerOpenAI, "gpt-5.6-sol", "max", fyne.NewSize(568, 280), true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			th := coverApp(t, tc.mobile)
			win := test.NewWindow(widget.NewLabel("Reading"))
			t.Cleanup(win.Close)
			win.Resize(tc.size)
			state := sampleState()
			state.window, state.theme = win, th
			state.aiKeys = newKeyStoreWith(newFakePrefs())
			store := state.keys()
			store.setActiveProvider(tc.provider)
			store.setOverrideModel(tc.provider, tc.model)
			showAISettings(state)
			button := findTreeButton(settingsPopup(t, state), "Recommended")
			if button == nil {
				t.Fatal("effort button missing")
			}
			test.Tap(button)
			list := settingsPickerList(t, state)
			scroll := test.WidgetRenderer(list).Objects()[0].(*container.Scroll)
			content, viewport := scroll.Content.MinSize().Height, scroll.Size().Height
			if (content > viewport) != tc.needsScroll {
				t.Fatalf("choice rows need %g points, viewport has %g, want scrolling %v", content, viewport, tc.needsScroll)
			}
			list.ScrollToBottom()
			offset := list.GetScrollOffset()
			if (offset > 0) != tc.needsScroll {
				t.Fatalf("scroll to bottom moved by %g, want scrolling %v", offset, tc.needsScroll)
			}
			// Tap the final row through the canvas, including clipping and
			// hit testing at the bottom edge of the scroll viewport.
			lastRowCenter := content - offset - list.CreateItem().MinSize().Height/2
			at := fyne.CurrentApp().Driver().AbsolutePositionForObject(scroll)
			test.TapCanvas(win.Canvas(), at.Add(fyne.NewPos(scroll.Size().Width/2, lastRowCenter)))
			if got := aiReasoningEffort(store.prefs, tc.provider, tc.model); got != tc.last {
				t.Fatalf("tap on the last visible row saved %q, want %q", got, tc.last)
			}
		})
	}
}

func TestSettingsProviderRadioOpensPinnedEffortAndBudget(t *testing.T) {
	for _, tc := range []struct {
		provider, model, caption string
		rows, pick               int
		label, saved             string
	}{
		{providerAnthropic, "claude-opus-4-6", "Reasoning effort", 5, 4, "Max", "max"},
		{providerAnthropic, "claude-haiku-4-5", "Thinking budget", 4, 2, "4,096 tokens", "4096"},
		{providerGrok, "grok-4.7", "Reasoning effort", 5, 4, "Extra high", "xhigh"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			state := newAppearanceHarness(t, true).state
			state.aiKeys = newKeyStoreWith(newFakePrefs())
			store := state.keys()
			store.setActiveProvider(providerGemini)
			store.setOverrideModel(tc.provider, tc.model)
			showAISettings(state)
			sheet := settingsPopup(t, state)
			info, ok := providerByID(tc.provider)
			if !ok {
				t.Fatal("unknown provider")
			}
			var radioItem fyne.Tappable
			walkTree(sheet, func(obj fyne.CanvasObject) {
				if radio, ok := obj.(*widget.RadioGroup); ok {
					if i := slices.Index(radio.Options, info.Name); i >= 0 {
						radioItem, _ = test.WidgetRenderer(radio).Objects()[i].(fyne.Tappable)
					}
				}
			})
			if radioItem == nil {
				t.Fatal("missing provider radio button")
			}
			test.Tap(radioItem)
			if store.activeProvider() != tc.provider || findTreeButton(sheet, tc.model) == nil || !treeHasText(sheet, tc.caption) {
				t.Fatal("switching provider did not show its pinned model and effort or budget label")
			}
			button := findTreeButton(sheet, "Recommended")
			if button == nil {
				t.Fatal("missing Recommended effort or budget button")
			}
			test.Tap(button)
			list := settingsPickerList(t, state)
			if n := list.Length(); n != tc.rows {
				t.Fatalf("picker has %d rows, want %d", n, tc.rows)
			}
			row := list.CreateItem()
			list.UpdateItem(tc.pick, row)
			if !treeHasText(row, tc.label) {
				t.Fatalf("choice %d should display %q", tc.pick, tc.label)
			}
			list.Select(tc.pick)
			if got := aiReasoningEffort(store.prefs, tc.provider, tc.model); got != tc.saved || button.Text != tc.label {
				t.Fatalf("choice displayed %q and saved %q", button.Text, got)
			}
		})
	}
}

func TestSettingsEffortPickerUsesEffectiveModel(t *testing.T) {
	h := newAppearanceHarness(t, true)
	state := h.state
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	store := state.keys()
	store.setActiveProvider(providerOpenAI)
	// The pinned model is also the Recommended model: both choices must
	// reopen the same saved effort even without a key or a live model list.
	store.setOverrideModel(providerOpenAI, openAIModel)
	showAISettings(state)
	var rows *modelSettingsLayout
	walkTree(settingsPopup(t, state), func(obj fyne.CanvasObject) {
		if c, ok := obj.(*fyne.Container); ok {
			if r, ok := c.Layout.(*modelSettingsLayout); ok {
				rows = r
			}
		}
	})
	if rows == nil || !rows.effort.Visible() {
		t.Fatal("the supported model did not show its effort picker")
	}
	effort := findTreeButton(rows.effort, "Recommended")
	model := findTreeButton(rows.model, openAIModel)
	if effort == nil || model == nil {
		t.Fatal("missing Model or Reasoning effort button")
	}
	effort.OnTapped()
	list := settingsPickerList(t, state)
	if list.Length() != 5 {
		t.Fatalf("effort picker has %d rows, want Recommended and four levels", list.Length())
	}
	list.Select(4)
	if effort.Text != "High" || aiReasoningEffort(store.prefs, providerOpenAI, openAIModel) != "high" {
		t.Fatal("selecting High did not update the button and its saved model preference")
	}
	model.OnTapped()
	settingsPickerList(t, state).Select(0)
	if model.Text != "Recommended" || effort.Text != "High" {
		t.Fatalf("returning to the same model gave model %q, effort %q", model.Text, effort.Text)
	}
	effort.OnTapped()
	settingsPickerList(t, state).Select(0)
	if effort.Text != "Recommended" || aiReasoningEffort(store.prefs, providerOpenAI, openAIModel) != "" {
		t.Fatal("Recommended did not clear the explicit override")
	}
	// A change to a model without this option removes both the label
	// and the button without changing the other model's preference.
	store.setResolvedModel(providerOpenAI, "gpt-4.1")
	model.OnTapped()
	settingsPickerList(t, state).Select(0)
	if rows.effort.Visible() || rows.effortLabel.Visible() {
		t.Fatal("unsupported model displayed an effort control")
	}
}

func TestModelEffortRowsShareColumnsAndPreserveModelTarget(t *testing.T) {
	state := newAppearanceHarness(t, true).state
	model := widget.NewButton("Recommended", func() {})
	effort := widget.NewButton("Recommended", func() {})
	pair := modelSettingsRows(state, model, effort)
	r := pair.Layout.(*modelSettingsLayout)
	pair.Resize(fyne.NewSize(330, 100))
	if r.model.Position().X != r.effort.Position().X || r.model.Size() != r.effort.Size() {
		t.Fatalf("picker columns differ: Model %v/%v, effort %v/%v", r.model.Position(), r.model.Size(), r.effort.Position(), r.effort.Size())
	}
	if gap := r.effort.Position().Y - r.model.Size().Height; gap != 4 {
		t.Errorf("gap = %g, want 4", gap)
	}
	if !r.modelHit.Visible() || r.modelHit.Size().Width <= 0 || r.modelHit.Position().X+r.modelHit.Size().Width != r.model.Position().X {
		t.Fatalf("hit extension does not end at Model's left edge: %v/%v", r.modelHit.Position(), r.modelHit.Size())
	}
	pair.Resize(fyne.NewSize(r.labelWidth()+r.minButtonWidth-1, 180))
	if r.model.Position().X != 0 || r.model.Position().Y < r.modelLabel.Size().Height || r.modelHit.Visible() || r.modelHit.Size().Width != 0 {
		t.Fatal("narrow picker escaped its label or retained a stray hit area")
	}
}

func TestSettingsEffortPickerDistinguishesNoneAndExtraHigh(t *testing.T) {
	state := newAppearanceHarness(t, true).state
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	store := state.keys()
	const model = "gpt-5.4"
	store.setActiveProvider(providerOpenAI)
	store.setOverrideModel(providerOpenAI, model)
	showAISettings(state)
	popup := settingsPopup(t, state)
	effort := findTreeButton(popup, "Recommended")
	if effort == nil {
		t.Fatal("missing effort button for GPT-5.4")
	}
	for _, tc := range []struct {
		row         int
		label, wire string
	}{{1, "None", "none"}, {5, "Extra high", "xhigh"}, {0, "Recommended", ""}} {
		effort.OnTapped()
		list := settingsPickerList(t, state)
		if list.Length() != 6 {
			t.Fatalf("GPT-5.4 picker has %d choices, want 6", list.Length())
		}
		list.Select(tc.row)
		if effort.Text != tc.label || aiReasoningEffort(store.prefs, providerOpenAI, model) != tc.wire {
			t.Fatalf("choice %d displayed %q and stored %q", tc.row, effort.Text, aiReasoningEffort(store.prefs, providerOpenAI, model))
		}
	}
}

func TestSettingsReopensWithPinnedGeminiCarbonEffort(t *testing.T) {
	state := newAppearanceHarness(t, true).state
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	store := state.keys()
	const model = "gemini-4-carbon"
	store.setActiveProvider(providerGemini)
	store.setOverrideModel(providerGemini, model)

	showAISettings(state)
	sheet := settingsPopup(t, state)
	if findTreeButton(sheet, model) == nil {
		t.Fatal("Settings did not show the pinned Gemini model")
	}
	button := findTreeButton(sheet, "Recommended")
	if button == nil {
		t.Fatal("missing reasoning effort picker beneath Gemini Carbon")
	}
	button.OnTapped()
	list := settingsPickerList(t, state)
	if list.Length() != 4 {
		t.Fatalf("Carbon picker has %d choices, want Recommended, Low, Medium and High", list.Length())
	}
	list.Select(3)
	sheetCloseButton(t, sheet).OnTapped()
	showAISettings(state)
	sheet = settingsPopup(t, state)
	button = findTreeButton(sheet, "High")
	if button == nil || store.overrideModel(providerGemini) != model {
		t.Fatal("reopening Settings lost the Gemini model or the selected effort")
	}
	button.OnTapped()
	settingsPickerList(t, state).Select(0)
	if aiReasoningEffort(store.prefs, providerGemini, model) != "" {
		t.Fatal("Recommended did not restore the provider default")
	}
}
