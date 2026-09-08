package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

// networkSegments counts the two segment kinds that can reach the network, at
// any nesting depth. Paragraphs and lists carry their own children, so a
// shallow scan of the top-level slice would miss a link inside a bullet.
func networkSegments(segs []widget.RichTextSegment) (images, links int) {
	for _, seg := range segs {
		switch t := seg.(type) {
		case *widget.ImageSegment:
			images++
		case *widget.HyperlinkSegment:
			links++
		case *widget.ParagraphSegment:
			i, l := networkSegments(t.Texts)
			images += i
			links += l
		case *widget.ListSegment:
			i, l := networkSegments(t.Items)
			images += i
			links += l
		}
	}
	return images, links
}

func parsedNetworkSegments(t *testing.T, markdown string) (images, links int) {
	t.Helper()
	return networkSegments(widget.NewRichTextFromMarkdown(markdown).Segments)
}

// hostile is what a compromised or merely creative model could return. The
// image is the dangerous one: it is fetched at render time with no reader
// action, which is what makes it a tracking pixel rather than a broken picture.
const hostile = `Here is the answer.

![](https://tracker.example/pixel.png)

See [this commentary](https://tracker.example/click) for more.

- a bullet with [a nested link](https://tracker.example/nested)

[ref]: https://tracker.example/refdef
A reference-style [link][ref] and a reference [image][ref].
`

// TestTheAnswerMarkdownVectorIsReal is the control. If parsing the hostile
// answer UNSANITISED stops producing image and link segments — because Fyne
// changed, or the renderer stopped handling them — then the guard below would
// pass for the wrong reason and quietly protect nothing.
func TestTheAnswerMarkdownVectorIsReal(t *testing.T) {
	images, links := parsedNetworkSegments(t, hostile)
	if images == 0 {
		t.Fatal("expected the unsanitised answer to produce at least one image segment; " +
			"without that the sanitiser test proves nothing")
	}
	if links == 0 {
		t.Fatal("expected the unsanitised answer to produce at least one hyperlink segment; " +
			"without that the sanitiser test proves nothing")
	}
}

func TestASanitisedAnswerReachesNoNetwork(t *testing.T) {
	images, links := parsedNetworkSegments(t, sanitizeAIMarkdown(hostile))
	if images != 0 {
		t.Errorf("sanitised answer still produced %d image segment(s); each one is an "+
			"unattended fetch to a host the model chose", images)
	}
	if links != 0 {
		t.Errorf("sanitised answer still produced %d hyperlink segment(s)", links)
	}
}

func TestSanitisingKeepsTheReadableText(t *testing.T) {
	got := sanitizeAIMarkdown(hostile)
	for _, want := range []string{"Here is the answer.", "this commentary", "a nested link", "a bullet with"} {
		if !strings.Contains(got, want) {
			t.Errorf("sanitised answer dropped %q, which the reader was meant to see:\n%s", want, got)
		}
	}
	if strings.Contains(got, "tracker.example") {
		t.Errorf("sanitised answer still carries a model-chosen host:\n%s", got)
	}
}

// Study answers use emphasis, headings, lists and code. Stripping links must not
// cost the formatting that makes an answer readable.
func TestSanitisingLeavesOrdinaryFormattingAlone(t *testing.T) {
	const ordinary = "## Context\n\n**Paul** writes to the *church* in `Corinth`:\n\n- first\n- second\n\n> a quotation\n"
	if got := sanitizeAIMarkdown(ordinary); got != ordinary {
		t.Errorf("ordinary markdown was altered:\n got: %q\nwant: %q", got, ordinary)
	}
}
