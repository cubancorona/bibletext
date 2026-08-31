package bibletext

import (
	"fmt"
	"regexp"
	"testing"
)

// The verb set crosses four ABIs as a bare int. Nothing checks that the names
// on the far side mean what the iota means here: swap two of them and every
// build is clean, every test that reads the SOURCE still passes, and the only
// symptom is a bin on a card whose press puts the note away. So the numbers are
// read out of each native's own enum and held against the Go constants.
func TestPushedVerbSetsAgreeWithTheIota(t *testing.T) {
	want := map[string]int{
		"None":     int(noteVerbsNone),
		"Received": int(noteVerbsReceived),
		"Own":      int(noteVerbsOwn),
	}
	// Guard the guard: if the Go side ever collapses two of these onto one
	// number, comparing names to numbers stops distinguishing anything.
	if want["None"] == want["Received"] || want["Received"] == want["Own"] {
		t.Fatalf("the Go verb sets are not distinct: %v", want)
	}

	for _, c := range []struct{ path, prefix string }{
		{"reading_ios.go", "kNoteVerbs"},
		{"reading_macos.go", "kMacNoteVerbs"},
		{"android/BtBridge.java", ""},
	} {
		t.Run(c.path, func(t *testing.T) {
			src := readNativeSource(t, c.path)
			for name, n := range want {
				var re *regexp.Regexp
				if c.prefix == "" {
					// Java: a single `static final int VERBS_A = 0, VERBS_B = 1, ...`
					re = regexp.MustCompile(`VERBS_` + upper(name) + `\s*=\s*(\d+)`)
				} else {
					re = regexp.MustCompile(c.prefix + name + `\s*=\s*(\d+)`)
				}
				m := re.FindStringSubmatch(src)
				if m == nil {
					t.Errorf("%s never names the %q verb set — the int it receives "+
						"is being compared against something else", c.path, name)
					continue
				}
				var got int
				fmt.Sscanf(m[1], "%d", &got)
				if got != n {
					t.Errorf("%s says %s = %d, the Go iota says %d. The glyph this "+
						"picks is a promise about what the press does; off by one, "+
						"it promises the wrong thing and nothing fails but the app.",
						c.path, name, got, n)
				}
			}
		})
	}
}

func upper(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' {
			ch -= 32
		}
		out[i] = ch
	}
	return string(out)
}

// The arrival classes cross the ABI as a bare int too, and the same hazard
// applies: swap two names on the far side and every build is clean, every
// source test still passes, and the only symptom is a reader landing in the
// wrong place — which looks exactly like "nothing happened".
func TestPushedArrivalClassesAgreeWithTheIota(t *testing.T) {
	want := map[string]int{
		"Nothing": int(arriveNothing),
		"Verse":   int(arriveVerse),
		"Band":    int(arriveBand),
	}
	if want["Nothing"] == want["Verse"] || want["Verse"] == want["Band"] {
		t.Fatalf("the Go arrival classes are not distinct: %v", want)
	}
	for _, c := range []struct{ path, prefix string }{
		{"reading_ios.go", "kArrive"},
		{"reading_macos.go", "kMacArrive"},
		{"android/BtBridge.java", ""},
	} {
		t.Run(c.path, func(t *testing.T) {
			src := readNativeSource(t, c.path)
			for name, n := range want {
				var re *regexp.Regexp
				if c.prefix == "" {
					re = regexp.MustCompile(`ARRIVE_` + upper(name) + `\s*=\s*(\d+)`)
				} else {
					re = regexp.MustCompile(c.prefix + name + `\s*=\s*(\d+)`)
				}
				m := re.FindStringSubmatch(src)
				if m == nil {
					t.Errorf("%s never names the %q arrival class", c.path, name)
					continue
				}
				var got int
				fmt.Sscanf(m[1], "%d", &got)
				if got != n {
					t.Errorf("%s says %s = %d, the Go iota says %d. Off by one, the "+
						"renderer places the view somewhere nobody chose and nothing "+
						"fails but the app.", c.path, name, got, n)
				}
			}
		})
	}
}
