package bibletext

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func aiErrorDetail(p aiErrorPresentation) fyne.CanvasObject {
	text := widget.NewRichText(&widget.TextSegment{Text: p.detail,
		Style: widget.RichTextStyle{SizeName: theme.SizeNameCaptionText, ColorName: colorNameMuted}})
	text.Wrapping = fyne.TextWrapWord
	return text
}

func aiBillingLink(p aiErrorPresentation) fyne.CanvasObject {
	u, err := url.Parse(p.billingURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil
	}
	return newOutboundLink(p.provider+" billing", u)
}

// A timeout keeps its destination, saved effort and Settings change together in
// the action. Its height is one row wherever the text fits, leaving short
// landscape sheets the same room for the error and its separate Try again row.
func timeoutSwitchLink(state *AppState, target aiModelInUse, activate func()) fyne.CanvasObject {
	label := "Set " + target.model + " in Settings"
	if target.effortLabel != "" {
		label += " · " + target.effortLabel
	}
	link := fasterModelLink(label, activate)
	natural := link.MinSize().Width
	link.Alignment = fyne.TextAlignCenter
	link.Wrapping = fyne.TextWrapWord
	link.Refresh()
	return container.New(timeoutLinkLayout{link, natural, state.window}, link)
}

type timeoutLinkLayout struct {
	link    *widget.Hyperlink
	natural float32
	window  fyne.Window
}

func (l timeoutLinkLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	width := l.natural
	if l.window != nil {
		width = min(width, aiPanelSize(l.window.Canvas().Size()).Width-64)
	}
	if l.link.Size().Width != width {
		l.link.Resize(fyne.NewSize(width, l.link.MinSize().Height))
	}
	return fyne.NewSize(width, l.link.MinSize().Height)
}

func (l timeoutLinkLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	measured := l.MinSize(objects)
	l.link.Resize(fyne.NewSize(min(measured.Width, size.Width), size.Height))
	l.link.Move(fyne.NewPos(max(0, (size.Width-measured.Width)/2), 0))
}

func (c *aiSpeedControl) timeoutSwitch(p aiErrorPresentation) fyne.CanvasObject {
	if c == nil || !p.timeout || p.providerID == "" || c.provider != p.providerID || len(c.box.Objects) == 0 {
		return nil
	}
	return c.box
}

func (c *aiSpeedControl) tryAgain() {
	if c == nil || c.closed || c.retry == nil || (c.active != nil && !c.active()) {
		return
	}
	if c.state != nil && c.state.window != nil && c.state.window.Canvas() != nil && c.tryBtn != nil &&
		c.state.window.Canvas().Focused() == fyne.Focusable(c.tryBtn) {
		c.state.requestWaitFocusRestore()
	}
	next := validatedSpeedChoice(c.state, c.choice)
	if next.effort != "" && c.state != nil {
		if aiReasoningEffort(c.state.keys().prefs, next.provider, next.model) != c.offer.savedEffort {
			next = aiSpeedChoice{}
		}
	}
	c.close()
	c.retry(next)
	c.state.consumeWaitFocusRestore(nil)
}

const aiTimeoutChoicesMeasure float32 = 280

type timeoutChoicesLayout struct {
	control *aiSpeedControl
}

func (l timeoutChoicesLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 || l.control == nil || !objs[0].Visible() {
		return fyne.Size{}
	}
	if l.control.keyTest {
		return objs[0].MinSize()
	}
	w := aiTimeoutChoicesMeasure
	if lay, ok := l.control.box.Layout.(*aiSpeedRowsLayout); ok && lay.width > 0 {
		w = min(aiTimeoutChoicesMeasure, lay.width)
	}
	l.control.setWidth(w)
	return fyne.NewSize(w, objs[0].MinSize().Height)
}

func (l timeoutChoicesLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 || l.control == nil || !objs[0].Visible() {
		return
	}
	if l.control.keyTest {
		objs[0].Move(fyne.NewPos(0, 0))
		objs[0].Resize(size)
		return
	}
	m := aiTimeoutChoicesMeasure
	if size.Width > 0 {
		m = min(aiTimeoutChoicesMeasure, size.Width)
	}
	l.control.setWidth(m)
	h := objs[0].MinSize().Height
	objs[0].Move(fyne.NewPos(max(0, (size.Width-m)/2), 0))
	objs[0].Resize(fyne.NewSize(m, h))
	if c, ok := objs[0].(*fyne.Container); ok && c.Layout != nil {
		c.Layout.Layout(c.Objects, fyne.NewSize(m, h))
	}
}

// The message and its actions share the same validated switch offer.
func aiErrorRecovery(p aiErrorPresentation, control *aiSpeedControl) (string, *fyne.Container) {
	border := lightPalette.Border
	if control != nil && control.state != nil {
		border = control.state.pal().Border
	}
	tryBtn := widget.NewButton("Try again", control.tryAgain)
	if control != nil {
		control.tryBtn = tryBtn
	}
	items := []fyne.CanvasObject{container.NewCenter(inputFrame(tryBtn, border))}
	if link := aiBillingLink(p); link != nil {
		items = append(items, container.NewCenter(link))
	}
	faster := control.timeoutSwitch(p)
	if faster != nil {
		items = append(items, container.New(timeoutChoicesLayout{control: control}, faster))
	}
	fasterModelOffered := faster != nil && control != nil && control.offer.target.model != ""
	return p.shown(fasterModelOffered), container.NewVBox(items...)
}

func aiSearchErrorView(state *AppState, err error, retry func(aiSpeedChoice)) fyne.CanvasObject {
	if state.findSpeed != nil {
		state.findSpeed.close()
	}
	control := newAITimeoutSpeedControl(state, func(next aiSpeedChoice) {
		state.aiSearchRestarted = false
		if retry != nil {
			retry(next)
		}
	}, state.aiSearchChoice, state.aiSearchLastModel)
	state.findSpeed = control
	gen, query := state.askSession.gen, state.aiSearchQuery
	control.active = func() bool {
		return state.findSpeed == control && state.askSession.Current(gen) && state.aiSearchActive &&
			state.aiSearchErr != nil && !state.aiSearchLoading && state.aiSearchQuery == query
	}
	p, special := aiSpecialError(err)
	if !special {
		_, actions := aiErrorRecovery(aiErrorPresentation{}, control)
		lbl := widget.NewLabel(friendlyAIError(err))
		lbl.Wrapping = fyne.TextWrapWord
		lbl.Alignment = fyne.TextAlignCenter
		return findWaitScroll(container.NewVBox(wrappedParagraph(lbl, 300), spacer(12), actions))
	}
	message, actions := aiErrorRecovery(p, control)
	lbl := widget.NewLabel(message)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Alignment = fyne.TextAlignCenter
	items := []fyne.CanvasObject{wrappedParagraph(lbl, 300)}
	if p.detail != "" {
		detail := aiErrorDetail(p)
		detail.Resize(fyne.NewSize(300, detail.MinSize().Height))
		items = append(items, container.NewGridWrap(fyne.NewSize(300, detail.MinSize().Height), detail))
	}
	items = append(items, spacer(12), actions)
	return findWaitScroll(container.NewVBox(items...))
}
