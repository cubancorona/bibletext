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
