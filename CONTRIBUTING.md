# Contributing to BibleText

Thanks for your interest! BibleText is a cross-platform Bible reader in one Go +
[Fyne](https://fyne.io/) codebase. Contributions of all sizes are welcome.

## Getting started

You need [Go](https://go.dev/dl/) 1.21 or newer.

```bash
git clone https://github.com/cubancorona/bibletext.git
cd bibletext
go run ./cmd/desktop        # launch the desktop reader
go test ./...               # run the test suite
```

On Linux, the Fyne GUI needs OpenGL/X11 headers to build:

```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev
```

## Before opening a pull request

- Format and vet: `gofmt -w . && go vet ./...`
- Keep the suite green: `go test ./...` (and `go test -race ./...`)
- One logical change per commit, with a clear message. CI runs the above on every push.

## Platform build tags (the non-obvious bit)

The UI is selected at compile time by build tags, so some files look "greyed out" in an
editor that only analyses the host platform — that's expected:

- `!ios && !android` → desktop UI; `ios || android` → mobile UI
- `darwin && !ios` → native macOS code (the NSTextView reading overlay)
- `ios || !darwin` → the Fyne reading pane (Linux/Windows, plus the mobile fallback)

Validate mobile-tagged code with `fyne package -os iossimulator` rather than the host
build. Tests that exercise the Fyne reading widget skip on macOS (which uses the native
overlay) and run on Linux/Windows.

## Scope & data

- Scripture text and cross-references are fetched at runtime and cached — not bundled
  (see [NOTICE](NOTICE) for licenses).
- AI study is bring-your-own-key. **Never commit API keys.**

## License

By contributing, you agree your contributions are licensed under the project's
[Apache License 2.0](LICENSE).
