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
