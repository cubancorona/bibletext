//go:build !race

package bibletext

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Compare the finished header with its previous centering structure. Only the
// structure that used to surround the two existing buttons is reconstructed;
// the actual buttons, theme and complete header remain the shipping ones.
func TestHeaderTouchTargetsKeepTheirVisibleLayout(t *testing.T) {
	type headerCase struct {
		name    string
		mobile  bool
		width   float32
		variant fyne.ThemeVariant
	}
	cases := []headerCase{
		{"phone/320", true, 320, theme.VariantLight},
		{"phone/390", true, 390, theme.VariantLight},
	}
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		look := "light"
		if variant == theme.VariantDark {
			look = "dark"
		}
		for _, width := range []float32{320, 390, 900} {
			cases = append(cases, headerCase{fmt.Sprintf("desktop/%s/%g", look, width), false, width, variant})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			th := coverApp(t, tc.mobile)
			previousVariant := appearanceVariant
			appearanceVariant = func(*AppState) fyne.ThemeVariant { return tc.variant }
			t.Cleanup(func() { appearanceVariant = previousVariant })
			fyne.CurrentApp().Settings().SetTheme(forcedVariant{Theme: th, v: tc.variant})
			if got := th.palette().Background; got != paletteFor(tc.variant).Background {
				t.Fatalf("header palette = %v; want variant %v", got, tc.variant)
			}
			st := planTestState(t)
			st.theme = th
			header := buildHeader(st)
			page := container.NewBorder(header, nil, nil, nil, canvas.NewRectangle(st.pal().Background))
			w := test.NewTempWindow(t, page)
			w.Resize(fyne.NewSize(tc.width, 180))

			tree := drawnTree(t, page)
			sparkle := touchIconButton(t, tree, iconVerseOfDay)
			gear := touchIconButton(t, tree, theme.SettingsIcon())
			controls, _ := branchBelow(tree, sparkle, gear)
			parent, index, current := touchContainerSlot(t, controls)
			if len(parent.Objects) != 3 || index != 2 {
				t.Fatal("header fixture no longer has center, left and right border children")
			}
			originalLayout, left := parent.Layout, parent.Objects[1]
			buttons := []drawnObject{sparkle, gear}
			buttonSizes := []fyne.Size{sparkle.obj.Size(), gear.obj.Size()}
			before := w.Canvas().Capture()
			minimum := header.MinSize()
			legacy := container.NewVBox(layout.NewSpacer(),
				container.NewHBox(sparkle.obj, gear.obj), layout.NewSpacer())
			parent.Objects[index] = legacy
			// BorderLayout remembers edge identities separately from Objects.
			parent.Layout = layout.NewBorderLayout(nil, nil, left, legacy)
			header.Refresh()
			legacyTree := drawnTree(t, page)
			if got := header.MinSize(); got != minimum {
				t.Errorf("header minimum changed from %v to %v", got, minimum)
			}
			for i, beforeButton := range buttons {
				after := drawnAs(t, legacyTree, beforeButton.obj)
				if after.pos != beforeButton.pos || after.obj.Size() != buttonSizes[i] {
					t.Errorf("visible header button moved: %v -> %v", beforeButton.pos, after.pos)
				}
			}
			touchSamePixels(t, before, w.Canvas().Capture())

			parent.Objects[index] = current
			parent.Layout = originalLayout
			header.Refresh()
			if !tc.mobile {
				return
			}
			// Use fresh objects for dispatch checks: the comparison above moves
			// buttons between parents whose renderer caches can retain them.
			st = planTestState(t)
			st.theme = th
			header = buildHeader(st)
			page = container.NewBorder(header, nil, nil, nil, canvas.NewRectangle(st.pal().Background))
			w = test.NewTempWindow(t, page)
			w.Resize(fyne.NewSize(tc.width, 180))
			tree = drawnTree(t, page)
			buttons = []drawnObject{
				touchIconButton(t, tree, iconVerseOfDay),
				touchIconButton(t, tree, theme.SettingsIcon()),
			}
			fired := make([]int, 2)
			targets := make([]drawnObject, 2)
			for i, beforeButton := range buttons {
				button := beforeButton.obj.(*widget.Button)
				button.OnTapped = func() { fired[i]++ }
				buttonAt := drawnAs(t, tree, button)
				targets[i] = touchAddedTarget(t, tree, buttonAt)
				target := targets[i]
				if target.obj.Size().Height < 44 {
					t.Errorf("header target is only %v", target.obj.Size())
				}
				for _, p := range touchRingPoints(t, target, buttonAt) {
					old := fired[i]
					test.TapCanvas(w.Canvas(), p)
					if fired[i] != old+1 || fired[1-i] != 0 {
						t.Fatalf("tap in button %d's clear space reached the wrong control: %v", i, fired)
					}
				}
				button.Disable()
				old := fired[i]
				test.TapCanvas(w.Canvas(), touchRingPoints(t, target, buttonAt)[0])
				if fired[i] != old {
					t.Error("disabled header button accepted an expanded-area tap")
				}
				button.Enable()
				fired[i] = 0
			}
			if targets[0].overlaps(targets[1]) {
				t.Fatal("header touch targets overlap")
			}
			gap := fyne.NewPos((targets[0].pos.X+targets[0].obj.Size().Width+targets[1].pos.X)/2,
				targets[0].pos.Y+targets[0].obj.Size().Height/2)
			test.TapCanvas(w.Canvas(), gap)
			if fired[0] != 0 || fired[1] != 0 {
				t.Errorf("the gap between controls became clickable: %v", fired)
			}
		})
	}
}

func TestPooledNoteTrashUsesOnlyItsExistingColumn(t *testing.T) {
	th := coverApp(t, true)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() {
		deleteAllNotes(appPrefs())
		setNotesEnabled(true)
	})
	seed := func(verse int, text string) StoredNote {
		t.Helper()
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: verse, Text: text, Received: 1_700_000_000})
		if !ok {
			t.Fatal("could not seed a synthetic note")
		}
		return n
	}
	first := seed(16, "fixture first note")
	current := seed(17, strings.Repeat("fixture alpha beta gamma delta epsilon ", 8))
	keep := seed(18, "fixture retained note")
	st := planTestState(t)
	st.theme = th
	row := newBrowseRow(st, lightPalette)
	row.show(first)
	opened := 0
	row.card.onTap = func() { opened++ }
	w := test.NewTempWindow(t, row.root)
	w.Resize(fyne.NewSize(320, row.root.MinSize().Height))
	row.show(current)
	row.root.Refresh()
	w.Resize(fyne.NewSize(320, row.root.MinSize().Height))
	tree := drawnTree(t, row.root)
	trash := touchIconButton(t, tree, theme.DeleteIcon())
	buttonSize := trash.obj.Size()
	target := touchAddedTarget(t, tree, trash)
	if target.obj.Size().Width != buttonSize.Width || target.obj.Size().Height > 44 {
		t.Fatalf("trash target grew outside its narrow existing column: button=%v target=%v", buttonSize, target.obj.Size())
	}
	if target.obj.Size().Height <= buttonSize.Height {
		t.Fatal("the wrapping note did not gain any clear-space tap area")
	}

	// Restoring just the old NewCenter wrapper must produce the same row.
	common, _ := branchBelow(tree, trash, target)
	parent, index, expanded := touchContainerSlot(t, common)
	before, minimum := w.Canvas().Capture(), row.root.MinSize()
	parent.Objects[index] = container.NewCenter(trash.obj)
	row.root.Refresh()
	if got := row.root.MinSize(); got != minimum {
		t.Errorf("trash target changed row minimum: %v -> %v", minimum, got)
	}
	touchSamePixels(t, before, w.Canvas().Capture())
	parent.Objects[index] = expanded
	row.root.Refresh()
	tree = drawnTree(t, row.root)
	trash = drawnAs(t, tree, trash.obj)
	target = touchAddedTarget(t, tree, trash)

	// Blank space beyond the capped target still belongs to the note card.
	outside := fyne.NewPos(target.pos.X+target.obj.Size().Width/2, target.pos.Y-1)
	if !drawnAs(t, tree, row.card).contains(outside) {
		t.Fatal("fixture did not leave card space beyond the capped target")
	}
	test.TapCanvas(w.Canvas(), outside)
	if opened != 1 || len(allNotesForBrowsing(appPrefs())) != 3 {
		t.Fatal("space outside the trash target deleted a note or failed to open its row")
	}
	opened = 0
	test.TapCanvas(w.Canvas(), touchRingPoints(t, target, trash)[0])
	all := allNotesForBrowsing(appPrefs())
	if opened != 0 || len(all) != 2 {
		t.Fatalf("trash ring tap opened its card or deleted the wrong count: opens=%d notes=%d", opened, len(all))
	}
	for _, n := range all {
		if n.ID != first.ID && n.ID != keep.ID {
			t.Errorf("pooled row deleted its old note instead of current note %d", current.ID)
		}
	}
}

func touchIconButton(t *testing.T, tree []drawnObject, icon fyne.Resource) drawnObject {
	t.Helper()
	return oneDrawn(t, tree, "button "+icon.Name(), func(o fyne.CanvasObject) bool {
		b, ok := o.(*widget.Button)
		return ok && b.Icon != nil && b.Icon.Name() == icon.Name()
	})
}

func touchAddedTarget(t *testing.T, tree []drawnObject, button drawnObject) drawnObject {
	t.Helper()
	centre := button.pos.Add(fyne.NewPos(button.obj.Size().Width/2, button.obj.Size().Height/2))
	var found []drawnObject
	for _, d := range tree {
		if _, ok := d.obj.(fyne.Tappable); ok && d.obj != button.obj &&
			!button.under(d.obj) && d.contains(centre) {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d extra targets around %T, want one", len(found), button.obj)
	}
	return found[0]
}

// Locate a replaceable child even when a widget renderer lies between the
// common ancestor and the first containing *fyne.Container.
func touchContainerSlot(t *testing.T, child drawnObject) (*fyne.Container, int, fyne.CanvasObject) {
	t.Helper()
	branch := child.obj
	for i := len(child.path) - 1; i >= 0; i-- {
		if parent, ok := child.path[i].(*fyne.Container); ok {
			for j, object := range parent.Objects {
				if object == branch {
					return parent, j, branch
				}
			}
		}
		branch = child.path[i]
	}
	t.Fatal("no containing layout slot for the touch target")
	return nil, 0, nil
}

func touchRingPoints(t *testing.T, target, button drawnObject) []fyne.Position {
	t.Helper()
	size := target.obj.Size()
	var points []fyne.Position
	for _, p := range []fyne.Position{
		target.pos.Add(fyne.NewPos(size.Width/2, 0.5)),
		target.pos.Add(fyne.NewPos(size.Width/2, size.Height-0.5)),
	} {
		if !button.contains(p) {
			points = append(points, p)
		}
	}
	if len(points) == 0 {
		t.Fatal("fixture has no expanded tap space outside the original button")
	}
	return points
}

func touchSamePixels(t *testing.T, before, after image.Image) {
	t.Helper()
	if before.Bounds() != after.Bounds() {
		t.Fatalf("touch target changed capture bounds: %v -> %v", before.Bounds(), after.Bounds())
	}
	for y := before.Bounds().Min.Y; y < before.Bounds().Max.Y; y++ {
		for x := before.Bounds().Min.X; x < before.Bounds().Max.X; x++ {
			br, bg, bb, ba := before.At(x, y).RGBA()
			ar, ag, ab, aa := after.At(x, y).RGBA()
			if br != ar || bg != ag || bb != ab || ba != aa {
				t.Fatalf("touch target changed visible pixel at (%d,%d)", x, y)
			}
		}
	}
}
