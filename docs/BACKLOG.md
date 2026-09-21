# Backlog

Deferred work, one entry per item, and the record of the work that has closed.
An entry carries enough scope to be picked up cold. The rule here was to delete
an entry when the work landed; it is kept instead — marked DONE, or FIXED with
the date — and says what shipped and why. Closed entries earn their place: this
is the file to read before re-investigating a defect that may already be fixed,
and a fix's reasoning is the expensive half to reconstruct.

## One universal macOS download instead of two — DONE 17 September 2026

The direct download offers `BibleText-macOS-AppleSilicon.zip` and
`BibleText-macOS-Intel.zip`; the Mac App Store build is already universal
(`scripts/release-mac-store.sh` builds both slices and joins them with
`lipo -create`). The direct path is the odd one out, not a considered
difference.

Measured on the 1.2.10 artifacts rather than estimated: the two zips are
21.1 MB and 21.6 MB, their executables 40.8 MB and 41.6 MB, and the bundle is
essentially all executable — so a real `lipo` join of the two published
binaries produces an 82.5 MB executable and a **42.2 MB zip**. There is no
shared-resource saving to be had; universal is almost exactly double.

Three reasons it is still probably right:

- One of the two wrong choices is fatal. An Apple Silicon build on an Intel
  Mac does not launch at all; an Intel build on Apple Silicon runs under
  Rosetta, slower but working. The page cannot tell which machine a reader
  has.
- It deletes a hazard this repository has already paid for. The macOS job
  packages twice from one checkout, and `fyne package` rewrites the ledger's
  `Build` after each success — 1.2.5 shipped two zips stamped 46 and 47 for
  the same commit. The job now carries a saved ledger, a restore function, a
  trap and a two-plist comparison purely to contain that. One universal
  package needs none of it.
- It removes a button from a page whose whole virtue is that it is short, and
  a decision from a reader who may not know which Mac they own.

The cost is one thing only: about 21 MB of dead weight in every download.

Shipped in the release workflow the same day, following the Store script:
build each slice, `lipo -create`, package once. The asset is now
`BibleText-macOS.zip`; the two old names are gone, so a bookmark of either
returns 404. The job asserts both slices are in the packaged binary — a
packager that rebuilt instead of copying would ship one slice and still
look right from outside — and checks each slice's own `minos`, which is
what the loader enforces. The ledger save-and-restore stays, because the
packager still rewrites `Build` and the verification reads it back; what
went is the two-packages-one-checkout hazard that stamped 1.2.5 with two
different builds.

## The first frame is laid out for a window the app did not get

`app.go:548` asks for a fixed window size and never reconciles it with what the
desktop actually granted:

    window := myApp.NewWindow("BibleText")
    window.Resize(fyne.NewSize(1280, 860))

There is no clamp to the work area. On a desktop that cannot give 1280x860 the
window comes back smaller, but the first layout pass has already run against the
requested size and nothing re-runs it. The reading column is centred in
`styledColumn.Layout` (`reading_styled_area.go:386`) with

    x := (size.Width - w) / 2

so it is placed for a canvas nobody has: a wide dead gutter down the left and
the text crowding the right edge. It is not a flash. It persists until something
forces a fresh layout pass -- maximising, or dragging any edge -- after which it
is correct for the rest of the session.

THIS IS ALREADY IN THE REPOSITORY. `docs/windows-store-smoke-john3.jpg`, added
by 84536fd9a on 16 September, is a capture of exactly this defect from the
`windows-latest` runner (1024x768). Measured: the pane runs x=62..899, the text
x=379..806, margins 317px left against 93px right. It predates the ANGLE switch
by a day, so Direct3D has nothing to do with it. The reason it was committed
without comment is that the only check looking at that image asked whether the
app had DRAWN -- and it had.

What pins the trigger to the missing resize rather than to the width: MAXIMISING
cures it. A maximised window on an 800x600 guest is about 800x552, far narrower
than 1280, and its layout is correct. So it is not "the window must be at least
1280 wide" -- any resize event at any size reconciles it, and a launch that never
receives one stays wrong.

Invisible on the machines it is built on, for two independent reasons. A display
of at least 1280x860 grants the request exactly, so there is no mismatch to
correct, and this development Mac clears it with room to spare: GLFW reports a
videomode of 1728x1117 and a work area of 1728x1022 at y=33, measured on the
machine rather than inferred.
And the pane that mis-places the column is Windows/Linux only: `useStyledPane()`
is false on macOS, which uses the native NSTextView overlay in
`reading_macos.go` instead. The configurations that do NOT clear the size are
ordinary: 1366x768 at 125% scaling is 1093x614 logical, and 1920x1080 at 150% is
1280x720.

THE GATE IS IN PLACE BEFORE THE FIX, deliberately.
`scripts/check-reading-centred.py` measures a screenshot's ink profile, finds the
text block and its pane, and requires the two margins to agree within 15% of the
pane width. It self-tests against synthetic images with a known answer on every
run, and `msstore.yml` runs it against the committed capture above with
`--expect-fail` before judging the live one, so a rule that has stopped working
fails loudly instead of passing. Mutation-proved four ways: a limit too
permissive, a limit too strict, a blinded ink detector and a blinded column
threshold all trip the self-test rather than returning a false pass.

An earlier attempt at this gate asserted that the WINDOW fits the work area.
That check would have passed on the committed failure: in that capture the
caption buttons are fully visible and the content stops at the taskbar, so the
window was clamped correctly and it is the canvas that was not. Recorded because
the wrong invariant looked obviously right.

FIXED, by asking for a window the desktop can grant. `app.go` now calls
`startupWindowSize(startupWorkArea(myApp))`:

  - `window_size.go` holds the arithmetic and nothing else, so it is testable on
    a host with any screen or none. The work area arrives as a parameter.
  - `window_workarea_desktop.go` measures it. Fyne exposes no screen accessor at
    all -- no `Window.Size`, no monitors on `fyne.Driver`, only
    `Settings().Scale()` -- but GLFW is already linked into the binary (it is
    what the desktop driver is built on), and `Monitor.GetWorkarea` is the
    number the toolkit never asks for. `glfw.Init` is idempotent and Fyne sets
    no init hints before its own, so calling it early discards nothing;
    verified, along with a second `Init` being a harmless no-op.
  - `window_workarea_mobile.go` stubs it for ios||android, because `app.go`
    carries no build tag and so `Run` compiles there even though neither mobile
    entry point calls it.

The scale term is the whole correctness argument and it is the part the original
design shipped untested. Fyne multiplies points by
`round(system*user*10)/10`, and `SystemScaleForWindow` is per-platform: a hard
1.0 on macOS because its scaling happens at the texture level, the monitor's
content scale on Windows. Two things fell out of testing it that would each have
produced a wrong window:

  - Go rounds a half away from zero, so a 125% display becomes a scale of **1.3**,
    not 1.25. Dividing by 1.25 would leave the budget about 4% too generous --
    the same defect, just smaller.
  - This Mac reports a content scale of 2.00 while GLFW's work area is already in
    POINTS. Using content scale uniformly would compute an 864x511 work area and
    clamp the window to 860x471 on every Retina Mac. The darwin branch is
    load-bearing, and is now verified against the machine.

Verified: the seven-mutation battery on `window_size.go` (each clamp removed,
each frame allowance zeroed, the fallback inverted, the rounding dropped, the
scale ignored) is caught by the tests; the full suite, the iOS pane check and all
nine hygiene checks pass; and the app launches on this Mac with the early
`glfw.Init` and opens at 1280x892 outer -- the preferred 1280x860 content plus a
32pt title bar, granted unchanged, exactly as the probe predicted for a 1728x1022
work area.

NOT yet verified end to end: that a Windows build carrying this renders centred
at 1024x768. That is what the `msstore.yml` gate is for, and it cannot run from
this host.

TABLED, with triggers: an 8th Fyne patch making the toolkit keep the size it
was GRANTED. The root cause is that `internal/driver/glfw/window.go` sets
`w.canvas.size = size` synchronously and then calls `processResized` with the
requested figure; the patch would read back `w.view().GetSize()` after
`SetSize` and pass that instead. It was scored down during design on the claim
that a patch never reaches a build, which is FALSE -- `release.yml`,
`msstore.yml`, `linux-stores.yml` and the release scripts all inject
`go mod edit -replace fyne.io/fyne/v2=./third_party/fyne`.

Tabled anyway, for three reasons that outweigh being the root cause:

  - It cannot be tested where it runs. There is no `replace` in the committed
    go.mod, so local `go build` and `go test` use stock Fyne; the patch could
    only be held in place by asserting its TEXT, the way
    dark_mode_follow_test.go does. The clamp is plain Go with a seven-mutation
    battery behind it, and that is a much better verification loop for the case
    that actually bit.
  - Real blast radius. `processResized` also feeds fixed-size windows and
    `fitContent`, and the branch above it records the requested size
    deliberately, with a comment saying an invisible window may never get the
    event. Reading `GetSize()` back before the window is shown is precisely
    where that becomes delicate.
  - Its marginal value is over configurations there is no evidence of hitting,
    against a permanent cost at every Fyne bump.

What makes tabling safe is that the symptom now has a DETECTOR:
`scripts/check-reading-centred.py` runs on every Store build and fails with a
screenshot whenever the column is off-centre, whatever the cause. Fix the known
cause cheaply, watch for the unknown ones, and spend the expensive effort only
if the watch fires.

Do it when any of these happens:

  - the centring gate fails on a build that already carries the clamp -- that is
    a mismatch from a cause no clamp can predict, and the screenshot will say so;
  - a mixed-DPI multi-monitor report arrives. This is the clamp's one
    non-hypothetical weakness: it reads the PRIMARY monitor's content scale
    while Fyne uses the scale of the monitor the window actually lands on. It
    mostly does not bite at launch because the Win32 CW_USEDEFAULT cascade puts
    the first window on the primary;
  - tiling window managers become a supported configuration (i3, sway, yabai
    assign geometry regardless of what is asked for, so the clamp is inert
    there);
  - a Fyne bump changes the Resize path, at which point the patch and the clamp
    should be reconsidered together.

Note for whoever picks it up: upstream issue 6368 was filed and closed, so this
belongs on the fork (cubancorona/fyne, bt-main) rather than being re-filed. And
do not pin the assertion to a removed line -- a regenerated diff may emit that
hunk as pure insertions.

Still uncovered by either: window POSITION. Win32 places the first window by the
CW_USEDEFAULT cascade, the app never calls `CenterOnScreen`, and
`doCenterOnScreen` (`window_desktop.go:155-177`) centres against `GetVideoMode`
rather than the work area, so a clamped window is still a cascaded one.

## FIXED: Windows arm64

Attempted, withdrawn and fixed on 19 September 2026. The `windows-11-arm` image
ships an **x86_64 mingw gcc**, so cgo handed `runtime/cgo`'s aarch64 assembly to
an x86 assembler:

    gcc_arm64.S:30: Error: no such instruction: `stp x29,x30,[sp,'

The machine is ARM; its compiler was not. `scripts/fetch-llvm-mingw.ps1`
installs llvm-mingw's aarch64-hosted, aarch64-targeting toolchain, pinned by
sha256 the way ANGLE and the AppImage tools are. Both the Store package and the
release zip build it.

Verified on the runner rather than assumed: the MSIX declares
`ProcessorArchitecture="arm64"`, and `BibleText.exe`, `libEGL.dll`,
`libGLESv2.dll` and `d3dcompiler_47.dll` are all genuine ARM64 images
(`0xAA64`). It installed and passed the render smoke.

Neither blocker anyone expected was real: ANGLE publishes `angle-arm64` at the
tag already pinned, and the compiler was solvable by shipping one.

Still `runner`, not `field` — nobody has run it on a real Windows ARM machine.
The local VM is the obvious next step.

## The site must not be published before the release that first ships ARM

`docs/index.html` now offers **Linux — ARM** and **Windows — ARM** downloads,
pointing at `releases/latest/download/BibleText-Linux-arm64.tar.xz` and
`BibleText-Windows-arm64.zip`. Nothing produces those names yet: the jobs that
do are in this same change and have never run, so the current `latest` release
carries neither.

`scripts/check-public-surfaces.py` is deliberately forward-looking — it holds
the page equal to what the release WORKFLOW uploads, not to what is published —
so it is satisfied, correctly. The gap is one of ordering, and it is invisible
to every check in the repository:

  - publish the site before the next release and both buttons 404;
  - `scripts/publish-site.sh` is the only publisher and has no coupling to
    release state, so nothing stops that happening.

So: **cut the release first, confirm both assets are attached, then publish the
site.** Worth a check in publish-site.sh that every
`releases/latest/download/<name>` the page links actually exists on the latest
release — that would close it permanently rather than relying on remembering.

## FIXED: `make install` on the Linux tarball

Found on 19 September 2026 the first time anything ever ran the installer, and
fixed the same day. It had affected every architecture since the scheme-handler
work, because it was in the packaging template rather than the build.

The packager wrote the Makefile's icon variable without a file extension
(`Icon := "uk.co.bibletext"`) while packaging the icon as
`uk.co.bibletext.png`, so `make install` could not find it. Two consequences,
and the second is the one that mattered:

  - the target exited NON-ZERO after installing the executable and the desktop
    entry, leaving a reader an application with no icon and an error they had no
    way to interpret; and
  - because make stops at the failed line, it **never reached
    `update-desktop-database`** — so a tarball install produced no
    `mimeinfo.cache`, and a `bibletext:` link opened nothing. The scheme handler
    this whole patch exists for did not work for anyone installing from the
    tarball.

Fixed in `patches/fyne-tools-1.7.2-linux-scheme-handler.patch`: the four lines
that use the icon as a FILE NAME now say `$(Icon).png`, while the `Icon :=`
assignment and the .desktop entry's `Icon=` keep the bare name, which is what
freedesktop icon-theme lookup expects. The patch was regenerated against the
checksum-verified module zip rather than hand-edited, and applies with `-F 0`
and no fuzz. Its own test asserts both directions and was mutation-proved.

Verified end to end on an arm64 machine: `make install` exits 0, installs all
three files, and produces `mimeinfo.cache`. `scripts/check-linux-package.sh`
now requires all of that on every packaged tarball — proven to pass the fixed
tarball and fail the old one.

## FIXED: the executable was named after `cmd/desktop`, not after the app — 19 September 2026

`fyne package` names the executable after its SOURCE DIRECTORY. Ours was
`cmd/desktop`, so every channel that did not override the name shipped a binary
called `desktop`. Each consequence was user-visible:

- the Linux tarball installed **`/usr/bin/desktop`** — a generic command in the
  reader's PATH that any other Fyne app packaged from a `desktop/` directory
  installs over, and that `make uninstall` then removes from under it;
- `desktop` was the command a reader had to type to start the app;
- a Linux volume control listed the application playing narration as
  **`desktop`**;
- macOS shipped `CFBundleExecutable: desktop`, so Activity Monitor, `ps`,
  Console and every crash report named the app `desktop`, on BOTH the direct
  download and the Mac App Store build.

Windows was never affected: its packaging line already passed `--executable
BibleText.exe`, and the shipped zip and `.msix` both carry `BibleText.exe`.

**How it was found, and why nothing caught it.** The shipped 1.2.12 arm64
artefacts were run on a real Linux desktop. The snap registered with the sound
server as `ALSA plug-in [bibletext]`, the AppImage as `PipeWire ALSA
[bibletext]`, and the tarball as `PipeWire ALSA [desktop]` — the snap and the
AppImage each rename the executable on their way in, and only the plain tarball
shipped it raw. Cause was settled by copying one binary to a second filename and
running it under each name: same sha256, and the sound server's client name
moved with the filename. Meanwhile `scripts/check-linux-package.sh` had
*hardcoded* `grep -q '^Exec=desktop %u$'`, so the check ratified the name
instead of questioning it, and derived the installed-entry name from whatever
the archive happened to contain — a check that follows the artifact cannot
notice the artifact going wrong.

**Passing `--executable` per channel was not enough.** It repaired the packaged
artefacts but could never reach the two routes `README.md` advertises, because
those build on the reader's own machine from the module path: `fyne install
github.com/cubancorona/bibletext/cmd/desktop@latest` (the command the public
Fyne apps directory prints) and `go install` / `go run` of the same path.
**`fyne install` has no `--executable` flag at all**, so no packaging change
could have fixed that route.

**The root fix: `cmd/desktop` is now `cmd/bibletext`.** Every packager then
defaults to the right name instead of having to be told, the module-path routes
install `bibletext`, and macOS falls out with it. The per-platform spellings are
deliberately different, each the convention of the place it lands:

| Channel | Executable | Why that spelling |
| --- | --- | --- |
| Linux — tarball, snap, AppImage, Flatpak | `bibletext` | a command in the reader's PATH |
| Windows — zip, Microsoft Store | `BibleText.exe` | Task Manager's Description column and the AppxManifest |
| macOS — direct download, Mac App Store | `BibleText` | `Contents/MacOS/`, as `Safari.app/Contents/MacOS/Safari` reads |

Verified rather than assumed: a locally packaged bundle now carries
`CFBundleExecutable: BibleText` with `Contents/MacOS/BibleText`, and the running
process reports `BibleText` to `ps` where it read `desktop` before. On an arm64
Linux machine the renamed tree still packages a tarball containing
`usr/local/bin/bibletext`, and `scripts/build-appimage.sh` still produces an
AppImage whose desktop entry reads `Exec=bibletext %u`.

**What the macOS rename does NOT disturb**, each keyed on the bundle id
`uk.co.bibletext` or on the `NewWithID` identifier rather than on the executable
name: the App Sandbox container, the Fyne preferences directory holding notes
and reading position, the Keychain service, Universal Links, and the
code-signing identifier. The app requests no TCC-gated capability, and the Store
update path replaces the bundle wholesale. An earlier draft of this entry
deferred the macOS half on the grounds that the binary is "signed, notarised and
shipped through the App Store"; two thirds of that was wrong and the record
should not keep it — there is **no notarisation step anywhere in this
repository** (`docs/MAC_APP_STORE.md` says the Store was chosen over plain
notarisation), and the direct-download zip is not signed either. The real cost
was a signing re-run on one channel.

The one residual macOS hazard is a reader who unzips the direct download *over*
an existing `BibleText.app` instead of letting Finder replace it: they end up
with both `desktop` and `BibleText` inside `Contents/MacOS`. The bundle still
launches, because `CFBundleExecutable` names the new one, so this is untidy
rather than broken.

**Upgrade wrinkle on Linux, deliberately left to a release note.** A reader who
ran `sudo make install` from any tarball up to 1.2.12 keeps an orphaned
`/usr/local/bin/desktop` (or `/usr/bin/desktop`); the next package's `make
uninstall` removes only the new name. An `-rm …/bin/desktop` line in the
uninstall target was considered and REJECTED: it would delete a file we can no
longer prove is ours, which is the exact PATH collision this entry is about. It
is also the narrowest possible reach — it fires only for someone who installed
≤1.2.12, *then* installs the next tarball, *then* runs `make uninstall` from the
new package, while everyone who simply upgrades keeps the orphan regardless. The
numbers support the light touch: **58 tarball downloads across all 18
releases**, and until 19 September 2026 `make install` exited non-zero on the
icon line, so the population that completed an install is smaller still. The
orphan is inert — the new install overwrites the desktop entry, so nothing
launches it; the only live harm is someone typing `desktop` from shell history
and silently running 1.2.12 forever.

**DONE 20 September 2026:** the line is in the release-notes string in
`.github/workflows/release.yml`, so the first release carrying the rename tells
a reader how to remove the binary the new package's `make uninstall` no longer
names. It is a migration note, not a standing one — the comment beside it says
to drop it once the releases that shipped `desktop` are far enough back.

**Action before the next tag: the Fyne apps directory still prints the OLD
command.** Verified live on 19 September 2026 — apps.fyne.io/apps/uk.co.bibletext/
prints `fyne install github.com/cubancorona/bibletext/cmd/desktop@latest`. That
works TODAY, because `fyne install` resolves `@latest` to the newest v-tag and
shallow-clones it, and v1.2.12 still contains `cmd/desktop`. **It breaks the
moment the next tag is cut**, and it is the install route the README points
readers at. The listing lives outside this repository, so nothing here can fix
it and no check can catch it — it has to be updated by hand when the rename
ships. The same applies to any other external page that prints the command.

**Still open, lower priority: iOS ships `CFBundleExecutable: main`.** The same
class of defect with a different generic name, but it surfaces only in crash
reports and Xcode Organizer, nothing lands in a PATH, and
`scripts/release-ios.sh` passes no `--executable`. Read off the packaging line,
not observed on a built bundle.

## Verse attribution: three findings from measuring it, 21 September 2026

Prompted by a plain question — does the app really work out verse numbers by
matching text, and what about verses that repeat? The short answer is that
position is authoritative and the matching is a fenced fallback, and the
measurements bear that out: across **34,917 sampled drags over all 89 real WEB
gospel chapters**, a drag that begins inside a verse and carries an honest span
was cited correctly **every time**. Psalm 136's refrain — identical in all 26
verses — is attributed correctly in all 26 with a span. No shipped pane
dispatches a zero span: macOS, iOS, Android, the styled Windows/Linux pane and
the verse-of-the-day card all supply one, and the only zero-span dispatcher is
the legacy Fyne Entry pane, which is dead unless `styledPaneEnabledOnPlatform`
is flipped back. (Zero-span attribution on the same corpus is wrong 0.34% of the
time, which is what the span is buying.)

Three things did turn up.

### 1. A drag that opens on the previous verse's punctuation can cite the wrong verse

The real residual risk, and it is NOT the fallback. A `selSpan` narrows the
search REGION — `[start of verse lo, end of verse hi]` — which is always wider
than the drag itself. The pipeline then runs a chain of repairs (dangling
verse-number trim, mid-word start/end repair, orphan-punctuation strip, heading
strip), and **every one of them re-runs the locate from the start of that
region, first match wins**. So a trim can slide the match backwards into a verse
the reader never touched.

Begin a drag on the full stop ending verse N−1 and the span is honestly
(N−1, N). The orphan punctuation is correctly stripped from the quote — and the
re-locate then matches the earlier copy:

```
drag starts at verse 39's number   span=(39,39)  ->  Matthew 25:39   correct
drag starts on verse 38's '?'      span=(38,39)  ->  Matthew 25:38   WRONG
```
(identical shared words in both cases)

Measured on the real WEB gospels, for drags of that shape: 2.63% wrong at two
words, 1.08% at three, 0.27% at four, 0.00% by twelve. It concentrates exactly
where repetition does — the Beatitudes, Matthew 23's "He who swears by",
Matthew 25's "When did we see". Overall about 0.12% of boundary drags.

The fix is small and local: after a repair, prefer the match nearest the
previously-located index, or re-anchor the region to the SELECTION's own extent
rather than the span verses' full extent. Not done here because it is a
behaviour change to the attribution core and wants its own pass with the sweep
harness above as the before/after measure.

### 2. A Psalm superscription can be shared as scripture

`stripHeadings` and `headingOnlySelectionVerse` consult `Bible.Headings` and
never `Bible.Superscriptions`. On iOS/macOS/Android the title lives in the same
selectable text storage as the verses (not on the styled pane, which draws it
outside `p.lay.Text`), so a drag from the title into verse 1 shares
`"A Psalm by David, when he fled from Absalom his son. Yahweh, ..."` cited as
`Psalms 3:1` — editorial matter quoted as the text, under a verse reference.
The heading path deliberately returns reference-only for exactly this case;
superscriptions were simply missed when it was written. Related to the open
"quoting, notes and links" heading pass.

### 3. The omitted-verse strip rests on an untested assumption

`outboundText` strips a `[digits]` token by SHAPE, justified by a comment
claiming `outbound_text_test.go` "walks the feeds to keep that true". It does
not, and there is no sign it ever did — the file is 59 lines of hand-written
cases. The comment has been corrected to say so. The exposure is narrow but
real, and it is not only cosmetic: the strip changes the BYTE LENGTH, so a
publisher verse containing a `[12]`-shaped token would both lose it and shift
the offsets the share pipeline matches on. A superscript digit or an em/en space
in publisher text does the same. Wants a test that walks the shipped
translations.

### Also: committed test litter

`zz_probe_refute_test.go` is tracked, from commit `a0f34302d`. It looks like a
throwaway probe someone committed by accident. Left alone rather than deleted
on someone else's behalf.

## The web reader cannot render small capitals — latent until it serves an edition that uses them

Recorded because it reads like a live defect and is not one, and because the
condition that would make it real is easy to meet by accident.

The reading pane draws the divine name with Unicode small-capital characters
(see `docs/DIVINE_NAME.md`). The site ships the SAME typeface as the app, but a
deliberately tighter subset: `scripts/build-reading-fonts.sh` gives the app
`U+1D00-1D7F` and `U+A700-A7FF` and withholds both from `WEB_RANGES`, with the
reason stated in the script — the site publishes no edition that uses them.
Measured with fontTools: the desktop Junicode carries 25 of 25 small capitals at
409 KB, the web woff2 carries 0 of 25 at 27 KB. So a small capital reaching a
page would render as a fallback glyph or as tofu.

This is a considered decision, not an oversight, and the script already verifies
the APP's subset kept all 25 — they are scattered across three Unicode blocks
and a subset missing one loses letters from the divine name silently.

**It does not reach one today.** The reader publishes `/web/`, `/bsb/` and
`/webc/` only, and all three editions spell the divine name with ordinary
capitals — they carry no small-caps spans at all, because the feed behind them
has no field that could express a character style. Verified on the live site on
21 September 2026: zero small-capital codepoints across those chapters, and
`LORD` in ordinary capitals throughout. The NKJV is the only edition that marks
the name, and it is licensed: `/nkjv/` serves a notice rather than text.

**What would make it real**, any one of:

- publishing an edition whose feed carries `nd`/`sc` spans
- rendering shared-note TEXT on a page rather than a reference (today a note
  link carries a reference and the recipient's own copy supplies the words)
- any future page that echoes text the app drew rather than text a publisher
  sent

The fix, when needed, is two ranges: add `U+1D00-1D7F` and `U+A700-A7FF` to
`WEB_RANGES` in `scripts/build-reading-fonts.sh` and rebuild — the app's own
range list already names both. Cheap, but not free: the web cuts exist to be
small, and that is the trade the current ranges were chosen to make.

Note that Unicode provides no small-capital `x`, so a subset can never be
complete — see `docs/DIVINE_NAME.md`.

## What we ship has largely never been run

Found by a five-platform survey on 18 September 2026, each platform's claims
then checked by an adversarial reader that refuted 48 of them. The full picture
is docs/PLATFORM_MATRIX.md; these are the items that need a decision rather
than a document.

None of this is about architecture. It is about the gap between "CI is green"
and "somebody ran the thing we give people".

  - **macOS: every row is `builds`.** Nothing we ship has ever been launched
    from the artefact we ship. PARTLY ADDRESSED 19 Sep 2026: the local rehearsal
    now applies the Fyne patches and verifies the atomic preferences writer
    reached the binary, with a control string — measured, a stock build carries
    0 markers and a patched one 1, so the rehearsal really had been running the
    unpatched writer. Three axes remain: arm64-only rather than universal, a
    development certificate rather than the Store one, and no provisioning
    profile.

  - **No shipped Android artefact has been run.** Every recorded run installs
    the DEBUG apk; the universal sideload APK and the Play AAB have never been
    installed or launched, and the whole channel is host-locked to a single
    machine. DONE 19 Sep 2026: the compile half — `scripts/check-android-pane.sh`
    cross-compiles android/arm64 (preferring the pinned NDK r27, falling back to
    whichever NDK is present and saying which it used) and runs in CI beside
    the iOS gate, proven by a type error that `go build ./...` cannot see. What
    remains is RUNNING a shipped artefact.

  - **The iOS store artefact has never run on a device.** The on-device evidence
    in the repository concerns development builds. `UIDeviceFamily` — the single
    property that makes the app universal iPhone+iPad — is never read back out
    of the exported .ipa, though other Info.plist keys are.

  - **The Linux tarball's installer now runs, and fails.** See the entry above:
    `scripts/check-linux-package.sh` exercises it on every packaged tarball and
    the target exits non-zero on the icon line. What remains unproven is
    any workflow, script or doc. And the AppImage's type-2 runtime is never
    executed: the smoke runs `./squashfs-root/AppRun`, the extracted payload.

  - **windows-build.yml has zero runs, ever.** It is registered and
    dispatch-only, so no push has compiled it either.

  - The owner's outstanding Linux hardware pass needs an x86_64 machine or a
    cloud desktop; the arm64 VM explicitly cannot stand in for it.

The cheap wins are probably the compile checks (a `GOOS=android` gate costs
nothing) and launching the packaged macOS artefact rather than a development
rebuild. The expensive one is deciding what "tested" should mean for a store
binary we cannot run in CI at all.

## Headings: quoting, notes and links need one deliberate pass

Four heading defects were found and fixed on 18 September 2026 (selection led
by a heading, a drag begun inside one, a drag that ended in one, and a heading
selected alone). Each was found by pulling the thread of the one before it,
which is the signal that this area has never had a deliberate pass -- only
repairs. What follows is what is still open, or was decided quickly and should
be decided properly.

THE RULE IS ASSERTED IN ONE PLACE AND BROKEN IN OTHERS. `headings_test.go`
states that a heading must never reach sharing, yet two live breaches turned up
in an afternoon: a drag ending inside a heading carried the heading's words into
the quotation, and a heading selected alone quoted itself as scripture. Both are
fixed, but neither was caught by anything. The rule needs enforcing at the point
every quotation is produced, not asserted once and hoped for. Audit every path
that yields quoted text: `prepareShareQuoteIn`, `cleanQuoteTextIn`, the share
card, the note composer's preview, the AI panel's quotation, the web reader.

HEADING STYLES ARE NOT ALL ALIKE, AND THE CODE TREATS THEM AS IF THEY WERE.
`Heading.Style` carries the publisher's own classification -- `s`/`s1`..`s4` for
section heads, `ms` for a major section, `r`/`mr`/`sr` for a parallel-passage
reference line, `qa` for an acrostic letter. `stripHeadings` treats all of them
the same. They are not the same thing to a reader:

  - a reference line is "(Matthew 3:1-12)" -- arguably a citation the reader
    might legitimately want to copy;
  - an acrostic letter is a single Hebrew letter name standing over Psalm 119's
    stanzas, and the heading-only rule would anchor a note on "ALEPH" to the
    stanza's first verse, which may be right, or may be surprising;
  - a major section head spans many chapters' worth of material.

PSALM SUPERSCRIPTIONS ARE A DIFFERENT TYPE ENTIRELY and were not touched.
`Superscription` is its own struct, the timing tables carry verse-0 rows for it,
and nothing in this work asked what happens when one is selected, noted or
shared. It is the nearest neighbour to a heading and behaves differently.

THE HEADING-ONLY DECISION WAS TAKEN QUICKLY. A heading selected alone now
anchors to `Heading.BeforeVerse` and quotes NOTHING -- reference only. The
reasoning: a note anchored to a heading cannot place for a recipient, because
headings are the publisher's editorial matter and a shared link opens the
public-domain web reader whose headings are not the licensed edition's; and
quoting the verse would attribute words the reader did not select. That holds
up, but it was one judgement made in one exchange, and it deserves to be
revisited alongside the style question above rather than standing as settled.

THE NATIVE RULE IS STILL BACKWARD AT THE START. All three native panes resolve
an offset to a verse by the last verse-number run at or before it, so an offset
inside a heading resolves to the verse ABOVE. The fix corrects the attribution
downstream, in the shared normalizer, which is why one change reached macOS, iOS
and Android at once. But the raw span is still wrong at source, and any future
consumer of it inherits the defect. Worth deciding whether the native rule
should look FORWARD when the offset falls in heading matter -- it is the same
asymmetry noted in the arrival work: `hi` uses the inclusive last character and
is right, `lo` uses the same backward rule and is not.

WHAT A READER ACTUALLY WANTS IS THE UNANSWERED QUESTION. Everything above is
mechanism. Nobody has asked what selecting a heading should MEAN: the section it
introduces, the heading as a label, or nothing at all. The answer probably
differs by style, and it decides the rest.

## The Windows audio smoke is probably testing a silent sink

`windows-audio-smoke.yml` has failed repeatedly at the natural-end step, and
the investigation has been looking for a defect in the app. The likelier
explanation is that the test's central premise does not hold on the machine it
runs on.

Its header says it "plays a REAL narration chapter through the REAL desktop
audio engine (oto/WASAPI) on a real Windows machine", and that "samples are
decoded and submitted to WASAPI, playback position advances in real time". But
a GitHub-hosted Windows runner is a headless VM with NO audio endpoint. oto
v3.4.0 tries WASAPI, then WinMM, and when both report a missing device it falls
back to `nullContext` -- a sink that accepts samples and discards them. Nothing
is submitted to WASAPI because there is nothing to submit to, and a wait for
playback to reach its natural end is then waiting on a clock that is not the
one the test assumes.

The counter-evidence, measured 18 September 2026 on the UTM Windows guest,
which DOES have an audio device: with a chapter playing, `AUDIOSES.DLL` and
`MMDevAPI.dll` are both mapped into the process, so oto opened a real WASAPI
render session there and took no fallback. The same check on a runner would
show neither. So the app is fine and the harness is measuring something else.

What to do, in order:

  1. Make the smoke SAY which backend it got, and fail loudly rather than
     mysteriously when it is the null sink. A test that cannot distinguish
     "audio works" from "there is no audio device" is not evidence either way,
     and right now its failure is being read as the former. The cheap probe is
     the one used on the guest: the process's loaded modules, filtered for
     `audioses|mmdevapi`. Assert the expectation explicitly so a runner without
     a device reports that fact instead of timing out.
  2. Correct the workflow header and docs/PLATFORM_MIMIC.md, which both state
     that this proves the WASAPI backend. On a device-less runner it cannot.
  3. Only then look again at the natural-end path, on a machine that has an
     audio device -- the UTM guest is one, and so is any real Windows box.

Until 1 is done, a red run of this workflow says nothing about whether Windows
audio works, and the repeated failures should not be read as a shipping risk.

## The direct downloads do not register `bibletext:` links

Three channels hand a shared link back to the browser because nothing has put
a handler in front of the system. The packaged builds are fine: the Linux
tarball installs a desktop entry with `MimeType=x-scheme-handler/bibletext;`,
and the Store MSIX declares `uap3:Protocol`. The downloads people actually
take from the site mostly do not.

| Channel | State | What is missing |
| --- | --- | --- |
| Linux tarball | works | — |
| Microsoft Store MSIX | works | — |
| Flatpak, Snap | works when published | — |
| **AppImage** | no | nothing installs its desktop entry, and the entry inside says `Exec=bibletext`, a command that is not on `$PATH` |
| **Windows .zip** | no | no `HKCU\Software\Classes\bibletext` registry entry |
| **macOS direct .zip** | no | no `CFBundleURLTypes` in the packaged plist |

The AppImage is the one worth understanding, because the metadata is already
right and still does nothing. `linux/uk.co.bibletext.BibleText.desktop` is
packed inside the image and carries both `Exec=bibletext %u` and the MimeType;
the published file was checked and it is there. It fails twice over. A desktop
entry only becomes a handler once it is in `~/.local/share/applications` and
`update-desktop-database` has run, and running a single file installs nothing
— that is the format's whole point, and the entry inside is read only by
integration daemons most people do not run. And `Exec=bibletext` names a
binary on `$PATH`; an AppImage lives wherever the reader dropped it, with its
real path in `$APPIMAGE`, so even a copied-out entry would point at nothing.

Each fix is a first write outside the app's own directories on its platform,
which is why all three are deferred together rather than one at a time:

- **AppImage** — a Settings switch that, when `$APPIMAGE` is set, writes a
  user desktop entry with `Exec="$APPIMAGE" %u` and the MimeType, then runs
  `update-desktop-database`.
- **Windows .zip** — `HKCU\Software\Classes\bibletext`, gated on not being
  packaged (the MSIX must never race its own manifest), with a Settings
  switch to remove it.
- **macOS direct .zip** — `CFBundleURLTypes` in the packaged plist. The
  cheapest of the three: the delegate's `openURLs` entry point already
  exists, so this is plist work rather than new code.

Settle the stance once and apply it to all three: whether the app registers
itself silently on first run, on a Settings switch, or not at all. A reader
who downloads a zip and expects a link to open in the app has a reasonable
expectation; a reader who finds their URL handlers rearranged without asking
does not.

## The download page, grouped by platform

`docs/index.html` lists desktop downloads as a flat run of buttons: two
macOS, one Windows, and since 1.2.10 two Linux. Flat is still the right shape
at that size, and the canonical alternative — a heading per operating system
with the Linux formats as a sub-list — is what projects with many Linux
formats settle on because Linux always accumulates them.

The trigger is Flathub and the Snap Store going live. Linux then has four
ways in (tarball, AppImage, Flatpak, Snap) and a flat list stops answering
the only question a reader has, which is which one they want. Restructure
then, not before.

Two things to fold in at the same time:

- A line saying which Linux download to take. They are not equivalent: the
  tarball registers `bibletext:` links and the AppImage does not.
- Whether macOS is still two buttons by then (see the universal entry above).

Not worth doing: operating-system detection that picks for the reader. It is
the other canonical pattern and it is what the large projects do, but it
needs a no-JavaScript fallback that lists everything anyway, which is the
page as it stands.

## Age ratings: aim for all ages in every market

The IARC questionnaire answered for the Microsoft Store on 17 September 2026
mirrors the answers Google Play already carries, so the two stores say the same
thing. It yields IARC 3+, PEGI 3, Microsoft 3+, ClassInd L (all ages), CCC 8+,
ESRB Everyone 10+, USK 12 and PCBP 18+ — adults only in Russia, and twelve and
over in Germany, for a reader whose content is Scripture set as text with no
images at all.

The goal is all ages, everywhere. Three answers carry the rating up, and all
three came across from Play: violence against humans in a realistic setting
with realistic reactions and mild or limited blood and gore; references to
sexual activity without descriptive detail; and references to sexual violence.
Each is defensible for the text of Scripture, and each is exactly what a
ratings board reacts to.

Settle one question before changing any of them: what IARC intends by these
questions for a work of literature rather than a game. Several of them, and
their own tooltips, are written for interactive products — "the perpetrator is
able to commit them without penalty", uninvolved characters "visibly vulnerable
to the attacks" — and this app depicts nothing. It renders text; the violence
in it is referred to, never shown. Whether "Violence, Blood, or Gory Images"
is answerable as No at all for a Bible reader is the hinge, because every
raised rating follows from that one Yes.

Doing it properly means re-taking the questionnaire on BOTH stores in the same
sitting, so they do not drift apart, and reading each summary back afterwards.
The Store questionnaire is re-takeable from the submission's Age ratings page;
Play's is under App content -> Content ratings -> Start new questionnaire. A
retake replaces the certificate, so do it while no submission is in review.

## Verse of the day: possible later additions

The 10 Sep 2026 rework (branch verse-of-day: civil-date key with the device
zone on phones, frozen hash order, passages with alternates for the Catholic
edition's own books, the reading face on the card, Share in the passage's own
chapter, the desktop Escape guard, an offline snapshot of the rotation) left
these undone by decision or by scope. Each is small enough to pick up cold;
none is owed.

- **A page at bibletext.co.uk/today** — one verse, one "Read in context" link,
  no archive. Static, so the page must pick with the same civil-date
  arithmetic in JavaScript and be generated from the same commit as the app,
  or the two drift. Needs the site-root allow-list
  (cmd/websitegen/licensed_exclusion_test.go), its own hashed CSS/JS pair, and
  the Smart App Banner treatment for the link (a same-domain link stays in
  Safari on iPhone). Days of site work. Settle first whether one verse and one
  link fits "not a page".
- **A first-open-of-the-day surfacing** — a small dot on the sparkle until
  opened, at most. A modal on first open was declined: the desktop foreground
  hook fires on every alt-tab back, and an unbidden daily modal is the closest
  thing to an engagement mechanic on the list.
- **Yesterday / tomorrow** — declined as a feed by another name; the reader
  who missed a day has the reading pane. Revisit only if the stance changes.
- **A spoken label for the sparkle** — the toolkit has no accessibility layer,
  so nothing Fyne-side can name the button for VoiceOver or TalkBack. The
  Settings caption is the mitigation. If accessibility is ever taken on, do
  the header as a whole, not this one control.
- **The sparkle in phone landscape** — landscape strips every header control
  but the chapter arrows (reading_fullscreen_row.go), so the card is
  unreachable there. A 22 pt newIconTapButton beside the arrows, plus the
  count-pinning tests. A policy exception, not a bug, so it waits on a
  decision rather than a fix.
- **A feast overlay** (Christmas, Good Friday, Easter as substitutions on the
  civil date) — declined 10 Sep 2026. Hours if ever wanted; Western Easter
  only.

The next pieces that ARE wanted: one opt-in local notification a day, iOS
first, then Android after the Play closed-testing review clears — see the
rework's proposal for the design (calendar triggers carrying the full date,
re-issued on foreground, the passage computed with the fixed day key).

## Android's compact page: a pill under a heading or a title stands down

The one surface left out of the pill rule (below). On Android's compact page
— phone landscape reading — `Html.fromHtml` puts no blank separator line
between paragraphs, so the heading's tail rides as an ascent-mode `AirSpan` on
the next paragraph's first line and `btPillStackInkTop` has no separator line
to measure; it returns -1 and the pill keeps the band-top placement. A psalm
title gets no air on that page at all (`TITLE_GAP_EM` is applied only in the
blank-line branch). Centring there needs the ascent-mode air read back off
the span rather than off a line. Rare: it needs landscape reading, a note,
minimized, on the paragraph a heading or a title opens.

## A collapsed note pill directly under a section heading — DONE 12 Sep 2026

The surfaces disagreed: iOS and the web stood the centering lift down when
the air above was not the page's rhythm, Android centred in the measured
ink, and the styled pane lifted by the paragraph gap even under a heading,
where the air above is only the 0.35em tail. One rule now, the spec's own
doctrine: the stack centres in whatever air the page put above its paragraph
— the paragraph gap, a heading's tail, a psalm title's gap — and stands down
only at 0 or beside an open card. The styled layout records each band's
separator (`noteBand.SepAbove`), iOS reads the previous paragraph's
after-spacing without the rhythm gate, macOS gained the same mirror
(`btMacPillSeparatorLift`), and the web's `.text .sec + .notechip` and
`.text p.pst + .notechip` centre in `--htail` and `--tgap`.

## A non-default translation served from a superseded cache epoch is never refreshed

`loadVersionFromCacheOnly` serves a superseded-epoch cache by design (an
offline upgrader keeps their Bible), and `triggerFullDownload` upgrades it in
the background — for the DEFAULT translation only (`fullPending` is computed
for it; app.go's refresh targets `defaultVersionID`). A reader whose current
translation is another one, served from an old epoch at startup, keeps the old
decode until they switch away and back (the switch path, `loadVersionData`,
drops a stale cache and refetches). Seen 11 Sep 2026 on a Mac reading the WEB
Catholic edition from an epoch-5 cache four epochs behind. Fix: record that
the current translation was served superseded and refresh it as well; the
version-state tests (docs/VERSION_STATES.md) are the place to pin it.

## Microsoft Store: from the reserved name to the first submission

The name `BibleText` was reserved on 16 September 2026 (Store ID
9NDCCZH9RB9K; identity in msstore/identity.json) and the package pipeline
exists (.github/workflows/msstore.yml builds the unsigned MSIX and
smoke-installs a signed copy). docs/WINDOWS_STORE_LISTING.md is the listing.
A reservation lapses three months after it is made, so the first submission
is due by mid-December 2026. Left to do, in order:

1. **Windows renders through OpenGL, which Windows does not guarantee.** The
   toolkit draws with desktop OpenGL 2.1, and a Windows machine with no
   graphics driver offers only the generic OpenGL 1.1 from 1996, so the app
   does not start there. Ordinary laptops all have drivers; virtual machines
   and clean server installs do not, and the GitHub runner demonstrably does
   not. Whether Microsoft's certification hosts do is unknown: nothing has
   been submitted, and the claim that they are driverless is an analogy, not
   an observation. Two Store policies bite if they are (10.1.2 fully
   functional, 10.4.2 must start and stay responsive).

   **The route to take is ANGLE, not Mesa** (established 17 Sep 2026 against
   primary sources; the earlier entry here prescribed `SetDllDirectory` and
   was wrong twice over, so it is replaced rather than amended):

   - Windows guarantees Direct3D on a driverless machine, through the
     in-box software rasteriser WARP, and guarantees no useful OpenGL. That
     asymmetry is why Store games are Direct3D and why the apps that do need
     OpenGL ship a translator. Chrome, Firefox, Qt 5 and Krita all bundle
     ANGLE, which turns OpenGL ES into Direct3D; Microsoft's own porting
     guidance names it.
   - The toolkit already has that path: `-tags gles` selects
     `internal/painter/gl/gl_es.go` and an ES 2.0 context. Those same ES
     shaders ship from this repository to iOS and Android every day.
   - So the Windows build switches to the ES path and carries ANGLE's
     `libEGL.dll`, `libGLESv2.dll` and `d3dcompiler_47.dll`, in the Store
     package and the zip alike. One code path: the GPU renders where there
     is one, WARP where there is not. **This deletes the driver detection,
     the registry probe and the policy 10.4.1 message entirely**, rather
     than solving them.
   - Measured: 10.4 MB on disk from a pinned third-party build of ANGLE
     (there are no official binaries), against Mesa's larger bundle.

   **A correction to keep:** a bundled Mesa `opengl32.dll` beside the
   executable *does* override the system copy, because the executable's own
   folder is searched before the system folder and it is not a protected
   name. Our own Store smoke proves it. The static import only defeats the
   `SetDllDirectory` variant. So Mesa beside the exe, inside the MSIX only,
   is a zero-code fallback if ANGLE does not work out, at the cost of
   putting every Store reader on a software or wrapper renderer.

   **The decisive test comes before the work**
   (`.github/workflows/windows-gl-probe.yml`): build with the tag on the
   runner, put ANGLE beside it, launch, and assert on PIXELS rather than on
   the process still being alive, with a control run that deletes the DLLs
   and must fail. A pass that cannot fail proves nothing. Open until that
   runs: whether it links under mingw, whether ANGLE's Direct3D backend
   initialises on WARP in a runner session, and the frame cost.

2. **Screenshots** — DONE 16 Sep 2026: four captured on the runner at
   1600×960 (the window's client area, above the Store's floor) by
   `.github/workflows/windows-screenshots.yml`, committed under
   `docs/screenshots/windows/`, uploaded by hand at submission.
3. **Entra tenant and application** — DONE 16 Sep 2026: tenant
   `bibletext.onmicrosoft.com`, application `bibletext-store-api` with the
   Manager role, credentials in the login Keychain, `scripts/msstore-env.sh`
   and the read side of `msstore/msstore.py` proven against the account. The
   write side (create, upload, commit, poll) is written against the second
   release.
4. **First submission by hand** from the listing document, with the
   publishing hold on; then `msstore/msstore.py` for later releases.
5. **Links** — done and pushed 16 Sep 2026, proven on the runner for the
   `bibletext:` scheme and the single-instance handoff; the web-to-app
   handler (https links through the shell) and the browsers' scheme prompts
   still need a Windows 11 client. The ordered checklist for that sitting,
   and the sideload script that installs the workflow's artifact, are in
   `docs/WINDOWS_STORE_LISTING.md` under "Resuming with a Windows machine".
   Still unproven anywhere: the window coming to the front under XWayland
   on Linux, and whether the direct-download exe's loopback listener draws
   a firewall prompt on Windows (both on that checklist).
   Deferred from the same change: an "Open in BibleText" affordance on the
   reader pages, not only the notice pages. The Windows registry entry and
   the macOS `CFBundleURLTypes` moved to "The direct downloads do not
   register `bibletext:` links", with the AppImage, because all three are
   the same decision.

## Linux stores: from the listing source to three live channels

Prepared 16 September 2026 (`docs/LINUX_STORES.md` is the listing and the
runbook; `linux/listing.toml` the source; `cmd/linuxmeta` the generator).
Nothing has run on a Linux runner yet. In order: the account holder's
decisions (developer name, metadata licence, the Flathub AI disclosure) and
the Snap Store name registration; the first `linux-stores.yml` dispatch;
the next release; an afternoon on a Linux desktop; the Flathub PR, the snap
promotion, the AppImageHub PR, the site's verification token. The four
listing screenshots are captured (`docs/screenshots/linux/`, pinned by
`screenshots.ref`; re-dispatch `linux-screenshots.yml` to replace them).
Deferred with it:

- **AppImage registering the `bibletext:` scheme itself** — moved to "The
  direct downloads do not register `bibletext:` links", which gathers it
  with the Windows and macOS halves of the same decision.
- **oto 3.5 on Linux (pure-Go PulseAudio)** — needs Go 1.25 across CI and
  the release; would drop `libasound2-dev`, the snap's ALSA plumbing and the
  fallback question in both sandboxes.
- **An SVG of the shipped icon** — Flathub prefers one; the full-bleed PNG
  draws a quality note, not a rejection.
- **Snap preferences under the per-revision data dir** — a revert restores
  older notes; `XDG_CONFIG_HOME=$SNAP_USER_COMMON/.config` would opt out.
- **Bump the Flatpak runtime to 26.08** when the golang SDK extension has
  that branch.

## Recapture the App Store and Play screenshots — deferred from 1.2.7

1.2.7 shipped with the screenshot set inherited from 1.2.5, which in turn
inherits from the last captured set (build/appstore/screenshots-1.2.3/). Those
images predate two changes 1.2.7 makes to the text itself, so the store pages
show typography the installed app no longer uses:

- the reading face is now Junicode, with Ezra SIL for Hebrew, and the divine
  name is set in small capitals; and
- section headings and paragraph breaks now follow each publisher's own
  typesetting rather than a rule of our own.

Five of the eight iPad shots are reading views, so most of the set is affected.
`appstore/preflight.py` flags this every run — screenshots are a PER_RELEASE
field, and an inherited value is reported as "INHERITED from <previous>" under
"PER-RELEASE FIELDS THAT WERE NOT WRITTEN FOR THIS RELEASE". Expect that
warning until the set is recaptured; it is accurate, not noise.

What a recapture needs:

- iOS: the 6.9" iPhone and 13" iPad slots, in the pixel sizes
  `ACCEPTED_SCREENSHOT_SIZES` lists in appstore/preflight.py. An off-list size
  does not error at upload — it reaches assetDeliveryState FAILED silently
  later — so validate locally first, which the preflight does for
  `build/appstore/screenshots-ready-<version>/en-GB/` and `.../ipad13/`.
- macOS: landscape only, per the same table.
- Play: `docs/PLAY_LISTING.md` carries its own capture recipe, including
  `scripts/play-shot-check.py` and the boot-the-emulator-before-setting-the-
  appearance rule. The Play candidate set has the same staleness and is noted
  there.

Worth capturing the new reading face deliberately rather than incidentally: a
passage with the divine name in small capitals, and a chapter whose publisher
paragraphing differs visibly from the old uniform rule, are the two images that
show what changed.

## NKJV cross references: the panel's second pass — PAUSED 10 September 2026

The edition's own cross-reference apparatus is captured, resolved and drawn
in the cross-references panel behind the `nkjvxrefs` build tag (commits
ecdeb1cad, 94385ac27; the design memo and the decision are in
`docs/SCRIPTURE_WORKLIST.md` S20 and `docs/SOURCE_FIELDS_DECISIONS.md`).
What is DONE and stays: the decoder keeps each tagged citation as a span
(`Footnote.Refs`, no cache-epoch bump); `publisher_xrefs.go` resolves the
notes; `crossref_list.go` composes the panel in blocks; the flag is set for
the NKJV only in `versions_nkjvxrefs.go`; `scripts/run-ios-device.sh
--nkjvxrefs` builds it for a phone; the canon census and the copyright-line
assertion are in `apibible_live_test.go`; the tests carry controls.

What the first look on the phone found — and `TestRenderCrossRefPanel`
reproduces at phone size without a device — is that the presentation is
wrong, in ways a reader sees as "confusing and not consistent":

1. Rows run off the right edge. The Treasury's accordion title ("More
   references — Treasury of Scripture Knowledge") is wider than the panel
   and forces the whole list to that width, so every row is clipped, the
   parallel rows' previews included. A defect, not a design choice, and the
   biggest single cause.
2. Two row idioms in one list: the Treasury and parallel rows are cards
   (bold accent reference, preview, tap the card); the publisher's rows are
   a small grey verse number over a paragraph with inline links.
3. Two heading styles: the small bold block heading and the accordion's
   large bold button with a chevron.
4. The verse number repeated on every row of a single-verse selection.
5. The edition's notice shown twice, in the block and in the footer.
6. A lone parenthesised "compare" citation standing as a whole row,
   explained only by a legend further down.
7. The empty state (a verse with no note) is a heading, a line, a long
   notice, then the accordion — the notice dominates a screen with nothing
   to show.

WHERE IT IS GOING — agreed in principle, not yet built. One panel, not two
stacked in one:

- One row idiom everywhere. The publisher's references become the same
  cards the Treasury uses — bold reference, preview, tap the card — one
  card per cited passage, in the publisher's order, never re-sorted or
  capped, labelled from the tagged id ("John 19:39", so a continuation
  such as "19:39" is readable). The parenthesised "compare" citations
  become one muted "Compare: (1 John 4:9, 10; Rev. 1:5)" line under the
  cards of the note they belong to — kept, self-explanatory, no legend.
  Full-card taps also settle the small-tap-target concern the memo raised.
  This reverses the memo's verbatim-per-note row; the reason the memo gave
  (unreadable continuation labels) does not hold when the label comes from
  the id rather than the citation text.
- One heading style: small muted section labels — NKJV · Treasury of
  Scripture Knowledge — with the Treasury folded behind a plain "Show N
  more references" link rather than an accordion, which also removes the
  width defect.
- Verse labels only where they carry information: a small "Verse N"
  divider when the selection spans several verses, nothing otherwise.
- The notice once, in the footer, with the sources line naming each set
  once: NKJV (Thomas Nelson) · Treasury: OpenBible.info (CC-BY) · Gospel
  parallels.
- The empty state in one line — "No NKJV cross references for this
  verse" — with the Treasury shown open beneath.

This touches the presentation layer only — `crossref_list.go`, plus a small
helper in `publisher_xrefs.go` that splits a note's untagged "compare"
remainder from its tagged citations — and the tests in
`crossref_list_test.go`, which pin the current composition and must be
rewritten with it. The decoder, the data, the resolver's order and the
build tag are unchanged by it.

GATES. Nothing displays in a store build until the API.Bible licensing
reply (sent; awaiting) says it may; the second pass can be built and looked
at on a tagged device build meanwhile. On a yes: move the one line in
`versions_nkjvxrefs.go` into the registry entry and drop the tag; add the
dedicated copyright page the terms' §7 asks for (a standing pre-ship item,
`versions_ui.go`). On a no: nothing to unwind.

TO RESUME. Build for the phone: `scripts/run-ios-device.sh --nkjvxrefs`.
See the panel without a device: run `TestLiveAPIBibleFullCanon` with
`BIBLETEXT_FULL_CANON_OUT` set (19 s on the Pro plan; the decoded canon is
licensed text and must stay outside the repository), then
`BIBLETEXT_RENDER_XREFS=<that file> BIBLETEXT_RENDER_OUT=<dir> go test -run
TestRenderCrossRefPanel -v .` writes seven phone-sized PNGs (John 3:16 on and
off, John 3:1-3, Matthew 3:1 with parallels, Psalm 3:1 with a title note,
Psalm 3:2 empty). Look at those before and after the change; the seven
findings above are all visible in the "before" set.

## FIXED: Share as image failed silently on Android 6.0-9.0 (API 23-28) — 20 September 2026

Not a 1.2.7 regression: the behaviour is as old as the feature, and Android has
never shipped through a store, so nothing in the wild is affected. It surfaced
after 1.2.7 was already with Google for review, which is why the entry weighs
superseding the in-review build at the end.

`BtBridge.shareImage` (android/BtBridge.java) publishes the rendered verse card
through `MediaStore` rather than a FileProvider, and the manifest declares
`WRITE_EXTERNAL_STORAGE` capped at `maxSdkVersion="28"` to justify it
(cmd/mobile/AndroidManifest.xml). On API 29 and later, scoped storage means the
`MediaStore` insert needs no permission at all and the feature works. On API
23-28 that permission is runtime-granted, and the app never calls
`requestPermissions` for it — the only runtime request anywhere in the app is
POST_NOTIFICATIONS in BtAudio.java. So the insert fails, and because the whole
body is wrapped in `catch (Throwable)` that only logs, the reader taps "Share as
image" and nothing happens: no share sheet, no error, no toast.

Two things to fix, and they are separable:

1. The silence. A swallowed `Throwable` on a user-initiated action should say
   something. Cheapest correct fix even if the permission work is deferred.
2. The permission. Either request WRITE_EXTERNAL_STORAGE at runtime on API
   23-28 before the insert, or drop the API 23-28 path and gate the "Share as
   image" action off below API 29 so the action is absent rather than broken.
   Dropping it also removes the last justification for declaring the permission
   at all, which is worth having: an unused declared permission is the kind of
   thing a Play reviewer's unused-permission check flags, and the manifest
   comment already records that READ_EXTERNAL_STORAGE was removed for exactly
   that reason.

Whether to supersede the in-review build for this is a judgement call about
reach: API 23-28 is a small and shrinking slice, and the closed testers will
almost certainly be on API 29+. Fixing it in the next release is defensible.

**FIXED 20 September 2026, both halves.**

The silence went first, because it is the part that applies on every API: the
two ways out of `shareImage` that said nothing — a bare `return` when the
insert yields a null URI, and the `catch (Throwable)` that only reached logcat
— now both put a message on screen. A share the reader started cannot end in
silence, whatever the cause.

The permission is requested on 23..28, fire-and-forget, matching
`maybeRequestNotifPermission`: GoNativeActivity overrides no
`onRequestPermissionsResult`, so there is no callback to resume on, and the
honest behaviour is to ask and say "allow it, then share again" rather than
leave a share the reader thinks is still coming. The feature was NOT gated off
below 29 — the option that also drops the manifest declaration — because
telling a reader why something needs permission is better than removing the
action from under them; if the unused-permission concern returns, gating is
still available and this entry records the trade.

**The floor is 23, not 21.** `Context.checkSelfPermission` and
`Activity.requestPermissions` do not exist below API 23, and the app's minSdk
is 21 (asserted twice in `scripts/build-android.sh`). On 21-22 the manifest
declaration is granted at install, so there is nothing to ask for; the guard
reads `SDK_INT >= 23 && SDK_INT < 29`. This is the kind of detail that is
invisible to reading and obvious to a compiler — which is why the second half
of this fix is a new gate.

### The gap that let it live: nothing compiled the Java

`scripts/check-android-pane.sh` closed this gap for the android-tagged GO
sources. The Java on the other side of the JNI boundary — `BtBridge` and
`BtAudio`, 78 classes carrying the native selectable TextView, the selection
menu, the media session and the foreground audio service — had no gate
anywhere. `go build ./...`, `go vet`, the whole suite and both mobile pane
checks are blind to it, and the only thing that ever compiled it was a full
`scripts/build-android.sh` run on one machine with the SDK installed.

`scripts/check-android-java.sh` now compiles it in seconds with the same recipe
the real build uses (`javac --release 8 -Xlint:-options -cp android-36/android.jar`),
fails rather than skips on a runner with no SDK, and refuses to pass if it
produced implausibly few classes — a gate that compiles nothing reads as a
pass. It runs in `ci.yml` beside its Go twin. Proved against two controls: an
ordinary typo, and a Java 9 API used in an API 8 source, which is what makes
`--release 8` load-bearing rather than decorative.


## Phone landscape reading — shipped on both phone platforms — DONE

Shipped and on by default on both phone platforms, verified on an iPhone
simulator and now on a physical Pixel. Nothing outstanding; the entry below is
the record of what was built and why.

Landscape on a phone was the layout nothing was designed for. What shipped
before this work (`compactNavRail` in ui_mobile.go, `mobileRailWanted` in
layout.go, docs/IPAD.md): iPhone keeps the bottom tab bar in every orientation; Android
phones move the destinations to the left rail in landscape because the
fixed-height header, history strip, chapter toolbar and bottom bar can consume
the whole short edge; iPad and the desktop use the rail in landscape. So an
iPhone turned sideways gives the reading pane the least height of any surface,
and the chrome that costs it — header, history, chapter toolbar, bar — is the
portrait design carried over unchanged.

Direction (2026-09-04), iOS first: a phone turned to landscape reads like the
iPad. Rotating to landscape on the Read tab enters the distraction-free
presentation — the reading pane alone, no restore button because rotation is
the way out — and the text takes the iPad typography (the centred reporter
measure, 1.3 leading, first-line indents, no paragraph gaps: docs/IPAD.md);
rotating back restores the portrait layout and the reader's own full-screen
choice. Tablets are distinguished by construction (deviceIsTablet: the UIKit
idiom on iOS, the small-side-600dp rule on Android) and keep their rail. Books
and Search keep their landscape layout too — the bar on iPhone, the rail on
Android — because the mode applies to the Read tab only.

What the reader gives up in landscape, by design: every control — chapter
arrows, the picker, Go-to, search, narration, the "‹ Results" trail — is
reached by rotating back to portrait, the way out the mode is built around.
The label row keeps the reference and the selection menu still works, and it
carries the chapter arrows (fullScreenExitRow, shared
by both native panes): running out of chapter is the one thing that happens
while READING, and rotating out and back is a poor way to turn a page. The
reader's own full-screen keeps its restore button and only that.
On Android, tablet identity follows the live window, so a tablet pane split or
floated narrower than 600dp reads as a phone and takes the presentation, which
is the height problem the mode exists for.

Status (2026-09-04): ON BY DEFAULT for phones on iOS and Android
(phone_landscape.go), as two preferences a switch can turn off — the dev
Links tab carries both; a user-facing Settings row was considered and
declined. BOTH halves now ship on both platforms. The Android reporter page
(reporter_android.go) reaches the reader by three routes, because this dialect
has no stylesheet: the first-line indent is markup (android_chapter_html.go),
the paragraph gap is closed by importing in COMPACT rather than LEGACY mode
(BtBridge.setHtml — the gap is the importer's blank line between blocks, and
the markup keeps its <p> blocks so a note band still knows where a paragraph
begins), and the measure is pushed as a width the bridge centres against the
live view (androidReadingMeasureDp, BtBridge.applyReadingPadding, the shape of
the iOS textContainerInset). The leading is deliberately NOT changed: the
Apple dialect writes 2.0 and 1.3, but the UIKit importer honours neither, and
the drawn pitch measured 83px portrait against 82px landscape on the iPhone 16
Pro simulator — so following the CSS would have opened a 54% gap between panes
meant to match. The reporter flag
folds into the body fingerprint so a rotation re-imports under the new
grammar; the layout watcher carries the presentation as its own term and, on
iOS, captures the reading anchor before the rotation's frame lands (Android's
bridge re-places its own view by scroll fraction). Simulator-verified on the
iPhone 16 Pro: presentation, typography, the reading position across
rotation in both directions at the chapter top and mid-chapter, chosen
full-screen round-tripping with its restore button, a live selection
dropping cleanly, Books and Search keeping their layout in landscape.
Emulator-verified on Android 13 (API 33) and Android 15 (API 35): the Read
tab's landscape presentation with the position kept across both rotations, a
link's verse wash surviving them, and the Books tab keeping its rail.
`BIBLETEXT_DEV_PHONE_LANDSCAPE=on|typo|off` seeds a scripted simulator run.

## Android landscape: the status-bar strip is black in the light theme — DONE

On the API 33 emulator (720×1600) in landscape, the strip down the short edge
was black behind the light paper. It was not the status bar and not the
window: it was the WINDOW MANAGER'S CUTOUT LETTERBOX. In the default cutout
mode a window may sit under a display cutout only when that edge is at the
top (portrait); in landscape the system letterboxes the window away from the
cutout's whole short edge and paints the letterbox itself, in black. Dark
paper hid it; light paper showed it, and the landscape reading mode made it
the first thing seen.

The first attempt painted the activity window's background and system bars
in the paper colour from the style push, and changed nothing — this is a
NativeActivity window (takeSurface), so the view hierarchy never draws and a
window background or bar colour set on it is never rendered. The fix is
BtBridge.extendIntoTheCutout, from init: LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
on API 28 to 34, so the cutout arrives as an inset the canvas already honours
and the strip becomes paper (from Android 15 the platform forces "always" on
every window of an app targeting 35 or later — this app targets 36 — which is
why the API 35 emulator never showed it). The same pass found the reporter
column centred in the overlay rather than on the screen — the canvas keeps
the reading object out of the display cutout, so the overlay sits 42px from
one edge and 178px from the other and its centre is 67px off — and
applyReadingPadding now centres the column on the window, as the iPhone does. It centres against the overlay's FRAME
(frameX plus the canvas inset, the sum applyFrame positions the window by),
not the view's on-screen location: on a rotation the first layout runs before
the overlay window has moved, a location read then said x=0, and the column
landed ~130px off once the window moved — and only with the cutout on the
left, which is what made it look intermittent. applyFrame re-centres after
every move.

Emulator note: the rotation helper used to write `accelerometer_rotation 0` + `user_rotation`, which
switches auto-rotate OFF and leaves the emulator ignoring its own toolbar
rotate button afterwards. Rotate emulators with `adb emu rotate` (what the
button does) and never write those settings. Separately, the API 33 image
has mAllowAllRotations=false, so the 180° posture in the button's four-step
cycle is refused and that one click looks dead; the next click works.


## Android split-screen: the reading overlay doubled the task origin — DONE

applyFrame places the overlay Dialog at frame + windowContentOrigin, and
windowContentOrigin added the decor's ON-SCREEN location — but the window
manager resolves a dialog's x/y against the TASK bounds, so in a task that
does not start at the display origin (the bottom or right pane of
split-screen, a freeform window) the origin was counted twice and the overlay
was pinned below the tab bar with the reading area empty. windowContentOrigin
now subtracts the window metrics' bounds origin on API 30+, a no-op for a
full-screen task. Reproduced and verified on the API 35 emulator in a
freeform window (`settings put global enable_freeform_support 1` and
`force_resizable_activities 1`, then `am start … --windowingMode 5`): the
overlay moved from (297,1426) to (21,730) against a task at (276,696), and
sat over the reading rect. Below API 30 the task origin is not cheaply
readable, so a split pane there keeps the old placement.

## macOS: a launch restore that never died, and a note card placed before the text settled — DONE

Two defects in the NSTextView pane, both measured on the desktop app on
2026-09-05 and fixed in reading_macos.go.

The pane had no user-scroll hook, where iOS has scrollViewDidScroll. So the
Go side's `state.restore` was never consumed on macOS: after a launch (or a
history tap) every same-chapter rebuild — an appearance flip, a text-size
change, a note verb on the slow path — re-armed the launch anchor and moved
the reader back to it, however far they had read (measured: verse 37 back to
verse 1 on an appearance flip). Natively, the arm was disarmed only by the
first changed frame, which re-applied a superseded width and then let a later
resize re-apply the stale anchor. Now the clip view's bounds observer calls
`btMacUserScrolled` for any movement that is not one of the pane's own — the
wheel, the trackpad, the scroller, keyboard paging, a drag-select autoscroll,
an accessibility scrollbar write — and narration's follow-scroll calls it
too; every programmatic scroll (the restore, a highlight, an import's clamp,
a frame change) runs under a latch. It disarms natively at once and calls
`bibleTextReadingScrolled` once the motion settles, the iOS decision in
AppKit's vocabulary, and the arm survives frame changes as it does on iOS and
the Fyne pane. A `scrollWheel:` override was tried first and rejected: it saw
only the wheel and the trackpad, and on the document view it switches off
AppKit's responsive scrolling.

The note card sat a line above its passage with the band's air under its
tail on any launch into a saved position with an open note. `BT_NOTE_GEOM`
showed the card placed exactly where the arithmetic put it and the passage
31pt lower a second later: the paragraph's fragment had answered at the
previous line's y at placement time. The card is a subview placed FROM the
layout and nothing moved it when the text moved. `HBLayoutWatcher`, the
layout manager's delegate, re-places the card and the pills after every
completed layout pass — placement, not a refresh, so it converges — and the
card now sits 10pt above its passage. Pinned by
`reading_native_scroll_contract_test.go`; the pane itself cannot run on the
host, so the measurements are the evidence.

## Bible version states: transition diagram + comprehensive tests — DONE

Both halves are in docs/VERSION_STATES.md: the storage diagram (M1–M3) and,
since 2026-09-05, a second diagram for M4, the launch space and the arrivals
layer drawn as the enumerations drive them, with a table of every space's
own count as its test logs it (15, 10, 160 + 310 journeys, 8, 16, and 780
journeys over 2930 steps). The document's closing section had still said
M4–M7 were unenumerated; it now says what is, and what the enumerations do
not claim. The record of the work follows.


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

Both have since been enumerated. The launch space went in joined to reading
position and canon shape rather than alone (`version_launch_flow_test.go`,
M5 x M6 x M7), because the failure it was built for lives in that intersection
and in none of the three separately; the block below is its record. The
download space went in as M3, refresh and download
(`version_refresh_flow_test.go`): 160 cells over those axes plus the active
version, and 310 journeys to depth four.

### Reported defects — status, severity, and the order to take them

Each was raised while scouting the runtime and cache lanes. **PROBED** means the
scout executed it against a copy of the repo; **TRACED** means it is a code walk
with exact paths but no execution. None is claimed in docs/VERSION_STATES.md,
because that document only names what the enumeration has driven. Confirm each
with a cell as its space is enumerated.

| # | Defect | Evidence | Costs the reader | Fix |
|---|---|---|---|---|
| ~~D1~~ | ~~`purgeUnavailableLicensedCaches` deletes on an answer the app could not verify~~ | **CONFIRMED by the M2 enumeration and FIXED 2026-08-28** | — | done |
| ~~D2~~ | ~~Superseded epochs of a licensed version are never age-checked and never purged~~ | **FIXED 2026-08-28** | — | done |
| ~~D3~~ | ~~A non-default translation served from a superseded epoch is silently stale~~ | **FIXED 2026-08-28** | — | done |
| ~~D4~~ | ~~`seedOnly` is not cleared when the download lands while the reader is away~~ | **CONFIRMED by the M3 trajectory walk and FIXED 2026-08-28** | — | done |
| ~~D5~~ | ~~The picker's manual retry makes the waiting notice unreachable~~ | **CONFIRMED by a reachability assertion and FIXED 2026-08-28** | — | done |
| ~~D6~~ | ~~A successful fetch that cannot be persisted is discarded entirely~~ | **FIXED 2026-08-28** | — | done |
| ~~D7~~ | ~~Registry-resolved vs value-resolved cache paths can collide~~ | **GUARDED 2026-08-28** | — | done |
| ~~D8~~ | ~~The cache-only read's miss branches disagree about the mode~~ | **FIXED 2026-08-28** | — | done |

**All eight are closed** (2026-08-28), each confirmed against real code and
fixed with a test that fails without the fix.

**M5 x M6 x M7 — launch, reading position and canon shape — enumerated
2026-08-29**, together rather than separately, because the failure they were
built for lives in their intersection. Two more defects, both **durable**
(they rewrite the only record of something the reader cannot re-derive):

| | Defect | Status |
|---|---|---|
| ~~D9~~ | ~~A translation merely unselectable this launch has the reader's choice overwritten by the fallback, permanently~~ | **FIXED 2026-08-29** |
| ~~D10~~ | ~~The reader is shown a different translation than they chose, and nothing says so~~ | **FIXED 2026-08-29** |

The history-erasure invariant itself held in all sixteen cells, including the
73-book-trail-meets-66-book-fallback shape of the original incident.

**M4 — active selection — enumerated 2026-08-29.** One more defect:

| | Defect | Status |
|---|---|---|
| ~~D11~~ | ~~A stale version's notice is retired by the disk while its previous decode is still on screen~~ | **FIXED 2026-08-29** |

Not reachable today (nothing writes a non-default version's current epoch
while its previous decode is in memory), but one obvious feature away, and the
fix also closes a live liveness hole: a stale non-default translation
previously had no way to stop being stale within a session.

**The arrivals layer — enumerated 2026-08-29, as JOURNEYS** (780 to depth
four), because an arrival is a promise kept or broken over time and every way
it breaks is a sequence. One more defect:

| | Defect | Status |
|---|---|---|
| ~~D12~~ | ~~A link whose translation failed to load keeps its park; a later unrelated switch to that translation honours the dead link and moves the reader~~ | **FIXED 2026-08-29** |

**A second, model-free scouting pass over the same four arrival surfaces**
(each candidate refuted or confirmed by two adversarial verifiers on separate
lenses) found four more, every one on a coupling the map had not drawn:

| | Defect | Severity | Status |
|---|---|---|---|
| ~~D13~~ | ~~A link switching translations spends the reader's remembered fallback choice~~ | durable | **FIXED 2026-08-29** |
| ~~D16~~ | ~~A wider canon's trail is offered dead after a switch and deleted at the next launch~~ | durable | **FIXED 2026-08-29** |
| ~~D15~~ | ~~Search results survive a switch: old wording under a new name; a tap writes a dead reference~~ | session | **FIXED 2026-08-29** |
| ~~D14~~ | ~~A link displaced by another translation's load is dropped with nothing said~~ | session | **FIXED 2026-08-29** |

**All seven machines and the arrivals layer are enumerated, and every defect
found is closed.** Two lessons worth keeping. Third confirmation that
cross-products are blind to flow (the arrivals defect is invisible to every
cross-product in the suite; the journeys found it by four routes). And: a
harness can only look where its model points. The journeys found D12 and went
quiet; re-reading the same ground with no model in hand found four more,
including the original incident arriving through a version switch rather than
a launch.

## CI runs with the race detector, so 14 tests never run there

DONE for the cheap half. The Linux job now runs the suite a second time
without the race detector, so those 14 tests compile and run on every push;
they cover focus order, render output, wrap layout, the download-status row,
the audio card, the search mode row and the legacy pane. Retiring the `!race`
tag by teaching them to stand up under the detector is the real fix and is
still open.

Still true, and still untested anywhere: no CI job builds or tests the Android
app. The only Android-named step checks the target SDK. With a physical device
now available that gap is worth closing separately.

## Source fields: three defects and a decision list

Tracked in `docs/SCRIPTURE_WORKLIST.md`, not here. That file is the worklist
for work on the TEXT — what the decoders read from each source and what
reaches the reader; this backlog stays with the app around the text. The
analysis behind each item is in `docs/SOURCE_FIELDS_DECISIONS.md`, and the
inventory of every field is in `docs/SOURCE_FIELDS.md`.

docs/SOURCE_FIELDS_DECISIONS.md analyses every field the decoders skip or
half-keep, with costs and a recommendation for each. Three of its findings
are defects rather than decisions, in the order they should be fixed:

1. The paragraph rule tests only straight quotation marks, so a verse ending
   in curly-quoted speech never starts a new paragraph. About 2,100 eligible
   breaks per edition are suppressed. One line, no cache epoch.
2. Psalm 119's acrostic letters are in WEB and WEB Catholic verse text: ALEPH
   is stored as the psalm's title, and 21 verses end with a stray letter, so
   they reach search, sharing, copying, the website and speech. The NKJV
   decoder already drops exactly this. Needs epochs for web and webc.
3. The NKJV reads "call His name JESUS" in Matthew 1:21, Matthew 1:25, Luke
   1:31 and Luke 2:21, because the feed marks the name with the small-caps
   style the app uppercases for the divine name. Check a printed copy before
   changing anything; every other capitalised phrase in the canon is a
   genuine inscription.

The same document recommends a set of decode-time checks that cost no epoch
(the feed's own verse counts, the note reference field, a census of unknown
node and style shapes, and the words-of-Jesus cross-check), and sequences the
larger questions: source paragraph boundaries, section headings, and the
NKJV's italics for supplied words.

## NKJV Psalm superscriptions

DONE. decodeAPIBiblePassage reads the `d` (descriptive title) paragraph into
BibleData.Superscriptions instead of skipping it: text and notes go through
the same walk as a verse's, the title is attached at the next verse marker
(on the passages endpoint it is read while the decoder is still in the
previous chapter), and a title left over at a chunk's tail is attached only
when its own verseId says which chapter. nkjv cacheEpoch 1 → 2. The
renderers and the footnote section already handled titles for every version,
so nothing else moved there. The feed only carries `d` with
include-titles=true, so the query now asks for titles and apiBibleSkipPara
drops the section headings and acrostic letters that come with them. The
same epoch fixes a join defect the note capture had introduced: a verse
opening with a cross-reference took the paragraph boundary's poem break in
front of its first word (1,995 poetry verses began with a blank line).
Pinned by apibible_superscription_test.go and apibible_note_join_test.go;
TestLiveAPIBibleProbe checks Psalm 3 and the 3→4 passage boundary, and
TestLiveAPIBibleFullCanon (opt-in, ~200 requests) downloads the whole
canon and reproduces the 2026-08-23 text byte for byte with 116 titles
added. What every source carries and what is kept: docs/SOURCE_FIELDS.md.

## Show a note you just sent, the way opening one from the browser does

Sending a note stores it and draws nothing. `share.go` says so where it saves
the record — "never drawn in the text, and visible in the notes browser — that
visibility is deliberate" — and `notes_plan.go` enforces it: a `noteKindMine`
record joins the plan only while `noteFocus` names it, so the browser can show
one and the send path cannot.

The proposal is to let sending focus the note it just stored, exactly as
`openNoteFromBrowser` does: `state.focusNote(stored.ID)` then
`applyNoteForCurrentChapter(state)` after `saveMyNote` returns. That inherits
the transient behaviour already in place — `resetNoteFocus` runs on every
chapter arrival, so the note goes away on navigating away, with no new lifetime
rule to define and nothing persisted that was not persisted before.

This REVERSES a stated decision rather than fixing a defect, which is why it is
written down instead of done. What argues for it: sending is the one moment a
reader has no confirmation that their words were kept, and the browser is
several taps away.

The report that raised it was a misreading worth recording, because the app
invited it. A highlight was still standing after a send with no note beside it,
which read as a note that had lost its text; it was a search mark, from arriving
at the passage through Results. `hlOrigin` (mark.go) records provenance but does
not change the tint, so a note's mark and a search mark are indistinguishable to
a reader. Whether or not the change above is made, that ambiguity is its own
item: a reader cannot tell why a verse is lit.

## One pill, several noted paragraphs: what the count says and where it points

With more than one note on a chapter, the native reading pane draws ONE
anchored sticker — `planOpenLimit` is 1 — over the focused note's verse, and
puts the rest behind a counter that rotates focus and scrolls to the next one
(`advanceNoteFocus`). Two things about that are worth revisiting.

**The count says "passage" and means "chapter."** `placed` is
`len(plan.Notes)` — every placed received note in the CHAPTER — so a sticker
sitting over one paragraph can read "1 of 3 on this passage" while notes 2 and
3 are on paragraphs elsewhere. A reader looking at a pill over one paragraph
reads "this passage" as the paragraph under it, and the sentence then claims
three notes on a verse range that has one. The wording is only accurate in the
case it was written for, several notes sharing one range (the S10 scenario in
`dev_links_on.go`).

**Nothing marks the paragraphs the other notes are on.** The counter is the
only route to them, so a reader who does not tap it has no way to learn they
exist. Note that the platforms diverge here: the Fyne banner
(`notes_banner.go`, Windows and Linux) draws a CHIP PER NOTE, so those readers
do see the whole set while the native ones see one.

Three ways to take it, cheapest first, not exclusive:

1. ~~**Say the true scope.**~~ DONE. The count now reads "K of N in this
   chapter" unconditionally, rather than only where the placements differ.
   A conditional wording would have kept "on this passage" for notes that
   genuinely share a range, which is more precise when true; it was rejected
   because the string would then change under the reader with no way to tell
   which rule was in force, while the rotation it describes is chapter-wide in
   every case. The replacement is the same 15 characters, so the iOS and macOS
   WHO-line fitting is unaffected.

   One inaccuracy survives it, and is separate: N counts RECEIVED notes only,
   so a chapter holding five received and three of the reader's own still says
   "of 5". "Passage" was vague enough to hide that; "chapter" is checkable.
   What keeps it tolerable is that the byline names whose notes are counted,
   and an own note never displays a count at all — see the next item.
2. **Mark the other noted passages in the text**, so the pill over one
   paragraph no longer implies it is the only one. This is the discovery
   problem, and it is what the banner already gives the non-native platforms.
   ADDRESSED EVERYWHERE: the per-paragraph pills
   (`notesPillPerParagraph`) draw one pill per noted paragraph with that
   paragraph's own count on the styled pane, and iOS, macOS and Android draw
   the same groups through their band-spec pushes (`bibleTextSetNoteBands`
   and its twins). The gate defaults ON now — the shipped collapsed state
   says where the chapter's notes are on every surface.
3. **Lift the cap.** `planOpenLimit`'s own comment invites it: "TO LIFT THE
   CAP: raise this number (or drop the counter). Nothing else changes — not the
   store, not drawnNote, not the fingerprint." Largest change, and worth
   weighing against a page carrying three open bubbles at once.

(2) was watched on the API 35 emulator with four received notes on four
John 3 paragraphs (8d-And): four chips in their own bands, the single sticker
stood down, and a pressed pill opened its own paragraph's group. What remains
open here is (3) alone, and it is a choice rather than a defect.

## Candidate: a neutral graphite wash for the dark-mode highlight

The dark highlight is `#3A326F`, a violet. Held against seven alternatives on a
real screenshot, graphite `#2C343E` was the one worth keeping in mind: the
violet carries enough chroma to read like a system text selection, where a
neutral band reads more like a mark someone made. Light mode is not in
question — `#FFE08A` amber stays, and it is the one hue that says "highlighted"
while leaving red letters red.

Measured, so the trade is not a matter of taste alone:

| | vs the #191715 ground | red letters on it | body text on it |
|---|---|---|---|
| `#3A326F` violet (now) | 1.59:1 | 3.77:1 | 9.88:1 |
| `#2C343E` graphite | **1.42:1** | **4.22:1** | **11.05:1** |

So graphite is easier to read text ON and harder to spot AT A GLANCE. The
second half is the risk: 1.42:1 against a near-black page is quiet in daylight.

**What blocks a straight swap.** `TestMultiNoteWashKeepsScriptureLegible` pins a
hue relationship, not just a contrast floor: in dark the primary wash must be
violet (`B > R > G`) and the multi-note wash slate-blue (`B > G > R`), so
"several notes here" cannot be misread as "one strong note". Graphite is
`B > G > R` — it lands in the multi wash's own family and the pair collapses.
The test is right to refuse it.

**The way through, if this is ever taken up.** Give the violet the multi-note
job: primary graphite (neutral), multi `#3A326F` (chromatic). A neutral against
a chromatic separates more sharply than the current violet-against-slate pair,
and it keeps a colour worth keeping. `HighlightMulti` is unreachable today
(`tintMulti` is not wired), so that half costs nothing visually until it is.

Three approvals would move with it, and each is a deliberate gate rather than a
formality: `TestApprovedHighlightTokensStayPinned` and
`TestMultiNoteWashKeepsScriptureLegible` in `theme_contrast_test.go`, and
`TestWebReaderPaletteValues` in `cmd/websitegen` — the web reader's dark
highlight is meant to match the app's, and that parity is intended, so any
change here changes the web reader too.

## Per-paragraph note pills: notes that belong to no paragraph — DONE

Decided and built as the first option below: a chapter-top group carries
them. `groupNotesByParagraph` sends a whole-chapter note (anchored at verse 0)
to the chapter-top group, and `chapterNoteGroups` puts the book's unplaced
notes on that same group — creating it when nothing else needs it — with the
"· N not shown" suffix the single pill always carried. The collision with a
first-paragraph pill at the same band verse is resolved by matching bands by
KEY rather than by verse (8a). Pinned by
`TestTheTopGroupCarriesTheNotesThatBelongToNoParagraph` and
`TestTheTopPillDisclosesUnplacedNotes`, so the counts sum to the chapter's
total again. The original statement of the gap follows for the record.


The pills are per paragraph, and two kinds of note belong to no paragraph. Both
are counted by the chapter-scope single pill and both fall out of the reading
view once the pills take over:

- **unplaced notes** — filed on this book, with no home in the translation
  being read. The single pill discloses them (`Notes · 2 · 1 not shown`); no
  pill mentions them.
- **chapter-level notes** — anchored at `VerseLo 0`, on the whole chapter.
  They sit in `plan.Notes` with `Here = [{c 0 0}]`, so `noteAnchorVerse`
  returns 0 and `groupNotesByParagraph` skips them. With two ordinary notes
  and one chapter-level note, the pills account for 2 of 3.

One decision covers both: where does a paragraph-less note's pill go? Options:

- a chapter-top pill carrying them, reusing `stickerUnplacedOnlyWho` for the
  unplaced sentence — principled (chapter-scope things go to the top, the same
  reasoning that puts the collapsed single pill there), but it can collide with
  a first-paragraph pill at the same band verse, and the placement loop keeps
  only the first match per band
- a suffix on the first or last pill — misattributes a chapter-scope fact to
  one paragraph
- suppressing the pills whenever either kind is present — preserves today's
  disclosure exactly but silently disables the feature

Until it is decided the pills' counts do not sum to the chapter's note total,
which is the honesty property the single pill has always had.
`TestTheSinglePillStillDisclosesUnplacedNotes` pins the shipped gate-off
guarantee so the ungated path cannot regress meanwhile.

## Opening your own note hides every trace of everyone else's — DONE

DONE by side effect: per-paragraph note pills default on, which is what made
the other notes visible again. The description below predates that.

Not introduced by the pills — the shipped single-sticker path does it too, on
all five platforms.

The pills and the sticker are both the collapsed state, so opening any note
stands the pills down. For a RECEIVED note that costs nothing: the who line
becomes "Note from Friend · K of N in this chapter", which still says the
others exist and still offers the count control that rotates to them. For the
reader's OWN note it costs everything: an own note is deliberately not a member
of N and has no next-tap, so its who line reads "Note from you" alone. Measured
on a chapter with three received notes in two paragraphs plus one own note:

    a received note open  -> pills 0, who "Note from Friend · 3 of 3 in this chapter"
    your own note open    -> pills 0, who "Note from you"

So the one case where the reader loses all evidence of their friends' notes is
the case where they opened something of their own — and nothing on the page
tells them to close it to get that evidence back.

A principled fix exists: stand the pills down only for an open RECEIVED note,
whose who line then carries the count. An own note is not in the pills' set at
all (they count received notes), so leaving them up alongside it double-counts
nothing — which is the reason the one-collapsed-state-at-a-time rule exists.
That still leaves the three native surfaces, which have no pills to leave up;
for them the answer would have to be a count in the own note's who line, which
contradicts "displaying an own note must not change N" unless it is written as
a separate clause rather than folded into N.

## Per-paragraph pills on every surface — DONE

The port and the unification were one job, and both are complete: see the
status block at the top of
[docs/NOTE_CHROME_UNIFICATION.md](NOTE_CHROME_UNIFICATION.md), which records
all nine steps landed. iOS, macOS and Android draw the paragraph groups
through their band-spec pushes (`bibleTextSetNoteBands` and its twins, 8d),
`notesPillPerParagraph` defaults on (8f), and the web reader takes its chrome
from the same functions (step 9). X16 in docs/NOTES_STATE.md — an open own
note leaving the received set represented nowhere on the native surfaces — is
closed by it. The dev toggle's off value remains the one-line reversion.

## Deferred-UI timers under the test driver (audited 2026-09-02, no live races)

`verse_of_day.go`'s re-measure timer is the precedent: under Fyne's TEST
driver `fyne.Do` runs its closure on the timer's own goroutine — there is no
UI thread to marshal to — so a pending `time.AfterFunc` races the test
goroutine (including a `t.Cleanup` `Hide`), and even a `Visible()` read is a
data race. The shipped fix stands the timer down under `testing.Testing()`
because the tests drive that re-measure synchronously.

Every other `AfterFunc`+`fyne.Do` site was audited against the same hazard.
None races today — a full `-race` pass of the suite is clean — but each is
safe for a DIFFERENT reason, and the first test that changes that reason
must bring the fix with it:

- `goto.go` (60ms inset scroll; 200ms self-rearming `watchDismiss`) and
  `share_note_ui.go` (50ms slot push; 150ms self-rearming watch): the arms
  live on the `IsMobile()` / native-entry branches, which no host test
  compiles into. If a mobile-tagged test target ever opens these, gate the
  arms with `testing.Testing()` AND add a direct synchronous call for the
  watch's close-out work — both watches are functional (tap-outside close,
  overlay restore), not cosmetic, so a bare gate would orphan real behavior.
- `audio_menu.go` (150ms self-rearming watch): no test constructs the
  source menu (the card tests stub `onSrc` deliberately). Same rule as the
  two above: gate plus a synchronous driver, never a bare gate.
- `search.go:~25` (60ms scroll restore) and `notes_browse.go:~783` (16ms
  scroll restore): armed only when a remembered scroll offset is positive,
  which no test produces. The natural regression test for either restore
  (build, scroll, rebuild, assert offset) arms the timer and reproduces the
  verse-of-day race verbatim — write the `testing.Testing()` stand-down in
  the same change and drive the restore synchronously in the test.
- `ai_panel.go:~305` (40ms re-fit): armed only by a delivered answer, which
  every current test's stub avoids. A test that lets `setResult` run needs
  the stand-down (the closure measures fonts with no `Visible()` guard).
- `reading.go:~209` (`flashIcon`): the one arming test passes `time.Hour`
  and drives the restore synchronously — a deliberate crutch. A test that
  taps a real copy button (1200ms flash) needs the arm gated.

The audit's rule, kept: a gate lands only with a demonstrated race (a
`-race -count=30` loop that fails before and is clean after) — no
prophylactic gates over dead code.


## `fyne package` bumps the desktop Build ledger after every package

DONE. Both packaging paths now save the ledger before packaging and restore it
afterwards, which is the pattern `build-android.sh` already used. The store
script restores it in the same EXIT trap that restores go.mod, so a build can
no longer leave an uncommitted bump in the tree. The release workflow restores
it between the two macOS architectures and again at the end, so both downloads
carry the committed Build rather than the second one carrying a number nobody
chose, and it then unzips both and checks their CFBundleVersion against the
ledger rather than trusting the restore. `test-release-key-flow.sh` asserts all
of it and fails if a restore is removed.

## The verse wash was measured in two conventions — FIXED 2026-09-08

`Layout.getHorizontal` never applies justification on any release from 26 to 36,
while the line-level extents (`getLineLeft` / `getLineRight`, through
`getLineExtent`) do. WashSpan took one edge from each: line extents where the
verse reached a line edge, `getPrimaryHorizontal` where it stopped mid-line.

So on a justified line — Android 15 and newer, which is where the pane justifies
— a verse ending mid-line had its right edge measured as if the line were ragged,
which is left of where the glyphs are. The full stop closing the verse fell
OUTSIDE the wash. Verified against the ragged Android 13 build, where the two
conventions agree and the same verse washed correctly; that control is what ruled
out the obvious alternative, that the verse range simply excluded the period.

Fixed by reconstructing the platform's own INTER_WORD arithmetic from public API:
`TextLine.justify` spreads (target − unjustified) over the line's stretchable
spaces, so the drawn x of an offset is its blind x plus one share for every
stretchable space before it (`drawnHorizontal`, android/BtBridge.java). It is
self-disabling — on a ragged line the extra comes out at or below zero and the
blind value is returned unchanged, which a pixel diff of the Android 13 build
confirms at zero differing pixels.

The selection popup's anchor had the same blind measurement and is routed through
the same helper.

## Android 13/14: the reading text cannot be justified while it stays selectable

The native reading pane is a selectable `TextView`, which Android lays out
with a `DynamicLayout`. Before Android 15 that layout never hands the
justification mode to the `Layout` that draws: the line breaker still breaks
in justified mode — which lets a line exceed the width by the amount its
spaces could shrink — but nothing shrinks them, so on API 33 every such line
spilled past the right edge and the rest read ragged (stock emulator, measured:
layout width == view width, line width > both; the android13/14-release
`DynamicLayout` constructors omit the hand-off that android15-release makes).
The app now justifies only on API 35+, and older releases read ragged but
whole. Non-selectable text (`StaticLayout`) justifies on every release, but
selection is the native feature the pane exists for.

Ways back, if ragged text on Android 13/14 ever matters enough: a custom
`TextView` that draws its own justified lines from a `StaticLayout` while
keeping a selectable `DynamicLayout` for hit-testing (two layouts, one text);
or reflection on the protected `Layout.setJustificationMode`, which the
hidden-API policy may refuse. Neither is worth it for a rendering that
Android 15 already gets right.

**REVISITED 2026-09-08, and the gate STAYS.** Two things were established and
then weighed the other way round from the sentence above.

The fleet share is worse than that sentence assumes: Android 15 and 16 together
are roughly a quarter to a third of active devices, so justification is the
MINORITY rendering here and will be for years. Levelling everything down to
ragged was therefore considered — it would have deleted a split in a pane that
already forks once on API 29 for the reading face, and ragged is defensible at a
phone measure (the justified page's word spaces vary nearly threefold from line
to line, the rivers a narrow column always gives).

It was rejected, and the principle is worth keeping: **give each platform the
best rendering it can manage, rather than holding everyone to what the weakest
can do.** A reader on a capable release should not get a worse page so that it
matches an older one. Ragged stays where the platform cannot do better, which is
what the gate already expresses.

So: the gate is correct, neither workaround above should be built, and the
inconsistency between fleet bands is accepted rather than overlooked.

## Tag 1.2.6 so `go install` and `go run …@latest` resolve the module — DONE

Cut as an ANNOTATED tag (the kind docs/VERSIONING.md asked the next cut to
settle on). Deliberately a tag and NOTHING else: no release assets, no store
submission, and the two FyneApp.toml ledgers deliberately left at 1.2.5.

The ledgers stay because in this repository the ledger version IS the prepared
store submission — check-release-identity.py holds it against
appstore/review-notes.txt, and the review-notes tests hold that against the
writer's version pin and a What's New file named for it. Moving the ledgers to
1.2.6 would have prepared a submission nobody is sending.

The cost, stated so a bug report stays readable: a binary built from the
v1.2.6 tag reports 1.2.5 in Settings, because no binary channel carries 1.2.6.
A source build is already distinguishable — it carries no bundled NKJV key
(the entry below) — and the next store release takes 1.2.7 with both ledgers
and both review-notes files moving together.

Every tag up to v1.2.5 carries the old bare `module bibletext` line, which Go's
module resolution rejects: `go install github.com/cubancorona/bibletext/cmd/desktop@latest`
and `go run …@latest` resolve `@latest` to the newest tag and stop at "module
declares its path as: bibletext but was required as:
github.com/cubancorona/bibletext". main declares the repository path
(`module_path_test.go` holds it) and `…@main` already builds; those routes
reach `@latest` when v1.2.6 is tagged.

The directory entry (apps.fyne.io/apps/uk.co.bibletext/) prints
`fyne install github.com/cubancorona/bibletext/cmd/desktop@latest`, which takes
a different route — `git ls-remote` for the newest v-tag, a depth-1 clone of
that tag, then `go build` inside the clone — so the module line never enters
into it and that command works today on v1.2.5. A source build by either
route carries no release ldflags, so it has no bundled NKJV key: the reader
adds their own API.Bible key in Settings for that translation.

## A shared verse carries the app's own small capitals, and then cannot find itself

Found while checking whether a sentence in docs/ADDITIONS_AND_DROPS.md was true.
It was not, and the reason is a live defect.

`outboundText` (outbound_text.go) is the cleaner that strips the app's own
typography from text on its way out — superscript verse numbers back to digits,
the no-break join back to a space, the paragraph indent dropped, and the drawn
small capitals mapped back. It has exactly two call sites: `ai.go:266` and the
styled pane's Copy at `reading_styled_select.go:390`. **Share is not one of
them.**

The share path takes `plainSelection` (reading.go:1409) instead, which is
`cleanCopy` plus superscript-to-digit and nothing else, and then
`stripVerseMarkers(collapseSpaces(raw))` at share.go:435. Neither maps a small
capital back. Two consequences, on any verse the edition sets in small capitals
— which is the whole point of the NKJV divine-name work:

1. The shared text carries the substituted characters. A reader pastes `Lᴏʀᴅ`
   where every other edition and reader gives `LORD`.
2. `normalizeShareSelection` then looks for that text inside `chapterProse`
   (share.go:260), which is built from `Verse.Text` — the publisher's own
   letters. `Lᴏʀᴅ` cannot match `Lord`, so the selection is not located and the
   share drops to the legacy probe-based citation path.

The fix is probably to route the share path through `outboundText` as the AI
path already does, but the locate is the part to think about: `chapterProse` is
publisher text, so whatever the share carries has to be in those letters by the
time it is matched. Worth a test that shares a divine-name verse and asserts
both the outgoing text and that the selection was located rather than fell back.

## Three comments that describe code that changed under them

Each verified against the source; all are comments, none change behaviour.

- `outbound_text.go:61-62` says the small-capital branch maps "Back to the
  publisher's own letter". It does not. `smallCapitalToLetter`
  (small_caps_draw.go:47-62) maps to the CAPITAL, deliberately, and gives the
  reason two lines down: a small capital does not record its original case, and
  uppercase is the conventional plain-text realisation that keeps the
  Tetragrammaton distinct from Adonai. The call-site comment contradicts the
  map's own reasoning.
- `dev_mimic_test.go:72` says a Mac, a Windows machine and a Linux machine "all
  draw the same Spectral" while the assertion two lines below requires
  `Junicode`. The face changed; the sentence did not. `dev_mimic_on.go`'s file
  header still lists "and font candidates" among the seams, though that seam is
  gone from the tree.
- `app.go:508-511` carries a duplicated fragment from the same commit that
  retired the font seam: "Must run before CreateMainUI (installSheetCloseConsume
  reads a seam) and / (installSheetCloseConsume reads a seam)." The sentence
  never completes. `dev_mimic_on.go:56-57` has the same kind of damage.

## docs/ANDROID.md and reading_android.go disagree about the note sticker

docs/ANDROID.md:183-185 says the Android shared-note sticker "is gated to
`IsFullScreen` in Go (`pushNoteToOverlay`, reading_android.go) precisely so the
reader never sees the note twice." The function it cites opens by saying the
opposite: "BOTH reading modes now, not full-screen alone (settled by comparing
the two platforms side by side) ... Same sticker, both modes, and the Fyne
banner stands down" (reading_android.go:615).

The code is the newer of the two. Left alone rather than guessed at because the
sentence also asserts a reason — that the gate exists to prevent a double
draw — and whether that concern is now handled by the banner standing down, or
was simply overtaken, is worth confirming before rewriting it.

## Target audience is 13+, and it was meant to include 9-12

The Play target-audience declaration was saved as **13-15, 16-17, 18 and over**.
That is not the intended answer. The intent was to include **9-12** so the app
is offered to children as well, and the content rating supports it: ESRB
Everyone 10+ and PEGI 3 put the floor at 9, not 13.

It was set to 13+ deliberately, to get the app-content checklist finished.
Including any band under 13 turns the app into a Families-policy app and adds a
step the console will not let anyone else complete: a legal certification that
the app, including all APIs, SDKs and ads, complies with COPPA and GDPR. That is
the owner's to sign, and only the owner's.

Nothing is lost in the meantime. The rating is unchanged, so a child can still
find and install the app; target audience governs Families-programme placement
and the policy obligations that come with it, not who may download it.

To change it: App content -> Target audience, tick 9-12, and sign the
certification at step 2 of 5. Note the selection is NOT saved until the wizard's
final Save, and leaving the page discards it, so it has to be finished in one
sitting.

Before signing, the two things a Families reviewer would look at are worth
having closed: the AI answer markdown vector (fixed) and the storage permissions
(fixed), plus the recovery breadcrumb for a key that did not survive a device
move, which is still outstanding. The app has no ads at all, which is the single
biggest Families-policy failure mode, and the audit in docs/PLAY_LISTING.md
found no identifiers, analytics or crash reporting anywhere.
