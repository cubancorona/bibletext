# Architecture

BibleText is a cross-platform Bible reader — **macOS, Windows, Linux, iOS, and
Android** — built from a single Go codebase on [Fyne](https://fyne.io/) (v2.7.4). This
document covers how the pieces fit together. See [README.md](README.md) for
features and usage, and [[redacted-retired-private-reference]]([redacted-retired-private-reference]) for the day-to-day developer
guide and the non-obvious invariants.

## Big picture

The whole shared codebase is **one Go library package, `bibletext`** (every
`*.go` file in the repo root). It is *not* a `main` package — you cannot
`go run .` here. Two thin entry points under `cmd/` consume it:

- `cmd/desktop/main.go` — desktop window (HSplit + sidebar + keyboard shortcuts).
- `cmd/mobile/main.go` — iOS / Android; the OS owns the window size and the
  Bible loads on a background goroutine behind a spinner. The mobile build picks
  its chrome **at runtime** by canvas width: phones (and narrow iPad
  multitasking columns) get the compact bottom-tab layout, a wide iPad gets the
  regular sidebar+split layout (see UI architecture below and
  [docs/IPAD.md](docs/IPAD.md)).

Per-platform behaviour is selected at compile time by **Go build tags**, not at
runtime, so each target links only the drivers and native code it needs:

| Tag | Platforms | Examples |
| --- | --- | --- |
| `!ios && !android` | desktop (macOS/Win/Linux) | `ui_desktop.go` |
| `ios \|\| android` | mobile | `ui_mobile.go`, `ui_regular.go` |
| `darwin && !ios` | macOS only | `reading_macos.go` (cgo) |
| `ios` | iOS only | `reading_ios.go` (cgo) |
| `android` | Android only | `reading_android.go`, `audio_android.go` (cgo/JNI) |
| `ios \|\| !darwin` | everything but macOS | `reading_fyne.go` (fallback pane) |
| `!ios && !darwin && !android` | Linux/Win | `reading_scroll_fyne.go` |
| `!darwin && !android` | Linux/Win | `share_other.go` (no-op share stubs), `audio_other.go` (the oto desktop audio engine) |

> Note: gopls analyses only the host build, so iOS/Android/cgo-tagged files look
> greyed-out in the editor. Validate them with `fyne package -os iossimulator`
> (iOS) or `scripts/build-android.sh` (Android).

## Data pipeline

```
embedded seed:  assets/seed/web-gospels.json ──▶ BibleData (Matthew–John)   [instant, offline]
first run:      bible.helloao.org ──fetch──▶ BibleData ──save──▶ on-disk cache (JSON)
every run:      cache ──load──▶ BibleData ──PrepareSearchIndex──▶ in-memory, ready
                      └─ on cache miss/corruption, fall back to fetch, then re-cache
```

The shipped Bible text now comes from the free, **key-less** API at
**bible.helloao.org** — a single request per translation (~7 MB
`complete.json`), not the old per-chapter walk. The legacy bible-api.com path is
retired; `fetch_bible_data.go` survives only as a generic chapter-by-chapter
fallback client.

- [bsb.go](bsb.go) — the helloao client + decoder. `fetchHelloAOComplete`
  downloads a whole-translation `complete.json` and `decodeBSBComplete` maps
  helloao's USFM book `order` onto the app's canonical 66-book names (with
  whitespace tidy-up). Both the BSB (`BSB/complete.json`) and the WEB
  (`ENGWEBP/complete.json`, via `fetchWEBFromHelloAO`) go through this one path.
- [cache.go](cache.go) — versioned cache with an atomic write (temp file +
  rename), structure-validated on load; a corrupt/old cache is discarded and
  refetched. Each version caches to its own file `bibletext-<id>.json` (plus a
  `-v<epoch>` suffix when a decoder change bumps `cacheEpoch`); the default WEB version
  keeps the legacy `bibletext-cache.json` path. Location: OS cache dir, or
  `BIBLETEXT_CACHE_PATH`.
- [seed.go](seed.go) — an **embedded** WEB Gospels seed
  (`assets/seed/web-gospels.json`, `//go:embed`). So a first launch with no
  network opens to Matthew–John instead of a dead-end "couldn't load" screen.
- [bible.go](bible.go) — the `BibleData` model
  (`map[book]map[chapter][]Verse` + ordered `Books`), reference parsing, book
  aliases, and search. `PrepareSearchIndex` precomputes lowercased verse text,
  per-verse `Verse.Ref`, and `chapterNums` so search/nav never re-format or
  re-sort per keystroke (it runs on the load goroutine).
- [fetch_bible_data.go](fetch_bible_data.go) — generic HTTP client with retry /
  exponential backoff / `429` `Retry-After` handling; a fallback, not the primary
  source.

### Background load + loading screen

The heavy work (a ~6.4 MB JSON parse + `PrepareSearchIndex` over ~31k verses, or
a multi-minute first-run fetch) runs on a goroutine via `StartBackgroundLoad`
([app.go](app.go)) **after** the window is shown — otherwise the iOS launch
watchdog would SIGKILL a slow first run. Entry points build a
`NewLoadingState` (`loadPhase == loadPending`), show the window, then kick off
the load. While `loadPhase != loadReady`, `CreateMainUI` renders only
`buildLoadingView` (a spinner) and keeps the native overlay detached; on success
the loaded fields are copied into the live state under `fyne.Do` and
`rebuildWindow` re-pins the overlay and re-arms the saved scroll restore. An
offline first run → `loadFailed` → `buildLoadErrorView` with Retry. The
`loadPhase` state machine lives in [state.go](state.go) (`loadReady` is the zero
value, so a bare `AppState` in tests renders the real UI).

## Module map

The whole shared codebase is one Go package, `bibletext`. The table covers the
real files; `*_test.go` files are omitted.

### Entry points

| File | Responsibility |
| --- | --- |
| `cmd/desktop/main.go` | Desktop entry — calls `bibletext.Run()` |
| `cmd/mobile/main.go` | Mobile entry — `app.NewWithID`, show window + spinner, `StartBackgroundLoad`; packaged via the iOS scripts (`scripts/run-ios-*.sh` / `release-ios.sh`) and `scripts/build-android.sh` |

### Data layer (no UI deps; compile everywhere)

| File | Responsibility |
| --- | --- |
| `bible.go` | `BibleData` model, search ranking, reference parsing, book aliases, `PrepareSearchIndex` |
| `cache.go` | Per-version, atomic, validated on-disk cache (`bibletext-<id>.json`; default WEB at legacy `bibletext-cache.json`) |
| `bsb.go` | helloao `complete.json` client + decoder (backs both WEB and BSB) |
| `catholic.go` | WEB-Catholic decoder: maps helloao USFM **id** → traditional Catholic order (73-book deuterocanon) |
| `audio.go` | Per-chapter audio resolution: `recordedURLFor` (BSB/WEB/TTS dispatch), `chapterHasRecording`, the `chapterAudio` struct, `chapterSpeechText` (TTS text), `chapterAudioFingerprint` |
| `bsb_audio.go` | BSB recorded-narration URLs (Barry Hays, all 66 books, streamed from the project's audio mirror) |
| `fetch_bible_data.go` | Generic chapter-walk HTTP client (retry/backoff/rate-limit) — fallback only |
| `seed.go` | Embedded WEB-Gospels seed for an offline first launch |
| `versions.go` | `BibleVersion` registry + `bibleSource` interface (web/BSB/licensed), `canSelect`, switching |
| `annotation.go` | Verse-anchored annotation store (foundation for notes/highlights) |
| `crossrefs.go` | OpenBible.info TSK cross-references: fetch-once/cache zip, OSIS parsing, per-verse index |
| `parallels.go` | Embedded Gospel synopsis (`assets/parallels/gospel_parallels.json`); parallel-passage lookup |
| `red_letter.go`, `red_letter_data.go` | Words-of-Christ ranges + red-letter toggle |
| `verse_of_day.go` | Daily-rotating Christ-centred verse + jump-to-context |

### Cross-platform state, theme, fonts

| File | Responsibility |
| --- | --- |
| `app.go` | `Run()`, `loadStateData()`, `StartBackgroundLoad`, `applyTheme`, `ObserveSystemThemeChanges` |
| `state.go` | `AppState`, navigation/search/history logic, UI hooks, `loadPhase` machine, `newSearchDebouncer` |
| `reading_state.go` | Reading-position + history persistence (translation/book/chapter/scroll anchor) in `fyne.Preferences` |
| `history.go` | Recent-chapters history list/bar |
| `theme.go` | `palette`, light/dark `bibleTheme`, custom colour names, `surface` modal helper |
| `font.go` | OS-serif discovery (Georgia / DejaVuSerif) — fallback faces for the card/artwork renderers |
| `fonts_embed.go` | Embedded **Atkinson Hyperlegible** UI font family (`//go:embed`, OFL) |

### Shared UI / widgets

| File | Responsibility |
| --- | --- |
| `ui.go` | Shared header (incl. the iPad sidebar-toggle button), loading/error views |
| `ui_desktop.go` | `!ios && !android` — `CreateMainUI` (HSplit + sidebar) + keyboard shortcuts |
| `ui_mobile.go` | `ios \|\| android` — `CreateMainUI` picks compact vs regular at runtime; `buildCompactUI` (bottom tabs: Read / Books / Search), 44pt touch rows |
| `ui_regular.go` | `ios \|\| android` — `buildRegularWidthUI`, the iPad sidebar+split layout (native reading overlay in the right pane) + `layoutWatcher` (rebuild on breakpoint crossing, or orientation flip while regular) |
| `layout.go` | Untagged: `classifyLayout` (compact vs regular by width+idiom, breakpoint 700pt), `regularSplitOffset` (~250pt sidebar), the orientation-driven sidebar default (`resolveSidebarDefault`) |
| `device_ios.go` / `device_android.go` / `device_other.go` | `deviceIsTablet()` — UIKit interface idiom on iOS; sw600dp-style smallest-dimension test on Android (`isTabletDimensions`, live canvas); false on desktop. Also `layoutMayChange()`, gating the `layoutWatcher` install |
| `textsize.go` | Settings → Reading → Text size: the persisted scale (1.0/1.15/1.3) the scripture body renders at on every platform |
| `chapter_header_mobile.go` | `ios \|\| android` — the compact mobile chapter toolbar shared by both native reading views |
| `icons_embed.go` | Bundled icon resources (e.g. the read-aloud waveform glyph) |
| `sidebar.go` | Navigation sidebar (desktop + iPad regular layout): search box, AI Find, book filter, book list |
| `reading.go` | Reading-pane scaffolding: header (incl. the audio control on Apple platforms), chapter HTML build, `chapterRenderFingerprint`, `rebuildWindow` |
| `audio_button.go` | The reading-header audio control: collapsed speaker → expanded mini-player (source indicator + ±15s skip + play/pause), self-refreshing host (no pane rebuild) |
| `audio_menu.go` | Source picker popup — choose recorded narration ↔ read-aloud (sets the preference; never auto-plays) |
| `search.go` | Keyword search results view + match-term highlighting |
| `goto.go` | Chapter / go-to-verse picker modal, book alphabet jump, numeric keyboards |
| `versions_ui.go` | Header translation picker; `switchVersionInteractive` (sync swap or spinner-gated fetch) |

### Native reading overlays (the reading view)

| File | Tag | Responsibility |
| --- | --- | --- |
| `reading_macos.go` | `darwin && !ios` | cgo: native `NSTextView` overlay + scroll capture/restore |
| `reading_ios.go` | `ios` | cgo: native `UITextView` overlay, custom selection menu, scroll hooks |
| `reading_ios_visibility.go` | `ios` | overlay show/hide on lifecycle |
| `reading_android.go` | `android` | cgo/JNI: native selectable `TextView` overlay (a `Dialog` over the GL surface), selection menu, native share, scroll capture/restore — Java half in `android/BtBridge.java` |
| `reading_android_export.go` + `reading_jni_android.c` | `android` | Native → Go callbacks for the Android overlay (the `//export` twin of `ai_menu_darwin.go`) + the JNI thunk definitions |
| `reading_fyne.go` | `ios \|\| !darwin` | Fyne `RichText` fallback reading pane (Linux/Win; on Android the fallback when the bridge dex is absent) |
| `reading_scroll_fyne.go` | `!ios && !darwin && !android` | Fyne-pane scroll capture/restore (Linux/Win): the same verse-anchor persistence as the native overlays, from `chapterText`'s wrap geometry |
| `reading_scroll_native_stub.go` | `darwin \|\| android` | No-op Fyne-scroll shims on the native-overlay platforms (their overlays persist scroll themselves) |
| `overlay_recovery_other.go` / `cache_path_android.go` / `cache_path_other.go` / `ai_menu_sync_other.go` | various | Small per-platform glue: the no-op twin of Android's overlay re-show after activity recreation, per-platform cache dir resolution, no-op AI-menu enabled-state sync off-Apple |
| `reading_mobile.go` / `reading_scroll_android.go` | `android` | Fallback-pane glue + the inert initial-touch hooks |
| `ai_menu_darwin.go` | `darwin` | Native → Go `//export` callbacks: AI-menu tap, iOS scroll-end, highlight clear, keyboard frame (audio has its own `//export` in `audio_export_apple.go`) |

### Audio (per-chapter playback)

| File | Tag | Responsibility |
| --- | --- | --- |
| `audio_controller.go` | (untagged) | `gAudio` — the cross-platform play-state owner: source preference, `playPauseCurrent` / `selectSource` / `effectiveKind`, continuous playback (`advanceToNextChapter`), drives the `nativeAudio*` shims |
| `audio_ios.go` | `ios` | cgo engine: AVPlayer + AVSpeechSynthesizer + AVAudioSession + MPNowPlayingInfoCenter + MPRemoteCommandCenter |
| `audio_macos.go` | `darwin && !ios` | cgo engine twin (same, minus AVAudioSession; AppKit/NSImage artwork) |
| `audio_android.go` | `android` | cgo/JNI engine: drives `android/BtAudio.java` — MediaPlayer (recordings) + TextToSpeech (read-aloud) + audio-focus + a 200 ms read-along position poll |
| `audio_export_android.go` + `audio_jni_android.c` | `android` | Java → JNI thunk → Go `//export` state callbacks (twin of `audio_export_apple.go`) |
| `audio_other.go` | `!darwin && !android` | The desktop audio engine (Windows/Linux): recorded narration via oto (WASAPI / ALSA-through-purego) + go-mp3 — same `nativeAudio*` shim and state transitions as the native engines; no TTS |
| `audio_export_apple.go` | `darwin` | The `bibleTextAudioStateChanged` `//export` (serves both Apple engines) |
| `audio_supported_apple.go` / `audio_supported_android.go` / `audio_supported_other.go` | `darwin` / `android` / rest | Capability gates: `audioSupported()` (true everywhere but wasm) + `ttsSupported()` (true only where a native speech engine exists: Apple + Android). `chapterAudioAvailable()` in `audio.go` combines them per chapter |
| `audio_artwork.go` | (untagged) | Renders the lock-screen "Book Chapter" art card (share-image style) |
| `readalong.go` / `readalong_stub.go` | untagged / `!darwin && !android` | Bundled read-along timing tables (`assets/timings/`, keyed by recording id) driving verse highlight + follow-scroll; no-op stub on Linux/Windows |
| `android/BtAudio.java` + `android/BtAudioService.java` | (dex) | The Java engine + the `mediaPlayback` foreground service (MediaSession + MediaStyle notification) for background/lock-screen playback |

### AI study (bring your own key)

| File | Responsibility |
| --- | --- |
| `ai.go` | Action constants + `buildAIPrompt`; `runAIAction` (cache scope, dispatch) |
| `ai_ask.go` | "Ask a question…" input sheet (`promptAskQuestion`) + `buildAskPrompt` |
| `ai_providers.go` | Gemini / OpenAI / Anthropic / Grok HTTP clients + models |
| `ai_keystore.go` | On-device key storage over `fyne.Preferences` (`keyStore`); env-var override |
| `ai_settings.go` | AI-study settings sheet (provider pick, key paste, Test key) |
| `ai_panel.go` | AI answer panel (prose result, Report button, disclosure line) |
| `ai_search.go` | AI "Find" passage search on the Search tab (returns verses) |
| `ai_menu_darwin.go` | Native selection-menu → Go bridge (shared with reading overlays) |

### Share

| File | Responsibility |
| --- | --- |
| `share.go` | Selection-action dispatcher; "Share with citation" text (Bluebook Rule 5 quote formatting + citation) |
| `share_image.go` | "Share as image" renderer — text-only card, 13 colour schemes × 7 embedded OFL serifs |
| `share_preview.go` | Preview-and-regenerate sheet before sharing |
| `share_other.go` | `!darwin && !android` no-op stubs for `nativeShareText` / `nativeShareImage` (Android's live in `reading_android.go`) |

`CreateMainUI` exists in exactly one of `ui_desktop.go` / `ui_mobile.go` per
build — the Go build tag picks the *platform*. The mobile build then branches
**at runtime** between the compact (phone) and regular (iPad sidebar+split)
layouts by live canvas width (`classifyLayout`, `layout.go`); a `layoutWatcher`
rebuilds the window when a resize crosses the 700pt breakpoint, or — while the
regular layout is up — flips orientation. Desktop has no runtime branching.

## UI architecture

The window is built once by `CreateMainUI`. On desktop the split, header, and
**sidebar are persistent**; only the reading/results pane is swapped on
navigation. The iPad regular layout reuses the same sidebar and header beside
the mobile **native** reading overlay, with a header toggle (and an
orientation-driven default: shown in landscape, collapsed in portrait) that
hides the sidebar for full-width reading — collapsing while a search is active
ends the search, since the search field lives in the sidebar. `AppState` holds
function hooks that the widgets install:

- `showReading()` — rebuild only the reading/results pane.
- `syncSidebar()` — re-highlight the current book (no entry rebuilds).
- `refresh()` — both of the above; the usual post-navigation call.
- `focusSearch()` / `setSearchText()` — used by keyboard shortcuts.
- `hideReadingOverlay()` / `showReadingOverlay()` — pull the native text overlay
  down while a Fyne modal is up (see Reading view).

Typing in the book filter never loses focus because the filter only refreshes
the list *data*, it does not rebuild the sidebar. Toggling light/dark is the one
full rebuild (`palette`-coloured canvas objects are recreated), and
`applyTheme` calls Fyne's `SetTheme` **only when the theme object changes** —
re-running it per build would force a full canvas theme-walk (an iOS perf gate).

## Reading view

The reading pane is a **native text view floating above the Fyne GL canvas**, not
a Fyne widget, on macOS, iOS, and Android:

- **macOS** ([reading_macos.go](reading_macos.go), `darwin && !ios`): a real
  AppKit `NSTextView` (editable=NO, selectable=YES) inside an `NSScrollView`,
  attached to the Fyne window's content view.
- **iOS** ([reading_ios.go](reading_ios.go), `ios`): a real `UITextView`
  attached to the Fyne app's `UIWindow`. It **must** be added to
  `window.rootViewController.view` (not the bare window), because the selection
  edit menu walks the responder chain for a view controller to present from —
  the system Look Up / Translate / Define actions *crash* without one. The custom
  selection menu (Study with AI submenu + Share + Cross-references) is built in
  `HBReadingTextView`'s `editMenuForTextInRange:`.
- **Android** ([reading_android.go](reading_android.go), `android`): a real
  selectable `android.widget.TextView` in a `ScrollView`, floated over the GL
  surface inside a `Dialog`. The Java half
  ([android/BtBridge.java](android/BtBridge.java)) is compiled to a
  `classes2.dex` that `scripts/build-android.sh` injects into the APK — a bare
  `fyne package -os android` omits it, and the app then degrades to the Fyne
  fallback below. The selection action-mode menu carries the same Study with
  AI / Share / Cross-references actions as iOS.
- **Linux / Windows** ([reading_fyne.go](reading_fyne.go),
  `ios || !darwin`): a Fyne `RichText` fallback in a vertical scroll. Verse
  numbers are superscript segments coloured via custom theme colour names so they
  track the active palette. (This is also Android's no-dex fallback, via
  `reading_mobile.go`.)

Chapter content is produced as **HTML** (`buildChapterHTML` in
[reading.go](reading.go)) and imported as an attributed string on the native
side. Because the overlay floats on top, **any Fyne modal** (chapter picker, AI
panels, share sheet) calls `hideReadingOverlay()` on open and
`showReadingOverlay()` on close; a `gReadingSuppressed` latch keeps it down for
the whole modal.

### Reading perf invariants

Three gates keep the native overlay cheap on every nav/tab tap:

1. `applyTheme` re-applies the Fyne theme only when the theme object actually
   changes.
2. The HTML rebuild + attributed-string re-import is skipped when
   `chapterRenderFingerprint` ([reading.go](reading.go)) is unchanged and no
   scroll restore is pending. The fingerprint includes book/chapter/version,
   theme variant, red-letter state, and the highlighted-verse identity — so a
   search-jump or light/dark flip still re-renders.
3. Live search is debounced via `newSearchDebouncer` ([state.go](state.go)),
   whose trailing timer marshals back through `fyne.Do`.

## Bible versions (translations)

[versions.go](versions.go) defines `BibleVersion` + a registry and a
`bibleSource` per version. The interface has a few implementations:

- `webSource` — the public-domain **World English Bible (WEB)**, one helloao
  request (`fetchWEBFromHelloAO` in [bsb.go](bsb.go)).
- `bsbSource` ([bsb.go](bsb.go)) — the public-domain/CC0 **Berean Standard Bible
  (BSB)**, one `BSB/complete.json` request from helloao.
- `webCatholicSource` ([catholic.go](catholic.go)) — the **World English Bible
  (Catholic)**: helloao's WEBC decoded by USFM **id** (not order) and emitted in
  traditional Catholic order, adding the 73-book deuterocanon.
- `licensedAPISource` — a scaffold for a licensed API provider (e.g. API.Bible),
  gated on a license opt-in **and** `BIBLE_API_KEY`. **NRSV** and **LSB** are
  wired here but copyrighted, so they are **not user-selectable**.

`canSelect()` is true only when real, redistributable text is available, so the
picker renders not-yet-licensed versions de-emphasized and non-tappable
("evaluation in progress"), and `switchVersion` refuses them — no copyrighted
placeholder text ever reaches users. A clearly-labelled placeholder path exists
for internal QA, unlocked by `BIBLETEXT_ENABLE_TESTING=1`.

The header subtitle is the picker (`versions_ui.go`, shared across platforms).
`switchVersionInteractive` swaps in-memory/placeholder versions synchronously but
runs a first-time real fetch (the BSB download) on a goroutine behind a spinner
modal — so the iOS main-thread watchdog is never at risk; the shared apply tail
is `applyLoadedVersion`, ending in `switchVersion` → swap `AppState.Bible` →
`rebuildWindow`. Reading / search / AI / navigation are data-driven off
`BibleData.Books`, so they need no per-version code — most versions are the
canonical 66-book Protestant canon, and the WEB-Catholic's 73-book deuterocanon
simply flows through, while 66-book-only features (cross-refs, red-letter,
verse-of-day) skip it. See README → "Bible versions".

## Per-chapter audio (recorded narration & read-aloud)

A reading-header control plays the current chapter as a recorded human **narration**
or on-device **read-aloud** (text-to-speech). Available on **iOS, Android, and
macOS**; `audioSupported()` is false on Linux/Windows, so the control doesn't
appear there.

**Source resolution** ([audio.go](audio.go)) is dispatched by translation so each
version plays a recording made from its own text: **BSB** has a complete CC0 narration
(Barry Hays — [bsb_audio.go](bsb_audio.go), all 66 books) and **WEB / WEB-Catholic**
a complete public-domain narration (David Williams — `webAudioURL`, all 66 books);
everything else (other versions, the deuterocanon) falls back to TTS of the on-screen
verses (`chapterSpeechText`). Both recordings stream from the project's own audio
mirror (github.com/cubancorona/bibletext-audio, GitHub release assets), which pins the
exact bytes the bundled read-along timings (`assets/timings/`) were aligned against.
Recordings are HTTP-range-seekable (the ±15-second skip). The reader **chooses** the
source from a popup ([audio_menu.go](audio_menu.go)); choosing only sets a per-chapter
preference — the **play button** is the only thing that starts audio.

**The controller** ([audio_controller.go](audio_controller.go), the package singleton
`gAudio`, untagged) owns play state and resolves `(version, book, chapter)` → a native
call, tracking what the native layer reports back so the button renders the right
glyph. **Continuous playback:** when a chapter finishes on its own it rolls onto the
next chapter (crossing book boundaries, stopping after Revelation 22), carrying the
reading pane along, until the reader pauses or the Bible ends — driven by the native
ENDED callback, gated so a pause / manual nav (which don't post ENDED) can't trigger
it. The hand-off hops through `fyne.Do`: on **Android** the foreground audio service
keeps the run loop draining, so chapters roll on with the screen off
(emulator-verified); on **iOS** backgrounded continuation is still a planned
follow-up, since iOS suspends the UI loop in the background.

**The native engine** is cgo, on both Apple platforms: [audio_ios.go](audio_ios.go)
(`ios`) and [audio_macos.go](audio_macos.go) (`darwin && !ios`) wrap AVPlayer (recorded
MP3) + AVSpeechSynthesizer (TTS) + MPNowPlayingInfoCenter + MPRemoteCommandCenter (±15s
`MPSkipIntervalCommand`, no track-skip). iOS additionally uses AVAudioSession(.playback)
+ `UIBackgroundModes=audio` for background playback (injected by the iOS packaging
scripts); macOS has neither (a desktop app plays in the background for free). State
posts back through one `//export`, `bibleTextAudioStateChanged`
([audio_export_apple.go](audio_export_apple.go), `//go:build darwin` so it serves both
engines) → `applyNativeState` → `fyne.Do`.

**Android** has its own full engine: [audio_android.go](audio_android.go) drives
[android/BtAudio.java](android/BtAudio.java) over JNI — MediaPlayer for recordings,
TextToSpeech for read-aloud, AudioManager focus handling, and a 200 ms position poll
for read-along. Callbacks travel Java `native` method → JNI thunk
([audio_jni_android.c](audio_jni_android.c)) → the `//export`s in
[audio_export_android.go](audio_export_android.go) → the same `applyNativeState`.
Background / lock-screen playback runs through
[android/BtAudioService.java](android/BtAudioService.java): a `mediaPlayback`
foreground service + framework `MediaSession` + MediaStyle notification (play/pause,
±15s, artwork), enabled by the custom `cmd/mobile/AndroidManifest.xml` on Fyne's
aapt2 resource path — build details in [docs/ANDROID.md](docs/ANDROID.md).
**Windows/Linux** play recorded narration through their own engine,
[audio_other.go](audio_other.go) (`//go:build !darwin && !android`): oto
(WASAPI on Windows; ALSA loaded at runtime through purego on Linux — building
on Linux needs `libasound2-dev`) decoding the narration MP3s with go-mp3. It
speaks the same `nativeAudio*` shim and posts the same `applyNativeState`
transitions — play/pause, ±15s seek, natural-end detection feeding continuous
chapter advance, generation-counter staleness guards — so the controller and
the whole audio UI are unchanged. Deliberately out of scope there: TTS
(`ttsSupported()` gates every read-aloud surface), media keys / MPRIS / SMTC,
and read-along highlight (the Fyne pane has no per-verse highlight hook).

**Build-tag trap:** a `*_ios.go` filename is GOOS=ios-only and `*_darwin.go` is
GOOS=darwin-only (which *excludes* iOS), so files shared by both Apple platforms
(`audio_export_apple.go`, `audio_supported_apple.go`) carry **no** GOOS filename suffix
and an explicit `//go:build darwin` (the `darwin` build *tag* is set for ios AND macos).

**Stale-callback gotcha:** every native delegate/KVO callback is gated on the
controller's current `mode` — the `AVSpeechSynthesizer` delegate stays wired across a
teardown (unlike the AVPlayer's KVO observer), so without the guard a stopped
utterance's late `didFinish` could post a spurious `ENDED` after the next source had
started. Audio auto-stops on any chapter/book/version change (`stopAudioForNav`) and on
app stop / window-close (raw `nativeAudioStop()` from the lifecycle hooks — never
`gAudio.stop()`, to avoid `fyne.Do` on the off-main shutdown path).

## Cross-references, parallels, red-letter, verse of the day

- **Cross-references** ([crossrefs.go](crossrefs.go), `crossref_panel.go`) — the
  public-domain/CC-BY **OpenBible.info** Treasury of Scripture Knowledge set,
  fetched once as a ~2 MB zip from `a.openbible.info`, cached, then fully
  offline. OSIS refs are parsed into a per-verse index, vote-ranked.
- **Gospel parallels** ([parallels.go](parallels.go)) — an **embedded** synopsis
  (`assets/parallels/gospel_parallels.json`, `//go:embed`). For a Gospel verse,
  the same event in the other Gospels is surfaced first, tagged **Parallel**
  (`crossRef.Parallel = true`), so it works without any network.
- **Red-letter mode** ([red_letter.go](red_letter.go),
  `red_letter_data.go`) — words-of-Christ verse ranges; toggle persisted in
  preferences; folded into the reading fingerprint.
- **Verse of the day** ([verse_of_day.go](verse_of_day.go)) — a deterministic
  daily-rotating Christ-centred verse with a jump-to-context.

## AI study (bring your own key)

Select a passage → native "Study with AI" menu with three actions: **Explain**,
**Analyze context**, **Analyze translation** (constants
`aiActionExplain/Context/Translation` in [ai.go](ai.go); the free-form **Ask a
question…** verb was removed from the menu in build 94/1.0.2 — its code,
`aiActionAsk` / [ai_ask.go](ai_ask.go), remains but is unwired). Plus an
AI **Find** passage search on the Search tab ([ai_search.go](ai_search.go)) and
plain keyword **Search**. The three search/AI verbs are kept distinct on purpose:
*Search* = keyword/reference lookup, *Find* = AI passage search returning verses,
*Ask* = AI narrative answer about a selection.

Both search paths are **supersession-safe**: Find submissions are
generation-stamped (`aiSearchSession`, [ai_search.go](ai_search.go)) so a slow
completion for an abandoned query can never clobber a newer search, and the
keyword debouncer (`newTrailingDebouncer`, [state.go](state.go)) carries the same
stamp so a fired-but-not-yet-marshalled run drops itself when Enter or a newer
keystroke supersedes it (pinned in `search_race_test.go`).

- Prompts are built by `buildAIPrompt` / `buildAskPrompt` ([ai.go](ai.go),
  [ai_ask.go](ai_ask.go)): a shared even-handed preamble + per-action task + the
  quoted selection. Only the selected text plus its **book and chapter** (not the
  verse number) leave the device. Sent as one user message at temperature `0.4`,
  capped `4096` output tokens; identical requests are cached in memory.
- Providers Gemini / OpenAI / Anthropic / Grok live in
  [ai_providers.go](ai_providers.go). Keys are stored **on-device only** via
  `keyStore` over `fyne.Preferences` ([ai_keystore.go](ai_keystore.go)); a
  `<PROVIDER>_API_KEY` env var overrides. Settings sheet:
  [ai_settings.go](ai_settings.go) (header gear). Result panel with a **Report**
  button and an in-app disclosure line: [ai_panel.go](ai_panel.go).
- The `//export` callbacks are confined to two files: [ai_menu_darwin.go](ai_menu_darwin.go)
  (`bibleTextAIMenuTapped`, `bibleTextReadingScrolled`, and the other reading/AI
  callbacks) and [audio_export_apple.go](audio_export_apple.go)
  (`bibleTextAudioStateChanged`); their cgo preambles must contain only C
  *declarations* (no definitions), as required alongside `//export`.

See README → "AI study" for exactly what is sent.

## Share

From the selection menu ([share.go](share.go), dispatched by
`dispatchSelectionAction`):

- **Share with citation** — plain text: the formatted quote + a reference line.
  Quote and citation follow **Bluebook** style: spelled-out translation, en-dash
  ranges, and the Rule 5 quotation rules — the 50-word block-quote threshold
  (counting quoted words only), quotation-mark nesting (5.1(b)),
  wholly-enclosed quotations (5.2(f)(i)), bracketed initial capitals
  (5.3(b)(i)), and " . . . ." end omissions that preserve the original
  sentence's terminal punctuation (5.3(b)(iii)). The formatter is pinned by
  corpus-grounded tests plus a real-world sweep over the embedded Gospels seed
  (`bluebook_test.go`, `share_realworld_test.go`).
- **Share as image** ([share_image.go](share_image.go)) — a text-only card
  (no imagery) with a dynamic colour treatment, serif typesetting, and a clean
  citation; preview/regenerate via [share_preview.go](share_preview.go).
  13 colour schemes × 7 embedded book serifs (Gelasio, Cardo, Crimson Text,
  Spectral, Libre Baskerville, Prata, DM Serif Display — all SIL OFL 1.1,
  `assets/fonts/share/`), so cards render identically on every platform. Scheme
  and typeface are picked per-verse by independent FNV hashes of the reference;
  the counts are **coprime**, so Regenerate walks all 91 pairings before any
  repeat.

Both hand off to the device's native share sheet on iOS / macOS / Android (the
Android share `Intent` goes through the bridge — `nativeShareText` /
`nativeShareImage` in [reading_android.go](reading_android.go));
[share_other.go](share_other.go) provides graceful no-ops on Linux/Windows.

## Reading-position + history persistence

[reading_state.go](reading_state.go) persists *where the reader left off* —
translation, book, chapter, the within-chapter **scroll position**, and the
recent-chapters history — as one JSON blob in `fyne.Preferences` (key
`reading.state`). Scroll is stored as a **verse anchor** (top-visible verse +
within-verse delta, with a whole-chapter `scrollFrac` fallback) so it survives
re-wrap on width / orientation / translation / text-size changes (the body
renders at a 21px base scaled by Settings → Reading → Text size,
[textsize.go](textsize.go); verse-number runs are located by RELATIVE size —
the only runs under ~80% of the body size, a threshold derived from the
rendered text — so the anchor survives the reader changing text size too).

- **Saving:** continuously on navigation (`addRecentChapter` / `clearHistory` /
  `switchVersion` → `persistReadingPosition`, chapter pinned to top) **and** the
  precise scroll via `flushReadingState` — on iOS from a native scroll-end
  `//export` (`bibleTextReadingScrolled` in `ai_menu_darwin.go`; the iOS
  background lifecycle hook is unreliable) plus app-lifecycle/close hooks
  (`InstallReadingStateFlush`); on macOS the window-close/stop hooks.
- **Restoring:** once in `LoadAndPrepareState` (`applyRestoredState`, validated
  against the loaded Bible); the native overlay arms a one-shot scroll target
  (`armPendingRestore` → `armReadingRestore`) applied through the existing
  re-assert cadence and dropped on the first user scroll.
- **Platform split:** scroll hooks are real cgo on iOS/macOS and JNI on Android
  (`captureReadingAnchor` / `armReadingRestore` in
  [reading_android.go](reading_android.go)); the Fyne platforms
  ([reading_scroll_fyne.go](reading_scroll_fyne.go), Linux/Win) capture and
  restore the SAME verse anchor from `chapterText`'s wrap geometry — full
  scroll-persistence parity with the native overlays.

## Threading

Fyne v2.7.4 provides **`fyne.Do`** for main-thread dispatch. All widget mutation
must still happen on the UI goroutine: compute off-thread if you must, but
marshal UI changes back through `fyne.Do` (the search debouncer's trailing timer,
the background-load apply tail, and the version-switch apply tail all do this).
Do not `Refresh()` or mutate widgets directly from `time.AfterFunc`/`go`
routines — that races with rendering; verify with `go test -race ./...`.

cgo / native caveat: on macOS, Fyne runs `OnStopped` **off** the main thread
during shutdown, so any cgo on the reading-state flush path must not
`dispatch_sync(main)` or the app hangs on quit. The audio engine posts state from
the native audio thread through its `//export` → `applyNativeState` → `fyne.Do`;
continuous playback's chapter→chapter advance also hops through `fyne.Do`, which is
why it's foreground-only on iOS (iOS suspends the UI loop in the background); on
Android the foreground audio service keeps the run loop draining, so the same
advance works with the screen off.

Widget tests using `fyne.io/fyne/v2/test` are tagged `//go:build !race` because
Fyne's *test app* clears its font cache on a background goroutine when settings
change, which the detector flags against text measurement — that race is in the
test harness, not the app.

## Extending

- **Add a translation:** register a `BibleVersion` in [versions.go](versions.go)
  with a `bibleSource` (public-domain → a helloao-style source like `bsbSource`;
  licensed-via-API → `licensedAPISource`; licensed-as-data → a small file-based
  source satisfying the same interface). Gate it behind `canSelect` /
  license env vars so copyrighted text only loads once licensed. Bump
  `cacheSchemaVersion` in [cache.go](cache.go) if the on-disk shape changes.
- **Add a selection-menu action:** wire a new constant + handler into
  `dispatchSelectionAction` ([share.go](share.go)) and the native menu builders
  (`reading_macos.go` / `reading_ios.go`).
- **Different AI provider:** add a client in [ai_providers.go](ai_providers.go)
  and surface it in [ai_settings.go](ai_settings.go).
- **Add a recorded-audio source:** register a `recording` in `recordingsFor`
  ([audio.go](audio.go)) — an id (also the read-along timing-table key), the
  narrator's display name, and a per-chapter URL func (see `bsbAudioURL` /
  `webAudioURL`). It appears as its own "Recorded · <narrator>" row in
  [audio_menu.go](audio_menu.go) automatically; chapters the recording doesn't
  cover fall back to TTS.

## Cross-platform builds

Desktop targets compile from `./cmd/desktop` (Fyne pulls in OpenGL/GLFW). Plain
`go` commands need no setup — `go.mod` ships **stock** Fyne:

```bash
go run ./cmd/desktop                                  # fast desktop launch
GOOS=linux   GOARCH=amd64 go build -o bibletext-linux ./cmd/desktop
GOOS=windows GOARCH=amd64 go build -o bibletext.exe   ./cmd/desktop
GOOS=darwin  GOARCH=arm64 go build -o bibletext-macos ./cmd/desktop
go test -race ./...                                   # tests live in the root package
```

Mobile targets are packaged by the `fyne` CLI from `./cmd/mobile` (it sets up the
iOS SDK / Android NDK CGO toolchain and assembles the bundle with `FyneApp.toml`
+ `Icon.png`):

```bash
cd cmd/mobile && fyne package -os iossimulator --app-id uk.co.bibletext
./scripts/build-android.sh                # Android debug APK
./scripts/build-android.sh --release      # signed .aab + universal APK
```

**Android must be built with `scripts/build-android.sh`**, not a bare
`fyne package -os android`: the script compiles `android/*.java` to a
`classes2.dex` and injects it into the APK (the native reading overlay + the
background-audio service live there) and carries the custom
`cmd/mobile/AndroidManifest.xml`. Toolchain setup (JDK 21, SDK/NDK under
`$HOME`), signing, and distribution: [docs/ANDROID.md](docs/ANDROID.md).

**Patched Fyne (iOS scroll-lag + caret-CPU fixes).** Two small in-Fyne changes —
the iOS drawloop idle timeout (100ms→2ms, `//go:build darwin && ios`; inert
elsewhere) and the Entry caret discrete-blink fix (8→2 repaints/s while any
entry has focus; a real CPU/battery win on iOS AND Android) — are applied by
every mobile packaging script: `scripts/run-ios-sim.sh`,
`scripts/run-ios-device.sh`, `scripts/release-ios.sh`, and
`scripts/build-android.sh` (each regenerates a patched Fyne into
`third_party/fyne` — gitignored — and injects a temporary `replace` for that
build, restoring stock `go.mod` on exit). Do **not** run a bare
`fyne package -os ios|android` yourself; use the scripts. Rationale + the
patches + removal steps: [`patches/README.md`](patches/README.md). iOS device
installs additionally need Xcode code-signing (see `scripts/run-ios-device.sh`).

Fyne needs a C toolchain and the platform's graphics/dev libraries on every
target — see the [Fyne docs](https://docs.fyne.io/started/).

## Licensing of bundled data

The source code is **Apache License 2.0** ([LICENSE](LICENSE)). Bundled data and
assets keep their own licenses ([NOTICE](NOTICE)):

- Scripture: **World English Bible** (incl. the Catholic deuterocanon edition) and
  **Berean Standard Bible** — public domain.
- Audio narration: **Berean Standard Bible** recording (Barry Hays) — CC0;
  **World English Bible** recording (David Williams) — public domain; both streamed
  from the project's [audio mirror](https://github.com/cubancorona/bibletext-audio).
- Cross-references: **OpenBible.info** Treasury of Scripture Knowledge — **CC BY**.
- UI font: **Atkinson Hyperlegible** (Braille Institute) — **SIL OFL 1.1**.
- Share-card serifs: **Gelasio, Cardo, Crimson Text, Spectral, Libre Baskerville,
  Prata, DM Serif Display** — all **SIL OFL 1.1**
  (`assets/fonts/share/OFL-LICENSES.txt`).
