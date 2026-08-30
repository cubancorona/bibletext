# Platform mimic — previewing the Windows/Linux app on macOS

This dev-build-only mode makes the macOS **dev** build follow the Windows/Linux
code paths as closely as the codebase allows. It provides a fast local preview
before the CI visual smokes when virtualized graphics cannot reliably exercise
the Windows UI and a Linux environment provides only audio validation.

**The standing rule: this mode is the step BEFORE the CI visual smokes and
real-hardware checks, never a replacement for them.** `linux-visual-smoke.yml`,
`windows-visual-smoke.yml`, `windows-audio-smoke.yml` and the Linux ARM VM
remain the final word on what those platforms actually render and play.

## Launching

```bash
BIBLETEXT_MIMIC=windows go run -tags bibletextdev ./cmd/desktop
BIBLETEXT_MIMIC=linux   go run -tags bibletextdev ./cmd/desktop
```

Any other value (or unset) leaves the mode off. While active, the app header
shows a bold accent **`MIMIC: Windows`** / **`MIMIC: Linux`** badge beside the
title — a screenshot from this mode must never masquerade as the real
platform, and the badge is what enforces that. (The distraction-free
full-screen reading view drops the whole header, badge included — screenshot
mimic sessions with the header up.)

The switch is compiled in only behind the `bibletextdev` tag
(`dev_mimic_on.go` / `dev_mimic_off.go`, the dev_links/dev_autoopen
arrangement). Release builds do not merely ignore `BIBLETEXT_MIMIC` — the code
that reads it does not exist there, `dev_mimic_guard_test.go` proves it in the
ordinary test run, and `TestReleaseScriptsNeverPassTheDevTag` pins that the
release pipelines never pass the tag.

## What the mode covers (the seams it flips)

Every one is a runtime var over a per-platform constant — the house seam
pattern — flipped once at startup by `devApplyMimic` (called first thing in
`Run()`), so release behaviour is bit-identical.

| Seam | Under mimic | What you get |
|---|---|---|
| `useStyledPane` | true | The real `styledReadingPane` — the shipping Windows/Linux reading surface: styled runs, red letters, selection + the shared study menu, verse-anchor scroll persistence. The NSTextView overlay is never created (`setReadingOverlayVisible` early-outs). |
| `sheetConsumeClosure` | true | `installSheetCloseConsume` installs the Windows/Linux sheet-close consume stand-in, so a theme flip / background data swap under an open sheet rebuilds on close instead of leaking — the exact bug class the consume point exists for. |
| `reporterLayout` | false | The styled pane applies its own **width-gated** reporter typesetting (wide pane → centred 27.5em, 1.3 leading, geometric indent; narrow → cozy 1.55), exactly the target platforms' truth. |
| `ttsSupported` | false | No "Read aloud" source row, and **no audio button at all** on chapters without a recording (licensed versions, the deuterocanon) — the most visible Win/Linux audio difference, in the reading header. |
| `nativeNoteSticker` | **true** (not pinned — it follows `useStyledPane`) | Shared notes render as the styled pane's own **in-text sticker** (`reading_styled_note.go`): one card drawn in a band above the note's verse, with a speech tail pointing at the passage, the byline and "K of N in this chapter ›" counts on one row, the sender's words below, and − / ✕ at the top right — the exact Win/Linux surface since 19 Aug. `dev_mimic_on.go` no longer assigns this seam at all: it asks "does the pane draw the note itself?", and `useStyledPane` (set two lines above it) already answers yes. What still comes from the Fyne **banner** on those platforms, and so under mimic: the could-not-read-the-payload **notice**, and the **R4 unplaced** rows, whose sentence does not fit a one-band sticker. |
| `serifFontCandidates` | linux target only | The Georgia candidates are dropped, so the pane falls to the embedded Gelasio — the app's own no-serif-found path. See the caveat below. |
| Share verbs | fallback bodies | `nativeShareText`/`nativeShareImage` on darwin route to the real Win/Linux bodies (`share_fallback.go`): clipboard + notice popup, save-to-`~/Downloads` + file-manager reveal (`open -R` stands in for Explorer/xdg-open). |

Also active by construction (safe, unconditional two-line delegations added to
`reading_macos.go`): within-chapter scroll capture/arm delegates to the styled
pane when it is live (position persistence + history restore work), and the
read-along narration highlight / clear / follow-pill calls forward to the
styled helpers on the UI goroutine exactly as `readalong_other.go` does.

**Identical by construction (checked, not skipped):** keystore
(preferences-backed on every desktop), cache paths, open-URL/pasted-link
handling, device class, overlay recovery, the lifecycle close-intercept +
reading-state flush. These are the *same code* on macOS and Windows/Linux, so
mimic does not touch them. (Storage *locations* differ per OS — `%AppData%` vs
`~/Library` vs `~/.config` — but that is the OS, not app behaviour.)

**Harmlessly present, exercising nothing:** `syncNativeAIMenu` still sets a C
flag no menu reads (the native selection menu never exists under mimic); the
darwin `//export` callbacks never fire; the dev tint benchmark and
sticker-next-tap driver measure nothing (native pane absent). The in-process
window capture (`InstallDebugCapture`) still works and is the right screenshot
tool for mimic sessions — it captures the Fyne canvas the styled pane draws
into.

## What the mode CANNOT prove (do not read mimic as evidence here)

- **Recorded-audio engine.** Playback still runs through AVPlayer
  (`audio_macos.go`), not the Win/Linux oto + go-mp3 engine
  (`audio_other.go`). Scoped out of this phase deliberately: co-compiling the
  two engines (both define the six `nativeAudio*` shims) needs an
  extraction + hook layer that was judged more than the contained-work
  budget. The read-along *display* and the button *gating* still mimic
  correctly (time updates feed the shared controller from whichever engine
  runs). The download/decode/buffering transitions, generation guards,
  countingSeeker seek, natural-end chapter advance — and below them the WASAPI
  backend, the ALSA `dlopen` and its missing-`libasound` failure mode, and
  platform mixers — are provable only by `windows-audio-smoke.yml`, the
  `audiosmoke` test, and the Linux ARM VM.
- **Pixels.** All three desktops share the Fyne/GLFW/GL code, but the Mac
  renders through Apple's Metal-backed GL while CI smokes render llvmpipe and
  real users render vendor drivers. Antialiasing, glyph rasterisation and
  scaling differ per driver. Mimic proves *which widgets/layout/behaviour* the
  Win/Linux build has, not the pixels a Windows or Linux GPU produces.
- **Window chrome, menu bar, shortcut modifier.** Fyne's darwin driver
  supplies the macOS title bar, application menu and Cmd (vs Ctrl) below any
  app seam. Functionally trivial here (the only shortcuts are Find-focus and
  Escape), but never read a mimic screenshot's title bar as evidence.
- **Linux font inventory.** Mimic=linux drops to the embedded Gelasio when the
  macOS host does not expose DejaVu Serif at the Linux paths — the **Linux face
  is approximated**, and real fontconfig/distro variation is not reproducible
  here; only the shipped candidate chain is. (Mimic=windows is a near-exact
  match: macOS loads the same Georgia family Windows ships.)
- **OS integration endpoints.** Explorer `/select` vs `xdg-open` vs the Mac's
  `open -R`, what the default browser is, whether `libasound` is present,
  desktop-portal behaviour — properties of the target OS, not the binary.
  Mimic substitutes the Mac equivalent where a flow must complete.
- **Dev-only darwin extras** (tint benchmark, notes next-tap driver) reach
  native-pane state and measure nothing under mimic — they are dev tooling,
  not app behaviour.

## Not included (deliberate scope)

- The **legacy `chapterText` revert sub-mode** (forcing `useStyledPane` false
  for a Win/Linux target, to eyeball the `styledPaneEnabledOnPlatform` revert
  path). The one-line constant flip in `reading_styled_platform_on.go` remains
  the revert; the primary mimic is already truthful for `redLetterSupported`
  (true on both sides while the styled pane ships), so no red-letter routing
  was needed either.
- A bundled **DejaVu Serif** embed for mimic=linux. The ~350 KB dev-only embed
  was rejected in favor of the Gelasio approximation and this caveat.

## Tests

- `dev_mimic_test.go` (dev tag): seam flips per target, font-candidate
  handling, unknown-target inertness, and the styled pane actually building —
  verses visible in the Fyne tree — on the darwin host.
- `dev_mimic_guard_test.go` (release-shaped): with `BIBLETEXT_MIMIC` set, the
  switch is absent and every seam still answers its platform constant.
