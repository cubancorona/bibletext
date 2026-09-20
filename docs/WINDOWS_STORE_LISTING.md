# Microsoft Store listing — BibleText for Windows

The prepared Partner Center listing, kept here the way `docs/PLAY_LISTING.md`
keeps the Play one: every field the console asks for, its value, and the
reasoning where a reviewer will want it. Copy from here into Partner Center;
never compose in the console.

The account was registered on 15 September 2026 as an individual developer
account. Partner Center shows it as Active; the Windows publisher ID and the
public publisher name are both `bibletext.co.uk`. The publisher name is what
the Store shows under the app's name, so it is deliberately the site rather
than a person. Store Policy 10.14 asks for a company account when "a
reasonable consumer would interpret your application or publisher name to be
that of a business entity", and an individual account cannot be converted to
a company one; whether to keep the domain-style name is the account
holder's decision, and it may draw a certification note (see "Risks" at
the end).

## Product identity

The name `BibleText` was reserved on 16 September 2026 as an "MSIX or PWA
app". A reservation lapses if nothing is submitted within three months. The
values Partner Center assigned (Product management → Product identity) are
recorded in `msstore/identity.json`; the manifest is filled from that file, so
the console and the package cannot drift.

| Field | Value | Note |
| --- | --- | --- |
| Product type | MSIX app | see "Packaging" for why not the EXE/MSI route |
| Store ID | `9NDCCZH9RB9K` | `https://apps.microsoft.com/detail/9NDCCZH9RB9K` |
| Package/Identity/Name | `bibletext.co.uk.BibleText` | manifest `Identity Name` |
| Package/Identity/Publisher | `CN=97067B4D-4523-4164-AFFB-B44233E7CCF2` | manifest `Identity Publisher`, verbatim; the Store signs with it |
| Package/Properties/PublisherDisplayName | `bibletext.co.uk` | manifest `PublisherDisplayName` |
| Package family name | `bibletext.co.uk.BibleText_ktgtejn4szjem` | where the app's redirected data lives on a customer's machine |
| Package version | `<Version>.0` from `cmd/bibletext/FyneApp.toml` | the Store reserves the fourth part; a re-upload needs a patch bump |
| Default language | `en-GB` | the manifest's `Resource Language`; the PRI is built for it |
| Category | Books + reference → Reference | secondary category none |
| Price | Free | all markets |
| Privacy policy | `https://bibletext.co.uk/privacy.html` | required (Policy 10.5.1 names Win32 apps explicitly) |
| Support contact | the project support mailbox (`config/product.json`) | |
| Website | `https://bibletext.co.uk/` | |
| Copyright | `© 2026 bibletext.co.uk` | optional field, ≤ 200 characters |

## Store listing (en-GB)

The listing needs only a description and one screenshot to count as
complete; everything else below is what makes it a good page. The
description is plain text, up to 10,000 characters, and may not contain
HTML or URLs, so the source link that the Play listing carries is left out
here (the website field covers it). "What's new" stays blank on a first
submission.

### Description

> BibleText is a clean, unhurried place to read Scripture. No ads, no accounts,
> no tracking — just the text, carefully set, with the tools you use while
> reading.
>
> READ
> • World English Bible, WEB Catholic with the deuterocanonical books, Berean
> Standard Bible, and the licensed New King James Version
> • Words of Christ follow each edition's own publisher markings, and section
> headings and paragraphs follow each publisher's own
> • Adjustable text size, light and dark appearance, poetry set as poetry
> • Chapter navigation, recent history, and Go to for references such as John
> 3:16
> • Works offline after the selected translation has downloaded
>
> SEARCH AND STUDY
> • Search words, phrases, and references across the active translation
> • Follow cross-references and Gospel parallels
> • Optional Study with AI and Find using your own Gemini, OpenAI, Anthropic, or
> Grok provider key; leave Assistant set to None to disable them completely
>
> SHARE AND NOTES
> • Share a passage as citation text, a typeset image, or a link
> • Add a short note to a verse link; it travels inside the link, not through a
> BibleText account or server
> • Browse, search, dismiss, or delete notes on your device
>
> LISTEN
> • Complete public-domain recorded narration for the WEB and the BSB, with
> read-along highlighting and chapter continuation
>
> PRIVATE BY DESIGN
> • No ads, analytics, account, or tracking
> • Full Bible texts and study data are fetched from their documented providers
> and cached; the licensed NKJV is fetched through API.Bible
> • AI requests go directly to the provider selected under the reader's own key
> • Free and open source

### Short description (where the form offers it)

> A quiet, fast Bible reader. Search, study, and read — no ads, no tracking.

### Product features (up to 20, each ≤ 200 characters; the Store adds the bullets)

- Four translations: WEB, WEB Catholic, Berean Standard Bible, NKJV
- Words of Christ in red, publisher headings, poetry set as poetry
- Search, cross-references, Gospel parallels
- Notes carried inside a shared link — no account, no server
- Recorded narration with read-along highlighting
- Optional AI study with your own provider key
- Free and open source; no ads, analytics, or tracking

### Search terms (at most 7, each ≤ 30 characters, 21 unique words in all)

`Bible`, `Scripture`, `NKJV`, `Berean`, `Catholic Bible`, `Bible notes`,
`audio Bible`

No pricing words ("free" is a Policy 10.1.3 violation as a search term) and
nothing that is not in the app.

### What's new

Blank on the first submission. From the second on, the same three-bullet
note the other stores get (`build/appstore/metadata/en-GB/whats-new-<v>.txt`).

## Properties and declarations

| Field | Answer | Reasoning |
| --- | --- | --- |
| Category | Books + reference / Reference | a reader, not a game, not education-only |
| Age rating | the IARC questionnaire, or the IARC rating ID from the Play Console (App content → Content rating) | first submission only; `docs/PLAY_LISTING.md` "Content rating" has the two answers that need a human |
| System requirements | Windows 10 version 2004 (build 19041) or later, x64 | the manifest floor; the app renders through Direct3D, which Windows provides on every machine, so no graphics driver is required; the toolkit draws in OpenGL ES, which the bundled ANGLE libraries translate to Direct3D |
| Incorporates generative AI | Yes | Policy 11.16: the optional Study with AI and Find generate text from the reader's own provider key; the description discloses it; no AI content is stored or shared by the app |
| Accessibility | not declared | the toolkit has no accessibility tree; declaring it would be untrue |
| Pen and ink, non-Microsoft drivers or services | No | |
| Privacy policy | `https://bibletext.co.uk/privacy.html` | |
| Capabilities | `runFullTrust` only | a full-trust Win32 process already runs as the user; `internetClient` and the other AppContainer capabilities do not apply |

## Images

Package tiles and listing logos are generated from the shipped icon,
`cmd/bibletext/Icon.png` (the same mark every other channel carries), by
`go run ./cmd/msstore assets` and `go run ./cmd/msstore listing`; the tests
in `cmd/msstore` hold the committed files equal to a fresh render.

| Image | Size | Where |
| --- | --- | --- |
| Store logo (tile) | 300×300 PNG | `msstore/listing/store-logo-300.png` |
| Box art 1:1 | 1080×1080 PNG | `msstore/listing/box-art-1080.png` |
| Desktop screenshots | PNG, 1366×768 or larger, ≤ 50 MB, up to 10; one is required, four or more recommended; key content in the top two thirds; no added logos or marketing text; optional caption ≤ 200 characters | `docs/screenshots/windows/` — four at 1600×960 |

Screenshots must come from the Windows build itself; the Store rejects
composed or foreign-platform images. They are captured on the runner rather
than by hand: `.github/workflows/windows-screenshots.yml` raises the runner's
screen from 1024×768 to 1920×1080 (`Set-DisplayResolution`), builds the
release executable with software graphics beside it, opens it with links
minted by the app's own link code, and saves the WINDOW's client area — no
desktop, no frame, no evaluation watermark — for four scenes: a note received
inside a shared link under its section heading, a passage, search results,
and the settings sheet. Dispatch it, download the artifact, replace
`docs/screenshots/windows/` (`docs/screenshots/README.md`), then upload them
in Partner Center. Different scenes are a change to
`scripts/capture-windows-screenshots.ps1`.

## Packaging

The GitHub release ships `BibleText-Windows-amd64.zip` (one `BibleText.exe`
inside), built by the Windows job of `.github/workflows/release.yml`;
`scripts/build-windows-exe.sh` is the same recipe, held line-identical to
that job by `scripts/test-release-key-flow.sh`. The Store wants either an MSIX
package or an EXE/MSI installer hosted by the developer and signed with the
developer's own certificate. The MSIX route needs no certificate (the Store
re-signs the package with the identity above) and hosts the binary in the
Store, so it is the one taken. `fyne release -os windows` produces an .appx
whose manifest is not ours to shape, so it is not used.

`.github/workflows/msstore.yml` (windows-latest; on demand, and on pushes to
`main` that touch the packaging inputs or the link code):

1. builds `BibleText.exe` with the same recipe as the release;
2. runs the `cmd/msstore` tests, then fills `msstore/AppxManifest.xml.in`
   from `msstore/identity.json` and the desktop ledger, and lays out the
   exe, the manifest and `msstore/Assets/` under `build/msstore/layout/`;
3. runs `makepri` for the `en-GB` qualifier set and `makeappx pack`, and
   uploads the unsigned `BibleText-Windows-x64.msix` as the run's artifact;
4. smoke-installs a COPY of the package as it ships, signed with a
   throwaway certificate whose subject is the reserved publisher:
   `Add-AppxPackage`, activation through the shell, and then the app must
   have a window with something drawn in it, because a graphics failure
   leaves the process alive and blank; then the link checks under "Links"
   below. The Store package carries no signature of ours.

Manifest choices (also in the template's own comment): `Windows.Desktop` from
`10.0.19041.0` with `uap10:RuntimeBehavior="packagedClassicApp"` and
`uap10:TrustLevel="mediumIL"` (that form needs the 19041 floor); `runFullTrust`
alone; `BackgroundColor="transparent"` so the tile shows the icon's own
plate. The Store requires the package revision (fourth version part) to be 0,
so the package version is the ledger's `Version` with `.0`, and re-uploading
for the same app version means a patch bump: one version, one tree, on every
channel.

**Links.** The manifest declares two handlers under the application's
`Extensions`, both tested by `cmd/msstore`:

- a web-to-app handler (`uap3:AppUriHandler`) for `bibletext.co.uk`, so an
  `https://bibletext.co.uk/...` link launched through the shell (Outlook,
  Teams, the Run box, anything that calls ShellExecute) opens the app at the
  verse. The site consents in `/.well-known/windows-app-web-link` (source
  `docs/windows-app-web-link`, published by `scripts/publish-site.sh` at the
  root as well); Windows fetches that file itself and sees a change between the same
  day and eight days later; the Store checks nothing. Links clicked inside Edge, Chrome or Firefox
  stay in the browser, by Microsoft's design, and the reader chooses the
  default app under Settings → Apps → Apps for websites (there is no per-app
  switch to promise; an administrator can disable the feature device-wide).
- the `bibletext:` scheme (`uap3:Protocol`), the only route from a browser
  click on any desktop and what the site's "Open in BibleText" button uses.
  The payload is the https link with the scheme swapped.

Both put the URL on `BibleText.exe`'s command line (`desktop2:Parameters`
and `Parameters`); `share_link_argv.go` reads it once at start. A second
launch while the reader is open hands the link to the running instance over
loopback and exits (`single_instance.go`; the record lives at
`%LocalAppData%\bibletext\single-instance.bibletext.store.json`, which MSIX
redirects to `…\Packages\<package family name>\LocalCache\Local\bibletext\`). "Read it in the browser" starts the
default browser directly rather than through the shell, or the handler would
catch the app's own link; an echo guard bounds the loop if that fallback is
ever taken. The smoke job checks the installed declarations, validates the
consent file with Windows' own verifier, activates the scheme cold (the
command line must carry the URL and its fragment), activates it again while
running (one process must remain), and tries an https link through the shell
(advisory on the server runner).

What a packaged full-trust app changes at runtime: the install directory is
read-only, so nothing may be written beside the exe (BibleText writes only
under the user's config and cache directories); new files under
`%LocalAppData%` and `%AppData%` are redirected to
`%LocalAppData%\Packages\<package family name>\LocalCache\Local\` and
`…\LocalCache\Roaming\`, which is where the Bible cache, the preferences
and the notes land for a Store install and what an uninstall removes. Nothing in the app assumes otherwise.

## The next submission should carry both architectures

Since 1.2.12 the release builds an **arm64** MSIX as well as the x64 one, and
`msstore.yml` builds and smokes both. Neither has been submitted: that workflow
uploads artifacts and nothing more, and the Store has not moved since 1.2.10.

So the next submission should include **both** packages. Partner Center accepts
several `.msix` files in one submission, or they can be combined into a single
`.msixbundle` — that is the decision to make when the submission is prepared.

The arm64 package is not speculative. It is built with a pinned
aarch64-targeting toolchain (`scripts/fetch-llvm-mingw.ps1`, needed because the
`windows-11-arm` runner image ships an x86_64 gcc, so cgo cannot otherwise
assemble aarch64), and on 19 September 2026 it was verified on a real ARM
runner: the manifest declares `ProcessorArchitecture="arm64"`, and
`BibleText.exe` with all three bundled ANGLE libraries are genuine ARM64
images. It installed and passed the render smoke.

Windows on ARM already runs the x64 package under emulation, so this is a
performance and battery improvement, not a fix for something broken.

## Submission (first release, by hand)

The first submission cannot be made through the API (name reservation, the
age rating and the first publish are console-only), so it follows this
document in Partner Center:

1. **Packages**: upload `BibleText-Windows-x64.msix` from the workflow run.
   The console checks the identity, version and manifest against the
   reservation.
2. **Properties**: category, the declarations table above, system
   requirements, privacy policy URL, website, support contact.
3. **Age ratings**: the IARC questionnaire (or the Play IARC rating ID).
4. **Store listing (en-GB)**: description, product features, search terms,
   screenshots, store logo and box art, copyright.
5. **Pricing and availability**: Free, all markets; publishing hold set to
   "Don't publish this submission until I select Publish now" so the first
   release goes live on the owner's word, not the certifier's clock. Verify
   the account email in Action Center first or the certification mail never
   arrives.
6. **Submission options → Notes for certification** (dated):
   > BibleText is a classic Win32 desktop application (Go, Fyne toolkit)
   > packaged as MSIX; runFullTrust is required for that packaging form. It
   > runs as the invoking user, asks for no elevation, installs no service,
   > driver or secondary software, and writes only to the user's config and
   > cache folders. No login: reading, search, notes and narration work
   > without any account. The NKJV translation is fetched from API.Bible
   > with a bundled key. The optional AI study features are off until the
   > reader enters their own provider key under Settings → Assistant; they
   > generate text from that provider and store nothing. The app renders
   > through Direct3D by way of the bundled ANGLE libraries, so it runs on a
   > machine with no graphics driver.
7. Submit. Certification takes up to three business days; after it passes,
   Publish now.

## Published

BibleText went live on the Microsoft Store on 17 September 2026 at 21:29 UTC.
Submission 1 passed certification and published itself: the publishing hold was
set to publish on passing rather than to wait for a manual Publish now, so
there was no second step. The API is the plainest confirmation —
`firstPublishedDate` is no longer the 1601 never-published sentinel,
`pendingApplicationSubmission` is gone and `lastPublishedApplicationSubmission`
is submission 1. The listing answers at apps.microsoft.com/detail/9NDCCZH9RB9K.

Two things change from here. The name reservation stops being a clock. And the
submission API's write side is unblocked: `POST /submissions` clones the last
published submission, and there is one at last, so the next Windows release can
go create → upload to the SAS URL → commit → poll rather than through a browser.
The section above records why the first package could not.

1.2.11 was deliberately not submitted. Nothing in it reaches a Windows reader —
the appearance fix is Android's and the universal build is the macOS direct
download — and the Store had published hours earlier. The next change that
touches Windows carries the number.

## Submission 1 — what is filled and what is not

Submission 1 exists in Partner Center as a draft. Four of its six sections
are Complete; two are deliberately untouched.

Filled:

- **Pricing and availability** — base price £0 (GBP, United Kingdom) so the
  product is free, all worldwide markets plus future markets at the base
  price, public audience, discoverable, release as soon as possible, stop
  acquisition never. No free trial, no sale pricing. Organizational
  licensing left at its default: volume acquisition allowed, offline
  licensing not allowed.
- **Properties** — Books + reference / Reference, no secondary category.
  Privacy question answered Yes with `https://bibletext.co.uk/privacy.html`,
  because the app stores a reader-supplied provider key and sends it to the
  provider they choose. Website `https://bibletext.co.uk`, support
  `https://bibletext.co.uk/support.html`; the postal address and phone
  fields are left empty because the Store prints them on the listing. The
  generative-AI declaration is checked; accessibility is not. The
  record-and-broadcast declaration, checked by default, is cleared: the
  console warns that it only applies to the Games category. No hardware
  feature, memory, DirectX, processor or graphics requirement is declared —
  the OS floor comes from the manifest and the app renders through Direct3D
  on any machine.
- **Store listing (en-GB only)** — description, seven product features,
  four screenshots from `docs/screenshots/windows/` in the order reading,
  search, note, settings with a caption each, 1:1 box art, 300×300 app tile
  icon, short description, the seven search terms, copyright
  `© 2026 bibletext.co.uk`. What's new is blank, as a first submission
  should be. "Developed by" is blank, so the Store shows the publisher
  display name.
- **Submission options** — publishing hold set to "Don't publish this
  submission until I select Publish now".
- **Additional Testing Information → Notes for Certification** — the note in
  step 6 above. This is where that note lives; the Submission options page
  only links to it. No credentials are entered; the app has no login.

Not filled:

- **Packages** — the version block is cleared. 1.2.9 was spent, so 1.2.10 was
  cut (annotated tag, both ledgers, mobile build 180, desktop build 51) and
  `.github/workflows/msstore.yml` was dispatched **at the tag**, never at the
  branch: the MSIX version is the desktop ledger plus `.0`, so a run against a
  moved branch would label a package for a tree the tag does not name. The
  artifact was checked before upload — `Version="1.2.10.0"`, the reserved
  Identity and Publisher, and the three ANGLE libraries inside. Attaching it
  is the one step that cannot be automated from here: the package is 28 MB and
  the browser bridge carries at most 10 MB per file, so it is dragged into the
  Packages page by hand. The device family (Windows 10/11 Desktop) is set.
- **Age ratings** — the IARC questionnaire needs the account holder's own
  answers, or the existing IARC rating ID from the Play Console under App
  content → Content rating.
- **Submit for certification** — the account holder's to press.

Skipped on purpose: 9:16 poster art, 16:9 super hero art, the Xbox images
and trailers (none exist, and none is required for a Windows-only listing);
the 150×150 and 71×71 store display images, so the package's own tiles are
used; short title and voice title, which are Xbox fields; additional licence
terms, since the Standard Application License Terms are unchanged.

## Automation after the first release

The Microsoft Store submission API needs a Microsoft Entra tenant
associated with the account and an application registered in that tenant
with the Manager role. Both exist since 16 September 2026: the tenant
`bibletext.onmicrosoft.com` (created through Partner Center's own business
sign-up, an Entra ID Free tenant with a separate work account; Partner
Center itself stays on the personal account) and the application
`bibletext-store-api` (Account settings → User management → Microsoft Entra
applications, role Manager). Its key was generated there, shown once, and
went straight into the login Keychain with the tenant and client ids:

| Keychain service | account |
| --- | --- |
| `uk.co.bibletext.msstore` | `tenant-id`, `client-id`, `client-secret` |

`. scripts/msstore-env.sh` exports them (no credential is printed), the way
`scripts/asc-env.sh` does for App Store Connect. `msstore/msstore.py`
(standard library) takes a client-credentials token for the classic Dev
Center resource and reads the account: `apps` lists every product with its
pending and published submission ids, `app <store id>` prints one product,
`submission <store id> <id>` prints a submission's status. The first call
listed `9NDCCZH9RB9K BibleText`, which proves the tenant association, the
role and the key. The write side (create a submission, upload the package to
its SAS URL, commit, poll) is written against the second release, once the
first has gone through the console: name reservation, the age rating and the
first publish are console-only. Microsoft's own command-line tool (`msstore`, from
github.com/microsoft/msstore-cli; it runs on macOS on .NET) answers the same
reads with `msstore apps list` and is the cross-check when this client and
the console disagree. Rotating the key is the same console path; the Keychain item
is replaced and nothing in the repository changes.

### Why the first package is attached by hand

Tested against the live account on 17 September 2026 rather than inferred.
The API reads this submission perfectly: a GET of
`applications/9NDCCZH9RB9K/submissions/<id>` returns twenty-four fields —
the en-GB listing, the notes for certification, `targetPublishMode:
Manual` — every value the console holds. What it does not return is
`fileUploadUrl`. The field is absent, not empty, and `applicationPackages`
is `[]`. That URL is the SAS address a package is uploaded to, and the API
issues one only for a submission the API itself created.

Creating one is the part that cannot work yet. `POST .../submissions`
"creates a new in-progress submission, which is a copy of your last
published submission", and this product has never been published — the
account reports `firstPublishedDate: 1601-01-01`, the never-published
sentinel. There is nothing to clone, so the call is the documented 409.
Only one pending submission may exist at a time besides, and this one
belongs to the console.

So the ~28 MB package is dragged into the Packages page by hand, once. The
browser bridge used for the rest of the form carries at most 10 MB per
file, which is a limit of that tool, not of the Store. The package is
uploaded unsigned on purpose: the Store re-signs MSIX with the publisher
identity the reservation issued.

**Do not run Microsoft's own Python submission sample against this
account.** It opens its app-submission flow by DELETING
`pendingApplicationSubmission`, and StoreBroker does the same behind
`-Force`. Either would destroy this submission and the five sections
already completed in it. StoreBroker's `-SubmissionId` switch is the
documented way to target an existing pending submission instead.

From the second release on, none of this applies: with one submission
published, `POST` has something to clone, the SAS upload works, and the
write side becomes create → upload → commit → poll. Microsoft's own
`msstore` CLI is the alternative it recommends for CI, and it handles MSIX
and runs on macOS; note that its update path is currently restricted to
free products, which this one is.


## Resuming with a Windows machine

State on 16 September 2026, so the work can be picked up cold. Everything
below the line "verified" was observed; everything under "to verify" needs a
Windows 11 client (the runner is Windows Server) and is the reason to sit
down at one.

**Verified, on the windows-latest runner (Windows Server 2025, build
26100; `.github/workflows/msstore.yml`, run 35076702153 of 16 September
2026):** the package builds by the release's own recipe; a copy signed with a
throwaway certificate installs under the reserved identity, and the
installed manifest carries both link handlers; the consent file passes
Windows' verifier (exit 0, and exit 1 for a deliberately broken copy); a
cold `bibletext://…/web/john/3/#v16` activation starts the app with the full
URL, fragment included, on its command line — the smoke asserts the command
line, and that the reader landed on John 3:16 was read off the run's
screenshot, kept as `docs/windows-store-smoke-john3.jpg`; a second
activation while running hands off to the first process and exits. An https
link launched through the shell did NOT open the app on the server edition,
which is expected there and settles nothing. On the Mac, the handoff and the
intake are covered by `go test` and `go test -tags bibletextdev`
(`share_link_argv_test.go`, `single_instance_test.go`,
`single_instance_dev_test.go`); the Store package tests run in CI on every
push. Run logs and artifacts expire after 90 days, so the numbers above are
history, not something to fetch.

**To get the package onto the machine:** download the
`BibleText-Windows-x64-msix` artifact from the latest green run of the Store
workflow (Actions → Microsoft Store package). Artifacts live 90 days and the
workflow runs only on pushes that touch its inputs, so if no live one
exists, start it by hand (Run workflow) and take that run's artifact; it
carries the desktop ledger's version of that tree. Then, in a PowerShell 7
started with Run as administrator under your own account, at the
repository root, with the Windows SDK installed:

```powershell
pwsh scripts/msstore-sideload.ps1 -Package .\BibleText-Windows-x64.msix
```

It signs a copy with a throwaway certificate (the artifact itself stays
unsigned, as the Store wants it), trusts the certificate's public half,
installs, and prints the launch, scheme and https commands and the record
file's location. `-Uninstall` removes package and certificate; `-Mesa <dir>`
adds software OpenGL for a virtual machine with no GPU driver. A sideloaded
package has its web-to-app links validated at install without the site's
consent file being fetched (Microsoft's rule for sideloads), which shapes
step 4 below.

**To verify, in this order** (each line names what it settles):

1. Plain launch from Start; open a chapter; quit; launch again — the app
   runs from `C:\Program Files\WindowsApps\…` and keeps its reading position
   (data under `%LocalAppData%\Packages\<package family name>\LocalCache\`).
2. Paste `bibletext://bibletext.co.uk/web/john/3/#v16` into Win+R (the
   Run box takes a bare URL; `start` is a cmd built-in, so in cmd it is
   `start "" "bibletext://…"` as the smoke runs it, and in PowerShell
   `start` is Start-Process) with the app CLOSED, then again with it OPEN
   and minimised — cold start opens John 3:16; the warm one comes to the
   front un-minimised, one process in Task Manager
   (`AllowSetForegroundWindow`, `SW_RESTORE`).
3. The notice page's "Open in BibleText" button (any `/nkjv/…` chapter on
   the site) in Edge, Chrome and Firefox, twice: (a) with the app installed
   — each browser's own prompt for an unknown scheme, whether it offers
   "always allow", then the app; (b) after `-Uninstall` — whether the
   browser shows nothing at all, which is what the hidden "Get BibleText"
   line beneath the button assumes (`cmd/websitegen/notice.go`), or a
   dialog. Reinstall afterwards.
4. The web-to-app handler, in two passes. (a) Paste
   `https://bibletext.co.uk/web/john/3/#v16` into Win+R, and click an https
   link to the site in Outlook or Teams — a sideloaded package has these
   links validated at install, so this settles `desktop2:Parameters` on
   `uap3:AppUriHandler` and the handler firing on a client, and nothing
   about the site's consent file. Settings → Apps → Apps for websites
   should list BibleText for bibletext.co.uk. The same link clicked INSIDE
   Edge or Chrome must stay in the browser. (b) Set the registry value
   `ForceValidation` = 1 (DWORD) under
   `HKCU\Software\Classes\LocalSettings\Software\Microsoft\Windows\CurrentVersion\AppModel\SystemAppData\<package family name>\AppUriHandlers`
   (the family name is in `msstore/identity.json`), reinstall, and repeat —
   only this pass, or the Store's own install after certification, shows
   whether the consent file as GitHub Pages serves it
   (`application/octet-stream`) is accepted; Windows re-reads it between
   the same day and eight days later.
5. In the app, a note-bearing link with notes switched off → "Read it in
   the browser" — the default browser opens the link directly (the
   association query, not ShellExecute) and the app does not relaunch
   itself; check with Edge, Chrome and Firefox as the default in turn.
6. Book-index link `bibletext://bibletext.co.uk/web/john/` — the app opens
   the browser at that index rather than showing nothing (invariant I2).
7. A machine or VM with no graphics driver: the app must still start and
   draw, because it renders through Direct3D. The runner proves this on
   every Store package build; a real virtual machine is the confirmation.
8. The direct-download build on the same machine: run the release zip's
   `BibleText.exe` once and note whether Windows Defender Firewall prompts
   for its loopback listener (the package never prompts; the bare exe is
   the open question), then launch it a second time with a `bibletext:`
   link — the direct channel's record is
   `%LocalAppData%\bibletext\single-instance.bibletext.direct.json` and the
   handoff must work there too.
9. Screenshots: only if the runner-captured set in `docs/screenshots/windows/`
   needs a scene the capture script cannot drive — see "Images".

**Then, in this order:** the
screenshots; the first submission by hand from this document with the
publishing hold on; after it is live, the write side of `msstore/msstore.py`
against the second release. The Linux half has its own checks: `make
user-install` from the release tarball, `xdg-open bibletext://…`, the
browsers' prompts, and whether the window comes to the front under
XWayland (the toolkit asks X11 for it; the compositor decides), with the
desktop entry's `Exec=bibletext %u` and `MimeType=x-scheme-handler/bibletext;`
already asserted by the release job.

## Risks

- **Graphics — settled 17 September 2026.** A Windows machine with no
  graphics driver offers only the generic OpenGL 1.1, on which the app used
  to fail at start-up; whether Microsoft's certification hosts are such
  machines is still unknown, and the GitHub runner is. The app now renders
  through Direct3D, which Windows provides everywhere, by building Windows
  with the toolkit's OpenGL ES path and bundling ANGLE, as Chrome, Firefox
  and Qt do. Proven on the runner both bare and inside the installed
  package, with a control that fails when the libraries are removed. What
  remains unproven is only the frame cost on a real graphics card, which
  the Windows sitting will show.
- **Publisher name on an individual account** (Policy 10.14, above).
- **Unverifiable without a Windows 11 client**: that `desktop2:Parameters`
  on the web-to-app handler is honoured for a packaged classic app (the
  smoke's scheme step hard-fails only on the `uap3:Protocol` `Parameters`;
  the https step that would exercise the handler is advisory on the server
  runner), that the web-to-app handler fires with the consent file
  served by GitHub Pages as `application/octet-stream` (Microsoft documents
  no content type; Outlook's own file is served the same way; a plain
  sideload proves nothing here because sideloaded links are validated
  without the file — `ForceValidation`, step 4b above), the browsers' prompt
  for the `bibletext:` scheme, the foreground grant, and Partner Center's
  reaction to the two declarations.
- **Policy 11.16** requires the generative-AI declaration and disclosure in
  the description; both are in place. Policy 10.3 asks that a product be
  testable: the notes for certification say how, and API.Bible must be up.
