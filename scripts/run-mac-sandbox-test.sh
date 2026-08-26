#!/usr/bin/env bash
# Build and launch a SANDBOXED BibleText locally, signed with an ordinary Apple
# Development certificate, to answer the two questions that cannot be answered
# after a Mac App Store submission:
#
#   1. Does the app actually work inside the sandbox — do downloads, audio and
#      links still function with only the two entitlements we request?
#   2. Does an existing reader's data survive? The container migration is the
#      difference between an updating reader keeping their notes and opening a
#      first-run app.
#
# This is NOT a submission build. It is signed for local development, not for
# distribution, and produces no .pkg. Use scripts/release-mac-store.sh for that.
#
# WHY A SEPARATE BUNDLE ID
#
# macOS consults a Container-Migration.plist exactly once: on the launch that
# CREATES the container. Testing under the shipping id would create the real
# container here and permanently spend that one chance on this Mac — a later
# genuine Store build would find the container already present and migrate
# nothing. So this build takes its own id and its own throwaway container, and
# the shipping id stays untouched.
#
# The migration source is the same either way: the unsandboxed preferences
# directory that every non-sandboxed build shares.
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"
WORK="$REPO_ROOT/build/mac-sandbox-test"
APP="$WORK/BibleText.app"
ENTITLEMENTS="$REPO_ROOT/appstore/mac/BibleText.entitlements"
MIGRATION="$REPO_ROOT/appstore/mac/Container-Migration.plist"

note() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
fail() { printf '\033[31mERROR: %s\033[0m\n' "$1" >&2; exit 1; }

SHIP_ID=$(python3 -c 'import json;print(json.load(open("config/product.json"))["desktopAppID"],end="")')
# A fresh id per run. Containers cannot be deleted (the container manager
# protects their metadata), and the migration only runs when a container is
# CREATED — so reusing one id would test the migration exactly once, ever.
# "--ship" rehearses under the REAL bundle id, which is the only way to
# exercise the container the store build will actually use. It spends that
# container's single migration chance on this Mac, so it is opt-in and the
# preferences it Moves should be backed up first.
if [ "${1:-}" = "--ship" ]; then
  TEST_ID="$SHIP_ID"
else
  SUFFIX="${1:-$(date +%H%M%S)}"
  TEST_ID="${SHIP_ID}.sbx$SUFFIX"
fi
CONTAINER="$HOME/Library/Containers/$TEST_ID"

note "checking the store configuration"
python3 scripts/check-mac-store-config.py || fail "store configuration is not shippable"

CERT=$(security find-identity -v -p codesigning 2>/dev/null |
  grep "Apple Development" | head -1 | awk '{print $2}') || true
[ -n "${CERT:-}" ] || fail "no 'Apple Development' certificate found; sign in to Xcode once"

note "building (arm64 only — this is a local test, not a release)"
rm -rf "$WORK"; mkdir -p "$WORK"
# The key is injected here for the same reason the release build injects it:
# without it bundledBibleKeyEnc is empty, so the app reports no bundled key,
# the API.Bible field reads "Paste your API.Bible key" rather than showing the
# included one, and the NKJV cannot download at all. A rehearsal that omits it
# is not rehearsing the build readers will run.
# shellcheck source=/dev/null
source scripts/release-bible-key.sh
load_release_bible_key
trap clear_release_bible_key EXIT
(
  cd cmd/desktop
  CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o "$WORK/desktop" .
  cp "$WORK/desktop" ./desktop
  "$(go env GOPATH)/bin/fyne" package -os darwin --app-id "$TEST_ID" --executable desktop
  rm -f ./desktop
  mv BibleText.app "$APP"
)

note "embedding the container migration"
mkdir -p "$APP/Contents/Resources"
cp "$MIGRATION" "$APP/Contents/Resources/Container-Migration.plist"

note "signing sandboxed with the real entitlements"
# The sandbox and network entitlements are self-granted on macOS: a development
# certificate can carry them locally with no provisioning profile. Distribution
# is what needs the Store certificates.
codesign -f -s "$CERT" --entitlements "$ENTITLEMENTS" --generate-entitlement-der "$APP"

note "confirming the signature really is sandboxed"
SIGNED=$(codesign -d --entitlements - --xml "$APP" 2>/dev/null || true)
for key in com.apple.security.app-sandbox com.apple.security.network.client; do
  printf '%s' "$SIGNED" | grep -q "$key" || fail "signed bundle is missing $key"
done
echo "  both entitlements present"

note "launching"
open -n "$APP"
cat <<EOF

  Sandboxed test build running — bundle id $TEST_ID
  (the shipping id $SHIP_ID is untouched, and so is its container)

  WHAT TO CHECK

  Migration — the expensive one to get wrong:
    • Are your notes there? Open the notes browser.
    • Is your reading position, text size and translation as you left it?
    • If the app looks brand new, the migration did not run.

  Sandbox — everything below needs the network entitlement to work:
    • Does a translation download or is it stuck on the Gospels?
    • Does chapter audio play?
    • Do the links open: Settings → "Get a key", the privacy link, the
      site link, and Report on an AI answer? These now go through
      NSWorkspace rather than a subprocess precisely because the sandbox
      refuses the latter — a failure here is silent by design, so click them.

  Container:  $CONTAINER
  Sandbox denials, if any:
    log stream --predicate 'sender == "Sandbox"' --style compact
EOF
