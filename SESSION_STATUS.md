# Holy Bible — Session Status (2026-05-25)

This document captures the state of the project at the end of the iOS
porting + selection session, so you can pick up where we left off.

## Overall shape

**Single Go codebase that builds for desktop (macOS / Linux / Windows) and
iOS.** Same `holybible` package; build tags pick the right UI per platform.

```
holy-bible/
├── go.mod                # module holybible, Fyne v2.7.4
├── go.sum
├── ARCHITECTURE.md
├── README.md
├── SESSION_STATUS.md     ← this file
├── *.go (data + shared UI; package holybible)
│   ├── bible.go cache.go fetch_bible_data.go annotation.go  (pure data)
│   ├── state.go theme.go font.go                            (shared scaffolding)
│   ├── sidebar.go reading.go search.go history.go ui.go     (shared widgets)
│   ├── app.go                                               (Run + LoadAndPrepareState)
│   ├── ui_desktop.go            //go:build !ios && !android
│   ├── ui_mobile.go             //go:build ios || android
│   ├── reading_mobile.go        //go:build android
│   ├── reading_ios.go           //go:build ios   — NEW: CGO+UITextView overlay
│   ├── reading_ios_visibility.go        //go:build ios
│   └── reading_android_visibility.go    //go:build android
├── cmd/
│   ├── desktop/{main.go, FyneApp.toml, Icon.png, Holy Bible.app}
│   └── mobile/{main.go, FyneApp.toml, Icon.png, Holy-Bible.app}
└── scripts/
    ├── run-ios-sim.sh
    └── install-fake-dev-cert.sh
```

## What's confirmed working

### Desktop (macOS)
- ✅ Builds cleanly: `go build -o holy-bible ./cmd/desktop`
- ✅ Also packages as a `.app` bundle via `fyne package -os darwin` in `cmd/desktop/`
- ✅ All non-Fyne-harness tests pass: `go test ./...` (one in-memory render test
  panics inside Fyne 2.7.4 — see "Known Issues" below)
- ✅ Behaviour matches the original: sidebar + reading pane, `Cmd+F`, `Esc`,
  light/dark, chapter copy, history bar, selectable verses

### iOS Simulator
- ✅ Builds cleanly: `cd cmd/mobile && fyne package -os iossimulator --app-id com.willow.holybible`
- ✅ Simulator runtime installed (iOS 26.5)
- ✅ Self-signed "Apple Development" cert in keychain (via
  `scripts/install-fake-dev-cert.sh`) — Fyne's iOS packager needs ANY such cert
  to extract a team-ID, then it ad-hoc resigns the simulator build at the end
- ✅ App container is pre-staged with the WEB Bible cache so first launch is
  instant
- ✅ Bottom tabs (Read / Books / Search) all work and switch correctly
- ✅ Books tab: filterable book list, tapping a book switches to Read with
  that book + first chapter
- ✅ Search tab: live results as you type, substring highlighting, tapping a
  result auto-switches to Read tab with the verse highlighted ("Back to
  results" returns you)
- ✅ Verses render with inline paragraph flow + superscript verse numbers
  (¹²³⁴⁵⁶⁷⁸⁹⁰) — matches desktop visual

### Helpers / docs
- ✅ `scripts/run-ios-sim.sh` — full pipeline (build + boot sim + install + launch)
- ✅ `scripts/install-fake-dev-cert.sh` — recreates the keychain workaround
- ✅ README + ARCHITECTURE updated with iOS build steps
- ✅ `.gitignore` covers `.app`, `.ipa`, `.apk`, per-cmd build outputs

## ✅ Native UITextView selection — installed and confirmed rendering

**Verified live at the end of session:** the UITextView overlay attaches to the
Fyne UIWindow, parses the chapter HTML into a 5254-char NSAttributedString,
positions itself at the reading area's frame (e.g. `(14, 175) 374×500` on the
iPhone 15 sim), and renders the verses with the native iOS text engine — which
means **drag-select, magnifier, system Copy/Look Up/Share menu, character-level
selection across paragraphs all work natively** because they're built into
UITextView. The Read tab content is now genuinely "like an email."

Log lines confirming the wire-up (filtered from iOS syslog with
`xcrun simctl spawn UDID log stream --predicate 'eventMessage CONTAINS "holybible:"'`):

```
holybible: ensureTV — created UITextView, attaching to window <UIWindow…>
holybible: HTML set (7183 bytes input → 5254 attr chars)
holybible: setFrame (14.0,174.6) 374.0x500.9 hidden=0 superview=<UIWindow…>
```

### Resolved this session

- **Verse numbers render fine.** The `<sup class="v">` superscript numbers show
  as small accent-coloured raised digits — the earlier "invisible" note was a
  downscaled-screenshot artifact; they were always there.
- **Header-overlap bug FIXED.** The UITextView was floating ~59pt too high,
  overlapping the "Chapter N of M" header. Root cause: Fyne renders its canvas
  inset below the device safe area (Dynamic Island), so a Fyne coordinate Y
  maps to window Y + safeAreaInsets.top, but the UITextView is a raw window
  subview. Fix is in `holyBibleTVSetFrame` (reading_ios.go) — it now adds
  `superview.safeAreaInsets` (.top/.left) to the frame origin. Verified: text
  now starts cleanly below the header and ends above the tab bar.

### Full feature test — all passing (verified on iOS 26.5 sim)

Ran an interactive pass before committing:
- Chapter next/prev arrows ✓ (John 1→2)
- Chapter picker grid ✓ — **this surfaced a z-order bug** (the picker rendered
  *behind* the UITextView and stole its taps). Fixed: showChapterPicker now
  hides the native overlay while open via `state.hideReadingOverlay` /
  `showReadingOverlay` (no-op on desktop/Android, wired to the CGO show/hide on
  iOS). Re-tested: picker renders cleanly and selecting a chapter works.
- Native text selection ✓ — drag across verses/paragraphs, native handles,
  system menu Copy / Look Up / Translate / Share. The headline feature.
- Books tab + book navigation ✓ (→ Genesis); overlay correctly hidden on Books.
- Search + live results + match highlighting ✓ ("shepherd").
- Jump-to-verse from a search result ✓ (→ Genesis 46) + "Back to results".
- Recent history bar ✓.
- Dark/Light toggle ✓ — UITextView re-renders with the dark palette (light text
  on dark paper, gold verse numbers).

### Minor cosmetic nits (non-blocking, noted for next pass)

1. **Overlay scroll position on chapter change.** Switching chapters in place
   sometimes leaves the UITextView scrolled a little below verse 1 (a full
   rebuild — e.g. dark-mode toggle — scrolls to top correctly). The
   `contentOffset = CGPointZero` reset in `holyBibleTVSetHTML` races the frame
   set; reset it again right after `holyBibleTVSetFrame`, or after a tick.
2. **No scroll-to-matched-verse.** Tapping a search result loads the right
   chapter but doesn't scroll the UITextView to the matched verse (desktop's
   chapterText did this via highlightLine). Add a `scrollRangeToVisible:` call
   keyed off the highlighted verse's character range.

### Remaining polish on the iOS reading view

1. **Selection persistence across chapter changes** — the UITextView is reset
   when chapter changes; previous selection is lost. Probably fine, but if you
   want to preserve it, save `gReadingTV.selectedRange` before setText and
   restore after.
3. **OnSelected didn't fire for initial Read tab** — the first build had the
   UITextView starting hidden because of this. Worked around by making the new
   UITextView start *visible*. Still consider firing OnSelected explicitly on
   AppTabs initial selection in `ui_mobile.go` if Fyne ever changes this.

### Old code path (now legacy — but kept for ref)

The original UITextView code is intact and was the version that
was put on the simulator at the end of the session. The previous attempt was a
"long-press → context menu" approach in `reading_mobile.go` (now Android-only).

### Native UITextView overlay for text selection (`reading_ios.go`) — original notes
The big change at the end of this session — adds **real iOS-native text
selection** (drag handles, magnifier, system Copy/Look Up/Share menu,
character-level, cross-paragraph). Architecture:

- CGO Objective-C creates a `UITextView(isEditable=NO, isSelectable=YES)`
- Attached as a sibling subview of Fyne's UIWindow, on top of the GL canvas
- Fyne reserves space with `nativeReadingHost`, a transparent widget that
  pushes its absolute screen rect to the UITextView frame on every Resize/Move
- Chapter is rendered as HTML → NSAttributedString (superscript verse numbers,
  themed colours), text background transparent so Fyne's parchment shows through
- `tabs.OnSelected` shows/hides the overlay so it doesn't float over Books/Search

**Status:** compiles cleanly (`go build ./cmd/desktop` ✅, `fyne package -os iossimulator` ✅),
but the app on the simulator right now is still the previous build (long-press
verse menu). The new UITextView build was packaged at 10:52 but install/launch
was blocked when the Anthropic safety classifier went down mid-session.

**To resume:**

```bash
UDID=$(cat /tmp/holybible-sim-udid)
xcrun simctl install "$UDID" /Users/willow/Dev/holy-bible/cmd/mobile/Holy-Bible.app
xcrun simctl terminate "$UDID" com.willow.holybible
xcrun simctl launch --console "$UDID" com.willow.holybible
```

then in the simulator:
- long-press a verse → confirm the iOS selection magnifier + Copy/Look Up menu appears
- drag selection across multiple verses + paragraphs
- confirm Books/Search tabs hide the UITextView (no leaking over)
- confirm Read tab restores it
- toggle Dark mode and confirm text colors update (HTML is regenerated with
  the new palette every time `pushChapterHTML` runs)

### Known things to verify on first run
1. UITextView actually finds the window via `UIApplication.sharedApplication.connectedScenes`
   (scene-based; iOS 13+). If not, falls back to deprecated `keyWindow`.
2. AbsolutePositionForObject() returns UIKit-compatible logical points. Should
   be 1:1 on iOS; if the overlay shows up at the wrong position, that's where
   to look.
3. HTML → NSAttributedString respects our CSS font-size/color attributes.
   If styling looks broken, the fallback path sets `gReadingTV.text = ...`
   (plain text) — at least selection still works.
4. Light/dark theme toggle: rebuilds the reading view, which re-runs
   `pushChapterHTML` → new HTML with new colors. Should "just work."

## 📋 Outstanding follow-ups (not blocking)

### Small / quick

1. **Fyne 2.7.4 test regression**
   `TestBookFilterKeepsFocusWhileTyping` panics in `internal/painter/font.go:142`
   inside Fyne's in-memory test driver. Real app is unaffected. Either skip the
   test on 2.7+, patch around the painter race, or pin to Fyne 2.6.x. Not yet done.

2. **"Back to results" tab indicator inconsistency (mobile)**
   When the user taps "Back to results" from a verse-highlight Read view,
   `IsSearching` flips back to true and the read pane shows search results,
   but the bottom-tab indicator stays on Read. Either route that button to
   also call `tabs.SelectIndex(2)` or rewrite "Back to results" to send the user
   back to the Search tab (which still has the field + results).

3. **Placeholder Icon.png**
   Solid parchment 1024×1024. Replace with real artwork before any App Store
   submission. Lives at `cmd/mobile/Icon.png` and `cmd/desktop/Icon.png`.

### Medium

4. **First-launch watchdog risk on real device**
   `LoadAndPrepareState()` blocks `main()` while the WEB Bible is fetched on
   first run (~5–10s on Wi-Fi). iOS's launch watchdog can kill an app that
   doesn't draw a frame within ~20s. On the simulator this is fine because
   `--console-pty` mode bypasses the watchdog, but on a real device a slow
   first launch could be killed. Fix: open the window with a "Loading…" view
   first, then fetch on a background goroutine that swaps the loaded
   `BibleData` in once ready.

5. **Android fallback for selection**
   `reading_mobile.go` (now Android-only) renders verses with `widget.RichText`
   and provides a long-press → context menu (Copy verse / Copy with reference /
   Look up / Share). That works but isn't as native-feeling as iOS. If you
   ship Android, consider a similar `EditText` overlay (Android equivalent of
   the UITextView trick).

### Bigger

6. **Selection state across navigation on iOS**
   The UITextView is recreated each chapter change (we set new attributedText).
   Selection is lost on chapter change. To preserve it across e.g. a tap on
   the "next chapter" button would need a selection-state save/restore.

7. **Native Share sheet**
   The Android fallback uses `mailto:` to share. On iOS with the UITextView,
   the system context menu already includes Share, which launches
   UIActivityViewController natively — no extra work needed once selection is
   confirmed working.

8. **Native dictionary lookup**
   The "Look Up" item in the UITextView context menu uses iOS's built-in
   UIReferenceLibraryViewController — also free as part of UITextView's
   selection menu. The Android fallback opens Safari with a Google search
   instead.

## Quick verification commands (when classifier is back)

```bash
# Confirm desktop still builds + most tests pass
cd /Users/willow/Dev/holy-bible
go build -o /tmp/holy-bible ./cmd/desktop
go test ./...   # one Fyne 2.7.4 painter panic is expected — see #1 above

# Confirm iOS sim build is current
export PATH="$(go env GOPATH)/bin:$PATH"
cd cmd/mobile
fyne package -os iossimulator --app-id com.willow.holybible

# Install + launch the latest build
UDID=$(cat /tmp/holybible-sim-udid)
xcrun simctl install "$UDID" Holy-Bible.app
xcrun simctl terminate "$UDID" com.willow.holybible
xcrun simctl launch --console "$UDID" com.willow.holybible
```

The simulator UDID `cat /tmp/holybible-sim-udid` was saved earlier this session;
the device is named `HolyBibleTestSim` and runs iOS 26.5.

## Where to look in the code

| Question | File |
| --- | --- |
| iOS UITextView CGO bridge | [reading_ios.go](reading_ios.go) |
| Mobile bottom-tabs layout | [ui_mobile.go](ui_mobile.go) |
| Desktop sidebar + HSplit | [ui_desktop.go](ui_desktop.go) |
| Tab show/hide hook (build-tag shim) | [reading_ios_visibility.go](reading_ios_visibility.go), [reading_android_visibility.go](reading_android_visibility.go) |
| Android fallback (long-press menu) | [reading_mobile.go](reading_mobile.go) |
| Shared state + the `surfaceReading` hook | [state.go](state.go) |
| Bible data, search, cache, fetch | [bible.go](bible.go), [cache.go](cache.go), [fetch_bible_data.go](fetch_bible_data.go) |
| iOS / Android packaging | [cmd/mobile/](cmd/mobile/) |
| Desktop packaging | [cmd/desktop/](cmd/desktop/) |
| Reproducible setup scripts | [scripts/](scripts/) |
