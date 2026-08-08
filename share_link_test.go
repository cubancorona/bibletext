package bibletext

// The URL contract's tripwire. These tests exist to FAIL LOUDLY if anyone ever
// edits a slug or the URL shape: every link the app has shared lives forever in
// somebody's message thread, so a "tidy-up" here silently breaks the past.

import (
	"regexp"
	"strings"
	"testing"
)

// TestBookSlugsCoverEveryBook: every book the app can display must have a slug,
// or sharing that chapter produces no link at all.
func TestBookSlugsCoverEveryBook(t *testing.T) {
	for _, b := range NewBibleData().Books {
		if _, ok := BookSlug(b); !ok {
			t.Errorf("protestant canon book %q has no slug", b)
		}
	}
	for _, b := range catholicBooks {
		if _, ok := BookSlug(b); !ok {
			t.Errorf("catholic canon book %q has no slug", b)
		}
	}
}

// TestBookSlugsAreWellFormedAndUnique: the charset is what makes a slug survive
// every messenger's linkifier and GitHub Pages' case-sensitive serving.
func TestBookSlugsAreWellFormedAndUnique(t *testing.T) {
	valid := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	seen := map[string]string{}
	for name, slug := range bookSlugs {
		if !valid.MatchString(slug) {
			t.Errorf("book %q: slug %q is not lowercase-hyphenated ASCII", name, slug)
		}
		if prev, dup := seen[slug]; dup {
			t.Errorf("slug %q is used by both %q and %q — slugs must be unique", slug, prev, name)
		}
		seen[slug] = name
		if got, ok := BookFromSlug(slug); !ok || got != name {
			t.Errorf("slug %q does not round-trip: got (%q,%v), want %q", slug, got, ok, name)
		}
	}
}

// TestBookSlugsGolden is the contract itself, written out. A slug may be ADDED
// here; changing or removing one is what this test is for.
func TestBookSlugsGolden(t *testing.T) {
	golden := map[string]string{
		"Genesis": "genesis", "Exodus": "exodus", "Leviticus": "leviticus",
		"Numbers": "numbers", "Deuteronomy": "deuteronomy", "Joshua": "joshua",
		"Judges": "judges", "Ruth": "ruth", "1 Samuel": "1-samuel",
		"2 Samuel": "2-samuel", "1 Kings": "1-kings", "2 Kings": "2-kings",
		"1 Chronicles": "1-chronicles", "2 Chronicles": "2-chronicles",
		"Ezra": "ezra", "Nehemiah": "nehemiah", "Esther": "esther", "Job": "job",
		"Psalms": "psalms", "Proverbs": "proverbs", "Ecclesiastes": "ecclesiastes",
		"Song of Solomon": "song-of-solomon", "Isaiah": "isaiah",
		"Jeremiah": "jeremiah", "Lamentations": "lamentations",
		"Ezekiel": "ezekiel", "Daniel": "daniel", "Hosea": "hosea", "Joel": "joel",
		"Amos": "amos", "Obadiah": "obadiah", "Jonah": "jonah", "Micah": "micah",
		"Nahum": "nahum", "Habakkuk": "habakkuk", "Zephaniah": "zephaniah",
		"Haggai": "haggai", "Zechariah": "zechariah", "Malachi": "malachi",
		"Tobit": "tobit", "Judith": "judith", "1 Maccabees": "1-maccabees",
		"2 Maccabees": "2-maccabees", "Wisdom": "wisdom", "Sirach": "sirach",
		"Baruch": "baruch", "Matthew": "matthew", "Mark": "mark", "Luke": "luke",
		"John": "john", "Acts": "acts", "Romans": "romans",
		"1 Corinthians": "1-corinthians", "2 Corinthians": "2-corinthians",
		"Galatians": "galatians", "Ephesians": "ephesians",
		"Philippians": "philippians", "Colossians": "colossians",
		"1 Thessalonians": "1-thessalonians", "2 Thessalonians": "2-thessalonians",
		"1 Timothy": "1-timothy", "2 Timothy": "2-timothy", "Titus": "titus",
		"Philemon": "philemon", "Hebrews": "hebrews", "James": "james",
		"1 Peter": "1-peter", "2 Peter": "2-peter", "1 John": "1-john",
		"2 John": "2-john", "3 John": "3-john", "Jude": "jude",
		"Revelation": "revelation",
	}
	for name, want := range golden {
		got, ok := BookSlug(name)
		if !ok {
			t.Errorf("%q lost its slug (was %q) — every shared link to it is now broken", name, want)
			continue
		}
		if got != want {
			t.Errorf("%q slug changed %q -> %q — this breaks every link ever shared", name, want, got)
		}
	}
	if len(bookSlugs) < len(golden) {
		t.Errorf("the slug table shrank to %d entries (golden has %d) — the table is append-only",
			len(bookSlugs), len(golden))
	}
}

func TestShareLinkURL(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		version, book         string
		chapter, lo, hi, _pad int
		want                  string
	}{
		{name: "single verse", version: "web", book: "John", chapter: 3, lo: 16,
			want: "https://bibletext.co.uk/read/web/john/3/#v16"},
		{name: "range", version: "web", book: "John", chapter: 3, lo: 16, hi: 18,
			want: "https://bibletext.co.uk/read/web/john/3/#v16-18"},
		{name: "chapter only", version: "bsb", book: "Psalms", chapter: 119,
			want: "https://bibletext.co.uk/read/bsb/psalms/119/"},
		{name: "no zero padding", version: "bsb", book: "Psalms", chapter: 119, lo: 105,
			want: "https://bibletext.co.uk/read/bsb/psalms/119/#v105"},
		{name: "hi below lo collapses to single", version: "web", book: "Acts", chapter: 2, lo: 5, hi: 2,
			want: "https://bibletext.co.uk/read/web/acts/2/#v5"},
		{name: "hi equal to lo is single", version: "web", book: "Acts", chapter: 2, lo: 5, hi: 5,
			want: "https://bibletext.co.uk/read/web/acts/2/#v5"},
		{name: "multi-word book", version: "web", book: "Song of Solomon", chapter: 2, lo: 1,
			want: "https://bibletext.co.uk/read/web/song-of-solomon/2/#v1"},
		{name: "numbered book", version: "web", book: "1 Corinthians", chapter: 13, lo: 4, hi: 7,
			want: "https://bibletext.co.uk/read/web/1-corinthians/13/#v4-7"},
		// A licensed version must never appear in a public URL.
		{name: "licensed version falls back to web", version: "nkjv", book: "John", chapter: 3, lo: 16,
			want: "https://bibletext.co.uk/read/web/john/3/#v16"},
		{name: "unknown version falls back to web", version: "zzz", book: "John", chapter: 3,
			want: "https://bibletext.co.uk/read/web/john/3/"},
		// Deuterocanon exists only in the Catholic canon, so the link must say so.
		{name: "deuterocanon forces webc", version: "web", book: "1 Maccabees", chapter: 2, lo: 19, hi: 22,
			want: "https://bibletext.co.uk/read/webc/1-maccabees/2/#v19-22"},
		{name: "webc stays webc", version: "webc", book: "Wisdom", chapter: 3, lo: 1,
			want: "https://bibletext.co.uk/read/webc/wisdom/3/#v1"},
		// Greek Daniel: chapter 13 exists only under webc, but the slug is shared.
		{name: "greek daniel under webc", version: "webc", book: "Daniel", chapter: 13, lo: 45,
			want: "https://bibletext.co.uk/read/webc/daniel/13/#v45"},
		{name: "chapter clamped", version: "web", book: "John", chapter: 0, lo: 1,
			want: "https://bibletext.co.uk/read/web/john/1/#v1"},
	} {
		if got := ShareLinkURL(tc.version, tc.book, tc.chapter, tc.lo, tc.hi); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}

	if got := ShareLinkURL("web", "Nonexistent Book", 1, 1, 0); got != "" {
		t.Errorf("an unknown book must yield no link (so the caller shares text instead), got %q", got)
	}
}

// TestShareLinkURLsAreAlwaysSafe sweeps every book in both canons: no uppercase,
// no spaces, no characters a messenger would mangle, and always a trailing
// slash before any fragment.
func TestShareLinkURLsAreAlwaysSafe(t *testing.T) {
	safe := regexp.MustCompile(`^https://bibletext\.co\.uk/read/(web|webc|bsb)/[a-z0-9-]+/[0-9]+/(#v[0-9]+(-[0-9]+)?)?$`)
	books := append(append([]string{}, NewBibleData().Books...), catholicBooks...)
	for _, b := range books {
		for _, v := range []string{"web", "webc", "bsb", "nkjv", ""} {
			got := ShareLinkURL(v, b, 3, 16, 18)
			if got == "" {
				t.Errorf("%q/%q produced no link", v, b)
				continue
			}
			if !safe.MatchString(got) {
				t.Errorf("unsafe share URL for %q/%q: %q", v, b, got)
			}
			if got != strings.ToLower(got) {
				t.Errorf("share URL must be lowercase: %q", got)
			}
		}
	}
}
