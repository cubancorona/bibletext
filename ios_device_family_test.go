package bibletext

// The iOS build reads its device families back out of what it built, since
// nothing else in the build would notice iPad support lost. The script is
// read to show it does, and the check is run on bundles made to pass and to
// fail.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// THE iOS BUILD READS ITS DEVICE FAMILIES BACK. UIDeviceFamily [1, 2] is the
// one key that makes the app universal, App Review refuses an update that
// drops a family it shipped, and Fyne's packager writes its own Info.plist,
// so release-ios.sh sets the key, and now reads it back from the archive
// (the upload path) and from the exported .ipa, before anything is
// published. Mutation: drop either read-back.
func TestReleaseIOSReadsItsDeviceFamiliesBack(t *testing.T) {
	releaseInOrder(t, "scripts/release-ios.sh",
		`plutil -replace UIDeviceFamily -json '[1, 2]'`,
		`check-ios-device-family.py" "$AAPP"`,
		`xcodebuild -exportArchive`,
		`check-ios-device-family.py" "$VAPP"`,
		`mv -f "$IPA_TMP" "$OUT_DIR/BibleText.ipa"`)
	if !strings.Contains(releaseCode(t, "scripts/release-ios.sh"), `FAMILY_FLAG="--iphone-only"`) {
		t.Error("release-ios.sh no longer reads back an iPhone-only build (BIBLETEXT_IPAD=0) as iPhone-only")
	}
}

func TestCheckIOSDeviceFamily(t *testing.T) {
	py := pythonForScripts(t)
	dir := t.TempDir()
	plist := func(name, families string) string {
		app := filepath.Join(dir, name+".app")
		if err := os.MkdirAll(app, 0o700); err != nil {
			t.Fatal(err)
		}
		body := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>CFBundleIdentifier</key><string>org.example.reader</string>` +
			families + `</dict></plist>`
		if err := os.WriteFile(filepath.Join(app, "Info.plist"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return app
	}
	family := func(ns ...string) string {
		return "<key>UIDeviceFamily</key><array><integer>" + strings.Join(ns, "</integer><integer>") +
			"</integer></array>"
	}
	for _, c := range []struct {
		name, app string
		args      []string
		ok        bool
	}{
		{"iPhone and iPad", plist("universal", family("1", "2")), nil, true},
		{"iPad and iPhone, in the other order", plist("reversed", family("2", "1")), nil, true},
		{"iPhone alone, for a universal build", plist("phone", family("1")), nil, false},
		{"iPad alone", plist("pad", family("2")), nil, false},
		{"no UIDeviceFamily", plist("none", ""), nil, false},
		{"a family repeated", plist("repeated", family("1", "2", "2")), nil, false},
		{"a family beyond iPhone and iPad", plist("more", family("1", "2", "6")), nil, false},
		{"iPhone alone, asked for", plist("phone-asked", family("1")), []string{"--iphone-only"}, true},
		{"iPhone and iPad, for an iPhone-only build", plist("universal-asked", family("1", "2")),
			[]string{"--iphone-only"}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := runScript(t, py, append([]string{"scripts/check-ios-device-family.py", c.app}, c.args...)...)
			if (err == nil) != c.ok {
				t.Errorf("check-ios-device-family.py: err=%v, want ok=%v\n%s", err, c.ok, out)
			}
		})
	}
}

// pythonForScripts is python3, which the release paths run the verifiers
// with; the Windows runner's own Python is not this test's concern.
func pythonForScripts(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the release paths run these verifiers on the macOS and Linux runners; those jobs run this")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Fatalf("no python3 on PATH, which every release path runs the verifier with: %v", err)
	}
	return py
}

func runScript(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return out.String(), err
}
