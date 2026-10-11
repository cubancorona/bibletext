package bibletext

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/rivo/uniseg"
)

func TestAIAnswerLayoutPreservesUnicode(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("A synthetic word with café and e\u0301. ", 100),
		strings.Repeat("漢字👩🏽‍💻e\u0301", 300),
		strings.Repeat("a", 511) + "👨‍👩‍👧‍👦" + strings.Repeat("b", 600),
		strings.Repeat("A fixture with a line.\r\n", 80),
	} {
		pieces := aiAnswerTextPieces(text)
		if strings.Join(pieces, "") != text {
			t.Fatal("layout splitting changed the answer's words")
		}
		boundaries := map[int]bool{0: true}
		graphemes := uniseg.NewGraphemes(text)
		for graphemes.Next() {
			_, end := graphemes.Positions()
			boundaries[end] = true
		}
		end := 0
		for _, piece := range pieces {
			if piece == "" || !utf8.ValidString(piece) || utf8.RuneCountInString(piece) > aiAnswerSegmentRunes {
				t.Errorf("invalid or oversized layout piece with %d runes", utf8.RuneCountInString(piece))
			}
			end += len(piece)
			if !boundaries[end] {
				t.Error("layout splitting cut a grapheme cluster")
			}
		}
	}
	cluster := "a" + strings.Repeat("\u0301", aiAnswerSegmentRunes+10)
	if pieces := aiAnswerTextPieces(cluster); len(pieces) != 1 || pieces[0] != cluster {
		t.Error("an unusually long combining sequence must remain whole")
	}
}

func TestAIAnswerLayoutPreservesBlockStyles(t *testing.T) {
	text := strings.Repeat("A synthetic passage has an opening and an ending. ", 30)
	for _, style := range []widget.RichTextStyle{
		widget.RichTextStyleInline, widget.RichTextStyleHeading,
		widget.RichTextStyleCodeBlock, widget.RichTextStyleStrong,
	} {
		original := &widget.TextSegment{Text: text, Style: style}
		segments := aiAnswerLayoutSegments([]widget.RichTextSegment{original})
		if len(segments) < 2 || segmentText(segments) != text {
			t.Fatal("a long segment was not split with every word retained")
		}
		for i, segment := range segments {
			piece := segment.(*widget.TextSegment)
			expected := style
			if i < len(segments)-1 {
				expected.Inline = true
			}
			if piece.Style != expected {
				t.Error("splitting changed a font, colour, alignment or block ending")
			}
		}
	}
	quote := &widget.TextSegment{Text: text, Style: widget.RichTextStyleBlockquote}
	segments := aiAnswerLayoutSegments([]widget.RichTextSegment{quote})
	if len(segments) != 1 || segments[0] != quote {
		t.Error("a quote must retain the exact style Fyne uses for indentation")
	}
	list := &widget.ListSegment{Ordered: true, Items: []widget.RichTextSegment{
		&widget.ParagraphSegment{Texts: []widget.RichTextSegment{
			&widget.TextSegment{Text: text, Style: widget.RichTextStyleInline},
		}},
	}}
	list.SetStartNumber(4)
	aiAnswerLayoutSegments([]widget.RichTextSegment{list})
	paragraph := list.Items[0].(*widget.ParagraphSegment)
	if !list.Ordered || list.StartNumber() != 4 || len(list.Items) != 1 || len(paragraph.Texts) < 2 || segmentText(paragraph.Texts) != text {
		t.Error("splitting changed list numbering, item structure or text")
	}
	direct := &widget.ListSegment{Items: []widget.RichTextSegment{&widget.TextSegment{Text: text, Style: widget.RichTextStyleInline}}}
	aiAnswerLayoutSegments([]widget.RichTextSegment{direct})
	directParagraph, ok := direct.Items[0].(*widget.ParagraphSegment)
	if len(direct.Items) != 1 || !ok || segmentText(directParagraph.Texts) != text {
		t.Error("splitting a direct text item added list bullets or lost words")
	}
}

func captureAIAnswerLayout(t *testing.T, raw string, bounded bool, width float32) (float32, []byte, []string) {
	t.Helper()
	answer := widget.NewRichTextFromMarkdown("")
	answer.Wrapping = fyne.TextWrapWord
	answer.Resize(fyne.NewSize(width, 200))
	if bounded {
		setAIAnswerText(answer, raw)
	} else {
		answer.ParseMarkdown(raw)
	}
	scroll := container.NewVScroll(answer)
	w := test.NewWindow(scroll)
	defer w.Close()
	w.Resize(fyne.NewSize(width, 240))
	answer.Refresh()
	var image bytes.Buffer
	if err := png.Encode(&image, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
	rows := map[float32][]*canvas.Text{}
	var ys []float32
	for _, object := range test.WidgetRenderer(answer).Objects() {
		if text, ok := object.(*canvas.Text); ok {
			y := text.Position().Y
			if _, exists := rows[y]; !exists {
				ys = append(ys, y)
			}
			rows[y] = append(rows[y], text)
		}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
	var rowText []string
	for _, y := range ys {
		texts := rows[y]
		sort.Slice(texts, func(i, j int) bool { return texts[i].Position().X < texts[j].Position().X })
		var b strings.Builder
		for _, text := range texts {
			b.WriteString(text.Text)
		}
		rowText = append(rowText, b.String())
	}
	return answer.MinSize().Height, image.Bytes(), rowText
}

func TestAIAnswerLayoutPreservesRenderedProse(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	prose := strings.Repeat("A synthetic passage connects the field with the river. ", 18)
	for i, example := range []struct {
		text  string
		exact bool
	}{
		{prose, false},
		{"## A heading\n\n" + prose + "\n\nA second paragraph.", false},
		{"**" + prose + "**\n\n*An emphasized ending.*", false},
		{"- " + prose + "\n- A second item.\n\n1. A first step.\n2. " + prose, false},
		{"```\n" + prose + "\n```", false},
		{"> " + prose + "\n\nA paragraph after the quote.", true},
		{"## A short heading\n\n**Ordinary** *formatting* and `code`.\n\n> A short quote.", true},
		{"## " + prose, false},
		{strings.Repeat("a", 1200), false},
		{strings.Repeat("漢字👩🏽‍💻e\u0301", 180), false},
	} {
		raw := example.text
		for _, width := range []float32{310, 530} {
			beforeHeight, before, beforeRows := captureAIAnswerLayout(t, raw, false, width)
			afterHeight, after, afterRows := captureAIAnswerLayout(t, raw, true, width)
			if beforeHeight != afterHeight {
				t.Errorf("layout height changed at %.0fpt: %.2f to %.2f", width, beforeHeight, afterHeight)
			}
			if example.exact && !bytes.Equal(before, after) {
				t.Errorf("rendered answer changed at %.0fpt for %.40q", width, raw)
			}
			if strings.Join(beforeRows, "\n") != strings.Join(afterRows, "\n") {
				t.Errorf("answer row text changed at %.0fpt:\nbefore: %q\nafter: %q", width, beforeRows, afterRows)
			}
			if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
				for name, image := range map[string][]byte{"before": before, "after": after} {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("answer-%02d-%.0f-%s.png", i, width, name)), image, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}

func TestAIAnswerLayoutKeepsWordsOnPartialRows(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	// This fixture puts "meaning" at a segment boundary after a partially filled
	// row. A new segment beginning with the word instead of its preceding space
	// makes Fyne break it, even though the whole word fits the following row.
	unit := "The passage describes careful reading, historical context, and a thoughtful question about its meaning. "
	var text strings.Builder
	for text.Len() < 8192 {
		text.WriteString(unit)
		if text.Len()%700 < len(unit) {
			text.WriteString("\n\n")
		}
	}
	raw := text.String()[:8192]
	if pieces := aiAnswerTextPieces(raw); strings.Join(pieces, "") != raw {
		t.Fatal("partial-row fixture lost text while splitting")
	}
	for _, width := range []float32{294, 310} {
		beforeHeight, _, beforeRows := captureAIAnswerLayout(t, raw, false, width)
		afterHeight, _, afterRows := captureAIAnswerLayout(t, raw, true, width)
		if beforeHeight != afterHeight {
			t.Errorf("partial-row wrapping changed answer height at %.0fpt: %.2f to %.2f", width, beforeHeight, afterHeight)
		}
		if len(beforeRows) != len(afterRows) {
			t.Errorf("partial-row wrapping changed row count at %.0fpt: %d to %d", width, len(beforeRows), len(afterRows))
		}
		if strings.Count(strings.Join(afterRows, "\n"), "meaning.") != strings.Count(raw, "meaning.") {
			t.Errorf("partial-row wrapping split a word that fits a full row at %.0fpt", width)
		}
		for i := range beforeRows {
			// A separator can stay at a row's edge instead of being consumed by
			// wrapping; the words and their row boundaries must remain identical.
			if i >= len(afterRows) || strings.TrimSpace(beforeRows[i]) != strings.TrimSpace(afterRows[i]) {
				var after string
				if i < len(afterRows) {
					after = afterRows[i]
				}
				t.Errorf("partial-row wrapping changed row %d at %.0fpt: %q to %q", i, width, beforeRows[i], after)
				break
			}
		}
	}
}

func TestAIAnswerLayoutBoundsAllocations(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	text := strings.Repeat("A synthetic sentence about a passage. Each word is only a layout fixture. ", 115)
	allocated := func(bounded bool) uint64 {
		answer := widget.NewRichTextFromMarkdown("")
		answer.Wrapping = fyne.TextWrapWord
		answer.Resize(fyne.NewSize(310, 300))
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if bounded {
			setAIAnswerText(answer, text)
		} else {
			answer.ParseMarkdown(text)
		}
		answer.Resize(fyne.NewSize(310, answer.MinSize().Height))
		answer.Refresh()
		answer.MinSize()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(answer)
		return after.TotalAlloc - before.TotalAlloc
	}
	before, after := allocated(false), allocated(true)
	t.Logf("complete %d-byte answer: original %.1f MiB; bounded %.1f MiB", len(text), float64(before)/(1<<20), float64(after)/(1<<20))
	if after >= before/4 {
		t.Errorf("answer layout still allocates %.1f%% of the original long-paragraph path", 100*float64(after)/float64(before))
	}
}
