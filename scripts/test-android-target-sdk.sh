#!/usr/bin/env bash
# Exercise the generated Fyne-tools patch without building or signing an app.
set -euo pipefail

cd "$(dirname "$0")/.."
scripts/setup-fyne-tools-patch.sh

if find third_party/fyne-tools -type f \( -name '*.orig' -o -name '*.rej' \) -print -quit \
  | grep . >/dev/null; then
  echo "ERROR: generated Fyne tools tree contains patch residue" >&2
  exit 1
fi

TEST_WORK="$(mktemp -d /tmp/bibletext-android-sdk-test.XXXXXX)"
trap 'rm -rf -- "$TEST_WORK"' EXIT
mkdir -p "$TEST_WORK/go-cache" "$TEST_WORK/go-tmp"

(
  cd third_party/fyne-tools
  GOCACHE="$TEST_WORK/go-cache" GOTMPDIR="$TEST_WORK/go-tmp" \
    go test ./cmd/fyne/internal/mobile ./cmd/fyne/internal/mobile/binres \
      ./cmd/fyne/internal/util \
      -run '^TestBibleTextAndroid'
)

# Fyne forwards its stdin to jarsigner. Prove with a synthetic keystore that a
# one-line password signs successfully without appearing in process arguments,
# then exercise the strict AAB verifier against signed, unsigned, and appended
# (partially unsigned) archives. No developer signing material is consulted.
SYNTHETIC_DIR="$TEST_WORK/synthetic-signing"
mkdir -p "$SYNTHETIC_DIR/payload" "$SYNTHETIC_DIR/tamper"
SYNTHETIC_PASSWORD='synthetic-android-password'
printf '%s\n' "$SYNTHETIC_PASSWORD" > "$SYNTHETIC_DIR/password.txt"
printf '%s\n' 'synthetic payload' > "$SYNTHETIC_DIR/payload/content.txt"
keytool -genkeypair -noprompt -keyalg RSA -validity 1 \
  -dname 'CN=Synthetic Android Test' -alias synthetic \
  -keystore "$SYNTHETIC_DIR/upload.p12" -storetype PKCS12 \
  -storepass:file "$SYNTHETIC_DIR/password.txt" \
  -keypass:file "$SYNTHETIC_DIR/password.txt" >/dev/null 2>&1
(cd "$SYNTHETIC_DIR/payload" && jar --create --file "$SYNTHETIC_DIR/unsigned.aab" content.txt)
cp "$SYNTHETIC_DIR/unsigned.aab" "$SYNTHETIC_DIR/signed.aab"
jarsigner -keystore "$SYNTHETIC_DIR/upload.p12" \
  "$SYNTHETIC_DIR/signed.aab" synthetic \
  < "$SYNTHETIC_DIR/password.txt" >/dev/null 2>&1
SYNTHETIC_DIGEST="$(keytool -exportcert -keystore "$SYNTHETIC_DIR/upload.p12" \
  -alias synthetic -storepass:file "$SYNTHETIC_DIR/password.txt" \
  | python3 -c 'import hashlib,sys; print(hashlib.sha256(sys.stdin.buffer.read()).hexdigest())')"

# shellcheck source=android-artifact-verification.sh
source scripts/android-artifact-verification.sh
verify_android_aab_signature "$SYNTHETIC_DIR/signed.aab" "$SYNTHETIC_DIGEST" \
  "$SYNTHETIC_DIR/signed-verify.log" "$SYNTHETIC_DIR/signed-cert.log"
if verify_android_aab_signature "$SYNTHETIC_DIR/unsigned.aab" "$SYNTHETIC_DIGEST" \
  "$SYNTHETIC_DIR/unsigned-verify.log" "$SYNTHETIC_DIR/unsigned-cert.log" \
  >/dev/null 2>&1; then
  echo "ERROR: Android AAB verifier accepted an unsigned archive" >&2
  exit 1
fi
cp "$SYNTHETIC_DIR/signed.aab" "$SYNTHETIC_DIR/tampered.aab"
printf '%s\n' 'appended after signing' > "$SYNTHETIC_DIR/tamper/appended.txt"
(cd "$SYNTHETIC_DIR/tamper" && zip -q "$SYNTHETIC_DIR/tampered.aab" appended.txt)
if verify_android_aab_signature "$SYNTHETIC_DIR/tampered.aab" "$SYNTHETIC_DIGEST" \
  "$SYNTHETIC_DIR/tampered-verify.log" "$SYNTHETIC_DIR/tampered-cert.log" \
  >/dev/null 2>&1; then
  echo "ERROR: Android AAB verifier accepted an archive with unsigned entries" >&2
  exit 1
fi

python3 - "$SYNTHETIC_DIR" <<'PY'
from pathlib import Path
import sys
import zipfile

root = Path(sys.argv[1])
primary = b"synthetic dex payload with onNewIntent bridge"
secondary = b"synthetic BtBridge and BtAudioService dex payload"
with zipfile.ZipFile(root / "dex-good.aab", "w") as archive:
    archive.writestr("base/dex/classes.dex", primary)
    archive.writestr("base/dex/classes2.dex", secondary)
with zipfile.ZipFile(root / "dex-good.apk", "w") as archive:
    archive.writestr("classes.dex", primary)
    archive.writestr("classes2.dex", secondary)
with zipfile.ZipFile(root / "dex-renamed.aab", "w") as archive:
    archive.writestr("base/dex/classes.dex", primary)
    archive.writestr("base/dex/classes3.dex", secondary)
PY
mkdir -p "$SYNTHETIC_DIR/dex-work"
verify_android_dex_bridge aab "$SYNTHETIC_DIR/dex-good.aab" "$SYNTHETIC_DIR/dex-work"
verify_android_dex_bridge apk "$SYNTHETIC_DIR/dex-good.apk" "$SYNTHETIC_DIR/dex-work"
if verify_android_dex_bridge aab "$SYNTHETIC_DIR/dex-renamed.aab" \
  "$SYNTHETIC_DIR/dex-work" >/dev/null 2>&1; then
  echo "ERROR: Android dex verifier accepted a missing classes2.dex entry" >&2
  exit 1
fi

# The JNI descriptor verifier needs REAL bytecode, so this compiles the repo's
# own BtBridge.java and dexes it. Two runs: the tree as it stands must pass, and
# the same tree with one parameter dropped from setNote must be rejected — the
# exact skew that otherwise reaches a device as a silently dead bridge.
#
# The bridge is compiled with every other android/*.java, as
# scripts/build-android.sh and scripts/check-android-java.sh compile it: it
# calls classes kept in files of their own (BtReadingColumn), and alone it
# does not compile. Each compile and dex keeps its output and prints it when
# the step fails, so a failure names its cause.
#
# The portable half of this contract is TestJNIDescriptorsMatchBtBridge, which
# compares the two SOURCES and runs on every platform with no SDK. This block is
# the stronger check against real bytecode, and needs an Android SDK; where
# there is none it says so on stderr rather than passing quietly.
JNI_DIR="$TEST_WORK/jni-descriptors"
mkdir -p "$JNI_DIR"
ANDROID_JAR=""
for candidate in "${ANDROID_HOME:-}/platforms"/android-*/android.jar; do
  [ -f "$candidate" ] && ANDROID_JAR="$candidate"
done
D8=""
for candidate in "${ANDROID_HOME:-}/build-tools"/*/d8; do
  [ -x "$candidate" ] && D8="$candidate"
done

# A real API-24 APK can omit obsolete v1 metadata. The supported-range guard
# must still enforce v2/v3 and its single expected signer, and reject tampering.
APK_AAPT2="${D8%/d8}/aapt2"
APK_SIGNER="${D8%/d8}/apksigner"
if [ -n "$ANDROID_JAR" ] && [ -n "$D8" ] && [ -x "$APK_AAPT2" ] && [ -x "$APK_SIGNER" ]; then
  echo "APK signature controls: build-tools $(basename "$(dirname "$APK_SIGNER")"), apksigner $("$APK_SIGNER" version)"
  APK_TEST="$SYNTHETIC_DIR/apk"
  mkdir -p "$APK_TEST"
  cat > "$APK_TEST/AndroidManifest.xml" <<'XML'
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.example.synthetic">
  <application android:label="Synthetic signature test"/>
</manifest>
XML
  "$APK_AAPT2" link -o "$APK_TEST/unsigned.apk" --manifest "$APK_TEST/AndroidManifest.xml" \
    -I "$ANDROID_JAR" --min-sdk-version 24 --target-sdk-version 36
  "$APK_SIGNER" sign --ks "$SYNTHETIC_DIR/upload.p12" --ks-key-alias synthetic \
    --ks-pass "file:$SYNTHETIC_DIR/password.txt" \
    --v1-signing-enabled false --v2-signing-enabled true --v3-signing-enabled true \
    --out "$APK_TEST/modern.apk" "$APK_TEST/unsigned.apk"
  verify_android_apk_signature "$APK_TEST/modern.apk" "$SYNTHETIC_DIGEST" \
    "$APK_SIGNER" "$APK_TEST/valid.log"
  "$APK_AAPT2" dump badging "$APK_TEST/modern.apk" > "$APK_TEST/badging.log"
  grep -Fq "minSdkVersion:'24'" "$APK_TEST/badging.log" \
    || { echo "ERROR: signature verification changed the APK minimum SDK" >&2; exit 1; }
  if unzip -Z1 "$APK_TEST/modern.apk" | grep -Fq 'META-INF/MANIFEST.MF'; then
    echo "ERROR: API-24 fixture unexpectedly contains legacy v1 metadata" >&2
    exit 1
  fi

  for missing in v2 v3; do
    v2=true; v3=true
    [ "$missing" != v2 ] || v2=false
    [ "$missing" != v3 ] || v3=false
    "$APK_SIGNER" sign --ks "$SYNTHETIC_DIR/upload.p12" --ks-key-alias synthetic \
      --ks-pass "file:$SYNTHETIC_DIR/password.txt" \
      --v1-signing-enabled true --v2-signing-enabled "$v2" --v3-signing-enabled "$v3" \
      --out "$APK_TEST/no-$missing.apk" "$APK_TEST/unsigned.apk"
    if verify_android_apk_signature "$APK_TEST/no-$missing.apk" "$SYNTHETIC_DIGEST" \
      "$APK_SIGNER" "$APK_TEST/no-$missing.log" >"$APK_TEST/no-$missing-error.log" 2>&1; then
      echo "ERROR: APK verifier accepted a missing $missing signature" >&2
      exit 1
    fi
    grep -Fq "Verified using $missing scheme (APK Signature Scheme $missing): false" "$APK_TEST/no-$missing.log" \
      || { echo "ERROR: missing-$missing control did not exercise the intended scheme" >&2; exit 1; }
    grep -Fq 'APK verifier output:' "$APK_TEST/no-$missing-error.log" \
      || { echo "ERROR: failed APK verification discarded its diagnostic output" >&2; exit 1; }
  done
  if verify_android_apk_signature "$APK_TEST/modern.apk" \
    0000000000000000000000000000000000000000000000000000000000000000 \
    "$APK_SIGNER" "$APK_TEST/wrong-signer.log" >/dev/null 2>&1; then
    echo "ERROR: APK verifier accepted an unexpected signer certificate" >&2
    exit 1
  fi
  cp "$APK_TEST/modern.apk" "$APK_TEST/tampered.apk"
  (cd "$SYNTHETIC_DIR/tamper" && zip -q "$APK_TEST/tampered.apk" appended.txt)
  if verify_android_apk_signature "$APK_TEST/tampered.apk" "$SYNTHETIC_DIGEST" \
    "$APK_SIGNER" "$APK_TEST/tampered.log" >/dev/null 2>&1; then
    echo "ERROR: APK verifier accepted a payload changed after signing" >&2
    exit 1
  fi
else
  echo "NOTE: no Android SDK signing tools here — APK signature controls did NOT run." >&2
fi

if [ -z "$ANDROID_JAR" ] || [ -z "$D8" ] || ! command -v javac >/dev/null 2>&1; then
  echo "NOTE: no Android SDK/javac here — the JNI descriptor BYTECODE control did" >&2
  echo "      NOT run. TestJNIDescriptorsMatchBtBridge still compared the sources," >&2
  echo "      and every Android build runs the bytecode check (build-android.sh)." >&2
else
  export BIBLETEXT_ANDROID_BUILD_TOOLS="$(dirname "$D8")"
  mkdir -p "$JNI_DIR/classes-good" "$JNI_DIR/classes-skew"

  # Runs one compile or dex step with its output in a log, and on failure
  # prints that log and stops the script.
  run_jni_step() {
    local what="$1" log="$2"
    shift 2
    if ! "$@" >"$log" 2>&1; then
      echo "ERROR: $what failed:" >&2
      cat "$log" >&2
      exit 1
    fi
  }

  # The sources the bridge is compiled with: android/*.java but BtBridge.java,
  # which each run supplies, the tree's own or the skewed copy.
  BRIDGE_PEERS=()
  for source in android/*.java; do
    [ "$source" = android/BtBridge.java ] || BRIDGE_PEERS+=("$source")
  done
  if [ "${#BRIDGE_PEERS[@]}" -eq 0 ]; then
    echo "ERROR: no android/*.java beside BtBridge.java; the glob or the tree moved" >&2
    exit 1
  fi

  run_jni_step "javac of android/*.java" "$JNI_DIR/javac-good.log" \
    javac -nowarn --release 17 -classpath "$ANDROID_JAR" \
    -d "$JNI_DIR/classes-good" android/BtBridge.java "${BRIDGE_PEERS[@]}"
  run_jni_step "d8 of the compiled android/*.java" "$JNI_DIR/d8-good.log" \
    "$D8" --min-api 24 --output "$JNI_DIR" \
    $(find "$JNI_DIR/classes-good" -name '*.class')
  mv "$JNI_DIR/classes.dex" "$JNI_DIR/good.dex"
  verify_android_jni_descriptors "$JNI_DIR/good.dex" reading_android.go

  # The control. Without it a verifier that silently matched nothing would pass
  # the line above and prove exactly as much as no test at all.
  #
  # The skew APPENDS an unused parameter rather than removing one: an extra
  # parameter still compiles (the body never names it) while changing the
  # descriptor, which is precisely what has to be caught. Dropping one instead
  # would fail at javac and prove nothing about the verifier.
  #
  # STRUCTURAL, not a literal. This used to name setNote's trailing parameter,
  # and every appended field since made the sed match nothing — the guard below
  # then failed the build to say so, which is the right outcome and a poor use
  # of a CI run. The signature is parsed instead, so it survives the next four.
  python3 - "$JNI_DIR/BtBridgeSkew.java" <<'PY_SKEW'
import pathlib, re, sys
src = pathlib.Path("android/BtBridge.java").read_text()
m = re.search(r"(public static void setNote\()(.*?)(\)\s*\{)", src, re.S)
if m is None:
    raise SystemExit("setNote's declaration could not be parsed; the skew control is blind")
skewed = src[:m.end(2)] + ", final boolean spare" + src[m.end(2):]
if skewed == src:
    raise SystemExit("the skew edited nothing")
pathlib.Path(sys.argv[1]).write_text(skewed)
PY_SKEW
  mv "$JNI_DIR/BtBridgeSkew.java" "$JNI_DIR/BtBridge.java"
  if cmp -s android/BtBridge.java "$JNI_DIR/BtBridge.java"; then
    echo "ERROR: the JNI skew control edited nothing, so it is not a control" >&2
    exit 1
  fi
  run_jni_step "javac of the skewed BtBridge.java with android/*.java" \
    "$JNI_DIR/javac-skew.log" \
    javac -nowarn --release 17 -classpath "$ANDROID_JAR" \
    -d "$JNI_DIR/classes-skew" "$JNI_DIR/BtBridge.java" "${BRIDGE_PEERS[@]}"
  run_jni_step "d8 of the skewed compile" "$JNI_DIR/d8-skew.log" \
    "$D8" --min-api 24 --output "$JNI_DIR" \
    $(find "$JNI_DIR/classes-skew" -name '*.class')
  mv "$JNI_DIR/classes.dex" "$JNI_DIR/skew.dex"
  if verify_android_jni_descriptors "$JNI_DIR/skew.dex" reading_android.go \
    >/dev/null 2>&1; then
    echo "ERROR: JNI descriptor verifier accepted a dex whose setNote lost a parameter" >&2
    exit 1
  fi
fi

python3 - scripts/build-android.sh <<'PY'
from pathlib import Path
import sys

data = Path(sys.argv[1]).read_text(encoding="utf-8")
start = data.index("  fyne release -os android")
end = data.index("\n  AAB_STAGE=", start)
invocation = data[start:end]
if '-keyStorePass' in invocation or '-keyPass' in invocation:
    raise SystemExit("Fyne release invocation exposes the keystore password in argv")
if '< "$KS_PASS_FILE"' not in invocation:
    raise SystemExit("Fyne release invocation no longer reads its password from stdin")
if '$(<"$KS_PASS_FILE")' in data:
    raise SystemExit("Android build wrapper expands the keystore password into argv")
PY

echo "Android target/signing regression tests passed."
