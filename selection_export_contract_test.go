package bibletext

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// A Go //export is called from C through a declaration the calling file
// writes for itself: reading_ios.go and reading_macos.go declare the Apple
// exports `extern` in their own preambles, and reading_jni_android.c declares
// the Android ones. Nothing compares those declarations with the Go function:
// the C compiler sees one translation unit at a time, so a Go export widened
// by a parameter (the selection's lead, docs/SELECTION_OFFSET.md) while a
// declaration still has the old list compiles, links, and passes garbage, or
// nothing, in the new parameter's register. A host test cannot run either
// side, but it can read both, which are source here. These tests hold the
// exports a selection crosses; the Java half of the Android seam is
// TestEveryJavaNativeHasAMatchingThunk's.

var cExternDecl = regexp.MustCompile(`(?m)^extern\s+(\w+(?:\s*\*)?)\s*(\w+)\(([^)]*)\);`)

// cParamTypes is the parameter types of a C list, "char *action, int lo" as
// "char*", "int"; "void" or nothing is no parameters.
func cParamTypes(list string) []string {
	list = strings.TrimSpace(list)
	if list == "" || list == "void" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(list, ",") {
		p = strings.TrimSpace(p)
		star := strings.Contains(p, "*")
		f := strings.Fields(strings.ReplaceAll(p, "*", " "))
		typ := strings.Join(f[:len(f)-1], " ")
		if star {
			typ += "*"
		}
		out = append(out, typ)
	}
	return out
}

// goExportParamTypes is the parameter types of the //export'ed Go function
// name in src, each spelled as cgo spells it ("*C.char", "C.int"), or nil and
// false when src does not export it.
func goExportParamTypes(src, name string) ([]string, bool) {
	m := regexp.MustCompile(`(?m)^//export ` + regexp.QuoteMeta(name) + `\nfunc ` + regexp.QuoteMeta(name) + `\(([^)]*)\)`).FindStringSubmatch(src)
	if m == nil {
		return nil, false
	}
	var out []string
	var pending int // names waiting for the type that follows them
	for _, p := range strings.Split(m[1], ",") {
		f := strings.Fields(p)
		switch len(f) {
		case 0:
		case 1:
			pending++
		default:
			for i := 0; i <= pending; i++ {
				out = append(out, f[len(f)-1])
			}
			pending = 0
		}
	}
	return out, true
}

var cToCgo = map[string]string{"char*": "*C.char", "int": "C.int", "float": "C.float", "double": "C.double"}

// selectionExports are the exports a selection crosses, and the number of
// files that declare each.
var selectionExports = map[string]int{"bibleTextAIMenuTapped": 2, "bibleTextStudyMenuTapped": 2, "btaSelectionAction": 1}

func TestTheSelectionExportsMatchTheirCDeclarations(t *testing.T) {
	var goSrc strings.Builder
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".go") && !strings.HasSuffix(f.Name(), "_test.go") {
			b, err := os.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			goSrc.Write(b)
			goSrc.WriteByte('\n')
		}
	}
	checked := map[string]int{}
	for _, file := range []string{"reading_ios.go", "reading_macos.go", "reading_jni_android.c"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range cExternDecl.FindAllStringSubmatch(string(b), -1) {
			name := m[2]
			if _, ok := selectionExports[name]; !ok {
				continue
			}
			goTypes, ok := goExportParamTypes(goSrc.String(), name)
			if !ok {
				t.Errorf("%s declares %s, which no Go file exports", file, name)
				continue
			}
			cTypes := cParamTypes(m[3])
			var want []string
			for _, c := range cTypes {
				g, ok := cToCgo[c]
				if !ok {
					t.Fatalf("%s: %s takes a C %q; add its cgo spelling to cToCgo", file, name, c)
				}
				want = append(want, g)
			}
			if strings.Join(want, ", ") != strings.Join(goTypes, ", ") {
				t.Errorf("%s declares %s(%s), but the Go export takes (%s)", file, name, m[3], strings.Join(goTypes, ", "))
			}
			checked[name]++
		}
	}
	// Each must have been found in every file that declares it, or this test
	// is reading nothing.
	for name, files := range selectionExports {
		if checked[name] != files {
			t.Errorf("%s was checked in %d files, want %d", name, checked[name], files)
		}
	}
}

// callArgCounts is the number of arguments of every call to fn in src.
func callArgCounts(src, fn string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(src[i:], fn+"(")
		if j < 0 {
			return out
		}
		i += j + len(fn) + 1
		depth, n, empty := 0, 1, true
		for k := i; k < len(src); k++ {
			switch src[k] {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth == 0 {
					if empty {
						n = 0
					}
					out = append(out, n)
					k = len(src)
					continue
				}
				depth--
			case ',':
				if depth == 0 {
					n++
				}
			case ' ', '\n', '\t':
			default:
				empty = false
			}
		}
	}
}

// Every call site of the selection's exports passes whether the selection
// lies in a heading and the lead, and after them the chapter the pane's text
// holds (selection_chapter_contract_test.go holds which values,
// heading_word_selection_contract_test.go the heading's): the two Apple
// panes' four calls and BtBridge's two. A call left short would not compile
// in C or Java, but a declaration widened without its call sites is exactly
// what this guards together with the tests above.
func TestEverySelectionDispatchPassesTheLead(t *testing.T) {
	for _, c := range []struct {
		file, fn string
		args     int
		calls    int
	}{
		{"reading_ios.go", "bibleTextAIMenuTapped", 8, 2},
		{"reading_ios.go", "bibleTextStudyMenuTapped", 8, 2},
		{"reading_macos.go", "bibleTextAIMenuTapped", 8, 2},
		{"reading_macos.go", "bibleTextStudyMenuTapped", 8, 2},
		{"android/BtBridge.java", "nativeSelectionAction", 8, 3},
		{"reading_jni_android.c", "btaSelectionAction", 8, 2},
	} {
		b, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		counts := callArgCounts(string(b), c.fn)
		if len(counts) != c.calls {
			t.Errorf("%s: %d occurrences of %s(, want %d (the declaration and the calls)", c.file, len(counts), c.fn, c.calls)
		}
		for _, n := range counts {
			if n != c.args {
				t.Errorf("%s: %s( with %d arguments, want %d", c.file, c.fn, n, c.args)
			}
		}
	}
}
