package bibletext

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// Each cut of the reading family is one resource for the life of the process,
// and the Windows and Linux pane measures and draws with those resources. The
// toolkit keeps a parsed face for every resource it is handed, keyed on the
// resource, so a cut built afresh for each call was parsed again and kept.
func TestTheReadingFacesAreResolvedOnce(t *testing.T) {
	a, b := loadReadingFonts(), loadReadingFonts()
	for _, c := range []struct {
		cut  string
		x, y fyne.Resource
	}{
		{"regular", a.regular, b.regular},
		{"bold", a.bold, b.bold},
		{"italic", a.italic, b.italic},
		{"bold italic", a.boldItalic, b.boldItalic},
	} {
		if c.x == nil || c.x != c.y {
			t.Errorf("the %s cut is a different resource on each call", c.cut)
		}
	}

	app := test.NewApp()
	defer app.Quit()
	p := newTestPane(t, psalm23State(), 420)
	if p.faceFor("word", true) != a.italic {
		t.Error("the pane's italic is not the family's italic resource")
	}
	if p.headingFace() != a.bold || p.numeralFace() != a.bold {
		t.Error("the pane's bold is not the family's bold resource")
	}
}
