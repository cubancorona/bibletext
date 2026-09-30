# Store screenshot playbook

Every store listing shows the same eight scenes, in the same order, each
captured from a store build on the platform the listing is for. This document
is the whole procedure, so that a retake has nothing to rediscover: the shot
list and what each image must contain (§1), the standard for shot 08 (§2),
what each store takes and shows today (§3), a capture recipe per device (§4),
the three-lens check every set passes (§5), the approval gate and the upload
order (§6), and what is still missing (§7).

What happened to past sets stays where it happened: "The 1.2.17 set" and "The
macOS set" in [APP_STORE_SUBMISSION.md](APP_STORE_SUBMISSION.md), "Graphics"
in [PLAY_LISTING.md](PLAY_LISTING.md), and the runner captures in
[screenshots/README.md](screenshots/README.md).

## Rules for every set

- **The eight scenes of §1, in its order, on every device and in every
  store.** A store that holds fewer images takes a subset in the same order
  (§7 for the Snap Store).
- **A store build.** iOS is never built with `--dev`: a dev build adds the
  Links tab and a debug line. Android is the release APK. The Mac, Windows
  and Linux executables carry the release API.Bible value, as the store
  packages do, or shot 01 cannot open the NKJV.
- **Status bar at 09:41 with a full battery and full signal**: `xcrun simctl
  status_bar … override` on iOS, the System UI demo mode on Android. Desktop
  images are the window alone.
- **Only synthetic notes, written for the listing** — never a reader's data,
  never a copy of anyone's preferences file. The notes are invented, but they
  are written the way people actually send a verse to someone, because that
  is what the feature is for, and every store shows the same six (§1). Their
  text lives only in the images and on the capture devices: the repository
  does not quote it, since `scripts/check-repository-hygiene.py` refuses
  realistic private-message phrases in tracked text
  ([PRIVACY_RELEASE_CHECKLIST.md](PRIVACY_RELEASE_CHECKLIST.md)).
- **No real API key is typed, pasted or shown**, on any device. Shot 03 shows
  the key field empty, with its placeholder.
- **Nothing composed.** Every image is a capture of the app. The only edits
  allowed make a set consistent with itself — a status-bar strip or a
  window-corner backdrop taken from another image of the same set — and each
  is recorded with the set's history.
- **Three independent reviewers check every set** (§5). Anything any of them
  flags is retaken, and all three check again.
- **Nothing goes to any store until the account holder has seen the final
  set** (§6).

## 1. The eight shots

Read from the upload-ready 1.2.17 iPhone set, which meets this list except
where §7 says otherwise.

| # | Scene | Exact content | Appearance |
| --- | --- | --- | --- |
| 01 | Matthew 1, NKJV | Read. Header: BibleText, "New King James Version ▾", Go to, the verse-of-the-day sparkle, the gear; the recent-chapters row. "Matthew 1", "Chapter 1 of 28". The NKJV heading "The Genealogy of Jesus Christ", then verse 1 as its own paragraph, verses 2–6 as one paragraph and verse 7 opening the next: the Junicode reading face, the publisher's paragraphing, italic supplied words ("who had been the wife"). | Light |
| 02 | Matthew 1, NKJV | The same frame as 01. | Dark |
| 03 | Settings, Assistant | The Settings sheet over Read, at its ASSISTANT section: None, Gemini (Google) selected, ChatGPT (OpenAI), Claude (Anthropic), Grok (SpaceXAI); "Gemini key" with "Get a key ↗"; the key field **empty**, showing its placeholder; Paste, Test key, Clear; Model ▾ Recommended with its one-line note; the paragraph on what AI study sends, and the Privacy Policy link; TRANSLATIONS starting below. | Light |
| 04 | The translation list | The Translation sheet ("Choose a Bible version.") over Read: New King James Version (NKJV) ticked, with "© Thomas Nelson (HarperCollins Christian) — license required" and the permission notice ending "Text provided via API.Bible (api.bible)."; then World English Bible (WEB), Berean Standard Bible (BSB), World English Bible (Catholic) (WEBC), each with its licence line; Close. | Light |
| 05 | The notes list | Search, with the notes button beside Search and Find selected: "Search your notes…", "All 6 notes.", Everyone, Newest first; the six notes below, newest first, each with its passage, "(NKJV)", byline, date and delete button. | Light |
| 06 | A note on Psalm 23 | Read, "Psalms 23", "Chapter 23 of 150". The received note on Psalms 23:1-3 open above verse 1, headed "Note from Friend · 2 of 2 in this chapter ›" with its minimise and delete controls; verses 1–3 under the note's wash, verse 4 without it; the divine name in small capitals. | Light |
| 07 | The Books grid | Books: "Filter books", OLD TESTAMENT, the two-column grid from Genesis and Exodus down to Lamentations and Ezekiel, Psalms highlighted as the current book. | Light |
| 08 | Study with AI on Psalm 82:1 | §2. | Light |

The layout follows the device. The iPhone, the iPad and the Android phone are
in portrait with the bottom bar (Read, Books, Search); the iPad and the
Android tablet set a chapter as a book page, a centred column. The Android
tablet is in landscape, with the rail at the left. The Mac is a 1280×800
window with its title bar and the rail at the left. Where the table says
Read, Books or Search, it means the rail entry on those two.

### The notes

Shot 05 lists six notes and shot 06 opens one of them. "Friend" is the byline
on every received note: the app shows no sender names yet, whatever name a
note carries (`senderNamesEnabled` in `notes_byline.go`). "From you" marks a
note the reader sent.

| Passage (NKJV) | Byline | Date in the 1.2.17 set |
| --- | --- | --- |
| Psalms 23:1-4 | From Friend | 22 Aug |
| Psalms 23:1-3 | From Friend | 21 Aug *(the note shot 06 opens)* |
| Philippians 4:6-7 | From you | 21 Aug |
| Isaiah 40:31 | From Friend | 20 Aug |
| Romans 8:38-39 | From you | 17 Aug |
| Matthew 11:28-30 | From Friend | 12 Aug |

The wording is the wording of the 1.2.17 iPhone set:
`build/appstore/screenshots-ready-1.2.17/en-GB/iphone-05-notes-list.png` is
the reference, read from the image (or from the simulator's preferences, which
hold it), and every store's set carries exactly the same six. It was kept
deliberately when a neutral alternative was offered in September 2026: five of
the six read as notes one person sends another, which is the use the listing
is showing. A change of wording is a decision for the account holder, and then
every store's 05 and 06 are retaken together.

Where the notes come from:

- **iOS simulators** keep them in the app's preferences. A simulator without
  them takes the file from one that has them: terminate the app on both, copy
  `preferences.json` between the two data containers (`xcrun simctl
  get_app_container <udid> uk.co.bibletext data`, then `find` the file there),
  and relaunch. A simulator cannot open a universal link and Apple builds
  register no custom scheme, so links are not a way in.
- **Android** opens a link minted by the app's own `ShareLinkURLWithNote`,
  the way the runner workflows mint theirs:
  `adb shell am start -a android.intent.action.VIEW -d '<link>'`. Press HOME
  between two links (`adb shell input keyevent KEYCODE_HOME`): with the app
  in the foreground, a second link is swallowed.
- **The Mac capture build** takes a link as its one argument at launch (§4,
  Mac, step 3), one link per launch: quit the app before launching it with the
  next. A Mac release build has no single-instance handoff
  (`single_instance_off.go`), so a second launch does not pass its link to the
  running one, as it does in the Linux and Windows runner workflows; it starts
  a second process, and two processes writing one `preferences.json` can lose
  notes. The Mac's preferences can seed a simulator in turn.
- The reader's own two notes are written in the app: select the passage, then
  Share > Share with note, write the text in the "Add a note" composer, and
  tap its Share button. The note is kept on the passage before the platform's
  share sheet opens (`share.go`), so close that sheet without sending
  anything. On Android, type the text by tapping the on-screen keys by
  coordinate, never with `adb shell input text`: characters typed through the
  hardware-key path arrive doubled in the app's text fields
  ([ANDROID.md](ANDROID.md)).

A note seeded on the day of capture shows that day's date. The content lens
checks passages, bylines and wording, not dates.

### File names

Every store's set is numbered in §1's order; the number and the name travel
together, and both `push-screenshots.py` tools, `appstore/` and `play/`,
upload in sorted name order.

| # | iPhone, `en-GB/` | iPad, `en-GB/ipad13/` | Mac, `en-GB/mac/` | Play, `phone/` and `tablet10/` |
| --- | --- | --- | --- | --- |
| 01 | `iphone-01-matthew1-nkjv.png` | `ipad-01-matthew1-nkjv.png` | `mac-01-matthew1-nkjv.png` | `01-matthew1-nkjv.png` |
| 02 | `iphone-02-matthew1-dark.png` | `ipad-02-matthew1-dark.png` | `mac-02-matthew1-dark.png` | `02-matthew1-dark.png` |
| 03 | `iphone-03-settings-ai.png` | `ipad-03-settings-ai.png` | `mac-03-settings-ai.png` | `03-settings-ai.png` |
| 04 | `iphone-04-translations-nkjv.png` | `ipad-04-translations-nkjv.png` | `mac-04-translations-nkjv.png` | `04-translations-nkjv.png` |
| 05 | `iphone-05-notes-list.png` | `ipad-05-notes-list.png` | `mac-05-notes-list.png` | `05-notes-list.png` |
| 06 | `iphone-06-note-psalm23.png` | `ipad-06-note-psalm23.png` | `mac-06-note-psalm23.png` | `06-note-psalm23.png` |
| 07 | `iphone-07-books-grid.png` | `ipad-07-books-grid.png` | `mac-07-books-grid.png` | `07-books-grid.png` |
| 08 | `iphone-08-study-with-ai-psalm82.png` | `ipad-08-study-with-ai-psalm82.png` | `mac-08-study-with-ai-psalm82.png` | `08-study-with-ai-psalm82.png` |

The App Store directories sit under `build/appstore/screenshots-ready-<v>/`,
with the captures they were drawn from in `build/appstore/screenshots-<v>/{iphone,ipad13,mac}/`.
The Play directories sit under `build/play/screenshots-<v>/`. The Microsoft
Store and the Snap Store have no eight-scene set yet (§7); when they do, the
files take the Play names under `docs/screenshots/windows/` and
`docs/screenshots/linux/`.

## 2. Shot 08 — Study with AI on Psalm 82:1

The other seven can be judged at a glance; this one cannot, and earlier sets
missed it three ways: a single word selected, the menu over the verse, and a
different verse on one device. The standard:

- The NKJV, Psalm 82, light. Visible above the selection: the superscription
  "A Psalm of Asaph." and the heading "A Plea for Justice".
- **The whole of verse 1 is selected** — "God stands in the congregation of
  the mighty; / He judges among the gods." — from its number or its first
  word to the full stop after "gods". Nothing of the superscription, the
  heading or verse 2 is selected.
- The three Study with AI items — **Explain, Analyze context, Analyze
  translation** — show **below the selection**, as a reader sees them after
  choosing Study with AI. They cover neither the verse nor its number;
  covering part of verse 2 is fine.
- On the Mac the context menu is open with Study with AI highlighted and its
  submenu, the same three items, opening to the left inside the window. The
  selection starts just after the verse number, so Look Up quotes the verse
  rather than the number.
- **The same verse on every device, in every store.**

The selection is read word by word from an enlarged crop (§5), never judged
from the whole image.

## 3. The stores

### What each store takes

| Store | Device classes | Size and shape | Images | Upload-ready files | Upload |
| --- | --- | --- | --- | --- | --- |
| App Store (iOS) | iPhone 6.9-inch (`APP_IPHONE_67`) and iPad 13-inch (`APP_IPAD_PRO_3GEN_129`); smaller devices scale from these | iPhone 1320×2868 (1290×2796 and 1260×2736 also taken), iPad 2064×2752 (2048×2732 also taken); portrait here, landscape also taken; PNG without alpha. `ACCEPTED_SCREENSHOT_SIZES` in `appstore/preflight.py` is the list | 8 + 8; a set holds at most 10 | `build/appstore/screenshots-ready-<v>/en-GB/` and `…/ipad13/` | `appstore/push-screenshots.py` |
| Mac App Store | Mac (`APP_DESKTOP`) | 2560×1600 (1280×800, 1440×900 and 2880×1800 also taken), landscape only; PNG without alpha | 8 | `…/en-GB/mac/` | `push-screenshots.py --platform MAC_OS` |
| Google Play | Phone, 10-inch tablet, feature graphic | Phone 1080×2160: the long side at most twice the short, and a Pixel's own 1080×2400 is past it. Tablet 2560×1600 from the Pixel Tablet profile. Feature graphic 1024×500. PNG without alpha (Play asks for JPEG or 24-bit PNG) | 8 phone + 8 tablet (at most eight per device type) + the feature graphic | `build/play/screenshots-<v>/phone/` and `…/tablet10/`; `docs/play-assets/feature-graphic.png` | `play/push-screenshots.py` for the phone and tablet; the feature graphic in the console |
| Microsoft Store | Desktop | PNG, 1366×768 or larger, at most 50 MB; key content in the top two thirds; no added logos or marketing text; an optional caption of at most 200 characters | At most 10, one required, four or more recommended | `docs/screenshots/windows/` | Partner Center only |
| Snap Store | Desktop | GIF, JPEG or PNG; 480×480 to 3840×2160; aspect between 1:2 and 2:1; at most 2 MB each; a 3:1 banner besides | At most 5 | `docs/screenshots/linux/` | The snapcraft.io dashboard only |
| AppStream metainfo (software centres, AppImageHub) | Desktop | The `docs/screenshots/linux/` files, by commit-pinned URL | As many as `linux/listing.toml` lists | `docs/screenshots/linux/` and `screenshots.ref` | `go run ./cmd/linuxmeta render`, then a commit |

Two of the Play limits above — JPEG or 24-bit PNG, and at most eight images
per device type — are not taken from a tracked source. Check them against the
console's own text on the main store listing page before an upload.
`play/push-screenshots.py` is stricter than Play on the format: it takes
24-bit RGB PNG only and refuses a JPEG. It holds at most eight images per
type and at most 8 MB per file, so a change in either limit is a change to
its `MAX_IMAGES` or `MAX_BYTES` too.

### What each store shows today

As the sources record it on 30 September 2026:

| Store | Shows | Last replaced |
| --- | --- | --- |
| App Store (iOS) | The 1.2.17 version record holds the 1.2.17 iPhone and iPad sets. Readers see them once 1.2.17 is released; until then the released version shows what every release since 1.2.2 inherited, the 1.2.2 captures (the 1.2.3 replacement was prepared and never uploaded). | 29 September 2026 |
| Mac App Store | The 1.2.17 version record holds the 1.2.17 Mac set, shown from 1.2.17's release. What earlier Mac versions carried is not recorded here. | 29 September 2026 |
| Google Play | The 1.2.17 set from `build/play/screenshots-1.2.17/`: eight phone images (1080×2160) and eight 10-inch tablet images (2560×1600), the first tablet images the listing has carried, with `docs/play-assets/feature-graphic.png` and `icon-512.png` unchanged. `play/push-screenshots.py --write` committed them on 30 September 2026, and a fresh edit read them back matching by count, order and sha256. A committed listing change goes through Play's review first, so the store shows them once the Play Console shows the change published; until then shoppers still see the 1.2.5 set in `docs/play-assets/2026-09-1.2.5/`. The phone images were taken at the Pixel 11 Pro's Small display size (density 356), not its default 420 (§4, Android phone, step 3). | Committed 30 September 2026; in Play's review |
| Microsoft Store | The four runner captures in `docs/screenshots/windows/` (1600×960; reading, search, note and settings in that order, each captioned), the BSB in light, uploaded by hand with submission 1. `msstore/submit.py` carries them into every later submission unchanged. | Captured 16 September 2026, live 17 September 2026 |
| Snap Store | No gallery images are recorded as set: the item stands open in [LINUX_STORES.md](LINUX_STORES.md), "Listing work that snapcraft.yaml cannot do". | — |
| AppStream metainfo | The four runner captures in `docs/screenshots/linux/` (1280×860; reading, search, note, settings), the BSB in light, pinned at the commit `screenshots.ref` names. | 16 September 2026 |

## 4. Capture recipes

### Before the first capture, on every device

1. The NKJV is the selected translation: the header reads New King James
   Version.
2. The six notes of §1 are present, with the wording of the reference image
   (§1), and nothing else is in the notes list.
3. Settings, Assistant: Gemini (Google) selected and no key saved.
4. The recent-chapters row shows only chapters opened while preparing the
   set; its bin button clears it.

### iPhone — the iOS simulator

The simulators are driven from the command line only — `xcrun simctl` for
everything it can do, lldb for taps and the edit menu — never through an
interactive simulator panel. A new simulator may be created when one is
needed, one at a time.

1. Use the iPhone 17 Pro Max simulator (1320×2868). If none exists, create
   one — `xcrun simctl create "iPhone 17 Pro Max" "iPhone 17 Pro Max"` — and
   boot it with `xcrun simctl boot <udid>`.
2. Install a store build: `BIBLETEXT_SIM_DEVICE="iPhone 17 Pro Max"
   scripts/run-ios-sim.sh`, never with `--dev`. The script installs on the
   first booted iOS simulator, not on the one named. With two booted, install
   by hand from the one that has it:
   `xcrun simctl install <udid> "$(xcrun simctl get_app_container <other udid> uk.co.bibletext app)"`.
3. Status bar:
   `xcrun simctl status_bar <udid> override --time 9:41 --batteryState charged --batteryLevel 100 --cellularMode active --cellularBars 4 --dataNetwork wifi --wifiMode active --wifiBars 3`.
   An en-GB simulator shows the time as 09:41.
4. Appearance, set before the app starts: `xcrun simctl ui <udid> appearance
   light` (`dark` for 02), then `xcrun simctl terminate <udid>
   uk.co.bibletext` and `xcrun simctl launch <udid> uk.co.bibletext`. The
   launch prints the process id lldb attaches to.
5. Taps. Attach with `lldb -p <pid>` and send each tap as a touch that
   begins and ends at the same point:
   `expr ((void (*)(unsigned long, unsigned long, float, float))sendTouch)(1, 0, <x>, <y>)`,
   then the same with `2` in place of `0`, then `continue`. `sendTouch` is
   the toolkit's own touch entry (`internal/driver/mobile/app/darwin_ios.go`
   in the patched Fyne tree); its coordinates are the screen's pixels, which
   are a capture's pixels. The call runs while lldb holds the app paused at an
   arbitrary point, and it kills the app on roughly one tap in five: relaunch
   and repeat. A death after an injected tap is not evidence of an app crash
   unless the previous build dies at the same rate under the same taps.
6. Capture: `xcrun simctl io <udid> screenshot
   build/appstore/screenshots-<v>/iphone/iphone-NN-<slug>.png`. Judge the
   pixels, not the file name: `simctl io screenshot` can store a landscape
   frame in a portrait buffer.
7. Shot 08 needs no gesture. With Psalm 82 open and lldb attached:
   1. `expr -l objc -- @import UIKit`, so lldb knows the UIKit types, then
      `expr -l objc -- (void)[(UITextView *)gReadingTV becomeFirstResponder]`.
   2. Find verse 1 in `[(UITextView *)gReadingTV text]` with `rangeOfString:`
      — the end is the end of "He judges among the gods.", the start is verse
      1's number just before "God stands" — and select it:
      `expr -l objc -- (void)[(UITextView *)gReadingTV setSelectedRange:(NSRange){<start>, <length>}]`.
   3. Prove the selection before going on:
      `po [[(UITextView *)gReadingTV text] substringWithRange:[(UITextView *)gReadingTV selectedRange]]`
      prints verse 1 and nothing else.
   4. The items a reader sees after choosing Study with AI are the "Study
      with AI" submenu of the menu the text view's delegate builds with
      `textView:editMenuForTextInRange:suggestedActions:`. Build that menu
      and keep the submenu in an lldb variable:
      `expr -l objc -- UIMenu *$all = [(id)[(UITextView *)gReadingTV delegate] textView:(UITextView *)gReadingTV editMenuForTextInRange:[(UITextView *)gReadingTV selectedRange] suggestedActions:@[]]`,
      then `po $all.children` and take the element titled "Study with AI":
      `expr -l objc -- UIMenu *$studyMenu = (UIMenu *)$all.children[<its index>]`.
      `po $studyMenu.children` lists Explain, Analyze context and Analyze
      translation.
   5. Present it below the selection through the text interaction's
      edit-menu assistant:
      `expr -l objc -- ((void (*)(id, SEL, long, id))objc_msgSend)([[(id)gReadingTV interactionAssistant] _editMenuAssistant], @selector(_presentEditMenuWithPreferredDirection:overrideMenu:), 1, $studyMenu)`.
      Direction 1 puts the menu below the selection, 2 above it. These are
      private UIKit methods: acceptable on a simulator for a capture, never in
      the app.
   6. `continue`, wait a second, capture, then `detach`.
8. Redraw opaque copies into `build/appstore/screenshots-ready-<v>/en-GB/`
   under the same names. The simulator writes an alpha channel and App Store
   Connect refuses a PNG that carries one:
   `python3 -c 'import sys; from PIL import Image; Image.open(sys.argv[1]).convert("RGB").save(sys.argv[2])' <capture> <ready copy>`.
9. `python3 appstore/push-screenshots.py --local-only` checks the ready set's
   sizes and alpha channels without touching the network.

### iPad — the iOS simulator

1. Use the iPad Pro 13-inch (M5) simulator (2064×2752), in portrait for every
   shot, the dark 02 included: the app cannot turn the simulated device from
   inside its own process. Create one as for the iPhone if none exists.
2. Install, status bar and appearance as for the iPhone. The iPad's status
   bar also shows the date, which the override cannot set, so capture the
   whole set on one day. (When one image did differ, the 1.2.17 set gave iPad
   08 the top 60 pixel rows of iPad 07.)
3. iPadOS 26 and 27 simulators open apps in windowed mode, which draws a
   small resize grip in the bottom-right corner. The 1.2.17 set keeps it.
   Only Settings > Multitasking & Gestures > Full Screen Apps, chosen by hand
   inside the simulated iPad, removes it; no command-line setting reaches it.
4. Taps, shot 08 (direction 1, below the selection), captures under
   `build/appstore/screenshots-<v>/ipad13/` and opaque copies under
   `…/screenshots-ready-<v>/en-GB/ipad13/`, all as for the iPhone.

### Mac

A Mac process is never attached to lldb: attaching raises a Developer Tools
password prompt. The Mac shots are driven with the mouse and keyboard.

1. Build an unsigned executable with the patched toolkit and the release
   API.Bible value, as the store package is built, and put `go.mod` back:

   ```
   scripts/setup-fyne-patch.sh
   mkdir -p build/mac-shots
   cp go.mod build/mac-shots/go.mod.original
   go mod edit -replace fyne.io/fyne/v2=./third_party/fyne
   source scripts/release-bible-key.sh
   load_release_bible_key
   go build -C cmd/bibletext -trimpath -ldflags="$BIBLE_KEY_LDFLAGS" -o ../../build/mac-shots/BibleText .
   BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" python3 scripts/verify-release-key.py build/mac-shots/BibleText
   clear_release_bible_key
   cp build/mac-shots/go.mod.original go.mod
   ```

   An unsigned build keeps no secret in the Keychain.
2. A throwaway home, seeded with the synthetic state. The desktop keys its
   preferences `bibletext` (`app.go`, `NewWithID`), not the bundle id:

   ```
   SHOTS_HOME="$PWD/build/mac-shots/home"
   mkdir -p "$SHOTS_HOME/Library/Preferences/fyne/bibletext"
   cp <the simulator's preferences.json> "$SHOTS_HOME/Library/Preferences/fyne/bibletext/"
   shasum -a 256 "$HOME/Library/Preferences/fyne/bibletext/preferences.json"
   ```

   The last line records the reader's own preferences file, which the capture
   never opens, moves or photographs; step 9 proves it unchanged.
3. Launch: `HOME="$SHOTS_HOME" CFFIXED_USER_HOME="$SHOTS_HOME"
   build/mac-shots/BibleText &`. `CFFIXED_USER_HOME` moves the home the
   system frameworks resolve, which `HOME` alone does not.
4. Pin the window to 1280×800 points, which is 2560×1600 on a Retina display,
   against the screen's right edge, so shot 08's submenu opens to the left
   inside the frame:
   `osascript -e 'tell application "System Events" to tell (first process whose unix id is <pid>) to set {position, size} of window 1 to {{<x>, <y>}, {1280, 800}}'`
   (System Events needs Accessibility permission for the terminal).
5. Capture by region: `screencapture -x -R<x>,<y>,1280,800
   build/appstore/screenshots-<v>/mac/mac-NN-<slug>.png`. A region capture
   keeps shot 08's context menu, which a window capture drops. The window's
   rounded corners show whatever is behind them, so keep one plain backdrop
   for the whole set (the 1.2.17 Mac 08 took Mac 07's backdrop for its 804
   corner pixels).
6. Shot 08: drag from just after verse 1's number to the full stop after
   "gods", right-click at the end of the first line, and hover over Study
   with AI.
7. Shot 02 needs the system appearance switched to Dark. `-AppleInterfaceStyle
   Dark` darkens the content but leaves a light title bar, a combination no
   reader can produce. The appearance is a setting of the whole Mac, so the
   account holder switches it (System Settings > Appearance) and switches it
   back, and the capture waits for that. Capture 02 by window id, so nothing
   else on the screen can enter the frame: `screencapture -x -o -l <window id>
   build/appstore/screenshots-<v>/mac/mac-02-matthew1-dark.png`, with the id
   from `CGWindowListCopyWindowInfo`:

   ```
   swift - <<'SWIFT'
   import CoreGraphics
   let windows = CGWindowListCopyWindowInfo(.optionOnScreenOnly, kCGNullWindowID) as! [[String: Any]]
   for w in windows where w[kCGWindowOwnerName as String] as? String == "BibleText" {
       print(w[kCGWindowNumber as String]!)
   }
   SWIFT
   ```

   An appearance switch can bring up Stage Manager's introduction panel;
   dismiss it before capturing.
8. `screencapture` always writes an alpha channel. Redraw opaque copies into
   `…/screenshots-ready-<v>/en-GB/mac/`: a region capture as for the iPhone;
   the window capture has transparent corners, so lay it over the set's
   backdrop first, so its corners match the other seven:
   `python3 -c 'import sys; from PIL import Image; bg = Image.open(sys.argv[1]).convert("RGB"); w = Image.open(sys.argv[2]).convert("RGBA"); bg.paste(w, (0, 0), w); bg.save(sys.argv[3])' <a region capture> <the window capture> <ready copy>`.
9. `python3 appstore/push-screenshots.py --platform MAC_OS --local-only`, and
   the reader's preferences hash from step 2 again: it must not have changed.

### Android phone

1. Build: `scripts/build-android.sh --release`. Its
   `~/Library/Android/bibletext-dist/BibleText-Android.apk` is the universal
   APK drawn from the release AAB, which is the store build.
2. A capture AVD of its own, so an emulator in use for verification is not
   disturbed, matching the phone class the account holder uses: the Pixel 11
   Pro, 1080×2410 at 420 dpi, on Android 17. `bibletext_pixel11pro` is the
   Pixel 9 profile with its display set to those values in `config.ini`
   (`hw.lcd.width=1080`, `hw.lcd.height=2410`, `hw.lcd.density=420`) and the
   `system-images;android-37.0;google_apis;arm64-v8a` image. It is kept
   between releases. The line the 1.2.17 set was taken with:

   ```
   emulator -avd bibletext_pixel11pro -port 5580 -memory 4096 -feature GLDirectMem,HasSharedSlotsHostMemoryAllocator -no-snapshot-save -no-boot-anim -no-window -no-audio -no-metrics -qemu -enable-hvf &
   adb -s emulator-5580 wait-for-device
   ```

   Capture on a freshly booted emulator. Every command below names it with
   `-s emulator-5580`.
3. The display, before anything else. The Pixel 11 Pro captures 1080×2410,
   2.23:1 and past Play's 2:1, so set the display itself to exactly 1080×2160
   — `adb -s emulator-5580 shell wm size 1080x2160` — and confirm that `adb
   -s emulator-5580 shell wm size` reports the override. **Never crop a
   capture instead**: cropping the 2026-09 set to 1080×2160 took the top of
   the app header with the status bar.

   The density stays at the default 420, what most people see. The 1.2.17
   phone set is the exception: it was taken at `adb -s emulator-5580 shell
   wm density 356`, the smallest step of Android's Display size setting on a
   420 phone (`int(420 × 0.85)`, rounded down to even), because at 420 three
   screens were taken for faults of the app (§7). Two are its phone layout
   working as designed at 420: Books in one column, and Settings putting
   Clear on a line of its own and scrolling. The third, the Translation
   sheet starting partway down the header, was a defect, since fixed. The
   toolkit draws the interface at 3× from 405 dpi and at 2× below, so the
   two sizes differ by more than the density ratio. Whichever density a set
   uses, every image in it uses the same one.
4. Appearance, set before the app starts: `adb -s emulator-5580 shell cmd
   uimode night no` (`yes` for 02). Changing it while the app runs has
   brought the app back with its pane in the top 45% of the window.
5. The demo-mode status bar:

   ```
   adb -s emulator-5580 shell settings put global sysui_demo_allowed 1
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command enter
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command clock -e hhmm 0941
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command battery -e level 100 -e plugged false
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command network -e wifi show -e level 4
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command network -e mobile show -e datatype none -e level 4
   adb -s emulator-5580 shell am broadcast -a com.android.systemui.demo -e command notifications -e visible false
   ```

6. Install and launch: `adb -s emulator-5580 install -r
   ~/Library/Android/bibletext-dist/BibleText-Android.apk`, then `adb -s
   emulator-5580 shell am start -n
   uk.co.bibletext/org.golang.app.GoNativeActivity`. Read the version back
   with `adb -s emulator-5580 shell dumpsys package uk.co.bibletext | grep
   versionName`.
7. Taps: `adb -s emulator-5580 shell input tap <x> <y>`, in a capture's
   pixels.
8. Capture: `adb -s emulator-5580 exec-out screencap -p >
   build/play/screenshots-<v>/phone/NN-<slug>.png`. Gate every frame on the
   layout check — is there ink where the tab bar belongs — which catches the
   half-height pane that a "content reaches the bottom" test passes:
   `python3 -c 'import importlib.util as u, sys; s = u.spec_from_file_location("c", "scripts/play-shot-check.py"); m = u.module_from_spec(s); s.loader.exec_module(m); print(m.laid_out(sys.argv[1]))' <capture>`.
   (Run as a script, `play-shot-check.py` grabs from emulator-5554 only;
   §7.) If `png_inspect` in `appstore/preflight.py` reports an alpha channel,
   redraw the file as RGB, as for the iPhone.
9. Shot 08. A long press selects one word: `adb -s emulator-5580 shell input
   swipe <x> <y> <x> <y> 1000`. Drag the start handle to the start of verse
   1's first word and the end handle to just after the full stop after
   "gods" — on that line, never onto the next, where verse 2 begins — with
   one `input swipe <from x> <from y> <to x> <to y> 800` each, and confirm on
   a capture that exactly verse 1 is selected. Then open Study with AI from
   the selection menu — on Android its three actions sit one level below the
   platform's own selection menu — so that they show below the selection.
10. Afterwards: `adb -s emulator-5580 shell am broadcast -a
    com.android.systemui.demo -e command exit`, `adb -s emulator-5580 shell wm
    size reset`, `adb -s emulator-5580 shell wm density reset`, `adb -s
    emulator-5580 emu kill`.

### Android tablet

1. A second capture AVD with the Pixel Tablet profile (2560×1600;
   `avdmanager list device` names the profiles the installed tools know):
   `avdmanager create avd -n bibletext_tablet -k
   "system-images;android-36;google_apis;arm64-v8a" -d pixel_tablet`, booted
   as in phone step 2 on `-port 5582`, so every command names `-s
   emulator-5582`.
2. No `wm size` change: 2560×1600 is 1.6:1, within Play's 2:1.
3. Landscape, so the tablet layout shows: the rail at the left and the
   chapter as a book page, a centred column with indented paragraphs. Tablet
   identity follows the sw600dp rule in `device_android.go`. The Pixel Tablet
   profile boots in landscape; if it does not, turn it with `adb -s
   emulator-5582 emu rotate`, never by writing the rotation settings, which
   switches auto-rotate off.
4. Appearance, demo mode, install, notes and shot 08 as for the phone;
   captures to `build/play/screenshots-<v>/tablet10/NN-<slug>.png`. The
   layout check looks for the portrait tab bar, which the landscape rail
   replaces, so it does not apply here; the framing lens checks the rail and
   the book page by eye.

### Windows

The Microsoft Store rejects composed or foreign-platform images, so the
Windows images come from the Windows build, captured on a GitHub runner by
`.github/workflows/windows-screenshots.yml` and
`scripts/capture-windows-screenshots.ps1`.

1. Dispatch the workflow: Actions > Windows screenshots > Run workflow, or
   `gh workflow run windows-screenshots.yml --ref main`.
2. The run raises the runner's screen from 1024×768 to 1920×1080
   (`Set-DisplayResolution`, with a warning if it stays below the Store's
   1366×768 floor), builds the keyed release executable against the patched
   toolkit with Mesa's software OpenGL beside it, mints its links with the
   app's own `ShareLinkURLWithNote` and `ShareLinkURL`, sizes the window's
   client area to 1600×960, and saves the client area alone — no desktop,
   frame or evaluation watermark — for four scenes in the light appearance:
   note, reading, search, settings.
3. Download: `gh run download <run id> -n windows-screenshots -D
   build/shots/windows-<v>/`.
4. Check the set (§5), replace `docs/screenshots/windows/`, and commit.
5. Upload in Partner Center (§6).

Other scenes, the eight of §1 among them, are a change to
`scripts/capture-windows-screenshots.ps1`, whose scene steps click in client
coordinates.

### Linux

1. Dispatch `.github/workflows/linux-screenshots.yml`.
2. The run builds the keyed executable, runs it under Xvfb (a 1600×1000
   screen) with software OpenGL in the light appearance, drives the same four
   scenes with xdotool from the same minted links, and saves the top-left
   1280×860 of the screen.
3. Download: `gh run download <run id> -n linux-screenshots -D
   build/shots/linux-<v>/`; check the set (§5); replace
   `docs/screenshots/linux/`; commit.
4. The AppStream metainfo names those files by commit-pinned URLs, which
   resolve only once the commit is on GitHub. After that, set
   `screenshots.ref` in `linux/listing.toml` to the commit (and the captions,
   if the scenes changed), run `go run ./cmd/linuxmeta render`, and commit
   the result.
5. The Snap Store gallery takes the same files by hand (§6).

## 5. The three-lens check

Three reviewers, each with one lens, each working from the images and this
document alone — not from the capture notes, and not from one another's
verdicts. Each reports pass or fail for every image, with the reason.
Anything any reviewer flags is retaken, never repainted; then all three check
the whole set again, not just the retaken image. The set is final when a full
round passes with nothing flagged.

### Lens 1 — content

- [ ] Each image is the scene its number names in §1: the chapter, the
      translation, the appearance, and what is open.
- [ ] Every note shown is one of §1's, on its passage, with its byline, and
      its wording matches the reference image word for word. Run OCR over
      every image (macOS's Vision text recognition will do) and compare the
      recognised text, not only the picture.
- [ ] Shot 08 against §2, from an enlarged crop of the selection read word by
      word: exactly verse 1 selected, nothing of the superscription, the
      heading or verse 2; the three items labelled Explain, Analyze context
      and Analyze translation, below the selection; the verse and its number
      uncovered; the same verse as every other device's 08.
- [ ] No key anywhere: shot 03's key field is empty.
- [ ] A store build: no Links tab, no debug line.
- [ ] Nothing of a reader's own: no note, chapter or setting that §1 and §4
      do not put there.

### Lens 2 — framing

- [ ] The whole header is visible: the BibleText title uncut, the version
      name, Go to, the sparkle and the gear.
- [ ] One clean status bar across the set: 09:41, full battery, full signal,
      no notification icons; one date across an iPad set; on the iPhone, the
      Dynamic Island on every image or on none.
- [ ] The bottom bar complete, all three destinations with their labels — or
      the rail, on the Mac and the Android tablet.
- [ ] Nothing half-drawn: no transition caught midway, no keyboard, toast,
      stray popup or Stage Manager panel; every Android phone image passes
      the layout check.
- [ ] Legible at thumbnail size: at about 300 pixels wide, the header and the
      scene's subject still read.
- [ ] The tablet layout is genuine: the Android tablet shows the rail and the
      book page, not a phone layout stretched.
- [ ] The Mac window's corners show one backdrop across the set.

### Lens 3 — files and store rules

- [ ] Every file's pixel size is one its store takes (§3). For the App Store,
      `push-screenshots.py --local-only`, and again with `--platform
      MAC_OS`; for Play, the phone at exactly 1080×2160 and the tablet at
      exactly 2560×1600, which `python3 play/push-screenshots.py --version
      <v> --local-only` checks, and the feature graphic 1024×500.
- [ ] PNG, RGB, no alpha channel: `png_inspect` in `appstore/preflight.py`
      reports `has_alpha` False for every file.
- [ ] Every file present: 24 for the App Store (8 iPhone, 8 iPad, 8 Mac), 16
      for Play (8 phone, 8 tablet) plus the feature graphic.
- [ ] Numbered 01–08 in §1's order under §1's names, and nothing else in the
      directory.
- [ ] No duplicates: `shasum -a 256 <dir>/*.png | awk '{print $1}' | sort |
      uniq -d` prints nothing, and no scene appears twice under two names.
- [ ] The set sits in the directory the upload reads, named for this version
      — never an older `screenshots-ready-*` or a helper's default path.

## 6. Approval, then upload

Nothing goes to any store until the account holder has seen the final set:
after the three lenses pass, as the very files that will be uploaded. A
change after that is shown again. A store's current images stay in place
until the new set has been compared with them side by side.

### App Store and Mac App Store

Per platform, in this order (also [RELEASING.md](RELEASING.md), stage 7, and
"Uploading a set" in [APP_STORE_SUBMISSION.md](APP_STORE_SUBMISSION.md)).
Steps 2 to 5 reach App Store Connect through the local client
`build/appstore/asc.py`, which reads its credentials from the environment, so
load them first in the shell that runs those steps: `. scripts/asc-env.sh`.
The script is sourced, never executed or piped: it exports into the calling
shell and prints one status line naming the key id,
never the issuer id or the key path. `--local-only` needs no credentials.

1. `python3 appstore/push-screenshots.py --local-only`, and with `--platform
   MAC_OS`.
2. The version record must exist: `appstore/submit-version.py … --write
   --confirm-version <v>` creates it, and ends non-zero naming `screenshots`
   — the expected report at this point.
3. `push-screenshots.py --platform IOS` without `--write`, and read the plan.
   Then the same with `--write --confirm-version <v>`: it replaces each set
   in file order, waits for every image to reach `assetDeliveryState`
   COMPLETE, and reads the sets back (count, order, MD5). Repeat with
   `--platform MAC_OS`.
4. `python3 appstore/preflight.py`, and with `--platform MAC_OS`: every image
   COMPLETE.
5. The account holder's `submit-version.py … --submit`. A version in a review
   submission takes no more image edits, so images always go first, and
   `push-screenshots.py` refuses a version past its editable states.

A release that keeps the previous images says so with
`--accept-inherited-screenshots`.

### Google Play

`play/push-screenshots.py` replaces the phone and 10-inch tablet screenshots
on the `en-GB` listing in one edit of the Play Developer API. It uses the
token that `scripts/play-publish.py`'s `access_token()` mints from the
service-account key it reads (`BIBLETEXT_PLAY_KEY`, or its default path);
that account has store-presence access. It touches `phoneScreenshots`, from
`phone/`, and `tenInchScreenshots`, from `tablet10/`, and nothing else: the
feature graphic, the icon and every other language keep what they hold. The
set is `build/play/screenshots-<v>/`, or `--set-dir`. In this order:

1. `python3 play/push-screenshots.py --version <v> --local-only` checks the
   files and makes no request: 1 to 8 per type, 24-bit RGB PNG without
   alpha, the phone exactly 1080×2160 and the tablet exactly 2560×1600, at
   most 8 MB each, no two alike. Every mode runs these checks first.
2. The same without `--local-only` is read-only. It opens an edit, names the
   listing's languages (images are per language, so any language beside
   `en-GB` keeps its own), lists what each type holds by `sha256` beside the
   files that would replace it, and deletes the edit. Read the plan.
   Read-only means the listing does not change. Play keeps one edit open per
   user, so opening one invalidates any other edit the service account has
   open. Every mode but `--local-only` opens one, and `play-publish.py` uses
   the same account. Run nothing else against the account while
   `--rehearse` or `--write` runs.
3. `--rehearse` does everything the write does except the commit. In one
   edit it deletes every image of each type that changes, uploads each file
   in sorted name order, reads the type back — the count, the order, and
   each image's `sha256` against its file's — has Play validate the edit,
   and deletes it. Nothing reaches the listing.
4. `--write --confirm-version <v>`, with `<v>` exactly the `--version`
   given: the rehearsal's steps, then the commit, then a fresh edit that
   reads the committed listing back the same way and is deleted. It ends
   non-zero if the listing does not match the files. The commit sends the
   change to Play's review, and the store shows the new images only after
   that, so a match is a committed listing, not a public one. Record the set
   as in review until the Play Console shows it published.

The commit asks Play to refuse rather than cancel a review already in
progress (`changesInReviewBehavior=ERROR_IF_IN_REVIEW`). If Play answers that
the changes cannot be sent for review automatically, the tool deletes the
edit and says so; `--changes-not-sent-for-review` then commits them with
`changesNotSentForReview=true`, and they wait in the Play Console until they
are sent for review there.

A run stopped before the commit deletes its edit and changes nothing. A run
stopped during the commit may have committed: when the commit gets no answer,
a server error or an interrupt, the tool says the outcome is unknown, and a
read-only run shows what the listing holds before any retry.

The console does the same by hand under the main store listing (Store
presence): phone screenshots, 10-inch tablet screenshots, feature graphic.
The feature graphic goes up there.

### Microsoft Store

Partner Center only. `msstore/submit.py` sends one listing field, What's
New, and holds every other listing field, the screenshots included, to what
the published submission carries; it refuses a change in the image count. So:

1. Load the submission API's credentials: `. scripts/msstore-env.sh`,
   sourced, never executed or piped. With no submission pending
   (`msstore/msstore.py app 9NDCCZH9RB9K` shows none), start a submission in
   Partner Center; under Store listings > English (United Kingdom), remove
   the screenshots and add the new ones in order, each with its caption.
2. Submit it for certification. The next `msstore/submit.py create` clones
   the published submission and so carries the new images forward.

### Snap Store

The snapcraft.io dashboard only: the bibletext listing page, Images — at
most five screenshots and the 3:1 banner. Setting images there leaves
update-metadata-on-release on; editing the summary, the description or the
icon there turns it off for good ([LINUX_STORES.md](LINUX_STORES.md)), so
those fields are never touched in the dashboard.

### AppStream metainfo

The files are committed, the commit reaches GitHub (every push under the
standing rule in [RELEASING.md](RELEASING.md)), and only then does
`screenshots.ref` name it (§4, Linux).

## 7. Known gaps

- **The Mac dark shot needs the account holder.** Shot 02 on the Mac waits
  for the system appearance to be switched and switched back by hand, so a
  Mac set cannot be captured unattended.
- **Windows and Linux show four other scenes.** The runner workflows capture
  a shared-link note, a passage, search results and Settings in the BSB, not
  the eight of §1. Moving them is a change to
  `scripts/capture-windows-screenshots.ps1` and
  `.github/workflows/linux-screenshots.yml`, with new captions in
  `linux/listing.toml` and in Partner Center.
- **The Snap gallery holds five.** Which five of the eight it shows is not
  decided.
- **The Windows PNGs carry an alpha channel.** The runner's
  `System.Drawing.Bitmap` is 32-bit ARGB by default. Partner Center took the
  four, but §5 asks for RGB: creating the bitmap as `Format24bppRgb` in the
  capture script would meet it.
- **`scripts/play-shot-check.py` grabs from emulator-5554 only.** A capture
  AVD on another port needs its `laid_out` check called on a file, as in §4.
- **The Dynamic Island in the 1.2.17 iPhone set.** It shows on 01, 03, 06,
  07 and 08 and not on 02, 04 and 05, in the captures as well as the ready
  copies. The cause is not established; the framing lens now checks for it.
- **The iPad resize grip.** The 1.2.17 iPad set keeps windowed mode's corner
  grip; removing it is the manual Settings change in §4.
- **Android shot 08's menu placement** is not yet written down step by step:
  how the Study with AI menu is brought below verse 1 on Android is to be
  recorded from the first Android set that meets §2.
- **`docs/play-assets/2026-09-1.2.5/` is history.** The 1.2.17 set
  replaced it on the listing on 30 September 2026 (§3), pending Play's
  review. It is never uploaded again.
- **The Play phone set is at the Small display size.** The 1.2.17 phone
  set was taken at 356, not the Pixel 11 Pro's default 420, because at 420
  three screens looked like faults: Books in one column, the Settings sheet
  with Clear on a line of its own and a row cut by the sheet's foot, and the
  Translation sheet over the header. The first two are by design. At 420
  the toolkit lays the phone out about 360 units wide, too narrow for two
  of the Books grid's cells, which are sized so that the longest book name
  fits (`denseGridWrapLayout`, `books_grid.go`), so Books shows one column;
  and Settings starts Clear on a line of its own where the key card is
  narrow (`keyActionsRow`) and scrolls its body, so a row part-shown at the
  foot is the scroll's edge (`sheet_fit.go`). The third was a defect: on a
  phone whose status bar is shallow, a sheet could start partway down the
  header's controls — the Translation sheet in the 2:1 frame, and at the
  phone's own 1080×2410 Settings, the cross-references, a long verse of the
  day and the audio source menu, cutting the title's letters. On the
  Android phone, an iPhone held upright and an iPad, sheets now clear the
  header's controls or cover them; on an iPhone on its side most stay where
  they were, since neither place is open to them without a worse cost
  (`sheet_touch_header.go`; `docs/BACKLOG.md`, "Phones and tablets: a sheet
  started partway down the header's controls"). The next phone set can be
  taken at 420, which is what most people see.
- **Light status-bar icons on Android.** The 1.2.17 light Play images show
  the status bar's icons white on the cream page, so the clock and battery
  barely show. Builds after 1.2.17 ask for dark status-bar and
  navigation-bar icons over the light page (`docs/BACKLOG.md`, "Android: the
  status-bar icons were white on the light page"), so the light images are
  retaken from a release that carries it. Demo mode cannot change the
  icons, and nothing is composed over a capture (§2).
