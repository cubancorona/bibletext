# Google Play listing — copy, assets, and console review

BibleText does not yet have a public Google Play listing. This document prepares
the record without claiming that an account, closed test, production access, or
submission already exists.

Prepared release identity:

- package: `uk.co.bibletext`
- version: 1.2.5
- versionCode: 176
- minimum SDK: Android 5.0 / API 21
- target SDK: Android 16 / API 36
- upload artifact: `~/Library/Android/bibletext-dist/BibleText.aab`

The version and versionCode come from `cmd/mobile/FyneApp.toml`; do not supply a
different manual `-app-build`. Produce the AAB only with
`scripts/build-android.sh --release`, then verify its manifest and signer as
described in [ANDROID.md](ANDROID.md). The current wrapper requires platform
`android-36` and build-tools `36.1.0`, applies the pinned Fyne tools v1.7.2
target-SDK patch, and verifies both debug APK and release AAB manifests. Google
requires API 36 for new apps and updates from 31 August 2026, so an API 35
artifact is not a valid prepared upload.

## App details

| Field | Value |
| --- | --- |
| App name | `BibleText` |
| Default language | `en-GB` |
| App or game | App |
| Price | Free |
| Category | Books & Reference |
| Tags | Bible, Reference |
| Contact email | configured project support mailbox |
| Website | `https://bibletext.co.uk/` |
| Privacy policy | `https://bibletext.co.uk/privacy.html` |
| Support page | `https://bibletext.co.uk/support.html` |

Keep the support address synchronized through the project's central contact
mechanism; do not copy a personal mailbox into this checklist.

## Short description

> A quiet, fast Bible reader. Search, study, and read — no ads, no tracking.

## Full description

> BibleText is a clean, unhurried place to read Scripture. No ads, no accounts,
> no tracking — just the text, carefully set, with the tools you use while
> reading.
>
> READ
> • World English Bible, WEB Catholic with the deuterocanonical books, Berean
> Standard Bible, and the licensed New King James Version
> • Words of Christ follow each edition's own publisher markings
> • Adjustable text size, light and dark appearance, poetry set as poetry
> • Chapter navigation, recent history, and Go to for references such as John
> 3:16
> • Works offline after the selected translation has downloaded
>
> SEARCH AND STUDY
> • Search words, phrases, and references across the active translation
> • Follow cross-references and Gospel parallels
> • Optional Study with AI and Find using your own Gemini, OpenAI, Anthropic, or
> Grok provider key; leave Assistant set to None to disable them completely
>
> SHARE AND NOTES
> • Share a passage as citation text, a typeset image, or a link
> • Add a short note to a verse link; it travels inside the link, not through a
> BibleText account or server
> • Browse, search, dismiss, or delete notes on your device
>
> LISTEN
> • Complete public-domain recorded narration for WEB and BSB, a synthetic
> public-domain voice for the WEB Catholic edition's Greek books, plus on-device
> read-aloud where supported
> • Read-along highlighting, chapter continuation, background playback, and
> lock-screen/notification controls on Android
>
> PRIVATE BY DESIGN
> • No ads, analytics, account, or tracking
> • Full Bible texts and study data are fetched from their documented providers
> and cached; the licensed NKJV is fetched through API.Bible
> • AI requests go directly to the provider selected under the reader's own key
> • Free and open source: github.com/cubancorona/bibletext

## Graphics

The icon and feature graphic under `docs/play-assets/` remain usable:

| Asset | File | Spec |
| --- | --- | --- |
| App icon | `icon-512.png` | 512×512 PNG |
| Feature graphic | `feature-graphic.png` | 1024×500 PNG |

The existing `01-reading.png`, `02-search.png`, and `03-books.png` phone images
show an older interface and must not be uploaded for 1.2.3. Recapture at least:

1. reading with edition-correct red letters;
2. Search/cross-references;
3. the grouped Old Testament / New Testament Books grid;
4. shared notes with plainly synthetic text; and
5. the NKJV translation/settings state.

Use the release build or an equivalent current emulator build, inspect every
final image visually and with OCR, and meet Play's current aspect-ratio and pixel
requirements. Do not overwrite the old assets in place until the new set has
been reviewed side by side.

## Data safety

Do not copy a previous “No data collected or shared” answer without reviewing
Google's current definitions. The developer operates no analytics, advertising,
account, or application server and does not receive reading history, notes, or
keys. However, the app makes off-device requests:

- translation/study/audio providers receive the resource being requested;
- API.Bible receives passage requests plus the project or reader API key; and
- when the reader invokes an optional AI feature, the selected AI provider
  receives the query or selected passage/action under the reader's account.

Google treats collection and sharing as separate questions, and a
user-initiated transfer exception for sharing does not automatically answer the
collection question. Complete the form from the final binary and current policy,
document the reasoning, and make the privacy policy match. All network traffic
is HTTPS.

## Content rating and other declarations

Review the live IARC questionnaire rather than carrying forward “No” answers.
BibleText has no chat room, stranger messaging, advertising, purchases,
gambling, location sharing, or account system, but the Bible contains mature
themes and optional AI services generate responses. The account holder must
choose the truthful frequency and age answers shown by the current form.

Also confirm from the final AAB:

- ads: none;
- in-app purchases/subscriptions: none;
- advertising ID: unused and `AD_ID` permission absent;
- app access: core reading/search needs no login; optional AI uses the reviewer's
  own provider key if they choose to test it;
- data deletion: no server account exists; on-device keys and notes have in-app
  removal controls; and
- target audience and Families eligibility are selected from the real intended
  audience, not to avoid or trigger a policy track.

## Closed-test release notes — 1.2.3

> BibleText 1.2.3 reads WEB, WEB Catholic, BSB, and the licensed NKJV; includes
> search, cross-references, narration/read-along, shared verse notes, and optional
> bring-your-own-key AI study. Red-letter text now follows each translation's own
> publisher markings. No ads, account, analytics, or tracking.

Suggested tester coverage:

- install/upgrade and confirm version 1.2.3 (174);
- switch all four translations and test first-download/offline behaviour;
- check NKJV Mark 5:31, Matthew 27:63, Luke 17:36, and Luke 24:7 with Words of
  Jesus enabled;
- use the grouped Books grid, Go to, keyword/reference search, and Back to
  results;
- open cross-references and Gospel parallels in differently numbered editions;
- send/receive/delete a note using neutral synthetic text;
- play recorded narration/read-aloud, read-along, background continuation, and
  notification controls; and
- rotate phones and tablets between portrait bottom tabs and the landscape left
  rail, confirming the reading pane remains usable after a warm App Link.

## Play Console flow

1. Create and verify the developer account.
2. Create the app record and complete the current policy/declaration forms.
3. Upload reviewed graphics and the verified API-36 AAB.
4. Create a closed-testing track, add the tester list, and roll out the test.
5. For a new personal account, keep at least 12 testers continuously opted in for
   14 days, then apply for production access.
6. Re-check listing copy, data safety, rating, target devices, countries, and
   release notes before any production rollout.

Console actions are deliberate human release steps. Nothing in this document
creates the account, uploads an artifact, or publishes a listing.
