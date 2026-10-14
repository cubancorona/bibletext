package bibletext

// scripts/gen-bsb-redletter.py and gen-web-redletter.py, run on invented
// sources and invented runtime text. Each writes a table of rune spans; these
// tests run it into a temporary file, never over the shipped tables.
//
// The defect they guard against: a generator took the first copy of a run's
// words at or after the end of the run before it, so a run could land inside
// a longer word ("Go" in "Gospel") or on the same words earlier in the verse.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// redLetterGenVerse is one verse of a runtime cache file as the generators
// read it ("Verses": book, chapter, then the verses).
type redLetterGenVerse struct {
	Verse int
	Text  string
}

// writeRedLetterGenCache writes Matthew 1's verses as a runtime cache file,
// wrapped in "data" as the NKJV generator requires and the others accept.
func writeRedLetterGenCache(t *testing.T, path string, texts map[int]string) {
	t.Helper()
	var verses []redLetterGenVerse
	for v := 1; v <= len(texts); v++ {
		verses = append(verses, redLetterGenVerse{v, texts[v]})
	}
	blob, err := json.Marshal(map[string]any{"data": map[string]any{
		"Verses": map[string]any{"Matthew": map[string]any{"1": verses}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRedLetterGenFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runRedLetterGen runs one generator and returns the table it wrote, or its
// output and error when it refused.
func runRedLetterGen(t *testing.T, out string, args ...string) (string, string, error) {
	t.Helper()
	py := pythonForScripts(t)
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Fatalf("no gofmt on PATH, which every generator formats its table with: %v", err)
	}
	log, err := runScript(t, py, append(args, "--out", out)...)
	if err != nil {
		return "", log, err
	}
	src, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("%v\n%s", readErr, log)
	}
	return string(src), log, nil
}

// redLetterGenSpans is each verse's span list in the generated table's map
// named name, as its literal: "{4, 6}, {9, 12}".
func redLetterGenSpans(t *testing.T, src, name string) map[string]string {
	t.Helper()
	at := strings.Index(src, "var "+name+" = map[string][]redLetterSpan{\n")
	if at < 0 {
		t.Fatalf("no %s in the generated table:\n%s", name, src)
	}
	block := src[at:]
	block = block[:strings.Index(block, "\n}\n")]
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`\t"([^"]+)":\s+\{(.*)\},\n`).FindAllStringSubmatch(block+"\n", -1) {
		out[m[1]] = m[2]
	}
	return out
}

// lastCopy is the rune span of the last copy of part in text, the copy each
// fixture's publisher marked.
func lastCopy(text, part string) string {
	i := strings.LastIndex(text, part)
	start := utf8.RuneCountInString(text[:i])
	return fmt.Sprintf("{%d, %d}", start, start+utf8.RuneCountInString(part))
}

func checkRedLetterGenSpans(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()
	for key, w := range want {
		if got[key] != w {
			t.Errorf("%s: spans {%s}, want {%s}", key, got[key], w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("the table holds %d verses, want %d: %v", len(got), len(want), got)
	}
}

// The BSB fixture's runtime text. Verses 1 to 4 each hold the marked words
// more than once: inside a longer word; in narration; in narration, with
// typography the publisher's file does not share; and in narration before a
// heading line that the publisher's file has inside the verse, whose letters
// the runtime verse does not hold.
var redLetterGenText = map[int]string{
	1: "The Gospel he preached was this: he said, “Go.”",
	2: "The steward said, I am he. Then the teacher said, I am he.",
	3: "In the town lived a potter named Tobiah. The teacher called to him, “Tobiah!”",
	4: "He answered them, come and see. Then he said, “Come and see.”",
	5: "He said, “Follow",
	6: "me now.” And they went.",
	7: "“Ask, and it will be given” he said.",
}

func TestTheBSBRedLetterGeneratorPlacesEachRunAtItsOwnPlace(t *testing.T) {
	dir := t.TempDir()
	writeRedLetterGenFile(t, filepath.Join(dir, "usfm", "MAT.usfm"), `\id MAT
\c 1
\p
\v 1 The Gospel he preached was this: he said, “\wj Go\wj*.”
\v 2 The steward said, I am he. Then the teacher said, \wj I am he.\wj*
\v 3 In the town lived a potter named Tobiah. The teacher called to him, \wj "Tobiah!"\wj*
\v 4 He answered them, come and see. Then he said,
\s1 Come and See
\r (\ref Elsewhere 1:1|ELS 1:1\ref*)
\p \wj “Come and see.”\wj*
\v 5 He said, \wj “Follow
\v 6 me now.”\wj* And they went.
\v 7 \wj “Ask, and it will be given\wj*\f + \fr 1:7 \ft An invented note.\f*\wj ”\wj* he said.
`)
	cache := filepath.Join(dir, "bsb.json")
	writeRedLetterGenCache(t, cache, redLetterGenText)
	src, log, err := runRedLetterGen(t, filepath.Join(dir, "out.go"),
		filepath.Join("scripts", "gen-bsb-redletter.py"), "--usfm", filepath.Join(dir, "usfm"), "--bsb", cache)
	if err != nil {
		t.Fatalf("gen-bsb-redletter.py: %v\n%s", err, log)
	}
	text := redLetterGenText
	checkRedLetterGenSpans(t, redLetterGenSpans(t, src, "bsbRedLetterSpans"), map[string]string{
		"Matthew 1:1": lastCopy(text[1], "Go"),
		"Matthew 1:2": lastCopy(text[2], "I am he."),
		// The publisher's straight quotation marks are not in the runtime
		// text, so the letters alone are matched and the span is the name.
		"Matthew 1:3": lastCopy(text[3], "Tobiah"),
		"Matthew 1:4": lastCopy(text[4], "“Come and see.”"),
		"Matthew 1:5": lastCopy(text[5], "“Follow"),
		"Matthew 1:6": lastCopy(text[6], "me now.”"),
		// The closing mark, marked on its own after the note, joins the run
		// it closes.
		"Matthew 1:7": lastCopy(text[7], "“Ask, and it will be given”"),
	})
}

// Two refusals: the runtime text differs before the run and holds its words
// twice, so nothing says which copy was marked; and the only place for the
// run's letters is the start of a longer word.
func TestTheBSBRedLetterGeneratorRefusesARunItCannotPlace(t *testing.T) {
	for _, c := range []struct{ name, usfm, runtime string }{
		{"two copies, and the text before the run differs",
			`\v 1 They asked him, and he said, \wj “Go.”\wj*`, "He said, “Go.” They said, “Go.”"},
		{"an end inside a word", `\v 1 He said, \wj Go\wj* now.`, "He said, Gospel now."},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRedLetterGenFile(t, filepath.Join(dir, "usfm", "MAT.usfm"), "\\id MAT\n\\c 1\n\\p\n"+c.usfm+"\n")
			cache := filepath.Join(dir, "bsb.json")
			writeRedLetterGenCache(t, cache, map[int]string{1: c.runtime})
			src, log, err := runRedLetterGen(t, filepath.Join(dir, "out.go"),
				filepath.Join("scripts", "gen-bsb-redletter.py"), "--usfm", filepath.Join(dir, "usfm"), "--bsb", cache)
			if err == nil {
				t.Fatalf("the generator wrote a table:\n%s", src)
			}
			if !strings.Contains(log, "Matthew 1:1") {
				t.Errorf("the refusal does not name the verse:\n%s", log)
			}
		})
	}
}

// The WEB's generator matches the publisher's punctuation only, and reads both
// editions; the same source stands for both here.
func TestTheWEBRedLetterGeneratorPlacesEachRunAtItsOwnPlace(t *testing.T) {
	dir := t.TempDir()
	usfm := `\id MAT
\c 1
\p
\v 1 The steward said, “I am he.” The teacher said, \wj “I am he.”\wj*
\v 2 The Gospel he preached was this: he said, “\wj Go\wj*.”
\v 3 He said, \wj “Come.”\wj* And they came, and he said again, \wj “Come.”\wj*
\v 4 \wj “Ask, and it will be given\wj*\f + \fr 1:4 \ft An invented note.\f*\wj ”\wj* he said.
\v 5 He answered them, come and see. Then he said,
\s1 Come and See
\p \wj “Come and see.”\wj*
`
	text := map[int]string{
		1: "The steward said, “I am he.” The teacher said, “I am he.”",
		2: "The Gospel he preached was this: he said, “Go.”",
		3: "He said, “Come.” And they came, and he said again, “Come.”",
		4: "“Ask, and it will be given” he said.",
		5: "He answered them, come and see. Then he said, “Come and see.”",
	}
	writeRedLetterGenFile(t, filepath.Join(dir, "web", "70-MATeng-web.usfm"), usfm)
	writeRedLetterGenFile(t, filepath.Join(dir, "webc", "70-MATeng-web-c.usfm"), usfm)
	writeRedLetterGenCache(t, filepath.Join(dir, "web.json"), text)
	writeRedLetterGenCache(t, filepath.Join(dir, "webc.json"), text)
	src, log, err := runRedLetterGen(t, filepath.Join(dir, "out.go"),
		filepath.Join("scripts", "gen-web-redletter.py"),
		"--web-usfm", filepath.Join(dir, "web"), "--web-cache", filepath.Join(dir, "web.json"),
		"--webc-usfm", filepath.Join(dir, "webc"), "--webc-cache", filepath.Join(dir, "webc.json"))
	if err != nil {
		t.Fatalf("gen-web-redletter.py: %v\n%s", err, log)
	}
	want := map[string]string{
		"Matthew 1:1": lastCopy(text[1], "“I am he.”"),
		"Matthew 1:2": lastCopy(text[2], "Go"),
		// Both copies are marked, each its own span with the narration black.
		"Matthew 1:3": lastCopy(text[3][:strings.Index(text[3], " And")], "“Come.”") + ", " + lastCopy(text[3], "“Come.”"),
		// The WEB's table keeps the closing mark as a span of its own.
		"Matthew 1:4": lastCopy(text[4], "“Ask, and it will be given") + ", " + lastCopy(text[4], "”"),
		"Matthew 1:5": lastCopy(text[5], "“Come and see.”"),
	}
	for _, name := range []string{"webRedLetterSpans", "webcRedLetterSpans"} {
		t.Run(name, func(t *testing.T) { checkRedLetterGenSpans(t, redLetterGenSpans(t, src, name), want) })
	}
}

func TestTheWEBRedLetterGeneratorRefusesARunItCannotPlace(t *testing.T) {
	for _, c := range []struct{ name, usfm, runtime string }{
		{"two copies, and the text before the run differs",
			`\v 1 They asked him, and he said, \wj “Go.”\wj*`, "He said, “Go.” They said, “Go.”"},
		{"an end inside a word", `\v 1 He said, \wj Go\wj* now.`, "He said, Gospel now."},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			usfm := "\\id MAT\n\\c 1\n\\p\n" + c.usfm + "\n"
			writeRedLetterGenFile(t, filepath.Join(dir, "web", "70-MATeng-web.usfm"), usfm)
			writeRedLetterGenFile(t, filepath.Join(dir, "webc", "70-MATeng-web-c.usfm"), usfm)
			writeRedLetterGenCache(t, filepath.Join(dir, "web.json"), map[int]string{1: c.runtime})
			writeRedLetterGenCache(t, filepath.Join(dir, "webc.json"), map[int]string{1: c.runtime})
			src, log, err := runRedLetterGen(t, filepath.Join(dir, "out.go"),
				filepath.Join("scripts", "gen-web-redletter.py"),
				"--web-usfm", filepath.Join(dir, "web"), "--web-cache", filepath.Join(dir, "web.json"),
				"--webc-usfm", filepath.Join(dir, "webc"), "--webc-cache", filepath.Join(dir, "webc.json"))
			if err == nil {
				t.Fatalf("the generator wrote a table:\n%s", src)
			}
			if !strings.Contains(log, "Matthew 1:1") {
				t.Errorf("the refusal does not name the verse:\n%s", log)
			}
		})
	}
}

// The BSB and WEB generators, run on the publishers' own USFM and the caches
// the app keeps, write the shipped tables byte for byte: a change to their
// rules that would change a shipped span shows here before anyone
// regenerates. LOCAL-ONLY: neither the USFM nor the caches are in a checkout.
// BIBLETEXT_BSB_USFM names the bsb_usfm directory unpacked from the BSB's
// download, and BIBLETEXT_WEB_USFM and BIBLETEXT_WEBC_USFM the directories
// unpacked from eBible's eng-web and eng-web-c USFM zips (TEXTUAL-DATA.md,
// section 4); each table is skipped without its sources. The NKJV is not
// here: its text is licensed and is not read by the suite.
func TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly(t *testing.T) {
	sources := func(t *testing.T, vars ...string) []string {
		var dirs []string
		for _, name := range vars {
			dir := os.Getenv(name)
			if dir == "" {
				t.Skipf("local-only: %s names no unpacked USFM", name)
			}
			dirs = append(dirs, dir)
		}
		return dirs
	}
	cache := func(t *testing.T, id string) string {
		path := realCachePath(cachePathForVersion(id))
		if _, err := os.Stat(path); err != nil {
			t.Skipf("local-only: no %s cache at the current epoch", id)
		}
		return path
	}
	same := func(t *testing.T, script, shipped string, args ...string) {
		src, log, err := runRedLetterGen(t, filepath.Join(t.TempDir(), shipped),
			append([]string{filepath.Join("scripts", script)}, args...)...)
		if err != nil {
			t.Fatalf("%s: %v\n%s", script, err, log)
		}
		want, err := os.ReadFile(shipped)
		if err != nil {
			t.Fatal(err)
		}
		if src != string(want) {
			t.Errorf("%s, run on the publisher's USFM and the local caches, does not write %s", script, shipped)
		}
	}
	t.Run("bsb", func(t *testing.T) {
		usfm := sources(t, "BIBLETEXT_BSB_USFM")
		same(t, "gen-bsb-redletter.py", "red_letter_bsb_data.go", "--usfm", usfm[0], "--bsb", cache(t, "bsb"))
	})
	t.Run("web", func(t *testing.T) {
		usfm := sources(t, "BIBLETEXT_WEB_USFM", "BIBLETEXT_WEBC_USFM")
		same(t, "gen-web-redletter.py", "red_letter_web_data.go",
			"--web-usfm", usfm[0], "--web-cache", cache(t, "web"),
			"--webc-usfm", usfm[1], "--webc-cache", cache(t, "webc"))
	})
}
