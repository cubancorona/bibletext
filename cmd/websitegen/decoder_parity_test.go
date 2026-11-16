package main

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func publishedVersionForTest(t *testing.T, id string) webVersion {
	t.Helper()
	for _, version := range publishedVersions() {
		if version.ID == id {
			return version
		}
	}
	t.Fatalf("published version %q is missing", id)
	return webVersion{}
}

func TestWEBPagesRecoverPublisherParagraphs(t *testing.T) {
	// The fixture has no feed paragraph markers. The publisher table opens
	// a paragraph at Genesis 1:6, between these synthetic verse strings.
	body := []byte(`{"books":[{"id":"GEN","order":1,"chapters":[{"chapter":{"number":1,"content":[
		{"type":"verse","number":5,"content":["Sample opening clause"]},
		{"type":"verse","number":6,"content":["Sample paragraph opening"]},
		{"type":"verse","number":7,"content":["Sample continuation"]}
	]}}]}]}`)
	generic, err := bibletext.DecodeCanonical66(body)
	if err != nil {
		t.Fatal(err)
	}
	if generic.Verses["Genesis"][1][1].ParaStart {
		t.Fatal("control fixture already carries the missing paragraph break")
	}
	version := publishedVersionForTest(t, "web")
	bd, err := version.decode(body)
	if err != nil {
		t.Fatal(err)
	}
	verses := bd.Verses["Genesis"][1]
	if len(verses) != 3 || !verses[1].ParaStart || verses[2].ParaStart {
		t.Fatal("WEB site decoder did not restore the publisher's paragraph at verse 6")
	}
	got := chapterBody(bd, version.ID, "Genesis", 1, verses)
	if strings.Count(got, "<p>") != 2 || !strings.Contains(got, `</p><p><span class="v" id="v6">`) {
		t.Fatal("generated WEB chapter does not start its second paragraph at verse 6")
	}
}

func TestBSBPagesOmitLeakedReferenceHeadings(t *testing.T) {
	// The parenthesised reference belongs to the retained section heading;
	// the feed misclassifies it as a second heading at the same verse.
	body := []byte(`{"books":[{"id":"HEB","order":58,"chapters":[{"chapter":{"number":11,"content":[
		{"type":"heading","content":["Sample section"]},
		{"type":"heading","content":["(Joshua–Malachi)"]},
		{"type":"verse","number":30,"content":["Sample passage for decoder checks"]}
	]}}]}]}`)
	generic, err := bibletext.DecodeCanonical66(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(generic.Headings["Hebrews"][11]) != 2 {
		t.Fatal("control fixture did not retain both raw feed headings")
	}
	version := publishedVersionForTest(t, "bsb")
	bd, err := version.decode(body)
	if err != nil {
		t.Fatal(err)
	}
	headings := bd.Headings["Hebrews"][11]
	if len(headings) != 1 || headings[0].Text != "Sample section" {
		t.Fatal("BSB site decoder did not remove only the leaked reference heading")
	}
	got := chapterBody(bd, version.ID, "Hebrews", 11, bd.Verses["Hebrews"][11])
	if strings.Count(got, `<h2 class="sec">`) != 1 ||
		!strings.Contains(got, `<h2 class="sec">Sample section</h2>`) ||
		strings.Contains(got, "Joshua–Malachi") || !strings.Contains(got, `id="v30"`) {
		t.Fatal("generated BSB chapter lost its section or verse, or retained the leaked heading")
	}
}
