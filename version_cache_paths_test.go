package bibletext

// CLEARING A VERSION'S CACHE HAS TO CLEAR EVERY EPOCH OF IT.
//

// 22 Aug 2026), and the way that button can be quietly wrong is by deleting
// only the CURRENT epoch. Every shipping translation has had at least one
// decoder bump, and loadVersionFromCacheOnly deliberately falls back through
// supersededCachePaths so an offline reader is never stranded by an upgrade —
// which means a leftover older file lets the app open the translation offline
// after its cache was "cleared". That looks exactly like the button not
// working, and it would send someone hunting in the wrong place.
//
// So this pins the property the button relies on: the current path plus the
// superseded ones enumerate every file a version can leave behind, and none of
// them collide.

import (
	"path/filepath"
	"testing"
)

func TestVersionCachePathsCoverEveryEpoch(t *testing.T) {
	for _, v := range registeredVersions {
		if v.cacheEpoch <= 0 {
			continue // no epochs, nothing superseded
		}
		cur := cachePathForVersion(v.ID)
		old := supersededCachePaths(v)

		// One file per superseded epoch, counting epoch 0 (the pre-epoch name).
		if got, want := len(old), v.cacheEpoch; got != want {
			t.Errorf("%s (epoch %d): %d superseded paths, want %d — a version that "+
				"has been bumped %d times can leave that many files behind, and a "+
				"clear that misses one still opens offline",
				v.ID, v.cacheEpoch, got, want, v.cacheEpoch)
		}

		seen := map[string]bool{cur: true}
		for _, p := range old {
			if seen[p] {
				t.Errorf("%s: %q appears twice in the paths to clear", v.ID, filepath.Base(p))
			}
			seen[p] = true
			if p == cur {
				t.Errorf("%s: a superseded path equals the current one (%q) — the "+
					"epoch suffix is not distinguishing them", v.ID, filepath.Base(p))
			}
		}
	}
}
