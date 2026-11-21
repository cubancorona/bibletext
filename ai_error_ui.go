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
	if !p.timeout || p.providerID == "" || c.provider != p.providerID || c.offer.target.model == "" {
		return nil
	}
	return c.box
}

func (c *aiSpeedControl) tryAgain() {
	if c.closed || c.retry == nil || (c.active != nil && !c.active()) {
		return
	}
	c.close()
	c.retry(aiSpeedChoice{})
}

// The message and its actions share the same validated switch offer.
func aiErrorRecovery(p aiErrorPresentation, control *aiSpeedControl) (string, *fyne.Container) {
	items := []fyne.CanvasObject{container.NewCenter(widget.NewButton("Try again", control.tryAgain))}
	if link := aiBillingLink(p); link != nil {
		items = append(items, container.NewCenter(link))
	}
	faster := control.timeoutSwitch(p)
	if faster != nil {
		items = append(items, faster)
	}
	return p.shown(faster != nil), container.NewVBox(items...)
}

func aiSearchErrorView(state *AppState, err error, retry func(aiSpeedChoice)) fyne.CanvasObject {
	if state.findSpeed != nil {
		state.findSpeed.close()
	}
	control := makeAISpeedControl(state, retry, true)
	state.findSpeed = control
	gen, query := state.askSession.gen, state.aiSearchQuery
	control.active = func() bool {
		return state.findSpeed == control && state.askSession.Current(gen) && state.aiSearchActive &&
			state.aiSearchErr != nil && !state.aiSearchLoading && state.aiSearchQuery == query
	}
	p, special := aiSpecialError(err)
	if !special {
		return aiSearchMessageView(friendlyAIError(err), "Try again", control.tryAgain)
	}
	message, actions := aiErrorRecovery(p, control)
	lbl := widget.NewLabel(message)
	lbl.Wrapping = fyne.TextWrapWord
	lbl.Alignment = fyne.TextAlignCenter
	items := []fyne.CanvasObject{wrappedParagraph(lbl, 300)}
	if p.detail != "" {
		items = append(items, aiErrorDetail(p))
	}
	items = append(items, spacer(12), actions)
	return findWaitScroll(container.NewVBox(items...))
}
