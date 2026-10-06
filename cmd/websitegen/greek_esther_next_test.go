//go:build next

package main

// THE GREEK ESTHER ON THE NEXT MAJOR RELEASE'S SITE (docs/NEXT.md). WEB
// Catholic's Greek Esther keeps the Hebrew book's verse numbers, so the next
// release's versification table maps it verse for verse: a chapter it adds
// nothing to and lacks nothing of carries the verse across like any other, and
// a chapter where it lacks a verse opens the chapter and says a verse is
// missing. The shipping site's behaviour is pinned in
// greek_esther_current_test.go.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// greekEstherCarryCases are the switcher's Greek Esther cases: a verse both
// number alike travels as it is, and one only one of them has opens the
// chapter, the note still travelling.
func greekEstherCarryCases(t *testing.T, root string) []carryCase {
	var cs []carryCase
	cs = append(cs, casesOn(t, root, "webc/esther/1/index.html", "web",
		[3]string{"3", "NOTE", "../../../web/esther/1/#v3&n=NOTE"},
		[3]string{"3", "", "../../../web/esther/1/#v3"},
	)...)
	cs = append(cs, casesOn(t, root, "web/esther/1/index.html", "webc",
		[3]string{"3", "", "../../../webc/esther/1/#v3"},
	)...)
	// Esther 4: the prayers, 4:18-47, are the Greek Esther's alone, and it
	// has nothing at 4:6.
	cs = append(cs, casesOn(t, root, "webc/esther/4/index.html", "web",
		[3]string{"3", "", "../../../web/esther/4/#v3"},
		[3]string{"20", "", "../../../web/esther/4/"},
		[3]string{"20", "NOTE", "../../../web/esther/4/#n=NOTE"},
	)...)
	cs = append(cs, casesOn(t, root, "web/esther/4/index.html", "webc",
		[3]string{"5", "", "../../../webc/esther/4/#v5"},
		[3]string{"6", "", "../../../webc/esther/4/"},
		[3]string{"7", "", "../../../webc/esther/4/#v7"},
	)...)
	return cs
}

// The NKJV's Esther 4 says WEB Catholic does not carry every verse: the Greek
// Esther has nothing at 4:6.
func TestTheGreekEstherCaveatSaysAVerseIsMissing(t *testing.T) {
	site, _ := noticeFixture(t)
	b, err := os.ReadFile(filepath.Join(site.root, "nkjv", "esther", "4", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "The World English Bible (Catholic) doesn&#39;t carry every verse") {
		t.Error("the Greek Esther has nothing at 4:6; the page does not say a verse is missing")
	}
}

// The Greek Esther keeps the Hebrew book's verse numbers, so a chapter it adds
// nothing to and lacks nothing of carries the verse across like any other, and
// a chapter where it lacks a verse opens the chapter and says why.
func TestTheGreekEstherLinkCarriesTheVerseWhereTheNumbersAgree(t *testing.T) {
	site, _ := noticeFixture(t)
	read := func(path string) string {
		b, err := os.ReadFile(filepath.Join(site.root, filepath.FromSlash(path), "index.html"))
		if err != nil {
			t.Fatalf("read /%s/: %v", path, err)
		}
		return string(b)
	}
	link := func(page, href string) string {
		m := regexp.MustCompile(`<a([^>]*)href="` + regexp.QuoteMeta(href) + `"`).FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no link to %s on the page", href)
		}
		return m[1]
	}
	one := read("nkjv/esther/1")
	if !strings.Contains(link(one, "../../../webc/esther/1/"), `data-frag="verse"`) {
		t.Error("Esther 1 maps verse for verse into the Greek Esther; the link should carry the verse")
	}
	if strings.Contains(one, `class="vnote"`) {
		t.Error("Esther 1 needs no versification caveat, but the page has one")
	}
	four := read("nkjv/esther/4")
	if !strings.Contains(link(four, "../../../webc/esther/4/"), `data-frag="note"`) {
		t.Error("the Greek Esther has nothing at 4:6; the link must open the chapter and keep the note")
	}
	// CONTROL: the WEB's Esther 4 is the NKJV's, verse for verse.
	if !strings.Contains(link(four, "../../../web/esther/4/"), `data-frag="verse"`) {
		t.Error("Esther 4 maps verse for verse into the WEB; the link should carry the verse")
	}
}

// No published translation's book fails to correspond in the next release, so
// no page prints the incommensurable caveat; it is a draft awaiting approval
// (docs/NEXT.md, decision C), pinned here word for word so that its wording
// changes only on purpose. Reader-facing the day a book needs it.
func TestTheIncommensurableCaveatIsPinned(t *testing.T) {
	if incommensurableCaveat != "%s %s the whole of %s differently, so %s %s %s rather than the verse." {
		t.Errorf("incommensurableCaveat changed: %q", incommensurableCaveat)
	}
	n := noticeSpec{Book: "Esther", Chapter: 4, Scope: scopeChapter}
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"World English Bible (Catholic)"}, "The World English Bible (Catholic) numbers the whole of Esther " +
			"differently, so that link opens Esther 4 rather than the verse."},
		{[]string{"World English Bible", "Berean Standard Bible"}, "The World English Bible and the Berean Standard " +
			"Bible number the whole of Esther differently, so those links open Esther 4 rather than the verse."},
	} {
		if got := n.caveat(bibletext.NumberingIncommensurable, tc.names); got != tc.want {
			t.Errorf("caveat for %v:\n got %q\nwant %q", tc.names, got, tc.want)
		}
	}
}
