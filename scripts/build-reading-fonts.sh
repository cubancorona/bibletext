#!/usr/bin/env bash
# Build the faces that set Scripture, from upstream releases, into
# assets/fonts/reading/.
#
# TWO FACES, because no single one exists. Sixty-six were measured against what
# the app actually draws: the faces that cover every script have three cuts or
# put their small capitals in the regular alone, and the faces with four
# properly featured cuts have no Hebrew.
#
#   Junicode   Latin and polytonic Greek, four real cuts, and — the reason it
#              wins — the UNICODE SMALL CAPITALS in all four, so the divine
#              name can be set without any OpenType feature control and
#              therefore without a patched toolkit. SIL OFL, and it declares no
#              Reserved Font Name, so this subset keeps the family's name.
#
#   Ezra SIL   pointed Hebrew, as the BHS sets it: every letter, every mark in
#              mark coverage. Shipped UNMODIFIED and unsubsetted. "Ezra" and
#              "SIL" ARE Reserved Font Names, so a modified build would have to
#              be renamed; at 151 KB it is not worth the obligation.
#
# THE SUBSET RANGES ARE LOAD-BEARING. Small capitals are not one block: they are
# scattered across IPA Extensions, Phonetic Extensions and Latin Extended-D, and
# a subset that misses one of those silently loses letters from the divine name.
#
# Requires fontTools and brotli:  pip3 install --user fonttools brotli
set -euo pipefail
cd "$(dirname "$0")/.."

PYFTSUBSET="${PYFTSUBSET:-$HOME/Library/Python/3.9/bin/pyftsubset}"
[ -x "$PYFTSUBSET" ] || PYFTSUBSET="$(command -v pyftsubset || true)"
if [ -z "$PYFTSUBSET" ] || [ ! -x "$PYFTSUBSET" ]; then
  echo "pyftsubset not found. pip3 install --user fonttools brotli" >&2; exit 1
fi

SRC="${1:-}"
if [ -z "$SRC" ] || [ ! -d "$SRC" ]; then
  echo "usage: $0 <dir holding Junicode-*.ttf and EzraSIL-Regular.ttf>" >&2; exit 1
fi

RANGES='U+0020-007E,U+00A0-00FF,U+0100-017F,U+0180-024F,U+0250-02AF,U+0300-036F,U+0370-03FF,U+1D00-1D7F,U+1E00-1EFF,U+1F00-1FFF,U+2000-206F,U+2070-209F,U+20A0-20BF,U+2100-214F,U+A700-A7FF'
FEATURES='kern,liga,calt,clig,onum,lnum,smcp,c2sc,ccmp,mark,mkmk,locl,rlig,sups'

for cut in Regular Italic Bold BoldItalic; do
  "$PYFTSUBSET" "$SRC/Junicode-$cut.ttf" \
    --output-file="assets/fonts/reading/Junicode-$cut.ttf" \
    --unicodes="$RANGES" --layout-features+="$FEATURES" \
    --notdef-outline --no-hinting --desubroutinize
  printf '  %-46s %5s KB\n' "assets/fonts/reading/Junicode-$cut.ttf" \
    "$(( $(wc -c < "assets/fonts/reading/Junicode-$cut.ttf") / 1024 ))"
done

cp "$SRC/EzraSIL-Regular.ttf" assets/fonts/reading/EzraSIL-Regular.ttf
printf '  %-46s %5s KB   (unmodified: Reserved Font Name)\n' \
  "assets/fonts/reading/EzraSIL-Regular.ttf" \
  "$(( $(wc -c < assets/fonts/reading/EzraSIL-Regular.ttf) / 1024 ))"

echo
echo "Verifying the subset kept what the app depends on:"
python3 - <<'PY'
from fontTools.ttLib import TTFont
import sys
SC = {'A':0x1D00,'B':0x0299,'C':0x1D04,'D':0x1D05,'E':0x1D07,'G':0x0262,'H':0x029C,
      'I':0x026A,'J':0x1D0A,'L':0x029F,'M':0x1D0D,'N':0x0274,'O':0x1D0F,'R':0x0280,
      'S':0xA731,'T':0x1D1B,'U':0x1D1C,'Y':0x028F}
GREEKX = [0x1F04,0x1F10,0x1F30,0x1F7A,0x1FB3,0x1FC6,0x1FE5,0x1FE6]
SUPS   = [0x00B9,0x00B2,0x00B3,0x2070]+list(range(0x2074,0x207A))
bad = False
for cut in ("Regular","Italic","Bold","BoldItalic"):
    p = f"assets/fonts/reading/Junicode-{cut}.ttf"
    f = TTFont(p, lazy=True); cm = set(f.getBestCmap()); f.close()
    miss_sc = [k for k,v in SC.items() if v not in cm]
    miss_gx = [hex(c) for c in GREEKX if c not in cm]
    miss_su = [hex(c) for c in SUPS if c not in cm]
    ok = not (miss_sc or miss_gx or miss_su)
    print(f"  Junicode-{cut:11} small caps {len(SC)-len(miss_sc)}/{len(SC)}  "
          f"Greek Ext {len(GREEKX)-len(miss_gx)}/{len(GREEKX)}  "
          f"superiors {len(SUPS)-len(miss_su)}/{len(SUPS)}  {'OK' if ok else 'INCOMPLETE'}")
    if not ok:
        bad = True
        if miss_sc: print(f"      small capitals lost: {miss_sc}")
        if miss_gx: print(f"      Greek Extended lost: {miss_gx}")
        if miss_su: print(f"      superior figures lost: {miss_su}")
f = TTFont("assets/fonts/reading/EzraSIL-Regular.ttf", lazy=True)
cm = set(f.getBestCmap())
marks = [0x591,0x59B,0x5B1,0x5B4,0x5B5,0x5B9,0x5BC]
have = sum(1 for c in marks if c in cm)
letters = sum(1 for c in range(0x5D0,0x5EB) if c in cm)
f.close()
print(f"  EzraSIL-Regular        Hebrew letters {letters}/27  marks {have}/7  "
      f"{'OK' if have==7 else 'INCOMPLETE'}")
bad = bad or have != 7
sys.exit(1 if bad else 0)
PY
