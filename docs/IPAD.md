# BibleText on iPad — unified navigation and book typography

The same iOS binary serves iPhone and iPad. Since 1.2.2, every touch device uses
one Read / Books / Search composition rather than maintaining a separate
sidebar-and-split iPad UI.

## Navigation

The destinations, state, and behaviour are shared in `ui_compact.go`:

| Device state | Navigation placement |
| --- | --- |
| iPhone, portrait | bottom tab bar |
| iPhone, landscape | left navigation rail on Books and Search; the Read tab reads full-screen (`phone_landscape.go`, on by default) |
| iPad, portrait | bottom tab bar |
| iPad, landscape | left navigation rail |
| iPad window (Split View, Stage Manager) | left navigation rail while the window is wider than tall, bottom tab bar otherwise |

One rule decides every row: the rail while the window is wider than it is
tall (a square window counts as wide), the bottom bar otherwise
(`mobileRailWanted` in `layout.go`). Before 1.2.18 the iPhone was the
exception and kept the bar held sideways; its Books and Search tabs drew it 874
or 956 points wide, wider than `tabBarSpreadMaxWidth`, so the tabs sat
bunched in the middle in the iPad's centred dress. That dress is now drawn
only by a window taller than wide and wider than 560 points: an iPad held
upright, or a tall iPad window that wide.

Rotating an iPad or an iPhone moves the same three destinations; it does not
switch to a different navigation model. Selecting a book or search result opens
it on Read, and Search remains a normal destination for returning to results.
Wide lists use `readableColumn` rather than stretching across the whole
display.

`compactNavRail` in `ui_mobile.go` asks that rule of the live canvas
(`railForWindow`). `layoutWatcher` in `ui_regular.go`, installed on every phone
and tablet, coalesces resize events and rebuilds when the resolved bar/rail
placement changes. Keyboard appearance is not treated as rotation because
orientation is read from the canvas rather than a laid-out child.

The rail keeps clear of a sideways iPhone's Dynamic Island, and of the inset
UIKit reports opposite it, by the same rule as everything else in the window:
the driver lays the window's tree inside the safe area, so the rail stands
beside the inset rather than under it, and no padding of its own is added
(`tab_rail.go`; `tab_rail_side_insets_test.go` measures it).

Android phones and tablets use the same rule; an Android tablet's identity
follows the sw600dp smallest-dimension convention in `device_android.go`, and
matters to the layout only before the window has a size. On the Read tab
both phone platforms go further and drop the navigation altogether in
landscape, reading full-screen, on the book page when the width allows it;
the entry in [BACKLOG.md](BACKLOG.md) records the mode's state.

The former `buildRegularWidthUI`, sidebar toggle, 700pt split threshold, and
HSplit sizing helpers remain in the tree as recorded/diagnostic machinery, but
`classifyLayout` deliberately returns the shared layout for every touch device.
They do not describe the shipped UI.

## The reading page: the U.S. Reports layout

The page is chosen by the WIDTH the reading pane has, as on every surface
(`reading_page.go`, docs/READING_TYPOGRAPHY.md "The reading page"), not by the
device — save for a chapter pushed before the window has any size, which an
iPad sets on the book page until its pane reports a width: the book page — a centred **27.5em text column**, approximately 58–60
characters per line at the Normal 21px base, first-line paragraph indents
without blank paragraph gaps — whenever the column and 15pt each side fit; the
phone page otherwise. A full-screen iPad reads the book page in both
orientations at Normal and Large; an iPad mini in portrait (744pt) reads the
phone page at Extra large, as does a narrow Split View or Stage Manager window at
any size, and an iPhone in landscape reads the book page at Normal because its
width allows it. The pane reports its width with every frame
(`setFrameFromObject`), and a width that settles on the other page re-renders
the chapter in place (`reading_page_width.go`). Until 24 September 2026 the
choice was by device — every iPad, and a landscape iPhone with its typography
switch on — and the book page's leading was 1.3; both pages now use the spec's
pitch.

The leading and paragraph grammar live in `buildChapterHTML` (`reading.go`). The
centred measure is native: `bibleTextSetReadingMeasure` → `btIOSApplyInsets`
(`reading_ios.go`) updates the `UITextView.textContainerInset` from its live
frame. Rotation and text-size changes therefore re-centre the column without
needing a separate reading implementation. A window the reader resizes (Split
View, Stage Manager, the resizable windows of iPadOS 26 and later) is laid out
for its own size: the canvas is the window, not the screen
(`patches/fyne-2.7.4-ios-window-size.patch`, Patch 10 in patches/README.md).

The native `UITextView` still supplies selection, Study with AI, sharing, notes,
audio/read-along, and scroll restoration. Navigation placement does not change
those behaviours.

## Testing in Simulator

Use the repository wrapper so the universal device family, patched Fyne tree,
and native bridge are all present:

```bash
go install fyne.io/tools/cmd/fyne@v1.7.2
BIBLETEXT_SIM_DEVICE="iPad Pro 11-inch (M5)" scripts/run-ios-sim.sh
```

Verify at least:

- portrait bottom bar and landscape left rail;
- Read / Books / Search state across rotation;
- grouped Books grid and readable-width Search/Notes lists;
- native reading overlay frame after rotation and Split View resizing;
- selection menus, notes, audio, and scroll restoration;
- the book page at every text-size setting on a full-screen iPad, and the phone
  page in a Split View or Stage Manager window narrower than the column plus
  15pt each side; and
- the iPhone's Books and Search show the left rail in landscape, standing
  beside the Dynamic Island with the phone turned either way, and the bottom
  bar upright (the Read tab reads full-screen in landscape).

`simctl io <udid> screenshot out.png` captures the simulator framebuffer. A
landscape capture may be stored in the native portrait buffer and need lossless
rotation before App Store upload; confirm the final pixel dimensions and visual
orientation rather than relying on the filename.

## Shipping to the App Store

Every release since 1.1.0 is universal (`UIDeviceFamily=[1,2]`).
`scripts/release-ios.sh` preserves that requirement; an iPhone-only update cannot
remove iPad support from the existing App Store record.

App Store Connect requires current iPad screenshots. Until 1.2.17, the
released version's images were the eight iPhone and eight iPad captures made
for 1.2.2, which show the unified navigation: the replacement set prepared for
1.2.3 was never uploaded, and App Store Connect copied the older images forward
with each release. The 1.2.17 release carries new iPhone and iPad sets,
uploaded on 29 September 2026 and live since 1.2.17 was released; how they are taken is in
docs/SCREENSHOT_PLAYBOOK.md. The next release must read those fields back
with `appstore/preflight.py` before submission; do not assume App Store Connect
copied the intended set forward.

Hardware-keyboard shortcuts remain desktop-only unless a later change wires
them explicitly on iPad.
