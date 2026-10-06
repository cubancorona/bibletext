package main

// THE SITE WITHOUT THE NKJV'S TEXT, PINNED FILE BY FILE.
//
// The generator can publish the New King James Version's text or only the
// notice pages that name its passages, and the second of those is the site as
// it has been published since the notice pages landed. Whatever is done to the
// first must leave the second exactly as it is: a reader of /web/john/3/ must
// not be served a different file because the licensed path learned something.
//
// So the whole tree a fixture builds is pinned here by the digest of every
// file, with its path. It is a fixture, not the real canon — the real tree is
// held to the live site by scripts/publish-site.sh's drift report — but it runs
// every renderer the switch reaches: the three published trees, their canon
// gaps, the notice tree, the shared assets and the 404.
//
// WHEN -update IS LEGITIMATE. Only for a change that is meant to alter the
// published site in both states — a new rule in the reader's stylesheet, a
// fix to the chapter renderer — and then the drift report of the next publish
// says the same thing. A change to the licensed path alone must never need it:
//
//	go test ./cmd/websitegen -run TestNKJVTextOffSiteIsByteIdenticalToTheBase -update
//
// TWO PINNED TREES, ONE PER STATE OF THE NEXT SWITCH (docs/NEXT.md). The next
// major release changes what the generator writes, so its tree is pinned in a
// file of its own (offGoldenPath), rewritten by the same command with
// -tags next. The shipping tree's file is never touched by a next-release
// change, and TestTheNextReleaseSiteDiffersOnlyWhereItsPiecesSay holds the
// two to the pages those pieces are meant to change.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

var updateGolden = flag.Bool("update", false, "rewrite this state's pinned digests (offGoldenPath) from this build")

// offGoldenPath is the digests pinned for the state of the next switch this
// build is in: the shipping site's, or the next major release's.
var offGoldenPath = goldenPathFor(bibletext.NextRelease)

func goldenPathFor(next bool) string {
	if next {
		return "testdata/nkjv_off_site_next.sha256"
	}
	return "testdata/nkjv_off_site.sha256"
}

// nextReleaseSitePages are the fixture's pages the next major release writes
// differently from the shipping build, each for the piece behind the switch
// that changes it (docs/NEXT.md).
var nextReleaseSitePages = map[string]string{
	// The Greek Esther maps verse for verse: the NKJV's notice page needs no
	// caveat for Esther 1, and the switcher carries the verse between WEB
	// Catholic's Esther and the WEB's and the BSB's.
	"bsb/esther/1/index.html":  "the Greek Esther",
	"nkjv/esther/1/index.html": "the Greek Esther",
	"web/esther/1/index.html":  "the Greek Esther",
	"webc/esther/1/index.html": "the Greek Esther",
}

// goldenFixtureVersions is the canon the golden site is built from: the three
// published editions over a handful of books, with the Catholic edition's
// extra book and chapters so the canon-gap pages are written, and the
// features a chapter page draws — a psalm's title, a publisher's heading,
// poem lines, supplied words, small capitals and a translators' note — so the
// digests cover every branch of the chapter renderer.
func goldenFixtureVersions() []loadedVersion {
	verse := func(book string, ch, n int, text string) bibletext.Verse {
		return bibletext.Verse{BookName: book, Book: book, Chapter: ch, Verse: n, Text: text}
	}
	chapter := func(book string, ch, n int) []bibletext.Verse {
		out := make([]bibletext.Verse, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, verse(book, ch, i, fmt.Sprintf("Golden fixture verse %d of %s %d.", i, book, ch)))
		}
		return out
	}
	build := func(catholic bool) *bibletext.BibleData {
		psalm := []bibletext.Verse{
			verse("Psalms", 3, 1, "First fixture line of the psalm,\nanswered by its second line."),
			verse("Psalms", 3, 2, "A third fixture line;\nand the Lord answers the fourth."),
		}
		lord := len([]rune(psalm[1].Text[:strings.Index(psalm[1].Text, "Lord")]))
		psalm[1].SmallCaps = []bibletext.TextSpan{{Start: lord, End: lord + 4}}
		john := chapter("John", 3, 18)
		john[15].Supplied = []bibletext.TextSpan{{Start: 0, End: 6}}
		john[15].Footnotes = []bibletext.Footnote{{Anchor: 6, Text: "A fixture translators' note."}}
		john[0].ParaStart = true
		john[8].ParaStart = true
		bd := &bibletext.BibleData{
			Books: []string{"Psalms", "John", "Acts", "Romans", "Daniel", "Esther"},
			Verses: map[string]map[int][]bibletext.Verse{
				"Psalms": {3: psalm},
				"John":   {1: chapter("John", 1, 5), 3: john},
				"Acts":   {8: chapter("Acts", 8, 40)},
				"Romans": {14: chapter("Romans", 14, 26), 16: chapter("Romans", 16, 24)},
				"Daniel": {12: chapter("Daniel", 12, 13)},
				"Esther": {1: chapter("Esther", 1, 22)},
			},
			Superscriptions: map[string]map[int]bibletext.Superscription{
				"Psalms": {3: {Text: "A fixture title for the psalm."}},
			},
			Headings: map[string]map[int][]bibletext.Heading{
				"John": {3: {{Text: "A Fixture Heading", Style: "heading", BeforeVerse: 9}}},
			},
		}
		if catholic {
			bd.Books = append(bd.Books, "Tobit")
			bd.Verses["Romans"] = map[int][]bibletext.Verse{
				14: chapter("Romans", 14, 23), 16: chapter("Romans", 16, 27),
			}
			bd.Verses["Daniel"] = map[int][]bibletext.Verse{
				12: chapter("Daniel", 12, 13), 13: chapter("Daniel", 13, 64), 14: chapter("Daniel", 14, 42),
			}
			bd.Verses["Tobit"] = map[int][]bibletext.Verse{1: chapter("Tobit", 1, 22)}
		}
		return bd
	}
	var loaded []loadedVersion
	for _, pv := range publishedVersions() {
		loaded = append(loaded, loadedVersion{webVersion: pv, bible: build(pv.ID == "webc")})
	}
	return loaded
}

// siteDigests is "<digest>  <path>" for every file under root, sorted by path —
// the shape sha256sum writes, so a mismatch can be checked by hand.
func siteDigests(t *testing.T, root string) []string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		lines = append(lines, hex.EncodeToString(sum[:])+"  "+filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i][66:] < lines[j][66:] })
	return lines
}

func readGolden(t *testing.T) []string {
	t.Helper()
	return readGoldenAt(t, offGoldenPath)
}

func readGoldenAt(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with -update only if the change is meant to alter the site)", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

// TestNKJVTextOffSiteIsByteIdenticalToTheBase builds the golden fixture with
// the NKJV's text off and compares every file with the pinned digests.
func TestNKJVTextOffSiteIsByteIdenticalToTheBase(t *testing.T) {
	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, goldenFixtureVersions(), noticedVersionsFor(false)); err != nil {
		t.Fatalf("writeSite: %v", err)
	}
	got := siteDigests(t, site.root)
	if *updateGolden {
		if err := os.WriteFile(offGoldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d digests to %s", len(got), offGoldenPath)
		return
	}
	want := readGolden(t)
	byPath := func(lines []string) map[string]string {
		m := map[string]string{}
		for _, l := range lines {
			m[l[66:]] = l[:64]
		}
		return m
	}
	g, w := byPath(got), byPath(want)
	for path, sum := range w {
		switch other, ok := g[path]; {
		case !ok:
			t.Errorf("%s is no longer written", path)
		case other != sum:
			t.Errorf("%s changed", path)
		}
	}
	for path := range g {
		if _, ok := w[path]; !ok {
			t.Errorf("%s is written and was not before", path)
		}
	}
	// A control on the comparison itself: the fixture must reach every kind of
	// page, or a match proves less than it appears to.
	for _, must := range []string{"web/john/3/index.html", "web/tobit/1/index.html", "nkjv/john/3/index.html",
		"nkjv/index.html", "404.html", "webc/daniel/14/index.html"} {
		if _, ok := g[must]; !ok {
			t.Errorf("the fixture wrote no %s; the golden would not cover it", must)
		}
	}
}

// THE NEXT RELEASE'S TREE DIFFERS FROM THE SHIPPING ONE ONLY WHERE ITS PIECES
// SAY. Read in both states of the switch, from the two pinned files: a page
// the next release writes differently that no piece names, or a named page it
// writes the same, fails here, so a change meant for one state cannot quietly
// rewrite the other's pages.
func TestTheNextReleaseSiteDiffersOnlyWhereItsPiecesSay(t *testing.T) {
	byPath := func(lines []string) map[string]string {
		m := map[string]string{}
		for _, l := range lines {
			m[l[66:]] = l[:64]
		}
		return m
	}
	current, next := byPath(readGoldenAt(t, goldenPathFor(false))), byPath(readGoldenAt(t, goldenPathFor(true)))
	differ := map[string]bool{}
	for path, sum := range current {
		if next[path] != sum {
			differ[path] = true
		}
	}
	for path := range next {
		if _, ok := current[path]; !ok {
			differ[path] = true
		}
	}
	for path := range differ {
		if _, ok := nextReleaseSitePages[path]; !ok {
			t.Errorf("%s differs in the next release's tree, and no piece behind the switch names it", path)
		}
	}
	for path, piece := range nextReleaseSitePages {
		if !differ[path] {
			t.Errorf("%s is named for %s but is the same in both trees", path, piece)
		}
	}
}
