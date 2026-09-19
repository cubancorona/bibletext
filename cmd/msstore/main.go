// Command msstore prepares the inputs of the Microsoft Store package: the
// tile assets scaled from the shipped icon (cmd/bibletext/Icon.png, the mark
// every other channel carries), and the AppxManifest filled from the
// desktop ledger (cmd/bibletext/FyneApp.toml) and the identity Partner Center
// assigned when the name was reserved (msstore/identity.json).
//
//	go run ./cmd/msstore assets
//	go run ./cmd/msstore listing
//	go run ./cmd/msstore manifest -out build/msstore/layout/AppxManifest.xml
//
// The packaging itself (makepri, makeappx) runs on the Windows runner; see
// .github/workflows/msstore.yml and docs/WINDOWS_STORE_LISTING.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// asset is one tile the manifest names, at one scale or target size.
type asset struct {
	Name   string // file name under Assets/
	Width  int
	Height int
}

// assetTable lists every tile the manifest references, at the scale factors
// the Store's certification expects (100/125/150/200/400) plus the target
// sizes Windows uses for the taskbar, Start and the file system, each also in
// the unplated form so the icon is drawn without a colour plate.
// listingTable lists the logos the Store listing itself takes (not part of
// the package): the 300x300 tile and the 1:1 box art.
func listingTable() []asset {
	return []asset{
		{"store-logo-300.png", 300, 300},
		{"box-art-1080.png", 1080, 1080},
	}
}

func assetTable() []asset {
	var out []asset
	square := func(base string, size int) {
		for _, s := range []struct {
			scale int
			px    int
		}{{100, size}, {125, size * 125 / 100}, {150, size * 150 / 100}, {200, size * 2}, {400, size * 4}} {
			px := s.px
			if s.scale == 125 && size*125%100 != 0 {
				px++ // 187.5 → 188, 62.5 → 63: the Store rounds half up
			}
			out = append(out, asset{fmt.Sprintf("%s.scale-%d.png", base, s.scale), px, px})
		}
	}
	square("Square44x44Logo", 44)
	for _, t := range []int{16, 24, 32, 48, 256} {
		out = append(out, asset{fmt.Sprintf("Square44x44Logo.targetsize-%d.png", t), t, t})
		out = append(out, asset{fmt.Sprintf("Square44x44Logo.targetsize-%d_altform-unplated.png", t), t, t})
	}
	square("Square150x150Logo", 150)
	square("StoreLogo", 50)
	for _, s := range []struct{ scale, w, h int }{{100, 310, 150}, {125, 388, 188}, {150, 465, 225}, {200, 620, 300}, {400, 1240, 600}} {
		out = append(out, asset{fmt.Sprintf("Wide310x150Logo.scale-%d.png", s.scale), s.w, s.h})
	}
	return out
}

// renderAsset scales the square source icon (the shipped mark) to the asset's size. A wide tile
// keeps the icon square, scaled to the tile's height and centred on a
// transparent canvas, so the tile's own background colour shows either side.
func renderAsset(src image.Image, a asset) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, a.Width, a.Height))
	side := a.Height
	if a.Width < side {
		side = a.Width
	}
	x0 := (a.Width - side) / 2
	y0 := (a.Height - side) / 2
	target := image.Rect(x0, y0, x0+side, y0+side)
	xdraw.CatmullRom.Scale(dst, target, src, src.Bounds(), draw.Over, nil)
	return dst
}

func loadIcon(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() {
		return nil, fmt.Errorf("%s: the source icon must be square, got %dx%d", path, b.Dx(), b.Dy())
	}
	return img, nil
}

func writeAssets(icon image.Image, dir string, table []asset) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, a := range table {
		f, err := os.Create(filepath.Join(dir, a.Name))
		if err != nil {
			return err
		}
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(f, renderAsset(icon, a)); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

// identity is what Partner Center assigns at name reservation (Product
// management → Product identity). None of it is secret; the manifest's
// Identity element must carry it verbatim or the Store rejects the package.
type identity struct {
	IdentityName         string `json:"identityName"`
	IdentityPublisher    string `json:"identityPublisher"`
	PublisherDisplayName string `json:"publisherDisplayName"`
	PackageFamilyName    string `json:"packageFamilyName"`
}

func loadIdentity(path string) (identity, error) {
	var id identity
	data, err := os.ReadFile(path)
	if err != nil {
		return id, err
	}
	if err := json.Unmarshal(data, &id); err != nil {
		return id, fmt.Errorf("%s: %w", path, err)
	}
	switch {
	case id.IdentityName == "":
		return id, fmt.Errorf("%s: identityName is empty", path)
	case !strings.HasPrefix(id.IdentityPublisher, "CN="):
		return id, fmt.Errorf("%s: identityPublisher must be the CN=… value Partner Center shows, got %q", path, id.IdentityPublisher)
	case id.PublisherDisplayName == "":
		return id, fmt.Errorf("%s: publisherDisplayName is empty", path)
	case !strings.HasPrefix(id.PackageFamilyName, id.IdentityName+"_"):
		return id, fmt.Errorf("%s: packageFamilyName must be the identity name plus the Store's suffix, got %q", path, id.PackageFamilyName)
	}
	return id, nil
}

// ledgerVersion tolerates a Windows checkout: git may hand the runner CRLF
// line endings, and a strict $ would then find no Version line at all.
var ledgerVersion = regexp.MustCompile(`(?m)^Version = "(\d+)\.(\d+)\.(\d+)"\r?$`)

// packageVersion derives the four-part MSIX version from the desktop ledger.
// The Store reserves the fourth part and requires it to be 0, so a package
// re-uploaded for the same app version needs a patch bump: one version, one
// tree, on every channel.
func packageVersion(ledgerPath string) (string, error) {
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return "", err
	}
	m := ledgerVersion.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("%s: no Version = \"x.y.z\" line", ledgerPath)
	}
	return fmt.Sprintf("%s.%s.%s.0", m[1], m[2], m[3]), nil
}

var placeholder = regexp.MustCompile(`__[A-Z_]+__`)

// fillManifest substitutes the identity and version into the template and
// refuses a result that still carries a placeholder.
func fillManifest(tmpl string, id identity, version, arch string) (string, error) {
	r := strings.NewReplacer(
		"__IDENTITY_NAME__", id.IdentityName,
		"__IDENTITY_PUBLISHER__", id.IdentityPublisher,
		"__PUBLISHER_DISPLAY_NAME__", id.PublisherDisplayName,
		"__VERSION__", version,
		"__PROCESSOR_ARCHITECTURE__", arch,
	)
	out := r.Replace(tmpl)
	if left := placeholder.FindAllString(out, -1); left != nil {
		return "", fmt.Errorf("manifest template still carries %s", strings.Join(left, ", "))
	}
	return out, nil
}

func repoRelative(parts ...string) string {
	return filepath.Join(parts...)
}

func runAssets(args []string) error {
	fs := flag.NewFlagSet("assets", flag.ContinueOnError)
	icon := fs.String("icon", repoRelative("cmd", "bibletext", "Icon.png"), "square source icon")
	out := fs.String("out", repoRelative("msstore", "Assets"), "directory the tiles are written to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	img, err := loadIcon(*icon)
	if err != nil {
		return err
	}
	return writeAssets(img, *out, assetTable())
}

func runListing(args []string) error {
	fs := flag.NewFlagSet("listing", flag.ContinueOnError)
	icon := fs.String("icon", repoRelative("cmd", "bibletext", "Icon.png"), "square source icon")
	out := fs.String("out", repoRelative("msstore", "listing"), "directory the listing logos are written to")
	if err := fs.Parse(args); err != nil {
		return err
	}
	img, err := loadIcon(*icon)
	if err != nil {
		return err
	}
	return writeAssets(img, *out, listingTable())
}

func runManifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ContinueOnError)
	tmplPath := fs.String("template", repoRelative("msstore", "AppxManifest.xml.in"), "manifest template")
	idPath := fs.String("identity", repoRelative("msstore", "identity.json"), "identity Partner Center assigned")
	ledger := fs.String("ledger", repoRelative("cmd", "bibletext", "FyneApp.toml"), "desktop version ledger")
	out := fs.String("out", "", "where to write AppxManifest.xml (required)")
	// Windows on ARM runs x64 packages under emulation, so a mislabelled
	// manifest does not fail — it installs and runs slowly, forever.
	arch := fs.String("arch", "x64", "package processor architecture (x64 or arm64)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("manifest: -out is required")
	}
	if *arch != "x64" && *arch != "arm64" {
		return fmt.Errorf("manifest: -arch %q; known: x64, arm64", *arch)
	}
	tmpl, err := os.ReadFile(*tmplPath)
	if err != nil {
		return err
	}
	id, err := loadIdentity(*idPath)
	if err != nil {
		return err
	}
	version, err := packageVersion(*ledger)
	if err != nil {
		return err
	}
	filled, err := fillManifest(string(tmpl), id, version, *arch)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(*out, []byte(filled), 0o644)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: msstore assets|listing|manifest [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "assets":
		err = runAssets(os.Args[2:])
	case "listing":
		err = runListing(os.Args[2:])
	case "manifest":
		err = runManifest(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "msstore:", err)
		os.Exit(1)
	}
}
