//go:build darwin && !ios

package bibletext

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestMacPageReflowSelectionIntent(t *testing.T) {
	for _, test := range []struct {
		name                   string
		same, page, navigation bool
		keep                   bool
	}{
		{"same chapter changing page", true, true, false, true},
		{"explicit arrival changing page", true, true, true, false},
		{"different chapter changing page", false, true, false, false},
		{"same page ordinary repaint", true, false, false, false},
		{"different chapter same page", false, false, false, false},
	} {
		if got := macSelectionCarryWanted(test.same, test.page, test.navigation); got != test.keep {
			t.Errorf("%s: selection carry=%v, want %v", test.name, got, test.keep)
		}
	}
}

// macSelectionGuardSource is the pane's own selection guard, with the
// content-end finder it reads, as Objective-C to compile on the host.
func macSelectionGuardSource(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, signature := range []string{
		"static NSUInteger btMacPageContentEnd(NSAttributedString *ts) {",
		"static BOOL btMacSelectionChapterMatches(NSString *storedBook, int storedChapter, NSString *targetBook, int targetChapter) {",
		"static BOOL btMacCanPreserveSelection(BOOL requested, BOOL sameChapter, NSAttributedString *before, NSAttributedString *after, NSArray<NSValue *> *ranges) {",
	} {
		b.WriteString(nativeFunctionSource(t, "reading_macos.go", signature))
		b.WriteString("\n}\n")
	}
	return b.String()
}

// runMacNativeProgram compiles an Objective-C program against AppKit and runs
// it with args; a failure to build or a nonzero exit fails the test with the
// program's output.
func runMacNativeProgram(t *testing.T, what, program string, args ...string) {
	t.Helper()
	dir := t.TempDir()
	source, binary := filepath.Join(dir, "program.m"), filepath.Join(dir, "program")
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("xcrun", "clang", "-fobjc-arc", "-framework", "AppKit", "-framework", "CoreText", "-framework", "Foundation", source, "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile %s: %v\n%s", what, err, output)
	}
	if output, err := exec.Command(binary, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", what, err, output)
	}
}

func TestMacPageReflowSelectionValidatesActualTextAndRanges(t *testing.T) {
	program := `#import <AppKit/AppKit.h>
` + macSelectionGuardSource(t) + `
// A page with no fonts, so its content end is its length.
static NSAttributedString *P(NSString *s) { return s == nil ? nil : [[NSAttributedString alloc] initWithString:s]; }
// A page shaped like the pane's: Scripture at the body size, then a footnote
// section at 0.85 of it.
static NSAttributedString *Page(NSString *scripture, NSString *section) {
    NSMutableAttributedString *page = [[NSMutableAttributedString alloc] initWithString:scripture
        attributes:@{NSFontAttributeName: [NSFont systemFontOfSize:20]}];
    if (section != nil) [page appendAttributedString:[[NSAttributedString alloc] initWithString:section
        attributes:@{NSFontAttributeName: [NSFont systemFontOfSize:17]}]];
    return page;
}
int main(void) { @autoreleasepool {
    NSString *before = @"First repeated phrase. First repeated phrase. 🌿 שלום";
    NSRange second = [before rangeOfString:@"First" options:0 range:NSMakeRange(10, before.length-10)];
    NSArray *valid = @[[NSValue valueWithRange:second]];
    if (!btMacCanPreserveSelection(YES, YES, P(before), P([[NSMutableString alloc] initWithString:before]), valid)) return 1;
    if (btMacCanPreserveSelection(NO, YES, P(before), P(before), valid)) return 2;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(@"Different repeated phrase. 🌿 שלום"), valid)) return 3;
    if (btMacCanPreserveSelection(YES, YES, P(before), P([before stringByAppendingString:@" "]), valid)) return 4;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(before), @[])) return 5;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(before), @[[NSValue valueWithRange:NSMakeRange(3,0)]])) return 6;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(before), @[[NSValue valueWithRange:NSMakeRange(NSNotFound,2)]])) return 7;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(before), @[[NSValue valueWithRange:NSMakeRange(before.length-1,3)]])) return 8;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(before), @[[NSValue valueWithRange:NSMakeRange(2,NSUIntegerMax)]])) return 9;
    if (!btMacCanPreserveSelection(YES, YES, P(before), P(before), @[valid[0], [NSValue valueWithRange:NSMakeRange(before.length-4,4)]])) return 10;
    if (btMacCanPreserveSelection(YES, YES, nil, P(before), valid)) return 11;
    BOOL differentBook = btMacSelectionChapterMatches(@"Synthetic A",1,@"Synthetic B",1);
    BOOL differentChapter = btMacSelectionChapterMatches(@"Synthetic A",1,@"Synthetic A",2);
    if (btMacCanPreserveSelection(YES, differentBook, P(before), P(before), valid)) return 12;
    if (btMacCanPreserveSelection(YES, differentChapter, P(before), P(before), valid)) return 13;
    if (!btMacSelectionChapterMatches([[NSMutableString alloc] initWithString:@"Synthetic A"],1,@"Synthetic A",1)) return 14;
    if (btMacSelectionChapterMatches(nil,1,@"Synthetic A",1)) return 15;

    // Pages that differ only after the Scripture text, as the phone and book
    // pages do with the footnotes shown: the book page's section opens with a
    // line break.
    NSString *scripture = @"1 Synthetic words of a verse. 2 More synthetic words.\n";
    NSAttributedString *phone = Page(scripture, @"    \n1 A synthetic note.\n");
    NSAttributedString *book = Page(scripture, @"     \n1 A synthetic note.\n");
    NSRange words = [scripture rangeOfString:@"words of a verse. 2 More"];
    NSArray *inScripture = @[[NSValue valueWithRange:words]];
    if (!btMacCanPreserveSelection(YES, YES, phone, book, inScripture)) return 16;
    if (!btMacCanPreserveSelection(YES, YES, book, phone, inScripture)) return 17;
    if (!btMacCanPreserveSelection(YES, YES, phone, book, @[[NSValue valueWithRange:NSMakeRange(words.location, scripture.length-words.location)]])) return 18;
    if (btMacCanPreserveSelection(YES, YES, phone, book, @[[NSValue valueWithRange:NSMakeRange(words.location, scripture.length-words.location+1)]])) return 19;
    if (btMacCanPreserveSelection(YES, YES, phone, book, @[inScripture[0], [NSValue valueWithRange:NSMakeRange(scripture.length+2, 3)]])) return 20;
    if (btMacCanPreserveSelection(YES, YES, phone, Page(@"1 Synthetic words of a verse. 2 Some synthetic words.\n", @"     \n1 A synthetic note.\n"), inScripture)) return 21;
    if (btMacCanPreserveSelection(YES, YES, phone, Page([scripture stringByAppendingString:@"3 "], @"     \n1 A synthetic note.\n"), inScripture)) return 22;
    if (btMacCanPreserveSelection(YES, differentChapter, phone, book, inScripture)) return 23;
    if (btMacCanPreserveSelection(YES, YES, phone, book, @[[NSValue valueWithRange:NSMakeRange(NSNotFound,2)]])) return 24;
    if (btMacCanPreserveSelection(NO, YES, phone, book, inScripture)) return 25;
    if (btMacCanPreserveSelection(YES, YES, P(before), P(@"First repeated phrase. Fixed repeated phrase. 🌿 שלום"), valid)) return 26;
    return 0;
} }
`
	runMacNativeProgram(t, "native selection guard over its behavioral cases", program)
}

// With the footnotes shown, the book page writes a line break before the
// footnote separator that the phone page does not (writeFootnoteSection), so
// the two pages a same-chapter page-format import moves between differ after
// the Scripture text. Here the pages are buildChapterHTML's, imported by the
// pane's own btMacImportHTML, the carry is decided by the pane's own guard,
// and the place is checked where the restore finds it: each verse's number
// (btMacLocForVerse, which the reading-position restore scrolls to) must stand
// at the same character on both pages.
func TestMacPageReflowSelectionCarriesWithFootnotesShown(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer setFootnotesEnabled(false)
	page := func(footnotes, book bool, verses []Verse) string {
		setFootnotesEnabled(footnotes)
		var html string
		withReporterLayout(book, func() { html = buildChapterHTML(footnoteFixtureState(verses), verses) })
		return html
	}
	// The same length as the fixture's words, so only the comparison of the
	// Scripture text, not its length, can refuse the carry.
	changed := footnoteFixtureVerses()
	changed[1].Text = "God did not send his Son to condemn the earth."
	dir := t.TempDir()
	var paths []string
	for _, p := range []struct {
		name string
		html string
	}{
		{"phone-footnotes.html", page(true, false, footnoteFixtureVerses())},
		{"book-footnotes.html", page(true, true, footnoteFixtureVerses())},
		{"phone.html", page(false, false, footnoteFixtureVerses())},
		{"book.html", page(false, true, footnoteFixtureVerses())},
		{"book-footnotes-changed.html", page(true, true, changed)},
	} {
		path := filepath.Join(dir, p.name)
		if err := os.WriteFile(path, []byte(p.html), 0600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	importer := nativeFunctionSource(t, "reading_macos.go", "static NSMutableAttributedString *btMacImportHTML(NSData *data) {")
	threshold := nativeFunctionSource(t, "reading_macos.go", "static CGFloat btMacVerseFontThreshold(NSTextStorage *ts) {")
	verseAt := nativeFunctionSource(t, "reading_macos.go", "static NSUInteger btMacLocForVerse(NSTextStorage *ts, NSInteger verse) {")
	program := `#import <AppKit/AppKit.h>
#import <CoreText/CoreText.h>
// What btMacImportHTML reads beside the HTML. The leading and the indent are
// paragraph attributes and the reading face keeps each run's size, so none of
// them changes the string or a run's size, which are all the guard reads.
static int gBTMacFailImports = 0;
static uint64_t btMacPerfNow(void) { return 0; }
static void btMacPerfLog(const char *what, uint64_t t0) { (void)what; (void)t0; }
static CGFloat gReadingLinePitchEm = 0;
static CGFloat gMacReporterIndent = 0;
static CTFontRef btReadingFontLike(CTFontRef src) { (void)src; return NULL; }
` + importer + "\n}\n" + threshold + "\n}\n" + verseAt + "\n}\n" + macSelectionGuardSource(t) + `
static int failures = 0;
static void expect(BOOL ok, NSString *what) {
    if (!ok) { failures++; printf("%s\n", what.UTF8String); }
}
static NSAttributedString *load(const char *path) {
    NSData *data = [NSData dataWithContentsOfFile:[NSString stringWithUTF8String:path]];
    NSAttributedString *page = data != nil ? btMacImportHTML(data) : nil;
    if (page == nil) { printf("could not import %s\n", path); exit(2); }
    return page;
}
static NSArray *one(NSRange r) { return @[[NSValue valueWithRange:r]]; }
int main(int argc, const char **argv) { @autoreleasepool {
    if (argc != 6) return 3;
    NSAttributedString *phone = load(argv[1]), *book = load(argv[2]);
    NSAttributedString *phoneHidden = load(argv[3]), *bookHidden = load(argv[4]), *changed = load(argv[5]);
    NSUInteger end = btMacPageContentEnd(phone);
    expect(![phone.string isEqualToString:book.string],
        @"the phone and book pages import identical strings with the footnotes shown, so the pages no longer differ as this test expects");
    expect(end > 0 && end < phone.length && end == btMacPageContentEnd(book),
        @"the footnote section's start was not found at the same place on both pages");
    NSRange words = [phone.string rangeOfString:@"so loved the world" options:0 range:NSMakeRange(0, end)];
    NSRange from = [phone.string rangeOfString:@"loved" options:0 range:NSMakeRange(0, end)];
    NSRange to = [phone.string rangeOfString:@"believes" options:0 range:NSMakeRange(0, end)];
    if (words.location == NSNotFound || from.location == NSNotFound || to.location == NSNotFound) { printf("fixture words not found\n"); return 4; }
    NSRange span = NSMakeRange(from.location, NSMaxRange(to) - from.location);
    NSRange toEnd = NSMakeRange(words.location, end - words.location);
    NSRange intoNotes = NSMakeRange(to.location, end - to.location + 3);

    expect(btMacCanPreserveSelection(YES, YES, phone, book, one(words)),
        @"phone page to book page, footnotes shown: a selection of Scripture words did not carry");
    expect(btMacCanPreserveSelection(YES, YES, book, phone, one(words)),
        @"book page to phone page, footnotes shown: a selection of Scripture words did not carry");
    expect(btMacCanPreserveSelection(YES, YES, phone, book, one(span)),
        @"footnotes shown: a selection across three verses did not carry");
    expect(btMacCanPreserveSelection(YES, YES, phone, book, one(toEnd)),
        @"footnotes shown: a selection ending where the Scripture ends did not carry");
    expect([[phone.string substringWithRange:span] isEqualToString:[book.string substringWithRange:span]],
        @"the carried range names other characters on the book page");
    expect(!btMacCanPreserveSelection(YES, YES, phone, book, one(intoNotes)) &&
        !btMacCanPreserveSelection(YES, YES, book, phone, one(intoNotes)),
        @"a selection reaching into the footnote section carried, where the two pages hold other characters");
    expect(!btMacCanPreserveSelection(YES, YES, phone, changed, one(words)),
        @"footnotes shown: a selection carried to a page whose words changed");
    expect(!btMacCanPreserveSelection(YES, NO, phone, book, one(words)),
        @"footnotes shown: a selection carried to another chapter");
    expect(!btMacCanPreserveSelection(YES, YES, phone, book, one(NSMakeRange(NSNotFound, 2))) &&
        !btMacCanPreserveSelection(YES, YES, phone, book, one(NSMakeRange(phone.length - 1, 3))),
        @"footnotes shown: an invalid range carried");
    expect([phoneHidden.string isEqualToString:bookHidden.string] &&
        btMacCanPreserveSelection(YES, YES, phoneHidden, bookHidden, one(span)),
        @"footnotes hidden: a selection of Scripture words did not carry");

    NSTextStorage *phoneStorage = [[NSTextStorage alloc] initWithAttributedString:phone];
    NSTextStorage *bookStorage = [[NSTextStorage alloc] initWithAttributedString:book];
    for (NSInteger verse = 16; verse <= 18; verse++) {
        NSUInteger a = btMacLocForVerse(phoneStorage, verse), b = btMacLocForVerse(bookStorage, verse);
        expect(a != NSNotFound && a == b && a < end,
            [NSString stringWithFormat:@"footnotes shown: verse %ld's number is not at the same character inside the Scripture on both pages (%lu, %lu)", (long)verse, (unsigned long)a, (unsigned long)b]);
    }
    return failures == 0 ? 0 : 1;
} }
`
	runMacNativeProgram(t, "native selection carry over the real pages with the footnotes shown", program, paths...)
}

func TestMacPageReflowSelectionStaysWithItsPush(t *testing.T) {
	begin := nativeFunctionSource(t, "reading_macos.go", "int bibleTextMacBeginChapterPush(int sameChapter) {")
	if !inSequence(begin, "if (gen <= gBodyGenPending) return;", "gBodyGenPending = gen;", "gMacPreserveSelectionForPush = NO;") {
		t.Fatal("a newer chapter push did not invalidate the previous selection intent")
	}
	setter := nativeFunctionSource(t, "reading_macos.go", "void bibleTextMacPreserveSelectionForPush(int gen, int preserve, const char *book, int chapter) {")
	if !strings.Contains(setter, "if (gen != gBodyGenPending) return;") {
		t.Fatal("a stale push can change the current selection intent")
	}
	install := nativeFunctionSource(t, "reading_macos.go", "static void btMacInstallPageLatched(NSAttributedString *as) {")
	if !strings.Contains(install, "btMacSelectionChapterMatches(gMacTextBook, gMacTextChapter,") || !strings.Contains(install, "gMacSelectionTargetBook, gMacSelectionTargetChapter)") ||
		!strings.Contains(install, "gMacPreserveSelectionForPush, sameChapter,") {
		t.Fatal("a selection from the previous storage chapter can carry across a failed newer import")
	}
	if !inSequence(install, "gTextView.selectedRanges", "btMacCanPreserveSelection(", "gTextView.textStorage, as, selected);", "setAttributedString:as", "setSelectedRanges:selected affinity:affinity stillSelecting:NO", "bibleTextMacScrollTV();") {
		t.Fatal("a successful page install does not validate the live storage against the new page and restore the selection before final scroll placement")
	}
	host := nativeFunctionSource(t, "reading_macos.go", "func newMacReadingHost(state *AppState, verses []Verse) *macReadingHost {")
	if !inSequence(host, "macSelectionCarryWanted(sameChapter != 0, pageChanged, state.forceReposition)", "state.forceReposition = false", "C.bibleTextMacBeginChapterPush(sameChapter)", "C.bibleTextMacPreserveSelectionForPush(gen, preserveSelection, cb, C.int(state.CurrentChapter))", "C.bibleTextMacTVSetHTML(") {
		t.Fatal("selection carry intent was read after explicit arrival was consumed, or was not attached to its generation")
	}
}
