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
