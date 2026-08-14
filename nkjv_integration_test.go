package bibletext

// App-level NKJV tests: the licensed translation flowing through the same
// surfaces every public-domain version uses — version switching, share links,
// search, poetry share structure, verse of the day, and TTS speech text. All
// hermetic: "fetching NKJV" hits the apiBibleFixture httptest server
// (apibible_test.go), never the network, and licence configuration comes from
// t.Setenv, never a real key.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// nkjvShapedBible assembles a BibleData through the NKJV decoder
// (decodeAPIBibleChapter) — the exact verse shape a licensed API.Bible fetch
// produces, including "\n" poem lines — with one poetry chapter (Psalm 46) and
// one prose chapter (John 3).
func nkjvShapedBible(t *testing.T) *BibleData {
	t.Helper()
	psalm, err := decodeAPIBibleChapter(json.RawMessage(psalmChapterContent), "Psalms", 46)
	if err != nil {
		t.Fatal(err)
	}
	john, err := decodeAPIBibleChapter(json.RawMessage(proseChapterContent), "John", 3)
	if err != nil {
		t.Fatal(err)
	}
	return &BibleData{
		Books: []string{"Psalms", "John"},
		Verses: map[string]map[int][]Verse{
			"Psalms": {46: psalm},
			"John":   {3: john},
		},
	}
}

// --- switchVersion / canSelect ------------------------------------------------

// TestNKJVSelectableAndSwitchLoadsRealText: with the env trio set (key +
// licence opt-in + provider id) NKJV serves real text, is selectable, and
// switchVersion swaps the reader onto the fetched canon in modeReal.
func TestNKJVSelectableAndSwitchLoadsRealText(t *testing.T) {
	srv := apiBibleFixture(t)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible")
	// Off on purpose: NKJV must be selectable because its text is REAL, not
	// because the internal-QA placeholder flag happens to be set.
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "")
	t.Setenv("BIBLETEXT_CACHE_PATH", filepath.Join(t.TempDir(), "bibletext-cache.json"))

	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if v.isTesting() {
		t.Fatal("with the env trio set nkjv must serve real text, not a placeholder")
	}
	if !v.canSelect() {
		t.Fatal("with the env trio set nkjv must be selectable")
	}

	base := baseSampleBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: "web",
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{"web": base},
		CurrentBook:    "John",
		CurrentChapter: 1,
	}
	switchVersion(state, "nkjv")

	if state.CurrentVersion != "nkjv" || state.currentMode != modeReal {
		t.Fatalf("after switch: version=%q mode=%v", state.CurrentVersion, state.currentMode)
	}
	if state.currentVersion().Abbrev != "NKJV" {
		t.Errorf("currentVersion abbrev = %q", state.currentVersion().Abbrev)
	}
	if len(state.Bible.Books) != 66 {
		t.Fatalf("switched canon has %d books, want 66", len(state.Bible.Books))
	}
	// The reader is on the fixture's real text, not placeholder boilerplate.
	ps := state.Bible.GetChapter("Psalms", 1)
	if len(ps) == 0 || !strings.Contains(ps[0].Text, "refuge") {
		t.Errorf("switched text is not the fixture's real text: %+v", ps)
	}
	for _, verses := range state.Bible.Verses {
		for _, vs := range verses {
			for _, verse := range vs {
				if strings.Contains(verse.Text, "licensed text not available") {
					t.Fatalf("placeholder text leaked into a real NKJV load: %q", verse.Text)
				}
			}
		}
	}
}

// TestNKJVSwitchRefusedWithoutLicenseEnv pins the exact refusal in
// versions.go: without the env trio nkjv fails canSelect, and switchVersion
// returns early as a silent no-op — the current version, data pointer, and
// mode are untouched, and nothing is cached under "nkjv".
func TestNKJVSwitchRefusedWithoutLicenseEnv(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "")

	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if v.canSelect() {
		t.Fatal("nkjv must not be selectable without the licence env trio")
	}

	base := baseSampleBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: "web",
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{"web": base},
		CurrentBook:    "John",
		CurrentChapter: 1,
	}
	switchVersion(state, "nkjv")

	if state.CurrentVersion != "web" {
		t.Errorf("refusal must keep the current version; got %q", state.CurrentVersion)
	}
	if state.Bible != base || state.currentMode != modeReal {
		t.Error("refusal must leave the reader's data and mode untouched")
	}
	if _, cached := state.loadedVersions["nkjv"]; cached {
		t.Error("a refused switch must not cache anything under nkjv")
	}
}

// --- Share-as-link fallback ---------------------------------------------------

// TestNKJVShareLinkNamesNKJV asserts the OPPOSITE of what this test asserted
// before, deliberately. It was TestNKJVShareLinkFallsBackToWeb, and it pinned
// an NKJV link as byte-for-byte the WEB link with "nkjv" appearing nowhere in
// any URL — on the rule that a licensed version id must never leak into a
// public URL.
//
// The rule was wrong about what a licence covers. It protects the TEXT; the
// name of a translation is not the text, and "nkjv" as a path segment discloses
// nothing a licence has any claim on. Meanwhile the fallback quietly broke the
// thing links exist for: the sender's own link reopened in the WEB, in wording
// they were not reading, and the note they attached was filed against it. The
// owner's instruction settled it — "The NKJV links should work just like the
// others: bibletext.co.uk/nkjv/john/... etc".
//
// What has NOT changed, and is still tested here: the deuterocanon still forces
// webc, a genuinely unknown id still falls back to web, and every /web/ link is
// byte-identical to what it was before (TestWebLinksAreByteStable).
func TestNKJVShareLinkNamesNKJV(t *testing.T) {
	for _, tc := range []struct {
		name            string
		book            string
		chapter, lo, hi int
		want            string
	}{
		{"range", "John", 3, 16, 18, "https://bibletext.co.uk/nkjv/john/3/#v16-18"},
		{"chapter link", "Psalms", 46, 0, 0, "https://bibletext.co.uk/nkjv/psalms/46/"},
		{"single poetry verse", "Psalms", 46, 1, 0, "https://bibletext.co.uk/nkjv/psalms/46/#v1"},
	} {
		if got := ShareLinkURL("nkjv", tc.book, tc.chapter, tc.lo, tc.hi); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}

	// A note rides alongside the translation, exactly as it does for web/bsb.
	got := ShareLinkURLWithNote("nkjv", "John", 3, 16, 16, "hope you love this")
	if !strings.HasPrefix(got, shareLinkBase+"/nkjv/john/3/#v16&n=") {
		t.Errorf("noted nkjv link:\n got %q\nwant the /nkjv/ path with an n= key", got)
	}

	// Every 66-canon book keeps its own path. The NKJV has no deuterocanon, so
	// nothing here should ever be rewritten to webc.
	for _, book := range NewBibleData().Books {
		url := ShareLinkURL("nkjv", book, 3, 16, 18)
		if !strings.HasPrefix(url, shareLinkBase+"/nkjv/") {
			t.Errorf("%s: nkjv share URL must name /nkjv/: %q", book, url)
		}
	}

	// The link the app emits is one the app can open again — a /nkjv/ URL that
	// did not parse back would be worse than the old downgrade: the OS would
	// hand it to a browser and the site serves nothing there.
	target, ok := ParseShareLink("https://bibletext.co.uk/nkjv/john/3/#v16-18")
	if !ok {
		t.Fatal("an emitted /nkjv/ link must parse back; otherwise it opens a 404 in the browser")
	}
	if target.VersionID != "nkjv" || target.Book != "John" || target.Chapter != 3 ||
		target.VerseLo != 16 || target.VerseHi != 18 {
		t.Errorf("round-tripped nkjv link: %+v", target)
	}
}

// TestWebLinksAreByteStable: links already sitting in message threads must not
// move because a new id joined the path grammar. These strings are goldens —
// they are what the app emitted before /nkjv/ existed.
func TestWebLinksAreByteStable(t *testing.T) {
	for _, tc := range []struct {
		version, book   string
		chapter, lo, hi int
		want            string
	}{
		{"web", "John", 3, 16, 18, "https://bibletext.co.uk/web/john/3/#v16-18"},
		{"web", "Psalms", 119, 105, 0, "https://bibletext.co.uk/web/psalms/119/#v105"},
		{"bsb", "1 Corinthians", 13, 4, 7, "https://bibletext.co.uk/bsb/1-corinthians/13/#v4-7"},
		{"webc", "Wisdom", 3, 1, 0, "https://bibletext.co.uk/webc/wisdom/3/#v1"},
		// Still rewritten: the deuterocanon exists only in the Catholic canon.
		{"web", "1 Maccabees", 2, 19, 22, "https://bibletext.co.uk/webc/1-maccabees/2/#v19-22"},
		{"nkjv", "1 Maccabees", 2, 19, 22, "https://bibletext.co.uk/webc/1-maccabees/2/#v19-22"},
		// Still falls back: an id nothing recognises names no reader page.
		{"zzz", "John", 3, 16, 0, "https://bibletext.co.uk/web/john/3/#v16"},
		{"", "John", 3, 16, 0, "https://bibletext.co.uk/web/john/3/#v16"},
	} {
		if got := ShareLinkURL(tc.version, tc.book, tc.chapter, tc.lo, tc.hi); got != tc.want {
			t.Errorf("%s %s %d:%d-%d:\n got %q\nwant %q",
				tc.version, tc.book, tc.chapter, tc.lo, tc.hi, got, tc.want)
		}
	}
}

// TestLinkPathVersionIDsIsNotThePublishedSet pins the split itself. The two
// answered the same question until /nkjv/ arrived, under one name; re-merging
// them is how someone concludes the site publishes the NKJV and either points
// cmd/websitegen at it or relaxes its licensed-exclusion tests.
func TestLinkPathVersionIDsIsNotThePublishedSet(t *testing.T) {
	if webPublishedVersionIDs["nkjv"] {
		t.Error("the web reader must NOT publish nkjv — that half is deferred (owner)")
	}
	if !linkPathVersionIDs["nkjv"] {
		t.Error("a share-link path must be able to name nkjv")
	}
	for id := range webPublishedVersionIDs {
		if !linkPathVersionIDs[id] {
			t.Errorf("%q is published but cannot appear in a link path", id)
		}
	}
	// Enumerated, not derived: BIBLETEXT_ENABLE_TESTING makes canSelect() true
	// for every registered version, and a registry-wide set would start emitting
	// paths nothing claims and nothing serves.
	for _, id := range []string{"nrsv", "lsb"} {
		if linkPathVersionIDs[id] {
			t.Errorf("%q must not be a link path id: no app or site surface honours it", id)
		}
	}
}

// --- Search over NKJV-shaped data ---------------------------------------------

func TestNKJVSearchIndexReferenceAndKeyword(t *testing.T) {
	bd := nkjvShapedBible(t)
	bd.PrepareSearchIndex()

	// Reference lookup: "John 3:16" resolves to exactly the decoded verse.
	results, truncated := bd.SearchSmartLimited("John 3:16", 5)
	if truncated || len(results) != 1 {
		t.Fatalf("John 3:16 lookup: %d results (truncated=%v)", len(results), truncated)
	}
	got := results[0]
	if got.BookName != "John" || got.Chapter != 3 || got.Verse != 16 {
		t.Errorf("John 3:16 resolved to %s %d:%d", got.BookName, got.Chapter, got.Verse)
	}
	if got.Text != "For God so loved the world, that he gave his one and only Son." {
		t.Errorf("John 3:16 text = %q", got.Text)
	}

	// Keyword search works over the same data.
	kw, _ := bd.SearchSmartLimited("refuge", 10)
	if len(kw) != 1 || kw[0].BookName != "Psalms" || kw[0].Verse != 1 {
		t.Errorf("keyword 'refuge': %+v", kw)
	}

	// A phrase spanning a poem-line "\n" must still match: the index flattens
	// authored line breaks to spaces for search.
	span := bd.Search("strength, a very present")
	if len(span) != 1 || span[0].Verse != 1 || span[0].Chapter != 46 {
		t.Errorf("phrase across a poetry line break: %+v", span)
	}

	// Verse.Ref is the precomputed lowercased "book c:v".
	if ref := bd.Verses["John"][3][0].Ref; ref != "john 3:16" {
		t.Errorf("John ref = %q, want %q", ref, "john 3:16")
	}
	if ref := bd.Verses["Psalms"][46][0].Ref; ref != "psalms 46:1" {
		t.Errorf("Psalms ref = %q, want %q", ref, "psalms 46:1")
	}
}

// --- Poetry through the share structure ---------------------------------------

// TestNKJVPoetryShareLines: NKJV poetry verses (decoder-produced "\n") flow
// through verseIsPoetic/poeticJoin into chapterShareStructure, so shared text
// restores the authored line boundaries — both the break INSIDE a verse and
// the verse-join break that exists because the join touches a poetic verse.
func TestNKJVPoetryShareLines(t *testing.T) {
	bd := nkjvShapedBible(t)
	psalm := bd.Verses["Psalms"][46]
	if !verseIsPoetic(psalm[0].Text) {
		t.Fatalf("decoded Psalm 46:1 must carry a poem line: %q", psalm[0].Text)
	}
	if verseIsPoetic(psalm[1].Text) {
		t.Fatalf("Psalm 46:2 is a single poem line, so it reads as prose: %q", psalm[1].Text)
	}
	if !poeticJoin(psalm[0].Text, psalm[1].Text) {
		t.Fatal("a join touching a poetic verse must be a line boundary")
	}

	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 46}
	flat, breaks := chapterShareStructure(state)
	wantFlat := "God is our refuge and strength, a very present help in trouble. Therefore we won't be afraid,"
	if flat != wantFlat {
		t.Fatalf("flat chapter:\n got %q\nwant %q", flat, wantFlat)
	}
	if len(breaks) != 2 {
		t.Fatalf("want 2 structural breaks (in-verse line + poetic join), got %d: %+v", len(breaks), breaks)
	}
	restored := restoreShareLineBreaks(state, flat)
	want := "God is our refuge and strength,\na very present help in trouble.\nTherefore we won't be afraid,"
	if restored != want {
		t.Errorf("restored poetry lines:\n got %q\nwant %q", restored, want)
	}
}

func TestNKJVProseShareStaysProse(t *testing.T) {
	bd := nkjvShapedBible(t)
	john := bd.Verses["John"][3]
	if verseIsPoetic(john[0].Text) || verseIsPoetic(john[1].Text) {
		t.Fatalf("prose verses must carry no poem lines: %+v", john)
	}
	if poeticJoin(john[0].Text, john[1].Text) {
		t.Fatal("a prose-prose join is not a line boundary")
	}

	state := &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}
	flat, breaks := chapterShareStructure(state)
	want := john[0].Text + " " + john[1].Text
	if flat != want {
		t.Errorf("prose chapter:\n got %q\nwant %q", flat, want)
	}
	if len(breaks) != 0 {
		t.Errorf("a short prose chapter has no structural breaks, got %+v", breaks)
	}
	if restored := restoreShareLineBreaks(state, flat); restored != flat {
		t.Errorf("prose share must come back unchanged:\n got %q", restored)
	}
}

// --- LicenseNotice --------------------------------------------------------------

// TestNKJVLicenseNotice pins the attribution contract: NKJV carries the notice
// Thomas Nelson requires plus the visible API.Bible citation, and it is the
// ONLY registered version with a LicenseNotice (public-domain versions say
// everything in their Publisher line; NRSV/LSB have no licence yet).
func TestNKJVLicenseNotice(t *testing.T) {
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if v.LicenseNotice == "" {
		t.Fatal("nkjv must carry a LicenseNotice")
	}
	if !strings.Contains(v.LicenseNotice, "Thomas Nelson") {
		t.Errorf("notice must credit Thomas Nelson: %q", v.LicenseNotice)
	}
	if !strings.Contains(strings.ToLower(v.LicenseNotice), "api.bible") {
		t.Errorf("notice must carry the visible api.bible citation: %q", v.LicenseNotice)
	}

	for _, other := range bibleVersions() {
		if other.ID == "nkjv" {
			continue
		}
		if other.LicenseNotice != "" {
			t.Errorf("%s must not carry a LicenseNotice: %q", other.ID, other.LicenseNotice)
		}
	}
}

// --- Verse of the day -----------------------------------------------------------

// TestNKJVVerseOfDayOnCanon: over a full NKJV-shaped canon (the fixture
// fetch), verse of the day honours its contract — unresolvable curated refs
// are skipped, the pick exists in the loaded translation with real text, and
// the choice is stable within a day.
func TestNKJVVerseOfDayOnCanon(t *testing.T) {
	srv := apiBibleFixture(t)
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	data, err := fetchAPIBible("NKJV", "test-bible", "test-key")
	if err != nil {
		t.Fatal(err)
	}
	data.PrepareSearchIndex()
	state := &AppState{Bible: data, CurrentVersion: "nkjv", CurrentBook: "John", CurrentChapter: 1}

	valid := resolvedVerseOfDay(state)
	if len(valid) == 0 {
		t.Fatal("the rotation must resolve at least one ref against the canon")
	}
	for _, rv := range valid {
		if state.Bible.GetVerse(rv.BookName, rv.Chapter, rv.Verse) == nil {
			t.Errorf("resolved rotation entry %s %d:%d does not exist in the canon",
				rv.BookName, rv.Chapter, rv.Verse)
		}
	}

	v, ok := verseOfTheDay(state)
	if !ok {
		t.Fatal("verseOfTheDay must return a verse for a populated canon")
	}
	if strings.TrimSpace(v.Text) == "" {
		t.Errorf("verse of the day has no text: %+v", v)
	}
	if state.Bible.GetVerse(v.BookName, v.Chapter, v.Verse) == nil {
		t.Errorf("verse of the day %s %d:%d is not in the loaded translation",
			v.BookName, v.Chapter, v.Verse)
	}
	if again, ok2 := verseOfTheDay(state); !ok2 || again != v {
		t.Error("verse of the day must be stable within a calendar day")
	}
}

// --- TTS speech text -------------------------------------------------------------

// TestNKJVChapterSpeechTextPoetry: on an NKJV poetry chapter the "\n" poem
// lines must not break the spoken text — every word survives, line boundaries
// stay whitespace (never gluing "strength,christ" style), and
// speechVerseOffsets stays aligned with exactly the string handed to TTS (the
// read-along contract).
func TestNKJVChapterSpeechTextPoetry(t *testing.T) {
	bd := nkjvShapedBible(t)
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 46}

	got := chapterSpeechText(state)
	if got == "" {
		t.Fatal("poetry chapter must produce speech text")
	}
	// Word-for-word identical to the chapter, with the poem-line "\n" acting as
	// plain whitespace between words: joining the fields back with single
	// spaces yields the prose reading, proving nothing was glued or lost.
	wantWords := "God is our refuge and strength, a very present help in trouble. Therefore we won't be afraid,"
	if joined := strings.Join(strings.Fields(got), " "); joined != wantWords {
		t.Errorf("spoken words:\n got %q\nwant %q", joined, wantWords)
	}

	offs := speechVerseOffsets(state)
	if len(offs) != 2 || offs[0].verse != 1 || offs[1].verse != 2 {
		t.Fatalf("speech offsets: %+v", offs)
	}
	if offs[0].start != 0 {
		t.Errorf("verse 1 must start the utterance, got offset %v", offs[0].start)
	}
	// The fixture is pure ASCII, so UTF-16 units index the Go string directly:
	// verse 2's offset must land exactly on its first word even though verse 1
	// carries an internal "\n".
	if at := int(offs[1].start); at >= len(got) || !strings.HasPrefix(got[at:], "Therefore") {
		t.Errorf("verse 2 offset %v does not land on its text in %q", offs[1].start, got)
	}
}
