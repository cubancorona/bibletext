package bibletext

// Permission before the reader's text goes to an AI provider
// (ai_permission.go). The study actions, Ask and both Find panes send nothing
// to a provider until the reader allows it, ask once per provider, and ask
// again once the permission is withdrawn by a change of the key or by
// choosing None.
//
// Requests go to a stand-in provider on this machine, through the seams that
// sit below the permission check (aiStudyGenerate, aiSearchGenerate), so the
// check runs as the app runs it and what reaches the provider is counted
// where it arrives. A request's completion paints on its own goroutine (the
// test driver runs fyne.Do inline), so a test reads a panel only after the
// request's context has ended, which is after the completion (ai_span_test.go).

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The approved words, held here apart from the code that shows them.
func approvedPermissionTitle(provider string) string {
	return "Send text to " + provider + "?"
}

func approvedPermissionBody(provider string) string {
	return "Study with AI sends the passage you select, the study action, and any question or search you type straight from this device to " +
		provider + ", using your key. They may keep it under your account terms. Nothing is sent until you allow it. " +
		"To withdraw, choose None in Settings or clear your key."
}

const notSentMessage = "Nothing is sent until you allow it."

// providerStandIn answers as an OpenAI-shaped chat endpoint and keeps the
// body of every request that reaches it.
type providerStandIn struct {
	srv    *httptest.Server
	mu     sync.Mutex
	bodies []string
}

func newProviderStandIn(t *testing.T) *providerStandIn {
	t.Helper()
	s := &providerStandIn{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(b))
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"Synthetic answer."},"finish_reason":"stop"}]}`)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *providerStandIn) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.bodies...)
}

// route sends every study request and every Find to the stand-in, and hands
// each request's context to the channel once its reply is in. It empties the
// answer cache, so no request is answered without reaching the stand-in, and
// makes a landed answer fit at once rather than on a timer.
func (s *providerStandIn) route(t *testing.T) chan context.Context {
	t.Helper()
	aiCacheMu.Lock()
	aiCache = map[string]string{}
	aiCacheOrder = nil
	aiCacheMu.Unlock()
	prevFit := studyRemeasure
	studyRemeasure = func(fit func()) { fit() }
	sent := make(chan context.Context, 8)
	send := func(ctx context.Context, _ providerInfo, _ *keyStore, key, prompt string) (string, error) {
		out, err := newOpenAIClient(key, s.srv.URL, "synthetic-model").generate(ctx, prompt)
		sent <- ctx
		return out, err
	}
	prevStudy, prevFind := aiStudyGenerate, aiSearchGenerate
	aiStudyGenerate, aiSearchGenerate = send, send
	t.Cleanup(func() {
		aiStudyGenerate, aiSearchGenerate = prevStudy, prevFind
		studyRemeasure = prevFit
	})
	return sent
}

// nothingSent fails if any request reached the stand-in, after giving one
// that was started time to arrive.
func (s *providerStandIn) nothingSent(t *testing.T, when string) {
	t.Helper()
	time.Sleep(300 * time.Millisecond)
	if got := s.received(); len(got) != 0 {
		t.Fatalf("%s: %d request(s) reached the provider before the reader allowed it", when, len(got))
	}
}

// landed waits for the next request to reach the stand-in and for the
// completion it started to finish painting.
func landed(t *testing.T, sent chan context.Context, what string) {
	t.Helper()
	select {
	case ctx := <-sent:
		<-ctx.Done()
	case <-time.After(3 * time.Second):
		t.Fatalf("%s never reached the provider", what)
	}
}

// permissionQuestion is the question on top of the canvas, held to the
// approved words for the provider named.
func permissionQuestion(t *testing.T, cnv fyne.Canvas, provider string) *widget.PopUp {
	t.Helper()
	top := cnv.Overlays().Top()
	p, ok := top.(*widget.PopUp)
	if !ok || p == nil || !treeHasText(p, approvedPermissionTitle(provider)) {
		t.Fatalf("the question whether text may go to %s is not on top; texts %v", provider, treeTexts(top))
	}
	if !treeHasText(p, approvedPermissionBody(provider)) {
		t.Errorf("the question's body is not the approved one; texts %v", treeTexts(p))
	}
	for _, label := range []string{"Not now", "Allow"} {
		if findTreeButton(p, label) == nil {
			t.Errorf("the question has no %q button; texts %v", label, treeTexts(p))
		}
	}
	if got := len(treeTexts(p)); got != 4 {
		t.Errorf("the question shows %d texts, want the title, the body and two buttons: %v", got, treeTexts(p))
	}
	return p
}

// beneath is the overlay under the top one.
func beneath(t *testing.T, cnv fyne.Canvas) fyne.CanvasObject {
	t.Helper()
	list := cnv.Overlays().List()
	if len(list) < 2 {
		t.Fatalf("nothing under the top overlay; %d overlay(s)", len(list))
	}
	return list[len(list)-2]
}

// assertNotSentState holds a panel or pane to the state a request not sent
// leaves: the not-sent message, and Try again to ask again.
func assertNotSentState(t *testing.T, o fyne.CanvasObject, where string) *widget.Button {
	t.Helper()
	if !treeHasText(o, notSentMessage) {
		t.Errorf("%s does not say nothing was sent; texts %v", where, treeTexts(o))
	}
	retry := findTreeButton(o, "Try again")
	if retry == nil {
		t.Fatalf("%s offers no Try again; texts %v", where, treeTexts(o))
	}
	return retry
}

// claudeReaderState is the refrain chapter in a window, with Claude
// (Anthropic) chosen and its key saved, and no permission yet.
func claudeReaderState(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(900, 760))
	st := refrainChapterState()
	st.window = w
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setActiveProvider(providerAnthropic)
	st.aiKeys.setAPIKey(providerAnthropic, "test-key")
	return st, w
}

// A study action sends nothing until the reader allows the provider. The
// question names the provider in the approved words; the panel under it says
// nothing was sent; Not now records nothing and sends nothing; Try again asks
// again; Allow records the permission and sends the passage; the next action
// goes straight to the provider.
func TestNoStudyRequestLeavesBeforeTheReaderAllowsIt(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)
	st, w := claudeReaderState(t)
	cnv := w.Canvas()
	const claude = "Claude (Anthropic)"

	dispatchAIAction(st, aiActionExplain, refrain, verseTwo)
	q := permissionQuestion(t, cnv, claude)
	s.nothingSent(t, "with the question open")
	assertNotSentState(t, beneath(t, cnv), "the panel under the question")

	test.Tap(findTreeButton(q, "Not now"))
	s.nothingSent(t, "after Not now")
	if aiPermitted(st.aiKeys, providerAnthropic) {
		t.Fatal("Not now recorded a permission")
	}
	panel := cnv.Overlays().Top()
	if treeHasText(panel, approvedPermissionTitle(claude)) {
		t.Fatal("Not now left the question up")
	}
	retry := assertNotSentState(t, panel, "the panel after Not now")

	test.Tap(retry)
	q = permissionQuestion(t, cnv, claude)
	s.nothingSent(t, "with the question asked again")

	test.Tap(findTreeButton(q, "Allow"))
	landed(t, sent, "the request Allow sends")
	if !aiPermitted(st.aiKeys, providerAnthropic) {
		t.Error("Allow recorded no permission")
	}
	got := s.received()
	if len(got) != 1 || !strings.Contains(got[0], refrain) {
		t.Fatalf("Allow must send the passage once; the provider received %q", got)
	}
	panel = cnv.Overlays().Top()
	if !treeHasText(panel, "Synthetic answer.") || treeHasText(panel, notSentMessage) {
		t.Errorf("the panel does not show the answer; texts %v", treeTexts(panel))
	}
	test.Tap(findTreeButton(panel, "Close"))

	// A request held by the question would never land; the panel is read
	// only once the request's completion has painted it.
	dispatchAIAction(st, aiActionContext, refrain, verseTwo)
	landed(t, sent, "the second action, with no question for a provider already allowed")
	panel = cnv.Overlays().Top()
	if treeHasText(panel, approvedPermissionTitle(claude)) || !treeHasText(panel, "Synthetic answer.") {
		t.Errorf("the second action asked again or showed no answer; texts %v", treeTexts(panel))
	}
	if n := len(s.received()); n != 2 {
		t.Errorf("the provider received %d requests, want 2", n)
	}
	test.Tap(findTreeButton(panel, "Close"))
}

// Ask's question is typed first and sent only after Allow: Not now leaves the
// answer panel saying nothing was sent, and the question goes with the
// passage once the reader allows it.
func TestAskSendsTheQuestionOnlyAfterAllow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	holdNoteSheetTimers(t)
	s := newProviderStandIn(t)
	sent := s.route(t)
	st, w := claudeReaderState(t)
	cnv := w.Canvas()
	const claude = "Claude (Anthropic)"
	const question = "Who is thanked in this refrain?"

	dispatchAIAction(st, aiActionAsk, refrain, verseTwo)
	composer := topPopup(t, w)
	field := composerField(composer)
	if field == nil {
		t.Fatalf("the Ask sheet has no question field; texts %v", treeTexts(composer))
	}
	field.SetText(question)
	test.Tap(findTreeButton(composer, "Ask"))

	q := permissionQuestion(t, cnv, claude)
	s.nothingSent(t, "with the question typed and the permission asked")
	test.Tap(findTreeButton(q, "Not now"))
	s.nothingSent(t, "after Not now")
	retry := assertNotSentState(t, cnv.Overlays().Top(), "the answer panel after Not now")

	test.Tap(retry)
	test.Tap(findTreeButton(permissionQuestion(t, cnv, claude), "Allow"))
	landed(t, sent, "the question Allow sends")
	got := s.received()
	if len(got) != 1 || !strings.Contains(got[0], question) || !strings.Contains(got[0], refrain) {
		t.Fatalf("Allow must send the question with the passage once; the provider received %q", got)
	}
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))
}

// The desktop sidebar's Find sends nothing before Allow, and its pane says
// so, with Try again; Allow runs the search the reader submitted.
func TestTheDesktopFindSendsNothingBeforeAllow(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	w.Resize(fyne.NewSize(900, 760))
	st := sampleState()
	st.window = w
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	st.aiSearchMode = true
	st.aiSearchQuery = "mercy"
	buildSidebar(st)
	var pane fyne.CanvasObject
	st.showReading = func() { pane = buildSearchResultsView(st) }
	t.Cleanup(func() { st.showReading = nil })
	gemini, _ := providerByID(defaultProviderID)

	st.retryAISearch() // submits "mercy" through the sidebar's own path
	q := permissionQuestion(t, w.Canvas(), gemini.Name)
	s.nothingSent(t, "with the Find's question open")
	retry := assertNotSentState(t, pane, "the Find pane under the question")

	test.Tap(findTreeButton(q, "Not now"))
	s.nothingSent(t, "after Not now")
	if aiPermitted(st.aiKeys, defaultProviderID) {
		t.Fatal("Not now recorded a permission")
	}

	test.Tap(retry)
	test.Tap(findTreeButton(permissionQuestion(t, w.Canvas(), gemini.Name), "Allow"))
	landed(t, sent, "the Find Allow runs")
	got := s.received()
	if len(got) != 1 || !strings.Contains(got[0], "Request: mercy") {
		t.Fatalf("Allow must send the search once; the provider received %q", got)
	}
}

// The phone's Search tab asks the same question and draws the same not-sent
// state from state; Allow starts the Find the reader submitted, and nothing
// starts before it.
func TestThePhoneFindAsksBeforeItStarts(t *testing.T) {
	held := holdFinds(t)
	h := newAppearanceHarness(t, true)
	h.state.aiKeys = newKeyStoreWith(newFakePrefs())
	h.state.aiKeys.setActiveProvider(providerOpenAI)
	h.state.aiKeys.setAPIKey(providerOpenAI, "test-key")
	h.state.aiSearchMode = true
	h.state.CurrentTab = 2
	rebuildWindow(h.state)
	cnv := h.state.window.Canvas()
	const chatGPT = "ChatGPT (OpenAI)"

	h.ask("mercy")
	q := permissionQuestion(t, cnv, chatGPT)
	if len(*held) != 0 {
		t.Fatalf("%d Find(s) started before the reader allowed the provider", len(*held))
	}
	assertNotSentState(t, h.pane(), "the Search tab under the question")

	test.Tap(findTreeButton(q, "Not now"))
	if len(*held) != 0 || aiPermitted(h.state.aiKeys, providerOpenAI) {
		t.Fatalf("Not now started %d Find(s) or recorded a permission", len(*held))
	}
	// A rebuild draws the tab from state: still nothing sent.
	rebuildWindow(h.state)
	retry := assertNotSentState(t, h.pane(), "the rebuilt Search tab")

	test.Tap(retry)
	test.Tap(findTreeButton(permissionQuestion(t, cnv, chatGPT), "Allow"))
	if len(*held) != 1 || (*held)[0].q != "mercy" {
		t.Fatalf("Allow must start the submitted Find once; started %d", len(*held))
	}
	if !treeHasText(h.pane(), findRunning) {
		t.Errorf("the Find Allow started is not shown running; texts %v", treeTexts(h.pane()))
	}
}

// Whatever path reaches them, the two places a request leaves the device
// refuse a provider with no permission, and the refusal reads as nothing
// sent. Control: with the permission recorded, both send.
func TestTheRequestsRefuseAProviderWithoutPermission(t *testing.T) {
	s := newProviderStandIn(t)
	sent := s.route(t)
	st := refrainChapterState()
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")

	_, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", verseTwo)
	if friendlyAIError(err) != notSentMessage || isNoKeyError(err) {
		t.Errorf("a study request without permission returned %v (%q)", err, friendlyAIError(err))
	}
	_, err = runAISearch(context.Background(), st, "mercy")
	if friendlyAIError(err) != notSentMessage {
		t.Errorf("a Find without permission returned %v (%q)", err, friendlyAIError(err))
	}
	if got := s.received(); len(got) != 0 {
		t.Fatalf("%d request(s) reached the provider without permission", len(got))
	}

	grantAIPermission(st.aiKeys, defaultProviderID)
	if out, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", verseTwo); err != nil || out != "Synthetic answer." {
		t.Errorf("control: the allowed study request returned (%q, %v)", out, err)
	}
	<-sent
	if _, err := runAISearch(context.Background(), st, "mercy"); err != nil {
		t.Errorf("control: the allowed Find returned %v", err)
	}
	<-sent
	if n := len(s.received()); n != 2 {
		t.Errorf("control: the provider received %d requests, want 2", n)
	}
}

// Any change of a provider's key withdraws that provider's permission and no
// other: a key replaced by another, or cleared; choosing None withdraws every
// one. The same key saved again keeps it, and a key entered again after a
// change does not bring it back (ai_permission_key_test.go asks through the
// panel).
func TestChangingTheKeyOrChoosingNoneWithdrawsThePermission(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	for _, p := range aiProviders() {
		ks.setAPIKey(p.ID, "key-"+p.ID)
		grantAIPermission(ks, p.ID)
	}
	ks.setAPIKey(providerAnthropic, "key-"+providerAnthropic)
	if !aiPermitted(ks, providerAnthropic) {
		t.Error("saving the same key again withdrew the permission")
	}
	ks.setAPIKey(providerAnthropic, "another-key")
	if aiPermitted(ks, providerAnthropic) {
		t.Error("replacing a key with another kept the permission")
	}
	ks.setAPIKey(providerAnthropic, "key-"+providerAnthropic)
	if aiPermitted(ks, providerAnthropic) {
		t.Error("the first key put back brought the permission back without asking")
	}
	grantAIPermission(ks, providerAnthropic)
	ks.setAPIKey(providerAnthropic, "  ")
	if aiPermitted(ks, providerAnthropic) {
		t.Error("clearing the key kept the permission")
	}
	ks.setAPIKey(providerAnthropic, "key-"+providerAnthropic)
	if aiPermitted(ks, providerAnthropic) {
		t.Error("the key entered again after a clear brought the permission back without asking")
	}
	if !aiPermitted(ks, providerOpenAI) || !aiPermitted(ks, providerGemini) || !aiPermitted(ks, providerGrok) {
		t.Error("changing one provider's key withdrew another's permission")
	}
	ks.setAIEnabled(false)
	for _, p := range aiProviders() {
		if aiPermitted(ks, p.ID) {
			t.Errorf("None kept %s's permission", p.Name)
		}
	}
	ks.setAIEnabled(true)
	for _, p := range aiProviders() {
		if aiPermitted(ks, p.ID) {
			t.Errorf("choosing a provider again restored %s's permission without asking", p.Name)
		}
	}
	// The API.Bible key shares the store and has no permission record.
	ks.setBibleAPIKey("")
	if _, ok := ks.prefs.(*fakePrefs).m[prefAIAllowedPrefix+bibleKeyID]; ok {
		t.Error("clearing the API.Bible key wrote an AI permission record")
	}
	// A failed delete still withdraws: the reader asked to be done with it.
	secure := &keyStore{prefs: newFakePrefs(), secrets: &fakeSecrets{m: map[string]string{providerGemini: "k"}}}
	grantAIPermission(secure, providerGemini)
	secure.secrets.(*fakeSecrets).failWrite = true
	if secure.setAPIKey(providerGemini, "") {
		t.Fatal("control: the failing store reported the delete done")
	}
	if aiPermitted(secure, providerGemini) {
		t.Error("a clear the store could not finish kept the permission")
	}
}

// The same withdrawal through the Settings sheet the reader uses: the key
// field's Clear, then None. After each, the next study action asks again.
func TestTheSettingsSheetWithdrawsThePermission(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	// The sheet lists models with the key; the list parks, reaching no
	// provider, and the test waits for each list to park before going on.
	_, listed := holdKeyTests(t)
	ctxs := stubAIActionParked(t)
	state := deferredTestState(t, app)
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.aiKeys.setAIEnabled(true)
	state.aiKeys.setAPIKey(defaultProviderID, "test-key")
	grantAIPermission(state.aiKeys, defaultProviderID)
	state.aiKeys.setAPIKey(providerOpenAI, "test-key")
	grantAIPermission(state.aiKeys, providerOpenAI)

	showAISettings(state)
	waitParked(t, listed, "Gemini's model list")
	popup := settingsPopup(t, state)
	clearBtn := findTreeButton(popup, "Clear")
	if clearBtn == nil {
		t.Fatalf("no Clear on the key row; texts %v", treeTexts(popup))
	}
	test.Tap(clearBtn)
	if aiPermitted(state.aiKeys, defaultProviderID) {
		t.Error("Clear kept the provider's permission")
	}
	if !aiPermitted(state.aiKeys, providerOpenAI) {
		t.Error("Clear withdrew another provider's permission")
	}

	var radio *widget.RadioGroup
	walkTree(popup, func(o fyne.CanvasObject) {
		if r, ok := o.(*widget.RadioGroup); ok && radio == nil {
			radio = r
		}
	})
	if radio == nil {
		t.Fatal("no assistant radio on the sheet")
	}
	radio.SetSelected("None")
	if aiPermitted(state.aiKeys, providerOpenAI) {
		t.Error("None kept a provider's permission")
	}
	chatGPT, _ := providerByID(providerOpenAI)
	radio.SetSelected(chatGPT.Name)
	waitParked(t, listed, "ChatGPT's model list")
	test.Tap(sheetCloseButton(t, popup))

	dispatchAIAction(state, aiActionExplain, "In the beginning", selSpan{})
	permissionQuestion(t, state.window.Canvas(), chatGPT.Name)
	select {
	case <-ctxs:
		t.Fatal("a study request started after None, before the reader allowed the provider again")
	case <-time.After(300 * time.Millisecond):
	}
}

// One permission per provider: allowing Claude does not allow ChatGPT, whose
// question names it, and allowing ChatGPT leaves Claude's in place.
func TestThePermissionIsAskedForEachProvider(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)
	st, w := claudeReaderState(t)
	cnv := w.Canvas()
	grantAIPermission(st.aiKeys, providerAnthropic)
	st.aiKeys.setAPIKey(providerOpenAI, "test-key")
	st.aiKeys.setActiveProvider(providerOpenAI)

	dispatchAIAction(st, aiActionExplain, refrain, verseTwo)
	q := permissionQuestion(t, cnv, "ChatGPT (OpenAI)")
	s.nothingSent(t, "with ChatGPT not yet allowed")
	test.Tap(findTreeButton(q, "Allow"))
	landed(t, sent, "ChatGPT's first request")
	if !aiPermitted(st.aiKeys, providerOpenAI) || !aiPermitted(st.aiKeys, providerAnthropic) {
		t.Errorf("after allowing ChatGPT: ChatGPT %v, Claude %v; want both allowed",
			aiPermitted(st.aiKeys, providerOpenAI), aiPermitted(st.aiKeys, providerAnthropic))
	}
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))
}

// The question shows the approved words for each of the four providers, and
// on a phone's narrowest canvas its title wraps inside the card and every
// word and both buttons stay on the canvas. Without a window nothing is
// asked, nothing recorded, and Not now's path runs.
func TestTheQuestionWordsAreTheApprovedOnesOnEverySurface(t *testing.T) {
	for _, phone := range []bool{false, true} {
		name := map[bool]string{false: "desktop", true: "phone"}[phone]
		t.Run(name, func(t *testing.T) {
			app := test.NewApp()
			t.Cleanup(app.Quit)
			if phone {
				fyne.SetCurrentApp(phoneTestApp{app})
			}
			w := app.NewWindow("question")
			t.Cleanup(w.Close)
			size := fyne.NewSize(900, 760)
			if phone {
				size = fyne.NewSize(320, 568)
			}
			w.Resize(size)
			for _, p := range aiProviders() {
				st := sampleState()
				st.window = w
				st.aiKeys = newKeyStoreWith(newFakePrefs())
				promptAIPermission(st, p, nil, nil, nil)
				q := permissionQuestion(t, w.Canvas(), p.Name)
				qPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(q.Content)
				if qPos.X < 0 || qPos.Y < 0 || qPos.X+q.Content.Size().Width > size.Width ||
					qPos.Y+q.Content.Size().Height > size.Height {
					t.Errorf("%s: the question's card %v at %v leaves the %v canvas", p.Name, q.Content.Size(), qPos, size)
				}
				walkTree(q, func(o fyne.CanvasObject) {
					switch o.(type) {
					case *widget.RichText, *widget.Label, *widget.Button:
					default:
						return
					}
					pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
					if pos.X+o.Size().Width > qPos.X+q.Content.Size().Width+0.5 || o.Size().Width <= 0 {
						t.Errorf("%s: %q runs past the card (%v at %v)", p.Name, treeTexts(o), o.Size(), pos)
					}
				})
				test.Tap(findTreeButton(q, "Not now"))
				if aiPermitted(st.aiKeys, p.ID) {
					t.Errorf("%s: Not now recorded a permission", p.Name)
				}
			}
		})
	}

	declined := false
	promptAIPermission(&AppState{aiKeys: newKeyStoreWith(newFakePrefs())}, aiProviders()[0], func() {
		t.Error("Allow ran with no window to ask in")
	}, func() { declined = true }, func() {
		t.Error("the reopen ran with no window to ask in")
	})
	if !declined {
		t.Error("with no window to ask in, Not now's path did not run")
	}
}

// askBehindTheQuestion types question into the Ask sheet for the passage,
// taps Ask, and returns the permission question that opens over the panel.
func askBehindTheQuestion(t *testing.T, h *appearanceHarness, passage, question, provider string) *widget.PopUp {
	t.Helper()
	dispatchAIAction(h.state, aiActionAsk, passage, selSpan{})
	composer := h.top()
	field := composerField(composer)
	if field == nil {
		t.Fatalf("the Ask sheet has no question field; texts %v", treeTexts(composer))
	}
	field.SetText(question)
	test.Tap(findTreeButton(composer, "Ask"))
	return permissionQuestion(t, h.state.window.Canvas(), provider)
}

// A light/dark change or a rotation with the question up over the study
// panel brings both back, as the sheets the reader is in the middle of come
// back: the panel about the same passage, with Ask's question as typed,
// saying nothing was sent, and the question over it. Nothing is sent or
// recorded across the change, one panel and one question are left, and
// Allow then sends the question with the passage once.
func TestTheQuestionComesBackOverItsPanelAfterARebuild(t *testing.T) {
	const passage = "In the beginning"
	const question = "Who is speaking in this verse?"
	const claude = "Claude (Anthropic)"
	for _, change := range []string{"light/dark", "rotation"} {
		t.Run(change, func(t *testing.T) {
			s := newProviderStandIn(t)
			sent := s.route(t)
			h := newAppearanceHarness(t, false)
			h.state.aiKeys = newKeyStoreWith(newFakePrefs())
			h.state.aiKeys.setActiveProvider(providerAnthropic)
			h.state.aiKeys.setAPIKey(providerAnthropic, "test-key")
			cnv := h.state.window.Canvas()

			first := askBehindTheQuestion(t, h, passage, question, claude)
			if change == "light/dark" {
				h.flip()
			} else {
				h.state.window.Resize(fyne.NewSize(760, 900))
				rebuildForLayoutChange(h.state)
			}

			q := permissionQuestion(t, cnv, claude)
			if q == first || first.Visible() {
				t.Fatal("the question on top is the drained one")
			}
			if n := h.overlays(); n != 2 {
				t.Fatalf("%d overlays after the %s change, want the panel and the question", n, change)
			}
			panel := beneath(t, cnv)
			if !treeHasText(panel, question) {
				t.Errorf("the panel under the question lost the typed question; texts %v", treeTexts(panel))
			}
			assertNotSentState(t, panel, "the panel under the question after the change")
			s.nothingSent(t, "across the "+change+" change")
			if aiPermitted(h.state.aiKeys, providerAnthropic) {
				t.Fatal("the change recorded a permission")
			}

			test.Tap(findTreeButton(q, "Allow"))
			landed(t, sent, "the question Allow sends after the change")
			got := s.received()
			if len(got) != 1 || !strings.Contains(got[0], question) || !strings.Contains(got[0], passage) {
				t.Fatalf("Allow must send the question with the passage once; the provider received %q", got)
			}
			top := cnv.Overlays().Top()
			if h.overlays() != 1 || !treeHasText(top, "Synthetic answer.") || !treeHasText(top, question) {
				t.Errorf("after Allow: %d overlay(s), top texts %v; want the one panel with the question and the answer",
					h.overlays(), treeTexts(top))
			}
			test.Tap(findTreeButton(top, "Close"))
		})
	}
}

// Over a Find pane the question registers no reopen: a light/dark change
// closes it, nothing starts, and the Search tab comes back from state saying
// nothing was sent, with the search in its field; Try again asks again, and
// Allow starts the search once.
func TestAFindQuestionClosesOnARebuildAndThePaneSaysNothingWasSent(t *testing.T) {
	held := holdFinds(t)
	h := newAppearanceHarness(t, true)
	h.state.aiKeys = newKeyStoreWith(newFakePrefs())
	h.state.aiKeys.setActiveProvider(providerOpenAI)
	h.state.aiKeys.setAPIKey(providerOpenAI, "test-key")
	h.state.aiSearchMode = true
	h.state.CurrentTab = 2
	rebuildWindow(h.state)
	cnv := h.state.window.Canvas()
	const chatGPT = "ChatGPT (OpenAI)"

	h.ask("mercy")
	permissionQuestion(t, cnv, chatGPT)
	h.flip()
	if n := h.overlays(); n != 0 {
		t.Fatalf("%d overlay(s) after the change; texts %v", n, treeTexts(cnv.Overlays().Top()))
	}
	if len(*held) != 0 || aiPermitted(h.state.aiKeys, providerOpenAI) {
		t.Fatalf("the change started %d Find(s) or recorded a permission", len(*held))
	}
	retry := assertNotSentState(t, h.pane(), "the Search tab after the change")
	if got := pageEntry(t, h.state, pageFieldFind).Text; got != "mercy" {
		t.Errorf("the Find field holds %q after the change, want the search", got)
	}
	test.Tap(retry)
	test.Tap(findTreeButton(permissionQuestion(t, cnv, chatGPT), "Allow"))
	if len(*held) != 1 || (*held)[0].q != "mercy" {
		t.Fatalf("Allow after the change started %d Find(s)", len(*held))
	}
}

// The supporting note stays readable at the same caption size and muted colour
// as the other settings guidance.
func TestTheSettingsNoteIsAMutedCaption(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	holdKeyTests(t)
	state := deferredTestState(t, app)
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.aiKeys.setAIEnabled(true)
	showAISettings(state)
	popup := settingsPopup(t, state)
	var note *widget.TextSegment
	walkTree(popup, func(o fyne.CanvasObject) {
		rt, ok := o.(*widget.RichText)
		if !ok || note != nil {
			return
		}
		for _, s := range rt.Segments {
			if ts, ok := s.(*widget.TextSegment); ok && strings.HasPrefix(ts.Text, "When you use AI study, ") {
				note = ts
			}
		}
	})
	if note == nil {
		t.Fatalf("the note under the key field is not on the sheet; texts %v", treeTexts(popup))
	}
	if note.Style.SizeName != theme.SizeNameCaptionText || note.Style.ColorName != colorNameMuted {
		t.Errorf("the note is not a muted caption: colour %q, size %q", note.Style.ColorName, note.Style.SizeName)
	}
	test.Tap(sheetCloseButton(t, popup))
}
