
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
  var BOOKS = [{"name":"Genesis","slug":"genesis","ch":{"bsb":50,"web":50,"webc":50}},{"name":"Exodus","slug":"exodus","ch":{"bsb":40,"web":40,"webc":40}},{"name":"Leviticus","slug":"leviticus","ch":{"bsb":27,"web":27,"webc":27}},{"name":"Numbers","slug":"numbers","ch":{"bsb":36,"web":36,"webc":36}},{"name":"Deuteronomy","slug":"deuteronomy","ch":{"bsb":34,"web":34,"webc":34}},{"name":"Joshua","slug":"joshua","ch":{"bsb":24,"web":24,"webc":24}},{"name":"Judges","slug":"judges","ch":{"bsb":21,"web":21,"webc":21}},{"name":"Ruth","slug":"ruth","ch":{"bsb":4,"web":4,"webc":4}},{"name":"1 Samuel","slug":"1-samuel","ch":{"bsb":31,"web":31,"webc":31}},{"name":"2 Samuel","slug":"2-samuel","ch":{"bsb":24,"web":24,"webc":24}},{"name":"1 Kings","slug":"1-kings","ch":{"bsb":22,"web":22,"webc":22}},{"name":"2 Kings","slug":"2-kings","ch":{"bsb":25,"web":25,"webc":25}},{"name":"1 Chronicles","slug":"1-chronicles","ch":{"bsb":29,"web":29,"webc":29}},{"name":"2 Chronicles","slug":"2-chronicles","ch":{"bsb":36,"web":36,"webc":36}},{"name":"Ezra","slug":"ezra","ch":{"bsb":10,"web":10,"webc":10}},{"name":"Nehemiah","slug":"nehemiah","ch":{"bsb":13,"web":13,"webc":13}},{"name":"Esther","slug":"esther","ch":{"bsb":10,"web":10,"webc":10}},{"name":"Job","slug":"job","ch":{"bsb":42,"web":42,"webc":42}},{"name":"Psalms","slug":"psalms","ch":{"bsb":150,"web":150,"webc":150}},{"name":"Proverbs","slug":"proverbs","ch":{"bsb":31,"web":31,"webc":31}},{"name":"Ecclesiastes","slug":"ecclesiastes","ch":{"bsb":12,"web":12,"webc":12}},{"name":"Song of Solomon","slug":"song-of-solomon","ch":{"bsb":8,"web":8,"webc":8}},{"name":"Isaiah","slug":"isaiah","ch":{"bsb":66,"web":66,"webc":66}},{"name":"Jeremiah","slug":"jeremiah","ch":{"bsb":52,"web":52,"webc":52}},{"name":"Lamentations","slug":"lamentations","ch":{"bsb":5,"web":5,"webc":5}},{"name":"Ezekiel","slug":"ezekiel","ch":{"bsb":48,"web":48,"webc":48}},{"name":"Daniel","slug":"daniel","ch":{"bsb":12,"web":12,"webc":14}},{"name":"Hosea","slug":"hosea","ch":{"bsb":14,"web":14,"webc":14}},{"name":"Joel","slug":"joel","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Amos","slug":"amos","ch":{"bsb":9,"web":9,"webc":9}},{"name":"Obadiah","slug":"obadiah","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Jonah","slug":"jonah","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Micah","slug":"micah","ch":{"bsb":7,"web":7,"webc":7}},{"name":"Nahum","slug":"nahum","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Habakkuk","slug":"habakkuk","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Zephaniah","slug":"zephaniah","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Haggai","slug":"haggai","ch":{"bsb":2,"web":2,"webc":2}},{"name":"Zechariah","slug":"zechariah","ch":{"bsb":14,"web":14,"webc":14}},{"name":"Malachi","slug":"malachi","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Matthew","slug":"matthew","ch":{"bsb":28,"web":28,"webc":28}},{"name":"Mark","slug":"mark","ch":{"bsb":16,"web":16,"webc":16}},{"name":"Luke","slug":"luke","ch":{"bsb":24,"web":24,"webc":24}},{"name":"John","slug":"john","ch":{"bsb":21,"web":21,"webc":21}},{"name":"Acts","slug":"acts","ch":{"bsb":28,"web":28,"webc":28}},{"name":"Romans","slug":"romans","ch":{"bsb":16,"web":16,"webc":16}},{"name":"1 Corinthians","slug":"1-corinthians","ch":{"bsb":16,"web":16,"webc":16}},{"name":"2 Corinthians","slug":"2-corinthians","ch":{"bsb":13,"web":13,"webc":13}},{"name":"Galatians","slug":"galatians","ch":{"bsb":6,"web":6,"webc":6}},{"name":"Ephesians","slug":"ephesians","ch":{"bsb":6,"web":6,"webc":6}},{"name":"Philippians","slug":"philippians","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Colossians","slug":"colossians","ch":{"bsb":4,"web":4,"webc":4}},{"name":"1 Thessalonians","slug":"1-thessalonians","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 Thessalonians","slug":"2-thessalonians","ch":{"bsb":3,"web":3,"webc":3}},{"name":"1 Timothy","slug":"1-timothy","ch":{"bsb":6,"web":6,"webc":6}},{"name":"2 Timothy","slug":"2-timothy","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Titus","slug":"titus","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Philemon","slug":"philemon","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Hebrews","slug":"hebrews","ch":{"bsb":13,"web":13,"webc":13}},{"name":"James","slug":"james","ch":{"bsb":5,"web":5,"webc":5}},{"name":"1 Peter","slug":"1-peter","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 Peter","slug":"2-peter","ch":{"bsb":3,"web":3,"webc":3}},{"name":"1 John","slug":"1-john","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 John","slug":"2-john","ch":{"bsb":1,"web":1,"webc":1}},{"name":"3 John","slug":"3-john","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Jude","slug":"jude","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Revelation","slug":"revelation","ch":{"bsb":22,"web":22,"webc":22}},{"name":"Tobit","slug":"tobit","ch":{"webc":14}},{"name":"Judith","slug":"judith","ch":{"webc":16}},{"name":"1 Maccabees","slug":"1-maccabees","ch":{"webc":16}},{"name":"2 Maccabees","slug":"2-maccabees","ch":{"webc":15}},{"name":"Wisdom","slug":"wisdom","ch":{"webc":19}},{"name":"Sirach","slug":"sirach","ch":{"webc":51}},{"name":"Baruch","slug":"baruch","ch":{"webc":6}}];
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
