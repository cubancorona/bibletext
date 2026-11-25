package bibletext

import (
	"image/color"
	"slices"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// aiSpeedNow is the clock read by the 500ms tap-arm guard on choice rows.
// Tests advance a synthetic clock through it without sleeping.
var aiSpeedNow = time.Now

const (
	aiSpeedArmDelay        = 500 * time.Millisecond
	aiSwitchRowSideInset   = float32(8)
	aiSwitchRowTitleTop    = float32(5)
	aiSwitchRowNoteTop     = float32(23)
	aiSwitchTitleTextSize  = float32(14)
	aiSwitchNoteTextSize   = float32(11)
	aiSwitchDisclosureIcon = float32(14)
	aiSwitchDisclosureGap  = float32(7)
	aiSwitchWashRadius     = float32(8)
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
	disclosure           *aiSwitchDisclosure
	rows                 []*aiSwitchActionRow
	previewRows          []*aiSwitchActionRow
	tryBtn               *widget.Button
	retry                func(aiSpeedChoice)
	relayout             func()
	onExpand             func()
	active               func() bool
	provider, credential string
	keySeq               uint64
	requiresGrant        bool
	timeout              bool
	keyTest              bool
	choice               aiSpeedChoice
	source               aiModelInUse
	offer                aiSpeedOffer
	generation           int
	closed               bool
}

func newAISpeedControl(state *AppState, retry func(aiSpeedChoice)) *aiSpeedControl {
	return newAISpeedControlForChoice(state, retry, aiSpeedChoice{})
}

func newAISpeedControlForChoice(state *AppState, retry func(aiSpeedChoice), choice aiSpeedChoice) *aiSpeedControl {
	seed := seedAISpeedReport(state, choice)
	return buildAISpeedControl(state, retry, false, false, choice, seed)
}

func newAITimeoutSpeedControl(state *AppState, retry func(aiSpeedChoice), choice aiSpeedChoice, lastReport aiModelInUse) *aiSpeedControl {
	if lastReport.model == "" {
		lastReport = seedAISpeedReport(state, choice)
	}
	return buildAISpeedControl(state, retry, true, false, choice, lastReport)
}

// makeAISpeedControl retains the three-argument constructor used by Settings'
// key-test failure view, where timeout mode renders a compact inline link.
func makeAISpeedControl(state *AppState, retry func(aiSpeedChoice), timeout bool) *aiSpeedControl {
	seed := seedAISpeedReport(state, aiSpeedChoice{})
	return buildAISpeedControl(state, retry, timeout, timeout, aiSpeedChoice{}, seed)
}

func buildAISpeedControl(
	state *AppState,
	retry func(aiSpeedChoice),
	timeout, keyTest bool,
	choice aiSpeedChoice,
	seed aiModelInUse,
) *aiSpeedControl {
	lay := &aiSpeedRowsLayout{width: aiWaitMeasure}
	c := &aiSpeedControl{
		state:   state,
		box:     container.New(lay),
		retry:   retry,
		timeout: timeout,
		keyTest: keyTest,
		choice:  choice,
	}
	c.box.Hide()
	if retry != nil && state != nil && aiFeaturesEnabled(state) {
		store := state.keys()
		c.provider = store.activeProvider()
		key := providerAPIKey(store, c.provider)
		c.credential = aiAllowedRecord(c.provider, key)
		c.requiresGrant = store.aiAllowed(c.provider, key)
		c.keySeq = store.seq()
		if seed.model == "" {
			seed = aiModelInUse{provider: c.provider, model: activeModelFor(store, c.provider)}
		}
		c.show(seed)
	}
	return c
}

// validatedSpeedChoice preserves a still-valid one-time speed choice across
// Try again, falling back to saved Settings if the provider, active model or
// supported effort levels have changed.
func validatedSpeedChoice(state *AppState, choice aiSpeedChoice) aiSpeedChoice {
	if state == nil || !aiFeaturesEnabled(state) || choice.model == "" || choice.effort == "" {
		return aiSpeedChoice{}
	}
	store := state.keys()
	id := store.activeProvider()
	if choice.provider != id || activeModelFor(store, id) != choice.model {
		return aiSpeedChoice{}
	}
	if !slices.Contains(modelEffortChoices(store.prefs, id, choice.model), choice.effort) {
		return aiSpeedChoice{}
	}
	return choice
}

func (c *aiSpeedControl) setWidth(width float32) {
	if c == nil {
		return
	}
	if lay, ok := c.box.Layout.(*aiSpeedRowsLayout); ok {
		lay.width = width
	}
	if c.disclosure != nil {
		if lay, ok := c.disclosure.box.Layout.(*aiSwitchDisclosureLayout); ok {
			lay.width = width
		}
	}
	for _, r := range c.rows {
		if lay, ok := r.box.Layout.(*aiSwitchActionLayout); ok {
			lay.width = width
		}
	}
	for _, r := range c.previewRows {
		if lay, ok := r.box.Layout.(*aiSwitchActionLayout); ok {
			lay.width = width
		}
	}
}

func (c *aiSpeedControl) closedHeight() float32 {
	if c == nil || !c.box.Visible() || len(c.box.Objects) == 0 {
		return 0
	}
	if !c.timeout && c.disclosure != nil && len(c.box.Objects) > 0 && c.box.Objects[0] == c.disclosure.box {
		return c.disclosure.box.MinSize().Height
	}
	return c.box.MinSize().Height
}

func (c *aiSpeedControl) openHeight() float32 {
	if c == nil || !c.box.Visible() || len(c.box.Objects) == 0 {
		return 0
	}
	if !c.timeout && c.disclosure != nil && len(c.box.Objects) > 0 && c.box.Objects[0] == c.disclosure.box {
		h := c.disclosure.box.MinSize().Height
		active := c.rows
		if len(active) == 0 {
			active = c.previewRows
		}
		for _, r := range active {
			h += r.box.MinSize().Height
		}
		return h
	}
	return c.box.MinSize().Height
}

func (c *aiSpeedControl) firstFocusable() fyne.Focusable {
	if c == nil || c.closed || !c.box.Visible() {
		return nil
	}
	if !c.timeout && c.disclosure != nil && len(c.box.Objects) > 0 && c.box.Objects[0] == c.disclosure.box {
		return c.disclosure
	}
	if len(c.rows) > 0 {
		return c.rows[0]
	}
	return nil
}

func (c *aiSpeedControl) credentialStillValid(store *keyStore) bool {
	if c.requiresGrant && store.prefs != nil && store.prefs.String(prefAIAllowedPrefix+c.provider) != c.credential {
		return false
	}
	if store.seq() == c.keySeq {
		return true
	}
	key := providerAPIKey(store, c.provider)
	if aiAllowedRecord(c.provider, key) != c.credential || (c.requiresGrant && !store.aiAllowed(c.provider, key)) {
		return false
	}
	c.keySeq = store.seq()
	return true
}

func (c *aiSpeedControl) options() (o aiSpeedOffer) {
	if c.closed || c.retry == nil || c.credential == "" || c.state == nil || !aiFeaturesEnabled(c.state) || c.source.provider != c.provider {
		return o
	}
	store := c.state.keys()
	if store.activeProvider() != c.provider || activeModelFor(store, c.provider) != c.source.model || !c.credentialStillValid(store) {
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

// effortNoteLabel formats a reasoning level ("Medium effort", "Extra high effort")
// or numeric token budget ("4,096 tokens", "Off") without appending " effort"
// to numeric budgets or "none".
func effortNoteLabel(effort string) string {
	if strings.EqualFold(effort, "none") {
		return "No reasoning"
	}
	label := effortDisplayName(effort)
	if _, err := strconv.Atoi(effort); err == nil {
		return label
	}
	return label + " effort"
}

func fasterModelNoteLabel(target aiModelInUse) string {
	return target.model + " · saves in Settings"
}

func waitingEffortCopy(effort string) (title, note string) {
	return "Use less reasoning this time", effortNoteLabel(effort)
}

func timeoutEffortCopy(effort string) (title, note string) {
	return "Try again with less reasoning", effortNoteLabel(effort)
}

func (c *aiSpeedControl) render(o aiSpeedOffer) {
	c.offer = o
	c.generation++
	generation := c.generation
	// A rebuilt view, a new attempt, or a change in credentials or options
	// invalidates a row even if a queued tap still holds it.
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
		if c.hasFocusedControl() {
			c.state.requestWaitFocusRestore()
		}
		c.close()
		if choice.effort == "" {
			applyFasterModel(c.state, choice.provider, choice.model)
		}
		c.retry(choice)
		c.state.consumeWaitFocusRestore(nil)
	}

	if c.timeout && c.keyTest {
		c.rows = nil
		c.previewRows = nil
		var items []fyne.CanvasObject
		if o.target.model != "" {
			choice := aiSpeedChoice{provider: o.target.provider, model: o.target.model}
			items = append(items, timeoutSwitchLink(c.state, o.target, func() { choose(choice) }))
		}
		c.box.Layout = &aiSpeedRowsLayout{width: aiWaitMeasure}
		c.box.Objects = items
		if len(items) == 0 {
			c.box.Hide()
		} else {
			c.box.Show()
		}
		c.refresh()
		return
	}

	buildRows := func() []*aiSwitchActionRow {
		var rows []*aiSwitchActionRow
		if effort := o.source.lowerEffort; effort != "" {
			choice := aiSpeedChoice{o.source.provider, o.source.model, effort}
			title, note := waitingEffortCopy(effort)
			if c.timeout {
				title, note = timeoutEffortCopy(effort)
			}
			rows = append(rows, newAISwitchAction(c.state, title, note, func() { choose(choice) }))
		}
		if o.target.model != "" {
			choice := aiSpeedChoice{provider: o.target.provider, model: o.target.model}
			title := "Switch to a faster model"
			if c.timeout {
				title = "Switch to a faster model and try again"
			}
			rows = append(rows, newAISwitchAction(c.state, title, fasterModelNoteLabel(o.target), func() { choose(choice) }))
		}
		return rows
	}

	rows := buildRows()
	c.rows = rows
	c.previewRows = nil
	grouped := !c.timeout && len(rows) == 2

	width := aiWaitMeasure
	if old, ok := c.box.Layout.(*aiSpeedRowsLayout); ok && old.width > 0 {
		width = old.width
	}
	c.box.Layout = &aiSpeedRowsLayout{width: width}

	switch {
	case len(rows) == 0:
		c.box.Objects = nil
		c.box.Hide()
	case grouped:
		if c.disclosure == nil {
			c.disclosure = newAISwitchDisclosure(c.state)
		}
		toggle := c.disclosure
		toggle.setExpanded(false)
		c.rows = nil
		c.previewRows = rows
		c.setWidth(width)
		c.box.Objects = []fyne.CanvasObject{toggle.box}
		c.box.Show()
		toggle.onTap = func() {
			if !current() {
				return
			}
			if toggle.expanded {
				// Re-render closed while keeping toggle mounted so focus stays on it
				// and any references to the previously expanded rows become inert.
				c.render(o)
				return
			}
			freshRows := buildRows()
			c.rows = freshRows
			c.setWidth(width)
			toggle.setExpanded(true)
			objs := make([]fyne.CanvasObject, 0, 1+len(freshRows))
			objs = append(objs, toggle.box)
			for _, r := range freshRows {
				objs = append(objs, r.box)
			}
			c.box.Objects = objs
			c.refresh()
			if c.onExpand != nil {
				c.onExpand()
			}
		}
	default:
		c.setWidth(width)
		objs := make([]fyne.CanvasObject, 0, len(rows))
		for _, r := range rows {
			objs = append(objs, r.box)
		}
		c.box.Objects = objs
		c.box.Show()
	}
	c.refresh()
}

func (c *aiSpeedControl) hasFocusedControl() bool {
	if c == nil || c.state == nil || c.state.window == nil || c.state.window.Canvas() == nil {
		return false
	}
	focused := c.state.window.Canvas().Focused()
	if focused == nil {
		return false
	}
	if c.disclosure != nil && focused == fyne.Focusable(c.disclosure) {
		return true
	}
	for _, r := range c.rows {
		if focused == fyne.Focusable(r) {
			return true
		}
	}
	return false
}

func (s *AppState) requestWaitFocusRestore() {
	if s != nil {
		s.restoreWaitFocus = true
	}
}

func (s *AppState) consumeWaitFocusRestore(root fyne.CanvasObject) {
	if s == nil || !s.restoreWaitFocus {
		return
	}
	s.restoreWaitFocus = false
	if s.window == nil || s.window.Canvas() == nil {
		return
	}
	cnv := s.window.Canvas()
	if root == nil && s.findSpeed != nil && !s.findSpeed.closed {
		if f := s.findSpeed.firstFocusable(); f != nil {
			cnv.Focus(f)
			return
		}
	}
	if root == nil {
		root = cnv.Content()
		if top := cnv.Overlays().Top(); top != nil && top.Visible() {
			root = top
		}
	}
	if target := findWaitLiveFocusable(root); target != nil {
		cnv.Focus(target)
	}
}

func findWaitLiveFocusable(root fyne.CanvasObject) fyne.Focusable {
	var disc *aiSwitchDisclosure
	var act *aiSwitchActionRow
	var cancelBtn *widget.Button
	var visit func(fyne.CanvasObject)
	visit = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *aiSwitchDisclosure:
			if disc == nil {
				disc = v
			}
		case *aiSwitchActionRow:
			if act == nil {
				act = v
			}
		case *widget.Button:
			if cancelBtn == nil && (v.Text == "Cancel" || v.Text == "Close") {
				cancelBtn = v
			}
		case *fyne.Container:
			for _, child := range v.Objects {
				visit(child)
			}
		case *container.Scroll:
			visit(v.Content)
		case *container.ThemeOverride:
			visit(v.Content)
		case *widget.PopUp:
			visit(v.Content)
		}
	}
	visit(root)
	if disc != nil {
		return disc
	}
	if act != nil {
		return act
	}
	if cancelBtn != nil {
		return cancelBtn
	}
	return nil
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

type aiSpeedRowsLayout struct {
	width float32
}

func (l *aiSpeedRowsLayout) effectiveWidth() float32 {
	if l.width <= 0 {
		return aiWaitMeasure
	}
	return l.width
}

func (l *aiSpeedRowsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	w := l.effectiveWidth()
	h := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		if c, ok := o.(*fyne.Container); ok {
			switch rl := c.Layout.(type) {
			case *aiSwitchActionLayout:
				rl.width = w
			case *aiSwitchDisclosureLayout:
				rl.width = w
			}
		}
		h += o.MinSize().Height
	}
	return fyne.NewSize(w, h)
}

func (l *aiSpeedRowsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := size.Width
	if w <= 0 {
		w = l.effectiveWidth()
	}
	l.width = w
	y := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		if c, ok := o.(*fyne.Container); ok {
			switch rl := c.Layout.(type) {
			case *aiSwitchActionLayout:
				rl.width = w
			case *aiSwitchDisclosureLayout:
				rl.width = w
			}
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(w, h))
		if c, ok := o.(*fyne.Container); ok && c.Layout != nil {
			c.Layout.Layout(c.Objects, fyne.NewSize(w, h))
		}
		y += h
	}
}

// aiSwitchHit implements the shared whole-row hit target, hover wash, press
// flash, focus ring and Space/Return keyboard activation without any resting
// fill or stroke in either theme.
type aiSwitchHit struct {
	widget.BaseWidget
	state      *AppState
	wash, ring *canvas.Rectangle
	onTap      func()
	armedAt    time.Time
	guard      bool
	hovered    bool
	pressed    bool
	focused    bool
}

func (r *aiSwitchHit) initHit(state *AppState, guard bool) {
	r.state = state
	r.guard = guard
	if guard {
		r.rearm()
	}
	r.wash = canvas.NewRectangle(color.Transparent)
	r.wash.CornerRadius = aiSwitchWashRadius
	r.ring = canvas.NewRectangle(color.Transparent)
	r.ring.StrokeColor = theme.Color(theme.ColorNameFocus)
	r.ring.CornerRadius = aiSwitchWashRadius
}

func (r *aiSwitchHit) rearm() {
	if r.guard {
		r.armedAt = aiSpeedNow().Add(aiSpeedArmDelay)
	}
}

func (r *aiSwitchHit) pal() palette {
	if r.state != nil {
		return r.state.pal()
	}
	if isDark() {
		return darkPalette
	}
	return lightPalette
}

func (r *aiSwitchHit) syncChrome() {
	switch {
	case r.pressed:
		r.wash.FillColor = theme.Color(theme.ColorNamePressed)
	case r.hovered:
		r.wash.FillColor = paletteHover(r.pal())
	default:
		r.wash.FillColor = color.Transparent
	}
	r.wash.Refresh()

	r.ring.StrokeColor = theme.Color(theme.ColorNameFocus)
	if r.focused {
		r.ring.StrokeWidth = 2
	} else {
		r.ring.StrokeWidth = 0
	}
	r.ring.Refresh()
}

func (r *aiSwitchHit) CreateRenderer() fyne.WidgetRenderer {
	return &aiSwitchHitRenderer{hit: r, rect: canvas.NewRectangle(color.Transparent)}
}

type aiSwitchHitRenderer struct {
	hit  *aiSwitchHit
	rect *canvas.Rectangle
}

func (hr *aiSwitchHitRenderer) Layout(size fyne.Size)        { hr.rect.Resize(size) }
func (hr *aiSwitchHitRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, minTapTarget) }
func (hr *aiSwitchHitRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{hr.rect} }
func (hr *aiSwitchHitRenderer) Destroy()                     {}
func (hr *aiSwitchHitRenderer) Refresh()                     { hr.hit.syncChrome() }

func (r *aiSwitchHit) Tapped(*fyne.PointEvent) {
	r.pressed = false
	r.syncChrome()
	if r.onTap == nil || (r.guard && aiSpeedNow().Before(r.armedAt)) {
		return
	}
	r.onTap()
}

func (r *aiSwitchHit) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeySpace || ev.Name == fyne.KeyReturn {
		r.Tapped(nil)
	}
}

func (r *aiSwitchHit) TypedRune(rune) {}

func (r *aiSwitchHit) FocusGained() {
	r.focused = true
	r.syncChrome()
}

func (r *aiSwitchHit) FocusLost() {
	r.focused = false
	r.syncChrome()
}

func (r *aiSwitchHit) MouseIn(*desktop.MouseEvent) {
	r.hovered = true
	r.syncChrome()
}

func (r *aiSwitchHit) MouseMoved(*desktop.MouseEvent) {}

func (r *aiSwitchHit) MouseOut() {
	r.hovered = false
	r.pressed = false
	r.syncChrome()
}

func (r *aiSwitchHit) MouseDown(*desktop.MouseEvent) {
	if r.guard && aiSpeedNow().Before(r.armedAt) {
		return
	}
	r.pressed = true
	r.syncChrome()
}

func (r *aiSwitchHit) MouseUp(*desktop.MouseEvent) {
	if r.pressed {
		r.pressed = false
		r.syncChrome()
	}
}

func (r *aiSwitchHit) Cursor() desktop.Cursor { return desktop.PointerCursor }

var (
	_ fyne.Tappable      = (*aiSwitchDisclosure)(nil)
	_ fyne.Focusable     = (*aiSwitchDisclosure)(nil)
	_ desktop.Hoverable  = (*aiSwitchDisclosure)(nil)
	_ desktop.Mouseable  = (*aiSwitchDisclosure)(nil)
	_ desktop.Cursorable = (*aiSwitchDisclosure)(nil)
	_ fyne.Tappable      = (*aiSwitchActionRow)(nil)
	_ fyne.Focusable     = (*aiSwitchActionRow)(nil)
	_ desktop.Hoverable  = (*aiSwitchActionRow)(nil)
	_ desktop.Mouseable  = (*aiSwitchActionRow)(nil)
	_ desktop.Cursorable = (*aiSwitchActionRow)(nil)
)

// aiSwitchDisclosure is the quiet centered "Faster options" toggle row
// with a trailing 14pt stroke chevron (down when closed, up when open).
type aiSwitchDisclosure struct {
	aiSwitchHit
	title    *canvas.Text
	arrow    *widget.Icon
	box      *fyne.Container
	expanded bool
}

func newAISwitchDisclosure(state *AppState) *aiSwitchDisclosure {
	r := &aiSwitchDisclosure{}
	r.initHit(state, false)
	pal := r.pal()
	r.title = canvas.NewText("Faster options", pal.Text)
	r.title.TextSize = aiSwitchTitleTextSize
	r.arrow = widget.NewIcon(iconDisclosureExpand)
	r.ExtendBaseWidget(r)
	lay := &aiSwitchDisclosureLayout{row: r, width: aiWaitMeasure}
	r.box = container.New(lay, r.wash, r.title, r.arrow, r.ring, r)
	return r
}

func (r *aiSwitchDisclosure) setExpanded(open bool) {
	r.expanded = open
	if open {
		r.arrow.SetResource(iconDisclosureCollapse)
	} else {
		r.arrow.SetResource(iconDisclosureExpand)
	}
}

type aiSwitchDisclosureLayout struct {
	row   *aiSwitchDisclosure
	width float32
}

func (l *aiSwitchDisclosureLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	w := l.width
	if w <= 0 {
		w = aiWaitMeasure
	}
	return fyne.NewSize(w, minTapTarget)
}

func (l *aiSwitchDisclosureLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	l.width = size.Width
	r.title.Color = r.pal().Text
	for _, o := range []fyne.CanvasObject{r.wash, r.ring, r} {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
	th := r.title.MinSize().Height
	tw := fyne.MeasureText(r.title.Text, r.title.TextSize, fyne.TextStyle{}).Width
	groupW := tw + aiSwitchDisclosureGap + aiSwitchDisclosureIcon
	x := max(aiSwitchRowSideInset, (size.Width-groupW)/2)
	titleY := (size.Height - th) / 2
	r.title.Move(fyne.NewPos(x, titleY))
	r.title.Resize(fyne.NewSize(tw, th))
	iconY := titleY + (th-aiSwitchDisclosureIcon)/2 + 0.5
	r.arrow.Move(fyne.NewPos(x+tw+aiSwitchDisclosureGap, iconY))
	r.arrow.Resize(fyne.NewSize(aiSwitchDisclosureIcon, aiSwitchDisclosureIcon))
}

// aiSwitchActionRow is one plain centered choice row: 14pt accent title over an
// 11pt muted scope note, with 8pt side insets, word-wrapping when needed, and a
// 500ms tap-arm delay.
type aiSwitchActionRow struct {
	aiSwitchHit
	titleText  string
	detail     string
	titleLines []*canvas.Text
	noteLines  []*canvas.Text
	box        *fyne.Container
}

func newAISwitchAction(state *AppState, title, detail string, apply func()) *aiSwitchActionRow {
	r := &aiSwitchActionRow{titleText: title, detail: detail}
	r.initHit(state, true)
	r.onTap = apply
	r.ExtendBaseWidget(r)
	lay := &aiSwitchActionLayout{row: r, width: aiWaitMeasure}
	r.box = container.New(lay)
	lay.syncObjects(aiWaitMeasure)
	return r
}

type aiSwitchActionLayout struct {
	row   *aiSwitchActionRow
	width float32
}

func wrapSpeedRowLines(text string, textSize, maxW float32) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxW <= 0 || fyne.MeasureText(text, textSize, fyne.TextStyle{}).Width <= maxW {
		return []string{text}
	}
	splitLongToken := func(tok string) []string {
		if fyne.MeasureText(tok, textSize, fyne.TextStyle{}).Width <= maxW {
			return []string{tok}
		}
		var parts []string
		rem := tok
		for rem != "" {
			if fyne.MeasureText(rem, textSize, fyne.TextStyle{}).Width <= maxW {
				parts = append(parts, rem)
				break
			}
			// Prefer breaking right after a hyphen when one fits within maxW.
			bestHyphen := -1
			bestRune := 0
			runes := []rune(rem)
			for i := 1; i <= len(runes); i++ {
				sub := string(runes[:i])
				if fyne.MeasureText(sub, textSize, fyne.TextStyle{}).Width > maxW {
					break
				}
				bestRune = i
				if runes[i-1] == '-' {
					bestHyphen = i
				}
			}
			cut := bestHyphen
			if cut <= 0 {
				cut = max(1, bestRune)
			}
			parts = append(parts, string(runes[:cut]))
			rem = string(runes[cut:])
		}
		return parts
	}

	var tokens []string
	for _, w := range strings.Fields(text) {
		tokens = append(tokens, splitLongToken(w)...)
	}
	var lines []string
	cur := tokens[0]
	for _, tok := range tokens[1:] {
		cand := cur + " " + tok
		if fyne.MeasureText(cand, textSize, fyne.TextStyle{}).Width <= maxW {
			cur = cand
		} else {
			lines = append(lines, cur)
			cur = tok
		}
	}
	lines = append(lines, cur)
	return lines
}

func (l *aiSwitchActionLayout) syncObjects(width float32) (tLines, nLines []string, th, nh, totalH float32) {
	r := l.row
	if width <= 0 {
		width = aiWaitMeasure
	}
	textW := max(32, width-2*aiSwitchRowSideInset)
	tLines = wrapSpeedRowLines(r.titleText, aiSwitchTitleTextSize, textW)
	nLines = wrapSpeedRowLines(r.detail, aiSwitchNoteTextSize, textW)
	th = fyne.MeasureText("M", aiSwitchTitleTextSize, fyne.TextStyle{}).Height
	nh = fyne.MeasureText("M", aiSwitchNoteTextSize, fyne.TextStyle{}).Height

	pal := r.pal()
	for len(r.titleLines) < len(tLines) {
		txt := canvas.NewText("", pal.Accent)
		txt.TextSize = aiSwitchTitleTextSize
		txt.Alignment = fyne.TextAlignCenter
		r.titleLines = append(r.titleLines, txt)
	}
	r.titleLines = r.titleLines[:len(tLines)]
	for i, s := range tLines {
		r.titleLines[i].Text = s
		r.titleLines[i].Color = pal.Accent
	}

	for len(r.noteLines) < len(nLines) {
		txt := canvas.NewText("", pal.TextMuted)
		txt.TextSize = aiSwitchNoteTextSize
		txt.Alignment = fyne.TextAlignCenter
		r.noteLines = append(r.noteLines, txt)
	}
	r.noteLines = r.noteLines[:len(nLines)]
	for i, s := range nLines {
		r.noteLines[i].Text = s
		r.noteLines[i].Color = pal.TextMuted
	}

	objs := make([]fyne.CanvasObject, 0, 3+len(r.titleLines)+len(r.noteLines))
	objs = append(objs, r.wash)
	for _, t := range r.titleLines {
		objs = append(objs, t)
	}
	for _, n := range r.noteLines {
		objs = append(objs, n)
	}
	objs = append(objs, r.ring, r)
	r.box.Objects = objs

	tCount := max(1, len(tLines))
	noteStart := aiSwitchRowNoteTop + float32(tCount-1)*th
	bottomPad := minTapTarget - (aiSwitchRowNoteTop + nh)
	end := aiSwitchRowTitleTop + float32(tCount)*th + bottomPad
	if len(nLines) > 0 {
		end = noteStart + float32(len(nLines))*nh + bottomPad
	}
	totalH = max(minTapTarget, end)
	return tLines, nLines, th, nh, totalH
}

func (l *aiSwitchActionLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	w := l.width
	if w <= 0 {
		w = aiWaitMeasure
	}
	_, _, _, _, totalH := l.syncObjects(w)
	return fyne.NewSize(w, totalH)
}

func (l *aiSwitchActionLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r := l.row
	l.width = size.Width
	tLines, _, th, nh, _ := l.syncObjects(size.Width)
	for _, o := range []fyne.CanvasObject{r.wash, r.ring, r} {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
	textW := max(0, size.Width-2*aiSwitchRowSideInset)
	for i, txt := range r.titleLines {
		txt.Move(fyne.NewPos(aiSwitchRowSideInset, aiSwitchRowTitleTop+float32(i)*th))
		txt.Resize(fyne.NewSize(textW, th))
	}
	tCount := max(1, len(tLines))
	noteStart := aiSwitchRowNoteTop + float32(tCount-1)*th
	for i, txt := range r.noteLines {
		txt.Move(fyne.NewPos(aiSwitchRowSideInset, noteStart+float32(i)*nh))
		txt.Resize(fyne.NewSize(textW, nh))
	}
}
