package bibletext

import (
	"strings"
	"unicode/utf8"
)

// styledReadingReflowAnchor identifies text, rather than a distance from a
// verse number. It is transient: saved reading/history anchors keep their
// existing verse-and-point representation.
type styledReadingReflowAnchor struct {
	footnote      bool
	valid, top    bool
	model         string
	offset        int
	heading       string
	headingOffset int
	lineFraction  float32
}

func captureStyledReflow(lay *chapterLayout, y float32) styledReadingReflowAnchor {
	if lay == nil || len(lay.Lines) == 0 {
		return styledReadingReflowAnchor{}
	}
	anchor := styledReadingReflowAnchor{valid: true, top: y <= 0, model: lay.Text}
	if anchor.top {
		return anchor
	}
	index := len(lay.Lines) - 1
	for i, line := range lay.Lines {
		if y < line.Y+line.H {
			index = i
			break
		}
	}
	line := lay.Lines[index]
	if line.H <= 0 {
		return styledReadingReflowAnchor{}
	}
	anchor.offset = line.StartOffset
	anchor.lineFraction = (y - line.Y) / line.H
	if line.Heading != "" {
		first, end := index, index+1
		for first > 0 && lay.Lines[first-1].Heading != "" && lay.Lines[first-1].StartOffset == line.StartOffset {
			first--
		}
		for end < len(lay.Lines) && lay.Lines[end].Heading != "" && lay.Lines[end].StartOffset == line.StartOffset {
			end++
		}
		rows := make([]string, 0, end-first)
		for i := first; i < end; i++ {
			rows = append(rows, lay.Lines[i].Heading)
			if i < index {
				anchor.headingOffset += utf8.RuneCountInString(lay.Lines[i].Heading) + 1
			}
		}
		anchor.heading = strings.Join(rows, " ")
	}
	return anchor
}

func (anchor styledReadingReflowAnchor) y(lay *chapterLayout) (float32, bool) {
	if !anchor.valid || lay == nil || !sameStyledSelectionModel(anchor.model, lay.Text) {
		return 0, false
	}
	if anchor.top {
		return 0, true
	}
	if anchor.heading != "" {
		for i := 0; i < len(lay.Lines); i++ {
			first := i
			if lay.Lines[i].Heading == "" || lay.Lines[i].StartOffset != anchor.offset {
				continue
			}
			rows := []string{}
			for i < len(lay.Lines) && lay.Lines[i].Heading != "" && lay.Lines[i].StartOffset == anchor.offset {
				rows = append(rows, lay.Lines[i].Heading)
				i++
			}
			if strings.Join(rows, " ") != anchor.heading {
				return 0, false
			}
			offset := 0
			for n, row := range rows {
				end := offset + utf8.RuneCountInString(row)
				if anchor.headingOffset <= end {
					line := lay.Lines[first+n]
					return line.Y + anchor.lineFraction*line.H, true
				}
				offset = end + 1
			}
			return 0, false
		}
		return 0, false
	}
	for _, line := range lay.Lines {
		if line.Heading == "" && line.StartOffset <= anchor.offset && anchor.offset < line.EndOffset {
			return line.Y + anchor.lineFraction*line.H, true
		}
	}
	return 0, false
}

// The apparatus is not selectable chapter text. Its own transient line map
// lets a viewport inside it survive wrapping without entering lay.Text.
func styledFootnoteReflowLayout(pane *styledReadingPane) *chapterLayout {
	if pane == nil || !pane.fnGeom.present {
		return nil
	}
	lay := &chapterLayout{}
	var rows []string
	offset := 0
	for _, text := range pane.fnGeom.texts {
		length := utf8.RuneCountInString(text.Text)
		lay.Lines = append(lay.Lines, styledLine{Y: text.Y, H: pane.styledLineHeight() * styledFnRatio, StartOffset: offset, EndOffset: offset + length})
		rows = append(rows, text.Text)
		offset += length + 1
	}
	lay.Text = strings.Join(rows, " ")
	return lay
}

func captureStyledPaneReflow(pane *styledReadingPane, y float32) styledReadingReflowAnchor {
	if pane == nil {
		return styledReadingReflowAnchor{}
	}
	if pane.fnGeom.present && y >= pane.fnGeom.rect.Y {
		anchor := captureStyledReflow(styledFootnoteReflowLayout(pane), y)
		anchor.footnote = true
		return anchor
	}
	return captureStyledReflow(pane.lay, y)
}

func (anchor styledReadingReflowAnchor) paneY(pane *styledReadingPane) (float32, bool) {
	if pane == nil {
		return 0, false
	}
	if anchor.footnote {
		return anchor.y(styledFootnoteReflowLayout(pane))
	}
	return anchor.y(pane.lay)
}
