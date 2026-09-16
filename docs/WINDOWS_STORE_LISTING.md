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
| Package version | `<Version>.0` from `cmd/desktop/FyneApp.toml` | the Store reserves the fourth part; a re-upload needs a patch bump |
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
| System requirements | Windows 10 version 2004 (build 19041) or later, x64; a graphics driver with OpenGL 2.0 | the manifest floor; the toolkit renders through OpenGL |
| Incorporates generative AI | Yes | Policy 11.16: the optional Study with AI and Find generate text from the reader's own provider key; the description discloses it; no AI content is stored or shared by the app |
| Accessibility | not declared | the toolkit has no accessibility tree; declaring it would be untrue |
| Pen and ink, non-Microsoft drivers or services | No | |
| Privacy policy | `https://bibletext.co.uk/privacy.html` | |
| Capabilities | `runFullTrust` only | a full-trust Win32 process already runs as the user; `internetClient` and the other AppContainer capabilities do not apply |

## Images

Package tiles and listing logos are generated from `icon/full.png` by
`go run ./cmd/msstore assets` and `go run ./cmd/msstore listing`; the tests
in `cmd/msstore` hold the committed files equal to a fresh render.

| Image | Size | Where |
| --- | --- | --- |
| Store logo (tile) | 300×300 PNG | `msstore/listing/store-logo-300.png` |
| Box art 1:1 | 1080×1080 PNG | `msstore/listing/box-art-1080.png` |
| Desktop screenshots | PNG, 1366×768 or larger (1920×1080 preferred), ≤ 50 MB, up to 10; one is required, four or more recommended; key content in the top two thirds; no added logos or marketing text; optional caption ≤ 200 characters | to capture |

Screenshots must come from the Windows build itself (the Store rejects
composed or foreign-platform images), with the current typeface, headings and
the verse-of-the-day card in frame: the same recapture the other stores wait
on (`docs/BACKLOG.md`). The GitHub Windows runner's screen is 1024×768, too
small for the Store, so the capture is done on a Windows machine with the
release zip or the smoke package. Suggested four: a chapter with a heading
and a note pill, search results, the notes browser, and the settings page
with the audio controls visible.

## Packaging

The GitHub release ships a bare `BibleText.exe` (`scripts/build-windows-exe.sh`,
run by `.github/workflows/release.yml`). The Store wants either an MSIX
package or an EXE/MSI installer hosted by the developer and signed with the
developer's own certificate. The MSIX route needs no certificate (the Store
re-signs the package with the identity above) and hosts the binary in the
Store, so it is the one taken. `fyne release -os windows` produces an .appx
whose manifest is not ours to shape, so it is not used.

`.github/workflows/msstore.yml` (windows-latest; on demand, and on pushes to
`main` that touch the packaging inputs):

1. builds `BibleText.exe` with the same script as the release;
2. runs the `cmd/msstore` tests, then fills `msstore/AppxManifest.xml.in`
   from `msstore/identity.json` and the desktop ledger, and lays out the
   exe, the manifest and `msstore/Assets/` under `build/msstore/layout/`;
3. runs `makepri` for the `en-GB` qualifier set and `makeappx pack`, and
   uploads the unsigned `BibleText-Windows-x64.msix` as the run's artifact;
4. smoke-installs a COPY: Mesa's software OpenGL beside the exe (the runner
   has no GPU), a throwaway self-signed certificate whose subject is the
   reserved publisher, `Add-AppxPackage`, activation through the shell, and
   the process must still be running after 30 s. The Store package never
   carries Mesa or a signature of ours.

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
  root as well); Windows fetches that file itself and re-reads it every few
  days; the Store checks nothing. Links clicked inside Edge, Chrome or Firefox
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
redirects into the package's LocalCache). "Read it in the browser" starts the
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
`%AppData%\Local` and `%AppData%\Roaming` are redirected to
`%LocalAppData%\Packages\<package family name>\LocalCache\`, which is where
preferences, notes and the Bible cache land for a Store install and what an
uninstall removes. Nothing in the app assumes otherwise.

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
   > generate text from that provider and store nothing. Rendering needs an
   > OpenGL 2.0-capable graphics driver.
7. Submit. Certification takes up to three business days; after it passes,
   Publish now.

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

`. scripts/msstore-env.sh` exports them (nothing printed), the way
`scripts/asc-env.sh` does for App Store Connect. `msstore/msstore.py`
(standard library) takes a client-credentials token for the classic Dev
Center resource and reads the account: `apps` lists every product with its
pending and published submission ids, `app <store id>` prints one product,
`submission <store id> <id>` prints a submission's status. The first call
listed `9NDCCZH9RB9K BibleText`, which proves the tenant association, the
role and the key. The write side (create a submission, upload the package to
its SAS URL, commit, poll) is written against the second release, once the
first has gone through the console: name reservation, the age rating and the
first publish are console-only. `msstore-cli` remains the oracle when the
two disagree. Rotating the key is the same console path; the Keychain item
is replaced and nothing in the repository changes.

## Risks

- **OpenGL on the certification machines.** Certification runs on virtual
  machines whose display adapter offers OpenGL 1.1, below the toolkit's 2.0
  floor; on such a machine the app fails at start-up. The GitHub runner is
  the same kind of machine, which is why the smoke copy carries Mesa. Before
  the first submission the app should choose software OpenGL itself when no
  hardware ICD is registered (a subfolder with Mesa's `opengl32.dll`, chosen
  with `SetDllDirectory` before the toolkit loads OpenGL); until then a
  certification failure on this point is possible and would cost a round.
- **Publisher name on an individual account** (Policy 10.14, above).
- **Unverifiable without a Windows 11 client**: that `desktop2:Parameters`
  on the web-to-app handler is honoured for a packaged classic app (the
  smoke's scheme step hard-fails only on the `uap3:Protocol` `Parameters`;
  the https step that would exercise the handler is advisory on the server
  runner), that the web-to-app handler fires with the consent file
  served by GitHub Pages as `application/octet-stream` (Microsoft documents
  no content type; Outlook's own file is served the same way), the browsers'
  prompt for the `bibletext:` scheme, the foreground grant, and Partner
  Center's reaction to the two declarations.
- **Policy 11.16** requires the generative-AI declaration and disclosure in
  the description; both are in place. Policy 10.3 asks that a product be
  testable: the notes for certification say how, and API.Bible must be up.
