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
// Endpoint shape (rest.api.bible, verified against docs.api.bible 2026-08-11):
//
//	GET /v1/bibles/{bibleId}/books                          — the canon
//	GET /v1/bibles/{bibleId}/books/{bookId}/chapters        — chapter ids
//	GET /v1/bibles/{bibleId}/chapters/{chapterId}
//	    ?content-type=json&include-titles=false&...         — verse content
//	header: api-key: <key>
//
// A full fetch is ~1,255 requests. The Starter plan allows 5,000 calls per
// MONTH shared across every install, so this whole-Bible path is for the
// developer's own licensed testing — a public rollout needs either a higher
// tier or the streamed per-chapter mode, and in any case waits on API.Bible's
// answers about offline storage (see the support@api.bible enquiry).
//
// FUMS: API.Bible's usage tracker is required for web apps only — "if you only
// use API.Bible for your mobile app … you can skip this section" — and the web
// reader never carries licensed ids (share_link falls back to a public-domain
// version), so no FUMS integration is needed here.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	client := &http.Client{Timeout: apiBibleRequestTimeout}

	// One call for the canon, with chapter lists piggybacked where the API
	// supports it (include-chapters). Books the provider returns are matched to
	// the canonical 66 by USFM id; extras (intros, apocrypha in other bibles)
	// are ignored, and a missing canonical book fails validation later.
	var books apiBibleBooksResponse
	if err := apiBibleGet(ctx, client, apiKey,
		"/bibles/"+providerBibleID+"/books?include-chapters=true", &books); err != nil {
		return nil, fmt.Errorf("%s: list books: %w", displayName, err)
	}

	type chapterJob struct {
		usfmID    string
		bookName  string
		chapterID string
		number    int
	}
	var jobs []chapterJob
	byID := map[string]int{}
	for i, b := range books.Data {
		byID[strings.ToUpper(b.ID)] = i
	}
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
		for _, c := range chapters {
			n, err := strconv.Atoi(strings.TrimSpace(c.Number))
			if err != nil {
				continue // "intro" and other non-numeric pseudo-chapters
			}
			jobs = append(jobs, chapterJob{usfmID: usfm, bookName: name, chapterID: c.ID, number: n})
		}
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("%s: provider returned no chapters", displayName)
	}

	// Fan the chapter downloads out over a small worker pool; first error wins
	// and cancels the rest.
	verses := make(map[string]map[int][]Verse, 66)
	for _, usfm := range usfmCanonical66 {
		verses[apiBibleBookName(usfm)] = map[int][]Verse{}
	}
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
	for _, job := range jobs {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(job chapterJob) {
			defer wg.Done()
			defer func() { <-sem }()
			var cr apiBibleChapterResponse
			err := apiBibleGet(ctx, client, apiKey,
				"/bibles/"+providerBibleID+"/chapters/"+job.chapterID+
					"?content-type=json&include-titles=false&include-notes=false&include-chapter-numbers=false", &cr)
			if err != nil {
				fail(fmt.Errorf("%s %d: %w", job.bookName, job.number, err))
				return
			}
			vs, err := decodeAPIBibleChapter(cr.Data.Content, job.bookName, job.number)
			if err != nil {
				fail(fmt.Errorf("%s %d: %w", job.bookName, job.number, err))
				return
			}
			mu.Lock()
			verses[job.bookName][job.number] = vs
			mu.Unlock()
		}(job)
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
	if err := validateBibleData(data); err != nil {
		return nil, fmt.Errorf("%s: incomplete download: %w", displayName, err)
	}
	return data, nil
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
			return fmt.Errorf("HTTP %d from API.Bible for %s: %s", resp.StatusCode, path, truncateForError(body))
		}
	}
	return lastErr
}

func truncateForError(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

// --- Content decoding --------------------------------------------------------

// decodeAPIBibleChapter turns content-type=json paragraph blocks into the
// chapter's verses. Verse boundaries come from the embedded verse-marker tags
// (attrs.number, with attrs.sid/"verseId" fallbacks); text nodes append to the
// current verse. Authored poem lines are preserved the same way the helloao
// decoders do it: each new POETRY paragraph (USFM q styles) that continues a
// verse contributes a "\n" line break, so psalms render as lines on every
// surface (see reading.go verseIsPoetic).
func decodeAPIBibleChapter(raw json.RawMessage, bookName string, chapter int) ([]Verse, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty chapter content")
	}
	var blocks []apiBibleNode
	if err := json.Unmarshal(raw, &blocks); err != nil {
		// A string here means the API answered with html/text content —
		// somebody changed the query or the API changed shape. Fail loudly.
		return nil, fmt.Errorf("unexpected chapter content shape (want json blocks): %w", err)
	}

	order := []int{}
	texts := map[int]*strings.Builder{}
	current := 0

	// Fragments are concatenated RAW: the source's text nodes carry their own
	// spacing ("The " + sc"Lord" + " said to my Lord,"), and any inserted
	// space corrupts constructions where a span abuts punctuation ("Lord" +
	// ";") or splits a word ("G" + sc"OD" → "G OD" — the live NKJV caught
	// both). The only synthetic joins are the poetry "\n" and a single space
	// where a PROSE verse flows across a paragraph boundary.
	pendingBreak := false // next text for the current verse starts a poem line
	pendingSpace := false // next text for the current verse crosses a prose para join
	appendText := func(verse, from int, s string) {
		if verse == 0 || s == "" {
			return
		}
		b, ok := texts[verse]
		if !ok {
			b = &strings.Builder{}
			texts[verse] = b
			order = append(order, verse)
		}
		if verse == from {
			cur := b.String()
			if pendingBreak {
				if b.Len() > 0 && !strings.HasSuffix(cur, "\n") {
					b.WriteByte('\n')
				}
			} else if pendingSpace && b.Len() > 0 &&
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
				v := current
				if id := verseNumFromID(n.Attrs.VerseID); id != 0 {
					v = id
				}
				s := n.Text
				if upcase {
					s = strings.ToUpper(s)
				}
				appendText(v, current, s)
			case n.Name == "verse":
				if num := verseNumFromMarker(n); num != 0 {
					current = num
				}
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
		if apiBibleSkipPara(style) {
			// Headings are not scripture: acrostic letters (qa — which the
			// "q" poetry prefix would otherwise claim), superscriptions (d),
			// section heads (s*/ms*/mr/sr/r/sp/cl/cd). include-titles=false
			// does NOT strip qa, so Psalm 119's א/Aleph headings once leaked
			// into verse text.
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

	if len(order) == 0 {
		return nil, fmt.Errorf("no verse text decoded")
	}
	sort.Ints(order)
	out := make([]Verse, 0, len(order))
	for _, num := range order {
		text := normalizeVerseSpaces(texts[num].String())
		if text == "" {
			continue
		}
		out = append(out, Verse{
			BookName: bookName,
			Book:     bookName,
			Chapter:  chapter,
			Verse:    num,
			Text:     text,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no verse text decoded")
	}
	return out, nil
}

// apiBibleSkipPara reports whether a paragraph style carries headings rather
// than scripture text: acrostic letters (qa), superscriptions (d), section
// heads and cross-reference lines. Matched exactly — prefix tests would claim
// scripture styles (a bare "s" prefix eats "sp"-adjacent poetry, and "q"
// already claims qa for poetry, which is exactly how Psalm 119's headings
// once leaked).
func apiBibleSkipPara(style string) bool {
	switch style {
	case "qa", "d", "cl", "cd", "mr", "sr", "r", "sp",
		"s", "s1", "s2", "s3", "s4", "ms", "ms1", "ms2", "ms3":
		return true
	}
	return false
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
