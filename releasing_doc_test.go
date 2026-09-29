package bibletext

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docs/RELEASING.md makes claims about two workflow files that only those
// files can vouch for, and each changes on its own schedule.
//
// Stage 3 says the push of main builds the Windows Store package because the
// release bump touches one of msstore.yml's path filters, and that a later
// fix is rebuilt by its own push only when it touches one too. The filter
// names a small minority of the root package, so a release-time fix in reader
// code starts CI and no Store run at all; a stage that promised otherwise
// would send stage 9 to a package labelled for a tree the tag does not name.
//
// Stage 2 says CI's "Repository hygiene" step is a list of scripts, not one,
// so that a local run of the last of them is not mistaken for the step. The
// list is held to the step by name, in both directions, rather than counted:
// a count drifts silently whenever a suite joins.

// releasingStage returns one "### n — ..." section of docs/RELEASING.md, up
// to the next heading of any level.
func releasingStage(t *testing.T, number string) string {
	t.Helper()
	doc := readRepoFile(t, "docs/RELEASING.md")
	start := strings.Index(doc, "\n### "+number+" — ")
	if start < 0 {
		t.Fatalf("docs/RELEASING.md has no stage %s heading", number)
	}
	rest := doc[start+1:]
	// Search for the next heading from the end of this one's line, not from
	// its second character: the heading itself matches the pattern.
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return rest
	}
	if next := regexp.MustCompile(`(?m)^##+ `).FindStringIndex(rest[nl:]); next != nil {
		rest = rest[:nl+next[0]]
	}
	return rest
}

// msstorePushPaths reads the on.push.paths list out of msstore.yml by its
// shape, the way the other workflow tests read theirs: a YAML parser would
// be a new direct dependency for one list of quoted strings.
func msstorePushPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	in, done := false, false
	for _, line := range strings.Split(readRepoFile(t, ".github/workflows/msstore.yml"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case done:
		case !in:
			in = trimmed == "paths:"
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case strings.HasPrefix(trimmed, "- "):
			paths = append(paths, strings.Trim(strings.TrimSpace(trimmed[2:]), `"'`))
		default:
			done = true
		}
	}
	if len(paths) == 0 {
		t.Fatal("msstore.yml has no on.push.paths list, or it is no longer written one quoted item per line")
	}
	return paths
}

// pathFilterGlob turns one workflow filter pattern into a regexp with the
// filter's own semantics: "*" and "?" stop at a slash, "**" does not.
func pathFilterGlob(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// pathFilterMatches applies a workflow's paths filter the way GitHub does: a
// path is in when the last pattern that matches it is not a "!" pattern, so
// an exclusion after an inclusion takes a subtree back out.
func pathFilterMatches(patterns []string, path string) bool {
	in := false
	for _, p := range patterns {
		negate := strings.HasPrefix(p, "!")
		if pathFilterGlob(strings.TrimPrefix(p, "!")).MatchString(path) {
			in = !negate
		}
	}
	return in
}

// ciHygieneStep returns the lines of the "Repository hygiene" step's run
// block in ci.yml: everything under "run: |" indented deeper than it.
func ciHygieneStep(t *testing.T) []string {
	t.Helper()
	lines := strings.Split(readRepoFile(t, ".github/workflows/ci.yml"), "\n")
	indentOf := func(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }
	step := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "- name: Repository hygiene" {
			step = i
			break
		}
	}
	if step < 0 {
		t.Fatal("ci.yml has no step named Repository hygiene")
	}
	run := -1
	for i := step + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "run: |" {
			run = i
			break
		}
		if strings.HasPrefix(trimmed, "- name:") {
			break
		}
	}
	if run < 0 {
		t.Fatal("the Repository hygiene step has no `run: |` block")
	}
	var body []string
	for i := run + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentOf(lines[i]) <= indentOf(lines[run]) {
			break
		}
		body = append(body, strings.TrimSpace(lines[i]))
	}
	if len(body) == 0 {
		t.Fatal("the Repository hygiene step's run block is empty")
	}
	return body
}

var (
	hygieneScript = regexp.MustCompile(`(?:^|\s)scripts/([a-z0-9_-]+\.py)`)
	hygieneSuite  = regexp.MustCompile(`unittest discover -s (\S+)`)
	stageScript   = regexp.MustCompile("`(check-[a-z0-9-]+\\.py)`")
)

// The desktop ledger is what the MSIX version is stamped from, and it is the
// one file every release bump changes; stage 3 rests on the push of that bump
// starting the workflow, which is only so while the ledger is inside the
// filter.
func TestTheReleaseBumpIsAnInputOfTheStoreWorkflow(t *testing.T) {
	const ledger = "cmd/bibletext/FyneApp.toml"
	if !pathFilterMatches(msstorePushPaths(t), ledger) {
		t.Errorf("msstore.yml's push filter no longer matches %s, so a release bump does not build "+
			"the Store package; stage 3 of docs/RELEASING.md is built on it doing so", ledger)
	}
	if !strings.Contains(releasingStage(t, "3"), "`"+ledger+"`") {
		t.Errorf("stage 3 of docs/RELEASING.md must name `%s` as the input the release bump touches", ledger)
	}
}

// Most of the root package is outside the filter, so a fix in reader code
// pushed after the bump starts CI and nothing else. Stage 3 may not promise
// that a fix's push rebuilds the package; it must tie the rebuild to the
// workflow's inputs and send the reader back to the headSha check.
func TestAFixInReaderCodeDoesNotRebuildTheStorePackage(t *testing.T) {
	paths := msstorePushPaths(t)
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "*.go"))
	if err != nil {
		t.Fatalf("glob root: %v", err)
	}
	var total, matched int
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		total++
		if pathFilterMatches(paths, name) {
			matched++
		}
	}
	if total < 100 {
		t.Fatalf("only %d root package files found; the glob has drifted and this test would "+
			"pass by checking almost nothing", total)
	}
	if matched*2 >= total {
		t.Fatalf("msstore.yml's filter now matches %d of %d root package files; stage 3 of "+
			"docs/RELEASING.md says most reader code is outside it, and must be rewritten first",
			matched, total)
	}
	for _, name := range []string{"reading.go", "ui.go", "state.go"} {
		if _, err := os.Stat(filepath.Join(repoRoot(t), name)); err != nil {
			t.Errorf("%s is gone; pick another reader file for this check", name)
		} else if pathFilterMatches(paths, name) {
			t.Errorf("%s is inside msstore.yml's filter; if that is deliberate, stage 3 of "+
				"docs/RELEASING.md needs a different example of a fix that starts no run", name)
		}
	}

	var about []string
	for _, para := range strings.Split(releasingStage(t, "3"), "\n\n") {
		if strings.Contains(para, "later fix") {
			about = append(about, strings.Join(strings.Fields(para), " "))
		}
	}
	if len(about) == 0 {
		t.Fatal("stage 3 of docs/RELEASING.md no longer says what to do when a later fix moves the tag")
	}
	for _, para := range about {
		if !strings.Contains(para, "inputs") {
			t.Errorf("stage 3 lets a later fix's push rebuild the package without tying that to the "+
				"workflow's inputs; the filter matches %d of %d root package files, so most such "+
				"pushes start no run:\n  %s", matched, total, para)
		}
		if strings.Contains(para, "Either way") {
			t.Errorf("stage 3 promises the package exists either way; it exists only after the "+
				"headSha check on the fix's push, or the dispatch that stands in for it:\n  %s", para)
		}
	}
}

// Stage 2 names what the hygiene step runs so that no single script can be
// mistaken for it. The names are held to ci.yml in both directions.
func TestStage2NamesEveryCheckInTheHygieneStep(t *testing.T) {
	var scripts, suites []string
	for _, line := range ciHygieneStep(t) {
		if m := hygieneScript.FindStringSubmatch(line); m != nil {
			scripts = append(scripts, m[1])
		}
		if m := hygieneSuite.FindStringSubmatch(line); m != nil {
			suites = append(suites, m[1])
		}
	}
	if len(scripts) < 5 || len(suites) < 2 {
		t.Fatalf("the hygiene step parser found %d scripts and %d unit-test suites; the step's shape "+
			"has drifted and the checks below would compare nothing", len(scripts), len(suites))
	}
	stage := releasingStage(t, "2")
	runs := map[string]bool{}
	for _, s := range scripts {
		runs[s] = true
		if !strings.Contains(stage, "`"+s+"`") {
			t.Errorf("ci.yml's Repository hygiene step runs %s and stage 2 of docs/RELEASING.md does "+
				"not name it; a reader who runs only what the stage lists has not run the step", s)
		}
	}
	for _, d := range suites {
		if !strings.Contains(stage, "`"+d+"`") {
			t.Errorf("ci.yml's Repository hygiene step runs the unit tests under %s and stage 2 of "+
				"docs/RELEASING.md does not name that directory", d)
		}
	}
	for _, m := range stageScript.FindAllStringSubmatch(stage, -1) {
		if !runs[m[1]] {
			t.Errorf("stage 2 of docs/RELEASING.md names %s, which the Repository hygiene step does "+
				"not run; the list is stale", m[1])
		}
	}
}

// CONTROL: the filter matcher must have GitHub's semantics and the parsers
// must see the real files, or the tests above pass by checking nothing.
func TestTheWorkflowParsersSeeTheRealFiles(t *testing.T) {
	sample := []string{"msstore/**", "share_link_*.go", "!msstore/metadata/**", "cmd/bibletext/**"}
	for _, c := range []struct {
		path string
		want bool
	}{
		{"msstore/pack.py", true},
		{"msstore/metadata/en-gb/whats-new.txt", false},
		{"share_link_argv.go", true},
		{"share_link/argv.go", false},
		{"cmd/bibletext/FyneApp.toml", true},
		{"reading.go", false},
	} {
		if got := pathFilterMatches(sample, c.path); got != c.want {
			t.Errorf("pathFilterMatches(%q) = %v, want %v", c.path, got, c.want)
		}
	}
	paths := msstorePushPaths(t)
	if len(paths) < 10 {
		t.Fatalf("the msstore.yml parser found only %d patterns", len(paths))
	}
	var negated bool
	for _, p := range paths {
		negated = negated || strings.HasPrefix(p, "!")
	}
	if !negated {
		t.Error("the msstore.yml parser lost the filter's negation, so exclusions are not applied")
	}
	if n := len(ciHygieneStep(t)); n < 5 {
		t.Fatalf("the ci.yml parser found only %d lines in the hygiene step", n)
	}
	if s := releasingStage(t, "3"); !strings.HasPrefix(s, "### 3 — ") || strings.Contains(s, "\n### 4 — ") {
		t.Errorf("releasingStage returned the wrong slice:\n%.80s...", s)
	}
}
