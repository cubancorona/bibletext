package bibletext

// Pluggable AI providers. Every backend implements one method — generate(prompt)
// → text — so the menu, result panel, prompts, and cache stay provider-agnostic.
// The user picks the active provider and supplies their own key (see
// ai_keystore.go / ai_settings.go); nothing is embedded in the app.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// aiClient is one chat/completion backend.
type aiClient interface {
	generate(ctx context.Context, prompt string) (string, error)
}

// providerInfo describes one selectable AI: identity, default model, where to get
// a key, and a constructor.
type providerInfo struct {
	ID      string
	Name    string
	Model   string
	KeyURL  string
	KeyHint string
	// New builds a client for this provider. It takes the on-device key store so
	// the returned client can self-heal its model (override → discovered → default,
	// re-discovering on a model-not-found error). See modelResolver.
	New func(store *keyStore, apiKey string) aiClient
	// ListModels fetches the provider's CURRENT model list with the user's key —
	// the same lister the self-heal uses. The settings sheet's model dropdown is
	// populated from it live, so new models appear as soon as the provider
	// publishes them, with no app update (and no free-typing of model ids).
	ListModels modelLister
	// ExtraModelExclude are provider-specific id substrings to keep OUT of the
	// dropdown on top of the shared modelExcludeSubstrings — for models a provider
	// lists but that don't work on its chat endpoint. Kept per-provider because
	// the same token can be valid elsewhere (OpenAI "-pro" is Responses-only and
	// 404s on chat, but Gemini's "gemini-2.5-pro" is a fine chat model).
	ExtraModelExclude []string
}

const (
	providerGemini    = "gemini"
	providerOpenAI    = "openai"
	providerAnthropic = "anthropic"
	providerGrok      = "grok"
	defaultProviderID = providerGemini

	// Default model per provider — a starting point, not a hard dependency. These
	// are the model actually sent only until the provider retires it; on a
	// model-not-found error the client self-heals to a current model in the same
	// tier (see ai_model_resolve.go), and a per-provider override in Settings can
	// pin a specific one. Update them when convenient, but a stale default no
	// longer breaks the feature on shipped installs.
	geminiModel    = "gemini-2.5-flash"
	openAIModel    = "gpt-4o-mini"
	anthropicModel = "claude-haiku-4-5"
	// The grok-2 line was retired when xAI became SpaceXAI; grok-4.3 is the
	// current mainline chat model. Self-heal covered the dead pin, but only by
	// picking the NEWEST in-tier model — grok-4.5, whose reasoning pass took
	// ~48s on a broad Find (past the 35s timeout). See reasoningModelSubstrings.
	grokModel = "grok-4.3"

	geminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta"
	openAIBaseURL    = "https://api.openai.com/v1"
	anthropicBaseURL = "https://api.anthropic.com"
	grokBaseURL      = "https://api.x.ai/v1"

	anthropicVersion = "2023-06-01"

	// aiRequestBudget is how long an AI request may take before the app gives up
	// — the SINGLE deadline for the whole operation (see newHTTPClient).
	//
	// It is deliberately generous because the reader chooses the model. The old
	// 35s (over a 30s transport cap) quietly excluded the high-capability tier:
	// measured 2026-08-07, a broad Find took ~1s on Claude/OpenAI's fast models
	// but ~48s on SpaceXAI's reasoning model — a legitimate answer the app
	// reported as a failure. Three minutes clears every model measured with room
	// to spare, and it is a BACKSTOP against a hung connection, not the reader's
	// patience: aiSearchingView offers Cancel, so a long wait is always the
	// reader's choice to end.
	aiRequestBudget = 3 * time.Minute

	// aiProbeBudget covers the short, interactive round-trips — "Test key" and
	// the model-list fetch. Shorter because they are a handshake, not a
	// generation, and the reader is watching a button.
	aiProbeBudget = 45 * time.Second

	// aiMaxOutputTokens caps each answer. It's generous because "thinking" models
	// (e.g. gemini-2.5-flash) spend part of this budget on hidden reasoning, so a
	// low cap truncates the visible answer mid-sentence. The prompt keeps answers
	// concise, so a high cap just prevents truncation rather than producing essays.
	aiMaxOutputTokens = 4096
)

// aiProviders is the registry shown in settings and used to build clients.
func aiProviders() []providerInfo {
	return []providerInfo{
		{
			ID: providerGemini, Name: "Google Gemini", Model: geminiModel,
			KeyURL: "https://aistudio.google.com/apikey", KeyHint: "key from Google AI Studio (starts with “AIza” or “AQ.”)",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerGemini, def: geminiModel, tier: "flash", apiKey: k,
					list:  listGeminiModels(geminiBaseURL),
					build: func(m string) aiClient { return newGeminiClient(k, m) },
				}
			},
			ListModels: listGeminiModels(geminiBaseURL),
		},
		{
			ID: providerOpenAI, Name: "ChatGPT (OpenAI)", Model: openAIModel,
			KeyURL: "https://platform.openai.com/api-keys", KeyHint: "key starts with “sk-”",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerOpenAI, def: openAIModel, tier: "mini", apiKey: k,
					list:  listOpenAIModels(openAIBaseURL),
					build: func(m string) aiClient { return newOpenAIClient(k, openAIBaseURL, m) },
				}
			},
			ListModels: listOpenAIModels(openAIBaseURL),
			// OpenAI's "-pro" reasoning tier (gpt-5.x-pro, o*-pro) is Responses-API
			// only and 404s on /chat/completions — keep it out of the dropdown.
			ExtraModelExclude: []string{"-pro"},
		},
		{
			ID: providerAnthropic, Name: "Claude (Anthropic)", Model: anthropicModel,
			KeyURL: "https://console.anthropic.com/settings/keys", KeyHint: "key starts with “sk-ant-”",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerAnthropic, def: anthropicModel, tier: "haiku", apiKey: k,
					list:  listAnthropicModels(anthropicBaseURL),
					build: func(m string) aiClient { return newAnthropicClient(k, m) },
				}
			},
			ListModels: listAnthropicModels(anthropicBaseURL),
		},
		{
			// xAI merged into SpaceX and rebranded SpaceXAI (July 2026); the
			// assistant keeps the name Grok, and api.x.ai / console.x.ai still
			// serve (a SpaceX-branded endpoint is promised with a long
			// transition — swap grokBaseURL when it lands).
			ID: providerGrok, Name: "Grok (SpaceXAI)", Model: grokModel,
			KeyURL: "https://console.x.ai", KeyHint: "key starts with “xai-”",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerGrok, def: grokModel, tier: "grok", apiKey: k,
					list:  listOpenAIModels(grokBaseURL),
					build: func(m string) aiClient { return newOpenAIClient(k, grokBaseURL, m) },
				}
			},
			ListModels: listOpenAIModels(grokBaseURL),
		},
	}
}

func providerByID(id string) (providerInfo, bool) {
	for _, p := range aiProviders() {
		if p.ID == id {
			return p, true
		}
	}
	return providerInfo{}, false
}

// providerAPIKey resolves a provider's key: a per-provider env var wins (handy for
// dev), otherwise the user's stored key.
func providerAPIKey(store *keyStore, id string) string {
	if k := strings.TrimSpace(os.Getenv(envVarFor(id))); k != "" {
		return k
	}
	return store.apiKey(id)
}

func envVarFor(id string) string {
	switch id {
	case providerOpenAI:
		return "OPENAI_API_KEY"
	case providerAnthropic:
		return "ANTHROPIC_API_KEY"
	case providerGrok:
		return "XAI_API_KEY"
	default:
		return "GEMINI_API_KEY"
	}
}

// newHTTPClient deliberately sets NO client-level timeout: the caller's context
// is the single deadline (aiRequestBudget and friends). A client Timeout is a
// PER-ATTEMPT cap that fights the context — the old 30s one silently outranked
// the 35s search budget, so a reader who picked a high-capability model could
// never get an answer from it: the request was killed at 30s no matter what,
// and (worse) doAIRequest read that as a network blip and fired the same
// expensive request again. One authority for "how long to wait", and it belongs
// to the operation, not the transport.
func newHTTPClient() *http.Client { return &http.Client{} }

// --- Gemini (generateContent) ----------------------------------------------

type geminiClient struct {
	apiKey  string
	model   string
	baseURL string
	http    httpClient
}

func newGeminiClient(apiKey, model string) *geminiClient {
	return &geminiClient{apiKey: apiKey, model: model, baseURL: geminiBaseURL, http: newHTTPClient()}
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig *geminiGenConfig `json:"generationConfig,omitempty"`
}
type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}
type geminiPart struct {
	Text string `json:"text"`
}
type geminiGenConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}
type geminiResponse struct {
	Candidates []struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

func (c *geminiClient) generate(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errNoAPIKey
	}
	payload, err := json.Marshal(geminiRequest{
		Contents:         []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: &geminiGenConfig{Temperature: 0.4, MaxOutputTokens: aiMaxOutputTokens},
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	body, err := doAIRequest(c.http, req)
	if err != nil {
		return "", err
	}
	return parseGeminiText(body)
}

func parseGeminiText(body []byte) (string, error) {
	var gr geminiResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return "", fmt.Errorf("decoding AI response: %w", err)
	}
	if gr.PromptFeedback != nil && gr.PromptFeedback.BlockReason != "" {
		return "", fmt.Errorf("request blocked (%s)", gr.PromptFeedback.BlockReason)
	}
	if len(gr.Candidates) == 0 {
		return "", errors.New("the AI returned no answer")
	}
	var sb strings.Builder
	for _, p := range gr.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		if reason := gr.Candidates[0].FinishReason; reason != "" && reason != "STOP" {
			return "", fmt.Errorf("the AI stopped early (%s)", reason)
		}
		return "", errors.New("the AI returned an empty answer")
	}
	return text, nil
}

// --- OpenAI-compatible (ChatGPT + Grok share /chat/completions) -------------

type openAIClient struct {
	apiKey  string
	model   string
	baseURL string
	http    httpClient
}

func newOpenAIClient(apiKey, baseURL, model string) *openAIClient {
	return &openAIClient{apiKey: apiKey, model: model, baseURL: baseURL, http: newHTTPClient()}
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}
type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *openAIClient) generate(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errNoAPIKey
	}
	payload, err := json.Marshal(openAIRequest{
		Model:       c.model,
		Messages:    []openAIMessage{{Role: "user", Content: prompt}},
		Temperature: 0.4,
		MaxTokens:   aiMaxOutputTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	body, err := doAIRequest(c.http, req)
	if err != nil {
		return "", err
	}
	return parseOpenAIText(body)
}

func parseOpenAIText(body []byte) (string, error) {
	var r openAIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("decoding AI response: %w", err)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("the AI returned no answer")
	}
	text := strings.TrimSpace(r.Choices[0].Message.Content)
	if text == "" {
		return "", errors.New("the AI returned an empty answer")
	}
	return text, nil
}

// --- Anthropic (Claude /v1/messages) ---------------------------------------

type anthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	http    httpClient
}

func newAnthropicClient(apiKey, model string) *anthropicClient {
	return &anthropicClient{apiKey: apiKey, model: model, baseURL: anthropicBaseURL, http: newHTTPClient()}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *anthropicClient) generate(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errNoAPIKey
	}
	payload, err := json.Marshal(anthropicRequest{
		Model:     c.model,
		MaxTokens: aiMaxOutputTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	body, err := doAIRequest(c.http, req)
	if err != nil {
		return "", err
	}
	return parseAnthropicText(body)
}

func parseAnthropicText(body []byte) (string, error) {
	var r anthropicResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("decoding AI response: %w", err)
	}
	var sb strings.Builder
	for _, b := range r.Content {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", errors.New("the AI returned an empty answer")
	}
	return text, nil
}

// --- Shared transport -------------------------------------------------------

// doAIRequest performs the request and returns the body, mapping non-200 to a
// typed apiHTTPError (shared with the bible fetcher) carrying a short detail.
//
// It retries a couple of times on transient failures — network errors and 5xx
// server responses, which the providers return intermittently under load — so a
// momentary blip doesn't surface as a hard error to the reader. 4xx (bad key,
// bad request, rate limit) are returned immediately; retrying those wouldn't help.
func doAIRequest(client httpClient, req *http.Request) ([]byte, error) {
	const attempts = 3
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if req.GetBody != nil {
				if b, err := req.GetBody(); err == nil {
					req.Body = b
				}
			}
			aiRetrySleep(time.Duration(attempt) * 600 * time.Millisecond)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err // network error — transient, retry
			// …unless the caller's deadline passed or they cancelled. That is
			// not transient: retrying burns what's left of the budget and can
			// bill the reader a second time for a request the provider may
			// already be completing. Give up and report it.
			if ctxErr := req.Context().Err(); ctxErr != nil {
				return nil, err
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		apiErr := &apiHTTPError{StatusCode: resp.StatusCode, Details: errorSnippet(body)}
		if resp.StatusCode >= 500 {
			lastErr = apiErr // server error — transient, retry
			continue
		}
		return nil, apiErr // 4xx — caller-fixable, don't retry
	}
	return nil, lastErr
}

// aiRetrySleep is a seam for tests.
var aiRetrySleep = time.Sleep

// errorSnippet extracts a short, human-ish message from an error response body
// (OpenAI/Gemini use {"error":{"message":...}}; Anthropic uses the same shape).
func errorSnippet(body []byte) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		return env.Error.Message
	}
	s := strings.TrimSpace(string(body))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

var (
	_ aiClient = (*geminiClient)(nil)
	_ aiClient = (*openAIClient)(nil)
	_ aiClient = (*anthropicClient)(nil)
)
