#!/usr/bin/env bash
# The publish guards for the NKJV, one per state of the switch in
# cmd/websitegen/nkjv_text.go. Sourced by publish-site.sh; tested by
# test-site-nkjv-guards.sh.
#
#   nkjv_text_state BINARY
#       the state the built generator reports (-print-nkjv-text): prints "on"
#       or "off", or prints why it cannot tell and returns 1. The script and
#       the generator therefore cannot disagree: the script asks the very
#       binary it then runs.
#   nkjv_guard_off TREE
#       the notice-only tree: /nkjv/ names the translation, offers the app and
#       the parallel passage, links the notice assets, and carries NO verse
#       markup and nothing the text-on state writes.
#   nkjv_guard_on TREE SCRIPTURE_PAGES VERSES DATE...
#       the text: exactly SCRIPTURE_PAGES chapter pages of text and the rest
#       canon-gap notices, exactly VERSES verse anchors, the copyright notice
#       and the retrieval line ("Text provided by API.Bible (api.bible),
#       retrieved <date>.", retrievedLineFormat in cmd/websitegen/nkjv_text.go)
#       on every /nkjv/ page with a date that is one of DATE (this run's London
#       dates, so the text was fetched by this build), the provider credited by
#       that line alone, none of it anywhere else, and every face its
#       stylesheet names present.
#
#   site_guard_glyphs TREE
#       the type, in BOTH states: every character a page sets in a web face is
#       drawn by a face its stylesheets declare, and no run is set in a cut the
#       browser must fake (scripts/check-site-glyphs.py, which reads the built
#       stylesheets and the faces' own character maps). Psalm 119's stanza
#       letters in a system Hebrew, a small capital of the divine name in a
#       system italic, a psalm title as a slanted regular: each stops the
#       publish. A tree the guard cannot judge stops it too.
#
# Each prints what failed (paths and code points only — never page text) and
# returns 1, or returns 0 silently.
#
#   arm_site_cleanup
#       the one exit path of publish-site.sh, armed before anything is built.
#       However the run ends — published, dry run, refused, interrupted — it
#       removes the copy of the live branch (LIVE_TREE, while set), the gh-pages
#       checkout (WORKTREE, once PAGES_CHECKOUT is true) and, while the NKJV's
#       text is on (NKJV_TEXT), the built tree (OUT), keeping the run's exit
#       status. The caller sets OUT and WORKTREE; the other three start empty.

nkjv_text_state() {
  local bin="$1" state
  if ! state=$("$bin" -print-nkjv-text 2>/dev/null); then
    echo "the generator could not report the NKJV switch (-print-nkjv-text failed)"
    return 1
  fi
  case "$state" in
    on|off) echo "$state" ;;
    *)
      echo "the generator reports the NKJV switch as '$state', not on or off"
      return 1
      ;;
  esac
}

nkjv_guard_off() {
  local out="$1" page leak asset
  page="$out/nkjv/john/3/index.html"
  # The licensed tree gets the strongest check this script can make: it must
  # name the translation, offer the app AND the parallel passage, and carry NO
  # verse markup. paragraphBody is the only thing that writes a verse anchor or
  # a red-letter span, so either appearing under /nkjv/ means scripture reached
  # a tree that must hold none while the switch is off.
  [[ -s "$page" ]] || { echo "/nkjv/john/3/ is missing — NKJV share links would 404 again"; return 1; }
  grep -q 'New King James Version' "$page" || { echo "/nkjv/john/3/ does not name the translation"; return 1; }
  grep -q 'id="openapp"' "$page" || { echo "/nkjv/john/3/ has no open-in-app affordance"; return 1; }
  grep -q 'href="../../../web/john/3/"' "$page" || { echo "/nkjv/john/3/ offers no parallel passage"; return 1; }
  leak=$(find "$out/nkjv" -name index.html -print0 |
    xargs -0 -r grep -lE 'class="v" id="v|class="wj"|<sup class="n"' | head -3 || true)
  [[ -z "$leak" ]] || { echo "pages under /nkjv/ carry verse markup — LICENSED TEXT IS ABOUT TO BE PUBLISHED with the switch off: $leak"; return 1; }
  # The notice pages carry their OWN hashed pair, on top of the reader's.
  for asset in $(find "$out/assets" -name 'notice.*.css' -o -name 'notice.*.js'); do
    grep -q "assets/$(basename "$asset")" "$page" ||
      { echo "notice pages do not reference $(basename "$asset") — the build linked an asset it did not write"; return 1; }
  done
  # And nothing the text-on state writes.
  [[ -z "$(find "$out/assets" -name 'nkjv.*.css')" ]] ||
    { echo "the tree carries the NKJV text's stylesheet with the switch off"; return 1; }
  leak=$(grep -rl --include='*.html' -e 'class="retrieved"' -e 'class="lic"' "$out" | head -3 || true)
  [[ -z "$leak" ]] || { echo "pages carry the NKJV text's footer with the switch off: $leak"; return 1; }
  return 0
}

nkjv_guard_on() {
  local out="$1" want_pages="$2" want_verses="$3"
  shift 3
  local page="$out/nkjv/john/3/index.html" got bad date d dated=false css f
  [[ -s "$page" ]] || { echo "/nkjv/john/3/ is missing — NKJV share links would 404"; return 1; }
  grep -q 'id="v16"' "$page" || { echo "/nkjv/john/3/ has no verse anchors — the NKJV's text is not on the page"; return 1; }
  if grep -q 'id="openapp"' "$page" || grep -q 'assets/notice\.' "$page"; then
    echo "/nkjv/john/3/ is a notice page, not the text"; return 1
  fi

  # Exactly SCRIPTURE_PAGES chapter pages carry text, and every other chapter
  # page is a canon-gap notice. -mindepth/-maxdepth 3 is the chapter depth, as
  # in the count loop.
  got=$(find "$out/nkjv" -mindepth 3 -maxdepth 3 -name index.html -print0 |
    xargs -0 -r grep -l 'class="v" id="v' | wc -l | tr -d ' ' || true)
  [[ "$got" -eq "$want_pages" ]] ||
    { echo "/nkjv/ has $got chapter pages of text, expected exactly $want_pages — the text looks incomplete"; return 1; }
  bad=""
  while IFS= read -r -d '' f; do
    grep -q 'class="v" id="v' "$f" || grep -q 'assets/notice\.' "$f" || { bad="$f"; break; }
  done < <(find "$out/nkjv" -mindepth 3 -maxdepth 3 -name index.html -print0)
  [[ -z "$bad" ]] || { echo "a chapter page under /nkjv/ carries neither text nor a notice: $bad"; return 1; }
  got=$(grep -rho --include=index.html 'class="v" id="v' "$out/nkjv" | wc -l | tr -d ' ' || true)
  [[ "$got" -eq "$want_verses" ]] ||
    { echo "/nkjv/ has $got verses, expected exactly $want_verses — the text looks incomplete"; return 1; }

  # The rights holder's notice and the retrieval line, on every page of the
  # tree, and the date this run's.
  bad=$(find "$out/nkjv" -name '*.html' -print0 |
    xargs -0 -r grep -L 'Scripture taken from the New King James Version' | head -3 || true)
  [[ -z "$bad" ]] || { echo "pages under /nkjv/ lack the copyright notice: $bad"; return 1; }
  bad=$(find "$out/nkjv" -name '*.html' -print0 |
    xargs -0 -r grep -L -F 'Text provided by API.Bible (<a href="https://api.bible">api.bible</a>), retrieved ' |
    head -3 || true)
  [[ -z "$bad" ]] || { echo "pages under /nkjv/ lack the retrieval line: $bad"; return 1; }
  # The line is the site's API.Bible credit; the registry's own, which the app
  # prints at the end of the notice, is dropped from the site's copy of it.
  bad=$(grep -rl --include='*.html' -F 'Text provided via API.Bible' "$out" | head -3 || true)
  [[ -z "$bad" ]] || { echo "pages carry the registry's API.Bible credit as well as the retrieval line: $bad"; return 1; }
  date=$(grep -o '<time datetime="[0-9-]*">' "$page" | head -1 | sed 's/.*datetime="\([0-9-]*\)".*/\1/' || true)
  for d in "$@"; do
    [[ -n "$d" && "$date" == "$d" ]] && dated=true
  done
  $dated || { echo "/nkjv/john/3/ states the text was retrieved on '${date}', not on this run's date ($*)"; return 1; }
  bad=$(find "$out/nkjv" -name '*.html' -print0 |
    xargs -0 -r grep -L "<time datetime=\"$date\">" | head -3 || true)
  [[ -z "$bad" ]] || { echo "pages under /nkjv/ state a different retrieval date from /nkjv/john/3/: $bad"; return 1; }
  # And the whole line, as retrievedLineFormat writes it: the date in words
  # inside the <time>, and the line closed after it, in its own paragraph.
  local months=(January February March April May June July August September October November December) words m
  m=${date#*-}; m=${m%-*}
  words="$((10#${date##*-})) ${months[$((10#$m - 1))]} ${date%%-*}"
  bad=$(find "$out/nkjv" -name '*.html' -print0 |
    xargs -0 -r grep -L -F "<p class=\"retrieved\">Text provided by API.Bible (<a href=\"https://api.bible\">api.bible</a>), retrieved <time datetime=\"$date\">$words</time>.</p>" |
    head -3 || true)
  [[ -z "$bad" ]] || { echo "pages under /nkjv/ do not carry the retrieval line as it is written, dated $words: $bad"; return 1; }
  bad=$(grep -rl --include='*.html' 'class="retrieved"' "$out" | grep -v "^$out/nkjv/" | head -3 || true)
  [[ -z "$bad" ]] || { echo "pages outside /nkjv/ carry the NKJV's retrieval line: $bad"; return 1; }

  # Its stylesheet, linked, with every face it names in the tree.
  css=$(find "$out/assets" -name 'nkjv.*.css' | head -1)
  [[ -s "$css" ]] || { echo "the NKJV's stylesheet is missing"; return 1; }
  grep -q "assets/$(basename "$css")" "$page" ||
    { echo "/nkjv/john/3/ does not link $(basename "$css") — the build linked an asset it did not write"; return 1; }
  for f in $(grep -o 'url([^)]*)' "$css" | sed 's/^url(\(.*\))$/\1/'); do
    [[ -s "$out/assets/$f" ]] || { echo "$(basename "$css") names $f, which is not in the tree"; return 1; }
  done
  return 0
}

site_guard_glyphs() {
  local out="$1" report status=0
  report=$(python3 scripts/check-site-glyphs.py "$out" 2>&1) || status=$?
  case "$status" in
    0) return 0 ;;
    1)
      echo "pages set characters no face they declare carries, or in a cut the browser must fake:"
      echo "$report"
      return 1
      ;;
    *)
      echo "the glyph guard could not judge the tree, so it refuses it:"
      echo "$report"
      return 1
      ;;
  esac
}

arm_site_cleanup() {
  NKJV_TEXT=""
  LIVE_TREE=""
  PAGES_CHECKOUT=false
  # bash runs an EXIT trap when an interrupt, TERM or HUP ends the script too,
  # with the signal's status (test-site-nkjv-guards.sh proves INT and TERM).
  trap site_cleanup EXIT
}

site_cleanup() {
  if [[ -n "$LIVE_TREE" ]]; then
    rm -rf "$LIVE_TREE"
  fi
  if $PAGES_CHECKOUT; then
    # rm -rf alone leaves git's registration behind, so a single crashed run
    # would block every future publish with "already exists". Remove, then prune.
    git worktree remove --force "$WORKTREE" 2>/dev/null || true
    rm -rf "$WORKTREE"
    git worktree prune
  fi
  if [[ "$NKJV_TEXT" == on ]]; then
    rm -rf "$OUT"
    echo "==> removed $OUT: it held the NKJV's text"
  fi
}
