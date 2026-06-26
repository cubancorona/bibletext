# Architecture

BibleText is a cross-platform Bible reader — **macOS, Windows, Linux, and iOS** —
built from a single Go codebase on [Fyne](https://fyne.io/) (v2.7.4). This
document covers how the pieces fit together. See [README.md](README.md) for
features and usage, and [CLAUDE.md](CLAUDE.md) for the day-to-day developer
guide and the non-obvious invariants.

## Big picture

The whole shared codebase is **one Go library package, `bibletext`** (every
`*.go` file in the repo root). It is *not* a `main` package — you cannot
`go run .` here. Two thin entry points under `cmd/` consume it:

- `cmd/desktop/main.go` — desktop window (HSplit + sidebar + keyboard shortcuts).
- `cmd/mobile/main.go` — iOS / Android (bottom tabs + touch); the OS owns the
  window size and the Bible loads on a background goroutine behind a spinner.

Per-platform behaviour is selected at compile time by **Go build tags**, not at
runtime, so each target links only the drivers and native code it needs:

| Tag | Platforms | Examples |
| --- | --- | --- |
| `!ios && !android` | desktop (macOS/Win/Linux) | `ui_desktop.go` |
| `ios \|\| android` | mobile | `ui_mobile.go` |
| `darwin && !ios` | macOS only | `reading_macos.go` (cgo) |
| `ios` | iOS only | `reading_ios.go` (cgo) |
| `ios \|\| !darwin` | iOS + Linux/Win/Android | `reading_fyne.go` |
| `!ios && !darwin` | Linux/Win/Android | `reading_scroll_fyne.go` |
| `!darwin` | non-Apple | `share_other.go` |

> Note: gopls analyses only the host build, so iOS/cgo-tagged files look
> greyed-out in the editor. Validate them with `fyne package -os iossimulator`.

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
  refetched. Each version caches to its own file `bibletext-<id>.json`. Location:
  OS cache dir, or `BIBLETEXT_CACHE_PATH`.
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
| `cmd/mobile/main.go` | Mobile entry — `app.NewWithID`, show window + spinner, `StartBackgroundLoad`; packaged via `fyne package -os ios/android -src ./cmd/mobile` |

### Data layer (no UI deps; compile everywhere)

| File | Responsibility |
| --- | --- |
| `bible.go` | `BibleData` model, search ranking, reference parsing, book aliases, `PrepareSearchIndex` |
| `cache.go` | Per-version, atomic, validated on-disk cache (`bibletext-<id>.json`) |
| `bsb.go` | helloao `complete.json` client + decoder (backs both WEB and BSB) |
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
| `font.go` | OS-serif loading (Georgia / DejaVuSerif), used for share-image rendering |
| `fonts_embed.go` | Embedded **Atkinson Hyperlegible** UI font family (`//go:embed`, OFL) |

### Shared UI / widgets

| File | Responsibility |
| --- | --- |
| `ui.go` | Shared header, theme toggle, loading/error views |
| `ui_desktop.go` | `!ios && !android` — `CreateMainUI` (HSplit + sidebar) + keyboard shortcuts |
| `ui_mobile.go` | `ios \|\| android` — `CreateMainUI` (bottom tabs: Read / Books / Search), 44pt touch rows |
| `sidebar.go` | Desktop sidebar: search box, book filter, book list |
| `reading.go` | Reading-pane scaffolding: header, chapter HTML build, `chapterRenderFingerprint`, `rebuildWindow` |
| `search.go` | Keyword search results view + match-term highlighting |
| `goto.go` | Chapter / go-to-verse picker modal, book alphabet jump, numeric keyboards |
| `versions_ui.go` | Header translation picker; `switchVersionInteractive` (sync swap or spinner-gated fetch) |

### Native reading overlays (the reading view)

| File | Tag | Responsibility |
| --- | --- | --- |
| `reading_macos.go` | `darwin && !ios` | cgo: native `NSTextView` overlay + scroll capture/restore |
| `reading_ios.go` | `ios` | cgo: native `UITextView` overlay, custom selection menu, scroll hooks |
| `reading_fyne.go` | `ios \|\| !darwin` | Fyne `RichText` fallback reading pane (Linux/Win/Android; also iOS-buildable) |
| `reading_scroll_fyne.go` | `!ios && !darwin` | No-op scroll capture/restore for the Fyne fallback |
| `reading_mobile.go` | `android` | Android-specific reading glue |
| `reading_ios_visibility.go` / `reading_android_visibility.go` | `ios` / `android` | overlay show/hide on lifecycle |
| `ai_menu_darwin.go` | `darwin` | The repo's only `//export` callbacks (AI-menu tap, iOS scroll-end) bridging native → Go |

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
| `share.go` | Selection-action dispatcher; "Share with citation" text (Bluebook-style quote/citation) |
| `share_image.go` | "Share as image" renderer — text-only card, dynamic gradient, serif typesetting |
| `share_preview.go` | Preview-and-regenerate sheet before sharing |
| `share_other.go` | `!darwin` no-op stubs for `nativeShareText` / `nativeShareImage` |

`CreateMainUI` exists in exactly one of `ui_desktop.go` / `ui_mobile.go` per
build — the Go build tag picks the layout with no runtime branching.

## UI architecture

The window is built once by `CreateMainUI`. On desktop the split, header, and
**sidebar are persistent**; only the reading/results pane is swapped on
navigation. `AppState` holds function hooks that the widgets install:

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
a Fyne widget, on the two Apple platforms:

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
- **Linux / Windows / Android** ([reading_fyne.go](reading_fyne.go),
  `ios || !darwin`): a Fyne `RichText` fallback in a vertical scroll. Verse
  numbers are superscript segments coloured via custom theme colour names so they
  track the active palette.

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
`rebuildWindow`. All versions share the canonical 66-book structure, so
reading / search / AI need no per-version code. See README → "Bible versions".

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

Select a passage → native "Study with AI" menu with four actions: **Ask a
question…**, **Explain**, **Analyze context**, **Analyze translation**
(constants `aiActionAsk/Explain/Context/Translation` in [ai.go](ai.go)). Plus an
AI **Find** passage search on the Search tab ([ai_search.go](ai_search.go)) and
plain keyword **Search**. The three search/AI verbs are kept distinct on purpose:
*Search* = keyword/reference lookup, *Find* = AI passage search returning verses,
*Ask* = AI narrative answer about a selection.

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
- `ai_menu_darwin.go` holds the repo's only `//export` callback
  (`bibleTextAIMenuTapped`); its cgo preamble must contain only C *declarations*
  (no definitions), as required alongside `//export`.

See README → "AI study" for exactly what is sent.

## Share

From the selection menu ([share.go](share.go), dispatched by
`dispatchSelectionAction`):

- **Share with citation** — plain text: the formatted quote + a reference line.
  Quote and citation follow **Bluebook** style (spelled-out translation, en-dash
  ranges, block-quote rule).
- **Share as image** ([share_image.go](share_image.go)) — a text-only card
  (no imagery) with a dynamic gradient treatment, serif typesetting, and a clean
  citation; preview/regenerate via [share_preview.go](share_preview.go).

Both hand off to the device's native share sheet on Apple platforms;
[share_other.go](share_other.go) provides graceful no-ops elsewhere.

## Reading-position + history persistence

[reading_state.go](reading_state.go) persists *where the reader left off* —
translation, book, chapter, the within-chapter **scroll position**, and the
recent-chapters history — as one JSON blob in `fyne.Preferences` (key
`reading.state`). Scroll is stored as a **verse anchor** (top-visible verse +
within-verse delta, with a whole-chapter `scrollFrac` fallback) so it survives
re-wrap on width / orientation / translation changes (font is fixed at 19px;
verse-number runs are the only sub-19px runs, used to locate verses in the
attributed string).

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
- **Platform split:** scroll hooks are real cgo on iOS/macOS; the Fyne
  platforms ([reading_scroll_fyne.go](reading_scroll_fyne.go), Linux/Win/Android)
  restore translation/book/chapter/history only — not the precise scroll.

## Threading

Fyne v2.7.4 provides **`fyne.Do`** for main-thread dispatch. All widget mutation
must still happen on the UI goroutine: compute off-thread if you must, but
marshal UI changes back through `fyne.Do` (the search debouncer's trailing timer,
the background-load apply tail, and the version-switch apply tail all do this).
Do not `Refresh()` or mutate widgets directly from `time.AfterFunc`/`go`
routines — that races with rendering; verify with `go test -race ./...`.

cgo / native caveat: on macOS, Fyne runs `OnStopped` **off** the main thread
during shutdown, so any cgo on the reading-state flush path must not
`dispatch_sync(main)` or the app hangs on quit.

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
fyne package -os android -appID uk.co.bibletext -src ./cmd/mobile
```

**Patched Fyne (iOS scroll-lag fix).** A one-line change to Fyne's iOS drawloop
idle timeout (100ms→2ms, `//go:build darwin && ios`) is applied **only** on the
iOS packaging path by `scripts/run-ios-sim.sh` / `scripts/run-ios-device.sh`
(which regenerate a patched Fyne into `third_party/fyne` — gitignored — and inject
a temporary `replace` for that build, restoring stock `go.mod` on exit). Do
**not** run a bare `fyne package -os ios` yourself; use the scripts. Rationale +
the patch + removal steps: [`patches/README.md`](patches/README.md). iOS device
installs additionally need Xcode code-signing (see `scripts/run-ios-device.sh`).

Fyne needs a C toolchain and the platform's graphics/dev libraries on every
target — see the [Fyne docs](https://docs.fyne.io/started/).

## Licensing of bundled data

The source code is **Apache License 2.0** ([LICENSE](LICENSE)). Bundled data and
assets keep their own licenses ([NOTICE](NOTICE)):

- Scripture: **World English Bible** and **Berean Standard Bible** — public domain.
- Cross-references: **OpenBible.info** Treasury of Scripture Knowledge — **CC BY**.
- UI font: **Atkinson Hyperlegible** (Braille Institute) — **SIL OFL 1.1**.
