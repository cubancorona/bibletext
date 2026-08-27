# Backlog

Deferred work, one entry per item. An entry carries enough scope to be picked
up cold; delete it when the work lands.

## Bible version states: transition diagram + comprehensive tests

Model the full life of a translation as an explicit state machine — in the
NOTES_STATE.md mould — and pin every transition with tests. The machinery has
grown organically (epoch fallbacks, licensed recency windows, BYOK gates, the
background refresh with backoff) and each piece is individually tested, but
no single artifact enumerates the states or proves the transition table
complete.

States to enumerate (at least): absent · embedded Gospels seed ·
superseded-epoch cache (the stale-serve fallback) · current-epoch cache ·
downloading · retry-backoff (offline) · placeholder (evaluation, not
selectable) · BYOK-locked / BYOK-unlocked · licensed-fresh ·
licensed-stale (past the recency window, revalidate-don't-serve).

Transitions to cover: launch (versionCacheIsCurrent), epoch bump, fetch
success / fetch failure (stall watchdog, fetch_stall.go), foreground
re-entry, picker-open manual retry, API key added / cleared, licence recency
expiry, version switch, purge ordering (purgeSupersededCaches only after a
verified current cache).

Evidence anchors today: versions.go (loadVersionFromCacheOnly,
loadVersionData, supersededCachePaths, purgeSupersededCaches), app.go
(fullPending / triggerFullDownload / applyFullDownload and the bounded
backoff), fetch_stall.go, versions_ui.go (fullPendingNotice).

Testing shape: a table-driven transition suite (state × event → state,
exhaustive over the enumeration) plus the standing invariants as their own
pins — a stale-serving state is never silent (a visible notice exists);
licensed content never serves stale; a purge never destroys the only local
copy; every fallback path is nil-safe for pre-field caches.
