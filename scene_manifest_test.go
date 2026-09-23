package bibletext

// An app linked against the iOS 27 SDK is refused at launch on iOS 27 unless it
// adopts the UIScene life cycle: UIKit stops it in
// _UIApplicationEvaluateRuntimeIssueForNoSceneLifecycleAdoption before the
// first frame, on every iPhone and iPad running 27. Fyne 2.7.4's iOS driver
// started from the app delegate alone, so every build made with Xcode 27 had
// this defect, and no simulator run could show it — run-ios-sim.sh stamped its
// binary with the iOS 18 SDK, and UIKit keys the requirement on that stamp.
//
// The fix has three parts that only work together:
//
//   1. patches/fyne-2.7.4-ios-scene-lifecycle.patch gives the driver a scene
//      delegate, and takes the scene path when the bundle declares a manifest;
//   2. scripts/ios-scene-manifest.sh writes that manifest into every iOS
//      bundle, and checks the archived and exported App Store bundle for it;
//   3. the simulator build stamps the SDK it was really built with, so a
//      simulator run meets the same UIKit the App Store build does.
//
// A patch without the manifest starts the old way and is refused again; a
// manifest without the patch names a class the binary does not have. These
// tests hold the parts together. The launch itself is exercised by
// scripts/run-ios-sim.sh on an iOS 27 simulator, which is a manual gate: no
// simulator runs in CI.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sceneLifecyclePatch = "patches/fyne-2.7.4-ios-scene-lifecycle.patch"
const sceneManifestScript = "scripts/ios-scene-manifest.sh"

// addedLines returns the lines a unified diff adds, without the leading '+',
// so a check cannot be satisfied by a comment in the removed or context lines.
func addedLines(patch string) string {
	var b strings.Builder
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			b.WriteString(line[1:])
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// methodBody returns the Objective-C method whose signature starts with sig,
// up to the first line that is a lone closing brace.
func methodBody(t *testing.T, src, sig string) string {
	t.Helper()
	i := strings.Index(src, sig)
	if i < 0 {
		t.Fatalf("the patch no longer defines %q", sig)
	}
	rest := src[i:]
	if j := strings.Index(rest, "\n}\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

func TestSceneLifecyclePatchStillDoesWhatItMust(t *testing.T) {
	patch := readRepoFile(t, sceneLifecyclePatch)
	if !strings.Contains(patch, "+++ b/internal/driver/mobile/app/darwin_ios.m") {
		t.Fatalf("%s no longer targets the iOS app delegate", sceneLifecyclePatch)
	}
	src := addedLines(patch)

	// Statements the fix stands on, each a separate claim.
	for claim, want := range map[string]string{
		"defines a scene delegate":                      "@implementation GoAppSceneDelegate",
		"takes the scene path only with a manifest":     `objectForInfoDictionaryKey:@"UIApplicationSceneManifest"] != nil`,
		"makes the window from the scene":               "initWithWindowScene:(UIWindowScene *)scene",
		"puts the one controller in that window":        "[appDelegate attachToWindow:self.window];",
		"hands UIKit the delegate class":                "config.delegateClass = [GoAppSceneDelegate class];",
		"reads orientation from the scene":              "return scene.interfaceOrientation;",
		"lets only the owning scene drive the app":      "- (BOOL)ownsAppWindow",
		"delivers a link that launched the app":         "for (NSUserActivity *activity in connectionOptions.userActivities)",
		"hands a declined web link back to iOS":         "goAppHandBackWhenActive(activity.webpageURL);",
		"opens that link through the system":            "[[UIApplication sharedApplication] openURL:url options:@{} completionHandler:nil];",
		"cannot loop on a link the system returns":      "now - goAppLastHandBackAt < 10",
		"waits for an active scene before handing back": "goAppPendingHandBack = [url retain];",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s no longer %s: %q is not among its added lines", sceneLifecyclePatch, claim, want)
		}
	}

	// Only the app's own window scene is given the delegate, and only such a
	// scene may take the controller: an external display must keep mirroring.
	config := methodBody(t, src, "configurationForConnectingSceneSession:")
	if !strings.Contains(config, "isEqualToString:UIWindowSceneSessionRoleApplication") {
		t.Error("the scene configuration no longer checks the session role; an external display would take the app's only controller")
	}
	connect := methodBody(t, src, "- (void)scene:(UIScene *)scene willConnectToSession:")
	if !strings.Contains(connect, "isEqualToString:UIWindowSceneSessionRoleApplication") {
		t.Error("willConnectToSession no longer checks the session role")
	}

	// The lifecycle stages Fyne receives, each from its own callback, and each
	// only from the scene that owns the app's window.
	for sig, stage := range map[string]string{
		"- (void)sceneDidBecomeActive:":    "lifecycleFocused();",
		"- (void)sceneWillResignActive:":   "lifecycleVisible();",
		"- (void)sceneDidEnterBackground:": "lifecycleAlive();",
		"- (void)sceneDidDisconnect:":      "lifecycleAlive();",
	} {
		body := methodBody(t, src, sig)
		if !strings.Contains(body, stage) {
			t.Errorf("%s no longer sends %s", sig, stage)
		}
		if !strings.Contains(body, "ownsAppWindow") {
			t.Errorf("%s acts for any scene, not only the one that owns the app's window", sig)
		}
	}
	for sig, call := range map[string]string{
		"- (void)scene:(UIScene *)scene continueUserActivity:": "goAppForwardUserActivity(userActivity);",
		"- (void)scene:(UIScene *)scene openURLContexts:":      "goAppForwardURLContexts(URLContexts);",
	} {
		if !strings.Contains(methodBody(t, src, sig), call) {
			t.Errorf("%s no longer forwards the link (%s)", sig, call)
		}
	}
}

// The forwarders call the app delegate's own link methods, which BibleText
// implements in a category; if the two stop meeting, links are dropped
// without a sound.
func TestSceneForwardersReachTheLinkCategory(t *testing.T) {
	src := addedLines(readRepoFile(t, sceneLifecyclePatch))
	category := readRepoFile(t, "share_link_ios.go")
	if !strings.Contains(category, "@implementation GoAppAppDelegate (BibleTextLinks)") {
		t.Fatal("share_link_ios.go no longer extends GoAppAppDelegate with the link methods")
	}
	// The selector is what must match; parameter names are free.
	for _, pair := range []struct {
		forwarder string
		method    *regexp.Regexp
	}{
		{"@selector(application:continueUserActivity:restorationHandler:)",
			regexp.MustCompile(`\(BOOL\)application:\(UIApplication \*\)\w+\s+continueUserActivity:\(NSUserActivity \*\)\w+\s+restorationHandler:`)},
		{"@selector(application:openURL:options:)",
			regexp.MustCompile(`\(BOOL\)application:\(UIApplication \*\)\w+\s+openURL:\(NSURL \*\)\w+\s+options:`)},
	} {
		if !strings.Contains(src, pair.forwarder) {
			t.Errorf("the scene forwarders no longer look for %s", pair.forwarder)
		}
		if !pair.method.MatchString(category) {
			t.Errorf("share_link_ios.go no longer implements the method behind %s", pair.forwarder)
		}
	}
}

// The three iOS delegate patches build on one another, so the setup script must
// apply each of them, in this order: the later two were made against the
// file the earlier ones leave.
func TestTheBuildAppliesTheIOSDelegatePatchesInOrder(t *testing.T) {
	// Comment lines removed, so a commented-out `patch` line does not count.
	script := regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAllString(readRepoFile(t, "scripts/setup-fyne-patch.sh"), "")
	last := -1
	for _, patch := range []string{sceneLifecyclePatch, touchCancelPatch, windowSizePatch} {
		base := filepath.Base(patch)
		var variable string
		for _, line := range strings.Split(script, "\n") {
			if strings.Contains(line, base) && strings.Contains(line, "=") {
				variable = strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
				break
			}
		}
		if variable == "" {
			t.Fatalf("scripts/setup-fyne-patch.sh does not name %s, so an iOS build would ship without it", base)
		}
		apply := `patch -p1 -d "$DEST" < "$` + variable + `"`
		at := strings.Index(script, apply)
		if at < 0 {
			t.Fatalf("%s is assigned to %s but never applied with patch -p1", base, variable)
		}
		if at < last {
			t.Errorf("%s is applied before the patch it was made against", base)
		}
		last = at
	}
}

const touchCancelPatch = "patches/fyne-2.7.4-ios-touch-cancel.patch"
const windowSizePatch = "patches/fyne-2.7.4-ios-window-size.patch"

// UIKit reports a touch the system takes over through touchesCancelled:, which
// Fyne never implemented (it spelled the method touchesCanceled:). The patch
// must end such a touch, and end it where nothing can be tapped.
func TestTouchCancelPatchEndsTheTouchWhereNothingIsTapped(t *testing.T) {
	src := addedLines(readRepoFile(t, touchCancelPatch))
	body := methodBody(t, src, "- (void)touchesCancelled:(NSSet*)touches withEvent:(UIEvent*)event {")
	if !strings.Contains(body, "sendTouch((GoUintptr)touch, (GoUintptr)TOUCH_TYPE_END, -100000, -100000);") {
		t.Error("touchesCancelled: no longer ends each touch off the canvas; a cancel at the finger's position could fire a tap")
	}
	if strings.Contains(body, "sendTouches(") {
		t.Error("touchesCancelled: sends the touches at their own positions, where a cancel can become a tap")
	}
}

// The canvas is the app's window, not the screen: every size report goes
// through goAppReportSize, which measures the view.
func TestWindowSizePatchReportsTheViewNotTheScreen(t *testing.T) {
	patch := readRepoFile(t, windowSizePatch)
	src := addedLines(patch)
	var removed int
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") &&
			strings.Contains(line, "updateConfig((int)size.width, (int)size.height") {
			removed++
		}
	}
	if removed != 6 {
		t.Errorf("the patch replaces %d screen-sized updateConfig calls; the delegate has 6", removed)
	}
	if strings.Contains(src, "nativeBounds.size;\n") && strings.Contains(src, "updateConfig((int)size.width") {
		t.Error("the patch adds back a report of the screen's size")
	}
	report := methodBody(t, src, "static void goAppReportSize(UIView *view) {")
	for claim, want := range map[string]string{
		"measures the view":                          "CGSize pt = view.bounds.size;",
		"gives a full-screen view nativeBounds":      "if (CGSizeEqualToSize(pt, screen.bounds.size)) {",
		"takes those numbers from nativeBounds":      "CGSize native = screen.nativeBounds.size;",
		"counts a smaller window in the same pixels": "lround(pt.width * screen.nativeScale)",
		"takes the window's shape":                   "BOOL wide = w > h;",
		"hands the pair over portrait-first":         "if (wide) {",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("goAppReportSize no longer %s (%q)", claim, want)
		}
	}
	if !strings.Contains(methodBody(t, src, "- (void)viewSafeAreaInsetsDidChange {"), "goAppReportSize(self.view);") {
		t.Error("a safe-area change without a size change is no longer reported")
	}
	// keyboardWillShow: itself is context, not an added line; its new body is.
	if !strings.Contains(src, "CGFloat covered = CGRectGetMaxY(inScreen) - CGRectGetMinY(keyboard);") {
		t.Error("the keyboard reserves its whole height again, even over a window it does not cover")
	}
	layout := methodBody(t, src, "- (void)viewDidLayoutSubviews {")
	if !strings.Contains(layout, "self.reportedSize = self.view.bounds.size;") {
		t.Error("viewDidLayoutSubviews no longer records the size it reported; every layout pass would report again")
	}
	if !strings.Contains(layout, "CGSizeEqualToSize(self.view.bounds.size, self.reportedSize)") {
		t.Error("viewDidLayoutSubviews reports without checking the size changed; a report re-lays the view and would loop")
	}
}

// The manifest and the patch name the same class. The patch also sets that
// class in code, so a mismatch would not blank the app today, but a manifest
// naming a class that does not exist is a trap for the day the code path goes.
func TestSceneManifestNamesThePatchedDelegate(t *testing.T) {
	script := readRepoFile(t, sceneManifestScript)
	m := regexp.MustCompile(`(?m)^SCENE_DELEGATE_CLASS="([A-Za-z_][A-Za-z0-9_]*)"$`).FindStringSubmatch(script)
	if m == nil {
		t.Fatalf("%s no longer sets SCENE_DELEGATE_CLASS", sceneManifestScript)
	}
	class := m[1]
	src := addedLines(readRepoFile(t, sceneLifecyclePatch))
	for _, want := range []string{"@interface " + class + " ", "@implementation " + class} {
		if !strings.Contains(src, want) {
			t.Errorf("the manifest names %s but %s adds no %q", class, sceneLifecyclePatch, want)
		}
	}
	if !strings.Contains(script, `UISceneDelegateClassName\": \"${SCENE_DELEGATE_CLASS}\"`) {
		t.Errorf("%s does not put SCENE_DELEGATE_CLASS into UISceneDelegateClassName", sceneManifestScript)
	}
	// One window only. The app has a single controller, and the patch lets
	// only the scene holding it drive the app; a second scene would be a
	// window with nothing in it.
	if !strings.Contains(script, `\"UIApplicationSupportsMultipleScenes\": false`) {
		t.Errorf("%s no longer declares a single scene (UIApplicationSupportsMultipleScenes false)", sceneManifestScript)
	}
}

// The quick iOS check (and the macOS CI job that runs it) must compile the
// patched toolkit, regenerated from patches/ on every run: go.mod carries no
// replace, so otherwise nothing short of a release build compiles a patch.
func TestTheIOSCheckCompilesThePatchedToolkit(t *testing.T) {
	script := readRepoFile(t, "scripts/check-ios-pane.sh")
	code := regexp.MustCompile(`(?m)^\s*#.*$`).ReplaceAllString(script, "")
	for claim, want := range map[string]string{
		"regenerates the patched toolkit":      "scripts/setup-fyne-patch.sh",
		"builds it in its own directory":       `BIBLETEXT_FYNE_DEST="$PATCHED" scripts/setup-fyne-patch.sh`,
		"points a scratch go.mod at it":        `-replace "fyne.io/fyne/v2=$ROOT/$PATCHED"`,
		"compiles against that scratch go.mod": `compile patched -modfile="$scratch/go.mod"`,
		"still compiles the stock toolkit":     "compile stock",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("scripts/check-ios-pane.sh no longer %s (%q)", claim, want)
		}
	}
	if strings.Contains(code, "go mod edit -replace") {
		t.Error("scripts/check-ios-pane.sh edits the real go.mod; the replace belongs in the scratch copy")
	}
	if !strings.Contains(code, `PATCHED="build/ios-check/fyne"`) {
		t.Error("scripts/check-ios-pane.sh no longer builds into its own directory; it would rewrite third_party/fyne under a running packaging build")
	}
}

// Every script that makes an iOS bundle writes the manifest, as a plain
// command (not disabled by `|| true` or wrapped in a condition), after fyne has
// packaged (fyne writes Info.plist afresh) and before anything is signed (a
// change after signing breaks the signature).
func TestEveryIOSBuildDeclaresTheSceneManifest(t *testing.T) {
	const call = `"${REPO_ROOT}/scripts/ios-scene-manifest.sh" "$APP/Info.plist"`
	for _, rel := range []string{"scripts/release-ios.sh", "scripts/run-ios-device.sh", "scripts/run-ios-sim.sh"} {
		lines := strings.Split(readRepoFile(t, rel), "\n")
		find := func(match func(string) bool) int {
			for i, line := range lines {
				if match(strings.TrimSpace(line)) {
					return i
				}
			}
			return -1
		}
		manifest := find(func(l string) bool { return l == call })
		if manifest < 0 {
			t.Errorf("%s never runs %s as a plain command: its bundle would be refused at launch on iOS 27", rel, call)
			continue
		}
		packaged := find(func(l string) bool {
			return strings.Contains(l, "fyne package") && strings.HasPrefix(l, "(")
		})
		if packaged < 0 || packaged > manifest {
			t.Errorf("%s writes the scene manifest before fyne packages the app, which rewrites Info.plist", rel)
		}
		if signed := find(func(l string) bool { return strings.HasPrefix(l, "codesign ") }); signed >= 0 && signed < manifest {
			t.Errorf("%s signs the app (line %d) before writing the scene manifest (line %d)", rel, signed+1, manifest+1)
		}
	}
}

// The App Store bundle is checked after archiving and again after export, the
// two points where the app actually sent to Apple can be read.
func TestReleaseChecksTheSceneLifeCycleInWhatShips(t *testing.T) {
	script := readRepoFile(t, "scripts/release-ios.sh")
	for _, app := range []string{"$AAPP", "$VAPP"} {
		want := `"${REPO_ROOT}/scripts/ios-scene-manifest.sh" --check "` + app + `" || fail`
		if !strings.Contains(script, want) {
			t.Errorf("scripts/release-ios.sh no longer checks %s for the scene life cycle (%s)", app, want)
		}
	}
}

// UIKit keys behaviour on the SDK a binary says it was built with. A simulator
// binary stamped with an older SDK than the App Store build gets older
// behaviour, and passes where the store build fails.
func TestSimulatorBuildStampsTheSDKItWasBuiltWith(t *testing.T) {
	script := readRepoFile(t, "scripts/run-ios-sim.sh")
	if !regexp.MustCompile(`-set-build-version 7 "\$IOS_MIN" "\$SIM_SDK_VERSION" -replace`).MatchString(script) {
		t.Error("scripts/run-ios-sim.sh no longer stamps \"$SIM_SDK_VERSION\"; a fixed SDK stamp hides SDK-keyed UIKit behaviour")
	}
	assign := regexp.MustCompile(`(?m)^SIM_SDK_VERSION=(.*)$`).FindAllStringSubmatch(script, -1)
	if len(assign) != 1 || assign[0][1] != `"$(xcrun --sdk iphonesimulator --show-sdk-version)"` {
		t.Errorf("SIM_SDK_VERSION must be assigned once, from xcrun --show-sdk-version; found %q", assign)
	}
}

// The materialised toolkit is regenerated by scripts/setup-fyne-patch.sh and is
// not tracked, so this runs only where a build has happened.
func TestPatchedToolkitAdoptsTheSceneLifeCycle(t *testing.T) {
	path := filepath.Join(repoRoot(t), "third_party", "fyne", "internal", "driver", "mobile", "app", "darwin_ios.m")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("third_party/fyne is absent (regenerated by scripts/setup-fyne-patch.sh, not tracked)")
	}
	if !strings.Contains(string(body), "@implementation GoAppSceneDelegate") {
		t.Error("the materialised iOS driver has no scene delegate; rerun scripts/setup-fyne-patch.sh")
	}
}
