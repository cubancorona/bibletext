package bibletext

// Tests for the Fyne reading pane's selection study menu (the Win/Linux twin of
// the native selection menus), the selection normalization it feeds the study
// actions, and the text-size setting finally applying to the Fyne pane.
// Untagged: chapterText is untagged, so these run on every platform's CI.

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func menuLabels(m *fyne.Menu) []string {
	labels := make([]string, 0, len(m.Items))
	for _, it := range m.Items {
		if it.IsSeparator {
			labels = append(labels, "---")
			continue
		}
		labels = append(labels, it.Label)
	}
	return labels
}

func TestPlainSelection(t *testing.T) {
	in := "¹⁶ For God so\nloved the world,\n\n¹⁷ For God did not send"
	want := "16 For God so loved the world,\n\n17 For God did not send"
	if got := plainSelection(in); got != want {
		t.Fatalf("plainSelection = %q, want %q", got, want)
	}
}

func TestSelectionMenuMirrorsNativeLayout(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	state := sampleState()
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	verses := state.Bible.GetChapter("John", 1)
	c := newChapterText(state, verses)

	// AI on (default): Copy, Select all, ---, Study with AI, Share, Cross-references.
	m := c.menuForSelection("For God so loved the world")
	want := []string{"Copy", "Select all", "---", "Study with AI", "Share", "Cross-references"}
	if got := menuLabels(m); len(got) != len(want) {
		t.Fatalf("AI-on menu = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("AI-on menu = %v, want %v", got, want)
			}
		}
	}
	// The AI submenu is the study verbs, in order, each dispatching its own
	// action string (read at the request seam, where the request parks);
	// Share carries all four forms — with citation, as image, as a link, and
	// as a link with a note.
	ai := m.Items[3]
	if ai.ChildMenu == nil || len(ai.ChildMenu.Items) != len(aiStudyVerbs) {
		t.Fatalf("Study with AI submenu wrong: %+v", ai.ChildMenu)
	}
	type menuStudyRequest struct {
		action  string
		release chan struct{}
		done    chan struct{}
	}
	requests := make(chan menuStudyRequest, 8)
	stop := make(chan struct{})
	prevRun := aiActionRun
	aiActionRun = func(ctx context.Context, _ *AppState, action, _, _ string, _ selSpan) (string, error) {
		release := make(chan struct{})
		done := make(chan struct{})
		defer close(done)
		requests <- menuStudyRequest{action: action, release: release, done: done}
		select {
		case <-ctx.Done():
			// The test driver runs completion on this goroutine. Let the
			// button finish its cancellation cleanup before returning.
			select {
			case <-release:
			case <-stop:
			}
			return "", ctx.Err()
		case <-stop:
			return "", context.Canceled
		}
	}
	state.window = app.NewWindow("menu")
	dismissPanel := func() bool {
		top := state.window.Canvas().Overlays().Top()
		if top == nil {
			return false
		}
		for _, label := range []string{"Close", "Cancel"} {
			if button := findTreeButton(top, label); button != nil {
				test.Tap(button)
				return true
			}
		}
		return false
	}
	attempted, observed := 0, 0
	var completions []chan struct{}
	t.Cleanup(func() {
		dismissPanel()
		close(stop)
		// A failing assertion can leave a request not yet observed at the seam.
		// Drain it before restoring the stub or closing the test window.
		deadline := time.NewTimer(2 * time.Second)
		defer deadline.Stop()
		for observed < attempted {
			select {
			case request := <-requests:
				observed++
				completions = append(completions, request.done)
			case <-deadline.C:
				t.Errorf("request cleanup observed %d of %d attempted requests", observed, attempted)
				aiActionRun = prevRun
				state.window.Close()
				return
			}
		}
		for _, done := range completions {
			<-done
		}
		aiActionRun = prevRun
		state.window.Close()
	})
	for i, v := range aiStudyVerbs {
		it := ai.ChildMenu.Items[i]
		if it.Label != v.Label {
			t.Errorf("study item %d is %q, want %q", i, it.Label, v.Label)
		}
		if v.Action != aiActionAsk {
			attempted++
		}
		it.Action()
		if v.Action == aiActionAsk {
			top := state.window.Canvas().Overlays().Top()
			if top == nil || !treeHasText(top, "Ask about this passage") {
				t.Fatal("Ask did not open the question composer")
			}
			var entry *searchKeyEntry
			var submit *widget.Button
			walkTree(top, func(o fyne.CanvasObject) {
				if e, ok := o.(*searchKeyEntry); ok {
					entry = e
				}
				if b, ok := o.(*widget.Button); ok && b.Text == "Ask" {
					submit = b
				}
			})
			if entry == nil || submit == nil {
				t.Fatal("question composer lacks field or Ask button")
			}
			entry.SetText("What does this passage mean?")
			attempted++
			submit.OnTapped()
		}
		select {
		case request := <-requests:
			observed++
			completions = append(completions, request.done)
			if request.action != v.Action {
				t.Errorf("%q dispatched %q, want %q", it.Label, request.action, v.Action)
			}
			if !dismissPanel() {
				t.Fatal("study panel has no Close or Cancel button")
			}
			close(request.release)
			<-request.done
		case <-time.After(2 * time.Second):
			t.Fatalf("%q never reached the request seam", it.Label)
		}
	}
	if share := m.Items[4]; share.ChildMenu == nil || len(share.ChildMenu.Items) != 4 {
		t.Fatalf("Share submenu wrong: %+v", share.ChildMenu)
	}

	// AI off ("None"): Cross-references takes the study slot, ahead of Share.
	state.aiKeys.setAIEnabled(false)
	m = c.menuForSelection("For God so loved the world")
	want = []string{"Copy", "Select all", "---", "Cross-references", "Share"}
	got := menuLabels(m)
	if len(got) != len(want) {
		t.Fatalf("AI-off menu = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AI-off menu = %v, want %v", got, want)
		}
	}

	// No selection: just Copy (disabled) + Select all — no study group.
	m = c.menuForSelection("")
	if got := menuLabels(m); len(got) != 2 || got[0] != "Copy" || got[1] != "Select all" {
		t.Fatalf("empty-selection menu = %v", got)
	}
	if !m.Items[0].Disabled {
		t.Fatal("Copy must be disabled with no selection")
	}
}

func TestReadingTextSizeApplies(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	state := sampleState()
	verses := state.Bible.GetChapter("John", 1)

	setReadingTextSizeID("normal")
	normal := newChapterText(state, verses)
	if diff := normal.textSize - theme.TextSize(); diff < -0.01 || diff > 0.01 {
		t.Fatalf("normal textSize = %v, want theme default %v", normal.textSize, theme.TextSize())
	}
	normal.rewrap(300)

	setReadingTextSizeID("xl")
	defer setReadingTextSizeID("normal")
	xl := newChapterText(state, verses)
	wantXL := theme.TextSize() * 1.3
	if diff := xl.textSize - wantXL; diff < -0.01 || diff > 0.01 {
		t.Fatalf("xl textSize = %v, want %v", xl.textSize, wantXL)
	}
	xl.rewrap(300)

	// Bigger text at the same width must wrap onto more lines — the visible
	// proof the setting reaches the pane (it changed nothing before).
	if xl.totalLines <= normal.totalLines {
		t.Fatalf("xl must wrap more lines than normal: xl=%d normal=%d", xl.totalLines, normal.totalLines)
	}
}
