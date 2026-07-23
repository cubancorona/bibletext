# BibleText — project guide for AI assistants

> **Note for humans:** this file is an internal guide for AI coding assistants.
> If you're here to use or contribute to the project, start with
> [README.md](README.md) (features & usage) and [ARCHITECTURE.md](ARCHITECTURE.md)
> (how it's built).

A cross-platform Bible reader (macOS / Windows / Linux / iOS / Android) from one
Go codebase, built with [Fyne](https://fyne.io/). Module name: `bibletext`.

## Layout

- Repo root = the shared **`bibletext` library package** (all the `*.go` files).
  It is NOT a `main` package — do not try to `go run .` or `go build .` here.
- Entry points live under `cmd/`:
  - `cmd/desktop/main.go` — desktop window (HSplit + sidebar + shortcuts)
  - `cmd/mobile/main.go` — iOS/Android (bottom tabs + touch)

## Build / run / test

```bash
go build ./...                      # compile-check everything (for your host OS)
go run ./cmd/desktop                # fast launch of the desktop reader
go test -race ./...                 # tests live in the root package
gofmt -w .  &&  go vet ./...        # format + vet before committing

# Packaged bundles (run from the cmd dir, not the repo root):
cd cmd/desktop && fyne package -os darwin       --app-id uk.co.bibletext.desktop
cd cmd/mobile  && fyne package -os iossimulator --app-id uk.co.bibletext
```

**Patched Fyne (iOS scroll-lag + caret-CPU fixes).** `go.mod` ships **stock**
Fyne, so `go build ./...` / `go run ./cmd/desktop` / `go test ./...` are
one-line with no setup step. Two small in-Fyne fixes: the `drawloop`
idle-timeout change (100ms→2ms, iOS scroll lag; inert off-iOS) and the Entry
caret discrete-blink change (the stock smooth caret fade full-canvas repaints
~8×/s while any entry has focus → 30-60% CPU / battery burn; the patch snaps
dim↔opaque, 2 repaints/s — this one matters on Android too). Both are applied
by EVERY mobile packaging script — `scripts/run-ios-device.sh`,
`run-ios-sim.sh`, `release-ios.sh` (the App Store pipeline), and
`build-android.sh`: each regenerates a patched Fyne
(`scripts/setup-fyne-patch.sh` → `third_party/fyne`, gitignored) and injects a
temporary `replace fyne.io/fyne/v2 => ./third_party/fyne` for just that build,
restoring stock `go.mod` on exit. Do **not** run a bare
`fyne package -os ios|android` yourself — it would ship the unpatched (laggy,
hot-caret) build; use the scripts. Rationale + measurements + removal steps:
[`patches/README.md`](patches/README.md).

VS Code: `.vscode/tasks.json` wraps all of the above; `launch.json` →
"Debug Desktop App" runs it under the debugger.

**On-demand CI smokes** (all `workflow_dispatch`, `gh workflow run <name>`):
`linux-visual-smoke.yml` (Xvfb + llvmpipe screenshots), `windows-visual-smoke.yml`
(mesa-dist-win llvmpipe `opengl32.dll` beside the exe + desktop screenshots —
the same DLL trick lets humans run the app in GPU-less Windows VMs), and
`windows-audio-smoke.yml` (the `audiosmoke`-tagged end-to-end test in
`audio_smoke_test.go`: real narration download through the real oto/WASAPI
engine — buffering → playing → skip → pause → resume → natural end).

**Android:** toolchain (JDK 21, SDK/NDK r27, bundletool — all under `$HOME`),
the native selection overlay, build/sign/emulator/distribution, and quirks live
in [`docs/ANDROID.md`](docs/ANDROID.md). **Build with `scripts/build-android.sh`**
(debug APK) or `scripts/build-android.sh --release` (signed `.aab` + universal
APK) — NOT bare `fyne package`, which drops the `classes2.dex` bridge and falls
back to the Fyne-widget reading pane. Android has a native selectable-TextView
reading overlay (a `Dialog` over the GL surface; `android/BtBridge.java` +
`reading_android.go`) giving iOS-parity text selection + the study context menu,
AND full audio parity (`android/BtAudio.java` + `audio_android.go`: MediaPlayer
recordings + TextToSpeech read-aloud + read-along + ±15s, PLUS background/
lock-screen playback via `android/BtAudioService.java` — a mediaPlayback
foreground service + framework MediaSession + MediaStyle notification, enabled
by the custom `cmd/mobile/AndroidManifest.xml` on fyne's aapt2 resource path,
which the adaptive-icon layers `cmd/mobile/Icon-foreground/-background.png`
trigger; that manifest must NOT declare versionCode/uses-sdk — fyne passes
those as aapt2 flags. See `docs/ANDROID.md`). `reading_mobile.go`'s
`buildReadingViewMobileFyne` is the fallback when the bridge dex is absent.

## Website (bibletext.co.uk)

The landing/download page, privacy policy, and support page live in `docs/`
on main (`index.html`, `privacy.html`, `support.html`) — the SOURCE OF TRUTH —
but GitHub Pages serves the **`gh-pages` branch root**, so editing `docs/` on
main does not change the live site. To publish: copy the changed files onto a
fresh checkout of `origin/gh-pages` (temp worktree) and push; the branch's
`CNAME` file (`bibletext.co.uk`) must survive every push or the custom domain
detaches. DNS/registrar is Cloudflare (4×A + 4×AAAA GitHub Pages records, www
CNAME). `release.yml`'s asset names (BibleText-macOS-AppleSilicon.zip etc.) are
a stable contract with the page's download links — never rename them; the
Android APK (`BibleText-Android.apk`) is uploaded to the release manually.

## Architecture notes (the non-obvious bits)

- **Build tags select the UI per platform.** Files are tagged
  `//go:build !ios && !android` (desktop) vs `ios || android` (mobile), and
  `darwin` / `darwin && !ios` for native code. gopls only analyses the host
  build, so iOS-tagged files look greyed-out in the editor — that's expected;
  validate them with the `fyne package -os iossimulator` task.
- **The mobile UI picks its layout at RUNTIME, not by build tag (iPad).** The one
  mobile binary serves phones and tablets. `CreateMainUI` (`ui_mobile.go`) calls
  `state.layoutClass()` (`layout.go`): a wide-enough iPad gets the **regular**
  layout — a persistent sidebar beside the reading pane (`buildRegularWidthUI`,
  `ui_regular.go`: an `HSplit` + the app header, structurally the desktop layout)
  — while phones, and an iPad squeezed into a narrow multitasking column, get the
  **compact** bottom-tab layout (`buildCompactUI`). `classifyLayout(width,
  isTablet)` decides: tablets (`deviceIsTablet()` — the UIKit interface idiom
  on iOS, `device_ios.go`; the sw600dp-style smallest-dimension test on Android,
  `device_android.go` reading the live canvas via `isTabletDimensions` in
  `layout.go`; false on desktop) at
  ≥ `tabletLayoutMinWidth` (700pt) are regular. `layoutMayChange()` gates the
  watcher install (iOS: only when the idiom is pad; Android: always — the
  canvas size that makes a device a tablet arrives after the first build). **The regular layout still uses
  the mobile NATIVE reading overlay** (`buildReadingViewMobile` via
  `rebuildMobileReadingPane`), so selection / Study-with-AI / scroll persistence
  are unchanged; the overlay tracks the reading host's real absolute rect
  (`setFrameFromObject`), so it sits over just the right pane with no
  split-specific frame math. On a tablet, `layoutWatcher` wraps the root and
  rebuilds the window when a width change crosses the breakpoint (rotation /
  Split View / Stage Manager); phones never cross it, so their path is unchanged.
  `overlayShouldShow` is layout-aware (regular → visible whenever search results
  aren't occupying the pane). The regular layout's sidebar is **hideable**: a
  header toggle (`iconSidebarLeft`) flips `state.sidebarCollapsed`, and the
  default follows orientation (landscape shown / portrait collapsed,
  `resolveSidebarDefault` — re-asserted on rotation, an explicit toggle sticks
  within an orientation). **Invariant:** the search/Find field lives only in the
  sidebar, so collapsing while `IsSearching` clears the search (also discarding
  any in-flight AI Find via `aiSearchActive`), and `surfaceSearch` re-shows the
  sidebar. To exercise the iPad layout in the simulator:
  `BIBLETEXT_SIM_DEVICE="iPad Pro 11-inch (M5)" scripts/run-ios-sim.sh` (the sim
  build is packaged universal so the iPad runs it natively, idiom=pad, not iPhone
  compatibility mode). Sim quirk: synthetic `type` into Fyne entries is flaky —
  use per-key presses. Release builds ship universal (`release-ios.sh`) — see
  [`docs/IPAD.md`](docs/IPAD.md). iPads also typeset the reading pane to the
  U.S. Reports geometry (centred 27.5em native-inset column, 1.3 leading,
  indented paragraphs — `reporterLayoutActive()`, `reporterMeasureEm`; phones
  unchanged); details in docs/IPAD.md → "The reading page".
- **Native text overlays (cgo).** On macOS the reading pane is a native
  `NSTextView` and on iOS a `UITextView`, floating *above* the Fyne canvas
  (`reading_macos.go` / `reading_ios.go`, Objective-C in the cgo preamble).
  Because they float on top, any Fyne modal (chapter picker, AI panels) must
  call `state.hideReadingOverlay()` on open and `state.showReadingOverlay()` on
  close; a `gReadingSuppressed` latch keeps the overlay down for the whole modal.
  **The iOS UITextView MUST be added to `window.rootViewController.view`, NOT to
  the bare window** (`bibleTextEnsureTV`). The selection edit menu walks the text
  view's responder chain to find a view controller to present from — its ▸
  overflow page, its submenus, and the system actions (Look Up / Translate /
  Define) all need one, and the system actions *crash* without it. A bare
  window-subview has no VC in its chain, so those silently fail / crash while only
  flat top-level taps (Copy) work. The custom selection menu is built in
  `HBReadingTextView`'s `editMenuForTextInRange:` (Study with AI submenu + Share +
  Cross-references, prepended before iOS's suggestedActions).
- **Native → Go bridge.** The `//export` callbacks live in exactly two files —
  `ai_menu_darwin.go` (reading/AI callbacks: `bibleTextAIMenuTapped`,
  `bibleTextReadingScrolled`, etc.) and `audio_export_apple.go`
  (`bibleTextAudioStateChanged`); their cgo preambles must stay empty of C
  *definitions* (only declarations allowed alongside `//export`).
- **Background load + loading screen.** The Bible (~6.4 MB JSON parse +
  `PrepareSearchIndex` over ~31k verses, or a multi-minute first-run API fetch)
  loads on a goroutine via `StartBackgroundLoad` (`app.go`), NOT before the
  window shows — otherwise the iOS launch watchdog SIGKILLs the app on a slow
  first run. Entry points (`cmd/mobile/main.go`, `Run`) build a `NewLoadingState`
  (`loadPhase == loadPending`), show the window immediately, then kick off the
  load; `CreateMainUI` renders `buildLoadingView` (a spinner, `ui.go`) and keeps
  the native overlay detached until `loadPhase == loadReady`, at which point the
  loaded fields are copied into the live state under `fyne.Do` and `rebuildWindow`
  re-pins the overlay + re-arms the saved scroll restore. Offline first run →
  `loadFailed` → `buildLoadErrorView` with Retry (replaces the old fatal
  `os.Exit`). `loadStateData` does the heavy work and returns an error.
- **Reading perf invariants (iOS sluggishness fixes).** Three gates keep the
  native-overlay reading view cheap on every nav/tab tap: (1) `applyTheme`
  (`app.go`) calls `SetTheme` only when the theme object changes — re-running it
  per build forces a full canvas theme-walk; (2) `pushChapterHTML` (iOS) /
  `newMacReadingHost` (macOS) skip the costly HTML rebuild + NSAttributedString
  re-import when `chapterRenderFingerprint` (`reading.go`) is unchanged and no
  scroll restore is pending — the fingerprint MUST include book/chapter/version,
  theme variant, red-letter, and the highlighted-verse identity, or a search-jump
  / light-dark flip would show stale text; (3) live search is debounced via
  `newSearchDebouncer` (`state.go`), whose trailing timer marshals back through
  `fyne.Do`. `Verse.Ref` and `BibleData.chapterNums` are precomputed in
  `PrepareSearchIndex` (on the load goroutine) so search/nav don't re-format or
  re-sort per keystroke.
- **AI study (BYOK).** Select text → native "Study with AI" menu → Explain /
  Analyze context / Analyze translation. (The free-form **"Ask a question…"** verb
  was removed from the selection menu in build 94/1.0.2 — its code — `aiActionAsk`,
  `ai_ask.go`, `promptAskQuestion`, `buildAskPrompt` — remains but is not wired to
  the menu.) The whole AI surface is also runtime-disableable: Settings → Assistant
  → **None** (`keyStore.aiEnabled`, gates the native menus + the Find toggle).
  **Three search/AI verbs, kept
  distinct on purpose:** *Search* = keyword / reference lookup (Search tab), *Find* =
  AI passage search that returns verses (Search-tab toggle, `ai_search.go`), *Ask* =
  AI narrative answer about a selection (reading menu). "Ask a question…" opens a small
  input sheet (`ai_ask.go`, `promptAskQuestion` — full-canvas top-anchored non-modal
  popup on iOS so the field clears the soft keyboard; centered modal on desktop), then
  shows a prose answer grounded in the selection (`buildAskPrompt`). Providers (Gemini /
  OpenAI / Anthropic / Grok) live in `ai_providers.go`; keys are stored on-device via
  `keyStore` (`ai_keystore.go`) over `fyne.Preferences`, with `<PROVIDER>_API_KEY` env
  vars overriding (Grok's is `XAI_API_KEY`, not GROK_). Per-action prompts are built by `buildAIPrompt` / `buildAskPrompt` in
  `ai.go` (shared preamble + per-action task + the quoted selection; the fixed actions
  documented in README → "AI study"). `runAIAction` threads the Ask question and folds
  it into the cache scope. Settings sheet: `ai_settings.go` (header gear). Result panel:
  `ai_panel.go`.
- **Bible versions (translations).** `versions.go` defines `BibleVersion` +
  registry (WEB + BSB public-domain, NRSV/LSB licensed) and a `bibleSource` per
  version (`webSource` = the WEB as ONE request from bible.helloao.org (it replaced
  the old per-chapter bible-api.com walk, which remains only as the seed/fallback);
  `bsbSource` (`bsb.go`) = the
  Berean Standard Bible, public-domain/CC0, fetched as ONE ~7 MB `complete.json`
  from the free, key-less bible.helloao.org and decoded via `decodeBSBComplete`
  mapping helloao's USFM `order` → the app's canonical book names; `licensedAPISource`
  = scaffold gated on a license opt-in + `BIBLE_API_KEY`). The version picker calls
  `switchVersionInteractive` (`versions_ui.go`): in-memory/placeholder versions swap
  synchronously, but a first-time real fetch (the BSB download) runs on a goroutine
  behind a spinner modal so the iOS main-thread watchdog is never at risk — the
  shared apply tail is `applyLoadedVersion`. **Not-yet-licensed versions are NOT user-selectable**
  (`canSelect` = real text available, i.e. `!isTesting()`): the picker
  (`versions_ui.go`) renders them de-emphasized and non-tappable as "evaluation in
  progress", and `switchVersion` refuses them — so no copyrighted placeholder text
  reaches users. The placeholder path (`makePlaceholderBible`, mirrors WEB's
  structure) stays in the code and is unlocked for internal QA by
  `BIBLETEXT_ENABLE_TESTING=1` (`testingVersionsEnabled`); once a license is
  configured the version flips to selectable with real text automatically.
  `switchVersion` swaps `AppState.Bible` and `rebuildWindow`s; per-version cache is
  `bibletext-<id>.json`. UI: the header subtitle is the picker (`versions_ui.go`,
  shared → both platforms). Most versions are the canonical 66-book Protestant canon;
  the **World English Bible (Catholic)** (`webCatholicSource`, `catholic.go`) adds the
  73-book deuterocanon — decoded by USFM **id** (helloao appends the deuterocanon and
  gives the Greek Esther/Daniel, so the order-based `decodeBSBComplete` can't be reused)
  and emitted in traditional Catholic order. Reading/search/AI/navigation are data-driven
  off `BibleData.Books`, so they need no per-version code; 66-book-only features
  (cross-refs, red-letter, verse-of-day) simply skip the deuterocanon. Docs: README →
  "Bible versions".

- **Two reading headers — edit BOTH.** The reading toolbar is built per platform:
  desktop + Android use `chapterHeader` (`reading.go`, via `buildReadingView`),
  but **iOS uses its own `chapterHeaderMobile`** (`reading_ios.go`, via
  `buildReadingViewMobile`). A header control (e.g. the audio play button) must be
  added to *both* or it won't appear on the phone — `reading.go` alone is not the
  iOS path.
- **Per-chapter audio (iOS + macOS + Android).** `audio.go` `recordedURLFor`
  resolves what to play, dispatched by translation so each version plays a recording made from its
  own text: the **BSB** has a COMPLETE CC0 narration (Barry Hays,
  `bsb_audio.go`) and **WEB / WEB-Catholic** a COMPLETE public-domain narration
  (David Williams, `webAudioURL` in `audio.go`) — both all 66 books, streamed from
  the project's own mirror (github.com/cubancorona/bibletext-audio, release assets
  pinning the exact bytes the bundled read-along timings were aligned against);
  any other version, plus the deuterocanon, falls back to
  on-device TTS of the displayed verses (`chapterSpeechText`). All recordings are
  range-seekable (the ±15s skip). Recordings are NAMED per version
  (`recordingsFor` in `audio.go`: id like "bsb-hays" keys the bundled timing tables,
  narrator is the display name) — adding a narrator there is all it takes for a new
  "Recorded · <name>" row to appear in the source menu (`audio_menu.go`), which lets
  the reader CHOOSE between recordings ↔ read-aloud. **Selecting a source never
  starts playback** — `selectSource` only records the per-chapter preference
  (`gAudio.preferred`/`preferredRecID`/`preferredFP`) and stops any now-stale loaded
  audio; the play button is the only thing that begins audio, via `effectiveSource`
  (the chosen source, or the per-chapter default). `audioController` (`audio_controller.go`, the package
  singleton `gAudio`, untagged) tracks play state and drives the per-platform
  `nativeAudio*` shims; the reading-header play button is `audio_button.go`
  (recorded → MediaPlay/Pause; TTS → the bundled `iconAudioWave` waveform glyph in
  `icons_embed.go`), shown only where `chapterAudioAvailable()` (= an engine exists AND this
  chapter has a recording or the platform has TTS).
  **The native engine runs on both Apple platforms.** `audio_ios.go` (cgo,
  `//go:build ios`) wraps AVPlayer + AVSpeechSynthesizer + AVAudioSession(.playback) +
  MPNowPlayingInfoCenter + MPRemoteCommandCenter (±15s `MPSkipIntervalCommand`, no
  track-skip); `audio_macos.go` (`//go:build darwin && !ios`) is the desktop twin —
  the same code MINUS AVAudioSession (macOS has none: no session activation, no
  interruption handler) and using AppKit/NSImage for the Now Playing artwork. State
  posts back via `bibleTextAudioStateChanged` (`audio_export_apple.go`, the
  empty-preamble `//export` twin, `//go:build darwin` so it serves both engines) →
  `applyNativeState` → `fyne.Do`. Android has its own full engine:
  `audio_android.go` + `android/BtAudio.java`, callbacks via
  `audio_export_android.go` / `audio_jni_android.c`; see the Layout note and
  `docs/ANDROID.md`. **Windows/Linux have an engine too** (`audio_other.go`,
  `//go:build !darwin && !android`): recorded narration through oto
  (WASAPI on Windows, ALSA loaded via purego on Linux — CI/dev builds on Linux
  need `libasound2-dev`) decoding with go-mp3, same `nativeAudio*` shim +
  `applyNativeState` transitions, natural-end detection feeding continuous
  chapter advance, generation-counter staleness guards. No TTS there —
  `ttsSupported()` (true only on `darwin`/`android`) hides the read-aloud
  source, and `chapterAudioAvailable()` hides the whole button for chapters
  with no recording. `audioSupported()` is now true everywhere except wasm. **Stale-callback gotcha:** every native delegate/KVO callback is
  gated on the controller's current `mode` (`if (self.mode != BT_MODE_TTS) return;`
  etc.). The AVPlayer's KVO observer is removed in `teardownEngines`, but the
  `AVSpeechSynthesizer` delegate stays wired, so after switching TTS→recording a
  stopped utterance's `didFinish/didCancel` could still fire LATE and post a spurious
  `ENDED` that wiped the freshly-loaded chapter — leaving audio playing but the button
  stuck on ▶. The mode guard drops it; `applyNativeState` also re-asserts `loaded` on
  any playing/paused report as belt-and-suspenders. **Build-tag trap:** a file named
  `*_ios.go` is GOOS=ios-only and `*_darwin.go` is GOOS=darwin-only (which EXCLUDES
  ios) — so the files shared by both Apple platforms (`audio_export_apple.go`,
  `audio_supported_apple.go`) carry NO GOOS filename suffix and use an explicit
  `//go:build darwin` (the `darwin` build *tag* is set for ios AND macos). iOS Background playback needs
  **`UIBackgroundModes=audio`** in Info.plist, which Fyne's iOS packager never emits —
  it's injected by `plutil` in `scripts/run-ios-device.sh`, `release-ios.sh`, and
  `run-ios-sim.sh`, **before** their codesign step. Audio auto-stops on any
  chapter/book/version change (one fingerprint-guarded `stopAudioForNav`,
  `audio_controller.go`, called from `addRecentChapter` in `state.go`, plus
  `applyLoadedVersion`) and on app stop/window-close (raw
  `nativeAudioStop()` from the lifecycle hooks — never `gAudio.stop()`, to avoid
  `fyne.Do` on the off-main shutdown path). **Continuous playback:** when a chapter
  finishes on its own the controller rolls onto the next one and keeps playing in the
  same mode, carrying the reading pane along, until the reader pauses or the Bible
  ends. The native ENDED callback → `applyNativeState(audioEnded)` → `advanceAndContinue`
  → `advanceToNextChapter` (next chapter, crossing book boundaries; stops after
  Revelation 22) → `state.refresh()` (pane follows) → `startChapter` for the new
  chapter. The controller caches the live `boundState` on each start so the
  native-thread callback can reach navigation. It advances ONLY on a natural end
  (a pause posts PAUSED; a manual nav stops via `stopAudioForNav` — neither posts
  ENDED), so there's no runaway. NOTE: the advance hops through `fyne.Do`, so it's
  verified in the foreground; screen-off background continuation would want a
  native-side queue (AVQueuePlayer / queued utterances) instead.
- **Reading-position + history persistence.** `reading_state.go` persists *where
  the reader left off* — translation, book, chapter, the within-chapter **scroll
  position**, and the recent-chapters history — as one JSON blob in
  `fyne.Preferences` (key `reading.state`). Scroll is stored as a **verse anchor**
  (top-visible verse + within-verse delta, with a whole-chapter `scrollFrac`
  fallback) so it survives re-wrap on width/orientation/translation/text-size
  changes (the scripture font scales with the Settings → Reading → Text size
  choice, `textsize.go`; base 21px). Saving: continuously on navigation (`addRecentChapter` /
  `clearHistory` / `switchVersion` → `persistReadingPosition`, chapter pinned to
  top) **and** the precise scroll via `flushReadingState` — on iOS from a native
  scroll-end callback (`bibleTextReadingScrolled`, an `//export` in
  `ai_menu_darwin.go`; the iOS background lifecycle hook is unreliable) plus the
  app-lifecycle/close hooks (`InstallReadingStateFlush`); on macOS the
  window-close/stop hooks. Restoring happens once in `LoadAndPrepareState`
  (`applyRestoredState`, validated against the loaded Bible); the native overlay
  arms a one-shot scroll target (`armPendingRestore` → `armReadingRestore`) that
  `bibleTextScrollReadingTV` / `bibleTextMacScrollTV` apply through their existing
  re-assert cadence and drop on the first user scroll. Verse numbers are located
  in the attributed string by font size (the only runs under 80% of the body
  size — the threshold is derived from the rendered text, not a constant, so it
  tracks the reader's text-size setting). Per-platform
  scroll hooks live in `reading_ios.go` (cgo), `reading_macos.go` (cgo),
  `reading_android.go`/`reading_scroll_android.go` (the JNI bridge), and a no-op
  `reading_scroll_fyne.go` (Linux/Windows restore book/chapter only).

## Conventions

- Always `gofmt -w .` and `go vet ./...`; keep `go test -race ./...` green.
- Fyne mobile-driver hit-testing needs solid widget bounds (use `GridWrap`
  sizing), not a bare `canvas.Text` renderer.
- Wrap modal content the chapter-picker way (`widget.NewModalPopUp` +
  `surface(...)`), and remember the overlay hide/restore dance above.
