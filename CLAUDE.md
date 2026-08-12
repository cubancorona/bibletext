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
one-line with no setup step. FOUR in-Fyne fixes (see `patches/`, all applied
unconditionally by `setup-fyne-patch.sh`): the Noto Color Emoji font swap
(Fyne's bundled set is from 2016 and lacks anything newer than Emoji 11), the
Android `onNewIntent` forward (without it warm shared links are dropped — the
Android build hard-fails if this patch is missing), the `drawloop`
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

## Website (bibletext.co.uk) — landing pages + the web reader

The landing/download page, privacy policy, and support page live in `docs/`
on main (`index.html`, `privacy.html`, `support.html`) — the SOURCE OF TRUTH —
and the **web reader** (`/web/…`, `/bsb/…`, `/webc/…` — the destination of the app's "Share as link")
is generated by `cmd/websitegen`. GitHub Pages serves the **`gh-pages` branch
root**, so editing `docs/` on main does not change the live site.

**Publish with `scripts/publish-site.sh` — never by hand.** It is now the only
publisher, and it writes the WHOLE tree (landing pages + reader) in one go. That
matters: a hand-copy of the three pages would delete the reader, and a
reader-only publish would delete the pages, because each writes a tree lacking
the other's files. `--dry-run` builds and verifies without pushing. Its guards
abort the publish if `CNAME` (the custom domain — losing it detaches
bibletext.co.uk and kills every shared verse link), `.nojekyll`, any of the
three root pages, or a plausible number of chapter pages is missing.

DNS/registrar is Cloudflare (4×A + 4×AAAA GitHub Pages records, www CNAME).
`release.yml`'s asset names (BibleText-macOS-AppleSilicon.zip etc.) are a stable
contract with the page's download links — never rename them; the Android APK
(`BibleText-Android.apk`) is uploaded to the release manually.

### The web reader (`cmd/websitegen`)

A third entry point beside `cmd/desktop` and `cmd/mobile`, importing the SAME
package: it renders chapters through the app's own poem-line rule
(`PoeticJoin`), red-letter data and decoders, so the page and the app can never
drift. Pure stdlib `html/template`, no npm, no framework. ~3,900 static files,
~34 MB, built in seconds; translation JSON is cached under `build/` (gitignored)
so rebuilds are offline.

**Pre-rendering is not a preference — it is the requirement.** Link unfurlers
(iMessage/WhatsApp/Slack) do not run JavaScript, so per-chapter Open Graph
previews only exist because every chapter is its own file; an SPA would also
404 on every deep link, since Pages has no rewrites.

**The URL contract is frozen** (`bookslugs.go`, `share_link.go`):
`https://bibletext.co.uk/<version>/<book-slug>/<chapter>/#v<lo>[-<hi>]`.
Shared links live forever in message threads, so: slugs are append-only and
IMMUTABLE (a display-name change must never move a slug — `share_link_test.go`
holds a golden of all 73 and fails loudly), the trailing slash is part of the
contract, the verse rides in the FRAGMENT (so a renumbered verse degrades to
"chapter opens, no scroll" instead of a dead link, and single verses highlight
with CSS `:target` and zero JS), and only the public-domain version ids ever
appear — a licensed id falls back to `web`. The reader sits at the ROOT (no
`/read/` prefix) to keep shared links short, which means it SHARES the root
namespace with the hand-written pages: the three version ids and `/assets/` are
reserved there forever, and `cmd/websitegen` refuses to write `index.html`,
`privacy.html`, `support.html` or `CNAME` (reservedRootNames) so a generator
change can never clobber the live landing or App Store pages.

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
  use per-key presses. **To screenshot a SHEET in the simulator:** `simctl` can
  boot/install/launch/screenshot but cannot TAP anything, and clicking the
  Simulator window from outside needs a desktop-automation tool plus macOS
  accessibility, which may not be available. Dev builds therefore open a sheet
  themselves — `BIBLETEXT_DEV_OPEN=settings|goto|versions|votd`
  (`dev_autoopen_on.go`, behind the `bibletextdev` tag like the Links tab) — and
  `simctl` forwards it under the `SIMCTL_CHILD_` prefix:
  `SIMCTL_CHILD_BIBLETEXT_DEV_OPEN=settings xcrun simctl launch <udid> uk.co.bibletext`
  then `xcrun simctl io <udid> screenshot shot.png`. Release builds ship universal (`release-ios.sh`) — see
  [`docs/IPAD.md`](docs/IPAD.md). iPads also typeset the reading pane to the
  U.S. Reports geometry (centred 27.5em native-inset column, 1.3 leading,
  indented paragraphs — `reporterLayoutActive()`, `reporterMeasureEm`; phones
  unchanged); details in docs/IPAD.md → "The reading page".
- **Desktop reads as the reporter page too (owner directive, task #17).**
  macOS: `reporterLayoutActive()` is now TRUE on darwin (`reporter_macos.go`),
  so `buildChapterHTML` emits the U.S. Reports set and
  `bibleTextMacSetReadingMeasure` (reading_macos.go) centres the 27.5em column
  via `textContainerInset` — the NSTextView twin of the iPad's
  `bibleTextSetReadingMeasure`, re-applied on every pane resize. Windows/Linux:
  the styled pane does it WIDTH-GATED in `relayout` (reading_styled_pane.go) —
  a pane wider than `reporterMeasureEm × textSize` centres the measure
  (`extraInset`, read through `insetX()` by draw AND selection so glyphs and
  hit-tests share one ruler), sets 1.3 leading, zero paragraph gap and a
  geometric first-line indent (`styledLayoutParams.Indent`; unlike iOS's
  literal em+en spaces it never enters the selection text model, so copies
  stay clean); narrower panes keep the cozy 1.55/gap layout. Tests:
  `reading_styled_reporter_test.go`; host tests asserting PHONE HTML must pin
  the `reporterLayout` seam to false (the host is darwin → reporter default).
- **Desktop styled pane (Windows/Linux).** Since the milestone-4 swap the
  Windows/Linux reading pane is `styledReadingPane` (`reading_styled_*.go`,
  untagged so the whole engine unit-tests on the Mac): styled runs (red
  letters, raised serif verse numbers), real selection + the SHARED
  `selectionStudyMenu`, exact verse-anchor scroll persistence, and the iOS
  scripture face (system Georgia, else embedded Gelasio) drawn AND measured
  via `FontSource`/`RenderedTextSize` so glyphs and hit-tests share one
  ruler. Dispatch is `useStyledPane()` — a per-platform constant
  (`reading_styled_platform_on/off.go`); flipping the `on` constant reverts
  to the legacy `chapterText` Entry pane in one line. iOS/macOS/Android
  behaviour is untouched (false constant; macOS keeps NSTextView).
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
  theme variant, red-letter, the highlighted-verse identity AND the `state.Bible`
  pointer, or a search-jump / light-dark flip would show stale text and a
  background DATA SWAP (the Gospels-seed→full download, or the stale-epoch
  refresh) would be skipped entirely, leaving the old decode on screen until the
  next navigation; (3) live search is debounced via
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
  `keyStore` (`ai_keystore.go`) in the Apple Keychain on **iOS only** — macOS release
  builds are ad-hoc signed, so a keychain ACL breaks on every update — with one-time
  migration from the legacy `fyne.Preferences` value, and preferences everywhere else
  (macOS included); `<PROVIDER>_API_KEY` env
  vars overriding (Grok's is `XAI_API_KEY`, not GROK_). Per-action prompts are built by `buildAIPrompt` / `buildAskPrompt` in
  `ai.go` (shared preamble + per-action task + the quoted selection; the fixed actions
  documented in README → "AI study"). `runAIAction` threads the Ask question and folds
  it into the cache scope. Settings sheet: `ai_settings.go` (header gear). Result panel:
  `ai_panel.go`.
- **Bible versions (translations).** `versions.go` defines `BibleVersion` +
  registry (WEB + WEB-Catholic + BSB public-domain; NRSV/LSB/NKJV licensed) and a `bibleSource` per
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
  `bibletext-<id>.json` plus a `-v<epoch>` suffix once a decoder change bumps
  `cacheEpoch` — true of all three shipping translations today (WEB v2, BSB v3,
  WEBC v2), so even the default WEB version has left the legacy path. A bumped
  epoch never strands a reader: `loadVersionFromCacheOnly` and the restore/switch
  paths fall back through `supersededCachePaths` (so an OFFLINE upgrade still
  opens on the previous decode) and the old file is purged only AFTER a
  successful refetch. Superseded per-version cache was
  `bibletext-<id>.json`. UI: the header subtitle is the picker (`versions_ui.go`,
  shared → both platforms). Most versions are the canonical 66-book Protestant canon;
  the **World English Bible (Catholic)** (`webCatholicSource`, `catholic.go`) adds the
  73-book deuterocanon — decoded by USFM **id** (helloao appends the deuterocanon and
  gives the Greek Esther/Daniel, so the order-based `decodeBSBComplete` can't be reused)
  and emitted in traditional Catholic order. Reading/search/AI/navigation are data-driven
  off `BibleData.Books`, so they need no per-version code; 66-book-only features
  (cross-refs, red-letter, verse-of-day) simply skip the deuterocanon. Docs: README →
  "Bible versions".

- **Poetry renders as authored lines — FIVE surfaces in lockstep.** Verse text
  carries `"\n"` poem-line breaks (decoder: `bsbVerseText` in `bsb.go`; all
  helloao translations). The display rule is `verseIsPoetic` / `poeticJoin` in
  `reading.go` — any verse join touching a poetic verse is a line boundary —
  and it is the SAME rule `chapterShareStructure` (share.go) restores with.
  Changing poetry presentation means changing ALL of: `buildChapterHTML`
  (reading.go; `<br>` + `p.pm` ragged-right + reporter-indent skip),
  `buildChapterHTMLAndroid` (`android_chapter_html.go` — untagged so host
  tests pin both dialects), the desktop rewrap (`"\n"` sentinel tokens in
  `verseTokens`), the Android Fyne fallback (`reading_mobile.go`), and
  `verse_of_day.go`; the ObjC `bibleTextPlainFromHTML` fallbacks map
  `<br>`→`\n`. Tests: `reading_poetry_test.go`. A one-line poem verse has no
  internal break and reads as prose (known limitation, shared with the share
  pipeline).
- **Simulator Keychain needs a Mach-O entitlements SECTION, not a signature.**
  A simulator app gets entitlements from `__TEXT,__entitlements` in the binary;
  signing one with `codesign --entitlements` makes AMFI refuse to exec it
  ("adhoc signed app with restricted entitlements", launch dies with POSIX
  163). `scripts/run-ios-sim.sh` therefore relinks the executable after
  `fyne package` with
  `-Wl,-sectcreate,__TEXT,__entitlements,<plist>` (application-identifier +
  keychain-access-groups, both `uk.co.bibletext`) and then signs plain ad-hoc.
  Without it `SecItemAdd` returns -34018 and `keyStore` silently falls back to
  Preferences, so the whole Keychain path is untestable. **That string is the
  keychain partition — changing it orphans every key saved in a simulator.**
  `scripts/verify-sim-keychain.sh` asserts the round-trip and that the item is
  `AfterFirstUnlock` (`pdmn='ck'`), never a ThisDeviceOnly class that backups
  and device migration exclude. **The simulator still cannot prove:**
  locked-device protection classes, backup/restore, lock-screen Now Playing +
  remote commands, AVAudioSession interruptions, screen-off background audio,
  the launch watchdog, jetsam, iOS Library/Caches eviction, data protection,
  the silent switch, or anything below the installed runtime (only iOS 26.5 is
  installed here, so the pre-iOS-17 audio branch has never run) — those need a
  real device.
- **Two reading headers — edit BOTH.** The reading toolbar is built per platform:
  desktop uses `chapterHeader` (`reading.go`, via `buildReadingView`), while
  **both phones use `chapterHeaderMobile`** (`chapter_header_mobile.go`, tagged
  `ios || android`, via each platform's `buildReadingViewMobile`). Android reaches
  `chapterHeader` only through the bridge-absent Fyne fallback. A header control
  (e.g. the audio play button) must be added to *both* or it won't appear on the
  phones — `reading.go` alone is not the
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
  (collapsed it is a single speaker glyph; expanded, play/pause is always
  MediaPlay/MediaPause — `iconAudioWave` in `icons_embed.go` is the SOURCE
  selector's glyph, person = recording vs waveform = read-aloud, not a play
  button), shown only where `chapterAudioAvailable()` (= an engine exists AND this
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
  `run-ios-sim.sh`, **before** their codesign step (the same block also injects
  `NSPhotoLibraryAddUsageDescription` — without it iOS silently HIDES the share
  sheet's "Save Image" action for the verse-image cards). Audio auto-stops on any
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
- `rebuildWindow` (`reading.go`) **drains `Canvas().Overlays()` before
  `SetContent`**: `SetContent` only reassigns the content tree, so an open sheet
  would otherwise survive a rebuild with its build-time palette colours (the
  field-reported half-dark Settings sheet after an overnight light/dark flip).
  It `Hide()`s popups rather than bare-removing them, because the popup watchdog
  timers poll `Visible()` and a bare remove leaves them polling forever, and it
  stops descendant `ProgressBarInfinite`s so a hidden spinner stops repainting.
- A popup's close-out work that runs on a timer must check that no OTHER overlay
  now owns the canvas before restoring the reading overlay, and skip its own
  "changed while open" rebuild when `windowRebuildGen` moved (a rebuild already
  rebuilt from live prefs).
