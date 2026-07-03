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
		{"Tobit", 1, "", false}, // deuterocanon: no WEB recording
		{"John", 0, "", false},  // nonsense chapter
	}
	for _, c := range cases {
		got, ok := webAudioURL(c.book, c.chapter)
		if got != c.want || ok != c.ok {
			t.Errorf("webAudioURL(%q,%d) = (%q,%v), want (%q,%v)", c.book, c.chapter, got, ok, c.want, c.ok)
		}
	}
}

func TestVersionUsesWEBAudio(t *testing.T) {
	for v, want := range map[string]bool{"web": true, "webc": true, "bsb": false, "nrsv": false} {
		if got := versionUsesWEBAudio(v); got != want {
			t.Errorf("versionUsesWEBAudio(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestAudioForChapter(t *testing.T) {
	bd := &BibleData{
		Books: []string{"John", "Tobit"},
		Verses: map[string]map[int][]Verse{
			"John":  {20: {{Text: "Now on the first day of the week"}, {Text: "Mary Magdalene went"}}},
			"Tobit": {1: {{Text: "The book of the words of Tobit"}}},
		},
	}
	// WEB John 20 → recorded.
	a := audioForChapter(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20, Bible: bd})
	if a.Kind != audioRecorded || a.URL != testAudioHost+"web-williams-nt-v1/WEB_43_020.mp3" || a.Title != "John 20" {
		t.Errorf("WEB John 20: got %+v, want recorded WEB_43_020.mp3", a)
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
		{"Tobit", 1, "", false}, // deuterocanon: no BSB recording
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
	// Both bundled tables must load and cover the full canon.
	for _, version := range []string{"bsb", "web"} {
		vs := chapterTimings(version, "Genesis", 1)
		if len(vs) == 0 {
			t.Fatalf("chapterTimings(%q, Genesis, 1) is empty", version)
		}
		if vs[0].verse != 1 || vs[0].start <= 0 {
			t.Errorf("%s Genesis 1 first entry = %+v, want verse 1 with a positive start", version, vs[0])
		}
		if got := chapterTimings(version, "Revelation", 22); len(got) == 0 {
			t.Errorf("chapterTimings(%q, Revelation, 22) is empty — table incomplete?", version)
		}
	}
	// The WEB-Catholic shares the WEB tables (same text, same verse numbers).
	if got, want := chapterTimings("webc", "John", 3), chapterTimings("web", "John", 3); len(got) == 0 || len(got) != len(want) {
		t.Errorf("webc John 3 timings (%d entries) should mirror web (%d)", len(got), len(want))
	}
	// Versions without a bundled recording table highlight nothing.
	if got := chapterTimings("nrsv", "John", 3); got != nil {
		t.Errorf("chapterTimings(nrsv) = %v, want nil", got)
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
