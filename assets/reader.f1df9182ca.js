
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
  function highlightRange() {
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

  function showBubble() {
    hideBubble();
    // A real <button>: natively tappable everywhere, and reachable by keyboard.
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'clearbub';
    b.textContent = 'Clear highlight';
    document.body.appendChild(b);
    positionBubble(b);
    bubble = b;
  }

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
    if (t.closest('.clearbub')) { e.preventDefault(); clearHighlight(); return; }
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
  var BOOKS = [{"name":"Acts","slug":"acts","l":"A","ch":{"bsb":28,"web":28,"webc":28}},{"name":"Amos","slug":"amos","l":"A","ch":{"bsb":9,"web":9,"webc":9}},{"name":"Baruch","slug":"baruch","l":"B","ch":{"webc":6}},{"name":"1 Chronicles","slug":"1-chronicles","l":"C","ch":{"bsb":29,"web":29,"webc":29}},{"name":"2 Chronicles","slug":"2-chronicles","l":"C","ch":{"bsb":36,"web":36,"webc":36}},{"name":"Colossians","slug":"colossians","l":"C","ch":{"bsb":4,"web":4,"webc":4}},{"name":"1 Corinthians","slug":"1-corinthians","l":"C","ch":{"bsb":16,"web":16,"webc":16}},{"name":"2 Corinthians","slug":"2-corinthians","l":"C","ch":{"bsb":13,"web":13,"webc":13}},{"name":"Daniel","slug":"daniel","l":"D","ch":{"bsb":12,"web":12,"webc":14}},{"name":"Deuteronomy","slug":"deuteronomy","l":"D","ch":{"bsb":34,"web":34,"webc":34}},{"name":"Ecclesiastes","slug":"ecclesiastes","l":"E","ch":{"bsb":12,"web":12,"webc":12}},{"name":"Ephesians","slug":"ephesians","l":"E","ch":{"bsb":6,"web":6,"webc":6}},{"name":"Esther","slug":"esther","l":"E","ch":{"bsb":10,"web":10,"webc":10}},{"name":"Exodus","slug":"exodus","l":"E","ch":{"bsb":40,"web":40,"webc":40}},{"name":"Ezekiel","slug":"ezekiel","l":"E","ch":{"bsb":48,"web":48,"webc":48}},{"name":"Ezra","slug":"ezra","l":"E","ch":{"bsb":10,"web":10,"webc":10}},{"name":"Galatians","slug":"galatians","l":"G","ch":{"bsb":6,"web":6,"webc":6}},{"name":"Genesis","slug":"genesis","l":"G","ch":{"bsb":50,"web":50,"webc":50}},{"name":"Habakkuk","slug":"habakkuk","l":"H","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Haggai","slug":"haggai","l":"H","ch":{"bsb":2,"web":2,"webc":2}},{"name":"Hebrews","slug":"hebrews","l":"H","ch":{"bsb":13,"web":13,"webc":13}},{"name":"Hosea","slug":"hosea","l":"H","ch":{"bsb":14,"web":14,"webc":14}},{"name":"Isaiah","slug":"isaiah","l":"I","ch":{"bsb":66,"web":66,"webc":66}},{"name":"James","slug":"james","l":"J","ch":{"bsb":5,"web":5,"webc":5}},{"name":"Jeremiah","slug":"jeremiah","l":"J","ch":{"bsb":52,"web":52,"webc":52}},{"name":"Job","slug":"job","l":"J","ch":{"bsb":42,"web":42,"webc":42}},{"name":"Joel","slug":"joel","l":"J","ch":{"bsb":3,"web":3,"webc":3}},{"name":"John","slug":"john","l":"J","ch":{"bsb":21,"web":21,"webc":21}},{"name":"1 John","slug":"1-john","l":"J","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 John","slug":"2-john","l":"J","ch":{"bsb":1,"web":1,"webc":1}},{"name":"3 John","slug":"3-john","l":"J","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Jonah","slug":"jonah","l":"J","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Joshua","slug":"joshua","l":"J","ch":{"bsb":24,"web":24,"webc":24}},{"name":"Jude","slug":"jude","l":"J","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Judges","slug":"judges","l":"J","ch":{"bsb":21,"web":21,"webc":21}},{"name":"Judith","slug":"judith","l":"J","ch":{"webc":16}},{"name":"1 Kings","slug":"1-kings","l":"K","ch":{"bsb":22,"web":22,"webc":22}},{"name":"2 Kings","slug":"2-kings","l":"K","ch":{"bsb":25,"web":25,"webc":25}},{"name":"Lamentations","slug":"lamentations","l":"L","ch":{"bsb":5,"web":5,"webc":5}},{"name":"Leviticus","slug":"leviticus","l":"L","ch":{"bsb":27,"web":27,"webc":27}},{"name":"Luke","slug":"luke","l":"L","ch":{"bsb":24,"web":24,"webc":24}},{"name":"1 Maccabees","slug":"1-maccabees","l":"M","ch":{"webc":16}},{"name":"2 Maccabees","slug":"2-maccabees","l":"M","ch":{"webc":15}},{"name":"Malachi","slug":"malachi","l":"M","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Mark","slug":"mark","l":"M","ch":{"bsb":16,"web":16,"webc":16}},{"name":"Matthew","slug":"matthew","l":"M","ch":{"bsb":28,"web":28,"webc":28}},{"name":"Micah","slug":"micah","l":"M","ch":{"bsb":7,"web":7,"webc":7}},{"name":"Nahum","slug":"nahum","l":"N","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Nehemiah","slug":"nehemiah","l":"N","ch":{"bsb":13,"web":13,"webc":13}},{"name":"Numbers","slug":"numbers","l":"N","ch":{"bsb":36,"web":36,"webc":36}},{"name":"Obadiah","slug":"obadiah","l":"O","ch":{"bsb":1,"web":1,"webc":1}},{"name":"1 Peter","slug":"1-peter","l":"P","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 Peter","slug":"2-peter","l":"P","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Philemon","slug":"philemon","l":"P","ch":{"bsb":1,"web":1,"webc":1}},{"name":"Philippians","slug":"philippians","l":"P","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Proverbs","slug":"proverbs","l":"P","ch":{"bsb":31,"web":31,"webc":31}},{"name":"Psalms","slug":"psalms","l":"P","ch":{"bsb":150,"web":150,"webc":150}},{"name":"Revelation","slug":"revelation","l":"R","ch":{"bsb":22,"web":22,"webc":22}},{"name":"Romans","slug":"romans","l":"R","ch":{"bsb":16,"web":16,"webc":16}},{"name":"Ruth","slug":"ruth","l":"R","ch":{"bsb":4,"web":4,"webc":4}},{"name":"1 Samuel","slug":"1-samuel","l":"S","ch":{"bsb":31,"web":31,"webc":31}},{"name":"2 Samuel","slug":"2-samuel","l":"S","ch":{"bsb":24,"web":24,"webc":24}},{"name":"Sirach","slug":"sirach","l":"S","ch":{"webc":51}},{"name":"Song of Solomon","slug":"song-of-solomon","l":"S","ch":{"bsb":8,"web":8,"webc":8}},{"name":"1 Thessalonians","slug":"1-thessalonians","l":"T","ch":{"bsb":5,"web":5,"webc":5}},{"name":"2 Thessalonians","slug":"2-thessalonians","l":"T","ch":{"bsb":3,"web":3,"webc":3}},{"name":"1 Timothy","slug":"1-timothy","l":"T","ch":{"bsb":6,"web":6,"webc":6}},{"name":"2 Timothy","slug":"2-timothy","l":"T","ch":{"bsb":4,"web":4,"webc":4}},{"name":"Titus","slug":"titus","l":"T","ch":{"bsb":3,"web":3,"webc":3}},{"name":"Tobit","slug":"tobit","l":"T","ch":{"webc":14}},{"name":"Wisdom","slug":"wisdom","l":"W","ch":{"webc":19}},{"name":"Zechariah","slug":"zechariah","l":"Z","ch":{"bsb":14,"web":14,"webc":14}},{"name":"Zephaniah","slug":"zephaniah","l":"Z","ch":{"bsb":3,"web":3,"webc":3}}];
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

  // 1b) THE SENDER'S NOTE. The link may carry one in the "n" key; it is the
  // same payload the app writes (share_note.go): a format byte, then UTF-8,
  // raw or raw-DEFLATE'd, then unpadded base64url.
  //
  // It is UNTRUSTED TEXT — anyone can write a link. It is inserted with
  // textContent and never as markup, no part of it is ever made a live link,
  // and the bubble says whose it is, because a note styled as though BibleText
  // said it would be a phishing kit on our own domain.
  function decodeNote(payload) {
    if (!payload) return Promise.resolve('');
    var bytes;
    try {
      var b64 = payload.replace(/-/g, '+').replace(/_/g, '/');
      while (b64.length % 4) b64 += '=';
      var bin = atob(b64);
      bytes = new Uint8Array(bin.length);
      for (var i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    } catch (e) { return Promise.resolve(''); }
    if (bytes.length < 2) return Promise.resolve('');

    var tag = bytes[0], body = bytes.subarray(1);
    var utf8 = function (b) { return new TextDecoder('utf-8', { fatal: false }).decode(b); };
    if (tag === 112 /* p */) return Promise.resolve(cleanNote(utf8(body)));
    if (tag !== 122 /* z */) return Promise.resolve('');   // a format we do not know
    if (typeof DecompressionStream !== 'function') return Promise.resolve('');
    try {
      var ds = new DecompressionStream('deflate-raw');
      var stream = new Blob([body]).stream().pipeThrough(ds);
      return new Response(stream).arrayBuffer()
        .then(function (buf) { return cleanNote(utf8(new Uint8Array(buf))); })
        .catch(function () { return ''; });
    } catch (e) { return Promise.resolve(''); }
  }

  // The browser half of normalizeNote (share_note.go): strip the control
  // characters and bidi overrides that let text hide or reverse itself, keep
  // the newlines that make a note readable, and cap the length.
  function cleanNote(s) {
    if (!s) return '';
    s = s.replace(/\r\n?/g, '\n')
         // C0 and C1 controls, keeping tab (09) and newline (0a).
         .replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f-\u009f]/g, '')
         // Bidi marks, embeddings, isolates, and the BOM.
         .replace(/[\u200e\u200f\u202a-\u202e\u2066-\u2069\ufeff]/g, '')
         .replace(/\n{3,}/g, '\n\n')
         .trim();
    var runes = Array.from(s);
    if (runes.length > 280) s = runes.slice(0, 280).join('').trim();
    return s;
  }

  var noteBox = null, noteChip = null;

  function showNote(text) {
    hideNote();
    var wrap = document.querySelector('.wrap');
    if (!wrap || !text) return;

    var box = document.createElement('aside');
    box.className = 'note';
    var who = document.createElement('p');
    who.className = 'notewho';
    who.textContent = 'Note from Friend';   // a person, never "from BibleText"
    var body = document.createElement('p');
    body.className = 'notetext';
    body.textContent = text;                             // TEXT, never markup
    var x = document.createElement('button');
    x.type = 'button';
    x.className = 'notex';
    x.setAttribute('aria-label', 'Dismiss note');
    x.textContent = '×';
    x.addEventListener('click', function (e) { e.preventDefault(); hideNote(); showChip(text); });
    box.appendChild(x); box.appendChild(who); box.appendChild(body);
    anchorToPassage(box);
    noteBox = box;
    rescrollToHighlight();
  }

  // Dismissing collapses the note to a small marker rather than destroying it —
  // an accidental tap should not lose someone's message.
  function showChip(text) {
    hideNote();
    var chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'notechip';
    chip.textContent = 'Show note';
    chip.addEventListener('click', function (e) { e.preventDefault(); showNote(text); });
    anchorToPassage(chip);
    noteChip = chip;
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
  function anchorToPassage(el) {
    var lit = document.querySelector('.v.hl') || document.querySelector('.v:target');
    var para = lit && lit.closest ? lit.closest('p') : null;
    if (para && para.parentNode) { para.parentNode.insertBefore(el, para); return; }
    var text = document.querySelector('.text');
    if (text && text.parentNode) text.parentNode.insertBefore(el, text);
    else document.querySelector('.wrap').appendChild(el);
  }

  // Inserting the note pushes the passage down, so whatever scroll brought the
  // reader to their verse is now pointing at the wrong place. Put it back.
  function rescrollToHighlight() {
    var lit = document.querySelector('.v.hl') || document.querySelector('.v:target');
    if (lit) lit.scrollIntoView({ block: 'center' });
  }

  function renderNote() {
    var payload = fragKeys().n;
    if (!payload) { hideNote(); return; }
    decodeNote(payload).then(function (text) { if (text) showNote(text); });
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
