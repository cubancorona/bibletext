#!/usr/bin/env bash
# Build the two reading-face supplements the web reader's NKJV pages load, into
# assets/fonts/reading/web/, from the reading faces this repository already
# carries (assets/fonts/reading/Junicode-*.ttf, built by
# build-reading-fonts.sh).
#
#   Junicode-SmallCaps.woff2  the Unicode small capitals the app draws the
#                             divine name with (smallCapitals,
#                             small_caps_draw.go) and nothing else. The web
#                             subset the public-domain pages load carries none
#                             of them, because no edition the site publishes
#                             without the NKJV marks a divine name. The list is
#                             read from the Go table here, so the face and the
#                             app cannot disagree about which letters exist.
#   Junicode-Italic.woff2     the italic cut over the web subset's own ranges,
#                             for the words the translators supplied and the
#                             psalm titles, which the NKJV pages set in italic
#                             and would otherwise get as a slanted regular.
#
# Only the pages under /nkjv/ link them (cmd/websitegen/nkjv_assets.go), and
# only while the NKJV's text is published (cmd/websitegen/nkjv_text.go). The
# Regular and Bold web subsets are NOT rebuilt here and must not change: their
# bytes are hashed into every page of the public-domain editions.
#
# Requires fontTools and brotli:  pip3 install --user fonttools brotli
set -euo pipefail
cd "$(dirname "$0")/.."

PYFTSUBSET="${PYFTSUBSET:-$HOME/Library/Python/3.9/bin/pyftsubset}"
[ -x "$PYFTSUBSET" ] || PYFTSUBSET="$(command -v pyftsubset || true)"
if [ -z "$PYFTSUBSET" ] || [ ! -x "$PYFTSUBSET" ]; then
  echo "pyftsubset not found. pip3 install --user fonttools brotli" >&2; exit 1
fi

# The small capitals, as U+XXXX, straight from the Go table.
SMALL_CAPS=$(python3 - <<'PY'
import re, sys
src = open("small_caps_draw.go", encoding="utf-8").read()
block = re.search(r"var smallCapitals = map\[rune\]rune\{(.*?)\n\}", src, re.S)
if not block:
    sys.exit("smallCapitals table not found in small_caps_draw.go")
pairs = re.findall(r"'([a-z])':\s*'(.)'", block.group(1))
if len(pairs) != 25:
    sys.exit(f"expected 25 small capitals in small_caps_draw.go, found {len(pairs)}")
print(",".join(f"U+{ord(sc):04X}" for _, sc in sorted(pairs)))
PY
)

# The same ranges and features as the Regular and Bold web subsets
# (build-reading-fonts.sh, WEB_RANGES and WEB_FEATURES).
WEB_RANGES='U+0020-007E,U+00A0-00FF,U+0100-017F,U+2013-2014,U+2018-201D,U+2026,U+00B2,U+00B3,U+00B9,U+2070,U+2074-2079'
WEB_FEATURES='kern,liga,calt,onum,ccmp,locl'

mkdir -p assets/fonts/reading/web
"$PYFTSUBSET" assets/fonts/reading/Junicode-Regular.ttf --flavor=woff2 \
  --output-file=assets/fonts/reading/web/Junicode-SmallCaps.woff2 \
  --unicodes="$SMALL_CAPS" --layout-features+="$WEB_FEATURES" --no-hinting --desubroutinize
"$PYFTSUBSET" assets/fonts/reading/Junicode-Italic.ttf --flavor=woff2 \
  --output-file=assets/fonts/reading/web/Junicode-Italic.woff2 \
  --unicodes="$WEB_RANGES" --layout-features+="$WEB_FEATURES" --no-hinting --desubroutinize
for f in SmallCaps Italic; do
  printf '  %-50s %5s KB\n' "assets/fonts/reading/web/Junicode-$f.woff2" \
    "$(( $(wc -c < "assets/fonts/reading/web/Junicode-$f.woff2") / 1024 ))"
done

echo
echo "Verifying the supplements kept what the pages draw:"
SMALL_CAPS="$SMALL_CAPS" python3 - <<'PY'
import os, sys
from fontTools.ttLib import TTFont
want = [int(u[2:], 16) for u in os.environ["SMALL_CAPS"].split(",")]
bad = False
f = TTFont("assets/fonts/reading/web/Junicode-SmallCaps.woff2", lazy=True)
cm = set(f.getBestCmap()); f.close()
missing = [hex(c) for c in want if c not in cm]
print(f"  Junicode-SmallCaps  small capitals {len(want)-len(missing)}/{len(want)}  "
      f"{'OK' if not missing else 'INCOMPLETE ' + ','.join(missing)}")
bad = bad or bool(missing)
f = TTFont("assets/fonts/reading/web/Junicode-Italic.woff2", lazy=True)
cm = set(f.getBestCmap()); f.close()
basic = [c for c in range(0x20, 0x7F)] + [0x2018, 0x2019, 0x201C, 0x201D, 0x2014]
missing = [hex(c) for c in basic if c not in cm]
print(f"  Junicode-Italic     Latin and punctuation {len(basic)-len(missing)}/{len(basic)}  "
      f"{'OK' if not missing else 'INCOMPLETE ' + ','.join(missing)}")
bad = bad or bool(missing)
sys.exit(1 if bad else 0)
PY
