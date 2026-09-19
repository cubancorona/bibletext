# Platform matrix

What BibleText can be built and shipped as, on which architecture, through
which channel — and **how well each of those is actually proven**.

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
| macOS | x86_64 | Mac App Store | shipping | builds | 1, 2, 3 |
| macOS | arm64 | Direct download (.zip) | shipping | builds | 1, 2, 4 |
| macOS | x86_64 | Direct download (.zip) | shipping | builds | 1, 2, 4 |
| macOS | arm64 | Build from source (local checkout) | shipping | hardware | 5 |
| macOS | x86_64 | Build from source | untried | none | 5 |
| iOS | arm64 | App Store — iPhone | shipping | field | 6, 7, 8 |
| iPadOS | arm64 | App Store — iPad | shipping | builds | 6, 7, 8 |
| iOS | arm64 | Development install to a device | proven | hardware | 7 |
| iOS | arm64 | Simulator | proven | runner | 9 |
| iOS | x86_64 | anything | excluded | none | A |
| Android | arm64-v8a | Play — closed testing (AAB) | shipping | field | 10, 11, 12 |
| Android | armeabi-v7a, x86, x86_64 | Play — closed testing (AAB) | shipping | builds | 10, 11 |
| Android | all four ABIs | GitHub Releases — universal APK | shipping | builds | 10, 12 |
| Android | arm64-v8a | Local install / adb (debug APK) | proven | hardware | 11 |
| Android | any | Play — production | untried | none | — |
| Windows | x64 | Microsoft Store (MSIX) | shipping | runner | 14, 15, 16, 18 |
| Windows | x64 | Direct download (.zip) | shipping | builds | 14, 15 |
| Windows | arm64 | Microsoft Store (MSIX) | ready | runner | 14, 15, 16, 17, 18 |
| Windows | arm64 | Direct download (.zip) | ready | builds | 14, 15, 17 |
| Windows | x86 (32-bit) | anything | untried | none | — |
| Linux | x86_64 | Direct download (.tar.xz) | shipping | builds | 19, 20, 25 |
| Linux | arm64 | Direct download (.tar.xz) | ready | hardware | 19, 20, 25, 26 |
| Linux | x86_64 | AppImage | shipping | builds | 21 |
| Linux | arm64 | AppImage | untried | none | C |
| Linux | x86_64 | Snap Store | ready | runner | 22, 23 |
| Linux | arm64 | Snap Store | ready | hardware | 22, 23, 26 |
| Linux | x86_64 | Flathub | ready | runner | 24 |
| Linux | arm64 | Flathub | ready | runner | 22, 24, 26 |
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
   `UIBackgroundModes=['audio']`, privacy strings, `UIDeviceFamily`. *Risk:*
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
21. **AppImage tooling pinned by sha256** in `scripts/build-appimage.sh` —
    appimagetool 1.9.1 and the type-2 runtime — which is exactly why there is no arm64 AppImage yet.
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
24. **Flathub is keyless and built with `-tags flatpak`** from a *second,
    parallel* recipe that patches the vendored toolkit in `build-commands`
    rather than via `setup-fyne-patch.sh`. *Risk:* two places to apply six
    patches; they can drift.
25. Both Linux architectures run the same packaged-tarball assertions, via
    `scripts/check-linux-package.sh`, so they cannot drift apart.
26. arm64 Linux was proven on a local UTM VM, 18–19 September 2026: the
    executable builds with no new patches and passes
    `scripts/smoke-linux-launch.sh` (launch, `bibletext:` handoff, single
    instance); the snap packs in LXD, installs, exports its entry with `%u`, and
    its ALSA layout resolves inside the confinement; and the **tarball** was
    packaged with the patched CLI and put through
    `scripts/check-linux-package.sh`, which is how the `make install` defect was
    found. What has NOT happened is launching the installed application from
    where `make install` puts it — the row's `hardware` covers the executable
    and the snap, not that last step.

## Deliberate exclusions

Decisions, with the reason — so that none of these is ever mistaken for
something nobody got round to.

- **A. iOS x86_64** — Apple ships no x86_64 iOS devices. The only x86_64 iOS
  target is the simulator on an Intel Mac host.
- **B. Windows arm64** — **no longer excluded, and now built.** Neither
  assumed blocker was real: ANGLE publishes `angle-arm64` at the tag already
  pinned, and the runner's missing aarch64 compiler is supplied by a pinned
  llvm-mingw. Both channels build it; see divergence 17.
- **C. Linux arm64 AppImage** — held until an arm64 appimagetool and type-2
  runtime are pinned by sha256 to the same standard as the x86_64 pair.
  Shipping an AppImage built with an unpinned tool would be worse than not
  shipping one.
- **D. Flathub aarch64** — *resolved 19 September 2026.* It was held until the
  `aarch64` leg of `linux-stores.yml` had gone green rather than enabled on
  optimism; both the build and the smoke passed, so `flatpak/flathub.json` now
  names both architectures.

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
  The Play closed-testing build is with testers on real devices and the iOS App
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
- **The Linux tarball's installer now runs on every packaged tarball**
  (`scripts/check-linux-package.sh`), and the first run of it found `make
  install` had always failed — see docs/BACKLOG.md. Fixed. What is still
  unproven is launching the installed application from where it installs.
- **Windows arm64 has never been run by a person.** It builds, packs,
  installs and renders on the runner, which is `runner` and not `field`. The
  local Windows ARM VM is the machine that could change that.
- The owner's outstanding Linux hardware pass needs **an x86_64 machine or a
  cloud desktop**: the arm64 VM explicitly cannot stand in for it.
