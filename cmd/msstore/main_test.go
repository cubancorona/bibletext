package main

import (
	"encoding/xml"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
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

func TestManifestFillsFromTheLedgerAndTheReservedIdentity(t *testing.T) {
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
	if !regexp.MustCompile(`^\d+\.\d+\.\d+\.0$`).MatchString(version) {
		t.Fatalf("package version %q is not x.y.z.0 (the Store reserves the fourth part)", version)
	}
	filled, err := fillManifest(string(tmpl), id, version)
	if err != nil {
		t.Fatal(err)
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
