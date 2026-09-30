//go:build !ios && !android

package bibletext

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// THE WINDOWS SHARE SHEET IS HANDED THE CITATION, THE MESSAGE AND THE LINK,
// read from the message every platform shares: the citation as the title for
// every verb, the link only for a link or note share. Mutations: the link
// taken for a citation share (its last line is the citation), the title
// read from the wrong line of a note share.
func TestTheSharePartsAreReadFromTheMessage(t *testing.T) {
	link := ShareLinkURL(defaultVersionID, "John", 3, 16, 16)
	if !strings.HasPrefix(link, shareLinkBase+"/") {
		t.Fatalf("control: the fixture link %q is not a site link", link)
	}
	const title = "John 3:16 (World English Bible)"
	for _, c := range []struct {
		name, msg   string
		title, link string
	}{
		{"citation", composeShareText("“For God so loved the world.”", "John 3:16", "World English Bible"), title, ""},
		{"verse of the day, a passage", composeShareText("“For God so loved the world.”\n“For God didn’t send his Son.”", "John 3:16-17", "World English Bible"),
			"John 3:16-17 (World English Bible)", ""},
		{"link", title + "\n" + link, title, link},
		{"note", "read this with me\n\n" + title + "\n" + link, title, link},
		{"a note of several lines", "one\ntwo\n\n" + title + "\n" + link, title, link},
		{"nothing to cite", "", ProductName(), ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := sharePartsFor(c.msg)
			if p != (shareParts{title: c.title, text: c.msg, link: c.link}) {
				t.Errorf("sharePartsFor = %+v, want title %q, the message as text, link %q", p, c.title, c.link)
			}
		})
	}
}

// A SHARED PICTURE IS NAMED FOR THE READER, the name the Windows Share sheet
// shows in place of a title and the one Downloads keeps.
func TestTheSharedPictureIsNamedForTheReader(t *testing.T) {
	at := time.Date(2026, 9, 30, 14, 2, 11, 0, time.Local)
	if got, want := shareImageName(at), ProductName()+" verse 2026-09-30 14.02.11.png"; got != want {
		t.Errorf("shareImageName = %q, want %q", got, want)
	}
}

// THE COPY THE WINDOWS SHARE SHEET IS GIVEN outlives the render: it is the
// card's bytes under the reader's name, in a folder of the app's own under
// the render directory, and copies over a day old go as the next is made,
// while a newer one stays. Mutations: the age test inverted (the fresh copy
// goes), the old copies never cleared.
func TestTheSharedPictureIsCopiedUnderItsName(t *testing.T) {
	dir := t.TempDir()
	prev := imageRenderDir
	imageRenderDir = func() string { return dir }
	t.Cleanup(func() { imageRenderDir = prev })

	card := filepath.Join(dir, "bibletext-verse-0.png")
	if err := os.WriteFile(card, []byte("card bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 14, 2, 11, 0, time.Local)
	first, err := copyShareImage(card, now.Add(-48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(first, now.Add(-48*time.Hour), now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	recent, err := copyShareImage(card, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := copyShareImage(card, now)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) || filepath.Base(got) != shareImageName(now) ||
		filepath.Dir(got) != filepath.Join(dir, strings.ToLower(ProductName())+"-share") {
		t.Errorf("the copy is %q, want %q in the app's share folder under %q", got, shareImageName(now), dir)
	}
	if b, err := os.ReadFile(got); err != nil || string(b) != "card bytes" {
		t.Errorf("the copy holds %q (%v), want the card's bytes", b, err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("a copy two days old is still there (%v)", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("a copy an hour old was cleared: %v", err)
	}
	if _, err := os.Stat(card); err != nil {
		t.Errorf("the render itself must stay: %v", err)
	}
}

// winrtIID derives a parameterized interface's id from its type signature:
// SHA-1 over the Windows Runtime's pinterface namespace id, in network byte
// order, then the signature in UTF-8; the first 16 bytes, with the version
// set to 5 and the RFC 4122 variant.
func winrtIID(signature string) string {
	namespace := []byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}
	sum := sha1.Sum(append(namespace, signature...))
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}

// THE PARAMETERIZED INTERFACE IDS ARE THE HASHES OF THEIR SIGNATURES, as
// Windows derives them. A mistyped id is refused with E_NOINTERFACE and every
// Windows share falls back; this finds it on any machine. The control is the
// SDK's own id for IVectorView<IStorageItem>, which the share does not use,
// derived by the same code. Mutations: one digit of any id in
// share_winrt_iids.go changed, an argument's id included.
func TestTheWindowsShareInterfaceIdsAreDerivedFromTheirSignatures(t *testing.T) {
	control := winrtPinterface("BBE1FA4C-B0E3-4583-BAEF-1F1B2E483E56", winrtInterface(iidStrIStorageItem))
	if got := winrtIID(control); got != "85575A41-06CB-58D0-B98A-7C8F06E6E9D7" {
		t.Fatalf("control: the derivation gives %s for IVectorView<IStorageItem>, whose SDK id is 85575A41-06CB-58D0-B98A-7C8F06E6E9D7", got)
	}
	if len(winrtParameterized) != 5 {
		t.Errorf("%d parameterized ids, want the five the share uses", len(winrtParameterized))
	}
	for _, p := range winrtParameterized {
		if got := winrtIID(p.signature); got != p.iid {
			t.Errorf("%s: its signature %s hashes to %s, not %s", p.name, p.signature, got, p.iid)
		}
	}
	well := regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)
	for _, id := range []string{iidStrIUnknown, iidStrIInspectable, iidStrIAgileObject, iidStrIMarshal,
		iidStrIActivationFactory, iidStrIDataTransferManagerInterop, iidStrIDataTransferManager,
		iidStrIDataRequestedEventArgs, iidStrIDataPackage, iidStrIDataPackage2, iidStrIUriRuntimeClassFactory,
		iidStrIStorageFileStatics, iidStrIStorageFile, iidStrIStorageItem, iidStrIAsyncInfo,
		piidTypedEventHandler, piidAsyncOperationCompletedHandler, piidIAsyncOperation, piidIIterable, piidIIterator} {
		if !well.MatchString(id) {
			t.Errorf("%q is not an interface id in the SDK's upper-case form", id)
		}
	}
}
