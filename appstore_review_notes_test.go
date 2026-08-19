package bibletext

// THE APP REVIEW NOTES ARE PART OF THE RELEASE, so they are guarded like one.
//
// WHAT WENT WRONG, 19 Aug 2026. Version 1.2.0 went to Apple carrying review
// notes headed "VERSION 1.1.8 — HOTFIX" that described a search-results fix,
// while the release's headline feature — shared notes, the one part of this app
// where content arrives from another person — was never mentioned. Nobody wrote
// those notes for 1.2.0: App Store Connect COPIES the previous version's review
// detail forward when a new version record is created, so the field is never
// empty and never looks wrong. The same text rode 1.1.5, 1.1.6 and 1.1.7
// unchanged too, under the heading "NEW IN 1.1.0 — IPAD".
//
// THE TRAP UNDERNEATH IT. build/appstore/review_notes.txt looks exactly like
// the release's review notes and is not: it is read only by push_betareview.py
// and push_testflight.py, which write betaAppReviewDetail — TESTFLIGHT review,
// a different field from the appStoreReviewDetail App Review reads. Nothing in
// the repo ever wrote the App Store field. And build/ is gitignored, so that
// file was not even version-controlled: no diff, no history, no CI, and absent
// entirely from a fresh clone.
//
// So the notes now live HERE, tracked, and this test is what makes a stale one
// impossible to ship quietly: it fails on the the development environment and in CI the moment
// the marketing version moves past the version the notes describe.
//
// It cannot check that the notes are TRUE — only a person can — but it can
// insist they are about this release, that they say something about the feature
// this release is for, and that no key was pasted into a file bound for Apple.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const reviewNotesPath = "appstore/review-notes.txt"

// marketingVersion reads the version the app actually ships as. FyneApp.toml is
// the packaging source of truth (docs/APP_STORE_SUBMISSION.md), so the notes are
// held to it rather than to a number repeated somewhere else.
func marketingVersion(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("cmd/mobile/FyneApp.toml")
	if err != nil {
		t.Fatalf("cannot read cmd/mobile/FyneApp.toml: %v", err)
	}
	m := regexp.MustCompile(`(?m)^\s*Version\s*=\s*"([0-9]+(?:\.[0-9]+)*)"`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("no Version in cmd/mobile/FyneApp.toml — the packaging source of truth moved")
	}
	return m[1]
}

// TestAppReviewNotesAreForThisRelease is the guard the 1.2.0 submission needed.
func TestAppReviewNotesAreForThisRelease(t *testing.T) {
	raw, err := os.ReadFile(reviewNotesPath)
	if err != nil {
		t.Fatalf("%s is missing. The App Review notes are a shipped artifact — "+
			"if they moved, move this test with them; do not delete the guard: "+
			"App Store Connect silently carries the PREVIOUS version's notes "+
			"forward, so an absent file reads as a correct one. (%v)", reviewNotesPath, err)
	}
	notes := string(raw)
	want := marketingVersion(t)

	// 1. THE NOTES NAME THIS RELEASE. The failure this whole file exists for.
	first := strings.TrimSpace(strings.SplitN(notes, "\n", 2)[0])
	if !strings.Contains(first, want) {
		t.Errorf("%s opens with %q but the app ships as %s.\n\n"+
			"This is EXACTLY the 1.2.0 defect: App Store Connect copies the previous\n"+
			"version's review notes onto a new version record, so App Review reads a\n"+
			"description of the LAST release while testing this one. Rewrite the notes\n"+
			"for %s before submitting.", reviewNotesPath, first, want, want)
	}
	// And no OTHER version may be announced in the heading position.
	for _, stale := range regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`).FindAllString(first, -1) {
		if stale != want {
			t.Errorf("%s's first line also names version %s; the notes must describe %s alone",
				reviewNotesPath, stale, want)
		}
	}

	// 2. THEY SAY HOW TO EXERCISE THE APP. Notes with no review path are the
	//    kind that get a release rejected for "we could not locate the feature".
	if !regexp.MustCompile(`(?i)\b(to exercise|review path|how to test|to receive|to send)\b`).MatchString(notes) {
		t.Error("the review notes give App Review no way to exercise the app — " +
			"no review path, no steps, nothing to tap")
	}

	// 3. NO CREDENTIAL EVER GOES IN THIS FILE. It is tracked now, and it is
	//    pasted verbatim into App Store Connect; a review-only provider key
	//    belongs in the ASC form at submission time and nowhere else
	//    (docs/APP_STORE_SUBMISSION.md says so, and this enforces it).
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
				"the ASC form at submission time and is never committed", reviewNotesPath, pat.name)
		}
	}
}

// TestAppReviewNotesCoverTheHeadlineFeature is the softer half: 1.2.0's notes
// said nothing about shared notes, the one feature in this app where content
// arrives from ANOTHER PERSON — precisely what App Review most needs explained,
// and what the repo's own carefully-written provenance paragraph existed to
// explain. While the notes feature ships, the notes must account for it.
func TestAppReviewNotesCoverTheHeadlineFeature(t *testing.T) {
	raw, err := os.ReadFile(reviewNotesPath)
	if err != nil {
		t.Skipf("%s missing; the release guard above already reports that", reviewNotesPath)
	}
	notes := strings.ToLower(string(raw))

	// The provenance claims Apple cares about for user-to-user content. Each is
	// a property the app really has (share_link.go, notes_store.go, the note
	// surfaces render TEXT and always attribute to a person) — so if one stops
	// being true, the fix is the app, not this list.
	for _, must := range []struct{ what, needle string }{
		{"that the feature exists at all", "shared notes"},
		{"that there is no server behind it", "no server"},
		{"that a message is rendered as plain text, never markup", "plain text"},
		{"how a recipient turns it off or deletes", "delete"},
	} {
		if !strings.Contains(notes, must.needle) {
			t.Errorf("the review notes do not say %s (looked for %q).\n"+
				"Shared notes are the one place this app shows a user content written "+
				"by someone else; leaving that unexplained is how a release gets held.",
				must.what, must.needle)
		}
	}
}

// TestDesktopAndMobileShipTheSameVersion — the desktop bundles carry their own
// FyneApp.toml, and nothing had ever compared the two.
//
// WHAT THIS CATCHES, and it was live when the test was written: mobile said
// 1.2.0 while cmd/desktop/FyneApp.toml still said 1.1.8. .github/workflows/
// release.yml titles the GitHub Release from the TAG ("BibleText 1.2.0") but
// passes no -appVersion to `fyne package`, so the macOS, Windows and Linux
// bundles take their version from that file. Tagging v1.2.0 would therefore
// have published a release whose desktop downloads report 1.1.8 in About /
// Get Info — with nobody forgetting a step, because no step exists.
//
// It is a real convention, not two independent numbers: v1.1.5, v1.1.6 and
// v1.1.7 all carried identical versions in both files. HEAD was the first
// release where the bump happened on one side only.
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
			"Bump both, or say here why they may differ.", desktop, mobile, desktop, mobile)
	}
}

// TestWhatsNewIsNamedForThisRelease guards the customer-facing half of the
// carry-forward problem.
//
// push_metadata.py resolved the target VERSION correctly from FyneApp.toml and
// then read the text from a constant filename, whats_new.txt. That file today
// holds neither 1.1.8's notes nor 1.2.0's — it is an older shared-notes draft —
// so running the documented command would have replaced the correct live 1.2.0
// What's New with it.
//
// appstore/preflight.py cannot catch this: it compares the LIVE value against
// the PREVIOUS version's, so it detects inheritance, not staleness, and a
// wrong-but-different file sails through. Naming the file after the version is
// what closes it, and this asserts the file that name refers to actually exists
// and is distinct from other releases' notes.
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
			"push_metadata.py reads the file named for the version precisely so there "+
			"is no \"current\" file to go stale. (%v)", mine, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		t.Fatalf("%s is empty", mine)
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
