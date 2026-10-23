package bibletext

// One permission rule on every platform (ai_permission.go). The question is
// a sheet the toolkit draws, the same on the iPhone, iPad, Mac, Android,
// Windows and Linux; what could make a platform diverge is a way to a
// provider that does not pass it. A host test runs neither the Objective-C
// nor the Java, but reads both, which are source here. These hold every
// surface's study menu to the one Go dispatch whose panel asks, the reader's
// text to leaving Go only through the two seams behind the permission check,
// the native sources to naming no provider, and both Find panes to asking
// before a search starts. The behaviour itself is ai_permission_test.go's.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// sourceFrom is body from the first mark on, or a failure if it has none.
func sourceFrom(t *testing.T, body, mark string) string {
	t.Helper()
	i := strings.Index(body, mark)
	if i < 0 {
		t.Fatalf("%q is gone; this contract has lost its subject", mark)
	}
	return body[i:]
}

// sourceOrder fails unless first stands in body, and before second.
func sourceOrder(t *testing.T, where, body, first, second string) {
	t.Helper()
	i, j := strings.Index(body, first), strings.Index(body, second)
	if i < 0 || j < 0 || i > j {
		t.Errorf("%s: %q must come before %q (at %d and %d)", where, first, second, i, j)
	}
}

// Every surface's study menu reaches the one dispatch, which opens the panel
// or, for Ask, the composer whose Ask opens it; the panel asks before its
// request starts, and nothing else opens it.
func TestEveryStudyMenuReachesThePermissionThroughOneDispatch(t *testing.T) {
	for _, c := range []struct{ platforms, file, fn string }{
		{"the iPhone, iPad and Mac panes", "ai_menu_darwin.go", "bibleTextAIMenuTapped"},
		{"the Android pane", "reading_android_export.go", "btaSelectionAction"},
		{"the Windows and Linux panes", "reading.go", "selectionStudyMenu"},
	} {
		if !strings.Contains(funcBody(t, readRepoFile(t, c.file), c.fn), "dispatchAIAction(") {
			t.Errorf("%s: %s no longer hands its study verbs to dispatchAIAction", c.platforms, c.fn)
		}
	}
	dispatch := funcBody(t, readRepoFile(t, "ai.go"), "dispatchAIAction")
	for _, call := range []string{"promptAskQuestion(", "showAIPanel("} {
		if !strings.Contains(dispatch, call) {
			t.Errorf("dispatchAIAction no longer calls %s", call)
		}
	}
	if !strings.Contains(funcBody(t, readRepoFile(t, "ai_ask.go"), "promptAskQuestionForPassage"),
		"showAIPanelForPassage(state, aiActionAsk") {
		t.Error("the Ask composer no longer sends its question through the study panel")
	}
	fetch := sourceFrom(t, funcBody(t, readRepoFile(t, "ai_panel.go"), "showAIPanelForPassage"), "startFetch = func() {")
	sourceOrder(t, "the panel's startFetch", fetch, "aiPermissionPending(state)", "aiActionRun(")
	sourceOrder(t, "the panel's startFetch", fetch, "promptAIPermission(", "aiActionRun(")

	// Nothing but the dispatch's own chain, and the development scenes,
	// opens the panel.
	allowed := map[string]bool{
		"ai.go": true, "ai_ask.go": true, "ai_panel.go": true, "dev_open_sheets_on.go": true,
	}
	for _, f := range productionGoFiles(t) {
		src := readRepoFile(t, f)
		if (strings.Contains(src, "showAIPanel(") || strings.Contains(src, "showAIPanelForPassage(")) && !allowed[f] {
			t.Errorf("%s opens the study panel outside the dispatch", f)
		}
	}
}

// productionGoFiles is every non-test Go file of the root package, under
// every build tag.
func productionGoFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range paths {
		if name := filepath.Base(p); !strings.HasSuffix(name, "_test.go") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	if len(out) < 100 {
		t.Fatalf("found %d Go files; the listing is broken, so this test proves nothing", len(out))
	}
	return out
}

// The reader's text leaves Go for a provider only through the two seams, and
// each is called only behind the permission check. Every call to a client's
// generate is accounted for: the two seams, Test key's fixed probe and the
// model resolver inside a client.
func TestOnlyTheTwoSeamsBehindThePermissionSendTheReadersText(t *testing.T) {
	fset := token.NewFileSet()
	where := map[string][]string{}
	for _, f := range productionGoFiles(t) {
		file, err := parser.ParseFile(fset, filepath.Join(repoRoot(t), f), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		// Each function, and each package-level value, is its own owner.
		var owned []struct {
			owner string
			node  ast.Node
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				owner := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) == 1 {
					if star, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
						if id, ok := star.X.(*ast.Ident); ok {
							owner = id.Name + "." + owner
						}
					}
				}
				owned = append(owned, struct {
					owner string
					node  ast.Node
				}{owner, d})
			case *ast.GenDecl:
				for _, s := range d.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok {
						owned = append(owned, struct {
							owner string
							node  ast.Node
						}{vs.Names[0].Name, vs})
					}
				}
			}
		}
		for _, o := range owned {
			owner := o.owner
			ast.Inspect(o.node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "generate" {
					where[owner] = append(where[owner], fset.Position(call.Pos()).String())
					if owner == "aiKeyTestRun" {
						if lit, ok := call.Args[len(call.Args)-1].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
							t.Errorf("Test key's probe is no longer a fixed string: %s", fset.Position(call.Pos()))
						}
					}
				}
				return true
			})
		}
	}
	want := map[string]bool{
		"aiStudyGenerate": true, "aiSearchGenerate": true, "aiKeyTestRun": true, "modelResolver.generate": true,
	}
	for owner, at := range where {
		if !want[owner] {
			t.Errorf("%s calls a provider's generate outside the seams behind the permission check: %v", owner, at)
		}
	}
	for owner := range want {
		if len(where[owner]) == 0 {
			t.Errorf("%s no longer calls generate; this contract has lost its subject", owner)
		}
	}

	for _, c := range []struct{ file, fn, seam string }{
		{"ai.go", "runAIActionForPassage", "aiStudyGenerate("},
		{"ai_search.go", "runAISearch", "aiSearchGenerate("},
	} {
		body := funcBody(t, readRepoFile(t, c.file), c.fn)
		// The check is made with the key the request sends (keyStore.aiAllowed).
		sourceOrder(t, c.fn, body, "key := providerAPIKey(store, id)", "store.aiAllowed(id, key)")
		sourceOrder(t, c.fn, body, "store.aiAllowed(id, key)", c.seam)
		sourceOrder(t, c.fn, body, "&aiPermissionError{", c.seam)
		for _, f := range productionGoFiles(t) {
			src := readRepoFile(t, f)
			n := strings.Count(src, c.seam)
			if f == c.file {
				n -= strings.Count(body, c.seam)
			}
			if n > 0 {
				t.Errorf("%s calls %s outside %s", f, strings.TrimSuffix(c.seam, "("), c.fn)
			}
		}
	}
}

// No native source talks to a provider: the Objective-C in the cgo preambles,
// the Java, the C and the toolkit patches name none of the providers' hosts.
// Their hosts are read from the clients' own base URLs.
func TestNoNativeSourceNamesAProviderHost(t *testing.T) {
	var hosts []string
	for _, base := range []string{geminiBaseURL, openAIBaseURL, anthropicBaseURL, grokBaseURL} {
		u, err := url.Parse(base)
		if err != nil || u.Host == "" {
			t.Fatalf("cannot read a host from %q", base)
		}
		hosts = append(hosts, u.Host)
	}
	root := repoRoot(t)
	sources := map[string]string{}
	for _, f := range productionGoFiles(t) {
		src := readRepoFile(t, f)
		if i := strings.Index(src, "\nimport \"C\""); i >= 0 {
			sources[f+" (cgo preamble)"] = src[:i]
		}
	}
	for _, pattern := range []string{"android/*.java", "*.c", "*.m", "*.h", "patches/*"} {
		paths, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			if st, err := os.Stat(p); err != nil || st.IsDir() {
				continue
			}
			rel, _ := filepath.Rel(root, p)
			sources[rel] = readRepoFile(t, rel)
		}
	}
	if len(sources) < 8 {
		t.Fatalf("read %d native sources; the listing is broken, so this test proves nothing", len(sources))
	}
	for name, src := range sources {
		for _, h := range hosts {
			if strings.Contains(src, h) {
				t.Errorf("%s names the provider host %s: a request from there would pass no permission", name, h)
			}
		}
	}
}

// Both Find panes ask before a search starts: the desktop sidebar's and the
// phone's Search tab's submissions each go through findAsksPermission, after
// the key check and before the request.
func TestBothFindPanesAskBeforeASearchStarts(t *testing.T) {
	for _, c := range []struct{ surface, file, fn, start string }{
		{"the desktop sidebar", "sidebar.go", "buildSidebar", "startAISearch("},
		{"the phone's Search tab", "ui_compact.go", "buildMobileSearchTab", "startFind("},
	} {
		submit := sourceFrom(t, funcBody(t, readRepoFile(t, c.file), c.fn), "runAsk = func(q string) {")
		sourceOrder(t, c.surface, submit, "hasAIKey(state)", "findAsksPermission(")
		sourceOrder(t, c.surface, submit, "findAsksPermission(", c.start)
	}
}
