package bibletext

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// modelSettingsLayout places the Model and Reasoning effort / Thinking budget
// plain value rows on the ASSISTANT card. Each row decides from its own live
// label and value text whether both fit on one line or stack onto two lines.
// The container's reported bottom aligns flush with the final visible row's
// padded text box so the following VBox caption sits at the same bottom-note
// spacing as READING's caption.
type modelSettingsLayout struct {
	modelLabel, effortLabel *widget.Label
	model, effort           *settingsPickerRow
	width                   float32
}

func modelSettingsRows(model, effort *settingsPickerRow) *fyne.Container {
	rows := &modelSettingsLayout{
		modelLabel:  model.label,
		effortLabel: effort.label,
		model:       model,
		effort:      effort,
	}
	return container.New(rows, model, model.label, effort, effort.label)
}

func (r *modelSettingsLayout) blockGeometry(w float32) (modelY, modelH, effortY, effortH, totalH float32, mg, eg pickerRowGeometry) {
	in := r.model.Theme().Size(theme.SizeNameInnerPadding)
	mg = r.model.geometry(w)
	modelY = -mg.labelPos.Y
	modelH = mg.height
	modelFoot := modelY + mg.textBottom + in
	totalH = modelFoot
	if r.effort.Visible() {
		eg = r.effort.geometry(w)
		effortLabelY := modelFoot + theme.Padding()
		effortY = effortLabelY - eg.labelPos.Y
		effortH = eg.height
		totalH = effortY + eg.textBottom + in
	}
	return
}

func (r *modelSettingsLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	_, _, _, _, totalH, _, _ := r.blockGeometry(r.width)
	minW := r.modelLabel.MinSize().Width
	if r.effort.Visible() {
		minW = max(minW, r.effortLabel.MinSize().Width)
	}
	return fyne.NewSize(minW, totalH)
}

func (r *modelSettingsLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	r.width = size.Width
	modelY, modelH, effortY, effortH, _, mg, eg := r.blockGeometry(size.Width)
	r.model.Move(fyne.NewPos(0, modelY))
	r.model.Resize(fyne.NewSize(size.Width, modelH))
	r.model.Refresh()
	r.modelLabel.Move(fyne.NewPos(0, modelY+mg.labelPos.Y))
	r.modelLabel.Resize(mg.labelSize)
	if r.effort.Visible() {
		r.effort.Move(fyne.NewPos(0, effortY))
		r.effort.Resize(fyne.NewSize(size.Width, effortH))
		r.effort.Refresh()
		r.effortLabel.Move(fyne.NewPos(0, effortY+eg.labelPos.Y))
		r.effortLabel.Resize(eg.labelSize)
	}
}
