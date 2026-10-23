package bibletext

// Decision 13's missing-key line can be put away per account on the device.
// These Android cases retain dismissal across sheet openings and launches,
// then lose it after reinstall or reset removes the no_backup store. Apple
// same-device backup restoration is covered separately.
//
// These drive the Settings sections the reader sees and the ✕ in them.

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// statusClose is a key field's status line and the ✕ in its row: line is the
// line itself, col the column the ✕ stands in and row the row holding both.
type statusClose struct {
	text    string
	btn     *widget.Button
	showing bool
	line    *statusLine
	col     *fyne.Container
	row     *fyne.Container
}

// keyStatusCloses finds, under root, each row that holds a key field's status
// line beside a column with a close button, an icon of the close mark and no
// words, and says what the line reads and whether the ✕ is showing.
func keyStatusCloses(root fyne.CanvasObject) []statusClose {
	var out []statusClose
	walkTree(root, func(o fyne.CanvasObject) {
		c, ok := o.(*fyne.Container)
		if !ok {
			return
		}
		var line *statusLine
		for _, child := range c.Objects {
			if s, ok := child.(*statusLine); ok {
				line = s
			}
		}
		if line == nil {
			return
		}
		for _, child := range c.Objects {
			col, ok := child.(*fyne.Container)
			if !ok {
				continue
			}
			walkTree(col, func(n fyne.CanvasObject) {
				if b, ok := n.(*widget.Button); ok && b.Text == "" && b.Icon != nil && b.Icon.Name() == theme.CancelIcon().Name() {
					out = append(out, statusClose{text: line.text, btn: b, showing: col.Visible() && b.Visible(), line: line, col: col, row: c})
				}
			})
		}
	})
	return out
}

// lineClose is the one ✕ beside a key field's status line under root.
func lineClose(t *testing.T, root fyne.CanvasObject) statusClose {
	t.Helper()
	found := keyStatusCloses(root)
	if len(found) != 1 {
		t.Fatalf("want one ✕ beside the key field's status line, found %d", len(found))
	}
	return found[0]
}

// refusingDeviceBoundStore is a device-bound store that answers reads and,
// once refuse is set, refuses every write, as Android's does when its
// Keystore call fails on the way in.
type refusingDeviceBoundStore struct {
	*deviceBoundTestStore
	refuse bool
}

func (s *refusingDeviceBoundStore) Write(account, value string) bool {
	if s.refuse {
		return false
	}
	return s.deviceBoundTestStore.Write(account, value)
}

func (s *refusingDeviceBoundStore) writeThisDeviceOnly(account, value string) bool {
	return s.Write(account, value)
}

// TestTheNotRestoredLineIsDismissedForThatRestore walks the API.Bible key's
// field through a restore, the ✕, the sheet opened again, the next launch, and
// the two kinds of new restore.
func TestTheNotRestoredLineIsDismissedForThatRestore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLE_API_KEY", "")
	withFakeSharedKeys(t)

	field := func(keys *keyStore) fyne.CanvasObject {
		st := sampleState()
		st.aiKeys = keys
		rows, _ := bibleKeySection(st, lightPalette, nil)
		return rows
	}

	store := newDeviceBoundTestStore()
	phone := restoredOntoNewPhone(t, store, bibleKeyID)
	rows := field(phone)
	x := lineClose(t, rows)
	if x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("the restored phone's field reads %q with the ✕ showing %v; want the line %q and its ✕", x.text, x.showing, approvedKeyNotRestoredLine)
	}

	x.btn.OnTapped()
	if x := lineClose(t, rows); x.text != "Free for personal use — no card, no charge." || x.showing {
		t.Fatalf("after the ✕ the field reads %q with the ✕ showing %v; want the field's own status and no ✕", x.text, x.showing)
	}
	if x := lineClose(t, field(phone)); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("the sheet opened again says the dismissed line: %q, ✕ showing %v", x.text, x.showing)
	}
	relaunched := &keyStore{prefs: phone.prefs, secrets: store}
	if x := lineClose(t, field(relaunched)); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("the next launch says the dismissed line again: %q, ✕ showing %v", x.text, x.showing)
	}
	if !relaunched.keyLeftOnOldDevice(bibleKeyID) {
		t.Fatal("the ✕ took away the record that the key was not restored, which the picker and a key pasted again still read")
	}

	// A new restore, from a backup taken after the ✕, onto another phone:
	// that phone's store starts empty, and its key is missing again.
	backup := newFakePrefs()
	for k, v := range phone.prefs.(*fakePrefs).m {
		backup.m[k] = v
	}
	another := &keyStore{prefs: backup, secrets: newDeviceBoundTestStore()}
	if x := lineClose(t, field(another)); x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("a new restore onto another phone reads %q with the ✕ showing %v; want the line back", x.text, x.showing)
	}

	// The same phone reinstalled and restored from its own backup: its store
	// is emptied, the record of the ✕ with it.
	store.m = map[string]string{}
	reinstalled := &keyStore{prefs: phone.prefs, secrets: store}
	if x := lineClose(t, field(reinstalled)); x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("the same phone reinstalled reads %q with the ✕ showing %v; want the line back", x.text, x.showing)
	}

	// The ✕ is only beside the line: a field that says anything else has
	// none showing.
	never := &keyStore{prefs: newFakePrefs(), secrets: newDeviceBoundTestStore()}
	if x := lineClose(t, field(never)); x.showing {
		t.Fatalf("a phone that never had a key shows a ✕ beside %q", x.text)
	}
	back := restoredOntoNewPhone(t, newDeviceBoundTestStore(), bibleKeyID)
	back.setBibleAPIKey("old-phone-key")
	if x := lineClose(t, field(back)); x.showing {
		t.Fatalf("with the key pasted again a ✕ shows beside %q", x.text)
	}
}

// TestTheNotRestoredLineIsDismissedBesideTheIncludedKey: a release build's
// field whose own key was not restored says the line while the included key
// works meanwhile; dismissed, it says what the included key's field says.
func TestTheNotRestoredLineIsDismissedBesideTheIncludedKey(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLE_API_KEY", "")
	withBundledKey(t, "the-included-key")
	withFakeSharedKeys(t)

	phone := restoredOntoNewPhone(t, newDeviceBoundTestStore(), bibleKeyID)
	if !phone.usingBundledBibleKey() {
		t.Fatal("control: with no key restored the included key must be in force")
	}
	st := sampleState()
	st.aiKeys = phone
	rows, _ := bibleKeySection(st, lightPalette, nil)
	x := lineClose(t, rows)
	if x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("the field reads %q with the ✕ showing %v; want the line and its ✕", x.text, x.showing)
	}
	x.btn.OnTapped()
	if x := lineClose(t, rows); x.text != "✓ Included with BibleText — or paste your own." || x.showing {
		t.Fatalf("after the ✕ the field reads %q with the ✕ showing %v; want the included key's status and no ✕", x.text, x.showing)
	}
	if !phone.usingBundledBibleKey() {
		t.Fatal("the ✕ turned the included key off; it dismisses a line and removes nothing")
	}
}

// TestTheNotRestoredLineDismissedWhereTheStoreRefuses: a store that refuses
// the record still has the ✕ dismiss the line for the rest of the run, the
// sheet opened again included, and the next launch says it again, as it is
// true and nothing kept the reader's ✕.
func TestTheNotRestoredLineDismissedWhereTheStoreRefuses(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	t.Setenv("BIBLE_API_KEY", "")
	withFakeSharedKeys(t)

	store := &refusingDeviceBoundStore{deviceBoundTestStore: newDeviceBoundTestStore()}
	phone := restoredOntoNewPhone(t, store, bibleKeyID)
	store.refuse = true
	field := func(keys *keyStore) fyne.CanvasObject {
		st := sampleState()
		st.aiKeys = keys
		rows, _ := bibleKeySection(st, lightPalette, nil)
		return rows
	}
	x := lineClose(t, field(phone))
	if x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("control: the field reads %q with the ✕ showing %v", x.text, x.showing)
	}
	x.btn.OnTapped()
	if x := lineClose(t, field(phone)); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("the store refused the record and the sheet opened again says the dismissed line: %q", x.text)
	}
	relaunched := &keyStore{prefs: phone.prefs, secrets: store}
	if x := lineClose(t, field(relaunched)); x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("nothing kept the ✕ and the next launch reads %q with the ✕ showing %v; want the line", x.text, x.showing)
	}
}

// TestTheNotRestoredLineIsDismissedUnderEachField: the same ✕ under an AI
// provider's field, and each field's ✕ puts away its own line only.
func TestTheNotRestoredLineIsDismissedUnderEachField(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	withFakeSharedKeys(t)
	info, ok := providerByID(defaultProviderID)
	if !ok {
		t.Fatal("control: the default provider is not registered")
	}

	store := newDeviceBoundTestStore()
	phone := restoredOntoNewPhone(t, store, info.ID, bibleKeyID)
	phone.setActiveProvider(info.ID)
	phone.setAIEnabled(true)
	state := deferredTestState(t, app)
	state.aiKeys = phone

	// The sheet holds two key fields, the assistant's above the API.Bible
	// key's (ASSISTANT, then TRANSLATIONS), so two rows, in that order.
	sheet := func() (ai, bible statusClose) {
		t.Helper()
		showAISettings(state)
		rows := keyStatusCloses(settingsPopup(t, state))
		if len(rows) != 2 {
			t.Fatalf("want a ✕ beside each of the sheet's two key fields' status lines, found %d", len(rows))
		}
		return rows[0], rows[1]
	}

	ai, bible := sheet()
	for name, x := range map[string]statusClose{info.Name: ai, "API.Bible": bible} {
		if x.text != approvedKeyNotRestoredLine || !x.showing {
			t.Fatalf("the restored phone's %s field reads %q with the ✕ showing %v; want the line and its ✕", name, x.text, x.showing)
		}
	}
	ai.btn.OnTapped()
	state.window.Canvas().Overlays().Top().Hide()

	ai, bible = sheet()
	if ai.text != info.KeyHint || ai.showing {
		t.Fatalf("after its ✕ the %s field reads %q with the ✕ showing %v, the sheet opened again; want its hint and no ✕", info.Name, ai.text, ai.showing)
	}
	if bible.text != approvedKeyNotRestoredLine || !bible.showing {
		t.Fatalf("the %s field's ✕ put away the API.Bible field's line too: it reads %q, ✕ showing %v", info.Name, bible.text, bible.showing)
	}
	bible.btn.OnTapped()
	state.window.Canvas().Overlays().Top().Hide()

	relaunched := &keyStore{prefs: phone.prefs, secrets: store}
	relaunched.setActiveProvider(info.ID)
	state.aiKeys = relaunched
	ai, bible = sheet()
	state.window.Canvas().Overlays().Top().Hide()
	if ai.text != info.KeyHint || ai.showing || bible.text != "Free for personal use — no card, no charge." || bible.showing {
		t.Fatalf("the next launch reads %q (✕ %v) and %q (✕ %v); want both lines dismissed", ai.text, ai.showing, bible.text, bible.showing)
	}

	// A new restore onto another phone brings both back.
	backup := newFakePrefs()
	for k, v := range phone.prefs.(*fakePrefs).m {
		backup.m[k] = v
	}
	state.aiKeys = &keyStore{prefs: backup, secrets: newDeviceBoundTestStore()}
	ai, bible = sheet()
	state.window.Canvas().Overlays().Top().Hide()
	if ai.text != approvedKeyNotRestoredLine || !ai.showing || bible.text != approvedKeyNotRestoredLine || !bible.showing {
		t.Fatalf("a new restore onto another phone reads %q (✕ %v) and %q (✕ %v); want both lines back", ai.text, ai.showing, bible.text, bible.showing)
	}
}
