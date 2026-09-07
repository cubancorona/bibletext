package bibletext

// THE PAGE'S TYPOGRAPHY MUST NOT LEAVE THE PAGE. Every verb that turns a
// selection into text — copy, share, ask an assistant, a link — carries the
// reading surface's own characters with it unless they are taken out first.

import "testing"

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
