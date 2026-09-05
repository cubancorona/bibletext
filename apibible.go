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
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
		Content    json.RawMessage `json:"content"`
	} `json:"data"`
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
			book, bookOrphans, bookSupers, err := fetchAPIBibleBookByPassages(ctx, client, apiKey, providerBibleID, plan)
			var se *apiBibleStatusError
			if errors.As(err, &se) && (se.Status == http.StatusBadRequest || se.Status == http.StatusNotFound) {
				book, bookOrphans, bookSupers, err = fetchAPIBibleBookByChapters(ctx, client, apiKey, providerBibleID, plan)
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
func fetchAPIBibleBookByPassages(ctx context.Context, client *http.Client, apiKey, bibleID string, plan apiBibleBookPlan) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, error) {
	out := map[int][]Verse{}
	var orphans map[int][]OrphanFootnote
	var supers map[int]Superscription
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
					return nil, nil, nil, err // passages unsupported here — caller falls back
				}
				if !bumpedChapter && startCh < plan.lastChapter {
					bumpedChapter = true
					startCh, startV = startCh+1, 1
					continue
				}
				break // past the book's real end — done
			}
			return nil, nil, nil, err
		}
		bumpedChapter = false
		chunk, chunkOrphans, chunkSupers, err := decodeAPIBiblePassage(pr.Data.Content, plan.name, startCh)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s passage %s: %w", plan.name, rangeID, err)
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
			return nil, nil, nil, fmt.Errorf("%s: unparseable passage range id %q", plan.name, pr.Data.ID)
		}
		startCh, startV = endCh, endV+1
	}
	if len(out) == 0 {
		return nil, nil, nil, fmt.Errorf("%s: passages yielded no verses", plan.name)
	}
	for ch := range out {
		out[ch] = sortVersesDedupe(out[ch])
	}
	return out, orphans, supers, nil
}

// fetchAPIBibleBookByChapters is the chapter-by-chapter path — one request
// per chapter, sequential within the book (books already run in parallel).
// It is the fallback for providers without the passages endpoint.
func fetchAPIBibleBookByChapters(ctx context.Context, client *http.Client, apiKey, bibleID string, plan apiBibleBookPlan) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, error) {
	out := make(map[int][]Verse, len(plan.chapters))
	var orphans map[int][]OrphanFootnote
	var supers map[int]Superscription
	for _, c := range plan.chapters {
		var cr apiBibleChapterResponse
		if err := apiBibleGet(ctx, client, apiKey,
			"/bibles/"+bibleID+"/chapters/"+c.id+"?"+apiBibleContentQuery, &cr); err != nil {
			return nil, nil, nil, fmt.Errorf("%s %d: %w", plan.name, c.number, err)
		}
		vs, chOrphans, sup, err := decodeAPIBibleChapter(cr.Data.Content, plan.name, c.number)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("%s %d: %w", plan.name, c.number, err)
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
	}
	return out, orphans, supers, nil
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
func decodeAPIBibleChapter(raw json.RawMessage, bookName string, chapter int) ([]Verse, map[int][]OrphanFootnote, Superscription, error) {
	byChapter, orphans, supers, err := decodeAPIBiblePassage(raw, bookName, chapter)
	if err != nil {
		return nil, nil, Superscription{}, err
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
		return nil, nil, Superscription{}, fmt.Errorf("no verse text decoded")
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
	return vs, orphans, sup, nil
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
func decodeAPIBiblePassage(raw json.RawMessage, bookName string, defaultChapter int) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, error) {
	if len(raw) == 0 {
		return nil, nil, nil, fmt.Errorf("empty chapter content")
	}
	var blocks []apiBibleNode
	if err := json.Unmarshal(raw, &blocks); err != nil {
		// A string here means the API answered with html/text content —
		// somebody changed the query or the API changed shape. Fail loudly.
		return nil, nil, nil, fmt.Errorf("unexpected chapter content shape (want json blocks): %w", err)
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
		if key == from {
			// The join is decided on what the reader will see. A note
			// sentinel is not text: a verse that opens with a note has no
			// words yet and takes no break or space, exactly as if the note
			// were absent (a whole poetry canon once gained a blank first
			// line on every verse whose cross-reference sits at its start).
			cur := strings.ReplaceAll(b.String(), string(footnoteSentinel), "")
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

	var walk func(nodes []apiBibleNode, upcase bool)
	walk = func(nodes []apiBibleNode, upcase bool) {
		for _, n := range nodes {
			switch {
			case n.Type == "text" || (n.Text != "" && len(n.Items) == 0):
				s := n.Text
				if upcase {
					s = strings.ToUpper(s)
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
				body := apiBibleNoteBody(n)
				if body != "" && inTitle {
					titleBuf.WriteRune(footnoteSentinel)
					titleNotes = append(titleNotes, Footnote{
						Text:   body,
						Kind:   apiBibleNoteKind(n.Attrs.Style),
						Caller: strings.TrimSpace(n.Attrs.Caller),
					})
				} else if body != "" {
					key := pack(currentCh, current)
					if key%1000 != 0 {
						b, ok := texts[key]
						if !ok {
							b = &strings.Builder{}
							texts[key] = b
							order = append(order, key)
						}
						b.WriteRune(footnoteSentinel)
						pendingNotes[key] = append(pendingNotes[key], Footnote{
							Text:   body,
							Kind:   apiBibleNoteKind(n.Attrs.Style),
							Caller: strings.TrimSpace(n.Attrs.Caller),
						})
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
				// A title read before this marker belongs to the chapter
				// the marker opens.
				finishTitle(currentCh)
				// The marker's own items render the verse NUMBER ("10"), not
				// verse text — the live NKJV proved it arrives as a nested
				// text node. The app draws its own verse numbers, so the
				// marker subtree is presentation only: never walk it.
			default:
				// Char spans (sc, nd, wj, it, …) and anything else that
				// nests. Small-caps and divine-name spans read as UPPERCASE
				// in plain text — that is what preserves the NKJV's
				// LORD/Lord (YHWH/Adonai) distinction and reassembles
				// "G"+sc"OD" into "GOD".
				style := strings.ToLower(n.Attrs.Style)
				walk(n.Items, upcase || style == "sc" || style == "nd")
			}
		}
	}

	for _, block := range blocks {
		style := strings.ToLower(block.Attrs.Style)
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
			walk(block.Items, false)
			inTitle = false
			continue
		}
		if apiBibleSkipPara(style) {
			// Headings are not scripture: acrostic letters (qa — which the
			// "q" poetry prefix would otherwise claim), section heads
			// (s*/ms*/mr/sr/r/sp/cl/cd). include-titles=false does NOT strip
			// qa, so Psalm 119's א/Aleph headings once leaked into verse text.
			continue
		}
		isPoetry := strings.HasPrefix(style, "q")
		// A paragraph boundary continues the current verse. For poetry that
		// boundary is an authored line; for prose it is just flow.
		if current != 0 {
			if isPoetry {
				pendingBreak = true
			} else {
				pendingSpace = true
			}
		}
		walk(block.Items, false)
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
		return nil, nil, nil, fmt.Errorf("no verse text decoded")
	}
	sort.Ints(order)
	out := map[int][]Verse{}
	var orphans map[int][]OrphanFootnote
	total := 0
	for _, key := range order {
		text, anchors := stripFootnoteSentinels(normalizeVerseSpaces(texts[key].String()))
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
		})
		total++
	}
	if total == 0 {
		return nil, nil, nil, fmt.Errorf("no verse text decoded")
	}
	return out, orphans, supers, nil
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

// stripFootnoteSentinels removes every sentinel from s, returning the clean
// text and the rune offset each sentinel occupied. A sentinel standing alone
// between two spaces takes the following space with it, so the text reads as
// if the marker had never been there.
func stripFootnoteSentinels(s string) (string, []int) {
	if !strings.ContainsRune(s, footnoteSentinel) {
		return s, nil
	}
	runes := []rune(s)
	var b strings.Builder
	var anchors []int
	emitted := 0
	lastEmitted := rune(0)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != footnoteSentinel {
			b.WriteRune(r)
			lastEmitted = r
			emitted++
			continue
		}
		anchors = append(anchors, emitted)
		if i+1 < len(runes) && runes[i+1] == ' ' &&
			(emitted == 0 || lastEmitted == ' ' || lastEmitted == '\n') {
			i++ // the sentinel owned this space; dropping both avoids a double
		}
	}
	return b.String(), anchors
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
// node in document order — including text inside ref tags — EXCEPT the origin
// reference spans (xo for cross-references, fr for footnotes), which restate
// the verse the note belongs to; the anchor already carries that. Fragments
// concatenate raw (they hold their own spacing) and the result is
// space-normalized.
func apiBibleNoteBody(n apiBibleNode) string {
	var b strings.Builder
	var walk func(nodes []apiBibleNode)
	walk = func(nodes []apiBibleNode) {
		for _, c := range nodes {
			style := strings.ToLower(c.Attrs.Style)
			if c.Name == "char" && (style == "xo" || style == "fr") {
				continue
			}
			if c.Text != "" {
				b.WriteString(c.Text)
			}
			walk(c.Items)
		}
	}
	walk(n.Items)
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
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
