package bibletext

// WHAT A LIGHT/DARK CHANGE MUST NOT LEAVE BEHIND OUTSIDE THE SHEETS.
//
// appearance_test.go holds the rebuild and the sheets it brings back. These
// hold the rest of the window: the rasters Fyne keeps by name, the title bar
// Windows draws, the native reading panes' restore rule and the study popup
// Android floats over them, and the reader's place — the caret in a page
// field, the Books grid's scroll. Each was a way the window stayed partly in
// the palette just left, or lost what the reader had, after a switch; each
// test fails with its fix taken out.

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// --- the rasters Fyne keeps by name --------------------------------------------

// switchThemeUnder does what a system switch does to the tree on screen before
// the app's rebuild runs: Fyne's settings apply clears its caches and then
// refreshes the OLD tree, which rasterises every SVG in it again, into the
// cache, under its name. The test driver's settings apply is the same pair
// (ResetThemeCaches, then ApplySettings over every window).
func switchThemeUnder(app fyne.App, v fyne.ThemeVariant) {
	app.Settings().SetTheme(forcedVariant{Theme: &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}, v: v})
}

// sampleNear counts the pixels of r near c.
func sampleNear(img image.Image, c color.NRGBA, r image.Rectangle) int {
	return countColorNear(img, c, r.Min.X, r.Min.Y, r.Dx(), r.Dy())
}

// THE STYLED PANE'S NOTE CARD IS DRAWN IN THE NEW PALETTE. The card is a
// generated SVG with the palette in its bytes; under a fixed name the rebuilt
// card, same size as the old one, was handed the raster the settings apply
// had just made from the OLD card — a parchment card carrying the dark
// palette's pale ink. Windows and Linux, most visibly in focus mode, where no
// widget's theme override clears the cache by accident. Mutation guarded: the
// fixed name "note-bubble.svg" back (the rebuilt card's centre is the light
// SurfaceAlt, 0 dark pixels).
func TestTheStyledNoteCardIsDrawnInTheNewPalette(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	switchThemeUnder(app, light)
	st, _ := styledNoteFixture(t, []int{2}, []string{"the words the band is for"})

	pane := func(pal palette) (*styledReadingPane, fyne.CanvasObject) {
		p := newStyledReadingPane(st, st.Bible.GetChapter("Ruth", 1))
		p.pal = pal
		return p, container.NewStack(canvas.NewRectangle(pal.Surface), p)
	}
	old, oldTree := pane(lightPalette)
	w := test.NewWindow(oldTree)
	defer w.Close()
	w.Resize(fyne.NewSize(520, 420))
	old.Refresh()
	if !old.noteGeom.present || old.noteGeom.pill {
		t.Fatal("control: the fixture must open the note as a card, not a pill")
	}
	inside := func(p *styledReadingPane) image.Rectangle {
		g := p.noteGeom
		return image.Rect(int(g.card.X)+6, int(g.card.Y)+6, int(g.card.X+g.card.W)-6, int(g.card.Y+g.cardH)-6)
	}
	if n := sampleNear(w.Canvas().Capture(), lightPalette.SurfaceAlt, inside(old)); n < 40 {
		t.Fatalf("control: the light card barely shows its own fill (%d px)", n)
	}

	switchThemeUnder(app, dark)
	rebuilt, tree := pane(darkPalette)
	w.SetContent(tree)
	rebuilt.Refresh()
	if old.noteGeom.card != rebuilt.noteGeom.card || old.noteGeom.cardH != rebuilt.noteGeom.cardH {
		t.Fatal("control: the rebuilt card must be the old one's size, which is what the cache keys on")
	}
	img := w.Canvas().Capture()
	if n := sampleNear(img, darkPalette.SurfaceAlt, inside(rebuilt)); n < 40 {
		t.Errorf("the rebuilt card is not in the dark palette's fill (%d px of it)", n)
	}
	if n := sampleNear(img, lightPalette.SurfaceAlt, inside(rebuilt)); n > 0 {
		t.Errorf("the rebuilt card still shows the light palette's fill (%d px): the old raster", n)
	}
}

// THE NOTE BUBBLE'S TAIL IS DRAWN IN THE NEW PALETTE, however it is hosted.
// No shipped screen showed a stale tail: the notes browser wraps every row in
// a theme override of its own, whose rasters Fyne keys under a scope no
// rebuilt row shares, and the Fyne banner that hangs a bare one is off on
// every shipped pane. The tail is always the same size, though, so a bubble
// built outside such a scope would be handed the old raster every time — the
// case built here, bare, and the reason the tail is named after its look like
// the card. Mutation guarded: the fixed name "note-tail.svg" back (the rebuilt
// tail's lid is the light SurfaceAlt).
func TestTheNoteTailIsDrawnInTheNewPalette(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	switchThemeUnder(app, light)

	bubble := func(pal palette) (fyne.CanvasObject, *canvas.Image) {
		b := noteBubbleAround(widget.NewLabel("a note someone shared"), pal, 8)
		var tail *canvas.Image
		walkTree(b, func(o fyne.CanvasObject) {
			if img, ok := o.(*canvas.Image); ok && tail == nil {
				tail = img
			}
		})
		if tail == nil {
			t.Fatal("the bubble has no tail image")
		}
		return container.NewStack(canvas.NewRectangle(pal.Surface), container.NewVBox(b)), tail
	}
	// The lid: the fill strip over the card's bottom border, and the top of
	// the triangle under it, across the tail's middle.
	lid := func(tail *canvas.Image) image.Rectangle {
		p := app.Driver().AbsolutePositionForObject(tail)
		x, y := int(p.X+tail.Size().Width/2), int(p.Y)
		return image.Rect(x-3, y, x+3, y+noteTailLidOverlap+3)
	}

	oldTree, oldTail := bubble(lightPalette)
	w := test.NewWindow(oldTree)
	defer w.Close()
	w.Resize(fyne.NewSize(320, 200))
	if n := sampleNear(w.Canvas().Capture(), lightPalette.SurfaceAlt, lid(oldTail)); n < 12 {
		t.Fatalf("control: the light tail's lid barely shows its fill (%d px)", n)
	}

	switchThemeUnder(app, dark)
	tree, tail := bubble(darkPalette)
	w.SetContent(tree)
	img := w.Canvas().Capture()
	if n := sampleNear(img, darkPalette.SurfaceAlt, lid(tail)); n < 12 {
		t.Errorf("the rebuilt tail is not in the dark palette's fill (%d px of it)", n)
	}
	if n := sampleNear(img, lightPalette.SurfaceAlt, lid(tail)); n > 0 {
		t.Errorf("the rebuilt tail still shows the light palette's fill (%d px): the old raster", n)
	}
}

// Every generated SVG with literal colours is named after its look, and never
// after its size: two resources that draw differently at the same size cannot
// share a name, and a card that follows the window's width through a resize
// keeps ONE name, so Fyne's cache keeps one raster per palette instead of one
// per width the drag passed through. The pixel tests above prove the palette
// half; this holds the rest. Mutations guarded: a fixed name (the palettes
// share it), the shape left out (a tailed card and a taller flat one, which
// can come to one pixel size, share it), and the size in the name (every
// width its own name).
func TestGeneratedNoteSVGsAreNamedAfterTheirLookNotTheirSize(t *testing.T) {
	a, b := noteTailSVG(lightPalette.SurfaceAlt, lightPalette.Border), noteTailSVG(darkPalette.SurfaceAlt, darkPalette.Border)
	if a.Name() == b.Name() {
		t.Errorf("the light and dark tails share the name %q", a.Name())
	}
	if again := noteTailSVG(lightPalette.SurfaceAlt, lightPalette.Border); again.Name() != a.Name() {
		t.Error("the same tail must keep one name, or every row rasterises its own copy")
	}
	c := noteBubblePathSVG(200, 80, lightPalette.SurfaceAlt, lightPalette.Border, true)
	faded := lightPalette.SurfaceAlt
	faded.A = 0x80
	for name, other := range map[string]fyne.Resource{
		"the dark palette": noteBubblePathSVG(200, 80, darkPalette.SurfaceAlt, darkPalette.Border, true),
		"no tail, and as tall as the tailed card": noteBubblePathSVG(200, 80+noteTailDepth,
			lightPalette.SurfaceAlt, lightPalette.Border, false),
		"another fill alpha": noteBubblePathSVG(200, 80, faded, lightPalette.Border, true),
		"another stroke":     noteBubblePathSVG(200, 80, lightPalette.SurfaceAlt, darkPalette.Border, true),
	} {
		if other.Name() == c.Name() {
			t.Errorf("%s: two bubbles that draw differently at one size share the name %q", name, c.Name())
		}
	}
	for _, size := range [][2]float32{{201, 80}, {577.5, 79}, {200, 140}} {
		if r := noteBubblePathSVG(size[0], size[1], lightPalette.SurfaceAlt, lightPalette.Border, true); r.Name() != c.Name() {
			t.Errorf("a %vx%v card is named %q and a 200x80 one %q: the size is Fyne's cache's to key on, "+
				"and a name per width keeps a raster per width through a resize", size[0], size[1], r.Name(), c.Name())
		}
	}
	for _, r := range []fyne.Resource{a, b, c} {
		if !strings.HasSuffix(r.Name(), ".svg") {
			t.Errorf("%q: Fyne tells an SVG by its name's suffix", r.Name())
		}
	}
}

// --- the Windows title bar -----------------------------------------------------

// THE TITLE BAR FOLLOWS THE VARIANT THE CONTENT WAS BUILT IN. Fyne sets the
// window's immersive dark mode once, at creation; the app sends it again
// whenever a rebuild moves the variant — whatever asked for that rebuild — and
// never for a rebuild that leaves the variant where it was. Mutations guarded:
// the call made only when the appearance gate orders a rebuild (the rebuild
// that got in first, below, leaves the frame in the old mode for good), made
// on every rebuild (a plain rebuild sends the frame again), and a value that
// ignores the variant.
func TestTheTitleBarFollowsTheVariantTheContentWasBuiltIn(t *testing.T) {
	for _, tc := range []struct {
		v    fyne.ThemeVariant
		want int32
	}{{dark, 1}, {light, 0}, {2 /* no preference */, 0}} {
		if got := titleBarDarkMode(tc.v); got != tc.want {
			t.Errorf("variant %v sends %d for DWMWA_USE_IMMERSIVE_DARK_MODE, want %d", tc.v, got, tc.want)
		}
	}

	record := func(t *testing.T, h *appearanceHarness) *[]fyne.ThemeVariant {
		var sent []fyne.ThemeVariant
		prev := syncTitleBar
		syncTitleBar = func(w fyne.Window, v fyne.ThemeVariant) {
			if w != h.state.window {
				t.Error("the title bar was synced on another window")
			}
			sent = append(sent, v)
		}
		t.Cleanup(func() { syncTitleBar = prev })
		return &sent
	}

	t.Run("desktop", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		sent := record(t, h)
		h.flip()
		h.flip()
		if got := *sent; len(got) != 2 || got[0] != light || got[1] != dark {
			t.Fatalf("two changes sent %v, want [light dark]: the frame must follow each rebuild", got)
		}
		rebuildWindow(h.state) // focus mode, a download landing, a tab switch
		h.state.CurrentTab = 1
		rebuildWindow(h.state)
		if len(*sent) != 2 {
			t.Errorf("a rebuild in the same variant sent the frame %v; it is already there", (*sent)[2:])
		}
		observeAppearance(h.state, appearanceChanged) // heard, but nothing changed
		if len(*sent) != 2 {
			t.Error("a change that is no change sent the frame again")
		}
	})
	// The listener reaches the gate through a goroutine and fyne.Do, after
	// Fyne has moved the variant; a tab tapped in that moment rebuilds first,
	// in the new variant, and the gate's closure then finds nothing to do.
	t.Run("another rebuild gets there first", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		sent := record(t, h)
		h.variant = light // Fyne's settings apply has run; the closure has not
		h.state.CurrentTab = 1
		rebuildWindow(h.state) // the tab tap, handled first
		observeAppearance(h.state, appearanceChanged)
		if got := *sent; len(got) != 1 || got[0] != light {
			t.Errorf("the frame was sent %v, want [light]: the content is light, and the gate "+
				"found nothing left to rebuild", got)
		}
	})
	t.Run("the snapshot round trip", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		sent := record(t, h)
		for _, step := range []string{"X", "V1", "C1", "V2", "C2", "E"} {
			h.step(step, map[string]fyne.ThemeVariant{"V1": light, "V2": dark})
		}
		if len(*sent) != 0 {
			t.Errorf("the round trip sent the frame %v", *sent)
		}
	})
}

// THE CAPTION IS REPAINTED, AND LEFT DRAWN AS IT WAS. Windows 10 stores the
// attribute and repaints the caption only when its activation changes, so the
// Windows file sends a WM_NCACTIVATE pair after the attribute: the opposite of
// the caption's real state, then the real state. The decision is host-tested
// here and its use held at the source, as the house holds every native call it
// cannot run (the file is built for Windows only). Mutations guarded: the pair
// sent in the wrong order (a focused window left with a grey caption), the
// pair not sent, sent before the attribute, or its state read from the
// thread-local active window.
func TestTheTitleBarCaptionIsRepaintedAsItWasDrawn(t *testing.T) {
	for _, drawnActive := range []bool{true, false} {
		pair := titleBarRepaint(drawnActive)
		want := uintptr(0)
		if drawnActive {
			want = 1
		}
		if pair[1] != want {
			t.Errorf("drawn active=%v: the pair ends on %d, want %d — the caption must be left as it was drawn",
				drawnActive, pair[1], want)
		}
		if pair[0] == pair[1] {
			t.Errorf("drawn active=%v: the pair %v changes nothing, and nothing is repainted", drawnActive, pair)
		}
	}

	src := readSourceFile(t, "title_bar_windows.go")
	if !strings.HasPrefix(src, "//go:build windows\n") {
		t.Fatal("title_bar_windows.go must be built for Windows alone")
	}
	body := funcBody(t, src, "syncNativeTitleBar")
	steps := []string{
		"procDwmSetWindowAttribute.Call(c.HWND, dwmwaUseImmersiveDarkMode,",
		"return\n\t\t}",
		"procGetForegroundWindow.Call()",
		"titleBarRepaint(fg == c.HWND)",
		"procSendMessageW.Call(c.HWND, wmNCActivate, wParam, 0)",
	}
	if !inSequence(body, steps...) {
		t.Errorf("syncNativeTitleBar must set the attribute, stop on a failure, then send the "+
			"repaint pair for the caption's real state:\n%s", body)
	}
	// CONTROL: a repaint sent before the attribute is set repaints the old mode.
	if inSequence(swapOnce(body, "procDwmSetWindowAttribute.Call(", "procSendMessageW.Call("), steps...) {
		t.Fatal("control: the order check passes a repaint sent before the attribute")
	}
	if strings.Contains(body, "GetActiveWindow") {
		t.Error("the caption's state must be read as the foreground window, which any thread can ask")
	}
	if !strings.Contains(src, "wmNCActivate   = 0x0086") && !strings.Contains(src, "wmNCActivate = 0x0086") {
		t.Error("WM_NCACTIVATE is 0x0086")
	}
}

// --- the native reading panes -----------------------------------------------------

// showReadingOverlayClosure is the source of a pane's showReadingOverlay
// closure, from its assignment to its closing brace one tab in.
func showReadingOverlayClosure(t *testing.T, src, file string) string {
	t.Helper()
	const head = "state.showReadingOverlay = func() {"
	i := strings.Index(src, head)
	if i < 0 {
		t.Fatalf("%s no longer assigns state.showReadingOverlay", file)
	}
	j := strings.Index(src[i:], "\n\t}\n")
	if j < 0 {
		t.Fatalf("%s: the closure has no stable end", file)
	}
	return src[i : i+j]
}

// EVERY NATIVE PANE RESTORES BY THE SAME RULE. The closure a sheet's close and
// every rebuild's drain run shows the pane exactly when overlayShouldShow
// says so. macOS asked !IsSearching, which said "show" on the Books and
// Search tabs: closing a sheet there put the NSTextView over the tab, holding
// the chapter it last had — after a switch made on that tab, in the palette
// just left. Mutations guarded: the macOS closure back on !IsSearching (the
// source check, whose control is that very closure), the helper asking
// anything but overlayShouldShow (the Books and Search rows), and the helper
// dropping the deferred-rebuild consume.
func TestEveryNativePaneRestoresByOverlayShouldShow(t *testing.T) {
	st := sampleState()
	for _, tc := range []struct {
		name      string
		tab       int
		searching bool
		full      bool
		want      bool
	}{
		{"Read", 0, false, false, true},
		{"Read, with results", 0, true, false, false},
		{"Books", 1, false, false, false},
		{"Search", 2, false, false, false},
		{"focus mode", 1, false, true, true},
	} {
		st.CurrentTab, st.IsSearching, st.IsFullScreen = tc.tab, tc.searching, tc.full
		var got []string
		restoreNativeReadingOverlay(st, func() { got = append(got, "unsuppress") },
			func(v bool) {
				if v {
					got = append(got, "show")
				} else {
					got = append(got, "hide")
				}
			})
		want := "hide"
		if tc.want {
			want = "show"
		}
		if len(got) != 2 || got[0] != "unsuppress" || got[1] != want {
			t.Errorf("%s: the restore did %v, want [unsuppress %s]", tc.name, got, want)
		}
		if tc.name == "Books" && st.IsSearching {
			t.Fatal("control: on Books with no search the old rule (!IsSearching) says show")
		}
	}

	t.Run("it still consumes a deferred rebuild", func(t *testing.T) {
		h := newAppearanceHarness(t, false)
		h.state.fullRebuildDeferred = true
		gen := windowRebuildGen
		restoreNativeReadingOverlay(h.state, func() {}, func(bool) {})
		if windowRebuildGen != gen+1 || h.state.fullRebuildDeferred {
			t.Error("the sheet's close must still run the rebuild a data swap deferred to spare it")
		}
	})

	t.Run("each pane's closure is the helper", func(t *testing.T) {
		check := func(body string) bool {
			return strings.Contains(body, "restoreNativeReadingOverlay(state,") &&
				!strings.Contains(body, "IsSearching")
		}
		// CONTROL: the closure macOS had, which this check must refuse.
		old := "state.showReadingOverlay = func() {\n\t\tC.bibleTextMacTVUnsuppress()\n" +
			"\t\tsetReadingOverlayVisible(!state.IsSearching)\n\t\tconsumeDeferredFullRebuild(state)"
		if check(old) {
			t.Fatal("control: the check passes the closure that showed verses over the Books tab")
		}
		for _, file := range []string{"reading_macos.go", "reading_ios.go", "reading_android.go"} {
			if body := showReadingOverlayClosure(t, readSourceFile(t, file), file); !check(body) {
				t.Errorf("%s: the restore closure is not restoreNativeReadingOverlay:\n%s", file, body)
			}
		}
	})
}

// A PANE HOLDING THE OTHER PALETTE'S CHAPTER IS NOT SHOWN. After a switch made
// on the Books or Search tab nothing re-pushes the macOS chapter until the
// reader goes back to Read; whatever asks to show the pane before then is
// refused, and the Read build's own push shows it. Mutations guarded: the
// decision ignoring the palette, the gate missing from
// setReadingOverlayVisible, and the push not recording its palette.
func TestAPaneHoldingTheOtherPaletteIsNotShown(t *testing.T) {
	for _, tc := range []struct {
		pushed, pushedDark, nowDark, stale bool
	}{
		{false, false, true, false}, // nothing pushed yet: nothing to be stale
		{false, true, false, false},
		{true, false, false, false},
		{true, true, true, false},
		{true, false, true, true},
		{true, true, false, true},
	} {
		if got := nativePaneStale(tc.pushed, tc.pushedDark, tc.nowDark); got != tc.stale {
			t.Errorf("pushed=%v pushedDark=%v nowDark=%v: stale=%v, want %v",
				tc.pushed, tc.pushedDark, tc.nowDark, got, tc.stale)
		}
	}

	src := readSourceFile(t, "reading_macos.go")
	gate := funcBody(t, src, "setReadingOverlayVisible")
	gateSteps := []string{"nativePaneStale(macPanePushed, macPaneDark, isDark())", "visible = false", "C.bibleTextMacTVShow()"}
	if !inSequence(gate, gateSteps...) {
		t.Errorf("setReadingOverlayVisible must refuse a stale pane before it shows one:\n%s", gate)
	}
	if inSequence(swapOnce(gate, "visible = false", "C.bibleTextMacTVShow()"), gateSteps...) {
		t.Fatal("control: the order check passes a show before the refusal")
	}
	push := funcBody(t, src, "newMacReadingHost")
	pushSteps := []string{"lastPushedBodyFP = body", "macPanePushed, macPaneDark = true, isDark()", "C.bibleTextMacTVSetHTML(c)"}
	if !inSequence(push, pushSteps...) {
		t.Error("the macOS push must record the palette it pushes in, with the fingerprint")
	}
	if inSequence(strings.Replace(push, "macPanePushed, macPaneDark = true, isDark()", "", 1), pushSteps...) {
		t.Fatal("control: the check passes a push that records nothing")
	}
}

// cFuncBody is the source of one C function in a cgo preamble, from its
// signature line to the closing brace in column 1.
func cFuncBody(t *testing.T, src, signature string) string {
	t.Helper()
	i := strings.Index(src, signature)
	if i < 0 {
		t.Fatalf("cannot find %q", signature)
	}
	j := strings.Index(src[i:], "\n}\n")
	if j < 0 {
		t.Fatalf("%q has no closing brace in column 1", signature)
	}
	return src[i : i+j]
}

// THE macOS PANE'S PLACE IS READ FROM A HIDDEN PANE TOO. A same-chapter
// re-render — every light/dark switch on Read — captures the reader's place
// before the import snaps the view to the top. The capture read the text
// view's visibleRect, which AppKit gives as NSZeroRect for a hidden view, and
// the pane is hidden with any sheet up and on the Books and Search tabs: a
// switch made with Settings open, or on Books before going back to Read,
// captured "the top" and the import pinned the reader to verse 1 (a probe of
// a hidden NSScrollView scrolled to 500: visibleRect 0, clip bounds 500). The
// clip view's bounds survive hiding. Held at the source, as the house holds
// the pane's other native contracts. Mutation guarded: the capture back on
// visibleRect (the control below is that very body).
func TestMacCaptureReadsTheClipViewAHiddenPaneKeeps(t *testing.T) {
	src := readSourceFile(t, "reading_macos.go")
	body := cFuncBody(t, src, "BTAnchor bibleTextMacCaptureAnchor(void) {")
	steps := []string{
		"NSClipView *clip = gScroll.contentView;",
		"CGFloat offY = clip.bounds.origin.y - tv.frame.origin.y;",
		"if (offY <= 0.5) return;",
		"CGFloat viewH = clip.bounds.size.height;",
	}
	check := func(b string) bool { return inSequence(b, steps...) && !strings.Contains(b, "tv.visibleRect") }
	if !check(body) {
		t.Errorf("the macOS capture must read the scroll from the clip view, never the text view's "+
			"visibleRect, which a hidden pane answers as zero:\n%s", body)
	}
	// CONTROL: the capture as it was.
	old := "out.ok = 1;\n        CGFloat offY = tv.visibleRect.origin.y;\n        if (offY <= 0.5) return;\n" +
		"        CGFloat viewH = tv.visibleRect.size.height;"
	if check(old) {
		t.Fatal("control: the check passes the capture that read a hidden pane as the top")
	}
}

// ANDROID'S STUDY POPUP DOES NOT OUTLIVE ITS PALETTE. Explain / Analyze
// context / Analyze translation is a PopupWindow of its own, filled from the
// palette when it opened; a switch re-rendered the page under it and left it
// floating in the old one. setStyle now closes it when the text or paper
// colour moves, before the palette does, and hide and suppress close it with
// the Dialog. Held at the source, as the house holds every Java contract (the
// host cannot run it; scripts/check-android-java.sh compiles it). Mutations
// guarded: the decision ignoring either colour, the dismissal after the
// palette moves, the popup never held, and hide/suppress leaving it up.
func TestAndroidStudyPopupDoesNotOutliveItsPalette(t *testing.T) {
	const path = "android/BtBridge.java"
	decision := javaMethodSource(t, path, "static boolean studyPopupStale(int oldText, int oldPaper, int newText, int newPaper)")
	if !strings.Contains(decision, "return oldText != newText || oldPaper != newPaper;") {
		t.Errorf("the dismissal decision must close the popup when EITHER colour moves:\n%s", decision)
	}

	style := javaMethodSource(t, path, "public static void setStyle(")
	styleSteps := []string{
		"studyPopupStale(lastTextColor, lastPaperColor, textColor, paperColor)",
		"dismissStudyPopup();",
		"lastTextColor = textColor;",
		"lastPaperColor = paperColor;",
	}
	if !inSequence(style, styleSteps...) {
		t.Errorf("setStyle must close a stale study popup BEFORE the palette moves:\n%s", style)
	}
	// CONTROL: the decision taken after the move compares the new palette with
	// itself and never fires; the order check must refuse it.
	if inSequence(swapOnce(style, "dismissStudyPopup();", "lastTextColor = textColor;"), styleSteps...) {
		t.Fatal("control: the order check passes a dismissal after the palette moved")
	}

	show := javaMethodSource(t, path, "private static void showStudyPopup(")
	if !inSequence(show, "dismissStudyPopup();", "studyPopup = pw;", "setOnDismissListener", "pw.showAtLocation(") {
		t.Error("showStudyPopup must hold the popup it shows, and let it go when it closes")
	}
	dismiss := javaMethodSource(t, path, "private static void dismissStudyPopup()")
	if !inSequence(dismiss, "studyPopup = null;", ".dismiss()") {
		t.Error("dismissStudyPopup must drop its hold and close the popup")
	}
	for _, sig := range []string{"public static void hide()", "public static void suppress()"} {
		if body := javaMethodSource(t, path, sig); !strings.Contains(body, "dismissStudyPopup();") {
			t.Errorf("%s leaves the study popup up over what replaces the page", sig)
		}
	}
}

// --- the reader's place -----------------------------------------------------------

// pageEntry is the registered page field of a kind, as the tab built it.
func pageEntry(t *testing.T, state *AppState, f pageField) *widget.Entry {
	t.Helper()
	o, ok := state.pageFields[f]
	if !ok {
		t.Fatalf("the tab registered no page field %d", f)
	}
	e := entryOf(o)
	if e == nil {
		t.Fatalf("page field %d is not an entry", f)
	}
	return e
}

// softKeyboard sets what the phone's keyboard last reported for one test.
func softKeyboard(t *testing.T, shown bool) {
	t.Helper()
	prev := softKeyboardShown
	softKeyboardShown = shown
	t.Cleanup(func() { softKeyboardShown = prev })
}

// THE PAGE'S CARET IS DROPPED BEFORE A LIGHT/DARK REBUILD, AND PUT BACK. With
// no sheet up the rebuild used to leave the field that had the caret
// unfocused and untold: a phone's keyboard stayed up typing into nothing, and
// on desktop the keystrokes went nowhere. On a phone it comes back only if
// its keyboard was up: focus raises the keyboard there, and a reader who had
// put it away (Done, Back) must not see it rise again over the rebuilt page.
// Mutations guarded: the unfocus still gated on a sheet (the page's field is
// never told), no restore (the rebuilt field has no caret), the restore not
// carrying the text a field held ahead of state (the Find question typed but
// not asked is lost), and the restore ignoring the keyboard (a phone's put-away
// keyboard comes back).
func TestAppearanceChangeDropsThePageCaretAndPutsItBack(t *testing.T) {
	t.Run("the page's field is told", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		cnv := h.state.window.Canvas()
		probe := &focusProbe{}
		probe.ExtendBaseWidget(probe)
		cnv.SetContent(container.NewStack(cnv.Content(), probe))
		cnv.Focus(probe)
		if cnv.Focused() != probe || cnv.Overlays().Top() != nil {
			t.Fatal("control: the probe must hold the caret, with no sheet up")
		}
		h.flip()
		if probe.lost != 1 {
			t.Errorf("the page's field was told it lost the caret %d times, want 1", probe.lost)
		}
	})
	for _, tc := range []struct {
		name  string
		tab   int
		setup func(*AppState)
		field pageField
		typed string // typed but not yet in state; "" types nothing
	}{
		{"Search", 2, func(*AppState) {}, pageFieldSearch, ""},
		{"Find", 2, func(s *AppState) { s.aiSearchMode = true }, pageFieldFind, "the fruit of"},
		{"the notes filter", 2, func(s *AppState) { setNotesEnabled(true); setNotesMode(s, true) }, pageFieldNotes, "grace"},
		{"the Books filter", 1, func(*AppState) {}, pageFieldBooks, "Jo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newAppearanceHarness(t, true)
			softKeyboard(t, true) // a phone, typing
			h.state.aiKeys = newKeyStoreWith(newFakePrefs())
			h.state.aiKeys.setAPIKey(defaultProviderID, "test-key")
			tc.setup(h.state)
			t.Cleanup(func() { setNotesMode(h.state, false) })
			h.state.CurrentTab = tc.tab
			rebuildWindow(h.state)
			cnv := h.state.window.Canvas()
			old := pageEntry(t, h.state, tc.field)
			if tc.typed != "" {
				old.SetText(tc.typed)
			}
			old.CursorColumn = len([]rune(old.Text))
			cnv.Focus(h.state.pageFields[tc.field]) // the field as laid out, not the Entry inside it
			if cnv.Focused() != fyne.Focusable(h.state.pageFields[tc.field]) {
				t.Fatal("control: the field must hold the caret")
			}
			col := old.CursorColumn
			h.flip()
			twin := pageEntry(t, h.state, tc.field)
			if twin == old {
				t.Fatal("control: the rebuild must build the field new")
			}
			if cnv.Focused() != fyne.Focusable(h.state.pageFields[tc.field]) {
				t.Errorf("the caret must be back in the rebuilt %s field; focused %v", tc.name, cnv.Focused())
			}
			if tc.typed != "" && twin.Text != tc.typed {
				t.Errorf("the rebuilt %s field holds %q, want what was typed, %q", tc.name, twin.Text, tc.typed)
			}
			if twin.CursorColumn != col {
				t.Errorf("the caret is at column %d, want %d", twin.CursorColumn, col)
			}
		})
	}
	for _, tc := range []struct {
		name          string
		mobile, shown bool
		back          bool
	}{
		{"a phone whose keyboard was put away", true, false, false},
		{"desktop, which has no soft keyboard", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newAppearanceHarness(t, tc.mobile)
			softKeyboard(t, tc.shown)
			h.state.aiKeys = newKeyStoreWith(newFakePrefs())
			h.state.aiKeys.setAPIKey(defaultProviderID, "test-key")
			h.state.aiSearchMode = true
			h.state.CurrentTab = 2
			rebuildWindow(h.state)
			cnv := h.state.window.Canvas()
			old := pageEntry(t, h.state, pageFieldFind)
			old.SetText("the fruit of") // typed, not asked: in the field alone
			cnv.Focus(h.state.pageFields[pageFieldFind])
			if cnv.Focused() == nil {
				t.Fatal("control: the Find field must hold the caret")
			}
			h.flip()
			twin := pageEntry(t, h.state, pageFieldFind)
			if twin == old {
				t.Fatal("control: the rebuild must build the field new")
			}
			if twin.Text != "the fruit of" {
				t.Errorf("the rebuilt field holds %q: what was typed must come back, caret or not", twin.Text)
			}
			if got := cnv.Focused() != nil; got != tc.back {
				t.Errorf("the caret came back: %v, want %v (focused %v)", got, tc.back, cnv.Focused())
			}
		})
	}
	t.Run("no field, no caret", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		h.state.CurrentTab = 2
		rebuildWindow(h.state)
		h.flip()
		if f := h.state.window.Canvas().Focused(); f != nil {
			t.Errorf("a page nobody was typing in must not come back with the caret (a keyboard would rise unasked): %v", f)
		}
	})
}

// booksScroll is the Books tab's grid scroll.
func booksScroll(t *testing.T, state *AppState) *container.Scroll {
	t.Helper()
	var s *container.Scroll
	walkTree(state.window.Content(), func(o fyne.CanvasObject) {
		if sc, ok := o.(*container.Scroll); ok && s == nil {
			s = sc
		}
	})
	if s == nil {
		t.Fatal("the Books tab has no scroll")
	}
	return s
}

// THE BOOKS GRID KEEPS ITS PLACE ACROSS A REBUILD. The tab is built new by
// every rebuild, and a switch made halfway down the canon put the reader back
// at Genesis. Mutation guarded: the offset not restored (the rebuilt grid is at
// the top).
func TestTheBooksGridKeepsItsPlaceAcrossARebuild(t *testing.T) {
	h := newAppearanceHarness(t, false)
	h.state.window.Resize(fyne.NewSize(420, 480))
	h.state.CurrentTab = 1
	rebuildWindow(h.state)
	s := booksScroll(t, h.state)
	if s.Content.MinSize().Height <= s.Size().Height+200 {
		t.Fatalf("control: the canon must be taller than the pane (content %v, pane %v)", s.Content.MinSize(), s.Size())
	}
	s.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -180)}) // the reader's wheel or finger
	if h.state.booksScrollY != 180 {
		t.Fatalf("control: scrolling must be recorded, got %v", h.state.booksScrollY)
	}
	h.flip()
	again := booksScroll(t, h.state)
	if again == s {
		t.Fatal("control: the rebuild must build the grid new")
	}
	if again.Offset.Y != 180 {
		t.Errorf("the rebuilt grid is at %v, want the reader's 180", again.Offset.Y)
	}
}
