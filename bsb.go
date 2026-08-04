package bibletext

// Berean Standard Bible (BSB) source.
//
// The BSB is a modern translation dedicated to the PUBLIC DOMAIN (CC0) by BSB
// Publishing on 2023-04-30, so no license is required to ship its text — it is a
// real, selectable version like the WEB, not one of the licensed/evaluation
// entries. bible-api.com (the WEB's source) does not carry the BSB, so it has its
// own source here: the free, key-less "Free Use Bible API" at bible.helloao.org,
// which serves the whole translation as a single JSON document. That one ~7 MB
// fetch is decoded into BibleData and cached like any other version (see
// loadVersionData / cachePathForVersion). The cache filename carries the version's
// cacheEpoch so decoder fixes re-decode existing installs instead of being masked
// by stale, already-flattened caches.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// bsbCompleteURL is helloao's whole-translation endpoint for the BSB: one request
// for all 66 books / 1189 chapters (vs. the WEB's per-chapter fetch loop).
const bsbCompleteURL = "https://bible.helloao.org/api/BSB/complete.json"

// bsbSource serves the public-domain Berean Standard Bible from bible.helloao.org.
type bsbSource struct{}

func (bsbSource) available() bool { return true }

func (bsbSource) fetch() (*BibleData, error) {
	// 120s timeout: the whole translation is one ~7 MB body, so this must cover a
	// slow connection's full download, not a per-chapter request.
	return fetchHelloAOComplete("BSB", bsbCompleteURL, &http.Client{Timeout: 120 * time.Second}, decodeCanonical66)
}

// webCompleteURL is helloao's whole-translation endpoint for the 66-book World English
// Bible (Protestant — the "P"). One request for the entire Bible, vs. bible-api.com's
// ~1189 chapter-by-chapter, rate-limited requests that often never finished, leaving a
// first-run reader stuck on the embedded Gospels seed. The WEB is public domain.
const webCompleteURL = "https://bible.helloao.org/api/ENGWEBP/complete.json"

// fetchWEBFromHelloAO downloads the complete World English Bible from helloao in ONE
// request, decoded by the same path as the BSB (decodeBSBComplete maps a 66-book helloao
// complete.json by canonical book order). It backs webSource (versions.go).
func fetchWEBFromHelloAO() (*BibleData, error) {
	return fetchHelloAOComplete("WEB", webCompleteURL, &http.Client{Timeout: 120 * time.Second}, decodeCanonical66)
}

// decodeCanonical66 decodes a 66-book helloao complete.json (BSB, WEB) by canonical book
// order. The Catholic edition has 73 books in a different arrangement, so it uses
// decodeHelloAOCatholic (id-based) — see catholic.go.
func decodeCanonical66(body []byte) (*BibleData, error) {
	return decodeBSBComplete(body, NewBibleData().Books)
}

// fetchHelloAOComplete fetches one of helloao's whole-translation complete.json bodies
// and decodes it with the given decoder. Shared by the BSB, WEB and WEB-Catholic
// sources; label only flavours the error messages. validateBibleData checks against the
// DECODED book list, so it adapts to each edition's canon (66 vs. 73).
func fetchHelloAOComplete(label, url string, client httpClient, decode func([]byte) (*BibleData, error)) (*BibleData, error) {
	body, err := fetchWithRetry(client, url, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", label, err)
	}
	bd, err := decode(body)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	// Guard against a truncated/partial parse silently caching an incomplete Bible:
	// every book in the decoded list must have come through with chapters and verses.
	if err := validateBibleData(bd); err != nil {
		return nil, fmt.Errorf("%s data incomplete: %w", label, err)
	}
	return bd, nil
}

// helloAOBook mirrors one book in a bible.helloao.org complete.json. Shared by the
// order-based 66-book decoder (decodeBSBComplete) and the id-based Catholic decoder
// (decodeHelloAOCatholic, catholic.go).
type helloAOBook struct {
	ID       string `json:"id"`
	Order    int    `json:"order"`
	Chapters []struct {
		Chapter struct {
			Number  int               `json:"number"`
			Content []json.RawMessage `json:"content"`
		} `json:"chapter"`
	} `json:"chapters"`
}

// decodeHelloAOChapters turns one helloao book's chapters into the app's chapter→[]Verse
// map under book. Each chapter's `content` is a flat array of typed nodes; only `verse`
// nodes carry reader text (bsbVerseText retains their internal line breaks). Chapter-level
// line breaks, headings, and Hebrew subtitles (Psalm superscriptions like "A Psalm of
// David") are editorial nodes outside verse text and remain omitted. Shared by both
// decoders.
func decodeHelloAOChapters(book string, b helloAOBook) map[int][]Verse {
	chapters := make(map[int][]Verse, len(b.Chapters))
	for _, cj := range b.Chapters {
		num := cj.Chapter.Number
		var verses []Verse
		for _, node := range cj.Chapter.Content {
			var head struct {
				Type    string            `json:"type"`
				Number  int               `json:"number"`
				Content []json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(node, &head); err != nil || head.Type != "verse" {
				continue
			}
			text := bsbVerseText(head.Content)
			if text == "" {
				continue
			}
			verses = append(verses, Verse{
				BookName: book,
				Book:     book,
				Chapter:  num,
				Verse:    head.Number,
				Text:     text,
			})
		}
		if len(verses) > 0 {
			chapters[num] = verses
		}
	}
	return chapters
}

// decodeBSBComplete maps a 66-book bible.helloao.org complete.json into a BibleData
// using the app's canonical book NAMES. helloao identifies books by a USFM code and a
// canonical `order` (1=Genesis … 66=Revelation); appBooks is that same canonical
// sequence, so books are matched by order — the decoded data therefore carries the app's
// own book names, keeping navigation, search, caching and reading-state aligned. Backs
// the BSB and (Protestant) WEB; the Catholic edition maps by id (decodeHelloAOCatholic).
func decodeBSBComplete(body []byte, appBooks []string) (*BibleData, error) {
	var doc struct {
		Books []helloAOBook `json:"books"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(doc.Books) == 0 {
		return nil, fmt.Errorf("no books in response")
	}

	bd := &BibleData{
		Verses: make(map[string]map[int][]Verse, len(appBooks)),
		Books:  append([]string(nil), appBooks...),
	}
	for _, b := range doc.Books {
		if b.Order < 1 || b.Order > len(appBooks) {
			continue // outside the canonical 66 (not expected for the BSB/WEB)
		}
		book := appBooks[b.Order-1]
		if chapters := decodeHelloAOChapters(book, b); len(chapters) > 0 {
			bd.Verses[book] = chapters
		}
	}
	// Note: PrepareSearchIndex is left to the caller (loadBibleData), matching
	// FetchBibleFromAPI — the index is built once after caching.
	return bd, nil
}

// bsbVerseText turns one verse's content array into reader text. Elements are
// either plain strings or objects; objects with a "text" field (poetry lines,
// descriptive text) contribute that text, footnote markers ({"noteId":N})
// contribute nothing, and source-authored line breaks ({"lineBreak":true}) are
// retained. Reading surfaces may lay the verse out as prose, but the share-text
// formatter can therefore recover poetry lines without mistaking visual wrapping
// for source structure.
//
// helloao TRIMS the whitespace around every boundary it introduces — both the
// dropped footnote nodes and the poetry/descriptive clauses carry no surrounding
// spaces. So prose runs on either side of a footnote abut with nothing between
// them: "...Eve," + {noteId} + "because..." would render as "Eve,because" if
// concatenated verbatim. Every such boundary sits at a word boundary (verified
// across the whole translation: there are no mid-word footnote splits), so the
// right join is a single space between contributing pieces. An explicit line-break
// node instead becomes a newline. The only exception is when the next piece begins
// with closing
// punctuation or a quote ("...egg" + {noteId} + "?" → "egg?", "...heel." +
// {noteId} + "”" → "heel.”"); bsbTidySpacing strips those — and any space that
// lands just after an opening bracket/quote — after the fact, which is always safe
// because English never spaces before closing or after opening punctuation.
func bsbVerseText(content []json.RawMessage) string {
	var pieces []string
	for _, node := range content {
		var s string
		if err := json.Unmarshal(node, &s); err == nil {
			if s != "" {
				pieces = append(pieces, s)
			}
			continue
		}
		var obj struct {
			Text      *string         `json:"text"`
			LineBreak bool            `json:"lineBreak"`
			Poem      json.RawMessage `json:"poem"`
		}
		if err := json.Unmarshal(node, &obj); err == nil {
			switch {
			case obj.Text != nil && *obj.Text != "":
				// Real helloao poetry carries NO lineBreak nodes — each
				// {"text","poem":N} clause IS one source line (verified against
				// live captures: Gen 1:27 = three clauses/three lines, Ps 23:2 =
				// two, all without a single lineBreak). So a poem clause after
				// prior content starts a new line — EXCEPT a clause that begins
				// with closing punctuation (Job 6:6 ends with a bare "?" clause),
				// which belongs to the previous line and must abut. An explicit
				// lineBreak (prose lists like Gen 10:2) already broke the line,
				// so no second break is added after one.
				isPoem := len(obj.Poem) > 0 && string(obj.Poem) != "null"
				prevBreak := len(pieces) > 0 && pieces[len(pieces)-1] == "\n"
				if isPoem && len(pieces) > 0 && !prevBreak && !startsWithClosingPunct(*obj.Text) {
					pieces = append(pieces, "\n")
				}
				pieces = append(pieces, *obj.Text)
			case obj.LineBreak:
				pieces = append(pieces, "\n")
			}
			continue
		}
		// noteId: contributes nothing.
	}
	return bsbTidySpacing(strings.Join(pieces, " "))
}

// Spacing artifacts that survive the synthesized-space join: a space before
// closing punctuation/quotes, or after an opening bracket/quote. Removing them is
// always safe — neither ever takes an adjacent space in English prose.
var (
	bsbSpaceBeforeClose = regexp.MustCompile(`\s+([,.;:!?)\]}’”])`)
	bsbSpaceAfterOpen   = regexp.MustCompile(`([(\[{“‘])\s+`)
)

// startsWithClosingPunct reports a piece that opens with punctuation belonging
// to the PREVIOUS clause ("?" / "”" / ")." …) — it must join that line, never
// begin a new one; bsbTidySpacing then merges the space away.
func startsWithClosingPunct(s string) bool {
	for _, r := range s {
		switch r {
		case ')', ']', '}', '”', '’', '!', '?', ';', ':', ',', '.':
			return true
		}
		return false
	}
	return false
}

// bsbTidySpacing collapses redundant whitespace WITHIN each source line and
// removes spaces that the per-piece join wrongly placed adjacent to punctuation.
// Newlines themselves survive for the share-text formatter.
func bsbTidySpacing(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		line = bsbSpaceBeforeClose.ReplaceAllString(line, "$1")
		lines[i] = bsbSpaceAfterOpen.ReplaceAllString(line, "$1")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
