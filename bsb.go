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
// cacheEpoch (bibletext-bsb-v1.json) so a decoder fix re-decodes existing installs
// instead of being masked by a stale cache.

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
	return fetchBSBFromHelloAO(bsbCompleteURL, &http.Client{Timeout: 120 * time.Second})
}

func fetchBSBFromHelloAO(url string, client httpClient) (*BibleData, error) {
	body, err := fetchWithRetry(client, url, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("fetch BSB: %w", err)
	}
	bd, err := decodeBSBComplete(body, NewBibleData().Books)
	if err != nil {
		return nil, fmt.Errorf("decode BSB: %w", err)
	}
	// Guard against a truncated/partial parse silently caching an incomplete BSB:
	// every canonical book must have come through with chapters and verses.
	if err := validateBibleData(bd); err != nil {
		return nil, fmt.Errorf("BSB data incomplete: %w", err)
	}
	return bd, nil
}

// decodeBSBComplete maps bible.helloao.org's complete-translation JSON into a
// BibleData using the app's canonical book NAMES. helloao identifies books by a
// USFM code and a canonical `order` (1=Genesis … 66=Revelation); the app's book
// list (appBooks) is that same canonical sequence, so books are matched by order
// — the decoded data therefore carries the app's own book names, keeping
// navigation, search, caching and reading-state aligned across versions.
//
// Each chapter's `content` is a flat array of typed nodes; only `verse` nodes
// carry reader text. Headings and line breaks are editorial layout the app
// doesn't render (it shows one wrapped block per chapter). Hebrew subtitles
// (Psalm superscriptions like "A Psalm of David") are also dropped — the WEB
// source folds them out of its verse text too, so dropping them here keeps the
// reader consistent across translations.
func decodeBSBComplete(body []byte, appBooks []string) (*BibleData, error) {
	var doc struct {
		Books []struct {
			ID       string `json:"id"`
			Order    int    `json:"order"`
			Chapters []struct {
				Chapter struct {
					Number  int               `json:"number"`
					Content []json.RawMessage `json:"content"`
				} `json:"chapter"`
			} `json:"chapters"`
		} `json:"books"`
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
			continue // outside the canonical 66 (not expected for the BSB)
		}
		book := appBooks[b.Order-1]
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
		if len(chapters) > 0 {
			bd.Verses[book] = chapters
		}
	}
	// Note: PrepareSearchIndex is left to the caller (loadBibleData), matching
	// FetchBibleFromAPI — the index is built once after caching.
	return bd, nil
}

// bsbVerseText flattens one verse's content array into reader text. Elements are
// either plain strings or objects; objects with a "text" field (poetry lines,
// descriptive text) contribute that text, while footnote markers ({"noteId":N})
// and line breaks ({"lineBreak":true}) contribute nothing.
//
// helloao TRIMS the whitespace around every boundary it introduces — both the
// dropped nodes (a footnote or line break that splits a sentence) and the
// poetry/descriptive clauses carry no surrounding spaces. So the runs on either
// side abut with nothing between them: "...Eve," + {noteId} + "because..." and
// "The sons of Japheth:" + {lineBreak} + "Gomer..." would render as "Eve,because"
// and "Japheth:Gomer" if concatenated verbatim. Every such boundary sits at a
// word boundary (verified across the whole translation: there are no mid-word
// footnote splits), so the right join is a single space between every contributing
// piece. The only exception is when the next piece begins with closing
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
			Text *string `json:"text"`
		}
		if err := json.Unmarshal(node, &obj); err == nil && obj.Text != nil {
			if *obj.Text != "" {
				pieces = append(pieces, *obj.Text)
			}
			continue
		}
		// noteId / lineBreak: contribute nothing.
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

// bsbTidySpacing collapses redundant whitespace and removes spaces that the
// per-piece join wrongly placed adjacent to punctuation.
func bsbTidySpacing(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = bsbSpaceBeforeClose.ReplaceAllString(s, "$1")
	s = bsbSpaceAfterOpen.ReplaceAllString(s, "$1")
	return s
}
