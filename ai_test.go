package holybible

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTP implements httpClient, capturing the request and returning canned data.
type mockHTTP struct {
	lastReq    *http.Request
	statusCode int
	body       string
	err        error
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func testClient(m *mockHTTP) *geminiClient {
	return &geminiClient{apiKey: "test-key", model: "gemini-test", baseURL: "https://example.test/v1", http: m}
}

func TestGenerateSendsKeyAndURLAndParsesText(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"candidates":[{"content":{"parts":[{"text":"Hello "},{"text":"world."}]}}]}`}
	out, err := testClient(m).generate(context.Background(), "prompt text")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if out != "Hello world." {
		t.Errorf("text = %q, want %q", out, "Hello world.")
	}
	if got := m.lastReq.Header.Get("x-goog-api-key"); got != "test-key" {
		t.Errorf("api key header = %q", got)
	}
	if m.lastReq.Method != http.MethodPost {
		t.Errorf("method = %q", m.lastReq.Method)
	}
	wantURL := "https://example.test/v1/models/gemini-test:generateContent"
	if m.lastReq.URL.String() != wantURL {
		t.Errorf("url = %q, want %q", m.lastReq.URL.String(), wantURL)
	}
}

func TestGenerateRateLimitedReturnsAPIError(t *testing.T) {
	m := &mockHTTP{statusCode: http.StatusTooManyRequests, body: `{"error":{"message":"quota"}}`}
	_, err := testClient(m).generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want apiHTTPError 429, got %v", err)
	}
	if friendlyAIError(err) == "" || !strings.Contains(friendlyAIError(err), "busy") {
		t.Errorf("friendly message = %q", friendlyAIError(err))
	}
}

func TestGenerateEmptyKey(t *testing.T) {
	c := testClient(&mockHTTP{})
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
		{"empty text", `{"candidates":[{"content":{"parts":[{"text":"  "}]},"finishReason":"SAFETY"}]}`, "", true},
		{"garbage", `not json`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGeminiText([]byte(tc.body))
			if tc.wantErr && err == nil {
				t.Fatalf("want error, got %q", got)
			}
			if !tc.wantErr && (err != nil || got != tc.want) {
				t.Fatalf("got (%q, %v), want %q", got, err, tc.want)
			}
		})
	}
}

func TestBuildAIPromptVariesByAction(t *testing.T) {
	const book, chap, sel = "John", 3, "For God so loved the world"

	explain := buildAIPrompt(aiActionExplain, book, chap, sel)
	context := buildAIPrompt(aiActionContext, book, chap, sel)
	translation := buildAIPrompt(aiActionTranslation, book, chap, sel)

	for _, p := range []string{explain, context, translation} {
		if !strings.Contains(p, "John 3") {
			t.Errorf("prompt missing reference: %q", p)
		}
		if !strings.Contains(p, sel) {
			t.Errorf("prompt missing selected text: %q", p)
		}
	}
	if !strings.Contains(strings.ToLower(context), "context") {
		t.Errorf("context prompt should mention context")
	}
	if !strings.Contains(translation, "World English Bible") {
		t.Errorf("translation prompt should mention the WEB translation")
	}
	if explain == context || context == translation {
		t.Errorf("prompts should differ by action")
	}
}

func TestGeminiAPIKeyPrefersEnv(t *testing.T) {
	saved := embeddedGeminiKey
	t.Cleanup(func() { embeddedGeminiKey = saved })

	embeddedGeminiKey = "embedded-key"
	t.Setenv("GEMINI_API_KEY", "env-key")
	if got := geminiAPIKey(); got != "env-key" {
		t.Errorf("with env set, key = %q, want env-key", got)
	}

	t.Setenv("GEMINI_API_KEY", "")
	if got := geminiAPIKey(); got != "embedded-key" {
		t.Errorf("without env, key = %q, want embedded-key", got)
	}
}

func TestAIActionTitle(t *testing.T) {
	for action, want := range map[string]string{
		aiActionExplain:     "Explanation",
		aiActionContext:     "Context",
		aiActionTranslation: "Translation",
		"unknown":           "Explanation",
	} {
		if got := aiActionTitle(action); got != want {
			t.Errorf("aiActionTitle(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestRunAIActionUsesCache(t *testing.T) {
	saved := embeddedGeminiKey
	t.Cleanup(func() { embeddedGeminiKey = saved })
	t.Setenv("GEMINI_API_KEY", "") // force use of embedded
	embeddedGeminiKey = ""         // no key -> errNoAPIKey, but cache should short-circuit

	state := &AppState{CurrentBook: "Mark", CurrentChapter: 1}
	key := aiCacheKey(aiActionExplain, "Mark", 1, "the beginning")
	aiCacheMu.Lock()
	aiCache[key] = "cached answer"
	aiCacheMu.Unlock()
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
