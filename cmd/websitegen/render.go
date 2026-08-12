package main

// Page rendering. Every page is finished HTML — no client-side routing, no
// hydration, no framework. A shared link therefore paints scripture on the
// first response, which is the whole point: the recipient is usually on a phone
// tapping a link in a message.

import (
	"fmt"
	"html/template"
	"strings"

	bibletext "bibletext"
)

// pageShell wraps body content in the common document. ogDesc is plain text.
func pageShell(title, ogTitle, ogDesc, canonical, body string, depth int) string {
	up := strings.Repeat("../", depth)
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1">`)
	fmt.Fprintf(&b, `<title>%s</title>`, template.HTMLEscapeString(title))
	// Open Graph tags must be in the served HTML: link unfurlers (iMessage,
	// WhatsApp, Slack) do not run JavaScript, so a per-chapter preview is only
	// possible because every chapter is its own pre-rendered file.
	fmt.Fprintf(&b, `<meta property="og:title" content="%s">`, template.HTMLEscapeString(ogTitle))
	fmt.Fprintf(&b, `<meta property="og:description" content="%s">`, template.HTMLEscapeString(ogDesc))
	fmt.Fprintf(&b, `<meta name="description" content="%s">`, template.HTMLEscapeString(ogDesc))
	b.WriteString(`<meta property="og:type" content="article">`)
	b.WriteString(`<meta property="og:site_name" content="BibleText">`)
	if canonical != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`, template.HTMLEscapeString(canonical))
	}
	fmt.Fprintf(&b, `<link rel="stylesheet" href="%s%s">`, up, cssName)
	b.WriteString(`</head><body>`)
	b.WriteString(body)
	// Footer is identical on every page: a quiet route to the app. The default
	// href is the all-platforms landing page, so it is correct with JS off; the
	// script only narrows it to the App Store on an Apple device.
	b.WriteString(`<footer class="foot"><a id="getapp" href="https://bibletext.co.uk/">Get the BibleText app</a>` +
		platformIcons + `</footer>`)
	fmt.Fprintf(&b, `<script src="%s%s" defer></script>`, up, jsName)
	b.WriteString(`</body></html>`)
	return b.String()
}

// platformIcons is the row of platform marks under the footer link. Inline SVG
// rather than an icon font or image files: no extra request, no external host
// (the site has a strict no-dependency shape), and they inherit currentColor so
// one copy serves both themes. Every glyph is a simple silhouette drawn here,
// not a vendor logo asset.
const platformIcons = `<div class="plats">` +
	// Apple
	`<a href="https://apps.apple.com/app/id6784567351" title="iPhone, iPad and Mac" aria-label="Apple">` +
	`<svg viewBox="0 0 24 24" role="img"><path d="M16.4 12.8c0-2.2 1.8-3.3 1.9-3.3-1-1.5-2.6-1.7-3.2-1.7-1.4-.1-2.7.8-3.3.8-.7 0-1.7-.8-2.8-.8-1.5 0-2.8.8-3.6 2.1-1.5 2.6-.4 6.5 1.1 8.6.7 1 1.6 2.2 2.7 2.2 1.1 0 1.5-.7 2.8-.7 1.3 0 1.6.7 2.8.7 1.2 0 1.9-1 2.6-2.1.8-1.2 1.2-2.4 1.2-2.5 0 0-2.2-.9-2.2-3.3zM14.3 5.8c.6-.7 1-1.7.9-2.7-.9 0-2 .6-2.6 1.3-.6.6-1.1 1.7-.9 2.7 1 .1 2-.5 2.6-1.3z"/></svg></a>` +
	// Android
	`<a href="https://bibletext.co.uk/" title="Android" aria-label="Android">` +
	`<svg viewBox="0 0 24 24" role="img"><path d="M6 9.5v7.2c0 .5.4.9.9.9h.8v2.6c0 .7.6 1.3 1.3 1.3s1.3-.6 1.3-1.3v-2.6h3.4v2.6c0 .7.6 1.3 1.3 1.3s1.3-.6 1.3-1.3v-2.6h.8c.5 0 .9-.4.9-.9V9.5H6zm-1.6 0c-.7 0-1.3.6-1.3 1.3v4.8c0 .7.6 1.3 1.3 1.3s1.3-.6 1.3-1.3v-4.8c0-.7-.6-1.3-1.3-1.3zm15.2 0c-.7 0-1.3.6-1.3 1.3v4.8c0 .7.6 1.3 1.3 1.3s1.3-.6 1.3-1.3v-4.8c0-.7-.6-1.3-1.3-1.3zM15.6 3.6l.9-1.6c.1-.1 0-.3-.1-.4-.1-.1-.3 0-.4.1l-.9 1.6c-.8-.3-1.6-.5-2.5-.5h-.2c-.9 0-1.7.2-2.5.5l-.9-1.6c-.1-.1-.2-.2-.4-.1-.1.1-.2.2-.1.4l.9 1.6C7.6 4.6 6.3 6.3 6 8.4h12c-.3-2.1-1.6-3.8-2.4-4.8zM9.6 6.6c-.3 0-.5-.2-.5-.5s.2-.5.5-.5.5.2.5.5-.2.5-.5.5zm4.8 0c-.3 0-.5-.2-.5-.5s.2-.5.5-.5.5.2.5.5-.2.5-.5.5z"/></svg></a>` +
	// Windows
	`<a href="https://bibletext.co.uk/" title="Windows" aria-label="Windows">` +
	`<svg viewBox="0 0 24 24" role="img"><path d="M3 5.7l7.3-1v7H3v-6zM11.2 4.6L21 3.2v8.4h-9.8v-7zM3 13.6h7.3v7L3 19.6v-6zM11.2 13.6H21V22l-9.8-1.4v-7z"/></svg></a>` +
	// Linux — a plain Tux silhouette. Redrawn after the first attempt read as a
	// blob at 17px: at this size the shape has to carry head, beak and feet, and
	// nothing else survives.
	`<a href="https://bibletext.co.uk/" title="Linux" aria-label="Linux">` +
	`<svg viewBox="0 0 24 24" role="img"><path d="M12 2C10.1 2 8.6 3.5 8.6 5.4v2C7.1 8.8 6 11 6 13.5c0 1.9.6 3.6 1.7 4.8l-1.2 2.1c-.3.5.1 1.1.7 1.1h9.6c.6 0 1-.6.7-1.1l-1.2-2.1c1.1-1.2 1.7-2.9 1.7-4.8 0-2.5-1.1-4.7-2.6-6.1v-2C15.4 3.5 13.9 2 12 2zm-1.4 3c.4 0 .8.4.8 1s-.4 1-.8 1-.8-.4-.8-1 .4-1 .8-1zm2.8 0c.4 0 .8.4.8 1s-.4 1-.8 1-.8-.4-.8-1 .4-1 .8-1zM12 7.7c.9 0 1.7.5 1.7 1 0 .4-.8 1-1.7 1s-1.7-.6-1.7-1c0-.5.8-1 1.7-1z"/></svg></a>` +
	`</div>`

// renderChapter builds one chapter page — the unit of the whole site.
func renderChapter(v loadedVersion, all []loadedVersion, book, slug string, chapter, prev, next int) string {
	verses := v.bible.Verses[book][chapter]
	ref := fmt.Sprintf("%s %d", book, chapter)

	var b strings.Builder
	b.WriteString(`<div class="wrap">`)
	b.WriteString(navBar(v, all, book, slug, chapter))
	// Heading row: title left, quiet prev/next arrows right — the app's shape.
	// Arrows only here; the labelled pager at the foot does the wordy version.
	b.WriteString(`<div class="chapbar">`)
	fmt.Fprintf(&b, `<h1 class="ref">%s</h1><div class="chapnav">`, template.HTMLEscapeString(ref))
	b.WriteString(arrowLink("&larr;", "prev", book, prev))
	b.WriteString(arrowLink("&rarr;", "next", book, next))
	b.WriteString(`</div></div>`)
	fmt.Fprintf(&b, `<p class="ver">%s</p>`, template.HTMLEscapeString(v.Name))
	b.WriteString(`<article class="text">`)
	b.WriteString(chapterBody(book, verses))
	b.WriteString(`</article>`)

	// Prev/next keep the reader moving without going back to an index.
	b.WriteString(`<nav class="pager">`)
	if prev > 0 {
		fmt.Fprintf(&b, `<a rel="prev" href="../%d/">&larr; %s %d</a>`, prev, template.HTMLEscapeString(book), prev)
	} else {
		b.WriteString(`<span></span>`)
	}
	if next > 0 {
		fmt.Fprintf(&b, `<a rel="next" href="../%d/">%s %d &rarr;</a>`, next, template.HTMLEscapeString(book), next)
	} else {
		b.WriteString(`<span></span>`)
	}
	b.WriteString(`</nav></div>`)

	title := fmt.Sprintf("%s — %s | BibleText", ref, v.Name)
	canonical := fmt.Sprintf("https://bibletext.co.uk/%s/%s/%d/", v.ID, slug, chapter)
	return pageShell(title, fmt.Sprintf("%s (%s)", ref, v.Name), chapterPreview(verses), canonical,
		b.String(), 3)
}

// chapterBody renders verses into paragraphs using the app's own rules: a join
// touching a poetic verse is a line break (PoeticJoin), authored "\n" inside a
// verse becomes <br>, and words of Christ get the red-letter class. Verse ids
// are what make #v16 work with no JavaScript at all.
func chapterBody(book string, verses []bibletext.Verse) string {
	if len(verses) == 0 {
		return `<p class="empty">This chapter is not available in this translation.</p>`
	}
	var b strings.Builder
	// Paragraphs come from the APP's rule, not a web-specific one, so the page
	// breaks where the reading pane breaks. A paragraph that OPENS with a poetic
	// verse is marked .pm: the app skips its reporter indent and sets it ragged
	// rather than justified, because a psalm is lines, not prose.
	for _, para := range bibletext.GroupVersesIntoParagraphs(verses) {
		if len(para) == 0 {
			continue
		}
		if bibletext.VerseIsPoetic(para[0].Text) {
			b.WriteString(`<p class="pm">`)
		} else {
			b.WriteString(`<p>`)
		}
		b.WriteString(paragraphBody(book, para))
		b.WriteString(`</p>`)
	}
	return b.String()
}

// paragraphBody renders the verses of one paragraph, joining them the way the
// app does: a join touching a poetic verse is a line break, everything else is
// a space.
func paragraphBody(book string, verses []bibletext.Verse) string {
	var b strings.Builder
	for i, v := range verses {
		if i > 0 {
			if bibletext.PoeticJoin(verses[i-1].Text, v.Text) {
				b.WriteString(`<br>`)
			} else {
				b.WriteByte(' ')
			}
		}
		fmt.Fprintf(&b, `<span class="v" id="v%d">`, v.Verse)
		fmt.Fprintf(&b, `<sup class="n"><a href="#v%d">%d</a></sup>&nbsp;`, v.Verse, v.Verse)
		// Escape first, then turn authored poem breaks into real <br> — a bare
		// "\n" is only whitespace in HTML and would flatten a psalm to prose.
		body := strings.ReplaceAll(template.HTMLEscapeString(strings.TrimSpace(v.Text)), "\n", "<br>")
		if bibletext.IsWordsOfChrist(book, v.Chapter, v.Verse) {
			fmt.Fprintf(&b, `<span class="wj">%s</span>`, body)
		} else {
			b.WriteString(body)
		}
		b.WriteString(`</span>`)
	}
	return b.String()
}

// chapterPreview is the unfurl text: the opening of the chapter, trimmed to a
// sentence-ish length so a shared link reads well in a message thread.
func chapterPreview(verses []bibletext.Verse) string {
	var b strings.Builder
	for _, v := range verses {
		if b.Len() > 180 {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strings.Join(strings.Fields(v.Text), " "))
	}
	s := b.String()
	if len(s) > 200 {
		if cut := strings.LastIndexByte(s[:200], ' '); cut > 0 {
			s = s[:cut]
		} else {
			s = s[:200]
		}
		s += "…"
	}
	return s
}

// navBar carries the version switcher and the trail back up. The switcher links
// to the same book and chapter in each other version, and reader.js appends the
// current verse fragment at runtime (carryVerse) so switching keeps your place —
// matching the app. The fragment cannot be baked in here: it is not known until
// someone opens the page with a verse in the URL.
func navBar(v loadedVersion, all []loadedVersion, book, slug string, chapter int) string {
	var b strings.Builder
	b.WriteString(`<nav class="top"><a class="home" href="../../../">BibleText</a><span class="crumbs">`)
	fmt.Fprintf(&b, `<a href="../../">%s</a> <span class="sep">/</span> <a href="../">%s</a>`,
		template.HTMLEscapeString(strings.ToUpper(v.ID)), template.HTMLEscapeString(book))
	b.WriteString(`</span><span class="vers">`)
	for _, other := range all {
		if !hasChapter(other, book, chapter) {
			continue
		}
		cls := "vpick"
		if other.ID == v.ID {
			cls = "vpick on"
		}
		otherSlug, _ := bibletext.BookSlug(book)
		fmt.Fprintf(&b, `<a class="%s" title="%s" href="../../../%s/%s/%d/">%s</a>`,
			cls, template.HTMLEscapeString(other.Name), other.ID, otherSlug, chapter,
			template.HTMLEscapeString(strings.ToUpper(other.ID)))
	}
	b.WriteString(`</span></nav>`)
	// "Go to" gets its own centred line under the nav row, so it competes with
	// neither the trail nor the version pills for width. It is a real link to
	// the book grid, so it works with JavaScript off; reader.js upgrades it
	// into a reference type-ahead.
	b.WriteString(`<div class="gotorow"><a class="goto" id="gotobtn" href="../../">Go to</a></div>`)
	return b.String()
}

// arrowLink is one chapter arrow. A missing neighbour stays in the layout as a
// dimmed, unclickable glyph rather than vanishing, so the pair does not jump
// sideways at the first or last chapter of a book.
func arrowLink(glyph, rel, book string, chapter int) string {
	if chapter <= 0 {
		return fmt.Sprintf(`<span class="arrow off" aria-hidden="true">%s</span>`, glyph)
	}
	return fmt.Sprintf(`<a class="arrow" rel="%s" href="../%d/" title="%s %d" aria-label="%s %d">%s</a>`,
		rel, chapter, template.HTMLEscapeString(book), chapter,
		template.HTMLEscapeString(book), chapter, glyph)
}

func hasChapter(v loadedVersion, book string, chapter int) bool {
	return len(v.bible.Verses[book][chapter]) > 0
}

func renderBookList(v loadedVersion, all []loadedVersion) string {
	var b strings.Builder
	b.WriteString(`<div class="wrap"><nav class="top plain"><a class="home" href="../">BibleText</a><span class="vers">`)
	for _, other := range all {
		cls := "vpick"
		if other.ID == v.ID {
			cls = "vpick on"
		}
		fmt.Fprintf(&b, `<a class="%s" title="%s" href="../%s/">%s</a>`,
			cls, template.HTMLEscapeString(other.Name), other.ID, strings.ToUpper(other.ID))
	}
	b.WriteString(`</span></nav>`)
	fmt.Fprintf(&b, `<h1 class="ref">%s</h1><p class="ver">Choose a book</p><ul class="grid">`,
		template.HTMLEscapeString(v.Name))
	for _, book := range v.bible.Books {
		slug, ok := bibletext.BookSlug(book)
		if !ok || len(v.bible.Verses[book]) == 0 {
			continue
		}
		fmt.Fprintf(&b, `<li><a href="%s/">%s</a></li>`, slug, template.HTMLEscapeString(book))
	}
	b.WriteString(`</ul></div>`)
	return pageShell(v.Name+" | BibleText", v.Name, "Read the "+v.Name+" — free, no ads, no account.",
		"https://bibletext.co.uk/"+v.ID+"/", b.String(), 1)
}

func renderChapterList(v loadedVersion, book, slug string, chapters []int) string {
	var b strings.Builder
	b.WriteString(`<div class="wrap"><nav class="top plain"><a class="home" href="../../">BibleText</a><span class="crumbs">`)
	fmt.Fprintf(&b, `<a href="../">%s</a>`, template.HTMLEscapeString(strings.ToUpper(v.ID)))
	b.WriteString(`</span></nav>`)
	fmt.Fprintf(&b, `<h1 class="ref">%s</h1><p class="ver">%s — choose a chapter</p><ul class="grid nums">`,
		template.HTMLEscapeString(book), template.HTMLEscapeString(v.Name))
	for _, c := range chapters {
		fmt.Fprintf(&b, `<li><a href="%d/">%d</a></li>`, c, c)
	}
	b.WriteString(`</ul></div>`)
	title := book + " — " + v.Name + " | BibleText"
	return pageShell(title, book+" ("+v.Name+")",
		fmt.Sprintf("%s has %d chapters. Read it free in the %s.", book, len(chapters), v.Name),
		"https://bibletext.co.uk/"+v.ID+"/"+slug+"/", b.String(), 2)
}

// writeNotFound emits the SITE-WIDE 404 at the root. GitHub Pages serves it for
// any unknown path, which is the only server-side hook a static host offers —
// so it is where a mistyped or aging link gets rescued rather than dead-ending.
// Today that rescue is only the three whole-Bible links below: the body is
// fixed, the `versions` argument is unused, and the #guess paragraph is left
// empty — nothing populates it. Any per-book or per-version guessing (and the
// care that would need, so a Catholic-only book is never offered under a version
// lacking it) remains to be written.
func writeNotFound(site *siteWriter, versions []loadedVersion) error {
	body := `<div class="wrap"><h1 class="ref">Not found</h1>` +
		`<p class="ver">That page isn't here — but the whole Bible is.</p>` +
		`<p id="guess" class="guess"></p>` +
		`<ul class="grid"><li><a href="/web/">World English Bible</a></li>` +
		`<li><a href="/bsb/">Berean Standard Bible</a></li>` +
		`<li><a href="/webc/">WEB Catholic</a></li></ul></div>`
	page := pageShell("Not found | BibleText", "BibleText",
		"Read the Bible online — free, no ads, no account.", "", body, 0)
	// The 404 lives at the site root, so its asset paths must be absolute.
	page = strings.ReplaceAll(page, `href="assets/`, `href="/assets/`)
	page = strings.ReplaceAll(page, `src="assets/`, `src="/assets/`)
	return site.write("404.html", page)
}
