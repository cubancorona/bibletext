# Store submissions

Which store fronts BibleText has, what a release does and does not do for each,
and who can do the rest.

This page is a map, not a status board. It deliberately records **no version
numbers and no review states** — those change weekly and would go stale here
while the per-store documents stayed right. Each row points at the document
that holds the truth.

## The rule

> **A release is not a submission.**

Cutting a tag runs `.github/workflows/release.yml`, which does exactly two
kinds of thing:

- `gh release upload` — attaches the desktop artifacts to the **GitHub
  release**. That is a download, not a store.
- `snapcore/action-publish` — pushes the snap to the Snap Store's **`edge`**
  channel, for both architectures.

That is the whole of it. **No release has ever sent anything to Apple,
Microsoft or Google.** Every one of those is a separate, deliberate act by the
account holder, made from an artifact built at the tagged commit.

The snap is the single exception, and even there `edge` is not what
`snap install` gives anyone: promotion to `stable` is a separate deliberate
step too.

## The store fronts

| Store | What a release does | What a submission needs | Who | Detail |
| --- | --- | --- | --- | --- |
| **Apple App Store** (iPhone, iPad) | nothing | `scripts/release-ios.sh` builds and uploads only with `BIBLETEXT_UPLOAD=1`; then metadata, screenshots and review notes in App Store Connect | account holder | [APP_STORE_SUBMISSION.md](APP_STORE_SUBMISSION.md) |
| **Mac App Store** | nothing | `scripts/release-mac-store.sh` builds a signed, sandboxed `.pkg` against the Store certificates; uploaded and submitted separately | account holder | [MAC_APP_STORE.md](MAC_APP_STORE.md), [APP_STORE_SUBMISSION.md](APP_STORE_SUBMISSION.md) |
| **Microsoft Store** | nothing | `msstore.yml` builds and smokes the MSIX as an **artifact only**; the submission is made in Partner Center. The first submission after a name reservation cannot use the API | account holder | [WINDOWS_STORE_LISTING.md](WINDOWS_STORE_LISTING.md) |
| **Google Play** | nothing — the APK is attached to the GitHub release by hand | `scripts/build-android.sh --release` produces the signed AAB; uploaded to a track in the Play Console | account holder | [PLAY_LISTING.md](PLAY_LISTING.md) |
| **Snap Store** | **publishes both architectures to `edge`** | promotion to `stable` is one command and can be automated; categories, screenshots and visibility are console-only | mixed | [LINUX_STORES.md](LINUX_STORES.md) |
| **Flathub** | nothing | a pull request to `flathub/flathub`, which **their policy requires the account holder to write and post personally**, with an AI-assistance disclosure | account holder only | [LINUX_STORES.md](LINUX_STORES.md) |
| **AppImageHub** | nothing | a pull request to `AppImage/appimage.github.io` adding `data/BibleText` | account holder | [LINUX_STORES.md](LINUX_STORES.md) |

## What every submission has in common

**The artifact must come from the tagged commit.** A version number names one
tree on every channel ([VERSIONING.md](VERSIONING.md)). The release workflow
rebuilds the desktop assets; everything else — the AAB, the `.ipa`, the Mac
`.pkg`, the MSIX — is built locally or in a separate workflow, and each must be
built from the tag, not from a later HEAD.

**Version bumps touch more than the two packaging ledgers**, and each coupling
is enforced by a test: `cmd/desktop/FyneApp.toml`, `cmd/mobile/FyneApp.toml`,
both review-notes files' first line, `appstore/push-review-notes.py`'s
`TARGET_VERSION`, and version-named What's New files for iOS and Mac.

**Not every release is submitted anywhere.** Skipping a store for a release
that changes nothing a reader of that platform would notice is normal and
deliberate; it is recorded in that store's own document.

## Architectures

Since 1.2.12 every desktop channel ships both x86-64 and ARM64
([PLATFORM_MATRIX.md](PLATFORM_MATRIX.md)). Store submissions have not caught
up with that, and each store handles it differently:

- **Microsoft Store** — the next submission should carry **both** the x64 and
  the arm64 package. Partner Center accepts several `.msix` files in one
  submission, or they can be combined into a `.msixbundle`; today `msstore.yml`
  produces two separate `.msix` artifacts and nothing has submitted the arm64
  one. Windows on ARM runs the x64 package under emulation, so this is a
  performance and battery improvement rather than a fix.
- **Snap Store** — already carries both; a single `snapcraft promote` covers
  every architecture and refuses a partial set.
- **Flathub** — `flatpak/flathub.json` names both, and both build and smoke in
  `linux-stores.yml`.
- **Apple and Google** — a single universal artifact each, so nothing to do.
