# Mac App Store distribution

The direct download and the Mac App Store are two channels for the same app,
and they are **two different apps to macOS**. This document records what that
means, what the Store build needs, and the one decision that cannot be undone
after the first submission.

Nothing here is wired into CI. `scripts/release-mac-store.sh` is run by hand.
The plain desktop release — unsigned, unsandboxed, one zip per architecture —
is unchanged and remains available beside the Store build.

**Status: shipped.** The first Mac submission — 1.2.4, desktop build 44,
uploaded 28 August 2026 — is `READY_FOR_SALE`; the second, 1.2.5 (desktop
build 46), was submitted 3 September 2026 with the platform's first What's
New. The listing carries both platforms and the
"one decision that cannot be undone" below is spent: the bundle id, the minimum
macOS version and the sandbox posture recorded here are what the Store holds.
Everything below is therefore a record of how it was done and what constrains
the next submission, not a plan.

## Checking which platforms are live

The public `itunes.apple.com/lookup` CANNOT answer this. Universal Purchase puts
both platforms behind one product id and the lookup returns a single shared
entity — the same `version` and `minimumOsVersion` for `entity=software` and
`entity=macSoftware` alike — so it reads as an iOS-only answer whatever the Mac
state is. Ask App Store Connect, which reports the platforms separately:

```bash
. scripts/asc-env.sh
python3 -c 'import sys; sys.path.insert(0,"build/appstore"); import asc
print(asc.request("GET","/v1/apps/6784567351/appStoreVersions"
      "?limit=20&fields[appStoreVersions]=versionString,platform,appStoreState")[1])'
```

`scripts/asc-env.sh` resolves the issuer id, the key id and the path to the
signing key from the login Keychain (service `uk.co.bibletext.appstoreconnect`,
accounts `issuer-id`, `key-id`, `key-path`) and exports the three variables the
tools read. It refuses if any is missing, if the key is unreadable, or if the
key is not mode 600 or 400.

None of those three values is in this repository, and the key filename is not
either — it contains the key id, which is why `check-repository-hygiene.py`
fails the build on it. That check caught the first draft of this very section.

## One listing, two platforms

The macOS app shares the iOS app's bundle id, `uk.co.bibletext`, so App Store
Connect treats it as the same product on a second platform rather than a
separate app. The existing listing, its reviews and its ratings carry the Mac
version, the page reads "Also available on Mac", and Universal Purchase can
apply — which is the discovery argument that justified the Store over plain
notarization in the first place.

That id is now the desktop id on every platform, not just macOS: Linux and
Windows packaging used the `.desktop` suffix purely as a desktop marker, and
keeping two ids would have meant the Mac build alone diverging from its
siblings. The registered bundle id is already UNIVERSAL, which is the type
Universal Purchase requires.

The consequence worth knowing: the direct-download macOS build now carries the
same id as the Store build, so macOS treats them as one app and the Store
version replaces a direct download rather than sitting beside it. Preferences
survive that either way, because Fyne keys them by the identifier passed to
NewWithID rather than by the bundle id.

## What the Store requires that the direct download does not

| | Direct download | Mac App Store |
| --- | --- | --- |
| Signing | none (readers right-click → Open) | 3rd Party Mac Developer Application + Installer |
| Sandbox | no | **mandatory** |
| Architecture | one zip each for Intel and Apple Silicon | one universal app |
| Container | reads `~/Library/Preferences`, `~/Library/Caches` | its own container, redirected |
| Artifact | `.zip` of the `.app` | signed `.pkg` |
| Review | none | Apple review |

## The entitlements

`appstore/mac/BibleText.entitlements` — two keys. Network access is the one
that matters: without it the app still launches, but only the embedded Gospels
seed is reachable, so it looks broken rather than restricted.

The file records what is deliberately absent and why. In short: nothing
listens, so no server entitlement; there are no file dialogs anywhere in the
app; the desktop build uses no Keychain at all; and no temporary exception is
requested, because the migration below needs none.

`scripts/check-mac-store-config.py` holds that file to the code and fails CI if
a required key is dropped or an unaudited capability appears.

## The migration, and the decision that cannot be undone

**A Store build is a separate app.** It installs alongside the direct download,
gets its own container at
`~/Library/Containers/uk.co.bibletext/Data`, and both can sit in
`/Applications` and run at once.

Under the sandbox, `HOME` points at the container. Go's `os.UserHomeDir()`
follows it, Fyne builds its preferences path from that, and the app writes a
new, empty preferences file. Without a migration an updating reader opens a
first-run app: no notes, no reading position, no settings, no saved API keys.
Nothing is destroyed — the old file is still on disk, just on the other side
of the container boundary — but the app cannot tell. It reads an empty store
and takes the branch for a genuinely new reader, and the deliberate-wipe
sentinel cannot fire, because it lived in the file that is now unreachable.

Notes are the part that matters: they are irreplaceable, and they include
notes other people shared, which exist nowhere else. Caches are not migrated
on purpose — they re-download with progress already shown, and the seed keeps
the reader in scripture meanwhile.

`appstore/mac/Container-Migration.plist` handles this. The schema is the
operation as the top-level key holding an array of paths, with `${Library}`
rather than `${HOME}` — the shape Safari and OneDrive ship. A manifest in any
other shape is ignored **in silence**, which looks exactly like a reader
having no data. Two things are easy to get wrong:

- **The path is keyed `bibletext`, not the bundle id.** `app.go` calls
  `NewWithID("bibletext")`, and Fyne's identifier wins over the `FyneApp.toml`
  metadata id when it builds
  `$HOME/Library/Preferences/fyne/<id>`. A manifest written against
  `uk.co.bibletext` migrates **nothing, silently**. The checker reads
  the identifier out of `app.go` and fails if the manifest disagrees.
- **Move, not Copy — measured, not chosen.** A `Copy` entry is ignored: a
  sandboxed build launched with one created its container and wrote a fresh
  empty preferences file while the source sat untouched beside it. `Move`
  carried all eleven keys, including the notes store. Neither Safari's nor
  OneDrive's manifest uses `Copy`. The cost is real: `Move` takes the data, so
  a direct-download build still installed alongside opens as a new reader
  afterwards.

> **macOS consults the manifest only on the launch that creates the
> container.** A reader who opens an un-migrated Store build once can never be
> migrated afterwards — the container already exists. This has to be right in
> the **first** submitted build; it cannot be fixed in 1.0.1.

## What the migration does not solve

Because the migration Moves rather than copies, a direct-download build still
installed alongside opens as a new reader once the Store build has run. If
both are then used, they diverge with no reconciliation. The options are to accept it and say so plainly in the Store
description and release notes, or to build an export/import path through the
existing share machinery so a reader can move notes deliberately. Nothing in
the app currently reconciles them.

## The minimum macOS version

The floor is **macOS 12 Monterey**, declared once as `macMinimumOSVersion` in
`config/product.json`. Until August 2026 the packaged app shipped whatever it
inherited, and the two inherited values contradicted each other:

- the packager's Info.plist template hardcodes `LSMinimumSystemVersion`
  **10.11** — a 2015 OS nobody here ever chose, tested, or could support (the
  sandbox behaviours, the container-migration manifest, and the native
  overlay's AppKit selectors — `application:openURLs:` needs 10.13 — all
  postdate it);
- the binary itself was linked with **no** `-mmacosx-version-min`, so the
  external linker stamped the build machine's SDK version as its `minos`.
  Measured on an Xcode 26 Mac: `minos 26.0` on both slices — an app whose
  plist promised El Capitan and whose loader refused anything older than the
  machine that built it.

Why 12 and not 11: Go 1.24 dropped macOS 11, and its darwin binaries are
linked for 12.0 — forcing the flag lower would advertise a floor the
toolchain vendor does not support. Monterey still covers every Apple Silicon
Mac and Intel models back to ~2015, so the universal build loses nothing real.

The floor now reaches the artifact in two places, both asserted after the
build: `release-mac-store.sh` compiles both architectures with
`-mmacosx-version-min=$MAC_MIN`, rewrites `LSMinimumSystemVersion` before
signing (next to the category, for the same signature reason), and then fails
unless the signed app's plist and both slices' `minos` equal the declared
value. The direct-download job in `release.yml` does the same.
`scripts/check-min-os-versions.py` (self-testing, run by CI and by both
release scripts) holds the declaration to its lower bounds and every build
file to the declaration. To raise the floor, edit `config/product.json`;
nothing else needs touching.

## Building a submission

```bash
export BIBLETEXT_TEAM_ID=...          # your Apple Developer team
export BIBLETEXT_MAC_PROFILE=...      # Mac App Store provisioning profile
scripts/release-mac-store.sh          # → build/mac-store/BibleText.pkg
```

The script refuses to start unless both checkers pass, builds both
architectures and `lipo`s them into one universal binary, packages the app,
verifies the keyed binary and `-trimpath` before signing, embeds the profile
and the migration manifest, re-signs with the real entitlements, and **reads
the signature back** to confirm the entitlements actually landed — because the
packager writes its own single-key file and overwrites anything left in the
build directory.

Upload with Transporter, or `xcrun altool --upload-app -t macos`. The existing
`appstore/` tooling talks to the same App Store Connect service and mostly
applies, with a new platform target.

## Before the first submission

- Test a signed sandboxed build on a Mac that **already has** the direct
  download and real notes, and confirm they appear.
  A package signed for the store **cannot be installed locally** — Apple's
  installer reports success and writes nothing, so the `.pkg` is an upload
  artifact only. `scripts/run-mac-sandbox-test.sh` is therefore the way to
  rehearse, and `--ship` does it under the real bundle id: it builds a sandboxed,
  development-signed copy under a throwaway bundle id, so the migration can be
  exercised without spending the shipping id's single chance. Back up
  `~/Library/Preferences/fyne/bibletext` first — a successful Move consumes it.
- Click every external link in the signed build — the two "Get a key" links,
  the privacy link, the site link, the Report button. They open through
  `NSWorkspace` (`external_link_darwin.go`) rather than a subprocess precisely
  so the sandbox does not refuse them, but a reviewer will click them and a
  failure is silent by nature.
- Settle whether the bundled API.Bible key ships in a Store build. It is
  extractable from any released binary by design, and the Store raises the
  stakes; see `docs/API_KEY_HANDLING.md`.
