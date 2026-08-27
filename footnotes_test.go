package bibletext

// The footnotes MACHINERY contract (docs/FOOTNOTES.md). Two decoders
// capture the translators'
// apparatus side-band; these tests pin the three properties everything else
// depends on: (1) verse TEXT is byte-identical with capture on — apparatus
// never enters Scripture; (2) anchors are exact rune offsets into the final
// text, including at poem-line and punctuation-glue boundaries; (3) the
// side-band data survives the cache round trip and never reaches the search
// index, the spoken text, or the share pipeline's prose.

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2/test"
)

// --- helloao (WEB/BSB/WEBC) ---------------------------------------------------

// A realistic helloao book: prose with a mid-verse marker, the Psalm 23:1
// marker-between-poem-lines shape (real capture), the "egg?" punctuation-glue
// shape, a verse-final marker, and a superscription-anchored body (noteId 9)
// whose marker the decoder never renders.
const helloAOFixtureBook = `{
  "id": "PSA", "order": 19,
  "chapters": [
    {"chapter": {"number": 23,
      "content": [
        {"type": "verse", "number": 1, "content": [
          {"text": "The LORD is my shepherd;", "poem": 1},
          {"noteId": 1},
          {"text": "I shall not want.", "poem": 2}
        ]},
        {"type": "verse", "number": 2, "content": [
          "He makes me lie down", {"noteId": 2}, "in green pastures."
        ]},
        {"type": "verse", "number": 3, "content": [
          "Can that which has no taste be eaten without salt, or is there any taste in the white of an egg", {"noteId": 3}, "?"
        ]},
        {"type": "verse", "number": 4, "content": [
          "He restores my soul.", {"noteId": 4}
        ]},
        {"type": "verse", "number": 5, "content": [
          "A verse whose marker has no body", {"noteId": 8}, "keeps reading."
        ]},
        {"type": "hebrew_subtitle", "content": [
          "A Psalm of David.", {"noteId": 9}
        ]}
      ],
      "footnotes": [
        {"noteId": 1, "caller": "+", "text": "Or The LORD tends me as His sheep.", "reference": {"chapter": 23, "verse": 1}},
        {"noteId": 2, "caller": "+", "text": "Hebrew waters of rest.", "reference": {"chapter": 23, "verse": 2}},
        {"noteId": 3, "caller": "+", "text": "A note glued before punctuation.", "reference": {"chapter": 23, "verse": 3}},
        {"noteId": 4, "caller": "+", "text": "A verse-final note.", "reference": {"chapter": 23, "verse": 4}},
        {"noteId": 9, "caller": "+", "text": "Anchored in a superscription; must be dropped.", "reference": {"chapter": 23, "verse": 1}}
      ]
    }}
  ]
}`

func decodeFixtureBook(t *testing.T) map[int][]Verse {
	t.Helper()
	chapters, _ := decodeFixtureBookAll(t)
	return chapters
}

func decodeFixtureBookAll(t *testing.T) (map[int][]Verse, map[int][]OrphanFootnote) {
	t.Helper()
	var b helloAOBook
	if err := json.Unmarshal([]byte(helloAOFixtureBook), &b); err != nil {
		t.Fatal(err)
	}
	return decodeHelloAOChapters("Psalms", b)
}

// (1) Byte-identity: the text with capture on is the historical text — the
// markers contribute nothing, the seams heal exactly as before.
func TestFootnotesHelloAOTextUnchanged(t *testing.T) {
	vs := decodeFixtureBook(t)[23]
	want := []string{
		"The LORD is my shepherd;\nI shall not want.",
		"He makes me lie down in green pastures.",
		"Can that which has no taste be eaten without salt, or is there any taste in the white of an egg?",
		"He restores my soul.",
		"A verse whose marker has no body keeps reading.",
	}
	if len(vs) != len(want) {
		t.Fatalf("got %d verses, want %d", len(vs), len(want))
	}
	for i, w := range want {
		if vs[i].Text != w {
			t.Errorf("verse %d text changed:\n got  %q\n want %q", i+1, vs[i].Text, w)
		}
		if strings.ContainsRune(vs[i].Text, footnoteSentinel) {
			t.Errorf("verse %d leaked a sentinel", i+1)
		}
		for _, fn := range vs[i].Footnotes {
			if strings.Contains(vs[i].Text, fn.Text) {
				t.Errorf("verse %d: footnote body appears inside Scripture text", i+1)
			}
		}
	}
}

// (2) Anchors: end-of-poem-line, mid-prose word boundary, before glued
// punctuation, and verse-final — all exact rune offsets into the final text.
func TestFootnotesHelloAOAnchors(t *testing.T) {
	vs := decodeFixtureBook(t)[23]

	v1 := vs[0] // marker between the two poem lines → end of line 1
	if len(v1.Footnotes) != 1 {
		t.Fatalf("v1 footnotes = %d, want 1", len(v1.Footnotes))
	}
	if want := utf8.RuneCountInString("The LORD is my shepherd;"); v1.Footnotes[0].Anchor != want {
		t.Errorf("poem-line anchor = %d, want %d (end of line 1, before the \\n)", v1.Footnotes[0].Anchor, want)
	}
	if v1.Footnotes[0].Text != "Or The LORD tends me as His sheep." || v1.Footnotes[0].Caller != "+" {
		t.Errorf("v1 footnote body/caller wrong: %+v", v1.Footnotes[0])
	}

	v2 := vs[1] // mid-prose
	if want := utf8.RuneCountInString("He makes me lie down"); v2.Footnotes[0].Anchor != want {
		t.Errorf("mid-prose anchor = %d, want %d", v2.Footnotes[0].Anchor, want)
	}

	v3 := vs[2] // before glued "?": anchor sits before the question mark
	if want := utf8.RuneCountInString(v3.Text) - 1; v3.Footnotes[0].Anchor != want {
		t.Errorf("punctuation-glue anchor = %d, want %d (before the ?)", v3.Footnotes[0].Anchor, want)
	}

	v4 := vs[3] // verse-final
	if want := utf8.RuneCountInString(v4.Text); v4.Footnotes[0].Anchor != want {
		t.Errorf("verse-final anchor = %d, want %d", v4.Footnotes[0].Anchor, want)
	}

	// Every anchor is a legal offset.
	for _, v := range vs {
		max := utf8.RuneCountInString(v.Text)
		for _, fn := range v.Footnotes {
			if fn.Anchor < 0 || fn.Anchor > max {
				t.Errorf("verse %d anchor %d out of range 0..%d", v.Verse, fn.Anchor, max)
			}
		}
	}
}

// A verse whose ONLY content is a footnote marker — the critical-text
// omissions (Luke 17:36, Acts 8:37, …), where the note explains the verse's
// absence. The TEXT decodes exactly as it always has: no verse, no empty
// number on the page. The NOTE, though, is captured as an orphan (34 real
// ones across WEB/WEBC) so the chapter-bottom section can carry the
// explanation, keyed by the very verse number the reader won't find above.
func TestFootnotesOmittedVerseNoteBecomesOrphan(t *testing.T) {
	book := `{"id":"LUK","order":42,"chapters":[{"chapter":{"number":17,
	  "content":[
	    {"type":"verse","number":35,"content":["Two will be grinding together."]},
	    {"type":"verse","number":36,"content":[{"noteId":37}]}
	  ],
	  "footnotes":[{"noteId":37,"caller":"+","text":"Some Greek copies add verse 36.","reference":{"chapter":17,"verse":36}}]
	}}]}`
	var b helloAOBook
	if err := json.Unmarshal([]byte(book), &b); err != nil {
		t.Fatal(err)
	}
	chapters, orphans := decodeHelloAOChapters("Luke", b)
	vs := chapters[17]
	if len(vs) != 1 || vs[0].Verse != 35 {
		t.Fatalf("empty verse 36 must stay out of the TEXT as it always was: %+v", vs)
	}
	if len(vs[0].Footnotes) != 0 {
		t.Errorf("verse 36's note must not migrate onto verse 35: %+v", vs[0].Footnotes)
	}
	// The note itself is no longer dropped: it is captured as an ORPHAN keyed
	// by the omitted verse — the chapter-bottom section's row that explains
	// why there is no verse 36 on the page.
	want := []OrphanFootnote{{Verse: 36, Text: "Some Greek copies add verse 36.", Caller: "+"}}
	if got := orphans[17]; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("omitted verse 36's note must be captured as an orphan:\n got: %+v\nwant: %+v", got, want)
	}
}

// (3) Orphans on both sides: a marker with no body captures nothing; a body
// whose marker sits in an unrendered superscription is dropped.
func TestFootnotesHelloAOOrphans(t *testing.T) {
	chapters, orphans := decodeFixtureBookAll(t)
	vs := chapters[23]
	if n := len(vs[4].Footnotes); n != 0 {
		t.Errorf("bodiless marker produced %d footnotes, want 0", n)
	}
	for _, v := range vs {
		for _, fn := range v.Footnotes {
			if strings.Contains(fn.Text, "superscription") {
				t.Errorf("superscription-anchored body attached to verse %d", v.Verse)
			}
		}
	}
	// The superscription body must not slip into the ORPHAN table either:
	// omitted-verse capture reads only markers inside emitted-but-textless
	// verse nodes, never unconsumed bodies by reference.
	for ch, os := range orphans {
		for _, o := range os {
			if strings.Contains(o.Text, "superscription") {
				t.Errorf("superscription-anchored body captured as an orphan in chapter %d: %+v", ch, o)
			}
		}
	}
}

// --- purity: the pipelines that read Verse.Text stay footnote-free ------------

func TestFootnotesNeverReachSearchSpeechOrProse(t *testing.T) {
	chapters := decodeFixtureBook(t)
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": chapters}}
	// An orphan with a distinctive probe word: the table is invisible to
	// every Verse.Text pipeline by construction, and this proves it.
	bd.OrphanFootnotes = map[string]map[int][]OrphanFootnote{
		"Psalms": {23: {{Verse: 7, Text: "Some copies add an orphanprobe verse."}}},
	}
	bd.PrepareSearchIndex()

	// Search: a distinctive word from a footnote body must find nothing.
	if hits := bd.Search("tends"); len(hits) != 0 {
		t.Errorf("footnote text is searchable: %d hits for a body-only word", len(hits))
	}
	// The index itself: Verse.Search built from Text only.
	for _, v := range bd.Verses["Psalms"][23] {
		for _, fn := range v.Footnotes {
			if strings.Contains(v.Search, strings.ToLower(fn.Text)) {
				t.Error("footnote body leaked into the search index")
			}
		}
	}

	// Spoken text and the share pipeline's prose are joins of Verse.Text.
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
	speech := chapterSpeechText(state)
	prose, _ := chapterShareStructure(state)
	for _, probe := range []string{"tends", "waters of rest", "glued", "verse-final", "orphanprobe"} {
		if strings.Contains(strings.ToLower(speech), probe) {
			t.Errorf("footnote text reached the spoken chapter: %q", probe)
		}
		if strings.Contains(strings.ToLower(prose), probe) {
			t.Errorf("footnote text reached the share prose: %q", probe)
		}
	}

	// The whole-chapter copy icon, probed with the bottom section RENDERED
	// (the sharpest case): even while the translators' notes are on screen,
	// the chapter copy is a join of Verse.Text and carries none of them.
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	copied := strings.ToLower(chapterCopyText(state))
	if copied == "" || !strings.Contains(copied, "psalms 23") {
		t.Fatalf("copy probe produced no chapter text — the control string is missing, so the check can't prove anything: %q", copied)
	}
	for _, probe := range []string{"tends", "waters of rest", "orphanprobe"} {
		if strings.Contains(copied, probe) {
			t.Errorf("footnote text reached the chapter copy: %q", probe)
		}
	}
}

// --- cache round trip ---------------------------------------------------------

func TestFootnotesSurviveCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", dir+"/bibletext-cache.json")
	chapters := decodeFixtureBook(t)
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": chapters}}

	path := cachePathForVersion("bsb")
	if err := saveBibleToCache(path, bd, currentUTCTime); err != nil {
		t.Fatal(err)
	}
	back, err := loadBibleFromCache(path)
	if err != nil {
		t.Fatal(err)
	}
	got := back.Verses["Psalms"][23]
	if len(got[0].Footnotes) != 1 || got[0].Footnotes[0].Text != "Or The LORD tends me as His sheep." ||
		got[0].Footnotes[0].Anchor != utf8.RuneCountInString("The LORD is my shepherd;") {
		t.Errorf("footnotes did not survive the cache round trip: %+v", got[0].Footnotes)
	}
	// A verse without footnotes stays clean (omitempty — no phantom field).
	if got[4].Footnotes != nil && len(got[4].Footnotes) != 0 {
		t.Errorf("noteless verse gained footnotes: %+v", got[4].Footnotes)
	}
}

// The orphan table survives its own round trip.
func TestOrphanFootnotesSurviveCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", dir+"/bibletext-cache.json")
	bd := &BibleData{
		Books:  []string{"Luke"},
		Verses: map[string]map[int][]Verse{"Luke": {17: {{BookName: "Luke", Book: "Luke", Chapter: 17, Verse: 35, Text: "Two will be grinding together."}}}},
		OrphanFootnotes: map[string]map[int][]OrphanFootnote{
			"Luke": {17: {{Verse: 36, Text: "Some Greek copies add verse 36.", Caller: "+"}}},
		},
	}
	path := cachePathForVersion("web")
	if err := saveBibleToCache(path, bd, currentUTCTime); err != nil {
		t.Fatal(err)
	}
	back, err := loadBibleFromCache(path)
	if err != nil {
		t.Fatal(err)
	}
	got := back.OrphanNotesFor("Luke", 17)
	if len(got) != 1 || got[0] != (OrphanFootnote{Verse: 36, Text: "Some Greek copies add verse 36.", Caller: "+"}) {
		t.Fatalf("orphans did not survive the cache round trip: %+v", got)
	}
	// Nil-safety of the accessor at every level.
	if (*BibleData)(nil).OrphanNotesFor("Luke", 17) != nil {
		t.Error("nil BibleData must yield nil orphans")
	}
	if back.OrphanNotesFor("Mark", 1) != nil {
		t.Error("absent book must yield nil orphans")
	}
}

// A pre-footnotes cache (no footnotes field anywhere) still loads — the field
// is additive, the schema version unchanged.
func TestFootnotesOldCacheStillLoads(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BIBLETEXT_CACHE_PATH", dir+"/bibletext-cache.json")
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {23: {
		{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: "The LORD is my shepherd."},
	}}}}
	path := cachePathForVersion("web")
	if err := saveBibleToCache(path, bd, currentUTCTime); err != nil {
		t.Fatal(err)
	}
	back, err := loadBibleFromCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if v := back.Verses["Psalms"][23][0]; v.Footnotes != nil {
		t.Errorf("verse from an old cache grew footnotes: %+v", v.Footnotes)
	}
	if back.OrphanFootnotes != nil {
		t.Errorf("old cache grew an orphan table: %+v", back.OrphanFootnotes)
	}
}

// --- the NKJV walk (API.Bible) ------------------------------------------------

// A realistic NKJV-shaped chapter: a cross-reference note (style x — the only
// kind the live feed carries, probed 2026-08-26) with an xo origin span to
// strip and ref tags to flatten, plus a translator-footnote-shaped note
// (style f) so the machinery is proven for both kinds, a note at a poem-line
// boundary, and a note inside a red-letter span.
const nkjvNotedChapter = `[
  {"name":"para","type":"tag","attrs":{"style":"p"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"2","sid":"JHN 3:2"},"items":[{"type":"text","text":"2"}]},
    {"type":"text","text":"This man came to Jesus by night","attrs":{"verseId":"JHN.3.2"}},
    {"name":"note","type":"tag","attrs":{"style":"x","caller":"-","id":"JHN.3.2!x.1","verseId":"JHN.3.2"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"xo"},"items":[{"type":"text","text":"3:2 "}]},
      {"name":"char","type":"tag","attrs":{"style":"xt"},"items":[
        {"name":"ref","type":"tag","attrs":{"id":"JHN.7.50"},"items":[{"type":"text","text":"John 7:50"}]},
        {"type":"text","text":"; "},
        {"name":"ref","type":"tag","attrs":{"id":"JHN.19.39"},"items":[{"type":"text","text":"19:39"}]}
      ]}
    ]},
    {"type":"text","text":" and said to Him.","attrs":{"verseId":"JHN.3.2"}},
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"13","sid":"JHN 3:13"},"items":[{"type":"text","text":"13"}]},
    {"type":"text","text":"No one has ascended to heaven","attrs":{"verseId":"JHN.3.13"}},
    {"name":"note","type":"tag","attrs":{"style":"f","caller":"+","id":"JHN.3.13!f.1","verseId":"JHN.3.13"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"fr"},"items":[{"type":"text","text":"3:13 "}]},
      {"name":"char","type":"tag","attrs":{"style":"ft"},"items":[{"type":"text","text":"NU-Text omits who is in heaven."}]}
    ]},
    {"type":"text","text":" but He who came down from heaven.","attrs":{"verseId":"JHN.3.13"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"14","sid":"JHN 3:14"},"items":[{"type":"text","text":"14"}]},
    {"type":"text","text":"As Moses lifted up the serpent,","attrs":{"verseId":"JHN.3.14"}},
    {"name":"note","type":"tag","attrs":{"style":"x","caller":"-","id":"JHN.3.14!x.1","verseId":"JHN.3.14"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"xo"},"items":[{"type":"text","text":"3:14 "}]},
      {"name":"char","type":"tag","attrs":{"style":"xt"},"items":[{"type":"text","text":"Numbers 21:9"}]}
    ]}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q2"},"items":[
    {"type":"text","text":"even so must the Son of Man be lifted up.","attrs":{"verseId":"JHN.3.14"}}
  ]}
]`

func TestFootnotesNKJVCaptureAndPurity(t *testing.T) {
	byCh, err := decodeAPIBiblePassage(json.RawMessage(nkjvNotedChapter), "John", 3)
	if err != nil {
		t.Fatal(err)
	}
	vs := byCh[3]
	if len(vs) != 3 {
		t.Fatalf("got %d verses, want 3", len(vs))
	}

	v2 := vs[0]
	if v2.Text != "This man came to Jesus by night and said to Him." {
		t.Errorf("v2 text corrupted by note capture: %q", v2.Text)
	}
	if len(v2.Footnotes) != 1 {
		t.Fatalf("v2 footnotes = %d, want 1", len(v2.Footnotes))
	}
	fn := v2.Footnotes[0]
	if fn.Kind != footnoteKindCrossref || fn.Caller != "-" {
		t.Errorf("crossref kind/caller wrong: %+v", fn)
	}
	if fn.Text != "John 7:50; 19:39" {
		t.Errorf("crossref body = %q, want the flattened refs without the xo origin", fn.Text)
	}
	if want := utf8.RuneCountInString("This man came to Jesus by night"); fn.Anchor != want {
		t.Errorf("crossref anchor = %d, want %d", fn.Anchor, want)
	}

	v13 := vs[1]
	if v13.Text != "No one has ascended to heaven but He who came down from heaven." {
		t.Errorf("v13 text corrupted: %q", v13.Text)
	}
	if len(v13.Footnotes) != 1 || v13.Footnotes[0].Kind != "" ||
		v13.Footnotes[0].Text != "NU-Text omits who is in heaven." {
		t.Fatalf("translator footnote wrong: %+v", v13.Footnotes)
	}
	if want := utf8.RuneCountInString("No one has ascended to heaven"); v13.Footnotes[0].Anchor != want {
		t.Errorf("footnote anchor = %d, want %d", v13.Footnotes[0].Anchor, want)
	}

	v14 := vs[2] // note at the poem-line boundary: anchor ends line 1
	if v14.Text != "As Moses lifted up the serpent,\neven so must the Son of Man be lifted up." {
		t.Errorf("v14 poem structure changed by the note: %q", v14.Text)
	}
	if len(v14.Footnotes) != 1 {
		t.Fatalf("v14 footnotes = %d, want 1", len(v14.Footnotes))
	}
	if want := utf8.RuneCountInString("As Moses lifted up the serpent,"); v14.Footnotes[0].Anchor != want {
		t.Errorf("poem-boundary anchor = %d, want %d (end of line 1)", v14.Footnotes[0].Anchor, want)
	}

	for _, v := range vs {
		if strings.ContainsRune(v.Text, footnoteSentinel) {
			t.Errorf("verse %d leaked the sentinel", v.Verse)
		}
	}
}

func TestStripFootnoteSentinels(t *testing.T) {
	sent := string(footnoteSentinel)
	cases := []struct {
		in      string
		want    string
		anchors []int
	}{
		{"no sentinels here", "no sentinels here", nil},
		{"word" + sent + " next", "word next", []int{4}},
		{sent + "leading", "leading", []int{0}},
		{"trailing" + sent, "trailing", []int{8}},
		{"a " + sent + " b", "a b", []int{2}}, // lone sentinel owns its trailing space
		{"line one" + sent + "\nline two", "line one\nline two", []int{8}},
		{"two" + sent + sent + " marks", "two marks", []int{3, 3}},
	}
	for _, c := range cases {
		got, anchors := stripFootnoteSentinels(c.in)
		if got != c.want {
			t.Errorf("strip(%q) text = %q, want %q", c.in, got, c.want)
		}
		if len(anchors) != len(c.anchors) {
			t.Errorf("strip(%q) anchors = %v, want %v", c.in, anchors, c.anchors)
			continue
		}
		for i := range anchors {
			if anchors[i] != c.anchors[i] {
				t.Errorf("strip(%q) anchors = %v, want %v", c.in, anchors, c.anchors)
				break
			}
		}
	}
}
