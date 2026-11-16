//go:build !race

package bibletext

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestCrossRefSourceGroupBuildsOnlyOnFirstOpen(t *testing.T) {
	appThemeForTest(t)
	built, relaid := 0, 0
	child := newCrossRefPrinted("Synthetic destination preview.", lightPalette)
	r := newCrossRefSourceGroup("origin-176", "176", "", "", "Alpha 1:2; Beta 3:4", false, lightPalette,
		&crossRefRelayout{fn: func() { relaid++ }}, func() []fyne.CanvasObject {
			built++
			return []fyne.CanvasObject{child}
		})
	other := newCrossRefSourceGroup("origin-177", "177", "", "", "Gamma 5:6", false, lightPalette, nil, nil)
	if r.id != "origin-176" || r.open || r.body.Visible() || len(r.body.Objects) != 0 || built != 0 || relaid != 0 {
		t.Fatal("a folded group built or displayed its children")
	}
	if r.header.MinSize().Height < 44 {
		t.Fatalf("tap height %.1f is below the existing 44-point minimum", r.header.MinSize().Height)
	}
	test.Tap(r.header)
	if !r.open || !r.body.Visible() || built != 1 || relaid != 1 || len(r.body.Objects) != 1 || r.body.Objects[0] != child {
		t.Fatal("opening did not build and show its children exactly once")
	}
	if other.open || other.built || other.body.Visible() {
		t.Fatal("opening one origin changed another")
	}
	r.setOpen(true)
	if built != 1 || relaid != 1 {
		t.Fatal("setting the current state rebuilt or relaid the group")
	}
	test.Tap(r.header)
	if r.open || r.body.Visible() || r.stem.Visible() || built != 1 || relaid != 2 {
		t.Fatal("closing failed to hide the body or asked for multiple layouts")
	}
	r.setOpen(true)
	if built != 1 || relaid != 3 || r.body.Objects[0] != child {
		t.Fatal("reopening did not retain the original child")
	}
}

func TestCrossRefSourceGroupEmptyBuilderRunsOnce(t *testing.T) {
	appThemeForTest(t)
	calls := 0
	r := newCrossRefSourceGroup("empty", "1", "", "", "Alpha 1:1", false, lightPalette, nil, func() []fyne.CanvasObject {
		calls++
		return nil
	})
	r.toggle()
	r.toggle()
	r.toggle()
	if calls != 1 {
		t.Fatalf("an empty result was built %d times, want once", calls)
	}
}

func TestCrossRefSourceGroupStartsOpenWithoutRelayout(t *testing.T) {
	appThemeForTest(t)
	calls, relaid := 0, 0
	r := newCrossRefSourceGroup("episode", "", "A synthetic episode", "Alpha 1:2–4", "Beta 5:6; Gamma 7:8", true, lightPalette,
		&crossRefRelayout{fn: func() { relaid++ }}, func() []fyne.CanvasObject {
			calls++
			return []fyne.CanvasObject{newCrossRefPrinted("A synthetic destination.", lightPalette)}
		})
	if !r.open || !r.body.Visible() || calls != 1 || relaid != 0 {
		t.Fatal("initial open state did not build once without requesting a parent layout")
	}
	if r.key != nil || r.pin != nil || r.stem != nil || r.margin != 0 {
		t.Fatal("an episode acquired a numeric key or an empty key margin")
	}
	r.keepLetterInView(200)
	r.toggle()
	if r.open || r.body.Visible() || len(r.labels) != 3 {
		t.Fatal("folding lost the title, origin or summary")
	}
	for i, want := range []string{"A synthetic episode", "Alpha 1:2–4", "Beta 5:6; Gamma 7:8"} {
		if r.labels[i].Text != want {
			t.Errorf("folded label %d = %q, want %q", i, r.labels[i].Text, want)
		}
	}
}

func settleSourceGroup(r *crossRefSourceGroup, width float32) {
	for i := 0; i < 8; i++ {
		r.Resize(fyne.NewSize(width, r.MinSize().Height))
		r.Refresh()
	}
}

func TestCrossRefSourceGroupFitsThreeDigitOriginsWithoutShrinking(t *testing.T) {
	appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	// A 320-point window gives the panel 280 points, less 44 of surrounding
	// chrome and 16 for the source group's two side insets.
	const width float32 = 220
	for _, size := range textSizeOptions {
		setReadingTextSizeID(size.ID)
		for _, key := range []string{"1", "43", "176", ""} {
			t.Run(fmt.Sprintf("%s-key-%s", size.ID, key), func(t *testing.T) {
				r := newCrossRefSourceGroup("fixture", key, "A longer synthetic heading for wrapping", "Heading before verse 176", "Alpha 1:2–3; Beta 4:5; Gamma 6:7, 8; Delta 9:10", false, lightPalette, nil,
					func() []fyne.CanvasObject {
						return []fyne.CanvasObject{newCrossRefPrinted("A synthetic body that wraps inside its origin's margin.", lightPalette)}
					})
				for _, open := range []bool{false, true, false} {
					r.setOpen(open)
					settleSourceGroup(r, width)
					if r.MinSize().Width > width || math.Abs(float64(r.Size().Height-r.MinSize().Height)) > 0.5 {
						t.Fatalf("open=%v: size %v, minimum %v exceeds or has not settled at width %.0f", open, r.Size(), r.MinSize(), width)
					}
					if r.key != nil {
						if r.key.TextSize != crossRefPanelSizes().reference || r.key.TextStyle.Italic || !r.key.TextStyle.Bold {
							t.Fatal("the numeric origin shrank or adopted the publisher letter's italic style")
						}
						if r.key.MinSize().Width+crossRefSourceKeyGap > r.margin {
							t.Fatal("numeric origin exceeds its measured margin")
						}
					}
					last := float32(0)
					for _, p := range r.labels {
						renderer := test.WidgetRenderer(p).(*crossRefPrintedRenderer)
						if got := strings.Join(renderer.rowTexts(), " "); got != p.Text {
							t.Errorf("open=%v: wrapped text lost content: %q", open, got)
						}
						for _, row := range renderer.rows {
							if renderer.measure(row.Text) > p.Size().Width+0.5 {
								t.Errorf("open=%v: row %q exceeds available width %.1f", open, row.Text, p.Size().Width)
							}
						}
						if p.Position().X < r.margin || p.Position().Y < last || p.Position().X+p.Size().Width > r.fold.Position().X-crossRefSourceIconGap+0.5 {
							t.Errorf("open=%v: label overlaps the key, preceding label or disclosure", open)
						}
						last = p.Position().Y + p.Size().Height
					}
					if last > r.header.Size().Height || r.fold.Size().Width != crossRefFoldIconSize {
						t.Fatal("header text overflows vertically or disclosure width changed")
					}
					if open && (r.body.Position().X != r.margin || r.body.Position().Y < last || r.body.Position().X+r.body.Size().Width > width+0.5) {
						t.Fatal("opened children do not fit beneath the header beside the key")
					}
				}
			})
		}
	}
}

func TestCrossRefSourceGroupKeepsNumericOriginInView(t *testing.T) {
	appThemeForTest(t)
	r := newCrossRefSourceGroup("long", "176", "", "", "Alpha 1:2; Beta 3:4", true, lightPalette, nil,
		func() []fyne.CanvasObject {
			var children []fyne.CanvasObject
			for i := 0; i < 8; i++ {
				children = append(children, crossRefWordsRow("A synthetic member with enough words to occupy several lines.", lightPalette, false))
			}
			return children
		})
	settleSourceGroup(r, 220)
	r.keepLetterInView(r.header.Size().Height + 30)
	if r.pinned <= 0 || r.pin == nil || !r.pin.Visible() || r.key.Visible() {
		t.Fatal("the numeric origin was not retained while its header scrolled away")
	}
	if r.pin.Text != "176" || r.pin.TextSize != r.key.TextSize || r.pin.TextStyle != r.key.TextStyle {
		t.Fatal("the retained origin changed its text or typography")
	}
	if r.stem.Position().Y <= r.pin.Position().Y || r.stem.Size().Height < 0 || r.stem.Position().X+r.stem.Size().Width >= r.margin {
		t.Fatal("the origin stem is not below the retained key inside its margin")
	}
	r.setOpen(false)
	if r.pinned != 0 || r.pin.Visible() || !r.key.Visible() || r.stem.Visible() {
		t.Fatal("closing did not restore the original key and remove its retained copy")
	}
	r.setOpen(true)
	settleSourceGroup(r, 220)
	r.keepLetterInView(10000)
	if r.pin.Position().Y+r.keyLine()+crossRefStemGap+crossRefStemFoot > r.Size().Height+0.5 {
		t.Fatal("the retained key extends beyond its own group")
	}
}
