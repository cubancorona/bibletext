package bibletext

// THE PSALM TITLES on the API.Bible feed: the "d" paragraph is the chapter's
// superscription (BibleData.Superscriptions), never verse text, and it
// belongs to the chapter whose first verse follows it — which, on the
// passages endpoint, is not the chapter the decoder was in when it read it.

import (
	"encoding/json"
	"strings"
	"testing"
)

// A titled Psalm as the chapter endpoint serves it: the title carries a
// cross-reference note mid-sentence, and verse 1 opens with a divine-name
// span, so both the note and the char-span handling are exercised on the
// title's side of the line.
const psalmTitledChapterContent = `[
  {"name":"para","type":"tag","attrs":{"style":"d"},"items":[
    {"type":"text","text":"A Psalm of David","attrs":{"verseId":"PSA.3.1"}},
    {"name":"note","type":"tag","attrs":{"style":"x","caller":"-","id":"PSA.3.1!x.1","verseId":"PSA.3.1"},"items":[
      {"name":"char","type":"tag","attrs":{"style":"xo"},"items":[{"type":"text","text":"3:title "}]},
      {"name":"char","type":"tag","attrs":{"style":"xt"},"items":[{"type":"text","text":"2 Samuel 15:13-17"}]}
    ]},
    {"type":"text","text":" when he fled from Absalom his son.","attrs":{"verseId":"PSA.3.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 3:1"},"items":[{"type":"text","text":"1"}]},
    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord","attrs":{"verseId":"PSA.3.1"}}]},
    {"type":"text","text":", how they have increased who trouble me!","attrs":{"verseId":"PSA.3.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q2"},"items":[
    {"type":"text","text":"Many are they who rise up against me.","attrs":{"verseId":"PSA.3.1"}}
  ]}
]`

const psalm3LastVersePara = `{"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"8","sid":"PSA 3:8"},"items":[{"type":"text","text":"8"}]},
    {"type":"text","text":"Salvation belongs to the LORD.","attrs":{"verseId":"PSA.3.8"}}]}`

const psalm4FirstVersePara = `{"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 4:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"Hear me when I call, O God of my righteousness!","attrs":{"verseId":"PSA.4.1"}}]}`

const psalm4Title = "To the Chief Musician. With stringed instruments. A Psalm of David."

// psalm4TitlePara is Psalm 4's "d" paragraph, with or without the provider's
// verseId stamp on its text.
func psalm4TitlePara(verseID string) string {
	attrs := ""
	if verseID != "" {
		attrs = `,"attrs":{"verseId":"` + verseID + `"}`
	}
	return `{"name":"para","type":"tag","attrs":{"style":"d"},"items":[{"type":"text","text":"` + psalm4Title + `"` + attrs + `}]}`
}

func passageOf(paras ...string) json.RawMessage {
	return json.RawMessage("[" + strings.Join(paras, ",") + "]")
}

func TestDecodeAPIBibleChapterKeepsThePsalmTitleBesideTheChapter(t *testing.T) {
	vs, _, sup, err := decodeAPIBibleChapter(json.RawMessage(psalmTitledChapterContent), "Psalms", 3)
	if err != nil {
		t.Fatal(err)
	}
	if sup.Text != "A Psalm of David when he fled from Absalom his son." {
		t.Errorf("superscription = %q", sup.Text)
	}
	if strings.ContainsRune(sup.Text, footnoteSentinel) {
		t.Errorf("sentinel left in the title: %q", sup.Text)
	}
	if len(sup.Footnotes) != 1 {
		t.Fatalf("title notes = %+v, want the one cross-reference", sup.Footnotes)
	}
	note := sup.Footnotes[0]
	if !strings.Contains(note.Text, "2 Samuel 15:13-17") {
		t.Errorf("title note body = %q", note.Text)
	}
	if note.Kind != apiBibleNoteKind("x") {
		t.Errorf("title note kind = %v, want a cross-reference", note.Kind)
	}
	if want := len([]rune("A Psalm of David")); note.Anchor != want {
		t.Errorf("title note anchor = %d, want %d (after \"David\")", note.Anchor, want)
	}

	// The verse is exactly the verse: no title words, no title note.
	if len(vs) != 1 {
		t.Fatalf("got %d verses, want 1: %+v", len(vs), vs)
	}
	want := "LORD, how they have increased who trouble me!\nMany are they who rise up against me."
	if vs[0].Text != want {
		t.Errorf("verse 1:\n got  %q\n want %q", vs[0].Text, want)
	}
	if len(vs[0].Footnotes) != 0 {
		t.Errorf("verse 1 took the title's note: %+v", vs[0].Footnotes)
	}
}

// On the passages endpoint the title for chapter 4 is read while the decoder
// is still in chapter 3; it must land on 4.
func TestDecodeAPIBiblePassageAttachesATitleToTheChapterItOpens(t *testing.T) {
	byCh, _, supers, err := decodeAPIBiblePassage(passageOf(psalm3LastVersePara, psalm4TitlePara(""), psalm4FirstVersePara), "Psalms", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := supers[3]; has {
		t.Errorf("chapter 3 was given chapter 4's title: %+v", supers)
	}
	if supers[4].Text != psalm4Title {
		t.Errorf("chapter 4 title = %q, want %q", supers[4].Text, psalm4Title)
	}
	if got := byCh[3][0].Text; got != "Salvation belongs to the LORD." {
		t.Errorf("3:8 = %q", got)
	}
	if got := byCh[4][0].Text; !strings.HasPrefix(got, "Hear me when I call") {
		t.Errorf("4:1 = %q", got)
	}
}

// A title at the tail of a chunk, with no verse after it, is the NEXT
// chapter's — it is attached only when the provider's own verseId says which
// chapter, and never guessed onto the chapter just read.
func TestDecodeAPIBiblePassageNeverGuessesAChunkTailTitle(t *testing.T) {
	_, _, supers, err := decodeAPIBiblePassage(passageOf(psalm3LastVersePara, psalm4TitlePara("")), "Psalms", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(supers) != 0 {
		t.Errorf("an unattributable tail title was attached: %+v", supers)
	}

	_, _, supers, err = decodeAPIBiblePassage(passageOf(psalm3LastVersePara, psalm4TitlePara("PSA.4.1")), "Psalms", 3)
	if err != nil {
		t.Fatal(err)
	}
	if supers[4].Text != psalm4Title || len(supers) != 1 {
		t.Errorf("a verseId-stamped tail title should reach chapter 4: %+v", supers)
	}
}

// A title is not a verse: a chunk of nothing but a title is still "no verse
// text decoded", exactly as before titles were read.
func TestDecodeAPIBiblePassageTitleAloneIsNotAChapter(t *testing.T) {
	if _, _, _, err := decodeAPIBiblePassage(passageOf(psalm4TitlePara("PSA.4.1")), "Psalms", 4); err == nil {
		t.Error("a title with no verses decoded as a chapter")
	}
}
