# Contributing to BibleText

Thanks for your interest! BibleText is a cross-platform Bible reader in one Go +
[Fyne](https://fyne.io/) codebase. Contributions of all sizes are welcome.

## Getting started

You need [Go](https://go.dev/dl/) 1.24 or newer.

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

## Editor setup

Nothing here is required — the repo builds and tests from the command line
alone — but the configuration is committed so you do not have to work it out.

The root `.editorconfig` carries the indentation and whitespace conventions in
the format most editors read, so Vim, Zed, GoLand and Sublime agree with VS
Code without further setup.

For VS Code, open the folder (or `bibletext.code-workspace`, which is
equivalent) and accept the recommended extensions. You then get:

- **Run and Debug** — the desktop reader, the `bibletextdev` build, the Linux
  and Windows mimics, and the iOS Simulator, each with and without delve.
- **Tasks** — build, the race-detector test run, the view-test mutation gate,
  packaging, the Android APK and emulator, and logcat. `Cmd+Shift+B` is the
  compile check; Run Test Task is `go test -race ./...`.
- Format and organise-imports on save via gopls, matching the `gofmt`
  convention above.

Two things worth knowing. Files behind another platform's build tag appear
greyed out, which is expected and explained under *Platform build tags* below.
And `build/` and `third_party/` are excluded from search and the file watcher:
together they are the large majority of the files in a working tree, all of it
generated or vendored, and watching them makes the editor stutter whenever a
release script rewrites them.

## Before opening a pull request

- Format changed Go files with `gofmt -w <files>`, then run `go vet ./...`.
- Keep the suite green: `go test ./...` (and `go test -race ./...`)
- One logical change per commit, with a clear message. CI runs the above on every push.

## Platform build tags (the non-obvious bit)

The UI is selected at compile time by build tags, so some files look "greyed out" in an
editor that only analyses the host platform — that's expected:

- `!ios && !android` → desktop UI; `ios || android` → mobile UI
- `darwin && !ios` → native macOS code (the NSTextView reading overlay)
- `ios || !darwin` → the Fyne reading pane (Linux/Windows, plus the mobile fallback)

Validate mobile-tagged code with `scripts/run-ios-sim.sh` rather than a bare
`fyne package` command; the wrapper applies the required Fyne patches and native
bridge setup. Before that, `scripts/check-ios-pane.sh` compiles the iOS pane in
about nine seconds without packaging or signing anything — `go build ./...`
never sees that file, so a typo in its Objective-C otherwise survives a full
green test run and is not found until a three-minute package fails. Tests that exercise the Fyne reading widget skip on macOS (which uses the native
overlay) and run on Linux/Windows.

## Scope & data

- The full scripture text and the Treasury-of-Scripture-Knowledge cross-references
  are fetched at runtime and cached, not bundled. Two datasets *are* embedded in
  the binary: a World English Bible Gospels seed (`assets/seed/`, so a first run
  opens instantly to readable scripture while the full canon downloads) and the
  Gospel-parallels synopsis (`assets/parallels/`). See [NOTICE](NOTICE) for licenses.
- AI study is bring-your-own-key. **Never commit API keys.** Store-release
  API.Bible credentials are supplied only through the external release-key flow
  documented in `docs/API_KEY_HANDLING.md`; they do not belong in source files.

## Forking and deploying your own build

The product's identity lives in one tracked file, `config/product.json` —
name, site origin, bundle identifiers, support mailbox, audio host, source
repository, and the release-key secret's name. Everything in Go derives from
it and refuses to build if it is malformed. To ship your own deployment:

1. Edit `config/product.json` with your values.
2. Mirror the bundle ids, name, and website into `cmd/mobile/FyneApp.toml`
   and `cmd/desktop/FyneApp.toml`, and the fallback bundle id in
   `ai_secure_store_darwin.go` — external tools read these, so they cannot
   derive from JSON; `scripts/check-product-identity.py` fails CI until they
   all agree.
3. Regenerate `docs/apple-app-site-association` and `docs/assetlinks.json`
   with **your** Apple team id and Android signing certificate — those are
   publisher records, not product identity.
4. Point your DNS at your Pages deployment; the publisher stamps the domain
   from its `DOMAIN` variable, which the checker also holds to the identity
   file.
5. Supply your own `BIBLETEXT_BUNDLED_KEY_ENC`-style Actions secret, or ship
   keyless — the NKJV then simply requires readers to bring their own
   API.Bible key, and every public-domain translation is unaffected.
6. Replace the store metadata under `appstore/` before submitting anywhere.

## License

By contributing, you agree your contributions are licensed under the project's
[Apache License 2.0](LICENSE).
