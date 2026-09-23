#!/usr/bin/env bash
# Declare, or check, the UIScene life cycle in an iOS app bundle.
#
#   scripts/ios-scene-manifest.sh <path/to/BibleText.app/Info.plist>
#   scripts/ios-scene-manifest.sh --check <path/to/BibleText.app>
#
# An app linked against the iOS 27 SDK is refused at launch on iOS 27 unless it
# adopts the scene life cycle ("UIScene life cycle is required for apps built
# with this SDK"). The patched Fyne delegate
# (patches/fyne-2.7.4-ios-scene-lifecycle.patch) takes the scene path when the
# bundle declares this manifest and the old path when it does not, so every iOS
# bundle — App Store, device and simulator — must carry it.
#
# The first form writes the manifest. Fyne's packager writes Info.plist afresh
# on every run, so each build script calls it after packaging and before
# signing. The second form checks a finished bundle: the manifest names the
# scene delegate, and the executable carries that class, so a bundle built
# without the patch or stripped of the manifest fails before it is shipped.
#
# The class named here is the scene delegate the patch defines. The patch also
# sets that class in code for the application role, so the two are redundant
# on purpose; scene_manifest_test.go holds them equal so that neither can drift
# into naming something that does not exist.
set -euo pipefail

SCENE_DELEGATE_CLASS="GoAppSceneDelegate"
KEY=':UIApplicationSceneManifest:UISceneConfigurations:UIWindowSceneSessionRoleApplication:0:UISceneDelegateClassName'

declared_class() {
  /usr/libexec/PlistBuddy -c "Print $KEY" "$1" 2>/dev/null || true
}

if [ "${1:-}" = "--check" ]; then
  APP="${2:?usage: ios-scene-manifest.sh --check <App.app>}"
  [ -f "$APP/Info.plist" ] || { echo "ios-scene-manifest: no Info.plist in $APP" >&2; exit 1; }
  got="$(declared_class "$APP/Info.plist")"
  if [ "$got" != "$SCENE_DELEGATE_CLASS" ]; then
    echo "ios-scene-manifest: $APP declares no scene delegate (read '${got}'); iOS 27 refuses it at launch" >&2
    exit 1
  fi
  exe="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Info.plist")"
  if ! LC_ALL=C grep -a -q "$SCENE_DELEGATE_CLASS" "$APP/$exe"; then
    echo "ios-scene-manifest: $APP/$exe has no $SCENE_DELEGATE_CLASS class; the Fyne scene patch did not reach this build" >&2
    exit 1
  fi
  exit 0
fi

PLIST="${1:?usage: ios-scene-manifest.sh <Info.plist> | --check <App.app>}"
[ -f "$PLIST" ] || { echo "ios-scene-manifest: no such file: $PLIST" >&2; exit 1; }

plutil -replace UIApplicationSceneManifest -json "{
  \"UIApplicationSupportsMultipleScenes\": false,
  \"UISceneConfigurations\": {
    \"UIWindowSceneSessionRoleApplication\": [
      {
        \"UISceneConfigurationName\": \"Default Configuration\",
        \"UISceneDelegateClassName\": \"${SCENE_DELEGATE_CLASS}\"
      }
    ]
  }
}" "$PLIST"

# Read it back: a manifest that did not land is a launch failure on iOS 27.
got="$(declared_class "$PLIST")"
if [ "$got" != "$SCENE_DELEGATE_CLASS" ]; then
  echo "ios-scene-manifest: the scene manifest did not land in $PLIST (read back '${got}')" >&2
  exit 1
fi
