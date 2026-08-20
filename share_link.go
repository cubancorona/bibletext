package bibletext

// "Share as link" — the URL half of sharing a verse.
//
// THE CONTRACT (frozen; see bookslugs.go for why nothing here may change):
//
//	https://bibletext.co.uk/<version>/<book-slug>/<chapter>/#v<lo>[-<hi>][&n=<note>]
//
// THE FRAGMENT IS A KEY LIST, "&"-separated. Today two keys exist:
//
//	v=16 / v=16-18   the verse span, written bare as "v16" / "v16-18"
//	n=<payload>      an optional note from the sender (share_note.go)
//
// UNKNOWN KEYS ARE IGNORED, NEVER REJECTED — by every parser, from the first
// release onward. That single rule is what lets a key be added later without
// stranding the links already sent, and it is why the grammar was made a key
// list before the first link-capable release shipped rather than after.
//
// The bare "v16" spelling (rather than "v=16") is kept because it is what the
// web reader already emits and what every link in the wild already looks like;
// the parser accepts both.
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
//   - The path may name a translation the WEBSITE DOES NOT PUBLISH. It named
//     only the public-domain three until the owner asked for NKJV links that
//     "work just like the others", and the rule that kept it that way was
//     mis-stated: what a licence protects is the TEXT, and a translation id is a
//     name, not text. So "nkjv" in a URL gives nothing away — but it does mean
//     /nkjv/ is APP-ONLY today, because the website half is deferred (owner's
//     call) and bibletext.co.uk/nkjv/john/3/ is a 404. See linkPathVersionIDs
//     for the two sets this split into. An id nothing recognises still falls
//     back to WEB.
//   - The version id is the FIRST path segment — there is no /read/ prefix. That
//     makes a shared link as short as it can be, at the cost of reserving those
//     ids at the site root: no future root page may be called web, bsb, webc or
//     nkjv, and the generator refuses to overwrite the hand-written pages
//     (see cmd/websitegen's reservedRootNames).

import (
	"strconv"
	"strings"
)

const shareLinkBase = "https://bibletext.co.uk"

// webPublishedVersionIDs are the versions whose TEXT the web reader publishes —
// the public-domain three. Kept as its own set (rather than reading PublicDomain
// off the registry) so that adding a version to the app is a deliberate,
// separate decision from publishing its text on the web.
//
// cmd/websitegen keeps its own hardcoded copy (publishedVersions) and does not
// read this one: the generator's list is guarded by
// cmd/websitegen/licensed_exclusion_test.go, and a licensed translation must
// never reach the site because a shared set drifted. Publishing NKJV pages is a
// separate, owner-gated decision — this change deliberately does not make it.
var webPublishedVersionIDs = map[string]bool{"web": true, "bsb": true, "webc": true}

// linkPathVersionIDs are the ids a share-link PATH may name — the grammar both
// ShareLinkURLWithNote and ParseShareLink are gated on.
//
// It USED to be the same set as webPublishedVersionIDs, and one name served
// both questions. It cannot any more: /nkjv/ is a link the app emits and opens,
// and a page the site does not serve. Conflating them again is how someone
// concludes NKJV is published and either points the generator at it or relaxes
// the licensed-exclusion tests.
//
// Enumerated, never derived from the registry: canSelect() is true for every
// registered version under BIBLETEXT_ENABLE_TESTING, so a registry-wide set
// would start emitting /nrsv/ and /lsb/ paths that nothing claims and nothing
// serves.
var linkPathVersionIDs = func() map[string]bool {
	m := make(map[string]bool, len(webPublishedVersionIDs)+1)
	for id := range webPublishedVersionIDs {
		m[id] = true
	}
	// nkjv is app-only: the app emits it, the AASA and the Android manifest
	// claim it, and the website does not serve it (docs/apple-app-site-
	// association, cmd/mobile/AndroidManifest.xml).
	m["nkjv"] = true
	return m
}()

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
	return ShareLinkURLWithNote(versionID, book, chapter, lo, hi, "")
}

// ShareLinkURLWithNote is ShareLinkURL plus an optional note from the sender.
// An empty note (or one that is empty once normalized) yields exactly the link
// ShareLinkURL would have produced — byte for byte — so adding this feature
// changed nothing about the links that do not use it.
func ShareLinkURLWithNote(versionID, book string, chapter, lo, hi int, note string) string {
	return ShareLinkURLWithNoteNonce(versionID, book, chapter, lo, hi, note, nil)
}

// ShareLinkURLWithNoteNonce is ShareLinkURLWithNote plus the note's per-share
// identity, which the sending device keeps so it can recognise its own note
// coming home (share_note.go, noteTagNonce). A nil nonce emits nothing and
// produces exactly the link the plain builder would — so every existing caller,
// and every link already in the world, is unaffected.
func ShareLinkURLWithNoteNonce(versionID, book string, chapter, lo, hi int, note string, nonce []byte) string {
	slug, ok := BookSlug(book)
	if !ok {
		return ""
	}
	version := strings.ToLower(strings.TrimSpace(versionID))
	if !linkPathVersionIDs[version] {
		version = defaultWebVersionID
	}
	// The deuterocanon exists only in the Catholic canon, so a link to one of
	// those books must name webc whatever the reader had selected. This runs
	// AFTER the gate above and must stay there: now that a translation the site
	// does not publish can survive that gate, this is the line that stops a
	// /<licensed>/tobit/ link pointing at a canon nothing can serve. Still
	// unreachable today — those books are only readable under webc, and the NKJV
	// is a 66-book canon — belt and braces for later.
	if !protestantCanonBooks[book] {
		version = "webc"
	}
	if chapter < 1 {
		chapter = 1
	}
	url := shareLinkBase + "/" + version + "/" + slug + "/" + strconv.Itoa(chapter) + "/"

	var keys []string
	switch {
	case lo <= 0: // a chapter link names no verse
	case hi > lo:
		keys = append(keys, "v"+strconv.Itoa(lo)+"-"+strconv.Itoa(hi))
	default:
		keys = append(keys, "v"+strconv.Itoa(lo))
	}
	// The note payload carries its OWN anchor beside the text (share_note.go):
	// 'v' is the SENDER's translation id — the caller's, because the PATH is
	// lossy by construction (webc is forced for the deuterocanon just above,
	// and an unknown id falls back to web), and reconstructing the sender's
	// translation from the path is identity rebuilt from context. Only when the
	// caller gave nothing usable does the path version stand in, so 'v' is
	// always present on anything we emit. 'b'/'c'/'a' name the passage in the
	// wire's own frame; the fragment's verse span above remains what navigates
	// the page.
	noteVersion := strings.ToLower(strings.TrimSpace(versionID))
	if noteWireVersionID(noteVersion) == "" {
		noteVersion = version
	}
	if payload := (EncodeNoteWire(NoteWire{
		Text:    note,
		Version: noteVersion,
		Book:    book,
		Chapter: chapter,
		VerseLo: lo,
		VerseHi: hi,
		Nonce:   nonce,
	})); payload != "" {
		keys = append(keys, "n="+payload)
	}
	// NO TRANSLATION KEY HERE — the PATH carries the translation now.
	//
	// It was built the other way round first: a "t=nkjv" fragment hint beside
	// "n=", on the belief that the path was licensing-constrained to the
	// published three and a fragment (never sent to a server) was the only place
	// a licensed id could ride. That was solving the wrong problem — the licence
	// covers the text, not the name — and it was strictly worse: a fragment is
	// invisible to the web reader, which is the half that serves everyone who has
	// no app. The hint and its parser were deleted when the path learned to say
	// it; two mechanisms for one job is worse than either.
	if len(keys) == 0 {
		return url
	}
	return url + "#" + strings.Join(keys, "&")
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
