package bibletext

// The shared-link flow, enumerated. See docs/NKJV_FLOW.md.
//
// WHY THIS IS AN ENUMERATION AND NOT A LIST OF CASES. Every blocked state this
// project has shipped was invisible until the whole space was laid out: a card
// over the loading spinner whose two in-app answers both dead-ended, a seed
// install that claimed a link and did nothing, a download that consumed a park
// belonging to another translation. Each was reachable by a combination nobody
// thought to write a test for. A test that checks the cases we thought of is
// exactly what failed us, so this one walks the cross-product instead.
//
// HOW IT FAILS USEFULLY. Combinations that are blocked TODAY are pinned in
// knownBlocked below. The test asserts the set of blocked combinations matches
// that list EXACTLY — so it fails both when a new dead end appears AND when one
// is fixed without being struck off. The second half is the point: a fix that
// leaves this list stale makes the next reader trust a document that lies.

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// linkFlowState is the slice of AppState this flow actually branches on. Kept as
// its own type so the enumeration below is readable, and so adding a variable to
// the flow means adding it HERE, where the cross-product will pick it up.
type linkFlowState struct {
	phase      loadPhase
	seedOnly   bool
	bookInHand bool // the target book exists in the canon currently loaded
	sameVer    bool // the link names the translation already open
	verLoading bool // a translation fetch already owns the spinner
}

func (s linkFlowState) id() string {
	return fmt.Sprintf("phase=%d seed=%v book=%v same=%v loading=%v",
		s.phase, s.seedOnly, s.bookInHand, s.sameVer, s.verLoading)
}

// knownBlocked is the pinned list from docs/NKJV_FLOW.md: combinations where a
// reader today gets no passage AND no message. Each entry names the state in
// that document so the two cannot drift apart silently.
//
// STRIKING ONE OFF IS PART OF FIXING IT. If a fix makes a combination live, this
// test fails until the entry is removed — which is how the document stays true.
// EMPTY, and that is the point. It held four entries — every combination where
// the book was not in the loaded canon — until Batch 1 (2026-08-14) gave each of
// them something to say: a park with a reason where a download can still supply
// the book, and a plain "it isn't in this translation" where none can.
//
// Leave it empty rather than deleting the machinery. The enumeration asserts set
// EQUALITY, so an empty map is the strongest possible assertion — any new dead
// end fails immediately, named, with the combination that reaches it.
var knownBlocked = map[string]string{}

// TestLinkFlowHasNoUnknownDeadEnds walks the cross-product and asserts I1:
// every state either opens the passage or says something. Anything that does
// neither must be in knownBlocked, by name.
func TestLinkFlowHasNoUnknownDeadEnds(t *testing.T) {
	var blocked []string
	seen := 0

	for _, phase := range []loadPhase{loadReady, loadPending, loadFailed} {
		for _, seedOnly := range []bool{false, true} {
			for _, bookInHand := range []bool{false, true} {
				for _, sameVer := range []bool{false, true} {
					for _, verLoading := range []bool{false, true} {
						s := linkFlowState{phase, seedOnly, bookInHand, sameVer, verLoading}
						seen++

						opened, said, parked := runLinkFlow(t, s)
						if opened || said || parked {
							continue // a way forward exists
						}
						blocked = append(blocked, s.id())
					}
				}
			}
		}
	}

	sort.Strings(blocked)
	var want []string
	for k := range knownBlocked {
		want = append(want, k)
	}
	sort.Strings(want)

	if strings.Join(blocked, "\n") != strings.Join(want, "\n") {
		t.Errorf("the set of dead-end states changed.\n\nBLOCKED NOW (%d):\n  %s\n\nPINNED IN docs/NKJV_FLOW.md (%d):\n  %s\n\n"+
			"If you FIXED one, strike it from knownBlocked and from the document.\n"+
			"If you INTRODUCED one, it is a dead end: the reader gets no passage and no message.",
			len(blocked), strings.Join(blocked, "\n  "), len(want), strings.Join(want, "\n  "))
	}
	t.Logf("enumerated %d states, %d blocked (all pinned)", seen, len(blocked))
}

// runLinkFlow drives the REAL entry point for one combination and reports what
// the reader got: the passage, a message, or a park that will produce one later.
// Anything else is a dead end.
//
// HandleShareLink, not applyShareTarget. The first cut called applyShareTarget
// and produced nine "dead ends" that were harness artefacts: HandleShareLink is
// where a link arriving before the data is PARKED, so bypassing it invented
// failures at loadPending and loadFailed that the app does not have. Driving
// from the URL also exercises the parser, which is part of the flow.
//
// The link's translation is preloaded into loadedVersions so a switch resolves
// synchronously — switchVersionInteractive owns a spinner and a window, neither
// of which exists in a host test, and the question here is which STATES are
// reachable, not how a download is presented.
func runLinkFlow(t *testing.T, s linkFlowState) (opened, said, parked bool) {
	t.Helper()

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible:          bd,
		CurrentBook:    "Genesis",
		CurrentChapter: 1,
		CurrentVersion: "web",
		loadPhase:      s.phase,
		seedOnly:       s.seedOnly,
		fullPending:    s.seedOnly,
		versionLoading: s.verLoading,
		loadedVersions: map[string]*BibleData{"web": bd},
	}

	book := "John" // in the sample canon
	if !s.bookInHand {
		book = "Habakkuk" // a real book the sample data does not carry
		if bd.GetChaptersForBook(book) != 0 {
			t.Fatalf("precondition: %s must be absent from the sample canon", book)
		}
	}
	version := "web"
	if !s.sameVer {
		version = "bsb"
		other := NewBibleData()
		other.PopulateWithSampleVerses()
		other.PrepareSearchIndex()
		st.loadedVersions["bsb"] = other
	}

	url := ShareLinkURL(version, book, 3, 16, 0)
	if url == "" {
		t.Fatalf("could not build a link for %s %s", version, book)
	}
	target, ok := ParseShareLink(url)
	if !ok {
		t.Fatalf("the flow's own link did not parse: %q", url)
	}
	HandleShareLink(st, url)

	opened = st.CurrentBook == book
	parked = st.pendingLink != nil
	// "Said something" means ANY of the three things this flow can tell a reader.
	// Asking only linkVersionUnavailable — as the first cut did — reported states
	// as dead ends that had already been fixed, because the other two messages
	// were invisible to it. Every message the flow can produce has a pure
	// function precisely so this question is answerable.
	said = linkVersionUnavailable(st, target) != "" ||
		linkBookUnavailableMessage(st, target) != "" ||
		linkParkedMessage(st, false) != ""
	return opened, said, parked
}

// The pinned list must stay honest about itself: every entry has to name a state
// the document actually describes, or the two have drifted.
func TestKnownBlockedNamesRealStates(t *testing.T) {
	for combo, name := range knownBlocked {
		if !strings.HasPrefix(name, "B_") {
			t.Errorf("%s: %q does not name a state in docs/NKJV_FLOW.md", combo, name)
		}
	}
}

// I2: if the app claims a link it must do something visible, and if it cannot
// handle the link it must DECLINE so the OS opens it in the browser instead.
//
// The URLs below are not hypothetical. iOS routes them to the app because they
// match the "/web/*" component of the association file, and every one of them
// parses to nothing: a book index, a singular slug, a non-numeric chapter. The
// native side used to return YES for all of them, so iOS considered the link
// handled, never offered Safari, and the app simply foregrounded on whatever
// chapter it was already showing — which reads as the tap having gone wrong.
func TestDeclinedLinksFallBackToTheBrowser(t *testing.T) {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
	}
	prev := activeAIState
	registerAIState(st)
	defer registerAIState(prev)

	for _, url := range []string{
		"https://bibletext.co.uk/web/john/",       // a book index, matched by /web/*
		"https://bibletext.co.uk/web/",            // a version index
		"https://bibletext.co.uk/web/psalm/23/",   // singular slug; the contract is "psalms"
		"https://bibletext.co.uk/web/john/three/", // chapter is not a number
		"https://bibletext.co.uk/privacy.html",    // must always stay in the browser
		"https://example.com/web/john/3/",         // not our host at all
	} {
		if deliverShareLink(url) {
			t.Errorf("claimed a link it cannot open, so the OS will not offer the browser: %s", url)
		}
	}

	// ...and a real one is still claimed, or the fix would have broken every link.
	if !deliverShareLink("https://bibletext.co.uk/web/john/3/#v16") {
		t.Error("declined a link that IS ours")
	}
}
