package bibletext

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// THE DIVINE NAME ON THE IMAGE CARD, IN EVERY CARD TYPEFACE.
//
// Only Cardo of the seven card faces carries the Unicode small capitals, so the
// others draw the name from their own capitals at smallCapScale (cardText). The
// four things that must hold are below; together they are why that is safe.

const divineNameLine = "“The Lᴏʀᴅ is my shepherd; Gᴏᴅ is my strength.”"

// 1. Measured is drawn. The card wraps and centres a line to the width measure
// reports; if drawing advanced further, text would run past the card's edge.
func TestASynthesizedSmallCapitalMeasuresWhatItDraws(t *testing.T) {
	for _, tf := range loadShareTypefaces() {
		ct := newCardText(tf.regular, 48)
		img := image.NewRGBA(image.Rect(0, 0, 2000, 120))
		d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: ct.face, Dot: fixed.P(10, 80)}
		start := d.Dot.X
		ct.draw(d, divineNameLine)
		if drawn, measured := d.Dot.X-start, ct.measure(divineNameLine); drawn != measured {
			t.Errorf("%s: drew %v but measured %v — a wrapped line would not fit what was drawn", tf.name, drawn, measured)
		}
	}
}

// 2. An ordinary line takes the unchanged path: the same width and the same
// pixels as the plain font calls every card used before cardText existed.
func TestAnOrdinaryCardLineIsDrawnExactlyAsBefore(t *testing.T) {
	const line = "“For God so loved the world, that he gave his one and only Son.”"
	for _, tf := range loadShareTypefaces() {
		ct := newCardText(tf.regular, 48)
		if got, want := ct.measure(line), font.MeasureString(ct.face, line); got != want {
			t.Errorf("%s: measure %v, plain measure %v", tf.name, got, want)
		}
		a := image.NewRGBA(image.Rect(0, 0, 2000, 120))
		b := image.NewRGBA(image.Rect(0, 0, 2000, 120))
		ct.draw(&font.Drawer{Dst: a, Src: image.NewUniform(color.Black), Face: ct.face, Dot: fixed.P(10, 80)}, line)
		(&font.Drawer{Dst: b, Src: image.NewUniform(color.Black), Face: ct.face, Dot: fixed.P(10, 80)}).DrawString(line)
		for i := range a.Pix {
			if a.Pix[i] != b.Pix[i] {
				t.Errorf("%s: an ordinary line drew different pixels through cardText", tf.name)
				break
			}
		}
	}
}

// 3. No missing glyph: every rune actually drawn — after a small capital the
// face lacks has become its capital — exists in the face. A missing glyph is a
// blank on the card, with nothing to say so.
func TestNoCardTypefaceDrawsAMissingGlyphForTheDivineName(t *testing.T) {
	var buf sfnt.Buffer
	for _, tf := range loadShareTypefaces() {
		ct := newCardText(tf.regular, 48)
		for _, p := range ct.pieces(divineNameLine) {
			for _, r := range p.text {
				if idx, err := tf.regular.GlyphIndex(&buf, r); err != nil || idx == 0 {
					t.Errorf("%s draws %q with no glyph for it", tf.name, r)
				}
			}
		}
		// And the face that HAS the small capitals uses them rather than
		// synthesizing: a designed small capital beats a scaled one.
		if idx, _ := tf.regular.GlyphIndex(&buf, 'ᴏ'); idx != 0 && ct.synthesizes(divineNameLine) {
			t.Errorf("%s carries the small capitals but synthesized them anyway", tf.name)
		}
	}
}

// 4. The rotation is back: a divine-name card walks all seven typefaces under
// Regenerate, as every other card does, instead of always landing on Cardo.
func TestADivineNameCardRotatesThroughEveryTypeface(t *testing.T) {
	seen := map[string]bool{}
	faces := loadShareTypefaces()
	for variant := 0; variant < len(faces); variant++ {
		tf, ok := typefaceForText("Psalms 23:1|NKJV", variant, divineNameLine)
		if !ok {
			t.Fatal("no typeface for a divine-name card")
		}
		seen[tf.name] = true
	}
	if len(seen) != len(faces) {
		t.Errorf("a divine-name card reached %d of %d typefaces: %v", len(seen), len(faces), seen)
	}
}
