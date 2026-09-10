//go:build ios

package bibletext

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation

#import <Foundation/Foundation.h>
#include <string.h>

// bibleTextTimeZoneName copies the device's zone database name (Europe/London)
// into buf. resetSystemTimeZone first: Foundation caches the system zone, and
// without the reset a zone changed in Settings while the app was in the
// background would still read as the old one on the next foreground.
static void bibleTextTimeZoneName(char *buf, size_t n) {
    @autoreleasepool {
        [NSTimeZone resetSystemTimeZone];
        const char *name = [[NSTimeZone localTimeZone].name UTF8String];
        strncpy(buf, name ? name : "", n - 1);
        buf[n - 1] = 0;
    }
}
*/
import "C"

// deviceTimeZoneName is the zone database name Foundation reports for the
// device — the one the standard library's iOS initialisation never reads,
// leaving time.Local at UTC (time/zoneinfo_ios.go). Consumed by
// refreshLocalTimeZone (timezone_mobile.go).
func deviceTimeZoneName() string {
	var buf [128]C.char
	C.bibleTextTimeZoneName(&buf[0], C.size_t(len(buf)))
	return C.GoString(&buf[0])
}
