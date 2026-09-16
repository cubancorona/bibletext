package main

import (
	"encoding/json"
	"encoding/xml"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const repo = "../.."

// The committed tiles are the generator's own output from icon/full.png: a
// stray, missing, resized or hand-edited file fails here, and the fix is to
// run `go run ./cmd/msstore assets` again.
func TestCommittedTilesAreTheGeneratorsOutput(t *testing.T) {
	checkRendered(t, filepath.Join(repo, "msstore", "Assets"), assetTable(), "assets")
}

func TestCommittedListingLogosAreTheGeneratorsOutput(t *testing.T) {
	checkRendered(t, filepath.Join(repo, "msstore", "listing"), listingTable(), "listing")
}

func checkRendered(t *testing.T, dir string, table []asset, command string) {
	t.Helper()
	icon, err := loadIcon(filepath.Join(repo, "icon", "full.png"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]asset{}
	for _, a := range table {
		want[a.Name] = a
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := want[e.Name()]; !ok {
			t.Errorf("%s is not a file the %s table names", e.Name(), command)
		}
	}
	for _, a := range table {
		f, err := os.Open(filepath.Join(dir, a.Name))
		if err != nil {
			t.Errorf("missing tile: %v", err)
			continue
		}
		got, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", a.Name, err)
			continue
		}
		if b := got.Bounds(); b.Dx() != a.Width || b.Dy() != a.Height {
			t.Errorf("%s is %dx%d, the table says %dx%d", a.Name, b.Dx(), b.Dy(), a.Width, a.Height)
			continue
		}
		fresh := renderAsset(icon, a)
	pixels:
		for y := 0; y < a.Height; y++ {
			for x := 0; x < a.Width; x++ {
				r1, g1, b1, a1 := got.At(x, y).RGBA()
				r2, g2, b2, a2 := fresh.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Errorf("%s differs from a fresh render at (%d,%d); regenerate with `go run ./cmd/msstore %s`", a.Name, x, y, command)
					break pixels
				}
			}
		}
	}
}

// filledManifest is the manifest as the runner fills it: template, reserved
// identity and the desktop ledger's version.
func filledManifest(t *testing.T) (string, identity, string) {
	t.Helper()
	tmpl, err := os.ReadFile(filepath.Join(repo, "msstore", "AppxManifest.xml.in"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := loadIdentity(filepath.Join(repo, "msstore", "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := packageVersion(filepath.Join(repo, "cmd", "desktop", "FyneApp.toml"))
	if err != nil {
		t.Fatal(err)
	}
	filled, err := fillManifest(string(tmpl), id, version)
	if err != nil {
		t.Fatal(err)
	}
	return filled, id, version
}

func TestManifestFillsFromTheLedgerAndTheReservedIdentity(t *testing.T) {
	filled, id, version := filledManifest(t)
	if !regexp.MustCompile(`^\d+\.\d+\.\d+\.0$`).MatchString(version) {
		t.Fatalf("package version %q is not x.y.z.0 (the Store reserves the fourth part)", version)
	}
	var doc struct {
		Identity struct {
			Name      string `xml:"Name,attr"`
			Publisher string `xml:"Publisher,attr"`
			Version   string `xml:"Version,attr"`
		} `xml:"Identity"`
		Properties struct {
			PublisherDisplayName string `xml:"PublisherDisplayName"`
		} `xml:"Properties"`
		Capabilities struct {
			Capability []struct {
				Name string `xml:"Name,attr"`
			} `xml:"Capability"`
		} `xml:"Capabilities"`
	}
	if err := xml.Unmarshal([]byte(filled), &doc); err != nil {
		t.Fatalf("filled manifest is not XML: %v", err)
	}
	if doc.Identity.Name != id.IdentityName || doc.Identity.Publisher != id.IdentityPublisher || doc.Identity.Version != version {
		t.Errorf("Identity = %+v, want %s / %s / %s", doc.Identity, id.IdentityName, id.IdentityPublisher, version)
	}
	if doc.Properties.PublisherDisplayName != id.PublisherDisplayName {
		t.Errorf("PublisherDisplayName = %q, want %q", doc.Properties.PublisherDisplayName, id.PublisherDisplayName)
	}
	if len(doc.Capabilities.Capability) != 1 || doc.Capabilities.Capability[0].Name != "runFullTrust" {
		t.Errorf("capabilities = %+v, want runFullTrust alone", doc.Capabilities.Capability)
	}
	for _, must := range []string{
		`MinVersion="10.0.19041.0"`,
		`uap10:RuntimeBehavior="packagedClassicApp"`,
		`uap10:TrustLevel="mediumIL"`,
		`Executable="BibleText.exe"`,
	} {
		if !strings.Contains(filled, must) {
			t.Errorf("manifest lacks %s", must)
		}
	}
	// Every tile the manifest names has qualifier-named files in the table.
	bases := map[string]bool{}
	for _, a := range assetTable() {
		bases[strings.SplitN(a.Name, ".", 2)[0]] = true
	}
	for _, m := range regexp.MustCompile(`Assets\\([A-Za-z0-9]+)\.png`).FindAllStringSubmatch(filled, -1) {
		if !bases[m[1]] {
			t.Errorf("manifest names Assets\\%s.png but the asset table has no such tile", m[1])
		}
	}
}

func TestFillRefusesALeftoverPlaceholder(t *testing.T) {
	id := identity{IdentityName: "n", IdentityPublisher: "CN=x", PublisherDisplayName: "d"}
	if _, err := fillManifest(`<x a="__IDENTITY_NAME__" b="__NOT_A_FIELD__"/>`, id, "1.0.0.0"); err == nil {
		t.Fatal("a placeholder the filler does not know survived")
	}
}

func TestIdentityLoaderRejectsAPublisherWithoutCN(t *testing.T) {
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, []byte(`{"identityName":"n","identityPublisher":"97067B4D","publisherDisplayName":"d"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadIdentity(path); err == nil {
		t.Fatal("a publisher that is not the CN=… value was accepted")
	}
}

func TestPackageVersionNeedsAThreePartLedgerVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "FyneApp.toml")
	if err := os.WriteFile(path, []byte("[Details]\nVersion = \"1.2\"\nBuild = 50\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := packageVersion(path); err == nil {
		t.Fatal("a two-part version was accepted")
	}
}

// The two link handlers: the web-to-app handler for our host alone (www
// redirects to the apex and is deliberately absent) with the URL passed on
// the command line, and the bibletext: scheme, both under one application.
func TestManifestDeclaresTheLinkHandlers(t *testing.T) {
	filled, _, _ := filledManifest(t)
	var doc struct {
		Ignorable    string `xml:"IgnorableNamespaces,attr"`
		Applications struct {
			Application []struct {
				Extensions struct {
					Extension []struct {
						Category      string `xml:"Category,attr"`
						AppUriHandler *struct {
							// The namespace is the point: Windows honours the
							// attribute only in desktop2, so a tidied prefix must fail.
							Parameters string `xml:"http://schemas.microsoft.com/appx/manifest/desktop/windows10/2 Parameters,attr"`
							Host       []struct {
								Name string `xml:"Name,attr"`
							} `xml:"Host"`
						} `xml:"AppUriHandler"`
						Protocol *struct {
							Name       string `xml:"Name,attr"`
							Parameters string `xml:"Parameters,attr"`
						} `xml:"Protocol"`
					} `xml:"Extension"`
				} `xml:"Extensions"`
			} `xml:"Application"`
		} `xml:"Applications"`
	}
	if err := xml.Unmarshal([]byte(filled), &doc); err != nil {
		t.Fatalf("filled manifest is not XML: %v", err)
	}
	if n := len(doc.Applications.Application); n != 1 {
		t.Fatalf("%d applications, want 1", n)
	}
	var uri, proto int
	for _, e := range doc.Applications.Application[0].Extensions.Extension {
		switch e.Category {
		case "windows.appUriHandler":
			uri++
			if e.AppUriHandler == nil || len(e.AppUriHandler.Host) != 1 || e.AppUriHandler.Host[0].Name != "bibletext.co.uk" {
				t.Errorf("appUriHandler = %+v, want exactly the host bibletext.co.uk", e.AppUriHandler)
			}
			if e.AppUriHandler != nil && e.AppUriHandler.Parameters != "%1" {
				t.Errorf("desktop2:Parameters = %q, want %%1: without it the URL never reaches os.Args", e.AppUriHandler.Parameters)
			}
		case "windows.protocol":
			proto++
			if e.Protocol == nil || e.Protocol.Name != "bibletext" || e.Protocol.Parameters != `"%1"` {
				t.Errorf("protocol = %+v, want Name=bibletext Parameters=\"%%1\"", e.Protocol)
			}
		}
	}
	if uri != 1 || proto != 1 {
		t.Errorf("%d appUriHandler and %d protocol extensions, want one of each", uri, proto)
	}
	for _, ns := range []string{"uap3", "desktop2"} {
		if !slices.Contains(strings.Fields(doc.Ignorable), ns) {
			t.Errorf("IgnorableNamespaces %q lacks %s", doc.Ignorable, ns)
		}
	}
	for _, must := range []string{
		`xmlns:uap3="http://schemas.microsoft.com/appx/manifest/uap/windows10/3"`,
		`xmlns:desktop2="http://schemas.microsoft.com/appx/manifest/desktop/windows10/2"`,
		`<uap3:Protocol Name="bibletext" Parameters="&quot;%1&quot;">`,
	} {
		if !strings.Contains(filled, must) {
			t.Errorf("manifest lacks %s", must)
		}
	}
}

// The site's consent file for the Windows handler names the reserved package
// family and claims exactly the paths the Apple file claims, excluding
// exactly what it excludes — one scope on both platforms.
func TestWindowsAppWebLinkAgreesWithTheAppleFileAndTheIdentity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repo, "docs", "windows-app-web-link"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		PackageFamilyName string   `json:"packageFamilyName"`
		Paths             []string `json:"paths"`
		ExcludePaths      []string `json:"excludePaths"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("docs/windows-app-web-link is not the documented JSON array: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("%d entries, want one package", len(entries))
	}
	id, err := loadIdentity(filepath.Join(repo, "msstore", "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].PackageFamilyName != id.PackageFamilyName {
		t.Errorf("packageFamilyName %q, identity says %q", entries[0].PackageFamilyName, id.PackageFamilyName)
	}
	apple, err := os.ReadFile(filepath.Join(repo, "docs", "apple-app-site-association"))
	if err != nil {
		t.Fatal(err)
	}
	var aasa struct {
		Applinks struct {
			Details []struct {
				Components []struct {
					Path    string `json:"/"`
					Exclude bool   `json:"exclude"`
				} `json:"components"`
			} `json:"details"`
		} `json:"applinks"`
	}
	if err := json.Unmarshal(apple, &aasa); err != nil {
		t.Fatal(err)
	}
	var claimed, excluded []string
	for _, d := range aasa.Applinks.Details {
		for _, c := range d.Components {
			if c.Exclude {
				excluded = append(excluded, c.Path)
			} else {
				claimed = append(claimed, c.Path)
			}
		}
	}
	if len(claimed) == 0 || len(excluded) == 0 {
		t.Fatalf("the Apple file yielded %d claims and %d exclusions; the comparison proves nothing", len(claimed), len(excluded))
	}
	if !slices.Equal(entries[0].Paths, claimed) {
		t.Errorf("paths %q, the Apple file claims %q", entries[0].Paths, claimed)
	}
	if !slices.Equal(entries[0].ExcludePaths, excluded) {
		t.Errorf("excludePaths %q, the Apple file excludes %q", entries[0].ExcludePaths, excluded)
	}
	if _, err := os.Stat(filepath.Join(repo, "docs", "windows-app-web-link.json")); err == nil {
		t.Error("docs/windows-app-web-link.json exists; Windows ignores the file with a .json suffix")
	}
}
