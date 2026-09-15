#!/usr/bin/env bash
# Exercise the iOS half of the Fyne-tools patch without building or signing an
# app: the generated Xcode project must declare the deployment target the app
# ships for (config/product.json), never upstream's 9.0, which Xcode 27 refuses.
set -euo pipefail

cd "$(dirname "$0")/.."
scripts/setup-fyne-tools-patch.sh

IOS_MIN="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["iosMinimumOSVersion"],end="")' config/product.json)"
TEMPLATE="third_party/fyne-tools/cmd/fyne/internal/mobile/build_iosapp.go"
grep -q "IPHONEOS_DEPLOYMENT_TARGET = ${IOS_MIN};" "$TEMPLATE" || {
  echo "ERROR: the Xcode project template does not declare IPHONEOS_DEPLOYMENT_TARGET = ${IOS_MIN}" >&2
  exit 1
}
if grep -q 'IPHONEOS_DEPLOYMENT_TARGET = 9.0;' "$TEMPLATE"; then
  echo "ERROR: the Xcode project template still declares upstream's 9.0 deployment target" >&2
  exit 1
fi
grep -q "\"--minimum-deployment-target\", \"${IOS_MIN}\"" \
  third_party/fyne-tools/cmd/fyne/internal/commands/package-mobile.go || {
  echo "ERROR: the asset catalog compile does not name the ${IOS_MIN} deployment target" >&2
  exit 1
}

TEST_WORK="$(mktemp -d /tmp/bibletext-ios-target-test.XXXXXX)"
trap 'rm -rf -- "$TEST_WORK"' EXIT
mkdir -p "$TEST_WORK/go-cache" "$TEST_WORK/go-tmp"
(
  cd third_party/fyne-tools
  GOCACHE="$TEST_WORK/go-cache" GOTMPDIR="$TEST_WORK/go-tmp" \
    go test ./cmd/fyne/internal/mobile -run '^TestBibleTextIOS'
)
echo "iOS deployment-target regression tests passed."
