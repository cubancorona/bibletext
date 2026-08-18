package bibletext

// The frozen canon order behind the wire's 'b' record.
//
// A note's 'b' record carries a BOOK INDEX — one or two bytes instead of up to
// fifteen — so the index-to-name table below is part of the wire format the
// moment a link is sent, exactly as bookslugs.go's slug table is part of the
// URL contract. The rules are the slug table's rules:
//
//  1. An index, once shipped, is IMMUTABLE. Position 0 is Genesis forever.
//  2. The table is APPEND-ONLY. A book this app learns about later gets the
//     next free index; nothing is ever removed, reordered, or renamed in place
//     (a display-name change would be a NEW entry — the old name keeps its
//     slot, exactly as an old slug keeps its URL).
//  3. The set of names is bookslugs.go's set: every book that can appear in a
//     shared link has an index, and share_note_canon_test.go holds a GOLDEN of
//     all 73 pairs plus the set-equality check, so editing either table fails
//     CI loudly.
//
// THE ORDER, pinned here so nobody reconstructs it differently: indices 0–65
// are the 66-book Protestant canon in traditional order (the order
// bible.go's NewBibleData().Books ships and has always shipped); indices 66–72
// are the seven deuterocanonical books in the order bookslugs.go lists them
// (Tobit, Judith, 1 Maccabees, 2 Maccabees, Wisdom, Sirach, Baruch). The
// deuterocanon is APPENDED rather than interleaved at its traditional
// positions because interleaving would renumber the New Testament — and rule 1
// exists precisely so that a later canon addition never moves an existing
// index.
//
// (Greek Esther and Greek Daniel share the "Esther"/"Daniel" entries with
// their shorter forms, exactly as they share slugs — catholic.go carries them
// under the plain names.)
var noteBookOrder = []string{
	// 0–38: Old Testament, traditional Protestant order
	"Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy",
	"Joshua", "Judges", "Ruth", "1 Samuel", "2 Samuel",
	"1 Kings", "2 Kings", "1 Chronicles", "2 Chronicles", "Ezra",
	"Nehemiah", "Esther",
	"Job", "Psalms", "Proverbs", "Ecclesiastes", "Song of Solomon",
	"Isaiah", "Jeremiah", "Lamentations", "Ezekiel", "Daniel",
	"Hosea", "Joel", "Amos", "Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk",
	"Zephaniah", "Haggai", "Zechariah", "Malachi",
	// 39–65: New Testament
	"Matthew", "Mark", "Luke", "John", "Acts",
	"Romans", "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians",
	"Philippians", "Colossians", "1 Thessalonians", "2 Thessalonians",
	"1 Timothy", "2 Timothy", "Titus", "Philemon",
	"Hebrews", "James", "1 Peter", "2 Peter", "1 John", "2 John", "3 John",
	"Jude", "Revelation",
	// 66–72: the deuterocanon, appended (see the header for why not interleaved)
	"Tobit", "Judith", "1 Maccabees", "2 Maccabees", "Wisdom", "Sirach", "Baruch",
}

// noteBookIndexByName is the reverse lookup, built once at init.
var noteBookIndexByName = func() map[string]int {
	m := make(map[string]int, len(noteBookOrder))
	for i, name := range noteBookOrder {
		m[name] = i
	}
	return m
}()

// noteBookIndexOf returns a canonical book name's frozen wire index.
func noteBookIndexOf(name string) (int, bool) {
	i, ok := noteBookIndexByName[name]
	return i, ok
}
