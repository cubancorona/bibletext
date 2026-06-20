# Vendored Fyne patch

This directory holds a **single, surgical patch to Fyne** plus the documentation
for it. It exists because the iOS scroll-lag fix is a one-line change *inside the
Fyne library*, which can't live in our own source.

| | |
|---|---|
| **Patch** | [`fyne-2.7.4-ios-drawloop.patch`](fyne-2.7.4-ios-drawloop.patch) |
| **Target** | `fyne.io/fyne/v2@v2.7.4` → `internal/driver/mobile/app/darwin_ios.go` |
| **Change** | `drawloop()`'s idle fallback timeout: **`100ms` → `2ms`** (one `case` line; the rest of the hunk is an explanatory comment) |
| **Applied by** | [`../scripts/setup-fyne-patch.sh`](../scripts/setup-fyne-patch.sh) |
| **Build wiring** | `replace fyne.io/fyne/v2 => ./third_party/fyne` in `go.mod` |

## Why the patch is needed

On iOS, Fyne runs its draw routine (`drawloop`) **on the main thread** every
display tick. `drawloop` waits for GL work or a "present" signal and, if neither
arrives, falls back to a timeout. Stock Fyne uses **100 ms**. When a *native* iOS
scroll view (the reading `UITextView`, a `UITableView`) scrolls over a *static*
Fyne canvas, Fyne has nothing to draw — so `drawloop` parked the main run loop for
the full 100 ms **every tick, back-to-back**, starving the native scroll. An
on-device run-loop trace showed **~95% of a scroll spent inside ~100 ms main-thread
iterations**. Shrinking the fallback to **2 ms** frees the main thread between
ticks; dirty frames still return instantly via the work/publish cases, so Fyne's
own rendering is unaffected. (Full investigation: the project's scroll-lag notes.)

## How the build uses it (iOS-only, applied by the iOS scripts)

`go.mod` ships **stock** Fyne with **no `replace`**, so `go build ./...`,
`go run ./cmd/desktop`, and `go test ./...` are one-line with no setup — correct,
because the bug is iOS-only (`//go:build darwin && ios`) and desktop builds are
byte-identical to stock.

The patch is applied **only on the iOS packaging path**. `scripts/run-ios-device.sh`
and `scripts/run-ios-sim.sh` each:

1. run `scripts/setup-fyne-patch.sh` → regenerate `third_party/fyne` (a patched
   copy of stock Fyne v2.7.4; `third_party/` is `.gitignore`d, ~22 MB, never
   committed);
2. `go mod edit -replace fyne.io/fyne/v2=./third_party/fyne` — inject the patch
   for just this build;
3. build/package the iOS app (which now ships the 2 ms fix);
4. restore stock `go.mod` via an `EXIT` trap (success, failure, or Ctrl-C).

So your working tree's `go.mod` is always stock; the `replace` exists only for the
seconds an iOS build runs. **Don't run a bare `fyne package -os ios` yourself** — it
would build against stock Fyne and ship the laggy version. Use the scripts.

## Setup

Nothing to do for desktop. For iOS, just run `scripts/run-ios-device.sh` (or
`run-ios-sim.sh`) — they apply the patch automatically. `setup-fyne-patch.sh` is
safe to run standalone too (it regenerates `third_party/fyne` from the module
cache + this patch, fetching stock v2.7.4 if it isn't cached).

## How to remove the patch entirely (surgical)

When Fyne ships the fix upstream (or you bump to a version that includes it):

1. **Un-hook the iOS scripts:** delete the "apply the iOS-only Fyne patch" block
   (the `setup-fyne-patch.sh` + `go mod edit -replace` + the `EXIT` trap) from
   `scripts/run-ios-device.sh` and `scripts/run-ios-sim.sh`.
2. **Delete the tooling:** `rm -rf third_party/fyne patches/ scripts/setup-fyne-patch.sh`
   (and the `third_party/` line in `.gitignore` if nothing else needs it).
3. **Verify:** `go build ./...` and the iOS scripts both build against stock Fyne.

`go.mod` is already stock, and nothing in the app's own code references the patch,
so removal touches only the items above. (Until upstream lands it, removing the
patch re-introduces the iOS scroll lag.)

## Updating the patch for a new Fyne version

If `go.mod`'s `fyne.io/fyne/v2` version changes, regenerate the patch against the
new version:

```bash
# 1. point the script's FYNE_VERSION + this patch's filename at the new version
# 2. apply by hand to a fresh copy, re-make the one-line edit, then:
diff -u --label a/internal/driver/mobile/app/darwin_ios.go \
        --label b/internal/driver/mobile/app/darwin_ios.go \
        "$(go env GOMODCACHE)/fyne.io/fyne/v2@<NEWVER>/internal/driver/mobile/app/darwin_ios.go" \
        third_party/fyne/internal/driver/mobile/app/darwin_ios.go \
        > patches/fyne-<NEWVER>-ios-drawloop.patch
```

Then confirm the upstream `drawloop` still has the `time.After(100 * time.Millisecond)`
fallback (it may have changed structure between releases).
