# Fyne 2.8.3 assessment for the next major release

Assess the **final upstream v2.8.3 tag** for the next BibleText major release.
On 21 November 2026, v2.8.2 was the latest stable release and v2.8.3 had
reached RC1; upstream has since tagged v2.8.3 final at the same commit. The
release includes fixes for popups moving under a finger when the Android
keyboard opens, iOS launch under Xcode 27, and excess font memory. The
existing port already uses the same release branch. [Upstream v2.8.3
changelog](https://github.com/fyne-io/fyne/blob/v2.8.3/CHANGELOG.md)

## Starting point on 21 November 2026

BibleText `main` (`ed93b40deb`) uses Fyne 2.7.4. The application port
(`fu/fyne-2.8`, `d3cb25ef3f`) and engine fork (`bt-2.8-next`, `3c3615914`)
are local branches. The port moves to GLFW 3.4, tracks popup lifetimes, sizes
modals before showing them, and uses Fyne CLI 1.7.4. Its macOS build, vet,
full default and `next` test suites, focused race tests, and iOS and Android
arm64 compile checks passed. The engine base is two upstream commits before RC1.
The seven application-port commits touch 140 files, including 58 Go test files.

## Estimated blast radius

The comparison is with the shipping 2.7.4 build. Installation is the first
known failure. Shared UI has the widest reader exposure across iPhone, iPad,
Android, macOS, Windows and Linux. Losing saved notes would have the most
lasting effect on an individual reader. Device upgrades and interrupted-save
checks are still pending. There is no field sample from which to estimate a
percentage of users.

| Area and reach | What a reader or builder could experience | Evidence and remaining uncertainty |
|---|---|---|
| Installation and packaging · high | A clean builder cannot resolve the port's sibling engine checkout. Even an immutable remote `replace` breaks the documented versioned `go run …@latest` source-install route. A Linux build can fail before producing a download. | Both Go module limitations were verified. The port changes seven workflows, and its Linux dependency lists still omit `libwayland-dev`. Test both documented install commands and packaged artifacts from a clean checkout. |
| Popups and reading layout · broad reach | A dialog can hide its only dismissal button. Closing Settings can leave the native reading pane blank. A leaked spinner can keep repainting. | Current `main` has 23 direct popup constructors across 15 production files (18 modal, five non-modal), plus two popup-menu sites. The port tracks all 23 and passes host tests. Its stack-order check still reads the wrapper's `Content` field. Recheck dismissal, rotation and keyboard resizing against the final tag. |
| Platform drivers and bridges · high impact on the affected platform | iOS can miss a touch or display a stale frame. Android can lose a warm shared link or typed text. Windows can open a blank GLES window. Linux can render differently on X11 and Wayland. | The fork carries the behaviours covered by 13 shipping runtime patches. Cross-compilation exercises the mobile source. Launching and exercising the packages tests the driver, native bridge and bundled assets together. |
| Stored notes and settings · high consequence | An interrupted preferences write could lose notes, settings or reading position. | The fork carries atomic preference writes. Upgrade a synthetic profile, interrupt a save, then reopen it and check that a complete old or new profile survives. |
| Fonts and text · long sessions | Font fallback can change verse wrapping most directly on Windows and Linux, where Fyne draws the reading pane. The native readers on Apple platforms and Android depend on Fyne for placement. Repeated theme changes could increase memory use. | Compare verse layouts, native pane placement and retained heap after the same sequence of theme changes on both builds. Investigate sustained growth in the candidate. [Upstream retention report](https://github.com/fyne-io/fyne/issues/6619) |
| Static web reader · indirect exposure | Pages already served run in the browser. A toolkit dependency failure could block the Go generator used for a later publication. | Build and test `cmd/websitegen` and compare its offline fixtures in both build states. The assessment requires no live text fetch or site publication. |

## Assessment and inclusion decision

1. **Before the upstream tag**, bring the application port up to date with
   `main` on its toolkit branch. The merge preview found one conflict in
   `ai_settings.go`. Check the model picker with a long list on a short
   screen. Settle how clean checkouts, CI, release builds and both source
   installs will resolve Fyne. Record any proposed change to a supported
   install route for the release decision.
2. **When v2.8.3 final is tagged**, rebase the engine patch stack onto that
   exact tag under the [fork policy](FYNE_FORK_POLICY.md). Record the app and
   engine revisions. Compare the candidate with the same app changes on 2.7.4
   where possible. Run build, vet, the full default and `next` suites, the CI
   race matrix, and the fork's regression tests.
3. **Before the major release run-up**, verify a clean install and an upgrade
   over 2.7.4 with saved notes, preferences and reading position. Exercise the
   affected flows on iPhone and iPad, Android debug and release packages,
   macOS, Windows desktop GL and GLES, and Linux X11 and Wayland. Use the
   repository's mobile wrappers and record the artifact and proof level for
   each platform in the [platform matrix](PLATFORM_MATRIX.md).
4. **At the inclusion decision**, record the tested revisions, outcomes,
   regressions and outstanding platform evidence under decision 30 in the
   [roadmap](ROADMAP.md). Include the port if the stable build is reproducible,
   the suites pass and the supported install, reading and data-preservation
   paths pass. If the final tag or material evidence is missing, or a blocking
   regression remains, ship the major on 2.7.4 and record what would reopen
   the assessment.

## Historical baseline for v2.8.0

The August comparison below used `bt-2.8` against the 2.7.4 control. Its
failures explain the port's regression tests. The November results above
apply to the newer application and engine branches.

### Evidence

| Check | 2.7.4 (`bt-main`) | 2.8 (`bt-2.8`) |
|---|---|---|
| `go build ./...` | pass | pass |
| `go test -race ./...` | **100% green** | **24 top-level failures** |
| iOS simulator package | pass | pass, zero source changes |
| Android arm64 cross-compile | pass | pass, zero source changes |
| Desktop launch + render | pass | launches, renders; ~4px content shift |

Both mobile targets and the desktop build are fine. Every one of the 24
failures is caused by 2.8 (the 2.7.4 control has no pre-existing failures), and
they reduce to **two root causes**.

### Root cause 1 — the overlay unification (upstream `744f4e006`, fixes fyne#707)

`PopUp.Show()` no longer adds the popup to the overlay stack. It wraps itself in
an **unexported** `internal/widget.OverlayContainer` and adds *that*. So
`Canvas().Overlays().Top()` returns a type the app cannot name, and every
`o.(*widget.PopUp)` assertion silently stops matching. `internal/widget` is
unimportable from this module, so adding a type case is not an option.

16 tests fail on this. More importantly, **three production paths break**:

1. `rebuildWindow`'s overlay drain (`reading.go:406-424`) — the popup is
   bare-removed instead of `Hide()`n, which is exactly what its own comment
   warns against: the popup watchdogs poll `Visible()` and would spin forever.
2. `stopInfiniteBars` (`reading.go:396-409`) — no case matches an
   OverlayContainer, so a drained spinner repaints the canvas forever (the
   battery bug that function exists to prevent).
3. The Settings close-out watchdog (`ai_settings.go:776-786`) tests
   `o == popup` over `Overlays().List()`, now always false — so `done()` never
   runs on an outside-tap close and the reader is left with a **blank verse
   pane**, because `hideReadingOverlay()` is never undone.

**Fix strategy:** stop reading popups back off the overlay stack. Hold the
`*widget.PopUp` the app already created (production paths) and descend through
`test.WidgetRenderer(...).Objects()` when the top overlay is a bare
`fyne.Widget` (test helpers). Also add a comma-ok at
`notes_delete_all_test.go:29`, whose unchecked assertion turns one ordinary
failure into a suite-killing panic.

### Root cause 2 — popup geometry: three separate changes, all silent

- **Centring.** `modalPopUpRenderer` is gone; centring moved to the parent
  overlay renderer, which does not re-run on a child `Resize()`. The app's
  universal idiom is `Show()` *then* `Resize()`, so a modal stays centred for
  its pre-Resize size. Measured on a 320×568 canvas: the version-load-error
  card lands 77pt off the right edge — and OK is its only dismissal. The
  chapter picker lands 89pt below the bottom. ~12 call sites.
- **Coordinate space.** `PopUp.Move`/`Resize` overrides were deleted, so
  positions are now safe-area-relative rather than canvas-absolute. Every popup
  that positions against the safe area double-applies the inset (~44-59pt on a
  notched iPhone).
- **Padding.** Popups no longer add `SizeNameInnerPadding` (8pt here), so
  `MinSize()` reports 16pt less per axis. `sheet_fit.go` is written against the
  old renderer's exact arithmetic and is now off by one inner padding.

Also: `ShowAtPosition` lost Fyne's on-canvas clamp (an app-supplied position is
honoured verbatim, so several sites can now go off-screen), and a popup
deliberately placed at exactly `(0,0)` is treated as unpositioned and re-centred
— which defeats the top-anchored Ask/compose sheets on any zero-inset canvas.

### Upstream v2.8.0 engine race

`internal/painter/font.go` introduces a **package-level shared**
`shaping.HarfbuzzShaper`; 2.7.4 allocated one per call. The shaper holds mutable
state (harfbuzz buffer, font LRU). Concurrent text measurement produced eight
load-dependent `-race` failures on the v2.8.0 candidate. Fyne synchronized the
shared shaper in [v2.8.2](https://github.com/fyne-io/fyne/blob/v2.8.2/internal/painter/font.go#L314-L333).

### Other visual and test differences

- **`RichText` was rewritten** (619 lines; word wrap moved to `wrapWordLines`).
  Break positions and row heights can shift. Highest exposure:
  `reading_mobile.go:142`, the mobile reading pane.
- **Text measurement changed** (`lookupFaces` now takes additional faces and
  passes the symbols font too). `fyne.MeasureText` is load-bearing for our
  custom reading layout — this is the likely cause of the ~4px content shift
  seen on desktop.
- **macOS scroll bars now auto-hide** per `NSScrollerStyle` (18 scroll
  containers).
- `test.StartAnimation` now ticks auto-reversing animations to 0.0 instead of
  1.0 — flips assertions on initial animation state.
- Fyne deprecated `canvas.Polygon` in favour of `RegularPolygon`. BibleText
  used neither.

### Android warm links on the August fork

The August fork carried the drawloop, caret and emoji commits. The Android
`onNewIntent` bridge that delivers a shared link to a running app was still
in the 2.7.4 patch pipeline. The November engine fork carries the bridge.
The check on the finished Android package and a warm-link test on a device
cover two different ways it could fail.
