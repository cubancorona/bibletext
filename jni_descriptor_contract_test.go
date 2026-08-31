package bibletext

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// A JNI descriptor is a string. It is never checked against the Java it names:
// GetStaticMethodID on a mismatch returns NULL, leaves a pending
// NoSuchMethodError, and every wrapper in reading_android.go guards only on
// btaClass — so a widened Java signature whose descriptor was not updated
// compiles clean, links clean, packages clean, and does nothing on a device.
// A host test cannot load the class, but it can read both sides of the
// contract, because both are source in this repo.

var jniLookup = regexp.MustCompile(
	`GetStaticMethodID\(env,\s*btaClass,\s*"([A-Za-z0-9_]+)",\s*\n?\s*"([^"]*)"\)`)

// Only the types the bridge actually passes. An unknown type is a test
// failure, not a silent pass — a new parameter type must be added here
// deliberately.
var javaTypeDescriptor = map[string]string{
	"byte[]":   "[B",
	"int[]":    "[I",
	"String":   "Ljava/lang/String;",
	"boolean":  "Z",
	"int":      "I",
	"long":     "J",
	"float":    "F",
	"double":   "D",
	"void":     "V",
	"Activity": "Landroid/app/Activity;",
}

func javaMethodDescriptor(t *testing.T, java, name string) (string, bool) {
	t.Helper()
	// public static <ret> <name>(<params>)
	decl := regexp.MustCompile(
		`public static ([A-Za-z0-9_\[\]]+) ` + regexp.QuoteMeta(name) + `\(([^)]*)\)`)
	m := decl.FindStringSubmatch(java)
	if m == nil {
		return "", false
	}
	var args []string
	for _, p := range strings.Split(m[2], ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f := strings.Fields(strings.TrimPrefix(p, "final "))
		if len(f) < 2 {
			t.Fatalf("%s: cannot parse parameter %q", name, p)
		}
		d, ok := javaTypeDescriptor[f[0]]
		if !ok {
			t.Fatalf("%s: unknown Java parameter type %q — add it to javaTypeDescriptor", name, f[0])
		}
		args = append(args, d)
	}
	ret, ok := javaTypeDescriptor[m[1]]
	if !ok {
		t.Fatalf("%s: unknown Java return type %q", name, m[1])
	}
	return fmt.Sprintf("(%s)%s", strings.Join(args, ""), ret), true
}

func TestJNIDescriptorsMatchBtBridge(t *testing.T) {
	goSrc := readNativeSource(t, "reading_android.go")
	java := readNativeSource(t, "android/BtBridge.java")

	found := jniLookup.FindAllStringSubmatch(goSrc, -1)
	if len(found) < 10 {
		t.Fatalf("matched only %d GetStaticMethodID lookups — the regexp has drifted "+
			"from reading_android.go and this test is checking nothing", len(found))
	}
	for _, m := range found {
		name, want := m[1], m[2]
		got, ok := javaMethodDescriptor(t, java, name)
		if !ok {
			t.Errorf("reading_android.go looks up %q, which BtBridge.java does not declare "+
				"as a public static method", name)
			continue
		}
		if got != want {
			t.Errorf("JNI descriptor skew for %s:\n  reading_android.go: %s\n  BtBridge.java:      %s\n"+
				"On a device this lookup returns NULL and the call silently does nothing.",
				name, want, got)
		}
	}
}
