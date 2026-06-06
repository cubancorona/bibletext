package bibletext

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Verse represents a single verse (sentence/passage) from the Bible
// This is the smallest unit of text in our application
type Verse struct {
	BookName string // Name of the book (e.g., "John", "Genesis")
	Book     string // Abbreviated book name/reference
	Chapter  int    // Chapter number within the book
	Verse    int    // Verse number within the chapter
	Text     string // The actual text of the verse
	Search   string `json:"-"` // Lowercased text for fast case-insensitive search
}

// BibleData holds all Bible verses organized by book and chapter
// This is the data model/storage for the entire Bible
// Structure:
//   - Books: list of all 66 book names in order
//   - Verses: map[book][chapter] = list of verses
//     Example: Verses["John"][1] = [Verse1, Verse2, ...]
type BibleData struct {
	// Verses is a nested map (map of maps) organizing all verses
	// First key: book name ("John", "Genesis", etc.)
	// Second key: chapter number (1, 2, 3, etc.)
	// Value: slice of all Verse objects in that chapter
	Verses map[string]map[int][]Verse

	// Books is a slice containing all 66 book names in canonical order
	// Used to display the book list in the sidebar
	Books []string
}

// NewBibleData creates and initializes a new BibleData structure
// This sets up the empty data structure ready to be populated with verses
func NewBibleData() *BibleData {
	return &BibleData{
		// Initialize the verses map (nested maps must be created explicitly)
		// Without this, trying to add verses would cause a nil pointer error
		Verses: make(map[string]map[int][]Verse),
		// Initialize the list of all 66 books of the Bible in their traditional order
		// Old Testament (39 books), New Testament (27 books)
		Books: []string{
			// Old Testament - The Law
			"Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy",
			// Old Testament - History
			"Joshua", "Judges", "Ruth", "1 Samuel", "2 Samuel",
			"1 Kings", "2 Kings", "1 Chronicles", "2 Chronicles", "Ezra",
			"Nehemiah", "Esther",
			// Old Testament - Poetry & Wisdom
			"Job", "Psalms", "Proverbs", "Ecclesiastes", "Song of Solomon",
			// Old Testament - Major Prophets
			"Isaiah", "Jeremiah", "Lamentations", "Ezekiel", "Daniel",
			// Old Testament - Minor Prophets
			"Hosea", "Joel", "Amos", "Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk",
			"Zephaniah", "Haggai", "Zechariah", "Malachi",
			// New Testament - Gospels and Acts
			"Matthew", "Mark", "Luke", "John", "Acts",
			// New Testament - Paul's Letters
			"Romans", "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians",
			"Philippians", "Colossians", "1 Thessalonians", "2 Thessalonians",
			"1 Timothy", "2 Timothy", "Titus", "Philemon",
			// New Testament - Hebrews and other epistles
			"Hebrews", "James", "1 Peter", "2 Peter", "1 John", "2 John", "3 John", "Jude",
			// New Testament - Revelation
			"Revelation",
		},
	}
}

// PopulateWithSampleVerses loads a small set of World English Bible (public
// domain) verses. This is demo/fixture data used by tests and offline examples;
// the running app always loads the complete WEB text from cache or the API.
func (bd *BibleData) PopulateWithSampleVerses() {
	// Initialize chapter maps for every book
	// Without this, trying to add verses would panic (nil map)
	for _, book := range bd.Books {
		bd.Verses[book] = make(map[int][]Verse)
	}

	// Create a struct to hold verse data during the loop
	// This is a convenient way to add multiple verses at once
	verses := []struct {
		book    string // Book name
		chapter int    // Chapter number
		verse   int    // Verse number
		text    string // The actual verse text
	}{
		// John - Gospel of John
		{"John", 1, 1, "In the beginning was the Word, and the Word was with God, and the Word was God."},
		{"John", 1, 2, "The same was in the beginning with God."},
		{"John", 1, 3, "All things were made through him. Without him was not anything made that has been made."},
		{"John", 3, 16, "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish, but have eternal life."},
		{"John", 2, 1, "The third day, there was a marriage in Cana of Galilee. Jesus\u2019 mother was there."},

		// Genesis - Creation
		{"Genesis", 1, 1, "In the beginning, God created the heavens and the earth."},
		{"Genesis", 1, 2, "The earth was formless and empty. Darkness was on the surface of the deep and God\u2019s Spirit was hovering over the surface of the waters."},
		{"Genesis", 2, 1, "The heavens, the earth, and all their vast array were finished."},
		{"Genesis", 2, 2, "On the seventh day God finished his work which he had done; and he rested on the seventh day from all his work which he had done."},

		// Psalms
		{"Psalms", 23, 1, "Yahweh is my shepherd: I shall lack nothing."},
		{"Psalms", 23, 2, "He makes me lie down in green pastures. He leads me beside still waters."},
		{"Psalms", 23, 3, "He restores my soul. He guides me in the paths of righteousness for his name\u2019s sake."},
		{"Psalms", 42, 1, "As the deer pants for the water brooks, so my soul pants after you, God."},
		{"Psalms", 42, 2, "My soul thirsts for God, for the living God. When shall I come and appear before God?"},

		// Matthew - Beatitudes & the Lord's Prayer
		{"Matthew", 5, 3, "\u201cBlessed are the poor in spirit, for theirs is the Kingdom of Heaven."},
		{"Matthew", 5, 4, "Blessed are those who mourn, for they shall be comforted."},
		{"Matthew", 5, 5, "Blessed are the gentle, for they shall inherit the earth."},
		{"Matthew", 6, 9, "Pray like this: \u2018Our Father in heaven, may your name be kept holy."},
		{"Matthew", 6, 10, "Let your Kingdom come. Let your will be done, as in heaven, so on earth."},

		// Proverbs - Wisdom
		{"Proverbs", 3, 5, "Trust in Yahweh with all your heart, and don\u2019t lean on your own understanding."},
		{"Proverbs", 3, 6, "In all your ways acknowledge him, and he will make your paths straight."},
		{"Proverbs", 1, 1, "The proverbs of Solomon, the son of David, king of Israel:"},
		{"Proverbs", 1, 2, "to know wisdom and instruction; to discern the words of understanding;"},

		// Romans - Faith
		{"Romans", 3, 22, "even the righteousness of God through faith in Jesus Christ to all and on all those who believe. For there is no distinction,"},
		{"Romans", 3, 23, "for all have sinned, and fall short of the glory of God;"},
		{"Romans", 8, 28, "We know that all things work together for good for those who love God, to those who are called according to his purpose."},
		{"Romans", 8, 29, "For whom he foreknew, he also predestined to be conformed to the image of his Son, that he might be the firstborn among many brothers."},

		// 1 Corinthians - Love
		{"1 Corinthians", 13, 4, "Love is patient and is kind; love doesn\u2019t envy. Love doesn\u2019t brag, is not proud,"},
		{"1 Corinthians", 13, 5, "doesn\u2019t behave itself inappropriately, doesn\u2019t seek its own way, is not provoked, takes no account of evil;"},
		{"1 Corinthians", 13, 13, "But now faith, hope, and love remain\u2014these three. The greatest of these is love."},
		{"1 Corinthians", 1, 1, "Paul, called to be an apostle of Jesus Christ through the will of God, and our brother Sosthenes,"},

		// Hebrews - Faith
		{"Hebrews", 11, 1, "Now faith is assurance of things hoped for, proof of things not seen."},
		{"Hebrews", 11, 2, "For by this, the elders obtained testimony."},
		{"Hebrews", 12, 1, "Therefore let us also, seeing we are surrounded by so great a cloud of witnesses, lay aside every weight and the sin which so easily entangles us, and let us run with perseverance the race that is set before us,"},
		{"Hebrews", 12, 2, "looking to Jesus, the author and perfecter of faith, who for the joy that was set before him endured the cross, despising its shame, and has sat down at the right hand of the throne of God."},

		// Revelation - Hope
		{"Revelation", 21, 3, "I heard a loud voice out of heaven saying, \u201cBehold, God\u2019s dwelling is with people, and he will dwell with them, and they will be his people, and God himself will be with them as their God."},
		{"Revelation", 21, 4, "He will wipe away every tear from their eyes. Death will be no more; neither will there be mourning, nor crying, nor pain, any more. The first things have passed away.\u201d"},
		{"Revelation", 1, 1, "This is the Revelation of Jesus Christ, which God gave him to show to his servants the things which must happen soon, which he sent and made known by his angel to his servant, John,"},
		{"Revelation", 1, 2, "who testified to God\u2019s word, and of the testimony of Jesus Christ, about everything that he saw."},

		// Mark
		{"Mark", 1, 1, "The beginning of the Good News of Jesus Christ, the Son of God."},
		{"Mark", 1, 2, "As it is written in the prophets, \u201cBehold, I send my messenger before your face, who will prepare your way before you:"},
		{"Mark", 14, 36, "He said, \u201cAbba, Father, all things are possible to you. Please remove this cup from me. However, not what I desire, but what you desire.\u201d"},
		{"Mark", 15, 34, "At the ninth hour Jesus cried with a loud voice, saying, \u201cEloi, Eloi, lama sabachthani?\u201d which is, being interpreted, \u201cMy God, my God, why have you forsaken me?\u201d"},

		// Luke
		{"Luke", 1, 1, "Since many have undertaken to set in order a narrative concerning those matters which have been fulfilled among us,"},
		{"Luke", 1, 2, "even as those who from the beginning were eyewitnesses and servants of the word delivered them to us,"},
		{"Luke", 2, 10, "The angel said to them, \u201cDon\u2019t be afraid, for behold, I bring you good news of great joy which will be to all the people."},
		{"Luke", 2, 11, "For there is born to you today, in David\u2019s city, a Savior, who is Christ the Lord."},

		// Acts
		{"Acts", 1, 1, "The first book I wrote, Theophilus, concerned all that Jesus began both to do and to teach,"},
		{"Acts", 1, 2, "until the day in which he was received up, after he had given commandment through the Holy Spirit to the apostles whom he had chosen."},
		{"Acts", 2, 38, "Peter said to them, \u201cRepent, and be baptized, every one of you, in the name of Jesus Christ for the forgiveness of sins, and you will receive the gift of the Holy Spirit."},
		{"Acts", 2, 39, "For the promise is to you, and to your children, and to all who are far off, even as many as the Lord our God will call to himself.\u201d"},

		// Exodus
		{"Exodus", 1, 1, "Now these are the names of the sons of Israel, who came into Egypt (every man and his household came with Jacob):"},
		{"Exodus", 3, 1, "Now Moses was keeping the flock of Jethro, his father-in-law, the priest of Midian, and he led the flock to the back of the wilderness, and came to God\u2019s mountain, to Horeb."},
		{"Exodus", 20, 1, "God spoke all these words, saying,"},
		{"Exodus", 20, 2, "\u201cI am Yahweh your God, who brought you out of the land of Egypt, out of the house of bondage."},

		// Leviticus
		{"Leviticus", 1, 1, "Yahweh called to Moses, and spoke to him from the Tent of Meeting, saying,"},
		{"Leviticus", 19, 1, "Yahweh spoke to Moses, saying,"},
		{"Leviticus", 19, 2, "\u201cSpeak to all the congregation of the children of Israel, and tell them, \u2018You shall be holy; for I, Yahweh your God, am holy."},

		// Numbers
		{"Numbers", 1, 1, "Yahweh spoke to Moses in the wilderness of Sinai, in the Tent of Meeting, on the first day of the second month, in the second year after they had come out of the land of Egypt, saying,"},
		{"Numbers", 14, 1, "All the congregation lifted up their voice, and cried; and the people wept that night."},

		// Deuteronomy
		{"Deuteronomy", 1, 1, "These are the words which Moses spoke to all Israel beyond the Jordan in the wilderness, in the Arabah over against Suf, between Paran, Tophel, Laban, Hazeroth, and Dizahab."},
		{"Deuteronomy", 6, 4, "Hear, Israel: Yahweh is our God. Yahweh is one."},
	}

	// Add all verses to the data structure
	// Loop through each verse struct and add it to the nested map
	for _, v := range verses {
		// Check if this chapter already exists in the map
		if _, exists := bd.Verses[v.book][v.chapter]; !exists {
			// If chapter doesn't exist, create an empty slice for it
			bd.Verses[v.book][v.chapter] = []Verse{}
		}
		// Append the verse to the chapter's verse slice
		// This adds the verse to the list of verses in that chapter
		bd.Verses[v.book][v.chapter] = append(bd.Verses[v.book][v.chapter], Verse{
			BookName: v.book,
			Book:     v.book,
			Chapter:  v.chapter,
			Verse:    v.verse,
			Text:     v.text,
		})
	}
}

// GetVerse returns a specific verse from the Bible
// Returns nil if the verse doesn't exist
// Parameters:
//   - book: name of the book (e.g., "John")
//   - chapter: chapter number
//   - verse: verse number
//
// Example: GetVerse("John", 3, 16) returns "For God so loved the world..."
func (bd *BibleData) GetVerse(book string, chapter int, verse int) *Verse {
	// Check if the book exists in our data
	if chapters, ok := bd.Verses[book]; ok {
		// Check if the chapter exists in this book
		if verses, ok := chapters[chapter]; ok {
			// Search through all verses in this chapter
			for _, v := range verses {
				// Return the verse if we found it
				if v.Verse == verse {
					return &v
				}
			}
		}
	}
	// Verse not found, return nil
	return nil
}

// GetChapter returns all verses in a specific chapter
// Returns an empty slice if the chapter has no verses
// This is the main function for displaying a chapter
// Example: GetChapter("John", 1) returns all verses in John chapter 1
func (bd *BibleData) GetChapter(book string, chapter int) []Verse {
	// Check if the book exists
	if chapters, ok := bd.Verses[book]; ok {
		// Check if the chapter exists
		if verses, ok := chapters[chapter]; ok {
			// Return all verses in this chapter
			return verses
		}
	}
	// Chapter not found, return empty slice (not nil)
	// Empty slice is better than nil because it's safer to iterate over
	return []Verse{}
}

// GetChaptersForBook returns the number of chapters in a book
// Used by the navigation buttons to determine if we can go to the next chapter
// Example: GetChaptersForBook("John") might return 21 (John has 21 chapters)
func (bd *BibleData) GetChaptersForBook(book string) int {
	return len(bd.GetChapterNumbersForBook(book))
}

// GetChapterNumbersForBook returns sorted available chapter numbers for a book.
func (bd *BibleData) GetChapterNumbersForBook(book string) []int {
	chapters, ok := bd.Verses[book]
	if !ok {
		return []int{}
	}

	numbers := make([]int, 0, len(chapters))
	for chapter := range chapters {
		numbers = append(numbers, chapter)
	}
	sort.Ints(numbers)
	return numbers
}

// Search searches for verses containing the given query text
// Returns a slice of all verses that match (case-insensitive)
// Example: Search("faith") returns all verses containing the word "faith"
func (bd *BibleData) Search(query string) []Verse {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Verse{}
	}

	// Create a slice to hold the search results
	var results []Verse
	// Convert query to lowercase for case-insensitive search
	query = strings.ToLower(query)

	// Loop through books in canonical order for deterministic results.
	for _, book := range bd.Books {
		chapters, ok := bd.Verses[book]
		if !ok {
			continue
		}
		for _, chapter := range bd.GetChapterNumbersForBook(book) {
			verses := chapters[chapter]
			// Loop through every verse in the chapter
			for _, verse := range verses {
				// Check if the verse text contains the query (case-insensitive).
				if strings.Contains(searchText(verse), query) {
					// Add this verse to the results
					results = append(results, verse)
				}
			}
		}
	}

	// Return all matching verses
	return results
}

// SearchLimited searches for verses containing the query and caps returned results.
// It returns the matches and whether additional matches were omitted due to limit.
func (bd *BibleData) SearchLimited(query string, limit int) ([]Verse, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []Verse{}, false
	}

	var results []Verse
	query = strings.ToLower(query)
	truncated := false

	for _, book := range bd.Books {
		chapters, ok := bd.Verses[book]
		if !ok {
			continue
		}
		for _, chapter := range bd.GetChapterNumbersForBook(book) {
			verses := chapters[chapter]
			for _, verse := range verses {
				if strings.Contains(searchText(verse), query) {
					if limit > 0 && len(results) >= limit {
						truncated = true
						return results, truncated
					}
					results = append(results, verse)
				}
			}
		}
	}

	return results, truncated
}

var chapterReferencePattern = regexp.MustCompile(`^(.+?)\s+(\d+)(?::(\d+)|\s+(\d+))?$`)

// SearchSmartLimited supports verse reference queries and ranked term matching.
func (bd *BibleData) SearchSmartLimited(query string, limit int) ([]Verse, bool) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []Verse{}, false
	}

	if book, chapter, verse, hasVerse, ok := bd.parseReferenceQuery(trimmed); ok {
		if hasVerse {
			match := bd.GetVerse(book, chapter, verse)
			if match == nil {
				return []Verse{}, false
			}
			return []Verse{*match}, false
		}

		chapterVerses := bd.GetChapter(book, chapter)
		if limit > 0 && len(chapterVerses) > limit {
			return chapterVerses[:limit], true
		}
		return chapterVerses, false
	}

	phrase := strings.ToLower(trimmed)
	terms := strings.Fields(phrase)
	if len(terms) == 0 {
		return []Verse{}, false
	}

	type scoredVerse struct {
		verse Verse
		score int
	}
	matches := make([]scoredVerse, 0, 256)

	for _, book := range bd.Books {
		chapters, ok := bd.Verses[book]
		if !ok {
			continue
		}
		for _, chapter := range bd.GetChapterNumbersForBook(book) {
			verses := chapters[chapter]
			for _, verse := range verses {
				text := searchText(verse)
				ref := strings.ToLower(fmt.Sprintf("%s %d:%d", verse.BookName, verse.Chapter, verse.Verse))

				allTermsMatch := true
				score := 0
				for _, term := range terms {
					inText := strings.Contains(text, term)
					inRef := strings.Contains(ref, term)
					if !inText && !inRef {
						allTermsMatch = false
						break
					}
					if inRef {
						score += 3
					}
					if inText {
						score += 1
					}
				}
				if !allTermsMatch {
					continue
				}
				if strings.Contains(text, phrase) || strings.Contains(ref, phrase) {
					score += 5
				}
				score -= len(verse.Text) / 300

				matches = append(matches, scoredVerse{verse: verse, score: score})
			}
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	results := make([]Verse, 0, len(matches))
	for _, m := range matches {
		results = append(results, m.verse)
	}
	if limit > 0 && len(results) > limit {
		return results[:limit], true
	}
	return results, false
}

func (bd *BibleData) parseReferenceQuery(query string) (book string, chapter int, verse int, hasVerse bool, ok bool) {
	matches := chapterReferencePattern.FindStringSubmatch(strings.TrimSpace(query))
	if len(matches) == 0 {
		return "", 0, 0, false, false
	}

	bookName, ok := resolveBookName(bd.Books, matches[1])
	if !ok {
		return "", 0, 0, false, false
	}

	chapterNum, err := strconv.Atoi(matches[2])
	if err != nil || chapterNum < 1 {
		return "", 0, 0, false, false
	}

	versePart := strings.TrimSpace(matches[3])
	if versePart == "" {
		versePart = strings.TrimSpace(matches[4])
	}
	if versePart == "" {
		return bookName, chapterNum, 0, false, true
	}

	verseNum, err := strconv.Atoi(versePart)
	if err != nil || verseNum < 1 {
		return "", 0, 0, false, false
	}
	return bookName, chapterNum, verseNum, true, true
}

// bookAliases maps common abbreviations and alternate spellings to canonical
// book names so references like "Ps 23", "Jn 3:16" or "1 Cor 13" resolve.
var bookAliases = map[string]string{
	"gen": "Genesis", "ex": "Exodus", "exod": "Exodus", "lev": "Leviticus",
	"num": "Numbers", "deut": "Deuteronomy", "dt": "Deuteronomy",
	"josh": "Joshua", "judg": "Judges", "rt": "Ruth",
	"1 sam": "1 Samuel", "2 sam": "2 Samuel", "1 kgs": "1 Kings", "2 kgs": "2 Kings",
	"1 chr": "1 Chronicles", "2 chr": "2 Chronicles", "neh": "Nehemiah", "est": "Esther",
	"ps": "Psalms", "psa": "Psalms", "psalm": "Psalms", "prov": "Proverbs", "prv": "Proverbs",
	"eccl": "Ecclesiastes", "qoh": "Ecclesiastes", "song": "Song of Solomon",
	"song of songs": "Song of Solomon", "sos": "Song of Solomon", "canticles": "Song of Solomon",
	"isa": "Isaiah", "jer": "Jeremiah", "lam": "Lamentations", "ezek": "Ezekiel",
	"eze": "Ezekiel", "dan": "Daniel", "hos": "Hosea", "obad": "Obadiah", "jon": "Jonah",
	"mic": "Micah", "nah": "Nahum", "hab": "Habakkuk", "zeph": "Zephaniah", "hag": "Haggai",
	"zech": "Zechariah", "mal": "Malachi",
	"matt": "Matthew", "mt": "Matthew", "mk": "Mark", "mrk": "Mark", "lk": "Luke",
	"jn": "John", "jhn": "John", "rom": "Romans",
	"1 cor": "1 Corinthians", "2 cor": "2 Corinthians", "gal": "Galatians", "eph": "Ephesians",
	"phil": "Philippians", "php": "Philippians", "col": "Colossians",
	"1 thess": "1 Thessalonians", "2 thess": "2 Thessalonians", "1 thes": "1 Thessalonians",
	"2 thes": "2 Thessalonians", "1 tim": "1 Timothy", "2 tim": "2 Timothy",
	"tit": "Titus", "phlm": "Philemon", "philem": "Philemon", "heb": "Hebrews",
	"jas": "James", "jam": "James", "1 pet": "1 Peter", "2 pet": "2 Peter",
	"1 pt": "1 Peter", "2 pt": "2 Peter", "1 jn": "1 John", "2 jn": "2 John",
	"3 jn": "3 John", "rev": "Revelation",
}

// resolveBookName matches user input to a canonical book name via exact match,
// known aliases, then a unique case-insensitive prefix (e.g. "philipp").
func resolveBookName(books []string, input string) (string, bool) {
	q := strings.ToLower(strings.TrimSpace(input))
	if q == "" {
		return "", false
	}

	for _, b := range books {
		if strings.ToLower(b) == q {
			return b, true
		}
	}

	if name, ok := bookAliases[q]; ok {
		for _, b := range books {
			if b == name {
				return b, true
			}
		}
	}

	match := ""
	for _, b := range books {
		if strings.HasPrefix(strings.ToLower(b), q) {
			if match != "" {
				return "", false // ambiguous prefix
			}
			match = b
		}
	}
	if match != "" {
		return match, true
	}
	return "", false
}

func searchText(v Verse) string {
	if v.Search != "" {
		return v.Search
	}
	return strings.ToLower(v.Text)
}

// PrepareSearchIndex precomputes normalized verse text used by search.
func (bd *BibleData) PrepareSearchIndex() {
	for book, chapters := range bd.Verses {
		for chapter, verses := range chapters {
			for i := range verses {
				verses[i].Search = strings.ToLower(verses[i].Text)
			}
			chapters[chapter] = verses
		}
		bd.Verses[book] = chapters
	}
}
