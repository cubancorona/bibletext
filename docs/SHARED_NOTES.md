# Shared notes — historical design plan

> **Historical and superseded.** This records the decision period before shared
> notes shipped in 1.1.8. It is not a current implementation or release-status
> document. See [`NOTES_SPEC.md`](NOTES_SPEC.md) for the live contract and
> [`NOTES_STATE.md`](NOTES_STATE.md) for the legacy-to-current state history.

When someone shares a verse as a link, let them attach a short note. Whoever
opens the link sees it as a dismissable speech bubble beside the passage —
on the web reader, or in the app when they have it installed.

## Two facts that decide most of the design

**1. There is no server.** bibletext.co.uk is static files on GitHub Pages and
the app has no account system, so a note can live in exactly one of two places:
in the link itself, or on a server we would first have to build — which means
storage, IDs, retention policy, abuse and moderation duty, GDPR answers, cost,
and an entirely new privacy story for an app whose current one is "your keys
never leave your device, we have no accounts, we collect nothing." The note goes
in the link.

**2. At design time, nothing that parsed these links had shipped.** Share-as-link,
the deep links and `share_link_parse.go` were being prepared for 1.1.8; the then
current public release, 1.1.7, contained none of that code. There was therefore
**no installed base to stay compatible with** while choosing the initial
grammar. This is historical rationale, not a statement about current releases.

That second fact removed what would otherwise have been the feature's first
compatibility wart (see the historical 1.1.8 decision below).

## The link grammar

The verse already rides in the fragment, for a reason worth keeping: browsers
never transmit fragments, so the note is seen by the sender, the recipient, and
whatever messenger carried the link — and by nobody else, ever, including us. A
query string would put the private half of the message into the request line
where GitHub's infrastructure logs it and any proxy can read it. In an app that
keeps API keys on device precisely so nobody else holds them, that is the one
option that contradicts the product.

Since the grammar is frozen the moment a link is sent, make it *extensible* now
rather than concatenating a second time later:

```
#v16                       one verse, as today
#v16-18                    a range, as today
#v16-18&n=<payload>        a range, with a note
#n=<payload>               a chapter link with a note, no verse
```

Keys are `&`-separated, `=`-valued; the `v` token keeps exactly the shape the
web reader already emits, so today's links stay byte-identical and a future key
costs no new format decision. It is unambiguous without regex gymnastics because
base64url's alphabet is `A–Z a–z 0–9 - _` and the payload is unpadded, so neither
`&` nor `=` can occur inside it.

Unknown keys must be **ignored, not rejected**, in every parser from the first
release onward. This preserves forward compatibility without another format
rewrite.

## The historical 1.1.8 question

Before 1.1.8 was uploaded, three paths were considered:

- **(a) Submit as-is.** Notes land in 1.1.9. Measured against the parser as it
  stands, a 1.1.8 user tapping a future note link gets John 3:16 instead of
  3:16-18 — the range collapses, because `parseVersePayload` cuts on the first
  `-` and fails `Atoi` on the tail. Right passage, narrower highlight, no note.
- **(b) Teach 1.1.8 the grammar, not the feature.** Roughly five lines: split the
  fragment on `&`, read the `v` key, ignore everything else. No UI, no new
  surface, no new risk — and every 1.1.8 user then opens every future note link
  at exactly the right passage. **Recommended.**
- **(c) Hold 1.1.8 and ship notes with it.** Largest delay, and it puts a brand
  new feature into a release that is otherwise finished and verified.

(b) is the cheap permanent fix: it costs one small, testable change now and
retires the only degradation this feature would otherwise carry forever.

## Encoding, and how long the links get

UTF-8 → raw deflate *only if it comes out smaller* (a flag says which) →
base64url, unpadded. Measured on realistic notes:

| Note | chars | raw | deflated | encoded | final URL |
|---|---|---|---|---|---|
| short | 44 | 46 | 48 | 62 | 107 |
| typical | 142 | 142 | 106 | 142 | 187 |
| long | 271 | 271 | 179 | 239 | 284 |
| emoji + accents | 73 | 81 | 84 | 108 | 153 |

Deflate earns its keep on longer notes and *costs* bytes on short ones, hence the
"only if smaller" rule. A **280-character cap** keeps every link under ~290
characters, which messengers handle without wrapping or truncating.

## Security: this is untrusted text on our own branded page

A note is arbitrary text, written by anyone, displayed inside BibleText. Treat it
as hostile input:

- **Never as HTML.** `textContent` on the web; escaped on the way into
  `buildChapterHTML` / `buildChapterHTMLAndroid`, both of which build HTML
  strings and would otherwise be an injection path straight into the reading pane.
- **Never chrome.** The bubble must read unmistakably as *"a note from whoever
  sent you this link"*, never as a message from BibleText. Otherwise the feature
  is a phishing kit: "BibleText security notice — confirm your key at …" rendered
  in our own typeface on our own domain.
- **No live links inside a note.** Plain text only, no auto-linking. A note that
  can render a tappable URL on our domain is the same problem wearing a hat.
- **Cap the length** (above), which also bounds the abuse surface.

Worth stating plainly: because the note never reaches a server, we never store or
transmit it, so there is nothing to breach and no moderation duty. That is a
consequence of the architecture, not a policy anyone has to maintain.

## What this cannot do

**Link previews will not show the note.** Unfurlers (iMessage, WhatsApp, Slack)
don't run JavaScript and never receive the fragment, so the preview shows the
chapter, as today. This one is permanent and there is no way around it while the
note stays private.

A partial answer, if it matters: **also put the note text in the shared message
body**, above the link. It is how people share things anyway, and it reaches the
recipient before they tap — including recipients who never tap, and those without
the app. Optional, not required — with (b) above, the note is no longer invisible
to anyone who opens the link.

## Surfaces to touch

Composing (share menu) and displaying (reading pane) are *separate* sets, and
each has four members. The "Share as link" work missed the macOS native menu on
the first pass precisely because the count is four, not three:

| | Compose | Display |
|---|---|---|
| iOS | native selection menu (`reading_ios.go`) | native `UITextView` overlay |
| macOS | NSMenu (`reading_macos.go`) | native `NSTextView` overlay |
| Android | bridge menu (`android/BtBridge.java`) | dialog overlay |
| desktop (Win/Linux) | `selectionStudyMenu` | `styledReadingPane` |
| web | — (read-only) | `cmd/websitegen` |

The web bubble should reuse the positioning machinery already built for the
clear-highlight pill (`positionBubble` in `assets.go`): document-absolute, pinned
under the last visible line of the highlight, re-pinned on resize.

## What had been built in this snapshot

- **Grammar + codec** (`share_link.go`, `share_note.go`) — done, with the
  back-compat table above pinned as assertions.
- **Web reader** — done: bubble anchored to the passage, minimize and delete,
  the tap on the highlight offering the same pair, the note carried across a
  translation switch.
- **App compose** — done, "Share with note" on all four menu surfaces. The note
  also goes into the shared message body.
- **Persistence** (`notes_store.go`) — done: one note per version+book+chapter,
  minimize and delete recorded in the store, picked up again in
  `addRecentChapter`, so a note returns on a later visit and survives relaunch.
- **iOS bubble** — done, as a native sticker (below).
- **Notes browser** (`notes_browse.go`) — done, on the SEARCH tab, not in
  Settings. A note is a message about a passage, so the only thing to do with
  one besides read it is go to the passage; the Search tab already owns "find
  something and tap through to it", and a note row is literally the same
  `searchResultCard` as a search hit. At the time, the desktop/iPad sidebar came
  free because both surfaces rendered through `buildSearchResultsView`.

  The control is an ICON beside the Search/Find pair rather than a third segment
  inside it: notes are a different corpus from the scripture those two look
  through, and a third text segment implied all three were the same kind of
  thing. Filtering matches the note's text AND its reference, is live, and keeps
  its own query (`AppState.NotesQuery`) — switching Search → Notes with a
  scripture term still in the box would otherwise answer "no notes match" to a
  search the reader never made. Empty query lists everything under a line saying
  so, which is what makes it a browser first and a search second.

  Sort is newest-first by default, because these are messages: the one that just
  arrived is the one you opened the list for. That needed `SharedNote.Received`,
  added additively — notes stored before it have a zero stamp and sort as the
  oldest, which is truthful and beats a migration. `saveNote` preserves an
  existing stamp, because saveNote is also how minimize persists itself and an
  unconditional stamp would shuffle a note to the top every time it was
  collapsed.
- **Dev link-testing page** (`dev_links_on.go`, `//go:build bibletextdev`) —
  done. A universal link cannot be triggered in the simulator and needs a tap
  from another app on a device, so this is the only way to drive that path
  directly: a fourth bottom-bar page whose rows call the real `HandleShareLink`.
  A build tag rather than a runtime flag so the scenarios are not compiled into
  the App Store binary at all; `TestReleaseScriptsNeverPassTheDevTag` asserts the
  release pipelines never opt in. Build with `run-ios-device.sh --dev`.
- **macOS / Android / desktop bubbles (recorded snapshot)** — were not done at
  this point in the implementation history. Those platforms then fell back to
  a dismissable card over the passage.

### The iOS sticker, and why it was not HTML

Measured on iOS 26.5, the NSAttributedString HTML importer drops `border`,
`border-radius`, `padding`, `box-shadow` and every margin on a `<div>`, so the
web reader's bubble markup arrives as a borderless, tinted run of text. It would
also join the text storage, where it would be selectable, copyable into "Share
with citation", and visible to the verse-index font-size scan. A Fyne widget is
equally impossible: the UITextView floats above the whole Fyne canvas, so a Fyne
bubble renders BEHIND the scripture.

So the note is a native `UIView`, placed in a band the text reserves via
`paragraphSpacingBefore` on the paragraph holding the highlight. That, rather
than an exclusion path, because an exclusion path needs the paragraph's rect to
place it while itself moving that paragraph — a feedback loop. Paragraph spacing
is part of the paragraph's own metrics, so layout converges in one pass. The
sticker is a subview of the text view, which IS a scroll view, so it scrolls
with the passage for free.

Two things that will bite anyone repeating this on macOS or Android:

1. `bibleTextApplyHTML` zeroes `paragraphSpacingBefore` across the whole string
   on every import (to kill a phantom band the importer injects before verse 1).
   The band must be carved AFTER that pass or it is wiped every render.
2. `chapterRenderFingerprint` gates the whole re-render. The note had to become
   part of it, or every appear, hide, restore and delete is silently skipped.

## Decisions that were open in this snapshot

1. **The 1.1.8 question** — (a), (b) or (c) above. Recommended: (b).
2. **Cap** — 280 characters unless there's a reason to go shorter.
3. **Note in the message body too?** Now optional rather than load-bearing.
4. **Attribution.** Recommended none: the messenger already shows who sent it,
   and a sender-typed name is an impersonation surface for nothing gained.
5. **Re-openable after dismiss?** A small marker to bring the note back, so an
   accidental tap doesn't lose it permanently.
