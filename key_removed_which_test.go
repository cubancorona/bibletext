package bibletext

// Decision 11's line names the key the reader removed (docs/ROADMAP.md,
// 1.2.23, row 16). A reader of a release build who taps Clear while only the
// key included with BibleText is in force removed that key, not one of their
// own, and the picker says so in the words approved on 21 October 2026; a
// reader who removed a key of their own is told what they always were.
//
// These drive the app's own calls: the Settings section's Clear and its
// field, the picker's notice, and the launch that follows.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// approvedIncludedKeyRemovedLine is the picker's sentence for the included key
// removed, word for word as approved on 21 October 2026.
const approvedIncludedKeyRemovedLine = "You removed the included API.Bible key, so World English Bible is shown in place of New King James Version."

// releaseBuildSettings is a release build, its included key in force, whose
// launch could not open the NKJV and remembered it (D9), with the Settings
// section the reader meets it in. ownKey, when not empty, is a key of the
// reader's own saved in the field, which takes precedence over the included
// one.
func releaseBuildSettings(t *testing.T, ownKey string) (*AppState, fyne.CanvasObject) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	withBundledKey(t, "the-included-key")
	keys := withFakeSharedKeys(t)
	if ownKey != "" {
		keys.setBibleAPIKey(ownKey)
		keys.noteBibleKeyCleared(false)
	}
	if got := keys.usingBundledBibleKey(); got != (ownKey == "") {
		t.Fatalf("control: the included key in force = %v, want %v", got, ownKey == "")
	}
	st := sampleState()
	st.aiKeys = keys
	st.CurrentVersion = defaultVersionID
	st.preferredVersion = "nkjv"
	if nk, ok := versionByID("nkjv"); !ok || !nk.canSelect() {
		t.Fatal("control: the NKJV must open with a key in force, or the cell is not the state it names")
	}
	rows, _ := bibleKeySection(st, lightPalette, nil)
	return st, rows
}

// tapClear taps the section's Clear, which must be there and enabled.
func tapClear(t *testing.T, rows fyne.CanvasObject) {
	t.Helper()
	clear := findTreeButton(rows, "Clear")
	if clear == nil || clear.Disabled() {
		t.Fatal("control: Clear must be there and enabled while a key is in force")
	}
	clear.OnTapped()
}

// launchAfter is the next launch of a reader whose saved place is in the
// NKJV, with the keys the session left, and the live state it hands over.
func launchAfter(t *testing.T) *AppState {
	t.Helper()
	base := fullValidBible()
	state := &AppState{Bible: base, CurrentVersion: defaultVersionID, currentMode: modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base}, loadPhase: loadReady}
	restored, err := restoreReadingState(state, readingState{Version: "nkjv", Book: "John", Chapter: 1}, base)
	if err != nil {
		t.Fatalf("control: the launch must open: %v", err)
	}
	if !restored {
		startAtDefault(state, nil, true)
	}
	live := NewLoadingState()
	adoptLaunch(live, state)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("control: with the key removed the launch must show the default; shows %s", live.CurrentVersion)
	}
	return live
}

// TestTheRemovedKeyIsNamedInThePicker: Clear with only the included key in
// force names the included key, in the session and at the next launch; a key
// of the reader's own removed in the same build, with Clear or by emptying
// the field by hand, is named as theirs.
func TestTheRemovedKeyIsNamedInThePicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}

	t.Run("Clear with the included key in force", func(t *testing.T) {
		st, rows := releaseBuildSettings(t, "")
		tapClear(t, rows)
		if st.keyRemovedVersion != "nkjv" {
			t.Fatalf("control: removing the key must give the NKJV up; given up %q", st.keyRemovedVersion)
		}
		if n := fullPendingNotice(st); n != approvedIncludedKeyRemovedLine {
			t.Fatalf("the included key removed, the picker reads\n  %q\nwant\n  %q", n, approvedIncludedKeyRemovedLine)
		}
		if n := fullPendingNotice(launchAfter(t)); n != approvedIncludedKeyRemovedLine {
			t.Fatalf("the launch after the included key was removed: the picker reads\n  %q\nwant\n  %q", n, approvedIncludedKeyRemovedLine)
		}
	})

	for name, remove := range map[string]func(t *testing.T, rows fyne.CanvasObject){
		"Clear":                     tapClear,
		"the field emptied by hand": func(t *testing.T, rows fyne.CanvasObject) { bibleKeyField(t, rows).SetText("") },
	} {
		t.Run("the reader's own key, "+name, func(t *testing.T) {
			st, rows := releaseBuildSettings(t, "the-readers-key")
			remove(t, rows)
			if n := fullPendingNotice(st); n != approvedKeyRemovedLine {
				t.Fatalf("the reader's own key removed, the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
			}
			if n := fullPendingNotice(launchAfter(t)); n != approvedKeyRemovedLine {
				t.Fatalf("the launch after the reader's own key was removed: the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
			}
		})
	}

	// The field saves each change, so a key typed in over the included one
	// is the reader's own until it is deleted again: the key in force when
	// the field went empty was theirs.
	t.Run("a key typed in and deleted again", func(t *testing.T) {
		st, rows := releaseBuildSettings(t, "")
		field := bibleKeyField(t, rows)
		field.SetText("t")
		field.SetText("")
		if n := fullPendingNotice(st); n != approvedKeyRemovedLine {
			t.Fatalf("a key typed in and deleted again: the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
		}
	})

	// A key entered again and removed again is named for the second removal:
	// the included key, then the reader's own, says the reader's own.
	t.Run("the included key, then a key of the reader's own", func(t *testing.T) {
		st, rows := releaseBuildSettings(t, "")
		tapClear(t, rows)
		if n := fullPendingNotice(st); n != approvedIncludedKeyRemovedLine {
			t.Fatalf("control: the included key removed, the picker reads %q", n)
		}
		field := bibleKeyField(t, rows)
		field.SetText("the-readers-key")
		if n := fullPendingNotice(st); n == approvedIncludedKeyRemovedLine || n == approvedKeyRemovedLine {
			t.Fatalf("with a key entered again the picker still says one was removed: %q", n)
		}
		field.SetText("")
		if n := fullPendingNotice(st); n != approvedKeyRemovedLine {
			t.Fatalf("the reader's own key removed after the included one: the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
		}
	})
}

// recoveringStore is a credential store that, while down, can neither read
// nor write, and afterwards answers as an ordinary empty store does.
type recoveringStore struct {
	down bool
	m    map[string]string
}

func (s *recoveringStore) Name() string { return "recovering test store" }
func (s *recoveringStore) Read(account string) (string, bool, bool) {
	if s.down {
		return "", false, false
	}
	v, ok := s.m[account]
	return v, ok, true
}
func (s *recoveringStore) Write(account, value string) bool {
	if s.down {
		return false
	}
	if value == "" {
		delete(s.m, account)
	} else {
		s.m[account] = value
	}
	return true
}

// TestAClearTappedAgainOverARefusedDeleteNamesTheIncludedKey: Clear with
// only the included key in force, where the store refuses the delete, leaves
// Clear enabled under "Couldn't remove the stored key", and a reader taps it
// again. Nothing is in force at the second tap, the included key being off
// since the first, so the record still names the included key, and once the
// store answers the picker says so.
func TestAClearTappedAgainOverARefusedDeleteNamesTheIncludedKey(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	withBundledKey(t, "the-included-key")
	keys := withFakeSharedKeys(t)
	store := &recoveringStore{down: true, m: map[string]string{}}
	keys.secrets = store
	if !keys.usingBundledBibleKey() {
		t.Fatal("control: the included key must be in force while the store cannot answer")
	}
	st := sampleState()
	st.aiKeys = keys
	st.CurrentVersion = defaultVersionID
	st.preferredVersion = "nkjv"
	rows, _ := bibleKeySection(st, lightPalette, nil)

	tapClear(t, rows)
	if !treeHasText(rows, "Couldn't remove the stored key. Please try again.") {
		t.Fatal("control: a refused delete must say so and leave Clear enabled, or the second tap is not reachable")
	}
	if !keys.removedBibleKeyWasIncluded() {
		t.Fatal("control: the first Clear must record the included key")
	}
	tapClear(t, rows)
	if !keys.removedBibleKeyWasIncluded() {
		t.Errorf("a second Clear with no key in force recorded %q, naming the reader's own key, where the only key removed was the included one",
			keys.prefs.String(prefBibleKeyCleared))
	}

	store.down = false
	if n := fullPendingNotice(launchAfter(t)); n != approvedIncludedKeyRemovedLine {
		t.Fatalf("the launch once the store answers: the picker reads\n  %q\nwant\n  %q", n, approvedIncludedKeyRemovedLine)
	}

	// A key of the reader's own in force at the second tap is theirs: here
	// the store held it all along and could not read it at the first tap,
	// when the included key was the one in force.
	t.Run("the reader's own key read again before the second tap", func(t *testing.T) {
		keys := withFakeSharedKeys(t)
		store := &recoveringStore{down: true, m: map[string]string{bibleKeyID: "the-readers-key"}}
		keys.secrets = store
		st := sampleState()
		st.aiKeys = keys
		st.CurrentVersion = defaultVersionID
		st.preferredVersion = "nkjv"
		rows, _ := bibleKeySection(st, lightPalette, nil)
		tapClear(t, rows)
		if !keys.removedBibleKeyWasIncluded() {
			t.Fatal("control: the first Clear, with the store unread and the included key in force, must record the included key")
		}
		store.down = false
		if keys.bibleAPIKey() != "the-readers-key" {
			t.Fatal("control: once the store answers, the reader's own key must be the one in force")
		}
		tapClear(t, rows)
		if keys.removedBibleKeyWasIncluded() {
			t.Error("the reader's own key, in force at the second Clear, is recorded as the included key")
		}
	})
}
