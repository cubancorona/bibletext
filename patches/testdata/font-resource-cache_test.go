package painter_test

import (
	"image"
	"image/color"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/internal/painter"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

type btFontResourceTheme struct {
	fyne.Theme
	font    fyne.Resource
	padding float32
}

type btNonComparableFontResource struct{ data []byte }

func (r btNonComparableFontResource) Name() string    { return "synthetic-font.ttf" }
func (r btNonComparableFontResource) Content() []byte { return r.data }

type btInterfaceFontResource struct{ data any }

func (r btInterfaceFontResource) Name() string    { return "synthetic-font.ttf" }
func (r btInterfaceFontResource) Content() []byte { return r.data.([]byte) }

func (t btFontResourceTheme) Font(fyne.TextStyle) fyne.Resource { return t.font }

func (t btFontResourceTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return t.padding
	}
	return t.Theme.Size(name)
}

func btFontResourceScope(t *testing.T, font fyne.Resource, padding float32) (*canvas.Text, *container.ThemeOverride) {
	t.Helper()
	text := canvas.NewText("A synthetic citation.", color.Black)
	override := container.NewThemeOverride(text, btFontResourceTheme{
		Theme: fyne.CurrentApp().Settings().Theme(), font: font, padding: padding,
	})
	return text, override
}

func TestBTFontResourcesShareTablesAcrossScopes(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	resource := theme.DefaultTextFont()
	first, _ := btFontResourceScope(t, resource, 0)
	initial := painter.CachedFontFace(fyne.TextStyle{}, nil, first)
	for i := 0; i < 32; i++ {
		text, override := btFontResourceScope(t, resource, float32(i))
		face := painter.CachedFontFace(fyne.TextStyle{}, nil, text)
		for _, r := range "A🙂" {
			before, after := initial.Fonts.ResolveFace(r), face.Fonts.ResolveFace(r)
			if before.Font != after.Font {
				t.Fatal("identical resources retained another copy of immutable font tables")
			}
			if before == after {
				t.Fatal("separate scopes shared mutable glyph caches")
			}
		}
		override.Refresh()
		refreshed := painter.CachedFontFace(fyne.TextStyle{}, nil, text)
		if refreshed.Fonts.ResolveFace('A').Font != initial.Fonts.ResolveFace('A').Font {
			t.Fatal("refresh retained another copy of immutable font tables")
		}
	}
}

func TestBTFontResourcesPreserveFontIsolation(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	first, _ := btFontResourceScope(t, theme.DefaultTextFont(), 0)
	second, _ := btFontResourceScope(t, theme.DefaultTextMonospaceFont(), 0)
	regular := painter.CachedFontFace(fyne.TextStyle{}, nil, first).Fonts.ResolveFace('A')
	other := painter.CachedFontFace(fyne.TextStyle{}, nil, second).Fonts.ResolveFace('A')
	if regular.Font == other.Font {
		t.Fatal("distinct primary resources shared font tables")
	}
	custom := painter.CachedFontFace(fyne.TextStyle{}, theme.DefaultTextFont(), first).Fonts.ResolveFace('A')
	if custom.Font != regular.Font || custom == regular {
		t.Fatal("an explicit font did not reuse immutable tables with independent glyph caches")
	}
	bold := painter.CachedFontFace(fyne.TextStyle{Bold: true}, nil, first)
	if bold.Fonts.ResolveFace('A') == regular {
		t.Fatal("distinct styles shared mutable glyph caches")
	}
}

func TestBTFontResourcesSupportNonComparableThemeValues(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for name, resource := range map[string]fyne.Resource{
		"slice field":     btNonComparableFontResource{data: theme.DefaultTextFont().Content()},
		"interface field": btInterfaceFontResource{data: theme.DefaultTextFont().Content()},
	} {
		t.Run(name, func(t *testing.T) {
			first, _ := btFontResourceScope(t, resource, 0)
			second, _ := btFontResourceScope(t, resource, 0)
			one := painter.CachedFontFace(fyne.TextStyle{}, nil, first)
			two := painter.CachedFontFace(fyne.TextStyle{}, nil, second)
			want, _ := painter.MeasureString(one.Fonts, "A synthetic citation.", 13, fyne.TextStyle{})
			got, _ := painter.MeasureString(two.Fonts, "A synthetic citation.", 13, fyne.TextStyle{})
			if got != want || got.Width <= 0 {
				t.Fatal("a non-comparable theme font lost its ordinary measurement behavior")
			}
		})
	}
}

func TestBTFontResourceScopesEvictRetiredFaces(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	resource := theme.DefaultTextFont()
	first, _ := btFontResourceScope(t, resource, 0)
	before := painter.CachedFontFace(fyne.TextStyle{}, nil, first)
	defaultFace := painter.CachedFontFace(fyne.TextStyle{}, nil, nil)
	customFace := painter.CachedFontFace(fyne.TextStyle{}, resource, nil)
	var last *canvas.Text
	var lastFace *painter.FontCacheItem
	for i := 0; i < 128; i++ {
		last, _ = btFontResourceScope(t, resource, float32(i))
		lastFace = painter.CachedFontFace(fyne.TextStyle{}, nil, last)
	}
	if got := painter.CachedFontFace(fyne.TextStyle{}, nil, last); got != lastFace {
		t.Fatal("the most recently used scope was evicted")
	}
	after := painter.CachedFontFace(fyne.TextStyle{}, nil, first)
	if before == after {
		t.Fatal("a retired scope survived beyond the scoped cache limit")
	}
	if before.Fonts.ResolveFace('A').Font != after.Fonts.ResolveFace('A').Font {
		t.Fatal("eviction discarded reusable immutable font tables")
	}
	if painter.CachedFontFace(fyne.TextStyle{}, nil, nil) != defaultFace {
		t.Fatal("scope eviction changed an unscoped font lifetime")
	}
	if painter.CachedFontFace(fyne.TextStyle{}, resource, nil) != customFace {
		t.Fatal("scope eviction changed an explicit font lifetime")
	}
}

func TestBTFontResourceScopesKeepActiveFaces(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	resource := theme.DefaultTextFont()
	active, _ := btFontResourceScope(t, resource, 0)
	face := painter.CachedFontFace(fyne.TextStyle{}, nil, active)
	for i := 0; i < 512; i++ {
		text, _ := btFontResourceScope(t, resource, float32(i))
		painter.CachedFontFace(fyne.TextStyle{}, nil, text)
		if i%16 == 0 && painter.CachedFontFace(fyne.TextStyle{}, nil, active) != face {
			t.Fatal("a recently used face was evicted")
		}
	}
}

func TestBTFontResourcesClearReloadsTables(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	text, _ := btFontResourceScope(t, theme.DefaultTextFont(), 0)
	before := painter.CachedFontFace(fyne.TextStyle{}, nil, text).Fonts.ResolveFace('A')
	painter.ClearFontCache()
	after := painter.CachedFontFace(fyne.TextStyle{}, nil, text).Fonts.ResolveFace('A')
	if before.Font == after.Font {
		t.Fatal("a settings reset kept the old font tables")
	}
}

func TestBTFontResourceScopesAllowConcurrentClear(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var objects []*canvas.Text
	for i := 0; i < 4; i++ {
		text, _ := btFontResourceScope(t, theme.DefaultTextFont(), float32(i))
		objects = append(objects, text)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			painter.ClearFontCache()
		}
	}()
	for _, object := range objects {
		wg.Add(1)
		go func(object *canvas.Text) {
			defer wg.Done()
			for i := 0; i < 16; i++ {
				if painter.CachedFontFace(fyne.TextStyle{}, nil, object) == nil {
					t.Error("a concurrent cache reset lost the scoped face")
					return
				}
			}
		}(object)
	}
	wg.Wait()
	before := painter.CachedFontFace(fyne.TextStyle{}, nil, objects[0]).Fonts.ResolveFace('A')
	painter.ClearFontCache()
	after := painter.CachedFontFace(fyne.TextStyle{}, nil, objects[0]).Fonts.ResolveFace('A')
	if before.Font == after.Font {
		t.Fatal("a final scoped cache reset retained old font tables")
	}
}

func TestBTFontResourceScopesMeasureInParallel(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	const content = "Synthetic Latin citation."
	var faces []*painter.FontCacheItem
	for i := 0; i < 8; i++ {
		text, _ := btFontResourceScope(t, theme.DefaultTextFont(), float32(i))
		faces = append(faces, painter.CachedFontFace(fyne.TextStyle{}, nil, text))
	}
	want, _ := painter.MeasureString(faces[0].Fonts, content, 13, fyne.TextStyle{})
	var wg sync.WaitGroup
	for _, face := range faces {
		wg.Add(1)
		go func(face *painter.FontCacheItem) {
			defer wg.Done()
			pixels := image.NewRGBA(image.Rect(0, 0, 300, 40))
			for n := 0; n < 16; n++ {
				got, _ := painter.MeasureString(face.Fonts, content, 13, fyne.TextStyle{})
				if got != want {
					t.Errorf("parallel measurement changed size: %v to %v", want, got)
					return
				}
				painter.DrawString(pixels, content, color.Black, face.Fonts, 13, 1, fyne.TextStyle{})
			}
		}(face)
	}
	wg.Wait()
}
