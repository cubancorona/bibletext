package bibletext

// Every release path proves the atomic preferences writer reached what it
// built. The paths are read to show the check runs after their last compile,
// and the check itself is run against files made to pass and to fail.

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EVERY RELEASE PATH PROVES THE ATOMIC PREFERENCES WRITER REACHED ITS BINARY.
// patches/fyne-2.7.4-atomic-prefs.patch changes Fyne's
// app/preferences_nonweb.go, which every platform but the web builds, and it
// reaches a build only through that build's patch step. Stock Fyne truncates
// preferences.json before rewriting it, so a death mid-save empties every
// note. Only release-mac-store.sh checked the binary; now every path runs
// scripts/verify-atomic-prefs.py on what it built, after its last compile.
// Mutation: drop the verifier from any one path, or move it before the
// compile it should follow.
func TestEveryReleasePathChecksTheAtomicPreferencesWriter(t *testing.T) {
	const verifier = "verify-atomic-prefs.py"
	releaseInOrder(t, "scripts/release-ios.sh",
		`-o "$WORK/bibletext-arm64"`,
		verifier+`" "$WORK/bibletext-arm64"`,
		`mv -f "$WORK/bibletext-arm64"`)
	releaseInOrder(t, "scripts/build-android.sh",
		`fyne release -os android`,
		verifier+`" "$AAB_STAGE"`,
		verifier+`" "$APK_STAGE"`,
		`mv -f "$AAB_TMP" "$DIST_DIR/BibleText.aab"`)
	releaseInOrder(t, "scripts/release-mac-store.sh",
		`GOARCH=amd64 go build`,
		verifier+` "$WORK/BibleText-arm64" "$WORK/BibleText-amd64"`,
		`lipo -create -output "$WORK/BibleText"`)
	// The Microsoft Store's executable, after the packager's rebuild.
	releaseInOrder(t, "scripts/build-windows-exe.sh",
		`package -os windows`,
		verifier+` BibleText.exe`)
	// The Linux tarball's executable, as the release jobs package it.
	releaseInOrder(t, "scripts/check-linux-package.sh",
		`tar -xJf "$tarball"`,
		verifier+`" "${packaged_binaries[0]}"`)

	// Every workflow job that compiles the app checks what it compiled,
	// after its last compile, as the next guard requires of its own check.
	for path, known := range map[string][]string{
		".github/workflows/release.yml":      {"macos", "linux", "linux-arm64", "windows"},
		".github/workflows/linux-stores.yml": {"binary"},
	} {
		compiles := map[string]bool{}
		for name, job := range nextWorkflowJobs(nextReadFile(t, path)) {
			cmds := job.commands()
			last := -1
			for i, c := range cmds {
				if nextCompilesApp(c) {
					last = i
				}
			}
			if last < 0 {
				continue
			}
			compiles[name] = true
			checked := false
			for _, c := range cmds[last+1:] {
				checked = checked || strings.Contains(c, verifier) || strings.Contains(c, "check-linux-package.sh")
			}
			if !checked {
				t.Errorf("%s: job %s compiles the app and does not run scripts/%s on it afterwards",
					path, name, verifier)
			}
		}
		for _, name := range known {
			if !compiles[name] {
				t.Errorf("%s: the scan finds no compile in job %s; the per-job check cannot see it", path, name)
			}
		}
	}
	if !strings.Contains(releaseCode(t, ".github/workflows/msstore.yml"), "scripts/build-windows-exe.sh") {
		t.Error(".github/workflows/msstore.yml no longer builds through scripts/build-windows-exe.sh, " +
			"which is where its executable's preferences writer is checked")
	}
}

// THE VERIFIER REFUSES A STOCK WRITER, AND CANNOT PASS BY SEEING NOTHING.
// Files made to look like each kind of artifact: a binary with the patched
// writer's message and the control string passes; without the message it is
// refused; without the control it is refused as unreadable; a universal
// binary with one stock slice is refused although the other slice carries
// the message; and an .aab with one stock library, or with none at all, is
// refused.
func TestVerifyAtomicPrefsRefusesAStockWriter(t *testing.T) {
	py := pythonForScripts(t)
	dir := t.TempDir()
	const marker, control = "Preferences save not published", "World English Bible"
	patched := []byte("\x7fELF..." + control + "...log: " + marker + "; stored file left unchanged...")
	stock := []byte("\x7fELF..." + control + "...os.Create(storagePath)...")
	blind := []byte("\x7fELF..." + marker + "...")
	write := func(name string, b []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	universal := func(name string, slices ...[]byte) string {
		be := binary.BigEndian
		header := be.AppendUint32(be.AppendUint32(nil, 0xcafebabe), uint32(len(slices)))
		offset := 8 + 20*len(slices)
		var body []byte
		for i, s := range slices {
			header = be.AppendUint32(be.AppendUint32(header, uint32(0x0100000c+i)), 0)
			header = be.AppendUint32(be.AppendUint32(header, uint32(offset)), uint32(len(s)))
			header = be.AppendUint32(header, 0)
			body = append(body, s...)
			offset += len(s)
		}
		return write(name, append(header, body...))
	}
	bundle := func(name string, libs map[string][]byte) string {
		var buf bytes.Buffer
		w := zip.NewWriter(&buf)
		for lib, b := range libs {
			f, err := w.Create(lib)
			if err != nil {
				t.Fatal(err)
			}
			f.Write(b)
		}
		f, _ := w.Create("base/manifest/AndroidManifest.xml")
		f.Write([]byte("<manifest/>"))
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return write(name, buf.Bytes())
	}
	for _, c := range []struct {
		name, path, want string
		ok               bool
	}{
		{"a patched binary", write("patched", patched), "atomic preferences writer present", true},
		{"a stock binary", write("stock", stock), "lacks the atomic preferences writer", false},
		{"a binary the search cannot read", write("blind", blind), "cannot be trusted", false},
		{"a universal binary, both slices patched", universal("both", patched, patched), "cputype", true},
		{"a universal binary with one stock slice", universal("half", patched, stock),
			"lacks the atomic preferences writer", false},
		{"an .aab, every library patched", bundle("good.aab", map[string][]byte{
			"base/lib/arm64-v8a/libbibletext.so": patched, "base/lib/x86_64/libbibletext.so": patched}),
			"libbibletext.so", true},
		{"an .aab with one stock library", bundle("half.aab", map[string][]byte{
			"base/lib/arm64-v8a/libbibletext.so": patched, "base/lib/x86_64/libbibletext.so": stock}),
			"lacks the atomic preferences writer", false},
		{"an .apk with no native library", bundle("empty.apk", nil), "no native library", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, err := runScript(t, py, "scripts/verify-atomic-prefs.py", c.path)
			if (err == nil) != c.ok {
				t.Fatalf("verify-atomic-prefs.py %s: err=%v, want ok=%v\n%s", c.name, err, c.ok, out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("verify-atomic-prefs.py %s said:\n%s\nwant it to say %q", c.name, out, c.want)
			}
		})
	}
}
