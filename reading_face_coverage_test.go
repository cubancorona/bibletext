package bibletext

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/image/font/sfnt"
)

// The reading face draws the translators' apparatus as well as their text, and
// the apparatus is where the alphabets are: Greek in the New Testament notes,
// Hebrew in the Old. A face that lacks a codepoint does not fail — the toolkit
// falls back per RUNE — so the failure is silent and cosmetic and shows up as
// one word set in two different fonts, half of it not matching the English
// beside it.
//
// This is worth pinning rather than eyeballing, because the face has already
// been changed once under it.

func faceCovers(t *testing.T, ttf []byte, s string) (missing []rune) {
	t.Helper()
	f, err := sfnt.Parse(ttf)
	if err != nil {
		t.Fatalf("parse face: %v", err)
	}
	var buf sfnt.Buffer
	seen := map[rune]bool{}
	for _, r := range s {
		if r < 0x80 || seen[r] {
			continue
		}
		seen[r] = true
		if i, err := f.GlyphIndex(&buf, r); err != nil || i == 0 {
			missing = append(missing, r)
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return missing
}

// The words the worklist named when it recorded this as open.
func TestTheReadingFaceSetsTheGreekTheNotesUse(t *testing.T) {
	for _, word := range []string{
		"μονογενη",  // John 3:16's note
		"ἐπίσκοπον", // the headline example, Greek Extended
		"Χριστός", "θεός", "λόγος",
	} {
		if missing := faceCovers(t, readingFontRegular, word); len(missing) > 0 {
			t.Errorf("the reading face cannot set %q; missing %q", word, string(missing))
		}
	}
}

// The whole Greek block, not just the samples — the earlier face carried three
// codepoints of it, which is what made a Greek word split across two fonts.
func TestTheReadingFaceCoversTheGreekBlocks(t *testing.T) {
	f, err := sfnt.Parse(readingFontRegular)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	count := func(lo, hi rune) (have, total int) {
		for r := lo; r <= hi; r++ {
			if !unicode.IsGraphic(r) {
				continue
			}
			total++
			if i, err := f.GlyphIndex(&buf, r); err == nil && i != 0 {
				have++
			}
		}
		return have, total
	}
	basicHave, basicTotal := count(0x0370, 0x03FF)
	extHave, extTotal := count(0x1F00, 0x1FFF)
	t.Logf("reading face Greek coverage: basic %d/%d, extended %d/%d",
		basicHave, basicTotal, extHave, extTotal)

	// Basic Greek is what a footnote's transliterated headword needs; Greek
	// Extended is what a quoted New Testament phrase needs.
	if basicHave < basicTotal/2 {
		t.Errorf("basic Greek coverage is %d of %d — a Greek word will split across two fonts",
			basicHave, basicTotal)
	}
	if extHave == 0 {
		t.Error("no Greek Extended at all; a quoted phrase like ἐπίσκοπον cannot be set")
	}
}

// And the corpus, not just a sample of it: every non-ASCII rune the shipped
// notes actually contain must be drawable BY THE FACE THAT DRAWS IT.
//
// There are two, and that is the point of this test rather than an awkwardness
// in it: Junicode sets the Latin and the Greek, and Ezra SIL sets the Hebrew,
// so a Hebrew consonant absent from Junicode is correct rather than a defect.
// Checking the corpus against one face reports the whole Hebrew alphabet as
// missing, which is how this test first ran.
//
// Format characters are excluded: a bidi mark (U+200E and kin) steers the
// layout and has no glyph to lack.
func TestTheReadingFaceCoversEveryRuneTheNotesContain(t *testing.T) {
	body, err := os.ReadFile("build/biblecache/web.json")
	if err != nil {
		t.Skip("build/biblecache/web.json not present (build/ is gitignored); skipping")
	}
	var doc struct {
		Books []helloAOBook `json:"books"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	notes := 0
	for _, b := range doc.Books {
		for _, w := range b.Chapters {
			for _, fn := range w.Chapter.Footnotes {
				notes++
				all.WriteString(fn.Text)
			}
		}
	}
	if notes < 1000 {
		t.Fatalf("only %d notes read; this test proves nothing", notes)
	}
	t.Logf("checked the runes of %d notes", notes)

	var latin, hebrew strings.Builder
	for _, r := range all.String() {
		switch {
		case unicode.Is(unicode.Cf, r):
			// A format character (a bidi mark) has no glyph by design.
		case unicode.Is(unicode.Hebrew, r):
			hebrew.WriteRune(r)
		default:
			latin.WriteRune(r)
		}
	}
	if hebrew.Len() == 0 {
		t.Fatal("no Hebrew found in the notes; the split below is not being exercised")
	}
	for _, tc := range []struct {
		face string
		ttf  []byte
		text string
	}{
		{"Junicode (Latin and Greek)", readingFontRegular, latin.String()},
		{"Ezra SIL (Hebrew)", readingFontHebrew, hebrew.String()},
	} {
		if missing := faceCovers(t, tc.ttf, tc.text); len(missing) > 0 {
			var names []string
			for _, r := range missing {
				names = append(names, string(r))
			}
			t.Errorf("%s cannot draw %d rune(s) the notes contain: %s",
				tc.face, len(missing), strings.Join(names, " "))
		}
	}
}
