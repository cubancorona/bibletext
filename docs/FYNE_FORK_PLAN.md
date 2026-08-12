# Maintaining a Fyne fork

*Status: approved plan, 2026-08-11. Owner sign-offs still needed on the three
decision points at the bottom. Supersedes the patch pipeline described in
`patches/README.md` once executed.*

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

> **Already in hand (found 2026-08-12).** This repo **exists**: a public fork
> of fyne-io/fyne created in late June, and a local clone at `~/Dev/fyne` with
> both remotes wired (`origin` = the fork, `upstream` = fyne-io). The clone
> carries the drawloop fix as a real commit — `572ea43ad` on branch
> `fix/ios-drawloop-idle-timeout`, based on upstream `develop` (2026-06-19) —
> plus two untracked, **never-filed** upstream drafts: `ISSUE_DRAFT.md` (the
> iOS drawloop main-thread-park bug report, root-caused through
> `darwin_ios.m`/CADisplayLink) and `PR_DRAFT.md` (the fix PR, issue number
> still a placeholder). The commit exists **only on that disk** — it is not
> pushed to the fork. Migration step 1 becomes *reuse*, not *create*: fetch
> upstream in the clone, cut `bt-main` from the release tag, cherry-pick
> `572ea43ad` as stack-commit ①, and push branch + drafts for safekeeping.
> (A separate repo, `~/Dev/fyne-5422-investigation`, documents an earlier
> upstream-issue investigation — evidence the upstream-first workflow in §4
> has been exercised before.)

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

Two candidate mechanisms. **Spike this on day one** — Go's module-path vanity
check on remote `replace` targets has bitten many forks, and five minutes of
empiricism beats an afternoon of documentation archaeology:

- **Option 1 — remote replace.** Checked-in `go.mod` carries
  `require fyne.io/fyne/v2 v2.8.0` +
  `replace fyne.io/fyne/v2 => github.com/cubancorona/fyne/v2 v2.8.0-bt.1`.
  Cleanest if the `/v2` tag and module-path semantics cooperate.
- **Option 2 — committed tree.** The fork synced into `third_party/fyne`
  (checked in; no longer gitignored) with a permanent local `replace`.
  ~22 MB one-time in git history, near-zero deltas after. Bulletproof,
  offline, atomic app+engine commits — an agent can fix an engine bug and its
  app-side consequence in one commit. A sync script pulls from the fork repo,
  which remains where the real git history and rebases live.

Either way the CLAUDE.md invariant survives and improves: `go build ./...`,
`go run ./cmd/desktop`, `go test -race ./...` stay one-line with no setup step
— and now they **test the fork**. Mild preference for Option 2 in a
solo-maintainer-plus-agents shop: it deletes the "which fork tag does this app
commit need?" coordination problem entirely.

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

1. **Draw loop, properly.** The 2ms hack trades battery for latency; the real
   fix is event-driven invalidation in the mobile driver — paint on dirty,
   idle at zero. The measurement harness (the caret CPU numbers in
   `patches/README.md`) already exists to prove it.
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

1. **Reuse the existing fork** (see §1): in `~/Dev/fyne`, fetch upstream, cut
   `bt-main` from `v2.7.4`, cherry-pick `572ea43ad` (drawloop) as commit ①,
   convert the remaining patches + font into their own commits; push `bt-main`
   and the fix branch. Note the stack is growing: an Android new-intent patch
   is in flight in the app repo and would ride along as a fourth code commit.
2. **Rebase onto `v2.8.0` immediately** — upstream moved while we pinned, and
   doing the first rebase while the stack is tiny shakes down the whole
   process. The app then needs a 2.8 regression pass (whoever holds the app).
3. Spike the consumption mechanism (Option 1 vs 2); pick; wire `go.mod`.
4. Delete the patch machinery (§3); update `CLAUDE.md` + `patches/README.md`.
5. Full matrix: host `-race` suite (now on the fork), sim build, device build,
   Android build, desktop run.
6. Stand up fork CI.

**Coordination:** steps 3–4 touch `go.mod` and every build script. Land as one
commit in a quiet window agreed with whoever holds the app tree — everything
in flight rebases over it.

## Decision points (owner)

1. **Option 1 (remote replace) vs Option 2 (committed tree).** Lean: 2. The
   spike may decide for us.
2. **Fork at 2.7.4 first, take 2.8.0 as a separate validated step** (lean), or
   jump straight to 2.8.0 during migration?
3. **Engage upstream early?** An issue/PR conversation with fyne-io about the
   draw loop and a keyboard-inset API before building could save carrying
   anything at all. **The homework is already done**: `~/Dev/fyne` holds a
   finished issue draft and PR draft for the drawloop (see §1) — a "yes" here
   is now a filing action, not a writing project.
