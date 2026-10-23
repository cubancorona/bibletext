package bibletext

// The permission ends with its key (ai_permission.go). A provider's
// permission holds for the key it was given with: any change of that key,
// cleared, replaced by another or gone after a restore that brought the
// preferences back without it, ends it, and the app asks again before the
// next request. The same key saved again keeps it.

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// receivedStays fails if the stand-in has received other than n requests,
// after giving one that was started time to arrive.
func (s *providerStandIn) receivedStays(t *testing.T, n int, when string) {
	t.Helper()
	time.Sleep(300 * time.Millisecond)
	if got := len(s.received()); got != n {
		t.Fatalf("%s: the provider has received %d request(s), want %d", when, got, n)
	}
}

// A key replaced by another asks again before the next request, and sends
// nothing until Allow; the first key put back asks again too, since the
// change ended the permission. Control: the same key saved again, as the
// Settings field saves on every edit, asks nothing.
func TestAReplacedKeyAsksAgainBeforeTheNextRequest(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)
	st, w := claudeReaderState(t)
	cnv := w.Canvas()
	const claude = "Claude (Anthropic)"

	dispatchAIAction(st, aiActionExplain, refrain, verseTwo)
	test.Tap(findTreeButton(permissionQuestion(t, cnv, claude), "Allow"))
	landed(t, sent, "the request allowed with the first key")
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))

	st.aiKeys.setAPIKey(providerAnthropic, "test-key")
	dispatchAIAction(st, aiActionContext, refrain, verseTwo)
	landed(t, sent, "control: the request after the same key was saved again, with no question")
	panel := cnv.Overlays().Top()
	if treeHasText(panel, approvedPermissionTitle(claude)) || !treeHasText(panel, "Synthetic answer.") {
		t.Errorf("control: the same key saved again asked again or showed no answer; texts %v", treeTexts(panel))
	}
	test.Tap(findTreeButton(panel, "Close"))

	st.aiKeys.setAPIKey(providerAnthropic, "another-test-key")
	dispatchAIAction(st, aiActionTranslation, refrain, verseTwo)
	q := permissionQuestion(t, cnv, claude)
	s.receivedStays(t, 2, "with the key replaced and the question open")
	test.Tap(findTreeButton(q, "Not now"))
	s.receivedStays(t, 2, "after Not now with the key replaced")
	assertNotSentState(t, cnv.Overlays().Top(), "the panel after Not now with the key replaced")
	if aiPermitted(st.aiKeys, providerAnthropic) {
		t.Error("the replaced key holds a permission the reader did not give it")
	}
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))

	st.aiKeys.setAPIKey(providerAnthropic, "test-key")
	if aiPermitted(st.aiKeys, providerAnthropic) {
		t.Error("the first key put back holds its old permission without asking")
	}
	dispatchAIAction(st, aiActionTranslation, refrain, verseTwo)
	q = permissionQuestion(t, cnv, claude)
	s.receivedStays(t, 2, "with the first key put back and the question open")
	test.Tap(findTreeButton(q, "Allow"))
	landed(t, sent, "the request Allow sends with the first key put back")
	if n := len(s.received()); n != 3 {
		t.Errorf("the provider received %d requests, want 3", n)
	}
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))
}

// A key changed while the question is open: Allow records the permission
// for the key the question was asked about, not the key in place at the
// tap, so the request, which would go with the other key, is not sent, and
// the question is asked again for that key; Allow there sends it once.
// Control: the same steps with the key unchanged send at the first Allow
// (TestAReplacedKeyAsksAgainBeforeTheNextRequest).
func TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)
	st, w := claudeReaderState(t)
	cnv := w.Canvas()
	const claude = "Claude (Anthropic)"

	dispatchAIAction(st, aiActionExplain, refrain, verseTwo)
	q := permissionQuestion(t, cnv, claude)
	st.aiKeys.setAPIKey(providerAnthropic, "another-test-key")
	test.Tap(findTreeButton(q, "Allow"))
	s.receivedStays(t, 0, "after Allow with the key changed while the question was open")
	if got, want := st.aiKeys.prefs.String(prefAIAllowedPrefix+providerAnthropic), aiAllowedRecord(providerAnthropic, "test-key"); got != want {
		t.Errorf("Allow recorded %q, want the record of the key the question was asked about", got)
	}
	if aiPermitted(st.aiKeys, providerAnthropic) {
		t.Error("the key put in while the question was open holds the permission the reader gave the key before it")
	}

	q = permissionQuestion(t, cnv, claude)
	s.receivedStays(t, 0, "with the question asked again for the changed key")
	test.Tap(findTreeButton(q, "Allow"))
	landed(t, sent, "the request Allow sends for the changed key")
	if n := len(s.received()); n != 1 {
		t.Errorf("the provider received %d requests, want 1", n)
	}
	if got, want := st.aiKeys.prefs.String(prefAIAllowedPrefix+providerAnthropic), aiAllowedRecord(providerAnthropic, "another-test-key"); got != want {
		t.Errorf("the second Allow recorded %q, want the record of the changed key", got)
	}
	test.Tap(findTreeButton(cnv.Overlays().Top(), "Close"))
}

// The record is checked against the key each request would use, however
// the key changed: a key the credential store holds that is not the one the
// reader allowed, here written there without the app, is refused at both
// places a request leaves the device. Control: the key that was allowed,
// back in the store, sends from both.
func TestTheRequestsRefuseAKeyThePermissionWasNotGivenWith(t *testing.T) {
	s := newProviderStandIn(t)
	sent := s.route(t)
	secrets := &fakeSecrets{m: map[string]string{}}
	st := refrainChapterState()
	st.aiKeys = &keyStore{prefs: newFakePrefs(), secrets: secrets}
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	grantAIPermission(st.aiKeys, defaultProviderID)

	secrets.m[defaultProviderID] = "another-test-key"
	if _, pending := aiPermissionPending(st); !pending {
		t.Error("the study panel and Find would not ask for a key the reader did not allow")
	}
	_, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", verseTwo)
	if friendlyAIError(err) != notSentMessage {
		t.Errorf("a study request with another key returned %v (%q)", err, friendlyAIError(err))
	}
	_, err = runAISearch(context.Background(), st, "mercy")
	if friendlyAIError(err) != notSentMessage {
		t.Errorf("a Find with another key returned %v (%q)", err, friendlyAIError(err))
	}
	if got := s.received(); len(got) != 0 {
		t.Fatalf("%d request(s) reached the provider with a key the reader did not allow", len(got))
	}

	secrets.m[defaultProviderID] = "test-key"
	if _, pending := aiPermissionPending(st); pending {
		t.Error("control: the allowed key back in the store is asked about again")
	}
	if out, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", verseTwo); err != nil || out != "Synthetic answer." {
		t.Errorf("control: the study request with the allowed key returned (%q, %v)", out, err)
	}
	<-sent
	if _, err := runAISearch(context.Background(), st, "mercy"); err != nil {
		t.Errorf("control: the Find with the allowed key returned %v", err)
	}
	<-sent
}

// An Android restore brings the preferences back, the permission record
// among them, and not the key, which Android's credential store keeps under
// no_backup/ with a Keystore key that never leaves the phone
// (ai_secure_store_blob.go). The record is then no permission: nothing can be
// sent without a key, and the same key pasted again asks again before the
// next request.
func TestPreferencesRestoredWithoutTheKeyHoldNoPermission(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	s := newProviderStandIn(t)
	sent := s.route(t)

	oldStore, _ := newTestBlobStore(t)
	oldPrefs := newFakePrefs()
	old := &keyStore{prefs: oldPrefs, secrets: oldStore}
	old.setAPIKey(defaultProviderID, "test-key")
	grantAIPermission(old, defaultProviderID)
	if !aiPermitted(old, defaultProviderID) {
		t.Fatal("control: the old phone holds no permission to restore")
	}

	restoredPrefs := newFakePrefs()
	oldPrefs.mu.Lock()
	for k, v := range oldPrefs.m {
		restoredPrefs.m[k] = v
	}
	oldPrefs.mu.Unlock()
	if restoredPrefs.String(prefAIAllowedPrefix+defaultProviderID) == "" {
		t.Fatal("control: the restored preferences carry no permission record")
	}
	newStore, _ := newTestBlobStore(t)
	restored := &keyStore{prefs: restoredPrefs, secrets: newStore}
	if got := restored.apiKey(defaultProviderID); got != "" {
		t.Fatalf("control: the restored phone has a key %d characters long; the restore must leave it behind", len(got))
	}
	if aiPermitted(restored, defaultProviderID) {
		t.Error("the restored preferences hold a permission with no key")
	}

	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(900, 760))
	st := refrainChapterState()
	st.window = w
	st.aiKeys = restored
	_, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", verseTwo)
	if !isNoKeyError(err) {
		t.Errorf("a study request on the restored phone with no key returned %v", err)
	}

	restored.setAPIKey(defaultProviderID, "test-key")
	if aiPermitted(restored, defaultProviderID) {
		t.Error("the key pasted again on the restored phone holds the old phone's permission without asking")
	}
	gemini, _ := providerByID(defaultProviderID)
	dispatchAIAction(st, aiActionExplain, refrain, verseTwo)
	q := permissionQuestion(t, w.Canvas(), gemini.Name)
	s.receivedStays(t, 0, "with the key pasted again and the question open")
	test.Tap(findTreeButton(q, "Allow"))
	landed(t, sent, "the request Allow sends on the restored phone")
	if !aiPermitted(restored, defaultProviderID) {
		t.Error("Allow on the restored phone recorded no permission")
	}
	test.Tap(findTreeButton(w.Canvas().Overlays().Top(), "Close"))
}
