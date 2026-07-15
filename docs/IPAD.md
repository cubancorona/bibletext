# BibleText on iPad — the regular-width layout

The **one iOS binary serves both iPhone and iPad**. Rather than a separate build,
the mobile UI chooses its chrome at **runtime** from the live canvas width, so an
iPad gets a desktop-style two-pane layout while iPhones keep the phone layout —
and an iPad squeezed into a narrow multitasking column falls back to the phone
layout automatically.

## The two layouts

| | Compact (phone / narrow) | Regular (iPad, wide) |
|---|---|---|
| Chrome | app header + full-screen tab body + bottom tab bar (Read / Books / Search) | app header + `HSplit`: **sidebar** (search/find + book list) beside the **reading pane** |
| Built by | `buildCompactUI` (`ui_mobile.go`) | `buildRegularWidthUI` (`ui_regular.go`) |
| Reading pane | native overlay (`buildReadingViewMobile`) | **same** native overlay |
| Navigation | tap a book/result → switch to Read tab | sidebar is always present; picking a book just refreshes the pane |

The regular layout is **structurally the desktop layout** — it reuses the
platform-agnostic `buildSidebar` and `buildHeader` verbatim — but its reading
pane is the mobile **native overlay** (iOS `UITextView` / Android selectable
`TextView`), so text selection, the Study-with-AI menu, audio, and scroll
persistence are all unchanged from the phone.

## How the layout is chosen

`CreateMainUI` (`ui_mobile.go`) → `state.layoutClass()` (`layout.go`):

```
classifyLayout(width, isTablet):
    phone            → compact
    tablet & width≥700pt (or width unknown) → regular
    tablet & width<700pt                     → compact   (narrow Split View / Slide Over)
```

- **`deviceIsTablet()`** is the UIKit interface idiom (`device_ios.go`,
  `UIUserInterfaceIdiomPad`). It's fixed for the process and known at launch —
  unlike the canvas size, which is 0 until the first layout — so an iPad's first
  real frame is already the regular layout with no phone-layout flash. It is
  `false` everywhere off iOS (`device_other.go`), so **Android tablets and the
  desktop are untouched** (an Android idiom check can be added later without
  touching the shared logic).
- **`tabletLayoutMinWidth` = 700pt**: an iPad mini in portrait (744pt) clears it;
  a half-width column on an 11" iPad (~397pt portrait) stays compact, where the
  sidebar would crowd the reading column.
- **`regularSplitOffset(width)`** aims for a consistent ~250pt sidebar (clamped to
  18–30% of width) so the panel doesn't balloon on a 13" iPad.

### Reacting to width changes

On a tablet, `CreateMainUI` wraps the root in **`layoutWatcher`** (`ui_regular.go`),
a transparent widget that, on `Resize`, rebuilds the window if the new width
crosses the breakpoint — rotation, Split View, or Stage Manager resizing. A burst
of resizes during a live drag is coalesced into one rebuild. Phones are never
wrapped (they never cross the breakpoint), so the phone path is byte-for-byte
unchanged.

## Why the native overlay "just works" in a split

The iOS `UITextView` / Android `TextView` overlay floats **above** the Fyne
canvas, pinned to the reading host's rectangle. `setFrameFromObject`
(`reading_ios.go`) projects the host's **actual absolute canvas rect** and size —
so when the host occupies only the right pane of the split, the overlay lands over
just that pane, clips to it, and follows the divider as it's dragged. There is no
split-specific frame math. `overlayShouldShow` is layout-aware: in the regular
layout the overlay is visible whenever search results aren't occupying the pane
(in the compact layout it's the Read tab, with no search).

## Testing in the simulator

```bash
BIBLETEXT_SIM_DEVICE="iPad Pro 11-inch (M5)" scripts/run-ios-sim.sh
```

`run-ios-sim.sh` packages the sim build **universal** (`UIDeviceFamily=[1,2]`) so
the iPad runs it **natively** (idiom = pad). Without that, an iPhone-only build
runs on iPad in *iPhone compatibility mode*, where the idiom reports iPhone and
the regular layout never appears. (The **App Store** device family is separate —
see below.) `simctl io <udid> screenshot out.png` grabs the sim framebuffer
directly, which is handy for capturing the layout.

## Shipping to the App Store — still pending

The App Store build is **still iPhone-only** (`UIDeviceFamily=[1]`), set by
[`scripts/release-ios.sh`](../scripts/release-ios.sh). To ship the universal
(iPad) build:

1. Build the release with `BIBLETEXT_IPAD=1` so `release-ios.sh` writes
   `UIDeviceFamily=[1,2]`.
2. Add **iPad screenshots** to App Store Connect (a new required display size).
3. Re-check the layout on the iPad screen sizes ASC requires (12.9"/13" and 11").

## Verified / not-yet-verified

- **Verified:** the regular layout renders on an iPad Pro 11" simulator (portrait)
  with the native overlay correctly clipped to the reading pane; the compact
  (iPhone) layout is unregressed; `classifyLayout` / `regularSplitOffset` unit
  tests; `-race` suite; iOS + Android cross-compile.
- **Not yet runtime-verified (covered by unit tests + logic):** landscape and the
  `layoutWatcher` rebuild on a live rotation / Split-View resize; hardware-keyboard
  shortcuts on iPad (not wired — the desktop `Cmd-F` / `Esc` shortcuts live in
  `installShortcuts`, which is desktop-only).
