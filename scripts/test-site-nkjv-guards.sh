#!/usr/bin/env bash
# Regression for scripts/site-nkjv-guards.sh and the built-tree key scan in
# scripts/check-repository-hygiene.py — the publish guards for each state of
# the NKJV switch (cmd/websitegen/nkjv_text.go).
#
# Every tree here is synthetic MARKUP: the shapes the generator writes, with no
# scripture in them. Every key is a synthetic string, and the key scan runs
# with no `security` on its PATH, so no developer Keychain is ever consulted.
#
# Each guard is shown passing on a good tree and failing on each fault it
# exists for, including the other state's tree — the guard that would let
# either state publish the other's tree is the one this file pins hardest.
set -euo pipefail
umask 077
unset BIBLE_API_KEY BIBLETEXT_SITE_NKJV_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY

cd "$(dirname "$0")/.."
. scripts/site-nkjv-guards.sh
T=$(mktemp -d "${TMPDIR:-/tmp}/bibletext-nkjv-guards.XXXXXX")
trap 'rm -rf "$T"' EXIT
PY="$(command -v python3)"

fail() { echo "FAIL: $*" >&2; exit 1; }
passes() { local what="$1"; shift; "$@" >"$T/out" 2>&1 || fail "$what: $(cat "$T/out")"; }
refuses() {
  local what="$1" want="$2"; shift 2
  if "$@" >"$T/out" 2>&1; then fail "$what was accepted"; fi
  grep -Fq -- "$want" "$T/out" || fail "$what failed for the wrong reason: $(cat "$T/out")"
}

DATE=2026-10-02
NOTICE='Scripture taken from the New King James Version®. Copyright © 1982 by Thomas Nelson.'
foot() {
  printf '<footer class="foot"><a id="getapp" href="/">app</a><p class="lic">%s</p>' "$NOTICE"
  printf '<p class="retrieved">Text retrieved from <a href="https://api.bible">API.Bible</a> on <time datetime="%s">%s</time>.</p></footer>' "$1" "$1"
}

# The text-on tree: three chapters of text (John 3's sixteen verses, John 1's
# two and Jude 1's one, so the totals below are 3 pages and 19 verses), one
# canon-gap notice, two indexes, the stylesheet and the face it names, and a
# public-domain page beside them.
make_on() {
  local out="$1" d n
  mkdir -p "$out/assets" "$out/web/john/3" "$out/nkjv/john/3" "$out/nkjv/john/1" "$out/nkjv/jude/1" "$out/nkjv/tobit/1"
  printf 'font' > "$out/assets/Junicode-SmallCaps.0123456789.woff2"
  printf '@font-face{src:url(Junicode-SmallCaps.0123456789.woff2)}' > "$out/assets/nkjv.abcdef0123.css"
  printf 'notice' > "$out/assets/notice.1111111111.css"
  verses() { for ((n = 1; n <= $1; n++)); do printf '<span class="v" id="v%d">x</span>' "$n"; done; }
  for d in john/3:16 john/1:2 jude/1:1; do
    { printf '<link rel="stylesheet" href="../../../assets/nkjv.abcdef0123.css">'
      verses "${d##*:}"; foot "$DATE"; } > "$out/nkjv/${d%%:*}/index.html"
  done
  { printf '<link rel="stylesheet" href="../../../assets/notice.1111111111.css">'; foot "$DATE"; } > "$out/nkjv/tobit/1/index.html"
  foot "$DATE" > "$out/nkjv/index.html"
  foot "$DATE" > "$out/nkjv/john/index.html"
  printf '<span class="v" id="v16">x</span><footer class="foot"></footer>' > "$out/web/john/3/index.html"
}

# The notice-only tree.
make_off() {
  local out="$1"
  mkdir -p "$out/assets" "$out/nkjv/john/3" "$out/nkjv/jude/1" "$out/web/john/3"
  printf 'notice' > "$out/assets/notice.1111111111.css"
  printf 'notice' > "$out/assets/notice.2222222222.js"
  printf '%s' '<h1>John 3</h1><p>New King James Version</p><a id="openapp" href="/">app</a><a href="../../../web/john/3/">WEB</a><link href="../../../assets/notice.1111111111.css"><script src="../../../assets/notice.2222222222.js"></script>' \
    > "$out/nkjv/john/3/index.html"
  printf 'New King James Version' > "$out/nkjv/jude/1/index.html"
  printf '<span class="v" id="v16">x</span>' > "$out/web/john/3/index.html"
}

fresh() { rm -rf "$T/$1"; "make_$2" "$T/$1"; echo "$T/$1"; }
# sub EXPR FILE edits a file in place on both sed dialects.
sub() { sed "$1" "$2" > "$2.edit" && mv "$2.edit" "$2"; }

# --- the state the generator reports --------------------------------------------
fake_gen() { printf '#!/bin/sh\n%s\n' "$2" > "$T/$1"; chmod +x "$T/$1"; echo "$T/$1"; }
[[ "$(nkjv_text_state "$(fake_gen on-gen 'echo on')")" == on ]] || fail "an 'on' generator was not read as on"
[[ "$(nkjv_text_state "$(fake_gen off-gen 'echo off')")" == off ]] || fail "an 'off' generator was not read as off"
refuses "a generator reporting 'maybe'" "not on or off" nkjv_text_state "$(fake_gen maybe-gen 'echo maybe')"
refuses "a generator reporting nothing" "not on or off" nkjv_text_state "$(fake_gen empty-gen 'true')"
refuses "a generator that cannot report" "could not report" nkjv_text_state "$(fake_gen broken-gen 'exit 2')"

# --- off ---------------------------------------------------------------------------
passes "a good notice-only tree" nkjv_guard_off "$(fresh off off)"
o=$(fresh off off); printf '<span class="v" id="v1">x</span>' >> "$o/nkjv/jude/1/index.html"
refuses "verse markup planted under /nkjv/ with the switch off" "LICENSED TEXT IS ABOUT TO BE PUBLISHED" nkjv_guard_off "$o"
o=$(fresh off off); printf '<span class="wj">x</span>' >> "$o/nkjv/jude/1/index.html"
refuses "a red-letter span under /nkjv/ with the switch off" "carry verse markup" nkjv_guard_off "$o"
refuses "the text-on tree, fed to the off guard" "no open-in-app affordance" nkjv_guard_off "$(fresh on on)"
o=$(fresh on on); printf '<a id="openapp"></a><a href="../../../web/john/3/"></a>' >> "$o/nkjv/john/3/index.html"
refuses "the text-on tree dressed as notices, fed to the off guard" "LICENSED TEXT IS ABOUT TO BE PUBLISHED" nkjv_guard_off "$o"
o=$(fresh off off); rm "$o/nkjv/john/3/index.html"
refuses "a missing /nkjv/john/3/" "is missing" nkjv_guard_off "$o"
o=$(fresh off off); sub 's/notice\.1111111111\.css/notice.9999999999.css/' "$o/nkjv/john/3/index.html"
refuses "a notice page not linking the notice stylesheet" "do not reference notice.1111111111.css" nkjv_guard_off "$o"
o=$(fresh off off); printf 'x' > "$o/assets/nkjv.abcdef0123.css"
refuses "the text's stylesheet in a notice-only tree" "stylesheet with the switch off" nkjv_guard_off "$o"
o=$(fresh off off); foot "$DATE" >> "$o/web/john/3/index.html"
refuses "the text's footer in a notice-only tree" "footer with the switch off" nkjv_guard_off "$o"

# --- on ----------------------------------------------------------------------------
passes "a good text tree" nkjv_guard_on "$(fresh on on)" 3 19 "$DATE"
passes "a good text tree across midnight" nkjv_guard_on "$(fresh on on)" 3 19 2026-10-01 "$DATE"
refuses "the notice-only tree, fed to the on guard" "no verse anchors" nkjv_guard_on "$(fresh off off)" 3 19 "$DATE"
o=$(fresh on on); sub 's/<span class="v" id="v2">x<\/span>//' "$o/nkjv/john/1/index.html"
refuses "one verse anchor removed" "has 18 verses, expected exactly 19" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); rm "$o/nkjv/jude/1/index.html"
refuses "one chapter of text removed" "has 2 chapter pages of text, expected exactly 3" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); printf 'nothing' > "$o/nkjv/jude/1/index.html"
refuses "a chapter page with neither text nor notice" "expected exactly 3" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); sub 's/Scripture taken from the New King James Version/Scripture/' "$o/nkjv/john/index.html"
refuses "the copyright notice stripped from one page" "lack the copyright notice" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); sub 's|https://api.bible|https://example.invalid|' "$o/nkjv/tobit/1/index.html"
refuses "the retrieval line stripped from one page" "lack the retrieval line" nkjv_guard_on "$o" 3 19 "$DATE"
refuses "yesterday's text" "not on this run's date" nkjv_guard_on "$(fresh on on)" 3 19 2026-10-01
o=$(fresh on on); sub "s/$DATE/2026-09-30/" "$o/nkjv/jude/1/index.html"
refuses "one page with a different date" "different retrieval date" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); sub 's/<time datetime="[0-9-]*">//' "$o/nkjv/index.html"
refuses "one page without the date" "different retrieval date" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); foot "$DATE" >> "$o/web/john/3/index.html"
refuses "the retrieval line planted under /web/" "outside /nkjv/ carry" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); printf '<link href="../../../assets/notice.1111111111.css">' >> "$o/nkjv/john/3/index.html"
refuses "/nkjv/john/3/ linking the notice assets" "is a notice page" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); rm "$o/assets/Junicode-SmallCaps.0123456789.woff2"
refuses "a face the stylesheet names, missing" "which is not in the tree" nkjv_guard_on "$o" 3 19 "$DATE"
o=$(fresh on on); sub 's/nkjv\.abcdef0123\.css/nkjv.0000000000.css/' "$o/nkjv/john/3/index.html"
refuses "the stylesheet not linked" "does not link nkjv.abcdef0123.css" nkjv_guard_on "$o" 3 19 "$DATE"

# --- the key, in the tree about to be published ---------------------------------
# PATH holds no `security`, so the scan sees exactly the synthetic key given to
# it in BIBLE_API_KEY and never a Keychain.
NOBIN="$T/nobin"; mkdir -p "$NOBIN"
KEY=synthetic-site-key-0123456789abcdef
scan() { env -i PATH="$NOBIN" HOME="$T" ${1:+BIBLE_API_KEY="$1"} "$PY" scripts/check-repository-hygiene.py --scan-built-tree "$2" --require-release-key; }
passes "a clean tree" scan "$KEY" "$(fresh on on)"
o=$(fresh on on); printf '%s' "$KEY" >> "$o/nkjv/john/3/index.html"
refuses "the key in plaintext" "nkjv/john/3/index.html: BIBLE_API_KEY from the process environment appears in plaintext form" scan "$KEY" "$o"
grep -Fq "$KEY" "$T/out" && fail "the scan printed the key"
o=$(fresh on on); printf '%s' "$KEY" | "$PY" -c 'import base64,sys; sys.stdout.write(base64.b64encode(sys.stdin.buffer.read()).decode())' >> "$o/assets/nkjv.abcdef0123.css"
refuses "the key in base64" "appears in base64 form" scan "$KEY" "$o"
o=$(fresh on on); printf '%s' "$KEY" | "$PY" -c '
import base64, sys
key = sys.stdin.buffer.read(); mask = b"bibletext-nkjv"
sys.stdout.write(base64.b64encode(bytes(b ^ mask[i % len(mask)] for i, b in enumerate(key))).decode())' >> "$o/web/john/3/index.html"
refuses "the key in the release linker's form" "appears in release-linker encoding form" scan "$KEY" "$o"
refuses "a scan with no key to look for" "the API.Bible key was not found" scan "" "$(fresh on on)"

echo "site NKJV guards: OK"
