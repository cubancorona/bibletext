//go:build ios || android

package bibletext

/*
#include <string.h>
#include <time.h>

// btLocalOffset reports the C library's offset east of UTC, in seconds, for
// this instant, and copies the zone abbreviation (BST) into buf. tzset first:
// the library caches the zone it read at first use, and the phone's settings
// change it behind the app's back.
static long btLocalOffset(char *buf, size_t n) {
	time_t now = time(NULL);
	struct tm tm;
	tzset();
	localtime_r(&now, &tm);
	strncpy(buf, tm.tm_zone ? tm.tm_zone : "", n - 1);
	buf[n - 1] = 0;
	return tm.tm_gmtoff;
}
*/
import "C"

import (
	"time"
	// The zone database, so LoadLocation can answer a name on a phone. iOS
	// ships none the standard library can find; Android's system copy is read
	// first and this stands behind it. About 450 KB in the binary.
	_ "time/tzdata"
)

// refreshLocalTimeZone sets time.Local to the device's current zone: the zone
// database entry for the name the platform reports (deviceTimeZoneName —
// Foundation on iOS, the bridge's TimeZone.getDefault() on Android), or a
// FixedZone from the C library's offset when that name cannot be loaded. See
// localZoneFor for the choice and timezone.go for why the standard library's
// own answer is wrong here.
//
// Called once at startup, before the loaded UI exists (StartBackgroundLoad), and
// on every return to the foreground (InstallReadingStateFlush), so a clock
// change or a change of country while the app was away lands on the next
// foreground rather than the next launch. A plain pointer assignment, as the
// mobile driver's own launch-time assignment is.
func refreshLocalTimeZone() {
	var buf [64]C.char
	offset := int(C.btLocalOffset(&buf[0], C.size_t(len(buf))))
	time.Local = localZoneFor(deviceTimeZoneName(), C.GoString(&buf[0]), offset)
}
