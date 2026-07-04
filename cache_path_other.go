//go:build !android

package bibletext

// appStorageDir is an Android-only hook (see cache_path_android.go). On iOS and
// desktop, os.UserCacheDir() already resolves to a proper writable location, so
// defaultCachePath needs no override here.
func appStorageDir() string { return "" }
