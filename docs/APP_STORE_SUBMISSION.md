# App Store submission checklist — BibleText 1.1.6

Prepared 1 August 2026. This is a release-preparation record, not confirmation
that the binary has been uploaded or the version submitted for review.

## Release identity

| Field | Value |
| --- | --- |
| App | BibleText |
| Apple app ID | `6784567351` |
| Bundle ID | `uk.co.bibletext` |
| Version | `1.1.6` |
| Build | `103` |
| Platform | Universal iPhone and iPad |
| Minimum OS | iOS 13.0 |
| Price | Free |
| Primary category | Reference |
| Secondary category | Education |
| Copyright | `2026 Willow Noonan` |

The public App Store version at preparation time is 1.1.5. Do not change the
bundle ID or remove iPad support from this update.

## Locally verified release inputs

- `cmd/mobile/FyneApp.toml` is the source of version 1.1.6/build 103.
- `scripts/release-ios.sh` derives the marketing version from that file, keeps
  exact copies of locally edited build files, builds arm64, creates an App Store
  archive, and does not upload unless `BIBLETEXT_UPLOAD=1` is explicitly set.
- `cmd/mobile/PrivacyInfo.xcprivacy` is copied to the root of the app bundle.
- `cmd/mobile/LaunchScreen.storyboard` is compiled into the bundle for both
  iPhone and iPad.
- `UIRequiresFullScreen` is removed so iPad Split View, Stage Manager, and
  resizable windows are not disabled.
- AI provider keys use Apple Keychain on iOS/macOS and migrate from the old
  preferences value on first access.
- The App Store icon is an opaque 1024×1024 RGB image and includes the required
  phone and tablet icon renditions.
- The release uses the App Store distribution profile for team `R8PC7239T2`,
  has `get-task-allow=false`, and declares `ITSAppUsesNonExemptEncryption=false`.

Run the final local checks from the repository root:

```sh
go test ./...
go vet ./...
./scripts/release-ios.sh
```

The last command must finish with `build/BibleText.ipa is ready (version 1.1.6,
build 103)`. Do **not** set `BIBLETEXT_UPLOAD=1` during preparation.

## Store metadata prepared locally

The editable staging files are in `build/appstore/metadata/` (this directory is
ignored by Git). The prepared English (UK) values are:

- Name: `BibleText`
- Subtitle: `Read and study Scripture`
- Promotional text: the current private, bring-your-own-key summary
- Description: covers WEB, WEB Catholic, BSB, search, optional AI study, audio,
  iPad, offline use, no ads, and no accounts
- Keywords: 95 characters (within the 100-character limit)
- Support URL: <https://bibletext.co.uk/support.html>
- Marketing URL: <https://bibletext.co.uk/>
- Privacy URL: <https://bibletext.co.uk/privacy.html>
- What's New: Save Image, quotation correctness, Go-to navigation, and secure
  Keychain migration

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

Use the opaque JPEG exports, not the source PNGs (the PNG files have alpha):

- iPhone 6.9-inch slot: `build/appstore/screenshots-ready/en-GB/` — four
  1320×2868 images
- iPad 13-inch slot: `build/appstore/screenshots-ready/en-GB/ipad13/` — three
  2752×2064 images and one 2064×2752 image

Upload after the 1.1.6 English (UK) localization exists:

```sh
python3 build/appstore/upload_screenshots.py

ASC_DISPLAY_TYPE=APP_IPAD_PRO_3GEN_129 \
ASC_SHOTS_DIR=build/appstore/screenshots-ready/en-GB/ipad13 \
python3 build/appstore/upload_screenshots.py
```

The helper resolves the current version automatically and does not submit it.
Visually inspect the resulting order and device frame in App Store Connect.

## Privacy answers that need updating in App Store Connect

The public product page currently says **Data Not Collected**. That is too broad
for the optional AI requests sent to the provider selected by the user. Use the
same conservative disclosure as the bundled privacy manifest:

| Data type | Linked to the user | Tracking | Purpose |
| --- | --- | --- | --- |
| User ID | Yes | No | App Functionality |
| Search History | Yes | No | App Functionality |
| Product Interaction | Yes | No | App Functionality |

The rationale is that the user's API key authenticates a provider account, and a
Find query, or a selected passage and AI study action, leaves the device. The
provider may associate and retain the request under that account. BibleText has
no developer-operated server and does not use the data for tracking. Confirm
these answers against the chosen providers' live terms before saving the
questionnaire.

Publish the updated `docs/privacy.html` and `docs/support.html` to the live URLs
before review. The source files now describe Keychain storage and the direct AI
provider data flow, but editing `main` alone does not update the `gh-pages`
deployment.

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
4. Version 1.1.6 exists for iOS and is in an editable state.
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
   prompts; select exactly version 1.1.6/build 103.
11. Run an installed-build smoke test on a real iPhone and iPad: first launch,
   Books/Search/Go-to, light/dark mode, rotation and Split View, Save Image and
   Photos permission, streaming/device audio with background controls, offline
   relaunch, Keychain migration, AI key test, AI action, and key deletion.
12. Resolve every warning, then use **Add for Review** / **Submit for Review**.

Submission itself is intentionally left to the authorised account holder.
