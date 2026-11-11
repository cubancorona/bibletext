package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// printedTurnProblems is what is wrong with rows as a wrapping of text at
// width: text changed, a row too wide that could have been turned, or a row
// turned inside a citation that would have fitted a row of its own.
func printedTurnProblems(text string, rows []string, width float32, measure func(string) float32) []string {
	var out []string
	if strings.Join(rows, " ") != text {
		out = append(out, "the rows joined are not the text")
	}
	for i, row := range rows {
		if row == "" {
			out = append(out, "an empty row")
		}
		if measure(row) > width && strings.Contains(row, " ") {
			out = append(out, "row "+row+" is wider than the line and has a space to turn at")
		}
		if i == len(rows)-1 || strings.HasSuffix(row, ";") {
			continue
		}
		// The row ends inside a citation. That is in order only if the
		// citation it cuts is longer than a row.
		whole := row
		if at := strings.LastIndex(row, "; "); at >= 0 {
			whole = row[at+2:]
		}
		rest := strings.Join(rows[i+1:], " ")
		if at := strings.Index(rest, "; "); at >= 0 {
			rest = rest[:at+1]
		}
		if measure(whole+" "+rest) <= width {
			out = append(out, "row "+row+" turns inside a citation that fits a row")
		}
	}
	return out
}

// A PRINTED LINE TURNS BETWEEN CITATIONS. The toolkit's wrap turns at any
// space, and ended a row on "(1" before "John 4:9".
func TestAPrintedLineTurnsBetweenCitations(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	m := crossRefPrintedMeasure
	text := "Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; (1 John 4:9, 10; Rev. 1:5)"
	widest := m("(1 John 4:9, 10;")
	turned := 0
	for w := widest; w <= m(text)+8; w += 5 {
		rows := printedRows(text, w, m)
		for _, p := range printedTurnProblems(text, rows, w, m) {
			t.Errorf("at %v points: %s (rows %q)", w, p, rows)
		}
		for _, row := range rows[:len(rows)-1] {
			if !strings.HasSuffix(row, ";") {
				t.Errorf("at %v points a row ends inside a citation: %q", w, rows)
			}
		}
		if len(rows) > 1 {
			turned++
		}
	}
	if turned == 0 {
		t.Fatal("control: the line never turned, so nothing was tested")
	}
	// CONTROL: a wrap that turns at any space, as the toolkit's does, ends a
	// row inside a citation at one of the same widths.
	inside := false
	for w := widest; w <= m(text)+8; w += 5 {
		var rows []string
		row := ""
		for _, word := range strings.Fields(text) {
			if row != "" && m(row+" "+word) > w {
				rows = append(rows, row)
				row = ""
			}
			if row != "" {
				row += " "
			}
			row += word
		}
		for _, r := range rows {
			if !strings.HasSuffix(r, ";") {
				inside = true
			}
		}
	}
	if !inside {
		t.Fatal("control: a wrap at any space never ended a row inside a citation here, so the rule above tested nothing")
	}

	// A citation longer than a row turns after a comma of its own list, and
	// keeps to "; " where it can.
	list := "Ps. 18:2, 30, 31, 46; 28:1, 7, 8; 31:3; 71:3"
	w := m("Ps. 18:2, 30, 31,") + 1
	rows := printedRows(list, w, m)
	if got := strings.Join(rows, "|"); got != "Ps. 18:2, 30, 31,|46; 28:1, 7, 8;|31:3; 71:3" {
		t.Errorf("a citation longer than the row: %q", got)
	}
	for _, p := range printedTurnProblems(list, rows, w, m) {
		t.Errorf("a citation longer than the row: %s", p)
	}
	// A piece with neither a semicolon nor a comma turns at a space, and a
	// word longer than the row stands on a row of its own.
	one := "Song of Solomon 2:8—3:5"
	rows = printedRows(one, m("Song of Solomon")+1, m)
	if got := strings.Join(rows, "|"); got != "Song of Solomon|2:8—3:5" {
		t.Errorf("a single citation longer than the row: %q", got)
	}
	rows = printedRows(one, 4, m)
	if got := strings.Join(rows, "|"); got != "Song|of|Solomon|2:8—3:5" {
		t.Errorf("a row narrower than a word: %q", got)
	}
	// A line that fits is one row, and an empty one is none.
	if rows := printedRows(text, m(text), m); len(rows) != 1 || rows[0] != text {
		t.Errorf("a line that fits: %q", rows)
	}
	if rows := printedRows("", 100, m); len(rows) != 0 {
		t.Errorf("an empty line: %q", rows)
	}
}

// The widget draws those rows, at the size and in the colour of a line of
// the edition's print, and reports the height of the rows it drew.
func TestAPrintedLineDrawsItsRows(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	text := "Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; (1 John 4:9, 10; Rev. 1:5)"
	p := newCrossRefPrinted(text, lightPalette)
	w := crossRefPrintedMeasure("2 Thess. 2:16; (1 John 4:9, 10;") + 2
	p.Resize(fyne.NewSize(w, 200))
	r := test.WidgetRenderer(p).(*crossRefPrintedRenderer)
	r.Layout(p.Size())
	if got := strings.Join(r.rowTexts(), "|"); got != "Rom. 5:8; Eph. 2:4;|2 Thess. 2:16; (1 John 4:9, 10;|Rev. 1:5)" {
		t.Fatalf("rows = %q", got)
	}
	for i, row := range r.rows {
		if row.TextSize != crossRefGroupTextSize || row.Color != lightPalette.TextMuted {
			t.Errorf("row %d is %v points in %v", i, row.TextSize, row.Color)
		}
		if i > 0 && row.Position().Y <= r.rows[i-1].Position().Y {
			t.Errorf("row %d does not stand under the row before it", i)
		}
	}
	want := 3*r.rowH + 2*crossRefPrintedLeading
	if got := p.MinSize().Height; got != want {
		t.Errorf("the line reports %v points of height for three rows, want %v", got, want)
	}
	if got := p.MinSize().Width; got > w || got <= 0 {
		t.Errorf("the line wants %v points of width in %v", got, w)
	}
}
