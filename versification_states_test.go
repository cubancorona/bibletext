package bibletext

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// THE NEXT RELEASE'S TABLES DIFFER FROM THE SHIPPING ONES ONLY IN THE GREEK
// ESTHER. scripts/gen-versification.py and gen-omitted-verses.py write each
// table twice from the same caches, once for the shipping build and once with
// --next for the next major release (docs/NEXT.md), and the only rule between
// them is the one that maps the Greek Esther verse for verse. Read from the
// generated files themselves, in both states of the switch, so a
// regeneration of either that changed anything else fails here.
func TestTheNextReleasesTablesDifferOnlyInTheGreekEsther(t *testing.T) {
	for _, tc := range []struct {
		current, next       string // file
		currentVar, nextVar string
		added, removed      int
		atLeast             int // entries in the shipping table, as a control
	}{
		// Three verses absent, forty-one its own, and no longer incommensurable.
		{"versification_data.go", "versification_data_next.go", "versificationDeltas", "nextVersificationDeltas", 44, 1, 200},
		// Its three gaps recorded as holes, in one entry for the book.
		{"omitted_verses_data.go", "omitted_verses_data_next.go", "omittedVerses", "nextOmittedVerses", 1, 0, 10},
	} {
		cur, next := generatedEntries(t, tc.current, tc.currentVar), generatedEntries(t, tc.next, tc.nextVar)
		var added, removed []string
		for e := range next {
			if !cur[e] {
				added = append(added, e)
			}
		}
		for e := range cur {
			if !next[e] {
				removed = append(removed, e)
			}
		}
		sort.Strings(added)
		sort.Strings(removed)
		for _, e := range append(append([]string(nil), added...), removed...) {
			if !strings.HasPrefix(e, "webc ") || !strings.Contains(e, `"Esther"`) {
				t.Errorf("%s and %s differ at %s, which is not WEB Catholic's Esther", tc.current, tc.next, e)
			}
		}
		if len(added) != tc.added || len(removed) != tc.removed {
			t.Errorf("%s adds %d entries to %s and drops %d, want %d and %d:\n+ %s\n- %s", tc.next, len(added),
				tc.current, len(removed), tc.added, tc.removed, strings.Join(added, "\n+ "), strings.Join(removed, "\n- "))
		}
		// CONTROL: the rest of each table is really there to be compared.
		if len(cur) < tc.atLeast {
			t.Errorf("%s yields only %d entries; the comparison proves nothing", tc.current, len(cur))
		}
	}
}

// generatedEntries reads the map literal declared as name in a generated
// table file into one string per leaf entry, "<edition> <field> <entry>" for a
// field of a struct value and "<edition> <entry>" otherwise, in the file's own
// spelling.
func generatedEntries(t *testing.T, path, name string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	text := func(n ast.Node) string {
		return string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
	}
	var table *ast.CompositeLit
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, sp := range g.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if n.Name == name && i < len(vs.Values) {
					table, _ = vs.Values[i].(*ast.CompositeLit)
				}
			}
		}
	}
	if table == nil {
		t.Fatalf("%s declares no map literal %s", path, name)
	}
	out := map[string]bool{}
	for _, el := range table.Elts {
		kv := el.(*ast.KeyValueExpr)
		edition := strings.Trim(text(kv.Key), `"`)
		for _, inner := range kv.Value.(*ast.CompositeLit).Elts {
			field, ok := inner.(*ast.KeyValueExpr)
			if !ok {
				t.Fatalf("%s: %s holds %s, not a keyed entry", path, edition, text(inner))
			}
			lit, ok := field.Value.(*ast.CompositeLit)
			if _, isIdent := field.Key.(*ast.Ident); isIdent && ok {
				// A struct field (absent, moved, extra, incommensurable): one
				// entry per element.
				for _, leaf := range lit.Elts {
					out[edition+" "+text(field.Key)+" "+text(leaf)] = true
				}
				continue
			}
			out[edition+" "+text(field)] = true
		}
	}
	return out
}
