package bibletext

// A key that is gone is said for what it is. Decisions 11 and 13 in
// docs/ROADMAP.md: a reader who removed their API.Bible key in Settings gave
// the translation it opens up, and is told so rather than promised a launch
// that will try it again. A separate store that definitively lacks a key
// whose saved mark remains gets a neutral recovery line under its field.
//
// These drive the app's own calls only: the restore the launch runs, the
// picker's notice, the Settings sections the reader sees. The launch walk
// (version_launch_flow_test.go) holds both fates in every launch cell; these
// hold the sentences word for word, what the next launch does, and the
// fields.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Exact recovery sentences: the picker distinguishes a key removed on
// purpose from a missing saved key; both surfaces describe only the absence.
const (
	approvedKeyRemovedLine       = "You removed your API.Bible key, so World English Bible is shown in place of New King James Version."
	approvedKeyNotRestoredLine   = "Your saved key isn't available on this device. Paste it again here."
	approvedKeyNotRestoredPicker = "Your saved API.Bible key isn't available on this device, so World English Bible is shown in place of New King James Version."
)

// rememberedNKJVLine is D9's sentence for the NKJV remembered while the WEB is
// shown, its row selectable (D24), and rememberedNKJVLineNoRow the same with
// the row not selectable.
const (
	rememberedNKJVLineNoRow = "New King James Version could not be opened this time — World English Bible is shown instead. Your choice is remembered and tried again each time the app starts."
	rememberedNKJVLine      = rememberedNKJVLineNoRow + " To try now, choose it above."
)

// launchLive runs one launch of rs, its translation meeting fate, through the
// restore, the tail that falls back to the default's start, and the hand-off
// to the live state, and returns the live state the reader uses.
func launchLive(t *testing.T, rs readingState, choice savedChoice, fate choiceFate) *AppState {
	t.Helper()
	restoreVersionFate(t, choice, fate, widerCanonBible())
	base := fullValidBible()
	state := &AppState{
		Bible:          base,
		CurrentVersion: defaultVersionID,
		currentMode:    modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base},
		loadPhase:      loadReady,
	}
	restored, err := restoreReadingState(state, rs, base)
	if err != nil {
		t.Fatalf("control: the launch must open: %v", err)
	}
	if !restored {
		startAtDefault(state, rs.Recent, true)
	}
	live := NewLoadingState()
	adoptLaunch(live, state)
	return live
}

// TestAKeyRemovedOnPurposeGivesTheTranslationUp is decision 11. The launch
// after the reader removed their key cannot open the NKJV; it used to remember
// it and promise to try it at every launch, which nothing can do until they
// add a key. Now the picker says why the WEB is shown, the next save names the
// WEB, and the launch after that tries nothing and has nothing to say.
func TestAKeyRemovedOnPurposeGivesTheTranslationUp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	live := launchLive(t, readingState{Version: "nkjv", Book: "John", Chapter: 1}, savedLicensed, fateKeyCleared)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("control: with the key removed the launch must show the default; shows %s", live.CurrentVersion)
	}
	if n := fullPendingNotice(live); n != approvedKeyRemovedLine {
		t.Fatalf("the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
	}
	saved := snapshotReadingState(live, 0, 0, 0, 0, 0)
	if saved.Version != defaultVersionID {
		t.Fatalf("the next save writes %q, so the next launch tries the NKJV again; want %q", saved.Version, defaultVersionID)
	}

	// The next launch, the key still removed: the saved place opens in the
	// WEB, nothing asks for the NKJV and nothing is said.
	asked := 0
	prev := loadVersionForRestore
	loadVersionForRestore = func(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
		asked++
		return prev(v, base)
	}
	t.Cleanup(func() { loadVersionForRestore = prev })
	base := fullValidBible()
	next := &AppState{Bible: base, CurrentVersion: defaultVersionID, currentMode: modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base}, loadPhase: loadReady}
	if ok, err := restoreReadingState(next, saved, base); !ok || err != nil {
		t.Fatalf("the next launch did not open the saved place: restored %v, err %v", ok, err)
	}
	if asked != 0 || next.CurrentVersion != defaultVersionID || fullPendingNotice(next) != "" {
		t.Fatalf("the next launch asked for a translation %d times, shows %s and says %q; want none, the default, nothing",
			asked, next.CurrentVersion, fullPendingNotice(next))
	}

	// A key added again in the same session: the sentence says the key is
	// removed, so it goes, and the NKJV's own row says what to do.
	keys := sharedKeys()
	keys.setBibleAPIKey("a-new-key")
	keys.noteBibleKeyCleared(false)
	if n := fullPendingNotice(live); strings.Contains(n, "You removed your API.Bible key") {
		t.Fatalf("with a key added again the picker still says it was removed: %q", n)
	}
	keys.setBibleAPIKey("")
	keys.noteBibleKeyCleared(true)
	if n := fullPendingNotice(live); n != approvedKeyRemovedLine {
		t.Fatalf("control: with the key removed again the sentence must come back; reads %q", n)
	}

	// Any landing ends the default's being shown in its place. Off the default
	// the sentence is not said anyway, so the reader's coming back to the
	// default is what shows the landing spent it.
	bsb, ok := versionByID("bsb")
	if !ok {
		t.Skip("bsb not registered")
	}
	applyLoadedVersion(live, bsb, fullValidBible(), modeReal, byArrival)
	if n := fullPendingNotice(live); n != "" {
		t.Fatalf("a link to the BSB landed and the picker still says %q", n)
	}
	web, _ := versionByID(defaultVersionID)
	applyLoadedVersion(live, web, fullValidBible(), modeReal, byReader)
	if n := fullPendingNotice(live); n != "" {
		t.Fatalf("a link to the BSB landed, the reader then chose %s, and the picker says %q", web.Name, n)
	}
	if saved := snapshotReadingState(live, 0, 0, 0, 0, 0).Version; saved != defaultVersionID {
		t.Fatalf("the reader chose %s and the next save writes %q", web.Name, saved)
	}
}

// bibleKeyField is the API.Bible key field in the section's rows, found
// without creating a renderer (walkTree), so the tests that type into it run
// under -race too.
func bibleKeyField(t *testing.T, rows fyne.CanvasObject) *widget.Entry {
	t.Helper()
	var field *widget.Entry
	walkTree(rows, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.Password && field == nil {
			field = e
		}
	})
	if field == nil {
		t.Fatal("control: the section has no API.Bible key field")
	}
	return field
}

// rememberedWithKey is the state a launch leaves when it could not ask the
// store and remembered the NKJV (D9), the store answering later in the
// session and holding the reader's key, with the Settings section the reader
// meets it in. The section is built with the light palette and no theme set
// on the app, so it is safe under -race (widgettree_test.go).
func rememberedWithKey(t *testing.T) (*AppState, fyne.CanvasObject) {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	keys := withFakeSharedKeys(t)
	keys.setBibleAPIKey("the-readers-key")
	keys.noteBibleKeyCleared(false)

	st := sampleState()
	st.aiKeys = keys
	st.CurrentVersion = defaultVersionID
	st.preferredVersion = "nkjv" // the record a launch makes when it cannot ask the store (D9)
	if n := fullPendingNotice(st); n != rememberedNKJVLine {
		t.Fatalf("control: the remembered NKJV must be promised a launch that tries it; the picker reads %q", n)
	}
	rows, _ := bibleKeySection(st, lightPalette, nil)
	return st, rows
}

// TestAKeyClearedWhileRememberedIsGivenUpThere is decision 11 inside a
// session. A launch that could not ask the store remembered the NKJV and
// promised to try it at every launch; the reader then removes their key in
// Settings, with Clear or by emptying the field by hand. The next launch will
// give it up, so the promise is withdrawn now: the picker says why, and the
// next save names what is on screen.
func TestAKeyClearedWhileRememberedIsGivenUpThere(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	for name, remove := range map[string]func(t *testing.T, rows fyne.CanvasObject){
		"Clear": func(t *testing.T, rows fyne.CanvasObject) {
			clear := findTreeButton(rows, "Clear")
			if clear == nil || clear.Disabled() {
				t.Fatal("control: Clear must be there and enabled while the reader's key is stored")
			}
			clear.OnTapped()
		},
		"the field emptied by hand": func(t *testing.T, rows fyne.CanvasObject) {
			entry := bibleKeyField(t, rows)
			if entry.Text != "the-readers-key" {
				t.Fatalf("control: the field must hold the reader's key; holds %q", entry.Text)
			}
			entry.SetText("")
		},
	} {
		t.Run(name, func(t *testing.T) {
			st, rows := rememberedWithKey(t)
			remove(t, rows)
			if n := fullPendingNotice(st); n != approvedKeyRemovedLine {
				t.Fatalf("the key removed, the picker reads\n  %q\nwant\n  %q", n, approvedKeyRemovedLine)
			}
			if saved := snapshotReadingState(st, 0, 0, 0, 0, 0).Version; saved != defaultVersionID {
				t.Fatalf("the key removed, the next save writes %q, so the next launch tries the NKJV again", saved)
			}
		})
	}
}

// TestAKeyEnteredAgainMakesTheTranslationTheChoiceAgain: a translation given
// up with the key was given up only because nothing could open it without
// one. A reader who enters a key in the field again, as one replacing their
// key does by emptying the field or tapping Clear first, has the translation
// remembered again as D9 records it: the next save names it and the picker
// says D9's sentence, offering its row. That holds whether the session gave
// it up or the launch did.
func TestAKeyEnteredAgainMakesTheTranslationTheChoiceAgain(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	check := func(t *testing.T, st *AppState) {
		t.Helper()
		if saved := snapshotReadingState(st, 0, 0, 0, 0, 0).Version; saved != "nkjv" {
			t.Errorf("the reader's key is back and the next save writes %q: the NKJV they chose is dropped", saved)
		}
		if n := fullPendingNotice(st); n != rememberedNKJVLine {
			t.Errorf("the reader's key is back and the picker reads\n  %q\nwant\n  %q", n, rememberedNKJVLine)
		}
	}

	for name, remove := range map[string]func(t *testing.T, rows fyne.CanvasObject){
		"the field emptied by hand": func(t *testing.T, rows fyne.CanvasObject) {
			bibleKeyField(t, rows).SetText("")
		},
		"Clear": func(t *testing.T, rows fyne.CanvasObject) {
			clear := findTreeButton(rows, "Clear")
			if clear == nil || clear.Disabled() {
				t.Fatal("control: Clear must be there and enabled while the reader's key is stored")
			}
			clear.OnTapped()
		},
	} {
		t.Run(name+", then the key entered again", func(t *testing.T) {
			st, rows := rememberedWithKey(t)
			remove(t, rows)
			if st.keyRemovedVersion != "nkjv" {
				t.Fatalf("control: removing the key must give the NKJV up; given up %q", st.keyRemovedVersion)
			}
			bibleKeyField(t, rows).SetText("the-readers-key")
			check(t, st)
		})
	}

	t.Run("given up by the launch, then a key pasted", func(t *testing.T) {
		live := launchLive(t, readingState{Version: "nkjv", Book: "John", Chapter: 1}, savedLicensed, fateKeyCleared)
		if n := fullPendingNotice(live); n != approvedKeyRemovedLine {
			t.Fatalf("control: the launch must give the NKJV up and say why; the picker reads %q", n)
		}
		// The walk's store answers "none" to every read; the reader's own
		// keeps what is written to it, as a real one does.
		keys := sharedKeys()
		keys.secrets = &fakeSecrets{m: map[string]string{}}
		live.aiKeys = keys
		rows, _ := bibleKeySection(live, lightPalette, nil)
		bibleKeyField(t, rows).SetText("a-new-key")
		check(t, live)
	})
}

// TestAStoreThatCannotAnswerKeepsTheRecord: decision 11 gives a translation
// up only where the store answered that it holds no key. A launch whose store
// cannot answer, before a phone's first unlock or on a Keystore error, cannot
// tell that no key is there, though the reader once emptied the field (a
// removal the store refused leaves the key in place), so D9's record stands:
// the next save names the NKJV, and the picker says it could not be opened
// this time, not that the reader removed their key.
func TestAStoreThatCannotAnswerKeepsTheRecord(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")
	secrets := &fakeSecrets{m: map[string]string{}}
	ks := &keyStore{prefs: newFakePrefs(), secrets: secrets}
	ks.setBibleAPIKey("")
	ks.noteBibleKeyCleared(true)
	prev := sharedKeys
	sharedKeys = func() *keyStore { return ks }
	t.Cleanup(func() { sharedKeys = prev })
	if !ks.bibleKeyRemovedOnPurpose() {
		t.Fatal("control: with the store answering, the key must read as removed on purpose")
	}
	secrets.failRead = true

	base := fullValidBible()
	state := &AppState{Bible: base, CurrentVersion: defaultVersionID, currentMode: modeReal,
		loadedVersions: map[string]*BibleData{defaultVersionID: base}, loadPhase: loadReady}
	if _, err := restoreReadingState(state, readingState{Version: "nkjv", Book: "John", Chapter: 1}, base); err != nil {
		t.Fatalf("control: the launch must open: %v", err)
	}
	live := NewLoadingState()
	adoptLaunch(live, state)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("control: with no key readable the launch must show the default; shows %s", live.CurrentVersion)
	}
	if saved := snapshotReadingState(live, 0, 0, 0, 0, 0).Version; saved != "nkjv" {
		t.Errorf("a store that cannot answer gave the NKJV up: the next save writes %q", saved)
	}
	if n := fullPendingNotice(live); n != rememberedNKJVLineNoRow {
		t.Errorf("a store that cannot answer: the picker reads\n  %q\nwant\n  %q", n, rememberedNKJVLineNoRow)
	}
}

// TestAKeyNotRestoredIsSaidInThePicker is decision 13's picker line. A
// restored phone holds the reader's settings, which remember the NKJV, and
// not their key. The NKJV stays remembered, since the reader did not give it
// up and comes back once they paste the key, and the picker says why it is
// not shown, rather than "could not be opened this time".
func TestAKeyNotRestoredIsSaidInThePicker(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	live := launchLive(t, readingState{Version: "nkjv", Book: "John", Chapter: 1}, savedLicensed, fateKeyOnOldDevice)
	if live.CurrentVersion != defaultVersionID {
		t.Fatalf("control: with no key on the phone the launch must show the default; shows %s", live.CurrentVersion)
	}
	if n := fullPendingNotice(live); n != approvedKeyNotRestoredPicker {
		t.Fatalf("the picker reads\n  %q\nwant\n  %q", n, approvedKeyNotRestoredPicker)
	}
	if saved := snapshotReadingState(live, 0, 0, 0, 0, 0).Version; saved != "nkjv" {
		t.Fatalf("the next save writes %q: the reader did not give the NKJV up", saved)
	}
	// The key pasted again: the cause is gone, and the general sentence, which
	// offers the row, takes its place.
	keys := sharedKeys()
	keys.setBibleAPIKey("old-phone-key")
	if n := fullPendingNotice(live); strings.Contains(n, "Your saved API.Bible key isn't available") || !strings.Contains(n, "To try now, choose it above.") {
		t.Fatalf("with the key pasted again the picker reads %q", n)
	}
}

// reinstalledOnTheSamePhone saves a key for each account on a phone, through
// the calls the reader's Settings make, and returns that phone after a
// reinstall or a factory reset that restored its own backup: the same
// settings, and its store emptied, since no backup carries a store whose keys
// stay on the device.
func reinstalledOnTheSamePhone(t *testing.T, accounts ...string) *keyStore {
	t.Helper()
	store := newDeviceBoundTestStore()
	ks := &keyStore{prefs: newFakePrefs(), secrets: store}
	for _, a := range accounts {
		if a == bibleKeyID {
			ks.setBibleAPIKey("same-phone-key")
			ks.noteBibleKeyCleared(false)
		} else {
			ks.setAPIKey(a, "same-phone-key")
		}
		if ks.apiKey(a) != "same-phone-key" {
			t.Fatalf("control: the phone did not keep the %s key", a)
		}
	}
	store.m = map[string]string{}
	return ks
}

// TestAKeyNotRestoredIsSaidUnderItsField is decision 13's field line, for the
// API.Bible key and an AI provider's key alike: the empty field of a phone
// whose settings a restore brought back without the key, another phone or
// the same one after a reinstall or a factory reset, says the key was not
// restored, in place of the hint for getting one. Only where the app knows
// it: the controls are the same settings on a store whose keys travel with
// them, a store that cannot answer, a phone that never had a key, and the key
// pasted again.
func TestAKeyNotRestoredIsSaidUnderItsField(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")

	bibleField := func(t *testing.T, keys *keyStore) []string {
		t.Helper()
		st := sampleState()
		st.theme = th
		st.aiKeys = keys
		rows, _ := bibleKeySection(st, st.pal(), nil)
		return treeTexts(rows)
	}
	has := func(texts []string, want string) bool {
		for _, s := range texts {
			if s == want {
				return true
			}
		}
		return false
	}

	t.Run("API.Bible key", func(t *testing.T) {
		withFakeSharedKeys(t)
		if got := bibleField(t, restoredOntoNewPhone(t, newDeviceBoundTestStore(), bibleKeyID)); !has(got, approvedKeyNotRestoredLine) {
			t.Fatalf("the restored phone's empty field says %q, want the line %q", got, approvedKeyNotRestoredLine)
		}
		if got := bibleField(t, reinstalledOnTheSamePhone(t, bibleKeyID)); !has(got, approvedKeyNotRestoredLine) {
			t.Fatalf("the reinstalled phone's empty field says %q, want the line %q", got, approvedKeyNotRestoredLine)
		}
		for name, store := range map[string]secretStore{
			"a store whose keys travel":  emptySecretStore{},
			"a store that cannot answer": unreadableDeviceBoundStore{},
		} {
			if got := bibleField(t, restoredOntoNewPhone(t, store, bibleKeyID)); has(got, approvedKeyNotRestoredLine) {
				t.Errorf("%s: the field says the key was not restored, which the app cannot know there", name)
			}
		}
		if got := bibleField(t, &keyStore{prefs: newFakePrefs(), secrets: newDeviceBoundTestStore()}); has(got, approvedKeyNotRestoredLine) ||
			!has(got, "Free for personal use — no card, no charge.") {
			t.Errorf("a phone that never had a key reads %q", got)
		}
		back := restoredOntoNewPhone(t, newDeviceBoundTestStore(), bibleKeyID)
		back.setBibleAPIKey("old-phone-key")
		if got := bibleField(t, back); has(got, approvedKeyNotRestoredLine) {
			t.Errorf("with the key pasted again the field still says it was not restored: %q", got)
		}
	})

	t.Run("AI key", func(t *testing.T) {
		info, ok := providerByID(defaultProviderID)
		if !ok {
			t.Fatal("control: the default provider is not registered")
		}
		aiField := func(t *testing.T, keys *keyStore) []string {
			t.Helper()
			keys.setActiveProvider(info.ID)
			keys.setAIEnabled(true)
			state := deferredTestState(t, app)
			state.aiKeys = keys
			showAISettings(state)
			texts := treeTexts(settingsPopup(t, state))
			state.window.Canvas().Overlays().Top().Hide()
			return texts
		}
		if got := aiField(t, restoredOntoNewPhone(t, newDeviceBoundTestStore(), info.ID)); !has(got, approvedKeyNotRestoredLine) || has(got, info.KeyHint) {
			t.Fatalf("the restored phone's empty %s field says %q, want the line %q in place of the hint", info.Name, got, approvedKeyNotRestoredLine)
		}
		if got := aiField(t, reinstalledOnTheSamePhone(t, info.ID)); !has(got, approvedKeyNotRestoredLine) || has(got, info.KeyHint) {
			t.Fatalf("the reinstalled phone's empty %s field says %q, want the line %q in place of the hint", info.Name, got, approvedKeyNotRestoredLine)
		}
		if got := aiField(t, restoredOntoNewPhone(t, emptySecretStore{}, info.ID)); has(got, approvedKeyNotRestoredLine) || !has(got, info.KeyHint) {
			t.Errorf("on a store whose keys travel the field reads %q, want the hint and no claim", got)
		}
		if got := aiField(t, &keyStore{prefs: newFakePrefs(), secrets: newDeviceBoundTestStore()}); has(got, approvedKeyNotRestoredLine) {
			t.Errorf("a phone that never had a key says its key was not restored: %q", got)
		}
	})
}

// unreadableDeviceBoundStore is a device-bound store that cannot answer, as
// Android's does when its Keystore call fails: it may hold the key.
type unreadableDeviceBoundStore struct{}

func (unreadableDeviceBoundStore) Name() string                            { return "unreadable device-bound test store" }
func (unreadableDeviceBoundStore) Read(string) (string, bool, bool)        { return "", false, false }
func (unreadableDeviceBoundStore) Write(string, string) bool               { return false }
func (unreadableDeviceBoundStore) restoreCanLeaveKeys() bool               { return true }
func (unreadableDeviceBoundStore) writeThisDeviceOnly(string, string) bool { return false }
