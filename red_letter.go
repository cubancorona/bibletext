package bibletext

// Red-letter mode: render the words of Christ in red. Rendering is trivial (a
// themed <span class="wj"> in buildChapterHTML); the data is a table of verse
// ranges where Jesus is speaking.
//
// COVERAGE: this is a curated set of the principal red-letter passages — the
// major discourses and best-known sayings, which account for the great bulk of
// Christ's words. It is deliberately easy to extend or replace: drop a complete,
// authoritative list (e.g. derived from the WEB's \wj USFM markers) into
// wordsOfChristRanges and nothing else changes. Ranges are chosen to avoid the
// obvious narrative interruptions ("his disciples asked him…"), but a curated set
// will never be perfect; treat it as a study aid, not a critical text.

import "fyne.io/fyne/v2"

const prefRedLetter = "reading.redLetter"

// redLetterEnabled reports the persisted toggle (off by default). nil-safe so it
// works in tests with no running app.
func redLetterEnabled() bool {
	if app := fyne.CurrentApp(); app != nil {
		return app.Preferences().Bool(prefRedLetter)
	}
	return false
}

func setRedLetterEnabled(v bool) {
	if app := fyne.CurrentApp(); app != nil {
		app.Preferences().SetBool(prefRedLetter, v)
	}
}

// wjRange is an inclusive verse range (within a single book) spoken by Christ.
type wjRange struct {
	startCh, startV int
	endCh, endV     int
}

// pos packs chapter+verse into one comparable integer (verse < 1000 always).
func pos(ch, v int) int { return ch*1000 + v }

func (r wjRange) contains(ch, v int) bool {
	p := pos(ch, v)
	return p >= pos(r.startCh, r.startV) && p <= pos(r.endCh, r.endV)
}

// wordsOfChristRanges maps a book to the verse ranges spoken by Christ. Curated
// principal passages (see file header) — extend freely.
var wordsOfChristRanges = map[string][]wjRange{
	"Matthew": {
		{5, 3, 7, 27},   // Sermon on the Mount
		{10, 5, 10, 42}, // Sending the Twelve
		{11, 7, 11, 30},
		{13, 3, 13, 9}, {13, 11, 13, 23}, {13, 24, 13, 33}, {13, 37, 13, 52}, // Kingdom parables
		{16, 24, 16, 28},
		{18, 3, 18, 35},
		{20, 1, 20, 16},
		{22, 37, 22, 40},
		{23, 2, 23, 39},  // Woes
		{24, 4, 25, 46},  // Olivet Discourse
		{28, 18, 28, 20}, // Great Commission
	},
	"Mark": {
		{4, 3, 4, 32}, // Parables
		{8, 34, 8, 38},
		{13, 5, 13, 37}, // Olivet Discourse
		{16, 15, 16, 18},
	},
	"Luke": {
		{6, 20, 6, 49}, // Sermon on the Plain
		{8, 5, 8, 18},
		{10, 2, 10, 24},
		{11, 2, 11, 13},
		{12, 22, 12, 59},
		{15, 3, 15, 32}, // Lost sheep / coin / son
		{16, 1, 16, 31},
		{21, 8, 21, 36}, // Olivet Discourse
	},
	"John": {
		{3, 3, 3, 8}, {3, 10, 3, 21}, // To Nicodemus
		{4, 21, 4, 26},
		{6, 26, 6, 58}, // Bread of Life
		{8, 12, 8, 58},
		{10, 1, 10, 18}, {10, 25, 10, 38}, // Good Shepherd
		{14, 1, 16, 33}, // Upper Room Discourse
		{17, 1, 17, 26}, // High-Priestly Prayer
	},
	"Revelation": {
		{1, 17, 1, 20},
		{2, 1, 3, 22}, // Letters to the seven churches
		{22, 12, 22, 16},
	},
}

// isWordsOfChrist reports whether (book, chapter, verse) is within a curated
// words-of-Christ range.
func isWordsOfChrist(book string, chapter, verse int) bool {
	for _, r := range wordsOfChristRanges[book] {
		if r.contains(chapter, verse) {
			return true
		}
	}
	return false
}
