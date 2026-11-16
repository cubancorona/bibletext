package bibletext

import (
	"strings"
	"testing"
)

// Native hit testing cannot run in the host Go suite. Guard the distinction
// between the pill's existing artwork, its reserved blank band and the extra
// points UIKit may deliver to that artwork's control.
func TestIOSNotePillHitAreaFitsItsExistingBand(t *testing.T) {
	src := readNativeSource(t, "reading_ios.go")
	pill := constValue(t, src, "reading_ios.go", "kNotePill")
	pad := constValue(t, src, "reading_ios.go", "kNotePillHitPad")
	above := constValue(t, src, "reading_ios.go", "kNoteGapAbove")
	below := constValue(t, src, "reading_ios.go", "kNoteGapBelow")
	if pill != 28 || pill+2*pad != 44 {
		t.Errorf("artwork %v with hit padding %v must retain 28pt artwork and a 44pt target", pill, pad)
	}
	if pad >= above || pad >= below {
		t.Errorf("hit padding %v must remain inside the existing %v/%v blank gaps", pad, above, below)
	}
	if band := pill + above + below; band != 48 || pill+2*pad >= band {
		t.Errorf("44pt target must stay inside the unchanged 48pt reservation; band is %v", band)
	}
	for _, fragment := range []string{
		"chip.frame = CGRectMake(x, y, cw, kNotePill);",
		"gNoteView.frame = CGRectMake(x, y, cw, kNotePill);",
		"chip.frame = gNoteView.bounds;",
		"return kNoteGapAbove + kNotePill + kNoteGapBelow;",
		"chip.layer.cornerRadius = kNotePill / 2;",
		"box.layer.cornerRadius = kNotePill / 2;",
		"box.clipsToBounds = YES;",
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("pill artwork or band geometry changed: missing %q", fragment)
		}
	}
}

func TestIOSNotePillHitSlopRequiresItsOwnResolvedReservation(t *testing.T) {
	src := readNativeSource(t, "reading_ios.go")
	gate := nativeFunctionSource(t, "reading_ios.go", "static CGFloat btIOSPillHitPadForBand(int key, NSInteger verse, CGRect artwork)")
	for _, fragment := range []string{
		"verse <= 0 || gReadingTV == nil",
		"loc == NSNotFound || loc >= ts.length",
		"band->key != key",
		"band->contribution < kNotePill + 2 * kNotePillHitPad",
		"para.location != 0 || gNoteTopInset < band->contribution",
		"band->para.location != para.location",
		"top += btIOSBandStackAbove(b) - kNoteGapAbove",
		"CGRect target = CGRectInset(artwork, 0, -kNotePillHitPad)",
		"if (CGRectContainsRect(slot, target)) return kNotePillHitPad",
	} {
		if !strings.Contains(gate, fragment) {
			t.Errorf("outside-bounds tap area must reject missing or unrelated reservations: missing %q", fragment)
		}
	}
	for _, fragment := range []string{
		"chip.pillHitPad = btIOSPillHitPadForBand(spec->key, spec->verse, chip.frame);",
		"chip.pillHitPad = btIOSPillHitPadForBand(kNoteStickerBandKey, gNoteAnchorVerse, gNoteView.frame);",
	} {
		if !strings.Contains(src, fragment) {
			t.Errorf("pill must resolve its own band before accepting outside taps: missing %q", fragment)
		}
	}
}

func TestIOSNotePillHitTestingPreservesOtherControls(t *testing.T) {
	src := readNativeSource(t, "reading_ios.go")
	implementation := func(name string) string {
		t.Helper()
		start := strings.Index(src, "@implementation "+name+"\n")
		if start < 0 {
			t.Fatalf("missing %s implementation", name)
		}
		rest := src[start:]
		end := strings.Index(rest, "\n@end")
		if end < 0 {
			t.Fatalf("missing %s implementation boundary", name)
		}
		return rest[:end]
	}
	button := implementation("BTNotePillButton")
	if !strings.Contains(button, "CGRectInset(self.bounds, 0, -self.pillHitPad)") {
		t.Error("pill tap area must extend vertically while keeping its original width")
	}
	if !strings.Contains(button, "self.hidden || !self.userInteractionEnabled || !self.enabled || self.alpha <= 0.01") {
		t.Error("hidden, disabled or noninteractive pills must reject the added tap area")
	}
	host := implementation("BTNotePillHostView")
	if !strings.Contains(host, "[super pointInside:point withEvent:event]") ||
		!strings.Contains(host, "[chip pointInside:[self convertPoint:point toView:chip] withEvent:event]") {
		t.Error("singleton parent must admit only its existing bounds or its button's guarded tap area")
	}
	for _, impl := range []string{button, host} {
		for _, fragment := range []string{"setFrame:", "layoutSubviews", "drawRect:", "backgroundColor", "titleLabel"} {
			if strings.Contains(impl, fragment) {
				t.Errorf("hit testing must not change pill rendering or layout: found %q", fragment)
			}
		}
	}
	if n := strings.Count(src, "[BTNotePillButton buttonWithType:UIButtonTypeSystem]"); n != 2 {
		t.Errorf("only the singleton and per-paragraph pills should use enlarged hit testing; found %d constructors", n)
	}
	if !strings.Contains(src, "UIView *box = btIOSNotePill()\n        ? [[BTNotePillHostView alloc] initWithFrame:CGRectZero]\n        : [[UIView alloc] initWithFrame:CGRectZero];") {
		t.Error("expanded note cards must retain their ordinary UIView parent")
	}
}
