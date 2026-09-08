#!/usr/bin/env bash
# Build the WOFF2 faces the generated website serves, from the SAME TTFs the
# app embeds, so the site and the app can never drift to different releases of
# a face.
#
# Why this script exists: the first pair of web faces was built by hand and the
# command was never recorded, so nobody could rebuild or audit them. Everything
# that decides what comes out is spelled out here.
#
# TWO FLAGS DECIDE WHETHER THE RESULT IS CORRECT, and getting either wrong
# fails silently — the font loads, the text renders, and a feature the
# stylesheet asks for simply does nothing:
#
#   --layout-features   pyftsubset keeps only a DEFAULT feature set, which does
#                       NOT include onum. The reading column's old-style verse
#                       numbers come from onum, so a subset built without this
#                       flag ships lining figures and looks like a CSS bug.
#                       smcp and c2sc are kept for the same reason: they are
#                       what sets small capitals as real glyphs.
#   --unicodes          The scripture face needs 69 codepoints for verse text
#                       across all three published editions (five of them
#                       non-ASCII: an em dash and the four curly quotes). The
#                       range below is deliberately wider, so a future edition
#                       that uses an accented name does not fall back to
#                       another face for one letter in the middle of a word.
#
# Requires fontTools and brotli:  pip3 install --user fonttools brotli
set -euo pipefail

cd "$(dirname "$0")/.."

PYFTSUBSET="${PYFTSUBSET:-$HOME/Library/Python/3.9/bin/pyftsubset}"
if [ ! -x "$PYFTSUBSET" ]; then
  PYFTSUBSET="$(command -v pyftsubset || true)"
fi
if [ -z "$PYFTSUBSET" ] || [ ! -x "$PYFTSUBSET" ]; then
  echo "pyftsubset not found. Install it with: pip3 install --user fonttools brotli" >&2
  exit 1
fi

UNICODES='U+0020-007E,U+00A0-00FF,U+0100-017F,U+2010-2015,U+2018-201D,U+2026,U+2039,U+203A'
FEATURES='kern,liga,calt,onum,smcp,c2sc,ccmp,locl'

build() {
  local src="$1" out="$2"
  mkdir -p "$(dirname "$out")"
  "$PYFTSUBSET" "$src" \
    --flavor=woff2 \
    --output-file="$out" \
    --unicodes="$UNICODES" \
    --layout-features+="$FEATURES" \
    --no-hinting --desubroutinize
  printf '  %-52s %6s KB\n' "$out" "$(( $(wc -c < "$out") / 1024 ))"
}

echo "Scripture face (Spectral) — the reading column:"
build assets/fonts/share/Spectral-Regular.ttf assets/fonts/share/web/Spectral-Regular.woff2
build assets/fonts/share/Spectral-Bold.ttf    assets/fonts/share/web/Spectral-Bold.woff2

echo
echo "Verifying the features survived the subset:"
python3 - <<'PY'
from fontTools.ttLib import TTFont
import sys
want = {"onum", "smcp", "c2sc", "kern", "liga"}
bad = False
for p in ("assets/fonts/share/web/Spectral-Regular.woff2",
          "assets/fonts/share/web/Spectral-Bold.woff2"):
    f = TTFont(p)
    have = set()
    for tag in ("GSUB", "GPOS"):
        if tag in f:
            have |= {r.FeatureTag for r in f[tag].table.FeatureList.FeatureRecord}
    missing = want - have
    print(f"  {p}: {'OK' if not missing else 'MISSING ' + ','.join(sorted(missing))}")
    bad = bad or bool(missing)
    f.close()

# The rule in small_caps.go depends on HOW THIS FACE implements the two
# features, not on what the OpenType spec says they mean. Spectral's c2sc
# covers lower case as well as capitals, so asking for both on a mixed-case
# span would set the divine name's initial as a small capital too. If a future
# face is dropped in that behaves differently, that rule is wrong and this
# fails rather than letting it ship silently.
print()
print("Checking the assumption smallCapsFeatures() is built on:")
f = TTFont("assets/fonts/share/Spectral-Regular.ttf")
gsub = f["GSUB"].table


def mapping(tag):
    out = {}
    for rec in gsub.FeatureList.FeatureRecord:
        if rec.FeatureTag != tag:
            continue
        for li in rec.Feature.LookupListIndex:
            for st in gsub.LookupList.Lookup[li].SubTable:
                out.update(getattr(st, "mapping", {}) or {})
    return out


smcp, c2sc = mapping("smcp"), mapping("c2sc")
one = lambda m, pred: sum(1 for g in m if len(g) == 1 and pred(g))
checks = [
    ("smcp maps lower case", one(smcp, str.islower) == 26),
    ("smcp leaves capitals alone", one(smcp, str.isupper) == 0),
    ("c2sc maps capitals", one(c2sc, str.isupper) == 26),
    ("c2sc ALSO maps lower case (why both is wrong)", one(c2sc, str.islower) == 26),
]
for label, ok in checks:
    print(f"  {'OK  ' if ok else 'FAIL'} {label}")
    bad = bad or not ok
f.close()

sys.exit(1 if bad else 0)
PY
