package bibletext

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestTheSharingTableSaysOfAnUnreadableCardWhatTheFallbackSays holds what the
// Linux and Windows rows of docs/PLATFORM_MATRIX.md's Sharing table say of a
// card that could not be read at the tap to the code that ends that share.
// Both rows reach the same branch of fallbackShareImage for it: Linux hands
// every picture there (nativeShareImage, share_other.go), and Windows' Share
// sheet hands it a picture it could not take (sharedImage.fallback,
// share_parts.go). The words that branch has the reader told are the string
// constants and literals it passes to the package's own functions; the log
// line, passed to fyne.LogError, is not among them. A Share as image cell
// that names the case must quote each of those words, and while the branch
// tells the reader nothing, no such cell may quote a "Could not share" line.
// A cell that does not name the case is not held. The one branch described
// two ways, which a row's record and a change to that code can leave behind
// when they merge, fails here.
func TestTheSharingTableSaysOfAnUnreadableCardWhatTheFallbackSays(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	parse := func(name string) *ast.File {
		t.Helper()
		if f, ok := parsed[name]; ok {
			return f
		}
		f, err := parser.ParseFile(fset, filepath.Join(root, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		parsed[name] = f
		return f
	}
	method := func(file, recv, name string) *ast.FuncDecl {
		t.Helper()
		for _, decl := range parse(file).Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			got := ""
			if fn.Recv != nil && len(fn.Recv.List) == 1 {
				if id, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
					got = id.Name
				}
			}
			if got == recv {
				return fn
			}
		}
		t.Fatalf("%s declares no %s with receiver %q", file, name, recv)
		return nil
	}

	// The rows held here are the ones whose image share reaches the fallback.
	for _, route := range []struct{ row, file, recv, name string }{
		{"Linux", "share_other.go", "", "nativeShareImage"},
		{"Windows", "share_parts.go", "sharedImage", "fallback"},
	} {
		if !callsFunc(method(route.file, route.recv, route.name), "fallbackShareImage") {
			t.Errorf("%s: %s in %s no longer calls fallbackShareImage; the %s row's picture may no "+
				"longer end there, so this test's rows need updating", route.row, route.name, route.file, route.row)
		}
	}

	// The branch for a card that could not be read: the if right after the
	// card is read with os.ReadFile.
	fallback := method("share_fallback.go", "", "fallbackShareImage")
	var branch *ast.BlockStmt
	stmts := fallback.Body.List
	for i, stmt := range stmts {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 || i+1 == len(stmts) {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "ReadFile" {
			continue
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "os" {
			continue
		}
		if next, ok := stmts[i+1].(*ast.IfStmt); ok {
			branch = next.Body
		}
		break
	}
	if branch == nil {
		t.Fatal("share_fallback.go: fallbackShareImage has no if right after reading the card with os.ReadFile; " +
			"find the branch for a card that could not be read and update this test")
	}

	// The package's string constants, by name, from every non-test file.
	consts := map[string]string{}
	matches, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, decl := range parse(name).Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, id := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						if s, err := strconv.Unquote(lit.Value); err == nil {
							consts[id.Name] = s
						}
					}
				}
			}
		}
	}

	said := map[string]bool{}
	ast.Inspect(branch, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if _, own := call.Fun.(*ast.Ident); !own {
			return true
		}
		for _, arg := range call.Args {
			switch arg := arg.(type) {
			case *ast.BasicLit:
				if arg.Kind == token.STRING {
					if s, err := strconv.Unquote(arg.Value); err == nil {
						said[s] = true
					}
				}
			case *ast.Ident:
				if s, ok := consts[arg.Name]; ok {
					said[s] = true
				}
			}
		}
		return true
	})
	var words []string
	for s := range said {
		words = append(words, s)
	}
	sort.Strings(words)

	table := sharingTable(t)
	for _, row := range []string{"Linux", "Windows"} {
		cells, ok := table[row]
		if !ok {
			t.Errorf("the Sharing table has no %s row", row)
			continue
		}
		cell := cells["Share as image"]
		if !strings.Contains(cell, "card that could not be read") {
			continue
		}
		if len(words) == 0 {
			if strings.Contains(cell, "Could not share") {
				t.Errorf("%s: the Share as image cell quotes a \"Could not share\" line for a card that could not "+
					"be read, but fallbackShareImage tells the reader nothing for it; update the cell or the code", row)
			}
			continue
		}
		for _, w := range words {
			if !strings.Contains(cell, w) {
				t.Errorf("%s: fallbackShareImage tells the reader %q for a card that could not be read, but the "+
					"Share as image cell, which names that case, does not say so; update the cell or the code", row, w)
			}
		}
	}
}
