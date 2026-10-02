package main

// THE TREES THE GLYPH GUARD IS TESTED ON. scripts/test-site-nkjv-guards.sh asks
// for a whole site in one state of the switch, written through run() with the
// network replaced, and then holds scripts/check-site-glyphs.py to it: passing
// the tree as built, refusing it with a face taken away.
//
// Everything that reaches the page is SYNTHETIC — the fixture editions of
// nkjv_fixture_test.go and site_off_golden_test.go — but the characters are the
// ones the real editions set and the faces are the real ones: every Hebrew
// letter and mark the site draws, the Greek of the WEB's notes, and the divine
// name in small capitals in a verse, a heading, a title, a supplied word and
// Christ's words.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	bibletext "github.com/cubancorona/bibletext"
)

// glyphFixtureEnv names the directory the tree is written to. Unset, the test
// is skipped: it writes outside the test's own temporary directory, which only
// the script asks for.
const glyphFixtureEnv = "BIBLETEXT_GLYPH_FIXTURE_OUT"

// smallCapsSpan marks the first "Lord" in text as the edition marks the divine
// name.
func smallCapsSpan(text string) []bibletext.TextSpan {
	at := strings.Index(text, "Lord")
	start := utf8.RuneCountInString(text[:at])
	return []bibletext.TextSpan{{Start: start, End: start + 4}}
}

// hebrewFixture is every Hebrew code point the site draws, as one string.
func hebrewFixture() string {
	return string(bibletext.WebHebrewRunes())
}

// greekFixture is every Greek code point the site draws, as one string.
func greekFixture() string {
	return string(bibletext.WebGreekRunes())
}

func glyphFixtureVersions(t *testing.T) []loadedVersion {
	published := goldenFixtureVersions()
	for _, v := range published {
		// A note in the public-domain editions carrying Hebrew and Greek, as
		// the WEB's notes on the divine name and on the Greek do.
		john := v.bible.Verses["John"][3]
		john[2].Footnotes = []bibletext.Footnote{{Anchor: 4,
			Text: "Fixture note: " + hebrewFixture() + " and " + greekFixture() + "."}}
		// The golden fixture gives its public-domain psalm a small-capital
		// span to reach a renderer branch; no public-domain edition marks
		// one, and its pages carry no small-capital face.
		for i := range v.bible.Verses["Psalms"][3] {
			v.bible.Verses["Psalms"][3][i].SmallCaps = nil
		}
	}
	return published
}

func glyphLicensedEdition(t *testing.T, ref *bibletext.BibleData) bibletext.LicensedEdition {
	ed := richLicensedEdition(t, ref)
	bd := ed.Bible
	// Psalm 3 carries everything the NKJV sets in a face of its own: the
	// stanza letters of an acrostic (set bold, in the Hebrew face), the divine
	// name in small capitals in a section heading (bold), in its title and in
	// a supplied word (italic), and in a verse.
	var letters []rune
	for _, r := range bibletext.WebHebrewRunes() {
		if r >= 0x05D0 {
			letters = append(letters, r)
		}
	}
	heading := "The Lord Keeps the Fixture"
	bd.Headings["Psalms"] = map[int][]bibletext.Heading{3: {
		{Text: string(letters) + " Fixture", Style: "qa", BeforeVerse: 1},
		{Text: heading, Style: "s", BeforeVerse: 2, SmallCaps: smallCapsSpan(heading)},
	}}
	title := "A licensed fixture title naming the Lord."
	bd.Superscriptions["Psalms"][3] = bibletext.Superscription{Text: title, SmallCaps: smallCapsSpan(title)}
	ps := bd.Verses["Psalms"][3]
	if len(ps) < 2 {
		t.Fatalf("the fixture's Psalm 3 has %d verses", len(ps))
	}
	supplied := "Fixture words supplied about the Lord"
	ps[0].Text = supplied + ", and plain words."
	ps[0].Supplied = []bibletext.TextSpan{{Start: 0, End: utf8.RuneCountInString(supplied)}}
	ps[0].SmallCaps = smallCapsSpan(ps[0].Text)
	ps[1].Text = "A fixture verse naming the Lord."
	ps[1].SmallCaps = smallCapsSpan(ps[1].Text)
	return ed
}

// TestWriteGlyphFixtureSite writes the site in the state glyphFixtureEnv's
// sibling asks for ("on" or "off") to the directory it names.
func TestWriteGlyphFixtureSite(t *testing.T) {
	out := os.Getenv(glyphFixtureEnv)
	if out == "" {
		t.Skip("writes the glyph guard's fixture tree for scripts/test-site-nkjv-guards.sh")
	}
	on := os.Getenv("BIBLETEXT_GLYPH_FIXTURE_STATE") == "on"
	published := glyphFixtureVersions(t)
	standIn(t, published, func(string) (bibletext.LicensedEdition, error) {
		return glyphLicensedEdition(t, referenceOf(t, published)), nil
	})
	if on {
		t.Setenv(siteKeyEnv, testSiteKey)
	}
	if err := run(runOptions{out: filepath.Clean(out), cache: t.TempDir(), nkjvText: on, now: fixedNow}); err != nil {
		t.Fatalf("run: %v", err)
	}
}
