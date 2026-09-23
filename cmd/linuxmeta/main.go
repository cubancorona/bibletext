// Command linuxmeta renders the Linux packaging inputs from one listing
// source, the way cmd/msstore renders the Windows package inputs: the
// AppStream MetaInfo (a keyed variant for the snap and the AppImage, a
// keyless one for Flathub), the desktop entries, the icons, snap/snapcraft.yaml
// and the Flatpak manifest.
//
//	go run ./cmd/linuxmeta render
//	go run ./cmd/linuxmeta flatpak-manifest -tag v1.2.10 -commit <sha>   # the flathub-repo copy
//
// linux/listing.toml is the source; config/product.json the identity;
// cmd/bibletext/FyneApp.toml the version; linux/releases.toml the history. The
// tests in this package hold every committed output equal to a fresh render.
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
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	xdraw "golang.org/x/image/draw"
)

type product struct {
	ProductName string `json:"productName"`
	SiteBase    string `json:"siteBase"`
	SourceRepo  string `json:"sourceRepo"`
}

type screenshotItem struct {
	File    string `toml:"file"`
	Caption string `toml:"caption"`
}

type listing struct {
	AppstreamID     string            `toml:"appstream_id"`
	Executable      string            `toml:"executable"`
	Summary         string            `toml:"summary"`
	GenericName     string            `toml:"generic_name"`
	DeveloperID     string            `toml:"developer_id"`
	Developer       string            `toml:"developer"`
	MetadataLicense string            `toml:"metadata_license"`
	ProjectLicense  string            `toml:"project_license"`
	Categories      []string          `toml:"categories"`
	Keywords        []string          `toml:"keywords"`
	BrandLight      string            `toml:"brand_light"`
	BrandDark       string            `toml:"brand_dark"`
	SchemeMediatype string            `toml:"scheme_mediatype"`
	Intro           string            `toml:"intro"`
	Features        []string          `toml:"features"`
	Outro           string            `toml:"outro"`
	KeylessNote     string            `toml:"keyless_note"`
	OARS            map[string]string `toml:"oars"`
	Screenshots     struct {
		Ref  string           `toml:"ref"`
		Item []screenshotItem `toml:"item"`
	} `toml:"screenshots"`
}

type release struct {
	Version string   `toml:"version"`
	Date    string   `toml:"date"`
	Head    string   `toml:"head"`
	Bullets []string `toml:"bullets"`
}

type releasesFile struct {
	Release []release `toml:"release"`
}

// inputs is everything a render reads, resolved relative to the repository.
type inputs struct {
	Product  product
	Listing  listing
	Releases []release
	Version  string
	Patches  []string // the Fyne patches, in the order scripts/setup-fyne-patch.sh applies them
}

// fynePatchOrder is the order scripts/setup-fyne-patch.sh applies the
// toolkit patches; the Flatpak build applies them to the vendored module in
// the same order. TestManifestAppliesEveryTrackedPatch holds it to patches/,
// allowing for fynePatchesNotOnLinux.
var fynePatchOrder = []string{
	"fyne-2.7.4-ios-drawloop.patch",
	"fyne-2.7.4-caret-blink.patch",
	"fyne-2.7.4-noto-emoji.patch",
	"fyne-2.7.4-android-newintent.patch",
	"fyne-2.7.4-android-night-mode.patch",
	"fyne-2.7.4-atomic-prefs.patch",
}

// fynePatchesNotOnLinux are the tracked toolkit patches the Flatpak build
// deliberately leaves out, with the reason. A patch listed here is still
// applied by scripts/setup-fyne-patch.sh everywhere else; it simply has
// nothing to do on this platform.
var fynePatchesNotOnLinux = map[string]string{
	"fyne-2.7.4-windows-egl.patch":         "Windows only: it routes the OpenGL ES context through EGL so ANGLE can render with Direct3D",
	"fyne-2.7.4-ios-scene-lifecycle.patch": "iOS only: it changes darwin_ios.m, the UIKit app delegate, which no Linux build compiles",
}

var ledgerVersion = regexp.MustCompile(`(?m)^Version = "(\d+\.\d+\.\d+)"\r?$`)

func readInputs(repo string) (inputs, error) {
	var in inputs
	data, err := os.ReadFile(filepath.Join(repo, "config", "product.json"))
	if err != nil {
		return in, err
	}
	if err := json.Unmarshal(data, &in.Product); err != nil {
		return in, fmt.Errorf("config/product.json: %w", err)
	}
	if _, err := toml.DecodeFile(filepath.Join(repo, "linux", "listing.toml"), &in.Listing); err != nil {
		return in, fmt.Errorf("linux/listing.toml: %w", err)
	}
	var rel releasesFile
	if _, err := toml.DecodeFile(filepath.Join(repo, "linux", "releases.toml"), &rel); err != nil {
		return in, fmt.Errorf("linux/releases.toml: %w", err)
	}
	in.Releases = rel.Release
	ledger, err := os.ReadFile(filepath.Join(repo, "cmd", "bibletext", "FyneApp.toml"))
	if err != nil {
		return in, err
	}
	m := ledgerVersion.FindSubmatch(ledger)
	if m == nil {
		return in, errors.New("cmd/bibletext/FyneApp.toml: no Version = \"x.y.z\" line")
	}
	in.Version = string(m[1])
	in.Patches = fynePatchOrder
	return in, validate(in)
}

func validate(in inputs) error {
	l := in.Listing
	switch {
	case l.AppstreamID == "" || strings.Count(l.AppstreamID, ".") < 2:
		return fmt.Errorf("appstream_id %q is not a reverse-DNS id", l.AppstreamID)
	case len(l.Summary) > 35 || strings.HasSuffix(l.Summary, "."):
		return fmt.Errorf("summary %q: Flathub wants at most 35 characters and no full stop", l.Summary)
	case len(l.Features) == 0 || len(l.Categories) == 0 || len(l.Keywords) == 0:
		return errors.New("the listing needs features, categories and keywords")
	case len(in.Releases) == 0 || in.Releases[0].Version != in.Version:
		return fmt.Errorf("the newest release in linux/releases.toml must be the ledger's %s", in.Version)
	}
	// The desktop entry and the snap description are line-oriented: a stray
	// newline or list separator in a value would silently change a field.
	flat := append([]string{l.Summary, l.GenericName, l.Intro, l.Outro, l.KeylessNote}, l.Features...)
	for _, s := range flat {
		if strings.ContainsAny(s, "\r\n") {
			return fmt.Errorf("a listing value carries a line break: %q", s)
		}
	}
	for _, s := range append(append([]string{}, l.Categories...), l.Keywords...) {
		if strings.ContainsAny(s, ";\r\n") || s == "" {
			return fmt.Errorf("category or keyword %q must be one plain word or phrase", s)
		}
	}
	return nil
}

func esc(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// screenshotsReady reports whether the listing names the commit that holds
// docs/screenshots/linux/; until then no <screenshots> block is rendered.
func screenshotsReady(l listing) bool {
	ref := l.Screenshots.Ref
	return len(ref) == 40 && strings.Trim(ref, "0") != "" && len(l.Screenshots.Item) > 0
}

func renderMetainfo(in inputs, keyless bool) string {
	l, p := in.Listing, in.Product
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	w("<!-- Rendered by cmd/linuxmeta from linux/listing.toml; edit the source, not this file. -->\n")
	w("<component type=\"desktop-application\">\n")
	w("  <id>%s</id>\n", esc(l.AppstreamID))
	w("  <metadata_license>%s</metadata_license>\n", esc(l.MetadataLicense))
	w("  <project_license>%s</project_license>\n", esc(l.ProjectLicense))
	w("  <name>%s</name>\n", esc(p.ProductName))
	w("  <summary>%s</summary>\n", esc(l.Summary))
	w("  <developer id=\"%s\">\n    <name>%s</name>\n  </developer>\n", esc(l.DeveloperID), esc(l.Developer))
	w("  <description>\n")
	w("    <p>%s</p>\n", esc(l.Intro))
	w("    <ul>\n")
	for _, f := range l.Features {
		w("      <li>%s</li>\n", esc(f))
	}
	w("    </ul>\n")
	w("    <p>%s</p>\n", esc(l.Outro))
	if keyless {
		w("    <p>%s</p>\n", esc(l.KeylessNote))
	}
	w("  </description>\n")
	w("  <launchable type=\"desktop-id\">%s.desktop</launchable>\n", esc(l.AppstreamID))
	w("  <url type=\"homepage\">%s/</url>\n", esc(p.SiteBase))
	w("  <url type=\"bugtracker\">%s/issues</url>\n", esc(p.SourceRepo))
	w("  <url type=\"help\">%s/support.html</url>\n", esc(p.SiteBase))
	w("  <url type=\"contact\">%s/support.html</url>\n", esc(p.SiteBase))
	w("  <url type=\"vcs-browser\">%s</url>\n", esc(p.SourceRepo))
	w("  <categories>\n")
	for _, c := range l.Categories {
		w("    <category>%s</category>\n", esc(c))
	}
	w("  </categories>\n")
	w("  <keywords>\n")
	for _, k := range l.Keywords {
		w("    <keyword>%s</keyword>\n", esc(k))
	}
	w("  </keywords>\n")
	w("  <provides>\n    <mediatype>%s</mediatype>\n  </provides>\n", esc(l.SchemeMediatype))
	w("  <recommends>\n    <internet>first-run</internet>\n  </recommends>\n")
	w("  <branding>\n")
	w("    <color type=\"primary\" scheme_preference=\"light\">%s</color>\n", esc(l.BrandLight))
	w("    <color type=\"primary\" scheme_preference=\"dark\">%s</color>\n", esc(l.BrandDark))
	w("  </branding>\n")
	if len(l.OARS) == 0 {
		w("  <content_rating type=\"oars-1.1\"/>\n")
	} else {
		w("  <content_rating type=\"oars-1.1\">\n")
		keys := make([]string, 0, len(l.OARS))
		for k := range l.OARS {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			w("    <content_attribute id=\"%s\">%s</content_attribute>\n", esc(k), esc(l.OARS[k]))
		}
		w("  </content_rating>\n")
	}
	if screenshotsReady(l) {
		w("  <screenshots>\n")
		for i, s := range l.Screenshots.Item {
			if i == 0 {
				w("    <screenshot type=\"default\">\n")
			} else {
				w("    <screenshot>\n")
			}
			w("      <caption>%s</caption>\n", esc(s.Caption))
			w("      <image>https://raw.githubusercontent.com/cubancorona/bibletext/%s/docs/screenshots/linux/%s</image>\n", esc(l.Screenshots.Ref), esc(s.File))
			w("    </screenshot>\n")
		}
		w("  </screenshots>\n")
	}
	w("  <releases>\n")
	for _, r := range in.Releases {
		if r.Head == "" && len(r.Bullets) == 0 {
			w("    <release version=\"%s\" date=\"%s\"/>\n", esc(r.Version), esc(r.Date))
			continue
		}
		w("    <release version=\"%s\" date=\"%s\">\n      <description>\n", esc(r.Version), esc(r.Date))
		if r.Head != "" {
			w("        <p>%s</p>\n", esc(r.Head))
		}
		if len(r.Bullets) > 0 {
			w("        <ul>\n")
			for _, bl := range r.Bullets {
				w("          <li>%s</li>\n", esc(bl))
			}
			w("        </ul>\n")
		}
		w("      </description>\n    </release>\n")
	}
	w("  </releases>\n")
	w("</component>\n")
	return b.String()
}

func joinList(items []string) string { return strings.Join(items, ";") + ";" }

// renderDesktop is the freedesktop entry. StartupWMClass is load-bearing:
// the toolkit sets no class hint and GLFW derives WM_CLASS from the window
// title, which Run() sets to the product name and never changes.
func renderDesktop(in inputs, icon string) string {
	l, p := in.Listing, in.Product
	return strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + p.ProductName,
		"GenericName=" + l.GenericName,
		"Comment=" + l.Summary,
		"Exec=" + l.Executable + " %u",
		"Icon=" + icon,
		"Terminal=false",
		"Categories=" + joinList(l.Categories),
		"Keywords=" + joinList(l.Keywords),
		"MimeType=" + l.SchemeMediatype + ";",
		"StartupWMClass=" + p.ProductName,
		"StartupNotify=false",
	}, "\n") + "\n"
}

// snapArch carries the two things that genuinely differ between architectures
// in the snap, and the second one is not cosmetic.
//
// `platforms` is what snapcraft builds for. The GNU triplet is the directory
// ALSA looks in for its plugins, and the `layout` below binds a REAL path: an
// arm64 snap carrying the x86_64 triplet compiles, packs, installs and launches
// perfectly, and then has no narration audio at all — a defect visible only to a
// reader who presses play. Verified on an arm64 machine by checking that the
// bound directory actually resolves inside the snap's confined namespace.
//
// Snap layouts take no variables and snapcraft will not vary one per platform,
// so a single committed snapcraft.yaml cannot serve both. The committed file is
// the amd64 render; the arm64 build re-renders with -arch arm64 first.
type snapArch struct {
	Platform string // the snapcraft `platforms:` key
	Triplet  string // the multiarch tuple under /usr/lib
}

var snapArches = map[string]snapArch{
	"amd64": {Platform: "amd64", Triplet: "x86_64-linux-gnu"},
	"arm64": {Platform: "arm64", Triplet: "aarch64-linux-gnu"},
}

// defaultSnapArch is what the committed outputs are rendered for, so that
// TestCommittedFilesAreTheGeneratorsOutput keeps comparing like with like.
const defaultSnapArch = "amd64"

func renderSnapcraft(in inputs, a snapArch) string {
	l, p := in.Listing, in.Product
	var d strings.Builder
	d.WriteString("  " + l.Intro + "\n\n")
	for _, f := range l.Features {
		d.WriteString("  • " + f + "\n")
	}
	d.WriteString("\n  " + l.Outro + "\n")
	d.WriteString("  Privacy: " + p.SiteBase + "/privacy.html\n")
	return fmt.Sprintf(`# Rendered by cmd/linuxmeta from linux/listing.toml; edit the source, not
# this file. The binary is NOT built here: the release workflow builds and
# verifies the keyed executable once (the LXD build sees no repository
# secrets) and stages it under build/snap-stage with linux/asound.conf.
# The desktop entry is snap/gui/%s.desktop, picked up by name.
name: %s
title: %s
base: core24
version: "%s"
summary: %s
description: |
%slicense: %s
icon: snap/gui/icon.png
website: %s
contact: %s/support.html
issues: %s/issues
source-code: %s
confinement: strict
grade: stable
platforms:
  %s:

apps:
  %s:
    command: bin/%s
    extensions: [gnome]
    plugs:
      - network
      - network-bind
      - audio-playback
      - home

layout:
  /usr/lib/%s/alsa-lib:
    bind: $SNAP/usr/lib/%s/alsa-lib
  /usr/share/alsa:
    bind: $SNAP/usr/share/alsa
  /etc/asound.conf:
    bind-file: $SNAP/etc/asound.conf

parts:
  %s:
    plugin: dump
    source: build/snap-stage
    stage-packages:
      - libasound2-plugins
      - libasound2-data
`, l.Executable, l.Executable, p.ProductName, in.Version, l.Summary, d.String(), l.ProjectLicense,
		p.SiteBase, p.SiteBase, p.SourceRepo, p.SourceRepo,
		a.Platform,
		l.Executable, l.Executable,
		a.Triplet, a.Triplet,
		l.Executable)
}

// flatpakSource is where the manifest's first source points: the checked-out
// tree (the copy in this repository, built by CI) or a tagged commit of the
// public repository (the copy submitted to Flathub).
type flatpakSource struct {
	Tag, Commit string // both empty means the directory source
}

func renderFlatpakManifest(in inputs, src flatpakSource) string {
	l, p := in.Listing, in.Product
	id := l.AppstreamID
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }
	w("# Rendered by cmd/linuxmeta from linux/listing.toml; edit the source, not\n")
	w("# this file. This copy builds the checked-out tree (CI verification); the\n")
	w("# copy submitted to Flathub pins a release tag and commit instead\n")
	w("# (go run ./cmd/linuxmeta flatpak-manifest -tag v1.2.x -commit <sha>).\n")
	w("app-id: %s\n", id)
	w("runtime: org.freedesktop.Platform\nruntime-version: '25.08'\nsdk: org.freedesktop.Sdk\n")
	w("sdk-extensions:\n  - org.freedesktop.Sdk.Extension.golang\n")
	w("command: %s\n", l.Executable)
	w("finish-args:\n")
	w("  - --share=ipc\n  - --socket=x11\n  - --device=dri\n  - --socket=pulseaudio\n  - --share=network\n  - --filesystem=xdg-download\n")
	w("build-options:\n  append-path: /usr/lib/sdk/golang/bin\n  env:\n    GOROOT: /usr/lib/sdk/golang\n")
	w("    GOFLAGS: -mod=vendor -trimpath -buildvcs=false\n    GOPROXY: 'off'\n    GOTOOLCHAIN: local\n    CGO_ENABLED: '1'\n")
	w("modules:\n  - name: %s\n    buildsystem: simple\n    build-commands:\n", l.Executable)
	w("      # The patched toolkit, applied to the vendored stock module the way\n")
	w("      # scripts/setup-fyne-patch.sh applies it to third_party/fyne.\n")
	w("      - chmod -R u+w vendor/fyne.io/fyne/v2\n")
	for _, pt := range in.Patches {
		w("      - patch -p1 -d vendor/fyne.io/fyne/v2 < patches/%s\n", pt)
	}
	w("      - cp patches/NotoColorEmoji.ttf vendor/fyne.io/fyne/v2/theme/font/NotoColorEmoji.ttf\n")
	w("      - rm -f vendor/fyne.io/fyne/v2/theme/font/EmojiOneColor.otf\n")
	w("      - grep -q 'BibleText patch: was 100ms' vendor/fyne.io/fyne/v2/internal/driver/mobile/app/darwin_ios.go\n")
	w("      - grep -q 'BibleText patch: discrete caret blink' vendor/fyne.io/fyne/v2/widget/entry_cursor_anim.go\n")
	w("      - grep -q 'BibleText patch: atomic preferences write' vendor/fyne.io/fyne/v2/app/preferences_nonweb.go\n")
	w("      - grep -q 'BibleText patch: current emoji' vendor/fyne.io/fyne/v2/theme/bundled-emoji.go\n")
	w("      # Keyless by design: no linker value for the API.Bible key here\n")
	w("      # (docs/API_KEY_HANDLING.md); the reader adds a key under Settings.\n")
	w("      - go build -tags flatpak -ldflags '-s -w' -o %s ./cmd/bibletext\n", l.Executable)
	w("      - install -Dm755 %s ${FLATPAK_DEST}/bin/%s\n", l.Executable, l.Executable)
	w("      - install -Dm644 linux/%s.desktop ${FLATPAK_DEST}/share/applications/%s.desktop\n", id, id)
	w("      - install -Dm644 linux/flathub/%s.metainfo.xml ${FLATPAK_DEST}/share/metainfo/%s.metainfo.xml\n", id, id)
	for _, s := range []string{"256x256", "512x512"} {
		w("      - install -Dm644 linux/icons/hicolor/%s/apps/%s.png ${FLATPAK_DEST}/share/icons/hicolor/%s/apps/%s.png\n", s, id, s, id)
	}
	lic := "${FLATPAK_DEST}/share/licenses/" + id
	for _, f := range [][2]string{
		{"LICENSE", "LICENSE"},
		{"NOTICE", "NOTICE"},
		{"assets/fonts/atkinson/OFL.txt", "OFL-Atkinson-Hyperlegible.txt"},
		{"assets/fonts/reading/Junicode-OFL.txt", "OFL-Junicode.txt"},
		{"assets/fonts/reading/EzraSIL-Licenses.txt", "EzraSIL-Licenses.txt"},
		{"assets/fonts/share/OFL-LICENSES.txt", "OFL-share-card-fonts.txt"},
		{"patches/NotoColorEmoji-LICENSE-OFL.txt", "OFL-NotoColorEmoji.txt"},
	} {
		w("      - install -Dm644 %s %s/%s\n", f[0], lic, f[1])
	}
	w("    sources:\n")
	if src.Tag == "" {
		w("      - type: dir\n        path: ..\n")
	} else {
		w("      - type: git\n        url: %s.git\n        tag: %s\n        commit: %s\n", p.SourceRepo, src.Tag, src.Commit)
		w("        x-checker-data:\n          type: git\n          tag-pattern: '^v([\\d.]+)$'\n")
	}
	w("      - go.mod.yml\n") // flatpak-go-mod's output; its first entry places modules.txt under vendor/
	return b.String()
}

// renderIcon scales the shipped mark (cmd/bibletext/Icon.png) to one size.
func renderIcon(src image.Image, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
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
	if b := img.Bounds(); b.Dx() != b.Dy() {
		return nil, fmt.Errorf("%s: the icon must be square, got %dx%d", path, b.Dx(), b.Dy())
	}
	return img, nil
}

// output is one rendered file; textual outputs carry Text, images Image.
type output struct {
	Path  string // repository-relative
	Text  string
	Image *image.NRGBA
}

func renderAll(repo string, in inputs, a snapArch) ([]output, error) {
	icon, err := loadIcon(filepath.Join(repo, "cmd", "bibletext", "Icon.png"))
	if err != nil {
		return nil, err
	}
	id := in.Listing.AppstreamID
	exe := in.Listing.Executable
	return []output{
		{Path: filepath.Join("linux", id+".metainfo.xml"), Text: renderMetainfo(in, false)},
		{Path: filepath.Join("linux", "flathub", id+".metainfo.xml"), Text: renderMetainfo(in, true)},
		{Path: filepath.Join("linux", id+".desktop"), Text: renderDesktop(in, id)},
		{Path: filepath.Join("snap", "gui", exe+".desktop"), Text: renderDesktop(in, "${SNAP}/meta/gui/icon.png")},
		{Path: filepath.Join("linux", "icons", "hicolor", "256x256", "apps", id+".png"), Image: renderIcon(icon, 256)},
		{Path: filepath.Join("linux", "icons", "hicolor", "512x512", "apps", id+".png"), Image: renderIcon(icon, 512)},
		{Path: filepath.Join("snap", "gui", "icon.png"), Image: renderIcon(icon, 256)},
		{Path: filepath.Join("snap", "snapcraft.yaml"), Text: renderSnapcraft(in, a)},
		{Path: filepath.Join("flatpak", id+".yml"), Text: renderFlatpakManifest(in, flatpakSource{})},
	}, nil
}

func writeOutputs(repo string, outs []output) error {
	for _, o := range outs {
		path := filepath.Join(repo, o.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if o.Image != nil {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			enc := png.Encoder{CompressionLevel: png.BestCompression}
			if err := enc.Encode(f, o.Image); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			continue
		}
		if err := os.WriteFile(path, []byte(o.Text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: linuxmeta render|flatpak-manifest [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "render":
		fs := flag.NewFlagSet("render", flag.ContinueOnError)
		repo := fs.String("repo", ".", "repository root")
		arch := fs.String("arch", defaultSnapArch, "architecture the snap is rendered for (amd64 or arm64)")
		if err = fs.Parse(os.Args[2:]); err == nil {
			a, ok := snapArches[*arch]
			if !ok {
				err = fmt.Errorf("unknown -arch %q; known: amd64, arm64", *arch)
				break
			}
			var in inputs
			if in, err = readInputs(*repo); err == nil {
				var outs []output
				if outs, err = renderAll(*repo, in, a); err == nil {
					err = writeOutputs(*repo, outs)
				}
			}
		}
	case "flatpak-manifest":
		fs := flag.NewFlagSet("flatpak-manifest", flag.ContinueOnError)
		repo := fs.String("repo", ".", "repository root")
		tag := fs.String("tag", "", "release tag the Flathub copy pins (with -commit)")
		commit := fs.String("commit", "", "the tag's commit")
		if err = fs.Parse(os.Args[2:]); err == nil {
			if (*tag == "") != (*commit == "") {
				err = errors.New("-tag and -commit go together")
			} else if *commit != "" && !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(*commit) {
				err = errors.New("-commit must be the full 40-hex commit")
			} else {
				var in inputs
				if in, err = readInputs(*repo); err == nil {
					fmt.Print(renderFlatpakManifest(in, flatpakSource{Tag: *tag, Commit: *commit}))
				}
			}
		}
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "linuxmeta:", err)
		os.Exit(1)
	}
}
