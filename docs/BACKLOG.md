# Backlog

Deferred work, one entry per item. An entry carries enough scope to be picked
up cold; delete it when the work lands.

## Bible version states: transition diagram + comprehensive tests

**Storage space DONE 2026-08-28** — `docs/VERSION_STATES.md` models the machine
and `version_state_flow_test.go` enumerates it (15 cells, every one reached,
zero incoherent states standing). The enumeration found and closed V1 (an
unusable current-epoch cache served the previous epoch silently, with the
refresh switched off and the picker mute) and its root cause V2 (a cache write
renamed without an fsync).

Two spaces remain to enumerate, both scouted with the cells already chosen:

- **The launch space** — storage shape x saved-reading x seed-usability x
  fetch outcome, driven through `loadStartupBible`, which takes its three
  loaders as parameters and is therefore already injectable.
- **The download space** — `fullPending` x `seedOnly` x `fullDownloading` x
  backoff delay, across the apply / retry / foreground / picker-open events,
  built on an `AppState` literal.

### Reported defects — status, severity, and the order to take them

Each was raised while scouting the runtime and cache lanes. **PROBED** means the
scout executed it against a copy of the repo; **TRACED** means it is a code walk
with exact paths but no execution. None is claimed in docs/VERSION_STATES.md,
because that document only names what the enumeration has driven. Confirm each
with a cell as its space is enumerated.

| # | Defect | Evidence | Costs the reader | Fix |
|---|---|---|---|---|
| D1 | `purgeUnavailableLicensedCaches` deletes a licensed version's caches whenever `available()` is false — and an unreadable credential store reads as false | PROBED | Their only local copy of a licensed translation, destroyed by a transient keychain failure; offline, they have nothing | moderate |
| D2 | Superseded epochs of a licensed version are never age-checked and never purged | TRACED | Licensed text with an unbounded lifetime on disk — a §11 recency obligation leaking through a file the §11 machinery never looks at | trivial |
| D3 | A non-default translation served from a superseded epoch is silently stale: `fullPending` is computed for the default version only | TRACED | The previous decoder's text with no notice, no banner and no upgrade for the whole session — the V1 class, in a place V1's fix does not reach | moderate |
| D4 | `seedOnly` is not cleared when the download lands while the reader is on another translation | TRACED, exact sequence | The "showing the Gospels" banner sits over the complete text | trivial |
| D5 | The picker's manual retry makes the "waiting for a connection" notice unreachable — the retry sets `fullDownloading` before the notice is computed | TRACED, exact lines | An offline reader is always told the update is in progress, never that it is waiting | trivial |
| D6 | A successful fetch that cannot be persisted is discarded entirely | PROBED | On a device with an unwritable cache directory the app can never open a version, and retries forever at 10-minute intervals with no possibility of success | moderate |
| D7 | `cachePathForVersion` resolves through the registry while `supersededCachePaths` reads the value handed to it | PROBED | Nothing in production — every version is registered. A live trap for tests: an unregistered version's current path is item [2] of its own superseded list, so a purge would delete the live cache | trivial (guard) |
| D8 | `loadVersionFromCacheOnly`'s four miss branches disagree about the mode they report | PROBED | Nothing today — every caller checks the error first. One reader away from being load-bearing | trivial |

**Order to take them.** D2, D4, D5 are one-line fixes with real value and should
go first — D2 in particular becomes live the moment the NKJV `cacheEpoch` is
bumped, which the entry below schedules. D1 and D3 are the two that cost a
reader something they cannot recover or diagnose, and both deserve a cell in
the enumeration before and after. D6 next. D7 is already mitigated for the new
suite by `withRegisteredVersion`; D8 is a tidy-up to pin whichever answer is
chosen.

## NKJV Psalm superscriptions

The NKJV prints the Psalm titles too, but its API.Bible feed delivers them as
`d` (descriptive title) paragraphs, which decodeAPIBiblePassage currently
skips via apiBibleSkipPara. Rendering them means capturing `d` content into
BibleData.Superscriptions during the passages walk (anchoring any note
markers the way the helloao branch does), bumping the nkjv cacheEpoch, and
nothing else — the renderers and the section already handle titles for every
version. Worth batching with the next NKJV decode change.
