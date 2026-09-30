//go:build windows

package bibletext

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// ON WINDOWS THE PICTURE GETS NO EMAIL…: a mailto: link carries no file, so
// the image sheet withholds the button, and a compose with a file fails
// without opening anything. Mutation: the withAttachment check dropped.
func TestWindowsOffersNoEmailForThePicture(t *testing.T) {
	if shareEmailAvailable(true) {
		t.Error("image: Email… must be withheld on Windows")
	}
	prev := externalOpener
	opened := 0
	externalOpener = func(*url.URL) error { opened++; return nil }
	t.Cleanup(func() { externalOpener = prev })
	if err := composeShareEmail("John 1:1 (Sample)", "text", `C:\nowhere\card.png`); err == nil {
		t.Error("a compose with a file must fail")
	}
	if opened != 0 {
		t.Errorf("a compose with a file opened %d links", opened)
	}
}

// registerFixtureScheme registers a URL scheme for the current user with
// the values given on its open verb's command key — "" the command line,
// "DelegateExecute" the CLSID of the COM handler the verb runs instead,
// which Windows reads from that key and not from the verb's own — and
// removes it when the test ends.
func registerFixtureScheme(t *testing.T, scheme string, command map[string]string) {
	t.Helper()
	root := `Software\Classes\` + scheme
	k, _, err := registry.CreateKey(registry.CURRENT_USER, root, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	k.SetStringValue("", "URL:"+scheme)
	k.SetStringValue("URL Protocol", "")
	k.Close()
	t.Cleanup(func() {
		for _, sub := range []string{`\shell\open\command`, `\shell\open`, `\shell`, ""} {
			registry.DeleteKey(registry.CURRENT_USER, root+sub)
		}
	})
	c, _, err := registry.CreateKey(registry.CURRENT_USER, root+`\shell\open\command`, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for name, value := range command {
		if err := c.SetStringValue(name, value); err != nil {
			t.Fatal(err)
		}
	}
}

// settingsDelegateExecute is the CLSID of the COM handler the Settings
// app's ms-settings: scheme opens through, read from the machine, so the
// packaged fixture names a handler that is really registered, on the key a
// packaged app's scheme carries its own on; "" where there is none.
func settingsDelegateExecute() string {
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `ms-settings\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	clsid, _, err := k.GetStringValue("DelegateExecute")
	if err != nil {
		return ""
	}
	return clsid
}

// THE SHELL'S OWN ANSWER DECIDES WHETHER A HANDLER EXISTS: a scheme whose
// handler's executable is on disk has one; one registered to an executable
// that is gone, as an uninstalled client leaves it, has none; a packaged
// app's, whose command key holds no command line and a DelegateExecute
// handler, has one; a scheme nobody registered has none. Mutations: the
// key's presence taken for a handler (the stale case passes); the
// DelegateExecute query dropped (the packaged case fails).
func TestTheShellSaysWhetherASchemeHasAHandler(t *testing.T) {
	notepad := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	if _, err := os.Stat(notepad); err != nil {
		t.Skipf("no %s to stand in for a mail client: %v", notepad, err)
	}
	registerFixtureScheme(t, "x-bibletext-fixture-live", map[string]string{"": `"` + notepad + `" "%1"`})
	registerFixtureScheme(t, "x-bibletext-fixture-stale", map[string]string{"": `"C:\no\such\fixture-mail.exe" "%1"`})
	packaged := settingsDelegateExecute()
	if packaged != "" {
		registerFixtureScheme(t, "x-bibletext-fixture-packaged", map[string]string{"DelegateExecute": packaged})
	}
	for _, c := range []struct {
		scheme string
		want   bool
	}{
		{"x-bibletext-fixture-live", true},
		{"x-bibletext-fixture-stale", false},
		{"x-bibletext-fixture-packaged", true},
		{"x-bibletext-fixture-unregistered", false},
	} {
		t.Run(c.scheme, func(t *testing.T) {
			if c.scheme == "x-bibletext-fixture-packaged" && packaged == "" {
				t.Skip(`ms-settings has no DelegateExecute on its open verb's command key, so there is no registered handler for the packaged fixture to name`)
			}
			if got := schemeHandlerRegistered(c.scheme); got != c.want {
				t.Errorf("schemeHandlerRegistered(%q) = %v, want %v", c.scheme, got, c.want)
			}
		})
	}
}
