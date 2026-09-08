package bibletext

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The edition sets the divine name in small capitals, and the feed sends that
// span two different ways. Neither may reach the stored text as a changed
// letter: what is kept is the publisher's own characters plus the range the
// feature applies to.
func TestSmallCapsKeepThePublishersCharacters(t *testing.T) {
	cases := []struct {
		name      string
		content   string
		wantText  string
		wantSpans []TextSpan
		covers    []string
	}{
		{
			// The common shape: the whole word inside the span, with its
			// remainder in lower case. A renderer asks for smcp and the
			// initial stays full size.
			name: "whole word in the span",
			content: `[{"name":"para","type":"tag","attrs":{"style":"p"},"items":[
			  {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[{"type":"text","text":"1"}]},
			  {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[
			    {"type":"text","text":"Lord","attrs":{"verseId":"PSA.1.1"}}]},
			  {"type":"text","text":" is my shepherd.","attrs":{"verseId":"PSA.1.1"}}]}]`,
			wantText:  "Lord is my shepherd.",
			wantSpans: []TextSpan{{Start: 0, End: 4}},
			covers:    []string{"Lord"},
		},
		{
			// The other shape: a full-size capital OUTSIDE the span and the
			// remainder already capitalised INSIDE it. A renderer asks for
			// c2sc and the face sets "OD" as small capitals.
			name: "capital outside, remainder inside",
			content: `[{"name":"para","type":"tag","attrs":{"style":"p"},"items":[
			  {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[{"type":"text","text":"1"}]},
			  {"type":"text","text":"O Lord G","attrs":{"verseId":"PSA.1.1"}},
			  {"name":"char","type":"tag","attrs":{"style":"sc"},"items":[
			    {"type":"text","text":"OD","attrs":{"verseId":"PSA.1.1"}}]},
			  {"type":"text","text":" of hosts.","attrs":{"verseId":"PSA.1.1"}}]}]`,
			wantText:  "O Lord GOD of hosts.",
			wantSpans: []TextSpan{{Start: 8, End: 10}},
			covers:    []string{"OD"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(tc.content), "Psalms", 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(vs) != 1 {
				t.Fatalf("got %d verses, want 1", len(vs))
			}
			if vs[0].Text != tc.wantText {
				t.Errorf("text:\n got  %q\n want %q", vs[0].Text, tc.wantText)
			}
			if !reflect.DeepEqual(vs[0].SmallCaps, tc.wantSpans) {
				t.Fatalf("spans:\n got  %+v\n want %+v", vs[0].SmallCaps, tc.wantSpans)
			}
			runes := []rune(vs[0].Text)
			for i, sp := range vs[0].SmallCaps {
				if got := string(runes[sp.Start:sp.End]); got != tc.covers[i] {
					t.Errorf("span %d covers %q, want %q", i, got, tc.covers[i])
				}
			}
		})
	}
}

// A verse whose FIRST content is a marked span used to lose that span: the
// bracket was only written when the verse already had a text builder, so an
// opening bracket was dropped and its closing one went unmatched.
func TestSpanOpeningAVerseIsKept(t *testing.T) {
	content := `[{"name":"para","type":"tag","attrs":{"style":"p"},"items":[
	  {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[{"type":"text","text":"1"}]},
	  {"name":"char","type":"tag","attrs":{"style":"it"},"items":[
	    {"type":"text","text":"There is","attrs":{"verseId":"PSA.1.1"}}]},
	  {"type":"text","text":" none righteous.","attrs":{"verseId":"PSA.1.1"}}]}]`
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Psalms", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1", len(vs))
	}
	if want := "There is none righteous."; vs[0].Text != want {
		t.Errorf("text:\n got  %q\n want %q", vs[0].Text, want)
	}
	want := []TextSpan{{Start: 0, End: 8}}
	if !reflect.DeepEqual(vs[0].Supplied, want) {
		t.Errorf("supplied span opening the verse:\n got  %+v\n want %+v", vs[0].Supplied, want)
	}
}

// A marked span at the very start of a POETRY verse must not gain a line
// break: the join is decided on what the reader sees, and a bracket is not
// something the reader sees. The same mistake with the footnote sentinel once
// put a blank first line on every verse in the poetry canon that opens with a
// cross-reference.
func TestSpanAtVerseStartAddsNoLineBreak(t *testing.T) {
	content := `[
	  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
	    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1"},"items":[{"type":"text","text":"1"}]}]},
	  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
	    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[
	      {"type":"text","text":"Lord","attrs":{"verseId":"PSA.1.1"}}]},
	    {"type":"text","text":", hear my prayer.","attrs":{"verseId":"PSA.1.1"}}]}]`
	vs, _, _, _, err := decodeAPIBibleChapter(json.RawMessage(content), "Psalms", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1", len(vs))
	}
	if got := vs[0].Text; got != "Lord, hear my prayer." {
		t.Errorf("verse opening with a marked span:\n got  %q\n want %q",
			got, "Lord, hear my prayer.")
	}
	if len(vs[0].SmallCaps) != 1 || vs[0].SmallCaps[0] != (TextSpan{Start: 0, End: 4}) {
		t.Errorf("small-caps span = %+v, want one covering \"Lord\"", vs[0].SmallCaps)
	}
}

// withoutSentinels must know every sentinel. A new one that it does not strip
// makes a part-assembled verse look non-empty, which is how the join gets
// decided wrongly.
func TestWithoutSentinelsKnowsEverySentinel(t *testing.T) {
	all := string([]rune{
		footnoteSentinel, suppliedOpen, suppliedClose, smallCapsOpen, smallCapsClose,
	})
	if got := withoutSentinels("a" + all + "b"); got != "ab" {
		t.Errorf("withoutSentinels left a marker behind: %q", got)
	}
	// The control: it must not eat ordinary text.
	if got := withoutSentinels("Vanity of vanities"); got != "Vanity of vanities" {
		t.Errorf("withoutSentinels changed plain text: %q", got)
	}
}
