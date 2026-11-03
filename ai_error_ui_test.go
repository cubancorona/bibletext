package bibletext

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func syntheticAIAccountError() error {
	return aiResponseFailure(429, []byte(`{"error":{"code":"insufficient_credits","message":"Your credit balance is too low. synthetic-private-tail"}}`))
}

func aiErrorOutbound(root fyne.CanvasObject, label string) *outboundLink {
	var found *outboundLink
	walkTree(root, func(object fyne.CanvasObject) {
		if link, ok := object.(*outboundLink); ok && link.Text == label {
			found = link
		}
	})
	return found
}

func TestAIErrorStudyRetryKeepsCapturedRequest(t *testing.T) {
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionCompare, aiActionAsk} {
		for _, timeout := range []bool{false, true} {
			t.Run(action+map[bool]string{false: "/account", true: "/timeout"}[timeout], func(t *testing.T) {
				app := test.NewApp()
				t.Cleanup(app.Quit)
				w := test.NewWindow(widget.NewLabel("reading"))
				t.Cleanup(w.Close)
				w.Resize(fyne.NewSize(720, 700))
				state := studyPanelState(t, w)
				requests := stubCapturedStudyRequests(t)
				passage, question := syntheticAskPassage(), "What does this synthetic selection mean?"
				showAIPanelForPassage(state, action, passage, question, selSpan{})
				first := nextCapturedStudyRequest(t, requests, "initial request")
				failure := syntheticAIAccountError()
				if timeout {
					failure = syntheticAITimeout()
				}
				finishCapturedStudyRequest(t, first, "", failure)
				popup := topPopup(t, w)
				info := activeAIProvider(state)
				// The faster model is offered here, so a timeout names it.
				p, _ := aiSpecialError(withAIErrorProvider(failure, info))
				if !treeHasText(popup, p.shown(true)) {
					t.Fatal("the study error lost its provider-specific explanation")
				}
				for _, label := range []string{"Copy", "Report"} {
					if b := findTreeButton(popup, label); b == nil || !b.Disabled() {
						t.Fatal("an error left answer actions enabled")
					}
				}
				if timeout {
					faster := findTreeLink(popup, "Switch to a faster model")
					if faster == nil {
						t.Fatal("a timeout lost the faster-model action")
					}
					faster.OnTapped()
					if state.keys().overrideModel(info.ID) != info.FastModel {
						t.Fatal("the timeout action did not select the provider's faster model")
					}
				} else {
					link := aiErrorOutbound(popup, info.ShortName+" billing")
					if link == nil || !treeHasText(popup, "Google said: “Your credit balance is too low”") {
						t.Fatal("the account error lost its safe reason or billing link")
					}
					openedInTests()
					link.OnTapped()
					_, billingURL := aiBillingDestination(info)
					if opened := openedInTests(); len(opened) != 1 || opened[0] != billingURL {
						t.Fatal("billing opened an untrusted provider destination")
					}
					findTreeButton(popup, "Try again").OnTapped()
				}
				second := nextCapturedStudyRequest(t, requests, "error retry")
				if second.action != first.action || second.passage != passage || second.question != question {
					t.Fatal("the error action changed the captured passage, question or action")
				}
				finishCapturedStudyRequest(t, second, "A synthetic answer paragraph.", nil)
				if b := findTreeButton(popup, "Copy"); b == nil || b.Disabled() {
					t.Fatal("a successful retry did not restore answer controls")
				}
			})
		}
	}
}

func TestAIHTTP408StudyKeepsGenericRetryWithoutFasterModel(t *testing.T) {
	for _, action := range []string{aiActionCompare, aiActionAsk} {
		t.Run(action, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(720, 700))
			state := studyPanelState(t, w)
			requests := stubCapturedStudyRequests(t)
			passage, question := syntheticAskPassage(), "What does this synthetic selection mean?"
			showAIPanelForPassage(state, action, passage, question, selSpan{})
			first := nextCapturedStudyRequest(t, requests, "initial request")
			finishCapturedStudyRequest(t, first, "", &apiHTTPError{StatusCode: http.StatusRequestTimeout})
			popup := topPopup(t, w)
			const want = "The AI service returned an error. Please try again shortly."
			if !treeHasText(popup, want) || findTreeLink(popup, "Switch to a faster model") != nil {
				t.Fatal("HTTP 408 did not keep generic guidance without timeout recovery")
			}
			retry := findTreeButton(popup, "Try again")
			if retry == nil {
				t.Fatal("HTTP 408 lost ordinary Try again")
			}
			retry.OnTapped()
			second := nextCapturedStudyRequest(t, requests, "generic error retry")
			if second.action != first.action || second.passage != passage || second.question != question {
				t.Fatal("generic retry lost the captured request")
			}
			finishCapturedStudyRequest(t, second, "A synthetic answer paragraph.", nil)
		})
	}
}

// The panel names the provider that was active when the request was sent,
// not the one active when its failure lands.
func TestAIErrorStudyNamesTheProviderOfTheRequest(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(720, 700))
	state := studyPanelState(t, w)
	requests := stubCapturedStudyRequests(t)
	before := activeAIProvider(state)
	showAIPanelForPassage(state, aiActionAsk, syntheticAskPassage(), "What does this synthetic selection mean?", selSpan{})
	first := nextCapturedStudyRequest(t, requests, "initial request")
	state.keys().setActiveProvider(providerAnthropic)
	after := activeAIProvider(state)
	if before.ID == after.ID {
		t.Fatal("control: the provider change changed nothing")
	}
	finishCapturedStudyRequest(t, first, "", syntheticAITimeout())
	popup := topPopup(t, w)
	if !treeHasText(popup, friendlyAIError(withAIErrorProvider(syntheticAITimeout(), before))) ||
		treeHasText(popup, friendlyAIError(withAIErrorProvider(syntheticAITimeout(), after))) {
		t.Fatal("a provider change while the request ran renamed its failure")
	}
}

// A billing error that is not an exhausted balance keeps the billing link
// and Try again, names the provider, and does not assert exhausted credit.
func TestAIErrorBillingKindOffersTheBillingLink(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	for _, info := range aiProviders() {
		err := withAIErrorProvider(aiResponseFailure(402, []byte(`{"error":{"type":"billing_error","message":"Payment information needs attention. synthetic-private-tail"}}`)), info)
		account, _ := aiSpecialError(withAIErrorProvider(aiResponseFailure(429, []byte(`{"error":{"code":"credit_balance_exhausted"}}`)), info))
		_, address := aiBillingDestination(info)
		p, ok := aiSpecialError(err)
		if !ok || p.timeout || p.billingURL != address || p.detail != "" || p.message == account.message ||
			!strings.HasPrefix(p.message, info.ShortName) || strings.Contains(p.message, "busy") ||
			strings.Contains(p.message, "run out of credit") || strings.Contains(p.message, "synthetic-") {
			t.Fatalf("%s billing error presentation: %#v", info.ID, p)
		}
		view := aiSearchErrorView(state, err, func() {})
		if aiErrorOutbound(view, info.ShortName+" billing") == nil || findTreeButton(view, "Try again") == nil || treeHasText(view, account.message) {
			t.Fatalf("%s billing error view lost its billing link or Try again, or asserted exhausted credit", info.ID)
		}
	}
}

func TestAIErrorFasterActionNeverSwitchesAnotherProvider(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	info, _ := providerByID(defaultProviderID)
	p, _ := aiSpecialError(withAIErrorProvider(syntheticAITimeout(), info))
	retries := 0
	view := aiErrorActions(state, p, func() { retries++ })
	faster := findTreeLink(view, "Switch to a faster model")
	if faster == nil {
		t.Fatal("control: the active provider has no faster-model offer")
	}
	state.keys().setActiveProvider(providerAnthropic)
	faster.OnTapped()
	if retries != 0 || state.keys().overrideModel(info.ID) != "" || state.keys().overrideModel(providerAnthropic) != "" {
		t.Fatal("a stale timeout view switched a different account or sent another request")
	}
	if findTreeLink(aiErrorActions(state, p, func() { retries++ }), "Switch to a faster model") != nil {
		t.Fatal("a non-active provider's error offered the active provider's faster model")
	}
	state.keys().setActiveProvider(info.ID)
	state.keys().setOverrideModel(info.ID, info.FastModel)
	if findTreeLink(aiErrorActions(state, p, func() { retries++ }), "Switch to a faster model") != nil {
		t.Fatal("a provider already on its faster model was offered it again")
	}
}

func TestAIErrorFindActionsAndProviderCapture(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.keys().setActiveProvider(providerAnthropic)
	state.keys().setAPIKey(providerAnthropic, "synthetic-key")
	grantAIPermission(state.keys(), providerAnthropic)
	previousStudy, previousSearch := aiStudyGenerate, aiSearchGenerate
	t.Cleanup(func() { aiStudyGenerate, aiSearchGenerate = previousStudy, previousSearch })
	response := func(context.Context, providerInfo, *keyStore, string, string) (string, error) {
		state.keys().setActiveProvider(defaultProviderID)
		return "", syntheticAITimeout()
	}
	aiStudyGenerate, aiSearchGenerate = response, response
	_, studyErr := runAIActionForPassage(context.Background(), state, aiActionAsk, syntheticAskPassage(), "Synthetic provider capture question")
	state.keys().setActiveProvider(providerAnthropic)
	_, searchErr := runAISearch(context.Background(), state, "Synthetic provider capture search")
	for _, err := range []error{studyErr, searchErr} {
		if !strings.HasPrefix(friendlyAIError(err), "Claude took too long") {
			t.Fatal("a provider change renamed an in-flight request's failure")
		}
	}
	retries := 0
	view := aiSearchErrorView(state, searchErr, func() { retries++ })
	if findTreeLink(view, "Switch to a faster model") != nil {
		t.Fatal("Find offered the wrong provider's faster model")
	}
	findTreeButton(view, "Try again").OnTapped()
	if retries != 1 {
		t.Fatal("Find lost its existing retry callback")
	}
	info := activeAIProvider(state)
	account := withAIErrorProvider(syntheticAIAccountError(), info)
	state.aiSearchActive, state.aiSearchErr = true, account
	state.keys().setAPIKey(info.ID, "synthetic-key")
	state.retryAISearch = func() { retries++ }
	desktop := buildSearchResultsView(state)
	if aiErrorOutbound(desktop, info.ShortName+" billing") == nil {
		t.Fatal("the desktop Find error lost its billing action")
	}
	findTreeButton(desktop, "Try again").OnTapped()
	if retries != 2 {
		t.Fatal("the desktop Find error did not retry the current search")
	}
}

func TestAIErrorKeyTestKeepsFailureProviderAndClearsOldActions(t *testing.T) {
	calls, listed := holdKeyTests(t)
	h := newAppearanceHarness(t, false)
	state := h.state
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.keys().setAPIKey(defaultProviderID, "synthetic-key")
	showAISettings(state)
	waitParked(t, listed, "model list")
	popup := settingsPopup(t, state)
	keyTestButtons(popup)[0].OnTapped()
	var first keyTestCall
	select {
	case first = <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("key test did not start")
	}
	state.keys().setActiveProvider(providerAnthropic)
	painted := paintsOf(state, keyTestAI)
	first.release <- syntheticAITimeout()
	awaitPaint(t, painted, "timeout")
	<-first.ctx.Done()
	if !strings.Contains(keyTestResult(popup), "Gemini took too long") || findTreeLink(popup, "Switch to a faster model") != nil {
		t.Fatal("the non-active provider's key test offered a switch for another account")
	}
	state.keys().setActiveProvider(defaultProviderID)
	keyTestButtons(popup)[0].OnTapped()
	second := <-calls
	// Discard the begin paint before waiting for the result paint.
	awaitPaint(t, painted, "retry wait")
	second.release <- syntheticAIAccountError()
	awaitPaint(t, painted, "account error")
	<-second.ctx.Done()
	if aiErrorOutbound(popup, "Gemini billing") == nil || !strings.Contains(keyTestResult(popup), "spending limit") {
		t.Fatal("Test key lost the account explanation or billing action")
	}
	keyTestButtons(popup)[0].OnTapped()
	third := <-calls
	awaitPaint(t, painted, "second retry wait")
	if link := aiErrorOutbound(popup, "Gemini billing"); link != nil && studyObjectVisible(popup, link) {
		t.Fatal("a retry kept the old billing action visible")
	}
	third.release <- nil
	awaitPaint(t, painted, "success")
	<-third.ctx.Done()
	if !strings.HasPrefix(keyTestResult(popup), "✓ Key works") {
		t.Fatal("a successful key retry did not replace the failure")
	}
}

// settingsPickerList is the model picker's list, once the Model row has
// opened it over the sheet.
func settingsPickerList(t *testing.T, state *AppState) *widget.List {
	t.Helper()
	var list *widget.List
	walkTree(state.window.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.List); ok && list == nil {
			list = l
		}
	})
	if list == nil {
		t.Fatal("the Model row did not open its picker")
	}
	return list
}

// aiTimeoutOffer is a state a provider's timeout can land in, as the
// faster-model offer reads it: prepare runs once the request is under way.
type aiTimeoutOffer struct {
	name    string
	before  func(st *AppState)
	prepare func(st *AppState)
	faster  bool
}

func aiTimeoutOffers() []aiTimeoutOffer {
	gemini, _ := providerByID(providerGemini)
	return []aiTimeoutOffer{
		{name: "offered", before: func(*AppState) {}, prepare: func(*AppState) {}, faster: true},
		{name: "already on the faster model", before: func(st *AppState) {
			st.keys().setOverrideModel(gemini.ID, gemini.FastModel)
		}, prepare: func(*AppState) {}},
		{name: "another provider active", before: func(*AppState) {}, prepare: func(st *AppState) {
			st.keys().setAPIKey(providerAnthropic, "synthetic-key")
			grantAIPermission(st.keys(), providerAnthropic)
			st.keys().setActiveProvider(providerAnthropic)
		}},
	}
}

// The timeout line names the faster model exactly where the screen offers
// it, and ends at Try again where it does not: on the provider's faster
// model already, or with another provider active since the request was
// sent. The same on the study panel's fixed verb, Ask and Compare, on both
// Find layouts and under Settings' Test key. The code before named the
// faster model on every screen, with the action there or not.
func TestTheTimeoutLineNamesTheFasterModelOnlyWhereItIsOffered(t *testing.T) {
	const withFaster = "Gemini took too long to answer, so the request was stopped. Try again, or switch to a faster model."
	const without = "Gemini took too long to answer, so the request was stopped. Try again."
	gemini, _ := providerByID(providerGemini)
	if defaultProviderID != gemini.ID {
		t.Fatal("control: the default provider is not Gemini")
	}
	want := func(t *testing.T, root fyne.CanvasObject, o aiTimeoutOffer, prefix string) {
		t.Helper()
		line, other := withFaster, without
		if !o.faster {
			line, other = without, withFaster
		}
		if !treeHasText(root, prefix+line) || treeHasText(root, prefix+other) {
			t.Errorf("%s: the timeout reads %v, want %q", o.name, treeTexts(root), prefix+line)
		}
		if got := findTreeLink(root, "Switch to a faster model") != nil; got != o.faster {
			t.Errorf("%s: the faster-model action shown %v, want %v", o.name, got, o.faster)
		}
	}
	for _, o := range aiTimeoutOffers() {
		for _, form := range []struct{ action, question string }{
			{aiActionExplain, ""}, {aiActionAsk, "What does this synthetic selection mean?"}, {aiActionCompare, ""},
		} {
			t.Run("study panel/"+form.action+"/"+o.name, func(t *testing.T) {
				app := test.NewApp()
				t.Cleanup(app.Quit)
				w := test.NewWindow(widget.NewLabel("reading"))
				t.Cleanup(w.Close)
				w.Resize(fyne.NewSize(720, 700))
				state := studyPanelState(t, w)
				requests := stubCapturedStudyRequests(t)
				o.before(state)
				showAIPanelForPassage(state, form.action, syntheticAskPassage(), form.question, selSpan{})
				request := nextCapturedStudyRequest(t, requests, "the request")
				o.prepare(state)
				finishCapturedStudyRequest(t, request, "", syntheticAITimeout())
				want(t, topPopup(t, w), o, "")
			})
		}
		findState := func(t *testing.T) *AppState {
			t.Helper()
			state := sampleState()
			state.aiKeys = newKeyStoreWith(newFakePrefs())
			state.keys().setAPIKey(gemini.ID, "synthetic-key")
			grantAIPermission(state.keys(), gemini.ID)
			o.before(state)
			o.prepare(state)
			state.aiSearchActive, state.aiSearchMode = true, true
			state.aiSearchQuery = "Synthetic Find question"
			state.aiSearchErr = withAIErrorProvider(syntheticAITimeout(), gemini)
			return state
		}
		t.Run("desktop Find/"+o.name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			want(t, buildSearchResultsView(findState(t)), o, "")
		})
		t.Run("phone Find/"+o.name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			w := test.NewWindow(widget.NewLabel("reading"))
			t.Cleanup(w.Close)
			state := findState(t)
			state.window = w
			want(t, buildMobileSearchTab(state, func() {}), o, "")
		})
		t.Run("Test key/"+o.name, func(t *testing.T) {
			calls, listed := holdKeyTests(t)
			h := newAppearanceHarness(t, false)
			state := h.state
			state.aiKeys = newKeyStoreWith(newFakePrefs())
			state.keys().setAPIKey(gemini.ID, "synthetic-key")
			o.before(state)
			showAISettings(state)
			waitParked(t, listed, "model list")
			popup := settingsPopup(t, state)
			keyTestButtons(popup)[0].OnTapped()
			var call keyTestCall
			select {
			case call = <-calls:
			case <-time.After(2 * time.Second):
				t.Fatal("key test did not start")
			}
			o.prepare(state)
			painted := paintsOf(state, keyTestAI)
			call.release <- syntheticAITimeout()
			awaitPaint(t, painted, "timeout")
			<-call.ctx.Done()
			want(t, popup, o, "✗ ")
		})
	}
	// With no provider captured the fallback names no one and offers no
	// faster model.
	if got := friendlyAIError(syntheticAITimeout()); got != "The AI service took too long to answer, so the request was stopped. Try again." {
		t.Errorf("the fallback timeout reads %q", got)
	}
}

func TestAIErrorKeyTestFasterActionUpdatesTheModelRow(t *testing.T) {
	calls, listed := holdKeyTests(t)
	h := newAppearanceHarness(t, false)
	state := h.state
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.keys().setAPIKey(defaultProviderID, "synthetic-key")
	info, _ := providerByID(defaultProviderID)
	showAISettings(state)
	waitParked(t, listed, "model list")
	popup := settingsPopup(t, state)
	model := findTreeButton(popup, "Recommended")
	if model == nil {
		t.Fatal("control: the Model row does not read Recommended before any test")
	}
	keyTestButtons(popup)[0].OnTapped()
	var first keyTestCall
	select {
	case first = <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("key test did not start")
	}
	painted := paintsOf(state, keyTestAI)
	first.release <- syntheticAITimeout()
	awaitPaint(t, painted, "timeout")
	<-first.ctx.Done()
	faster := findTreeLink(popup, "Switch to a faster model")
	if faster == nil {
		t.Fatal("control: the active provider's timeout offered no faster-model action")
	}
	faster.OnTapped()
	var second keyTestCall
	select {
	case second = <-calls:
	case <-time.After(2 * time.Second):
		t.Fatal("the faster-model action did not test the key again")
	}
	awaitPaint(t, painted, "retest wait")
	if state.keys().overrideModel(info.ID) != info.FastModel {
		t.Fatal("control: the faster-model action did not pin the provider's faster model")
	}
	if model.Text != info.FastModel {
		t.Fatalf("the Model row reads %q while the store pins %q", model.Text, info.FastModel)
	}
	model.OnTapped()
	list := settingsPickerList(t, state)
	if list.Length() != 2 {
		t.Fatalf("the picker offers %d choices, want Recommended and the pinned model", list.Length())
	}
	state.window.Canvas().Overlays().Top().Hide()
	second.release <- nil
	awaitPaint(t, painted, "success")
	<-second.ctx.Done()
	if model.Text != info.FastModel {
		t.Fatalf("the verdict reset the Model row to %q", model.Text)
	}
}

// aiPanelHeader is the study panel's header: its title, the passage and the
// rule under them, one to a row.
func aiPanelHeader(t *testing.T, popup *widget.PopUp, action string) *fyne.Container {
	t.Helper()
	var header *fyne.Container
	walkTree(popup, func(o fyne.CanvasObject) {
		if c, ok := o.(*fyne.Container); ok && len(c.Objects) > 0 {
			if title, ok := c.Objects[0].(*canvas.Text); ok && title.Text == aiActionTitle(action) {
				header = c
			}
		}
	})
	if header == nil {
		t.Fatalf("control: the panel has no header titled %q", aiActionTitle(action))
	}
	return header
}

// aiPanelTitle is the study panel's title.
func aiPanelTitle(t *testing.T, popup *widget.PopUp, action string) fyne.CanvasObject {
	t.Helper()
	return aiPanelHeader(t, popup, action).Objects[0]
}

// aiPanelHeaderGaps is the space between the rows of the study panel's
// header as laid out.
func aiPanelHeaderGaps(t *testing.T, popup *widget.PopUp, action string) []float32 {
	t.Helper()
	header := aiPanelHeader(t, popup, action)
	if len(header.Objects) == 1 {
		// The exceptional layout fixes only the title. Its remaining rows
		// retain the same vertical gaps in the combined scrolling column.
		if scroll := errorRecoveryScroll(popup); scroll != nil {
			if wrapper, ok := scroll.Content.(*fyne.Container); ok && len(wrapper.Objects) == 1 {
				if column, ok := wrapper.Objects[0].(*fyne.Container); ok {
					header = column
				}
			}
		}
	}
	var gaps []float32
	var above fyne.CanvasObject
	for _, row := range header.Objects {
		if !row.Visible() {
			continue
		}
		if above != nil {
			gaps = append(gaps, row.Position().Y-above.Position().Y-above.Size().Height)
		}
		above = row
	}
	if len(gaps) == 0 {
		t.Fatal("control: the panel's header shows a single row")
	}
	return gaps
}

// aiErrorSecondAction is what an account-limit or timeout error offers under
// Try again: the provider's billing link, or the faster model.
func aiErrorSecondAction(t *testing.T, popup *widget.PopUp, info providerInfo, timeout bool) fyne.CanvasObject {
	t.Helper()
	if timeout {
		if link := findTreeLink(popup, "Switch to a faster model"); link != nil {
			return link
		}
	} else if link := aiErrorOutbound(popup, info.ShortName+" billing"); link != nil {
		return link
	}
	t.Fatalf("the error lost its second action; texts %v", treeTexts(popup))
	return nil
}

// wantTheErrorSpacing fails unless the error keeps the panel's own spacing:
// the header's rows the theme's padding apart, as in every other state of
// the panel, and the second action on a row of its own under Try again.
func wantTheErrorSpacing(t *testing.T, popup *widget.PopUp, action string, try, second fyne.CanvasObject) {
	t.Helper()
	pad := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNamePadding)
	for _, gap := range aiPanelHeaderGaps(t, popup, action) {
		if gap < pad-0.01 || gap > pad+0.01 {
			t.Errorf("the header's rows stand %.2fpt apart, not the theme's %.2fpt padding", gap, pad)
		}
	}
	drv := fyne.CurrentApp().Driver()
	tryFoot := drv.AbsolutePositionForObject(try).Y + try.Size().Height
	if top := drv.AbsolutePositionForObject(second).Y; top < tryFoot-0.5 {
		t.Errorf("the second action starts at %.1f, inside Try again's row, which ends at %.1f", top, tryFoot)
	}
}

// aiSheetLeast is the least height popup's sheet can be laid out at: its
// content's minimum, its body at the height the panel last gave it, and the
// popup's padding round it.
func aiSheetLeast(popup *widget.PopUp) float32 {
	return popup.Content.MinSize().Height + popup.Theme().Size(theme.SizeNameInnerPadding)
}

// On short phones held sideways the account-limit and timeout errors keep
// the panel's spacing with a long question, with the passage closed and
// opened by hand, scrolled, and across a resize: the header's rows stay the
// theme's padding apart, and the billing link or the faster model keeps a
// row of its own under Try again. Where the room runs short it is the
// question and the message that scroll, with Try again, the second action
// and the footer in sight. Where that fixed content cannot leave a readable
// message line, context, message and recovery scroll together under the title
// and above Close. The code before shared one row between the two actions on the
// 800x360 canvas under its system bars, and on the one with its cutout for
// an account limit with the passage opened; under the bars with the passage
// opened it also drew the header's rows 3.57pt apart.
func TestAIErrorAccountAndTimeoutActionsFitShortPhones(t *testing.T) {
	for _, screen := range []insetScreen{
		{"667x375 synthetic phone", 667, 375, 0, 0, 0, 0},
		{"800x360 synthetic phone", 800, 360, 0, 0, 21, 0},
		{"800x360 with system bars", 800, 360, 24, 24, 0, 0},
	} {
		for _, providerID := range []string{defaultProviderID, providerOpenAI} {
			for _, timeout := range []bool{false, true} {
				t.Run(screen.name+"/"+providerID+map[bool]string{false: "/account", true: "/timeout"}[timeout], func(t *testing.T) {
					state, w := insetWindow(t, screen, 0)
					state.keys().setActiveProvider(providerID)
					state.keys().setAPIKey(providerID, "synthetic-key")
					grantAIPermission(state.keys(), providerID)
					requests := stubCapturedStudyRequests(t)
					failure := syntheticAIAccountError()
					if timeout {
						failure = syntheticAITimeout()
					}
					info := activeAIProvider(state)
					f := aiErrorForm{name: "synthetic account or timeout", action: aiActionAsk,
						question: strings.Repeat("What does this synthetic passage mean? ", 36),
						err:      withAIErrorProvider(failure, info), button: "Try again"}
					openAIError(t, requests, state, f, aiLongSelection)
					popup := topPopup(t, w)
					stage := "collapsed"
					check := func() {
						t.Helper()
						scroll, _, button := aiErrorIn(t, popup, f)
						second := aiErrorSecondAction(t, popup, info, timeout)
						wantTheErrorSpacing(t, popup, f.action, button, second)
						if errorRecoveryScroll(popup) != nil {
							wantInsideTheSafeArea(t, screen, popup)
							wantAIErrorOverflowReachable(t, popup)
							return
						}
						top, bottom := sheetBox(t, popup)
						drv := fyne.CurrentApp().Driver()

						wantInsideTheSafeArea(t, screen, popup)
						for _, cut := range aiErrorActionCuts(t, popup, f, scroll, button) {
							t.Error(stage + ": " + cut)
						}
						y := drv.AbsolutePositionForObject(second).Y
						footer := drv.AbsolutePositionForObject(findTreeButton(popup, "Close")).Y
						if y < top || y+second.Size().Height > bottom || y+second.Size().Height > footer ||
							second.Size().Height < second.MinSize().Height || button.Size().Height < button.MinSize().Height {
							t.Errorf("%s: an action was clipped, shrunk or overlapped the footer", stage)
						}
					}
					check()
					test.Tap(quoteDisclosure(t, popup))
					stage = "expanded"
					check()
					if quoteDisclosure(t, popup).Icon != iconDisclosureCollapse {
						t.Fatal("the explicit expanded passage choice was lost")
					}
					scroll, message, _ := aiErrorIn(t, popup, f)
					scroll.ScrollToBottom()
					if scroll.Offset.Y <= 0 {
						t.Fatal("the long question and error cannot be scrolled")
					}
					// Scrolled to its end, the column's last line is in sight:
					// the message's, or the account limit's quoted reason.
					var last fyne.CanvasObject = message
					if p, _ := aiSpecialError(f.err); p.detail != "" {
						if last = aiErrorRichText(popup, p.detail); last == nil {
							t.Fatal("the account limit lost its quoted reason")
						}
					}
					lastFoot := fyne.CurrentApp().Driver().AbsolutePositionForObject(last).Y + last.Size().Height
					scrollFoot := fyne.CurrentApp().Driver().AbsolutePositionForObject(scroll).Y + scroll.Size().Height
					if lastFoot > scrollFoot+0.5 {
						t.Fatal("the error's final line is not reachable")
					}
					var quote *container.Scroll
					walkTree(popup, func(o fyne.CanvasObject) {
						if other, ok := o.(*container.Scroll); ok && other != scroll {
							quote = other
						}
					})
					if quote == nil || quote.Size().Height < 32 {
						t.Fatal("the expanded passage lost its own readable scroll")
					}
					quote.ScrollToBottom()
					if quote.Offset.Y <= 0 {
						t.Fatal("the expanded passage cannot be scrolled to its end")
					}
					check()
					w.Resize(fyne.NewSize(screen.w, screen.h+300))
					refitSheets(state)
					sheetBox(t, popup)
					wantTheErrorSpacing(t, popup, f.action, findTreeButton(popup, "Try again"), aiErrorSecondAction(t, popup, info, timeout))
					w.Resize(fyne.NewSize(screen.w, screen.h))
					refitSheets(state)
					stage = "resized back"
					check()
					if quoteDisclosure(t, popup).Icon != iconDisclosureCollapse {
						t.Fatal("a resize lost the explicit expanded passage choice")
					}
				})
			}
		}
	}
}

// aiErrorRichText is the rich text under root that reads text: the account
// limit's quoted reason.
func aiErrorRichText(root fyne.CanvasObject, text string) *widget.RichText {
	var found *widget.RichText
	walkTree(root, func(o fyne.CanvasObject) {
		if r, ok := o.(*widget.RichText); ok && segmentText(r.Segments) == text {
			found = r
		}
	})
	return found
}

// syntheticAIAccountLimit is an exhausted account as each provider reports
// one, in its own shape and with a clause the classification quotes.
func syntheticAIAccountLimit(info providerInfo) error {
	switch info.ID {
	case providerOpenAI:
		return aiResponseFailure(429, []byte(`{"error":{"message":"You exceeded your current quota, please check your plan and billing details. synthetic-private-tail","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`))
	case providerAnthropic:
		return aiResponseFailure(400, []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the synthetic API. synthetic-private-tail"}}`))
	case providerGrok:
		return aiResponseFailure(403, []byte(`{"code":"insufficient_credits","error":"Your team has exhausted its credits. synthetic-private-tail"}`))
	default:
		return aiResponseFailure(402, []byte(`{"error":{"code":402,"message":"Your Prepay credit balance is depleted. synthetic-private-tail","status":"RESOURCE_EXHAUSTED"}}`))
	}
}

// aiRecoveryPhones are the phone canvases the error tests lay out:
// TestTheAIErrorKeepsItsRuleOnPhones's, TestTheAIErrorActionStaysVisibleOnLandscapePhones's
// and TestAIErrorAccountAndTimeoutActionsFitShortPhones's.
var aiRecoveryPhones = []insetScreen{
	proMaxSideways,
	{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
	{"iPhone SE (1st generation), landscape", 568, 320, 0, 0, 0, 0},
	{"Android phone, landscape, cutout on the left", 803, 360, 0, 0, 21, 0},
	{"640x320 under a 24pt status bar", 640, 320, 24, 0, 0, 0},
	{"732x240 split screen", 732, 240, 0, 0, 0, 0},
	{"Android QA phone, landscape", 914, 411, 24, 24, 0, 0},
	{"800x360 phone, landscape", 800, 360, 0, 0, 21, 0},
	{"800x360 with system bars", 800, 360, 24, 24, 0, 0},
	{"667x375 phone, landscape", 667, 375, 0, 0, 0, 0},
}

// aiRecoveryCutOn is, for each canvas in aiRecoveryPhones, the account-limit
// and timeout errors whose column, the message and, under an account limit,
// the line quoting the provider, is taller than the message viewport the
// most a sheet may be leaves there, for every provider. The column scrolls;
// Try again, the second action and the footer stay in sight. On the iPhone
// 17 Pro Max held sideways the message of an account limit on Explain and
// Compare is whole, but for Claude's, which runs to a fourth line, and the
// quoted line under it is what scrolls.
var aiRecoveryCutOn = map[string][]string{
	"iPhone 17 Pro Max, landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask"},
	"iPhone 16 Pro, landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Ask"},
	"iPhone SE (1st generation), landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Explain", "timeout, Compare", "timeout, Ask"},
	"Android phone, landscape, cutout on the left": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Explain", "timeout, Compare", "timeout, Ask"},
	"Android QA phone, landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Ask"},
	"800x360 phone, landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Explain", "timeout, Compare", "timeout, Ask"},
	"800x360 with system bars": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Explain", "timeout, Compare", "timeout, Ask"},
	"667x375 phone, landscape": {"account limit, Explain", "account limit, Compare", "account limit, Ask",
		"timeout, Ask"},
}

// The account-limit and timeout errors keep the error state's rule on every
// phone canvas the error tests lay out, held sideways: the panel's spacing
// stays the panel's (wantTheErrorSpacing), the panel stays inside the safe
// area, and the column is whole, or, where even the most a sheet may be is
// too short for it (aiRecoveryCutOn), the panel takes that most and the
// column scrolls to its last line, Try again, the second action and the
// footer in sight. If the fixed chrome cannot leave one readable message
// line, the combined overflow layout keeps title and Close fixed instead. Every
// provider's own account-limit body and its quoted clause are measured, on
// the fixed study verb, the comparison and Ask with a one-line question.
// The code before shared one row between the two actions on the iPhone SE
// (1st generation), the 640x320, the 732x240 and the barred 800x360
// canvases, and on the 640x320 and 732x240 ones drew the header's rows with
// no space between them.
func TestTheAIAccountAndTimeoutErrorsKeepTheErrorRuleOnPhones(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	for _, s := range aiRecoveryPhones {
		cut := map[string]bool{}
		for _, name := range aiRecoveryCutOn[s.name] {
			cut[name] = true
		}
		for _, info := range aiProviders() {
			for _, timeout := range []bool{false, true} {
				for _, form := range []struct{ name, action, question string }{
					{"Explain", aiActionExplain, ""}, {"Compare", aiActionCompare, ""}, {"Ask", aiActionAsk, aiShortQuestion},
				} {
					kind := map[bool]string{false: "account limit", true: "timeout"}[timeout]
					t.Run(fmt.Sprintf("%s/%s/%s, %s", s.name, info.ID, kind, form.name), func(t *testing.T) {
						st, w := insetWindow(t, s, 0)
						st.keys().setActiveProvider(info.ID)
						failure := syntheticAIAccountLimit(info)
						if timeout {
							failure = syntheticAITimeout()
						}
						f := aiErrorForm{name: kind + ", " + form.name, action: form.action, question: form.question,
							err: withAIErrorProvider(failure, info), button: "Try again"}
						openAIError(t, reqs, st, f, "For God so loved the world")
						popup := topPopup(t, w)
						defer popup.Hide()
						p, _ := aiSpecialError(f.err)
						sc, lbl, try := aiErrorIn(t, popup, f)
						second := aiErrorSecondAction(t, popup, info, timeout)
						wantTheErrorSpacing(t, popup, f.action, try, second)
						if errorRecoveryScroll(popup) != nil {
							wantInsideTheSafeArea(t, s, popup)
							wantAIErrorOverflowReachable(t, popup)
							return
						}
						drv := fyne.CurrentApp().Driver()
						top, bottom := sheetBox(t, popup)
						most := sheetMostHeight(st, w.Canvas())
						scTop := drv.AbsolutePositionForObject(sc).Y
						var column []fyne.CanvasObject = []fyne.CanvasObject{lbl}
						if p.detail != "" {
							said := aiErrorRichText(popup, p.detail)
							if said == nil {
								t.Fatal("the account limit lost its quoted reason")
							}
							column = append(column, said)
						}
						shown, ends := true, ""
						for _, part := range column {
							y := drv.AbsolutePositionForObject(part).Y
							if y < max(scTop, top)-0.5 || y+part.Size().Height > min(scTop+sc.Size().Height, bottom)+0.5 {
								shown = false
							}
							ends += fmt.Sprintf(" %.1f..%.1f", y, y+part.Size().Height)
						}
						tryY, secondY := drv.AbsolutePositionForObject(try).Y, drv.AbsolutePositionForObject(second).Y
						header := aiPanelHeader(t, popup, f.action)
						headerY := drv.AbsolutePositionForObject(header).Y
						closeBtn := findTreeButton(popup, "Close")
						closeY := drv.AbsolutePositionForObject(closeBtn).Y
						least := aiSheetLeast(popup)
						t.Logf("record: canvas %.0fx%.0f, safe %.0f..%.0f, most %.1f, least %.1f; sheet %.1f..%.1f; header %.1f..%.1f; viewport %.1f..%.1f; column%s; Try again %.1f..%.1f; second %.1f..%.1f; Close %.1f..%.1f",
							s.w, s.h, s.top, s.h-s.bottom, most, least, top, bottom, headerY, headerY+header.Size().Height,
							scTop, scTop+sc.Size().Height, ends, tryY, tryY+try.Size().Height, secondY, secondY+second.Size().Height,
							closeY, closeY+closeBtn.Size().Height)

						wantInsideTheSafeArea(t, s, popup)
						for _, c := range aiErrorActionCuts(t, popup, f, sc, try) {
							t.Error(c)
						}
						footer := drv.AbsolutePositionForObject(findTreeButton(popup, "Close")).Y
						if secondY < top-0.5 || secondY+second.Size().Height > footer+0.5 {
							t.Errorf("the second action spans %.1f..%.1f, outside the sheet from %.1f or over the footer at %.1f",
								secondY, secondY+second.Size().Height, top, footer)
						}
						if !cut[f.name] {
							if !shown {
								t.Errorf("the error's column is cut:%s, its viewport %.1f..%.1f", ends, scTop, scTop+sc.Size().Height)
							}
							return
						}
						if bottom-top < most-0.5 {
							t.Errorf("the panel is %.1fpt tall with its column cut, where a sheet may be %.1fpt", bottom-top, most)
						}
						if shown {
							t.Errorf("control: the column is whole here, where aiRecoveryCutOn records it cut")
						}
						sc.ScrollToBottom()
						last := column[len(column)-1]
						if foot := drv.AbsolutePositionForObject(last).Y + last.Size().Height; foot > scTop+sc.Size().Height+0.5 {
							t.Errorf("scrolled to its end, the column's last line ends at %.1f, below its viewport's foot %.1f", foot, scTop+sc.Size().Height)
						}
					})
				}
			}
		}
	}
}
