package bibletext

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// modelSettingsLayout shares the value column between the two pickers.
// A single Model picker keeps the original row geometry. Where both labels
// and a Recommended button cannot fit, each label sits above its own button.
type modelSettingsLayout struct {
	modelLabel, effortLabel *widget.Label
	model, effort           fyne.CanvasObject
	modelHit                fyne.CanvasObject
	minButtonWidth, width   float32
}

func modelSettingsRows(state *AppState, model, effort *widget.Button) *fyne.Container {
	compact := compactTheme{Theme: state.theme, text: 15}
	modelSlot := container.NewThemeOverride(model, compact)
	effortSlot := container.NewThemeOverride(effort, compact)
	rows := &modelSettingsLayout{
		modelLabel: widget.NewLabel("Model"), effortLabel: widget.NewLabel("Reasoning effort"),
		model: container.NewClip(modelSlot), effort: effortSlot,
		minButtonWidth: effortSlot.MinSize().Width,
		modelHit:       newTappableArea(canvas.NewRectangle(color.Transparent), func() { model.OnTapped() }),
	}
	return container.New(rows, rows.modelLabel, rows.model, rows.effortLabel, rows.effort, rows.modelHit)
}

const modelPickerGap float32 = 4

func (r *modelSettingsLayout) labelWidth() float32 {
	if !r.effort.Visible() {
		return r.modelLabel.MinSize().Width
	}
	return max(r.modelLabel.MinSize().Width, r.effortLabel.MinSize().Width)
}

func (r *modelSettingsLayout) stacked() bool {
	return r.effort.Visible() && r.width > 0 && r.width < r.labelWidth()+modelPickerGap+r.minButtonWidth
}

func (r *modelSettingsLayout) rowHeight(label, control fyne.CanvasObject) float32 {
	h := max(float32(38), control.MinSize().Height)
	if r.stacked() {
		return label.MinSize().Height + h
	}
	return max(label.MinSize().Height, h)
}

func (r *modelSettingsLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	h := r.rowHeight(r.modelLabel, r.model)
	if r.effort.Visible() {
		h += modelPickerGap + r.rowHeight(r.effortLabel, r.effort)
	}
	return fyne.NewSize(max(r.labelWidth(), r.minButtonWidth), h)
}

func (r *modelSettingsLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r.width = size.Width
	column := r.labelWidth() + modelPickerGap
	originalStart := r.modelLabel.MinSize().Width + theme.Padding()
	if !r.effort.Visible() {
		column = originalStart
	}
	place := func(label, control fyne.CanvasObject, y float32) float32 {
		lh := label.MinSize().Height
		h := r.rowHeight(label, control)
		if r.stacked() {
			label.Move(fyne.NewPos(0, y))
			label.Resize(fyne.NewSize(size.Width, lh))
			control.Move(fyne.NewPos(0, y+lh))
			control.Resize(fyne.NewSize(size.Width, h-lh))
		} else {
			label.Move(fyne.NewPos(0, y+(h-lh)/2))
			label.Resize(fyne.NewSize(r.labelWidth(), lh))
			control.Move(fyne.NewPos(column, y))
			control.Resize(fyne.NewSize(max(float32(0), size.Width-column), h))
		}
		return h
	}
	h := place(r.modelLabel, r.model, 0)
	if r.effort.Visible() {
		place(r.effortLabel, r.effort, h+modelPickerGap)
	}
	// Extend the Model button to its original left edge only where the
	// shared column has actually moved it to the right.
	if !r.stacked() && column > originalStart {
		r.modelHit.Move(fyne.NewPos(originalStart, 0))
		r.modelHit.Resize(fyne.NewSize(column-originalStart, h))
		r.modelHit.Show()
	} else {
		r.modelHit.Hide()
		r.modelHit.Resize(fyne.Size{})
	}
}
