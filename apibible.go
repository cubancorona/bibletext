package bibletext

// API.Bible (American Bible Society) client — the provider behind licensed
// translations. HarperCollins' own permissions desk directs NKJV licensing
// here, and the app's Starter-plan agreement with API.Bible is strictly
// non-commercial, which BibleText satisfies (free, no ads, no revenue).
//
// The fetch shape mirrors the other whole-Bible sources: assemble a complete
// *BibleData once, let loadVersionData cache it, and the rest of the app
// (reading, search, share, navigation) needs no per-version code. What is
// DIFFERENT from the public-domain sources is the compliance posture around
// that cache — see licensedRecencyWindow in versions.go: API.Bible's terms
// (§11, 3 Aug 2026) require stored content to be re-checked at least every 30
// days, so a licensed cache is revalidated rather than served forever.
//
// Endpoint shape (rest.api.bible, verified against docs.api.bible and live
// probes 2026-08-11):
//
//	GET /v1/bibles/{bibleId}/books                          — the canon
//	GET /v1/bibles/{bibleId}/books/{bookId}/chapters        — chapter ids
//	GET /v1/bibles/{bibleId}/passages/{rangeId}
//	    ?content-type=json&include-titles=false&...         — ≤200 verses/call
//	GET /v1/bibles/{bibleId}/chapters/{chapterId}?…         — fallback path
//	header: api-key: <key>
//
// The Starter plan bills per call (5,000/MONTH shared across every install),
// so the fetch walks the PASSAGES endpoint in ≤200-verse ranges: ~200 calls
// for the whole canon versus ~1,190 chapter-by-chapter (the per-chapter path
// survives as an automatic fallback for providers without passages support).
// A public rollout still needs the streamed per-chapter mode and waits on
// API.Bible's answers about offline storage (see the support@api.bible
// enquiry).
//
// FUMS: API.Bible's usage tracker is required for web apps only — "if you only
// use API.Bible for your mobile app … you can skip this section" — and the web
// reader never carries licensed ids (share_link falls back to a public-domain
// version), so no FUMS integration is needed here.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
)

// apiBibleBaseURL is a var so tests can point the client at a local server.
var apiBibleBaseURL = "https://rest.api.bible/v1"

const (
	// apiBibleRequestTimeout bounds ONE request; the overall fetch is bounded
	// by apiBibleFetchBudget. Generous because first-run downloads already sit
	// behind the version-switch spinner (the old per-chapter WEB walk took
	// minutes and the UX was built for it).
	apiBibleRequestTimeout = 30 * time.Second
	apiBibleFetchBudget    = 15 * time.Minute
	// apiBibleConcurrency keeps the chapter fan-out polite. Four workers turn
	// ~1,200 sequential round-trips into a couple of minutes without hammering
	// the service (whose monthly quota is the real limit, not throughput).
	apiBibleConcurrency = 4
)

// usfmCanonical66 is the Protestant canon in canonical order, keyed by USFM
// book id — the order BibleData.Books must carry. Names resolve through
// usfmToCatholicName (catholic.go), whose 66-book subset matches the app's
// canon names exactly; the deuterocanon entries are simply never referenced.
var usfmCanonical66 = []string{
	"GEN", "EXO", "LEV", "NUM", "DEU", "JOS", "JDG", "RUT", "1SA", "2SA",
	"1KI", "2KI", "1CH", "2CH", "EZR", "NEH", "EST", "JOB", "PSA", "PRO",
	"ECC", "SNG", "ISA", "JER", "LAM", "EZK", "DAN", "HOS", "JOL", "AMO",
	"OBA", "JON", "MIC", "NAM", "HAB", "ZEP", "HAG", "ZEC", "MAL",
	"MAT", "MRK", "LUK", "JHN", "ACT", "ROM", "1CO", "2CO", "GAL", "EPH",
	"PHP", "COL", "1TH", "2TH", "1TI", "2TI", "TIT", "PHM", "HEB", "JAS",
	"1PE", "2PE", "1JN", "2JN", "3JN", "JUD", "REV",
}

// --- Wire shapes -------------------------------------------------------------

type apiBibleBooksResponse struct {
	Data []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Chapters []struct {
			ID     string `json:"id"`
			Number string `json:"number"`
		} `json:"chapters"`
	} `json:"data"`
}

type apiBibleChaptersResponse struct {
	Data []struct {
		ID     string `json:"id"`
		Number string `json:"number"`
	} `json:"data"`
}

type apiBibleChapterResponse struct {
	Data struct {
		ID      string          `json:"id"`
		BookID  string          `json:"bookId"`
		Number  string          `json:"number"`
		Content json.RawMessage `json:"content"`
	} `json:"data"`
}

type apiBiblePassageResponse struct {
	Data struct {
		ID         string          `json:"id"` // the range actually served, e.g. "GEN.8.17-GEN.17.2"
		VerseCount int             `json:"verseCount"`
		Copyright  string          `json:"copyright"` // the rights holder's line, per response
		Content    json.RawMessage `json:"content"`
	} `json:"data"`
}

// apiBibleCopyrightSeen counts the distinct copyright lines the responses of a
// fetch carried. The app credits a licensed edition with ONE pinned notice
// (BibleVersion.LicenseNotice), which is only honest while every response
// carries the same line; the live full-canon test asserts that it does, so a
// provider that starts varying the line per book is caught before the notice
// is wrong anywhere.
var (
	apiBibleCopyrightMu   sync.Mutex
	apiBibleCopyrightSeen = map[string]int{}
)

func apiBibleNoteCopyright(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	apiBibleCopyrightMu.Lock()
	apiBibleCopyrightSeen[line]++
	apiBibleCopyrightMu.Unlock()
}

// apiBiblePassageCap is the API's per-passage verse limit (verified live: a
// GEN.1-GEN.50 request serves exactly 200 verses and reports where it
// stopped). The range walk leans on it: a chunk under the cap means the
// requested range is exhausted.
const apiBiblePassageCap = 200

// apiBibleContentQuery is the shared content-shaping query for chapter and
// passage bodies.
// include-notes=true since the footnotes machinery landed: notes ride the
// SAME requests (zero extra quota) and are captured side-band by the walk —
// never into verse text. Live-probed 2026-08-26: the NKJV feed carries ONLY
// cross-reference notes (USX style "x"); the print edition's NU-/M-Text
// apparatus is not in the feed at all. See docs/FOOTNOTES.md.
// include-titles=true because API.Bible counts the Psalm superscription (the
// "d" paragraph) as a title: with titles off it is simply absent from the
// feed (probed live: Psalm 3 opened at verse 1). Titles on also brings the
// publisher's section headings ("s": "The Beatitudes") and the acrostic
// letters of Psalm 119 ("qa"), which apiBibleSkipPara drops on purpose —
// see docs/SOURCE_FIELDS.md for what each source carries and what is kept.
// A var only so the live full-canon test can fetch the titles-off feed and
// prove the flag changes nothing but the titles.
var apiBibleContentQuery = "content-type=json&include-titles=true&include-notes=true&include-chapter-numbers=false"

// apiBibleStatusError reports a non-retryable HTTP status. The passage walk
// depends on distinguishing 400/404 (range past the book's real end, or a
// provider without the passages endpoint) from everything else.
type apiBibleStatusError struct {
	Status int
	Path   string
}

func (e *apiBibleStatusError) Error() string {
	return fmt.Sprintf("HTTP %d from API.Bible for %s", e.Status, e.Path)
}

// apiBibleCallCount counts real requests issued, for quota accounting in
// logs and tests (the Starter plan bills per call).
var apiBibleCallCount atomic.Int64

// apiBibleNode is one node of the content-type=json tree: paragraph blocks at
// the top, then a mix of tag nodes (verse markers, char spans — which NEST)
// and text nodes. Decoded defensively: fields we don't recognise are ignored,
// and a chapter that yields no keyed verse text fails loudly rather than
// producing a silently empty chapter.
type apiBibleNode struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Text  string `json:"text"`
	Attrs struct {
		Style   string `json:"style"`
		Number  string `json:"number"`
		VerseID string `json:"verseId"`
		Caller  string `json:"caller"`
		SID     string `json:"sid"`
		ID      string `json:"id"` // a ref tag's target ("JHN.7.50"); a note's own id otherwise
	} `json:"attrs"`
	Items []apiBibleNode `json:"items"`
}

// --- Fetch -------------------------------------------------------------------

// fetchAPIBible downloads the complete translation from API.Bible and
// assembles it into the app's BibleData shape.
func fetchAPIBible(displayName, providerBibleID, apiKey string) (*BibleData, error) {
	if providerBibleID == "" || apiKey == "" {
		return nil, fmt.Errorf("%s: API.Bible source missing bible id or key", displayName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiBibleFetchBudget)
	defer cancel()
	client := newHTTPClient()
	client.Timeout = apiBibleRequestTimeout

	// One call for the canon, with chapter lists piggybacked where the API
	// supports it (include-chapters). Books the provider returns are matched to
	// the canonical 66 by USFM id; extras (intros, apocrypha in other bibles)
	// are ignored, and a missing canonical book fails validation later.
	var books apiBibleBooksResponse
	if err := apiBibleGet(ctx, client, apiKey,
		"/bibles/"+providerBibleID+"/books?include-chapters=true", &books); err != nil {
		return nil, fmt.Errorf("%s: list books: %w", displayName, err)
	}

	byID := map[string]int{}
	for i, b := range books.Data {
		byID[strings.ToUpper(b.ID)] = i
	}
	var plans []apiBibleBookPlan
	for _, usfm := range usfmCanonical66 {
		idx, ok := byID[usfm]
		if !ok {
			return nil, fmt.Errorf("%s: provider canon is missing %s", displayName, usfm)
		}
		b := books.Data[idx]
		chapters := b.Chapters
		if len(chapters) == 0 {
			// include-chapters unsupported or empty — fall back to the
			// per-book chapters listing.
			var cr apiBibleChaptersResponse
			if err := apiBibleGet(ctx, client, apiKey,
				"/bibles/"+providerBibleID+"/books/"+b.ID+"/chapters", &cr); err != nil {
				return nil, fmt.Errorf("%s: list chapters for %s: %w", displayName, usfm, err)
			}
			for _, c := range cr.Data {
				chapters = append(chapters, struct {
					ID     string `json:"id"`
					Number string `json:"number"`
				}{c.ID, c.Number})
			}
		}
		name := apiBibleBookName(usfm)
		if name == "" {
			// A silent "" here once cost a whole book: Esther vanished and
			// Daniel lost its name (the Catholic map only knows ESG/DAG).
			return nil, fmt.Errorf("%s: no app book name for USFM id %s", displayName, usfm)
		}
		plan := apiBibleBookPlan{usfm: usfm, name: name}
		for _, c := range chapters {
			n, err := strconv.Atoi(strings.TrimSpace(c.Number))
			if err != nil {
				continue // "intro" and other non-numeric pseudo-chapters
			}
			plan.chapters = append(plan.chapters, apiBibleChapterRef{id: c.ID, number: n})
			if n > plan.lastChapter {
				plan.lastChapter = n
			}
		}
		if len(plan.chapters) == 0 {
			return nil, fmt.Errorf("%s: provider returned no chapters for %s", displayName, usfm)
		}
		plans = append(plans, plan)
	}

	// Fan the downloads out over a small worker pool, one BOOK per job. Each
	// book walks the passages endpoint in ≤200-verse ranges (~200 calls for
	// the whole canon — the Starter plan bills per call, and the old
	// chapter-by-chapter walk cost ~1,190). A provider without passages
	// support (a 400/404 on a book's FIRST range) falls back to the
	// per-chapter path for that book. First error wins and cancels the rest.
	verses := make(map[string]map[int][]Verse, 66)
	// Guarded by the same mutex as verses: notes anchored in verses this
	// translation omits, keyed book -> chapter, exactly as the helloao
	// decoder assembles them.
	orphans := make(map[string]map[int][]OrphanFootnote)
	supers := map[string]map[int]Superscription{}
	heads := map[string]map[int][]Heading{}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
	)
	fail := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		mu.Unlock()
	}
	sem := make(chan struct{}, apiBibleConcurrency)
	for _, plan := range plans {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(plan apiBibleBookPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			book, bookOrphans, bookSupers, bookHeads, err := fetchAPIBibleBookByPassages(ctx, client, apiKey, providerBibleID, plan)
			var se *apiBibleStatusError
			if errors.As(err, &se) && (se.Status == http.StatusBadRequest || se.Status == http.StatusNotFound) {
				book, bookOrphans, bookSupers, bookHeads, err = fetchAPIBibleBookByChapters(ctx, client, apiKey, providerBibleID, plan)
			}
			if err != nil {
				fail(err)
				return
			}
			mu.Lock()
			verses[plan.name] = book
			if len(bookOrphans) > 0 {
				orphans[plan.name] = bookOrphans
			}
			if len(bookSupers) > 0 {
				supers[plan.name] = bookSupers
			}
			if len(bookHeads) > 0 {
				heads[plan.name] = bookHeads
			}
			mu.Unlock()
		}(plan)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, fmt.Errorf("%s download failed: %w", displayName, firstErr)
	}

	booksOut := make([]string, 0, len(usfmCanonical66))
	for _, usfm := range usfmCanonical66 {
		booksOut = append(booksOut, apiBibleBookName(usfm))
	}
	data := &BibleData{Verses: verses, Books: booksOut}
	if len(orphans) > 0 {
		data.OrphanFootnotes = orphans
	}
	if len(supers) > 0 {
		data.Superscriptions = supers
	}
	if len(heads) > 0 {
		data.Headings = heads
	}
	if err := validateBibleData(data); err != nil {
		return nil, fmt.Errorf("%s: incomplete download: %w", displayName, err)
	}
	return data, nil
}

type apiBibleChapterRef struct {
	id     string
	number int
}

type apiBibleBookPlan struct {
	usfm        string
	name        string
	lastChapter int
	chapters    []apiBibleChapterRef
}

// fetchAPIBibleBookByPassages downloads one book through the passages
// endpoint in ≤apiBiblePassageCap-verse ranges. The API truncates a range
// gracefully and reports the range actually served in data.id, so the walk
// self-paginates: request start→end-of-book, continue from one past the
// served end, stop when a chunk comes back under the cap. A continuation
// start can be invalid in two ways the API answers 400/404 for — one past a
// chapter's last verse (an exactly-cap chunk ending on a chapter boundary),
// or past the book's end — handled by advancing a chapter once, then
// stopping.
func fetchAPIBibleBookByPassages(ctx context.Context, client *http.Client, apiKey, bibleID string, plan apiBibleBookPlan) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, map[int][]Heading, error) {
	out := map[int][]Verse{}
	var orphans map[int][]OrphanFootnote
	var supers map[int]Superscription
	var heads map[int][]Heading
	styleCensus := newAPIBibleStyleCensus(plan.name, log.Printf)
	defer styleCensus.report()
	startCh, startV := 1, 1
	bumpedChapter := false
	for {
		rangeID := fmt.Sprintf("%s.%d.%d-%s.%d", plan.usfm, startCh, startV, plan.usfm, plan.lastChapter)
		var pr apiBiblePassageResponse
		err := apiBibleGet(ctx, client, apiKey,
			"/bibles/"+bibleID+"/passages/"+rangeID+"?"+apiBibleContentQuery, &pr)
		if err != nil {
			var se *apiBibleStatusError
			if errors.As(err, &se) && (se.Status == http.StatusBadRequest || se.Status == http.StatusNotFound) {
				if startCh == 1 && startV == 1 {
					return nil, nil, nil, nil, err // passages unsupported here — caller falls back
				}
				if !bumpedChapter && startCh < plan.lastChapter {
					bumpedChapter = true
					startCh, startV = startCh+1, 1
					continue
				}
				break // past the book's real end — done
			}
			return nil, nil, nil, nil, err
		}
		bumpedChapter = false
		apiBibleNoteCopyright(pr.Data.Copyright)
		chunk, chunkOrphans, chunkSupers, chunkHeads, err := decodeAPIBiblePassageChecked(pr.Data.Content, plan.name, startCh, styleCensus)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("%s passage %s: %w", plan.name, rangeID, err)
		}
		for ch, vs := range chunk {
			out[ch] = append(out[ch], vs...)
		}
		for ch, sup := range chunkSupers {
			if supers == nil {
				supers = make(map[int]Superscription)
			}
			supers[ch] = sup // a re-served title is the same title
		}
		for ch, hs := range chunkHeads {
			if heads == nil {
				heads = make(map[int][]Heading)
			}
			// A chunk boundary can re-serve a chapter's headings; keep the
			// first decode, exactly as sortVersesDedupe keeps the first verse.
			if _, seen := heads[ch]; !seen {
				heads[ch] = hs
			}
		}
		for ch, fns := range chunkOrphans {
			if orphans == nil {
				orphans = make(map[int][]OrphanFootnote)
			}
			orphans[ch] = append(orphans[ch], fns...)
		}
		if pr.Data.VerseCount < apiBiblePassageCap {
			break // the requested range is exhausted
		}
		endCh, endV := chapterVerseFromRef(passageEndRef(pr.Data.ID))
		if endCh == 0 || endV == 0 {
			return nil, nil, nil, nil, fmt.Errorf("%s: unparseable passage range id %q", plan.name, pr.Data.ID)
		}
		// Continue from the served end itself, one verse of OVERLAP, not one
		// past it. A chunk's first verse is by definition the first verse of
		// its first paragraph block, so decoded alone it always looks like a
		// paragraph opener — Romans 8:15 is verse 201 of its book, and the
		// chunk boundary landed on it and invented a paragraph the publisher
		// never set. Re-serving that verse inside the NEXT chunk is harmless
		// because sortVersesDedupe keeps the FIRST decode, which is the one
		// that saw the verse in its real context.
		startCh, startV = endCh, endV
	}
	if len(out) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("%s: passages yielded no verses", plan.name)
	}
	for ch := range out {
		out[ch] = sortVersesDedupe(out[ch])
	}
	return out, orphans, supers, heads, nil
}

// fetchAPIBibleBookByChapters is the chapter-by-chapter path — one request
// per chapter, sequential within the book (books already run in parallel).
// It is the fallback for providers without the passages endpoint.
func fetchAPIBibleBookByChapters(ctx context.Context, client *http.Client, apiKey, bibleID string, plan apiBibleBookPlan) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, map[int][]Heading, error) {
	out := make(map[int][]Verse, len(plan.chapters))
	var orphans map[int][]OrphanFootnote
	var supers map[int]Superscription
	var heads map[int][]Heading
	for _, c := range plan.chapters {
		var cr apiBibleChapterResponse
		if err := apiBibleGet(ctx, client, apiKey,
			"/bibles/"+bibleID+"/chapters/"+c.id+"?"+apiBibleContentQuery, &cr); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("%s %d: %w", plan.name, c.number, err)
		}
		vs, chOrphans, sup, chHeads, err := decodeAPIBibleChapter(cr.Data.Content, plan.name, c.number)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("%s %d: %w", plan.name, c.number, err)
		}
		out[c.number] = vs
		if sup.Text != "" {
			if supers == nil {
				supers = make(map[int]Superscription)
			}
			supers[c.number] = sup
		}
		for ch, fns := range chOrphans {
			if orphans == nil {
				orphans = make(map[int][]OrphanFootnote)
			}
			orphans[ch] = append(orphans[ch], fns...)
		}
		if len(chHeads) > 0 {
			if heads == nil {
				heads = make(map[int][]Heading)
			}
			heads[c.number] = chHeads
		}
	}
	return out, orphans, supers, heads, nil
}

// passageEndRef extracts the end reference of a served range id:
// "GEN.8.17-GEN.17.2" → "GEN.17.2" (a single-verse id passes through whole).
func passageEndRef(id string) string {
	if i := strings.LastIndexByte(id, '-'); i >= 0 {
		return id[i+1:]
	}
	return id
}

// chapterVerseFromRef parses "GEN 8:17", "GEN.8.17" or "PSA 46:11" into
// (chapter, verse); zero values mean the component was absent.
func chapterVerseFromRef(s string) (ch, v int) {
	i := strings.LastIndexAny(s, ":.")
	if i < 0 {
		return 0, 0
	}
	v = leadingInt(s[i+1:])
	rest := s[:i]
	j := strings.LastIndexAny(rest, " .")
	if j < 0 {
		return 0, v
	}
	return leadingInt(rest[j+1:]), v
}

// sortVersesDedupe orders a chapter's verses and drops duplicate verse
// numbers (a normalized continuation overlap keeps the first decode).
func sortVersesDedupe(vs []Verse) []Verse {
	sort.Slice(vs, func(i, j int) bool { return vs[i].Verse < vs[j].Verse })
	out := vs[:0]
	last := -1
	for _, v := range vs {
		if v.Verse == last {
			continue
		}
		last = v.Verse
		out = append(out, v)
	}
	return out
}

// apiBibleGet performs one authenticated GET and decodes the JSON body. One
// polite retry on 5xx and 429 (with a short pause) — quota exhaustion and real
// errors surface immediately rather than being hammered.
func apiBibleGet(ctx context.Context, client *http.Client, apiKey, path string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBibleBaseURL+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("api-key", apiKey)
		req.Header.Set("Accept", "application/json")
		apiBibleCallCount.Add(1)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK:
			if readErr != nil {
				lastErr = readErr
				continue
			}
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("decode %s: %w", path, err)
			}
			return nil
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return fmt.Errorf("API.Bible rejected the key (HTTP %d) — check BIBLE_API_KEY and that this translation is licensed to it", resp.StatusCode)
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d from API.Bible", resp.StatusCode)
			continue
		default:
			// Authenticated response bodies are deliberately omitted from errors.
			// A provider or proxy diagnostic must not be able to echo a credential
			// into application logs.
			return &apiBibleStatusError{Status: resp.StatusCode, Path: path}
		}
	}
	return lastErr
}

// --- Content decoding --------------------------------------------------------

// decodeAPIBibleChapter turns one chapter's content-type=json blocks into its
// verses — a thin wrapper over the passage decoder for the chapter endpoint
// and the tests. The CALLER's chapter is authoritative here: it asked the
// endpoint for exactly one chapter, so embedded references only order the
// verses and the result is re-stamped, preserving the chapter path's
// long-standing contract.
func decodeAPIBibleChapter(raw json.RawMessage, bookName string, chapter int) ([]Verse, map[int][]OrphanFootnote, Superscription, []Heading, error) {
	byChapter, orphans, supers, heads, err := decodeAPIBiblePassage(raw, bookName, chapter)
	if err != nil {
		return nil, nil, Superscription{}, nil, err
	}
	var vs []Verse
	for _, chunk := range byChapter {
		vs = append(vs, chunk...)
	}
	for i := range vs {
		vs[i].Chapter = chapter
	}
	vs = sortVersesDedupe(vs)
	if len(vs) == 0 {
		return nil, nil, Superscription{}, nil, fmt.Errorf("no verse text decoded")
	}
	// One chapter was asked for, so the one title decoded is its title,
	// whatever chapter number the markers carried (vs[i].Chapter is forced
	// above for the same reason).
	sup := supers[chapter]
	if sup.Text == "" {
		for _, s := range supers {
			sup = s
			break
		}
	}
	// One chapter was asked for, so its headings are whichever were decoded.
	var chapterHeads []Heading
	for _, hs := range heads {
		chapterHeads = append(chapterHeads, hs...)
	}
	return vs, orphans, sup, chapterHeads, nil
}

// decodeAPIBiblePassage turns content-type=json paragraph blocks — possibly
// spanning several chapters, as the passages endpoint serves them — into
// verses grouped by chapter. Verse boundaries come from the embedded
// verse-marker tags (attrs.number, with attrs.sid/"verseId" fallbacks, whose
// references also carry the CHAPTER: "GEN 8:17"); text nodes append to the
// current verse. defaultChapter anchors content that arrives before any
// chapter-bearing reference (single-chapter responses, and a range chunk
// resuming mid-chapter). Authored poem lines are preserved the same way the
// helloao decoders do it: each new POETRY paragraph (USFM q styles) that
// continues a verse contributes a "\n" line break, so psalms render as lines
// on every surface (see reading.go verseIsPoetic).
// The second return is the orphan footnotes: notes anchored in verses this
// translation omits, keyed by chapter. A verse node that decodes to no text
// is a critical-text omission — the verse number exists in the versification,
// the translation omits its words, and the note explains the omission. The
// helloao decoder has captured these since the footnotes work landed; this
// path used to discard them with the verse, so a provider translation that
// omitted verses would silently lose exactly the notes the orphan machinery
// exists to keep.
func decodeAPIBiblePassage(raw json.RawMessage, bookName string, defaultChapter int) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, map[int][]Heading, error) {
	return decodeAPIBiblePassageChecked(raw, bookName, defaultChapter, nil)
}

// decodeAPIBiblePassageChecked is decodeAPIBiblePassage with the fetch's style
// census. The census only counts; it never changes what is decoded.
func decodeAPIBiblePassageChecked(raw json.RawMessage, bookName string, defaultChapter int, cen *apiBibleStyleCensus) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, map[int][]Heading, error) {
	if len(raw) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("empty chapter content")
	}
	var blocks []apiBibleNode
	if err := json.Unmarshal(raw, &blocks); err != nil {
		// A string here means the API answered with html/text content —
		// somebody changed the query or the API changed shape. Fail loudly.
		return nil, nil, nil, nil, fmt.Errorf("unexpected chapter content shape (want json blocks): %w", err)
	}

	// Verses are keyed by a packed (chapter, verse) int — real chapter and
	// verse numbers never reach 1000, and saneRef drops anything that would
	// overflow the packing (an absurd marker number is ignored, so its text
	// accrues to the previous verse rather than keying garbage).
	saneRef := func(n int) int {
		if n < 1 || n > 999 {
			return 0
		}
		return n
	}
	pack := func(ch, v int) int { return ch*1000 + v }
	order := []int{}
	texts := map[int]*strings.Builder{}
	pendingNotes := map[int][]Footnote{} // per packed (ch,v), in sentinel order
	currentCh := defaultChapter
	current := 0 // current verse number; 0 = before the first verse

	// A Psalm's superscription ("A Psalm of David when he fled from Absalom
	// his son") is the "d" paragraph, and it arrives BEFORE the chapter's
	// first verse marker — so while it is being read, currentCh can still
	// name the previous chapter of a multi-chapter passage. The title is
	// held and attached at the next verse marker, whose sid names the
	// chapter it opens. Notes inside a title ride the same sentinel scheme
	// as verse notes.
	// paraStarts holds the packed (chapter, verse) keys of verses that OPEN a
	// paragraph in the publisher's setting. A prose block (p, m, pi, pc, nb …)
	// is a paragraph; a q block is a LINE inside one, so only a non-poetry
	// block opens one — marking q blocks would make every line of a psalm its
	// own paragraph. The block's first verse marker is the opener; a block
	// that begins mid-verse has its boundary inside that verse, which this
	// model cannot express, so the break lands at the following verse instead
	// of a few words earlier, and a block carrying no marker at all adds none.
	paraStarts := map[int]bool{}
	blockOpens := false

	// The indent depth of each line, per verse. A q block carries its own
	// depth in its style name — q1 opens a Hebrew couplet, q2 answers it — and
	// the app read only that the block was poetry at all, so every line drew
	// flush left. One entry per line of the finished verse, zero for a line
	// that is not poetry.
	poemLevels := map[int][]int{}
	curPoemLevel := 0

	// The publisher's headings. Their text is not Scripture and never enters a
	// verse, but it is the translators' own map of the chapter and it is kept
	// (BibleData.Headings). Read through the same walk as a title so char
	// spans behave identically; attached, like a title, at the next verse
	// marker, because on the passages endpoint a heading can be read while the
	// decoder is still inside the previous chapter.
	headings := map[int][]Heading{}
	var pendingHeads []Heading
	inHeading := false
	var headBuf *strings.Builder
	var headNotes []Footnote

	supers := map[int]Superscription{}
	var titleBuf *strings.Builder // non-nil while a title is pending
	var titleNotes []Footnote
	titleIDCh := 0   // the chapter the title's own verseId attrs name, if any
	inTitle := false // walking a "d" paragraph: text goes to the title
	sawVerse := false
	finishTitle := func(ch int) {
		if titleBuf == nil {
			return
		}
		text, anchors := stripFootnoteSentinels(normalizeVerseSpaces(titleBuf.String()))
		notes := titleNotes
		if len(anchors) != len(notes) {
			notes = nil // see the same guard on verses below
		} else {
			for i := range notes {
				notes[i].Anchor = anchors[i]
			}
		}
		if text != "" && saneRef(ch) != 0 {
			supers[ch] = Superscription{Text: text, Footnotes: notes}
		}
		titleBuf, titleNotes, titleIDCh = nil, nil, 0
	}

	// Fragments are concatenated RAW: the source's text nodes carry their own
	// spacing ("The " + sc"Lord" + " said to my Lord,"), and any inserted
	// space corrupts constructions where a span abuts punctuation ("Lord" +
	// ";") or splits a word ("G" + sc"OD" → "G OD" — the live NKJV caught
	// both). The only synthetic joins are the poetry "\n" and a single space
	// where a PROSE verse flows across a paragraph boundary.
	pendingBreak := false // next text for the current verse starts a poem line
	pendingSpace := false // next text for the current verse crosses a prose para join
	appendText := func(key, from int, s string) {
		if key%1000 == 0 || s == "" {
			return
		}
		b, ok := texts[key]
		if !ok {
			b = &strings.Builder{}
			texts[key] = b
			order = append(order, key)
		}
		// The first text of a verse opens its first line; a poem break opens
		// another. Recorded here rather than after the fact, because only this
		// loop knows which block a line came from.
		if _, seen := poemLevels[key]; !seen {
			poemLevels[key] = []int{curPoemLevel}
		} else if key == from && pendingBreak {
			built := withoutSentinels(b.String())
			if built != "" && !strings.HasSuffix(built, "\n") {
				poemLevels[key] = append(poemLevels[key], curPoemLevel)
			}
		}
		if key == from {
			// The join is decided on what the reader will see. NO sentinel is
			// text: a verse that opens with a note, or whose first words are
			// marked as supplied, has no words yet and takes no break or
			// space, exactly as if the marker were absent. A whole poetry
			// canon once gained a blank first line on every verse whose
			// cross-reference sits at its start, and the supplied-word
			// brackets would have done the same to another forty-one.
			cur := withoutSentinels(b.String())
			if pendingBreak {
				if cur != "" && !strings.HasSuffix(cur, "\n") {
					b.WriteByte('\n')
				}
			} else if pendingSpace && cur != "" &&
				!strings.HasSuffix(cur, " ") && !strings.HasSuffix(cur, "\n") &&
				!strings.HasPrefix(s, " ") {
				b.WriteByte(' ')
			}
		}
		pendingBreak, pendingSpace = false, false
		b.WriteString(s)
	}

	var walk func(nodes []apiBibleNode)
	walk = func(nodes []apiBibleNode) {
		for _, n := range nodes {
			switch {
			case n.Type == "text" || (n.Text != "" && len(n.Items) == 0):
				s := n.Text
				if inHeading {
					headBuf.WriteString(s)
					continue
				}
				if inTitle {
					// The title's own words, never a verse's. When the
					// provider stamps them with a verseId, that names the
					// chapter the title belongs to.
					if idCh, _ := chapterVerseFromRef(n.Attrs.VerseID); saneRef(idCh) != 0 && titleIDCh == 0 {
						titleIDCh = idCh
					}
					titleBuf.WriteString(s)
					continue
				}
				ch, v := currentCh, current
				if idCh, idV := chapterVerseFromRef(n.Attrs.VerseID); saneRef(idV) != 0 {
					v = idV
					if saneRef(idCh) != 0 {
						ch = idCh
					}
				}
				appendText(pack(ch, v), pack(currentCh, current), s)
			case n.Name == "note":
				// A note. Its children are the translators' words (xt/ft/fq
				// spans) — the default case below would walk them straight
				// into the verse builders, apparatus read as Scripture, the
				// one thing this decoder must never do. Instead the subtree
				// is diverted into the side-band footnote store: a sentinel
				// rune marks the spot in the verse builder (resolved to a
				// rune anchor after normalizeVerseSpaces, then stripped —
				// stripFootnoteSentinels), and the flattened body rides
				// beside it. The sentinel is written DIRECTLY, bypassing
				// appendText, so a pending poem break or paragraph space is
				// left for the next real text exactly as if the note were
				// absent — which is what keeps the text byte-identical.
				fn, has := apiBibleFootnote(n)
				if has && inHeading {
					// A note inside a heading belongs to the heading. It used
					// to be discarded with the block; before that it would
					// have attached itself to whatever verse was current,
					// which is a different verse from the one the heading
					// stands above.
					headNotes = append(headNotes, fn)
				} else if has && inTitle {
					titleBuf.WriteRune(footnoteSentinel)
					titleNotes = append(titleNotes, fn)
				} else if has {
					key := pack(currentCh, current)
					if key%1000 != 0 {
						b, ok := texts[key]
						if !ok {
							b = &strings.Builder{}
							texts[key] = b
							order = append(order, key)
						}
						b.WriteRune(footnoteSentinel)
						pendingNotes[key] = append(pendingNotes[key], fn)
					}
				}
			case n.Name == "verse":
				if sidCh, _ := chapterVerseFromRef(n.Attrs.SID); saneRef(sidCh) != 0 {
					currentCh = sidCh
				}
				if num := saneRef(verseNumFromMarker(n)); num != 0 {
					current = num
				}
				sawVerse = true
				if blockOpens {
					if key := pack(currentCh, current); key%1000 != 0 {
						paraStarts[key] = true
					}
					blockOpens = false
				}
				if len(pendingHeads) > 0 && saneRef(current) != 0 {
					for i := range pendingHeads {
						pendingHeads[i].BeforeVerse = current
					}
					headings[currentCh] = append(headings[currentCh], pendingHeads...)
					pendingHeads = nil
				}
				// A title read before this marker belongs to the chapter
				// the marker opens.
				finishTitle(currentCh)
				// The marker's own items render the verse NUMBER ("10"), not
				// verse text — the live NKJV proved it arrives as a nested
				// text node. The app draws its own verse numbers, so the
				// marker subtree is presentation only: never walk it.
			default:
				// Char spans (sc, nd, wj, it, …) and anything else that
				// nests. Two of them are the edition's own typography, and
				// they are kept AS SPANS rather than folded into the letters:
				// the words it sets in italic because the translators
				// supplied them, and the small capitals it sets the divine
				// name in. Both are bracketed in place and resolved to rune
				// offsets once the text has settled (stripSentinels), so
				// nothing is added to the verse and nothing taken from it.
				style := strings.ToLower(n.Attrs.Style)
				cen.char(style)
				if openRune, closeRune, marked := spanSentinels(style); marked && !inTitle && !inHeading {
					bracket := func(r rune) {
						key := pack(currentCh, current)
						if key%1000 == 0 {
							return
						}
						b, ok := texts[key]
						if !ok {
							b = &strings.Builder{}
							texts[key] = b
							order = append(order, key)
						}
						b.WriteRune(r)
					}
					bracket(openRune)
					walk(n.Items)
					bracket(closeRune)
					continue
				}
				walk(n.Items)
			}
		}
	}

	for _, block := range blocks {
		style := strings.ToLower(block.Attrs.Style)
		cen.para(style, apiBibleBlockHasText(block.Items))
		if style == "d" {
			// The superscription: Scripture's own title for the Psalm, kept
			// beside the chapter (BibleData.Superscriptions) and never in a
			// verse — where the helloao decoders put it (bsb.go). It is read
			// through the same walk so its char spans and notes are handled
			// exactly as a verse's are; which chapter it belongs to is
			// settled at the next verse marker (finishTitle).
			if titleBuf == nil {
				titleBuf = &strings.Builder{}
			} else if titleBuf.Len() > 0 {
				titleBuf.WriteByte(' ')
			}
			inTitle = true
			walk(block.Items)
			inTitle = false
			continue
		}
		if apiBibleSkipPara(style) {
			// Headings are not scripture: acrostic letters (qa — which the
			// "q" poetry prefix would otherwise claim), section heads
			// (s*/ms*/mr/sr/r/sp/cl/cd). include-titles=false does NOT strip
			// qa, so Psalm 119's א/Aleph headings once leaked into verse text.
			//
			// The TEXT is dropped; the POSITION is not. In print a heading
			// always begins a new unit, and in an acrostic each letter marks a
			// stanza — Psalm 119's twenty-two of them are the psalm's whole
			// structure. This feed sends no blank-line instruction at all, so
			// without this a chapter of poetry arrives with nothing at all to
			// break it.
			blockOpens = true
			headBuf = &strings.Builder{}
			inHeading = true
			walk(block.Items)
			inHeading = false
			if text := strings.TrimSpace(normalizeVerseSpaces(headBuf.String())); text != "" {
				pendingHeads = append(pendingHeads, Heading{Text: text, Style: style, Footnotes: headNotes})
			}
			headBuf, headNotes = nil, nil
			continue
		}
		isPoetry := strings.HasPrefix(style, "q")
		curPoemLevel = 0
		if isPoetry {
			// "q1" → 1, "q2" → 2; a bare "q" is the shallowest depth.
			curPoemLevel = 1
			if n := leadingInt(style[1:]); n > 0 {
				curPoemLevel = n
			}
		}
		// A prose block opens a paragraph. A q block is a LINE inside one, so
		// it opens nothing of its own — but it must not CLEAR a break a
		// skipped heading just set, or an acrostic letter followed by its
		// first poetry line would lose the stanza it marks.
		blockOpens = blockOpens || !isPoetry
		// A paragraph boundary continues the current verse. For poetry that
		// boundary is an authored line; for prose it is just flow.
		if current != 0 {
			if isPoetry {
				pendingBreak = true
			} else {
				pendingSpace = true
			}
		}
		walk(block.Items)
		pendingBreak, pendingSpace = false, false
	}
	// A title still pending when the blocks run out had no verse after it,
	// which only a passage chunk's tail can do. It belongs to whichever
	// chapter its own verseId names; with no verseId it is dropped rather
	// than guessed — after any verse it can only be the NEXT chapter's, and
	// before any verse there is nothing to attach it to.
	if titleBuf != nil {
		if titleIDCh != 0 {
			finishTitle(titleIDCh)
		} else if !sawVerse {
			finishTitle(currentCh)
		} else {
			titleBuf, titleNotes, titleIDCh = nil, nil, 0
		}
	}

	if len(order) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no verse text decoded")
	}
	sort.Ints(order)
	out := map[int][]Verse{}
	var orphans map[int][]OrphanFootnote
	total := 0
	for _, key := range order {
		text, anchors, supplied, smallCaps := stripSentinels(normalizeVerseSpaces(texts[key].String()))
		if text == "" {
			// An omitted verse: the key exists because the provider sent the
			// verse's markup, but it decodes to no words. Keep any note that
			// was anchored in it as an orphan, keyed by the verse it belongs
			// to, so the chapter-bottom section can still say why the number
			// is absent. Anchors are meaningless without text and are not
			// carried; bodies stay in the side-band exactly as verse notes
			// do — never in Verse.Text, and so never in search, share or
			// speech, which all walk Verses.
			ch, num := key/1000, key%1000
			for _, n := range pendingNotes[key] {
				if strings.TrimSpace(n.Text) == "" {
					continue
				}
				if orphans == nil {
					orphans = make(map[int][]OrphanFootnote)
				}
				orphans[ch] = append(orphans[ch], OrphanFootnote{
					Verse:  num,
					Text:   strings.TrimSpace(n.Text),
					Kind:   n.Kind,
					Caller: n.Caller,
				})
			}
			continue
		}
		notes := pendingNotes[key]
		if len(anchors) != len(notes) {
			// A sentinel went missing or multiplied — impossible by design
			// (normalizeVerseSpaces never drops non-space runes), so treat it
			// as corruption and ship the verse without notes rather than
			// mis-anchored ones.
			notes = nil
		} else {
			for i := range notes {
				notes[i].Anchor = anchors[i]
			}
		}
		ch, num := key/1000, key%1000
		out[ch] = append(out[ch], Verse{
			BookName:  bookName,
			Book:      bookName,
			Chapter:   ch,
			Verse:     num,
			Text:      text,
			Footnotes: notes,
			ParaStart: paraStarts[key],
			Supplied:  supplied,
			SmallCaps: smallCaps,
			// Only when the depths describe THIS text: a verse whose lines
			// were rebuilt by normalisation says nothing rather than
			// something that no longer fits.
			PoemLevels: describedLevels(poemLevels[key], text),
		})
		total++
	}
	if total == 0 {
		return nil, nil, nil, nil, fmt.Errorf("no verse text decoded")
	}
	return out, orphans, supers, headings, nil
}

// apiBibleSkipPara reports whether a paragraph style carries headings rather
// than scripture text: acrostic letters (qa), section heads and
// cross-reference lines. The superscription (d) is NOT here — it is read into
// the chapter's title by decodeAPIBiblePassage. Matched exactly — prefix tests would claim
// scripture styles (a bare "s" prefix eats "sp"-adjacent poetry, and "q"
// already claims qa for poetry, which is exactly how Psalm 119's headings
// once leaked).
func apiBibleSkipPara(style string) bool {
	switch style {
	case "qa", "cl", "cd", "mr", "sr", "r", "sp",
		"s", "s1", "s2", "s3", "s4", "ms", "ms1", "ms2", "ms3":
		return true
	}
	return false
}

// footnoteSentinel marks a footnote's in-text position while a verse is being
// assembled. U+E000 (private use) cannot occur in scripture text; it survives
// normalizeVerseSpaces untouched (Fields never drops non-space runes) and is
// stripped — recording rune anchors — before the text leaves the decoder.
const footnoteSentinel = '\uE000'

// suppliedOpen and suppliedClose bracket a span of words the TRANSLATORS
// SUPPLIED — the italics of the King James tradition, marking what was added
// for English sense and stands in no Hebrew or Greek word. The feed sends them
// as char spans (style "it"); they are bracketed here for the same reason a
// footnote is marked with a sentinel, because the text is still being
// assembled and normalised, and a rune offset taken now would not survive.
// Both are private-use runes that cannot occur in scripture and that
// normalizeVerseSpaces leaves alone.
const (
	suppliedOpen  = '\uE001'
	suppliedClose = '\uE002'
)

// smallCapsOpen and smallCapsClose bracket a span the edition sets in SMALL
// CAPITALS — the divine name above all. They are bracketed rather than applied,
// for the same reason the supplied words are: the text is still being assembled
// and normalised, so a rune offset taken now would not survive.
//
// The app used to uppercase these spans into the text instead. That put
// characters in the reader's hands that the publisher never sent, and it threw
// away the position, so nothing downstream could set the words properly.
const (
	smallCapsOpen  = '\uE003'
	smallCapsClose = '\uE004'
	// The brackets around a cited passage inside a NOTE's text (apiBibleNote).
	// They never enter a verse builder: a note's text is assembled on its own
	// and stripped before it is stored, so withoutSentinels need not know them.
	noteRefOpen  = '\uE005'
	noteRefClose = '\uE006'
)

// withoutSentinels removes every marker from a part-assembled verse so the
// JOIN can be decided on what the reader will actually see. It exists as one
// function because the decision has been got wrong twice by adding a sentinel
// and forgetting one of the places that has to ignore it: a whole poetry canon
// once gained a blank first line on every verse whose cross-reference sits at
// its start.
func withoutSentinels(s string) string {
	return sentinelStripper.Replace(s)
}

var sentinelStripper = strings.NewReplacer(
	string(footnoteSentinel), "",
	string(suppliedOpen), "",
	string(suppliedClose), "",
	string(smallCapsOpen), "",
	string(smallCapsClose), "",
)

// spanSentinels reports the brackets for a char span the app keeps as a SPAN
// rather than as characters: "it" is the words the translators supplied, and
// "sc"/"nd" are the small capitals an edition sets the divine name in. Every
// other style nests through unmarked.
func spanSentinels(style string) (openRune, closeRune rune, marked bool) {
	switch style {
	case "it":
		return suppliedOpen, suppliedClose, true
	case "sc", "nd":
		return smallCapsOpen, smallCapsClose, true
	}
	return 0, 0, false
}

// stripFootnoteSentinels removes every sentinel from s, returning the clean
// text and the rune offset each sentinel occupied. A sentinel standing alone
// between two spaces takes the following space with it, so the text reads as
// if the marker had never been there.
func stripFootnoteSentinels(s string) (string, []int) {
	text, anchors, _, _ := stripSentinels(s)
	return text, anchors
}

// stripSentinels removes every sentinel in ONE pass and reports what each
// marked, in offsets into the text that is left. One pass rather than two,
// because each kind shifts the other's offsets: a footnote stripped after an
// italic range was measured would leave that range pointing a rune or two past
// where its words now sit.
//
// A footnote sentinel standing alone between two spaces takes the following
// space with it, so the text reads as if the marker had never been there. A
// supplied-word bracket never owns a space: it sits tight against the words it
// marks.
func stripSentinels(s string) (string, []int, []TextSpan, []TextSpan) {
	if !strings.ContainsAny(s, string([]rune{footnoteSentinel,
		suppliedOpen, suppliedClose, smallCapsOpen, smallCapsClose})) {
		return s, nil, nil, nil
	}
	runes := []rune(s)
	var b strings.Builder
	var anchors []int
	var supplied, smallCaps []TextSpan
	var open, capsOpen []int
	emitted := 0
	lastEmitted := rune(0)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case footnoteSentinel:
			anchors = append(anchors, emitted)
			if i+1 < len(runes) && runes[i+1] == ' ' &&
				(emitted == 0 || lastEmitted == ' ' || lastEmitted == '\n') {
				i++ // the sentinel owned this space; dropping both avoids a double
			}
		case suppliedOpen:
			open = append(open, emitted)
		case suppliedClose:
			// An unmatched close is dropped rather than guessed at: the feed
			// has never sent one, and inventing a start would mark words the
			// publisher did not.
			if n := len(open); n > 0 {
				start := open[n-1]
				open = open[:n-1]
				if emitted > start {
					supplied = append(supplied, TextSpan{Start: start, End: emitted})
				}
			}
		case smallCapsOpen:
			capsOpen = append(capsOpen, emitted)
		case smallCapsClose:
			// Unmatched closes are dropped for the same reason.
			if n := len(capsOpen); n > 0 {
				start := capsOpen[n-1]
				capsOpen = capsOpen[:n-1]
				if emitted > start {
					smallCaps = append(smallCaps, TextSpan{Start: start, End: emitted})
				}
			}
		default:
			b.WriteRune(r)
			lastEmitted = r
			emitted++
		}
	}
	sort.Slice(supplied, func(i, j int) bool { return supplied[i].Start < supplied[j].Start })
	sort.Slice(smallCaps, func(i, j int) bool { return smallCaps[i].Start < smallCaps[j].Start })
	return b.String(), anchors, supplied, smallCaps
}

// describedLevels returns the per-line indent depths only when they describe
// the text they are meant for, and only when at least one line is poetry.
func describedLevels(levels []int, text string) []int {
	if len(levels) != strings.Count(text, "\n")+1 {
		return nil
	}
	for _, n := range levels {
		if n > 0 {
			return levels
		}
	}
	return nil
}

// apiBibleNoteKind maps a USX note style to the app's Footnote.Kind: "x"/"ex"
// are cross-reference apparatus, everything else ("f", "fe", …) is a
// translator footnote.
func apiBibleNoteKind(style string) string {
	switch strings.ToLower(style) {
	case "x", "ex":
		return footnoteKindCrossref
	}
	return ""
}

// apiBibleNoteBody flattens a note subtree into its display text: every text
// node in document order — including the words inside ref tags — EXCEPT the
// origin reference spans (xo for cross-references, fr for footnotes), which
// restate the verse the note belongs to; the anchor already carries that. The
// citations' ids are dropped here; apiBibleNote keeps them.
func apiBibleNoteBody(n apiBibleNode) string {
	text, _ := apiBibleNote(n)
	return text
}

// apiBibleNote is apiBibleNoteBody with the citations kept: each ref tag's
// id, with the rune span its words occupy in the finished text. The ids are
// bracketed in place while the text is assembled and resolved once it has
// settled — the discipline the supplied-word and small-capital spans follow
// (spanSentinels) — so the text is the same whether or not the ids are kept,
// and a reader who copies a note receives the publisher's own words.
// Fragments concatenate raw (they hold their own spacing) and the result is
// space-normalized exactly as apiBibleNoteBody always normalized it, with the
// brackets invisible to that normalization (normalizeNoteText), so keeping
// the ids cannot move a space.
func apiBibleNote(n apiBibleNode) (string, []NoteRef) {
	var b strings.Builder
	var ids []string
	var walk func(nodes []apiBibleNode)
	walk = func(nodes []apiBibleNode) {
		for _, c := range nodes {
			style := strings.ToLower(c.Attrs.Style)
			if c.Name == "char" && (style == "xo" || style == "fr") {
				continue
			}
			id := strings.TrimSpace(c.Attrs.ID)
			cited := c.Name == "ref" && id != ""
			if cited {
				ids = append(ids, id)
				b.WriteRune(noteRefOpen)
			}
			if c.Text != "" {
				b.WriteString(c.Text)
			}
			walk(c.Items)
			if cited {
				b.WriteRune(noteRefClose)
			}
		}
	}
	walk(n.Items)
	return normalizeNoteText(b.String(), ids)
}

// normalizeNoteText is the note flattening's whitespace rule — every run of
// whitespace becomes one space and the ends are trimmed, which is what
// strings.Join(strings.Fields(s), " ") does — applied while the citation
// brackets are lifted out, so that a bracket standing beside a space can
// neither keep the space nor become one. It reports each bracketed
// citation's span in offsets into the text that is left. Ids pair with
// brackets in opening order, so a citation nested inside another (which no
// feed sends, but which costs nothing to get right) resolves to its own id. A
// span starts at its first word and ends after its last; a citation with no
// words — an id and nothing to tap — is dropped, and the text keeps nothing
// of it either way.
func normalizeNoteText(raw string, ids []string) (string, []NoteRef) {
	type opened struct{ start, id int } // start is -1 until the first word lands
	var stack []opened
	var out []rune
	var refs []NoteRef
	seen := 0
	pendingSpace := false
	for _, r := range raw {
		switch {
		case r == noteRefOpen:
			stack = append(stack, opened{start: -1, id: seen})
			seen++
		case r == noteRefClose:
			if len(stack) == 0 {
				continue // a stray close: nothing was opened for it
			}
			o := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if o.start >= 0 && o.start < len(out) && o.id < len(ids) {
				refs = append(refs, NoteRef{ID: ids[o.id], Start: o.start, End: len(out)})
			}
		case unicode.IsSpace(r):
			pendingSpace = len(out) > 0
		default:
			if pendingSpace {
				out = append(out, ' ')
				pendingSpace = false
			}
			for i := range stack {
				if stack[i].start < 0 {
					stack[i].start = len(out)
				}
			}
			out = append(out, r)
		}
	}
	// Refs are appended as their brackets CLOSE; a nested citation closes
	// before its parent, so put them back into text order.
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Start < refs[j].Start })
	return string(out), refs
}

// apiBibleFootnote is the side-band note for one note node; has is false when
// the note carries no words. One constructor, so a note inside a title or a
// heading keeps its citations exactly as a verse's does.
func apiBibleFootnote(n apiBibleNode) (Footnote, bool) {
	body, refs := apiBibleNote(n)
	if body == "" {
		return Footnote{}, false
	}
	return Footnote{
		Text:   body,
		Kind:   apiBibleNoteKind(n.Attrs.Style),
		Caller: strings.TrimSpace(n.Attrs.Caller),
		Refs:   refs,
	}, true
}

// normalizeVerseSpaces trims each poem line and collapses interior space runs
// (raw fragment concatenation can double a space when two neighbouring nodes
// both carry one), preserving the "\n" poem-line structure.
func normalizeVerseSpaces(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, ln := range lines {
		ln = strings.Join(strings.Fields(ln), " ")
		if ln != "" {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// apiBibleBookName resolves a standard USFM book id to the app's book name.
// usfmToCatholicName serves helloao's CATHOLIC edition, which carries the
// Greek Esther and Daniel under ESG/DAG — the plain EST/DAN ids every
// standard 66-book canon uses are absent from that map, so they are named
// here. A lookup miss is a hard error in fetchAPIBible, never a silent "".
func apiBibleBookName(usfm string) string {
	switch usfm {
	case "EST":
		return "Esther"
	case "DAN":
		return "Daniel"
	}
	return usfmToCatholicName[usfm]
}

// verseNumFromMarker extracts the verse number from a verse-marker tag:
// attrs.number first, then the sid — which separates chapter from verse with
// a colon ("PSA 46:11") or a dot depending on the source. Ranges ("17-18")
// take the first number, matching how the app keys verses.
func verseNumFromMarker(n apiBibleNode) int {
	if v := leadingInt(n.Attrs.Number); v != 0 {
		return v
	}
	if sid := n.Attrs.SID; sid != "" {
		if i := strings.LastIndexAny(sid, ":."); i >= 0 {
			return leadingInt(sid[i+1:])
		}
	}
	return 0
}

// verseNumFromID parses the trailing verse number out of a text node's
// verseId ("PSA.46.10" → 10).
func verseNumFromID(id string) int {
	if id == "" {
		return 0
	}
	if i := strings.LastIndexByte(id, '.'); i >= 0 {
		return leadingInt(id[i+1:])
	}
	return 0
}

func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}
