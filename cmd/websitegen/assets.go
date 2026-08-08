package main

// The site's entire client-side surface: one stylesheet and one small script.
//
// The colours are the app's palette (theme.go) transcribed to CSS so the web
// page and the app are recognisably the same product — warm parchment in light,
// warm near-black with a luminous sapphire accent in dark. The type is a system
// serif stack: Georgia is present on Apple and Windows devices, and a serif
// fallback covers the rest. Shipping no webfont keeps a shared link as fast as
// it can possibly be, which matters more here than exact glyph parity — this
// page is usually opened on a phone, on mobile data, from a message.

const readerCSS = `
:root{
  --bg:#ede9e0; --surface:#fdfcf8; --text:#25221d; --muted:#6b6456;
  --accent:#2f4c86; --border:#bdb29f; --verse:#53688f; --red:#b23a2e;
  --hl:#dde7f7;
}
@media (prefers-color-scheme:dark){
  :root{
    --bg:#191715; --surface:#221f1c; --text:#e9e3d9; --muted:#9d9487;
    --accent:#7ca0e4; --border:#39342e; --verse:#8ca8d8; --red:#e57373;
    --hl:#222d42;
  }
}
*{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{
  margin:0; background:var(--bg); color:var(--text);
  font-family:Georgia,'Times New Roman',serif; line-height:1.6;
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
.goto:hover{border-color:var(--accent); background:var(--hl)}
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
  /* A system sans renders arrows with real weight; Georgia draws them as
     hairlines that vanish on a phone screen. */
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  font-size:1.45rem; font-weight:700; line-height:1;
  color:var(--accent); text-decoration:none;
}
.arrow:hover{border-color:var(--accent); background:var(--hl)}
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
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif}
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
.gchaps a.on{border-color:var(--accent); color:var(--accent); font-weight:700; background:var(--hl)}
.gbooks{display:block}
.gbooks a{display:block; padding:.35rem .4rem; border-radius:6px; font-size:.92rem;
  color:var(--text); text-decoration:none}
.gbooks a:hover{background:var(--hl)}
.gbooks a.on{color:var(--accent); font-weight:700}
/* The way back to the alphabet, and the heading telling you which letter you
   are in. It was a muted ‹ and a small letter — easy to miss and easy to miss
   with a thumb. Accent-coloured, bold, with a rule under it so it reads as the
   pane's header rather than the first entry in the book list. */
.gback{display:flex; align-items:center; gap:.45rem;
  padding:.25rem .4rem .5rem; margin:0 0 .4rem;
  border-bottom:1px solid var(--border);
  color:var(--accent); text-decoration:none; font-weight:700;
  font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif}
.gback:hover{border-bottom-color:var(--accent)}
.bkarrow{font-size:1.3rem; line-height:1}
.bkletter{font-size:1.05rem; letter-spacing:.02em}
/* Verse RANGE as two number fields — "verse [16] to [18]" — so there is no
   hyphen to type on a phone number pad. Exactly the app's row. */
.gverse{display:flex; align-items:center; gap:.5rem; margin:.9rem 0 0;
  padding:.8rem 0 0; border-top:1px solid var(--border)}
.gverse input{
  width:4.6rem; font-family:Georgia,serif; font-size:1rem; padding:.4rem .5rem;
  color:var(--text); background:var(--bg); text-align:center;
  border:1px solid var(--border); border-radius:8px; outline:none; min-width:0;
}
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
.text{font-size:1.28rem; line-height:1.62}
.text p{margin:0 0 1.1rem; text-align:justify; hyphens:auto}
/* Paragraph shape mirrors the app: on a phone, paragraphs are separated by
   space (the app's phone reading pane); from tablet width up the app switches
   to its iPad "reporter" setting — a first-line indent with no blank line
   between paragraphs, which is how a printed Bible sets prose. A paragraph
   that OPENS with poetry keeps its space and takes no indent, matching the
   app's rule that the reporter indent is skipped for poetic paragraphs. */
@media (min-width:46rem){
  .text p{margin:0; text-indent:1.4em; line-height:1.5}
  .text p:first-child{text-indent:0}
  .text p.pm{margin:.55rem 0; text-indent:0}
  .text p.pm + p{text-indent:0}
}
.text p.pm{text-align:left; hyphens:none}
.n{font-size:.62em; vertical-align:.45em; line-height:0}
.n a{color:var(--verse); text-decoration:none}
.n a:hover{text-decoration:underline}
.wj{color:var(--red)}
/* A single-verse link works with NO JavaScript: the fragment matches the
   verse's id and :target paints it. reader.js only adds ranges. */
/* Generous scroll-margin so a deep link lands the verse with the chapter
   heading still on screen, rather than pinning it to the very top edge. */
.v:target,.v.hl{background:var(--hl); border-radius:3px;
  box-shadow:0 0 0 .18em var(--hl); scroll-margin-top:6.5rem}
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
/* Clearing a highlight is a TAP ON THE HIGHLIGHT, the way the app does it —
   no floating control, nothing covering the text. The cursor hints at it on a
   pointer device; on touch it is simply the obvious thing to try. */
.v:target,.v.hl{cursor:pointer}
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

// readerJS holds the few things that need scripting, all progressive
// enhancements: with JS off a single-verse link still highlights (CSS :target),
// "Get the app" still points at the all-platforms landing page, and the only
// thing genuinely lost is the clear-highlight control (navigating clears it).
const readerJSTemplate = `
(function () {
  // 1) Verse RANGES (#v16-18). A single verse needs no help — :target has it.
  function highlightRange() {
    document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    var m = /^#v(\d+)(?:-(\d+))?$/.exec(location.hash || '');
    if (!m) return;
    // A fresh fragment re-lights the verse: drop the suppression flag a
    // previous clear may have set.
    document.documentElement.classList.remove('nohl');
    var lo = parseInt(m[1], 10), hi = m[2] ? parseInt(m[2], 10) : lo;
    if (!(lo > 0) || hi < lo) return;
    var first = null;
    for (var n = lo; n <= hi; n++) {
      var el = document.getElementById('v' + n);
      if (!el) continue;            // out of range: highlight what exists, never error
      el.classList.add('hl');
      if (!first) first = el;
    }
    if (first && hi > lo) first.scrollIntoView({ block: 'center' });
  }
  // Clearing a highlight: tap the highlighted text, like the app. Removing the
  // fragment is what does the work — it un-targets the CSS :target rule (single
  // verse) as well as letting us drop the range classes.
  function highlighted() { return /^#v\d+(-\d+)?$/.test(location.hash || ''); }

  function clearHighlight() {
    // replaceState, not a hash change: it leaves no extra history entry, so Back
    // still returns where the reader came from rather than re-highlighting.
    history.replaceState(null, '', location.pathname + location.search);
    // Dropping the fragment does NOT un-target the verse — browsers keep the
    // :target match until a real navigation — so a single-verse highlight (the
    // usual shared link) would stay lit. This flag overrides it in CSS.
    document.documentElement.classList.add('nohl');
    document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    // replaceState fires NO hashchange, so anything reading the fragment has to
    // be updated by hand — otherwise the version switcher keeps carrying a verse
    // that is no longer highlighted.
    carryVerse();
  }

  document.addEventListener('click', function (e) {
    if (!highlighted()) return;
    // Ignore a click that is really a text selection, and never swallow a link.
    var sel = window.getSelection();
    if (sel && !sel.isCollapsed) return;
    if (e.target.closest('a')) return;
    if (e.target.closest('.v.hl') || e.target.closest('.v:target')) clearHighlight();
  });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && highlighted()) clearHighlight();
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
          '<input type="text" inputmode="numeric" pattern="[0-9]*" placeholder="verse" aria-label="Verse">' +
          '<span class="gto">to</span>' +
          '<input type="text" inputmode="numeric" pattern="[0-9]*" placeholder="end" aria-label="End verse">' +
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

  // 2) Carry the verse across a translation switch. The switcher's hrefs are
  // plain chapter links (they must be: the fragment is not known at build
  // time), so without this a reader who followed a shared John 3:16 link and
  // tapped "BSB" to compare would land at the top of the chapter with no idea
  // which verse was shared — on the page that exists to show that one verse.
  function carryVerse() {
    var hash = /^#v\d+(-\d+)?$/.test(location.hash || '') ? location.hash : '';
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
