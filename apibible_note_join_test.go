package bibletext

// A NOTE AT THE START OF A VERSE: the sentinel it leaves in the builder is
// not text, so the poem break or prose space owed at the paragraph boundary
// must not be written in front of the verse's first word. Every NKJV
// cross-reference sits at the start of its verse, so this is the common
// case, not the edge — a whole canon once opened 1,995 poetry verses with a
// blank line.

import (
	"encoding/json"
	"strings"
	"testing"
)

const noteAtVerseStartX = `{"name":"note","type":"tag","attrs":{"style":"x","caller":"-","id":"1CH.16.23!x.1","verseId":"1CH.16.23"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"xo"},"items":[{"type":"text","text":"16:23 "}]},
      {"name":"char","type":"tag","attrs":{"style":"xt"},"items":[{"type":"text","text":"Psalm 96:1-13"}]}
    ]}`

func chapterWithNoteAtVerseStart(paraStyle string) json.RawMessage {
	return json.RawMessage(`[
  {"name":"para","type":"tag","attrs":{"style":"` + paraStyle + `"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"22","sid":"1CH 16:22"},"items":[{"type":"text","text":"22"}]},
    {"type":"text","text":"Saying, Do not touch My anointed ones.","attrs":{"verseId":"1CH.16.22"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"` + paraStyle + `"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"23","sid":"1CH 16:23"},"items":[{"type":"text","text":"23"}]},
    ` + noteAtVerseStartX + `,
    {"type":"text","text":"Sing to the LORD, all the earth;","attrs":{"verseId":"1CH.16.23"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q2"},"items":[
    {"type":"text","text":"Proclaim the good news of His salvation.","attrs":{"verseId":"1CH.16.23"}}
  ]}
]`)
}

func TestNoteAtVerseStartTakesNoJoin(t *testing.T) {
	for _, tc := range []struct {
		style, want string
	}{
		{"q1", "Sing to the LORD, all the earth;\nProclaim the good news of His salvation."},
		{"p", "Sing to the LORD, all the earth;\nProclaim the good news of His salvation."},
	} {
		vs, _, _, err := decodeAPIBibleChapter(chapterWithNoteAtVerseStart(tc.style), "1 Chronicles", 16)
		if err != nil {
			t.Fatal(err)
		}
		if len(vs) != 2 {
			t.Fatalf("%s: got %d verses, want 2", tc.style, len(vs))
		}
		v := vs[1]
		if v.Text != tc.want {
			t.Errorf("%s: verse 23:\n got  %q\n want %q", tc.style, v.Text, tc.want)
		}
		if strings.HasPrefix(v.Text, "\n") || strings.HasPrefix(v.Text, " ") {
			t.Errorf("%s: the join was written in front of the first word: %q", tc.style, v.Text)
		}
		if len(v.Footnotes) != 1 || v.Footnotes[0].Anchor != 0 {
			t.Errorf("%s: the note must anchor at the verse's start: %+v", tc.style, v.Footnotes)
		}
		// The verse before is untouched, and the boundary's break for a
		// note-free verse is still written (the q2 line inside verse 23).
		if vs[0].Text != "Saying, Do not touch My anointed ones." {
			t.Errorf("%s: verse 22 = %q", tc.style, vs[0].Text)
		}
	}
}
