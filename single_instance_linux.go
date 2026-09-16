//go:build linux

package bibletext

import (
	"os"
	"path/filepath"
)

// singleInstanceDir prefers $XDG_RUNTIME_DIR: per session, private to the
// user, and cleared at logout, so a crashed instance's record never outlives
// the session. Without one (a bare X session, a container) the cache
// directory serves, where a stale record is removed by the next forwarder.
func singleInstanceDir() (string, error) {
	if rt := os.Getenv("XDG_RUNTIME_DIR"); rt != "" {
		return filepath.Join(rt, "bibletext"), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bibletext"), nil
}
