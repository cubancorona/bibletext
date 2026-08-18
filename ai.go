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
	aiActionAsk         = "ask" // free-form question about the selection → narrative answer
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
// errBudgetExhausted means the model used its whole output allowance on hidden
// reasoning and produced no visible answer. It is a distinct error because the
// remedy is specific — ask something narrower, or switch to the faster model —
// and because reporting it as an "empty answer" blames the provider for a
// limit the app set.
var errBudgetExhausted = errors.New("ai: output budget exhausted by reasoning")

func friendlyAIError(err error) string {
	if errors.Is(err, errBudgetExhausted) {
		return "The model spent its whole allowance thinking and didn't finish an answer. " +
			"Try a narrower question, or switch to the faster model."
	}
	var nk *noKeyError
	if errors.As(err, &nk) {
		return "No API key for " + nk.provider.Name + " yet. Open AI settings to add one."
	}
	var mg modelGoneError
	if errors.As(err, &mg) {
		// Most often a PINNED model the key can't invoke (providers list models —
		// e.g. Gemini's pro tier — that some keys aren't entitled to call), or a
		// retired default with no discoverable replacement. Name the model and
		// point at the fix; the model picker sits in AI settings (and right below
		// this message when it appears in the settings sheet's Test key result).
		if mg.tried != "" {
			return "The model “" + mg.tried + "” isn't available with your key. In AI settings, pick a different model — or “Recommended”, which keeps itself current."
		}
		return "That AI model isn't available with your key. In AI settings, pick a different model — or “Recommended”, which keeps itself current."
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
			// Google rejects an invalid key with 400 ("API key not valid…"), not
			// 401 — sniff the detail so a bad key doesn't read as "selection too
			// long" (observed in practice from the settings sheet's Test key).
			if strings.Contains(strings.ToLower(apiErr.Details), "api key") {
				return "That API key was rejected. Check it in AI settings."
			}
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
	case aiActionAsk:
		return "Answer"
	default:
		return "Explanation"
	}
}

func buildAIPrompt(action, book string, chapter int, text, version string) string {
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
			"is from the " + version + "."
	default:
		task = "Explain what the passage below means: its main idea, any imagery or terms a " +
			"general reader might not know, and how its parts connect."
	}

	return fmt.Sprintf("%s\n\n%s\n\nPassage (%s %d):\n%q", preamble, task, book, chapter, text)
}

// buildAskPrompt turns a reader's free-form question about the selected passage into a
// prompt for a narrative answer. This is the reading-menu "Ask" — distinct from the search
// page's "Find", which returns matching verses. The answer is grounded in the passage and
// may draw on directly relevant Scripture, but stays focused on the question, and says so
// plainly when the passage doesn't address it rather than speculating.
func buildAskPrompt(book string, chapter int, text, version, question string) string {
	const preamble = "You are a knowledgeable, even-handed Bible study assistant. " +
		"Answer the reader's question directly and clearly in plain language, and keep it " +
		"concise — a few short paragraphs at most. Ground your answer in the passage below " +
		"and its context; you may draw on directly relevant Scripture, but stay focused on " +
		"the question. If the passage does not address the question, say so plainly rather " +
		"than speculating. Where scholars disagree or a point is uncertain, say so briefly. " +
		"Do not use markdown headings or bullet lists."

	return fmt.Sprintf("%s\n\nThe passage is from the %s.\n\nReader's question: %q\n\nPassage (%s %d):\n%q",
		preamble, version, question, book, chapter, text)
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
// aiActionRun is a seam over the study-action call (twin of aiSearchGenerate),
// so tests can observe the context a study request runs under — the only way to
// prove Close and Cancel abandon the REQUEST rather than just the spinner.
var aiActionRun = func(ctx context.Context, state *AppState, action, selectedText, question string) (string, error) {
	return runAIAction(ctx, state, action, selectedText, question)
}

func runAIAction(ctx context.Context, state *AppState, action, selectedText, question string) (string, error) {
	store := state.keys()
	id := store.activeProvider()
	info, ok := providerByID(id)
	if !ok {
		info, _ = providerByID(defaultProviderID)
		id = info.ID
	}

	book, chapter := state.CurrentBook, state.CurrentChapter
	version := state.currentVersion().Name

	// The fixed actions (Explain / Context / Translation) build their prompt from the
	// action alone; "ask" carries a free-form question, so it builds a different prompt and
	// folds the question into the cache scope (same passage, new question → fresh answer).
	// The MODEL is part of the scope too: a faster-model switch re-asks the same
	// passage+action, and the superseded request can still settle and cache —
	// under one shared key, which model's prose the panel later serves would be
	// whichever wrote last. Distinct keys keep every answer filed under the
	// model that produced it.
	model := activeModelFor(store, id)
	scope := id + "|" + model + "|" + action
	prompt := buildAIPrompt(action, book, chapter, selectedText, version)
	if action == aiActionAsk {
		scope = id + "|" + model + "|ask|" + question
		prompt = buildAskPrompt(book, chapter, selectedText, version, question)
	}

	cacheKey := aiCacheKey(scope, book, chapter, selectedText)
	if cached, ok := aiCacheGet(cacheKey); ok {
		return cached, nil
	}

	key := providerAPIKey(store, id)
	if strings.TrimSpace(key) == "" {
		return "", &noKeyError{provider: info}
	}

	out, err := info.New(store, key).generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	aiCacheSet(cacheKey, out)
	return out, nil
}

// dispatchAIAction is the entry point the native selection-menu callback calls
// (on the Fyne UI goroutine). It opens the result panel, which drives the fetch.
// span rides the same ABI as the study dispatch so both callbacks carry the
// selection's position; the AI prompts name only book+chapter (buildAIPrompt),
// so nothing here consumes it yet — an AI verb that cites verses must take it
// from this parameter, never re-derive it from the words.
func dispatchAIAction(state *AppState, action, selectedText string, _ selSpan) {
	if state == nil {
		return
	}
	// Defense in depth: with Settings → Assistant on "None" the native menus omit
	// the Study-with-AI items (syncNativeAIMenu), so this is normally unreachable —
	// but if a stale native menu ever delivered an AI action, drop it rather than
	// open an AI panel.
	if !aiFeaturesEnabled(state) {
		return
	}
	selectedText = strings.TrimSpace(selectedText)
	if selectedText == "" {
		return
	}
	// "Ask" first collects a free-form question (a small input sheet), then opens the
	// answer panel; the fixed actions go straight to the panel.
	if action == aiActionAsk {
		promptAskQuestion(state, selectedText)
		return
	}
	showAIPanel(state, action, selectedText, "")
}
