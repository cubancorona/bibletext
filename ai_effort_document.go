package bibletext

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Some catalogs list a model without its effort settings. For the selected
// model only, consult the provider's public documentation. No key, passage,
// question or account identifier is sent to a documentation host.
const effortDocAge = 24 * time.Hour

type effortDocument struct {
	Model   string   `json:"model"`
	Checked int64    `json:"checked"`
	Levels  []string `json:"levels"`
}

func effortDocKey(provider, model string) string { return "ai.effortDoc." + provider + "." + model }

func savedEffortDocument(prefs prefStore, provider, model string) (d effortDocument) {
	if prefs != nil {
		_ = json.Unmarshal([]byte(prefs.String(effortDocKey(provider, model))), &d)
	}
	return d
}

func saveEffortDocument(prefs prefStore, provider, model, target string, levels []string) {
	if prefs == nil || target == "" || levels == nil {
		return
	}
	if target != effectiveEffortTarget(prefs, provider, model, readEffortCatalog(prefs, provider)) {
		return
	}
	data, err := json.Marshal(effortDocument{target, time.Now().Unix(), levels})
	if err == nil {
		prefs.SetString(effortDocKey(provider, target), string(data))
	}
}

// Consult the public source only for a model the signed-in catalog actually
// returned. A catalog's explicit list (including an empty list) takes priority.
func effortDocumentTarget(prefs prefStore, provider, model string, models []discoveredModel) string {
	if prefs == nil || len(models) == 0 || (provider != providerOpenAI && provider != providerGemini) {
		return ""
	}
	c := readEffortCatalog(prefs, provider)
	if _, reported := c.Levels[model]; reported {
		return ""
	}
	available := slices.ContainsFunc(models, func(m discoveredModel) bool {
		return m.id == model || slices.Contains(m.aliases, model)
	})
	if !available {
		return ""
	}
	target := effectiveEffortTarget(prefs, provider, model, c)
	d := savedEffortDocument(prefs, provider, target)
	if d.Model == target && time.Since(time.Unix(d.Checked, 0)) < effortDocAge {
		return ""
	}

	return target
}

var publicModelSlug = regexp.MustCompile(`^[a-z][a-z0-9]*([.-][a-z0-9]+)*$`)
var datedModel = regexp.MustCompile(`-[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
var cardLevelList = regexp.MustCompile(`(?i)\breasoning[._ ]effort\s+supports\s*:?\s*([^.!]{1,200})(?:[.!]|$)`)

func openAIDocumentEfforts(body []byte, model string) []string {
	card := strings.ToLower(string(body))
	if !strings.Contains(card, "| chat completions | `v1/chat/completions` | supported |") {
		return nil
	}
	description, _, _ := strings.Cut(card, "## model details")
	base := datedModel.ReplaceAllString(model, "")
	identity := "model id: `" + base + "`"
	if !slices.Contains(strings.Split(description, "\n"), identity) {
		// An alias can redirect to a different card. Require a declaration
		// identifying the requested alias, rather than a pricing comparison.
		if !strings.Contains(description, "the `"+model+"` alias routes requests to ") {
			return nil
		}
	} else if base != model {
		snapshot := "\n- default snapshot: `" + model + "`\n"
		_, section, _ := strings.Cut(card, "\n## snapshots\n")
		section, _, _ = strings.Cut(section, "\n## ")
		if !strings.Contains(card, snapshot) && !slices.Contains(strings.Split(section, "\n"), "- `"+model+"`") {
			return nil
		}
	}
	description = strings.NewReplacer("`", "", "(default)", "").Replace(description)
	// Model cards can wrap a single Markdown list sentence across lines.
	// Stop at paragraph boundaries so unrelated prose cannot finish a list.
	var list string
	for _, paragraph := range strings.Split(description, "\n\n") {
		match := cardLevelList.FindStringSubmatch(strings.Join(strings.Fields(paragraph), " "))
		if len(match) == 2 {
			list = strings.TrimSpace(match[1])
			break
		}
	}
	if list == "" || strings.HasSuffix(list, ",") || strings.HasSuffix(list, " and") || strings.HasSuffix(list, " or") {
		return nil
	}
	var levels []string
	for _, level := range strings.Fields(strings.ReplaceAll(list, ",", " ")) {
		if level == "and" || level == "or" {
			continue
		}
		switch level {
		case "none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
			if !slices.Contains(levels, level) {
				levels = append(levels, level)
			}
		default:
			return nil // prose is not a list of choices
		}
	}
	return levels
}

var lookupEffortDocument = fetchEffortDocument

func fetchEffortDocument(ctx context.Context, provider, model string) []string {
	if len(model) > 100 || !publicModelSlug.MatchString(model) {
		return nil
	}
	var address string
	switch provider {
	case providerOpenAI:
		address = "https://developers.openai.com/api/docs/models/" + datedModel.ReplaceAllString(model, "") + ".md"
	case providerGemini:
		address = "https://aistudio.google.com/docs/thinking.md"
	default:
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "text/markdown")
	client := &http.Client{
		Timeout: 6 * time.Second,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) > 3 || next.URL.Scheme != "https" || next.URL.Host != req.URL.Host {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	const limit = 512 << 10
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || len(body) > limit {
		return nil
	}
	if provider == providerGemini {
		return geminiDocumentEfforts(body, model)
	}
	return openAIDocumentEfforts(body, model)
}
