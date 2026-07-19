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

## Hiding the sidebar (the iPad convention)

A leading **sidebar-toggle button** in the header (the `sidebar.left` glyph,
`iconSidebarLeft`) hides/shows the sidebar so the reader can reclaim the full
width — the standard iPad affordance. Shown only in the regular layout (desktop
has its own always-on sidebar; the compact layout uses bottom tabs).

The **default follows orientation**: shown in landscape, collapsed in portrait,
so a portrait iPad reads full-width like Books. `resolveSidebarDefault`
(`layout.go`) applies that default on the first regular build and re-applies it
whenever the orientation flips, while an explicit toggle **within the same
orientation is preserved** (`sidebarInit` / `sidebarLandscape` track this).
Toggling flips `state.sidebarCollapsed` and rebuilds; when collapsed the reading
pane is built full-width (no `HSplit`) and the sidebar-widget hooks
(`syncSidebar` / `focusSearch` / `setSearchText`) become no-ops. The native
overlay re-clips to whichever rectangle the reading pane occupies (full width or
the split's right pane) automatically.

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
a transparent widget that, on `Resize`, rebuilds the window on either of two
triggers: the new width **crosses the compact/regular breakpoint** (Split View or
Stage Manager resizing), or — while the layout is regular — the canvas **flips
between portrait and landscape** (a full-screen iPad rotation never crosses the
700pt breakpoint, so this second trigger is what re-applies the orientation-driven
sidebar default and recomputes the split offset on rotation). A burst of resizes
during a live drag is coalesced into one rebuild. Phones are never wrapped (they
never hit either trigger), so the phone path is byte-for-byte unchanged.

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

## Shipping to the App Store

Since **1.1.0** the release build ships **universal** (`UIDeviceFamily=[1,2]`):
[`scripts/release-ios.sh`](../scripts/release-ios.sh) sets it explicitly, by
default. App Review does **not** allow an update to remove a shipped device
family, so the `BIBLETEXT_IPAD=0` iPhone-only escape hatch is for local
experiments only — an iPhone-only upload would now be rejected.

An App Store version with iPad support needs **iPad screenshots** in App Store
Connect (the 13" display size is required; `simctl io <udid> screenshot` on an
iPad Pro 13-inch simulator produces the right 2064×2752 PNGs directly, no bezel).

## Verified / not-yet-verified

- **Verified on iPad simulators (interactive, 11" + 13"):** the regular layout in
  both portrait and landscape, the native overlay correctly clipped to the reading
  pane (full-width and split); the sidebar toggle expand/collapse in both
  orientations; the orientation default re-asserting on a live rotation both
  directions (which also exercises the `layoutWatcher` orientation-flip rebuild);
  search / Find with the soft keyboard up (incl. the 1.1.1 fix for the
  keyboard-height rebuild loop — see the CRITICAL note on `layoutWatcher.Resize`
  in `ui_regular.go`); the compact (iPhone) layout unregressed. Plus
  `classifyLayout` / `regularSplitOffset` unit tests, `-race` suite, `go vet`,
  iOS + Android cross-compile. Shipped: 1.1.0 (universal) and the 1.1.1 fixes
  are live on the App Store.
- **Not yet runtime-verified (covered by unit tests + logic):** the
  compact↔regular breakpoint crossing on a live Split-View / Stage-Manager resize;
  hardware-keyboard shortcuts on iPad (not wired — the desktop `Cmd-F` / `Esc`
  shortcuts live in `installShortcuts`, which is desktop-only).
