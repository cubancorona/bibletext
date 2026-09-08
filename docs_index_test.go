package bibletext

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// docs/README.md indexes docs/, and this is what stops it becoming the usual
// stale index that is worse than none.
//
// The repository carries ~10,000 lines across three dozen documents, and before
// this index five of them were referenced from nowhere at all — including
// PRIVACY_RELEASE_CHECKLIST.md, whose first instruction is to re-evaluate the
// store declarations whenever the key-storage path changes. That is precisely
// what a session changed while re-deriving several of its points from scratch.
// A document nothing points at is a document nobody reads.
//
// So the index is held to SET EQUALITY with the directory, in both directions:
// a new document with no entry fails, and an entry whose file was renamed or
// deleted fails too. Neither can be fixed by editing this test without also
// telling the truth in the index.

// docsIndexEntry matches a line like:
//
//   - [ANDROID.md](ANDROID.md) — before touching anything under `android/`
//
// The link text and target must be the same filename, so the link resolves
// from inside docs/.
var docsIndexEntry = regexp.MustCompile(`^- \[([A-Za-z0-9_.-]+\.md)\]\(([A-Za-z0-9_.-]+\.md)\)\s+—\s+(\S.*)$`)

// docsIndexFile is the index, and is itself excluded from the set it indexes.
const docsIndexFile = "README.md"

func parseDocsIndex(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("docs", docsIndexFile))
	if err != nil {
		t.Fatalf("read docs/%s: %v — the documentation index is missing", docsIndexFile, err)
	}
	out := map[string]string{}
	for i, line := range strings.Split(string(raw), "\n") {
		m := docsIndexEntry.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		name, target, trigger := m[1], m[2], m[3]
		if name != target {
			t.Errorf("docs/%s:%d links %q to %q; the text and the target must be the same "+
				"file or the link does not resolve from inside docs/", docsIndexFile, i+1, name, target)
		}
		if prev, dup := out[name]; dup {
			t.Errorf("docs/%s: %s is indexed twice (%q and %q)", docsIndexFile, name, prev, trigger)
		}
		out[name] = trigger
	}
	return out
}

func docsOnDisk(t *testing.T) []string {
	t.Helper()
	found, err := filepath.Glob(filepath.Join("docs", "*.md"))
	if err != nil {
		t.Fatalf("glob docs: %v", err)
	}
	var names []string
	for _, p := range found {
		if b := filepath.Base(p); b != docsIndexFile {
			names = append(names, b)
		}
	}
	sort.Strings(names)
	if len(names) < 20 {
		t.Fatalf("only %d documents found under docs/ — the glob has drifted and this test "+
			"would pass by checking almost nothing", len(names))
	}
	return names
}

func TestEveryDocIsIndexed(t *testing.T) {
	index := parseDocsIndex(t)
	for _, name := range docsOnDisk(t) {
		if _, ok := index[name]; !ok {
			t.Errorf("docs/%s exists but docs/%s does not index it. Add a line saying WHEN to "+
				"read it — a document nothing points at is a document nobody reads.",
				name, docsIndexFile)
		}
	}
}

func TestTheIndexNamesNoMissingDoc(t *testing.T) {
	onDisk := map[string]bool{}
	for _, name := range docsOnDisk(t) {
		onDisk[name] = true
	}
	for name := range parseDocsIndex(t) {
		if !onDisk[name] {
			t.Errorf("docs/%s indexes %q, which does not exist — it was renamed or deleted. "+
				"A stale index is worse than none, because it is confidently wrong.",
				docsIndexFile, name)
		}
	}
}

// The trigger is the whole value of the index: the filename is already visible
// in a directory listing, and what is missing is the circumstance that should
// send someone to the file.
func TestEveryIndexEntryGivesATrigger(t *testing.T) {
	for name, trigger := range parseDocsIndex(t) {
		if len(trigger) > 160 {
			t.Errorf("%s: the trigger is %d characters; keep it to one scannable line", name, len(trigger))
		}
		// A trigger that merely restates the title tells a reader nothing they
		// could not get from ls.
		bare := strings.ToLower(strings.TrimSuffix(name, ".md"))
		words := strings.FieldsFunc(strings.ToLower(trigger), func(r rune) bool {
			return !(r >= 'a' && r <= 'z')
		})
		if len(words) < 4 {
			t.Errorf("%s: trigger %q is too short to name a circumstance", name, trigger)
		}
		if strings.ReplaceAll(bare, "_", " ") == strings.Join(words, " ") {
			t.Errorf("%s: trigger %q only restates the filename", name, trigger)
		}
	}
}

// CONTROL: the parser must actually match the index's real format. Without this
// a formatting change would silently yield zero entries, and both set-equality
// tests above would pass by comparing nothing.
func TestTheDocsIndexParserMatchesTheRealFormat(t *testing.T) {
	// Pure pattern checks first, so they still run when the index is missing.
	if !docsIndexEntry.MatchString("- [A_B.md](A_B.md) — before doing the thing") {
		t.Error("the entry pattern rejected a well-formed line")
	}
	if docsIndexEntry.MatchString("- [NOPE.md](NOPE.md) —") {
		t.Error("the entry pattern accepted an entry with no trigger")
	}
	if docsIndexEntry.MatchString("  - [NOPE.md](NOPE.md) — indented, so not a top-level entry") {
		t.Error("the entry pattern accepted an indented line")
	}
	// A mismatched link target is deliberately NOT the regexp's job: RE2 has no
	// backreferences, so the two names are captured separately and compared in
	// parseDocsIndex, which can then report the offending line number.
	m := docsIndexEntry.FindStringSubmatch("- [NOPE.md](OTHER.md) — mismatched target")
	if m == nil || m[1] == m[2] {
		t.Error("the entry pattern no longer captures link text and target separately, " +
			"so parseDocsIndex cannot compare them")
	}

	if n := len(parseDocsIndex(t)); n < 20 {
		t.Fatalf("the index parser matched only %d entries — the regexp has drifted from the "+
			"format used in docs/%s, and the coverage tests are checking nothing", n, docsIndexFile)
	}
}
