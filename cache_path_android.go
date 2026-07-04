//go:build android

package bibletext

import "fyne.io/fyne/v2"

// appStorageDir returns a writable per-app directory on Android. Go's
// os.UserCacheDir() resolves to $HOME/.cache there, which is read-only for an
// app, so the Bible cache (cache.go) would otherwise fall back to an unwritable
// CWD-relative path. Fyne's app-storage root is the app's internal files dir,
// which is always writable. Nil-guarded so a failure just yields "" and the
// caller keeps its old (network-only) behaviour rather than crashing.
func appStorageDir() string {
	app := fyne.CurrentApp()
	if app == nil {
		return ""
	}
	st := app.Storage()
	if st == nil {
		return ""
	}
	root := st.RootURI()
	if root == nil {
		return ""
	}
	return root.Path()
}
