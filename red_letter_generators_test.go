package bibletext

// scripts/gen-bsb-redletter.py, gen-web-redletter.py and gen-nkjv-redletter.py,
// run on invented sources and invented runtime text. Each writes a table of
// rune spans; these tests run it into a temporary file, never over the shipped
// tables, and the NKJV's runs derive only (--no-fetch): no key is read and no
// call is made.
//
// The defect they guard against: a generator took the first copy of a run's words
// at or after the end of the run before it, so a run could land inside a
// longer word ("Go" in "Gospel") or on the same words earlier in the verse,
// and the NKJV's searched again from the verse's start and folded what it
// found into the run before, dropping the run and reporting success. A run
// placed by its offset alone has the same defect where the runtime verse
// differs before it: the offset can fall on a later copy of its words, so
// the text before the run is compared too.

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
// read it ("Verses": book, chapter, then the verses), with its notes in the
// app's own shape.
type redLetterGenVerse struct {
	Verse     int
	Text      string
	Footnotes []Footnote `json:"footnotes,omitempty"`
}

// writeRedLetterGenCache writes Matthew 1's verses as a runtime cache file,
// wrapped in "data" as the NKJV generator requires and the others accept.
func writeRedLetterGenCache(t *testing.T, path string, texts map[int]string) {
	t.Helper()
	writeRedLetterGenCacheWithNotes(t, path, texts, nil)
}

// writeRedLetterGenCacheWithNotes is writeRedLetterGenCache with a note
// anchored at each rune offset that notes lists for a verse.
func writeRedLetterGenCacheWithNotes(t *testing.T, path string, texts map[int]string, notes map[int][]int) {
	t.Helper()
	var verses []redLetterGenVerse
	for v := 1; v <= len(texts); v++ {
		verse := redLetterGenVerse{Verse: v, Text: texts[v]}
		for _, at := range notes[v] {
			verse.Footnotes = append(verse.Footnotes, Footnote{Anchor: at, Text: "An invented note.", Kind: "crossref", Caller: "-"})
		}
		verses = append(verses, verse)
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

// runeAt is the rune offset in text of the first copy of part.
func runeAt(text, part string) int {
	return utf8.RuneCountInString(text[:strings.Index(text, part)])
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

// redLetterGenModes are the two ways each refusal is run: it must stand
// whether or not a run may be placed by the one copy of its words.
var redLetterGenModes = []struct {
	name  string
	flags []string
}{{"by default", nil}, {"with --allow-relocated", []string{"--allow-relocated"}}}

// runBSBGen runs the BSB generator on an invented Matthew 1, as USFM, and its
// runtime verses.
func runBSBGen(t *testing.T, usfm string, text map[int]string, flags ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	writeRedLetterGenFile(t, filepath.Join(dir, "usfm", "MAT.usfm"), usfm)
	cache := filepath.Join(dir, "bsb.json")
	writeRedLetterGenCache(t, cache, text)
	return runRedLetterGen(t, filepath.Join(dir, "out.go"), append([]string{
		filepath.Join("scripts", "gen-bsb-redletter.py"), "--usfm", filepath.Join(dir, "usfm"), "--bsb", cache,
	}, flags...)...)
}

// runWEBGen runs the WEB generator with the same invented source and runtime
// verses standing for both editions.
func runWEBGen(t *testing.T, usfm string, text map[int]string, flags ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	writeRedLetterGenFile(t, filepath.Join(dir, "web", "70-MATeng-web.usfm"), usfm)
	writeRedLetterGenFile(t, filepath.Join(dir, "webc", "70-MATeng-web-c.usfm"), usfm)
	writeRedLetterGenCache(t, filepath.Join(dir, "web.json"), text)
	writeRedLetterGenCache(t, filepath.Join(dir, "webc.json"), text)
	return runRedLetterGen(t, filepath.Join(dir, "out.go"), append([]string{
		filepath.Join("scripts", "gen-web-redletter.py"),
		"--web-usfm", filepath.Join(dir, "web"), "--web-cache", filepath.Join(dir, "web.json"),
		"--webc-usfm", filepath.Join(dir, "webc"), "--webc-cache", filepath.Join(dir, "webc.json"),
	}, flags...)...)
}

// The refusals: the runtime text differs before the run and holds its words
// twice, so nothing says which copy was marked, whether the run's offset
// falls on neither copy or on the later one; and the only place for the run's
// letters is the start of a longer word.
func TestTheBSBRedLetterGeneratorRefusesARunItCannotPlace(t *testing.T) {
	for _, c := range []struct{ name, usfm, runtime string }{
		{"two copies, and the text before the run differs",
			`\v 1 They asked him, and he said, \wj “Go.”\wj*`, "He said, “Go.” They said, “Go.”"},
		{"two copies, the text before the run differs, and its offset falls on the later copy",
			`\v 1 Jesus answered them, \wj “Come.”\wj* And they, “Come.”`, "He said, “Come.” And they, “Come.”"},
		{"an end inside a word", `\v 1 He said, \wj Go\wj* now.`, "He said, Gospel now."},
	} {
		for _, mode := range redLetterGenModes {
			t.Run(c.name+", "+mode.name, func(t *testing.T) {
				src, log, err := runBSBGen(t, "\\id MAT\n\\c 1\n\\p\n"+c.usfm+"\n", map[int]string{1: c.runtime}, mode.flags...)
				if err == nil {
					t.Fatalf("the generator wrote a table:\n%s", src)
				}
				if !strings.Contains(log, "Matthew 1:1") {
					t.Errorf("the refusal does not name the verse:\n%s", log)
				}
			})
		}
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
		{"two copies, the text before the run differs, and its offset falls on the later copy",
			`\v 1 Jesus answered them, \wj “Come.”\wj* And they, “Come.”`, "He said, “Come.” And they, “Come.”"},
		{"an end inside a word", `\v 1 He said, \wj Go\wj* now.`, "He said, Gospel now."},
	} {
		for _, mode := range redLetterGenModes {
			t.Run(c.name+", "+mode.name, func(t *testing.T) {
				src, log, err := runWEBGen(t, "\\id MAT\n\\c 1\n\\p\n"+c.usfm+"\n", map[int]string{1: c.runtime}, mode.flags...)
				if err == nil {
					t.Fatalf("the generator wrote a table:\n%s", src)
				}
				if !strings.Contains(log, "Matthew 1:1") {
					t.Errorf("the refusal does not name the verse:\n%s", log)
				}
			})
		}
	}
}

// A run whose verse differs before it in the runtime text, and whose words the
// verse holds once: by default generation fails and names the verse; with
// --allow-relocated the run is placed on that copy and the summary lists it.
func TestTheRedLetterGeneratorsPlaceARunByItsOnlyCopyOnlyWhenAllowed(t *testing.T) {
	runtime := map[int]string{1: "He said, “Follow me.” And they went."}
	usfm := "\\id MAT\n\\c 1\n\\p\n\\v 1 Jesus answered them, \\wj “Follow me.”\\wj* And they went.\n"
	nkjv := `<p class="p">` + nkjvGenVerse(1, `Jesus answered them, <span class="wj">“Follow me.”</span> And they went.`) + `</p>`
	for _, g := range []struct {
		name, table, listed string
		run                 func(t *testing.T, flags ...string) (string, string, error)
	}{
		{"bsb", "bsbRedLetterSpans", "placed by its only copy: Matthew 1:1", func(t *testing.T, flags ...string) (string, string, error) {
			return runBSBGen(t, usfm, runtime, flags...)
		}},
		{"web", "webRedLetterSpans", "placed by its only copy: Matthew 1:1", func(t *testing.T, flags ...string) (string, string, error) {
			return runWEBGen(t, usfm, runtime, flags...)
		}},
		{"nkjv", "nkjvRedLetterSpans", "not their place: Matthew 1:1 run 1", func(t *testing.T, flags ...string) (string, string, error) {
			return runNKJVGen(t, nkjv, runtime, nil, nil, flags...)
		}},
	} {
		t.Run(g.name+", by default", func(t *testing.T) {
			src, log, err := g.run(t)
			if err == nil {
				t.Fatalf("the generator wrote a table:\n%s\n%s", src, log)
			}
			if !strings.Contains(log, "Matthew 1:1") || !strings.Contains(log, "--allow-relocated") {
				t.Errorf("the refusal does not name the verse and the option:\n%s", log)
			}
		})
		t.Run(g.name+", with --allow-relocated", func(t *testing.T) {
			src, log, err := g.run(t, "--allow-relocated")
			if err != nil {
				t.Fatalf("%v\n%s", err, log)
			}
			checkRedLetterGenSpans(t, redLetterGenSpans(t, src, g.table), map[string]string{
				"Matthew 1:1": lastCopy(runtime[1], "“Follow me.”"),
			})
			if !strings.Contains(log, g.listed) {
				t.Errorf("the summary does not list the run (%q):\n%s", g.listed, log)
			}
		})
	}
}

// A chapter's label and description and a verse's alternate or published
// number begin with the letters of the chapter and verse markers. They are
// not chapter or verse numbers, and their contents are not the verse's text.
func TestTheRedLetterGeneratorsReadChapterAndVerseMarkersExactly(t *testing.T) {
	usfm := `\id MAT
\cl Chapter
\c 1
\cd An invented description of the chapter.
\p
\v 1 \va 2\va* He said, \wj “Come.”\wj*
\v 2 \vp 2a\vp* They went, and he said, \wj “Go.”\wj*
`
	text := map[int]string{1: "He said, “Come.”", 2: "They went, and he said, “Go.”"}
	want := map[string]string{"Matthew 1:1": lastCopy(text[1], "“Come.”"), "Matthew 1:2": lastCopy(text[2], "“Go.”")}
	t.Run("bsb", func(t *testing.T) {
		src, log, err := runBSBGen(t, usfm, text)
		if err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
		checkRedLetterGenSpans(t, redLetterGenSpans(t, src, "bsbRedLetterSpans"), want)
	})
	t.Run("web", func(t *testing.T) {
		src, log, err := runWEBGen(t, usfm, text)
		if err != nil {
			t.Fatalf("%v\n%s", err, log)
		}
		checkRedLetterGenSpans(t, redLetterGenSpans(t, src, "webRedLetterSpans"), want)
	})
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

// nkjvGenChapters are the chapters the NKJV generator fetches and requires,
// as BOOKS in the script lists them.
var nkjvGenChapters = map[string]int{"MAT": 28, "MRK": 16, "LUK": 24, "JHN": 21, "ACT": 28,
	"1CO": 16, "2CO": 13, "1TI": 6, "REV": 22}

// writeNKJVGenCache writes every chapter response the generator reads, each
// empty but Matthew 1, in the shape API.Bible serves ("data", "content"),
// leaving out the chapters named in skip.
func writeNKJVGenCache(t *testing.T, dir, matthew1 string, skip ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for code, n := range nkjvGenChapters {
		for ch := 1; ch <= n; ch++ {
			name := fmt.Sprintf("%s.%d", code, ch)
			if strings.Contains(" "+strings.Join(skip, " ")+" ", " "+name+" ") {
				continue
			}
			content := ""
			if name == "MAT.1" {
				content = matthew1
			}
			blob, err := json.Marshal(map[string]any{"data": map[string]any{"content": content}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".json"), blob, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func nkjvGenVerse(v int, inner string) string {
	return fmt.Sprintf(`<span class="verse-span" data-verse-id="MAT.1.%d" data-verse-org-ids="MAT.1.%d">`+
		`<span data-number="%d" data-sid="MAT 1:%d" class="v">%d</span>%s</span>`, v, v, v, v, v, inner)
}

// runNKJVGen derives the NKJV's table from an invented Matthew 1, as API.Bible
// serves it, and the app's verses with a note anchored at each offset notes
// lists, with the chapters named in skip left out of the cache.
func runNKJVGen(t *testing.T, matthew1 string, text map[int]string, notes map[int][]int, skip []string, flags ...string) (string, string, error) {
	t.Helper()
	dir := t.TempDir()
	writeNKJVGenCache(t, filepath.Join(dir, "html"), matthew1, skip...)
	cache := filepath.Join(dir, "nkjv.json")
	writeRedLetterGenCacheWithNotes(t, cache, text, notes)
	return runRedLetterGen(t, filepath.Join(dir, "out.go"), append([]string{
		filepath.Join("scripts", "gen-nkjv-redletter.py"), "--no-fetch",
		"--html-cache", filepath.Join(dir, "html"), "--app-cache", cache,
	}, flags...)...)
}

// The NKJV's markup is HTML, read for its text; a verse's number and a
// heading inside it are not in the app's verse, italic supplied words and the
// small capitals nest inside a run, and the HTML runs verses together where
// the app's text has a space. At some of the NKJV's notes the publisher's
// text has no space on either side of the note, so two words run together in
// the markup fetched without notes and in the app's verse alike; where the
// app's verse anchors a note, that place is the edge of a word, whether two
// runs touch there (verse 7), a run ends there before black text (verse 8) or
// a run begins there after black text (verse 9). Each verse is placed the same
// with --allow-relocated, which none needs. The text here is invented.
func TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace(t *testing.T) {
	matthew1 := `<p class="p">` +
		nkjvGenVerse(1, `The Gospel he preached was this: he said, “<span class="wj">Go</span>.”`) +
		nkjvGenVerse(2, `The steward said, I am he. Then the teacher said, <span class="wj">I am he.</span>`) +
		nkjvGenVerse(3, `He answered them, come and see. Then he said,`) + `</p>` +
		`<p class="s1">Come and See</p>` +
		`<p class="p"><span class="verse-span" data-verse-id="MAT.1.3" data-verse-org-ids="MAT.1.3">“<span class="wj">Come and see.</span>”</span>` +
		nkjvGenVerse(4, `He said, <span class="wj">“My yoke <span class="it">is</span> light, says the <span class="nd">Keeper</span>.”</span>`) +
		nkjvGenVerse(5, `He said, <span class="wj">“Follow</span>`) +
		nkjvGenVerse(6, `<span class="wj">me now.”</span> And they went.`) +
		nkjvGenVerse(7, `He said, <span class="wj">“Come</span><span class="wj">home now.”</span>`) +
		nkjvGenVerse(8, `<span class="wj">“These</span>folk <span class="wj">come near.”</span>`) +
		nkjvGenVerse(9, `He said<span class="wj">home now.</span>`) + `</p>`
	text := map[int]string{
		1: "The Gospel he preached was this: he said, “Go.”",
		2: "The steward said, I am he. Then the teacher said, I am he.",
		3: "He answered them, come and see. Then he said, “Come and see.”",
		4: "He said, “My yoke is light, says the KEEPER.”",
		5: "He said, “Follow",
		6: "me now.” And they went.",
		7: "He said, “Comehome now.”",
		8: "“Thesefolk come near.”",
		9: "He saidhome now.",
	}
	notes := map[int][]int{7: {runeAt(text[7], "home")}, 8: {runeAt(text[8], "folk")}, 9: {runeAt(text[9], "home")}}
	for _, mode := range redLetterGenModes {
		t.Run(mode.name, func(t *testing.T) {
			src, log, err := runNKJVGen(t, matthew1, text, notes, nil, mode.flags...)
			if err != nil {
				t.Fatalf("gen-nkjv-redletter.py: %v\n%s", err, log)
			}
			checkRedLetterGenSpans(t, redLetterGenSpans(t, src, "nkjvRedLetterSpans"), map[string]string{
				"Matthew 1:1": lastCopy(text[1], "Go"),
				"Matthew 1:2": lastCopy(text[2], "I am he."),
				"Matthew 1:3": lastCopy(text[3], "Come and see."),
				"Matthew 1:4": lastCopy(text[4], "“My yoke is light, says the KEEPER.”"),
				"Matthew 1:5": lastCopy(text[5], "“Follow"),
				"Matthew 1:6": lastCopy(text[6], "me now.”"),
				// The two runs touch at the note's place and make one span.
				"Matthew 1:7": lastCopy(text[7], "“Comehome now.”"),
				// The run ends at the note's place; the words after it are black.
				"Matthew 1:8": lastCopy(text[8], "“These") + ", " + lastCopy(text[8], "come near.”"),
				// The run begins at the note's place; the words before it are black.
				"Matthew 1:9": lastCopy(text[9], "home now."),
			})
		})
	}
}

// Each refusal is a table the generator used to write and report as made: a
// second run whose words the app's verse holds only before the first (it
// searched again from the start and folded the run into the first, so the
// run shipped black); a run whose verse differs before it, so that its count
// falls on a later copy of its words; a run whose only place is the start of
// a longer word; and a chapter missing from the cache, whose verses were left
// out of the table, which reads as black letter. Two words run together at a
// run's edge are one word unless the app's verse anchors a note at that very
// place: where two runs touch there, a run ends there or a run begins there,
// with no note, or a note elsewhere in the verse, the run is refused.
func TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace(t *testing.T) {
	const (
		joined    = "He said, “Comehome now.”"
		ending    = "“Thesefolk come near.”"
		beginning = "He saidhome now."
	)
	for _, c := range []struct {
		name, verse, text string
		skip              []string
		notes             []int
	}{
		{"a second run found only before the first",
			nkjvGenVerse(1, `<span class="wj">Come.</span> He went on. <span class="wj">Come.</span>`),
			"Come. He went on. Go.", nil, nil},
		{"two copies, the text before the run differs, and its count falls on the later copy",
			nkjvGenVerse(1, `Then Jesus answered them, <span class="wj">“Come.”</span> And they, “Come.”`),
			"He said, “Come.” And they, “Come.”", nil, nil},
		{"an end inside a word",
			nkjvGenVerse(1, `He said, <span class="wj">Go</span> now.`), "He said, Gospel now.", nil, nil},
		{"a chapter missing from the cache",
			nkjvGenVerse(1, `He said, <span class="wj">“Go.”</span>`), "He said, “Go.”", []string{"MRK.3"}, nil},
		{"two runs touching where two words run together, with no note there",
			nkjvGenVerse(1, `He said, <span class="wj">“Come</span><span class="wj">home now.”</span>`), joined, nil, nil},
		{"two runs touching where two words run together, with a note elsewhere in the verse",
			nkjvGenVerse(1, `He said, <span class="wj">“Come</span><span class="wj">home now.”</span>`), joined, nil,
			[]int{runeAt(joined, "now")}},
		{"a run ending where two words run together, with no note there",
			nkjvGenVerse(1, `<span class="wj">“These</span>folk <span class="wj">come near.”</span>`), ending, nil, nil},
		{"a run ending where two words run together, with a note elsewhere in the verse",
			nkjvGenVerse(1, `<span class="wj">“These</span>folk <span class="wj">come near.”</span>`), ending, nil,
			[]int{runeAt(ending, "near")}},
		{"a run beginning where two words run together, with no note there",
			nkjvGenVerse(1, `He said<span class="wj">home now.</span>`), beginning, nil, nil},
		{"a run beginning where two words run together, with a note elsewhere in the verse",
			nkjvGenVerse(1, `He said<span class="wj">home now.</span>`), beginning, nil,
			[]int{runeAt(beginning, "now")}},
	} {
		for _, mode := range redLetterGenModes {
			t.Run(c.name+", "+mode.name, func(t *testing.T) {
				src, log, err := runNKJVGen(t, `<p class="p">`+c.verse+`</p>`, map[int]string{1: c.text}, map[int][]int{1: c.notes}, c.skip, mode.flags...)
				if err == nil {
					t.Fatalf("the generator wrote a table:\n%s\n%s", src, log)
				}
				want := "Matthew 1:1"
				if c.skip != nil {
					want = c.skip[0]
				}
				if !strings.Contains(log, want) {
					t.Errorf("the refusal does not name %s:\n%s", want, log)
				}
			})
		}
	}
}

// A derive-only run reads no key: the generator, loaded and run with
// --no-fetch while an invented key is in its environment, holds the key in
// none of its globals afterwards, and has taken it out of the environment of
// what it starts.
func TestTheNKJVRedLetterGeneratorKeepsNoKeyWhenItDoesNotFetch(t *testing.T) {
	py := pythonForScripts(t)
	dir := t.TempDir()
	writeNKJVGenCache(t, filepath.Join(dir, "html"), `<p class="p">`+nkjvGenVerse(1, `He said, <span class="wj">“Go.”</span>`)+`</p>`)
	cache := filepath.Join(dir, "nkjv.json")
	writeRedLetterGenCache(t, cache, map[int]string{1: "He said, “Go.”"})
	const invented = "invented-key-for-the-derive-only-test"
	probe := `import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("gen_nkjv_redletter", sys.argv[1])
gen = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gen)
sys.argv = sys.argv[1:]
gen.main()
held = sorted(name for name, value in vars(gen).items() if isinstance(value, str) and "` + invented + `" in value)
print("globals holding the key:", held, "; in the environment:", "BIBLE_API_KEY" in os.environ)
sys.exit(1 if held or "BIBLE_API_KEY" in os.environ else 0)
`
	// -B: loading the script as a module would otherwise leave its bytecode
	// beside it in the checkout.
	cmd := exec.Command(py, "-I", "-B", "-c", probe, filepath.Join("scripts", "gen-nkjv-redletter.py"), "--no-fetch",
		"--html-cache", filepath.Join(dir, "html"), "--app-cache", cache, "--out", filepath.Join(dir, "out.go"))
	cmd.Env = append(os.Environ(), "BIBLE_API_KEY="+invented)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
