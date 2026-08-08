#!/usr/bin/env bash
# Publish bibletext.co.uk — the landing/privacy/support pages AND the web reader
# — to the gh-pages branch, in ONE atomic tree write.
#
# READ THIS BEFORE CHANGING ANYTHING HERE.
#
# The live site is published from the ROOT of the gh-pages branch. Two things on
# that branch are load-bearing and a bad push takes the whole site down:
#
#   CNAME      holds the custom domain. Lose it and bibletext.co.uk detaches —
#              every shared verse link, plus the App Store privacy and support
#              URLs, dies at once.
#   .nojekyll  turns off Jekyll. Without it GitHub rebuilds ~3,900 files through
#              Jekyll on every push, slowly and for no reason.
#
# This script is now the ONLY publisher. Before it existed, the landing pages
# were hand-copied onto gh-pages; doing that again would delete the reader (and
# a reader publish would delete the landing pages), because each would write a
# tree that lacks the other's files. Copy nothing by hand — run this.
#
#   scripts/publish-site.sh            build, verify, and push
#   scripts/publish-site.sh --dry-run  build and verify only; push nothing
#
set -euo pipefail

cd "$(dirname "$0")/.."
DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

OUT=build/site
WORKTREE=build/gh-pages
DOMAIN=bibletext.co.uk

echo "==> generating the reader"
go run ./cmd/websitegen -out "$OUT"

# --- Verify the BUILD before it goes anywhere near the live branch -----------
# A truncated or half-generated site must never reach the branch. Counts are
# per version so adding or removing a translation forces a deliberate edit here,
# rather than silently sliding under one global threshold.
echo "==> verifying the build"
fail() { echo "PUBLISH ABORTED: $*" >&2; exit 1; }

for spec in "web:1189" "bsb:1189" "webc:1300"; do
  id="${spec%%:*}"; min="${spec##*:}"
  got=$(find "$OUT/read/$id" -name index.html -path '*/[0-9]*/*' | wc -l | tr -d ' ')
  [[ "$got" -ge "$min" ]] || fail "$id has only $got chapter pages (expected >= $min) — generation looks truncated"
  echo "    $id: $got chapter pages"
done
[[ -s "$OUT/read/web/john/3/index.html" ]] || fail "smoke page read/web/john/3/ is missing"
grep -q 'id="v16"' "$OUT/read/web/john/3/index.html" || fail "John 3 has no verse anchors — deep links would not highlight"
[[ -s "$OUT/read/assets/reader.css" ]] || fail "stylesheet missing"
[[ -s "$OUT/404.html" ]] || fail "404.html missing"

# --- Assemble the FULL tree (reader + the hand-written pages) ----------------
echo "==> assembling the site tree"
for page in index.html privacy.html support.html; do
  [[ -s "docs/$page" ]] || fail "docs/$page is missing or empty — it is the source of truth for the live site"
  cp "docs/$page" "$OUT/$page"
done
printf '%s\n' "$DOMAIN" > "$OUT/CNAME"
touch "$OUT/.nojekyll"

# --- Final gate: never push a tree that would break the domain or the pages --
[[ "$(cat "$OUT/CNAME")" == "$DOMAIN" ]] || fail "CNAME is not $DOMAIN"
[[ -f "$OUT/.nojekyll" ]] || fail ".nojekyll missing"
for page in index.html privacy.html support.html; do
  [[ -s "$OUT/$page" ]] || fail "$page missing from the tree about to be published"
done
echo "    CNAME, .nojekyll and all three root pages present"

if $DRY_RUN; then
  echo "==> --dry-run: built and verified $OUT; nothing pushed"
  exit 0
fi

# --- Publish -----------------------------------------------------------------
# rsync --delete makes the branch mirror the built tree exactly, which is why
# every required file is asserted above: this step removes anything not present
# in $OUT.
echo "==> publishing to gh-pages"
git fetch origin gh-pages --quiet
rm -rf "$WORKTREE"
git worktree add --quiet "$WORKTREE" origin/gh-pages
trap 'git worktree remove --force "$WORKTREE" 2>/dev/null || true' EXIT

rsync -a --delete --exclude '.git' "$OUT"/ "$WORKTREE"/
git -C "$WORKTREE" add -A
if git -C "$WORKTREE" diff --cached --quiet; then
  echo "==> no changes to publish"
  exit 0
fi
# Last line of defence: refuse a commit that stages a deletion of the domain
# file or one of the hand-written pages.
if git -C "$WORKTREE" diff --cached --name-status | grep -E '^D\s+(CNAME|index\.html|privacy\.html|support\.html)$'; then
  fail "this commit would delete a load-bearing root file"
fi
git -C "$WORKTREE" commit --quiet -m "Publish site: landing pages + web reader ($(date -u +%Y-%m-%d))"
git -C "$WORKTREE" push --quiet origin HEAD:gh-pages
echo "==> published. https://$DOMAIN/read/web/john/3/ should be live within a minute."
