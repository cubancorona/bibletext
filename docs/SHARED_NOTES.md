# Shared notes — design plan

> Status: **plan only**, nothing implemented. Branch: `shared-notes`.

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

**2. Nothing that parses these links has shipped yet.** Share-as-link, the deep
links and `share_link_parse.go` are all in 1.1.8, which is built but not
submitted; the newest release in the wild, 1.1.7, contains none of that code.
So there is **no installed base to stay compatible with** — the grammar is free
to be whatever is best, and the first release that can open a shared link can
also be the first that understands a note.

That second fact removes what would otherwise be this feature's one permanent
wart. It is worth spending a little of 1.1.8's remaining pre-submission time on
(see "The 1.1.8 question").

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
release onward. That is the whole cost of never having this conversation again.

## The 1.1.8 question

1.1.8 is built and verified but not uploaded. Three ways forward:

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

## Phases

- **P0 — grammar tolerance in 1.1.8.** The five-line change above, plus tests
  pinning that unknown keys are ignored and the verse still parses. Ships with
  the release that is already built.
- **P1 — contract + codec.** `ShareLinkURL` / `ParseShareLink` carry the note;
  encoder/decoder with round-trip, cap, emoji and hostile-input tests.
- **P2 — web reader shows the bubble.** A hand-made link now works end to end,
  which proves the format before any app UI exists.
- **P3 — app composes.** Optional note field in the share flow. One-tap "share as
  link" must stay one tap — the note is an opt-in second step, not a new modal in
  everyone's way.
- **P4 — app shows the bubble** on all four reading surfaces.

P2 before P3 is deliberate: the web is the only surface that can render a note
without shipping an app release, so it is where the format should be proven.

## Open decisions

1. **The 1.1.8 question** — (a), (b) or (c) above. Recommended: (b).
2. **Cap** — 280 characters unless there's a reason to go shorter.
3. **Note in the message body too?** Now optional rather than load-bearing.
4. **Attribution.** Recommended none: the messenger already shows who sent it,
   and a sender-typed name is an impersonation surface for nothing gained.
5. **Re-openable after dismiss?** A small marker to bring the note back, so an
   accidental tap doesn't lose it permanently.
