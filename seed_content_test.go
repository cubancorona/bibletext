package bibletext

// THE SEED IS A SNAPSHOT OF THE DECODER, and snapshots rot. It is the first
// thing a reader ever sees when a first launch has neither a cache nor a
// network, so it should look like the app rather than an older version of it.
// These hold it to what the decoder produces today; when one fails, regenerate
// with scripts/gen-seed-gospels.py rather than editing the expectation.

import "testing"

func TestSeedCarriesWhatTheDecoderProduces(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatal(err)
	}
	verses, breaks, notes, starts := 0, 0, 0, 0
	for _, chapters := range bd.Verses {
		for _, chapter := range chapters {
			for _, v := range chapter {
				verses++
				for _, r := range v.Text {
					if r == '\n' {
						breaks++
						break
					}
				}
				if len(v.Footnotes) > 0 {
					notes++
				}
				if v.ParaStart {
					starts++
				}
			}
		}
	}
	if verses != 3778 {
		t.Errorf("seed carries %d verses, want 3778", verses)
	}
	// Each of these was zero in the seed this replaced, which is what made its
	// poetry read as prose and its pages break where no publisher said.
	for _, tc := range []struct {
		what string
		got  int
	}{
		{"verses with authored poem lines", breaks},
		{"verses carrying the translators' notes", notes},
		{"verses opening a publisher's paragraph", starts},
	} {
		if tc.got == 0 {
			t.Errorf("%s: none — the seed predates the decoder that produced it", tc.what)
		}
	}
	t.Logf("seed: %d verses, %d with poem lines, %d with notes, %d opening a paragraph", verses, breaks, notes, starts)
}

// The Beatitudes are the clearest case: poetry in the Gospels, and prose in
// the seed that this replaced.
func TestSeedSetsTheBeatitudesAsPoetry(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatal(err)
	}
	paras := groupVersesIntoParagraphs(bd.Verses["Matthew"][5])
	if len(paras) < 4 {
		t.Errorf("Matthew 5 opens %d paragraphs; the publisher sets many more", len(paras))
	}
	if len(bd.Verses["John"][1]) == 0 {
		t.Fatal("the seed is missing John 1")
	}
}
