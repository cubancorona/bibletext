package bibletext

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

// compareAnswer keeps Markdown blocks intact while Fyne shapes their inline
// text. Block spacing and indentation are independent of tight-list syntax.
type compareAnswer struct {
	widget.BaseWidget
	content *fyne.Container
	scope   *container.ThemeOverride
}

func newCompareAnswer() *compareAnswer {
	a := &compareAnswer{content: container.New(compareColumnLayout{padding: 8})}
	a.ExtendBaseWidget(a)
	a.scope = container.NewThemeOverride(a.content, compareTextTheme{Theme: a.Theme()})
	return a
}

func (a *compareAnswer) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(a.scope)
}

func (a *compareAnswer) Resize(size fyne.Size) {
	// Wrap the children before the outer scroll asks for their height.
	a.scope.Resize(size)
	a.BaseWidget.Resize(size)
}

func (a *compareAnswer) setText(raw string) {
	clean := sanitizeAIMarkdown(raw)
	source := []byte(clean)
	doc := goldmark.New().Parser().Parse(text.NewReader(source))
	width := a.Size().Width - 16
	if width <= 0 {
		// Construction has no viewport yet; avoid shaping at zero width.
		width = 310
	}
	r := compareMarkdownRenderer{source: source, theme: a.Theme()}
	var blocks []compareBlock
	if aiMarkdownNodeReachesNetwork(doc) {
		blocks = []compareBlock{r.paragraph([]widget.RichTextSegment{
			&widget.TextSegment{Text: clean, Style: widget.RichTextStyleInline},
		}, 0, width)}
	} else {
		blocks = r.blocks(doc, width)
	}
	a.content.Objects = compareBlockObjects(blocks)
	a.content.Layout = compareColumnLayout{blocks: blocks, padding: 8}
	a.scope.Theme = compareTextTheme{Theme: r.theme}
	a.scope.Refresh()
	a.Refresh()
}

type compareBlock struct {
	view          fyne.CanvasObject
	before, after float32
}

func compareBlockObjects(blocks []compareBlock) []fyne.CanvasObject {
	objects := make([]fyne.CanvasObject, len(blocks))
	for i, block := range blocks {
		objects[i] = block.view
	}
	return objects
}

// compareColumnLayout measures each wrapping child at its assigned width before
// placing the next block. Reporting zero minimum width keeps vertical scrolls
// from widening a narrow answer to a list marker or an unbroken word.
type compareColumnLayout struct {
	blocks                []compareBlock
	padding               float32
	leftInset, rightInset float32
}

func (l compareColumnLayout) gap(i int) float32 {
	if i == 0 || i >= len(l.blocks) {
		return 0
	}
	return fyne.Max(l.blocks[i-1].after, l.blocks[i].before)
}

func (l compareColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := l.padding * 2
	for i, object := range objects {
		h += l.gap(i) + object.MinSize().Height
	}
	return fyne.NewSize(0, h)
}

func (l compareColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if size.Width <= 0 {
		return
	}
	width := fyne.Max(1, size.Width-2*l.padding-l.leftInset-l.rightInset)
	y := l.padding
	for i, object := range objects {
		y += l.gap(i)
		object.Resize(fyne.NewSize(width, object.Size().Height))
		height := object.MinSize().Height
		object.Resize(fyne.NewSize(width, height))
		object.Move(fyne.NewPos(l.padding+l.leftInset, y))
		y += height
	}
}

type compareListItemLayout struct{ gutter float32 }

func (l compareListItemLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, fyne.Max(objects[0].MinSize().Height, objects[1].MinSize().Height))
}

func (l compareListItemLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if size.Width <= 0 {
		return
	}
	marker, body := objects[0], objects[1]
	width := fyne.Max(1, size.Width-l.gutter-8)
	body.Resize(fyne.NewSize(width, body.Size().Height))
	body.Resize(fyne.NewSize(width, body.MinSize().Height))
	body.Move(fyne.NewPos(l.gutter+8, 0))
	marker.Resize(fyne.NewSize(l.gutter, marker.MinSize().Height))
	marker.Move(fyne.NewPos(0, 0))
}

// Separate text blocks share the app's fonts and colours without adding Fyne's
// inner padding to every paragraph. Heading sizes belong to the answer, rather
// than the smaller global subheading used by navigation labels.
type compareTextTheme struct {
	fyne.Theme
}

func (t compareTextTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 0
	}
	if name == theme.SizeNameHeadingText {
		return 22
	}
	if name == theme.SizeNameSubHeadingText {
		return 20
	}
	return t.Theme.Size(name)
}

type compareMarkdownRenderer struct {
	source []byte
	theme  fyne.Theme
}

func (r compareMarkdownRenderer) textView(segments []widget.RichTextSegment, width float32) fyne.CanvasObject {
	rich := widget.NewRichText()
	rich.Wrapping = fyne.TextWrapWord
	rich.Resize(fyne.NewSize(fyne.Max(1, width), 0))
	rich.Segments = aiAnswerLayoutSegments(segments)
	rich.Refresh()
	rich.Resize(fyne.NewSize(fyne.Max(1, width), rich.MinSize().Height))
	return rich
}

func (r compareMarkdownRenderer) paragraph(segments []widget.RichTextSegment, level int, width float32) compareBlock {
	block := compareBlock{view: r.textView(segments, width), after: 10}
	if level > 0 {
		block.before, block.after = 16, 6
	}
	return block
}

func (r compareMarkdownRenderer) column(blocks []compareBlock, width float32, inset float32) *fyne.Container {
	column := container.New(compareColumnLayout{blocks: blocks, leftInset: inset, rightInset: inset}, compareBlockObjects(blocks)...)
	column.Resize(fyne.NewSize(fyne.Max(1, width), 0))
	column.Resize(fyne.NewSize(fyne.Max(1, width), column.MinSize().Height))
	return column
}

func (r compareMarkdownRenderer) blocks(parent ast.Node, width float32) []compareBlock {
	var blocks []compareBlock
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		switch n := node.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			blocks = append(blocks, r.paragraph(r.inline(node, widget.RichTextStyleInline), 0, width))
		case *ast.Heading:
			style := widget.RichTextStyleInline
			style.TextStyle.Bold = true
			if n.Level == 1 {
				style.SizeName = theme.SizeNameHeadingText
			} else if n.Level == 2 {
				style.SizeName = theme.SizeNameSubHeadingText
			}
			blocks = append(blocks, r.paragraph(r.inline(node, style), n.Level, width))
		case *ast.List:
			blocks = append(blocks, compareBlock{view: r.list(n, width), after: 10})
		case *ast.Blockquote:
			const inset = float32(18)
			children := r.blocks(node, fyne.Max(1, width-inset*2))
			blocks = append(blocks, compareBlock{view: r.column(children, width, inset), after: 10})
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			var code strings.Builder
			lines := node.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				code.Write(line.Value(r.source))
			}
			style := widget.RichTextStyleCodeInline
			blocks = append(blocks, r.paragraph([]widget.RichTextSegment{
				&widget.TextSegment{Text: strings.TrimSuffix(code.String(), "\n"), Style: style},
			}, 0, width))
		case *ast.ThematicBreak:
			blocks = append(blocks, compareBlock{view: widget.NewSeparator(), before: 10, after: 10})
		}
	}
	return blocks
}

func (r compareMarkdownRenderer) list(list *ast.List, width float32) *fyne.Container {
	var markers []fyne.CanvasObject
	gutter := float32(0)
	for i, item := 0, list.FirstChild(); item != nil; i, item = i+1, item.NextSibling() {
		label := "•"
		if list.IsOrdered() {
			label = fmt.Sprintf("%d.", list.Start+i)
		}
		marker := r.textView([]widget.RichTextSegment{
			&widget.TextSegment{Text: label, Style: widget.RichTextStyleInline},
		}, width).(*widget.RichText)
		marker.Wrapping = fyne.TextWrapOff
		marker.Refresh()
		markerSize, _ := fyne.CurrentApp().Driver().RenderedTextSize(label,
			r.theme.Size(theme.SizeNameText), fyne.TextStyle{}, r.theme.Font(fyne.TextStyle{}))
		gutter = fyne.Max(gutter, markerSize.Width)
		markers = append(markers, marker)
	}
	var items []compareBlock
	for i, item := 0, list.FirstChild(); item != nil; i, item = i+1, item.NextSibling() {
		contentWidth := fyne.Max(1, width-gutter-8)
		body := r.column(r.blocks(item, contentWidth), contentWidth, 0)
		row := container.New(compareListItemLayout{gutter: gutter}, markers[i], body)
		row.Resize(fyne.NewSize(width, row.MinSize().Height))
		items = append(items, compareBlock{view: row, after: 6})
	}
	return r.column(items, width, 0)
}

func compareDecodedText(value []byte) string {
	if bytes.IndexAny(value, "\\&\x00") < 0 {
		return string(value)
	}
	// Goldmark's text writer handles escapes and references in one pass.
	// Decode its escaped text once; this does not render or execute HTML.
	var escaped bytes.Buffer
	writer := bufio.NewWriter(&escaped)
	goldmarkhtml.DefaultWriter.Write(writer, value)
	_ = writer.Flush()
	return html.UnescapeString(escaped.String())
}

func (r compareMarkdownRenderer) inline(parent ast.Node, style widget.RichTextStyle) []widget.RichTextSegment {
	var segments []widget.RichTextSegment
	var run strings.Builder
	var runStyle widget.RichTextStyle
	flush := func() {
		if run.Len() > 0 {
			segments = append(segments, &widget.TextSegment{Text: run.String(), Style: runStyle})
			run.Reset()
		}
	}
	appendText := func(value string, s widget.RichTextStyle) {
		if value == "" {
			return
		}
		if runStyle != s {
			flush()
			runStyle = s
		}
		run.WriteString(value)
	}
	for node := parent.FirstChild(); node != nil; node = node.NextSibling() {
		switch n := node.(type) {
		case *ast.Text:
			value := string(n.Value(r.source))
			if !n.IsRaw() {
				value = compareDecodedText(n.Value(r.source))
			}
			appendText(value, style)
			if n.HardLineBreak() {
				appendText("\n", style)
			} else if n.SoftLineBreak() {
				appendText(" ", style)
			}
		case *ast.String:
			value := string(n.Value)
			if !n.IsCode() && !n.IsRaw() {
				value = compareDecodedText(n.Value)
			}
			appendText(value, style)
		case *ast.Emphasis:
			flush()
			inherited := style
			if n.Level == 2 {
				inherited.TextStyle.Bold = true
			} else {
				inherited.TextStyle.Italic = true
			}
			segments = append(segments, r.inline(node, inherited)...)
		case *ast.CodeSpan:
			inherited := style
			inherited.TextStyle.Monospace = true
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				value := child.(*ast.Text).Value(r.source)
				appendText(strings.ReplaceAll(string(value), "\n", " "), inherited)
			}
		}
	}
	flush()
	return segments
}
