# App Store submission checklist — BibleText 1.2.0

Prepared 1 August 2026 for 1.1.6 (build 124); refreshed 8 August for 1.1.7
(build 127); refreshed 12 August for the 1.1.8 cycle (shared notes, the NKJV via
API.Bible, the Settings redesign).

VERIFIED against App Store Connect on 12 August, not remembered: **1.1.6 and
1.1.7 are both live** (READY_FOR_SALE), the highest build ever uploaded is
**127**, and no 1.1.8 version record exists yet — one must be created before a
build can be attached.

## Release identity

| Field | Value |
| --- | --- |
| App | BibleText |
| Apple app ID | `6784567351` |
| Bundle ID | `uk.co.bibletext` |
| Version | `1.2.0` |
| Build | `161` |
| Platform | Universal iPhone and iPad |
| Minimum OS | iOS 13.0 |
| Price | Free |
| Primary category | Reference |
| Secondary category | Education |
| Copyright | `2026 Willow Noonan` |

The public App Store version at preparation time is 1.1.8 (build 161 of the 1.1.8 train, released 18 Aug 2026 — submitted overnight by an agent session from the 16 Aug tree; 1.2.0 is the quality fast-follow carrying the scrapbook store, the note selector, the positional citations and the day's fixes). Do not change the
bundle ID or remove iPad support from this update.

## Locally verified release inputs

- `cmd/mobile/FyneApp.toml` is the source of version 1.2.0/build 161.
- `scripts/release-ios.sh` derives the marketing version from that file, keeps
  exact copies of locally edited build files, builds arm64, creates an App Store
  archive, and does not upload unless `BIBLETEXT_UPLOAD=1` is explicitly set.
- `cmd/mobile/PrivacyInfo.xcprivacy` is copied to the root of the app bundle.
- `cmd/mobile/LaunchScreen.storyboard` is compiled into the bundle for both
  iPhone and iPad.
- `UIRequiresFullScreen` is removed so iPad Split View, Stage Manager, and
  resizable windows are not disabled.
- AI provider keys use the Apple Keychain on iOS (preferences elsewhere) and
  migrate from the old preferences value on first access.
- The App Store icon is an opaque 1024×1024 RGB image. `release-ios.sh`
  regenerates the whole icon asset catalog with `actool` from that single
  source, because fyne emits no iPad Pro 167×167 rendition and App Store Connect
  rejects the upload with error 90023 without it.
- The release uses the App Store distribution profile for team `R8PC7239T2`,
  has `get-task-allow=false`, and declares `ITSAppUsesNonExemptEncryption=false`.

Run the final local checks from the repository root:

```sh
go test ./...
go vet ./...
./scripts/release-ios.sh
```

The last command must finish with `build/BibleText.ipa is ready (version 1.2.0,
build 161)`. Do **not** set `BIBLETEXT_UPLOAD=1` during preparation.

Run it as `BIBLETEXT_SHORT_VERSION=1.2.0 ./scripts/release-ios.sh` — the script's
own default is "1.0", and the marketing version must match the ASC record.

## Store metadata prepared locally

The editable staging files are in `build/appstore/metadata/` (this directory is
ignored by Git). The prepared English (UK) values are:

- Name: `BibleText`
- Subtitle: `Read and study Scripture`
- Promotional text: the current private, bring-your-own-key summary
- Description: covers WEB, WEB Catholic, BSB, search, optional AI study, audio,
  iPad, offline use, no ads, and no accounts
- Keywords: 94 characters (within the 100-character limit)
- Support URL: <https://bibletext.co.uk/support.html>
- Marketing URL: <https://bibletext.co.uk/>
- Privacy URL: <https://bibletext.co.uk/privacy.html>
- What's New (1.1.8 DRAFT — needs the owner's approval before it goes in; the
  text itself is staged at `build/appstore/metadata/en-GB/whats-new-1.1.8.txt`):
  Someone can now send you a verse with a note attached. Tap their link and
  BibleText opens the passage with their message beside it, in the translation
  they were reading. Your notes are collected on the Search tab, where you can
  search them, sort them and see when each one arrived. The New King James
  Version is now available, and works as soon as you install the app. Settings
  has been rebuilt around what you actually change, and reading is tidier
  throughout: the words of Christ keep their red when a verse is highlighted,
  and controls that were hard to make out in dark mode are now clearly drawn.

`build/appstore/review_notes.txt` has current feature paths, iPad behaviour,
the optional-AI test procedure, data flow, age-rating context, and contact
details. Before submission, add a temporary review-only provider API key in App
Review Information if Apple needs to exercise AI. Never commit that key.

The metadata helper resolves version/localization IDs at runtime:

```sh
ASC_KEY_PATH=/path/AuthKey.p8 \
ASC_KEY_ID=... \
ASC_ISSUER_ID=... \
python3 build/appstore/push_metadata.py
```

It updates text only; it does not choose a build or submit for review. The old
build-87 TestFlight scripts are guarded and are not part of this release.

## Screenshots

Use the opaque JPEG exports, not the source PNGs (the PNG files have alpha).

**Current sets: `build/appstore/screenshots-ready-1.1.8/`** (captured 9 Aug
2026 — AI study, Matthew 1 reading view, Explain, dark mode, and the share
card, for both slots; full capture recipes and exact upload commands in
`build/appstore/screenshots-1.1.8/HANDOFF.md`):

- iPhone 6.9-inch slot: `build/appstore/screenshots-ready-1.1.8/en-GB/` —
  1320×2868 images
- iPad 13-inch slot: `build/appstore/screenshots-ready-1.1.8/en-GB/ipad13/` —
  2752×2064 images

The older `screenshots-ready/` directories are the 1.1.6-era sets, superseded.
Upload once the current review cycle clears (screenshots are per-device-size,
independent of app version):

```sh
ASC_SHOTS_DIR=build/appstore/screenshots-ready-1.1.8/en-GB \
python3 build/appstore/upload_screenshots.py

ASC_DISPLAY_TYPE=APP_IPAD_PRO_3GEN_129 \
ASC_SHOTS_DIR=build/appstore/screenshots-ready-1.1.8/en-GB/ipad13 \
python3 build/appstore/upload_screenshots.py
```

ALWAYS set `ASC_SHOTS_DIR` explicitly. `upload_screenshots.py` defaults to
`screenshots-ready/en-GB` — the superseded 1.1.6 set this section just told you
not to use — so the bare command uploads the wrong images. The 1.1.8 filenames
differ entirely from the 1.1.6 ones, so nothing would overwrite: you would simply
end up with both sets on the listing.

The helper resolves the current version automatically and does not submit it.
Visually inspect the resulting order and device frame in App Store Connect.

## Privacy answers to confirm in App Store Connect

Answer **Data Not Collected**. That matches the bundled privacy manifest
(`cmd/mobile/PrivacyInfo.xcprivacy` declares an empty
`NSPrivacyCollectedDataTypes`) and `docs/privacy.html`. The developer operates no
server, no accounts and no analytics, and receives none of this data.

The optional AI features are not developer collection: a Find query, or a
selected passage and study action, goes from the reader's device straight to the
AI provider they configured, under their own API key. That provider may associate
and retain the request under the reader's own account and terms — their
disclosure to their own vendor, which `docs/privacy.html` states plainly. Nothing
is routed through or retained by any BibleText server. Re-check that wording
against the supported providers' live terms before review.

Leave `NSPrivacyAccessedAPITypes` as declared: file timestamp (C617.1), system
boot time (35F9.1), and user defaults (CA92.1).

Publish the updated `docs/privacy.html` and `docs/support.html` to the live URLs
before review. This cycle they add the NKJV and API.Bible: support.html no longer
says the NKJV is unselectable, and privacy.html now lists `rest.api.bible` among
the services contacted.

Publish with **`scripts/publish-site.sh`** and nothing else. Editing `main` alone
does not update the live site, but neither should you hand-copy files onto
`gh-pages`: that script is the ONLY publisher precisely because it writes the
whole tree (landing pages AND the generated web reader) in one go — a hand-copy
of the three pages deletes the reader, and a reader-only publish deletes the
pages. It also refuses to publish if `CNAME`, `.nojekyll`, any root page, or a
plausible number of chapter pages is missing. `--dry-run` builds and verifies
without pushing.

## Age rating and compliance review

Re-open the current age-rating questionnaire even though the public version is
shown as 4+. Suggested capability answers, subject to checking the final app:

- Parental controls: No
- Age assurance: No
- Unrestricted web access: No
- User-generated content shared broadly: No
- Messaging/chat between users: No
- Advertising: No

Do not copy the old content-frequency answers blindly. Review the scriptural
descriptions of violence/mature themes and the optional AI-generated responses,
because Apple's current questionnaire applies content criteria to AI/chatbot
features. The account holder must choose the truthful frequency values and
accept any resulting rating.

Also confirm in App Store Connect:

- Content rights cover the bundled/public-domain and attributed data listed in
  `NOTICE`.
- Export compliance is answered consistently with
  `ITSAppUsesNonExemptEncryption=false` (ordinary HTTPS only).
- Regulated Medical Devices is answered No/not applicable; BibleText provides no
  medical functionality or claims.
- No IDFA, tracking, ads, login, subscriptions, in-app purchases, or demo
  account are declared.

## Accessibility Nutrition Labels

These labels are voluntary as of this checklist date, but App Store product
pages now show whether support has been indicated. Do not publish a feature
claim until all common reading, navigation, search, settings, audio, and sharing
tasks have been tested on both iPhone and iPad against Apple's criteria.

- Dark Interface is the strongest candidate: the app follows the system light
  and dark appearance throughout.
- Do not claim Larger Text yet. The in-app scripture control currently tops out
  at 130%, below Apple's 200% label criterion, and the Fyne UI does not inherit
  iOS Dynamic Type throughout.
- VoiceOver, Voice Control, Differentiate Without Color Alone, Sufficient
  Contrast, and Reduced Motion remain unverified end-to-end. Leave them
  unclaimed until a device audit proves all common tasks.
- Captions and Audio Descriptions do not apply to the Bible narration feature.

## Account-only fields and final submission order

These cannot be proved from the repository or the public product page. The
Account Holder/App Manager must confirm each item:

1. Agreements are active; tax and banking status does not block the free app.
2. Digital Services Act trader status and required contact display are complete.
3. Country-specific availability declarations (including China mainland,
   Vietnam, and South Korea if applicable to the account type) are complete.
4. Version 1.1.8 exists for iOS and is in an editable state (it does NOT yet —
   see "Gate" below; it must be created before a build can attach). While it is
   editable, also set the version-level Privacy Policy URL, Marketing URL, and
   Support URL to the bibletext.co.uk pages (standing TODO from 1.1.6).
5. English (UK) metadata, both screenshot sets, categories, price, territories,
   and platform availability are correct. Check Mac and Apple Vision Pro
   availability explicitly rather than inheriting an unintended default.
   Review optional App Tags as well, but select only tags the app genuinely
   supports.
6. App Privacy and the age-rating questionnaire match the sections above.
7. App Accessibility either remains honestly unindicated or publishes only
   claims proved by the device audit above.
8. App Review contact name, reachable email, and phone are current. Paste the
   prepared notes and, if supplied, the temporary AI review key.
9. Release mode (manual, automatic, or phased) and the intended release date are
   selected deliberately.
10. Upload `build/BibleText.ipa`; wait for processing and any export-compliance
   prompts; select exactly version 1.1.8/build 157.
11. Run an installed-build smoke test on a real iPhone and iPad: first launch,
   Books/Search/Go-to, light/dark mode, rotation and Split View, Save Image and
   Photos permission, streaming/device audio with background controls, offline
   relaunch, Keychain migration, AI key test, AI action, and key deletion.
12. Resolve every warning, then use **Add for Review** / **Submit for Review**.

Submission itself is intentionally left to the authorised account holder.
