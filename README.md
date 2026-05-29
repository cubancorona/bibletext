# Holy Bible Study

A clean, modern reader for the Bible that runs on **macOS, Windows, Linux, and
iOS** from a single Go codebase, built with [Fyne](https://fyne.io/). It
presents the **World English Bible (WEB)**, a public domain translation, in a
calm, responsive reading layout.

## Features

- 📖 **Responsive reading** — scripture flows as a centred column that wraps to
  the window width with a comfortable line length, and superscript verse numbers.
- 🔍 **Smart search** — keyword search across every verse with the matched terms
  highlighted, plus reference lookups like `John 3:16`, `Ps 23`, or `1 Cor 13`
  (common abbreviations are understood). An exact verse reference jumps straight
  to that verse in context.
- 🧭 **Quick navigation** — filterable book list, previous/next chapter, and a
  chapter picker grid.
- 🕮 **Recent history** — a slim, unobtrusive bar of recently read chapters you
  can jump back to, or clear.
- 🌗 **Light & dark mode** — a warm "paper" light theme or an easy-on-the-eyes
  dark theme.
- 📋 **Copy** — copy the current chapter to the clipboard.
- ⌨️ **Keyboard shortcuts** (desktop) — `Cmd/Ctrl+F` focuses search, `Esc` clears.
- 📱 **Touch UI** (iOS) — bottom-tab layout (Read / Books / Search) with full-size
  touch targets; the same data, search and theme code as the desktop build.

## Repository layout

```
holy-bible/
├── go.mod                  # module holybible
├── *.go                    # shared package: holybible
│   ├── bible.go cache.go fetch_bible_data.go annotation.go   (pure data layer)
│   ├── state.go theme.go font.go                              (cross-platform UI scaffolding)
│   ├── sidebar.go reading.go search.go history.go ui.go       (shared widgets)
│   ├── ui_desktop.go    # //go:build !ios && !android  — HSplit + keyboard shortcuts
│   ├── ui_mobile.go     # //go:build ios  || android   — bottom tabs + touch drawer
│   └── app.go              # Run() + LoadAndPrepareState() shared entry helpers
└── cmd/
    ├── desktop/main.go     # `go build ./cmd/desktop`
    └── mobile/                # `cd cmd/mobile && fyne package -os ios`
        ├── main.go
        ├── FyneApp.toml      # bundle ID, version (read by `fyne package`)
        └── Icon.png          # 1024×1024 app icon — replace before App Store
```

The same `holybible` package is consumed by both `cmd/` entry points; build tags
on `ui_desktop.go` / `ui_mobile.go` make the linker pick the platform-appropriate
`CreateMainUI` implementation. Pure data files (`bible.go`, `cache.go`,
`fetch_bible_data.go`, `annotation.go`) have no UI deps and compile everywhere.

## Requirements

- Go 1.21 or newer
- Fyne v2.4.1 and its [system dependencies](https://docs.fyne.io/started/)
- For iOS packaging: macOS, **Xcode** (full install, not just Command Line
  Tools), and an Apple Developer account for signing
- For Android packaging: the Android SDK + NDK

## Build & run

### Desktop (macOS / Windows / Linux)

```bash
go mod download
go build -o holy-bible ./cmd/desktop
./holy-bible
```

Cross-compile for other desktop OSes:

```bash
GOOS=linux   GOARCH=amd64 go build -o holy-bible-linux  ./cmd/desktop
GOOS=windows GOARCH=amd64 go build -o holy-bible.exe    ./cmd/desktop
GOOS=darwin  GOARCH=arm64 go build -o holy-bible-macos  ./cmd/desktop
```

### iOS

```bash
# one-time setup
go install fyne.io/tools/cmd/fyne@latest    # NB: the new tools repo, not the
                                            # deprecated fyne.io/fyne/v2/cmd/fyne
# install Xcode (the full app from the App Store, not just CLT) and download an
# iOS simulator runtime once: `xcodebuild -downloadPlatform iOS` (several GB)

# Before the first iOS build, sign into Xcode with your Apple ID
# (Xcode → Settings → Accounts → +) so an "Apple Development" certificate is
# created in your keychain. Fyne uses it to extract a team ID and to satisfy
# xcodebuild; the simulator build is ad-hoc re-signed at the end so no paid
# Apple Developer Program membership is needed for local testing.

# Build & run on the iOS simulator (`-src` must point to the directory with
# main.go + FyneApp.toml + Icon.png — i.e. ./cmd/mobile, or just cd into it):
cd cmd/mobile
fyne package -os iossimulator --app-id com.willow.holybible

# Boot a simulator and install:
xcrun simctl boot "iPhone 15" 2>/dev/null   # or any simulator name from `simctl list devices`
open -a Simulator
xcrun simctl install booted "Holy Bible.app"
xcrun simctl launch booted com.willow.holybible

# Build a signed .ipa for a real device (paid Developer Program required):
fyne package -os ios --app-id com.willow.holybible \
             --certificate "Apple Development: Your Name (TEAMID)" \
             --profile "Your Provisioning Profile Name"
```

> **Icon.** The bundled `Icon.png` is a placeholder (a solid parchment colour).
> Replace it with a real 1024×1024 PNG before submitting to the App Store.

### Android

```bash
fyne package -os android -appID com.willow.holybible -src ./cmd/mobile
```

On first launch the app downloads the World English Bible from
[bible-api.com](https://bible-api.com/) (about 30–60 seconds) and saves a local
cache in the OS cache directory (on iOS, inside the app's container). Every
later launch loads instantly from cache and works offline. Set
`HOLY_BIBLE_CACHE_PATH` to override the cache location on desktop.

## Tests

```bash
go test ./...           # everything, including in-memory UI render tests
go test -race ./...     # logic tests (UI-render tests are skipped — see below)
```

The widget tests that use `fyne.io/fyne/v2/test` are excluded under `-race`
because Fyne's test app clears its font cache on a background goroutine, which
the race detector flags against text measurement. That race is in Fyne's test
harness, not in this app or the real renderer.

## License

Application code is provided for educational and devotional use. The bundled
scripture text is the **World English Bible**, which is in the public domain.

---

> "Your word is a lamp to my feet and a light to my path." — Psalm 119:105
