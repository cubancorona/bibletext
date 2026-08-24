# BibleText on Android — build, sign, distribute

Companion to the iOS notes. Android has a **native reading overlay** (a
selectable `TextView` in a `Dialog` floated over the Fyne GL surface —
`android/BtBridge.java` + `reading_android.go`), the twin of the iOS
`UITextView`, so readers get real long-press selection with drag handles, the
floating toolbar, and BibleText's study menu — the same Go dispatch as iOS.
**"Study with AI" sits INLINE on the floating toolbar** (a plain item that
opens a popup with Explain / Analyze context / Analyze translation, anchored
at the selection — Android's floating toolbar can't show a SubMenu inline and
flattens SubMenus in its overflow, so a real submenu would bury the AI verbs
while hoisting Cross-references above them). Cross-references also gets an
inline slot when there's room; Share with citation / Share as image live in
the ⋮ overflow. Android now has **full audio parity** (`android/BtAudio.java`
+ `audio_android.go`): a `MediaPlayer` streams recorded chapters (±15s skip) and
`TextToSpeech` reads aloud, with read-along verse highlight + the floating
"Follow narration" pill painted on the reading overlay — the twin of the iOS
AVFoundation engine, driven by the SAME cross-platform `audio_controller.go` —
**including background playback and lock-screen transport** (a `mediaPlayback`
foreground service + framework `MediaSession` + MediaStyle notification; see
the Audio section). Everything else (reading, search, Find/Ask AI, navigation,
versions, history, text size) is data-driven off the same code.

**Build with [`scripts/build-android.sh`](../scripts/build-android.sh), NOT a
bare `fyne package`** — the overlay's Java half ships as `classes2.dex`, which
that script compiles and injects. A plain `fyne package` build still runs, but
silently falls back to the old Fyne-widget reading pane (no native selection).

**Verified 2026-07-04** on the API-35 arm64 emulator: native selection (handles
+ floating toolbar), each study action reaching the Go panel (Explain →
Explanation panel), overlay hide on tab switch + suppress behind the AI modal,
scroll-position persistence, reading, Books, live Search, and the Bible cache.

## Native overlay (BtBridge)

The overlay MUST be a **Dialog** at **TYPE_APPLICATION**, not a
`WindowManager.addView` root or a sub-window, for two hard Android reasons:
1. NativeActivity's `Window.takeSurface()` means the activity's Java view
   hierarchy never draws — a separate window is required to show anything over
   the GL. (A `WindowManager.addView` window draws, but…)
2. Text selection's floating toolbar is created by
   `DecorView.startActionModeForChild`; a bare `WindowManager` root has no
   DecorView, so `ViewRootImpl` returns null and no toolbar appears. A Dialog
   has a real decor. And the Editor **force-disables selection handles** for
   sub-windows (types 1000–1999, `windowSupportsHandles`), so TYPE_APPLICATION
   (2) it is.

Other essentials: a **NO_OP `TextClassifier`** keeps selection synchronous (the
async smart-selection round-trip fails to raise the toolbar in the overlay
window); `FLAG_NOT_TOUCH_MODAL` + non-dim lets Fyne header/tab taps fall
through; BACK is forwarded to the activity. Go drives the bridge over JNI via
`fyne driver.RunNative`, resolving `org.bibletext.BtBridge` through the
activity's classloader (a `FindClass` on the RunNative background thread can't
see app dex classes). Selection actions call back via
`reading_jni_android.c` → `reading_android_export.go` into the shared
`dispatchAIAction` / `dispatchSelectionAction` — the SAME action strings and
downstream code as iOS. "Share as image" publishes the rendered card through
**MediaStore** (no FileProvider/androidx needed — that would require a custom
manifest we don't ship), which also drops the card into the user's Pictures.

**Menu-parity limits vs iOS (platform, not fixable):** iOS nests the actions
under labeled *Study with AI* / *Share* submenus. Android's floating-selection
toolbar **flattens submenus** — `menu.addSubMenu` collapses into one flat,
scrollable overflow list — so grouping doesn't render; and the toolbar only
shows ~3 items inline (we promote Cross-references), pushing the rest behind ⋮.
The dark pill styling and system items (Copy / Select all / OEM extras) are the
OS's. This is native Android behaviour and matching iOS exactly isn't possible.
(Note for testing: `adb input tap` hits the floating toolbar's popup window
unreliably — verify menu taps with a real finger, not synthetic input.)

## Audio (BtAudio + read-along)

The Android audio engine (`android/BtAudio.java`, driven from `audio_android.go`
over JNI) is the twin of the iOS `BTAudioController` and implements the SAME
interface the cross-platform `audio_controller.go` calls
(`nativeAudioStartURL/StartTTS/Toggle/Stop/Skip/SetArtwork`), so recorded ↔
read-aloud, ±15s skip, continuous playback and the source menu are all shared Go:

- **Recorded MP3** → `android.media.MediaPlayer`, streamed from the same
  self-hosted audio releases as iOS. **No MIME override needed** (unlike
  AVPlayer): MediaPlayer's extractor sniffs MP3 by content, so the CDN's
  extension-less `application/octet-stream` and its 302 redirect are handled
  natively. `seekTo(long, SEEK_CLOSEST)` on API 26+ for the ±15s. Buffering →
  `OnInfoListener` `BUFFERING_START/END`; a stream error posts FAILED, and Go
  restarts the chapter as read-aloud (the iOS fallback).
- **Read-aloud** → `android.speech.tts.TextToSpeech`. TextToSpeech can't pause,
  so toggle re-speaks from the current verse's offset on resume. Long chapters
  exceed `getMaxSpeechInputLength()` (~4000), so the text is split at spaces into
  `QUEUE_ADD` utterances; each carries a global base offset so
  `onRangeStart` (API 26+) maps to a whole-chapter offset (word-level read-along
  needs API 26+; below that, no per-word highlight). A generation counter drops
  late callbacks from a stopped utterance (the Android twin of the iOS mode guard).
- **Audio focus** (`AudioFocusRequest`, API 26+) gives phone-call parity: a
  transient loss pauses, GAIN resumes unless the reader paused first.

**Read-along highlight + "Follow narration" pill** live on the reading overlay
(`BtBridge.java`), driven via `reading_android.go`'s `readAlong*` wrappers. The
verse index is built in `setHtml` by scanning the verse-number `SuperscriptSpan`s
(`Html.fromHtml` makes one per `<sup>`) and parsing the digits — so a verse maps
to its char range with no HTML changes. Highlighting applies a
`BackgroundColorSpan` (kept as a reference so it's removed cleanly, never the
search highlight's own span); follow-scroll uses the `Layout` line geometry into
a comfortable band, exactly like iOS. The pill is a `Button` child of the overlay
window (only a view in that window can float over the GL-drawn verses), and a
hand scroll during narration suspends follow (fires `nativeReadAlongUserScrolled`,
gated once per suspension by `raFollowing` — the iOS `gReadAlongUserLatch` twin).

**Background + lock screen (Phase 2 — DONE, emulator-verified 2026-07-04).**
Narration keeps playing with the screen off / app backgrounded, with the full
Now Playing card (title · "World English Bible · David Williams", the rendered
chapter artwork, a scrubber, play/pause + ±15) on the lock screen and in the
quick-settings media carousel, and phone-parity continuous playback: chapters
roll over while the device sleeps (verified 117→118→119 asleep; `fyne.Do` runs
while backgrounded — the mobile driver's func queue is drained by the always-on
run loop — so the SAME Go `advanceAndContinue` drives it). The pieces:

- **`android/BtAudioService.java`** — a `mediaPlayback` foreground service; its
  MediaStyle notification anchors the process (no cached-app freeze) and hosts
  pre-13 transport actions. Swiping the app from recents (`onTaskRemoved`)
  stops playback outright, matching iOS.
- **`BtAudio`** owns a framework `android.media.session.MediaSession` (no
  androidx): PlaybackState (Android 13+ renders lock-screen buttons from its
  actions — REWIND/FAST_FORWARD are the ±15s, SEEK_TO the scrubber; TTS omits
  them), MediaMetadata (duration only for recorded → no TTS scrubber, like
  iOS), a partial wake lock while audible, and the `POST_NOTIFICATIONS` runtime
  prompt at first play (33+; safe — `GoNativeActivity` overrides no
  `onRequestPermissionsResult`).
- **`cmd/mobile/AndroidManifest.xml`** — a CUSTOM manifest (fyne uses it
  verbatim), which required flipping fyne onto its **aapt2** resource path:
  fyne's legacy binres AXML encoder can't encode `foregroundServiceType` (an
  API-29 attribute vs its baked-in API-21 table — the same limit that forced
  MediaStore over FileProvider). The flip is triggered by
  **`cmd/mobile/Icon-foreground.png`/`Icon-background.png`** (the adaptive-icon
  layers, generated from Icon.png). Rules for editing the manifest: do NOT add
  versionCode/versionName/`<uses-sdk>` (fyne passes them as aapt2 link flags —
  hardcoding freezes them); keep `org.golang.app.GoNativeActivity` + the
  `android.app.lib_name` meta-data (fyne validates both); the icon must be
  `@mipmap/ic_launcher` (what fyne compiles the layers into).
- The build script asserts the aapt2 path was taken (adaptive-icon resources in
  the output) in BOTH the debug and release branches.

## Shared-note sticker (full-screen reading)

Full-screen reading draws the shared note NATIVELY on the overlay, matching the
iOS interaction model: the bubble with the WHO line ("Note from Friend · 2 of 3 on this
passage ›"), Hide ("–") / Delete ("✕") on the bubble, the count region as the
next-tap selector when the passage holds more than one note, and the collapsed
pill (tap = Restore) when the note is minimized or a foreign mark (search/link
highlight) suppresses it. Normal (compact) reading keeps the Fyne banner
(`notes_banner.go`) — the sticker is gated to `IsFullScreen` in Go
(`pushNoteToOverlay`, reading_android.go) precisely so the reader never sees
the note twice.

The pushed tuple is composed by `androidStickerPush` (notes_plan.go) — a thin
alias of `appleStickerPush`, so WHO lines, counts, pill labels and derived
suppression are byte-identical to iOS (pinned by
`notes_verb_screen_test.go`). It rides EVERY chapter push, before the
fingerprint skip gate; `BtBridge.setNote` compares the tuple and re-derives
the views only on change (the iOS `bibleTextSetNote` pattern), so
presentation-only flips render without a Spanned re-push. Verbs call back over
`nativeNoteNextTapped/Hidden/Deleted/Restored` → `reading_jni_android.c` →
`reading_android_export.go` into the SAME Go verbs iOS uses, each ending on
the projection + repaint.

**Geometry, honestly:** iOS reserves the band with `paragraphSpacingBefore` on
the anchor paragraph; a TextView has no per-paragraph spacing, so the band is
a one-character `LineHeightSpan` (`NoteBandSpan`, implements `UpdateLayout` so
DynamicLayout reflows) on the anchor verse's first character, raising that
line's top by the measured sticker height + gap. The sticker view lives INSIDE
the scroll content (a `FrameLayout` sibling of the TextView), positioned into
the reserved gap from the Layout's line geometry — so it scrolls with its
verse natively, no per-scroll tracking. Recorded simplifications vs iOS: no
speech tail under the bubble; the WHOLE WHO line (accent + "›") is the
next-tap control rather than just the counts span (one TextView cannot split
the tap); an unplaced-only pill parks at the top of the text with no band.

## Known quirks

- **Debug builds link `--target-sdk-version 29`** (fyne hardcodes 29 for
  `package`, 35 for `release`). Practical effects on an Android 13+ device:
  the notification-permission flow differs (auto-prompt vs our runtime request)
  and API-34 FGS-type enforcement is relaxed. Always re-verify
  background-audio behavior on the RELEASE universal APK, not just the debug
  APK (both were verified 2026-07-04).
- **Debug APKs are not JDWP-debuggable**: the custom AndroidManifest.xml omits
  `android:debuggable` (fyne's template set it per build type; ours is used
  verbatim for both). Diagnosis is logcat-based anyway; if a debugger is ever
  needed, temporarily add `android:debuggable="true"` to the `<application>`.

- **Landscape / rotation (native overlay geometry).** The reading overlay is a
  separate window positioned to the Fyne reading pane's rect
  (`setFrameFromObject` → `BtBridge.setFrame`, offset by the decor view's
  on-screen origin). On an orientation change the Fyne UI reflows but the overlay
  doesn't always re-fit cleanly to the new geometry until the next layout-
  triggering interaction (scroll / chapter nav / version switch re-pushes the
  frame). Audio, read-along and text all survive rotation; the visual is just a
  transiently-stale overlay rect in landscape / right after rotating back. This is
  a general native-overlay limitation (not audio-specific); the app is portrait-
  first. If it becomes a priority, add an explicit frame re-push on the Android
  configuration-change hook.

- **Bible cache**: `os.UserCacheDir()` has no writable target on Android, so
  `defaultCachePath()` uses Fyne's per-app storage there
  (`cache_path_android.go` → `/data/data/uk.co.bibletext/files/fyne/`). Without
  it the app re-downloaded the Bible every launch and couldn't work offline.
- **Hardware-key doubling (upstream Fyne)**: characters typed through the
  HARDWARE key path arrive doubled in Fyne entries ("sheep" → "sshheeeepp") —
  Fyne processes both the raw key event and the IME echo. On-glass touch typing
  (GBoard taps) is unaffected — verified. Affects `adb shell input text/keyevent`
  (test automation) and likely Bluetooth keyboards; not worth blocking testers
  over, but know it when driving the emulator: tap the on-screen keys by
  coordinate instead of using `input text`.

## Toolchain (installed under $HOME, no Homebrew/sudo)

- **JDK 21** (Temurin): `~/Library/Java/jdk-21.0.11+10/Contents/Home`
- **Android SDK**: `~/Library/Android/sdk` — cmdline-tools, platform-tools,
  `platforms;android-35` + `android-36`, `build-tools;35.0.0` + `36.1.0`,
  **`ndk;27.2.12479018`** (r27 LTS — do NOT use r28+, its 16 KB page alignment
  fights gomobile), `emulator`, `system-images;android-35;google_apis;arm64-v8a`
- **bundletool 1.18.3**: `~/bin/bundletool` (a JAR + wrapper; **required on PATH**
  for `fyne release` to emit the `.aab`)
- Env is in `~/Library/Android/env.sh` and appended to `~/.zshrc`:
  `JAVA_HOME`, `ANDROID_HOME`, `ANDROID_SDK_ROOT`, `ANDROID_NDK_HOME`, PATH.

`source ~/Library/Android/env.sh` (or open a new shell) before any Android command.

## Build

Use **`scripts/build-android.sh`** — it compiles `android/BtBridge.java` to
`classes2.dex`, runs the Fyne CLI (`fyne-io/tools` v1.7.2 — current; the CLI
versions separately from the v2.7 toolkit), **builds against the patched
Fyne** (the script runs `setup-fyne-patch.sh` and injects the temporary
`replace`, exactly like the iOS scripts — the caret-blink fix is a real
battery win on Android; the iOS drawloop hunk is inert here), injects the
dex, and re-signs.

It also **replaces `classes.dex`**. The Fyne CLI packages a *prebuilt*
`classes.dex` (gendex output baked into the tool as a base64 blob), so patching
`third_party/fyne`'s `GoNativeActivity.java` would otherwise never reach the
APK — which is why warm shared links were silently dropped before
`patches/fyne-2.7.4-android-newintent.patch` existed. The script recompiles that
patched activity with the same javac/d8 recipe gendex uses, alongside
`android/goapp/FyneNotificationReceiver.java` (a hand-reconstructed twin of the
receiver in the tool's dex, kept so the manifest's `<receiver>` still resolves).

A side benefit: the tool's prebuilt dex comes from whatever Fyne the *CLI* was
built against, which is NEWER than the v2.7.4 we link — it carries an
accessibility bridge (`onInitializeAccessibilityNodeInfo`, `ROLE_*`) that v2.7.4's
Go half never calls. Compiling `third_party/fyne`'s own Java keeps the Java and
Go halves a *matched pair*. If the toolkit is ever upgraded to a version whose
Go side does call those methods, `third_party/fyne` regenerates at the same
version, so they stay matched automatically.

```bash
# Debug/test APK (signed with the upload key, target 29) → cmd/mobile/BibleText.apk
scripts/build-android.sh

# Signed release AAB (Play, target 35) + universal APK (sideload/Firebase)
# → ~/Library/Android/bibletext-dist/{BibleText.aab, BibleText-universal.apk}
BIBLETEXT_ANDROID_BUILD=<versionCode> scripts/build-android.sh --release
```

Release builds require the dedicated API.Bible environment or macOS Keychain
source and fail closed when it is unavailable. They never read `.env.local`;
see [API_KEY_HANDLING.md](API_KEY_HANDLING.md).

The release path: `fyne release -os android` emits a signed `.aab` via
bundletool, then the script swaps in `base/dex/classes.dex`, adds
`base/dex/classes2.dex` and re-signs
(jarsigner for the AAB; apksigner v1+v2+v3 for the debug APK, after zipalign).
`versionCode` MUST strictly increase per Play upload. There is no `-targetSdk`
flag (Fyne hardcodes 35); pass `-androidapi <n>` inside the script to raise
`minSdkVersion`.

**Gotcha:** the debug APK is signed with the *upload key*, not Fyne's built-in
debug key, so `adb install` fails against a previously-installed differently-
signed build — `adb uninstall uk.co.bibletext` first.

`Html.fromHtml` can't do CSS classes, so Android has its own chapter emitter
(`buildChapterHTMLAndroid`): verse numbers are `<sup><small><font><b>`, red
letter is `<font color>`, the search highlight is an inline `style=` span. Body
typography (serif, size, line spacing, justify) is set on the TextView, not in
HTML.

## Signing key — READ THIS

- Keystore: `~/Library/Android/bibletext-signing/bibletext-upload.keystore`
  (alias `bibletext`), password in `keystore-password.txt` (perms 600), both
  **outside the git repo**. NEVER commit the keystore or password.
- **Back it up somewhere safe** (password manager / offline). It is the **upload
  key**. With Google **Play App Signing** (the default), Google holds the real
  app-signing key and can reset a lost upload key via support — so losing it is
  recoverable, but don't rely on that. SHA-256 of the current cert is recorded in
  Verify the signing certificate fingerprint against the approved release record before packaging.

## Emulator (local testing)

```bash
avdmanager create avd -n bibletext_test \
  -k "system-images;android-35;google_apis;arm64-v8a" -d pixel_7
emulator -avd bibletext_test -no-window -no-audio -no-boot-anim \
  -gpu swiftshader_indirect &   # headless; works with the Mac display asleep
adb wait-for-device
adb install -r cmd/mobile/BibleText.apk
adb shell am start -n uk.co.bibletext/org.golang.app.GoNativeActivity  # launch
#   (monkey -p uk.co.bibletext 1 now exits 251 against the current manifest)
adb exec-out screencap -p > shot.png      # capture without a visible screen
```

VS Code: **Run Task → "Run on Android Emulator"** does the whole loop —
`scripts/build-android.sh`, boots the `bibletext_test` AVD when no emulator is
online, installs, launches. "Android Logcat (BibleText)" tails the app's log
(Go's `log.Printf` arrives under the `GoLog` tag).

## Getting it to Android testers — two paths

### A. Fast loop (the TestFlight analog): signed APK, no Play review
- **Firebase App Distribution** accepts a plain signed **APK** (not AAB). Free.
  ```bash
  npm install -g firebase-tools   # under ~ if npm prefix is user-owned
  firebase login
  firebase appdistribution:distribute BibleText-universal.apk \
    --app <FIREBASE_ANDROID_APP_ID> \
    --release-notes "Beta" --testers "a@x.com,b@y.com"
  ```
  Testers get an email + the Firebase Tester app; installs like TestFlight.
- **Direct sideload**: send the `.apk`; the tester taps it and grants "install
  unknown apps" for their browser/file manager. Simplest, zero infra.
- Note: an **AAB cannot be installed on a device** — only APKs sideload.

### B. Google Play (needs a developer account + the web Console)
Steps that ONLY the Play Console web UI can do (no CLI):
1. **Create a Google Play Developer account — $25 one-time.** Needs a Google
   account with 2-Step Verification, payment, and government-ID verification.
2. Create the app record (`uk.co.bibletext`), then fill: **store listing**
   (title, short/full description, screenshots, feature graphic, icon),
   **content rating** questionnaire, **Data safety** form, **target audience**,
   **app content** declarations (ads?, privacy-policy URL), **Play App Signing**
   opt-in (recommended).
3. Upload the **`.aab`** to a **Closed testing** track and add testers.
4. **New personal accounts (created after 2023-11-13) must run a closed test
   with ≥12 testers, continuously opted in for 14 days, BEFORE applying for
   production access.** (Was 20 testers; reduced to 12 on 2024-12-11.) Org
   accounts (need a D-U-N-S number) are exempt.
5. After the 14-day test, apply for production access, then roll out.
   Uploading the `.aab` and rolling tracks can be automated later (Play Developer
   Publishing API / Gradle Play Publisher); the listing + declarations + the
   production-access application are UI-only.

**Recommendation:** use path A (Firebase or sideload with the signed APK) to get
Android testers going immediately — same role TestFlight plays for iOS — and run
path B in parallel (create the account, start the 12×14 closed test) since that
gate takes two calendar weeks regardless.

## Watch-outs
- Play may raise the target-API floor to **36** around Aug 2026; Fyne hardcodes
  target 35, so we'd bump Fyne or supply a custom `AndroidManifest.xml` then.
- The debug APK is ~120 MB (all ABIs bundled); the Play `.aab` splits per device
  so real downloads are far smaller.

## Tablets

The one mobile binary serves phones and tablets: `deviceIsTablet()` on Android
(`device_android.go`) applies the sw600dp convention to the live canvas — a
window whose smallest dimension is ≥ 600 logical units (`isTabletDimensions`,
`layout.go`) is tablet-class, and at ≥ 700pt width `classifyLayout` gives it
the **regular** sidebar+split layout (the iPad layout — hideable sidebar,
orientation-driven default, native overlay clipped to the reading pane).
Because the canvas has no size until after the first build, `layoutMayChange()`
is always true on Android, so the `layoutWatcher` is installed on phones too —
they just never cross the breakpoint. Verified on the Pixel Tablet AVD.
