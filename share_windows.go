//go:build windows

package bibletext

// SHARE ON WINDOWS: THE SYSTEM SHARE SHEET, WITH THE IN-APP SHEET BEHIND IT.
//
// Every share verb — Share with note, with citation, as link, the verse of
// the day's Share and Share as image — opens the Windows Share sheet for the
// app's window, as the picker opens on macOS
// (https://learn.microsoft.com/en-us/windows/apps/develop/windows-integration/integrate-sharesheet-send):
// IDataTransferManagerInterop.GetForWindow gives the window's
// DataTransferManager, a DataRequested handler fills the package when the
// sheet asks for it, and ShowShareUIForWindow opens the sheet. The package's
// title, without which the sheet will not open, is the citation; its text is
// the message every platform shares; a link or note share sets the link as a
// web link too, which gives the sheet its Copy link and the apps that take a
// link the link; the picture goes as a file, named for the reader.
//
// Whatever keeps the sheet from taking the share — no native window, a
// Windows without the interface, a thread already in the other kind of
// apartment, an HRESULT from any step, a sheet that never asks, a package
// that will not fill — ends the share in the verb's in-app confirmation
// sheet (share_fallback.go, share_sheet_desktop.go), the one Linux shares
// through, so a share the reader started never ends in silence
// (shareSession, share_session.go).
//
// Threads. Every call is made on the window's thread. That is the Fyne UI
// goroutine's, which the toolkit locks to the main thread and makes the
// window on, and the share verbs run from its event handling. RunNative on
// Windows runs its callback on whichever goroutine calls it rather than on
// the window's thread, so the thread is checked, not assumed
// (windowsShareVerb). The sheet's request arrives as a window message, which
// the toolkit's event loop dispatches, so the handler runs inside that loop;
// it fills the package and posts everything else.
//
// The unpackaged download opens the sheet as the Store's MSIX does: from an
// executable without package identity on Windows 11 arm64, natively and
// under x64 emulation, every step answered S_OK and the sheet opened for
// text, a link and a picture. Microsoft's pages disagree about whether it
// should, so a refusal on some other Windows lands in the fallback like any
// other failure.

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

// nativeShareText opens the Windows Share sheet with a composed text share:
// the citation as its title, the message as its text, and a link or note
// share's link as a web link. The in-app confirmation sheet is its fallback.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func nativeShareText(s string) {
	p := sharePartsFor(s)
	payload := windowsSharePayload{title: p.title, text: p.text, link: p.link}
	windowsShareVerb(func() { fallbackShareText(s) }, func(sh *shareSession) { presentShare(sh, payload) })
}

// nativeShareImage opens the Windows Share sheet with the rendered card, as
// a file named for the reader (shareImageName) under the verse's citation.
// The in-app confirmation sheet is its fallback.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func nativeShareImage(path string) {
	title := shareImageMail.subject // read now: the next preview rewrites it
	if title == "" {
		title = ProductName()
	}
	windowsShareVerb(func() { fallbackShareImage(path) }, func(sh *shareSession) { shareImageFile(sh, path, title) })
}

// windowsSharePayload is what one share hands the sheet.
type windowsSharePayload struct {
	title, text, link string
	items             *comObject // the picture as a collection of one, which the session holds; nil for text
}

// windowsShare is the package's parts, as the DataRequested handler hands
// them to the sheet. They are made on the window's thread before the sheet
// opens, because the handler may run on any thread and makes nothing
// itself, and the session frees them when the share ends.
type windowsShare struct {
	session *shareSession
	title   hstring
	text    hstring    // 0 for the picture
	uri     *comRef    // the link as an IUriRuntimeClass, or nil
	items   *comObject // the picture's collection, or nil
}

// windowsImage is a picture whose file is resolving.
type windowsImage struct {
	session *shareSession
	op      *comRef // the IAsyncOperation<StorageFile>, which the session holds
	title   string
	status  atomic.Int32 // the operation's AsyncStatus once it completes
}

// The steps that go through windowsShareStep.
const (
	stepGetForWindow = "IDataTransferManagerInterop.GetForWindow"
	stepAddHandler   = "DataTransferManager.add_DataRequested"
	stepShowSheet    = "IDataTransferManagerInterop.ShowShareUIForWindow"
	stepResolveFile  = "StorageFile.GetFileFromPathAsync"
	stepFileHandler  = "IAsyncOperation<StorageFile>.put_Completed"
)

// windowsShareStep makes the call named step. A test replaces it to fail a
// step, or to keep the sheet from opening on the machine running it.
var windowsShareStep = func(step string, call func() int32) int32 { return call() }

// windowsShareFailText is what the sheet says when its package could not be
// filled; the in-app sheet opens as well.
const windowsShareFailText = "The share could not be prepared."

// windowsShareBusy is set while the share's calls to Windows are being made.
// A call that waits can dispatch window messages, the reader's clicks among
// them, and a share started from one of those waits for the one beneath it.
var windowsShareBusy bool

// windowsShareVerb starts a share on the window's thread: here when this is
// that thread and no share is being set up beneath this call, otherwise
// posted there.
func windowsShareVerb(fallback func(), start func(*shareSession)) {
	run := func() {
		s := startShareSession(fallback)
		windowsShareCalls(func() { start(s) })
	}
	if windowsShareBusy || offWindowThread() {
		shareSessionOnUI(run)
		return
	}
	run()
}

// windowsShareCalls runs f, which calls Windows, with windowsShareBusy set.
func windowsShareCalls(f func()) {
	windowsShareBusy = true
	defer func() { windowsShareBusy = false }()
	f()
}

// windowsShareHWND is the app window's handle, reached as the title bar and
// restoreNative reach it, or 0 where there is no native window.
func windowsShareHWND() uintptr {
	state := activeAIState
	if state == nil || state.window == nil {
		return 0
	}
	nw, ok := state.window.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var hwnd uintptr
	nw.RunNative(func(ctx any) {
		if c, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = c.HWND
		}
	})
	return hwnd
}

// offWindowThread reports whether there is a window and this is not its
// thread.
func offWindowThread() bool {
	hwnd := windowsShareHWND()
	if hwnd == 0 {
		return false
	}
	tid, _ := windows.GetWindowThreadProcessId(windows.HWND(hwnd), nil)
	return tid != windows.GetCurrentThreadId()
}

var errNoShareWindow = errors.New("no native window to open the Share sheet for")

// shareWindowHere is the window's handle when this is the window's thread.
func shareWindowHere() (uintptr, error) {
	hwnd := windowsShareHWND()
	if hwnd == 0 {
		return 0, errNoShareWindow
	}
	if tid, _ := windows.GetWindowThreadProcessId(windows.HWND(hwnd), nil); tid != windows.GetCurrentThreadId() {
		return 0, fmt.Errorf("the share runs on thread %d, not the window's %d", windows.GetCurrentThreadId(), tid)
	}
	return hwnd, nil
}

// failWindowsShare ends s in its fallback, and logs whether this is the
// Store's packaged build or the download: Microsoft's pages disagree on
// whether an app without package identity may open the sheet, and a refusal
// is where that would show.
func failWindowsShare(s *shareSession, err error) {
	s.finish(fmt.Errorf("%w (packaged: %t)", err, packagedWindows()))
}

// presentShare opens the Windows Share sheet for the window with p. The
// session holds what is made here until the share ends. Window's thread.
func presentShare(s *shareSession, p windowsSharePayload) {
	hwnd, err := shareWindowHere()
	if err == nil {
		err = ensureWinRT()
	}
	if err != nil {
		failWindowsShare(s, err)
		return
	}
	factory, err := activationFactory("Windows.ApplicationModel.DataTransfer.DataTransferManager", &iidIActivationFactory)
	if err != nil {
		failWindowsShare(s, err)
		return
	}
	interop, hr := factory.query(&iidIDataTransferManagerInterop)
	factory.release()
	if hr < 0 {
		failWindowsShare(s, hresultError{"DataTransferManager as IDataTransferManagerInterop", hr})
		return
	}
	defer interop.release()
	var dtm *comRef
	hr = windowsShareStep(stepGetForWindow, func() int32 {
		return comCall(interop, 3, hwnd, uintptr(unsafe.Pointer(&iidIDataTransferManager)), uintptr(unsafe.Pointer(&dtm)))
	})
	if hr < 0 {
		failWindowsShare(s, hresultError{stepGetForWindow, hr})
		return
	}
	s.hold(dtm.release)

	ws := &windowsShare{session: s, items: p.items}
	if ws.title, err = newHString(p.title); err != nil {
		failWindowsShare(s, err)
		return
	}
	s.hold(func() { deleteHString(ws.title) })
	if p.text != "" {
		if ws.text, err = newHString(p.text); err != nil {
			failWindowsShare(s, err)
			return
		}
		s.hold(func() { deleteHString(ws.text) })
	}
	if p.link != "" {
		// Without the web link the share still carries the link in its text.
		if uri, err := newURI(p.link); err != nil {
			fyne.LogError("Windows share: the link as a web link", err)
		} else {
			ws.uri = uri
			s.hold(uri.release)
		}
	}

	// One handler per share, removed as the share ends. That is always
	// posted, so never inside ShowShareUIForWindow, whose own call to the
	// handler, where it makes one, is a point Chromium's implementation
	// records the system as not handling a removal at.
	handler := newComObject(&comObject{iid: iidDataRequestedHandler, share: ws},
		func() *comVtbl { return &dataRequestedVtbl })
	s.hold(handler.release)
	var token int64
	hr = windowsShareStep(stepAddHandler, func() int32 {
		return comCall(dtm, 6, uintptr(unsafe.Pointer(handler)), uintptr(unsafe.Pointer(&token)))
	})
	if hr < 0 {
		failWindowsShare(s, hresultError{stepAddHandler, hr})
		return
	}
	s.hold(func() { comCall(dtm, 7, tokenArgs(token)...) })

	s.await("the Share sheet asking for the share")
	hr = windowsShareStep(stepShowSheet, func() int32 { return comCall(interop, 4, hwnd) })
	if hr < 0 {
		failWindowsShare(s, hresultError{stepShowSheet, hr})
	}
}

// tokenArgs is an EventRegistrationToken, a struct holding one int64, as it
// is passed by value: in one register on 64-bit Windows, and as two stack
// words, low first, on 32-bit.
func tokenArgs(tok int64) []uintptr {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return []uintptr{uintptr(tok)}
	}
	return []uintptr{uintptr(uint32(tok)), uintptr(uint32(uint64(tok) >> 32))}
}

// newURI is link as a Windows.Foundation.Uri.
func newURI(link string) (*comRef, error) {
	f, err := activationFactory("Windows.Foundation.Uri", &iidIUriRuntimeClassFactory)
	if err != nil {
		return nil, err
	}
	defer f.release()
	hs, err := newHString(link)
	if err != nil {
		return nil, err
	}
	defer deleteHString(hs)
	var uri *comRef
	if hr := comCall(f, 6, uintptr(hs), uintptr(unsafe.Pointer(&uri))); hr < 0 {
		return nil, hresultError{"Uri.CreateUri", hr}
	}
	return uri, nil
}

// dataRequestedInvoke is the sheet asking for the share. It answers S_OK
// whatever happens: Windows ends a process whose DataRequested handler
// returns a failure, as Chromium's implementation records.
func dataRequestedInvoke(this *comObject, sender, args *comRef) (ret uintptr) {
	defer recoverHR(&ret, hrOK)
	if args == nil || !comLiveHas(this) || this.share == nil {
		return hrWord(hrOK)
	}
	ws := this.share
	ws.session.deliver(func() error { return fillShareRequest(args, ws) })
	return hrWord(hrOK)
}

// fillShareRequest fills the package of the sheet's request, or tells the
// sheet it could not.
func fillShareRequest(args *comRef, ws *windowsShare) error {
	var req *comRef
	if hr := comCall(args, 6, uintptr(unsafe.Pointer(&req))); hr < 0 {
		return hresultError{"DataRequestedEventArgs.get_Request", hr}
	}
	defer req.release()
	var pkg *comRef
	if hr := comCall(req, 6, uintptr(unsafe.Pointer(&pkg))); hr < 0 {
		return hresultError{"DataRequest.get_Data", hr}
	}
	defer pkg.release()
	err := fillSharePackage(pkg, ws)
	if err != nil {
		if hs, e := newHString(windowsShareFailText); e == nil {
			comCall(req, 9, uintptr(hs)) // FailWithDisplayText
			deleteHString(hs)
		}
	}
	return err
}

// fillSharePackage puts ws into pkg, a DataPackage: the title the sheet
// needs, then whichever of the text, the web link and the picture the share
// has. It fails when the title or every one of those fails.
func fillSharePackage(pkg *comRef, ws *windowsShare) error {
	var props *comRef
	if hr := comCall(pkg, 7, uintptr(unsafe.Pointer(&props))); hr < 0 {
		return hresultError{"DataPackage.get_Properties", hr}
	}
	hr := comCall(props, 7, uintptr(ws.title))
	props.release()
	if hr < 0 {
		return hresultError{"DataPackagePropertySet.put_Title", hr}
	}
	var errs []error
	filled := 0
	add := func(step string, hr int32) {
		if hr < 0 {
			errs = append(errs, hresultError{step, hr})
		} else {
			filled++
		}
	}
	if ws.text != 0 {
		add("DataPackage.SetText", comCall(pkg, 16, uintptr(ws.text)))
	}
	if ws.uri != nil {
		pkg2, hr := pkg.query(&iidIDataPackage2)
		if hr >= 0 {
			hr = comCall(pkg2, 7, uintptr(unsafe.Pointer(ws.uri)))
			pkg2.release()
		}
		add("DataPackage.SetWebLink", hr)
	}
	if ws.items != nil {
		// SetStorageItems(items, readOnly): the overload at slot 23. The
		// sheet's apps get the file to read, not to change.
		add("DataPackage.SetStorageItems", comCall(pkg, 23, uintptr(unsafe.Pointer(ws.items)), 1))
	}
	if filled == 0 {
		if len(errs) == 0 {
			return errors.New("the share has nothing to put in the package")
		}
		return errors.Join(errs...)
	}
	return nil
}

// shareImageFile copies the card to a file named for the reader, has Windows
// resolve it as a StorageFile, and opens the sheet with it once it has. The
// file is resolved before the sheet opens, as .NET MAUI does it, not inside
// the sheet's request: a request that defers has 200 ms by the
// documentation, which a first resolve, loading the storage broker, can
// exceed, and a picture that cannot be resolved falls back before any system
// UI has appeared. Window's thread.
func shareImageFile(s *shareSession, path, title string) {
	if _, err := shareWindowHere(); err != nil {
		failWindowsShare(s, err)
		return
	}
	if err := ensureWinRT(); err != nil {
		failWindowsShare(s, err)
		return
	}
	file, err := copyShareImage(path, time.Now())
	if err != nil {
		failWindowsShare(s, err)
		return
	}
	statics, err := activationFactory("Windows.Storage.StorageFile", &iidIStorageFileStatics)
	if err != nil {
		failWindowsShare(s, err)
		return
	}
	hs, err := newHString(file)
	if err != nil {
		statics.release()
		failWindowsShare(s, err)
		return
	}
	var op *comRef
	hr := windowsShareStep(stepResolveFile, func() int32 {
		return comCall(statics, 6, uintptr(hs), uintptr(unsafe.Pointer(&op)))
	})
	deleteHString(hs)
	statics.release()
	if hr < 0 {
		failWindowsShare(s, hresultError{stepResolveFile, hr})
		return
	}
	s.hold(op.release)
	img := &windowsImage{session: s, op: op, title: title}
	img.status.Store(-1)
	done := newComObject(&comObject{iid: iidStorageFileCompletedHandler, image: img},
		func() *comVtbl { return &fileResolvedVtbl })
	s.await("the picture's file resolving")
	hr = windowsShareStep(stepFileHandler, func() int32 {
		return comCall(op, 6, uintptr(unsafe.Pointer(done)))
	})
	done.release() // the operation holds its own reference
	if hr < 0 {
		failWindowsShare(s, hresultError{stepFileHandler, hr})
	}
}

// asyncStatusCompleted is AsyncStatus.Completed.
const asyncStatusCompleted = 1

// fileResolvedInvoke hears the picture's file resolve: on a thread-pool
// thread the first time a file is asked for, and inside put_Completed on the
// window's thread when it has been asked for before. It records the status
// and posts the rest to the window's thread, where the operation was made.
func fileResolvedInvoke(this *comObject, op *comRef, status uintptr) (ret uintptr) {
	defer recoverHR(&ret, hrOK)
	if !comLiveHas(this) || this.image == nil {
		return hrWord(hrOK)
	}
	img := this.image
	img.status.Store(int32(uint32(status)))
	img.session.post(func() { windowsShareCalls(func() { shareResolvedImage(img) }) })
	return hrWord(hrOK)
}

// shareResolvedImage opens the sheet with the resolved file. Window's thread.
func shareResolvedImage(img *windowsImage) {
	s := img.session
	if st := img.status.Load(); st != asyncStatusCompleted {
		failWindowsShare(s, fmt.Errorf("the picture's file did not resolve: AsyncStatus %d", st))
		return
	}
	var file *comRef
	if hr := comCall(img.op, 8, uintptr(unsafe.Pointer(&file))); hr < 0 {
		failWindowsShare(s, hresultError{"IAsyncOperation<StorageFile>.GetResults", hr})
		return
	}
	item, hr := file.query(&iidIStorageItem)
	file.release()
	if hr < 0 {
		failWindowsShare(s, hresultError{"StorageFile as IStorageItem", hr})
		return
	}
	items := newStorageItems(item)
	s.hold(items.release)
	presentShare(s, windowsSharePayload{title: img.title, items: items})
}
