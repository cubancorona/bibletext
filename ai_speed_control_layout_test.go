//go:build !race

package bibletext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func scopeRows(o fyne.CanvasObject) (header *aiSwitchDisclosure, rows []*aiSwitchActionRow) {
	walkTree(o, func(obj fyne.CanvasObject) {
		switch r := obj.(type) {
		case *aiSwitchDisclosure:
			header = r
		case *aiSwitchActionRow:
			rows = append(rows, r)
		}
	})
	return
}

// Check the visible text runs after word wrapping. Comparing only the caption
// string would miss a clipped final line or a title overflowing its tap area.
func scopeRowText(t *testing.T, r *aiSwitchActionRow) string {
	t.Helper()
	var runs []string
	var visit func(fyne.CanvasObject)
	visit = func(root fyne.CanvasObject) {
		walkTree(root, func(o fyne.CanvasObject) {
			switch v := o.(type) {
			case *widget.RichText:
				for _, obj := range test.WidgetRenderer(v).Objects() {
					visit(obj)
				}
			case *widget.Label:
				for _, obj := range test.WidgetRenderer(v).Objects() {
					visit(obj)
				}
			case *canvas.Text:
				if !v.Visible() || strings.TrimSpace(v.Text) == "" {
					return
				}
				runs = append(runs, v.Text)
				pos := fyne.CurrentApp().Driver().AbsolutePositionForObject
				p, origin := pos(v), pos(r)
				measured := v.MinSize()
				if p.X < origin.X-0.75 || p.X+measured.Width > origin.X+r.Size().Width+0.75 ||
					p.Y < origin.Y-0.75 || p.Y+measured.Height > origin.Y+r.Size().Height+0.75 {
					t.Errorf("clipped %q at %v (%v) in row at %v (%v)", v.Text, p, measured, origin, r.Size())
				}
			}
		})
	}
	visit(r.box)
	return strings.Join(strings.Fields(strings.Join(runs, " ")), " ")
}

func scopeContains(t *testing.T, rendered, want string) {
	t.Helper()
	if !strings.Contains(rendered, want) {
		t.Fatalf("missing visible words %q in %q", want, rendered)
	}
}

func TestWaitingActionExplainsItsScopeOnAShortPhone(t *testing.T) {
	armAISpeedClock(t)
	for _, tc := range []struct {
		name, model, saved, lower string
		levels                    []string
	}{
		{"named-effort", geminiFastModel, "high", "medium", []string{"medium", "high"}},
		{"numeric-budget", geminiFastModel, "4096", "1024", []string{"1024", "4096"}},
		{"only-model", "gemini-3-pro", "low", "", []string{"low", "high"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := insetScreen{"short portrait", 320, 568, 20, 0, 0, 0}
			st, w := insetWindow(t, sc, 0)
			store := st.keys()
			store.setActiveProvider(providerGemini)
			store.setAPIKey(providerGemini, "synthetic")
			grantAIPermission(store, providerGemini)
			store.setOverrideModel(providerGemini, tc.model)
			rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{{id: tc.model, efforts: tc.levels}})
			setAIReasoningEffort(store.prefs, providerGemini, tc.model, tc.saved)
			sends := routeStudy(t, "")
			showAIPanel(st, aiActionExplain, "Synthetic passage.", "", selSpan{})
			s := nextSend(t, sends)
			if s.effort != tc.saved || s.model != tc.model {
				t.Fatal("wrong starting model or effort")
			}
			p := topPopup(t, w)
			head, rows := scopeRows(p)
			if head != nil || len(rows) != 1 {
				t.Fatalf("expected standalone choice, got chooser=%v, rows=%d", head != nil, len(rows))
			}
			words := scopeRowText(t, rows[0])
			if tc.lower != "" {
				scopeContains(t, words, "Use less reasoning this time")
				scopeContains(t, words, effortNoteLabel(tc.lower))
			} else {
				scopeContains(t, words, "Switch to a faster model")
				scopeContains(t, words, geminiFastModel+" · saves in Settings")
			}
			rows[0].Tapped(nil)
			next := nextSend(t, sends)
			if tc.lower != "" && (next.model != tc.model || next.effort != tc.lower || store.overrideModel(providerGemini) != tc.model) {
				t.Fatal("standalone one-time choice changed model or sent wrong effort")
			}
			if tc.lower == "" && (next.model != geminiFastModel || store.overrideModel(providerGemini) != geminiFastModel) {
				t.Fatal("standalone switch did not send and save faster model")
			}
			if aiReasoningEffort(store.prefs, providerGemini, tc.model) != tc.saved {
				t.Fatal("saved source effort changed")
			}
			findTreeButton(topPopup(t, w), "Cancel").OnTapped()
		})
	}
}

func TestWaitingActionTextFitsAndMatchesTheRestart(t *testing.T) {
	armAISpeedClock(t)
	for _, sc := range []insetScreen{
		{"portrait", 393, 852, 53, 34, 0, 0},
		{"compact-landscape", 667, 375, 0, 21, 44, 44},
	} {
		for _, action := range []string{"effort", "budget", "model"} {
			t.Run(sc.name+"/"+action, func(t *testing.T) {
				original := appearanceVariant
				variant := theme.VariantLight
				if sc.name == "portrait" {
					variant = theme.VariantDark
				}
				appearanceVariant = func(*AppState) fyne.ThemeVariant { return variant }
				t.Cleanup(func() { appearanceVariant = original })
				st, w := insetWindow(t, sc, 0)
				switchThemeUnder(st.app, variant)
				const model = "gemini-3-pro"
				store := st.keys()
				store.setActiveProvider(providerGemini)
				store.setAPIKey(providerGemini, "synthetic")
				grantAIPermission(store, providerGemini)
				store.setOverrideModel(providerGemini, model)
				high, lower := "high", "medium"
				choices := []string{"low", "medium", "high"}
				title := "Use less reasoning this time"
				detail := "Medium effort"
				if action == "budget" {
					high, lower = "4096", "1024"
					choices = []string{lower, high}
					detail = "1,024 tokens"
				}
				rememberModelEfforts(store.prefs, providerGemini, []discoveredModel{{id: model, efforts: choices}, {id: geminiFastModel, efforts: []string{"low", "high"}}})
				setAIReasoningEffort(store.prefs, providerGemini, model, high)
				setAIReasoningEffort(store.prefs, providerGemini, geminiFastModel, "low")
				sends := routeStudy(t, "")
				ask := func(q string) *widget.PopUp {
					showAIPanel(st, aiActionAsk, "A synthetic passage.", q, selSpan{})
					return topPopup(t, w)
				}
				checkSend := func(m, e string) {
					t.Helper()
					s := nextSend(t, sends)
					if s.model != m || s.effort != e {
						t.Fatalf("sent model %q effort %q, want %q %q", s.model, s.effort, m, e)
					}
				}
				p := ask("First synthetic question?")
				checkSend(model, high)
				wantInsideTheSafeArea(t, sc, p)
				h, rows := scopeRows(p)
				if h == nil || len(rows) != 0 || h.expanded {
					t.Fatalf("wrong initial disclosure: %v, %d", h != nil, len(rows))
				}
				h.Tapped(nil)
				_, rows = scopeRows(p)
				if !h.expanded || len(rows) != 2 {
					t.Fatalf("not expanded cleanly: expanded=%v, rows=%d", h.expanded, len(rows))
				}
				position := fyne.CurrentApp().Driver().AbsolutePositionForObject
				if position(rows[1]).Y+0.5 < position(rows[0]).Y+rows[0].Size().Height {
					t.Fatal("action rows overlap")
				}
				effortRow, modelRow := scopeRowText(t, rows[0]), scopeRowText(t, rows[1])
				scopeContains(t, effortRow, title)
				scopeContains(t, effortRow, detail)
				scopeContains(t, modelRow, "Switch to a faster model")
				scopeContains(t, modelRow, geminiFastModel+" · saves in Settings")
				cancel := findTreeButton(p, "Cancel")
				if cancel == nil {
					t.Fatal("no Cancel")
				}
				scroll := scrollHolding(p, h)
				if scroll == nil {
					t.Fatal("no waiting column")
				}
				for _, act := range []fyne.CanvasObject{h, rows[0], rows[1]} {
					local, ok := descendantPos(scroll.Content, act)
					size := act.Size()
					if !ok || size.Width <= 0 || size.Height <= 0 || local.Y < -0.5 ||
						local.Y+size.Height > scroll.Content.Size().Height+0.5 || size.Height > scroll.Size().Height+0.5 {
						t.Fatalf("unreachable action %T: position %v, size %v, content %v viewport %v", act, local, size, scroll.Content.Size(), scroll.Size())
					}
				}
				if scrollHolding(p, cancel) != nil {
					t.Fatal("Study Cancel must stand in the fixed footer, not inside the waiting scroll")
				}
				if dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR"); dir != "" && action != "model" {
					writeRenderPNG(t, filepath.Join(dir, "ai-wait-"+sc.name+"-"+action+".png"), w.Canvas().Capture())
				}
				selected, sendModel, sendEffort, nextModel, nextEffort := rows[0], model, lower, model, high
				if action == "model" {
					selected, sendModel, sendEffort, nextModel, nextEffort = rows[1], geminiFastModel, "low", geminiFastModel, "low"
				}
				selected.Tapped(nil)
				checkSend(sendModel, sendEffort)
				if store.overrideModel(providerGemini) != nextModel || aiReasoningEffort(store.prefs, providerGemini, model) != high {
					t.Fatal("saved preferences disagree with stated scope")
				}
				cancel = findTreeButton(topPopup(t, w), "Cancel")
				if cancel == nil {
					t.Fatal("no Cancel after restart")
				}
				cancel.OnTapped()
				fresh := ask("Second synthetic question?")
				checkSend(nextModel, nextEffort)
				findTreeButton(fresh, "Cancel").OnTapped()
			})
		}
	}
}
