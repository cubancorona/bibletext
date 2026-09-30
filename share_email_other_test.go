//go:build !linux && !windows && !ios && !android

package bibletext

import (
	"net/url"
	"testing"
)

// WHERE A MAILTO: LINK IS THE ONLY ROUTE THE PICTURE GETS NO EMAIL…: the
// macOS mimic of the desktop path offers Email… for text and withholds it
// for the image, and a compose with a file fails without opening anything.
// Mutation: the image offered (Email… then shows on the image sheet and
// does nothing when pressed).
func TestAMailtoLinkCarriesNoPicture(t *testing.T) {
	if !shareEmailAvailable(false) {
		t.Error("text: Email… must be offered")
	}
	if shareEmailAvailable(true) {
		t.Error("image: Email… must be withheld where only a mailto: link can be opened")
	}
	prev := externalOpener
	opened := 0
	externalOpener = func(*url.URL) error { opened++; return nil }
	t.Cleanup(func() { externalOpener = prev })
	if err := composeShareEmail("John 1:1 (Sample)", "text", "/nowhere/card.png"); err == nil {
		t.Error("a compose with a file must fail")
	}
	if opened != 0 {
		t.Errorf("a compose with a file opened %d links", opened)
	}
	if err := composeShareEmail("John 1:1 (Sample)", "text", ""); err != nil || opened != 1 {
		t.Errorf("control: a text compose = %v after %d opens, want one link opened", err, opened)
	}
}
