package bibletext

import (
	"image/color"
	"slices"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// effort is empty for a model switch. A nonempty effort belongs to the
// restarted request; the model's saved effort is never changed by this action.
type aiSpeedChoice struct {
	provider, model, effort string
}

type aiSpeedOffer struct {
	source, target aiModelInUse
	savedEffort    string
}

type aiSpeedControl struct {
	state                *AppState
	box                  *fyne.Container
	toggle               *widget.Hyperlink
	disclosure           *aiSwitchDisclosure
	retry                func(aiSpeedChoice)
	relayout             func()
	active               func() bool
	provider, credential string
	requiresGrant        bool
	timeout              bool
	source               aiModelInUse
	offer                aiSpeedOffer
	generation           int
	closed               bool
}

func newAISpeedControl(state *AppState, retry func(aiSpeedChoice)) *aiSpeedControl {
	return makeAISpeedControl(state, retry, false)
}

func makeAISpeedControl(state *AppState, retry func(aiSpeedChoice), timeout bool) *aiSpeedControl {
	c := &aiSpeedControl{state: state, box: container.NewVBox(), retry: retry, timeout: timeout}
	c.box.Hide()
	if retry != nil && state != nil && aiFeaturesEnabled(state) {
		store := state.keys()
		c.provider = store.activeProvider()
		key := providerAPIKey(store, c.provider)
		c.credential = aiAllowedRecord(c.provider, key)
		c.requiresGrant = store.aiAllowed(c.provider, key)
		// Before the first report, only the configured model switch is known.
		c.show(aiModelInUse{provider: c.provider, model: activeModelFor(store, c.provider)})
	}
	return c
}

func (c *aiSpeedControl) options() (o aiSpeedOffer) {
	if c.closed || c.retry == nil || c.credential == "" || c.state == nil || !aiFeaturesEnabled(c.state) || c.source.provider != c.provider {
		return o
	}
	store := c.state.keys()
	key := providerAPIKey(store, c.provider)
	if store.activeProvider() != c.provider || activeModelFor(store, c.provider) != c.source.model ||
		aiAllowedRecord(c.provider, key) != c.credential || (c.requiresGrant && !store.aiAllowed(c.provider, key)) {
		return o
	}
	o.source = c.source
	o.savedEffort = aiReasoningEffort(store.prefs, c.provider, c.source.model)
	if !slices.Contains(modelEffortChoices(store.prefs, c.provider, c.source.model), o.source.lowerEffort) {
		o.source.lowerEffort = ""
	}
	if id, model, _, ok := fasterModelOffer(c.state); ok {
		o.target = speedTargetReport(store.prefs, id, model)
	}
	return o
}

func (c *aiSpeedControl) show(m aiModelInUse) {
	if c == nil || c.closed || m.model == "" {
		return
	}
	c.source = m
	o := c.options()
	if o != c.offer {
		c.render(o)
	}
}

func retryEffortLabel(effort string) string {
	label := effortDisplayName(effort)
	if _, err := strconv.Atoi(effort); err == nil {
		return "Retry with " + label
	}
	return "Retry at " + label + " effort"
}

func switchModelDetail(target aiModelInUse) string {
	detail := "Restart this request and save " + target.model + " in Settings."
	if target.effortLabel != "" {
		detail += " Effort: " + target.effortLabel + "."
	}
	return detail
}

func progressEffortCopy(effort string) (title, detail string) {
	label := effortDisplayName(effort)
	if _, err := strconv.Atoi(effort); err == nil {
		return "Use a smaller thinking budget this time", "Restart this request with a limit of " + label + ". Your saved budget stays the same."
	}
	return "Use " + label + " effort this time", "Restart this request with " + label + " effort. Your saved effort stays the same."
}

func (c *aiSpeedControl) render(o aiSpeedOffer) {
	c.offer = o
	c.generation++
	generation := c.generation
	// A rebuilt view, a new attempt, or a change in credentials or options
	// invalidates a link even if a queued tap still holds it.
	current := func() bool {
		if c.closed || c.generation != generation || (c.active != nil && !c.active()) {
			return false
		}
		if fresh := c.options(); fresh != o {
			c.render(fresh)
			return false
		}
		return true
	}
	choose := func(choice aiSpeedChoice) {
		if !current() {
			return
		}
		c.close()
		if choice.effort == "" {
			applyFasterModel(c.state, choice.provider, choice.model)
		}
		c.retry(choice)
	}
	grouped := !c.timeout && o.source.lowerEffort != "" && o.target.model != ""
	var actions []fyne.CanvasObject
	if effort := o.source.lowerEffort; effort != "" {
		choice := aiSpeedChoice{o.source.provider, o.source.model, effort}
		if c.timeout {
			actions = append(actions, fasterModelControl(retryEffortLabel(effort), func() { choose(choice) }))
		} else {
			title, detail := progressEffortCopy(effort)
			actions = append(actions, newAISwitchAction(title, detail, func() { choose(choice) }))
		}
	}
	if o.target.model != "" {
		choice := aiSpeedChoice{provider: o.target.provider, model: o.target.model}
		switchModel := func() { choose(choice) }
		if c.timeout {
			actions = append(actions, timeoutSwitchLink(c.state, o.target, switchModel))
		} else {
			actions = append(actions, newAISwitchAction("Switch to a faster model", switchModelDetail(o.target), switchModel))
		}
	}
	c.box.Objects = actions
	if grouped {
		if c.disclosure == nil {
			c.disclosure = newAISwitchDisclosure(c.state)
		}
		row := c.disclosure
		row.setExpanded(false)
		width := canvas.NewRectangle(color.Transparent)
		width.SetMinSize(fyne.NewSize(aiSwitchWidth, 1))
		rule := func() fyne.CanvasObject {
			return container.New(layout.NewCustomPaddedLayout(0, 0, 12, 12), widget.NewSeparator())
		}
		options := container.New(layout.NewCustomPaddedVBoxLayout(0), rule(), actions[0], rule(), actions[1])
		options.Hide()
		group := container.New(layout.NewCustomPaddedVBoxLayout(0), row, options)
		pal := c.state.pal()
		ground := canvas.NewRectangle(pal.Surface)
		ground.CornerRadius = crossRefSectionRadius
		if isDark() {
			// The sheet and the card are adjacent dark surfaces. Give the card
			// the same quiet edge as the other grouped controls.
			ground.StrokeColor = pal.Border
			ground.StrokeWidth = 1
		}

		c.box.Objects = []fyne.CanvasObject{container.NewCenter(container.NewStack(width, ground, group))}
		row.onTap = func() {
			if !current() {
				return
			}
			if options.Visible() {
				c.render(o)
				return
			}
			row.setExpanded(true)
			options.Show()
			c.refresh()
		}
	} else if len(actions) == 1 && !c.timeout {
		c.box.Objects = []fyne.CanvasObject{container.NewCenter(actions[0])}
	} else if len(actions) == 2 {
		// Keep the link itself when collapsing so keyboard focus stays on it.
		if c.toggle == nil {
			c.toggle = fasterModelLink("Faster options…", nil)
		}
		toggle := container.NewCenter(c.toggle)
		c.toggle.OnTapped = func() {
			if !current() {
				return
			}
			if len(c.box.Objects) == 1 {
				c.box.Objects = []fyne.CanvasObject{toggle, actions[0], actions[1]}
			} else {
				c.render(o) // evict the choices along with their callbacks
				return
			}
			c.refresh()
		}
		c.box.Objects = []fyne.CanvasObject{toggle}
	}
	if len(actions) == 0 {
		c.box.Hide()
	} else {
		c.box.Show()
	}
	c.refresh()
}

func (c *aiSpeedControl) refresh() {
	c.box.Refresh()
	if c.relayout != nil {
		c.relayout()
	}
}

func (c *aiSpeedControl) close() {
	if c != nil {
		c.closed = true
		c.box.Hide()
	}
}

const aiSwitchWidth float32 = 240

type aiSwitchDetailTheme struct{ fyne.Theme }

func (t aiSwitchDetailTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInnerPadding {
		return 0
	}
	if n == theme.SizeNameText {
		return crossRefGroupTextSize
	}
	return t.Theme.Size(n)
}
func aiSwitchText(note fyne.CanvasObject, width float32) fyne.CanvasObject {
	themed := container.NewThemeOverride(note, aiSwitchDetailTheme{fyne.CurrentApp().Settings().Theme()})
	themed.Resize(fyne.NewSize(width, themed.MinSize().Height))
	return container.NewGridWrap(fyne.NewSize(width, themed.MinSize().Height), themed)
}

// Each row owns one full-width hit region, including its note or disclosure mark.
type aiSwitchHit struct {
	widget.BaseWidget
	ring  *canvas.Rectangle
	onTap func()
}

func (r *aiSwitchHit) initFocus() {
	r.ring = canvas.NewRectangle(color.Transparent)
	r.ring.StrokeColor = theme.FocusColor()
	r.ring.CornerRadius = 6
}
func (r *aiSwitchHit) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}
func (r *aiSwitchHit) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeySpace || ev.Name == fyne.KeyReturn {
		r.Tapped(nil)
	}
}
func (r *aiSwitchHit) TypedRune(rune)         {}
func (r *aiSwitchHit) FocusGained()           { r.ring.StrokeWidth = 2; r.ring.Refresh() }
func (r *aiSwitchHit) FocusLost()             { r.ring.StrokeWidth = 0; r.ring.Refresh() }
func (r *aiSwitchHit) Cursor() desktop.Cursor { return desktop.PointerCursor }

type aiSwitchDisclosure struct {
	aiSwitchHit
	title, second *canvas.Text
	arrow         *widget.Icon
	expanded      bool
}

func newAISwitchDisclosure(state *AppState) *aiSwitchDisclosure {
	pal := state.pal()
	r := &aiSwitchDisclosure{}
	r.initFocus()
	r.title = canvas.NewText("Switch to a faster model", pal.Text)
	r.second = canvas.NewText("or reasoning effort", pal.Text)
	r.title.TextSize = crossRefGroupTextSize
	r.second.TextSize = crossRefGroupTextSize
	r.title.Alignment = fyne.TextAlignCenter
	r.second.Alignment = fyne.TextAlignCenter
	r.arrow = widget.NewIcon(crossRefGroupIcon(false))
	r.ExtendBaseWidget(r)
	return r
}
func (r *aiSwitchDisclosure) setExpanded(open bool) {
	r.expanded = open
	r.arrow.SetResource(crossRefGroupIcon(open))
}
func aiSwitchBand(h float32, words fyne.CanvasObject, ring *canvas.Rectangle) fyne.WidgetRenderer {
	band := canvas.NewRectangle(color.Transparent)
	band.SetMinSize(fyne.NewSize(aiSwitchWidth, h))
	return widget.NewSimpleRenderer(container.NewStack(band, ring, words))
}
func (r *aiSwitchDisclosure) CreateRenderer() fyne.WidgetRenderer {
	lines := container.New(layout.NewCustomPaddedVBoxLayout(0), layout.NewSpacer(), r.title, r.second, layout.NewSpacer())
	// Reserve the same space opposite the arrow so both lines are centered
	// on the card, including when the arrow rotates.
	content := container.NewBorder(nil, nil, hgap(crossRefFoldIconSize), crossRefFoldIcon(r.arrow), lines)
	return aiSwitchBand(48, container.New(layout.NewCustomPaddedLayout(5, 5, 6, 6), content), r.ring)
}

type aiSwitchActionRow struct {
	aiSwitchHit
	label  *widget.TextSegment
	detail string
}

func newAISwitchAction(label, detail string, apply func()) *aiSwitchActionRow {
	r := &aiSwitchActionRow{detail: detail}
	r.initFocus()
	r.onTap = apply
	r.label = &widget.TextSegment{Text: label, Style: widget.RichTextStyle{ColorName: theme.ColorNameHyperlink, SizeName: theme.SizeNameText}}
	r.ExtendBaseWidget(r)
	return r
}
func (r *aiSwitchActionRow) CreateRenderer() fyne.WidgetRenderer {
	const left, right = 12, 12
	title := widget.NewRichText(r.label)
	title.Wrapping = fyne.TextWrapWord
	const width = aiSwitchWidth - left - right
	body := container.New(layout.NewCustomPaddedVBoxLayout(1), layout.NewSpacer(), aiSwitchText(title, width), aiSwitchText(caption(r.detail), width), layout.NewSpacer())
	return aiSwitchBand(minTapTarget, container.New(layout.NewCustomPaddedLayout(6, 6, left, right), body), r.ring)
}
