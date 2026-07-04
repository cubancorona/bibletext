# BibleText on Android — build, sign, distribute

Companion to the iOS notes. Android uses the **Fyne RichText reading view**
(`reading_mobile.go`), NOT the native text overlay iOS/macOS use, and has **no
audio** (the AVFoundation engine is `darwin`-only; `audio_other.go` stubs it), so
the audio control and read-along simply don't appear on Android. Everything else
(reading, search, Find/Ask AI, navigation, versions, history, text size) is
data-driven and works from the same code.

**Verified 2026-07-04** on the API-35 arm64 emulator: reading (John 1), Books,
live Search ("sheep" → 120 matches, highlighted), and the Bible cache
(cache_path_android.go — see Known quirks). Not exercised: AI features (BYOK
keys) and Android hardware keyboards.

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

The Fyne CLI (`fyne-io/tools` v1.7.2 — this IS current; the CLI versions
separately from the v2.7 toolkit) drives it. No patched Fyne needed (the iOS
drawloop patch is iOS-only).

```bash
cd cmd/mobile

# Debug/test APK — installable on emulator or by sideload. Target API 29, min 15.
fyne package -os android -app-id uk.co.bibletext -icon Icon.png
#  → BibleText.apk  (a "fat" APK with all ABIs; ~120 MB)

# Signed RELEASE Android App Bundle (.aab) for Google Play. Target API 35 (meets
# Play's current floor), min 15. Emits an .aab via bundletool.
fyne release -os android \
  -app-id uk.co.bibletext -icon Icon.png \
  -app-version 1.0.0 -app-build <N> \
  -keyStore   ~/Library/Android/bibletext-signing/bibletext-upload.keystore \
  -keyName    bibletext \
  -keyStorePass "$(cat ~/Library/Android/bibletext-signing/keystore-password.txt)" \
  -keyPass      "$(cat ~/Library/Android/bibletext-signing/keystore-password.txt)"
#  → BibleText.aab
```

`-app-build <N>` = Android `versionCode`; it MUST strictly increase with every
Play upload. There is no `-targetSdk` flag (Fyne hardcodes 35); `-androidapi <n>`
raises `minSdkVersion` if desired (e.g. `-androidapi 24` for Android 7.0).

To get a **signed installable APK from the AAB** (for Firebase/sideload):

```bash
bundletool build-apks --mode=universal \
  --bundle=BibleText.aab --output=BibleText.apks \
  --ks=~/Library/Android/bibletext-signing/bibletext-upload.keystore \
  --ks-key-alias=bibletext \
  --ks-pass=file:~/Library/Android/bibletext-signing/keystore-password.txt \
  --key-pass=file:~/Library/Android/bibletext-signing/keystore-password.txt
unzip -p BibleText.apks universal.apk > BibleText-universal.apk
```

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
