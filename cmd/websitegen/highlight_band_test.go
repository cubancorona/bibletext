package main

// THE HIGHLIGHT BAND, RUN. The lines between reader.js's HIGHLIGHT_BAND markers
// light a verse range the way the app panes wash one: the lit verses, the
// spaces between them, and an omitted verse's mark (verse_gaps.go) where the
// fragment names that verse, and only there (gapOf, tint.go). They are
// run here — the shipped bytes, against a small
// stand-in DOM — because what they do to the page is the order of two passes
// over its nodes, and a check that the source names the right classes passed
// while a range across a mark left the mark unlit and a minimized note left the
// spaces and marks lit on their own.
//
// The runtime, as note_vectors_test.go chooses it: node when it is on PATH,
// otherwise JavaScriptCore through osascript (every macOS). With neither, the
// test says so and skips.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	bandBegin = "/*__HIGHLIGHT_BAND_BEGIN__*/"
	bandEnd   = "/*__HIGHLIGHT_BAND_END__*/"
)

func extractHighlightBandJS(t *testing.T) string {
	t.Helper()
	i := strings.Index(readerJSTemplate, bandBegin)
	j := strings.Index(readerJSTemplate, bandEnd)
	if i < 0 || j < 0 || j < i {
		t.Fatal("reader.js has lost its HIGHLIGHT_BAND markers")
	}
	return readerJSTemplate[i : j+len(bandEnd)]
}

// bandHarness is the stand-in DOM, the shipped span, and the scenarios. It
// evaluates to "PASS <n>" or "FAIL: ..." as its last expression.
const bandDOM = `
function Txt(t) { this.nodeType = 3; this.textContent = t; this.parentNode = null; }
function El(tag, cls, id) {
  this.nodeType = 1; this.tagName = tag; this.id = id || ''; this.childNodes = []; this.parentNode = null;
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
function sib(n, d) { var p = n.parentNode; if (!p) return null; return p.childNodes[p.childNodes.indexOf(n) + d] || null; }
[El.prototype, Txt.prototype].forEach(function (proto) {
  Object.defineProperty(proto, 'nextSibling', { get: function () { return sib(this, 1); } });
  Object.defineProperty(proto, 'previousSibling', { get: function () { return sib(this, -1); } });
});
El.prototype.appendChild = function (c) { c.parentNode = this; this.childNodes.push(c); return c; };
El.prototype.replaceChild = function (nw, old) {
  var i = this.childNodes.indexOf(old); nw.parentNode = this; old.parentNode = null; this.childNodes[i] = nw; return old;
};
var body = new El('body');
var document = {
  createElement: function (tag) { return new El(tag); },
  createTextNode: function (t) { return new Txt(t); },
  querySelectorAll: function (sel) {
    var want = sel.split('.').filter(Boolean), out = [];
    (function walk(n) {
      if (n.nodeType !== 1) return;
      if (n !== body && want.every(function (c) { return n.classList.contains(c); })) out.push(n);
      n.childNodes.forEach(walk);
    })(body);
    return out;
  }
};
`

const bandScenarios = `
function para(tokens) {
  body.childNodes = [];
  var p = body.appendChild(new El('p'));
  tokens.forEach(function (tok) {
    if (tok[0] === 'v') { var e = new El('span', 'v', tok); e.appendChild(new Txt('verse ' + tok.slice(1) + '.')); p.appendChild(e); }
    else if (tok[0] === '[') { var m = new El('span', 'vg'); m.appendChild(new Txt(tok)); p.appendChild(m); }
    else p.appendChild(new Txt(tok));
  });
  return p;
}
function byId(p, id) { return p.childNodes.filter(function (c) { return c.id === id; })[0]; }
// lightSpan lights a fragment's span lo..hi as lightFragment does: the verses
// the page has, the marks it names (lightMarksIn), then the band.
function lightSpan(p, ids, lo, hi) {
  ids.forEach(function (id) { byId(p, id).classList.add('hl'); });
  lightMarksIn(lo, hi);
  bridgeHighlightGaps();
}
function show(p) {
  return p.childNodes.map(function (c) {
    return c.nodeType === 3 ? JSON.stringify(c.textContent) : c.className.replace(/ /g, '.') + (c.id ? '#' + c.id : '');
  }).join(' ');
}
var bad = [], count = 0;
function expect(name, got, want) { count++; if (got !== want) bad.push(name + ':\n   got  ' + got + '\n   want ' + want); }

var hole = ['v20', ' ', '[21]', ' ', 'v22', ' ', 'v23'];
var p = para(hole), text = p.textContent, plain = show(p);

lightSpan(p, ['v20', 'v22'], 20, 22);
expect('a range across the hole lights its mark', show(p),
  'v.hl#v20 hl.hlgap vg.hlmark hl.hlgap v.hl#v22 " " v#v23');
unlightVerses();
expect('taking the band down leaves nothing lit', show(p), plain);
expect('taking the band down leaves the text as it was', p.textContent, text);

// A mark stands for its omitted verse alone: a verse lit beside it, and
// not named, leaves it bare.
lightSpan(p, ['v22'], 22, 22);
expect('the verse after the hole leaves its mark bare', show(p),
  'v#v20 " " vg " " v.hl#v22 " " v#v23');
unlightVerses();
lightSpan(p, ['v22', 'v23'], 22, 23);
expect('a range from the verse after the hole leaves its mark bare', show(p),
  'v#v20 " " vg " " v.hl#v22 hl.hlgap v.hl#v23');
unlightVerses();
lightSpan(p, ['v20'], 20, 20);
expect('the verse before the hole leaves its mark bare', show(p),
  'v.hl#v20 " " vg " " v#v22 " " v#v23');
unlightVerses();
lightSpan(p, ['v23'], 23, 23);
expect('a verse after the hole leaves its mark alone', show(p),
  'v#v20 " " vg " " v#v22 " " v.hl#v23');
unlightVerses();
expect('re-lighting leaves nothing stale', show(p), plain);

p = para(['v20', ' ', '[21]', ' ', '[22]', ' ', 'v23']);
lightSpan(p, ['v23'], 23, 23);
expect('two marks before a lit verse stay bare', show(p),
  'v#v20 " " vg " " vg " " v.hl#v23');
unlightVerses();
lightSpan(p, ['v20', 'v23'], 20, 23);
expect('a range across two holes lights both marks', show(p),
  'v.hl#v20 hl.hlgap vg.hlmark hl.hlgap vg.hlmark hl.hlgap v.hl#v23');

// The WEB Catholic Edition's Sirach 26 omits verses 19 to 27 and draws their
// nine marks before verse 28. A link to 26:28 lights the verse alone.
var nine = ['v18', ' '], bare = ['v#v18', '" "'], across = ['v.hl#v18'];
for (var k = 19; k <= 27; k++) {
  nine.push('[' + k + ']', ' ');
  bare.push('vg', '" "');
  across.push('hl.hlgap', 'vg.hlmark');
}
nine.push('v28', ' ', 'v29');
bare.push('v.hl#v28', '" "', 'v#v29');
across.push('hl.hlgap', 'v.hl#v28', '" "', 'v#v29');
p = para(nine);
lightSpan(p, ['v28'], 28, 28);
expect('the verse after nine holes leaves their marks bare', show(p), bare.join(' '));
unlightVerses();
lightSpan(p, ['v18', 'v28'], 18, 28);
expect('a range across nine holes lights every mark', show(p), across.join(' '));

// The WEB omits Acts 8:37: a link to 37 lights its mark, one to 38 does not.
p = para(['v36', ' ', '[37]', ' ', 'v38']);
lightSpan(p, [], 37, 37);
expect('a link to Acts 8:37 lights its mark', show(p), 'v#v36 " " vg.hlmark " " v#v38');
unlightVerses();
lightSpan(p, ['v38'], 38, 38);
expect('a link to Acts 8:38 leaves the mark bare', show(p), 'v#v36 " " vg " " v.hl#v38');

// A fragment that names an omitted verse lights its mark (lightMarksIn), as
// lightFragment calls it, before the band is bridged.
p = para(hole); plain = show(p);
lightSpan(p, [], 21, 21);
expect('a fragment naming the omitted verse lights its mark alone', show(p),
  'v#v20 " " vg.hlmark " " v#v22 " " v#v23');
unlightVerses();
expect('taking the mark down leaves nothing lit', show(p), plain);
lightSpan(p, ['v20'], 20, 21);
expect('a range ending at the hole runs through the space to its mark', show(p),
  'v.hl#v20 hl.hlgap vg.hlmark " " v#v22 " " v#v23');
unlightVerses();
lightSpan(p, ['v22'], 21, 22);
expect('a range from the hole lights its mark and the space after it', show(p),
  'v#v20 " " vg.hlmark hl.hlgap v.hl#v22 " " v#v23');
unlightVerses();
lightSpan(p, ['v23'], 23, 23);
expect('a verse after the hole still leaves its mark alone', show(p),
  'v#v20 " " vg " " v#v22 " " v.hl#v23');
unlightVerses();
expect('re-lighting the mark leaves nothing stale', show(p), plain);

p = para(['v20', ' ', '[21]', ' ', '[22]', ' ', 'v23']);
lightSpan(p, [], 21, 21);
expect('one of two holes named lights its mark alone', show(p),
  'v#v20 " " vg.hlmark " " vg " " v#v23');
unlightVerses();
lightSpan(p, [], 21, 22);
expect('two holes named light both marks and the space between', show(p),
  'v#v20 " " vg.hlmark hl.hlgap vg.hlmark " " v#v23');

bad.length ? ('FAIL(' + count + '):\n' + bad.join('\n')) : ('PASS ' + count);
`

func TestTheHighlightBandLightsTheHolesItCrosses(t *testing.T) {
	body := bandDOM + "\n" + extractHighlightBandJS(t) + "\n"
	dir := t.TempDir()
	var out []byte
	var err error
	if node, lookErr := exec.LookPath("node"); lookErr == nil {
		script := filepath.Join(dir, "band.js")
		if werr := os.WriteFile(script, []byte(body+"console.log((function(){ return eval("+jsQuote(bandScenarios)+"); })());\n"), 0o644); werr != nil {
			t.Fatal(werr)
		}
		t.Logf("running the shipped band code under node (%s)", node)
		out, err = exec.Command(node, script).CombinedOutput()
	} else if osa, lookErr := exec.LookPath("osascript"); lookErr == nil {
		script := filepath.Join(dir, "band.js")
		if werr := os.WriteFile(script, []byte(body+bandScenarios), 0o644); werr != nil {
			t.Fatal(werr)
		}
		t.Log("node not on PATH: running the shipped band code under JavaScriptCore (osascript)")
		out, err = exec.Command(osa, "-l", "JavaScript", script).CombinedOutput()
	} else {
		t.Skip("no JavaScript runtime on PATH (node or osascript); the band code was not run")
	}
	got := strings.TrimSpace(string(out))
	if err != nil || !strings.HasPrefix(got, "PASS ") {
		t.Fatalf("the highlight band:\n%s (%v)", got, err)
	}
	t.Log(got)
}

// jsQuote writes s as a JavaScript string literal.
func jsQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "`", "\\`", "$", "\\$")
	return "`" + r.Replace(s) + "`"
}

// Every path that unlights a verse takes the band down whole, through
// unlightVerses; a tap on any painted part of the band is a tap on the
// highlight, and the pointer says so.
func TestEveryUnlightTakesTheBandDown(t *testing.T) {
	band := extractHighlightBandJS(t)
	if n := strings.Count(readerJSTemplate, "classList.remove('hl')"); n != 1 || !strings.Contains(band, "classList.remove('hl')") {
		t.Errorf("verses are unlit in %d places, not once inside unlightVerses; a path that unlights the verses alone leaves the spaces and marks lit", n)
	}
	for _, caller := range []string{"function highlightRange() {\n    unlightVerses();", "unlightVerses(); // the bridged spaces and marks go too", "classList.add('nohl');\n      unlightVerses();"} {
		if !strings.Contains(readerJSTemplate, caller) {
			t.Errorf("reader.js no longer takes the band down through unlightVerses at %q", caller)
		}
	}
	if !strings.Contains(readerJSTemplate, "t.closest('.hlgap') || t.closest('.vg.hlmark')") {
		t.Error("a tap on a lit space or mark does not reach the highlight's bubble")
	}
	if !strings.Contains(testCSS(), ".v:target,.v.hl,.hlgap,.vg.hlmark{cursor:pointer;") {
		t.Error("the lit spaces and marks do not take the highlight's pointer")
	}
}
