package bibletext

// The Microsoft Store listing's What's New ("What's new in this version" in
// Partner Center, releaseNotes in the submission API) is written for each
// release in a tracked file per listing language, which msstore/submit.py
// sends as written:
//
//	msstore/metadata/<language>/whats-new-<version>.txt
//
// submit.py refuses to create a submission without a fit one. This holds a
// release to it earlier, at the version bump, as TestWhatsNewIsNamedForThisRelease
// holds the App Store's files; because this one is tracked, it holds on every
// machine and in CI, where the Apple test skips for want of build/.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const windowsWhatsNewDir = "msstore/metadata"

// The releases the Store has carried, 1.2.10, 1.2.13 and 1.2.14, all went out
// with the field blank: the first through the console, the others because
// submit.py sent the cloned listing unchanged. The ledger stood at 1.2.15,
// with no file written for it, when the file became required, so the next
// version is the first this test holds to it. submit.py itself has no such
// floor: a Store submission of any version reads that version's file or
// refuses to start.
const firstWindowsWhatsNew = "1.2.16"

// windowsWhatsNewRules reads the limit and the listing languages from
// msstore/submit.py, which enforces them at submission, so the two cannot
// come to disagree about either.
func windowsWhatsNewRules(t *testing.T) (int, []string) {
	t.Helper()
	b, err := os.ReadFile("msstore/submit.py")
	if err != nil {
		t.Fatalf("cannot read msstore/submit.py: %v", err)
	}
	src := string(b)
	m := regexp.MustCompile(`(?m)^RELEASE_NOTES_LIMIT = (\d+)$`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("msstore/submit.py no longer declares RELEASE_NOTES_LIMIT; this test reads the limit from it")
	}
	limit, _ := strconv.Atoi(m[1])
	l := regexp.MustCompile(`(?m)^EXPECTED_LISTINGS = \{([^}]*)\}$`).FindStringSubmatch(src)
	if l == nil {
		t.Fatal("msstore/submit.py no longer declares EXPECTED_LISTINGS; this test reads the languages from it")
	}
	var langs []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(l[1], -1) {
		langs = append(langs, q[1])
	}
	if len(langs) == 0 {
		t.Fatal("EXPECTED_LISTINGS names no language")
	}
	return limit, langs
}

// windowsWhatsNewProblems makes read_release_notes' checks in msstore/submit.py
// on one language's directory: the file for the version exists, is UTF-8, is
// not empty, carries only visible characters, spaces and line breaks and none
// of the drawn small capitals, fits the limit in UTF-16 code units, and is not
// another release's text. The two are written apart because one runs in CI
// and the other at submission; the limit and the languages are read from
// submit.py, and both sides hold the small capitals to small_caps_draw.go.
func windowsWhatsNewProblems(dir, version string, limit int) []string {
	path := filepath.Join(dir, "whats-new-"+version+".txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("%s is missing: the Store listing's What's New for %s is written "+
			"for each release, and msstore/submit.py will not create the submission without it (%v)",
			path, version, err)}
	}
	if !utf8.Valid(raw) {
		return []string{fmt.Sprintf("%s is not UTF-8", path)}
	}
	text := strings.TrimRight(string(raw), "\n")
	if strings.TrimSpace(text) == "" {
		return []string{fmt.Sprintf("%s is empty", path)}
	}
	var out []string
	for n, line := range strings.Split(text, "\n") {
		col := 0
		for _, r := range line {
			col++
			switch {
			case smallCapitalToLetter[r] != 0:
				out = append(out, fmt.Sprintf("%s: line %d, column %d is U+%04X, a drawn small capital; "+
					"describe the divine name instead of showing it", path, n+1, col, r))
			case !unicode.IsGraphic(r):
				out = append(out, fmt.Sprintf("%s: line %d, column %d is U+%04X, not a visible character",
					path, n+1, col, r))
			}
		}
	}
	if size := len(utf16.Encode([]rune(text))); size > limit {
		out = append(out, fmt.Sprintf("%s is %d characters; the Store takes at most %d", path, size, limit))
	}
	others, _ := filepath.Glob(filepath.Join(dir, "whats-new-*.txt"))
	for _, o := range others {
		if filepath.Base(o) == filepath.Base(path) {
			continue
		}
		ob, err := os.ReadFile(o)
		if err == nil && strings.TrimSpace(string(ob)) == strings.TrimSpace(text) {
			out = append(out, fmt.Sprintf("%s is the same text as %s; the notes for %s would describe "+
				"a different release", path, o, version))
		}
	}
	return out
}

// versionBefore reports whether dotted version a precedes b.
func versionBefore(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func TestWindowsWhatsNewIsNamedForThisRelease(t *testing.T) {
	want := packagedVersion(t, "cmd/bibletext/FyneApp.toml")
	limit, langs := windowsWhatsNewRules(t)
	if versionBefore(want, firstWindowsWhatsNew) {
		t.Logf("%s predates %s, the first release whose Store What's New is sent; no file is required",
			want, firstWindowsWhatsNew)
		return
	}
	for _, lang := range langs {
		for _, p := range windowsWhatsNewProblems(filepath.Join(windowsWhatsNewDir, lang), want, limit) {
			t.Error(p)
		}
	}
}

// The release test above has nothing to find until the ledger reaches
// firstWindowsWhatsNew, so each of its checks is proved here against a file
// built to trip it, beside a control that must pass.
func TestWindowsWhatsNewChecksFire(t *testing.T) {
	const version = "9.9.9"
	const limit = 1500
	book := "\U0001F4D6" // outside the BMP: two UTF-16 code units
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string // "" = no problem
	}{
		{"a fit note", map[string]string{version: "One.\n• Two — “three”.\n"}, ""},
		{"visible text of every kind", map[string]string{version: "Café, 3 × 4, a\u00a0space, " + book}, ""},
		{"exactly at the limit", map[string]string{version: strings.Repeat("a", 1498) + book}, ""},
		{"another release's different note", map[string]string{version: "New.", "1.0.0": "Old."}, ""},
		{"missing", map[string]string{"1.0.0": "Old."}, "is missing"},
		{"empty", map[string]string{version: " \n\n"}, "is empty"},
		{"not UTF-8", map[string]string{version: "One \xff"}, "not UTF-8"},
		{"over the limit", map[string]string{version: strings.Repeat("a", 1501)}, "at most 1500"},
		{"over the limit in UTF-16", map[string]string{version: strings.Repeat("a", 1499) + book}, "at most 1500"},
		{"a tab", map[string]string{version: "One\tTwo"}, "not a visible character"},
		{"a carriage return", map[string]string{version: "One\r\nTwo"}, "not a visible character"},
		{"a byte-order mark", map[string]string{version: "\ufeffOne"}, "not a visible character"},
		{"a zero-width joiner", map[string]string{version: "One\u200dTwo"}, "not a visible character"},
		{"a private-use character", map[string]string{version: "One \ue000"}, "not a visible character"},
		{"an unassigned code point", map[string]string{version: "One \u0378"}, "not a visible character"},
		{"a drawn small capital", map[string]string{version: "L\u1d0f\u0280\u1d05"}, "drawn small capital"},
		{"another release's note", map[string]string{version: "Same.", "1.0.0": "Same.\n"}, "the same text as"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for v, text := range c.files {
				if err := os.WriteFile(filepath.Join(dir, "whats-new-"+v+".txt"), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := windowsWhatsNewProblems(dir, version, limit)
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

func TestWindowsWhatsNewIsRequiredFromItsFirstRelease(t *testing.T) {
	for _, c := range []struct {
		version string
		exempt  bool
	}{
		{"1.2.15", true},
		{"1.2.9", true},
		{"1.2.16", false},
		{"1.2.100", false},
		{"1.3.0", false},
		{"2.0", false},
	} {
		if got := versionBefore(c.version, firstWindowsWhatsNew); got != c.exempt {
			t.Errorf("versionBefore(%q, %q) = %v, want %v", c.version, firstWindowsWhatsNew, got, c.exempt)
		}
	}
}
