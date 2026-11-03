package bibletext

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These cases execute the production Java editor with deterministic framework
// protocols. They cover its owner, draft and window transitions; they do not
// model Android rendering, real IME timing or accessibility announcements.
func TestNativeAndroidEditorLifecycle(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("Java compiler unavailable; native editor lifecycle checks require a JDK")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("Java runtime unavailable; native editor lifecycle checks require a JDK")
	}
	classes := t.TempDir()
	args := []string{"--release", "8", "-Xlint:-options", "-d", classes, noteEntryJava}
	fixtures := filepath.Join("testdata", "android_editor_lifecycle")
	err = filepath.WalkDir(fixtures, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".java") {
			args = append(args, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, javac, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile production editor and lifecycle fixtures: %v\n%s", err, out)
	}
	for _, name := range []string{
		"finishing-activity", "same-text-selection", "keyboard-refresh", "focus-refresh",
		"current-owner-callbacks", "stale-field-callbacks", "dismiss-already-removed",
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := exec.CommandContext(ctx, java, "-cp", classes, "org.bibletext.EditorLifecycleHarness", name).CombinedOutput()
			if err != nil {
				t.Fatalf("native editor lifecycle: %v\n%s", err, out)
			}
			if strings.TrimSpace(string(out)) != "PASS "+name {
				t.Fatalf("lifecycle fixture did not finish: %s", out)
			}
		})
	}
}
