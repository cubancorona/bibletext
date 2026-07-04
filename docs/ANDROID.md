# BibleText on Android — build, sign, distribute

Companion to the iOS notes. Android has a **native reading overlay** (a
selectable `TextView` in a `Dialog` floated over the Fyne GL surface —
`android/BtBridge.java` + `reading_android.go`), the twin of the iOS
`UITextView`, so readers get real long-press selection with drag handles, the
floating toolbar, and BibleText's study menu (Ask AI / Explain / Analyze
context / Analyze translation / Cross-references / Share with citation) — the
same Go dispatch as iOS. Android has **no audio** (the AVFoundation engine is
`darwin`-only; `audio_other.go` stubs it), so the audio control and read-along
don't appear. Everything else (reading, search, Find/Ask AI, navigation,
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

## Known quirks

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
versions separately from the v2.7 toolkit; no patched Fyne needed, the iOS
drawloop patch is iOS-only), injects the dex, and re-signs.

```bash
# Debug/test APK (signed with the upload key, target 29) → cmd/mobile/BibleText.apk
scripts/build-android.sh

# Signed release AAB (Play, target 35) + universal APK (sideload/Firebase)
# → ~/Library/Android/bibletext-dist/{BibleText.aab, BibleText-universal.apk}
BIBLETEXT_ANDROID_BUILD=<versionCode> scripts/build-android.sh --release
```

The release path: `fyne release -os android` emits a signed `.aab` via
bundletool, then the script adds `base/dex/classes2.dex` and re-signs
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
  the session notes.

## Emulator (local testing)

```bash
avdmanager create avd -n bibletext_test \
  -k "system-images;android-35;google_apis;arm64-v8a" -d pixel_7
emulator -avd bibletext_test -no-window -no-audio -no-boot-anim \
  -gpu swiftshader_indirect &   # headless; works with the Mac display asleep
adb wait-for-device
adb install -r cmd/mobile/BibleText.apk
adb shell monkey -p uk.co.bibletext 1     # launch
adb exec-out screencap -p > shot.png      # capture without a visible screen
```

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
