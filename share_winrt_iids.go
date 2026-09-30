//go:build !ios && !android

package bibletext

// The interface ids the Windows Share sheet is reached through
// (share_windows.go, share_winrt_windows.go), as the Windows SDK's generated
// headers spell them (windows.applicationmodel.datatransfer.h,
// windows.storage.h, windows.foundation.h, ShObjIdl_core.h, inspectable.h,
// objidlbase.h). They live in a file every desktop build compiles, not only
// the Windows one, so the host suite can hold the parameterized ones to the
// rule they are derived by (share_parts_test.go): a digit mistyped here
// would make Windows answer E_NOINTERFACE, and every share would fall back to
// the in-app sheet on every machine.

import "strings"

const (
	iidStrIUnknown                    = "00000000-0000-0000-C000-000000000046"
	iidStrIInspectable                = "AF86E2E0-B12D-4C6A-9C5A-D7AA65101E90"
	iidStrIAgileObject                = "94EA2B94-E9CC-49E0-C0FF-EE64CA8F5B90"
	iidStrIMarshal                    = "00000003-0000-0000-C000-000000000046"
	iidStrIActivationFactory          = "00000035-0000-0000-C000-000000000046"
	iidStrIDataTransferManagerInterop = "3A3DCD6C-3EAB-43DC-BCDE-45671CE800C8"
	iidStrIDataTransferManager        = "A5CAEE9B-8708-49D1-8D36-67D25A8DA00C"
	iidStrIDataRequestedEventArgs     = "CB8BA807-6AC5-43C9-8AC5-9BA232163182"
	iidStrIDataPackage                = "61EBF5C7-EFEA-4346-9554-981D7E198FFE"
	iidStrIDataPackage2               = "041C1FE9-2409-45E1-A538-4C53EEEE04A7"
	iidStrIUriRuntimeClassFactory     = "44A9796F-723E-4FDF-A218-033E75B0C084"
	iidStrIStorageFileStatics         = "5984C710-DAF2-43C8-8BB4-A4D3EACFD03F"
	iidStrIStorageFile                = "FA3F6186-4214-428C-A64C-14C9AC7315EA"
	iidStrIStorageItem                = "4207A996-CA2F-42F7-BDE8-8B10457A7F30"
	iidStrIAsyncInfo                  = "00000036-0000-0000-C000-000000000046"

	// The parameterized interfaces: one generic's id and its arguments,
	// hashed into an id of their own (winrtParameterized).
	iidStrDataRequestedHandler        = "EC6F9CC8-46D0-5E0E-B4D2-7D7773AE37A0"
	iidStrStorageFileCompletedHandler = "E521C894-2C26-5946-9E61-2B5E188D01ED"
	iidStrIAsyncOperationStorageFile  = "5E52F8CE-ACED-5A42-95B4-F674DD84885E"
	iidStrIIterableStorageItem        = "BB8B8418-65D1-544B-B083-6D172F568C73"
	iidStrIIteratorStorageItem        = "05B487C2-3830-5D3C-98DA-25FA11542DBD"
)

// The generics' own ids ("PIIDs"), which the parameterized ids above are
// derived from.
const (
	piidTypedEventHandler              = "9DE1C534-6AE1-11E0-84E1-18A905BCC53F"
	piidAsyncOperationCompletedHandler = "FCDCF02C-E5D8-4478-915A-4D90B74B83A5"
	piidIAsyncOperation                = "9FC2B0BB-E446-44E2-AA61-9CAB8F636AF2"
	piidIIterable                      = "FAA585EA-6214-4217-AFDA-7F46DE5869B3"
	piidIIterator                      = "6A79E863-4300-459A-9966-CBB660963EE1"
)

// winrtParameterized is each parameterized interface the share uses, with
// the type signature its id is the hash of ("Guid generation for
// parameterized types" in the Windows Runtime type system): the generic's
// own id, then each argument, a runtime class as rc(name;{its default
// interface}) and an interface as its id. The signatures are built from the
// ids above, so a mistyped argument id changes the derived id too.
var winrtParameterized = []struct{ name, signature, iid string }{
	{"TypedEventHandler<DataTransferManager, DataRequestedEventArgs>",
		winrtPinterface(piidTypedEventHandler,
			winrtRuntimeClass("Windows.ApplicationModel.DataTransfer.DataTransferManager", iidStrIDataTransferManager),
			winrtRuntimeClass("Windows.ApplicationModel.DataTransfer.DataRequestedEventArgs", iidStrIDataRequestedEventArgs)),
		iidStrDataRequestedHandler},
	{"AsyncOperationCompletedHandler<StorageFile>",
		winrtPinterface(piidAsyncOperationCompletedHandler,
			winrtRuntimeClass("Windows.Storage.StorageFile", iidStrIStorageFile)),
		iidStrStorageFileCompletedHandler},
	{"IAsyncOperation<StorageFile>",
		winrtPinterface(piidIAsyncOperation, winrtRuntimeClass("Windows.Storage.StorageFile", iidStrIStorageFile)),
		iidStrIAsyncOperationStorageFile},
	{"IIterable<IStorageItem>",
		winrtPinterface(piidIIterable, winrtInterface(iidStrIStorageItem)),
		iidStrIIterableStorageItem},
	{"IIterator<IStorageItem>",
		winrtPinterface(piidIIterator, winrtInterface(iidStrIStorageItem)),
		iidStrIIteratorStorageItem},
}

// winrtInterface is an interface's part of a type signature: its id, lower
// case, in braces.
func winrtInterface(iid string) string { return "{" + strings.ToLower(iid) + "}" }

// winrtRuntimeClass is a runtime class's part of a type signature.
func winrtRuntimeClass(name, defaultIID string) string {
	return "rc(" + name + ";" + winrtInterface(defaultIID) + ")"
}

// winrtPinterface is a parameterized interface's type signature.
func winrtPinterface(piid string, args ...string) string {
	return "pinterface(" + winrtInterface(piid) + ";" + strings.Join(args, ";") + ")"
}
