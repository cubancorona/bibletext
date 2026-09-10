package bibletext

// The zone refresh itself is behind //go:build ios || android, so no host test
// can call it. What the host CAN hold is the decision it delegates
// (localZoneFor, pure) and the shape the invisible files have to keep — the
// idiom overlay_recovery_shape_test.go and cache_path_android_guard_test.go
// already use for the other platform-only paths.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Mutation guarded: answering a real name with a FixedZone from the offset
// (fyne's launch-time stopgap) instead of loading it. A FixedZone has one
// offset for the whole year; Europe/London has two, and this holds an instant
// on each side of the March change apart.
func TestLocalZoneForLoadsTheNamedZoneWithItsTransitions(t *testing.T) {
	loc := localZoneFor("Europe/London", "GMT", 0)
	if loc.String() != "Europe/London" {
		t.Fatalf("localZoneFor answered %q for Europe/London", loc)
	}
	_, winter := time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	_, summer := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if winter == summer {
		t.Fatalf("February and July carry the same offset (%d) — the zone has no "+
			"transitions, so the day boundary would sit an hour off all summer", winter)
	}
	if winter != 0 || summer != 3600 {
		t.Errorf("Europe/London offsets: winter %d, summer %d; want 0 and 3600", winter, summer)
	}
}

// Mutation guarded: returning UTC (or the loader's nil) when the name cannot
// be loaded. The fallback must carry the C library's offset, or a device with
// a zone the embedded database lacks would roll the day at 00:00 UTC.
func TestLocalZoneForFallsBackToTheOffsetForAnUnknownName(t *testing.T) {
	// A different pair from the other fallback tests, so a fallback that
	// returned a hard-coded zone could not pass all three.
	loc := localZoneFor("Mars/Olympus_Mons", "PST", -28800)
	name, offset := time.Now().In(loc).Zone()
	if name != "PST" || offset != -28800 {
		t.Fatalf("unknown name answered %s/%d; want the fallback PST/-28800", name, offset)
	}
}

// Mutation guarded: dropping the empty-name guard. LoadLocation("") succeeds
// with UTC, so a bridge that is absent (the plain fyne package build, or a
// Java exception) would silently reinstate the placeholder zone the refresh
// exists to replace.
func TestLocalZoneForFallsBackToTheOffsetForAnEmptyName(t *testing.T) {
	loc := localZoneFor("", "BST", 3600)
	name, offset := time.Now().In(loc).Zone()
	if name != "BST" || offset != 3600 {
		t.Fatalf("empty name answered %s/%d; want the fallback BST/3600", name, offset)
	}
}

// Mutation guarded: dropping "Local" from the guard. LoadLocation("Local")
// returns the current time.Local — on iOS the placeholder UTC — so the refresh
// would hand back exactly the value it was asked to replace. Modelled here by
// parking time.Local at UTC for the call.
func TestLocalZoneForNeverAnswersWithTheZoneBeingReplaced(t *testing.T) {
	saved := time.Local
	time.Local = time.UTC
	defer func() { time.Local = saved }()

	loc := localZoneFor("Local", "BST", 3600)
	name, offset := time.Now().In(loc).Zone()
	if name != "BST" || offset != 3600 {
		t.Fatalf("\"Local\" answered %s/%d; want the fallback BST/3600", name, offset)
	}
}

// liveLine is the index of the line in src that IS stmt once trimmed — no
// comment marker, nothing else on it — or -1. A substring search counts a
// statement that has been commented out; this does not.
func liveLine(src, stmt string) int {
	at := 0
	for _, line := range strings.SplitAfter(src, "\n") {
		if strings.TrimSpace(line) == stmt {
			return at
		}
		at += len(line)
	}
	return -1
}

func readSourceForShape(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s is gone: %v", path, err)
	}
	return string(src)
}

// Mutation guarded: the refresh dropped from either call site, or moved from
// the startup path onto the goroutine (where the UI goroutine could compute a
// date first), or moved into the overlay-recovery functions, which run only on
// the platforms and conditions those care about.
func TestTheZoneRefreshIsWiredAtStartupAndOnEveryForeground(t *testing.T) {
	app := readSourceForShape(t, "app.go")

	start := strings.Index(app, "func StartBackgroundLoad(")
	if start < 0 {
		t.Fatal("StartBackgroundLoad is gone")
	}
	body := app[start:]
	refresh := liveLine(body, "refreshLocalTimeZone()")
	spawn := strings.Index(body, "go func() {")
	if refresh < 0 {
		t.Fatal("StartBackgroundLoad no longer refreshes the zone; a phone's first " +
			"day number would be computed against UTC")
	}
	if spawn < 0 || refresh > spawn {
		t.Errorf("the startup refresh (at %d) runs on or after the load goroutine (at %d) "+
			"rather than before it", refresh, spawn)
	}

	hook := strings.Index(app, "lc.SetOnEnteredForeground(func() {")
	if hook < 0 {
		t.Fatal("the foreground lifecycle hook is gone")
	}
	hookBody := app[hook:]
	if end := strings.Index(hookBody, "\n\t})"); end > 0 {
		hookBody = hookBody[:end]
	}
	if liveLine(hookBody, "refreshLocalTimeZone()") < 0 {
		t.Error("the foreground hook no longer refreshes the zone, so a clock change " +
			"while the app was in the background lands only at the next launch")
	}

	// It belongs beside the recovery, not inside it: the Android recovery
	// stands down unless the activity was recreated, and the iOS one unless
	// the pane emptied — neither has anything to do with the clock.
	for _, f := range []string{"overlay_recovery_ios.go", "reading_android.go"} {
		if strings.Contains(readSourceForShape(t, f), "refreshLocalTimeZone(") {
			t.Errorf("%s calls the zone refresh; it belongs in app.go's hook, where it "+
				"runs on every foreground unconditionally", f)
		}
	}

	// CONTROL: the sweep must be able to fail.
	if strings.Contains(app, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}

// Mutation guarded: the mobile refresh reading the zone through anything but
// localZoneFor (say, a FixedZone directly), assigning it anywhere but
// time.Local, or dropping the embedded database — without which LoadLocation
// fails on every iPhone and the name path is dead code there.
func TestTheMobileRefreshIsAThinWrapperAroundLocalZoneFor(t *testing.T) {
	src := readSourceForShape(t, "timezone_mobile.go")
	head := src
	if i := strings.Index(head, "package "); i > 0 {
		head = head[:i]
	}
	if liveLine(head, "//go:build ios || android") < 0 {
		t.Errorf("timezone_mobile.go's build tag is not `ios || android`: %q", strings.TrimSpace(head))
	}
	for _, want := range []string{
		`_ "time/tzdata"`,
		"time.Local = localZoneFor(deviceTimeZoneName(),",
		"tzset();",      // the C library re-reads a changed zone
		"tm.tm_gmtoff;", // the fallback offset comes from localtime_r
	} {
		if !strings.Contains(src, want) {
			t.Errorf("timezone_mobile.go no longer contains %q", want)
		}
	}
	if strings.Contains(src, "time.FixedZone(") {
		t.Error("timezone_mobile.go builds a FixedZone itself; the choice belongs to " +
			"localZoneFor, which is the half the host suite can hold")
	}
	// The C library caches the zone it first read; tzset before localtime_r is
	// what makes a zone changed in the phone's settings visible to the fallback.
	if ts, lr := liveLine(src, "tzset();"), strings.Index(src, "localtime_r("); ts < 0 || lr < 0 || ts > lr {
		t.Error("timezone_mobile.go's fallback does not run tzset() before localtime_r(), so a " +
			"changed zone would be read from the library's stale cache")
	}

	// CONTROL.
	if strings.Contains(src, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}

// Mutation guarded: a second refreshLocalTimeZone compiling for a phone (the
// stub's tag widened), or the stub gaining a body — the desktop platforms read
// the system zone themselves and must stay untouched.
func TestExactlyOneZoneRefreshCompilesPerPlatform(t *testing.T) {
	stub := readSourceForShape(t, "timezone_other.go")
	head := stub
	if i := strings.Index(head, "package "); i > 0 {
		head = head[:i]
	}
	if liveLine(head, "//go:build !ios && !android") < 0 {
		t.Errorf("timezone_other.go's build tag is %q; it must exclude exactly the two "+
			"platforms timezone_mobile.go claims", strings.TrimSpace(head))
	}
	if !strings.Contains(stub, "func refreshLocalTimeZone() {}") {
		t.Error("timezone_other.go no longer defines the no-op refresh, so the tag " +
			"assertion above is not guarding what it claims to")
	}
}

// Mutation guarded: the iOS name read losing resetSystemTimeZone (Foundation
// would keep answering the zone cached at launch, and the foreground refresh
// would refresh nothing), and the Android read going around the bridge or the
// bridge losing the lookup (the descriptor itself is held against the Java by
// TestJNIDescriptorsMatchJava).
func TestEachPhoneReadsItsZoneNameFromThePlatform(t *testing.T) {
	ios := readSourceForShape(t, "timezone_ios.go")
	for _, want := range []string{
		"[NSTimeZone resetSystemTimeZone];",
		"[NSTimeZone localTimeZone].name",
		"func deviceTimeZoneName() string {",
	} {
		if !strings.Contains(ios, want) {
			t.Errorf("timezone_ios.go no longer contains %q", want)
		}
	}
	reset := liveLine(ios, "[NSTimeZone resetSystemTimeZone];")
	if reset < 0 {
		t.Error("timezone_ios.go no longer resets the cached system zone as a live statement")
	} else if reset > strings.Index(ios, "localTimeZone].name") {
		t.Error("timezone_ios.go reads the name BEFORE resetting the cached system zone")
	}

	android := readSourceForShape(t, "reading_android.go")
	for _, want := range []string{
		`"timeZoneID", "()Ljava/lang/String;"`,
		"btaTimeZoneIDM == NULL", // a skewed descriptor drops the whole bridge, as for every other method
		"func deviceTimeZoneName() string {",
		"C.btaTimeZoneID(C.uintptr_t(env),",
	} {
		if !strings.Contains(android, want) {
			t.Errorf("reading_android.go no longer contains %q", want)
		}
	}
	java := readSourceForShape(t, "android/BtBridge.java")
	if !strings.Contains(java, "public static String timeZoneID() { return java.util.TimeZone.getDefault().getID(); }") {
		t.Error("BtBridge.timeZoneID no longer returns TimeZone.getDefault().getID()")
	}

	// CONTROL.
	if strings.Contains(ios+android+java, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}
