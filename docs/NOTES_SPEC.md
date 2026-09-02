# Notes subsystem specification

This document records the durable architecture of shared and locally composed
notes. The implementation is authoritative for current behaviour. The frozen
wire contract in [`NOTE_WIRE_FORMAT.md`](NOTE_WIRE_FORMAT.md) is authoritative
for payload compatibility.

## Scope

The subsystem provides offline, link-carried notes without accounts or a
service-side note store. Its foundations are:

- stable local identity for every stored record;
- set-valued anchors that survive translation changes and discontinuities;
- a tolerant wire decoder paired with a canonical encoder;
- a line-framed, loss-averse local store;
- one derived chapter plan consumed by presentation surfaces;
- view-only focus and expansion limits with no persisted residue; and
- explicit preservation of data that the current build cannot interpret.

Notes and note metadata are untrusted input. Text is rendered as text, never as
markup. Record identity, placement, and presentation state remain separate.

## Record model

[`StoredNote`](../notes_model.go) is the durable record. Its significant fields
are:

| Field | Contract |
|---|---|
| `ID` | Local monotonic identifier. Minted once, never reconstructed from the current passage, and never reused after deletion. |
| `Kind` | Extensible string. Current classes are received and locally composed notes; unknown kinds survive rewrites but are not displayed. |
| `VersionID`, `Book`, `Chapter` | Filing frame for the original anchor. |
| `VerseLo`, `VerseHi` | Compatibility form of the first anchor run. |
| `AnchorRuns` | Full set-valued anchor, including gaps and cross-chapter runs. |
| `Text` | Normalized untrusted text. |
| `Minimized` | Durable only when an explicit close action changes it. |
| `Received` | Local arrival time used for stable display ordering. |
| `Nonce` | Per-note share identity used to recognize a locally sent note when its link returns. |
| `SenderName`, `SenderID` | Reserved attribution fields; inactive in current presentation. |
| `WireSkipped`, `WireOpaque` | Uninterpreted payload bytes retained for later forwarding. |
| `Extra` | Unknown JSON fields retained across read-modify-write cycles. |

All destructive and state-changing verbs address `ID`. Passage coordinates are
never an identity. Locally composed notes share the same store and identity
rules as received notes.

## Wire contract

A note occupies the URL fragment's `n` value as unpadded base64url. The decoded
blob starts with an envelope byte followed by a record stream:

```text
blob   = format record*
record = tag uvarint-length value
```

| Format | Meaning |
|---|---|
| `r` | Raw record stream. |
| `d` | DEFLATE record stream, emitted only when smaller. |
| `p` | Legacy plain text; decode only. |
| `z` | Legacy DEFLATE text; decode only. |
| `A`–`Z` | Reserved envelope indicating a newer format. |

Current emitted records are:

| Tag | Meaning |
|---|---|
| `a` | Anchor runs. |
| `b` | Book index in the frozen canon order. |
| `c` | Chapter. |
| `n` | Six-byte per-note nonce. |
| `t` | UTF-8 note text; required. |
| `v` | Translation identifier when known. |

Sender-related tags remain reserved and inactive. They must not acquire display
semantics through an incidental decoder change.

The encoder emits records in ascending tag order, uses at most one record per
tag, and uses minimal uvarints. The decoder accepts any order, accepts
non-minimal uvarints, and applies first-occurrence-wins to duplicates. Unknown
lowercase records are skipped and preserved verbatim. Unknown uppercase records
produce the explicit newer-format outcome. `0xFF` stops parsing and preserves
the remaining bytes as opaque data. Framing failures produce the explicit
damaged outcome; they never cause a store write.

The record-stream cap is 4096 bytes before or after inflation. Encoder input is
normalized to at most `NoteMaxRunes` (280 runes). Decoder compatibility remains
tolerant within the frozen limits. Envelope bytes change only for framing or
codec changes, not for record semantics.

The nonce belongs to one shared note, not to a device or a person. It may match
a returning link to a locally composed record, but it must never group multiple
notes or establish attribution.

## Store contract

[`notes_store.go`](../notes_store.go) uses two preference keys:

- `notes.store`: JSON Lines, one record per line;
- `notes.store.nextid`: decimal last-allocated local ID.

Serialization is deterministic: an optional wipe sentinel comes first, records
follow in ascending ID order, and quarantined lines remain at the tail. Known
JSON fields have stable order; unknown fields are appended in sorted key order.
An unchanged store therefore serializes to unchanged bytes.

An unparseable line is quarantined verbatim and re-emitted on every later
write. Byte-identical quarantine entries may be collapsed because they carry
identical bytes. A present value that yields no recognizable structure is a
whole-store read failure; every writer stands down instead of serializing an
empty collection over it.

An empty value denotes a new store. Intentional deletion of all notes writes
the first-line sentinel `{"wiped":true}` so an explicit wipe is distinguishable
from an ordinary empty value. The sentinel cannot diagnose loss of the entire
preferences file, so atomic preference-file publication remains required.

There is no capacity cap and no automatic eviction. Deletion requires an
explicitly addressed operation. The ID counter advances past IDs found in live
records and, where recoverable, quarantined records. Deletion never lowers the
counter.

Deduplication compares normalized content, anchor, version, and kind rather
than payload bytes. A duplicate retains its original ID, arrival time, and
minimized state. Missing preserved wire data or a fuller anchor may be adopted
from the later copy without replacing local history.

Migration from both legacy note preferences is best-effort. Unreadable legacy
data enters quarantine. Legacy keys are cleared only after the new store has
been written and read back successfully.

## Anchor and placement contract

[`resolveNoteAnchor`](../notes_anchor.go) returns a set of disjoint runs plus a
total placement classification. A bounding span is insufficient because a
translation mapping can contain gaps or move part of a passage to another
chapter.

Current placement classes are:

- native, with no mapping;
- exact after mapping;
- moved within or across chapters;
- partial, with both present and absent verses;
- entirely in another chapter;
- verses absent;
- incommensurable numbering; and
- book absent from the selected translation.

Book existence is checked against loaded translation data before a versification
table is trusted. A resolver receives the filing record and target translation
as inputs and does not read the current application position. Placement is
derived, never persisted.

## Chapter plan and presentation state

[`buildChapterPlan`](../notes_plan.go) is the single passage derive. It produces
stable ordered sets of placed and unplaced notes, the current tint answer, any
payload notice, session presentation, and a deterministic fingerprint.

The plan is read-only. It does not mutate records, application position, or the
active mark. A single feature gate yields an empty note plan while leaving the
stored collection intact.

`noteFocus` is session-only. Its states are unset, explicitly none, or one note
ID. `planOpenLimit` currently permits at most one expanded received note, and
zero is valid. The limit exists only in the derived view and writes no bytes.
`Minimized` remains the result of an explicit durable action. A non-note mark
suppresses expansion through derivation and restores automatically when that
mark clears.

Received notes form the passage set. A locally composed note is included in the
reading presentation only when explicitly focused; it remains available in the
notes browser at all times. Unplaced notes retain their records and appear with
placement-specific neutral copy rather than disappearing silently.

Every surface consumes the same plan or an explicitly bounded projection of it.
The fingerprint covers every plan field that can change rendered output and is
stable over unchanged ordered inputs.

## Overlap and tint contract

Tint is a value, not a boolean. Each renderer receives the same per-verse tint
answer and never infers coverage from card placement or storage coordinates.
Joining whitespace is tinted only when adjacent verses carry the same tint.
Break characters remain outside the painted wash.

The current implementation flattens one active mark into one `tintHighlight`
span. `tintMulti` and its light/dark palette pair are wired but unreachable
while `planOpenLimit` is one. This preserves current single-band native layout.

Multiple active notes must be implemented as a per-verse set union in Go,
flattened into maximal disjoint ascending runs before any platform renders it.
Only placed, visible, non-minimized notes may contribute. A future focused-note
mode may distinguish the focused coverage from other coverage, but a verse must
still receive exactly one flattened tint value. Color communicates category,
never an exact count; note selection supplies the identities behind an overlap.

No renderer may accept independent layered backgrounds for separate notes.
UIKit/AppKit attributed strings, Android HTML import, and the styled renderer
all have a single effective background per character or run.

## Sender and attribution contract

The current share flow collects no sender name and emits no stable sender
identity. Received-note presentation uses neutral attribution. Reserved sender
fields remain stored but are dormant behind one display gate.

Any later attribution layer must remain additive and optional:

- a sender identifier is untrusted and must never become a note address;
- no identifier may be derived from a device, platform account, contact list,
  or message carrier;
- display names are single-line, length-limited, control-stripped, and bidi
  isolated;
- app impersonation and link-like names fall back to neutral attribution;
- local grouping metadata remains separate from note ownership;
- merge and split operations are explicit and reversible; and
- no local grouping implies verified real-world identity.

Deleting or muting a local grouping must not make its notes unreachable. Note
records remain independently addressable by local ID.

## Invariants

The following properties are architectural:

1. A passage coordinate is never a note identity.
2. Every verb receives the ID of the record represented by its surface.
3. A mark carries its origin and numbering frame.
4. Anchor resolution is set-valued and placement is total.
5. Placement is derived and never stored as authoritative state.
6. `Minimized` changes only through an explicit durable action.
7. View limits and suppression leave zero persistent residue.
8. Unreadable records and unknown fields survive unrelated writes.
9. A whole-store read failure prevents every writer from mutating the store.
10. IDs are monotonic and never reused.
11. Tint runs are disjoint, ordered, and identical across renderers.
12. Color never serves as a note count or record identity.
13. Feature-off behaviour is controlled by one plan gate.
14. Presentation fingerprints include every visible plan dependency.
15. Sender metadata never owns, addresses, or authenticates a note.

Semantic uncertainty fails toward preserving and surfacing data. Storage
uncertainty fails toward refusing mutation. Wire damage and newer semantics are
reported explicitly rather than dropped silently.

## Staging and validation

Changes should preserve a green, shippable tree at each boundary:

1. codec and cross-decoder conformance;
2. record/store shape and migration;
3. set-valued anchor resolution;
4. plan derivation, focus, suppression, and fingerprints;
5. shared tint flattening and surface parity;
6. optional attribution plumbing with the display gate disabled;
7. browser scalability, export, and explicit deletion tools.

Relevant automated coverage includes codec vectors, store guards and migration,
anchor hard cases, state-space enumeration, chapter-plan fingerprints, HTML
goldens, native tint-run conversion, styled geometry, and source-parsed native
spacing constants. Platform release wrappers remain part of validation where
native code cannot compile on the development host.

### Browser cost

The browser's cost is bounded by the VIEWPORT, not by the size of the
scrapbook: the list is windowed, and its rows are pooled objects that are
refilled per note rather than rebuilt. Rebuilding a row per update is the
regression to watch for, because the row carries a `container.ThemeOverride`
and constructing one clears the process-wide font measurement cache — so a
per-update rebuild re-shapes every string in every visible row on every layout
pass. [`notes_browse_bench_test.go`](../notes_browse_bench_test.go) measures
open, keystroke, scroll, and single-row cost at 100, 500 and 2,000 notes;
opening and laying out the list should stay in the tens of milliseconds and
should not scale with the stored count.

## Sticker spacing

[`noteMetrics`](../notes_bubble.go) is the shared spacing table. Values use
Apple points, Android density-independent pixels, and styled-layout units.

| Metric | Value |
|---|---:|
| `GapAbove` | 10 |
| `GapBelow` | 10 |
| `Pad` | 12 |
| `WhoH` | `ceil(fontSize × 1.27)` |
| `WhoGap` | 4 |
| `TailDepth` | 9 |
| `TailWidth` | 18 |
| `TailInset` | 24 |
| `Radius` | 10 |
| `PillH` | 28 |
| `PillPadX` | 14 |
| `PillMinW` | 86 |

The drawn shape owns the gap below; for an expanded card that means the tail
apex, and for a pill it means the pill bottom. macOS may reserve more than ten
points above through `max(GapAbove, lineHeight)` to accommodate TextKit line
geometry. Paragraph spacing outside the reserved band belongs to the reading
layout, and the RESERVATION never touches it. Body font size and platform
control hit areas are also outside this table.

At PLACEMENT the collapsed pill stack centres in the inter-paragraph air a
reader SEES — the previous paragraph's ink bottom to the noted paragraph's
first ink top (`notes_bubble.go` states the rule and its exemptions). Panes
whose engines split leading evenly around the glyphs implement that as box
arithmetic (`notePillSeparatorLift`, separator/2 above the band top); the
natives measure the ink off the live layout, because their imports pile the
leading above each line's glyphs and the box answer sat visibly low. The
expanded card never centres — its tail's distance to the passage is the
pinned `GapBelow` — and neither does a stack whose bottom neighbour is an
open card, nor any chapter-top tenancy (no separator there). Where the
separator is zero (the reporter layouts) every form stands down and the pill
sits `GapAbove` into its band exactly as the table reads.

`notes_spacing_spec_test.go` parses the native source constants and checks them
against the table. Styled-layout tests additionally validate computed geometry.

## Future work

- Prove Android native-bridge initialization order before making the Fyne
  fallback depend on bridge availability.
- Lift `planOpenLimit` only with set-valued focus, flattened multi-note tint
  runs, selection for every contributing note, and native multi-band layout.
- Keep attribution disabled until its privacy, spoofing, merge, and split rules
  are implemented across every surface.
- Define locally composed note editing and versioning without changing stable
  record identity.
- Evaluate placing the sticker below its anchor paragraph. Such a change must
  reserve space after the paragraph, invert the tail, and revise scroll framing.
- Resolve styled-pane cursor behaviour, over-height narrow cards, and a second
  anchor band before enabling multiple open stickers there.
- Test emitted markup separately from importer-rendered wash geometry,
  especially around poetry and break characters.
- Add export, filtered deletion, and a non-modal large-store notice without
  introducing automatic eviction.
