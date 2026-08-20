# The shared-note payload — the frozen format

> **Read this before changing anything in `share_note.go`.** Once a build ships,
> its decoder can never be changed: those readers may never update, and links
> live for years in message threads. Everything here is chosen on that basis.

## The situation, stated exactly

A note travels **inside the URL fragment**, base64url-encoded. A fragment is
never transmitted to a server, so the note is seen by the sender, the recipient,
whatever messenger carried the link — and by nobody else, ever, including us.

Two facts shape every decision below, and both were verified rather than assumed:

1. **There is no installed base.** `git ls-tree -r v1.1.7` — the newest shipped
   release — contains none of this code. The `'p'` and `'z'` formats have never
   reached a user. We are choosing a format with nothing to be compatible with,
   which is a position Bitcoin has not been in since 2009.
2. **Not every decoder freezes.** The web reader is a static site republished by
   one script; its decoder can be corrected tomorrow. The frozen tier is **app
   builds only**. That maps onto Bitcoin's own consensus/policy split, and it is
   why tolerance lives in the app decoder while strictness lives in the encoder,
   the web reader and the store — the three we can still change.

## The format

```
byte 0     framing discriminator (NOT a semantics version — see below)
             'r'  record stream, raw
             'd'  record stream, DEFLATE (emitted only when it comes out smaller)
             'p'  legacy bare text, plain      } never shipped to a user;
             'z'  legacy bare text, DEFLATE    } kept only for the owner's dev links
             'A'-'Z'  RESERVED: "a newer BibleText format"
bytes 1..  a record stream

record:    <tag: 1 byte> <len: uvarint> <value: len bytes>
```

| tag | field | required |
|---|---|---|
| `t` | note text, ≤ `NoteMaxRunes` | **yes** |
| `v` | sender's translation id | no — missing must degrade, never destroy |
| `b` | book, index into the frozen canon table (`bookslugs.go`) | no |
| `c` | chapter | no |
| `a` | verse runs | no |
| `n` | this note's own identity, 6 random bytes, minted once when the note is shared | no |
| `f` | sender display name — **reserved**, decoded and stored, never written or shown | no |
| `i` | sender id, 6 opaque bytes — **reserved** | no |
| `0xFF` | **reserved**: stop parsing; everything after is opaque | — |

### `n` — why a note carries an identity, and why it is per-NOTE

A device has to be able to recognise its own note coming home. You share a
verse with a message, you tap your own link later, and without this the app
stores a second copy and shows your own words back to you under "Note from
Friend" — which is exactly what it did until 20 Aug 2026.

**Content cannot answer that question.** "Amen" on the same verse from a friend
is a different note that happens to read the same. Collapsing on the words would
file their message as yours and lose it, against a store whose charter is that
it keeps what it is given.

**It is per-NOTE, not per-sender, and that is the privacy line.** The reserved
`i` and `f` are *sender* identity: one value, in every link you ever send,
visible to everyone each is forwarded to. `n` is minted fresh for each share, so
two links you sent carry unrelated values and nothing links them to each other
or to you. It is 6 bytes — 2^48 values, from `crypto/rand` rather than a
predictable source, so no one can craft a link a reader's device mistakes for
one of their own.

**It is optional in both directions.** A link without `n` — every link made
before this existed, and every link whose note a build could not read — simply
does not collapse, which is the older behaviour and no worse. And because `n` is
lowercase, every already-shipped decoder skips it by its length and preserves it
verbatim (rule 3), so old builds are unaffected and a forward from one keeps the
identity intact.

## The five rules, and why each is the way it is

### 1. Tag case carries criticality

**Lowercase = optional.** An unknown lowercase tag is skipped by its length and
the note is still shown.

**Uppercase = critical.** An unknown uppercase tag means the note body is *not*
rendered and the reader is told the note needs a newer version.

Precedent: BIP 21's `req-` parameters, BOLT 1's odd/even type parity, X.509
critical extensions, COSE `crit`. Every one of them has **both halves**.

Only having the permissive half is a permanent promise that no future field can
ever be required. The first time one is wanted — a redaction, an expiry, an
"edited", a "do not forward" — the only remaining tool is a new format byte,
which downgrades *"shown without its new field"* to *"not shown at all"*. Segwit
v0 script made exactly this mistake and had to invent `OP_SUCCESSx` in tapscript
to get out of it.

Cost today: one sentence and one `if tag >= 'A' && tag <= 'Z'`. Cost tomorrow:
impossible.

### 2. Canonical form is an EMITTER rule, not a decoder rule

Emitters MUST write records ascending by tag, at most one of each. Decoders MUST
accept any order, and apply **first-occurrence-wins** on a duplicate tag.
**Reject only on framing failure.**

This reverses the original design, and the reason is that our situation is not
Bitcoin's. Every canonicality rule in Bitcoin exists because bytes are hashed and
signed, so a re-encoding is a different txid and that was a theft vector. Nothing
here is hashed, signed, or identified by its bytes. A stream arriving as `s,c,t`
instead of `c,s,t` steals nothing — you know exactly what the sender meant — and
rejecting it costs somebody their only copy of a message.

The party most likely to emit a non-canonical stream is **our own encoder**, on a
platform we did not test. Gregory Maxwell, on Bitcoin's low-S rule: *"randomly
violated by all deployed bitcoin software prior to its discovery."* Their
response was encoder-first across four years — produce canonical (2013), relay
policy (0.11.1, October 2015, explicitly "Consensus behavior is unchanged"), hard
rule only inside a NEW script version (2017), never retroactive. We cannot stage
it that way, so we take the forgiving side at the start.

Corollary, from BIP 62: **enumerating malleability vectors one at a time
failed** — nine of them, two unfixable, three years, abandoned. Every degree of
encoding freedom left in a frozen format is permanent. Ours, counted and
accepted: record order, duplicate tags, non-minimal varints, DEFLATE's many
encodings of the same bytes, and trailing bytes after a complete DEFLATE stream
(verified: Go's `compress/flate` accepts them and returns `nil`).

⇒ **Byte equality implies note equality. Note equality does NOT imply byte
equality.** No feature may ever depend on note identity — no dedup by payload, no
edit-detection, no "you already have this one". Dedup is by content tuple
(`sameNoteContent`), which is a different and stable thing.

### 3. Unknown records are PRESERVED, not merely skipped

BIP 174 makes this a MUST: a parser "must pass those key-value pairs through when
re-serializing". The moment anything can forward, quote or re-share a note,
skipping-and-dropping silently destroys the sender's data on its way through us.

### 4. Byte 0 is a framing discriminator, and there is no semantics version

Bitcoin keeps two kinds of version cleanly apart and never conflates them: the
segwit marker/flag byte, which the decoder branches on, and transaction
`nVersion`, which the decoder never branches on and which was semantically
meaningless from 2009 until BIP 68 in 2016. Byte 0 is the marker.

The original argument — that a version integer "mainly creates opportunities to
refuse notes" — was **wrong in its reasoning even though the conclusion holds**.
Refusal is not a side effect of a version field; it is its entire purpose. BIP
174: *"if a parser encounters a version number it does not recognize, it should
exit immediately."* And that safeguard was genuinely spent — BIP 370 removed a
global field that no unknown-key-skipping rule could have absorbed.

The correct statement: **we already have the only version field we need, and it
is a framing discriminator rather than a semantics gate.**

BIP 9's own motivation is the supporting evidence: BIP 34's monotonic integer
"only supports one single change being rolled out at once… and does not allow for
permanent rejection", and Bitcoin destroyed most of a 32-bit field's value space
before abandoning it for independent feature bits — which is structurally what
"skip unknown tags" already is.

**MUST NOT #1** — never add a semantics-version record inside the stream. An old
reader would skip it as an unknown lowercase tag and then apply v1 meaning to a
v2 note, silently, with no error. That is the worst failure mode available. If
semantics versioning is ever needed it is an UPPERCASE tag or a new format byte.

**MUST NOT #2** — byte 0 changes only for a change to the *envelope* (framing or
codec), never for a semantic change to the records.

### 5. What a reader does when it cannot read

**Tell the reader. Never fail silently. Never prompt an install.**

Silence is the one option no Bitcoin artifact ever chose for its own user — and
CVE-2018-17144 is the sharpest evidence why: Core 0.14.x hit an assertion and
crashed loudly, while 0.15.0–0.16.2 quietly accepted a double-spend, and quiet
was the dangerous one. Our own asymmetry points the same way for a simpler
reason: the note exists nowhere else, but **the sender still has the text**. A
silent drop is unrecoverable; a message is recoverable, because the recipient can
say "your note didn't come through."

Three distinct messages, and **the passage opens in all three**:

| condition | what the reader is told |
|---|---|
| byte 0 in the reserved `A`–`Z` range | this link carries a note written in a newer note format |
| any other unknown byte 0, or a framing failure | this link's note looks damaged |
| over the size cap, or a refused inflation | this link's note looks damaged |

**No call to action and no link.** Bitcoin Core *merged the removal* of exactly
this class of warning — PR #15471, shipped in 0.18.0 — because it "has the
tendency to scare users unnecessarily (and might get them to 'update' to
something bad)". A note arrives inside a message thread, the most
phishing-saturated context in consumer software, and `docs/SHARED_NOTES.md`
already requires that a note never read as a message from BibleText. A modal
saying "install a newer version to read this" is that phishing template rendered
in our own typeface on our own domain.

## The limits that are part of the format whether we say so or not

Bitcoin's March 2013 chain split was caused by a **Berkeley DB lock limit** — an
implementation artifact nobody had written down that had silently become
consensus, so v0.8 accepted a block v0.7 could not. Whatever v1.0 does IS the
spec. So these are named, pinned, and tested:

| limit | rule |
|---|---|
| `noteMaxInflatedBytes` | a named constant, not arithmetic at the call site. Read one byte past it and treat the surplus as proof of a lie. **Was broken** — see below. FROZEN at `NoteMaxRunes*4+1` for the legacy `'p'`/`'z'` framings, which carry only text. |
| `noteMaxRecordBytes` | the `'r'`/`'d'` RECORD STREAM cap — raw length and inflated length, one number (4096). A record stream carries more than the text (the anchor records, the reserved sender fields, unknown fields a build must skip-and-preserve), so the legacy cap would refuse legal notes; this one bounds what a hostile payload can make a phone hold instead. Named here because it is part of the format from the first shipped build: it may GROW in a later build (old readers would then refuse the biggest new payloads as damaged — a degradation), it may never shrink. |
| `NoteMaxRunes` | encoder-side truncation; the decoder must not enforce a rune cap and refuse a note over it |
| URL length | encoder-only, degrading in a defined order; the decoder must NEVER enforce it |
| non-minimal varints | required minimal on emit, **accepted** on decode (Go's `binary.Uvarint` accepts `80 00` as 0) |
| trailing bytes after DEFLATE | accepted, deliberately, and recorded here as the reason byte-uniqueness is unavailable |
| empty record stream | invalid |

**The bomb guard was broken and is fixed.** It read exactly the cap through an
`io.LimitReader`, and a `LimitReader` reports `io.EOF` at its limit, which is not
an error — so a payload engineered to expand without bound came back TRUNCATED
with `err == nil`, and those bytes were rendered as words somebody had written.
Measured: 5,114 bytes expanding to 5 MB returned 1,121 bytes and no error. The
function's own doc comment had promised the opposite since the day it was
written. This is the whole lesson in one function: **never let a declared or
produced size drive an allocation, and validate before you trust.**

## Where the Bitcoin analogy breaks

Recorded so nobody cargo-cults it later.

- **Anything justified by consensus does not transfer.** Activation thresholds,
  signalling, BIP 9 timeouts, flag days, chain splits. We have no network, no way
  to count our readers, and nothing that can "split". Any lesson whose payoff
  sentence is "otherwise the chain forks" is not a lesson for us.
- **Malleability does not transfer**, except as the reason to forbid
  identity-based features (rule 2).
- **"Fail closed because money" is not the rule Bitcoin actually follows.** It has
  two policies chosen per layer, and the discriminator is not stakes — it is
  whether a wrong parse yields a plausible-looking wrong object. Framing: closed
  (the segwit marker byte was chosen so old parsers *cannot* succeed). Semantics:
  open (old nodes treat witness programs as anyone-can-spend and assume
  validity). In a money system they chose to ACCEPT what they could not verify.
  We apply the same discriminator, and it lands the same way: **closed on
  framing, open on semantics.**
- **Threat model.** Ours is a messenger truncating a URL, our own encoder
  misbehaving on an untested platform, and a prankster crafting a hostile
  fragment. Only the last is adversarial.

## Accepted hazards, named rather than discovered

- **A truncation landing exactly on a record boundary yields a shorter note, not
  a damaged one.** Canonical emission order is ascending by tag, so `t` precedes
  `v`; a messenger that cuts the URL precisely after the `t` record produces a
  payload that decodes ok with the full text and no translation record, and the
  note files under the (lossy) path version. Every other cut point is damaged.
  The window is narrow — it requires the `r` form (long notes take `d`, where any
  cut breaks the DEFLATE stream) and a cut on the exact boundary — and the
  alternative, refusing notes missing `v`, violates "missing must degrade, never
  destroy". Accepted, with eyes open.
- **The base64 tolerance is exactly: url-safe alphabet, correctly-formed
  trailing padding.** A `+` or `/` spelling, interior `=`, or wrong-length
  padding is damaged. The web reader initially forgave all three (atob accepts
  both alphabets); pinned in the corpus after review.

## Conformance corpus

`testdata/note_vectors.txt` is append-only: `(payload, expected outcome)` pairs
that **both** the Go decoder and the web reader must agree on — out-of-order
records, duplicate tags, non-minimal varints, zero-length values, trailing bytes,
truncated records, unknown lowercase tags, unknown uppercase tags, unknown format
bytes, and a deflate bomb.

It exists because the rule that lives only in an encoder's behaviour is the rule
the second implementation silently breaks. Bitcoin's own CompactSize canonicality
requirement was consensus-critical and went **undocumented for seven years**
(bitcoin#8721), and Zcash nearly inherited a security bug from the omission. We
have four decoders across iOS, Android, desktop and the web; two of them already
disagree about invalid UTF-8.
