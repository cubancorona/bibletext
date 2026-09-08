# Documentation index

Every document under `docs/`, and — the part a directory listing cannot give
you — **the circumstance that should send you to it**. The titles are already
visible in `ls`; the trigger is what is missing.

Start with [AGENTS.md](../AGENTS.md) for repository guidance,
[ARCHITECTURE.md](../ARCHITECTURE.md) for how the app fits together, and
[CONTRIBUTING.md](../CONTRIBUTING.md). This file covers `docs/` only.

This index is held to **set equality** with the directory by
`docs_index_test.go`: a new document with no entry fails the build, and so does
an entry whose file was renamed or deleted. That is deliberate. Before this
index existed, five documents were referenced from nowhere at all — among them
`PRIVACY_RELEASE_CHECKLIST.md`, whose first instruction is to re-evaluate the
store declarations whenever the key-storage path changes, which is exactly what
was being changed when nobody found it. A document nothing points at is a
document nobody reads, and an index that is allowed to drift is worse than
none, because it is confidently wrong.

Adding a document? Add its line here. One line, saying **when** to read it.


## How we work

- [COMMIT_AND_CODE_PROTOCOL.md](COMMIT_AND_CODE_PROTOCOL.md) — before writing any commit message, code comment or doc text — the rules here are enforced by a hook and by CI
- [VERSIONING.md](VERSIONING.md) — before choosing a version number, cutting or moving a tag, or shipping any channel of a release
- [BACKLOG.md](BACKLOG.md) — before picking up deferred work, or before re-investigating a defect that may already be closed here
- [PRIVACY_RELEASE_CHECKLIST.md](PRIVACY_RELEASE_CHECKLIST.md) — before any release that touches screenshots, store declarations, the public mailbox, or data flows — a checklist


## Scripture text, and where it comes from

- [SCRIPTURE_WORKLIST.md](SCRIPTURE_WORKLIST.md) — before any decoder, text-pipeline or reading-face work — the governing standard, S-item statuses, epoch rule
- [ADDITIONS_AND_DROPS.md](ADDITIONS_AND_DROPS.md) — before the app adds or drops any text — what is agreed, what never was, and the escape channels
- [SOURCE_FIELDS.md](SOURCE_FIELDS.md) — when changing a decoder or asking what a source sends — a decoder change must edit this inventory too
- [SOURCE_FIELDS_DECISIONS.md](SOURCE_FIELDS_DECISIONS.md) — before arguing to capture, drop or show a source field — each is already costed with an effort/epoch verdict
- [TEXTUAL-DATA.md](TEXTUAL-DATA.md) — before regenerating or trusting a derived table — versification, red-letter spans, cross-edition verse mapping
- [FOOTNOTES.md](FOOTNOTES.md) — before changing footnote capture or the chapter-bottom section, or when apparatus could leak into verse text


## Translations, keys and links

- [VERSION_STATES.md](VERSION_STATES.md) — before changing version caching, epoch bumps, translation switching or parked links; invariants are pinned
- [NKJV_FLOW.md](NKJV_FLOW.md) — before touching shared-link handling, web/app handoffs, or the /nkjv/ notice pages — I1-I6 are binding
- [API_KEY_HANDLING.md](API_KEY_HANDLING.md) — before a release build, an API.Bible key rotation, or a support-address change — has a pre-publish checklist
- [WEB_READER_PLAN.md](WEB_READER_PLAN.md) — before changing share links, book slugs, or cmd/websitegen — the URL contract here is frozen forever


## Notes and sharing

- [NOTES_SPEC.md](NOTES_SPEC.md) — before changing any notes code — store, anchor, plan, tint, spacing: the 15 binding invariants are here
- [NOTES_STATE.md](NOTES_STATE.md) — when a note or highlight bug resurfaces, or before striking/adding a defect id in the state-enumeration harness
- [SHARED_NOTES.md](SHARED_NOTES.md) — before adding a key to the share-link fragment or drawing a note on a new surface — note-safety rules are here
- [NOTE_WIRE_FORMAT.md](NOTE_WIRE_FORMAT.md) — before editing share_note.go, any note decoder, or adding a wire tag — this format is frozen for shipped builds
- [NOTE_CHROME_UNIFICATION.md](NOTE_CHROME_UNIFICATION.md) — before adding a noteChrome field or changing the native note ABI — the rule is one appended field per commit
- [NOTE_SPACING.md](NOTE_SPACING.md) — before changing note band, card or pill spacing on any surface — a map to the real source; it holds no numbers
- [NOTES_DESIGN.md](NOTES_DESIGN.md) — only when you need why a notes option was rejected (cap-by-view vs cap-by-action, no Expanded field); superseded


## The reading surface

- [READING_TYPOGRAPHY.md](READING_TYPOGRAPHY.md) — before changing scripture size, leading, measure or the reading face — rule: the measure never takes the scale
- [VISUAL_TESTS.md](VISUAL_TESTS.md) — before shipping any change to note chrome, bands, arrivals or reading layout — the per-surface screen checklist
- [PLATFORM_MIMIC.md](PLATFORM_MIMIC.md) — before treating a BIBLETEXT_MIMIC run as Windows/Linux evidence — rule: it never replaces the CI visual smokes


## Platforms and release

- [ANDROID.md](ANDROID.md) — before touching android/ or cmd/mobile/AndroidManifest.xml, or building, signing or shipping an APK/AAB
- [IPAD.md](IPAD.md) — before changing touch navigation or iPad reading typography, or shipping iOS — every release must stay universal
- [MAC_APP_STORE.md](MAC_APP_STORE.md) — before building, signing or submitting a Mac Store package, or changing the bundle id, sandbox, or macOS floor
- [APP_STORE_SUBMISSION.md](APP_STORE_SUBMISSION.md) — before any App Store or Mac App Store step — build identity, metadata, screenshots, review notes, submission
- [PLAY_LISTING.md](PLAY_LISTING.md) — before creating the Play app record, answering Data safety or IARC, or uploading an AAB or store graphics


## Fyne itself

- [FYNE_28_PORT.md](FYNE_28_PORT.md) — before upgrading Fyne past 2.7.4, or when touching popup/overlay code or text measurement
- [FYNE_FORK_POLICY.md](FYNE_FORK_POLICY.md) — before patching, rebasing or tagging Fyne itself, or before moving the app off the temporary patch pipeline
