package bibletext

// The verb → screen contract, held for EVERY notes verb at once.
//
// THE CLASS THIS PINS: a verb mutates the store or state but does not end on
// the shared projection + repaint, so the visible pane disagrees with the
// store until navigation re-derives (the dropCurrentNote field report: "all
// the note pills disappear... until I navigate away and come back"). Each verb
// below runs against one 3-note fixture chapter and is then answered at
// SCREEN level on BOTH seams:
//
//   (a) the Fyne banner path — the real banner is built and laid out in a test
//       window and the assertion walks what is SEEN (seenText,
//       screen_seen_test.go — the flattened screen, not the object tree);
//   (b) the Apple push seam — appleStickerPush's tuple must AGREE WITH THE
//       STORE: the pushed words are a stored note's words, the count lines
//       carry the store's honest count, pill vs bubble follows the mirror the
//       projection wrote, and the push is empty ONLY when the store holds
//       nothing for the chapter (or the feature is off).
//
// The store-agreement half (assertStickerAgreesWithStore) recomputes the
// chapter's notes from the raw store, NOT from the plan — the plan is the very
// projection a broken verb fails to run, so agreeing with it would be
// agreeing with the defect.
//
// The acceptance gate for this file is scripts/view-test-gate.sh M8: it
// removes dropCurrentNote's ending projection call and this suite must go red
// for it — the mechanical proof that the contract here can feel a verb that
// writes the store and stops before the screen.

import (
	"fmt"
	"fyne.io/fyne/v2/theme"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// verbScreenFixture seeds three received notes on John 3 — distinct verses,
// distinct texts, deterministic clock — and lands the reader on the chapter,
// which projects the newest note open. Returns the state and the notes OLDEST
// FIRST as stored ("alpha…" v16, "beta…" v17, "gamma…" v18); the plan's
// stable order is the reverse.
func verbScreenFixture(t *testing.T) (*AppState, []StoredNote) {
	t.Helper()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() {
		setNotesEnabled(true) // rows that turn the feature off must not leak it
		deleteAllNotes(appPrefs())
	})

	origNow := noteNow
	now := int64(1_700_000_000)
	noteNow = func() int64 { now++; return now }
	t.Cleanup(func() { noteNow = origNow })

	notes := make([]StoredNote, 0, 3)
	for i, text := range []string{
		"alpha words on sixteen",
		"beta words on seventeen",
		"gamma words on eighteen",
	} {
		n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: 16 + i, Text: text})
		if !ok {
			t.Fatalf("seeding note %q failed", text)
		}
		notes = append(notes, n)
	}
	st := planTestState(t)
	addRecentChapter(st, "John", 3)
	if st.NoteID != notes[2].ID {
		t.Fatal("precondition: the newest note should lead the passage")
	}
	return st, notes
}

// storeNotesOnChapter is the STORE's own answer for the current chapter,
// computed without the plan (see the file header for why). The fixture files
// every note natively under the reading translation, so version/book/chapter
// equality is the whole placement question here.
func storeNotesOnChapter(st *AppState) []StoredNote {
	var out []StoredNote
	for _, n := range readNoteStore(appPrefs()).notes {
		if n.Kind == noteKindReceived && displayableNote(n) &&
			strings.EqualFold(n.VersionID, st.currentVersion().ID) &&
			n.Book == st.CurrentBook && n.Chapter == st.CurrentChapter {
			out = append(out, n)
		}
	}
	return out
}

// assertStickerAgreesWithStore is seam (b)'s generic half: whatever the verb
// did, the pushed tuple must tell the store's truth.
func assertStickerAgreesWithStore(t *testing.T, st *AppState) {
	t.Helper()
	plan := buildChapterPlan(st, appPrefs(), st.Bible)
	text, who, pill, next := appleStickerPush(st, plan)
	// The Android full-screen sticker rides the SAME composition (task #19):
	// androidStickerPush is a thin alias of the Apple push, held here to
	// byte-equality across every verb so the two native channels cannot
	// drift — this is the Android-channel pin of the store-agreement seam.
	aText, aWho, aPill, aNext := androidStickerPush(st, plan)
	if aText != text || aWho != who || aPill != pill || aNext != next {
		t.Errorf("androidStickerPush diverged from appleStickerPush: android=(%q,%q,%v,%v) apple=(%q,%q,%v,%v)",
			aText, aWho, aPill, aNext, text, who, pill, next)
	}
	// …and the FOURTH surface: the styled Windows/Linux pane's own in-text
	// sticker (reading_styled_note.go) rides the same composition through the
	// same kind of alias, held here to byte-equality for the same reason — four
	// stickers, one who-line grammar, one set of counts, one pill vocabulary.
	sText, sWho, sPill, sNext := styledStickerPush(st, plan)
	if sText != text || sWho != who || sPill != pill || sNext != next {
		t.Errorf("styledStickerPush diverged from appleStickerPush: styled=(%q,%q,%v,%v) apple=(%q,%q,%v,%v)",
			sText, sWho, sPill, sNext, text, who, pill, next)
	}
	notes := storeNotesOnChapter(st)

	if !notesEnabled() || len(notes) == 0 {
		// Empty only when the store is empty for the chapter (or the feature
		// is off): anything pushed here is a sticker over nothing.
		if text != "" || who != "" || pill || next {
			t.Fatalf("store holds nothing for this chapter (enabled=%v) yet the sticker pushes text=%q who=%q pill=%v next=%v",
				notesEnabled(), text, who, pill, next)
		}
		return
	}

	// The store holds notes here: an empty push is the exact stale-screen
	// defect this file exists for.
	if text == "" {
		t.Fatalf("store holds %d notes on this chapter and the sticker pushes no text at all (who=%q)", len(notes), who)
	}
	found := false
	for _, n := range notes {
		if n.Text == text {
			found = true
		}
	}
	if !found {
		t.Errorf("the pushed words %q are no stored note's words — the sticker disagrees with the store", text)
	}

	n := len(notes)
	if pill {
		want := "Note"
		if n > 1 {
			want = fmt.Sprintf("Notes · %d", n)
		}
		if !strings.Contains(who, want) {
			t.Errorf("pill who = %q, want the store's honest count %q", who, want)
		}
		if next {
			t.Error("the pill must never carry the next control")
		}
		return
	}
	if n > 1 {
		if !strings.Contains(who, fmt.Sprintf("of %d on this passage", n)) {
			t.Errorf("expanded who = %q, want the store's honest count \"of %d on this passage\"", who, n)
		}
		if !next {
			t.Errorf("expanded with %d notes on the passage must offer the next control", n)
		}
	} else if next {
		t.Error("one note: nothing to advance to, next must be false")
	}
}

// assertBannerAgreesWithStore is seam (a)'s generic half: every stored note on
// the chapter leaves a visible trace — its words when open, its citation as a
// chip otherwise. A note whose only existence is in the store is exactly the
// stale pane the class produces.
func assertBannerAgreesWithStore(t *testing.T, st *AppState, seen string) {
	t.Helper()
	for _, n := range storeNotesOnChapter(st) {
		ref := strings.ToLower(noteReference(n))
		if !strings.Contains(seen, strings.ToLower(n.Text)) && !strings.Contains(seen, ref) {
			t.Errorf("stored note %q (%s) has no visible trace on the banner — store and screen disagree.\nseen:\n%s",
				n.Text, ref, seen)
		}
	}
}

// seenBannerButton lays root out in a real window and returns the first
// VISIBLE, laid-out button match — so the rows that press a control press one
// a reader could actually see, not one merely present in the tree.
func seenBannerButton(t *testing.T, root fyne.CanvasObject, size fyne.Size, match func(*widget.Button) bool) *widget.Button {
	t.Helper()
	w := test.NewWindow(root)
	t.Cleanup(w.Close)
	w.Resize(size)
	var found *widget.Button
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil || !o.Visible() || found != nil {
			return
		}
		if b, ok := o.(*widget.Button); ok {
			if sz := b.Size(); sz.Width > 0 && sz.Height > 0 && match(b) {
				found = b
			}
			return
		}
		if sc, ok := o.(*container.Scroll); ok {
			walk(sc.Content)
			return
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if wdg, ok := o.(fyne.Widget); ok {
			if r := test.WidgetRenderer(wdg); r != nil {
				for _, ch := range r.Objects() {
					walk(ch)
				}
			}
		}
	}
	walk(root)
	return found
}

// TestEveryNotesVerbEndsOnTheScreen is the table: run the verb, then ask both
// seams for the screen-level truth.
func TestEveryNotesVerbEndsOnTheScreen(t *testing.T) {
	const (
		alphaText = "alpha words on sixteen"
		betaText  = "beta words on seventeen"
		gammaText = "gamma words on eighteen"
	)
	size := fyne.NewSize(700, 700)

	cases := []struct {
		name string
		// act runs the verb (or the real tap that carries it).
		act func(t *testing.T, st *AppState, notes []StoredNote)
		// after runs any row-specific store assertions.
		after func(t *testing.T, st *AppState, notes []StoredNote)

		// seam (a): the banner. bannerGone asserts buildNoteBanner returns
		// nil — the screen truth for "nothing to show".
		bannerGone bool
		wantSeen   []string
		rejectSeen []string

		// seam (b): the sticker tuple.
		stickerGone bool
		wantText    string
		wantWho     []string
		wantPill    bool
		wantNext    bool
	}{
		{
			// A link arrives carrying a NEW note: the whole navigation + store +
			// focus + projection tail of applyShareTarget.
			name: "arrival",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3,
					VerseLo: 19, Note: "delta words just arrived"})
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				if n := storedNoteCount(appPrefs()); n != 4 {
					t.Errorf("the arrival should be stored: %d notes, want 4", n)
				}
			},
			wantSeen:   []string{"delta words just arrived", "john 3:16", "john 3:17", "john 3:18", "from friend"},
			rejectSeen: []string{alphaText, betaText, gammaText},
			wantText:   "delta words just arrived",
			wantWho:    []string{"Note from Friend", "1 of 4 on this passage"},
			wantNext:   true,
		},
		{
			// Hide keeps the note and collapses the page: the bubble becomes a
			// chip marked hidden; the sticker becomes the pill with the count.
			name: "hide",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				hideCurrentNote(st)
			},
			wantSeen:   []string{"john 3:18", "hidden", "john 3:16", "john 3:17"},
			rejectSeen: []string{alphaText, betaText, gammaText},
			wantText:   gammaText,
			wantWho:    []string{"Notes · 3"},
			wantPill:   true,
		},
		{
			// Restore (the pill press) brings the hidden note back whole.
			name: "restore",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				hideCurrentNote(st)
				restoreCurrentNote(st)
			},
			wantSeen:   []string{gammaText, "john 3:16", "john 3:17"},
			rejectSeen: []string{"hidden", alphaText, betaText},
			wantText:   gammaText,
			wantWho:    []string{"1 of 3 on this passage"},
			wantNext:   true,
		},
		{
			// Delete (the native menu's verb): the rest of the set must SURFACE
			// — the class's founding instance, at screen level. The two seams
			// tell it differently, both deliberately: the banner honours N3
			// (deleting is closing, not choosing a neighbour — focus falls to
			// none, so the remaining notes are CHIPS, nothing auto-expands in
			// the closed note's place), while the Apple mirror surfaces the
			// next note in the sticker (TestDeleteOfManySurfacesTheRemaining).
			// Either way, NOTHING of the store may be invisible.
			name: "delete",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				dropCurrentNote(st)
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				if n := storedNoteCount(appPrefs()); n != 2 {
					t.Errorf("one delete: %d notes remain, want 2", n)
				}
			},
			// AMENDED 20 Aug 2026: betaText moved from rejectSeen to wantSeen.
			// The survivor is the OPEN note after a delete — the sticker has
			// always said so ("1 of 2 on this passage", not a pill) — and the
			// banner draws the open note's text, exactly as it draws "delta
			// words just arrived" on the arrival row above. The old expectation
			// pinned a DISAGREEMENT between the two surfaces: the banner showed
			// only chips because the plan's Open was false, while the sticker
			// showed the note expanded because the mirror ignored Open and read
			// the stored Minimized bit alone. That is the divergence this very
			// harness exists to catch, and it was written into its expectations.
			wantSeen:   []string{betaText, "john 3:17", "john 3:16"},
			rejectSeen: []string{gammaText, "john 3:18", alphaText},
			wantText:   betaText,
			wantWho:    []string{"1 of 2 on this passage"},
			wantNext:   true,
		},
		{
			// The SAME delete reached the way a Fyne-surface reader reaches it:
			// the banner's own Delete button, found laid-out and visible, and
			// really tapped — the browse/banner surface path, not the verb
			// called by name.
			name: "delete via the banner's own button",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				del := seenBannerButton(t, buildNoteBanner(st), size, func(b *widget.Button) bool {
					// Matched by the RESOURCE NAME: the banner's hide control is
					// icon-only too, and "any icon-only button" tapped hide
					// instead of delete.
					return b.Text == "" && b.Icon != nil && b.Icon.Name() == theme.DeleteIcon().Name()
				})
				if del == nil {
					t.Fatal("no visible Delete button on the banner's open note")
				}
				test.Tap(del)
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				if n := storedNoteCount(appPrefs()); n != 2 {
					t.Errorf("the tapped delete: %d notes remain, want 2", n)
				}
			},
			// AMENDED 20 Aug 2026: betaText moved from rejectSeen to wantSeen.
			// The survivor is the OPEN note after a delete — the sticker has
			// always said so ("1 of 2 on this passage", not a pill) — and the
			// banner draws the open note's text, exactly as it draws "delta
			// words just arrived" on the arrival row above. The old expectation
			// pinned a DISAGREEMENT between the two surfaces: the banner showed
			// only chips because the plan's Open was false, while the sticker
			// showed the note expanded because the mirror ignored Open and read
			// the stored Minimized bit alone. That is the divergence this very
			// harness exists to catch, and it was written into its expectations.
			wantSeen:   []string{betaText, "john 3:17", "john 3:16"},
			rejectSeen: []string{gammaText, "john 3:18", alphaText},
			wantText:   betaText,
			wantWho:    []string{"1 of 2 on this passage"},
			wantNext:   true,
		},
		{
			// The sticker's next-tap: focus advances, the screen follows.
			name: "advance focus",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				advanceNoteFocus(st)
			},
			wantSeen:   []string{betaText, "john 3:18", "john 3:16"},
			rejectSeen: []string{gammaText, alphaText},
			wantText:   betaText,
			wantWho:    []string{"2 of 3 on this passage"},
			wantNext:   true,
		},
		{
			// The chip IS the Show verb for a stored-minimized note: really tap
			// the "hidden" chip, and the un-minimize-by-ID must reach the store
			// AND the screen at once.
			name: "un-minimize by id via the chip tap",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				setNoteMinimizedByID(appPrefs(), notes[1].ID, true)
				applyNoteForCurrentChapter(st) // the state a navigation would have left
				chip := seenBannerButton(t, buildNoteBanner(st), size, func(b *widget.Button) bool {
					return strings.Contains(b.Text, "hidden")
				})
				if chip == nil {
					t.Fatal("no visible chip carries the hidden marker")
				}
				test.Tap(chip)
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				for _, n := range allNotesForBrowsing(appPrefs()) {
					if n.ID == notes[1].ID && n.Minimized {
						t.Error("the chip tap is the Show verb: the store must hold Minimized=false")
					}
				}
			},
			wantSeen:   []string{betaText, "john 3:18", "john 3:16"},
			rejectSeen: []string{"hidden", gammaText, alphaText},
			wantText:   betaText,
			wantWho:    []string{"2 of 3 on this passage"},
			wantNext:   true,
		},
		{
			// Off means off, NOW: no banner, no sticker — and nothing deleted.
			name: "turn notes off",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				turnNotesOff(st)
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				setNotesEnabled(true) // read past the gate: the store must be intact
				if n := storedNoteCount(appPrefs()); n != 3 {
					t.Errorf("off must not delete: %d notes, want 3", n)
				}
				setNotesEnabled(false) // the row's screen truth is asserted with it off
			},
			bannerGone:  true,
			stickerGone: true,
		},
		{
			// Delete all (the Settings prompt's Delete button, verbatim): store
			// emptied, mirror cleared, both screens bare at once.
			name: "delete all",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				deleteAllNotes(appPrefs())
				clearLiveNote(st)
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				if n := storedNoteCount(appPrefs()); n != 0 {
					t.Errorf("delete all left %d notes", n)
				}
			},
			bannerGone:  true,
			stickerGone: true,
		},
		{
			// Re-opening the link of a note the reader HID: the dedup finds the
			// stored record, the tap is the Show verb (owner, 2026-08-18), and
			// no duplicate is minted.
			name: "link re-arrival",
			act: func(t *testing.T, st *AppState, notes []StoredNote) {
				hideCurrentNote(st)
				applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3,
					VerseLo: 18, Note: gammaText})
			},
			after: func(t *testing.T, st *AppState, notes []StoredNote) {
				if n := storedNoteCount(appPrefs()); n != 3 {
					t.Errorf("the re-arrival must dedup: %d notes, want 3", n)
				}
			},
			wantSeen:   []string{gammaText, "john 3:16", "john 3:17"},
			rejectSeen: []string{"hidden", alphaText, betaText},
			wantText:   gammaText,
			wantWho:    []string{"1 of 3 on this passage"},
			wantNext:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			pinBannerPlatform(t) // ask the banner question the shipped Fyne platforms ask

			st, notes := verbScreenFixture(t)
			tc.act(t, st, notes)

			// Seam (a): the Fyne banner, laid out, walked as SEEN.
			banner := buildNoteBanner(st)
			if tc.bannerGone {
				if banner != nil {
					t.Errorf("%s: the banner should be gone, and something is still on screen", tc.name)
				}
			} else {
				if banner == nil {
					t.Fatalf("%s: no banner at all — the store's notes are invisible", tc.name)
				}
				seen := seenText(t, banner, size)
				for _, want := range tc.wantSeen {
					if !strings.Contains(seen, want) {
						t.Errorf("%s: the reader cannot see %q on the banner.\nseen:\n%s", tc.name, want, seen)
					}
				}
				for _, reject := range tc.rejectSeen {
					if strings.Contains(seen, reject) {
						t.Errorf("%s: %q is on screen and must not be.\nseen:\n%s", tc.name, reject, seen)
					}
				}
				assertBannerAgreesWithStore(t, st, seen)
			}

			// Seam (b): the Apple push tuple — row expectations, then the
			// generic store agreement.
			text, who, pill, next := appleStickerPush(st, buildChapterPlan(st, appPrefs(), st.Bible))
			if tc.stickerGone {
				if text != "" || who != "" || pill || next {
					t.Errorf("%s: the sticker should push nothing, got text=%q who=%q pill=%v next=%v",
						tc.name, text, who, pill, next)
				}
			} else {
				if text != tc.wantText {
					t.Errorf("%s: sticker text = %q, want %q", tc.name, text, tc.wantText)
				}
				for _, want := range tc.wantWho {
					if !strings.Contains(who, want) {
						t.Errorf("%s: sticker who = %q, want it to carry %q", tc.name, who, want)
					}
				}
				if pill != tc.wantPill {
					t.Errorf("%s: pill = %v, want %v", tc.name, pill, tc.wantPill)
				}
				if next != tc.wantNext {
					t.Errorf("%s: next = %v, want %v", tc.name, next, tc.wantNext)
				}
			}
			assertStickerAgreesWithStore(t, st)

			if tc.after != nil {
				tc.after(t, st, notes)
			}
		})
	}
}
