# Contributing to BibleText

Thanks for your interest! BibleText is a cross-platform Bible reader in one Go +
[Fyne](https://fyne.io/) codebase. Contributions of all sizes are welcome.

## Getting started

You need [Go](https://go.dev/dl/) 1.24 or newer.

```bash
git clone https://github.com/cubancorona/bibletext.git
cd bibletext
go run ./cmd/bibletext        # launch the desktop reader
go test ./...               # run the test suite
```

On Linux, the Fyne GUI needs OpenGL/X11 headers to build:

```bash
sudo apt-get install gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev libasound2-dev
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
bridge setup. Before that, `scripts/check-ios-pane.sh` compiles the iOS pane
in well under a minute without packaging or signing anything — `go build ./...`
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
   and `cmd/bibletext/FyneApp.toml`, and the fallback bundle id in
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

## Adding a download or a store

Every artifact a reader can get is named by filename on the download page at
`docs/index.html`, which no generator writes. A new release asset is therefore
invisible until someone links it — which is how the AppImage shipped in 1.2.10
with nothing pointing at it. The README's Download section names some of the
same files and sends the rest to the Releases page, so it is held to linking
nothing that does not exist rather than to naming everything.

`scripts/check-public-surfaces.py` (self-testing, run by CI) closes the
mechanical half: it reads the asset names out of the `gh release upload`
steps in `.github/workflows/release.yml` and fails if the download page does
not offer one of them, or if either page links a name no release uploads. An
asset that is deliberately not offered — the AppImage's `.zsync` sidecar, which
update tools fetch by themselves — is listed in the checker's `NOT_LINKED`
map with the reason, so the omission is a reviewable decision rather than a
gap. The same checker holds the Linux build dependencies identical across the
README, this file and CI, and holds the README's count of `cmd/` programs to
what is actually in `cmd/`. It also holds the four explanations of opening the
unsigned Mac download — the release notes in `release.yml`, the download page,
the README and `docs/MAC_APP_STORE.md` — to the route macOS 15 and later
require, Open Anyway in Privacy & Security, and, while `macMinimumOSVersion`
in `config/product.json` is below 15, to Control-click → Open for the older
releases, and it fails on the right-click → Open advice macOS 15 retired. The
floor is read rather than assumed, so raising it to 15 drops the Control-click
requirement without an edit to the checker.

A store is different from a download and needs its own step. The checker holds
any Microsoft Store link on either page equal to the `storeUrl` in
`msstore/identity.json`, so a typed or moved id fails — but it cannot know
that a listing went live and ought to be linked at all. That belongs to the
release sequence in `docs/APP_STORE_SUBMISSION.md`, which now carries it.

What none of it can do is notice that a true sentence has become a stale one.
When a release changes what a platform can do, read the prose on both pages.

## License

By contributing, you agree your contributions are licensed under the project's
[Apache License 2.0](LICENSE).
