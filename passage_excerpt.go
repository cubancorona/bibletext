package bibletext

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const passageExcerptInset float32 = 16

// passageCitationTheme lets the block own its spacing. RichText's usual field
// padding would add another sixteen points to a short composer's height.
type passageCitationTheme struct{ fyne.Theme }

func (t passageCitationTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 0
	}
	return t.Theme.Size(name)
}

// passagePreviewCitation keeps the translation beside its passage. An unknown
// edition may use its full name; the wrapping citation never widens the card.
func passagePreviewCitation(ref, version string) string {
	ref, version = strings.TrimSpace(ref), strings.TrimSpace(version)
	if version == "" {
		return ref
	}
	if ref == "" {
		return "(" + version + ")"
	}
	return ref + " (" + version + ")"
}

// newPassageExcerptPreview shares the composers' and answer panel's selected
// passage treatment: an upright inset quote, with its source below it. The
// excerpt keeps the reading face, small capitals, row budget and ellipsis.
func newPassageExcerptPreview(ref, version, text string, rows int, pal palette) (*fyne.Container, *noteExcerpt) {
	excerpt := newNoteExcerpt(text, rows, pal.TextMuted)
	if strings.TrimSpace(text) == "" {
		excerpt.Hide()
	}
	citation := widget.NewRichText(&widget.TextSegment{
		Text: passagePreviewCitation(ref, version),
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignTrailing,
			ColorName: theme.ColorNamePrimary,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	citation.Wrapping = fyne.TextWrapWord
	base := fyne.CurrentApp().Settings().Theme()
	block := container.NewVBox(excerpt, container.NewThemeOverride(citation, passageCitationTheme{base}))
	return container.New(layout.NewCustomPaddedLayout(0, 0, passageExcerptInset, passageExcerptInset), block), excerpt
}

func (p aiPassage) previewText() string {
	if p.HasPreview {
		return p.PreviewText
	}
	return p.Text
}

func (p aiPassage) previewVersion() string {
	if p.VersionAbbrev != "" {
		return p.VersionAbbrev
	}
	return p.Version
}
