//go:build !next

package main

// THE GREEK ESTHER ON THE SHIPPING SITE. The shipping versification table
// records WEB Catholic's Greek Esther as incommensurable with the Hebrew
// Esther, so no link carries a verse between them: the notice pages say the
// book does not correspond, and the version switcher carries the note alone.
// The next major release maps it verse for verse (greek_esther_next_test.go,
// docs/NEXT.md); this file is deleted on the day that release ships.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// greekEstherCarryCases are the switcher's Greek Esther cases: no verse
// corresponds, and the note still travels.
func greekEstherCarryCases(t *testing.T, root string) []carryCase {
	var cs []carryCase
	cs = append(cs, casesOn(t, root, "webc/esther/1/index.html", "web",
		[3]string{"3", "NOTE", "../../../web/esther/1/#n=NOTE"},
		[3]string{"3", "", "../../../web/esther/1/"},
	)...)
	cs = append(cs, casesOn(t, root, "web/esther/1/index.html", "webc",
		[3]string{"3", "", "../../../webc/esther/1/"},
	)...)
	cs = append(cs, casesOn(t, root, "webc/esther/4/index.html", "web",
		[3]string{"3", "", "../../../web/esther/4/"},
		[3]string{"20", "NOTE", "../../../web/esther/4/#n=NOTE"},
	)...)
	cs = append(cs, casesOn(t, root, "web/esther/4/index.html", "webc",
		[3]string{"5", "", "../../../webc/esther/4/"},
	)...)
	return cs
}

// The NKJV's Esther pages say WEB Catholic's Esther is a different text whose
// verses do not correspond, in every chapter.
func TestTheGreekEstherCaveatSaysItDoesNotCorrespond(t *testing.T) {
	site, _ := noticeFixture(t)
	for _, path := range []string{"nkjv/esther/1", "nkjv/esther/4"} {
		b, err := os.ReadFile(filepath.Join(site.root, filepath.FromSlash(path), "index.html"))
		if err != nil {
			t.Fatalf("read /%s/: %v", path, err)
		}
		if !strings.Contains(string(b), "different text whose verses don&#39;t correspond") {
			t.Errorf("/%s/: WEBC's Esther is a different book, not a renumbering; the page does not say so", path)
		}
	}
}

// The sentence the shipping site prints for a book that does not correspond,
// word for word: its Esther pages print it.
func TestTheIncommensurableCaveatIsTheShippingOne(t *testing.T) {
	n := noticeSpec{Book: "Esther", Chapter: 1, Scope: scopeChapter}
	const want = "Esther in the World English Bible (Catholic) is a different text whose verses don't " +
		"correspond to these, so that link opens the chapter."
	if got := n.caveat(bibletext.NumberingIncommensurable, []string{"World English Bible (Catholic)"}); got != want {
		t.Errorf("caveat:\n got %q\nwant %q", got, want)
	}
}
