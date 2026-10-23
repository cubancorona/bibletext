package bibletext

// The selection's chapter across every pane's seam (selSpan.book and
// selSpan.chapter, selection_span.go). A host test runs neither the
// Objective-C nor the Java, but it reads both, which are source here. These
// hold each native pane to keying the text it holds with the chapter Go
// pushed it as, and to handing that chapter over with every selection; Go's
// pushes to naming the chapter ahead of its HTML; and every Go function that
// takes a span to reading the selection in the span's chapter.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// callArgLists is the argument list of every call to fn in src, each
// argument with its white space collapsed, in the order they stand.
func callArgLists(src, fn string) [][]string {
	var out [][]string
	for i := 0; ; {
		j := strings.Index(src[i:], fn+"(")
		if j < 0 {
			return out
		}
		i += j + len(fn) + 1
		var args []string
		depth, from := 0, i
		for k := i; k < len(src); k++ {
			c := src[k]
			if c == '(' || c == '[' || c == '{' {
				depth++
				continue
			}
			if (c == ')' || c == ']' || c == '}') && depth > 0 {
				depth--
				continue
			}
			if c == ',' && depth == 0 || c == ')' && depth == 0 {
				if a := strings.Join(strings.Fields(src[from:k]), " "); a != "" {
					args = append(args, a)
				}
				from = k + 1
				if c == ')' {
					break
				}
			}
		}
		out = append(out, args)
	}
}

// Every dispatch of a selection hands Go the chapter its pane's text holds,
// read with the selection's words, span and lead: on iOS when the menu is
// built over the selection, on the Mac when the action runs, on Android when
// the toolbar item is chosen (and carried through the study popup), and
// through the JNI thunk unchanged.
func TestEverySelectionDispatchPassesItsChapter(t *testing.T) {
	for _, c := range []struct {
		file, fn    string
		book, chap  string // the call's last two arguments
		calls       int
		within      string // the function the calls stand in
		takenBefore []string
	}{
		{"reading_ios.go", "bibleTextAIMenuTapped", "(char *)book.UTF8String", "chapter", 1,
			"editMenuForTextInRange:(NSRange)range", []string{"NSString *book = gTVBook;", "int chapter = gTVChapter;"}},
		{"reading_ios.go", "bibleTextStudyMenuTapped", "(char *)book.UTF8String", "chapter", 1,
			"editMenuForTextInRange:(NSRange)range", []string{"NSString *book = gTVBook;", "int chapter = gTVChapter;"}},
		{"reading_macos.go", "bibleTextAIMenuTapped", "(char *)gMacTextBook.UTF8String", "gMacTextChapter", 1,
			"- (void)hbSendAI:(const char *)action {", nil},
		{"reading_macos.go", "bibleTextStudyMenuTapped", "(char *)gMacTextBook.UTF8String", "gMacTextChapter", 1,
			"- (void)hbSendStudy:(const char *)action {", nil},
		{"android/BtBridge.java", "nativeSelectionAction", "selBook", "selChapter", 2, "", nil},
		{"android/BtBridge.java", "showStudyPopup", "selBook", "selChapter", 1,
			"@Override public boolean onActionItemClicked(ActionMode mode, MenuItem item) {",
			[]string{"final String selBook = textBook;", "final int selChapter = textChapter;"}},
		{"reading_jni_android.c", "btaSelectionAction", "(char *)book", "(int)jChapter", 1, "", nil},
	} {
		src := readNativeSource(t, c.file)
		if c.within != "" {
			i := strings.Index(src, c.within)
			if i < 0 {
				t.Fatalf("%s: no %q", c.file, c.within)
			}
			src = src[i:]
			if c.file == "android/BtBridge.java" {
				src = src[:strings.Index(src, "\n            }\n")]
			} else {
				src = src[:strings.Index(src, "\n}\n")]
			}
		}
		n := 0
		for _, args := range callArgLists(src, c.fn) {
			if len(args) >= 2 && args[len(args)-2] == c.book && args[len(args)-1] == c.chap {
				n++
			}
		}
		if n != c.calls {
			t.Errorf("%s: %d call(s) of %s pass (%s, %s) last, want %d", c.file, n, c.fn, c.book, c.chap, c.calls)
		}
		first := strings.Index(src, c.fn+"(")
		for _, s := range c.takenBefore {
			if i := strings.Index(src, s); i < 0 || i > first {
				t.Errorf("%s: %q is not read ahead of %s", c.file, s, c.fn)
			}
		}
	}
	// The thunk's book is the Java string's, converted only when there is one.
	thunk := readNativeSource(t, "reading_jni_android.c")
	if !strings.Contains(thunk, "jstring jLead, jstring jBook, jint jChapter)") ||
		!strings.Contains(thunk, "const char *book = jBook != NULL ? (*env)->GetStringUTFChars(env, jBook, NULL) : NULL;") ||
		!strings.Contains(thunk, "if (book != NULL) (*env)->ReleaseStringUTFChars(env, jBook, book);") {
		t.Error("reading_jni_android.c: the thunk does not take the book as a jstring, convert a non-null one and release it")
	}
}

// Each native pane keys the text it holds with the chapter Go announced
// just ahead of that text's HTML, on every path that replaces the text and
// nowhere else, so the chapter a selection is handed over with is the one
// its words are from.
func TestEveryNativePaneKeysItsTextWithItsChapter(t *testing.T) {
	for _, c := range []struct {
		file, setter, setHTML, apply, fallback, key, keyCh, next, nextCh string
	}{
		{"reading_ios.go", "void bibleTextTVSetChapter(const char *book, int chapter) {",
			"void bibleTextTVSetHTML(const char *html, const char *fallbackText, const int *fallbackRuns,", "bibleTextApplyHTML(data)",
			"btIOSInstallPage(btIOSFallbackPage(fbText, fbRuns, fallbackStyle));",
			"gTVBook", "gTVChapter", "gTVNextBook", "gTVNextChapter"},
		{"reading_macos.go", "void bibleTextMacTVSetChapter(const char *book, int chapter) {",
			"void bibleTextMacTVSetHTML(const char *html, const char *fallbackText, const int *fallbackRuns,", "bibleTextMacApplyHTML(data)",
			"btMacInstallPage(btMacFallbackPage(fbText, fbRuns, fallbackStyle));",
			"gMacTextBook", "gMacTextChapter", "gMacNextBook", "gMacNextChapter"},
	} {
		setter := nativeFunctionSource(t, c.file, c.setter)
		if !inOrder(setter, "dispatch_async(dispatch_get_main_queue(), ^{", c.next+" = b;") || !strings.Contains(setter, c.nextCh+" = chapter;") {
			t.Errorf("%s: the chapter setter does not announce the chapter on the main queue", c.file)
		}
		body := nativeFunctionSource(t, c.file, c.setHTML)
		take := "NSString *book = " + c.next + ";"
		if !inOrder(body, "dispatch_async(dispatch_get_main_queue(), ^{", take) || !inOrder(body, take, c.apply) ||
			!strings.Contains(body, "int chapter = "+c.nextCh+";") {
			t.Errorf("%s: the HTML's block does not take the announced chapter before it imports", c.file)
		}
		// Taken once, behind the push's own generation check and ahead of the
		// first import and of the retry's check, so a retry or the fallback
		// keys the page with its own push's chapter even after the next push
		// has announced another.
		takeCh := "int chapter = " + c.nextCh + ";"
		if !chapterTakenOnceBehindTheGuard(body, take, takeCh, c.apply) {
			t.Errorf("%s: the announced chapter is not taken once, behind the push's generation check and ahead of the first import and the retries", c.file)
		}
		// CONTROLS: the chapter taken ahead of the generation check, taken in
		// the retry instead, or taken again in the retry, is each seen.
		ahead := swapOnce(body, chapterGuard, take)
		inRetry := afterNth(strings.Replace(body, take, "", 1), chapterGuard, 2, "\n"+take)
		again := afterNth(body, chapterGuard, 2, "\n"+take)
		for _, bad := range []string{ahead, inRetry, again} {
			if chapterTakenOnceBehindTheGuard(bad, take, takeCh, c.apply) {
				t.Fatalf("%s: the chapter check passes a chapter taken ahead of the generation check or in the retry; it proves nothing", c.file)
			}
		}
		keyed := "if (" + c.apply + ") { " + c.key + " = book; " + c.keyCh + " = chapter; return; }"
		if got, want := strings.Count(body, keyed), strings.Count(body, c.apply); got != want || want != 3 {
			t.Errorf("%s: %d of the %d imports key the text with its chapter, want all of 3", c.file, got, want)
		}
		// The fallback page is keyed where it is installed, in the branch that
		// installs it: a push with no fallback page installs nothing, and the
		// text then stays the previous page's, under its own key.
		if !fallbackKeyedWhereInstalled(body, c.fallback, c.key, c.keyCh) {
			t.Errorf("%s: the fallback page is not keyed with the chapter in the branch that installs it", c.file)
		}
		// CONTROLS: the key moved out of the install's branch, or dropped, is seen.
		moved := strings.Replace(body, c.fallback+"\n", c.fallback+"\n}\n", 1)
		dropped := strings.Replace(body, c.key+" = book;\n", "", 1)
		if fallbackKeyedWhereInstalled(moved, c.fallback, c.key, c.keyCh) || fallbackKeyedWhereInstalled(dropped, c.fallback, c.key, c.keyCh) {
			t.Fatalf("%s: the fallback key check passes with the key outside the install's branch or gone; it proves nothing", c.file)
		}
		// Nowhere else: the key changes only with the text.
		src := readNativeSource(t, c.file)
		if got := strings.Count(src, c.key+" = ") - strings.Count(src, "*"+c.key+" = nil;"); got != 4 {
			t.Errorf("%s: %s is written %d times, want the three imports and the fallback", c.file, c.key, got)
		}
	}

	java := readNativeSource(t, "android/BtBridge.java")
	setter := javaMethodSource(t, "android/BtBridge.java", "public static void setChapter(final String book, final int chapter) {")
	if !inSequence(setter, "UI.post(new Runnable() {", "nextBook = book;", "nextChapter = chapter;") {
		t.Error("BtBridge.setChapter does not announce the chapter on the UI queue")
	}
	setHTML := javaMethodSource(t, "android/BtBridge.java", "public static void setHtml(final String html,")
	if !inSequence(setHTML, "UI.post(new Runnable() {", "text.setText(s, TextView.BufferType.SPANNABLE);", "textBook = nextBook;", "textChapter = nextChapter;") {
		t.Error("BtBridge.setHtml does not key the text it sets with the announced chapter")
	}
	if got := strings.Count(java, "textBook = ") - strings.Count(java, "String textBook = null;"); got != 2 || !strings.Contains(java, "textBook = nextBook;") || !strings.Contains(java, "textBook = null;") {
		t.Errorf("BtBridge writes textBook %d times, want setHtml's key and the placeholder's reset", got)
	}
}

// chapterGuard is the check each step of an Apple pane's import makes that
// its push is still the latest (reading_native_fallback_contract_test.go,
// TestEveryImportStepActsOnlyForTheLatestPush).
const chapterGuard = "if (gen != gBodyGenPending) return;"

// chapterTakenOnceBehindTheGuard reports whether the HTML's block takes the
// announced chapter (take, takeCh) exactly once, after the first generation
// check, before the first import (apply) and before the next check, the
// retry's: a push already replaced takes nothing, and each retry and the
// fallback use the chapter the block took for its own push.
func chapterTakenOnceBehindTheGuard(body, take, takeCh, apply string) bool {
	return strings.Count(body, take) == 1 && strings.Count(body, takeCh) == 1 &&
		inSequence(body, chapterGuard, take, takeCh, apply, chapterGuard)
}

// afterNth inserts ins straight after the nth occurrence of sub in s, or
// returns s unchanged when there are fewer.
func afterNth(s, sub string, n int, ins string) string {
	at := 0
	for k := 0; k < n; k++ {
		i := strings.Index(s[at:], sub)
		if i < 0 {
			return s
		}
		at += i + len(sub)
	}
	return s[:at] + ins + s[at:]
}

// fallbackKeyedWhereInstalled reports whether body installs the fallback
// page in the branch taken when there is one (fbText != nil) and keys the
// text with the chapter straight after it, before that branch closes.
func fallbackKeyedWhereInstalled(body, install, key, keyCh string) bool {
	const branch = "&& fbText != nil) {"
	b := strings.Index(body, branch)
	if b < 0 {
		return false
	}
	i := strings.Index(body[b:], install)
	if i < 0 {
		return false
	}
	at := b + i + len(install)
	k := strings.Index(body[at:], key+" = book;")
	if k < 0 || strings.Contains(body[b+len(branch):at+k], "}") {
		return false
	}
	rest := body[at+k:]
	ch, end := strings.Index(rest, keyCh+" = chapter;"), strings.Index(rest, "}")
	return ch >= 0 && end >= 0 && ch < end
}

// Go names the chapter ahead of every chapter's HTML it pushes, on the same
// thread, so the pane receives the two in that order.
func TestEveryChapterPushNamesItsChapterFirst(t *testing.T) {
	for _, c := range []struct{ file, chapter, html string }{
		{"reading_ios.go", "C.bibleTextTVSetChapter(cb, C.int(state.CurrentChapter))", "C.bibleTextTVSetHTML(c, fbText, fbRuns,"},
		{"reading_macos.go", "C.bibleTextMacTVSetChapter(cb, C.int(state.CurrentChapter))", "C.bibleTextMacTVSetHTML(c, fbText, fbRuns,"},
		{"reading_android.go", "C.btaSetChapter(C.uintptr_t(env), cb, C.int(chapter))", "C.btaSetHtml(C.uintptr_t(env), ch,"},
	} {
		src := readNativeSource(t, c.file)
		if strings.Count(src, c.html) != 1 || strings.Count(src, c.chapter) != 1 {
			t.Errorf("%s: %d HTML push(es) and %d chapter announcement(s), want one of each",
				c.file, strings.Count(src, c.html), strings.Count(src, c.chapter))
			continue
		}
		if !inOrder(src, c.chapter, c.html) {
			t.Errorf("%s: the chapter is not announced ahead of the HTML", c.file)
		}
	}
	android := readNativeSource(t, "reading_android.go")
	if !strings.Contains(android, "book, chapter := state.CurrentBook, state.CurrentChapter") ||
		!strings.Contains(android, "cb := C.CString(book)") {
		t.Error("reading_android.go: the announced chapter is not the state's")
	}
	for _, f := range []string{"reading_ios.go", "reading_macos.go"} {
		if !strings.Contains(readNativeSource(t, f), "cb := C.CString(state.CurrentBook)") {
			t.Errorf("%s: the announced book is not the state's", f)
		}
	}
	if !strings.Contains(android, "btaSetChapterM == NULL") {
		t.Error("reading_android.go: a missing setChapter does not stand the bridge down as every other lookup does")
	}
}

// The exports build the span through one function, with the chapter the pane
// sent.
func TestTheNativeExportsBuildTheSpanWithItsChapter(t *testing.T) {
	darwin := readNativeSource(t, "ai_menu_darwin.go")
	if n := strings.Count(darwin, "span := nativeSelSpan(lo, hi, heading, cLead, cBook, chapter)"); n != 2 {
		t.Errorf("ai_menu_darwin.go: %d export(s) build the span with the chapter, want both", n)
	}
	if !strings.Contains(darwin, "return paneSelSpan(int(lo), int(hi), lead, cLead != nil, book, int(chapter))") {
		t.Error("ai_menu_darwin.go: nativeSelSpan does not build the span through paneSelSpan")
	}
	android := readNativeSource(t, "reading_android_export.go")
	if !strings.Contains(android, "span := paneSelSpan(int(lo), int(hi), lead, cLead != nil, book, int(chapter))") {
		t.Error("reading_android_export.go: btaSelectionAction does not build the span through paneSelSpan")
	}
}

// Every Go function that takes a selection's span, or the selected verses
// selectionVerses read for it, reads the selection in its own chapter
// (selectionChapter, selectedChapter), not the reader's: no call of
// readerChapter or readerPassage and no read of CurrentBook or
// CurrentChapter in its body. The one exception is the stamp a menu builder
// puts on a span its pane named no chapter for, span.inChapter(...), and
// selectionChapter and selectedChapter themselves, whose fallback the
// reader's chapter is.
func TestEverySpanConsumerReadsTheSelectionsChapter(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var offenders []string
	consumers, verseConsumers := 0, 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Name.Name == "selectionChapter" || fn.Name.Name == "selectedChapter" {
				continue
			}
			switch {
			case takesSelSpan(fn.Type):
				consumers++
			case takesSelectedVerses(fn.Type):
				verseConsumers++
			default:
				continue
			}
			stamped := map[ast.Node]bool{}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "CurrentBook" || sel.Sel.Name == "CurrentChapter") {
						offenders = append(offenders, fset.Position(sel.Pos()).String()+" "+fn.Name.Name+" reads "+sel.Sel.Name)
					}
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "inChapter" {
					for _, a := range call.Args {
						stamped[a] = true
					}
				}
				if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "readerChapter" || id.Name == "readerPassage") && !stamped[call] {
					offenders = append(offenders, fset.Position(call.Pos()).String()+" "+fn.Name.Name+" calls "+id.Name)
				}
				return true
			})
		}
	}
	if consumers < 20 {
		t.Fatalf("found only %d functions taking a selSpan; the parse is reading nothing", consumers)
	}
	if verseConsumers < 3 {
		t.Fatalf("found only %d functions taking the selected verses; the parse is reading nothing", verseConsumers)
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("%s: a selection is read in the reader's chapter, not its own (selectionChapter, selectedChapter)", o)
	}
}

// takesSelectedVerses reports whether a function's parameters include the
// selected verses, a []Verse named selected.
func takesSelectedVerses(ft *ast.FuncType) bool {
	if ft.Params == nil {
		return false
	}
	for _, p := range ft.Params.List {
		arr, ok := p.Type.(*ast.ArrayType)
		if !ok || arr.Len != nil {
			continue
		}
		if id, ok := arr.Elt.(*ast.Ident); !ok || id.Name != "Verse" {
			continue
		}
		for _, n := range p.Names {
			if n.Name == "selected" {
				return true
			}
		}
	}
	return false
}

// takesSelSpan reports whether a function's parameters include a selSpan.
func takesSelSpan(ft *ast.FuncType) bool {
	if ft.Params == nil {
		return false
	}
	for _, p := range ft.Params.List {
		if id, ok := p.Type.(*ast.Ident); ok && id.Name == "selSpan" {
			return true
		}
	}
	return false
}
