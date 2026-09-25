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

Read-only, creates nothing, safe at any moment. It prints what this tree
declares and what each of the five store fronts is actually serving, asked of
the stores themselves. A store it cannot reach is printed as `UNREACHABLE` and
the exit code is non-zero — it never reports a network failure as up to date.

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

**The two irreversible acts**: `--submit` in `appstore/submit-version.py`, and
`commit` in `msstore/submit.py`. Both are mechanically automatable and both are
deliberately left as their own step, because Apple releases `AFTER_APPROVAL` and
the Store publishes `Immediate` — once certification passes there is no second
checkpoint on either.

**Console-only, per store**: the IARC age-rating questionnaire; Snap categories,
screenshots, banner and publisher display name; promotion of Google Play beyond
closed testing (the service account is scoped so it structurally cannot reach
production); the Play Foreground-service declaration video.

**The Flathub pull request, entirely.** Flathub's policy is that AI tools must
not open or automate submission PRs, or generate their commit messages,
descriptions, review comments or replies. A conductor may generate the
tag-pinned manifest and stage the files; the PR, its text and every reply are
the account holder's own words. This is not a preference to be weighed.

**The per-release judgement calls**: the privacy answer, content-rights and
export-compliance declarations, and the accessibility labels, re-read against
Apple's current definitions and the live terms of every AI provider.

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
   `go run ./cmd/linuxmeta render`, which rewrites both AppStream files, the
   desktop entry and `snap/snapcraft.yaml`
8. `msstore/metadata/en-gb/whats-new-<v>.txt` — the Microsoft Store's What's
   New, tracked: at most 1,500 characters, nothing but visible text, spaces
   and line breaks, and not a copy of another release's; required from 1.2.16
   on, including for a release the Store is not sent
   (`docs/WINDOWS_STORE_LISTING.md`, "What's new")

Plus the Play notes blockquote in `docs/PLAY_LISTING.md`, under 500 characters.

### 2 — Push and wait for green

Push `main` (permission required) and wait for CI green on **all three** OSes.
Nothing is uploaded before that. A darwin host cannot see a linux or windows
vet failure, and a red run has sat unnoticed for a day before. The hygiene step
alone runs nine checks, so a local run of `check-repository-hygiene.py` on its
own proves nothing.

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

### 4 — Read each artefact back

Version, build number, minimum OS — from the artefact, not the ledger. An
artefact that was never read back is an assumption.

The iOS and Mac scripts do this themselves: `release-ios.sh` reads the signed
`.ipa` back in its final step, and `release-mac-store.sh` checks the signed
`.pkg` after signing and fails if the entitlement did not survive. The AAB is
read with `bundletool dump manifest`. Record what each one printed.

`scripts/verify-release-package.sh` is **not** this step. It is the CI-only
check on the GitHub release assets — trimpath, the release key, and that no
runner workspace path leaked into the package — and it refuses to run without
`GITHUB_WORKSPACE` set. It reads no version and no build number.

### 5 — Upload to Apple, then Play

`xcrun altool --upload-app -t ios` and `-t macos`. **Keep both delivery
UUIDs**: each is that build's App Store Connect id and `submit-version.py`
checks the attached build against it.

Play: `scripts/play-publish.py --dry-run --notes <file> upload <aab> alpha`
first — it uploads into an edit it then discards — then the identical command
without `--dry-run`, with `--status completed`.

### 6 — Submit to Apple

Wait for each build to reach `VALID`, then per platform:

```
python3 appstore/submit-version.py --platform IOS --build N --delivery-uuid U \
    --write --confirm-version <v> --accept-inherited-screenshots --submit
```

Screenshots are the one field a release may knowingly inherit, and saying so
explicitly is how that stays a decision rather than an oversight. Apple takes
**one version per platform** into review at a time, so check nothing else is in
review on that platform first — `release-status.py` prints it.

### 7 — Tag, last

An annotated `v<version>` at the build commit, pushed (permission required).

**This is why the order is what it is.** Tagging last means never publishing a
version number the store builds turned out to be unable to produce. On
21 September 2026 the tag went first and it was recoverable only because `HEAD`
still equalled the tag and the tree was clean.

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
publishes both snaps to the Snap Store's **edge** channel.

### 8 — Finish the GitHub release

```
gh release upload v<version> BibleText-Android.apk --clobber
gh release edit v<version> --draft=false
```

Compare the APK's sha256 after downloading it back. Publish only once every
asset is attached: `/releases/latest` reassigns the moment the draft clears, so
a half-built release becomes everyone's download.

### 9 — Microsoft Store

```
gh workflow run msstore.yml --ref v<version>     # at the TAG, never the branch
```

The MSIX version is the desktop ledger plus a fourth `.0`, so a run dispatched
at `main` produces a package labelled for a tree the tag does not name. Nothing
in `msstore.yml` reads the tag — `--ref` simply decides which tree is checked
out — so this is a discipline the workflow cannot enforce for you. Check the
run's ref before trusting its artefacts.

This stage does not otherwise depend on the tag, so it can run before stage 7
if a Windows-only fix needs to reach the Store sooner. What it must not do is
run against a tree that differs from whatever `v<version>` ends up naming.

Download both `BibleText-Windows-<arch>-msix` artefacts, then:

```
. scripts/msstore-env.sh
msstore/submit.py preflight <dir>    # identity, version, ANGLE, PE machine word, What's New
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

The listing's What's New goes with the packages: `preflight` measures
`msstore/metadata/en-gb/whats-new-<v>.txt`, and `create` reads it before
anything exists on the server, refuses one that is missing, empty, over 1,500
characters, another release's text or carrying a character the listing must
not, and sets it as the listing's `releaseNotes`, the only listing field a
submission changes. `verify` holds the server to that exact text and every
other listing field to the clone. If the PUT is refused over the notes,
`abort`, correct the file and `create` again. `submit.py` reads the file on
disk, so the correction need not be committed first, and after the tag it
should not be until no store build of `<v>` remains: a commit after the tag
makes `<v>` unbuildable (stage 7).

### 10 — Snap

On a Linux host (snapcraft runs nowhere else; the Ubuntu arm64 VM serves, over
SSH):

```
snapcraft promote bibletext --from-channel edge --to-channel stable
```

One action covers every architecture and refuses a partial set.

### 11 — Link anything newly live, then publish the site

If a channel has gone live for the **first** time, add it to `docs/index.html`
and `README.md` **before** publishing. `check-public-surfaces.py` cannot catch
this: a new store is not a new release asset, so nothing fails.

```
scripts/publish-site.sh --dry-run    # drift report
scripts/publish-site.sh
```

Owner machine only — the reader is generated from the app's own decoder and the
local translation caches, which CI does not have. It refuses a dirty tree, so
the site always matches a known revision.

---

## When a stage fails

Re-running from the top is usually wrong. These steps are not idempotent:

| step | re-running does | recovery |
| --- | --- | --- |
| `altool --upload-app` | rejects a duplicate build number | bump Build, rebuild |
| `play-publish.py upload` | rejects a used versionCode | bump Build, rebuild |
| `submit-version.py --submit` | 409 — already in review | remove from review in the console |
| `msstore/submit.py create` | 409 — one pending submission at a time | `msstore/submit.py abort`, then create |
| `git tag` push | tags are immutable | use the next number; the old one is spent |
| `gh release edit --draft=false` | `/releases/latest` has already moved | attach what is missing, fast |

`msstore/submit.py abort` refuses to delete a pending submission the run did
not create, so a draft started in Partner Center is safe from it.

## What a conductor refuses

- Writing the human copy and shipping it unread.
- Pushing anything — `main`, a tag, or `gh-pages` — without being told.
- Opening or writing any part of a Flathub PR.
- Entering a password, or printing, committing or transmitting any credential
  value. Sourcing a credential script into its own process is fine; that is
  what they are for.
- Deleting a pending store submission it did not create.
- `--submit` or `commit` as part of an unattended sweep.
- Reporting a store as up to date that it could not actually reach.

## Where "all stores" stops being true

Two limits are structural rather than missing tooling, and a release should say
so plainly rather than look incomplete:

**Google Play production is closed** until the 12-tester / 14-day requirement is
met, and the service account is deliberately scoped so it cannot reach
production at all. A release reaches the alpha track and stops there.

**Flathub and AppImageHub have no submission** and cannot get one from a
conductor.

Everything else — GitHub, Snap, the App Store, the Mac App Store, the Microsoft
Store — is reachable in one pass.
