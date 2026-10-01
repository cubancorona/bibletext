package bibletext

// THE AI WAITING SCREENS NAME THE MODEL THE REQUEST SENDS.
//
// The Study with AI panel, the Search tab's Find (the shared layout every
// platform ships, and the former sidebar layout) and Settings' Test key each
// carry a quiet line naming the provider and model the reader is waiting on.
// The line comes from the request (ai_model_in_use.go): the resolver reports
// every model it is about to send to, the retry onto a discovered replacement
// included, so the screens name what is sent, not what was configured. These
// tests hold the requests at the provider seams and look at the screens.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// resolverSend is one send a routed resolver made: the model it went to, and
// the answer the test gives it. A send nobody answers waits forever — a parked
// goroutine is a harmless leak, and a completion running app code on its own
// goroutine against a later test is not (see stubAIGenerate).
type resolverSend struct {
	model  string
	answer chan error
}

// sendClient is a provider client that publishes each send and waits for the
// test's answer.
type sendClient struct {
	model string
	sends chan<- *resolverSend
}

func (c sendClient) generate(context.Context, string) (string, error) {
	s := &resolverSend{model: c.model, answer: make(chan error, 1)}
	c.sends <- s
	if err := <-s.answer; err != nil {
		return "", err
	}
	return "1 John 4:8", nil
}

// routedResolver is the provider's real resolver — its id, shipped default,
// tier and exclusions, reading the store as a request does — with its sends
// routed to the test and a model list offering replacement. It runs on the
// request's goroutine, so it cannot fail a test; a provider that stopped
// building a modelResolver panics here, which is loud enough.
func routedResolver(store *keyStore, id string, sends chan<- *resolverSend, replacement string) *modelResolver {
	info, _ := providerByID(id)
	r := info.New(store, "test-key").(*modelResolver)
	r.build = func(m string) aiClient { return sendClient{model: m, sends: sends} }
	r.list = func(context.Context, string) ([]discoveredModel, error) {
		return []discoveredModel{{id: replacement, rank: 1}}, nil
	}
	return r
}

// routeStudy sends every study request through routedResolver for the
// store's active provider.
func routeStudy(t *testing.T, replacement string) chan *resolverSend {
	t.Helper()
	sends := make(chan *resolverSend, 8)
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, st *AppState, _, _, _ string) (string, error) {
		store := st.keys()
		return routedResolver(store, store.activeProvider(), sends, replacement).generate(ctx, "study")
	}
	t.Cleanup(func() { aiActionRun = prev })
	return sends
}

// routeFind sends every Find through routedResolver, behind the real
// startFind and runAISearch.
func routeFind(t *testing.T, replacement string) chan *resolverSend {
	t.Helper()
	aiCacheMu.Lock()
	aiCache = map[string]string{} // no Find may be answered from an earlier test's reply
	aiCacheMu.Unlock()
	sends := make(chan *resolverSend, 8)
	prev := aiSearchGenerate
	aiSearchGenerate = func(ctx context.Context, info providerInfo, store *keyStore, _, prompt string) (string, error) {
		return routedResolver(store, info.ID, sends, replacement).generate(ctx, prompt)
	}
	t.Cleanup(func() { aiSearchGenerate = prev })
	return sends
}

func nextSend(t *testing.T, sends chan *resolverSend) *resolverSend {
	t.Helper()
	select {
	case s := <-sends:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("no send reached the provider")
		return nil
	}
}

func nextCtx(t *testing.T, ctxs chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-ctxs:
		return ctx
	case <-time.After(2 * time.Second):
		t.Fatal("no request reached the provider seam")
		return nil
	}
}

// modelLineBox is the model line in o — its box, and the text inside — or
// nils.
func modelLineBox(o fyne.CanvasObject) (*fyne.Container, *widget.RichText) {
	var box *fyne.Container
	walkTree(o, func(n fyne.CanvasObject) {
		if c, ok := n.(*fyne.Container); ok && box == nil {
			if _, ok := c.Layout.(modelLineLayout); ok {
				box = c
			}
		}
	})
	if box == nil || len(box.Objects) == 0 {
		return nil, nil
	}
	rt, _ := box.Objects[0].(*widget.RichText)
	return box, rt
}

// modelLineIn is the text of the model line in o, and whether o has one.
func modelLineIn(o fyne.CanvasObject) (string, bool) {
	_, rt := modelLineBox(o)
	if rt == nil {
		return "", false
	}
	return rt.String(), true
}

// captionLine is the height the hint's box gives one line of the waiting
// screens' caption, and so the height of a one-line model line.
func captionLine() float32 { return captionHeightFor(1) }

// hintText is the muted hint every AI wait carries beside the model line.
const hintText = "Capable models can take a minute or more."

// wantHintStyle fails unless o's model line is in the text style of the hint
// beside it: caption size, muted, centred, whatever those are made of.
func wantHintStyle(t *testing.T, o fyne.CanvasObject, when string) {
	t.Helper()
	_, line := modelLineBox(o)
	var hint *widget.RichText
	walkTree(o, func(n fyne.CanvasObject) {
		if rt, ok := n.(*widget.RichText); ok && hint == nil && rt.String() == hintText {
			hint = rt
		}
	})
	if line == nil || hint == nil {
		t.Fatalf("%s: the wait needs both the model line and the hint; texts %v", when, treeTexts(o))
	}
	ls, lok := line.Segments[0].(*widget.TextSegment)
	hs, hok := hint.Segments[0].(*widget.TextSegment)
	if !lok || !hok {
		t.Fatalf("%s: the model line and the hint must each be a text segment", when)
	}
	if ls.Style != hs.Style {
		t.Errorf("%s: the model line is styled %+v, the hint beside it %+v", when, ls.Style, hs.Style)
	}
	if line.Wrapping != hint.Wrapping {
		t.Errorf("%s: the model line wraps %v, the hint %v", when, line.Wrapping, hint.Wrapping)
	}
}

// wantModelLine fails unless o's model line reads want ("" for a line still
// waiting for its request's report).
func wantModelLine(t *testing.T, o fyne.CanvasObject, want, when string) {
	t.Helper()
	got, ok := modelLineIn(o)
	if !ok {
		t.Fatalf("%s: the waiting screen has no model line; texts %v", when, treeTexts(o))
	}
	if got != want {
		t.Fatalf("%s: the model line reads %q, want %q", when, got, want)
	}
}

func label(provider, model string) string { return aiModelInUse{provider, model}.label() }

// THE WORDING IS THE SETTINGS PICKERS' OWN. The assistant picker lists each
// provider by Name and the model picker lists each model by its id; the line
// is the two, and nothing before a report. Mutation: ShortName for Name (the
// Settings key row's word, not the picker's).
func TestTheModelLineUsesThePickersNames(t *testing.T) {
	for _, p := range aiProviders() {
		want := p.Name + " · " + p.Model
		if got := label(p.ID, p.Model); got != want {
			t.Errorf("%s: label %q, want %q", p.ID, got, want)
		}
	}
	if got := (aiModelInUse{}).label(); got != "" {
		t.Errorf("before a report the line must be empty, got %q", got)
	}
	if got := label(providerAnthropic, "claude-opus-5"); got != "Claude (Anthropic) · claude-opus-5" {
		t.Errorf("got %q", got)
	}
}

// EVERY SEND IS ANNOUNCED BEFORE IT GOES, THE RETRY INCLUDED. Mutations: no
// report before the first send; no report before the retry; the retry
// reported after its send (the screen would name the old model for the
// whole wait on it); the shipped default reported in place of the model
// resolved (an override the reader chose).
func TestTheResolverReportsEachModelBeforeSendingIt(t *testing.T) {
	run := func(store *keyStore) []string {
		var log []string
		r := routedResolver(store, providerGemini, nil, "gemini-3-pro")
		r.build = func(m string) aiClient {
			return clientFunc(func(context.Context, string) (string, error) {
				log = append(log, "send "+m)
				if m != "gemini-3-pro" { // every model but the replacement is gone
					return "", &apiHTTPError{StatusCode: http.StatusNotFound}
				}
				return "OK", nil
			})
		}
		ctx := withAIModelObserver(context.Background(), func(m aiModelInUse) {
			log = append(log, "report "+m.provider+" "+m.model)
		})
		_, _ = r.generate(ctx, "prompt")
		return log
	}

	store := newKeyStoreWith(newFakePrefs())
	got := strings.Join(run(store), "; ")
	want := strings.Join([]string{
		"report gemini " + geminiModel, "send " + geminiModel,
		"report gemini gemini-3-pro", "send gemini-3-pro",
	}, "; ")
	if got != want {
		t.Fatalf("a retired default and its replacement:\n got  %s\n want %s", got, want)
	}

	// A model the reader pinned is what is sent, and so what is named; when it
	// is gone it is not replaced (pinned is pinned), so nothing more is named.
	store = newKeyStoreWith(newFakePrefs())
	store.setOverrideModel(providerGemini, geminiFastModel)
	got = strings.Join(run(store), "; ")
	want = "report gemini " + geminiFastModel + "; send " + geminiFastModel
	if got != want {
		t.Fatalf("a pinned model:\n got  %s\n want %s", got, want)
	}
}

type clientFunc func(context.Context, string) (string, error)

func (f clientFunc) generate(ctx context.Context, p string) (string, error) { return f(ctx, p) }

// A LINE DRAWN WITH A LONG NAME ALREADY ON IT MEASURES ITS WRAPPED HEIGHT
// BEFORE ANYTHING HAS LAID IT OUT, AND A LINE OF ONE LINE IS AS TALL AS THE
// HINT'S BOX MAKES A LINE. The first is how a Find tab rebuilt mid-Find draws
// it, from state, and a parent sized from MinSize before its first layout
// (fitBody measures the waiting column so) must be given every line; a
// word-wrapping RichText answers one line until it has a width. The second
// keeps the line from costing the column more than a line of the hint does:
// the RichText's own height carries inner padding the hint's box leaves out,
// and on a phone in landscape every point of the column is one Cancel does not
// have. Empty or filled with a short name it is the same one line, so the
// report moves nothing. Mutations: the layout asking the text its height
// before giving it the measure; the layout reporting the text's own height.
func TestAModelLineMeasuresWrappedBeforeLayout(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	long := aiModelInUse{providerGemini, "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"}
	line := newAIModelLine(long)
	got := line.box.MinSize()
	if got.Width != aiModelLineWidth {
		t.Errorf("the line measures %vpt wide, want its %vpt measure", got.Width, aiModelLineWidth)
	}
	if got.Height < captionLine()+captionRow() {
		t.Errorf("the line measures %vpt tall before layout, one line of %vpt: its name needs more",
			got.Height, captionLine())
	}
	for _, m := range []aiModelInUse{{}, {providerGemini, geminiModel}} {
		if h := newAIModelLine(m).box.MinSize().Height; h != captionLine() {
			t.Errorf("a line reading %q measures %vpt; one line of the hint's box is %vpt", m.label(), h, captionLine())
		}
	}
}

// --- The Study with AI panel -------------------------------------------------

func studyOverlay(t *testing.T, w fyne.Window) *widget.PopUp {
	t.Helper()
	p, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
	if !ok || p == nil {
		t.Fatalf("the study panel did not open (%T)", w.Canvas().Overlays().Top())
	}
	return p
}

// THE STUDY PANEL NAMES THE MODEL THE REQUEST SENDS, AND FOLLOWS THE RETRY.
// The shipped default is gone (404); the resolver finds a replacement and
// sends again, and the line moves with it. The line is in the hint's style.
// Mutations: the panel putting no observer on the request's context (the line
// stays empty); the resolver not reporting the retry (the line keeps naming
// the model that is gone); the line in body text, unmuted and left-aligned.
func TestTheStudyWaitNamesTheModelAndFollowsTheRetry(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	sends := routeStudy(t, "gemini-3-pro")
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionExplain, "God is love.", "")
	first := nextSend(t, sends)
	if first.model != geminiModel {
		t.Fatalf("control: the first send went to %q, want the default %q", first.model, geminiModel)
	}
	overlay := studyOverlay(t, w)
	if !treeHasText(overlay, "Reading the passage…") {
		t.Fatalf("control: not the waiting state; texts %v", treeTexts(overlay))
	}
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiModel, "first send")
	wantHintStyle(t, overlay, "the study wait")

	first.answer <- &apiHTTPError{StatusCode: http.StatusNotFound}
	retry := nextSend(t, sends)
	if retry.model != "gemini-3-pro" {
		t.Fatalf("control: the retry went to %q, want the replacement", retry.model)
	}
	wantModelLine(t, overlay, "Gemini (Google) · gemini-3-pro", "retry on the replacement")
}

// THE LINE FOLLOWS THE FASTER-MODEL SWITCH. Mutation: the resolver naming the
// shipped default rather than the model it resolves (the switch writes the
// reader's override; the line would keep the capable model's name while the
// fast one works).
func TestTheStudyWaitFollowsTheFasterModel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	sends := routeStudy(t, "gemini-3-pro")
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionExplain, "God is love.", "")
	nextSend(t, sends)
	overlay := studyOverlay(t, w)
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiModel, "before the switch")

	sw := findTreeLink(overlay, "Switch to a faster model")
	if sw == nil {
		t.Fatalf("control: no faster-model offer; texts %v", treeTexts(overlay))
	}
	tapLinkText(sw)
	fast := nextSend(t, sends)
	if fast.model != geminiFastModel {
		t.Fatalf("control: the switch sent to %q, want %q", fast.model, geminiFastModel)
	}
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiFastModel, "after the switch")
}

// A REPORT FROM A REQUEST THE PANEL NO LONGER WAITS ON PAINTS NOTHING. The
// requests here report only when the test says, on the test's goroutine, so a
// late report can be put exactly where the race would put it: after the
// faster-model switch has started the next request, and after Cancel.
// Mutations: the panel's generation check (the replaced request's retry
// renames the new wait); its waiting check (a cancelled request's report
// repaints the dismissed panel); setThinking reusing the last line (the new
// wait names the replaced request's model before its own has said).
func TestTheStudyWaitIgnoresAStaleReport(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctxs := stubAIActionParked(t)
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionExplain, "God is love.", "")
	slow := nextCtx(t, ctxs)
	overlay := studyOverlay(t, w)
	wantModelLine(t, overlay, "", "before the request reports")
	reportAIModel(slow, providerGemini, geminiModel)
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiModel, "the request reported")

	sw := findTreeLink(overlay, "Switch to a faster model")
	if sw == nil {
		t.Fatalf("control: no faster-model offer; texts %v", treeTexts(overlay))
	}
	tapLinkText(sw)
	fast := nextCtx(t, ctxs)
	wantModelLine(t, overlay, "", "the new wait, before its request reports")
	reportAIModel(fast, providerGemini, geminiFastModel)
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiFastModel, "the new request reported")

	// The replaced request was between attempts when it was abandoned, and
	// its retry's report arrives now.
	reportAIModel(slow, providerGemini, "gemini-3-pro")
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiFastModel, "the replaced request's late retry")

	cancel := findTreeButton(overlay, "Cancel")
	if cancel == nil {
		t.Fatalf("control: no Cancel; texts %v", treeTexts(overlay))
	}
	cancel.OnTapped()
	reportAIModel(fast, providerGemini, "gemini-3-pro")
	wantModelLine(t, overlay, "Gemini (Google) · "+geminiFastModel, "a report after Cancel")
}

// descendantPos is target's position inside root, adding up the positions of
// the containers between them; ok is false when target is not under root.
func descendantPos(root, target fyne.CanvasObject) (fyne.Position, bool) {
	if root == target {
		return fyne.NewPos(0, 0), true
	}
	var kids []fyne.CanvasObject
	switch v := root.(type) {
	case *fyne.Container:
		kids = v.Objects
	case *container.Scroll:
		kids = []fyne.CanvasObject{v.Content}
	}
	for _, k := range kids {
		if p, ok := descendantPos(k, target); ok {
			return k.Position().Add(p), true
		}
	}
	return fyne.Position{}, false
}

// scrollHolding is the scroll in o whose content holds target.
func scrollHolding(o, target fyne.CanvasObject) *container.Scroll {
	var found *container.Scroll
	walkTree(o, func(n fyne.CanvasObject) {
		if s, ok := n.(*container.Scroll); ok && found == nil {
			if _, in := descendantPos(s.Content, target); in {
				found = s
			}
		}
	})
	return found
}

// A NAME TOO LONG FOR ONE LINE WRAPS; IT NEVER WIDENS THE PANEL, AND ON A
// LANDSCAPE PHONE CANCEL STAYS IN REACH. The waiting column is in a scroll
// sized by fitBody because a phone on its side leaves it about half its
// natural height; a line that grows must lay the column out again at its new
// height, or the line overlaps what is under it and the scroll's reach stops
// short of it. Where the screen has room for the grown column, the panel grows
// to show it whole, as it was sized to show the column before the report.
// Mutations: no wrapping (the text runs one line, wider than the measure and
// the card); a line as wide as its text; no re-fit when the line grows (the
// column keeps its old height); the column laid out again but the panel not
// re-fitted (Cancel needs a scroll on a screen with room for it).
func TestTheStudyWaitWrapsALongModelAndKeepsCancelInReach(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	for _, sc := range []struct {
		name string
		w, h float32
		room bool // the screen can show the whole grown column
	}{
		{"phone portrait", 375, 667, true},
		{"phone landscape", 667, 375, false},
		{"small phone landscape", 568, 320, false},
	} {
		t.Run(sc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			ctxs := stubAIActionParked(t)
			w := test.NewWindow(widget.NewLabel("reading"))
			defer w.Close()
			w.Resize(fyne.NewSize(sc.w, sc.h))
			st := studyPanelState(t, w)

			showAIPanel(st, aiActionExplain, "God is love.", "")
			ctx := nextCtx(t, ctxs)
			p := studyOverlay(t, w)
			reportAIModel(ctx, providerGemini, long)
			wantModelLine(t, p, "Gemini (Google) · "+long, "the long name reported")

			line, text := modelLineBox(p)
			if lw := line.MinSize().Width; lw > aiModelLineWidth {
				t.Errorf("the line wants %vpt, wider than its %vpt measure", lw, aiModelLineWidth)
			}
			if tw := text.MinSize().Width; tw > aiModelLineWidth {
				t.Errorf("the name wants %vpt on one line: it does not wrap", tw)
			}
			if h := line.MinSize().Height; h < 1.5*captionLine() {
				t.Errorf("the line is %vpt tall, about one line of %vpt: the long name did not wrap",
					h, captionLine())
			}

			top, bottom := sheetBox(t, p)
			if top < 0 || bottom > sc.h {
				t.Errorf("the panel runs off the screen: %v..%v on a %vpt canvas", top, bottom, sc.h)
			}
			if cw := p.Content.Size().Width; cw > sc.w {
				t.Errorf("the panel is %vpt wide on a %vpt canvas", cw, sc.w)
			}

			cancel := findTreeButton(p, "Cancel")
			if cancel == nil {
				t.Fatalf("no Cancel; texts %v", treeTexts(p))
			}
			scroll := scrollHolding(p, cancel)
			if scroll == nil {
				t.Fatal("Cancel is not inside the waiting column's scroll")
			}
			col := scroll.Content
			if got, want := col.Size().Height, col.MinSize().Height; got < want {
				t.Errorf("the waiting column is laid out %vpt tall but needs %vpt: the grown line "+
					"overlaps what is under it and the scroll stops short of it", got, want)
			}
			pos, _ := descendantPos(col, cancel)
			if end := pos.Y + cancel.Size().Height; end > col.Size().Height {
				t.Errorf("Cancel ends at %vpt, past the %vpt the scroll can reach", end, col.Size().Height)
			}
			if vh := scroll.Size().Height; vh <= 0 || vh > bottom-top {
				t.Errorf("the scroll's viewport is %vpt inside a %vpt panel", vh, bottom-top)
			}
			if vh, need := scroll.Size().Height, col.MinSize().Height; sc.room && vh < need {
				t.Errorf("the screen has room for the %vpt column, but the panel shows %vpt of it: "+
					"Cancel needs a scroll", need, vh)
			}
		})
	}
}

// --- Find on the Search tab (the shared layout, every platform) ------------

// liveFindTab is the Search tab in Find mode with a key, whose Finds go
// through the real startFind.
func liveFindTab(t *testing.T) *appearanceHarness {
	t.Helper()
	h := newAppearanceHarness(t, true)
	h.state.aiKeys = newKeyStoreWith(newFakePrefs())
	h.state.aiKeys.setAPIKey(defaultProviderID, "test-key")
	h.state.aiSearchMode = true
	h.state.CurrentTab = 2
	rebuildWindow(h.state)
	return h
}

// FIND NAMES THE MODEL THE REQUEST SENDS, FOLLOWS THE RETRY, AND NAMES IT
// AGAIN IN A TAB REBUILT MID-FIND, in the hint's style. Mutations: the tab's
// submit putting no observer on the request; showFindModel not reaching the
// line on the canvas; the resolver not reporting the retry; the waiting view
// drawing its line empty rather than from state (a light/dark rebuild forgets
// the model); the line in body text, unmuted and left-aligned.
func TestTheFindWaitNamesTheModelAndFollowsTheRetry(t *testing.T) {
	sends := routeFind(t, "gemini-3-pro")
	h := liveFindTab(t)
	h.ask("where love is described")
	first := nextSend(t, sends)
	if !treeHasText(h.pane(), findRunning) {
		t.Fatalf("control: the Find is not waiting; texts %v", treeTexts(h.pane()))
	}
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiModel, "first send")
	wantHintStyle(t, h.pane(), "the Find wait")

	first.answer <- &apiHTTPError{StatusCode: http.StatusNotFound}
	if retry := nextSend(t, sends); retry.model != "gemini-3-pro" {
		t.Fatalf("control: the retry went to %q", retry.model)
	}
	wantModelLine(t, h.pane(), "Gemini (Google) · gemini-3-pro", "retry on the replacement")

	h.flip() // a light/dark change mid-Find rebuilds the tab
	if !treeHasText(h.pane(), findRunning) {
		t.Fatalf("control: the rebuilt tab is not waiting; texts %v", treeTexts(h.pane()))
	}
	wantModelLine(t, h.pane(), "Gemini (Google) · gemini-3-pro", "the tab rebuilt mid-Find")
}

// FIND'S LINE FOLLOWS THE FASTER-MODEL SWITCH. Mutation: the resolver naming
// the shipped default rather than the model it resolves.
func TestTheFindWaitFollowsTheFasterModel(t *testing.T) {
	sends := routeFind(t, "gemini-3-pro")
	h := liveFindTab(t)
	h.ask("where love is described")
	nextSend(t, sends)
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiModel, "before the switch")

	sw := findTreeLink(h.pane(), "Switch to a faster model")
	if sw == nil {
		t.Fatalf("control: no faster-model offer; texts %v", treeTexts(h.pane()))
	}
	tapLinkText(sw)
	if fast := nextSend(t, sends); fast.model != geminiFastModel {
		t.Fatalf("control: the switch sent to %q, want %q", fast.model, geminiFastModel)
	}
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiFastModel, "after the switch")
}

// A REPORT FROM A FIND THAT WAS CANCELLED PAINTS NOTHING ON THE FIND AFTER
// IT. Mutations: showFindModel without its session check (the cancelled
// Find's retry renames the new wait); the submit not clearing the last
// Find's model (the new wait names the old model before its own has said).
func TestTheFindWaitIgnoresAStaleReport(t *testing.T) {
	stubAIGenerate(t)
	h := liveFindTab(t)
	h.ask("first question")
	first := nextCtx(t, aiCtxSeen)
	wantModelLine(t, h.pane(), "", "before the request reports")
	reportAIModel(first, providerGemini, geminiModel)
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiModel, "the request reported")

	cancel := findTreeButton(h.pane(), "Cancel")
	if cancel == nil {
		t.Fatalf("control: no Cancel; texts %v", treeTexts(h.pane()))
	}
	cancel.OnTapped()
	h.ask("second question")
	second := nextCtx(t, aiCtxSeen)
	wantModelLine(t, h.pane(), "", "the next Find, before its request reports")
	reportAIModel(second, providerGemini, geminiFastModel)
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiFastModel, "the next Find reported")

	reportAIModel(first, providerGemini, "gemini-3-pro") // the cancelled Find's retry
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiFastModel, "the cancelled Find's late retry")
	if got := h.state.aiSearchModel; got.model != geminiFastModel {
		t.Fatalf("state holds %q for the Find in flight, want %q", got.model, geminiFastModel)
	}
}

// A FIND ASKED AGAIN WHILE ONE IS IN FLIGHT NAMES ITS OWN MODEL, AND ITS
// ANSWER LANDS. The resubmit runs its predecessor's teardown hook after taking
// its own session token; the hook used to give up whatever submission was
// latest, which was the new one, so the new Find's model and answer were both
// dropped as stale and the tab searched until Cancel. Mutation: the hook
// giving up the session unconditionally (Abandon without its check).
func TestAFindAskedAgainMidFlightNamesItsModelAndLands(t *testing.T) {
	h, held := findTab(t)
	h.ask("mercy")
	h.ask("grace") // while the first is in flight
	if len(*held) != 2 || !(*held)[0].abandoned {
		t.Fatalf("control: the resubmit must reach the seam and abandon the first; held %d", len(*held))
	}
	(*held)[1].onModel(aiModelInUse{providerGemini, geminiModel})
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiModel, "the second Find reported")
	(*held)[0].onModel(aiModelInUse{providerGemini, "gemini-3-pro"}) // the first one's late retry
	wantModelLine(t, h.pane(), "Gemini (Google) · "+geminiModel, "the first Find's late report")

	verse := h.state.Bible.GetChapter("John", 1)[0]
	(*held)[1].done([]Verse{verse}, nil)
	if !treeHasText(h.pane(), "1 passage found by AI") || treeHasText(h.pane(), findRunning) {
		t.Fatalf("the second Find's answer must land; texts %v", treeTexts(h.pane()))
	}
}

// wantLineLaidOut fails unless o's model line is laid out at the height its
// name needs, with the hint under it starting where the line ends: a line
// laid out shorter than its wrapped name has its last lines over the hint.
func wantLineLaidOut(t *testing.T, o fyne.CanvasObject, when string) {
	t.Helper()
	box, _ := modelLineBox(o)
	var hint *widget.RichText
	walkTree(o, func(n fyne.CanvasObject) {
		if rt, ok := n.(*widget.RichText); ok && hint == nil && rt.String() == hintText {
			hint = rt
		}
	})
	if box == nil || hint == nil {
		t.Fatalf("%s: the wait needs both the model line and the hint; texts %v", when, treeTexts(o))
	}
	if got, want := box.Size().Height, box.MinSize().Height; got < want {
		t.Errorf("%s: the line is laid out %vpt tall; its wrapped name needs %vpt", when, got, want)
	}
	lp, lok := descendantPos(o, box)
	hp, hok := descendantPos(o, hint)
	if !lok || !hok {
		t.Fatalf("%s: the line or the hint is not in the wait's tree of containers", when)
	}
	if end := lp.Y + box.MinSize().Height; hp.Y < end {
		t.Errorf("%s: the hint starts at %vpt, inside the line that ends at %vpt", when, hp.Y, end)
	}
}

// ON A 375-WIDE PHONE A LONG NAME WRAPS INSIDE THE TAB, the wait lays it out
// again at its wrapped height the moment the report lands, and a tab rebuilt
// with that name already on state lays it out so too. The drivers lay out a
// grown line within a frame of their own accord; the wait does not leave it
// to them. Mutations: no wrapping (the name runs wider than the canvas); the
// Search tab's wait setting no relayout on its line (the wrapped name lies
// over the hint until something else lays the tab out).
func TestTheFindWaitWrapsALongModelOnAPhone(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	stubAIGenerate(t)
	h := liveFindTab(t)
	h.state.window.Resize(fyne.NewSize(375, 812))
	rebuildWindow(h.state)
	before := h.pane().MinSize().Width
	if before > 375 {
		t.Fatalf("control: the tab already wants %vpt on a 375pt phone", before)
	}
	h.ask("where love is described")
	ctx := nextCtx(t, aiCtxSeen)
	reportAIModel(ctx, providerGemini, long)
	wantModelLine(t, h.pane(), "Gemini (Google) · "+long, "the long name reported")
	if got := h.pane().MinSize().Width; got > 375 {
		t.Errorf("the Search tab wants %vpt with the model named, on a 375pt phone", got)
	}
	if _, text := modelLineBox(h.pane()); text.MinSize().Width > aiModelLineWidth {
		t.Errorf("the name wants %vpt on one line: it does not wrap", text.MinSize().Width)
	}
	wantLineLaidOut(t, h.pane(), "the long name, as its report lands")

	h.flip() // rebuilt mid-Find, the long name already on state
	wantModelLine(t, h.pane(), "Gemini (Google) · "+long, "the rebuilt tab")
	box, _ := modelLineBox(h.pane())
	if got, want := box.Size().Height, box.MinSize().Height; got < want || got < 1.5*captionLine() {
		t.Errorf("the rebuilt tab lays the line out %vpt tall; its wrapped name needs %vpt", got, want)
	}
	wantLineLaidOut(t, h.pane(), "the rebuilt tab")
}

// --- Find in the former sidebar layout ---------------------------------------

// THE SIDEBAR'S FIND WAIT NAMES THE MODEL, in the hint's style, AND A
// RESUBMIT'S PREDECESSOR CANNOT RENAME IT. Mutations: the sidebar's submit
// putting no observer on the request; aiSearchingView not making its line the
// one reports land on; the line in body text, unmuted and left-aligned.
func TestTheSidebarFindWaitNamesTheModel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	stubAIGenerate(t)
	st := sidebarFindState(t)

	pane, first := findSidebarSearchPane(t, st)
	wantModelLine(t, pane, "", "before the request reports")
	reportAIModel(first, providerGemini, geminiModel)
	wantModelLine(t, pane, "Gemini (Google) · "+geminiModel, "the request reported")
	wantHintStyle(t, pane, "the sidebar's Find wait")

	pane, second := findSidebarSearchPane(t, st) // a resubmit supersedes the first
	wantModelLine(t, pane, "", "the resubmit, before its request reports")
	reportAIModel(second, providerGemini, geminiFastModel)
	reportAIModel(first, providerGemini, "gemini-3-pro")
	wantModelLine(t, pane, "Gemini (Google) · "+geminiFastModel, "the superseded Find's late report")
}

// THE SIDEBAR'S FIND WAIT LAYS A WRAPPED NAME OUT AS ITS REPORT LANDS.
// Mutation: aiSearchingView setting no relayout on its line.
func TestTheSidebarFindWaitLaysOutALongModel(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	app := test.NewApp()
	defer app.Quit()
	stubAIGenerate(t)
	st := sidebarFindState(t)
	pane, ctx := findSidebarSearchPane(t, st)
	area := container.NewStack(pane) // the reading pane the results replace
	area.Resize(fyne.NewSize(600, 700))
	wantLineLaidOut(t, area, "control: the empty line")
	reportAIModel(ctx, providerGemini, long)
	wantModelLine(t, area, "Gemini (Google) · "+long, "the long name reported")
	if h := func() float32 { b, _ := modelLineBox(area); return b.MinSize().Height }(); h < captionLine()+captionRow() {
		t.Fatalf("control: the long name must wrap; the line wants %vpt", h)
	}
	wantLineLaidOut(t, area, "the long name, as its report lands")
}

// --- The Find waits in a short space ------------------------------------------

// pathTo is the chain of objects from root down to target, through containers
// and scrolls, or nil when target is not under root.
func pathTo(root, target fyne.CanvasObject) []fyne.CanvasObject {
	if root == target {
		return []fyne.CanvasObject{root}
	}
	var kids []fyne.CanvasObject
	switch v := root.(type) {
	case *fyne.Container:
		kids = v.Objects
	case *container.Scroll:
		kids = []fyne.CanvasObject{v.Content}
	}
	for _, k := range kids {
		if p := pathTo(k, target); p != nil {
			return append([]fyne.CanvasObject{root}, p...)
		}
	}
	return nil
}

// pathOffset is target's position inside the first object of path, which ends
// at target: the positions along the way added up. A scroll's content sits at
// minus its offset, so this is where target is drawn now.
func pathOffset(path []fyne.CanvasObject) fyne.Position {
	var p fyne.Position
	for _, o := range path[1:] {
		p = p.Add(o.Position())
	}
	return p
}

// resultsAreaOf is the area a Find wait is drawn in: the nearest stack above
// target holding one object, which on the Search tab is the results host
// under the Find field.
func resultsAreaOf(t *testing.T, root, target fyne.CanvasObject) *fyne.Container {
	t.Helper()
	path := pathTo(root, target)
	if path == nil {
		t.Fatal("the wait is not in the tab's tree of containers")
	}
	stack := reflect.TypeOf(layout.NewStackLayout())
	for i := len(path) - 2; i >= 0; i-- {
		if c, ok := path[i].(*fyne.Container); ok && len(c.Objects) == 1 && reflect.TypeOf(c.Layout) == stack {
			return c
		}
	}
	t.Fatal("no results area above the wait")
	return nil
}

// waitReach says where the reader can see el, part of a wait drawn in area.
// shown: el lies wholly inside area as it is laid out now. reachable: el is
// shown, or it lies wholly inside the laid-out content of a vertical scroll
// that itself lies inside area, no wider than the scroll and no taller than
// its view, so scrolling brings it wholly into view.
func waitReach(t *testing.T, area, el fyne.CanvasObject) (shown, reachable bool) {
	t.Helper()
	path := pathTo(area, el)
	if path == nil {
		t.Fatal("the element is not inside the results area")
	}
	const slack = 0.5
	within := func(p fyne.Position, sz, box fyne.Size) bool {
		return p.X >= -slack && p.Y >= -slack &&
			p.X+sz.Width <= box.Width+slack && p.Y+sz.Height <= box.Height+slack
	}
	es := el.Size()
	if within(pathOffset(path), es, area.Size()) {
		return true, true
	}
	for i := len(path) - 2; i >= 1; i-- {
		s, ok := path[i].(*container.Scroll)
		if !ok || s.Direction != container.ScrollVerticalOnly {
			continue
		}
		if !within(pathOffset(path[:i+1]), s.Size(), area.Size()) {
			continue // the scroll itself runs out of the area
		}
		p := pathOffset(path[i+1:]) // el inside the scroll's content
		view := s.Size()
		if within(fyne.NewPos(p.X, p.Y), es, fyne.NewSize(view.Width, s.Content.Size().Height)) &&
			es.Height <= view.Height+slack {
			return false, true
		}
	}
	return false, false
}

// objectWithText is the first text object under root reading text: a
// canvas.Text, a Label or a RichText.
func objectWithText(root fyne.CanvasObject, text string) fyne.CanvasObject {
	var found fyne.CanvasObject
	walkTree(root, func(n fyne.CanvasObject) {
		if found != nil {
			return
		}
		switch v := n.(type) {
		case *canvas.Text:
			if v.Text == text {
				found = v
			}
		case *widget.Label:
			if v.Text == text {
				found = v
			}
		case *widget.RichText:
			if v.String() == text {
				found = v
			}
		}
	})
	return found
}

// findWaitParts is the wait's status line, then the rest of it the reader
// needs: the model line, the hint, Cancel and the faster-model offer.
func findWaitParts(t *testing.T, root fyne.CanvasObject) (fyne.CanvasObject, map[string]fyne.CanvasObject) {
	t.Helper()
	msg := objectWithText(root, findRunning)
	hint := objectWithText(root, hintText)
	box, _ := modelLineBox(root)
	cancel := findTreeButton(root, "Cancel")
	faster := findTreeLink(root, "Switch to a faster model")
	if msg == nil || hint == nil || box == nil || cancel == nil || faster == nil {
		t.Fatalf("the wait is missing a part: status %v, hint %v, model %v, Cancel %v, faster offer %v; texts %v",
			msg != nil, hint != nil, box != nil, cancel != nil, faster != nil, treeTexts(root))
	}
	return msg, map[string]fyne.CanvasObject{
		"the model line": box, "the hint": hint, "Cancel": cancel, "the faster-model offer": faster,
	}
}

// wantWaitInReach fails unless, in area, the wait's status line shows without
// scrolling and everything else in it can be scrolled to; when the area has
// room for the whole wait, everything shows at once.
func wantWaitInReach(t *testing.T, area fyne.CanvasObject, msg fyne.CanvasObject, parts map[string]fyne.CanvasObject, room bool) {
	t.Helper()
	if shown, _ := waitReach(t, area, msg); !shown {
		p := pathOffset(pathTo(area, msg))
		t.Errorf("%q is drawn at %vpt in a %vpt area: out of sight, under what borders the area",
			findRunning, p.Y, area.Size().Height)
	}
	for name, o := range parts {
		shown, reachable := waitReach(t, area, o)
		switch {
		case !reachable:
			p := pathOffset(pathTo(area, o))
			t.Errorf("%s is drawn at %v..%vpt in a %vpt area, and no scroll reaches it",
				name, p.Y, p.Y+o.Size().Height, area.Size().Height)
		case room && !shown:
			t.Errorf("%s needs a scroll though the area has room for the whole wait", name)
		}
	}
}

// columnOf is the wait's column: the nearest vertical box above msg.
func columnOf(t *testing.T, area, msg fyne.CanvasObject) *fyne.Container {
	t.Helper()
	path := pathTo(area, msg)
	vbox := reflect.TypeOf(layout.NewVBoxLayout())
	for i := len(path) - 2; i >= 0; i-- {
		if c, ok := path[i].(*fyne.Container); ok && reflect.TypeOf(c.Layout) == vbox {
			return c
		}
	}
	t.Fatal("the wait has no column")
	return nil
}

// ON A PHONE IN LANDSCAPE THE FIND WAIT KEEPS ITS STATUS IN SIGHT AND CANCEL
// IN REACH. An iPhone kept its bottom bar in landscape, and between the bar
// and the Find field the results area is about half the wait's height, the
// model line (the longest, wrapped, here) included. The wait's column was
// centred in that area and ran out of it at both ends: "Searching with AI…"
// and the bar under the Find field, Cancel and the faster-model offer under
// the tab bar, with nothing to scroll them back. It now starts at the area's
// top, in a scroll, and all of it is in reach. The rail that every phone now
// has sideways leaves more room than the bar did, but still not enough at any
// iPhone's or an Android phone's sideways size; the bar's cases stay for a
// window that draws the bar at those sizes. In portrait there is room, and
// the whole wait shows at once, centred, as it always has. Mutation: the
// Search tab's wait without its scroll (findWaitScroll).
func TestTheFindWaitKeepsCancelInReachOnALandscapePhone(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	for _, sc := range []struct {
		nav   string
		w, h  float32
		short bool // the results area is shorter than the wait
	}{
		{"bar", 375, 667, false},
		{"bar", 375, 812, false},
		{"bar", 667, 375, true},
		{"bar", 568, 320, true},
		{"bar", 844, 390, true},
		{"bar", 932, 430, true},
		{"rail", 568, 320, true},
		{"rail", 667, 375, true},
		{"rail", 800, 360, true},
		{"rail", 844, 390, true},
		{"rail", 932, 430, true},
		{"rail", 956, 440, true},
	} {
		t.Run(fmt.Sprintf("%s %vx%v", sc.nav, sc.w, sc.h), func(t *testing.T) {
			t.Setenv("BIBLETEXT_DESKTOP_TABS", sc.nav)
			h, held := findTab(t)
			sz := fyne.NewSize(sc.w, sc.h)
			h.state.window.Resize(sz)
			rebuildWindow(h.state)
			h.ask("where love is described")
			if len(*held) != 1 {
				t.Fatalf("control: want one Find in flight, have %d", len(*held))
			}
			(*held)[0].onModel(aiModelInUse{providerGemini, long})
			root := h.pane()
			root.Resize(sz) // a phone lays the tree out at its screen, whatever the tree asks for
			msg, parts := findWaitParts(t, root)
			area := resultsAreaOf(t, root, msg)
			room := columnOf(t, area, msg).MinSize().Height <= area.Size().Height
			if room == sc.short {
				t.Fatalf("control: the results area is %vpt for a %vpt wait; this case expects it short=%v",
					area.Size().Height, columnOf(t, area, msg).MinSize().Height, sc.short)
			}
			wantWaitInReach(t, area, msg, parts, room)
		})
	}
}

// THE DESKTOP FIND WAIT KEEPS CANCEL IN REACH IN A SHORT PANE. The same wait
// without the bar (aiSearchingView) is what the former sidebar layout draws,
// and what the shared layout's reading slot draws while a search holds it,
// in a window the reader can make as short as they like. Mutation: that wait
// without its scroll.
func TestTheDesktopFindWaitKeepsCancelInReachInAShortPane(t *testing.T) {
	const long = "gemini-2.5-flash-lite-preview-09-2025-thinking-experimental"
	app := test.NewApp()
	defer app.Quit()
	stubAIGenerate(t)
	st := sidebarFindState(t)
	pane, ctx := findSidebarSearchPane(t, st)
	reportAIModel(ctx, providerGemini, long)
	area := container.NewStack(pane) // the pane the results take over
	for _, sc := range []struct {
		h     float32
		short bool
	}{{700, false}, {120, true}} {
		area.Resize(fyne.NewSize(600, sc.h))
		msg, parts := findWaitParts(t, area)
		room := columnOf(t, area, msg).MinSize().Height <= area.Size().Height
		if room == sc.short {
			t.Fatalf("control: a %vpt pane for a %vpt wait; this case expects it short=%v",
				sc.h, columnOf(t, area, msg).MinSize().Height, sc.short)
		}
		wantWaitInReach(t, area, msg, parts, room)
	}
}

// --- Settings: Test key -------------------------------------------------------

// keyTestCall is one Test key request held at its seam.
type keyTestCall struct {
	ctx     context.Context
	release chan error
}

// holdKeyTests parks the model list the sheet fetches on opening, and holds
// every Test key request until the test releases it. listed hears each list
// fetch arrive, which the test waits for: that receive is what orders the
// fetch's goroutine, parked for good, before the cleanup restores the seam.
func holdKeyTests(t *testing.T) (calls chan keyTestCall, listed chan struct{}) {
	t.Helper()
	calls = make(chan keyTestCall, 4)
	listed = make(chan struct{}, 4)
	prevRun, prevList := aiKeyTestRun, aiSettingsListModels
	aiKeyTestRun = func(ctx context.Context, _ providerInfo, _ *keyStore, _ string) error {
		c := keyTestCall{ctx: ctx, release: make(chan error, 1)}
		calls <- c
		return <-c.release
	}
	never := make(chan struct{}) // intentionally never closed
	aiSettingsListModels = func(context.Context, providerInfo, string) ([]discoveredModel, error) {
		listed <- struct{}{}
		<-never
		return nil, errors.New("unreachable")
	}
	t.Cleanup(func() { aiKeyTestRun, aiSettingsListModels = prevRun, prevList })
	return calls, listed
}

// keyTestLine is the text under the key row's buttons, or nil.
func keyTestLine(p fyne.CanvasObject) *widget.RichText {
	var got *widget.RichText
	walkTree(p, func(n fyne.CanvasObject) {
		if rt, ok := n.(*widget.RichText); ok && got == nil && rt.Visible() {
			if s := segmentText(rt.Segments); strings.HasPrefix(s, "Testing…") ||
				strings.HasPrefix(s, "✓") || strings.HasPrefix(s, "✗") {
				got = rt
			}
		}
	})
	return got
}

// keyTestResult is the text under the key row's buttons.
func keyTestResult(p fyne.CanvasObject) string {
	if rt := keyTestLine(p); rt != nil {
		return segmentText(rt.Segments)
	}
	return ""
}

// TEST KEY'S WAIT NAMES THE MODEL THE TEST SENDS TO, in caption size and
// muted like the model caption above it, AND A SECOND TEST OWNS THE LINE.
// Mutations: the test putting no observer on its request; the model's
// segment unmuted; its sequence check on the report (the first test's late
// retry renames the second's wait); its sequence check on the verdict (the
// first test's "Key works" lands over the second's wait).
func TestTheKeyTestNamesTheModel(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	calls, listed := holdKeyTests(t)
	state := deferredTestState(t, app)
	state.aiKeys = newKeyStoreWith(newFakePrefs())
	state.aiKeys.setAPIKey(defaultProviderID, "test-key")
	showAISettings(state)
	select {
	case <-listed:
	case <-time.After(2 * time.Second):
		t.Fatal("control: the sheet did not fetch the model list for the saved key")
	}
	popup := settingsPopup(t, state)
	t.Cleanup(popup.Hide)
	testBtn := findTreeButton(popup, "Test key")
	if testBtn == nil {
		t.Fatalf("no Test key; texts %v", treeTexts(popup))
	}

	testBtn.OnTapped()
	first := <-calls
	if got := keyTestResult(popup); got != "Testing…" {
		t.Fatalf("before the request reports the wait reads %q, want just \"Testing…\"", got)
	}
	reportAIModel(first.ctx, providerGemini, geminiModel)
	if got, want := keyTestResult(popup), "Testing…"+"Gemini (Google) · "+geminiModel; got != want {
		t.Fatalf("the wait reads %q, want %q", got, want)
	}
	if segs := keyTestLine(popup).Segments; len(segs) == 2 {
		st := segs[1].(*widget.TextSegment).Style
		if st.SizeName != theme.SizeNameCaptionText || st.ColorName != colorNameMuted {
			t.Errorf("the model under \"Testing…\" is styled %+v; want caption size, muted", st)
		}
	} else {
		t.Fatalf("the wait has %d segments, want \"Testing…\" and the model", len(segs))
	}

	testBtn.OnTapped()
	second := <-calls
	if got := keyTestResult(popup); got != "Testing…" {
		t.Fatalf("a second test's wait reads %q before its request reports", got)
	}
	reportAIModel(second.ctx, providerGemini, "gemini-3-pro")
	reportAIModel(first.ctx, providerGemini, geminiFastModel) // the first test's late retry
	want := "Testing…" + "Gemini (Google) · gemini-3-pro"
	if got := keyTestResult(popup); got != want {
		t.Fatalf("after the first test's late report the wait reads %q, want %q", got, want)
	}

	// The first test's verdict, after the second test has started. Its
	// completion runs on the held call's goroutine (the test driver's fyne.Do
	// is inline); with the check it returns before touching the sheet, and
	// the wait lets it do so. Too short a wait weakens the check toward
	// vacuous, never toward a false failure.
	first.release <- nil
	time.Sleep(200 * time.Millisecond)
	if got := keyTestResult(popup); got != want {
		t.Fatalf("the first test's verdict landed over the second's wait: %q", got)
	}
}
