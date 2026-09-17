package bibletext

// The app follows the system appearance; it has no in-app light/dark control
// (theme.go). That promise is only as good as the toolkit's willingness to
// report a change, and the two mobile platforms did not report it the same way.
//
// iOS routes a trait change through the same size.Event as a resize, so the
// variant moves the moment the phone switches at sunset. Android read the night
// flag correctly and then dropped it: the Java side calls setDarkMode on every
// configuration change, but that only assigns a package variable, and the flag
// is stamped into a size.Event in exactly one place — the redraw branch. A
// theme-only change produces no redraw, so the value sat unread and the reader
// kept the old palette until a rotation or a return from the background
// happened to produce one.
//
// patches/fyne-2.7.4-android-night-mode.patch closes that gap. These tests hold
// it in place. They are deliberately layered, because the behaviour itself
// cannot be exercised on this host or in CI — android.go is behind GOOS=android
// and cgo, and no emulator runs in the hygiene job:
//
//   1. the tracked patch still says what it must (runs everywhere, always);
//   2. the build really applies it (runs everywhere, always);
//   3. the materialised toolkit really carries it, on both platforms (runs
//      wherever third_party/fyne exists — after scripts/setup-fyne-patch.sh,
//      which is every release build).
//
// The end-to-end behaviour is proven by scripts/test-android-dark-mode.sh,
// which drives a real emulator and compares an unpatched build against a
// patched one. That cannot run in CI, so it is a documented manual gate rather
// than a silent gap.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nightModePatch = "patches/fyne-2.7.4-android-night-mode.patch"

// The substance of the patch, as strings that must survive any regeneration of
// it against a newer Fyne. Each one is a separate claim about the fix, so a
// patch that still applies but has lost a piece fails here rather than in a
// reader's hands.
var nightModeMustContain = map[string]string{
	"targets the Android driver":       "internal/driver/mobile/app/android.go",
	"acts on the config-change branch": "case cfg := <-windowConfigChange:",
	"only forwards a real change":      "currentSize.DarkMode != darkMode",
	"adopts the new flag":              "currentSize.DarkMode = darkMode",
	"sends it as a size event":         "theApp.events.In() <- currentSize",
	"waits for a surface":              "surfaceInitialized",
}

func TestAndroidNightModePatchStillSaysWhatItMust(t *testing.T) {
	body := readRepoFile(t, nightModePatch)
	for claim, want := range nightModeMustContain {
		if !strings.Contains(body, want) {
			t.Errorf("%s no longer %s: %q is absent.\n"+
				"Without it a system theme switch does not reach the app while it is running.",
				nightModePatch, claim, want)
		}
	}
	// The guard is the difference between following the system and repainting
	// at the wrong moment: a rotation fires the config-change branch and then
	// the redraw branch, so an unconditional send adds a size.Event carrying
	// the pre-rotation dimensions.
	if strings.Contains(body, "+\t\t\tcurrentSize.DarkMode = darkMode") &&
		!strings.Contains(body, "currentSize.DarkMode != darkMode") {
		t.Error("the patch adopts the flag unconditionally; a rotation would gain a stale size.Event")
	}
}

func TestTheBuildAppliesTheNightModePatch(t *testing.T) {
	script := readRepoFile(t, "scripts/setup-fyne-patch.sh")
	base := filepath.Base(nightModePatch)
	if !strings.Contains(script, base) {
		t.Fatalf("scripts/setup-fyne-patch.sh does not name %s, so a build would ship without it", base)
	}
	// Named is not applied: the script assigns each patch to a variable and
	// then runs `patch` with it, and a patch that is only assigned is a patch
	// that silently does nothing.
	var variable string
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, base) && strings.Contains(line, "=") {
			variable = strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
			break
		}
	}
	if variable == "" {
		t.Fatalf("no variable in setup-fyne-patch.sh holds %s", base)
	}
	if !strings.Contains(script, `patch -p1 -d "$DEST" < "$`+variable+`"`) {
		t.Errorf("%s is assigned to %s but never applied with patch -p1", base, variable)
	}
}

// The materialised toolkit is regenerated from scratch by
// scripts/setup-fyne-patch.sh and is not tracked, so this can only run where a
// build has already happened. It is the closest thing to a behavioural check
// that a host without an emulator can make, and it covers BOTH platforms: the
// Android half is ours, the iOS half is upstream's, and a Fyne bump could take
// either away.
func TestPatchedToolkitFollowsTheSystemOnBothMobilePlatforms(t *testing.T) {
	root := repoRoot(t)
	android := filepath.Join(root, "third_party", "fyne", "internal", "driver", "mobile", "app", "android.go")
	if _, err := os.Stat(android); err != nil {
		t.Skipf("third_party/fyne is absent (regenerated by scripts/setup-fyne-patch.sh, not tracked)")
	}

	body, err := os.ReadFile(android)
	if err != nil {
		t.Fatalf("reading the patched android driver: %v", err)
	}
	src := string(body)
	for claim, want := range map[string]string{
		"forwards only a real change": "currentSize.DarkMode != darkMode",
		"sends it as a size event":    "theApp.events.In() <- currentSize",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the patched android driver no longer %s (%q absent) — "+
				"a system theme switch will not reach a running app", claim, want)
		}
	}

	// iOS: upstream stamps the flag into the same size.Event a resize sends,
	// which is why the phone switching at sunset has always worked there. If a
	// Fyne bump ever drops it, iOS quietly acquires the bug Android had.
	ios := filepath.Join(root, "third_party", "fyne", "internal", "driver", "mobile", "app", "darwin_ios.go")
	if body, err := os.ReadFile(ios); err != nil {
		t.Errorf("reading the patched iOS driver: %v", err)
	} else if !strings.Contains(string(body), "DarkMode:") {
		t.Error("the iOS driver no longer carries DarkMode in its size.Event — " +
			"iOS would stop following the system appearance while running")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	return root
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(body)
}
