#!/usr/bin/env bash
# Build BibleText for a PHYSICAL iPhone and install it, using FREE provisioning
# (a personal Apple ID, no paid Developer Program). Works on an Intel Mac.
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
#   • Create the cert + a provisioning profile for io.github.cubancorona.bibletext once, by
#     building any app with that bundle id + your Personal Team to the phone in
#     Xcode (a throwaway "Signer" project works). The cert + profile then persist
#     and this script reuses them.
#
# After that: just run this script. The free signature lapses after ~7 days —
# re-run to reinstall.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_DIR="${REPO_ROOT}/cmd/mobile"
APP_NAME="BibleText.app"
APP_ID="${BIBLETEXT_APP_ID:-io.github.cubancorona.bibletext}"
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

# ── 1. signing certificate ──────────────────────────────────────────────────
CERT_LINE="$(security find-identity -v -p codesigning 2>/dev/null | grep 'Apple Development' | head -1 || true)"
CERT_HASH="$(printf '%s' "$CERT_LINE" | awk '{print $2}')"
CERT_NAME="$(printf '%s' "$CERT_LINE" | sed -E 's/.*"(.*)"/\1/')"
[ -n "$CERT_HASH" ] || fail "No 'Apple Development' certificate. Sign into Xcode and run a throwaway app to the phone once (one-time setup above)."
note "signing identity: $CERT_NAME"

# ── 2. connected device ─────────────────────────────────────────────────────
DEVICE_ID="${BIBLETEXT_DEVICE_ID:-$(xcrun devicectl list devices 2>/dev/null | awk '/(iPhone|iPad)/ && /connected/ {print $(NF-1); exit}')}"
[ -n "$DEVICE_ID" ] || { xcrun devicectl list devices 2>&1 | sed 's/^/  /'; fail "No connected iPhone. Plug it in, unlock, Trust, enable Developer Mode."; }
note "target device: $DEVICE_ID"

# ── 3. provisioning profile for this app id (Xcode 16 uses the UserData dir) ──
PROFILE_FILE=""; PROFILE_NAME=""
for dir in "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles" "$HOME/Library/MobileDevice/Provisioning Profiles"; do
    [ -d "$dir" ] || continue
    while IFS= read -r -d '' p; do
        plist="$(security cms -D -i "$p" 2>/dev/null || true)"
        if printf '%s' "$plist" | grep -q "$APP_ID"; then
            PROFILE_FILE="$p"
            PROFILE_NAME="$(printf '%s' "$plist" | python3 -c 'import sys,plistlib;print(plistlib.loads(sys.stdin.buffer.read()).get("Name",""))')"
            break 2
        fi
    done < <(find "$dir" -name '*.mobileprovision' -print0 2>/dev/null)
done
[ -n "$PROFILE_FILE" ] || fail "No provisioning profile for $APP_ID. Do the one-time Xcode run with that exact bundle id."
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
  • The signature expires in ~7 days — re-run this script to reinstall.
EOF
