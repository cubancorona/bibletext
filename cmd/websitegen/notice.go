package main

// THE ONE PAGE ON THIS SITE THAT NAMES A PASSAGE WITHOUT SHOWING IT.
//
// Two different holes in the site turned out to be the same page, because the
// reader arrives at both with the same question — "I followed a link and the
// words aren't here, now what?" — and only the reason and the route out differ:
//
//	/nkjv/john/3/   the text is licensed and this site does not publish it
//	                (B_WEB_404, B_UNFURL_NKJV in docs/NKJV_FLOW.md)
//	/web/tobit/1/   the book is not in this translation's canon
//	                (B_DEUTERO_WEB_404)
//
// Before this, the first was a bare 404 for every recipient without the app and
// a bare URL in every message thread; the second was a bare 404 that
// writeVersion produced by silently skipping zero-chapter books.
//
// THE RULE THIS PAGE EXISTS TO SATISFY is docs/NKJV_FLOW.md's I1: every state
// must offer at least one action that reaches a state where the reader is
// reading something. A page that says "not here" and stops is the failure, not
// the fix. Every variant below therefore ends in at least one link to a chapter
// this site really serves, and TestNoticePagesAlwaysOfferARouteOut asserts it
// over the whole generated tree rather than trusting this comment.
//
// WHAT IT MUST NEVER DO. Not one word of the New King James Version appears on
// these pages — not in the body, not in og:description, not in the title beyond
// the reference itself. The structural guarantee is that noticeSpec has no
// field that can hold scripture and renderNotice is handed no BibleData at all;
// the site holds no NKJV text to leak in the first place. licensed_exclusion_test.go
// re-proves it over the emitted files.
//
// WHAT THE SERVER CANNOT KNOW. The verse and the sender's note ride in the URL
// FRAGMENT (share_link.go), which never reaches any server — so a
// server-rendered "read this verse in the WEB" link cannot carry them. The
// links below are therefore written chapter-level and REWRITTEN client-side
// from location.hash by notice.js. With scripting off the reader still gets the
// right chapter in a translation this site serves, which is the honest floor.

import (
	"fmt"
	"html/template"
	"strconv"
	"strings"

	bibletext "bibletext"
)

// noticeReason is why the words are missing. It selects the copy and whether
// the page offers the app at all; nothing else in the renderer branches.
type noticeReason int

const (
	// reasonLicensed — the translation is real, the app reads it, and this site
	// does not publish its text.
	reasonLicensed noticeReason = iota
	// reasonAbsent — this translation's canon does not contain the book or the
	// chapter. The app does not have it either, so the app is not the answer;
	// the translation switch is.
	reasonAbsent
)

// noticeScope is how much of a reference the page names, which is also its
// depth under the site root.
type noticeScope int

const (
	scopeVersion noticeScope = iota // /nkjv/
	scopeBook                       // /nkjv/john/ or /web/tobit/
	scopeChapter                    // /nkjv/john/3/ or /web/tobit/1/
)

func (s noticeScope) depth() int { return int(s) + 1 }

// noticeOffer is one route out: the same passage in a translation this site
// really serves.
type noticeOffer struct {
	ID   string
	Name string
	Href string // relative to the page, chapter-level, no fragment
	// Diff is bibletext.Numbering* — why the link may not carry the shared verse
	// into this translation, or NumberingSame when it may. Measured per chapter
	// by bibletext.ChapterNumberingDifference; never assumed, and never
	// flattened to one sentence, because "the numbers moved" and "that verse
	// isn't in this translation" are things a reader would act on differently.
	Diff string
}

// CarryVerse reports whether notice.js may append the shared verse to this
// link's href.
func (o noticeOffer) CarryVerse() bool { return o.Diff == bibletext.NumberingSame }

// noticeSpec is everything a notice page is. Deliberately holds no verse text
// and no BibleData: the renderer below CANNOT emit scripture because it is
// never given any.
type noticeSpec struct {
	Reason      noticeReason
	Scope       noticeScope
	VersionID   string // the version whose root this page lives under
	VersionName string
	Book        string // "" at scopeVersion
	Slug        string
	Chapter     int // 0 above scopeChapter
	Prev, Next  int // neighbouring chapters that exist as pages, 0 for none
	// OwnLastChapter is the LAST chapter number this translation has of this
	// book — 0 when it has none of it. It is what separates the two absent-canon
	// stories: 0 means the whole book is missing, anything else means the reader
	// has the book open and has run off the end of it.
	//
	// It is the last NUMBER, not the count. Those coincide on the real data (the
	// WEB's Daniel is 1..12) and diverge the moment a canon has a gap in the
	// middle, which is how the first version of this told a fixture reader that
	// Daniel "ends at chapter 2".
	OwnLastChapter int
	// Offers are the routes out, in the published-version order. At least one,
	// always — see the I1 note at the top of this file.
	Offers []noticeOffer
}

const (
	appStoreURL   = "https://apps.apple.com/app/id6784567351"
	appLandingURL = "https://bibletext.co.uk/"
	// androidPackage is the applicationId scripts/build-android.sh packages with
	// and cmd/mobile/AndroidManifest.xml declares. notice.js builds an
	// intent:// URL around it.
	androidPackage = "uk.co.bibletext"
	// appleAppID feeds the Smart App Banner.
	appleAppID = "6784567351"
)

// ref is the passage as a reader would say it: "John 3", "John", or the
// translation's own name at the top of its root.
func (n noticeSpec) ref() string {
	switch n.Scope {
	case scopeChapter:
		return fmt.Sprintf("%s %d", n.Book, n.Chapter)
	case scopeBook:
		return n.Book
	}
	return n.VersionName
}

func (n noticeSpec) canonical() string {
	switch n.Scope {
	case scopeChapter:
		return fmt.Sprintf("https://bibletext.co.uk/%s/%s/%d/", n.VersionID, n.Slug, n.Chapter)
	case scopeBook:
		return fmt.Sprintf("https://bibletext.co.uk/%s/%s/", n.VersionID, n.Slug)
	}
	return "https://bibletext.co.uk/" + n.VersionID + "/"
}

// renderNotice builds the page. One function, four call sites (version root,
// book index, chapter, and the absent-canon variants of the last two), because
// they are one page with one job.
func renderNotice(n noticeSpec) string {
	var b strings.Builder
	b.WriteString(`<div class="wrap">`)
	b.WriteString(noticeNav(n))

	if n.Scope == scopeChapter {
		b.WriteString(`<div class="chapbar">`)
		fmt.Fprintf(&b, `<h1 class="ref"><span class="passage">%s</span></h1><div class="chapnav">`,
			template.HTMLEscapeString(n.ref()))
		b.WriteString(arrowLink("&larr;", "prev", n.Book, n.Prev))
		b.WriteString(arrowLink("&rarr;", "next", n.Book, n.Next))
		b.WriteString(`</div></div>`)
	} else {
		fmt.Fprintf(&b, `<h1 class="ref"><span class="passage">%s</span></h1>`,
			template.HTMLEscapeString(n.ref()))
	}
	fmt.Fprintf(&b, `<p class="ver">%s</p>`, template.HTMLEscapeString(n.verLine()))

	// THE SENDER'S NOTE LANDS HERE, and this empty element is the whole reason
	// it does. reader.js renders the note (it is the only thing that decodes the
	// fragment payload) and inserts it immediately before `.text`; with no
	// `.text` on the page its fallback appends to `.wrap`, which put a
	// stranger's message underneath the footer links. Thirty bytes buys the
	// right position. It renders nothing itself.
	//
	// The note matters MORE here than on a chapter page: the reader cannot see
	// the verse, so the message is the only thing on the page that is actually
	// from the person who sent the link (I3).
	b.WriteString(`<article class="text"></article>`)

	fmt.Fprintf(&b, `<p class="lede">%s</p>`, n.lede())

	if n.Reason == reasonLicensed {
		b.WriteString(openInApp(n))
	}
	b.WriteString(parallelSection(n))

	if n.Scope == scopeChapter {
		b.WriteString(`<nav class="pager">`)
		if n.Prev > 0 {
			fmt.Fprintf(&b, `<a rel="prev" href="../%d/">&larr; %s %d</a>`, n.Prev,
				template.HTMLEscapeString(n.Book), n.Prev)
		} else {
			b.WriteString(`<span></span>`)
		}
		if n.Next > 0 {
			fmt.Fprintf(&b, `<a rel="next" href="../%d/">%s %d &rarr;</a>`, n.Next,
				template.HTMLEscapeString(n.Book), n.Next)
		} else {
			b.WriteString(`<span></span>`)
		}
		b.WriteString(`</nav>`)
	}
	b.WriteString(`</div>`)

	head := pageHead{
		// noindex,follow. These are ~1,500 thin, near-identical pages that exist
		// for shared links and for message-thread unfurls, not for search; an
		// index full of them would bury the pages that actually carry
		// scripture. follow, so the routes out still pass authority on. This
		// costs the feature nothing: unfurlers read og: tags and ignore robots
		// meta entirely.
		robots: "noindex,follow",
		css:    []string{noticeCSSName},
		js:     []string{noticeJSName},
	}
	if n.Reason == reasonLicensed {
		// APPLE'S SMART APP BANNER — see openInApp for why this is here and not
		// a link. app-argument is the canonical chapter URL; it is the sanctioned
		// way to hand the app context, and it costs nothing if iOS drops it.
		head.appleBanner = "app-id=" + appleAppID + ", app-argument=" + n.canonical()
	}
	return pageShellHead(n.title(), n.ogTitle(), n.ogDesc(), n.canonical(), b.String(), n.Scope.depth(), head)
}

// verLine is the accent line under the heading — the same slot the published
// pages use to name the translation.
func (n noticeSpec) verLine() string {
	if n.Scope == scopeVersion {
		return "Not published on this site"
	}
	return n.VersionName
}

// noticeNav is the trail and the translation pills.
//
// TWO THINGS ARE DELIBERATELY ABSENT, both because reader.js is FROZEN — its
// bytes are in the content-hashed filename every one of the 3,906 published
// pages links, so changing it would change all of them and the three published
// trees would no longer be byte-identical:
//
//  1. No "Go to" row. reader.js builds its picker from a book table filtered by
//     b.ch[<version from the path>]; there is no nkjv column, so on /nkjv/ the
//     picker would open empty — a dead end, which is the one thing this page
//     exists to not be. Giving it a column means regenerating reader.js.
//  2. The pills are `.npick`, not `.vpick`. reader.js's carryVerse() appends the
//     whole fragment to every `.vpick` href unconditionally; on the ~23 chapters
//     where the numbering does not agree that would hand the reader a confident
//     link to the wrong verse. Fragment carrying on these pages is notice.js's
//     single job, so nothing else may do half of it.
//
// Both are worth folding in the day someone accepts a one-time asset-hash churn
// across the whole site.
func noticeNav(n noticeSpec) string {
	var b strings.Builder
	up := strings.Repeat("../", n.Scope.depth())
	cls := `<nav class="top">`
	if n.Scope != scopeChapter {
		cls = `<nav class="top plain">`
	}
	b.WriteString(cls)
	fmt.Fprintf(&b, `<a class="home" href="%s">BibleText</a>`, up)
	if n.Scope != scopeVersion {
		b.WriteString(`<span class="crumbs">`)
		switch n.Scope {
		case scopeChapter:
			fmt.Fprintf(&b, `<a href="../../">%s</a> <span class="sep">/</span> <a href="../">%s</a>`,
				template.HTMLEscapeString(strings.ToUpper(n.VersionID)), template.HTMLEscapeString(n.Book))
		default:
			fmt.Fprintf(&b, `<a href="../">%s</a>`, template.HTMLEscapeString(strings.ToUpper(n.VersionID)))
		}
		b.WriteString(`</span>`)
	}
	// FULL NAMES, not the WEB/BSB/WEBC/NKJV pills the scripture pages use.
	//
	// Everybody who reads this page arrived by following somebody else's link and
	// found no words on it. They are, by definition, the reader least likely to
	// know what "WEBC" stands for — and this row is one of the two routes out, so
	// an abbreviation is a route they cannot evaluate. The offers further down
	// already say "World English Bible (Catholic)" in full; a page that names the
	// same translation two ways is talking past itself.
	//
	// This is the .npick row, which exists only on notice pages, so the .vpick
	// switcher on all 3,906 scripture pages is untouched — there the abbreviation
	// earns its place, because it is a control a returning reader uses often and
	// four full names would not fit a phone.
	b.WriteString(`<span class="vers vers-full">`)
	for _, o := range n.Offers {
		fmt.Fprintf(&b, `<a class="npick"%s title="%s" href="%s">%s</a>`,
			fragAttr(n, o), template.HTMLEscapeString(o.Name), o.Href,
			template.HTMLEscapeString(o.Name))
	}
	fmt.Fprintf(&b, `<span class="npick on">%s</span>`, template.HTMLEscapeString(n.VersionName))
	b.WriteString(`</span></nav>`)
	return b.String()
}

// passage wraps a reference so notice.js can upgrade "John 3" to "John 3:16"
// from the fragment. One mechanism, every occurrence, including the <h1>.
func passage(ref string) string {
	return `<span class="passage">` + template.HTMLEscapeString(ref) + `</span>`
}

// chapterOnlyPassage names the CHAPTER and is never upgraded to name the verse,
// because the sentence it appears in is a promise about what the app will do —
// and the app cannot keep a verse-level one.
//
// notice.js rewrites every .passage span from the fragment, so "John 3" becomes
// "John 3:16-18" wherever it appears. That is right for the parallel-passage
// offer, which really does carry the verse across, and WRONG here: on Android
// the intent:// grammar has no room for the target's own fragment (its "#"
// opens the Intent block), so the app lands on the chapter; on iOS the route is
// the App Store, which does not open a passage at all. The page would have been
// promising a verse that neither platform delivers, in a sentence the reader has
// no way to check until they have already tapped.
//
// So it opts out of the class. Chapter-level is a promise both platforms keep.
func chapterOnlyPassage(ref string) string {
	return `<span>` + template.HTMLEscapeString(ref) + `</span>`
}

// lede is the explanation: what this page is, and why the words are not on it.
//
// VOICE. Plain declaratives, contractions, an em dash for the pivot; no
// apology, no exclamation, no copyright lecture. The models are the 404's
// "That page isn't here — but the whole Bible is." and the app's own card,
// whose comment states the rule outright: "No apology, no licensing lecture —
// neither helps them read the verse."
//
// NO © LINE. The publisher's attribution belongs with the publisher's TEXT, and
// there is none here — a copyright notice on a page carrying no scripture is
// noise that reads as a complaint. versions.go's LicenseNotice travels with the
// text in the app, which is where it is owed.
func (n noticeSpec) lede() string {
	if n.Reason == reasonAbsent {
		return n.absentLede()
	}
	switch n.Scope {
	case scopeChapter:
		return fmt.Sprintf(
			`This link was shared in the %s. That text isn't published on this site — the BibleText `+
				`app reads it instead. Two ways to reach %s:`,
			template.HTMLEscapeString(n.VersionName), passage(n.ref()))
	case scopeBook:
		return fmt.Sprintf(
			`The %s isn't published on this site. Open %s in the BibleText app, or read it here in `+
				`another translation.`,
			template.HTMLEscapeString(n.VersionName), passage(n.Book))
	}
	return fmt.Sprintf(
		`The BibleText app reads the %s; this site doesn't publish it. The whole Bible is here in `+
			`three other translations.`,
		template.HTMLEscapeString(n.VersionName))
}

// absentLede names the gap without inventing a reason for it. The seven
// deuterocanonical books get the one extra clause that actually helps, because
// "why isn't Tobit here" has a real answer a reader can use; a chapter gap
// inside a book they already have gets the plain arithmetic.
func (n noticeSpec) absentLede() string {
	other := ""
	if len(n.Offers) > 0 {
		other = n.Offers[0].Name
	}
	if n.OwnLastChapter > 0 {
		// The book is here, this chapter is not — the WEB's Daniel against the
		// longer Greek Daniel.
		return fmt.Sprintf(
			`The %s's %s ends at chapter %d. Chapter %d is in the %s.`,
			template.HTMLEscapeString(n.VersionName), template.HTMLEscapeString(n.Book),
			n.OwnLastChapter, n.Chapter, template.HTMLEscapeString(other))
	}
	return fmt.Sprintf(
		`%s isn't in the %s. It's one of the seven deuterocanonical books, which this edition doesn't `+
			`carry — the %s does.`,
		template.HTMLEscapeString(n.Book), template.HTMLEscapeString(n.VersionName),
		template.HTMLEscapeString(other))
}

// openInApp is the explicit open-in-app button, incorporated with the
// explanation. The heading carries the explanation and the button carries the
// action, so neither can be read without the other.
//
// WHY THIS IS NOT A PLAIN LINK, and what each platform actually gets today:
//
//	iOS      A Universal Link does NOT open the app when the reader is already
//	         in Safari on the same domain, and no custom URL scheme is
//	         registered anywhere in this repo (no CFBundleURLTypes; the Android
//	         manifest declares only https app links). So the honest route is
//	         Apple's own: the Smart App Banner in the head (which reads OPEN
//	         when the app is installed), plus a button to the App Store product
//	         page — whose own button also reads OPEN when it is installed.
//	Android  intent:// with package=uk.co.bibletext and a
//	         S.browser_fallback_url, which really does hand the URL to the app
//	         when it is there and falls back to the download page when it is
//	         not.
//	Desktop  there is no app to hand off to from a browser, so the affordance is
//	         the download — which is also the server-rendered default, and
//	         therefore what a reader with scripting off gets everywhere.
//
// The href here is that default; notice.js narrows it per platform. Structured
// so that registering a custom scheme later changes ONE branch of notice.js and
// no markup: the button is already an id with a data- attribute for each
// platform's target.
func openInApp(n noticeSpec) string {
	var b strings.Builder
	b.WriteString(`<section class="nsec">`)
	b.WriteString(`<h2>Not available here &mdash; but you can open it in the app</h2>`)
	if n.Scope != scopeVersion {
		fmt.Fprintf(&b, `<p>BibleText opens straight at %s.</p>`, chapterOnlyPassage(n.ref()))
	}
	b.WriteString(`<div class="appbtnrow">`)
	fmt.Fprintf(&b, `<a class="goto btnapp" id="openapp" href="%s" data-ios="%s" data-pkg="%s" `+
		`data-label="Open in BibleText">Get the BibleText app</a>`,
		appLandingURL, appStoreURL, androidPackage)
	b.WriteString(`</div>`)
	b.WriteString(`<p class="opensub">Free on iPhone, iPad, Mac, Android, Windows and Linux &mdash; ` +
		`no ads, no account.</p>`)
	// Said out loud rather than left as a surprise: on iOS the hop really is
	// through the App Store, because Safari will not hand a bibletext.co.uk
	// link to the app.
	b.WriteString(`<p class="opensub" id="iosnote" hidden>On iPhone and iPad this goes by way of the ` +
		`App Store &mdash; Safari can't pass a bibletext.co.uk link to the app itself.</p>`)
	b.WriteString(`</section>`)
	return b.String()
}

// parallelSection is the option to view the parallel passage (with its note)
// in another version. Every variant of this page carries it; on the
// absent-canon pages it is the only route out, which is why it is never
// conditional.
func parallelSection(n noticeSpec) string {
	var b strings.Builder
	b.WriteString(`<section class="nsec">`)
	if n.Reason == reasonAbsent {
		b.WriteString(`<h2>Read it in a translation that has it</h2>`)
	} else if n.Scope == scopeVersion {
		b.WriteString(`<h2>Or read the Bible here</h2>`)
	} else {
		b.WriteString(`<h2>Or read the passage here</h2>`)
	}

	switch {
	case n.Scope == scopeVersion:
		// Nothing to say beyond the grid.
	case n.Reason == reasonAbsent && n.Scope == scopeBook && len(n.Offers) > 0:
		fmt.Fprintf(&b, `<p>%s is published here in the %s.</p>`,
			passage(n.Book), template.HTMLEscapeString(n.Offers[0].Name))
	case n.Reason == reasonAbsent && len(n.Offers) > 0:
		fmt.Fprintf(&b, `<p>%s is published here in the %s.</p>`,
			passage(n.ref()), template.HTMLEscapeString(n.Offers[0].Name))
	case len(n.Offers) == 1:
		fmt.Fprintf(&b, `<p>%s is published here in the %s.</p>`,
			passage(n.ref()), template.HTMLEscapeString(n.Offers[0].Name))
	default:
		fmt.Fprintf(&b, `<p>%s is published here in all %s.</p>`, passage(n.ref()), numberWord(len(n.Offers)))
	}

	b.WriteString(`<ul class="grid">`)
	for _, o := range n.Offers {
		fmt.Fprintf(&b, `<li><a%s href="%s">%s</a></li>`,
			fragAttr(n, o), o.Href, template.HTMLEscapeString(o.Name))
	}
	b.WriteString(`</ul>`)

	// The versification caveats. Written into the HTML rather than left to the
	// script, because they are true with scripting off too — the link is
	// chapter-level either way.
	//
	// GROUPED BY KIND, not one sentence per translation. Romans 16 differs in
	// all three, and three near-identical sentences under three links read as a
	// stutter and get skipped; one sentence naming all three is the same
	// information a reader will actually take in.
	for _, kind := range []string{
		bibletext.NumberingIncommensurable, bibletext.NumberingAbsent, bibletext.NumberingMoved,
	} {
		var names []string
		for _, o := range n.Offers {
			if o.Diff == kind {
				names = append(names, o.Name)
			}
		}
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<p class="vnote">%s</p>`, template.HTMLEscapeString(n.caveat(kind, names)))
	}
	b.WriteString(`</section>`)
	return b.String()
}

// caveat is the one sentence that explains a chapter-level link. It says what
// is actually true of each kind: a move is a move, a missing verse is missing,
// and a book that does not correspond at all is neither.
func (n noticeSpec) caveat(kind string, names []string) string {
	list := englishList(names)
	plural := len(names) > 1
	those, opens := "that link", "opens"
	if plural {
		those, opens = "those links", "open"
	}
	switch kind {
	case bibletext.NumberingIncommensurable:
		return fmt.Sprintf("%s in %s is a different text whose verses don't correspond to these, so %s "+
			"opens the chapter.", n.Book, lower(list), those)
	case bibletext.NumberingAbsent:
		return fmt.Sprintf("%s %s carry every verse the %s numbers here, so %s %s %s rather than the "+
			"verse.", list, pick(plural, "don't", "doesn't"), n.VersionName, those, opens, n.ref())
	default:
		return fmt.Sprintf("%s %s this chapter differently, so %s %s %s rather than the verse.",
			list, pick(plural, "number", "numbers"), those, opens, n.ref())
	}
}

func pick(plural bool, many, one string) string {
	if plural {
		return many
	}
	return one
}

// englishList joins translation names the way a sentence does: "the WEB",
// "the WEB and the BSB", "the WEB, the BSB and the WEBC" — capitalised for the
// head of a sentence. No serial comma; the site's prose does not use one.
func englishList(names []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = "the " + n
	}
	s := parts[0]
	if len(parts) > 1 {
		s = strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// fragAttr says how much of the fragment notice.js may put on this link.
//
//	chapter scope, numbering agrees   the verse AND the note
//	chapter scope, numbering differs  the note only — the verse would name a
//	                                  different passage there, and .vnote says so
//	book scope                        the note only; there is no verse to carry
//	                                  to a chapter list, and dropping a person's
//	                                  message on the way is I3's failure even
//	                                  though the index shows it low on the page
//	version scope                     nothing. A whole-Bible index is not where
//	                                  a note belongs, and no note-bearing link
//	                                  ever points at one.
func fragAttr(n noticeSpec, o noticeOffer) string {
	switch n.Scope {
	case scopeChapter:
		if o.CarryVerse() {
			return ` data-frag="verse"`
		}
		return ` data-frag="note"`
	case scopeBook:
		return ` data-frag="note"`
	}
	return ""
}

func numberWord(n int) string {
	switch n {
	case 2:
		return "two"
	case 3:
		return "three"
	}
	return "of " + strconv.Itoa(n)
}

// --- head metadata -----------------------------------------------------------
//
// The og: tags are the whole reason this page exists for a licensed link: an
// NKJV link unfurled in a message thread was a bare URL with no reference at
// all (B_UNFURL_NKJV). It now names the passage and says what the recipient can
// do — and it carries NO scripture, because an unfurl is republication to a
// group chat.

func (n noticeSpec) title() string {
	if n.Scope == scopeVersion {
		return n.VersionName + " | BibleText"
	}
	return n.ref() + " — " + n.VersionName + " | BibleText"
}

func (n noticeSpec) ogTitle() string {
	if n.Scope == scopeVersion {
		return n.VersionName
	}
	return fmt.Sprintf("%s (%s)", n.ref(), n.VersionName)
}

func (n noticeSpec) ogDesc() string {
	other := ""
	if len(n.Offers) > 0 {
		other = n.Offers[0].Name
	}
	if n.Reason == reasonAbsent {
		if n.Scope == scopeVersion {
			return "Read the Bible online — free, no ads, no account."
		}
		// The same split the lede makes, for the same reason: telling an unfurl
		// that Daniel isn't in the WEB would be false, and the preview is what
		// most recipients see first.
		if n.OwnLastChapter > 0 {
			return fmt.Sprintf("The %s's %s ends at chapter %d. Read %s in the %s — free, no ads, "+
				"no account.", n.VersionName, n.Book, n.OwnLastChapter, n.ref(), other)
		}
		return fmt.Sprintf("%s isn't in the %s. Read %s in the %s — free, no ads, no account.",
			n.Book, n.VersionName, n.ref(), other)
	}
	switch n.Scope {
	case scopeChapter:
		return fmt.Sprintf("Shared in the %s, whose text isn't published on this site. Open %s in the "+
			"BibleText app, or read it here in another translation.", n.VersionName, n.ref())
	case scopeBook:
		return fmt.Sprintf("The %s isn't published on this site. Open %s in the BibleText app, or read "+
			"it here in another translation.", n.VersionName, n.Book)
	}
	return fmt.Sprintf("The %s isn't published on this site. Read the Bible here in three other "+
		"translations — free, no ads, no account.", n.VersionName)
}

// --- building the offers -----------------------------------------------------

// offersForChapter lists the published translations that carry this chapter,
// with the per-chapter versification verdict that decides whether the link may
// carry the shared verse.
//
// `from` is the translation the READER's link named — "nkjv" on a licensed
// page, and the page's own version on an absent-canon page (where the reference
// came from a canon that simply lacks the passage, so the numbering question is
// still asked against the version the link said).
func offersForChapter(from string, all []loadedVersion, book, slug string, chapter, upDepth int) []noticeOffer {
	up := strings.Repeat("../", upDepth)
	var out []noticeOffer
	for _, o := range all {
		if !hasChapter(o, book, chapter) {
			continue
		}
		out = append(out, noticeOffer{
			ID:   o.ID,
			Name: o.Name,
			Href: fmt.Sprintf("%s%s/%s/%d/", up, o.ID, slug, chapter),
			Diff: numberingDiff(from, o.ID, book, chapter, all),
		})
	}
	return out
}

// numberingDiff asks bibletext.ChapterNumberingDifference once per chapter per
// target.
//
// spanEnd comes from the REFERENCE translation's own copy of the chapter (the
// WEB), which the generator has loaded.
//
// WHERE THE REFERENCE HAS NO SUCH CHAPTER — the seven deuterocanonical books
// and the Greek Daniel's two extra chapters — the TARGET's own verse count
// stands in. That is sound rather than a shrug: every entry in the versification
// deltas is measured against the reference, so a book the reference does not
// contain has no entries at all and every verse of it maps straight through. The
// first cut returned "incommensurable" here, which printed a sentence about
// verses not corresponding onto /web/tobit/1/ — a page whose whole point is that
// there are no verses on this side to correspond WITH. Wrong, and worse than
// saying nothing.
func numberingDiff(from, to, book string, chapter int, all []loadedVersion) string {
	if from == to {
		return bibletext.NumberingSame
	}
	spanEnd := spanEndIn(all, versificationReferenceID, book, chapter)
	if spanEnd == 0 {
		spanEnd = spanEndIn(all, to, book, chapter)
	}
	if spanEnd == 0 {
		return bibletext.NumberingIncommensurable
	}
	return bibletext.ChapterNumberingDifference(from, to, book, chapter, spanEnd)
}

// spanEndIn is the last verse number one loaded translation gives this chapter,
// or 0 if it does not carry it.
func spanEndIn(all []loadedVersion, id, book string, chapter int) int {
	for _, v := range all {
		if v.ID != id {
			continue
		}
		last := 0
		for _, vs := range v.bible.Verses[book][chapter] {
			if vs.Verse > last {
				last = vs.Verse
			}
		}
		return last
	}
	return 0
}

// versificationReferenceID mirrors the root package's unexported
// versificationReference. Kept as its own constant rather than exported from
// there because the generator's need is narrow: it only has to know which of
// the translations it loaded is the one the deltas are measured against.
const versificationReferenceID = "web"
