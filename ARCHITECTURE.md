# Architecture

A Fyne desktop reader for the World English Bible. This document covers how the
pieces fit together; see [README.md](README.md) for features and usage.

## Data pipeline

```
first run:  bible-api.com ──fetch──▶ BibleData ──save──▶ on-disk cache (JSON)
every run:  cache ──load──▶ BibleData ──PrepareSearchIndex──▶ in-memory, ready
                  └─ on cache miss/corruption, fall back to fetch, then re-cache
```

- [fetch_bible_data.go](fetch_bible_data.go) — HTTP client for bible-api.com.
  Walks each book chapter-by-chapter and handles the messy parts:
  - `404` on chapter > 1 = normal end of book; `404` on chapter 1 = book skipped.
  - Transient failures retried with exponential backoff.
  - `429 Too Many Requests` honours `Retry-After` with extended recovery attempts.
  - Repeated chapter failures abort the book; a load that misses any book errors out.
- [cache.go](cache.go) — versioned cache with an atomic write (temp file + rename).
  Validates structure on load; a corrupt/old cache is discarded and refetched.
  Location: OS cache dir, or `BIBLETEXT_CACHE_PATH`.
- [bible.go](bible.go) — `BibleData` (`map[book]map[chapter][]Verse` + ordered
  `Books`), verse lookup, and search. `PrepareSearchIndex` precomputes lowercased
  verse text so search is allocation-light.

## Module map

The whole shared codebase is one Go package, `bibletext`. Two thin entry points
under `cmd/` consume it — one for desktop, one for the Fyne mobile target.

| File | Responsibility |
| --- | --- |
| `cmd/desktop/main.go` | Desktop entry — opens a sized window, calls `bibletext.Run()` |
| `cmd/mobile/main.go` | Mobile entry — built via `fyne package -os ios -src ./cmd/mobile`; the OS controls the window size |
| `app.go` | `LoadAndPrepareState()` + `Run()`: data load, state bootstrap, desktop window glue |
| `bible.go` | Data model, search ranking, reference parsing, book aliases |
| `cache.go` | Versioned, atomic, validated on-disk cache |
| `fetch_bible_data.go` | API client with retry / backoff / rate-limit handling |
| `annotation.go` | Verse-anchored annotation store (foundation for notes/highlights) |
| `theme.go` | `palette`, light/dark `bibleTheme`, custom colour names, `surface` helper |
| `font.go` | OS-serif loading (Georgia / DejaVuSerif); returns `nil` on iOS — theme falls back to Fyne's bundled font |
| `state.go` | `AppState`, navigation/search/history logic (no widget code) |
| `ui.go` | Shared header + theme toggle (used by both desktop and mobile layouts) |
| `ui_desktop.go` | `//go:build !ios && !android` — `CreateMainUI` (HSplit + sidebar) and keyboard shortcuts |
| `ui_mobile.go` | `//go:build ios || android` — `CreateMainUI` (bottom tabs: Read / Books / Search), 44pt touch rows |
| `sidebar.go` | Desktop sidebar: search, book filter, book list |
| `reading.go` | Reading view: flowing column, verse highlight, chapter picker, copy |
| `search.go` | Search results view + match-term highlighting |
| `history.go` | Slim recent-history bar |

`CreateMainUI` exists in exactly one of `ui_desktop.go` / `ui_mobile.go` per
build — selected by Go build tag — so each platform gets the right layout
without runtime branching, and pulls in only the drivers it actually uses
(`driver/desktop` for shortcuts on desktop; nothing extra on mobile).

## UI architecture

The window is built once by `CreateMainUI`. The split, header, and **sidebar are
persistent**; only the reading pane's content is swapped on navigation. `AppState`
holds function hooks the widgets install:

- `showReading()` — rebuild only the right-hand reading/results pane.
- `syncSidebar()` — re-highlight the current book in the list (no entry rebuilds).
- `refresh()` — both of the above; the usual call after a navigation.
- `focusSearch()` / `setSearchText()` — used by keyboard shortcuts.

This is why typing in the book filter never loses focus: the filter only refreshes
the list data, it does not rebuild the sidebar (the original bug). Toggling light/
dark mode is the one full rebuild — `palette`-coloured canvas objects are recreated
via `window.SetContent(CreateMainUI(...))`.

## Reading view

Scripture is grouped into paragraphs and rendered as wrapping `widget.RichText`
blocks inside a vertical scroll. `readingLayout` (a custom `fyne.Layout`) stacks
the paragraphs, centres them, and caps the line length for comfortable reading.
Verse numbers are superscript segments coloured via custom theme colour names
(`colorNameVerseNumber`, etc.), so they track the active palette.

When a search result is opened, its verse gets a faint highlight wash and bold
accent text, and `readingLayout` scrolls it into view by setting the scroll offset
**during layout** — on the render goroutine — so there is no background goroutine
and no data race.

## Search

`SearchSmartLimited` powers everything ([bible.go](bible.go)):

- A reference like `John 3:16`, `Ps 23`, or `1 Cor 13` is parsed via
  `parseReferenceQuery` + `resolveBookName` (exact name, alias table, or unique
  prefix). An exact verse reference jumps to that verse in context (on Enter).
- Otherwise it ranks verses by term/reference matches, capped at 120 results.
- Live, as-you-type search (`searchResultsOnly`) lists matches without navigating;
  matched terms are emphasised in the results.

## Threading

Fyne v2.4.1 has no main-thread dispatch primitive, so **all widget mutation must
happen on the UI goroutine** (button callbacks, `OnChanged`, and `Layout` all run
there). Do not call `Refresh()` or mutate widgets from `time.AfterFunc`/`go`
routines — that races with rendering. Compute off-thread if you must, but apply UI
changes on the UI goroutine, and verify with `go test -race ./...`.

The widget tests that use `fyne.io/fyne/v2/test` are tagged `//go:build !race`
because Fyne's *test app* clears its font cache on a background goroutine when
settings change, which the detector flags against text measurement. That race is
in the test harness, not the app.

## Extending

- **Different translation / source:** swap the fetcher passed to `loadBibleData`
  in `main.go` (anything returning a populated `*BibleData`). Bump
  `cacheSchemaVersion` in `cache.go` so old caches are discarded.
- **Note:** `PopulateWithSampleVerses` is demo/fixture data for tests only; the
  shipped app always loads the full WEB text from cache or the API.

## Cross-platform builds

Desktop targets compile from `./cmd/desktop` (Fyne pulls in OpenGL/GLFW):

```bash
GOOS=linux   GOARCH=amd64 go build -o bibletext-linux ./cmd/desktop
GOOS=windows GOARCH=amd64 go build -o bibletext.exe   ./cmd/desktop
GOOS=darwin  GOARCH=arm64 go build -o bibletext-macos ./cmd/desktop
```

Mobile targets are packaged by the `fyne` CLI from `./cmd/mobile`. This sets up
the right CGO toolchain (iOS SDK or Android NDK) and assembles a `.app`/`.ipa`
or `.apk`/`.aab` together with `FyneApp.toml` and `Icon.png`:

```bash
fyne package -os ios       -appID com.willow.bibletext -src ./cmd/mobile
fyne package -os android   -appID com.willow.bibletext -src ./cmd/mobile
```

Fyne needs a C toolchain and the platform's graphics/dev libraries on every
target — see the [Fyne docs](https://docs.fyne.io/started/).
