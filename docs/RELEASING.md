# Releasing

One document for the whole thing: every store, every platform, in the order
they have to happen.

It is written so that "release", or "release to the stores", is a sufficient
instruction. Whoever picks that up — a person returning to this cold, or an
agent asked to conduct it — reads this page, reads the live state, and works
down the stages. Nothing here needs to be remembered between releases; that is
the point of writing it down.

**This page holds the order and the decisions. It does not hold the details of
any one store** — those live in `docs/APP_STORE_SUBMISSION.md` (Apple, and the
authoritative sequence this page condenses), `docs/WINDOWS_STORE_LISTING.md`,
`docs/LINUX_STORES.md` and `docs/PLAY_LISTING.md`. Where this page and one of
those disagree, the specific document is right and this one needs fixing.

---

## Start by reading the world, not the notes

```
. scripts/asc-env.sh
. scripts/msstore-env.sh
scripts/release-status.py
```

Read-only and creates nothing; safe at any moment except beside another
Google Play step, because its Play read opens an edit and ends the one an
upload or a promotion has open (stage 6). It prints what this tree declares
and what each of the five store fronts is actually serving, asked of the
stores themselves. A store it cannot reach is printed as `UNREACHABLE` and
the exit code is non-zero — it never reports a network failure as up to
date.

**Do this before believing any note, including this one.** Written state goes
stale between releases; a release decision made against a stale note is how a
whole preparation gets wasted. Source the two credential scripts, never pipe
them: a pipe is a subshell and the exports die with it, surfacing later as a
bare `KeyError` that reads like a missing credential.

---

## What only the owner decides

A conductor may prepare, verify, upload and report. These stay with the
account holder, and a conductor that finds one undone stops and says so rather
than inventing an answer.

**All the human copy.** The iOS and Mac What's New files, the Microsoft Store
What's New (`msstore/metadata/en-gb/whats-new-<v>.txt`), the review notes for
both platforms, the Play release notes, the `[[release]]` head and bullets in
`linux/releases.toml`, the GitHub release notes. A conductor checks they exist,
are named for this version, are non-empty, are inside the character limits and
are not a copy of another release's — it does not write them. Drafting a
suggestion to be accepted or rewritten is fine and welcome; publishing one that
was never read is not.

**Every push.** Standing rule: never push to GitHub without clear explicit
permission. That covers the `main` push, the tag push and the `gh-pages` push,
each separately.

**The three irreversible acts**: `--submit` in `appstore/submit-version.py`,
`commit` in `msstore/submit.py`, and `promote` in `scripts/play-publish.py`.
All three are mechanically automatable and each is deliberately left as its
own step, run only on the account holder's OK, because Apple releases
`AFTER_APPROVAL`, the Store publishes `Immediate`, and Play sends a promotion
to production straight into its review — once certification passes there is
no second checkpoint on any of them.

**Google Play's promotion to production** is one of those three: the account
holder's OK, then the script (stage 7). Until 2 October 2026 it was a Play
Console step, because the service account was scoped so it could not reach
production; that day it was granted "release to production", for this app
only. The What's new text production shows is the alpha release's, carried
unchanged, and the dry run prints it line by line for that OK.

**Console-only, per store**: the IARC age-rating questionnaire; Snap categories,
screenshots, banner and publisher display name; raising, completing or halting
a staged Google Play rollout; the Play Foreground-service declaration video.

**The per-release judgement calls**: the privacy answer, content-rights and
export-compliance declarations, and the accessibility labels, re-read against
Apple's current definitions and the live terms of every AI provider.

---

## Minor releases are built without `next`

Work for the next major version is on `main` too, behind the `next` build tag
([NEXT.md](NEXT.md)). A release cut from `main` by this page is built without
the tag, so the work behind it is compiled out of every store package, every
GitHub asset and the site; none of the commands below passes it. That does
not rest on care. `next_release_guard_test.go`, part of stage 0's suite,
fails if any release path could carry the tag; and every path refuses an
artefact built with it, however the tag reached the build (stage 3).

The major release goes through these same stages, once NEXT.md's release-day
steps have made its work unconditional. It is never built by setting the tag.
In its run-up, once a branch has merged into `main`, a minor release is cut
from the last release's tag instead of `main`; NEXT.md, *A fix during the
run-up*, gives the few steps that change.

---

## Stages

Numbered for reference, not because each needs a ceremony. Stages 0–2 are
reversible; from stage 5 things leave the building.

### 0 — Preflight, read-only

Working tree clean, `HEAD` the commit to release, and:

```
scripts/check-release-identity.py
scripts/check-version-not-spent.sh <version>
go run ./cmd/linuxmeta render && git diff --exit-code
scripts/release-status.py
```

`check-version-not-spent.sh` is the one that saves a wasted afternoon: a
version number names ONE tree on every channel, so if `origin` already has
`v<version>` pointing somewhere other than `HEAD`, the number is spent and the
release needs the next one. Never run it with the override set.

Then the full local gate — `go build ./...`, `go test ./...`,
`go test -race ./...`, `go vet ./...` — plus `scripts/check-ios-pane.sh` and
`scripts/check-android-pane.sh` if any `ios`- or `android`-tagged file moved.
The host build is blind to those, so nothing else will catch them.

That `go test ./...` includes the release guard, `next_release_guard_test.go`:
it fails if any release script or workflow could build with the next-release
switch ([NEXT.md](NEXT.md)). And `go env GOFLAGS` must name no build tags. The
release scripts empty `GOFLAGS` in their own environment, but a value saved
with `go env -w` still reaches them (stage 3).

Confirm `build/appstore/metadata/en-GB` exists before trusting the test run:
`TestWhatsNewIsNamedForThisRelease` SKIPS when it does not, which is why CI
cannot catch a missing Mac What's New — the exact fault that got 1.2.8's Mac
submission refused. The Microsoft Store's What's New is tracked, so
`TestWindowsWhatsNewIsNamedForThisRelease` runs everywhere and needs no such
check.

### 1 — Prepare, on main

Eight coupled surfaces have to name the same version, each test-enforced:

1. `cmd/mobile/FyneApp.toml` — Version and Build
2. `cmd/bibletext/FyneApp.toml` — Version and Build
3. `appstore/review-notes.txt` — first line `VERSION <v>`
4. `appstore/review-notes-macos.txt` — same
5. `TARGET_VERSION` in `appstore/push-review-notes.py`
6. `build/appstore/metadata/en-GB/whats-new-<v>.txt` **and**
   `build/appstore/metadata/en-GB/mac/whats-new-<v>.txt` — the Mac has its own
   and the write refuses without it; neither may be byte-identical to another
   release's
7. `linux/releases.toml` — a new `[[release]]` block, then
   `go run ./cmd/linuxmeta render`, which rewrites the AppStream metainfo,
   the desktop entries and `snap/snapcraft.yaml`
8. `msstore/metadata/en-gb/whats-new-<v>.txt` — the Microsoft Store's What's
   New, tracked: at most 1,500 characters, nothing but visible text, spaces
   and line breaks, and not a copy of another release's; required from 1.2.16
   on, including for a release the Store is not sent
   (`docs/WINDOWS_STORE_LISTING.md`, "What's new")

Plus the Play notes blockquote in `docs/PLAY_LISTING.md`, under 500 characters
with its line breaks counted.

### 2 — Push and wait for green

Push `main` (permission required) and wait for CI green on **all three** OSes.
Nothing is uploaded before that. A darwin host cannot see a linux or windows
vet failure, and a red run has sat unnoticed for a day before. The step named
"Repository hygiene" is not one script but the whole list in `ci.yml` —
`check-support-contact.py`, `check-release-identity.py`,
`check-product-identity.py`, `check-mac-store-config.py`,
`check-min-os-versions.py` and `check-public-surfaces.py`, the Python unit
tests under `scripts/audio-align`, `msstore`, `appstore` and `play`, and
`check-repository-hygiene.py` last — so a local run of
`check-repository-hygiene.py` on its own proves nothing.
`releasing_doc_test.go` holds this list to the step.

### 3 — Build the store artefacts, one at a time

**Strictly one at a time.** All three of these swap `go.mod` — and
`FyneApp.toml` — under an `EXIT` trap, replacing Fyne with the local patched
copy for the duration of the build: `release-ios.sh`, `release-mac-store.sh`
and `build-android.sh` alike. They share one repository root, so two running
at once read each other's half-applied tree, and the trap that restores it
fires per script. It is not an Android quirk; it is every store build.

```
scripts/release-ios.sh          # BIBLETEXT_UPLOAD unset -> build/BibleText.ipa
scripts/release-mac-store.sh    # -> build/mac-store/BibleText.pkg
scripts/build-android.sh --release
```

Each refuses a binary built with the next-release switch
(`scripts/verify-not-next.sh`, [NEXT.md](NEXT.md)), however the tag reached
the build. None of them passes it, and each empties `GOFLAGS` in its own
environment, which does not reach a value saved with `go env -w`: Go reads
the saved one whenever the variable is empty. So a refusal means a saved
`GOFLAGS` carries build tags. `go env GOFLAGS` shows it, `go env -u GOFLAGS`
clears it; then build again.

The Windows Store package needs nothing from this machine: the push of stage 2
builds it. `msstore.yml` runs on a push of `main` that touches its inputs, and
the release bump touches one of them, `cmd/bibletext/FyneApp.toml` — the
desktop ledger the MSIX version is stamped from, plus a fourth `.0`. On
29 September 2026 the push of 1.2.17 started it at once, and five minutes
later both packages were built at the pushed commit. Confirm that rather
than assume it:

```
gh run list --workflow msstore.yml --limit 1 --json databaseId,headSha,event,conclusion
```

Its `headSha` must be the pushed commit — the one the tag will name. Nothing
in `msstore.yml` reads the tag; the tree checked out decides what the package
is labelled with, so a run at any other tree produces a package labelled for
one the tag will not name. Only if the push did not start it — the bump went
up in an earlier push and this one touched none of its inputs — dispatch it
while `main` is still that commit, and read the new run's `headSha` the same
way. `workflow_dispatch` takes a branch or a tag, never a commit, so
`--ref <sha>` is refused:

```
gh workflow run msstore.yml --ref main           # only while main IS the release commit
```

If a later fix moves the tag, its push builds the package again only if the
fix touched one of the workflow's inputs, and most reader code is not among
them: the filter names a small minority of the root package's files, so a
push that starts CI need not have started this workflow — on 28 September
2026 a push of thirty-two files, reader code, docs and an icon, started CI
and no Store run at all. So run the same `headSha` check on that push, and
if no run started, dispatch at `main` while `main` is the fixed commit. Done
that way the package exists before the tag, and stage 9 can go the moment
the tag is pushed. `releasing_doc_test.go` holds the ledger inside the
filter and most of the root package outside it.

### 4 — Read each artefact back

Version, build number, minimum OS — from the artefact, not the ledger. An
artefact that was never read back is an assumption.

The iOS and Mac scripts do this themselves: `release-ios.sh` reads the signed
`.ipa` back in its final step, and `release-mac-store.sh` checks the signed
`.pkg` after signing and fails if the entitlement did not survive. The AAB is
read with `bundletool dump manifest`. Record what each one printed.

`scripts/verify-release-package.sh` is **not** this step. It checks how a
desktop package's executable was built — trimpath, the release key, no
next-release switch, and that no build machine's path leaked into the
package — and it reads no version and no build number. CI runs it on the
GitHub release assets and the Linux and Microsoft Store packages;
`release-mac-store.sh` runs it here on the Mac App Store binary, before
signing, giving it the checkout as `GITHUB_WORKSPACE`, without which it
refuses to run.

### 5 — Tag, once every artefact exists

An annotated `v<version>` at the build commit, pushed (permission required),
as soon as stage 4 has read all three artefacts back — before anything is
uploaded.

**This is why it waits for stage 4 and no longer.** The tag publishes a
version number, and a number must never be published for a tree the store
builds turned out unable to produce; by the end of stage 4 that is proven.
Waiting longer bought nothing: a refusal from here on is a store's — a missing
What's New, a rejected upload — and the one that has happened (1.2.8, Mac) was
put right in App Store Connect, not in the tree. Until 29 September 2026 the
tag went after the Apple submission, which held the GitHub release, both snaps
and the Windows package behind Apple's processing wait for no reason. On
21 September 2026 the tag went first, before any build, and it was recoverable
only because `HEAD` still equalled the tag and the tree was clean.

**And once it is pushed, stop committing to `main` under that number.** The tag
is what `check-version-not-spent.sh` compares against, so the first commit
after it makes the version unbuildable — not by policy but by that check, which
every store build honours. Later the same day:

```
$ scripts/check-version-not-spent.sh 1.2.13
ERROR: version 1.2.13 is SPENT — origin's v1.2.13 points at 2edaa9d19100,
which is not HEAD (774417353b5e).
```

That is correct behaviour, and it is survivable only because every 1.2.13
artefact had already been built from the tag. If a store build has still to
happen, run it before the next commit lands, or that store waits for the next
number. Anything merged after the tag ships under the next version.

`release.yml` then builds a **draft** release — macOS universal, Linux amd64
and arm64 tarballs, both AppImages with `.zsync`, both Windows zips — and
publishes both snaps to the Snap Store's **edge** channel, while the Apple
uploads of stage 6 go on.

### 6 — Upload to Apple, then Play

`xcrun altool --upload-app -t ios` and `-t macos`. **Keep both delivery
UUIDs**: each is that build's App Store Connect id and `submit-version.py`
checks the attached build against it.

Play: `scripts/play-publish.py --dry-run --notes <file> upload <aab> alpha`
first — it uploads into an edit it then discards — then the identical command
without `--dry-run`, with `--status completed`. The notes file keeps its line
breaks, so the opening line and each bullet reach Play as lines. That is where
stage 6 ends on Play: the release is on the alpha track, `upload` refuses the
production track, and production waits for the promotion in stage 7. The
script refuses an option it does not know, such as `--dryrun`, before it
reads the key, so a mistyped dry run never commits.

**An upload's commit restarts a review already in progress.** It takes
Play's default for changes in review, which cancels that review and sends
everything again, so an upload committed while a production release or a
listing change is still in Play's review sends that review back to the
start. Before the real upload, check that the Play Console's Publishing
overview shows nothing in review, or accept the restart. `promote` and
`play/push-screenshots.py` ask Play to refuse instead (docs/BACKLOG.md, "A
Play upload's commit restarts a review in progress").

**Play steps run one at a time.** Every Play command opens an edit as the one
service account, and Play lets that account hold one open edit: a new edit
ends the one already open, and a commit or a change in the Play Console ends
every other. So uploads, `promote`, `tracks`, `release-status.py` reads and
`play/push-screenshots.py` go in turn, never as parallel steps. An upload whose
edit was ended that way says so; nothing reached Play, and the same command
runs again once nothing else is touching it.

Play's screenshots are not part of a release: the listing keeps its images
from one release to the next. An approved new phone and tablet set goes up
with `play/push-screenshots.py` — read-only first, then `--rehearse`, then
`--write --confirm-version <v>` (docs/SCREENSHOT_PLAYBOOK.md, §6).

### 7 — Submit to Apple, promote on Play

Two of the three irreversible acts, each on the account holder's OK (the
third is the Microsoft Store's `commit`, stage 9). Apple: wait for each build
to reach `VALID`, then per platform, three commands in this order:

```
python3 appstore/submit-version.py --platform IOS --build N --delivery-uuid U \
    --write --confirm-version <v>
python3 appstore/push-screenshots.py --platform IOS --write --confirm-version <v>
python3 appstore/submit-version.py --platform IOS --build N --delivery-uuid U \
    --write --confirm-version <v> --submit
```

The first creates the version record, attaches the build and writes the
text, then ends non-zero with `NOT SUBMITTED` naming `screenshots`: App Store
Connect seeds a new version with the previous release's sets, and that is
the report to expect at this point; anything else in that list is a real
gap. The second replaces the sets with the release's own — read-only without
`--write`, so run it that way first and read the plan. The third submits, and
the preflight it runs now finds the screenshots written for this release.
The order is not negotiable: a version placed in a review submission no
longer takes image edits, so the screenshots cannot follow the submission,
and `push-screenshots.py` refuses a version past that point.

`--accept-inherited-screenshots` on the last command is for the release that
knowingly uploads no screenshots. Screenshots are the one field a release may
knowingly inherit, and saying so explicitly is how that stays a decision
rather than an oversight. Apple takes **one version per platform** into
review at a time, so check nothing else is in review on that platform first —
`release-status.py` prints it.

Play's promotion waits on nothing of Apple's, only on an OK that names Play:
the dry run first, and the same command without `--dry-run` once its output
has been read.

```
scripts/play-publish.py promote alpha production --confirm-version <v> --dry-run
scripts/play-publish.py promote alpha production --confirm-version <v>
```

The dry run reads the alpha and production tracks in a fresh edit, prints
both, prints the release notes line by line, writes production in the edit,
has Play validate it, and deletes the edit: nothing changes. Those notes are
the What's new text production will show, carried from the alpha release
unchanged; the script writes none of its own. The second command does the
same and commits, asking Play to refuse rather than cancel and restart a
review already in progress. Both refuse, changing nothing, unless `<v>` is
the mobile ledger's Version and alpha holds exactly one release — completed,
with notes, and with one versionCode, the ledger's Build — and they refuse
once production carries that versionCode in any state, or a higher one. A
halted staged rollout on production gives way to the promotion, since that
is where a fix is promoted from, and the dry run names it; a rollout still
going out is completed or halted in the Play Console first, and a draft
there is rolled out or discarded, and until then both commands refuse.
`--rollout 0.2` stages the release to a fifth of readers instead; raising,
completing or halting a staged rollout is done in the Play Console. A commit
refused over changes in review — Play reviews the alpha upload too — is run
again once that review clears. After the commit, `scripts/release-status.py`,
run on its own, shows production on the new versionCode.

### 8 — Finish the GitHub release

```
gh release upload v<version> BibleText-Android.apk --clobber
gh release edit v<version> --draft=false
```

Compare the APK's sha256 after downloading it back. Publish only once every
asset is attached: `/releases/latest` reassigns the moment the draft clears, so
a half-built release becomes everyone's download. The APK is one of them: the
download page links it, and `release-status.py` reports it missing from a
release that skipped this upload.

### 9 — Microsoft Store

The package was built in stage 3, at the commit the tag now names — by the
push of `main`, or by the dispatch that stood in for it. Check the run's
`headSha` against the tag before trusting its artefacts
(`gh run view <run-id> --json headSha`; the plain view does not print it); if
the tag ended up elsewhere, dispatch again at the tag and take that run's
artefacts instead:

```
gh workflow run msstore.yml --ref v<version>     # at the TAG, never the branch
```

The MSIX version is the desktop ledger plus a fourth `.0`, so a run at any
other tree produces a package labelled for one the tag does not name. Nothing
in `msstore.yml` reads the tag — the ref simply decides which tree is checked
out — so this is a discipline the workflow cannot enforce for you.

Download both `BibleText-<v>.0-<arch>-msix` artefacts, then:

```
. scripts/msstore-env.sh
msstore/submit.py preflight <dir>    # identity, version, ANGLE, PE machine word, What's New, names
msstore/submit.py create   <dir>     # POST + PUT + upload, stops before commit
msstore/submit.py verify   <dir>     # re-read from the server
msstore/submit.py commit             # <- the irreversible one
msstore/submit.py poll
```

`create` is safe: the submission is a copy and the live listing is untouched,
so `abort` deletes it with no public trace. The previously published package
stays `Uploaded` rather than being marked `PendingDelete` — it reaches nobody
while a higher version exists, and keeping it makes a rollback one PUT instead
of a rebuild against a spent number.

The artefacts, and the files inside them, carry the version:
`BibleText-<v>.0-<arch>.msix`, the four-part version the manifest is stamped
with and then the architecture, and that is the name to upload under. The
copy `create` makes holds every package the published submission was ever
sent, and the Store refuses a new package whose file name the copy already
has — after the submission exists, so a clash found there leaves a draft to
`abort` and files to rename before a second attempt. A name without the
version uploads once and never again, which is why every release's names are
new. `preflight` and `create` each read the published submission's package
list and refuse a repeated name while the server holds nothing, naming the
file and the name to give it; a package renamed by hand keeps the same form.

The listing's What's New goes with the packages: `preflight` measures
`msstore/metadata/en-gb/whats-new-<v>.txt`, and `create` reads it before
anything exists on the server, refuses one that is missing, empty, over 1,500
characters, another release's text or carrying a character the listing must
not, and sets it as the listing's `releaseNotes`, the only listing field a
submission changes. `verify` holds the server to that exact text and every
other listing field to the clone. If the PUT is refused over the notes,
`abort`, correct the file and `create` again. `submit.py` reads the file on
disk, so the correction is sent without a commit. After the tag it stays
uncommitted until the release is done: a commit after the tag makes `<v>`
unbuildable, which matters while a store build of it remains (stage 5), and
moves `main` off the tag the site is published from. Stage 11 refuses the
dirty tree the file leaves, so set it aside for that stage rather than let it
ride along:

```
git stash push -- msstore/metadata/en-gb/whats-new-<v>.txt   # before publish-site.sh
git stash pop                                                # after it
```

Then commit it, once no store build of `<v>` remains. Like any work after the
tag, it ships under the next number; the tag keeps the text that was refused.

### 10 — Snap

On a Linux host (snapcraft runs nowhere else; the Ubuntu arm64 VM serves, over
SSH):

```
snapcraft promote bibletext --from-channel edge --to-channel stable
```

One action covers every architecture and refuses a partial set.

### 11 — Link anything newly live, then publish the site

If a channel has gone live for the **first** time, add it to `docs/index.html`
and `README.md` **before** publishing, and draft its line for the release
notes, the `NOTES` printf in `.github/workflows/release.yml`, for the account
holder to accept, since the GitHub release notes are their copy. The printf is
read when a tag is pushed, so the line reaches the next release's notes; one
already published changes only by `gh release edit`, a change to a public page
that waits for their word. The printf is also its own format string, so the
line carries no single quote and no percent sign. `check-public-surfaces.py`
cannot catch a missing channel: a new store is not a new release asset, so
nothing fails. It holds Google Play in all three places only because it names
Play; it does not know the next store. Play went live on 30 September 2026,
the page and the README linked it that day, and the release notes, which this
stage did not then name, went on offering Android readers only the APK until
the printf gained its Play line later the same day.

```
scripts/publish-site.sh --dry-run    # drift report
scripts/publish-site.sh
```

Owner machine only — the reader is generated from the app's own decoder and the
local translation caches, which CI does not have. While the NKJV's text is
switched on (`cmd/websitegen/nkjv_text.go`) it also needs the API.Bible key in
the login Keychain, and the dry run fetches the NKJV live, as the publish does. It refuses a dirty tree, so
the site always matches a known revision. A Microsoft Store What's New
corrected after the tag (stage 9) is stashed across this stage, not committed
into it.

---

## When a stage fails

Re-running from the top is usually wrong. These steps are not idempotent:

| step | re-running does | recovery |
| --- | --- | --- |
| `altool --upload-app` | rejects a duplicate build number | bump Build, rebuild |
| `play-publish.py upload` | rejects a used versionCode once a run has committed; a run whose edit Play ended ("This Edit has been deleted") committed nothing | after a commit, bump Build and rebuild; after an ended edit, run the same command again on its own |
| `play-publish.py promote` | refuses, printing what production holds, once production carries the versionCode, and while production holds a rollout still going out or a draft | a run stopped before its commit changed nothing and deleted its edit; one that reports the commit's outcome unknown may have committed, so run `play-publish.py tracks` on its own first; a commit refused over a review in progress runs again once the review clears; a rollout still going out is completed or halted in the Play Console, and a draft rolled out or discarded there, before a re-run (a halted rollout gives way to the promotion) |
| `submit-version.py --submit` | 409 — already in review | remove from review in the console |
| `push-screenshots.py --write` | leaves a set that holds the files, reorders one that holds them out of order, replaces the rest | re-run once a FAILED image or a refusal is understood |
| `play/push-screenshots.py --write` | leaves a type that holds the files, replaces the rest | a run stopped before the commit deleted its edit and changed nothing; one that reports the commit's outcome unknown may have committed, so run it read-only first; re-run once the reason is understood |
| `msstore/submit.py create` | 409 — one pending submission at a time | `msstore/submit.py abort`, then create |
| `git tag` push | tags are immutable | use the next number; the old one is spent |
| `gh release edit --draft=false` | `/releases/latest` has already moved | attach what is missing, fast |

`msstore/submit.py abort` refuses to delete a pending submission the run did
not create, so a draft started in Partner Center is safe from it.

## What a conductor refuses

- Writing the human copy and shipping it unread.
- Pushing anything — `main`, a tag, or `gh-pages` — without being told.
- Entering a password, or printing, committing or transmitting any credential
  value. Sourcing a credential script into its own process is fine; that is
  what they are for.
- Deleting a pending store submission it did not create.
- `--submit`, `commit` or `play-publish.py promote` as part of an unattended
  sweep.
- Two Google Play steps at once.
- Reporting a store as up to date that it could not actually reach.
- Building a release with the `next` tag, or getting past a refusal from
  `scripts/verify-not-next.sh` any way but removing the tag from where it came.

## Where "all stores" stops being true

One limit is structural rather than missing tooling, and a release should say
so plainly rather than look incomplete:

**AppImageHub has no submission** and cannot get one from a conductor.

**Google Play production was a second, until 2 October 2026.** Production
has been open since 30 September 2026 (1.2.17, versionCode 187); until the
service account was granted "release to production" for this app on
2 October, an upload stopped on the alpha track and the promotion was made in
the Play Console. It is now `scripts/play-publish.py promote`, on the account
holder's OK like Apple's `--submit` and the Microsoft Store's `commit`
(stage 7); only raising, completing or halting a staged rollout stays in the
console.

Everything else — GitHub, Snap, the App Store, the Mac App Store, the Microsoft
Store and Google Play — is reachable in one pass.
