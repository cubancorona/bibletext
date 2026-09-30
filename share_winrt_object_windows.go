//go:build windows

package bibletext

// The COM objects the Windows share implements in Go (share_windows.go): the
// sheet's DataRequested handler, the handler that hears the picture's file
// resolve, and the one-item collection that carries the picture.
//
// Each is a Go heap object whose first word points at a vtable of callbacks,
// which is all COM reads of it. From creation to its final Release it is
// pinned, so it can neither move nor be freed while Windows holds it, and
// kept in comLive, which is also how a callback on another thread comes to
// see the object as it was built: the registry's lock is taken after the
// object is complete and before any callback reads it.
//
// Each aggregates the free-threaded marshaler and answers IAgileObject, as
// every Windows Runtime object C++/WinRT, WRL or C# makes does: the picture's
// file resolves on a thread-pool thread, and Windows asks the handler for
// IAgileObject before it accepts it. So each is safe on any thread — its
// count and the iterator's position are atomics, and the rest is fixed
// before it is registered.
//
// The rules that keep the collector, go vet and the race detector's pointer
// checks satisfied: a callback receives COM's pointers as typed pointers,
// never as a uintptr turned back into a pointer; it writes an out parameter
// only as a uintptr or a byte, since a pointer-typed store would run a write
// barrier over whatever the caller's buffer held; its callbacks are made
// once, from top-level functions, since each NewCallback takes one of the
// runtime's 2,000 slots for good; and a panic is recovered inside the
// callback, since one that unwound into Windows would end the process.

import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// comVtbl is a vtable: IUnknown's three methods, IInspectable's three where
// the interface has them, then its own. The iterator's ten is the most.
type comVtbl [10]uintptr

type comObject struct {
	vtbl *comVtbl // word 0, which COM reads; points into a package-level vtable

	refs        atomic.Int32
	iid         windows.GUID // the interface beyond IUnknown, IInspectable and IAgileObject
	inspectable bool         // the collection's two are Windows Runtime interfaces; the handlers are plain delegates
	className   string       // what GetRuntimeClassName answers, for an inspectable
	ftm         *comRef      // the aggregated free-threaded marshaler's inner IUnknown, or nil
	pin         runtime.Pinner

	// Go's own state, never read by Windows.
	share *windowsShare // the DataRequested handler's share
	image *windowsImage // the file handler's picture
	item  *comRef       // the collection's one IStorageItem, a reference it owns
	next  atomic.Uint32 // the iterator: 0 on the item, 1 past it
}

// comLive is every object Windows may still call: the collector's root for
// them, and the lock that publishes each to the thread that calls it.
var comLive = struct {
	sync.Mutex
	m map[*comObject]struct{}
}{m: map[*comObject]struct{}{}}

// The vtables are filled on first use, not as package-level initializers:
// the handlers' callbacks lead back to the constructors that name these
// tables, which Go would refuse as an initialization cycle.
var (
	comVtblOnce       sync.Once
	dataRequestedVtbl comVtbl
	fileResolvedVtbl  comVtbl
	iterableVtbl      comVtbl
	iteratorVtbl      comVtbl
)

func fillComVtbls() {
	qi := windows.NewCallback(comQueryInterface)
	addRef := windows.NewCallback(comAddRef)
	release := windows.NewCallback(comRelease)
	iids := windows.NewCallback(comGetIids)
	class := windows.NewCallback(comGetRuntimeClassName)
	trust := windows.NewCallback(comGetTrustLevel)
	dataRequestedVtbl = comVtbl{qi, addRef, release, windows.NewCallback(dataRequestedInvoke)}
	fileResolvedVtbl = comVtbl{qi, addRef, release, windows.NewCallback(fileResolvedInvoke)}
	iterableVtbl = comVtbl{qi, addRef, release, iids, class, trust, windows.NewCallback(iterableFirst)}
	iteratorVtbl = comVtbl{qi, addRef, release, iids, class, trust,
		windows.NewCallback(iteratorCurrent), windows.NewCallback(iteratorHasCurrent),
		windows.NewCallback(iteratorMoveNext), windows.NewCallback(iteratorGetMany)}
}

// newComObject makes o live, with one reference, the caller's: o comes
// complete apart from its vtable, which vtbl names.
func newComObject(o *comObject, vtbl func() *comVtbl) *comObject {
	comVtblOnce.Do(fillComVtbls)
	o.vtbl = vtbl()
	o.refs.Store(1)
	o.pin.Pin(o)
	comLive.Lock()
	comLive.m[o] = struct{}{}
	comLive.Unlock()
	// Without the marshaler the object still works in its own apartment; it
	// answers neither IAgileObject nor IMarshal.
	var inner *comRef
	if r, _, _ := procCoCreateFreeThreadedMarshaler.Call(uintptr(unsafe.Pointer(o)), uintptr(unsafe.Pointer(&inner))); hresult(r) >= 0 {
		comLive.Lock()
		o.ftm = inner
		comLive.Unlock()
	}
	return o
}

// comLiveHas reports whether o is still live, taking the registry's lock, so
// that a callback on any thread then sees o as it was built.
func comLiveHas(o *comObject) bool {
	comLive.Lock()
	_, ok := comLive.m[o]
	comLive.Unlock()
	return ok
}

func comLiveCount() int {
	comLive.Lock()
	defer comLive.Unlock()
	return len(comLive.m)
}

func (o *comObject) release() { comRelease(o) }

func (o *comObject) destroy() {
	comLive.Lock()
	delete(comLive.m, o)
	comLive.Unlock()
	if o.ftm != nil {
		o.ftm.release()
		o.ftm = nil
	}
	if o.item != nil {
		o.item.release()
		o.item = nil
	}
	o.share, o.image = nil, nil
	o.pin.Unpin()
}

// recoverHR turns a panic in a callback into the HRESULT hr.
func recoverHR(ret *uintptr, hr int32) {
	if recover() != nil {
		*ret = hrWord(hr)
	}
}

func comQueryInterface(this *comObject, riid *windows.GUID, ppv *uintptr) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if ppv == nil {
		return hrWord(hrPointer)
	}
	*ppv = 0
	if riid == nil {
		return hrWord(hrInvalidArg)
	}
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	switch g := *riid; {
	case g == iidIUnknown, g == this.iid, this.inspectable && g == iidIInspectable, this.ftm != nil && g == iidIAgileObject:
		this.refs.Add(1)
		*ppv = uintptr(unsafe.Pointer(this))
		return hrWord(hrOK)
	case this.ftm != nil && g == iidIMarshal:
		// The marshaler's own, non-delegating QueryInterface: the IMarshal it
		// hands out counts its references on this object.
		return hrWord(comCall(this.ftm, 0, uintptr(unsafe.Pointer(riid)), uintptr(unsafe.Pointer(ppv))))
	}
	return hrWord(hrNoInterface)
}

func comAddRef(this *comObject) uintptr {
	return uintptr(uint32(this.refs.Add(1)))
}

func comRelease(this *comObject) uintptr {
	n := this.refs.Add(-1)
	if n == 0 {
		this.destroy()
	}
	return uintptr(uint32(n))
}

func comGetIids(this *comObject, count *uint32, iids *uintptr) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if count == nil || iids == nil {
		return hrWord(hrPointer)
	}
	*count, *iids = 0, 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	// The caller frees the list with CoTaskMemFree; it names the interfaces
	// beyond IUnknown and IInspectable.
	p, _, _ := procCoTaskMemAlloc.Call(unsafe.Sizeof(this.iid))
	if p == 0 {
		return hrWord(hrOutOfMemory)
	}
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&this.iid)), unsafe.Sizeof(this.iid))
	*count, *iids = 1, p
	return hrWord(hrOK)
}

func comGetRuntimeClassName(this *comObject, name *hstring) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if name == nil {
		return hrWord(hrPointer)
	}
	*name = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	h, err := newHString(this.className)
	if err != nil {
		return hrWord(hrOutOfMemory)
	}
	*name = h
	return hrWord(hrOK)
}

func comGetTrustLevel(this *comObject, level *int32) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if level == nil {
		return hrWord(hrPointer)
	}
	*level = 0 // BaseTrust
	return hrWord(hrOK)
}

// --- IIterable<IStorageItem> and IIterator<IStorageItem> ------------------------

// The picture goes to the sheet as a collection of one: SetStorageItems takes
// an IIterable<IStorageItem>. Chromium hands it a WRL vector, and .NET an
// array that C#/WinRT wraps; this is the least of those. The sheet's package
// walks it with First, HasCurrent, Current and MoveNext, and lets it go when
// the package is filled.

// newStorageItems is a collection of item, which it takes the caller's
// reference to.
func newStorageItems(item *comRef) *comObject {
	return newComObject(&comObject{
		iid:         iidIIterableStorageItem,
		inspectable: true,
		className:   "Windows.Foundation.Collections.IIterable`1<Windows.Storage.IStorageItem>",
		item:        item,
	}, func() *comVtbl { return &iterableVtbl })
}

func iterableFirst(this *comObject, out *uintptr) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if out == nil {
		return hrWord(hrPointer)
	}
	*out = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	this.item.addRef()
	it := newComObject(&comObject{
		iid:         iidIIteratorStorageItem,
		inspectable: true,
		className:   "Windows.Foundation.Collections.IIterator`1<Windows.Storage.IStorageItem>",
		item:        this.item,
	}, func() *comVtbl { return &iteratorVtbl })
	*out = uintptr(unsafe.Pointer(it)) // the new iterator's one reference goes to the caller
	return hrWord(hrOK)
}

func iteratorCurrent(this *comObject, out *uintptr) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if out == nil {
		return hrWord(hrPointer)
	}
	*out = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	if this.next.Load() != 0 || this.item == nil {
		return hrWord(hrBounds)
	}
	this.item.addRef()
	*out = uintptr(unsafe.Pointer(this.item))
	return hrWord(hrOK)
}

func iteratorHasCurrent(this *comObject, out *uint8) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if out == nil {
		return hrWord(hrPointer)
	}
	*out = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	if this.next.Load() == 0 && this.item != nil {
		*out = 1
	}
	return hrWord(hrOK)
}

// iteratorMoveNext steps past the one item; there is never a current after it.
func iteratorMoveNext(this *comObject, out *uint8) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if out == nil {
		return hrWord(hrPointer)
	}
	*out = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	this.next.Store(1)
	return hrWord(hrOK)
}

// iteratorGetMany hands over the item, if the iterator is still on it, and
// steps past it.
func iteratorGetMany(this *comObject, capacity uintptr, items *[1 << 16]uintptr, actual *uint32) (ret uintptr) {
	defer recoverHR(&ret, hrUnexpected)
	if actual == nil {
		return hrWord(hrPointer)
	}
	*actual = 0
	if !comLiveHas(this) {
		return hrWord(hrUnexpected)
	}
	if uint32(capacity) >= 1 && items != nil && this.item != nil && this.next.CompareAndSwap(0, 1) {
		this.item.addRef()
		items[0] = uintptr(unsafe.Pointer(this.item))
		*actual = 1
	}
	return hrWord(hrOK)
}
