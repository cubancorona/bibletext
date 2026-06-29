# BibleText — project guide for AI assistants

> **Note for humans:** this file is an internal guide for AI coding assistants.
> If you're here to use or contribute to the project, start with
> [README.md](README.md) (features & usage) and [ARCHITECTURE.md](ARCHITECTURE.md)
> (how it's built).

A cross-platform Bible reader (macOS / Windows / Linux / iOS) from one Go
codebase, built with [Fyne](https://fyne.io/). Module name: `bibletext`.

## Layout

- Repo root = the shared **`bibletext` library package** (all the `*.go` files).
  It is NOT a `main` package — do not try to `go run .` or `go build .` here.
- Entry points live under `cmd/`:
  - `cmd/desktop/main.go` — desktop window (HSplit + sidebar + shortcuts)
  - `cmd/mobile/main.go` — iOS/Android (bottom tabs + touch)

## Build / run / test

```bash
go build ./...                      # compile-check everything (host = macOS)
go run ./cmd/desktop                # fast launch of the desktop reader
go test -race ./...                 # tests live in the root package
gofmt -w .  &&  go vet ./...        # format + vet before committing

# Packaged bundles (run from the cmd dir, not the repo root):
cd cmd/desktop && fyne package -os darwin       --app-id uk.co.bibletext.desktop
cd cmd/mobile  && fyne package -os iossimulator --app-id uk.co.bibletext
```

**Patched Fyne (iOS scroll-lag fix).** `go.mod` ships **stock** Fyne, so
`go build ./...` / `go run ./cmd/desktop` / `go test ./...` are one-line with no
setup step. The fix is a one-line change to Fyne's iOS `drawloop` idle timeout
(100ms→2ms) that only matters on iOS (`//go:build darwin && ios`), so it is
applied ONLY on the iOS packaging path: `scripts/run-ios-device.sh` and
`run-ios-sim.sh` regenerate a patched Fyne (`scripts/setup-fyne-patch.sh` →
`third_party/fyne`, gitignored) and inject a temporary `replace fyne.io/fyne/v2
=> ./third_party/fyne` for just that build, restoring stock `go.mod` on exit. Do
**not** run a bare `fyne package -os ios` yourself — it would ship the unpatched
(laggy) build; use the scripts. Rationale + the one-line patch + removal steps:
[`patches/README.md`](patches/README.md).

VS Code: `.vscode/tasks.json` wraps all of the above; `launch.json` →
"Debug Desktop App" runs it under the debugger.

## Architecture notes (the non-obvious bits)

- **Build tags select the UI per platform.** Files are tagged
  `//go:build !ios && !android` (desktop) vs `ios || android` (mobile), and
  `darwin` / `darwin && !ios` for native code. gopls only analyses the host
  build, so iOS-tagged files look greyed-out in the editor — that's expected;
  validate them with the `fyne package -os iossimulator` task.
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
- **Native → Go bridge.** `ai_menu_darwin.go` has the repo's only `//export`
  callback (`bibleTextAIMenuTapped`); its cgo preamble must stay empty of C
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
- **AI study (BYOK).** Select text → native "Study with AI" menu → Ask a question /
  Explain / Analyze context / Analyze translation. **Three search/AI verbs, kept
  distinct on purpose:** *Search* = keyword / reference lookup (Search tab), *Find* =
  AI passage search that returns verses (Search-tab toggle, `ai_search.go`), *Ask* =
  AI narrative answer about a selection (reading menu). "Ask a question…" opens a small
  input sheet (`ai_ask.go`, `promptAskQuestion` — full-canvas top-anchored non-modal
  popup on iOS so the field clears the soft keyboard; centered modal on desktop), then
  shows a prose answer grounded in the selection (`buildAskPrompt`). Providers (Gemini /
  OpenAI / Anthropic / Grok) live in `ai_providers.go`; keys are stored on-device via
  `keyStore` (`ai_keystore.go`) over `fyne.Preferences`, with `<PROVIDER>_API_KEY` env
  vars overriding. Per-action prompts are built by `buildAIPrompt` / `buildAskPrompt` in
  `ai.go` (shared preamble + per-action task + the quoted selection; the fixed actions
  documented in README → "AI study"). `runAIAction` threads the Ask question and folds
  it into the cache scope. Settings sheet: `ai_settings.go` (header gear). Result panel:
  `ai_panel.go`.
- **Bible versions (translations).** `versions.go` defines `BibleVersion` +
  registry (WEB + BSB public-domain, NRSV/LSB licensed) and a `bibleSource` per
  version (`webSource` = bible-api.com, per-chapter; `bsbSource` (`bsb.go`) = the
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
- **Per-chapter audio (iOS only).** `audio.go` `recordedURLFor` resolves what to
  play, dispatched by translation so each version plays a recording made from its
  own text: the **BSB** has a COMPLETE CC0 narration (Barry Hays) streamed
  per-chapter from openbible.com (`bsb_audio.go`, all 66 books); **WEB /
  WEB-Catholic** use the *partial* public-domain eBible WEB set (`ebibleAudioBooks`);
  any other version, plus unrecorded books / the deuterocanon, falls back to
  on-device TTS of the displayed verses (`chapterSpeechText`). All recordings are
  range-seekable (the ±15s skip). The source menu (`audio_menu.go`) lets the reader
  switch recording ↔ read-aloud and is where additional narrators/recordings would
  surface as rows. `audioController` (`audio_controller.go`, the package
  singleton `gAudio`, untagged) tracks play state and drives the per-platform
  `nativeAudio*` shims; the reading-header play button is `audio_button.go`
  (recorded → MediaPlay/Pause; TTS → the bundled `iconSpeak` voice glyph in
  `icons_embed.go`/`assets/icons/speak.svg`), shown only where `audioSupported()`.
  The engine is **iOS-only**: `audio_ios.go` (cgo, `//go:build ios`) wraps
  AVPlayer + AVSpeechSynthesizer + AVAudioSession(.playback) + MPNowPlayingInfoCenter
  + MPRemoteCommandCenter (±15s `MPSkipIntervalCommand`, no track-skip); state posts
  back via `bibleTextAudioStateChanged` (`audio_export_ios.go`, the empty-preamble
  `//export` twin) → `applyNativeState` → `fyne.Do`. **Stale-callback gotcha:** every
  native delegate/KVO callback is gated on the controller's current `mode`
  (`if (self.mode != BT_MODE_TTS) return;` etc.). The AVPlayer's KVO observer is
  removed in `teardownEngines`, but the `AVSpeechSynthesizer` delegate stays wired, so
  after switching TTS→recording a stopped utterance's `didFinish/didCancel` could still
  fire LATE and post a spurious `ENDED` that wiped the freshly-loaded chapter — leaving
  audio playing but the button stuck on ▶. The mode guard drops it; `applyNativeState`
  also re-asserts `loaded` on any playing/paused report as belt-and-suspenders.
  Everything else (macOS desktop,
  Linux, Windows, Android) gets no-op `nativeAudio*` stubs (`audio_other.go`,
  `//go:build !ios`) so the host build/tests stay cgo-free. Background playback needs
  **`UIBackgroundModes=audio`** in Info.plist, which Fyne's iOS packager never emits —
  it's injected by `plutil` in `scripts/run-ios-device.sh`, `release-ios.sh`, and
  `run-ios-sim.sh`, **before** their codesign step. Audio auto-stops on any
  chapter/book/version change (one fingerprint-guarded `stopAudioForNav` hook in
  `addRecentChapter`, plus `applyLoadedVersion`) and on app stop/window-close (raw
  `nativeAudioStop()` from the lifecycle hooks — never `gAudio.stop()`, to avoid
  `fyne.Do` on the off-main shutdown path).
- **Reading-position + history persistence.** `reading_state.go` persists *where
  the reader left off* — translation, book, chapter, the within-chapter **scroll
  position**, and the recent-chapters history — as one JSON blob in
  `fyne.Preferences` (key `reading.state`). Scroll is stored as a **verse anchor**
  (top-visible verse + within-verse delta, with a whole-chapter `scrollFrac`
  fallback) so it survives re-wrap on width/orientation/translation changes (font
  is fixed at 19px). Saving: continuously on navigation (`addRecentChapter` /
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
  in the attributed string by font size (the only sub-19px runs). Per-platform
  scroll hooks live in `reading_ios.go` (cgo), `reading_macos.go` (cgo), and a
  no-op `reading_scroll_fyne.go` (Linux/Windows/Android restore book/chapter only).

## Conventions

- Always `gofmt -w .` and `go vet ./...`; keep `go test -race ./...` green.
- Fyne mobile-driver hit-testing needs solid widget bounds (use `GridWrap`
  sizing), not a bare `canvas.Text` renderer.
- Wrap modal content the chapter-picker way (`widget.NewModalPopUp` +
  `surface(...)`), and remember the overlay hide/restore dance above.
