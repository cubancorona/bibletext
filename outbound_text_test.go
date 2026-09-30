package bibletext

// THE PAGE'S TYPOGRAPHY MUST NOT LEAVE THE PAGE. Every verb that turns a
// selection into text — copy, share, ask an assistant, a link — carries the
// reading surface's own characters with it unless they are taken out first.

import (
	"testing"
	"unicode/utf8"
)

func TestOutboundTextKeepsThePublishersWordsAndDropsOurOwn(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{
			"the no-break space glued to a verse number becomes an ordinary space",
			"16\u00a0For God so loved the world",
			"16 For God so loved the world",
		},
		{
			"a superscript verse number becomes an ordinary number",
			"\u00b9\u2076 For God so loved the world",
			"16 For God so loved the world",
		},
		{
			"the reporter layout's indent is the page's alone and does not travel",
			"\u2003\u2002In the beginning God created",
			"In the beginning God created",
		},
		{
			"the publisher's own words, spacing and line breaks are untouched",
			"The LORD is my shepherd;\nI shall not want.",
			"The LORD is my shepherd;\nI shall not want.",
		},
		{
			"an ordinary space is not a no-break space and is left alone",
			"one two  three",
			"one two  three",
		},
		{"nothing at all", "", ""},
	} {
		if got := outboundText(tc.in); got != tc.want {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
}

// The characters this removes are exactly the ones the surfaces write. If a
// surface ever changes what it writes, this test is where that shows up.
func TestTheCharactersWeStripAreTheOnesTheSurfacesWrite(t *testing.T) {
	if got := superscriptNumber(16); outboundText(got) != "16" {
		t.Errorf("the Fyne panes draw %q, which does not clean to 16", got)
	}
	// The HTML dialects join a number to its verse with a no-break space, and
	// the reporter page indents with an em-space and an en-space.
	for _, r := range []rune{noBreakSpace, emSpace, enSpace} {
		if outboundText(string(r)) == string(r) {
			t.Errorf("%q survives the clean", r)
		}
	}
}

// outboundForm hands back text with none of the page's typography in it as it
// came, without copying it (hasOutboundRewrite). That shortcut must see every
// character the copying loop would change, or the character rides out. It
// never looks up ASCII, which is sound only while none of those characters is
// ASCII; and an invalid byte must still go out as U+FFFD, as the loop writes it.
func TestOutboundShortcutSeesAllThePageTypography(t *testing.T) {
	for r := rune(0); r < utf8.RuneSelf; r++ {
		if superToDigit[r] != 0 || smallCapitalToLetter[r] != 0 ||
			r == noBreakSpace || r == emSpace || r == enSpace {
			t.Errorf("%U is rewritten on the way out, but hasOutboundRewrite never looks at ASCII", r)
		}
	}
	for _, keepSmallCaps := range []bool{false, true} {
		for r := range superToDigit {
			if !hasOutboundRewrite("x"+string(r), keepSmallCaps) {
				t.Errorf("superscript %q is not seen (keepSmallCaps=%v)", r, keepSmallCaps)
			}
		}
		for _, r := range []rune{noBreakSpace, emSpace, enSpace} {
			if !hasOutboundRewrite("x"+string(r), keepSmallCaps) {
				t.Errorf("%U is not seen (keepSmallCaps=%v)", r, keepSmallCaps)
			}
		}
		for r := range smallCapitalToLetter {
			if got := hasOutboundRewrite("x"+string(r), keepSmallCaps); got == keepSmallCaps {
				t.Errorf("small capital %q: rewrite %v with keepSmallCaps=%v", r, got, keepSmallCaps)
			}
		}
	}
	for in, want := range map[string]string{
		"a\xffb":           "a\uFFFDb",
		"\xe2\x80":         "\uFFFD\uFFFD",
		"“Truly,” he said": "“Truly,” he said",
	} {
		if got := sharedText(in); got != want {
			t.Errorf("sharedText(%q) = %q, want %q", in, got, want)
		}
		if got := outboundText(in); got != want {
			t.Errorf("outboundText(%q) = %q, want %q", in, got, want)
		}
	}
}
