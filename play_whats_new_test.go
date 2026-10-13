package bibletext

// Google Play's release notes, the What's new a track release carries
// (releaseNotes in the Developer API, at most 500 characters per language
// with line breaks counted), are written for each release in a tracked
// file, which scripts/play-publish.py upload reads unless --notes names
// another and sends as written, the whitespace at its two ends and each
// line's trailing spaces dropped:
//
//	play/metadata/en-GB/whats-new-<version>.txt
//
// The notes are drafted, and read for the OK that sends them, as the
// blockquote under "Release notes — <version>" in docs/PLAY_LISTING.md, so
// the file is held to that blockquote as well as to the cap, at the version
// bump: a release whose file is missing, or says something the page does
// not, fails here before anything is uploaded, as msstore_whats_new_test.go
// holds the Microsoft Store's file and TestWhatsNewIsNamedForThisRelease the
// App Store's.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

const playWhatsNewDir = "play/metadata/en-GB"
const playListingPath = "docs/PLAY_LISTING.md"

// playNotesCap reads the cap from scripts/play-publish.py, which enforces it
// at upload, so the two cannot come to disagree.
func playNotesCap(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile("scripts/play-publish.py")
	if err != nil {
		t.Fatalf("cannot read scripts/play-publish.py: %v", err)
	}
	m := regexp.MustCompile(`(?m)^NOTES_CAP = (\d+)$`).FindStringSubmatch(string(b))
	if m == nil {
		t.Fatal("scripts/play-publish.py no longer declares NOTES_CAP; this test reads the cap from it")
	}
	limit, _ := strconv.Atoi(m[1])
	return limit
}

// playNotesAsSent is read_notes in scripts/play-publish.py: CRLF read as LF,
// the whitespace at both ends of the whole dropped, and each line's trailing
// whitespace; the line breaks between lines and the indent of every line
// after the first stay.
func playNotesAsSent(text string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n")), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

// playListingNotes returns the blockquote under "## Release notes —
// <version>" on the listing page, each line's "> " dropped and the lines
// joined by line breaks: the first run of quoted lines in the section, so
// the tester coverage that may follow it is not read. The second result is
// false when the page has no such section.
func playListingNotes(doc, version string) (string, bool) {
	heading := "## Release notes — " + version
	_, rest, ok := strings.Cut(strings.ReplaceAll(doc, "\r\n", "\n"), "\n"+heading+"\n")
	if !ok {
		return "", false
	}
	var quoted []string
	for _, line := range strings.Split(rest, "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			return strings.Join(quoted, "\n"), true
		case strings.HasPrefix(line, ">"):
			quoted = append(quoted, strings.TrimPrefix(strings.TrimPrefix(line, ">"), " "))
		case len(quoted) > 0 && strings.TrimSpace(line) != "":
			return strings.Join(quoted, "\n"), true
		}
	}
	return strings.Join(quoted, "\n"), true
}

// playWhatsNewProblems makes the checks on one release's file: it exists, is
// UTF-8, is not empty, is the text of the page's blockquote for the version
// as both are sent, fits the cap with its line breaks counted, and is not
// another release's text. `doc` is the listing page.
func playWhatsNewProblems(dir, doc, version string, limit int) []string {
	path := filepath.Join(dir, "whats-new-"+version+".txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("%s is missing: Google Play's release notes for %s are written "+
			"for each release, and scripts/play-publish.py upload reads the file and refuses "+
			"without it (%v)", path, version, err)}
	}
	if !utf8.Valid(raw) {
		return []string{fmt.Sprintf("%s is not UTF-8", path)}
	}
	text := playNotesAsSent(string(raw))
	if text == "" {
		return []string{fmt.Sprintf("%s is empty", path)}
	}
	var out []string
	listed, ok := playListingNotes(doc, version)
	switch {
	case !ok:
		out = append(out, fmt.Sprintf("%s has no \"## Release notes — %s\" section, where the notes "+
			"in %s are drafted and read", playListingPath, version, path))
	case playNotesAsSent(listed) == "":
		out = append(out, fmt.Sprintf("the \"Release notes — %s\" section of %s holds no blockquote",
			version, playListingPath))
	case playNotesAsSent(listed) != text:
		out = append(out, fmt.Sprintf("%s is not the blockquote under \"Release notes — %s\" in %s: "+
			"the file is what Play is sent and the page is what was read, so the two must say "+
			"the same", path, version, playListingPath))
	}
	if n := len([]rune(text)); n > limit {
		out = append(out, fmt.Sprintf("%s is %d characters with its line breaks counted; Play caps "+
			"release notes at %d", path, n, limit))
	}
	others, _ := filepath.Glob(filepath.Join(dir, "whats-new-*.txt"))
	for _, o := range others {
		if filepath.Base(o) == filepath.Base(path) {
			continue
		}
		ob, err := os.ReadFile(o)
		if err == nil && playNotesAsSent(string(ob)) == text {
			out = append(out, fmt.Sprintf("%s is the same text as %s; the notes for %s would describe "+
				"a different release", path, o, version))
		}
	}
	return out
}

func TestPlayWhatsNewIsNamedForThisRelease(t *testing.T) {
	want := packagedVersion(t, "cmd/mobile/FyneApp.toml")
	limit := playNotesCap(t)
	doc := readRepoFile(t, playListingPath)
	for _, p := range playWhatsNewProblems(playWhatsNewDir, doc, want, limit) {
		t.Error(p)
	}
}

// Each check above is proved against a file and a page built to trip it,
// beside the controls that must pass.
func TestPlayWhatsNewChecksFire(t *testing.T) {
	const version = "9.9.9"
	const limit = 500
	section := func(v, body string) string { return "## Release notes — " + v + "\n\n" + body + "\n" }
	page := func(sections ...string) string { return "# Listing\n\n" + strings.Join(sections, "\n") }
	a249, b249 := strings.Repeat("a", 249), strings.Repeat("b", 249)
	for _, c := range []struct {
		name  string
		files map[string]string
		doc   string
		want  string // "" = no problem
	}{
		{"a fit note of several lines", map[string]string{version: "One.\n• Two — “three”.\n"},
			page(section(version, "> One.\n> • Two — “three”.")), ""},
		{"a fit note of one line", map[string]string{version: "One line… and more.\n"},
			page(section(version, "> One line… and more.")), ""},
		{"blank ends, trailing spaces and CRLF are not a difference",
			map[string]string{version: "\r\n  One.   \r\n• Two\t\r\n\r\n"},
			page(section(version, "> One.\n> • Two")), ""},
		{"the indent of a line after the first is kept on both sides",
			map[string]string{version: "One.\n  • Two\n"}, page(section(version, "> One.\n>   • Two")), ""},
		{"what follows the blockquote is not read", map[string]string{version: "One.\n"},
			page(section(version, "> One.\n\nSuggested tester coverage:\n\n- install it;\n- open it.")), ""},
		{"a later release's section comes first", map[string]string{version: "One."},
			page(section("9.9.10", "> Later."), section(version, "> One.")), ""},
		{"exactly at the cap", map[string]string{version: a249 + "\n\n" + b249},
			page(section(version, "> "+a249+"\n>\n> "+b249)), ""},
		{"another release's different note", map[string]string{version: "New.", "1.0.0": "Old."},
			page(section(version, "> New.")), ""},
		{"missing", map[string]string{"1.0.0": "Old."}, page(section(version, "> One.")), "is missing"},
		{"empty", map[string]string{version: " \n\n"}, page(section(version, "> One.")), "is empty"},
		{"not UTF-8", map[string]string{version: "One \xff"}, page(section(version, "> One.")), "not UTF-8"},
		{"no section on the page", map[string]string{version: "One."},
			page(section("9.9.8", "> Old.")), "has no \"## Release notes — 9.9.9\" section"},
		{"a section without a blockquote", map[string]string{version: "One."},
			page(section(version, "Suggested tester coverage:\n\n- install it.")), "holds no blockquote"},
		{"not the page's text", map[string]string{version: "One.\n• Two."},
			page(section(version, "> One.\n> • Three.")), "is not the blockquote"},
		{"the page's text for another release", map[string]string{version: "One."},
			page(section("9.9.8", "> One."), section(version, "> Two.")), "is not the blockquote"},
		{"a line the page does not have", map[string]string{version: "One.\n• Two."},
			page(section(version, "> One.")), "is not the blockquote"},
		{"over the cap", map[string]string{version: strings.Repeat("a", 501)},
			page(section(version, "> "+strings.Repeat("a", 501))), "caps release notes at 500"},
		{"over the cap by its line breaks", map[string]string{version: a249 + "a\n\n" + b249},
			page(section(version, "> "+a249+"a\n>\n> "+b249)), "caps release notes at 500"},
		{"another release's note", map[string]string{version: "Same.", "1.0.0": "Same.\n"},
			page(section(version, "> Same.")), "the same text as"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for v, text := range c.files {
				if err := os.WriteFile(filepath.Join(dir, "whats-new-"+v+".txt"), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := playWhatsNewProblems(dir, c.doc, version, limit)
			if c.want == "" {
				if len(got) != 0 {
					t.Errorf("a fit note was refused: %q", got)
				}
				return
			}
			if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), c.want) {
				t.Errorf("want a problem containing %q, got %q", c.want, got)
			}
		})
	}
}

// The upload reads the tracked file this test holds, so the two name the
// same directory; a move of either without the other would leave the
// upload reading a file nothing holds.
func TestTheUploadReadsTheTrackedPlayNotes(t *testing.T) {
	src := readRepoFile(t, "scripts/play-publish.py")
	for name, needle := range map[string]string{
		"the tracked directory":              `NOTES_DIR = os.path.join(REPO, "play", "metadata", "en-GB")`,
		"the file named for the version":     `f"whats-new-{version}.txt"`,
		"the default when --notes is absent": "notes_path = default_notes(version)",
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("scripts/play-publish.py no longer has %s (%q)", name, needle)
		}
	}
	if filepath.ToSlash(playWhatsNewDir) != "play/metadata/en-GB" {
		t.Errorf("playWhatsNewDir is %q; the script reads play/metadata/en-GB", playWhatsNewDir)
	}
}
