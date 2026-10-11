//go:build !bibletextdev || !ios

package bibletext

func devDiagnosticsNativeStart() {}

func devDiagnosticsNativeFootprint() (int64, bool) { return 0, false }

func devDiagnosticsNativeBuild() (string, int, bool) { return "", 0, false }
