#!/usr/bin/env bash
# Assert that the API key really lands in the SIMULATOR's Keychain, with the
# right protection class — the two properties that were silently wrong before
# and cannot be checked by `go test` (the Keychain path is //go:build ios cgo).
#
#   ./scripts/run-ios-sim.sh          # build + install + launch first
#   ./scripts/verify-sim-keychain.sh  # then assert
#
# WHY THIS EXISTS
#   1. A bare ad-hoc signature carries no entitlements, so SecItemAdd used to
#      fail with -34018 and the store silently fell back to Preferences — the
#      whole Keychain path was untestable off-device. run-ios-sim.sh now links
#      the entitlements into a Mach-O __TEXT,__entitlements section (the same
#      mechanism Xcode uses for simulator builds; a code-signature entitlement
#      would make AMFI refuse to exec the binary).
#   2. Keys were briefly stored ...WhenUnlockedThisDeviceOnly, which Apple
#      excludes from backups and device migration: upgrading, then moving to a
#      new iPhone, would have silently lost the key after the plaintext copy
#      was erased. They are AfterFirstUnlock now, and this asserts it.
set -euo pipefail

APP_ID="uk.co.bibletext"
ACCESS_GROUP="${BIBLETEXT_SIM_KEYCHAIN_GROUP:-$APP_ID}"

udid_re='[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}'
UDID=$(xcrun simctl list devices booted | grep -oE "$udid_re" | head -1 || true)
if [ -z "${UDID:-}" ]; then
    echo "No booted simulator. Run ./scripts/run-ios-sim.sh first." >&2
    exit 1
fi

CONT=$(xcrun simctl get_app_container "$UDID" "$APP_ID" data 2>/dev/null || true)
if [ -z "${CONT:-}" ]; then
    echo "BibleText is not installed on the booted simulator." >&2
    exit 1
fi
PREFS="$CONT/Documents/fyne/preferences.json"
KC="$HOME/Library/Developer/CoreSimulator/Devices/$UDID/data/Library/Keychains/keychain-2-debug.db"

# 0. Does a Keychain item already exist? That decides which half of the
#    contract this run proves — apiKey returns EARLY on a Keychain hit, so a
#    staged plaintext value is only consumed when the Keychain is empty.
had_item=$(sqlite3 "$KC" "select count(*) from genp where agrp='$ACCESS_GROUP';" 2>/dev/null || echo 0)

# 1. Stage a pre-1.1.6 plaintext key exactly as an upgrading reader would have
#    it, then relaunch so newKeyStore's migrateAllKeys runs.
PROBE="sim-keychain-probe-$$"
python3 - "$PREFS" "$PROBE" <<'PY'
import json, sys
path, probe = sys.argv[1], sys.argv[2]
p = json.load(open(path))
p["ai.key.gemini"] = probe
json.dump(p, open(path, "w"))
PY
xcrun simctl terminate "$UDID" "$APP_ID" >/dev/null 2>&1 || true
xcrun simctl launch "$UDID" "$APP_ID" >/dev/null
sleep 6

fail=0

# 2. WRITE path (empty Keychain): the plaintext copy must be GONE — it is
#    erased only after a confirmed Keychain write.
#    READ path (item already present): the plaintext copy must be UNTOUCHED —
#    apiKey found the key in the Keychain and returned before the migration.
left=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1])).get('ai.key.gemini',''))" "$PREFS")
if [ "$had_item" -eq 0 ]; then
    if [ -n "$left" ]; then
        echo "FAIL: the legacy plaintext key survived migration — the Keychain WRITE failed." >&2
        echo "      (Expected on a build without the __entitlements section; check run-ios-sim.sh.)" >&2
        fail=1
    else
        echo "ok: WRITE path — plaintext copy erased after a confirmed Keychain write"
    fi
elif [ "$left" != "$PROBE" ]; then
    echo "FAIL: a stored Keychain key was not read back — apiKey fell through to" >&2
    echo "      the legacy path and re-migrated (Keychain READ is failing)." >&2
    fail=1
else
    echo "ok: READ path — stored Keychain key read back; legacy value left untouched"
fi

# 3. The item must exist under our access group AND be AfterFirstUnlock ('ck'),
#    never a ThisDeviceOnly class ('cku' / 'dku') that backups exclude.
pdmn=$(sqlite3 "$KC" "select pdmn from genp where agrp='$ACCESS_GROUP' limit 1;" 2>/dev/null || true)
case "$pdmn" in
    ck)  echo "ok: Keychain item is AfterFirstUnlock (backup- and migration-restorable)" ;;
    "")  echo "FAIL: no Keychain item for access group '$ACCESS_GROUP'." >&2; fail=1 ;;
    cku|dku)
         echo "FAIL: Keychain item is ThisDeviceOnly ('$pdmn') — Apple excludes it from" >&2
         echo "      backups and device migration, so the key vanishes on a new iPhone." >&2
         fail=1 ;;
    *)   echo "FAIL: unexpected protection class '$pdmn' (want 'ck')." >&2; fail=1 ;;
esac

# 4. Leave no probe value behind.
python3 - "$PREFS" <<'PY'
import json, sys
path = sys.argv[1]
p = json.load(open(path))
p["ai.key.gemini"] = ""
json.dump(p, open(path, "w"))
PY

exit "$fail"
