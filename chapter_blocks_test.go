package bibletext

import (
	"reflect"
	"testing"
)

func blockShape(bs []chapterBlock) []string {
	var out []string
	for _, b := range bs {
		if b.IsHeading() {
			out = append(out, "H:"+b.Heading.Text)
			continue
		}
		s := "P:"
		for i, v := range b.Verses {
			if i > 0 {
				s += ","
			}
			s += itoa(v.Verse)
		}
		out = append(out, s)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestChapterBlocksPlaceTheHeadings(t *testing.T) {
	verses := []Verse{
		{Verse: 1, ParaStart: true}, {Verse: 2}, {Verse: 3},
		{Verse: 4, ParaStart: true}, {Verse: 5},
	}
	cases := []struct {
		name  string
		heads []Heading
		want  []string
	}{
		{
			name: "no headings is just the paragraphs",
			want: []string{"P:1,2,3", "P:4,5"},
		},
		{
			name:  "a heading opens the chapter",
			heads: []Heading{{Text: "The Beatitudes", BeforeVerse: 1}},
			want:  []string{"H:The Beatitudes", "P:1,2,3", "P:4,5"},
		},
		{
			name:  "a heading on a paragraph's own first verse",
			heads: []Heading{{Text: "The Feeding", BeforeVerse: 4}},
			want:  []string{"P:1,2,3", "H:The Feeding", "P:4,5"},
		},
		{
			// The publisher's placement wins and the paragraph splits: a
			// heading that opened nothing would not read as a heading.
			name:  "a heading partway through a paragraph splits it",
			heads: []Heading{{Text: "A Turn", BeforeVerse: 2}},
			want:  []string{"P:1", "H:A Turn", "P:2,3", "P:4,5"},
		},
		{
			name: "two headings stacked on one verse keep their order",
			heads: []Heading{
				{Text: "Book Two", BeforeVerse: 4},
				{Text: "A Psalm", BeforeVerse: 4},
			},
			want: []string{"P:1,2,3", "H:Book Two", "H:A Psalm", "P:4,5"},
		},
		{
			// The decoders do not assume a heading has a verse after it.
			name:  "a heading naming no verse is dropped",
			heads: []Heading{{Text: "Orphan", BeforeVerse: 0}},
			want:  []string{"P:1,2,3", "P:4,5"},
		},
		{
			name:  "an empty heading is dropped",
			heads: []Heading{{Text: "", BeforeVerse: 1}},
			want:  []string{"P:1,2,3", "P:4,5"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bd := &BibleData{Headings: map[string]map[int][]Heading{
				"Matthew": {5: tc.heads},
			}}
			got := blockShape(chapterBlocksFor(bd, "Matthew", 5, verses))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("blocks:\n got  %v\n want %v", got, tc.want)
			}
			// Whatever the shape, every verse appears exactly once and in order.
			var seen []int
			for _, b := range chapterBlocksFor(bd, "Matthew", 5, verses) {
				for _, v := range b.Verses {
					seen = append(seen, v.Verse)
				}
			}
			if !reflect.DeepEqual(seen, []int{1, 2, 3, 4, 5}) {
				t.Errorf("the blocks lost or reordered verses: %v", seen)
			}
		})
	}
}
