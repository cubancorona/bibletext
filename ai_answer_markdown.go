package bibletext

import (
	"regexp"
	"strings"
)

// An AI answer is rendered as markdown, and two markdown constructs reach the
// network. A link becomes a tappable HyperlinkSegment that opens the reader's
// browser; an image becomes an ImageSegment whose Visual() calls
// canvas.NewImageFromURI, and the http/https storage repositories registered by
// app.NewWithID fetch it AT RENDER TIME — an outbound request to a host the
// model chose, with no reader action at all.
//
// The prompts ask for plain prose, but a prompt is a request rather than a
// guarantee, and the answer arrives from a third party over the network. So the
// answer is stripped before it is parsed rather than trusted after.
//
// Stripping is deliberately narrow. Emphasis, headings, lists, blockquotes, and
// code all survive, because they are what a study answer actually uses; only the
// two link-shaped constructs are flattened to the text a reader would have seen.
// Raw HTML and autolinks need no handling: neither has a case in the markdown
// renderer's node switch, so both are already dropped before any URL is parsed.

// aiInlineLinkPattern matches an inline link or image — an optional leading "!",
// then bracketed text, then a parenthesised destination.
var aiInlineLinkPattern = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// aiLinkDefinitionPattern matches a link reference definition line, e.g.
// `[ref]: https://host/path "title"`. Removing the definitions is what
// disarms reference-style links: with no definition to resolve against,
// goldmark leaves `[text][ref]` as ordinary literal text.
var aiLinkDefinitionPattern = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\]]+\]:[ \t]*\S+.*(?:\r?\n|$)`)

// sanitizeAIMarkdown removes every markdown construct that could make the
// answer reach the network, leaving the reader's visible text in place.
func sanitizeAIMarkdown(s string) string {
	s = aiLinkDefinitionPattern.ReplaceAllString(s, "")

	// Flattening an outer link can expose an inner one, so repeat until the
	// text stops changing. The loop is bounded because every pass that changes
	// the text removes at least one bracket pair.
	for i := 0; i < 8; i++ {
		next := aiInlineLinkPattern.ReplaceAllString(s, "$1")
		if next == s {
			break
		}
		s = next
	}

	// Pathological nesting can outlast the loop. Rather than ship a half-stripped
	// answer, neutralise every remaining opening bracket so no construct can form
	// at all; goldmark renders the escape as a bare "[".
	if strings.Contains(s, "](") {
		s = strings.ReplaceAll(s, "[", `\[`)
	}
	return s
}
