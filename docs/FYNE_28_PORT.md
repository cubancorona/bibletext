# Moving BibleText to Fyne 2.8 — what it costs

*Regression pass run 2026-08-12 against `cubancorona/fyne` branch `bt-2.8`
(= upstream v2.8.0 + our fork stack). Verdict: **do not adopt yet.** The app
compiles and launches, but 2.8 rewrote `widget.PopUp`, and the app's popup
layer breaks at runtime while compiling clean — the worst failure shape.
The app stays on 2.7.4 (`v2.7.4-bt.2`); this file scopes the port.*

## Evidence

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

## Root cause 1 — the overlay unification (upstream `744f4e006`, fixes fyne#707)

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

## Root cause 2 — popup geometry: three separate changes, all silent

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

## A real 2.8 engine bug we found (not our code)

`internal/painter/font.go` introduces a **package-level shared**
`shaping.HarfbuzzShaper`; 2.7.4 allocated one per call. The shaper holds mutable
state (harfbuzz buffer, font LRU), so two goroutines measuring text concurrently
corrupt it. This surfaced as 8 load-dependent `-race` failures, but it is **not
a test artifact — it affects the shipping app.** This is a clean, uncontroversial
upstream bug: worth a fork commit on `bt-2.8` and a strong upstream PR (unlike
the drawloop, whose framing upstream rejected — see FYNE_FORK_PLAN.md).

## Secondary, needs eyes rather than fixes

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
- Only hard API deletion is `canvas.Polygon` → `RegularPolygon`; the app has
  zero uses.

## Not yet on the fork

`bt-2.8` carries the drawloop, caret and emoji commits, but **nothing touches
`GoNativeActivity.java`** — 2.8 still has no `onNewIntent`, so the Android
new-intent patch (in flight in this repo) must land as a fork commit before an
Android build can go the fork route on either branch.

## Recommended order

1. Stay on `v2.7.4-bt.2`. Nothing about 2.8 is urgent.
2. Fix the HarfbuzzShaper race on `bt-2.8` and offer it upstream — cheap, and
   it is the most upstreamable thing we have.
3. Do the port as its own project when there is a reason to: root cause 1 is
   mechanical (~14 sites, one pattern), root cause 2 needs care because
   `sheet_fit.go` and the popup-fit regression net were both written against the
   old renderer's arithmetic. Note the whole net currently fails at the type
   assertion *before* it can report the geometry damage — so fix root cause 1
   first, then let those tests tell you the truth about geometry.
