#!/usr/bin/env bash
# Regenerate the Android-only Fyne CLI copy used by build-android.sh.
#
# fyne.io/tools v1.7.2 hardcodes target SDK 29 for debug packages and 35 for
# release packages. BibleText re-signs its debug APK with current signature
# schemes, so both packaging modes can and must target API 36. The tiny patch
# also lets the wrapper pin the exact API-36 platform and build-tools paths.
set -euo pipefail

cd "$(dirname "$0")/.."

TOOLS_VERSION="v1.7.2"
TOOLS_SUM="h1:+uDZ3uOPVfdcOGRxzTI7uwBj7y3VRzz9qwntZ59J62M="
TOOLS_GOMOD_SUM="h1:MOPy1Z0+abfaOOyFxFqiuVuKx587jlfprGANBcOqvO0="
PATCH="patches/fyne-tools-1.7.2-android-api-36.patch"
DEST="third_party/fyne-tools"
FETCH_DIR="$(mktemp -d "${TMPDIR:-/tmp}/bibletext-fyne-tools.XXXXXX")"
trap 'rm -rf -- "$FETCH_DIR"' EXIT

# Always rebuild from the checksum-verified module zip. The extracted Go module
# cache is writable and may be stale or locally altered, so it is not a trusted
# patch input even when `go mod download` reports the expected version.
DOWNLOAD_JSON="$(go mod download -json "fyne.io/tools@${TOOLS_VERSION}")"
ACTUAL_SUM="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("Sum", ""))' \
  <<<"$DOWNLOAD_JSON")"
ACTUAL_GOMOD_SUM="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("GoModSum", ""))' \
  <<<"$DOWNLOAD_JSON")"
MODULE_ZIP="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("Zip", ""))' \
  <<<"$DOWNLOAD_JSON")"
[ "$ACTUAL_SUM" = "$TOOLS_SUM" ] || {
  echo "ERROR: fyne.io/tools ${TOOLS_VERSION} module checksum mismatch" >&2
  exit 1
}
[ "$ACTUAL_GOMOD_SUM" = "$TOOLS_GOMOD_SUM" ] || {
  echo "ERROR: fyne.io/tools ${TOOLS_VERSION} go.mod checksum mismatch" >&2
  exit 1
}
[ -s "$MODULE_ZIP" ] || {
  echo "ERROR: verified fyne.io/tools ${TOOLS_VERSION} module zip is unavailable" >&2
  exit 1
}
ZIP_SUM="$(python3 - "$MODULE_ZIP" <<'PY'
import base64
import hashlib
import sys
import zipfile

names = []
contents = {}
with zipfile.ZipFile(sys.argv[1]) as archive:
    for item in archive.infolist():
        names.append(item.filename)
        contents[item.filename] = archive.read(item)

manifest = hashlib.sha256()
for name in sorted(names):
    manifest.update(
        f"{hashlib.sha256(contents[name]).hexdigest()}  {name}\n".encode("utf-8")
    )
print("h1:" + base64.b64encode(manifest.digest()).decode("ascii"))
PY
)"
[ "$ZIP_SUM" = "$TOOLS_SUM" ] || {
  echo "ERROR: fyne.io/tools ${TOOLS_VERSION} module zip content is corrupt" >&2
  exit 1
}

unzip -q "$MODULE_ZIP" -d "$FETCH_DIR/source"
SOURCE="$FETCH_DIR/source/fyne.io/tools@${TOOLS_VERSION}"
[ -d "$SOURCE" ] || {
  echo "ERROR: verified fyne.io/tools ${TOOLS_VERSION} zip has an unexpected layout" >&2
  exit 1
}

echo "Regenerating ${DEST} from fyne.io/tools ${TOOLS_VERSION} ..."
rm -rf -- "$DEST"
mkdir -p third_party
cp -R "$SOURCE" "$DEST"
chmod -R u+w "$DEST"

# A dry run must match the exact pinned source without fuzzy context or line
# offsets. Suppress backup files as an additional guarantee that generated
# patch residue can never be mistaken for source by the local CLI build.
PATCH_CHECK_LOG="$FETCH_DIR/patch-check.log"
if ! patch --dry-run -p1 -F 0 -V none -d "$DEST" < "$PATCH" >"$PATCH_CHECK_LOG" 2>&1; then
  sed -n '1,160p' "$PATCH_CHECK_LOG" >&2
  echo "ERROR: Fyne tools patch does not apply to the verified ${TOOLS_VERSION} source" >&2
  exit 1
fi
if grep -Eiq 'offset|fuzz' "$PATCH_CHECK_LOG"; then
  sed -n '1,160p' "$PATCH_CHECK_LOG" >&2
  echo "ERROR: Fyne tools patch requires an offset or fuzzy context" >&2
  exit 1
fi
PATCH_APPLY_LOG="$FETCH_DIR/patch-apply.log"
if ! patch -p1 -F 0 -V none -d "$DEST" < "$PATCH" >"$PATCH_APPLY_LOG" 2>&1; then
  sed -n '1,160p' "$PATCH_APPLY_LOG" >&2
  echo "ERROR: Fyne tools patch application failed" >&2
  exit 1
fi
if grep -Eiq 'offset|fuzz' "$PATCH_APPLY_LOG"; then
  sed -n '1,160p' "$PATCH_APPLY_LOG" >&2
  echo "ERROR: Fyne tools patch applied non-deterministically" >&2
  exit 1
fi

PATCH_RESIDUE="$(find "$DEST" -type f \( -name '*.orig' -o -name '*.rej' \) -print -quit)"
[ -z "$PATCH_RESIDUE" ] || {
  echo "ERROR: Fyne tools patch left generated residue: $PATCH_RESIDUE" >&2
  exit 1
}

TARGET="$DEST/cmd/fyne/internal/mobile/build.go"
TOOLS_PICKER="$DEST/cmd/fyne/internal/util/mobile.go"
PLATFORM_PICKER="$DEST/cmd/fyne/internal/mobile/binres/sdk.go"
if ! grep -q 'androidTargetSDK = 36' "$TARGET" \
   || ! grep -q 'target := androidTargetSDK' "$TARGET"; then
  echo "ERROR: fyne.io/tools Android target-SDK patch did not apply" >&2
  exit 1
fi
if ! grep -q 'BIBLETEXT_ANDROID_BUILD_TOOLS' "$TOOLS_PICKER"; then
  echo "ERROR: fyne.io/tools build-tools pin did not apply" >&2
  exit 1
fi
if ! grep -q 'BIBLETEXT_ANDROID_PLATFORM' "$PLATFORM_PICKER"; then
  echo "ERROR: fyne.io/tools Android-platform pin did not apply" >&2
  exit 1
fi
[ -s "$DEST/cmd/fyne/internal/mobile/build_bibletext_test.go" ] \
  || { echo "ERROR: fyne.io/tools target-SDK regression test is missing" >&2; exit 1; }
[ -s "$DEST/cmd/fyne/internal/util/mobile_bibletext_test.go" ] \
  || { echo "ERROR: fyne.io/tools build-tools regression test is missing" >&2; exit 1; }
[ -s "$DEST/cmd/fyne/internal/mobile/binres/sdk_bibletext_test.go" ] \
  || { echo "ERROR: fyne.io/tools platform regression test is missing" >&2; exit 1; }

echo "OK: patched fyne.io/tools ${TOOLS_VERSION} for Android target API 36."
