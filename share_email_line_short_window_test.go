//go:build !ios && !android

package bibletext

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"fyne.io/fyne/v2"
)

// The share sheet's Email… line on a short desktop window keeps the sheet
// clear of the header or over it, never partway down it. The line makes the
// sheet taller; the sheet is sized again with it through its own fit, which
// covers the header where the taller sheet cannot clear it (decision 35).
// Before the two were merged, the line pushed a sheet that cleared the header
// up into the gap below it at 420x470, and partway down the Go to chip at
// 420x430 and 360x380. At 420x500 the sheet clears the header either way.
func TestTheEmailLineKeepsTheShareSheetClearOfOrOverTheHeader(t *testing.T) {
	for _, sz := range []fyne.Size{{Width: 420, Height: 500}, {Width: 420, Height: 470}, {Width: 420, Height: 430}, {Width: 360, Height: 380}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", sz.Width, sz.Height), func(t *testing.T) {
			h := newShareSheetHarness(t)
			h.st.window.Resize(sz)
			h.shareCitation()
			p := h.sheet()
			if h.st.header == nil {
				t.Fatal("control: no header recorded")
			}
			drv := fyne.CurrentApp().Driver()
			headerTop := drv.AbsolutePositionForObject(h.st.header).Y
			headerBottom := headerTop + h.st.header.Size().Height
			canvasH := h.st.window.Canvas().Size().Height
			h.composeErr = errors.New("no mail program")
			h.tapEmail(p)
			if !emailFailedShown(p) {
				t.Fatal("control: the Email… line is not shown")
			}
			top, bottom := sheetBox(t, p)
			clear := top >= headerBottom+sheetHeaderGap-0.5
			// Covering, the sheet starts at the header's top edge and is no
			// taller than the room from there (coverHeaderUnlessClear).
			covering := math.Abs(float64(top-headerTop)) <= 0.5 && bottom <= canvasH-headerTop+0.5
			if !clear && !covering {
				t.Errorf("with the line the sheet spans %.1f..%.1f: neither clear of the header (from %.1f) nor covering it (from %.1f)",
					top, bottom, headerBottom+sheetHeaderGap, headerTop)
			}
		})
	}
}
