#!/usr/bin/env bash
# Regression coverage for the release-key helper, Android Go wrapper, and
# packaged-artifact verifier. All credentials and artifacts in this test are
# synthetic; the test never consults a developer Keychain or local env file.
set -euo pipefail
umask 077

# A developer may run this from a shell that contains real credentials. Remove
# them before resolving paths, creating subprocesses, or enabling test tracing.
unset BIBLE_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
unset BIBLETEXT_BUNDLED_KEY_ENC
unset GOFLAGS GODEBUG

# The wrapper execs this script as a fake Go binary. Emit its exact arguments
# as JSON and prove that the reversible linker value was removed from the
# child environment before exec.
if [ "${BIBLETEXT_TEST_CAPTURE:-}" = "1" ]; then
  if [ "${BIBLETEXT_RELEASE_LDFLAGS+x}" = "x" ]; then
    echo "ERROR: release linker value reached the Go child environment." >&2
    exit 1
  fi
  python3 - "$@" <<'PY'
import json
import sys

print(json.dumps(sys.argv[1:]))
PY
  exit 0
fi

unset BIBLETEXT_RELEASE_LDFLAGS BIBLETEXT_REAL_GO BIBLE_KEY_LDFLAGS

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="$SCRIPT_DIR/release-bible-key.sh"
WRAPPER="$SCRIPT_DIR/go-release-wrapper.sh"
VERIFIER="$SCRIPT_DIR/verify-release-key.py"
PACKAGE_VERIFIER="$SCRIPT_DIR/verify-release-package.sh"
ANDROID_RELEASE="$SCRIPT_DIR/build-android.sh"
IOS_RELEASE="$SCRIPT_DIR/release-ios.sh"
IOS_SIM="$SCRIPT_DIR/run-ios-sim.sh"
DESKTOP_RELEASE="$SCRIPT_DIR/../.github/workflows/release.yml"
SELF="$SCRIPT_DIR/$(basename "${BASH_SOURCE[0]}")"
TEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/bibletext-release-key-test.XXXXXX")"
trap 'rm -rf "$TEST_TMP"' EXIT

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

assert_contains() {
  local path="$1"
  local expected="$2"
  if ! grep -Fq -- "$expected" "$path"; then
    fail "expected output was absent from $(basename "$path")"
  fi
}

assert_absent() {
  local path="$1"
  local forbidden="$2"
  if grep -Fq -- "$forbidden" "$path"; then
    fail "sensitive synthetic value appeared in $(basename "$path")"
  fi
}

assert_json_args() {
  local path="$1"
  shift
  python3 - "$path" "$@" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    actual = json.load(source)
expected = sys.argv[2:]
if actual != expected:
    raise SystemExit(f"argument mismatch: expected {expected!r}, got {actual!r}")
PY
}

for required in "$HELPER" "$WRAPPER" "$VERIFIER" "$PACKAGE_VERIFIER" \
  "$ANDROID_RELEASE" "$IOS_RELEASE" "$IOS_SIM" "$DESKTOP_RELEASE"; do
  [ -f "$required" ] || fail "required release component is missing"
done

# An exported raw key must be consumed and unset before any command
# substitution can copy the environment into a child. Keep repository-path
# resolution and every public-contact/repository gate after that boundary.
python3 - "$ANDROID_RELEASE" "$IOS_RELEASE" <<'PY'
from pathlib import Path
import sys

for raw_path in sys.argv[1:]:
    path = Path(raw_path)
    data = path.read_text(encoding="utf-8")
    load = data.index("load_release_bible_key")
    root = data.index('REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"')
    contact = data.index("check-support-contact.py")
    hygiene = data.index("check-repository-hygiene.py")
    if "$(" in data[:load]:
        raise SystemExit(f"{path.name}: command substitution occurs before release-key isolation")
    if not load < root < contact < hygiene:
        raise SystemExit(f"{path.name}: release-key isolation/gate ordering changed")

android = Path(sys.argv[1]).read_text(encoding="utf-8")
saved_unset = android.index("unset bibletext_saved_release_ldflags")
saved = android.index('bibletext_saved_release_ldflags="$BIBLE_KEY_LDFLAGS"')
toolchain = android.index('source "$HOME/Library/Android/env.sh"')
raw_reset = android.index(
    "unset BIBLE_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY",
    toolchain,
)
intermediate_reset = android.index(
    "unset BIBLE_KEY_LDFLAGS BIBLETEXT_RELEASE_LDFLAGS BIBLETEXT_REAL_GO",
    raw_reset,
)
restored = android.index(
    'BIBLE_KEY_LDFLAGS="$bibletext_saved_release_ldflags"', intermediate_reset
)
saved_clear = android.index('bibletext_saved_release_ldflags=""', restored)
if not saved_unset < saved < toolchain < raw_reset < intermediate_reset < restored < saved_clear:
    raise SystemExit("build-android.sh: toolchain credential re-sanitization ordering changed")
if "$(" in android[toolchain + len('source "$HOME/Library/Android/env.sh"'):raw_reset]:
    raise SystemExit("build-android.sh: subprocess occurs before post-toolchain credential sanitization")
if "export BIBLE_KEY_LDFLAGS" in android:
    raise SystemExit("build-android.sh: isolated linker value became exported")
debug = android[android.index('note "fyne package -os android (debug APK)"'):]
for required in (
    'BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS"',
    'PATH="$WORK/bin:$PATH"',
    'verify-release-key.py" BibleText.apk',
):
    if required not in debug:
        raise SystemExit("build-android.sh: debug APK no longer injects and verifies the release key")
if "unset BIBLE_API_KEY" in android[:android.index("REPO_ROOT=")]:
    raise SystemExit("build-android.sh: debug path restored a keyless pre-build branch")
PY

# Simulator builds are development artifacts, but they follow the same
# externally sourced fallback rule as every other build. They must fail closed
# before installation rather than retaining Fyne's initial keyless executable.
python3 - "$IOS_SIM" <<'PY'
from pathlib import Path
import sys

path = Path(sys.argv[1])
data = path.read_text(encoding="utf-8")
load = data.index("load_release_bible_key")
root = data.index('REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"')
build = data.index("go build -trimpath")
flags = data.index('$BIBLE_KEY_LDFLAGS -w', build)
verify_first = data.index("verify-release-key.py", flags)
clear = data.index("clear_release_bible_key", verify_first)
install = data.index('simctl install "$BOOTED" "$APP"', clear)
if "$(" in data[:load]:
    raise SystemExit("run-ios-sim.sh: command substitution occurs before release-key isolation")
if not load < root < build < flags < verify_first < clear < install:
    raise SystemExit("run-ios-sim.sh: keyed build/verification/install ordering changed")
if "continuing with fyne's binary" in data:
    raise SystemExit("run-ios-sim.sh: a keyless simulator fallback was restored")
if 'mv "$APP_DIR/$APP_NAME" "$PREEXISTING_APP"' not in data or \
        'mv "$PREEXISTING_APP" "$APP_DIR/$APP_NAME"' not in data:
    raise SystemExit("run-ios-sim.sh: pre-existing generated app is not preserved")
PY

# Desktop jobs receive only the encoded payload through an Actions secret and
# must inject and verify it for every published architecture. The raw project
# key must never be configured on a GitHub runner.
python3 - "$DESKTOP_RELEASE" "$PACKAGE_VERIFIER" <<'PY'
from pathlib import Path
import sys

workflow = Path(sys.argv[1]).read_text(encoding="utf-8")
verifier = Path(sys.argv[2]).read_text(encoding="utf-8")

encoded_secret = "BIBLETEXT_BUNDLED_KEY_ENC: ${{ secrets.BIBLETEXT_BUNDLED_KEY_ENC }}"
if workflow.count(encoded_secret) != 3:
    raise SystemExit("release.yml: every desktop job must receive the encoded secret")
if "secrets.BIBLE_API_KEY" in workflow:
    raise SystemExit("release.yml: raw API.Bible credential must not reach GitHub Actions")
if workflow.count("load_encoded_release_bible_key") != 3:
    raise SystemExit("release.yml: every desktop job must isolate the encoded secret")
if workflow.count('go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w"') != 4:
    raise SystemExit("release.yml: every desktop architecture must receive the linker value")
verified = (
    'BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" '
    "../../scripts/verify-release-package.sh"
)
if workflow.count(verified) != 4:
    raise SystemExit("release.yml: every packaged desktop executable must be key-verified")
if '-ldflags=$BIBLE_KEY_LDFLAGS' not in workflow:
    raise SystemExit("release.yml: Windows resource rebuild must preserve the linker value")
if 'verify-release-key.py" "$binary_path"' not in verifier:
    raise SystemExit("verify-release-package.sh: packaged key verification is missing")
PY

# shellcheck source=release-bible-key.sh
source "$HELPER"

fixture_key="fixture-release-key-not-a-real-credential"

# An imported variable keeps its export attribute across ordinary Bash
# assignment. Prove the helper first removes that attribute, so the reversible
# linker value cannot become ambient in unrelated children.
(
  export BIBLE_KEY_LDFLAGS="fixture-stale-export"
  export BIBLE_API_KEY="$fixture_key"
  source "$HELPER"
  load_release_bible_key
  if bash -c '[ "${BIBLE_KEY_LDFLAGS+x}" = x ]'; then
    fail "helper exported the release linker value to a child"
  fi
  clear_release_bible_key
  if bash -c '[ "${BIBLE_KEY_LDFLAGS+x}" = x ]'; then
    fail "helper retained an exported linker variable after cleanup"
  fi
)

export BIBLE_API_KEY="$fixture_key"
load_release_bible_key

[ "${BIBLE_API_KEY+x}" != "x" ] || fail "helper retained BIBLE_API_KEY"
case "$BIBLE_KEY_LDFLAGS" in
  -X=github.com/cubancorona/bibletext.bundledBibleKeyEnc=*) ;;
  *) fail "helper did not produce the expected linker assignment" ;;
esac
release_flags="$BIBLE_KEY_LDFLAGS"
marker="${release_flags#-X=github.com/cubancorona/bibletext.bundledBibleKeyEnc=}"
[ "${#marker}" -ge 16 ] || fail "helper produced a malformed linker value"
case "$release_flags" in
  *"$fixture_key"*) fail "helper retained the raw synthetic key in linker flags" ;;
esac

clear_release_bible_key
[ -z "$BIBLE_KEY_LDFLAGS" ] || fail "clear_release_bible_key retained linker flags"
[ "${BIBLE_API_KEY+x}" != "x" ] || fail "clear_release_bible_key retained BIBLE_API_KEY"

# GitHub Actions stores only the already-transformed binary payload. Prove the
# helper consumes it without exporting it to unrelated children and reconstructs
# the exact same linker assignment as the raw-key path.
(
  export BIBLETEXT_BUNDLED_KEY_ENC="$marker"
  source "$HELPER"
  load_encoded_release_bible_key
  [ "$BIBLE_KEY_LDFLAGS" = "$release_flags" ] || \
    fail "encoded helper changed the linker assignment"
  [ "${BIBLETEXT_BUNDLED_KEY_ENC+x}" != "x" ] || \
    fail "encoded helper retained its Actions input"
  if bash -c '[ "${BIBLE_KEY_LDFLAGS+x}" = x ]'; then
    fail "encoded helper exported the linker value to a child"
  fi
  clear_release_bible_key
)

if BIBLETEXT_BUNDLED_KEY_ENC="not-valid-base64!" \
  load_encoded_release_bible_key >"$TEST_TMP/helper-encoded-malformed.log" 2>&1; then
  fail "encoded helper accepted malformed input"
fi
assert_contains "$TEST_TMP/helper-encoded-malformed.log" "is malformed"
assert_absent "$TEST_TMP/helper-encoded-malformed.log" "$marker"

if BIBLETEXT_BUNDLED_KEY_ENC="$marker" bash -x -c \
  'source "$1"; load_encoded_release_bible_key' _ "$HELPER" \
  >"$TEST_TMP/helper-encoded-xtrace.log" 2>&1; then
  fail "encoded helper accepted shell tracing"
fi
assert_contains "$TEST_TMP/helper-encoded-xtrace.log" "disable shell tracing"
assert_absent "$TEST_TMP/helper-encoded-xtrace.log" "$marker"

# Compile the real package test binary with the synthetic linker assignment and
# execute its production decoder. Marker-only inspection cannot prove that the
# linker found the intended symbol or that runtime decoding works.
LINKED_TEST="$TEST_TMP/bibletext-linked-key.test"
(
  cd "$SCRIPT_DIR/.."
  go test -c -trimpath -ldflags "$release_flags" -o "$LINKED_TEST" .
)
BIBLETEXT_TEST_LINKED_KEY="$fixture_key" \
  "$LINKED_TEST" -test.run '^TestLinkedBundledKey$' \
  >"$TEST_TMP/linked-runtime.log"
assert_contains "$TEST_TMP/linked-runtime.log" "PASS"
assert_absent "$TEST_TMP/linked-runtime.log" "$fixture_key"
assert_absent "$TEST_TMP/linked-runtime.log" "$marker"

GITHUB_WORKSPACE="$TEST_TMP/runner/_work/bibletext/bibletext" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$PACKAGE_VERIFIER" "$LINKED_TEST" "$LINKED_TEST" \
  >"$TEST_TMP/package-verifier.log"
assert_contains "$TEST_TMP/package-verifier.log" "Verified keyed release value"
assert_absent "$TEST_TMP/package-verifier.log" "$marker"

# Shadow the macOS utility so the missing-key case cannot inspect a real login
# Keychain, even when the test is run on a developer Mac.
security() {
  return 1
}
if load_release_bible_key >"$TEST_TMP/helper-missing.log" 2>&1; then
  fail "helper accepted a missing release credential"
fi
assert_contains "$TEST_TMP/helper-missing.log" "release API.Bible key unavailable"
assert_absent "$TEST_TMP/helper-missing.log" "$fixture_key"
assert_absent "$TEST_TMP/helper-missing.log" "$marker"
unset -f security

# Tracing must fail before the helper reads or expands the invoking credential.
if BIBLE_API_KEY="$fixture_key" bash -x -c \
  'source "$1"; load_release_bible_key' _ "$HELPER" \
  >"$TEST_TMP/helper-xtrace.log" 2>&1; then
  fail "helper accepted shell tracing"
fi
assert_contains "$TEST_TMP/helper-xtrace.log" "disable shell tracing"
assert_absent "$TEST_TMP/helper-xtrace.log" "$fixture_key"
assert_absent "$TEST_TMP/helper-xtrace.log" "$marker"

# Fyne may supply either supported Go -ldflags spelling. The wrapper must merge
# the release assignment exactly once while preserving every other argument.
BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -trimpath -buildmode=c-shared -ldflags "-w" ./cmd/mobile \
  >"$TEST_TMP/wrapper-separated.json"
assert_json_args "$TEST_TMP/wrapper-separated.json" \
  build -trimpath -buildmode=c-shared -ldflags "-w $release_flags" ./cmd/mobile

BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -trimpath=true -buildmode=c-shared "-ldflags=-s -w" ./cmd/mobile \
  >"$TEST_TMP/wrapper-equals.json"
assert_json_args "$TEST_TMP/wrapper-equals.json" \
  build -trimpath=true -buildmode=c-shared "-ldflags=-s -w $release_flags" ./cmd/mobile

# Fyne's debug Android packager does not pass -trimpath. The wrapper must add
# it before the package path rather than producing an untrimmed keyed binary.
BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -buildmode=c-shared -ldflags=-w ./cmd/mobile \
  >"$TEST_TMP/wrapper-trimpath-injected.json"
assert_json_args "$TEST_TMP/wrapper-trimpath-injected.json" \
  build -trimpath -buildmode=c-shared "-ldflags=-w $release_flags" ./cmd/mobile

# Fyne's debug package command also supplies no -ldflags at all. The wrapper
# must inject the isolated assignment as an argument, never ambient state.
BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -buildmode=c-shared -o fixture.so ./cmd/mobile \
  >"$TEST_TMP/wrapper-debug-fyne.json"
assert_json_args "$TEST_TMP/wrapper-debug-fyne.json" \
  build -ldflags "$release_flags" -trimpath -buildmode=c-shared -o fixture.so ./cmd/mobile

# An explicit opt-out is different from Fyne omitting the flag and remains a
# hard failure.
if BIBLETEXT_TEST_CAPTURE=1 \
  BIBLETEXT_REAL_GO="$SELF" \
  BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -trimpath=false -buildmode=c-shared -ldflags=-w ./cmd/mobile \
  >"$TEST_TMP/wrapper-trimpath-false.log" 2>&1; then
  fail "wrapper accepted a keyed shared build with -trimpath=false"
fi
assert_contains "$TEST_TMP/wrapper-trimpath-false.log" "forbid -trimpath=false"
assert_absent "$TEST_TMP/wrapper-trimpath-false.log" "$marker"

# Empty release flags and non-shared invocations are strict pass-throughs.
BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="" \
  "$WRAPPER" env GOOS android \
  >"$TEST_TMP/wrapper-empty.json"
assert_json_args "$TEST_TMP/wrapper-empty.json" env GOOS android

BIBLETEXT_TEST_CAPTURE=1 \
BIBLETEXT_REAL_GO="$SELF" \
BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -trimpath -ldflags=-w ./cmd/desktop \
  >"$TEST_TMP/wrapper-nonshared.json"
assert_json_args "$TEST_TMP/wrapper-nonshared.json" \
  build -trimpath -ldflags=-w ./cmd/desktop

python3 - "$TEST_TMP" "$marker" <<'PY'
from pathlib import Path
import sys
import zipfile

root = Path(sys.argv[1])
marker = sys.argv[2].encode("ascii", "strict")
keyed = b"\x7fELF synthetic keyed payload\0" + marker + b"\0end"
unkeyed = b"\x7fELF synthetic unkeyed payload\0end"

with zipfile.ZipFile(root / "positive.apk", "w", zipfile.ZIP_DEFLATED) as artifact:
    artifact.writestr("lib/arm64-v8a/libBibleText.so", keyed)
    artifact.writestr("lib/x86_64/libBibleText.so", keyed)

with zipfile.ZipFile(root / "partial.aab", "w", zipfile.ZIP_DEFLATED) as artifact:
    artifact.writestr("base/lib/arm64-v8a/libBibleText.so", keyed)
    artifact.writestr("base/lib/x86_64/libBibleText.so", unkeyed)

with zipfile.ZipFile(root / "no-native.apk", "w", zipfile.ZIP_DEFLATED) as artifact:
    artifact.writestr("assets/synthetic.txt", b"not a native payload")
PY

BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  python3 "$VERIFIER" "$TEST_TMP/positive.apk" \
  >"$TEST_TMP/verifier-positive.log"
assert_contains "$TEST_TMP/verifier-positive.log" "2 native payloads"

if BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  python3 "$VERIFIER" "$TEST_TMP/partial.aab" \
  >"$TEST_TMP/verifier-partial.log" 2>&1; then
  fail "verifier accepted a partially keyed multi-ABI artifact"
fi
assert_contains "$TEST_TMP/verifier-partial.log" "absent from 1 of 2 native payloads"
assert_absent "$TEST_TMP/verifier-partial.log" "$marker"

if BIBLETEXT_RELEASE_LDFLAGS="-X=github.com/cubancorona/bibletext.bundledBibleKeyEnc=short" \
  python3 "$VERIFIER" "$TEST_TMP/positive.apk" \
  >"$TEST_TMP/verifier-malformed.log" 2>&1; then
  fail "verifier accepted a malformed release linker value"
fi
assert_contains "$TEST_TMP/verifier-malformed.log" "release linker value is malformed"

if BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  python3 "$VERIFIER" "$TEST_TMP/no-native.apk" \
  >"$TEST_TMP/verifier-no-native.log" 2>&1; then
  fail "verifier accepted an artifact without a native payload"
fi
assert_contains "$TEST_TMP/verifier-no-native.log" "contains no native payload"

echo "Release-key flow regression tests passed."
