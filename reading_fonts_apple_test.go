//go:build darwin

package bibletext

import "testing"

// The shipped faces must actually reach CoreText. Everything the Apple panes do
// with them depends on this and NOTHING reports its failure: the post-import
// sweep would simply find no font by that name, leave the imported one alone,
// and the pane would read in the system serif with nobody the wiser.
func TestAppleReadingFontsRegister(t *testing.T) {
	if !registerAppleReadingFonts() {
		t.Fatal("the shipped reading faces did not reach CoreText, so the panes would " +
			"silently keep the system serif")
	}
	for _, family := range []string{readingFaceFamily, hebrewFaceFamily} {
		if !fontFamilyAvailable(family) {
			t.Errorf("%q does not resolve after registration", family)
		}
	}
	// The control: the check must be able to say no. CTFontCreateWithName
	// never fails — an unknown name yields a default face — so a check that
	// did not compare what came back would pass for anything at all.
	if fontFamilyAvailable("NoSuchFaceShipsWithThisApp") {
		t.Error("an unregistered family reported as available — the check cannot fail")
	}
}

// The reading face the sweep and the fallback page set is checked by its
// name. CoreText answers a face it does not have with another font, upright
// and regular, which would replace every run of a page, the italic title and
// the bold headings the panes find by their traits among them; so a face
// CoreText does not have is no face, and the run keeps its own. The
// registered faces come back as asked, and the fallback page's faces, at
// each pair of traits, are the reading face's.
func TestAReadingFaceCoreTextDoesNotHaveIsNoFace(t *testing.T) {
	if !registerAppleReadingFonts() {
		t.Fatal("the shipped reading faces did not reach CoreText")
	}
	for _, name := range []string{"Junicode-Regular", "Junicode-Italic", "Junicode-Bold", "Junicode-BoldItalic"} {
		if got := readingFaceName(name, 18); got != name {
			t.Errorf("the registered face %s came back as %q", name, got)
		}
	}
	if got := readingFaceName("NoSuchFaceShipsWithThisApp-Italic", 18); got != "" {
		t.Errorf("a face CoreText does not have came back as %q; want none, so the run keeps its own face", got)
	}
	for _, tc := range []struct {
		bold, italic bool
		want         string
	}{
		{false, false, "Junicode-Regular"},
		{false, true, "Junicode-Italic"},
		{true, false, "Junicode-Bold"},
		{true, true, "Junicode-BoldItalic"},
	} {
		if got := readingFontOfSizeName(18, tc.bold, tc.italic); got != tc.want {
			t.Errorf("the fallback page's face at bold=%v italic=%v is %q, want %s", tc.bold, tc.italic, got, tc.want)
		}
	}
}
