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
.goto{
  color:var(--text); text-decoration:none; font-size:.8rem; font-weight:700;
  background:var(--surface); border:1px solid var(--border); border-radius:9px;
  padding:.22rem .8rem; white-space:nowrap;
}
.goto:hover{border-color:var(--accent); color:var(--accent)}
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
   a bare glyph does. */
.arrow{
  display:inline-flex; align-items:center; justify-content:center;
  min-width:2.75rem; min-height:2.75rem;
  border:1px solid var(--border); border-radius:10px; background:var(--surface);
  font-size:1.35rem; line-height:1; color:var(--accent); text-decoration:none;
}
.arrow:hover{border-color:var(--accent); background:var(--hl)}
.arrow.off{color:var(--muted); opacity:.3; pointer-events:none; background:none}
/* Go-to overlay: a dim sheet with a single reference field. */
.gotobg{position:fixed; inset:0; background:rgba(0,0,0,.45); display:flex;
  align-items:flex-start; justify-content:center; padding:12vh 1rem 1rem; z-index:20}
.gotobox{background:var(--surface); border:1px solid var(--border); border-radius:12px;
  width:100%; max-width:26rem; padding:1rem; box-shadow:0 8px 30px rgba(0,0,0,.28)}
.gotobox input{
  width:100%; font-family:Georgia,serif; font-size:1.05rem; padding:.55rem .7rem;
  color:var(--text); background:var(--bg);
  border:1px solid var(--border); border-radius:8px; outline:none;
}
.gotobox input:focus{border-color:var(--accent)}
.gotohint{color:var(--muted); font-size:.78rem; margin:.5rem 0 0}
.gotolist{list-style:none; margin:.6rem 0 0; padding:0; max-height:40vh; overflow-y:auto}
.gotolist li a{display:block; padding:.4rem .5rem; border-radius:6px;
  color:var(--text); text-decoration:none; font-size:.95rem}
.gotolist li a:hover,.gotolist li a.on{background:var(--hl); color:var(--accent)}
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
/* The "clear highlight" pill. A shared link lands you scrolled to the verse, so
   a control at the top of the page would be off-screen exactly when it is
   wanted — this floats just above the thumb instead, and exists only while
   something is highlighted (reader.js adds it; with JS off there is no
   highlight to clear beyond a single :target verse, and navigating clears
   that). */
.clearhl{
  position:fixed; left:50%; transform:translateX(-50%); bottom:1.1rem;
  background:var(--surface); color:var(--accent);
  border:1px solid var(--border); border-radius:999px;
  padding:.4rem .9rem; font-size:.8rem; font-family:Georgia,serif;
  cursor:pointer; box-shadow:0 2px 10px rgba(0,0,0,.14);
}
.clearhl:hover{border-color:var(--accent)}
@media print{.clearhl{display:none}}
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
.plats svg{width:1.05rem; height:1.05rem; fill:currentColor; display:block}
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
  // Clearing a highlight. A shared link arrives highlighted, and until now
  // there was no way to put the page back to plain reading short of editing the
  // URL. Removing the fragment is what does the work: it un-targets the CSS
  // :target rule (single verse) as well as letting us drop the range classes.
  var pill = null;
  function highlighted() { return /^#v\d+(-\d+)?$/.test(location.hash || ''); }

  function clearHighlight() {
    // replaceState, not a hash change: it leaves no extra history entry, so Back
    // still returns where the reader came from rather than re-highlighting.
    history.replaceState(null, '', location.pathname + location.search);
    document.querySelectorAll('.v.hl').forEach(function (el) { el.classList.remove('hl'); });
    // replaceState does NOT fire hashchange, so the listeners below never run —
    // update everything that reads the fragment by hand.
    carryVerse();
    syncPill();
  }

  function syncPill() {
    if (!highlighted()) {
      if (pill) { pill.remove(); pill = null; }
      return;
    }
    if (pill) return;
    pill = document.createElement('button');
    pill.className = 'clearhl';
    pill.type = 'button';
    pill.textContent = 'Clear highlight';
    pill.addEventListener('click', clearHighlight);
    document.body.appendChild(pill);
  }

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && highlighted()) clearHighlight();
  });

  highlightRange();
  window.addEventListener('hashchange', function () { highlightRange(); syncPill(); });

  // 3) "Go to" — the app has one in its header, so the web reader does too.
  // The button is a real link to the book grid, so it works with no JavaScript;
  // this upgrades it into a type-ahead over the book list (injected at build
  // time from the SAME canonical names and slugs the pages are generated from,
  // so it can never reference a page that does not exist).
  // One table for the whole site, so reader.js is cached once rather than per
  // version. Each entry carries its chapter count PER version, because the
  // canons differ (Greek Daniel has 14 chapters under webc, 12 elsewhere) and a
  // suggestion must never point at a page that was not generated.
  var BOOKS = __BOOKS__;
  var VERSION = (location.pathname.split('/')[1] || 'web');

  function parseRef(q) {
    // "John 3", "1 cor 13:4", "ps23" — book prefix, then optional chapter.
    var m = /^\s*(.+?)\s*(\d+)?\s*(?::\s*(\d+))?\s*$/.exec(q || '');
    if (!m) return { hits: [], chapter: 0, verse: 0 };
    var name = (m[1] || '').toLowerCase().replace(/\s+/g, ' ').trim();
    var chapter = m[2] ? parseInt(m[2], 10) : 0;
    var verse = m[3] ? parseInt(m[3], 10) : 0;
    if (!name) return { hits: [], chapter: chapter, verse: verse };
    var starts = [], contains = [];
    BOOKS.forEach(function (b) {
      if (!b.ch[VERSION]) return;          // not in this translation's canon
      var n = b.name.toLowerCase();
      if (n.indexOf(name) === 0 || b.slug.indexOf(name.replace(/ /g, '-')) === 0) starts.push(b);
      else if (n.indexOf(name) >= 0) contains.push(b);
    });
    return { hits: starts.concat(contains).slice(0, 8), chapter: chapter, verse: verse };
  }

  function hrefFor(b, chapter, verse) {
    var max = b.ch[VERSION] || 1;
    var ch = chapter > 0 ? chapter : 1;
    if (ch > max) ch = max;
    return '/' + VERSION + '/' + b.slug + '/' + ch + '/' + (verse > 0 ? '#v' + verse : '');
  }

  function openGoto(ev) {
    if (ev) ev.preventDefault();
    var bg = document.createElement('div');
    bg.className = 'gotobg';
    bg.innerHTML = '<div class="gotobox"><input type="text" autocomplete="off" ' +
      'autocapitalize="none" spellcheck="false" placeholder="Book and chapter, e.g. John 3">' +
      '<p class="gotohint">Type a reference, then Enter. Escape closes.</p>' +
      '<ul class="gotolist"></ul></div>';
    document.body.appendChild(bg);
    var input = bg.querySelector('input'), list = bg.querySelector('.gotolist'), sel = 0;

    function render() {
      var r = parseRef(input.value);
      list.innerHTML = '';
      if (sel >= r.hits.length) sel = 0;
      r.hits.forEach(function (b, i) {
        var li = document.createElement('li');
        var a = document.createElement('a');
        a.href = hrefFor(b, r.chapter, r.verse);
        a.textContent = b.name + (r.chapter ? ' ' + Math.min(r.chapter, b.ch[VERSION] || 1) : '') +
          (r.verse ? ':' + r.verse : '');
        if (i === sel) a.className = 'on';
        li.appendChild(a); list.appendChild(li);
      });
      return r;
    }
    function close() { bg.remove(); document.removeEventListener('keydown', onKey, true); }
    function onKey(e) {
      if (e.key === 'Escape') { e.preventDefault(); close(); return; }
      var links = list.querySelectorAll('a');
      if (e.key === 'ArrowDown') { e.preventDefault(); sel = Math.min(sel + 1, links.length - 1); render(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); sel = Math.max(sel - 1, 0); render(); }
      else if (e.key === 'Enter') {
        var target = list.querySelectorAll('a')[sel];
        if (target) { e.preventDefault(); location.href = target.getAttribute('href'); }
      }
    }
    input.addEventListener('input', function () { sel = 0; render(); });
    document.addEventListener('keydown', onKey, true);
    bg.addEventListener('click', function (e) { if (e.target === bg) close(); });
    render();
    input.focus();
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
  syncPill(); // show the clear control if we arrived on a highlighted link

  // 3) "Get the app" — point Apple devices at the App Store, everyone else at
  // the landing page (which is already the no-JS default href).
  var a = document.getElementById('getapp');
  if (a && /iPhone|iPad|iPod/.test(navigator.userAgent)) {
    a.href = 'https://apps.apple.com/app/id6784567351';
  }
})();
`
