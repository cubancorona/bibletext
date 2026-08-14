package main

// The notice pages' own stylesheet and script — a SECOND pair of assets, loaded
// only by the pages in notice.go, on top of reader.css/reader.js.
//
// WHY A SECOND PAIR RATHER THAN A FEW MORE RULES IN reader.css. The asset
// filenames are content-hashed, and that hash appears in the <link> and
// <script> of every published page. Adding one rule to readerCSSTemplate would
// therefore rewrite all 3,906 files under /web/, /bsb/ and /webc/ — which this
// change is required not to do, and which would also make every reader on the
// site re-download a stylesheet for a feature none of those pages use. Two
// small files that no published page requests is the cheaper answer in both
// directions.
//
// The notice pages still load reader.css and reader.js first, so they inherit
// the palette, the type, the chrome, the grid and — the part that matters — the
// sender's note, which only reader.js can decode.

// noticeCSSName/noticeJSName are the content-hashed paths for this build, set
// in writeSite before any notice page is rendered (the same contract cssName
// and jsName have).
var noticeCSSName, noticeJSName string

// noticeCSS is deliberately small: everything it does not define is inherited
// from reader.css, so the notice page and a chapter page are visibly the same
// site.
const noticeCSS = `
/* The notice pages spell the translations out (noticeNav) — this row is three
   or four multi-word names, not four four-letter pills — so it has to wrap, or
   on a 375px phone it overflows the viewport or squeezes the crumbs to nothing.
   Right-aligned wrapping keeps it reading as one block hanging off the nav.

   IT LIVES HERE, NOT IN reader.css, AND THAT IS THE POINT. reader.css is
   content-hashed into the <link> of all 3,906 scripture pages, so adding one
   rule there rewrites every one of them — I did exactly that and turned a
   0-pages-changed publish into 3,914. This stylesheet is loaded only by notice
   pages, so the rule reaches the pages that need it and nothing else moves. */
.vers-full{flex-wrap:wrap; justify-content:flex-end; row-gap:.25rem}
/* The explanation. Same rhythm as the 404's .guess, one size up from the
   chrome default, because on this page the paragraph IS the content. */
.lede{margin:0 0 1.6rem; font-size:1.05rem}
.lede a{color:var(--accent)}
/* Each route out is its own block with its own heading, so "open it in the app"
   and "read it in another translation" are two offers rather than one wall. */
.nsec{margin:0 0 1.8rem; padding:1rem 0 0; border-top:1px solid var(--border)}
.nsec h2{margin:0 0 .5rem; font-size:1.05rem; letter-spacing:-.005em}
.nsec p{margin:0 0 .8rem; font-size:.95rem}
/* The button's row. NOT reader.css's .gotorow, which is the header's "Go to"
   line and carries a bottom border to close the whole header — reused here it
   drew a rule between the button and the line explaining it, which read as the
   start of the next section. Same centring, no border. */
.appbtnrow{display:flex; justify-content:center; padding:.2rem 0 0}
/* The one action on the page, so it is allowed to be bigger than .goto — but it
   keeps .goto's unfilled shape, because a solid accent slab next to scripture
   chrome reads as an advert. */
.btnapp{font-size:.95rem; padding:.6rem 1.4rem}
.opensub{margin:.55rem 0 0; color:var(--muted); font-size:.8rem}
/* The translation pills. A near-copy of .vpick on purpose: these are NOT
   .vpick, because reader.js rewrites every .vpick href with the whole fragment
   and these links must only carry the verse where the numbering agrees. Same
   look, different owner. */
.npick{
  color:var(--muted); text-decoration:none; font-size:.7rem; letter-spacing:.04em;
  border:1px solid var(--border); border-radius:999px; padding:.15rem .5rem;
}
.npick.on{color:var(--accent); border-color:var(--accent)}
/* A versification caveat. Muted and italic like .empty, with room around it —
   it qualifies the links above it and must not read as one of them. */
.vnote{margin:.7rem 0 0; color:var(--muted); font-style:italic; font-size:.85rem}
`

// noticeJS does the three things the SERVER cannot, because the verse and the
// sender's note ride in the fragment and a fragment never reaches a server:
//
//  1. name the exact passage ("John 3" -> "John 3:16")
//  2. put the verse and the note onto the parallel-passage links
//  3. narrow the one app affordance to what the reader's platform can do
//
// With scripting off all three degrade to something honest rather than to
// something wrong: the page names the chapter, the links open the chapter, and
// the button offers the download — which works on every platform there is.
const noticeJS = `
(function () {
  // The fragment is a KEY LIST, "&"-separated (share_link.go): the verse span
  // written bare as "v16" / "v16-18", plus optional keys like "n=<note>".
  // Unknown keys are ignored, which is what lets a future key be added without
  // stranding links already sent.
  //
  // This repeats reader.js's parser. reader.js exports nothing and its bytes
  // are frozen — they are in the filename every published page links — so the
  // choice was twenty duplicated lines here or rewriting the whole site's
  // assets. Nothing SECURITY-sensitive is duplicated: the note payload is never
  // decoded here, only passed along as the opaque string it arrived as, and
  // reader.js remains the only thing on the site that turns it into text.
  function fragKeys() {
    var out = { v: '' };
    var raw = (location.hash || '').replace(/^#/, '');
    if (!raw) return out;
    raw.split('&').forEach(function (kv, i) {
      var eq = kv.indexOf('=');
      if (eq < 0) {
        if (i === 0 && /^v\d/.test(kv)) out.v = kv.slice(1);
        return;
      }
      out[kv.slice(0, eq)] = kv.slice(eq + 1);
    });
    return out;
  }

  function verseSpan() {
    var m = /^(\d+)(?:-(\d+))?$/.exec(fragKeys().v || '');
    if (!m) return null;
    var lo = parseInt(m[1], 10), hi = m[2] ? parseInt(m[2], 10) : lo;
    if (!(lo > 0) || hi < lo) return null;
    return [lo, hi];
  }

  // ":16" or ":16-18", or "" when the link named no verse.
  function verseLabel() {
    var s = verseSpan();
    if (!s) return '';
    return ':' + s[0] + (s[1] > s[0] ? '-' + s[1] : '');
  }

  // 1) NAME THE PASSAGE. The heading and every reference in the prose are
  // rendered as the chapter, because that is all the server was told; this
  // completes them from the URL the reader actually followed.
  //
  // It reads from a remembered base rather than appending to what is on screen,
  // and it re-runs on hashchange alongside wireOffers below. Appending was the
  // first cut and it is wrong twice over: run it again and the heading reads
  // "John 3:16:16", and if the fragment ever changes the links would update
  // while the heading kept naming the old verse — a page disagreeing with
  // itself about which passage this is.
  var heading = document.querySelector('h1 .passage');
  var baseRef = heading ? heading.textContent : '';
  var baseTitle = document.title;

  function nameThePassage() {
    var label = verseLabel();
    document.querySelectorAll('.passage').forEach(function (el) {
      if (el.getAttribute('data-ref') === null) el.setAttribute('data-ref', el.textContent);
      el.textContent = el.getAttribute('data-ref') + label;
    });
    // The tab title too, so a reader with several of these open can tell them
    // apart. Not the og: tags — an unfurler never runs this and never sees the
    // fragment either, so its preview is honestly chapter-level.
    if (baseRef) document.title = baseTitle.replace(baseRef, baseRef + label);
  }
  nameThePassage();
  window.addEventListener('hashchange', nameThePassage);

  // 2) THE PARALLEL LINKS. data-frag says how much of the fragment a link may
  // carry:
  //   verse  the numbering agrees for this chapter, so the verse travels
  //   note   it does not, so only the sender's note travels and the link opens
  //          the chapter — the page already says why, in .vnote
  // The note travels either way. It is the reason the link was sent, and losing
  // it at a translation switch is exactly the failure docs/NKJV_FLOW.md calls
  // I3.
  function wireOffers() {
    var keys = fragKeys();
    document.querySelectorAll('a[data-frag]').forEach(function (a) {
      var parts = [];
      if (a.getAttribute('data-frag') === 'verse' && verseSpan()) parts.push('v' + keys.v);
      if (keys.n) parts.push('n=' + keys.n);
      var base = (a.getAttribute('href') || '').split('#')[0];
      a.setAttribute('href', base + (parts.length ? '#' + parts.join('&') : ''));
    });
  }
  wireOffers();
  window.addEventListener('hashchange', wireOffers);

  // 3) OPEN IN APP. The server-rendered href is the all-platforms download
  // page, which is correct with no JavaScript and correct on every desktop —
  // there is no app for a desktop browser to hand off to. This narrows it where
  // a handoff is actually possible.
  var btn = document.getElementById('openapp');
  if (btn) {
    var ua = navigator.userAgent || '';
    // iPadOS reports itself as Macintosh; the touch points are what give it
    // away. A real Mac reports 0.
    var isIOS = /iPhone|iPad|iPod/.test(ua) ||
      (/Macintosh/.test(ua) && (navigator.maxTouchPoints || 0) > 1);
    var isAndroid = /Android/.test(ua);
    var label2 = btn.getAttribute('data-label') || 'Open in BibleText';
    if (isAndroid) {
      // intent:// really does hand the link to the app when it is installed,
      // and S.browser_fallback_url sends everyone else to the download page.
      //
      // KNOWN LOSS: the intent grammar has no room for the target's own
      // fragment (its "#" starts the Intent block, and percent-encoding it
      // would hand the app a path it cannot parse), so the app opens at the
      // CHAPTER and the verse and note do not cross. Closing that needs either
      // a custom URL scheme or an app that reads an S. extra — both app
      // changes, and neither is decided. When one lands, it is this branch and
      // nothing else.
      btn.setAttribute('href',
        'intent://' + location.host + location.pathname +
        '#Intent;scheme=https;package=' + (btn.getAttribute('data-pkg') || '') +
        ';S.browser_fallback_url=' + encodeURIComponent(btn.getAttribute('href') || '') +
        ';end');
      btn.textContent = label2;
    } else if (isIOS) {
      // Safari will not open the app from a link on the app's OWN domain, and
      // no custom scheme is registered — so the App Store product page is the
      // route, and it reads OPEN rather than GET when the app is installed.
      // The Smart App Banner in the head is the one-tap version of the same
      // thing.
      btn.setAttribute('href', btn.getAttribute('data-ios') || btn.getAttribute('href'));
      btn.textContent = label2;
      var note = document.getElementById('iosnote');
      if (note) note.hidden = false;
    }
  }
})();
`
