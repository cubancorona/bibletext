package bibletext

// Geometry, state-transition, keyboard, timing and side-inset gates for the
// shared AI waiting column, Study single-exit sheet, Find layouts and timeout
// recovery rows.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const aiGateLongModelID = "gemini-2.5-pro-preview-09-2025-thinking-budget-experimental"

var aiGateCanvases = []insetScreen{
	{"320x568", 320, 568, 20, 0, 0, 0},
	{"375x667", 375, 667, 20, 0, 0, 0},
	{"402x874", 402, 874, 62, 34, 0, 0},
	{"440x956", 440, 956, 62, 34, 0, 0},
	{"667x375", 667, 375, 0, 0, 0, 0},
	{"667x375-insets", 667, 375, 0, 21, 44, 44},
	{"834x1194", 834, 1194, 24, 20, 0, 0},
}

func seedKeyedStudyWindow(t *testing.T, s insetScreen, provider, model, effort string, efforts []string) (*AppState, fyne.Window) {
	t.Helper()
	st, w := insetWindow(t, s, 0)
	store := st.keys()
	store.setActiveProvider(provider)
	store.setAPIKey(provider, "synthetic-key")
	grantAIPermission(store, provider)
	if model != "" {
		store.setOverrideModel(provider, model)
	}
	if len(efforts) > 0 {
		m := model
		if m == "" {
			info, _ := providerByID(provider)
			m = info.Model
		}
		rememberModelEfforts(store.prefs, provider, []discoveredModel{{id: m, efforts: efforts}})
		if effort != "" {
			setAIReasoningEffort(store.prefs, provider, m, effort)
		}
	}
	return st, w
}

func seedKeyedFindWindow(t *testing.T, s insetScreen, provider, model, effort string, efforts []string) (*AppState, fyne.Window) {
	t.Helper()
	st, w := insetWindow(t, s, 2)
	store := st.keys()
	store.setActiveProvider(provider)
	store.setAPIKey(provider, "synthetic-key")
	grantAIPermission(store, provider)
	if model != "" {
		store.setOverrideModel(provider, model)
	}
	if len(efforts) > 0 {
		m := model
		if m == "" {
			info, _ := providerByID(provider)
			m = info.Model
		}
		rememberModelEfforts(store.prefs, provider, []discoveredModel{{id: m, efforts: efforts}})
		if effort != "" {
			setAIReasoningEffort(store.prefs, provider, m, effort)
		}
	}
	st.aiSearchMode = true
	root := &wrappingRoot{phoneRoot{content: buildCompactUI(st)}}
	root.ExtendBaseWidget(root)
	w.SetContent(container.New(safeAreaLayout{s, st.theme.Size(theme.SizeNamePadding)}, root))
	w.Resize(fyne.NewSize(s.w, s.h))
	return st, w
}

func compactPaneOf(w fyne.Window) fyne.CanvasObject {
	if c, ok := w.Content().(*fyne.Container); ok && len(c.Objects) == 1 {
		if wr, ok := c.Objects[0].(*wrappingRoot); ok {
			return wr.content
		}
	}
	if pr, ok := w.Content().(*phoneRoot); ok {
		return pr.content
	}
	return w.Content()
}

func submitCompactFind(t *testing.T, st *AppState, query string) {
	t.Helper()
	e := pageEntry(t, st, pageFieldFind)
	e.SetText(query)
	e.OnSubmitted(query)
}

func collectSpeedRows(root fyne.CanvasObject) (disc *aiSwitchDisclosure, rows []*aiSwitchActionRow) {
	walkVisibleControls(root, func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *aiSwitchDisclosure:
			disc = v
		case *aiSwitchActionRow:
			rows = append(rows, v)
		}
	})
	return disc, rows
}

// Every choice row and disclosure toggle is at least 44pt tall and spans
// the waiting column's measure; taps 0.5pt inside each edge fire it while taps
// 0.5pt outside do not; the Study citation row is a full-width >=44pt target.
func TestAIWaitChoiceRowsAreWholeTapTargets(t *testing.T) {
	armAISpeedClock(t)
	studies := stubAIActionParked(t)
	for _, sc := range aiGateCanvases {
		t.Run("Study/"+sc.name, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			showAIPanel(st, aiActionExplain, aiLongSelection, "", selSpan{})
			waitParked(t, studies, "study")
			popup := topPopup(t, w)
			defer popup.Hide()

			var colBox *fyne.Container
			walkTree(popup, func(o fyne.CanvasObject) {
				if c, ok := o.(*fyne.Container); ok {
					if _, ok := c.Layout.(*aiWaitLayout); ok {
						colBox = c
					}
				}
			})
			if colBox == nil {
				t.Fatal("missing aiWaitLayout container")
			}
			wantMeasure := min(aiWaitMeasure, colBox.Size().Width)
			disc, _ := collectSpeedRows(popup)
			if disc == nil {
				t.Fatal("missing Faster options disclosure")
			}
			if math.Abs(float64(disc.Size().Width-wantMeasure)) > 0.5 || disc.Size().Height < minTapTarget-0.1 {
				t.Errorf("disclosure size %v, want width %.1f height >= %.1f", disc.Size(), wantMeasure, minTapTarget)
			}
			disc.Tapped(nil)
			_, rows := collectSpeedRows(popup)
			if len(rows) != 2 {
				t.Fatalf("expected 2 expanded rows, got %d", len(rows))
			}
			for i, r := range rows {
				if math.Abs(float64(r.Size().Width-wantMeasure)) > 0.5 || r.Size().Height < minTapTarget-0.1 {
					t.Errorf("row[%d] size %v, want width %.1f height >= %.1f", i, r.Size(), wantMeasure, minTapTarget)
				}
			}

			// Citation row + disclosure button span the full header width at >= 44pt height.
			var citeTap *aiCitationTapArea
			walkTree(popup, func(o fyne.CanvasObject) {
				if ct, ok := o.(*aiCitationTapArea); ok {
					citeTap = ct
				}
			})
			discBtn := quoteDisclosure(t, popup)
			if citeTap == nil || citeTap.Size().Height < minTapTarget-0.1 || citeTap.Size().Width+discBtn.Size().Width < colBox.Size().Width-0.5 {
				t.Fatalf("citation tap row missing or undersized: tap=%v btn=%v colW=%.1f", citeTap, discBtn.Size(), colBox.Size().Width)
			}
		})
	}

	// Ring-tap test: 0.5pt outside each edge misses; 0.5pt inside each edge hits.
	t.Run("ring-taps", func(t *testing.T) {
		app := test.NewTempApp(t)
		fyne.SetCurrentApp(app)
		th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
		app.Settings().SetTheme(th)
		hits := 0
		row := newAISwitchAction(nil, "Use less reasoning this time", "Medium effort", func() { hits++ })
		host := container.NewWithoutLayout(row.box)
		row.box.Move(fyne.NewPos(40, 40))
		row.box.Resize(fyne.NewSize(260, 44))
		row.box.Layout.Layout(row.box.Objects, fyne.NewSize(260, 44))
		w := app.NewWindow("ring")
		w.SetPadded(false)
		w.SetContent(host)
		w.Resize(fyne.NewSize(340, 140))
		t.Cleanup(w.Close)

		drv := app.Driver()
		inset, _ := w.Canvas().InteractiveArea()
		pos := drv.AbsolutePositionForObject(row).Add(inset)
		sz := row.Size()
		outside := []fyne.Position{
			{X: pos.X - 0.5, Y: pos.Y + sz.Height/2},
			{X: pos.X + sz.Width + 0.5, Y: pos.Y + sz.Height/2},
			{X: pos.X + sz.Width/2, Y: pos.Y - 0.5},
			{X: pos.X + sz.Width/2, Y: pos.Y + sz.Height + 0.5},
		}
		for _, pt := range outside {
			test.TapCanvas(w.Canvas(), pt)
			if hits != 0 {
				t.Fatalf("outside point %v triggered row", pt)
			}
		}
		inside := []fyne.Position{
			{X: pos.X + 0.5, Y: pos.Y + sz.Height/2},
			{X: pos.X + sz.Width - 0.5, Y: pos.Y + sz.Height/2},
			{X: pos.X + sz.Width/2, Y: pos.Y + 0.5},
			{X: pos.X + sz.Width/2, Y: pos.Y + sz.Height - 0.5},
		}
		for i, pt := range inside {
			test.TapCanvas(w.Canvas(), pt)
			if hits != i+1 {
				t.Fatalf("inside point %v missed row (hits=%d, want %d)", pt, hits, i+1)
			}
		}
	})
}

// Vertical rhythm, 4pt x 240pt slim bar dimensions, 18pt status text in
// pal.Text, and 14pt quiet disclosure chevron hold within ±0.5pt on both Study
// and Find.
func TestAIWaitColumnVerticalRhythm(t *testing.T) {
	studies := stubAIActionParked(t)
	stubAIGenerate(t)

	checkColumn := func(t *testing.T, root fyne.CanvasObject, pal palette, isFind bool) {
		t.Helper()
		var col *aiWaitColumn
		walkTree(root, func(o fyne.CanvasObject) {
			if c, ok := o.(*fyne.Container); ok {
				if wl, ok := c.Layout.(*aiWaitLayout); ok {
					col = wl.col
				}
			}
		})
		if col == nil {
			t.Fatal("missing aiWaitColumn")
		}
		if col.status.TextSize != 18 || col.status.Color != pal.Text {
			t.Errorf("status style size=%v color=%v, want 18 %v", col.status.TextSize, col.status.Color, pal.Text)
		}
		if math.Abs(float64(col.status.Position().Y-aiWaitTop)) > 0.5 {
			t.Errorf("status top = %.2f, want %.2f", col.status.Position().Y, aiWaitTop)
		}
		statusFoot := col.status.Position().Y + col.status.Size().Height
		barGap := col.barCell.Position().Y - statusFoot
		if math.Abs(float64(barGap-aiWaitStatusToBar)) > 0.5 {
			t.Errorf("status-to-bar = %.2f, want %.2f", barGap, aiWaitStatusToBar)
		}
		if math.Abs(float64(col.bar.Size().Height-4.0)) > 0.5 || math.Abs(float64(col.bar.Size().Width-240.0)) > 0.5 {
			t.Errorf("bar size = %v, want 240x4pt", col.bar.Size())
		}
		pad := theme.InnerPadding()
		barFoot := col.barCell.Position().Y + col.barCell.Size().Height
		captionTop := col.line.box.Position().Y + pad
		if math.Abs(float64((captionTop-barFoot)-aiWaitBarToCaption)) > 0.5 {
			t.Errorf("bar-to-caption = %.2f, want %.2f", captionTop-barFoot, aiWaitBarToCaption)
		}
		hintTop := col.hint.Position().Y + pad
		if math.Abs(float64((hintTop-captionTop)-aiWaitCaptionPitch)) > 0.5 {
			t.Errorf("caption pitch = %.2f, want %.2f", hintTop-captionTop, aiWaitCaptionPitch)
		}
		captionFoot := hintTop + captionRow()
		if isFind {
			cancelGap := col.cancel.Position().Y - captionFoot
			if math.Abs(float64(cancelGap-aiWaitCaptionToCancel)) > 0.5 {
				t.Errorf("caption-to-cancel = %.2f, want %.2f", cancelGap, aiWaitCaptionToCancel)
			}
			cancelFoot := col.cancel.Position().Y + col.cancel.Size().Height
			choicesGap := col.speed.box.Position().Y - cancelFoot
			if math.Abs(float64(choicesGap-aiWaitCancelToChoices)) > 0.5 {
				t.Errorf("cancel-to-choices = %.2f, want %.2f", choicesGap, aiWaitCancelToChoices)
			}
		} else {
			choicesGap := col.speed.box.Position().Y - captionFoot
			if math.Abs(float64(choicesGap-aiWaitCaptionToChoices)) > 0.5 {
				t.Errorf("caption-to-choices = %.2f, want %.2f", choicesGap, aiWaitCaptionToChoices)
			}
		}
		disc := waitingDisclosure(root)
		if disc == nil {
			t.Fatal("missing Faster options disclosure")
		}
		if math.Abs(float64(disc.arrow.Size().Width-14.0)) > 0.5 || math.Abs(float64(disc.arrow.Size().Height-14.0)) > 0.5 {
			t.Errorf("disclosure chevron size = %v, want 14x14pt", disc.arrow.Size())
		}
		if disc.title.TextSize != 14 || disc.title.Color != pal.Text {
			t.Errorf("disclosure title size=%v color=%v, want 14 %v", disc.title.TextSize, disc.title.Color, pal.Text)
		}
	}

	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}
	stStudy, wStudy := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
	showAIPanel(stStudy, aiActionExplain, "For God so loved the world", "", selSpan{})
	waitParked(t, studies, "study rhythm")
	popup := topPopup(t, wStudy)
	sheetBox(t, popup)
	checkColumn(t, popup, stStudy.pal(), false)
	popup.Hide()

	stFind, wFind := seedKeyedFindWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
	submitCompactFind(t, stFind, "where love is described")
	nextCtx(t, aiCtxSeen)
	checkColumn(t, compactPaneOf(wFind), stFind.pal(), true)
}

// Card horizontal containment holds across phone widths, modal width stays
// identical within ±0.5pt across waiting, answer and error at 320..402pt, and
// the resting waiting column fits inside its scroll viewport.
func TestStudySheetContainmentAndConstantModalWidth(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	for _, width := range []float32{320, 360, 375, 402} {
		t.Run(fmt.Sprintf("width-%.0f", width), func(t *testing.T) {
			sc := insetScreen{"phone", width, 667, 20, 0, 0, 0}
			if width == 320 {
				sc.h = 568
			}
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			showAIPanel(st, aiActionExplain, aiLongSelection, "", selSpan{})
			req1 := nextCapturedStudyRequest(t, reqs, "waiting")
			popup := topPopup(t, w)
			defer popup.Hide()

			boxWait := sheetBoxOn(t, popup)
			waitW := boxWait.x1 - boxWait.x0
			if width == 320 && waitW > 280+0.5 {
				t.Errorf("320pt card width %.2f exceeds 280pt floor", waitW)
			}
			waitSc, _ := waitingLineIn(t, popup)
			if waitSc.Content.MinSize().Height > waitSc.Size().Height+0.5 {
				t.Errorf("at %.0fx%.0f, resting waiting column %.1fpt exceeds viewport %.1fpt",
					width, sc.h, waitSc.Content.MinSize().Height, waitSc.Size().Height)
			}
			for _, part := range sheetParts(popup) {
				if part.x0 < boxWait.x0-0.5 || part.x1 > boxWait.x1+0.5 {
					t.Errorf("part %s escapes card horizontal bounds %.1f..%.1f", part, boxWait.x0, boxWait.x1)
				}
			}

			finishCapturedStudyRequest(t, req1, "A concise explanation of the passage.", nil)
			boxAns := sheetBoxOn(t, popup)
			ansW := boxAns.x1 - boxAns.x0
			if math.Abs(float64(ansW-waitW)) > 0.5 {
				t.Errorf("width %.0f: answer width %.2f differs from waiting width %.2f", width, ansW, waitW)
			}

			popup.Hide()
			showAIPanel(st, aiActionExplain, aiLongSelection, "", selSpan{})
			req2 := nextCapturedStudyRequest(t, reqs, "error")
			finishCapturedStudyRequest(t, req2, "", syntheticAITimeout())
			popupErr := topPopup(t, w)
			defer popupErr.Hide()
			boxErr := sheetBoxOn(t, popupErr)
			errW := boxErr.x1 - boxErr.x0
			if math.Abs(float64(errW-waitW)) > 0.5 {
				t.Errorf("width %.0f: error width %.2f differs from waiting width %.2f", width, errW, waitW)
			}
		})
	}
}

type clipRect struct{ x0, y0, x1, y1 float32 }

func (c clipRect) intersect(o clipRect) clipRect {
	return clipRect{max(c.x0, o.x0), max(c.y0, o.y0), min(c.x1, o.x1), min(c.y1, o.y1)}
}

func assertNoHorizontalMidGlyphCut(t *testing.T, root fyne.CanvasObject, bounds clipRect) {
	t.Helper()
	var walk func(o fyne.CanvasObject, at fyne.Position, clip clipRect)
	walk = func(o fyne.CanvasObject, at fyne.Position, clip clipRect) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *container.Scroll:
			scClip := clip.intersect(clipRect{at.X, at.Y, at.X + v.Size().Width, at.Y + v.Size().Height})
			walk(v.Content, at.Add(v.Content.Position()), scClip)
			return
		case *container.Clip:
			cClip := clip.intersect(clipRect{at.X, at.Y, at.X + v.Size().Width, at.Y + v.Size().Height})
			walk(v.Content, at.Add(v.Content.Position()), cClip)
			return
		case *fyne.Container:
			for _, k := range v.Objects {
				walk(k, at.Add(k.Position()), clip)
			}
			return
		case *container.ThemeOverride:
			walk(v.Content, at.Add(v.Content.Position()), clip)
			return
		case *widget.RichText:
			for _, k := range test.WidgetRenderer(v).Objects() {
				walk(k, at.Add(k.Position()), clip)
			}
			return
		case *widget.Label:
			for _, k := range test.WidgetRenderer(v).Objects() {
				walk(k, at.Add(k.Position()), clip)
			}
			return
		case *canvas.Text:
			if strings.TrimSpace(v.Text) == "" {
				return
			}
			mw := fyne.MeasureText(v.Text, v.TextSize, v.TextStyle).Width
			x0 := at.X
			if v.Alignment == fyne.TextAlignCenter && v.Size().Width > mw {
				x0 = at.X + (v.Size().Width-mw)/2
			} else if v.Alignment == fyne.TextAlignTrailing && v.Size().Width > mw {
				x0 = at.X + v.Size().Width - mw
			}
			x1 := x0 + mw
			if x0 < clip.x0-0.75 || x1 > clip.x1+0.75 {
				t.Errorf("text %q (%.1f..%.1f) clipped horizontally by %.1f..%.1f", v.Text, x0, x1, clip.x0, clip.x1)
			}
		}
	}
	walk(root, root.Position(), bounds)
}

// Long model IDs, token budgets, Extra high effort, 18-repeat Ask questions
// and long citations wrap without horizontal mid-glyph clipping.
func TestAIWaitLongStringsWrapWithoutMidGlyphCut(t *testing.T) {
	armAISpeedClock(t)
	studies := stubAIActionParked(t)
	longQuestion := strings.TrimSpace(strings.Repeat("Who is meant by the world here, and how far does the love reach? ", 18))
	for _, sc := range aiGateCanvases {
		for _, effort := range []string{"xhigh", "24576"} {
			t.Run(sc.name+"/"+effort, func(t *testing.T) {
				levels := []string{"low", "high", "xhigh"}
				if effort == "24576" {
					levels = []string{"8192", "24576"}
				}
				st, w := seedKeyedStudyWindow(t, sc, providerGemini, aiGateLongModelID, effort, levels)
				passage := syntheticAskPassage()
				passage.Ref = "2 Thessalonians 2:1–17"
				passage.VersionAbbrev = "SYNTHETIC-LONG"
				showAIPanelForPassage(st, aiActionAsk, passage, longQuestion, selSpan{})
				ctx := nextCtx(t, studies)
				reportAIModel(ctx, speedTargetReport(st.keys().prefs, providerGemini, aiGateLongModelID))
				popup := topPopup(t, w)
				defer popup.Hide()

				disc, _ := collectSpeedRows(popup)
				if disc == nil {
					t.Fatal("missing Faster options disclosure for long model + lower effort")
				}
				disc.Tapped(nil)
				box := sheetBoxOn(t, popup)
				assertNoHorizontalMidGlyphCut(t, popup.Content, clipRect{box.x0 - 0.5, box.y0 - 0.5, box.x1 + 0.5, box.y1 + 0.5})
			})
		}
	}
}

// Expanding Faster options keeps Find's Cancel fixed at 0.0pt shift (with both
// unfolded rows in sight on >=375pt portrait screens, and reachable by scroll on
// shorter viewports), while Study scrolls both rows into view and keeps an open
// multi-verse quote expanded.
func TestFasterOptionsExpansionKeepsFindCancelFixed(t *testing.T) {
	armAISpeedClock(t)
	stubAIGenerate(t)
	for _, sc := range []insetScreen{
		{"320x568", 320, 568, 20, 0, 0, 0},
		{"375x667", 375, 667, 20, 0, 0, 0},
		{"402x874", 402, 874, 62, 34, 0, 0},
		{"440x956", 440, 956, 62, 34, 0, 0},
		{"667x375", 667, 375, 0, 21, 44, 44},
		{"834x1194", 834, 1194, 24, 20, 0, 0},
	} {
		t.Run("Find/"+sc.name, func(t *testing.T) {
			st, w := seedKeyedFindWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			submitCompactFind(t, st, "where mercy is promised")
			nextCtx(t, aiCtxSeen)

			pane := compactPaneOf(w)
			drv := fyne.CurrentApp().Driver()
			cancel := findTreeButton(pane, "Cancel")
			disc := waitingDisclosure(pane)
			if cancel == nil || disc == nil {
				t.Fatal("missing Cancel or Faster options")
			}
			if disc.arrow.Resource != iconDisclosureExpand {
				t.Fatal("expected expand chevron while collapsed")
			}
			cancelBefore := drv.AbsolutePositionForObject(cancel).Y
			discBefore := drv.AbsolutePositionForObject(disc).Y

			disc.Tapped(nil)
			if disc.arrow.Resource != iconDisclosureCollapse {
				t.Fatal("expected collapse chevron while expanded")
			}
			_, rows := collectSpeedRows(pane)
			if len(rows) != 2 {
				t.Fatalf("expected 2 revealed rows, got %d", len(rows))
			}

			// Cancel and Faster options never shift upward on expand on any screen.
			cancelAfter := drv.AbsolutePositionForObject(cancel).Y
			discAfter := drv.AbsolutePositionForObject(disc).Y
			if math.Abs(float64(cancelAfter-cancelBefore)) > 0.05 || math.Abs(float64(discAfter-discBefore)) > 0.05 {
				t.Errorf("expanding Faster options shifted Cancel %.2f->%.2f or toggle %.2f->%.2f",
					cancelBefore, cancelAfter, discBefore, discAfter)
			}

			scView := scrollHolding(pane, rows[0])
			if scView == nil {
				t.Fatal("rows not inside findWaitScroll")
			}
			if sc.w >= 375 && !sc.sideways() {
				if scView.Content.MinSize().Height > scView.Size().Height+0.5 {
					t.Errorf("portrait %s should fit open column without scrolling: col=%.1f > view=%.1f",
						sc.name, scView.Content.MinSize().Height, scView.Size().Height)
				}
			} else if scView.Content.MinSize().Height > scView.Size().Height {
				// On 320x568 or landscape viewports shorter than the open column,
				// Cancel remains fixed and scrolling reaches both unfolded rows.
				scView.ScrollToBottom()
			}
			scTop := drv.AbsolutePositionForObject(scView).Y
			scBottom := scTop + scView.Size().Height
			for i, r := range rows {
				ry := drv.AbsolutePositionForObject(r).Y
				if ry < scTop-0.5 || ry+r.Size().Height > scBottom+0.5 {
					t.Errorf("row[%d] %.1f..%.1f outside Find scroll viewport %.1f..%.1f",
						i, ry, ry+r.Size().Height, scTop, scBottom)
				}
			}
		})
	}

	// Study capped viewport: expanding Faster options scrolls both rows into view
	// without collapsing an open multi-verse passage quote.
	studies := stubAIActionParked(t)
	for _, sc := range []insetScreen{
		{"320x568", 320, 568, 20, 0, 0, 0},
		{"375x667", 375, 667, 20, 0, 0, 0},
		{"667x375", 667, 375, 0, 21, 44, 44},
	} {
		t.Run("Study/"+sc.name, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			showAIPanel(st, aiActionExplain, aiLongSelection, "", selSpan{})
			waitParked(t, studies, "study expand")
			popup := topPopup(t, w)
			defer popup.Hide()

			if !sc.sideways() && quoteDisclosure(t, popup).Icon != iconDisclosureCollapse {
				t.Fatal("multi-verse quote did not start expanded on portrait phone")
			}

			disc := waitingDisclosure(popup)
			if disc == nil {
				t.Fatal("missing Faster options on Study")
			}
			disc.Tapped(nil)
			if !sc.sideways() && quoteDisclosure(t, popup).Icon != iconDisclosureCollapse {
				t.Error("opening Faster options collapsed an open passage quote")
			}
			_, rows := collectSpeedRows(popup)
			if len(rows) != 2 {
				t.Fatalf("expected 2 rows, got %d", len(rows))
			}
			scView := scrollHolding(popup, rows[0])
			drv := fyne.CurrentApp().Driver()
			scTop := drv.AbsolutePositionForObject(scView).Y
			scBottom := scTop + scView.Size().Height
			for i, r := range rows {
				ry := drv.AbsolutePositionForObject(r).Y
				if ry < scTop-0.5 || ry+r.Size().Height > scBottom+0.5 {
					t.Errorf("Study row[%d] %.1f..%.1f outside viewport %.1f..%.1f",
						i, ry, ry+r.Size().Height, scTop, scBottom)
				}
			}
		})
	}
}

func snapshotPlacedRects(root fyne.CanvasObject) map[fyne.CanvasObject][4]float32 {
	out := map[fyne.CanvasObject][4]float32{}
	var walk func(o fyne.CanvasObject, at fyne.Position)
	walk = func(o fyne.CanvasObject, at fyne.Position) {
		if o == nil || !o.Visible() {
			return
		}
		sz := o.Size()
		out[o] = [4]float32{at.X, at.Y, sz.Width, sz.Height}
		switch v := o.(type) {
		case *fyne.Container:
			for _, k := range v.Objects {
				walk(k, at.Add(k.Position()))
			}
		case *container.Scroll:
			walk(v.Content, at.Add(v.Content.Position()))
		case *container.ThemeOverride:
			walk(v.Content, at.Add(v.Content.Position()))
		}
	}
	walk(root, root.Position())
	return out
}

// First model report for the configured model (across fast, capable, token-budget,
// one-time retry, and 60-character wrapped model IDs) moves no visible rectangle
// by more than 0.5pt.
func TestAIWaitFirstReportMovesNothing(t *testing.T) {
	studies := stubAIActionParked(t)
	stubAIGenerate(t)
	cases := []struct {
		name   string
		model  string
		effort string
		levels []string
		choice aiSpeedChoice
	}{
		{"fast-lowest", geminiFastModel, "low", []string{"low", "high"}, aiSpeedChoice{}},
		{"fast-lower-effort", geminiFastModel, "high", []string{"low", "high"}, aiSpeedChoice{}},
		{"fast-smaller-budget", geminiFastModel, "8192", []string{"2048", "8192"}, aiSpeedChoice{}},
		{"faster-model-only", "gemini-3-pro", "low", []string{"low", "high"}, aiSpeedChoice{}},
		{"both-options", "gemini-3-pro", "high", []string{"low", "medium", "high"}, aiSpeedChoice{}},
		{"long-60char", aiGateLongModelID, "24576", []string{"8192", "24576"}, aiSpeedChoice{}},
		{"one-time-choice", "gemini-3-pro", "high", []string{"low", "medium", "high"},
			aiSpeedChoice{provider: providerGemini, model: "gemini-3-pro", effort: "medium"}},
	}
	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}
	for _, tc := range cases {
		t.Run("Study/"+tc.name, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, tc.model, tc.effort, tc.levels)
			showAIPanelWithSpeed(st, aiActionExplain, aiPassage{Book: "John", Chapter: 3, Text: "For God so loved the world"}, "", selSpan{}, tc.choice)
			ctx := nextCtx(t, studies)
			popup := topPopup(t, w)
			defer popup.Hide()
			sheetBox(t, popup)

			before := snapshotPlacedRects(popup.Content)
			seed := seedAISpeedReport(st, tc.choice)
			reportAIModel(ctx, seed)
			after := snapshotPlacedRects(popup.Content)
			for obj, b := range before {
				a, ok := after[obj]
				if !ok {
					t.Fatalf("object %T vanished on first report", obj)
				}
				for k := range 4 {
					if math.Abs(float64(a[k]-b[k])) > 0.5 {
						t.Errorf("object %T rect changed from %v to %v on first report", obj, b, a)
						break
					}
				}
			}
		})

		t.Run("Find/"+tc.name, func(t *testing.T) {
			st, w := seedKeyedFindWindow(t, sc, providerGemini, tc.model, tc.effort, tc.levels)
			if tc.choice.model != "" {
				st.aiSearchQuery = "grace"
				// Trigger Find with the one-time choice via renderFind/startFind path.
				submitCompactFind(t, st, "grace")
				_ = nextCtx(t, aiCtxSeen)
				st.aiSearchChoice = tc.choice
				st.aiSearchLastModel = seedAISpeedReport(st, tc.choice)
				st.aiSearchModel = aiModelInUse{}
				if st.repaintFind != nil {
					st.repaintFind()
				}
			} else {
				submitCompactFind(t, st, "grace")
				_ = nextCtx(t, aiCtxSeen)
			}
			pane := compactPaneOf(w)
			before := snapshotPlacedRects(pane)
			showFindModel(st, st.askSession.gen, seedAISpeedReport(st, tc.choice))
			after := snapshotPlacedRects(pane)
			for obj, b := range before {
				a, ok := after[obj]
				if !ok {
					t.Fatalf("Find object %T vanished on first report", obj)
				}
				for k := range 4 {
					if math.Abs(float64(a[k]-b[k])) > 0.5 {
						t.Errorf("Find object %T rect changed from %v to %v on first report", obj, b, a)
						break
					}
				}
			}
		})
	}
}

// Study exposes a single framed Cancel in the footer while waiting (with
// disclaimer, Report and Copy hidden), switches to unframed Close + visible
// 11pt zero-padding disclaimer + Report + Copy on answer, and shows unframed
// Close alone on error.
func TestStudySheetFooterStatesAndDisclaimer(t *testing.T) {
	reqs := stubCapturedStudyRequests(t)
	exitFramed := func(popup *widget.PopUp, btn *widget.Button, pal palette) bool {
		framed := false
		walkTree(popup, func(o fyne.CanvasObject) {
			c, ok := o.(*fyne.Container)
			if !ok || len(c.Objects) != 2 || c.Objects[0] != btn {
				return
			}
			if r, ok := c.Objects[1].(*canvas.Rectangle); ok && r.Visible() && r.StrokeWidth == 1 && r.StrokeColor == pal.Border {
				framed = true
			}
		})
		return framed
	}
	findDisclaimer := func(popup *widget.PopUp) *container.ThemeOverride {
		var found *container.ThemeOverride
		walkTree(popup, func(o fyne.CanvasObject) {
			to, ok := o.(*container.ThemeOverride)
			if !ok {
				return
			}
			if rt, ok := to.Content.(*widget.RichText); ok && strings.Contains(segmentText(rt.Segments), "AI-generated") {
				found = to
			}
		})
		return found
	}

	for _, sc := range aiGateCanvases {
		t.Run(sc.name, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			pal := st.pal()
			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			req := nextCapturedStudyRequest(t, reqs, "exit")
			popup := topPopup(t, w)

			var buttons []*widget.Button
			walkVisibleControls(popup, func(o fyne.CanvasObject) {
				if b, ok := o.(*widget.Button); ok && b.Text != "" {
					buttons = append(buttons, b)
				}
			})
			if len(buttons) != 1 || buttons[0].Text != "Cancel" {
				t.Fatalf("expected single visible Cancel button while waiting, got %d (%v)", len(buttons), treeTexts(popup))
			}
			if !exitFramed(popup, buttons[0], pal) {
				t.Error("waiting Cancel button is missing visible 1pt pal.Border frame")
			}
			if d := findDisclaimer(popup); d == nil || d.Visible() {
				t.Error("disclaimer must exist and stay hidden while waiting")
			}
			box := sheetBoxOn(t, popup)
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(buttons[0])
			if pos.X < box.x0-0.5 || pos.X+buttons[0].Size().Width > box.x1+0.5 || pos.Y+buttons[0].Size().Height > box.y1+0.5 {
				t.Errorf("Cancel %v/%v outside card %v", pos, buttons[0].Size(), box)
			}
			buttons[0].OnTapped()
			if !errors.Is(req.ctx.Err(), context.Canceled) {
				t.Fatalf("footer Cancel did not cancel request context: %v", req.ctx.Err())
			}

			// Answer state: unframed Close, visible Report + Copy, visible 11pt zero-pad disclaimer.
			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			reqAns := nextCapturedStudyRequest(t, reqs, "answer")
			finishCapturedStudyRequest(t, reqAns, "A concise explanation.", nil)
			popupAns := topPopup(t, w)
			closeAns := findTreeButton(popupAns, "Close")
			if closeAns == nil || exitFramed(popupAns, closeAns, pal) {
				t.Error("answer Close button missing or unexpectedly framed")
			}
			repAns, cpAns := findTreeButton(popupAns, "Report"), findTreeButton(popupAns, "Copy")
			if repAns == nil || !repAns.Visible() || cpAns == nil || !cpAns.Visible() {
				t.Error("answer footer missing visible Report or Copy")
			}
			discAns := findDisclaimer(popupAns)
			if discAns == nil || !discAns.Visible() {
				t.Fatal("answer footer disclaimer is hidden")
			}
			if discAns.Theme.Size(theme.SizeNameCaptionText) != 11 || discAns.Theme.Size(theme.SizeNameInnerPadding) != 0 {
				t.Errorf("disclaimer theme caption=%v pad=%v, want 11 and 0",
					discAns.Theme.Size(theme.SizeNameCaptionText), discAns.Theme.Size(theme.SizeNameInnerPadding))
			}
			popupAns.Hide()

			// Error state: unframed Close, hidden disclaimer/Report/Copy.
			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			reqErr := nextCapturedStudyRequest(t, reqs, "error")
			finishCapturedStudyRequest(t, reqErr, "", syntheticAITimeout())
			popupErr := topPopup(t, w)
			closeErr := findTreeButton(popupErr, "Close")
			if closeErr == nil || !closeErr.Visible() || exitFramed(popupErr, closeErr, pal) {
				t.Error("error Close button missing or unexpectedly framed")
			}
			if d := findDisclaimer(popupErr); d != nil && d.Visible() {
				t.Error("error footer must hide disclaimer")
			}
			if r := findTreeButton(popupErr, "Report"); r != nil && r.Visible() {
				t.Error("error footer must hide Report")
			}
			if c := findTreeButton(popupErr, "Copy"); c != nil && c.Visible() {
				t.Error("error footer must hide Copy")
			}
			popupErr.Hide()
		})
	}
}

func barReachableViaContainersOnly(root fyne.CanvasObject) *widget.ProgressBarInfinite {
	var found *widget.ProgressBarInfinite
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || found != nil {
			return
		}
		switch v := o.(type) {
		case *widget.ProgressBarInfinite:
			found = v
		case *fyne.Container:
			for _, k := range v.Objects {
				walk(k)
			}
		case *container.Scroll:
			walk(v.Content)
		}
	}
	walk(root)
	return found
}

// ProgressBarInfinite is reachable through plain Containers and Scrolls alone
// (never hidden inside a custom widget or ThemeOverride), and stops on every
// Study and Find exit path.
func TestAIWaitBarsStopOnEveryExit(t *testing.T) {
	armAISpeedClock(t)
	reqs := stubCapturedStudyRequests(t)
	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}

	for _, exit := range []string{"cancel", "choice-then-cancel", "answer", "error"} {
		t.Run("Study/"+exit, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			req := nextCapturedStudyRequest(t, reqs, exit)
			popup := topPopup(t, w)
			bar := barReachableViaContainersOnly(popup.Content)
			if bar == nil || !bar.Running() {
				t.Fatal("Study ProgressBarInfinite not reachable via Containers/Scrolls or not running")
			}
			switch exit {
			case "cancel":
				findTreeButton(popup, "Cancel").OnTapped()
			case "choice-then-cancel":
				waitingDisclosure(popup).Tapped(nil)
				waitingAction(popup, "Use less reasoning this time").Tapped(nil)
				if bar.Running() {
					t.Fatal("previous Study bar stayed running after choice restart")
				}
				req2 := nextCapturedStudyRequest(t, reqs, "restarted")
				bar2 := barReachableViaContainersOnly(popup.Content)
				findTreeButton(popup, "Cancel").OnTapped()
				if bar2.Running() || req2.ctx.Err() == nil {
					t.Fatal("restarted Study bar stayed running after Cancel")
				}
			case "answer":
				finishCapturedStudyRequest(t, req, "Answer.", nil)
				defer popup.Hide()
			case "error":
				finishCapturedStudyRequest(t, req, "", syntheticAITimeout())
				defer popup.Hide()
			}
			if bar.Running() {
				t.Fatalf("Study bar stayed running after %s", exit)
			}
		})
	}

	for _, exit := range []string{"cancel", "answer", "error", "clear", "tab-switch", "assistant-none", "rebuild"} {
		t.Run("Find/"+exit, func(t *testing.T) {
			h, held := findTab(t)
			h.ask("mercy")
			bar := h.state.findBar
			if bar == nil || !bar.Running() {
				t.Fatal("Find bar not running")
			}
			// Also verify bar is reachable inside resultsHost via Containers/Scrolls alone.
			if barReachableViaContainersOnly(h.pane()) != bar {
				t.Fatal("Find bar not reachable via plain Containers and Scrolls")
			}
			switch exit {
			case "cancel":
				findTreeButton(h.pane(), "Cancel").OnTapped()
			case "answer":
				(*held)[0].done([]Verse{h.state.Bible.GetChapter("John", 1)[0]}, nil)
			case "error":
				(*held)[0].done(nil, syntheticAITimeout())
			case "clear":
				abandonAISearch(h.state)
			case "tab-switch":
				abandonAISearch(h.state)
				h.state.CurrentTab = 0
				rebuildWindow(h.state)
			case "assistant-none":
				h.state.keys().setAIEnabled(false)
				clearAISearchContext(h.state)
			case "rebuild":
				h.flip()
				defer abandonAISearch(h.state)
			}
			if bar.Running() {
				t.Fatalf("Find bar stayed running after %s", exit)
			}
		})
	}
}

// Taps and keyboard activation at +499ms after mounting a choice row are
// ignored while +500ms fires cleanly, and a follow-up tap within 500ms of the
// restarted wait neither triggers a second restart nor mutates Settings.
func TestAIWaitTapArmDelayBlocksEarlyTapsAndKeys(t *testing.T) {
	prevClock := aiSpeedNow
	now := time.Unix(1_700_000_000, 0)
	aiSpeedNow = func() time.Time { return now }
	t.Cleanup(func() { aiSpeedNow = prevClock })

	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}
	st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "high"})
	sends := routeStudy(t, "")
	showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
	first := nextSend(t, sends)
	if first.model != "gemini-3-pro" || first.effort != "high" {
		t.Fatalf("initial send %+v", first)
	}
	popup := topPopup(t, w)
	defer popup.Hide()

	disc := waitingDisclosure(popup)
	disc.Tapped(nil)
	lowerRow := waitingAction(popup, "Use less reasoning this time")
	if lowerRow == nil {
		t.Fatal("missing lower-effort row")
	}

	drv := fyne.CurrentApp().Driver()
	inset, _ := w.Canvas().InteractiveArea()
	rowPos := drv.AbsolutePositionForObject(lowerRow).Add(inset)
	tapPt := fyne.NewPos(rowPos.X+lowerRow.Size().Width/2, rowPos.Y+lowerRow.Size().Height/2)

	// Up through +499ms after opening Faster options, both canvas taps and
	// keyboard Return/Space are ignored.
	now = now.Add(499 * time.Millisecond)
	test.TapCanvas(w.Canvas(), tapPt)
	lowerRow.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	lowerRow.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	select {
	case s := <-sends:
		t.Fatalf("unarmed tap/key at +499ms restarted early: %+v", s)
	default:
	}

	// At +500ms, the tap fires and restarts at Low effort, where ONLY the
	// standalone "Switch to a faster model" row now occupies the choice slot.
	now = now.Add(1 * time.Millisecond)
	test.TapCanvas(w.Canvas(), tapPt)
	second := nextSend(t, sends)
	if second.model != "gemini-3-pro" || second.effort != "low" {
		t.Fatalf("expected restart at low effort, got %+v", second)
	}
	if !treeHasText(popup, "Reading the passage again…") {
		t.Fatalf("missing restart acknowledgement line; texts %v", treeTexts(popup))
	}

	// Tap the new standalone model-switch row within 120ms (<500ms): must NOT
	// switch Settings or start a third request.
	swRow := waitingAction(popup, "Switch to a faster model")
	if swRow == nil {
		t.Fatal("expected standalone faster-model row at Low effort")
	}
	swPos := drv.AbsolutePositionForObject(swRow).Add(inset)
	swPt := fyne.NewPos(swPos.X+swRow.Size().Width/2, swPos.Y+swRow.Size().Height/2)
	now = now.Add(120 * time.Millisecond)
	test.TapCanvas(w.Canvas(), swPt)
	if got := st.keys().overrideModel(providerGemini); got != "gemini-3-pro" {
		t.Fatalf("double-tap within 500ms mutated Settings model to %q", got)
	}
	select {
	case s := <-sends:
		t.Fatalf("double-tap within 500ms started third request: %+v", s)
	default:
	}

	// Once 500ms has elapsed since the restart, tapping switches cleanly.
	now = now.Add(500 * time.Millisecond)
	test.TapCanvas(w.Canvas(), swPt)
	third := nextSend(t, sends)
	if third.model != geminiFastModel || st.keys().overrideModel(providerGemini) != geminiFastModel {
		t.Fatalf("armed tap did not switch to %s: sent=%+v saved=%q", geminiFastModel, third, st.keys().overrideModel(providerGemini))
	}
}

type gateSyncSend struct {
	ctx    context.Context
	model  string
	effort string
	answer chan error
}

func (s *gateSyncSend) finish(err error) {
	s.answer <- err
	<-s.ctx.Done()
}

type gateSyncClient struct {
	model, effort string
	sends         chan<- *gateSyncSend
}

func (c gateSyncClient) generate(ctx context.Context, _ string) (string, error) {
	s := &gateSyncSend{ctx: ctx, model: c.model, effort: c.effort, answer: make(chan error, 1)}
	c.sends <- s
	if err := <-s.answer; err != nil {
		return "", err
	}
	return "1 John 4:8", nil
}

func routeSyncStudy(t *testing.T) chan *gateSyncSend {
	t.Helper()
	sends := make(chan *gateSyncSend, 8)
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, st *AppState, _, _, _ string, _ selSpan) (string, error) {
		store := st.keys()
		info, _ := providerByID(store.activeProvider())
		r := info.New(store, "synthetic-key").(*modelResolver)
		r.build = func(m, e string) aiClient { return gateSyncClient{model: m, effort: e, sends: sends} }
		out, err := r.generate(ctx, "study")
		if err != nil {
			return "", withAIErrorProvider(err, info)
		}
		return out, nil
	}
	t.Cleanup(func() { aiActionRun = prev })
	return sends
}

func routeSyncFind(t *testing.T) chan *gateSyncSend {
	t.Helper()
	isolateAICache(t)
	sends := make(chan *gateSyncSend, 8)
	prev := aiSearchGenerate
	aiSearchGenerate = func(ctx context.Context, info providerInfo, store *keyStore, _, prompt string) (string, error) {
		r := info.New(store, "synthetic-key").(*modelResolver)
		r.build = func(m, e string) aiClient { return gateSyncClient{model: m, effort: e, sends: sends} }
		return r.generate(ctx, prompt)
	}
	t.Cleanup(func() { aiSearchGenerate = prev })
	return sends
}

func nextGateSend(t *testing.T, sends chan *gateSyncSend) *gateSyncSend {
	t.Helper()
	select {
	case s := <-sends:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("no send reached provider")
		return nil
	}
}

// After a one-time lower effort fails, Try again preserves that valid choice;
// if the reader changes the saved effort or active model in Settings before
// tapping Try again, it sends the newly saved setting instead. Verified on
// Study and both Find layouts.
func TestTimeoutTryAgainPreservesValidEffortChoice(t *testing.T) {
	armAISpeedClock(t)

	t.Run("Study", func(t *testing.T) {
		sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}
		st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
		rememberModelEfforts(st.keys().prefs, providerGemini, []discoveredModel{
			{id: "gemini-3-pro", efforts: []string{"low", "medium", "high"}},
			{id: geminiFastModel, efforts: []string{"low", "high"}},
		})
		setAIReasoningEffort(st.keys().prefs, providerGemini, geminiFastModel, "low")
		sends := routeSyncStudy(t)
		showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
		first := nextGateSend(t, sends)
		popup := topPopup(t, w)
		defer popup.Hide()

		waitingDisclosure(popup).Tapped(nil)
		waitingAction(popup, "Use less reasoning this time").Tapped(nil)
		first.finish(context.Canceled)
		second := nextGateSend(t, sends)
		if second.effort != "medium" {
			t.Fatalf("expected medium one-time effort, got %q", second.effort)
		}
		second.finish(syntheticAITimeout())

		// Try again preserves the valid one-time medium choice.
		findTreeButton(popup, "Try again").OnTapped()
		third := nextGateSend(t, sends)
		if third.model != "gemini-3-pro" || third.effort != "medium" {
			t.Fatalf("Try again lost valid one-time choice: got %s/%s", third.model, third.effort)
		}

		// Fail again, change saved effort on the same model in Settings -> Try again sends newly saved effort.
		third.finish(syntheticAITimeout())
		setAIReasoningEffort(st.keys().prefs, providerGemini, "gemini-3-pro", "low")
		findTreeButton(popup, "Try again").OnTapped()
		fourth := nextGateSend(t, sends)
		if fourth.model != "gemini-3-pro" || fourth.effort != "low" {
			t.Fatalf("after Settings effort change, Try again sent %s/%s, want gemini-3-pro/low", fourth.model, fourth.effort)
		}

		// Fail again, change model in Settings, then Try again -> sends saved setting.
		fourth.finish(syntheticAITimeout())
		st.keys().setOverrideModel(providerGemini, geminiFastModel)
		findTreeButton(popup, "Try again").OnTapped()
		fifth := nextGateSend(t, sends)
		if fifth.model != geminiFastModel || fifth.effort != "low" {
			t.Fatalf("after Settings model change, Try again sent %s/%s, want %s/low", fifth.model, fifth.effort, geminiFastModel)
		}
	})

	for _, layoutName := range []string{"Search tab", "sidebar"} {
		t.Run("Find/"+layoutName, func(t *testing.T) {
			sends := routeSyncFind(t)
			var state *AppState
			var pane func() fyne.CanvasObject
			var ask func(string)
			if layoutName == "Search tab" {
				h := liveFindTab(t)
				state, pane, ask = h.state, h.pane, h.ask
			} else {
				app := test.NewTempApp(t)
				t.Cleanup(app.Quit)
				app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
				state = sidebarFindState(t)
				var current fyne.CanvasObject
				state.showReading = func() { current = buildSearchResultsView(state) }
				pane = func() fyne.CanvasObject { return current }
				ask = func(q string) { state.aiSearchQuery = q; state.retryAISearch(aiSpeedChoice{}) }
			}
			store := state.keys()
			store.setOverrideModel(providerGemini, "gemini-3-pro")
			rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{
				{id: "gemini-3-pro", efforts: []string{"low", "medium", "high"}},
				{id: geminiFastModel, efforts: []string{"low", "high"}},
			})
			setAIReasoningEffort(store.prefs, providerGemini, "gemini-3-pro", "high")
			setAIReasoningEffort(store.prefs, providerGemini, geminiFastModel, "low")

			ask("where peace is promised")
			first := nextGateSend(t, sends)
			waitingDisclosure(pane()).Tapped(nil)
			waitingAction(pane(), "Use less reasoning this time").Tapped(nil)
			first.finish(context.Canceled)
			second := nextGateSend(t, sends)
			if second.effort != "medium" {
				t.Fatalf("expected medium, got %q", second.effort)
			}
			second.finish(syntheticAITimeout())

			findTreeButton(pane(), "Try again").OnTapped()
			third := nextGateSend(t, sends)
			if third.model != "gemini-3-pro" || third.effort != "medium" {
				t.Fatalf("%s Try again lost valid one-time choice: %s/%s", layoutName, third.model, third.effort)
			}

			third.finish(syntheticAITimeout())
			setAIReasoningEffort(store.prefs, providerGemini, "gemini-3-pro", "low")
			findTreeButton(pane(), "Try again").OnTapped()
			fourth := nextGateSend(t, sends)
			if fourth.model != "gemini-3-pro" || fourth.effort != "low" {
				t.Fatalf("%s Try again after Settings effort change sent %s/%s, want gemini-3-pro/low", layoutName, fourth.model, fourth.effort)
			}

			fourth.finish(syntheticAITimeout())
			store.setOverrideModel(providerGemini, geminiFastModel)
			findTreeButton(pane(), "Try again").OnTapped()
			fifth := nextGateSend(t, sends)
			if fifth.model != geminiFastModel || fifth.effort != "low" {
				t.Fatalf("%s Try again after Settings model change sent %s/%s, want %s/low", layoutName, fifth.model, fifth.effort, geminiFastModel)
			}
			abandonAISearch(state)
		})
	}
}

// Timeout mounts framed Try again and centred open recovery rows (both,
// model-only, and effort-only without claiming a faster model in prose);
// lighter overrides do not offer FastModel; and keyStore.seq() advances
// monotonically on credential or model updates.
func TestTimeoutRecoveryRowsAndCapableModelOffer(t *testing.T) {
	armAISpeedClock(t)
	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}

	t.Run("both-open-on-timeout", func(t *testing.T) {
		st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
		pal := st.pal()
		sends := routeSyncStudy(t)
		showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
		first := nextGateSend(t, sends)
		first.finish(syntheticAITimeout())
		popup := topPopup(t, w)
		defer popup.Hide()

		tryBtn := findTreeButton(popup, "Try again")
		if tryBtn == nil {
			t.Fatal("missing Try again button on timeout")
		}
		tryFramed := false
		walkTree(popup, func(o fyne.CanvasObject) {
			c, ok := o.(*fyne.Container)
			if !ok || len(c.Objects) != 2 || c.Objects[0] != tryBtn {
				return
			}
			if r, ok := c.Objects[1].(*canvas.Rectangle); ok && r.Visible() && r.StrokeWidth == 1 && r.StrokeColor == pal.Border && r.CornerRadius == 10 {
				tryFramed = true
			}
		})
		if !tryFramed {
			t.Error("timeout Try again button is missing 1pt pal.Border frame")
		}

		disc, rows := collectSpeedRows(popup)
		if disc != nil || len(rows) != 2 {
			t.Fatalf("timeout should render both rows directly open without Faster options: disc=%v rows=%d", disc != nil, len(rows))
		}
		drv := fyne.CurrentApp().Driver()
		tryMid := drv.AbsolutePositionForObject(tryBtn).X + tryBtn.Size().Width/2
		for i, r := range rows {
			rx := drv.AbsolutePositionForObject(r).X
			rowMid := rx + r.Size().Width/2
			if math.Abs(float64(rowMid-tryMid)) > 0.5 {
				t.Errorf("timeout row[%d] centre %.1f differs from Try again centre %.1f", i, rowMid, tryMid)
			}
		}
		if rows[0].titleText != "Try again with less reasoning" || rows[0].detail != "Medium effort" {
			t.Errorf("unexpected row[0]: %q / %q", rows[0].titleText, rows[0].detail)
		}
		if rows[1].titleText != "Switch to a faster model and try again" || rows[1].detail != geminiFastModel+" · saves in Settings" {
			t.Errorf("unexpected row[1]: %q / %q", rows[1].titleText, rows[1].detail)
		}
		rows[0].Tapped(nil)
		second := nextGateSend(t, sends)
		if second.model != "gemini-3-pro" || second.effort != "medium" {
			t.Fatalf("timeout lower-effort row sent %+v", second)
		}
	})

	t.Run("effort-only-timeout-mounts-row", func(t *testing.T) {
		st, w := seedKeyedStudyWindow(t, sc, providerGemini, geminiFastModel, "24576", []string{"8192", "24576"})
		sends := routeSyncStudy(t)
		showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
		first := nextGateSend(t, sends)
		first.finish(syntheticAITimeout())
		popup := topPopup(t, w)
		defer popup.Hide()

		disc, rows := collectSpeedRows(popup)
		if disc != nil || len(rows) != 1 || rows[0].titleText != "Try again with less reasoning" || rows[0].detail != "8,192 tokens" {
			t.Fatalf("effort-only timeout failed to mount single lower-budget row: disc=%v rows=%d texts=%v", disc != nil, len(rows), treeTexts(popup))
		}
		if treeHasText(popup, "Gemini took too long to answer, so the request was stopped. Try again, or switch to a faster model.") {
			t.Fatal("effort-only timeout falsely claimed a faster model in the message")
		}
		if !treeHasText(popup, "Gemini took too long to answer, so the request was stopped. Try again.") {
			t.Fatalf("missing effort-only timeout message: %v", treeTexts(popup))
		}
	})

	t.Run("lighter-overrides-refuse-faster-offer", func(t *testing.T) {
		for _, tc := range []struct {
			provider, override string
			wantOffer          bool
		}{
			{providerGemini, "", true},
			{providerGemini, "gemini-3-pro", true},
			{providerGemini, "gemini-2.5-flash-lite", false},
			{providerGemini, "gemini-2.0-flash", false},
			{providerOpenAI, "gpt-5.4-pro", true},
			{providerOpenAI, "gpt-5-nano", false},
			{providerOpenAI, "gpt-4.1-mini", false},
			{providerAnthropic, "claude-opus-4-7", true},
			{providerAnthropic, "claude-3-5-haiku-latest", false},
			{providerGrok, "grok-4.20-0309-reasoning", true},
			{providerGrok, "grok-4.20-0309-non-reasoning", false},
			{providerGrok, "grok-3-mini", false},
		} {
			st := sampleState()
			st.aiKeys = newTestKeyStore()
			st.keys().setActiveProvider(tc.provider)
			st.keys().setAPIKey(tc.provider, "k")
			if tc.override != "" {
				st.keys().setOverrideModel(tc.provider, tc.override)
			}
			_, _, _, ok := fasterModelOffer(st)
			if ok != tc.wantOffer {
				t.Errorf("%s override %q: fasterModelOffer=%v, want %v", tc.provider, tc.override, ok, tc.wantOffer)
			}
		}
	})

	t.Run("keystore-mutation-sequence", func(t *testing.T) {
		ks := newTestKeyStore()
		s0 := ks.seq()
		ks.setAPIKey(providerGemini, "k1")
		s1 := ks.seq()
		ks.setOverrideModel(providerGemini, "gemini-3-pro")
		s2 := ks.seq()
		if !(s0 < s1 && s1 < s2) {
			t.Fatalf("monotonic counter did not advance: %d -> %d -> %d", s0, s1, s2)
		}
	})
}

// Every speed control takes focus, updates its focus ring, activates on Space
// and Return, and leaves focus on a live control after a restart.
func TestAIWaitKeyboardFocusAndActivation(t *testing.T) {
	armAISpeedClock(t)
	sc := insetScreen{"402x874", 402, 874, 62, 34, 0, 0}
	st, w := seedKeyedStudyWindow(t, sc, providerGemini, "gemini-3-pro", "high", []string{"low", "medium", "high"})
	sends := routeStudy(t, "")
	showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
	first := nextSend(t, sends)
	popup := topPopup(t, w)
	defer popup.Hide()

	cnv := w.Canvas()
	disc := waitingDisclosure(popup)
	cnv.Focus(disc)
	if cnv.Focused() != disc || disc.ring.StrokeWidth != 2 {
		t.Fatal("disclosure did not show focus ring on focus")
	}
	disc.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	_, rows := collectSpeedRows(popup)
	if len(rows) != 2 {
		t.Fatal("Return did not expand Faster options")
	}

	cnv.Focus(rows[0])
	if cnv.Focused() != rows[0] || rows[0].ring.StrokeWidth != 2 {
		t.Fatal("choice row did not show focus ring")
	}
	rows[0].TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	first.answer <- context.Canceled
	second := nextSend(t, sends)
	if second.effort != "medium" {
		t.Fatalf("Return on lower-effort row sent %+v", second)
	}
	focused := cnv.Focused()
	if focused == nil || focused == fyne.Focusable(rows[0]) {
		t.Fatalf("focus stayed nil or on detached row after restart: %T", focused)
	}
	if obj, ok := focused.(fyne.CanvasObject); ok && !objectUnder(popup, obj) {
		t.Fatal("post-restart focus is not inside the live popup")
	}
	findTreeButton(popup, "Cancel").OnTapped()
	second.answer <- context.Canceled
}

// Nothing drawn by a keyed Study or Find wait reaches under a landscape phone's
// side insets on 956x440, 874x402 or 803x360, and desktop Study waits clear or
// cleanly cover the header.
func TestAIWaitRespectsLandscapeSideInsetsAndHeader(t *testing.T) {
	armAISpeedClock(t)
	studies := stubAIActionParked(t)
	stubAIGenerate(t)
	screens := []insetScreen{
		proMaxSideways,
		{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
		{"Android cutout left", 803, 360, 0, 0, 21, 0},
		{"Android nav bar right", 803, 360, 0, 21, 0, 42},
	}
	for _, sc := range screens {
		t.Run("Study/"+sc.name, func(t *testing.T) {
			st, w := seedKeyedStudyWindow(t, sc, providerGemini, aiGateLongModelID, "high", []string{"low", "medium", "high"})
			showAIPanel(st, aiActionExplain, aiLongSelection, "", selSpan{})
			waitParked(t, studies, "inset study")
			popup := topPopup(t, w)
			defer popup.Hide()

			if hits := underASideInset(sc, sheetParts(popup)); len(hits) > 0 {
				t.Errorf("closed Study wait under side inset: %v", hits)
			}
			waitingDisclosure(popup).Tapped(nil)
			if hits := underASideInset(sc, sheetParts(popup)); len(hits) > 0 {
				t.Errorf("open Study wait under side inset: %v", hits)
			}
		})

		t.Run("Find/"+sc.name, func(t *testing.T) {
			st, w := seedKeyedFindWindow(t, sc, providerGemini, aiGateLongModelID, "high", []string{"low", "medium", "high"})
			submitCompactFind(t, st, "where wisdom speaks")
			nextCtx(t, aiCtxSeen)
			pane := compactPaneOf(w)
			waitingDisclosure(pane).Tapped(nil)

			drv := fyne.CurrentApp().Driver()
			_, rows := collectSpeedRows(pane)
			for i, r := range rows {
				x := drv.AbsolutePositionForObject(r).X
				if x < sc.left-0.5 || x+r.Size().Width > sc.w-sc.right+0.5 {
					t.Errorf("Find row[%d] %.1f..%.1f reaches under side inset (safe %.1f..%.1f)",
						i, x, x+r.Size().Width, sc.left, sc.w-sc.right)
				}
			}
		})
	}

	// Keyed desktop header clearance/covering across standard and short windows.
	app := newDesktopApp(t)
	for _, win := range []fyne.Size{
		{Width: 1280, Height: 800},
		{Width: 1280, Height: 440},
		{Width: 1280, Height: 401},
		{Width: 507, Height: 386},
		{Width: 1280, Height: 385},
	} {
		t.Run(fmt.Sprintf("DesktopHeader/%.0fx%.0f", win.Width, win.Height), func(t *testing.T) {
			app.asNew(t)
			st, w := app.window(t, win)
			store := st.keys()
			store.setActiveProvider(providerGemini)
			store.setAPIKey(providerGemini, "synthetic-key")
			grantAIPermission(store, providerGemini)
			store.setOverrideModel(providerGemini, "gemini-3-pro")
			rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{{id: "gemini-3-pro", efforts: []string{"low", "medium", "high"}}})
			setAIReasoningEffort(store.prefs, providerGemini, "gemini-3-pro", "high")

			showAIPanel(st, aiActionExplain, "For God so loved the world", "", selSpan{})
			waitParked(t, studies, "desktop header")
			popup := topPopup(t, w)
			defer popup.Hide()
			if win.Height >= 440 {
				wantClearOfHeader(t, st, w, popup)
			}
			_ = wantClearOrCovering(t, st, w, popup)
			waitingDisclosure(popup).Tapped(nil)
			if win.Height >= 440 {
				wantClearOfHeader(t, st, w, popup)
			}
			_ = wantClearOrCovering(t, st, w, popup)
		})
	}
}
