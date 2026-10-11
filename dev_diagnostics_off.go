//go:build !bibletextdev

package bibletext

import "time"

const devDiagnosticsEnabled = false

type devDiagnosticMetric struct {
	Key   string
	Value int64
}

func devDiagnosticsStart()                              {}
func devDiagnosticEvent(string, ...devDiagnosticMetric) {}
func devDiagnosticID() uint64                           { return 0 }
func devDiagnosticNow() time.Time                       { return time.Time{} }
func devDiagnosticSince(time.Time) devDiagnosticMetric {
	return devDiagnosticMetric{Key: "duration_ms"}
}
