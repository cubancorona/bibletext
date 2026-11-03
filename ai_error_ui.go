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

func aiErrorFasterModel(state *AppState, p aiErrorPresentation, retry func()) fyne.CanvasObject {
	if state == nil || !p.timeout || p.providerID == "" || activeAIProvider(state).ID != p.providerID {
		return nil
	}
	if id, model, label, ok := fasterModelOffer(state); ok {
		return fasterModelControl(label, func() {
			// A stale error view must not switch one provider and retry another.
			if activeAIProvider(state).ID != id {
				return
			}
			applyFasterModel(state, id, model)
			retry()
		})
	}
	return nil
}

// aiErrorRecovery is what a screen shows for p: its message, and under it
// its actions, one to a row, Try again and then the billing link or the
// faster model. Both come from one reading of the faster-model offer, so a
// timeout's message names the faster model exactly when the action is
// there.
func aiErrorRecovery(state *AppState, p aiErrorPresentation, retry func()) (string, *fyne.Container) {
	items := []fyne.CanvasObject{container.NewCenter(widget.NewButton("Try again", retry))}
	if link := aiBillingLink(p); link != nil {
		items = append(items, container.NewCenter(link))
	}
	faster := aiErrorFasterModel(state, p, retry)
	if faster != nil {
		items = append(items, faster)
	}
	return p.shown(faster != nil), container.NewVBox(items...)
}

func aiErrorActions(state *AppState, p aiErrorPresentation, retry func()) *fyne.Container {
	_, actions := aiErrorRecovery(state, p, retry)
	return actions
}

func aiSearchErrorView(state *AppState, err error, retry func()) fyne.CanvasObject {
	p, special := aiSpecialError(err)
	if !special {
		return aiSearchMessageView(friendlyAIError(err), "Try again", retry)
	}
	message, actions := aiErrorRecovery(state, p, retry)
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
