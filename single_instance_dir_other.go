//go:build !windows && !linux

package bibletext

import (
	"os"
	"path/filepath"
)

// singleInstanceDir on the Mac (dev builds under BIBLETEXT_MIMIC are the only
// callers): the user's cache directory, beside the Bible cache.
func singleInstanceDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bibletext"), nil
}
