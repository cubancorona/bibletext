package bibletext

// AI study actions. When the reader selects a passage and picks "Explain",
// "Analyze context", "Analyze translation" or "Compare perspectives" from the
// selection menu, the chosen action + selected text arrive here, get turned
// into a prompt, and are sent to the user's selected AI provider (see
// ai_providers.go). The response is shown in a modal panel (see ai_panel.go).
// The verbs the menus offer are listed once, in order, in ai_compare.go
// (aiStudyVerbs); the comparison's own prompt lives there too.
//
// Bring-your-own-key: the user supplies a key per provider in settings
// (ai_settings.go / ai_keystore.go). Nothing is embedded in the app.

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

const (
	aiActionExplain     = "explain"
	aiActionContext     = "context"
	aiActionTranslation = "translation"
	aiActionCompare     = "compare" // the passage's differing readings, side by side (ai_compare.go)
	aiActionAsk         = "ask"     // free-form question about the selection → narrative answer
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
	var declined *aiDeclinedError
	if errors.As(err, &declined) {
		return "The model declined this passage. Try again, or choose a different model in AI settings."
	}
	var apiErr *apiHTTPError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusTooManyRequests:
			return "The AI service is busy right now — please try again in a moment."
		case http.StatusBadRequest:
			// Google rejects an invalid key with 400 ("API key not valid…"), not
			// 401 — sniff the detail so a bad key doesn't read as "selection too
			// long" in the settings sheet's Test-key flow.
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

// aiDeclinedError is the provider withholding an answer on this passage: a
// prompt it blocked, a reply it emptied for a safety or content reason, or a
// model's refusal. Each provider reports it in its own field (ai_providers.go),
// and each used to reach the reader as a plain error that friendlyAIError
// could not tell from a dropped connection, so a reader on a contested
// passage was told to check their network. The remedy is the one the panel's
// Try again button offers: ask again, or switch model.
type aiDeclinedError struct{ reason string }

func (e *aiDeclinedError) Error() string {
	return "the model declined this passage (" + e.reason + ")"
}

// isNoKeyError reports whether the panel should offer "Open AI settings" rather
// than "Try again".
func isNoKeyError(err error) bool {
	var nk *noKeyError
	return errors.As(err, &nk) || errors.Is(err, errNoAPIKey)
}

// --- The passage a request is about -----------------------------------------

// aiPassage is what a study prompt may name about the selection: the chapter
// it was made in, the verses it resolves to, the citation the share card
// would carry for the same selection, the cleaned text and the translation.
// Ask captures it on the UI goroutine and copies it through the question's
// lifetime; retries and sheet rebuilds must not resolve a different page.
// Ref is "John 3:16", "John 3:16–18" (an en dash for a range, as the share
// card writes it), or "John 3" when no verse can be pinned. Lo and Hi are
// 0,0 then.
type aiPassage struct {
	Book    string
	Chapter int
	Lo, Hi  int
	Ref     string
	Text    string
	Version string
	// Preview fields are captured with the selection. Text remains the
	// provider's input; HasPreview preserves an intentionally empty quote.
	VersionAbbrev string
	PreviewText   string
	HasPreview    bool
}

// aiPassageContextKey carries the panel's captured passage through aiActionRun,
// whose text-and-span signature is also the study-request test seam.
type aiPassageContextKey struct{}

func withAIPassage(ctx context.Context, p aiPassage) context.Context {
	return context.WithValue(ctx, aiPassageContextKey{}, p)
}

func aiPassageFromContext(ctx context.Context) (aiPassage, bool) {
	p, ok := ctx.Value(aiPassageContextKey{}).(aiPassage)
	return p, ok
}

// aiPassageFor resolves the selection once, in the share card's order
// (prepareShareQuoteIn), so the prompt, the panel header, the Report mail and
// the share card all cite the same verses for one and the same drag. The
// share pipeline's normalize places the selection first: a span positionally,
// dropping a verse that contributes no words; a selection that arrives with
// no position (the legacy Entry pane, the plain-text page the Mac and the
// iPhone and iPad show when a chapter fails to import, a whitespace-only
// selection on the styled pane) at its first copy in the chapter that splits
// no word. Only where that declines does selectionVersesIn answer: the raw
// span for a single partial word, the probe matching for the zero span. The
// probe alone over-attributes a selection with no position — it matches the
// first 24 runes of every verse, so a repeated refrain is every verse that
// carries it and a verse whose words open another verse is both — and the
// cross-references panel, which still takes it first (selectionVerses), can
// name more verses than the card for such a drag. A state without a Bible, or
// a selection nothing resolves, cites the chapter alone.
func aiPassageFor(state *AppState, text string, span selSpan) aiPassage {
	p := aiPassage{Text: text}
	if state == nil {
		return p
	}
	p.Book, p.Chapter = readerChapter(state)
	version := state.currentVersion()
	p.Version = version.Name
	p.VersionAbbrev = version.Abbrev
	p.PreviewText = cleanQuoteTextIn(state, p.Book, p.Chapter, text)
	p.HasPreview = true
	p.Ref = fmt.Sprintf("%s %d", p.Book, p.Chapter)
	if state.Bible == nil {
		return p
	}
	if quote, lo, hi, _, ok := normalizeShareSelectionIn(state, p.Book, p.Chapter, text, span); ok && lo >= 1 {
		p.Lo, p.Hi = lo, hi
		p.PreviewText = quote
	} else if vs := selectionVersesIn(state, p.Book, p.Chapter, text, span); len(vs) > 0 {
		p.Lo, p.Hi = vs[0].Verse, vs[len(vs)-1].Verse
	}
	if p.Lo > 0 {
		p.Ref = verseRangeCitation(p.Book, p.Chapter, p.Lo, p.Hi)
	}
	return p
}

// --- Prompts ----------------------------------------------------------------

func aiActionTitle(action string) string {
	switch action {
	case aiActionContext:
		return "Context"
	case aiActionTranslation:
		return "Translation"
	case aiActionCompare:
		return "Perspectives"
	case aiActionAsk:
		return "Answer"
	default:
		return "Explanation"
	}
}

// aiDifferingReadingsSentence is what every prose prompt says about a passage that is
// read more than one way: name the readings and what each rests on, and let
// the wording of the passage weigh, rather than flatten the difference into
// "scholars disagree". Compare shares this instruction and allows brief
// headings (ai_compare.go). Ask has its own quotation boundary below.
const aiDifferingReadingsSentence = "Where this passage is read differently, name the main readings, " +
	"who holds them and the assumption each rests on, and say what the wording of the " +
	"passage counts for or against; where a point is uncertain, say so briefly."

const aiReadingsSentence = aiDifferingReadingsSentence + " Do not quote anyone, and name " +
	"a document or writer only when you are sure of it."

func buildAIPrompt(action string, p aiPassage) string {
	const preamble = "You are a knowledgeable, even-handed Bible study assistant. " +
		"Write in clear, plain language for a general reader and keep it concise — a " +
		"few short paragraphs at most. " + aiReadingsSentence +
		" Do not use markdown headings or bullet lists."

	var task string
	switch action {
	case aiActionContext:
		task = "Explain the context of the passage below: who wrote it and to whom, what " +
			"is happening in the surrounding narrative, and how it fits the historical, " +
			"literary, and theological themes of " + p.Book + "."
	case aiActionTranslation:
		task = "Discuss translation considerations for the passage below: notable Hebrew or " +
			"Greek words behind the English, how major English translations render it " +
			"differently, and nuances that are hard to carry into English. The quoted text " +
			"is from the " + p.Version + "."
	default:
		task = "Explain what the passage below means: its main idea, any imagery or terms a " +
			"general reader might not know, and how its parts connect."
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", preamble, task, aiPassageLines(p))
}

// aiPassageLines is the tail every study prompt ends on: the citation the
// share card would carry, then the cleaned selection between curly quotation
// marks. The text goes in as it stands, not through %q: that quoting wrote an
// authored poem-line break as the two characters backslash and n, and the
// model read a line of poetry as prose with a stray escape in it.
func aiPassageLines(p aiPassage) string {
	return fmt.Sprintf("Passage (%s):\n“%s”", p.Ref, p.Text)
}

// buildAskPrompt turns a reader's free-form question about the selected passage into a
// prompt for a narrative answer. This is the reading-menu "Ask" — distinct from the search
// page's "Find", which returns matching verses. The answer is grounded in the passage and
// may draw on directly relevant Scripture, but stays focused on the question, and says so
// plainly when the passage doesn't address it rather than speculating.
func buildAskPrompt(p aiPassage, question string) string {
	const preamble = "You are a knowledgeable, even-handed Bible study assistant. " +
		"Answer the reader's question directly and clearly in plain language, and keep it " +
		"concise — a few short paragraphs at most. Ground your answer in the passage below " +
		"and its context; you may draw on directly relevant Scripture, but stay focused on " +
		"the question. If the passage does not address the question, say so plainly rather " +
		"than speculating. Distinguish what the selected passage states, what it reports " +
		"a speaker saying, and what a reading infers. Quote only wording supplied in the " +
		"passage; paraphrase surrounding context and other Scripture. If the question " +
		"assumes something the passage does not establish, say so briefly and answer " +
		"what it does support. " + aiDifferingReadingsSentence +
		" Name a document or writer only when you are sure of it. Do not use markdown " +
		"headings or bullet lists, and do not add follow-up questions."

	return fmt.Sprintf("%s\n\nThe passage is from the %s.\n\nReader's question: %q\n\n%s",
		preamble, p.Version, question, aiPassageLines(p))
}

// aiStudyPrompt is the prompt a study action sends: the three prose verbs
// through buildAIPrompt, the comparison through its own builder, Ask with the
// reader's question. The switch is the allow-list of action strings the app
// has a prompt for. The native menus hand the action over as a string
// (ai_menu_darwin.go, reading_android_export.go), and a string outside the
// list used to fall into buildAIPrompt's default and run Explain under
// Explain's title, so a mistyped or stale native entry answered as if it were
// a different verb. It is an error now, before any cache lookup or request.
func aiStudyPrompt(action string, p aiPassage, question string) (string, error) {
	switch action {
	case aiActionExplain, aiActionContext, aiActionTranslation:
		return buildAIPrompt(action, p), nil
	case aiActionCompare:
		return buildComparePrompt(p), nil
	case aiActionAsk:
		return buildAskPrompt(p, question), nil
	}
	return "", fmt.Errorf("unknown study action %q", action)
}

// aiStudyActionKnown reports whether action is one aiStudyPrompt has a prompt
// for, so a dispatch can drop an unknown string before it opens a panel.
func aiStudyActionKnown(action string) bool {
	_, err := aiStudyPrompt(action, aiPassage{}, "")
	return err == nil
}

// --- Orchestration + cache --------------------------------------------------

var (
	aiCacheMu    sync.Mutex
	aiCache      = map[string]string{}
	aiCacheOrder []string // least recently used first; guarded by aiCacheMu
)

const (
	aiCacheMaxBytes   = 8 << 20
	aiCacheMaxEntries = 64
)

func aiCacheKey(scope, book string, chapter int, text string) string {
	sum := sha1.Sum([]byte(text))
	return fmt.Sprintf("%s|%s|%d|%x", scope, book, chapter, sum)
}

func aiCacheGet(key string) (string, bool) {
	aiCacheMu.Lock()
	defer aiCacheMu.Unlock()
	aiCacheSyncLocked()
	v, ok := aiCache[key]
	if ok {
		aiCacheTouchLocked(key)
	}
	return v, ok
}

func aiCacheSet(key, value string) {
	aiCacheMu.Lock()
	defer aiCacheMu.Unlock()
	aiCacheSyncLocked()
	delete(aiCache, key)
	aiCacheRemoveLocked(key)
	// A single large answer must not displace every reusable answer. The
	// panel still displays it; only retention is bounded here.
	if len(key) > aiCacheMaxBytes || len(value) > aiCacheMaxBytes-len(key) {
		return
	}
	// Own the retained strings so a small substring cannot keep a much larger
	// request or response allocation alive beyond the byte budget.
	key, value = strings.Clone(key), strings.Clone(value)
	aiCache[key] = value
	aiCacheOrder = append(aiCacheOrder, key)
	bytes := 0
	for k, v := range aiCache {
		bytes += len(k) + len(v)
	}
	for len(aiCache) > aiCacheMaxEntries || bytes > aiCacheMaxBytes {
		oldest := aiCacheOrder[0]
		bytes -= len(oldest) + len(aiCache[oldest])
		delete(aiCache, oldest)
		aiCacheRemoveLocked(oldest)
	}
}

// Keep recency consistent with cache resets and direct removals. Missing
// entries are ordered by key so reconciliation never depends on map order.
func aiCacheSyncLocked() {
	old := aiCacheOrder
	live := old[:0]
	for _, key := range old {
		if _, ok := aiCache[key]; ok {
			live = append(live, key)
		}
	}
	clear(old[len(live):])
	aiCacheOrder = live
	if len(live) == len(aiCache) {
		return
	}
	var missing []string
	for key := range aiCache {
		found := false
		for _, known := range live {
			if key == known {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	aiCacheOrder = append(live, missing...)
}

func aiCacheRemoveLocked(key string) {
	for i, known := range aiCacheOrder {
		if known == key {
			copy(aiCacheOrder[i:], aiCacheOrder[i+1:])
			last := len(aiCacheOrder) - 1
			aiCacheOrder[last] = ""
			aiCacheOrder = aiCacheOrder[:last]
			return
		}
	}
}

func aiCacheTouchLocked(key string) {
	// Reuse the owned key rather than retaining the lookup's backing storage.
	for _, known := range aiCacheOrder {
		if known == key {
			aiCacheRemoveLocked(key)
			aiCacheOrder = append(aiCacheOrder, known)
			return
		}
	}
}

// aiRequestScope is the cache scope of one study request. The fixed actions
// (Explain / Context / Translation) are scoped by the action alone; "ask"
// carries a free-form question and the captured translation, which are folded
// in (same passage, new question or translation → fresh answer). The model
// is part of the scope too: a
// faster-model switch re-asks the same passage+action, and the superseded
// request can still settle and cache — under one shared key, which model's
// prose the panel later serves would be whichever wrote last. Distinct keys
// keep every answer filed under the model that produced it. The verse range
// enters a fixed action's scope only when the selection resolved to verses:
// the prompt then names them, so an answer written for "Psalms 136:3" must
// never be served for the identical refrain selected at verse 10, and a
// request that resolved to no verse keeps the key it always had. The
// question is hashed separately to keep retained keys fixed in size even
// for long questions, while distinguishing every question's exact wording.
func aiRequestScope(id, model, action, question string, p aiPassage) string {
	if action == aiActionAsk {
		version := sha1.Sum([]byte(p.Version))
		questionSum := sha256.Sum256([]byte(question))
		return fmt.Sprintf("%s|%s|ask|%x|%d-%d|%x", id, model, version, p.Lo, p.Hi, questionSum)
	}
	scope := id + "|" + model + "|" + action
	if p.Lo > 0 {
		scope += fmt.Sprintf("|%d-%d", p.Lo, p.Hi)
	}
	return scope
}

// runAIAction returns the analysis for a selection using the active provider and
// the user's key. Results are cached (keyed by provider+model+action+passage) so
// re-opening the same thing doesn't spend another request.
// aiActionRun is a seam over the study-action call (twin of aiSearchGenerate),
// so tests can observe the context a study request runs under — the only way to
// prove Close and Cancel abandon the REQUEST rather than just the spinner.
var aiActionRun = func(ctx context.Context, state *AppState, action, selectedText, question string, span selSpan) (string, error) {
	return runAIAction(ctx, state, action, selectedText, question, span)
}

func runAIAction(ctx context.Context, state *AppState, action, selectedText, question string, span selSpan) (string, error) {
	p, captured := aiPassageFromContext(ctx)
	if !captured {
		p = aiPassageFor(state, selectedText, span)
	}
	return runAIActionForPassage(ctx, state, action, p, question)
}

// runAIActionForPassage may read the provider and model settings, but never
// the reader's page: the prompt and cache both use the captured passage.
func runAIActionForPassage(ctx context.Context, state *AppState, action string, p aiPassage, question string) (string, error) {
	store := state.keys()
	id := store.activeProvider()
	info, ok := providerByID(id)
	if !ok {
		info, _ = providerByID(defaultProviderID)
		id = info.ID
	}

	prompt, err := aiStudyPrompt(action, p, question)
	if err != nil {
		return "", err
	}
	model := activeModelFor(store, id)
	scope := aiRequestScope(id, model, action, question, p)

	cacheKey := aiCacheKey(scope, p.Book, p.Chapter, p.Text)
	if cached, ok := aiCacheGet(cacheKey); ok {
		devDiagnosticEvent("cache_hit", devDiagnosticMetric{Key: "action", Value: devDiagnosticAction(action)},
			devDiagnosticMetric{Key: "bytes", Value: int64(len(cached))},
			devDiagnosticMetric{Key: "request_id", Value: devDiagnosticRequestID(ctx)})
		return cached, nil
	}
	devDiagnosticEvent("cache_miss", devDiagnosticMetric{Key: "action", Value: devDiagnosticAction(action)},
		devDiagnosticMetric{Key: "request_id", Value: devDiagnosticRequestID(ctx)})

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
// span is the selection's position as the pane resolved it, on the same ABI
// as the study dispatch; it rides with the text through the panel to the
// request, where aiPassageFor turns it into the verses the prompt, the panel
// header, the Report mail and the cache key name. The words alone cannot say
// which verse they came from (selection_span.go), so every consumer takes the
// span from here and none re-derives it.
func dispatchAIAction(state *AppState, action, selectedText string, span selSpan) {
	if state == nil {
		return
	}
	if action == aiActionAsk && !askQuestionEnabled {
		return
	}
	// Defense in depth: with Settings → Assistant on "None" the native menus omit
	// the Study-with-AI items (syncNativeAIMenu), so this is normally unreachable —
	// but if a stale native menu ever delivered an AI action, drop it rather than
	// open an AI panel.
	if !aiFeaturesEnabled(state) {
		return
	}
	// The same for an action string the app has no prompt for: the panel it
	// would open titles itself and asks as Explain (aiStudyPrompt), so a stale
	// or mistyped native menu entry would answer as a verb the reader did not
	// choose. Dropped here, before any text leaves the selection.
	if !aiStudyActionKnown(action) {
		return
	}
	// The reader's selection carries the page's own typography with it — the
	// superscript verse number, the no-break space glued to it, the reporter
	// layout's indent. None of that is the publisher's, and this text leaves
	// the machine, so it is cleaned exactly as the share pipeline cleans its
	// own copy of the same selection (outbound_text.go).
	selectedText = strings.TrimSpace(outboundText(selectedText))
	if selectedText == "" {
		return
	}
	// "Ask" first collects a free-form question (a small input sheet), then opens the
	// answer panel; the fixed actions go straight to the panel.
	if action == aiActionAsk {
		promptAskQuestion(state, selectedText, span)
		return
	}
	showAIPanel(state, action, selectedText, "", span)
}
