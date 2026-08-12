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

# Build against the patched Fyne, like the iOS scripts (see patches/README.md).
# Android benefits from the caret-blink battery fix (widget/entry_cursor_anim.go,
# cross-platform); the iOS drawloop patch is inert here — its shared-file hunks
# are a write-only flag and the 2ms timing change lives in darwin_ios.go, which
# Android never compiles. go.mod is restored on exit so plain `go` commands stay
# stock. (The go.mod restore lives in the single EXIT trap below — bash keeps
# only the LAST trap per signal, so it must not be registered separately here.)
"$REPO_ROOT/scripts/setup-fyne-patch.sh"

# Compile in the project's own API.Bible key (from .env.local) so the NKJV
# works out of the box; a build with no key present is simply bring-your-own-key.
# NOTE: embed-bible-key.sh arms its OWN EXIT trap to delete the generated file,
# but this script registers a trap BELOW it and bash keeps only the last trap per
# signal — so that one would be silently clobbered and the key file would survive
# every build. The deletion is therefore folded into the single trap below, which
# is the convention this script already documents. (The iOS scripts have the same
# collision the other way round: they arm their trap ABOVE the source, so it is
# THEIRS that gets clobbered — their go.mod restore was being eaten until they
# were given the same fold. Registered above means clobbered, not safe.)
source "${REPO_ROOT}/scripts/embed-bible-key.sh"
# Bundled by default, per the owner's decision recorded in release-ios.sh: the
# NKJV works on install now, and a later release drops the key so new installs
# are bring-your-own-key. BIBLETEXT_BUNDLE_KEY=0 is that switch.
if [ "${BIBLETEXT_BUNDLE_KEY:-1}" = "1" ]; then
  embed_bible_key
else
  echo "==> NO bundled API.Bible key — new installs are bring-your-own-key"
fi

( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

ANDROID_JAR="$ANDROID_HOME/platforms/android-35/android.jar"
BT="$ANDROID_HOME/build-tools/35.0.0"
WORK="$(mktemp -d /tmp/bibletext-android.XXXXXX)"
# The trap reverts fyne's Build++ writeback to FyneApp.toml — but ONLY when the
# file was clean at start, so it can never destroy uncommitted manual edits.
TOML_WAS_DIRTY="$(git -C "$REPO_ROOT" status --porcelain -- cmd/mobile/FyneApp.toml 2>/dev/null || true)"
trap 'rm -rf "$WORK"; rm -f "$APP_DIR/classes2.dex" "$APP_DIR/classes.dex" "$REPO_ROOT/bundled_key_gen.go"; git -C "$REPO_ROOT" checkout -- go.mod 2>/dev/null || true; if [ -z "$TOML_WAS_DIRTY" ]; then git -C "$REPO_ROOT" checkout -- cmd/mobile/FyneApp.toml 2>/dev/null || true; fi' EXIT

note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }

note "compiling android/*.java (BtBridge + BtAudio) -> classes2.dex"
mkdir -p "$WORK/classes"
javac --release 8 -Xlint:-options -cp "$ANDROID_JAR" -d "$WORK/classes" \
  "$REPO_ROOT"/android/*.java
mkdir -p "$WORK/dexout"
"$BT/d8" --min-api 21 --lib "$ANDROID_JAR" --output "$WORK/dexout" \
  "$WORK/classes/org/bibletext/"*.class
mv "$WORK/dexout/classes.dex" "$WORK/classes2.dex"
ls -la "$WORK/classes2.dex"

note "compiling patched GoNativeActivity (+ FyneNotificationReceiver) -> replacement classes.dex"
# The fyne CLI packages a PREBUILT classes.dex (gendex output baked into the
# tool), so the newintent patch applied to third_party/fyne's GoNativeActivity
# .java would never reach the APK. Recompile the patched activity ourselves —
# same javac/d8 recipe gendex uses — plus android/goapp's reconstructed
# FyneNotificationReceiver (the manifest's <receiver> must keep resolving),
# and swap the result in over the tool's dex below.
mkdir -p "$WORK/classes1" "$WORK/dexout1"
javac --release 8 -Xlint:-options -cp "$ANDROID_JAR" -d "$WORK/classes1" \
  "$REPO_ROOT/third_party/fyne/internal/driver/mobile/app/GoNativeActivity.java" \
  "$REPO_ROOT/android/goapp/FyneNotificationReceiver.java"
"$BT/d8" --min-api 21 --lib "$ANDROID_JAR" --output "$WORK/dexout1" \
  "$WORK/classes1/org/golang/app/"*.class
mv "$WORK/dexout1/classes.dex" "$WORK/classes1.dex"
grep -q "onNewIntent" "$REPO_ROOT/third_party/fyne/internal/driver/mobile/app/GoNativeActivity.java" \
  || { echo "ERROR: newintent patch missing from third_party GoNativeActivity"; exit 1; }
ls -la "$WORK/classes1.dex"

if [ "${1:-}" = "--release" ]; then
  note "fyne release -os android (signed AAB)"
  cd "$APP_DIR"
  rm -f BibleText.aab
  # Marketing version defaults to the Version in FyneApp.toml (override with
  # BIBLETEXT_ANDROID_VERSION); the build number stays explicit per upload.
  APP_VERSION="${BIBLETEXT_ANDROID_VERSION:-$(sed -n 's/^Version = "\(.*\)"/\1/p' FyneApp.toml)}"
  fyne release -os android -app-id uk.co.bibletext -icon Icon.png \
    -app-version "${APP_VERSION:-1.0.0}" -app-build "${BIBLETEXT_ANDROID_BUILD:-1}" \
    -keyStore "$KS" -keyName "$KEY_ALIAS" -keyStorePass "$KS_PASS" -keyPass "$KS_PASS"

  note "injecting classes2.dex into the AAB (base/dex/) + re-signing"
  mkdir -p "$WORK/aab/base/dex"
  cp "$WORK/classes1.dex" "$WORK/aab/base/dex/classes.dex"
  cp "$WORK/classes2.dex" "$WORK/aab/base/dex/classes2.dex"
  (cd "$WORK/aab" && zip -q -X -d "$APP_DIR/BibleText.aab" base/dex/classes.dex \
     && zip -q -X "$APP_DIR/BibleText.aab" base/dex/classes.dex base/dex/classes2.dex)
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
  # Same aapt2-path assertion as the debug branch (background audio's <service>
  # only exists if the adaptive-icon resources flipped fyne onto aapt2).
  unzip -l "$DIST_DIR/BibleText-universal.apk" | grep "mipmap-anydpi" >/dev/null \
    || { echo "ERROR: adaptive-icon resources missing from release APK"; exit 1; }
  mv "$APP_DIR/BibleText.aab" "$DIST_DIR/BibleText.aab"
  note "done: $DIST_DIR/BibleText.aab + BibleText-universal.apk"
  ls -lh "$DIST_DIR"
else
  note "fyne package -os android (debug APK)"
  cd "$APP_DIR"
  rm -f BibleText.apk
  fyne package -os android -app-id uk.co.bibletext -icon Icon.png

  note "verifying the aapt2 resource path was taken (adaptive icon present)"
  # The custom AndroidManifest.xml (background-audio <service>) only compiles on
  # fyne's aapt2 path, which is triggered by cmd/mobile/Icon-foreground.png. If
  # that file went missing, fyne would silently fall back to the legacy binres
  # encoder — which errors on foregroundServiceType — so assert positively here.
  # (plain grep >/dev/null, NOT grep -q: -q exits on first match, unzip takes a
  #  SIGPIPE, and pipefail turns the successful check into a build failure)
  unzip -l "$APP_DIR/BibleText.apk" | grep "res/mipmap-anydpi-v26/ic_launcher.xml" >/dev/null \
    || { echo "ERROR: adaptive-icon resources missing — aapt2 path not taken"; exit 1; }

  note "replacing classes.dex (patched GoNativeActivity) + injecting classes2.dex + zipalign + apksigner"
  cp "$WORK/classes1.dex" "$APP_DIR/classes.dex"   # zip stores the path as given — add from cwd
  cp "$WORK/classes2.dex" "$APP_DIR"
  (cd "$APP_DIR" && zip -q -X -d BibleText.apk classes.dex \
     && zip -q -X BibleText.apk classes.dex classes2.dex \
     && rm -f classes.dex classes2.dex)
  "$BT/zipalign" -f -p 4 BibleText.apk "$WORK/aligned.apk"
  "$BT/apksigner" sign --ks "$KS" --ks-key-alias "$KEY_ALIAS" \
    --ks-pass "pass:$KS_PASS" --key-pass "pass:$KS_PASS" \
    --out BibleText.apk "$WORK/aligned.apk"
  "$BT/apksigner" verify BibleText.apk
  # Assert the swap actually landed: a silent miss here would ship an APK whose
  # GoNativeActivity has no onNewIntent, i.e. warm shared links dropped again —
  # the exact failure this whole path exists to prevent, and one that is
  # invisible until someone taps a link with the app already open.
  unzip -p BibleText.apk classes.dex | strings | grep "onNewIntent" >/dev/null \
    || { echo "ERROR: classes.dex in the APK has no onNewIntent — the dex swap did not land"; exit 1; }
  note "done: $APP_DIR/BibleText.apk (signed with the upload key — uninstall any"
  echo "    build signed with a different key before installing)"
  ls -lh BibleText.apk
fi
