package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/widget"
)

// networkSegments counts the two segment kinds that can reach the network, at
// any nesting depth. Paragraphs and lists carry their own children, so a shallow
// scan of the top-level slice would miss a link inside a bullet.
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

// renderedNetworkSegments drives the real path the panel uses.
func renderedNetworkSegments(text string) (images, links int) {
	w := widget.NewRichTextFromMarkdown("")
	setAIAnswerText(w, text)
	return networkSegments(w.Segments)
}

// hostile is the plain shape: an image that is fetched at render time with no
// reader action, and a link that becomes tappable.
const hostile = "Here is the answer.\n\n" +
	"![](https://tracker.example/pixel.png)\n\n" +
	"See [this commentary](https://tracker.example/click) for more.\n\n" +
	"- a bullet with [a nested link](https://tracker.example/nested)\n"

// bypasses are inputs that defeat pattern-based stripping. Each was derived by
// reading goldmark's parser rather than guessed: reference definitions hidden
// behind a blockquote or list marker, labels containing an escaped bracket,
// destinations on the line after "[ref]:", odd-length backslash runs, and the
// case where removing an inline link synthesises a definition behind it.
//
// They are the reason the guarantee comes from parsing the result rather than
// from the stripping.
var bypasses = []string{
	"\\[a\\]b](https://evil.example/click)",
	"![](x)[ref]: https://evil.example/pixel.png\n\n![ref]\n",
	"> [ref]: https://evil.example/pixel.png\n\n![ref]\n",
	"[Foo*bar\\]]: https://evil.example/pixel.png\n\n![Foo*bar\\]]\n",
	"[ref]:\nhttps://evil.example/pixel.png\n\n![ref]\n",
	"- [ref]: https://evil.example/pixel.png\n\n![ref]\n",
	"[a]:\nhttps://evil.example/x\n\nSee [a] now.\n",
	"[a]:\nhttps://evil.example/x\n\n![a]\n",
	"[a]:   \nhttps://evil.example/x\n\n[a]\n",
	"[a]:\r\nhttps://evil.example/x\r\n\r\n[a]\r\n",
	"[x\\]y]: https://evil.example/x\n\nread [more][x\\]y] now\n",
	"[q\\]]: https://evil.example/x\n\n![picture][q\\]]\n",
	"> [a]: https://evil.example/x\n\n[a]\n",
	"> [a]: https://evil.example/x\n\n![alt][a]\n",
	"- [a]: https://evil.example/x\n\n[a]\n",
	"[a]:\fhttps://evil.example/x\n\n[a]\n",
	"\\[a\\]b](https://evil.example)",
	"Genesis 1:1 is discussed further here: \\[Verify your account\\]now](https://evil.example/phish)\n\nHope that helps.",
	"[ref]:\nhttps://evil.example/pixel.png\n\n![alt][ref]\n",
	"[a\\]b]: https://evil.example/pixel.png\n\n![alt][a\\]b]\n",
	"> [ref]: https://evil.example/pixel.png\n\n![alt][ref]\n",
	"- [ref]: https://evil.example/track\n\n[click here][ref]\n",
	"[note]:\nhttps://tracker.example.com/beacon.png\n\n![note]",
	"[foo]:\nhttps://evil.example.com/a\n\n[foo]",
	"[foo]:\nhttps://evil.example.com/a\n[foo]",
	"[foo]:\r\nhttps://evil.example.com/a\r\n\r\n[foo]",
	"[foo]:\n   https://evil.example.com/a\n\n[foo][]",
}

// TestTheAnswerMarkdownVectorIsReal is the control. If parsing these inputs
// UNSANITISED stops producing image and link segments — because Fyne or goldmark
// changed — then the guards below would pass for the wrong reason and protect
// nothing.
func TestTheAnswerMarkdownVectorIsReal(t *testing.T) {
	images, links := networkSegments(widget.NewRichTextFromMarkdown(hostile).Segments)
	if images == 0 || links == 0 {
		t.Fatalf("expected the unsanitised answer to produce image and link segments, got %d and %d; "+
			"without that the guard proves nothing", images, links)
	}
	live := 0
	for _, b := range bypasses {
		if i, l := networkSegments(widget.NewRichTextFromMarkdown(b).Segments); i+l > 0 {
			live++
		}
	}
	if live == 0 {
		t.Fatal("no bypass input produced a network segment unsanitised; the corpus has gone stale " +
			"and TestNoBypassReachesTheNetwork would pass vacuously")
	}
	t.Logf("%d of %d bypass inputs reach the network unsanitised", live, len(bypasses))
}

func TestASanitisedAnswerReachesNoNetwork(t *testing.T) {
	if images, links := renderedNetworkSegments(hostile); images != 0 || links != 0 {
		t.Errorf("rendered answer produced %d image and %d hyperlink segment(s)", images, links)
	}
}

// The corpus is the point: pattern-stripping alone fails every one of these.
func TestNoBypassReachesTheNetwork(t *testing.T) {
	for _, b := range bypasses {
		images, links := renderedNetworkSegments(b)
		if images != 0 || links != 0 {
			t.Errorf("bypass reached the network (%d image, %d link): %q", images, links, b)
		}
	}
}

// TestStrippingAloneWouldNotBeEnough pins why the parser check exists. Every one
// of these inputs walks straight through the pattern stripping; if a later change
// drops the check as redundant, this fails and says what it is protecting.
func TestStrippingAloneWouldNotBeEnough(t *testing.T) {
	survived := 0
	for _, b := range bypasses {
		if i, l := networkSegments(widget.NewRichTextFromMarkdown(sanitizeAIMarkdown(b)).Segments); i+l > 0 {
			survived++
		}
	}
	if survived == 0 {
		t.Fatal("pattern stripping alone now neutralises every bypass, which it cannot soundly do; " +
			"the corpus has gone stale — regenerate it before trusting the stripping")
	}
	t.Logf("%d of %d bypasses defeat the stripping and are stopped only by the parser check",
		survived, len(bypasses))
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
// cost the formatting that makes an answer readable, and an ordinary answer must
// still take the markdown path rather than the literal-text fallback.
func TestOrdinaryFormattingSurvives(t *testing.T) {
	const ordinary = "## Context\n\n**Paul** writes to the *church* in `Corinth`:\n\n- first\n- second\n\n> a quotation\n"
	if got := sanitizeAIMarkdown(ordinary); got != ordinary {
		t.Errorf("ordinary markdown was altered:\n got: %q\nwant: %q", got, ordinary)
	}
	if aiMarkdownReachesNetwork(ordinary) {
		t.Fatal("ordinary markdown was judged to reach the network, so it would render as flat text")
	}
	w := widget.NewRichTextFromMarkdown("")
	setAIAnswerText(w, ordinary)
	var headings int
	for _, seg := range w.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok && ts.Style.Inline == false && ts.Text == "Context" {
			headings++
		}
	}
	if headings == 0 {
		t.Error("the heading did not survive; the answer fell back to literal text")
	}
}
