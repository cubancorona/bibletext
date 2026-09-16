package bibletext

import (
	"os"
	"strings"
	"testing"
)

// readSourceFile reads a source file with its line endings normalised: a
// Windows checkout can carry CRLF, and a test that scans for "\n}\n" or a
// first line must not care.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
