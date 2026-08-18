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
// HOW THE JS IS EXECUTED, in order of preference; the test log says which ran:
//
//  1. node, when it is on PATH: the decoder span is extracted from
//     readerJSTemplate between its two markers — the span is pure by contract,
//     no DOM — and node runs the REAL shipped code over every vector.
//  2. osascript -l JavaScript (JavaScriptCore, present on every macOS): the
//     SAME shipped span, with test-only polyfills for the two browser
//     primitives JSC lacks (atob, TextDecoder). The inflate, the record walk
//     and the outcome logic are the real bytes; only base64 and UTF-8
//     primitives are stand-ins.
//  3. jsDecodeNotePayload below: a deliberately line-by-line Go
//     re-implementation OF THE JAVASCRIPT (not of share_note.go — porting the
//     Go decoder again would only prove Go agrees with itself).

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
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
// The Go re-implementation OF THE JAVASCRIPT, function for function. Where the
// JS uses floats the port uses uint64 — equivalent over every length a 4 KB
// stream can hold. Keep this in lockstep with the span between the markers.
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

func jsHuffConstruct(lengths []int, n int) *jsHuff {
	h := &jsHuff{}
	for i := 0; i < n; i++ {
		l := 0
		if i < len(lengths) {
			l = lengths[i]
		}
		h.count[l]++
	}
	left := 1
	for l := 1; l <= 15; l++ {
		left <<= 1
		left -= h.count[l]
		if left < 0 {
			return nil
		}
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
		clcode := jsHuffConstruct(lengths, 19)
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
		lc := jsHuffConstruct(symlens[:nlen], nlen)
		dc := jsHuffConstruct(symlens[nlen:], ndist)
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
				fixedLen = jsHuffConstruct(fl, 288)
				fixedDist = jsHuffConstruct(fd, 30)
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

func jsBase64Bytes(payload string) []byte {
	b64 := strings.ReplaceAll(payload, "-", "+")
	b64 = strings.ReplaceAll(b64, "_", "/")
	for len(b64)%4 != 0 {
		b64 += "="
	}
	out, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	return out
}

func jsUTF8(b []byte) (string, bool) {
	if !utf8.Valid(b) {
		return "", false // TextDecoder {fatal:true} throws
	}
	return string(b), true
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
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > 280 {
		s = strings.TrimSpace(string(runes[:280]))
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
	payload = strings.TrimSpace(payload)
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
