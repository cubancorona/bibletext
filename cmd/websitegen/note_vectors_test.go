package main

// The web reader's half of the shared-note conformance corpus.
//
// testdata/note_vectors.txt (at the repo root) is APPEND-ONLY and is walked by
// BOTH decoders — the app's (share_note.go, tested beside it) and reader.js's
// decodeNotePayload, tested here — because a rule that lives only in one
// implementation's behaviour is the rule the second implementation silently
// breaks. The two had ALREADY diverged once, over invalid UTF-8: the page's
// TextDecoder({fatal:false}) rendered U+FFFD where the app showed nothing
// (docs/NOTE_WIRE_FORMAT.md, "Conformance corpus").
//
// HOW TestNoteVectorCorpusAgainstReaderJS EXECUTES THE JS, in order of
// preference; the test log says which ran:
//
//  1. node, when it is on PATH: the decoder span is extracted from
//     readerJSTemplate between its two markers — the span is pure by contract,
//     no DOM — and node runs the REAL shipped code over every vector.
//  2. osascript -l JavaScript (JavaScriptCore, present on every macOS): the
//     SAME shipped span, with test-only polyfills for the two browser
//     primitives JSC lacks (atob, TextDecoder). The inflate, the record walk
//     and the outcome logic are the real bytes; only base64 and UTF-8
//     primitives are stand-ins.
//  3. jsDecodeNotePayload below, the last resort on a machine with neither: a
//     deliberately line-by-line Go re-implementation OF THE JAVASCRIPT (not of
//     share_note.go — porting the Go decoder again would only prove Go agrees
//     with itself).
//
// The Go mirror is also tested on its own, on every machine. It used to run
// only where neither runtime existed, and every CI runner has node and every
// Mac has osascript, so nothing ever ran it and it drifted: reader.js came to
// reject incomplete Huffman tables, the standard base64 alphabet and
// wrong-length padding while the mirror went on accepting all three, and the
// only symptom was three corpus failures on a machine with no JS runtime.
// Two tests now keep it in step with the span it stands in for:
//
//   - TestNoteVectorCorpusAgainstGoMirrorOfReaderJS walks the corpus through
//     the mirror whatever runtimes the machine has.
//   - TestGoMirrorOfReaderJSAgreesWithNode, wherever node is on PATH, decodes
//     the corpus, hand-built boundary payloads and seeded mutations of the
//     corpus with both the shipped span and the mirror, and requires the same
//     outcome and text from each. The corpus pins only what it has lines for;
//     the probes also reach the rules it has no line for, and every bound in
//     the span from both sides. Without node it skips, except in CI, where it
//     fails: it is the only guard most of the mirror's rules have, and a
//     runner image that stopped shipping node would otherwise drop it
//     without a sign. It needs node rather than JavaScriptCore because the
//     JavaScriptCore path's atob is a polyfill, and the mirror follows the
//     browser's atob.

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

const (
	noteDecoderBegin = "/*__NOTE_DECODER_BEGIN__*/"
	noteDecoderEnd   = "/*__NOTE_DECODER_END__*/"
)

// extractNoteDecoderJS pulls the pure decoder span out of the shipped
// template.
func extractNoteDecoderJS(t *testing.T) string {
	t.Helper()
	i := strings.Index(readerJSTemplate, noteDecoderBegin)
	j := strings.Index(readerJSTemplate, noteDecoderEnd)
	if i < 0 || j < 0 || j < i {
		t.Fatal("reader.js has lost its NOTE_DECODER markers")
	}
	return readerJSTemplate[i : j+len(noteDecoderEnd)]
}

type noteVector struct {
	line     int
	payload  string
	expected string // ok | newer | damaged
	text     string
}

func loadNoteVectors(t *testing.T) []noteVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "note_vectors.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var out []noteVector
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		v := noteVector{line: i + 1, payload: parts[0], expected: parts[1]}
		if len(parts) == 3 {
			v.text = parts[2]
		}
		out = append(out, v)
	}
	if len(out) < 25 {
		t.Fatalf("only %d vectors — the corpus should never shrink", len(out))
	}
	return out
}

func TestNoteVectorCorpusAgainstReaderJS(t *testing.T) {
	vectors := loadNoteVectors(t)

	if node, err := exec.LookPath("node"); err == nil {
		t.Logf("running the REAL reader.js decoder under node (%s)", node)
		runVectorsUnderNode(t, node, vectors)
		return
	}
	if osa, err := exec.LookPath("osascript"); err == nil {
		t.Log("node not on PATH: running the REAL reader.js decoder under JavaScriptCore (osascript)")
		runVectorsUnderJXA(t, osa)
		return
	}
	t.Log("no JS runtime on PATH: walking the vectors with the Go re-implementation of the JS decoder")
	walkVectorsThroughGoMirror(t, vectors)
}

// TestNoteVectorCorpusAgainstGoMirrorOfReaderJS holds the Go mirror to the
// corpus on every machine, independent of which JS runtime exists, so the
// last-resort path above is known to work before the day it is needed.
func TestNoteVectorCorpusAgainstGoMirrorOfReaderJS(t *testing.T) {
	walkVectorsThroughGoMirror(t, loadNoteVectors(t))
}

func walkVectorsThroughGoMirror(t *testing.T, vectors []noteVector) {
	t.Helper()
	for _, v := range vectors {
		got := jsDecodeNotePayload(v.payload)
		if got.outcome != v.expected {
			t.Errorf("line %d: JS outcome %q, want %q (%s)", v.line, got.outcome, v.expected, v.payload)
			continue
		}
		if v.expected == "ok" && v.text != "" && got.text != v.text {
			t.Errorf("line %d: JS text\n got %q\nwant %q", v.line, got.text, v.text)
		}
		if v.expected != "ok" && got.text != "" {
			t.Errorf("line %d: a %s payload returned text %q", v.line, v.expected, got.text)
		}
	}
}

// runVectorsUnderNode writes the extracted decoder plus a tiny driver and lets
// node judge every vector.
func runVectorsUnderNode(t *testing.T, node string, vectors []noteVector) {
	t.Helper()
	dir := t.TempDir()
	harness := `
'use strict';
` + extractNoteDecoderJS(t) + `
const fs = require('fs');
const lines = fs.readFileSync(process.argv[2], 'utf8').split('\n');
let bad = 0;
for (let i = 0; i < lines.length; i++) {
  const line = lines[i].replace(/\r$/, '');
  if (!line.trim() || line[0] === '#') continue;
  const sp1 = line.indexOf(' ');
  if (sp1 < 0) continue;
  const payload = line.slice(0, sp1);
  const rest = line.slice(sp1 + 1);
  const sp2 = rest.indexOf(' ');
  const expected = sp2 < 0 ? rest : rest.slice(0, sp2);
  const wantText = sp2 < 0 ? null : rest.slice(sp2 + 1);
  const got = decodeNotePayload(payload);
  if (got.outcome !== expected) {
    console.log('line ' + (i + 1) + ': outcome ' + got.outcome + ', want ' + expected);
    bad++;
    continue;
  }
  if (expected === 'ok' && wantText !== null && got.text !== wantText) {
    console.log('line ' + (i + 1) + ': text ' + JSON.stringify(got.text) + ', want ' + JSON.stringify(wantText));
    bad++;
  }
  if (expected !== 'ok' && got.text !== '') {
    console.log('line ' + (i + 1) + ': a ' + expected + ' payload returned text');
    bad++;
  }
}
process.exit(bad ? 1 : 0);
`
	script := filepath.Join(dir, "harness.js")
	if err := os.WriteFile(script, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, script,
		filepath.Join("..", "..", "testdata", "note_vectors.txt")).CombinedOutput()
	if len(out) > 0 {
		t.Logf("node:\n%s", out)
	}
	if err != nil {
		t.Fatalf("reader.js decoder disagrees with the corpus: %v", err)
	}
	_ = vectors // node reads the file itself; the slice guarded its size above
}

// runVectorsUnderJXA executes the same shipped span under JavaScriptCore.
// atob and TextDecoder are polyfilled — they are the browser's, not ours, and
// JSC has neither; everything the corpus actually pins (the inflate, the
// record framing, the outcomes, strict UTF-8 rejection) runs as shipped.
func runVectorsUnderJXA(t *testing.T, osa string) {
	t.Helper()
	vectorsPath, err := filepath.Abs(filepath.Join("..", "..", "testdata", "note_vectors.txt"))
	if err != nil {
		t.Fatal(err)
	}
	harness := jxaPolyfills + "\n" + extractNoteDecoderJS(t) + `
ObjC.import('Foundation');
function run() {
  var raw = ObjC.unwrap($.NSString.stringWithContentsOfFileEncodingError(
    ` + "`" + vectorsPath + "`" + `, $.NSUTF8StringEncoding, null));
  var lines = raw.split('\n');
  var bad = [], count = 0;
  for (var i = 0; i < lines.length; i++) {
    var line = lines[i].replace(/\r$/, '');
    if (!line.trim() || line[0] === '#') continue;
    var sp1 = line.indexOf(' ');
    if (sp1 < 0) continue;
    count++;
    var payload = line.slice(0, sp1);
    var rest = line.slice(sp1 + 1);
    var sp2 = rest.indexOf(' ');
    var expected = sp2 < 0 ? rest : rest.slice(0, sp2);
    var wantText = sp2 < 0 ? null : rest.slice(sp2 + 1);
    var got = decodeNotePayload(payload);
    if (got.outcome !== expected) {
      bad.push('line ' + (i + 1) + ': outcome ' + got.outcome + ' want ' + expected);
      continue;
    }
    if (expected === 'ok' && wantText !== null && got.text !== wantText) {
      bad.push('line ' + (i + 1) + ': text ' + JSON.stringify(got.text) + ' want ' + JSON.stringify(wantText));
    }
    if (expected !== 'ok' && got.text !== '') bad.push('line ' + (i + 1) + ': text on failure');
  }
  return bad.length ? ('FAIL(' + count + '):\n' + bad.join('\n')) : ('PASS ' + count + ' vectors');
}
run();
`
	script := filepath.Join(t.TempDir(), "harness.js")
	if err := os.WriteFile(script, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(osa, "-l", "JavaScript", script).CombinedOutput()
	got := strings.TrimSpace(string(out))
	t.Logf("JavaScriptCore: %s", got)
	if err != nil || !strings.HasPrefix(got, "PASS ") {
		t.Fatalf("reader.js decoder disagrees with the corpus under JavaScriptCore (err=%v)", err)
	}
}

// jxaPolyfills stands in for the two browser primitives JavaScriptCore lacks.
// TEST-ONLY — nothing here ships.
const jxaPolyfills = `
function atob(s) {
  if (!/^[A-Za-z0-9+\/]*={0,2}$/.test(s) || s.length % 4 !== 0) throw new Error('bad b64');
  var chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  var body = s.replace(/=+$/, '');
  var out = '';
  var buf = 0, bits = 0;
  for (var i = 0; i < body.length; i++) {
    var v = chars.indexOf(body[i]);
    if (v < 0) throw new Error('bad char');
    buf = (buf << 6) | v; bits += 6;
    if (bits >= 8) { bits -= 8; out += String.fromCharCode((buf >> bits) & 0xff); }
  }
  return out;
}
function TextDecoder(enc, opts) { this.fatal = !!(opts && opts.fatal); }
TextDecoder.prototype.decode = function (bytes) {
  var out = '', i = 0, n = bytes.length;
  var die = function () { throw new Error('invalid utf-8'); };
  while (i < n) {
    var b0 = bytes[i++];
    var cp, extra, min;
    if (b0 < 0x80) { out += String.fromCharCode(b0); continue; }
    else if ((b0 & 0xe0) === 0xc0) { cp = b0 & 0x1f; extra = 1; min = 0x80; }
    else if ((b0 & 0xf0) === 0xe0) { cp = b0 & 0x0f; extra = 2; min = 0x800; }
    else if ((b0 & 0xf8) === 0xf0) { cp = b0 & 0x07; extra = 3; min = 0x10000; }
    else die();
    if (i + extra > n) die();
    for (var k = 0; k < extra; k++) {
      var bb = bytes[i++];
      if ((bb & 0xc0) !== 0x80) die();
      cp = (cp << 6) | (bb & 0x3f);
    }
    if (cp < min || cp > 0x10ffff || (cp >= 0xd800 && cp <= 0xdfff)) die();
    out += String.fromCodePoint(cp);
  }
  return out;
};
`

// ---------------------------------------------------------------------------
// The Go re-implementation OF THE JAVASCRIPT, function for function and check
// for check, in the span's order. Keep it in lockstep with the span between
// the markers: TestGoMirrorOfReaderJSAgreesWithNode fails when it is not.
//
// Where a browser primitive or a language built-in decides an outcome, the
// port models that primitive, not the nearest Go library call: atob is the
// WHATWG forgiving-base64 decode (jsAtob), String.prototype.trim strips
// ECMAScript's white space and line terminators (jsTrim), and TextDecoder
// drops a leading byte-order mark (jsUTF8). Go's base64.StdEncoding and
// strings.TrimSpace each disagree with the JS on payloads the JS accepts.
//
// Two differences are deliberate and cannot change a result: where the JS
// uses floats the port uses uint64, equivalent over every length a 4 KB
// stream can hold, and the fixed Huffman tables, which the JS builds once and
// caches, are built afresh on each call.
// ---------------------------------------------------------------------------

type jsNoteResult struct {
	outcome string
	text    string
}

var jsDamaged = jsNoteResult{outcome: "damaged"}

var (
	jsLENS = []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51,
		59, 67, 83, 99, 115, 131, 163, 195, 227, 258}
	jsLEXT = []int{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4,
		4, 5, 5, 5, 5, 0}
	jsDISTS = []int{1, 2, 3, 4, 5, 7, 9, 13, 17, 25, 33, 49, 65, 97, 129, 193, 257, 385,
		513, 769, 1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577}
	jsDEXT = []int{0, 0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10,
		10, 11, 11, 12, 12, 13, 13}
	jsCLORDER = []int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}
)

type jsHuff struct {
	count  [16]int
	symbol []int
}

// jsHuffConstruct is noteHuffConstruct: nil for an over-subscribed table and,
// unless allowIncomplete, for an incomplete one other than a single code of
// length 1. Only the fixed tables pass allowIncomplete.
func jsHuffConstruct(lengths []int, n int, allowIncomplete bool) *jsHuff {
	h := &jsHuff{}
	for i := 0; i < n; i++ {
		l := 0
		if i < len(lengths) {
			l = lengths[i]
		}
		h.count[l]++
	}
	left, total := 1, 0
	for l := 1; l <= 15; l++ {
		left <<= 1
		left -= h.count[l]
		if left < 0 {
			return nil
		}
		total += h.count[l]
	}
	if !allowIncomplete && left > 0 && !(total == 1 && h.count[1] == 1) {
		return nil
	}
	var offs [16]int
	offs[1] = 0
	for l := 1; l < 15; l++ {
		offs[l+1] = offs[l] + h.count[l]
	}
	h.symbol = make([]int, n)
	for i := 0; i < n; i++ {
		if i < len(lengths) && lengths[i] != 0 {
			h.symbol[offs[lengths[i]]] = i
			offs[lengths[i]]++
		}
	}
	return h
}

func jsInflateRaw(src []byte, limit int) []byte {
	pos, bitbuf, bitcnt := 0, 0, 0
	var out []byte
	errFlag := false

	bits := func(need int) int {
		val := bitbuf
		for bitcnt < need {
			if pos >= len(src) {
				errFlag = true
				return 0
			}
			val |= int(src[pos]) << bitcnt
			pos++
			bitcnt += 8
		}
		bitbuf = val >> need
		bitcnt -= need
		return val & ((1 << need) - 1)
	}
	decode := func(h *jsHuff) int {
		code, first, index := 0, 0, 0
		for l := 1; l <= 15; l++ {
			code |= bits(1)
			if errFlag {
				return -1
			}
			count := h.count[l]
			if code-first < count {
				return h.symbol[index+(code-first)]
			}
			index += count
			first += count
			first <<= 1
			code <<= 1
		}
		return -1
	}
	push := func(b byte) {
		out = append(out, b)
		if len(out) > limit {
			errFlag = true
		}
	}
	codes := func(lencode, distcode *jsHuff) bool {
		for {
			sym := decode(lencode)
			if sym < 0 || errFlag {
				return false
			}
			switch {
			case sym < 256:
				push(byte(sym))
				if errFlag {
					return false
				}
			case sym == 256:
				return true
			default:
				sym -= 257
				if sym >= 29 {
					return false
				}
				length := jsLENS[sym] + bits(jsLEXT[sym])
				if errFlag {
					return false
				}
				dsym := decode(distcode)
				if dsym < 0 || errFlag {
					return false
				}
				dist := jsDISTS[dsym] + bits(jsDEXT[dsym])
				if errFlag {
					return false
				}
				if dist > len(out) {
					return false
				}
				for ; length > 0; length-- {
					push(out[len(out)-dist])
					if errFlag {
						return false
					}
				}
			}
		}
	}
	stored := func() bool {
		bitbuf, bitcnt = 0, 0
		if pos+4 > len(src) {
			return false
		}
		length := int(src[pos]) | int(src[pos+1])<<8
		nlen := int(src[pos+2]) | int(src[pos+3])<<8
		pos += 4
		if length != (^nlen)&0xffff {
			return false
		}
		if pos+length > len(src) {
			return false
		}
		for i := 0; i < length; i++ {
			push(src[pos+i])
			if errFlag {
				return false
			}
		}
		pos += length
		return true
	}
	dynamicTables := func() (*jsHuff, *jsHuff) {
		nlen := bits(5) + 257
		ndist := bits(5) + 1
		ncode := bits(4) + 4
		if errFlag || nlen > 286 || ndist > 30 {
			return nil, nil
		}
		lengths := make([]int, 19)
		for i := 0; i < ncode; i++ {
			lengths[jsCLORDER[i]] = bits(3)
		}
		if errFlag {
			return nil, nil
		}
		clcode := jsHuffConstruct(lengths, 19, false)
		if clcode == nil {
			return nil, nil
		}
		symlens := make([]int, nlen+ndist)
		index := 0
		for index < nlen+ndist {
			sym := decode(clcode)
			if sym < 0 || errFlag {
				return nil, nil
			}
			if sym < 16 {
				symlens[index] = sym
				index++
				continue
			}
			repLen, count := 0, 0
			switch sym {
			case 16:
				if index == 0 {
					return nil, nil
				}
				repLen = symlens[index-1]
				count = 3 + bits(2)
			case 17:
				count = 3 + bits(3)
			default:
				count = 11 + bits(7)
			}
			if errFlag || index+count > nlen+ndist {
				return nil, nil
			}
			for ; count > 0; count-- {
				symlens[index] = repLen
				index++
			}
		}
		if symlens[256] == 0 {
			return nil, nil
		}
		lc := jsHuffConstruct(symlens[:nlen], nlen, false)
		dc := jsHuffConstruct(symlens[nlen:], ndist, false)
		if lc == nil || dc == nil {
			return nil, nil
		}
		return lc, dc
	}

	var fixedLen, fixedDist *jsHuff
	for {
		last := bits(1)
		typ := bits(2)
		if errFlag {
			return nil
		}
		var blockOK bool
		switch typ {
		case 0:
			blockOK = stored()
		case 1:
			if fixedLen == nil {
				fl := make([]int, 288)
				i := 0
				for ; i < 144; i++ {
					fl[i] = 8
				}
				for ; i < 256; i++ {
					fl[i] = 9
				}
				for ; i < 280; i++ {
					fl[i] = 7
				}
				for ; i < 288; i++ {
					fl[i] = 8
				}
				fd := make([]int, 30)
				for i := range fd {
					fd[i] = 5
				}
				fixedLen = jsHuffConstruct(fl, 288, true)
				fixedDist = jsHuffConstruct(fd, 30, true)
			}
			blockOK = codes(fixedLen, fixedDist)
		case 2:
			lc, dc := dynamicTables()
			if lc == nil {
				blockOK = false
			} else {
				blockOK = codes(lc, dc)
			}
		default:
			blockOK = false
		}
		if !blockOK || errFlag {
			return nil
		}
		if last != 0 {
			break
		}
	}
	if out == nil {
		out = []byte{}
	}
	return out // trailing input bytes are ignored, as in the JS
}

// jsPaddedPayload is the JS's /^[A-Za-z0-9_-]+={1,2}$/.
var jsPaddedPayload = regexp.MustCompile(`^[A-Za-z0-9_-]+={1,2}$`)

// jsBase64Bytes is noteBase64Bytes: the url-safe alphabet only, and '=' only
// as correctly-formed trailing padding; anything else is nil.
func jsBase64Bytes(payload string) []byte {
	if strings.ContainsAny(payload, "+/") {
		return nil
	}
	eq := strings.IndexByte(payload, '=')
	if eq != -1 {
		if !jsPaddedPayload.MatchString(payload) || jsLength(payload)%4 != 0 {
			return nil
		}
	}
	b64 := strings.ReplaceAll(payload, "-", "+")
	b64 = strings.ReplaceAll(b64, "_", "/")
	if eq == -1 {
		for jsLength(b64)%4 != 0 {
			b64 += "="
		}
	}
	out, ok := jsAtob(b64)
	if !ok {
		return nil // atob throws; the JS catches it and returns null
	}
	return out
}

// jsLength is a JS string's length, which counts UTF-16 code units, not
// bytes.
func jsLength(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xffff {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// jsAtob is atob, the WHATWG forgiving-base64 decode: ASCII white space is
// removed wherever it is, one or two '=' come off a length that is a multiple
// of four, a remainder of one or any character outside the standard alphabet
// fails, and the bits after the last whole byte are ignored.
// base64.StdEncoding is stricter: it ignores only CR and LF, and it requires
// the padding atob makes optional.
func jsAtob(data string) ([]byte, bool) {
	data = strings.Map(func(r rune) rune {
		switch r {
		case '\t', '\n', '\f', '\r', ' ':
			return -1
		}
		return r
	}, data)
	if utf8.RuneCountInString(data)%4 == 0 {
		if strings.HasSuffix(data, "==") {
			data = data[:len(data)-2]
		} else if strings.HasSuffix(data, "=") {
			data = data[:len(data)-1]
		}
	}
	if utf8.RuneCountInString(data)%4 == 1 {
		return nil, false
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := []byte{}
	buf, bits := 0, 0
	for _, r := range data {
		v := strings.IndexRune(alphabet, r)
		if v < 0 {
			return nil, false
		}
		buf = buf<<6 | v
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= 1<<bits - 1
		}
	}
	return out, true
}

// jsUTF8 is noteUTF8: TextDecoder('utf-8', {fatal: true}).decode, which
// throws on invalid UTF-8 and, since ignoreBOM is left false, drops one
// leading byte-order mark from the string it returns.
func jsUTF8(b []byte) (string, bool) {
	if !utf8.Valid(b) {
		return "", false // TextDecoder {fatal:true} throws
	}
	return strings.TrimPrefix(string(b), "\ufeff"), true
}

// jsTrim is String.prototype.trim, which strips ECMAScript's WhiteSpace (tab,
// vertical tab, form feed, U+FEFF and the Zs category) and LineTerminator (LF,
// CR, U+2028, U+2029). strings.TrimSpace differs on two characters: it keeps
// U+FEFF and strips U+0085.
func jsTrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029,
			0x202f, 0x205f, 0x3000, 0xfeff:
			return true
		}
		return r >= 0x2000 && r <= 0x200a
	})
}

func jsCleanNote(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r <= 0x08) || r == 0x0b || r == 0x0c || (r >= 0x0e && r <= 0x1f) ||
			(r >= 0x7f && r <= 0x9f):
			// the JS C0/C1 class, keeping tab and newline
		case r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) ||
			(r >= 0x2066 && r <= 0x2069) || r == 0xfeff || r == 0xfffd:
			// the JS bidi/BOM/U+FFFD class
		default:
			b.WriteRune(r)
		}
	}
	s = b.String()
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	s = jsTrim(s)
	runes := []rune(s)
	if len(runes) > 280 {
		s = jsTrim(string(runes[:280]))
	}
	return s
}

func jsUvarint(s []byte, at int) (uint64, int) {
	var x uint64
	var mult uint64 = 1
	for k := 0; k < 10; k++ {
		if at+k >= len(s) {
			return 0, 0
		}
		b := s[at+k]
		if b < 0x80 {
			if k == 9 && b > 1 {
				return 0, -10
			}
			return x + uint64(b)*mult, k + 1
		}
		x += uint64(b&0x7f) * mult
		mult *= 128
	}
	return 0, -10
}

func jsLegacyText(raw []byte) jsNoteResult {
	s, ok := jsUTF8(raw)
	if !ok {
		return jsDamaged
	}
	text := jsCleanNote(s)
	if text == "" {
		return jsDamaged
	}
	return jsNoteResult{outcome: "ok", text: text}
}

func jsParseRecords(s []byte) jsNoteResult {
	var textRaw []byte
	sawText := false
	i := 0
	for i < len(s) {
		tag := s[i]
		if tag == 0xff {
			break
		}
		i++
		length, n := jsUvarint(s, i)
		if n <= 0 {
			return jsDamaged
		}
		if length > uint64(len(s)-i-n) {
			return jsDamaged
		}
		i += n
		val := s[i : i+int(length)]
		i += int(length)
		if tag >= 'A' && tag <= 'Z' {
			return jsNoteResult{outcome: "newer"}
		}
		if tag == 't' && !sawText {
			sawText = true
			textRaw = val
		}
	}
	if !sawText {
		return jsDamaged
	}
	return jsLegacyText(textRaw)
}

func jsDecodeNotePayload(payload string) jsNoteResult {
	payload = jsTrim(payload)
	if payload == "" {
		return jsDamaged
	}
	bytes := jsBase64Bytes(payload)
	if bytes == nil || len(bytes) < 2 {
		return jsDamaged
	}
	b0, body := bytes[0], bytes[1:]
	const jsMaxInflated = 280*4 + 1
	const jsMaxRecordBytes = 4096
	switch {
	case b0 == 'p':
		return jsLegacyText(body)
	case b0 == 'z':
		raw := jsInflateRaw(body, jsMaxInflated)
		if raw == nil {
			return jsDamaged
		}
		return jsLegacyText(raw)
	case b0 == 'r':
		if len(body) > jsMaxRecordBytes {
			return jsDamaged
		}
		return jsParseRecords(body)
	case b0 == 'd':
		stream := jsInflateRaw(body, jsMaxRecordBytes)
		if stream == nil {
			return jsDamaged
		}
		return jsParseRecords(stream)
	case b0 >= 'A' && b0 <= 'Z':
		return jsNoteResult{outcome: "newer"}
	default:
		return jsDamaged
	}
}

// Whatever executes the JS, the SENTENCES must be the app's sentences, and the
// old fatal:false divergence must never come back.
func TestReaderJSCarriesTheTwoMessagesAndStrictUTF8(t *testing.T) {
	span := extractNoteDecoderJS(t)
	for _, want := range []string{
		"This link carries a note written in a newer note format.",
		"This link's note looks damaged.",
		"fatal: true",
	} {
		if !strings.Contains(span, want) {
			t.Errorf("reader.js decoder is missing %q", want)
		}
	}
	if strings.Contains(readerJSTemplate, "fatal: false") ||
		strings.Contains(readerJSTemplate, "fatal:false") {
		t.Error("reader.js still tolerates invalid UTF-8 (fatal:false) — the divergence the corpus exists to prevent")
	}
}

// ---------------------------------------------------------------------------
// The mirror against the shipped span, payload by payload.
// ---------------------------------------------------------------------------

// TestGoMirrorOfReaderJSAgreesWithNode decodes every probe with the shipped
// span under node and with jsDecodeNotePayload, and requires the same outcome
// and the same text from both. The corpus says what the app and the page must
// both do; this says the mirror does what the page does, including where the
// corpus has no line, so a rule added to the span without its twin in the
// mirror fails here wherever node is installed, not only on a machine with no
// JS runtime. A new rule that none of the probes reaches, or a new bound,
// needs probes added to mirrorProbes in the same change, on both sides of a
// bound: a mirror whose bound is one off agrees with node everywhere else.
func TestGoMirrorOfReaderJSAgreesWithNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("node not on PATH in CI, so the Go mirror of reader.js is held only to the corpus: %v", err)
		}
		t.Skip("node not on PATH; TestNoteVectorCorpusAgainstGoMirrorOfReaderJS still holds the mirror to the corpus")
	}
	probes := mirrorProbes(t)
	want := decodePayloadsUnderNode(t, node, probes)
	outcomes := map[string]int{}
	bad := 0
	for i, p := range probes {
		outcomes[want[i].Outcome]++
		got := jsDecodeNotePayload(p)
		if got.outcome == want[i].Outcome && got.text == want[i].Text {
			continue
		}
		bad++
		if bad <= 25 {
			t.Errorf("probe %d %s: mirror %s %q, reader.js %s %q",
				i, probeLabel(p), got.outcome, got.text, want[i].Outcome, want[i].Text)
		}
	}
	if bad > 25 {
		t.Errorf("%d disagreements in all", bad)
	}
	// A probe set that never reaches one of the three outcomes is not testing
	// the decoder, however many probes it has.
	for _, o := range []string{"ok", "newer", "damaged"} {
		if outcomes[o] == 0 {
			t.Errorf("no probe decodes as %s under node; the probe set has collapsed", o)
		}
	}
	t.Logf("%d probes under node (%s): %d ok, %d newer, %d damaged; %d disagreements",
		len(probes), node, outcomes["ok"], outcomes["newer"], outcomes["damaged"], bad)
}

type nodeNoteResult struct {
	Outcome string `json:"outcome"`
	Text    string `json:"text"`
}

// decodePayloadsUnderNode runs the shipped span under node over payloads and
// returns its results in the same order.
func decodePayloadsUnderNode(t *testing.T, node string, payloads []string) []nodeNoteResult {
	t.Helper()
	dir := t.TempDir()
	in, err := json.Marshal(payloads)
	if err != nil {
		t.Fatal(err)
	}
	inPath := filepath.Join(dir, "payloads.json")
	if err := os.WriteFile(inPath, in, 0o644); err != nil {
		t.Fatal(err)
	}
	harness := "'use strict';\n" + extractNoteDecoderJS(t) + `
const fs = require('fs');
const payloads = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
process.stdout.write(JSON.stringify(payloads.map(function (p) { return decodeNotePayload(p); })));
`
	script := filepath.Join(dir, "harness.js")
	if err := os.WriteFile(script, []byte(harness), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, script, inPath)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, stderr.String())
	}
	var res []nodeNoteResult
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("node output: %v", err)
	}
	if len(res) != len(payloads) {
		t.Fatalf("node returned %d results for %d payloads", len(res), len(payloads))
	}
	return res
}

func probeLabel(p string) string {
	if len(p) <= 48 {
		return fmt.Sprintf("%q", p)
	}
	return fmt.Sprintf("%q... (%d bytes)", p[:48], len(p))
}

// mirrorProbes is every corpus payload, payloads built by hand for rules the
// corpus has no line for, and seeded mutations of both. The generator's seed
// is fixed, so a failure reproduces from run to run.
func mirrorProbes(t *testing.T) []string {
	t.Helper()
	enc := base64.RawURLEncoding.EncodeToString
	var probes, seeds []string
	add := func(p ...string) { probes = append(probes, p...) }
	seed := func(p ...string) { seeds = append(seeds, p...); add(p...) }

	for _, v := range loadNoteVectors(t) {
		seed(v.payload)
	}

	// The base64 spelling and the trim in front of it, at all three unpadded
	// lengths a payload can have (4k, 4k+2 and 4k+3 characters): padding of
	// every length, '=' in the wrong place, the standard alphabet, non-ASCII,
	// ASCII whitespace inside (atob discards it, but only after the JS has
	// padded by a length that still counts it), the characters one trim
	// removes and another does not, and nonzero bits after the last byte.
	for _, note := range []string{"pab", "pabc", "pabcd"} {
		p := enc([]byte(note))
		add(p, p+"=", p+"==", p+"===", "="+p, p[:2]+"="+p[2:], p+"==cA",
			p[:1]+"+"+p[2:], p[:1]+"/"+p[2:], p+"\u00e9", "\u00e9"+p)
		for _, last := range []string{"w", "x", "z", "-", "_"} {
			add(p[:len(p)-1] + last)
		}
		for _, ws := range []string{" ", "\t", "\n", "\r", "\f", "\v", "\r\n", "  "} {
			for at := 1; at < len(p); at++ {
				add(p[:at] + ws + p[at:])
			}
			add(ws+p, p+ws, p+ws+"=")
		}
		for _, sp := range []string{"\ufeff", "\u0085", "\u00a0", "\u1680", "\u2000", "\u200a",
			"\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "\u180e", "\u200b"} {
			add(sp+p, p+sp)
		}
	}
	add("", " ", "\ufeff", "\u0085", "cA", "cA=", "cA==", "cGE", "cGF", "cGG", "cGH")

	// Text the decoder must clean, under the legacy 'p' framing.
	for _, note := range []string{
		"\xef\xbb\xbfled by a byte-order mark", "trailed by a byte-order mark\xef\xbb\xbf",
		"a\u2028\u2028\u2028b", "zero\u200bwidth", "\u200b", "tag\U000e0041char", "\u0085",
		"a\r\r\nb\n\n\n\nc", " \t spaced \u00a0", "\ufffd", "\xed\xa0\x80", "\xc0\xaf",
		strings.Repeat("\u00e9", 300), strings.Repeat("x", 279) + " y", "\u202eevil\u202c",
	} {
		add(enc(append([]byte{'p'}, note...)))
	}

	// A record stream at the 'r' size cap and one byte over it.
	for _, n := range []int{4093, 4094} {
		rec := append([]byte{'r', 't'}, binaryUvarint(uint64(n))...)
		add(enc(append(rec, strings.Repeat("a", n)...)))
	}

	// Real DEFLATE streams of every block type the encoder can choose, under
	// both framings that inflate.
	for _, level := range []int{flate.NoCompression, flate.HuffmanOnly, flate.BestSpeed, flate.BestCompression} {
		for _, note := range []string{"hi", "a probe note, a probe note, a probe note"} {
			rec := append(append([]byte{'t'}, binaryUvarint(uint64(len(note)))...), note...)
			seed(enc(append([]byte{'z'}, deflateProbe(t, []byte(note), level)...)),
				enc(append([]byte{'d'}, deflateProbe(t, rec, level)...)))
		}
	}

	// Hand-built dynamic blocks, one table rule each. The literal/length
	// table "full" is complete; the code-length table "cl" is complete.
	lens := func(n int, set map[int]int) []int {
		l := make([]int, n)
		for sym, length := range set {
			l[sym] = length
		}
		return l
	}
	full := lens(257, map[int]int{'h': 2, 'i': 2, '!': 2, 256: 2})
	short := lens(257, map[int]int{'h': 2, 'i': 2, 256: 2})
	over := lens(257, map[int]int{'h': 1, 'i': 1, 256: 1})
	cl := lens(19, map[int]int{0: 1, 1: 2, 2: 2})
	clShort := lens(19, map[int]int{0: 2, 1: 2, 2: 2})
	dyn := func(litLens, distLens, clLens []int) string {
		return enc(append([]byte{'z'}, dynamicBlock("hi", litLens, distLens, clLens)...))
	}
	seed(
		dyn(full, []int{1}, cl),      // one distance code of length 1: the degenerate table
		dyn(full, []int{1, 1}, cl),   // complete distance table
		dyn(full, []int{2}, cl),      // one distance code of length 2: incomplete
		dyn(full, []int{2, 2}, cl),   // two of four length-2 codes: incomplete
		dyn(full, []int{0}, cl),      // no distance codes at all
		dyn(short, []int{1}, cl),     // incomplete literal/length table
		dyn(over, []int{1}, cl),      // over-subscribed literal/length table
		dyn(full, []int{1}, clShort), // incomplete code-length table
	)

	// Every bound in the span, from both sides. A mirror whose bound is one
	// off agrees with node on everything else, so neither the corpus nor the
	// seeded mutations below reach these. They are added, not seeded, so the
	// mutations stay as they were.
	//
	// HLIT and HDIST at the most codes the span allows (286 literal/length,
	// 30 distance) and at the two counts above each that the 5-bit header
	// fields can still express.
	for _, n := range []int{286, 287, 288} {
		add(dyn(append(append([]int(nil), full...), make([]int, n-len(full))...), []int{1}, cl))
	}
	for _, n := range []int{30, 31, 32} {
		d := make([]int, n)
		d[0], d[1] = 1, 1
		add(dyn(full, d, cl))
	}
	// A repeat code that ends exactly where the code lengths do, and one that
	// runs one length past them.
	clRun := lens(19, map[int]int{0: 2, 1: 2, 2: 2, 17: 2})
	add(enc(append([]byte{'z'}, dynamicBlockRun("hi", full, []int{1, 1, 0, 0, 0}, clRun, len(full)+2, 3)...)),
		enc(append([]byte{'z'}, dynamicBlockRun("hi", full, []int{1, 1, 0, 0}, clRun, len(full)+2, 3)...)))

	// Fixed-code blocks: a back-reference to exactly the first byte out and
	// to one byte before it, length symbol 285 (258 bytes, the longest), and
	// symbols 286 and 287, which the fixed code has codes for and RFC 1951
	// does not use.
	for _, block := range [][]byte{
		fixedBlock("hi"),
		fixedBlock("ab", [2]int{257, 2}),
		fixedBlock("ab", [2]int{257, 3}),
		fixedBlock("a", [2]int{257, 1}),
		fixedBlock("", [2]int{257, 1}),
		fixedBlock("a", [2]int{285, 1}),
		fixedBlock("a", [2]int{286, 1}),
		fixedBlock("a", [2]int{287, 1}),
	} {
		add(enc(append([]byte{'z'}, block...)))
	}

	// The inflate caps: a 'z' note may inflate to 1121 bytes and a 'd' record
	// stream to 4096, and one byte more is damaged. A stored block reaches a
	// cap on a literal byte, a compressed one inside a back-reference.
	for _, level := range []int{flate.NoCompression, flate.BestCompression} {
		for _, n := range []int{1120, 1121, 1122} {
			add(enc(append([]byte{'z'}, deflateProbe(t, bytes.Repeat([]byte{'a'}, n), level)...)))
		}
		for _, n := range []int{4092, 4093, 4094} { // a stream of 4095, 4096 and 4097 bytes
			rec := append([]byte{'t'}, binaryUvarint(uint64(n))...)
			rec = append(rec, bytes.Repeat([]byte{'a'}, n)...)
			add(enc(append([]byte{'d'}, deflateProbe(t, rec, level)...)))
		}
	}

	// A ten-byte uvarint, the longest binary.Uvarint reads, whose last byte
	// may be only 0 or 1, as the length of a lowercase and an uppercase record
	// in front of a 't' record: zero, longer than the stream, or an overflow.
	// And eleven bytes, which have no last byte within the ten.
	for _, tag := range []byte{'x', 'A'} {
		for _, last := range []byte{0, 1, 2, 3, 0x7f} {
			rec := append([]byte{'r', tag}, bytes.Repeat([]byte{0x80}, 9)...)
			add(enc(append(rec, last, 't', 2, 'h', 'i')))
		}
		rec := append([]byte{'r', tag}, bytes.Repeat([]byte{0x80}, 10)...)
		add(enc(append(rec, 0, 't', 2, 'h', 'i')))
	}

	// Record tags either side of the uppercase range and of the stop byte,
	// each in front of a 't' record; a uvarint that ends on the stream's last
	// byte; and byte 0 either side of the uppercase range.
	for _, tag := range []byte{'@', 'A', 'Z', '[', 0xfe, 0xff} {
		add(enc([]byte{'r', tag, 0, 't', 2, 'h', 'i'}))
	}
	add(enc([]byte{'r', 't', 2, 'h', 'i', 'x', 0}))
	for _, b0 := range []byte{'@', 'A', 'Z', '['} {
		add(enc([]byte{b0, 't', 2, 'h', 'i'}))
	}

	// cleanNote's character classes at each end, between two letters so that
	// a character kept and a character dropped make different notes, and
	// runs of newlines either side of the three it shortens to two.
	for _, r := range []rune{0x00, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x1f, 0x20,
		0x7e, 0x7f, 0x9f, 0xa0, 0x200d, 0x200e, 0x200f, 0x2010, 0x2029, 0x202a, 0x202e,
		0x202f, 0x2065, 0x2066, 0x2069, 0x206a, 0xfefe, 0xfeff, 0xff00, 0xfffc, 0xfffd, 0xfffe} {
		add(enc([]byte("pa" + string(r) + "b")))
	}
	for n := 1; n <= 5; n++ {
		add(enc([]byte("pa" + strings.Repeat("\n", n) + "b")))
	}

	// atob's failure on one character over a whole group, which only interior
	// white space reaches: the JS pads by a length that still counts the white
	// space, so three such characters leave the payload unpadded, and atob,
	// once it has removed them, has one character too many.
	add("cGFi   Y", "c G F iY", "cGFiYWJj\t \nY")

	// Payloads a differential fuzz of the mirror against node turned up, one
	// for each of ten rules above that the earlier probes missed, kept as
	// found.
	add("UuRjr0x\n 0\fK", "cGEKCgpi", "cMKf", "ZCoAwf", "ekocKw", "ckGAgICAgICAgIAC",
		"ejq8chSOwlE4CmkBDy2oAgQAAP__", "ZOzAAQkAAAgDsChmFMwv73G2-1kAAACgXAIAAP__",
		"evXAK27AMAwA0ABLFwjKRayjBCfnCAgKD80JynOCgOgKEc4ZeoAoMB8p1SeSS55SioT_uexkoL5bKDXZ6pqtEN",
		"es3-OY4gMBDYAH5Mv6gXVFaFO6lcQf2hcKnwpfpBBYVCfaP0DgJ8BBEQdS_x1wI")

	// Seeded mutations: of the decoded bytes (so they land inside DEFLATE
	// headers, Huffman data and record framing), and of the spelling.
	rng := rand.New(rand.NewPCG(0x6e6f7465, 0x6d6972726f72))
	const noise = " \t\n\r\f\v=+/-_A"
	for _, s := range seeds {
		if raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")); err == nil && len(raw) > 0 {
			for k := 0; k < 24; k++ {
				m := append([]byte(nil), raw...)
				i := rng.IntN(len(m))
				switch k % 4 {
				case 0:
					m[i] ^= 1 << rng.IntN(8)
				case 1:
					m = m[:i]
				case 2:
					m[i] = byte(rng.IntN(256))
				default:
					m = append(m[:i:i], append([]byte{byte(rng.IntN(256))}, m[i:]...)...)
				}
				add(enc(m))
			}
		}
		for k := 0; k < 6; k++ {
			i := rng.IntN(len(s) + 1)
			add(s[:i] + string(noise[rng.IntN(len(noise))]) + s[i:])
		}
	}

	for _, p := range probes {
		if !utf8.ValidString(p) {
			t.Fatalf("probe %s is not valid UTF-8 and cannot reach the JS unchanged", probeLabel(p))
		}
	}
	return probes
}

func binaryUvarint(v uint64) []byte {
	return binary.AppendUvarint(nil, v)
}

func deflateProbe(t *testing.T, raw []byte, level int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// deflateBits packs a DEFLATE bit stream: fields least significant bit
// first, Huffman codes most significant bit first (RFC 1951, 3.1.1).
type deflateBits struct {
	out  []byte
	cur  byte
	used uint
}

func (w *deflateBits) bit(b uint64) {
	w.cur |= byte(b&1) << w.used
	if w.used++; w.used == 8 {
		w.out = append(w.out, w.cur)
		w.cur, w.used = 0, 0
	}
}

func (w *deflateBits) field(v uint64, n int) {
	for i := 0; i < n; i++ {
		w.bit(v >> i)
	}
}

func (w *deflateBits) code(c uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit(c >> i)
	}
}

func (w *deflateBits) bytes() []byte {
	if w.used > 0 {
		return append(w.out, w.cur)
	}
	return w.out
}

// canonicalCodes assigns the canonical Huffman code of RFC 1951, 3.2.2, to
// every symbol with a nonzero length. It does not check that the lengths
// make a valid code; the probes above depend on that.
func canonicalCodes(lengths []int) []uint64 {
	var count [16]int
	for _, l := range lengths {
		if l > 0 {
			count[l]++
		}
	}
	var next [16]uint64
	code := uint64(0)
	for l := 1; l <= 15; l++ {
		code = (code + uint64(count[l-1])) << 1
		next[l] = code
	}
	codes := make([]uint64, len(lengths))
	for sym, l := range lengths {
		if l > 0 {
			codes[sym] = next[l]
			next[l]++
		}
	}
	return codes
}

// fixedBlock is one final DEFLATE block of type 1 (RFC 1951, 3.2.6): each
// byte of lits as a literal, then each ref as a literal/length symbol ref[0]
// and the distance code for distance ref[1], then end-of-block. Only the
// symbols whose lengths carry no extra bits (257 to 264, lengths 3 to 10, and
// 285, length 258) and distances 1 to 4, which carry none either, are sent
// as meant; symbols 286 and 287, which name no length, are sent all the same,
// with a distance code after them.
func fixedBlock(lits string, refs ...[2]int) []byte {
	lens := make([]int, 288)
	for i := range lens {
		switch {
		case i < 144:
			lens[i] = 8
		case i < 256:
			lens[i] = 9
		case i < 280:
			lens[i] = 7
		default:
			lens[i] = 8
		}
	}
	codes := canonicalCodes(lens)
	w := &deflateBits{}
	w.field(1, 1) // BFINAL
	w.field(1, 2) // BTYPE: fixed Huffman codes
	for i := 0; i < len(lits); i++ {
		w.code(codes[lits[i]], lens[lits[i]])
	}
	for _, ref := range refs {
		w.code(codes[ref[0]], lens[ref[0]])
		w.code(uint64(ref[1]-1), 5)
	}
	w.code(codes[256], lens[256])
	return w.bytes()
}

// dynamicBlock is one final DEFLATE block of type 2 holding text as
// literals, under the given code lengths for the literal/length alphabet
// (litLens, 257 or more entries), the distance alphabet (distLens) and the
// code-length alphabet (clLens, 19 entries by symbol). Every code length is
// sent as itself, never as a repeat, so clLens needs a code for each length
// used.
func dynamicBlock(text string, litLens, distLens, clLens []int) []byte {
	return dynamicBlockRun(text, litLens, distLens, clLens, len(litLens)+len(distLens), 0)
}

// dynamicBlockRun is dynamicBlock with only the first sent code lengths sent
// one by one, followed, when run is not zero, by one repeat-zero code (17)
// for run more zero lengths, 3 to 10 of them, which may end where the
// header's two counts end or past it. clLens needs a code for 17 then.
func dynamicBlockRun(text string, litLens, distLens, clLens []int, sent, run int) []byte {
	order := [19]int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}
	w := &deflateBits{}
	w.field(1, 1) // BFINAL
	w.field(2, 2) // BTYPE: dynamic Huffman codes
	w.field(uint64(len(litLens)-257), 5)
	w.field(uint64(len(distLens)-1), 5)
	w.field(19-4, 4)
	for _, sym := range order {
		w.field(uint64(clLens[sym]), 3)
	}
	clCodes := canonicalCodes(clLens)
	for _, l := range append(append([]int(nil), litLens...), distLens...)[:sent] {
		w.code(clCodes[l], clLens[l])
	}
	if run > 0 {
		w.code(clCodes[17], clLens[17])
		w.field(uint64(run-3), 3)
	}
	litCodes := canonicalCodes(litLens)
	for i := 0; i < len(text); i++ {
		w.code(litCodes[text[i]], litLens[text[i]])
	}
	w.code(litCodes[256], litLens[256])
	return w.bytes()
}
