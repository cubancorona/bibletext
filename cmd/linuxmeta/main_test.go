package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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
	outs, err := renderAll(repo, loadInputs(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(outs) < 9 {
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
	// The Flathub variant says it is keyless; the keyed one must not.
	flathub := readFile(t, filepath.Join("linux", "flathub", id+".metainfo.xml"))
	if !strings.Contains(flathub, in.Listing.KeylessNote) || strings.Contains(meta, in.Listing.KeylessNote) {
		t.Error("the keyless note is not on exactly the Flathub metainfo")
	}
}

func TestSummaryFitsEveryStore(t *testing.T) {
	s := loadInputs(t).Listing.Summary
	if len(s) > 35 || len(s) > 78 {
		t.Errorf("summary %q is %d characters; Flathub allows 35, the Snap Store 78", s, len(s))
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
		t.Fatal("validate accepted a summary that breaks Flathub's rules")
	}
}

// The offline Flatpak build vendors every module go.mod requires: each has
// an archive in flatpak/go.mod.yml and a module line in flatpak/modules.txt,
// and the toolkit entry is the version the tracked patches were made for.
func TestFlatpakSourcesCoverGoMod(t *testing.T) {
	gomod := readFile(t, "go.mod")
	sources := readFile(t, filepath.Join("flatpak", "go.mod.yml"))
	modules := readFile(t, filepath.Join("flatpak", "modules.txt"))
	req := regexp.MustCompile(`(?m)^\s*([A-Za-z0-9./_\-]+)\s+(v[0-9][^\s]*)`)
	n := 0
	for _, m := range req.FindAllStringSubmatch(gomod, -1) {
		mod, ver := m[1], m[2]
		if mod == "go" || mod == "module" || mod == "toolchain" {
			continue
		}
		n++
		if !strings.Contains(sources, "dest: vendor/"+mod+"\n") {
			t.Errorf("flatpak/go.mod.yml has no archive for %s", mod)
		}
		if !strings.Contains(modules, "# "+mod+" "+ver+"\n") {
			t.Errorf("flatpak/modules.txt has no line for %s %s", mod, ver)
		}
	}
	if n < 10 {
		t.Fatalf("only %d requirements parsed from go.mod; the check proves little", n)
	}
	if !strings.Contains(modules, "# fyne.io/fyne/v2 v2.7.4\n") {
		t.Error("the vendored toolkit is not v2.7.4, which the patches under patches/ were made for")
	}
	if strings.Contains(gomod, "\nreplace ") {
		t.Error("go.mod carries a replace; the Flatpak sources were generated from the stock file and would no longer match")
	}
	// And nothing vendored that go.mod no longer requires.
	for _, m := range regexp.MustCompile(`(?m)^# (\S+) (v\S+)$`).FindAllStringSubmatch(modules, -1) {
		if !strings.Contains(gomod, m[1]+" "+m[2]) {
			t.Errorf("flatpak/modules.txt vendors %s %s, which go.mod does not require; regenerate with flatpak-go-mod", m[1], m[2])
		}
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

// The manifest applies every tracked toolkit patch, in the order the setup
// script does, and no patch the tree does not have.
func TestManifestAppliesEveryTrackedPatch(t *testing.T) {
	manifest := readFile(t, filepath.Join("flatpak", loadInputs(t).Listing.AppstreamID+".yml"))
	tracked, err := filepath.Glob(filepath.Join(repo, "patches", "fyne-2.7.4-*.patch"))
	if err != nil || len(tracked) == 0 {
		t.Fatalf("no toolkit patches found: %v", err)
	}
	if len(tracked) != len(fynePatchOrder)+len(fynePatchesNotOnLinux) {
		t.Errorf("%d tracked toolkit patches, the manifest applies %d and %d are declared not to apply on Linux",
			len(tracked), len(fynePatchOrder), len(fynePatchesNotOnLinux))
	}
	// A patch excused from the Linux build must say why, and must really be
	// absent from the manifest rather than quietly listed in both places.
	for name, why := range fynePatchesNotOnLinux {
		if why == "" {
			t.Errorf("%s is excused from the Flatpak build with no reason given", name)
		}
		if slices.Contains(fynePatchOrder, name) {
			t.Errorf("%s is both applied and excused", name)
		}
		if strings.Contains(manifest, "patches/"+name) {
			t.Errorf("%s is excused from the Flatpak build but the manifest applies it", name)
		}
	}
	// The order is the setup script's: the first mention of each patch file
	// in scripts/setup-fyne-patch.sh must come in the same sequence.
	script := readFile(t, filepath.Join("scripts", "setup-fyne-patch.sh"))
	last := -1
	for _, p := range fynePatchOrder {
		i := strings.Index(script, p)
		if i < 0 {
			t.Errorf("scripts/setup-fyne-patch.sh does not apply %s", p)
			continue
		}
		if i < last {
			t.Errorf("%s is applied in a different order from scripts/setup-fyne-patch.sh", p)
		}
		last = i
	}
	last = -1
	for _, p := range fynePatchOrder {
		line := "patch -p1 -d vendor/fyne.io/fyne/v2 < patches/" + p + "\n"
		i := strings.Index(manifest, line)
		if i < 0 {
			t.Errorf("the manifest does not apply %s", p)
			continue
		}
		if i < last {
			t.Errorf("%s is applied out of order", p)
		}
		last = i
	}
	for _, p := range tracked {
		name := filepath.Base(p)
		if _, excused := fynePatchesNotOnLinux[name]; excused {
			continue
		}
		if !strings.Contains(manifest, "patches/"+name+"\n") {
			t.Errorf("tracked patch %s is neither applied by the manifest nor declared in fynePatchesNotOnLinux", name)
		}
	}
	if !strings.Contains(manifest, "go build -tags flatpak") || strings.Contains(manifest, "-X ") {
		t.Error("the Flathub build must use the flatpak tag and carry no linker value")
	}
}

// The tarball's desktop entry comes from the ledger's own table: it must say
// what the listing says, or the four Linux channels disagree on the menu.
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
	if _, err := toml.DecodeFile(filepath.Join(repo, "cmd", "desktop", "FyneApp.toml"), &ledger); err != nil {
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
	if strings.Contains(renderMetainfo(in, false), "<screenshots>") {
		t.Fatal("a <screenshots> block was rendered with no captured screenshots")
	}
	ready := in
	ready.Listing.Screenshots.Ref = "0123456789abcdef0123456789abcdef01234567"
	if !strings.Contains(renderMetainfo(ready, false), "<screenshot type=\"default\">") {
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
