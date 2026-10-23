package bibletext

// Permission before the reader's text goes to an AI provider.
//
// App Store guideline 5.1.2(i) asks an app to say clearly where personal data
// goes, naming a third-party AI, and to have explicit permission before it is
// sent. Study with AI sends the passage the reader selects, the study action,
// any question they type and their Find search, from the device to the
// provider they chose, under their own key. The note under the key field in
// Settings says so; the permission is asked for here, once for each provider,
// before its first request, on every platform alike: the question is a sheet
// the toolkit draws, and every surface's study menu, Ask and Find reach it
// through the same Go (ai_permission_contract_test.go).
//
// Allow records the permission in the app's preferences, one record for each
// provider, tied to the key the provider had when the reader allowed it: the
// record holds a fingerprint of that key, never the key (keyStore.aiAllowed,
// aiAllowedRecord). The permission ends with its key. Any change of that
// provider's key, cleared, replaced by another or entered again where there
// was none, erases the record (keyStore.setAPIKey), and choosing "None" as
// the assistant erases every one (keyStore.setAIEnabled). Every check
// compares the record with the key the request would use, so a record is no
// permission beside another key or none, however the key changed: on Android
// a restore brings the preferences back without the key. The app then asks
// again before the next request.
//
// Two layers hold the rule. The study panel and both Find panes ask before
// they start a request (aiPermissionPending). The two places a request leaves
// the device, runAIActionForPassage and runAISearch, refuse one for a
// provider with no permission, whatever path reached them, and the refusal
// reads as the not-sent message below. Settings' Test key and its model list
// are not asked about: they send the key, and Test key a fixed probe, none of
// the reader's text.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The question's wording, with the provider's display name filled in
// (providerInfo.Name, "Claude (Anthropic)").
func aiPermissionTitle(info providerInfo) string {
	return "Send text to " + info.Name + "?"
}

func aiPermissionBody(info providerInfo) string {
	return "Study with AI sends the passage you select, the study action, and any question or search you type " +
		"straight from this device to " + info.Name + ", using your key. They may keep it under your " +
		"account terms. Nothing is sent until you allow it. To withdraw, choose None in Settings or " +
		"clear your key."
}

const (
	aiPermissionNotNow = "Not now"
	aiPermissionAllow  = "Allow"

	// aiPermissionNotSent is what the study panel and the Find pane say when
	// a request was not sent because the reader has not allowed the provider:
	// the question's own sentence, which is true of the moment it is shown.
	aiPermissionNotSent = "Nothing is sent until you allow it."
)

// aiPermissionError is a request stopped before it left the device: the
// reader has not allowed their text to go to this provider.
type aiPermissionError struct{ provider providerInfo }

func (e *aiPermissionError) Error() string {
	return "not sent: no permission recorded for " + e.provider.Name
}

// aiPermissionPending reports the provider a request would go to when the
// reader has not yet allowed it: AI is on, the active provider has a key, and
// no permission is recorded for it with that key. Without a key nothing can
// be sent, and the surfaces ask for a key instead, so there is nothing to ask.
func aiPermissionPending(state *AppState) (providerInfo, bool) {
	if state == nil || !aiFeaturesEnabled(state) {
		return providerInfo{}, false
	}
	store := state.keys()
	id := store.activeProvider()
	info, ok := providerByID(id)
	key := providerAPIKey(store, id)
	if !ok || strings.TrimSpace(key) == "" || store.aiAllowed(id, key) {
		return providerInfo{}, false
	}
	return info, true
}

// findAsksPermission is the Find panes' half of the rule, one for the
// desktop sidebar and the phone's Search tab alike. When the provider has no
// permission it gives up any search in flight, leaves the pane saying
// nothing was sent, with Try again (repaint draws it from state, so a rebuild
// draws the same), and asks; Allow runs the search the reader submitted,
// unless a newer one has replaced it (gen, the submission's askSession
// token). It reports whether it stopped the search to ask. The question
// registers no reopen: a light/dark rebuild or a rotation closes it, and the
// pane comes back from state with the search in its field, saying nothing
// was sent, with Try again; run belongs to the pane the rebuild replaced.
func findAsksPermission(state *AppState, gen int, q string, repaint func(), run func(string)) bool {
	info, pending := aiPermissionPending(state)
	if !pending {
		return false
	}
	abandonAISearch(state)
	state.aiSearchActive = true
	state.aiSearchQuery = q
	state.aiSearchResults = nil
	state.aiSearchErr = &aiPermissionError{provider: info}
	state.aiSearchCancelled = false
	if repaint != nil {
		repaint()
	}
	promptAIPermission(state, info, func() {
		if state.askSession.Current(gen) {
			run(q)
		}
	}, nil, nil)
	return true
}

// promptAIPermission asks whether the reader's text may go to info's
// provider. Allow records the permission, for the key the provider had when
// the question was asked, and runs allow; Not now records
// nothing and runs notNow. A question closed any other way records nothing
// and runs neither, so each caller shows the not-sent state before asking
// and leaves it standing until Allow. A light/dark rebuild or a rotation
// drains the question with the sheet or pane it was asked over; reopen,
// when given, is how they come back after the rebuild (sheet_reopen.go),
// and it runs only then, never on Allow or Not now. With no window to ask
// in, nothing is asked or recorded, and notNow runs.
func promptAIPermission(state *AppState, info providerInfo, allow, notNow, reopen func()) {
	declined := func() {
		if notNow != nil {
			notNow()
		}
	}
	if state == nil || state.window == nil {
		declined()
		return
	}
	cnv := pickerCanvas(state)
	if cnv == nil {
		declined()
		return
	}
	pal := state.pal()
	// The key the question is about. A key changed while it is open leaves
	// the permission with this one, so the request, made with the other,
	// asks again.
	key := providerAPIKey(state.keys(), info.ID)

	// The native reading view floats above the canvas; it is hidden while the
	// question is up, and shown again only when nothing else owns the canvas,
	// since the question usually sits on the study panel, which hid it first.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	var popup *widget.PopUp
	closed := false
	closeIt := func() {
		if closed {
			return
		}
		closed = true
		if popup != nil {
			popup.Hide()
		}
		if state.showReadingOverlay != nil && cnv.Overlays().Top() == nil {
			state.showReadingOverlay()
		}
	}

	// The title wraps: with the provider's name in it, it is wider than a
	// phone's card at the sheet title's size.
	title := widget.NewRichText(&widget.TextSegment{
		Text: aiPermissionTitle(info),
		Style: widget.RichTextStyle{
			SizeName:  theme.SizeNameSubHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	title.Wrapping = fyne.TextWrapWord
	body := widget.NewLabel(aiPermissionBody(info))
	body.Wrapping = fyne.TextWrapWord

	notNowBtn := widget.NewButton(aiPermissionNotNow, func() {
		closeIt()
		declined()
	})
	allowBtn := widget.NewButton(aiPermissionAllow, func() {
		state.keys().setAIAllowed(info.ID, key)
		closeIt()
		if allow != nil {
			allow()
		}
	})
	allowBtn.Importance = widget.HighImportance

	// The words scroll on a canvas too short for them; the buttons stay on it.
	text := container.NewVBox(title, body)
	textScroll := container.NewVScroll(text)
	buttons := container.NewBorder(nil, nil, nil, container.NewHBox(notNowBtn, allowBtn))
	form := container.NewBorder(nil, container.NewVBox(widget.NewSeparator(), buttons), nil, nil, textScroll)
	card := surface(container.NewPadded(form), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	popup.Show()

	// The notes questions' width, and as tall as the words need, up to the
	// canvas less a margin, kept clear of a desktop window's header.
	fit := func() {
		cs := cnv.Size()
		w := float32(420)
		if cw := cs.Width - 40; cw > 260 && w > cw {
			w = cw
		}
		maxH := clearOfHeader(cs.Height-40, cs.Height, headerClearance(state))
		h := scrollingSheetHeight(popup.MinSize().Height, textScroll.MinSize().Height,
			text.MinSize().Height, maxH)
		popup.Resize(fyne.NewSize(w, h))
	}
	// Twice: the wrapped title and body report their height at the card's
	// width only after a layout pass at that width, which the first runs.
	fit()
	fit()
	registerSheetRefit(state, popup, func() {
		fit()
		fit()
	})
	// Registered while the question is showing, so a rebuild's take finds it
	// on top; a question that has closed, by Allow, Not now or the drain,
	// answers for nothing (sheet_reopen.go). A nil reopen registers nothing.
	registerSheetReopen(state, popup, reopen)
}
