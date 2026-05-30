package holybible

// AI study actions. When the reader selects a passage and picks "Explain",
// "Analyze context" or "Analyze translation" from the selection menu, the chosen
// action + selected text arrive here, get turned into a prompt, and are sent to
// Google Gemini's generateContent REST endpoint. The response is shown in a modal
// panel (see ai_panel.go).
//
// The app authenticates with a single embedded key (see geminiAPIKey) rather than
// a per-user login. geminiAPIKey is the one seam to change if this ever moves
// behind a hosted proxy: point the client at the proxy URL and drop the header.

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	// gemini-2.5-flash balances quality, speed, and cost. Swap to
	// "gemini-2.5-flash-lite" for cheaper/faster, lower-nuance answers.
	geminiModel = "gemini-2.5-flash"

	aiActionExplain     = "explain"
	aiActionContext     = "context"
	aiActionTranslation = "translation"
)

// errNoAPIKey is returned when no Gemini key is configured; the panel turns it
// into a friendly "not configured" message.
var errNoAPIKey = errors.New("no Gemini API key configured")

// embeddedGeminiKey is set by a git-ignored ai_key.go (see ai_key.go.example).
// It's the fallback when the GEMINI_API_KEY env var is unset.
var embeddedGeminiKey string

// geminiAPIKey resolves the key: the GEMINI_API_KEY env var wins (handy for dev),
// otherwise the embedded build-time key.
func geminiAPIKey() string {
	if k := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); k != "" {
		return k
	}
	return strings.TrimSpace(embeddedGeminiKey)
}

// activeAIState is the AppState the native menu callback dispatches against. The
// app is single-window/single-state, so a package singleton is fine here — it
// mirrors the existing native-overlay singletons (gReadingTV, currentHost).
var activeAIState *AppState

func registerAIState(state *AppState) { activeAIState = state }

// --- Gemini client ----------------------------------------------------------

type geminiClient struct {
	apiKey  string
	model   string
	baseURL string
	http    httpClient // interface (shared with the bible fetcher) so tests can mock
}

func newGeminiClient(apiKey string) *geminiClient {
	return &geminiClient{
		apiKey:  apiKey,
		model:   geminiModel,
		baseURL: geminiBaseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
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

// generate sends a single prompt and returns the model's text answer.
func (c *geminiClient) generate(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return "", errNoAPIKey
	}

	payload, err := json.Marshal(geminiRequest{
		Contents:         []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: &geminiGenConfig{Temperature: 0.4, MaxOutputTokens: 1024},
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

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", &apiHTTPError{StatusCode: resp.StatusCode, Details: errorSnippet(body)}
	}

	return parseGeminiText(body)
}

// parseGeminiText pulls the answer text out of a generateContent response,
// turning the various "no usable text" shapes into errors.
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

// errorSnippet extracts a short, human-ish message from an error response body.
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

// friendlyAIError maps a raw error to a calm, reader-facing message for the panel.
func friendlyAIError(err error) string {
	if errors.Is(err, errNoAPIKey) {
		return "AI study isn't set up yet. Add a Gemini API key to enable it."
	}
	var apiErr *apiHTTPError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			return "The AI service is busy right now — please try again in a moment."
		case http.StatusBadRequest:
			return "The AI couldn't process that selection. It may be too long — try a shorter passage."
		case http.StatusUnauthorized, http.StatusForbidden:
			return "AI study isn't configured correctly (the API key was rejected)."
		}
		return "The AI service returned an error. Please try again shortly."
	}
	return "Couldn't reach the AI service. Check your connection and try again."
}

// --- Prompts ----------------------------------------------------------------

// aiActionTitle is the short label shown in the result panel header.
func aiActionTitle(action string) string {
	switch action {
	case aiActionContext:
		return "Context"
	case aiActionTranslation:
		return "Translation"
	default:
		return "Explanation"
	}
}

// buildAIPrompt composes the prompt for one action over the selected passage.
func buildAIPrompt(action, book string, chapter int, text string) string {
	const preamble = "You are a knowledgeable, even-handed Bible study assistant. " +
		"Write in clear, plain language for a general reader and keep it concise — a " +
		"few short paragraphs at most. Where scholars disagree or a point is uncertain, " +
		"say so briefly rather than overstating. Do not use markdown headings or bullet lists."

	var task string
	switch action {
	case aiActionContext:
		task = "Explain the context of the passage below: who wrote it and to whom, what " +
			"is happening in the surrounding narrative, and how it fits the historical, " +
			"literary, and theological themes of " + book + "."
	case aiActionTranslation:
		task = "Discuss translation considerations for the passage below: notable Hebrew or " +
			"Greek words behind the English, how major English translations render it " +
			"differently, and nuances that are hard to carry into English. The quoted text " +
			"is from the World English Bible."
	default:
		task = "Explain what the passage below means: its main idea, any imagery or terms a " +
			"general reader might not know, and how its parts connect."
	}

	return fmt.Sprintf("%s\n\n%s\n\nPassage (%s %d):\n%q", preamble, task, book, chapter, text)
}

// --- Orchestration + cache --------------------------------------------------

var (
	aiCacheMu sync.Mutex
	aiCache   = map[string]string{}
)

func aiCacheKey(action, book string, chapter int, text string) string {
	sum := sha1.Sum([]byte(text))
	return fmt.Sprintf("%s|%s|%d|%x", action, book, chapter, sum)
}

// runAIAction returns the analysis for a selection, using a small in-memory cache
// so re-opening the same passage+action doesn't spend another request against the
// (shared, tightly rate-limited) free-tier quota.
func runAIAction(ctx context.Context, state *AppState, action, selectedText string) (string, error) {
	book, chapter := state.CurrentBook, state.CurrentChapter
	key := aiCacheKey(action, book, chapter, selectedText)

	aiCacheMu.Lock()
	cached, ok := aiCache[key]
	aiCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	apiKey := geminiAPIKey()
	if apiKey == "" {
		return "", errNoAPIKey
	}

	out, err := newGeminiClient(apiKey).generate(ctx, buildAIPrompt(action, book, chapter, selectedText))
	if err != nil {
		return "", err
	}

	aiCacheMu.Lock()
	aiCache[key] = out
	aiCacheMu.Unlock()
	return out, nil
}

// dispatchAIAction is the entry point the native selection-menu callback calls
// (on the Fyne UI goroutine). It opens the result panel, which drives the fetch.
func dispatchAIAction(state *AppState, action, selectedText string) {
	if state == nil {
		return
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText == "" {
		return
	}
	showAIPanel(state, action, selectedText)
}
