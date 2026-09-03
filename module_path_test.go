package bibletext

import (
	"os"
	"strings"
	"testing"
)

// The module path is the repository's import path, not a bare name.
//
// A bare `module bibletext` builds fine from a checkout — including the tagged
// clone that `fyne install github.com/cubancorona/bibletext/cmd/desktop@latest`
// (the command the Fyne apps directory prints) makes and builds inside — and
// fails for anyone who resolves the module by its path instead: `go install`
// and `go run` of github.com/cubancorona/bibletext/cmd/desktop@latest stop at
// "module declares its path as: bibletext but was required as:
// github.com/cubancorona/bibletext". @latest resolves to the newest tag, so the
// path reaches those routes from the first tag cut after it; this guards the
// line, and docs/VERSIONING.md governs the tags.
func TestModulePathIsTheRepositoryImportPath(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	first = strings.TrimRight(first, "\r") // a Windows checkout may carry CRLF
	if want := "module github.com/cubancorona/bibletext"; first != want {
		t.Fatalf("go.mod begins %q, want %q — the module must declare its import path, or `go install …@latest` and `go run …@latest` cannot resolve it", first, want)
	}
}
