# What we add, and what we drop

An accounting of the text against the standard in docs/SCRIPTURE_WORKLIST.md:
nothing dropped without an explicit decision, nothing added at all. It exists
so that each item can be decided rather than defended, and it is the input to
the worklist, not a substitute for it.

Two cautions about the figures. Counts drawn from the raw captures, the decoded
caches and a full NKJV download are measured; the rest are marked "not
measured" and should be treated as unknown rather than small. And a handful of
the sharpest claims were re-checked directly against the code: that the Share
verb cleans a selection while the AI path does not, that the website carries no
copy-cleaning at all, and that verse numbers are inside the selectable text.
Those three hold.

# Accounting: what BibleText adds and drops against its own publishers

Standard applied throughout: nothing is dropped unless explicitly agreed, and nothing is added at all. Four editions are in scope — WEB, WEB Catholic, BSB (decoded from helloao) and NKJV (decoded from API.Bible) — plus the generated red-letter tables, the ten downstream pipelines (search, speech, share text/image, copy verbs, shared links, AI, cross-references, verse-of-day, the website), and the five rendering surfaces. Counts are drawn from direct measurement against the decoded canons and cached captures wherever the source material states this; where a pass could not establish a count, it is reported below as "not measured" rather than estimated.

## 1. Verdict

Measured, reader-facing drops are substantial and mostly concentrated in a small number of root causes: the BSB loses all 3,091 of its publisher's chapter headings (72,690 characters) outright, with no side-channel to recover them from; the NKJV drops an estimated but unmeasured ~3,300 section headings and cross-reference lines (including some, mr/sr/r, treated worse than an ordinary footnote); the WEB and WEB Catholic lose nearly all stanza structure in Psalms, Song of Solomon and Wisdom; and Psalm superscriptions, though retained internally, fail to reach seven of the app's own surfaces (search, speech, sharing, the dedicated chapter-copy verb, the website, cross-references, and nine of every 371 verse-of-day rotations). The single worst drop is the BSB's chapter headings, because unlike every other omission audited here, it is not merely unshown — the text never enters the data model at all, so there is nothing left to change one's mind about later. On the addition side, every one of roughly 31,000 verses per edition carries a manufactured verse-number-and-joiner character pair, and that pair — along with a literal paragraph-indent character on Apple platforms and the reader's own uncleaned selection — escapes uncleaned into native OS copy/paste on iOS, macOS and Android, into every browser copy from the public website (which has no cleaning code at all), into the Windows/Linux desktop's own Copy command, and into what is transmitted to the external AI provider. The single worst addition is this unclamped, largely undocumented set of escape channels, because it is the one place where app-manufactured characters leave the app disguised as the publisher's own words, with no recorded decision anywhere that this should happen.

## 2. What we add

Most severe first. "Escapes the page" means the added material can leave the reading pane as copied, shared, spoken, indexed, or linked text — as opposed to staying purely visual.

| Addition | How many | Escapes the page? |
|---|---|---|
| Verse-number digit + its joiner (NBSP on three dialects, a plain space on the fourth) drawn into the text stream | ~31,000+ verses per edition, universal | Yes — native OS copy (iOS/macOS/Android, unfiltered), desktop Copy (Windows/Linux, unfiltered), website copy (no cleaning exists), and the AI prompt (unfiltered). Not via the app's own Share verb, which is cleaned. |
| Reader's raw, uncleaned selection (digits, NBSP, paragraph indent) sent to the external AI provider; poem-line breaks corrupted to literal `\n` text by Go's `%q` string-quoting | Every "Study with AI"/"Ask" call touching a verse number (effectively all) or a paragraph boundary | Yes — transmitted to a third-party AI service; the on-screen preview shown to the reader is cleaner than what is actually sent |
| Website has no copy-cleaning code of any kind | Every verse on every published chapter page | Yes — every browser-native copy |
| Reporter-layout paragraph indent written as literal em-space + en-space characters (not CSS) on Apple/Android | ~8,500+ paragraph-opening verses per edition (NKJV: 8,534) | Yes on Apple platforms, by the code's own admission — untested claim of "clean" does not hold for this indent |
| NKJV small-caps/divine-name uppercasing writes literal capital letters not present in the feed's own casing | LORD 5,622 verses, GOD 298, JESUS 6, plus 30 further verses (Exodus 3:14, John 8:58, YAH, BRANCH, Daniel 5, Deuteronomy 28:58, Gospel inscriptions) | Yes — baked into stored Verse.Text, so searchable, shareable, copyable, spoken and linked |
| Windows/Linux styled-pane paints a whole wrap-token red if any one rune in it is red, bleeding colour onto adjacent narrator text | 48 verses / 82 runes across all four editions (documentation claims only 2 verses / 3 runes) | No — visual only on that one pane, does not survive copy |
| Footnote-section separator (16 no-break spaces) and manufactured entry-key labels ("16"/"Title" + a joiner) | Fires once per chapter render with footnotes on and notes present: BSB 4,817 / WEB 1,218 / WEBC 1,641 non-cross-reference notes; 0 for NKJV | Yes, via native unclamped system Copy/Select-All, contradicting the section's own claim that nothing here ever reaches Scripture text |
| Bluebook citation apparatus (bracketed capital, four-dot ellipsis, added/rebalanced quotation marks) manufactured on a partial-verse share | Routine — any share that doesn't begin/end on a sentence boundary | Yes, by definition — it is inside the shared message or image |
| Manufactured join characters: a plain space between decode-time text pieces (BSB/WEB/WEBC), a synthesized poem-line newline before poetry runs, an NKJV prose-boundary space and poetry-boundary newline, and a render-time/pipeline-time inter-verse join space | Decode-time: ~30,671 verses across the three helloao editions differ from a pure concatenation of the feed's own strings; NKJV poetry-boundary newlines: 8,349 verses / 16,666 characters (exact, test-pinned); render/pipeline joins: ~29,913 boundary points in the NKJV alone | Yes — baked into Verse.Text (decode-time) or into every rendered/derived text stream (render-time) |
| Descriptive-run text (Psalm 119's Hebrew letter names, one Zechariah oracle title) kept undifferentiated inside ordinary verse text | WEB 21, WEB Catholic 21, BSB 1 | Yes — fully inside Verse.Text: searchable, shareable, spoken as if translated Scripture |
| Whole-verse red-letter fallback when a table's stored hash goes stale; hand-mapped WEB/WEBC red-letter boundary "recoveries" at 7 verses | 0 instances of the fallback firing today (proven capable of failing); 7 verses / 13 recovered entries, verified faithful to the publisher's actual marking today | Visual only; both are defensive/reconstructive mechanisms currently exercising no real drift |

**The escape-channel additions (rows 1–3).** Every reading surface draws the verse number and a joining space or non-breaking space directly into the same text stream the reader can select. Three of the four ways that selection can leave the app — the operating system's own Copy/Select-All on iOS, macOS and Android; the Windows/Linux desktop pane's own Copy command; and the public website, which has no cleaning code whatsoever — carry that manufactured content through unfiltered, and a fourth, the "Study with AI" pipeline, sends the identical uncleaned selection to an external service, with poem-line structure additionally corrupted into literal `\n` text. Only the app's own Share verb is cleaned. None of this is discussed anywhere as a decision; the code's own rationale for leaving system Copy unclamped addresses a different problem entirely (stopping the footnote apparatus from being misattributed as Scripture), not this one.

**The reporter-layout indent.** Because the HTML importer on Apple's reading pane drops CSS `text-indent`, the app substitutes two literal Unicode space characters at the front of every paragraph-opening prose verse. A code comment admits this "carries" into copied text on Apple platforms; neither audit document mentions it at all.

**The NKJV uppercasing.** Divine-name and small-caps conventions are collapsed into one identical uppercase treatment, and the owner has reviewed and accepted this specifically for the four verses where Jesus is named ("Matthew 1:21," etc.). Thirty further verses carrying the same mechanism — including Exodus 3:14 and John 8:58's paired "I AM," and a full multi-word phrase in Deuteronomy 28:58 — have never been surfaced for the same review.

## 3. What we drop

Most severe first, same escape/visibility framing in reverse (what a reader is denied).

| Drop | How many | Where it's missing |
|---|---|---|
| BSB chapter headings — entire node type unhandled, no side-channel exists | 3,091 headings, 72,690 characters, every BSB chapter | Everywhere — not stored anywhere in the data model |
| NKJV section headings, major-section headings, and section/parallel-passage cross-reference lines (s, ms, mr, sr, r, sp, cl, cd) — whole blocks including any nested note | Estimated ~3,300 canon-wide (not measured exactly; extrapolated from 814 in 174 available NT chapters) | Everywhere — no side-channel; a note inside one of these blocks is lost with less trace than an ordinary footnote |
| WEB/WEB Catholic stanza (`\b`) structure in Psalms, Song of Solomon, Wisdom | Psalms: 1 table entry + 95 feed breaks across 2,461 verses; Song of Solomon: 5 table entries + 17 feed breaks across 117 verses; Wisdom: 0 table entries at all | The reading pane itself — these books read as near-unbroken blocks |
| Psalm/superscription titles (retained in the data model) absent from downstream surfaces | 116–117 titles per edition, across: search index, TTS, share text/image, the dedicated "Copy chapter" verb, the website, the cross-reference panel, and 9 of 371 verse-of-day rotation entries | Seven separate surfaces, each independently |
| Translators' footnotes and NKJV's cross-references withheld from downstream pipelines | WEB 1,226 / WEB Catholic 1,673 / BSB 4,853 / NKJV 32,473 notes | 9 of 10 downstream pipelines by explicit, tested design; the website (10th) is open, not fully deliberate |
| Red letter (words-of-Christ distinction) absent from secondary surfaces | Up to ~2,059 marked verses per edition, on: search cards/AI-find results, verse-of-day (62 of 371 rotation days), cross-reference snippets, the AI panel's quoted passage, share text, share-as-image | Six surfaces; only the two reading panes and the shared web link preserve it |
| Poem-line indent depth (level 1/2/3) discarded everywhere | BSB/WEB/WEB Catholic: tens of thousands of poetry clauses; NKJV: 8,349 verses carrying a poem-line break | Every surface — all poetry renders flush left regardless of source depth |
| Omitted-verse orphan notes never marked as a gap outside the reading pane's own toggle | 34 chapters canon-wide | All 10 downstream pipelines |
| Authored poem-line breaks flattened to a plain space | Every poetic verse rendered in a one-line context (thousands per edition) | Search index/cards, AI-find results, website link-unfurl previews, cross-reference snippets (also truncated to 90 runes) |
| NKJV italicised "supplied words" (char style "it") flattened to plain text | 2,244 spans / 1,661 verses counted in the New Testament alone; no Old Testament or canon-wide figure exists | Every surface |
| NKJV "Selah" flattened, joined to the end of the previous poem line | 74 verses | Every surface; matches an already-accepted pattern for WEB/BSB, but never separately reviewed for NKJV |
| TTS never signals a paragraph or stanza boundary | Structural — affects every multi-paragraph chapter | Spoken audio only |
| Notes nested inside NKJV's skipped heading/reference blocks | Not measured — unknown quantity, a recommended probe was never run | Everywhere, with zero trace, not even the side-channel other dropped notes get |
| NKJV wire attributes never unmarshalled (Strong's numbers, altnumber/pubnumber print-verse-number variants, and others) | Not measured — no raw NKJV capture exists locally to check presence | Everywhere |
| Text or a note arriving before an NKJV chapter's first verse marker | Not measured — never probed | Everywhere |
| Bibliographic/structural metadata (translation block, book name/commonName/title fields, chapter-wrapper audio-link/verse-count fields, the verse marker's own printed-number child) | 1 translation block, 66–73 books × 5 fields, ~1,200+ chapter wrappers, per edition | No reader-facing surface — redundant with the app's own canon tables and self-hosted audio |

**Psalm titles.** The text is stored and displayed on the reading pane, yet it independently fails to reach search, speech, sharing, the dedicated chapter-copy command, the website, cross-references, and one in forty-one verse-of-day rotations. No single decision produced this pattern — each surface simply never called the existing accessor.

**BSB headings and NKJV headings/cross-references.** These are the two places where the publisher's own structural apparatus for guiding a reader through a chapter is removed completely, in every instance, with no toggle and, for the BSB, no possible way to recover the text later because it was never captured.

**WEB/WEB Catholic stanza breaks.** For the books most defined by poetic structure — Psalms, Song of Solomon, and especially Wisdom, whose generated table is empty — the paragraphing a print WEB edition uses is not delivered at all.

## 4. Already agreed

Items the repository states as a deliberate choice, with the substance quoted, for confirmation or reversal.

| Item | Quoted substance | Source |
|---|---|---|
| Synthesized poem-line newline (BSB/WEB/WEBC) | "a '\n' is written before each poem run that follows earlier content, so every surface draws it as a line" | `docs/SOURCE_FIELDS.md` row 66 |
| Translation metadata block, book name/commonName/title fields, chapter-wrapper audio/verse-count fields dropped | decision recorded: "keep skipping" | `docs/SOURCE_FIELDS_DECISIONS.md` item 29 |
| Poem indent depth (1/2/3) discarded | "keep skipping, and record the reason ... revisit only alongside source paragraphs" | Decision #7 |
| Words-of-Jesus flag unread, red letter reconstructed from a separately generated, verified table | "keep tables, add a decode-time cross-check" (cross-check itself not yet built) | Decision #10 |
| Footnote `reference` field discarded, verse join done by `noteId` alone | recommends adopting the field's redundancy as a formal decode-time integrity check | Decision #25 |
| NKJV verse marker's own printed-number child skipped | "the app draws its own verse numbers; the subtree is presentation" | `docs/SOURCE_FIELDS.md`, NKJV table |
| NKJV acrostic letter-names ("qa") dropped, position (paragraph break) kept | cited approvingly as the model the WEB/WEB Catholic decoders should be fixed to match | `docs/SOURCE_FIELDS_DECISIONS.md`, Defect #1 |
| NKJV italicised "supplied words" flattened | future generated offset table recommended as later work, not started | Decision #6 |
| NKJV words-of-Jesus tag flattened, red letter from a separate exact-matched table | documented and guarded by rune-count and hash | `docs/SOURCE_FIELDS.md`, NKJV table |
| NKJV uppercasing of the four Jesus-naming verses specifically | "a faithful flattening of what the publisher set ... No change is recommended" | `docs/SOURCE_FIELDS_DECISIONS.md`, Defect #3 |
| NKJV chunk-overlap: the duplicate verse decode at each continuation point is discarded, keeping the first (correctly contextualised) decode | reasoned through directly in the code's own comment | `apibible.go`, `fetchAPIBibleBookByPassages` |
| Red-letter whole-verse fallback when a table entry's hash goes stale | "the fallback stays inside the same edition's judgement" | `red_letter_runs.go` |
| WEB/WEB Catholic hand-mapped red-letter boundary "recoveries" at 7 verses | "these are boundary recoveries, not red-letter judgements ... the recovery merely maps the publisher's \wj boundaries onto the older runtime wording" | `scripts/gen-web-redletter.py` |
| Search index/cards and AI-find results flatten poem breaks to spaces | "one-line contexts" | `docs/SOURCE_FIELDS.md` |
| Website link-unfurl preview flattens poem breaks | same "one-line contexts" reasoning | `docs/SOURCE_FIELDS.md` |
| Bluebook citation apparatus manufactured on partial-verse shares | explicit implementation of Bluebook Rules 5.1–5.3, with a dedicated test suite | `share.go`, `bluebook_test.go` |
| Sharing omits Psalm titles | "there is no whole-chapter share to attach one to, so this is low priority" | `docs/SOURCE_FIELDS_DECISIONS.md` |
| TTS does not read Psalm titles | "the work is the read-along model, which has no slot for a title before verse 1. Do it when that model gains one" | `docs/SOURCE_FIELDS_DECISIONS.md` |
| Footnotes withheld from nine of ten downstream pipelines | "nothing here may ever enter Text, the search index, the share pipeline, spoken audio, or a link ... enforced by construction and pinned by tests" | `bible.go`, `Footnotes` field comment |
| Omitted-verse orphan notes invisible to search/speech/share/copy/links | "Nothing but the chapter-bottom footnote section reads it ... invisible ... by construction" | `bible.go`, `OrphanFootnotes` field comment |
| Android "Fyne fallback" reading pane's internal inconsistencies | "genuinely unreachable in what ships ... no feature work, pin the release path" | Decision #22 |
| Deuterocanonical books carry no words-of-Christ markup | consistent with the source USFM, which contains none there | `docs/SOURCE_FIELDS.md` |

## 5. Never agreed

The list that matters most. Each of these is either flagged as an open question with no resolution recorded, or not discussed at all, or the recorded claim about it does not match what the code actually does.

1. **WEB/WEB Catholic stanza-break consequence.** The generator's exclusion of `\b` is a stated rule; its actual, near-total effect on Psalms, Song of Solomon, and Wisdom was never quantified or put to a decision.
2. **NKJV section headings and cross-reference lines.** Flagged OPEN, unresolved. The mr/sr/r cross-reference sub-case — erased with less protection than an ordinary footnote — is never separately named. The skip list also matches by exact string, so a related but unlisted style (s5, ms4, mt*, sd*) would silently fall through to the addition side rather than being skipped; this has never been checked against a live feed.
3. **Notes inside NKJV's skipped heading/reference blocks.** Unknown quantity. A probe was recommended and never run.
4. **Text or a note before an NKJV chapter's first verse marker.** Unknown quantity, never probed; the existing dismissal ("nothing to key it to") is asserted, not evidenced.
5. **NKJV "Selah."** Flattened exactly as WEB/BSB are (an accepted pattern for those two), but never named for NKJV specifically — 74 verses.
6. **NKJV bold ("bd") character style.** Flattening is asserted as policy, but whether the NKJV feed ever actually contains a "bd" span has never been verified either way.
7. **NKJV wire attributes never read** (Strong's numbers, altnumber/pubnumber print-verse-number variants, and others). No raw capture exists locally to check whether these are ever present.
8. **NKJV uppercasing beyond the four reviewed verses.** Roughly 30 further verses (Exodus 3:14 and John 8:58's "I AM," three "YAH" verses, two "BRANCH" verses, the phrase in Deuteronomy 28:58, Daniel 5's handwriting, the Gospel cross-inscriptions) are never mentioned; nor is the loss of the typographic distinction between small caps ("sc") and the divine name ("nd") as two different publisher conventions.
9. **Windows/Linux styled-pane token overpaint.** The code's own comment and test claim an exhaustive 2-verse/3-rune case. The real footprint is 48 verses / 82 runes across all four editions, including NKJV cases where the narrator's own "He said" is bled into red alongside Christ's words.
10. **Red letter's absence from six secondary surfaces.** Flagged OPEN in one document; a second document falsely claims this was already "analysed and answered." It was not.
11. **Verse-of-day never shows a Psalm's title** (9 of 371 rotation entries). Not discussed anywhere.
12. **Verse-of-day never applies red letter**, despite the rotation being deliberately curated to spotlight Christ (62 of 371 entries, about one day in six). The generic gap is flagged OPEN; this specific figure has never been surfaced.
13. **"Copy chapter" never includes a Psalm's title.** A different code path from the native-selection copy the documents actually discuss; never separately assessed.
14. **TTS never signals a paragraph or stanza boundary.** Not discussed anywhere.
15. **Cross-reference panel omits titles and red letter.** Its poem-break flattening is flagged OPEN with an instruction to "write it down," never done; the title and red-letter gaps in this same panel were never named at all.
16. **Share-as-image collapses paragraph breaks into poem-line breaks.** The existing code comment on this area is stale and describes a different, already-resolved distinction.
17. **The Android "Fyne fallback" pane's "Share…" bypasses the entire citation-formatting pipeline** via a bare, unformatted email link. Not mentioned anywhere (the pane is confirmed unreachable in current shipping builds, so exposure today is theoretical).
18. **The AI pipeline transmits the reader's raw, uncleaned selection** — verse-number digits, the NBSP glued to them, and the paragraph-indent characters whenever the selection spans a paragraph — to the external AI provider, with none of the cleaning the sibling Share pipeline applies to the identical captured text. The on-screen preview shown to the reader is already cleaner than what is actually sent. Not discussed anywhere.
19. **The AI prompt's `%q` string-quoting** turns an authored poem-line break into the literal two-character text `\n` rather than a real line break. Not discussed anywhere.
20. **Verse numbers, their NBSP joiner, and the paragraph-indent characters survive into native OS Copy/Select-All** on iOS, macOS and Android, completely unfiltered. The code's own rationale for leaving system Copy unclamped covers only preventing footnote misattribution, and never mentions this leak for an ordinary in-scripture selection.
21. **The public website has no copy-cleaning code of any kind.** Every browser-native copy of any verse carries the raw verse-number digit and its NBSP into the clipboard. Not discussed anywhere.
22. **The Windows/Linux desktop Copy command reads the raw selection model unfiltered**, even though the same pane's own Share and Study-with-AI commands clean the identical captured text before use — an inconsistency never acknowledged, and one that contradicts the pane's own "copy stays clean" comment.
23. **The footnote section's separator and entry-key labels ride along on native unclamped system Copy**, directly contradicting the section's own comment that "no markers are added to the Scripture text itself."
24. **A latent duplicate-orphan-footnote risk at API.Bible chunk-overlap boundaries.** Verses are deduplicated at these boundaries; the accompanying orphan-note map is not. Zero instances today only because the NKJV happens to have no omitted verses. Not discussed anywhere.
25. **`normalizeVerseSpaces` (NKJV) collapses any run of Unicode whitespace, including a non-breaking space, to one plain ASCII character**, with no way to tell afterward whether a collapsed run was a decode artefact or the publisher's own deliberate non-breaking space. Not discussed anywhere.

**Where passes disagree, direct evidence is preferred:** the styled-pane token-overpaint documentation (item 9) claims an exhaustive 2-verse case where recomputation against the real tables found 48; the claim that red-letter's secondary-surface gap was "analysed and answered" (item 10) is contradicted by a direct grep across every surface named, which found no such handling anywhere; and the share.go comment describing paragraph-versus-poem-line handling for share-as-image (item 16) is contradicted by tracing the actual call order in the code, which shows both the text and image paths receive the identical restored string before the real defect (a dropped blank segment) occurs later.

## 6. Recommended order

**Fix now — cheap, and clearly wrong regardless of any product judgement:**
- Route the Psalm 119 acrostic letters and the one Zechariah oracle title out of Verse.Text into the existing side-band mechanism (already specced as Defect #1; unimplemented).
- Clean the "Study with AI" pipeline's captured selection using the same helpers the Share pipeline already calls (`stripVerseMarkers`, `collapseSpaces`, `normalizeShareSelection`) before it is used or transmitted.
- Add the same cleaning to the Windows/Linux desktop's Copy command, which already has the cleaning function available for its sibling Share/AI verbs on the identical pane.
- Add copy-cleaning to the website (verse-number and NBSP stripping before selection), which currently has none.
- Include the Psalm superscription in the "Copy chapter" verb, using the accessor already used elsewhere in the codebase.
- Correct the two stale claims in the repository's own documentation: the styled-pane token-overpaint's actual scope (48 verses, not 2), and the false statement that red letter on secondary surfaces was already "analysed and answered."
- Run a live probe against the NKJV feed to establish whether Strong's numbers, `altnumber`/`pubnumber`, or a "bd" character style are ever actually present, so future decisions rest on evidence rather than assumption.

**Decide — genuine product or scope calls for the owner:**
- Whether to show BSB and NKJV chapter headings and cross-reference lines behind a toggle (default off), as already recommended, or continue dropping them entirely.
- Whether to invest in recovering WEB/WEB Catholic stanza breaks from source USFM, given how little of that structure survives today in Psalms, Song of Solomon and Wisdom.
- Whether Psalm superscriptions should be extended to search, TTS, sharing, the website, the cross-reference panel and verse-of-day as one unified decision, rather than seven separate silent gaps.
- Whether red letter should extend to the six secondary surfaces that currently render it monochrome.
- Whether TTS should signal paragraph and stanza boundaries (this needs new read-along model work, not a small change).
- Whether the roughly 30 further NKJV sc/nd uppercasing locations should get the same review the four Jesus-naming verses already received, and whether the sc/nd typographic distinction itself is worth preserving.
- Whether the website should finally render titles, footnotes and orphan-verse notes — the earlier "near-ready" recommendation assumed an accessor that does not actually exist, so this needs fresh scoping.
- Whether the reporter-layout paragraph indent needs a mechanism that does not leak literal characters into copied text on Apple platforms.

**Accept with a written reason — sound as they stand, just needs the decision recorded rather than left silent:**
- Bibliographic-only helloao drops (translation block, book name/commonName/title fields, chapter-wrapper audio/verse-count fields) — redundant with the app's own canon tables and self-hosted audio; no reader impact.
- The app drawing its own verse numbers and the NKJV verse marker's own printed-number child being skipped — necessary and already how every edition works.
- The manufactured decode-time and render-time join spaces and the poetry-boundary newline insertion — functionally necessary for correct prose and poetry rendering; only their exact scale was previously unstated, and is now recorded in this accounting.
- Footnotes and the NKJV's cross-references being withheld from nine of ten downstream pipelines — deliberately enforced by construction and pinned by tests, for the sound reason that translator apparatus should never be dispatched or attributed as Scripture.
- The Bluebook citation apparatus manufactured on partial-verse shares — a considered, tested, rules-based design for a chat or card medium, not an accidental leak.
- The chunk-overlap duplicate-verse-decode discard and the whole-verse red-letter fallback for a stale table entry — sound, defensive designs currently exercising zero real instances.
- The Android "Fyne fallback" pane's internal inconsistencies — confirmed unreachable in shipping builds, not worth reconciling unless it becomes reachable again.
- Deuterocanonical books carrying no words-of-Christ markup — correct, since the source text contains none.
