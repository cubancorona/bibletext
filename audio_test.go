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
		{"Psalms", 5, "https://ebible.org/webaudio/Psalm005.mp3", true},   // 3-digit pad
		{"Psalms", 119, "https://ebible.org/webaudio/Psalm119.mp3", true}, // 3-digit pad
		{"Jude", 1, "https://ebible.org/webaudio/Jude.mp3", true},         // single-file
		{"3 John", 1, "https://ebible.org/webaudio/3John.mp3", true},      // single-file
		{"Genesis", 1, "", false},                                         // not recorded
		{"Tobit", 1, "", false},                                           // deuterocanon
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
	// BSB John 20 → TTS (different translation, recordings wouldn't match).
	if a := audioForChapter(&AppState{CurrentVersion: "bsb", CurrentBook: "John", CurrentChapter: 20, Bible: bd}); a.Kind != audioTTS {
		t.Errorf("BSB John 20: want TTS, got kind %d", a.Kind)
	}
}
