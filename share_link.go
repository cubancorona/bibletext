package bibletext

// "Share as link" — the URL half of sharing a verse.
//
// THE CONTRACT (frozen; see bookslugs.go for why nothing here may change):
//
//	https://bibletext.co.uk/read/<version>/<book-slug>/<chapter>/#v<lo>[-<hi>]
//
// Every part is deliberate:
//
//   - TRAILING SLASH always. GitHub Pages 301s the slash-less form, so emitting
//     it costs a redirect hop today and would hard-404 on a host that doesn't
//     redirect. The slash is part of the contract.
//   - The verse rides in the FRAGMENT, never the path or query. Fragments never
//     reach the server, so one static page per chapter serves every verse link;
//     and if a future text revision renumbers a verse, the link degrades to
//     "chapter opens, anchor doesn't scroll" instead of dying. A single verse
//     needs no JavaScript at all — the page gives each verse an id and lets CSS
//     :target do the highlight.
//   - Lowercase ASCII only. Pages serves case-sensitively, and the citation's
//     en dash (John 3:16–18) must never reach a URL: messengers mangle it.
//   - Only the public-domain versions ever appear. A licensed version id must
//     never leak into a public URL, so anything unrecognised falls back to WEB.

import (
	"strconv"
	"strings"
)

const shareLinkBase = "https://bibletext.co.uk/read"

// webVersionIDs are the versions the web reader publishes — the public-domain
// three. Kept as its own set (rather than reading PublicDomain off the
// registry) so that adding a version to the app is a deliberate, separate
// decision from publishing its text on the web.
var webVersionIDs = map[string]bool{"web": true, "bsb": true, "webc": true}

// ShareLinkURL builds the permanent web URL for a passage. lo/hi are verse
// numbers; pass lo <= 0 for a chapter-level link, or hi <= lo for a single
// verse. It returns "" only if the book is unknown — callers fall back to
// sharing plain text rather than a broken link.
//
// A selection spanning several chapters passes its FIRST chapter and that
// chapter's verse span: the link then under-highlights rather than pointing at
// the wrong chapter. (The app cannot select across chapters today; fixing the
// rule now means tomorrow's app needs no new pages.)
func ShareLinkURL(versionID, book string, chapter, lo, hi int) string {
	slug, ok := BookSlug(book)
	if !ok {
		return ""
	}
	version := strings.ToLower(strings.TrimSpace(versionID))
	if !webVersionIDs[version] {
		version = defaultWebVersionID
	}
	// The deuterocanon exists only in the Catholic canon, so a link to one of
	// those books must name webc whatever the reader had selected. Unreachable
	// today (they are only readable under webc) — belt and braces for later.
	if !protestantCanonBooks[book] {
		version = "webc"
	}
	if chapter < 1 {
		chapter = 1
	}
	url := shareLinkBase + "/" + version + "/" + slug + "/" + strconv.Itoa(chapter) + "/"
	switch {
	case lo <= 0:
		return url
	case hi > lo:
		return url + "#v" + strconv.Itoa(lo) + "-" + strconv.Itoa(hi)
	default:
		return url + "#v" + strconv.Itoa(lo)
	}
}

const defaultWebVersionID = "web"

// protestantCanonBooks is the 66-book set, used only to detect deuterocanonical
// books (which force the webc version in a link).
var protestantCanonBooks = func() map[string]bool {
	m := make(map[string]bool, 66)
	for _, b := range NewBibleData().Books {
		m[b] = true
	}
	return m
}()
