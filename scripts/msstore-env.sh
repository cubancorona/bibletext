# Microsoft Store submission API credentials, resolved from the login Keychain.
#
#   . scripts/msstore-env.sh    # then run msstore/msstore.py
#
# The three values the API needs — the Entra tenant id, the application's
# client id, and its key — live as login-Keychain items under the service
# uk.co.bibletext.msstore (accounts tenant-id, client-id, client-secret), the
# way the App Store Connect ones do under uk.co.bibletext.appstoreconnect.
# The key is shown once when it is created in Partner Center (Account
# settings → User management → Microsoft Entra applications → the app →
# Add new key) and goes straight into the Keychain; it is never written down.
#
# NO CREDENTIAL IS PRINTED. Sourced, not executed: it exports into the calling shell.

case "$-" in
  *x*)
    echo "ERROR: disable shell tracing before loading Microsoft Store credentials." >&2
    return 1
    ;;
esac

_msstore_kc() {
  security find-generic-password -s uk.co.bibletext.msstore -a "$1" -w 2>/dev/null || true
}

# An already-exported value wins, so a one-off key can be used without touching
# the Keychain — the same escape hatch asc-env.sh gives.
[ -n "${MSSTORE_TENANT_ID:-}" ]     || MSSTORE_TENANT_ID="$(_msstore_kc tenant-id)"
[ -n "${MSSTORE_CLIENT_ID:-}" ]     || MSSTORE_CLIENT_ID="$(_msstore_kc client-id)"
[ -n "${MSSTORE_CLIENT_SECRET:-}" ] || MSSTORE_CLIENT_SECRET="$(_msstore_kc client-secret)"

unset -f _msstore_kc

if [ -z "$MSSTORE_TENANT_ID" ] || [ -z "$MSSTORE_CLIENT_ID" ] || [ -z "$MSSTORE_CLIENT_SECRET" ]; then
  echo "ERROR: Microsoft Store credentials are not available." >&2
  echo "Expected login-Keychain items under service uk.co.bibletext.msstore" >&2
  echo "with accounts: tenant-id, client-id, client-secret." >&2
  unset MSSTORE_TENANT_ID MSSTORE_CLIENT_ID MSSTORE_CLIENT_SECRET
  return 1
fi

export MSSTORE_TENANT_ID MSSTORE_CLIENT_ID MSSTORE_CLIENT_SECRET
echo "Microsoft Store credentials loaded."
