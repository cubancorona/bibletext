package main

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// THE WEB'S AIR COMES FROM THE APP: the paragraph gap, the title's gap and the
// reporter indent are substituted from reading_spacing.go, never written here.
// Mutation: any of the three back as a literal in the stylesheet template.
func TestTheReadingPanesAirIsTheApps(t *testing.T) {
	css := testCSS()
	for _, want := range []string{
		"--pgap:calc(" + trimFloat(bibletext.ReadingParaGapEm()) + " * ",
		".text p.pst{font-style:italic; text-indent:0; margin:0 0 " + bibletext.EmCSS(bibletext.ReadingTitleGapEm()) + "}",
		"text-indent:" + bibletext.EmCSS(bibletext.ReadingReporterIndentEm()),
	} {
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet does not carry %q", want)
		}
	}
	for _, leftover := range []string{"__PARA_GAP__", "__HEAD_LEAD__", "__HEAD_TAIL__", "__TITLE_GAP__", "__INDENT__"} {
		if strings.Contains(css, leftover) {
			t.Errorf("placeholder %s was not substituted", leftover)
		}
	}
}

func trimFloat(v float64) string {
	s := bibletext.EmCSS(v)
	return strings.TrimSuffix(s, "em")
}
