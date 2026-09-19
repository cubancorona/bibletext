package bibletext

// Linux ships on two architectures now, and the two release jobs must stay
// honest about each other.
//
// The defect these guard against is not a build failure — it is an arm64
// artefact that builds, packs, installs and launches perfectly while quietly
// checking less than the amd64 one did, or binding an ALSA path that does not
// exist on the machine it runs on. Neither shows up in CI going green.

import (
	"regexp"
	"strings"
	"testing"
)

// Both Linux jobs must run the SAME package assertions. They were inline YAML
// in one job; adding a second architecture by copy-paste is how one of them
// silently stops checking the desktop entry's %u, which breaks every
// "Open in BibleText" link on that architecture only.
func TestBothLinuxReleaseJobsRunTheSamePackageChecks(t *testing.T) {
	wf := readRepoFile(t, ".github/workflows/release.yml")

	const checker = "scripts/check-linux-package.sh"
	if n := strings.Count(wf, checker); n != 2 {
		t.Errorf("release.yml calls %s %d times, want 2 (the amd64 job and the arm64 job); "+
			"an architecture that does not call it is shipping unchecked", checker, n)
	}
	for _, tarball := range []string{"BibleText-Linux-amd64.tar.xz", "BibleText-Linux-arm64.tar.xz"} {
		if !strings.Contains(wf, checker+" ../../"+tarball) {
			t.Errorf("%s is never passed to %s", tarball, checker)
		}
	}
	// The old inline block must be gone, or it will drift back.
	if strings.Contains(wf, "Expected one packaged Linux executable") {
		t.Error("release.yml still carries the inline package assertions; they belong in " + checker)
	}
}

// The arm64 snap MUST re-render before packing, because the committed
// snapcraft.yaml is the amd64 render and snap layouts take no variables.
// Without this the arm64 snap binds /usr/lib/x86_64-linux-gnu/alsa-lib, which
// does not exist on arm64 — the app still launches, and narration is silent.
func TestTheArm64SnapJobReRendersForItsArchitecture(t *testing.T) {
	whole := readRepoFile(t, ".github/workflows/release.yml")

	// Scoped to the job, not the file: matching anywhere would be satisfied by a
	// mention in a comment or another job while snap-arm64 packed the amd64
	// render — the exact defect this is here to prevent.
	start := strings.Index(whole, "\n  snap-arm64:\n")
	if start < 0 {
		t.Fatal("release.yml has no snap-arm64 job")
	}
	rest := whole[start+1:]
	end := len(rest)
	if next := regexp.MustCompile(`(?m)^  [a-z][a-z0-9-]*:$`).FindStringIndex(rest[1:]); next != nil {
		end = next[0] + 1
	}
	wf := rest[:end]

	if !strings.Contains(wf, "linuxmeta render -arch arm64") {
		t.Fatal("the arm64 snap job does not re-render snapcraft.yaml for arm64; " +
			"it would pack the committed amd64 render, binding an ALSA path that does not exist")
	}
	if !strings.Contains(wf, "aarch64-linux-gnu/alsa-lib") {
		t.Error("the arm64 snap job does not assert the aarch64 ALSA layout reached the manifest")
	}
	// Asserting the render happened is not enough: the plugins must resolve
	// inside the confinement, which is the thing a reader would notice.
	if !strings.Contains(wf, "snap run --shell bibletext") {
		t.Error("the arm64 snap job never checks that the ALSA layout resolves inside the snap's confinement")
	}
}
