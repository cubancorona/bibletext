package bibletext

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveAPIBibleProbe validates the decoder against the real API.Bible
// service — the fixture tests pin the de-facto JSON shape, but only a live
// response proves we read it right. Deliberately tiny: one metadata call plus
// five content calls (~6 of the Starter plan's 5,000 monthly requests), never
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
	verses, _, _, err := decodeAPIBibleChapter(psalm.Data.Content, "Psalms", 46)
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
	jv, _, _, err := decodeAPIBibleChapter(john.Data.Content, "John", 11)
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
	// The Psalm titles. The "d" paragraph must come back as the chapter's
	// superscription and never as verse text.
	var ps3 apiBibleChapterResponse
	if err := apiBibleGet(ctx, client, key, "/bibles/"+bibleID+"/chapters/PSA.3?"+apiBibleContentQuery, &ps3); err != nil {
		t.Fatalf("PSA.3: %v", err)
	}
	p3, _, sup3, err := decodeAPIBibleChapter(ps3.Data.Content, "Psalms", 3)
	if err != nil {
		t.Fatalf("decode PSA.3: %v", err)
	}
	t.Logf("PSA.3 title: %q (%d notes)", sup3.Text, len(sup3.Footnotes))
	if !strings.Contains(sup3.Text, "Absalom") {
		t.Errorf("PSA.3 superscription = %q, want the flight from Absalom", sup3.Text)
	}
	if len(p3) > 0 && strings.Contains(p3[0].Text, "Absalom") {
		t.Errorf("PSA.3 verse 1 carries the title: %q", p3[0].Text)
	}

	// The passages endpoint is the download's real route, and there a title
	// sits BETWEEN chapters: one range across the 3→4 boundary, and one that
	// starts at 4:1, to learn whether a range beginning at a chapter's first
	// verse still carries the paragraph before it.
	for _, rng := range []string{"PSA.3.1-PSA.4.8", "PSA.4.1-PSA.4.8"} {
		var pr apiBiblePassageResponse
		if err := apiBibleGet(ctx, client, key, "/bibles/"+bibleID+"/passages/"+rng+"?"+apiBibleContentQuery, &pr); err != nil {
			t.Fatalf("%s: %v", rng, err)
		}
		head := string(pr.Data.Content)
		if len(head) > 240 {
			head = head[:240]
		}
		t.Logf("%s opens: %s", rng, head)
		_, _, supers, err := decodeAPIBiblePassage(pr.Data.Content, "Psalms", 3)
		if err != nil {
			t.Fatalf("decode %s: %v", rng, err)
		}
		for ch, s := range supers {
			t.Logf("%s: chapter %d title %q (%d notes)", rng, ch, s.Text, len(s.Footnotes))
		}
		if rng == "PSA.3.1-PSA.4.8" && (supers[3].Text == "" || supers[4].Text == "") {
			t.Errorf("%s: titles for both chapters expected, got %v", rng, supers)
		}
	}

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

// TestLiveAPIBibleFullCanon downloads the whole NKJV exactly as the app does
// (fetchAPIBible: ~200 passage requests of the plan's 5,000 a month) and
// checks it against the figures of the canon verified on 2026-08-11, so a
// decoder change that alters verse text anywhere in 31,102 verses shows up
// as a number. Twice gated — the key AND BIBLETEXT_LIVE_FULL_CANON=1 — so the
// cheap probe above never triggers it. BIBLETEXT_FULL_CANON_OUT=<path> keeps
// the decoded BibleData, which is what seeds a device's cache without a
// second download.
//
//	BIBLE_API_KEY=… BIBLETEXT_LIVE_FULL_CANON=1 go test -run TestLiveAPIBibleFullCanon -v -timeout 20m .
func TestLiveAPIBibleFullCanon(t *testing.T) {
	key := os.Getenv("BIBLE_API_KEY")
	if key == "" || os.Getenv("BIBLETEXT_LIVE_FULL_CANON") != "1" {
		t.Skip("BIBLE_API_KEY and BIBLETEXT_LIVE_FULL_CANON=1 not both set — full-canon fetch skipped")
	}
	bibleID := os.Getenv("BIBLETEXT_PROVIDER_ID_NKJV")
	if bibleID == "" {
		bibleID = "63097d2a0a2f7db3-01"
	}
	started := time.Now()
	data, err := fetchAPIBible("NKJV", bibleID, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fetched in %s", time.Since(started).Round(time.Second))

	verses, poetry, lord, notes := 0, 0, 0, 0
	for book, chapters := range data.Verses {
		for _, vs := range chapters {
			for _, v := range vs {
				verses++
				if strings.Contains(v.Text, "\n") {
					poetry++
				}
				if strings.Contains(v.Text, "LORD") {
					lord++
				}
				notes += len(v.Footnotes)
				if strings.ContainsRune(v.Text, footnoteSentinel) {
					t.Errorf("%s %d:%d carries a sentinel", book, v.Chapter, v.Verse)
				}
			}
		}
	}
	t.Logf("books %d, verses %d, poetry %d, LORD %d, notes %d", len(data.Verses), verses, poetry, lord, notes)
	for _, tc := range []struct {
		name      string
		got, want int
	}{
		{"books", len(data.Verses), 66},
		{"verses", verses, 31102},
		{"poetry verses", poetry, 8349},
		{"verses naming the LORD", lord, 5622},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d (the canon of 2026-08-23)", tc.name, tc.got, tc.want)
		}
	}

	// BIBLETEXT_FULL_CANON_COMPARE=<path>: an earlier decoded canon — a
	// BibleData JSON, or the app's own cache file with its envelope — whose
	// verse text this download must reproduce byte for byte. Notes and
	// titles may be new; words may not.
	if ref := os.Getenv("BIBLETEXT_FULL_CANON_COMPARE"); ref != "" {
		raw, err := os.ReadFile(ref)
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(raw, &envelope) == nil && len(envelope.Data) > 0 {
			raw = envelope.Data
		}
		var before BibleData
		if err := json.Unmarshal(raw, &before); err != nil {
			t.Fatalf("%s: %v", ref, err)
		}
		diffs := 0
		for book, chapters := range data.Verses {
			for ch, vs := range chapters {
				was := before.Verses[book][ch]
				if len(was) != len(vs) {
					t.Errorf("%s %d: %d verses now, %d before", book, ch, len(vs), len(was))
					continue
				}
				for i := range vs {
					if vs[i].Text != was[i].Text {
						diffs++
						if diffs <= 12 {
							t.Errorf("%s %d:%d changed\n now:    %q\n before: %q", book, ch, vs[i].Verse, vs[i].Text, was[i].Text)
						}
					}
				}
			}
		}
		t.Logf("against %s: %d verses changed", ref, diffs)
		if diffs != 0 {
			t.Errorf("%d verses differ from the earlier canon", diffs)
		}
	}

	// The titles, and only the titles: 116 Psalms carry one in the NKJV.
	supers := data.Superscriptions["Psalms"]
	var untitled []int
	for ch := 1; ch <= 150; ch++ {
		s, ok := supers[ch]
		if !ok {
			untitled = append(untitled, ch)
			continue
		}
		if s.Text == "" || s.Text != strings.TrimSpace(s.Text) || strings.ContainsRune(s.Text, footnoteSentinel) {
			t.Errorf("Psalm %d title malformed: %q", ch, s.Text)
		}
	}
	wantUntitled := []int{1, 2, 10, 33, 43, 71, 91, 93, 94, 95, 96, 97, 99, 104, 105, 106, 107,
		111, 112, 113, 114, 115, 116, 117, 118, 119, 135, 136, 137, 146, 147, 148, 149, 150}
	if fmt.Sprint(untitled) != fmt.Sprint(wantUntitled) {
		t.Errorf("untitled Psalms = %v\n want %v", untitled, wantUntitled)
	}
	for book, m := range data.Superscriptions {
		if book != "Psalms" {
			t.Errorf("%s carries %d titles; only the Psalms have them", book, len(m))
		}
	}
	t.Logf("Psalm 3 title: %q (%d notes)", supers[3].Text, len(supers[3].Footnotes))

	// Heading text never reaches a verse.
	first := func(book string, ch int) string {
		for _, v := range data.Verses[book][ch] {
			if v.Verse == 1 {
				return v.Text
			}
		}
		return ""
	}
	if got := first("Psalms", 3); !strings.HasPrefix(got, "LORD, how they have increased") {
		t.Errorf("Psalm 3:1 = %q", got)
	}
	if got := first("Psalms", 119); strings.Contains(got, "Aleph") {
		t.Errorf("Psalm 119:1 carries the acrostic heading: %q", got)
	}
	if got := first("Matthew", 5); strings.Contains(got, "Beatitudes") {
		t.Errorf("Matthew 5:1 carries the section heading: %q", got)
	}

	if os.Getenv("BIBLETEXT_LIVE_FULL_CANON_AB") == "1" {
		// The same decoder over the titles-off feed: the query flag must
		// change nothing but the titles. Another ~200 requests.
		saved := apiBibleContentQuery
		apiBibleContentQuery = strings.Replace(saved, "include-titles=true", "include-titles=false", 1)
		off, err := fetchAPIBible("NKJV", bibleID, key)
		apiBibleContentQuery = saved
		if err != nil {
			t.Fatal(err)
		}
		if len(off.Superscriptions) != 0 {
			t.Errorf("the titles-off feed yielded titles in %d books", len(off.Superscriptions))
		}
		diffs := 0
		for book, chapters := range data.Verses {
			for ch, vs := range chapters {
				offVs := off.Verses[book][ch]
				if len(offVs) != len(vs) {
					t.Errorf("%s %d: %d verses with titles, %d without", book, ch, len(vs), len(offVs))
					continue
				}
				for i := range vs {
					if vs[i].Text != offVs[i].Text || fmt.Sprint(vs[i].Footnotes) != fmt.Sprint(offVs[i].Footnotes) {
						diffs++
						if diffs <= 12 {
							t.Errorf("%s %d:%d differs\n titles on:  %q\n titles off: %q", book, ch, vs[i].Verse, vs[i].Text, offVs[i].Text)
						}
					}
				}
			}
		}
		t.Logf("titles on/off: %d differing verses", diffs)
		if diffs != 0 {
			t.Errorf("the titles flag changed %d verses", diffs)
		}
	}

	if out := os.Getenv("BIBLETEXT_FULL_CANON_OUT"); out != "" {
		b, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, b, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d bytes to %s", len(b), out)
	}
}
