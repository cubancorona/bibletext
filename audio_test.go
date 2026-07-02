package bibletext

import "testing"

func TestEbibleAudioURL(t *testing.T) {
	cases := []struct {
		book    string
		chapter int
		want    string
		ok      bool
	}{
		{"John", 20, "https://ebible.org/webaudio/John20.mp3", true},
		{"Matthew", 5, "https://ebible.org/webaudio/Mat05.mp3", true},
		{"Psalms", 2, "https://ebible.org/webaudio/Psalm002.mp3", true}, // 3-digit pad; last recorded psalm
		{"Jude", 1, "https://ebible.org/webaudio/Jude.mp3", true},       // single-file
		{"3 John", 1, "https://ebible.org/webaudio/3John.mp3", true},    // single-file
		{"Genesis", 1, "", false},                                       // not recorded
		{"Tobit", 1, "", false},                                         // deuterocanon
		// Chapters beyond eBible's actual coverage must report "no recording" —
		// the server 404s them, which used to leave a silently dead play button
		// (the source menu claimed a narration and TTS never took over).
		{"Psalms", 3, "", false},                                        // Ps 3-150 not on the server
		{"Psalms", 119, "", false},                                      // ditto
		{"Mark", 7, "https://ebible.org/webaudio/Mark07.mp3", true},     // last recorded Mark chapter
		{"Mark", 8, "", false},                                          // Mark 8-16 not on the server
		{"Romans", 5, "https://ebible.org/webaudio/Romans05.mp3", true}, // last recorded Romans chapter
		{"Romans", 6, "", false},                                        // Romans 6-16 not on the server
	}
	for _, c := range cases {
		got, ok := ebibleAudioURL(c.book, c.chapter)
		if got != c.want || ok != c.ok {
			t.Errorf("ebibleAudioURL(%q,%d) = (%q,%v), want (%q,%v)", c.book, c.chapter, got, ok, c.want, c.ok)
		}
	}
}

func TestVersionUsesEBibleAudio(t *testing.T) {
	for v, want := range map[string]bool{"web": true, "webc": true, "bsb": false, "nrsv": false} {
		if got := versionUsesEBibleAudio(v); got != want {
			t.Errorf("versionUsesEBibleAudio(%q) = %v, want %v", v, got, want)
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
	if a.Kind != audioRecorded || a.URL != "https://ebible.org/webaudio/John20.mp3" || a.Title != "John 20" {
		t.Errorf("WEB John 20: got %+v, want recorded John20.mp3", a)
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
	if a := audioForChapter(&AppState{CurrentVersion: "bsb", CurrentBook: "John", CurrentChapter: 20, Bible: bd}); a.Kind != audioRecorded || a.URL != "https://openbible.com/audio/hays/BSB_43_Jhn_020_H.mp3" {
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
		{"John", 3, "https://openbible.com/audio/hays/BSB_43_Jhn_003_H.mp3", true},
		{"Genesis", 1, "https://openbible.com/audio/hays/BSB_01_Gen_001_H.mp3", true},
		{"Psalms", 23, "https://openbible.com/audio/hays/BSB_19_Psa_023_H.mp3", true},
		{"Titus", 2, "https://openbible.com/audio/hays/BSB_56_Tts_002_H.mp3", true}, // non-obvious abbr
		{"Revelation", 22, "https://openbible.com/audio/hays/BSB_66_Rev_022_H.mp3", true},
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
