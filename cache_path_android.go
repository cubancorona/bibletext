//go:build android

package bibletext

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

// appStorageDir returns a writable per-app directory for the Bible caches on
// Android. Go's os.UserCacheDir() resolves to $HOME/.cache there, which is
// read-only for an app, so the cache (cache.go) would otherwise fall back to
// an unwritable CWD-relative path. Nil-guarded so a failure just yields "" and
// the caller keeps its old (network-only) behaviour rather than crashing.
//
// WHY NOT THE APP-STORAGE ROOT ITSELF. Fyne's storage root is the app's
// internal files/ directory, and Android's Auto Backup includes files/ by
// default — so the cached text was being uploaded to the reader's Google Drive
// backup and copied onto every device they later set up. For the licensed
// translation that is a copy of Thomas Nelson's text leaving the device
// entirely, and it also defeated purgeUnavailableLicensedCaches, which can
// delete the on-device copy but has no reach into a cloud one.
//
// no_backup/ is the directory Android provides for exactly this: persistent
// like files/, never included in backup or device-to-device transfer, and NOT
// cleared under storage pressure the way cache/ is — which matters for a whole
// Bible a reader expects to work offline. It sits beside files/, so it is
// derived from the storage root rather than needing a new JNI call.
//
// Preferences are untouched and stay in files/: the notes scrapbook is
// irreplaceable, shared notes exist nowhere else, and it should keep surviving
// a lost phone.
func appStorageDir() string {
	root := fyneStorageRoot()
	if root == "" {
		return ""
	}
	// .../files/fyne -> .../no_backup. Walk up to the package directory rather
	// than assuming a depth: the root is Fyne's own subdirectory of files/.
	dir := root
	for i := 0; i < 4; i++ {
		parent := filepath.Dir(dir)
		if parent == dir || parent == "/" {
			break
		}
		if filepath.Base(dir) == "files" {
			noBackup := filepath.Join(parent, "no_backup", "bibletext")
			if err := os.MkdirAll(noBackup, 0o700); err != nil {
				break // fall through to the old location rather than failing
			}
			return noBackup
		}
		dir = parent
	}
	// The layout was not what we expected. Keeping the old location is the
	// safe answer: the app still works, and the containment gap is reported
	// by the manifest guard rather than by a crash here.
	return root
}

func fyneStorageRoot() string {
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
