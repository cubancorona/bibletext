package bibletext

// AI study actions. When the reader selects a passage and picks "Explain",
// "Analyze context" or "Analyze translation" from the selection menu, the chosen
// action + selected text arrive here, get turned into a prompt, and are sent to
// the user's selected AI provider (see ai_providers.go). The response is shown in
// a modal panel (see ai_panel.go).
//
// Bring-your-own-key: the user supplies a key per provider in settings
// (ai_settings.go / ai_keystore.go). Nothing is embedded in the app.

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const (
	aiActionExplain     = "explain"
	aiActionContext     = "context"
	aiActionTranslation = "translation"
)

// errNoAPIKey is a low-level "client constructed without a key" guard. The
// user-facing "you haven't set a key" case is noKeyError, which carries the
// provider so the message and the "open settings" affordance can name it.
var errNoAPIKey = errors.New("no API key configured")

type noKeyError struct{ provider providerInfo }

func (e *noKeyError) Error() string { return "no API key configured for " + e.provider.Name }

// activeAIState is the AppState the native menu callback dispatches against. The
// app is single-window/single-state, so a package singleton is fine — it mirrors
// the existing native-overlay singletons (gReadingTV, currentHost).
var activeAIState *AppState

func registerAIState(state *AppState) { activeAIState = state }

// friendlyAIError maps a raw error to a calm, reader-facing message for the panel.
func friendlyAIError(err error) string {
	var nk *noKeyError
	if errors.As(err, &nk) {
		return "No API key for " + nk.provider.Name + " yet. Open AI settings to add one."
	}
	if errors.Is(err, errNoAPIKey) {
		return "No API key configured. Open AI settings to add one."
	}
	var apiErr *apiHTTPError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			return "The AI service is busy right now — please try again in a moment."
		case http.StatusBadRequest:
			return "The AI couldn't process that selection. It may be too long — try a shorter passage."
		case http.StatusUnauthorized, http.StatusForbidden:
			return "That API key was rejected. Check it in AI settings."
		}
		return "The AI service returned an error. Please try again shortly."
	}
	return "Couldn't reach the AI service. Check your connection and try again."
}

// isNoKeyError reports whether the panel should offer "Open AI settings" rather
// than "Try again".
func isNoKeyError(err error) bool {
	var nk *noKeyError
	return errors.As(err, &nk) || errors.Is(err, errNoAPIKey)
}

// --- Prompts ----------------------------------------------------------------

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

func aiCacheKey(scope, book string, chapter int, text string) string {
	sum := sha1.Sum([]byte(text))
	return fmt.Sprintf("%s|%s|%d|%x", scope, book, chapter, sum)
}

func aiCacheGet(key string) (string, bool) {
	aiCacheMu.Lock()
	defer aiCacheMu.Unlock()
	v, ok := aiCache[key]
	return v, ok
}

func aiCacheSet(key, value string) {
	aiCacheMu.Lock()
	aiCache[key] = value
	aiCacheMu.Unlock()
}

// runAIAction returns the analysis for a selection using the active provider and
// the user's key. Results are cached (keyed by provider+action+passage) so
// re-opening the same thing doesn't spend another request.
func runAIAction(ctx context.Context, state *AppState, action, selectedText string) (string, error) {
	store := state.keys()
	id := store.activeProvider()
	info, ok := providerByID(id)
	if !ok {
		info, _ = providerByID(defaultProviderID)
		id = info.ID
	}

	book, chapter := state.CurrentBook, state.CurrentChapter
	cacheKey := aiCacheKey(id+"|"+action, book, chapter, selectedText)
	if cached, ok := aiCacheGet(cacheKey); ok {
		return cached, nil
	}

	key := providerAPIKey(store, id)
	if strings.TrimSpace(key) == "" {
		return "", &noKeyError{provider: info}
	}

	out, err := info.New(key).generate(ctx, buildAIPrompt(action, book, chapter, selectedText))
	if err != nil {
		return "", err
	}
	aiCacheSet(cacheKey, out)
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
