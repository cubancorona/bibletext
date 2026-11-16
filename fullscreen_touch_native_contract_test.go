package bibletext

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Ordinary host tests cannot compile either native mobile pane. Both must
// put the full-screen native host under the owner that supplies bounded taps.
func TestLandscapeTouchTargetsWireBothNativeReadingBodies(t *testing.T) {
	for _, path := range []string{"reading_ios.go", "reading_android.go"} {
		t.Run(path, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var body *ast.BlockStmt
			for _, decl := range file.Decls {
				if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "buildReadingViewMobile" {
					body = f.Body
				}
			}
			if body == nil {
				t.Fatal("native reading builder is absent")
			}
			calls, guarded := 0, 0
			ast.Inspect(body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "fullScreenReadingBody" {
						calls++
					}
				}
				branch, ok := node.(*ast.IfStmt)
				if !ok {
					return true
				}
				condition, ok := branch.Cond.(*ast.CallExpr)
				if !ok {
					return true
				}
				method, ok := condition.Fun.(*ast.SelectorExpr)
				if !ok || method.Sel.Name != "readingFullScreen" {
					return true
				}
				ast.Inspect(branch.Body, func(child ast.Node) bool {
					ret, ok := child.(*ast.ReturnStmt)
					if !ok || len(ret.Results) != 1 {
						return true
					}
					call, ok := ret.Results[0].(*ast.CallExpr)
					if !ok {
						return true
					}
					name, ok := call.Fun.(*ast.Ident)
					if !ok || name.Name != "fullScreenReadingBody" {
						return true
					}
					if len(call.Args) != 2 {
						t.Fatal("full-screen helper has lost state or native content")
					}
					stack, ok := call.Args[1].(*ast.CallExpr)
					if !ok || len(stack.Args) != 2 {
						t.Fatal("full-screen helper no longer receives paper plus native host")
					}
					for i, want := range []string{"paper", "host"} {
						arg, ok := stack.Args[i].(*ast.Ident)
						if !ok || arg.Name != want {
							t.Fatalf("native content argument %d is not %s", i, want)
						}
					}
					guarded++
					return true
				})
				return true
			})
			if calls != 1 || guarded != 1 {
				t.Fatalf("native helper calls=%d, full-screen returns=%d; want one guarded native-body call", calls, guarded)
			}
		})
	}
}
