# Links: how a shared verse reaches the app

Read this before touching an association file, an intent filter, an entitlement,
a desktop entry's `MimeType`, or the "Open in BibleText" button — and before
telling anyone that links work on a platform.

The prose about links already existed, spread across six documents. What did not
exist, and what this page is for, is the **matrix**: platform × channel ×
mechanism, with the three states below kept apart.

> **CONFIGURED** — the declaration exists.
> **ENFORCED** — something fails if it goes missing.
> **OBSERVED** — a person has watched it work.
>
> These are not the same thing, and the distance between them is easy to lose.
> "The Mac is wired but nobody has watched it" is one sentence away from "the
> Mac does not work", and the second of those is false: the Mac App Store is
> simultaneously the **most** heavily enforced channel here and one of the
> **least** observed. A claim about links belongs in one of these three columns
> or it should not be made.

---

## 1. Three mechanisms, one per platform family

Not redundancy, and not alternatives. Each operating system offers exactly one
way to hand an ordinary `https` link to a native app, so the app uses whichever
its platform provides.

| platform | mechanism | the site's half |
| --- | --- | --- |
| iOS, macOS | Apple **Universal Links** | `apple-app-site-association` |
| Android | **App Links** (`autoVerify`) | `.well-known/assetlinks.json` |
| Windows, Linux | the **`bibletext:` scheme** | `windows-app-web-link` (Store only) |

**The link a reader receives is always `https`, never `bibletext:`.** The custom
scheme exists only as a browser-to-app bridge on the two desktops that have no
domain-verification system, and it is emitted in exactly one place on the web —
the Windows/Linux branch of the notice page's button, which rewrites its own
href to `bibletext://` + the current host, path and fragment.

There is **no custom URL scheme on Apple at all.** No `CFBundleURLTypes` exists
anywhere in the tree. `cmd/bibletext/FyneApp.toml`'s `[CanOpen]` block looks
like a cross-platform scheme registration and is not: only the Unix packager
reads it.

---

## 2. The matrix

| platform | channel | mechanism | configured | enforced | observed |
| --- | --- | --- | --- | --- | --- |
| iOS | App Store | Universal Link | yes | partial | **yes** — real iPhone |
| iOS | Safari already on the domain | **none** (Apple suppresses it) | n/a | n/a | n/a |
| iOS | dev sideload | Universal Link, if the profile allows | yes | no | yes |
| iOS | Simulator | **none reachable** | n/a | n/a | n/a |
| macOS | Mac App Store | Universal Link | yes | **strongest here** | **never** |
| macOS | direct `.zip` | **none**, by design | n/a | n/a | n/a |
| Android | Play | App Links | yes | partial | **never on the Play cert** |
| Android | sideload APK | App Links | yes | yes | yes (upload cert) |
| Windows | Microsoft Store | `bibletext:` + appUriHandler | yes | yes | **yes** |
| Windows | direct `.zip` | **none** — there is no installer | n/a | n/a | n/a |
| Linux | tarball, before `make install` | none yet | n/a | yes | n/a |
| Linux | tarball, after `make install` | `bibletext:` | yes | yes | **yes** |
| Linux | Snap | `bibletext:` | yes | yes | **yes** |
| Linux | Flatpak | `bibletext:` | yes | yes | never |
| Linux | AppImage | **none** at system level | n/a | yes | n/a |

Three channels register nothing: the **Windows `.zip`**, the **Linux AppImage**
and the **macOS direct `.zip`**. That is one deferred decision rather than three
bugs — each fix would be the app's first write outside its own directories on
its platform. `docs/BACKLOG.md` holds the reasoning.

A reader on those channels is not stranded: they paste the `https` link they
were sent into Search, and it is parsed the same way.

---

## 3. The allow-list, and why it matters

The association file is an **allow-list**, not a domain claim. It claims
`/web/*`, `/bsb/*`, `/webc/*` and `/nkjv/*`, and explicitly EXCLUDES:

- `/privacy.html` and `/support.html` — the URLs App Store Connect points a
  reviewer at, which **must** open in a browser
- `/` and `/index.html` — the landing and download page
- `/404.html`, `/assets/*`

Widening this to `/*` would look harmless in review and would bounce a reviewer
tapping the privacy URL into the app. `applinks_test.go` is the tripwire, and
`scripts/publish-site.sh` re-validates the app id and the privacy/support
exclusions before pushing the site.

The Android manifest carries the same allow-list as `pathPrefix` entries, and
its test **parses the XML** rather than grepping it — see §6.

---

## 4. Two constraints that shape the web side

**A Universal Link does not open the app when the reader is already in Safari
on the same domain.** So the notice pages cannot simply link; they use Apple's
Smart App Banner in the head plus a button to the App Store product page, whose
own button reads OPEN when the app is installed. The reasoning is set out at
length in `cmd/websitegen/notice.go`.

**The "Open in BibleText" button exists only on the ~1,200 `/nkjv/` notice
pages.** The ~3,900 published scripture pages under `/web/`, `/bsb/` and
`/webc/` have no button, no Smart App Banner, no `intent://` and no
`bibletext://` — only a quiet footer link that becomes the App Store on an
Apple device. That is a deliberate asymmetry, not an oversight: the notice pages
exist *because* the licensed text cannot be shown, so getting the reader into
the app is their whole purpose.

---

## 5. Reaching an app that is already running

None of the three mechanisms handles this; the operating system simply starts
the program again with the URL as an argument. The second process then hands it
to the first over **loopback TCP with a two-step HMAC handshake** and exits, so
one window comes to the front at the passage instead of a second opening.

`share_link_argv.go` finds the URL position-independently in `os.Args`, strips
shell quoting and takes the first site URL. `single_instance.go` owns the
record and the handshake. `scripts/smoke-linux-launch.sh` exercises this path —
note that it puts the URL **straight on the command line**, so it proves the
handoff and *not* the desktop registration.

---

## 6. Known gaps

**The Android tripwire was text-matching until 21 September 2026.** Deleting a
`pathPrefix` failed it, but commenting out the entire intent-filter did not —
every string it looked for survived inside the XML comment. That is not
hypothetical: commit `3587ae3ee` removed that filter deliberately on 9 August
2026. It now parses the manifest, so a commented-out claim counts as absent.

**Nothing has ever asked Android whether verification actually succeeded.**
`adb shell pm get-app-links` appears nowhere in the repo. The one recorded
observation predates the Play signing certificate being added to
`assetlinks.json`, so it cannot have exercised the Play-signed install — which
is the one real readers get.

**The iOS release script never re-reads the signed artefact** to confirm the
entitlement survived export. The Mac script does, and fails if it is missing.

**Nothing checks the live site or Apple's CDN.** The association files are
validated as they are published, never as they are served.

---

## Related

- `docs/NKJV_FLOW.md` — the per-platform handoff narrative (H4) and the binding
  invariants; the most complete prose account, inside a document about NKJV
  licensing
- `docs/WEB_READER_PLAN.md` — the frozen URL contract the links are built from
- `docs/LINUX_STORES.md` — per-channel Linux packaging, including which
  channels register the scheme
- `docs/PLATFORM_MATRIX.md` — the OS × architecture × channel map; it models
  proof the same way but has no link row
- `docs/BACKLOG.md` — the deferred decision on the three unregistered channels
