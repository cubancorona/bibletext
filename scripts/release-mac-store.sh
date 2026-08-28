#!/usr/bin/env bash
# Build a Mac App Store submission package: a universal, sandboxed, signed
# BibleText.app wrapped in a signed .pkg ready for Transporter or altool.
#
# This is the Store twin of the plain desktop release in release.yml. The
# direct-download build stays exactly as it is — unsigned, unsandboxed, two
# architecture-specific zips — because that channel has no reason to change.
#
# WHY A SCRIPT AND NOT `fyne release -os darwin`
#
# `fyne release` is nominally the Store path, but two of its behaviours make it
# unusable on its own here:
#
#   1. It writes its own entitlements file from a built-in template holding a
#      single key (the sandbox) and OVERWRITES anything left in the build
#      directory. An app signed with that template launches sandboxed with
#      networking denied: no translation downloads, no cross references, no
#      NKJV, no audio, no AI. It does not crash — the embedded Gospels seed
#      keeps it open on Matthew to John forever — which is worse, because it
#      looks like a broken app rather than a permissions problem.
#   2. It requires --profile but never writes an embedded.provisionprofile into
#      the bundle, so its output is not submittable.
#
# So this script packages with the ordinary `fyne package`, then re-signs with
# the real entitlements and embeds the profile itself. scripts/release-ios.sh
# already does the same thing for the same reason.
#
#   BIBLETEXT_TEAM_ID              your Apple Developer team id
#   BIBLETEXT_MAC_PROFILE          path to the Mac App Store provisioning profile
#   BIBLE_API_KEY (or the Keychain item release-bible-key.sh reads)
#
# Output: build/mac-store/BibleText.pkg
set -euo pipefail

cd "$(dirname "$0")/.."
REPO_ROOT="$PWD"
WORK="$REPO_ROOT/build/mac-store"
APP="$WORK/BibleText.app"
ENTITLEMENTS="$REPO_ROOT/appstore/mac/BibleText.entitlements"
MIGRATION="$REPO_ROOT/appstore/mac/Container-Migration.plist"

note() { printf '\n\033[1m==> %s\033[0m\n' "$1"; }
fail() { printf '\033[31mERROR: %s\033[0m\n' "$1" >&2; exit 1; }

APP_ID=$(python3 -c 'import json;print(json.load(open("config/product.json"))["desktopAppID"],end="")')
[ -n "$APP_ID" ] || fail "could not read desktopAppID from config/product.json"
# The declared macOS floor (docs/MAC_APP_STORE.md records the choice). It must
# reach BOTH the compile and the plist: without -mmacosx-version-min the
# external linker stamps the build machine's SDK version as the binary's minos
# — measured as 26.0 on an Xcode 26 Mac — producing an app that launches
# nowhere older than the machine that built it, whatever the plist claims.
MAC_MIN=$(python3 -c 'import json;print(json.load(open("config/product.json"))["macMinimumOSVersion"],end="")')
[ -n "$MAC_MIN" ] || fail "could not read macMinimumOSVersion from config/product.json"

TEAM_ID="${BIBLETEXT_TEAM_ID:-}"
[ -n "$TEAM_ID" ] || fail "set BIBLETEXT_TEAM_ID to your Apple Developer team id"
PROFILE="${BIBLETEXT_MAC_PROFILE:-}"
[ -n "$PROFILE" ] && [ -f "$PROFILE" ] ||
  fail "set BIBLETEXT_MAC_PROFILE to a Mac App Store provisioning profile for $APP_ID"

note "checking the store configuration before building anything"
python3 scripts/check-mac-store-config.py || fail "store configuration is not shippable"
python3 scripts/check-product-identity.py || fail "product identity is inconsistent"
python3 scripts/check-min-os-versions.py || fail "declared OS floors are not shippable"
# A spent version (docs/VERSIONING.md) must never be rebuilt with new code.
DESKTOP_VERSION=$(sed -n 's/^Version = "\(.*\)"/\1/p' cmd/desktop/FyneApp.toml)
[ -n "$DESKTOP_VERSION" ] || fail "could not read Version from cmd/desktop/FyneApp.toml"
./scripts/check-version-not-spent.sh "$DESKTOP_VERSION" || fail "version $DESKTOP_VERSION is spent"

# Distribution certificates, not the Development ones the device scripts use.
APP_CERT=$(security find-identity -v -p codesigning 2>/dev/null |
  grep "3rd Party Mac Developer Application" | grep "$TEAM_ID" | head -1 |
  awk '{print $2}') || true
[ -n "${APP_CERT:-}" ] ||
  fail "no '3rd Party Mac Developer Application' certificate for team $TEAM_ID"
INSTALLER_CERT=$(security find-identity -v 2>/dev/null |
  grep "3rd Party Mac Developer Installer" | grep "$TEAM_ID" | head -1 |
  awk '{print $2}') || true
[ -n "${INSTALLER_CERT:-}" ] ||
  fail "no '3rd Party Mac Developer Installer' certificate for team $TEAM_ID"

rm -rf "$WORK"; mkdir -p "$WORK"

# go.mod ships STOCK; the local Fyne patches are applied for this build only
# and go.mod is restored on exit, exactly as release-ios.sh and
# build-android.sh do it. This step was MISSING here, so Mac App Store builds
# linked stock Fyne — whose preferences writer truncates preferences.json in
# place before rewriting it. A death inside that window (a crash, a force
# quit, the kill a Store update performs) leaves the file empty, and an empty
# store reads as a brand-new reader: every note gone, including notes other
# people shared, which exist nowhere else. The patched writer publishes by
# rename instead. The same step also brings the emoji and caret-blink fixes
# the other platforms already ship.
cp "$REPO_ROOT/go.mod" "$WORK/go.mod.original"
trap 'cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true' EXIT
note "applying the Fyne patches (go.mod restored on exit)"
"$REPO_ROOT/scripts/setup-fyne-patch.sh"
( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

note "building both architectures"
# shellcheck source=/dev/null
source scripts/release-bible-key.sh
# load_release_bible_key, not the encoded variant: this script runs on a
# maintainer's Mac and reads the dedicated login-Keychain item, the way
# release-ios.sh does. The encoded variant exists for GitHub's runners, which
# cannot reach a Keychain and receive a pre-encoded value through a secret.
load_release_bible_key
# Replaces the go.mod-only trap above: both cleanups, or the key would outlive
# the build when this trap overwrote the first one.
trap 'clear_release_bible_key; cp "$WORK/go.mod.original" "$REPO_ROOT/go.mod" 2>/dev/null || true' EXIT
(
  cd cmd/desktop
  export CGO_CFLAGS="-mmacosx-version-min=$MAC_MIN" CGO_LDFLAGS="-mmacosx-version-min=$MAC_MIN"
  CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o "$WORK/desktop-arm64" .
  CGO_ENABLED=1 GOARCH=amd64 go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o "$WORK/desktop-amd64" .
)

note "confirming the atomic preferences writer is in both slices"
# The patch is the difference between a torn write losing a reader's notes and
# not. Assert it in the BINARIES rather than trusting that the patch step ran:
# each slice must carry the patched writer's marker. The control proves the
# probe can find anything at all, so a zero means absence rather than a broken
# search.
for slice in "$WORK/desktop-arm64" "$WORK/desktop-amd64"; do
  strings -a "$slice" | grep -qF "Preferences save not published" ||
    fail "$(basename "$slice") lacks the atomic preferences writer — the Fyne patch did not reach this build"
  strings -a "$slice" | grep -qF "World English Bible" ||
    fail "$(basename "$slice") is missing a string every build has; the marker probe cannot be trusted"
done
echo "  both slices carry the patched writer"

note "joining them into one universal binary"
# The Store expects a single app, not one upload per architecture.
lipo -create -output "$WORK/desktop" "$WORK/desktop-arm64" "$WORK/desktop-amd64"
lipo -info "$WORK/desktop"

note "packaging the .app"
(
  cd cmd/desktop
  cp "$WORK/desktop" ./desktop
  "$(go env GOPATH)/bin/fyne" package -os darwin --app-id "$APP_ID" --executable desktop
  rm -f ./desktop
  mv BibleText.app "$APP"
)

note "verifying the packaged binary before it is signed"
# The verifier derives the build machine's root by stripping two components
# from the workspace, then fails if that root appears anywhere in the binary.
# On a runner the workspace is GITHUB_WORKSPACE; here it is the checkout, which
# makes the derived root the home directory — exactly the path a local build
# would leak, and the reason -trimpath is not taken on trust.
GITHUB_WORKSPACE="$REPO_ROOT" BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" \
  ./scripts/verify-release-package.sh "$APP/Contents/MacOS/desktop" "$APP"

note "setting the store category"
# The packager's Info.plist template writes public.app-category.<category>
# from a flag only `fyne release` accepts, so `fyne package` leaves a bare
# "public.app-category." which Apple rejects as not a valid UTI. Written here,
# before signing, because editing Info.plist afterwards breaks the signature.
# "reference" is the primary category the existing listing already uses.
/usr/libexec/PlistBuddy -c "Set :LSApplicationCategoryType public.app-category.reference" \
  "$APP/Contents/Info.plist"

note "declaring the minimum macOS version"
# The packager's template hardcodes LSMinimumSystemVersion 10.11, a value
# nobody here chose and no build since Go 1.24 could honour. Replace it with
# the declared floor — before signing, like the category above.
/usr/libexec/PlistBuddy -c "Set :LSMinimumSystemVersion $MAC_MIN" "$APP/Contents/Info.plist" ||
  /usr/libexec/PlistBuddy -c "Add :LSMinimumSystemVersion string $MAC_MIN" "$APP/Contents/Info.plist"

note "declaring export compliance"
# Without this key the uploaded build sits in App Store Connect waiting for an
# export-compliance answer instead of becoming selectable for a version. The
# app speaks ordinary HTTPS and nothing else, so the answer is false — the same
# declaration release-ios.sh makes, and for the same reason. Written before
# signing, like the category above.
/usr/libexec/PlistBuddy -c "Set :ITSAppUsesNonExemptEncryption false" "$APP/Contents/Info.plist" \
  2>/dev/null ||
  /usr/libexec/PlistBuddy -c "Add :ITSAppUsesNonExemptEncryption bool false" "$APP/Contents/Info.plist"

note "embedding the provisioning profile and the container migration"
cp "$PROFILE" "$APP/Contents/embedded.provisionprofile"
mkdir -p "$APP/Contents/Resources"
cp "$MIGRATION" "$APP/Contents/Resources/Container-Migration.plist"

note "signing with the real entitlements"
# A store build must carry the application and team identifiers in its
# signature, matching the ones inside the provisioning profile; without them
# the upload is accepted but flagged, and the bundle is not TestFlight
# eligible. They are per-publisher, so they are added here from the team and
# app id rather than written into the tracked entitlements file, which stays
# generic enough for a fork to use unchanged.
SIGN_ENTS="$WORK/entitlements-signing.plist"
cp "$ENTITLEMENTS" "$SIGN_ENTS"
/usr/libexec/PlistBuddy -c "Add :com.apple.application-identifier string $TEAM_ID.$APP_ID" "$SIGN_ENTS"
/usr/libexec/PlistBuddy -c "Add :com.apple.developer.team-identifier string $TEAM_ID" "$SIGN_ENTS"
# Universal Links: the domain claim that lets a clicked bibletext.co.uk share
# link open the app instead of the browser (share_link_macos.go receives it).
# This is a RESTRICTED entitlement — only a build carrying a provisioning
# profile that authorises it may launch — which is exactly why it is injected
# here and not written into the tracked entitlements file: the dev-signed
# sandbox rehearsal embeds no profile, and adding the claim there would make
# macOS refuse to launch it. The Mac App Store profile carries the
# authorisation (Associated Domains is enabled on the App ID).
SITE_HOST=$(python3 -c 'import json;print(json.load(open("config/product.json"))["siteBase"].removeprefix("https://"),end="")')
[ -n "$SITE_HOST" ] || fail "could not derive the site host from config/product.json"
# The profile must AUTHORISE the claim, not merely exist: a Mac App Store
# profile generated before Associated Domains was enabled on the App ID signs
# cleanly and passes every signature check, and the failure then surfaces only
# at upload or as a launch kill of the store build. Checked here, before the
# slow build, where the fix (regenerate the profile) is still cheap.
security cms -D -i "$PROFILE" 2>/dev/null |
  plutil -extract Entitlements xml1 -o - - 2>/dev/null |
  grep -qF "com.apple.developer.associated-domains" ||
  fail "the provisioning profile does not authorise associated-domains — regenerate it with Associated Domains enabled on the App ID"
/usr/libexec/PlistBuddy -c "Add :com.apple.developer.associated-domains array" "$SIGN_ENTS"
/usr/libexec/PlistBuddy -c "Add :com.apple.developer.associated-domains:0 string applinks:$SITE_HOST" "$SIGN_ENTS"

# The packager already wrote its own single-key entitlements; this replaces
# them. --generate-entitlement-der matches what release-ios.sh does and what
# current macOS expects.
codesign -f -s "$APP_CERT" --timestamp --options runtime \
  --entitlements "$SIGN_ENTS" --generate-entitlement-der "$APP"

note "confirming the signature carries the entitlements we asked for"
SIGNED=$(codesign -d --entitlements - --xml "$APP" 2>/dev/null || true)
for key in com.apple.security.app-sandbox com.apple.security.network.client \
           com.apple.developer.associated-domains "applinks:$SITE_HOST"; do
  printf '%s' "$SIGNED" | grep -qF "$key" ||
    fail "the signed bundle is missing $key — the packager's template won"
done
codesign --verify --deep --strict --verbose=2 "$APP"

note "confirming the app honours the declared macOS floor"
# Two claims must agree with config/product.json: the plist value the Store
# displays, and each slice's linked minos, which is what the loader enforces.
# A toolchain that one day refuses to target $MAC_MIN and stamps higher fails
# here, which is the prompt to raise the declared floor deliberately.
PLIST_MIN=$(/usr/libexec/PlistBuddy -c 'Print :LSMinimumSystemVersion' "$APP/Contents/Info.plist")
[ "$PLIST_MIN" = "$MAC_MIN" ] || fail "Info.plist declares macOS $PLIST_MIN, not $MAC_MIN"
MINOS_SEEN=$(otool -l "$APP/Contents/MacOS/desktop" | awk '/LC_BUILD_VERSION/{v=1} v && $1=="minos"{print $2; v=0}')
[ -n "$MINOS_SEEN" ] || fail "could not read LC_BUILD_VERSION minos from the universal binary"
SLICES=$(printf '%s\n' "$MINOS_SEEN" | wc -l | tr -d ' ')
[ "$SLICES" = "2" ] || fail "expected 2 binary slices with a minos, found $SLICES"
for m in $MINOS_SEEN; do
  [ "$m" = "$MAC_MIN" ] ||
    fail "a binary slice is linked for macOS $m, not $MAC_MIN — the version-min flags did not reach the compile"
done
echo "  floor: macOS $MAC_MIN (plist and both slices agree)"

note "building the installer package"
productbuild --component "$APP" /Applications \
  --sign "$INSTALLER_CERT" "$WORK/BibleText.pkg"

note "done"
echo "  $WORK/BibleText.pkg"
echo
echo "  Upload with Transporter.app, or:"
echo "    xcrun altool --upload-app -t macos -f $WORK/BibleText.pkg \\"
echo "      --apiKey \"\$ASC_KEY_ID\" --apiIssuer \"\$ASC_ISSUER_ID\""
echo
echo "  First submission: read docs/MAC_APP_STORE.md — the container migration"
echo "  is consumed on the launch that creates the container, so a reader who"
echo "  opens an un-migrated build once cannot be migrated afterwards."
