package bibletext

// The AI request deadline. A reader who chooses a high-capability model must be
// able to actually use it, so these lock the three rules that made the old
// setup preclude one: the transport imposes no deadline of its own, the retry
// loop never re-fires a request whose budget is gone, and the reader can
// abandon a long request for real.

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestHTTPClientImposesNoDeadline: the operation's context is the ONLY deadline.
// A per-attempt client Timeout silently outranks it — the old 30s one capped
// every request below the 35s search budget, so a model that thinks for 48s
// could never answer no matter how generous the caller was.
func TestHTTPClientImposesNoDeadline(t *testing.T) {
	if got := newHTTPClient().Timeout; got != 0 {
		t.Errorf("client Timeout = %v, want 0 — the caller's context owns the deadline", got)
	}
	if aiRequestBudget < time.Minute {
		t.Errorf("aiRequestBudget = %v — too short for a thinking model", aiRequestBudget)
	}
	if aiProbeBudget >= aiRequestBudget {
		t.Errorf("probe budget %v should be shorter than the generation budget %v",
			aiProbeBudget, aiRequestBudget)
	}
}

// TestDoAIRequestDoesNotRetryExpiredBudget: once the caller's deadline has
// passed (or they cancelled), the request must NOT be re-fired. Retrying spends
// what's left of the budget on a doomed call and can bill the reader a second
// time for work the provider may already be doing.
func TestDoAIRequestDoesNotRetryExpiredBudget(t *testing.T) {
	prevSleep := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	defer func() { aiRetrySleep = prevSleep }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the reader hit Cancel / the budget ran out
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://example.invalid", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	m := &mockHTTP{err: errors.New("context deadline exceeded")}
	if _, err := doAIRequest(m, req); err == nil {
		t.Fatal("expected the failure to surface")
	}
	if m.calls != 1 {
		t.Errorf("made %d attempts on a dead budget, want exactly 1", m.calls)
	}

	// A live budget still retries genuine transient failures.
	m2 := &mockHTTP{err: errors.New("connection reset by peer")}
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"https://example.invalid", strings.NewReader("{}"))
	if _, err := doAIRequest(m2, req2); err == nil {
		t.Fatal("expected the transient failure to surface after retries")
	}
	if m2.calls < 2 {
		t.Errorf("a live budget should retry a transient error, got %d attempts", m2.calls)
	}
}

// TestStartAISearchCancelAbandonsRequest: Cancel must abort the request itself,
// not merely drop its callback — otherwise a generous budget means a cancelled
// search keeps the connection open and the tokens billing.
func TestStartAISearchCancelAbandonsRequest(t *testing.T) {
	st := psalm23State()
	// A key must be present or runAISearch short-circuits before the seam.
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")

	seen := make(chan context.Context, 1)
	release := make(chan struct{})

	prev := aiSearchGenerate
	aiSearchGenerate = func(ctx context.Context, _ providerInfo, _ *keyStore, _, _ string) (string, error) {
		seen <- ctx
		<-release // hold the "request" open until the test lets go
		return "", ctx.Err()
	}
	defer func() { aiSearchGenerate = prev }()

	cancel := startAISearch(st, "anything", func([]Verse, error) {})
	var reqCtx context.Context
	select {
	case reqCtx = <-seen:
	case <-time.After(2 * time.Second):
		t.Fatal("the search never reached the provider seam")
	}
	if reqCtx.Err() != nil {
		t.Fatal("the request context must start live")
	}

	cancel()
	select {
	case <-reqCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not abort the in-flight request context")
	}
	close(release)

	cancel() // idempotent: a second press (or one after completion) is harmless
}

// --- The desktop Find wiring -------------------------------------------------
//
// These exist because the first cut of Cancel passed every test above while
// being broken three ways in the real sidebar: the button was built before the
// hook was set (so the FIRST Find had no Cancel), the button captured the hook
// by value (so later Finds cancelled the PREVIOUS request), and the cancelled
// pane fell through to "AI didn't find matching passages" (telling the reader
// the model failed when they had stopped it). All three are user-visible and
// none were reachable by testing startAISearch alone.

// findSidebarSearchPane drives a real sidebar Find and returns the rendered
// results pane plus the context the request is running under.
func findSidebarSearchPane(t *testing.T, st *AppState) (fyne.CanvasObject, context.Context) {
	t.Helper()
	if st.retryAISearch == nil {
		t.Fatal("sidebar did not install the Find runner")
	}
	// Capture the pane the submit's OWN refresh builds. Rebuilding after the
	// call returns would hide any ordering bug, because by then every hook is
	// installed — exactly how the first cut of Cancel passed its tests while
	// rendering no button in the real app (state.refresh() -> showReading ->
	// buildSearchResultsView happens INSIDE runAsk, ui_desktop.go:38).
	var paneAtRefresh fyne.CanvasObject
	st.showReading = func() { paneAtRefresh = buildSearchResultsView(st) }

	st.retryAISearch() // re-runs the last query through the real submit path
	select {
	case ctx := <-aiCtxSeen:
		if paneAtRefresh == nil {
			t.Fatal("the submit never refreshed the reading pane")
		}
		return paneAtRefresh, ctx
	case <-time.After(2 * time.Second):
		t.Fatal("the Find never reached the provider seam")
	}
	return nil, nil
}

var aiCtxSeen chan context.Context

// stubAIGenerate makes every Find hand its context to aiCtxSeen and block.
func stubAIGenerate(t *testing.T) (release func()) {
	t.Helper()
	aiCtxSeen = make(chan context.Context, 8)
	// The AI response cache is package-global and outlives a test: when cleanup
	// releases a blocked stub whose context is still live it returns a nil
	// error, caching an empty reply for that query — the next test's Find would
	// then be served from cache and never reach the seam.
	aiCacheMu.Lock()
	aiCache = map[string]string{}
	aiCacheMu.Unlock()
	done := make(chan struct{})
	prev := aiSearchGenerate
	aiSearchGenerate = func(ctx context.Context, _ providerInfo, _ *keyStore, _, _ string) (string, error) {
		aiCtxSeen <- ctx
		<-done
		return "", ctx.Err()
	}
	var once sync.Once
	t.Cleanup(func() { aiSearchGenerate = prev; once.Do(func() { close(done) }) })
	return func() { once.Do(func() { close(done) }) }
}

func sidebarFindState(t *testing.T) *AppState {
	t.Helper()
	st := sampleState()
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	st.aiSearchMode = true
	st.aiSearchQuery = "mercy"
	buildSidebar(st) // installs retryAISearch + the Find submit path
	return st
}

// TestSidebarFindShowsCancelOnFirstSearch: the hook must be installed BEFORE
// the refresh that builds the searching view, or the first Find of a session
// renders with no way out — the exact regression the ordering caused.
func TestSidebarFindShowsCancelOnFirstSearch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer stubAIGenerate(t)()

	st := sidebarFindState(t)
	pane, _ := findSidebarSearchPane(t, st)
	if findTreeButton(pane, "Cancel") == nil {
		t.Fatalf("the first Find must render a Cancel button; got texts %v", treeTexts(pane))
	}
}

// TestSidebarCancelAbandonsTheCurrentSearch: from the second Find on, Cancel
// must abandon the request that is ACTUALLY running. Binding the hook by value
// pinned the previous one, leaving the live request billing for the full
// budget while the UI claimed it was cancelled.
func TestSidebarCancelAbandonsTheCurrentSearch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer stubAIGenerate(t)()

	st := sidebarFindState(t)
	_, ctx1 := findSidebarSearchPane(t, st)
	pane2, ctx2 := findSidebarSearchPane(t, st)

	btn := findTreeButton(pane2, "Cancel")
	if btn == nil {
		t.Fatal("no Cancel button on the second Find")
	}
	btn.OnTapped()

	select {
	case <-ctx2.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not abandon the CURRENT request (bound to a stale one?)")
	}
	// The FIRST request must already be dead — starting Find #2 supersedes it.
	// This line previously read "the first request's fate is irrelevant", which
	// blessed a real leak in writing: a reworded query left the original
	// request unreachable by every control in the app, billing the reader's key
	// for the rest of the three-minute budget.
	select {
	case <-ctx1.Done():
	default:
		t.Fatal("resubmitting orphaned the first request — nothing can cancel it now")
	}
}

// TestResubmitAbandonsPreviousRequest is the direct statement of that rule:
// N submissions must never leave N-1 uncancellable requests running.
func TestResubmitAbandonsPreviousRequest(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer stubAIGenerate(t)()

	st := sidebarFindState(t)
	_, ctx1 := findSidebarSearchPane(t, st)
	_, ctx2 := findSidebarSearchPane(t, st)

	select {
	case <-ctx1.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the superseded request is still live and unreachable")
	}
	if ctx2.Err() != nil {
		t.Fatal("the newest request must stay live")
	}

	// And the survivor is still reachable by every teardown route.
	clearSearchState(st)
	select {
	case <-ctx2.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("teardown could not reach the surviving request")
	}
}

// TestAbandonClearsProgressWithoutHook: the progress flag must clear even when
// no cancel hook is registered — that early return re-armed a previously-fixed
// bug (a permanent "Searching with AI…" pane after the ✕ or the mode toggle).
func TestAbandonClearsProgressWithoutHook(t *testing.T) {
	st := sampleState()
	st.aiSearchLoading = true
	st.cancelAISearch = nil
	abandonAISearch(st)
	if st.aiSearchLoading {
		t.Error("abandonAISearch must clear the progress flag even with no hook registered")
	}
}

// TestCancelledFlagNeverHidesRealResults: aiSearchCancelled exists only to
// suppress a FALSE zero-result message. If any path leaves it stale, it must
// still not hide passages the reader already paid for.
func TestCancelledFlagNeverHidesRealResults(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	// A key is required or buildSearchResultsView short-circuits to the no-key
	// view before ever reaching the cancelled case.
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	st.aiSearchActive = true
	st.aiSearchQuery = "shepherd"
	st.aiSearchCancelled = true // stale
	st.aiSearchResults = st.Bible.GetChapter("Psalms", 23)[:1]

	for _, txt := range treeTexts(buildSearchResultsView(st)) {
		if strings.Contains(txt, "Search cancelled") {
			t.Fatalf("a stale cancelled flag hid real results: %q", txt)
		}
	}
}

// TestAssistantNoneClearsCancelledFlag: Settings → Assistant → None tears down
// every other Find field; leaving this one set strands the next search.
func TestAssistantNoneClearsCancelledFlag(t *testing.T) {
	st := sampleState()
	st.aiSearchActive = true
	st.aiSearchCancelled = true
	clearAISearchContext(st)
	if st.aiSearchCancelled {
		t.Error("clearAISearchContext must clear aiSearchCancelled with the rest of the Find state")
	}
}

// TestSidebarCancelDoesNotClaimNoResults: a cancelled Find must never render
// the zero-results copy — that tells the reader the model failed when they
// stopped it, and invites a re-run they'd pay for on a false premise.
func TestSidebarCancelDoesNotClaimNoResults(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer stubAIGenerate(t)()

	st := sidebarFindState(t)
	pane, _ := findSidebarSearchPane(t, st)
	findTreeButton(pane, "Cancel").OnTapped()

	for _, txt := range treeTexts(buildSearchResultsView(st)) {
		if strings.Contains(txt, "didn’t find matching passages") ||
			strings.Contains(txt, "didn't find matching passages") {
			t.Fatalf("cancelled Find rendered a false zero-result message: %q", txt)
		}
	}
}

// TestAbandonAISearchCoversEveryTeardown: the ✕, the Search/Find toggle and
// clearSearchState (the iPad sidebar collapse) must all cancel the REQUEST.
// Before centralising, each of them only dropped the callback, leaking a live
// request for the rest of the three-minute budget.
func TestAbandonAISearchCoversEveryTeardown(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer stubAIGenerate(t)()

	for _, tc := range []struct {
		name     string
		teardown func(*AppState)
	}{
		{"clearSearchState", clearSearchState},
		{"clearAISearchContext", clearAISearchContext},
		{"abandonAISearch", abandonAISearch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := sidebarFindState(t)
			_, ctx := findSidebarSearchPane(t, st)
			tc.teardown(st)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
				t.Fatalf("%s left the request running", tc.name)
			}
		})
	}
}

// --- Study with AI (Explain / Analyze context / Analyze translation) ---------
//
// The panel got the same generous budget as Find, so it needs the same escape
// hatch. Before this, Close only stopped the SPINNER: the request ran on for
// the rest of the budget, billing the reader's key for an answer nobody would
// ever see — a leak that grew from 35s to three minutes with the new budget.

func studyPanelState(t *testing.T, w fyne.Window) *AppState {
	t.Helper()
	st := sampleState()
	st.window = w
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	return st
}

// stubAIAction makes every study action publish its context and block.
func stubAIAction(t *testing.T) (ctxs chan context.Context, release func()) {
	t.Helper()
	ctxs = make(chan context.Context, 4)
	done := make(chan struct{})
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string) (string, error) {
		ctxs <- ctx
		<-done
		return "", ctx.Err()
	}
	var once sync.Once
	t.Cleanup(func() { aiActionRun = prev; once.Do(func() { close(done) }) })
	return ctxs, func() { once.Do(func() { close(done) }) }
}

// TestStudyPanelCloseAbandonsRequest: Close must cancel the request, not just
// hide the panel and stop the bar.
func TestStudyPanelCloseAbandonsRequest(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctxs, release := stubAIAction(t)
	defer release()

	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionExplain, "For God so loved the world", "")
	var ctx context.Context
	select {
	case ctx = <-ctxs:
	case <-time.After(2 * time.Second):
		t.Fatal("the study action never reached the provider seam")
	}

	overlay := w.Canvas().Overlays().Top()
	if overlay == nil {
		t.Fatal("the study panel did not open")
	}
	closeBtn := findTreeButton(overlay, "Close")
	if closeBtn == nil {
		t.Fatalf("no Close button on the study panel; texts %v", treeTexts(overlay))
	}
	closeBtn.OnTapped()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Close left the study request running — it keeps billing for the whole budget")
	}
}

// TestStudyPanelOffersCancelWhileThinking: the wait can now be minutes, so the
// thinking state must offer a way out in place, and it must abandon the
// request too.
func TestStudyPanelOffersCancelWhileThinking(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctxs, release := stubAIAction(t)
	defer release()

	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionExplain, "For God so loved the world", "")
	var ctx context.Context
	select {
	case ctx = <-ctxs:
	case <-time.After(2 * time.Second):
		t.Fatal("the study action never reached the provider seam")
	}

	overlay := w.Canvas().Overlays().Top()
	cancelBtn := findTreeButton(overlay, "Cancel")
	if cancelBtn == nil {
		t.Fatalf("the thinking state must offer Cancel; texts %v", treeTexts(overlay))
	}
	cancelBtn.OnTapped()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not abandon the study request")
	}
}

// --- Settings close must refresh the Find surface ----------------------------

// TestAISurfacesChangedCoversKeyArrival locks the observed in practice case: with a
// provider already selected but no key, Find shows "Find needs your own AI
// key". Pasting a key leaves aiEnabled() unchanged, so a rule that watches only
// the assistant toggle leaves that panel up until the reader navigates away and
// back.
func TestAISurfacesChangedCoversKeyArrival(t *testing.T) {
	for _, tc := range []struct {
		name                                         string
		enabledAtOpen, keyAtOpen, enabledNow, keyNow bool
		want                                         bool
	}{
		{"key pasted for the already-selected provider", true, false, true, true, true},
		{"switched to a provider that already has a key", true, false, true, true, true},
		{"key cleared", true, true, true, false, true},
		{"assistant set to None", true, true, false, false, true},
		{"assistant turned back on with a key", false, false, true, true, true},
		{"nothing relevant changed (text size only)", true, true, true, true, false},
		{"still no key after fiddling", true, false, true, false, false},
	} {
		if got := aiSurfacesChanged(tc.enabledAtOpen, tc.keyAtOpen, tc.enabledNow, tc.keyNow); got != tc.want {
			t.Errorf("%s: aiSurfacesChanged = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestFindPaneFollowsKeyAvailability is the other half: once a rebuild happens,
// the pane must actually stop showing the set-up panel.
func TestFindPaneFollowsKeyAvailability(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiSearchActive = true

	const setup = "Find needs your own AI key"
	found := func() bool {
		for _, txt := range treeTexts(buildSearchResultsView(st)) {
			if strings.Contains(txt, setup) {
				return true
			}
		}
		return false
	}
	if !found() {
		t.Fatal("with no key the Find pane must offer the set-up panel")
	}
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	if found() {
		t.Error("with a key present the set-up panel must be gone")
	}
}

// --- The "faster model" offer -----------------------------------------------
//
// The defaults are now the capable tier, so the waiting screens carry an escape
// hatch. It must be an OFFER: present only when there is genuinely something
// faster to move to, never automatic, and never a downgrade the reader did not
// ask for.

func TestFasterModelOfferOnlyWhenThereIsSomethingFaster(t *testing.T) {
	newState := func() *AppState {
		st := sampleState()
		st.aiKeys = newKeyStoreWith(newFakePrefs())
		return st
	}

	// Default (capable) model → the economy model is on offer.
	st := newState()
	pid, model, label, ok := fasterModelOffer(st)
	if !ok {
		t.Fatal("on the capable default the offer must be available")
	}
	info, _ := providerByID(pid)
	if model != info.FastModel || model == info.Model {
		t.Errorf("offer must point at the provider's economy model: got %q (fast=%q default=%q)",
			model, info.FastModel, info.Model)
	}
	if label == "" {
		t.Error("the offer needs a label")
	}

	// Already on the economy model → nothing to offer.
	st2 := newState()
	st2.aiKeys.setOverrideModel(st2.aiKeys.activeProvider(), info.FastModel)
	if _, _, _, ok := fasterModelOffer(st2); ok {
		t.Error("no offer when the reader is already on the fast model")
	}

	// Assistant off → no AI surfaces at all.
	st3 := newState()
	st3.aiKeys.setAIEnabled(false)
	if _, _, _, ok := fasterModelOffer(st3); ok {
		t.Error("no offer when the assistant is None")
	}
}

// TestApplyFasterModelIsTheSameAsChoosingItInSettings: the switch must persist
// through the ordinary override, so the reader can see and undo it.
func TestApplyFasterModelIsTheSameAsChoosingItInSettings(t *testing.T) {
	st := sampleState()
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	pid, model, _, ok := fasterModelOffer(st)
	if !ok {
		t.Fatal("expected an offer")
	}
	applyFasterModel(st, pid, model)
	if got := st.aiKeys.overrideModel(pid); got != model {
		t.Errorf("override = %q, want %q", got, model)
	}
	if _, _, _, ok := fasterModelOffer(st); ok {
		t.Error("after switching, the offer must disappear")
	}
}

// TestEveryProviderHasADistinctFastOption: the offer is only meaningful if each
// provider actually names an economy model different from its default.
func TestEveryProviderHasADistinctFastOption(t *testing.T) {
	for _, p := range aiProviders() {
		if p.FastModel == "" {
			t.Errorf("%s has no FastModel — the waiting screens can offer nothing", p.Name)
			continue
		}
		if p.FastModel == p.Model {
			t.Errorf("%s: FastModel equals the default (%q) — the offer would be a no-op", p.Name, p.Model)
		}
	}
}

// TestOpenAIReasoningParams locks the client fix that made the capable default
// reachable at all: the gpt-5 / o-series families REJECT max_tokens and reject
// an explicit temperature, so sending either made them look "unavailable".
func TestOpenAIReasoningParams(t *testing.T) {
	for _, m := range []string{"gpt-5", "gpt-5-mini", "o1", "o3", "o4-mini"} {
		if !openAIFixedTemperature(m) {
			t.Errorf("%s must not be sent an explicit temperature", m)
		}
	}
	for _, m := range []string{"gpt-4o-mini", "gpt-4.1", "grok-4.5"} {
		if openAIFixedTemperature(m) {
			t.Errorf("%s accepts a temperature and should still get ours", m)
		}
	}
}

// TestSelfHealCacheYieldsToANewRecommendation locks the migration: a model the
// app CHOSE for a reader (self-heal, when the old default died) must not
// outrank a newer recommendation. Only an explicit choice in Settings does.
func TestSelfHealCacheYieldsToANewRecommendation(t *testing.T) {
	prefs := newFakePrefs()
	store := newKeyStoreWith(prefs)
	info, _ := providerByID(defaultProviderID)

	// Self-heal caches a stand-in for the CURRENT default.
	store.setResolvedModel(defaultProviderID, "some-healed-model")
	if got := store.resolvedModel(defaultProviderID); got != "some-healed-model" {
		t.Fatalf("cache should be honoured while it stands in for the current default, got %q", got)
	}

	// The app now ships a different recommendation: the stand-in is stale.
	prefs.SetString(prefModelResolvedForPrefix+defaultProviderID, "an-older-default")
	if got := store.resolvedModel(defaultProviderID); got != "" {
		t.Errorf("a cache healed from an older default must be discarded, got %q", got)
	}

	// An explicit choice is the reader's and survives everything.
	store.setOverrideModel(defaultProviderID, info.FastModel)
	if got := store.overrideModel(defaultProviderID); got != info.FastModel {
		t.Errorf("an explicit model choice must never be migrated away, got %q", got)
	}
}

// TestBudgetExhaustionIsReportedHonestly: a reasoning model that spends its
// whole allowance thinking must not be reported as the provider returning
// nothing — that blames the model for a limit the app set, and tells the reader
// nothing they can act on.
func TestBudgetExhaustionIsReportedHonestly(t *testing.T) {
	_, err := parseOpenAIText([]byte(`{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, errBudgetExhausted) {
		t.Fatalf("want errBudgetExhausted, got %v", err)
	}
	msg := friendlyAIError(err)
	for _, want := range []string{"thinking", "faster model"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message should mention %q and be actionable, got %q", want, msg)
		}
	}
	if strings.Contains(msg, "empty answer") {
		t.Error("must not blame the provider for our own budget")
	}

	// A genuinely empty answer still reads as one.
	_, err = parseOpenAIText([]byte(`{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`))
	if errors.Is(err, errBudgetExhausted) {
		t.Error("a normal stop must not be reported as budget exhaustion")
	}
}

// TestOutputBudgetIsNotASilentPolicy: the app makes no promise about token
// usage, so its backstop must be far above what a reasoning model needs to
// answer at all (measured: gpt-5 emitted NOTHING at 4096 and at 8192).
func TestOutputBudgetIsNotASilentPolicy(t *testing.T) {
	if aiMaxOutputTokens < 16384 {
		t.Errorf("aiMaxOutputTokens = %d — below the measured floor for a reasoning model to answer",
			aiMaxOutputTokens)
	}
	if aiSearchResultCap < 100 {
		t.Errorf("aiSearchResultCap = %d — the capable models return 76-93 on a broad question",
			aiSearchResultCap)
	}
}
