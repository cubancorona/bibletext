package bibletext

import "time"

// THE DAY BOUNDARY ON A PHONE.
//
// Go's standard library leaves time.Local at UTC on iOS: its initialisation
// there is a placeholder that never asks the device (time/zoneinfo_ios.go), and
// the app bundle carries no zone database for it to consult. On Android the
// library does the same and fyne patches over it once, at launch, with a
// FixedZone built from the offset then in force. So the verse of the day, which
// rolls at local midnight, rolled at 01:00 during British Summer Time on the
// phone and disagreed with the desktop for twenty-three hours a day all summer;
// and on Android an app left alive across the clock change kept the old offset
// until it was next killed.
//
// refreshLocalTimeZone (timezone_mobile.go, a no-op elsewhere) sets time.Local
// from the device's CURRENT zone, once at startup and again on every return to
// the foreground. The decision of what to set it to is this function, which is
// pure so the host suite can hold it.

// localZoneFor returns the Location to use as time.Local for a device whose
// zone database name is name (Europe/London) and whose C library currently
// reports the abbreviation abbrev at offset seconds east of UTC.
//
// The name is preferred. A Location loaded from the zone database carries the
// zone's transitions, so a day boundary computed against it moves with the
// clock change at the right minute even when the app has been open across it;
// a FixedZone knows only the offset it was built with, and is wrong for every
// hour between a transition and the next time the zone is read. The database
// is embedded in the mobile builds for this (timezone_mobile.go).
//
// An unloadable name — a device with a zone the embedded database has never
// heard of — falls back to the offset, which is the C library's word on where
// the device is right now. So do the two names LoadLocation would answer
// without consulting the database: "" (UTC) and "Local" (the current
// time.Local, which on iOS is the placeholder UTC — the very value being
// replaced).
func localZoneFor(name, abbrev string, offset int) *time.Location {
	switch name {
	case "", "Local":
	default:
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.FixedZone(abbrev, offset)
}
