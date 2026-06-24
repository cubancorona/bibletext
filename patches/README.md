# Vendored Fyne patch

This directory holds a **single, surgical patch to Fyne** plus the documentation
for it. It exists because the iOS scroll-lag fix is a one-line change *inside the
Fyne library*, which can't live in our own source.

| | |
|---|---|
| **Patch** | [`fyne-2.7.4-ios-drawloop.patch`](fyne-2.7.4-ios-drawloop.patch) |
| **Target** | `fyne.io/fyne/v2@v2.7.4` → `internal/driver/mobile/app/{darwin_ios.go, app.go}` + `internal/driver/mobile/driver.go` |
| **Change 1 (scroll lag)** | `drawloop()`'s idle fallback timeout: **`100ms` → `2ms`** (frees the main thread between ticks so native scroll views aren't starved). |
| **Change 2 (scroll flicker)** | `drawloop()` won't return on its idle timeout **while a paint is in progress** — so the GLKView never presents a half-drawn frame. The driver sets a `framePainting` flag around `paintWindow`→`Publish` (`SetFramePainting`); `drawloop` keeps waiting for the complete frame while it's set, and returns fast only when Fyne is genuinely idle. |
| **Applied by** | [`../scripts/setup-fyne-patch.sh`](../scripts/setup-fyne-patch.sh) |
| **Build wiring** | `replace fyne.io/fyne/v2 => ./third_party/fyne` in `go.mod` |

## Why change 2 (no half-drawn frames) is needed

The CADisplayLink calls `render:` → `[glview display]` every tick, and the GLKView
**presents once `drawloop` returns**. With the 2ms idle timeout, if the painter
stalls mid-frame for >2ms (e.g. rasterizing glyph textures as new list rows scroll
in), `drawloop` times out and returns → the GLKView presents a **half-drawn frame**.
`paintWindow` (`driver.go`) does `glClear` then walks the tree, and `container.NewBorder`
draws the **center first, then the edges**, so a half-drawn present shows the list but
the header / bottom bar (drawn last) flash to the clear color = the scroll flicker on
static elements. The fix gates the timeout on `framePainting`: keep waiting for the
complete frame while painting, return fast only when idle (native scroll preserved). It
doesn't change the present cadence, so there's no scroll lag. The old 100ms timeout hid
this by waiting long enough that frames always finished first.

## Why change 1 is needed

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

## Relationship to upstream (PR #5422 / issue #2506)

We reported the iOS symptom upstream ([fyne-io/fyne#6368]) and Andy
(maintainer) declined it — *"not a common pattern… we do not support
multiplexing with other toolkits"* — but pointed at the open desktop draft
**[fyne-io/fyne#5422] "Implement more efficient run loop"** as *"related in
concept not in the actual file."* He's right:

- **Same bug class:** a fixed-cadence, dirty-agnostic render timer that should
  go event-driven (idle until something is actually dirty; wake on demand).
  #5422 fixes it for desktop GLFW (#2506, an idle-CPU complaint) by blocking in
  `glfw.WaitEvents()` when idle and waking via `glfw.PostEmptyEvent()`.
- **Different file, no shared loop code:** #5422 is `internal/driver/glfw/loop.go`;
  ours is `internal/driver/mobile/app/darwin_ios.go`. #5422 is a *design
  template*, not a portable patch — and it's an **unmerged draft** (its
  `WaitEvents`/`WakeUp` code is **not** in our vendored tree; it lives only on
  upstream's branch).
- **Two loops on iOS (the non-obvious bit):** `drawloop()` runs on the **main
  thread** per CADisplayLink tick and is what our 2 ms patch touches; the actual
  structural twin of glfw's `eventTick` is the unconditional 60 Hz `draw` ticker
  in `internal/driver/mobile/driver.go` (dirty-gated in `handlePaint`). A real
  #5422-style port would target both, and must **yield** (pause the CADisplayLink),
  never **block** — UIKit owns the iOS main run loop; Fyne is a guest.

**Our 2 ms change is a mitigation, not the architectural fix** — idle frames
still enter `drawloop` at the display rate (~12% main-thread occupancy vs.
saturation at 100 ms). The principled event-driven fix is a larger redesign with
real iOS hazards (main-thread dispatch for every wake; an `app.Publish()` ↔
`drawloop` deadlock if the link is paused without a reader). The upstream door is
currently closed, so this stays a local patch.

If a mobile-driver efficiency effort ever opens upstream, this is the payload
worth handing them (info only, no pitch):

> Mobile driver has the same idle-loop pattern as #2506/#5422 — and on iOS it
> surfaces as a responsiveness bug, not just idle CPU.
>
> - `drawloop()` (`internal/driver/mobile/app/darwin_ios.go`) runs on the MAIN
>   thread, entered once per CADisplayLink tick. On an idle frame (no
>   `workAvailable`, no `publish`) the select falls through to
>   `case <-time.After(100ms)`, parking the main thread back-to-back.
> - UIKit's run loop is on that same thread, so the park starves a co-resident
>   UIKit view's touch + CoreAnimation commit. Measured on iPhone 16 Pro Max
>   (Instruments `runloop-events`): ~95% of a scroll inside ~100 ms main-thread
>   iterations, with a native `UIScrollView` over a static Fyne canvas.
> - `100ms → 2ms` on that fallback removes the lag. Mitigation only — idle frames
>   still enter `drawloop` at display rate.
> - The structural twin of glfw's `eventTick` on mobile is the 60 Hz `draw`
>   ticker in `internal/driver/mobile/driver.go` (dirty-gated in `handlePaint`,
>   unconditional cadence). A #5422-style event-driven idle would target that +
>   the CADisplayLink.
> - iOS port gotchas: must **yield** (pause CADisplayLink / `enableSetNeedsDisplay`),
>   not block — UIKit owns the main run loop; `paused`/`setNeedsDisplay` mutations
>   need main-thread dispatch; and since `startloop`/`loop()` is unused on iOS, all
>   GL work (incl. `swapBuffers`) is pumped inside `drawloop` gated by
>   `app.Publish()`, so pausing the link naively deadlocks publish (un-pause first).

[fyne-io/fyne#6368]: https://github.com/fyne-io/fyne/issues/6368
[fyne-io/fyne#5422]: https://github.com/fyne-io/fyne/pull/5422

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
