package main

// The web reader's half of the tint model.
//
// It is the fifth surface, and the only one that cannot call chapterTint: the
// pages are generated once and served static, so nothing here knows which
// verses a reader will arrive wanting lit. reader.js decides that at read time
// from the URL fragment, and it decides it by adding a CLASS to the per-verse
// carrier this file's markup emits.
//
// So the site shares the app's tint VOCABULARY rather than its tint answer, and
// these tests pin both halves of that:
//
//  1. The carrier. Every verse gets its own <span class="v" id="vN">, which is
//     what makes a per-verse tint expressible at all. Losing it (flowing a
//     paragraph as one blob) would make the site the one surface that could
//     never show a second tint.
//  2. The vocabulary. The class the app's emitter stamps
//     (bibletext.HighlightTintClass) is the class this site's CSS styles and
//     its JS adds. Rename it in tint.go and this fails here — which is the only
//     coupling available, since the site's copy is hand-written CSS and JS, not
//     generated markup.
//
// Plus a byte-for-byte pin on chapterBody, so the S3 refactor can be shown to
// have moved nothing on this surface either.

import (
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// TestChapterBodyIsByteIdenticalAcrossTheTintRefactor is the web arm of the S3
// golden. The expected string was captured from the shipping generator BEFORE
// the tint seam existed; it is written out in full rather than recomputed, so
// the test cannot agree with a change by being derived from it.
func TestChapterBodyIsByteIdenticalAcrossTheTintRefactor(t *testing.T) {
	verses := []bibletext.Verse{
		v("Romans", 8, 1, "Therefore, there is now no condemnation."),
		v("Romans", 8, 2, "For the law of the Spirit set you free."),
		v("Romans", 8, 3, "For what the law was powerless to do."),
	}
	const want = `<p>` +
		`<span class="v" id="v1"><sup class="n"><a href="#v1">1</a></sup>&nbsp;Therefore, there is now no condemnation.</span>` +
		` <span class="v" id="v2"><sup class="n"><a href="#v2">2</a></sup>&nbsp;For the law of the Spirit set you free.</span>` +
		` <span class="v" id="v3"><sup class="n"><a href="#v3">3</a></sup>&nbsp;For what the law was powerless to do.</span>` +
		`</p>`
	if got := chapterBody("web", "Romans", verses); got != want {
		t.Errorf("chapter markup moved.\n got: %s\nwant: %s", got, want)
	}
}

// The per-verse carrier is what a tint attaches to. Without one span per verse
// there is nowhere for reader.js to put a class, and a second tint could never
// be shown on this surface at all.
func TestEveryVerseGetsItsOwnTintCarrier(t *testing.T) {
	verses := []bibletext.Verse{
		v("Romans", 8, 1, "Therefore, there is now no condemnation."),
		v("Romans", 8, 2, "For the law of the Spirit set you free."),
	}
	got := chapterBody("web", "Romans", verses)
	for _, want := range []string{`<span class="v" id="v1">`, `<span class="v" id="v2">`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing per-verse tint carrier %s in:\n%s", want, got)
		}
	}
}

// The site and the app must name the wash the same thing, or a shared link
// lights differently in the browser than it does in the app that sent it.
func TestSiteUsesTheAppsTintClass(t *testing.T) {
	cls := bibletext.HighlightTintClass()
	if cls == "" {
		t.Fatal("the highlight tint must name a class — the static reader has no other way to paint it")
	}

	css := readerCSS("x.woff2", "y.woff2")
	if !strings.Contains(css, ".v:target,.v."+cls+"{") {
		t.Errorf("reader.css does not style the app's tint class %q — a highlighted range would render unlit", cls)
	}

	// The JS adds and removes it by name. Both directions matter: a mismatch on
	// the remove side leaves a stale band lit after the reader clears it.
	js := readerJSTemplate
	for _, want := range []string{
		`classList.add('` + cls + `')`,
		`classList.remove('` + cls + `')`,
		`querySelectorAll('.v.` + cls + `')`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("reader.js does not use the app's tint class: expected %s", want)
		}
	}
}
