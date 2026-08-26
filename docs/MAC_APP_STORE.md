# Mac App Store distribution

The direct download and the Mac App Store are two channels for the same app,
and they are **two different apps to macOS**. This document records what that
means, what the Store build needs, and the one decision that cannot be undone
after the first submission.

Nothing here is wired into CI. `scripts/release-mac-store.sh` is run by hand.
The plain desktop release — unsigned, unsandboxed, one zip per architecture —
is unchanged and remains the default channel.

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
`~/Library/Containers/uk.co.bibletext.desktop/Data`, and both can sit in
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
  `uk.co.bibletext.desktop` migrates **nothing, silently**. The checker reads
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
  `scripts/run-mac-sandbox-test.sh` does exactly this: it builds a sandboxed,
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
