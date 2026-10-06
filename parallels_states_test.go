package bibletext

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// THE NEXT RELEASE'S SYNOPSIS DIFFERS FROM THE SHIPPING ONE ONLY IN THE
// TWENTY-TWO VERSES. gospel_parallels_next.json is gospel_parallels.json with
// the Gospel verses that were in no set placed (docs/TEXTUAL-DATA.md §9.4,
// docs/NEXT.md): three sets gain the passage of a Gospel they lacked, and
// four sets of one Gospel each are added, each where the harmony puts it.
// Every other set is the same, byte for byte, in the same order. Read from the
// files themselves, in both states of the switch, so a change made to one file
// and not the other fails here; a fix to the synopsis goes into both.
func TestTheNextReleasesSynopsisDiffersOnlyInTheTwentyTwoVerses(t *testing.T) {
	read := func(path string) []json.RawMessage {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var sets []json.RawMessage
		if err := json.Unmarshal(data, &sets); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return sets
	}
	type set struct {
		ID      int                `json:"id"`
		Section string             `json:"section"`
		Title   string             `json:"title"`
		Refs    map[string]*string `json:"refs"`
	}
	decode := func(raw json.RawMessage) set {
		var s set
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	str := func(s string) *string { return &s }

	// The sets that gain a Gospel's passage, by id: the passage each Gospel
	// has in the next release where it differs.
	extended := map[int]map[string]*string{
		39: {"matthew": str("4:23")},                                     // Preaching tour of Galilee
		49: {"matthew": str("4:24-25,12:15-21"), "luke": str("6:17-19")}, // Healing the multitudes by the sea
		71: {"luke": str("6:43-45")},                                     // A tree and its fruit
	}
	// The sets added, by id, each after the shipping set it follows.
	added := map[int]struct {
		after int
		title string
		book  string
		ref   string
	}{
		268: {51, "Woes on the rich and the satisfied", "luke", "6:24-26"},
		270: {184, "The Passover draws near", "john", "11:55-57"},
		269: {220, "Teaching daily in the Temple", "luke", "21:37-38"},
		271: {231, "The new commandment", "john", "13:31-35"},
	}

	cur := read("assets/parallels/gospel_parallels.json")
	next := read("assets/parallels/gospel_parallels_next.json")
	// CONTROL: there is a synopsis to compare.
	if len(cur) < 200 {
		t.Fatalf("the shipping synopsis holds %d sets; the comparison proves nothing", len(cur))
	}
	if len(next) != len(cur)+len(added) {
		t.Errorf("the next synopsis holds %d sets, want the shipping %d and %d more", len(next), len(cur), len(added))
	}
	seenExtended, seenAdded := 0, 0
	i := 0
	for j := 0; j < len(next); j++ {
		n := decode(next[j])
		if a, ok := added[n.ID]; ok {
			seenAdded++
			prev := -1
			if i > 0 {
				prev = decode(cur[i-1]).ID
			}
			want := map[string]*string{"matthew": nil, "mark": nil, "luke": nil, "john": nil}
			want[a.book] = str(a.ref)
			if prev != a.after || n.Title != a.title || !reflect.DeepEqual(n.Refs, want) {
				t.Errorf("added set %d is %q %s after set %d, want %q %s %s after set %d",
					n.ID, n.Title, refsString(n.Refs), prev, a.title, a.book, a.ref, a.after)
			}
			continue
		}
		if i >= len(cur) {
			t.Errorf("the next synopsis has set %d (%q) past the shipping one's end", n.ID, n.Title)
			continue
		}
		c := decode(cur[i])
		i++
		if c.ID != n.ID {
			t.Errorf("set %d stands where the shipping synopsis has set %d", n.ID, c.ID)
			continue
		}
		gains, ok := extended[n.ID]
		if !ok {
			if string(cur[i-1]) != string(next[j]) {
				t.Errorf("set %d (%q) differs between the two files", n.ID, n.Title)
			}
			continue
		}
		seenExtended++
		want := map[string]*string{}
		for k, v := range c.Refs {
			want[k] = v
		}
		for k, v := range gains {
			want[k] = v
		}
		if n.Section != c.Section || n.Title != c.Title || !reflect.DeepEqual(n.Refs, want) {
			t.Errorf("set %d (%q) is %s, want the shipping %s with %s", n.ID, n.Title,
				refsString(n.Refs), refsString(c.Refs), refsString(gains))
		}
	}
	if i != len(cur) {
		t.Errorf("the next synopsis lacks the shipping sets from set %d on", decode(cur[i]).ID)
	}
	if seenExtended != len(extended) || seenAdded != len(added) {
		t.Errorf("found %d of the %d sets extended and %d of the %d added", seenExtended, len(extended), seenAdded, len(added))
	}
}

func refsString(refs map[string]*string) string {
	var parts []string
	for _, g := range gospelColumns {
		if r, ok := refs[g.key]; ok && r != nil {
			parts = append(parts, fmt.Sprintf("%s %s", g.book, *r))
		}
	}
	return "[" + strings.Join(parts, "; ") + "]"
}
