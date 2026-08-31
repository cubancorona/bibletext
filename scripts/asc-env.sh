# App Store Connect API credentials, resolved from the login Keychain.
#
#   . scripts/asc-env.sh        # then run the appstore tools
#
# WHY THIS EXISTS. The three values the API needs — issuer id, key id, and the
# path to the .p8 signing key — were spread across the Keychain and a directory
# nobody had written down, so every release began by hunting for them. The
# hunting is the problem this removes; the values themselves stay out of the
# repository, which is the reason they were never written down here.
#
# NOTHING IS PRINTED. The issuer id is a credential and the key path names the
# key id in its filename, so this reports success without echoing either. The
# repository hygiene check fails the build if that filename ever lands in a
# tracked file, which is how the first version of this was caught.
#
# Sourced, not executed: it exports into the calling shell.

case "$-" in
  *x*)
    echo "ERROR: disable shell tracing before loading App Store Connect credentials." >&2
    return 1
    ;;
esac

_asc_kc() {
  security find-generic-password -s uk.co.bibletext.appstoreconnect -a "$1" -w 2>/dev/null || true
}

# An already-exported value wins, so a one-off key can be used without touching
# the Keychain — the same escape hatch release-bible-key.sh gives.
[ -n "${ASC_ISSUER_ID:-}" ] || ASC_ISSUER_ID="$(_asc_kc issuer-id)"
[ -n "${ASC_KEY_ID:-}" ]    || ASC_KEY_ID="$(_asc_kc key-id)"
[ -n "${ASC_KEY_PATH:-}" ]  || ASC_KEY_PATH="$(_asc_kc key-path)"

unset -f _asc_kc

if [ -z "$ASC_ISSUER_ID" ] || [ -z "$ASC_KEY_ID" ] || [ -z "$ASC_KEY_PATH" ]; then
  echo "ERROR: App Store Connect credentials are not available." >&2
  echo "Expected login-Keychain items under service uk.co.bibletext.appstoreconnect" >&2
  echo "with accounts: issuer-id, key-id, key-path." >&2
  unset ASC_ISSUER_ID ASC_KEY_ID ASC_KEY_PATH
  return 1
fi

# Expand a leading ~ the Keychain stored literally.
case "$ASC_KEY_PATH" in
  "~/"*) ASC_KEY_PATH="$HOME/${ASC_KEY_PATH#\~/}" ;;
esac

if [ ! -r "$ASC_KEY_PATH" ]; then
  echo "ERROR: the App Store Connect signing key named by the Keychain is not readable." >&2
  unset ASC_ISSUER_ID ASC_KEY_ID ASC_KEY_PATH
  return 1
fi

# A private key readable by anyone else is a finding, not a warning.
_asc_mode="$(stat -f '%Lp' "$ASC_KEY_PATH" 2>/dev/null || echo '')"
case "$_asc_mode" in
  600|400) : ;;
  *)
    echo "ERROR: the signing key is mode ${_asc_mode:-unknown}; it must be 600 or 400." >&2
    unset ASC_ISSUER_ID ASC_KEY_ID ASC_KEY_PATH _asc_mode
    return 1
    ;;
esac
unset _asc_mode

export ASC_ISSUER_ID ASC_KEY_ID ASC_KEY_PATH
echo "App Store Connect credentials loaded (key id ${ASC_KEY_ID})."
