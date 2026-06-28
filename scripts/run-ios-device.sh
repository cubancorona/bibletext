#!/usr/bin/env bash
# Build BibleText for a PHYSICAL iPhone and install it, signed with your paid
# Apple Developer Program team (BIBLETEXT_TEAM_ID). Works on an Intel Mac.
#
# Why this is more than `fyne package -os ios`:
#   On an Intel host, fyne's (and gomobile's) iOS packaging compile the app for
#   the host arch (x86_64), which a real arm64 iPhone rejects ("IncorrectArchitecture").
#   They also use manual code-signing, which clashes with the Xcode-managed
#   profile a free account issues. So instead we:
#     1. let fyne assemble the .app bundle (Info.plist, icons, asset catalog),
#     2. cross-compile the Go app ourselves to ios/arm64 and swap that binary in,
#     3. re-sign with the free Apple Development cert + the managed profile,
#     4. install (and try to launch) via devicectl.
#
# ── One-time setup (only you can do this) ───────────────────────────────────
#   • iPhone: connect by cable, unlock, Trust This Computer, and enable
#     Settings → Privacy & Security → Developer Mode.
#   • Xcode → Settings → Accounts → sign in with your Apple ID (free).
#   • Create the cert + a provisioning profile for uk.co.bibletext once, by
#     building any app with that bundle id + your Personal Team to the phone in
#     Xcode (a throwaway "Signer" project works). The cert + profile then persist
#     and this script reuses them.
#
# After that: just run this script. A paid-team development profile is valid for
# ~1 year (vs a free team's 7 days) — re-run any time to reinstall.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="${BIBLETEXT_APP_ID:-uk.co.bibletext}"
TEAM_ID="${BIBLETEXT_TEAM_ID:-R8PC7239T2}"   # paid Apple Developer Program team
IOS_MIN="13.0"

export PATH="$(go env GOPATH)/bin:$PATH"
note() { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
fail() { printf '\n\033[31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ── 0. apply the iOS-only Fyne scroll-lag patch for this build ───────────────
# go.mod ships STOCK (so `go build` / `go run ./cmd/desktop` stay one-line); the
# fix is a one-line change to Fyne's iOS drawloop (see patches/README.md). We
# regenerate a patched Fyne and inject a temporary `replace` just for this build,
# restoring stock go.mod on exit (success, failure, or Ctrl-C).
trap 'git -C "$REPO_ROOT" checkout -- go.mod 2>/dev/null || true' EXIT
note "applying iOS Fyne drawloop patch (go.mod restored on exit)"
"${REPO_ROOT}/scripts/setup-fyne-patch.sh"
( cd "$REPO_ROOT" && go mod edit -replace fyne.io/fyne/v2=./third_party/fyne )

# ── 1. signing certificate (the Apple Development cert under TEAM_ID) ─────────
# There may be several "Apple Development" certs (e.g. an old free team's). A
# cert's team is the OU of its subject — match THAT to TEAM_ID, not the per-cert
# id Xcode shows in parentheses after the name (that is NOT the team).
CERT_HASH=""; CERT_NAME=""
while IFS= read -r line; do
    h="$(printf '%s' "$line" | awk '{print $2}')"
    n="$(printf '%s' "$line" | sed -E 's/.*"(.*)"/\1/')"
    ou="$(security find-certificate -c "$n" -p 2>/dev/null | openssl x509 -noout -subject -nameopt sep_multiline 2>/dev/null | awk -F= '/OU/{print $2; exit}' | tr -d ' ')"
    if [ "$ou" = "$TEAM_ID" ]; then CERT_HASH="$h"; CERT_NAME="$n"; break; fi
done < <(security find-identity -v -p codesigning 2>/dev/null | grep 'Apple Development')
[ -n "$CERT_HASH" ] || fail "No 'Apple Development' cert for team $TEAM_ID. Sign into Xcode with that account and mint one (header)."
note "signing identity: $CERT_NAME  (team $TEAM_ID)"

# ── 2. reachable device ─────────────────────────────────────────────────────
# devicectl reports a usable phone as "connected" (USB) OR "available (paired)"
# (CoreDevice network tunnel) — install works in either state. Match any reachable
# iPhone/iPad and pull the UDID by its UUID shape, not column position (the State
# column is one or two words, which shifts the positional fields).
DEVICE_ID="${BIBLETEXT_DEVICE_ID:-$(xcrun devicectl list devices 2>/dev/null | awk '/(iPhone|iPad)/ && (/connected/ || /available/ || /paired/) { for (i=1; i<=NF; i++) if ($i ~ /^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$/) { print $i; exit } }')}"
[ -n "$DEVICE_ID" ] || { xcrun devicectl list devices 2>&1 | sed 's/^/  /'; fail "No connected iPhone. Plug it in, unlock, Trust, enable Developer Mode."; }
note "target device: $DEVICE_ID"

# ── 3. provisioning profile under TEAM_ID covering this app id ───────────────
# Match by the profile's application-identifier ("<TEAM>.<bundle>", or the team
# wildcard "<TEAM>.*"), so we pick the RIGHT team's profile and accept the wildcard
# "iOS Team Provisioning Profile: *". An explicit bundle-id profile wins if present.
# Only DEVELOPMENT profiles are eligible: once an App Store *distribution* profile
# for this exact bundle id exists (e.g. "iOS Team Store Provisioning Profile:
# uk.co.bibletext", minted for release), it would win the explicit match but cannot
# install directly on a device — it fails with MIInstallerErrorDomain 13 "Attempted
# to install a Beta profile without the proper entitlement". Distribution profiles
# have no ProvisionedDevices key, so we skip any profile lacking one.
PROFILE_FILE=""; PROFILE_NAME=""; WILD_FILE=""; WILD_NAME=""
for dir in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles" "$HOME/Library/MobileDevice/Provisioning Profiles"; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' p; do
        plist="$(security cms -D -i "$p" 2>/dev/null || true)"
        # skip distribution/App Store profiles (no ProvisionedDevices) — dev installs only
        printf '%s' "$plist" | plutil -extract ProvisionedDevices raw -o - - >/dev/null 2>&1 || continue
        appid="$(printf '%s' "$plist" | plutil -extract Entitlements.application-identifier raw -o - - 2>/dev/null || true)"
        name="$(printf '%s' "$plist" | plutil -extract Name raw -o - - 2>/dev/null || true)"
        case "$appid" in
            "$TEAM_ID.$APP_ID") PROFILE_FILE="$p"; PROFILE_NAME="$name"; break 2 ;;
            "$TEAM_ID."\*)      WILD_FILE="$p"; WILD_NAME="$name" ;;
        esac
    done < <(find "$dir" -name '*.mobileprovision' -print0 2>/dev/null)
done
[ -n "$PROFILE_FILE" ] || { PROFILE_FILE="$WILD_FILE"; PROFILE_NAME="$WILD_NAME"; }
[ -n "$PROFILE_FILE" ] || fail "No provisioning profile for $APP_ID under team $TEAM_ID. Mint one (header)."
note "provisioning profile: $PROFILE_NAME"

# ── 4. let fyne assemble the .app bundle (Info.plist + icons + assets) ───────
# We re-sign manually in step 6, so do NOT pass --certificate/--profile here.
# Passing them makes fyne configure *manual* signing in its generated xcodeproj;
# if the named provisioning profile is Xcode-*managed* (Xcode may flip it to managed
# at any time), xcodebuild then aborts the ENTIRE build before assembling a bundle
# ("… is Xcode managed, but signing settings require a manually managed profile"),
# leaving nothing to reuse. Assembling unsigned keeps this step independent of the
# provisioning state — fyne exits 0 and leaves the bundle, and step 6 signs it.
note "fyne package -os ios (assembling the app bundle, unsigned; we re-sign in step 6)"
( cd "$APP_DIR" && fyne package -os ios --app-id "$APP_ID" >/tmp/fyne_bundle.log 2>&1 ) || true
git -C "$REPO_ROOT" checkout -- cmd/mobile/FyneApp.toml 2>/dev/null || true
APP="$APP_DIR/$APP_NAME"
[ -f "$APP/Info.plist" ] || { tail -20 /tmp/fyne_bundle.log; fail "fyne did not leave an app bundle to reuse."; }

# ── 5. cross-compile the Go app to ios/arm64 and swap the binary in ─────────
note "cross-compiling Go → ios/arm64"
SDK="$(xcrun --sdk iphoneos --show-sdk-path)"
CC="$(xcrun --sdk iphoneos -f clang)"
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC="$CC" \
    CGO_CFLAGS="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN" \
    CGO_LDFLAGS="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN" \
    go build -o /tmp/bibletext-ios-arm64 "$REPO_ROOT/cmd/mobile"
EXE="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Info.plist")"
cp /tmp/bibletext-ios-arm64 "$APP/$EXE"; chmod +x "$APP/$EXE"
note "binary arch: $(lipo -archs "$APP/$EXE")"
/usr/libexec/PlistBuddy -c "Set :MinimumOSVersion $IOS_MIN" "$APP/Info.plist" 2>/dev/null \
  || /usr/libexec/PlistBuddy -c "Add :MinimumOSVersion string $IOS_MIN" "$APP/Info.plist"

# ── 5b. enable background audio + Now Playing / Control Center ───────────────
# Fyne's iOS packager hardcodes Info.plist and never adds UIBackgroundModes, so
# inject it here — BEFORE the step-6 codesign, or the mutation invalidates the
# signature. fyne regenerates Info.plist on every `fyne package` run, so this must
# run each build (it does — it's inline). plutil -replace upserts (no entitlement
# is needed for background audio; the plist key is the only requirement).
note "adding UIBackgroundModes=[audio] (background playback + Now Playing)"
plutil -replace UIBackgroundModes -json '["audio"]' "$APP/Info.plist"

# ── 6. re-sign with the dev cert + managed profile + its entitlements ───────
note "re-signing"
rm -rf "$APP/_CodeSignature"
cp "$PROFILE_FILE" "$APP/embedded.mobileprovision"
security cms -D -i "$PROFILE_FILE" > /tmp/bt_prof.plist
plutil -extract Entitlements xml1 -o /tmp/bt_ent.plist /tmp/bt_prof.plist
codesign -f -s "$CERT_HASH" --entitlements /tmp/bt_ent.plist --generate-entitlement-der "$APP"

# ── 7. install + launch ─────────────────────────────────────────────────────
note "installing on device"
xcrun devicectl device install app --device "$DEVICE_ID" "$APP"
note "launching (unlock the phone if it refuses)"
xcrun devicectl device process launch --device "$DEVICE_ID" "$APP_ID" 2>&1 | grep -iE 'launched|error|Locked' || true

cat <<EOF

✓ Done. BibleText is on the iPhone.
  • If launch said "Locked", just unlock the phone and tap the BibleText icon.
  • First ever install: Settings → General → VPN & Device Management → (your
    Apple ID) → Trust.
  • The development profile is valid ~1 year (paid team) — re-run any time.
EOF
