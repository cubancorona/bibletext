package bibletext

import (
	"regexp"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2/widget"
	"github.com/rivo/uniseg"
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
	var segments []widget.RichTextSegment
	if aiMarkdownReachesNetwork(clean) {
		// Literal text: a TextSegment has no URL and fetches nothing, so this is
		// safe whatever the model sent. Formatting is the price, and it is only
		// paid by an answer that tried to link in the first place.
		segments = []widget.RichTextSegment{
			&widget.TextSegment{Style: widget.RichTextStyleInline, Text: clean},
		}
	} else {
		// Parse without refreshing the sized answer first. Refreshing one long
		// paragraph makes Fyne shape the entire remaining text for every row.
		segments = widget.NewRichTextFromMarkdown(clean).Segments
	}
	answer.Segments = aiAnswerLayoutSegments(segments)
	answer.Refresh()
}

const aiAnswerSegmentRunes = 512

// aiAnswerLayoutSegments bounds the text Fyne remeasures for each wrapped row.
// Intermediate pieces stay inline, so splitting adds no paragraph or list break;
// the final piece retains the original block ending. The answer used by Copy,
// Report and the request cache is unchanged.
func aiAnswerLayoutSegments(segments []widget.RichTextSegment) []widget.RichTextSegment {
	var out []widget.RichTextSegment
	for _, segment := range segments {
		switch s := segment.(type) {
		case *widget.TextSegment:
			// Fyne recognizes quote indentation by exact style equality. Making
			// an intermediate quote inline would lose it, so keep quotes intact.
			if s.Style == widget.RichTextStyleBlockquote || utf8.RuneCountInString(s.Text) <= aiAnswerSegmentRunes {
				out = append(out, s)
				continue
			}
			pieces := aiAnswerTextPieces(s.Text)
			for i, piece := range pieces {
				style := s.Style
				if i < len(pieces)-1 {
					style.Inline = true
				}
				out = append(out, &widget.TextSegment{Text: piece, Style: style})
			}
		case *widget.ParagraphSegment:
			s.Texts = aiAnswerLayoutSegments(s.Texts)
			out = append(out, s)
		case *widget.ListSegment:
			for i, item := range s.Items {
				pieces := aiAnswerLayoutSegments([]widget.RichTextSegment{item})
				if len(pieces) == 1 {
					s.Items[i] = pieces[0]
				} else {
					// One Items entry owns one bullet. A plain text item may need
					// several layout pieces, but it is still one list item.
					s.Items[i] = &widget.ParagraphSegment{Texts: pieces}
				}
			}
			out = append(out, s)
		default:
			out = append(out, segment)
		}
	}
	return out
}

// aiAnswerTextPieces prefers whitespace boundaries and keeps grapheme clusters
// whole when a word has to span pieces. One unusually large grapheme may exceed
// the budget: preserving a combining sequence is preferable to breaking it.
func aiAnswerTextPieces(text string) []string {
	type boundary struct {
		byteEnd, runeEnd int
		space            bool
	}
	ends := []boundary{{}}
	graphemes := uniseg.NewGraphemes(text)
	runes := 0
	for graphemes.Next() {
		_, end := graphemes.Positions()
		cluster := graphemes.Runes()
		runes += len(cluster)
		ends = append(ends, boundary{end, runes, unicode.IsSpace(cluster[len(cluster)-1])})
	}
	var pieces []string
	for start := 0; start < len(ends)-1; {
		end := start + 1
		space := 0
		for end < len(ends) && ends[end].runeEnd-ends[start].runeEnd <= aiAnswerSegmentRunes {
			if ends[end].space {
				// Keep the separator at the start of the next piece. Fyne's
				// word wrapper otherwise splits that piece's first word when
				// it does not fit the preceding piece's partially filled row.
				space = end - 1
			}
			end++
		}
		end--
		if end == start {
			end++ // a single grapheme longer than the normal budget
		} else if end < len(ends)-1 && space > start {
			end = space
		}
		pieces = append(pieces, text[ends[start].byteEnd:ends[end].byteEnd])
		start = end
	}
	return pieces
}
