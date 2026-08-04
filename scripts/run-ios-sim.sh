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
APP_ID="uk.co.bibletext"
DEVICE_NAME="${BIBLETEXT_SIM_DEVICE:-iPhone 15}"

export PATH="$(go env GOPATH)/bin:$PATH"

# Apply the iOS-only Fyne scroll-lag patch for this build, then restore stock
# go.mod on exit so `go build` / desktop stay one-line. See patches/README.md.
trap 'git -C "$REPO_ROOT" checkout -- go.mod 2>/dev/null || true' EXIT
"${REPO_ROOT}/scripts/setup-fyne-patch.sh"
( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

# fyne's iOS packager requires an "Apple Development" certificate even for the
# simulator (it reads the team id from the cert). Fail early with the remedy
# instead of dying inside fyne with a cryptic "exit status 44".
if ! security find-certificate -c "Apple Development" >/dev/null 2>&1; then
    echo "No 'Apple Development' certificate in your keychain (fyne needs one even for the simulator)." >&2
    echo "Either: Xcode → Settings → Accounts → sign in with any free Apple ID and mint one," >&2
    echo "or (no Apple account needed): ./scripts/install-fake-dev-cert.sh" >&2
    exit 1
fi

echo "==> fyne package -os iossimulator"
(cd "$APP_DIR" && fyne package -os iossimulator --app-id "$APP_ID")
# Match the device build: add UIBackgroundModes=[audio] so the audio path is exercised.
# (No codesign on the sim path, so post-package is fine; true background behavior is
# only reliably testable on a device via run-ios-device.sh.)
plutil -replace UIBackgroundModes -json '["audio"]' "$APP_DIR/$APP_NAME/Info.plist"
# Match the device build: declare add-only Photos access so the share sheet's
# "Save Image" action appears in the simulator too.
plutil -replace NSPhotoLibraryAddUsageDescription -string "BibleText saves a shared verse image to your photo library only when you choose Save Image." "$APP_DIR/$APP_NAME/Info.plist"

# App Store parity (audit finding): the release build deletes UIRequiresFullScreen
# (iPad multitasking) and compiles the launch storyboard — the smoke builds must
# match, or Split View/Stage Manager resizing ships untested.
PBX() { /usr/libexec/PlistBuddy -c "$1" "$APPPLIST" 2>/dev/null || true; }
APPPLIST="$APP_DIR/$APP_NAME/Info.plist"
PBX "Delete :UIRequiresFullScreen"
xcrun ibtool --compile "$APP_DIR/$APP_NAME/LaunchScreen.storyboardc" "$APP_DIR/LaunchScreen.storyboard" >/dev/null

# Ship the sim build as universal (iPhone + iPad) so it runs NATIVELY on an iPad
# simulator — otherwise the iPad runs it in iPhone compatibility mode, where the
# interface idiom reports iPhone and the regular-width (tablet) layout never
# appears. The App Store device family is controlled separately by release-ios.sh
# (BIBLETEXT_IPAD), so this only affects local simulator runs. Test the iPad
# layout with e.g. BIBLETEXT_SIM_DEVICE="iPad Pro 11-inch (M5)".
plutil -replace UIDeviceFamily -json '[1, 2]' "$APP_DIR/$APP_NAME/Info.plist"

# KEYCHAIN PARITY. A simulator app gets its entitlements from a Mach-O
# __TEXT,__entitlements SECTION — never from the code signature. Signing a
# simulator binary with `codesign --entitlements` looks like it works (codesign
# exits 0) but AMFI then refuses to exec it: "adhoc signed app with restricted
# entitlements" → launch fails with POSIX 163. This is Apple's own split:
# Xcode's Embedded-Simulator.xcspec drops CODE_SIGN_ENTITLEMENTS for the
# simulator and maps LD_ENTITLEMENTS_SECTION to exactly the -sectcreate flag
# used below (the runtime's own MobileSafari carries the same section).
#
# Without it, SecItemAdd/SecItemUpdate return -34018 errSecMissingEntitlement
# ("neither application-identifier nor keychain-access-groups"), so the app
# silently falls back to Preferences — leaving the Keychain path, the pre-1.1.6
# key migration and the "saved securely" status UNTESTABLE on the simulator
# while working on device (audit finding).
#
# fyne package offers no linker hook (it sets CGO_LDFLAGS and -ldflags itself),
# so relink just the executable with fyne's own iOS-simulator env plus the
# section, and drop it into the packaged bundle before vtool + codesign.
#
# WARNING: this string IS the keychain partition. Changing it orphans every key
# previously saved in a simulator — pick once, never churn.
SIM_ENT="$(mktemp -t bt_sim_ent).plist"
cat >"$SIM_ENT" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>application-identifier</key>
    <string>${APP_ID}</string>
    <key>keychain-access-groups</key>
    <array>
        <string>${APP_ID}</string>
    </array>
</dict>
</plist>
PLIST
SIM_SDK="$(xcrun --sdk iphonesimulator --show-sdk-path)"
SIM_CLANG="$(xcrun --sdk iphonesimulator --find clang)"
SIM_CF="-isysroot $SIM_SDK -mios-simulator-version-min=15.0 -arch $(uname -m)"
echo "==> relinking with the simulator entitlements section (Keychain)"
# Build to a temp path: `go build -o` refuses to overwrite an existing file it
# does not recognise as its own output ("already exists and is not an object
# file"), and fyne has just written its own binary there.
SIM_MAIN="$(mktemp -t bt_sim_main)"
if ( cd "$REPO_ROOT" && CGO_ENABLED=1 GOOS=ios GOARCH="$(go env GOARCH)" \
        CC="$SIM_CLANG" CXX="${SIM_CLANG}++" \
        CGO_CFLAGS="$SIM_CF" CGO_CXXFLAGS="$SIM_CF" \
        CGO_LDFLAGS="$SIM_CF -Wl,-sectcreate,__TEXT,__entitlements,$SIM_ENT" \
        go build -tags ios -ldflags=-w -o "$SIM_MAIN" ./cmd/mobile ); then
    mv -f "$SIM_MAIN" "$APP_DIR/$APP_NAME/main"
else
    echo "==> entitlements relink failed; continuing with fyne's binary (Keychain falls back to Preferences)" >&2
    rm -f "$SIM_MAIN"
fi
rm -f "$SIM_ENT"

# Fyne builds the simulator binary for min iOS 7.0, which modern Simulator
# runtimes reject ("This app needs to be updated by the developer"). Rewrite the
# Mach-O build-version to a current minimum and re-sign (ad-hoc) so it installs.
xcrun vtool -arch "$(uname -m)" -set-build-version 7 15.0 18.0 -replace \
    -output "$APP_DIR/$APP_NAME/main" "$APP_DIR/$APP_NAME/main" 2>/dev/null \
    || echo "==> vtool min-version bump skipped (older Simulator? continuing)" >&2
codesign --force --sign - "$APP_DIR/$APP_NAME" >/dev/null 2>&1 || true

# A simulator UDID is a 36-char dashed hex string. We extract it by that pattern
# rather than by field position: device names can contain parentheses (e.g.
# "iPad Pro 11-inch (M5)"), which broke a naive `-F'[()]'` split — it grabbed the
# "M5" chip token instead of the UDID.
udid_re='[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}'

# Boot the requested simulator if not already booted. Match only devices in the
# iOS runtime sections — a booted watchOS/tvOS simulator must not hijack the install.
# NOTE the `|| true` on every detection pipeline: under `set -euo pipefail` an
# empty grep match (no booted device / device name missing) exits 1 and would
# kill the script BEFORE its own fallback logic — the failure mode is a silent
# stop right after the "==> fyne package" line.
BOOTED=$(xcrun simctl list devices booted | awk '/^-- iOS/{ios=1; next} /^--/{ios=0} ios && /\(Booted\)/ {print; exit}' | grep -oE "$udid_re" | head -1 || true)
if [ -z "${BOOTED:-}" ]; then
    DEVICE_UDID=$(xcrun simctl list devices available | grep -F "$DEVICE_NAME (" | grep -oE "$udid_re" | head -1 || true)
    if [ -z "${DEVICE_UDID:-}" ]; then
        # Requested device isn't available (e.g. a newer Xcode ships newer
        # models and dropped "$DEVICE_NAME"). Fall back to the first available
        # iPhone simulator so the script keeps working across Xcode versions.
        DEVICE_UDID=$(xcrun simctl list devices available | grep -E 'iPhone.*\(' | grep -oE "$udid_re" | head -1 || true)
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
