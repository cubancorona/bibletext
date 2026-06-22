# Vendored Fyne patches

This directory holds **two surgical patches to Fyne** plus the documentation
for them. It exists because both fixes are small changes *inside the Fyne
library*, which can't live in our own source: the iOS scroll-lag fix (the
drawloop patch) and the focused-Entry CPU fix (the caret-blink patch).

| | |
|---|---|
| **Patch 1** | [`fyne-2.7.4-ios-drawloop.patch`](fyne-2.7.4-ios-drawloop.patch) |
| **Target** | `fyne.io/fyne/v2@v2.7.4` → `internal/driver/mobile/app/{darwin_ios.go, app.go}` + `internal/driver/mobile/driver.go` |
| **Change 1 (scroll lag)** | `drawloop()`'s idle fallback timeout: **`100ms` → `2ms`** (frees the main thread between ticks so native scroll views aren't starved). |
| **Change 2 (scroll flicker)** | `drawloop()` won't return on its idle timeout **while a paint is in progress** — so the GLKView never presents a half-drawn frame. The driver sets a `framePainting` flag around `paintWindow`→`Publish` (`SetFramePainting`); `drawloop` keeps waiting for the complete frame while it's set, and returns fast only when Fyne is genuinely idle. |
| **Patch 2** | [`fyne-2.7.4-caret-blink.patch`](fyne-2.7.4-caret-blink.patch) |
| **Target** | `fyne.io/fyne/v2@v2.7.4` → `widget/entry_cursor_anim.go` |
| **Change 3 (caret CPU burn)** | The Entry caret's smooth fade → a **discrete blink** (snap dim↔opaque at the half-cycle). Cuts a focused-but-idle Entry from ~8 full-canvas repaints/s to 2/s: **~50% CPU → ~22%** (iPad Pro 13" sim, ambient ~20%). Cadence and typing-interrupt behaviour unchanged. |
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

## Why the caret-blink patch is needed

Fyne's Entry caret "blink" is a smooth alpha fade: a forever-repeating 500 ms
`fyne.Animation` (`widget/entry_cursor_anim.go`) whose callback calls
`cursor.Refresh()` on **every animation frame inside the fade band** (the
ease-in-out middle of each half-cycle) — roughly 8 refreshes/second, forever,
while any Entry has focus. Fyne has no partial repaints: each `Refresh()` marks
the canvas dirty, and the next `handlePaint` walks the **entire** widget tree
and re-issues the full GL command stream. So an idle, focused search field kept
the whole canvas repainting ~8×/s.

Measured on the iPad Pro 13" simulator (iPad regular layout, sidebar search
field, `top -l N -pid $(pgrep -f BibleText.app/main)`, 12-sample means):

| state | stock caret fade | discrete blink |
|---|---|---|
| entry focused, idle | **50.4%** (35-57) | **41.9%** (40-49) |
| unfocused ambient | ~24% | ~25.5% |

The patch snaps the caret between dim and opaque at the half-cycle instead of
fading — same 1 s cadence, same typing-interrupt behaviour (caret goes solid
while typing), but only **2 refreshes/second**. Frame-capture verification shows
exactly two caret states (no intermediate alphas).

Two caveats the numbers above teach:

- The **simulator exaggerates the per-repaint cost** — its GL is a software
  rasterizer (JIT-compiled shader code on the CPU), so one full-canvas repaint
  of a 2064×2752 iPad frame costs ~50-100 ms of CPU. On a real device the
  repaint is mostly GPU. The patch removes ~6 repaints/s on **both**.
- Roughly **14 points of the focused-state cost are NOT the caret**: comparing
  deltas across the two builds (+25 pts at ~8 repaints/s vs +16.5 pts at
  2 repaints/s) leaves a fixed, repaint-rate-independent overhead that appears
  the moment an iOS text-input session opens (the hidden native `GoInputView`
  becomes first responder). That's UIKit machinery outside Fyne, also
  sim-inflated; it ends when focus is dismissed.

**Why not Fyne's own animation switch?** v2.7 has `Settings().ShowAnimations()`
(`no_animations` build tag / `fyne_settings` schema): entry.go then skips the
cursor animation entirely. But it's all-or-nothing — it also kills button-tap
feedback, Select popup animation, etc., and the caret stops blinking entirely
(a static caret reads as "stuck" on iOS). The surgical patch keeps the blink
and every other animation.

## Relationship to upstream (PR #5422 / issue #2506)

[fyne-io/fyne#6368] records the iOS symptom and classifies multiplexing with
other toolkits as unsupported. It also identifies the open desktop draft
**[fyne-io/fyne#5422] "Implement more efficient run loop"** as conceptually
related. The technical relationship is:

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

**The caret-blink patch, unlike the drawloop one, is a clean upstream
candidate.** It is platform-neutral (`widget/entry_cursor_anim.go`, no driver
code), strictly reduces work on every platform (desktop repaints the window for
those fade frames too — same bug class as #2506), and changes nothing
user-visible but the fade's smoothness. If it's ever proposed upstream, note the
alternative framings: keep the fade but only Refresh when the *quantized* alpha
actually changed, or respect `ShowAnimations()` with a discrete fallback.

[fyne-io/fyne#6368]: https://github.com/fyne-io/fyne/issues/6368
[fyne-io/fyne#5422]: https://github.com/fyne-io/fyne/pull/5422

## How the build uses it (applied by the mobile packaging scripts)

`go.mod` ships **stock** Fyne with **no `replace`**, so `go build ./...`,
`go run ./cmd/desktop`, and `go test ./...` are one-line with no setup. That is a
statement about LOCAL builds only: the three desktop release jobs in
`.github/workflows/release.yml` run `setup-fyne-patch.sh` and inject the
`replace` too, so the released macOS, Windows and Linux binaries carry the
patches (the caret-blink fix and the current emoji font) just as the mobile
builds do. Only an ad-hoc `go build` on your machine is stock.

The patches are applied on **every mobile packaging path**:
`scripts/run-ios-device.sh`, `scripts/run-ios-sim.sh`, `scripts/release-ios.sh`
(the App Store pipeline), and `scripts/build-android.sh` (the caret-blink fix
is a real battery win on Android; the drawloop hunk is inert off-iOS). Each:

1. run `scripts/setup-fyne-patch.sh` → regenerate `third_party/fyne` (a patched
   copy of stock Fyne v2.7.4; `third_party/` is `.gitignore`d, ~22 MB, never
   committed);
2. `go mod edit -replace fyne.io/fyne/v2=./third_party/fyne` — inject the patches
   for just this build;
3. build/package the iOS app (which now ships both fixes);
4. restore stock `go.mod` via an `EXIT` trap (success, failure, or Ctrl-C).

So your working tree's `go.mod` is always stock; the `replace` exists only
while a packaging script runs. **Don't run a bare `fyne package -os
ios|android` yourself** — it would build against stock Fyne and ship the
laggy / hot-caret version. Use the scripts.

## Setup

Nothing to do for desktop. For iOS run `scripts/run-ios-device.sh` /
`run-ios-sim.sh` / `release-ios.sh`, for Android `scripts/build-android.sh` —
they all apply the patches automatically. `setup-fyne-patch.sh` is
safe to run standalone too (it regenerates `third_party/fyne` from the module
cache + these patches, fetching stock v2.7.4 if it isn't cached).

## How to remove the patches entirely (surgical)

The two patches are independent — to drop just one (e.g. upstream ships one fix),
delete its `.patch` file and its `patch -p1` + verify-grep lines in
`setup-fyne-patch.sh`. To remove everything when upstream ships both (or you bump
to a version that includes them):

1. **Un-hook the packaging scripts:** delete the Fyne-patch block (the
   `setup-fyne-patch.sh` call + `go mod edit -replace` + the `EXIT`-trap
   restore) from `scripts/run-ios-device.sh`, `scripts/run-ios-sim.sh`,
   `scripts/release-ios.sh`, **and** `scripts/build-android.sh`.
2. **Delete the tooling:** `rm -rf third_party/fyne patches/ scripts/setup-fyne-patch.sh`
   (and the `third_party/` line in `.gitignore` if nothing else needs it).
3. **Verify:** `go build ./...` and the iOS scripts both build against stock Fyne.

`go.mod` is already stock, and nothing in the app's own code references the patches,
so removal touches only the items above. (Until upstream lands them, removing the
drawloop patch re-introduces the iOS scroll lag, and removing the caret patch
re-introduces the 30-60% focused-Entry CPU burn.)

## Updating the patches for a new Fyne version

If `go.mod`'s `fyne.io/fyne/v2` version changes, regenerate each patch against the
new version:

```bash
# 1. point the script's FYNE_VERSION + the patch filenames at the new version
# 2. apply by hand to a fresh copy, re-make the edits, then (per patch):
diff -u --label a/internal/driver/mobile/app/darwin_ios.go \
        --label b/internal/driver/mobile/app/darwin_ios.go \
        "$(go env GOMODCACHE)/fyne.io/fyne/v2@<NEWVER>/internal/driver/mobile/app/darwin_ios.go" \
        third_party/fyne/internal/driver/mobile/app/darwin_ios.go \
        > patches/fyne-<NEWVER>-ios-drawloop.patch
diff -u --label a/widget/entry_cursor_anim.go \
        --label b/widget/entry_cursor_anim.go \
        "$(go env GOMODCACHE)/fyne.io/fyne/v2@<NEWVER>/widget/entry_cursor_anim.go" \
        third_party/fyne/widget/entry_cursor_anim.go \
        > patches/fyne-<NEWVER>-caret-blink.patch
```

Then confirm the upstream `drawloop` still has the `time.After(100 * time.Millisecond)`
fallback, and that `entry_cursor_anim.go` still drives the caret via the fade-band
animation (either may have changed structure between releases).

## Patch 3: current emoji (`fyne-2.7.4-noto-emoji.patch` + `NotoColorEmoji.ttf`)

Fyne bundles `EmojiOneColor.otf`, a ~2016 EmojiOne set. Anything added to
Unicode from Emoji 11.0 (2018) onward — 🥺 🤏 🤌 🫶 🫠 — has no glyph there, and a
note carrying one drew a faint notdef box in every Fyne-drawn surface (the
Share-with-note box and the notes browser); for example, 🤏 did not appear.

The swap is two pieces, because a binary cannot ride a unified diff:

- the `.patch` retargets `theme/bundled-emoji.go`'s embed directive, and
- `setup-fyne-patch.sh` copies `patches/NotoColorEmoji.ttf` into
  `third_party/fyne/theme/font/` (and deletes the no-longer-embedded
  EmojiOneColor.otf).

The font is Noto Color Emoji (CBDT/CBLC bitmap build, ~10.7 MB — app grows by
roughly the 6.5 MB difference), fetched 2026-08-11 from
`googlefonts/noto-emoji@main`, licensed OFL 1.1 — the licence text is tracked
beside it as `NotoColorEmoji-LICENSE-OFL.txt` and must ship wherever the font
does. Verified against go-text/render v0.2.1: the renderer's `GlyphBitmap`
path rasterises it (probed: 🤏 draws 1452 opaque pixels at 48px where EmojiOne
drew a notdef box).

Removal: delete the two `patches/NotoColorEmoji*` files and the `.patch`, and
drop the three emoji lines from `setup-fyne-patch.sh`.

NOTE the scope: only builds that go through the patch scripts get this — every
MOBILE build and every desktop RELEASE build (the release workflow applies the
patches). A local `go build`/`go run ./cmd/desktop` still uses stock go.mod and
so still shows Fyne's old emoji set.

## Patch 5: atomic preferences write (`fyne-2.7.4-atomic-prefs.patch`)

**This one is not a performance fix. It is the difference between keeping and
losing the reader's shared notes.**

Fyne rewrites the WHOLE preferences store on every change, and it opened the
real file to do it:

```go
// app/preferences_nonweb.go, storageWriterForPath
file, err := os.Create(path)   // O_TRUNC — the file is now zero bytes
```

`saveToStorage` then marshals the values map and writes it in one go. Between
those two moments the reader's only copy of `preferences.json` is empty. Kill
the process there — force quit, an iOS jetsam, a crash, the kill an app update
performs — and everything is gone: settings, reading position, and every shared
note. An emptied file is not even detectably broken; it parses as "no values",
so `readNotesChecked`'s refusal-to-overwrite guard (`notes_store.go`) cannot
help — that guard catches an *unparseable* store, and zero bytes parse fine.

BibleText hits the window far harder than a settings file normally would:
`persistReadingPosition` (`reading_state.go`) rewrites the blob on **every
chapter turn**, and `flushReadingStateAsync` rewrites it again on **every iOS
finger-lift**, from a background goroutine — two writers, nothing serialising
them. It has already happened here: a torn file turned 6 notes / 6 keys into
0 notes / 1 key on the simulator.

**The fix.** `storageWriterForPath` now returns an `atomicWriter`: the blob is
staged in `preferences.json.tmp-NNNN` **in the same directory** (rename is only
atomic within a filesystem) and `Close` renames it over the target. A reader —
or a relaunch after a kill — sees the whole old file or the whole new one, never
a half of either. Every failure path removes the temp file and leaves the stored
file untouched, because a lost save is recoverable and a published empty one is
not.

Two smaller things ride along, both required for the first to be honest:

- **`preferences.go`**: `defer writer.Close()` became a deferred closure feeding
  a named return. Close is now where the save is *published*, so its error means
  "your preferences never reached disk" — throwing it away would have made
  every failure silent.
- **A once-per-process sweep** of temp files older than a day, since a process
  killed mid-save leaves a whole copy of the store behind. The age cut-off is
  what makes it safe: a recent temp file may belong to a live writer.

**What it costs** (`go test ./app/ -run XXX -bench BenchmarkBT`, 200-note store,
300 iterations x4; all variants measured in one process to control for large
run-to-run latency variation):

| | ns/op |
|---|---|
| stock `os.Create` + write + fsync | 4.07 - 4.21 ms |
| temp + fsync + rename | 5.37 - 5.69 ms |
| **the shipped `atomicWriter`** | **4.86 - 5.74 ms** |
| temp + fsync + rename + **directory fsync** | 8.44 - 9.70 ms |

Note what dominates: the milliseconds are the `fsync` **stock Fyne already
performs** (`os.File.Sync` is `fcntl(F_FULLFSYNC)` on APFS, a full device
barrier). The patch adds roughly **+1 ms per save**.

The last row is the thing we tried and removed. fsyncing the directory so the
rename itself survives a power cut is textbook, and it cost as much again for no
benefit against the threat we actually have: a *killed process* cannot tear a
rename, because the page cache belongs to the kernel and outlives the process.
Without it a power cut can lose the last save and leave the previous complete
file — the outcome we would choose anyway. It also removes a Windows wrinkle
(a directory handle cannot be fsynced there).

**Proof ships with the patch**: `patches/testdata/atomic-prefs_test.go` is
copied into `third_party/fyne/app/` by `setup-fyne-patch.sh` (copied, not
diffed, because it is a whole new file — and copied on *every* run, because the
script deletes `third_party/fyne` first; a test written straight into the
vendored tree survives exactly until the next build). Run it with:

```bash
( cd third_party/fyne && go test ./app/ -run TestBT -v )
```

`TestBTStockWriteIsTorn` is the control — it reproduces the defect with stock's
`os.Create` so the fix's zero means something. A representative run:

```
stock os.Create:      18216 reads, 565 EMPTY (every note lost), 21 partial/invalid
patched atomic write: 13995 reads,   0 empty,                    0 partial/invalid
```

### Desktop coverage — check this before assuming you are covered

A Fyne-side fix protects a build only if that build applies the patches. As of
this writing **every shipped build does**: `release-ios.sh`, `run-ios-sim.sh`,
`run-ios-device.sh`, `build-android.sh`, and all three desktop jobs in
`.github/workflows/release.yml` (macOS arm64+Intel, Windows, Linux) call
`setup-fyne-patch.sh` and inject the `replace`.

What is NOT covered, and cannot be: a bare `go build ./cmd/desktop`, `go run
./cmd/desktop`, the VS Code tasks, and anyone building from a clone by hand.
`go.mod` ships stock on purpose. Those builds still truncate. That is a
tolerable gap for a developer convenience build — but it means the protection
lives in the *release scripts*, so **removing or bypassing the patch step in
any packaging path silently un-fixes data loss**, not just emoji and caret
blink. The verify-grep in `setup-fyne-patch.sh` fails the build if either half
of this patch goes missing.

### Upstream

This is a clean upstream candidate — platform-neutral, no driver code, strictly
safer on every platform, and Fyne's own `resetSavedRecently` already carries the
comment *"writes are not always atomic"*, which is this bug seen from the other
side. If it is ever proposed, note that the patch also deletes stock's
`os.IsExist` fallback in `storageWriterForPath`: it was unreachable (`os.Create`
passes `O_CREATE|O_TRUNC` without `O_EXCL`, so it never returns an exists error)
and wrong if reached (`os.Open` hands back a read-only descriptor that would
fail on the first `Write`).

An **optional follow-up we did not take**: `watchFile` (`app/settings_desktop.go`)
watches the whole config *directory*, so it already fires the reload callback for
any file that changes there. Measured with fsnotify v1.9.0, one save produces
2 events stock (`CHMOD`, `WRITE`) and 4 patched (`CREATE(tmp)`, `REMOVE`,
`CREATE`, `RENAME(tmp)`) — of which the `REMOVE` takes fsnotify's re-watch
branch, so reload callbacks go from 2 to 3. Same class of spurious reload Fyne
already had, one more of them, all inside the `savedRecently` guard. Filtering
`event.Name` against the watched path in `watchFile` would remove them all *and*
fix the pre-existing over-firing, but it changes settings-watch behaviour too,
so it belongs in its own patch with its own testing.
