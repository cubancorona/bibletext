# Platform matrix

What BibleText can be built and shipped as, on which architecture, through
which channel — and **how well each of those is actually proven**. One feature
has a table of its own, [Sharing](#sharing), because Share is the verb whose
mechanism differs on every platform.

This is a capability map, not a status board. It does not track store review
state or which version is live; the release ledger and `linux/releases.toml`
do that, and they change weekly while these facts change a few times a year.

## How to read the proof column

The proof level is the point of this document. It was written because an
architecture was excluded for a year on the strength of a sentence that read
like a decision and was really a gap:

> Architectures — x86_64 only — *the toolkit selects GLES on arm64 and the
> repository has never built or smoked Linux/arm64*

"Never built or smoked" is a proof level, not a reason. Recorded in the
decisions table it looked settled; recorded here it would have read `none` and
sat in the to-do list where it belonged. So:

| Proof | Means |
| --- | --- |
| `hardware` | the artefact we actually ship was run on a real machine of that architecture |
| `field` | in real use — readers or testers are running the shipped artefact |
| `runner` | run in CI only — headless, software OpenGL, no sound device, no store signing |
| `builds` | compiles and packages; nothing has ever launched it |
| `none` | never attempted |

Two rules keep the column honest. **The artefact must be the shipped one** — a
locally-signed development build of the same source is not the Store package,
and a simulator is not hardware. **Someone must have looked** — a job that
compiles a thing and uploads it proves `builds`, however green it is.

`field` is the strongest evidence and the weakest guarantee. It says the thing
works for real people on real devices, which no runner can tell you. It says
nothing about the NEXT build, because nobody is watching for a regression —
so a `field` row still needs a repeatable check, and the absence of one is
recorded under "Not yet proven" rather than pretended away. Where the repo
itself evidences nothing, `field` rests on the maintainer's own knowledge of
the store consoles; it is deliberately not a claim about anything in the tree.

Status is separate: `shipping` (a release or store submission carries it),
`ready` (built and smoked, not yet released), `proven` (runs, no release path),
`builds`, `untried`, `excluded` (a decision, quoted below).

## The matrix

| OS | Arch | Channel | Status | Proof | Notes |
| --- | --- | --- | --- | --- | --- |
| macOS | arm64 | Mac App Store | shipping | builds | 1, 2, 3 |
| macOS | x86_64 | Mac App Store | shipping | builds | 1, 2, 3, 26 |
| macOS | arm64 | Direct download (.zip) | shipping | builds | 1, 2, 4 |
| macOS | x86_64 | Direct download (.zip) | shipping | builds | 1, 2, 4, 26 |
| macOS | arm64 | Build from source (local checkout) | shipping | hardware | 5 |
| macOS | x86_64 | Build from source | proven | hardware | 5, 26 |
| iOS | arm64 | App Store — iPhone | shipping | field | 6, 7, 8 |
| iPadOS | arm64 | App Store — iPad | shipping | builds | 6, 7, 8 |
| iOS | arm64 | Development install to a device | proven | hardware | 7 |
| iOS | arm64 | Simulator | proven | runner | 9 |
| iOS | x86_64 | anything | excluded | none | A |
| Android | arm64-v8a | Google Play — production and closed testing (AAB) | shipping | field | 10, 11, 12 |
| Android | armeabi-v7a, x86, x86_64 | Google Play — production and closed testing (AAB) | shipping | builds | 10, 11 |
| Android | all four ABIs | GitHub Releases — universal APK, not linked from the site | shipping | builds | 10, 12 |
| Android | arm64-v8a | Local install / adb (debug APK) | proven | hardware | 11 |
| Windows | x64 | Microsoft Store (MSIX) | shipping | runner | 14, 15, 16, 18 |
| Windows | x64 | Direct download (.zip) | shipping | builds | 14, 15 |
| Windows | arm64 | Microsoft Store (MSIX) | shipping | runner | 14, 15, 16, 17, 18 |
| Windows | arm64 | Direct download (.zip) | ready | builds | 14, 15, 17 |
| Windows | x86 (32-bit) | anything | untried | none | — |
| Linux | x86_64 | Direct download (.tar.xz) | shipping | builds | 19, 20, 24 |
| Linux | arm64 | Direct download (.tar.xz) | ready | hardware | 19, 20, 24, 25 |
| Linux | x86_64 | AppImage | shipping | builds | 21 |
| Linux | arm64 | AppImage | ready | hardware | 21, 25 |
| Linux | x86_64 | Snap Store | shipping | runner | 22, 23 |
| Linux | arm64 | Snap Store | shipping | hardware | 22, 23, 25 |
| Linux | any | AppImageHub catalogue | untried | none | — |

## Divergences

Things done for one platform and nowhere else. This is where the risk lives:
each one is a place the platforms stop sharing a code path, and therefore a
place a fix on one does not reach the others.

**macOS**

1. **A native AppKit `NSTextView` reading overlay** replaces the shared Fyne
   pane (`reading_macos.go`, ~188 KB of cgo; `styledPaneEnabledOnPlatform =
   false` for darwin). *Risk:* every typography, spacing or selection change
   must be implemented and verified twice, and unlike Windows/Linux there is no
   Fyne fallback pane to revert to — `chapterTextScrollArea` is not compiled on
   darwin at all.
2. **The toolkit patches are applied at build time only**, and the binary-level
   proof that the atomic-preferences patch landed exists in
   `scripts/release-mac-store.sh` but **not** in the release workflow's direct
   download. *Risk:* if the patch step silently no-ops, the download ships the
   truncating preferences writer — a crash mid-write empties the store and the
   app reads that as a brand-new reader, losing every note.
3. **The deployment floor is injected twice** (`-mmacosx-version-min` into
   CGO flags, then `LSMinimumSystemVersion` rewritten with PlistBuddy; floor held by
   `scripts/check-min-os-versions.py`), because
   cgo otherwise stamps the build machine's SDK version. *Risk:* drop it and
   the app refuses to launch on anything older than the machine that built it.
4. **Signature-dependent secret storage at runtime** — the same binary uses the
   login Keychain when signed one way and something else when not.
5. macOS is the **only platform whose daily development loop is the shipped
   artefact's architecture**, which is why its source-build row is the one
   `hardware` entry in the whole macOS block.

**iOS / iPadOS**

6. **A hand-assembled `.xcarchive` plus `xcodebuild -exportArchive`**, instead
   of `fyne release -os ios` (`scripts/release-ios.sh`).
7. **Info.plist keys Fyne never emits, injected per build before signing** —
   `UIBackgroundModes=['audio']`, privacy strings, `UIDeviceFamily`, and the
   scene manifest (`UIApplicationSceneManifest`, `scripts/ios-scene-manifest.sh`;
   without it iOS 27 refuses the app at launch — read back from the archived and
   exported app by `release-ios.sh`). *Risk:*
   `UIDeviceFamily` is the single property that makes the app a universal
   iPhone+iPad binary, and it is **never read back out of the exported `.ipa`**.
8. **The icon catalogue is rebuilt from scratch** (18 slots scaled with `sips`)
   and the launch screen compiled by hand with `ibtool`.
9. The simulator **cannot exercise the device-only paths** — Keychain
   entitlements need a `__TEXT,__entitlements` hack there — so simulator runs
   are `runner`, never `hardware`.

**Android**

10. **`scripts/build-android.sh` is the only supported build path**; a bare
    `fyne` command does not produce a shippable artefact. The CLI's prebuilt
    `classes.dex` is **replaced** with a locally compiled patched
    `GoNativeActivity`.
11. **Four Java sources** (`BtBridge`, `BtAudio`, `BtAudioService`, `BtKeys`)
    compiled with `javac --release 8` + `d8`, and a **custom
    `cmd/mobile/AndroidManifest.xml` consumed verbatim**, whose `foregroundServiceType`
    is what makes background narration legal.
12. **NDK pinned to r27 LTS**; r28+ explicitly forbidden.
13. Android's only CI was a target-SDK text check until 19 September 2026;
    `scripts/check-android-pane.sh` now cross-compiles android/arm64 beside
    the iOS gate. Building is still the whole of it — nothing runs an APK.

**Windows**

14. **`-tags gles` on both the build and the package line**, plus **bundled
    ANGLE** (`libEGL.dll`, `libGLESv2.dll`, `d3dcompiler_47.dll`) fetched by
    `scripts/fetch-angle.ps1` and pinned to a third-party build, so rendering goes through Direct3D. No other platform
    does this. *Risk:* without it the app does not render at all on a machine
    with no OpenGL driver, which is the default Windows Server runner and many
    real PCs.
15. **A Windows-only seventh Fyne patch** (`fyne-2.7.4-windows-egl.patch`)
    excluding Windows from the GLFW OpenGL path.
16. The render is gated on a real capture by `scripts/check-reading-centred.py`,
    and the package manifest is `msstore/AppxManifest.xml.in`.
17. Windows arm64 needs a **toolchain we ship ourselves**. The
    `windows-11-arm` image's gcc is x86_64, so cgo cannot assemble aarch64 —
    `runtime/cgo` dies on its first instruction.
    `scripts/fetch-llvm-mingw.ps1` installs a pinned llvm-mingw that both runs
    on an ARM64 host and targets `aarch64-w64-mingw32`, with `CC` pointed at
    it. A compiler is as much a part of what ships as a library is, so it is
    pinned by sha256 like ANGLE and the AppImage tools. Verified on the runner:
    the MSIX declares arm64 and its executable and all three ANGLE libraries
    are genuine ARM64 images.
18. **Native code for package identity** (`GetCurrentPackageFullName`) to tell
    Store-installed from loose builds, and the browser is started directly via
    `AssocQueryStringW` rather than `ShellExecute`.

**Linux**

19. **A patched Fyne packaging CLI**, built from source for the Linux job only,
    because the stock packager emits `Exec=… %F` — which breaks every
    "Open in BibleText" link.
20. **A hard glibc ceiling** (2.35) enforced by `scripts/check-glibc-floor.sh`
    with the
    runner pinned to `ubuntu-24.04`, so a runner image drifting upward cannot
    silently raise the floor.
21. **AppImage tooling pinned by sha256 per architecture** in
    `scripts/build-appimage.sh`. appimagetool RUNS on the build host while the
    runtime it embeds must match the executable being wrapped, so the script
    refuses a target it cannot build here rather than producing an AppImage
    that launches on the build machine and nowhere else. The
    update-information string carries the architecture too, or both would
    advertise the same zsync file and readers would be offered the wrong one.
22. **Snap-only ALSA plumbing**: `libasound2-plugins` staged plus a `layout`
    binding `/usr/lib/<triplet>/alsa-lib`. **The triplet is architecture
    specific and the bind is a real path.** *Risk:* a snap carrying another
    architecture's triplet builds, packs, installs and launches perfectly and
    has **no narration audio at all** — invisible to every build and launch
    check, discoverable only by a reader pressing play. This is why
    `cmd/linuxmeta/main.go` takes `-arch`, why `snap/snapcraft.yaml` is the amd64
    render, and why the arm64 jobs in `.github/workflows/release.yml` re-render.
23. **The snap is a `dump` plugin from a prebuilt keyed binary**, so the LXD
    build never sees repository secrets.
24. Both Linux architectures run the same packaged-tarball assertions, via
    `scripts/check-linux-package.sh`, so they cannot drift apart.
25. arm64 Linux was proven on a local UTM VM, 18–19 September 2026 — first at
    the build bench, then on a real GNOME X11 desktop. At the bench: the
    executable builds with no new patches and passes
    `scripts/smoke-linux-launch.sh` (launch, `bibletext:` handoff, single
    instance); the snap packs in LXD, installs and exports its entry with `%u`;
    and the **tarball** was packaged with the patched CLI and put through
    `scripts/check-linux-package.sh`, which is how the `make install` defect was
    found. On the desktop, 19 September, all three channels were run the way a
    reader runs them:

    - the snap was **installed from the Snap Store** (`snap install bibletext
      --edge`) and auto-connected all fifteen interfaces, `audio-playback`
      included, with no manual `snap connect`;
    - **narration reached the sound server from every sandbox** — the snap as
      `ALSA plug-in [bibletext]`, the AppImage as `PipeWire ALSA [bibletext]`,
      the tarball as `PipeWire ALSA [desktop]`, each with both channels active
      on the hardware sink. This is the check the arch-specific ALSA layout
      exists for: a wrong triplet builds, packs, installs and launches
      perfectly and is silent, so only a stream on a real sound server settles
      it. (That third client name was the executable-name defect: the
      packager took the name from the `cmd/desktop` source directory, which is
      now `cmd/bibletext` — see `docs/BACKLOG.md`.)
    - **read-along** advanced through the chapter and auto-scrolled to follow;
    - a `bibletext:` link fired with `xdg-open` reached the running instance,
      brought its window to the front and navigated, leaving one process. This
      is the desktop-registration half that `smoke-linux-launch.sh` cannot
      reach, because that script hands the URL straight to the binary;
    - the **AppImage runs with no libfuse2 package installed at all**;
    - the reading pane does **not** scroll sideways — wheel-left and
      wheel-right moved zero pixels, against a vertical control that moved
      119,715;
    - switching GNOME to dark repainted the **confined** snap live, through the
      `org.freedesktop.appearance` portal, with no restart.

    Still not done: launching the application from where `make install` puts it.
    The install was run into a `DESTDIR` and its paths checked, and the binary
    was run from the extracted tree — but not from `/usr/bin` on a live system.

26. **x86_64 macOS is now compiled and tested on Intel hardware**, not only
    cross-compiled from arm64. On 19 September 2026 an Intel MacBook Pro built
    the whole tree natively — including the 188 KB of AppKit cgo in
    `reading_macos.go`, which CI has never compiled on this architecture —
    passed `go vet` and passed the **entire test suite**. The shipped 1.2.12
    universal build was also checked there: genuinely two slices, `minos 12.0`
    on each, and the Intel slice carries the patched preferences writer
    (marker present, control string present). What is still missing is a
    LAUNCH: driving a GUI app needs a console session, and SSH cannot supply
    one, so the shipped rows stay `builds`.

**Linux and Windows**

27. **Linux and Windows have no system share sheet, so the text verbs copy
    to the clipboard and open the app's own confirmation sheet — the
    recorded Linux and Windows counterpart of the system share sheet.**
    `share_other.go:1` (`//go:build !darwin && !android`) gives both
    platforms the desktop fallback: `share_other.go:18`
    (`fallbackShareText(s)`) and `share_other.go:21`
    (`fallbackShareImage(path)`). `go list` for linux and windows, amd64 and
    arm64, with and without `-tags gles`, takes the two verbs from
    `share_other.go` and `share_fallback.go` and compiles none of
    `reading_macos.go`, `reading_ios.go` or `reading_android.go`. Every text
    verb ends in `share.go:220` (`var shareTextOut`); the fallback copies
    (`setShareClipboard`) and opens the sheet
    (`showShareCopiedSheet`, `share_sheet_desktop.go`): "Copied — ready to
    paste", one line saying what to do next, by verb, the clipboard's text
    in a selectable scrolling box, Copy again, Email… when the desktop has a
    mail client (`share_email_linux.go`, `share_email_windows.go`), and
    Done, with Escape and Return. It is modal, registered for the
    light/dark reopen and the window refit, opens below the header, and
    over the verse-of-the-day card comes back with the card beneath it.
    Share as image saves the PNG to Downloads, opens the file manager on
    it, and ends in the same sheet, "Picture saved", with Email… attaching
    the file on Linux.

    Until 30 September 2026 the text verbs ended instead in a 13 pt
    "Copied to the clipboard" pill at the window's foot for 1.4 s, 1.07:1
    against the page in light and about 1.2:1 in dark, gone at the next
    click, and on Share with note drawn in the frame the composer closed and
    the "Note from you" card appeared; it read as nothing happening, which
    the rule `docs/BACKLOG.md` records with the Android Share as image fix —
    *"A share the reader started cannot end in silence, whatever the
    cause."* — does not allow. What remains divergent is what the sheet is
    not: a way to hand the text straight to another app. Linux has no
    portal for that; Windows has a native Share sheet, planned under
    [Sharing](#sharing), Planned, and shares through this sheet until it
    lands. Held by `share_sheet_desktop_test.go`, driven on the Linux VM
    (the Linux row's proof), and recorded in `docs/BACKLOG.md`, "Linux and
    Windows: an in-app share sheet in place of the 1.4-second notice".

## Deliberate exclusions

Decisions, with the reason — so that none of these is ever mistaken for
something nobody got round to.

- **A. iOS x86_64** — Apple ships no x86_64 iOS devices. The only x86_64 iOS
  target is the simulator on an Intel Mac host.
- **B. Windows arm64** — **no longer excluded, and now built.** Neither
  assumed blocker was real: ANGLE publishes `angle-arm64` at the tag already
  pinned, and the runner's missing aarch64 compiler is supplied by a pinned
  llvm-mingw. Both channels build it; see divergence 17.
- **C. Linux arm64 AppImage** — *resolved 19 September 2026.* Both
  appimagetool and the type-2 runtime publish aarch64 builds at the tags
  already pinned, verified with the x86_64 hashes as controls, and both are
  genuine aarch64 ELF.

## Not yet proven

The honest to-do list, in proof-level terms.

- **Nothing we ship on macOS has ever been launched from the artefact we
  ship.** Every macOS row is `builds`. The local rehearsal
  (`scripts/run-mac-sandbox-test.sh`) still differs from the Store package: it is
  arm64-only rather than universal, signed with a development certificate, and
  carries no provisioning profile. It no longer differs on the toolkit — as of
  19 September 2026 it applies the Fyne patches and refuses to launch a build
  whose atomic preferences writer is missing, measured against a control string.
  That closed the axis that mattered most, and left three.
- **Android and iOS are in real use, and neither has a repeatable check.**
  The Play build is with closed testers on real devices, and with readers
  since it reached production on 30 September 2026; the iOS App
  Store build has many installs, so both work — that is `field`, and it is
  better evidence than any runner could give. What neither has is anything that
  would catch a regression before readers do: no Android CI beyond the new
  compile gate, no automated launch of either shipped artefact, and an Android
  build still host-locked to one machine. The armeabi-v7a, x86 and x86_64
  slices of the AAB remain `builds`: they ship inside it, but nothing indicates
  a tester is on one.
- **The iPad build is `builds`, not `field`.** The universal binary ships and
  iPhone use is established; nothing distinguishes an iPad install, and
  `UIDeviceFamily` — the single property that makes it universal — is never
  read back out of the exported `.ipa` (divergence 7).
- **The AppImage's type-2 runtime is not exercised in CI.** The smoke runs the
  extracted payload, deliberately: launching the runtime twice would extract
  twice into one content-addressed directory. It is covered there by the
  update-information read and the extraction itself, and the mount-and-launch
  path was proven by hand on arm64 on 19 September 2026 — but no job does it.
- **The Linux tarball's installer now runs on every packaged tarball**
  (`scripts/check-linux-package.sh`), and the first run of it found `make
  install` had always failed — see docs/BACKLOG.md. Fixed. What is still
  unproven is launching the installed application from where it installs.
- **Windows arm64 has never been run by a person.** It builds, packs,
  installs and renders on the runner, which is `runner` and not `field`. The
  local Windows ARM VM is the machine that could change that.
- The owner's outstanding Linux hardware pass needs **an x86_64 machine or a
  cloud desktop**: the arm64 VM explicitly cannot stand in for it.
- **Share has been watched on two platforms of six.** Linux on the arm64 VM
  and Android on the emulator; macOS, iOS, iPadOS and Windows are recorded
  from the code (see [Sharing](#sharing)). The iPad and Mac rows carry
  anchoring that is probable from the code and has not been seen.

## Sharing

What each Share verb does on each platform: the mechanism, and what the
reader sees. Every text verb — Share with note, Share with citation, Share as
link, and the Share icon on the verse of the day — ends in `shareTextOut`
(`share.go`), which calls `nativeShareText`; Share as image goes through the
preview sheet (`share_preview.go`) to `nativeShareImage`. The file that
defines those two functions for a platform's release build decides its row,
and **Defined in** names it, followed, where that file only hands the verbs
on, by the file it hands them to.
`TestTheSharingTableNamesTheFileThatSharesOnEachPlatform`
(`platform_matrix_test.go`) resolves the two definitions and their hand-ons
for each row with that platform's GOOS, every architecture it ships and the
build tags read off its release lines, and fails when the row and the code
disagree in either direction — a platform moved to a new share
implementation with the row left alone, or a row edited with the code left
alone. It holds two claims in the cells to the code as well: a row names
the desktop confirmation sheet by its heading, "Copied — ready to paste",
for the text verbs exactly when the function they end in calls
`showShareCopiedSheet`, and says "Downloads" for the image exactly when it
calls `revealInFileManager`. Numbers in a cell are divergences.

Recorded 29–30 September 2026: the Apple and Android rows from the share
code as it shipped in 1.2.17, the Windows and Linux rows from the desktop
confirmation sheet built on 30 September 2026, which ships next.

| Platform | Share with note | Share with citation | Share as link | Share as image | Verse of the day | Defined in | Proof |
| --- | --- | --- | --- | --- | --- | --- | --- |
| iOS | System share sheet, over the new note card | System share sheet | System share sheet | Preview, then the system share sheet | System share sheet | `reading_ios.go` | `builds` — code reading, 29–30 September 2026 |
| iPadOS | Share popover; probably points mid-page rather than at the selection | Share popover at the selection | Share popover at the selection | Preview, then a share popover; probably mid-page | Probably a share popover pointing at the hidden reading view, not the icon; not seen | `reading_ios.go` | `builds` — code reading, 29–30 September 2026 |
| macOS | Share picker at the selection; the note card appears beneath | Share picker at the selection | Share picker at the selection | Preview, then the share picker with the image | Probably the share picker, anchored to the hidden reading view, not the icon; not seen | `reading_macos.go` | `builds` — code reading, 29–30 September 2026 |
| Android | The note card, then the "Sharing text" sheet with the note, citation and link | "Sharing text" sheet with the quote and citation | "Sharing text" sheet with the link | Preview, then the "Sharing image" sheet | "Sharing text" sheet, by the citation's route; not driven | `reading_android.go` | `runner` — emulator, 1.2.17, 29–30 September 2026 |
| Windows | Copied; the "Copied — ready to paste" sheet over the new note card, with Copy again, Email… and Done (27) | Copied; the "Copied — ready to paste" sheet (27) | Copied; the "Copied — ready to paste" sheet (27) | Saved to Downloads; Explorer opens with it selected; the "Picture saved" sheet, without Email… | Copied; the "Copied — ready to paste" sheet over the card (27) | `share_other.go`, handing on to `share_fallback.go` | `builds` — build tags and `go list`, 30 September 2026 |
| Linux | Copied; the "Copied — ready to paste" sheet over the new note card, with Copy again, Email… and Done (27) | Copied; the "Copied — ready to paste" sheet (27) | Copied; the "Copied — ready to paste" sheet (27) | Saved to ~/Downloads; the file manager opens on the folder; the "Picture saved" sheet, Email… attaching the file | Copied; the "Copied — ready to paste" sheet over the card (27) | `share_other.go`, handing on to `share_fallback.go` | `hardware` — arm64 VM, X11, 30 September 2026 |

What each proof rests on:

- **Linux, `hardware`, as an exception to the rule that the artefact must
  be the shipped one.** The sheet is not yet in a release: a local build of
  the tree that adds it ran on the arm64 VM's GNOME X11 desktop (divergence
  25's machine), and every verb was driven from its own entry point — the
  selection menu's Share with citation and Share as link, the composer's
  Share, the verse-of-the-day card's icon, the image preview's Share —
  in light and in dark, with a screenshot of each sheet; the clipboard was
  read back after each text share. Email… was pressed with no mail client
  installed, where the button is withheld (the desktop reports no mailto:
  handler) — the portal's compose with a real client has not been seen. No
  shipped package was run; the row moves to the shipped artefact with the
  release that carries it. Not Wayland.
- **Android, `runner`.** The 1.2.17 build on the emulator. A simulator is not
  hardware (divergence 9), and an emulator is not either. Every verb but the
  verse of the day was driven to its share sheet; no share target was tapped.
- **macOS, iOS and iPadOS, `builds`.** Read from the code; nothing was
  launched for this table. The cells marked "probably" — the iPad's Share
  with note and Share as image, and the verse of the day on iPad and Mac —
  are recorded in `docs/BACKLOG.md` to be seen on a device. On the Mac it is
  not even known that the picker shows while the view it is anchored to is
  hidden.
- **Windows, `builds`.** Read from the build tags and `go list`; the Windows
  VM was not started. It runs the Linux code — the same sheet, from the same
  files — but for two things: the file manager the image share opens
  (Explorer, with the file selected), and Email…, which opens a `mailto:`
  link through the shell (`share_email_windows.go`) when a mailto: handler
  is registered, and which a link cannot carry a file through, so the image
  sheet has no Email… there.

### Planned

- **Windows: the native Share sheet — decided 30 September 2026, not built.**
  Every verb opens the Windows Share UI beside the window, as the picker does
  on macOS: `IDataTransferManagerInterop` gives the window's
  DataTransferManager (`GetForWindow`), a `DataRequested` handler fills the
  package (`SetText`, `SetWebLink`, `SetStorageItems`), and
  `ShowShareUIForWindow` opens it
  (https://learn.microsoft.com/en-us/windows/apps/develop/windows-integration/integrate-sharesheet-send).
  Files: a new *share_windows.go* defines `nativeShareText` and
  `nativeShareImage`; `share_other.go` narrows to exclude windows;
  `share_fallback.go` stays behind the sheet for a share UI that fails; the
  window handle is reached as `title_bar_windows.go` reaches it; and the
  Windows row above, which the test holds to whichever files define the two
  functions and whatever they hand on to, and which must stop naming the
  confirmation sheet once the verbs no longer end in `showShareCopiedSheet`.
  Whether the unpackaged zip can open the sheet as the MSIX can is to be
  proven on the Windows VM. The route, its constraints and its sources are in
  `docs/BACKLOG.md`, "Windows: use the native Share sheet". Until it lands
  Windows shares through the confirmation sheet Linux has (divergence 27).
