package bibletext

// Red-letter mode: render the words of Christ in red. Rendering is trivial (a
// themed <span class="wj"> in buildChapterHTML); the data is a table of verse
// ranges where Jesus is speaking.
//
// COVERAGE: wordsOfChristRanges (red_letter_data.go) is GENERATED from the World
// English Bible's own USFM \wj (words-of-Jesus) markers (eBible.org, public
// domain), so coverage is complete and authoritative — every verse the WEB marks
// as Christ speaking is included (the Gospels, plus Acts, 1–2 Corinthians,
// 1 Timothy, and Revelation). It is verse-granular: a verse is flagged when any
// part of it falls inside a \wj span, so a verse mixing narration and speech
// ("Jesus said to her, …") is reddened whole. To change coverage, regenerate the
// data file from the WEB USFM rather than editing ranges by hand.

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

// wordsOfChristRanges (the WEB \wj data) lives in the generated red_letter_data.go.

// isWordsOfChrist reports whether (book, chapter, verse) is within a
// words-of-Christ range.
func isWordsOfChrist(book string, chapter, verse int) bool {
	for _, r := range wordsOfChristRanges[book] {
		if r.contains(chapter, verse) {
			return true
		}
	}
	return false
}
