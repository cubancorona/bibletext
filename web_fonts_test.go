package bibletext

import (
	"bytes"
	"testing"
)

// The NKJV pages' two supplements are real WOFF2 files and are what they say:
// a face of a few kilobytes holding the small capitals, and an italic the size
// of the regular subset. A swapped embed or an empty file would pass the build
// and render as the fallback serif on every NKJV page.
func TestWebScriptureSupplementsAreWOFF2(t *testing.T) {
	for _, tc := range []struct {
		name     string
		data     []byte
		min, max int
	}{
		{"small capitals", WebScriptureFontSmallCaps(), 1 << 10, 8 << 10},
		{"bold small capitals", WebScriptureFontBoldSmallCaps(), 1 << 10, 8 << 10},
		{"italic small capitals", WebScriptureFontItalicSmallCaps(), 1 << 10, 8 << 10},
		{"italic", WebScriptureFontItalic(), 16 << 10, 48 << 10},
		{"Hebrew", WebHebrewFont(), 4 << 10, 24 << 10},
		{"Greek", WebScriptureFontGreek(), 1 << 10, 12 << 10},
	} {
		if !bytes.HasPrefix(tc.data, []byte("wOF2")) {
			t.Errorf("the %s face is not a WOFF2 file", tc.name)
		}
		if n := len(tc.data); n < tc.min || n > tc.max {
			t.Errorf("the %s face is %d bytes, outside %d..%d — rebuild it with scripts/build-web-nkjv-fonts.sh",
				tc.name, n, tc.min, tc.max)
		}
	}
	if bytes.Equal(WebScriptureFontItalic(), WebScriptureFontRegular()) {
		t.Error("the italic embed is the regular face")
	}
	for _, pair := range [][2][]byte{
		{WebScriptureFontSmallCaps(), WebScriptureFontBoldSmallCaps()},
		{WebScriptureFontSmallCaps(), WebScriptureFontItalicSmallCaps()},
		{WebScriptureFontBoldSmallCaps(), WebScriptureFontItalicSmallCaps()},
	} {
		if bytes.Equal(pair[0], pair[1]) {
			t.Error("two of the small-capital supplements are the same file; each cut needs its own")
		}
	}
	if !bytes.Contains(WebHebrewFontLicense(), []byte("SIL Open Font License")) {
		t.Error("the Hebrew face's licence is not the OFL")
	}
}

// The Hebrew and Greek supplements' code points are the unicode-ranges the
// stylesheet declares, so each list must be its own script, sorted and each
// code point listed once; the build script reads the same lists
// (scripts/build-web-nkjv-fonts.sh) and checks the faces carry exactly them.
func TestWebSupplementRunesAreTheirScriptSortedOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		runes  func() []rune
		inside func(rune) bool
	}{
		{"Hebrew", WebHebrewRunes, func(r rune) bool { return r >= 0x0590 && r <= 0x05FF }},
		{"Greek", WebGreekRunes, func(r rune) bool { return (r >= 0x0370 && r <= 0x03FF) || (r >= 0x1F00 && r <= 0x1FFF) }},
	} {
		runes := tc.runes()
		if len(runes) == 0 {
			t.Fatalf("no %s code points", tc.name)
		}
		for i, r := range runes {
			if !tc.inside(r) {
				t.Errorf("%U is not %s", r, tc.name)
			}
			if i > 0 && r <= runes[i-1] {
				t.Errorf("%s: %U follows %U: the list is not sorted and unique", tc.name, r, runes[i-1])
			}
		}
		runes[0] = 'x'
		if tc.runes()[0] == 'x' {
			t.Errorf("Web%sRunes hands out the table itself", tc.name)
		}
	}
}
