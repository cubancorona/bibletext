# Google Play listing — copy, assets, and console review

BibleText's Google Play listing exists and its first submission is with Google.
1.2.7 (versionCode 177) was sent for review on 8 September 2026 on the closed
testing track "Alpha", targeting 176 countries, with testers supplied through the
Google Group `testers-community@googlegroups.com`. Production access has not been
granted and no production release exists.

Prepared release identity:

- package: `uk.co.bibletext`
- version: 1.2.7
- versionCode: 177
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
show an older interface and must not be uploaded for 1.2.7. They date from 4
July 2026 and predate the note chrome, the current typography, and the grouped
Books grid. Recapture at least:

1. reading with edition-correct red letters;
2. Search/cross-references;
3. the grouped Old Testament / New Testament Books grid;
4. shared notes with plainly synthetic text; and
5. the NKJV translation/settings state.

Use the release build or an equivalent current emulator build, inspect every
final image visually and with OCR, and meet Play's current aspect-ratio and pixel
requirements. Do not overwrite the old assets in place until the new set has
been reviewed side by side.

**Capture on a freshly booted emulator, and set the appearance BEFORE the app
starts.** Changing the system theme while it is running brought it back with
its pane occupying the top ~45% of the window - readable, and useless as a
store image. Worse, it is invisible to the obvious check: "does content reach
the bottom of the screen" passes anyway, because the page background is not
the system background. `scripts/play-shot-check.py` asks the specific question
instead - is there ink where the tab bar belongs - and it caught two images
that had already been committed as fine.

**A candidate set is in `play-assets/2026-09-1.2.5/`**, captured from the
versionCode 176 release APK on a Pixel 7 emulator. It predates 1.2.7's reading
face (Junicode with small-capital divine name) and the publishers' own
paragraphing, so it no longer shows the shipped text; recapture before the
listing goes public. It shows: reading with red
letters, search results, the grouped Books list, the same passage in the NKJV
fetched live through API.Bible, and the translation picker showing the licence
notice. They sit beside the old set rather than replacing it, per the paragraph
above.

**A raw phone capture is not uploadable.** A Pixel screenshot is 1080x2400,
which is 2.222:1, and Play rejects anything past 2:1. The candidates are
cropped to 1080x2160 - exactly 2:1 - by removing the status bar and the gesture
pill, which a store image should not show anyway. Check this on any future
capture: the aspect rule is the failure that only shows up at upload.

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

## Closed-test release notes — 1.2.8

> BibleText 1.2.8 rebuilds the verse of the day: a short passage rather than a
> clause, set in the reading face at your text size with Christ's words in red,
> shareable from the card in its own chapter. The rotation moves between books
> daily and every reader on every translation sees the same passage on the same
> date; the day changes at your local midnight. Updating re-times the rotation
> once. No ads, account, analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.8 (178);
- tap the sparkle in the header: the card shows a passage in the reading face
  at the Settings text size, with red text where the passage is Christ's words
  (Matthew 5 or John 14 are good days to look for) and none when red letter is
  off in Settings;
- share from the card's paper-plane control and confirm the citation names the
  passage's own chapter, not the chapter you were reading;
- tap "Read in context" and confirm the whole passage is highlighted;
- check the card the day after: a different book, and the change happens at
  local midnight, not at 01:00;
- switch to WEB Catholic and confirm the card can show a passage from Wisdom,
  Sirach, Judith or Baruch on its day, while WEB shows a Psalm or Isaiah on
  the same day;

## Closed-test release notes — 1.2.7

> BibleText 1.2.7 reads WEB, WEB Catholic, BSB, and the licensed NKJV; includes
> search, cross-references, narration/read-along, shared verse notes, and optional
> bring-your-own-key AI study. This release adds full-screen landscape reading on
> the phone, the publishers' own section headings and paragraphing, a new reading
> face, NKJV Psalm titles, and chapter-bottom footnotes. No ads, account,
> analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.7 (177);
- rotate the phone on the Read tab: the chapter fills the screen with no header
  or tab bar and the chapter arrows sit beside the reference, and rotating back
  restores the usual chrome; Books and Search keep the bottom tab bar;
- switch all four translations and test first-download/offline behaviour, and
  confirm section headings and paragraph breaks follow each publisher;
- check that the divine name renders in small capitals but copies as ordinary
  letters, and read a Hebrew-script passage in the new face;
- open Psalm 3 in the NKJV and confirm the superscription sits above verse 1;
- follow a footnote marker to the chapter-bottom footnote section;
- select a verse that carries a highlight and confirm the selection is visible
  over it, in light and dark appearance at more than one text size;
- **enter an AI provider key in Settings and confirm it survives a force-stop and
  relaunch** — 1.2.7 moves Android provider keys into the Android Keystore, and
  this is the change on this platform most worth a human confirming;
- use the grouped Books grid, Go to, keyword/reference search, and Back to
  results;
- open cross-references and Gospel parallels in differently numbered editions;
- send/receive/delete a note using neutral synthetic text; and
- play recorded narration/read-aloud, read-along, background continuation, and
  notification controls.

## Prepared console answers, and what iOS actually declared

Pulled from App Store Connect on 7 September 2026, so this is the shipped
record rather than a recollection:

| iOS declaration | Value |
| --- | --- |
| App Store age rating | **4+** (Brazil: L) |
| Age-rating declaration | **every content question at its default** - no violence, no mature or suggestive themes, no profanity, no horror, no gambling, no contests, no unrestricted web access, no user-generated content flagged |
| Primary category | Reference |
| Primary locale | en-GB |

That is the precedent for the IARC questionnaire, but it is not a substitute
for answering it: Apple and IARC ask different questions, and two of them below
have a defensible answer either way. Where that is so, it is said outright
rather than hidden in a tick.

### Create app

| Field | Value |
| --- | --- |
| App name | `BibleText` |
| Package name | `uk.co.bibletext` (confirmed available) |
| Default language | English (United Kingdom) - en-GB |
| App or game | App |
| Free or paid | Free |

### Content rating - the two that need a human answer

**User interaction.** IARC asks whether users can interact or exchange content
with other users. BibleText has no account, server, directory or messaging of
its own - but a reader can attach a short note to a verse and send it as a
link, and the recipient sees text another person wrote. The message travels in
the URL fragment through the sender's own messaging app and never reaches any
host of ours. So "no in-app user interaction" is true of the architecture, and
"users can share user-generated content" is true of the experience. Answer it
from what a rating body would consider a reader capable of receiving, and
declare the sharing rather than the plumbing.

**Mature themes.** The app presents scripture unaltered, and scripture narrates
violence and sexual content. Apple's form produced 4+ with no descriptors,
because the questions there are about depiction and simulation. IARC's wording
differs. Read the live questions and answer them about the text as presented.

Everything else is unambiguous and matches iOS: no ads, no in-app purchases or
subscriptions, no gambling or contests, no location sharing, no account, no
unrestricted web browsing.

### Data safety - the reasoning, not just the answer

The developer operates no analytics, advertising, account or application
server, and receives no reading history, notes or keys. Three off-device
transfers exist and each is initiated by the reader:

1. **Scripture and study data** are fetched from their documented providers.
   The provider learns which resource was requested. No reader identifier is
   attached.
2. **The licensed NKJV** is fetched through API.Bible, with the project key or
   the reader's own.
3. **Optional AI** sends the query or the selected passage to the provider the
   reader chose, under the reader's own key. The Assistant setting ships *on*
   with a provider preselected, but it is inert until the reader supplies a
   key - present and offered, not enabled. No key is bundled. The key is held
   in the system credential store on Apple platforms and in the app's own
   settings store on Android, Windows, and Linux.

Google treats collection and sharing as separate questions, and the
user-initiated-transfer exception answers only the sharing one. Collection,
though, turns on *transmission off the device* - not on whether the developer
persists anything. On that test the AI path is collection: the app itself POSTs
the reader's typed question to another company, which may retain it. So declare
**In-app search history** and **Other user-generated content** as collected and
shared, marked Optional, for App functionality; every other data type on
Google's list is No. Do not claim ephemeral processing - provider retention is
outside our knowledge. Declare all traffic encrypted in transit (it is - HTTPS
throughout), and that there is no account to delete, with in-app removal for
on-device notes and keys.

Notes are *not* declarable. A shared note rides in the URL fragment, which HTTP
never transmits, and the share sheet is a hand-off to another app on the device.

**Answer the live form, and record the reasoning where it is used.** The above
is the argument, not a set of ticks to copy.

### Two console decisions that are not paperwork

**Play App Signing.** Accepting its Terms of Service is a condition of
uploading an app bundle, and it means Google holds the app signing key while
this project keeps its own upload keystore and signs the GitHub APK itself.
That is a key-custody decision for the owner, not a formality.

**Automatic protection.** The create-app form offers an installer check added
to the app's code: a reader who obtained the app from another source is
prompted to get it from Google Play. BibleText deliberately publishes a
sideload APK on GitHub, so this would nag exactly the readers that channel
exists for. The form has a "Turn off" control at creation time. Decide it
deliberately.

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
