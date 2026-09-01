package main

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	bibletext "bibletext"
)

// The site's entire client-side surface: one stylesheet and one small script.
//
// The colours come from the app's palette (theme.go) so the web page and the
// app remain recognisably the same product — warm parchment in light, warm
// near-black with a luminous sapphire accent in dark.
//
// The TYPE follows the app's split: scripture in Georgia (a system font on the
// phones and desktops that open a shared link, so it costs nothing) with the
// app's own metrics, and chrome in Atkinson Hyperlegible, which is installed
// nowhere and so ships as a ~15 KB WOFF2 subset per weight (web_fonts.go). That
// is the one webfont on the page; font-display:swap means a reader on mobile
// data sees text immediately and the chrome settles a moment later.

// readerCSS fills in the webfont URLs. They are content-hashed like the
// stylesheet itself, and since both live in assets/ the src is a bare filename.
func readerCSS(regularFile, boldFile string) string {
	light, dark := bibletext.WebReaderPalettes()
	return strings.NewReplacer(
		"__FONT_REGULAR__", regularFile,
		"__FONT_BOLD__", boldFile,
		"__LIGHT_PALETTE__", readerPaletteCSS(light, "  "),
		"__DARK_PALETTE__", readerPaletteCSS(dark, "    "),
		"__NOTE_LEAD__", strconv.Itoa(bibletext.WebNoteArrivalLeadPx()),
	).Replace(readerCSSTemplate)
}

func readerPaletteCSS(p bibletext.WebReaderPalette, indent string) string {
	return fmt.Sprintf(
		"%s--bg:%s; --surface:%s; --text:%s; --muted:%s;\n"+
			"%s--accent:%s; --border:%s; --verse:%s; --red:%s;\n"+
			"%s--verse-hl:%s; --control-hover:%s; --control-selected:%s;",
		indent, cssColor(p.Background), cssColor(p.Surface), cssColor(p.Text), cssColor(p.TextMuted),
		indent, cssColor(p.Accent), cssColor(p.Border), cssColor(p.VerseNumber), cssColor(p.RedLetter),
		indent, cssColor(p.Highlight), cssColor(p.ControlHover), cssColor(p.ControlSelection),
	)
}

func cssColor(c color.NRGBA) string {
	if c.A == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.4f)", c.R, c.G, c.B, float64(c.A)/255)
}

// readerCSSTemplate carries placeholders for content-hashed webfont filenames
// and the app-owned palettes. readerCSS fills them once the faces have been
// written; their URLs are relative to the stylesheet in the same assets/
// directory.
const readerCSSTemplate = `
/* Atkinson Hyperlegible (c) Braille Institute of America, Inc. — SIL Open Font
   License 1.1, published beside these files as assets/atkinson-OFL.txt. It is
   the app's UI typeface; swap keeps text visible while it loads. */
@font-face{
  font-family:"Atkinson Hyperlegible"; font-style:normal; font-weight:400;
  font-display:swap; src:url(__FONT_REGULAR__) format("woff2");
}
@font-face{
  font-family:"Atkinson Hyperlegible"; font-style:normal; font-weight:700;
  font-display:swap; src:url(__FONT_BOLD__) format("woff2");
}
:root{
__LIGHT_PALETTE__
  /* The app's two faces: chrome in Atkinson, scripture in Georgia. The system
     stack trails Atkinson so glyphs it lacks — the ← → of the chapter nav —
     fall back per-glyph instead of tofu. */
  --ui:"Atkinson Hyperlegible",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  --scripture:Georgia,"Iowan Old Style","Times New Roman",serif;
}
@media (prefers-color-scheme:dark){
  :root{
__DARK_PALETTE__
  }
}
*{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{
  margin:0; background:var(--bg); color:var(--text);
  /* Chrome, like the app. Scripture opts back into the serif on .text. */
  font-family:var(--ui); line-height:1.6;
  /* iOS Safari answers a double-tap with a small zoom instead of selecting the
     word under the finger — on a page of scripture, selecting is what a reader
     means. manipulation drops double-tap-to-zoom (and the old 300ms tap delay)
     while leaving pinch-zoom and scrolling untouched. */
  touch-action:manipulation;
}
.wrap{max-width:40rem; margin:0 auto; padding:1rem 1.6rem 2rem}
.top{
  display:flex; flex-wrap:wrap; align-items:center; gap:.5rem;
  font-size:.8rem; padding:.35rem 0 .55rem;
}
.top.plain{padding-bottom:.9rem; border-bottom:1px solid var(--border)}
/* "Go to" gets its own centred line under the nav row, so it never competes
   with the trail or the version pills for width — and the divider closes the
   whole header rather than sitting between its two halves. */
.gotorow{display:flex; justify-content:center; padding:0 0 .8rem;
  border-bottom:1px solid var(--border)}
.home{font-weight:700; color:var(--text); text-decoration:none; letter-spacing:.01em}
/* nowrap keeps the trail on one line — it was stacking into single words when
   "Go to" shared the row; now that Go to has its own line there is room. */
.crumbs{color:var(--muted); white-space:nowrap} .crumbs a{color:var(--muted); text-decoration:none}
.crumbs a:hover{text-decoration:underline} .sep{opacity:.5; padding:0 .15rem}
.vers{display:flex; gap:.3rem; margin-left:auto}
/* Deliberately NOT a version bubble: those are small, pill-shaped, muted and
   act like tabs. This is an action — accent-coloured, square-ish corners, a
   touch larger. It carries NO fill: a tinted chip on warm parchment reads as
   abrasive next to scripture, and the shape + weight + accent already say
   "button". The tint is kept for hover only, where it is transient. */
.goto{
  color:var(--accent); text-decoration:none; font-size:.85rem; font-weight:700;
  background:none; border:1px solid var(--border); border-radius:8px;
  padding:.3rem 1.1rem; white-space:nowrap; letter-spacing:.01em;
}
.goto:hover{border-color:var(--accent); background:var(--control-hover)}
/* Chapter heading row: title on the left, quiet prev/next arrows on the right.
   Arrows only up here — you land mid-text from a shared link, so this bar is
   for moving, not for labels; the labelled pager at the foot of the chapter is
   where a finished reader actually is. */
.chapbar{display:flex; align-items:center; gap:.75rem}
.chapnav{margin-left:auto; display:flex; gap:.4rem; white-space:nowrap}
/* Real arrows in real buttons. The first cut used thin ‹ › glyphs in a ~32px
   box: hard to see and under the 44px minimum a thumb needs. These match the
   page's other controls (Go to, version pills, the book grid all carry the
   same border and radius), which on the web reads as "clickable" better than
   a bare glyph does. Unfilled, like Go to — a pair of near-white tiles beside
   the chapter title fought with the text; the weight of the glyph is what
   makes them findable, not a bright ground. */
.arrow{
  display:inline-flex; align-items:center; justify-content:center;
  min-width:2.75rem; min-height:2.75rem;
  border:1px solid var(--border); border-radius:10px; background:none;
  /* The chrome stack: Georgia draws arrows as hairlines that vanish on a phone
     screen. Atkinson has no arrow glyphs, so these fall through to the system
     sans behind it — which is exactly what draws them with weight. */
  font-family:var(--ui);
  font-size:1.45rem; font-weight:700; line-height:1;
  color:var(--accent); text-decoration:none;
}
.arrow:hover{border-color:var(--accent); background:var(--control-hover)}
.arrow.off{color:var(--muted); opacity:.3; pointer-events:none; background:none}
/* Go-to overlay — the APP'S picker, not a search box: a two-stage alphabet
   navigator on the left (letters → that letter's books → back), the selected
   book's chapter grid on the right, and a verse range + Go along the bottom.
   Tapping a book or a chapter only SELECTS; Go commits. Mirroring the app
   matters more than web convention here: the reader arriving from a shared link
   is usually an app user, and typing a reference on a phone is the thing the
   app's picker exists to avoid. */
.gotobg{position:fixed; inset:0; background:rgba(0,0,0,.45); display:flex;
  align-items:flex-start; justify-content:center; padding:6vh 1rem 1rem; z-index:20}
.gotobox{background:var(--surface); border:1px solid var(--border); border-radius:12px;
  width:100%; max-width:30rem; padding:1rem; box-shadow:0 8px 30px rgba(0,0,0,.28)}
.gotohead{display:flex; align-items:center; margin:0 0 .7rem}
.gotohead h2{margin:0; font-size:1.15rem; letter-spacing:-.01em}
.gotox{margin-left:auto; background:none; border:0; padding:.1rem .3rem; cursor:pointer;
  color:var(--muted); font-size:1.4rem; line-height:1;
  font-family:var(--ui)}
.gotox:hover{color:var(--text)}
.gpanes{display:flex; gap:.7rem; align-items:stretch}
.gleft{flex:0 0 8.6rem; width:8.6rem; max-height:44vh; overflow-y:auto;
  border-right:1px solid var(--border); padding-right:.6rem}
.gright{flex:1 1 auto; max-height:44vh; overflow-y:auto; min-width:0}
/* The alphabet grid: only letters that actually have books in this canon, so
   there are no dead keys (the app's bookLetters rule). */
.galpha{display:grid; grid-template-columns:repeat(4,1fr); gap:.3rem}
.gchaps{display:grid; grid-template-columns:repeat(auto-fill,minmax(2.5rem,1fr)); gap:.3rem}
.galpha a,.gchaps a{
  display:flex; align-items:center; justify-content:center; min-height:2.2rem;
  border:1px solid var(--border); border-radius:8px; text-decoration:none;
  color:var(--text); font-size:.9rem; background:none;
}
.galpha a:hover,.gchaps a:hover{border-color:var(--accent); color:var(--accent)}
.gchaps a.on{border-color:var(--accent); color:var(--accent); font-weight:700; background:var(--control-selected)}
.gbooks{display:block}
.gbooks a{display:block; padding:.35rem .4rem; border-radius:6px; font-size:.92rem;
  color:var(--text); text-decoration:none}
.gbooks a:hover{background:var(--control-hover)}
.gbooks a.on{color:var(--accent); font-weight:700}
/* The way back to the alphabet, and the heading telling you which letter you
   are in. It was a muted ‹ and a small letter — easy to miss and easy to miss
   with a thumb. Accent-coloured, bold, with a rule under it so it reads as the
   pane's header rather than the first entry in the book list. */
.gback{display:flex; align-items:center; gap:.45rem;
  padding:.25rem .4rem .5rem; margin:0 0 .4rem;
  border-bottom:1px solid var(--border);
  color:var(--accent); text-decoration:none; font-weight:500;
  font-family:var(--ui)}
.gback:hover{border-bottom-color:var(--accent)}
/* Findable, not shouty: the accent colour and the rule do the work, so the
   glyph and letter stay close to the size of the book names under them. */
.bkarrow{font-size:1.1rem; line-height:1}
.bkletter{font-size:.95rem; letter-spacing:.02em}
/* Verse RANGE as two number fields — "verse [16] to [18]" — so there is no
   hyphen to type on a phone number pad. Exactly the app's row. */
.gverse{display:flex; align-items:center; gap:.5rem; margin:.9rem 0 0;
  padding:.8rem 0 0; border-top:1px solid var(--border)}
.gverse input{
  font-family:var(--ui); font-size:1rem; padding:.4rem .6rem;
  color:var(--text); background:var(--bg);
  border:1px solid var(--border); border-radius:8px; outline:none; min-width:0;
}
/* The app's proportions: the start field FLEXES to fill the row and the end
   field is a fixed narrow cell, with "to" and Go compact on the right. */
.gverse .gvstart{flex:1 1 auto; width:auto}
.gverse .gvend{flex:0 0 4rem; width:4rem}
.gverse input:focus{border-color:var(--accent)}
.gto{color:var(--muted); font-size:.85rem}
/* Filled, unlike the page's quiet controls: this is the one commit action, it
   lives on a dimmed sheet, and it is the app's HighImportance Go button. */
.ggo{margin-left:auto; background:var(--accent); color:var(--surface); border:0;
  border-radius:8px; padding:.45rem 1.4rem; font-size:.9rem; font-weight:700;
  cursor:pointer; font-family:inherit}
.ggo:hover{opacity:.9}
.vpick{
  color:var(--muted); text-decoration:none; font-size:.7rem; letter-spacing:.04em;
  border:1px solid var(--border); border-radius:999px; padding:.15rem .5rem;
}
.vpick.on{color:var(--accent); border-color:var(--accent)}
.ref{font-size:1.9rem; margin:1.1rem 0 .1rem; font-weight:700; letter-spacing:-.01em}
.ver{margin:0 0 1.3rem; color:var(--muted); font-size:.85rem}
/* THE SCRIPTURE FACE, matched to the iOS app. Every value here is the app's
   own reading CSS (buildChapterHTML, reading.go): the same stack led by
   Georgia, the same 21px base, the same 0.004em tracking and the same OpenType
   features — kerning, ligatures, contextual alternates and OLD-STYLE numerals,
   which is what gives the app's verse numbers their book-page look.
   21px is written as 1.3125rem rather than px so it still answers a reader who
   has set a larger default text size in their browser, while landing on exactly
   21px at the default. */
/* LEADING — MEASURED off the app, not copied from its stylesheet, because the
   app does not render what its stylesheet says. buildChapterHTML asks for
   line-height 2.0 on phones, but that CSS goes through the UIKit HTML importer
   into an attributed string, and what comes out is neither 2.0 nor the font's
   natural height:

     importer on macOS   drops line-height entirely (multiple 0, spacing 0)
     importer on iOS     turns it into minimumLineHeight 42 (= 2.0 x 21)
     what iOS RENDERS    a 27.67pt line pitch at 21px  ->  1.3175

   27.67pt was measured twice at 3x — on a real iPhone screenshot and on the
   simulator at default text size — as 83 device px, both to the pixel. So 1.3175
   is what a reader actually sees, and it is what the page sets.

   The paragraph gap is measured the same way: a boundary comes out at exactly
   166 device px, two line pitches, i.e. ONE BLANK LINE — not the 24px the
   stylesheet asks for. Hence 1.3175em below rather than 24px.

   Both numbers are unitless/em so they still scale with a reader's own font
   size. If the app's reading pane is ever re-typeset, re-measure — do not read
   these off buildChapterHTML. */
.text{
  font-family:var(--scripture);
  font-size:1.3125rem; line-height:1.3175; letter-spacing:.004em;
  -webkit-font-smoothing:antialiased;
  font-feature-settings:"kern" 1,"liga" 1,"calt" 1,"onum" 1;
}
.text p{margin:0 0 1.3175em; text-align:justify; hyphens:auto; -webkit-hyphens:auto}
/* Paragraph shape mirrors the app: on a phone, paragraphs are separated by
   space (the app's phone reading pane); from tablet width up the app switches
   to its iPad "reporter" setting — a first-line indent with no blank line
   between paragraphs, which is how a printed Bible sets prose. A paragraph
   that OPENS with poetry keeps its space and takes no indent, matching the
   app's rule that the reporter indent is skipped for poetic paragraphs. */
@media (min-width:46rem){
  /* The reporter set: no gap between paragraphs, a first-line indent instead
     (the app's em+en spaces). Leading stays natural — the importer drops the
     app's 1.3 exactly as it drops the phone's 2.0. */
  .text p{margin:0; text-indent:1.5em}
  .text p:first-child{text-indent:0}
  .text p.pm{margin:.55rem 0; text-indent:0}
  .text p.pm + p{text-indent:0}
}
.text p.pm{text-align:left; hyphens:none}
/* The app's verse number: sup.v — 0.66em, weight 600, no tracking, 2px of air
   after it. The raise is left to the browser's own <sup> handling, damped so a
   superscript cannot open up the line it sits on. */
.n{font-size:.66em; font-weight:600; letter-spacing:0; margin-right:2px;
  vertical-align:.45em; line-height:0}
.n a{color:var(--verse); text-decoration:none}
.n a:hover{text-decoration:underline}
.wj{color:var(--red)}
/* A single-verse link works with NO JavaScript: the fragment matches the
   verse's id and :target paints it. reader.js only adds ranges. */
/* Generous scroll-margin so a deep link lands the verse with the chapter
   heading still on screen, rather than pinning it to the very top edge. */
.v:target,.v.hl{background:var(--verse-hl); border-radius:3px;
  box-shadow:0 0 0 .18em var(--verse-hl); scroll-margin-top:6.5rem}
/* Browsers do NOT re-evaluate :target when history.replaceState drops the
   fragment — the URL loses #v16 but the verse stays lit, so tapping a
   single-verse highlight appeared to do nothing (a range cleared, because that
   is class-based). reader.js sets this flag when it clears, and the extra
   class beats the bare :target on specificity. Without JS the flag is never
   set, so the no-JS highlight is untouched. */
html.nohl .v:target{background:none; box-shadow:none; cursor:auto}
.empty{color:var(--muted); font-style:italic}
.grid{list-style:none; padding:0; margin:0;
  display:grid; grid-template-columns:repeat(auto-fill,minmax(9.5rem,1fr)); gap:.4rem}
.grid.nums{grid-template-columns:repeat(auto-fill,minmax(3.2rem,1fr))}
.grid a{
  display:block; padding:.55rem .6rem; text-decoration:none; color:var(--text);
  background:var(--surface); border:1px solid var(--border); border-radius:8px;
  font-size:.92rem;
}
.grid.nums a{text-align:center}
.grid a:hover{border-color:var(--accent); color:var(--accent)}
.pager{display:flex; justify-content:space-between; gap:1rem; margin-top:1.5rem;
  padding-top:1rem; border-top:1px solid var(--border); font-size:.9rem}
.pager a{color:var(--accent); text-decoration:none}
.pager a:hover{text-decoration:underline}
.guess{margin:0 0 1.2rem}
.guess a{color:var(--accent)}
/* Clearing a highlight: tap the highlighted text and a small "Clear highlight"
   bubble appears at the tap; tap the bubble to clear, tap anywhere else (or the
   verse again) and it goes away. cursor:pointer is not only a hint here — it is
   also what makes iOS Safari treat a bare <span> as tappable at all. */
.v:target,.v.hl{cursor:pointer; -webkit-tap-highlight-color:transparent}
/* Sits just under the highlighted text, centred on the column — so it points at
   what it offers without covering it. Absolute (document coordinates), so it
   travels with the passage as the reader scrolls; reader.js sets top/left from
   the last line of the highlight. It holds ONE button for a plain highlight and
   TWO when the highlight belongs to a note, because "hide" and "delete" are
   different enough that a reader should not have to guess. */
.clearbar{
  position:absolute; z-index:15; transform:translateX(-50%);
  display:flex; gap:.4rem;
}
.clearbub{
  background:var(--surface); color:var(--accent);
  border:1px solid var(--border); border-radius:999px;
  padding:.4rem .9rem; font-size:.8rem; font-weight:700; cursor:pointer;
  font-family:var(--ui);
  box-shadow:0 3px 12px rgba(0,0,0,.18); white-space:nowrap;
  -webkit-user-select:none; user-select:none; -webkit-tap-highlight-color:transparent;
}
.clearbub:hover{border-color:var(--accent)}
/* THE SENDER'S NOTE. It sits above the chapter, in the flow, so it never covers
   the words it is about and needs no JavaScript to follow the page.
   It must NOT look like BibleText chrome: the whole point of the attribution
   line and the quoted, off-palette card is that a reader can tell at a glance
   that a person wrote this, not the app. A note that could pass for our own
   voice would be a phishing surface on our own domain. */
/* Anchoring the note beside its passage puts it INSIDE .text, so it would
   otherwise inherit the scripture face, its justification, its 21px, and the
   reporter first-line indent — i.e. it would look like scripture. Everything
   here is an explicit reset: a reader must never have to work out whether the
   words in front of them are the Bible or a stranger's message. */
.note{
  position:relative; margin:1.1rem 0; padding:.85rem 2.2rem .9rem 1rem;
  /* The shared arrival lead (noteMetrics().Lead), not this file's own offset.
     The VERSE target keeps its larger 6.5rem margin — a bare #vN jump is the
     browser's own scroll, held well clear of the page top for legibility.
     That larger number is this surface's stated divergence from the single
     Lead, pinned by note_chrome_shared_test.go. */
  scroll-margin-top:__NOTE_LEAD__px;
  background:var(--surface); border:1px solid var(--border);
  border-left:3px solid var(--accent); border-radius:10px;
  font-family:var(--ui); font-size:1rem; line-height:1.5;
  letter-spacing:normal; text-align:left; text-indent:0;
  font-feature-settings:normal;
}
.note p{margin:0; text-align:left; text-indent:0; hyphens:none}
/* The tail, which is what makes it read as somebody speaking. */
.note::after{
  content:""; position:absolute; left:1.6rem; bottom:-9px;
  width:14px; height:14px; background:var(--surface);
  border-right:1px solid var(--border); border-bottom:1px solid var(--border);
  transform:rotate(45deg);
}
/* No passage to point at, no tail — the shared rule (noteChrome.hasTail);
   anchorToPassage sets the class on the chapter-top parking. */
.note.notail::after{display:none}
.notewho{
  margin:0 0 .3rem; color:var(--muted); font-size:.78rem;
  letter-spacing:.01em;
}
/* The note itself is set in the CHROME face, not the scripture serif: it is
   somebody's message, and it must never be mistaken for the text. */
.notetext{margin:0; font-size:.95rem; line-height:1.5; white-space:pre-wrap; overflow-wrap:anywhere}
/* Minimize and delete, in that order: the reversible one first, so the
   destructive one is never the one a thumb reaches by accident. */
.notetools{position:absolute; top:.3rem; right:.35rem; display:flex; gap:.1rem}
.notebtn{
  background:none; border:0; padding:.3rem; cursor:pointer; color:var(--muted);
  line-height:0; border-radius:6px;
}
.notebtn svg{width:15px; height:15px; fill:currentColor; display:block}
.notebtn:hover{color:var(--accent); background:var(--control-hover)}
/* The minimized marker. Small and quiet, but unmistakably a thing to press:
   the note is still there and the reader has to be able to find it again. */
.notechip{
  display:inline-flex; align-items:center; gap:.35rem; margin:1.1rem 0;
  scroll-margin-top:__NOTE_LEAD__px;
  letter-spacing:normal; text-indent:0;
  background:none; border:1px solid var(--border); border-radius:999px;
  padding:.3rem .8rem; font-size:.78rem; font-family:var(--ui);
  color:var(--muted); cursor:pointer; line-height:1.2;
}
.notechip svg{width:13px; height:13px; fill:currentColor; display:block}
.notechip:hover{border-color:var(--accent); color:var(--accent)}
/* THE COULD-NOT-READ NOTICE: what stands in the note's place when a link's
   payload cannot be rendered (a newer note format, or damage). Quiet and
   attributed to nobody — no tail (nobody is speaking), no accent edge, no
   button, no link. Same explicit resets as .note: it sits inside .text and
   must read as neither scripture nor a person's message. */
.notenotice{
  margin:1.1rem 0; padding:.8rem 1rem; scroll-margin-top:1.2rem;
  background:var(--surface); border:1px solid var(--border); border-radius:10px;
  color:var(--muted); font-family:var(--ui); font-size:.9rem; line-height:1.5;
  letter-spacing:normal; text-align:left; text-indent:0;
  font-feature-settings:normal;
}
.foot{max-width:40rem; margin:0 auto; padding:0 1.6rem 2.5rem; text-align:center}
.foot a{color:var(--muted); font-size:.8rem; text-decoration:none}
.foot a:hover{color:var(--accent); text-decoration:underline}
/* Platform row under the app link. Monochrome and quiet on purpose — this is a
   footnote, not a call to action; the icons say "it runs on your thing" at a
   glance without shouting. They inherit currentColor, so they follow the muted
   colour and the light/dark flip with no second asset. */
.plats{display:flex; justify-content:center; align-items:center; gap:.6rem;
  margin:.2rem 0 0; color:var(--muted)}
.plats a{display:inline-flex; align-items:center; color:inherit; opacity:.75}
.plats a:hover{opacity:1; color:var(--accent)}
/* px, not rem: on a phone with a large text-size setting these were rendering
   enormous. They are decoration and should stay the same modest size. */
.plats svg{width:15px; height:15px; fill:currentColor; display:block}
@media print{.top,.pager,.foot{display:none}}
`

// THE BYTES OF THIS TEMPLATE ARE FROZEN IN 3,906 FILENAMES. readerJSTemplate
// (and readerCSSTemplate above it) is content-hashed into the <script>/<link>
// of every page that carries scripture, so editing one character here rewrites
// all of them — which is why the notice pages (notice_assets.go) ship a second,
// separate pair instead of adding a rule or a function to these. That includes
// the COMMENTS inside the backticks: they are part of the shipped asset. The
// "~3,900 files" figure in bridgeHighlightGaps below is one of them, and it is
// left as written for that reason; the site is 5,470 files today, 3,906 of them
// pages with .v spans to bridge.
//
// readerJS holds everything the page cannot do in CSS alone. With JS off a
// single-verse link still highlights (CSS :target) and "Get the app" still
// points at the all-platforms landing page — but a verse RANGE does not
// highlight, and the sender's NOTE does not appear at all, because the note
// lives only in the fragment and only this file decodes it. That is a real
// degradation, not a cosmetic one: a note-bearing link looks like a plain
// chapter link to a reader without scripting.
const readerJSTemplate = `
(function () {
  // 0) The fragment is a KEY LIST, "&"-separated (share_link.go): the verse
  // span, written bare as "v16" / "v16-18", plus optional keys like "n=<note>".
  // Unknown keys are ignored — that rule is what lets a future key be added
  // without stranding the links already sent, and it has to hold here too.
  function fragKeys() {
    var out = { v: '' };
    var raw = (location.hash || '').replace(/^#/, '');
    if (!raw) return out;
    raw.split('&').forEach(function (kv, i) {
      var eq = kv.indexOf('=');
      if (eq < 0) {
        // The bare verse token, which only ever leads.
        if (i === 0 && /^v\d/.test(kv)) out.v = kv.slice(1);
        return;
      }
      out[kv.slice(0, eq)] = kv.slice(eq + 1);
    });
    if (!out.v && /^\d/.test(out['v='] || '')) out.v = out['v='];
    return out;
  }

  function verseSpan() {
    var m = /^(\d+)(?:-(\d+))?$/.exec(fragKeys().v || '');
    if (!m) return null;
    var lo = parseInt(m[1], 10), hi = m[2] ? parseInt(m[2], 10) : lo;
    if (!(lo > 0) || hi < lo) return null;
    return [lo, hi];
  }

  // 1) Verse RANGES (#v16-18). A single verse needs no help — :target has it.
  // THE JOINING SPACE BETWEEN TWO VERSES BELONGS TO THE BAND.
  //
  // paragraphBody writes that space BETWEEN two <span class="v"> elements, so it
  // is inside neither, and a highlighted range came out notched at every join —
  // one gap per pair, though only the joins falling mid-line are visible, which
  // is why the fault looked intermittent. Fixed here rather than in the markup so
  // it repairs every page already generated, without regenerating ~3,900 files.
  //
  // Whitespace-only text nodes only. A <br> between two verses (a poem join) has
  // no width, so there is nothing to bridge and the band should stop at the line
  // end anyway.
  function bridgeHighlightGaps() {
    var lit = document.querySelectorAll('.v.hl');
    for (var i = 1; i < lit.length; i++) {
      var n = lit[i - 1].nextSibling;
      while (n && n !== lit[i]) {
        var next = n.nextSibling;
        if (n.nodeType === 3 && n.textContent.length && !n.textContent.trim()) {
          var s = document.createElement('span');
          s.className = 'hl hlgap';
          s.textContent = n.textContent;
          n.parentNode.replaceChild(s, n);
        }
        n = next;
      }
    }
  }

  // Put the bridged spaces back to plain text. Without this, clearing and
  // re-highlighting would leave stale .hlgap spans lit between verses that are
  // no longer highlighted.
  function dropHighlightGaps() {
    document.querySelectorAll('.hlgap').forEach(function (el) {
      el.parentNode.replaceChild(document.createTextNode(el.textContent), el);
    });
  }

  function highlightRange() {
    dropHighlightGaps();
    document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    var m = verseSpan();
    if (!m) return;
    // A fresh fragment re-lights the verse: drop the suppression flag a
    // previous clear may have set.
    document.documentElement.classList.remove('nohl');
    var lo = m[0], hi = m[1];
    var first = null;
    for (var n = lo; n <= hi; n++) {
      var el = document.getElementById('v' + n);
      if (!el) continue;            // out of range: highlight what exists, never error
      el.classList.add('hl');
      if (!first) first = el;
    }
    bridgeHighlightGaps();
    if (first && hi > lo) first.scrollIntoView({ block: 'center' });
  }
  // Clearing a highlight: tap the highlighted text and a small bubble appears at
  // the tap; the bubble clears it. Removing the fragment is what does the work —
  // it un-targets the CSS :target rule (single verse) as well as letting us drop
  // the range classes.
  function highlighted() { return verseSpan() !== null; }

  var bubble = null;
  function hideBubble() { if (bubble) { bubble.remove(); bubble = null; } }

  // Place the bubble just below the LAST line of the highlight, centred on the
  // text column. getClientRects (not getBoundingClientRect) is what makes this
  // land correctly: a verse spanning several lines has one rect per line, and
  // the box around all of them would put the bubble under the widest line
  // rather than under where the verse actually ends.
  function positionBubble(b) {
    var els = document.querySelectorAll('.v.hl');
    if (!els.length) els = document.querySelectorAll('.v:target');
    if (!els.length) return;
    var lines = [], i, j, rl;
    for (i = 0; i < els.length; i++) {
      rl = els[i].getClientRects();
      for (j = 0; j < rl.length; j++) if (rl[j].height > 1) lines.push(rl[j]);
    }
    if (!lines.length) return;
    var h = b.offsetHeight || 34, gap = 8;
    // Under the LAST line of the highlight that still leaves room on screen. A
    // three-verse highlight can run past the fold, and pinning to its true last
    // line would drop the bubble off the screen the reader is looking at.
    var pick = null;
    for (i = 0; i < lines.length; i++) {
      if (lines[i].bottom + gap + h <= window.innerHeight) pick = lines[i];
    }
    if (!pick) pick = lines[0];
    var col = (document.querySelector('.wrap') || document.body).getBoundingClientRect();
    b.style.top = (pick.bottom + gap + window.pageYOffset) + 'px';
    b.style.left = (col.left + col.width / 2 + window.pageXOffset) + 'px';
  }

  // Tapping a highlight offers what that highlight actually IS. A plain one is
  // just a highlight, so it offers to clear it. One that belongs to a note is
  // the note's highlight, so it offers the note's own two actions — the same
  // pair the bubble carries, because "hide" and "delete" mean different things
  // and the reader should not have to guess which a single control does.
  function showBubble() {
    hideBubble();
    var bar = document.createElement('div');
    bar.className = 'clearbar';

    var noteText = currentNoteText();
    if (noteText) {
      // The same verb the card offers, under the same name — it was
      // 'Hide note' here and 'Minimize note' there, two vocabularies for one
      // action on one page.
      bar.appendChild(barButton('Minimize note', function () {
        hideBubble(); minimizeNote(noteText);
      }));
      bar.appendChild(barButton('Delete note', function () {
        hideBubble(); trashNote();
      }));
    } else {
      bar.appendChild(barButton('Clear highlight', function () { clearHighlight(); }));
    }

    document.body.appendChild(bar);
    positionBubble(bar);
    bubble = bar;
  }

  function barButton(label, onTap) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'clearbub';
    b.textContent = label;
    b.addEventListener('click', function (e) { e.preventDefault(); onTap(); });
    return b;
  }

  // The note this page is currently showing, if any — the bubble and the
  // minimized marker both count, because both mean "there is a note here".
  var activeNoteText = '';
  function currentNoteText() { return activeNoteText; }

  // The text reflows on rotation or a window resize, so the line the bubble was
  // pinned under moves. Re-pin rather than leave it stranded.
  window.addEventListener('resize', function () { if (bubble) positionBubble(bubble); });

  function clearHighlight() {
    // replaceState, not a hash change: it leaves no extra history entry, so Back
    // still returns where the reader came from rather than re-highlighting.
    // Only the VERSE key goes — a note in the same fragment is a separate thing
    // with its own dismiss, and clearing the highlight must not silently delete
    // it from the URL the reader might reload or re-share.
    var keep = (location.hash || '').replace(/^#/, '').split('&')
      .filter(function (kv, i) { return !(i === 0 && /^v\d/.test(kv)) && kv.indexOf('v=') !== 0; })
      .join('&');
    history.replaceState(null, '', location.pathname + location.search + (keep ? '#' + keep : ''));
    // Dropping the fragment does NOT un-target the verse — browsers keep the
    // :target match until a real navigation — so a single-verse highlight (the
    // usual shared link) would stay lit. This flag overrides it in CSS.
    document.documentElement.classList.add('nohl');
    hideBubble();
    document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    dropHighlightGaps(); // the bridged joins go too, or they stay lit alone
    // replaceState fires NO hashchange, so anything reading the fragment has to
    // be updated by hand — otherwise the version switcher keeps carrying a verse
    // that is no longer highlighted.
    carryVerse();
  }

  // POINTERUP, not click. iOS Safari only synthesises a click for elements it
  // considers tappable, and a verse is a bare <span>; a delegated click listener
  // is exactly the case that quietly never fires there, which is why tapping the
  // highlight did nothing on a phone while working on the desktop. Pointer
  // events are delivered regardless. Click is the fallback for anything without
  // Pointer Events.
  var TAP = window.PointerEvent ? 'pointerup' : 'click';

  // Was this a tap or the end of a drag? A drag is the reader selecting text,
  // and a bubble must not jump up over their selection. Measuring the movement
  // is the honest test: the earlier version asked whether a selection existed
  // at all, which meant that once a reader had selected anything, tapping the
  // highlight did nothing until they cleared it — with no way to know why.
  var downX = null, downY = null;
  document.addEventListener('pointerdown', function (e) { downX = e.clientX; downY = e.clientY; });
  function wasDrag(e) {
    if (downX === null || typeof e.clientX !== 'number') return false;
    var dx = e.clientX - downX, dy = e.clientY - downY;
    return (dx * dx + dy * dy) > 100;              // >10px of travel
  }

  document.addEventListener(TAP, function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    // The bubble is handled HERE rather than by its own click handler: on the
    // pointerup path this listener runs first and would remove the button
    // before any click of its own could fire.
    // The bar's buttons carry their own handlers; this listener runs first on
    // the pointerup path and would otherwise remove them before a click landed,
    // so let the tap through and stop here.
    if (t.closest('.clearbar')) { return; }
    if (!highlighted()) { hideBubble(); return; }
    if (wasDrag(e)) return;                       // a selection, not a tap
    var link = t.closest('a');
    if (link) {
      // The verse numbers are permalinks (#v16) — that is how a reader grabs a
      // link to one verse from this page. But while the clear bubble is up the
      // reader is dismissing, not navigating, and the numbers sit right against
      // the words being tapped: landing on one would silently move the
      // highlight to another verse. While dismissing, a number just dismisses.
      if (bubble && /^#v\d+/.test(link.getAttribute('href') || '')) {
        e.preventDefault();
        hideBubble();
      }
      return;
    }
    if (t.closest('.v.hl') || t.closest('.v:target')) {
      if (bubble) hideBubble();                   // tap again dismisses it
      else showBubble();
      return;
    }
    hideBubble();                                 // a tap anywhere else
  });
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    if (bubble) { hideBubble(); return; }
    if (highlighted()) clearHighlight();
  });

  highlightRange();
  window.addEventListener('hashchange', highlightRange);

  // 3) "Go to" — the APP'S picker, rebuilt for the page. The button is a real
  // link to the book grid, so it still works with no JavaScript; this upgrades
  // it into the same two-stage flow the app uses: alphabet grid → that letter's
  // books → chapter grid, with an optional verse range and a Go button that
  // commits. Tapping a book or chapter only selects it, exactly as in the app
  // (gotoPickerModal, withVerse=true).
  //
  // One table for the whole site, so reader.js is cached once rather than per
  // version. Books arrive in the app's alphabetical order, each carrying the
  // letter it files under, and its chapter count PER version — the canons
  // differ (Greek Daniel has 14 chapters under webc, 12 elsewhere) and the grid
  // must never offer a page that was not generated.
  var BOOKS = __BOOKS__;
  var VERSION = (location.pathname.split('/')[1] || 'web');
  // Books present in THIS translation, still in the app's order.
  var CANON = BOOKS.filter(function (b) { return b.ch[VERSION] > 0; });

  function currentBookSlug() { return location.pathname.split('/')[2] || ''; }

  function openGoto(ev) {
    if (ev) ev.preventDefault();

    // Start on the chapter the reader is actually in, like the app.
    var sel = null, selCh = parseInt(location.pathname.split('/')[3], 10) || 1;
    var here = currentBookSlug();
    CANON.forEach(function (b) { if (b.slug === here) sel = b; });
    if (!sel) { sel = CANON[0]; selCh = 1; }
    var stage = 0, letter = sel ? sel.l : '';

    var bg = document.createElement('div');
    bg.className = 'gotobg';
    bg.innerHTML =
      '<div class="gotobox">' +
        '<div class="gotohead"><h2>Go to</h2>' +
          '<button class="gotox" type="button" aria-label="Close">&times;</button></div>' +
        '<div class="gpanes"><div class="gleft"></div><div class="gright"></div></div>' +
        '<div class="gverse">' +
          '<input class="gvstart" type="text" inputmode="numeric" pattern="[0-9]*" placeholder="verse" aria-label="Verse">' +
          '<span class="gto">to</span>' +
          '<input class="gvend" type="text" inputmode="numeric" pattern="[0-9]*" placeholder="end" aria-label="End verse">' +
          '<button class="ggo" type="button">Go</button>' +
        '</div>' +
      '</div>';
    document.body.appendChild(bg);
    var left = bg.querySelector('.gleft'), right = bg.querySelector('.gright');
    var startBox = bg.querySelectorAll('.gverse input')[0];
    var endBox = bg.querySelectorAll('.gverse input')[1];

    function letters() {
      var seen = {}, out = [];
      CANON.forEach(function (b) { if (!seen[b.l]) { seen[b.l] = 1; out.push(b.l); } });
      return out;                             // first-seen order == A→Z, no dead keys
    }

    function cell(cls, text, onTap) {
      var a = document.createElement('a');
      a.href = '#'; a.className = cls; a.textContent = text;
      a.addEventListener('click', function (e) { e.preventDefault(); onTap(); });
      return a;
    }

    function renderLeft() {
      left.innerHTML = '';
      if (stage === 0) {
        var grid = document.createElement('div');
        grid.className = 'galpha';
        letters().forEach(function (L) {
          grid.appendChild(cell('', L, function () { letter = L; stage = 1; renderLeft(); }));
        });
        left.appendChild(grid);
        return;
      }
      var back = document.createElement('a');
      back.href = '#';
      back.className = 'gback';
      back.innerHTML = '<span class="bkarrow">&larr;</span><span class="bkletter"></span>';
      back.querySelector('.bkletter').textContent = letter;
      back.addEventListener('click', function (e) { e.preventDefault(); stage = 0; renderLeft(); });
      left.appendChild(back);
      var list = document.createElement('div');
      list.className = 'gbooks';
      CANON.forEach(function (b) {
        if (b.l !== letter) return;
        var a = cell(b === sel ? 'on' : '', b.name, function () {
          // Selecting a book selects its chapter 1 and repopulates the grid —
          // it does not navigate and does not leave this stage.
          sel = b; selCh = (b.slug === here) ? (parseInt(location.pathname.split('/')[3], 10) || 1) : 1;
          renderLeft(); renderRight();
        });
        list.appendChild(a);
      });
      left.appendChild(list);
    }

    function renderRight() {
      right.innerHTML = '';
      if (!sel) return;
      var grid = document.createElement('div');
      grid.className = 'gchaps';
      var max = sel.ch[VERSION] || 1;
      for (var n = 1; n <= max; n++) {
        (function (n) {
          grid.appendChild(cell(n === selCh ? 'on' : '', String(n), function () {
            selCh = n; renderRight();       // select only — Go commits
          }));
        })(n);
      }
      right.appendChild(grid);
      var on = grid.querySelector('.on');
      if (on) on.scrollIntoView({ block: 'nearest' });
    }

    function commit() {
      if (!sel) return;
      var max = sel.ch[VERSION] || 1;
      var ch = selCh > 0 && selCh <= max ? selCh : 1;
      var lo = parseInt(startBox.value, 10);
      var hi = parseInt(endBox.value, 10);
      var frag = '';
      if (lo > 0) frag = '#v' + lo + (hi > lo ? '-' + hi : '');
      // CLOSE FIRST. Going to a verse in the chapter already open changes only
      // the fragment, and a fragment change does not load a document — so
      // nothing tears the picker down and it sat there over the passage it had
      // just jumped to. Navigating to another chapter hid the bug, because that
      // really is a page load.
      close();
      location.href = '/' + VERSION + '/' + sel.slug + '/' + ch + '/' + frag;
    }

    function close() { bg.remove(); document.removeEventListener('keydown', onKey, true); }
    function onKey(e) {
      if (e.key === 'Escape') { e.preventDefault(); close(); }
      else if (e.key === 'Enter') { e.preventDefault(); commit(); }
    }
    bg.querySelector('.gotox').addEventListener('click', close);
    bg.querySelector('.ggo').addEventListener('click', commit);
    document.addEventListener('keydown', onKey, true);
    bg.addEventListener('click', function (e) { if (e.target === bg) close(); });

    stage = 1;                                 // open ON the current book's letter
    renderLeft(); renderRight();
  }

  var gotoBtn = document.getElementById('gotobtn');
  if (gotoBtn) gotoBtn.addEventListener('click', openGoto);

  // 1b) THE SENDER'S NOTE. The link may carry one in the "n" key; the payload
  // is the app's record codec (share_note.go is the reference implementation,
  // docs/NOTE_WIRE_FORMAT.md the frozen spec, and testdata/note_vectors.txt
  // the corpus BOTH decoders must agree on): a framing byte — 'r' records raw,
  // 'd' records DEFLATE'd, 'p'/'z' legacy bare text, 'A'-'Z' reserved for a
  // newer format — then <tag><uvarint len><value> records, unpadded base64url.
  //
  // It is UNTRUSTED TEXT — anyone can write a link. It is inserted with
  // textContent and never as markup, no part of it is ever made a live link,
  // and the bubble says whose it is, because a note styled as though BibleText
  // said it would be a phishing kit on our own domain.
  //
  // Between the two NOTE_DECODER markers the code is PURE — no DOM, nothing
  // from the page — because cmd/websitegen's conformance test extracts exactly
  // that span and walks it over the same vector file as the Go decoder. Keep
  // it that way.
  /*__NOTE_DECODER_BEGIN__*/
  var NOTE_MAX_RUNES = 280;
  var NOTE_MAX_INFLATED = NOTE_MAX_RUNES * 4 + 1; // legacy 'z' cap — frozen
  var NOTE_MAX_RECORD_BYTES = 4096;               // 'r'/'d' stream cap — frozen
  // The two sentences of docs/NOTE_WIRE_FORMAT.md rule 5. No call to action,
  // no link, never an install prompt; share_note.go carries the same strings.
  var NOTE_MSG_NEWER = 'This link carries a note written in a newer note format.';
  var NOTE_MSG_DAMAGED = "This link's note looks damaged.";

  var NOTE_LENS = [3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51,
    59, 67, 83, 99, 115, 131, 163, 195, 227, 258];
  var NOTE_LEXT = [0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4,
    4, 5, 5, 5, 5, 0];
  var NOTE_DISTS = [1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385,
    513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577];
  var NOTE_DEXT = [0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10,
    10, 11, 11, 12, 12, 13, 13];
  var NOTE_CLORDER = [16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15];

  // Canonical Huffman table from code lengths, or null when over-subscribed.
  function noteHuffConstruct(lengths, n, allowIncomplete) {
    var count = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0];
    var i, len;
    for (i = 0; i < n; i++) count[lengths[i] || 0]++;
    var left = 1, total = 0;
    for (len = 1; len <= 15; len++) {
      left <<= 1;
      left -= count[len];
      if (left < 0) return null;
      total += count[len];
    }
    // INCOMPLETE tables are rejected for DYNAMIC blocks, exactly as Go rejects
    // them — with Go's one exception: a single symbol of code length 1, the
    // degenerate one-distance-code table RFC 1951 lets an encoder emit. This
    // used to check only over-subscription (left < 0), which accepted
    // incomplete tables Go refuses; a hand-built stream then decoded as a NOTE
    // here and as DAMAGED in the app — two decoders, two formats, found by
    // differential fuzzing against compress/flate.
    //
    // allowIncomplete exists for the FIXED tables and nothing else: RFC
    // 1951's fixed DISTANCE table is incomplete BY SPECIFICATION (30 codes in
    // 32 slots), and Go never notices because it reads fixed-block distances
    // as raw 5-bit values without building a table at all. Applying the
    // dynamic rule to it made every fixed-Huffman block crash — the first
    // version of this very fix, caught by the conformance corpus.
    if (!allowIncomplete && left > 0 && !(total === 1 && count[1] === 1)) return null;
    var offs = [0, 0];
    for (len = 1; len < 15; len++) offs[len + 1] = offs[len] + count[len];
    var symbol = [];
    for (i = 0; i < n; i++) if (lengths[i]) symbol[offs[lengths[i]]++] = i;
    return { count: count, symbol: symbol };
  }

  var noteFixedHuff = null;

  // Raw-DEFLATE inflate (RFC 1951), self-contained, in the classic puff()
  // shape. DecompressionStream is NOT usable here: it errors on trailing bytes
  // after the final block, and the frozen format ACCEPTS them — Go's
  // compress/flate stops at the final block and returns nil, and byte-for-byte
  // agreement with the app decoder is the whole point. Returns a byte array,
  // or null on ANY error — including inflating past the limit, which mirrors
  // Go's read-one-byte-past-the-cap bomb refusal.
  function noteInflateRaw(src, limit) {
    var pos = 0, bitbuf = 0, bitcnt = 0, out = [], err = false;

    function bits(need) {
      var val = bitbuf;
      while (bitcnt < need) {
        if (pos >= src.length) { err = true; return 0; }
        val |= src[pos++] << bitcnt;
        bitcnt += 8;
      }
      bitbuf = val >>> need;
      bitcnt -= need;
      return val & ((1 << need) - 1);
    }

    function decode(h) {
      var code = 0, first = 0, index = 0;
      for (var len = 1; len <= 15; len++) {
        code |= bits(1);
        if (err) return -1;
        var count = h.count[len];
        if (code - first < count) return h.symbol[index + (code - first)];
        index += count;
        first += count;
        first <<= 1;
        code <<= 1;
      }
      return -1;
    }

    function push(b) {
      out.push(b);
      if (out.length > limit) err = true;
    }

    function codes(lencode, distcode) {
      for (;;) {
        var sym = decode(lencode);
        if (sym < 0 || err) return false;
        if (sym < 256) {
          push(sym);
          if (err) return false;
        } else if (sym === 256) {
          return true;
        } else {
          sym -= 257;
          if (sym >= 29) return false;
          var len = NOTE_LENS[sym] + bits(NOTE_LEXT[sym]);
          if (err) return false;
          var dsym = decode(distcode);
          if (dsym < 0 || err) return false;
          var dist = NOTE_DISTS[dsym] + bits(NOTE_DEXT[dsym]);
          if (err) return false;
          if (dist > out.length) return false;
          while (len--) {
            push(out[out.length - dist]);
            if (err) return false;
          }
        }
      }
    }

    function stored() {
      bitbuf = 0; bitcnt = 0;              // discard to a byte boundary
      if (pos + 4 > src.length) return false;
      var len = src[pos] | (src[pos + 1] << 8);
      var nlen = src[pos + 2] | (src[pos + 3] << 8);
      pos += 4;
      if (len !== ((~nlen) & 0xffff)) return false;
      if (pos + len > src.length) return false;
      for (var i = 0; i < len; i++) {
        push(src[pos + i]);
        if (err) return false;
      }
      pos += len;
      return true;
    }

    function dynamicTables() {
      var nlen = bits(5) + 257, ndist = bits(5) + 1, ncode = bits(4) + 4;
      if (err || nlen > 286 || ndist > 30) return null;
      var lengths = [], i;
      for (i = 0; i < 19; i++) lengths[i] = 0;
      for (i = 0; i < ncode; i++) lengths[NOTE_CLORDER[i]] = bits(3);
      if (err) return null;
      var clcode = noteHuffConstruct(lengths, 19);
      if (!clcode) return null;
      var symlens = [], index = 0;
      while (index < nlen + ndist) {
        var sym = decode(clcode);
        if (sym < 0 || err) return null;
        if (sym < 16) {
          symlens[index++] = sym;
        } else {
          var repLen = 0, count;
          if (sym === 16) {
            if (index === 0) return null;
            repLen = symlens[index - 1];
            count = 3 + bits(2);
          } else if (sym === 17) {
            count = 3 + bits(3);
          } else {
            count = 11 + bits(7);
          }
          if (err || index + count > nlen + ndist) return null;
          while (count--) symlens[index++] = repLen;
        }
      }
      if (!symlens[256]) return null;      // no end-of-block code
      var lc = noteHuffConstruct(symlens.slice(0, nlen), nlen);
      var dc = noteHuffConstruct(symlens.slice(nlen), ndist);
      if (!lc || !dc) return null;
      return { len: lc, dist: dc };
    }

    var last, type, blockOK;
    do {
      last = bits(1);
      type = bits(2);
      if (err) return null;
      if (type === 0) {
        blockOK = stored();
      } else if (type === 1) {
        if (!noteFixedHuff) {
          var fl = [], fi;
          for (fi = 0; fi < 144; fi++) fl[fi] = 8;
          for (; fi < 256; fi++) fl[fi] = 9;
          for (; fi < 280; fi++) fl[fi] = 7;
          for (; fi < 288; fi++) fl[fi] = 8;
          var fd = [];
          for (fi = 0; fi < 30; fi++) fd[fi] = 5;
          noteFixedHuff = { len: noteHuffConstruct(fl, 288, true), dist: noteHuffConstruct(fd, 30, true) };
        }
        blockOK = codes(noteFixedHuff.len, noteFixedHuff.dist);
      } else if (type === 2) {
        var dyn = dynamicTables();
        blockOK = dyn ? codes(dyn.len, dyn.dist) : false;
      } else {
        blockOK = false;
      }
      if (!blockOK || err) return null;
    } while (!last);
    return out;                            // trailing input bytes are ignored
  }

  function noteBase64Bytes(payload) {
    try {
      // EXACTLY what the app accepts, no more. Go uses RawURLEncoding — the
      // url-safe alphabet with no padding — and tolerates one thing beyond it:
      // correctly-formed TRAILING padding (it switches to URLEncoding when the
      // payload ends in '='). atob accepts the standard alphabet too, so
      // mapping -_ to +/ and topping up '=' quietly took payloads the app
      // calls damaged: a '+' or '/' spelling, interior '=', or short padding
      // atob would forgive once we padded it. Each was an ok-here /
      // damaged-in-app divergence.
      if (/[+\/]/.test(payload)) return null;
      var eq = payload.indexOf('=');
      if (eq !== -1) {
        if (!/^[A-Za-z0-9_-]+={1,2}$/.test(payload) || payload.length % 4 !== 0) return null;
      }
      var b64 = payload.replace(/-/g, '+').replace(/_/g, '/');
      if (eq === -1) { while (b64.length % 4) b64 += '='; }
      var bin = atob(b64);
      var out = new Uint8Array(bin.length);
      for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
      return out;
    } catch (e) { return null; }
  }

  // Strict UTF-8, or null. THE FIX for a real divergence: this decoder used
  // to decode non-fatally, rendering U+FFFD where the app decoder shows
  // nothing — invalid UTF-8 is DAMAGED here exactly as it is in Go
  // (utf8.Valid).
  function noteUTF8(bytes) {
    try {
      return new TextDecoder('utf-8', { fatal: true }).decode(
        bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes));
    } catch (e) { return null; }
  }

  // The browser half of normalizeNote (share_note.go): strip the control
  // characters and bidi overrides that let text hide or reverse itself — and
  // U+FFFD, which Go's rune walk drops — keep the newlines that make a note
  // readable, and cap the length in code points.
  function cleanNote(s) {
    if (!s) return '';
    s = s.replace(/\r\n?/g, '\n')
         // C0 and C1 controls, keeping tab (09) and newline (0a).
         .replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g, '')
         // Bidi marks, embeddings, isolates, the BOM — and U+FFFD.
         .replace(/[\u200e\u200f\u202a-\u202e\u2066-\u2069\ufeff\ufffd]/g, '')
         .replace(/\n{3,}/g, '\n\n')
         .trim();
    var runes = Array.from(s);
    if (runes.length > NOTE_MAX_RUNES) s = runes.slice(0, NOTE_MAX_RUNES).join('').trim();
    return s;
  }

  // binary.Uvarint's twin: [value, bytesRead]; bytesRead <= 0 is an error
  // (0 = truncated, negative = overflow), exactly Go's contract.
  function noteUvarint(s, at) {
    var x = 0, mult = 1;
    for (var k = 0; k < 10; k++) {
      if (at + k >= s.length) return [0, 0];
      var b = s[at + k];
      if (b < 0x80) {
        if (k === 9 && b > 1) return [0, -10];
        return [x + b * mult, k + 1];
      }
      x += (b & 0x7f) * mult;
      mult *= 128;
    }
    return [0, -10];
  }

  function noteLegacyText(raw) {
    var s = noteUTF8(raw);
    if (s === null) return { outcome: 'damaged', text: '' };
    var text = cleanNote(s);
    if (!text) return { outcome: 'damaged', text: '' };
    return { outcome: 'ok', text: text };
  }

  // The record stream. Reject ONLY framing failures; any order; duplicate tag
  // first-occurrence-wins; unknown lowercase tags skipped by their length;
  // unknown UPPERCASE = a newer format; 0xFF = stop and render what we have.
  // The page renders only 't' — the app additionally decodes v/b/c/a, none of
  // which can change the outcome, and preserves what it skips (the page never
  // re-emits, so it has nothing to preserve for).
  function noteParseRecords(s) {
    var textRaw = null, sawText = false;
    var i = 0;
    while (i < s.length) {
      var tag = s[i];
      if (tag === 0xff) break;             // stop: render what was read
      i++;
      var uv = noteUvarint(s, i);
      if (uv[1] <= 0) return { outcome: 'damaged', text: '' };
      var len = uv[0];
      // Checked against the REMAINING bytes before any offset arithmetic.
      if (len > s.length - i - uv[1]) return { outcome: 'damaged', text: '' };
      i += uv[1];
      var val = s.slice(i, i + len);
      i += len;
      if (tag >= 65 && tag <= 90) return { outcome: 'newer', text: '' };
      if (tag === 116 /* t */ && !sawText) {
        sawText = true;
        textRaw = val;
      }
    }
    if (!sawText) return { outcome: 'damaged', text: '' };
    return noteLegacyText(textRaw);
  }

  // decodeNotePayload — the whole decoder: {outcome: 'ok'|'newer'|'damaged',
  // text}. Synchronous, pure, corpus-tested against the Go decoder.
  function decodeNotePayload(payload) {
    var damaged = { outcome: 'damaged', text: '' };
    payload = (payload || '').trim();
    if (!payload) return damaged;
    var bytes = noteBase64Bytes(payload);
    if (!bytes || bytes.length < 2) return damaged;
    var b0 = bytes[0], body = bytes.subarray(1);
    if (b0 === 112 /* p */) return noteLegacyText(body);
    if (b0 === 122 /* z */) {
      var raw = noteInflateRaw(body, NOTE_MAX_INFLATED);
      return raw === null ? damaged : noteLegacyText(raw);
    }
    if (b0 === 114 /* r */) {
      if (body.length > NOTE_MAX_RECORD_BYTES) return damaged;
      return noteParseRecords(body);
    }
    if (b0 === 100 /* d */) {
      var stream = noteInflateRaw(body, NOTE_MAX_RECORD_BYTES);
      return stream === null ? damaged : noteParseRecords(stream);
    }
    if (b0 >= 65 && b0 <= 90) return { outcome: 'newer', text: '' };
    return damaged;                        // unknown byte 0: framing, fail closed
  }
  /*__NOTE_DECODER_END__*/

  var noteBox = null, noteChip = null;

  var ICON_MINIMIZE = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 8.75h10a.75.75 0 0 0 0-1.5H3a.75.75 0 0 0 0 1.5z"/></svg>';
  var ICON_TRASH = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M6.5 1.75a.75.75 0 0 0-.75.75V3H3a.75.75 0 0 0 0 1.5h.4l.62 8.06A1.75 1.75 0 0 0 5.77 14h4.46a1.75 1.75 0 0 0 1.75-1.44l.62-8.06H13A.75.75 0 0 0 13 3h-2.75v-.5a.75.75 0 0 0-.75-.75h-3zm.75 1.25h1.5V3h-1.5v-.0zM5.9 4.5h4.2l-.6 7.9a.25.25 0 0 1-.25.1H6.75a.25.25 0 0 1-.25-.1L5.9 4.5z"/></svg>';
  var ICON_NOTE = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 2.75A1.75 1.75 0 0 1 4.75 1h6.5A1.75 1.75 0 0 1 13 2.75v10.5A1.75 1.75 0 0 1 11.25 15h-6.5A1.75 1.75 0 0 1 3 13.25V2.75zM5.5 4.5a.75.75 0 0 0 0 1.5h5a.75.75 0 0 0 0-1.5h-5zm0 3a.75.75 0 0 0 0 1.5h5a.75.75 0 0 0 0-1.5h-5zm0 3a.75.75 0 0 0 0 1.5h3a.75.75 0 0 0 0-1.5h-3z"/></svg>';

  function showNote(text) {
    hideNote();
    var wrap = document.querySelector('.wrap');
    if (!wrap || !text) return;

    var box = document.createElement('aside');
    box.className = 'note';

    var who = document.createElement('p');
    who.className = 'notewho';
    // Composed in Go (senderByline) and emitted at generate time — the same
    // attribution every app pane draws, not this file's own literal. Still a
    // person, never "from BibleText".
    who.textContent = __NOTE_BYLINE__;

    var body = document.createElement('p');
    body.className = 'notetext';
    body.textContent = text;                             // TEXT, never markup

    // Two controls, and the difference between them is the whole point:
    // MINIMIZE is reversible and takes the highlight down with the note, so the
    // reader can see the passage plainly and bring the message back. TRASH is
    // the one that throws it away. This is the RECEIVED arm of the shared verb
    // table (noteChrome.verbs) — the OWN arm does not exist here on purpose,
    // because a link's note is always somebody else's; that absence is this
    // surface's reasoned divergence from the four-pane verb set.
    var tools = document.createElement('div');
    tools.className = 'notetools';
    tools.appendChild(noteButton('Minimize note', ICON_MINIMIZE, function () {
      minimizeNote(text);
    }));
    tools.appendChild(noteButton('Delete note', ICON_TRASH, function () {
      trashNote();
    }));

    box.appendChild(tools); box.appendChild(who); box.appendChild(body);
    anchorToPassage(box);
    noteBox = box;
    activeNoteText = text;
    rescrollToHighlight();
  }

  function noteButton(label, svg, onTap) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'notebtn';
    b.setAttribute('aria-label', label);
    b.title = label;
    b.innerHTML = svg;                 // our own markup, never the note's
    b.addEventListener('click', function (e) { e.preventDefault(); onTap(); });
    return b;
  }

  // Minimize: the note collapses to a marker AND the highlight comes down with
  // it, so the reader gets the passage as it would normally read. Nothing is
  // lost — the fragment still carries both, so this survives a reload and the
  // marker puts everything back.
  function minimizeNote(text) {
    hideNote();
    // suppressHighlight(true) is UNCONDITIONAL where the app's hide clears
    // only the mark the note itself owns. Reasoned: this page has no other
    // highlight source — no search, no cross-references, no go-to — so the
    // only wash a note page can carry is the note's own, and the global
    // suppress is the same act with less machinery. If this page ever grows a
    // second highlight source, this is the line that must learn ownership.
    suppressHighlight(true);
    var chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'notechip';
    chip.setAttribute('aria-label', 'Show note');
    // The label is the shared collapsed-pill wording (stickerPillWho), for
    // this page's structural case of one placed note, emitted at generate
    // time. textContent, so the wording can change in Go without this file
    // growing markup.
    chip.innerHTML = ICON_NOTE + '<span></span>';
    chip.querySelector('span').textContent = __NOTE_PILL_LABEL__;
    chip.addEventListener('click', function (e) {
      e.preventDefault();
      suppressHighlight(false);
      showNote(text);
    });
    anchorToPassage(chip);
    noteChip = chip;
    activeNoteText = text;
  }

  // Trash: the note and the highlight both go, and the fragment goes with them
  // so a reload does not resurrect what the reader threw away.
  function trashNote() {
    activeNoteText = '';
    hideNote();
    suppressHighlight(true);
    history.replaceState(null, '', location.pathname + location.search);
    carryVerse();
  }

  // Hide or restore the highlight without touching the URL, so minimize is
  // reversible. The nohl flag is what beats the bare CSS :target rule.
  function suppressHighlight(off) {
    if (off) {
      document.documentElement.classList.add('nohl');
      document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    } else {
      document.documentElement.classList.remove('nohl');
      highlightRange();
    }
  }

  function hideNote() {
    if (noteBox) { noteBox.remove(); noteBox = null; }
    if (noteChip) { noteChip.remove(); noteChip = null; }
  }

  // The note belongs to the passage, so it goes into the FLOW immediately above
  // the paragraph holding the highlighted verse — not floating, and not at the
  // top of the chapter. Two reasons: it can never cover the words it is about,
  // and a reader arriving on a shared link lands mid-chapter at their verse, so
  // a note parked at the top of the page is a note they never see.
  // The paragraph the note belongs to, remembered the first time we can see it.
  // It has to be remembered: minimizing CLEARS the highlight, so by the time the
  // marker is placed there is no .v.hl left to anchor to and the marker would
  // jump to the top of the chapter — which is exactly what it did.
  var noteAnchorPara = null;

  function anchorToPassage(el) {
    var lit = document.querySelector('.v.hl') || document.querySelector('.v:target');
    var para = (lit && lit.closest ? lit.closest('p') : null) || noteAnchorPara;
    if (para && para.parentNode) {
      noteAnchorPara = para;
      para.parentNode.insertBefore(el, para);
      el.classList.remove('notail');
      return;
    }
    // The chapter-top parking: nothing on this page to point at. The shared
    // rule (noteChrome.hasTail) says a tail exists iff the anchor names a
    // passage — an unconditional tail here claimed whatever paragraph came
    // first, and was reachable through any note link without a verse.
    el.classList.add('notail');
    var text = document.querySelector('.text');
    if (text && text.parentNode) text.parentNode.insertBefore(el, text);
    else document.querySelector('.wrap').appendChild(el);
  }

  // Inserting the note pushes the passage down, so whatever scroll brought the
  // reader to their verse is now pointing at the wrong place. Put it back — and
  // when there IS a note, land on the NOTE rather than the verse.
  //
  // That matters more than it sounds. The note is anchored to the top of the
  // paragraph holding the verse, and a paragraph can be long: a note on John
  // 11:35 sits with verse 30, some 570px above the verse, so centring the verse
  // scrolled the message clean off the top of the screen. The note is the reason
  // the link was sent, so it is what the reader should arrive at; the passage
  // follows immediately under it.
  function rescrollToHighlight() {
    var go = function () {
      var lit = document.querySelector('.v.hl') || document.querySelector('.v:target');
      var card = noteBox || noteChip;
      // THE SAME QUESTION THE OTHER FOUR SURFACES ASK, and this one used to ask
      // nothing at all: its only gate was a null-check on an element
      // assigned two statements before the call, so the note always won. That is
      // right on a fresh arrival — anchorToPassage inserts the card immediately
      // before the paragraph holding the lit verse — and wrong the moment the
      // anchor is STALE, which is exactly what the noteAnchorPara fallback
      // there produces: a hashchange to another passage then scrolled the reader
      // back to a note about the one they had left.
      //
      // So: the card wins when it belongs to the paragraph being arrived at.
      // Asking the DOM for that paragraph is not re-deriving the rule — these <p>
      // elements ARE the model's paragraphs, which is now enforced rather than
      // assumed (TestEverySurfaceBreaksParagraphsWhereTheModelDoes).
      if (card) {
        var para = lit && lit.closest ? lit.closest('p') : null;
        if (!para || card.nextElementSibling === para) {
          card.scrollIntoView({ block: 'start' });
          return;
        }
      }
      if (lit) lit.scrollIntoView({ block: 'center' });
    };
    // AFTER LAYOUT, not merely after insertion. Scrolling in the same turn as
    // the insert measured the old layout and landed exactly the note's own
    // height short — the note sat just above the top of the screen with only its
    // tail showing. Two frames is the reliable "the box has a height now" point.
    if (window.requestAnimationFrame) {
      requestAnimationFrame(function () { requestAnimationFrame(go); });
    } else {
      setTimeout(go, 0);
    }
  }

  // THE COULD-NOT-READ NOTICE. When the payload cannot be rendered the reader
  // is TOLD, in the note's own place in the flow — never silently
  // (docs/NOTE_WIRE_FORMAT.md rule 5). It is the app's sentence, not a
  // person's, so: no byline, no buttons, NO LINK and no install prompt — a
  // note arrives inside a message thread, and "update to read this" is a
  // phishing template in our own typeface on our own domain.
  var noticeBox = null;
  function hideNotice() { if (noticeBox) { noticeBox.remove(); noticeBox = null; } }
  function showNotice(msg) {
    hideNotice();
    var box = document.createElement('aside');
    box.className = 'notenotice';
    box.textContent = msg;                 // our own fixed sentence, still TEXT
    anchorToPassage(box);
    noticeBox = box;
  }

  function renderNote() {
    hideNote();
    hideNotice();
    var keys = fragKeys();
    // Only a fragment that CARRIES an "n" key gets a verdict: a plain passage
    // link shows nothing here. An "n=" that arrived EMPTY is a verdict —
    // damaged — exactly as in the app's parser (share_link_parse.go).
    if (!('n' in keys)) return;
    var got = decodeNotePayload(keys.n);
    if (got.outcome === 'ok') { showNote(got.text); return; }
    showNotice(got.outcome === 'newer' ? NOTE_MSG_NEWER : NOTE_MSG_DAMAGED);
  }

  renderNote();
  window.addEventListener('hashchange', renderNote);

  // 2) Carry the verse across a translation switch. The switcher's hrefs are
  // plain chapter links (they must be: the fragment is not known at build
  // time), so without this a reader who followed a shared John 3:16 link and
  // tapped "BSB" to compare would land at the top of the chapter with no idea
  // which verse was shared — on the page that exists to show that one verse.
  function carryVerse() {
    // Carry the whole fragment, not just the verse: a reader who followed a
    // link with a note and taps BSB to compare is still reading the same
    // message about the same passage, so the note travels with them.
    var keys = fragKeys();
    var parts = [];
    if (verseSpan()) parts.push('v' + keys.v);
    if (keys.n) parts.push('n=' + keys.n);
    var hash = parts.length ? '#' + parts.join('&') : '';
    document.querySelectorAll('.vpick').forEach(function (a) {
      var base = (a.getAttribute('href') || '').split('#')[0];
      a.setAttribute('href', base + hash);
    });
  }
  carryVerse();
  window.addEventListener('hashchange', carryVerse);

  // 3) "Get the app" — point Apple devices at the App Store, everyone else at
  // the landing page (which is already the no-JS default href).
  var a = document.getElementById('getapp');
  if (a && /iPhone|iPad|iPod/.test(navigator.userAgent)) {
    a.href = 'https://apps.apple.com/app/id6784567351';
  }
})();
`
