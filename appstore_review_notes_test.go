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
	"strconv"
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
	{macReviewNotesPath, "cmd/bibletext/FyneApp.toml"},
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

// App Review notes name no release but their own, anywhere in them. Notes
// whose heading was bumped over the last release's body name that release
// below the heading, and a comparison with an earlier release ("(1.0.0: ...)")
// names it too; either would go to App Review describing a release it is not
// reviewing. There is no allow-list: an x.y.z that is not the packaged version
// fails wherever it stands, the version of an operating system included.
// appstore/push-review-notes.py refuses the same notes before it sends them.
//
// Until 1.2.19 only the heading was read, and the notes up to then were
// written and sent under that rule; 1.2.19's iPhone notes compare two results
// with 1.2.18's. The whole notes are read from 1.2.20, the first release
// whose notes are written under this rule, so the tracked tree stays green
// while it still holds 1.2.19's, and the version bump to 1.2.20 is what makes
// those comparisons fail.
const reviewNotesWholeTextFrom = "1.2.20"

var reviewNotesVersion = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`)

// reviewNotesOtherVersions returns every x.y.z in text other than want, in
// the order they appear.
func reviewNotesOtherVersions(text, want string) []string {
	var others []string
	for _, v := range reviewNotesVersion.FindAllString(text, -1) {
		if v != want {
			others = append(others, v)
		}
	}
	return others
}

// reviewNotesStaleVersions is the version rule as the tracked notes are held
// to it: every x.y.z in notes other than want, read from the whole notes for
// a release from reviewNotesWholeTextFrom on and from the first line before
// it, with what was read, for the message.
func reviewNotesStaleVersions(notes, want string) (stale []string, where string) {
	first := strings.TrimSpace(strings.SplitN(notes, "\n", 2)[0])
	if !versionAtLeast(want, reviewNotesWholeTextFrom) {
		return reviewNotesOtherVersions(first, want), "names in its first line"
	}
	return reviewNotesOtherVersions(notes, want), "names"
}

// versionAtLeast compares two dotted versions number by number, so 1.2.100
// is after 1.2.20. A part that is not a number compares as 0.
func versionAtLeast(version, floor string) bool {
	a, b := strings.Split(version, "."), strings.Split(floor, ".")
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x, _ = strconv.Atoi(a[i])
		}
		if i < len(b) {
			y, _ = strconv.Atoi(b[i])
		}
		if x != y {
			return x > y
		}
	}
	return true
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

			// The heading must name the packaged release, and the notes no
			// other version anywhere (reviewNotesWholeTextFrom).
			first := strings.TrimSpace(strings.SplitN(notes, "\n", 2)[0])
			if !strings.Contains(first, want) {
				t.Errorf("%s opens with %q but the app ships as %s; rewrite the notes for "+
					"%s before submitting", file.path, first, want, want)
			}
			if !versionAtLeast(want, reviewNotesWholeTextFrom) {
				t.Logf("%s is for %s, before %s: only its first line is held to the version",
					file.path, want, reviewNotesWholeTextFrom)
			}
			others, where := reviewNotesStaleVersions(notes, want)
			for _, stale := range others {
				t.Errorf("%s %s version %s; App Review notes name no release but %s, the one "+
					"they are for: rewrite a body carried over from another release, and leave "+
					"out comparisons with one", file.path, where, stale, want)
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

// CONTROL for the version rule above, on notes made for it: the guard must
// read past the heading, or a body bumped under a new heading passes.
func TestTheReviewNotesVersionRuleReadsTheWholeNotes(t *testing.T) {
	const want = "9.9.9"
	for _, c := range []struct {
		name, notes string
		others      []string
	}{
		{"last release's body under a bumped heading",
			"VERSION 9.9.9 (macOS)\n\nWHAT IS NEW IN 9.9.8\nA change.\n\nHOW TO EXERCISE 9.9.8\nOpen it.\n",
			[]string{"9.9.8", "9.9.8"}},
		{"a comparison with an earlier release",
			"VERSION 9.9.9\n\nThe list includes \"A 1:2\" (9.9.8: \"A 1:1\").\n",
			[]string{"9.9.8"}},
		{"an operating system's version", "VERSION 9.9.9\n\nOn macOS 14.0.1, open it.\n",
			[]string{"14.0.1"}},
		{"the release's own version alone", "VERSION 9.9.9\n\nWHAT IS NEW IN 9.9.9\nA change.\n", nil},
		{"verse references and two-part numbers", "VERSION 9.9.9\n\nGenesis 1:1 and Psalm 119:1, in 2.5 seconds.\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := reviewNotesOtherVersions(c.notes, want)
			if strings.Join(got, ",") != strings.Join(c.others, ",") {
				t.Errorf("reviewNotesOtherVersions found %q, want %q", got, c.others)
			}
		})
	}
	// The heading alone, which is all the guard read before 1.2.20, passes the
	// bumped body: the case the whole-text rule exists for.
	if got := reviewNotesOtherVersions("VERSION 9.9.9 (macOS)", want); len(got) != 0 {
		t.Errorf("the bumped heading alone names %q", got)
	}
	// The rule as the tracked files meet it, through the version gate. The
	// tree holds 1.2.19's notes until the bump, so the tracked files take the
	// heading-only branch, and only notes made at 1.2.20 and after show the
	// whole notes are read where they must be. Mutation: the gated branch
	// reading the first line again.
	bumped := func(v, before string) string {
		return "VERSION " + v + " (macOS)\n\nWHAT IS NEW IN " + before + "\nA change.\n\n" +
			"HOW TO EXERCISE IT\nOpen it (" + before + ": closed).\n"
	}
	for _, c := range []struct {
		version, notes string
		stale          []string
		where          string
	}{
		{"1.2.20", bumped("1.2.20", "1.2.19"), []string{"1.2.19", "1.2.19"}, "names"},
		{"1.2.21", bumped("1.2.21", "1.2.20"), []string{"1.2.20", "1.2.20"}, "names"},
		{"1.10.0", bumped("1.10.0", "1.9.9"), []string{"1.9.9", "1.9.9"}, "names"},
		{"1.2.20", "VERSION 1.2.20\n\nWHAT IS NEW IN 1.2.20\nA change.\n", nil, "names"},
		{"1.2.19", bumped("1.2.19", "1.2.18"), nil, "names in its first line"},
		{"1.2.19", "VERSION 1.2.19, after 1.2.18\n\nA change.\n", []string{"1.2.18"}, "names in its first line"},
	} {
		stale, where := reviewNotesStaleVersions(c.notes, c.version)
		if strings.Join(stale, ",") != strings.Join(c.stale, ",") || where != c.where {
			t.Errorf("notes for %s, %q: reviewNotesStaleVersions found %q (%s), want %q (%s)",
				c.version, c.notes, stale, where, c.stale, c.where)
		}
	}
	for _, c := range []struct {
		version, floor string
		want           bool
	}{
		{"1.2.19", "1.2.20", false},
		{"1.2.20", "1.2.20", true},
		{"1.2.21", "1.2.20", true},
		{"1.2.100", "1.2.20", true},
		{"1.2.9", "1.2.20", false},
		{"1.10.0", "1.2.20", true},
		{"2.0.0", "1.2.20", true},
		{"0.9.99", "1.2.20", false},
	} {
		if got := versionAtLeast(c.version, c.floor); got != c.want {
			t.Errorf("versionAtLeast(%q, %q) = %v, want %v", c.version, c.floor, got, c.want)
		}
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
		"macOS packaging ledger":   `"cmd", "bibletext", "FyneApp.toml"`,
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

// The iOS notes say where App Review will find Read, Books and Search, and that
// paragraph carries from release to release: the version bump touches only the
// first line. It said for releases that an iPhone held sideways kept the bottom
// bar on Books and Search, and since 1.2.18 every phone held sideways has the
// rail (mobileRailWanted, layout.go). While the rule gives a sideways phone the
// rail, no sentence of the notes that speaks of an iPhone sideways may put its
// navigation at the bottom, and the notes must name the rail.
func TestAppReviewNotesPlaceTheSidewaysIPhoneNavigationByTheRule(t *testing.T) {
	if !mobileRailWanted(false, 874, 402) {
		t.Skip("the rule gives a sideways phone the bar, so the notes have nothing to contradict")
	}
	raw, err := os.ReadFile(reviewNotesPath)
	if err != nil {
		t.Skipf("%s missing; the release guard above already reports that", reviewNotesPath)
	}
	notes := strings.ToLower(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	if !strings.Contains(notes, "rail") {
		t.Errorf("%s never names the rail, which every phone and tablet held sideways shows",
			reviewNotesPath)
	}
	sideways := regexp.MustCompile(`\b(landscape|sideways)\b`)
	atTheBottom := regexp.MustCompile(`\bbottom\b|\btab bar\b`)
	for _, sentence := range regexp.MustCompile(`[.!?\n]+`).Split(notes, -1) {
		if strings.Contains(sentence, "iphone") && sideways.MatchString(sentence) &&
			atTheBottom.MatchString(sentence) {
			t.Errorf("%s puts a sideways iPhone's navigation at the bottom:\n  %q\n"+
				"a phone held sideways shows the rail on Books and Search (mobileRailWanted)",
				reviewNotesPath, strings.TrimSpace(sentence))
		}
	}
}

// Desktop bundles take their version from cmd/bibletext/FyneApp.toml, while
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
	desktop := read("cmd/bibletext/FyneApp.toml")
	mobile := read("cmd/mobile/FyneApp.toml")
	if desktop != mobile {
		t.Errorf("cmd/bibletext/FyneApp.toml ships %s but cmd/mobile/FyneApp.toml ships %s.\n\n"+
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
	// The Mac has its own What's New, in its own file: the two releases say
	// different things, and App Store Connect refuses a review submission for
	// a version whose What's New is empty. 1.2.8's Mac submission was refused
	// for exactly this before the file existed; the helper used to announce
	// the absence as a note and write nothing.
	macDir := filepath.Join(dir, "mac")
	if _, err := os.Stat(macDir); err == nil {
		mac := filepath.Join(macDir, "whats-new-"+want+".txt")
		mb, err := os.ReadFile(mac)
		if err != nil {
			t.Errorf("no Mac What's New for the shipping version: %s is missing — the Mac "+
				"submission will be refused (%v)", mac, err)
		} else if len(strings.TrimSpace(string(mb))) == 0 {
			t.Errorf("%s is empty", mac)
		}
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

// App Store Connect refuses What's New text containing the Unicode small
// capitals the app draws the divine name in: ERROR 409
// INVALID_CHARACTERS, naming "ᴅ" (U+1D05) and "ᴏ" (U+1D0F), on the 1.2.14
// submission. The refusal came before any review submission was created, so
// nothing was lost, but it stops a release at its last step. Describe the name
// ("in small capitals") instead of showing it. Google Play's release notes
// accepted the same characters, so this is App Store Connect's rule alone.
//
// The check covers the whole block the app substitutes from (small_caps_draw.go
// maps from three: IPA Extensions, Phonetic Extensions and Latin Extended-D),
// since App Store Connect named only the two it met and the rest are no safer.
func TestWhatsNewCarriesNoDrawnSmallCapitals(t *testing.T) {
	want := marketingVersion(t)
	dir := filepath.Join("build", "appstore", "metadata", "en-GB")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("%s is absent (build/ is gitignored — a fresh clone has no metadata yet)", dir)
	}
	for _, p := range []string{
		filepath.Join(dir, "whats-new-"+want+".txt"),
		filepath.Join(dir, "mac", "whats-new-"+want+".txt"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue // presence is TestWhatsNewIsNamedForThisRelease's job
		}
		for _, r := range string(b) {
			if smallCapitalToLetter[r] != 0 {
				t.Errorf("%s contains %q (U+%04X), a drawn small capital App Store Connect refuses in "+
					"What's New; describe the divine name instead of showing it", p, r, r)
			}
		}
	}
}
