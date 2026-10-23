package bibletext

// A STUDY PROMPT'S PASSAGE HOLDS ONLY SCRIPTURE. A drag from the end of one
// verse, through a publisher's heading or a psalm's title, into the next
// carries the heading's words, and the study prompt quoted them inside its
// quotation marks as the cited verses' words, where the share, for the same
// drag, quotes the verses alone. The prompt now quotes what the share quotes
// (aiPassageFor, selectionQuotationIn), so the heading's words stay out of
// both by the same code.

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2/test"
)

// aiHeadingFixture is an invented chapter drawn as the native panes draw it,
// with a heading between verses 1 and 2 and a stack of two between verses 3
// and 4.
func aiHeadingFixture() headingFixtureChapter {
	return headingFixture("Genesis", 10, aiHeadingTexts(), aiHeadingHeads()...)
}

func aiHeadingTexts() map[int]string {
	return map[int]string{
		1: "These are the generations of the sons, born to them after the flood.",
		2: "The Lord said to the sons of the north, “Go.”",
		3: "The sons of Gomer were three, and they spread over the coasts.",
		4: "From these the coastland peoples went out into their lands.",
	}
}

func aiHeadingHeads() []Heading {
	return []Heading{
		{Text: "The Table of Nations", BeforeVerse: 2},
		{Text: "The Northern Peoples", BeforeVerse: 4},
		{Text: "The Coastlands", BeforeVerse: 4},
	}
}

// aiHeadingSmallCapsFixture is the same chapter with verse 2's "Lord" set
// in small capitals, as an edition that marks the divine name records it,
// so the page draws "Lᴏʀᴅ" and a machine reads "LORD".
func aiHeadingSmallCapsFixture() headingFixtureChapter {
	const book, ch = "Genesis", 10
	st := leadChapter(book, ch, aiHeadingTexts())
	st.Bible.Verses[book][ch][1].SmallCaps = []TextSpan{{Start: 4, End: 8}}
	heads := aiHeadingHeads()
	st.Bible.Headings = map[string]map[int][]Heading{book: {ch: heads}}
	corpus, spans := chapterProseIn(st, book, ch)
	page, pvs := drawnHeadingPage(corpus, spans, heads)
	return headingFixtureChapter{st, book, ch, corpus, spans, page, pvs}
}

// aiHeadingWords are the fixture's headings' own words, which no prompt may
// quote.
var aiHeadingWords = []string{"Table", "Nations", "Northern", "Peoples", "Coastlands"}

// sentPrompt is one study request as it reached the provider seam.
type sentPrompt struct {
	ctx    context.Context
	prompt string
}

// capturedPrompts makes every study request hand the prompt it sends to the
// channel and answer at once, below the key and the permission, where a
// provider receives it.
func capturedPrompts(t *testing.T) chan sentPrompt {
	t.Helper()
	prevGenerate, prevFit := aiStudyGenerate, studyRemeasure
	studyRemeasure = func(fit func()) { fit() }
	sent := make(chan sentPrompt, 8)
	aiStudyGenerate = func(ctx context.Context, _ providerInfo, _ *keyStore, _, prompt string) (string, error) {
		sent <- sentPrompt{ctx, prompt}
		return "Synthetic answer.", nil
	}
	t.Cleanup(func() { aiStudyGenerate, studyRemeasure = prevGenerate, prevFit })
	return sent
}

// promptPassage is the citation and the quoted passage a study prompt ends
// on (aiPassageLines).
func promptPassage(t *testing.T, prompt string) (ref, text string) {
	t.Helper()
	const open, between = "Passage (", "):\n“"
	i := strings.LastIndex(prompt, open)
	if i < 0 || !strings.HasSuffix(prompt, "”") {
		t.Fatalf("the prompt does not end on a passage: %q", prompt)
	}
	j := strings.Index(prompt[i:], between)
	if j < 0 {
		t.Fatalf("the prompt's passage has no citation: %q", prompt)
	}
	j += i
	return prompt[i+len(open) : j], strings.TrimSuffix(prompt[j+len(between):], "”")
}

// studyPromptFor dispatches Explain for a selection as a pane hands it
// over, with a key set and the provider allowed, and returns the prompt the
// provider was sent, once the panel has taken the answer, and the words the
// panel shows for the passage. The answer cache is emptied first, so two
// selections that quote the same passage both reach the provider seam.
func studyPromptFor(t *testing.T, st *AppState, sent chan sentPrompt, sel string, span selSpan) (prompt, preview string) {
	t.Helper()
	aiCacheMu.Lock()
	aiCache = map[string]string{}
	aiCacheOrder = nil
	aiCacheMu.Unlock()
	dispatchAIAction(st, aiActionExplain, sel, span)
	select {
	case got := <-sent:
		<-got.ctx.Done()
		panel := studyOverlay(t, st.window)
		excerpt, _ := noteSheetExcerpt(t, panel)
		st.window.Canvas().Overlays().Remove(panel)
		return got.prompt, excerpt.text
	case <-time.After(3 * time.Second):
		t.Fatalf("%q: Explain sent no request", sel)
		return "", ""
	}
}

// A drag across a heading, or a stack of two, sends the provider the verses'
// words alone, and the prompt's passage is the share's quotation of the same
// drag, in the form a machine reads: from the last word above into the first
// words below, the verses above and below whole, begun inside the heading's
// last word, from the closing punctuation of the verse above, and from the
// last word above into the heading, which sends that word alone. Each was
// sent with the headings' words and the verse numbers in it. On a chapter
// that sets the divine name in small capitals the page draws "Lᴏʀᴅ" and the
// prompt reads "LORD", the heading out as before, and the panel shows the
// name as the page draws it; the selection was written as capitals before
// it was read, the chapter, which holds the small capitals, did not find
// it, the heading's words stayed in the prompt, and the panel showed the
// capitals. The controls: the share's message of each drag holds none of
// the headings' words either, and cites what the prompt cites.
func TestAStudyPromptLeavesOutTheHeadingADragRunsAcross(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("reading")
	defer win.Close()
	sent := capturedPrompts(t)
	shared := captureShares(t)

	for _, smallCaps := range []bool{false, true} {
		f := aiHeadingFixture()
		lord := "Lord"
		if smallCaps {
			f = aiHeadingSmallCapsFixture()
			lord = "LORD"
		}
		f.st.window = win
		f.st.aiKeys = newKeyStoreWith(newFakePrefs())
		f.st.aiKeys.setAPIKey(defaultProviderID, "test-key")
		grantAIPermission(f.st.aiKeys, defaultProviderID)
		v1end, heading := f.pvs[0].end, strings.Index(f.page, "The Table of Nations")
		for _, c := range []struct {
			name, from, to string
			after          int
			ref, want      string
		}{
			{"the last word above into the first words below", "flood.", "said", 0,
				"Genesis 10:1–2", "flood. The " + lord + " said"},
			{"the verses above and below whole", "¹", "“Go.”", 0,
				"Genesis 10:1–2", "These are the generations of the sons, born to them after the flood. The " + lord + " said to the sons of the north, “Go.”"},
			{"begun inside the heading's last word", "ations", "said", heading,
				"Genesis 10:2", "The " + lord + " said"},
			{"from the closing punctuation above", ".", "said", v1end - 1,
				"Genesis 10:2", "The " + lord + " said"},
			{"the last word above into the heading", "flood.", "Nations", 0,
				"Genesis 10:1", "flood."},
			{"across a stack of two headings", "coasts.", "went out", 0,
				"Genesis 10:3–4", "coasts. From these the coastland peoples went out"},
			{"from the stack into the verse below", "Peoples", "From these", 0,
				"Genesis 10:4", "From these"},
		} {
			name := fmt.Sprintf("%s (small capitals %v)", c.name, smallCaps)
			raw, span, _ := f.drag(t, c.after, c.from, c.to)
			prompt, preview := studyPromptFor(t, f.st, sent, raw, span)
			ref, passage := promptPassage(t, prompt)
			if ref != c.ref || passage != c.want {
				t.Errorf("%s, %q: the prompt quotes (%s) %q, want (%s) %q", name, raw, ref, passage, c.ref, c.want)
			}
			if smallCaps && strings.Contains(passage, "LORD") && !strings.Contains(preview, "Lᴏʀᴅ") {
				t.Errorf("%s: the panel shows %q, want the divine name as the page draws it", name, preview)
			}
			for _, w := range aiHeadingWords {
				if strings.Contains(passage, w) {
					t.Errorf("%s: the prompt quotes the heading's %q: %q", name, w, passage)
				}
			}
			*shared = nil
			dispatchSelectionAction(f.st, selActionShareCite, raw, span)
			if len(*shared) != 1 || !strings.Contains((*shared)[0], "— "+c.ref+" (") {
				t.Fatalf("control: %s: Share with citation sent %q, want it cited %s", name, *shared, c.ref)
			}
			for _, w := range aiHeadingWords {
				if strings.Contains((*shared)[0], w) {
					t.Errorf("control: %s: the share quotes the heading's %q: %q", name, w, (*shared)[0])
				}
			}
		}
	}
}

// A drag from a psalm's title on into the psalm, handed over with no span as
// a page that cannot place it hands it, sends the psalm's words alone, with
// a heading under the title and without one; the title's words went to the
// provider as verse 1's. The native panes cut such a drag at verse 1's
// number before Go sees it.
func TestAStudyPromptLeavesOutAPsalmTitleADragRunsFrom(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("reading")
	defer win.Close()
	sent := capturedPrompts(t)
	const words = "Yahweh, how my adversaries"
	for _, withHeading := range []bool{false, true} {
		st := psalmTitleState(withHeading)
		st.window = win
		st.aiKeys = newKeyStoreWith(newFakePrefs())
		st.aiKeys.setAPIKey(defaultProviderID, "test-key")
		grantAIPermission(st.aiKeys, defaultProviderID)
		title := st.Bible.SuperscriptionFor("Psalms", 3).DrawnText()
		under := ""
		if withHeading {
			under = "\n" + psalmHeading
		}
		for _, sel := range []string{title + under + "\n1" + nb + words, "his son." + under + "\n1" + nb + words} {
			prompt, _ := studyPromptFor(t, st, sent, sel, selSpan{})
			ref, passage := promptPassage(t, prompt)
			if ref != "Psalms 3:1" || passage != words {
				t.Errorf("heading %v, %q: the prompt quotes (%s) %q, want (Psalms 3:1) %q", withHeading, sel, ref, passage, words)
			}
		}
	}
}

// Over the local WEB, BSB and WEB Catholic caches, opt-in by
// BIBLETEXT_EXHAUSTIVE=1 as the other sweeps over them are: beside every
// heading, or stack of headings, between two verses, drags across it as the
// native panes hand them over (paneHeadingSpan) — from the last word above
// to the first word below and to the first two, the verse above whole to
// the verse below whole, from the closing punctuation above to the first
// word below, from the last word above and from the closing punctuation
// above into the headings, begun inside the headings' last word, and from
// the headings' first word to the first two words below — and every psalm's
// title run on into its first verse's first words, with no span. Each that
// a menu offers the study verbs sends a passage that is Scripture alone: the
// share's quotation of the drag, in the machine's form, which stands in the
// chapter's verses as one stretch of them, with no heading's or title's
// word and no verse number inside it, and which keeps the verses' words the
// drag holds, and holds a letter or figure: a drag from the closing
// punctuation above into the headings, whose quotation is that punctuation
// alone, is offered no study verb (no_scripture_study_test.go), and none of
// the drags studied sends punctuation alone. The drags that take the
// headings whole are counted where the passage holds the headings' words,
// and the study panel's preview of the same drags is measured beside it and
// logged, not held. With the passage sent as the selection came, every
// studied drag quoted otherwise (WEB 284, BSB 15,259, WEB Catholic 300), and
// 263, 13,374 and 277 of them held the headings' or the title's words.
func TestAStudyPromptQuotesNoHeadingOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: drags across every heading of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		var headings, titles, drags, studied, punctuation, wrong, carried, previewCarried int
		miss := func(format string, a ...any) {
			wrong++
			if wrong <= 8 {
				t.Logf(c.id+": "+format, a...)
			}
		}
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: c.bd, CurrentVersion: c.id, CurrentBook: book, CurrentChapter: ch}
				corpus, spans := chapterProseIn(st, book, ch)
				if len(spans) == 0 {
					continue
				}
				scripture := outboundText(corpus)
				// check reads one selection as the dispatch hands it on: kept
				// is the verses' words it holds, matter the editorial words it
				// takes whole, if it takes them whole.
				check := func(name, sel string, span selSpan, kept, matter string) {
					drags++
					if !aiStudyOffered(st, sel, span) {
						return
					}
					studied++
					text := strings.TrimSpace(sharedText(sel))
					p := aiPassageFor(st, text, span)
					flat := collapseSpaces(p.Text)
					quotation, _, _ := selectionQuotationIn(st, p.Book, p.Chapter, text, span)
					// The matter's words counted beyond the times the verses
					// the drag holds have them: a heading may repeat the
					// words of the verse below ("Shamgar" above Judges 3:31).
					inVerses := strings.Count(outboundText(kept), matter)
					if matter != "" && strings.Count(flat, matter) > inVerses {
						carried++
					}
					if matter != "" && strings.Count(collapseSpaces(outboundText(p.PreviewText)), matter) > inVerses {
						previewCarried++
					}
					if strings.IndexFunc(flat, isWordRune) < 0 {
						punctuation++
					}
					if flat == "" || !strings.Contains(scripture, flat) || !strings.Contains(flat, outboundText(kept)) ||
						p.Text != outboundText(quotation) {
						miss("%s %d, %s, %q %+v: the prompt quotes (%s) %q", book, ch, name, sel, span, p.Ref, p.Text)
					}
				}
				if title, first, ok := chapterTitle(st, book, ch); ok {
					for _, sp := range spans {
						if sp.verse != first {
							continue
						}
						kept := corpus[sp.start:sp.end]
						if fs := strings.Fields(kept); len(fs) > 2 {
							kept = strings.Join(fs[:2], " ")
						}
						titles++
						check("the title into the psalm", title+"\n"+strconv.Itoa(first)+" "+kept, selSpan{},
							strings.TrimFunc(kept, notWordRune), outboundText(title))
					}
				}
				hs := headingsFor(c.bd, book, ch)
				page, pvs := drawnHeadingPage(corpus, spans, hs)
				// stretch is the verses' words the drag page[a:b] holds,
				// less the punctuation at either end, which a quotation
				// may drop with a verse it is all of.
				stretch := func(a, b int) string {
					s, e := headingPageStretch(pvs, a, b)
					if s < 0 || e <= s {
						return ""
					}
					return strings.TrimFunc(corpus[s:e], notWordRune)
				}
				for k := 0; k+1 < len(pvs); k++ {
					hs0, hs1, ok := headingMatter(page, pvs, k)
					if !ok {
						continue
					}
					above, below := pvs[k], pvs[k+1]
					_, w1, _, ok := firstWordAt(page, below.body, below.end)
					last := strings.Fields(page[above.body:above.end])
					heads := strings.Fields(page[hs0:hs1])
					if !ok || len(last) == 0 || len(heads) == 0 {
						continue
					}
					headings++
					matter := outboundText(strings.Join(heads, " "))
					lastAt := above.end - len(last[len(last)-1])
					two := wordsEnd(page, below.body, below.end, 2)
					headLast := hs1 - len(heads[len(heads)-1])
					ds := []crossingDrag{
						{"the last word above to the first word below", lastAt, w1, true},
						{"the last word above to the first two words below", lastAt, two, true},
						{"the verse above whole to the verse below whole", above.num, below.end, true},
						{"the last word above into the headings", lastAt, hs1, true},
						{"the headings to the first two words below", hs0, two, true},
					}
					if r, size := utf8.DecodeLastRuneInString(page[above.body:above.end]); !isWordRune(r) {
						ds = append(ds,
							crossingDrag{"the closing punctuation above to the first word below", above.end - size, w1, true},
							crossingDrag{"the closing punctuation above into the headings", above.end - size, hs1, true})
					}
					if r, size := utf8.DecodeRuneInString(page[headLast:hs1]); isWordRune(r) && hs1-headLast > size {
						ds = append(ds, crossingDrag{"begun inside the headings' last word, to the first two words below", headLast + size, two, false})
					}
					for _, d := range ds {
						m := ""
						if d.whole {
							m = matter
						}
						check(d.name, page[d.a:d.b], paneHeadingSpan(page, pvs, d.a, d.b), stretch(d.a, d.b), m)
					}
				}
			}
		}
		t.Logf("%s: %d headings between two verses and %d psalm titles; %d drags, %d of them offered the study verbs, %d of those sending punctuation alone; prompts quoting otherwise than the rule: %d; prompts holding the headings' or the title's words: %d; study panel previews holding them (measured, not held): %d",
			c.id, headings, titles, drags, studied, punctuation, wrong, carried, previewCarried)
		if wrong != 0 || carried != 0 || punctuation != 0 {
			t.Errorf("%s: %d prompts quoted otherwise than the share's quotation of Scripture alone, %d with the headings' words, %d punctuation alone", c.id, wrong, carried, punctuation)
		}
	}
}

// crossingDrag is one drag of the sweep: page[a:b] on a drawnHeadingPage,
// and whether it takes the headings it crosses whole.
type crossingDrag struct {
	name  string
	a, b  int
	whole bool
}

// notWordRune is a rune that is neither a letter, a figure nor a combining
// mark, for trimming a stretch of verse words to the words at its ends.
func notWordRune(r rune) bool { return !isWordRune(r) && !unicode.IsMark(r) }
