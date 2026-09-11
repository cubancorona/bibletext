package main

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// The raw-feed cache is keyed by the app's decoder epoch, so a decoder change
// refetches the feed instead of decoding a copy taken under an older
// understanding of it. Mutation: the cache named by id alone again.
func TestRawFeedCacheIsKeyedByTheDecoderEpoch(t *testing.T) {
	for _, v := range publishedVersions() {
		epoch := bibletext.VersionCacheEpoch(v.ID)
		if epoch <= 0 {
			t.Fatalf("%s: the app reports no decoder epoch, so the key below is meaningless", v.ID)
		}
		name := cacheFileName(v)
		if !strings.Contains(name, "-v") || !strings.HasSuffix(name, ".json") || name == v.ID+".json" {
			t.Errorf("%s: cache file %q is not keyed by the epoch", v.ID, name)
		}
	}
}

// An edition whose feed carries the publisher's headings must not build from
// a feed that yields none. Mutation: the guard removed from checkDecoded.
func TestAnEditionWithHeadingsRefusesAFeedWithout(t *testing.T) {
	var bsb webVersion
	for _, v := range publishedVersions() {
		if v.ID == "bsb" {
			bsb = v
		}
	}
	if !bsb.headings {
		t.Fatal("the Berean Standard Bible is not marked as carrying headings; this test guards nothing")
	}
	empty := bibletext.NewBibleData()
	empty.Books = []string{"Matthew"}
	if err := checkDecoded(bsb, empty); err == nil || !strings.Contains(err.Error(), "heading") {
		t.Errorf("a headingless BSB decode was accepted (err=%v)", err)
	}
	empty.Headings = map[string]map[int][]bibletext.Heading{"Matthew": {5: {{Text: "The Beatitudes", BeforeVerse: 3}}}}
	if err := checkDecoded(bsb, empty); err != nil {
		t.Errorf("a BSB decode with a heading was refused: %v", err)
	}
	// The control: an edition not marked for headings passes without any.
	plain := bsb
	plain.headings = false
	plain.ID = "web"
	if err := checkDecoded(plain, &bibletext.BibleData{}); err == nil || !strings.Contains(err.Error(), "no books") {
		t.Errorf("the no-books guard did not fire (%v)", err)
	}
}
