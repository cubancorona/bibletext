# Shared notes — design plan

> Status: **plan only**, nothing implemented. Branch: `shared-notes`.

When someone shares a verse as a link, let them attach a short note. Whoever
opens the link sees it as a dismissable speech bubble beside the passage —
on the web reader, or in the app when they have it installed.

## The constraint that shapes everything

**There is no server.** bibletext.co.uk is static files on GitHub Pages, and the
app has no account system. So a note can live in exactly one of two places:

1. **In the link itself** — self-contained, no infrastructure, nothing to run,
   nothing to moderate, nothing to leak, and it still works in ten years.
2. **On a server we would have to build** — which means storage, IDs, retention
   policy, abuse and moderation duty, GDPR answers, cost, and an entirely new
   privacy story for an app whose current one is "your keys never leave your
   device, we have no accounts, we collect nothing".

This plan takes option 1. Everything below follows from that.

## Where in the link the note rides

The parser that already ships (`share_link_parse.go`, in every 1.1.x app in the
wild) knows nothing about notes. Those apps **will** receive note-bearing links,
so how they degrade is a design input, not an afterthought. Measured against the
shipped parser:

| Candidate | Old app sees | Note stays client-side? |
|---|---|---|
| A. `#v16-18-n<payload>` | John 3:16 — **range lost** | yes |
| B. `?n=<payload>#v16-18` | John 3:16-18 — intact | **no** — sent to the server |
| C. `?v=16-18#n<payload>` | John 3:16-18 — intact | yes |

(Measured, not assumed: A collapses the range because `parseVersePayload` cuts on
the first `-` and then fails `Atoi` on the tail, whose error path returns the low
verse. C works because the `?v=` alias is already an accepted form, consulted
whenever the fragment has no `v` prefix.)

**C dominates A**, so the real choice is B vs C:

- **B** keeps the fragment as `#v16-18`, so the single-verse highlight stays pure
  CSS `:target` with zero JavaScript — but the note text goes into the HTTP
  request line, where GitHub's infrastructure logs it and any proxy can read it.
  For a private message between two people, in an app that keeps API keys on
  device precisely so nobody else holds them, that is a real regression.
- **C** keeps the note in the fragment, which browsers never transmit. The note
  is seen by the sender, the recipient, and whatever messenger carried the link —
  and by nobody else, ever, including us. The cost is that a note-bearing link's
  verse highlight needs JavaScript, since the fragment is no longer `#v16`.

**Recommendation: C.** The privacy property is the whole posture of this project,
and the cost is close to nothing: a note-bearing link already needs JavaScript to
draw the bubble at all, so requiring it for that link's highlight adds no new
dependency. Links without notes are completely unchanged and keep the zero-JS
highlight.

## Encoding, and how long the links get

UTF-8 → raw deflate *only if it comes out smaller* (a flag bit says which) →
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
transmit it, which means there is nothing to breach and no moderation duty. That
is a consequence of the architecture, not a policy we have to maintain.

## What this cannot do

Two limits to accept up front rather than discover later:

- **Link previews will not show the note.** Unfurlers (iMessage, WhatsApp, Slack)
  don't run JavaScript and never receive the fragment. The preview will show the
  chapter, as today.
- **Apps older than this feature will show the passage with no note, silently.**
  Universal Links cannot be version-gated — the AASA file is static and cannot
  ask what version is installed. So during the transition a note is guaranteed
  only on the web and on updated apps.

The second one has a good mitigation: **also put the note text in the shared
message body**, above the link. That is how people share things anyway, it means
the note is never invisible to anyone on any version, and the bubble becomes an
enhancement rather than the only delivery mechanism. Recommended.

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

- **P0 — contract + codec.** Extend `ShareLinkURL` / `ParseShareLink` for the
  note, add the encoder/decoder, and pin it with tests. No UI. Ends with the
  golden test extended and the back-compat table above locked in as assertions.
- **P1 — web reader shows the bubble.** A hand-made link now works end to end,
  which derisks the contract before any app code exists.
- **P2 — app composes.** Optional note field in the share flow; note text also
  goes into the message body. One-tap "share as link" must stay one tap — the
  note is an opt-in second step, not a new modal in everyone's way.
- **P3 — app shows the bubble** on all four reading surfaces.

P1 before P2 is deliberate: the web is the only surface that can render a note
without shipping an app release, so it is where the format should be proven.

## Open decisions

1. **Placement B or C** — privacy versus a zero-JS highlight on note links.
   Recommended: C.
2. **Cap** — 280 characters unless there's a reason to go shorter.
3. **Note in the message body too?** Recommended yes; it is what makes the
   feature work for people on older app versions.
4. **Attribution.** Recommended none: the messenger already shows who sent it,
   and a sender-typed name is an impersonation surface for nothing gained.
5. **Re-openable after dismiss?** A small marker to bring the note back, so an
   accidental tap doesn't lose it permanently.
