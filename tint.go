package bibletext

// WHAT A VERSE'S BACKGROUND SAYS — one answer, five renderers.
//
// Until this file existed, "is this verse washed?" was asked SEPARATELY inside
// each renderer, as a bool, by calling isVerseHighlighted(state, verse):
// buildChapterHTML asked it five times per verse, buildChapterHTMLAndroid five
// more, the styled desktop layout once, the RichText fallback once, and
// chapterText.rewrap hand-rolled the same comparison off markSpan without
// calling it at all. Six copies of one rule, in four dialects of markup, two of
// them behind cgo.
//
// That shape is fine while the answer is a bool. It stops being fine the moment
// the answer is a VALUE — which is the whole notes rework: a verse can carry "a
// note is here", "another note is here too", or "more than one note is here"
// ([redacted-retired-private-reference], the tint table). With the rule copied six times,
// adding a second tint is six simultaneous drawing changes on a subsystem whose
// recent history is a defect per commit. With ONE function answering it, adding
// a second tint is a change to THIS file and nothing else: every renderer
// already switches on a tint value it was handed.
//
// So this file is deliberately dull. chapterTint returns exactly one tint today
// — tintHighlight, over the verses the mark covers — and every surface's output
// is byte-for-byte what it was (testdata/chapter_tint_golden.txt). The point of
// the step is that it is invisible.
//
// WHY NOT mark.go. mark.go is the SOURCE: who lit the verse and where they
// pointed, an ownership record that exists because ownership used to be guessed
// (X10). This is the PRESENTATION: what a renderer paints, which today derives
// from the mark and tomorrow also derives from the notes on the chapter. Those
// are different questions with different readers — nothing here needs an
// hlOrigin, and nothing in mark.go should have to know a palette — so mixing
// them would put the two halves of the very confusion mark.go was written to
// end back into one file.
//
// WHY NOT reading_styled_layout.go, where verseTint started. It was correct
// there while the type was pane-local. It is now the vocabulary five surfaces
// speak, and a Windows/Linux layout engine is not where the iOS HTML builder
// should be reaching for its enum.

import (
	"fmt"
	"image/color"
	"strings"
)

// washIsLiveMutation — whether a wash change on THIS build's reading pane is a
// live attribute mutation (Apple panes: arrivals must declare forceReposition)
// or a re-render that carries its own scroll (everywhere else). A var over the
// platform constant washIsLiveMutationOnPlatform (reading_tint_apple.go /
// reading_tint_other.go — the nativeNoteSticker arrangement, notes_banner.go):
// nothing in the app assigns it; the platform-mimic dev mode (dev_mimic_on.go)
// sets it false so goToVerseRange / next-note arrivals take exactly the
// Windows/Linux branch on a darwin host, and tests can pin either answer.
var washIsLiveMutation = washIsLiveMutationOnPlatform

// verseTint is the wash a verse's runs carry, per verse — NOT a bool, so a
// renderer paints one rect per contiguous SAME-TINT stretch and two verses
// tinted differently never share a rectangle. Today exactly one tint is in use;
// the type is the seam the notes rework needs ("this verse has a note" vs "more
// than one note here"), which is a distinction a bool cannot carry. The zero
// value is deliberately "no wash", so an untinted run needs no initialisation.
type verseTint uint8

const (
	tintNone      verseTint = iota // no wash
	tintHighlight                  // the search / cross-ref / mark band
	tintReadAlong                  // the verse the narration is currently on

	// tintMulti — MORE THAN ONE note covers this verse ([redacted-retired-private-reference],
	// the tint table). Fully wired through the tables below and the palette
	// (HighlightMulti, theme.go — a deliberate hue-separated pair, light
	// #C7DBF5 / dark #2E3E5C), and deliberately UNREACHABLE: chapterTint never
	// returns it, because one lit span at a time is the recorded invariant.
	// tint_multi_guard_test.go holds that line — wiring a code path to this
	// constant is a deliberate decision, not a side effect. When the behaviour
	// is requested, chapterTint (and only chapterTint) widens.
	tintMulti

	// tintCount bounds the markup tables below. It is the last constant on
	// purpose: adding a tint above it grows the tables, and a table built by
	// range-over-tintCount cannot forget the new row.
	tintCount
)

// overridesTextColour reports whether runs under this wash are drawn in the
// body colour instead of their own.
//
// An explicit choice PER TINT, deliberately not "any tint but none". The
// highlight band is a strong, chosen mark and a verse number's muted slate
// disappears into it, so under that band the numbers take the body colour. A
// tint added later must decide for itself: written as `!= tintNone`, the notes
// rework's "this verse has a note" would have drained the colour out of every
// verse number it touched, with no code change and no failing test to say so.
func (t verseTint) overridesTextColour() bool {
	switch t {
	case tintHighlight:
		return true
	case tintMulti:
		// The same decision as tintHighlight, for the same reason: this wash is
		// a strong, chosen band, and the verse numbers' muted slate disappears
		// into it — under the band the numbers take the body colour.
		return true
	default:
		return false
	}
}

// wash is the colour this tint paints, and whether it paints at all.
//
// ONE table for three surfaces: the styled pane's rectangles
// (styledPaneRenderer.tintColor), Android's inline background-color, and — via
// nrgbaToHex — the .hl rule in the Apple dialect's stylesheet. They used to
// reach for pal.Highlight independently, which is how a fourth surface would
// have got a slightly different gold.
//
// tintReadAlong is absent ON PURPOSE and answers false. The narration wash is
// not a chapter tint: it lives on the pane (styledReadAlongTint), it is
// translucent because it is meant to LAYER over whatever is underneath, and on
// the Apple panes it is a live range mutation rather than a rebuild. Giving it
// an opaque palette colour here would invite a renderer to treat it as one.
func (t verseTint) wash(pal palette) (color.NRGBA, bool) {
	switch t {
	case tintHighlight:
		return pal.Highlight, true
	case tintMulti:
		return pal.HighlightMulti, true
	default:
		return color.NRGBA{}, false
	}
}

// htmlClass is the CSS class the HTML surfaces stamp on a tinted run, and "" for
// a tint that stamps none.
//
// The name is short because it is in every published page: cmd/websitegen's
// reader.js adds this same ".hl" client-side to ~3,900 static chapters, and
// cmd/websitegen/chapter_tint_test.go asserts the site and this table still
// agree. Rename it here and that test fails there — which is the only way the
// two halves of the web reader can be kept honest, since the site's copy is
// hand-written CSS and JS rather than generated markup.
// markupFor is the guarded table lookup the emitters use.
//
// The tables are BUILT over 0..tintCount, which bounds construction and not
// lookup — and the value handed in comes from chapterTint, the one function
// this whole seam exists to let the notes rework widen. An index straight into
// the array would turn "somebody added a tint and missed a table" into a panic
// on the iOS and Android render paths. Falling back to the untinted row keeps
// the documented failure mode: a new wash that is not fully wired renders as
// nothing, rather than taking the reading page down.
func markupFor(table []tintHTML, t verseTint) tintHTML {
	if int(t) < 0 || int(t) >= len(table) {
		return table[tintNone]
	}
	return table[t]
}

func (t verseTint) htmlClass() string {
	switch t {
	case tintHighlight:
		return "hl"
	case tintMulti:
		// Same grammar as "hl", one letter of meaning: multi. Every class named
		// here gets its stylesheet rule from the same table row (tintHTML.CSS),
		// so the class cannot exist without its rule — the rule rides in every
		// Apple-dialect page from the day the class is named, which is why the
		// chapter-tint golden carries an .hlm rule no markup references yet.
		return "hlm"
	default:
		return ""
	}
}

// --- The markup a tint stamps, per HTML dialect, as a table. ---------------
//
// tintHTML is one tint's four pieces of markup in one dialect. Each is a
// COMPLETE format taking exactly ONE argument, and that is a measured
// requirement, not a style: written the obvious way — a shared format with the
// class or the colour passed as an extra %s — a highlighted 176-verse psalm
// paid 79 more allocations per render than the hand-written literals it
// replaced (measured; every extra fmt argument is an interface box). Baking the
// class and the colour into the format at table-build time puts that back to
// zero while keeping the tint, not the renderer, the thing that decides what a
// wash looks like.
//
// An empty Body means "write the text bare" — the untinted, un-reddened case,
// which must stay a WriteString rather than become a one-argument Fprintf, or
// the UNMARKED render (much the commoner one) grows an allocation per verse.
type tintHTML struct {
	JoinSpace string // complete markup for the space joining two same-tint verses
	Number    string // ONE %d — the verse number and whatever wash it sits in
	Body      string // ONE %s — verse text under this tint; "" = write it bare
	BodyWJ    string // ONE %s — verse text that is ALSO the words of Christ

	// CSS is this tint's stylesheet rule, ONE %s for its wash colour, and ""
	// for a tint that needs none. Class dialects only — the Android dialect
	// has no stylesheet, which is the whole reason it inlines colours.
	//
	// The rule and the markup that references it are one row, so a tint cannot
	// be given a class the stylesheet never defines. That is exactly the shape
	// of the failure this whole file guards against: a second wash that emits
	// correctly and renders as nothing.
	CSS string
}

// appleTintHTML is the class-based dialect (iOS + macOS: buildChapterHTML).
// Built once at init because a CSS class does not depend on the palette — the
// colours live in the stylesheet, which is why this dialect can have a table at
// all while Android's has to be rebuilt per render.
var appleTintHTML = func() [tintCount]tintHTML {
	var t [tintCount]tintHTML
	for tint := verseTint(0); tint < tintCount; tint++ {
		cls := tint.htmlClass()
		if cls == "" {
			// No wash: the number carries no span, the body is bare, and red
			// letters get the colour class alone.
			t[tint] = tintHTML{
				Number: `<sup class="v">%d</sup>&nbsp;`,
				BodyWJ: `<span class="wj">%s</span>`,
			}
			continue // and no CSS rule: there is no wash to define
		}
		// The verse NUMBER and the &nbsp; after it join the wash: leaving them
		// out punched a pale hole through the middle of the band. And BodyWJ
		// carries BOTH classes — .hl is only a background and .wj only a
		// colour, so together they paint red letters ON the wash. That pairing
		// is load-bearing: it used to be an either/or whose highlight arm won,
		// and a highlighted words-of-Christ verse silently lost its red (John
		// 11:25 under the band while v26 beside it stayed red).
		t[tint] = tintHTML{
			JoinSpace: fmt.Sprintf(`<span class="%s"> </span>`, cls),
			Number:    fmt.Sprintf(`<sup class="v %s">%%d</sup><span class="%s">&nbsp;</span>`, cls, cls),
			Body:      fmt.Sprintf(`<span class="%s">%%s</span>`, cls),
			BodyWJ:    fmt.Sprintf(`<span class="%s wj">%%s</span>`, cls),
			// NO font-weight, and NO colour override. Bold Georgia sets ~17%%
			// wider than the regular face, so a wash that bolded re-wrapped the
			// paragraph and the text jumped when it cleared; and recolouring
			// the run threw away the red letters exactly where a reader is most
			// likely to be looking. Leaving colour out here is what lets .wj
			// ride alongside — necessary, and not sufficient without BodyWJ.
			CSS: fmt.Sprintf(`.%s {
		background-color: %%s;
		padding: 0 2px;
		border-radius: 2px;
	}`, cls),
		}
	}
	return t
}()

// androidTintHTML is the Html.fromHtml-safe dialect (buildChapterHTMLAndroid):
// no classes, because fromHtml ignores <style>, so every wash is an inline
// background-color and every colour an explicit <font>.
//
// Rebuilt per render rather than at init: its markup contains PALETTE COLOURS,
// which change with the theme. Four Sprintfs per live tint, once, against the
// hundreds of verses that then cost one argument each.
//
// NO <b> anywhere, deliberately. Bolding a washed verse re-typesets it (the
// bold serif sets ~17% wider), so the paragraph re-wrapped and the text jumped
// the moment the wash cleared — the same refusal the Apple dialect and the
// styled pane make.
func androidTintHTML(pal palette, numHex, redHex string) [tintCount]tintHTML {
	var t [tintCount]tintHTML
	for tint := verseTint(0); tint < tintCount; tint++ {
		c, ok := tint.wash(pal)
		if !ok {
			t[tint] = tintHTML{
				Number: fmt.Sprintf(`<sup><small><font color="%s"><b>%%d</b></font></small></sup>&nbsp;`, numHex),
				BodyWJ: fmt.Sprintf(`<font color="%s">%%s</font>`, redHex),
			}
			continue
		}
		bg := nrgbaToHex(c)
		// The <font color> nests INSIDE the background span: Html.fromHtml turns
		// them into two independent spans over the same range, so the inner
		// colour keeps the outer band. That is the Android twin of the .hl/.wj
		// pairing above, and it was missing for the same reason and cost the
		// same red.
		t[tint] = tintHTML{
			JoinSpace: fmt.Sprintf(`<span style="background-color:%s"> </span>`, bg),
			Number: fmt.Sprintf(
				`<span style="background-color:%s"><sup><small><font color="%s"><b>%%d</b></font></small></sup>&nbsp;</span>`,
				bg, numHex),
			Body: fmt.Sprintf(`<span style="background-color:%s">%%s</span>`, bg),
			BodyWJ: fmt.Sprintf(
				`<span style="background-color:%s"><font color="%s">%%s</font></span>`, bg, redHex),
		}
	}
	return t
}

// writeTintedHTML writes one stretch of verse text under a tint, in either HTML
// dialect. Both dialects reduce to the same three-way choice, so they share it.
//
// BodyWJ covers both red cases — red under a wash and red on bare paper —
// because a tint's row already knows which of those it is. That is what stops
// the wash and the red becoming an either/or again: the arm where a washed
// words-of-Christ verse lost its colour did not exist in either dialect until
// it surfaced on John 11:25, and there is now no way to write a wash arm
// without also having written the red-under-wash one, because they are the same
// row of the same table.
func writeTintedHTML(b *strings.Builder, mk tintHTML, red bool, text string) {
	switch {
	case red:
		fmt.Fprintf(b, mk.BodyWJ, text)
	case mk.Body != "":
		fmt.Fprintf(b, mk.Body, text)
	default:
		b.WriteString(text) // no wash, not red: bare text, and no allocation
	}
}

// chapterTints is the answer to "what tint does each verse of this chapter
// carry?", computed once per render and asked per verse.
//
// A VALUE, not a map and not a slice, and that is a measured choice rather than
// a stylistic one. This runs on the iOS chapter-rebuild path, which
// chapterRenderFingerprint exists to avoid entering; the shape that survives
// that path is the one that never reaches the heap. A map[int]verseTint over a
// 176-verse psalm is one allocation plus a bucket array plus a hash per lookup,
// for a fact that at k=1 is a single span. So the k=1 answer is stored as the
// span itself: chapterTint allocates nothing, and asking it costs a string
// compare and two integer compares — cheaper than the isVerseHighlighted it
// replaces, which re-fetched the mark from AppState on every verse.
//
// When k > 1 lands, the fields here become the flattened []tintRun the design
// calls for (disjoint, ascending, at most 2k-1 runs) and `of` scans them. No
// call site changes: they already ask a chapter for a verse's tint and switch on
// the value they get.
type chapterTints struct {
	// at is the span the tint covers, IN THE NUMBERING OF THE VERSES BEING
	// DRAWN. It carries book and chapter, and `of` checks them against each
	// verse rather than against AppState's current location, because the two
	// are not the same question: a renderer can be handed verses for a chapter
	// the reader is not standing on (a share preview, a chapter built ahead of
	// the navigation that will show it), and isVerseHighlighted always tested
	// the VERSE. Scoping the tint to AppState.CurrentChapter instead would light
	// the wrong passage there, silently, and no golden would catch it because
	// no golden would have thought to render that case.
	at   VerseSpan
	tint verseTint
}

// chapterTint is THE tint source. Every renderer calls this once and asks it per
// verse; none of them reads the mark directly any more.
//
// Today it flattens exactly one thing — the mark — into exactly one tint, which
// is why every surface still emits what it emitted before. The notes rework
// widens this function and only this function.
func chapterTint(state *AppState) chapterTints {
	sp, ok := state.markSpan()
	if !ok {
		return chapterTints{}
	}
	return chapterTints{at: sp, tint: tintHighlight}
}

// of returns the tint for one verse. The zero chapterTints answers tintNone for
// everything, so "nothing is marked" needs no special case at any call site.
func (t chapterTints) of(v Verse) verseTint {
	if t.tint == tintNone {
		return tintNone
	}
	if !t.at.sameChapter(v.BookName, v.Chapter) || !t.at.covers(v.Verse) {
		return tintNone
	}
	return t.tint
}

// fingerprint is this tint answer's identity, folded into
// chapterRenderFingerprint (reading.go) so the native overlays can skip a
// rebuild that would change nothing.
//
// Two properties matter, and only one of them is symmetric.
//
// NECESSARY: if what any verse would be painted changes, this string must
// change. It does, because every tint here derives from the span and the span is
// written out whole. Missing that direction is the expensive one — it costs a
// silently stale pane: the reader taps "clear highlight" and the wash stays
// (reading.go already records that exact failure for the note clause).
//
// NOT SUFFICIENT, deliberately: a mark on ANOTHER chapter tints nothing here and
// still changes this string. That is a rebuild for no visible change, and it is
// the cheap side of the trade — an occasional extra HTML rebuild versus reading
// the whole chapter's verse list to prove the mark misses it, on the path whose
// entire purpose is not doing work. It is also nearly unreachable in practice:
// the mark and the chapter on screen move together.
//
// The SPAN, not the origin. Two marks on the same verses render identically
// whoever placed them — what differs between a note's mark and a search's is the
// note bubble, and reading.go folds the note separately. Adding the origin here
// would cost a full NSAttributedString re-import for a change no pixel reflects.
// NORMALISED, not the raw span. A single verse is spelled two ways by the
// callers that set marks — {Lo: 16, Hi: 0} and {Lo: 16, Hi: 16} — and covers()
// treats them as the same one verse, so they paint the same chapter and must
// fold to the same string. Written raw they did not, and arriving at a verse
// from a caller that spells it the other way cost a full rebuild for a render
// that was already on screen. A span that covers nothing at all (Lo <= 0, the
// chapter-level mark) folds to "0" for the same reason: it paints exactly what
// no mark paints.
func (t chapterTints) fingerprint() string {
	if t.tint == tintNone || t.at.Lo <= 0 {
		return "0"
	}
	hi := t.at.Hi
	if hi <= t.at.Lo {
		hi = t.at.Lo
	}
	return fmt.Sprintf("%s:%d:%d-%d:%d", t.at.Book, t.at.Chapter, t.at.Lo, hi, t.tint)
}

// joins reports whether the boundary between two adjacent verses is INSIDE a
// wash — the rule that keeps a highlighted range one continuous band instead of
// a band notched at every join (reading_highlight_gap_test.go).
//
// Same tint on both sides, not merely "both tinted". At k=1 those are the same
// test because there is only one tint; when there are two, a join between a
// verse under one wash and a verse under another belongs to neither, and drawing
// it in either colour would extend one note's band over the other's first word.
func (t chapterTints) joins(prev, cur Verse) bool {
	a := t.of(prev)
	return a != tintNone && a == t.of(cur)
}
