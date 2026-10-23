package bibletext

// Each platform reports its window before the canvas has a size, and the iOS
// read never holds up the layout (launch_window.go).
//
// device_ios.go, device_android.go and the bridge are compiled only for their
// platforms, so no host test runs them, and the simulator shows their effect
// only on a launch made in the shape the device answer gets wrong. These
// checks hold their source, and the Go that consults them, to what the first
// frame of such a launch stands on; launch_window_test.go has the behaviour.
// Kept free of the symbols they check, so they compile, and fail, against a
// tree without them.

import (
	"strings"
	"testing"
)

func TestTheLayoutNeverWaitsForTheWindowScene(t *testing.T) {
	src := readNativeSource(t, "device_ios.go")
	const sig = "static BTWindowReport bibleTextWindowReport(void) {"
	if !strings.Contains(src, sig) {
		t.Fatalf("device_ios.go has no %q: nothing reads the window scene before the canvas has a size", sig)
	}
	read := nativeFunctionSource(t, "device_ios.go", sig)
	for claim, want := range map[string]string{
		"looks among the app's connected scenes":      "app.connectedScenes",
		"takes only a window scene":                   "isKindOfClass:[UIWindowScene class]",
		"takes the app's own scene, not a display's":  "isEqualToString:UIWindowSceneSessionRoleApplication",
		"takes the scene's size":                      "bounds = windowScene.effectiveGeometry.coordinateSpace.bounds;",
		"takes it before iOS 26 too":                  "bounds = windowScene.coordinateSpace.bounds;",
		"waits for the app's own window":              "window.rootViewController == nil",
		"reports the width":                           "out.width = bounds.size.width;",
		"reports the height":                          "out.height = bounds.size.height;",
		"reports the left inset":                      "out.insetLeft = window.safeAreaInsets.left;",
		"reports the right inset":                     "out.insetRight = window.safeAreaInsets.right;",
		"says the report is ready":                    "out.ready = 1;",
		"reads inline when already on the main queue": "if ([NSThread isMainThread]) {\n        read();",
		"reads on the main queue from any other":      "dispatch_sync(dispatch_get_main_queue(), read);",
	} {
		if !strings.Contains(read, want) {
			t.Errorf("bibleTextWindowReport no longer %s (%q)", claim, want)
		}
	}

	// The read that waits for the main thread is made in one place, the
	// platform's read that keptWindowReport makes on a goroutine of its own.
	// The functions the layout and the push ask call into C for nothing.
	goSrc := src[strings.Index(src, `import "C"`):]
	if n := strings.Count(goSrc, "C.bibleText") - strings.Count(goSrc, "C.bibleTextIsPad()"); n != 1 {
		t.Errorf("device_ios.go reads the window scene from %d places in Go, want 1 (windowReportRead)", n)
	}
	if init := nativeFunctionSource(t, "device_ios.go", "func init() {"); !strings.Contains(init, "windowReportRead = func() (windowReport, bool) {") ||
		!strings.Contains(init, "C.bibleTextWindowReport()") {
		t.Error("device_ios.go's init does not register the window scene's read as windowReportRead")
	}
	for _, sig := range []string{
		"func launchWindowSize() (w, h float32, ok bool) {",
		"func launchPaneWidth() float64 {",
	} {
		if !strings.Contains(src, sig) {
			t.Errorf("device_ios.go has no %q", sig)
			continue
		}
		body := nativeFunctionSource(t, "device_ios.go", sig)
		if strings.Contains(body, "C.") {
			t.Errorf("%s calls into C; the layout asks it, and must not wait for the main thread", sig)
		}
		if !strings.Contains(body, "keptWindowReport()") {
			t.Errorf("%s does not answer from the kept report", sig)
		}
	}

	// The layout's decisions take the reported size while the canvas is
	// unsized; the landing asks the layout watcher again; iOS's first push
	// takes the reported pane width.
	for _, tc := range []struct{ path, sig, want string }{
		{"layout.go", "func railForWindow(state *AppState) bool {", "layoutSize(w, h)"},
		{"phone_landscape.go", "func phoneLandscapeReadingActive() bool {", "layoutSize(w, h)"},
		{"ui_compact.go", "func tabBarStyleFor(state *AppState) tabBarStyle {", "layoutSize(w, h)"},
		{"launch_window.go", "func layoutSize(w, h float32) (float32, float32) {", "heldLaunchSize()"},
		{"launch_window.go", "func heldLaunchSize() (float32, float32) {", "launchSize()"},
		{"launch_window.go", "func keepWindowReport(r windowReport) {", "releaseLaunchSize()"},
		{"ui_mobile.go", "func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {", "releaseLaunchSize()"},
		{"ui_regular.go", "func newLayoutWatcher(state *AppState, content fyne.CanvasObject) *layoutWatcher {", "liveLayoutWatcher = w"},
		{"ui_regular.go", "func relayoutUnsized() {", "canvasSized(sz.Width, sz.Height)"},
		{"ui_regular.go", "func relayoutUnsized() {", "layoutWatcherNeedsRebuild(w.built, w.state.renderedLayout("},
		{"ui_regular.go", "func relayoutUnsized() {", "rebuildForLayoutChange(w.state)"},
		{"reporter_ios.go", "func init() {", "readingColdWidth = launchPaneWidth"},
		{"reading_page_width.go", "func widestWindowWidth() float64 {", "canvasSized(sz.Width, sz.Height)"},
	} {
		if !strings.Contains(readNativeSource(t, tc.path), tc.sig) {
			t.Errorf("%s has no %q", tc.path, tc.sig)
			continue
		}
		if !strings.Contains(nativeFunctionSource(t, tc.path, tc.sig), tc.want) {
			t.Errorf("%s: %s does not say %q", tc.path, tc.sig, tc.want)
		}
	}
	// The release comes before the build's first decision.
	if body := nativeFunctionSource(t, "ui_mobile.go", "func CreateMainUI(app fyne.App, state *AppState, window fyne.Window) fyne.CanvasObject {"); strings.Index(body, "releaseLaunchSize()") > strings.Index(body, "state.readingFullScreen()") {
		t.Error("CreateMainUI decides its layout before it lets the report be read afresh")
	}
	if !strings.Contains(readNativeSource(t, "ui_regular.go"), "func init() { launchReportRelayout = relayoutUnsized }") {
		t.Error("ui_regular.go does not hand the landing to relayoutUnsized")
	}
}

// Android reports its activity's configured size, over the bridge, and its
// tablet idiom takes it while the canvas is unsized; the desktop reports none.
func TestAndroidReportsItsConfiguredWindow(t *testing.T) {
	android := readNativeSource(t, "device_android.go")
	const sig = "func launchWindowSize() (w, h float32, ok bool) {"
	if !strings.Contains(android, sig) {
		t.Fatalf("device_android.go has no %q", sig)
	}
	size := nativeFunctionSource(t, "device_android.go", sig)
	for _, want := range []string{"androidWindowWidthDp()", "androidWindowHeightDp()"} {
		if !strings.Contains(size, want) {
			t.Errorf("Android's launchWindowSize does not read %s", want)
		}
	}
	if !strings.Contains(nativeFunctionSource(t, "device_android.go", "func deviceIsTablet() bool {"), "isTabletDimensions(layoutSize(sz.Width, sz.Height))") {
		t.Error("Android's tablet idiom does not take the reported size while the canvas is unsized")
	}

	bridge := readNativeSource(t, "reading_android.go")
	for _, want := range []string{
		`btaWindowHeightM = (*env)->GetStaticMethodID(env, btaClass, "windowHeightDp", "()F");`,
		"btaWindowHeightM == NULL",
		"(*env)->CallStaticFloatMethod(env, btaClass, btaWindowHeightM);",
	} {
		if !strings.Contains(bridge, want) {
			t.Errorf("reading_android.go does not say %q", want)
		}
	}
	java := readNativeSource(t, "android/BtBridge.java")
	if !strings.Contains(java, "public static float windowHeightDp() {") ||
		!strings.Contains(java, "activity.getResources().getConfiguration().screenHeightDp") {
		t.Error("BtBridge has no windowHeightDp reading the configuration's screenHeightDp")
	}

	if !strings.Contains(readNativeSource(t, "device_other.go"), "func launchWindowSize() (w, h float32, ok bool) { return 0, 0, false }") {
		t.Error("device_other.go reports a window before its first layout; a desktop window has a size from it")
	}
}
