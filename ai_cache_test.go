package bibletext

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func isolateAICache(t *testing.T) {
	t.Helper()
	aiCacheMu.Lock()
	savedCache, savedOrder := aiCache, aiCacheOrder
	aiCache, aiCacheOrder = map[string]string{}, nil
	aiCacheMu.Unlock()
	t.Cleanup(func() {
		aiCacheMu.Lock()
		aiCache, aiCacheOrder = savedCache, savedOrder
		aiCacheMu.Unlock()
	})
}

func assertAICacheBudget(t *testing.T) {
	t.Helper()
	aiCacheMu.Lock()
	defer aiCacheMu.Unlock()
	bytes := 0
	for key, answer := range aiCache {
		bytes += len(key) + len(answer)
	}
	if bytes > aiCacheMaxBytes || len(aiCache) > aiCacheMaxEntries {
		t.Fatalf("cache retains %d bytes in %d entries; limits are %d and %d",
			bytes, len(aiCache), aiCacheMaxBytes, aiCacheMaxEntries)
	}
	if len(aiCacheOrder) != len(aiCache) {
		t.Fatalf("recency has %d entries for %d cached answers", len(aiCacheOrder), len(aiCache))
	}
	seen := make(map[string]bool, len(aiCacheOrder))
	for _, key := range aiCacheOrder {
		if _, ok := aiCache[key]; !ok || seen[key] {
			t.Fatalf("recency contains an absent or repeated key %q", key)
		}
		seen[key] = true
	}
}

func TestAICacheEntryLimitKeepsRecentlyUsedAnswers(t *testing.T) {
	isolateAICache(t)
	for i := 0; i < aiCacheMaxEntries; i++ {
		aiCacheSet(fmt.Sprintf("entry-%02d", i), fmt.Sprintf("answer-%02d", i))
	}
	if got, ok := aiCacheGet("entry-00"); !ok || got != "answer-00" {
		t.Fatalf("cached answer was not reusable: (%q, %v)", got, ok)
	}
	aiCacheSet("entry-64", "new answer")
	if _, ok := aiCacheGet("entry-01"); ok {
		t.Fatal("least recently used answer survived the entry limit")
	}
	if got, ok := aiCacheGet("entry-00"); !ok || got != "answer-00" {
		t.Fatalf("reopening an answer did not refresh recency: (%q, %v)", got, ok)
	}
	// Replacing an answer also refreshes recency without adding an entry.
	aiCacheSet("entry-02", "updated answer")
	aiCacheSet("entry-65", "another answer")
	if _, ok := aiCacheGet("entry-03"); ok {
		t.Fatal("replacement did not move the updated answer past the oldest one")
	}
	if got, ok := aiCacheGet("entry-02"); !ok || got != "updated answer" {
		t.Fatalf("updated answer was not reusable: (%q, %v)", got, ok)
	}
	assertAICacheBudget(t)
}

func TestAICacheByteLimitIncludesKeys(t *testing.T) {
	isolateAICache(t)
	keyA, keyB := strings.Repeat("a", 80), strings.Repeat("b", 80)
	answer := strings.Repeat("x", aiCacheMaxBytes/2-64)
	aiCacheSet(keyA, answer)
	aiCacheSet(keyB, answer)
	if _, ok := aiCacheGet(keyA); ok {
		t.Fatal("key bytes were omitted from the retention budget")
	}
	if got, ok := aiCacheGet(keyB); !ok || got != answer {
		t.Fatal("new answer was truncated or omitted while evicting the oldest")
	}
	assertAICacheBudget(t)
}

func TestAICacheOversizedAnswerDoesNotEvictOtherAnswers(t *testing.T) {
	isolateAICache(t)
	aiCacheSet("recent", "reusable answer")
	const key = "large"
	exact := strings.Repeat("x", aiCacheMaxBytes-len(key))
	aiCacheSet(key, exact)
	if got, ok := aiCacheGet(key); !ok || got != exact {
		t.Fatal("an answer fitting the exact byte budget was not retained intact")
	}
	assertAICacheBudget(t)
	// Seed a reusable answer again after the exact-budget entry displaced it.
	aiCacheSet("recent", "reusable answer")
	aiCacheSet(key, "previous small answer")
	aiCacheSet(key, exact+"x")
	if _, ok := aiCacheGet(key); ok {
		t.Fatal("an oversized answer was retained")
	}
	if got, ok := aiCacheGet("recent"); !ok || got != "reusable answer" {
		t.Fatal("an oversized answer displaced a reusable answer")
	}
	aiCacheSet(strings.Repeat("k", aiCacheMaxBytes+1), "")
	assertAICacheBudget(t)
}

func TestAICacheReconcilesDirectResetAndDelete(t *testing.T) {
	isolateAICache(t)
	aiCacheSet("removed", "old answer")
	aiCacheSet("kept", "current answer")
	aiCacheMu.Lock()
	delete(aiCache, "removed")
	aiCacheMu.Unlock()
	if got, ok := aiCacheGet("kept"); !ok || got != "current answer" {
		t.Fatal("direct removal changed the remaining cached answer")
	}
	assertAICacheBudget(t)
	aiCacheMu.Lock()
	aiCache = map[string]string{}
	aiCacheMu.Unlock()
	aiCacheSet("fresh", "fresh answer")
	assertAICacheBudget(t)
	if _, ok := aiCacheGet("kept"); ok {
		t.Fatal("direct cache reset left an old answer accessible")
	}
}

func TestAICacheAskIdentityHashesQuestion(t *testing.T) {
	isolateAICache(t)
	p := aiPassage{Book: "Acts", Chapter: 2, Lo: 3, Hi: 4, Version: "Synthetic edition", Text: "Synthetic selection."}
	const question = "What does this synthetic passage mean?"
	scope := aiRequestScope("synthetic-provider", "synthetic-model", aiActionAsk, question, p)
	longScope := aiRequestScope("synthetic-provider", "synthetic-model", aiActionAsk,
		strings.Repeat("A synthetic question. ", 100000), p)
	if len(scope) != len(longScope) || strings.Contains(scope, question) {
		t.Fatal("retained request scope depends on raw question length or contains the question")
	}
	key := aiCacheKey(scope, p.Book, p.Chapter, p.Text)
	aiCacheSet(key, "answer for this request")
	requestKey := func(provider, model, q string, passage aiPassage) string {
		return aiCacheKey(aiRequestScope(provider, model, aiActionAsk, q, passage),
			passage.Book, passage.Chapter, passage.Text)
	}
	if got, ok := aiCacheGet(requestKey("synthetic-provider", "synthetic-model", question, p)); !ok || got != "answer for this request" {
		t.Fatal("the same passage and question did not reuse its answer")
	}
	for _, field := range []string{"provider", "model", "question", "version", "range", "book", "chapter", "selection"} {
		t.Run(field, func(t *testing.T) {
			provider, model, q, other := "synthetic-provider", "synthetic-model", question, p
			switch field {
			case "provider":
				provider += "-other"
			case "model":
				model += "-other"
			case "question":
				q += "|3-4"
			case "version":
				other.Version += "-other"
			case "range":
				other.Lo, other.Hi = 5, 6
			case "book":
				other.Book = "Romans"
			case "chapter":
				other.Chapter++
			case "selection":
				other.Text += " More synthetic words."
			}
			if otherKey := requestKey(provider, model, q, other); otherKey == key {
				t.Fatal("distinct request produced the same cache key")
			} else if _, ok := aiCacheGet(otherKey); ok {
				t.Fatal("distinct request reused an unrelated answer")
			}
		})
	}
}

func TestAICacheConcurrentRetention(t *testing.T) {
	isolateAICache(t)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("concurrent-%d-%02d", worker, i%16)
				answer := strings.Repeat(fmt.Sprintf("synthetic-%d-%d ", worker, i), 12000)
				aiCacheSet(key, answer)
				if got, ok := aiCacheGet(key); ok && got != answer {
					t.Errorf("concurrent lookup returned a different key's answer")
					return
				}
			}
		}(worker)
	}
	workers.Wait()
	assertAICacheBudget(t)
}
