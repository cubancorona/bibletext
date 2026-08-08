package bibletext

// Live provider smoke tests. Each hits the REAL provider API using the key from
// that provider's env var (ANTHROPIC_API_KEY / OPENAI_API_KEY / GEMINI_API_KEY /
// XAI_API_KEY) and SKIPS when the key is blank — so this is inert in CI and in any
// checkout without keys, and costs nothing there.
//
// To run against real keys, put them in the gitignored .env.local (never
// committed) and, from the repo root:
//
//	set -a; source ./.env.local; set +a
//	go test -run TestLiveAIProviders -v .
//
// This exercises the whole path a "Test key" tap uses, including self-healing
// model resolution — if a provider's default model has been retired, the test
// still passes because the resolver discovers a current in-tier model.

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLiveAIProviders(t *testing.T) {
	// Isolated in-memory store: the key comes from the env override, and any
	// self-healed model is cached here (not on the real device).
	store := newKeyStoreWith(newFakePrefs())

	for _, p := range aiProviders() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			key := providerAPIKey(store, p.ID)
			if key == "" {
				t.Skipf("no key in %s — skipping (fill it in .env.local to run)", envVarFor(p.ID))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			out, err := p.New(store, key).generate(ctx, "Reply with the single word: OK")
			if err != nil {
				t.Fatalf("%s live call failed: %v", p.Name, err)
			}
			if strings.TrimSpace(out) == "" {
				t.Fatalf("%s returned an empty answer", p.Name)
			}

			reply := strings.TrimSpace(out)
			if len(reply) > 60 {
				reply = reply[:60] + "…"
			}
			model := p.Model
			if healed := store.resolvedModel(p.ID); healed != "" {
				model = healed + " (self-healed from " + p.Model + ")"
			}
			t.Logf("%s ✓ — model: %s — reply: %q", p.Name, model, reply)
		})
	}
}

// TestLiveModelDropdown exercises the settings dropdown's data path against the
// REAL providers: list models with the user's key, shape them with
// dropdownModelIDs, and require a non-empty, sane result. Same key-gating as
// TestLiveAIProviders — skips (free) wherever a key is absent.
func TestLiveModelDropdown(t *testing.T) {
	store := newKeyStoreWith(newFakePrefs())

	for _, p := range aiProviders() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			key := providerAPIKey(store, p.ID)
			if key == "" {
				t.Skipf("no key in %s — skipping (fill it in .env.local to run)", envVarFor(p.ID))
			}
			if p.ListModels == nil {
				t.Fatal("provider must expose ListModels for the dropdown")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			models, err := p.ListModels(ctx, key)
			if err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			ids := dropdownModelIDs(models, p.ExtraModelExclude, modelFamilyOf(p.Model))
			// Every offered model must actually be usable on the chat endpoint —
			// the whole point of curating the dropdown. Excludes should have
			// removed the globally non-chat families. ("-pro" is NOT here: it's
			// chat-capable on Gemini and only Responses-only on OpenAI, which the
			// per-provider p.ExtraModelExclude already applied above.)
			for _, badSub := range []string{"lyria", "codex", "robotics", "computer-use", "imagine", "embedding", "banana", "video"} {
				for _, mid := range ids {
					if strings.Contains(strings.ToLower(mid), badSub) {
						t.Errorf("%s dropdown offers non-chat model %q (matched %q)", p.Name, mid, badSub)
					}
				}
			}
			if len(ids) == 0 {
				t.Fatalf("dropdown would be empty for %s (raw list had %d)", p.Name, len(models))
			}
			if len(ids) > dropdownModelCap {
				t.Fatalf("dropdown over cap: %d", len(ids))
			}
			t.Logf("%s → %d choices, first: %v", p.Name, len(ids), ids[:min(3, len(ids))])
		})
	}
}

// TestLivePinnedDefaultsExist is the guard that would have caught the retired
// grok-2 pin: every provider's DEFAULT model must still be offered by that
// provider today. Self-heal covers a dead pin at runtime, but only by picking
// another in-tier model — for SpaceXAI that silently landed on a reasoning
// variant slow enough to blow the Find timeout. So a retired default is a real
// defect to fix deliberately, not a condition to leave to self-heal.
//
// It asserts against the provider's LIVE model list (not a chat call), so it
// costs nothing beyond one GET per provider. Same key-gating as the tests
// above: inert in CI and in any checkout without keys.
func TestLivePinnedDefaultsExist(t *testing.T) {
	store := newKeyStoreWith(newFakePrefs())

	for _, p := range aiProviders() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			key := providerAPIKey(store, p.ID)
			if key == "" {
				t.Skipf("no key in %s — skipping (fill it in .env.local to run)", envVarFor(p.ID))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// CALL the model, don't just look for it in the list. A listing is
			// not a promise: Gemini lists models that answer a chat request
			// with "no longer available to new users", so the list-only check
			// passed while the pinned default was dead and self-heal was
			// quietly covering for it on every request.
			store.setOverrideModel(p.ID, p.Model)
			defer store.setOverrideModel(p.ID, "")
			if _, err := p.New(store, key).generate(ctx, "Reply with the single word: OK"); err != nil {
				t.Errorf("%s default model %q is NOT CALLABLE — re-pin it in ai_providers.go: %v",
					p.Name, p.Model, err)
			} else {
				t.Logf("%s ✓ default %q answered", p.Name, p.Model)
			}
			// The economy model behind the "faster model" offer must work too.
			if p.FastModel != "" {
				store.setOverrideModel(p.ID, p.FastModel)
				if _, err := p.New(store, key).generate(ctx, "Reply with the single word: OK"); err != nil {
					t.Errorf("%s fast model %q is NOT CALLABLE — the waiting screens offer a dead switch: %v",
						p.Name, p.FastModel, err)
				} else {
					t.Logf("%s ✓ fast   %q answered", p.Name, p.FastModel)
				}
			}
			return
		})
	}
}

// TestLivePinnedDefaultsListed is the cheaper companion: the default should
// also appear in the provider's own catalogue (dated aliases accepted).
func TestLivePinnedDefaultsListed(t *testing.T) {
	store := newKeyStoreWith(newFakePrefs())
	for _, p := range aiProviders() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			key := providerAPIKey(store, p.ID)
			if key == "" {
				t.Skipf("no key in %s", envVarFor(p.ID))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			models, err := p.ListModels(ctx, key)
			if err != nil {
				t.Fatalf("ListModels: %v", err)
			}
			for _, m := range models {
				if m.id == p.Model {
					return
				}
				// Anthropic lists the DATED id ("claude-haiku-4-5-20251001")
				// while the undated alias we pin is what the chat endpoint
				// takes. Accept a listed id that is exactly our default plus a
				// date suffix — narrow on purpose, so "grok-4" could never be
				// satisfied by "grok-4.5".
				if rest, cut := strings.CutPrefix(m.id, p.Model+"-"); cut && isDateSuffix(rest) {
					t.Logf("%s ✓ default %q still offered (dated form %q)", p.Name, p.Model, m.id)
					return
				}
			}
			ids := make([]string, 0, len(models))
			for _, m := range models {
				ids = append(ids, m.id)
			}
			t.Errorf("%s default model %q is NO LONGER offered — re-pin it in ai_providers.go.\n"+
				"Currently offered: %s", p.Name, p.Model, strings.Join(ids, ", "))
		})
	}
}

// isDateSuffix reports whether s is a bare YYYYMMDD stamp — the only remainder
// TestLivePinnedDefaultsExist accepts after a pinned model id.
func isDateSuffix(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TestLiveKeyURLs checks the "Get a key" links each provider's settings row
// offers. They rot quietly — Anthropic's console moved to platform.claude.com
// and the old address only survived on a redirect — and a dead link is the
// first thing a new reader would hit.
//
// It asserts the link resolves to the provider's OWN domain without a
// cross-brand redirect: a redirect that lands somewhere differently branded is
// exactly the signal that the canonical URL has changed.
func TestLiveKeyURLs(t *testing.T) {
	if os.Getenv("BIBLETEXT_CHECK_LINKS") != "1" {
		t.Skip("set BIBLETEXT_CHECK_LINKS=1 to check the Get-a-key links (network)")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	for _, p := range aiProviders() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			if p.KeyURL == "" {
				t.Fatal("provider offers no Get-a-key link")
			}
			req, err := http.NewRequest(http.MethodGet, p.KeyURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Console pages block unknown agents; ask as a browser would.
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
				"AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s unreachable: %v", p.KeyURL, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 && resp.StatusCode != http.StatusForbidden {
				// 403 is bot-blocking, not a dead page (verified by hand in a
				// real browser); anything else is a genuine failure.
				t.Errorf("%s → HTTP %d", p.KeyURL, resp.StatusCode)
			}
			if final := resp.Request.URL; !sameSite(final.Host, mustHost(t, p.KeyURL)) {
				t.Errorf("%s redirects off-brand to %s — the canonical URL has probably moved",
					p.KeyURL, final.Host)
			}
			t.Logf("%-18s %s → %d %s", p.Name, p.KeyURL, resp.StatusCode, resp.Request.URL.Host)
		})
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

// sameSite treats a provider's own sign-in host as the same site (Google bounces
// aistudio → accounts.google.com, which is expected and not a moved URL).
func sameSite(got, want string) bool {
	if got == want {
		return true
	}
	reg := func(h string) string {
		parts := strings.Split(h, ".")
		if len(parts) < 2 {
			return h
		}
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return reg(got) == reg(want)
}
