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
// The reader lives at the ROOT, not under a /read/ prefix, so a shared link is
// as short as possible. The site root is therefore shared with the hand-written
// landing/privacy/support pages: reservedRootNames below makes it impossible for
// the generator to overwrite one of them, and the three version ids are reserved
// at the root forever.
//
// A single verse link (#v16) needs NO JavaScript: each verse carries an id and
// CSS :target draws the highlight. Ranges (#v16-18) and the platform-aware
// "Get the app" link are the only things reader.js does, and both degrade
// cleanly when it doesn't run.
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
	js := readerJS(versions)
	cssName = "assets/reader." + contentHash(readerCSS) + ".css"
	jsName = "assets/reader." + contentHash(js) + ".js"
	if err := site.write(cssName, readerCSS); err != nil {
		return err
	}
	if err := site.write(jsName, js); err != nil {
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
	return nil
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

func writeVersion(site *siteWriter, v loadedVersion, all []loadedVersion) error {
	if err := site.write(v.ID+"/index.html", renderBookList(v, all)); err != nil {
		return err
	}
	for _, book := range v.bible.Books {
		slug, ok := bibletext.BookSlug(book)
		if !ok {
			return fmt.Errorf("book %q has no slug — add it to bookslugs.go", book)
		}
		chapters := sortedChapters(v.bible, book)
		if len(chapters) == 0 {
			continue
		}
		base := v.ID + "/" + slug
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
	}
	return nil
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
