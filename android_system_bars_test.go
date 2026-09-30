package bibletext

// THE ANDROID STATUS AND NAVIGATION BARS' ICONS FOLLOW THE PAGE.
//
// The bars have no background of their own: the view hierarchy that would
// paint one never draws in a NativeActivity window, so the canvas's paper
// shows through them. Their icons came from the activity's theme, the
// platform's dark one, and were white on either page — 1.19:1 against the
// light paper on the Android 16 emulator. The Go side now sends the icons the
// page needs through the window-chrome seam the Windows title bar uses
// (followTitleBar), once at startup and again whenever a rebuild moves the
// variant, and BtBridge keeps the answer for a recreated activity. Java and
// the android-tagged Go cannot run here, so their half is held at the source.

import (
	"image/color"
	"math"
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

// over composites c, with its alpha, onto an opaque page.
func over(c, page color.NRGBA) color.NRGBA {
	a := float64(c.A) / 255
	mix := func(f, b uint8) uint8 { return uint8(float64(f)*a + float64(b)*(1-a) + 0.5) }
	return color.NRGBA{R: mix(c.R, page.R), G: mix(c.G, page.G), B: mix(c.B, page.B), A: 255}
}

// The icons SystemUI draws: white by default, and 60% black in the light
// appearance (measured on the emulator as #605E5A over the light paper, which
// is 60% black over it).
var (
	systemBarIconsLight = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	systemBarIconsDark  = color.NRGBA{A: 0x99}
)

// THE ICONS ASKED FOR READ ON THE PAGE THEY ARE DRAWN OVER, in every variant,
// "no preference" included (the light page, as isDark reads it). Mutations
// guarded: a choice that ignores the variant (the white the theme gave, on
// the light page), and the choice inverted.
func TestAndroidSystemBarIconsReadOnThePage(t *testing.T) {
	for _, v := range []fyne.ThemeVariant{light, dark, 2 /* no preference */} {
		page := paletteFor(v).Background
		whiteOn := contrastRatio(systemBarIconsLight, page)
		darkOn := contrastRatio(over(systemBarIconsDark, page), page)
		// CONTROL: on each page one of the two is illegible, so the choice
		// is what makes the bar readable, not the palette alone.
		if math.Min(whiteOn, darkOn) >= 3 {
			t.Fatalf("control: variant %v: both icon colours read on the page (%.2f:1, %.2f:1), so "+
				"this test cannot tell a right choice from a wrong one", v, whiteOn, darkOn)
		}
		chosen, name := whiteOn, "white"
		if systemBarsLight(v) {
			chosen, name = darkOn, "dark"
		}
		if chosen < 4.5 {
			t.Errorf("variant %v: the %s icons asked for stand at %.2f:1 on the page, want at least 4.5:1",
				v, name, chosen)
		}
	}
}

// recordChrome replaces the chrome seam for one test and returns what it was
// sent.
func recordChrome(t *testing.T, h *appearanceHarness) *[]fyne.ThemeVariant {
	t.Helper()
	var sent []fyne.ThemeVariant
	prev := syncTitleBar
	syncTitleBar = func(w fyne.Window, v fyne.ThemeVariant) {
		if w != h.state.window {
			t.Error("the chrome was synced on another window")
		}
		sent = append(sent, v)
	}
	t.Cleanup(func() { syncTitleBar = prev })
	return &sent
}

// THE BARS ARE SENT AT STARTUP, NOT ONLY AFTER A SWITCH. Nothing on Android
// sets them when the window is made, so a reader who never changes the
// system's appearance would otherwise read the light page under white icons
// for good. On Windows the frame is made in the content's variant and nothing
// is sent, as before. Mutations guarded: the chrome seeded with the content's
// variant on Android (nothing sent on a light page), the startup send left
// out, and a rebuild in the same variant sending again.
func TestAndroidSystemBarsAreSentAtStartup(t *testing.T) {
	t.Run("android, light page", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		sent := recordChrome(t, h)
		h.variant = light
		seedAppearance(h.state, "android")
		if got := *sent; len(got) != 1 || got[0] != light {
			t.Fatalf("startup on the light page sent %v, want [light]: the bars start with the "+
				"dark theme's white icons", got)
		}
		rebuildWindow(h.state) // a tab switch, a download landing
		if len(*sent) != 1 {
			t.Errorf("a rebuild in the same variant sent the bars again: %v", *sent)
		}
		h.flip()
		h.flip()
		if got := *sent; len(got) != 3 || got[1] != dark || got[2] != light {
			t.Errorf("two switches while the app runs sent %v, want [light dark light]", got)
		}
	})
	t.Run("android, dark page", func(t *testing.T) {
		h := newAppearanceHarness(t, true)
		sent := recordChrome(t, h)
		h.variant = dark
		seedAppearance(h.state, "android")
		if len(*sent) != 0 {
			t.Errorf("startup on the dark page sent %v; the bars already have light icons", *sent)
		}
		h.flip()
		if got := *sent; len(got) != 1 || got[0] != light {
			t.Errorf("a switch to light sent %v, want [light]", got)
		}
	})
	for _, goos := range []string{"windows", "darwin", "linux", "ios"} {
		t.Run(goos, func(t *testing.T) {
			h := newAppearanceHarness(t, goos == "ios")
			sent := recordChrome(t, h)
			h.variant = light
			seedAppearance(h.state, goos)
			if len(*sent) != 0 {
				t.Errorf("startup sent %v; this platform's chrome starts in the content's variant", *sent)
			}
		})
	}
}

// javaBlockAfter returns the brace block that follows the first occurrence of
// head in src, head included.
func javaBlockAfter(t *testing.T, src, head string) string {
	t.Helper()
	i := strings.Index(src, head)
	if i < 0 {
		t.Fatalf("cannot find %q", head)
	}
	open := strings.Index(src[i:], "{")
	return src[i:i+open] + braceBlock(t, src[i+open:])
}

// THE BRIDGE ASKS FOR THE APPEARANCE, ON EVERY API THAT HAS ONE, AND GIVES IT
// TO A RECREATED ACTIVITY. The Go side sends only when the variant moves; a
// rotation or a relaunch with the process alive hands the app a new window in
// the theme's appearance, and on the emulator a control build without the
// re-apply in init went back to white icons (1.19:1) after the activity was
// recreated. Mutations guarded: the re-apply in init removed (checked on a
// copy below), the store left out of setSystemBarsLight, either API path
// missing, the navigation bar left out, and the navigation flag used below the
// API that has it.
func TestAndroidSystemBarsAreAskedForAndKeptForANewActivity(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")

	set := javaBlockAfter(t, java, "public static void setSystemBarsLight(final boolean light)")
	if !inSequence(set, "systemBarsLight = light ? 1 : 0;", "applySystemBars(activity);") {
		t.Errorf("setSystemBarsLight must keep the answer, then apply it to the live activity:\n%s", set)
	}

	apply := javaBlockAfter(t, java, "private static void applySystemBars(Activity act)")
	for _, want := range []string{
		"if (act == null || systemBarsLight < 0) return;",
		"if (Build.VERSION.SDK_INT >= 30) {",
		"int mask = android.view.WindowInsetsController.APPEARANCE_LIGHT_STATUS_BARS",
		"| android.view.WindowInsetsController.APPEARANCE_LIGHT_NAVIGATION_BARS;",
		"c.setSystemBarsAppearance(light ? mask : 0, mask);",
		"} else if (Build.VERSION.SDK_INT >= 23) {",
		"int mask = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;",
		"if (Build.VERSION.SDK_INT >= 26) mask |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;",
		"decor.setSystemUiVisibility(light ? (flags | mask) : (flags & ~mask));",
	} {
		if !statementLine(t, want).MatchString(apply) {
			t.Errorf("applySystemBars has no line %q:\n%s", want, apply)
		}
	}

	// The re-apply for a new window rides init, beside the other per-window
	// request, and before the bridge's own view is built.
	initBody := javaBlockAfter(t, java, "public static void init(final Activity act)")
	reapplied := func(body string) bool {
		at := statementLine(t, "extendIntoTheCutout(act);").FindStringIndex(body)
		again := statementLine(t, "applySystemBars(act);").FindStringIndex(body)
		return at != nil && again != nil && again[0] > at[1]
	}
	if !reapplied(initBody) {
		t.Errorf("init does not give a new activity's window the bars' appearance:\n%s", initBody)
	}
	// CONTROL: the same check on init without the line fails.
	if reapplied(strings.Replace(initBody, "applySystemBars(act);", "// applySystemBars(act);", 1)) {
		t.Fatal("control: the check passes an init that no longer re-applies the appearance")
	}

	// The Go half: Android's chrome is the bars, sent through the bridge,
	// and the no-op file no longer claims Android.
	android := readSourceFile(t, "title_bar_android.go")
	if !strings.HasPrefix(android, "//go:build android\n") {
		t.Error("title_bar_android.go must be built for Android alone")
	}
	if body := funcBody(t, android, "syncNativeTitleBar"); !strings.Contains(body, "setAndroidSystemBarsLight(systemBarsLight(v))") {
		t.Errorf("Android's syncNativeTitleBar does not send the bars' appearance:\n%s", body)
	}
	if other := readSourceFile(t, "title_bar_other.go"); !strings.HasPrefix(other, "//go:build !windows && !android\n") {
		t.Error("title_bar_other.go must leave Android to title_bar_android.go")
	}
	bridge := readSourceFile(t, "reading_android.go")
	if body := funcBody(t, bridge, "setAndroidSystemBarsLight"); !strings.Contains(body, "C.btaSetSystemBarsLight(") {
		t.Errorf("setAndroidSystemBarsLight does not call the bridge:\n%s", body)
	}
	// A lookup the guard does not check would reach CallStaticVoidMethod as
	// NULL when the dex and the descriptor disagree.
	guard := regexp.MustCompile(`(?s)if \(\(\*env\)->ExceptionCheck\(env\) \|\|.*?\) \{`).FindString(bridge)
	if !strings.Contains(guard, "btaSetSystemBarsLightM == NULL") {
		t.Error("btaEnsureClass's guard does not check the setSystemBarsLight lookup")
	}
}
