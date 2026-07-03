package bibletext

// Reader-adjustable scripture text size. Three steps rather than a slider —
// discrete named sizes are easier to reason about for the readers this exists
// for (Fyne doesn't inherit iOS Dynamic Type, so without this the app ignores
// the phone's Larger Text setting entirely). The scale multiplies the reading
// HTML's base font (buildChapterHTML); verse-number superscripts ride along at
// 0.66em, and the native verse-locating threshold is derived from the rendered
// fonts rather than a constant, so read-along highlighting and scroll anchors
// keep working at every size.

import "fyne.io/fyne/v2"

const prefTextSize = "reading.textSize"

// textSizeOptions maps the setting's stored value to its scale and label, in
// display order.
var textSizeOptions = []struct {
	ID    string
	Label string
	Scale float64
}{
	{"normal", "Normal", 1.0},
	{"large", "Large", 1.15},
	{"xl", "Extra large", 1.3},
}

// readingTextSizeID returns the persisted choice ("normal" when unset/unknown).
func readingTextSizeID() string {
	if app := fyne.CurrentApp(); app != nil {
		id := app.Preferences().String(prefTextSize)
		for _, o := range textSizeOptions {
			if o.ID == id {
				return id
			}
		}
	}
	return "normal"
}

func setReadingTextSizeID(id string) {
	if app := fyne.CurrentApp(); app != nil {
		app.Preferences().SetString(prefTextSize, id)
	}
}

// readingTextScale is the multiplier applied to the scripture body font.
func readingTextScale() float64 {
	id := readingTextSizeID()
	for _, o := range textSizeOptions {
		if o.ID == id {
			return o.Scale
		}
	}
	return 1.0
}
