# Maintaining a Fyne fork

*Status: fork-side work EXECUTED 2026-08-12 (see §7 for what stands and what
remains). Owner sign-off still needed on the consumption flip. Supersedes the
patch pipeline described in `patches/README.md` once the flip lands.*

## Why fork now, when we said "don't fork" a week ago

The earlier call held for what we were carrying: three cosmetic patches
(184 diff lines) applied to a regenerated tree on every mobile build. The
ambition changed: **draw-loop work, text selection, and entry quality are
engine work**, and engine work cannot ride context diffs —

- patches against moving internals rot on every upstream release;
- binaries cannot ride diffs at all (the Noto emoji font already needs a
  side-channel copy step);
- there is no way to run Fyne's own test suite against a pile of `.patch`
  files, so every rebase is verified by hope;
- and our host test suite currently runs against **stock** Fyne — the deeper
  the changes go, the bigger that hole gets. A fork makes the patched engine
  the tested engine.

Ground facts, checked 2026-08-11: upstream releases run at roughly six to ten
events a year (`v2.7.1 … v2.7.4`, then `v2.8.0`, which is already out while we
pin `v2.7.4`). Fyne is ~22 MB, 865 Go files. We carry 184 diff lines plus one
10.7 MB font.

## 1. Shape of the fork

**A dedicated repo — `github.com/cubancorona/fyne` — holding upstream plus a
curated commit stack, maintained by rebase.**

> **Already in hand (found 2026-08-12, since built on).** The repo
> **existed**: a public fork of fyne-io/fyne created in June, with a local
> clone at `~/Dev/fyne` (remotes wired: `origin` = the fork, `upstream` =
> fyne-io) carrying the June drawloop fix as commit `572ea43ad` on branch
> `fix/ios-drawloop-idle-timeout`, plus `ISSUE_DRAFT.md`/`PR_DRAFT.md`.
> Corrections found during execution: the issue draft **had been filed** —
> it is [fyne-io/fyne#6368](https://github.com/fyne-io/fyne/issues/6368),
> closed 2026-06-20 after maintainer feedback (see decision point 3) — and
> the June commit is only *v1* of the drawloop fix: the shipped patch later
> grew the framePainting mid-paint present-guard, so fork commit ① was built
> from the shipped patch, not the June commit. (`~/Dev/fyne-5422-investigation`
> is the follow-up investigation of the PR the maintainer pointed to.)

- Branch `bt-main` = upstream release tag + our commits, **one commit per
  logical change, never squashed together**. Today's stack converts 1:1:
  ① drawloop idle-timeout (100ms→2ms), ② discrete caret blink, ③ Noto Color
  Emoji (an ordinary binary commit — the copy-step hack disappears).
- On each upstream release: `git rebase --onto v2.8.1 v2.8.0 bt-main`, run the
  fork CI, tag. Rebase-not-merge keeps the stack readable and each commit
  individually upstreamable. With a small stack, most rebases are silent.
- Tags: `v2.8.0-bt.1` — upstream version visible at a glance, our iteration
  after the suffix.
- **Fork CI is the rebase safety net**: on every push, run Fyne's own test
  suite plus a small file of our invariant tests (caret repaint cadence, emoji
  glyph coverage — things we already know how to measure). A rebase that
  breaks our changes fails in CI, not on a phone three weeks later.

## 2. How the app consumes it

Two candidate mechanisms. **Spiked 2026-08-12 — Option 1 works empirically**,
which flips the lean:

- **Option 1 — remote replace (VERIFIED, now recommended).** Checked-in
  `go.mod` carries `require fyne.io/fyne/v2 v2.7.4` +
  `replace fyne.io/fyne/v2 => github.com/cubancorona/fyne/v2 v2.7.4-bt.2`.
  Go's module-path check does NOT apply to replace targets: the fork keeps
  `module fyne.io/fyne/v2` in its go.mod (zero permanent upstream diff),
  and the spike confirmed tidy → build → the fork's code demonstrably in the
  module cache. One constraint: **consumption is by tag only** — a direct
  `go get github.com/cubancorona/fyne/v2@bt-main` fails the path check, so
  every fork change that the app should see gets a new `-bt.N` tag. Published
  tags are immortal (module proxies checksum them) — never move one; cut the
  next number.
- **Option 2 — committed tree.** The fallback if Option 1 ever misbehaves:
  fork synced into a checked-in `third_party/fyne` with a local directory
  `replace` (directory targets skip the path check — it's how the patch
  pipeline works today). ~22 MB one-time history cost. No longer needed on
  current evidence.

Either way the CLAUDE.md invariant survives and improves: `go build ./...`,
`go run ./cmd/desktop`, `go test -race ./...` stay one-line with no setup step
— and now they **test the fork**.

## 3. What gets deleted

The whole regenerate-and-patch machinery:

- `scripts/setup-fyne-patch.sh` shrinks to a version-marker sanity check;
- the patch steps come out of `run-ios-device.sh`, `run-ios-sim.sh`,
  `release-ios.sh`, `build-android.sh` and the three `release.yml` jobs;
- `patches/` reduces to a README pointing at the fork repo;
- the "desktop dev builds use stock Fyne" split ends — the emoji fix and
  everything after it reaches `go run ./cmd/desktop`, closing that parity row.

## 4. Upstream policy — the discipline that keeps the fork cheap

Every fork commit is labelled **[upstreamable]** or **[bt-only]** in its
message. Default is upstreamable; PR them to fyne-io as they stabilise — the
caret blink and the emoji-font modernisation are candidates today, a
configurable draw-loop timeout plausibly too. Every merged PR is a commit we
stop carrying.

Corollary rule: **nothing lands in the fork that could live in the app.** The
fork is only for what structurally cannot be done from outside — exactly the
test the current three patches already pass.

## 5. The engine roadmap this unlocks

In rough value order, each tied to a wound already taken:

1. **Draw loop, properly — BUILT 2026-08-12 on `bt-drawloop-ondemand`.** The
   2ms hack trades battery for latency; the real fix is event-driven
   invalidation. Done: the iOS `CADisplayLink` is now created **parked** and
   Fyne arms it only when its canvas is dirty (`requestDisplay`/`releaseDisplay`
   in `app/darwin_ios.m`, `RequestDisplay`/`ReleaseDisplay` on the `App`
   interface, called either side of `handlePaint`'s dirty branch), so idle
   frames never enter `drawloop` at all.
   The invariant that makes it safe: `Publish()` is an unbuffered rendezvous
   serviced only from inside the display callback, so **every path that paints
   and publishes must arm the link first** — exactly two in the driver
   (`handlePaint` and the backgrounding branch), both compliant.
   Verified: builds host + GOOS=ios; app's full `-race` suite green; simulator
   launch renders, and a tap after 20s idle correctly re-armed and advanced the
   chapter (no deadlock, no freeze); idle CPU 10.7% vs 17.9% for the 2ms build
   (simulator, indicative only). **Still needs a physical iPhone** for the
   actual stutter verdict, battery, and the backgrounding lifecycle path.
   If it holds, fork commit ① (2ms + paint guard) can be dropped entirely, and
   this is the upstream PR the maintainer left the door open to — draw-loop
   efficiency rather than native-overlay support.
2. **Keyboard insets as a driver API.** The Android IME gap (goto picker) is a
   framework hole — Fyne exposes no keyboard geometry. A fork commit adding it
   solves the picker properly on Android, replaces the iOS side-channel
   (`bibleTextKeyboardChanged`), and is a strong upstream PR.
3. **Entry quality:** undo, IME composition, region-based repaint. This is
   what would eventually make the native iOS compose box optional rather than
   necessary.
4. **Selectable text as a first-class widget** — biggest prize, biggest job.
   The styled desktop pane is half of this already, living in the app; the
   fork lets the selection/hit-testing core migrate down into the engine
   where every widget can share it.

## 6. Costs, honestly

- **Rebase debt:** 6–10 upstream events/year × minutes now, growing with the
  stack. Mitigated by small-stack discipline and fork CI; unmitigated if
  bt-only commits accumulate.
- **The engine work itself is hard** — selection/entry touches Fyne's oldest
  code. The fork removes the packaging obstacle, not the difficulty.
- **Repo weight** (Option 2): +22 MB history, once.
- **License:** Fyne is BSD-3. Fork freely, keep the notice.

## 7. Migration checklist (~a day, coordinated)

**Fork-side steps 1, 2, 3 and 6 were EXECUTED 2026-08-12** (this session):

1. ✅ `bt-main` = `v2.7.4` + five commits, pushed, tagged **`v2.7.4-bt.2`**
   (consume this; `-bt.1` predates the CI scoping and was never consumed):
   ① drawloop 2ms + framePainting paint-guard (the SHIPPED patch, which had
   evolved past the June commit `572ea43ad` — the guard against presenting
   half-drawn frames came out of on-device flicker testing), ② discrete caret
   blink, ③ Noto Color Emoji (+OFL licence; EmojiOneColor removed), ④⑤ fork
   CI + invariant tests (`TestBTCaretBlinkIsDiscrete`,
   `TestBTEmojiFontCoversModernEmoji` — both mutation-verified: they FAIL on
   stock). A script-tree diff proved ①–③ byte-identical to what
   `setup-fyne-patch.sh` ships (sole delta: a comment + gofmt fix in
   `bundled-emoji.go`). Verified: app's full `-race` suite green against the
   fork; `fyne package -os iossimulator` builds against it (GOOS=ios compile
   proof); Fyne's own four CI workflows pass on `bt-main`.
2. ✅ `bt-2.8` = the stack rebased onto `v2.8.0`, pushed. Real friction found
   and resolved: upstream rewrote the caret animation for 2.8 (the discrete
   blink is a re-port, not a diff replay, and upstream's new
   `TestEntryCursorAnim` asserts the smooth fade — the fork commit adapts it),
   and `driver.go` gained `updateAccessibility()` after `Publish()` (the
   paint-guard clears before it). This branch is the candidate for the app's
   2.8 move — needs the app-side regression pass first.
3. ✅ Consumption spike — Option 1 verified (see §2).
6. ✅ Fork CI live (`bt-ci.yml`, scoped to the invariants; Fyne's own
   workflows run on the fork too and are the full-suite authority).

**Remaining — the consumption flip (owner + whoever holds the app tree):**

4. Wire `go.mod`: `replace fyne.io/fyne/v2 => github.com/cubancorona/fyne/v2
   v2.7.4-bt.2`; delete the patch machinery (§3); update `CLAUDE.md` +
   `patches/README.md`. NOTE: the in-flight Android new-intent patch must
   either land as fork commit ⑥ (tag `-bt.3`) first, or the flip waits for it.
5. Full matrix on the flipped app: host `-race` suite, sim build, device
   build, Android build, desktop run.

**Coordination:** step 4 touches `go.mod` and every build script. Land as one
commit in a quiet window agreed with whoever holds the app tree — everything
in flight rebases over it.

## Decision points (owner) — updated 2026-08-12

1. **Option 1 vs Option 2** — the spike decided: Option 1 (remote replace)
   works with no module-path rewrite and is now the recommendation (§2).
   Sign-off = approving the flip commit.
2. **2.7.4 first, then 2.8.0** — executed as the lean suggested: the app can
   flip to `v2.7.4-bt.2` as a behavioural no-op (byte-identical engine to
   today's patched builds), and `bt-2.8` sits validated for a separate,
   app-regression-gated move.
3. **Engage upstream? — RESOLVED, and not by filing.** The issue draft in
   `~/Dev/fyne` **was already filed** as
   [fyne-io/fyne#6368](https://github.com/fyne-io/fyne/issues/6368)
   (2026-06-20) and closed the same day after maintainer feedback:
   andydotxyz — "this is not a common pattern … we do not support
   multiplexing with other toolkits", while allowing that on-demand
   rendering "may run parallel to" the GLFW run-loop work (open PR #5422 —
   the subject of `~/Dev/fyne-5422-investigation`). So: do NOT re-file the
   issue or the 2ms-timeout PR — the technical analysis all still holds
   (drawloop files are byte-identical v2.7.4 → develop, verified 2026-08-12),
   but the framing is rejected. The upstream path that remains open is
   roadmap item 1 done properly: an **on-demand iOS rendering PR framed as
   draw-loop efficiency** (idle frames never enter drawloop), referencing
   #6368/#5422. Fyne requires a signed CLA (fyne.cloud/contribute/cla) for
   non-trivial contributions — an owner action. The caret-blink and emoji
   commits remain upstreamable on their own merits, though 2.8's new
   TestEntryCursorAnim asserts the smooth fade, so the caret PR would be a
   behaviour-change conversation (or an option/theme setting), not a
   straight fix.
