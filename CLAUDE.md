# Holy Bible — project guide for AI assistants

A cross-platform Bible reader (macOS / Windows / Linux / iOS) from one Go
codebase, built with [Fyne](https://fyne.io/). Module name: `holybible`.

## Layout

- Repo root = the shared **`holybible` library package** (all the `*.go` files).
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
cd cmd/desktop && fyne package -os darwin       --app-id com.willow.holybibledesktop
cd cmd/mobile  && fyne package -os iossimulator --app-id com.willow.holybiblemobile
```

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
- **Native → Go bridge.** `ai_menu_darwin.go` has the repo's only `//export`
  callback (`holyBibleAIMenuTapped`); its cgo preamble must stay empty of C
  *definitions* (only declarations allowed alongside `//export`).
- **AI study (BYOK).** Select text → native "Study with AI" menu → Explain /
  Analyze context / Analyze translation. Providers (Gemini / OpenAI / Anthropic
  / Grok) live in `ai_providers.go`; keys are stored on-device via `keyStore`
  (`ai_keystore.go`) over `fyne.Preferences`, with `<PROVIDER>_API_KEY` env vars
  overriding. The exact prompt sent per action is built by `buildAIPrompt` in
  `ai.go` (shared preamble + per-action task + the quoted selection; documented
  in README → "AI study"). Settings sheet: `ai_settings.go` (header gear). Result
  panel: `ai_panel.go`.
- **Bible versions (translations).** `versions.go` defines `BibleVersion` +
  registry (WEB public-domain, NRSV/LSB licensed) and a `bibleSource` per version
  (`webSource` = bible-api.com; `licensedAPISource` = scaffold gated on a license
  opt-in + `BIBLE_API_KEY`). Unlicensed versions serve a clearly-labeled testing
  placeholder mirroring WEB's structure (`makePlaceholderBible`). `switchVersion`
  swaps `AppState.Bible` and `rebuildWindow`s; per-version cache is
  `holy-bible-<id>.json`. UI: the header subtitle is the picker (`versions_ui.go`,
  shared → both platforms). All versions share the canonical 66-book structure,
  so reading/search/AI need no per-version code. Docs: README → "Bible versions".

## Conventions

- Always `gofmt -w .` and `go vet ./...`; keep `go test -race ./...` green.
- Fyne mobile-driver hit-testing needs solid widget bounds (use `GridWrap`
  sizing), not a bare `canvas.Text` renderer.
- Wrap modal content the chapter-picker way (`widget.NewModalPopUp` +
  `surface(...)`), and remember the overlay hide/restore dance above.
