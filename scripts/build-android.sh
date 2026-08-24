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

unset ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
unset BIBLE_KEY_LDFLAGS BIBLETEXT_RELEASE_LDFLAGS BIBLETEXT_REAL_GO
export GOFLAGS="" GODEBUG=""
case "${BASH_SOURCE[0]}" in
  */*) script_dir_part="${BASH_SOURCE[0]%/*}" ;;
  *) script_dir_part="." ;;
esac
case "$script_dir_part" in
  /*|./*|../*) ;;
  *) script_dir_part="./$script_dir_part" ;;
esac
original_dir="$PWD"
builtin cd -P -- "$script_dir_part"
SCRIPT_DIR="$PWD"
builtin cd -- "$original_dir"
unset original_dir script_dir_part
source "${SCRIPT_DIR}/release-bible-key.sh"
if [ "${1:-}" = "--release" ]; then
  load_release_bible_key
else
  unset BIBLE_API_KEY
fi

REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
"$REPO_ROOT/scripts/check-repository-hygiene.py"
APP_DIR="$REPO_ROOT/cmd/mobile"
SIGN_DIR="$HOME/Library/Android/bibletext-signing"
DIST_DIR="${BIBLETEXT_ANDROID_DIST_DIR:-$HOME/Library/Android/bibletext-dist}"
KS="$SIGN_DIR/bibletext-upload.keystore"
KS_PASS_FILE="$SIGN_DIR/keystore-password.txt"
KEY_ALIAS=bibletext
[ -s "$KS_PASS_FILE" ] || { echo "ERROR: Android keystore password file is missing or empty" >&2; exit 1; }

# shellcheck disable=SC1090
source "$HOME/Library/Android/env.sh"
export PATH="$(go env GOPATH)/bin:$HOME/bin:$PATH"   # fyne + bundletool wrapper

# Build against the patched Fyne, like the iOS scripts (see patches/README.md).
# Android benefits from the caret-blink battery fix (widget/entry_cursor_anim.go,
# cross-platform); the iOS drawloop patch is inert here — its shared-file hunks
# are a write-only flag and the 2ms timing change lives in darwin_ios.go, which
# Android never compiles. go.mod is restored on exit so plain `go` commands stay
# stock. (The go.mod restore lives in the single EXIT trap below — bash keeps
# only the LAST trap per signal, so it must not be registered separately here.)
"$REPO_ROOT/scripts/setup-fyne-patch.sh"

ANDROID_JAR="$ANDROID_HOME/platforms/android-35/android.jar"
BT="$ANDROID_HOME/build-tools/35.0.0"
WORK="$(mktemp -d /tmp/bibletext-android.XXXXXX)"
cp "$REPO_ROOT/go.mod" "$WORK/go.mod.original"
cp "$APP_DIR/FyneApp.toml" "$WORK/FyneApp.toml.original"
# Restore the exact pre-build files, including any uncommitted operator edits.
# Release tooling must never replace them with the committed versions.
trap 'clear_release_bible_key; cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true; cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml" 2>/dev/null || true; rm -rf "$WORK"; rm -f "$APP_DIR/BibleText.aab" "$APP_DIR/classes2.dex" "$APP_DIR/classes.dex" "${AAB_TMP:-}" "${APK_TMP:-}"' EXIT

( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

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
  # Marketing version AND build number both come from FyneApp.toml, which is the
  # tracked ledger for this app's identity.
  #
  # THE BUILD NUMBER USED TO DEFAULT TO 1, and it was not hypothetical: the
  # shipped v1.1.7 artifact in ~/Library/Android/bibletext-dist reports
  # versionCode='1' versionName='1.1.7'. `set -u` does not catch ${VAR:-1};
  # the default just fired, silently, on a real release. Nothing keyed off
  # versionCode could then tell two builds apart, and the first Play upload
  # would have burned the lowest number there is.
  #
  # FyneApp.toml's Build is already monotonic across every tag (98, 99, 101,
  # 124, 127, 163) and is already the iOS build number, so one ledger now
  # answers for both stores. Override either with BIBLETEXT_ANDROID_VERSION /
  # BIBLETEXT_ANDROID_BUILD; an unreadable ledger is a hard stop, never a 1.
  APP_VERSION="${BIBLETEXT_ANDROID_VERSION:-$(sed -n 's/^Version = "\(.*\)"/\1/p' FyneApp.toml)}"
  APP_BUILD="${BIBLETEXT_ANDROID_BUILD:-$(sed -n 's/^Build = \([0-9]*\)/\1/p' FyneApp.toml)}"
  [ -n "$APP_VERSION" ] || { echo "ERROR: no Version in $APP_DIR/FyneApp.toml"; exit 1; }
  [ -n "$APP_BUILD" ] \
    || { echo "ERROR: no Build in $APP_DIR/FyneApp.toml — refusing to default the versionCode to 1"; exit 1; }
  note "android release $APP_VERSION (versionCode $APP_BUILD)"
  REAL_GO="$(command -v go)"
  mkdir -p "$WORK/bin" "$WORK/go-cache" "$WORK/go-tmp"
  cp "$REPO_ROOT/scripts/go-release-wrapper.sh" "$WORK/bin/go"
  chmod 700 "$WORK/bin/go"
  # Fyne passes its own -ldflags value and otherwise overrides GOFLAGS. The
  # temporary wrapper merges the release linker value only into c-shared Go
  # builds, then the package verifier proves every native library contains it.
  BIBLETEXT_REAL_GO="$REAL_GO" \
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
  GOCACHE="$WORK/go-cache" GOTMPDIR="$WORK/go-tmp" \
  PATH="$WORK/bin:$PATH" \
  fyne release -os android -app-id uk.co.bibletext -icon Icon.png \
    -app-version "$APP_VERSION" -app-build "$APP_BUILD" \
    -keyStore "$KS" -keyName "$KEY_ALIAS" \
    -keyStorePass "$(<"$KS_PASS_FILE")" -keyPass "$(<"$KS_PASS_FILE")"
  AAB_STAGE="$WORK/BibleText.aab"
  mv "$APP_DIR/BibleText.aab" "$AAB_STAGE"
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" "$AAB_STAGE"

  note "injecting classes2.dex into the AAB (base/dex/) + re-signing"
  mkdir -p "$WORK/aab/base/dex"
  cp "$WORK/classes1.dex" "$WORK/aab/base/dex/classes.dex"
  cp "$WORK/classes2.dex" "$WORK/aab/base/dex/classes2.dex"
  (cd "$WORK/aab" && zip -q -X -d "$AAB_STAGE" base/dex/classes.dex \
     && zip -q -X "$AAB_STAGE" base/dex/classes.dex base/dex/classes2.dex)
  # Modification invalidated the jar signature — strip and re-sign.
  zip -q -d "$AAB_STAGE" "META-INF/*" || true
  jarsigner -sigalg SHA256withRSA -digestalg SHA-256 \
    -keystore "$KS" -storepass:file "$KS_PASS_FILE" -keypass:file "$KS_PASS_FILE" \
    "$AAB_STAGE" "$KEY_ALIAS" >/dev/null

  note "universal APK for sideload/Firebase"
  bundletool build-apks --mode=universal --overwrite \
    --bundle="$AAB_STAGE" --output="$WORK/BibleText.apks" \
    --ks="$KS" --ks-key-alias="$KEY_ALIAS" \
    --ks-pass="file:$KS_PASS_FILE" --key-pass="file:$KS_PASS_FILE"
  APK_STAGE="$WORK/BibleText-universal.apk"
  unzip -p "$WORK/BibleText.apks" universal.apk > "$APK_STAGE"
  # Same aapt2-path assertion as the debug branch (background audio's <service>
  # only exists if the adaptive-icon resources flipped fyne onto aapt2).
  unzip -l "$APK_STAGE" | grep "mipmap-anydpi" >/dev/null \
    || { echo "ERROR: adaptive-icon resources missing from release APK"; exit 1; }
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" "$AAB_STAGE"
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" "$APK_STAGE"

  # Publish only verified packages. Copy to hidden names in the destination,
  # then atomically rename each into its canonical release path.
  mkdir -p "$DIST_DIR"
  AAB_TMP="$DIST_DIR/.BibleText.aab.tmp.$$"
  APK_TMP="$DIST_DIR/.BibleText-universal.apk.tmp.$$"
  cp "$AAB_STAGE" "$AAB_TMP"
  cp "$APK_STAGE" "$APK_TMP"
  mv -f "$AAB_TMP" "$DIST_DIR/BibleText.aab"
  AAB_TMP=""
  mv -f "$APK_TMP" "$DIST_DIR/BibleText-universal.apk"
  APK_TMP=""
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
    --ks-pass "file:$KS_PASS_FILE" --key-pass "file:$KS_PASS_FILE" \
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
