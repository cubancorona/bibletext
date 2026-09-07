package bibletext

import (
	"strings"
	"testing"
)

// The divine name must reach EVERY surface, including the one-run path that
// most verses take. Each chapter builder has a shortcut for a verse the
// red-letter split leaves whole, and each of those shortcuts read the stored
// text rather than the run — so the small capitals were drawn only on the
// verses that happened to be split.
func TestSmallCapsReachTheOneRunPath(t *testing.T) {
	v := Verse{
		BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1,
		Text:      "The Lord is my shepherd.",
		SmallCaps: []TextSpan{{Start: 4, End: 8}},
	}
	// The control: this verse must really take the one-run path, or the test
	// would be proving something else.
	if runs := redLetterRuns("web", v, true); len(runs) != 1 {
		t.Fatalf("fixture yields %d runs, wanted the single-run path", len(runs))
	}
	drawn := smallCapsText(v)
	if !strings.Contains(drawn, "Lᴏʀᴅ") {
		t.Fatalf("smallCapsText did not set the divine name: %q", drawn)
	}
	// And the stored text is untouched, which is the whole point.
	if v.Text != "The Lord is my shepherd." {
		t.Errorf("the stored text was changed: %q", v.Text)
	}

	// The real builder, not just the helper: the Apple panes are fed this
	// exact string, and it is the shortcut inside it that was dropping the
	// substitution.
	st := sampleState()
	html := buildChapterHTML(st, []Verse{v})
	if !strings.Contains(html, "L\u1d0f\u0280\u1d05") {
		t.Errorf("the chapter HTML carries no small capitals for the divine name")
	}
	if strings.Contains(html, ">The Lord is") {
		t.Errorf("the chapter HTML still carries the unsubstituted name")
	}
}
