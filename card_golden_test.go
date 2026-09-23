//go:build cardgolden

package bibletext

// CARD GOLDEN — hashes of rendered share cards, for proving a change to the
// card renderer leaves ordinary cards byte-identical.
//
//	CARD_GOLDEN_OUT=/path/outside/repo/hashes.txt go test -tags cardgolden -run TestCardGolden .
//
// Run it before a change and after, and diff the two files.

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

// Ordinary cards: no small capitals anywhere. Prose, a poem with authored
// breaks, a long quote that has to clamp, punctuation-heavy text, macrons.
var cardGoldenTexts = []struct{ text, cite, version string }{
	{"“For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish, but have eternal life.”", "John 3:16", "World English Bible"},
	{"“Yahweh is my shepherd;\nI shall lack nothing.\nHe makes me lie down in green pastures.\nHe leads me beside still waters.”", "Psalms 23:1–2", "World English Bible"},
	{"“In the beginning was the Word, and the Word was with God, and the Word was God. The same was in the beginning with God. All things were made through him. Without him, nothing was made that has been made. In him was life, and the life was the light of men. The light shines in the darkness, and the darkness hasn’t overcome it. There came a man sent from God, whose name was John. The same came as a witness, that he might testify about the light, that all might believe through him. He was not the light, but was sent that he might testify about the light. The true light that enlightens everyone was coming into the world.”", "John 1:1–9", "World English Bible"},
	{"“Jesus wept.”", "John 11:35", "Berean Standard Bible"},
	{"“Ēnoch begot Irad — and Irad begot Mehujael!”", "Genesis 4:18", "Test"},
	{"“The Lord is my shepherd; I shall not want.”", "Psalms 23:1", "New King James Version"},
}

func TestCardGolden(t *testing.T) {
	out := os.Getenv("CARD_GOLDEN_OUT")
	if out == "" {
		t.Skip("CARD_GOLDEN_OUT is not set")
	}
	var b strings.Builder
	for _, c := range cardGoldenTexts {
		for variant := 0; variant < 10; variant++ {
			path, err := renderVerseImage(nil, c.text, c.cite, c.version, variant)
			if err != nil {
				t.Fatalf("%s v%d: %v", c.cite, variant, err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tf, _ := typefaceForText(c.cite+"|"+c.version, variant, c.text)
			fmt.Fprintf(&b, "%-14s v%-2d %-18s %x\n", c.cite, variant, tf.name, sha256.Sum256(raw))
		}
	}
	// The lock-screen artwork shares the wrap and draw helpers.
	reg, bold := serifFontBytes(nil, fyne.TextStyle{}), serifFontBytes(nil, fyne.TextStyle{Bold: true})
	for _, title := range []string{"John 3", "Psalms 119", "Song of Solomon 2", "1 Thessalonians 5"} {
		path, err := renderChapterArtwork(title, "World English Bible", reg, bold)
		if err != nil {
			t.Fatalf("artwork %s: %v", title, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "artwork %-20s %x\n", title, sha256.Sum256(raw))
	}
	if err := os.WriteFile(out, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d cards hashed to %s", len(cardGoldenTexts)*10, out)
}

// TestCardDivineNameSheet renders one divine-name card per typeface into
// CARD_SHEET_DIR, for looking at.
func TestCardDivineNameSheet(t *testing.T) {
	dir := os.Getenv("CARD_SHEET_DIR")
	if dir == "" {
		t.Skip("CARD_SHEET_DIR is not set")
	}
	text := "“The Lᴏʀᴅ is my shepherd;\nI shall not want.\nHe makes me to lie down in green pastures;\nHe leads me beside the still waters.”"
	for variant := 0; variant < len(loadShareTypefaces()); variant++ {
		path, err := renderVerseImage(nil, text, "Psalms 23:1–2", "New King James Version", variant)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		tf, _ := typefaceForText("Psalms 23:1–2|New King James Version", variant, text)
		if err := os.WriteFile(fmt.Sprintf("%s/card-%d-%s.png", dir, variant, strings.ReplaceAll(tf.name, " ", "")), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
