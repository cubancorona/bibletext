//go:build !ios && !android

package bibletext

// refreshLocalTimeZone has a real implementation on the two platforms where the
// standard library leaves time.Local at UTC (timezone_mobile.go). On macOS,
// Windows and Linux the library reads the system zone itself, transitions and
// all, so there is nothing to refresh.
func refreshLocalTimeZone() {}
