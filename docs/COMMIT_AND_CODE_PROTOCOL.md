# Writing commits, comments, and code here

This repository is public. Commit messages, code comments, and documents in
it are permanent, world-readable, copied by anyone who clones, and served by
the GitHub API. Removing something later means rewriting history, and
anything already fetched is beyond recall. Write accordingly.

## Enforcement

The rules above are mechanical, twice over:

- `scripts/check-repository-hygiene.py` scans every pushed commit range in CI
  — messages and changed file states — with self-tested patterns for
  attribution, session narration, authorship trailers, and the rest. A
  violation fails the build.
- `.githooks/commit-msg` runs the same message patterns at commit time
  (`git config core.hooksPath .githooks` activates it, once per clone), so a
  banned message is refused while it is still cheapest to fix — before it is
  history.

The patterns are deny-lists and will always lag inventive new phrasings.
When one slips through, the fix is two lines: the phrasing goes into
PROCESS_PATTERNS, and a fixture proving the new rule can fire goes into the
self-test — the same commit that cleans up the instance.

## The one rule everything else follows from

> Describe **the change and why it is correct** — not the conversation that
> led to it.

The test: *would this make sense to a stranger reading the repository in five
years, who has no idea who worked on it?* If a sentence only makes sense as
part of a conversation, rewrite it or drop it.

---

## Never write a person into the repository

**No names.** Not a colleague's, not a tester's, not a family member's, not
your own. No nicknames, no handles, and no family relationships used as a
label for someone.

Someone who tried the app and mentioned a problem never agreed to appear in a
public repository, and cannot agree after the fact. Naming them — especially
quoting their criticism of the work — discloses something that was never
theirs to lose. Treat third parties as the strictest case.

Deliberate public attribution is different and stays: the project identity
and its published contact address, credits for public-domain or licensed
works (narrators, translation publishers, font and library authors), and
names that occur in scripture.

**No attributing the work to a person.** Don't record who asked for a change,
who noticed a defect, or who signed it off:

```
(owner asked)              the owner reported X        as requested
owner, <date>: "<their words>"  owner's report, exactly:    per your instruction
you were right             the maintainer's Macs       the developer's Library
```

`owner` is fine as a *domain* term — a note has an owner in the data model.
It is attribution of *work* that does not belong.

**No quoted speech.** Convert what someone said into an impersonal statement
of the defect or the requirement. The defect is the engineering content; who
reported it, and their exact words, are not.

The examples below use placeholders on purpose. Spelling out a real name or
a real quotation in order to warn against it would publish the very thing the
rule exists to keep out — this document lives in the repository too.

| Instead of | Write |
|---|---|
| `THE DEFECT (owner, <date>): "<verbatim words from a conversation>"` | `A note bubble the reader wrote offers both Minimize and Close, which have the same effect: both hide the pill.` |
| `UI: touch-target size pass (<person>'s "<their complaint>")` | `Enlarge audio controls to the 44pt minimum touch target` |
| `Reported via IMG_####.` | *(nothing — describe the defect instead)* |

---

## Never write a machine, device, or account

- **No camera-roll or screenshot filenames.** An `IMG_####` reference
  identifies a personal device and a moment in its photo library, and means
  nothing to any future reader.
- **No home-directory paths** — `/Users/<name>/…`, or `~/Library/Caches/…`
  described as *this machine's* state. No "the dev Mac", no "the previous
  machine".
- **No personal account state** — which AI provider you pay for, which keys
  are saved, what is in your real preferences file.
- **No private credential or console identifiers.** API key IDs, issuer IDs,
  device UDIDs, and private-key filenames use `XXXX` placeholders, and private
  key files are never tracked. Platform identifiers that must be public for a
  feature to work—such as the shipped bundle ID and the Apple application
  identifier in the universal-links association—are deliberate exceptions.
- **No obsolete personal bundle identifiers.** Refer to one as "the previous
  personal bundle id" rather than reprinting it. Use the current public bundle
  ID only where it is functionally required.

---

## Commit messages

- **Subject:** imperative, ≤ 72 characters, no trailing period, says what the
  change does — `Order the version picker by what the reader can act on`.
- One blank line, then a body if it adds something.
- **Body:** plain prose wrapped at ~72 columns. The problem, the change, and
  any measurement or trade-off a future maintainer would need.
- **No trailers.** No `Co-Authored-By`, no generated-by footer, no signature.
  Tools are not authors: authorship implies accountability, which a tool
  cannot hold.
- No first or second person, no apologies, no hedging, no chat tone, no
  emoji.
- No rhetorical capitals (`WHY NOT SAY BOTH`, `MEASURED, NOT ASSUMED`). Save
  capitals for real identifiers and constants.

Length is not a fault. A long message explaining a genuine trade-off in a
measured, impersonal voice is a good message. Keep the measurements, the
rejected alternatives, and the reasoning — drop the narration.

## Code comments

Same rules, plus one: a comment explaining why code exists should describe
the **behaviour or the defect**, never its social history.

```go
// BAD
// The owner's report, exactly: "<verbatim words from a conversation>".

// GOOD
// A browser tap must always surface the note. It previously left the pill
// minimized on some routes; this table enumerates every route so none is
// missed again.
```

A comment that names a person also ages badly in a way a behavioural one does
not: the person leaves, the reader changes, the behaviour stays true.

---

## Before you commit

Read the message and the diff's comments back once, asking only:

1. Does any sentence name or quote a person?
2. Does any sentence say who wanted this?
3. Does anything identify a machine, a device, a photo, or an account?
4. Would a stranger understand it without the conversation?
5. Subject ≤ 72 characters, imperative, no trailer?

If it is not yet pushed, `git commit --amend` is a complete fix. Once pushed,
assume it has been fetched: rewriting is still worth doing, but treat the
information as disclosed, and if it concerns another person, tell them.

When you do have to rewrite history, take a backup first
(`git bundle create backup.bundle --all`), use `git-filter-repo`, re-point
the tags, and verify with a control — prove your search finds the string
somewhere it still exists before you trust a zero anywhere else.

### While you are cleaning it up, do not spread it

This is the rule that is easiest to break, because breaking it feels like
being thorough.

Every artifact produced during a cleanup is a **new copy of the thing being
removed**: the status report, the remediation document, a note kept for next
time, a prompt handed to a tool or another person, a ticket, a message. Quote
the offending text into any of those and it now lives somewhere new — often
somewhere with no cleanup plan of its own.

**Describe the shape and the location. Never reproduce the content.**

> "A commit subject names a tester and quotes their complaint about the audio
> controls" — fully actionable, and it spreads nothing.

That is enough for anyone to find it and fix it. The verbatim string adds
nothing except another disclosure.

The same applies to this document and to any guidance written afterwards:
warning examples must be **manufactured or written as placeholders**, never
lifted from the real thing. A rule illustrated with the live data undoes
itself.

One trap worth knowing: making the repository private is **not** a safe way
to contain a leak. It deletes the GitHub Pages configuration, which takes
bibletext.co.uk offline, and the site does not come back when the repository
is made public again — Pages has to be recreated and HTTPS enforcement turned
back on by hand.
