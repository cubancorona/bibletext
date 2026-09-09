package bibletext

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

const testAudioHost = "https://github.com/cubancorona/bibletext-audio/releases/download/"

func TestWEBAudioURL(t *testing.T) {
	cases := []struct {
		book    string
		chapter int
		want    string
		ok      bool
	}{
		{"John", 20, testAudioHost + "web-williams-nt-v1/WEB_43_020.mp3", true},
		{"Matthew", 5, testAudioHost + "web-williams-nt-v1/WEB_40_005.mp3", true},
		{"Genesis", 1, testAudioHost + "web-williams-ot-v1/WEB_01_001.mp3", true},
		{"Psalms", 119, testAudioHost + "web-williams-ot-v1/WEB_19_119.mp3", true}, // complete — no per-book bounds anymore
		{"Jude", 1, testAudioHost + "web-williams-nt-v1/WEB_65_001.mp3", true},
		{"Revelation", 22, testAudioHost + "web-williams-nt-v1/WEB_66_022.mp3", true},
		{"Daniel", 12, testAudioHost + "web-williams-ot-v1/WEB_27_012.mp3", true}, // last recorded chapter
		{"Daniel", 13, "", false}, // Greek Daniel (Susanna): WEBC renders it, the narration ends at 12
		{"Daniel", 14, "", false}, // Greek Daniel (Bel and the Dragon)
		{"John", 22, "", false},   // past the end of the book
		{"Tobit", 1, "", false},   // deuterocanon: no WEB recording
		{"John", 0, "", false},    // nonsense chapter
	}
	for _, c := range cases {
		got, ok := webAudioURL(c.book, c.chapter)
		if got != c.want || ok != c.ok {
			t.Errorf("webAudioURL(%q,%d) = (%q,%v), want (%q,%v)", c.book, c.chapter, got, ok, c.want, c.ok)
		}
	}
}

func TestRecordingsFor(t *testing.T) {
	for v, want := range map[string][]string{
		"web": {"web-williams"},
		// The WEB-Catholic adds a second, complementary recording for its Greek
		// books; Williams stays FIRST so he wins every chapter he actually read.
		"webc": {"web-williams", webbeRecordingID},
		"bsb":  {"bsb-hays"},
		"nrsv": nil,
	} {
		recs := recordingsFor(v)
		if len(recs) != len(want) {
			t.Errorf("recordingsFor(%q) returned %d recordings, want %d", v, len(recs), len(want))
			continue
		}
		for i, r := range recs {
			if r.id != want[i] {
				t.Errorf("recordingsFor(%q)[%d].id = %q, want %q", v, i, r.id, want[i])
			}
			if r.narrator == "" || r.urlFor == nil {
				t.Errorf("recordingsFor(%q)[%d] missing narrator or urlFor", v, i)
			}
		}
	}
	if _, ok := recordingByID("web", "bsb-hays"); ok {
		t.Error("recordingByID must not resolve another version's recording")
	}
	if r, ok := recordingByID("webc", "web-williams"); !ok || r.narrator != "David Williams" {
		t.Errorf("recordingByID(webc, web-williams) = (%+v, %v), want the Williams recording", r, ok)
	}
}

func TestAudioForChapter(t *testing.T) {
	bd := &BibleData{
		Books: []string{"John", "Daniel", "Tobit"},
		Verses: map[string]map[int][]Verse{
			"John": {20: {{Text: "Now on the first day of the week"}, {Text: "Mary Magdalene went"}}},
			"Daniel": {
				12: {{Text: "At that time Michael shall stand up"}},
				13: {{Text: "There was a man living in Babylon whose name was Joakim"}},
			},
			"Tobit": {1: {{Text: "The book of the words of Tobit"}}},
		},
	}
	// WEB John 20 → recorded, carrying the recording id + narrator credit.
	a := audioForChapter(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20, Bible: bd})
	if a.Kind != audioRecorded || a.URL != testAudioHost+"web-williams-nt-v1/WEB_43_020.mp3" || a.Title != "John 20" {
		t.Errorf("WEB John 20: got %+v, want recorded WEB_43_020.mp3", a)
	}
	if a.RecordingID != "web-williams" || a.Subtitle != "World English Bible · David Williams" {
		t.Errorf("WEB John 20: RecordingID=%q Subtitle=%q, want web-williams / narrator credit", a.RecordingID, a.Subtitle)
	}
	// WEB-Catholic John 20 → recorded too (same WEB text).
	if a := audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "John", CurrentChapter: 20, Bible: bd}); a.Kind != audioRecorded {
		t.Errorf("webc John 20: want recorded, got kind %d", a.Kind)
	}
	// WEB-Catholic Tobit → the synthetic recording. Williams never read the
	// deuterocanon; what must never happen is HIS recording answering here.
	a = audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "Tobit", CurrentChapter: 1, Bible: bd})
	if a.Kind != audioRecorded || a.RecordingID != webbeRecordingID {
		t.Errorf("webc Tobit 1: got kind=%d rec=%q, want the synthetic recording", a.Kind, a.RecordingID)
	}
	// WEB-Catholic Daniel 13 (Susanna) → likewise. The WEB narration stops at
	// chapter 12, so this must never stream a Williams URL with no file behind it.
	a = audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "Daniel", CurrentChapter: 13, Bible: bd})
	if a.Kind != audioRecorded || a.RecordingID != webbeRecordingID {
		t.Errorf("webc Daniel 13: got kind=%d rec=%q, want the synthetic recording", a.Kind, a.RecordingID)
	}
	// ...while WEB-Catholic Daniel 12 still streams the recording.
	if a := audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "Daniel", CurrentChapter: 12, Bible: bd}); a.Kind != audioRecorded || a.URL != testAudioHost+"web-williams-ot-v1/WEB_27_012.mp3" {
		t.Errorf("webc Daniel 12: got %+v, want recorded WEB_27_012.mp3", a)
	}
	// BSB John 20 → recorded (the BSB has its own complete narration).
	if a := audioForChapter(&AppState{CurrentVersion: "bsb", CurrentBook: "John", CurrentChapter: 20, Bible: bd}); a.Kind != audioRecorded || a.URL != testAudioHost+"bsb-hays-nt-v1/BSB_43_Jhn_020_H.mp3" {
		t.Errorf("BSB John 20: got %+v, want recorded BSB_43_Jhn_020_H.mp3", a)
	}
}

func TestWEBCTextMismatchExclusions(t *testing.T) {
	// The WEB-Catholic displays the GREEK Esther (a different underlying book —
	// no verse correspondence with the Hebrew Esther the Williams recording
	// narrates) and a Greek Daniel 3 carrying the Prayer of Azariah and the Song
	// of the Three as verses 24–90. Offering the Williams recording there would play
	// different words than the screen shows and highlight the wrong verses, so webc
	// routes those chapters away from him — to the synthetic recording of the Greek
	// text — while the plain WEB, whose Esther and Daniel 3 ARE what he read, keeps
	// him.
	bd := &BibleData{
		Books: []string{"Esther", "Daniel"},
		Verses: map[string]map[int][]Verse{
			"Esther": {1: {{Text: "In the second year of the reign of Ahasuerus"}}},
			"Daniel": {3: {{Text: "Nebuchadnezzar the king made an image of gold"}}},
		},
	}
	// The durable invariant is WHICH recording answers: Williams must never narrate
	// the Greek Esther or Greek Daniel 3, whatever else may cover them. (Before the
	// synthetic recording existed these chapters fell to read-aloud; now they are
	// covered, and asserting the recording id keeps testing the actual rule.)
	for _, c := range []struct {
		version string
		book    string
		chapter int
		wantRec string // "" = read-aloud
	}{
		{"webc", "Esther", 1, webbeRecordingID},
		{"webc", "Esther", 10, webbeRecordingID},
		{"webc", "Daniel", 3, webbeRecordingID},
		{"webc", "Daniel", 2, "web-williams"},
		{"webc", "Daniel", 4, "web-williams"},
		{"web", "Esther", 1, "web-williams"},
		{"web", "Esther", 10, "web-williams"},
		{"web", "Daniel", 3, "web-williams"},
	} {
		a := audioForChapter(&AppState{CurrentVersion: c.version, CurrentBook: c.book, CurrentChapter: c.chapter, Bible: bd})
		if a.RecordingID != c.wantRec {
			t.Errorf("%s %s %d: recording = %q, want %q", c.version, c.book, c.chapter, a.RecordingID, c.wantRec)
		}
		if c.version == "webc" && a.RecordingID == "web-williams" && c.book == "Esther" {
			t.Errorf("%s %s %d: the Greek Esther must never use the Williams recording", c.version, c.book, c.chapter)
		}
	}
	// The exclusion belongs to the webc registry entry, not the URL builder —
	// the WEB set still has the files, and webc keeps every Daniel chapter
	// Williams narrated against matching text.
	if _, ok := webAudioURL("Esther", 5); !ok {
		t.Error("webAudioURL(Esther 5) should still map — the exclusion is webc's")
	}
	if _, ok := webcAudioURL("Daniel", 12); !ok {
		t.Error("webcAudioURL(Daniel 12) should still map — only chapter 3 diverges")
	}
	// A remembered source preference resolves through the same exclusion: the
	// webc registry's web-williams entry declines Esther.
	if rec, ok := recordingByID("webc", "web-williams"); !ok {
		t.Fatal("webc should still register web-williams")
	} else if _, ok := rec.urlFor("Esther", 1); ok {
		t.Error("webc's web-williams entry must decline the Greek Esther")
	}
}

func TestBSBAudioURL(t *testing.T) {
	cases := []struct {
		book    string
		chapter int
		want    string
		ok      bool
	}{
		{"John", 3, testAudioHost + "bsb-hays-nt-v1/BSB_43_Jhn_003_H.mp3", true},
		{"Genesis", 1, testAudioHost + "bsb-hays-ot-v1/BSB_01_Gen_001_H.mp3", true},
		{"Psalms", 23, testAudioHost + "bsb-hays-ot-v1/BSB_19_Psa_023_H.mp3", true},
		{"Titus", 2, testAudioHost + "bsb-hays-nt-v1/BSB_56_Tts_002_H.mp3", true}, // non-obvious abbr
		{"Revelation", 22, testAudioHost + "bsb-hays-nt-v1/BSB_66_Rev_022_H.mp3", true},
		{"Daniel", 12, testAudioHost + "bsb-hays-ot-v1/BSB_27_Dan_012_H.mp3", true}, // last recorded chapter
		{"Daniel", 13, "", false}, // past the end of the book
		{"Tobit", 1, "", false},   // deuterocanon: no BSB recording
	}
	for _, c := range cases {
		got, ok := bsbAudioURL(c.book, c.chapter)
		if got != c.want || ok != c.ok {
			t.Errorf("bsbAudioURL(%q,%d) = (%q,%v), want (%q,%v)", c.book, c.chapter, got, ok, c.want, c.ok)
		}
	}
	// Every canonical 66-book name must map (complete coverage).
	if n := len(bsbAudioBooks); n != 66 {
		t.Errorf("bsbAudioBooks has %d entries, want 66", n)
	}
}

func TestChapterTimings(t *testing.T) {
	// Both bundled tables must load (keyed by recording id) and cover the full canon.
	for _, recID := range []string{"bsb-hays", "web-williams"} {
		vs := chapterTimings(recID, "Genesis", 1)
		if len(vs) == 0 {
			t.Fatalf("chapterTimings(%q, Genesis, 1) is empty", recID)
		}
		if vs[0].verse != 1 || vs[0].start <= 0 {
			t.Errorf("%s Genesis 1 first entry = %+v, want verse 1 with a positive start", recID, vs[0])
		}
		if got := chapterTimings(recID, "Revelation", 22); len(got) == 0 {
			t.Errorf("chapterTimings(%q, Revelation, 22) is empty — table incomplete?", recID)
		}
	}
	// The WEB-Catholic reaches the Williams tables through the registry, not an alias.
	if rec, ok := recordingByID("webc", "web-williams"); !ok {
		t.Error("webc should register the web-williams recording")
	} else if got := chapterTimings(rec.id, "John", 3); len(got) == 0 {
		t.Errorf("webc's recording %q has no John 3 timings", rec.id)
	}
	// Unknown recording ids highlight nothing.
	if got := chapterTimings("nope", "John", 3); got != nil {
		t.Errorf("chapterTimings(nope) = %v, want nil", got)
	}
}

func TestVerseAtTime(t *testing.T) {
	vs := []verseTiming{{1, 4.1, 13.5}, {2, 14.4, 25.2}, {3, 26.2, 40.9}}
	for _, c := range []struct {
		t    float64
		want int
	}{
		{0.0, readAlongNone}, // intro — nothing highlighted yet
		{4.1, 1},             // exactly at verse 1's start
		{13.9, 1},            // in the gap before verse 2 — hold the previous verse
		{20.0, 2},
		{99.0, 3}, // past the last verse — hold it to the end
	} {
		if got := verseAtTime(vs, c.t); got != c.want {
			t.Errorf("verseAtTime(%.1f) = %d, want %d", c.t, got, c.want)
		}
	}
	if got := verseAtTime(nil, 5); got != readAlongNone {
		t.Errorf("verseAtTime(nil) = %d, want readAlongNone", got)
	}

	// A titled psalm's table leads with the title row: readAlongTitle during
	// the title, verse 1 from its own start. CONTROL: the same table without
	// the title row answers readAlongNone at the same instant — the 0 comes
	// from the row, not from the sentinel.
	titled := []verseTiming{{0, 1.3, 6.3}, {1, 8.5, 13.9}, {2, 14.9, 18.8}}
	for _, c := range []struct {
		t    float64
		want int
	}{
		{0.0, readAlongNone}, {1.3, readAlongTitle}, {2.0, readAlongTitle}, {7.0, readAlongTitle}, {8.5, 1}, {16.0, 2},
	} {
		if got := verseAtTime(titled, c.t); got != c.want {
			t.Errorf("titled verseAtTime(%.1f) = %d, want %d", c.t, got, c.want)
		}
	}
	if got := verseAtTime(titled[1:], 2.0); got != readAlongNone {
		t.Errorf("without the title row, verseAtTime(2.0) = %d, want readAlongNone", got)
	}
}

func TestSpeechVerseOffsets(t *testing.T) {
	bd := &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{
			"John": {3: {
				{Verse: 1, Text: "  First verse.  "},  // trimmed
				{Verse: 2, Text: "   "},               // empty after trim — skipped entirely
				{Verse: 3, Text: "Third — “quoted”."}, // non-ASCII, still 1 UTF-16 unit each
				{Verse: 4, Text: "Then 𝕏 spoke."},     // astral char = 2 UTF-16 units
				{Verse: 5, Text: "Last."},
			}},
		},
	}
	state := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 3, Bible: bd}

	got := speechVerseOffsets(state)
	// Mirror chapterSpeechText: "First verse." + " " + "Third — “quoted”." + " " + "Then 𝕏 spoke." + " " + "Last."
	want := []verseTiming{
		{verse: 1, start: 0},
		{verse: 3, start: float64(utf16Len("First verse.") + 1)},
		{verse: 4, start: float64(utf16Len("First verse.") + 1 + utf16Len("Third — “quoted”.") + 1)},
		{verse: 5, start: float64(utf16Len("First verse.") + 1 + utf16Len("Third — “quoted”.") + 1 + utf16Len("Then 𝕏 spoke.") + 1)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d offsets, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].verse != want[i].verse || got[i].start != want[i].start {
			t.Errorf("offset[%d] = {v%d @%v}, want {v%d @%v}", i, got[i].verse, got[i].start, want[i].verse, want[i].start)
		}
	}
	// The astral char must count 2 units: sanity-pin utf16Len itself.
	if utf16Len("𝕏") != 2 || utf16Len("“x”") != 3 {
		t.Errorf("utf16Len wrong: 𝕏=%d (want 2), “x”=%d (want 3)", utf16Len("𝕏"), utf16Len("“x”"))
	}
	// The lookup treats offsets like times: mid-verse-3 range reports verse 3.
	if v := verseAtTime(got, float64(utf16Len("First verse.")+1+2)); v != 3 {
		t.Errorf("verseAtTime(mid verse 3) = %d, want 3", v)
	}
}

// --- S18: the title is the read-along's verse-0 row ---------------------------

// A titled psalm's spoken text leads with its title, as a sentence of its own,
// and the offsets file it under verse 0 ahead of verse 1. The BSB/WEB shape
// already ends in a full stop; the NKJV shape does not and gains one. A title
// note never rides along.
func TestSpeechTextReadsTheTitle(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {
			3:  {{Verse: 1, Text: "LORD, how my foes have increased!"}, {Verse: 2, Text: "Many say of me."}},
			23: {{Verse: 1, Text: "The LORD is my shepherd."}},
		}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {
			3: {Text: "A Psalm of David when he fled from Absalom his son.",
				Footnotes: []Footnote{{Text: "A titlenoteprobe gloss.", Caller: "+"}}},
			23: {Text: "A Psalm of David"}, // the NKJV shape: no full stop of its own
		}},
	}
	for _, tc := range []struct {
		chapter  int
		sentence string
		verses   string
	}{
		{3, "A Psalm of David when he fled from Absalom his son.", "LORD, how my foes have increased! Many say of me."},
		{23, "A Psalm of David.", "The LORD is my shepherd."},
	} {
		state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: tc.chapter}
		got := chapterSpeechText(state)
		if want := tc.sentence + " " + tc.verses; got != want {
			t.Errorf("Psalm %d speech = %q, want %q", tc.chapter, got, want)
		}
		if strings.Contains(got, "titlenoteprobe") {
			t.Errorf("Psalm %d: a title note reached the spoken chapter", tc.chapter)
		}
		offs := speechVerseOffsets(state)
		if len(offs) < 2 || offs[0] != (verseTiming{verse: readAlongTitle, start: 0}) ||
			offs[1] != (verseTiming{verse: 1, start: float64(utf16Len(tc.sentence) + 1)}) {
			t.Errorf("Psalm %d offsets = %+v, want the title at 0 and verse 1 right after it", tc.chapter, offs)
		}
	}
}

// speechSentence's exact rule, with the shapes that must NOT gain a stop.
func TestSpeechSentence(t *testing.T) {
	for in, want := range map[string]string{
		"A Psalm of David.":    "A Psalm of David.",
		"  A Psalm of David  ": "A Psalm of David.",
		"A Psalm of David":     "A Psalm of David.",
		"To the choirmaster!":  "To the choirmaster!",
		"Why, O LORD?":         "Why, O LORD?",
		"He said, “Sing.”":     "He said, “Sing.”",
		"(A Psalm.)":           "(A Psalm.)",
		"A Psalm of David’s":   "A Psalm of David’s.",
		"":                     "",
		"   ":                  "",
		"”":                    "”.",
	} {
		if got := speechSentence(in); got != want {
			t.Errorf("speechSentence(%q) = %q, want %q", in, got, want)
		}
	}
}

// speechLockstepMismatch checks that every offset row lands on the start of
// its segment's text in the spoken string — slicing the string by UTF-16
// units, the synthesizer's own ruler. Returns "" when aligned, else what
// disagrees.
func speechLockstepMismatch(text string, offs []verseTiming, segs []speechSegment) string {
	if len(offs) != len(segs) {
		return fmt.Sprintf("%d offsets for %d segments", len(offs), len(segs))
	}
	units := utf16.Encode([]rune(text))
	for i, o := range offs {
		if o.verse != segs[i].verse {
			return fmt.Sprintf("row %d is verse %d, its segment is verse %d", i, o.verse, segs[i].verse)
		}
		at := int(o.start)
		if at < 0 || at > len(units) {
			return fmt.Sprintf("row %d offset %d is outside the %d-unit text", i, at, len(units))
		}
		rest := string(utf16.Decode(units[at:]))
		first := strings.Fields(segs[i].text)[0]
		if !strings.HasPrefix(rest, first) {
			return fmt.Sprintf("row %d (verse %d) offset %d lands on %.20q, not on %q", i, o.verse, at, rest, first)
		}
	}
	return ""
}

// The text and the offsets are derived from one walk, and this proves it on a
// fixture where a byte, a rune and a UTF-16 unit are three different counts —
// astral characters and curly quotes in the TITLE as well as the verses, and a
// newline inside a verse. Then each side is perturbed alone, to show the
// checker can fail.
func TestSpeechOffsetsLockstep(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {7: {
			{Verse: 1, Text: "  O LORD my God, in you I take refuge;\nsave me from all who pursue me.  "},
			{Verse: 2, Text: "   "},
			{Verse: 3, Text: "“𝕏” said the accuser — and “𝕐” the accused."},
			{Verse: 4, Text: "Last."},
		}}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {7: {
			Text: "A Shiggaion of David, which he sang to the LORD concerning “𝔻” the Benjamite."}}},
	}
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 7}
	text, offs, segs := chapterSpeechText(state), speechVerseOffsets(state), speechSegments(state)
	if got := speechLockstepMismatch(text, offs, segs); got != "" {
		t.Fatalf("text and offsets disagree: %s", got)
	}
	if len(offs) != 4 || offs[0].verse != readAlongTitle || offs[1].verse != 1 || offs[2].verse != 3 || offs[3].verse != 4 {
		t.Fatalf("offsets = %+v, want rows for the title, 1, 3 and 4", offs)
	}
	// CONTROLS: the text shifted alone, then one offset shifted alone.
	if speechLockstepMismatch(" "+text, offs, segs) == "" {
		t.Error("a shifted text went undetected; the checker proves nothing")
	}
	bent := append([]verseTiming(nil), offs...)
	bent[1].start++
	if speechLockstepMismatch(text, bent, segs) == "" {
		t.Error("a shifted offset went undetected; the checker proves nothing")
	}
}

// Every untitled chapter — Psalm 119, whose ALEPH is an acrostic label and not
// a title; any non-Psalm; a cache from before the Superscriptions field; a
// whitespace-only title — speaks EXACTLY what it did before the title joined:
// the old algorithm is written out here as the oracle. CONTROL: a titled psalm
// differs from that oracle and leads with the verse-0 row.
func TestSpeechTextUntitledChaptersUnchanged(t *testing.T) {
	oracle := func(verses []Verse) (string, []verseTiming) {
		var b strings.Builder
		var offs []verseTiming
		off := 0
		for _, v := range verses {
			t := strings.TrimSpace(v.Text)
			if t == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte(' ')
				off++
			}
			offs = append(offs, verseTiming{verse: v.Verse, start: float64(off)})
			b.WriteString(t)
			off += utf16Len(t)
		}
		return b.String(), offs
	}
	psalm119 := []Verse{
		{Verse: 1, Text: "Blessed are those whose ways are blameless."},
		{Verse: 2, Text: "  "},
		{Verse: 3, Text: "They do no wrong — “𝕏”."},
	}
	psalm23 := []Verse{{Verse: 1, Text: "The LORD is my shepherd."}}
	john3 := []Verse{{Verse: 16, Text: "For God so loved the world."}}
	bd := &BibleData{
		Books:  []string{"Psalms", "John"},
		Verses: map[string]map[int][]Verse{"Psalms": {119: psalm119, 23: psalm23, 150: psalm23}, "John": {3: john3}},
		Headings: map[string]map[int][]Heading{"Psalms": {119: {
			{Text: "ALEPH", Style: "acrostic", BeforeVerse: 1}}}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {
			23:  {Text: "A Psalm of David."},
			150: {Text: "   "},
		}},
	}
	check := func(name string, state *AppState, verses []Verse) {
		t.Helper()
		wantText, wantOffs := oracle(verses)
		if got := chapterSpeechText(state); got != wantText {
			t.Errorf("%s: speech = %q, want the pre-title text %q", name, got, wantText)
		}
		got := speechVerseOffsets(state)
		if !reflect.DeepEqual(got, wantOffs) {
			t.Errorf("%s: offsets = %+v, want the pre-title offsets %+v", name, got, wantOffs)
		}
		for _, o := range got {
			if o.verse == readAlongTitle {
				t.Errorf("%s: a verse-0 row with no title", name)
			}
		}
	}
	check("Psalm 119", &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 119}, psalm119)
	check("John 3", &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}, john3)
	check("a whitespace-only title", &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 150}, psalm23)
	old := *bd
	old.Superscriptions = nil // a cache from before the field existed
	check("a pre-field cache", &AppState{Bible: &old, CurrentBook: "Psalms", CurrentChapter: 23}, psalm23)

	// CONTROL: the titled psalm must DIFFER from the oracle, or the oracle
	// proves nothing.
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}
	if wantText, _ := oracle(psalm23); chapterSpeechText(state) == wantText {
		t.Fatal("a titled psalm matched the pre-title oracle; the oracle is vacuous")
	}
	if offs := speechVerseOffsets(state); len(offs) != 2 || offs[0].verse != readAlongTitle {
		t.Fatalf("the titled psalm's offsets = %+v, want the title row first", offs)
	}
}

// The loader keeps a verse-0 row like any other and still drops malformed
// rows; the lookup returns readAlongNone before the first row whatever that
// row is. Fed a synthetic blob, so the bundled tables are not what is tested.
func TestParseTimingsAcceptsTitleRow(t *testing.T) {
	got, err := parseTimings([]byte(`{"Psalms":{"3":[[0,3.4,6.9],[1,7.2,15.2],[1,7.2],[9,1,2,3]]}}`))
	if err != nil {
		t.Fatal(err)
	}
	rows := got["Psalms"]["3"]
	// CONTROL: the 2- and 4-element rows are dropped, so 2 rows, not 4.
	if len(rows) != 2 || rows[0] != (verseTiming{0, 3.4, 6.9}) || rows[1].verse != 1 {
		t.Fatalf("rows = %+v, want the title row then verse 1", rows)
	}
	if verseAtTime(rows, 1.0) != readAlongNone || verseAtTime(rows, 5.0) != readAlongTitle || verseAtTime(rows, 7.2) != 1 {
		t.Errorf("lookup over the title row: %d %d %d", verseAtTime(rows, 1.0), verseAtTime(rows, 5.0), verseAtTime(rows, 7.2))
	}
	if _, err := parseTimings([]byte(`{`)); err == nil {
		t.Error("a broken table parsed")
	}
}
