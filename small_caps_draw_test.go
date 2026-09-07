package bibletext

import (
	"reflect"
	"testing"
)

// The divine name is DRAWN in small capitals and STORED as the publisher sent
// it, and a reader who copies a verse gets the publisher's letters back.
func TestSmallCapsAreDrawnButNotStored(t *testing.T) {
	cases := []struct {
		name, text string
		spans      []TextSpan
		wantDrawn  string
		wantCopied string
	}{
		{
			// The common shape: the whole word inside the span in ordinary
			// case. The capital stays full size and the rest shrink.
			name: "whole word", text: "The Lord is my shepherd",
			spans: []TextSpan{{Start: 4, End: 8}}, wantDrawn: "The Lᴏʀᴅ is my shepherd",
			wantCopied: "The LORD is my shepherd",
		},
		{
			// The other shape: a capital outside the span and the remainder
			// capitalised inside it. There is no lower case to work on, so the
			// capitals are what shrink.
			name: "capitals only", text: "O Lord GOD of hosts",
			spans: []TextSpan{{Start: 8, End: 10}}, wantDrawn: "O Lord Gᴏᴅ of hosts",
			wantCopied: "O Lord GOD of hosts",
		},
		{
			name: "the name of Christ", text: "call His name Jesus",
			spans: []TextSpan{{Start: 14, End: 19}}, wantDrawn: "call His name Jᴇꜱᴜꜱ",
			wantCopied: "call His name JESUS",
		},
		{
			name: "no spans changes nothing", text: "In the beginning",
			spans: nil, wantDrawn: "In the beginning", wantCopied: "In the beginning",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Verse{Text: tc.text, SmallCaps: tc.spans}
			runs := applySmallCaps(v, []verseRun{{Text: tc.text}})
			if len(runs) != 1 {
				t.Fatalf("got %d runs, want 1", len(runs))
			}
			if runs[0].Text != tc.wantDrawn {
				t.Errorf("drawn:\n got  %q\n want %q", runs[0].Text, tc.wantDrawn)
			}
			if v.Text != tc.text {
				t.Errorf("the stored text was changed: %q", v.Text)
			}
			// Rune counts must match, or every offset the app records moves.
			if a, b := len([]rune(runs[0].Text)), len([]rune(tc.text)); a != b {
				t.Errorf("drawn text is %d runes and stored is %d — offsets would shift", a, b)
			}
			// And what a reader copies carries the distinction in the way
			// plain text always has: as capitals. A small capital cannot say
			// which case it stood for, and uppercase is both the convention
			// and what the publisher sent wherever the span held capitals.
			if got := outboundText(runs[0].Text); got != tc.wantCopied {
				t.Errorf("copied out:\n got  %q\n want %q", got, tc.wantCopied)
			}
		})
	}
}

// The substitution has to survive the red-letter split, since a divine name
// inside Christ's own words is the commonest case of all.
func TestSmallCapsSurviveTheRedLetterSplit(t *testing.T) {
	v := Verse{Text: "He said, The Lord is God.", SmallCaps: []TextSpan{{Start: 13, End: 17}}}
	runs := applySmallCaps(v, []verseRun{
		{Text: "He said, "},
		{Text: "The Lord is God.", Red: true},
	})
	want := []verseRun{
		{Text: "He said, "},
		{Text: "The Lᴏʀᴅ is God.", Red: true},
	}
	if !reflect.DeepEqual(runs, want) {
		t.Errorf("across runs:\n got  %+v\n want %+v", runs, want)
	}
}
