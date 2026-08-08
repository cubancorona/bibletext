package bibletext

// Selection-layer tests for the styled reading pane (milestone 3): geometry
// round-trips, simulated drags, copy fidelity (authored poem breaks kept,
// width wraps flattened), keyboard shortcuts, and the study menu's verbs.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
)

// fakeClipboard is a minimal fyne.Clipboard for copy assertions.
type fakeClipboard struct{ content string }

func (f *fakeClipboard) Content() string     { return f.content }
func (f *fakeClipboard) SetContent(s string) { f.content = s }

func newTestPane(t *testing.T, st *AppState, width float32) *styledReadingPane {
	t.Helper()
	p := newStyledReadingPane(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))
	p.Resize(fyne.NewSize(width, 400))
	return p
}

// posForOffset builds a widget position pointing at the given model offset —
// the inverse of offsetAtPos, for driving simulated gestures.
func posForOffset(p *styledReadingPane, off int) fyne.Position {
	for li, ln := range p.lay.Lines {
		if off >= ln.StartOffset && off <= ln.EndOffset {
			return fyne.NewPos(p.xForOffset(li, off)+0.5, ln.Y+ln.H/2)
		}
	}
	last := p.lay.Lines[len(p.lay.Lines)-1]
	return fyne.NewPos(p.xForOffset(len(p.lay.Lines)-1, last.EndOffset), last.Y+last.H/2)
}

func TestStyledSelectOffsetRoundTrip(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	p := newTestPane(t, psalm23State(), 420)
	for _, ln := range p.lay.Lines {
		for off := ln.StartOffset; off <= ln.EndOffset; off++ {
			pos := posForOffset(p, off)
			got := p.offsetAtPos(pos)
			// Hit-testing at the exact glyph boundary may land one rune either
			// side under kerning; a 1-rune tolerance is faithful to real text
			// widgets.
			if got < off-1 || got > off+1 {
				t.Fatalf("round-trip: offset %d -> pos %v -> %d", off, pos, got)
			}
		}
	}
}

func TestStyledSelectDragSelects(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	p := newTestPane(t, st, 420)

	// Drag from the start of "shepherd;" (line 0) to the end of "want." (line 1).
	model := []rune(p.lay.Text)
	startIdx := strings.Index(p.lay.Text, "shepherd;")
	endIdx := strings.Index(p.lay.Text, "want.") + len("want.")
	start := len([]rune(p.lay.Text[:startIdx]))
	end := len([]rune(p.lay.Text[:endIdx]))
	_ = model

	p.MouseDown(&desktop.MouseEvent{
		PointEvent: fyne.PointEvent{Position: posForOffset(p, start)},
		Button:     desktop.MouseButtonPrimary,
	})
	p.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: posForOffset(p, end)}})
	p.DragEnd()

	got := p.selectedRaw()
	want := "shepherd;\nI shall not want."
	if got != want {
		t.Errorf("drag selection:\n got %q\nwant %q", got, want)
	}

	// The selection paints spans on both poem lines.
	spans := p.selectionSpans()
	if len(spans) != 2 {
		t.Errorf("selection spans = %d, want 2 (two poem lines)", len(spans))
	}
}

func TestStyledSelectCopyFidelity(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// Poetry: authored breaks survive the clipboard.
	st := psalm23State()
	p := newTestPane(t, st, 420)
	clip := &fakeClipboard{}
	p.clipboard = clip
	p.selectAll()
	p.copyToClipboard()
	want := superscriptNumber(1) + " The LORD is my shepherd;\n" +
		"I shall not want.\n" +
		superscriptNumber(2) + " He makes me lie down in green pastures;\n" +
		"He leads me beside quiet waters."
	if clip.content != want {
		t.Errorf("poetry copy:\n got %q\nwant %q", clip.content, want)
	}

	// Prose at a narrow width: width wraps flatten to spaces, paragraph
	// boundaries stay blank lines.
	st2 := acts4ShareState()
	p2 := newTestPane(t, st2, 260)
	if len(p2.lay.Lines) < 4 {
		t.Fatalf("narrow prose should wrap, got %d lines", len(p2.lay.Lines))
	}
	clip2 := &fakeClipboard{}
	p2.clipboard = clip2
	p2.selectAll()
	p2.copyToClipboard()
	if strings.Contains(strings.ReplaceAll(clip2.content, "\n\n", "¶"), "\n") {
		t.Errorf("prose copy must flatten width wraps (only paragraph blanks allowed):\n%q", clip2.content)
	}
	if !strings.Contains(clip2.content, "God.") || !strings.Contains(clip2.content, "heard.") {
		t.Errorf("prose copy lost content: %q", clip2.content)
	}
}

func TestStyledSelectShortcutsAndEscape(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	p := newTestPane(t, psalm23State(), 420)
	clip := &fakeClipboard{}
	p.clipboard = clip

	p.TypedShortcut(&fyne.ShortcutSelectAll{})
	if p.selStart != 0 || p.selEnd != len([]rune(p.lay.Text)) {
		t.Fatalf("select-all = [%d,%d]", p.selStart, p.selEnd)
	}
	p.TypedShortcut(&fyne.ShortcutCopy{})
	if clip.content == "" {
		t.Fatal("copy shortcut must fill the clipboard")
	}
	p.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	if p.selStart != -1 {
		t.Fatal("escape must clear the selection")
	}
}

func TestStyledSelectDoubleTapWord(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	p := newTestPane(t, psalm23State(), 420)
	idx := strings.Index(p.lay.Text, "shepherd")
	off := len([]rune(p.lay.Text[:idx])) + 3 // inside the word
	p.DoubleTapped(&fyne.PointEvent{Position: posForOffset(p, off)})
	if got := p.selectedRaw(); got != "shepherd;" {
		t.Errorf("double-tap selection = %q, want %q", got, "shepherd;")
	}
}

func TestStyledSelectStudyMenuVerbs(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	p := newTestPane(t, psalm23State(), 420)

	// No selection: Copy disabled, no study verbs.
	m := p.studyMenu()
	if len(m.Items) != 2 || !m.Items[0].Disabled {
		t.Fatalf("empty-selection menu wrong: %d items, copy disabled=%v", len(m.Items), m.Items[0].Disabled)
	}

	p.selectAll()
	m = p.studyMenu()
	var labels []string
	for _, it := range m.Items {
		if !it.IsSeparator {
			labels = append(labels, it.Label)
		}
	}
	// AI is enabled by default → the full verb set in the shipping order.
	want := []string{"Copy", "Select all", "Study with AI", "Share", "Cross-references"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Errorf("menu = %v, want %v", labels, want)
	}
	// The Share submenu carries all three share verbs (citation, image, link).
	var share *fyne.MenuItem
	for _, it := range m.Items {
		if it.Label == "Share" {
			share = it
		}
	}
	if share == nil || share.ChildMenu == nil || len(share.ChildMenu.Items) != 3 {
		t.Fatalf("share submenu wrong: %+v", share)
	}
}

// TestStyledSelectVisual renders an active selection and checks accent-tinted
// pixels appear (and snapshots it, env-gated).
func TestStyledSelectVisual(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	st := psalm23State()
	p := newStyledReadingPane(st, st.Bible.GetChapter("Psalms", 23))
	w := test.NewWindow(p)
	defer w.Close()
	w.Resize(fyne.NewSize(420, 360))

	p.selectAll()
	img := w.Canvas().Capture()

	// The selection wash must tint a meaningful area.
	b := img.Bounds()
	tinted := 0
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			r, g, bl, _ := img.At(x, y).RGBA()
			// Accent-tinted: not the flat background, not pure text.
			if r != g || g != bl {
				tinted++
			}
		}
	}
	if tinted < 200 {
		t.Errorf("expected a visible selection wash, found %d tinted samples", tinted)
	}
	if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" {
		writePNG(t, filepath.Join(dir, "styled-pane-selection.png"), img)
	}
}
