package bibletext

// tintMulti IS WIRED AND MUST NOT BE REACHABLE
// (docs/NOTES_SPEC.md#overlap-and-tint-contract).
//
// The palette tokens, the table rows and the stylesheet rule for the multi-note
// wash all exist, so that turning it on is one deliberate change to chapterTint
// and nothing else. Until that change is deliberately made, ONE LIT SPAN AT A
// TIME is the recorded invariant — a chapter never carries two washes — and
// this test is what keeps a refactor from breaking that by accident:
// chapterTint is the one tint source every renderer consults (tint.go), so
// proving tintMulti cannot flow out of tint.go's answer proves no surface can
// draw it.
//
// The proof is an AST walk rather than behaviour: behaviour can only sample
// states, and the invariant is about ALL states. The identifier tintMulti may
// appear in exactly four places, all in tint.go — its declaration and its three
// table rows (overridesTextColour, wash, htmlClass). Anywhere else — above all
// inside chapterTint, or in a composite literal in another file — is a wiring,
// and a wiring must arrive as a decision that updates this list, not as a side
// effect.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestTintMultiIsUnreachableToday(t *testing.T) {
	allowed := map[string]bool{
		"tint.go:const":               true, // the declaration itself
		"tint.go:overridesTextColour": true,
		"tint.go:wash":                true,
		"tint.go:htmlClass":           true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var uses []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, decl := range f.Decls {
			where := name + ":const"
			if fd, ok := decl.(*ast.FuncDecl); ok {
				where = name + ":" + fd.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && id.Name == "tintMulti" {
					uses = append(uses, where)
				}
				return true
			})
		}
	}

	if len(uses) == 0 {
		// The guard must fail LOUDLY if the constant is renamed out from under
		// it — a guard that can go vacuously green guards nothing.
		t.Fatal("no reference to tintMulti anywhere — if it was renamed, rename it in this guard too")
	}
	sort.Strings(uses)
	declared := false
	for _, u := range uses {
		if u == "tint.go:const" {
			declared = true
		}
		if !allowed[u] {
			t.Errorf("tintMulti is referenced at %s — outside its declaration and its three "+
				"table rows. That is a WIRING: the one-lit-span invariant says nothing may "+
				"draw a second wash unless the invariant is changed on purpose. If this is "+
				"that decision, move the "+
				"invariant deliberately: widen chapterTint, update this guard, and revalidate "+
				"the palette on a rendered page (docs/NOTES_SPEC.md#overlap-and-tint-contract).", u)
		}
	}
	if !declared {
		t.Error("tintMulti's declaration is no longer in tint.go's const block")
	}
}
