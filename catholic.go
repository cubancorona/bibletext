package bibletext

// World English Bible — Catholic Edition (WEBC).
//
// A public-domain modern-English Bible with the full 73-book Catholic canon (the 7
// deuterocanonical books — Tobit, Judith, Wisdom, Sirach, Baruch, 1–2 Maccabees — plus
// the Greek additions to Esther and Daniel). It is the same WEB text the app already
// ships, so adding it is consistent for readers. Served by the free, key-less
// bible.helloao.org as one ~8 MB complete.json (translation id "eng_webc"), cached and
// switched like every other version.
//
// Two things differ from the 66-book BSB/WEB path:
//
//  1. helloao orders eng_webc as the 64 protocanonical books (Genesis…Revelation, minus
//     the short Hebrew Esther & Daniel) followed by the 9 deuterocanonical books APPENDED
//     at the end, with Esther & Daniel present only there in their Greek (expanded) forms.
//     So matching by helloao's `order` (as decodeBSBComplete does) would misalign — this
//     edition is decoded by USFM id (usfmToCatholicName) instead.
//
//  2. The decoded book list is emitted in traditional Catholic order (catholicBooks):
//     the deuterocanon interleaved where a Catholic Bible places it, and the Greek
//     Esther/Daniel carried under their normal names in their normal positions (so book
//     names stay aligned with the other versions and the reading position survives a
//     version switch). Their extra material is simply additional verses/chapters.
//
// The rest of the app is data-driven off BibleData.Books, so navigation, the Goto picker,
// search and reading need no per-canon special-casing. Features keyed to the 66-book set
// (cross-references, red-letter words, verse-of-the-day) simply have no entries for the
// deuterocanon and skip it gracefully.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// webcCompleteURL is helloao's whole-translation endpoint for the World English Bible
// (Catholic) — one request for all 73 books.
const webcCompleteURL = "https://bible.helloao.org/api/eng_webc/complete.json"

// catholicBooks is the 73-book Catholic canon in traditional order: the deuterocanonical
// books interleaved (Tobit/Judith/Esther/1–2 Maccabees after Nehemiah; Wisdom/Sirach
// after Song of Solomon; Baruch after Lamentations), with Esther and Daniel in their
// normal Old-Testament positions. The decoder emits BibleData.Books in this order.
var catholicBooks = []string{
	// Old Testament — the Law
	"Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy",
	// Old Testament — history (with Tobit, Judith, Esther, 1–2 Maccabees)
	"Joshua", "Judges", "Ruth", "1 Samuel", "2 Samuel",
	"1 Kings", "2 Kings", "1 Chronicles", "2 Chronicles", "Ezra", "Nehemiah",
	"Tobit", "Judith", "Esther", "1 Maccabees", "2 Maccabees",
	// Old Testament — poetry & wisdom (with Wisdom, Sirach)
	"Job", "Psalms", "Proverbs", "Ecclesiastes", "Song of Solomon", "Wisdom", "Sirach",
	// Old Testament — major prophets (with Baruch)
	"Isaiah", "Jeremiah", "Lamentations", "Baruch", "Ezekiel", "Daniel",
	// Old Testament — minor prophets
	"Hosea", "Joel", "Amos", "Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk",
	"Zephaniah", "Haggai", "Zechariah", "Malachi",
	// New Testament — Gospels and Acts
	"Matthew", "Mark", "Luke", "John", "Acts",
	// New Testament — Paul's letters
	"Romans", "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians",
	"Philippians", "Colossians", "1 Thessalonians", "2 Thessalonians",
	"1 Timothy", "2 Timothy", "Titus", "Philemon",
	// New Testament — Hebrews and other epistles
	"Hebrews", "James", "1 Peter", "2 Peter", "1 John", "2 John", "3 John", "Jude",
	// New Testament — Revelation
	"Revelation",
}

// usfmToCatholicName maps each helloao USFM book id in eng_webc to the app's book name.
// The protocanon ids map to the canonical names; the deuterocanon ids map to their
// Catholic names, with the Greek editions ESG and DAG carried as plain "Esther" and
// "Daniel" so they sit in their normal positions in their fuller form.
var usfmToCatholicName = map[string]string{
	"GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers", "DEU": "Deuteronomy",
	"JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth", "1SA": "1 Samuel", "2SA": "2 Samuel",
	"1KI": "1 Kings", "2KI": "2 Kings", "1CH": "1 Chronicles", "2CH": "2 Chronicles",
	"EZR": "Ezra", "NEH": "Nehemiah",
	"TOB": "Tobit", "JDT": "Judith", "ESG": "Esther", "1MA": "1 Maccabees", "2MA": "2 Maccabees",
	"JOB": "Job", "PSA": "Psalms", "PRO": "Proverbs", "ECC": "Ecclesiastes", "SNG": "Song of Solomon",
	"WIS": "Wisdom", "SIR": "Sirach",
	"ISA": "Isaiah", "JER": "Jeremiah", "LAM": "Lamentations", "BAR": "Baruch", "EZK": "Ezekiel", "DAG": "Daniel",
	"HOS": "Hosea", "JOL": "Joel", "AMO": "Amos", "OBA": "Obadiah", "JON": "Jonah", "MIC": "Micah",
	"NAM": "Nahum", "HAB": "Habakkuk", "ZEP": "Zephaniah", "HAG": "Haggai", "ZEC": "Zechariah", "MAL": "Malachi",
	"MAT": "Matthew", "MRK": "Mark", "LUK": "Luke", "JHN": "John", "ACT": "Acts",
	"ROM": "Romans", "1CO": "1 Corinthians", "2CO": "2 Corinthians", "GAL": "Galatians", "EPH": "Ephesians",
	"PHP": "Philippians", "COL": "Colossians", "1TH": "1 Thessalonians", "2TH": "2 Thessalonians",
	"1TI": "1 Timothy", "2TI": "2 Timothy", "TIT": "Titus", "PHM": "Philemon",
	"HEB": "Hebrews", "JAS": "James", "1PE": "1 Peter", "2PE": "2 Peter",
	"1JN": "1 John", "2JN": "2 John", "3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
}

// webCatholicSource serves the public-domain World English Bible (Catholic) from
// bible.helloao.org in a single request.
type webCatholicSource struct{}

func (webCatholicSource) available() bool { return true }

func (webCatholicSource) fetch() (*BibleData, error) {
	// 120s timeout: one ~8 MB body (73 books), so this must cover a slow connection's
	// full download. Shares the fetch/validate path with the BSB/WEB sources.
	return fetchHelloAOComplete("WEB Catholic", webcCompleteURL, &http.Client{Timeout: 120 * time.Second}, decodeHelloAOCatholic)
}

// decodeHelloAOCatholic decodes eng_webc's complete.json by USFM id (not helloao's
// order, which appends the deuterocanon), mapping each book through usfmToCatholicName.
// BibleData.Books is emitted in traditional Catholic order (catholicBooks), filtered to
// the books that actually came through.
func decodeHelloAOCatholic(body []byte) (*BibleData, error) {
	var doc struct {
		Books []helloAOBook `json:"books"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(doc.Books) == 0 {
		return nil, fmt.Errorf("no books in response")
	}

	bd := &BibleData{Verses: make(map[string]map[int][]Verse, len(catholicBooks))}
	present := make(map[string]bool, len(catholicBooks))
	for _, b := range doc.Books {
		name := usfmToCatholicName[b.ID]
		if name == "" {
			continue // unrecognized USFM id (not expected for eng_webc)
		}
		if chapters := decodeHelloAOChapters(name, b); len(chapters) > 0 {
			bd.Verses[name] = chapters
			present[name] = true
		}
	}
	for _, name := range catholicBooks {
		if present[name] {
			bd.Books = append(bd.Books, name)
		}
	}
	// PrepareSearchIndex is left to the caller (loadBibleData), matching the 66-book path.
	return bd, nil
}
