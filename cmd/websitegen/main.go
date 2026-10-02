// Command websitegen builds the static web reader published at
// bibletext.co.uk — the destination of the app's "Share as link".
//
// WHY A GO GENERATOR AND NOT A WEB FRAMEWORK. The scripture, the poem-line
// rule, the red-letter ranges and the book names all live in this repo's Go
// package. Any external site generator would need a Go export step anyway and
// would then re-implement those rendering rules in a second template language —
// two definitions of "how a psalm breaks", guaranteed to drift. This program is
// a third entry point beside cmd/bibletext and cmd/mobile, importing the same
// package, so the web page and the app render from one source of truth. It has
// no dependencies beyond the standard library.
//
// WHAT IT EMITS (every path is part of the frozen contract — see share_link.go):
//
//	<version>/index.html                 book list        e.g. /web/
//	<version>/<book>/index.html          chapter list     e.g. /web/john/
//	<version>/<book>/<ch>/index.html     the chapter      e.g. /web/john/3/
//	assets/reader.css                    one stylesheet
//	assets/reader.js                     two small progressive enhancements
//	404.html                             site-wide, at the root (see writeNotFound)
//
// AND, at exactly the same paths, the pages that name a passage this site does
// not carry the words of (notice.go) — the canon gaps inside the published
// translations (/web/tobit/1/, /web/daniel/13/), and, while the NKJV's text is
// switched off, the licensed translation the app links to (/nkjv/…). They are
// the same shape and the same URL contract as everything above; they simply
// say where the passage can be read instead. Their own stylesheet and script:
//
//	assets/notice.css                    the extra rules those pages need
//	assets/notice.js                     the fragment-aware parallel links
//
// THE NKJV IS ONE SWITCH (nkjv_text.go). Off, /nkjv/ is the notice tree above
// and nothing here talks to API.Bible. On, every build fetches the whole NKJV
// afresh and writes /nkjv/ exactly as it writes the public-domain editions,
// with the rights holder's notice and the retrieval date at the foot of every
// page, plus the supplementary faces and rules those pages load:
//
//	assets/nkjv.css                      small capitals, italic, the notice
//
// The reader lives at the ROOT, not under a /read/ prefix, so a shared link is
// as short as possible. The site root is therefore shared with the hand-written
// landing/privacy/support pages: reservedRootNames below makes it impossible for
// the generator to overwrite one of them, and the three version ids are reserved
// at the root forever.
//
// A single verse link (#v16) needs NO JavaScript: each verse carries an id and
// CSS :target draws the highlight. Everything else on the page is reader.js:
// verse RANGES (#v16-18 — :target can only match one id, and no element is ever
// given the id "v16-18"), the sender's shared NOTE (decoded from the fragment
// and rendered entirely client-side — the HTML carries no note markup at all),
// carrying the fragment across a translation switch, the "Go to" picker upgrade,
// the platform-aware "Get the app" link, and the note's hide/delete controls.
// With scripting off the passage still opens and a single verse still
// highlights; a note-bearing link renders as a bare chapter with no sign a
// message was attached.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	bibletext "github.com/cubancorona/bibletext"
)

// webVersion is one published translation.
type webVersion struct {
	ID   string // URL segment and the app's version id — never changes
	Name string // full display name, used in citations and OG titles
	URL  string // helloao complete.json
	// decode turns the downloaded body into the app's BibleData using the very
	// same decoder the app uses, so the site can never disagree with the app.
	decode func([]byte) (*bibletext.BibleData, error)
	// headings says the edition's feed carries the publisher's section
	// headings, so a decode that yields none is a stale or broken feed, not a
	// plain edition. The live site published the Catholic edition without a
	// single heading for weeks on the strength of a feed cached in August.
	headings bool
}

// cacheFileName keys the raw feed by the app's decoder epoch for the edition,
// so the site never decodes a feed older than the app's own understanding of
// it: the cache used to be keyed by id alone and was downloaded ONCE, on 9
// August, and reused for every publish after.
func cacheFileName(v webVersion) string {
	return fmt.Sprintf("%s-v%d.json", v.ID, bibletext.VersionCacheEpoch(v.ID))
}

// headingCount is the publisher's headings an edition decoded to.
func headingCount(bd *bibletext.BibleData) int {
	n := 0
	for _, chapters := range bd.Headings {
		for _, hs := range chapters {
			n += len(hs)
		}
	}
	return n
}

// checkDecoded refuses a decode that cannot be what the app shows.
func checkDecoded(v webVersion, bd *bibletext.BibleData) error {
	if len(bd.Books) == 0 {
		return fmt.Errorf("%s: decoded no books", v.ID)
	}
	if v.headings && headingCount(bd) == 0 {
		return fmt.Errorf("%s: decoded no publisher headings; the cached feed predates them — "+
			"delete build/biblecache/%s and rebuild", v.ID, cacheFileName(v))
	}
	return nil
}

func publishedVersions() []webVersion {
	return []webVersion{
		{ID: "web", Name: "World English Bible",
			URL: "https://bible.helloao.org/api/ENGWEBP/complete.json", decode: bibletext.DecodeCanonical66},
		{ID: "bsb", Name: "Berean Standard Bible",
			URL: "https://bible.helloao.org/api/BSB/complete.json", decode: bibletext.DecodeCanonical66, headings: true},
		{ID: "webc", Name: "World English Bible (Catholic)",
			URL: "https://bible.helloao.org/api/eng_webc/complete.json", decode: bibletext.DecodeHelloAOCatholic},
	}
}

const defaultVersionID = "web"

// noticeVersion is a licensed translation as the site names it: an id and a
// display name, and nothing that could hold scripture.
type noticeVersion struct {
	ID   string
	Name string
}

// The licensed translations live in nkjv_text.go (siteLicensedVersions), which
// says whether each is published as text or as notice pages.
//
// THEY ARE NOT publishedVersions AND MUST NOT BECOME IT. publishedVersions
// carries a decoder and a URL to fetch scripture from, with no key and no
// terms; a licensed translation's text reaches the site only through
// loadLicensed, which only the switch reaches, and which fetches it fresh with
// a key on every build. licensed_exclusion_test.go keeps the two apart.
//
// The notice tree exists because /nkjv/john/3/ was a live 404 while the app was
// emitting exactly that URL: the link was dead for every recipient without the
// app, and its preview was a bare URL in every message thread (B_WEB_404 and
// B_UNFURL_NKJV in docs/NKJV_FLOW.md). With the switch off every NKJV path the
// site serves carries a licensing message and a link to download the app; with
// it on, the same paths carry the text.

// noticeCanonSourceID names the loaded translation whose book and chapter list
// stands in for a noticed translation's own.
//
// WHY A STAND-IN AT ALL. With the NKJV's text off the site holds no NKJV data —
// that is the point — so the generator cannot ask it how many chapters Jude
// has. What it can rely on is that the NKJV is the ordinary 66-book Protestant
// canon, chapter for chapter, with the WEB: 66 books, 1,189 chapters, and
// versification.go's delta for the NKJV records not a single chapter-count
// difference (only four extra verses and the Romans doxology's move). So the
// WEB's shape is not an approximation of the NKJV's canon; it is the same
// canon. (With the text on, checkLicensedComplete holds the fetched edition to
// exactly that shape before a page is written.)
//
// If that ever stops being true the failure is a 404 on a chapter the app can
// link to, which is why the count is asserted in the tests and pinned by an
// equality guard in scripts/publish-site.sh rather than left to this comment.
const noticeCanonSourceID = defaultVersionID

func main() {
	out := flag.String("out", "build/site", "directory to write the site into")
	cache := flag.String("cache", "build/biblecache", "directory for downloaded translation JSON")
	offline := flag.Bool("offline", false, "fail rather than download; use only the cache")
	printState := flag.Bool("print-nkjv-text", false,
		"print whether this build publishes the NKJV's text (on or off) and exit")
	flag.Parse()

	// The switch is read here, once, and handed down. Nothing else in this
	// program consults it, and nothing can override it (nkjv_text.go).
	state := nkjvSiteText
	if *printState {
		fmt.Println(nkjvTextState(state))
		return
	}
	if err := run(runOptions{out: *out, cache: *cache, offline: *offline, nkjvText: state, now: time.Now}); err != nil {
		log.Fatal(err)
	}
}

// runOptions is one build: where it writes, where the public-domain feeds are
// cached, whether it may download, the state of the NKJV switch, and the clock
// the retrieval date is read from.
type runOptions struct {
	out, cache string
	offline    bool
	nkjvText   bool
	now        func() time.Time
}

// run builds the site. Everything that can refuse does so before the output
// directory is touched: a switched-on build without a key, or whose fetch
// fails or comes back incomplete, leaves the output directory as it was. A
// write refused part-way — the writer found the key in a page, say — removes
// the whole output directory rather than leave half a site, which with the
// NKJV's text on would be licensed text that no finished build stands behind.
func run(o runOptions) error {
	start := time.Now()
	if o.nkjvText && o.offline {
		return errors.New("the NKJV's text is switched on (cmd/websitegen/nkjv_text.go) and is fetched fresh " +
			"on every build, so -offline cannot build this site")
	}
	// The key first, before any download, so a build that cannot finish says so
	// at once — and out of the environment before anything else runs.
	var key string
	if o.nkjvText {
		k, err := takeSiteKey()
		if err != nil {
			return err
		}
		key = k
	}
	if err := os.MkdirAll(o.cache, 0o755); err != nil {
		return fmt.Errorf("cache dir: %w", err)
	}
	loaded, err := loadPublished(o.cache, o.offline)
	if err != nil {
		return err
	}
	var ref *bibletext.BibleData
	for _, v := range loaded {
		if v.ID == versificationReferenceID {
			ref = v.bible
		}
	}
	licensed, err := loadLicensed(o.nkjvText, o.offline, key, ref, o.now)
	if err != nil {
		return err
	}
	if !o.nkjvText {
		log.Printf("nkjv  text off: notice pages, no request to API.Bible")
	}

	site := &siteWriter{root: o.out, secret: key}
	if err := writeSite(site, append(loaded, licensed...), noticedVersionsFor(o.nkjvText)); err != nil {
		if rmErr := os.RemoveAll(o.out); rmErr != nil {
			return fmt.Errorf("write: %w (and the part written could not be removed: %v)", err, rmErr)
		}
		return fmt.Errorf("write: %w", err)
	}
	log.Printf("wrote %d files to %s in %s", site.files, o.out, time.Since(start).Round(time.Millisecond))
	return nil
}

// loadPublished downloads (or reads from the cache) and decodes the
// public-domain editions. A variable so the tests can stand in for the
// network; nothing else assigns it.
var loadPublished = loadPublishedVersions

func loadPublishedVersions(cache string, offline bool) ([]loadedVersion, error) {
	var loaded []loadedVersion
	for _, v := range publishedVersions() {
		body, err := fetchWithCache(v, cache, offline)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", v.ID, err)
		}
		bible, err := v.decode(body)
		if err != nil {
			return nil, fmt.Errorf("%s: decode: %w", v.ID, err)
		}
		if err := checkDecoded(v, bible); err != nil {
			return nil, err
		}
		loaded = append(loaded, loadedVersion{webVersion: v, bible: bible})
		log.Printf("%-5s %d books, %d headings", v.ID, len(bible.Books), headingCount(bible))
	}
	return loaded, nil
}

// cssName/jsName are the content-hashed asset paths for this build; pageShell
// links them. Set once in writeSite before any page is rendered.
var cssName, jsName string

func contentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:10]
}

type loadedVersion struct {
	webVersion
	bible *bibletext.BibleData
	// licence is set for a licensed edition published as text (nkjv_text.go),
	// and nil for the public-domain editions — whose pages are therefore
	// exactly what they were before licensed text could be published.
	licence *webLicence
}

// fetchWithCache downloads a translation once and reuses it thereafter, so
// rebuilding the site is offline and instant. The cache is build output, not
// source: delete it and the next run re-downloads.
func fetchWithCache(v webVersion, cacheDir string, offline bool) ([]byte, error) {
	path := filepath.Join(cacheDir, cacheFileName(v))
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b, nil
	}
	if offline {
		return nil, fmt.Errorf("no cached copy at %s and -offline was set", path)
	}
	log.Printf("%-5s downloading %s", v.ID, v.URL)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(v.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", v.URL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return nil, err
	}
	return body, nil
}

// siteWriter writes files under a root, counting them so the publish script can
// sanity-check the output size.
type siteWriter struct {
	root  string
	files int
	// secret is the API.Bible key while the NKJV's text is being published,
	// and "" otherwise. No file the site writes may contain it.
	secret string
}

// reservedRootNames are files at the site root that the HAND-WRITTEN site owns.
// The reader now lives at the root (no /read/ prefix), so the generator shares
// that namespace with the landing, privacy and support pages — and those three
// are also what the App Store's privacy and support URLs point at. Nothing here
// should ever try to write them, but "should never" is not a guarantee: this
// makes it impossible, loudly, at build time rather than at publish time.
var reservedRootNames = map[string]bool{
	"index.html": true, "privacy.html": true, "support.html": true, "CNAME": true,
}

func (s *siteWriter) write(relPath, content string) error {
	if reservedRootNames[relPath] {
		return fmt.Errorf("refusing to write %q: the hand-written site owns that file at the root", relPath)
	}
	if s.secret != "" && strings.Contains(content, s.secret) {
		return fmt.Errorf("refusing to write %q: it contains the API.Bible key", relPath)
	}
	full := filepath.Join(s.root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return err
	}
	s.files++
	return nil
}

// writeSite writes the whole tree: the published editions in versions (the
// public-domain three, and any licensed edition the switch loaded), and a
// notice tree for each of noticed.
func writeSite(site *siteWriter, versions []loadedVersion, noticed []noticeVersion) error {
	// One root, one tree: a translation published as text cannot also be
	// written as notices over it, whichever list a caller got wrong.
	for _, nv := range noticed {
		for _, v := range versions {
			if v.ID == nv.ID {
				return fmt.Errorf("%s is both published and noticed; the switch gives each root one tree", nv.ID)
			}
		}
	}
	if err := os.RemoveAll(site.root); err != nil {
		return err
	}
	// Assets and the site-wide 404 first, so a partial build still has chrome.
	// CACHE BUSTING. GitHub Pages serves assets with max-age=600 and we cannot
	// set headers, so a returning reader kept getting NEW html with an OLD
	// stylesheet — which renders as unstyled controls and giant icons, and is
	// impossible to diagnose from a screenshot. Content-hashed filenames make a
	// changed asset a different URL, so that can never happen again.
	// The chrome typeface, straight from the app's embedded copy, plus the OFL
	// text the licence requires to travel with it. Written FIRST because the
	// stylesheet's @font-face src carries their hashed names.
	regular := bibletext.WebUIFontRegular()
	bold := bibletext.WebUIFontBold()
	regularFile := "AtkinsonHyperlegible-Regular." + contentHash(string(regular)) + ".woff2"
	boldFile := "AtkinsonHyperlegible-Bold." + contentHash(string(bold)) + ".woff2"
	if err := site.write("assets/"+regularFile, string(regular)); err != nil {
		return err
	}
	if err := site.write("assets/"+boldFile, string(bold)); err != nil {
		return err
	}
	if err := site.write("assets/atkinson-OFL.txt", string(bibletext.WebUIFontLicense())); err != nil {
		return err
	}

	// The reading face, the same way and for the same reasons. Scripture is set
	// in it here exactly as it is in the app, so a shared link shows a reader
	// the page they already know.
	scrip := bibletext.WebScriptureFontRegular()
	scripBold := bibletext.WebScriptureFontBold()
	fonts := webFonts{
		uiRegular:        regularFile,
		uiBold:           boldFile,
		scriptureRegular: "Junicode-Regular." + contentHash(string(scrip)) + ".woff2",
		scriptureBold:    "Junicode-Bold." + contentHash(string(scripBold)) + ".woff2",
	}
	if err := site.write("assets/"+fonts.scriptureRegular, string(scrip)); err != nil {
		return err
	}
	if err := site.write("assets/"+fonts.scriptureBold, string(scripBold)); err != nil {
		return err
	}
	if err := site.write("assets/junicode-OFL.txt", string(bibletext.WebScriptureFontLicense())); err != nil {
		return err
	}

	js := readerJS(versions)
	css := readerCSS(fonts)
	cssName = "assets/reader." + contentHash(css) + ".css"
	jsName = "assets/reader." + contentHash(js) + ".js"
	if err := site.write(cssName, css); err != nil {
		return err
	}
	if err := site.write(jsName, js); err != nil {
		return err
	}
	// The notice pages' own pair. Written unconditionally so the tree always has
	// them, and hashed like everything else. No published page links them.
	noticeCSSName = "assets/notice." + contentHash(noticeCSS) + ".css"
	noticeJSName = "assets/notice." + contentHash(noticeJS) + ".js"
	if err := site.write(noticeCSSName, noticeCSS); err != nil {
		return err
	}
	if err := site.write(noticeJSName, noticeJS); err != nil {
		return err
	}
	// The licensed text's own stylesheet and its faces, only when a licensed
	// edition is loaded: with the NKJV's text off the tree gains no file at
	// all.
	nkjvCSSName = ""
	if anyLicensed(versions) {
		var nf nkjvFonts
		for _, face := range []struct {
			name string
			data []byte
			file *string
		}{
			{"Junicode-SmallCaps", bibletext.WebScriptureFontSmallCaps(), &nf.smallCaps},
			{"Junicode-BoldSmallCaps", bibletext.WebScriptureFontBoldSmallCaps(), &nf.boldSmallCaps},
			{"Junicode-ItalicSmallCaps", bibletext.WebScriptureFontItalicSmallCaps(), &nf.italicSmallCaps},
			{"Junicode-Italic", bibletext.WebScriptureFontItalic(), &nf.italic},
			{"BibleTextHebrew", bibletext.WebHebrewFont(), &nf.hebrew},
		} {
			*face.file = face.name + "." + contentHash(string(face.data)) + ".woff2"
			if err := site.write("assets/"+*face.file, string(face.data)); err != nil {
				return err
			}
		}
		if err := site.write("assets/hebrew-OFL.txt", string(bibletext.WebHebrewFontLicense())); err != nil {
			return err
		}
		css := nkjvCSS(nf)
		nkjvCSSName = "assets/nkjv." + contentHash(css) + ".css"
		if err := site.write(nkjvCSSName, css); err != nil {
			return err
		}
	}
	if err := writeNotFound(site, versions); err != nil {
		return err
	}
	for _, v := range versions {
		if err := writeVersion(site, v, versions); err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
	}
	for _, nv := range noticed {
		if err := writeNoticeVersion(site, nv, versions); err != nil {
			return fmt.Errorf("%s: %w", nv.ID, err)
		}
	}
	return nil
}

// anyLicensed reports whether any loaded edition is licensed text.
func anyLicensed(versions []loadedVersion) bool {
	for _, v := range versions {
		if v.licence != nil {
			return true
		}
	}
	return false
}

// canonUnion is every (book, chapter) ANY published translation carries, which
// is what makes a canon gap detectable: a chapter in the union that this
// version lacks is a page some other version of this site serves and a shared
// link could name.
//
// Books come out in first-seen order across the versions in their published
// order, so the seven deuterocanonical books trail the 66 in WEB Catholic's own
// arrangement. Order matters only for reproducibility — nothing renders from it.
type canonUnion struct {
	books    []string
	chapters map[string][]int
}

func unionCanon(all []loadedVersion) canonUnion {
	u := canonUnion{chapters: map[string][]int{}}
	seenBook := map[string]bool{}
	seenChapter := map[string]map[int]bool{}
	for _, v := range all {
		for _, book := range v.bible.Books {
			if !seenBook[book] {
				seenBook[book] = true
				u.books = append(u.books, book)
				seenChapter[book] = map[int]bool{}
			}
			for _, ch := range sortedChapters(v.bible, book) {
				if !seenChapter[book][ch] {
					seenChapter[book][ch] = true
					u.chapters[book] = append(u.chapters[book], ch)
				}
			}
		}
	}
	for book := range u.chapters {
		sort.Ints(u.chapters[book])
	}
	return u
}

// lastChapter is the highest chapter number in a sorted list, or 0 when empty.
func lastChapter(sorted []int) int {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[len(sorted)-1]
}

// neighbours returns the entries either side of n in a sorted list, or 0.
func neighbours(list []int, n int) (prev, next int) {
	for i, v := range list {
		if v != n {
			continue
		}
		if i > 0 {
			prev = list[i-1]
		}
		if i < len(list)-1 {
			next = list[i+1]
		}
		return
	}
	return
}

// readerJS builds the one script the whole site shares, injecting the book
// table the "Go to" picker navigates. Generated from the SAME loaded data the
// pages come from, so the picker can never offer a page that was not written —
// and per-version chapter counts keep it honest where canons differ.
//
// The list is emitted in the app picker's own alphabetical order, each book
// carrying the letter it files under (bibletext.AlphabeticalBooks /
// FirstLetter), so the web alphabet grid and the app's agree by construction.
func readerJS(versions []loadedVersion) string {
	type entry struct {
		Name   string         `json:"name"`
		Slug   string         `json:"slug"`
		Letter string         `json:"l"`
		Ch     map[string]int `json:"ch"`
	}
	byName := map[string]*entry{}
	var order []string
	for _, v := range versions {
		for _, book := range v.bible.Books {
			n := len(v.bible.Verses[book])
			if n == 0 {
				continue
			}
			e, seen := byName[book]
			if !seen {
				slug, ok := bibletext.BookSlug(book)
				if !ok {
					continue
				}
				e = &entry{Name: book, Slug: slug, Letter: bibletext.FirstLetter(book), Ch: map[string]int{}}
				byName[book] = e
				order = append(order, book)
			}
			e.Ch[v.ID] = n
		}
	}
	list := make([]*entry, 0, len(order))
	for _, n := range bibletext.AlphabeticalBooks(order) {
		list = append(list, byName[n])
	}
	table, err := json.Marshal(list)
	if err != nil { // unreachable: plain structs
		table = []byte("[]")
	}
	byline, err := json.Marshal(bibletext.WebNoteByline())
	if err != nil { // unreachable: a plain string
		byline = []byte(`"Note from Friend"`)
	}
	pill, err := json.Marshal(bibletext.WebNotePillLabel())
	if err != nil { // unreachable: a plain string
		pill = []byte(`"Note"`)
	}
	return strings.NewReplacer(
		"__BOOKS__", string(table),
		"__NOTE_BYLINE__", string(byline),
		"__NOTE_PILL_LABEL__", string(pill),
	).Replace(readerJSTemplate)
}

// writeVersion emits one published translation: its book list, a chapter list
// and a page per chapter it carries — AND a notice page at every path in the
// site's canon union that it does not.
//
// The gap-filling used to be a `continue`. `if len(chapters) == 0 { continue }`
// skipped the seven deuterocanonical books entirely, so /web/tobit/1/ was a
// bare 404 for the 137 chapters WEB Catholic serves next door
// (B_DEUTERO_WEB_404), and /web/daniel/13/ was one for the two chapters the
// Greek Daniel adds — a gap nobody had noticed, because it hides inside a book
// the translation does have.
//
// NOTE THE ASYMMETRY IN THE ARROWS, which is deliberate. A real chapter page's
// prev/next still run over the version's OWN chapters, so /web/daniel/12/ ends
// the book exactly as it always did; only the notice pages navigate the union.
// The alternative — extending the real pages' arrows into the gap — would have
// rewritten pages this change is required to leave byte-identical, and would
// also have claimed the WEB's Daniel runs to 14, which it does not.
func writeVersion(site *siteWriter, v loadedVersion, all []loadedVersion) error {
	if err := site.write(v.ID+"/index.html", renderBookList(v, all)); err != nil {
		return err
	}
	union := unionCanon(all)
	for _, book := range union.books {
		slug, ok := bibletext.BookSlug(book)
		if !ok {
			return fmt.Errorf("book %q has no slug — add it to bookslugs.go", book)
		}
		base := v.ID + "/" + slug
		chapters := sortedChapters(v.bible, book)

		if len(chapters) > 0 {
			if err := site.write(base+"/index.html", renderChapterList(v, book, slug, chapters)); err != nil {
				return err
			}
			for i, ch := range chapters {
				var prev, next int
				if i > 0 {
					prev = chapters[i-1]
				}
				if i < len(chapters)-1 {
					next = chapters[i+1]
				}
				page := renderChapter(v, all, book, slug, ch, prev, next)
				if err := site.write(base+"/"+strconv.Itoa(ch)+"/index.html", page); err != nil {
					return err
				}
			}
		} else {
			// The book is not in this canon at all, so its index says so and
			// points at the translation that has it.
			spec := noticeSpec{
				Reason: reasonAbsent, Scope: scopeBook,
				VersionID: v.ID, VersionName: v.Name, Book: book, Slug: slug,
				Offers:  offersForBook(v.ID, all, book, slug, scopeBook.depth()),
				Licence: v.licence,
			}
			if err := site.write(base+"/index.html", renderNotice(spec)); err != nil {
				return err
			}
		}

		for _, ch := range union.chapters[book] {
			if hasChapter(v, book, ch) {
				continue
			}
			prev, next := neighbours(union.chapters[book], ch)
			spec := noticeSpec{
				Reason: reasonAbsent, Scope: scopeChapter,
				VersionID: v.ID, VersionName: v.Name, Book: book, Slug: slug,
				Chapter: ch, Prev: prev, Next: next, OwnLastChapter: lastChapter(chapters),
				Offers:  offersForChapter(v.ID, all, book, slug, ch, scopeChapter.depth()),
				Licence: v.licence,
			}
			if err := site.write(base+"/"+strconv.Itoa(ch)+"/index.html", renderNotice(spec)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeNoticeVersion emits the whole tree for a translation the site publishes
// no text of: its root, a page per book and a page per chapter, so that EVERY
// URL the app can build for it resolves to something with a route out.
//
// Every chapter, not the popular ones: a shared link is whatever verse somebody
// was reading, and the pages exist precisely for the recipient who does not
// have the app. 1,189 files is the price of that promise.
func writeNoticeVersion(site *siteWriter, nv noticeVersion, all []loadedVersion) error {
	var canon *loadedVersion
	for i := range all {
		if all[i].ID == noticeCanonSourceID {
			canon = &all[i]
		}
	}
	if canon == nil {
		return fmt.Errorf("no %q among the published versions to take a canon from", noticeCanonSourceID)
	}

	root := noticeSpec{
		Reason: reasonLicensed, Scope: scopeVersion,
		VersionID: nv.ID, VersionName: nv.Name,
		Offers: offersForVersion(all, scopeVersion.depth()),
	}
	if err := site.write(nv.ID+"/index.html", renderNotice(root)); err != nil {
		return err
	}

	for _, book := range canon.bible.Books {
		slug, ok := bibletext.BookSlug(book)
		if !ok {
			return fmt.Errorf("book %q has no slug — add it to bookslugs.go", book)
		}
		chapters := sortedChapters(canon.bible, book)
		if len(chapters) == 0 {
			continue
		}
		base := nv.ID + "/" + slug
		bookSpec := noticeSpec{
			Reason: reasonLicensed, Scope: scopeBook,
			VersionID: nv.ID, VersionName: nv.Name, Book: book, Slug: slug,
			Offers: offersForBook(nv.ID, all, book, slug, scopeBook.depth()),
		}
		if err := site.write(base+"/index.html", renderNotice(bookSpec)); err != nil {
			return err
		}
		for i, ch := range chapters {
			var prev, next int
			if i > 0 {
				prev = chapters[i-1]
			}
			if i < len(chapters)-1 {
				next = chapters[i+1]
			}
			spec := noticeSpec{
				Reason: reasonLicensed, Scope: scopeChapter,
				VersionID: nv.ID, VersionName: nv.Name, Book: book, Slug: slug,
				Chapter: ch, Prev: prev, Next: next,
				Offers: offersForChapter(nv.ID, all, book, slug, ch, scopeChapter.depth()),
			}
			if err := site.write(base+"/"+strconv.Itoa(ch)+"/index.html", renderNotice(spec)); err != nil {
				return err
			}
		}
	}
	return nil
}

// offersForVersion is the route out of a version root: the whole-Bible index of
// each translation this site actually serves.
func offersForVersion(all []loadedVersion, upDepth int) []noticeOffer {
	up := strings.Repeat("../", upDepth)
	out := make([]noticeOffer, 0, len(all))
	for _, v := range all {
		out = append(out, noticeOffer{ID: v.ID, Name: v.Name, Href: up + v.ID + "/"})
	}
	return out
}

// offersForBook lists the translations carrying this book, pointing at their
// chapter lists. No verse can be carried to a book index, so these links take
// only the sender's note (parallelSection decides that from the scope).
func offersForBook(from string, all []loadedVersion, book, slug string, upDepth int) []noticeOffer {
	up := strings.Repeat("../", upDepth)
	var out []noticeOffer
	for _, v := range all {
		if v.ID == from || len(v.bible.Verses[book]) == 0 {
			continue
		}
		out = append(out, noticeOffer{ID: v.ID, Name: v.Name, Href: up + v.ID + "/" + slug + "/"})
	}
	return out
}

func sortedChapters(bd *bibletext.BibleData, book string) []int {
	chapters := bd.Verses[book]
	nums := make([]int, 0, len(chapters))
	for n := range chapters {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	return nums
}
