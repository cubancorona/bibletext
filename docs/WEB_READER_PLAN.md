# Share-as-link + web reader — plan

Built; see [ARCHITECTURE.md](../ARCHITECTURE.md) for the shipped shape.

> Decisions taken since this draft: Go generator (confirmed), same repo — no deploy
> repo and no subdomain, all three public-domain versions, audio deferred, and the
> reader lives at the site ROOT (`/web/john/3/`) rather than under `/read/`.

2026-08-08, branch `web-reader`, worktree `~/Dev/bibletext-web`. Status: PLAN —
Implementation status and unresolved design choices are recorded below.

## Target architecture

Sharing a verse gains a third option, **Share as link**, producing a URL like

    https://bibletext.co.uk/web/john/3/#v16-18

that opens a fast, elegant, static web reader: navigate versions (WEB / WEB
Catholic / BSB only — never licensed versions), books, chapters; verse
deep-links highlight and scroll; the app's look (parchment/serif/Gelasio,
light+dark, poetry lines, red letters); **no AI**; a quiet platform-aware
"Get the app" link (iOS → App Store, otherwise the landing page). Hosted free
on GitHub Pages.

## Verified constraints (from current GitHub docs, not memory)

- **Static only — the premise was right.** No server-side code, no custom
  headers, no server redirects, no URL rewriting. What exists: a custom
  `404.html`, optional Jekyll (disable with `.nojekyll`), HTTPS on custom
  domains, and arbitrary CI builds *before* deploy. So "all scripting
  client-side" is almost right — the refinement is that heavy processing can
  happen at **build time** in CI; the served artifact stays static.
- **Link previews decide the architecture.** iMessage/WhatsApp/Slack/Facebook
  unfurlers do **not** execute JavaScript. A single-page app can only ever show
  one generic preview card (and with path routing the unfurler receives a 404).
  Per-chapter previews ("John 3 — World English Bible" + verse text) require a
  **real pre-rendered HTML page per chapter**. That, plus deep links that load
  instantly and read with JS disabled, means **static site generation, not an
  SPA**.
- **Scale is a non-issue.** ~3,600 chapter pages ≈ 55 MB — under 10% of the
  1 GB site cap; the 100 GB/month soft bandwidth limit ≈ 7M chapter views.
- **One Pages site per repo**, and the existing bibletext.co.uk is the
  hand-tended gh-pages branch with the load-bearing CNAME file.

## Architecture

**Generator: a Go program in this repo — `cmd/websitegen` — not a JS framework.**
This corrects the initial instinct (Astro/Svelte were evaluated seriously):
the corpus lives in this repo's Go decoders (`bible.go`, `bsb.go`,
`catholic.go`, `red_letter_data.go`, poetry `\n` conventions with tests). Any
external SSG still needs a Go export step *plus* a second implementation of
poetry/red-letter rendering in its template language — permanent drift risk
with the app. A third entry point beside `cmd/desktop` and `cmd/mobile`, using
stdlib `html/template` + the same decoders, has **zero npm dependencies, zero
framework churn** (Astro shipped 3 majors in ~18 months), and compiles
identically in a decade. The site is finished text with almost no
interactivity — an app framework buys nothing here. Client JS is ~two tiny
progressive enhancements: range highlight (`#v16-18`) and the platform sniff
on "Get the app" (single-verse `#v16` highlight works with **zero JS** via CSS
`:target`; no-JS default href is the landing page.)

Maintenance cost: the project owns a micro-SSG (dev preview, cache-busting, publish script)
and design polish is manual CSS. Discipline: decoders in → HTML out, a few
hundred lines, table-driven render tests like `reading_poetry_test.go`.

**Hosting: build here, deploy to a separate repo → `read.bibletext.co.uk`.** ⚖
The generator's output is pushed to a new repo (`bibletext-reader`) whose Pages
site carries the subdomain (one Cloudflare CNAME record; scoped DNS token in
hand). Zero blast radius on the delicate hand-copied bibletext.co.uk flow; own
limits budget; no CNAME file to protect (Actions-mode domains live in repo
Settings). Alternative considered: `bibletext.co.uk/read/` via a unified
Actions deploy of the main site — better long-term shape but its *first* deploy
replaces the entire live tree of the site that must not break; available later
as a deliberate migration. Subdomain URLs are also shorter in messages.

## The URL contract (forever-frozen; links in old messages must never break)

    https://bibletext.co.uk/<version>/<book-slug>/<chapter>/#v<lo>[-<hi>]

- `<version>` ∈ {`web`, `webc`, `bsb`} — exactly the app's version ids; the
  builder hard-falls-back anything else to `web`; licensed ids never emitted.
- `<book-slug>`: lowercase-hyphen from canonical book names (`1-corinthians`,
  `song-of-solomon`; deuterocanon incl. `wisdom`, `sirach`, `1-maccabees` —
  the real `catholic.go` names; Greek Esther/Daniel share `esther`/`daniel`
  so positions survive version switches, and e.g. `daniel/13` exists only
  under `webc`). One **append-only, golden-tested table** (`bookslugs.go`,
  package bibletext) used by both the app's link builder and the generator —
  one source of truth, a rename can never change a slug.
- Verse payload is a **fragment**: `#v16` (works with zero JS via `:target`)
  or `#v16-18` (small script). `?v=16-18` is accepted as an alias (some
  corporate rewriters strip fragments) but never emitted. All-lowercase
  `[a-z0-9/#-]` only — survives every messenger's linkifier; the citation's
  en-dash never leaks into URLs.
- Navigation pages: `/` → version front door; `/<version>/` book grid;
  `/<version>/<book>/` chapter grid. `404.html` is the graceful-degradation
  layer: lowercase retry, canon-aware version rescue (webc-only pages never
  "fall back" to web), friendly find-your-verse page otherwise.
- Cross-chapter selections (future-proofed now): link targets the FIRST
  chapter, span clamped to it — under-highlights rather than lies.
- Behavior contract: bad verse payloads are ignored, never an error page; the
  chapter always renders. Evolution only by adding query params.

## App-side (small, shared, all platforms)

- `share_link.go`: pure `ShareLinkURL(versionID, book, chapter, lo, hi)` with
  golden tests (every book round-trips; lowercase invariant; deuterocanon →
  webc; unknown/licensed version → web; frozen full-URL goldens).
- `shareVerseLink` reuses the existing selection→span provenance
  (`normalizeShareSelection`) so **the link and the citation can never
  disagree**; chapter-only URL as the fallback tier.
- Menu wiring in the three existing surfaces: iOS edit menu
  (`reading_ios.go`), desktop `selectionStudyMenu` (`reading.go`), Android
  (`BtBridge.java` id 108) → `dispatchSelectionAction("share-link")` → the
  normal share sheet with `"John 3:16 (World English Bible)\n<url>"`.
- Cross-platform rule applies: iOS/Android compiles + visual checks before
  merge.

## Milestones

1. **M1 — the contract**: `bookslugs.go` + `share_link.go` + golden tests.
   (Pure Go; freezes the URL scheme before anything depends on it.)
2. **M2 — the generator**: `cmd/websitegen` → chapter pages (typography,
   poetry, red letters, verse anchors + `:target` highlight, per-page OG
   tags), nav pages, `404.html`, `.nojekyll`, the two JS enhancements;
   local preview; render tests.
3. **M3 — deploy**: create `bibletext-reader` repo, Pages via Actions,
   staging check on `*.github.io`, then the DNS CNAME + domain verification;
   DNS changes require explicit authorization.
4. **M4 — app wiring**: the three share menus + share composition; full
   matrix + phone build.
5. **M5 — polish & review**: visual pass against the app side-by-side, docs
   (ARCHITECTURE/README), and an approved landing-page mention.

## Decisions needed ⚖

1. **Domain**: `read.bibletext.co.uk` (recommended) vs `bibletext.co.uk/read/`
   (requires the risky unified-deploy migration of the main site).
2. **Generator**: confirm the Go `cmd/websitegen` recommendation over a JS
   framework (reverses the initial client-framework instinct — reasoning
   above).
3. **Versions at launch**: all three PD versions (recommended) or WEB only?
4. **Defer** client-side search and audio streaming (both fit the contract
   later; the audio mirror even serves range-seekable MP3s) — recommended.
5. New repo name: `bibletext-reader`?

## Known risks

- Micro-SSG scope creep — frozen job description, small file, tests.
- `html/template` escaping around red-letter/poetry spans — deliberate
  `template.HTML` only from decoder output, tested.
- Verse renumbering across data refreshes mis-highlights old links (never
  breaks them — fragments are hints on always-valid pages); regenerate the
  site from the same decode the app ships.
- OG preview *images* per verse are out of scope (text previews only).
