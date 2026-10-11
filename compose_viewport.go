package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// composeViewport keeps the sheet's background full height while its form
// fits above the keyboard. The native editor stays outside the passage scroll:
// a UIKit view does not inherit clipping from a Fyne scroll container.
type composeViewport struct {
	canvas     fyne.Canvas
	root       *fyne.Container
	form       *fyne.Container
	normal     []fyne.CanvasObject
	toolbar    *fyne.Container
	singleRow  *fyne.Container
	passage    fyne.CanvasObject
	scroll     *container.Scroll
	metadata   *fyne.Container
	summary    *fyne.Container
	citation   fyne.CanvasObject
	disclosure *widget.Button
	expanded   bool
	dense      bool
	entry      fyne.CanvasObject
	excerpt    *noteExcerpt
	native     bool
}

func newComposeViewport(c fyne.Canvas, form *fyne.Container, compactLabel, passage, entry, actions fyne.CanvasObject, excerpt *noteExcerpt, citation string, native, keepStatus bool, focusEntry func()) *fyne.Container {
	v := &composeViewport{
		canvas: c, form: form, normal: append([]fyne.CanvasObject(nil), form.Objects...),
		passage: passage, entry: entry, excerpt: excerpt, native: native,
	}
	v.toolbar = container.NewBorder(nil, nil, compactLabel, actions, layout.NewSpacer())
	v.scroll = container.NewVScroll(container.New(squeezeWidthLayout{}, passage))
	cite := widget.NewRichText(&widget.TextSegment{Text: citation, Style: widget.RichTextStyle{
		ColorName: theme.ColorNamePrimary, SizeName: theme.SizeNameCaptionText,
	}})
	cite.Wrapping = fyne.TextWrapWord
	citeView := container.NewThemeOverride(cite, passageCitationTheme{fyne.CurrentApp().Settings().Theme()})
	v.citation = citeView
	v.disclosure = widget.NewButton("Show passage", func() {
		v.expanded = !v.expanded
		if slot, ok := entry.(*noteEntrySlot); ok {
			slot.settle(v.root.Refresh)
		} else {
			v.root.Refresh()
			// Fyne buttons clear focus before their callback. Keep typing in
			// the same draft, except when a very short viewport needs the
			// keyboard dismissed to make the expanded passage readable.
			if !v.dense || !v.expanded {
				focusEntry()
			}
		}
	})
	v.summary = container.NewBorder(nil, nil, nil, v.disclosure, citeView)
	var singleLabel fyne.CanvasObject = v.disclosure
	if keepStatus {
		singleLabel = container.NewHBox(v.disclosure, compactLabel)
	}
	v.singleRow = container.NewBorder(nil, nil, singleLabel, actions, citeView)
	v.metadata = container.NewBorder(v.summary, nil, nil, nil, v.scroll)
	v.root = container.New(v, form)
	return v.root
}

// The explicit full-canvas popup owns the height, rather than a form minimum
// that could force a non-modal popup underneath the keyboard.
func (*composeViewport) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (v *composeViewport) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	if slot, ok := v.entry.(*noteEntrySlot); ok {
		quiet := slot.quiet
		slot.quiet = true
		defer func() {
			slot.quiet = quiet
			if !quiet {
				slot.push()
			}
		}()
	}
	pos, area := canvasArea(v.canvas)
	bottom := v.canvas.Size().Height
	if area.Height > 0 {
		bottom = pos.Y + area.Height
	}
	// Mobile driver positions are relative to the interactive-area origin;
	// restore that origin before comparing with the full-canvas bottom.
	driverOrigin, _ := v.canvas.InteractiveArea()
	top := fyne.CurrentApp().Driver().AbsolutePositionForObject(v.root).Y + driverOrigin.Y
	room := max(float32(0), min(size.Height, bottom-top))
	v.excerpt.setMaxLines(noteExcerptMaxLinesFor(true, v.canvas.Size().Height))
	setPassagePreviewCompact(v.passage, false)
	v.form.Layout = layout.NewVBoxLayout()
	v.form.Objects = v.normal
	// Wrapping content needs a width pass before its height can be measured.
	v.form.Resize(fyne.NewSize(size.Width, v.form.MinSize().Height))
	v.form.Resize(fyne.NewSize(size.Width, v.form.MinSize().Height))
	if v.form.MinSize().Height <= room && !v.expanded {
		v.form.Move(fyne.Position{})
		return
	}
	v.excerpt.setMaxLines(1)
	setPassagePreviewCompact(v.passage, true)
	v.summary.Resize(fyne.NewSize(size.Width, v.summary.MinSize().Height))
	v.summary.Refresh()
	minimumEditor := min(v.entry.MinSize().Height, float32(44))
	minimumContext := float32(0)
	if v.expanded {
		minimumContext = v.excerpt.MinSize().Height + theme.Padding()
	}
	v.dense = room < v.toolbar.MinSize().Height+v.summary.MinSize().Height+minimumEditor+minimumContext+2*theme.Padding()
	// Returning to typing on an exceptionally short canvas folds the passage
	// back into the single toolbar row, giving the draft the available room.
	if v.dense && !v.native && v.canvas.Focused() != nil {
		v.expanded = false
	}
	if v.expanded {
		v.excerpt.setMaxLines(len([]rune(v.excerpt.text)) + 1)
		v.citation.Hide() // the expanded passage contains its own citation
		if v.disclosure.Text != "Hide passage" {
			v.disclosure.SetText("Hide passage")
		}
		v.scroll.Show()
	} else {
		v.citation.Show()
		if v.disclosure.Text != "Show passage" {
			v.disclosure.SetText("Show passage")
		}
		v.scroll.Hide()
	}
	// A narrow refit can wrap the citation alongside the disclosure. Measure
	// it at the new width before reserving space above the editor.
	v.summary.Resize(fyne.NewSize(size.Width, v.summary.MinSize().Height))
	v.summary.Refresh()
	metadataH := v.summary.MinSize().Height
	if v.expanded {
		metadataH += theme.Padding() + v.passage.MinSize().Height
	}
	if v.dense {
		metadataH = 0
		if v.expanded {
			metadataH = v.passage.MinSize().Height
		}
		v.form.Objects = []fyne.CanvasObject{v.singleRow, v.scroll, v.entry}
		v.form.Layout = composeCompactLayout{native: v.native, metadataHeight: metadataH, expanded: v.expanded, dense: true}
		v.form.Resize(fyne.NewSize(size.Width, room))
		v.form.Move(fyne.Position{})
		return
	}
	v.form.Objects = []fyne.CanvasObject{v.toolbar, v.metadata, v.entry}
	v.form.Layout = composeCompactLayout{native: v.native, metadataHeight: metadataH, expanded: v.expanded}
	v.form.Resize(fyne.NewSize(size.Width, room))
	v.form.Move(fyne.Position{})
}

type composeCompactLayout struct {
	native         bool
	metadataHeight float32
	expanded       bool
	dense          bool
}

func (composeCompactLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (l composeCompactLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	toolbar, passage, entry := objects[0], objects[1], objects[2]
	gap := theme.Padding()
	toolbarH := toolbar.MinSize().Height
	toolbar.Resize(fyne.NewSize(size.Width, toolbarH))
	toolbar.Move(fyne.Position{})
	// Measuring the normal form moves the shared actions. An unchanged toolbar
	// size would otherwise skip restoring their positions during a later pass.
	toolbar.Refresh()
	gaps := 2 * gap
	if l.dense && !l.expanded {
		gaps = gap
	}
	remaining := max(float32(0), size.Height-toolbarH-gaps)
	// Two native text lines when there is room; the UITextView scrolls longer
	// drafts itself. The fallback Fyne editor retains its usual line height.
	entryH := entry.MinSize().Height
	if l.native && l.expanded {
		entryH = 60
	}
	passageH := l.metadataHeight
	if entryH+passageH > remaining {
		floor := min(entryH, float32(44))
		entryH = max(floor, remaining-passageH)
	}
	entryH = min(entryH, remaining)
	passageH = max(float32(0), min(passageH, remaining-entryH))
	passage.Resize(fyne.NewSize(size.Width, passageH))
	passage.Move(fyne.NewPos(0, toolbarH+gap))
	passage.Refresh()
	entry.Resize(fyne.NewSize(size.Width, entryH))
	entry.Move(fyne.NewPos(0, toolbarH+gaps+passageH))
}
