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

Nothing has been ported yet. Three pieces were reviewed and parked while 1.2.19
was prepared, and come next. Each is ported as a change, not merged: `main` already
holds earlier work from the same lines under different commits.

### 1. Every reader's NKJV refreshed at cache epoch 8, with an offline bridge

The NKJV's cache epoch rises from 7 to 8, so the first launch of the release
fetches the edition again (about 13 MB once): two decoder fixes change the
headings a reader holds after a 200-verse passage boundary, and the divine
name in small capitals inside headings and psalm titles. A reader offline at
that launch keeps reading the previous epoch's copy while it is inside its
30-day window, instead of the WEB. The verse-of-the-day snapshot's sources
line records the epochs it was checked against.

With the switch off, the NKJV's epoch is 7.

Open decision: while the bridged copy stays on screen nothing asks for the
update, but the picker's sentence, "showing a previous edition until the
update can be downloaded", promises one within the session. Either the words
change, or a tap on its checked row fetches.

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
