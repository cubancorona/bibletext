package bibletext

import (
	"os"
	"strings"
	"testing"
)

// The module path is the repository's import path, not a bare name.
//
// A bare `module bibletext` builds fine from a checkout and fails for anyone
// arriving from outside it: `go run github.com/cubancorona/bibletext/cmd/desktop@latest`
// (the install command the Fyne apps directory prints) stops at "module
// declares its path as: bibletext but was required as:
// github.com/cubancorona/bibletext". The path only takes effect for @latest
// from the next tagged release, so this guards the line rather than the
// experience; the release checklist covers the tag.
func TestModulePathIsTheRepositoryImportPath(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	if want := "module github.com/cubancorona/bibletext"; first != want {
		t.Fatalf("go.mod begins %q, want %q — a bare module name breaks `go run …@latest` and `fyne install …@latest` for everyone outside a checkout", first, want)
	}
}
