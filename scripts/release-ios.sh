#!/usr/bin/env bash
# Build BibleText as an uploadable App Store .ipa, signed for DISTRIBUTION under the
# paid Apple Developer Program team (R8PC7239T2). Works on an Intel Mac.
#
# Why this is more than `fyne release -os ios`:
#   • On Intel, fyne/gomobile compile for the host arch (x86_64); the App Store
#     requires arm64 — so we cross-compile the Go binary to ios/arm64 and swap it in
#     (same trick as run-ios-device.sh).
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
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="${BIBLETEXT_APP_ID:-uk.co.bibletext}"
TEAM_ID="${BIBLETEXT_TEAM_ID:-R8PC7239T2}"
IOS_MIN="13.0"
SHORT_VERSION="${BIBLETEXT_SHORT_VERSION:-1.0}"   # MUST match the App Store Connect version record
OUT_DIR="${REPO_ROOT}/build"
WORK="$(mktemp -d /tmp/bibletext-release.XXXXXX)"

export PATH="$(go env GOPATH)/bin:$PATH"
note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
fail() { printf '\n\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# go.mod ships STOCK; apply the iOS Fyne drawloop patch only for this build and
# restore go.mod + FyneApp.toml (fyne bumps its Build) + scratch dir on exit.
trap 'git -C "$REPO_ROOT" checkout -- go.mod cmd/mobile/FyneApp.toml 2>/dev/null || true; rm -rf "$WORK"' EXIT
note "applying iOS Fyne drawloop patch (go.mod restored on exit)"
"${REPO_ROOT}/scripts/setup-fyne-patch.sh"
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
[ -n "$CERT_HASH" ] || fail "No 'Apple Development' cert for team $TEAM_ID."
note "dev signing identity: $CERT_NAME"

# ── 2. wildcard dev profile (satisfies the pre-export signature) ─────────────
WILD_FILE=""
for dir in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles" "$HOME/Library/MobileDevice/Provisioning Profiles"; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' p; do
        appid="$(security cms -D -i "$p" 2>/dev/null | plutil -extract Entitlements.application-identifier raw -o - - 2>/dev/null || true)"
        case "$appid" in "$TEAM_ID."\*) WILD_FILE="$p"; break 2;; esac
    done < <(find "$dir" -name '*.mobileprovision' -print0 2>/dev/null)
done
[ -n "$WILD_FILE" ] || fail "No wildcard dev profile $TEAM_ID.* found."

# ── 3. fyne assembles the bundle (unsigned; we set version + re-sign below) ───
note "fyne package -os ios (assembling bundle)"
( cd "$APP_DIR" && fyne package -os ios --app-id "$APP_ID" >/tmp/fyne_release_bundle.log 2>&1 ) || true
git -C "$REPO_ROOT" checkout -- cmd/mobile/FyneApp.toml 2>/dev/null || true
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
# Declare no non-exempt encryption (HTTPS only) so the upload skips export-compliance.
PB "Set :ITSAppUsesNonExemptEncryption false" 2>/dev/null || PB "Add :ITSAppUsesNonExemptEncryption bool false"
# Device family for the App Store listing. Default = iPhone-only (UIDeviceFamily=[1]) so the
# v1.0 submission needs no iPad screenshots and Apple won't review an iPad layout (the app
# still runs on iPad in compatibility mode). Set BIBLETEXT_IPAD=1 to ship universal later.
if [ "${BIBLETEXT_IPAD:-0}" != "1" ]; then
    PB "Delete :UIDeviceFamily" 2>/dev/null || true
    PB "Add :UIDeviceFamily array"
    PB "Add :UIDeviceFamily:0 integer 1"
fi
BUILD_NUM="$(PB 'Print :CFBundleVersion' 2>/dev/null || echo 1)"

# ── 6. dev re-sign so the archived bundle has a valid signature ──────────────
note "dev re-signing the bundle"
rm -rf "$APP/_CodeSignature"
cp "$WILD_FILE" "$APP/embedded.mobileprovision"
security cms -D -i "$WILD_FILE" > "$WORK/prof.plist"
plutil -extract Entitlements xml1 -o "$WORK/ent.plist" "$WORK/prof.plist"
# The wildcard dev profile yields application-identifier "<TEAM>.*"; exportArchive later
# re-signs with the CONCRETE App Store profile ("<TEAM>.uk.co.bibletext") and aborts if
# the archived app's entitlement is the wildcard. Pin it to the concrete id now so they match.
/usr/libexec/PlistBuddy -c "Set :application-identifier $TEAM_ID.$APP_ID" "$WORK/ent.plist" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Add :application-identifier string $TEAM_ID.$APP_ID" "$WORK/ent.plist"
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

# ── 8. exportArchive → App Store .ipa (Xcode re-signs with the distribution cert) ──
note "xcodebuild -exportArchive (App Store distribution)"
cat > "$WORK/exportOptions.plist" <<EOPL
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>method</key><string>app-store-connect</string>
  <key>teamID</key><string>${TEAM_ID}</string>
  <key>signingStyle</key><string>automatic</string>
  <key>destination</key><string>export</string>
  <key>uploadSymbols</key><false/>
  <key>manageAppVersionAndBuildNumber</key><false/>
</dict></plist>
EOPL
rm -rf "$WORK/export"
xcodebuild -exportArchive -archivePath "$ARCH_DIR" \
    -exportOptionsPlist "$WORK/exportOptions.plist" \
    -exportPath "$WORK/export" -allowProvisioningUpdates 2>&1 | tail -25

IPA="$(ls "$WORK/export"/*.ipa 2>/dev/null | head -1)"
[ -n "$IPA" ] || fail "exportArchive did not produce an .ipa (see log above)."
mkdir -p "$OUT_DIR"; cp "$IPA" "$OUT_DIR/BibleText.ipa"

# ── 9. verify the .ipa ───────────────────────────────────────────────────────
note "verifying build/BibleText.ipa"
rm -rf "$WORK/verify"; unzip -q "$OUT_DIR/BibleText.ipa" -d "$WORK/verify"
VAPP="$(ls -d "$WORK/verify/Payload"/*.app)"
echo "  arch:      $(lipo -archs "$VAPP/$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$VAPP/Info.plist")")"
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
