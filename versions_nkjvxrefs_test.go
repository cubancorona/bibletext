//go:build nkjvxrefs

package bibletext

import "testing"

func TestNKJVXRefsTagEnablesPublisherApparatus(t *testing.T) {
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("missing nkjv in registeredVersions")
	}
	if !v.PublisherCrossRefs || !v.PublisherApparatus {
		t.Errorf("nkjv behind nkjvxrefs: PublisherCrossRefs=%v PublisherApparatus=%v, want both true",
			v.PublisherCrossRefs, v.PublisherApparatus)
	}
	for _, id := range []string{"web", "bsb", "webc"} {
		pv, ok := versionByID(id)
		if !ok {
			t.Fatalf("missing %s in registeredVersions", id)
		}
		if !pv.PublisherCrossRefs || pv.PublisherApparatus {
			t.Errorf("%s: PublisherCrossRefs=%v PublisherApparatus=%v, want true/false",
				id, pv.PublisherCrossRefs, pv.PublisherApparatus)
		}
	}
}
