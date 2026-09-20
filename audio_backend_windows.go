//go:build audiosmoke && windows

package bibletext

// WHICH AUDIO BACKEND DID WE ACTUALLY GET?
//
// oto's Windows driver tries WASAPI, then WinMM, and if BOTH report
// errDeviceNotFound it installs a nullContext — a sink that accepts samples
// and discards them (driver_windows.go in ebitengine/oto v3). No error is
// returned and nothing is logged: the engine reports playing, the position
// advances, and not one sample reaches a speaker.
//
// A GitHub-hosted Windows runner is a headless VM with no audio endpoint, so
// that is exactly the path it takes. The audio smoke test therefore claimed to
// prove "real WASAPI" while proving nothing of the sort, and when it failed
// later — at the drain, whose timing is a real device's, not a null sink's —
// the failure was read as a defect in the app.
//
// The backend leaves a fingerprint the process can read: COM activation of the
// MMDevice enumerator pulls AUDIOSES.DLL and MMDevAPI.dll into the address
// space, and the WinMM path pulls winmm.dll. The null sink loads neither.
// GetModuleHandleW asks whether a module is ALREADY loaded and never loads one
// itself, so the question is free of side effects and cannot change the answer.

import (
	"syscall"
	"unsafe"
)

// audioBackendModules reports which audio backend DLLs this process has loaded.
// probed is false on platforms where the question is not asked this way.
func audioBackendModules() (loaded []string, probed bool) {
	getModuleHandle := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW")
	for _, name := range []string{"AUDIOSES.DLL", "MMDevAPI.dll", "winmm.dll"} {
		p, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		if h, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(p))); h != 0 {
			loaded = append(loaded, name)
		}
	}
	return loaded, true
}
