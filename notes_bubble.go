package bibletext

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// noteBubble draws somebody's words the way the reading page draws them.
//
// ONE BUILDER, used by the reading banner and by the notes browser, because the
// owner asked for the two to match and two hand-built lookalikes drift the
// moment either is touched. On iOS and macOS the reading page draws its bubble
// natively (a rounded rect with a tail, reading_ios.go / reading_macos.go); this
// is the Fyne twin of that shape, and the browser is Fyne on every platform, so
// this is what "looks like the reading view" means there.
//
// The text inside is UNTRUSTED — another person's message. It is a Label, never
// markup, never RichText, and it is never the app speaking: whatever wraps this
// must attribute it. noteBubbleWithByline does that; use it unless you have a
// reason not to.
func noteBubble(text string, pal palette) fyne.CanvasObject {
	body := widget.NewLabel(strings.TrimSpace(text))
	body.Wrapping = fyne.TextWrapWord
	return surface(body, pal.SurfaceAlt, pal.Border, fyne.Size{})
}

// noteBubbleWithByline is the bubble plus the attribution that must always
// accompany it, and the attribution sits OUTSIDE the bubble (owner directive).
//
// That is not decoration. Inside the bubble, a line saying which translation a
// note came from would read as part of the message — as though the sender had
// typed it. Outside, it reads as what it is: the app telling you where this came
// from. The bubble holds their words and nothing else.
func noteBubbleWithByline(text, byline, version string, pal palette) fyne.CanvasObject {
	rows := []fyne.CanvasObject{noteBubble(text, pal)}

	attribution := strings.TrimSpace(byline)
	if v := strings.TrimSpace(version); v != "" {
		if attribution != "" {
			attribution += " · " + v
		} else {
			attribution = v
		}
	}
	if attribution != "" {
		who := widget.NewLabel(attribution)
		who.TextStyle = fyne.TextStyle{Italic: true}
		who.Importance = widget.LowImportance
		rows = append(rows, who)
	}
	return container.NewVBox(rows...)
}

// noteVersionName is the translation a note was written in, in words a reader
// recognises. An id the registry does not know degrades to the id itself rather
// than to nothing: a note is still from SOMEWHERE, and saying so is better than
// implying it came from the translation on screen.
func noteVersionName(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if v, ok := versionByID(id); ok {
		return v.Name
	}
	return id
}
