package main

// A licensed edition for the tests to publish, built from SYNTHETIC text in the
// shape API.Bible's NKJV decodes to. No word of the NKJV is in this repository,
// in a test or anywhere else; these fixtures say only "fixture" and the
// reference, and carry the features a real chapter page has to draw.

import (
	"fmt"
	"sort"
	"testing"
	"time"

	bibletext "github.com/cubancorona/bibletext"
)

// fixedRetrieval is the moment the fixtures' fetch "completed": 10:00 in London
// on 2 October 2026, so every page the tests render states that date.
var fixedRetrieval = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

// syntheticLicensedText is an NKJV-shaped edition of ref's canon: exactly the
// verse numbers bibletext.ExpectedVerseNumbers says the NKJV has, so it passes
// the completeness check, with text made by text. The first chapter of every
// book carries a publisher's heading; a test that wants supplied words, small
// capitals or a psalm title adds them to the verses it looks at.
func syntheticLicensedText(t *testing.T, ref *bibletext.BibleData, text func(book string, ch, n int) string) *bibletext.BibleData {
	t.Helper()
	want, err := bibletext.ExpectedVerseNumbers("nkjv", ref)
	if err != nil {
		t.Fatalf("ExpectedVerseNumbers: %v", err)
	}
	bd := &bibletext.BibleData{
		Verses:   map[string]map[int][]bibletext.Verse{},
		Headings: map[string]map[int][]bibletext.Heading{},
	}
	for _, book := range ref.Books {
		chapters, ok := want[book]
		if !ok {
			continue
		}
		bd.Books = append(bd.Books, book)
		bd.Verses[book] = map[int][]bibletext.Verse{}
		nums := make([]int, 0, len(chapters))
		for ch := range chapters {
			nums = append(nums, ch)
		}
		sort.Ints(nums)
		for i, ch := range nums {
			var vs []bibletext.Verse
			for _, n := range chapters[ch] {
				vs = append(vs, bibletext.Verse{BookName: book, Book: book, Chapter: ch, Verse: n, Text: text(book, ch, n)})
			}
			bd.Verses[book][ch] = vs
			if i == 0 {
				bd.Headings[book] = map[int][]bibletext.Heading{
					ch: {{Text: "A Licensed Fixture Heading", Style: "s", BeforeVerse: chapters[ch][0]}},
				}
			}
		}
	}
	return bd
}

// licensedFixtureVersion is the edition above as the generator loads it, with
// the licence the switch attaches: the registry's notice as the site prints it
// and fixedRetrieval.
func licensedFixtureVersion(bd *bibletext.BibleData) loadedVersion {
	notice, err := siteNotice("nkjv", bibletext.VersionLicenseNotice("nkjv"))
	if err != nil {
		panic(err)
	}
	return loadedVersion{
		webVersion: webVersion{ID: "nkjv", Name: "New King James Version", headings: true},
		bible:      bd,
		licence:    &webLicence{Notice: notice, Retrieved: londonDate(fixedRetrieval)},
	}
}

// fixtureVerseText is plain synthetic text naming its reference.
func fixtureVerseText(book string, ch, n int) string {
	return fmt.Sprintf("Licensed fixture verse %d of %s %d.", n, book, ch)
}
