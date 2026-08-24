#!/usr/bin/env bash
# Regression coverage for the release-key helper, Android Go wrapper, and
# packaged-artifact verifier. All credentials and artifacts in this test are
# synthetic; the test never consults a developer Keychain or local env file.
set -euo pipefail
umask 077

# A developer may run this from a shell that contains real credentials. Remove
# them before resolving paths, creating subprocesses, or enabling test tracing.
unset BIBLE_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
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

for required in "$HELPER" "$WRAPPER" "$VERIFIER"; do
  [ -f "$required" ] || fail "required release component is missing"
done

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
  -X=bibletext.bundledBibleKeyEnc=*) ;;
  *) fail "helper did not produce the expected linker assignment" ;;
esac
release_flags="$BIBLE_KEY_LDFLAGS"
marker="${release_flags#-X=bibletext.bundledBibleKeyEnc=}"
[ "${#marker}" -ge 16 ] || fail "helper produced a malformed linker value"
case "$release_flags" in
  *"$fixture_key"*) fail "helper retained the raw synthetic key in linker flags" ;;
esac

clear_release_bible_key
[ -z "$BIBLE_KEY_LDFLAGS" ] || fail "clear_release_bible_key retained linker flags"
[ "${BIBLE_API_KEY+x}" != "x" ] || fail "clear_release_bible_key retained BIBLE_API_KEY"

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

if BIBLETEXT_TEST_CAPTURE=1 \
  BIBLETEXT_REAL_GO="$SELF" \
  BIBLETEXT_RELEASE_LDFLAGS="$release_flags" \
  "$WRAPPER" build -buildmode=c-shared -ldflags=-w ./cmd/mobile \
  >"$TEST_TMP/wrapper-trimpath.log" 2>&1; then
  fail "wrapper accepted a keyed shared build without -trimpath"
fi
assert_contains "$TEST_TMP/wrapper-trimpath.log" "require -trimpath"
assert_absent "$TEST_TMP/wrapper-trimpath.log" "$marker"

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

if BIBLETEXT_RELEASE_LDFLAGS="-X=bibletext.bundledBibleKeyEnc=short" \
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
