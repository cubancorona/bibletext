package bibletext

// THE SEARCH TAB'S FIND SURVIVES A WINDOW REBUILD.
//
// Every rebuild builds the Search tab new, and the Find pane used to be drawn
// only by the calls that changed it — the submit, the landing, Cancel — into
// the results host of the build that made them. So a light/dark change (or a
// rotation, or a translation landing) mid-Find brought the tab back as the
// empty "Find passages by meaning" prompt: no spinner, no Cancel, and the
// answer, when it came, painted into the detached tree while the prompt
// stayed. An error card, and a finished "found nothing", came back as the
// prompt too, with no search running at all. The pane is now rendered from
// state (renderFind), and the landing repaints through the tab that is on the
// canvas (state.repaintFind).

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// heldFind is a Find stopped at the seam: its question, the screen's callback
// for the model its request reports, its completion, and whether anything
// abandoned it.
type heldFind struct {
	q         string
	onModel   func(aiModelInUse)
	done      func([]Verse, error)
	abandoned bool
}

// holdFinds routes every Find through the seam and holds it, so the test
// delivers its answer on the test's own goroutine, when it chooses.
func holdFinds(t *testing.T) *[]*heldFind {
	t.Helper()
	var held []*heldFind
	prev := startFind
	startFind = func(_ *AppState, q string, onModel func(aiModelInUse), done func([]Verse, error)) func() {
		f := &heldFind{q: q, onModel: onModel, done: done}
		held = append(held, f)
		return func() { f.abandoned = true }
	}
	t.Cleanup(func() { startFind = prev })
	return &held
}

// findTab is the appearance harness on the Search tab in Find mode, with a key.
func findTab(t *testing.T) (*appearanceHarness, *[]*heldFind) {
	t.Helper()
	held := holdFinds(t)
	h := newAppearanceHarness(t, true)
	h.state.aiKeys = newKeyStoreWith(newFakePrefs())
	h.state.aiKeys.setAPIKey(defaultProviderID, "test-key")
	h.state.aiSearchMode = true
	h.state.CurrentTab = 2
	rebuildWindow(h.state)
	return h, held
}

// ask submits a question through the tab's own Find field.
func (h *appearanceHarness) ask(q string) {
	h.t.Helper()
	e := pageEntry(h.t, h.state, pageFieldFind)
	e.SetText(q)
	e.OnSubmitted(q)
}

// pane is what the window shows now, inside the phone-shaped root if the test
// put one there (wrap).
func (h *appearanceHarness) pane() fyne.CanvasObject {
	if w, ok := h.state.window.Content().(*phoneRoot); ok {
		return w.content
	}
	return h.state.window.Content()
}

// phoneRoot stands in for the root CreateMainUI returns on the phones: a
// layoutWatcher, a widget whose renderer holds the whole compact tree. The
// host's own root is the bare tree, and a walk that goes only through
// containers reaches every object in it and none in a phone's. Deliberately
// not a contentWrapper: it proves the Find bar is stopped without any walk
// reaching it.
type phoneRoot struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func (w *phoneRoot) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(w.content) }

// wrap puts the window's content inside a phoneRoot, as every build does on
// Android and on an iPhone that may read in landscape. rebuildWindow replaces
// it with the bare tree, so a test wraps again after each rebuild it wants
// the next one to find wrapped.
func (h *appearanceHarness) wrap() {
	h.t.Helper()
	cnv := h.state.window.Canvas()
	if _, ok := cnv.Content().(*phoneRoot); ok {
		return
	}
	w := &phoneRoot{content: cnv.Content()}
	w.ExtendBaseWidget(w)
	cnv.SetContent(w)
}

// findBar is the first infinite progress bar in o, or nil.
func findBar(o fyne.CanvasObject) *widget.ProgressBarInfinite {
	var bar *widget.ProgressBarInfinite
	walkTree(o, func(n fyne.CanvasObject) {
		if b, ok := n.(*widget.ProgressBarInfinite); ok && bar == nil {
			bar = b
		}
	})
	return bar
}

const (
	findPrompt  = "Find passages by meaning"
	findRunning = "Searching with AI…"
	findNothing = "AI didn’t find matching passages — try rephrasing."
)

// A FIND IN FLIGHT COMES BACK IN FLIGHT, AND ITS ANSWER LANDS IN THE TAB ON
// THE CANVAS. Mutations guarded: renderFind without its in-flight case (the
// rebuilt tab shows the prompt), the landing painting into the submitting
// build's results host (the rebuilt tab keeps its spinner), and rebuildWindow
// not stopping the torn-down tree's bars (the old bar runs on).
func TestAFindInFlightSurvivesARebuild(t *testing.T) {
	h, held := findTab(t)
	h.ask("mercy")
	if len(*held) != 1 {
		t.Fatalf("control: the Find must reach the seam once, got %d", len(*held))
	}
	oldBar := findBar(h.pane())
	if !treeHasText(h.pane(), findRunning) || oldBar == nil || !oldBar.Running() {
		t.Fatalf("control: the tab must show the running search; texts %v", treeTexts(h.pane()))
	}

	h.flip()
	if !treeHasText(h.pane(), findRunning) || findTreeButton(h.pane(), "Cancel") == nil {
		t.Fatalf("the rebuilt tab must show the search still running, with Cancel; texts %v", treeTexts(h.pane()))
	}
	if treeHasText(h.pane(), findPrompt) {
		t.Error("the rebuilt tab shows the empty prompt while the Find runs")
	}
	if oldBar.Running() {
		t.Error("the torn-down tree's progress bar is still running")
	}
	newBar := findBar(h.pane())
	if newBar == nil || newBar == oldBar {
		t.Fatal("the rebuilt tab must draw a bar of its own")
	}

	verse := h.state.Bible.GetChapter("John", 1)[0]
	(*held)[0].done([]Verse{verse}, nil)
	if !treeHasText(h.pane(), "1 passage found by AI") {
		t.Errorf("the answer must land in the tab on the canvas; texts %v", treeTexts(h.pane()))
	}
	if treeHasText(h.pane(), findRunning) {
		t.Error("the rebuilt tab still shows the spinner after the answer landed")
	}
	if newBar.Running() {
		t.Error("the rebuilt tab's bar runs on after the answer landed")
	}
}

// CANCEL ON THE REBUILT TAB REACHES THE REQUEST THE OLD TAB STARTED, and a
// Find stopped — by Cancel, or by leaving the tab mid-flight — comes back as
// the prompt, never as the answer "found nothing". Mutations guarded:
// renderFind taking a stopped Find's empty results for an answer, and the
// tab switch not recording the stop.
func TestAStoppedFindIsNeverAnswerOfNothing(t *testing.T) {
	t.Run("Cancel after a rebuild", func(t *testing.T) {
		h, held := findTab(t)
		h.ask("mercy")
		h.flip()
		btn := findTreeButton(h.pane(), "Cancel")
		if btn == nil {
			t.Fatalf("the rebuilt tab must offer Cancel while the Find runs; texts %v", treeTexts(h.pane()))
		}
		btn.OnTapped()
		if !(*held)[0].abandoned {
			t.Error("Cancel on the rebuilt tab did not abandon the request the old tab started")
		}
		h.flip()
		if !treeHasText(h.pane(), findPrompt) || treeHasText(h.pane(), findNothing) {
			t.Errorf("a cancelled Find must come back as the prompt; texts %v", treeTexts(h.pane()))
		}
	})
	t.Run("leaving the tab mid-flight", func(t *testing.T) {
		h, held := findTab(t)
		h.ask("mercy")
		var readCell *tabCell
		walkTree(h.pane(), func(o fyne.CanvasObject) {
			if c, ok := o.(*tabCell); ok && readCell == nil && c.label == "Read" {
				readCell = c
			}
		})
		if readCell == nil {
			t.Fatal("control: no Read tab to leave by")
		}
		readCell.Tapped(nil)
		if !(*held)[0].abandoned || h.state.CurrentTab != 0 {
			t.Fatal("control: leaving the tab abandons the Find")
		}
		h.state.CurrentTab = 2
		rebuildWindow(h.state)
		if !treeHasText(h.pane(), findPrompt) || treeHasText(h.pane(), findNothing) {
			t.Errorf("a Find left mid-flight must come back as the prompt; texts %v", treeTexts(h.pane()))
		}
	})
}

// EVERY FINISHED FIND COMES BACK AS IT WAS: the error card, the answer of
// nothing, and the passages. Mutations guarded: renderFind without its error
// case, or with an empty answer falling to the prompt (each comes back as the
// prompt).
func TestAFinishedFindSurvivesARebuild(t *testing.T) {
	verse := func(h *appearanceHarness) []Verse { return h.state.Bible.GetChapter("John", 1)[:2] }
	for _, tc := range []struct {
		name string
		land func(h *appearanceHarness, f *heldFind)
		want func(fyne.CanvasObject) bool
	}{
		{"an error", func(h *appearanceHarness, f *heldFind) { f.done(nil, errors.New("the provider fell over")) },
			func(o fyne.CanvasObject) bool { return findTreeButton(o, "Try again") != nil }},
		{"an answer of nothing", func(h *appearanceHarness, f *heldFind) { f.done(nil, nil) },
			func(o fyne.CanvasObject) bool { return treeHasText(o, findNothing) }},
		{"passages", func(h *appearanceHarness, f *heldFind) { f.done(verse(h), nil) },
			func(o fyne.CanvasObject) bool { return treeHasText(o, "2 passages found by AI") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, held := findTab(t)
			h.ask("mercy")
			tc.land(h, (*held)[0])
			if !tc.want(h.pane()) {
				t.Fatalf("control: the landing must show %s; texts %v", tc.name, treeTexts(h.pane()))
			}
			h.flip()
			if !tc.want(h.pane()) || treeHasText(h.pane(), findPrompt) {
				t.Errorf("%s must survive the rebuild; texts %v", tc.name, treeTexts(h.pane()))
			}
			if strings.Contains(strings.Join(treeTexts(h.pane()), "|"), findRunning) {
				t.Error("a finished Find came back running")
			}
		})
	}
	t.Run("Try again asks the same question", func(t *testing.T) {
		h, held := findTab(t)
		h.ask("mercy")
		(*held)[0].done(nil, errors.New("the provider fell over"))
		h.flip()
		retry := findTreeButton(h.pane(), "Try again")
		if retry == nil {
			t.Fatalf("the rebuilt tab lost the error card; texts %v", treeTexts(h.pane()))
		}
		retry.OnTapped()
		if len(*held) != 2 || (*held)[1].q != "mercy" {
			t.Errorf("Try again on the rebuilt tab must re-ask the question; held %d", len(*held))
		}
	})
	t.Run("clearing the field clears the error", func(t *testing.T) {
		h, held := findTab(t)
		h.ask("mercy")
		(*held)[0].done(nil, errors.New("the provider fell over"))
		var clear *widget.Button
		walkTree(h.pane(), func(o fyne.CanvasObject) {
			if b, ok := o.(*widget.Button); ok && clear == nil && b.Text == "" && b.Icon != nil && b.Icon.Name() == theme.CancelIcon().Name() {
				clear = b
			}
		})
		if clear == nil {
			t.Fatal("control: the Find field has no ✕")
		}
		clear.OnTapped()
		if !treeHasText(h.pane(), findPrompt) || findTreeButton(h.pane(), "Try again") != nil {
			t.Errorf("the ✕ must leave the prompt, not the kept error; texts %v", treeTexts(h.pane()))
		}
	})
}

// NO FIND BAR OUTLIVES ITS TREE UNDER A PHONE'S ROOT. Every rebuild of the
// Search tab mid-Find draws a bar of its own, and a bar left running in a tree
// the window no longer shows marks the live canvas dirty on every tick — on
// the phones, where Fyne maps any object to the live canvas, a full repaint
// twenty times a second until the renderer cache expires it. rebuildWindow's
// walk over the old tree cannot reach into a phone's root, so the bar is held
// in state and stopped from there. Mutations guarded: the bar held by the
// build that drew it (leaving the tab after a rebuild leaves the rebuilt bar
// running; the middle build's bar of two runs on past the landing).
func TestNoFindBarOutlivesItsTreeUnderAPhoneRoot(t *testing.T) {
	t.Run("a rebuild, then leaving the tab", func(t *testing.T) {
		h, held := findTab(t)
		h.wrap()
		h.ask("mercy")
		a := findBar(h.pane())
		h.flip()
		h.wrap()
		b := findBar(h.pane())
		if a == nil || b == nil || a == b {
			t.Fatal("control: the submitting build and the rebuilt one must each draw a bar")
		}
		if a.Running() {
			t.Error("the submitting build's bar runs on after the rebuild replaced its tree")
		}
		var readCell *tabCell
		walkTree(h.pane(), func(o fyne.CanvasObject) {
			if c, ok := o.(*tabCell); ok && readCell == nil && c.label == "Read" {
				readCell = c
			}
		})
		if readCell == nil {
			t.Fatal("control: no Read tab to leave by")
		}
		readCell.Tapped(nil)
		if !(*held)[0].abandoned {
			t.Fatal("control: leaving the tab abandons the Find, so no landing will ever stop a bar")
		}
		if b.Running() {
			t.Error("the rebuilt tab's bar runs on after the reader left the tab; nothing is left to stop it")
		}
	})
	t.Run("two rebuilds, then the landing", func(t *testing.T) {
		h, held := findTab(t)
		h.wrap()
		h.ask("mercy")
		a := findBar(h.pane())
		h.flip()
		h.wrap()
		b := findBar(h.pane())
		h.flip()
		h.wrap()
		c := findBar(h.pane())
		(*held)[0].done([]Verse{h.state.Bible.GetChapter("John", 1)[0]}, nil)
		for name, bar := range map[string]*widget.ProgressBarInfinite{"the first build's": a, "the middle build's": b, "the last build's": c} {
			if bar == nil {
				t.Fatalf("control: %s bar was never drawn", name)
			}
			if bar.Running() {
				t.Errorf("%s bar runs on after the answer landed", name)
			}
		}
		if !treeHasText(h.pane(), "1 passage found by AI") {
			t.Errorf("the answer must land in the tab on the canvas; texts %v", treeTexts(h.pane()))
		}
	})
}

// wrappingRoot is a phone-shaped root that says what it wraps, as the phones'
// layoutWatcher does (contentWrapper).
type wrappingRoot struct{ phoneRoot }

func (w *wrappingRoot) wrappedContent() fyne.CanvasObject { return w.content }

// REBUILDWINDOW'S WALK GOES INTO A PHONE'S ROOT. Anything else left running in
// the page a rebuild replaces is stopped by the walk over the old tree, and on
// a phone the tree's root is a widget the walk once stopped at. Mutation
// guarded: the walk without its contentWrapper case (the bar runs on).
func TestRebuildStopsABarInsideAPhoneRoot(t *testing.T) {
	h := newAppearanceHarness(t, false)
	bar := widget.NewProgressBarInfinite()
	root := &wrappingRoot{phoneRoot{content: container.NewStack(h.state.window.Content(), bar)}}
	root.ExtendBaseWidget(root)
	h.state.window.SetContent(root)
	bar.Start()
	if !bar.Running() {
		t.Fatal("control: the bar must be running before the rebuild")
	}
	rebuildWindow(h.state)
	if bar.Running() {
		t.Error("a bar inside the phone's root runs on after the rebuild replaced the page")
	}
}

// A FIND STOPPED BY A MODE ROUND TRIP IS STILL A STOPPED FIND. Switching to
// Search mid-Find abandons it and records it as stopped; coming back to Find
// must not read that stop as an answer of nothing. Mutation guarded: the mode
// switch writing aiSearchCancelled = inFlight again (the second switch, with
// nothing in flight, clears the stop, and the pane reads "found nothing").
func TestAModeRoundTripKeepsAStoppedFindStopped(t *testing.T) {
	h, held := findTab(t)
	h.ask("mercy")
	search := findTreeButton(h.pane(), "Search")
	if search == nil {
		t.Fatalf("control: no Search mode button; texts %v", treeTexts(h.pane()))
	}
	search.OnTapped()
	if !(*held)[0].abandoned {
		t.Fatal("control: the mode switch abandons the Find in flight")
	}
	find := findTreeButton(h.pane(), "Find")
	if find == nil {
		t.Fatal("control: no Find mode button")
	}
	find.OnTapped()
	if !treeHasText(h.pane(), findPrompt) || treeHasText(h.pane(), findNothing) {
		t.Errorf("a Find stopped by a mode switch must come back as the prompt; texts %v", treeTexts(h.pane()))
	}
	h.flip()
	if !treeHasText(h.pane(), findPrompt) || treeHasText(h.pane(), findNothing) {
		t.Errorf("and still after a rebuild; texts %v", treeTexts(h.pane()))
	}
}

// A QUESTION ASKED WITH NO KEY ABANDONS THE ONE IN FLIGHT. The pane is drawn
// from state, and a Find left loading would draw a spinner nothing stops: its
// own landing is superseded by the new question and returns before it clears
// the flag. Mutation guarded: the no-key branch without abandonAISearch (the
// pane shows the spinner, the request bills on, and the landing leaves it).
func TestANoKeyQuestionAbandonsTheFindInFlight(t *testing.T) {
	h, held := findTab(t)
	h.ask("mercy")
	h.state.aiKeys.setAPIKey(defaultProviderID, "") // the key removed while the Find runs
	h.ask("grace")
	if len(*held) != 1 {
		t.Fatalf("control: with no key nothing reaches the provider; held %d", len(*held))
	}
	check := func(when string) {
		t.Helper()
		if !(*held)[0].abandoned {
			t.Errorf("%s: the Find in flight was not abandoned", when)
		}
		if !treeHasText(h.pane(), "Find needs your own AI key") || treeHasText(h.pane(), findRunning) {
			t.Errorf("%s: the pane must say a key is needed, not spin; texts %v", when, treeTexts(h.pane()))
		}
		if bar := findBar(h.pane()); bar != nil && bar.Running() {
			t.Errorf("%s: a Find bar is running", when)
		}
	}
	check("after the question")
	(*held)[0].done([]Verse{h.state.Bible.GetChapter("John", 1)[0]}, nil)
	check("after the abandoned Find's landing")
	h.flip()
	check("after a rebuild")
}

// THE FIND HOOKS BELONG TO THE TREE THAT SET THEM. A rebuild onto another tab
// leaves no Find repaint and only that tab's page fields; and the repaint
// draws Find only while the tab is in Find mode. Mutations guarded: the resets
// in buildCompactUI (the Search tab's repaint and fields outlive it) and the
// repaint's mode check (a repaint in Search mode draws the Find prompt over
// the keyword results).
func TestTheFindHooksBelongToTheTreeThatSetThem(t *testing.T) {
	h, _ := findTab(t)
	if h.state.repaintFind == nil || h.state.pageFields[pageFieldFind] == nil {
		t.Fatal("control: the Search tab sets its repaint and registers its fields")
	}
	h.state.CurrentTab = 1
	rebuildWindow(h.state)
	if h.state.repaintFind != nil {
		t.Error("the Books tab left the Search tab's Find repaint in place")
	}
	if len(h.state.pageFields) != 1 || h.state.pageFields[pageFieldBooks] == nil {
		t.Errorf("the Books tab must register its filter alone; got %v", h.state.pageFields)
	}

	h.state.CurrentTab = 2
	rebuildWindow(h.state)
	search := findTreeButton(h.pane(), "Search")
	if search == nil {
		t.Fatal("control: no Search mode button")
	}
	search.OnTapped()
	if treeHasText(h.pane(), findPrompt) {
		t.Fatal("control: Search mode does not show the Find prompt")
	}
	h.state.repaintFind()
	if treeHasText(h.pane(), findPrompt) {
		t.Error("a Find repaint in Search mode drew the Find pane over the keyword results")
	}
}
