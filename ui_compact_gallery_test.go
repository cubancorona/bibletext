package bibletext

// WHAT THE SHARED COMPACT LAYOUT LOOKS LIKE ON A DESKTOP-SIZED WINDOW.
//
// Rendered to a software canvas rather than grabbed off the screen, for the
// reason debug_capture_macos.go spells out: screencapture(1) needs the Screen
// Recording entitlement, macOS caches that at process start, and granting it
// mid-session does nothing until the terminal is relaunched — which ends the
// session doing the looking. A software canvas needs no permission, renders the
// Fyne chrome the native capture cannot see, and is deterministic besides.
//
// Snapshots: BIBLETEXT_PANE_SNAPSHOT_DIR=<dir> go test -run TestCompactLayoutGallery
//
// It is a test, not a screenshot dump: each case asserts the bar chose the
// dress its width calls for before the picture is worth keeping.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestCompactLayoutGallery(t *testing.T) {
	dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set BIBLETEXT_PANE_SNAPSHOT_DIR to write the gallery")
	}

	app := test.NewApp()
	realTheme := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(realTheme)

	cases := []struct {
		name      string
		w, h      float32
		tab       int
		wantStyle tabBarStyle
	}{
		// A desktop window, and the same window dragged narrow. The second is
		// the case worth having a picture of: the bar is supposed to become a
		// phone bar on the way past the threshold, with nothing written to make
		// it do so.
		{"desktop-1280-read", 1280, 800, 0, tabBarEdgeCentred},
		{"rail-1280-read", 1280, 800, 0, tabBarEdgeCentred},
		{"rail-1280-books", 1280, 800, 1, tabBarEdgeCentred},
		{"rail-1024-read", 1024, 768, 0, tabBarEdgeCentred},
		{"desktop-1280-books", 1280, 800, 1, tabBarEdgeCentred},
		{"desktop-1280-search", 1280, 800, 2, tabBarEdgeCentred},
		{"desktop-1024-read", 1024, 768, 0, tabBarEdgeCentred},
		{"desktop-narrow-480-read", 480, 720, 0, tabBarEdgeSpread},
		{"tablet-834-books", 834, 1000, 1, tabBarEdgeCentred},
		{"phone-393-books", 393, 780, 1, tabBarEdgeSpread},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := compactGalleryState(t)
			st.CurrentTab = tc.tab
			w := test.NewWindow(nil)
			defer w.Close()
			w.Resize(fyne.NewSize(tc.w, tc.h))
			st.window = w
			st.app = app
			st.theme = realTheme

			// The rail cases set the preview switch for their render only.
			if strings.HasPrefix(tc.name, "rail") {
				t.Setenv("BIBLETEXT_DESKTOP_TABS", "rail")
			} else {
				t.Setenv("BIBLETEXT_DESKTOP_TABS", "")
			}

			if got := tabBarStyleFor(st); got != tc.wantStyle {
				t.Fatalf("tab bar style at %.0fpt = %v, want %v", tc.w, got, tc.wantStyle)
			}

			w.SetContent(buildCompactUI(st))
			w.Resize(fyne.NewSize(tc.w, tc.h))
			writePNG(t, filepath.Join(dir, "compact-"+tc.name+".png"), w.Canvas().Capture())
		})
	}
}

// compactGalleryState renders from the real cached translation when one is
// available locally, and refuses to draw otherwise.
//
// The first version of this hand-wrote six verses of Psalm 23 as plain strings,
// and the pictures it produced were wrong in a way that is easy to miss and
// impossible to un-see once noticed: Psalm 23 came out as PROSE. Poetry in this
// app is not a property of the psalm, it is the "\n" the decoder puts inside
// Verse.Text (verseIsPoetic, reading.go), and a hand-written fixture has none.
// A layout gallery that misrepresents how the app sets its most familiar page
// is worse than no gallery, so this one uses the real bytes or skips.
func compactGalleryState(t *testing.T) *AppState {
	t.Helper()

	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skip("no user cache dir")
	}
	path := filepath.Join(cache, "bibletext", "bibletext-web-v2.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no cached translation at %s — run the app once to populate it", path)
	}
	var wrapper struct {
		Data *BibleData `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil || wrapper.Data == nil {
		t.Skipf("cached translation unreadable: %v", err)
	}
	bd := wrapper.Data
	if len(bd.Books) == 0 || len(bd.GetChapter("Psalms", 23)) == 0 {
		t.Skip("cached translation has no Psalms 23")
	}
	bd.PrepareSearchIndex()

	// The check that would have caught the original mistake.
	poetic := false
	for _, v := range bd.GetChapter("Psalms", 23) {
		if verseIsPoetic(v.Text) {
			poetic = true
			break
		}
	}
	if !poetic {
		t.Fatal("Psalms 23 carries no poem-line breaks — the gallery would render " +
			"the psalm as prose and misrepresent the app")
	}

	return &AppState{
		Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23,
		CurrentVersion: "web", loadPhase: loadReady,
	}
}
