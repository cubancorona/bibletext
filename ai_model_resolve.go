package bibletext

// Self-healing model resolution.
//
// Every provider's default model is a hardcoded ID (ai_providers.go) that the
// provider eventually retires — when that happens the API returns a
// model-not-found error and the AI features break for everyone still on that app
// build, with the fix (a new constant) gated behind an app-store release.
//
// modelResolver removes that dependency. The model actually sent is resolved in
// order: (1) the user's explicit override in Settings, (2) a model previously
// discovered by self-heal and cached on-device, (3) the hardcoded default. When a
// request fails because the model no longer exists, the resolver asks the provider
// for its live model list, picks a current model in the SAME tier as the default
// (a bring-your-own-key user is never silently upgraded to a pricier tier), caches
// it, and retries once. Discovery runs ONLY after a model-not-found error, so the
// common path adds no network call and behaves exactly as before.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// discoveredModel is one entry from a provider's model list, normalized to an id
// and a rank where higher = newer/preferred (a unix timestamp, or a parsed
// version number for providers that don't date their models).
type discoveredModel struct {
	id   string
	rank int64
}

// modelLister fetches a provider's available models using the user's key.
type modelLister func(ctx context.Context, apiKey string) ([]discoveredModel, error)

// modelResolver is the self-healing aiClient wrapper (see file comment).
type modelResolver struct {
	store  *keyStore
	id     string // provider id
	def    string // hardcoded default model
	tier   string // tier keyword: haiku / mini / flash / grok
	apiKey string
	list   modelLister
	build  func(model string) aiClient
}

var _ aiClient = (*modelResolver)(nil)

// currentModel resolves the model to send: override → cached-discovered → default.
func (r *modelResolver) currentModel() string {
	if r.store != nil {
		if o := strings.TrimSpace(r.store.overrideModel(r.id)); o != "" {
			return o
		}
		if c := strings.TrimSpace(r.store.resolvedModel(r.id)); c != "" {
			return c
		}
	}
	return r.def
}

func (r *modelResolver) generate(ctx context.Context, prompt string) (string, error) {
	model := r.currentModel()
	out, err := r.build(model).generate(ctx, prompt)
	if err == nil || !isModelNotFound(err) {
		return out, err
	}
	// The configured model is gone — try to self-heal to a current in-tier model.
	fresh, listErr := r.rediscover(ctx, model)
	if fresh == "" {
		// If discovery couldn't even list the models (bad key, no credits, offline),
		// that underlying error is far more useful than "model unavailable" — surface
		// it. Only when the list SUCCEEDED but held no usable in-tier model do we
		// report the model as gone.
		if listErr != nil {
			return "", listErr
		}
		return "", modelGoneError{provider: r.id, tried: model}
	}
	return r.build(fresh).generate(ctx, prompt)
}

// rediscover asks the provider for a current in-tier model distinct from the one
// that just failed, and caches it. It returns ("", nil) when the provider listed
// fine but has no usable in-tier replacement, and ("", err) when the list call
// itself failed (so the caller can surface the real reason).
func (r *modelResolver) rediscover(ctx context.Context, tried string) (string, error) {
	if r.list == nil {
		return "", nil
	}
	models, err := r.list(ctx, r.apiKey)
	if err != nil {
		return "", err
	}
	best, ok := pickInTier(models, r.tier)
	if !ok || best == tried {
		return "", nil
	}
	if r.store != nil {
		r.store.setResolvedModel(r.id, best)
	}
	return best, nil
}

// isModelNotFound reports whether an error is the provider saying the requested
// model doesn't exist — the trigger for self-heal. All four providers return 404
// for a retired/unknown model; Gemini can also answer 400 with "model" in the
// message. Anything else (401 bad key, 429 busy, 5xx) is NOT model-not-found, so
// self-heal never fires on a transient problem or masks a real one.
func isModelNotFound(err error) bool {
	var he *apiHTTPError
	if !errors.As(err, &he) {
		return false
	}
	if he.StatusCode == http.StatusNotFound {
		return true
	}
	return he.StatusCode == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(he.Details), "model")
}

// modelGoneError is returned when the configured model is gone AND self-heal
// couldn't supply a replacement (offline, key rejected by the models endpoint, or
// no in-tier model). friendlyAIError turns it into an actionable message.
type modelGoneError struct{ provider, tried string }

func (e modelGoneError) Error() string {
	return "AI model \"" + e.tried + "\" is unavailable for " + e.provider
}

// modelExcludeSubstrings drop non-chat variants that share a tier keyword — e.g.
// OpenAI's "gpt-4o-mini-tts" / "-realtime" / "-search-preview" all contain "mini",
// and Gemini ships "flash" TTS/image variants. Selecting one of these would send a
// model that can't answer a text prompt.
var modelExcludeSubstrings = []string{
	"audio", "realtime", "tts", "transcribe", "search", "embedding",
	"moderation", "image", "vision", "whisper", "dall", "guard", "rerank",
}

// modelUnstableSubstrings are de-prioritized (used only when nothing stable is in
// tier) so a stable model always wins over a preview/experimental one.
var modelUnstableSubstrings = []string{"preview", "-exp", "experimental", "nightly", "beta"}

// pickInTier chooses the best current model whose id contains the tier keyword,
// dropping non-chat variants, preferring stable over preview and newer over older.
// It NEVER falls back to an out-of-tier model, so self-heal can't silently swap a
// cheap default for a pricier one.
func pickInTier(models []discoveredModel, tier string) (string, bool) {
	tier = strings.ToLower(tier)
	if tier == "" {
		return "", false
	}
	type cand struct {
		id     string
		stable bool
		rank   int64
	}
	var cands []cand
	for _, m := range models {
		low := strings.ToLower(m.id)
		if !strings.Contains(low, tier) || containsAnySubstr(low, modelExcludeSubstrings) {
			continue
		}
		cands = append(cands, cand{m.id, !containsAnySubstr(low, modelUnstableSubstrings), m.rank})
	}
	if len(cands) == 0 {
		return "", false
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].stable != cands[j].stable {
			return cands[i].stable // stable before preview
		}
		if cands[i].rank != cands[j].rank {
			return cands[i].rank > cands[j].rank // newer first
		}
		return cands[i].id > cands[j].id // deterministic tiebreak
	})
	return cands[0].id, true
}

func containsAnySubstr(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// --- Provider model listers -------------------------------------------------
//
// Each returns the provider's available models. They reuse doAIRequest (retry +
// typed apiHTTPError) and the same auth as that provider's generate call.

// listAnthropicModels: GET /v1/models (x-api-key + version). rank = created_at.
func listAnthropicModels(baseURL string) modelLister {
	return func(ctx context.Context, apiKey string) ([]discoveredModel, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models?limit=1000", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		body, err := doAIRequest(newHTTPClient(), req)
		if err != nil {
			return nil, err
		}
		var r struct {
			Data []struct {
				ID        string `json:"id"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		out := make([]discoveredModel, 0, len(r.Data))
		for _, m := range r.Data {
			out = append(out, discoveredModel{id: m.ID, rank: parseRFC3339Unix(m.CreatedAt)})
		}
		return out, nil
	}
}

// listOpenAIModels: GET {base}/models (Bearer). rank = created (unix). Shared by
// OpenAI and Grok — xAI's API is OpenAI-compatible.
func listOpenAIModels(baseURL string) modelLister {
	return func(ctx context.Context, apiKey string) ([]discoveredModel, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		body, err := doAIRequest(newHTTPClient(), req)
		if err != nil {
			return nil, err
		}
		var r struct {
			Data []struct {
				ID      string `json:"id"`
				Created int64  `json:"created"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		out := make([]discoveredModel, 0, len(r.Data))
		for _, m := range r.Data {
			out = append(out, discoveredModel{id: m.ID, rank: m.Created})
		}
		return out, nil
	}
}

// listGeminiModels: GET {base}/models (x-goog-api-key). Keeps only models that
// support generateContent; rank = parsed version (2.5 → 25), since Gemini's list
// carries no timestamp.
func listGeminiModels(baseURL string) modelLister {
	return func(ctx context.Context, apiKey string) ([]discoveredModel, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models?pageSize=1000", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("x-goog-api-key", apiKey)
		body, err := doAIRequest(newHTTPClient(), req)
		if err != nil {
			return nil, err
		}
		var r struct {
			Models []struct {
				Name    string   `json:"name"`
				Methods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, err
		}
		var out []discoveredModel
		for _, m := range r.Models {
			if !containsExact(m.Methods, "generateContent") {
				continue
			}
			id := strings.TrimPrefix(m.Name, "models/")
			out = append(out, discoveredModel{id: id, rank: parseVersionRank(id)})
		}
		return out, nil
	}
}

func containsExact(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// parseRFC3339Unix parses an RFC3339 timestamp to unix seconds; 0 on failure.
func parseRFC3339Unix(s string) int64 {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	return 0
}

// parseVersionRank extracts the first "N" or "N.M" version in an id and returns
// N*10+M (e.g. "gemini-2.5-flash" → 25, "gemini-1.5-flash" → 15). 0 if none.
func parseVersionRank(id string) int64 {
	i := strings.IndexFunc(id, func(r rune) bool { return r >= '0' && r <= '9' })
	if i < 0 {
		return 0
	}
	j := i
	for j < len(id) && id[j] >= '0' && id[j] <= '9' {
		j++
	}
	major, _ := strconv.Atoi(id[i:j])
	minor := 0
	if j < len(id) && id[j] == '.' {
		k := j + 1
		for k < len(id) && id[k] >= '0' && id[k] <= '9' {
			k++
		}
		minor, _ = strconv.Atoi(id[j+1 : k])
	}
	return int64(major*10 + minor)
}
