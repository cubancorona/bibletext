// Command websitegen builds the static web reader published at
// bibletext.co.uk/read/ — the destination of the app's "Share as link".
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
//	read/index.html                      front door, redirects to the default version
//	read/<version>/index.html            book list
//	read/<version>/<book>/index.html     chapter list
//	read/<version>/<book>/<ch>/index.html   the chapter — one static file per chapter
//	read/assets/reader.css               one stylesheet
//	read/assets/reader.js                two small progressive enhancements
//	404.html                             site-wide, at the root (see writeNotFound)
//
// A single verse link (#v16) needs NO JavaScript: each verse carries an id and
// CSS :target draws the highlight. Ranges (#v16-18) and the platform-aware
// "Get the app" link are the only things reader.js does, and both degrade
// cleanly when it doesn't run.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

func (s *siteWriter) write(relPath, content string) error {
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
	if err := site.write("read/assets/reader.css", readerCSS); err != nil {
		return err
	}
	if err := site.write("read/assets/reader.js", readerJS); err != nil {
		return err
	}
	if err := writeNotFound(site, versions); err != nil {
		return err
	}
	if err := site.write("read/index.html", renderFrontDoor()); err != nil {
		return err
	}
	for _, v := range versions {
		if err := writeVersion(site, v, versions); err != nil {
			return fmt.Errorf("%s: %w", v.ID, err)
		}
	}
	return nil
}

func writeVersion(site *siteWriter, v loadedVersion, all []loadedVersion) error {
	if err := site.write("read/"+v.ID+"/index.html", renderBookList(v, all)); err != nil {
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
		base := "read/" + v.ID + "/" + slug
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
