package main

import (
	"encoding/json"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// The WEB Catholic's Greek Daniel 3 sets a heading with a translators' note
// on it, which the helloao feed runs into the heading's words. The page sets
// it as the app does (heading_note.go): the title as the heading inside the
// article, the note in the chapter's notes under 24, the reference the
// publisher gives it. The fixture is the feed's own nodes for 3:23-24.
func TestTheSongOfTheThreeIsSetAsItsPublisherSetsItOnThePage(t *testing.T) {
	const (
		title = "THE SONG OF THE THREE HOLY CHILDREN"
		note  = "The Song of the Three Holy Children is an addition to Daniel found in the Greek Septuagint " +
			"but not found in the traditional Hebrew text of Daniel. This portion is recognized as Deuterocanonical " +
			"Scripture by the Roman Catholic, Greek Orthodox, and Russian Orthodox Churches. It is found inserted " +
			"between Daniel 3:23 and Daniel 3:24 of the traditional Hebrew Bible. Here, the verses after 23 from the " +
			"Hebrew Bible are numbered starting at 91 to make room for these verses."
	)
	content := []any{
		map[string]any{"type": "verse", "number": 23, "content": []string{
			"These three men, Shadrach, Meshach, and Abednego, fell down bound into the middle of the burning fiery furnace."}},
		map[string]any{"type": "heading", "content": []string{title + "3:24 " + note}},
		map[string]any{"type": "verse", "number": 24, "content": []string{
			"They walked in the midst of the fire, praising God, and blessing the Lord."}},
	}
	doc := map[string]any{
		"translation": map[string]any{"shortName": "WEBC"},
		"books": []any{map[string]any{"id": "DAG", "order": 27, "chapters": []any{
			map[string]any{"chapter": map[string]any{"number": 3, "content": content}},
		}}},
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	bd, err := bibletext.DecodeHelloAOCatholic(body)
	if err != nil {
		t.Fatal(err)
	}
	verses := bd.Verses["Daniel"][3]
	if len(verses) != 2 {
		t.Fatalf("fixture: decoded %d verses of Daniel 3, want 2", len(verses))
	}

	page := chapterBody(bd, "webc", "Daniel", 3, verses)
	if !strings.Contains(page, `<h2 class="sec">`+title+`</h2>`) {
		t.Errorf("the page does not set the title alone as the heading:\n%s", page)
	}
	if strings.Contains(page, "Septuagint") {
		t.Errorf("the heading's note reached the article:\n%s", page)
	}
	notes := chapterNotes(bd, "Daniel", 3, verses)
	if !strings.Contains(notes, `<dt>24</dt><dd>`+note+`</dd>`) {
		t.Errorf("the chapter's notes do not carry the heading's note under 24:\n%s", notes)
	}
}
