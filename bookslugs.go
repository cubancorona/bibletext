package bibletext

// Book slugs for the web reader's URLs.
//
// THIS TABLE IS A PUBLIC, PERMANENT CONTRACT. Every verse link the app has ever
// shared contains one of these slugs, and those links live forever in other
// people's message threads. The rules, in order of importance:
//
//  1. A slug, once shipped, is IMMUTABLE. If a book's DISPLAY name ever changes
//     ("Song of Solomon" → "Song of Songs"), only the display side moves; the
//     slug stays. Renaming a slug breaks every link that ever used it.
//  2. The table is APPEND-ONLY. New books get new slugs; nothing is removed.
//  3. Slugs are lowercase [a-z0-9-] — GitHub Pages serves case-sensitively, and
//     messenger linkifiers mangle anything exotic.
//
// The same table serves BOTH the app's share-link builder and the site
// generator, so a URL the app emits and a page the generator writes can never
// disagree. share_link_test.go holds a golden copy of every pair, and asserts
// the table is append-only: editing a slug fails CI loudly, which is the point.
//
// Greek Esther and Greek Daniel deliberately share the "esther"/"daniel" slugs
// with their shorter protocanonical forms (catholic.go carries them under the
// plain names), so a reading position survives a version switch on the web
// exactly as it does in the app.

import "strings"

// bookSlugs maps every canonical book name — the 66-book Protestant canon plus
// the Catholic deuterocanon — to its permanent URL slug.
var bookSlugs = map[string]string{
	// Old Testament — the Law
	"Genesis": "genesis", "Exodus": "exodus", "Leviticus": "leviticus",
	"Numbers": "numbers", "Deuteronomy": "deuteronomy",
	// Old Testament — history
	"Joshua": "joshua", "Judges": "judges", "Ruth": "ruth",
	"1 Samuel": "1-samuel", "2 Samuel": "2-samuel",
	"1 Kings": "1-kings", "2 Kings": "2-kings",
	"1 Chronicles": "1-chronicles", "2 Chronicles": "2-chronicles",
	"Ezra": "ezra", "Nehemiah": "nehemiah", "Esther": "esther",
	// Old Testament — poetry & wisdom
	"Job": "job", "Psalms": "psalms", "Proverbs": "proverbs",
	"Ecclesiastes": "ecclesiastes", "Song of Solomon": "song-of-solomon",
	// Old Testament — major prophets
	"Isaiah": "isaiah", "Jeremiah": "jeremiah", "Lamentations": "lamentations",
	"Ezekiel": "ezekiel", "Daniel": "daniel",
	// Old Testament — minor prophets
	"Hosea": "hosea", "Joel": "joel", "Amos": "amos", "Obadiah": "obadiah",
	"Jonah": "jonah", "Micah": "micah", "Nahum": "nahum", "Habakkuk": "habakkuk",
	"Zephaniah": "zephaniah", "Haggai": "haggai", "Zechariah": "zechariah",
	"Malachi": "malachi",
	// Deuterocanon (WEB Catholic only)
	"Tobit": "tobit", "Judith": "judith",
	"1 Maccabees": "1-maccabees", "2 Maccabees": "2-maccabees",
	"Wisdom": "wisdom", "Sirach": "sirach", "Baruch": "baruch",
	// New Testament — Gospels and Acts
	"Matthew": "matthew", "Mark": "mark", "Luke": "luke", "John": "john",
	"Acts": "acts",
	// New Testament — Paul's letters
	"Romans": "romans", "1 Corinthians": "1-corinthians",
	"2 Corinthians": "2-corinthians", "Galatians": "galatians",
	"Ephesians": "ephesians", "Philippians": "philippians",
	"Colossians": "colossians", "1 Thessalonians": "1-thessalonians",
	"2 Thessalonians": "2-thessalonians", "1 Timothy": "1-timothy",
	"2 Timothy": "2-timothy", "Titus": "titus", "Philemon": "philemon",
	// New Testament — Hebrews and other epistles
	"Hebrews": "hebrews", "James": "james", "1 Peter": "1-peter",
	"2 Peter": "2-peter", "1 John": "1-john", "2 John": "2-john",
	"3 John": "3-john", "Jude": "jude",
	// New Testament — Revelation
	"Revelation": "revelation",
}

// slugToBook is the reverse lookup, built once at init.
var slugToBook = func() map[string]string {
	m := make(map[string]string, len(bookSlugs))
	for name, slug := range bookSlugs {
		m[slug] = name
	}
	return m
}()

// BookSlug returns a book's permanent URL slug.
func BookSlug(name string) (string, bool) {
	slug, ok := bookSlugs[strings.TrimSpace(name)]
	return slug, ok
}

// BookFromSlug resolves a URL slug back to its canonical book name.
func BookFromSlug(slug string) (string, bool) {
	name, ok := slugToBook[strings.ToLower(strings.TrimSpace(slug))]
	return name, ok
}
