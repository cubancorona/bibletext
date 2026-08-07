package bibletext

// Find result-cap benchmark: hits the REAL provider APIs with the Find prompt
// at several caps and measures latency and list quality (raw lines, verses
// that resolve against a full local Bible, duplicates, junk). Run to ground
// cap decisions in data rather than priors.
//
// Doubly gated so it can never spend money by accident: it needs BOTH the
// provider keys in the environment AND BIBLETEXT_CAPBENCH=1. From the repo root:
//
//	set -a; source ./.env.local; set +a
//	BIBLETEXT_CAPBENCH=1 go test -run TestFindCapBench -v -timeout 900s .
//
// It also needs a full Bible cache on disk (any installed copy's
// ~/Library/Caches/bibletext/bibletext-cache.json) — skips without one.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// benchFindPrompt mirrors buildAISearchPrompt with the cap as a parameter, so
// the bench varies ONLY the number while keeping the production wording.
func benchFindPrompt(query string, cap int) string {
	return "You help a reader find passages in the Bible from a request in their own words.\n\n" +
		"Request: " + strings.TrimSpace(query) + "\n\n" +
		"Reply with ONLY a list of relevant references, one per line, each written as " +
		"\"Book Chapter:Verse\" (for example: Jonah 1:2). Use full book names. Order by relevance, " +
		fmt.Sprintf("best first. Include every passage that genuinely answers the request, up to %d — ", cap) +
		"never pad with weak matches; a short list of strong matches is better than a long one. " +
		"No commentary, no numbering, no extra text — just the references."
}

// benchResolve is resolveReferenceList WITHOUT the production cap, and with
// duplicate/junk accounting — the bench must see everything the model sent.
func benchResolve(bd *BibleData, raw string) (resolved, dupes, junk int) {
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		v, ok := extractReference(bd, line)
		if !ok {
			junk++
			continue
		}
		k := fmt.Sprintf("%s|%d|%d", v.BookName, v.Chapter, v.Verse)
		if seen[k] {
			dupes++
			continue
		}
		seen[k] = true
		resolved++
	}
	return
}

func TestFindCapBench(t *testing.T) {
	if os.Getenv("BIBLETEXT_CAPBENCH") != "1" {
		t.Skip("set BIBLETEXT_CAPBENCH=1 (and source .env.local) to run the paid cap benchmark")
	}
	home, _ := os.UserHomeDir()
	bd, err := loadBibleFromCache(filepath.Join(home, "Library/Caches/bibletext/bibletext-cache.json"))
	if err != nil {
		t.Skipf("no full Bible cache to resolve against: %v", err)
	}
	bd.PrepareSearchIndex()

	caps := []int{15, 40, 120}
	queries := []struct{ name, q string }{
		{"narrow", "what did God say to Jonah?"},
		{"broad", "every 'one another' command in the New Testament"},
	}

	type row struct {
		provider, query          string
		cap                      int
		secs                     float64
		raw, resolved, dup, junk int
		err                      string
	}
	var mu sync.Mutex
	var rows []row
	var wg sync.WaitGroup

	store := newKeyStoreWith(newFakePrefs())
	for _, p := range aiProviders() {
		p := p
		key := providerAPIKey(store, p.ID)
		if key == "" {
			t.Logf("%s: no key — skipped", p.Name)
			continue
		}
		wg.Add(1)
		go func() { // providers in parallel, runs within a provider sequential
			defer wg.Done()
			client := p.New(store, key)
			for _, q := range queries {
				for _, c := range caps {
					ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					start := time.Now()
					out, err := client.generate(ctx, benchFindPrompt(q.q, c))
					secs := time.Since(start).Seconds()
					cancel()
					r := row{provider: p.Name, query: q.name, cap: c, secs: secs}
					if err != nil {
						r.err = err.Error()
					} else {
						for _, ln := range strings.Split(out, "\n") {
							if strings.TrimSpace(ln) != "" {
								r.raw++
							}
						}
						r.resolved, r.dup, r.junk = benchResolve(bd, out)
					}
					mu.Lock()
					rows = append(rows, r)
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	t.Logf("%-18s %-7s %4s %7s %5s %5s %4s %5s  %s",
		"provider", "query", "cap", "secs", "raw", "ok", "dup", "junk", "err")
	for _, r := range rows {
		t.Logf("%-18s %-7s %4d %7.2f %5d %5d %4d %5d  %s",
			r.provider, r.query, r.cap, r.secs, r.raw, r.resolved, r.dup, r.junk, r.err)
	}
}
