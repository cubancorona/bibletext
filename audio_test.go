package bibletext

import "testing"

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
		"web":  {"web-williams"},
		"webc": {"web-williams"}, // same WEB text for the 66 recorded books
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
	// WEB-Catholic Tobit → TTS of the chapter text (deuterocanon, no recording).
	a = audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "Tobit", CurrentChapter: 1, Bible: bd})
	if a.Kind != audioTTS || a.Text != "The book of the words of Tobit" {
		t.Errorf("webc Tobit 1: got %+v, want TTS of the verse", a)
	}
	// WEB-Catholic Daniel 13 (Susanna) → TTS: the Greek Daniel runs to 14 chapters
	// but the WEB narration ends at 12, so the extra chapters must not offer a
	// stream with no file behind it.
	a = audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: "Daniel", CurrentChapter: 13, Bible: bd})
	if a.Kind != audioTTS || a.Text != "There was a man living in Babylon whose name was Joakim" {
		t.Errorf("webc Daniel 13: got %+v, want TTS of the verse", a)
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
		{0.0, 0},  // intro — nothing highlighted yet
		{4.1, 1},  // exactly at verse 1's start
		{13.9, 1}, // in the gap before verse 2 — hold the previous verse
		{20.0, 2},
		{99.0, 3}, // past the last verse — hold it to the end
	} {
		if got := verseAtTime(vs, c.t); got != c.want {
			t.Errorf("verseAtTime(%.1f) = %d, want %d", c.t, got, c.want)
		}
	}
	if got := verseAtTime(nil, 5); got != 0 {
		t.Errorf("verseAtTime(nil) = %d, want 0", got)
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
