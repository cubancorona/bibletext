# BibleText on iPad — unified navigation and book typography

The same iOS binary serves iPhone and iPad. Since 1.2.2, every touch device uses
one Read / Books / Search composition rather than maintaining a separate
sidebar-and-split iPad UI.

## Navigation

The destinations, state, and behaviour are shared in `ui_compact.go`:

| Device state | Navigation placement |
| --- | --- |
| iPhone, any orientation | bottom tab bar |
| iPad, portrait | bottom tab bar |
| iPad, landscape | left navigation rail |

Rotating an iPad moves the same three destinations; it does not switch to a
different navigation model. Selecting a book or search result opens it on Read,
and Search remains a normal destination for returning to results. Wide lists
use `readableColumn` rather than stretching across the whole display.

`compactNavRail` in `ui_mobile.go` chooses the rail for a tablet in landscape.
`layoutWatcher` in `ui_regular.go` coalesces resize events and rebuilds when the
resolved bar/rail placement changes. Keyboard appearance is not treated as
rotation because orientation is read from the canvas rather than a laid-out
child.

Android tablets use the same rule. Their tablet identity follows the sw600dp
smallest-dimension convention in `device_android.go`. Android phones also use
the rail in landscape so fixed-height chrome cannot consume the short reading
edge; this Android-specific policy does not change iPhone navigation.

The former `buildRegularWidthUI`, sidebar toggle, 700pt split threshold, and
HSplit sizing helpers remain in the tree as recorded/diagnostic machinery, but
`classifyLayout` deliberately returns the shared layout for every touch device.
They do not describe the shipped UI.

## The reading page: the U.S. Reports layout

The navigation is unified, but iPad reading typography remains device-specific.
(The dev build can also give an iPhone this page in landscape, behind the
gated landscape reading mode — phone_landscape.go and docs/BACKLOG.md.)
`reporterLayoutActive()` enables a centred **27.5em text column**, approximately
58–60 characters per line at the Normal 21px base, with **1.3 leading** and
first-line paragraph indents without blank paragraph gaps.

The leading and paragraph grammar live in `buildChapterHTML` (`reading.go`). The
centred measure is native: `bibleTextSetReadingMeasure` → `btIOSApplyInsets`
(`reading_ios.go`) updates the `UITextView.textContainerInset` from its live
frame. Rotation, Split View, Stage Manager, and text-size changes therefore
re-centre the column without needing a separate reading implementation.

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
- reporter measure at every text-size setting; and
- the iPhone bottom bar remains unchanged.

`simctl io <udid> screenshot out.png` captures the simulator framebuffer. A
landscape capture may be stored in the native portrait buffer and need lossless
rotation before App Store upload; confirm the final pixel dimensions and visual
orientation rather than relying on the filename.

## Shipping to the App Store

Every release since 1.1.0 is universal (`UIDeviceFamily=[1,2]`).
`scripts/release-ios.sh` preserves that requirement; an iPhone-only update cannot
remove iPad support from the existing App Store record.

App Store Connect requires current iPad screenshots. The public 1.2.2 listing
uses eight iPhone and eight iPad images showing the unified navigation. The next
release must read those fields back with `appstore/preflight.py` before
submission; do not assume App Store Connect copied the intended set forward.

Hardware-keyboard shortcuts remain desktop-only unless a later change wires
them explicitly on iPad.
