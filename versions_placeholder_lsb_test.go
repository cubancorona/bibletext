//go:build lsb

package bibletext

// THE PLACEHOLDER SWITCH PATH, WHICH NEEDS A REGISTERED UNLICENSED VERSION.
//
// switchVersion refuses an id that is not in registeredVersions, so this cannot
// be exercised with a value built in the test — unlike the behaviour tests in
// versions_test.go, which ask how an unconfigured licensed SOURCE behaves and
// construct one directly.
//
// Both evaluation translations are behind build tags now (versions_nrsv.go /
// versions_lsb.go), so a default build has no such version to switch to and
// this test lives with the tag that supplies one:
//
//\tgo test -tags lsb ./...
//
// That is not coverage quietly lost — it is coverage that only means anything
// in a build where the path is reachable at all.

import "testing"

func TestSwitchVersionUpdatesState(t *testing.T) {
	// LSB is a not-yet-licensed (testing) version, so it is normally not
	// selectable; unlock internal testing mode so the switch is allowed and
	// exercises the placeholder path.
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "1")

	base := baseSampleBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: "web",
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{"web": base},
		CurrentBook:    "John",
		CurrentChapter: 1,
	}

	// No window in tests, so rebuildWindow is a no-op; the state still updates.
	switchVersion(state, "lsb")
	if state.CurrentVersion != "lsb" || state.currentMode != modeTesting {
		t.Fatalf("after switch: version=%q mode=%v", state.CurrentVersion, state.currentMode)
	}
	if state.Bible == base {
		t.Error("Bible should have swapped to the LSB placeholder")
	}
	if state.currentVersion().Abbrev != "LSB" {
		t.Errorf("currentVersion abbrev = %q", state.currentVersion().Abbrev)
	}
	// Switching back to the cached base is instant and restores it.
	switchVersion(state, "web")
	if state.CurrentVersion != "web" || state.Bible != base {
		t.Error("switching back to web should restore the base data")
	}
}
