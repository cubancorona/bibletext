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
#   .nojekyll  turns off Jekyll. Without it GitHub rebuilds ~5,500 files through
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
# The custom domain derives from the identity file rather than being repeated
# here. The extraction fails the whole publish (set -e) on a malformed file —
# a wrong or empty CNAME detaches the domain and kills every shared link.
DOMAIN=$(python3 -c '
import json, sys
base = json.load(open("config/product.json"))["siteBase"]
if not base.startswith("https://") or len(base) <= len("https://"):
    sys.exit("siteBase in config/product.json is not an https origin")
print(base.removeprefix("https://"), end="")
')
[[ -n "$DOMAIN" ]] || { echo "could not derive DOMAIN from config/product.json" >&2; exit 1; }

echo "==> checking the public support configuration"
python3 scripts/check-support-contact.py
echo "==> generating the reader"
go run ./cmd/websitegen -out "$OUT"
echo "==> rendering the project pages"
go run ./cmd/sitepages -source docs -out "$OUT"

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
#
# THE NUMBERS MOVED when the notice pages landed (cmd/websitegen/notice.go), and
# a fourth tree joined the list. Reading them:
#
#   web  1328  1189 chapters of scripture + 139 canon-gap notices (the seven
#   bsb  1328  deuterocanonical books' 137 chapters, plus Daniel 13 and 14,
#              which the Greek Daniel has and these editions do not)
#   webc 1328  all scripture — WEB Catholic is the widest canon, so it has no
#              gaps and its total is what the other two now match
#   nkjv 1189  all notices. Every chapter the app can build a /nkjv/ share link
#              for, and nothing else. This tree carries NO text, and the check
#              below proves that as well as its size — a fourth tree slipping
#              in unverified is exactly what this guard is for.
for spec in "web:1328" "bsb:1328" "webc:1328" "nkjv:1189"; do
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

# The notice pages, both placements. The licensed one gets the strongest check
# this script can make: it must name the translation, offer the app AND the
# parallel passage, and carry NO verse markup. paragraphBody is the only thing
# that writes a verse anchor or a red-letter span, so either appearing under
# /nkjv/ means scripture reached a tree that must never hold any.
[[ -s "$OUT/nkjv/john/3/index.html" ]] || fail "/nkjv/john/3/ is missing — NKJV share links would 404 again"
grep -q 'New King James Version' "$OUT/nkjv/john/3/index.html" || fail "/nkjv/john/3/ does not name the translation"
grep -q 'id="openapp"' "$OUT/nkjv/john/3/index.html" || fail "/nkjv/john/3/ has no open-in-app affordance"
grep -q 'href="../../../web/john/3/"' "$OUT/nkjv/john/3/index.html" || fail "/nkjv/john/3/ offers no parallel passage"
leak=$(find "$OUT/nkjv" -name index.html -print0 |
  xargs -0 grep -lE 'class="v" id="v|class="wj"|<sup class="n"' | head -3 || true)
[[ -z "$leak" ]] || fail "pages under /nkjv/ carry verse markup — LICENSED TEXT IS ABOUT TO BE PUBLISHED: $leak"
[[ -s "$OUT/web/tobit/1/index.html" ]] || fail "/web/tobit/1/ is missing — the deuterocanonical gap is a 404 again"
[[ -s "$OUT/web/daniel/13/index.html" ]] || fail "/web/daniel/13/ is missing — the Greek Daniel gap is a 404 again"
# Assets are content-hashed (reader.<hash>.css), so this checks the pair exists
# AND that a real page links the exact file that was built. A page pointing at a
# stylesheet that isn't in the tree is the failure this naming scheme exists to
# prevent, and it renders as an unstyled page rather than an error.
css=$(find "$OUT/assets" -name 'reader.*.css' | head -1)
js=$(find "$OUT/assets" -name 'reader.*.js' | head -1)
[[ -s "$css" ]] || fail "stylesheet missing"
[[ -s "$js" ]] || fail "script missing"
for asset in "$css" "$js"; do
  grep -q "assets/$(basename "$asset")" "$OUT/web/john/3/index.html" ||
    fail "pages do not reference $(basename "$asset") — the build linked an asset it did not write"
done
# The notice pages carry their OWN hashed pair, on top of the reader's, so that
# adding a rule for them can never rewrite the 3,906 pages that carry scripture.
# Same check, and one more: no page that carries scripture may request them.
ncss=$(find "$OUT/assets" -name 'notice.*.css' | head -1)
njs=$(find "$OUT/assets" -name 'notice.*.js' | head -1)
[[ -s "$ncss" ]] || fail "notice stylesheet missing"
[[ -s "$njs" ]] || fail "notice script missing"
for asset in "$ncss" "$njs"; do
  grep -q "assets/$(basename "$asset")" "$OUT/nkjv/john/3/index.html" ||
    fail "notice pages do not reference $(basename "$asset") — the build linked an asset it did not write"
  if grep -q "assets/$(basename "$asset")" "$OUT/web/john/3/index.html"; then
    fail "a scripture page links $(basename "$asset") — the reader's assets must stay untouched by it"
  fi
done
[[ -s "$OUT/404.html" ]] || fail "404.html missing"

# --- Assemble the FULL tree (reader + the hand-written page templates) -------
# Publishing from any dirty generator, renderer, template, or configuration
# would make the live site differ from every known revision. Require the whole
# tracked/untracked source tree to be clean; ignored build output is unaffected.
echo "==> checking the repo state"
branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$branch" != "main" && "${ALLOW_BRANCH:-0}" != "1" ]]; then
  fail "on branch '$branch', not main. The live site should be published from main; set ALLOW_BRANCH=1 to override deliberately."
fi
if [[ -n "$(git status --porcelain --untracked-files=all)" ]]; then
  fail "the repository has uncommitted changes — commit them first so the live site matches a known revision"
fi

echo "==> assembling the site tree"
for page in index.html privacy.html support.html; do
  [[ -s "docs/$page" ]] || fail "docs/$page is missing or empty — it is the source template for the live site"
  [[ -s "$OUT/$page" ]] || fail "$OUT/$page is missing or empty — project-page rendering failed"
done
printf '%s\n' "$DOMAIN" > "$OUT/CNAME"
touch "$OUT/.nojekyll"

# App-association files, so a shared verse link opens the APP on a phone that
# has it. Both are published from docs/ (the source of truth) into the tree
# rsync mirrors — a file added to gh-pages by hand would be deleted by the next
# publish. The Apple file is also copied to the legacy root location, which is
# still supported and costs nothing.
mkdir -p "$OUT/.well-known"
cp docs/apple-app-site-association "$OUT/.well-known/apple-app-site-association"
cp docs/apple-app-site-association "$OUT/apple-app-site-association"
cp docs/assetlinks.json "$OUT/.well-known/assetlinks.json"

# The favicon serves the whole site from the root: browsers request
# /favicon.ico by default, so the ~5,500 reader pages get it without carrying a
# link tag; the three hand-written pages also link it (plus the touch icon)
# explicitly.
cp docs/favicon.ico "$OUT/favicon.ico"
cp docs/apple-touch-icon.png "$OUT/apple-touch-icon.png"

# --- Final gate: never push a tree that would break the domain or the pages --
[[ "$(cat "$OUT/CNAME")" == "$DOMAIN" ]] || fail "CNAME is not $DOMAIN"
[[ -f "$OUT/.nojekyll" ]] || fail ".nojekyll missing"
for page in index.html privacy.html support.html; do
  [[ -s "$OUT/$page" ]] || fail "$page missing from the tree about to be published"
done
# The favicon must be a real ICO (it starts 00 00 01 00), not a stray PNG
# renamed — a PNG at /favicon.ico renders in browsers but not everywhere the
# .ico contract is assumed.
[[ -s "$OUT/favicon.ico" ]] || fail "favicon.ico missing from the tree about to be published"
# Whitespace-squeezed comparison: BSD od pads bytes with double spaces, which a
# spaced grep pattern silently never matches — the gate then fails a VALID ico.
[[ "$(head -c4 "$OUT/favicon.ico" | od -An -tx1 | tr -d ' \n')" == "00000100" ]] ||
  fail "favicon.ico is not an ICO file"
grep -q 'rel="icon"' "$OUT/index.html" || fail "index.html does not link the favicon"
support_email=$(python3 -c 'import json; print(json.load(open("config/product.json"))["supportEmail"], end="")')
for page in privacy.html support.html; do
  grep -Fq "$support_email" "$OUT/$page" || fail "$page does not contain the configured support address"
  grep -Fq "href=\"mailto:$support_email\"" "$OUT/$page" || \
    fail "$page does not contain the configured mailto recipient"
  if grep -Fq '{{BIBLETEXT_SUPPORT_EMAIL_' "$OUT/$page"; then
    fail "$page contains an unresolved support-address marker"
  fi
done
unset support_email
# The association files decide whether a tapped link opens the app, and a
# malformed one fails SILENTLY and slowly (Apple caches per-domain for ~24h,
# and a 404 is cached as a negative result). Validate before pushing.
for f in ".well-known/apple-app-site-association" ".well-known/assetlinks.json"; do
  [[ -s "$OUT/$f" ]] || fail "$f missing — shared links would stop opening the app"
  python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$OUT/$f" \
    || fail "$f is not valid JSON — shared links would stop opening the app"
done
grep -q 'R8PC7239T2.uk.co.bibletext' "$OUT/.well-known/apple-app-site-association" \
  || fail "the Apple association file does not name the app id"
# The scope is an ALLOW-LIST on purpose: privacy.html and support.html are the
# URLs App Store Connect points at and Apple's reviewer opens. If the app ever
# claimed them, tapping them would bounce into the app instead of the browser.
grep -q '"/privacy.html", "exclude": true' "$OUT/.well-known/apple-app-site-association" \
  || fail "the Apple association file no longer excludes privacy.html"
grep -q '"/support.html", "exclude": true' "$OUT/.well-known/apple-app-site-association" \
  || fail "the Apple association file no longer excludes support.html"
echo "    CNAME, .nojekyll, both association files and all three root pages present"

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
if git -C "$WORKTREE" diff --cached --name-status | grep -E '^D\s+(CNAME|index\.html|privacy\.html|support\.html|\.well-known/.*)$'; then
  fail "this commit would delete a load-bearing root file"
fi
git -C "$WORKTREE" commit --quiet -m "Publish site: landing pages + web reader ($(date -u +%Y-%m-%d))"
git -C "$WORKTREE" push --quiet origin HEAD:gh-pages
echo "==> published. https://$DOMAIN/web/john/3/ should be live within a minute."
