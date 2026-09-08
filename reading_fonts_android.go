//go:build android

package bibletext

// PUTTING THE SHIPPED READING FACES WHERE ANDROID CAN LOAD THEM.
//
// The reading overlay here is a native TextView, and a TextView takes a
// Typeface built from a FILE. The faces are compiled into the binary
// (reading_fonts_embed.go), so they are written out once into the app's own
// storage and loaded from there.
//
// Written rather than shipped as APK assets, because the assets would be a
// SECOND copy: this platform draws its Scripture in the native overlay, not in
// the toolkit's pane, so the embedded bytes would otherwise sit unused in the
// binary while a duplicate rode along beside them.
//
// The directory is one Java can find without being told. Go's storage here is
// <package>/no_backup/bibletext, and getNoBackupFilesDir() gives Java the same
// <package>/no_backup — so the bridge looks in bibletext/fonts under it.

import (
	"os"
	"path/filepath"
	"sync"
)

var androidFontsOnce sync.Once

// androidReadingFontDir is where the faces are written, or "" if they could not
// be written. Failing is not fatal: the bridge finds no fonts, keeps the
// platform serif it uses today, and the pane reads exactly as it did before.
func androidReadingFontDir() string {
	var dir string
	androidFontsOnce.Do(func() {
		root := appStorageDir()
		if root == "" {
			return
		}
		d := filepath.Join(root, "fonts")
		if err := os.MkdirAll(d, 0o700); err != nil {
			return
		}
		for name, b := range map[string][]byte{
			"Junicode-Regular.ttf":    readingFontRegular,
			"Junicode-Italic.ttf":     readingFontItalic,
			"Junicode-Bold.ttf":       readingFontBold,
			"Junicode-BoldItalic.ttf": readingFontBoldItalic,
			"EzraSIL-Regular.ttf":     readingFontHebrew,
		} {
			if len(b) == 0 {
				continue
			}
			p := filepath.Join(d, name)
			// Rewrite only when the file is missing or a different size. The
			// faces change when the app is updated, and a stale one would be
			// loaded forever otherwise; comparing bytes every launch would
			// read seven megabytes to learn nothing.
			if st, err := os.Stat(p); err == nil && st.Size() == int64(len(b)) {
				continue
			}
			tmp := p + ".tmp"
			if err := os.WriteFile(tmp, b, 0o600); err != nil {
				continue
			}
			// Rename, so a launch interrupted mid-write cannot leave a torn
			// font behind for the bridge to load.
			_ = os.Rename(tmp, p)
		}
		dir = d
	})
	return dir
}
