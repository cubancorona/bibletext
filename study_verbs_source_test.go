package bibletext

// The Study with AI verbs are listed once in Go (aiStudyVerbs) and copied
// into four native sources no Go build compiles: the iPhone and iPad text
// view's make() list (reading_ios.go), the Mac text view's label/action pairs
// and the hbAI_<action>: selectors AppKit resolves from them by name
// (reading_macos.go), the Android popup's items (android/BtBridge.java) and
// the Android export's case list that admits an action into the AI dispatch
// (reading_android_export.go). The Fyne pane ranges over aiStudyVerbs itself
// (reading_menu_test.go). Each copy is parsed out of its source and held to
// the complete ungated study list in order, so a verb added to one surface and not another, a
// label that drifts, or a Mac pair whose selector has no method, which is a
// silent no-op at run time, fails here.
//
// A parser that matched nothing would hold its source to nothing, so each is
// first proven on a fixture of its surface's shape, the source as it stood
// with three verbs: it must find exactly those three, and the hold must then
// refuse an incomplete list. Runtime gates are checked separately.

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// nativeVerbSource is one native copy of the verb list: where it lives, how
// its label/action pairs are read, and the fixture of its shape carrying the
// first three verbs.
type nativeVerbSource struct {
	path    string
	parse   func(src string) []aiStudyVerb
	fixture string
}

var (
	// iOS: make(@"Label", @"action") calls in the Study with AI menu's
	// children list.
	iosStudyMenu = regexp.MustCompile(`(?s)menuWithTitle:@"Study with AI".*?children:@\[(.*?)\]\];`)
	iosStudyMake = regexp.MustCompile(`make\(@"([^"]*)",\s*@"([^"]*)"\)`)
	// Mac: @[@"Label", @"action"] pairs in the loop that builds the submenu,
	// spread over several lines.
	macStudyPairs = regexp.MustCompile(`(?s)for \(NSArray \*pair in @\[(.*?)\]\) \{`)
	macStudyPair  = regexp.MustCompile(`@\[@"([^"]*)",\s*@"([^"]*)"\]`)
	// Mac: the hbAI_<action>: method a pair's selector names, and the action
	// string it sends.
	macStudyMethod = regexp.MustCompile(`- \(void\)hbAI_([A-Za-z]+):\(id\)sender\s*\{\s*\[self hbSendAI:"([^"]*)"\];\s*\}`)
	// Android: the {"Label", "action"} rows of the items literal, spread over
	// several lines.
	javaStudyItems = regexp.MustCompile(`(?s)String\[\]\[\] items = \{(.*?)\};`)
	javaStudyItem  = regexp.MustCompile(`\{"([^"]*)",\s*"([^"]*)"\}`)
	// Android export: the case list that sends an action to dispatchAIAction.
	exportStudyCase = regexp.MustCompile(`(?s)case ((?:"[a-z]+",?\s*)+):\s*dispatchAIAction\(`)
	exportQuoted    = regexp.MustCompile(`"([a-z]+)"`)
)

// pairsIn reads label/action pairs with pair from the first group block
// matches in src, in order; nothing when block matches nothing.
func pairsIn(src string, block, pair *regexp.Regexp) []aiStudyVerb {
	m := block.FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	var out []aiStudyVerb
	for _, p := range pair.FindAllStringSubmatch(m[1], -1) {
		out = append(out, aiStudyVerb{Label: p[1], Action: p[2]})
	}
	return out
}

func parseIOSStudyMenu(src string) []aiStudyVerb {
	if start := strings.Index(src, "NSMutableArray<UIMenuElement *> *aiItems ="); start >= 0 {
		if end := strings.Index(src[start:], "UIMenu *ai ="); end >= 0 {
			var out []aiStudyVerb
			for _, p := range iosStudyMake.FindAllStringSubmatch(src[start:start+end], -1) {
				out = append(out, aiStudyVerb{Label: p[1], Action: p[2]})
			}
			return out
		}
	}
	return pairsIn(src, iosStudyMenu, iosStudyMake)
}
func parseMacStudyPairs(src string) []aiStudyVerb { return pairsIn(src, macStudyPairs, macStudyPair) }
func parseJavaStudyItems(src string) []aiStudyVerb {
	return pairsIn(src, javaStudyItems, javaStudyItem)
}

// macStudyMethods reads each hbAI_<action>: method as the action its selector
// names and the action it sends, in order.
func macStudyMethods(src string) [][2]string {
	var out [][2]string
	for _, m := range macStudyMethod.FindAllStringSubmatch(src, -1) {
		out = append(out, [2]string{m[1], m[2]})
	}
	return out
}

// exportStudyActions reads the action strings the Android export's AI case
// admits, in order; nothing when there is no such case.
func exportStudyActions(src string) []string {
	m := exportStudyCase.FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range exportQuoted.FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

// holdStudyVerbs says what is wrong with got against aiStudyVerbs, or nil.
func holdStudyVerbs(got []aiStudyVerb) error {
	if len(got) != len(aiStudyVerbsForAsk(true)) {
		return fmt.Errorf("%d verbs, want the %d of aiStudyVerbs: %v", len(got), len(aiStudyVerbsForAsk(true)), got)
	}
	for i, v := range aiStudyVerbsForAsk(true) {
		if got[i] != v {
			return fmt.Errorf("verb %d is %+v, want %+v", i, got[i], v)
		}
	}
	return nil
}

// holdMacStudyMethods says what is wrong with the Mac's hbAI_ methods against
// aiStudyVerbs, or nil: one method per verb, in order, sending the action its
// selector names.
func holdMacStudyMethods(got [][2]string) error {
	if len(got) != len(aiStudyVerbsForAsk(true)) {
		return fmt.Errorf("%d hbAI_ methods, want one per verb of aiStudyVerbs (%d): %v", len(got), len(aiStudyVerbsForAsk(true)), got)
	}
	for i, v := range aiStudyVerbsForAsk(true) {
		if got[i][0] != v.Action {
			return fmt.Errorf("method %d is hbAI_%s:, want hbAI_%s:", i, got[i][0], v.Action)
		}
		if got[i][1] != v.Action {
			return fmt.Errorf("hbAI_%s: sends %q", got[i][0], got[i][1])
		}
	}
	return nil
}

// studyVerbActions is aiStudyVerbs' action strings, in order.
func studyVerbActions() []string {
	var out []string
	for _, v := range aiStudyVerbsForAsk(true) {
		out = append(out, v.Action)
	}
	return out
}

// The fixtures: each surface's source as it stood with three verbs, in its
// own shape, so the parsers are proven on the lines they must read.
const (
	iosStudyFixture = `    UIMenu *ai = [UIMenu menuWithTitle:@"Study with AI" image:nil identifier:nil options:0
                              children:@[
                                  make(@"Explain", @"explain"),
                                  make(@"Analyze context", @"context"),
                                  make(@"Analyze translation", @"translation"),
                              ]];

    // Share verse → with citation / as image (both go to the iOS share sheet).
    UIAction * (^study)(NSString *, NSString *) = ^UIAction *(NSString *title, NSString *act) {
`
	macStudyPairsFixture = `        NSMenu *ai = [[NSMenu alloc] initWithTitle:@"Study with AI"];
        for (NSArray *pair in @[@[@"Explain", @"explain"],
                                @[@"Analyze context", @"context"],
                                @[@"Analyze translation", @"translation"]]) {
            SEL action = NSSelectorFromString([NSString stringWithFormat:@"hbAI_%@:", pair[1]]);
`
	macStudyMethodsFixture = `- (void)hbAI_explain:(id)sender     { [self hbSendAI:"explain"]; }
- (void)hbAI_context:(id)sender     { [self hbSendAI:"context"]; }
- (void)hbAI_translation:(id)sender { [self hbSendAI:"translation"]; }
- (void)hbCrossRefs:(id)sender      { [self hbSendStudy:"crossref"]; }
`
	javaStudyFixture = `        String[][] items = {{"Explain", "explain"}, {"Analyze context", "context"},
                            {"Analyze translation", "translation"}};
        for (String[] it : items) {
`
	exportStudyFixture = `	fyne.Do(func() {
		switch action {
		case "ask", "explain", "context", "translation":
			dispatchAIAction(state, action, text, span)
		default:
			dispatchSelectionAction(state, action, text, span)
		}
	})
`
)

func TestStudyVerbSetAgreesAcrossMenus(t *testing.T) {
	if len(aiStudyVerbsForAsk(true)) != 5 {
		t.Fatalf("aiStudyVerbs has %d verbs; the fixtures here carry the first three of five, so a new verb means re-baking them", len(aiStudyVerbsForAsk(true)))
	}
	three := slices.Clone(aiStudyVerbsForAsk(true)[:3])

	for _, s := range []nativeVerbSource{
		{"reading_ios.go", parseIOSStudyMenu, iosStudyFixture},
		{"reading_macos.go", parseMacStudyPairs, macStudyPairsFixture},
		{"android/BtBridge.java", parseJavaStudyItems, javaStudyFixture},
	} {
		t.Run(s.path, func(t *testing.T) {
			// The parser reads the fixture as exactly the first three verbs,
			// and the hold refuses them as short of the fourth; a source the
			// parser cannot read, a swapped order and a drifted label are
			// refused too.
			got := s.parse(s.fixture)
			if !slices.Equal(got, three) {
				t.Fatalf("control: the parser reads the fixture as %v, not the first three verbs", got)
			}
			if holdStudyVerbs(got) == nil {
				t.Fatal("control: a list short of the fourth verb passed")
			}
			if holdStudyVerbs(s.parse("")) == nil {
				t.Fatal("control: a source the parser cannot read passed")
			}
			swapped := slices.Clone(aiStudyVerbsForAsk(true))
			swapped[2], swapped[3] = swapped[3], swapped[2]
			if holdStudyVerbs(swapped) == nil {
				t.Fatal("control: a swapped order passed")
			}
			drifted := slices.Clone(aiStudyVerbsForAsk(true))
			drifted[3].Label += " "
			if holdStudyVerbs(drifted) == nil {
				t.Fatal("control: a drifted label passed")
			}

			if err := holdStudyVerbs(s.parse(readNativeSource(t, s.path))); err != nil {
				t.Errorf("%s: %v", s.path, err)
			}
		})
	}

	t.Run("reading_macos.go hbAI_ selectors", func(t *testing.T) {
		got := macStudyMethods(macStudyMethodsFixture)
		if len(got) != 3 || got[0] != [2]string{"explain", "explain"} || got[2] != [2]string{"translation", "translation"} {
			t.Fatalf("control: the parser reads the fixture as %v, not the first three methods", got)
		}
		if holdMacStudyMethods(got) == nil {
			t.Fatal("control: a Mac without the fourth verb's method passed")
		}
		mismatched := [][2]string{{"explain", "explain"}, {"context", "context"}, {"translation", "translation"}, {"compare", "explain"}}
		if holdMacStudyMethods(mismatched) == nil {
			t.Fatal("control: a method sending another verb's action passed")
		}

		src := readNativeSource(t, "reading_macos.go")
		if err := holdMacStudyMethods(macStudyMethods(src)); err != nil {
			t.Errorf("reading_macos.go: %v", err)
		}
		// The source with the fourth verb's method deleted is refused: the
		// pair's selector would resolve to nothing and the item would do
		// nothing when chosen.
		compareMethod := regexp.MustCompile(`(?m)^.*hbAI_compare:.*\n`)
		without := compareMethod.ReplaceAllString(src, "")
		if without == src {
			t.Fatal("control: the source has no hbAI_compare: method to delete")
		}
		if holdMacStudyMethods(macStudyMethods(without)) == nil {
			t.Error("control: the source without hbAI_compare: passed")
		}
	})

	t.Run("reading_android_export.go", func(t *testing.T) {
		// The export admits Ask first; its dispatch checks the same gate as the menu.
		want := append([]string{aiActionAsk}, studyVerbActions()[:4]...)
		got := exportStudyActions(exportStudyFixture)
		if !slices.Equal(got, []string{"ask", "explain", "context", "translation"}) {
			t.Fatalf("control: the parser reads the fixture as %v", got)
		}
		if slices.Equal(got, want) {
			t.Fatal("control: a case list short of the fourth verb passed")
		}
		if got := exportStudyActions(""); got != nil {
			t.Fatalf("control: an empty source reads as %v", got)
		}

		src := readNativeSource(t, "reading_android_export.go")
		if got := exportStudyActions(src); !slices.Equal(got, want) {
			t.Errorf("reading_android_export.go admits %v to the AI dispatch, want %v", got, want)
		}
	})

	// The labels themselves are the native copies' and the Fyne pane's alike:
	// a native label is never a Go identifier, so a parser reading one as the
	// action string is caught by the hold, which compares both fields.
	for _, v := range aiStudyVerbsForAsk(true) {
		if strings.TrimSpace(v.Label) == "" || v.Label == v.Action {
			t.Errorf("verb %+v has no label of its own", v)
		}
	}
}
