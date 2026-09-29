package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

const repo = "../.."

func loadInputs(t *testing.T) inputs {
	t.Helper()
	in, err := readInputs(repo)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// Every committed output is the generator's own render of the sources: a
// hand edit, a stale render or a stray file fails here, and the fix is to
// run `go run ./cmd/linuxmeta render` again.
func TestCommittedFilesAreTheGeneratorsOutput(t *testing.T) {
	outs, err := renderAll(repo, loadInputs(t), snapArches[defaultSnapArch])
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) < 7 {
		t.Fatalf("%d outputs rendered; the comparison covers less than it should", len(outs))
	}
	for _, o := range outs {
		path := filepath.Join(repo, o.Path)
		if o.Image == nil {
			got, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("%s: %v", o.Path, err)
				continue
			}
			if strings.ReplaceAll(string(got), "\r\n", "\n") != o.Text {
				t.Errorf("%s differs from a fresh render; run `go run ./cmd/linuxmeta render`", o.Path)
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("%s: %v", o.Path, err)
			continue
		}
		got, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", o.Path, err)
			continue
		}
		if got.Bounds() != o.Image.Bounds() {
			t.Errorf("%s is %v, want %v", o.Path, got.Bounds(), o.Image.Bounds())
			continue
		}
	pixels:
		for y := 0; y < got.Bounds().Dy(); y++ {
			for x := 0; x < got.Bounds().Dx(); x++ {
				r1, g1, b1, a1 := got.At(x, y).RGBA()
				r2, g2, b2, a2 := o.Image.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Errorf("%s differs from a fresh render at (%d,%d)", o.Path, x, y)
					break pixels
				}
			}
		}
	}
}

func TestNewestReleaseIsTheLedgerVersionAndDatesDescend(t *testing.T) {
	in := loadInputs(t)
	if in.Releases[0].Version != in.Version {
		t.Fatalf("newest release %s, ledger %s", in.Releases[0].Version, in.Version)
	}
	prev := time.Now().AddDate(0, 0, 1)
	prevVersion := ""
	for _, r := range in.Releases {
		if prevVersion != "" && !semverLess(r.Version, prevVersion) {
			t.Errorf("%s follows %s; versions must descend", r.Version, prevVersion)
		}
		prevVersion = r.Version
		d, err := time.Parse("2006-01-02", r.Date)
		if err != nil {
			t.Fatalf("%s: %v", r.Version, err)
		}
		if d.After(time.Now()) {
			t.Errorf("%s is dated %s, in the future", r.Version, r.Date)
		}
		if d.After(prev) {
			t.Errorf("%s (%s) is newer than the entry above it", r.Version, r.Date)
		}
		prev = d
		if (r.Head == "") != (len(r.Bullets) == 0) && r.Head == "" {
			t.Errorf("%s has bullets but no first line", r.Version)
		}
	}
}

func TestDesktopEntriesAgreeWithTheMetainfo(t *testing.T) {
	in := loadInputs(t)
	id := in.Listing.AppstreamID
	linuxEntry := readFile(t, filepath.Join("linux", id+".desktop"))
	snapEntry := readFile(t, filepath.Join("snap", "gui", in.Listing.Executable+".desktop"))
	for _, line := range []string{
		"Exec=" + in.Listing.Executable + " %u",
		"Categories=" + joinList(in.Listing.Categories),
		"Keywords=" + joinList(in.Listing.Keywords),
		"Comment=" + in.Listing.Summary,
		"MimeType=" + in.Listing.SchemeMediatype + ";",
		"StartupWMClass=" + in.Product.ProductName,
	} {
		for name, entry := range map[string]string{"linux": linuxEntry, "snap": snapEntry} {
			if !strings.Contains(entry, "\n"+line+"\n") {
				t.Errorf("%s desktop entry lacks the line %q", name, line)
			}
		}
	}
	meta := readFile(t, filepath.Join("linux", id+".metainfo.xml"))
	if !strings.Contains(meta, "<launchable type=\"desktop-id\">"+id+".desktop</launchable>") {
		t.Error("the metainfo does not launch the desktop entry")
	}
	if !strings.Contains(meta, "<mediatype>"+in.Listing.SchemeMediatype+"</mediatype>") {
		t.Error("the metainfo does not provide the scheme mediatype")
	}
	for _, c := range in.Listing.Categories {
		if !strings.Contains(meta, "<category>"+c+"</category>") {
			t.Errorf("the metainfo lacks category %s", c)
		}
	}
}

func TestSummaryFitsEveryStore(t *testing.T) {
	s := loadInputs(t).Listing.Summary
	if len(s) > 35 || len(s) > 78 {
		t.Errorf("summary %q is %d characters; the metainfo allows 35, the Snap Store 78", s, len(s))
	}
	if strings.HasSuffix(s, ".") {
		t.Errorf("summary %q ends with a full stop", s)
	}
	for _, article := range []string{"A ", "An ", "The "} {
		if strings.HasPrefix(s, article) {
			t.Errorf("summary %q starts with an article", s)
		}
	}
	if _, err := readInputs(repo); err != nil {
		t.Fatal(err)
	}
	bad := loadInputs(t)
	bad.Listing.Summary = "A quiet, fast Bible reader with everything you need."
	if validate(bad) == nil {
		t.Fatal("validate accepted a summary that breaks the listing's rules")
	}
}

func semverLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3 && i < len(pa) && i < len(pb); i++ {
		var x, y int
		fmt.Sscanf(pa[i], "%d", &x)
		fmt.Sscanf(pb[i], "%d", &y)
		if x != y {
			return x < y
		}
	}
	return false
}

// The tarball's desktop entry comes from the ledger's own table: it must say
// what the listing says, or the three Linux channels disagree on the menu.
func TestLedgerLinuxTableMatchesTheListing(t *testing.T) {
	var ledger struct {
		LinuxAndBSD struct {
			GenericName string   `toml:"GenericName"`
			Comment     string   `toml:"Comment"`
			Categories  []string `toml:"Categories"`
			Keywords    []string `toml:"Keywords"`
		} `toml:"LinuxAndBSD"`
		CanOpen struct {
			MimeTypes string `toml:"MimeTypes"`
		} `toml:"CanOpen"`
	}
	if _, err := toml.DecodeFile(filepath.Join(repo, "cmd", "bibletext", "FyneApp.toml"), &ledger); err != nil {
		t.Fatal(err)
	}
	l := loadInputs(t).Listing
	if ledger.LinuxAndBSD.GenericName != l.GenericName || ledger.LinuxAndBSD.Comment != l.Summary {
		t.Errorf("ledger GenericName/Comment %q/%q, listing %q/%q", ledger.LinuxAndBSD.GenericName, ledger.LinuxAndBSD.Comment, l.GenericName, l.Summary)
	}
	if joinList(ledger.LinuxAndBSD.Categories) != joinList(l.Categories) || joinList(ledger.LinuxAndBSD.Keywords) != joinList(l.Keywords) {
		t.Error("the ledger's Categories or Keywords differ from the listing")
	}
	if ledger.CanOpen.MimeTypes != l.SchemeMediatype+";" {
		t.Errorf("ledger CanOpen %q, listing scheme %q", ledger.CanOpen.MimeTypes, l.SchemeMediatype)
	}
}

func TestListingDocQuotesTheListing(t *testing.T) {
	doc := readFile(t, filepath.Join("docs", "LINUX_STORES.md"))
	l := loadInputs(t).Listing
	for _, must := range []string{l.AppstreamID, "`" + l.Summary + "`", l.Executable + " %u"} {
		if !strings.Contains(doc, must) {
			t.Errorf("docs/LINUX_STORES.md does not mention %q", must)
		}
	}
}

func TestScreenshotsAreLeftOutUntilCaptured(t *testing.T) {
	in := loadInputs(t)
	if screenshotsReady(in.Listing) {
		if _, err := os.Stat(filepath.Join(repo, "docs", "screenshots", "linux", in.Listing.Screenshots.Item[0].File)); err != nil {
			t.Fatalf("the listing names a screenshots commit but the files are absent: %v", err)
		}
		return
	}
	if strings.Contains(renderMetainfo(in), "<screenshots>") {
		t.Fatal("a <screenshots> block was rendered with no captured screenshots")
	}
	ready := in
	ready.Listing.Screenshots.Ref = "0123456789abcdef0123456789abcdef01234567"
	if !strings.Contains(renderMetainfo(ready), "<screenshot type=\"default\">") {
		t.Fatal("a named commit did not render the screenshots")
	}
}

// readFile reads a repository file with CRLF normalised: .gitattributes asks
// for LF everywhere, but a test must not depend on the checkout's settings.
func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, rel))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// The snap's ALSA layout binds a real path, and the path is architecture
// specific. This is the defect it exists for: an arm64 snap rendered with the
// x86_64 triplet builds, packs, installs and LAUNCHES perfectly, and then has
// no narration audio — nothing in a build or a launch smoke can see it, because
// nothing goes looking for a sound plugin until a reader presses play.
//
// So every architecture must bind its own triplet, and must NOT carry another
// architecture's. Asserting only the first half would pass on a template that
// emitted both.
func TestEverySnapArchitectureBindsItsOwnAlsaPath(t *testing.T) {
	in := loadInputs(t)
	for name, a := range snapArches {
		got := renderSnapcraft(in, a)

		wantPlatform := "platforms:\n  " + name + ":"
		if !strings.Contains(got, wantPlatform) {
			t.Errorf("%s: snapcraft.yaml does not declare %q; snapcraft would build the wrong architecture",
				name, wantPlatform)
		}

		wantBind := "bind: $SNAP/usr/lib/" + a.Triplet + "/alsa-lib"
		if !strings.Contains(got, wantBind) {
			t.Errorf("%s: no layout binding %s — the snap would ship without reachable ALSA plugins",
				name, a.Triplet)
		}

		for other, b := range snapArches {
			if other == name {
				continue
			}
			if strings.Contains(got, b.Triplet) {
				t.Errorf("%s: carries %s's triplet %q as well; snapd binds a path that does not exist on this architecture",
					name, other, b.Triplet)
			}
		}
	}
}

// The committed snapcraft.yaml is one architecture's render, so the flag has to
// actually change something. A generator that ignored -arch would satisfy the
// test above for whichever arch it hard-coded.
func TestTheArchitectureFlagChangesTheRender(t *testing.T) {
	in := loadInputs(t)
	amd := renderSnapcraft(in, snapArches["amd64"])
	arm := renderSnapcraft(in, snapArches["arm64"])
	if amd == arm {
		t.Fatal("amd64 and arm64 render identically; the architecture is being ignored")
	}
	if !strings.Contains(amd, "x86_64-linux-gnu") || !strings.Contains(arm, "aarch64-linux-gnu") {
		t.Error("the renders do not carry their own triplets")
	}
}
