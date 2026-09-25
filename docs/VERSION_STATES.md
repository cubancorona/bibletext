# A Bible version, as a state machine

> **Status.** All seven machines are enumerated, and the refresh and arrivals
> layers also carry the TRAJECTORY harness, which walks journeys rather than
> cells, because a promise is broken by a sequence and no single cell is one.
> Together they record **no open** incoherent states. **Every defect the
> scouting reported has been fixed where it lived — twenty-three in all**: `V1`
> (a silent stale-serve), `V2` (its root cause, an unsynced cache write), `D1`
> (a destructive purge on an answer the app could not verify), `D2` (licensed
> text retained with an unbounded lifetime), `D3`
> (a non-default translation stale in silence), `D4` (a banner outliving the
> seed), `D5` (a notice unreachable from the only surface that shows it),
> `D6` (a downloaded Bible discarded because it could not be cached), `D7`
> (a path algebra that could delete a live cache), `D8` (miss branches
> disagreeing about the mode they report), `D9` (**the reader's chosen
> translation erased by a condition that fixes itself**), `D10` (a silent
> substitution of one translation for another) `D11` (a notice retired
> while the edition it describes is still on screen) `D12` (a dead link
> honoured behind the reader's back), `D13` (**someone else's link spending
> the reader's remembered translation**), `D14` (a displaced link discarded in
> silence), `D15` (one translation's search results read and navigated under
> another's name) and `D16` (**a wider canon's trail deleted for reading a
> narrower translation**), `D17` (a previous edition never updated while the
> app ran), `D18` (**the launch dropping the reader's remembered translation
> and the stale mark**), `D19` (the arrival mark spent by a load it was not
> set for), `D20` (a previous edition silent behind the substitution) and
> `D21` (the default's refresh sentences describing a screen that shows
> another translation). Anything a future change breaks appears as an
> unpinned violation, by name.

## Why a state machine and not a checklist

What a version serves is decided by seven functions that each ask a slightly
different question about the same files on disk — *does a cache exist, does it
load, is it current, is it stale, may it be served, may it be deleted, may it
be replaced* — and the reader sees only the answer, never which function gave
it. When two of those questions disagree, the disagreement is invisible: the
reader is simply looking at the wrong text, with no symptom to report and no
control that repairs it.

That is the rule this document exists to hold:

> **The reader is never silently served text that is not the best the app can
> give them — and every state that falls short says so and offers a way out.**

It is the data-layer twin of the shared-link document's liveness rule, and the
`docs/BACKLOG.md` entry that asked for this model named it as the standing
invariant to check. `V1` broke it in shipped code.

## The shape of the thing

```mermaid
stateDiagram-v2
    direction LR

    state "ON DISK — per version" as D {
        [*] --> Absent
        Absent --> Current: a fetch is cached
        Current --> Superseded: cacheEpoch bump
        Superseded --> Current: the upgrade lands
        Current --> Unusable: interrupted write / corruption
        Unusable --> Current: the next successful fetch
    }

    state "WHAT SERVES" as S {
        [*] --> Seed
        Seed --> Real: the full download lands
        Real --> Placeholder: licence goes away
        Placeholder --> Real: a key arrives
    }

    state "THE REFRESH" as R {
        [*] --> Settled
        Settled --> Pending: the default served from seed OR superseded, or a public-domain translation recorded stale (D17)
        Pending --> Downloading: triggerFullDownload
        Downloading --> Settled: applyFullDownload
        Downloading --> Backoff: fetch failed
        Backoff --> Downloading: timer, foreground, picker opened
    }

    D --> S: loadVersionFromCacheOnly
    S --> R: seeded || !versionCacheIsCurrent
    R --> D: a landed download rewrites the current epoch

    note right of Unusable
        V1 lived here: this state
        statted as Current, so the
        refresh never started and
        the picker said nothing
    end note
```

The narrow place is the arrow from `D` to `R`. The refresh decision is taken
by a **second, independent probe** of the same disk state that the serve path
already examined — and for the whole life of the code that probe was
`os.Stat`, while the serve path was a full load. Two questions, two answers,
one reader.

The diagram above is the storage half — M1 to M3. The other four machines and
the arrivals layer, drawn as the enumerations drive them:

```mermaid
stateDiagram-v2
    direction LR

    state "M4 — ACTIVE SELECTION (memory vs disk)" as M4 {
        [*] --> Agree
        Agree --> Disagree: disk moves under a live decode (epoch bump, purge, licence gone)
        Disagree --> Agree: switch re-reads the disk, or the stale decode is retired (D11)
    }

    state "M5 × M6 × M7 — LAUNCH × SAVED POSITION × CANON" as L {
        [*] --> Named: the saved state names a translation and a book
        Named --> Loads: it loads
        Named --> LoadFails: offline, no usable cache
        Named --> SupersededOnly: current epoch gone, previous on disk
        Named --> Unselectable: canSelect is false this launch
        LoadFails --> Fallback: the default serves INSTEAD, choice kept (D9)
        SupersededOnly --> Fallback
        Unselectable --> Fallback
        Fallback --> Said: the screen says which text this is (D10)
        Loads --> Trail: history validated in the canon IN HAND, never the fallback's (D16)
        Fallback --> Trail
    }

    state "ARRIVALS — a promise kept over time" as A {
        [*] --> Parked: a link names another translation
        Parked --> Honoured: the fetch lands, the passage opens
        Parked --> Dead: the fetch fails, with a sentence
        Dead --> Dead: a later switch to that translation must NOT move the reader (D12)
        Parked --> Displaced: another translation loads first — said, not dropped (D14)
        Parked --> Kept: the reader picks a different translation — the park survives (D13)
        Parked --> Previous: the fetch fails and the previous edition serves — recorded and said (D3)
        Previous --> Upgraded: the refresh owes it; the current edition lands in place and the mark clears (D17)
        Previous --> Previous: said behind the substitution sentence too, on a line of its own (D20)
        Kept --> Kept: a landing is the reader's or the link's by the call that started its load, never by a mark another left (D19)
    }

    M4 --> L: what the launch decodes is what M4 then holds
    L --> A: an arrival lands on whatever the launch left in hand
    A --> M4: a link's translation becomes the active one

    note right of Disagree
        the one place the app keeps
        a SECOND copy of a translation
    end note
```

Two of these have no cross-product at all. The launch space is three
machines enumerated together because the erasure lived in their
intersection; the arrivals layer is walked as journeys because a promise is
kept or broken over time and every way it breaks is a sequence.

## State variables

| Variable | Where | What it means |
|---|---|---|
| the cache file at `cachePathForVersion(id)` | disk | the current epoch's text |
| the files at `supersededCachePaths(v)` | disk | previous epochs, newest first |
| `cacheEpoch` | `versions.go` registry | which decoder wrote the current file |
| `AppState.fullPending` | memory | the default translation is owed an upgrade |
| `AppState.seedOnly` | memory | what serves is the 4-book seed |
| `AppState.staleVersions` | memory | the translations recorded as showing a previous edition (`D3`'s mark); with `fullPending`, the refresh's work list (`D17`) |
| `AppState.preferredVersion` | memory | the translation the reader chose when the launch had to show another (`D9`'s record); carried to the live state by `adoptLaunch` (`D18`) |
| `AppState.fullDownloading` | memory | single-flight guard |
| `AppState.fullRetryDelay` | memory | bounded backoff, 0 when settled |
| `dataMode` | memory | `modeReal` vs a `modeTesting` placeholder |
| the credential trio | keychain / env | whether a licensed source is `available()` |

What the refresh owes is `owedUpgrades`: the default while `fullPending`, and
every other translation while it is recorded in `staleVersions`, fetched with
the translation on screen first, then the default, then the rest in registry
order. `fullPending` is still computed for the default alone; the stale mark
is the rest of the list, and before `D17` nothing read it but the picker. Two
kinds of translation are never owed. A licensed one is never served stale
(V-E), and a fetch of it spends the API.Bible monthly quota, which the app
never spends on its own initiative: a remembered NKJV comes back only at a
launch that can revalidate it, or when the reader taps its row. A placeholder
has nothing to fetch. One fetch runs at a time (`fullDownloading`) and one
backoff serves the whole list (`fullRetryDelay`: 20 s, doubling, capped at 10
minutes, and zero when nothing is owed).

## The states

**Absent.** No cache at any epoch. A first run serves the embedded Gospels
seed (`seedOnly`), and the full download is pending. Legal and announced: the
picker says the full text is still downloading.

**Current.** The current epoch loads. Nothing is owed; the refresh is settled.

**Superseded-serving.** The current epoch is missing and a previous epoch
loads. This is a *deliberate* stale-serve — the epoch-migration fallback that
exists so an offline upgrader keeps their whole canon and their history rather
than dropping to a 4-book seed. It is legal **only because** it is announced
and repaired: `versionCacheIsCurrent` is false, so `fullPending` is set for
the default; any other public-domain translation is recorded stale, which the
refresh owes (`D17`). The background upgrade runs, and the picker says an
update is waiting.

**Unusable-current.** A file exists at the current epoch and cannot be served.
Before the fix this state *pretended to be Current* — see `V1`.

**Placeholder.** A licensed version with no credentials. Selectable only in
the sense that the picker explains how to unlock it.

**Licensed-stale.** A licensed cache past the 30-day recency window
(`licensedRecencyWindow`). Never served: the fast path reports a miss so the
load path revalidates. This is the §11 obligation and the one place where
"stale" means *refuse*, not *serve and repair*.

## Transitions

| From | Event | Code | Lands in |
|---|---|---|---|
| any | launch | `loadStartupBible` (`app.go`) | cache hit → **Current**/**Superseded-serving**; miss + saved reading → full load; miss, no history → **Seed** |
| **Absent** | full download lands | `applyFullDownload` (`app.go`) | **Current**, `fullPending` cleared |
| **Current** | `cacheEpoch` bump | registry edit + release | **Superseded-serving** at the next launch |
| **Superseded-serving** | upgrade lands | `loadVersionData` → `purgeSupersededCaches` | **Current**, old epochs removed |
| **Superseded-serving** | fetch fails | `upgradeLanded`, `triggerFullDownload`'s tail | **Backoff** (20 s doubling, capped), notice says "waiting for a connection" |
| non-default **Superseded-serving** | fetch fails in session | `finishVersionLoad` → `ensureUpgradeScheduled` | **Backoff** |
| non-default **Superseded-serving** | upgrade lands | `upgradeLanded` → `applyFullDownload` | **Current**, mark cleared, swapped in place if on screen |
| **Backoff** | foreground, timer, or the picker being opened | `app.go`, `versions_ui.go` | **Downloading** |
| **Current** | interrupted write | `saveBibleToCache` | **Unusable-current** — made unreachable by the fsync (`V2`) |
| **Placeholder** | a key arrives | `keyStore` write → picker re-derive | **Current**/**Absent** for that version, `modeReal` |
| **Licensed-stale** | any load | `licensedCacheStale` → `os.Remove` → refetch | **Current**, or an error — never a stale serve |
| any | purge | `purgeSupersededCaches`, only from inside a *successful* load | previous epochs removed; the current one never touched |

The purge's precondition is the important one, and it is why the enumeration
drives it through `loadVersionData` rather than calling it: purging first, and
discovering the network was down second, is how a reader lost their only copy
once already. Calling the purge directly in a test invents states the app does
not have — and an enumeration that invents states reports defects that are not
real.

## Invariants

These are what `version_state_flow_test.go` enforces. A change that breaks one
is a regression even if every existing test stays green.

- **V-A — Nothing is served that was not loadable.** No path returns text and
  an error together, and no probe reports a file as usable that the serve path
  would reject. *Was violated by `V1`; fixed.*
- **V-B — A superseded serve is always scheduled for upgrade.** If what
  reaches the reader came from a previous epoch, the refresh owes it an
  upgrade (`fullPending` for the default, the stale mark otherwise).
  *Was violated by `V1`, and by `D17` for every translation but the default;
  fixed.*
- **V-C — A stale-serving state is never silent.** Every state in which the
  reader is not looking at the best available text has a notice that says so
  — the picker footer, or the seed banner. *Was violated by `V1`; fixed.*
- **V-D — A purge never removes the only readable copy.** Superseded epochs
  are deleted only after a verified successful load of the current one.
- **V-E — Licensed text is never served past its window.** The recency check
  governs the serve, not merely the refresh.

## Incoherent states

Every entry was reached by driving the real functions. `cells` is how many of
the enumerated combinations reach it — not a count of defects, but a measure
of how much of the space the defect covers. For a defect the journeys found,
it is how many distinct broken promises the walk files under it, one per
world, step and invariant.

| # | Name | Exists today | Cells | What it costs the reader |
|---|---|---|---|---|
| ~~V1~~ | ~~An unusable current-epoch cache serves the previous epoch silently~~ | **FIXED 2026-08-28** | 0 | — |
| ~~V2~~ | ~~A cache write is renamed without being synced~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D1~~ | ~~A credential store that fails to answer is actioned as a revoked licence, and the reader's only copy is deleted~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D4~~ | ~~The seed banner outlives the seed and draws over the complete text~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D5~~ | ~~The waiting notice is unreachable from the only surface that shows it~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D2~~ | ~~Superseded epochs of a licensed translation are retained forever, unreadable and never age-checked~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D3~~ | ~~A non-default translation serving a superseded epoch is stale in silence~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D6~~ | ~~A downloaded Bible is discarded because it could not be cached~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D7~~ | ~~An unregistered version's current cache path appears in its own superseded list~~ | **GUARDED 2026-08-28** | 0 | — |
| ~~D8~~ | ~~The cache-only read's miss branches disagree about the mode~~ | **FIXED 2026-08-28** | 0 | — |
| ~~D9~~ | ~~A translation that is merely unselectable this launch has the reader's choice overwritten by the fallback, permanently~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D10~~ | ~~The reader asked for one translation and is shown another, with nothing on any surface saying so~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D11~~ | ~~A stale version's notice is retired by the disk while its previous decode is still on screen~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D12~~ | ~~A link whose translation failed to load keeps its park, and a later unrelated switch honours the dead link~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D13~~ | ~~A link switching translations spends the reader's remembered fallback choice, durably and in silence~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D14~~ | ~~A link displaced by another translation's load is dropped with nothing said~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D15~~ | ~~Search results survive a switch: old wording under a new name, and a tap writes a dead reference~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D16~~ | ~~A wider canon's history is offered dead after a switch and deleted at the next launch~~ | **FIXED 2026-08-29** | 0 | — |
| ~~D17~~ | ~~A translation shown from its previous edition is not updated while the app runs~~ | **FIXED 2026-09-25** | 0 | — |
| ~~D18~~ | ~~The launch hands its state to the screen field by field and drops the two records the restore makes~~ | **FIXED 2026-09-25** | 0 | — |
| ~~D19~~ | ~~The arrival mark is spent by a load it was not set for, so the reader's own choice is taken for an arrival~~ | **FIXED 2026-09-25** | 0 | — |
| ~~D20~~ | ~~A previous edition behind the substitution sentence: the picker says a translation was shown instead, and nothing about the edition~~ | **FIXED 2026-09-25** | 0 | — |
| ~~D21~~ | ~~The default's refresh sentences say its text is shown while another translation is on screen~~ | **FIXED 2026-09-25** | 0 | — |

### V1 — FIXED 2026-08-28

`versionCacheIsCurrent` asked `os.Stat`; the serve path asked for a full load.
A file that existed and could not be served — a write interrupted after the
rename, a truncated file, a wrong-schema file, a directory at the path —
answered *current* to the first question and *miss* to the second. The
consequences compounded: the epoch-migration fallback served the previous
decoder's text, `fullPending` was false so no upgrade was scheduled, the
picker's notice was empty because it keys off `fullPending`, the picker's
manual retry was inert for the same reason, and the foreground hook was too.
Nothing in the session repaired it — `switchVersion` short-circuits on the
already-loaded map — and the next launch reproduced it exactly. The reader
had no symptom to report and no control to press.

The fix makes the two questions the same question: `versionCacheIsCurrent`
now loads. Measured cost, on the background load goroutine that has just done
the same parse: ~50 ms for the 6.3 MB WEB cache on an M3 Max.

### V2 — FIXED 2026-08-28

`saveBibleToCache` wrote a temp file and renamed it. The rename is atomic
against a concurrent reader but is not a durability barrier: after a power
loss or an iOS jetsam kill the new name can be visible while the megabytes
behind it are not. That is precisely the input to `V1`. The write now fsyncs
the temp file before the rename, so the cache on disk is always either the
previous whole file or the new whole file.

### D1 — FIXED 2026-08-28

`purgeUnavailableLicensedCaches` deleted the current epoch and every
superseded epoch of any licensed version whose `available()` was false. But
`available()` is false in two quite different situations: the reader has no
key, and *the app could not find out*. `secretStore.Read` returns
`(value, found, ok)`, and its own contract says `ok=false` means the store
failed and "CALLERS MUST NOT treat that as 'no key'" — before the first unlock
after a reboot, or on any store error other than item-not-found. Every
consumer that merely reads a key honours that; the one consumer that acted
irreversibly did not.

The cost was the worst shape available: an offline reader, whose licence was
perfectly intact, losing their only local copy of a translation, at startup,
with nothing to restore it from.

The purge now requires a definitive negative — `keyStore.bibleKeyKnownAbsent`,
which answers `false` when the store did not answer at all. The requirement is
scoped to versions whose availability actually turns on the key, so a version
unavailable for a deterministic reason (no operator opt-in, no provider id)
stays purgeable and the §10 removal obligation is unweakened.

### D4 and D5 — FIXED 2026-08-28, and what found them

These two are the reason the suite now has a second kind of harness.

`D4` is a **flow** defect: `applyFullDownload` cleared `seedOnly` on the path
that swaps the text in, and returned early — without clearing it — on the path
taken when the reader has switched to another translation meanwhile. Every
step is individually correct. Only the composition is wrong: fresh install on
the seed, switch away, the download lands while away, switch back — and the
complete text is on screen under a banner still announcing the four-book seed.
The refresh cross-product walked 160 cells and found nothing; the trajectory
walk found it in 310 journeys, because no cell is a sequence.

`D5` is neither a wrong state nor a wrong journey but an **unreachable** one.
The picker fires the manual retry and computes its notice twenty lines later —
and the retry sets `fullDownloading` synchronously while zeroing the backoff,
so both halves of the waiting condition are false by the time it is read. The
wording written for a reader waiting offline could never be shown by the only
surface that shows it. A reachability property needs its own assertion: no
single cell is incoherent, the space is. The picker now reads the notice
first, through `noticeOnPickerOpen`, so the footer reports the situation the
reader came to ask about rather than the side effect of their asking.

### D2 and D3 — FIXED 2026-08-28

`D2` is a **retention** defect rather than a reading one. A licensed
superseded epoch can never be served — the licensed branch returns before the
superseded walk, because stale licensed text must be revalidated rather than
served — and the recency machinery only ever age-checks the *current* epoch.
So the file was unreadable by the app and invisible to the obligation that
governs it: licensed text on the reader's device with an unbounded lifetime
and nothing that would ever look at it again. The startup sweep now removes
superseded epochs of licensed versions unconditionally. The test carries the
control that makes that safe — it proves the file cannot be served before
deleting it — and its twin proves the **public-domain** lane is untouched,
because there the superseded epoch is the offline upgrader's whole canon.

`D3` is a **coupling** defect, and the reason the map matters. M1 knows a
version is serving a superseded epoch; M3 computes `fullPending` from the
default version alone and re-targets it deliberately. Both correct in
isolation. Together they meant a reader restored onto another translation
offline — or switched onto one whose fetch failed — read the previous
decoder's output with no notice, no banner and no upgrade for the entire
session, and it could not self-heal because the switch path short-circuits on
the already-loaded map. This is exactly the silence `V1` was, in a place
`V1`'s fix does not reach.

Staleness is now recorded per version (`AppState.staleVersions`) at both
fallback sites, reported by the picker in the reader's own translation's name,
and cleared by the only thing that repairs it — that version loading its
current epoch. The seed keeps precedence when both are true, because the seed
is what is on screen.

### D6, D7 and D8 — closed 2026-08-28

`D6`: a fetch that succeeded but could not be written was discarded whole.
That made an unwritable cache directory indistinguishable from being offline
at every call site — including the retry loop, which then retried forever at
ten-minute intervals with no possibility of success, on a device where the app
could never open a version at all. The download now reaches the reader for the
session and the failure is logged; the next launch tries the cache again.

`D7` is unreachable in production and a live trap for tests: the current path
is registry-resolved and the superseded list is value-resolved, so an
UNREGISTERED version with an epoch has its current path inside its own
superseded list — and a purge would delete the live cache. Guarded by an
invariant over the real registry, and avoided in the suites by registering
every constructed version.

`D8`: the cache-only read's four miss branches reported two different modes.
Harmless while every caller checks the error first — but one of them already
assigns the returned mode on its success path, so it was one reader away from
mattering. Every miss now reports the same mode, pinned.

### D9 and D10 — the launch machine, closed 2026-08-29

These are the first two found by enumerating **M5 x M6 x M7 together**, and
neither is visible inside any one of them. They are also the first defects in
this document that are **durable**: every earlier one cost the reader a
session, and these rewrite the only copy of something they cannot re-derive.

`D9` is `D1`'s shape in a new machine — a non-definitive answer driving an
irreversible act — and it is the M2 x M6 coupling the map predicted. A
licensed translation stops being selectable whenever its licence configuration
cannot be **read**, and a credential store that has not unlocked yet answers
exactly like one that is empty. On iOS, an app launched before first unlock is
a routine morning. The restore's saved-translation block is skipped entirely
for an unselectable version, so the fallback-remembering line inside it never
ran: `preferredVersion` stayed empty, and the reader's next navigation
persisted the fallback's id over their own. The comment on that line already
promised this could not happen ("without this, one offline launch would
overwrite `nkjv` with `web` permanently") — the promise was true only for the
route it guarded. The choice is now recorded before the block, on the
condition that the translation still exists in the build at all.

`D10` is the silence around the same event. Even on the route that worked, a
reader who asked for one translation and was handed another was told nothing:
the picker put its check mark on the fallback, citations named the fallback,
and the substitution was invisible. The picker footer — the surface that
answers *which translation am I on*, and the one `D3` uses — now says it, and
says the choice is remembered, which is only true because `D9` made it so.

**What the enumeration did NOT find is worth as much.** `L-B`, the
history-erasure invariant that this whole document descends from, is not
violated in any of the sixteen cells — including a 73-book trail meeting a
66-book fallback, the exact shape of the original incident. The guards that
were added by hand hold up under enumeration.

### D11 — the selection machine, closed 2026-08-29

`M4` is the only machine that keeps a **second copy** of a translation — the
in-memory `loadedVersions` map — and a second copy is a second source of
truth. The cross-product asks one question: can the map and the disk disagree
about the same translation, and does anything notice.

They can, at `mem-previous/disk-current`. A version recorded as showing a
previous edition holds that old decode in memory. If the current epoch then
arrives on disk, the switch serves memory — and `applyLoadedVersion` decides
staleness by asking the **disk**, so it retires the notice while the old text
is still on screen. The reader is told the edition is current, and it is not.

**On reachability, plainly.** Nothing in the app today writes a non-default
version's current epoch while that version's previous decode sits in memory:
the background refresh only upgrades the default translation, and a version
already in memory is never re-read from disk. So this is a guard in `D7`'s
sense rather than a live defect — but a far shorter reach than `D7`'s, because
the feature that makes it live is the obvious next one, and `D3`'s notice
already promises it ("until the update can be downloaded"). The fix also
closes a hole that IS live: before it, a non-default translation recorded as
stale had **no way to stop being stale within a session**, however long the
reader spent online — a straight violation of the liveness invariant.

**That last claim was wrong, and the hole stayed open until `D17` closed
it.** The fix lets a stale translation stop being stale once its current
edition is on disk, and the reachability note above is the reason that never
happened: nothing in a session wrote a non-default translation's current
edition while its previous decode was in memory. The walk that showed it is
the arrivals walk on the disk that holds only a previous edition. Since `D17`
the refresh writes that edition, so the reload is live in one window only:
between the refresh's cache write, on its goroutine, and its tail on the UI
goroutine. A switch to the translation made in that window re-reads the disk
rather than serving the old decode, which is right, and the refresh's tail
then finds the mark already cleared and has nothing to swap.

**A note on the enumeration itself.** Its first version passed, and proved
nothing: it routed the generation stamp through `M1`'s `servedFrom` helper,
which maps anything it does not recognise to `"none"`, so every
previous-edition assertion was a tautology. The stamp is now read directly,
and `TestTheSelectionInvariantsCanActuallyFail` hands the invariants the
shapes they exist to catch and requires them to complain — because a green
suite is evidence only when the checks in it can go red.

### D12 — the arrivals layer, closed 2026-08-29

The arrivals layer is walked as **journeys**, not cells, and the reason is in
what an arrival is. The seven machines are enumerated as cells because each of
their defects is a wrong answer to one question. An arrival is not a question,
it is a **promise**: the reader tapped something, and the app owes them either
the passage or a sentence. A promise is kept or broken over time, and every
way it breaks is a sequence — a park that outlives the load it waited for, a
park consumed by an action taken for a different reason, an arrival reported
failed and then honoured anyway. None of those is visible in any single state.

780 journeys to depth four found four distinct broken promises, all one root
cause. A shared link naming a translation not in memory parks its target and
lets that load own the spinner. On the arm where the load fails and the
previous-epoch cache cannot help either, the reader is shown the error — and
the park is left behind. It is not inert: `applyLoadedVersion`'s tail consumes
a park whose id matches, so the reader who later picks that same translation
from the picker, for their own reasons and long after being told the link
failed, is **silently moved to the dead link's passage**. The park is now
closed with the promise, and only ours — a target waiting on a different
translation belongs to another load that still has its own consumer, and
clearing every park would trade this defect for its mirror image. Both halves
are pinned.

The passage is deliberately **not** opened in the translation the reader
already has. Re-applying the target would call `switchToLinkVersion` again and
restart the very fetch that just failed; stripping the version to dodge that
would file the sender's note against wording it was never about — the one
thing `applyShareTarget` is written to prevent. The error card is the answer,
and the link is still in the reader's messages.

**This is the third confirmation that cross-products are blind to flow.** The
refresh machine's 160 cells found nothing and its 310 journeys found `D4`; the
arrivals layer's defects are invisible to every cross-product in this suite
and its 780 journeys found `D12` by four different routes. The existing
`share_link_flow_test.go` cross-product is not made redundant by this — it
asks a different question over the link's own axes ("is any state a dead
end"), and it still passes.

### D13–D16 — what the model missed, found by scouting the same ground again

The journeys above found `D12` and then went quiet, so the four arrival
surfaces were read again from scratch, independently, and each candidate put
to two adversarial verifiers on separate lenses (is it reachable, and does
anything actually go wrong). Four survived, all confirmed here against the
real code before anything was changed. **Two are durable.** They are recorded
together because they share a cause the model had not named: **a reference or
a record made in one translation, read back in another** — and every one of
them arrives through a caller that no machine in the map owns.

`D13` is the sharpest thing in this document, because it is a fix creating an
obligation elsewhere. `D10` made the app *promise*, in writing on the picker,
that the reader's translation is "remembered and comes back when it can".
`applyLoadedVersion` then spent that record on any successful load — its
comment naming two callers, "the reader picked it" and "the licensed one came
back". **A tapped link is neither.** So a reader in the fallback state whose
friend sends them a verse in some other translation lost the only record of
what they had chosen, at the moment of the tap, with the notice going silent
in the same breath. Not their doing, not announced, not recoverable. Switches
an arrival performs are now marked as such, and the preference survives them —
except when the link is *to* the chosen translation, which is exactly it
coming back.

`D16` is the original incident arriving by a door the launch enumeration does
not have. `L-B` proves a trail survives a launch that falls back; nothing
proved it survives the reader simply **changing translation**, which is the
ordinary thing the app is for. An evening in the WEBC leaves Tobit, Sirach and
1 Maccabees in the trail; switching to the WEB persisted that trail under
`"web"`, and the next launch validated it against 66 books and deleted all
three from the only copy. The two cases `restoreRecent` could not tell apart
are not alike — a book dropped from the *build* is gone, a book missing from
the translation the reader *happens to be in* is still theirs. Entries the
app's widest canon knows are now kept dormant rather than deleted, and
`recentJumpTargets` — the single place every renderer of the bar reads — does
not offer what the loaded canon cannot resolve. They come alive again the
moment the reader returns to a canon that has them.

`D15` is the same shape one layer up. A search result list is made of Verses
carrying the **old translation's wording**; it survived a switch, so the
reader read one translation's text under another's name, and tapping a row
navigated by the old numbering into a canon that need not contain the book —
a blank chapter with both arrows dead, written into the reading position and
the history. The mark beside those results was *already* renumbered on every
switch for exactly this reason; the results were the half nothing re-derived.
They are now re-run against the translation in hand, and the navigation
carries the canon guard for every other producer of a Verse.

`D14` is a dead end of precisely the kind `share_link_flow_test.go` exists to
forbid, reached by an axis that enumeration does not have. A link tapped while
some other translation is downloading parks behind that load; when the other
one lands it takes the screen and the park is dropped — correctly, the target
is stale — but until now without a word, and the platform glue has already
told the OS the link was handled, so there is no browser fallback either. Two
taps on shared scripture and nothing whatever happens. The drop was right; the
silence was the defect.

**What this says about the method.** The journeys were necessary and not
sufficient. Their axes were the ones the model already knew to look at, so
they could only find defects among those — `D12`, and nothing further. The
four that follow all live on couplings the map had not drawn: a link reaching
the preference machinery, a switch reaching the search list, a switch reaching
durable history. Re-reading the same ground with no model in hand is what
found them, and adversarial verification is what kept the count honest — a
fifth candidate was refuted on consequence and is not in this table.

**The disk the journeys walk.** Until 24 September 2026 the journeys read the
translation cache of whatever machine ran them: applyLoadedVersion asks
versionCacheIsCurrent, and that decoded the machine's own downloaded file, 849
times a walk. A developer's machine walked a world in which every landed
translation was current on disk, CI a world in which none was, and the walk
took ten minutes under the race detector. The suite now keeps its own cache
(TestMain, `main_test.go`), and the arrivals walk builds the disk an arrival
lands on, so every machine walks the same one. Since 25 September it walks
three: the link's translation with its current edition cached, with only the
edition before it, and with nothing. The second is the arm where the disk
decides an arrival — a failed fetch served by the previous edition and marked
stale (`versions_ui.go`) — and it is where `D17` was found. Since the same day
each disk is walked twice more ways: with the reader's licensed translation
remembered, as the launch's restore records it, and with the reader's own
choice of translation still loading when a link arrives. That is where `D19`
and `D20` were found.

### D17 — FIXED 2026-09-25, found the same day

On the disk that holds only a translation's previous edition, a link to it
whose fetch fails is served that edition: the load tail records it as stale
(`D3`), and the parked passage opens in it. Every invariant checked after a
step holds there — the record agrees with the screen, and the picker footer
names the translation — and it is the right answer for a reader who is
offline.

What does not hold is the promise the footer makes. It says the translation
"is showing a previous edition until the update can be downloaded", and nothing
while the app runs will download it. The previous decode is now in
`loadedVersions`. The picker (`switchVersionInteractive`) and a tapped link
(`switchToLinkVersion`) both treat a translation in memory as loaded and
switch to it without a load. `switchVersion` re-reads the disk for a stale one
(`D11`), but only once its current edition is there, and the one thing that
writes a non-default translation's current edition is a load of that
translation, which a translation in memory is never given. The background
refresh upgrades the default translation alone. So the reader who comes back
online, opens the picker and chooses the translation again is handed the same
previous edition under the same sentence, for as long as the app runs; the
next launch that restores it with a connection fetches it.

The shortest route, on the previous-edition disk, is link-names-other ->
fetch-fails-previous-serves. From there nothing the walk does next puts the
current edition on screen, though the network is up for every step that could.
That is a reachability property, not a state, so it is checked over the walk
(`A-F`, `checkArrivalLiveness`), the way `D5` was. It is not an arrivals defect
in origin: it is the refresh machine covering the default translation only,
meeting the selection machine's second copy. The reader's own choice of the
translation, made offline, lands in the same place. At launch it is quieter
still: the restore marks the previous edition, and the mark does not reach the
screen (`D18`).

`D11` records its fix as closing exactly this — "before it, a non-default
translation recorded as stale had no way to stop being stale within a
session" — and its own reachability note says why it could not: nothing
writes the current edition it waits for. The walk could not see the hole until
it walked this disk. `TestAPreviousEditionIsNotUpdatedWhileTheAppRuns`
reproduced it through the app's own entry points, the picker and a tapped
link, with the network up and no fetch ever made.

**Closed.** The refresh machine's work list is now the default while
`fullPending` and every public-domain translation in `staleVersions`, the one
on screen first (`owedUpgrades`). It is never a licensed translation, so it
spends no API.Bible quota, and never a placeholder. The load tail that serves
a previous edition arms the backoff (`finishVersionLoad` →
`ensureUpgradeScheduled`), so the upgrade waits out one step rather than
fetching again at once; the launch, the foreground hook, opening the picker
and the timer retry it. A landing goes into memory, clears the mark (asked of
memory, not the disk, so a landing whose cache write failed is not owed for
ever) and swaps the text in place under `deferOrRebuild` when the translation
is on screen, with nothing else about the reader changed. The switch itself
still never fetches: the picker and a link treat a copy in memory as loaded,
and serve the previous edition at once, still marked and said, while the
upgrade is on its way. The walk has two events for it, `update-lands` and
`reader-starts-it` (the reader's own choice made offline), and an invariant,
`A-H`: an owed upgrade is always on its way, a fetch in flight or a retry
armed. `A-F` reports nothing stuck. The guard is
`TestAPreviousEditionIsUpdatedWhileTheAppRuns`, by a link, by the reader's
offline choice, with the reader away when it lands, back through the picker
while it is still owed, with the picker opened, and at the launch; it also
checks the licensed and placeholder exclusions and the order.

### D18 — FIXED 2026-09-25, found the same day

The launch machine's cells, and the tests for `D9` and `D10`, read what the
restore decided off the state the restore wrote. The reader never sees that
state. `loadStateData` restores onto one of its own on the load goroutine, and
`StartBackgroundLoad` copies it into the live state field by field — the text,
the translation, the mode, the loaded translations, the book, the chapter, the
trail, the scroll target and the refresh flags. It does not copy the two
records the restore makes: `preferredVersion`, the translation the reader chose
when the launch has to show another, which `D9` writes and which `D10`'s
sentence and the next save read; and `staleVersions`, which carries `D3`'s mark
from the launch's fallback. Both are written, and neither reaches the screen.

So in the launch every platform runs, the desktop and mobile entry points
alike, `D9` and `D10` are open again in effect. A launch that cannot select the
saved NKJV — before the credential store unlocks, or offline past the licence
window — shows the WEB with nothing said, and the reader's next navigation
saves `web` over their choice for good. A translation restored on its previous
edition is shown in silence, which is `D3` at the site its fix was written
for. Driven through the real `StartBackgroundLoad`, the live state holds
neither record after the launch and the next save names the fallback; with
the two fields added to the copy it holds both, the picker says each, and the
save names the reader's translation.

It was found by reading, not by a walk: tracing where `D17` reaches led to the
launch's fallback, and the mark was not on the screen. The launch cells cross
the restore and the tail `loadStateData` runs after it, and stop there; the
seam they do not cross is the one the defect is in.
`TestTheLaunchDropsWhatTheRestoreRecords` pinned it. It ran the restore for
real, through `loadStateData`, to show both records are made, and read the
hand-off from the source, because it runs inside a goroutine a test cannot
await: every field the restore writes on its state must reach the live one,
and exactly these two did not. A record reaches the live state if the launch reads
it off the restore's state anywhere — a copy, a multiple assignment, a clone,
a range, an alias, or any function or method handed that state, however deep —
or writes it on the live state, directly or through a function it hands the
live state to, or one that function hands it on to. The pin failed for each
of those shapes of fix, and would have the day another field was dropped. Its
first version recognised only the one line the copy used, `state.X =
loaded.X`, and stayed green under a fix written as a multiple assignment or a
helper. What it cannot see is a fix that rebuilds the records on the live
state three or more calls deep in new code without reading the restore's: the
search stops at two because three reach `applyLoadedVersion` — through the dev
builds' automatic switch, and a few more through a link consumed at launch —
which writes both records for reasons of its own.

Closing it made live two defects that the dropped preference hid, because
each needs the remembered translation on the live state. They are recorded as
`D19` and `D20` below, and were fixed in the same change.

**Closed.** The hand-off is a named function, `adoptLaunch`, which carries
both records with everything else, and runs before the note on the reopened
chapter, the link tapped at cold start (whose landing reads the remembered
translation under `D13`) and the rebuild. It replaces rather than merges,
because Retry runs the launch again on the same live state and the new
restore is the truth. The launch cells now observe the live state through it,
which closes the seam they stopped short of, and ask two more questions: a
previous edition on screen at launch is said (`L-E`) and owed its upgrade
(`L-F`). The guard is `TestTheLaunchCarriesWhatTheRestoreRecords`: the same
source reading, now requiring that nothing the restore records is dropped,
and `adoptLaunch` run on what `loadStateData` returns, whose live state must
hold both records, say what the restore's state says, and save the reader's
choice. The source half's known blind spot, a rebuild three or more calls
deep, remains.

### D19 — FIXED 2026-09-25, found the same day, live once `D18` was fixed

`versionSwitchForArrival` is how `applyLoadedVersion` tells a link's landing
from the reader's own, and so whether to spend the remembered translation
(`D13`). It is one flag with one reader: whichever translation lands next reads
it and clears it. It is set for a link's load, and nothing ties it to that
load, so two routes hand it to the wrong one.

- **A failed link load leaves it set.** A link whose translation fails to load
  with nothing on disk to fall back on never reaches `applyLoadedVersion`, and
  only that clears the mark. The reader's next choice of their own — the same
  translation or another — is taken for an arrival. Shortest route, on the disk
  with nothing cached: link-names-other -> fetch-fails -> reader-picks-other.
- **A link parked behind the reader's own load gives the mark to that load.**
  The reader picks a translation that has to load; a link tapped meanwhile
  parks behind it (`switchToLinkVersion`, `share_link_open.go`) and sets the
  mark. The reader's load lands first, reads the mark, and is taken for an
  arrival; the park is then dropped and said (`D14`). Shortest route, on every
  disk: reader-starts-other -> link-names-other -> fetch-lands.

Either way the remembered translation is not spent by a choice the reader
made, the picker goes on saying it could not be opened and the reader's choice
is shown instead, and the next save writes the remembered one over the
translation the reader has just chosen: `D13`'s rule turned inside out.
Clearing the mark on the load-error arm closes the first route and not the
second. The mark has to belong to the load it was set for: recorded with the
translation it was set for, honoured only by that translation's landing, and
closed with that load whichever way it ends. The walk found both routes once
it was given the remembered translation and a load of the reader's own left in
flight across a step (`A-G`); `TestTheArrivalMarkIsSpentByTheWrongLoad`
reproduced each.

**Closed.** The field is gone. Who asked is a `switchCause`, `byReader` or
`byArrival`, an argument of `switchVersion`, `switchVersionInteractive` and
`applyLoadedVersion`, so the compiler makes every caller say. The call that
starts a load passes it, the load's goroutine captures it, and its own tail,
`finishVersionLoad`, hands it to its own landing, or it dies with the call
when the load fails. A link that parks behind a running load starts no load,
so it gives no cause, and the running load lands with its own. Nothing is
stored, so nothing can outlive its load or be read by another. The guard is
`TestTheArrivalMarkBelongsToItsLoad`, by both routes, by a link parked behind
the reader's own load of the same translation (the landing is the reader's,
and opens the link's passage), and by the evicted spinner: the reader picks a
translation from the real picker while a link's load is in flight. It also
reads from the source that every switch `switchToLinkVersion` starts is the
arrival's and every one the picker starts is the reader's.

### D20 — FIXED 2026-09-25, found the same day, live once `D18` was fixed

`fullPendingNotice` gives one sentence, and ranks what is on screen first:
another translation shown instead of the reader's (`D10`) outranks a previous
edition of the one shown (`D3`). With the remembered translation on the live
state, a link to a translation that holds only its previous edition, served
that edition offline, is recorded as stale — and the picker says only that the
remembered translation could not be opened and this one is shown instead.
Nothing says the text on screen is a previous edition. It is `D3`'s silence,
reached through `D10`'s sentence, and it holds for as long as the substitution
does. The launch has the same shape: a fallback to the default translation on
its previous edition reports the substitution and not the edition, though
there the background refresh repairs the text unannounced.

The first version of the stale-edition check (`A-E`) asked only that the
footer name the translation on screen. `D10`'s sentence names it too, so the
check passed. It now asks for the edition as well, which is how the walk found
this at link-names-other -> fetch-fails-previous-serves, on the disk that
holds only a previous edition. `TestAPreviousEditionIsSilentBehindASubstitution`
reproduced it.

**Closed.** The footer says every true fact, one per line, in the order a
reader asks: the substitution, then the default's own sentence when the
default is on screen, then the previous edition. The seed keeps its
precedence and stands alone, because it is what is on screen. No sentence was
added or reworded; where one fact holds the footer reads exactly as before.
The guard is `TestAPreviousEditionIsSaidBehindASubstitution`, which compares
the whole footer on the arrival's route and on the launch's, waiting and
downloading.

### D21 — FIXED 2026-09-25, found the same day

The default translation's three sentences — the seed still downloading, an
update waiting for a connection, an update in progress — each say the
default's text "is shown meanwhile", and `fullPendingNotice` returned them
whatever was on screen. That is reachable: a fresh install that picks the BSB
before the WEB lands is told a starter portion of the WEB is shown, and an
upgrader restored onto a current BSB after a WEB epoch bump while offline is
told the WEB's previous edition is. The seed banner was already gated on the
default being on screen (`incompleteBibleBanner`); the footer was not.

It was found placing the default's sentence in `D20`'s composition, which had
to say where it goes and when. The default's sentences are now said only
while the default is on screen, and a reader on another translation with
nothing else true is told nothing. The refresh machine has an invariant for
it, `R-D`, which its cells and journeys ask after every step, and a guard,
`TestTheDefaultsSentencesDescribeOnlyTheDefault`.

## The whole machine — what a complete model must cover

This document began as the storage question and grew into the map below,
because the question a reader actually has is not *which file serves this
version* but:

> **Which text am I looking at, is it the best this app can give me, and does
> everything that names it tell the truth?**

That is seven coupled machines and an arrivals layer, not one machine.
Enumerating them as one cross-product is neither possible nor useful; the
notes model already showed the alternative, running two separate enumerations
rather than one. So each machine below is enumerated on its own, and only the
couplings that are real get crossed.

### The machines

| | Machine | States |
|---|---|---|
| **M1** | Per-version storage *(enumerated)* | absent · current · superseded · unusable · licensed-stale — **a vector over versions, not a scalar** |
| **M2** | Credential and licence *(enumerated)* | unconfigured · bundled · BYOK · cleared-sticky · **unreadable-transient** · recency fresh/expired |
| **M3** | Refresh and download | settled · pending · downloading · backoff(n) · unpersistable-loop — over a work list (`owedUpgrades`) |
| **M4** | Active selection *(enumerated)* | `CurrentVersion` + the `loadedVersions` map — **memory and disk can disagree** |
| **M5** | App lifecycle *(enumerated, with M6+M7)* | loadPending · loadReady · loadFailed · foreground · background · teardown |
| **M6** | Reading position *(enumerated, with M5+M7)* | the saved state NAMES a version that may be gone, unlicensed, or uncached |
| **M7** | Canon shape *(enumerated, with M5+M6)* | 66 vs 73 books, and the renumbering between any two versions |

M2 is the machine with a state that is not a fact about the world but about
**our knowledge of it**: `unreadable-transient` is "we cannot tell", and the
whole of `D1` is one consumer treating it as "no".

M7 is the machine that makes wrong answers look right: `notes_anchor.go`
records in its own header that `MapVerse(webc->web, Tobit 1:1)` reports EXACT
— *and that the table lies*.

### The arrivals layer

Events that arrive from outside and collide with whatever state the machines
are in. Each crosses several machines at once, which is why they are the
richest source of incoherence:

- a **shared link** names a version AND a passage — which may be unavailable,
  unlicensed, undownloaded, or absent from that canon (`linkVersionUnavailable`
  already exists, so the state is real and only partly modelled);
- a **shared note** is STORED under a version — `docs/NOTES_STATE.md` models
  the note side exhaustively and the version side not at all;
- **search results** are version-scoped verse references held across a switch;
- a **cold-start deep link** arrives before the load phase ends (arrivals x M5);
- **audio and read-along** recordings are per version;
- the **footnote apparatus** is per version;
- **cross-references** and **verse-of-the-day** are 66-book data with no
  deuterocanonical entries.

### The surfaces that must not lie

Every degraded state must be visible, so every surface that names or implies a
version is a place a state can lie: the reading pane, the picker rows (check,
greyed, locked tag, TESTING badge), the picker footer notice, the
incomplete-Bible banner, **share citations** (which name the translation to
someone else), note bubbles, and audio availability.

### The invariants a complete model needs

V-A..V-E above are storage-only. The full set:

1. **Truth** — nothing on screen names a version other than the one being shown.
2. **Liveness** — every degraded state offers a way forward and says so.
3. **Non-destruction** — nothing deletes the reader's only copy.
4. **Compliance** — licensed text is never served past its window, nor retained
   without a licence.
5. **One ruler** — every verse number in play is in the numbering of the
   version being read.
6. **No one-way doors** — no state is unrecoverable within a session.
7. **Arrival safety** — an inbound link, note or result never lands the reader
   somewhere they cannot get back from.

### Order of work

Credentials (M2) first: the two destructive defects live there. Then
launch/restore x canon (M5 x M6 x M7), which is where the history-erasure
incident came from. **Both are done, and so is M4**
(active selection — the one machine where memory and disk can disagree). All
seven machines are enumerated, **and so is the arrivals layer** — as journeys
rather than cells, for the reason `D12` records. The model is complete as
mapped.

## What is enumerated, and what is not

Every machine below is driven by a test that fails on any incoherent state
not named in its register, and each test logs its own count when run with
`-v`. The numbers are the tests' numbers, not this document's.

| Space | Test | Cells / journeys |
|---|---|---|
| **M1** storage | `version_state_flow_test.go` | 15 cells — five disk shapes × three events, plus the licensed recency boundary from both sides and the four unusable-file shapes |
| **M2** credentials | `version_credentials_flow_test.go` | 10 cells — five knowledge states (absent, held, unreadable, legacy-only, unreadable-with-legacy) × two events, including the irreversible one |
| **M3** refresh | `version_refresh_flow_test.go` | 160 cells across pending × seed × downloading × backoff × active version, and **310 journeys** to depth 4 from the two starting states a launch can produce; `R-D` asked after every step since 2026-09-25 |
| **M4** selection | `version_selection_flow_test.go` | 8 cells — memory (absent, current epoch, previous epoch) × disk (absent, current, previous), one unserveable combination skipped |
| **M5 × M6 × M7** launch | `version_launch_flow_test.go` | 16 cells — the saved choice (default, wider canon, licensed) × its fate at launch (loads, load fails, superseded only, unselectable) × the saved book (Genesis, or Tobit under the wider canon) — observed on the live state, through the hand-off `adoptLaunch` (`D18`), with `L-E`/`L-F` asked of the two cells that put a previous edition on screen |
| **Arrivals** | `version_arrivals_flow_test.go` | **1142 journeys / 4780 steps** to depth 5 over nine events — a link naming another translation, its fetch failing with nothing to fall back on or with the previous edition serving, the load in flight landing, the reader picking that translation or a different one, the reader picking either while it is still loading at the next step, and the refresh's owed upgrade landing — in six worlds: three disks for the link's translation (its current edition: 107 journeys / 420 steps; only the previous one: 198 / 826; none: 266 / 1144), each with nothing remembered and with the reader's licensed translation remembered. The invariants are asserted after every step and liveness over the walk. A journey is a sequence of events that each did something; the walk used to count events that could not happen in the state they met, such as a fetch failing with nothing loading, which walked the same states again, and that is why 4662 journeys at depth 4 became fewer at depth 5 |

The cells found `V1`, `V2`, `D1`–`D3`, `D6`–`D11`; the journeys found `D4`,
`D12`, `D17`, `D19` and `D20`; the model-free second pass found `D13`–`D16`;
reading the launch's hand-off found `D18`; the refresh cells found `D21` once
`R-D` asked whether the default's sentences describe what is on screen, a
question `D20`'s composition raised. All are closed.

Nothing in the map above is unenumerated. What the enumerations do not
claim: the surfaces listed under *The surfaces that must not lie* are checked
where a test can read them (the picker rows, the notices, the citation
string) and not where only a screenshot can (the reading pane's rendering),
and the per-version data listed under *The arrivals layer* — audio,
footnotes, cross-references — is modelled as availability, not as content.
