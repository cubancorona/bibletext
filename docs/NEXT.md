# The next major release

There is one branch, `main`. Minor releases are cut from it whenever they are
ready. Work meant for the next major version lands on it too, behind a switch
that stays off in every release build until the day that version ships. So a
minor release never waits for the major one, the major one never sits on a
branch that drifts from `main`, and every push builds and tests both.

## The switch

One build tag, `next`, declared by one pair of files:

| File | Builds when | Declares |
|---|---|---|
| `next_off.go` | the tag is absent: every release build | `const nextRelease = false` |
| `next_on.go` | `-tags next` | `const nextRelease = true` |

A build tag rather than a setting, for the reason the NRSV, LSB and
`nkjvxrefs` builds give (`versions_nrsv.go`): a setting, even one that is off,
ships the code it switches. Without the tag the gated code is not in the binary
at all.

**Without the tag the app is exactly the current release.** That is the
contract every gated change is held to, and it is what makes the switch safe
to leave in place for months.

## Putting work behind it

Use the smallest seam that keeps both states readable:

1. **A decision in code.** `if nextRelease { … } else { … }`. The constant is
   known when compiling, so the branch not taken is dropped from the binary,
   and both branches are still type-checked in both builds.
2. **A value.** A cache epoch or a limit: one expression chooses it,
   `if nextRelease { epoch = 8 }`, or a pair of constants in split files.
3. **A pair of files.** When the states differ by whole declarations,
   `<name>_next.go` carries `//go:build next` and `<name>_current.go` carries
   `//go:build !next`, with the same identifiers in each.
4. **Data.** A table that differs (versification, the Gospel parallels, red
   letter) is chosen by the switch, never edited by hand into one state. The
   generator that owns the data writes both, through an option for the next
   state, into the split files of seam 3 or beside the embedded asset. The
   current state's file stays byte for byte what the last release shipped:
   `git diff <last release tag> -- <file>` is empty.
5. **Tests.** A test that holds in both states stays untagged. One that pins
   current behaviour the next release changes gets `//go:build !next`, and its
   counterpart for the next release goes in a `_next_test.go` file with
   `//go:build next`. Never `t.Skip` on the switch: a skipped test reads as a
   pass.

Some changes cannot be switched off, and they keep a branch of their own: the
Fyne 2.8 port (`FYNE_28_PORT.md`), raising the minimum iOS or macOS version,
and a change to how stored data is laid out that a current build would then
read. Everything else comes to `main` behind the switch.

A fix that both states need is written once, outside the switch, and reaches
readers in the next minor release.

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

## What keeps it out of every release

Two checks, and each is tested against a planted control:

- **The release paths' text.** `next_release_guard_test.go` reads
  `release-ios.sh`, `release-mac-store.sh`, `build-android.sh`,
  `build-windows-exe.sh`, `build-appimage.sh`, `go-release-wrapper.sh`,
  `publish-site.sh` and the `release`, `msstore` and `linux-stores`
  workflows. Any tag list that names `next`, directly or through a variable it
  expands, fails, and so does one it cannot resolve. `BT_ANDROID_TAGS` is
  tolerated only while `build-android.sh --release` refuses it.
- **What each path built.** `scripts/verify-not-next.sh` reads the build tags
  Go records inside a binary, so it also sees the tag arriving through
  `GOFLAGS` in the environment or saved with `go env -w`, which no script
  shows. `verify-release-package.sh` runs it on every desktop package (GitHub,
  Mac App Store, Microsoft Store, Snap Store); `release-ios.sh` on the App
  Store binary; `build-android.sh --release` on the Play bundle's libraries;
  `publish-site.sh` on the site generator. The guard test asserts each of
  those calls is in place, and builds probe binaries to show the verifier
  refuses the tag each way it can arrive.

## The day the major version ships

1. Every entry below is reviewed and every open decision settled.
2. Each gated change is made unconditional: the current branch of each
   `if nextRelease` is deleted with the test; each `_current.go` file is
   deleted and its `_next.go` partner loses its constraint; each generator's
   next output becomes its only output; `!next` tests are deleted and `next`
   tests lose their constraint.
3. `grep -rn 'nextRelease\|go:build.*next' --include='*.go' .` finds the
   switch files and the guard test and nothing else, and the suite passes with
   and without the tag, which now build the same app.
4. The version number takes its major step (`VERSIONING.md`), and the release
   follows `RELEASING.md` as any other does.

The switch, the CI steps and both guards stay. The register below empties, and
the next major version starts filling it.

## What is behind the switch

Three pieces were reviewed and parked while 1.2.19 was prepared. Each is
ported as a change, not merged: `main` already holds earlier work from the same
lines under different commits. The first is ported; the other two come next.

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

Open decision: while the bridged copy stays on screen nothing asks for the
update, but the picker's sentence, "showing a previous edition until the
update can be downloaded", promises one within the session. Either the words
change, or a tap on its checked row fetches (`VERSION_STATES.md`, *Open
decisions*).

### 2. The Greek Esther mapped verse for verse, like Daniel 3

WEB Catholic's Greek Esther keeps the Hebrew book's verse numbers, so notes,
highlights and cross-references cross between it and the other editions
instead of stopping at the book. The versification generator decides a
different text by whether its numbers line up. The cross-references panel says
why it lists nothing for a selection the Treasury does not cover.

Open decisions:

- A. Keep the gap marks at Esther 4:6 and 9:5, 9:30, the three verses the
  Greek Esther does not have.
- B. Show the same "Nothing is listed for verses …" line for the Song of the
  Three, Susanna, Bel and the NKJV's extra verses, which today show the
  panel's first line alone.
- C. Approve two sentences drafted for any book whose numbering does not
  correspond: the web reader's caveat (`incommensurableCaveat`,
  `cmd/websitegen/notice.go`) and the line under a note that cannot be placed
  (`placementCopyIncommensurable`, `notes_anchor.go`).

### 3. "Same saying, another occasion" among the Gospel parallels

A new group of rows in the parallels panel for passages that share a saying
but, by the harmonies, not an occasion (Luke's Lord's Prayer beside
Matthew's, among others), from Stevens and Burton's Harmony of the Gospels
(1904, public domain); and a set for each of the 22 Gospel verses that belong
to no synopsis set today.

Open decisions:

- The label: drafted as "Same saying, another occasion"; the alternatives are
  "Said on another occasion" and "Repeated on another occasion".
- Leave out the one group taken from Robertson's harmony (1922) alone.
- Place the 22 verses as new sets rather than changing three existing sets.
- Keep the five pairings the harmony marks only "Cf." out.

When ported: the change meets the cross-reference code that has moved on since
(the Treasury row lookup's signature, the panel depth measurement, the kingdom
woe test), and the deepest cross-reference read is measured again with the
switch on.
