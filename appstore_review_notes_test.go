package bibletext

// App Review notes are versioned release artifacts. App Store Connect can copy
// populated review detail into a new version, so presence alone does not show
// that the text belongs to the current release. These tests bind the tracked
// notes to the packaged version and enforce the field's structural and
// credential-safety constraints.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const reviewNotesPath = "appstore/review-notes.txt"
const macReviewNotesPath = "appstore/review-notes-macos.txt"

// The app record carries two platforms, each with its own review-notes field,
// tracked file, and packaging ledger. The same guards apply to both.
var reviewNotesFiles = []struct {
	path, versionConfig string
}{
	{reviewNotesPath, "cmd/mobile/FyneApp.toml"},
	{macReviewNotesPath, "cmd/desktop/FyneApp.toml"},
}

// packagedVersion reads the version a platform actually ships as. FyneApp.toml
// is the packaging source of truth (docs/APP_STORE_SUBMISSION.md), so the notes
// are held to it rather than to a number repeated somewhere else.
func packagedVersion(t *testing.T, config string) string {
	t.Helper()
	b, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("cannot read %s: %v", config, err)
	}
	m := regexp.MustCompile(`(?m)^\s*Version\s*=\s*"([0-9]+(?:\.[0-9]+)*)"`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatalf("no Version in %s — the packaging source of truth moved", config)
	}
	return m[1]
}

func marketingVersion(t *testing.T) string {
	t.Helper()
	return packagedVersion(t, "cmd/mobile/FyneApp.toml")
}

// TestAppReviewNotesAreForThisRelease guards the version-specific review field
// on both platforms.
func TestAppReviewNotesAreForThisRelease(t *testing.T) {
	for _, file := range reviewNotesFiles {
		t.Run(filepath.Base(file.path), func(t *testing.T) {
			raw, err := os.ReadFile(file.path)
			if err != nil {
				t.Fatalf("%s is missing; tracked App Review notes are required for every "+
					"release. (%v)", file.path, err)
			}
			// Normalise line endings before anything is measured: a Windows
			// checkout rewrites LF to CRLF, and counting the \r bytes once put
			// this test 31 characters over the cap on Windows CI alone — for a
			// file the helper (which runs on macOS, reading LF) sends at 3,977.
			// What App Store Connect receives is what must be measured.
			notes := strings.ReplaceAll(string(raw), "\r\n", "\n")
			want := packagedVersion(t, file.versionConfig)

			// App Store Connect limits the field to 4,000 Unicode characters. Count
			// runes rather than bytes so punctuation is measured as the service does.
			if n := len([]rune(notes)); n > 4000 {
				t.Fatalf("%s is %d characters; App Store Connect "+
					"caps the field at 4,000. Remove %d characters.", file.path, n, n-4000)
			}

			// The heading must name the packaged release and no other version.
			first := strings.TrimSpace(strings.SplitN(notes, "\n", 2)[0])
			if !strings.Contains(first, want) {
				t.Errorf("%s opens with %q but the app ships as %s; rewrite the notes for "+
					"%s before submitting", file.path, first, want, want)
			}
			for _, stale := range regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`).FindAllString(first, -1) {
				if stale != want {
					t.Errorf("%s's first line also names version %s; the notes must describe %s alone",
						file.path, stale, want)
				}
			}

			// The notes must give App Review a path through the feature.
			if !regexp.MustCompile(`(?i)\b(to exercise|review path|how to test|to receive|to send)\b`).MatchString(notes) {
				t.Error("the review notes give App Review no way to exercise the app — " +
					"no review path, no steps, nothing to tap")
			}

			// The tracked file is copied into App Store Connect, so credentials belong
			// only in the private review form at submission time.
			for _, pat := range []struct{ name, re string }{
				{"an OpenAI-style key", `\bsk-[A-Za-z0-9_-]{16,}`},
				{"a Google API key", `\bAIza[0-9A-Za-z_-]{20,}`},
				{"an Anthropic key", `\bsk-ant-[A-Za-z0-9_-]{16,}`},
				{"an xAI key", `\bxai-[A-Za-z0-9_-]{16,}`},
				{"a PEM private key", `-----BEGIN [A-Z ]*PRIVATE KEY-----`},
			} {
				if regexp.MustCompile(pat.re).MatchString(notes) {
					t.Errorf("%s contains what looks like %s. This file is tracked and is "+
						"pasted into App Store Connect verbatim — a review-only key goes in "+
						"the ASC form at submission time and is never committed", file.path, pat.name)
				}
			}
		})
	}
}

// The writer is pinned to a tracked release and keeps remote access read-only
// unless an operator supplies both write flags. Local gates and remote
// read-back are part of the same source-level contract.
func TestAppReviewNotesWriterIsPinnedAndGuarded(t *testing.T) {
	raw, err := os.ReadFile("appstore/push-review-notes.py")
	if err != nil {
		t.Fatalf("cannot read App Review notes writer: %v", err)
	}
	src := string(raw)
	wantVersion := marketingVersion(t)

	for name, needle := range map[string]string{
		"packaged-version pin":     `TARGET_VERSION = "` + wantVersion + `"`,
		"exact version filter":     `"filter[versionString]": TARGET_VERSION`,
		"platform-scoped filter":   `"filter[platform]": platform`,
		"iOS default platform":     `default="IOS"`,
		"closed platform choice":   `choices=("IOS", "MAC_OS")`,
		"macOS notes source":       `review-notes-macos.txt`,
		"macOS packaging ledger":   `"cmd", "desktop", "FyneApp.toml"`,
		"write opt-in":             `"--write"`,
		"version confirmation":     `"--confirm-version"`,
		"exact confirmation check": `args.confirm_version != TARGET_VERSION`,
		"editable-state guard":     `state not in EDITABLE_STATES`,
		"support-contact gate":     `check-support-contact.py`,
		"repository-hygiene gate":  `check-repository-hygiene.py`,
		"post-write verification":  `read-back mismatch`,
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("App Review notes writer lacks %s (%q)", name, needle)
		}
	}

	for name, needle := range map[string]string{
		"app-ID environment override":   `os.environ.get("ASC_APP_ID"`,
		"version environment override":  `os.environ.get("ASC_VERSION"`,
		"app-ID command-line override":  `add_argument("--app-id"`,
		"version command-line override": `add_argument("--version"`,
	} {
		if strings.Contains(src, needle) {
			t.Errorf("App Review notes writer contains a forbidden %s", name)
		}
	}

	mainStart := strings.Index(src, "def main(")
	if mainStart < 0 {
		t.Fatal("App Review notes writer has no main function")
	}
	mainSource := src[mainStart:]
	gates := strings.Index(mainSource, "run_repository_gates()")
	remoteRead := strings.Index(mainSource, "version = exact_version(")
	editable := strings.Index(mainSource, "if state not in EDITABLE_STATES:")
	patch := strings.Index(mainSource, `api_request("PATCH"`)
	readBack := strings.Index(mainSource, "read_back = document_data(")
	if gates < 0 || remoteRead < 0 || gates > remoteRead {
		t.Error("repository gates must run before the first authenticated request")
	}
	if editable < 0 || patch < 0 || editable > patch {
		t.Error("editable-state validation must run before PATCH")
	}
	if readBack < 0 || patch > readBack {
		t.Error("a PATCH must be followed by read-back verification")
	}
}

// Shared notes display content supplied by another person, on the Mac exactly
// as on iOS. While the feature ships, both platforms' review notes must explain
// its privacy and rendering boundaries.
func TestAppReviewNotesCoverTheHeadlineFeature(t *testing.T) {
	for _, file := range reviewNotesFiles {
		t.Run(filepath.Base(file.path), func(t *testing.T) {
			raw, err := os.ReadFile(file.path)
			if err != nil {
				t.Skipf("%s missing; the release guard above already reports that", file.path)
			}
			notes := strings.ToLower(string(raw))

			// These claims are properties of share_link.go, notes_store.go, and the note
			// surfaces. A code change that invalidates one must update the release text.
			for _, must := range []struct{ what, needle string }{
				{"that the feature exists at all", "shared notes"},
				{"that there is no server behind it", "no server"},
				{"that a message is rendered as plain text, never markup", "plain text"},
				{"how a recipient turns it off or deletes", "delete"},
			} {
				if !strings.Contains(notes, must.needle) {
					t.Errorf("the review notes do not say %s (looked for %q).\n"+
						"Shared notes display content written by someone else, so this property "+
						"must be explicit for App Review.",
						must.what, must.needle)
				}
			}
		})
	}
}

// Desktop bundles take their version from cmd/desktop/FyneApp.toml, while
// mobile bundles use cmd/mobile/FyneApp.toml. Release artifacts must present a
// single marketing version on every platform.
func TestDesktopAndMobileShipTheSameVersion(t *testing.T) {
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read %s: %v", path, err)
		}
		m := regexp.MustCompile(`(?m)^\s*Version\s*=\s*"([^"]+)"`).FindStringSubmatch(string(b))
		if m == nil {
			t.Fatalf("no Version in %s", path)
		}
		return m[1]
	}
	desktop := read("cmd/desktop/FyneApp.toml")
	mobile := read("cmd/mobile/FyneApp.toml")
	if desktop != mobile {
		t.Errorf("cmd/desktop/FyneApp.toml ships %s but cmd/mobile/FyneApp.toml ships %s.\n\n"+
			"release.yml titles the GitHub Release from the tag and passes no -appVersion,\n"+
			"so the desktop bundles would report %s inside a release announced as %s.\n"+
			"Keep both packaging versions identical.", desktop, mobile, desktop, mobile)
	}
}

// A version-named What's New file prevents a generic "current" file from being
// reused for the wrong release. The live preflight checks inheritance; this
// test checks the local file selected for the packaged version.
func TestWhatsNewIsNamedForThisRelease(t *testing.T) {
	want := marketingVersion(t)
	dir := filepath.Join("build", "appstore", "metadata", "en-GB")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("%s is absent (build/ is gitignored — a fresh clone has no metadata yet)", dir)
	}
	mine := filepath.Join(dir, "whats-new-"+want+".txt")
	b, err := os.ReadFile(mine)
	if err != nil {
		t.Fatalf("no What's New for the shipping version: %s is missing.\n"+
			"the metadata helper reads the file named for the version so there "+
			"is no \"current\" file to go stale. (%v)", mine, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		t.Fatalf("%s is empty", mine)
	}
	// App Store Connect applies the same 4,000-character limit to What's New.
	if n := len([]rune(string(b))); n > 4000 {
		t.Fatalf("%s is %d characters; App Store Connect caps What's New at 4,000", mine, n)
	}
	// And it must not merely be a copy of another release's notes.
	others, _ := filepath.Glob(filepath.Join(dir, "whats-new-*.txt"))
	for _, o := range others {
		if o == mine {
			continue
		}
		ob, err := os.ReadFile(o)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(ob)) == strings.TrimSpace(string(b)) {
			t.Errorf("%s is byte-identical to %s — the release notes for %s describe "+
				"a different release", mine, o, want)
		}
	}
}

// Android release identity comes only from the tracked FyneApp ledger. An
// environment override could produce a package that passes artifact checks but
// does not match the tagged source.
func TestAndroidReleaseIdentityComesFromTrackedLedger(t *testing.T) {
	b, err := os.ReadFile("scripts/build-android.sh")
	if err != nil {
		t.Fatalf("cannot read scripts/build-android.sh: %v", err)
	}
	src := string(b)
	if strings.Contains(src, "${BIBLETEXT_ANDROID_BUILD:-") ||
		strings.Contains(src, "${BIBLETEXT_ANDROID_VERSION:-") {
		t.Error("build-android.sh permits an environment release-identity override; " +
			"derive both values only from cmd/mobile/FyneApp.toml")
	}
	if !strings.Contains(src, `APP_VERSION="$TRACKED_APP_VERSION"`) ||
		!strings.Contains(src, `APP_BUILD="$TRACKED_APP_BUILD"`) {
		t.Error("build-android.sh does not use the tracked Android release identity")
	}
	if !strings.Contains(src, "refusing to default the versionCode to 1") {
		t.Error("the hard stop for an unreadable Build is gone — an unreadable " +
			"ledger must fail the release, never quietly become 1")
	}
}
