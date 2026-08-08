
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
