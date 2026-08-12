#!/usr/bin/env bash
# Build BibleText as an uploadable App Store .ipa, signed for DISTRIBUTION under the
# paid Apple Developer Program team (BIBLETEXT_TEAM_ID). Works on Intel and Apple
# Silicon Macs.
#
# Why this is more than `fyne release -os ios`:
#   • fyne/gomobile compile for the HOST arch; the App Store requires arm64 — so we
#     cross-compile the Go binary to ios/arm64 explicitly and swap it in, which is
#     correct on any host (same trick as run-ios-device.sh).
#   • Xcode 26 mints a CLOUD-MANAGED "Apple Distribution" cert whose private key never
#     lands in the local keychain, so plain `codesign` / `fyne release` cannot use it.
#     Instead we hand-assemble an .xcarchive around BibleText.app and let
#     `xcodebuild -exportArchive` (which CAN use the cloud cert) re-sign it for the
#     App Store and package the .ipa.
#
# One-time setup (already done): the app record in App Store Connect, the App Store
# provisioning profile "iOS Team Store Provisioning Profile: uk.co.bibletext", and the
# distribution cert (minted by archiving+exporting the throwaway Signer project).
#
# Output: build/BibleText.ipa  →  upload via Transporter.app or `xcrun altool`.
# OR: set BIBLETEXT_UPLOAD=1 to have xcodebuild UPLOAD straight to App Store
# Connect using the Xcode-authenticated account (no .ipa lands locally, no
# Transporter / API keys needed) — the build then appears under TestFlight
# after ~5-30 min of processing.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="${BIBLETEXT_APP_ID:-uk.co.bibletext}"
TEAM_ID="${BIBLETEXT_TEAM_ID:-R8PC7239T2}"
IOS_MIN="13.0"
CONFIG_VERSION="$(awk -F ' *= *' '/^Version *=/{gsub(/"/, "", $2); print $2; exit}' "$APP_DIR/FyneApp.toml")"
SHORT_VERSION="${BIBLETEXT_SHORT_VERSION:-$CONFIG_VERSION}"   # MUST match the App Store Connect version record
OUT_DIR="${REPO_ROOT}/build"
WORK="$(mktemp -d /tmp/bibletext-release.XXXXXX)"

export PATH="$(go env GOPATH)/bin:$PATH"
note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
fail() { printf '\n\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# go.mod ships STOCK; apply the iOS Fyne patches only for this build. Preserve
# the exact pre-build files (including any uncommitted user edits) because fyne
# increments FyneApp.toml's Build during packaging.
cp "$REPO_ROOT/go.mod" "$WORK/go.mod.original"
cp "$APP_DIR/FyneApp.toml" "$WORK/FyneApp.toml.original"
trap 'cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true; cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml" 2>/dev/null || true; rm -rf "$WORK"' EXIT
note "applying iOS Fyne drawloop patch (go.mod restored on exit)"
"${REPO_ROOT}/scripts/setup-fyne-patch.sh"

# The project's own API.Bible key can be compiled in so the NKJV works out of the
# box — but NOT by default for a STORE build, which is a public binary.
#
# Three reasons, all of them the project's own (README → "The NKJV key"):
#   1. LICENCE. "A distributed app needs commercial access ... the free Starter
#      plan's 3 licensed Bibles are non-commercial only." An App Store release is
#      distribution.
#   2. EXPOSURE. bundled_key_gen.go XOR-masks the key with the constant string
#      "bibletext-nkjv", which sits in the same binary — anyone with the .ipa can
#      recover the key in a minute. Verified: the masked value was present in a
#      store build made before this guard existed.
#   3. CEILING. 5,000 calls/month for the whole account, ~197 per NKJV download,
#      every device revalidating monthly — "roughly 25 active devices before a
#      month runs dry", after which readers on the bundled key get failures.
#
# OWNER'S DECISION (12 Aug 2026), made with the above in view: bundle it FOR NOW
# so the NKJV works the moment someone installs, and drop it in a later release —
# at which point new installs are bring-your-own-key while existing installs keep
# the key already in their store (README documents exactly that transition).
#
# So the default is ON, and turning it off for that future release is one flag:
#
#   BIBLETEXT_BUNDLE_KEY=0 ./scripts/release-ios.sh
#
# Whichever way it goes, the build says which it did — a store .ipa should never
# be ambiguous about whether it carries a credential.
source "${REPO_ROOT}/[redacted-retired-private-reference]"
if [ "${BIBLETEXT_BUNDLE_KEY:-1}" = "1" ]; then
  embed_bible_key
else
  note "NO bundled API.Bible key — new installs are bring-your-own-key"
fi
# Re-arm the FULL cleanup. Sourcing [redacted-retired-private-reference] above registered an EXIT
# trap of its own, and bash keeps only the LAST trap per signal — so it silently
# replaced the go.mod/FyneApp.toml restore armed earlier and left a temporary
# `replace` in go.mod after every device build. This trap does both jobs, and
# being last, it is the one that runs.
trap 'cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true; cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml" 2>/dev/null || true; rm -f "$REPO_ROOT/bundled_key_gen.go"; rm -rf "$WORK"' EXIT

( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

# ── 1. dev signing identity under TEAM_ID ────────────────────────────────────
# The archived app needs a valid signature for exportArchive to re-sign FROM; we use
# the same Apple Development cert (matched by subject OU == TEAM_ID) as device builds.
CERT_HASH=""; CERT_NAME=""
while IFS= read -r line; do
    h="$(printf '%s' "$line" | awk '{print $2}')"
    n="$(printf '%s' "$line" | sed -E 's/.*"(.*)"/\1/')"
    ou="$(security find-certificate -c "$n" -p 2>/dev/null | openssl x509 -noout -subject -nameopt sep_multiline 2>/dev/null | awk -F= '/OU/{print $2; exit}' | tr -d ' ')"
    if [ "$ou" = "$TEAM_ID" ]; then CERT_HASH="$h"; CERT_NAME="$n"; break; fi
done < <(security find-identity -v -p codesigning 2>/dev/null | grep 'Apple Development')
[ -n "$CERT_HASH" ] || fail "No 'Apple Development' cert for team $TEAM_ID. Set BIBLETEXT_TEAM_ID if that isn't your team; mint the cert by signing into Xcode."
note "dev signing identity: $CERT_NAME"

# ── 2. dev profile (satisfies the pre-export signature) ──────────────────────
# Prefer the explicit dev profile for this app id (what modern Xcode mints —
# "iOS Team Provisioning Profile: uk.co.bibletext"); accept a legacy team
# wildcard ("<TEAM>.*") as a fallback. Distribution profiles (no
# ProvisionedDevices) are skipped — exportArchive supplies the store profile.
DEV_FILE=""; WILD_FILE=""
for dir in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles" "$HOME/Library/MobileDevice/Provisioning Profiles"; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' p; do
        plist="$(security cms -D -i "$p" 2>/dev/null || true)"
        printf '%s' "$plist" | plutil -extract ProvisionedDevices raw -o - - >/dev/null 2>&1 || continue
        appid="$(printf '%s' "$plist" | plutil -extract Entitlements.application-identifier raw -o - - 2>/dev/null || true)"
        case "$appid" in
            "$TEAM_ID.$APP_ID") DEV_FILE="$p"; break 2 ;;
            "$TEAM_ID."\*)      WILD_FILE="$p" ;;
        esac
    done < <(find "$dir" -name '*.mobileprovision' -print0 2>/dev/null)
done
[ -n "$DEV_FILE" ] || DEV_FILE="$WILD_FILE"
[ -n "$DEV_FILE" ] || fail "No development profile for $TEAM_ID.$APP_ID (or wildcard $TEAM_ID.*) found. Run scripts/run-ios-device.sh once (or build any app for this id in Xcode) to mint one; set BIBLETEXT_TEAM_ID if $TEAM_ID isn't your team."

# ── 3. fyne assembles the bundle (unsigned; we set version + re-sign below) ───
note "fyne package -os ios (assembling bundle)"
( cd "$APP_DIR" && fyne package -os ios --app-id "$APP_ID" >/tmp/fyne_release_bundle.log 2>&1 ) || true
cp "$WORK/FyneApp.toml.original" "$APP_DIR/FyneApp.toml"
APP="$APP_DIR/$APP_NAME"
[ -f "$APP/Info.plist" ] || { tail -20 /tmp/fyne_release_bundle.log; fail "fyne did not leave an app bundle."; }

# ── 4. cross-compile arm64 + swap the binary in ──────────────────────────────
note "cross-compiling Go → ios/arm64"
SDK="$(xcrun --sdk iphoneos --show-sdk-path)"
CC="$(xcrun --sdk iphoneos -f clang)"
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC="$CC" \
    CGO_CFLAGS="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN" \
    CGO_LDFLAGS="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN" \
    go build -o "$WORK/bibletext-arm64" "$REPO_ROOT/cmd/mobile"
EXE="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Info.plist")"
cp "$WORK/bibletext-arm64" "$APP/$EXE"; chmod +x "$APP/$EXE"
note "binary arch: $(lipo -archs "$APP/$EXE")"

# ── 5. Info.plist tweaks for the App Store ───────────────────────────────────
PB() { /usr/libexec/PlistBuddy -c "$1" "$APP/Info.plist"; }
PB "Set :MinimumOSVersion $IOS_MIN" 2>/dev/null || PB "Add :MinimumOSVersion string $IOS_MIN"
PB "Set :CFBundleShortVersionString $SHORT_VERSION" 2>/dev/null || PB "Add :CFBundleShortVersionString string $SHORT_VERSION"
# Fyne 2.7.4's iOS template opts out of iPad multitasking even though BibleText's
# layout is width-adaptive. Remove the deprecated compatibility flag so Split
# View, Stage Manager, and iPadOS 26 window resizing deliver real size changes.
PB "Delete :UIRequiresFullScreen" 2>/dev/null || true
# Background audio + Now Playing / Control Center. fyne never emits UIBackgroundModes,
# so inject it here (before the step-6 codesign, or the signature breaks); a shipped
# build without it loses background playback + lock-screen controls. plutil -replace upserts.
plutil -replace UIBackgroundModes -json '["audio"]' "$APP/Info.plist"
# Add-only Photos access: without this key iOS hides "Save Image" in the share
# sheet for the verse-image cards. Write-only, used solely on the reader's tap.
plutil -replace NSPhotoLibraryAddUsageDescription -string "BibleText saves a shared verse image to your photo library only when you choose Save Image." "$APP/Info.plist"
# Declare no non-exempt encryption (HTTPS only) so the upload skips export-compliance.
PB "Set :ITSAppUsesNonExemptEncryption false" 2>/dev/null || PB "Add :ITSAppUsesNonExemptEncryption bool false"
# Fyne declares LaunchScreen in Info.plist but doesn't place a compiled storyboard
# in the app. Compile the tracked, adaptive launch screen for iOS 13+.
xcrun ibtool --compile "$APP/LaunchScreen.storyboardc" "$APP_DIR/LaunchScreen.storyboard" \
    --target-device iphone --target-device ipad --minimum-deployment-target "$IOS_MIN" \
    --module BibleText >/dev/null
# PrivacyInfo must be at the root of an iOS app bundle. It declares the optional
# third-party AI data flow plus the required-reason APIs present in the Go/Fyne
# executable (UserDefaults, app-container file metadata, and monotonic timers).
cp "$APP_DIR/PrivacyInfo.xcprivacy" "$APP/PrivacyInfo.xcprivacy"
plutil -lint "$APP/Info.plist" "$APP/PrivacyInfo.xcprivacy" >/dev/null
# Device family for the App Store listing. Since 1.1.0 the app ships UNIVERSAL
# (UIDeviceFamily=[1,2] — iPhone + iPad, the runtime width-adaptive layout in
# docs/IPAD.md), set explicitly rather than trusting fyne's default plist. App
# Review does NOT allow an update to remove a shipped device family, so
# iPhone-only (BIBLETEXT_IPAD=0) exists only for local experiments — an
# iPhone-only upload would be rejected now that 1.1.0 is out.
if [ "${BIBLETEXT_IPAD:-1}" != "0" ]; then
    plutil -replace UIDeviceFamily -json '[1, 2]' "$APP/Info.plist"
    # REBUILD THE ICON ASSET CATALOG. fyne's iOS packager emits only
    # 120/180/152/76 and no iPad Pro icon, so App Store Connect rejects the
    # upload with error 90023 ("no app icon for iPad of exactly '167x167'").
    # Adding a loose PNG + CFBundleIconFiles entry is NOT enough: the bundle
    # sets CFBundleIconName, which points the validator at Assets.car, so the
    # catalog itself must carry the full set. Regenerate it with actool from
    # the single 1024px source (cmd/mobile/Icon.png).
    note "rebuilding the icon asset catalog (adds the 167x167 iPad Pro icon)"
    ICONSET="$WORK/Assets.xcassets/AppIcon.appiconset"
    # actool needs BOTH the catalog's own root Contents.json and a pre-existing
    # output directory — without either it exits 0 having produced nothing.
    mkdir -p "$ICONSET" "$WORK/carout"
    printf '{"info":{"version":1,"author":"bibletext"}}' > "$WORK/Assets.xcassets/Contents.json"
    icon_json="" ; icon_sep=""
    # px, slot-name, idiom, size(pt), scale. 120px appears TWICE (iPhone 40pt@3x
    # and 60pt@2x), so each slot gets its own filename rather than being keyed
    # by pixel size.
    add_icon() {
        sips -s format png -z "$1" "$1" "$APP_DIR/Icon.png" --out "$ICONSET/$2.png" >/dev/null 2>&1 \
            || fail "could not scale the app icon to $1px"
        icon_json="$icon_json$icon_sep{\"filename\":\"$2.png\",\"idiom\":\"$3\",\"size\":\"$4\",\"scale\":\"$5\"}"
        icon_sep=","
    }
    add_icon 40   ip20x2   iphone 20x20     2x
    add_icon 60   ip20x3   iphone 20x20     3x
    add_icon 58   ip29x2   iphone 29x29     2x
    add_icon 87   ip29x3   iphone 29x29     3x
    add_icon 80   ip40x2   iphone 40x40     2x
    add_icon 120  ip40x3   iphone 40x40     3x
    add_icon 120  ip60x2   iphone 60x60     2x
    add_icon 180  ip60x3   iphone 60x60     3x
    add_icon 20   pad20x1  ipad   20x20     1x
    add_icon 40   pad20x2  ipad   20x20     2x
    add_icon 29   pad29x1  ipad   29x29     1x
    add_icon 58   pad29x2  ipad   29x29     2x
    add_icon 40   pad40x1  ipad   40x40     1x
    add_icon 80   pad40x2  ipad   40x40     2x
    add_icon 76   pad76x1  ipad   76x76     1x
    add_icon 152  pad76x2  ipad   76x76     2x
    add_icon 167  pad83x2  ipad   83.5x83.5 2x
    add_icon 1024 marketing ios-marketing 1024x1024 1x
    printf '{"images":[%s],"info":{"version":1,"author":"bibletext"}}' "$icon_json" > "$ICONSET/Contents.json"
    # NB: `plutil -lint` cannot parse JSON (it reports "Unexpected character {"),
    # so validate as actual JSON.
    python3 -c 'import json,sys;json.load(open(sys.argv[1]))' "$ICONSET/Contents.json" \
        || fail "generated Contents.json is invalid"
    xcrun actool "$WORK/Assets.xcassets" --compile "$WORK/carout" --platform iphoneos \
        --minimum-deployment-target "$IOS_MIN" --app-icon AppIcon \
        --target-device iphone --target-device ipad \
        --output-partial-info-plist "$WORK/icon-partial.plist" >/dev/null 2>&1 \
        || fail "actool could not compile the icon asset catalog"
    [ -f "$WORK/carout/Assets.car" ] || fail "actool produced no Assets.car"
    cp "$WORK/carout/Assets.car" "$APP/Assets.car"
    # Ship the 167 loose too (belt and braces for pre-catalog OS versions).
    cp "$ICONSET/pad83x2.png" "$APP/AppIcon83.5x83.5@2x~ipad.png"
    if ! PB 'Print :CFBundleIcons~ipad:CFBundlePrimaryIcon:CFBundleIconFiles' 2>/dev/null | grep -q 'AppIcon83.5x83.5'; then
        PB 'Add :CFBundleIcons~ipad:CFBundlePrimaryIcon:CFBundleIconFiles: string AppIcon83.5x83.5'
    fi
else
    PB "Delete :UIDeviceFamily" 2>/dev/null || true
    PB "Add :UIDeviceFamily array"
    PB "Add :UIDeviceFamily:0 integer 1"
fi
BUILD_NUM="$(PB 'Print :CFBundleVersion' 2>/dev/null || echo 1)"

# ── 6. dev re-sign so the archived bundle has a valid signature ──────────────
note "dev re-signing the bundle"
rm -rf "$APP/_CodeSignature"
cp "$DEV_FILE" "$APP/embedded.mobileprovision"
security cms -D -i "$DEV_FILE" > "$WORK/prof.plist"
plutil -extract Entitlements xml1 -o "$WORK/ent.plist" "$WORK/prof.plist"
# exportArchive later re-signs with the CONCRETE App Store profile
# ("<TEAM>.uk.co.bibletext") and aborts if the archived app's entitlement doesn't
# match. A legacy wildcard dev profile yields "<TEAM>.*", so pin the concrete id
# here; with an explicit dev profile this is an idempotent no-op.
/usr/libexec/PlistBuddy -c "Set :application-identifier $TEAM_ID.$APP_ID" "$WORK/ent.plist" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Add :application-identifier string $TEAM_ID.$APP_ID" "$WORK/ent.plist"
# Universal links. The profile carries the Associated Domains capability as a
# wildcard once it is enabled on the App ID, and a wildcard enumerates NO
# domains — the app would ship, install, and simply never open a shared link,
# with nothing to see in any log. So pin the concrete domain here, exactly as
# application-identifier is pinned above.
#
# The claim is scoped by the association file we publish at
# /.well-known/apple-app-site-association, which allow-lists ONLY the reader
# paths: privacy.html and support.html (the URLs App Store Connect points at)
# must keep opening in Safari. Never claim www. — it 301s to the apex, and
# Apple requires the association file be served without redirects.
/usr/libexec/PlistBuddy -c "Delete :com.apple.developer.associated-domains" "$WORK/ent.plist" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Add :com.apple.developer.associated-domains array" "$WORK/ent.plist"
/usr/libexec/PlistBuddy -c "Add :com.apple.developer.associated-domains:0 string applinks:bibletext.co.uk" "$WORK/ent.plist"
codesign -f -s "$CERT_HASH" --entitlements "$WORK/ent.plist" --generate-entitlement-der "$APP"

# ── 7. assemble an .xcarchive around BibleText.app ───────────────────────────
note "assembling .xcarchive"
ARCH_DIR="$WORK/BibleText.xcarchive"
mkdir -p "$ARCH_DIR/Products/Applications"
cp -R "$APP" "$ARCH_DIR/Products/Applications/"
cat > "$ARCH_DIR/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>ApplicationProperties</key><dict>
    <key>ApplicationPath</key><string>Applications/${APP_NAME}</string>
    <key>Architectures</key><array><string>arm64</string></array>
    <key>CFBundleIdentifier</key><string>${APP_ID}</string>
    <key>CFBundleShortVersionString</key><string>${SHORT_VERSION}</string>
    <key>CFBundleVersion</key><string>${BUILD_NUM}</string>
    <key>Team</key><string>${TEAM_ID}</string>
  </dict>
  <key>ArchiveVersion</key><integer>2</integer>
  <key>Name</key><string>BibleText</string>
  <key>SchemeName</key><string>BibleText</string>
</dict></plist>
PLIST

# ── 8. exportArchive → App Store .ipa or direct upload (Xcode re-signs with the
# distribution cert; BIBLETEXT_UPLOAD=1 sends it straight to App Store Connect
# over the Xcode-authenticated session instead of writing build/BibleText.ipa) ──
DESTINATION="export"
[ "${BIBLETEXT_UPLOAD:-0}" = "1" ] && DESTINATION="upload"
note "xcodebuild -exportArchive (App Store distribution, destination: $DESTINATION)"
cat > "$WORK/exportOptions.plist" <<EOPL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>method</key><string>app-store-connect</string>
  <key>teamID</key><string>${TEAM_ID}</string>
  <key>signingStyle</key><string>automatic</string>
  <key>destination</key><string>${DESTINATION}</string>
  <key>uploadSymbols</key><false/>
  <key>manageAppVersionAndBuildNumber</key><false/>
</dict></plist>
EOPL
rm -rf "$WORK/export"
xcodebuild -exportArchive -archivePath "$ARCH_DIR" \
    -exportOptionsPlist "$WORK/exportOptions.plist" \
    -exportPath "$WORK/export" -allowProvisioningUpdates 2>&1 | tail -25

if [ "$DESTINATION" = "upload" ]; then
    cat <<EOF

✓ Uploaded to App Store Connect (version ${SHORT_VERSION}, build ${BUILD_NUM}).
  Processing takes ~5-30 min; the build then appears under TestFlight /
  the version's Build picker in App Store Connect.
EOF
    exit 0
fi

IPA="$(ls "$WORK/export"/*.ipa 2>/dev/null | head -1)"
[ -n "$IPA" ] || fail "exportArchive did not produce an .ipa (see log above)."
mkdir -p "$OUT_DIR"; cp "$IPA" "$OUT_DIR/BibleText.ipa"

# ── 9. verify the .ipa ───────────────────────────────────────────────────────
note "verifying build/BibleText.ipa"
rm -rf "$WORK/verify"; unzip -q "$OUT_DIR/BibleText.ipa" -d "$WORK/verify"
VAPP="$(ls -d "$WORK/verify/Payload"/*.app)"
echo "  arch:      $(lipo -archs "$VAPP/$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$VAPP/Info.plist")")"
EXPORTED_VERSION="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$VAPP/Info.plist")"
EXPORTED_BUILD="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$VAPP/Info.plist")"
echo "  version:   $EXPORTED_VERSION ($EXPORTED_BUILD)"
[ "$EXPORTED_VERSION" = "$SHORT_VERSION" ] || fail "exported version $EXPORTED_VERSION does not match requested $SHORT_VERSION"
[ "$EXPORTED_BUILD" = "$BUILD_NUM" ] || fail "exported build $EXPORTED_BUILD does not match archived build $BUILD_NUM"
[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$VAPP/Info.plist")" = "$APP_ID" ] || fail "exported bundle identifier is wrong"
[ -f "$VAPP/PrivacyInfo.xcprivacy" ] || fail "exported app is missing PrivacyInfo.xcprivacy"
plutil -lint "$VAPP/PrivacyInfo.xcprivacy" >/dev/null || fail "exported privacy manifest is invalid"
[ -d "$VAPP/LaunchScreen.storyboardc" ] || fail "exported app is missing its compiled launch screen"
if /usr/libexec/PlistBuddy -c 'Print :UIRequiresFullScreen' "$VAPP/Info.plist" >/dev/null 2>&1; then
    fail "exported app still opts out of iPad multitasking"
fi
codesign -dvv "$VAPP" 2>&1 | grep -iE 'Authority=Apple|TeamIdentifier' | sed 's/^/  /'

cat <<EOF

✓ build/BibleText.ipa is ready (version ${SHORT_VERSION}, build ${BUILD_NUM}).
  Upload it with EITHER:
    • Transporter.app — drag the .ipa in, then Deliver; or
    • xcrun altool --upload-app -f build/BibleText.ipa -t ios \\
        --apiKey <KEY_ID> --apiIssuer <ISSUER_ID>
  The build then appears under TestFlight / the version's Build picker in
  App Store Connect after ~5–30 min of processing.
EOF
