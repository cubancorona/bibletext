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
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	// Android has TWO halves on the far side of the build tags: the Go sources
	// (check-android-pane.sh) and the Java on the other side of the JNI
	// boundary (check-android-java.sh). The Java gate was added after a
	// silent-failure defect in BtBridge.shareImage survived every green run,
	// and for a while this test did not name it -- so removing its ci.yml step
	// would have passed here, which is the one thing this test exists to stop.
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	gates := []string{"scripts/check-ios-pane.sh", "scripts/check-android-pane.sh", "scripts/check-android-java.sh"}
	for _, gate := range gates {
		if !strings.Contains(ci, gate) {
			t.Errorf("ci.yml never runs %s; that platform's tagged sources are compiled by nothing", gate)
		}
		if _, err := os.Stat(filepath.Join(repoRoot(t), gate)); err != nil {
			t.Errorf("%s is named by ci.yml but does not exist", gate)
		}
	}
	// A gate that skips when its toolchain is missing reads as a pass and is
	// worse than no gate; on a runner it must fail instead. Both Android
	// gates need a toolchain the host may lack, so both must make the
	// distinction.
	for _, gate := range []string{"scripts/check-android-pane.sh", "scripts/check-android-java.sh"} {
		if !strings.Contains(readRepoFile(t, gate), "${CI:-}") {
			t.Errorf("%s does not distinguish CI from a local run; "+
				"a silent skip on a runner would report success while compiling nothing", gate)
		}
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

// WHAT SHARE DOES IS DECIDED BY ONE FILE PER PLATFORM.
//
// Every share verb ends in nativeShareText or nativeShareImage, and exactly
// one file defines each for a given release build: the Apple and Android
// panes present their system share sheets, and share_other.go gives Linux and
// Windows the clipboard fallback. docs/PLATFORM_MATRIX.md's Sharing table
// records what each platform's verbs do, and its Defined in column names that
// file. This holds the column to the code in both directions: a platform moved
// to a new share implementation (the planned Windows sheet is the first) fails
// here until its row names the new file, and a row edited to name a file the
// build does not use fails too.
//
// The definitions are resolved the way the go command resolves them, with
// go/build's file matching under each platform's GOOS, every architecture it
// ships and the tags its release line passes — not by reading the build
// constraints by eye, which is how a file comes to be believed to cover a
// platform it does not: a file constrained to linux is compiled into the
// Android build too, since android satisfies the linux constraint.

// sharingRows are the Sharing table's rows and how each platform's release is
// built. buildFile and buildLine name the release command the GOOS and tags
// were read from, so a release line that changes shape fails here instead of
// leaving the resolution below answering for a build nobody runs.
var sharingRows = []struct {
	row, goos            string
	arches, tags         []string
	buildFile, buildLine string
}{
	// A plain cross-compile with no -tags; GOOS=ios satisfies both the ios
	// and the darwin constraints, as it does for the go command. iPadOS is the
	// same universal binary.
	{"iOS", "ios", []string{"arm64"}, nil, "scripts/release-ios.sh", "GOOS=ios GOARCH=arm64"},
	{"iPadOS", "ios", []string{"arm64"}, nil, "scripts/release-ios.sh", "GOOS=ios GOARCH=arm64"},
	// Universal: both slices are compiled.
	{"macOS", "darwin", []string{"arm64", "amd64"}, nil, "scripts/release-mac-store.sh", "package -os darwin"},
	// The fyne tool adds the release tag to `fyne release`, and
	// build-android.sh refuses extra tags on a release build. The four ABIs
	// the AAB carries; the universal APK is built from the AAB.
	{"Android", "android", []string{"arm64", "arm", "386", "amd64"}, []string{"release"}, "scripts/build-android.sh", "fyne release -os android"},
	{"Windows", "windows", []string{"amd64", "arm64"}, []string{"gles"}, "scripts/build-windows-exe.sh", "go build -tags gles"},
	{"Linux", "linux", []string{"amd64", "arm64"}, nil, ".github/workflows/release.yml", "package -os linux"},
}

var shareEntryPoints = []string{"nativeShareText", "nativeShareImage"}

func TestTheSharingTableNamesTheFileThatSharesOnEachPlatform(t *testing.T) {
	root := repoRoot(t)
	table := sharingTable(t)

	// The table has exactly the rows this test resolves: a platform dropped
	// from the table, or one added without a build to resolve it against,
	// fails rather than going unchecked.
	var want, got []string
	for _, r := range sharingRows {
		want = append(want, r.row)
	}
	for row := range table {
		got = append(got, row)
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Fatalf("the Sharing table's rows are %v, want %v", got, want)
	}

	defs := shareDefinitions(t, root)
	named := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.go)`")
	proof := regexp.MustCompile("^`(hardware|field|runner|builds|none)`.*\\b20\\d\\d\\b")

	for _, r := range sharingRows {
		if !strings.Contains(readRepoFile(t, r.buildFile), r.buildLine) {
			t.Errorf("%s: %s no longer contains %q, the release line this row's GOOS and tags were read from",
				r.row, r.buildFile, r.buildLine)
		}

		// One file per entry point, and the same one on every architecture.
		files := map[string]bool{}
		for _, arch := range r.arches {
			ctx := build.Default
			ctx.GOOS, ctx.GOARCH, ctx.BuildTags, ctx.CgoEnabled = r.goos, arch, r.tags, true
			for _, fn := range shareEntryPoints {
				var in []string
				for file, d := range defs {
					if d.defines[fn] {
						ok, err := ctx.MatchFile(root, file)
						if err != nil {
							t.Fatalf("matching %s for %s/%s: %v", file, r.goos, arch, err)
						}
						if ok {
							in = append(in, file)
						}
					}
				}
				sort.Strings(in)
				if len(in) != 1 {
					t.Errorf("%s (%s/%s, tags %v): %s is defined in %d files %v, want exactly one",
						r.row, r.goos, arch, r.tags, fn, len(in), in)
					continue
				}
				files[in[0]] = true
			}
		}

		var code []string
		for f := range files {
			code = append(code, f)
		}
		sort.Strings(code)

		cells := table[r.row]
		var doc []string
		seen := map[string]bool{}
		for _, m := range named.FindAllStringSubmatch(cells["Defined in"], -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				doc = append(doc, m[1])
			}
		}
		sort.Strings(doc)
		if strings.Join(code, ",") != strings.Join(doc, ",") {
			t.Errorf("%s: the Sharing table says the share verbs are defined in %v, but the %s release build "+
				"takes nativeShareText and nativeShareImage from %v; update the row (and what it says the "+
				"verbs do) or the code", r.row, doc, r.goos, code)
		}
		if !proof.MatchString(cells["Proof"]) {
			t.Errorf("%s: the Proof cell %q does not open with a proof level from the vocabulary and carry a date",
				r.row, cells["Proof"])
		}
	}

	// Whichever file a platform shares from points back at the table, so a
	// new implementation arrives with the pointer as well as the row.
	for file, d := range defs {
		for fn, ok := range d.defines {
			if ok && !strings.Contains(d.doc[fn], "docs/PLATFORM_MATRIX.md, Sharing") {
				t.Errorf("%s: %s has no comment pointing to docs/PLATFORM_MATRIX.md, Sharing", file, fn)
			}
		}
	}
}

type shareDefinition struct {
	defines map[string]bool
	doc     map[string]string
}

// shareDefinitions finds every non-test file in the package that defines a
// share entry point at top level, as a function or a variable, whatever its
// build constraints. Only files that mention a name are parsed.
func shareDefinitions(t *testing.T, root string) map[string]shareDefinition {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading %s: %v", root, err)
	}
	out := map[string]shareDefinition{}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !strings.Contains(string(src), "nativeShare") {
			continue
		}
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		d := shareDefinition{defines: map[string]bool{}, doc: map[string]string{}}
		for _, decl := range f.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil {
					d.defines[decl.Name.Name] = true
					d.doc[decl.Name.Name] = decl.Doc.Text()
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, id := range vs.Names {
							d.defines[id.Name] = true
							d.doc[id.Name] = decl.Doc.Text() + vs.Doc.Text()
						}
					}
				}
			}
		}
		keep := shareDefinition{defines: map[string]bool{}, doc: map[string]string{}}
		for _, fn := range shareEntryPoints {
			if d.defines[fn] {
				keep.defines[fn] = true
				keep.doc[fn] = d.doc[fn]
			}
		}
		if len(keep.defines) > 0 {
			out[name] = keep
		}
	}
	return out
}

// sharingTable reads the first table under docs/PLATFORM_MATRIX.md's
// "## Sharing" heading into row name -> column name -> cell.
func sharingTable(t *testing.T) map[string]map[string]string {
	t.Helper()
	doc := readRepoFile(t, "docs/PLATFORM_MATRIX.md")
	start := strings.Index(doc, "\n## Sharing\n")
	if start < 0 {
		t.Fatal("docs/PLATFORM_MATRIX.md has no \"## Sharing\" section")
	}
	section := doc[start+1:]
	if next := strings.Index(section[1:], "\n## "); next >= 0 {
		section = section[:next+1]
	}
	split := func(line string) []string {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
		cells := strings.Split(line, "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		return cells
	}
	var header []string
	rows := map[string]map[string]string{}
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			if header != nil {
				break // the table has ended
			}
			continue
		}
		cells := split(line)
		if header == nil {
			header = cells
			continue
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		if len(cells) != len(header) {
			t.Fatalf("Sharing table row %q has %d cells, the header %d", cells[0], len(cells), len(header))
		}
		row := map[string]string{}
		for i, c := range cells {
			row[header[i]] = c
		}
		rows[cells[0]] = row
	}
	for _, col := range []string{"Platform", "Share with note", "Share with citation", "Share as link",
		"Share as image", "Verse of the day", "Defined in", "Proof"} {
		found := false
		for _, h := range header {
			found = found || h == col
		}
		if !found {
			t.Errorf("the Sharing table has no %q column (header %v)", col, header)
		}
	}
	return rows
}

// A LINE NUMBER IN A DOCUMENT IS A CLAIM ABOUT CODE THAT MOVES.
//
// The matrix cites the cause of a divergence down to the line, in the form
// `file.go:N` (`what is on that line`). The quoted code is what makes such a
// citation checkable: a comment added above it, or a function moved, shifts
// the line, and the citation then points at something else while still
// reading as precise. So the quoted code must still be on the cited line.
func TestPlatformMatrixLineCitationsStillPointAtTheirCode(t *testing.T) {
	doc := readRepoFile(t, "docs/PLATFORM_MATRIX.md")
	// Line-wrapped prose can put the quoted code on the next line.
	doc = regexp.MustCompile(`\s+`).ReplaceAllString(doc, " ")
	cite := regexp.MustCompile("`([A-Za-z0-9_./-]+\\.go):(\\d+)` \\(`([^`]+)`\\)")
	matches := cite.FindAllStringSubmatch(doc, -1)
	if len(matches) < 8 {
		t.Fatalf("only %d line citations found; the extraction is broken, so this test proves nothing", len(matches))
	}
	for _, m := range matches {
		file, code := m[1], m[3]
		n, _ := strconv.Atoi(m[2])
		lines := strings.Split(readRepoFile(t, file), "\n")
		if n < 1 || n > len(lines) {
			t.Errorf("docs/PLATFORM_MATRIX.md cites %s:%d, but the file has %d lines", file, n, len(lines))
			continue
		}
		if !strings.Contains(lines[n-1], code) {
			t.Errorf("docs/PLATFORM_MATRIX.md cites %s:%d as %q, but that line is now %q",
				file, n, code, strings.TrimSpace(lines[n-1]))
		}
	}
}
