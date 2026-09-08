package bibletext

import (
	"reflect"
	"testing"
)

// The translators' supplied words are the edition's own disclosure of where a
// translator made a judgement, and they belong in italic. They are decided
// here, once, so that no surface has to work them out again.
func TestSuppliedWordsBecomeItalicRuns(t *testing.T) {
	cases := []struct {
		name  string
		verse Verse
		in    []verseRun
		want  []verseRun
	}{
		{
			name: "a supplied word inside a sentence",
			verse: Verse{
				Text:     "There is none righteous.",
				Supplied: []TextSpan{{Start: 6, End: 8}},
			},
			in: []verseRun{{Text: "There is none righteous."}},
			want: []verseRun{
				{Text: "There "},
				{Text: "is", Italic: true},
				{Text: " none righteous."},
			},
		},
		{
			name: "supplied words keep the red they were in",
			verse: Verse{
				Text:     "I am he.",
				Supplied: []TextSpan{{Start: 2, End: 4}},
			},
			in: []verseRun{{Text: "I am he.", Red: true}},
			want: []verseRun{
				{Text: "I ", Red: true},
				{Text: "am", Red: true, Italic: true},
				{Text: " he.", Red: true},
			},
		},
		{
			name:  "no spans changes nothing",
			verse: Verse{Text: "In the beginning"},
			in:    []verseRun{{Text: "In the beginning"}},
			want:  []verseRun{{Text: "In the beginning"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applySupplied(tc.verse, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("runs:\n got  %+v\n want %+v", got, tc.want)
			}
			// Whatever the split, the words are unchanged and complete.
			var joined string
			for _, r := range got {
				joined += r.Text
			}
			if joined != tc.verse.Text {
				t.Errorf("the split changed the verse:\n got  %q\n want %q", joined, tc.verse.Text)
			}
		})
	}
}

// The two decisions must compose: a divine name inside a supplied span, and a
// supplied span inside Christ's words, both have to survive the other.
func TestSuppliedAndSmallCapsCompose(t *testing.T) {
	v := Verse{
		Text:      "The Lord is God.",
		Supplied:  []TextSpan{{Start: 9, End: 11}},
		SmallCaps: []TextSpan{{Start: 4, End: 8}},
	}
	got := finishRuns(v, []verseRun{{Text: v.Text, Red: true}})
	want := []verseRun{
		{Text: "The Lᴏʀᴅ ", Red: true},
		{Text: "is", Red: true, Italic: true},
		{Text: " God.", Red: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("composed:\n got  %+v\n want %+v", got, want)
	}
	if v.Text != "The Lord is God." {
		t.Errorf("the stored text was changed: %q", v.Text)
	}
}
