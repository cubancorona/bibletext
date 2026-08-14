// Command websitegen builds the static web reader published at
// bibletext.co.uk — the destination of the app's "Share as link".
//
// WHY A GO GENERATOR AND NOT A WEB FRAMEWORK. The scripture, the poem-line
// rule, the red-letter ranges and the book names all live in this repo's Go
// package. Any external site generator would need a Go export step anyway and
// would then re-implement those rendering rules in a second template language —
// two definitions of "how a psalm breaks", guaranteed to drift. This program is
// a third entry point beside cmd/desktop and cmd/mobile, importing the same
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
// not carry the words of (notice.go) — the licensed translation the app links
// to but the site does not publish (/nkjv/…), and the canon gaps inside the
// published translations (/web/tobit/1/, /web/daniel/13/). They are the same
// shape and the same URL contract as everything above; they simply say where
// the passage can be read instead. Their own stylesheet and script:
//
//	assets/notice.css                    the extra rules those pages need
//	assets/notice.js                     the fragment-aware parallel links
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

	bibletext "bibletext"
)

// webVersion is one published translation.
type webVersion struct {
	ID   string // URL segment and the app's version id — never changes
	Name string // full display name, used in citations and OG titles
	URL  string // helloao complete.json
	// decode turns the downloaded body into the app's BibleData using the very
	// same decoder the app uses, so the site can never disagree with the app.
	decode func([]byte) (*bibletext.BibleData, error)
}

func publishedVersions() []webVersion {
	return []webVersion{
		{ID: "web", Name: "World English Bible",
			URL: "https://bible.helloao.org/api/ENGWEBP/complete.json", decode: bibletext.DecodeCanonical66},
		{ID: "bsb", Name: "Berean Standard Bible",
			URL: "https://bible.helloao.org/api/BSB/complete.json", decode: bibletext.DecodeCanonical66},
		{ID: "webc", Name: "World English Bible (Catholic)",
			URL: "https://bible.helloao.org/api/eng_webc/complete.json", decode: bibletext.DecodeHelloAOCatholic},
	}
}

const defaultVersionID = "web"

// noticeVersion is a translation this site publishes PAGES for and no TEXT of.
type noticeVersion struct {
	ID   string
	Name string
}

// noticedVersions are the translations the app emits share links for and the
// site does not carry the words of. Today that is the NKJV alone.
//
// THIS IS NOT publishedVersions AND MUST NOT BECOME IT. publishedVersions
// carries a decoder and a URL to fetch scripture from; this list carries an id
// and a display name and nothing else, because there is nothing else a notice
// page is allowed to know. Adding an id here publishes a signpost. Adding one
// to publishedVersions publishes a translation, which is a licensing decision

//
// It exists because /nkjv/john/3/ was a live 404 while the app was emitting
// exactly that URL: the link was dead for every recipient without the app, and
// its preview was a bare URL in every message thread (B_WEB_404 and

// "should have a licensing message and link to app download for all NKJV …
// this is temporary to get things stable and clean for now".
func noticedVersions() []noticeVersion {
	return []noticeVersion{{ID: "nkjv", Name: "New King James Version"}}
}

// noticeCanonSourceID names the loaded translation whose book and chapter list
// stands in for a noticed translation's own.
//
// WHY A STAND-IN AT ALL. The site holds no NKJV data — that is the point — so
// the generator cannot ask it how many chapters Jude has. What it can rely on
// is that the NKJV is the ordinary 66-book Protestant canon, chapter for
// chapter, with the WEB: 66 books, 1,189 chapters, and versification.go's delta
// for the NKJV records not a single chapter-count difference (only four extra
// verses and the Romans doxology's move). So the WEB's shape is not an
// approximation of the NKJV's canon; it is the same canon.
//
// If that ever stops being true the failure is a 404 on a chapter the app can
// link to, which is why the count is asserted in the tests and pinned by an
// equality guard in scripts/publish-site.sh rather than left to this comment.
const noticeCanonSourceID = defaultVersionID

func main() {
	out := flag.String("out", "build/site", "directory to write the site into")
	cache := flag.String("cache", "build/biblecache", "directory for downloaded translation JSON")
	offline := flag.Bool("offline", false, "fail rather than download; use only the cache")
	flag.Parse()

	start := time.Now()
	site := &siteWriter{root: *out}
	if err := os.MkdirAll(*cache, 0o755); err != nil {
		log.Fatalf("cache dir: %v", err)
	}

	var loaded []loadedVersion
	for _, v := range publishedVersions() {
		body, err := fetchWithCache(v, *cache, *offline)
		if err != nil {
			log.Fatalf("%s: %v", v.ID, err)
		}
		bible, err := v.decode(body)
		if err != nil {
			log.Fatalf("%s: decode: %v", v.ID, err)
		}
		loaded = append(loaded, loadedVersion{webVersion: v, bible: bible})
		log.Printf("%-5s %d books", v.ID, len(bible.Books))
	}

	if err := writeSite(site, loaded); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote %d files to %s in %s", site.files, *out, time.Since(start).Round(time.Millisecond))
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
}

// fetchWithCache downloads a translation once and reuses it thereafter, so
// rebuilding the site is offline and instant. The cache is build output, not
// source: delete it and the next run re-downloads.
func fetchWithCache(v webVersion, cacheDir string, offline bool) ([]byte, error) {
	path := filepath.Join(cacheDir, v.ID+".json")
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

func writeSite(site *siteWriter, versions []loadedVersion) error {
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

	js := readerJS(versions)
	css := readerCSS(regularFile, boldFile)
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
	if err := writeNotFound(site, versions); err != nil {
		return err
	}
	for _, v := range versions {
		if err := writeVersion(site, v, versions); err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
	}
	for _, nv := range noticedVersions() {
		if err := writeNoticeVersion(site, nv, versions); err != nil {
			return fmt.Errorf("%s: %w", nv.ID, err)
		}
	}
	return nil
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
	return strings.Replace(readerJSTemplate, "__BOOKS__", string(table), 1)
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
				Offers: offersForBook(v.ID, all, book, slug, scopeBook.depth()),
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
				Offers: offersForChapter(v.ID, all, book, slug, ch, scopeChapter.depth()),
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
