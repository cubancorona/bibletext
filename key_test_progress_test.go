package bibletext

// A Test key's wait and verdict outlive the sheet that started it
// (key_test_progress.go): a light/dark change drains Settings and reopens a
// fresh one, and the test's reports paint through whichever sheet is showing.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// keyTestButtons is every Test key button on the sheet in tree order: the
// assistant's first, the API.Bible key's second (the ASSISTANT section is
// built above TRANSLATIONS).
func keyTestButtons(p fyne.CanvasObject) []*widget.Button {
	var out []*widget.Button
	walkTree(p, func(n fyne.CanvasObject) {
		if b, ok := n.(*widget.Button); ok && b.Text == "Test key" {
			out = append(out, b)
		}
	})
	return out
}

// paintsOf signals each paint of a key section's line after it is painted,
// so the test can order a read of the sheet after a report made on the
// request's goroutine.
func paintsOf(state *AppState, section string) chan struct{} {
	slot := state.keyTest(section)
	painted := make(chan struct{}, 8)
	orig := slot.paint
	slot.paint = func() {
		orig()
		select {
		case painted <- struct{}{}:
		default:
		}
	}
	return painted
}

func awaitPaint(t *testing.T, painted chan struct{}, what string) {
	t.Helper()
	select {
	case <-painted:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s never painted the line", what)
	}
}

// THE ASSISTANT KEY'S TEST REPORTS TO THE REOPENED SHEET. Tapped before a
// light/dark change, the test's wait is on the sheet that comes back, the
// model it reports joins the wait there, and its verdict lands there. The ✕
// forgets it, and Settings opened afresh shows no line. Mutations guarded:
// the line painted from the drained sheet's closures alone (the reopened
// sheet shows neither the wait nor the verdict); the reopen forgetting the
// test with the ordinary open (no wait on the reopened sheet); the ✕ not
// forgetting it (the next sheet opens on a stale verdict).
func TestTheKeyTestOutlivesALightDarkReopen(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		t.Run(map[bool]string{false: "desktop", true: "phone"}[mobile], func(t *testing.T) {
			calls, listed := holdKeyTests(t)
			h := newAppearanceHarness(t, mobile)
			h.state.aiKeys = newKeyStoreWith(newFakePrefs())
			h.state.aiKeys.setAPIKey(defaultProviderID, "test-key")
			showAISettings(h.state)
			waitParked(t, listed, "the model list")
			popup := settingsPopup(t, h.state)
			keyTestButtons(popup)[0].OnTapped()
			var first keyTestCall
			select {
			case first = <-calls:
			case <-time.After(2 * time.Second):
				t.Fatal("the key test never started")
			}
			if got := keyTestResult(popup); got != "Testing…" {
				t.Fatalf("control: the sheet reads %q under the key, want the wait", got)
			}

			h.flip()
			again := h.top()
			if again == nil || again == popup || !again.Visible() || h.overlays() != 1 || !sheetHas(again, "Settings") {
				t.Fatalf("Settings must come back, rebuilt, alone (overlays %d)", h.overlays())
			}
			waitParked(t, listed, "the reopened sheet's model list")
			if got := keyTestResult(again); got != "Testing…" {
				t.Errorf("the reopened sheet reads %q under the key, want the running test's wait", got)
			}
			painted := paintsOf(h.state, keyTestAI)

			reportAIModel(first.ctx, providerGemini, geminiModel)
			awaitPaint(t, painted, "the model report")
			if got := keyTestResult(again); !strings.HasPrefix(got, "Testing…") || !strings.Contains(got, geminiModel) {
				t.Errorf("after the report the reopened sheet reads %q, want the wait naming %s", got, geminiModel)
			}

			first.release <- nil
			awaitPaint(t, painted, "the verdict")
			if got := keyTestResult(again); !strings.HasPrefix(got, "✓ Key works.") {
				t.Errorf("after the test answered the reopened sheet reads %q, want the verdict", got)
			}

			// The ✕ forgets the test; an ordinary open shows no line.
			sheetCloseButton(t, again).OnTapped()
			if h.top() != nil {
				t.Fatal("control: the ✕ must close the sheet")
			}
			showAISettings(h.state)
			waitParked(t, listed, "the third sheet's model list")
			third := settingsPopup(t, h.state)
			defer third.Hide()
			if got := keyTestResult(third); got != "" {
				t.Errorf("Settings opened afresh reads %q under the key, want no line", got)
			}
		})
	}
}

// THE API.BIBLE KEY'S TEST REPORTS TO THE REOPENED SHEET TOO. Its request is
// held at a local server until the test releases it.
func TestTheAPIBibleKeyTestOutlivesALightDarkReopen(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"name":"New King James Version"}}`))
	}))
	defer srv.Close()
	prev := apiBibleBaseURL
	apiBibleBaseURL = srv.URL
	t.Cleanup(func() { apiBibleBaseURL = prev })

	h := newAppearanceHarness(t, false)
	h.state.aiKeys = newKeyStoreWith(newFakePrefs())
	h.state.aiKeys.setBibleAPIKey("test-key")
	showAISettings(h.state)
	popup := settingsPopup(t, h.state)
	buttons := keyTestButtons(popup)
	if len(buttons) != 2 {
		t.Fatalf("control: %d Test key buttons, want the assistant's and the API.Bible key's", len(buttons))
	}
	buttons[1].OnTapped()
	waitParked(t, started, "the API.Bible probe")
	if got := keyTestResult(popup); got != "Testing…" {
		t.Fatalf("control: the sheet reads %q under the API.Bible key, want the wait", got)
	}

	h.flip()
	again := h.top()
	if again == nil || again == popup || !sheetHas(again, "Settings") {
		t.Fatal("Settings must come back")
	}
	if got := keyTestResult(again); got != "Testing…" {
		t.Errorf("the reopened sheet reads %q under the API.Bible key, want the running test's wait", got)
	}
	painted := paintsOf(h.state, keyTestBible)
	close(release)
	awaitPaint(t, painted, "the verdict")
	if got := keyTestResult(again); !strings.HasPrefix(got, "✓ Key works.") || !strings.Contains(got, "New King James Version") {
		t.Errorf("after the probe answered the reopened sheet reads %q, want the verdict naming the translation", got)
	}
	sheetCloseButton(t, again).OnTapped()
}
