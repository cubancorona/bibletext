package bibletext

// Guards the embedded share-typeface library: every face must parse (a corrupt
// asset would otherwise silently shrink the Regenerate cycle) and every
// scheme×face pairing reachable in one scheme cycle must render. Set
// BIBLETEXT_SHARE_SAMPLES=<dir> to also dump the rendered cards plus a single
// contact sheet for visual review — that's how the palette/typeface choices
// were tuned.

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	xdraw "golang.org/x/image/draw"
)

func TestShareTypefacesAllParse(t *testing.T) {
	faces := loadShareTypefaces()
	const want = 7
	if len(faces) != want {
		names := make([]string, 0, len(faces))
		for _, f := range faces {
			names = append(names, f.name)
		}
		t.Fatalf("expected %d embedded share typefaces, got %d (%v) — an asset failed to parse", want, len(faces), names)
	}
	for _, f := range faces {
		if f.regular == nil || f.bold == nil {
			t.Errorf("typeface %q has a nil face", f.name)
		}
	}
}

// TestRenderAllSchemesAndTypefaces walks one full scheme cycle (13 variants).
// Because the scheme and typeface counts are coprime, 13 consecutive variants
// also visit every typeface at least twice — so this exercises every scheme and
// every face through the real renderVerseImage path.
func TestRenderAllSchemesAndTypefaces(t *testing.T) {
	if len(shareSchemes)%len(loadShareTypefaces()) == 0 {
		t.Fatalf("scheme count (%d) must stay coprime with typeface count (%d) so Regenerate reaches every pairing",
			len(shareSchemes), len(loadShareTypefaces()))
	}
	dump := os.Getenv("BIBLETEXT_SHARE_SAMPLES")

	verse := "“Come to me, all you who labor and are heavily burdened, and I will give you rest . . .”"
	var paths []string
	for v := 0; v < len(shareSchemes); v++ {
		p, err := renderVerseImage(nil, verse, "Matthew 11:28", "World English Bible", v)
		if err != nil {
			t.Fatalf("variant %d failed to render: %v", v, err)
		}
		fi, err := os.Stat(p)
		if err != nil || fi.Size() == 0 {
			t.Fatalf("variant %d produced no image: %v", v, err)
		}
		if dump != "" {
			dst := filepath.Join(dump, fmt.Sprintf("share-sample-%02d.png", v))
			b, _ := os.ReadFile(p)
			_ = os.WriteFile(dst, b, 0o644)
			paths = append(paths, dst)
		}
	}

	if dump != "" {
		if err := writeContactSheet(paths, filepath.Join(dump, "share-contact-sheet.png")); err != nil {
			t.Logf("contact sheet: %v", err)
		}
	}
}

// writeContactSheet tiles the sample cards (scaled down) into one reviewable grid.
func writeContactSheet(paths []string, out string) error {
	const tile, cols = 320, 5
	rows := (len(paths) + cols - 1) / cols
	sheet := image.NewRGBA(image.Rect(0, 0, cols*tile, rows*tile))
	for i, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return err
		}
		x, y := (i%cols)*tile, (i/cols)*tile
		xdraw.ApproxBiLinear.Scale(sheet, image.Rect(x, y, x+tile, y+tile), img, img.Bounds(), xdraw.Src, nil)
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}
