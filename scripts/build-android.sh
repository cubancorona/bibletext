#!/usr/bin/env bash
# Build BibleText for Android WITH the native reading overlay's Java bridge.
#
# `fyne package/release -os android` can only ship the dex embedded in the fyne
# CLI (GoNativeActivity), so the BtBridge class (android/BtBridge.java — the
# native selectable TextView + selection menu) is compiled here with javac+d8
# and injected as classes2.dex, which ART auto-loads on API 21+. The APK is
# then re-signed (apksigner, v1+v2+v3 — strictly better than fyne's v1-only
# debug signing); the AAB is re-signed with jarsigner (the AAB scheme).
#
#   scripts/build-android.sh            debug APK  -> cmd/mobile/BibleText.apk
#   scripts/build-android.sh --release  signed AAB -> ~/Library/Android/bibletext-dist/BibleText.aab
#                                       + universal APK for sideload/Firebase
#
# Requires: ~/Library/Android/env.sh toolchain, bundletool on PATH (release),
# and the upload keystore in ~/Library/Android/bibletext-signing/.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="$REPO_ROOT/cmd/mobile"
SIGN_DIR="$HOME/Library/Android/bibletext-signing"
DIST_DIR="$HOME/Library/Android/bibletext-dist"
KS="$SIGN_DIR/bibletext-upload.keystore"
KS_PASS="$(cat "$SIGN_DIR/keystore-password.txt")"
KEY_ALIAS=bibletext

# shellcheck disable=SC1090
source "$HOME/Library/Android/env.sh"
export PATH="$HOME/bin:$PATH"   # bundletool wrapper

ANDROID_JAR="$ANDROID_HOME/platforms/android-35/android.jar"
BT="$ANDROID_HOME/build-tools/35.0.0"
WORK="$(mktemp -d /tmp/bibletext-android.XXXXXX)"
trap 'rm -rf "$WORK"; rm -f "$APP_DIR/classes2.dex"; git -C "$REPO_ROOT" checkout -- cmd/mobile/FyneApp.toml 2>/dev/null || true' EXIT

note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }

note "compiling BtBridge.java -> classes2.dex"
mkdir -p "$WORK/classes"
javac --release 8 -Xlint:-options -cp "$ANDROID_JAR" -d "$WORK/classes" \
  "$REPO_ROOT/android/BtBridge.java"
mkdir -p "$WORK/dexout"
"$BT/d8" --min-api 21 --lib "$ANDROID_JAR" --output "$WORK/dexout" \
  "$WORK/classes/org/bibletext/"*.class
mv "$WORK/dexout/classes.dex" "$WORK/classes2.dex"
ls -la "$WORK/classes2.dex"

if [ "${1:-}" = "--release" ]; then
  note "fyne release -os android (signed AAB)"
  cd "$APP_DIR"
  rm -f BibleText.aab
  fyne release -os android -app-id uk.co.bibletext -icon Icon.png \
    -app-version 1.0.0 -app-build "${BIBLETEXT_ANDROID_BUILD:-1}" \
    -keyStore "$KS" -keyName "$KEY_ALIAS" -keyStorePass "$KS_PASS" -keyPass "$KS_PASS"

  note "injecting classes2.dex into the AAB (base/dex/) + re-signing"
  mkdir -p "$WORK/aab/base/dex"
  cp "$WORK/classes2.dex" "$WORK/aab/base/dex/classes2.dex"
  (cd "$WORK/aab" && zip -q -X "$APP_DIR/BibleText.aab" base/dex/classes2.dex)
  # Modification invalidated the jar signature — strip and re-sign.
  zip -q -d "$APP_DIR/BibleText.aab" "META-INF/*" || true
  jarsigner -sigalg SHA256withRSA -digestalg SHA-256 \
    -keystore "$KS" -storepass "$KS_PASS" -keypass "$KS_PASS" \
    "$APP_DIR/BibleText.aab" "$KEY_ALIAS" >/dev/null

  note "universal APK for sideload/Firebase"
  mkdir -p "$DIST_DIR"
  bundletool build-apks --mode=universal --overwrite \
    --bundle="$APP_DIR/BibleText.aab" --output="$WORK/BibleText.apks" \
    --ks="$KS" --ks-key-alias="$KEY_ALIAS" \
    --ks-pass="pass:$KS_PASS" --key-pass="pass:$KS_PASS"
  unzip -p "$WORK/BibleText.apks" universal.apk > "$DIST_DIR/BibleText-universal.apk"
  mv "$APP_DIR/BibleText.aab" "$DIST_DIR/BibleText.aab"
  note "done: $DIST_DIR/BibleText.aab + BibleText-universal.apk"
  ls -lh "$DIST_DIR"
else
  note "fyne package -os android (debug APK)"
  cd "$APP_DIR"
  rm -f BibleText.apk
  fyne package -os android -app-id uk.co.bibletext -icon Icon.png

  note "injecting classes2.dex + zipalign + apksigner"
  cp "$WORK/classes2.dex" "$APP_DIR"  # zip stores the path as given — add from cwd
  (cd "$APP_DIR" && zip -q -X BibleText.apk classes2.dex && rm -f classes2.dex)
  "$BT/zipalign" -f -p 4 BibleText.apk "$WORK/aligned.apk"
  "$BT/apksigner" sign --ks "$KS" --ks-key-alias "$KEY_ALIAS" \
    --ks-pass "pass:$KS_PASS" --key-pass "pass:$KS_PASS" \
    --out BibleText.apk "$WORK/aligned.apk"
  "$BT/apksigner" verify BibleText.apk
  note "done: $APP_DIR/BibleText.apk (signed with the upload key — uninstall any"
  echo "    build signed with a different key before installing)"
  ls -lh BibleText.apk
fi
