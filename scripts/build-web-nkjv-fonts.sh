#!/usr/bin/env bash
# Build the reading-face supplements the web reader loads on top of the two
# public-domain subsets, into assets/fonts/reading/web/, from the faces this
# repository already carries (assets/fonts/reading/*.ttf, built by
# build-reading-fonts.sh). Named for the NKJV, whose pages needed the first of
# them; the italic, the Greek and the Hebrew serve every edition now.
#
#   Junicode-SmallCaps.woff2        the Unicode small capitals the app draws the
#   Junicode-BoldSmallCaps.woff2    divine name with (smallCapitals,
#   Junicode-ItalicSmallCaps.woff2  small_caps_draw.go) and nothing else, from
#                                   the regular, bold and italic cuts: the
#                                   name in a verse, in a section heading (set
#                                   bold) and in supplied words and psalm
#                                   titles (set italic). A browser matches a
#                                   bold or italic run against faces of that
#                                   weight and style only, so each cut needs
#                                   its own. The web subsets the public-domain
#                                   pages load carry none of them, because no
#                                   edition the site publishes without the NKJV
#                                   marks a divine name. The list is read from
#                                   the Go table here, so the faces and the app
#                                   cannot disagree about which letters exist.
#   Junicode-Italic.woff2           the italic cut over the web subset's own
#                                   ranges, for the psalm titles of every
#                                   edition and the NKJV's supplied words,
#                                   which would otherwise get a slanted regular.
#   Junicode-Greek.woff2            the regular cut over exactly the Greek the
#                                   site's pages draw (webGreekRunes,
#                                   web_fonts.go, read here): the WEB's notes.
#   BibleTextHebrew.woff2           the app's Hebrew face, Ezra SIL, subsetted
#                                   to exactly the Hebrew the site's pages draw
#                                   (webHebrewRunes, web_fonts.go, read here),
#                                   with its layout tables whole so the marks
#                                   still attach. "Ezra" and "SIL" are Reserved
#                                   Font Names and a subset is a Modified
#                                   Version, so it is RENAMED: every name
#                                   record that names the font says "BibleText
#                                   Hebrew", and the copyright and licence
#                                   records are kept as they are.
#
# The stylesheets that load them say where (cmd/websitegen): reader.css the
# italic, the Greek and the Hebrew, nkjv.css the small capitals. Each is a file
# of its own with a unicode-range or a style of its own, so a page downloads
# only what it draws. The Regular and Bold web subsets are NOT rebuilt here.
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
for pair in Regular:SmallCaps Bold:BoldSmallCaps Italic:ItalicSmallCaps; do
  "$PYFTSUBSET" "assets/fonts/reading/Junicode-${pair%%:*}.ttf" --flavor=woff2 \
    --output-file="assets/fonts/reading/web/Junicode-${pair##*:}.woff2" \
    --unicodes="$SMALL_CAPS" --layout-features+="$WEB_FEATURES" --no-hinting --desubroutinize
done
"$PYFTSUBSET" assets/fonts/reading/Junicode-Italic.ttf --flavor=woff2 \
  --output-file=assets/fonts/reading/web/Junicode-Italic.woff2 \
  --unicodes="$WEB_RANGES" --layout-features+="$WEB_FEATURES" --no-hinting --desubroutinize

# The Greek, as U+XXXX, from the Go list.
GREEK=$(python3 - <<'PY'
import re, sys
src = open("web_fonts.go", encoding="utf-8").read()
block = re.search(r"var webGreekRunes = \[\]rune\{(.*?)\n\}", src, re.S)
if not block:
    sys.exit("webGreekRunes not found in web_fonts.go")
print(",".join("U+" + h.upper() for h in re.findall(r"'\\u([0-9A-Fa-f]{4})'", block.group(1))))
PY
)
"$PYFTSUBSET" assets/fonts/reading/Junicode-Regular.ttf --flavor=woff2 \
  --output-file=assets/fonts/reading/web/Junicode-Greek.woff2 \
  --unicodes="$GREEK" --layout-features+="$WEB_FEATURES" --no-hinting --desubroutinize

# The Hebrew, through the fontTools API rather than pyftsubset: the subset is
# renamed before it is written. Every name record is kept (pyftsubset's default
# drops the licence's), then the ones that NAME the font are replaced. The
# timestamp is not touched, so the same inputs give the same bytes.
python3 - <<'PY'
import re, sys
from fontTools import subset
from fontTools.ttLib import TTFont
src = open("web_fonts.go", encoding="utf-8").read()
block = re.search(r"var webHebrewRunes = \[\]rune\{(.*?)\n\}", src, re.S)
if not block:
    sys.exit("webHebrewRunes not found in web_fonts.go")
runes = [int(h, 16) for h in re.findall(r"'\\u([0-9A-Fa-f]{4})'", block.group(1))]
if not runes or len(set(runes)) != len(runes):
    sys.exit("webHebrewRunes is empty or repeats a code point")
opts = subset.Options()
opts.layout_features = ["*"]
opts.name_IDs = ["*"]
opts.name_languages = ["*"]
opts.name_legacy = True
opts.hinting = False
opts.notdef_outline = True
font = TTFont("assets/fonts/reading/EzraSIL-Regular.ttf", recalcTimestamp=False)
sub = subset.Subsetter(opts)
sub.populate(unicodes=runes)
sub.subset(font)
FAMILY, POSTSCRIPT = "BibleText Hebrew", "BibleTextHebrew"
RENAMED = {1: FAMILY, 3: FAMILY + " 2.51 web subset", 4: FAMILY, 6: POSTSCRIPT, 16: FAMILY, 18: FAMILY}
for rec in font["name"].names:
    if rec.nameID in RENAMED:
        rec.string = RENAMED[rec.nameID]
font.flavor = "woff2"
font.save("assets/fonts/reading/web/BibleTextHebrew.woff2")
PY
for f in Junicode-SmallCaps Junicode-BoldSmallCaps Junicode-ItalicSmallCaps Junicode-Italic Junicode-Greek BibleTextHebrew; do
  printf '  %-50s %5s KB\n' "assets/fonts/reading/web/$f.woff2" \
    "$(( $(wc -c < "assets/fonts/reading/web/$f.woff2") / 1024 ))"
done

echo
echo "Verifying the supplements kept what the pages draw:"
SMALL_CAPS="$SMALL_CAPS" python3 - <<'PY'
import os, re, sys
from fontTools.ttLib import TTFont
want = [int(u[2:], 16) for u in os.environ["SMALL_CAPS"].split(",")]
bad = False
for cut in ("SmallCaps", "BoldSmallCaps", "ItalicSmallCaps"):
    f = TTFont(f"assets/fonts/reading/web/Junicode-{cut}.woff2", lazy=True)
    cm = set(f.getBestCmap()); f.close()
    missing = [hex(c) for c in want if c not in cm]
    print(f"  Junicode-{cut:15} small capitals {len(want)-len(missing)}/{len(want)}  "
          f"{'OK' if not missing else 'INCOMPLETE ' + ','.join(missing)}")
    bad = bad or bool(missing)
# The Hebrew: exactly the list, every mark still attached through GPOS, and no
# name record that names the font carrying a Reserved Font Name.
src = open("web_fonts.go", encoding="utf-8").read()
block = re.search(r"var webHebrewRunes = \[\]rune\{(.*?)\n\}", src, re.S)
heb = {int(h, 16) for h in re.findall(r"'\\u([0-9A-Fa-f]{4})'", block.group(1))}
f = TTFont("assets/fonts/reading/web/BibleTextHebrew.woff2")
cm = set(f.getBestCmap())
marks = {c for c in heb if 0x0591 <= c <= 0x05C7}
gpos = {r.FeatureTag for r in f["GPOS"].table.FeatureList.FeatureRecord} if "GPOS" in f else set()
naming = {r.nameID: r.toUnicode() for r in f["name"].names if r.nameID in (1, 3, 4, 6, 16, 17, 18)}
reserved = sorted({i for i, v in naming.items() if "Ezra" in v or "SIL" in v})
licence = any(r.nameID == 13 and "Open Font License" in r.toUnicode() for r in f["name"].names)
ok = cm == heb and (not marks or "mark" in gpos) and not reserved and licence
print(f"  BibleTextHebrew          code points {len(cm & heb)}/{len(heb)}"
      f"{' +' + str(len(cm - heb)) + ' extra' if cm - heb else ''}  mark attachment "
      f"{'kept' if 'mark' in gpos else 'LOST'}  reserved names {reserved or 'none'}  "
      f"licence {'kept' if licence else 'LOST'}  {'OK' if ok else 'WRONG'}")
f.close()
bad = bad or not ok
block = re.search(r"var webGreekRunes = \[\]rune\{(.*?)\n\}", src, re.S)
greek = {int(h, 16) for h in re.findall(r"'\\u([0-9A-Fa-f]{4})'", block.group(1))}
f = TTFont("assets/fonts/reading/web/Junicode-Greek.woff2", lazy=True)
cm = set(f.getBestCmap()); f.close()
ok = greek <= cm and not {c for c in cm - greek if c > 0x7E}
print(f"  Junicode-Greek           code points {len(cm & greek)}/{len(greek)}  {'OK' if ok else 'WRONG'}")
bad = bad or not ok
f = TTFont("assets/fonts/reading/web/Junicode-Italic.woff2", lazy=True)
cm = set(f.getBestCmap()); f.close()
basic = [c for c in range(0x20, 0x7F)] + [0x2018, 0x2019, 0x201C, 0x201D, 0x2014]
missing = [hex(c) for c in basic if c not in cm]
print(f"  Junicode-Italic     Latin and punctuation {len(basic)-len(missing)}/{len(basic)}  "
      f"{'OK' if not missing else 'INCOMPLETE ' + ','.join(missing)}")
bad = bad or bool(missing)
sys.exit(1 if bad else 0)
PY
