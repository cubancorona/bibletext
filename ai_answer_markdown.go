package bibletext

import (
	"regexp"

	"fyne.io/fyne/v2/widget"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// An AI answer is rendered as markdown, and two markdown constructs reach the
// network. A link becomes a tappable HyperlinkSegment that opens the reader's
// browser; an image becomes an ImageSegment whose Visual() calls
// canvas.NewImageFromURI, and the http/https storage repositories registered by
// app.NewWithID fetch it AT RENDER TIME — an outbound request to a host the
// model chose, with no reader action at all.
//
// The prompts ask for plain prose, but a prompt is a request rather than a
// guarantee, and the answer arrives from a third party over the network.
//
// The guarantee here does NOT come from the stripping below. Pattern-matching
// markdown cannot be made sound: the grammar has too many ways to spell a link.
// Reference definitions can hide behind a blockquote or list marker, which a
// line-anchored pattern never sees; a label can contain an escaped bracket that
// a bracket-excluding class cannot cross; a destination can sit on the line
// after its "[ref]:"; and removing an inline link can SYNTHESISE a definition
// out of the text that closes up behind it.
//
// So the stripping is presentation only, and the safety comes from checking the
// result with the same parser that will render it: parse, walk the tree, and if
// any link or image node survives, do not render markdown at all — show the text
// literally, which cannot produce either segment. A benign answer keeps its
// formatting; anything else loses formatting rather than reaching the network.

// aiInlineLinkPattern matches an inline link or image — an optional leading "!",
// then bracketed text, then a parenthesised destination.
var aiInlineLinkPattern = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// aiLinkDefinitionPattern matches a link reference definition line, e.g.
// `[ref]: https://host/path "title"`. It catches the common shape only; the
// parser check below is what makes the rest safe.
var aiLinkDefinitionPattern = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\]]+\]:[ \t]*\S+.*(?:\r?\n|$)`)

// sanitizeAIMarkdown flattens the ordinary link and image spellings to the text
// a reader would have seen. It is best-effort by design: see the note above.
func sanitizeAIMarkdown(s string) string {
	s = aiLinkDefinitionPattern.ReplaceAllString(s, "")

	// Flattening an outer link can expose an inner one, so repeat until the text
	// stops changing. Bounded because every pass that changes the text removes at
	// least one bracket pair.
	for i := 0; i < 8; i++ {
		next := aiInlineLinkPattern.ReplaceAllString(s, "$1")
		if next == s {
			break
		}
		s = next
	}
	return s
}

// aiMarkdownReachesNetwork reports whether md, parsed as the answer widget parses
// it, still contains a node that would fetch or open a URL.
//
// goldmark.New() with no options is the same parser the widget's markdown
// renderer is built on, so this asks the question in the renderer's own terms
// rather than approximating it.
func aiMarkdownReachesNetwork(md string) bool {
	doc := goldmark.New().Parser().Parse(text.NewReader([]byte(md)))
	found := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Link, *ast.Image, *ast.AutoLink:
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found
}

// setAIAnswerText puts an AI answer into the panel's rich-text widget without
// ever giving it a segment that reaches the network.
func setAIAnswerText(answer *widget.RichText, raw string) {
	clean := sanitizeAIMarkdown(raw)
	if aiMarkdownReachesNetwork(clean) {
		// Literal text: a TextSegment has no URL and fetches nothing, so this is
		// safe whatever the model sent. Formatting is the price, and it is only
		// paid by an answer that tried to link in the first place.
		answer.Segments = []widget.RichTextSegment{
			&widget.TextSegment{Style: widget.RichTextStyleInline, Text: clean},
		}
		answer.Refresh()
		return
	}
	answer.ParseMarkdown(clean)
}
