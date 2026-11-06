//go:build !race

package bibletext

// THE LIST FITS THE NARROWEST PHONE, BY MEASUREMENT. A vertical scroll lays
// its content out at the content's own minimum width and clips the excess
// sideways with no bar to reach it (sheet_fit.go), so one child wanting more
// width than the panel has clips every row: the fold this list replaced did
// exactly that, its title at 18 bold with a button's chrome. The list is built
// of bounded kinds only (crossref_list.go) and the panel wraps it in
// squeezeWidthLayout as a belt; this is the test that can fail, under the
// app's own theme and fonts, which is why it stays out of the race run (the
// theme switch clears the font cache; widgettree_test.go).

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// crossRefListMisfits is everything in objs the list may not hold: an object
// whose MinSize is wider than limit, and a widget of a kind whose width
// cannot wrap (bannedIn). Every object is measured, not only the top level,
// so a wide child inside a block is named by its path.
func crossRefListMisfits(objs []fyne.CanvasObject, limit float32) []string {
	var out []string
	var walk func(o fyne.CanvasObject, path string)
	walk = func(o fyne.CanvasObject, path string) {
		if w := o.MinSize().Width; w > limit {
			out = append(out, fmt.Sprintf("%s %T wants %.0f points, over %.0f", path, o, w, limit))
		}
		switch x := o.(type) {
		case *tapCard:
			walk(x.content, path+"/card")
		case *fyne.Container:
			for i, c := range x.Objects {
				walk(c, path+"/"+strconv.Itoa(i))
			}
		}
	}
	for i, o := range objs {
		walk(o, strconv.Itoa(i))
	}
	for _, b := range bannedIn(objs) {
		out = append(out, "a "+b+" in the list")
	}
	return out
}

// narrowestPhoneBody is the width the list has on the narrowest phone: the
// panel's width on a 375×667 canvas less the 44 points of chrome around the
// list (showCrossRefs). The figure every bound in this file is written
// against is 283; the test fails if the arithmetic moves.
func narrowestPhoneBody(t *testing.T) float32 {
	t.Helper()
	limit := aiPanelSize(fyne.NewSize(375, 667)).Width - 44
	if limit != 283 {
		t.Fatalf("control: the narrowest body is %v points; the bounds here are written against 283", limit)
	}
	return limit
}

func appThemeForTest(t *testing.T) *bibleTheme {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	return th
}

func versesBetween(bd *BibleData, book string, ch, lo, hi int) []Verse {
	var out []Verse
	for _, v := range bd.GetChapter(book, ch) {
		if v.Verse >= lo && v.Verse <= hi {
			out = append(out, v)
		}
	}
	return out
}

func TestCrossRefListFitsTheNarrowestPhone(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	limit := narrowestPhoneBody(t)
	bd := publisherFixture()
	bd.Verses["2 Samuel"] = map[int][]Verse{15: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 15, Verse: 13, Text: "A fixture messenger came."}}}
	st := &AppState{Bible: bd, CurrentVersion: "nkjv", theme: th}
	type selection struct {
		book       string
		ch, lo, hi int
	}
	selections := []selection{
		{"John", 3, 1, 3}, {"John", 3, 2, 2}, {"Psalms", 3, 1, 1}, {"Psalms", 3, 2, 2},
		{"1 Samuel", 19, 12, 12}, {"Genesis", 7, 2, 2}, {"Hebrews", 1, 6, 6},
	}
	checked := 0
	for _, flag := range []bool{false, true} {
		withPublisherCrossRefs(t, "nkjv", flag)
		for _, s := range selections {
			st.CurrentBook, st.CurrentChapter = s.book, s.ch
			lst := buildCrossRefList(st, versesBetween(bd, s.book, s.ch, s.lo, s.hi), listFixtureRefs(), nil, st.pal(), func(crossRef) {})
			if len(lst.Objects) == 0 {
				t.Fatalf("control: flag %v, %s %d:%d-%d built an empty list", flag, s.book, s.ch, s.lo, s.hi)
			}
			checked++
			if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
				t.Errorf("flag %v, %s %d:%d-%d: %s", flag, s.book, s.ch, s.lo, s.hi, strings.Join(problems, "; "))
			}
			all := joinedTexts(lst.Objects)
			if strings.Contains(all, "Thomas Nelson") {
				t.Errorf("flag %v, %s %d: the notice is in the list", flag, s.book, s.ch)
			}
			if strings.Contains(strings.ToLower(all), "compare") {
				t.Errorf("flag %v, %s %d: a legend or compare word is in the list:\n%s", flag, s.book, s.ch, all)
			}
			// The notice stands once, in the footer, whatever the list shows.
			footer, sources := crossRefFooter(st, widget.NewButton("Close", func() {}))
			line, _ := crossRefFooterCredit(st, lst.Sections)
			setCrossRefSources(sources, line, limit)
			if n := strings.Count(strings.Join(treeTexts(footer), "\n"), "Thomas Nelson"); n != 1 {
				t.Errorf("flag %v, %s %d: the notice stands %d times in the footer, want once", flag, s.book, s.ch, n)
			}
		}
	}
	if checked != 2*len(selections) {
		t.Fatalf("control: %d lists checked", checked)
	}

	// CONTROL: the fold this list replaced is reported twice over — for its
	// kind, and for the width its title wants.
	acc := widget.NewAccordion(widget.NewAccordionItem("More references — Treasury of Scripture Knowledge", widget.NewLabel("x")))
	if acc.MinSize().Width <= limit {
		t.Fatalf("control: the accordion's title fits in %v points, so the width check cannot be shown to fail", limit)
	}
	problems := crossRefListMisfits([]fyne.CanvasObject{acc}, limit)
	if len(problems) != 2 || !strings.Contains(problems[0], "Accordion") || problems[1] != "a Accordion in the list" {
		t.Errorf("an accordion in the list is reported as %q", problems)
	}
}

// THE TEXTS THAT DO NOT WRAP FIT TOO. A card's reference (16 bold), a
// parallel card's pericope title (12 italic) and the section labels (11 bold
// monospace) are the list's only unwrapped texts, so each is measured at its
// widest: every title and passage label the synopsis can give a card, the
// widest run label the Psalter can produce, and the widest book names in a
// range label.
func TestEveryUnwrappedLineOfTheListFitsTheNarrowestPhone(t *testing.T) {
	appThemeForTest(t)
	limit := narrowestPhoneBody(t)
	gospelOnce.Do(loadGospelParallels)
	if len(gospelPericopes) == 0 {
		t.Fatal("control: no pericopes loaded")
	}
	widest := func(texts []string, size float32, style fyne.TextStyle) (string, float32) {
		var text string
		var w float32
		for _, s := range texts {
			if m := fyne.MeasureText(s, size, style).Width; m > w {
				text, w = s, m
			}
		}
		return text, w
	}
	var titles, labels []string
	for _, p := range gospelPericopes {
		titles = append(titles, p.title)
		for _, g := range gospelColumns {
			for _, s := range p.spans[g.book] {
				labels = append(labels, spanToCrossRef(g.book, s, p.title).label())
			}
		}
	}
	labels = append(labels, "Song of Solomon 2:8–3:5", "2 Thessalonians 2:16–17", "1 Thessalonians 5:12–13", "Leviticus 11", "2 Kings 18–20")
	if text, w := widest(titles, 12, fyne.TextStyle{Italic: true}); w > limit {
		t.Errorf("the pericope title %q wants %.0f points at 12 italic, over %.0f", text, w, limit)
	} else {
		t.Logf("widest pericope title %q: %.0f points", text, w)
	}
	if text, w := widest(labels, 16, fyne.TextStyle{Bold: true}); w > limit {
		t.Errorf("the card reference %q wants %.0f points at 16 bold, over %.0f", text, w, limit)
	} else {
		t.Logf("widest card reference %q: %.0f points", text, w)
	}
	runs := []string{parallelsLabel, treasuryLabel, publisherRunLabel("NKJV", 176, true), publisherRunLabel("NKJV", 0, true)}
	if text, w := widest(runs, 11, fyne.TextStyle{Bold: true, Monospace: true}); w > limit {
		t.Errorf("the section label %q wants %.0f points, over %.0f", text, w, limit)
	} else {
		t.Logf("widest section label %q: %.0f points", text, w)
	}
	// CONTROL: the measure can exceed the limit.
	if _, w := widest([]string{strings.Repeat("Thessalonians ", 4)}, 16, fyne.TextStyle{Bold: true}); w <= limit {
		t.Fatalf("control: a 56-character reference measures %.0f, within %.0f", w, limit)
	}
}
