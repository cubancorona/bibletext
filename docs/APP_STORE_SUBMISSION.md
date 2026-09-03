# App Store submission checklist — BibleText

This is the current operational checklist. Historical release mistakes are
summarised at the end so they remain useful without being mistaken for live App
Store Connect state.

## Current release state — verify before acting

As observed against App Store Connect on 3 September 2026:

- **Live App Store version (iOS):** 1.2.4, build 175, READY_FOR_SALE.
- **Live Mac App Store version:** 1.2.4, desktop build 44, READY_FOR_SALE — the
  platform's first release, so it took no What's New; 1.2.5 is its second and
  does.
- **Prepared next version:** 1.2.5 — mobile build 176 (iOS build and Android
  versionCode; 175 is spent), desktop build 46 (the ledger moved 44→46 inside
  two intermediate commits; both numbers are valid for the Store's next upload).
- **Submission state:** both platforms' 1.2.5 were submitted on 3 September
  2026 (iOS build 176, Mac desktop build 46) and are WAITING_FOR_REVIEW; the
  annotated tag v1.2.5 sits at the release commit every channel built from.
  Note that `fyne package` bumps the desktop ledger's Build AFTER packaging
  (46 became 47 in the working tree once the Mac package existed); the
  shipped build is the committed number, so discard that bump rather than
  commit it. The same bump runs INSIDE `release.yml`, which packages the two
  direct-download architectures in sequence: the 1.2.5 Apple Silicon zip
  carries CFBundleVersion 46 and the Intel zip 47 — identical code, one tree,
  but 47 is now in the wild — so the next desktop Store upload starts at 48,
  and the workflow should reset the ledger between its two packages (see
  BACKLOG).
- **Bundle ID:** `uk.co.bibletext`; universal iPhone and iPad. Builds up to
  174 (the 1.2.3 submission) declare minimum iOS 13; the repository now
  declares iOS 15 for every FUTURE build (`iosMinimumOSVersion` in
  `config/product.json`, the one authoritative value — App Store Connect
  refuses floors below 15.0 from Spring 2027, upload warning 90068 until
  then; `check-min-os-versions.py` guards it and `release-ios.sh` reads it
  back out of the exported `.ipa`).

`scripts/asc-env.sh` resolves the issuer id, the key id and the path to the
signing key from the login Keychain and exports what these tools read. The three
values are deliberately absent from this repository, and so is the key's
filename — it contains the key id, and `check-repository-hygiene.py` fails the
build if it appears in a tracked file. Before the helper existed each release
began by hunting for all three, which is the whole reason it exists.

Public lookup data is cached, and App Store Connect is authoritative. Start every
release with the read-only preflight:

```bash
. scripts/asc-env.sh
python3 appstore/preflight.py
```

The preflight performs GET requests only. It prints every version's live state,
compares release-specific fields with the previous version, identifies
copy-forward metadata, verifies every uploaded screenshot reached
`assetDeliveryState` COMPLETE, and validates the local upload-ready screenshot
set's pixel sizes and alpha channels. Do not infer submission eligibility from
this document.

All three helpers — `preflight.py`, `push-metadata.py`, `push-review-notes.py`
— default to the iOS platform and never touch the other one. Pass
`--platform MAC_OS` to target the Mac version instead; each platform's version
string comes from its own ledger (`cmd/mobile/FyneApp.toml` for iOS,
`cmd/desktop/FyneApp.toml` for the Mac). A platform's first version has no
earlier same-platform record, so the preflight compares it against the newest
iOS versions — which is exactly what App Store Connect seeded it from.

## Release identity and local build

Before producing the 1.2.5 binary, verify that `cmd/mobile/FyneApp.toml` reads:

```toml
Version = "1.2.5"
Build = 176
```

The version/build change is a separate release step; this document does not make
it. Never rebuild changed code under an already-uploaded build number.

Run from the repository root:

```bash
go build ./...
go test ./...
go test -race ./...
go vet ./...
./scripts/release-ios.sh
```

`release-ios.sh` derives the version and build from `FyneApp.toml`, applies the
required Fyne patches, produces a universal archive, and does not upload unless
`BIBLETEXT_UPLOAD=1` is explicitly set. Leave that variable unset during
preparation. The final output must identify version 1.2.5, build 176.

Release builds intentionally contain the project's API.Bible fallback, supplied
from the dedicated external release-key source and transformed/injected at link
time. It is used only to fetch the licensed NKJV and may be overridden by a
reader's own API.Bible key. No AI-provider key is bundled. Repository files and
ordinary local environment files are not release-key fallbacks; see
`docs/API_KEY_HANDLING.md`.

The public-domain full translations are fetched and cached at runtime. The
embedded scripture content is only the small WEB Gospels first-launch seed;
Gospel-parallel data and presentation assets are also embedded. The licensed
NKJV text is fetched through API.Bible and is never packaged into the binary.

Also verify the archive contains the privacy manifest, launch screen, complete
icon catalog, `UIDeviceFamily=[1,2]`, `get-task-allow=false`, and
`ITSAppUsesNonExemptEncryption=false`.

## Metadata — preview first, write only deliberately

Editable staging files are under `build/appstore/metadata/` and are ignored by
Git. For 1.2.5 the English (UK) set must include:

- the current public description naming WEB, WEB Catholic, BSB, NKJV, shared
  notes, narration, and optional bring-your-own-key AI study;
- `whats-new-1.2.5.txt` describing this release;
- current name, subtitle, keywords, promotional text, support URL, marketing
  URL, and privacy URL.

The helper is read-only by default:

```bash
. scripts/asc-env.sh
python3 appstore/push-metadata.py
```

It validates every local input before making any request, resolves exactly one
1.2.5 record and `en-GB` localization, and prints the proposed differences. It
does not PATCH without both `--write` and an exact version confirmation. A
network-free local validation is also available:

```bash
python3 appstore/push-metadata.py --local-only
```

After reviewing the remote preview, an authorized operator may repeat the
command with `--write --confirm-version 1.2.5`. The helper reads every written
field back and fails on a mismatch. A metadata write neither selects a build nor
submits a version. `build/appstore/push_metadata.py` is retained only as a local
compatibility entry point for the tracked helper.

For the Mac version, `--platform MAC_OS` reads the Mac description and
promotional text from `build/appstore/metadata/en-GB/mac/` (deliberately not
the iOS text), falls back to the shared en-GB files for keywords and the URLs
when no Mac-specific file exists (each fallback is announced), and skips the
app-level name, subtitle, and privacy URL — those are one per app, and the
iOS run owns them. A platform's first version has no What's New field in App
Store Connect, so an absent `mac/whats-new-<version>.txt` is not an error
for that debut alone.

## Review notes

Each platform has its own review-notes field and its own tracked source of
truth: `appstore/review-notes.txt` for iOS and `appstore/review-notes-macos.txt`
for the Mac. Both must name 1.2.5, and each is held to its own platform's
FyneApp.toml by `appstore_review_notes_test.go`. Validate the local sources
without contacting App Store Connect:

```bash
python3 appstore/push-review-notes.py --local-only
```

```bash
python3 appstore/push-review-notes.py --local-only --platform MAC_OS
```

Then preview the current App Store Connect value (add `--platform MAC_OS` for
the Mac record — the default run resolves the iOS one only):

```bash
. scripts/asc-env.sh
python3 appstore/push-review-notes.py
```

Only an authorized operator should repeat the command with
`--write --confirm-version 1.2.5`; the helper reads the field back and fails on
a mismatch.

The macOS notes must additionally cover what is Mac-specific: the right-click
Study with AI gesture, the App Sandbox and the one-time container migration
(a local move performed by macOS, invisible on a machine with no prior
install), and the same compiled-API.Bible-fallback versus AI-credential
distinction the iOS notes make. The Mac record was seeded with the live iOS
notes when it was created, so until the tracked macOS file is written through
the helper, App Store Connect holds iPhone and iPad text on the Mac version.
If Apple needs to exercise optional AI features, place any temporary review-only
AI-provider credential in App Review Information, never in the repository or
review-notes file.

The notes must distinguish the intentional compiled API.Bible fallback from AI
provider credentials and accurately describe fetched versus embedded text.

## Screenshots

The live 1.2.2 listing has eight iPhone 6.9-inch images and eight iPad 13-inch
images. A complete 1.2.3 replacement set is prepared locally at:

- `build/appstore/screenshots-iphone-1.2.3/`
- `build/appstore/screenshots-1.2.3/`

Images 01–07 retain the 1.2.2 captures. Image 08 was recaptured on the current
iPhone and iPad interfaces with all and only Psalm 82:1 selected and the real
Study with AI menu showing Explain, Analyze context, and Analyze translation.
The upload-ready opaque PNG copies are under
`build/appstore/screenshots-ready-1.2.3/en-GB/` and its `ipad13/` directory.
These local assets are preparation only; an App Store upload remains a separate
explicit operation.

Before any future screenshot replacement, use neutral, clearly synthetic note
text and inspect every final image with OCR. Upload iPhone and iPad sets
together, set the source directory explicitly, verify order in App Store
Connect, and run `appstore/preflight.py` again **for the platform whose set
changed** — the default run covers iOS only, and `--platform MAC_OS` is the
only way it sees the Mac sets at all. The preflight validates the local
`-ready` set's pixel sizes and alpha channels before anything goes up, and
reads back every uploaded image's `assetDeliveryState`: a wrong-sized or
alpha-carrying upload is accepted by the API and then sits at FAILED with no
error at upload time, so COMPLETE is the only good answer. Do not use an older
`screenshots-ready-*` directory or a helper's default path.

### The macOS set

macOS is a second platform on the same app record, so it needs its own images
and its own description; nothing carries over from the iPhone and iPad sets.
Eight captures matching those sets shot for shot are prepared locally at:

- `build/appstore/screenshots-mac-1.2.3/` — as captured
- `build/appstore/screenshots-ready-1.2.3/en-GB/mac/` — opaque copies to upload

They were taken from a sandboxed, dev-signed build (`run-mac-sandbox-test.sh`)
with the window pinned to 1280×800, which is 2560×1600 on a Retina display —
one of the sizes Apple accepts. Two things about that build are worth
recording, because a repeat capture gets them wrong by default:

- **A reader's own data must never be photographed.** The rehearsal container
  holds whatever the container migration carried in — real notes, and any
  saved keys. Before capturing, replace the store with the same neutral,
  clearly synthetic note text already published in the iPhone and iPad
  listing, and restore the original afterwards, verified byte-for-byte.
- **`screencapture` always writes an alpha channel and App Store Connect
  refuses a PNG that carries one.** The `-ready` copies are redrawn opaque;
  `appstore/preflight.py --platform MAC_OS` now checks the `-ready` set's
  dimensions and alpha channels automatically (`sips -g hasAlpha` remains a
  fine manual cross-check).

The dark image needs the *system* appearance switched to Dark, not just
`FYNE_THEME=dark`: the app follows the OS, but the window's title bar follows
it too, and a dark app under a light title bar is a combination no reader can
actually produce.

The macOS description and promotional text are drafted at
`build/appstore/metadata/en-GB/mac/`; `appstore/push-metadata.py --platform
MAC_OS` previews them against the Mac record and writes them only behind
`--write --confirm-version`. They are not the iOS text: the AI study
gesture is a right-click rather than a selection popover, audio has no
lock-screen behaviour to describe, and the description states plainly that the
Store edition moves — not copies — an existing install's notes and settings
into its sandbox container.

## Privacy, age rating, and declarations

The public listing currently says **Data Not Collected**, consistent with the
empty collected-data array in `cmd/mobile/PrivacyInfo.xcprivacy` and the fact
that the developer operates no account, analytics, advertising, or application
server. Before each submission, re-evaluate that answer against Apple's current
definitions and the live terms of every supported AI provider: an optional AI
request goes directly from the reader's device to the provider selected under
the reader's own key, and that provider may associate or retain it.

Confirm the public privacy and support pages match the submitted binary and are
live before review. Keep the configured support mailbox consistent through the
project's central contact mechanism rather than copying an address into this
checklist.

Review rather than copy forward:

- the current age-rating questionnaire, including scriptural content and
  optional generated AI responses;
- content-rights declarations and the licences in `NOTICE`;
- export compliance (`ITSAppUsesNonExemptEncryption=false`, ordinary HTTPS);
- no IDFA, tracking, ads, login, subscriptions, purchases, or demo account; and
- accessibility nutrition labels. Do not claim an accessibility feature until
  common reading, navigation, search, settings, audio, and sharing tasks are
  verified on both iPhone and iPad.

## Final read-back and submission

Before a human submits 1.2.5:

1. Run `appstore/preflight.py` for every platform being submitted (the
   default run covers iOS only; add `--platform MAC_OS` for the Mac) and
   resolve every warning.
2. Confirm version 1.2.5/build 176 (iOS) or desktop build 46 (Mac) and the
   intended release mode.
3. Read back description, What's New, review notes, URLs, copyright, privacy
   answers, age rating, and screenshot order from App Store Connect.
4. Inspect the selected build and archive evidence.
5. Confirm no previous version is blocking review.
6. Submit only through an explicitly authorized App Store Connect action.

None of the repository helpers should submit a version implicitly.

## Release-specific metadata invariants

App Store Connect copies populated metadata forward, so a non-empty field is not
evidence that it belongs to the current release. Enforce these controls:

- tracked, version-checked review notes;
- a version-named What's New file;
- a read-only comparison against the previous version;
- metadata preview plus explicit `--write`;
- complete local validation before the first PATCH; and
- read-back after every write and immediately before submission.

An unchanged release-specific field requires explicit verification against the
current release.

### The first release on a platform takes no What's New

App Store Connect refuses `whatsNew` on the first public version of a
platform with `409 STATE_ERROR: Attribute 'whatsNew' cannot be edited at
this time`, because there is no earlier release for the notes to describe.
The Mac App Store hit this on 1.2.4, its first public version: the shared
`metadata/en-GB/whats-new-<version>.txt` is right for iOS, which has
shipped since 1.1.x, and wrong for a platform's debut.

So a platform's first release simply has no `mac/whats-new-<version>.txt`.
Add one for the release after it, when there is something to compare to.
