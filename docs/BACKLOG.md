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

### Suspected defects in those spaces — reported, NOT yet verified

Each was raised while scouting the runtime lane and is recorded here rather
than in VERSION_STATES.md, because that document only names what the
enumeration has actually driven. Confirm or dismiss each with a cell.

1. `purgeUnavailableLicensedCaches` deletes a licensed version's caches when
   `available()` is false, and a credential store that is temporarily
   unreadable (before first unlock, or any store error) reads as false. Every
   other consumer of that read deliberately distinguishes absence from
   failure; the one that acts irreversibly may not. Needs a definitive-negative
   predicate, not a falsy one.
2. Clearing your own API key mid-session leaves the reader inside a licensed
   translation they can no longer re-enter: the picker draws the row with both
   a current-version check and an "unlock this" tag, and switching away is
   one-directional. Nothing re-evaluates the active version on a key change.
3. Superseded epochs of a licensed version are never age-checked and never
   served, so nothing removes them until the licence goes away — licensed text
   with an unbounded lifetime on disk. Relevant the moment the NKJV epoch is
   bumped (see the entry below).
4. `seedOnly` is not cleared when the full download lands while the reader is
   on another translation, so the "showing the Gospels" banner can outlive the
   seed.
5. The picker's own manual retry may make the "waiting for a connection"
   notice unreachable — opening the picker starts a download, so the state the
   notice describes ends as it is read. The two offline states may collapse
   into one.
6. A non-default translation served from a superseded epoch is silently stale:
   the whole `fullPending` computation is about the default version only.
7. A successful fetch that cannot be persisted is discarded entirely rather
   than served for the session.
8. `cachePathForVersion` resolves through the registry while
   `supersededCachePaths` reads the value it is handed; for an unregistered
   version the current path can collide with a superseded one.

## NKJV Psalm superscriptions

The NKJV prints the Psalm titles too, but its API.Bible feed delivers them as
`d` (descriptive title) paragraphs, which decodeAPIBiblePassage currently
skips via apiBibleSkipPara. Rendering them means capturing `d` content into
BibleData.Superscriptions during the passages walk (anchoring any note
markers the way the helloao branch does), bumping the nkjv cacheEpoch, and
nothing else — the renderers and the section already handle titles for every
version. Worth batching with the next NKJV decode change.
