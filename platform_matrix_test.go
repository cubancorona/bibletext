package bibletext

// docs/PLATFORM_MATRIX.md is only worth having if it is true, and the failure
// mode for a document like this is not being wrong the day it is written — it
// is going quietly stale. docs/LINUX_STORES.md claimed the Snap credential was
// outstanding for hours after it wasn't, and its architectures row claimed
// x86_64-only for as long as nobody re-read it.
//
// So: everything the matrix names must exist. This cannot check that the STATUS
// and PROOF columns are honest — a person has to do that — but it catches the
// cheap rot, where a file is renamed and the document silently starts
// describing a repository that has moved on.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Where a bare name might live, so the matrix can say `linux-stores.yml`
// rather than spelling out the workflow directory every time.
var matrixSearchDirs = []string{"", ".github/workflows", "patches", "scripts", "docs"}

func TestPlatformMatrixNamesThingsThatExist(t *testing.T) {
	root := repoRoot(t)
	doc := readRepoFile(t, "docs/PLATFORM_MATRIX.md")

	// Strip blockquotes: the document deliberately QUOTES the old decision it
	// exists to correct, and a quotation is not a claim.
	var body []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), ">") {
			continue
		}
		body = append(body, line)
	}
	text := strings.Join(body, "\n")

	pathish := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.(?:go|sh|py|ps1|yml|yaml|json|toml|patch|xml))`")
	seen := map[string]bool{}
	for _, m := range pathish.FindAllStringSubmatch(text, -1) {
		seen[m[1]] = true
	}
	if len(seen) < 12 {
		t.Fatalf("only %d file names found in the matrix; the extraction is broken, so this test proves nothing", len(seen))
	}

	var missing []string
	for p := range seen {
		found := false
		for _, dir := range matrixSearchDirs {
			if _, err := os.Stat(filepath.Join(root, dir, p)); err == nil {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, p)
		}
	}
	sort.Strings(missing)
	for _, p := range missing {
		t.Errorf("docs/PLATFORM_MATRIX.md names %s, which exists nowhere in the tree; "+
			"the matrix is describing a repository that has moved on", p)
	}
	t.Logf("checked %d file names named by the matrix", len(seen))
}

// The matrix claims Linux ships two architectures. That is only true if the
// generator can actually render both — the thing that makes an arm64 snap carry
// its own ALSA triplet rather than one that does not exist on the machine.
func TestPlatformMatrixArchitectureClaimsMatchTheGenerator(t *testing.T) {
	gen := readRepoFile(t, "cmd/linuxmeta/main.go")
	for _, arch := range []string{"amd64", "arm64"} {
		if !strings.Contains(gen, `"`+arch+`": {Platform:`) {
			t.Errorf("the matrix lists Linux %s but cmd/linuxmeta has no snapArch entry for it", arch)
		}
	}
}

// Windows renders only because ANGLE is beside the executable, and Windows on
// ARM will happily load an x64 ANGLE next to an arm64 binary — the app would
// start, render through an emulated translation layer, and nothing would say
// so. The pinned hash per architecture is what makes that impossible rather
// than merely unlikely, so both must be present and they must differ.
func TestAngleIsPinnedSeparatelyForEachWindowsArchitecture(t *testing.T) {
	src := readRepoFile(t, "scripts/fetch-angle.ps1")

	hashes := map[string]string{}
	for _, arch := range []string{"x64", "arm64"} {
		m := regexp.MustCompile(`'` + arch + `'\s*=\s*'([0-9a-f]{64})'`).FindStringSubmatch(src)
		if m == nil {
			t.Fatalf("scripts/fetch-angle.ps1 pins no sha256 for %s; that architecture cannot be fetched safely", arch)
		}
		hashes[arch] = m[1]
	}
	if hashes["x64"] == hashes["arm64"] {
		t.Error("the x64 and arm64 ANGLE builds are pinned to the SAME hash; one of them is wrong, " +
			"and the wrong one would load and run under emulation rather than fail")
	}
	// The URL must vary by architecture too, or both pins fetch the same zip and
	// one of them simply fails the hash check at build time.
	if !strings.Contains(src, "angle-$Arch-$tag.zip") {
		t.Error("the ANGLE download URL does not vary by architecture")
	}
}

// iOS and Android both hide most of their platform code behind build tags, so
// `go build ./...`, `go vet` and the entire test suite compile none of it. iOS
// has had a cross-compile gate since a cgo typo survived several rounds of "the
// tests pass"; Android had none anywhere — not in CI, not as a local script —
// which is a gap this pair exists to keep closed.
func TestBothMobilePlatformsHaveACompileGate(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	for _, gate := range []string{"scripts/check-ios-pane.sh", "scripts/check-android-pane.sh"} {
		if !strings.Contains(ci, gate) {
			t.Errorf("ci.yml never runs %s; that platform's tagged sources are compiled by nothing", gate)
		}
		if _, err := os.Stat(filepath.Join(repoRoot(t), gate)); err != nil {
			t.Errorf("%s is named by ci.yml but does not exist", gate)
		}
	}
	// A gate that skips when its toolchain is missing reads as a pass and is
	// worse than no gate; on a runner it must fail instead.
	android := readRepoFile(t, "scripts/check-android-pane.sh")
	if !strings.Contains(android, "${CI:-}") {
		t.Error("check-android-pane.sh does not distinguish CI from a local run; " +
			"a silent skip on a runner would report success while compiling nothing")
	}
}

// The only macOS build any script launches used to link STOCK, UNPATCHED Fyne,
// while everything we ship links the patched toolkit. That made the local
// rehearsal a rehearsal of a different program — and the patch it was missing
// is the atomic preferences writer, whose absence empties the note store if the
// app dies mid-write.
//
// Measured on 19 September 2026: a stock build carries 0 occurrences of the
// marker and 4 of the control string; a patched build carries 1 and 4. So the
// probe can see, and the zero meant absence rather than blindness.
func TestTheLocalMacBuildUsesTheToolkitWeActuallyShip(t *testing.T) {
	sh := readRepoFile(t, "scripts/run-mac-sandbox-test.sh")

	if !strings.Contains(sh, "setup-fyne-patch.sh") {
		t.Error("run-mac-sandbox-test.sh never applies the Fyne patches, so it launches the stock toolkit — " +
			"a different program from the one we ship")
	}
	if !strings.Contains(sh, "go mod edit -replace fyne.io/fyne/v2=./third_party/fyne") {
		t.Error("run-mac-sandbox-test.sh does not point the build at the patched tree")
	}
	if !strings.Contains(sh, "Preferences save not published") {
		t.Error("run-mac-sandbox-test.sh never checks the patched writer reached the binary; " +
			"applying a patch and verifying it landed are different things")
	}
	// A marker probe with no control string cannot tell absence from blindness.
	if !strings.Contains(sh, "World English Bible") {
		t.Error("the patch-marker probe has no control string, so a zero could mean the probe itself is broken")
	}
	// go.mod ships stock and must stay that way after the script runs.
	if !strings.Contains(sh, "go.mod.original") {
		t.Error("run-mac-sandbox-test.sh rewrites go.mod without restoring it")
	}
}

// THE NAME A READER SEES. `fyne package` names the executable after the source
// directory -- cmd/desktop -- so a Linux tarball built without --executable
// installs /usr/local/bin/desktop. That name is wrong in three user-visible
// places at once: it is a generic command in the reader's PATH that any other
// Fyne app packaged from a desktop/ directory overwrites, it is what a reader
// must type to start the app, and it is the client name the sound server shows
// while narration plays -- a Linux volume control read "desktop", never
// "BibleText". Proved on an arm64 desktop by running one identical binary under
// both names: as desktop the stream was "PipeWire ALSA [desktop]", as bibletext
// it was "PipeWire ALSA [bibletext]".
//
// The snap and the AppImage each rename the same executable on their way in, so
// only the tarball ever shipped it raw. macOS still bundles CFBundleExecutable
// as desktop and is deliberately out of scope here: that binary is signed and
// shipped through the App Store, so changing it is a release decision rather
// than a packaging fix.
func TestTheLinuxPackagesShipTheAppsOwnExecutableName(t *testing.T) {
	wf := readRepoFile(t, ".github/workflows/release.yml")

	for _, bad := range []string{
		"-os linux --app-id uk.co.bibletext --executable desktop",
		"cmd/desktop/desktop",
	} {
		if strings.Contains(wf, bad) {
			t.Errorf("release.yml still has %q; a Linux install would put a binary called "+
				"'desktop' in the reader's PATH and name it 'desktop' in the volume control", bad)
		}
	}

	// Both architectures must package under the right name, not just one.
	if got := strings.Count(wf, "-os linux --app-id uk.co.bibletext --executable bibletext"); got != 2 {
		t.Errorf("found %d Linux packaging lines naming the executable bibletext, want 2 "+
			"(amd64 and arm64) -- an architecture that stops passing --executable silently reverts", got)
	}

	// macOS is the deliberate exception; if that line disappears this test's
	// scope note is stale and the exception needs re-deciding, not ignoring.
	if !strings.Contains(wf, "-os darwin --app-id uk.co.bibletext --executable desktop") {
		t.Error("the darwin packaging line no longer names its executable 'desktop'; " +
			"this test documents that as a deliberate exception, so update the note above")
	}

	// The check that runs in CI must pin the name too, or the workflow could
	// drift back with nothing to catch it.
	chk := readRepoFile(t, "scripts/check-linux-package.sh")
	if !strings.Contains(chk, `[ "$exe" = bibletext ]`) {
		t.Error("check-linux-package.sh no longer asserts the packaged executable is named bibletext")
	}
	if !strings.Contains(chk, `grep -q '^Exec=bibletext %u$'`) {
		t.Error("check-linux-package.sh no longer requires the shipped desktop entry to launch 'bibletext'")
	}
}
