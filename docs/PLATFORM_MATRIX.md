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
| Android | all four ABIs | GitHub Releases — universal APK | shipping | builds | 10, 12 |
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

27. **Linux has no system share sheet, so the text verbs copy to the
    clipboard and open the app's own confirmation sheet — the recorded Linux
    counterpart of the system share sheet, and the sheet Windows falls back
    to wherever its own Share sheet cannot open (28).**
    `share_other.go:1` (`//go:build !darwin && !android && !windows`) gives
    Linux the desktop fallback: `share_other.go:20`
    (`fallbackShareText(s)`) and `share_other.go:23`
    (`fallbackShareImage(path)`). `go list` for linux, amd64 and arm64, with
    and without `-tags gles`, takes the two verbs from `share_other.go` and
    `share_fallback.go` and compiles none of `reading_macos.go`,
    `reading_ios.go`, `reading_android.go` or `share_windows.go`; windows
    takes them from `share_windows.go`, whose fallbacks are the same two
    functions. Every text
    verb ends in `share.go:220` (`var shareTextOut`); the fallback copies,
    `share_fallback.go:36` (`setShareClipboard(s)`), and opens the sheet,
    `share_fallback.go:37` (`showShareCopiedSheet(state, shareDoneForText(s))`):
    the heading, `share_sheet_desktop.go:64` (`"Copied — ready to paste"`),
    one line saying what to do next, by verb, the clipboard's text in a
    read-only scrolling box, `share_sheet_desktop.go:221`
    (`boxScroll = container.NewVScroll(`), Email… when the desktop has a
    mail client, `share_sheet_desktop.go:389`
    (`shareEmailProbe(attachment != "", func(ok bool) {`), at the row's
    left end so that its late arrival moves nothing,
    `share_sheet_desktop.go:295`
    (`buttons = container.NewHBox(email, copyAgain, done)`), Copy again,
    whose "Copied again." keeps the line's height,
    `share_sheet_desktop.go:205`
    (`lineSlot := container.New(heldLineLayout{held: held}, line)`), and
    Done, with Escape and Return, `share_sheet_desktop.go:321`
    (`cnv.SetOnTypedKey(`), and after Tab has put the caret on a button,
    `share_sheet_desktop.go:423`
    (`func (b *shareSheetButton) TypedKey(`). It is modal,
    `share_sheet_desktop.go:307` (`widget.NewModalPopUp(card, cnv)`),
    registered for the light/dark reopen and the window refit, opens below
    the header, `share_sheet_desktop.go:358` (`room := clearOfHeader(`),
    and over the verse-of-the-day card comes back with the card beneath it,
    `share_sheet_desktop.go:379` (`under := takeReopenBeneath(state, popup)`).
    Email… on Linux is the desktop portal's Email interface,
    `share_email_linux.go:174` (`portalEmailIface+".ComposeEmail"`), its
    request's answer watched, `share_email_linux.go:180`
    (`case sig := <-responses:`), and only its 2 — no mail client — handing
    on to xdg-email and a `mailto:` link, `share_email_linux.go:191`
    (`return portalEmailResponse(code)`); the image goes as a descriptor,
    `share_email_linux.go:167` (`"attachment_fds"`), and the image's Email…
    is offered only outside the snap and the Flatpak, and only where the
    desktop's mailto: handler is a mail client rather than a browser,
    `share_email.go:296` (`return !f.confined && (f.portalEmail`): a
    browser, which is what a stock Ubuntu desktop names, is handed a
    `mailto:` link with the file's local path in it and opens a compose
    without the picture. Inside the snap and the Flatpak the text's Email…
    is offered only where the portal can answer `SchemeSupported`,
    `share_email.go:301` (`if f.confined {`), since `xdg-mime` there reads
    the sandbox's own handler, not the desktop's; Ubuntu 24.04's portal
    cannot, so the snap there offers no Email… at all. On Windows it is a
    `mailto:` link when the shell resolves a handler for the scheme,
    `share_email_windows.go:33` (`schemeHandlerRegistered("mailto")`). A
    `mailto:` link, on either platform, carries line breaks as CRLF,
    `share_email.go:85` (`strings.ReplaceAll(b, "\n", "\r\n")`), and is
    kept to 2,000 characters, `share_email.go:73`
    (`const mailtoMaxLen = 2000`): past that the passage or the note is cut
    at a word and ends in an ellipsis, and the citation, with a link
    share's link, is kept whole; the whole text is still on the clipboard.
    Share as image saves the PNG to Downloads, opens the file manager on
    it, and ends in the same sheet, "Picture saved",
    `share_fallback.go:65` (`showShareCopiedSheet(state, shareDone{line: line`),
    whose Email… carries the quote and its citation as the mail's text.

    Until 30 September 2026 the text verbs ended instead in a 13 pt
    "Copied to the clipboard" pill at the window's foot for 1.4 s, 1.07:1
    against the page in light and about 1.2:1 in dark, gone at the next
    click, and on Share with note drawn in the frame the composer closed and
    the "Note from you" card appeared; it read as nothing happening, which
    the rule `docs/BACKLOG.md` records with the Android Share as image fix —
    *"A share the reader started cannot end in silence, whatever the
    cause."* — does not allow. What remains divergent is what the sheet is
    not: a way to hand the text straight to another app. Linux has no
    portal for that; Windows has its Share sheet, which it now opens (28),
    reaching this sheet only when that cannot open. Held by
    `share_sheet_desktop_test.go`, driven on the Linux VM (the Linux row's
    proof), and recorded in `docs/BACKLOG.md`, "Linux and Windows: an
    in-app share sheet in place of the 1.4-second notice".

**Windows**

28. **Windows opens its own Share sheet, through COM written by hand, with
    the Linux sheet (27) behind it.** Every verb hands its share to
    `windowsShareVerb`, `share_windows.go:64`
    (`windowsShareVerb(func() { fallbackShareText(s) }`), with the in-app
    sheet as its fallback. The window's thread joins a single-threaded
    apartment on the first share, `share_winrt_windows.go:254`
    (`procRoInitialize.Call(roInitSingleThreaded)`): nothing else in the
    process initialises COM there, and RunNative on Windows runs on the
    calling goroutine, so the thread is checked, not assumed. The window's
    DataTransferManager, `share_windows.go:228`
    (`return comCall(interop, 3, hwnd`), takes a DataRequested handler,
    `share_windows.go:271` (`return comCall(dtm, 6,`), and the sheet opens,
    `share_windows.go:280` (`hr = windowsShareStep(stepShowSheet,`), over
    the window. When the sheet asks, the handler fills the package: the
    citation as the title the sheet requires, `share_windows.go:363`
    (`hr := comCall(props, 7, uintptr(ws.title))`), the message as text, a
    link or note share's link as a web link, `share_windows.go:383`
    (`hr = comCall(pkg2, 7, uintptr(unsafe.Pointer(ws.uri)))`), and the
    picture through a one-item collection the app implements,
    `share_windows.go:391`
    (`comCall(pkg, 23, uintptr(unsafe.Pointer(ws.items)), 1)`). The title
    shows only where the sheet has a line for it: on Windows 11 the link
    sheet shows it, the text sheet shows none, and the picture sheet shows
    the file's name in its place. The picture is taken at the tap,
    `share_windows.go:75` (`img := takeSharedImage(path, time.Now())`), and
    copied to a file named for the reader, `share_parts.go:98`
    (`if file, err := copyShareImage(path, now); err != nil {`), so that a
    later preview, which renders over the renderer's file, cannot change
    what is shared or what its fallback saves; the copy is resolved as a
    StorageFile before the sheet opens, `share_windows.go:450`
    (`return comCall(op, 6, uintptr(unsafe.Pointer(done)))`), since a
    request that defers has 200 ms by the documentation. The Go objects
    Windows calls aggregate the free-threaded marshaler,
    `share_winrt_object_windows.go:108`
    (`procCoCreateFreeThreadedMarshaler.Call(`), because the picture's file
    completes on a thread-pool thread and Windows asks the handler for
    IAgileObject before accepting it. Every step that fails, and a sheet
    that has not asked within `share_session.go:46`
    (`const shareSessionWait = 5 * time.Second`), ends the share in its
    fallback, once, `share_session.go:232` (`s.fallback()`); a reader's
    cancel is a choice, as on the other platforms, and opens nothing. A
    sheet asked to open, `share_windows.go:287` (`s.sheetOpening()`), does
    not know when the app stops waiting, so its handler stays registered
    after that fallback for a sheet that asks late, which then still gets
    the share, until it asks, a newer share starts or
    `share_session.go:54` (`const shareSheetKeep = 30 * time.Second`)
    runs out, the time Chromium gives the same sheet. The unpackaged
    download and the MSIX run the same code, although Microsoft's pages
    disagree on whether an app without package identity may open the
    sheet: a probe executable without it opened the sheet on Windows 11
    arm64, natively and under x64 emulation, and so did the app itself,
    for every verb: the arm64 Store package's executable, run without
    package identity as the zip runs (the Windows proof note under
    [Sharing](#sharing)). The installed MSIX has not yet been seen to, nor
    the zip itself. Held by `share_session_test.go`,
    `share_parts_test.go` and `share_sheet_desktop_test.go` on every
    platform, `share_windows_test.go` in the Windows CI job, and recorded
    in `docs/BACKLOG.md`, "Windows: use the native Share sheet".

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
- **Windows arm64 has not been run by a person from an artefact we ship.**
  It builds, packs, installs and renders on the runner, which is `runner`
  and not `field`. On 30 September 2026 the executable from the arm64
  Store package of a tree not yet released (the Microsoft Store package
  workflow, run 36720846058) was run unpackaged on the local Windows ARM
  VM and every Share verb driven there (the Windows proof note under
  [Sharing](#sharing)). That is not the shipped artefact, and the
  installed MSIX was not run, so the matrix rows stay where they are.
- The owner's outstanding Linux hardware pass needs **an x86_64 machine or a
  cloud desktop**: the arm64 VM explicitly cannot stand in for it.
- **Share has been watched on three platforms of six.** Linux and Windows
  on arm64 VMs and Android on the emulator; macOS, iOS and iPadOS are
  recorded from the code (see [Sharing](#sharing)). The iPad and Mac rows
  carry anchoring that is probable from the code and has not been seen.
  On Windows no share target was pressed, so what an app receives from
  the sheet has not been seen, and the in-app sheet behind it, which opens
  only when the sheet cannot, has not run there.

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
code as it shipped in 1.2.17, the Linux row from the desktop confirmation
sheet built on 30 September 2026, and the Windows row from its native Share
sheet built the same day and driven in the app on the Windows VM that day;
neither is in a release yet.

| Platform | Share with note | Share with citation | Share as link | Share as image | Verse of the day | Defined in | Proof |
| --- | --- | --- | --- | --- | --- | --- | --- |
| iOS | System share sheet, over the new note card | System share sheet | System share sheet | Preview, then the system share sheet | System share sheet | `reading_ios.go` | `builds` — code reading, 29–30 September 2026 |
| iPadOS | Share popover; probably points mid-page rather than at the selection | Share popover at the selection | Share popover at the selection | Preview, then a share popover; probably mid-page | Probably a share popover pointing at the hidden reading view, not the icon; not seen | `reading_ios.go` | `builds` — code reading, 29–30 September 2026 |
| macOS | Share picker at the selection; the note card appears beneath | Share picker at the selection | Share picker at the selection | Preview, then the share picker with the image | Probably the share picker, anchored to the hidden reading view, not the icon; not seen | `reading_macos.go` | `builds` — code reading, 29–30 September 2026 |
| Android | The note card, then the "Sharing text" sheet with the note, citation and link | "Sharing text" sheet with the quote and citation | "Sharing text" sheet with the link | Preview, then the "Sharing image" sheet | "Sharing text" sheet, by the citation's route; not driven | `reading_android.go` | `runner` — emulator, 1.2.17, 29–30 September 2026 |
| Windows | The Windows Share sheet over the window, headed "Share link", with the citation, the link carrying the note, a QR-code button and a link button with no label (Copy link, by its icon), which shows a check when pressed; the new note card on the page beneath. By the code the note, the citation and the link go to the chosen app as text, and the link as a web link too; not seen (28) | The Windows Share sheet over the window, headed "Share", listing the apps to share to, with no title, preview or Copy on Windows 11. By the code the quote and its citation go to the chosen app as text; not seen (28) | The Windows Share sheet over the window, headed "Share link", with the citation, the link, a QR-code button and a link button with no label (Copy link, by its icon) (28) | Preview, then the Windows Share sheet over the window with the card as a file named "BibleText verse <date> <time>.png" in local time, shown in place of a title, with its thumbnail, its size and a size picker, Edit, and a copy button with no label (28) | The Windows Share sheet, as for Share with citation (28) | `share_windows.go` | `hardware` — arm64 VM, Windows 11 build 26200, the Store package workflow's arm64 executable run unpackaged, 30 September 2026 (the proof note below) |
| Linux | Copied; the "Copied — ready to paste" sheet over the new note card, with Email…, Copy again and Done (27) | Copied; the "Copied — ready to paste" sheet (27) | Copied; the "Copied — ready to paste" sheet (27) | Saved to ~/Downloads; the file manager opens on the folder; the "Picture saved" sheet, with Email… attaching the file only where a mail client, not a browser, handles mailto:, and never in the snap (27). In the snap, neither the save nor the reveal happens (the proof note below) | Copied; the "Copied — ready to paste" sheet over the card (27) | `share_other.go`, handing on to `share_fallback.go` | `hardware` — arm64 VM, X11, 30 September 2026 |

What each proof rests on:

- **Linux, `hardware`, as an exception to the rule that the artefact must
  be the shipped one.** The sheet is not yet in a release: a local build of
  the tree that adds it ran on the arm64 VM's GNOME X11 desktop (divergence
  25's machine), and every verb was driven from its own entry point — the
  selection menu's Share with citation and Share as link, the composer's
  Share, the verse-of-the-day card's icon, the image preview's Share —
  in light and in dark, with a screenshot of each sheet, and the clipboard
  read back after each text share held what the box showed. A click
  outside left the sheet up; Done, Escape and Return closed it, Return also
  after Copy again and Escape after a drag across the box; Copy again put
  the share back on a clipboard something else had taken; a light/dark
  switch brought it back with the same text, and over the verse of the day
  with the card beneath; at the narrowest the window would go, 533x640,
  it sat inside the window and below the header. With the caret put on a
  button by Tab — Email…, then Copy again, then Done, then round again —
  Escape and Return still closed it, and Space on Copy again copied; Copy
  again left Done where it was (its fill on rows 616–651 before, during
  "Copied again." and after), and Copy again and Done sat at the same
  pixels with Email… in the row and without it. Email…: no mail client is
  installed there, and the session names the Firefox snap as the mailto:
  handler (`xdg-mime query default x-scheme-handler/mailto` answers
  `firefox_firefox.desktop`, an entry in the WebBrowser category). The
  text sheets offer Email… on that; pressed, it called the Email portal's
  `ComposeEmail` with the citation as the subject and the text as the
  body, the request answered 0, and Firefox opened the `mailto:` link and
  asked what should handle it. The image sheet does not: the first build
  offered it there too, and pressed, the same answer of 0 hid a compose
  whose body was empty and whose link carried the PNG's local path as
  `attachment=`, the picture going nowhere. With a fixture desktop entry in
  the Email category named as the handler in the app's own scratch home,
  the image sheet offered Email…, and pressing it sent `ComposeEmail` the
  citation, the quote and its citation as the body, and the picture as a
  descriptor (answered 0; the session's portal, which still names Firefox,
  opened Firefox as before). With no Downloads folder the image sheet said
  only that the picture is shown in the file manager, which opened on the
  temp folder. The portal there is 1.18.4, without `SchemeSupported`. The
  snap, packed from the same executable (its `bin/bibletext` hashed equal)
  and installed over the store's for the run: Share with citation's sheet
  had no Email…, Copy again and Done at the same pixels as in the local
  build, and Share as image's had none either. It also showed what the
  image share does inside the snap: `HOME` there is the snap's own
  `~/snap/bibletext/<revision>`, which has no Downloads folder, so the PNG
  stayed in the snap's private temp folder, which nothing outside the snap
  can open; no file manager opened, and the sheet's line, that the picture
  is shown in the file manager, was untrue (`docs/BACKLOG.md`, the Linux and
  Windows share-sheet entry, Still open). A mail client's compose has not
  been seen. The row moves to the shipped artefact with the release that
  carries it. Not Wayland.
- **Android, `runner`.** The 1.2.17 build on the emulator. A simulator is not
  hardware (divergence 9), and an emulator is not either. Every verb but the
  verse of the day was driven to its share sheet; no share target was tapped.
- **macOS, iOS and iPadOS, `builds`.** Read from the code; nothing was
  launched for this table. The cells marked "probably" — the iPad's Share
  with note and Share as image, and the verse of the day on iPad and Mac —
  are recorded in `docs/BACKLOG.md` to be seen on a device. On the Mac it is
  not even known that the picker shows while the view it is anchored to is
  hidden.
- **Windows, `hardware`, as an exception to the rule that the artefact
  must be the shipped one.** This code is not yet in a release. The
  Microsoft Store package workflow built a tree that carries it,
  1517e3771, in the release configuration — cgo, `-tags gles`, the
  patched Fyne, the GLFW window and its RunNative — and packed and
  smoke-installed it for x64 and arm64 (run 36720846058, 30 September
  2026). The arm64 package's
  executable (go1.24.13, windows/arm64; its build information marks the
  tree modified, which the workflow's `go mod edit -replace` before the
  build accounts for), with the three ANGLE libraries beside it, was
  carried to the arm64 VM (Windows 11, build 26200) on an ISO, hashed
  equal there to the file in the package, and run from a folder without
  package identity, as the direct download runs. Every verb was driven
  from its own entry point in the app, in the dark theme — the selection
  menu's Share with citation and Share as link, the composer's Share, the
  image preview's Share and the verse-of-the-day card's icon — with
  screenshots over the eleven seconds after each and after each close.
  The selection was one word, "loved" in John 3:16 in the World English
  Bible, taken by a double-click, since the VM's input has no drag. Each
  verb opened Windows' own Share sheet over the app's window within about
  2–4 s, and nothing opened in the app. The link, note and picture sheets
  carry what the app handed over; the two text sheets show nothing of the
  share, so that they are the app's rests on their timing, their place
  and that nothing else on screen opens a text sheet. What the sheets
  showed:

  - Share with citation and the verse of the day: "Share" and the apps
    to share to; no title and no Copy.
  - Share as link: "Share link", the title "John 3:16 (World English
    Bible)", the verse's web-reader link, ending `/web/john/3/#v16`, a
    QR-code button, a button with a link icon and no label, and the apps.
  - Share with note: the composer's Share put the "Note from you" card on
    the page, and the sheet followed with the same title and the note's
    link (`#v16&n=…`). Pressed, the link button turned to a check and the
    sheet stayed up. The page had moved, and the card showed between
    verses 9 and 10, probably the head of the paragraph that holds verse
    16; where it is anchored was not checked.
  - Share as image: the preview's Share closed the preview, and the sheet
    opened with placeholders and filled about two seconds later:
    "BibleText verse 2026-09-30 13.51.34.png" (the VM's local time), its
    size, a size picker at "Original", the card's thumbnail, Edit, a copy
    button with no label, and the apps. The thumbnail means the sheet
    read the copy in the unpackaged app's temp folder.

  Each sheet was closed with its X, seven times in all, and no screenshot
  6 to 18 s after a close shows an in-app sheet; the "Copied — ready to
  paste" sheet stays up until Done, Escape or Return, so it would have
  been seen. A second Share with citation, and a second share from the
  verse of the day, each opened the sheet again. An Escape sent through
  the VM's input left the second citation sheet up, the next screenshot
  unchanged; its X closed it. No share target was pressed and nothing
  left the VM; besides the X, the only button pressed on a sheet was the
  note sheet's link button. Not seen: what an app chosen in the sheet
  receives —
  Windows 11's text sheet shows neither the quote nor its citation, so no
  text a verb hands over has been seen; what the link button put on the
  clipboard, which was not read back; the picture's copy button and the
  QR-code button, not pressed; the light theme; x64, natively or under
  emulation; Windows 10; and the installed MSIX. What the cells describe
  is Windows 11's sheet. The app's standard error went to a file that
  held only the line written before launch, which says nothing either way
  about a fallback; the screenshots after each close are what show none
  opened. The row moves to the shipped artefact with the release that
  carries it.

  Before the app, the mechanism was seen from a standalone probe
  executable, without package identity, on the same VM the same day, for
  text, a link and a picture, natively and under x64 emulation, every
  call answering S_OK; its sheets looked as the app's did. The sheet
  asked for the share 140–549 ms after it was asked to open, on the
  window's thread, never within the call, and the picture's file first
  resolved on a thread-pool thread. A sheet that asks after the five
  seconds the app waits before opening the in-app sheet still gets the
  share (divergence 28); none that slow has been seen, from the probe or
  from the app. The code's rules are held by tests — the session's on
  every platform, and against Windows itself, with no sheet shown, in the
  Windows CI job (`share_windows_test.go`: the objects Windows calls, a
  real DataPackage filled as the sheet's request fills it, and every step
  up to the sheet's opening for text and for a picture, each refusal
  ending in one fallback). Those tests passed on the arm64 VM, natively
  and as x64 under emulation, from a test binary built with cgo off and
  without Fyne's GLFW driver, before the handler kept for a late sheet
  and the picture taken at the tap were added; a control build with
  IAgileObject unanswered, and another with the file handler's interface
  id mistyped, each failed there. At 1517e3771 the Windows CI job, on its
  x64 server image with cgo and the race detector, passed the whole
  suite; it runs without `-v`, so its log does not say whether that image
  let the Windows share tests reach the sheet's step or skipped them
  where it refuses a share.
  The Store's MSIX is still to be seen: Windows redirects a packaged
  app's writes under AppData, the temp folder the picture is copied into
  among them, and whether the apps the sheet hands the file to can read
  it there is not known; unpackaged, the sheet read it. Behind the sheet
  it runs the Linux code — the same confirmation sheet, from the same
  files — but for two things: the file manager the image share opens
  (Explorer, with the file selected), and Email…, which opens a
  `mailto:` link through the shell (`share_email_windows.go`) when the
  shell's association API resolves a handler for the scheme — the
  reader's own choice first, a handler whose executable is on disk or a
  packaged app's — and which a link cannot carry a file through, so the
  image sheet has no Email… there. The link is kept to 2,000 characters,
  a longer share's passage cut at a word. None of that has run in the app
  on Windows, where the Share sheet opened every time: neither the in-app
  sheet, Explorer's reveal, the handler check nor a long share's link;
  `share_email_windows_test.go` holds the check against handlers it
  registers itself, in the Windows CI job.

**When a share cannot start.** Once a share reaches the platform, it says
so when it cannot complete, on both phones, in each platform's own way.
Android shows a toast from `android/BtBridge.java`: "Could not share the
passage." when a text verb's chooser cannot start, and "Could not share
the card." for Share as image. Until 30 September 2026 the text verbs had
no such path, and a chooser that could not start closed the app; driven on
the Android 16 emulator with the system chooser disabled, before and
after. iOS and iPadOS have no toast, and `reading_ios.go` says "Could not
share the card." in an alert with an OK button when the card cannot be
read — a divergence in presentation only, compiled and held at the source
but not seen. The iOS text verbs have no failure the bridge can see beyond
the last case: with no window on screen (no view controller on iOS, no
activity on Android) there is nothing to show a message on, and nothing is
shown. Two silences remain. On every platform, a Share as image card that
fails to render (`renderVerseImage`) leaves the preview empty, and its
Share then closes the sheet without handing anything over
(`share_preview.go`). On iOS, a presentation UIKit refuses (a view
controller already presenting, or being dismissed) is not detected.
Recorded in `docs/BACKLOG.md`, "Android: a text share has no failure
path".
