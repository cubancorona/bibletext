package bibletext

import "testing"

func TestIsWordsOfChrist(t *testing.T) {
	in := []struct {
		book string
		ch   int
		v    int
	}{
		{"Matthew", 5, 9},  // Sermon on the Mount
		{"Matthew", 7, 27}, // last verse of the Sermon
		{"John", 3, 16},    // to Nicodemus (3:10-21)
		{"John", 14, 6},    // Upper Room Discourse (14:1-16:33)
		{"John", 17, 26},   // High-Priestly Prayer end
		{"Revelation", 3, 20},
	}
	for _, c := range in {
		if !isWordsOfChrist(c.book, c.ch, c.v) {
			t.Errorf("%s %d:%d should be words of Christ", c.book, c.ch, c.v)
		}
	}

	out := []struct {
		book string
		ch   int
		v    int
	}{
		{"Matthew", 7, 28}, // narration right after the Sermon
		{"John", 1, 1},     // prologue narration
		{"John", 3, 9},     // Nicodemus speaking (gap between 3:8 and 3:10)
		{"Genesis", 1, 1},  // not a red-letter book
	}
	for _, c := range out {
		if isWordsOfChrist(c.book, c.ch, c.v) {
			t.Errorf("%s %d:%d should NOT be words of Christ", c.book, c.ch, c.v)
		}
	}
}
