package bibletext

// The guard that keeps the next major release out of every release build
// (next_off.go, docs/NEXT.md).
//
// The switch is a build tag, so a release can carry it only if something hands
// the tag to the build. Two checks stand in the way, and this file holds both:
//
//   - The text of every release path is read, and any tag list that names
//     next, directly or through a variable it expands, fails. So does a tag
//     list the scan cannot resolve: a guard that cannot see a value has not
//     shown it is safe.
//   - Each release path runs scripts/verify-not-next.sh on what it built. That
//     reads the build tags Go records inside the binary, so it also catches the
//     tag arriving through GOFLAGS in the environment or saved with `go env -w`,
//     which no script text shows. This file asserts every path runs it, and
//     that it refuses binaries built with the tag each of those ways.
//
// The file has no build constraint: it runs in the shipping build and in the
// next build alike, since what it reads is the same in both.

import (
	"archive/zip"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// nextReleasePaths build or publish what a reader receives: the store and
// GitHub builds of every platform, and the website.
var nextReleasePaths = []string{
	"scripts/release-ios.sh",
	"scripts/release-mac-store.sh",
	"scripts/build-android.sh",
	"scripts/build-windows-exe.sh",
	"scripts/build-appimage.sh",
	"scripts/go-release-wrapper.sh",
	"scripts/publish-site.sh",
	".github/workflows/release.yml",
	".github/workflows/msstore.yml",
	".github/workflows/linux-stores.yml",
}

// nextToleratedTagVars are tag lists a release path takes from the
// environment. Each is tolerated only while that path refuses it on the
// release branch outright, and the refusal is asserted rather than trusted.
var nextToleratedTagVars = map[string]struct{ path, refusal string }{
	"BT_ANDROID_TAGS": {
		path:    "scripts/build-android.sh",
		refusal: "BT_ANDROID_TAGS is set — extra build tags are debug-APK only",
	},
}

var (
	// A tag list: -tags or --tags, then = or blanks, then one quoted or bare
	// word. The character before the flag may not be a word character or a
	// hyphen, so --tags is read once, and a word ending in "tags" never is.
	nextTagListRE = regexp.MustCompile(`(?:^|[^\w-])--?tags(?:=|[ \t]+)("[^"]*"|'[^']*'|[^\s;|&)]+)`)
	// NAME=value in a shell, with an optional export/local/readonly.
	nextShellAssignRE = regexp.MustCompile(`(?:^|[\s;("'{])(?:export[ \t]+|local[ \t]+|readonly[ \t]+)?([A-Za-z_]\w*)\+?=("[^"]*"|'[^']*'|\S*)`)
	// NAME: value in a workflow's env block.
	nextYAMLAssignRE = regexp.MustCompile(`^\s*([A-Za-z_]\w*):[ \t]+(.+)$`)
	// $env:NAME = value or $NAME = value in PowerShell.
	nextPwshAssignRE = regexp.MustCompile(`\$(?:env:)?([A-Za-z_]\w*)[ \t]*=[ \t]*(.+)$`)
	// A variable a value expands: $NAME, ${NAME…}, $env:NAME.
	nextVarRefRE = regexp.MustCompile(`\$\{?(?:env:)?([A-Za-z_]\w*)`)
	// What is left of a value once its variables are taken out.
	nextExprRE       = regexp.MustCompile(`\$\{\{[^}]*\}\}`)
	nextBracedRefRE  = regexp.MustCompile(`\$\{[^}]*\}`)
	nextPlainRefRE   = regexp.MustCompile(`\$(?:env:)?\w+`)
	nextTagSplitRE   = regexp.MustCompile(`[^A-Za-z0-9_.]+`)
	nextTrailingNote = regexp.MustCompile(`\s#.*$`)
)

// nextCodeLines returns a script's or workflow's lines with comments taken
// out and continued lines joined, so a sentence about the tag is never read as
// a use of it, and a flag split over two lines is read as one.
func nextCodeLines(text string) []string {
	var out []string
	continued := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		line = strings.TrimRight(nextTrailingNote.ReplaceAllString(line, ""), " \t")
		if line == "" {
			continue
		}
		// A shell line ending in a backslash, or a PowerShell one in a
		// backtick, goes on in the next.
		more := strings.HasSuffix(line, "\\") || strings.HasSuffix(line, "`")
		if more {
			line = line[:len(line)-1]
		}
		if continued {
			out[len(out)-1] += " " + strings.TrimSpace(line)
		} else {
			out = append(out, line)
		}
		continued = more
	}
	return out
}

// nextTagScan is one file's code, with what each variable is assigned.
type nextTagScan struct {
	lines   []string
	assigns map[string][]string
	allowed map[string]bool
}

func newNextTagScan(text string, allowed map[string]bool) *nextTagScan {
	s := &nextTagScan{lines: nextCodeLines(text), assigns: map[string][]string{}, allowed: allowed}
	for _, line := range s.lines {
		for _, m := range nextShellAssignRE.FindAllStringSubmatch(line, -1) {
			s.assigns[m[1]] = append(s.assigns[m[1]], m[2])
		}
		for _, re := range []*regexp.Regexp{nextYAMLAssignRE, nextPwshAssignRE} {
			if m := re.FindStringSubmatch(line); m != nil {
				s.assigns[m[1]] = append(s.assigns[m[1]], m[2])
			}
		}
	}
	return s
}

// nextLiteralTags splits what is left of a value, once its variables are taken
// out, into the words a tag list would read.
func nextLiteralTags(value string) []string {
	v := nextExprRE.ReplaceAllString(value, " ")
	v = nextBracedRefRE.ReplaceAllString(v, " ")
	v = nextPlainRefRE.ReplaceAllString(v, " ")
	return nextTagSplitRE.Split(v, -1)
}

// check reports why a value could put next into a tag list, if it could:
// next among its own words, an expansion the scan cannot follow, or a
// variable whose assignments could.
func (s *nextTagScan) check(value string, seen map[string]bool) []string {
	var why []string
	for _, w := range nextLiteralTags(value) {
		if w == "next" {
			why = append(why, "names next")
			break
		}
	}
	// A workflow expression, a command's output, a positional parameter: none
	// has a value in the file to follow.
	rest := nextVarRefRE.ReplaceAllString(value, "")
	if strings.Contains(rest, "$") || strings.Contains(rest, "`") {
		why = append(why, "expands something the scan cannot read ("+strings.TrimSpace(value)+")")
	}
	for _, m := range nextVarRefRE.FindAllStringSubmatch(value, -1) {
		name := m[1]
		if seen[name] || s.allowed[name] {
			continue
		}
		seen[name] = true
		values, ok := s.assigns[name]
		if !ok {
			why = append(why, "takes $"+name+", which nothing in the file assigns")
			continue
		}
		for _, v := range values {
			for _, w := range s.check(v, seen) {
				why = append(why, "takes $"+name+", which "+w)
			}
		}
	}
	return why
}

// nextTagFindings returns every tag list in a release path's text that names
// next or cannot be resolved. allowed names environment variables the caller
// has separately shown to be refused on the release branch.
func nextTagFindings(text string, allowed map[string]bool) []string {
	s := newNextTagScan(text, allowed)
	var out []string
	for _, line := range s.lines {
		for _, m := range nextTagListRE.FindAllStringSubmatch(line, -1) {
			for _, why := range s.check(m[1], map[string]bool{}) {
				out = append(out, strings.TrimSpace(m[0])+": the tag list "+why)
			}
		}
	}
	return out
}

func nextReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return string(b)
}

func nextAllowedVarsFor(t *testing.T, path, text string) map[string]bool {
	t.Helper()
	allowed := map[string]bool{}
	code := strings.Join(nextCodeLines(text), "\n")
	for name, tol := range nextToleratedTagVars {
		if tol.path != path {
			continue
		}
		if !strings.Contains(code, tol.refusal) {
			t.Errorf("%s takes build tags from $%s but no longer refuses it on the release branch "+
				"(want %q in its code)", path, name, tol.refusal)
			continue
		}
		allowed[name] = true
	}
	return allowed
}

func TestReleasePathsNeverPassTheNextTag(t *testing.T) {
	for _, path := range nextReleasePaths {
		text := nextReadFile(t, path)
		for _, f := range nextTagFindings(text, nextAllowedVarsFor(t, path, text)) {
			t.Errorf("%s: %s — a release build must never compile the next major release (docs/NEXT.md)", path, f)
		}
	}
}

// The development scripts pass the tag on purpose. They are the positive
// control for the scan above: if it cannot find the tag where it is known to
// be, its silence about the release paths proves nothing.
func TestDevScriptsOfferTheNextFlag(t *testing.T) {
	for _, path := range []string{"scripts/run-ios-device.sh", "scripts/run-ios-sim.sh"} {
		text := nextReadFile(t, path)
		code := strings.Join(nextCodeLines(text), "\n")
		if !strings.Contains(code, "--next)") {
			t.Errorf("%s no longer accepts --next", path)
		}
		found := false
		for _, f := range nextTagFindings(text, nil) {
			if strings.Contains(f, "names next") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: the scan finds no next tag in a script that passes it — the release-path scan is blind", path)
		}
	}
	for _, path := range []string{"scripts/check-ios-pane.sh", "scripts/check-android-pane.sh"} {
		code := strings.Join(nextCodeLines(nextReadFile(t, path)), "\n")
		if !strings.Contains(code, "--next) TAGS=next") {
			t.Errorf("%s no longer compiles the next build on --next", path)
		}
	}
}

// Planted controls for the scan: every way of reaching a tag list it is meant
// to catch, and the look-alikes it must leave alone.
func TestNextTagScanCatchesPlantedTags(t *testing.T) {
	for _, tc := range []struct {
		name    string
		text    string
		allowed []string
		want    bool
	}{
		{"bare flag", "go build -tags next ./cmd/mobile\n", nil, true},
		{"equals form", "go build -tags=next .\n", nil, true},
		{"quoted list", `go build -tags "ios,next" .` + "\n", nil, true},
		{"single-quoted list", `go build -tags 'gles,next' .` + "\n", nil, true},
		{"packager flag", "fyne package -os windows --tags gles,next\n", nil, true},
		{"through a variable", "T=\"$T,next\"\ngo build -tags \"ios$T\" .\n", nil, true},
		{"through two variables", "A=next\nB=\"x,$A\"\ngo build -tags \"$B\" .\n", nil, true},
		{"through ${VAR#,}", "T=\",next\"\ngo build ${T:+-tags \"${T#,}\"} .\n", nil, true},
		{"GOFLAGS in a shell", "export GOFLAGS=\"-tags=next\"\ngo build .\n", nil, true},
		{"GOFLAGS through a variable", "X=next\nGOFLAGS=\"-tags=$X\" go build .\n", nil, true},
		{"GOFLAGS in a workflow env block", "    env:\n      GOFLAGS: -tags=next\n", nil, true},
		{"PowerShell env", "$env:GOFLAGS = \"-tags=next\"\n", nil, true},
		{"flag continued onto the next line", "go build -trimpath -tags \\\n  next -o x .\n", nil, true},
		{"unassigned variable", "go build -tags \"$EXTRA\" .\n", nil, true},
		{"workflow expression", "go build -tags ${{ matrix.tags }} .\n", nil, true},
		{"command substitution", "T=$(cat tags.txt)\ngo build -tags \"$T\" .\n", nil, true},
		{"positional parameter", "go build -tags \"$1\" .\n", nil, true},
		{"environment variable not shown refused", "fyne package ${BT_ANDROID_TAGS:+--tags \"$BT_ANDROID_TAGS\"}\n", nil, true},

		{"another tag", "go build -tags gles -trimpath .\n", nil, false},
		{"several other tags", "go build -tags \"ios,bibletextdev\" .\n", nil, false},
		{"a comment line", "# never pass -tags next to a release build\n", nil, false},
		{"a trailing comment", "go build . # -tags next is for development only\n", nil, false},
		{"python next()", "path = next((p for p in paths), None)\n", nil, false},
		{"awk next", "awk '/x/ { next } { print }'\n", nil, false},
		{"git's tag flag", "git ls-remote --tags origin \"refs/tags/$TAG\"\n", nil, false},
		{"workflow trigger", "on:\n  push:\n    tags: [\"v*\"]\n", nil, false},
		{"environment variable shown refused", "fyne package ${BT_ANDROID_TAGS:+--tags \"$BT_ANDROID_TAGS\"}\n", []string{"BT_ANDROID_TAGS"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowed := map[string]bool{}
			for _, a := range tc.allowed {
				allowed[a] = true
			}
			got := nextTagFindings(tc.text, allowed)
			if (len(got) > 0) != tc.want {
				t.Errorf("findings %q; want any = %v", got, tc.want)
			}
		})
	}
}

// nextIndexAfter is the index of want in code at or after from, or -1.
func nextIndexAfter(code, want string, from int) int {
	if from < 0 {
		return -1
	}
	i := strings.Index(code[from:], want)
	if i < 0 {
		return -1
	}
	return from + i
}

// Every release path checks what it built, between building it and letting it
// go anywhere.
func TestReleasePathsVerifyTheirArtifactsAreNotNext(t *testing.T) {
	code := func(path string) string {
		return strings.Join(nextCodeLines(nextReadFile(t, path)), "\n")
	}
	// Each step must appear after the one before it.
	inOrder := func(path string, steps ...string) {
		t.Helper()
		c, at := code(path), 0
		for _, s := range steps {
			i := nextIndexAfter(c, s, at)
			if i < 0 {
				t.Errorf("%s: %q is missing, or no longer follows %q", path, s, steps[0])
				return
			}
			at = i + len(s)
		}
	}

	// The desktop packages, from GitHub and the Mac App Store and Microsoft
	// Store and Snap Store, all pass through the package verifier.
	inOrder("scripts/verify-release-package.sh",
		`go version -m "$binary_path"`,
		`verify-not-next.sh" "$binary_path"`)
	for _, path := range []string{
		"scripts/release-mac-store.sh",
		"scripts/build-windows-exe.sh",
		".github/workflows/release.yml",
		".github/workflows/linux-stores.yml",
	} {
		if !strings.Contains(code(path), "verify-release-package.sh") {
			t.Errorf("%s builds a desktop package and no longer runs scripts/verify-release-package.sh", path)
		}
	}
	if !strings.Contains(code(".github/workflows/msstore.yml"), "scripts/build-windows-exe.sh") {
		t.Errorf(".github/workflows/msstore.yml no longer builds through scripts/build-windows-exe.sh")
	}

	// The App Store build: the cross-compiled binary, before it is signed.
	inOrder("scripts/release-ios.sh",
		`-o "$WORK/bibletext-arm64"`,
		`verify-not-next.sh" "$WORK/bibletext-arm64"`,
		`mv -f "$WORK/bibletext-arm64"`)

	// Play: the bundle fyne released, before it is signed and published.
	inOrder("scripts/build-android.sh",
		`fyne release -os android`,
		`verify-not-next.sh" "$AAB_STAGE"`,
		`mv -f "$AAB_TMP" "$DIST_DIR/BibleText.aab"`)

	// The website: the generator, before it writes a page.
	inOrder("scripts/publish-site.sh",
		`go build -o build/websitegen ./cmd/websitegen`,
		`scripts/verify-not-next.sh build/websitegen`,
		`build/websitegen -out "$OUT"`)
}

// The verifier itself, against binaries built with the tag each way it can
// arrive, plus packages and files it must refuse because it cannot read them.
func TestVerifyNotNextRefusesTaggedBuilds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the verifier is a bash script; the linux and macOS jobs run this")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("no go on PATH to build the probes with: %v", err)
	}
	if _, err := exec.LookPath("unzip"); err != nil {
		t.Fatalf("no unzip on PATH, which the verifier needs for .apk and .aab: %v", err)
	}
	dir := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":  "module nextprobe\n\ngo 1.21\n",
		"main.go": "package main\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goEnvFile := filepath.Join(dir, "go.env")
	if err := os.WriteFile(goEnvFile, []byte("GOFLAGS=-tags=next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseEnv := func() []string {
		var env []string
		for _, kv := range os.Environ() {
			if strings.HasPrefix(kv, "GOFLAGS=") || strings.HasPrefix(kv, "GOENV=") {
				continue
			}
			env = append(env, kv)
		}
		return append(env, "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=", "GOENV=off")
	}
	build := func(name string, env []string, args ...string) string {
		t.Helper()
		out := filepath.Join(dir, name)
		cmd := exec.Command(goBin, append(append([]string{"build"}, args...), "-o", out, ".")...)
		cmd.Dir = dir
		cmd.Env = append(baseEnv(), env...)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("building the %s probe: %v\n%s", name, err, b)
		}
		return out
	}
	pack := func(name, entry, binary string) string {
		t.Helper()
		out := filepath.Join(dir, name)
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		zw := zip.NewWriter(f)
		w, err := zw.Create("AndroidManifest.xml")
		if err == nil {
			_, err = w.Write([]byte("<manifest/>"))
		}
		if err == nil && entry != "" {
			var b []byte
			if b, err = os.ReadFile(binary); err == nil {
				if w, err = zw.Create(entry); err == nil {
					_, err = w.Write(b)
				}
			}
		}
		if err == nil {
			err = zw.Close()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	plain := build("plain", nil, "-trimpath")
	otherTags := build("other-tags", nil, "-tags", "gles,nextish")
	onTheLine := build("on-the-command-line", nil, "-tags", "ios,next")
	fromEnv := build("from-goflags", []string{"GOFLAGS=-tags=next"})
	fromGoEnv := build("from-go-env-w", []string{"GOENV=" + goEnvFile})
	notGo := filepath.Join(dir, "not-go")
	if err := os.WriteFile(notGo, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		artifact string
		refusal  string // empty: must pass
	}{
		{"a plain build", plain, ""},
		{"other tags, one of them starting with next", otherTags, ""},
		{"a bundle of a plain build", pack("plain.aab", "base/lib/arm64-v8a/libBibleText.so", plain), ""},
		{"tag on the command line", onTheLine, "built with the next tag"},
		{"tag from GOFLAGS", fromEnv, "built with the next tag"},
		{"tag from go env -w", fromGoEnv, "built with the next tag"},
		{"a bundle carrying it", pack("next.aab", "base/lib/arm64-v8a/libBibleText.so", onTheLine), "built with the next tag"},
		{"an APK carrying it", pack("next.apk", "lib/arm64-v8a/libBibleText.so", fromEnv), "built with the next tag"},
		{"a package with no native library", pack("empty.aab", "", ""), "no native library"},
		{"a file Go did not build", notGo, "no Go build information"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("bash", "scripts/verify-not-next.sh", tc.artifact)
			cmd.Env = append(baseEnv(), "BIBLETEXT_REAL_GO="+goBin)
			out, err := cmd.CombinedOutput()
			switch {
			case tc.refusal == "" && err != nil:
				t.Errorf("refused: %v\n%s", err, out)
			case tc.refusal != "" && err == nil:
				t.Errorf("passed; want it refused (%s)\n%s", tc.refusal, out)
			case tc.refusal != "" && !strings.Contains(string(out), tc.refusal):
				t.Errorf("refused for another reason; want %q\n%s", tc.refusal, out)
			}
		})
	}
}

// The two switch files must declare opposite states under opposite
// constraints. An edit that sets nextRelease true in the shipping file, or
// drops a constraint, would ship the next release with no tag anywhere for the
// guards above to find.
func TestNextSwitchFilesDeclareOppositeStates(t *testing.T) {
	for _, tc := range []struct {
		path    string
		withTag bool // the file builds when next is set
		value   string
	}{
		{"next_on.go", true, "true"},
		{"next_off.go", false, "false"},
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, tc.path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		var expr constraint.Expr
		for _, g := range f.Comments {
			for _, c := range g.List {
				if constraint.IsGoBuild(c.Text) {
					if expr, err = constraint.Parse(c.Text); err != nil {
						t.Fatalf("%s: %v", tc.path, err)
					}
				}
			}
		}
		if expr == nil {
			t.Errorf("%s has no //go:build line; it would build in both states", tc.path)
			continue
		}
		for _, next := range []bool{true, false} {
			builds := expr.Eval(func(tag string) bool { return tag == "next" && next })
			if builds != (next == tc.withTag) {
				t.Errorf("%s: with next=%v it builds=%v; want %v", tc.path, next, builds, next == tc.withTag)
			}
		}
		var got []string
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if n.Name == "nextRelease" && i < len(vs.Values) {
						if id, ok := vs.Values[i].(*ast.Ident); ok {
							got = append(got, id.Name)
						}
					}
				}
			}
		}
		if len(got) != 1 || got[0] != tc.value {
			t.Errorf("%s declares nextRelease = %v; want exactly %s", tc.path, got, tc.value)
		}
	}
}

// Both states must keep being built and tested on every push, or the next
// build rots unseen between now and its release.
func TestCIBuildsBothStatesOfTheNextSwitch(t *testing.T) {
	lines := nextCodeLines(nextReadFile(t, ".github/workflows/ci.yml"))
	tagged := func(cmd string) bool {
		for _, line := range lines {
			if !strings.Contains(line, cmd) {
				continue
			}
			for _, m := range nextTagListRE.FindAllStringSubmatch(line, -1) {
				for _, w := range nextLiteralTags(m[1]) {
					if w == "next" {
						return true
					}
				}
			}
		}
		return false
	}
	var missing []string
	for _, cmd := range []string{"go vet", "go test -race", "go test -timeout"} {
		if !tagged(cmd) {
			missing = append(missing, cmd+" with -tags next")
		}
	}
	code := strings.Join(lines, "\n")
	for _, want := range []string{"scripts/check-ios-pane.sh --next", "scripts/check-android-pane.sh --next"} {
		if !strings.Contains(code, want) {
			missing = append(missing, want)
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf(".github/workflows/ci.yml no longer runs %s", m)
	}
}
