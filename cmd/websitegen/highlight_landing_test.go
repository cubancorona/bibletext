package main

// These tests run the code that decides where a link lands. The fragment's
// lines, the highlight band, the highlight's own landing (HIGHLIGHT_RANGE),
// the verse carry and the note's placement (NOTE_PLACE) are taken from the
// shipped reader.js between their markers and run against a small stand-in
// DOM: three paragraphs, verses 1-2, 3-4 and 5. A fresh arrival is a fresh
// closure, as a page load is; a hashchange calls highlightRange and then the
// listeners NOTE_PLACE registered, the page's registration order.
// requestAnimationFrame runs its callback at once, so a scroll the page asks
// for after layout is recorded in order with the others.
//
// The note's payload is not under test (note_vectors_test.go holds the
// decoder): the stand-in decodeNotePayload reads "OK", "NEWER" and anything
// else as damaged.
//
// The runtime is runJS's (switcher_vmap_test.go): node, else JavaScriptCore.

import (
	"strings"
	"testing"
)

// readerJSBetween is the shipped template from the begin marker to the end
// marker, both included.
func readerJSBetween(t *testing.T, begin, end string) string {
	t.Helper()
	i := strings.Index(readerJSTemplate, begin)
	j := strings.Index(readerJSTemplate, end)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("reader.js has lost its %s / %s markers", begin, end)
	}
	return readerJSTemplate[i : j+len(end)]
}

const landingDOM = `
var scrolled = [], listeners = [];
function Txt(t) { this.nodeType = 3; this.textContent = t; this.parentNode = null; }
function El(tag, cls, id) {
  this.nodeType = 1; this.tagName = tag; this.id = id || ''; this.childNodes = []; this.parentNode = null;
  this.attrs = {};
  this._cls = (cls || '').split(/\s+/).filter(Boolean);
  var self = this;
  this.classList = {
    add: function () { for (var i = 0; i < arguments.length; i++) if (self._cls.indexOf(arguments[i]) < 0) self._cls.push(arguments[i]); },
    remove: function () { for (var i = 0; i < arguments.length; i++) { var k = self._cls.indexOf(arguments[i]); if (k >= 0) self._cls.splice(k, 1); } },
    contains: function (c) { return self._cls.indexOf(c) >= 0; }
  };
}
Object.defineProperty(El.prototype, 'className', {
  get: function () { return this._cls.join(' '); },
  set: function (v) { this._cls = String(v).split(/\s+/).filter(Boolean); }
});
Object.defineProperty(El.prototype, 'textContent', {
  get: function () { return this.childNodes.map(function (c) { return c.textContent; }).join(''); },
  set: function (v) { this.childNodes = []; this.appendChild(new Txt(v)); }
});
Object.defineProperty(El.prototype, 'innerHTML', {
  set: function (v) { this.childNodes = []; if (/<span><\/span>/.test(v)) this.appendChild(new El('span')); }
});
function sib(n, d) { var p = n.parentNode; if (!p) return null; return p.childNodes[p.childNodes.indexOf(n) + d] || null; }
[El.prototype, Txt.prototype].forEach(function (proto) {
  Object.defineProperty(proto, 'nextSibling', { get: function () { return sib(this, 1); } });
  Object.defineProperty(proto, 'previousSibling', { get: function () { return sib(this, -1); } });
  Object.defineProperty(proto, 'nextElementSibling', { get: function () {
    var n = sib(this, 1); while (n && n.nodeType !== 1) n = sib(n, 1); return n;
  } });
  proto.remove = function () {
    var p = this.parentNode; if (!p) return;
    p.childNodes.splice(p.childNodes.indexOf(this), 1); this.parentNode = null;
  };
});
El.prototype.appendChild = function (c) { if (c.parentNode) c.remove(); c.parentNode = this; this.childNodes.push(c); return c; };
El.prototype.insertBefore = function (c, ref) {
  if (c.parentNode) c.remove(); c.parentNode = this; this.childNodes.splice(this.childNodes.indexOf(ref), 0, c); return c;
};
El.prototype.replaceChild = function (c, old) {
  var i = this.childNodes.indexOf(old); c.parentNode = this; old.parentNode = null; this.childNodes[i] = c; return old;
};
El.prototype.closest = function (tag) { var n = this; while (n && n.nodeType === 1) { if (n.tagName === tag) return n; n = n.parentNode; } return null; };
El.prototype.setAttribute = function (k, v) { this.attrs[k] = String(v); };
El.prototype.getAttribute = function (k) { return k in this.attrs ? this.attrs[k] : null; };
El.prototype.addEventListener = function (type, f) { this['on' + type] = f; };
El.prototype.querySelector = function (tag) {
  var found = null;
  (function walk(n) { if (found || n.nodeType !== 1) return; if (n.tagName === tag) { found = n; return; } n.childNodes.forEach(walk); })(this);
  return found === this ? null : found;
};
El.prototype.scrollIntoView = function (o) { scrolled.push({ el: this, block: o && o.block }); };

var html = new El('html'), wrap, text, verses;
function build() {
  html.childNodes = [];
  var body = html.appendChild(new El('body'));
  wrap = body.appendChild(new El('div', 'wrap'));
  text = wrap.appendChild(new El('article', 'text'));
  verses = {};
  [[1, 2], [3, 4], [5]].forEach(function (nums) {
    var p = text.appendChild(new El('p'));
    nums.forEach(function (n, i) {
      if (i) p.appendChild(new Txt(' '));
      var v = p.appendChild(new El('span', 'v', 'v' + n));
      v.appendChild(new Txt('verse ' + n + '.'));
      verses['v' + n] = v;
    });
  });
}
function matches(n, sel) {
  if (n === html || n.nodeType !== 1) return false;
  return sel.split('.').filter(Boolean).every(function (c) { return n.classList.contains(c); });
}
function all(sel) {
  var out = [];
  (function walk(n) { if (n.nodeType !== 1) return; if (matches(n, sel)) out.push(n); n.childNodes.forEach(walk); })(html);
  return out;
}
var location = { hash: '', pathname: '/web/probe/1/', search: '' };
var history = { replaceState: function (s, t, url) { var i = url.indexOf('#'); location.hash = i < 0 ? '' : url.slice(i); } };
var window = {
  requestAnimationFrame: function (f) { f(); },
  addEventListener: function (type, f) { if (type === 'hashchange') listeners.push(f); }
};
var requestAnimationFrame = window.requestAnimationFrame;
var document = {
  documentElement: html,
  createElement: function (tag) { return new El(tag); },
  createTextNode: function (t) { return new Txt(t); },
  getElementById: function (id) { return verses[id] || null; },
  querySelectorAll: function (sel) { return all(sel); },
  querySelector: function (sel) {
    if (sel === '.v:target') {
      var el = verses[(location.hash || '').replace(/^#/, '')];
      return el || null;
    }
    return all(sel)[0] || null;
  }
};
var NOTE_MSG_NEWER = 'A newer note.', NOTE_MSG_DAMAGED = 'A damaged note.';
function decodeNotePayload(p) {
  if (p === 'OK') return { outcome: 'ok', text: 'A note.' };
  if (p === 'NEWER') return { outcome: 'newer', text: '' };
  return { outcome: 'damaged', text: '' };
}
`

// landingPage is the stand-in DOM and the shipped code, wrapped so that each
// call of load() is a fresh page, and the helpers the scenarios use.
func landingPage(t *testing.T) string {
	code := readerJSBetween(t, "/*__FRAGMENT_BEGIN__*/", "/*__FRAGMENT_END__*/") + "\n" +
		readerJSBetween(t, "/*__HIGHLIGHT_BAND_BEGIN__*/", "/*__HIGHLIGHT_BAND_END__*/") + "\n" +
		readerJSBetween(t, "/*__HIGHLIGHT_RANGE_BEGIN__*/", "/*__HIGHLIGHT_RANGE_END__*/") + "\n" +
		readerJSBetween(t, "/*__VERSE_CARRY_BEGIN__*/", "/*__VERSE_CARRY_END__*/") + "\n" +
		"var activeNoteText = '';\nhighlightRange();\n" +
		readerJSBetween(t, "/*__NOTE_PLACE_BEGIN__*/", "/*__NOTE_PLACE_END__*/")
	code = strings.NewReplacer(
		"__NOTE_BYLINE__", `'Note from the sender'`,
		"__NOTE_PILL_LABEL__", `'Note'`,
	).Replace(code)
	return landingDOM + `
var page = null;
function load(hash) {
  build(); listeners = []; scrolled = []; location.hash = hash;
  page = (function () {
` + code + `
    return { highlightRange: highlightRange, minimizeNote: minimizeNote, activeNote: function () { return activeNoteText; } };
  })();
}
function hashchange(hash) {
  location.hash = hash; scrolled = [];
  page.highlightRange();
  listeners.forEach(function (f) { f(); });
}
function label(el) {
  if (!el) return 'nothing';
  if (el.tagName === 'p') return 'the paragraph of v' + el.childNodes.filter(function (c) { return c.nodeType === 1; })[0].id.slice(1);
  if (el === text) return 'the chapter top';
  if (el.classList && el.classList.contains('v')) return el.id;
  return el.className || el.tagName;
}
// Where the note, its marker or the notice stands, whether it has a tail,
// and what the page scrolled to last.
function report() {
  var card = all('note')[0] || all('notechip')[0] || all('notenotice')[0] || null;
  var last = scrolled.length ? scrolled[scrolled.length - 1].el : null;
  return {
    card: card ? card.className.split(' ')[0] : 'none',
    before: card ? label(card.nextElementSibling) : 'none',
    tail: card ? !card.classList.contains('notail') : false,
    landed: last === card && card ? 'the ' + card.className.split(' ')[0] : label(last)
  };
}
var out = {};
`
}

// A link to one verse that carries a note the page cannot read, damaged or
// in a newer format, lands on the notice it places before the verse's
// paragraph, as a readable note lands on the note; a link to one verse with
// any other key beside it lands on the verse. The fragment matches no
// element's id, so the browser does not scroll, and the page used to scroll
// only for a range: such a link opened at the top of the chapter with the
// verse and the notice off-screen (BSB John 11:35 with a damaged note, the
// verse 3,954 px down). A range with such a note, which the page scrolled to
// its verse, now lands on the notice too. Controls: a readable note lands on
// the note, and a bare "#v3" is left to the browser's own scroll.
func TestTheWebPageLandsOnAVerseWhoseLinkCarriesAKey(t *testing.T) {
	var got map[string]struct {
		Card, Before, Landed string
		Tail                 bool
	}
	runJS(t, landingPage(t)+`
['#v3&n=DAMAGED', '#v3&n=NEWER', '#v3&x=1', '#v3&n=OK', '#v3', '#v3-4&n=DAMAGED'].forEach(function (h) {
  load(h); out[h] = report();
});
`, &got)
	for hash, want := range map[string]string{
		"#v3&n=DAMAGED":   "the notenotice",
		"#v3&n=NEWER":     "the notenotice",
		"#v3&x=1":         "v3",
		"#v3&n=OK":        "the note",
		"#v3":             "nothing",
		"#v3-4&n=DAMAGED": "the notenotice",
	} {
		g, ok := got[hash]
		if !ok {
			t.Errorf("%s: the page reported nothing", hash)
			continue
		}
		if g.Landed != want {
			t.Errorf("%s: the page landed on %s, want %s (%+v)", hash, g.Landed, want, g)
		}
		if g.Card != "none" && (g.Before != "the paragraph of v3" || !g.Tail) {
			t.Errorf("%s: the %s stands before %s (tail %v), want before the paragraph of v3 with a tail", hash, g.Card, g.Before, g.Tail)
		}
	}
}

// A note link whose fragment lights no verse parks its note at the top of the
// chapter with no tail, whatever the page showed before: a hashchange to
// "#n=..." or to a verse the page lacks used to put the note, with a tail,
// before the paragraph the previous link had lit, and scroll there, while a
// reload of the same address parked it at the top. Controls: a fresh load of
// each such address, a hashchange to a verse the page has, and the marker,
// which keeps the note's own place when minimizing clears the highlight,
// also after the reader cleared it and so took the verse out of the address,
// and puts the note back there when tapped.
func TestTheWebNoteParksAtTheTopWhenItsFragmentLightsNoVerse(t *testing.T) {
	type place struct {
		Card, Before, Landed string
		Tail                 bool
	}
	var got map[string]place
	runJS(t, landingPage(t)+`
load('#v3&n=OK');
out['load #v3&n=OK'] = report();
hashchange('#n=OK'); out['then #n=OK'] = report();
hashchange('#v999&n=OK'); out['then #v999&n=OK'] = report();
hashchange('#v5&n=OK'); out['then #v5&n=OK'] = report();
hashchange('#n=OK'); out['then #n=OK after v5'] = report();
load('#v3&n=DAMAGED');
hashchange('#n=DAMAGED'); out['notice then #n=DAMAGED'] = report();
load('#n=OK'); out['load #n=OK'] = report();
load('#v999&n=OK'); out['load #v999&n=OK'] = report();
load('#v3&n=OK');
page.minimizeNote(page.activeNote()); out['minimized'] = report();
load('#v3&n=OK');
location.hash = '#n=OK'; page.highlightRange();
page.minimizeNote(page.activeNote()); out['cleared, then minimized'] = report();
all('notechip')[0].onclick({ preventDefault: function () {} });
out['cleared, minimized, marker tapped'] = report();
`, &got)
	top := place{Before: "the chapter top", Tail: false}
	at := func(n string) place { return place{Before: "the paragraph of " + n, Tail: true} }
	for name, want := range map[string]struct {
		card string
		p    place
	}{
		"load #v3&n=OK":                     {"note", at("v3")},
		"then #n=OK":                        {"note", top},
		"then #v999&n=OK":                   {"note", top},
		"then #v5&n=OK":                     {"note", at("v5")},
		"then #n=OK after v5":               {"note", top},
		"notice then #n=DAMAGED":            {"notenotice", top},
		"load #n=OK":                        {"note", top},
		"load #v999&n=OK":                   {"note", top},
		"minimized":                         {"notechip", at("v3")},
		"cleared, then minimized":           {"notechip", at("v3")},
		"cleared, minimized, marker tapped": {"note", at("v3")},
	} {
		g, ok := got[name]
		if !ok {
			t.Errorf("%s: the page reported nothing", name)
			continue
		}
		if g.Card != want.card || g.Before != want.p.Before || g.Tail != want.p.Tail {
			t.Errorf("%s: the %s stands before %s (tail %v), want the %s before %s (tail %v)",
				name, g.Card, g.Before, g.Tail, want.card, want.p.Before, want.p.Tail)
		}
	}
}
