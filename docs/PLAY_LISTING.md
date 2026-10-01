# Google Play listing — copy, assets, and console review

BibleText is live on Google Play, at
https://play.google.com/store/apps/details?id=uk.co.bibletext. Its first
production release, 1.2.17 (versionCode 187), went out on 30 September 2026 to
all 178 countries, and the download page links the listing with the Play
badge, above the APK attached to each GitHub release, which it also offers
among its files for anyone without Google Play. Before production, releases
went to the closed testing track "Alpha", targeting 176 countries, with
testers supplied through the Google Group
`testers-community@googlegroups.com`. The first submission, 1.2.7 (versionCode
177), was sent for review on 8 September 2026; 1.2.9 (179) followed on 15
September. 1.2.10 is a desktop-packaging release and was deliberately not sent
to Play.

Release identity in production:

- package: `uk.co.bibletext`
- version: 1.2.17
- versionCode: 187
- minimum SDK: Android 5.0 / API 21
- target SDK: Android 16 / API 36
- upload artifact: `~/Library/Android/bibletext-dist/BibleText.aab`

1.2.18, versionCode 188, is being prepared, with its release notes in the
1.2.18 section below. A release goes up to the alpha track, as far as the
service account can reach, and the owner promotes it to production in the
Play Console (docs/RELEASING.md, stage 6).

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

The scenes, the capture recipe for the phone and the 10-inch tablet, the
checks and the upload steps are in docs/SCREENSHOT_PLAYBOOK.md. The phone and
tablet screenshots go up with `play/push-screenshots.py`, read-only first,
then `--rehearse`, then `--write --confirm-version <v>` (§6 there); the
feature graphic and the icon go up in the console.

The existing `01-reading.png`, `02-search.png`, and `03-books.png` phone images
show an older interface and must not be uploaded. They date from 4 July 2026
and predate the note chrome, the current typography, and the grouped Books
grid.

The layout check in `scripts/play-shot-check.py` dates from the September
capture: changing the system theme while the app was running brought it back
with its pane in the top ~45% of the window, a "content reaches the bottom"
test passed anyway, and asking whether there is ink where the tab bar belongs
failed two images that had already been committed as fine.

**The 1.2.17 set replaces `play-assets/2026-09-1.2.5/` on the listing.**
`play/push-screenshots.py --write` committed eight phone and eight 10-inch
tablet images from `build/play/screenshots-1.2.17/` on 30 September 2026, and
a fresh edit read them back matching by sha256. The change goes through
Play's review, so the store shows the 1.2.5 set until the Play Console shows
it published. The feature graphic and the icon did not change. The 1.2.5
set stays in `play-assets/2026-09-1.2.5/` as history: captured on 7 and 8
September 2026 from the 1.2.5 release APK, it predates 1.2.7's reading face
and was cropped from 1080x2400, which took the status bar and the top of the
app header. [SCREENSHOT_PLAYBOOK.md](SCREENSHOT_PLAYBOOK.md) §3 records what
each store shows, and §7 why the 1.2.17 phone set uses the Small display
size.

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

## Release notes — 1.2.18

> Notes that close cleanly, and a status bar you can read.
> • Closing Add a note while the keyboard is up no longer leaves the page squashed above where the keyboard was.
> • On the light page, the clock and icons at the top of the screen are now dark. They were white and hard to see.
> • If the share menu cannot open, BibleText now says so instead of closing.
> • With your phone on its side, the Add a note box now stays clear of the camera cut-out and side buttons.

Suggested tester coverage:

- install and confirm version 1.2.18 (188);
- select a few words, choose Add a note, type a word so the keyboard is up,
  then press Cancel; repeat and press Share: each time the page, and the
  Read, Books and Search bar, come back to the full height of the screen;
- in light mode, check the clock, battery and signal icons at the top of the
  screen are dark and easy to read, and on a phone with three-button
  navigation that the buttons are too; switch to dark: both turn light;
- with the system share menu unavailable (an emulator with the chooser
  disabled), Share with citation shows "Could not share the passage." and
  the app stays open;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.17

> See which AI is working, and your note where you left it.
> • The AI waiting screen now names the assistant and model at work, and follows a switch to a faster model.
> • Adding a note shows the words you selected above it.
> • A note you send appears on the verse straight away.
> • Switching between light and dark with a sheet open no longer leaves it half in the old colours.
> • After Android closes the app's screen in the background, light and dark and downloads work again when you return.

Suggested tester coverage:

- install and confirm version 1.2.17 (187);
- with an AI key saved, ask a question and then run an AI Find: while each
  waits, the waiting screen names the assistant and model at work, and
  switching to a faster model while it waits changes the name;
- select a few words and choose Add a note: the selected words show above
  the note, and a long selection stops after three lines;
- send a note with Share with note: it shows on its verse straight away, and
  is gone after turning to another chapter;
- open Go to, the verse of the day, Settings and the translation list in
  turn, and switch between light and dark with each open: the sheet comes
  back whole, in the new colours;
- turn on "Don't keep activities" in Developer options (or leave the app
  with Home until Android closes its screen), return, then switch between
  light and dark and download a translation: both still work;
- open Settings on a small phone: the key buttons and the switch labels
  wrap inside their cards rather than running past the edge;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.16

> BibleText 1.2.16 fixes a crash on some phones when you press and hold on
> text already selected. On a tablet, a chapter now reads as a book page: a
> centred column with indented paragraphs. Verse numbers, footnotes and
> margins now match the iPhone's. The whole height of the tab bar answers a
> tap, and the narration no longer lights a section heading. If the NKJV
> cannot open when the app starts, the app now says so and keeps it as your
> choice. No ads, account, analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.16 (186);
- select a few words, then press and hold on the selection: the app stays
  open, and the selection and its menu stay;
- on a tablet (or a tablet emulator), open a chapter: a centred column with
  indented paragraphs; split the window narrow: full width with space between
  paragraphs;
- on a phone in portrait, compare a chapter with 1.2.14, the last build Play
  testers had: slightly wider side margins, and smaller verse numbers raised
  clear of the line;
- turn a phone sideways on a chapter with a section heading (Psalm 3 has a
  title, a heading and a footnote): the heading has space above and below it;
- tap each tab near the top edge of its icon, then under its label: each tap
  opens that tab;
- play the narration through a verse just above a section heading: the
  heading never lights;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.15

> BibleText 1.2.15 makes the whole height of the tab bar answer a tap, so
> tapping the top edge of a tab's icon now opens that tab, and the narration
> no longer lights a section heading along with the verse above it. The rest
> of this release is for iPhone and iPad. No ads, account, analytics, or
> tracking.

Suggested tester coverage:

- install and confirm version 1.2.15 (185);
- in portrait, tap each tab near the top edge of its icon, then near the
  bottom of the bar under its label: each tap should open that tab;
- play the narration through a verse that stands just above a section
  heading: the heading should never light up with it;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.14

> BibleText 1.2.14 shares the divine name as the page sets it: a verse from the
> NKJV now goes out with Lᴏʀᴅ in small capitals, in text, on picture cards and
> from the verse of the day. Sharing a whole such verse names exactly that verse
> and no longer carries its number into the message. The verse of the day no
> longer runs words together, and search results highlight your search as a
> phrase. No ads, account, analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.14 (184);
- open the NKJV, find a verse setting the divine name in small capitals, share it
  as text, and confirm the pasted text reads Lᴏʀᴅ in small capitals and the
  reference names the verse you chose — including when the selection starts at
  the verse number;
- share the same verse as an image and tap Regenerate a few times: the name
  should stay in small capitals in every typeface;
- open the verse of the day on a day whose verse is in red, and confirm no two
  words run together;
- search for a two-word phrase and confirm the result cards highlight the
  phrase, not each word wherever it appears.

## Closed-test release notes — 1.2.13

> BibleText 1.2.13 fixes Share as image on Android 6 to 9, where tapping it used
> to do nothing at all — it now asks for the permission it needs, and says so if
> it still cannot save. Sharing a verse whose translation sets the divine name in
> small capitals now sends LORD, the way plain text has always written it, and
> the reference under it names the verses you chose. No ads, account, analytics,
> or tracking.

Suggested tester coverage:

- install and confirm version 1.2.13 (183);
- tap Share as image on a verse. On Android 10 or later it should open the share
  sheet as before; on Android 6 to 9 it should either ask for permission and then
  work, or tell you why it cannot — the one thing it must never do again is
  nothing at all;
- open the NKJV, find a verse setting the divine name in small capitals, share it
  as text, and confirm the pasted text reads LORD and the reference names the
  verse you chose;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.11

> BibleText 1.2.11 follows your phone's dark mode while the app is open. If
> your phone switches to dark at sunset, the app switches with it — it used to
> keep the old appearance until you rotated the phone or came back to it.
> Reading, search, notes and narration are unchanged. No ads, account,
> analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.11 (181);
- with the app open, switch the phone between light and dark (Settings →
  Display, or let the scheduled switch do it) and confirm the page follows
  without being touched;
- confirm a shared link still opens at its passage with its note.

## Closed-test release notes — 1.2.9

> BibleText 1.2.9 sets headings and spacing right: section headings in the
> Berean Standard Bible and the NKJV stand at the text's own size, in bold,
> with even air above and below; paragraphs sit one line apart rather than
> nearly two; the gap under a psalm's title follows your text size; and a
> collapsed note pill under a heading centres in its space. No ads, account,
> analytics, or tracking.

Suggested tester coverage:

- install and confirm version 1.2.9 (179);
- open the Berean Standard Bible at John 11: the section headings are bold at
  the text's size with clear air above each and a little below, and the
  paragraphs sit one blank line apart;
- change the text size in Settings: the paragraph gaps and the heading air grow
  and shrink with the text;
- open Psalm 23: the italic title stands a little above verse 1, and that gap
  follows the text size too;
- open a shared note on a paragraph a heading opens and minimize it: the pill
  sits centred between the heading and the verse.

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
