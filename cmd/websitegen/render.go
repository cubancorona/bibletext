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
	fmt.Fprintf(&b, `<link rel="stylesheet" href="%sassets/reader.css">`, up)
	b.WriteString(`</head><body>`)
	b.WriteString(body)
	// Footer is identical on every page: a quiet route to the app. The default
	// href is the all-platforms landing page, so it is correct with JS off; the
	// script only narrows it to the App Store on an Apple device.
	b.WriteString(`<footer class="foot"><a id="getapp" href="https://bibletext.co.uk/">Get the BibleText app</a></footer>`)
	fmt.Fprintf(&b, `<script src="%sassets/reader.js" defer></script>`, up)
	b.WriteString(`</body></html>`)
	return b.String()
}

// renderChapter builds one chapter page — the unit of the whole site.
func renderChapter(v loadedVersion, all []loadedVersion, book, slug string, chapter, prev, next int) string {
	verses := v.bible.Verses[book][chapter]
	ref := fmt.Sprintf("%s %d", book, chapter)

	var b strings.Builder
	b.WriteString(`<div class="wrap">`)
	b.WriteString(navBar(v, all, book, slug, chapter))
	fmt.Fprintf(&b, `<h1 class="ref">%s</h1>`, template.HTMLEscapeString(ref))
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
	canonical := fmt.Sprintf("https://bibletext.co.uk/read/%s/%s/%d/", v.ID, slug, chapter)
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
	b.WriteString(`<p>`)
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
	b.WriteString(`</p>`)
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
// to the SAME book and chapter in each other version, so switching keeps your
// place — matching the app.
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
	return b.String()
}

func hasChapter(v loadedVersion, book string, chapter int) bool {
	return len(v.bible.Verses[book][chapter]) > 0
}

func renderBookList(v loadedVersion, all []loadedVersion) string {
	var b strings.Builder
	b.WriteString(`<div class="wrap"><nav class="top"><a class="home" href="../">BibleText</a><span class="vers">`)
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
		"https://bibletext.co.uk/read/"+v.ID+"/", b.String(), 1)
}

func renderChapterList(v loadedVersion, book, slug string, chapters []int) string {
	var b strings.Builder
	b.WriteString(`<div class="wrap"><nav class="top"><a class="home" href="../../">BibleText</a><span class="crumbs">`)
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
		"https://bibletext.co.uk/read/"+v.ID+"/"+slug+"/", b.String(), 2)
}

// renderFrontDoor is /read/ — a real page (so it is never a dead end) that also
// forwards to the default version. The meta refresh works with JS disabled.
func renderFrontDoor() string {
	body := `<div class="wrap"><h1 class="ref">BibleText</h1>` +
		`<p class="ver">Read the Bible — free, no ads, no account.</p><ul class="grid">` +
		`<li><a href="web/">World English Bible</a></li>` +
		`<li><a href="bsb/">Berean Standard Bible</a></li>` +
		`<li><a href="webc/">WEB Catholic</a></li></ul></div>`
	page := pageShell("BibleText — read the Bible online", "BibleText",
		"Read the Bible online — free, no ads, no account.",
		"https://bibletext.co.uk/read/", body, 0)
	return strings.Replace(page, `<link rel="stylesheet"`,
		`<meta http-equiv="refresh" content="0; url=`+defaultVersionID+`/"><link rel="stylesheet"`, 1)
}

// writeNotFound emits the SITE-WIDE 404 at the root. GitHub Pages serves it for
// any unknown path, which is the only server-side hook a static host offers —
// so it is where a mistyped or aging link gets rescued rather than dead-ending.
// It is careful never to send a Catholic-only book to a version that lacks it.
func writeNotFound(site *siteWriter, versions []loadedVersion) error {
	body := `<div class="wrap"><h1 class="ref">Not found</h1>` +
		`<p class="ver">That page isn't here — but the whole Bible is.</p>` +
		`<p id="guess" class="guess"></p>` +
		`<ul class="grid"><li><a href="/read/web/">World English Bible</a></li>` +
		`<li><a href="/read/bsb/">Berean Standard Bible</a></li>` +
		`<li><a href="/read/webc/">WEB Catholic</a></li></ul></div>`
	page := pageShell("Not found | BibleText", "BibleText",
		"Read the Bible online — free, no ads, no account.", "", body, 0)
	// The 404 lives at the site root, so its asset paths must be absolute.
	page = strings.ReplaceAll(page, `href="assets/`, `href="/read/assets/`)
	page = strings.ReplaceAll(page, `src="assets/`, `src="/read/assets/`)
	return site.write("404.html", page)
}
