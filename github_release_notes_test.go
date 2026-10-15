package bibletext

// The GitHub release's notes are the argument of the printf that fills NOTES
// in .github/workflows/release.yml, which `gh release create` sends when a
// tag is pushed. Unlike the stores' What's New files they are not named for a
// version, so nothing tied them to the release being made, and v1.2.21's
// release carried 1.2.20's notes word for word, opening "since 1.2.19".
// Stage 1 of docs/RELEASING.md lists them among the surfaces the version bump
// changes, and this holds them to the release at that bump:
//
//   - the head, everything above the store and download footer that every
//     release carries, opens with a line that names the release before this
//     one, the second [[release]] in linux/releases.toml, and no release but
//     that one and this; and
//   - below that line the head is not the previous release's, as the
//     previous release's tag holds it.
//
// The first rule runs wherever the tree does. The second needs the previous
// tag in the clone: CI's Linux job fetches the whole history, and a clone
// without the tag logs that the comparison did not run.

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

const releaseWorkflowPath = ".github/workflows/release.yml"

// githubNotesFooter opens the footer every release's notes carry: the store
// links and the files for readers without a store, which
// scripts/check-public-surfaces.py holds. The head is what stands above it.
const githubNotesFooter = "\n\nGet BibleText from a store."

// The same pattern scripts/check-public-surfaces.py reads the notes by: the
// printf's single-quoted argument, which holds no single quote.
var githubNotesPrintf = regexp.MustCompile(`NOTES="\$\(printf '([^']*)'`)

var dottedVersion = regexp.MustCompile(`\b[0-9]+\.[0-9]+\.[0-9]+\b`)

// printfText is what printf prints for a format with no arguments, for the
// escapes the notes may use: \n, \\ and %%. Any other escape or conversion
// is an error, since the text printf prints would not be the text read here.
func printfText(format string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '\\' && c != '%' {
			b.WriteByte(c)
			continue
		}
		if i+1 < len(format) {
			switch next := format[i+1]; {
			case c == '\\' && next == 'n':
				b.WriteByte('\n')
				i++
				continue
			case c == next:
				b.WriteByte(c)
				i++
				continue
			}
		}
		end := i + 2
		if end > len(format) {
			end = len(format)
		}
		return "", fmt.Errorf("%q at byte %d, which printf would not print as written", format[i:end], i)
	}
	return b.String(), nil
}

// githubNotesHead returns the head of the notes in a release workflow: the
// printf's text above the footer. The second result is a problem when there
// is no head to read.
func githubNotesHead(workflow string) (string, string) {
	m := githubNotesPrintf.FindStringSubmatch(workflow)
	if m == nil {
		return "", fmt.Sprintf("%s has no NOTES=\"$(printf '...')\" line, so the release notes cannot be read",
			releaseWorkflowPath)
	}
	notes, err := printfText(m[1])
	if err != nil {
		return "", fmt.Sprintf("the release notes in %s hold %v", releaseWorkflowPath, err)
	}
	head, _, ok := strings.Cut(notes, githubNotesFooter)
	if !ok {
		return "", fmt.Sprintf("the release notes in %s no longer hold %q, where the footer every release "+
			"carries begins; this test reads the head as what stands above it", releaseWorkflowPath,
			strings.TrimSpace(githubNotesFooter))
	}
	return head, ""
}

// releaseHistoryVersions lists the version of each [[release]] in
// linux/releases.toml, newest first, as the file keeps them.
func releaseHistoryVersions(toml string) []string {
	var out []string
	version := regexp.MustCompile(`(?m)^version = "([^"]+)"`)
	for _, block := range strings.Split("\n"+toml, "\n[[release]]")[1:] {
		if m := version.FindStringSubmatch(block); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// githubNotesProblems makes the checks on the notes in a release workflow.
// `releases` is linux/releases.toml, `ledger` the version being released and
// `previous` the release workflow at the previous release's tag, or "" when
// the tag is not to hand.
func githubNotesProblems(workflow, releases, ledger, previous string) []string {
	versions := releaseHistoryVersions(releases)
	if len(versions) < 2 {
		return []string{fmt.Sprintf("linux/releases.toml has %d [[release]] entries; the release before %s "+
			"is read from the second", len(versions), ledger)}
	}
	if versions[0] != ledger {
		return []string{fmt.Sprintf("linux/releases.toml's newest [[release]] is %s and the ledger's Version "+
			"is %s, so the release before this one cannot be read from it", versions[0], ledger)}
	}
	prior := versions[1]
	head, problem := githubNotesHead(workflow)
	if problem != "" {
		return []string{problem}
	}
	lead, body, _ := strings.Cut(head, "\n")
	body = strings.TrimSpace(body)
	var out []string
	named := dottedVersion.FindAllString(lead, -1)
	namesPrior, namesOther := false, false
	for _, v := range named {
		switch v {
		case prior:
			namesPrior = true
		case ledger:
		default:
			namesOther = true
		}
	}
	if !namesPrior || namesOther {
		out = append(out, fmt.Sprintf("the GitHub release notes for %s open %q, which names %v; the lead "+
			"line names %s, the release before this one, and no release but that one and %s, or the notes "+
			"describe another release (%s, NOTES)", ledger, lead, named, prior, ledger, releaseWorkflowPath))
	}
	if body == "" {
		out = append(out, fmt.Sprintf("the GitHub release notes for %s have nothing below their lead line "+
			"and above the footer", ledger))
	}
	if previous == "" {
		return out
	}
	previousHead, problem := githubNotesHead(previous)
	if problem != "" {
		return append(out, "at v"+prior+": "+problem)
	}
	_, previousBody, _ := strings.Cut(previousHead, "\n")
	if body != "" && body == strings.TrimSpace(previousBody) {
		out = append(out, fmt.Sprintf("below their lead line the GitHub release notes for %s are v%s's word "+
			"for word; they describe the release before this one", ledger, prior))
	}
	return out
}

// previousReleaseWorkflow is the release workflow as the previous release's
// tag holds it, or an error when the clone does not have the tag.
func previousReleaseWorkflow(t *testing.T, version string) (string, error) {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot(t), "show", "v"+version+":"+releaseWorkflowPath).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func TestTheGitHubReleaseNotesAreForThisRelease(t *testing.T) {
	ledger := packagedVersion(t, "cmd/bibletext/FyneApp.toml")
	workflow := readRepoFile(t, releaseWorkflowPath)
	releases := readRepoFile(t, "linux/releases.toml")

	// The parsers must see the real files, or the checks pass on nothing.
	versions := releaseHistoryVersions(releases)
	if len(versions) < 10 {
		t.Fatalf("read %d [[release]] versions from linux/releases.toml; its shape has drifted", len(versions))
	}
	head, problem := githubNotesHead(workflow)
	if problem != "" {
		t.Fatal(problem)
	}
	if !strings.Contains(head, "\n- ") {
		t.Fatalf("the head of the GitHub release notes holds no bullet; the parser has drifted:\n%s", head)
	}

	var previous string
	if versions[0] == ledger && len(versions) > 1 {
		var err error
		if previous, err = previousReleaseWorkflow(t, versions[1]); err != nil {
			t.Logf("v%s is not in this clone (%v); the comparison with its notes did not run", versions[1], err)
		}
	}
	for _, p := range githubNotesProblems(workflow, releases, ledger, previous) {
		t.Error(p)
	}
}

// Stage 1 of docs/RELEASING.md lists the surfaces the version bump changes;
// the GitHub release notes are among them, or a release prepared from the
// list leaves the last release's notes in place.
func TestStage1ListsTheGitHubReleaseNotes(t *testing.T) {
	stage := strings.Join(strings.Fields(releasingStage(t, "1")), " ")
	for _, want := range []string{"`" + releaseWorkflowPath + "`", "`NOTES`", "github_release_notes_test.go"} {
		if !strings.Contains(stage, want) {
			t.Errorf("stage 1 of docs/RELEASING.md does not name %s; the GitHub release notes are one of "+
				"the surfaces the version bump changes", want)
		}
	}
}

// Each check above is proved against a workflow built to trip it, beside the
// controls that must pass.
func TestGitHubReleaseNotesChecksFire(t *testing.T) {
	const ledger = "9.9.2"
	history := "# header\n\n[[release]]\nversion = \"9.9.2\"\n\n[[release]]\nversion = \"9.9.1\"\n\n" +
		"[[release]]\nversion = \"9.9.0\"\n"
	workflow := func(head string) string {
		return "        run: |\n          NOTES=\"$(printf '" + strings.ReplaceAll(head, "\n", `\n`) +
			`\n\nGet BibleText from a store. Store editions update themselves.\n\n- Linux: [Snap Store](x)')"` + "\n"
	}
	fit := "Fixes since 9.9.1.\n\n- One.\n- Two."
	old := workflow("Fixes since 9.9.0.\n\n- Three.")
	for _, c := range []struct {
		name               string
		workflow, previous string
		history            string
		want               string // "" = no problem
	}{
		{"fit notes", workflow(fit), old, history, ""},
		{"fit notes, the previous tag not to hand", workflow(fit), "", history, ""},
		{"the lead names this release too", workflow("9.9.2: fixes since 9.9.1.\n\n- One."), old, history, ""},
		{"an escaped backslash and percent sign", workflow(`Fixes since 9.9.1.` + "\n\n- 100%% and a \\\\."), old, history, ""},
		{"the previous release's bullets under a new lead line", workflow("Changes since 9.9.1.\n\n- Three."), "", history, ""},
		{"the lead names an older release", workflow("Fixes since 9.9.0.\n\n- One."), old, history, "names [9.9.0]"},
		{"the lead names no release", workflow("Fixes.\n\n- One."), old, history, "names []"},
		{"the lead names the previous release and an older one", workflow("Since 9.9.0 and 9.9.1.\n\n- One."), old, history, "names [9.9.0 9.9.1]"},
		{"a bullet names the previous release, the lead does not", workflow("Fixes.\n\n- Since 9.9.1."), old, history, "names []"},
		{"nothing below the lead line", workflow("Fixes since 9.9.1."), old, history, "nothing below their lead line"},
		{"the previous release's bullets under a new lead line, the tag to hand", workflow("Changes since 9.9.1.\n\n- Three."), old, history, "word for word"},
		{"the previous release's notes whole", old, old, history, "names [9.9.0]"},
		{"the newest entry is not the ledger's", workflow(fit), old, strings.Replace(history, `"9.9.2"`, `"9.9.3"`, 1), "newest [[release]] is 9.9.3"},
		{"one entry only", workflow(fit), old, "[[release]]\nversion = \"9.9.2\"\n", "has 1 [[release]]"},
		{"no printf", "run: echo\n", old, history, "has no NOTES="},
		{"no footer", strings.Replace(workflow(fit), "Get BibleText from a store.", "Stores.", 1), old, history, "no longer hold"},
		{"a conversion", workflow("Fixes since 9.9.1.\n\n- 100% of them."), old, history, "printf would not print"},
		{"an escape other than a line break", workflow("Fixes since 9.9.1.\n\n- One\\tTwo."), old, history, "printf would not print"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := githubNotesProblems(c.workflow, c.history, ledger, c.previous)
			if c.want == "" {
				if len(got) != 0 {
					t.Errorf("fit notes were refused: %q", got)
				}
				return
			}
			if len(got) == 0 || !strings.Contains(strings.Join(got, "\n"), c.want) {
				t.Errorf("want a problem containing %q, got %q", c.want, got)
			}
		})
	}
}
