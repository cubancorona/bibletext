package bibletext

// Renderer-free helpers for asserting on CONSTRUCTED widget trees. They descend
// containers, scrolls, splits, theme overrides, popups and this package's own
// wrapper widgets through their content fields — never via test.WidgetRenderer —
// so they add no renderer creation of their own on top of what the code under
// test already does. NOTE: that alone is not what keeps the race-enabled tests
// clean under -race. The racy font-cache clear is SETTINGS-change-driven, so the
// real rule is that race-enabled test files must never touch app settings/theme
// after test.NewApp() (no SetTheme, no themedTestApp — those live in the
// //go:build !race files; see ui_focus_test.go).

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// walkTree visits o and every descendant reachable without creating a renderer.
// Widgets not listed here are visited but not descended into.
func walkTree(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	visit(o)
	switch v := o.(type) {
	case *fyne.Container:
		for _, c := range v.Objects {
			walkTree(c, visit)
		}
	case *container.Scroll:
		walkTree(v.Content, visit)
	case *container.ThemeOverride:
		walkTree(v.Content, visit)
	case *container.Split:
		walkTree(v.Leading, visit)
		walkTree(v.Trailing, visit)
	case *widget.PopUp:
		walkTree(v.Content, visit)
	case *tapBox:
		walkTree(v.content, visit)
	case *searchResultCard:
		walkTree(v.content, visit)
	}
}

// treeTexts gathers the user-visible strings under o: canvas texts, labels,
// button labels, and rich-text content (segments joined per widget). Empty
// strings are dropped — otherwise treeHasText(o, "") would match vacuously on
// any blank label, letting an assertion whose expected value degenerated to ""
// pass against an empty view.
func treeTexts(o fyne.CanvasObject) []string {
	var out []string
	add := func(s string) {
		if s != "" {
			out = append(out, s)
		}
	}
	walkTree(o, func(n fyne.CanvasObject) {
		switch v := n.(type) {
		case *canvas.Text:
			add(v.Text)
		case *widget.Label:
			add(v.Text)
		case *widget.Button:
			add(v.Text)
		case *widget.RichText:
			add(segmentText(v.Segments))
		}
	})
	return out
}

// treeHasText reports whether any string gathered by treeTexts equals want.
func treeHasText(o fyne.CanvasObject, want string) bool {
	for _, s := range treeTexts(o) {
		if s == want {
			return true
		}
	}
	return false
}

// findTreeButton returns the first *widget.Button under o with the given label.
func findTreeButton(o fyne.CanvasObject, label string) *widget.Button {
	var found *widget.Button
	walkTree(o, func(n fyne.CanvasObject) {
		if b, ok := n.(*widget.Button); ok && found == nil && b.Text == label {
			found = b
		}
	})
	return found
}
