package bibletext

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestLiveAPIBibleProbe validates the decoder against the real API.Bible
// service — the fixture tests pin the de-facto JSON shape, but only a live
// response proves we read it right. Deliberately tiny: one metadata call plus
// two chapter calls (~3 of the Starter plan's 5,000 monthly requests), never
// the full ~1,255-call fetch. Skips unless BIBLE_API_KEY is exported into the
// test environment, so CI and plain `go test` spend nothing.
//
//	BIBLE_API_KEY=… go test -run TestLiveAPIBibleProbe -v .
func TestLiveAPIBibleProbe(t *testing.T) {
	key := os.Getenv("BIBLE_API_KEY")
	if key == "" {
		t.Skip("BIBLE_API_KEY not set — live probe skipped")
	}
	bibleID := os.Getenv("BIBLETEXT_PROVIDER_ID_NKJV")
	if bibleID == "" {
		bibleID = "63097d2a0a2f7db3-01" // NKJV per the API.Bible catalogue
	}
	ctx, cancel := context.WithTimeout(context.Background(), apiBibleRequestTimeout)
	defer cancel()
	client := newHTTPClient()
	client.Timeout = apiBibleRequestTimeout

	var meta struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := apiBibleGet(ctx, client, key, "/bibles/"+bibleID, &meta); err != nil {
		t.Fatalf("bible metadata: %v", err)
	}
	t.Logf("bible: %q (%s)", meta.Data.Name, meta.Data.ID)
	if !strings.Contains(strings.ToLower(meta.Data.Name), "king james") {
		t.Errorf("expected an NKJV bible, got %q — check BIBLETEXT_PROVIDER_ID_NKJV", meta.Data.Name)
	}

	// Poetry: Psalm 46 must decode with authored line breaks and the familiar
	// "Be still" in verse 10.
	var psalm apiBibleChapterResponse
	if err := apiBibleGet(ctx, client, key, "/bibles/"+bibleID+"/chapters/PSA.46?content-type=json", &psalm); err != nil {
		t.Fatalf("PSA.46: %v", err)
	}
	verses, _, err := decodeAPIBibleChapter(psalm.Data.Content, "Psalms", 46)
	if err != nil {
		t.Fatalf("decode PSA.46: %v", err)
	}
	if len(verses) < 11 {
		t.Fatalf("PSA.46: got %d verses, want >= 11", len(verses))
	}
	byNum := map[int]string{}
	poetic := false
	for _, v := range verses {
		byNum[v.Verse] = v.Text
		if strings.Contains(v.Text, "\n") {
			poetic = true
		}
	}
	if !strings.HasPrefix(byNum[10], "Be still") {
		t.Errorf("PSA.46:10 = %q, want it to BEGIN with \"Be still\" — a leading digit means the marker's rendered verse number leaked into the text", byNum[10])
	}
	if !poetic {
		t.Error("PSA.46 decoded with no poem line breaks — poetry paragraphs not being detected")
	}
	t.Logf("PSA.46:10 = %q", byNum[10])

	// Prose: John 11 exercises char spans (red-letter wj) inside paragraphs;
	// verse 35 is the canary.
	var john apiBibleChapterResponse
	if err := apiBibleGet(ctx, client, key, "/bibles/"+bibleID+"/chapters/JHN.11?content-type=json", &john); err != nil {
		t.Fatalf("JHN.11: %v", err)
	}
	jv, _, err := decodeAPIBibleChapter(john.Data.Content, "John", 11)
	if err != nil {
		t.Fatalf("decode JHN.11: %v", err)
	}
	if len(jv) < 57 {
		t.Fatalf("JHN.11: got %d verses, want >= 57", len(jv))
	}
	var v35 string
	for _, v := range jv {
		if v.Verse == 35 {
			v35 = v.Text
		}
	}
	if v35 != "Jesus wept." {
		t.Errorf("JHN.11:35 = %q, want exactly \"Jesus wept.\"", v35)
	}
	t.Logf("JHN.11:35 = %q", v35)
}

// TestLiveAPIBibleFullFetch runs the REAL full-canon fetch through the
// passages range walk, reports the true call count (the reason the walk
// exists: ~200 calls versus ~1,190 chapter-by-chapter against a
// 5,000/month quota), and — when BIBLETEXT_LIVE_COMPARE_CACHE names a cache
// file from the chapter-walk era — proves the two paths produce the
// identical canon, verse by verse. Gated hard: it spends real quota.
//
//	BIBLE_API_KEY=… BIBLETEXT_LIVE_FULL_FETCH=1 \
//	BIBLETEXT_LIVE_COMPARE_CACHE=/path/to/bibletext-nkjv.json \
//	go test -run TestLiveAPIBibleFullFetch -v .
func TestLiveAPIBibleFullFetch(t *testing.T) {
	key := os.Getenv("BIBLE_API_KEY")
	if key == "" || os.Getenv("BIBLETEXT_LIVE_FULL_FETCH") != "1" {
		t.Skip("live full fetch not requested")
	}
	bibleID := os.Getenv("BIBLETEXT_PROVIDER_ID_NKJV")
	if bibleID == "" {
		bibleID = "63097d2a0a2f7db3-01"
	}
	before := apiBibleCallCount.Load()
	data, err := fetchAPIBible("NKJV", bibleID, key)
	if err != nil {
		t.Fatal(err)
	}
	calls := apiBibleCallCount.Load() - before
	total := 0
	for _, chs := range data.Verses {
		for _, vs := range chs {
			total += len(vs)
		}
	}
	t.Logf("full fetch: %d books, %d verses, %d API calls", len(data.Books), total, calls)
	if len(data.Books) != 66 {
		t.Errorf("books = %d", len(data.Books))
	}
	if calls > 300 {
		t.Errorf("passage walk used %d calls — expected ~200", calls)
	}

	cachePath := os.Getenv("BIBLETEXT_LIVE_COMPARE_CACHE")
	if cachePath == "" {
		return
	}
	blob, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("compare cache: %v", err)
	}
	var envelope struct {
		Data *BibleData `json:"data"`
	}
	if err := json.Unmarshal(blob, &envelope); err != nil {
		t.Fatalf("compare cache decode: %v", err)
	}
	diffs := 0
	for book, chs := range envelope.Data.Verses {
		for ch, oldVs := range chs {
			newVs := data.Verses[book][ch]
			if len(newVs) != len(oldVs) {
				t.Errorf("%s %d: %d verses via passages, %d via chapters", book, ch, len(newVs), len(oldVs))
				diffs++
				continue
			}
			for i := range oldVs {
				if newVs[i].Text != oldVs[i].Text || newVs[i].Verse != oldVs[i].Verse {
					if diffs < 10 {
						t.Errorf("%s %d:%d differs:\n passages %q\n chapters %q",
							book, ch, oldVs[i].Verse, newVs[i].Text, oldVs[i].Text)
					}
					diffs++
				}
			}
		}
	}
	if diffs > 0 {
		t.Errorf("%d verse differences between passage and chapter walks", diffs)
	} else {
		t.Log("passage walk output is IDENTICAL to the chapter-walk cache")
	}
}
