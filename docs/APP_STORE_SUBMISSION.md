# App Store submission checklist — BibleText

This is the current operational checklist. Historical release mistakes are
summarised at the end so they remain useful without being mistaken for live App
Store Connect state.

## Current release state — verify before acting

As observed against App Store Connect on 6 September 2026, with the 1.2.7
preparation recorded on 8 September 2026:

- **Live App Store version (iOS):** 1.2.5, READY_FOR_SALE since its 2 September
  version record; 1.2.4 before it.
- **Live Mac App Store version:** 1.2.5, READY_FOR_SALE; 1.2.4 was the
  platform's first release and took no What's New, 1.2.5 is its second and does.
- **Prepared next version:** 1.2.7. Both ledgers now read 1.2.7 — mobile build
  177, desktop build 48 — and both review-notes files and a What's New file on
  each platform path describe this release rather than the last (v1.2.6 is a
  source-only tag with no ledger of its own — see docs/VERSIONING.md).
- **Submission state:** nothing is in review on either Apple platform. 1.2.5 was
  submitted on 3 September 2026 (iOS build 176, Mac desktop build 46) and both
  platforms cleared; the annotated tag v1.2.5 sits at the release commit every
  channel built from. Google Play is a separate channel and IS in review: 1.2.7
  / versionCode 177 was sent on 8 September 2026, the app's first publication
  anywhere on Android (see docs/PLAY_LISTING.md). The Apple 1.2.7 submissions
  carry phone landscape reading, the desktop full-screen row, the
  selection-under-wash fix, the macOS restore and note-placement fixes, the
  publishers' own paragraphing, the Junicode reading face, chapter-bottom
  footnotes, and the NKJV Psalm titles.
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

Before producing a binary, verify that `cmd/mobile/FyneApp.toml` names the
version being prepared and a build number nothing has been uploaded under. It
now holds the numbers 1.2.7 is prepared as:

```toml
Version = "1.2.7"
Build = 177
```

Build 177 has been uploaded to Google Play; it has NOT been uploaded to App
Store Connect, so it remains the correct iOS build number until an Apple upload
succeeds under it. The desktop ledger is at build 48, above the 47 that reached
the wild. A release after this one moves both ledgers again, along with both
review-notes files and a What's New file named for the new version.
`scripts/check-release-identity.py` holds the two ledgers to one version and to
`appstore/review-notes.txt` — and to the tag, when the release workflow passes
one; the macOS notes and the version-named What's New file are held by
`appstore_review_notes_test.go`.

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
preparation. The final output must identify the version and build the ledger
declares.

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
Git. For the version being prepared the English (UK) set must include:

- the current public description naming WEB, WEB Catholic, BSB, NKJV, shared
  notes, narration, and optional bring-your-own-key AI study;
- a `whats-new-<version>.txt` describing this release (`whats-new-1.2.5.txt`
  is the newest one written);
- current name, subtitle, keywords, promotional text, support URL, marketing
  URL, and privacy URL.

The helper is read-only by default:

```bash
. scripts/asc-env.sh
python3 appstore/push-metadata.py
```

It validates every local input before making any request, resolves exactly one
record for the ledger's version and `en-GB` localization, and prints the
proposed differences. It does not PATCH without both `--write` and an exact
version confirmation. A network-free local validation is also available:

```bash
python3 appstore/push-metadata.py --local-only
```

After reviewing the remote preview, an authorized operator may repeat the
command with `--write --confirm-version <the version being written>`. The
helper reads every written field back and fails on a mismatch. A metadata write
neither selects a build nor submits a version.
`build/appstore/push_metadata.py` is retained only as a local compatibility
entry point for the tracked helper.

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
for the Mac. Both must name the version their platform's ledger declares —
1.2.5 in both files today — and each is held to its own platform's
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
`--write --confirm-version <that version>`; the helper reads the field back and
fails on a mismatch.

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

The live listing is 1.2.5, and its images are still the eight iPhone 6.9-inch
and eight iPad 13-inch captures made for 1.2.2: the complete 1.2.3 replacement
set was prepared and never uploaded, so App Store Connect carried the older
images forward with each release since. That set is prepared locally at:

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

Before a human submits a version:

1. Run `appstore/preflight.py` for every platform being submitted (the
   default run covers iOS only; add `--platform MAC_OS` for the Mac) and
   resolve every warning.
2. Confirm the version and build being submitted against that platform's
   ledger — 1.2.5 shipped as iOS build 176 and Mac desktop build 46, and the
   next Mac upload starts at desktop build 48 — and the intended release mode.
3. Read back description, What's New, review notes, URLs, copyright, privacy
   answers, age rating, and screenshot order from App Store Connect.
4. Inspect the selected build and archive evidence.
5. Confirm no previous version is blocking review.
6. Submit only through an explicitly authorized App Store Connect action.

None of the repository helpers submit a version implicitly. The one that
submits at all, `appstore/submit-version.py`, does so only behind `--write`,
an exact `--confirm-version`, and a separate `--submit` flag — and only after
it has attached the build (checked against altool's delivery UUID, which IS
the build's App Store Connect id), run both metadata writers, and had
`preflight.py` report every per-release field written. It refuses to create a
review submission otherwise. That order is the lesson of 1.2.8's Mac
submission: App Store Connect accepted the submission record and then refused
the item naming the version because its What's New was empty, leaving an
empty submission behind. Screenshots are the one field a release may knowingly
inherit; say so with `--accept-inherited-screenshots`.

    . scripts/asc-env.sh
    python3 appstore/submit-version.py --platform IOS --build 178 \
        --delivery-uuid <from altool> --write --confirm-version 1.2.8 --submit
    python3 appstore/submit-version.py --platform MAC_OS --build 49 \
        --delivery-uuid <from altool> --write --confirm-version 1.2.8 --submit

## The whole release, every channel, in order

This is the order that keeps one version naming one tree everywhere
(docs/VERSIONING.md) and that the 1.2.8 release settled on after paying for
each step it lists.

1. Prepare on main: bump `cmd/mobile/FyneApp.toml` and `cmd/desktop/FyneApp.toml`,
   the `VERSION` lines of both review-notes files, the writer's pin in
   `appstore/push-review-notes.py`; write `build/appstore/metadata/en-GB/whats-new-<v>.txt`
   AND `…/en-GB/mac/whats-new-<v>.txt` (the Mac has its own — the write refuses
   without it); add the Play notes section to `docs/PLAY_LISTING.md`.
   `scripts/check-release-identity.py` and the review-notes tests must pass.
2. Push main; wait for CI on all three OSes. Nothing is uploaded before it is green.
3. Build the three store artifacts from that commit, ONE AT A TIME —
   `build-android.sh` swaps `go.mod` and `FyneApp.toml` under a trap, so a
   concurrent build reads a modified tree:
   `scripts/release-ios.sh` (no `BIBLETEXT_UPLOAD`; it leaves `build/BibleText.ipa`),
   `scripts/release-mac-store.sh` (defaults the team id and the Mac App Store
   profile at `~/.private_keys/mac-distribution/`; set `BIBLETEXT_TEAM_ID` /
   `BIBLETEXT_MAC_PROFILE` only to override), `scripts/build-android.sh --release`.
   Read each artifact back: version, build, minimum OS, and for Android the
   manifest through `bundletool dump manifest`.
4. Upload: `xcrun altool --upload-app -t ios|macos` with the ASC key (set
   `API_PRIVATE_KEYS_DIR` to the key's directory); keep each Delivery UUID.
   Play: `scripts/play-publish.py --dry-run --notes <file> upload <aab> alpha`
   first (uploads into a discarded edit), then the same without `--dry-run`.
   The notes file is the blockquote of that version's section in
   `docs/PLAY_LISTING.md`, under 500 characters.
5. Wait for each Apple build to reach VALID, then `submit-version.py` per
   platform as above.
6. Tag LAST: an annotated `v<version>` at the build commit, pushed; the release
   workflow builds the desktop assets into a DRAFT. Upload the sideload APK
   from `~/Library/Android/bibletext-dist`, compare its SHA after download,
   then `gh release edit v<version> --draft=false`, and verify every
   `/releases/latest/download/<asset>` link resolves to the new version.
7. Work merged after the tag ships under the next number.

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

So a platform's first release has no `mac/whats-new-<version>.txt`, and
`push-metadata.py` takes `--first-on-platform` to say so. Every release after
it REQUIRES the file: App Store Connect refuses the review submission for a
version whose What's New is empty (`ENTITY_ERROR.ATTRIBUTE.REQUIRED` on
`whatsNew`), and the helper used to announce the absent file as a note and
write nothing — which is how 1.2.8's Mac submission was refused once. The
write now fails without the file, and `TestWhatsNewIsNamedForThisRelease`
fails on a machine that has the metadata directory.
