//go:build bibletextdev

package bibletext

import (
	"strings"
	"testing"
)

// The dev Links tab's scenario rows are a list a reader taps through, so
// every row must be tellable from the next, every URL must be something, and
// the licensed translation must keep its rows: the NKJV is the text with a
// heading over nearly every section, and the rows that open notes on it are
// how the heading air, the note band and the pill's centering are looked at
// there. Mutations: a duplicated label; an emptied URL; the NKJV rows
// removed or their version changed.
func TestDevScenariosAreDistinctAndKeepTheNKJV(t *testing.T) {
	seen := map[string]bool{}
	nkjv := 0
	for _, sc := range devScenarios() {
		if seen[sc.name] {
			t.Errorf("two dev scenarios share the label %q", sc.name)
		}
		seen[sc.name] = true
		if strings.TrimSpace(sc.url) == "" {
			t.Errorf("dev scenario %q has no URL", sc.name)
		}
		if strings.Contains(sc.url, "/nkjv/") {
			tgt, ok := ParseShareLink(sc.url)
			if !ok || tgt.VersionID != "nkjv" {
				t.Errorf("dev scenario %q names the NKJV in its URL but does not parse as an NKJV link", sc.name)
			}
			nkjv++
		}
	}
	if nkjv < 7 {
		t.Errorf("only %d dev scenarios open a note on the NKJV; want 7 (one plain, a three-note spread, a heading, a range, a poem)", nkjv)
	}
}
