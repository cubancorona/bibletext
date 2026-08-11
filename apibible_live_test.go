package bibletext

import (
	"context"
	"net/http"
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
	client := &http.Client{Timeout: apiBibleRequestTimeout}

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
	verses, err := decodeAPIBibleChapter(psalm.Data.Content, "Psalms", 46)
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
	jv, err := decodeAPIBibleChapter(john.Data.Content, "John", 11)
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
