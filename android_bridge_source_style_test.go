package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// The cgo preamble of reading_android.go and reading_jni_android.c indent
// their statements with tabs, as gofmt indents the Go beside them. gofmt
// leaves a cgo preamble alone and git diff --check does not flag a leading
// space, so nothing else holds them to it. A continuation line aligned under
// an opening parenthesis with several spaces is the files' own form and is
// left alone. The compose bridge block of BtBridge.java keeps its members
// one blank line apart, as the rest of the class does.
func TestAndroidBridgeSourcesIndentWithTabs(t *testing.T) {
	single := regexp.MustCompile(`(?m)^ [^ *]`)
	for _, file := range []string{"reading_jni_android.c", "reading_android.go"} {
		src := readNativeSource(t, file)
		if file == "reading_android.go" {
			end := strings.Index(src, "*/\nimport \"C\"")
			if end < 0 {
				t.Fatalf("%s has no cgo preamble end, so this test reads nothing", file)
			}
			src = src[:end]
		}
		for _, m := range single.FindAllStringIndex(src, -1) {
			t.Errorf("%s:%d is indented with a single space", file, 1+strings.Count(src[:m[0]], "\n"))
		}
	}
	java := readNativeSource(t, "android/BtBridge.java")
	const last = "static int[] composeOrigin() { return windowContentOrigin(); }\n"
	i := strings.Index(java, last)
	if i < 0 {
		t.Fatal("BtBridge.java has no compose bridge block, so this test reads nothing")
	}
	if strings.HasPrefix(java[i+len(last):], "\n\n") {
		t.Error("BtBridge.java leaves two blank lines after the compose bridge block")
	}
}
