//go:build windows

package bibletext

// The Windows Runtime underneath the Windows Share sheet (share_windows.go):
// the flat functions, strings and interface ids, calls through a system
// object's vtable, and the apartment the window's thread joins. Written by
// hand on golang.org/x/sys/windows; the Go bindings that exist were ruled out
// (docs/BACKLOG.md, "Windows: use the native Share sheet"). Every interface
// id and vtable slot is the Windows SDK's (share_winrt_iids.go), and every
// call here was seen to answer S_OK on Windows 11 arm64, natively and under
// x64 emulation, from an unpackaged executable.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	combase = windows.NewLazySystemDLL("combase.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")

	procRoInitialize                  = combase.NewProc("RoInitialize")
	procRoGetActivationFactory        = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString           = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString           = combase.NewProc("WindowsDeleteString")
	procCoCreateFreeThreadedMarshaler = ole32.NewProc("CoCreateFreeThreadedMarshaler")
	procCoTaskMemAlloc                = ole32.NewProc("CoTaskMemAlloc")
	procRtlMoveMemory                 = kernel32.NewProc("RtlMoveMemory")
)

// HRESULTs, from winerror.h. A call's HRESULT is the low 32 bits of the
// return register, read as signed: the upper half is undefined after a
// function returning a 32-bit value, and a failure is negative.
const (
	hrOK             int32 = 0
	hrNoInterface    int32 = -0x7FFFBFFE // E_NOINTERFACE 0x80004002
	hrPointer        int32 = -0x7FFFBFFD // E_POINTER 0x80004003
	hrInvalidArg     int32 = -0x7FF8FFA9 // E_INVALIDARG 0x80070057
	hrOutOfMemory    int32 = -0x7FF8FFF2 // E_OUTOFMEMORY 0x8007000E
	hrUnexpected     int32 = -0x7FFF0001 // E_UNEXPECTED 0x8000FFFF
	hrBounds         int32 = -0x7FFFFFF5 // E_BOUNDS 0x8000000B
	hrRPCChangedMode int32 = -0x7FFEFEFA // RPC_E_CHANGED_MODE 0x80010106
)

// roInitSingleThreaded is RO_INIT_SINGLETHREADED: a single-threaded apartment.
const roInitSingleThreaded = 0

// hresult is the HRESULT in a call's return register.
func hresult(r uintptr) int32 { return int32(uint32(r)) }

// hrWord is an HRESULT as a callback returns it.
func hrWord(hr int32) uintptr { return uintptr(uint32(hr)) }

// hresultError is a step of the share that Windows refused, and its answer.
type hresultError struct {
	step string
	hr   int32
}

func (e hresultError) Error() string { return fmt.Sprintf("%s: 0x%08X", e.step, uint32(e.hr)) }

// hrError is nil for a success, and the step's failure otherwise.
func hrError(step string, hr int32) error {
	if hr >= 0 {
		return nil
	}
	return hresultError{step, hr}
}

// winrtProcs is every flat function the share calls. One missing means the
// share is unavailable on this Windows, for the whole session.
var winrtProcs = []*windows.LazyProc{procRoInitialize, procRoGetActivationFactory,
	procWindowsCreateString, procWindowsDeleteString, procCoCreateFreeThreadedMarshaler,
	procCoTaskMemAlloc, procRtlMoveMemory}

// --- strings ---------------------------------------------------------------

// hstring is a Windows Runtime string: an opaque handle, never read from Go.
// Each one made is deleted exactly once; one passed in to a method is only
// borrowed by it, and one handed back is the caller's to delete.
type hstring uintptr

func newHString(s string) (hstring, error) {
	u, err := windows.UTF16FromString(s) // refuses a NUL, which an HSTRING could carry but nothing here means
	if err != nil {
		return 0, err
	}
	var h hstring
	r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	if err := hrError("WindowsCreateString", hresult(r)); err != nil {
		return 0, err
	}
	return h, nil
}

func deleteHString(h hstring) {
	if h != 0 {
		procWindowsDeleteString.Call(uintptr(h))
	}
}

// --- interface ids ---------------------------------------------------------

// guidOf is the id s names, "XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX". The ids
// are constants of this package, so a malformed one is a programming error.
func guidOf(s string) windows.GUID {
	parts := strings.Split(strings.Trim(s, "{}"), "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 ||
		len(parts[3]) != 4 || len(parts[4]) != 12 {
		panic("malformed interface id " + s)
	}
	d1, err1 := strconv.ParseUint(parts[0], 16, 32)
	d2, err2 := strconv.ParseUint(parts[1], 16, 16)
	d3, err3 := strconv.ParseUint(parts[2], 16, 16)
	if err := errors.Join(err1, err2, err3); err != nil {
		panic("malformed interface id " + s)
	}
	g := windows.GUID{Data1: uint32(d1), Data2: uint16(d2), Data3: uint16(d3)}
	tail := parts[3] + parts[4]
	for i := range g.Data4 {
		b, err := strconv.ParseUint(tail[2*i:2*i+2], 16, 8)
		if err != nil {
			panic("malformed interface id " + s)
		}
		g.Data4[i] = byte(b)
	}
	return g
}

var (
	iidIUnknown                    = guidOf(iidStrIUnknown)
	iidIInspectable                = guidOf(iidStrIInspectable)
	iidIAgileObject                = guidOf(iidStrIAgileObject)
	iidIMarshal                    = guidOf(iidStrIMarshal)
	iidIActivationFactory          = guidOf(iidStrIActivationFactory)
	iidIDataTransferManagerInterop = guidOf(iidStrIDataTransferManagerInterop)
	iidIDataTransferManager        = guidOf(iidStrIDataTransferManager)
	iidIDataPackage2               = guidOf(iidStrIDataPackage2)
	iidIUriRuntimeClassFactory     = guidOf(iidStrIUriRuntimeClassFactory)
	iidIStorageFileStatics         = guidOf(iidStrIStorageFileStatics)
	iidIStorageItem                = guidOf(iidStrIStorageItem)
	iidDataRequestedHandler        = guidOf(iidStrDataRequestedHandler)
	iidStorageFileCompletedHandler = guidOf(iidStrStorageFileCompletedHandler)
	iidIIterableStorageItem        = guidOf(iidStrIIterableStorageItem)
	iidIIteratorStorageItem        = guidOf(iidStrIIteratorStorageItem)
)

// --- system objects ----------------------------------------------------------

// comRef is a system COM object. Its first word is its vtable, of which only
// the slots an interface really has are ever read. It is never a Go
// allocation: the pointer is Windows', and the collector ignores it.
type comRef struct{ vtbl *[32]uintptr }

// comCall calls method slot of r with args after the object itself, and
// returns its HRESULT. Slots count from 0: IUnknown's QueryInterface, AddRef
// and Release are 0-2, IInspectable adds 3-5, and a Windows Runtime
// interface's own methods start at 6.
//
// go:uintptrescapes keeps every uintptr(unsafe.Pointer(&v)) written in the
// argument list pointing at live memory for the whole call, as x/sys does
// for LazyProc.Call; each such conversion must be written in the call's own
// argument list for that to hold.
//
//go:uintptrescapes
func comCall(r *comRef, slot int, args ...uintptr) int32 {
	if r == nil {
		return hrPointer
	}
	a := make([]uintptr, 0, 1+len(args))
	a = append(a, uintptr(unsafe.Pointer(r)))
	a = append(a, args...)
	ret, _, _ := syscall.SyscallN(r.vtbl[slot], a...)
	return hresult(ret)
}

func (r *comRef) addRef() {
	if r != nil {
		syscall.SyscallN(r.vtbl[1], uintptr(unsafe.Pointer(r)))
	}
}

func (r *comRef) release() {
	if r != nil {
		syscall.SyscallN(r.vtbl[2], uintptr(unsafe.Pointer(r)))
	}
}

// query is QueryInterface: r's iid interface, a reference the caller owns.
func (r *comRef) query(iid *windows.GUID) (*comRef, int32) {
	var out *comRef
	hr := comCall(r, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, hr
}

// activationFactory is a runtime class's factory, as its iid interface.
func activationFactory(class string, iid *windows.GUID) (*comRef, error) {
	hs, err := newHString(class)
	if err != nil {
		return nil, err
	}
	defer deleteHString(hs)
	var f *comRef
	r, _, _ := procRoGetActivationFactory.Call(uintptr(hs), uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))
	if err := hrError("RoGetActivationFactory "+class, hresult(r)); err != nil {
		return nil, err
	}
	return f, nil
}

// --- the apartment -----------------------------------------------------------

// The window's thread joins a single-threaded apartment on the first share,
// and stays in it: nothing else in the process initialises COM on that
// thread — not GLFW, not the toolkit, not the Go runtime — and the audio
// engine's multithreaded apartment is on threads of its own. The apartment
// is never left, which the documentation asks for only to shut COM down
// gracefully: the thread lives as long as the process, and what the share
// caches would not survive leaving it. Incoming calls to an STA arrive as
// window messages, which the toolkit's event loop already dispatches.
// Window's thread only.
var winrt struct {
	state  int // 0 not yet asked, 1 ready, -1 unavailable for the session
	thread uint32
}

var errWinRTUnavailable = errors.New("the Windows Runtime is unavailable on this Windows")

// ensureWinRT readies the calling thread, the window's, for the share's
// Windows Runtime calls.
func ensureWinRT() error {
	switch winrt.state {
	case 1:
		if t := windows.GetCurrentThreadId(); t != winrt.thread {
			return fmt.Errorf("the share was started on thread %d, not the window's %d", t, winrt.thread)
		}
		return nil
	case -1:
		return errWinRTUnavailable
	}
	for _, p := range winrtProcs {
		if err := p.Find(); err != nil {
			winrt.state = -1
			return fmt.Errorf("%w: %v", errWinRTUnavailable, err)
		}
	}
	r, _, _ := procRoInitialize.Call(roInitSingleThreaded)
	switch hr := hresult(r); {
	case hr == hrRPCChangedMode:
		// The thread is already in a multithreaded apartment, where the
		// share's objects would be reached from other threads.
		winrt.state = -1
		return fmt.Errorf("%w: %v", errWinRTUnavailable, hrError("RoInitialize", hr))
	case hr < 0:
		return hrError("RoInitialize", hr)
	}
	winrt.state = 1
	winrt.thread = windows.GetCurrentThreadId()
	return nil
}
