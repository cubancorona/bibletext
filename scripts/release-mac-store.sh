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

TEAM_ID="${BIBLETEXT_TEAM_ID:-}"
[ -n "$TEAM_ID" ] || fail "set BIBLETEXT_TEAM_ID to your Apple Developer team id"
PROFILE="${BIBLETEXT_MAC_PROFILE:-}"
[ -n "$PROFILE" ] && [ -f "$PROFILE" ] ||
  fail "set BIBLETEXT_MAC_PROFILE to a Mac App Store provisioning profile for $APP_ID"

note "checking the store configuration before building anything"
python3 scripts/check-mac-store-config.py || fail "store configuration is not shippable"
python3 scripts/check-product-identity.py || fail "product identity is inconsistent"

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

note "building both architectures"
# shellcheck source=/dev/null
source scripts/release-bible-key.sh
# load_release_bible_key, not the encoded variant: this script runs on a
# maintainer's Mac and reads the dedicated login-Keychain item, the way
# release-ios.sh does. The encoded variant exists for GitHub's runners, which
# cannot reach a Keychain and receive a pre-encoded value through a secret.
load_release_bible_key
trap clear_release_bible_key EXIT
(
  cd cmd/desktop
  CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o "$WORK/desktop-arm64" .
  CGO_ENABLED=1 GOARCH=amd64 go build -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o "$WORK/desktop-amd64" .
)

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

note "embedding the provisioning profile and the container migration"
cp "$PROFILE" "$APP/Contents/embedded.provisionprofile"
mkdir -p "$APP/Contents/Resources"
cp "$MIGRATION" "$APP/Contents/Resources/Container-Migration.plist"

note "signing with the real entitlements"
# The packager already wrote its own single-key entitlements; this replaces
# them. --generate-entitlement-der matches what release-ios.sh does and what
# current macOS expects.
codesign -f -s "$APP_CERT" --timestamp --options runtime \
  --entitlements "$ENTITLEMENTS" --generate-entitlement-der "$APP"

note "confirming the signature carries the entitlements we asked for"
SIGNED=$(codesign -d --entitlements - --xml "$APP" 2>/dev/null || true)
for key in com.apple.security.app-sandbox com.apple.security.network.client; do
  printf '%s' "$SIGNED" | grep -q "$key" ||
    fail "the signed bundle is missing $key — the packager's template won"
done
codesign --verify --deep --strict --verbose=2 "$APP"

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
