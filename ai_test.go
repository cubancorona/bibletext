package bibletext

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockHTTP implements httpClient, capturing the request and returning canned data.
type mockHTTP struct {
	lastReq    *http.Request
	statusCode int
	body       string
	err        error
	calls      int
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

// fakePrefs is an in-memory prefStore for keyStore tests.
type fakePrefs struct{ m map[string]string }

func newFakePrefs() *fakePrefs                { return &fakePrefs{m: map[string]string{}} }
func (f *fakePrefs) String(key string) string { return f.m[key] }
func (f *fakePrefs) StringWithFallback(key, fb string) string {
	if v, ok := f.m[key]; ok {
		return v
	}
	return fb
}
func (f *fakePrefs) SetString(key, value string) { f.m[key] = value }

// --- Gemini -----------------------------------------------------------------

func geminiTestClient(m *mockHTTP) *geminiClient {
	return &geminiClient{apiKey: "test-key", model: "gemini-test", baseURL: "https://example.test/v1", http: m}
}

func TestGeminiGenerateSendsKeyURLAndParses(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"candidates":[{"content":{"parts":[{"text":"Hello "},{"text":"world."}]}}]}`}
	out, err := geminiTestClient(m).generate(context.Background(), "prompt")
	if err != nil || out != "Hello world." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("x-goog-api-key") != "test-key" {
		t.Errorf("missing api key header")
	}
	if m.lastReq.URL.String() != "https://example.test/v1/models/gemini-test:generateContent" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestGeminiRateLimited(t *testing.T) {
	m := &mockHTTP{statusCode: http.StatusTooManyRequests, body: `{"error":{"message":"quota"}}`}
	_, err := geminiTestClient(m).generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		t.Fatalf("want 429 apiHTTPError, got %v", err)
	}
	if !strings.Contains(friendlyAIError(err), "busy") {
		t.Errorf("friendly = %q", friendlyAIError(err))
	}
}

func TestDoAIRequestRetriesOn5xx(t *testing.T) {
	old := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { aiRetrySleep = old })

	m := &mockHTTP{statusCode: http.StatusServiceUnavailable, body: `{"error":{"message":"overloaded"}}`}
	c := &geminiClient{apiKey: "k", model: "m", baseURL: "https://x.test/v1", http: m}
	_, err := c.generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("want 503 apiHTTPError, got %v", err)
	}
	if m.calls != 3 {
		t.Errorf("expected 3 attempts on 5xx, got %d", m.calls)
	}
}

func TestDoAIRequestNoRetryOn4xx(t *testing.T) {
	old := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { aiRetrySleep = old })

	m := &mockHTTP{statusCode: http.StatusBadRequest, body: `{"error":{"message":"bad"}}`}
	c := &geminiClient{apiKey: "k", model: "m", baseURL: "https://x.test/v1", http: m}
	_, err := c.generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("want 400 apiHTTPError, got %v", err)
	}
	if m.calls != 1 {
		t.Errorf("expected 1 attempt on 4xx, got %d", m.calls)
	}
}

func TestGenerateEmptyKey(t *testing.T) {
	c := geminiTestClient(&mockHTTP{})
	c.apiKey = ""
	if _, err := c.generate(context.Background(), "p"); !errors.Is(err, errNoAPIKey) {
		t.Fatalf("want errNoAPIKey, got %v", err)
	}
}

func TestParseGeminiText(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"ok", `{"candidates":[{"content":{"parts":[{"text":"Answer."}]}}]}`, "Answer.", false},
		{"no candidates", `{"candidates":[]}`, "", true},
		{"blocked", `{"promptFeedback":{"blockReason":"SAFETY"}}`, "", true},
		{"garbage", `nope`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGeminiText([]byte(tc.body))
			if tc.wantErr != (err != nil) || (!tc.wantErr && got != tc.want) {
				t.Fatalf("got (%q, %v)", got, err)
			}
		})
	}
}

// --- OpenAI / Anthropic adapters --------------------------------------------

func TestOpenAIGenerate(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"choices":[{"message":{"content":"Hi."}}]}`}
	c := &openAIClient{apiKey: "k", model: "gpt", baseURL: "https://x.test/v1", http: m}
	out, err := c.generate(context.Background(), "p")
	if err != nil || out != "Hi." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("Authorization") != "Bearer k" {
		t.Errorf("auth header = %q", m.lastReq.Header.Get("Authorization"))
	}
	if m.lastReq.URL.String() != "https://x.test/v1/chat/completions" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestAnthropicGenerate(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"content":[{"type":"text","text":"Hello."}]}`}
	c := &anthropicClient{apiKey: "k", model: "claude", baseURL: "https://a.test", http: m}
	out, err := c.generate(context.Background(), "p")
	if err != nil || out != "Hello." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("x-api-key") != "k" {
		t.Errorf("x-api-key = %q", m.lastReq.Header.Get("x-api-key"))
	}
	if m.lastReq.Header.Get("anthropic-version") == "" {
		t.Errorf("missing anthropic-version header")
	}
	if m.lastReq.URL.String() != "https://a.test/v1/messages" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestParseAdapterText(t *testing.T) {
	if v, _ := parseOpenAIText([]byte(`{"choices":[{"message":{"content":"A"}}]}`)); v != "A" {
		t.Errorf("openai parse = %q", v)
	}
	if _, err := parseOpenAIText([]byte(`{"choices":[]}`)); err == nil {
		t.Errorf("openai empty should error")
	}
	if v, _ := parseAnthropicText([]byte(`{"content":[{"type":"text","text":"B"}]}`)); v != "B" {
		t.Errorf("anthropic parse = %q", v)
	}
	if _, err := parseAnthropicText([]byte(`{"content":[]}`)); err == nil {
		t.Errorf("anthropic empty should error")
	}
}

func TestProviderRegistry(t *testing.T) {
	for _, id := range []string{providerGemini, providerOpenAI, providerAnthropic, providerGrok} {
		p, ok := providerByID(id)
		if !ok || p.New == nil || p.Model == "" || p.KeyURL == "" || p.Name == "" {
			t.Errorf("provider %q incomplete: %+v ok=%v", id, p, ok)
		}
	}
	if _, ok := providerByID("nope"); ok {
		t.Errorf("unknown provider should not resolve")
	}
}

// --- Key store + resolution -------------------------------------------------

func TestKeyStore(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	if ks.activeProvider() != defaultProviderID {
		t.Errorf("default active = %q", ks.activeProvider())
	}
	ks.setActiveProvider(providerAnthropic)
	if ks.activeProvider() != providerAnthropic {
		t.Errorf("active = %q", ks.activeProvider())
	}
	ks.setActiveProvider("bogus") // ignored
	if ks.activeProvider() != providerAnthropic {
		t.Errorf("bogus provider should be ignored, got %q", ks.activeProvider())
	}
	ks.setAPIKey(providerOpenAI, "  sk-123  ")
	if ks.apiKey(providerOpenAI) != "sk-123" {
		t.Errorf("key not trimmed/stored: %q", ks.apiKey(providerOpenAI))
	}
}

func TestProviderAPIKeyPrefersEnv(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	ks.setAPIKey(providerGemini, "stored")
	t.Setenv("GEMINI_API_KEY", "env")
	if providerAPIKey(ks, providerGemini) != "env" {
		t.Errorf("env should win")
	}
	t.Setenv("GEMINI_API_KEY", "")
	if providerAPIKey(ks, providerGemini) != "stored" {
		t.Errorf("stored should be used without env")
	}
}

// --- Shared prompts / orchestration -----------------------------------------

func TestBuildAIPromptVariesByAction(t *testing.T) {
	const book, chap, sel = "John", 3, "For God so loved the world"
	explain := buildAIPrompt(aiActionExplain, book, chap, sel)
	ctx := buildAIPrompt(aiActionContext, book, chap, sel)
	trans := buildAIPrompt(aiActionTranslation, book, chap, sel)

	for _, p := range []string{explain, ctx, trans} {
		if !strings.Contains(p, "John 3") || !strings.Contains(p, sel) {
			t.Errorf("prompt missing ref/selection: %q", p)
		}
	}
	if !strings.Contains(strings.ToLower(ctx), "context") {
		t.Errorf("context prompt should mention context")
	}
	if !strings.Contains(trans, "World English Bible") {
		t.Errorf("translation prompt should mention WEB")
	}
	if explain == ctx || ctx == trans {
		t.Errorf("prompts should differ by action")
	}
}

func TestAIActionTitle(t *testing.T) {
	for action, want := range map[string]string{
		aiActionExplain: "Explanation", aiActionContext: "Context",
		aiActionTranslation: "Translation", "unknown": "Explanation",
	} {
		if got := aiActionTitle(action); got != want {
			t.Errorf("aiActionTitle(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestRunAIActionUsesCache(t *testing.T) {
	state := &AppState{CurrentBook: "Mark", CurrentChapter: 1} // no aiKeys -> inert store, active=gemini
	key := aiCacheKey(providerGemini+"|"+aiActionExplain, "Mark", 1, "the beginning")
	aiCacheSet(key, "cached answer")
	t.Cleanup(func() {
		aiCacheMu.Lock()
		delete(aiCache, key)
		aiCacheMu.Unlock()
	})

	got, err := runAIAction(context.Background(), state, aiActionExplain, "the beginning")
	if err != nil || got != "cached answer" {
		t.Fatalf("cache hit: got (%q, %v)", got, err)
	}
}

func TestRunAIActionNoKeyError(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "") // ensure no ambient dev key
	state := &AppState{CurrentBook: "Acts", CurrentChapter: 2}
	_, err := runAIAction(context.Background(), state, aiActionExplain, "unique selection no cache")
	if !isNoKeyError(err) {
		t.Fatalf("want noKeyError, got %v", err)
	}
	if !strings.Contains(friendlyAIError(err), "settings") {
		t.Errorf("friendly should point to settings: %q", friendlyAIError(err))
	}
}
