package bibletext

// A drag that runs on into the heading below its verse is cited from that
// verse. A heading is drawn after the text of the verse above it and before
// the next verse's number, so a drag from a verse's last words that runs past
// its full stop takes the heading's first letter or first words too, and the
// Apple and Android panes span it to the verse above, the verse whose number
// is the last before each end. The chapter's prose holds no heading, so the
// span's verses do not hold the selection, and the search found it in the
// chapter at its first copy of any kind: the same last word followed by a
// word that begins as the heading does, elsewhere in the chapter. The end
// repair cut that copy back to the last word where it stood, and that verse
// was cited. The words are the end of the verse above the heading, which is
// where they are placed now (verseEndBeforeHeading).

import (
	"strings"
	"testing"
)

func overrunChapter(book string, chapter int, verses []Verse, heads []Heading) *AppState {
	for i := range verses {
		verses[i].BookName, verses[i].Book, verses[i].Chapter = book, book, chapter
	}
	return &AppState{Bible: &BibleData{
		Books:    []string{book},
		Verses:   map[string]map[int][]Verse{book: {chapter: verses}},
		Headings: map[string]map[int][]Heading{book: {chapter: heads}},
	}, CurrentBook: book, CurrentChapter: chapter}
}

// The four shapes, on invented wording: a drag from a verse's last word on to
// the first letter of the heading below, with the chapter's first copy of the
// two in a later verse and in an earlier one, and on to the heading's whole
// first word, whose copy stands inside a longer word. Each was cited from the
// verse of that copy. A fifth runs through one heading on to the first letter
// of a second drawn under it, which the chapter holds nowhere, and was shared
// with both headings' words as Scripture. The control is the same drag
// stopped at the verse's full stop, which was cited from its own verse
// already.
func TestADragRunOnIntoTheHeadingBelowCitesItsOwnVerse(t *testing.T) {
	for _, c := range []struct {
		name     string
		st       *AppState
		raw      string
		verse    int
		wantText string
		wantCite string
	}{
		{
			name: "the heading's first letter, the chapter's copy in a later verse",
			st: overrunChapter("2 Chronicles", 3, []Verse{
				{Verse: 3, Text: "These are the measures he laid down for building the house."},
				{Verse: 4, Text: "The porch in front of the house was as wide as the house, and he lined its inside with pure gold."},
				{Verse: 5, Text: "He paneled the main hall with cypress and adorned it with palm trees and chains."},
				{Verse: 6, Text: "He adorned the house with precious stones for beauty."},
				{Verse: 7, Text: "He covered the beams, the thresholds and the doors with gold. Then he carved cherubim on the walls."},
			}, []Heading{{Text: "The Great Hall", Style: "s", BeforeVerse: 5}}),
			raw: "gold.\nT", verse: 4, wantText: "gold.", wantCite: "2 Chronicles 3:4",
		},
		{
			name: "the heading's first letter, the copy in the chapter's last verse",
			st: overrunChapter("Philippians", 1, []Verse{
				{Verse: 1, Text: "From servants of Christ Jesus, to all the holy ones in the city, with the overseers and deacons."},
				{Verse: 2, Text: "Grace to you and peace from God our Father and the Lord Jesus Christ."},
				{Verse: 3, Text: "I thank my God whenever I remember you."},
				{Verse: 27, Text: "Only let your way of life be worthy of the good news of Christ. Then, whether I come or not, I may hear of you."},
			}, []Heading{{Text: "Thanksgiving and Prayer", Style: "s", BeforeVerse: 3}}),
			raw: "Christ.\nT", verse: 2, wantText: "Christ.", wantCite: "Philippians 1:2",
		},
		{
			name: "the heading's whole first word, its copy inside a longer word",
			st: overrunChapter("2 Chronicles", 36, []Verse{
				{Verse: 14, Text: "The leaders of the priests and the people were unfaithful, and defiled the house he had set apart in Jerusalem."},
				{Verse: 15, Text: "God sent word to them by his messengers again and again."},
				{Verse: 19, Text: "They burned the house and broke down the wall of Jerusalem. They set all its palaces on fire."},
			}, []Heading{{Text: "The Fall of the City", Style: "s", BeforeVerse: 15}}),
			raw: "Jerusalem.\nThe", verse: 14, wantText: "Jerusalem.", wantCite: "2 Chronicles 36:14",
		},
		{
			name: "an acrostic letter's first letter, the copy in an earlier verse",
			st: overrunChapter("Psalms", 119, []Verse{
				{Verse: 22, Text: "Take scorn and contempt away from me, for I have kept your statutes. Then I will not be put to shame."},
				{Verse: 63, Text: "I am a companion of all who fear you, of those who keep your precepts."},
				{Verse: 64, Text: "The earth is full of your loving kindness. Teach me your statutes."},
				{Verse: 65, Text: "You have treated your servant well, according to your word."},
			}, []Heading{{Text: "TETH", Style: "qa", BeforeVerse: 65}}),
			raw: "statutes.\nT", verse: 64, wantText: "statutes.", wantCite: "Psalms 119:64",
		},
		{
			name: "through the first of two headings on to the second's first letter",
			st: overrunChapter("Proverbs", 22, []Verse{
				{Verse: 16, Text: "Whoever oppresses the poor to gain more, or gives to the rich, comes only to poverty."},
				{Verse: 17, Text: "Turn your ear and hear the words of the wise, and set your heart on my teaching."},
			}, []Heading{
				{Text: "Words of the Wise", Style: "s", BeforeVerse: 17},
				{Text: "Saying 1", Style: "s", BeforeVerse: 17},
			}),
			raw: "poverty.\nWords of the Wise\nS", verse: 16, wantText: "poverty.", wantCite: "Proverbs 22:16",
		},
	} {
		st := c.st
		span := selSpanFromNative(c.verse, c.verse)
		corpus, spans := chapterProse(st)
		end := -1
		for _, sp := range spans {
			if sp.verse == c.verse {
				end = sp.end
			}
		}
		if end < 0 || !strings.HasSuffix(corpus[:end], c.wantText) {
			t.Fatalf("%s: fixture: verse %d does not end %q", c.name, c.verse, c.wantText)
		}

		// The control: the drag stopped at the full stop.
		if _, cite, _, _ := prepareShareQuote(st, c.wantText, span); cite != c.wantCite {
			t.Fatalf("%s: control: %q alone, span %d–%d, cited %q, want %q; the fixture says nothing about headings",
				c.name, c.wantText, c.verse, c.verse, cite, c.wantCite)
		}

		text, cite, at, baseLen := prepareShareQuote(st, c.raw, span)
		if text != c.wantText || cite != c.wantCite {
			t.Errorf("%s: %q, span %d–%d, shared (%q, %q), want (%q, %q)",
				c.name, c.raw, c.verse, c.verse, text, cite, c.wantText, c.wantCite)
		}
		if at != end-len(c.wantText) || baseLen != len(c.wantText) {
			t.Errorf("%s: %q placed at %d+%d, want %d+%d, the end of verse %d",
				c.name, c.raw, at, baseLen, end-len(c.wantText), len(c.wantText), c.verse)
		}
		if lo, hi := linkVersesForSelection(st, c.raw, span); lo != c.verse || hi != c.verse {
			t.Errorf("%s: %q, span %d–%d, linked verses %d–%d", c.name, c.raw, c.verse, c.verse, lo, hi)
		}

		// The selection menu's share, end to end.
		var sent []string
		prev := shareTextOut
		shareTextOut = func(s string) { sent = append(sent, s) }
		dispatchSelectionAction(st, selActionShareCite, c.raw, span)
		shareTextOut = prev
		line := citationLine(c.wantCite, st.currentVersion().Name)
		if len(sent) != 1 || !strings.HasSuffix(sent[0], line) {
			t.Errorf("%s: Share with citation sent %q, want one message ending %q", c.name, sent, line)
		}
	}
}

// A heading can open as the next one does. A drag from a verse's last word
// through the heading below it to the next verse's number ends with that
// number, after the heading's own figure; with both taken off as verse
// numbers, what is left ends with the word the next heading opens with too.
// The drag never reached that heading, and is cited from the verse it began
// in, whose last word is all of the drag that is Scripture.
func TestADragThroughAHeadingToTheNumberBelowIsNotTakenForTheNextHeading(t *testing.T) {
	st := overrunChapter("Proverbs", 23, []Verse{
		{Verse: 8, Text: "You will bring up the morsel you have eaten and waste your kind words."},
		{Verse: 9, Text: "Do not speak in the hearing of a fool, for he will despise your words."},
		{Verse: 10, Text: "Do not move an old boundary stone."},
	}, []Heading{
		{Text: "Saying 10", Style: "s", BeforeVerse: 9},
		{Text: "Saying 11", Style: "s", BeforeVerse: 10},
	})
	for _, raw := range []string{"words.\nSaying 10\n9", "words.\nSaying 10\n9 "} {
		if _, cite, _, _ := prepareShareQuote(st, raw, selSpanFromNative(8, 9)); cite != "Proverbs 23:8" {
			t.Errorf("%q, span 8–9, cited %q, want \"Proverbs 23:8\"", raw, cite)
		}
	}
}

// overrunHeadings are invented headings for the sweep, opening as headings
// and the sentences under them often do: an article, a pronoun, a name.
var overrunHeadings = []string{
	"The Crowds Follow",
	"He Teaches by the Sea",
	"A Question about the Law",
	"When the Hour Came",
	"Jesus Goes Up to the City",
	"They Ask for a Sign",
}

// Every verse of the embedded WEB Gospels with an invented heading below it,
// dragged from its last one, two and three words on to the heading's first
// letter, first word and first two words, with the span the Apple and Android
// panes report, the verse above the heading. Each must be cited from that
// verse alone and placed inside it.
//
// Where the verse itself holds what the drag hands over, its last word and
// then a word that opens as the heading does ("him. He" in Matthew 3:16), the
// text and the span are those of a drag inside the verse too, and the verse's
// copy is taken, as it was before: the right verse, at another place in it.
// Those are counted apart, and held exactly so that a change shows.
func TestADragRunOnIntoTheHeadingBelowSweep(t *testing.T) {
	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	step := 1
	if testing.Short() {
		step = 7
	}
	drags, wrong, inVerse, shown := 0, 0, 0, 0
	for _, book := range seed.Books {
		for _, ch := range seed.GetChapterNumbersForBook(book) {
			bd := &BibleData{
				Books:    []string{book},
				Verses:   map[string]map[int][]Verse{book: {ch: seed.GetChapter(book, ch)}},
				Headings: map[string]map[int][]Heading{book: {ch: nil}},
			}
			st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch}
			corpus, spans := chapterProse(st)
			for vi := 1; vi < len(spans); vi++ {
				text := overrunHeadings[(ch+vi)%len(overrunHeadings)]
				bd.Headings[book][ch] = append(bd.Headings[book][ch], Heading{Text: text, Style: "s", BeforeVerse: spans[vi].verse})
			}
			for vi := 0; vi+1 < len(spans); vi += step {
				sp := spans[vi]
				head := overrunHeadings[(ch+vi+1)%len(overrunHeadings)]
				headWords := strings.Fields(head)
				pieces := []string{head[:1], headWords[0], headWords[0] + " " + headWords[1]}
				words := strings.Fields(corpus[sp.start:sp.end])
				for k := 1; k <= 3 && k <= len(words); k++ {
					tail := strings.Join(words[len(words)-k:], " ")
					from := sp.end - len(tail)
					if corpus[from:sp.end] != tail {
						t.Fatalf("%s %d:%d: the last %d words are not the verse's end", book, ch, sp.verse, k)
					}
					want := verseRangeCitation(book, ch, sp.verse, sp.verse)
					span := selSpanFromNative(sp.verse, sp.verse)
					for _, piece := range pieces {
						raw := tail + "\n" + piece
						drags++
						text, cite, at, baseLen := prepareShareQuote(st, raw, span)
						if cite == want && at >= from && at+baseLen <= sp.end {
							continue
						}
						if cite == want && at >= sp.start && at+baseLen <= sp.end && strings.Contains(corpus[sp.start:sp.end], sharedText(collapseSpaces(raw))) {
							inVerse++
							continue
						}
						wrong++
						if shown < 5 {
							shown++
							t.Logf("%q, span %d–%d: shared %q at %d+%d, cited %s; want %s inside %d–%d",
								raw, sp.verse, sp.verse, text, at, baseLen, cite, want, from, sp.end)
						}
					}
				}
			}
		}
	}
	t.Logf("%d drags, %d wrong, %d placed at the verse's own copy of what was dragged", drags, wrong, inVerse)
	if !testing.Short() && drags != 33198 {
		t.Fatalf("%d drags, want 33,198; the sweep did not cover the Gospels as it did", drags)
	}
	if wrong != 0 {
		t.Errorf("%d of %d drags into the heading below were cited from another verse, or placed outside it", wrong, drags)
	}
	if !testing.Short() && inVerse != 6 {
		t.Errorf("%d drags placed at their verse's own copy of what was dragged; recorded 6", inVerse)
	}
}
