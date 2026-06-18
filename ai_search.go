package bibletext

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// startAISearch runs runAISearch on a background goroutine and delivers the result
// back on the Fyne UI thread, so the caller can drive a spinner then render the
// passages without managing the goroutine itself.
func startAISearch(state *AppState, query string, done func([]Verse, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		verses, err := runAISearch(ctx, state, query)
		fyne.Do(func() { done(verses, err) })
	}()
}

// AI semantic search: the reader asks for passages in their own words
// ("what did God say to Jonah?") and the active AI provider returns the most
// relevant references. We use ONLY the references it returns — the verse text shown
// always comes from our own Bible data — so the model can surface passages without
// ever putting words in scripture's mouth, and a wrong reference simply drops out.

// hasAIKey reports whether the active provider has a usable key (on-device or via
// the matching env var) — i.e. whether AI features can actually run.
func hasAIKey(state *AppState) bool {
	store := state.keys()
	return strings.TrimSpace(providerAPIKey(store, store.activeProvider())) != ""
}

// aiListMarkerPattern strips a leading list bullet or "1." / "2)" numbering the
// model may add. It deliberately requires the number to be followed by a '.'/')' so
// a book number ("1 John") is never mistaken for a list marker.
var aiListMarkerPattern = regexp.MustCompile(`^\s*(?:[-*•]\s+|\d{1,2}[.)]\s+)`)

// buildAISearchPrompt asks the active provider for Bible references that answer a
// natural-language request, in a format we can parse back into real verses.
func buildAISearchPrompt(query string) string {
	return "You help a reader find passages in the Bible from a request in their own words.\n\n" +
		"Request: " + strings.TrimSpace(query) + "\n\n" +
		"Reply with ONLY a list of the most relevant references, one per line, each written as " +
		"\"Book Chapter:Verse\" (for example: Jonah 1:2). Use full book names. Order by relevance, " +
		"best first, and give at most 15. No commentary, no numbering, no extra text — just the references."
}

// runAISearch performs a natural-language passage search with the active provider
// and resolves the returned references against the loaded Bible. The returned verses
// are real scripture from our data. Results are cached per provider+query so
// re-asking the same thing is free.
func runAISearch(ctx context.Context, state *AppState, query string) ([]Verse, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	store := state.keys()
	id := store.activeProvider()
	info, ok := providerByID(id)
	if !ok {
		info, _ = providerByID(defaultProviderID)
		id = info.ID
	}
	key := providerAPIKey(store, id)
	if strings.TrimSpace(key) == "" {
		return nil, &noKeyError{provider: info}
	}

	cacheKey := aiCacheKey(id+"|search", "", 0, strings.ToLower(q))
	raw, cached := aiCacheGet(cacheKey)
	if !cached {
		out, err := info.New(key).generate(ctx, buildAISearchPrompt(q))
		if err != nil {
			return nil, err
		}
		raw = out
		aiCacheSet(cacheKey, out)
	}
	return resolveReferenceList(state.Bible, raw), nil
}

// resolveReferenceList turns the model's line-per-reference reply into real verses
// from our Bible, de-duplicated and in the order returned. Lines that don't resolve
// to an existing verse are skipped.
func resolveReferenceList(bd *BibleData, raw string) []Verse {
	if bd == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []Verse
	for _, line := range strings.Split(raw, "\n") {
		v, ok := extractReference(bd, line)
		if !ok {
			continue
		}
		k := fmt.Sprintf("%s|%d|%d", v.BookName, v.Chapter, v.Verse)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
		if len(out) >= 30 {
			break
		}
	}
	return out
}

// extractReference pulls the first resolvable "Book Chapter:Verse" out of one reply
// line. The whole line is parsed first (so multi-word books like "Song of Solomon"
// and "1 John" survive); if that fails, trailing words are dropped one at a time so
// any commentary the model tacked on is ignored.
func extractReference(bd *BibleData, line string) (Verse, bool) {
	line = strings.TrimSpace(aiListMarkerPattern.ReplaceAllString(line, ""))
	fields := strings.Fields(line)
	for n := len(fields); n >= 2; n-- {
		book, chapter, verse, hasVerse, ok := bd.parseReferenceQuery(strings.Join(fields[:n], " "))
		if !ok {
			continue
		}
		if !hasVerse {
			verse = 1 // a bare chapter -> its first verse as an anchor
		}
		if v := bd.GetVerse(book, chapter, verse); v != nil {
			return *v, true
		}
	}
	return Verse{}, false
}
