package bibletext

// Verse of the day: a subtle header icon (a small sparkle, far right) opens a
// calm little card with one grace-filled, Christ-centred passage that rotates by
// the calendar day. It is intentionally NOT a feed or a page — just a quiet
// daily pointer back into the Word, with a "Read in context" jump.
//
// FOUR THINGS ABOUT THE ROTATION ARE DELIBERATE, and each replaced something
// that was found to be wrong:
//
//   - The DAY is a civil date, counted from the reader's own year-month-day
//     (verseOfDayNumber). It used to be the Unix day of local midnight, which
//     is the previous UTC day for every zone east of Greenwich while on summer
//     time — so the United Kingdom repeated a verse the day after the clocks
//     went forward and skipped one the day after they went back. On the phone
//     it was worse: Go pins time.Local to UTC under GOOS=ios, so the card
//     changed at 01:00 BST. That half is fixed where the zone is set
//     (refreshLocalTimeZone), not here.
//
//   - The ORDER is a frozen permutation of the curated list (verseOfDaySequence),
//     so consecutive days move between books. The list itself stays grouped
//     by book, which is the shape a person edits. In canonical order the
//     rotation spent 58 straight days in the Psalms and 86% of consecutive
//     days in one book.
//
//   - The INDEX runs over the whole list, not over what happens to resolve in
//     the loaded edition. Compacting first meant a fresh install showed one
//     verse on the embedded Gospels and another the moment the download
//     landed, and any edition missing a single entry would have kept its own
//     calendar for ever. An entry that does not resolve falls back to the
//     compacted pick for that day, which is exactly what the seed always saw.
//
//   - An entry is a PASSAGE, one to five verses. Forty-four of the old single
//     verses were clauses of a sentence the next day completed — the Aaronic
//     blessing took three days — and sixty more began or ended mid-sentence.
//     What still does is framed with an ellipsis rather than shown as a
//     typesetting fault (fragmentFrame).
//
// The five passages from the books only the Catholic edition carries have an
// ALTERNATE on the same slot (verseOfDayAlternates), so a 66-book reader sees a
// verse that day too and nobody's calendar moves.

import (
	"fmt"
	"hash/fnv"
	"image/color"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// dayPassage is one entry of the rotation: a verse, or a short run of verses in
// one chapter. Hi of 0 means the single verse Lo, so an entry reads as it did
// when every entry was a single verse. Book names are resolved through
// resolveBookName, so "Psalm"/"Psalms" and an edition's own naming both land.
type dayPassage struct {
	Book    string
	Chapter int
	Lo, Hi  int
}

// hi is the passage's last verse, spelling out the Hi == 0 convention.
func (p dayPassage) hi() int {
	if p.Hi == 0 {
		return p.Lo
	}
	return p.Hi
}

// key names the passage the way the rotation's order is derived from it. It
// is deliberately independent of the edition: the same key on every platform
// and translation is what makes the sequence the same everywhere.
func (p dayPassage) key() string {
	if p.hi() == p.Lo {
		return fmt.Sprintf("%s %d:%d", p.Book, p.Chapter, p.Lo)
	}
	return fmt.Sprintf("%s %d:%d-%d", p.Book, p.Chapter, p.Lo, p.hi())
}

// verseOfDayRefs is the curated rotation, grouped by book for editing. The
// principle of selection, stated so that a later editor can apply it: a
// COMPLETE THOUGHT as printed, TRUE for a stranger reading it cold, and
// POINTING AT CHRIST or at God's character — the gospel, who Jesus is, and
// the hope, peace and promises found in him. Drawn from across the canon; a
// few books have nothing that meets the bar as a stand-alone passage and are
// left out rather than represented for the sake of it.
//
// EDITING RE-DEALS THE YEAR. Adding or removing an entry changes the modulus,
// so every reader's day moves once. Replacing an entry in place moves nothing
// but that entry. The golden test (TestVerseOfDayOrderIsFrozen) exists so that
// a re-deal is a deliberate diff, not a side effect; when the list changes,
// regenerate testdata/verse_of_day_snapshot.json with
// scripts/gen-votd-snapshot.py and update the golden slots together.
var verseOfDayRefs = []dayPassage{
	// Genesis
	{"Genesis", 1, 1, 0}, {"Genesis", 1, 27, 0}, {"Genesis", 15, 6, 0}, {"Genesis", 28, 15, 0}, {"Genesis", 50, 20, 0},
	// Exodus
	{"Exodus", 14, 14, 0}, {"Exodus", 15, 2, 0}, {"Exodus", 33, 14, 0}, {"Exodus", 34, 6, 0},
	// Leviticus
	{"Leviticus", 26, 12, 0},
	// Numbers
	{"Numbers", 6, 24, 26}, {"Numbers", 23, 19, 0},
	// Deuteronomy
	{"Deuteronomy", 6, 5, 0}, {"Deuteronomy", 7, 9, 0}, {"Deuteronomy", 31, 6, 0}, {"Deuteronomy", 31, 8, 0},
	// Joshua
	{"Joshua", 1, 9, 0}, {"Joshua", 24, 15, 0},
	// Ruth
	{"Ruth", 1, 16, 0},
	// 1 Samuel
	{"1 Samuel", 2, 2, 0}, {"1 Samuel", 16, 7, 0},
	// 2 Samuel
	{"2 Samuel", 22, 31, 0},
	// 1 Kings
	{"1 Kings", 8, 56, 0},
	// 2 Kings
	{"2 Kings", 6, 16, 0},
	// 1 Chronicles
	{"1 Chronicles", 16, 11, 0}, {"1 Chronicles", 16, 34, 0}, {"1 Chronicles", 29, 11, 0},
	// 2 Chronicles
	{"2 Chronicles", 20, 15, 0},
	// Nehemiah
	{"Nehemiah", 8, 10, 0},
	// Judith
	{"Judith", 9, 11, 0},
	// Esther
	{"Esther", 4, 14, 0},
	// Job
	{"Job", 19, 25, 0}, {"Job", 23, 10, 0}, {"Job", 42, 2, 0},
	// Psalms
	{"Psalms", 1, 1, 2}, {"Psalms", 16, 8, 0}, {"Psalms", 16, 11, 0}, {"Psalms", 18, 2, 0}, {"Psalms", 19, 1, 0}, {"Psalms", 19, 14, 0}, {"Psalms", 23, 1, 0}, {"Psalms", 23, 4, 0}, {"Psalms", 23, 6, 0}, {"Psalms", 27, 1, 0}, {"Psalms", 27, 4, 0}, {"Psalms", 27, 14, 0}, {"Psalms", 28, 7, 0}, {"Psalms", 30, 5, 0}, {"Psalms", 31, 24, 0}, {"Psalms", 32, 8, 0}, {"Psalms", 34, 8, 0}, {"Psalms", 34, 18, 0}, {"Psalms", 37, 4, 6}, {"Psalms", 42, 1, 0}, {"Psalms", 42, 11, 0}, {"Psalms", 46, 1, 0}, {"Psalms", 46, 10, 0}, {"Psalms", 51, 10, 0}, {"Psalms", 55, 22, 0}, {"Psalms", 56, 3, 0}, {"Psalms", 62, 1, 0}, {"Psalms", 63, 1, 0}, {"Psalms", 73, 26, 0}, {"Psalms", 84, 11, 0}, {"Psalms", 90, 12, 0}, {"Psalms", 91, 1, 2}, {"Psalms", 91, 4, 0}, {"Psalms", 94, 19, 0}, {"Psalms", 100, 5, 0}, {"Psalms", 103, 1, 0}, {"Psalms", 103, 2, 5}, {"Psalms", 103, 8, 0}, {"Psalms", 103, 12, 0}, {"Psalms", 107, 1, 0}, {"Psalms", 118, 24, 0}, {"Psalms", 119, 11, 0}, {"Psalms", 119, 105, 0}, {"Psalms", 121, 1, 2}, {"Psalms", 121, 7, 0}, {"Psalms", 130, 5, 0}, {"Psalms", 138, 8, 0}, {"Psalms", 139, 14, 0}, {"Psalms", 139, 23, 24}, {"Psalms", 143, 8, 0}, {"Psalms", 145, 8, 9}, {"Psalms", 147, 3, 0}, {"Psalms", 150, 6, 0},
	// Proverbs
	{"Proverbs", 3, 5, 6}, {"Proverbs", 4, 23, 0}, {"Proverbs", 11, 25, 0}, {"Proverbs", 16, 3, 0}, {"Proverbs", 16, 9, 0}, {"Proverbs", 18, 10, 0}, {"Proverbs", 19, 21, 0}, {"Proverbs", 29, 25, 0}, {"Proverbs", 30, 5, 0},
	// Ecclesiastes
	{"Ecclesiastes", 3, 11, 0}, {"Ecclesiastes", 12, 13, 0},
	// Song of Solomon
	{"Song of Solomon", 2, 4, 0},
	// Wisdom
	{"Wisdom", 3, 1, 3}, {"Wisdom", 11, 24, 26},
	// Sirach
	{"Sirach", 2, 10, 11},
	// Isaiah
	{"Isaiah", 6, 8, 0}, {"Isaiah", 9, 6, 0}, {"Isaiah", 12, 2, 0}, {"Isaiah", 25, 1, 0}, {"Isaiah", 26, 3, 0}, {"Isaiah", 30, 21, 0}, {"Isaiah", 40, 8, 0}, {"Isaiah", 40, 28, 29}, {"Isaiah", 40, 30, 31}, {"Isaiah", 41, 10, 0}, {"Isaiah", 41, 13, 0}, {"Isaiah", 43, 1, 2}, {"Isaiah", 43, 19, 0}, {"Isaiah", 46, 4, 0}, {"Isaiah", 53, 4, 6}, {"Isaiah", 54, 10, 0}, {"Isaiah", 55, 6, 0}, {"Isaiah", 55, 8, 0}, {"Isaiah", 55, 10, 11}, {"Isaiah", 58, 11, 0}, {"Isaiah", 61, 1, 0}, {"Isaiah", 64, 8, 0},
	// Jeremiah
	{"Jeremiah", 17, 7, 0}, {"Jeremiah", 29, 11, 13}, {"Jeremiah", 31, 3, 0}, {"Jeremiah", 32, 17, 0}, {"Jeremiah", 33, 3, 0},
	// Lamentations
	{"Lamentations", 3, 22, 23}, {"Lamentations", 3, 25, 0},
	// Baruch
	{"Baruch", 4, 22, 0},
	// Ezekiel
	{"Ezekiel", 36, 26, 0},
	// Daniel
	{"Daniel", 3, 17, 18},
	// Hosea
	{"Hosea", 6, 6, 0}, {"Hosea", 14, 4, 0},
	// Joel
	{"Joel", 2, 13, 0}, {"Joel", 2, 32, 0},
	// Amos
	{"Amos", 5, 4, 0},
	// Jonah
	{"Jonah", 2, 9, 0},
	// Micah
	{"Micah", 6, 8, 0}, {"Micah", 7, 7, 0},
	// Nahum
	{"Nahum", 1, 7, 0},
	// Habakkuk
	{"Habakkuk", 3, 17, 18},
	// Zephaniah
	{"Zephaniah", 3, 17, 0},
	// Zechariah
	{"Zechariah", 4, 6, 0},
	// Malachi
	{"Malachi", 3, 6, 0},
	// Matthew
	{"Matthew", 4, 4, 0}, {"Matthew", 5, 3, 0}, {"Matthew", 5, 6, 0}, {"Matthew", 5, 8, 0}, {"Matthew", 5, 14, 0}, {"Matthew", 5, 16, 0}, {"Matthew", 5, 44, 45}, {"Matthew", 6, 19, 21}, {"Matthew", 6, 33, 34}, {"Matthew", 7, 7, 0}, {"Matthew", 7, 12, 0}, {"Matthew", 11, 28, 30}, {"Matthew", 16, 24, 0}, {"Matthew", 18, 20, 0}, {"Matthew", 19, 26, 0}, {"Matthew", 22, 37, 39}, {"Matthew", 28, 6, 0}, {"Matthew", 28, 19, 20},
	// Mark
	{"Mark", 9, 23, 0}, {"Mark", 10, 27, 0}, {"Mark", 10, 45, 0}, {"Mark", 12, 30, 31}, {"Mark", 16, 15, 0},
	// Luke
	{"Luke", 1, 37, 0}, {"Luke", 2, 10, 11}, {"Luke", 4, 18, 19}, {"Luke", 6, 31, 0}, {"Luke", 6, 37, 0}, {"Luke", 11, 9, 0}, {"Luke", 12, 34, 0}, {"Luke", 19, 10, 0},
	// John
	{"John", 1, 1, 0}, {"John", 1, 12, 13}, {"John", 1, 14, 0}, {"John", 1, 29, 0}, {"John", 3, 16, 17}, {"John", 4, 13, 14}, {"John", 6, 35, 0}, {"John", 8, 12, 0}, {"John", 8, 32, 0}, {"John", 8, 36, 0}, {"John", 10, 10, 0}, {"John", 10, 11, 0}, {"John", 10, 28, 0}, {"John", 11, 25, 0}, {"John", 13, 34, 0}, {"John", 14, 1, 3}, {"John", 14, 6, 0}, {"John", 14, 27, 0}, {"John", 15, 5, 0}, {"John", 15, 13, 0}, {"John", 16, 33, 0}, {"John", 17, 3, 0}, {"John", 20, 29, 0},
	// Acts
	{"Acts", 1, 8, 0}, {"Acts", 2, 38, 0}, {"Acts", 4, 12, 0}, {"Acts", 16, 31, 0}, {"Acts", 17, 28, 0}, {"Acts", 20, 24, 0},
	// Romans
	{"Romans", 1, 16, 0}, {"Romans", 3, 23, 24}, {"Romans", 5, 1, 2}, {"Romans", 5, 8, 0}, {"Romans", 6, 23, 0}, {"Romans", 8, 1, 0}, {"Romans", 8, 11, 0}, {"Romans", 8, 18, 0}, {"Romans", 8, 28, 0}, {"Romans", 8, 31, 0}, {"Romans", 8, 37, 39}, {"Romans", 10, 9, 10}, {"Romans", 10, 13, 0}, {"Romans", 12, 1, 2}, {"Romans", 12, 12, 0}, {"Romans", 12, 21, 0}, {"Romans", 15, 13, 0},
	// 1 Corinthians
	{"1 Corinthians", 1, 18, 0}, {"1 Corinthians", 2, 9, 0}, {"1 Corinthians", 6, 19, 20}, {"1 Corinthians", 10, 13, 0}, {"1 Corinthians", 13, 4, 7}, {"1 Corinthians", 13, 13, 0}, {"1 Corinthians", 15, 57, 58}, {"1 Corinthians", 16, 13, 14},
	// 2 Corinthians
	{"2 Corinthians", 1, 3, 4}, {"2 Corinthians", 3, 18, 0}, {"2 Corinthians", 4, 16, 18}, {"2 Corinthians", 5, 7, 0}, {"2 Corinthians", 5, 17, 0}, {"2 Corinthians", 5, 21, 0}, {"2 Corinthians", 9, 8, 0}, {"2 Corinthians", 12, 9, 0},
	// Galatians
	{"Galatians", 2, 20, 0}, {"Galatians", 5, 1, 0}, {"Galatians", 5, 22, 23}, {"Galatians", 6, 9, 0},
	// Ephesians
	{"Ephesians", 1, 7, 0}, {"Ephesians", 2, 4, 5}, {"Ephesians", 2, 8, 10}, {"Ephesians", 3, 20, 21}, {"Ephesians", 4, 32, 0}, {"Ephesians", 6, 10, 11},
	// Philippians
	{"Philippians", 1, 6, 0}, {"Philippians", 2, 3, 4}, {"Philippians", 2, 5, 8}, {"Philippians", 3, 14, 0}, {"Philippians", 4, 4, 0}, {"Philippians", 4, 6, 7}, {"Philippians", 4, 8, 0}, {"Philippians", 4, 12, 13}, {"Philippians", 4, 19, 0},
	// Colossians
	{"Colossians", 1, 16, 17}, {"Colossians", 2, 6, 7}, {"Colossians", 3, 1, 2}, {"Colossians", 3, 12, 13}, {"Colossians", 3, 15, 0}, {"Colossians", 3, 17, 0}, {"Colossians", 3, 23, 24},
	// 1 Thessalonians
	{"1 Thessalonians", 5, 11, 0}, {"1 Thessalonians", 5, 16, 18}, {"1 Thessalonians", 5, 24, 0},
	// 2 Thessalonians
	{"2 Thessalonians", 3, 3, 0},
	// 1 Timothy
	{"1 Timothy", 2, 5, 6}, {"1 Timothy", 4, 12, 0}, {"1 Timothy", 6, 6, 0}, {"1 Timothy", 6, 12, 0},
	// 2 Timothy
	{"2 Timothy", 1, 7, 0}, {"2 Timothy", 2, 15, 0}, {"2 Timothy", 3, 16, 17}, {"2 Timothy", 4, 7, 0},
	// Titus
	{"Titus", 2, 11, 14}, {"Titus", 3, 4, 7},
	// Hebrews
	{"Hebrews", 4, 12, 0}, {"Hebrews", 4, 16, 0}, {"Hebrews", 6, 19, 20}, {"Hebrews", 10, 23, 25}, {"Hebrews", 11, 1, 0}, {"Hebrews", 11, 6, 0}, {"Hebrews", 12, 1, 2}, {"Hebrews", 13, 5, 6}, {"Hebrews", 13, 8, 0},
	// James
	{"James", 1, 2, 4}, {"James", 1, 5, 0}, {"James", 1, 12, 0}, {"James", 1, 17, 0}, {"James", 1, 22, 0}, {"James", 4, 7, 8}, {"James", 5, 16, 0},
	// 1 Peter
	{"1 Peter", 1, 3, 5}, {"1 Peter", 2, 9, 0}, {"1 Peter", 2, 24, 0}, {"1 Peter", 3, 15, 0}, {"1 Peter", 4, 8, 0}, {"1 Peter", 5, 6, 7}, {"1 Peter", 5, 8, 0}, {"1 Peter", 5, 10, 0},
	// 2 Peter
	{"2 Peter", 1, 3, 4}, {"2 Peter", 3, 9, 0}, {"2 Peter", 3, 18, 0},
	// 1 John
	{"1 John", 1, 7, 0}, {"1 John", 1, 9, 0}, {"1 John", 3, 1, 0}, {"1 John", 3, 16, 0}, {"1 John", 4, 4, 0}, {"1 John", 4, 7, 8}, {"1 John", 4, 9, 10}, {"1 John", 4, 16, 0}, {"1 John", 4, 18, 0}, {"1 John", 4, 19, 0}, {"1 John", 5, 4, 5}, {"1 John", 5, 11, 0}, {"1 John", 5, 14, 0},
	// 2 John
	{"2 John", 1, 3, 0},
	// Jude
	{"Jude", 1, 24, 25},
	// Revelation
	{"Revelation", 1, 8, 0}, {"Revelation", 3, 20, 0}, {"Revelation", 4, 11, 0}, {"Revelation", 5, 11, 12}, {"Revelation", 7, 16, 17}, {"Revelation", 21, 4, 5}, {"Revelation", 22, 13, 0},
}

// verseOfDayAlternates stands in for a passage whose BOOK is absent from the
// loaded edition. Only the Catholic edition carries Judith, Wisdom, Sirach and
// Baruch; on the same slot a 66-book edition shows the alternate, chosen for the
// same thought. The keys are entries of verseOfDayRefs exactly as written there
// (Hi 0 for a single verse); the values are not themselves entries, or a verse
// would come round twice in the year.
var verseOfDayAlternates = map[dayPassage]dayPassage{
	{"Judith", 9, 11, 0}:   {"Psalms", 10, 14, 0},
	{"Wisdom", 3, 1, 3}:    {"Psalms", 116, 15, 0},
	{"Wisdom", 11, 24, 26}: {"Psalms", 36, 7, 0},
	{"Sirach", 2, 10, 11}:  {"Psalms", 22, 4, 5},
	{"Baruch", 4, 22, 0}:   {"Isaiah", 25, 4, 0},
}

// THE ORDER.
//
// Each entry gets a slot from a hash of its key, and the sequence is the
// entries sorted by slot. Two properties matter and a plain shuffle has
// neither: the order must not depend on the SOURCE order (so regrouping the
// file for editing moves nothing), and it must not depend on the language's
// random generator (so a Go release cannot re-deal the year). The finalizer
// after the hash is load-bearing: raw FNV of "Romans 8:37-39" and "Romans
// 8:40" share most of their bits and sort adjacent, which is the very
// clustering the order exists to break.
//
// The seed is frozen. Changing it re-deals every reader's year.
const verseOfDayOrderSeed uint64 = 0x9E3779B97F4A7C15

// passageSlot is the entry's position key: FNV-1a of its name, seeded, then
// MurmurHash3's finalizer to scatter near-identical names.
func passageSlot(p dayPassage) uint64 {
	h := fnv.New64a()
	h.Write([]byte(p.key()))
	x := h.Sum64() ^ verseOfDayOrderSeed
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}

// verseOfDaySequence is verseOfDayRefs in the order the days walk it.
var verseOfDaySequence = orderedVerseOfDay(verseOfDayRefs)

func orderedVerseOfDay(refs []dayPassage) []dayPassage {
	seq := append([]dayPassage(nil), refs...)
	sort.SliceStable(seq, func(i, j int) bool {
		a, b := passageSlot(seq[i]), passageSlot(seq[j])
		if a != b {
			return a < b
		}
		return seq[i].key() < seq[j].key()
	})
	return seq
}

// THE DAY.
//
// verseOfDayNumber counts civil dates: the reader's own year, month and day,
// taken in the reader's zone and then counted as if every day were 86,400
// seconds long. Two consecutive local dates therefore always differ by one,
// whatever the clocks did in between. The formula this replaced divided the
// Unix time of LOCAL MIDNIGHT by a day, which is the previous UTC date for any
// zone east of Greenwich — and on the two nights a year the offset changes, the
// division landed on the same quotient twice (a repeated verse) or skipped one
// (a verse never shown). Every zone whose offset is zero in one season and
// positive in the other has that shape: the British Isles and Portugal among
// them.
//
// verseOfDayNow is the clock, replaceable so a test can stand on a chosen date
// in a chosen zone (the pattern noteNow uses).
var verseOfDayNow = time.Now

func verseOfDayNumber(now time.Time) int64 {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

// dayVerse is a rotation entry resolved against the loaded edition: the
// edition's own name for the book, the verse numbers, and the verses.
type dayVerse struct {
	Book    string
	Chapter int
	Lo, Hi  int
	Verses  []Verse
}

// reference is the citation line under the card, with the en dash the share
// citations use for a range.
func (d dayVerse) reference() string {
	if d.Lo == d.Hi {
		return fmt.Sprintf("%s %d:%d", d.Book, d.Chapter, d.Lo)
	}
	return fmt.Sprintf("%s %d:%d–%d", d.Book, d.Chapter, d.Lo, d.Hi)
}

// text is the passage as it should be DRAWN: each verse as the edition sets it
// (the divine name in small capitals, poem lines kept), one verse per line so
// a run of verses reads as the run it is.
func (d dayVerse) text() string {
	parts := make([]string, 0, len(d.Verses))
	for _, v := range d.Verses {
		parts = append(parts, strings.TrimSpace(smallCapsText(v)))
	}
	return strings.Join(parts, "\n")
}

// resolveDayPassage looks the passage up in the edition. Every verse of a run
// must be present: an edition that omits a verse leaves a hole in its own
// numbering (omitted_verses.go), and a run shown across a hole would read as
// one sentence with a piece missing.
func resolveDayPassage(bd *BibleData, p dayPassage) (dayVerse, bool) {
	if bd == nil {
		return dayVerse{}, false
	}
	book, ok := resolveBookName(bd.Books, p.Book)
	if !ok {
		return dayVerse{}, false
	}
	hi := p.hi()
	verses := make([]Verse, 0, hi-p.Lo+1)
	for n := p.Lo; n <= hi; n++ {
		v := bd.GetVerse(book, p.Chapter, n)
		if v == nil {
			return dayVerse{}, false
		}
		verses = append(verses, *v)
	}
	return dayVerse{Book: book, Chapter: p.Chapter, Lo: p.Lo, Hi: hi, Verses: verses}, true
}

// resolveDayEntry is resolveDayPassage plus the alternate: only when the BOOK is
// absent from the edition, never when a verse is — a missing verse is the
// edition's decision about that passage and the fallback pick covers it.
func resolveDayEntry(bd *BibleData, p dayPassage) (dayVerse, bool) {
	if d, ok := resolveDayPassage(bd, p); ok {
		return d, true
	}
	if bd == nil {
		return dayVerse{}, false
	}
	if _, hasBook := resolveBookName(bd.Books, p.Book); !hasBook {
		if alt, ok := verseOfDayAlternates[p]; ok {
			return resolveDayPassage(bd, alt)
		}
	}
	return dayVerse{}, false
}

// resolvedVerseOfDay returns the rotation compacted to what resolves in the
// loaded edition, in sequence order. It is the FALLBACK, not the index: see
// verseOfTheDayAt.
func resolvedVerseOfDay(state *AppState) []dayVerse {
	if state == nil || state.Bible == nil {
		return nil
	}
	out := make([]dayVerse, 0, len(verseOfDaySequence))
	for _, p := range verseOfDaySequence {
		if d, ok := resolveDayEntry(state.Bible, p); ok {
			out = append(out, d)
		}
	}
	return out
}

// verseOfTheDay picks today's passage.
func verseOfTheDay(state *AppState) (dayVerse, bool) {
	return verseOfTheDayAt(state, verseOfDayNumber(verseOfDayNow()))
}

// verseOfTheDayAt is the pick for a given day number. The day indexes the WHOLE
// sequence, so every edition that carries the entry agrees on it and the
// calendar never depends on what an edition lacks. Only an entry the edition
// cannot show falls back to the compacted list — and to that list's own
// modulus, not to the next entry that resolves: walking forward weights a
// partial edition's entries by the gap before each one, so on the embedded
// Gospels one passage would be shown a dozen days running and the next once.
// The compacted pick keeps every passage the edition can show in rotation.
// Not perfectly evenly — on a partial edition the days split between the two
// paths, so a passage may come round somewhat more or less often than its
// share — but bounded, and a partial edition is a first run before the
// download lands, not a year.
func verseOfTheDayAt(state *AppState, day int64) (dayVerse, bool) {
	if state == nil || state.Bible == nil || len(verseOfDaySequence) == 0 {
		return dayVerse{}, false
	}
	if d, ok := resolveDayEntry(state.Bible, verseOfDaySequence[dayIndex(day, len(verseOfDaySequence))]); ok {
		return d, true
	}
	valid := resolvedVerseOfDay(state)
	if len(valid) == 0 {
		return dayVerse{}, false
	}
	return valid[dayIndex(day, len(valid))], true
}

// dayIndex is day mod n, kept non-negative for dates before the epoch.
func dayIndex(day int64, n int) int {
	i := int(day % int64(n))
	if i < 0 {
		i += n
	}
	return i
}

// fragmentFrame marks a passage that begins or ends mid-sentence so it reads as
// a quotation rather than a fault: a leading "… " where the first letter is
// lower-case, a trailing "…" where the last mark is a letter or a clause
// mark. Opening and closing quotation marks are looked past, not counted; an
// ellipsis that belongs inside a closing quote is put there.
func fragmentFrame(s string) string {
	return frameTail(frameLead(strings.TrimSpace(s)))
}

// frameLead is the leading half: "… " before a lower-case first letter,
// placed inside any opening quotation marks — the ellipsis stands for words
// of the speech, not for narration before it.
func frameLead(t string) string {
	rs := []rune(t)
	i := 0
	for i < len(rs) && strings.ContainsRune("‘“\"'(", rs[i]) {
		i++
	}
	if i < len(rs) && unicode.IsLower(rs[i]) {
		return string(rs[:i]) + "… " + string(rs[i:])
	}
	return t
}

// frameTail is the trailing half: "…" after a last letter or clause mark,
// placed inside any closing quotation marks.
func frameTail(t string) string {
	rs := []rune(t)
	j := len(rs) - 1
	for j >= 0 && strings.ContainsRune("’”\"')", rs[j]) {
		j--
	}
	if j >= 0 && (unicode.IsLetter(rs[j]) || strings.ContainsRune(",;:—–", rs[j])) {
		return string(rs[:j+1]) + "…" + string(rs[j+1:])
	}
	return t
}

// THE MARKS. An edition closes a speech where the speech ends, chapters after
// the verse that opened it, and verse boundaries carry no marks at all — so a
// passage lifted out at verse granularity inherits an orphan: an opening mark
// with no close (Matthew 5:14), or a close with no opening (Exodus 14:14).
// Fifty-odd of the rotation's passages do, in every edition. The share route
// already answers this the way the Bluebook does for a quotation that is only
// part of the excerpt (Rule 5.2(f)(ii), balanceQuoteMarks): retain the marks
// and complete them, so the excerpt is self-contained. The card does the same,
// so the card and the share of the same passage agree on the marks. Not
// stripped: a completed pair says only what is true — these words are speech —
// while an orphan makes a claim the card never honours.
//
// cardMarkBalance is what to add at each end: the double marks by the share
// route's own count, and an opening single ‘ never closed gets its ’. A lone
// ’ is left alone — it is also the apostrophe, and there is no telling.
func cardMarkBalance(text string) (prefix, suffix string) {
	depth, minDepth := 0, 0
	for _, r := range text {
		switch r {
		case '“':
			depth++
		case '”':
			depth--
			if depth < minDepth {
				minDepth = depth
			}
		}
	}
	prefix = strings.Repeat("“", -minDepth)
	suffix = strings.Repeat("”", depth-minDepth)
	single := 0
	for _, r := range text {
		switch r {
		case '‘':
			single++
		case '’':
			if single > 0 {
				single--
			}
		}
	}
	suffix += strings.Repeat("’", single)
	return prefix, suffix
}

// balanceCardMarks is cardMarkBalance applied to a plain string.
func balanceCardMarks(text string) string {
	prefix, suffix := cardMarkBalance(text)
	return prefix + text + suffix
}

// cardRun is one styled stretch of the card's passage: the words, whether
// they are Christ's, whether the translators supplied them, and whether a
// line break precedes it — a poem line, or the next verse of a run.
type cardRun struct {
	Text   string
	Red    bool
	Italic bool
	Break  bool
}

// runs is the passage as the panes would draw it: each verse through the same
// redLetterRuns the reading surfaces use, so the card colours Christ's words
// where the pane does and sets the supplied words in italic, with the divine
// name's small capitals applied on the way. Verses of a run start on their own
// lines, as do a verse's poem lines.
func (d dayVerse) runs(versionID string, red bool) []cardRun {
	var out []cardRun
	for vi, v := range d.Verses {
		for ri, r := range redLetterRuns(versionID, v, red) {
			for li, line := range strings.Split(r.Text, "\n") {
				out = append(out, cardRun{
					Text:   line,
					Red:    r.Red,
					Italic: r.Italic,
					Break:  li > 0 || (ri == 0 && vi > 0),
				})
			}
		}
	}
	return out
}

// runsText is the runs read back as one string, breaks as newlines — what
// the framing and balancing rules look at, and what a test compares.
func runsText(runs []cardRun) string {
	var b strings.Builder
	for _, r := range runs {
		if r.Break {
			b.WriteByte('\n')
		}
		b.WriteString(r.Text)
	}
	return b.String()
}

// frameAndBalance applies the fragment framing and the mark balance to the
// runs, at their ends. Both rules place their marks inside any quotation
// marks they meet — the ellipsis stands for words of the speech — so the two
// commute, and “… for all have sinned…” comes out the same either way round.
func frameAndBalance(runs []cardRun) []cardRun {
	if len(runs) == 0 {
		return runs
	}
	out := append([]cardRun(nil), runs...)
	first, last := 0, len(out)-1
	out[first].Text = frameLead(strings.TrimLeft(out[first].Text, " \t"))
	out[last].Text = frameTail(strings.TrimRight(out[last].Text, " \t"))
	prefix, suffix := cardMarkBalance(runsText(out))
	out[first].Text = prefix + out[first].Text
	out[last].Text += suffix
	return out
}

// iconVerseOfDay is a small filled four-point "sparkle" — a quiet light, not a
// loud badge. Themed so it tracks the foreground colour in light/dark mode.
var iconVerseOfDay = theme.NewThemedResource(fyne.NewStaticResource("votd.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#000000" d="M12 2c.4 4.6 2.4 6.6 7 7-4.6.4-6.6 2.4-7 7-.4-4.6-2.4-6.6-7-7 4.6-.4 6.6-2.4 7-7z"/></svg>`)))

// verseOfDayButton builds the subtle header affordance.
func verseOfDayButton(state *AppState) *widget.Button {
	b := widget.NewButtonWithIcon("", iconVerseOfDay, func() { showVerseOfDay(state) })
	b.Importance = widget.LowImportance
	return b
}

// goToVerse navigates to a verse and highlights it in context. Unlike opening a
// search result, it does not leave a "back to results" trail — this is a direct
// jump (verse of the day, a cross-reference) into the reading view.
func goToVerse(state *AppState, v Verse) {
	goToVerseRange(state, v.BookName, v.Chapter, v.Verse, v.Verse)
}

// goToVerseRange navigates to book+chapter and highlights verses [start, end]
// (end == start for a single verse), scrolling to the first highlighted verse. The
// native overlays wash every .hl verse and scroll to the first; the Fyne reading
// widget scrolls to the start verse.
func goToVerseRange(state *AppState, book string, chapter, start, end int) {
	if end < start {
		end = start
	}
	// BEFORE the navigation: what would this arrival's mark suppress? (See
	// AppState.suppressionTookOpen — the navigation's derive transiently opens
	// the chapter's note, so a later capture would lie.)
	state.captureSuppressionTake(book, chapter)
	selectBook(state, book, false)
	state.CurrentChapter = chapter
	addRecentChapter(state, book, chapter)
	state.setMark(hlVerseOfDay, VerseSpan{
		VersionID: state.currentVersion().ID,
		Book:      book,
		Chapter:   chapter,
		Lo:        start,
		Hi:        end,
	})
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	// The reader asked to BE somewhere, so the view must go there — the same
	// declaration openSearchResultRange and a tapped link already make (see
	// AppState.forceReposition).
	//
	// It used to be carried accidentally, by the mark: the native panes folded
	// the wash into their one render fingerprint, so a new mark forced a rebuild
	// and the rebuild's scroll cadence happened to land on it. Now that a wash
	// change is a live mutation with no scroll of its own — which is right, a
	// wash arriving under the reader's eye must not move the page — the intent
	// has to be said out loud. Saying it also fixes the case that never worked:
	// a Go-to for the verse already marked produced an identical fingerprint,
	// skipped the push entirely, and did nothing at all.
	//
	// Every shipping renderer consumes this intent now. Apple pairs its live
	// wash mutation with a scroll; Android and the styled Windows/Linux pane use
	// it to keep same-chapter scroll carry (including a viewport parked at the
	// top) from outranking the requested verse.
	state.forceReposition = true
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
}

// THE CARD'S TYPE. The passage is set in the reading face at the size the reader
// chose for the reading pane — readingGlyphPx, the same figure the Apple and
// Android panes set their body at (docs/READING_TYPOGRAPHY.md). It used to be
// the toolkit's rich text: the chrome face at the chrome size, whatever the
// reader had chosen, which made the one verse handed to them unbidden the
// smallest Scripture in the app, and one that could not draw the divine name's
// small capitals (the chrome face has no such glyphs).
//
// readingParagraph wraps to whatever width it is given, so the card can be as
// wide as the window allows and a rotation re-wraps it. Rows are canvas.Text
// fragments, which is the only toolkit text that takes a FontSource; a row
// holds several because a run's colour or slant changes mid-line.
type readingParagraph struct {
	widget.BaseWidget
	runs    []cardRun
	size    float32
	color   color.Color
	red     color.Color
	regular fyne.Resource
	italic  fyne.Resource
}

func newReadingParagraph(runs []cardRun, size float32, col, red color.Color, regular, italic fyne.Resource) *readingParagraph {
	p := &readingParagraph{runs: runs, size: size, color: col, red: red, regular: regular, italic: italic}
	p.ExtendBaseWidget(p)
	return p
}

func (p *readingParagraph) CreateRenderer() fyne.WidgetRenderer {
	return &readingParagraphRenderer{p: p}
}

// text is the runs read back as one string, for a test to compare.
func (p *readingParagraph) text() string { return runsText(p.runs) }

// paragraphFragment is one word-piece of the passage with its style and whether
// a space (and so a possible line break) precedes it. A run boundary inside a
// word — the red closes and the black opens with no space between — yields two
// fragments drawn flush, never a break.
type paragraphFragment struct {
	text        string
	red, italic bool
	spaceBefore bool
	breakBefore bool
}

func (p *readingParagraph) fragments() []paragraphFragment {
	var out []paragraphFragment
	for _, r := range p.runs {
		leading := strings.HasPrefix(r.Text, " ")
		words := strings.Fields(r.Text)
		for i, w := range words {
			out = append(out, paragraphFragment{
				text: w, red: r.Red, italic: r.Italic,
				spaceBefore: i > 0 || leading,
				breakBefore: i == 0 && r.Break,
			})
		}
		if len(words) == 0 && r.Break {
			out = append(out, paragraphFragment{breakBefore: true})
		}
		if len(words) > 0 && strings.HasSuffix(r.Text, " ") && len(out) > 0 {
			// The space belongs to the text; carry it to the next fragment.
			out[len(out)-1].text += "\u0000" // marker consumed below
		}
	}
	// Resolve the trailing-space markers into spaceBefore on the follower.
	for i := range out {
		if strings.HasSuffix(out[i].text, "\u0000") {
			out[i].text = strings.TrimSuffix(out[i].text, "\u0000")
			if i+1 < len(out) {
				out[i+1].spaceBefore = true
			}
		}
	}
	return out
}

// readingParagraphRenderer re-wraps only when the width changes; MinSize
// reports the height of the rows at the LAST width it wrapped for, which is the
// contract the sheet-fitting arithmetic relies on (resize to the inner width,
// then read MinSize).
type readingParagraphRenderer struct {
	p      *readingParagraph
	width  float32
	rowH   float32
	spaceW float32
	rows   [][]*canvas.Text
	objs   []fyne.CanvasObject
}

// readingParagraphGuessWidth is the width wrapped for before any layout has
// handed one over — a card's inner width on a phone, so the first MinSize is
// near the truth rather than one word per row.
const readingParagraphGuessWidth = 300

func (r *readingParagraphRenderer) face(italic bool) fyne.Resource {
	if italic && r.p.italic != nil {
		return r.p.italic
	}
	return r.p.regular
}

func (r *readingParagraphRenderer) measure(s string, italic bool) fyne.Size {
	sz, _ := fyne.CurrentApp().Driver().RenderedTextSize(s, r.p.size, fyne.TextStyle{}, r.face(italic))
	return sz
}

func (r *readingParagraphRenderer) wrap(width float32) {
	if width <= 0 {
		width = readingParagraphGuessWidth
	}
	if width == r.width && r.rows != nil {
		return
	}
	r.width = width
	r.rows = r.rows[:0]
	r.objs = r.objs[:0]
	if r.rowH == 0 {
		r.rowH = r.measure("Ag", false).Height
		r.spaceW = r.measure("a a", false).Width - r.measure("aa", false).Width
	}
	var row []*canvas.Text
	x := float32(0)
	newRow := func() {
		r.rows = append(r.rows, row)
		row = nil
		x = 0
	}
	for _, f := range r.p.fragments() {
		if f.breakBefore && (len(row) > 0 || len(r.rows) > 0) {
			newRow()
		}
		if f.text == "" {
			continue
		}
		w := r.measure(f.text, f.italic).Width
		gap := float32(0)
		if f.spaceBefore && len(row) > 0 {
			gap = r.spaceW
		}
		if len(row) > 0 && f.spaceBefore && x+gap+w > width {
			newRow()
			gap = 0
		}
		col := r.p.color
		if f.red {
			col = r.p.red
		}
		t := canvas.NewText(f.text, col)
		t.TextSize = r.p.size
		t.FontSource = r.face(f.italic)
		t.Move(fyne.NewPos(x+gap, 0))
		t.Resize(fyne.NewSize(w, r.rowH))
		row = append(row, t)
		r.objs = append(r.objs, t)
		x += gap + w
	}
	if len(row) > 0 || len(r.rows) == 0 {
		r.rows = append(r.rows, row)
	}
}

func (r *readingParagraphRenderer) Layout(size fyne.Size) {
	r.wrap(size.Width)
	y := float32(0)
	for _, row := range r.rows {
		for _, t := range row {
			t.Move(fyne.NewPos(t.Position().X, y))
		}
		y += r.rowH
	}
}

func (r *readingParagraphRenderer) MinSize() fyne.Size {
	if r.rows == nil {
		r.wrap(r.width)
	}
	return fyne.NewSize(0, float32(len(r.rows))*r.rowH)
}

func (r *readingParagraphRenderer) Refresh() {
	w := r.width
	r.rows = nil
	r.wrap(w)
	r.Layout(r.p.Size())
	canvas.Refresh(r.p)
}

func (r *readingParagraphRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *readingParagraphRenderer) Destroy()                     {}

// rowWidths reports each drawn row's rendered width — what a test compares
// against the width the paragraph was given.
func (r *readingParagraphRenderer) rowWidths() []float32 {
	out := make([]float32, 0, len(r.rows))
	for _, row := range r.rows {
		if len(row) == 0 {
			out = append(out, 0)
			continue
		}
		last := row[len(row)-1]
		out = append(out, last.Position().X+last.Size().Width)
	}
	return out
}

// fragmentColours reports the colour of every drawn fragment, for a test.
func (r *readingParagraphRenderer) fragmentColours() []color.Color {
	var out []color.Color
	for _, row := range r.rows {
		for _, t := range row {
			out = append(out, t.Color)
		}
	}
	return out
}

// cardItalicFont is the reading family's italic cut, cached for the same
// reason styledPaneFont caches the regular: the toolkit's font cache is keyed
// on the resource.
func cardItalicFont() fyne.Resource {
	cardItalicOnce.Do(func() {
		if f := loadReadingFonts(); f != nil {
			cardItalicCached = f.italic
		}
	})
	return cardItalicCached
}

var (
	cardItalicOnce   sync.Once
	cardItalicCached fyne.Resource
)

// votdRemeasure schedules the card's second fit, once the real layout has
// landed, so the card fits the passage snugly. It is a variable so the test
// suite can run the fit synchronously: under the test driver there is no UI
// thread for fyne.Do to marshal to, so the closure would run on the timer's own
// goroutine and its font measurement would race the next test's (go-text's
// glyph cache is single-threaded; the real app measures only on its UI
// thread). The pattern is sheetConsumeClosure's.
var votdRemeasure = func(fit func()) {
	time.AfterFunc(40*time.Millisecond, func() { fyne.Do(fit) })
}

// THE CARD'S AIR. The passage is set at the reading size, and a reading size
// wants the margins a page gives it: the toolkit's two 7pt paddings left the
// kicker sitting on the first line, the reference hugging the last, and the
// glyphs within 12pt of the card's edge on a card that itself stood 36pt in
// from the screen's. These add the inner side margin and the room above and
// below the passage, and the phone margin is narrowed so the card gains
// measure rather than just losing it to padding.
const (
	votdCardPad      = 8  // inner side margin, beyond the surface's own paddings
	votdCardAbove    = 8  // kicker row to passage
	votdCardBelow    = 10 // passage to reference line
	votdCardTop      = 4  // extra at the card's top and bottom edges
	votdScreenMargin = 40 // the card's distance from the screen's edges, both sides together
)

// votdCardInset is how much of the card's width its own frame takes, both
// edges: the surface's padding, the padded container inside it, and the inner
// side margin. The paragraph is resized to the remainder before the first fit,
// so the first measurement wraps at the width it will really be drawn at.
func votdCardInset() float32 {
	return 4*theme.Padding() + 2*votdCardPad
}

// showVerseOfDay presents the calm one-passage card.
func showVerseOfDay(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	d, ok := verseOfTheDay(state)
	if !ok {
		return
	}
	pal := state.pal()

	// The native reading overlay (macOS/iOS) floats above the canvas; drop it
	// while the card is up, restore on close — same dance as the AI panel.
	if state.hideReadingOverlay != nil {
		state.hideReadingOverlay()
	}
	restore := func() {
		if state.showReadingOverlay != nil {
			state.showReadingOverlay()
		}
	}

	kicker := canvas.NewText("Verse of the day", pal.Accent)
	kicker.TextStyle = fyne.TextStyle{Bold: true}
	kicker.TextSize = 12
	// Share sits in the kicker row, not the button row: the row below holds
	// Close and Read in context at the narrowest card width with nothing to
	// spare, and a third control there overflows a small phone. A compact
	// icon beside the kicker costs the card no width and little height.
	//
	// THE PASSAGE ROUTE, NOT THE SELECTION ROUTE. Every stage of the selection
	// share reads the chapter the reader is on, so sharing Psalm 23 from the
	// card over Matthew 5 would cite "Matthew 5"; sharePassageText reads the
	// passage's own chapter and moves the reader nowhere.
	shareBtn := newIconTapButton(state, theme.MailSendIcon(), 17, 22, func() {
		sharePassageText(state, d.Book, d.Chapter, d.Lo, d.Hi)
	})
	top := container.NewBorder(nil, nil, nil, shareBtn,
		container.NewVBox(layout.NewSpacer(), kicker, layout.NewSpacer()))

	// The pane's own cached faces: the toolkit's font cache is keyed on the
	// resource, so handing it a fresh one per card would miss every time.
	body := newReadingParagraph(
		frameAndBalance(d.runs(state.currentVersion().ID, redLetterEnabled())),
		float32(readingGlyphPx()), pal.Text, pal.RedLetter, styledPaneFont(), cardItalicFont())

	ref := canvas.NewText(
		fmt.Sprintf("%s · %s", d.reference(), state.currentVersion().Abbrev),
		pal.TextMuted)
	ref.TextStyle = fyne.TextStyle{Italic: true}
	ref.TextSize = subheadingTextSize

	// Width: comfortable for one passage, capped, with a margin on a phone.
	w := cnv.Size().Width - votdScreenMargin
	if w > 420 {
		w = 420
	}
	if w < 260 {
		w = 260
	}
	// Pre-wrap the passage at the inner width so its height is known.
	body.Resize(fyne.NewSize(w-votdCardInset(), body.MinSize().Height))

	var popup *widget.PopUp
	closeAnd := func(after func()) func() {
		return func() {
			if popup != nil {
				popup.Hide()
			}
			state.dismissSheet = nil
			restore()
			if after != nil {
				after()
			}
		}
	}
	readBtn := widget.NewButton("Read in context", closeAnd(func() {
		goToVerseRange(state, d.Book, d.Chapter, d.Lo, d.Hi)
	}))
	readBtn.Importance = widget.HighImportance
	closeBtn := widget.NewButton("Close", closeAnd(nil))

	// The passage scrolls; the kicker, reference and buttons stay fixed. A long
	// passage at the Extra-large text size on a short canvas (Android
	// split-screen is the worst) pushed the button row past the frame the modal
	// renderer clamps to — buttons on a modal that ignores outside taps. Under
	// the cap the scroll never engages and the card looks exactly as before.
	bodyScroll := container.NewVScroll(container.New(squeezeWidthLayout{}, body))
	passage := container.New(layout.NewCustomPaddedLayout(votdCardAbove, votdCardBelow, 0, 0), bodyScroll)
	content := container.NewBorder(
		top,
		container.NewVBox(ref, widget.NewSeparator(),
			container.NewHBox(layout.NewSpacer(), closeBtn, readBtn)),
		nil, nil,
		passage,
	)
	inner := container.New(layout.NewCustomPaddedLayout(votdCardTop, votdCardTop, votdCardPad, votdCardPad), content)
	card := surface(container.NewPadded(inner), pal.SurfaceAlt, pal.Border, fyne.Size{})
	popup = widget.NewModalPopUp(card, cnv)
	// Escape on the desktop closes the card, rather than falling through to
	// the canvas handler and clearing whatever mark is live underneath it
	// (installShortcuts, ui_desktop.go). Guarded on Visible: rebuildWindow
	// drains popups with Hide alone, and a stale hook re-showing the native
	// reading pane over another sheet is the failure that guard prevents.
	state.dismissSheet = func() {
		if popup != nil && popup.Visible() {
			closeAnd(nil)()
		}
	}
	popup.Show()
	fitVOTD := func() {
		pos, sz := cnv.InteractiveArea()
		maxH := sheetMaxHeight(cnv.Size().Height, pos.Y, sz.Height, pos.Y+16)
		h := scrollingSheetHeight(
			popup.MinSize().Height,
			bodyScroll.MinSize().Height,
			body.MinSize().Height,
			maxH,
		)
		popup.Resize(fyne.NewSize(w, h))
	}
	fitVOTD()

	// Re-measure once the real layout has landed so the card fits the passage
	// snugly. Visible() gates it: a dismissed card must not re-measure.
	votdRemeasure(func() {
		if popup != nil && popup.Visible() {
			fitVOTD()
		}
	})
}
