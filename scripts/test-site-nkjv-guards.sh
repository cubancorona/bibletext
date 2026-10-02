#!/usr/bin/env bash
# Regression for scripts/site-nkjv-guards.sh and the built-tree key scan in
# scripts/check-repository-hygiene.py — the publish guards for each state of
# the NKJV switch (cmd/websitegen/nkjv_text.go).
#
# And the exit path publish-site.sh arms (arm_site_cleanup), which removes every
# tree that holds the text however a run with it on ends.
#
# And the glyph guard (site_guard_glyphs, scripts/check-site-glyphs.py), on a
# whole site the generator writes in each state from SYNTHETIC text, because
# that guard reads real stylesheets and real faces. It needs Go and fontTools.
#
# Every other tree here is synthetic MARKUP: the shapes the generator writes,
# with no scripture in them. Every key is a synthetic string, and the key scan
# runs with no `security` on its PATH, so no developer Keychain is ever
# consulted.
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

# --- the type: every character in a face the page declares -----------------------
# Real trees this time, because the guard reads real stylesheets and real faces:
# the generator writes each state through run() with the network replaced and
# SYNTHETIC text (cmd/websitegen/glyph_fixture_test.go) carrying every character
# the reading face's supplements exist for — each Hebrew letter and mark, the
# notes' Greek, the divine name in small capitals in a verse, a bold heading, an
# italic title and an italic supplied word. Each control then takes one thing
# away and the guard must name it.
glyph_tree() {
  rm -rf "$T/glyph-$1"
  BIBLETEXT_GLYPH_FIXTURE_OUT="$T/glyph-$1" BIBLETEXT_GLYPH_FIXTURE_STATE="$1" \
    go test -count=1 -run '^TestWriteGlyphFixtureSite$' ./cmd/websitegen >"$T/out" 2>&1 ||
    fail "the generator did not write the $1 glyph fixture: $(tail -5 "$T/out")"
  [[ -s "$T/glyph-$1/nkjv/john/3/index.html" ]] || fail "the $1 glyph fixture has no /nkjv/john/3/"
}
glyph_tree on
glyph_tree off
passes "the text-on site, every face in place" site_guard_glyphs "$T/glyph-on"
passes "the notice-only site, every face in place" site_guard_glyphs "$T/glyph-off"
gcopy() { rm -rf "$T/g"; cp -R "$T/glyph-on" "$T/g"; echo "$T/g"; }
# without CP FONT drops one code point from a copy of a face, keeping its name.
without() {
  "$PY" - "$1" "$2" <<'PY'
import sys
from fontTools import subset
from fontTools.ttLib import TTFont
cp, path = int(sys.argv[1], 16), sys.argv[2]
font = TTFont(path)
keep = [c for c in font.getBestCmap() if c != cp]
opts = subset.Options(); opts.layout_features = ["*"]; opts.name_IDs = ["*"]; opts.notdef_outline = True
sub = subset.Subsetter(opts); sub.populate(unicodes=keep); sub.subset(font)
font.flavor = "woff2"; font.save(path)
PY
}
# The Hebrew face without alef: Psalm 119's first stanza letter falls to a system Hebrew.
g=$(gcopy); without 05D0 "$(ls "$g"/assets/BibleTextHebrew.*.woff2)"
refuses "the Hebrew face without alef" "U+05D0 HEBREW LETTER ALEF" site_guard_glyphs "$g"
grep -Fq "h2.sec" "$T/out" || fail "the missing alef was not traced to the heading: $(cat "$T/out")"
# The italic small capitals undeclared: the divine name in a supplied word falls
# to a system italic, as it did before the italic supplement shipped.
g=$(gcopy); css=$(ls "$g"/assets/nkjv.*.css)
"$PY" - "$css" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p, encoding="utf-8").read()
s, n = re.subn(r"@font-face\{[^}]*Junicode-ItalicSmallCaps[^}]*\}", "", s)
assert n == 1, n
open(p, "w", encoding="utf-8").write(s)
PY
refuses "the italic small capitals undeclared" "U+1D0F LATIN LETTER SMALL CAPITAL O" site_guard_glyphs "$g"
grep -Fq "400 italic" "$T/out" || fail "the italic small capitals were not traced to an italic run: $(cat "$T/out")"
# The bold small capitals' file missing: a declared face that is not in the tree.
g=$(gcopy); rm "$g"/assets/Junicode-BoldSmallCaps.*.woff2
refuses "a declared face missing from the tree" "which is not in the tree" site_guard_glyphs "$g"
# The true italic undeclared: every psalm title becomes a slanted regular.
g=$(gcopy); css=$(ls "$g"/assets/reader.*.css)
"$PY" - "$css" <<'PY'
import re, sys
p = sys.argv[1]; s = open(p, encoding="utf-8").read()
s, n = re.subn(r"@font-face\{[^}]*font-style:italic[^}]*\}", "", s)
assert n == 1, n
open(p, "w", encoding="utf-8").write(s)
PY
refuses "the true italic undeclared" "as a slanted regular (no italic cut)" site_guard_glyphs "$g"
# The Greek supplement without its range: the notes' Greek falls to Georgia.
g=$(gcopy); without 03B1 "$(ls "$g"/assets/Junicode-Greek.*.woff2)"
refuses "the Greek supplement without alpha" "U+03B1 GREEK SMALL LETTER ALPHA" site_guard_glyphs "$g"
# A character no face carries, in a verse; and an arrow, which the chrome may
# leave to the system, in scripture, where it may not.
g=$(gcopy); sub 's/<span class="v" id="v1">/<span class="v" id="v1">Ж /' "$g/nkjv/john/3/index.html"
refuses "a Cyrillic letter in a verse" "U+0416 CYRILLIC CAPITAL LETTER ZHE" site_guard_glyphs "$g"
# A report names code points, contexts and paths, and never a page's words.
grep -Fq "fixture verse" "$T/out" && fail "the glyph guard printed page text: $(cat "$T/out")"
g=$(gcopy); sub 's/<span class="v" id="v1">/<span class="v" id="v1">→ /' "$g/web/john/3/index.html"
refuses "an arrow in scripture" "U+2192 RIGHTWARDS ARROW" site_guard_glyphs "$g"
grep -q 'class="arrow"' "$T/glyph-on/web/john/3/index.html" ||
  fail "the fixture has no chapter arrows, so the pass above did not cover the chrome's arrows"
# The font shorthand is read: Scripture moved into the chrome face by one loses
# the Hebrew of its headings.
g=$(gcopy); printf '.text{font:1em "Atkinson Hyperlegible",sans-serif}' >> "$(ls "$g"/assets/reader.*.css)"
refuses "Scripture set in the chrome face by the font shorthand" "U+05D0 HEBREW LETTER ALEF" site_guard_glyphs "$g"
# A hand-written page that asks for the system's face first, as the landing
# pages do, has chosen it: nothing of ours covers it, and nothing is refused.
g=$(gcopy); printf '<!doctype html><style>body{font: 17px/1.6 -apple-system, "Segoe UI", serif}</style><body><p>Ж → %s</p>' \
  "$(printf '\xd7\x90')" > "$g/privacy.html"
passes "a page set in the system face by its own choice" site_guard_glyphs "$g"
# What the guard cannot evaluate, it refuses rather than guesses.
g=$(gcopy); printf '.text{font:small-caps 1em "Junicode"}' >> "$(ls "$g"/assets/reader.*.css)"
refuses "a small-caps shorthand" "could not judge the tree" site_guard_glyphs "$g"
g=$(gcopy); printf '.text:hover{font-weight:700}' >> "$(ls "$g"/assets/reader.*.css)"
refuses "a font rule behind :hover" "could not judge the tree" site_guard_glyphs "$g"

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

# --- the exit path ------------------------------------------------------------------
# A child shell arms the cleanup as publish-site.sh does, holds the three trees a
# run makes, and leaves the way under test. It runs in a repository of its own,
# so the gh-pages prune never touches this one.
ROOT="$PWD"
git init -q "$T/repo"
leave() {
  local state="$1" how="$2" status=0
  rm -rf "$T/repo/build" "$T/live"
  mkdir -p "$T/repo/build/site/nkjv" "$T/repo/build/gh-pages" "$T/live"
  bash -c '
    set -euo pipefail
    cd "$1"
    . "$2/scripts/site-nkjv-guards.sh"
    OUT=build/site
    WORKTREE=build/gh-pages
    arm_site_cleanup
    NKJV_TEXT="$3"
    LIVE_TREE="$4"
    PAGES_CHECKOUT=true
    case "$5" in
      done) exit 0 ;;
      refused) exit 1 ;;
      failed) false ;;
      interrupted) kill -INT $$; sleep 5 ;;
      terminated) kill -TERM $$; sleep 5 ;;
    esac
  ' _ "$T/repo" "$ROOT" "$state" "$T/live" "$how" >"$T/out" 2>&1 || status=$?
  echo "$status"
}
gone() { [[ ! -e "$T/$1" ]] || fail "$2 left $1 behind"; }
kept() { [[ -d "$T/$1" ]] || fail "$2 removed $1"; }
for how in done:0 refused:1 failed:1 interrupted:130 terminated:143; do
  [[ "$(leave on "${how%%:*}")" == "${how##*:}" ]] || fail "a run with the text on, ${how%%:*}, did not keep its exit status ${how##*:}"
  gone repo/build/site "a run with the text on, ${how%%:*},"
  gone live "a run with the text on, ${how%%:*},"
  gone repo/build/gh-pages "a run with the text on, ${how%%:*},"
  grep -Fq "removed build/site: it held the NKJV's text" "$T/out" || fail "a run with the text on did not say it removed build/site"
done
for how in done:0 interrupted:130; do
  [[ "$(leave off "${how%%:*}")" == "${how##*:}" ]] || fail "a run with the text off, ${how%%:*}, did not keep its exit status ${how##*:}"
  kept repo/build/site "a run with the text off, ${how%%:*},"
  gone live "a run with the text off, ${how%%:*},"
  gone repo/build/gh-pages "a run with the text off, ${how%%:*},"
done

echo "site NKJV guards: OK"
