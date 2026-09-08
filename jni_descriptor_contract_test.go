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

// jniLookupFor matches the GetStaticMethodID calls made against one cached
// class variable. It is per-class rather than fixed because there are three
// bridges, and a regexp hard-wired to one of them silently checks nothing for
// the other two.
func jniLookupFor(classVar string) *regexp.Regexp {
	return regexp.MustCompile(
		`GetStaticMethodID\(env,\s*` + regexp.QuoteMeta(classVar) +
			`,\s*"([A-Za-z0-9_]+)",\s*\n?\s*"([^"]*)"\)`)
}

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

// Every Go↔Java bridge in the app. BtAudio's seven descriptors were checked by
// nothing before this became a table; BtKeys' three carry a reader's API key,
// and a skew there means the store silently reports itself absent.
var jniBridges = []struct {
	goFile   string
	javaFile string
	classVar string
	min      int // drift guard: fewer lookups than this means the regexp missed
}{
	{"reading_android.go", "android/BtBridge.java", "btaClass", 10},
	{"audio_android.go", "android/BtAudio.java", "btAudioClass", 5},
	{"ai_secure_store_android.go", "android/BtKeys.java", "btKeysClass", 3},
}

func TestJNIDescriptorsMatchJava(t *testing.T) {
	for _, b := range jniBridges {
		t.Run(b.classVar, func(t *testing.T) {
			goSrc := readNativeSource(t, b.goFile)
			java := readNativeSource(t, b.javaFile)

			found := jniLookupFor(b.classVar).FindAllStringSubmatch(goSrc, -1)
			if len(found) < b.min {
				t.Fatalf("matched only %d GetStaticMethodID lookups against %s — the regexp "+
					"has drifted from %s and this test is checking nothing",
					len(found), b.classVar, b.goFile)
			}
			for _, m := range found {
				name, want := m[1], m[2]
				got, ok := javaMethodDescriptor(t, java, name)
				if !ok {
					t.Errorf("%s looks up %q, which %s does not declare as a public static method",
						b.goFile, name, b.javaFile)
					continue
				}
				if got != want {
					t.Errorf("JNI descriptor skew for %s:\n  %s: %s\n  %s: %s\n"+
						"On a device this lookup returns NULL and the call silently does nothing.",
						name, b.goFile, want, b.javaFile, got)
				}
			}
		})
	}
}

// THE OTHER DIRECTION, which nothing checked.
//
// The test above covers Go→Java: a descriptor string handed to
// GetStaticMethodID, where a mismatch returns NULL and the call silently does
// nothing. The Java→native direction fails differently and worse. JNI's SHORT
// name mangling omits the signature entirely unless the method is overloaded,
// so `Java_org_bibletext_BtBridge_nativeNoteHidden` links against ANY C
// function of that name whatever its parameters. Widen the Java declaration and
// forget the C thunk and it still links — and reads whatever happens to be in
// the argument register, on a device, with nothing in the log.
//
// A missing thunk is the benign case: UnsatisfiedLinkError at the first press.
// A MISMATCHED one is silent. So both halves are checked here.
//
// The thunks live in more than one C file (the note callbacks in
// reading_jni_android.c, the shared-link one in share_link_jni_android.c), so
// this scans them all — counting one file and comparing totals reports a
// missing thunk that is merely somewhere else.
var javaNativeDecl = regexp.MustCompile(
	`(?m)^\s*private static native\s+(\w+)\s+(\w+)\s*\(([^)]*)\)\s*;`)

// The C parameter spelling each Java type must have in the thunk.
var javaToJNIParam = map[string]string{
	"String":  "jstring",
	"int":     "jint",
	"float":   "jfloat",
	"boolean": "jboolean",
	"long":    "jlong",
	"double":  "jdouble",
	"byte[]":  "jbyteArray",
	"int[]":   "jintArray",
}

func TestEveryJavaNativeHasAMatchingThunk(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")

	// Every C file that could hold a thunk.
	var cSrc string
	for _, f := range []string{"reading_jni_android.c", "share_link_jni_android.c"} {
		cSrc += readNativeSource(t, f)
	}
	if n := strings.Count(cSrc, "Java_org_bibletext_BtBridge_"); n < 5 {
		t.Fatalf("only %d thunks found across the C sources — a file has moved and "+
			"this test would report every native as missing", n)
	}

	decls := javaNativeDecl.FindAllStringSubmatch(java, -1)
	if len(decls) < 5 {
		t.Fatalf("parsed only %d native declarations from BtBridge.java; the "+
			"regexp has drifted and this test is checking nothing", len(decls))
	}

	for _, d := range decls {
		ret, name, params := d[1], d[2], d[3]
		t.Run(name, func(t *testing.T) {
			if ret != "void" {
				t.Errorf("%s returns %s; these callbacks are all void, and a "+
					"non-void one needs its return type checked here too", name, ret)
			}
			// The thunk, with its opening parenthesis so a prefix cannot match.
			marker := "Java_org_bibletext_BtBridge_" + name + "("
			i := strings.Index(cSrc, marker)
			if i < 0 {
				t.Fatalf("no C thunk for %s. Java declares it native and calls it; "+
					"without an implementation the first press throws "+
					"UnsatisfiedLinkError.", name)
			}
			sig := cSrc[i+len(marker):]
			if e := strings.Index(sig, ")"); e >= 0 {
				sig = sig[:e]
			}
			if !strings.Contains(sig, "JNIEnv") || !strings.Contains(sig, "jclass") {
				t.Errorf("%s's thunk does not take (JNIEnv *, jclass ...): %q", name, sig)
			}
			// Every Java parameter must appear as its JNI spelling, in order.
			at := 0
			for _, p := range strings.Split(params, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				f := strings.Fields(strings.TrimPrefix(p, "final "))
				if len(f) < 2 {
					t.Fatalf("%s: cannot parse parameter %q", name, p)
				}
				want, ok := javaToJNIParam[f[0]]
				if !ok {
					t.Fatalf("%s: unknown Java parameter type %q — add it to "+
						"javaToJNIParam rather than letting it pass unchecked", name, f[0])
				}
				k := strings.Index(sig[at:], want)
				if k < 0 {
					t.Errorf("%s: Java passes %s %s, and the thunk's parameters are %q "+
						"— no %s in that position. JNI short-name mangling omits the "+
						"signature, so this LINKS and reads a garbage argument on a "+
						"device.", name, f[0], f[1], sig, want)
					return
				}
				at += k + len(want)
			}
		})
	}
}
