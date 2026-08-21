package bibletext

// A READABLE MEASURE FOR LIST SURFACES, shared by every platform.
//
// The reading pane has had a measure for a long time: the reporter column
// (reporterMeasureEm) keeps scripture near the ~59-character line the U.S.
// Reports set, on iPad and desktop alike. The LISTS never had one. They were
// written on a phone, where "take the width you are given" is right because the
// width you are given is about 440pt, and they kept taking it when the same
// views were handed an iPad or a maximised desktop window.
//

// becomes a long thin box with its words stranded at the far left, reading as an
// empty input field rather than as something a person wrote; a book row is a
// name adrift in a metre of nothing; a search hit's reference and its verse end
// up so far apart the eye has to travel between them.
//

// "prioritize compatibility and uniformity with other platforms where it makes
// sense so we don't have to keep reworking everything for the various
// platforms"). A surface opts in by wrapping its content once; the phone is
// untouched because its panes are narrower than the measure, and the desktop
// sidebar is untouched for the same reason. No per-platform branch, no build
// tag, nothing to keep in step.
//
// It is NOT for the reading pane, which has its own typographic measure and
// whose native overlays own their insets.

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// readableColumnMax is the widest a list surface may become.
//
// Chosen just above the reading column's own measure (reporterMeasureEm × the
// body size ≈ 578pt at the default text size) so a note, a search hit and the
// scripture they refer to are all set to about the same line length. A phone
// pane is far narrower than this, which is what makes the helper inert there.
const readableColumnMax float32 = 620

// readableColumnLayout caps its child at readableColumnMax and centres it,
// leaving the full height alone.
//
// A layout rather than fixed padding because the inset needed is a function of
// the pane's width, which only the layout pass knows — the same view is 390pt
// on a phone, ~1150pt on an iPad in this layout, and anything at all in a
// resizable desktop window.
type readableColumnLayout struct{ max float32 }

func (l readableColumnLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	max := l.max
	if max <= 0 {
		max = readableColumnMax
	}
	for _, o := range objs {
		w := s.Width
		if w > max {
			// Never cap BELOW what the child actually needs: a child whose own
			// minimum is wider than the measure would be clipped, which is a
			// worse failure than a wide column and one that only shows up at a
			// text size or in a locale nobody tested.
			cap := max
			if mw := o.MinSize().Width; mw > cap {
				cap = mw
			}
			if w > cap {
				w = cap
			}
		}
		o.Resize(fyne.NewSize(w, s.Height))
		o.Move(fyne.NewPos((s.Width-w)/2, 0))
	}
}

func (l readableColumnLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var m fyne.Size
	for _, o := range objs {
		m = m.Max(o.MinSize())
	}
	return m
}

// readableColumn wraps a list surface so it stops growing past the measure.
func readableColumn(inner fyne.CanvasObject) fyne.CanvasObject {
	return boundedColumn(readableColumnMax, inner)
}

// boundedColumn is readableColumn at a width the caller chooses.
//
// It exists for the books GRID, which wants a wider bound than a list: extra
// width in a list is white space beside one short item, while extra width in a
// grid is another column — fewer rows to scroll and a shorter reach to a
// target. Same mechanism, same centring, one number apart, so the two cannot
// drift into two different ideas of what "too wide" means.
func boundedColumn(max float32, inner fyne.CanvasObject) fyne.CanvasObject {
	return container.New(readableColumnLayout{max: max}, inner)
}
