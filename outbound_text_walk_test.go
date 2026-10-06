package bibletext

// THE OUTBOUND CLEANER TAKES AWAY ONLY WHAT THE PAGE PUT IN.
//
// outboundText strips by shape: a "[digits]" token, the page's mark for an
// omitted verse, with the space after it; superscript digits; the no-break
// join; the em and en spaces of an indent. outbound_text.go rests that on a
// belief it states and used not to test: that no publisher's text contains
// any of them. Were one there, a share or a copy would lose it, and because
// every one of those rewrites changes the byte length, the offsets the share
// pipeline matches a selection on would shift as well. These walk real text
// for it. The embedded WEB Gospels, which every build carries, are walked in
// every run; every translation downloaded on the machine running the suite
// is walked through realCachePath, so CI, which has none, skips that half.
//
// Neither test quotes a verse when it fails: the NKJV is licensed, and its
// text stays out of logs as well as out of the tree. A failure names the
// passage and what the cleaner would change.

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// outboundRewrites returns, for every verse, heading and Psalm title in bd
// whose text outboundText would change, its reference and what would change,
// and how many texts were read.
func outboundRewrites(bd *BibleData) (read int, rewrites []string) {
	look := func(where, text string) {
		read++
		if out := outboundText(text); out != text {
			rewrites = append(rewrites, where+": "+outboundRewriteKinds(text))
		}
	}
	for _, book := range bd.Books {
		chapters := make([]int, 0, len(bd.Verses[book]))
		for ch := range bd.Verses[book] {
			chapters = append(chapters, ch)
		}
		sort.Ints(chapters)
		for _, ch := range chapters {
			for _, v := range bd.Verses[book][ch] {
				look(fmt.Sprintf("%s %d:%d", book, ch, v.Verse), v.Text)
			}
			for i, h := range bd.Headings[book][ch] {
				look(fmt.Sprintf("%s %d, heading %d", book, ch, i+1), h.Text)
			}
			if s, ok := bd.Superscriptions[book][ch]; ok {
				look(fmt.Sprintf("%s %d, title", book, ch), s.Text)
			}
		}
	}
	return read, rewrites
}

// outboundRewriteKinds names what in text the cleaner would take away,
// without quoting it.
func outboundRewriteKinds(text string) string {
	var kinds []string
	if stripVerseGapMarks(text) != text {
		kinds = append(kinds, "a bracketed number")
	}
	seen := map[string]bool{}
	for _, r := range text {
		var kind string
		switch {
		case superToDigit[r] != 0:
			kind = "a superscript digit"
		case r == noBreakSpace:
			kind = "a no-break space"
		case r == emSpace || r == enSpace:
			kind = "an em or en space"
		case smallCapitalToLetter[r] != 0:
			kind = "a small-capital letter"
		}
		if kind != "" && !seen[kind] {
			seen[kind] = true
			kinds = append(kinds, kind)
		}
	}
	return strings.Join(kinds, ", ")
}

func reportOutboundRewrites(t *testing.T, edition string, rewrites []string) {
	t.Helper()
	const shown = 20
	for i, r := range rewrites {
		if i == shown {
			t.Errorf("%s: and %d more", edition, len(rewrites)-shown)
			break
		}
		t.Errorf("%s %s: outboundText would change the publisher's text, losing it from a share "+
			"and moving the offsets a selection is matched on", edition, r)
	}
}

// The embedded WEB Gospels, every build's offline text: nothing in them is
// anything the cleaner takes away. CONTROL: the same walk over a copy with a
// "[12]" token planted in one verse and a superscript digit in another must
// report both, or it would pass by seeing nothing.
func TestTheEmbeddedGospelsPassThroughTheOutboundCleanerUnchanged(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("the embedded Gospels do not load: %v", err)
	}
	read, rewrites := outboundRewrites(bd)
	if read < 3700 {
		t.Fatalf("only %d texts read from the embedded Gospels; the walk is broken, so it proves nothing", read)
	}
	reportOutboundRewrites(t, "embedded WEB Gospels", rewrites)

	planted := &BibleData{Books: bd.Books, Verses: map[string]map[int][]Verse{}}
	for book, chs := range bd.Verses {
		planted.Verses[book] = map[int][]Verse{}
		for ch, vs := range chs {
			planted.Verses[book][ch] = append([]Verse(nil), vs...)
		}
	}
	john := planted.Verses["John"][3]
	if len(john) < 17 {
		t.Fatalf("control: the embedded Gospels have no John 3:16 to plant in (%d verses)", len(john))
	}
	john[15].Text = "For God so loved [12] the world"
	john[16].Text = "For God didn't send his Son¹ into the world"
	_, found := outboundRewrites(planted)
	want := []string{"John 3:16: a bracketed number", "John 3:17: a superscript digit"}
	if strings.Join(found, "|") != strings.Join(want, "|") {
		t.Errorf("control: the walk over planted text found %q, want %q", found, want)
	}
}

// Every translation downloaded on this machine, opted into by having it:
// the same walk, through realCachePath, never the suite's own cache. Skips
// where there is none, which is every CI runner.
func TestEveryDownloadedTranslationPassesThroughTheOutboundCleanerUnchanged(t *testing.T) {
	walked := 0
	for _, id := range []string{"web", "bsb", "webc", "nkjv"} {
		path := realCachePath(cachePathForVersion(id))
		if _, err := os.Stat(path); err != nil {
			t.Logf("%s: not downloaded on this machine, skipped", id)
			continue
		}
		bd, err := loadBibleFromCache(path)
		if err != nil {
			t.Errorf("%s: the downloaded copy does not load: %v", id, err)
			continue
		}
		walked++
		read, rewrites := outboundRewrites(bd)
		if read < 20000 {
			t.Errorf("%s: only %d texts read from a whole translation; the walk is broken", id, read)
		}
		reportOutboundRewrites(t, id, rewrites)
		t.Logf("%s: %d verses, headings and titles read", id, read)
	}
	if walked == 0 {
		t.Skip("no translation downloaded on this machine (CI never has one); open one in the app to run this walk")
	}
}
