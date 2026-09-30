//go:build windows

package bibletext

// The Windows share against Windows itself, with no sheet ever shown: the
// Go objects answer COM as Windows calls them, the real DataPackage takes
// the share the sheet's request would give it, and every step of a share
// given a window that is never shown — the package handed over, the
// picture's file resolved — either reaches the point where the sheet would
// open or ends in exactly one fallback. ShowShareUIForWindow is never called
// (windowsShareStep). What only a person can see — the sheet itself, and
// what it does with the share — is docs/VISUAL_TESTS.md's V12.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var (
	procRoUninitialize            = combase.NewProc("RoUninitialize")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
	procCoTaskMemFree             = ole32.NewProc("CoTaskMemFree")
	testProcCreateWindowExW       = user32.NewProc("CreateWindowExW")
	testProcDestroyWindow         = user32.NewProc("DestroyWindow")
	testProcPeekMessageW          = user32.NewProc("PeekMessageW")
	testProcTranslateMessage      = user32.NewProc("TranslateMessage")
	testProcDispatchMessageW      = user32.NewProc("DispatchMessageW")

	iidIAsyncInfo = guidOf(iidStrIAsyncInfo)
	iidIDataPkg   = guidOf(iidStrIDataPackage)
)

const hrFail int32 = -0x7FFFBFFB // E_FAIL 0x80004005

// lockWinRTThread keeps the test on one thread, joined to a single-threaded
// apartment by ensureWinRT as the window's thread is, and puts both back
// when the test ends.
func lockWinRTThread(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	prev := winrt
	winrt.state, winrt.thread = 0, 0
	if err := ensureWinRT(); err != nil {
		winrt = prev
		runtime.UnlockOSThread()
		t.Fatalf("the test's thread cannot join a single-threaded apartment: %v", err)
	}
	t.Cleanup(func() {
		procRoUninitialize.Call()
		winrt = prev
		runtime.UnlockOSThread()
	})
}

// hstringValue copies an HSTRING's characters into Go.
func hstringValue(h hstring) string {
	if h == 0 {
		return ""
	}
	var n uint32
	p, _, _ := procWindowsGetStringRawBuffer.Call(uintptr(h), uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), p, uintptr(n)*2)
	return windows.UTF16ToString(buf)
}

// asRef is one of the Go objects as Windows holds it.
func asRef(o *comObject) *comRef { return (*comRef)(unsafe.Pointer(o)) }

// THE GO OBJECTS ANSWER COM AS WINDOWS ASKS: the collection answers for
// IUnknown, IInspectable, its own interface, IAgileObject and IMarshal,
// through the aggregated marshaler, and for nothing else, and counts its
// references back to one; it names its interface and its runtime class
// and claims base trust; a walk gives its one item once — First, HasCurrent,
// Current, MoveNext, and GetMany on a fresh iterator — with a reference the
// caller owns each time; and when the last reference goes, every object
// and the item's reference go with it. Mutations: IAgileObject not answered,
// MoveNext leaving the iterator on the item, the item's reference not taken
// for Current, the registry not emptied on the final Release.
func TestTheShareObjectsAnswerCOM(t *testing.T) {
	lockWinRTThread(t)
	base := comLiveCount()
	item := newComObject(&comObject{iid: iidIStorageItem}, func() *comVtbl { return &fileResolvedVtbl })
	items := newStorageItems(asRef(item)) // takes the item's one reference
	ref := asRef(items)

	for _, c := range []struct {
		name string
		iid  windows.GUID
		want int32
	}{
		{"IUnknown", iidIUnknown, hrOK},
		{"IInspectable", iidIInspectable, hrOK},
		{"IIterable<IStorageItem>", iidIIterableStorageItem, hrOK},
		{"IAgileObject", iidIAgileObject, hrOK},
		{"IMarshal", iidIMarshal, hrOK},
		{"IIterator<IStorageItem>", iidIIteratorStorageItem, hrNoInterface},
		{"an interface it does not have", guidOf("0F0E0D0C-0B0A-0908-0706-050403020100"), hrNoInterface},
	} {
		got, hr := ref.query(&c.iid)
		if hr != c.want {
			t.Errorf("QueryInterface(%s) = 0x%08X, want 0x%08X", c.name, uint32(hr), uint32(c.want))
		}
		if hr >= 0 {
			got.release()
		} else if got != nil {
			t.Errorf("QueryInterface(%s) failed and still handed out an interface", c.name)
		}
	}
	if n := items.refs.Load(); n != 1 {
		t.Errorf("with every interface let go the collection counts %d references, want 1", n)
	}

	var count uint32
	var list uintptr
	if hr := comCall(ref, 3, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&list))); hr != hrOK || count != 1 || list == 0 {
		t.Errorf("GetIids = 0x%08X with %d ids, want S_OK and one", uint32(hr), count)
	} else {
		var got windows.GUID
		procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&got)), list, unsafe.Sizeof(got))
		procCoTaskMemFree.Call(list)
		if got != iidIIterableStorageItem {
			t.Errorf("GetIids names %v, want IIterable<IStorageItem>", got)
		}
	}
	var name hstring
	if hr := comCall(ref, 4, uintptr(unsafe.Pointer(&name))); hr != hrOK ||
		hstringValue(name) != "Windows.Foundation.Collections.IIterable`1<Windows.Storage.IStorageItem>" {
		t.Errorf("GetRuntimeClassName = 0x%08X %q", uint32(hr), hstringValue(name))
	}
	deleteHString(name)
	level := int32(-1)
	if hr := comCall(ref, 5, uintptr(unsafe.Pointer(&level))); hr != hrOK || level != 0 {
		t.Errorf("GetTrustLevel = 0x%08X, level %d, want S_OK and BaseTrust", uint32(hr), level)
	}

	var it *comRef
	if hr := comCall(ref, 6, uintptr(unsafe.Pointer(&it))); hr != hrOK || it == nil {
		t.Fatalf("First = 0x%08X", uint32(hr))
	}
	var has uint8
	if hr := comCall(it, 7, uintptr(unsafe.Pointer(&has))); hr != hrOK || has != 1 {
		t.Errorf("HasCurrent on a fresh iterator = 0x%08X, %d, want S_OK and 1", uint32(hr), has)
	}
	before := item.refs.Load()
	var cur *comRef
	if hr := comCall(it, 6, uintptr(unsafe.Pointer(&cur))); hr != hrOK || cur != asRef(item) {
		t.Errorf("Current = 0x%08X, want S_OK and the item", uint32(hr))
	}
	if item.refs.Load() != before+1 {
		t.Errorf("Current handed out the item without a reference for the caller (%d then %d)", before, item.refs.Load())
	}
	cur.release()
	moved := uint8(9)
	if hr := comCall(it, 8, uintptr(unsafe.Pointer(&moved))); hr != hrOK || moved != 0 {
		t.Errorf("MoveNext = 0x%08X, %d, want S_OK and 0: there is no second item", uint32(hr), moved)
	}
	if hr := comCall(it, 7, uintptr(unsafe.Pointer(&has))); hr != hrOK || has != 0 {
		t.Errorf("HasCurrent past the item = %d, want 0", has)
	}
	cur = nil
	if hr := comCall(it, 6, uintptr(unsafe.Pointer(&cur))); hr != hrBounds || cur != nil {
		t.Errorf("Current past the item = 0x%08X, want E_BOUNDS and nothing", uint32(hr))
	}
	it.release()

	var it2 *comRef
	comCall(ref, 6, uintptr(unsafe.Pointer(&it2)))
	var buf [4]uintptr
	var n uint32
	if hr := comCall(it2, 9, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); hr != hrOK || n != 1 ||
		buf[0] != uintptr(unsafe.Pointer(item)) {
		t.Errorf("GetMany = 0x%08X with %d items, want S_OK and the item", uint32(hr), n)
	} else {
		item.release() // the reference GetMany handed out
	}
	if hr := comCall(it2, 9, 4, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); hr != hrOK || n != 0 {
		t.Errorf("GetMany past the item = 0x%08X with %d items, want S_OK and none", uint32(hr), n)
	}
	it2.release()

	items.release()
	if got := comLiveCount(); got != base {
		t.Errorf("%d objects are still live after the last reference went, want %d", got, base)
	}
}

// awaitAsync waits, pumping the thread's messages, until an asynchronous
// operation has finished, and returns its AsyncStatus.
func awaitAsync(t *testing.T, op *comRef) int32 {
	t.Helper()
	info, hr := op.query(&iidIAsyncInfo)
	if hr < 0 {
		t.Fatalf("the operation as IAsyncInfo: 0x%08X", uint32(hr))
	}
	defer info.release()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var status int32
		if hr := comCall(info, 7, uintptr(unsafe.Pointer(&status))); hr < 0 {
			t.Fatalf("IAsyncInfo.get_Status: 0x%08X", uint32(hr))
		}
		if status != 0 {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatal("the operation did not finish within 10 s")
		}
		pumpMessages()
		time.Sleep(5 * time.Millisecond)
	}
}

// pumpMessages dispatches what is waiting on the thread's queue, as the
// toolkit's event loop does.
func pumpMessages() {
	var m struct {
		hwnd    uintptr
		message uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
		private uint32
	}
	for {
		if r, _, _ := testProcPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1 /* PM_REMOVE */); r == 0 {
			return
		}
		testProcTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		testProcDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// writeTestCard writes a small PNG for the picture share.
func writeTestCard(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 5), uint8(y * 5), 150, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "bibletext-verse-0.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// resolveStorageItem resolves path as a StorageFile, as the picture share
// does, and returns it as an IStorageItem the caller owns.
func resolveStorageItem(t *testing.T, path string) *comRef {
	t.Helper()
	statics, err := activationFactory("Windows.Storage.StorageFile", &iidIStorageFileStatics)
	if err != nil {
		t.Fatal(err)
	}
	defer statics.release()
	hs, err := newHString(path)
	if err != nil {
		t.Fatal(err)
	}
	defer deleteHString(hs)
	var op *comRef
	if hr := comCall(statics, 6, uintptr(hs), uintptr(unsafe.Pointer(&op))); hr < 0 {
		t.Fatalf("GetFileFromPathAsync: 0x%08X", uint32(hr))
	}
	defer op.release()
	if st := awaitAsync(t, op); st != asyncStatusCompleted {
		t.Fatalf("the file resolved with AsyncStatus %d", st)
	}
	var file *comRef
	if hr := comCall(op, 8, uintptr(unsafe.Pointer(&file))); hr < 0 {
		t.Fatalf("GetResults: 0x%08X", uint32(hr))
	}
	defer file.release()
	item, hr := file.query(&iidIStorageItem)
	if hr < 0 {
		t.Fatalf("the file as IStorageItem: 0x%08X", uint32(hr))
	}
	var name hstring
	if hr := comCall(item, 11, uintptr(unsafe.Pointer(&name))); hr < 0 || hstringValue(name) != filepath.Base(path) {
		t.Errorf("IStorageItem.get_Name = 0x%08X %q, want %q", uint32(hr), hstringValue(name), filepath.Base(path))
	}
	deleteHString(name)
	return item
}

// readPackage is what a DataPackage holds: its formats and its title.
func readPackage(t *testing.T, pkg *comRef) (formats []string, title string) {
	t.Helper()
	var view *comRef
	if hr := comCall(pkg, 6, uintptr(unsafe.Pointer(&view))); hr < 0 {
		t.Fatalf("DataPackage.GetView: 0x%08X", uint32(hr))
	}
	defer view.release()
	var list *comRef
	if hr := comCall(view, 9, uintptr(unsafe.Pointer(&list))); hr < 0 {
		t.Fatalf("DataPackageView.get_AvailableFormats: 0x%08X", uint32(hr))
	}
	var size uint32
	comCall(list, 7, uintptr(unsafe.Pointer(&size)))
	for i := uint32(0); i < size; i++ {
		var h hstring
		if comCall(list, 6, uintptr(i), uintptr(unsafe.Pointer(&h))) >= 0 {
			formats = append(formats, hstringValue(h))
			deleteHString(h)
		}
	}
	list.release()
	var props *comRef
	if hr := comCall(view, 6, uintptr(unsafe.Pointer(&props))); hr < 0 {
		t.Fatalf("DataPackageView.get_Properties: 0x%08X", uint32(hr))
	}
	var h hstring
	comCall(props, 6, uintptr(unsafe.Pointer(&h)))
	title = hstringValue(h)
	deleteHString(h)
	props.release()
	return formats, title
}

// newDataPackage is a DataPackage of the app's own, as the sheet's request
// hands one over.
func newDataPackage(t *testing.T) *comRef {
	t.Helper()
	f, err := activationFactory("Windows.ApplicationModel.DataTransfer.DataPackage", &iidIActivationFactory)
	if err != nil {
		t.Fatal(err)
	}
	defer f.release()
	var insp *comRef
	if hr := comCall(f, 6, uintptr(unsafe.Pointer(&insp))); hr < 0 {
		t.Fatalf("ActivateInstance: 0x%08X", uint32(hr))
	}
	defer insp.release()
	pkg, hr := insp.query(&iidIDataPkg)
	if hr < 0 {
		t.Fatalf("the instance as IDataPackage: 0x%08X", uint32(hr))
	}
	return pkg
}

// THE DATAPACKAGE TAKES THE SHARE as the sheet's request fills it: the
// citation as its title, the text, the link as a Windows Uri whose address
// reads back whole, and the picture through the one-item collection,
// which reads back under the file's name. A package with a title and nothing
// else is refused. Mutations: the web link given to SetApplicationLink,
// the slot before SetWebLink (no UniformResourceLocatorW); the title left
// unset; a package with no content taken as filled.
func TestTheDataPackageTakesTheShare(t *testing.T) {
	lockWinRTThread(t)
	const link = "https://example.org/web/john/3/#v16"
	uri, err := newURI(link)
	if err != nil {
		t.Fatal(err)
	}
	defer uri.release()
	var abs hstring
	if hr := comCall(uri, 6, uintptr(unsafe.Pointer(&abs))); hr < 0 || hstringValue(abs) != link {
		t.Errorf("Uri.get_AbsoluteUri = 0x%08X %q, want %q", uint32(hr), hstringValue(abs), link)
	}
	deleteHString(abs)

	title, err := newHString("John 3:16 (Sample)")
	if err != nil {
		t.Fatal(err)
	}
	defer deleteHString(title)
	text, err := newHString("“For God so loved the world.”\n\n— John 3:16 (Sample)")
	if err != nil {
		t.Fatal(err)
	}
	defer deleteHString(text)

	pkg := newDataPackage(t)
	if err := fillSharePackage(pkg, &windowsShare{title: title, text: text, uri: uri}); err != nil {
		t.Errorf("the text and link share: %v", err)
	}
	formats, got := readPackage(t, pkg)
	pkg.release()
	if got != "John 3:16 (Sample)" || !slices.Contains(formats, "Text") || !slices.Contains(formats, "UniformResourceLocatorW") {
		t.Errorf("the package reads back %q with formats %v, want the citation, Text and UniformResourceLocatorW", got, formats)
	}

	card := writeTestCard(t)
	items := newStorageItems(resolveStorageItem(t, card))
	pkg = newDataPackage(t)
	if err := fillSharePackage(pkg, &windowsShare{title: title, items: items}); err != nil {
		t.Errorf("the picture share: %v", err)
	}
	formats, got = readPackage(t, pkg)
	pkg.release()
	items.release()
	if got != "John 3:16 (Sample)" || !slices.Contains(formats, "Shell IDList Array") {
		t.Errorf("the picture's package reads back %q with formats %v, want the citation and the file", got, formats)
	}

	pkg = newDataPackage(t)
	if err := fillSharePackage(pkg, &windowsShare{title: title}); err == nil {
		t.Error("a package with a title and nothing to share was taken as filled")
	}
	pkg.release()
}

// nativeShareTestWindow is a window whose native handle is a window the test
// made and never shows.
type nativeShareTestWindow struct {
	fyne.Window
	hwnd uintptr
}

func (w nativeShareTestWindow) RunNative(f func(any)) { f(driver.WindowsWindowContext{HWND: w.hwnd}) }

// hiddenShareWindow makes a window on the test's thread that is never shown,
// and makes it the app's window for the share.
func hiddenShareWindow(t *testing.T) uintptr {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("STATIC")
	name, _ := windows.UTF16PtrFromString("")
	hwnd, _, err := testProcCreateWindowExW.Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(name)),
		0, 0, 0, 320, 240, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatalf("CreateWindowExW: %v", err)
	}
	prev := activeAIState
	activeAIState = &AppState{window: nativeShareTestWindow{hwnd: hwnd}}
	t.Cleanup(func() {
		activeAIState = prev
		testProcDestroyWindow.Call(hwnd)
	})
	return hwnd
}

// shareStepsHarness records the steps a share makes, fails the one named
// in fail, and never lets the sheet open.
type shareStepsHarness struct {
	steps []string
	fail  string
}

func holdShareSteps(t *testing.T) *shareStepsHarness {
	h := &shareStepsHarness{}
	prev := windowsShareStep
	windowsShareStep = func(step string, call func() int32) int32 {
		h.steps = append(h.steps, step)
		switch step {
		case h.fail:
			return hrFail
		case stepShowSheet:
			return hrOK // the sheet is never shown on the machine running the test
		}
		return call()
	}
	t.Cleanup(func() { windowsShareStep = prev })
	return h
}

// startTestShare runs a share through windowsShareVerb, as the verbs do, with
// a fallback the harness counts, and returns its session.
func startTestShare(h *sessionHarness, start func(*shareSession)) *shareSession {
	var s *shareSession
	windowsShareVerb(func() { h.fallbacks++ }, func(sh *shareSession) { s = sh; start(sh) })
	return s
}

// shareEnd says how a test's share ended, for a failure message.
func shareEnd(s *shareSession) string {
	switch {
	case s == nil:
		return "no share started"
	case !s.ended():
		return "the share has not ended"
	}
	return fmt.Sprintf("the share ended with %v", s.result)
}

// skipWithoutShareWindow skips where this Windows refused a share, none of
// whose steps the test failed, before it reached the sheet: a server image
// may have no DataTransferManager, give none for a window, or take no
// handler on one. The share then falls back there, which is right, and
// nothing past that step can be tested; the skip names the step and its
// HRESULT. A handler refused as E_NOINTERFACE is not skipped: that is the
// answer to a wrong interface id, which is the app's defect.
func skipWithoutShareWindow(t *testing.T, s *shareSession, steps []string) {
	t.Helper()
	if !s.ended() || slices.Contains(steps, stepShowSheet) {
		return
	}
	var refused hresultError
	if !errors.As(s.result, &refused) {
		return
	}
	switch {
	case strings.HasPrefix(refused.step, "RoGetActivationFactory Windows.ApplicationModel.DataTransfer.DataTransferManager"),
		strings.HasPrefix(refused.step, "DataTransferManager as "),
		refused.step == stepGetForWindow,
		refused.step == stepAddHandler && refused.hr != hrNoInterface:
		t.Skipf("this Windows does not share from a window: %v", s.result)
	}
}

// A TEXT SHARE REACHES THE POINT WHERE THE SHEET OPENS, with the window's
// DataTransferManager and a handler registered, and nothing falls back
// until the sheet has not asked within the wait; then exactly one fallback,
// with the handler still registered for a sheet that asks late, and removed
// when the keep runs out. And each step that Windows refuses ends the share
// in exactly one fallback at once, naming that step. Mutations: a step's
// failure not checked (no fallback, the share goes on), the watchdog not
// armed before the sheet is asked to open, the sheet's opening not recorded
// (the handler goes as the fallback opens). What this cannot see is the
// handler's interface id:
// add_DataRequested takes a handler that answers the wrong one, as a
// control build on Windows 11 showed, so that id is held by its derivation
// (share_parts_test.go) and was proven by the sheet asking a probe's
// handler for the share.
func TestAWindowsTextShareFallsBackAtEveryStep(t *testing.T) {
	lockWinRTThread(t)
	hiddenShareWindow(t)
	h := newSessionHarness(t)
	steps := holdShareSteps(t)
	payload := windowsSharePayload{title: "John 3:16 (Sample)", text: "John 3:16 (Sample)\nhttps://example.org/x",
		link: "https://example.org/x"}

	s := startTestShare(h, func(sh *shareSession) { presentShare(sh, payload) })
	h.drain()
	skipWithoutShareWindow(t, s, steps.steps)
	if want := []string{stepGetForWindow, stepAddHandler, stepShowSheet}; !slices.Equal(steps.steps, want) {
		t.Fatalf("the share made the steps %v, want %v; it ended with %v", steps.steps, want, s.result)
	}
	if s.ended() || h.fallbacks != 0 {
		t.Fatalf("the share ended (%v) before the sheet could ask, with %d fallbacks", s.result, h.fallbacks)
	}
	h.fire()
	h.drain()
	var noAnswer shareNoAnswer
	if h.fallbacks != 1 || !errors.As(s.result, &noAnswer) {
		t.Errorf("a sheet that never asked: %d fallbacks, ended with %v; want one, for no answer", h.fallbacks, s.result)
	}
	if currentShareSession != s {
		t.Error("the share removed its handler as the fallback opened; a sheet that asks late would find none")
	}
	h.fire() // the keep runs out, and remove_DataRequested is called
	h.drain()
	if currentShareSession != nil || h.fallbacks != 1 {
		t.Errorf("after the keep: still held %v, %d fallbacks; want let go, and the one fallback", currentShareSession == s, h.fallbacks)
	}

	// Not subtests: a subtest runs on a goroutine of its own, off the
	// window's thread, where a share rightly never starts.
	for _, step := range []string{stepGetForWindow, stepAddHandler, stepShowSheet} {
		h.fallbacks, steps.steps, steps.fail = 0, nil, step
		s := startTestShare(h, func(sh *shareSession) { presentShare(sh, payload) })
		h.drain()
		var refused hresultError
		if s == nil || h.fallbacks != 1 || !errors.As(s.result, &refused) || refused.step != step {
			t.Errorf("%s refused: %d fallbacks, %s; want one fallback, for %s", step, h.fallbacks, shareEnd(s), step)
		}
	}
}

// A PICTURE SHARE RESOLVES ITS FILE THROUGH THE COMPLETION HANDLER — the
// handler's interface is accepted, it hears the file complete, possibly on
// another thread, and the file becomes the collection — and reaches the
// point where the sheet opens under the reader's file name; a card that is
// not there, a resolve Windows refuses and a handler it refuses each end in
// exactly one fallback. Mutations: the completion handler's interface id
// mistyped (Windows refuses the handler with E_NOINTERFACE, as a control
// build on Windows 11 showed), the status check inverted.
func TestAWindowsPictureShareResolvesItsFileFirst(t *testing.T) {
	lockWinRTThread(t)
	hiddenShareWindow(t)
	renders := t.TempDir()
	prevDir := imageRenderDir
	imageRenderDir = func() string { return renders }
	t.Cleanup(func() { imageRenderDir = prevDir })
	h := newSessionHarness(t)
	steps := holdShareSteps(t)
	card := writeTestCard(t)

	pic := takeSharedImage(card, time.Now())
	s := startTestShare(h, func(sh *shareSession) { shareImageFile(sh, pic) })
	deadline := time.Now().Add(10 * time.Second)
	for !s.ended() && !slices.Contains(steps.steps, stepShowSheet) {
		if time.Now().After(deadline) {
			t.Fatalf("the picture share reached %v and no further within 10 s", steps.steps)
		}
		pumpMessages()
		h.drain()
		time.Sleep(5 * time.Millisecond)
	}
	h.drain()
	skipWithoutShareWindow(t, s, steps.steps)
	if want := []string{stepResolveFile, stepFileHandler, stepGetForWindow, stepAddHandler, stepShowSheet}; !slices.Equal(steps.steps, want) {
		t.Fatalf("the picture share made the steps %v, want %v; it ended with %v", steps.steps, want, s.result)
	}
	if s.ended() || h.fallbacks != 0 {
		t.Fatalf("the picture share ended (%v) before the sheet could ask, with %d fallbacks", s.result, h.fallbacks)
	}
	entries, _ := os.ReadDir(filepath.Join(renders, strings.ToLower(ProductName())+"-share"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), ProductName()+" verse ") {
		t.Errorf("the sheet was handed %v, want one copy under the reader's name", entries)
	}
	h.fire()
	h.drain()
	if h.fallbacks != 1 {
		t.Errorf("a sheet that never asked for the picture: %d fallbacks, want one", h.fallbacks)
	}

	// Not subtests, for the window's thread (above).
	for _, c := range []struct{ name, card, fail string }{
		{"a card that is not there", filepath.Join(t.TempDir(), "gone.png"), ""},
		{stepResolveFile, card, stepResolveFile},
		{stepFileHandler, card, stepFileHandler},
	} {
		h.fallbacks, steps.steps, steps.fail = 0, nil, c.fail
		pic := takeSharedImage(c.card, time.Now())
		s := startTestShare(h, func(sh *shareSession) { shareImageFile(sh, pic) })
		h.drain()
		if s == nil || h.fallbacks != 1 || !s.ended() {
			t.Errorf("%s: %d fallbacks, %s; want the share ended in one", c.name, h.fallbacks, shareEnd(s))
			continue
		}
		var refused hresultError
		if c.fail != "" && (!errors.As(s.result, &refused) || refused.step != c.fail) {
			t.Errorf("%s: the share ended with %v, want %s refused", c.name, s.result, c.fail)
		}
	}
}

// WITH NO NATIVE WINDOW THE VERBS END IN THE IN-APP SHEET, at once and with
// the text on the clipboard: the test driver's window has no HWND, as no
// window would that the sheet could open for. Mutation: the missing window
// taken for a failure that needs no fallback.
func TestAWindowsShareWithNoWindowEndsInTheInAppSheet(t *testing.T) {
	h := newShareSheetHarness(t)
	prevActive := activeAIState
	activeAIState = h.st
	t.Cleanup(func() { activeAIState = prevActive })
	msg := h.expectedCitationShare()
	nativeShareText(msg)
	p := h.sheet()
	if !sheetHas(p, shareSheetHeading) {
		t.Errorf("the sheet reads %v, want %q", sheetTexts(p), shareSheetHeading)
	}
	if got := h.clipboard(); got != msg {
		t.Errorf("the clipboard holds %q, want the share %q", got, msg)
	}
	if currentShareSession != nil {
		t.Error("the share is still current after its fallback")
	}
}
