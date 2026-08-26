# App Store submission checklist — BibleText

This is the current operational checklist. Historical release mistakes are
summarised at the end so they remain useful without being mistaken for live App
Store Connect state.

## Current release state — verify before acting

As publicly observed on 24 August 2026:

- **Live App Store version:** 1.2.2, released 23 August 2026.
- **Prepared next version:** 1.2.3, mobile build 174.
- **Submission state:** 1.2.3 is prepared only. It has not been submitted by
  this checklist or by the metadata helpers.
- **Bundle ID:** `uk.co.bibletext`; universal iPhone and iPad; minimum iOS 13.

Public lookup data is cached, and App Store Connect is authoritative. Start every
release with the read-only preflight:

```bash
ASC_KEY_PATH=/path/AuthKey.p8 \
ASC_KEY_ID=... \
ASC_ISSUER_ID=... \
python3 appstore/preflight.py
```

The preflight performs GET requests only. It prints every version's live state,
compares release-specific fields with the previous version, and identifies
copy-forward metadata. Do not infer submission eligibility from this document.

## Release identity and local build

Before producing the 1.2.3 binary, verify that `cmd/mobile/FyneApp.toml` reads:

```toml
Version = "1.2.3"
Build = 174
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
preparation. The final output must identify version 1.2.3, build 174.

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
Git. For 1.2.3 the English (UK) set must include:

- the current public description naming WEB, WEB Catholic, BSB, NKJV, shared
  notes, narration, and optional bring-your-own-key AI study;
- `whats-new-1.2.3.txt` describing this release;
- current name, subtitle, keywords, promotional text, support URL, marketing
  URL, and privacy URL.

The helper is read-only by default:

```bash
ASC_KEY_PATH=/path/AuthKey.p8 \
ASC_KEY_ID=... \
ASC_ISSUER_ID=... \
python3 appstore/push-metadata.py
```

It validates every local input before making any request, resolves exactly one
1.2.3 record and `en-GB` localization, and prints the proposed differences. It
does not PATCH without both `--write` and an exact version confirmation. A
network-free local validation is also available:

```bash
python3 appstore/push-metadata.py --local-only
```

After reviewing the remote preview, an authorized operator may repeat the
command with `--write --confirm-version 1.2.3`. The helper reads every written
field back and fails on a mismatch. A metadata write neither selects a build nor
submits a version. `build/appstore/push_metadata.py` is retained only as a local
compatibility entry point for the tracked helper.

## Review notes

`appstore/review-notes.txt` is the tracked source of truth and must name 1.2.3.
Validate the local source without contacting App Store Connect:

```bash
python3 appstore/push-review-notes.py --local-only
```

Then preview the current App Store Connect value:

```bash
ASC_KEY_PATH=/path/AuthKey.p8 \
ASC_KEY_ID=... \
ASC_ISSUER_ID=... \
python3 appstore/push-review-notes.py
```

Only an authorized operator should repeat the command with
`--write --confirm-version 1.2.3`; the helper reads the field back and fails on
a mismatch.
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
text and inspect every final image with OCR. Upload iPhone and iPad sets together,
set the source directory explicitly, verify dimensions/order in App Store
Connect, and run `appstore/preflight.py` again. Do not use an older
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

- **The reader's own data was not photographed.** The rehearsal container holds
  whatever the container migration carried in, which is real notes and a real
  AI key. Both were replaced for the session with the same neutral, clearly
  synthetic note text already published in the iPhone and iPad listing, and put
  back afterwards.
- **`screencapture` always writes an alpha channel and App Store Connect
  refuses a PNG that carries one.** The `-ready` copies are redrawn opaque;
  check with `sips -g hasAlpha` before uploading rather than after.

The dark image needs the *system* appearance switched to Dark, not just
`FYNE_THEME=dark`: the app follows the OS, but the window's title bar follows
it too, and a dark app under a light title bar is a combination no reader can
actually produce.

The macOS description and promotional text are drafted at
`build/appstore/metadata/en-GB/mac/`. They are not the iOS text: the AI study
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

Before a human submits 1.2.3:

1. Run `appstore/preflight.py` and resolve every warning.
2. Confirm version 1.2.3/build 174 and the intended release mode.
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
