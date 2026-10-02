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
		{"italic", WebScriptureFontItalic(), 16 << 10, 48 << 10},
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
}
