#!/usr/bin/env bash
# Prove that a running Android build follows a system light/dark switch.
#
# The app has no in-app appearance control (theme.go); it follows the system.
# On Android that promise depended on a toolkit path that dropped the night
# flag — the Java side reported it, but it was only ever stamped into a
# size.Event by the redraw branch, and a theme-only change produces no redraw.
# patches/fyne-2.7.4-android-night-mode.patch forwards it.
#
# The Go tests beside this script (dark_mode_follow_test.go) hold the patch in
# place, but they cannot exercise it: android.go is behind GOOS=android and
# cgo, and no emulator runs in CI. This is the check that watches the pixels.
#
#   scripts/test-android-dark-mode.sh                      # the debug APK
#   scripts/test-android-dark-mode.sh <path-to.apk>        # any build
#   scripts/test-android-dark-mode.sh --control <path.apk> # must NOT follow
#
# --control inverts the assertion: it is how the fix was proven, by running an
# unpatched build through the identical sequence and requiring that it does not
# change. A test that cannot fail proves nothing, and the control is what shows
# this one can.
#
# Needs: a booted emulator or device (adb), and python3 with Pillow for the
# pixel sampling. It never touches the app — only the system setting.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CONTROL=0
if [ "${1:-}" = "--control" ]; then
  CONTROL=1
  shift
fi
APK="${1:-$REPO_ROOT/cmd/mobile/BibleText.apk}"

fail() { echo "ERROR: $*" >&2; exit 1; }

[ -f "$APK" ] || fail "no APK at $APK (build one with scripts/build-android.sh)"
command -v adb >/dev/null 2>&1 || fail "adb is not on PATH; source ~/Library/Android/env.sh first"
python3 -c 'import PIL' 2>/dev/null || fail "python3 needs Pillow for the pixel sampling"
[ -n "$(adb devices | sed -n '2p')" ] || fail "no device or emulator is attached"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The dominant colour of the screen, as "r g b pct". The reading pane is most
# of the window, so the modal colour IS the palette ground; sampling one point
# lands on whatever widget happens to be there.
dominant() {
  adb exec-out screencap -p > "$WORK/shot.png"
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

echo "==> installing $(basename "$APK")"
adb uninstall uk.co.bibletext >/dev/null 2>&1 || true
adb install -r -d "$APK" >/dev/null

echo "==> system to light, then launching"
adb shell cmd uimode night no >/dev/null
adb shell am start -n uk.co.bibletext/org.golang.app.GoNativeActivity >/dev/null
for _ in $(seq 1 40); do
  adb shell dumpsys window 2>/dev/null | grep -q 'mCurrentFocus.*bibletext' && break
  sleep 4
done
adb shell dumpsys window 2>/dev/null | grep -q 'mCurrentFocus.*bibletext' ||
  fail "the app never took focus"
sleep 30   # first run downloads a translation before the reading pane settles

read -r LR LG LB LPCT <<<"$(dominant)"
echo "    light: rgb($LR,$LG,$LB) over $LPCT% of the screen"

echo "==> flipping the SYSTEM to dark; the app is not touched"
adb shell cmd uimode night yes >/dev/null
sleep 12
read -r DR DG DB DPCT <<<"$(dominant)"
echo "    dark:  rgb($DR,$DG,$DB) over $DPCT% of the screen"

# The app must still own the screen, or the comparison is of something else.
adb shell dumpsys window 2>/dev/null | grep -q 'mCurrentFocus.*bibletext' ||
  fail "the app lost focus during the flip; this measured something else"

echo "==> flipping back to light"
adb shell cmd uimode night no >/dev/null
sleep 12
read -r BR BG BB BPCT <<<"$(dominant)"
echo "    back:  rgb($BR,$BG,$BB) over $BPCT% of the screen"

LIGHT_LUMA="$(luma "$LR" "$LG" "$LB")"
DARK_LUMA="$(luma "$DR" "$DG" "$DB")"
CHANGED=1
[ "$LR $LG $LB" = "$DR $DG $DB" ] && CHANGED=0

if [ "$CONTROL" = "1" ]; then
  # An unpatched build latches the flag and never announces it, so every
  # frame is identical. Anything else means this build carries the fix.
  [ "$CHANGED" = "0" ] ||
    fail "the control build FOLLOWED the switch (light $LIGHT_LUMA -> dark $DARK_LUMA); it is not unpatched"
  echo "==> control passed: the build does not follow a live switch, as expected"
  exit 0
fi

[ "$CHANGED" = "1" ] ||
  fail "the app did not follow: the screen is still rgb($LR,$LG,$LB) after the system went dark"
[ "$DARK_LUMA" -lt "$LIGHT_LUMA" ] ||
  fail "the screen changed but did not darken (light luma $LIGHT_LUMA, dark luma $DARK_LUMA)"
[ $((LIGHT_LUMA - DARK_LUMA)) -gt 80 ] ||
  fail "the change is too small to be a palette switch (light $LIGHT_LUMA, dark $DARK_LUMA)"
[ "$BR $BG $BB" = "$LR $LG $LB" ] ||
  fail "the app did not return to the light palette: rgb($BR,$BG,$BB), was rgb($LR,$LG,$LB)"

echo "==> PASS: the running app followed the system both ways"
echo "    light luma $LIGHT_LUMA -> dark luma $DARK_LUMA -> back to rgb($BR,$BG,$BB)"
