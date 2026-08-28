package bibletext

import (
	"os"
	"strings"
	"testing"
)

// Android's Auto Backup includes the app's files/ directory by default, so a
// Bible cache stored there is uploaded to the reader's Google Drive backup and
// restored onto every device they later set up. For the licensed translation
// that is a copy of the publisher's text leaving the device, and it also
// defeats purgeUnavailableLicensedCaches, which can delete an on-device copy
// but has no reach into a cloud one.
//
// The caches therefore live in no_backup/. Nothing here compiles for Android
// and CI runs no Go tests for it, so this pins the decision in the one place
// that is checked on every platform: the source itself. A change that drops
// the relocation has to delete this test to land, which is the point.
func TestAndroidCachesStayOutOfBackedUpStorage(t *testing.T) {
	src, err := os.ReadFile("cache_path_android.go")
	if err != nil {
		t.Fatalf("read cache_path_android.go: %v", err)
	}
	body := string(src)

	for _, want := range []string{
		`"no_backup", "bibletext"`,      // the relocation itself
		`filepath.Base(dir) == "files"`, // anchored on the real Android layout
		"MkdirAll",                      // no_backup/ may not exist yet
	} {
		if !strings.Contains(body, want) {
			t.Errorf("cache_path_android.go no longer contains %q — the Bible caches "+
				"would return to backed-up storage, sending licensed text off-device", want)
		}
	}

	// CONTROL: the sweep must be able to fail.
	if strings.Contains(body, "this string is not in the file") {
		t.Fatal("the control string matched; this test proves nothing")
	}
}
