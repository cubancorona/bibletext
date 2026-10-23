package bibletext

// Decision 13 on the iPhone and iPad (docs/ROADMAP.md, 1.2.23, row 17): one
// rule with Android. Where the restored settings say a key was saved and the
// Keychain holds none, the empty key field says the approved line with its ✕,
// and the picker says its line for a remembered NKJV, exactly as Android
// does. The Keychain's keys travel in encrypted and iCloud backups and to a
// new device in a direct transfer, and stay when the app is deleted; an
// unencrypted computer backup restores them only onto the device it was taken
// from, so put on another iPhone it brings back the settings without them.
// The record of the ✕ is the one Keychain item written ThisDeviceOnly, which
// Apple never migrates to another device.
//
// These drive the app's own calls, the Settings sections and the picker, over
// a stand-in for the Keychain as ai_secure_store_darwin.go writes it, and read
// that file for what only an iOS build compiles.

import (
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// iPhoneKeychainItem is one item in iPhoneKeychainTestStore.
type iPhoneKeychainItem struct {
	value          string
	thisDeviceOnly bool
}

// iPhoneKeychainTestStore stands in for the iPhone's and iPad's Keychain as
// the app writes it (ai_secure_store_darwin.go): Write keeps a key as an item
// that travels (AfterFirstUnlock), re-classing one an earlier build kept
// ThisDeviceOnly; writeThisDeviceOnly keeps an item that does not. What a
// restore onto another device brings of it is onAnotherDevice's.
type iPhoneKeychainTestStore struct {
	items map[string]iPhoneKeychainItem
	// travelling and deviceOnly count the writes that stored an item, by
	// the item's class.
	travelling, deviceOnly int
}

func newIPhoneKeychainTestStore() *iPhoneKeychainTestStore {
	return &iPhoneKeychainTestStore{items: map[string]iPhoneKeychainItem{}}
}

func (s *iPhoneKeychainTestStore) Name() string { return "Keychain" }

func (s *iPhoneKeychainTestStore) Read(account string) (string, bool, bool) {
	it, ok := s.items[account]
	return it.value, ok, true
}

func (s *iPhoneKeychainTestStore) put(account, value string, thisDeviceOnly bool) bool {
	if value == "" {
		delete(s.items, account)
		return true
	}
	s.items[account] = iPhoneKeychainItem{value: value, thisDeviceOnly: thisDeviceOnly}
	if thisDeviceOnly {
		s.deviceOnly++
	} else {
		s.travelling++
	}
	return true
}

func (s *iPhoneKeychainTestStore) Write(account, value string) bool {
	return s.put(account, value, false)
}

func (s *iPhoneKeychainTestStore) writeThisDeviceOnly(account, value string) bool {
	return s.put(account, value, true)
}

func (s *iPhoneKeychainTestStore) restoreCanLeaveKeys() bool { return true }

// iPhoneBackup is the way a backup of one iPhone or iPad reaches another.
type iPhoneBackup int

const (
	// encryptedBackup is an encrypted computer backup, an iCloud backup or a
	// direct transfer: they carry the items that travel.
	encryptedBackup iPhoneBackup = iota
	// unencryptedBackup is an unencrypted computer backup, which restores
	// Keychain items only onto the device it was taken from.
	unencryptedBackup
)

func (b iPhoneBackup) String() string {
	if b == encryptedBackup {
		return "an encrypted backup"
	}
	return "an unencrypted computer backup"
}

// onAnotherDevice is this Keychain as a restore of its device's backup onto
// another device leaves that device's: every item that travels from an
// encrypted backup, none from an unencrypted one, and never an item kept
// ThisDeviceOnly.
func (s *iPhoneKeychainTestStore) onAnotherDevice(b iPhoneBackup) *iPhoneKeychainTestStore {
	out := newIPhoneKeychainTestStore()
	if b == unencryptedBackup {
		return out
	}
	for k, it := range s.items {
		if !it.thisDeviceOnly {
			out.items[k] = it
		}
	}
	return out
}

// copyOfPrefs is the settings as a backup restores them.
func copyOfPrefs(p prefStore) *fakePrefs {
	out := newFakePrefs()
	for k, v := range p.(*fakePrefs).m {
		out.m[k] = v
	}
	return out
}

// iPhoneWithKeys saves a key for each account on an iPhone, through the calls
// the reader's Settings make.
func iPhoneWithKeys(t *testing.T, accounts ...string) *keyStore {
	t.Helper()
	ks := &keyStore{prefs: newFakePrefs(), secrets: newIPhoneKeychainTestStore()}
	for _, a := range accounts {
		if a == bibleKeyID {
			ks.setBibleAPIKey("iphone-key")
			ks.noteBibleKeyCleared(false)
		} else {
			ks.setAPIKey(a, "iphone-key")
		}
		if ks.apiKey(a) != "iphone-key" {
			t.Fatalf("control: the iPhone did not keep the %s key", a)
		}
	}
	return ks
}

// restoredOntoAnotherIPhone is another iPhone restored from a backup of one
// that saved a key for each account: its settings a copy of the first's, and
// its Keychain what that backup brings.
func restoredOntoAnotherIPhone(t *testing.T, b iPhoneBackup, accounts ...string) (*keyStore, *iPhoneKeychainTestStore) {
	t.Helper()
	first := iPhoneWithKeys(t, accounts...)
	keychain := first.secrets.(*iPhoneKeychainTestStore).onAnotherDevice(b)
	return &keyStore{prefs: copyOfPrefs(first.prefs), secrets: keychain}, keychain
}

// TestAKeyNotRestoredIsSaidOnTheIPhone: an iPhone restored from an
// unencrypted computer backup of another says the line under both empty key
// fields, the API.Bible key's and the AI key's, in place of their hints. Only
// there: an encrypted backup carries the keys, and the same iPhone with the
// app deleted and installed again starts with empty settings beside a
// Keychain that kept the key.
func TestAKeyNotRestoredIsSaidOnTheIPhone(t *testing.T) {
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

	has := func(texts []string, want string) bool {
		for _, s := range texts {
			if s == want {
				return true
			}
		}
		return false
	}
	bibleField := func(keys *keyStore) []string {
		st := sampleState()
		st.theme = th
		st.aiKeys = keys
		rows, _ := bibleKeySection(st, st.pal(), nil)
		return treeTexts(rows)
	}
	aiField := func(keys *keyStore) []string {
		keys.setActiveProvider(info.ID)
		keys.setAIEnabled(true)
		state := deferredTestState(t, app)
		state.aiKeys = keys
		showAISettings(state)
		texts := treeTexts(settingsPopup(t, state))
		state.window.Canvas().Overlays().Top().Hide()
		return texts
	}

	phone, _ := restoredOntoAnotherIPhone(t, unencryptedBackup, info.ID, bibleKeyID)
	if got := bibleField(phone); !has(got, approvedKeyNotRestoredLine) {
		t.Errorf("an iPhone restored from %s: the empty API.Bible field says %q, want the line %q", unencryptedBackup, got, approvedKeyNotRestoredLine)
	}
	if got := aiField(phone); !has(got, approvedKeyNotRestoredLine) || has(got, info.KeyHint) {
		t.Errorf("an iPhone restored from %s: the empty %s field says %q, want the line %q in place of the hint", unencryptedBackup, info.Name, got, approvedKeyNotRestoredLine)
	}

	carried, _ := restoredOntoAnotherIPhone(t, encryptedBackup, info.ID, bibleKeyID)
	for _, a := range []string{info.ID, bibleKeyID} {
		if carried.apiKey(a) != "iphone-key" || carried.keyLeftOnOldDevice(a) {
			t.Errorf("an iPhone restored from %s: the %s key reads %q and is said to be left behind %v; want the key and no line",
				encryptedBackup, a, carried.apiKey(a), carried.keyLeftOnOldDevice(a))
		}
	}
	if got := bibleField(carried); has(got, approvedKeyNotRestoredLine) {
		t.Errorf("an iPhone restored from %s, which carried the key, says it was not restored: %q", encryptedBackup, got)
	}
	if got := aiField(carried); has(got, approvedKeyNotRestoredLine) {
		t.Errorf("an iPhone restored from %s, which carried the key, says it was not restored: %q", encryptedBackup, got)
	}

	// The same iPhone, the app deleted and installed again: its settings go
	// with its container, its Keychain keeps the key, and the next launch's
	// sweep finds the key and marks it saved.
	first := iPhoneWithKeys(t, info.ID, bibleKeyID)
	reinstalled := &keyStore{prefs: newFakePrefs(), secrets: first.secrets}
	reinstalled.migrateAllKeys()
	for _, a := range []string{info.ID, bibleKeyID} {
		if reinstalled.apiKey(a) != "iphone-key" || reinstalled.keyLeftOnOldDevice(a) {
			t.Errorf("the iPhone reinstalled: the %s key reads %q and is said to be left behind %v; want the key and no line",
				a, reinstalled.apiKey(a), reinstalled.keyLeftOnOldDevice(a))
		}
	}

	// The key pasted again takes the line away.
	phone.setBibleAPIKey("iphone-key")
	if got := bibleField(phone); has(got, approvedKeyNotRestoredLine) {
		t.Errorf("with the key pasted again the field still says it was not restored: %q", got)
	}
}

// TestAKeyNotRestoredIsSaidInThePickerOnTheIPhone: an iPhone restored from an
// unencrypted computer backup of one that read the NKJV with the reader's
// own key opens the WEB, keeps the NKJV remembered, since the reader did not
// give it up, and the picker says why, as Android's does.
func TestAKeyNotRestoredIsSaidInThePickerOnTheIPhone(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv not registered")
	}
	t.Setenv("BIBLETEXT_CACHE_PATH", t.TempDir()+"/bibletext-cache.json")
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-provider-id")

	for _, b := range []iPhoneBackup{unencryptedBackup, encryptedBackup} {
		t.Run(b.String(), func(t *testing.T) {
			phone, _ := restoredOntoAnotherIPhone(t, b, bibleKeyID)
			prev := sharedKeys
			sharedKeys = func() *keyStore { return phone }
			t.Cleanup(func() { sharedKeys = prev })
			prevLoad := loadVersionForRestore
			loadVersionForRestore = func(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
				if b == encryptedBackup {
					return fullValidBible(), modeReal, nil
				}
				return nil, modeReal, errors.New("offline")
			}
			t.Cleanup(func() { loadVersionForRestore = prevLoad })

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

			if b == encryptedBackup {
				if live.CurrentVersion != "nkjv" || strings.Contains(fullPendingNotice(live), "Your saved API.Bible key isn't available") {
					t.Fatalf("the key carried, the launch shows %s and the picker reads %q; want the NKJV and no line",
						live.CurrentVersion, fullPendingNotice(live))
				}
				return
			}
			if live.CurrentVersion != defaultVersionID {
				t.Fatalf("control: with no key on the iPhone the launch must show the default; shows %s", live.CurrentVersion)
			}
			if n := fullPendingNotice(live); n != approvedKeyNotRestoredPicker {
				t.Fatalf("the picker reads\n  %q\nwant\n  %q", n, approvedKeyNotRestoredPicker)
			}
			if saved := snapshotReadingState(live, 0, 0, 0, 0, 0).Version; saved != "nkjv" {
				t.Fatalf("the next save writes %q: the reader did not give the NKJV up", saved)
			}
		})
	}
}

// TestTheNotRestoredLineIsDismissedOnTheIPhone: the ✕ is recorded in the
// Keychain as an item kept ThisDeviceOnly, so the line stays away when the
// sheet opens again and at the next launch, and comes back on another device
// restored from this one's backup, even an encrypted one, which carries the
// settings and every item that travels but not the record.
func TestTheNotRestoredLineIsDismissedOnTheIPhone(t *testing.T) {
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

	phone, keychain := restoredOntoAnotherIPhone(t, unencryptedBackup, bibleKeyID)
	rows := field(phone)
	x := lineClose(t, rows)
	if x.text != approvedKeyNotRestoredLine || !x.showing {
		t.Fatalf("the restored iPhone's field reads %q with the ✕ showing %v; want the line %q and its ✕", x.text, x.showing, approvedKeyNotRestoredLine)
	}
	travelling := keychain.travelling
	x.btn.OnTapped()
	rec, ok := keychain.items[bibleKeyID+keyNotRestoredDismissedSuffix]
	if !ok || !rec.thisDeviceOnly || keychain.travelling != travelling {
		t.Fatalf("the ✕ is recorded %v, ThisDeviceOnly %v, with %d writes of items that travel; want one item kept ThisDeviceOnly and none that travels",
			ok, rec.thisDeviceOnly, keychain.travelling-travelling)
	}
	if strings.Contains(rec.value, "iphone-key") {
		t.Fatal("the record of the ✕ holds the key")
	}
	if x := lineClose(t, rows); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("after the ✕ the field reads %q with the ✕ showing %v", x.text, x.showing)
	}
	if x := lineClose(t, field(phone)); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("the sheet opened again says the dismissed line: %q, ✕ showing %v", x.text, x.showing)
	}
	relaunched := &keyStore{prefs: phone.prefs, secrets: keychain}
	if x := lineClose(t, field(relaunched)); x.text == approvedKeyNotRestoredLine || x.showing {
		t.Fatalf("the next launch says the dismissed line again: %q, ✕ showing %v", x.text, x.showing)
	}

	for _, b := range []iPhoneBackup{encryptedBackup, unencryptedBackup} {
		another := &keyStore{prefs: copyOfPrefs(phone.prefs), secrets: keychain.onAnotherDevice(b)}
		if x := lineClose(t, field(another)); x.text != approvedKeyNotRestoredLine || !x.showing {
			t.Fatalf("another iPhone restored from %s of this one reads %q with the ✕ showing %v; want the line back",
				b, x.text, x.showing)
		}
	}
}

// A backup of the same Apple device retains ThisDeviceOnly items. The
// dismissal is scoped to that device and account, rather than to every
// restore operation. This stand-in checks the app's response to those items;
// it does not exercise Apple's backup transport.
func TestTheNotRestoredLineStaysDismissedAfterSameDeviceRestore(t *testing.T) {
	phone, keychain := restoredOntoAnotherIPhone(t, unencryptedBackup, bibleKeyID, defaultProviderID)
	phone.dismissKeyNotRestored(bibleKeyID)
	backedUp := newIPhoneKeychainTestStore()
	for account, item := range keychain.items {
		backedUp.items[account] = item
	}
	restored := &keyStore{prefs: copyOfPrefs(phone.prefs), secrets: backedUp}
	if !restored.keyLeftOnOldDevice(bibleKeyID) {
		t.Fatal("control: the saved API.Bible key must still be absent")
	}
	if restored.keyNotRestoredShown(bibleKeyID) {
		t.Fatal("a same-device restore discarded the API.Bible field's dismissal")
	}
	if !restored.keyNotRestoredShown(defaultProviderID) {
		t.Fatal("the API.Bible dismissal hid another account's missing-key line")
	}
}

// Android and the Apple Keychain are the two separate stores whose absence
// can be checked against a saved mark. Apple uses the same answer on iOS and
// the signed Mac build. The question has one method, with no older alias.
// Android keeps the dismissal where it keeps its device-bound keys.
func TestOnlyTheStoresARestoreCanLeaveBehindSaySo(t *testing.T) {
	src := parsePackageSource(t)
	if ds := src.methods["keysStayOnDevice"]; len(ds) > 0 {
		t.Errorf("keysStayOnDevice is declared on %s beside restoreCanLeaveKeys; the rule is asked by one method", receiverName(ds[0]))
	}
	recvs := map[string]*ast.FuncDecl{}
	for _, d := range src.methods["restoreCanLeaveKeys"] {
		recvs[receiverName(d)] = d
	}
	var names []string
	for n := range recvs {
		names = append(names, n)
	}
	sort.Strings(names)
	if strings.Join(names, ", ") != "*blobSecretStore, appleKeychainStore" {
		t.Fatalf("restoreCanLeaveKeys is declared on [%s]; want Android's store, *blobSecretStore, and the Apple Keychain, appleKeychainStore",
			strings.Join(names, ", "))
	}
	if got := returnedExpr(t, recvs["appleKeychainStore"]); got != "true" {
		t.Errorf("the Keychain's restoreCanLeaveKeys returns %s; want true on both Apple platforms", got)
	}

	var store secretStore = &blobSecretStore{}
	left, ok := store.(interface {
		restoreCanLeaveKeys() bool
		writeThisDeviceOnly(account, value string) bool
	})
	if !ok || !left.restoreCanLeaveKeys() {
		t.Fatal("Android's store does not say a restore can leave its keys behind")
	}
	blob, _ := newTestBlobStore(t)
	left = any(blob).(interface {
		restoreCanLeaveKeys() bool
		writeThisDeviceOnly(account, value string) bool
	})
	if !left.writeThisDeviceOnly(bibleKeyID+keyNotRestoredDismissedSuffix, "1") {
		t.Fatal("Android's store refused the record of the ✕")
	}
	if v, found, ok := blob.Read(bibleKeyID + keyNotRestoredDismissedSuffix); !ok || !found || v != "1" {
		t.Fatalf("Android's store reads the record back as (%q, %v, %v)", v, found, ok)
	}
}

// TestTheKeychainKeepsOnlyTheRecordThisDeviceOnly reads the Keychain adapter,
// which only an Apple build compiles and only an iOS build runs with this
// answer: Write keeps a key AfterFirstUnlock, which travels with an encrypted
// backup and to a new device, and writeThisDeviceOnly keeps the record of the
// ✕ AfterFirstUnlockThisDeviceOnly, which Apple never migrates to another
// device. The flag reaches the C write as 1 for the record and 0 for a key.
// Mutation: either write given the other's class, a third class, or the flag
// lost on its way from Go to C.
func TestTheKeychainKeepsOnlyTheRecordThisDeviceOnly(t *testing.T) {
	text := readRepoFile(t, "ai_secure_store_darwin.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "ai_secure_store_darwin.go", text, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]*ast.FuncDecl{}
	var write *ast.FuncDecl
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		switch {
		case !ok:
		case fn.Recv != nil && receiverName(fn) == "appleKeychainStore":
			methods[fn.Name.Name] = fn
		case fn.Recv == nil && fn.Name.Name == "keychainWrite":
			write = fn
		}
	}

	// The Go side's flag, as the C write receives it: 0 unless the record is
	// written, 1 for the record.
	if write == nil {
		t.Fatal("ai_secure_store_darwin.go has no keychainWrite")
	}
	var body strings.Builder
	if err := format.Node(&body, fset, write.Body); err != nil {
		t.Fatal(err)
	}
	const wantFlag = "only := C.int(0) if thisDeviceOnly { only = 1 } return C.btAIKeychainWrite(ca, cv, only) == 1"
	if got := strings.Join(strings.Fields(body.String()), " "); !strings.Contains(got, wantFlag) {
		t.Errorf("keychainWrite reads\n  %s\nwant it to pass the C write\n  %s", got, wantFlag)
	}
	for name, want := range map[string]string{"Write": "false", "writeThisDeviceOnly": "true"} {
		fn := methods[name]
		if fn == nil {
			t.Errorf("appleKeychainStore has no %s", name)
			continue
		}
		if got := returnedExpr(t, fn); got != "keychainWrite(account, value, "+want+")" {
			t.Errorf("appleKeychainStore.%s returns %s; want keychainWrite(account, value, %s)", name, got, want)
		}
	}

	// The C write, comments set aside: one class chosen by the flag, used by
	// both the update and the add.
	var code []string
	for _, line := range strings.Split(text, "\n") {
		if l := strings.TrimSpace(line); !strings.HasPrefix(l, "//") {
			code = append(code, l)
		}
	}
	c := strings.Join(code, " ")
	for _, want := range []string{
		"static int btAIKeychainWrite(const char *accountUTF8, const char *valueUTF8, int thisDeviceOnly)",
		"CFTypeRef accessible = thisDeviceOnly ? kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly : kSecAttrAccessibleAfterFirstUnlock;",
		"(__bridge id)kSecAttrAccessible: (__bridge id)accessible}",
		"item[(__bridge id)kSecAttrAccessible] = (__bridge id)accessible;",
		"return C.btAIKeychainWrite(ca, cv, only) == 1",
	} {
		if !strings.Contains(strings.Join(strings.Fields(c), " "), want) {
			t.Errorf("ai_secure_store_darwin.go does not have %q", want)
		}
	}
	classes := regexp.MustCompile(`kSecAttrAccessible[A-Z]\w*`).FindAllString(c, -1)
	sort.Strings(classes)
	if got := strings.Join(classes, " "); got != "kSecAttrAccessibleAfterFirstUnlock kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly" {
		t.Errorf("the Keychain adapter names the classes %q; want AfterFirstUnlock for a key and AfterFirstUnlockThisDeviceOnly for the record, once each", got)
	}
}

// receiverName is a method's receiver type, with its star.
func receiverName(fn *ast.FuncDecl) string {
	switch r := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := r.X.(*ast.Ident); ok {
			return "*" + id.Name
		}
	case *ast.Ident:
		return r.Name
	}
	return ""
}

// returnedExpr is the expression a one-statement function returns, printed.
func returnedExpr(t *testing.T, fn *ast.FuncDecl) string {
	t.Helper()
	if fn == nil || fn.Body == nil || len(fn.Body.List) != 1 {
		return "(not a single return)"
	}
	ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "(not a single return)"
	}
	return exprString(ret.Results[0])
}

// exprString prints the few expression forms returnedExpr meets.
func exprString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.BinaryExpr:
		return exprString(e.X) + " " + e.Op.String() + " " + exprString(e.Y)
	case *ast.CallExpr:
		var args []string
		for _, a := range e.Args {
			args = append(args, exprString(a))
		}
		return exprString(e.Fun) + "(" + strings.Join(args, ", ") + ")"
	}
	return "(?)"
}
