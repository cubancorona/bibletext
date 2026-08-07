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
	_ = ctx1 // the first request's fate is irrelevant; the live one must die
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
