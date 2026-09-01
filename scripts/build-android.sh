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
#                                       + BibleText-Android.apk for sideload
#
# Requires: ~/Library/Android/env.sh toolchain, bundletool on PATH (release),
# and the upload keystore in ~/Library/Android/bibletext-signing/.
set -euo pipefail
umask 077

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
# Install key cleanup before loading so every early error path clears the
# isolated value. The full workspace-cleanup trap replaces this one below.
trap 'clear_release_bible_key' EXIT
load_release_bible_key

REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
if [ "${1:-}" = "--release" ]; then
  if [ -n "${BT_ANDROID_TAGS:-}" ]; then
    echo "ERROR: BT_ANDROID_TAGS is set — extra build tags are debug-APK only and must never reach a release build." >&2
    exit 1
  fi
  python3 "$REPO_ROOT/scripts/check-support-contact.py"
fi
"$REPO_ROOT/scripts/check-repository-hygiene.py"
# shellcheck source=android-artifact-verification.sh
source "$REPO_ROOT/scripts/android-artifact-verification.sh"
APP_DIR="$REPO_ROOT/cmd/mobile"
TRACKED_APP_VERSION="$(sed -n 's/^Version = "\(.*\)"/\1/p' "$APP_DIR/FyneApp.toml")"
TRACKED_APP_BUILD="$(sed -n 's/^Build = \([0-9]*\)/\1/p' "$APP_DIR/FyneApp.toml")"
if [ "${1:-}" = "--release" ]; then
  # A spent version (docs/VERSIONING.md) must never be rebuilt with new code.
  "$REPO_ROOT/scripts/check-version-not-spent.sh" "$TRACKED_APP_VERSION"
fi
[ -n "$TRACKED_APP_VERSION" ] \
  || { echo "ERROR: no Version in $APP_DIR/FyneApp.toml" >&2; exit 1; }
[ -n "$TRACKED_APP_BUILD" ] \
  || { echo "ERROR: no Build in $APP_DIR/FyneApp.toml — refusing to default the versionCode to 1" >&2; exit 1; }
APP_VERSION="$TRACKED_APP_VERSION"
APP_BUILD="$TRACKED_APP_BUILD"
APP_ID=uk.co.bibletext
SIGN_DIR="$HOME/Library/Android/bibletext-signing"
DIST_DIR="${BIBLETEXT_ANDROID_DIST_DIR:-$HOME/Library/Android/bibletext-dist}"
KS="$SIGN_DIR/bibletext-upload.keystore"
KS_PASS_FILE="$SIGN_DIR/keystore-password.txt"
KEY_ALIAS=bibletext
[ -s "$KS_PASS_FILE" ] || { echo "ERROR: Android keystore password file is missing or empty" >&2; exit 1; }

# shellcheck disable=SC1090
unset bibletext_saved_release_ldflags
bibletext_saved_release_ldflags="$BIBLE_KEY_LDFLAGS"
source "$HOME/Library/Android/env.sh"
# The local toolchain file is allowed to configure Android paths, but it must
# not reintroduce any raw credential or replace the already-isolated linker
# value. Restore that value as a non-exported shell variable after sourcing.
unset BIBLE_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
unset BIBLE_KEY_LDFLAGS BIBLETEXT_RELEASE_LDFLAGS BIBLETEXT_REAL_GO
BIBLE_KEY_LDFLAGS="$bibletext_saved_release_ldflags"
bibletext_saved_release_ldflags=""
unset bibletext_saved_release_ldflags
export GOFLAGS="" GODEBUG=""
export PATH="$(go env GOPATH)/bin:$HOME/bin:$PATH"   # fyne + bundletool wrapper

# Build against the patched Fyne library and the Android-target patch for the
# pinned Fyne CLI (see patches/README.md). Android benefits from the caret-blink
# battery fix; the iOS drawloop patch is inert here. fyne.io/tools v1.7.2 also
# needs a local patch because it otherwise emits target SDK 29 for debug and 35
# for release. go.mod is restored on exit so plain `go` commands stay stock.
# The full EXIT trap below replaces the early key-only cleanup trap; bash keeps
# only the last trap registered for a signal.
"$REPO_ROOT/scripts/setup-fyne-patch.sh"
"$REPO_ROOT/scripts/setup-fyne-tools-patch.sh"

ANDROID_TARGET_API=36
ANDROID_BUILD_TOOLS_VERSION=36.1.0
ANDROID_JAR="$ANDROID_HOME/platforms/android-${ANDROID_TARGET_API}/android.jar"
BT="$ANDROID_HOME/build-tools/$ANDROID_BUILD_TOOLS_VERSION"
[ -s "$ANDROID_JAR" ] || {
  echo "ERROR: Android platform API $ANDROID_TARGET_API is not installed ($ANDROID_JAR)" >&2
  exit 1
}
for required_tool in aapt2 apksigner d8 zipalign; do
  [ -x "$BT/$required_tool" ] || {
    echo "ERROR: Android build-tools $ANDROID_BUILD_TOOLS_VERSION lacks $required_tool" >&2
    exit 1
  }
done
export BIBLETEXT_ANDROID_BUILD_TOOLS="$BT"
export BIBLETEXT_ANDROID_PLATFORM="$ANDROID_JAR"
WORK="$(mktemp -d /tmp/bibletext-android.XXXXXX)"
cp "$REPO_ROOT/go.mod" "$WORK/go.mod.original"
cp "$APP_DIR/FyneApp.toml" "$WORK/FyneApp.toml.original"
# Restore the exact pre-build files, including any uncommitted operator edits.
# Release tooling must never replace them with the committed versions.
trap 'clear_release_bible_key; cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true; cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml" 2>/dev/null || true; rm -rf "$WORK" "$APP_DIR/tmpbundle"; rm -f "$APP_DIR/BibleText.aab" "$APP_DIR/BibleText.aab.sig" "$APP_DIR/classes2.dex" "$APP_DIR/classes.dex" "${AAB_TMP:-}" "${APK_TMP:-}"' EXIT

note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }

note "building patched Fyne CLI (Android target API $ANDROID_TARGET_API)"
mkdir -p "$WORK/bin" "$WORK/go-cache" "$WORK/go-tmp"
(
  cd "$REPO_ROOT/third_party/fyne-tools"
  GOCACHE="$WORK/go-cache" GOTMPDIR="$WORK/go-tmp" \
    go build -trimpath -o "$WORK/bin/fyne" ./cmd/fyne
)
export PATH="$WORK/bin:$PATH"

( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

# Every Android artifact, including a local development APK, carries the same
# externally sourced fallback. Fyne owns the c-shared Go invocation, so place
# the fail-closed linker wrapper ahead of the real Go binary for both branches.
REAL_GO="$(command -v go)"
cp "$REPO_ROOT/scripts/go-release-wrapper.sh" "$WORK/bin/go"
chmod 700 "$WORK/bin/go"

verify_apk_identity_sdk() {
  local package_path="$1"
  "$BT/aapt2" dump badging "$package_path" > "$WORK/android-sdk-badging.txt"
  grep -Fq "package: name='${APP_ID}'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path has the wrong application ID" >&2; exit 1; }
  grep -Fq "versionCode='${APP_BUILD}'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path has the wrong versionCode" >&2; exit 1; }
  grep -Fq "versionName='${APP_VERSION}'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path has the wrong versionName" >&2; exit 1; }
  grep -Fq "compileSdkVersion='${ANDROID_TARGET_API}'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path was not compiled with Android API $ANDROID_TARGET_API" >&2; exit 1; }
  grep -Fq "minSdkVersion:'21'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path does not declare minSdkVersion 21" >&2; exit 1; }
  grep -Fq "targetSdkVersion:'${ANDROID_TARGET_API}'" "$WORK/android-sdk-badging.txt" \
    || { echo "ERROR: $package_path does not target Android API $ANDROID_TARGET_API" >&2; exit 1; }
}

verify_aab_identity_sdk() {
  local package_path="$1"
  bundletool dump manifest --bundle="$package_path" --module=base \
    > "$WORK/android-sdk-manifest.xml"
  grep -Fq "package=\"${APP_ID}\"" "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path has the wrong application ID" >&2; exit 1; }
  grep -Fq "android:versionCode=\"${APP_BUILD}\"" "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path has the wrong versionCode" >&2; exit 1; }
  grep -Fq "android:versionName=\"${APP_VERSION}\"" "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path has the wrong versionName" >&2; exit 1; }
  grep -Fq "android:compileSdkVersion=\"${ANDROID_TARGET_API}\"" "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path was not compiled with Android API $ANDROID_TARGET_API" >&2; exit 1; }
  grep -Fq 'android:minSdkVersion="21"' "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path does not declare minSdkVersion 21" >&2; exit 1; }
  grep -Fq "android:targetSdkVersion=\"${ANDROID_TARGET_API}\"" "$WORK/android-sdk-manifest.xml" \
    || { echo "ERROR: $package_path does not target Android API $ANDROID_TARGET_API" >&2; exit 1; }
}

configured_android_signer_digest() {
  local digest
  if ! digest="$(LC_ALL=C keytool -exportcert -keystore "$KS" -alias "$KEY_ALIAS" \
    -storepass:file "$KS_PASS_FILE" \
    | python3 -c 'import hashlib,sys; print(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())')"; then
    echo "ERROR: could not read the configured Android upload certificate" >&2
    exit 1
  fi
  if [ "${#digest}" -ne 64 ] || [[ "$digest" = *[!0-9a-f]* ]]; then
    echo "ERROR: configured Android upload certificate digest is malformed" >&2
    exit 1
  fi
  printf '%s' "$digest"
}

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
  note "android release $APP_VERSION (versionCode $APP_BUILD)"
  # Fyne passes its own -ldflags value and otherwise overrides GOFLAGS. The
  # temporary wrapper merges the release linker value only into c-shared Go
  # builds, then the package verifier proves every native library contains it.
  BIBLETEXT_REAL_GO="$REAL_GO" \
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
  GOCACHE="$WORK/go-cache" GOTMPDIR="$WORK/go-tmp" \
  PATH="$WORK/bin:$PATH" \
  fyne release -os android -app-id "$APP_ID" -icon Icon.png \
    -app-version "$APP_VERSION" -app-build "$APP_BUILD" \
    -keyStore "$KS" -keyName "$KEY_ALIAS" < "$KS_PASS_FILE"
  AAB_STAGE="$WORK/BibleText.aab"
  mv "$APP_DIR/BibleText.aab" "$AAB_STAGE"
  chmod 600 "$AAB_STAGE"
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
  EXPECTED_SIGNER_SHA256="$(configured_android_signer_digest)"
  verify_aab_identity_sdk "$AAB_STAGE"
  verify_android_dex_bridge aab "$AAB_STAGE" "$WORK"
  verify_android_jni_descriptors "$WORK/aab-classes2.dex" "$REPO_ROOT/reading_android.go"
  verify_android_aab_signature "$AAB_STAGE" "$EXPECTED_SIGNER_SHA256" \
    "$WORK/aab-signature-verify.log" "$WORK/aab-signer-certificate.log"

  note "GitHub/sideload APK"
  bundletool build-apks --mode=universal --overwrite \
    --bundle="$AAB_STAGE" --output="$WORK/BibleText.apks" \
    --ks="$KS" --ks-key-alias="$KEY_ALIAS" \
    --ks-pass="file:$KS_PASS_FILE" --key-pass="file:$KS_PASS_FILE"
  APK_STAGE="$WORK/BibleText-Android.apk"
  unzip -p "$WORK/BibleText.apks" universal.apk > "$APK_STAGE"
  chmod 600 "$APK_STAGE"
  # Same aapt2-path assertion as the debug branch (background audio's <service>
  # only exists if the adaptive-icon resources flipped fyne onto aapt2).
  unzip -l "$APK_STAGE" | grep "mipmap-anydpi" >/dev/null \
    || { echo "ERROR: adaptive-icon resources missing from release APK"; exit 1; }
  verify_apk_identity_sdk "$APK_STAGE"
  verify_android_dex_bridge apk "$APK_STAGE" "$WORK"
  verify_android_jni_descriptors "$WORK/apk-classes2.dex" "$REPO_ROOT/reading_android.go"
  verify_android_apk_signature "$APK_STAGE" "$EXPECTED_SIGNER_SHA256" \
    "$BT/apksigner" "$WORK/apk-signature-verify.log"
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" "$AAB_STAGE"
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" "$APK_STAGE"

  # Publish only verified packages. Copy to hidden names in the destination,
  # then atomically rename each into its canonical release path.
  mkdir -p "$DIST_DIR"
  AAB_TMP="$DIST_DIR/.BibleText.aab.tmp.$$"
  APK_TMP="$DIST_DIR/.BibleText-Android.apk.tmp.$$"
  cp "$AAB_STAGE" "$AAB_TMP"
  cp "$APK_STAGE" "$APK_TMP"
  chmod 600 "$AAB_TMP" "$APK_TMP"
  mv -f "$AAB_TMP" "$DIST_DIR/BibleText.aab"
  AAB_TMP=""
  mv -f "$APK_TMP" "$DIST_DIR/BibleText-Android.apk"
  APK_TMP=""
  chmod 600 "$DIST_DIR/BibleText.aab" "$DIST_DIR/BibleText-Android.apk"
  note "done: $DIST_DIR/BibleText.aab + BibleText-Android.apk"
  ls -lh "$DIST_DIR"
else
  note "fyne package -os android (debug APK)"
  cd "$APP_DIR"
  rm -f BibleText.apk
  BIBLETEXT_REAL_GO="$REAL_GO" \
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
  GOCACHE="$WORK/go-cache" GOTMPDIR="$WORK/go-tmp" \
  PATH="$WORK/bin:$PATH" \
  # BT_ANDROID_TAGS: extra build tags for the DEBUG APK only (the dev Links
  # page's tag, for emulator work). The release path above refuses it outright
  # -- see the check near the top -- and the release guard test additionally
  # asserts this script never names that tag.
  fyne package -os android -app-id "$APP_ID" -icon Icon.png \
      ${BT_ANDROID_TAGS:+--tags "$BT_ANDROID_TAGS"}

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
  # The key uses the keystore password, which apksigner reuses when --key-pass
  # is omitted. Supplying the same one-line file for both options consumes two
  # lines from one reader and fails at EOF.
  "$BT/apksigner" sign --ks "$KS" --ks-key-alias "$KEY_ALIAS" \
    --ks-pass "file:$KS_PASS_FILE" \
    --out BibleText.apk "$WORK/aligned.apk"
  chmod 600 BibleText.apk
  EXPECTED_SIGNER_SHA256="$(configured_android_signer_digest)"
  verify_android_apk_signature BibleText.apk "$EXPECTED_SIGNER_SHA256" \
    "$BT/apksigner" "$WORK/debug-apk-signature-verify.log"
  verify_apk_identity_sdk BibleText.apk
  verify_android_dex_bridge apk BibleText.apk "$WORK"
  verify_android_jni_descriptors "$WORK/apk-classes2.dex" "$REPO_ROOT/reading_android.go"
  BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
    python3 "$REPO_ROOT/scripts/verify-release-key.py" BibleText.apk
  note "done: $APP_DIR/BibleText.apk (signed with the upload key — uninstall any"
  echo "    build signed with a different key before installing)"
  ls -lh BibleText.apk
fi
