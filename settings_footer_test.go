package bibletext

// THE SETTINGS BODY ENDS WHERE THE FOOTER BEGINS.
//
// A 13-inch iPad screenshot of Settings ends on the SHARED NOTES heading, its
// card out of sight below the pinned "Changes save automatically." footer, as
// if the section were empty. That is where the scroll happened to stop, not
// content drawn under the footer: the body is a scroll that ends above the
// footer, and scrolling it to the end brings the last section fully into view.
// This holds that, at the iPad sizes, so it stays a matter of scroll position.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func TestSettingsBodyEndsAboveTheFooter(t *testing.T) {
	for _, sc := range []struct {
		name string
		w, h float32
	}{
		{"13-inch iPad portrait", 1032, 1376},
		{"13-inch iPad landscape", 1376, 1032},
		{"11-inch iPad portrait", 834, 1194},
	} {
		t.Run(sc.name, func(t *testing.T) {
			_, _, popup := settingsWithIncludedKey(t, sc.w, sc.h, 1, nil)
			drv := fyne.CurrentApp().Driver()
			scroll := findScroll(popup.Content)
			if scroll == nil {
				t.Fatal("no scroll in the sheet")
			}
			var footer *canvas.Text
			walkTree(popup, func(o fyne.CanvasObject) {
				if tx, ok := o.(*canvas.Text); ok && strings.HasPrefix(tx.Text, "Changes save automatically") {
					footer = tx
				}
			})
			if footer == nil {
				t.Fatal("no footer in the sheet")
			}
			viewBottom := drv.AbsolutePositionForObject(scroll).Y + scroll.Size().Height
			footerTop := drv.AbsolutePositionForObject(footer).Y
			if viewBottom > footerTop+0.5 {
				t.Errorf("the body's scroll runs to %.1f, under the footer, which starts at %.1f", viewBottom, footerTop)
			}
			// Nothing of the body is a descendant of the footer's row, or the
			// footer of the body.
			if objectInTree(scroll.Content, footer) {
				t.Error("the footer scrolls with the body; it should be pinned below it")
			}

			// Scrolled to the end, the last card is wholly above the footer.
			scroll.ScrollToBottom()
			var last fyne.CanvasObject
			if c, ok := scroll.Content.(*fyne.Container); ok && len(c.Objects) == 1 {
				if form, ok := c.Objects[0].(*fyne.Container); ok && len(form.Objects) > 0 {
					last = form.Objects[len(form.Objects)-1]
				}
			}
			if last == nil {
				t.Fatal("could not find the body's last row")
			}
			lastBottom := drv.AbsolutePositionForObject(last).Y + last.Size().Height
			if lastBottom > viewBottom+0.5 {
				t.Errorf("scrolled to the end, the last row still ends at %.1f, below the body's %.1f", lastBottom, viewBottom)
			}
			t.Logf("body %.0fpt in a %.0fpt view", scroll.Content.Size().Height, scroll.Size().Height)
		})
	}
}
