#!/usr/bin/env bash
# Prove that a running iOS build follows a system light/dark switch.
#
# The twin of scripts/test-android-dark-mode.sh. iOS has never had the gap
# Android had — UIKit's traitCollectionDidChange calls updateConfig, which
# sends a size.Event carrying isDark(), the same path a resize takes — so this
# script exists to keep it that way rather than to prove a fix. A Fyne bump
# that stopped stamping DarkMode into that event would give iOS the bug Android
# had, silently, and only a reader at sunset would notice.
#
#   scripts/test-ios-dark-mode.sh
#
# Needs a booted simulator with the app already installed:
#   xcrun simctl boot "iPhone 17 Pro" && scripts/run-ios-sim.sh
#
# It never touches the app — only the simulator's appearance setting.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
# The identity comes from the one file that owns it; nothing here hardcodes it
# (scripts/check-product-identity.py exists to keep copies from appearing).
BUNDLE_ID="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["appID"],end="")' \
  "$REPO_ROOT/config/product.json")"

fail() { echo "ERROR: $*" >&2; exit 1; }

command -v xcrun >/dev/null 2>&1 || fail "xcrun is not on PATH; install Xcode"
python3 -c 'import PIL' 2>/dev/null || fail "python3 needs Pillow for the pixel sampling"
xcrun simctl list devices booted | grep -q Booted || fail "no simulator is booted"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The dominant colour of the screen. The reading pane is most of the window, so
# the modal colour IS the palette ground; one sampled point lands on whatever
# widget happens to sit there.
dominant() {
  xcrun simctl io booted screenshot --type=png "$WORK/shot.png" >/dev/null 2>&1
  python3 - "$WORK/shot.png" <<'PY'
from PIL import Image
from collections import Counter
import sys
im = Image.open(sys.argv[1]).convert("RGB")
(col, n) = Counter(im.getdata()).most_common(1)[0]
print(col[0], col[1], col[2], n * 100 // (im.size[0] * im.size[1]))
PY
}

luma() { echo "$1 $2 $3" | awk '{printf "%d", 0.2126*$1 + 0.7152*$2 + 0.0722*$3}'; }

echo "==> appearance to light, then launching $BUNDLE_ID"
xcrun simctl ui booted appearance light >/dev/null 2>&1 ||
  fail "this simulator runtime does not support 'simctl ui appearance'"
xcrun simctl launch booted "$BUNDLE_ID" >/dev/null 2>&1 ||
  fail "$BUNDLE_ID is not installed on the booted simulator (run scripts/run-ios-sim.sh)"
sleep 30   # a first run downloads a translation before the reading pane settles

read -r LR LG LB LPCT <<<"$(dominant)"
echo "    light: rgb($LR,$LG,$LB) over $LPCT% of the screen"

echo "==> switching the SYSTEM appearance to dark; the app is not touched"
xcrun simctl ui booted appearance dark >/dev/null
sleep 10
read -r DR DG DB DPCT <<<"$(dominant)"
echo "    dark:  rgb($DR,$DG,$DB) over $DPCT% of the screen"

echo "==> back to light"
xcrun simctl ui booted appearance light >/dev/null
sleep 10
read -r BR BG BB BPCT <<<"$(dominant)"
echo "    back:  rgb($BR,$BG,$BB) over $BPCT% of the screen"

LIGHT_LUMA="$(luma "$LR" "$LG" "$LB")"
DARK_LUMA="$(luma "$DR" "$DG" "$DB")"

[ "$LR $LG $LB" != "$DR $DG $DB" ] ||
  fail "the app did not follow: the screen is still rgb($LR,$LG,$LB) after the system went dark"
[ "$DARK_LUMA" -lt "$LIGHT_LUMA" ] ||
  fail "the screen changed but did not darken (light luma $LIGHT_LUMA, dark luma $DARK_LUMA)"
[ $((LIGHT_LUMA - DARK_LUMA)) -gt 80 ] ||
  fail "the change is too small to be a palette switch (light $LIGHT_LUMA, dark $DARK_LUMA)"
[ "$BR $BG $BB" = "$LR $LG $LB" ] ||
  fail "the app did not return to the light palette: rgb($BR,$BG,$BB), was rgb($LR,$LG,$LB)"

echo "==> PASS: the running app followed the system both ways"
echo "    light luma $LIGHT_LUMA -> dark luma $DARK_LUMA -> back to rgb($BR,$BG,$BB)"
