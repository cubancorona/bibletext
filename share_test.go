package bibletext

import (
	"os"
	"strings"
	"testing"
)

func shareTestBible() *BibleData {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{
		3: {
			{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world, that he gave his one and only Son."},
			{BookName: "John", Chapter: 3, Verse: 17, Text: "For God didn't send his Son into the world to judge the world."},
		},
	}
	return bd
}

func TestCitationForSelection(t *testing.T) {
	state := &AppState{Bible: shareTestBible(), CurrentBook: "John", CurrentChapter: 3}

	cases := []struct {
		name string
		sel  string
		want string
	}{
		{"single", "For God so loved the world, that he gave his one and only Son.", "John 3:16"},
		{"span", "For God so loved the world, that he gave his one and only Son. For God didn't send his Son into the world to judge the world.", "John 3:16-17"},
		{"unmatched", "a phrase not present anywhere here", "John 3"},
	}
	for _, c := range cases {
		if got := citationForSelection(state, c.sel); got != c.want {
			t.Errorf("%s: citationForSelection = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRenderVerseImage(t *testing.T) {
	path, err := renderVerseImage(&AppState{}, "For God so loved the world, that he gave his one and only Son.", "John 3:16", "WEB")
	if err != nil {
		t.Fatalf("renderVerseImage: %v", err)
	}
	defer os.Remove(path)
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("expected a .png path, got %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() < 1000 {
		t.Errorf("image suspiciously small: %d bytes", info.Size())
	}
}

func TestRenderVerseImageLongPassage(t *testing.T) {
	long := strings.Repeat("For God so loved the world that he gave his one and only Son. ", 12)
	path, err := renderVerseImage(&AppState{}, long, "John 3:16-18", "WEB")
	if err != nil {
		t.Fatalf("long render: %v", err)
	}
	os.Remove(path)
}
