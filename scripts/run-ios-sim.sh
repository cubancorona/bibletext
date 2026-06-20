#!/usr/bin/env bash
# Build the iOS-simulator .app, install it on a booted simulator, and launch it.
#
# One-time prerequisites:
#   1. Xcode (full app from the App Store, not just CLT) installed.
#   2. An iOS simulator runtime downloaded:
#        xcodebuild -downloadPlatform iOS
#      (Tip: prefix with `script -q /tmp/xcb.log` so the progress percentage is
#       actually visible — xcodebuild swallows it under a plain pipe.)
#   3. A code-signing certificate named "Apple Development" in your keychain.
#      The easiest source is Xcode → Settings → Accounts → sign in with an
#      Apple ID (free). Alternatively, see scripts/install-fake-dev-cert.sh.
#   4. The new Fyne CLI installed (the old fyne.io/fyne/v2/cmd/fyne refuses to
#      build simulator targets):
#        go install fyne.io/tools/cmd/fyne@latest
#
# This script is idempotent: re-run it to push a new build to the simulator.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="io.github.cubancorona.bibletext"
DEVICE_NAME="${BIBLETEXT_SIM_DEVICE:-iPhone 15}"

export PATH="$(go env GOPATH)/bin:$PATH"

# Apply the iOS-only Fyne scroll-lag patch for this build, then restore stock
# go.mod on exit so `go build` / desktop stay one-line. See patches/README.md.
trap 'git -C "$REPO_ROOT" checkout -- go.mod 2>/dev/null || true' EXIT
"${REPO_ROOT}/scripts/setup-fyne-patch.sh"
( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

echo "==> fyne package -os iossimulator"
(cd "$APP_DIR" && fyne package -os iossimulator --app-id "$APP_ID")

# Boot the requested simulator if not already booted.
BOOTED=$(xcrun simctl list devices booted | awk -F'[()]' '/\(Booted\)/ {print $2; exit}')
if [ -z "${BOOTED:-}" ]; then
    DEVICE_UDID=$(xcrun simctl list devices available | awk -F'[()]' -v name="$DEVICE_NAME" '
        $0 ~ name "[[:space:]]*\\(" { print $2; exit }')
    if [ -z "${DEVICE_UDID:-}" ]; then
        # Requested device isn't available (e.g. a newer Xcode ships newer
        # models and dropped "$DEVICE_NAME"). Fall back to the first available
        # iPhone simulator so the script keeps working across Xcode versions.
        DEVICE_UDID=$(xcrun simctl list devices available | awk -F'[()]' '/iPhone.*\(/ {print $2; exit}')
        [ -n "${DEVICE_UDID:-}" ] && echo "==> '$DEVICE_NAME' unavailable; using first available iPhone" >&2
    fi
    if [ -z "${DEVICE_UDID:-}" ]; then
        echo "No available iPhone simulator found." >&2
        echo "Available devices:" >&2
        xcrun simctl list devices available | sed 's/^/  /' >&2
        echo "Download a runtime with: xcodebuild -downloadPlatform iOS" >&2
        exit 1
    fi
    echo "==> booting $DEVICE_UDID"
    xcrun simctl boot "$DEVICE_UDID"
    BOOTED="$DEVICE_UDID"
fi

echo "==> opening Simulator.app"
open -a Simulator

echo "==> installing $APP_NAME on simulator $BOOTED"
xcrun simctl install "$BOOTED" "$APP_DIR/$APP_NAME"

echo "==> launching $APP_ID"
xcrun simctl launch "$BOOTED" "$APP_ID"

echo
echo "Done. To inspect logs:"
echo "  xcrun simctl spawn $BOOTED log stream --predicate 'process == \"main\"'"
