package main

// THE VERSION SWITCHER CARRIES THE SAME PASSAGE, NOT THE SAME NUMBER.
//
// reader.js appends the shared verse to each translation pill (carryVerse). It
// used to append it as it stood, so a reader on WEB Romans 14:24-26 who tapped
// BSB landed on the BSB's 14:24-26, which do not exist (the doxology is its
// 16:25-27), and a reader on WEBC Daniel 3:91 who tapped WEB was sent to a
// verse the WEB does not have. Each pill on a chapter numbered differently now
// carries the chapter's verse map, written from the versification tables
// (switcherVerseMap), and the shipped carry code reads it.
//
// The fixture below numbers its chapters as the real editions do, as the
// tables record them: the doxology, the Song of the Three, the BSB's omitted
// Mark 9:44 and 9:46, the NKJV's Acts 8:37, and the Greek Esther. The cases
// run the shipped bytes between reader.js's VERSE_CARRY markers, under node
// or JavaScriptCore as highlight_band_test.go does, with the data-vmap and
// href each page was really built with, in both states of the NKJV switch.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

const (
	carryBegin = "/*__VERSE_CARRY_BEGIN__*/"
	carryEnd   = "/*__VERSE_CARRY_END__*/"
)

// switcherFixtureVersions is the three public-domain editions over the
// chapters whose numbering differs, each numbered as the edition numbers it,
// with John 3 as a chapter every edition numbers alike.
func switcherFixtureVersions() []loadedVersion {
	run := func(lo, hi int, skip ...int) []int {
		var out []int
	next:
		for n := lo; n <= hi; n++ {
			for _, s := range skip {
				if n == s {
					continue next
				}
			}
			out = append(out, n)
		}
		return out
	}
	type chapter struct {
		book  string
		ch    int
		verse []int
	}
	build := func(chs []chapter) *bibletext.BibleData {
		bd := &bibletext.BibleData{Verses: map[string]map[int][]bibletext.Verse{}}
		for _, c := range chs {
			if bd.Verses[c.book] == nil {
				bd.Books = append(bd.Books, c.book)
				bd.Verses[c.book] = map[int][]bibletext.Verse{}
			}
			for _, n := range c.verse {
				bd.Verses[c.book][c.ch] = append(bd.Verses[c.book][c.ch], bibletext.Verse{
					BookName: c.book, Book: c.book, Chapter: c.ch, Verse: n,
					Text: fmt.Sprintf("Switcher fixture verse %d of %s %d.", n, c.book, c.ch),
				})
			}
		}
		return bd
	}
	web := []chapter{
		{"Esther", 1, run(1, 22)}, {"Daniel", 3, run(1, 30)}, {"Mark", 9, run(1, 50)},
		{"John", 3, run(1, 36)}, {"Acts", 8, run(1, 40, 37)},
		{"Romans", 14, run(1, 26)}, {"Romans", 16, run(1, 24)},
	}
	bsb := []chapter{
		{"Esther", 1, run(1, 22)}, {"Daniel", 3, run(1, 30)}, {"Mark", 9, run(1, 50, 44, 46)},
		{"John", 3, run(1, 36)}, {"Acts", 8, run(1, 40, 37)},
		{"Romans", 14, run(1, 23)}, {"Romans", 16, run(1, 27, 24)},
	}
	webc := []chapter{
		{"Esther", 1, run(1, 22)}, {"Daniel", 3, run(1, 97)}, {"Mark", 9, run(1, 50)},
		{"John", 3, run(1, 36)}, {"Acts", 8, run(1, 40, 37)},
		{"Romans", 14, run(1, 26)}, {"Romans", 16, run(1, 24)},
	}
	var loaded []loadedVersion
	for _, pv := range publishedVersions() {
		chs := map[string][]chapter{"web": web, "bsb": bsb, "webc": webc}[pv.ID]
		loaded = append(loaded, loadedVersion{webVersion: pv, bible: build(chs)})
	}
	return loaded
}

// switcherSiteOff builds the fixture with the NKJV's text off.
func switcherSiteOff(t *testing.T) (string, []loadedVersion) {
	t.Helper()
	versions := switcherFixtureVersions()
	site := &siteWriter{root: filepath.Join(t.TempDir(), "site")}
	if err := writeSite(site, versions, noticedVersionsFor(false)); err != nil {
		t.Fatalf("writeSite: %v", err)
	}
	return site.root, versions
}

// switcherSiteOn builds it with the text on, through run(), the NKJV numbered
// as the tables say it is.
func switcherSiteOn(t *testing.T) (string, []loadedVersion) {
	t.Helper()
	published := switcherFixtureVersions()
	licensed := syntheticLicensedText(t, referenceOf(t, published), fixtureVerseText)
	standIn(t, published, func(string) (bibletext.LicensedEdition, error) {
		return bibletext.LicensedEdition{Bible: licensed, Requests: 67, CopyrightLines: 1}, nil
	})
	t.Setenv(siteKeyEnv, testSiteKey)
	out := filepath.Join(t.TempDir(), "site")
	if err := run(runOptions{out: out, cache: t.TempDir(), nkjvText: true, now: fixedNow}); err != nil {
		t.Fatalf("run: %v", err)
	}
	return out, append(published, licensedFixtureVersion(licensed))
}

// pill is one translation link as a chapter page was built with it.
type pill struct {
	href string
	vmap *string // nil when the pill carries no data-vmap
}

var pillRE = regexp.MustCompile(`<a class="vpick( on)?" title="[^"]*"(?: data-vmap="([^"]*)")? href="([^"]*)">([A-Z]+)</a>`)

// pillsOf reads the switcher of one chapter page, by translation id.
func pillsOf(t *testing.T, root, rel string) map[string]pill {
	t.Helper()
	page := readSiteFile(t, root, rel)
	out := map[string]pill{}
	for _, m := range pillRE.FindAllStringSubmatch(page, -1) {
		p := pill{href: m[3]}
		if strings.Contains(m[0], ` data-vmap="`) {
			s := m[2]
			p.vmap = &s
		}
		out[strings.ToLower(m[4])] = p
	}
	if len(out) == 0 {
		t.Fatalf("/%s has no switcher", rel)
	}
	return out
}

// carryCase is one fragment carried by one pill.
type carryCase struct {
	Name string  `json:"-"`
	Base string  `json:"base"`
	VMap *string `json:"vmap"`
	V    string  `json:"v"`
	N    string  `json:"n"`
	Want string  `json:"-"`
}

// carrySection is the shipped carry code: the bytes between reader.js's
// VERSE_CARRY markers.
func carrySection(t *testing.T) string {
	t.Helper()
	i := strings.Index(readerJSTemplate, carryBegin)
	j := strings.Index(readerJSTemplate, carryEnd)
	if i < 0 || j < 0 || j < i {
		t.Fatal("reader.js has lost its VERSE_CARRY markers")
	}
	return readerJSTemplate[i : j+len(carryEnd)]
}

// runJS runs body, which leaves its answer as a JSON string in out, and
// decodes that answer into into.
func runJS(t *testing.T, body string, into any) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "carry.js")
	var cmd *exec.Cmd
	if node, lookErr := exec.LookPath("node"); lookErr == nil {
		body += "\nconsole.log(JSON.stringify(out));\n"
		cmd = exec.Command(node, script)
	} else if osa, lookErr := exec.LookPath("osascript"); lookErr == nil {
		body += "\nJSON.stringify(out);\n"
		cmd = exec.Command(osa, "-l", "JavaScript", script)
	} else {
		t.Skip("no JavaScript runtime on PATH (node or osascript); the carry code was not run")
	}
	if err := os.WriteFile(script, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the carry code: %v\n%s", err, raw)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(raw))), into); err != nil {
		t.Fatalf("the carry code returned %q (%v)", raw, err)
	}
}

// runCarry runs the shipped carry code over the cases and returns what each
// pill's href became.
func runCarry(t *testing.T, cases []carryCase) []string {
	t.Helper()
	input, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	runJS(t, carrySection(t)+"\nvar cases = "+string(input)+";\n"+
		"var out = cases.map(function (c) { return carriedHref(c.base, c.vmap, { v: c.v, n: c.n }); });", &got)
	if len(got) != len(cases) {
		t.Fatalf("the carry code answered %d cases of %d", len(got), len(cases))
	}
	return got
}

// checkCarry runs cases and reports every href that is not the one wanted.
func checkCarry(t *testing.T, cases []carryCase) {
	t.Helper()
	got := runCarry(t, cases)
	for k, c := range cases {
		if got[k] != c.Want {
			t.Errorf("%s: carried to %q, want %q", c.Name, got[k], c.Want)
		}
	}
}

// casesOn builds the cases for one pill of one page: each is a fragment and
// the href it should leave on the pill.
func casesOn(t *testing.T, root, page, to string, frags ...[3]string) []carryCase {
	t.Helper()
	p, ok := pillsOf(t, root, page)[to]
	if !ok {
		t.Fatalf("/%s has no %s pill", page, to)
	}
	var out []carryCase
	for _, f := range frags {
		out = append(out, carryCase{
			Name: fmt.Sprintf("/%s #v%s&n=%s -> %s", page, f[0], f[1], to),
			Base: p.href, VMap: p.vmap, V: f[0], N: f[1], Want: f[2],
		})
	}
	return out
}

// The cases every build must carry, whichever state the NKJV is in.
func publicDomainCarryCases(t *testing.T, root string) []carryCase {
	var cs []carryCase
	add := func(more []carryCase) { cs = append(cs, more...) }
	// The doxology: WEB Romans 14:24-26 is the BSB's 16:25-27.
	add(casesOn(t, root, "web/romans/14/index.html", "bsb",
		[3]string{"24-26", "", "../../../bsb/romans/16/#v25-27"},
		[3]string{"25", "", "../../../bsb/romans/16/#v26"},
		[3]string{"10", "", "../../../bsb/romans/14/#v10"},
		[3]string{"23-24", "", "../../../bsb/romans/14/"},
		[3]string{"23-24", "NOTE", "../../../bsb/romans/14/#n=NOTE"},
		[3]string{"24", "NOTE", "../../../bsb/romans/16/#v25&n=NOTE"},
		[3]string{"", "", "../../../bsb/romans/14/"},
	))
	add(casesOn(t, root, "web/romans/14/index.html", "webc",
		[3]string{"24-26", "", "../../../webc/romans/14/#v24-26"},
	))
	add(casesOn(t, root, "bsb/romans/16/index.html", "web",
		[3]string{"25-27", "", "../../../web/romans/14/#v24-26"},
		[3]string{"1-3", "", "../../../web/romans/16/#v1-3"},
		[3]string{"23-25", "", "../../../web/romans/16/"},
	))
	add(casesOn(t, root, "bsb/romans/16/index.html", "webc",
		[3]string{"25-27", "", "../../../webc/romans/14/#v24-26"},
	))
	// WEB Romans 16:24 is not in the BSB at all.
	add(casesOn(t, root, "web/romans/16/index.html", "bsb",
		[3]string{"24", "", "../../../bsb/romans/16/"},
		[3]string{"23-24", "", "../../../bsb/romans/16/#v23"},
		[3]string{"22-24", "", "../../../bsb/romans/16/#v22-23"},
	))
	// The Song of the Three: WEBC Daniel 3:91-97 is the WEB's 3:24-30, and
	// WEBC 3:24-90 is in neither the WEB nor the BSB.
	add(casesOn(t, root, "webc/daniel/3/index.html", "web",
		[3]string{"91-97", "", "../../../web/daniel/3/#v24-30"},
		[3]string{"50", "", "../../../web/daniel/3/"},
		[3]string{"23-91", "", "../../../web/daniel/3/#v23-24"},
		[3]string{"10", "", "../../../web/daniel/3/#v10"},
	))
	add(casesOn(t, root, "webc/daniel/3/index.html", "bsb",
		[3]string{"91-97", "", "../../../bsb/daniel/3/#v24-30"},
	))
	add(casesOn(t, root, "web/daniel/3/index.html", "webc",
		[3]string{"24-30", "", "../../../webc/daniel/3/#v91-97"},
		[3]string{"23-24", "", "../../../webc/daniel/3/"},
		[3]string{"1-23", "", "../../../webc/daniel/3/#v1-23"},
	))
	add(casesOn(t, root, "web/daniel/3/index.html", "bsb",
		[3]string{"24-30", "", "../../../bsb/daniel/3/#v24-30"},
	))
	// The BSB omits Mark 9:44 and 9:46.
	add(casesOn(t, root, "web/mark/9/index.html", "bsb",
		[3]string{"43-48", "", "../../../bsb/mark/9/#v43-48"},
		[3]string{"44", "", "../../../bsb/mark/9/"},
		[3]string{"44-45", "", "../../../bsb/mark/9/#v45"},
	))
	add(casesOn(t, root, "bsb/mark/9/index.html", "web",
		[3]string{"43-45", "", "../../../web/mark/9/#v43-45"},
	))
	// The Greek Esther: no verse corresponds, and the note still travels.
	add(casesOn(t, root, "webc/esther/1/index.html", "web",
		[3]string{"3", "NOTE", "../../../web/esther/1/#n=NOTE"},
		[3]string{"3", "", "../../../web/esther/1/"},
	))
	add(casesOn(t, root, "web/esther/1/index.html", "webc",
		[3]string{"3", "", "../../../webc/esther/1/"},
	))
	// A chapter every edition numbers alike.
	add(casesOn(t, root, "web/john/3/index.html", "bsb",
		[3]string{"16", "NOTE", "../../../bsb/john/3/#v16&n=NOTE"},
	))
	return cs
}

func TestSwitcherCarriesThePassageWithTheNKJVOff(t *testing.T) {
	root, _ := switcherSiteOff(t)
	checkCarry(t, publicDomainCarryCases(t, root))
}

func TestSwitcherCarriesThePassageWithTheNKJVOn(t *testing.T) {
	root, _ := switcherSiteOn(t)
	cs := publicDomainCarryCases(t, root)
	cs = append(cs, casesOn(t, root, "web/romans/14/index.html", "nkjv",
		[3]string{"24-26", "", "../../../nkjv/romans/16/#v25-27"},
		[3]string{"20-23", "", "../../../nkjv/romans/14/#v20-23"},
	)...)
	cs = append(cs, casesOn(t, root, "nkjv/romans/16/index.html", "web",
		[3]string{"25-27", "", "../../../web/romans/14/#v24-26"},
		[3]string{"24", "", "../../../web/romans/16/#v24"},
	)...)
	cs = append(cs, casesOn(t, root, "nkjv/romans/16/index.html", "bsb",
		[3]string{"25-27", "", "../../../bsb/romans/16/#v25-27"},
		[3]string{"24", "", "../../../bsb/romans/16/"},
	)...)
	cs = append(cs, casesOn(t, root, "nkjv/romans/16/index.html", "webc",
		[3]string{"25-27", "", "../../../webc/romans/14/#v24-26"},
	)...)
	cs = append(cs, casesOn(t, root, "bsb/romans/16/index.html", "nkjv",
		[3]string{"25-27", "", "../../../nkjv/romans/16/#v25-27"},
	)...)
	// The NKJV has Acts 8:37; the WEB does not.
	cs = append(cs, casesOn(t, root, "nkjv/acts/8/index.html", "web",
		[3]string{"37", "", "../../../web/acts/8/"},
		[3]string{"36-38", "", "../../../web/acts/8/#v36-38"},
	)...)
	cs = append(cs, casesOn(t, root, "web/acts/8/index.html", "nkjv",
		[3]string{"36-38", "", "../../../nkjv/acts/8/#v36-38"},
	)...)
	cs = append(cs, casesOn(t, root, "webc/daniel/3/index.html", "nkjv",
		[3]string{"91-97", "", "../../../nkjv/daniel/3/#v24-30"},
	)...)
	checkCarry(t, cs)
}

// The carry code on maps the fixture cannot build from today's tables: a pair
// one translation numbers the other way about — Matthew 23:13-14 between the
// WEB and the NKJV, the BSB's Philippians 1:16-17, once the tables record
// them — and the edges of the map's own grammar.
func TestSwitcherCarryCodeOnSwappedAndHostileMaps(t *testing.T) {
	str := func(s string) *string { return &s }
	swap := str("12:23.12.11 13:23.14.13 14:23.13.12 15:23.15.14")
	base := "../../../nkjv/matthew/23/"
	checkCarry(t, []carryCase{
		{Name: "a swapped pair carries as the same pair", Base: base, VMap: swap, V: "13-14", Want: base + "#v13-14"},
		{Name: "one verse of a swapped pair lands on its partner", Base: base, VMap: swap, V: "13", Want: base + "#v14"},
		{Name: "the other verse too", Base: base, VMap: swap, V: "14", Want: base + "#v13"},
		{Name: "a range that would take in a verse not sent", Base: base, VMap: swap, V: "12-13", Want: base},
		{Name: "a range across the whole pair", Base: base, VMap: swap, V: "12-15", Want: base + "#v12-15"},
		{Name: "a vast range reads the map, not the numbers", Base: base, VMap: swap, V: "1-999999999", Want: base + "#v12-15"},
		{Name: "a token the grammar does not know", Base: base, VMap: str("12:23.12.11 x"), V: "12", Want: base},
		{Name: "an empty map carries no verse", Base: base, VMap: str(""), V: "12", N: "NOTE", Want: base + "#n=NOTE"},
		{Name: "no map carries the fragment as it is", Base: base, VMap: nil, V: "16-18", N: "NOTE", Want: base + "#v16-18&n=NOTE"},
		{Name: "no verse at all", Base: base, VMap: swap, V: "", N: "NOTE", Want: base + "#n=NOTE"},
		{Name: "a malformed verse", Base: base, VMap: nil, V: "16-12", Want: base},
	})
}

// carryVerse rewrites every pill from the link the page was built with, so a
// verse carried into another chapter does not move the next one there too.
// Run over a stand-in page holding the real WEB Romans 14 pills, through a
// run of fragments in the order a reader's taps would give them.
func TestSwitcherRewritesFromEachPillsOwnLink(t *testing.T) {
	root, _ := switcherSiteOff(t)
	pills := pillsOf(t, root, "web/romans/14/index.html")
	type stub struct {
		Href string  `json:"href"`
		VMap *string `json:"vmap"`
	}
	page, err := json.Marshal([]stub{{pills["bsb"].href, pills["bsb"].vmap}, {pills["webc"].href, pills["webc"].vmap}})
	if err != nil {
		t.Fatal(err)
	}
	if pills["bsb"].vmap == nil || pills["webc"].vmap != nil {
		t.Fatal("the fixture's WEB Romans 14 must map its BSB pill and not its WEBC one")
	}
	body := carrySection(t) + `
var hash = '';
function fragKeys() {
  var out = { v: '' };
  hash.split('&').forEach(function (kv, i) {
    var eq = kv.indexOf('=');
    if (eq < 0) { if (i === 0 && /^v\d/.test(kv)) out.v = kv.slice(1); return; }
    out[kv.slice(0, eq)] = kv.slice(eq + 1);
  });
  return out;
}
var pills = ` + string(page) + `.map(function (p) {
  var attrs = { href: p.href };
  if (p.vmap !== null) attrs['data-vmap'] = p.vmap;
  return {
    getAttribute: function (k) { return k in attrs ? attrs[k] : null; },
    setAttribute: function (k, v) { attrs[k] = v; },
    href: function () { return attrs.href; }
  };
});
var document = { querySelectorAll: function () { return pills; } };
var out = ['v24-26', 'v10', 'v25&n=NOTE', ''].map(function (h) {
  hash = h;
  carryVerse();
  return pills.map(function (p) { return p.href(); });
});`
	var got [][]string
	runJS(t, body, &got)
	want := [][]string{
		{"../../../bsb/romans/16/#v25-27", "../../../webc/romans/14/#v24-26"},
		{"../../../bsb/romans/14/#v10", "../../../webc/romans/14/#v10"},
		{"../../../bsb/romans/16/#v26&n=NOTE", "../../../webc/romans/14/#v25&n=NOTE"},
		{"../../../bsb/romans/14/", "../../../webc/romans/14/"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the pills after each fragment:\n got  %v\n want %v", got, want)
	}
}

// Every pill of every chapter page, in both states, carries a map exactly
// when the notice pages' own question says the chapter's numbering differs,
// and the map is the versification tables' answer verse for verse. Where no
// map is written, every verse of the page lands under its own number in a
// chapter of the other translation that has it, so the fragment may travel
// as it is.
func TestSwitcherVerseMapIsTheVersificationTables(t *testing.T) {
	for _, state := range []struct {
		name  string
		build func(*testing.T) (string, []loadedVersion)
	}{{"off", switcherSiteOff}, {"on", switcherSiteOn}} {
		t.Run(state.name, func(t *testing.T) {
			root, all := state.build(t)
			byID := map[string]loadedVersion{}
			for _, v := range all {
				byID[v.ID] = v
			}
			maps, plain := 0, 0
			for _, v := range all {
				for _, book := range v.bible.Books {
					slug, _ := bibletext.BookSlug(book)
					for _, ch := range sortedChapters(v.bible, book) {
						rel := fmt.Sprintf("%s/%s/%d/index.html", v.ID, slug, ch)
						for id, p := range pillsOf(t, root, rel) {
							if id == v.ID {
								if p.vmap != nil {
									t.Errorf("/%s: its own pill carries a map", rel)
								}
								continue
							}
							other := byID[id]
							differs := numberingDiff(v.ID, id, book, ch, all) != bibletext.NumberingSame
							if differs != (p.vmap != nil) {
								t.Errorf("/%s -> %s: numbering differs %v, map written %v", rel, id, differs, p.vmap != nil)
								continue
							}
							if !differs {
								plain++
								for _, x := range v.bible.Verses[book][ch] {
									mc, mv, _ := bibletext.MapVerse(v.ID, id, book, ch, x.Verse)
									if mc != ch || mv != x.Verse || !hasVerse(other, book, ch, x.Verse) {
										t.Errorf("/%s -> %s: no map, but verse %d lands at %d:%d", rel, id, x.Verse, mc, mv)
									}
								}
								continue
							}
							maps++
							if want := tableMap(v, other, book, ch); *p.vmap != want {
								t.Errorf("/%s -> %s: map\n %q\nwant the tables'\n %q", rel, id, *p.vmap, want)
							}
						}
					}
				}
			}
			if maps == 0 || plain == 0 {
				t.Fatalf("the fixture reached %d mapped and %d plain pills; it must reach both", maps, plain)
			}
		})
	}
}

// tableMap is the map the tables give, worked out here from MapVerse and the
// other translation's own verses.
func tableMap(v, other loadedVersion, book string, ch int) string {
	var toks []string
	for _, x := range v.bible.Verses[book][ch] {
		mc, mv, _ := bibletext.MapVerse(v.ID, other.ID, book, ch, x.Verse)
		if mc == 0 || !hasVerse(other, book, mc, mv) {
			continue
		}
		place := 0
		for _, y := range other.bible.Verses[book][mc] {
			if y.Verse < mv {
				place++
			}
		}
		toks = append(toks, strconv.Itoa(x.Verse)+":"+strconv.Itoa(mc)+"."+strconv.Itoa(mv)+"."+strconv.Itoa(place))
	}
	return strings.Join(toks, " ")
}

func hasVerse(v loadedVersion, book string, ch, n int) bool {
	for _, x := range v.bible.Verses[book][ch] {
		if x.Verse == n {
			return true
		}
	}
	return false
}
