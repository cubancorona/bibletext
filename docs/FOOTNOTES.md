# Translation footnotes — investigation

*Branch `footnotes`, 26 August 2026. Four research lanes (data, surfaces,
licensing, presentation) over the live helloao corpus, the current codebase,
API.Bible's terms, and print/app prior art. Sections 1–7 are the original
investigation; §8 records what the machinery build and the live NKJV probe
then established.*

## The question, and the standard it must meet

Should the app show the translators' footnotes — and can it, without
violating the standard this project holds from the closing lines of Revelation
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
presentation that honours that standard is the one the notes feature
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
  with an Explain request is an open decision, not assumed.

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
  extract it. One manually-run call captures the ground truth:

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
   one-call probe confirms the feed carries them.

## 7. Open decisions

1. Does Phase 1 (sheet only, no markers, off by default) meet the standard —
   or is even a labelled sheet of translators' notes unwanted?
2. Superscription-anchored notes: drop (recommended) or re-anchor to v1?
3. Should note bodies ever travel with an AI Explain request? (Default: no.)
4. Send the API.Bible email with the six footnote questions added?

---

## 8. Machinery status (capture built ahead of any presentation; NO rendering)

The capture machinery is **built, humming and tested** on this branch, under
the strict rule that nothing renders anywhere:

- `Verse` carries `Footnotes []Footnote` (`anchor` = rune offset into the
  final text, `text`, `kind`, `caller`) — side-band, `omitempty`, riding the
  cache envelope; old caches load unchanged; epochs bumped (web 3, bsb 4,
  webc 3, nkjv 1) so installs refetch and gain the apparatus.
- **helloao**: `bsbVerseTextMarked` shares the byte-identical text path
  (same pieces, same join, same tidy) and computes each anchor by tidying the
  piece-prefix — no sentinel ever enters that pipeline. Bodies join by
  noteId; superscription-anchored bodies drop as decided.
- **NKJV**: `include-notes=true` (same requests, zero extra quota); the walk
  diverts note subtrees into the side-band store via a private-use sentinel
  resolved to anchors after normalization, with origin spans (`xo`/`fr`)
  stripped and `ref` text flattened. The purity guard stands: a note's words
  can never reach a verse builder.
- **Proven against the real world**, not fixtures alone: whole-corpus decode
  of all three helloao translations — BSB captures **4,817/4,817** in-verse
  notes, WEB 1,218, WEBC 1,641, zero bad anchors, zero sentinel leaks, zero
  body text inside Scripture; and a live NKJV John 3 differential — verse
  text **byte-identical** with notes on, 45 cross-references captured.

### Live NKJV probe — a decisive correction to §5

**The NKJV feed carries no translator footnotes at all.** John 3, Psalm 46
and 1 John 5 (home of the NU/M-Text note on the Johannine comma) return only
`style:"x"` cross-references — zero `f` notes, zero "NU-Text" anywhere.
Thomas Nelson's textual apparatus is not in the feed. Consequences: the
hardest §5 licensing question (displaying their *footnotes*) is moot for
now; what is capturable — and now captured, tagged `kind:"crossref"` — is a
cross-reference apparatus, a kind the app already offers from public-domain
TSK. Whether NKJV crossrefs ever display is a presentation question for
later; several §5 questions (do cached notes ride §11/§10; §12 copy limits)
still belong in the support email.

### The omitted verses — the investigation's most striking find

34 notes (5 WEB protocanon + 29 WEBC) anchor in verses that have **no words
at all**: Luke 17:36, Acts 8:37, Acts 15:34, Acts 24:7, Romans 16:25 and 24
deuterocanon versification gaps — the critical-text omissions, where the
footnote exists precisely to explain the verse's absence ("Some Greek copies
add…"). The app has never rendered these empty verses, so the machinery
drops their notes (pinned by test). Surfacing them would mean rendering
empty verse numbers — arguably the most pastorally valuable footnotes in the
corpus, and squarely a presentation decision. **Added to the open questions.**

### Open questions (superseding §7)

1. Approve Phase 1 presentation (the per-chapter sheet) when the time comes?
2. The omitted verses: should an empty verse number one day appear with its
   explanatory note, or stay silent as today?
3. NKJV crossrefs: display eventually, or leave captured-but-dark?
4. AI prompts: may note bodies travel with an Explain request? (Default no.)
5. Send the support email (its footnote questions now narrower — §8 above).

### Decision (2026-08-26): no dev tab — presentation trials happen on the main reading pane

A dev-only tab duplicating the reading view was considered as a presentation
laboratory and declined. Two read-only code scouts established that the reading
view is single-instance by design on every platform (native overlay singletons,
the currentHost frame guard, the addRecentChapter funnel into audio-stop and
reading.state, and the one-pane scroll/anchor registries), so a live duplicate
would break the real tab; a stripped passive renderer was safe but could not
show the production rendering. When presentation work is approved, footnote
display will be built and tested directly on the main reading pane in this
branch — no second surface.

## 9. Prototype (2026-08-27): the chapter-bottom section, live on the Apple panes

The adopted presentation is modelled on the Supreme Court slip opinions
(measured from *Medina v. Planned Parenthood*, 606 U.S. 357: a short
flush-left hairline rule, notes in the body serif at ~0.8× set tighter,
justified): after the last verse, air → a short muted solid rule (an underline over a no-break-space run — drawn by the text system, so continuous on both importers where every glyph run gaps on iOS) → the
chapter's translator footnotes at 0.85em in `TextMuted`, each keyed by its
semibold verse number. **No in-text markers** — Scripture above the rule is
byte-identical with the section on or off. Crossrefs (the NKJV feed) stay
dark. Off by default; toggled by a header icon (miniature of the section
itself, present only when the chapter has notes) and a READING-card checkbox
with the §3 caption.

Mechanics (footnote_section.go + buildChapterHTML + the native panes):

- **0.85em is a load-bearing pact.** The Apple verse scans only capture runs
  below 0.8× the largest font, so the section's digit-leading verse keys can
  never become phantom verses; and the native content-end detectors
  (`btIOSFindContentEnd` / `btMacFindContentEnd`) find the scripture/apparatus
  boundary as the first run in the [0.8, 0.95) font band — no sentinel string
  to collide with Scripture. `TestFootnoteSectionNativeContract` alarms drift.
- The boundary bounds the LAST verse's read-along/highlight ranges (they used
  to end at ts.length) and clamps the selection verbs: apparatus keeps the
  system Copy/Look Up but can never enter Share/AI/citation, and a straddling
  selection is cut at the rule. The one chosen, stated property: a reader CAN
  deliberately select and plain-copy the visibly-labelled section, exactly as
  they can copy verse-number digits today.
- The toggle rides the red-letter plumbing: pref `reading.footnotes`,
  `fn` slot in chapterFingerprint (moves BOTH body and render fingerprints),
  `refreshReadingOnly`, scroll preserved by the same-chapter restore capture.
- Surfaces: Apple only (one shared builder → macOS + iOS). Android
  (sentinel-`<sup>` recipe) and the styled pane (geometry-only, note-sticker
  mould) are documented in the scout results but not built. Fyne fallbacks
  stay documented gaps.

Sim-verified on iPhone 17 Pro (BSB Psalm 23): section renders, toggle
round-trips with scroll preserved, header button reflects state. The
omitted-verses question (§8) stands: this is the first presentation that
could carry those 34 notes, but they are dropped at decode today.

Two hardenings the first cut needed: (1) the plain-text HTML-import-failure
fallback used to carry the whole section into the untyped string with the
boundary disarmed — both panes now cut the HTML at `<p class="fnsep">` before
stripping tags, so the apparatus never enters the fallback at all (pinned in
TestFootnoteSectionNativeContract); (2) a toggle beside full-screen in the
mobile header widened the right column into the expanded audio card's
permanently reserved centre footprint (overlapping on 375pt phones with long
book names) — the stacked-under-full-screen placement in pinned 36pt cells is
the recorded mount recipe.

Icon + settings placement (design round, 2026-08-27): the
toggle's glyph is now **αω¹** — alpha and omega carrying a raised footnote
numeral in the word-processor insert-footnote lockup ("I am the Alpha and the
Omega", Rev 1:8 — the placeholder text is His letters). The glyph is adopted, chosen over three earlier families (miniature-layout, asterisk/dagger
marks, Latin ab¹/a¹ lockups) across four rendered comparison sheets; baked
from font outlines into a single-fill SVG (assets/icons/footnote.svg). The
setting moved OUT of the READING card to its own TRANSLATORS' FOOTNOTES card
at the VERY TOP of Settings (icon + checkbox + the §3 caption) — on screen
the moment the sheet opens while the feature is under evaluation.

Control simplified (same day): NO toggle in the reading-pane
chrome after all — the TRANSLATORS' FOOTNOTES card at the top of Settings is
the feature's ONE control (a plain checkbox; no icon in the row either). The
approved αω¹ glyph stays RESERVED in assets/icons/footnote.svg + iconFootnote
for when a header toggle returns; the header-mount recipe (availability gate,
stacked placement on mobile) is recorded in footnote_section.go.

## 10. Omitted-verse orphans + all-platform rendering (2026-08-27)

**Orphan capture.** The 34 notes anchored in critical-text-omitted verses are
no longer dropped: a verse node that carries a marker but no text (the Luke
17:36 shape) now yields an OrphanFootnote into BibleData.OrphanFootnotes
(book → chapter), corpus-verified at exactly WEB 5 (Luke 17:36; Acts 8:37,
15:34, 24:7; Rom 16:25) + WEBC 29 (those five + 24 in Sirach's versification
gaps) + BSB 0. Superscription-anchored bodies stay dropped — capture reads
only markers inside emitted-but-textless verse nodes, never unconsumed
bodies by reference. The TEXT is untouched: no empty verse numbers appear on
the page; the section carries the explanation instead, keyed by the absent
verse number and sorted into place between its neighbours
(chapterFootnoteEntries merge). Epochs: web 3→4, bsb 4→5 (decoder-version
hygiene; zero BSB bytes change), webc 3→4; nkjv stays 1 (the NKJV prints
these verses). Old caches load with a nil table; every accessor is nil-safe
for the superseded-epoch offline fallback. Purity is structural — nothing
but the section reads the table — and pinned in the search/speech/prose/copy
sweep. Sim-verified live: WEB Luke 17 renders "…35 … 37…" exactly as
printed, with "36 Some Greek manuscripts add: …" below the rule.

**Android.** buildChapterHTMLAndroid appends the section in the fromHtml
dialect: it OPENS with one sentinel `<sup>&#160;</sup>` — BtBridge's
buildVerseIndex ends the last verse's span at any SuperscriptSpan and skips
a non-digit one, so the sentinel bounds read-along tint, washes and scroll
anchors while rendering as invisible raised whitespace — then the same
underline-over-nbsp hairline (fromHtml maps <u> to a text-system-drawn
UnderlineSpan), then <small><font><b> verse keys (NEVER <sup>: a
digit-leading sup would index as a phantom verse). BtBridge records the
sentinel's start as contentEnd and clamps the selection verbs to it at
click time, the Android twin of the Apple content-end clamps.

**Styled pane (Windows/Linux).** Geometry-only, in the note-sticker's mould
(reading_styled_footnotes.go): the section never enters lay.Text, so
select-all/copy/share/verse-attribution exclude it BY CONSTRUCTION; the four
press handlers guard it like the sticker; MinSize carries its height so the
scroll machinery follows; it re-wraps inside relayout with the chapter.
Verse keys draw in the verse-number colour, bodies in TextMuted, at 0.85×
in the scripture serif; the rule is a 1px rectangle.

footnoteSectionSupported now covers darwin/ios/android/windows/linux; the
Fyne fallback panes remain documented gaps unreachable in shipping builds.
