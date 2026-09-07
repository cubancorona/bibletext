package bibletext

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestZZNK(t *testing.T) {
	key := os.Getenv("BIBLE_API_KEY")
	if key == "" {
		t.Skip("no key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiBibleRequestTimeout)
	defer cancel()
	c := newHTTPClient()
	for _, tc := range []struct {
		id, book string
		ch       int
	}{{"PSA.119", "Psalms", 119}, {"AMO.4", "Amos", 4}, {"HOS.11", "Hosea", 11}, {"JHN.3", "John", 3}, {"GEN.1", "Genesis", 1}} {
		var cr apiBibleChapterResponse
		if err := apiBibleGet(ctx, c, key, "/bibles/63097d2a0a2f7db3-01/chapters/"+tc.id+"?"+apiBibleContentQuery, &cr); err != nil {
			t.Fatal(err)
		}
		vs, _, _, err := decodeAPIBibleChapter(cr.Data.Content, tc.book, tc.ch)
		if err != nil {
			t.Fatal(err)
		}
		out := ""
		for _, p := range groupVersesIntoParagraphs(vs) {
			out += fmt.Sprintf("%d ", p[0].Verse)
		}
		t.Logf("%-8s opens at: %s", tc.id, out)
	}
}
