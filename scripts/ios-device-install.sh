# Installing a signed development build on a physical iPhone, for
# scripts/run-ios-device.sh, which sources this file.
#
# A locked phone refuses the install (CoreDevice error 12040 or 10003, or
# "Failed to acquire assertion"), and the build before it takes minutes: the
# Fyne patch, the fyne CLI, the package, the cross-compile and the signature.
# So the install is tried again a few times while the reader unlocks the
# phone, and a bundle that still did not go on is kept in a private folder to
# be installed later with `run-ios-device.sh --install-kept`, instead of being
# deleted with the build's work directory. The executable inside is linked
# with the project's bundled key, which is why the folder is private and the
# bundle is removed as soon as an install succeeds.
#
# Each step is a function, so ios_device_install_test.go can drive them with a
# stand-in for xcrun.

# What devicectl prints when the phone is locked or asleep.
IOS_INSTALL_LOCKED='12040|10003|[Ff]ailed to acquire assertion|[Ll]ocked'

# ios_find_device: the identifier of a reachable iPhone or iPad, or nothing.
# devicectl reports a usable phone as "connected" (USB) OR "available
# (paired)" (CoreDevice network tunnel); install works in either state. The
# identifier is matched by shape, not column position, since the State column
# is one or two words. It is either a CoreDevice UUID (8-4-4-4-12 hex, what
# Xcode 26 printed) or the hardware UDID (8-16 hex, what Xcode 27 prints for a
# phone); devicectl accepts both. BIBLETEXT_DEVICE_ID names one outright.
ios_find_device() {
    if [ -n "${BIBLETEXT_DEVICE_ID:-}" ]; then
        printf '%s\n' "$BIBLETEXT_DEVICE_ID"
        return 0
    fi
    xcrun devicectl list devices 2>/dev/null | awk '/(iPhone|iPad)/ && !/unavailable/ && !/simulated/ && (/connected/ || /available/ || /paired/) { for (i=1; i<=NF; i++) if ($i ~ /^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$/ || $i ~ /^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{16}$/) { print $i; exit } }'
}

# ios_install_with_retries <device> <app> [tries] [wait-seconds]
#
# Installs the bundle, trying again while devicectl says the phone is locked,
# up to tries attempts (BIBLETEXT_INSTALL_TRIES, 5) with wait seconds between
# (BIBLETEXT_INSTALL_WAIT, 10). Any other failure is not tried again.
# Succeeds only when an install succeeded.
ios_install_with_retries() {
    local device="$1" app="$2"
    local tries="${3:-${BIBLETEXT_INSTALL_TRIES:-5}}" wait="${4:-${BIBLETEXT_INSTALL_WAIT:-10}}"
    local attempt=1 log
    log="$(mktemp "${TMPDIR:-/tmp}/bibletext-install.XXXXXX")"
    while :; do
        if xcrun devicectl device install app --device "$device" "$app" >"$log" 2>&1; then
            cat "$log"
            rm -f "$log"
            return 0
        fi
        cat "$log" >&2
        if ! grep -Eq "$IOS_INSTALL_LOCKED" "$log"; then
            rm -f "$log"
            return 1
        fi
        if [ "$attempt" -ge "$tries" ]; then
            rm -f "$log"
            return 1
        fi
        printf 'The phone looks locked: unlock it. Trying again in %ss (%s of %s).\n' \
            "$wait" "$attempt" "$tries" >&2
        sleep "$wait"
        attempt=$((attempt + 1))
    done
}

# ios_keep_bundle <app> <kept-dir>
#
# Moves the signed bundle into kept-dir, made afresh and readable by its owner
# alone, replacing anything an earlier failed install left there.
ios_keep_bundle() {
    local app="$1" kept="$2"
    rm -rf "$kept"
    (umask 077 && mkdir -p "$kept")
    chmod 700 "$kept"
    mv "$app" "$kept/"
}
