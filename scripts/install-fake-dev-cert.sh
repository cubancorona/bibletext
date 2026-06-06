#!/usr/bin/env bash
# Install a self-signed "Apple Development" certificate so that Fyne's iOS
# packager will agree to build for the simulator without a real Apple Developer
# account.
#
# WHY: fyne package -os ios|iossimulator calls
#     security find-certificate -c "Apple Development" -p
# to extract a Team ID from the cert's OU field. Without ANY such cert in the
# keychain it errors out with "failed to look up certificate : exit status 44".
# The resulting .app gets ad-hoc re-signed at the end of the build, so this
# self-signed cert is only used to keep Fyne and xcodebuild happy during the
# build pipeline — the simulator does not validate it.
#
# The RIGHT way to get this cert is Xcode → Settings → Accounts → +
# (sign in with any Apple ID, free). Use this script only if that's not an
# option (e.g. headless box).
set -euo pipefail

TEAMID="${1:-BIBLETEXT1}"          # 10-char fake team identifier
PW="$(openssl rand -hex 8)"        # one-shot password, not stored anywhere
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

cd "$WORKDIR"

echo "==> generating self-signed cert with OU=$TEAMID"
openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -days 3650 -nodes \
    -subj "/CN=Apple Development: bibletext local ($TEAMID)/OU=$TEAMID/O=BibleText Local/C=US" \
    -addext "extendedKeyUsage=codeSigning" \
    -addext "keyUsage=digitalSignature" >/dev/null 2>&1

echo "==> wrapping in PKCS12 with legacy PBE (macOS keychain requires it)"
openssl pkcs12 -export -inkey key.pem -in cert.pem -out cert.p12 \
    -keypbe PBE-SHA1-3DES -certpbe PBE-SHA1-3DES -macalg SHA1 \
    -password "pass:$PW" -name "Apple Development: bibletext local" >/dev/null

echo "==> importing into login.keychain-db"
security import cert.p12 -k "$HOME/Library/Keychains/login.keychain-db" -P "$PW" \
    -T /usr/bin/codesign -T /usr/bin/security -T /usr/bin/xcodebuild -A

echo
echo "Done. Confirm with:"
echo "  security find-certificate -c 'Apple Development' -p"
