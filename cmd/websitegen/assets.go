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
.wrap{max-width:38rem; margin:0 auto; padding:1rem 1.15rem 2rem}
.top{
  display:flex; flex-wrap:wrap; align-items:center; gap:.5rem;
  font-size:.8rem; padding:.35rem 0 .9rem; border-bottom:1px solid var(--border);
}
.home{font-weight:700; color:var(--text); text-decoration:none; letter-spacing:.01em}
.crumbs{color:var(--muted)} .crumbs a{color:var(--muted); text-decoration:none}
.crumbs a:hover{text-decoration:underline} .sep{opacity:.5; padding:0 .15rem}
.vers{margin-left:auto; display:flex; gap:.3rem}
.vpick{
  color:var(--muted); text-decoration:none; font-size:.7rem; letter-spacing:.04em;
  border:1px solid var(--border); border-radius:999px; padding:.15rem .5rem;
}
.vpick.on{color:var(--accent); border-color:var(--accent)}
.ref{font-size:1.9rem; margin:1.1rem 0 .1rem; font-weight:700; letter-spacing:-.01em}
.ver{margin:0 0 1.3rem; color:var(--muted); font-size:.85rem}
.text{font-size:1.16rem}
.text p{margin:0 0 1.1rem; text-align:justify; hyphens:auto}
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
.foot{max-width:38rem; margin:0 auto; padding:0 1.15rem 2.5rem; text-align:center}
.foot a{color:var(--muted); font-size:.8rem; text-decoration:none}
.foot a:hover{color:var(--accent); text-decoration:underline}
@media print{.top,.pager,.foot{display:none}}
`

// readerJS holds the only two things that need scripting. Both are progressive
// enhancements: with JS off, a single-verse link still highlights (CSS :target),
// and "Get the app" still points at the all-platforms landing page.
const readerJS = `
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
  highlightRange();
  window.addEventListener('hashchange', highlightRange);

  // 2) "Get the app" — point Apple devices at the App Store, everyone else at
  // the landing page (which is already the no-JS default href).
  var a = document.getElementById('getapp');
  if (a && /iPhone|iPad|iPod/.test(navigator.userAgent)) {
    a.href = 'https://apps.apple.com/app/id6784567351';
  }
})();
`
