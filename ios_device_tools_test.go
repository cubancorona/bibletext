package bibletext

// Guards on the iOS device and release scripts that a phone or a store would
// only reveal on the day: the iOS builds compiling their own module wherever
// they are started from, the version check asking its own repository, and a
// locked phone costing a retry rather than a rebuild. The scripts are read to
// show the steps are in place, and the helper they share is run against a
// stand-in for xcrun made to pass and to fail.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// releaseCode is a script or workflow without its comments, as the next
// guard reads them.
func releaseCode(t *testing.T, path string) string {
	t.Helper()
	return strings.Join(nextCodeLines(nextReadFile(t, path)), "\n")
}

// releaseInOrder fails unless each step appears in path after the one before.
func releaseInOrder(t *testing.T, path string, steps ...string) {
	t.Helper()
	c, at := releaseCode(t, path), 0
	for _, s := range steps {
		i := nextIndexAfter(c, s, at)
		if i < 0 {
			t.Errorf("%s: %q is missing, or no longer follows %q", path, s, steps[0])
			return
		}
		at = i + len(s)
	}
}

// THE iOS BUILDS COMPILE THEIR OWN MODULE. Go takes the main module from the
// working directory, not from the package path, so the cross-compile of
// "$REPO_ROOT/cmd/mobile" failed from outside any module ("go.mod file not
// found") and from inside another ("outside main module"), after minutes of
// the slower steps before it. go -C names the module. Mutation: either
// script's cross-compile without -C.
func TestTheIOSBuildsCompileTheirOwnModule(t *testing.T) {
	for _, path := range []string{"scripts/release-ios.sh", "scripts/run-ios-device.sh"} {
		code := releaseCode(t, path)
		if !regexp.MustCompile(`go -C "\$REPO_ROOT" build .*-o "\$WORK/[a-z0-9-]+" \./cmd/mobile`).MatchString(code) {
			t.Errorf("%s: the cross-compile no longer runs as go -C \"$REPO_ROOT\" build ... ./cmd/mobile", path)
		}
		if strings.Contains(code, `"$REPO_ROOT/cmd/mobile"`) {
			t.Errorf("%s: a build still names \"$REPO_ROOT/cmd/mobile\", which resolves against the caller's module", path)
		}
	}
}

// A LOCKED PHONE COSTS A RETRY, NOT A REBUILD. scripts/ios-device-install.sh,
// driven with a stand-in for xcrun: a phone locked for two tries installs on
// the third; one locked throughout is tried the given number of times and no
// more; any other failure is not retried; and a bundle kept for later goes
// into a folder only its owner can read. Mutations guarded: no retry (the
// first case fails), a retry on every error (the third), a world-readable
// kept folder (the last).
func TestALockedPhoneIsTriedAgainAndItsBundleKept(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run-ios-device.sh runs on a Mac; the macOS and Linux jobs run this")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	// The stand-in fails the first $FAIL_TIMES installs with $FAIL_TEXT and
	// counts every call.
	xcrun := `#!/usr/bin/env bash
n=$(( $(cat "$COUNT" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$COUNT"
if [ "$n" -le "$FAIL_TIMES" ]; then echo "$FAIL_TEXT" >&2; exit 1; fi
echo "App installed"
`
	if err := os.WriteFile(filepath.Join(bin, "xcrun"), []byte(xcrun), 0o700); err != nil {
		t.Fatal(err)
	}
	helper, err := filepath.Abs("scripts/ios-device-install.sh")
	if err != nil {
		t.Fatal(err)
	}
	install := func(name string, failTimes int, failText string) (calls string, ok bool, out string) {
		count := filepath.Join(dir, name+".count")
		cmd := exec.Command("bash", "-c", `source "$1"; ios_install_with_retries device-id "$2" 4 0`,
			"_", helper, filepath.Join(dir, "BibleText.app"))
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"COUNT="+count, "FAIL_TIMES="+strconv.Itoa(failTimes), "FAIL_TEXT="+failText, "TMPDIR="+dir)
		b, err := cmd.CombinedOutput()
		c, _ := os.ReadFile(count)
		return strings.TrimSpace(string(c)), err == nil, string(b)
	}
	for _, c := range []struct {
		name      string
		failTimes int
		failText  string
		calls     string
		ok        bool
	}{
		{"locked for two tries", 2, "ERROR: The device is locked. (com.apple.dt.CoreDeviceError error 12040.)", "3", true},
		{"asleep for two tries", 2, "Failed to acquire assertion (error 10003)", "3", true},
		{"locked throughout", 99, "CoreDeviceError error 12040", "4", false},
		{"a refused signature", 99, "ERROR: Failed to install the app on the device. (MIInstallerErrorDomain error 13.)", "1", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			calls, ok, out := install(strings.ReplaceAll(c.name, " ", "-"), c.failTimes, c.failText)
			if calls != c.calls || ok != c.ok {
				t.Errorf("%s installs, ok=%v; want %s, ok=%v\n%s", calls, ok, c.calls, c.ok, out)
			}
		})
	}

	app := filepath.Join(dir, "signed", "BibleText.app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "main"), []byte("keyed"), 0o755); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(dir, "build", "ios-device-kept")
	if err := os.MkdirAll(filepath.Join(kept, "BibleText.app"), 0o755); err != nil { // an earlier run's
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `umask 022; source "$1"; ios_keep_bundle "$2" "$3"`, "_", helper, app, kept)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ios_keep_bundle: %v\n%s", err, out)
	}
	info, err := os.Stat(kept)
	if err != nil {
		t.Fatalf("no kept folder: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("the kept folder is %v; the bundle holds the project key, so it must be 0700", perm)
	}
	if b, err := os.ReadFile(filepath.Join(kept, "BibleText.app", "main")); err != nil || string(b) != "keyed" {
		t.Errorf("the kept folder does not hold this run's bundle (%q, %v)", b, err)
	}
	if _, err := os.Stat(app); !os.IsNotExist(err) {
		t.Error("the bundle was copied, not moved: a keyed copy is left in the work directory")
	}
}

// A COMMENT THAT NAMES THE TEST HOLDING A SCRIPT NAMES ONE THAT EXISTS. The
// scripts and workflows say which Go test holds them, so that whoever changes
// one finds the test to change with it; a name that matches no file sends
// them nowhere. scripts/ios-device-install.sh named ios_device_install_test.go
// for this file. A name with a directory must be that file; a bare name must
// be a test file somewhere in the module.
func TestCommentsNameTestFilesThatExist(t *testing.T) {
	tests := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "third_party" ||
			d.Name() == "build" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go") {
			tests[d.Name()] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tests["ios_device_tools_test.go"] {
		t.Fatalf("the walk did not find this file among %d test files", len(tests))
	}
	named := regexp.MustCompile(`[A-Za-z0-9_./-]*_test\.go\b`)
	var files []string
	for _, dir := range []string{"scripts", "appstore", "msstore", "play", filepath.Join(".github", "workflows")} {
		for _, ext := range []string{"*.sh", "*.py", "*.yml", "*.ps1"} {
			m, _ := filepath.Glob(filepath.Join(dir, ext))
			files = append(files, m...)
		}
	}
	seen := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				continue
			}
			for _, name := range named.FindAllString(line, -1) {
				seen++
				ok := tests[name]
				if strings.Contains(name, "/") {
					_, err := os.Stat(filepath.FromSlash(name))
					ok = err == nil
				}
				if !ok {
					t.Errorf("%s:%d names %s, and no such test file exists", f, i+1, name)
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("no comment in the scripts or workflows names a test file; the scan is not reading them")
	}
}

// run-ios-device.sh installs through the helper, keeps a bundle it could not
// install, and --install-kept installs that bundle with nothing built and no
// key loaded, removing it once it is on the phone. Mutation: delete the bundle
// with the work directory again (no ios_keep_bundle before the failure).
func TestRunIOSDeviceKeepsTheBundleItCouldNotInstall(t *testing.T) {
	const path = "scripts/run-ios-device.sh"
	releaseInOrder(t, path,
		`if ! ios_install_with_retries "$DEVICE_ID" "$APP"; then`,
		`ios_keep_bundle "$APP" "$KEPT_DIR"`,
		`fail "The install did not finish.`,
		`rm -rf "$KEPT_DIR"`,
		`launch "$DEVICE_ID"`)
	code := releaseCode(t, path)
	branch := strings.Index(code, "--install-kept) ;;")
	load := strings.Index(code, "load_release_bible_key")
	if branch < 0 || load < 0 || branch > load {
		t.Fatalf("%s: --install-kept is not handled before the release key is loaded", path)
	}
	kept := code[branch:load]
	for _, want := range []string{"unset BIBLE_API_KEY", `ios_install_with_retries "$DEVICE_ID" "$KEPT_DIR/$APP_NAME"`,
		`rm -rf "$KEPT_DIR"`, "exit 0"} {
		if !strings.Contains(kept, want) {
			t.Errorf("%s: the --install-kept branch lacks %q", path, want)
		}
	}
	if i := strings.Index(kept, "$("); i >= 0 && i < strings.Index(kept, "unset BIBLE_API_KEY") {
		t.Errorf("%s: the --install-kept branch runs a subshell before dropping a raw key", path)
	}
	if strings.Contains(code[:branch], "$(") {
		t.Errorf("%s: a subshell runs before the release key is isolated", path)
	}
}

// THE VERSION CHECK ASKS ITS OWN REPOSITORY. Two repositories, each with an
// origin holding v9.9.9: the script's own, where the tag is HEAD, and the
// caller's, where it is not. Run from the caller's, it must pass, reading its
// own; it used to read the caller's and refuse, or, the other way about, pass
// a spent number. Mutation: plain git, reading the working directory.
func TestTheVersionCheckReadsItsOwnRepository(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the release scripts run on macOS and Linux")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("no git on PATH: %v", err)
	}
	dir := t.TempDir()
	git := func(at string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", at}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
			"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	repo := func(name string, tagHead bool) string {
		work, origin := filepath.Join(dir, name), filepath.Join(dir, name+".git")
		for _, d := range []string{work, origin} {
			if err := os.MkdirAll(d, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		git(origin, "init", "-q", "--bare")
		git(work, "init", "-q")
		git(work, "commit", "-q", "--allow-empty", "-m", "one")
		if !tagHead {
			git(work, "tag", "-a", "v9.9.9", "-m", "v9.9.9")
		}
		git(work, "commit", "-q", "--allow-empty", "-m", "two")
		if tagHead {
			git(work, "tag", "-a", "v9.9.9", "-m", "v9.9.9")
		}
		git(work, "remote", "add", "origin", origin)
		git(work, "push", "-q", "origin", "HEAD:refs/heads/main", "refs/tags/v9.9.9")
		return work
	}
	own, caller := repo("own", true), repo("caller", false)
	if err := os.MkdirAll(filepath.Join(own, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile("scripts/check-version-not-spent.sh")
	if err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(own, "scripts", "check-version-not-spent.sh")
	if err := os.WriteFile(check, script, 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(from string) (string, error) {
		cmd := exec.Command("bash", check, "9.9.9")
		cmd.Dir = from
		cmd.Env = append(os.Environ(), "BIBLETEXT_ALLOW_SPENT_VERSION=")
		b, err := cmd.CombinedOutput()
		return string(b), err
	}
	if out, err := run(own); err != nil {
		t.Fatalf("control: from its own repository, where v9.9.9 is HEAD, the check refused:\n%s", out)
	}
	if out, err := run(caller); err != nil {
		t.Errorf("from another repository, where v9.9.9 is not HEAD, the check read that one and refused:\n%s", out)
	}
	if out, err := run(dir); err != nil {
		t.Errorf("from outside any repository the check refused:\n%s", out)
	}
	// The caller's repository, holding the script itself, still refuses: the
	// check reads whichever repository the script is in.
	if err := os.MkdirAll(filepath.Join(caller, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	callerCheck := filepath.Join(caller, "scripts", "check-version-not-spent.sh")
	if err := os.WriteFile(callerCheck, script, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", callerCheck, "9.9.9")
	cmd.Dir = own
	cmd.Env = append(os.Environ(), "BIBLETEXT_ALLOW_SPENT_VERSION=")
	if out, err := cmd.CombinedOutput(); err == nil || !strings.Contains(string(out), "is SPENT") {
		t.Errorf("control: a script in a repository whose v9.9.9 is not HEAD passed (%v):\n%s", err, out)
	}
}
