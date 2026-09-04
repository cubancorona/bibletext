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
#        go install fyne.io/tools/cmd/fyne@v1.7.2
#
# This script is idempotent: re-run it to push a new build to the simulator.
set -euo pipefail
umask 077

# Simulator builds carry the project fallback but never inherit unrelated
# provider credentials. Load and isolate the raw key before any subprocess.
unset ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
unset BIBLE_KEY_LDFLAGS BIBLETEXT_RELEASE_LDFLAGS BIBLETEXT_REAL_GO
export GOFLAGS="" GODEBUG=""
case "${BASH_SOURCE[0]}" in
  */*) script_dir_part="${BASH_SOURCE[0]%/*}" ;;
  *) script_dir_part="." ;;
esac
case "$script_dir_part" in
  /*|./*|../*) ;;
  *) script_dir_part="./$script_dir_part" ;;
esac
original_dir="$PWD"
builtin cd -P -- "$script_dir_part"
SCRIPT_DIR="$PWD"
builtin cd -- "$original_dir"
unset original_dir script_dir_part
source "${SCRIPT_DIR}/release-bible-key.sh"
load_release_bible_key

REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="uk.co.bibletext"
WORK="$(mktemp -d /tmp/bibletext-simulator.XXXXXX)"
cp "$REPO_ROOT/go.mod" "$WORK/go.mod.original"
cp "$APP_DIR/FyneApp.toml" "$WORK/FyneApp.toml.original"
PREEXISTING_APP="$WORK/preexisting-$APP_NAME"
if [[ -e "$APP_DIR/$APP_NAME" ]]; then
    mv "$APP_DIR/$APP_NAME" "$PREEXISTING_APP"
fi
cleanup() {
    clear_release_bible_key
    cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true
    cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml" 2>/dev/null || true
    rm -rf "$APP_DIR/$APP_NAME"
    if [[ -e "$PREEXISTING_APP" ]]; then
        mv "$PREEXISTING_APP" "$APP_DIR/$APP_NAME" 2>/dev/null || true
    fi
    rm -rf "$WORK"
}
trap cleanup EXIT

# --dev adds the bibletextdev build tag, which compiles in the Links tab: a page
# of shared-link scenarios that call the real HandleShareLink. It exists because
# a universal link cannot be triggered in the simulator and needs a tap from
# another app on a device, so this is the only way to exercise that path
# directly. Release builds never pass the tag, so the page cannot ship —
# dev_links_off.go is what a shipping build compiles instead.
DEV_TAG=""
for arg in "$@"; do
    case "$arg" in
        --dev) DEV_TAG=",bibletextdev" ;;
    esac
done

DEVICE_NAME="${BIBLETEXT_SIM_DEVICE:-iPhone 15}"

export PATH="$(go env GOPATH)/bin:$PATH"

# Apply the iOS-only Fyne scroll-lag patch for this build. The exact original
# go.mod and mobile package metadata are restored by the exit trap.
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
APP="$WORK/$APP_NAME"
mv "$APP_DIR/$APP_NAME" "$APP"
# Match the device build: add UIBackgroundModes=[audio] so the audio path is exercised.
# (No codesign on the sim path, so post-package is fine; true background behavior is
# only reliably testable on a device via run-ios-device.sh.)
plutil -replace UIBackgroundModes -json '["audio"]' "$APP/Info.plist"
# Match the device build: declare add-only Photos access so the share sheet's
# "Save Image" action appears in the simulator too.
plutil -replace NSPhotoLibraryAddUsageDescription -string "BibleText saves a shared verse image to your photo library only when you choose Save Image." "$APP/Info.plist"

# App Store parity: the release build deletes UIRequiresFullScreen
# (iPad multitasking) and compiles the launch storyboard — the smoke builds must
# match, or Split View/Stage Manager resizing ships untested.
PBX() { /usr/libexec/PlistBuddy -c "$1" "$APPPLIST" 2>/dev/null || true; }
APPPLIST="$APP/Info.plist"
PBX "Delete :UIRequiresFullScreen"
xcrun ibtool --compile "$APP/LaunchScreen.storyboardc" "$APP_DIR/LaunchScreen.storyboard" >/dev/null

# Ship the sim build as universal (iPhone + iPad) so it runs NATIVELY on an iPad
# simulator — otherwise the iPad runs it in iPhone compatibility mode and the
# interface idiom reports iPhone. Native iPad identity enables the shared
# layout's landscape navigation rail, portrait bottom bar, and reporter-width
# reading measure. The App Store device family is controlled separately by
# release-ios.sh (BIBLETEXT_IPAD), so this only affects local simulator runs.
# Test with e.g. BIBLETEXT_SIM_DEVICE="iPad Pro 11-inch (M5)".
plutil -replace UIDeviceFamily -json '[1, 2]' "$APP/Info.plist"

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
# key migration and the "saved securely" status untestable on the simulator
# even though the device path works.
#
# fyne package offers no linker hook (it sets CGO_LDFLAGS and -ldflags itself),
# so relink just the executable with fyne's own iOS-simulator env plus the
# section, and drop it into the packaged bundle before vtool + codesign.
#
# WARNING: this string IS the keychain partition. Changing it orphans every key
# previously saved in a simulator — pick once, never churn.
SIM_ENT="$WORK/simulator-entitlements.plist"
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
# Same floor as the device and Store builds (config/product.json).
IOS_MIN="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["iosMinimumOSVersion"],end="")' "$REPO_ROOT/config/product.json")"
[ -n "$IOS_MIN" ] || { echo "could not read iosMinimumOSVersion from config/product.json" >&2; exit 1; }
SIM_CF="-isysroot $SIM_SDK -mios-simulator-version-min=$IOS_MIN -arch $(uname -m)"
echo "==> relinking with the simulator entitlements section (Keychain)"
# Build to a temp path: `go build -o` refuses to overwrite an existing file it
# does not recognise as its own output ("already exists and is not an object
# file"), and fyne has just written its own binary there.
SIM_MAIN="$WORK/bibletext-simulator-main"
if ( cd "$REPO_ROOT" && CGO_ENABLED=1 GOOS=ios GOARCH="$(go env GOARCH)" \
        CC="$SIM_CLANG" CXX="${SIM_CLANG}++" \
        CGO_CFLAGS="$SIM_CF" CGO_CXXFLAGS="$SIM_CF" \
        CGO_LDFLAGS="$SIM_CF -Wl,-sectcreate,__TEXT,__entitlements,$SIM_ENT" \
        go build -trimpath -tags "ios$DEV_TAG" \
          -ldflags "$BIBLE_KEY_LDFLAGS -w" -o "$SIM_MAIN" ./cmd/mobile ); then
    BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
      python3 "$REPO_ROOT/scripts/verify-release-key.py" "$SIM_MAIN"
    mv -f "$SIM_MAIN" "$APP/main"
else
    echo "ERROR: keyed simulator relink failed." >&2
    exit 1
fi

# Fyne builds the simulator binary for min iOS 7.0, which modern Simulator
# runtimes reject ("This app needs to be updated by the developer"). Rewrite the
# Mach-O build-version to a current minimum and re-sign (ad-hoc) so it installs.
xcrun vtool -arch "$(uname -m)" -set-build-version 7 15.0 18.0 -replace \
    -output "$APP/main" "$APP/main" 2>/dev/null \
    || echo "==> vtool min-version bump skipped (older Simulator? continuing)" >&2
codesign --force --sign - "$APP" >/dev/null 2>&1 || true
BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
  python3 "$REPO_ROOT/scripts/verify-release-key.py" "$APP/main"
clear_release_bible_key

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
xcrun simctl install "$BOOTED" "$APP"

echo "==> launching $APP_ID"
SIMCTL_CHILD_BIBLETEXT_DEV_NOTES="${BIBLETEXT_DEV_NOTES:-}" \
SIMCTL_CHILD_BIBLETEXT_DEV_PHONE_LANDSCAPE="${BIBLETEXT_DEV_PHONE_LANDSCAPE:-}" \
SIMCTL_CHILD_BT_SCROLL_DEBUG="${BT_SCROLL_DEBUG:-}" \
  xcrun simctl launch "$BOOTED" "$APP_ID"

echo
echo "Done. To inspect logs:"
echo "  xcrun simctl spawn $BOOTED log stream --predicate 'process == \"main\"'"
