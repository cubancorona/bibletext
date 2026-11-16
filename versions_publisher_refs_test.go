package bibletext

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestPublisherReferencesAreEnabledForRegisteredEditions(t *testing.T) {
	for _, id := range []string{"nkjv", "web", "bsb", "webc"} {
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("missing %s in registeredVersions", id)
		}
		if !v.PublisherCrossRefs {
			t.Errorf("%s: publisher references must be enabled in ordinary builds", id)
		}
	}
}

// This exercises the ordinary registry without the per-test display override,
// including older caches that predate machine-readable note targets.
func TestNKJVPublisherReferencesDisplayWithoutRegistryOverride(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	bd := xrefBible(map[string]map[int]int{"Genesis": {1: 1}, "Exodus": {1: 1}})
	v := &bd.Verses["Genesis"][1][0]
	v.Text = "A synthetic selected passage."
	bd.Verses["Exodus"][1][0].Text = "A synthetic reference preview."
	st := &AppState{Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1, CurrentVersion: "nkjv"}
	for _, tc := range []struct {
		name string
		refs []NoteRef
	}{
		{"tagged citation", []NoteRef{{ID: "EXO.1.1", Start: 0, End: len([]rune("Ex. 1:1"))}}},
		{"older cache without targets", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v.Footnotes = []Footnote{{Anchor: 2, Text: "Ex. 1:1", Kind: footnoteKindCrossref, Refs: tc.refs}}
			list := buildCrossRefList(st, []Verse{*v}, nil, nil, lightPalette, func(crossRef) {})
			if !list.Sections.Publisher || list.PublisherNotes != 1 || list.PublisherCards != 1 || !list.Passage {
				t.Fatalf("ordinary NKJV list = %+v; want its selected excerpt and publisher reference", list)
			}
			cards := cardsIn(list.Objects)
			if len(cards) != 1 || cardReference(cards[0]) != "Exodus 1:1" {
				t.Fatal("ordinary NKJV panel did not retain its reference target")
			}
			passage := passageIn(list.Objects)
			if passage == nil || passagePlainText(passage.runs) != v.Text {
				t.Fatal("ordinary NKJV panel changed its selected passage")
			}
			letter := false
			for _, run := range passage.runs {
				letter = letter || run.Mark && !run.Dim && run.Text == "a"
			}
			if !letter {
				t.Fatal("ordinary NKJV panel lost the publisher's anchor letter")
			}
		})
	}
}
