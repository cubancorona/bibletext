package holybible

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
	New     func(apiKey string) aiClient
}

const (
	providerGemini    = "gemini"
	providerOpenAI    = "openai"
	providerAnthropic = "anthropic"
	providerGrok      = "grok"
	defaultProviderID = providerGemini

	// Default model per provider. These move fast — update them as providers
	// ship new models. (A per-provider model override in settings is a follow-up.)
	geminiModel    = "gemini-2.5-flash"
	openAIModel    = "gpt-4o-mini"
	anthropicModel = "claude-3-5-haiku-latest"
	grokModel      = "grok-2-latest"

	geminiBaseURL    = "https://generativelanguage.googleapis.com/v1beta"
	openAIBaseURL    = "https://api.openai.com/v1"
	anthropicBaseURL = "https://api.anthropic.com"
	grokBaseURL      = "https://api.x.ai/v1"

	anthropicVersion = "2023-06-01"

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
			KeyURL: "https://aistudio.google.com/apikey", KeyHint: "key starts with “AIza”",
			New: func(k string) aiClient { return newGeminiClient(k) },
		},
		{
			ID: providerOpenAI, Name: "ChatGPT (OpenAI)", Model: openAIModel,
			KeyURL: "https://platform.openai.com/api-keys", KeyHint: "key starts with “sk-”",
			New: func(k string) aiClient { return newOpenAIClient(k, openAIBaseURL, openAIModel) },
		},
		{
			ID: providerAnthropic, Name: "Claude (Anthropic)", Model: anthropicModel,
			KeyURL: "https://console.anthropic.com/settings/keys", KeyHint: "key starts with “sk-ant-”",
			New: func(k string) aiClient { return newAnthropicClient(k) },
		},
		{
			ID: providerGrok, Name: "Grok (xAI)", Model: grokModel,
			KeyURL: "https://console.x.ai", KeyHint: "key starts with “xai-”",
			New: func(k string) aiClient { return newOpenAIClient(k, grokBaseURL, grokModel) },
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

func newHTTPClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// --- Gemini (generateContent) ----------------------------------------------

type geminiClient struct {
	apiKey  string
	model   string
	baseURL string
	http    httpClient
}

func newGeminiClient(apiKey string) *geminiClient {
	return &geminiClient{apiKey: apiKey, model: geminiModel, baseURL: geminiBaseURL, http: newHTTPClient()}
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

func newAnthropicClient(apiKey string) *anthropicClient {
	return &anthropicClient{apiKey: apiKey, model: anthropicModel, baseURL: anthropicBaseURL, http: newHTTPClient()}
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
