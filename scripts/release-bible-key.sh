#!/usr/bin/env bash
# Prepare the linker value for a keyed BibleText release without writing a
# credential or reversible derivative anywhere under the repository.
#
# Credential sources, in order:
#
#   1. BIBLE_API_KEY in the invoking process environment; or
#   2. the macOS login Keychain item named uk.co.bibletext.apibible-release,
#      account "release".
#
# GitHub's desktop builders use load_encoded_release_bible_key instead. Their
# encrypted Actions secret contains only the same reversible encoding that is
# written into a release executable, never the raw API.Bible credential.
#
# This helper never reads .env.local. That file may contain unrelated personal
# provider keys which must not enter a release process.

# Remove both any inherited value and its export attribute. A plain assignment
# to an imported variable would otherwise leave the reversible linker value in
# every later subprocess environment.
unset BIBLE_KEY_LDFLAGS
BIBLE_KEY_LDFLAGS=""

load_release_bible_key() {
  local key=""
  local encoded=""

  # Shell tracing expands assignments and linker arguments, which would put
  # the credential or its reversible release form in a terminal/log.
  case "$-" in
    *x*)
      echo "ERROR: disable shell tracing before loading the release credential." >&2
      return 1
      ;;
  esac

  key="${BIBLE_API_KEY:-}"
  unset BIBLE_API_KEY

  if [ -z "$key" ] && command -v security >/dev/null 2>&1; then
    key="$(security find-generic-password \
      -a release -s uk.co.bibletext.apibible-release -w 2>/dev/null || true)"
  fi

  if [ -z "$key" ]; then
    echo "ERROR: release API.Bible key unavailable." >&2
    echo "Set BIBLE_API_KEY for this process or add the dedicated login-Keychain item." >&2
    return 1
  fi

  if [ "${#key}" -lt 16 ] || [ "${#key}" -gt 512 ]; then
    unset key
    echo "ERROR: release API.Bible key has an invalid length." >&2
    return 1
  fi
  case "$key" in
    *[[:space:]]*)
      unset key
      echo "ERROR: release API.Bible key contains whitespace." >&2
      return 1
      ;;
  esac

  # Pass the key over stdin, never argv. The result is obfuscation, not
  # encryption: a shipped client necessarily contains a recoverable key.
  if ! encoded="$(printf '%s' "$key" | python3 -c '
import base64, sys
key = sys.stdin.buffer.read()
mask = b"bibletext-nkjv"
print(base64.b64encode(bytes(b ^ mask[i % len(mask)] for i, b in enumerate(key))).decode())
')"; then
    unset key encoded
    echo "ERROR: failed to prepare the release API.Bible linker value." >&2
    return 1
  fi

  unset key BIBLE_API_KEY
  if [ -z "$encoded" ]; then
    echo "ERROR: failed to prepare the release API.Bible linker value." >&2
    return 1
  fi

  BIBLE_KEY_LDFLAGS="-X=bibletext.bundledBibleKeyEnc=$encoded"
  unset encoded
}

load_encoded_release_bible_key() {
  local encoded=""

  case "$-" in
    *x*)
      echo "ERROR: disable shell tracing before loading the encoded release credential." >&2
      return 1
      ;;
  esac

  encoded="${BIBLETEXT_BUNDLED_KEY_ENC:-}"
  unset BIBLETEXT_BUNDLED_KEY_ENC

  if [ -z "$encoded" ]; then
    echo "ERROR: encoded release API.Bible key unavailable." >&2
    return 1
  fi
  if [ "${#encoded}" -lt 16 ] || [ "${#encoded}" -gt 1024 ]; then
    unset encoded
    echo "ERROR: encoded release API.Bible key has an invalid length." >&2
    return 1
  fi
  case "$encoded" in
    *[!A-Za-z0-9+/=]*)
      unset encoded
      echo "ERROR: encoded release API.Bible key is malformed." >&2
      return 1
      ;;
  esac

  # Validate without decoding to stdout or persisting another copy. The
  # decoded size bounds match the raw-key helper above.
  if ! printf '%s' "$encoded" | python3 -c '
import base64, binascii, sys
try:
    value = base64.b64decode(sys.stdin.buffer.read(), validate=True)
except (binascii.Error, ValueError):
    raise SystemExit(1)
raise SystemExit(0 if 16 <= len(value) <= 512 else 1)
'; then
    unset encoded
    echo "ERROR: encoded release API.Bible key is malformed." >&2
    return 1
  fi

  BIBLE_KEY_LDFLAGS="-X=bibletext.bundledBibleKeyEnc=$encoded"
  unset encoded
}

clear_release_bible_key() {
  unset BIBLE_KEY_LDFLAGS
  BIBLE_KEY_LDFLAGS=""
  unset BIBLE_API_KEY BIBLETEXT_BUNDLED_KEY_ENC
}
