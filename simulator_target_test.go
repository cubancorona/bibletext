package bibletext

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise the packaging script's real selector against a fake inventory.
// An explicit target must never install over another session's booted app.
func TestSimulatorPackagingHonorsAnExplicitTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("iOS simulator tooling requires a Unix shell")
	}
	raw, err := os.ReadFile("scripts/run-ios-sim.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	start := strings.Index(script, "DEVICE_UDID=\"\"\n")
	end := strings.Index(script, "echo \"==> opening Simulator.app\"")
	if start < 0 || end <= start {
		t.Fatal("simulator selector boundaries missing")
	}
	const own = "11111111-1111-4111-8111-111111111111"
	const other = "22222222-2222-4222-8222-222222222222"
	for _, tc := range []struct {
		name, requested, want string
		fail, boot            bool
	}{
		{"explicit", "Fixture phone", own, false, true},
		{"missing", "Missing phone", "", true, false},
		{"default", "", other, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mock := "#!/bin/sh\ncase \"$*\" in\n" +
				"'simctl list devices available') printf '%s\\n' '-- iOS 27.0 --' '    Fixture phone (" + own + ") (Shutdown)' '    Other phone (" + other + ") (Booted)' ;;\n" +
				"'simctl list devices booted') printf '%s\\n' '-- iOS 27.0 --' '    Other phone (" + other + ") (Booted)' ;;\n" +
				"'simctl boot " + own + "') echo BOOT_OWN ;;\n" +
				"*) exit 91 ;;\nesac\n"
			if err := os.WriteFile(filepath.Join(dir, "xcrun"), []byte(mock), 0700); err != nil {
				t.Fatal(err)
			}
			command := "set -euo pipefail\nudid_re='[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}'\nDEVICE_NAME=\"${BIBLETEXT_SIM_DEVICE:-Fixture phone}\"\n" +
				script[start:end] + "\nprintf 'TARGET=%s\\n' \"$BOOTED\"\n"
			cmd := exec.Command("bash", "-c", command)
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "BIBLETEXT_SIM_DEVICE="+tc.requested)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.fail {
				t.Fatalf("selector error = %v, want failure %v: %s", err, tc.fail, output)
			}
			if !tc.fail && !strings.Contains(string(output), "TARGET="+tc.want) {
				t.Errorf("wrong installation target: %s", output)
			}
			if strings.Contains(string(output), "BOOT_OWN") != tc.boot {
				t.Errorf("unexpected boot operation: %s", output)
			}
		})
	}
}
