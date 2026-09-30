package bibletext

// THE SYSTEM BARS' ON/OFF VALUE KEEPS ITS SENSE ACROSS THE BRIDGE.
//
// systemBarsLight(v) answers true for the light page, meaning "dark icons",
// and android_system_bars_test.go holds that answer against the palette. On
// its way to SystemUI the answer is rewritten three times: the Go wrapper
// turns the bool into a C int, the C shim turns that into a jboolean, and
// BtBridge keeps it as an int it reads back as a boolean before choosing the
// flags. Any one of those written the wrong way round brings back white icons
// on the light page, and puts dark ones on the dark page too, while every
// test of the Go helper still passes. Java and the android-tagged Go cannot
// run here, so each rewrite is held at the source, as a statement line of its
// own, in the order the value passes through it.

import (
	"strings"
	"testing"
)

// statementsInOrder reports whether every statement is a line of its own in
// body (statementLine), each after the end of the one before it.
func statementsInOrder(t *testing.T, body string, stmts ...string) bool {
	t.Helper()
	pos := 0
	for _, s := range stmts {
		loc := statementLine(t, s).FindStringIndex(body[pos:])
		if loc == nil {
			return false
		}
		pos += loc[1]
	}
	return true
}

// Mutations guarded, each shown failing on a flipped copy below: the Go
// wrapper's test inverted or its default changed; the C shim's jboolean
// swapped; the Java store inverted; and the Java read-back compared against
// the other value.
func TestAndroidSystemBarsValueKeepsItsSenseAcrossTheBridge(t *testing.T) {
	bridge := readSourceFile(t, "reading_android.go")
	java := readNativeSource(t, "android/BtBridge.java")

	hops := []struct {
		name  string
		body  string
		stmts []string
		flips [][2]string // each a replacement that turns the value round
	}{
		{
			name: "Go wrapper setAndroidSystemBarsLight: true is 1",
			body: funcBody(t, bridge, "setAndroidSystemBarsLight"),
			stmts: []string{
				"on := C.int(0)",
				"if light {",
				"on = 1",
				"runBta(func(env uintptr) { C.btaSetSystemBarsLight(C.uintptr_t(env), on) })",
			},
			flips: [][2]string{
				{"if light {", "if !light {"},
				{"on := C.int(0)", "on := C.int(1)"},
				{"on = 1", "on = 0"},
			},
		},
		{
			name: "C shim btaSetSystemBarsLight: 1 is JNI_TRUE",
			body: cFuncBody(t, bridge, "static void btaSetSystemBarsLight(uintptr_t jni_env, int light) {"),
			stmts: []string{
				"static void btaSetSystemBarsLight(uintptr_t jni_env, int light) {",
				"(*env)->CallStaticVoidMethod(env, btaClass, btaSetSystemBarsLightM, light ? JNI_TRUE : JNI_FALSE);",
			},
			flips: [][2]string{
				{"light ? JNI_TRUE : JNI_FALSE", "light ? JNI_FALSE : JNI_TRUE"},
				{"light ? JNI_TRUE : JNI_FALSE", "!light ? JNI_TRUE : JNI_FALSE"},
			},
		},
		{
			name: "Java setSystemBarsLight: true is stored as 1",
			body: javaBlockAfter(t, java, "public static void setSystemBarsLight(final boolean light)"),
			stmts: []string{
				"systemBarsLight = light ? 1 : 0;",
				"applySystemBars(activity);",
			},
			flips: [][2]string{
				{"systemBarsLight = light ? 1 : 0;", "systemBarsLight = light ? 0 : 1;"},
			},
		},
		{
			name: "Java applySystemBars: 1 is read back as light, and light sets the flags",
			body: javaBlockAfter(t, java, "private static void applySystemBars(Activity act)"),
			stmts: []string{
				"final boolean light = systemBarsLight == 1;",
				"c.setSystemBarsAppearance(light ? mask : 0, mask);",
				"decor.setSystemUiVisibility(light ? (flags | mask) : (flags & ~mask));",
			},
			flips: [][2]string{
				{"systemBarsLight == 1;", "systemBarsLight == 0;"},
				{"systemBarsLight == 1;", "systemBarsLight != 1;"},
				{"light ? mask : 0, mask", "light ? 0 : mask, mask"},
				{"light ? (flags | mask) : (flags & ~mask)", "light ? (flags & ~mask) : (flags | mask)"},
			},
		},
	}

	for _, h := range hops {
		if !statementsInOrder(t, h.body, h.stmts...) {
			t.Errorf("%s: the value no longer passes through these lines, in order:\n  %s\nin:\n%s",
				h.name, strings.Join(h.stmts, "\n  "), h.body)
			continue
		}
		// CONTROL: each flipped copy of the hop fails the same check.
		for _, f := range h.flips {
			if !strings.Contains(h.body, f[0]) {
				t.Fatalf("control: %s has no %q to flip", h.name, f[0])
			}
			flipped := strings.Replace(h.body, f[0], f[1], 1)
			if statementsInOrder(t, flipped, h.stmts...) {
				t.Fatalf("control: %s: the check passes with %q turned into %q", h.name, f[0], f[1])
			}
		}
		// CONTROL: a hop whose lines are commented out fails as well.
		commented := strings.Replace(h.body, h.stmts[len(h.stmts)-1], "// "+h.stmts[len(h.stmts)-1], 1)
		if statementsInOrder(t, commented, h.stmts...) {
			t.Fatalf("control: %s: the check passes with its last line commented out", h.name)
		}
	}
}
