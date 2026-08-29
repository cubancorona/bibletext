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
	ID   string
	Name string
	// ShortName is the product alone, without the vendor parenthetical — for
	// places where the surrounding UI already establishes the context, such
	// as the Settings key row ("Gemini key"), where the full
	// "Google Gemini (OpenAI-style) key" would crowd out the field beside it.
	ShortName string
	Model     string
	KeyURL    string
	KeyHint   string
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
	// FastModel is this provider's economy model — the option the waiting
	// screens offer when the reader would rather not wait. Empty means none.
	FastModel string
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
	// Scripture study is worth the better model. Measured 2026-08-08 on two
	// literal queries whose answers can be CHECKED against our own text: the
	// capable tier roughly doubles recall at equal-or-better precision (Claude
	// 34→60 and 51→60 verses, 88→90% and 98→100%; Grok 30→52 and 16→50). A
	// reader asking "every verse where God says do not be afraid" was getting a
	// third of the answer. Cost is the reader's own key and is pennies per
	// search either way; the real price is latency (seconds → tens of seconds),
	// which aiRequestBudget + Cancel + the "faster model" offer now cover.
	geminiModel    = "gemini-pro-latest"
	openAIModel    = "gpt-5"
	anthropicModel = "claude-opus-5"

	// The economy option per provider — offered from the waiting screens, never
	// selected automatically. These were the defaults before 2026-08-08.
	geminiFastModel    = "gemini-2.5-flash"
	openAIFastModel    = "gpt-4o-mini"
	anthropicFastModel = "claude-haiku-4-5"
	grokFastModel      = "grok-4.3"
	grokModel          = "grok-4.5"

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
	// measured 2026-08-07, a broad Find took ~1s on the fast models but ~48s on
	// a reasoning model — a legitimate answer the app reported as a failure.
	//
	// Five minutes, not three: with the capable defaults a real broad Find
	// measured 34s (Claude), 50s (Gemini), 84s (Grok) and 160s (gpt-5) —
	// 160 of a 180s budget is not headroom, it is a coin toss. This is a
	// BACKSTOP against a hung connection, not a judgement about the reader's
	// patience: the waiting screens carry Cancel and the faster-model offer, so
	// a long wait is always theirs to end.
	aiRequestBudget = 5 * time.Minute

	// aiProbeBudget covers the short, interactive round-trips — "Test key" and
	// the model-list fetch. Shorter because they are a handshake, not a
	// generation, and the reader is watching a button.
	aiProbeBudget = 45 * time.Second

	// aiMaxOutputTokens is a RUNAWAY BACKSTOP, not a spending policy. The app
	// makes the reader no promise about token usage, so it must not quietly
	// impose one — the prompt is what keeps answers short.
	//
	// The old 4096 was a silent blocker on the reasoning models. Measured
	// 2026-08-08 with gpt-5 on a Find: at 4096 and at 8192 the model spent the
	// ENTIRE budget on hidden reasoning and returned nothing — finish_reason
	// "length", zero visible lines, the reader billed for it, and the app
	// reporting "the AI returned an empty answer" as though the model had
	// failed. At 16384 it answered with 59 references (13,248 reasoning
	// tokens); at 32768, 76. So the budget was not just a floor, it was
	// capping how complete an answer could be.
	//
	// All four providers accept this value (verified live).
	aiMaxOutputTokens = 32768
)

// openAIChatOnlyExclude names OpenAI id substrings that the models endpoint
// lists but /chat/completions rejects. One var, two consumers — the settings
// dropdown (ExtraModelExclude) and self-heal (modelResolver.extraExclude) —
// so the two filters can never drift apart.
var openAIChatOnlyExclude = []string{"-pro"}

// aiProviders is the registry shown in settings and used to build clients.
func aiProviders() []providerInfo {
	return []providerInfo{
		{
			// PRODUCT (VENDOR), like the other three. This read "Google Gemini"
			// — vendor first and no parenthetical — which made it the one row
			// in the picker that did not match its neighbours, and sorted the
			// list by a word the reader does not choose by.
			ID: providerGemini, Name: "Gemini (Google)", ShortName: "Gemini", Model: geminiModel, FastModel: geminiFastModel,
			KeyURL: "https://aistudio.google.com/apikey", KeyHint: "key from Google AI Studio (starts with “AIza” or “AQ.”)",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerGemini, def: geminiModel, tier: "pro", apiKey: k,
					list:  listGeminiModels(geminiBaseURL),
					build: func(m string) aiClient { return newGeminiClient(k, m) },
				}
			},
			ListModels: listGeminiModels(geminiBaseURL),
		},
		{
			ID: providerOpenAI, Name: "ChatGPT (OpenAI)", ShortName: "ChatGPT", Model: openAIModel, FastModel: openAIFastModel,
			KeyURL: "https://platform.openai.com/api-keys", KeyHint: "key starts with “sk-”",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerOpenAI, def: openAIModel, tier: "gpt-5", apiKey: k,
					extraExclude: openAIChatOnlyExclude,
					list:         listOpenAIModels(openAIBaseURL),
					build:        func(m string) aiClient { return newOpenAIClient(k, openAIBaseURL, m) },
				}
			},
			ListModels: listOpenAIModels(openAIBaseURL),
			// OpenAI's "-pro" reasoning tier (gpt-5.x-pro, o*-pro) is Responses-API
			// only and 404s on /chat/completions — keep it out of the dropdown AND
			// out of self-heal (the "gpt-5" tier keyword matches those ids too).
			ExtraModelExclude: openAIChatOnlyExclude,
		},
		{
			ID: providerAnthropic, Name: "Claude (Anthropic)", ShortName: "Claude", Model: anthropicModel, FastModel: anthropicFastModel,
			// Anthropic's developer console moved to platform.claude.com; the old
			// console.anthropic.com URL still redirects, but sending readers
			// through a redirect to a differently-branded site is a worse first
			// impression than naming the real one (verified 2026-08-08).
			KeyURL: "https://platform.claude.com/settings/keys", KeyHint: "key starts with “sk-ant-”",
			New: func(store *keyStore, k string) aiClient {
				return &modelResolver{
					store: store, id: providerAnthropic, def: anthropicModel, tier: "opus", apiKey: k,
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
			ID: providerGrok, Name: "Grok (SpaceXAI)", ShortName: "Grok", Model: grokModel, FastModel: grokFastModel,
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
// to the operation, not the transport. Redirects are limited to the request's
// original origin because several providers authenticate with custom headers;
// Go's default cross-host stripping covers Authorization but not those headers.
func newHTTPClient() *http.Client {
	return &http.Client{CheckRedirect: sameOriginRedirect}
}

func sameOriginRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return http.ErrUseLastResponse
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	origin := via[0].URL
	if !strings.EqualFold(req.URL.Scheme, origin.Scheme) ||
		!strings.EqualFold(req.URL.Host, origin.Host) {
		return http.ErrUseLastResponse
	}
	return nil
}

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

// A provider can stop a model mid-answer — the output limit reached, a
// thinking pass that consumed the budget, a refusal part-way through. The
// reply that comes back is a real answer that simply stops, often mid-word,
// and nothing about it says so. Presenting that as finished is the worst of
// the options: the reader has no way to tell a complete explanation from one
// that was cut off, and on a passage they are studying that matters.
//
// So an early stop is marked. The partial text is kept — it was paid for and
// is usually still useful — with a line saying plainly that it stopped short.
const aiTruncatedSuffix = "\n\n*(Cut short — the model stopped before finishing this answer. " +
	"Ask again, or choose a different model in Settings.)*"

// markIfTruncated appends that line when the provider signalled an early stop.
// An empty answer is left alone: its callers turn that into a proper error,
// which is better than a notice with nothing above it.
func markIfTruncated(text string, complete bool) string {
	if complete || text == "" {
		return text
	}
	return text + aiTruncatedSuffix
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
	reason := gr.Candidates[0].FinishReason
	if text == "" {
		if reason != "" && reason != "STOP" {
			if reason == "MAX_TOKENS" {
				return "", errBudgetExhausted
			}
			return "", fmt.Errorf("the AI stopped early (%s)", reason)
		}
		return "", errors.New("the AI returned an empty answer")
	}
	return markIfTruncated(text, reason == "" || reason == "STOP"), nil
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
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	// Temperature is omitted for the reasoning families, which accept only
	// their default and reject any explicit value with a 400.
	Temperature float64 `json:"temperature,omitempty"`
	// MaxCompletionTokens, not max_tokens: the reasoning families REJECT
	// max_tokens outright ("Unsupported parameter"), and the older chat models
	// accept this spelling too — so one field serves both. Sending the old name
	// is why gpt-5 and the o-series looked "unavailable" on a key that could in
	// fact call them.
	MaxCompletionTokens int `json:"max_completion_tokens,omitempty"`
}

// openAIFixedTemperature reports whether a model rejects an explicit
// temperature (the gpt-5 and o-series reasoning families).
func openAIFixedTemperature(model string) bool {
	m := strings.ToLower(model)
	for _, p := range []string{"gpt-5", "o1", "o3", "o4"} {
		if strings.HasPrefix(m, p) {
			return true
		}
	}
	return false
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
		// FinishReason distinguishes "the model had nothing to say" from "the
		// model ran out of budget while thinking" — on the reasoning families
		// those look identical in the content field (both empty) but mean very
		// different things to the reader.
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (c *openAIClient) generate(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errNoAPIKey
	}
	reqBody := openAIRequest{
		Model:               c.model,
		Messages:            []openAIMessage{{Role: "user", Content: prompt}},
		MaxCompletionTokens: aiMaxOutputTokens,
	}
	if !openAIFixedTemperature(c.model) {
		reqBody.Temperature = 0.4
	}
	payload, err := json.Marshal(reqBody)
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
	reason := r.Choices[0].FinishReason
	if text == "" {
		// A reasoning model can spend its whole budget thinking and emit
		// nothing. Saying "the AI returned an empty answer" blames the model
		// for a limit WE set, and leaves the reader with no idea what to do.
		if reason == "length" {
			return "", errBudgetExhausted
		}
		return "", errors.New("the AI returned an empty answer")
	}
	return markIfTruncated(text, reason == "" || reason == "stop"), nil
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
	// Absent until now, which left this the one provider that could not tell a
	// finished answer from one the model was cut off in the middle of.
	// "end_turn" and "stop_sequence" are the model finishing; "max_tokens" and
	// a refusal are not.
	StopReason string `json:"stop_reason"`
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
		if r.StopReason == "max_tokens" {
			return "", errBudgetExhausted
		}
		// A refusal is the model declining, not an empty reply, and it happens
		// to scripture: Jeremiah 23 has been observed refused part-way through
		// on one model and answered in full on the next attempt. Saying "empty
		// answer" sends the reader looking for a fault that is not there.
		if r.StopReason == "refusal" {
			return "", errors.New("the model declined to answer this passage — try again, " +
				"or choose a different model in Settings")
		}
		return "", errors.New("the AI returned an empty answer")
	}
	return markIfTruncated(text, r.StopReason == "" ||
		r.StopReason == "end_turn" || r.StopReason == "stop_sequence"), nil
}

// --- Shared transport -------------------------------------------------------

// doAIRequest performs the request and returns the body, mapping non-200 to a
// typed apiHTTPError (shared with the bible fetcher). Authenticated response
// bodies are never copied into the error: a provider or proxy must not be able
// to echo a reader's credential into UI or logs. The only retained detail is a
// fixed local category used for model self-healing.
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
		apiErr := &apiHTTPError{StatusCode: resp.StatusCode, Details: safeAIErrorDetail(body)}
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

// safeAIErrorDetail maps an authenticated response to a fixed local category.
// Gemini may report an unavailable model as HTTP 400 rather than 404; retaining
// only that category preserves model self-healing without exposing body text.
func safeAIErrorDetail(body []byte) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	message := ""
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		message = env.Error.Message
	} else {
		message = string(body)
	}
	if strings.Contains(strings.ToLower(message), "model") {
		return "model unavailable"
	}
	return ""
}

var (
	_ aiClient = (*geminiClient)(nil)
	_ aiClient = (*openAIClient)(nil)
	_ aiClient = (*anthropicClient)(nil)
)
