package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	bibletext "github.com/cubancorona/bibletext"
)

// A LINK PREVIEW SAYS WHAT THE APP'S SHARE SAYS. The og:description under a
// shared chapter is plain text a messenger draws, and the app sends a verse
// with the divine name in the small capitals the page draws (sharedText); the
// preview joined the stored text, so 437 of the NKJV's chapters previewed the
// name as an ordinary "Lord". The control is the same verse with nothing
// marked, which must come out exactly as stored.
func TestChapterPreviewWritesTheDivineNameAsTheAppSharesIt(t *testing.T) {
	text := "In the beginning of the fixture the Lord spoke."
	at := utf8.RuneCountInString(text[:strings.Index(text, "Lord")])
	marked := []bibletext.Verse{{Verse: 1, Text: text, SmallCaps: []bibletext.TextSpan{{Start: at, End: at + 4}}}}
	if got, want := chapterPreview(marked), "In the beginning of the fixture the Lᴏʀᴅ spoke."; got != want {
		t.Errorf("preview = %q, want %q", got, want)
	}
	if got, want := chapterPreview(marked), bibletext.VerseSharedText(marked[0]); got != want {
		t.Errorf("preview = %q, the app's share = %q", got, want)
	}
	plain := []bibletext.Verse{{Verse: 1, Text: text}}
	if got := chapterPreview(plain); got != text {
		t.Errorf("an unmarked verse previews as %q, want it unchanged", got)
	}
}

// The preview is cut near 200 bytes. With no space to cut at, the cut backs
// off to a whole character: small capitals are two and three bytes, and a cut
// through one would put invalid UTF-8 in every messenger's preview.
func TestChapterPreviewNeverSplitsACharacter(t *testing.T) {
	long := strings.Repeat("Lord", 80)
	var caps []bibletext.TextSpan
	for i := 0; i < 80; i++ {
		caps = append(caps, bibletext.TextSpan{Start: 4 * i, End: 4*i + 4})
	}
	for lead := 0; lead < 3; lead++ {
		v := bibletext.Verse{Verse: 1, Text: strings.Repeat("x", lead) + long}
		for _, sp := range caps {
			v.SmallCaps = append(v.SmallCaps, bibletext.TextSpan{Start: sp.Start + lead, End: sp.End + lead})
		}
		got := chapterPreview([]bibletext.Verse{v})
		if !utf8.ValidString(got) {
			t.Errorf("lead %d: the preview is not valid UTF-8", lead)
		}
		if !strings.HasSuffix(got, "…") || len(got) > 200+len("…") {
			t.Errorf("lead %d: the preview is %d bytes and not cut", lead, len(got))
		}
	}
}
