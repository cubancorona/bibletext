# The next major release

There is one branch, `main`. Minor releases are cut from it whenever they are
ready. Work meant for the next major version lands on it too, behind a switch
that stays off in every release build until the day that version ships. So a
minor release never waits for the major one, the major one never sits on a
branch that drifts from `main`, and every push builds and tests both.

**This page is the plan for the next major version and the rule for every
release before it.** It holds what the switch is, how to put work behind it
and try it, what keeps it out of every minor release, the few changes that
cannot go behind it, the register of what is behind it now with every open
decision, and the steps that turn it into a release. The detail of each
piece's data lives where that data is documented (`TEXTUAL-DATA.md`,
`VERSION_STATES.md`); where this page and one of those disagree about the
data, that document is right and this one needs fixing.

---

## Why one switch and not a branch

The three pieces in the register below were built on branches of their own
while 1.2.19 was prepared, and parked there. By the time they were wanted,
`main` had moved under them. It held earlier work from the same lines as
copies under other commits, so none of them could be merged; and 1.2.19's
cross-reference fixes had changed the code the Gospel parallels were built on
(`treasuryRowsFor`'s signature, the depth measurement, the test of the
kingdom woe). Each was ported by hand, change by change.

On `main` behind a switch, that cost is paid as it arises instead of all at
once. A change to shared code is compiled and tested against the gated work
on the push that makes it, a fix both states need is written once, and the
conflict a branch would have hidden until its merge shows the day it is made.

## The switch

One build tag, `next`, declared by one pair of files:

| File | Builds when | Declares |
|---|---|---|
| `next_off.go` | the tag is absent: every release build | `const nextRelease = false` |
| `next_on.go` | `-tags next` | `const nextRelease = true` |

Each file also exports the same value as `NextRelease`, for the commands built
beside the package (`cmd/websitegen` reads `bibletext.NextRelease`), which
cannot see an unexported name.

A build tag rather than a setting, for the reason the NRSV, LSB and
`nkjvxrefs` builds give (`versions_nrsv.go`): a setting, even one that is off,
ships the code it switches. Without the tag the gated code is not in the binary
at all.

**Without the tag the app is exactly the current release.** That is the
contract every gated change is held to, and it is what makes the switch safe
to leave in place for months.

## Minor releases never carry it

Every minor release, 1.2.20 and each one after it until the major version,
is built from `main` without the tag. It ships the last release's behaviour
plus its own fixes, and none of the work behind the switch. Nobody has to
remember that: no release path passes the tag, and two checks, each tested
against a planted control, stop one that would.

- **The release paths' text.** `next_release_guard_test.go`, which runs in
  both states, reads `release-ios.sh`, `release-mac-store.sh`,
  `build-android.sh`, `build-windows-exe.sh`, `build-appimage.sh`,
  `go-release-wrapper.sh`, `publish-site.sh` and the `release`, `msstore` and
  `linux-stores` workflows. Any tag list that names `next`, directly or
  through a variable it expands, fails, and so does one it cannot resolve.
  `BT_ANDROID_TAGS` is tolerated only while `build-android.sh --release`
  refuses it.
- **What each path built.** `scripts/verify-not-next.sh` reads the build tags
  Go records inside a binary, so it also sees the tag arriving through
  `GOFLAGS` in the environment or saved with `go env -w`, which no script
  shows. `verify-release-package.sh` runs it on every desktop package (GitHub,
  Mac App Store, Microsoft Store, Snap Store); `release-ios.sh` on the App
  Store binary; `build-android.sh --release` on the Play bundle's libraries;
  `publish-site.sh` on the site generator. The guard test asserts each of
  those calls is in place, and builds probe binaries to show the verifier
  refuses the tag each way it can arrive.

`RELEASING.md` runs the first in stage 0's suite and says, in stage 3, what a
refusal from the second means: a `GOFLAGS` saved with `go env -w` carries
build tags.

The major release is never built by setting the tag either. On its day the
gated work is made unconditional (*The major release*, below), and it goes
out through the same paths and the same two checks.

## Putting work behind it

Behaviour, wording, data tables and cache epochs go behind the switch: anything
one build can hold in both states. Use the smallest seam that keeps both
states readable:

1. **A decision in code.** `if nextRelease { … } else { … }`. The constant is
   known when compiling, so the branch not taken is dropped from the binary,
   and both branches are still type-checked in both builds.
2. **A value.** A cache epoch or a limit: one expression chooses it,
   `if nextRelease { epoch = 8 }`, or a pair of constants in split files.
3. **A pair of files.** When the states differ by whole declarations,
   `<name>_next.go` carries `//go:build next` and `<name>_current.go` carries
   `//go:build !next`, with the same identifiers in each. Older files that
   use the word in another sense, the note sticker's next-note control
   (`notes_next_test.go`, `dev_notes_next_*.go`), are not gated.
4. **Data.** A table that differs (versification, the Gospel parallels, red
   letter) is chosen by the switch, never edited by hand into one state. The
   generator that owns the data writes both, through an option for the next
   state (`--next`). The current state's file stays byte for byte what the
   last release shipped: `git diff <last release tag> -- <file>` is empty.
   The next state's is a generated `<name>_next.go` with `//go:build next`
   that declares its own table and installs it in place of the current one
   in an `init` function, so every reader of the table reads it and nothing
   else changes (`versification_data_next.go`). An embedded asset no
   generator owns gets a second asset beside the first, embedded by a
   `//go:build next` file whose `init` puts it in place the same way
   (`parallels_next.go`). A test reads both files and fails if they differ
   anywhere the piece does not say they should (`versification_states_test.go`,
   `parallels_states_test.go`), so a fix made to one and not the other fails.
5. **Tests.** A test that holds in both states stays untagged. One that pins
   current behaviour the next release changes gets `//go:build !next`, and its
   counterpart for the next release goes in a `_next_test.go` file with
   `//go:build next`. Never `t.Skip` on the switch: a skipped test reads as a
   pass.

A fix that both states need is written once, outside the switch, and reaches
readers in the next minor release.

Each piece gets an entry in the register below when it lands: what readers
will see, where the switch is read, its tests, what a build with the switch on
leaves on a device, and its open decisions.

## What needs a branch instead

Three kinds of change cannot be held off inside one build. Only these keep a
branch of their own:

| Change | Why the switch cannot hold it | Now |
|---|---|---|
| A toolkit upgrade | `go.mod` names one version of each module, and the release scripts swap in the patched Fyne for the build (`patches/README.md`); no build tag chooses a module version | the Fyne 2.8 port (`FYNE_28_PORT.md`), on a branch that is not published |
| Raising the minimum iOS or macOS version | the floor is one value in `config/product.json`, the one every Apple build and store listing derives from (`check-min-os-versions.py`); no build tag changes it | none |
| A change to how stored data is laid out | a build with the switch on shares a device's data with the store build (*Trying it on a device*), so data written the new way would be read the old way the moment the store build is reinstalled, and a move between layouts is one-way | none |

Such a branch is kept current so that it never becomes the parked kind:

1. **After every minor release ships**, it is brought up to date with `main`,
   and its suite passes again with and without the tag. A catch-up is then one
   release's worth of change, never several.
2. **At the start of the major release's run-up**, it is merged into `main`.
   From that merge on, `main` is the major release, so the run-up is kept
   short; a fix readers need before it ships is made on a branch from the
   last release's tag, released from there (every channel builds from the
   tagged commit, `VERSIONING.md`), and merged back.

The Fyne 2.8 port predates this rule and has not been brought up to date
since it was made, so its first catch-up, after the next minor release, is
the largest.

## Building and testing both states

```bash
go vet ./... && go vet -tags next ./...
go test ./...                     # the shipping build
go test -tags next ./...          # the next major release
scripts/check-ios-pane.sh --next  # and check-android-pane.sh --next
```

CI runs all of these on every push (`.github/workflows/ci.yml`): `go vet` with
`next` alone and beside `bibletextdev`, `gles`, and `nrsv,lsb`; the suite with
`next` under the race detector and once more with `next,bibletextdev`; the
suite with `next` on macOS for the darwin-only files; `go vet -tags next` on
Windows; and the iOS and Android pane compiles with `--next`.
`next_release_guard_test.go` fails if a cleanup drops any of them.

An editor sees the shipping state by default and greys out `//go:build next`
files. To work in them, give gopls `"buildFlags": ["-tags=next"]` in your own
editor settings, not the committed ones.

## Trying it on a device

| Where | Command |
|---|---|
| iPhone or iPad | `scripts/run-ios-device.sh --next` (add `--dev` for the Links tab) |
| Simulator | `scripts/run-ios-sim.sh --next` (add `--dev`) |
| Android, debug APK | `BT_ANDROID_TAGS=next scripts/build-android.sh` (or `bibletextdev,next`) |
| Desktop | `go run -tags next ./cmd/bibletext`; in VS Code, "Run Desktop NEXT release" |
| Web reader, locally | `go run -tags next ./cmd/websitegen -out <empty directory>`; never published |

A device build installs over the store build under the same bundle id, and
shares its data. Whatever the next build writes, a cache at a new epoch or a
note, is what the store build finds when it is reinstalled. A gated change that
writes something the current release would read differently says so in its
entry below, and reinstalling the store build is part of trying it.

---

## What is behind the switch now

Three pieces, reviewed and parked while 1.2.19 was prepared, then ported onto
`main` as changes, not merged (*Why one switch and not a branch*). All three
are ported. None has been tried on a device yet.

| Piece | What readers will see | Status | Tests | Open decisions |
|---|---|---|---|---|
| 1. The NKJV refreshed at cache epoch 8, with an offline bridge | One fresh download of the NKJV (about 13 MB) on the first launch that can reach API.Bible, bringing corrected headings after a 200-verse passage boundary and the divine name in small capitals inside headings and psalm titles; a reader offline at that launch keeps the previous copy for up to 30 days instead of falling back to the WEB | Ported; both states pass; not tried on a device | `licensed_epoch_bridge_test.go` (both states), `licensed_epoch_bridge_next_test.go`, `licensed_epoch_bridge_current_test.go`, `nkjv_epoch_next_test.go`, `nkjv_epoch_current_test.go`; switch-aware checks in `version_launch_flow_test.go` and `red_letter_epoch_test.go` | N1 |
| 2. The Greek Esther mapped verse for verse, like Daniel 3 | WEB Catholic's Esther lines up with the other editions: notes, highlights, shared links and cross-references cross between them, the three verses it lacks show gap marks, and the cross-references panel says why it lists nothing for verses the Treasury does not cover | Ported; both states pass; not tried on a device | `greek_esther_current_test.go` and `greek_esther_next_test.go` (here and in `cmd/websitegen/`), `crossref_coverage_test.go` with its `_current` and `_next` pair, `versification_states_test.go`, the site pins in `cmd/websitegen/testdata/` | E-A, E-B, E-C |
| 3. "Same saying, another occasion" among the Gospel parallels | The cross-references panel lists, after the synopsis rows, the passages that carry the same saying on another occasion, under a label of their own; and the twenty-two Gospel verses that belonged to no synopsis set each find one | Ported; both states pass; seen only in test renders | `parallels_occasions_test.go` and `parallels_states_test.go` (both states), `parallels_next_test.go`, `parallels_current_test.go`, the kingdom woe in `crossrefs_versification_test.go`, the depth in `crossrefs_realdata_test.go` | P1 to P6 |

### Open decisions

Every one is settled before the major release's run-up ends. Each is written
as the proposal to accept or change, with what the tree does now with the
switch on.

**1. The NKJV refresh**

- **N1. The picker's promise.** While a bridged copy is on screen nothing
  asks for the update, but the picker's sentence, "showing a previous edition
  until the update can be downloaded", promises an update within the session.
  Either the words change, or a tap on its checked row fetches
  (`VERSION_STATES.md`, *Open decisions*). The tree keeps the sentence and
  does not fetch.

**2. The Greek Esther**

- **E-A. The gap marks.** Keep the gap marks [6] in Esther 4 and [5], [30] in
  Esther 9: the three verses the Greek Esther does not have. The tree shows
  them.
- **E-B. One line for every uncovered addition.** Show the same "Nothing is
  listed for verses …" line for the Song of the Three, Susanna, Bel and the
  NKJV's extra verses, which today show the panel's first line alone. The
  tree gives the line to Esther only (`crossRefNotedAdditions`).
- **E-C. Two drafted sentences.** Approve the two sentences drafted for any
  book whose numbering does not correspond: the web reader's caveat
  (`incommensurableCaveat`, `cmd/websitegen/notice.go`) and the line under a
  note that cannot be placed (`placementCopyIncommensurable`,
  `notes_anchor.go`). No book reaches them in the next release; the shipping
  build keeps 1.2.19's sentences.

**3. Same saying, another occasion**

- **P1. The label.** Drafted as "Same saying, another occasion"; the
  alternatives are "Said on another occasion" and "Repeated on another
  occasion". Two groups (S33, R01) are a charge made by Jesus's opponents,
  not his own saying, so the label must not say "Jesus said". It is one
  constant, `otherOccasionLabel` (`crossref_panel.go`).
- **P2. Robertson's group.** Leave out R01, the one group taken from
  Robertson's harmony alone. The tree has it in the file.
- **P3. Where the twenty-two verses go.** Place them as new sets instead of
  changing sets 39, 49 and 71. The tree has them as reviewed: those three
  sets extended and four new ones added.
- **P4. The "Cf." pairings.** Keep out the five pairings the harmony marks
  only "Cf.": the pounds and the talents, John 17:2, Luke 12:46, Luke
  19:41-44, and Mark 11:19 beside Luke 21:37-38 (`TEXTUAL-DATA.md` §9.5). The
  tree leaves them out.
- **P5. The credit and the README.** The panel's footer still reads "Gospel
  parallels: synopsis". The harmonies, both public domain, are credited in
  `NOTICE` and `TEXTUAL-DATA.md` §9; whether the footer names them too, and
  how the README describes the second kind, waits on P1.
- **P6. Two hiding rules.** One rule for both kinds of row would hide
  verse-level Treasury rows inside whole pericopes. The tree keeps two (see
  piece 3's `crossRefHidden`); if one is wanted, `crossRefDeepestRead` is
  measured again with the real-data walk.

### 1. Every reader's NKJV refreshed at cache epoch 8, with an offline bridge

**Ported, behind the switch.** The NKJV's cache epoch rises from 7 to 8, so
the first launch of the release that can reach API.Bible fetches the edition
again (about 13 MB once). Two decoder fixes that the shipping decoder already
has change what a reader holds: the headings after a 200-verse passage
boundary, and the divine name in small capitals inside headings and psalm
titles. Without the epoch each copy takes them at its next fetch, within its
30-day window, which is what the shipping build does. A reader offline at
that first launch keeps reading the previous epoch's copy while it is inside
its own 30-day window, instead of the WEB: the **Licensed bridge** in
`VERSION_STATES.md`, where its states and transitions are written out.

Where the switch is read, all in `versions.go`:

- `nkjvCacheEpoch`: 7 off, 8 on.
- `loadLicensedBridge`, `bridgeInMemory`, and the rule in
  `purgeSupersededLicensedCaches` that keeps a copy the bridge may serve.
  Off, the bridge serves nothing, no copy in memory is the bridge's, and the
  sweep deletes every licensed superseded epoch, as 1.2.19 does.

The rest of the change is shared and, off, does what 1.2.19 does:
`loadVersionFallback` (the cache-only read, off), `sweepLicensedCachesAtLaunch`
(the two startup purges in their old order), the call sites in
`reading_state.go`, `versions_ui.go` and `share_link_open.go`, and
`currentUTCTime`, now a var so the tests can hold the clock still.

The tests: `licensed_epoch_bridge_test.go` holds the device the others launch
and what holds in both states. `licensed_epoch_bridge_next_test.go` and
`nkjv_epoch_next_test.go` hold the next release's behaviour, and
`licensed_epoch_bridge_current_test.go` and `nkjv_epoch_current_test.go` the
shipping build's, so a bridge or an epoch that reaches it fails there. The
launch enumeration asks for a licensed previous edition on screen with the
switch on and for none with it off. The red-letter table's recorded epoch is 7
or 8 to match: regenerated against an epoch-8 download, the table came out
byte for byte the same, because the epoch changes headings and titles and no
verse's text.

Trying it: a build with the switch on, at its first launch that can reach
API.Bible with the NKJV chosen, fetches `bibletext-nkjv-v8.json` and deletes
the epoch-7 copy, which spends the provider's quota once. A build
without it on the same data afterwards (the store build reinstalled, or
`go run ./cmd/bibletext` after `go run -tags next ./cmd/bibletext`, which share
one cache folder) finds no epoch-7 copy and fetches the NKJV again, and
neither reads nor removes the epoch-8 file. That file stays until a build with
the switch on runs again, the system clears the cache folder, or it is
deleted from that folder by hand.

The verse-of-the-day snapshot (`testdata/verse_of_day_snapshot.json`) holds in
both states: regenerated from an epoch-8 download when this was built, all 329
entries in all four editions came out as they were. Its `sources` block, which
names the cache files the last generation read, is left as it is; it already
names older public-domain epochs than the shipping build's, and is rewritten
when the snapshot is next regenerated.

Open decision: N1.

### 2. The Greek Esther mapped verse for verse, like Daniel 3

**Ported, behind the switch.** WEB Catholic's Greek Esther is translated from
a different text from the Hebrew Esther the other editions print, and the
shipping table records the whole book as incommensurable: no reference, note,
highlight or cross-reference crosses between them. Measured on the downloaded
texts, it keeps the Hebrew book's verse numbers: 164 of the WEB's 167 verses
are there under their own numbers, it has nothing at 4:6, 9:5 and 9:30, and
its additions are numbered 4:18-47 and 10:4-14 or sit inside 1:1, 3:13, 5:1-2
and 8:13 (`TEXTUAL-DATA.md` §2.2). In the next release it maps verse for verse,
and the cross-references panel says why it lists nothing for a selection the
Treasury does not cover.

The data, generated for both states from the same caches:

- `scripts/gen-versification.py --next` writes `versification_data_next.go`.
  It decides a different text by whether its numbers line up with the
  reference's instead of calling it incommensurable outright (§2.3 rule 4):
  the Greek Esther scores 0.92, against 0.06 to 0.58 for every control. Its
  table differs from `versification_data.go` only in WEB Catholic's Esther:
  three verses absent, forty-one its own, none incommensurable.
- `scripts/gen-omitted-verses.py --next` writes `omitted_verses_data_next.go`,
  which records the Greek Esther's three gaps as holes. The script now reads
  which books to leave out from the versification table it checks against.
- `versification_data.go` and `omitted_verses_data.go` are byte for byte
  1.2.19's, and each script without `--next` reproduces them from the same
  caches. `versification_states_test.go` fails if the two states' tables
  differ anywhere but WEB Catholic's Esther.

Where the switch is read:

- The two tables, through the generated files' `init` (seam 4).
- `buildCrossRefList` and `showCrossRefs`: the coverage line under "No
  cross-references for this selection." (`crossRefCoverage`), and the answer
  given before the Treasury loads for a selection it cannot cover at all.
- `placementCopy` (`notes_anchor.go`) and the web reader's caveat
  (`cmd/websitegen/notice.go`, through `bibletext.NextRelease`): the
  sentences for a book whose numbering does not correspond. No book reaches
  them in the next release; the shipping build keeps 1.2.19's, which its
  Greek Esther readers see.

Everything else follows the table: notes and highlights carried between
translations, shared links, the gap marks, the cross-references' rows, the
web reader's notice pages and version switcher, and its completeness check
for the NKJV. Read-along narration stays off for WEB Catholic's Esther in both
states: its words are not the recording's.

The tests: `greek_esther_current_test.go` pins the shipping build's Greek
Esther (incommensurable everywhere, its gaps not called omissions, the 1.2.19
sentences) and `greek_esther_next_test.go` the next release's, with a pair of
the same names under `cmd/websitegen/` for the web reader. Tests that only
needed a note or a highlight that cannot be placed now use one of the Greek
Esther's additions, which cannot be placed in either state, and expect the
state's own sentence (`greekAdditionUnplacedSentence`); the incommensurable
arms are exercised on a book a test marks as one (`withIncommensurableBook`).
`crossref_coverage_next_test.go` and `crossref_coverage_current_test.go` hold
the panel in each state. The web reader's fixture tree is pinned per state
(`cmd/websitegen/testdata/nkjv_off_site.sha256` and
`nkjv_off_site_next.sha256`), and the
next one differs only on the four Esther 1 pages the piece names.

The deepest cross-reference row a panel reads (`crossRefDeepestRead`) is the
twentieth in the shipping build, WEB Catholic's Genesis 41:42, whose rows into
the Greek Esther it cannot show. With the Greek Esther mapped it was the
eighteenth, first at the WEB's Matthew 10:1; with the third piece too it is
the twenty-first (below). The notice pages leave the verse off 28 chapters'
links in the shipping build and 20 in the next release. On the real site the
next release rewrites the forty Esther chapter pages, ten in each of the four
editions, and no other.

Trying it: nothing a build with the switch on writes is read differently by the
store build. A note or highlight carried into or out of the Greek Esther is
stored as any other is, and the store build shows it wherever its own table
can place it.

Open decisions: E-A, E-B, E-C.

### 3. "Same saying, another occasion" among the Gospel parallels

**Ported, behind the switch.** The synopsis answers one question: where do
the other Gospels tell this event? It deliberately keeps apart passages that
share words but, by the harmonies, not an occasion: Luke's Lord's Prayer,
lost sheep, faithful servant and lament over Jerusalem, and six Lukan sets
beside Matthew's Beelzebul controversy, sign of Jonah and teaching on anxiety.
A reader on either side is offered nothing. In the next release the
cross-references panel lists, after the synopsis rows and before the
Treasury, the passages that carry the same saying on another occasion, each
under its own label and the saying's title; and the twenty-two Gospel verses
that belong to no synopsis set are placed. The sources are Stevens and
Burton's *Harmony of the Gospels for Historical Study* (1904), checked
against Robertson's harmony (1922), both public domain; only verse pairings
are taken (`TEXTUAL-DATA.md` §9).

The data, both chosen by the switch (seam 4):

- `assets/parallels/gospel_occasions.json`: 102 groups, one saying each,
  with 297 passages and 274 pairs, each pair two passages placed on
  different occasions. Only the next release embeds it.
- `assets/parallels/gospel_parallels_next.json`: the synopsis with the
  twenty-two verses placed. Matthew 4:23 joins the preaching tour of
  Galilee; Matthew 4:24-25 and Luke 6:17-19 join the healing of the
  multitudes by the sea; Luke 6:43-45 joins a tree and its fruit. Luke
  6:24-26, 21:37-38, John 11:55-57 and 13:31-35, which have no parallel in
  the harmony, become four sets of one Gospel each, which show no rows but
  place the verses.
- `gospel_parallels.json` is byte for byte 1.2.19's. `parallels_next.go`
  embeds the two next files and puts them in place in its `init`.
  `parallels_states_test.go` fails unless the two synopses differ in exactly
  those three sets and four new ones.

Where the switch is read:

- `crossRefsForSelection` (`crossrefs.go`) looks up the other-occasion rows
  only `if nextRelease`. Without the tag there is no data to look up either.
- `crossRef.otherOccasion()` is the one test of a row's kind, read by the
  list (`composeCrossRefList`) and the row (`crossRefRow`). Without the tag
  it is false, so the label and its badge are compiled out.

The shared code, which does what 1.2.19 does when there are no
other-occasion rows:

- The reader of the occasions file and its lookup (`parallels.go`).
- `addOtherOccasionRow`, which lists a passage once, at its widest, and
  never one a row above it covers.
- `crossRefHidden`, what the rows above the Treasury hide. A synopsis row
  hides a Treasury row with its own label, as before; an other-occasion row
  hides every Treasury row inside its passage. Without the tag it holds
  labels alone.
- `treasuryRowsFor`, which now takes a `crossRefHidden` rather than a set of
  labels, adapted to `main`'s resolver, which returns a list of rows.

The label is one constant, `otherOccasionLabel` (`crossref_panel.go`), drawn
as an outlined tag where the synopsis's is filled.

The tests:

- `parallels_occasions_test.go` and `parallels_states_test.go` hold in both
  states: the file reader, the de-duplication and the hiding on synthetic
  rows, and the two synopses' difference.
- `parallels_next_test.go` holds the reviewed behaviour:
  - the file loads whole;
  - no pair lies inside one synopsis set;
  - the rows' order, label and de-duplication;
  - the kingdom woe mapped in every translation;
  - every passage mapped whole in every translation;
  - the six Lukan doublets and the named pairs;
  - the twenty-two verses where the harmony prints them;
  - the depth walk's bounds over every Gospel selection of up to six verses
    and every whole chapter.
- `parallels_current_test.go` pins the shipping build: no other-occasion
  data or rows, the twenty-two verses in no set, and a row marked as another
  occasion still drawn as the synopsis's.
- The kingdom woe test (`crossrefs_versification_test.go`) reads both kinds
  of row, so it holds in both states. In the next release Luke 11:52 is the
  kingdom woe's other occasion and hides the Treasury's copy of it, so the
  test gives the kingdom woe a second Treasury row (Isaiah 22:22) that
  nothing hides.

Measured over the real texts, in all four editions, every single-verse panel
is identical to 1.2.19's without the tag. With it, every panel lists exactly
the reviewed branch's 3,174 other-occasion rows, and only 2,042 panels
change. Where a panel differs from the branch beyond that, the cause is a
cross-reference fix made on `main` since the branch was cut, or the Greek
Esther:

- The kingdom woe's Treasury row from Luke 11:52 now opens the kingdom woe,
  so the other-occasion row hides it.
- Matthew 18:35's row to Mark 11:25 now opens Mark 11:26, which no
  other-occasion row covers.
- A range now ends at the last verse a translation has.

The deepest Treasury row a panel reads (`crossRefDeepestRead`, re-measured
with all three pieces on) is the twenty-first, Matthew 10:17 in every
edition. Five of its best rows lie inside passages its chapter lists as the
same saying on another occasion: two are Matthew 10:17's own, three its
neighbours'. That is inside the 32 the index keeps. The walk measures the
depth between what every selection holding a verse hides and the most any
selection hides (`crossRefSelectionsHide`). The narrower bound now leaves out
an other-occasion row that a synopsis row of the chapter covers. A selection
holding both verses lists the synopsis row instead, which hides by label
alone; the only verse this affects is Luke 4:24.

Trying it: a build with the switch on writes nothing new. The verse a
reader selects and the rows shown are not stored. It has been seen only in
`TestRenderCrossRefPanel`'s phone-sized renders (Matthew 12:25, Luke 11:2):
`scripts/run-ios-device.sh --next` is the next look.

`NOTICE` already credits the two harmonies, because the occasions file is in
the repository whichever build reads it. On the day the major version ships,
its sentence about `gospel_parallels_next.json` moves to
`gospel_parallels.json`, which that file becomes, and `TEXTUAL-DATA.md` §9
loses its note that it is the next release's.

Open decisions: P1 to P6.

---

## The major release

### The run-up

1. **Branches merge first.** Every branch from *What needs a branch instead*
   is merged into `main`, after the minor release before it has shipped. From
   here a fix readers need goes out from a branch cut at the last release's
   tag.
2. **Decisions settle.** Every open decision above is answered, and each
   answer is made behind the switch with its test, so the register's last
   state is the one reviewed.
3. **Each piece is tried on a device** with the switch on (*Trying it on a
   device*), with the store build reinstalled over it where the piece's entry
   says so.

### Release day

4. **Remove the switch from each piece; never flip it.** Setting `next_off.go`
   to `true` would ship the next release beside dead code for the old one,
   two copies of every gated table and tests of a state no build has, and
   `next_release_guard_test.go` refuses it. Each gated change is made
   unconditional instead:
   - the shipping arm of each `if nextRelease` (or `if bibletext.NextRelease`)
     is deleted, and the condition with it;
   - each `_current.go` file is deleted and its `_next.go` partner loses its
     constraint;
   - each generator's next rule becomes its only rule and its `--next` option
     goes; its shipping file is regenerated from it and its `_next.go` file
     deleted (`versification_data_next.go`, `omitted_verses_data_next.go`);
   - each next asset replaces the shipping one under its name
     (`gospel_parallels_next.json` becomes `gospel_parallels.json`), an asset
     the shipping build lacks is embedded where the rest are
     (`gospel_occasions.json`, in `parallels.go`), and the file that put them
     in place is deleted (`parallels_next.go`);
   - each file pinned per state (`nkjv_off_site_next.sha256`) replaces the
     shipping one and stays as its copy, and the register of pages it may
     differ on (`nextReleaseSitePages`) empties.
5. **Tests collapse to one state.** `!next` tests are deleted and `next` tests
   lose their constraint; a value a test chooses by the switch
   (`crossRefDeepestRead`) keeps its next one; the tests that compare two
   states' data (`versification_states_test.go`, `parallels_states_test.go`)
   go with the second copy.
6. **Nothing is left behind.**
   `grep -rli 'nextrelease\|go:build.*next' --include='*.go' .` finds the two
   switch files, the release guard (`next_release_guard_test.go`) and the site
   guard with its empty register (`cmd/websitegen/site_off_golden_test.go`),
   and nothing else; `grep -l -e '--next' scripts/gen-*.py` finds nothing; and
   the suite passes with and without the tag, which now build the same app.
7. **The version number is chosen then, not before**: 2.0 or 1.3, by what the
   release turns out to hold. The ledgers take it as any release's do
   (`RELEASING.md`, stage 1), under `VERSIONING.md`'s rules.
8. **It ships like any other release**, by `RELEASING.md`, built without the
   tag, which by now changes nothing.

The switch, the CI steps and both guards stay. The register empties, and the
next major version starts filling it.
