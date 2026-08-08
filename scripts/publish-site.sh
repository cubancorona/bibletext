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
# The reader lives at the ROOT (/web/, /bsb/, /webc/), sharing the namespace with
# those hand-written pages — which is why this script writes the whole tree at
# once and why the generator refuses to emit their filenames.
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

# EXACT counts at chapter depth. -mindepth/-maxdepth 3 is load-bearing: a
# -path '*/[0-9]*/*' glob crosses slashes, so it also matches every
# digit-prefixed BOOK index (read/web/1-samuel/index.html) and silently
# inflated the total by 17-19, leaving that many chapters of slack in the one
# guard standing between a truncated build and rsync --delete.
# Equality, not a floor: adding or removing a book must be a deliberate edit
# here, which is exactly what this block is for.
for spec in "web:1189" "bsb:1189" "webc:1328"; do
  id="${spec%%:*}"; want="${spec##*:}"
  got=$(find "$OUT/$id" -mindepth 3 -maxdepth 3 -name index.html | wc -l | tr -d ' ')
  [[ "$got" -eq "$want" ]] || fail "$id has $got chapter pages, expected exactly $want — generation looks truncated (or a book was added: update this list deliberately)"
  echo "    $id: $got chapter pages"
done
# A webc-only page: the Catholic decoder skips unrecognised USFM ids silently,
# so a helloao id change could drop whole deuterocanonical books while the
# counts above still looked plausible.
[[ -s "$OUT/webc/daniel/13/index.html" ]] || fail "webc/daniel/13 missing — the Catholic decode looks incomplete"
[[ -s "$OUT/web/john/3/index.html" ]] || fail "smoke page /web/john/3/ is missing"
grep -q 'id="v16"' "$OUT/web/john/3/index.html" || fail "John 3 has no verse anchors — deep links would not highlight"
[[ -s "$OUT/assets/reader.css" ]] || fail "stylesheet missing"
[[ -s "$OUT/404.html" ]] || fail "404.html missing"

# --- Assemble the FULL tree (reader + the hand-written pages) ----------------
# The three root pages are copied from the WORKING TREE, so publishing from a
# dirty or unexpected checkout would push whatever happens to be sitting there
# to the live site — including someone's half-finished privacy-policy edit.
echo "==> checking the repo state"
branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$branch" != "main" && "${ALLOW_BRANCH:-0}" != "1" ]]; then
  fail "on branch '$branch', not main. The live site should be published from main; set ALLOW_BRANCH=1 to override deliberately."
fi
if [[ -n "$(git status --porcelain -- docs/)" ]]; then
  fail "docs/ has uncommitted changes — commit them first so the live site matches a known revision"
fi

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
# rm -rf alone leaves git's registration behind, so a single crashed run would
# block every future publish with "already exists". Remove, then prune.
git worktree remove --force "$WORKTREE" 2>/dev/null || true
rm -rf "$WORKTREE"
git worktree prune
# Armed BEFORE the add so an interrupt mid-checkout still cleans up.
trap 'git worktree remove --force "$WORKTREE" 2>/dev/null || true; rm -rf "$WORKTREE"; git worktree prune' EXIT
git worktree add --quiet "$WORKTREE" origin/gh-pages

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
echo "==> published. https://$DOMAIN/web/john/3/ should be live within a minute."
