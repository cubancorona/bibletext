# Translation footnotes — investigation

*Branch `footnotes`, 26 August 2026. Four research lanes (data, surfaces,
licensing, presentation) over the live helloao corpus, the current codebase,
API.Bible's terms, and print/app prior art. This is an investigation, not a
build: the only code change on this branch is a defensive decoder guard.*

## The question, and the standard it must meet

Should the app show the translators' footnotes — and can it, without
violating the rule the owner set from the closing lines of Revelation
(22:18–19): nothing may be added to the words?

The app has answered the underlying question once already. Shared notes put
human words *beside* Scripture by making them **visibly human and ignorable**
— chrome type instead of the scripture serif, attributed, dismissible, never
inside the text. Translators' footnotes have, if anything, a stronger claim
to that treatment: "NU-Text omits *who is in heaven*" is not an addition to
the words but testimony *about* them — the translators showing their work,
as the apparatus has stood in the margins of printed Bibles for centuries.
But the separation must be **mechanical, not merely typographic**. The hard
requirement this document designs against:

> Nothing a footnote says may ever enter the verse text, the copied
> selection, the share card or citation, the spoken audio, the search index,
> or a shared link. Purity by construction, pinned by tests that can fail.

## Verdict

**Feasible, tractable, and worth doing in two quiet steps — with the NKJV's
footnotes licence-gated behind the pending API.Bible questions.** The data is
already arriving and being thrown away; the architecture that keeps the text
pure already exists in the codebase (the red-letter side table); and the
presentation that honours the owner's rule is the one the notes feature
already established.

## 1. What the data holds (measured, not assumed)

Fetched and analysed the complete corpus for all three helloao translations:

| | Footnote bodies | Median/chapter | Max chapter | Apparatus size |
|---|---|---|---|---|
| BSB | 4,853 | 3 | Ezekiel 40 (25) | 245 KB |
| WEB | 1,226 | 0 | Matthew 5 (12) | 80 KB |
| WEBC | 1,673 | 0.5 | 2 Maccabees 12 (26) | 96 KB |

- **Types** (classified, 40 hand-read + keyword pass over all): alternate
  renderings (BSB 35.5%), manuscript/textual variants (18.8%), Hebrew/Greek
  language notes (18.6%), cross-references (10% — BSB only has a real
  citation apparatus), measures/money (5.7%). **Zero notes read as
  Scripture; all are translator commentary.**
- **Integrity is exact.** Every in-verse `{"noteId":N}` marker has a body and
  every reference matches its enclosing chapter/verse — zero orphans, zero
  mismatches, no mid-word splits. The decoder (`bsb.go`) drops the markers
  today and has purpose-built spacing logic healing the seams; the real
  captured shapes are already pinned in `bsb_test.go`.
- **Two data decisions to make up front:**
  - 42 bodies (36 BSB, 3 WEB, 3 WEBC) anchor inside Psalm **superscriptions**
    (`hebrew_subtitle` nodes) the app doesn't render. Drop them until
    superscriptions render, or re-anchor to verse 1 — recommend **drop**, and
    revisit with superscriptions as their own question.
  - 351 BSB markers sit **between poem lines** (the Psalm 23:1 shape): anchor
    offsets must land at end-of-line, before the `"\n"` the decoder
    synthesizes — and offsets must be computed against the FINAL tidied text
    (`bsbTidySpacing` invalidates join-time positions; this is the one
    genuinely fiddly piece of the decode work).
- The deuterocanon is the note-densest region of the WEB corpus (2 Maccabees
  has notes in every chapter) — this is not a protocanon-only feature for
  WEBC.
- ⚠ Side finding, separate issue: WEB/WEBC `complete.json` has grown to
  **~47 MB** (Strong's word data) against the fetcher's 120 s timeout. Worth
  its own follow-up regardless of footnotes.

## 2. The architecture decision everything pivots on

**Footnotes ride BESIDE `Verse.Text`, never inside it.** `Verse.Text` is
consumed raw by search indexing, TTS (whose read-along offsets mirror it
character-for-character), the share pipeline's positional ground truth
(`chapterProse`), link-unfurl previews, verse-of-day, and every snippet
surface. Bake a marker in and every one of those leaks apparatus into
Scripture and every offset system shifts. Kept side-band, **all of them are
untouched by construction.**

The codebase already owns this exact pattern: red letters are a per-edition
side table returning rune offsets into untouched verse text
(`red_letter_runs.go`). Footnotes want the same shape —
`footnoteMarksFor(version, book, chapter, verse)` → anchors + bodies in a
parallel store, captured at decode time where the markers are discarded
today. `Verse` gains an `omitempty` field; the cache envelope absorbs it
compatibly; cacheEpoch bumps: web 2→3, bsb 3→4, webc 2→3.

Two leak points still need explicit work + tests, because they receive text
from the *rendered* panes rather than from `Verse.Text`:
- **Copied/shared selections** from native panes will carry marker glyphs
  exactly as they carry verse-number digits today. The share pipeline's own
  documented rule — "verse-number markers are apparatus, not scripture:
  stripped no matter what" — extends to footnote markers via the same strip
  sets (`cleanQuoteText`, `plainSelection`/`superToDigit`), and must, or
  positional location fails and citations silently degrade.
- **AI prompts**: the selected passage travels to the reader's AI provider —
  markers must be stripped; whether note *bodies* should optionally travel
  with an Explain request is an owner question, not assumed.

## 3. Presentation (prior art + this app's character)

Print settles the grammar: verse numbers and note markers live in **two
registers** — and every major app (YouVersion, ESV, Literal Word,
BibleGateway) converges on tap-to-peek from a small marker, most with an
off switch. For this app, ranked:

1. **Step one — no markers at all: a per-chapter "Translation notes" sheet.**
   A quiet chapter-header affordance (peer of Listen) opens a sheet listing
   the chapter's notes keyed by verse — "3:16 — Or *his only Son*". The page
   itself never changes: **text purity is perfect, the Revelation concern
   fully dissolved**, and the surface is a plain cross-platform sheet in the
   notes-browser mould — cheapest possible build, identical everywhere.
   Weakness: the reader can't see which word carries a note (rows can bold
   the annotated word to compensate).
2. **Step two, opt-in refinement — one quiet marker glyph, tap-to-peek.**
   A single repeated muted glyph (the BSB's own `+`/asterisk convention,
   ~0.6em, `pal.TextMuted` — visually *subordinate* to the semibold verse
   numbers), after the annotated word. Tap opens the same note in a card
   plainly labelled "Translators' footnote — BSB", set in chrome type, not
   the Georgia scripture serif — the same visible-humanity move the notes
   bubble makes. One fixed glyph, never letters or numerals: trivially
   strippable everywhere, one consistent VoiceOver word, and it cannot
   collide with the verse-number scanners (see §4).
3. Margin dots — ranked last; poor discoverability for no purity gain over
   step two.

**Off by default**, one checkbox in the Settings READING card ("Show the
translators' footnotes"), with a below-card footnote in the house style:
*"Notes from the translators about wording and manuscripts. They are not
part of the Scripture text."* Red letter defaults on because it colours the
words themselves and adds nothing; a marker is an added glyph between words —
a categorically stronger intrusion, so the default flips. The toggle rides
the exact plumbing red letter already uses (`refreshReadingOnly`,
`chapterRenderFingerprint` gains the flag).

## 4. Surface-by-surface effort

| Surface | Effort | The crux |
|---|---|---|
| Decode + side-band store + shared marker inserter | **M** | Prerequisite for everything; anchors vs `bsbTidySpacing`; composes with red-letter runs |
| Apple HTML pane (iOS + macOS, one builder) | **M** | CSS + inserter; marker excluded from the font-size verse-number scans; tap via native link interaction |
| Android native overlay | **L** | **Concrete breakage found**: `BtBridge.buildVerseIndex` lets ANY `<sup>` terminate the preceding verse's span — a footnote `<sup>` truncates read-along tint, selection attribution and scroll anchors. Fix the scan (or use a non-sup span) FIRST; tap needs a new gesture path; body sheet can follow the note-bubble push pattern |
| Windows/Linux styled pane | **M/L** | New runKind through layout/draw/hit-test + popover; fully host-testable |
| Fyne fallback panes | **S** | Distinct marker where possible; documented-gap escape hatch exists |
| Web reader | **S/M** | Friendliest surface: `.fn` rule, `:target`/popover, palette-parity test |
| Verse-of-day, search cards, cross-ref snippets, share-image, previews | **S** | Nothing beyond leak-pinning tests (clean by construction) |
| Share/AI apparatus strip | **M** | One shared normalize step + sweep tests |
| Settings toggle + fingerprint | **S** | Rides red-letter plumbing |

**Test slate** (the lockstep instruments to extend, by name):
`reading_poetry_test.go` (marker-bearing poetry across every dialect, incl.
the real Psalm 23:1 marker-at-poem-break), the five-renderer
`chapter_tint_golden_test.go` (marker-inside-wash, marker-inside-red-run),
`reading_fingerprint_test.go`, `share_partial/cut_sweep/positional_test.go`,
`audio_test.go` + the NKJV speech-text check, styled-pane layout/select
tests, websitegen render + palette parity, and new native-scan guards
pinning that a marker never enters `gVerseIndex`/BtBridge verse spans.

## 5. The NKJV: technically cheap, licence-gated

- Technically: notes ride the SAME 197-call canon walk — flipping
  `include-notes` in the one shared query const costs **zero extra quota**.
  The decoder then needs a `note`-node case diverting subtrees into footnote
  records (plus `Caller` in the parsed attrs, and cacheEpoch 0→1 for nkjv).
  Cached footnotes automatically ride the existing §11 30-day refresh and
  §10 purge — no new compliance code.
- **Landed on this branch already**: the walk's default case would have
  APPENDED a populated note's text into verse text if the server ever sent
  notes despite the flag — apparatus read as Scripture, the exact forbidden
  failure. The decoder now skips `note` nodes explicitly, pinned by
  `TestDecodeAPIBibleEdgeSkipsPopulatedNoteNodes`. This guard is worth
  having whether or not footnotes ever ship.
- **Unverified** (deliberately): the live shape of NKJV notes. The key now
  lives in the login Keychain (`uk.co.bibletext.apibible-release`), not
  `.env.local`, and this session's permission layer rightly refused to
  extract it. One owner-run call captures the ground truth:

  ```
  KEY="$(security find-generic-password -a release -s uk.co.bibletext.apibible-release -w)"; curl -sS -H "api-key: $KEY" 'https://rest.api.bible/v1/bibles/63097d2a0a2f7db3-01/chapters/JHN.3?content-type=json&include-notes=true' > /tmp/jhn3-notes.json
  ```

  (John 3 because the print NKJV carries NU-Text notes at 3:13 and a *Lit.*
  note at 3:20.)
- **Licence**: nothing public says whether the Starter grant covers
  *displaying* Thomas Nelson's apparatus, whether selective display
  (markers in text, bodies in a labelled panel, off by default) satisfies
  the no-alteration/format clauses, or whether §12 copy limits and the §9.1
  TTS restriction treat note text like verse text. **Six concrete questions
  are ready to add to the still-unsent support@api.bible draft**
  (r8986143052262947400) — sending that email is the gating action.

## 6. Recommended phasing

1. **Phase 0 (decisions)**: side-band model confirmed; drop
   superscription-anchored bodies for now; helloao translations only.
2. **Phase 1**: decode + store + the per-chapter "Translation notes" sheet,
   OFF by default. No markers anywhere; every purity property holds by
   construction; one new sheet, identical on all platforms.
3. **Phase 2 (opt-in, later)**: the quiet marker + tap-to-peek, surface by
   surface — web reader first, Apple panes next, styled pane, Android last
   (after the BtBridge scan fix).
4. **NKJV notes**: only after the API.Bible answers arrive, and after the
   owner's one-call probe confirms the feed carries them.

## 7. Open questions for the owner

1. Does Phase 1 (sheet only, no markers, off by default) meet the standard —
   or is even a labelled sheet of translators' notes unwanted?
2. Superscription-anchored notes: drop (recommended) or re-anchor to v1?
3. Should note bodies ever travel with an AI Explain request? (Default: no.)
4. Send the API.Bible email with the six footnote questions added?
