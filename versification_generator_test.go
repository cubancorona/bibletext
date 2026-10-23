package bibletext

// scripts/gen-versification.py, run on invented editions and on the caches
// the app keeps. It writes versification_data.go from the app's own cache
// files; these tests run it into a temporary file, never over the shipped
// table.

import (
	"encoding/json"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// genVersificationVerse is one verse of a cache file as the generator reads
// it ("Verses": book, chapter, then the verses).
type genVersificationVerse struct {
	Verse int
	Text  string
}

func writeGenVersificationCache(t *testing.T, dir, name string, verses map[string]map[string][]genVersificationVerse) string {
	t.Helper()
	blob, err := json.Marshal(map[string]any{"Verses": verses})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runGenVersification runs the generator over the reference and one target
// given as the BSB, and returns the source it wrote.
func runGenVersification(t *testing.T, args ...string) string {
	t.Helper()
	py := pythonForScripts(t)
	out := filepath.Join(t.TempDir(), "versification_data.go")
	args = append([]string{filepath.Join("scripts", "gen-versification.py")}, append(args, "--out", out)...)
	if log, err := runScript(t, py, args...); err != nil {
		t.Fatalf("gen-versification.py: %v\n%s", err, log)
	}
	src, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}

// genVersificationRows is the rows of one kind (absent, moved, extra) the
// generated source holds for one translation, each as its literal.
func genVersificationRows(t *testing.T, src, id, kind string) []string {
	t.Helper()
	at := strings.Index(src, "\t\""+id+"\": {")
	if at < 0 {
		t.Fatalf("no %q entry in the generated table:\n%s", id, src)
	}
	block := src[at:]
	k := strings.Index(block, "\t\t"+kind+": ")
	if k < 0 {
		t.Fatalf("no %s rows for %q", kind, id)
	}
	block = block[k:]
	block = block[:strings.Index(block, "\n\t\t},")]
	rows := regexp.MustCompile(`\{"[^"]+", [0-9, ]+\}`).FindAllString(block, -1)
	sort.Strings(rows)
	return rows
}

// An edition that drops a verse and prints a repeat of another of the
// reference's verses in a new slot: a refrain the book uses more than once,
// shaped on Mark 9, where "where their worm doesn't die" stands at 9:44, 9:46
// and 9:48; and a benediction the chapter already has, shaped on Romans 16,
// whose doxology the BSB numbers 16:25-27. The generator recorded the dropped
// verse as moved to the repeat, because the repeat matches it word for word;
// it matches the verse the edition kept just as well, so it says nothing
// about where the dropped one went. The doxology's own move to new slots is
// the control: it matches nothing the edition kept, and stays a move.
func TestTheVersificationGeneratorDoesNotMoveAVerseToARepeat(t *testing.T) {
	const refrain = "where the lamp never fails and the oil is never spent"
	ch := func(vs ...genVersificationVerse) []genVersificationVerse { return vs }
	v := func(n int, text string) genVersificationVerse { return genVersificationVerse{n, text} }
	reference := map[string]map[string][]genVersificationVerse{
		"Mark": {"9": ch(
			v(42, "Whoever troubles one of these small ones would be better off in the deep sea."),
			v(43, "If your hand leads you astray, cut it off and enter the city maimed."),
			v(44, refrain),
			v(45, "If your foot leads you astray, cut it off and enter the city lame."),
			v(46, refrain),
			v(47, "If your eye leads you astray, pluck it out and enter the kingdom with one eye."),
			v(48, refrain),
			v(49, "For everyone will be salted with fire."),
			v(50, "Salt is good, but if the salt has lost its savour, how will you season it?"),
		)},
		"Romans": {
			"14": ch(
				v(23, "Whatever does not come from trust is a stumbling."),
				v(24, "Now to the Keeper who steadies you through the long night of waiting"),
				v(25, "according to the mystery kept silent through ages and eons past"),
				v(26, "now revealed through the prophetic writings to every nation, to bring obedience"),
			),
			"16": ch(
				v(20, "The kindness of our Keeper rest upon you all, and peace soon."),
				v(21, "Timon my fellow worker greets you, with my kinsmen."),
				v(22, "I, the scribe of this letter, greet you in the Keeper."),
				v(23, "Gaius, host of the whole assembly, greets you."),
				v(24, "The kindness of our Keeper rest upon you all."),
			),
		},
	}
	edition := map[string]map[string][]genVersificationVerse{
		"Mark": {"9": ch(
			v(42, reference["Mark"]["9"][0].Text),
			v(43, reference["Mark"]["9"][1].Text),
			v(45, reference["Mark"]["9"][3].Text),
			v(47, reference["Mark"]["9"][5].Text),
			v(48, refrain),
			v(49, reference["Mark"]["9"][7].Text),
			v(50, reference["Mark"]["9"][8].Text),
			v(51, refrain),
		)},
		"Romans": {
			"14": ch(v(23, reference["Romans"]["14"][0].Text)),
			"16": ch(
				v(20, reference["Romans"]["16"][0].Text),
				v(21, reference["Romans"]["16"][1].Text),
				v(22, reference["Romans"]["16"][2].Text),
				v(23, reference["Romans"]["16"][3].Text),
				v(25, reference["Romans"]["14"][1].Text),
				v(26, reference["Romans"]["14"][2].Text),
				v(27, reference["Romans"]["14"][3].Text),
				v(28, reference["Romans"]["16"][0].Text),
			),
		},
	}
	dir := t.TempDir()
	src := runGenVersification(t,
		"--web", writeGenVersificationCache(t, dir, "web.json", reference),
		"--bsb", writeGenVersificationCache(t, dir, "bsb.json", edition))
	for _, c := range []struct {
		kind string
		want []string
	}{
		{"absent", []string{`{"Mark", 9, 44}`, `{"Mark", 9, 46}`, `{"Romans", 16, 24}`}},
		{"moved", []string{`{"Romans", 14, 24, 16, 25}`, `{"Romans", 14, 25, 16, 26}`, `{"Romans", 14, 26, 16, 27}`}},
		{"extra", []string{`{"Mark", 9, 51}`, `{"Romans", 16, 28}`}},
	} {
		got := genVersificationRows(t, src, "bsb", c.kind)
		sort.Strings(c.want)
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("%s rows %v, want %v", c.kind, got, c.want)
		}
	}
}

// The generator, run on the caches the app keeps, writes the shipped table
// byte for byte once formatted: a change to its rules that changes a shipped
// row shows here before anyone regenerates. LOCAL-ONLY: the caches are
// downloaded data a CI checkout does not have, and the test is skipped
// without them. The NKJV's rows are left out on both sides, since its cache
// is licensed and is not read here.
func TestTheVersificationGeneratorReproducesTheShippedTables_LocalOnly(t *testing.T) {
	var args []string
	for _, id := range []string{"web", "bsb", "webc"} {
		path := realCachePath(cachePathForVersion(id))
		if _, err := os.Stat(path); err != nil {
			t.Skipf("local-only: no %s cache at the current epoch", id)
		}
		args = append(args, "--"+id, path)
	}
	nkjv := regexp.MustCompile(`(?s)\t"nkjv": \{\n.*?\n\t\},\n`)
	got, err := format.Source([]byte(runGenVersification(t, args...)))
	if err != nil {
		t.Fatalf("the generated source does not format: %v", err)
	}
	want, err := os.ReadFile("versification_data.go")
	if err != nil {
		t.Fatal(err)
	}
	shipped := nkjv.ReplaceAllString(string(want), "")
	if shipped == string(want) {
		t.Fatal("no NKJV entry was found to leave out; the comparison would not be like for like")
	}
	if string(got) != shipped {
		t.Error("the generator, run on the local caches, does not write the shipped table (the NKJV's rows aside)")
	}
}
