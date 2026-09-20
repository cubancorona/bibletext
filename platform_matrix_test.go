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

// THE NAME A READER SEES, AND WHERE IT COMES FROM.
//
// `fyne package` names the executable after its SOURCE DIRECTORY. While that
// directory was cmd/desktop, every channel that did not override the name
// shipped a binary called `desktop`: the Linux tarball installed a generic
// /usr/local/bin/desktop, and a Linux volume control listed the app playing
// narration as "desktop" rather than BibleText. Proved on an arm64 desktop by
// running one identical binary under two filenames -- same sha256 -- and
// watching the sound server's client name move with the filename.
//
// Overriding the name per channel fixed the packaged artefacts but could never
// fix the two routes README.md advertises, because those run `fyne install` and
// `go install` on the reader's own machine against the module path, and
// `fyne install` has no --executable flag at all. Renaming the directory to
// cmd/bibletext is the root fix: it reaches those routes, and every packager
// then defaults to the right name instead of needing to be told.
//
// The per-platform spellings are deliberate and different, because each is the
// convention of the place it lands: `bibletext` is a command in the reader's
// PATH, `BibleText.exe` is what Windows shows in Task Manager and the Store
// manifest, and `BibleText` is what sits in Contents/MacOS the way
// Safari.app/Contents/MacOS/Safari does.
func TestTheExecutableIsNamedAfterTheAppOnEveryChannel(t *testing.T) {
	// The root fix. Everything below is downstream of this directory's name.
	if _, err := os.Stat(filepath.Join(repoRoot(t), "cmd/bibletext")); err != nil {
		t.Fatalf("cmd/bibletext is missing (%v); the packagers take the executable "+
			"name from this directory, so it is the name a reader ends up running", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot(t), "cmd/desktop")); err == nil {
		t.Error("cmd/desktop is back; `fyne install …/cmd/desktop@latest` and " +
			"`go install …/cmd/desktop@latest` would again put a binary called " +
			"'desktop' in the reader's PATH, and no packaging flag can reach those routes")
	}

	wf := readRepoFile(t, ".github/workflows/release.yml")

	// No packaging line on any platform may name the old generic executable.
	if strings.Contains(wf, "--executable desktop") {
		t.Error("release.yml packages an executable called 'desktop' again")
	}

	// Each channel, spelled the way that platform expects.
	for _, c := range []struct{ what, want string }{
		{"Linux (a command in PATH)", "-os linux --app-id uk.co.bibletext --executable bibletext"},
		{"Windows (Task Manager and the Store manifest)", "-os windows --tags gles --app-id uk.co.bibletext --executable BibleText.exe"},
		{"macOS (Contents/MacOS, Activity Monitor, crash reports)", "-os darwin --app-id uk.co.bibletext --executable BibleText"},
	} {
		if !strings.Contains(wf, c.want) {
			t.Errorf("release.yml no longer names the executable for %s: want a line containing %q", c.what, c.want)
		}
	}
	// Both Linux architectures, not just one -- an arch that stops passing the
	// flag reverts silently while the other still looks right.
	if got := strings.Count(wf, "--executable bibletext"); got != 2 {
		t.Errorf("found %d Linux packaging lines naming the executable bibletext, want 2 (amd64 and arm64)", got)
	}

	// macOS is set in three files, and the two outside the workflow are the
	// ones that build what actually reaches the Mac App Store.
	for _, f := range []string{"scripts/release-mac-store.sh", "scripts/run-mac-sandbox-test.sh"} {
		body := readRepoFile(t, f)
		if strings.Contains(body, "--executable desktop") || strings.Contains(body, "Contents/MacOS/desktop") {
			t.Errorf("%s still names the macOS executable 'desktop'", f)
		}
		if !strings.Contains(body, "--executable BibleText") {
			t.Errorf("%s no longer names the macOS executable BibleText", f)
		}
	}
	if !strings.Contains(readRepoFile(t, "scripts/build-windows-exe.sh"), "--executable BibleText.exe") {
		t.Error("scripts/build-windows-exe.sh no longer names the Windows executable BibleText.exe")
	}

	// The check that runs in CI must pin the Linux name too, or the workflow
	// could drift back with nothing to catch it.
	chk := readRepoFile(t, "scripts/check-linux-package.sh")
	if !strings.Contains(chk, `[ "$exe" = bibletext ]`) {
		t.Error("check-linux-package.sh no longer asserts the packaged executable is named bibletext")
	}
	if !strings.Contains(chk, `grep -q '^Exec=bibletext %u$'`) {
		t.Error("check-linux-package.sh no longer requires the shipped desktop entry to launch 'bibletext'")
	}

	// The README's own install routes are the ones a flag cannot reach, so they
	// have to point at the renamed directory or readers keep installing `desktop`.
	readme := readRepoFile(t, "README.md")
	// The module path, not the bare directory: the README explains in prose
	// that older tags carry cmd/desktop, which a reader resolving @latest
	// genuinely needs to know. What must not come back is an install COMMAND
	// pointing at it.
	if strings.Contains(readme, "github.com/cubancorona/bibletext/cmd/desktop") {
		t.Error("README.md gives an install command for github.com/cubancorona/bibletext/cmd/desktop")
	}
	for _, want := range []string{
		"fyne install github.com/cubancorona/bibletext/cmd/bibletext@latest",
		"go run github.com/cubancorona/bibletext/cmd/bibletext@latest",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md no longer documents %q", want)
		}
	}
}

// A PATH IS A PATH IN EITHER SLASH, AND ONE OF THEM IS INVISIBLE TO THE
// OBVIOUS SEARCH.
//
// When cmd/desktop became cmd/bibletext, every reference was rewritten except
// one: the Windows release job hands ANGLE's destination to a PowerShell
// script as `-Dest cmd\desktop`, with a BACKSLASH, so a repository-wide sweep
// for "cmd/desktop" passed straight over it while the zip step two lines below
// was correctly rewritten to cmd/bibletext.
//
// That miss would not have failed loudly. fetch-angle.ps1 creates its
// destination (`New-Item -ItemType Directory -Force $Dest`), so the ANGLE step
// would have gone green having filled a directory nothing reads, and the
// failure would have surfaced one step later as a zip that could not find four
// of its five files -- on both Windows architectures, for an executable built
// `-tags gles` that cannot create a window without those libraries.
//
// So this does not check one spelling of one path. It holds every cmd/ path in
// every workflow and script, in either slash, to a directory that actually
// exists -- which is the general form of the mistake.
func TestEveryCmdPathInTheBuildNamesADirectoryThatExists(t *testing.T) {
	root := repoRoot(t)

	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatalf("reading cmd/: %v", err)
	}
	real := map[string]bool{
		// third_party/fyne-tools' own program, built by the Linux job; not ours.
		"fyne": true,
	}
	for _, e := range entries {
		if e.IsDir() {
			real[e.Name()] = true
		}
	}

	ref := regexp.MustCompile(`cmd[/\\]([A-Za-z0-9_-]+)`)
	var checked int
	for _, dir := range []string{".github/workflows", "scripts"} {
		err := filepath.Walk(filepath.Join(root, dir), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			for _, line := range strings.Split(string(body), "\n") {
				// A whole-line comment is prose -- several of these scripts
				// explain the rename that made this test necessary, and must
				// be free to name the directory it moved from.
				if t := strings.TrimSpace(line); strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
					continue
				}
				for _, m := range ref.FindAllStringSubmatch(line, -1) {
					checked++
					if !real[m[1]] {
						t.Errorf("%s names %q, which is not a directory under cmd/\n    %s",
							rel, m[0], strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	// A regex that stopped matching would pass this test while checking
	// nothing, which is the failure mode these guards are prone to.
	if checked < 50 {
		t.Errorf("only %d cmd/ references were examined; the build references far more than that, "+
			"so the scan is no longer looking where it thinks it is", checked)
	}
}
