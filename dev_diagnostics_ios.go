//go:build bibletextdev && ios

package bibletext

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework UIKit -framework Foundation

#import <UIKit/UIKit.h>
#include <mach/mach.h>
#include <mach/task_info.h>
#include <limits.h>

extern void bibleTextDevDiagnosticNativeEvent(int code);

// UIKit observers have process lifetime. Retaining the tokens also keeps the
// blocks alive independently of the Go lifecycle callbacks.
static NSMutableArray *btDevDiagnosticObservers;

static void btDevDiagnosticsNativeStart(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        static dispatch_once_t once;
        dispatch_once(&once, ^{
            NSArray<NSNotificationName> *names = @[
                UIApplicationWillResignActiveNotification,
                UIApplicationDidEnterBackgroundNotification,
                UIApplicationWillEnterForegroundNotification,
                UIApplicationDidBecomeActiveNotification,
                UIApplicationDidReceiveMemoryWarningNotification,
                UIApplicationWillTerminateNotification
            ];
            btDevDiagnosticObservers = [NSMutableArray new];
            NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
            for (NSUInteger index = 0; index < names.count; index++) {
                int code = (int)index;
                id token = [center addObserverForName:names[index]
                    object:nil queue:[NSOperationQueue mainQueue]
                    usingBlock:^(NSNotification *notification) {
                        bibleTextDevDiagnosticNativeEvent(code);
                    }];
                [btDevDiagnosticObservers addObject:token];
            }
        });
    });
}

// task_info does not need the UI thread. The diagnostic worker samples it
// alongside Go memory; UIKit callbacks only enqueue their fixed event codes.
static unsigned long long btDevDiagnosticsNativeFootprint(int *ok) {
    task_vm_info_data_t info;
    mach_msg_type_number_t count = TASK_VM_INFO_COUNT;
    kern_return_t result = task_info(mach_task_self(), TASK_VM_INFO,
        (task_info_t)&info, &count);
    if (result != KERN_SUCCESS) {
        *ok = 0;
        return 0;
    }
    *ok = 1;
    return info.phys_footprint;
}

typedef struct {
    char version[65];
    int build;
    int ok;
} BTDevDiagnosticBuild;

// Packaging advances the source metadata for the next build. The installed
// bundle identifies the running binary; bounded copies keep worker sampling
// independent of UIKit and avoid retaining Foundation objects in Go.
static BTDevDiagnosticBuild btDevDiagnosticsNativeBuild(void) {
    BTDevDiagnosticBuild result = {0};
    @autoreleasepool {
        NSBundle *bundle = [NSBundle mainBundle];
        id version = [bundle objectForInfoDictionaryKey:@"CFBundleShortVersionString"];
        id build = [bundle objectForInfoDictionaryKey:@"CFBundleVersion"];
        if (![version isKindOfClass:[NSString class]]
            || ![build isKindOfClass:[NSString class]]
            || [version length] == 0 || [version length] > 64
            || [build length] == 0 || [build length] > 10) {
            return result;
        }
        if (![version getCString:result.version maxLength:sizeof(result.version)
            encoding:NSUTF8StringEncoding]) {
            return result;
        }
        char digits[11] = {0};
        if (![build getCString:digits maxLength:sizeof(digits)
            encoding:NSASCIIStringEncoding]) {
            return result;
        }
        unsigned long long value = 0;
        for (NSUInteger index = 0; digits[index] != 0; index++) {
            if (digits[index] < '0' || digits[index] > '9') {
                return result;
            }
            value = value * 10 + (unsigned int)(digits[index] - '0');
        }
        if (value == 0 || value > INT_MAX) {
            return result;
        }
        result.build = (int)value;
        result.ok = 1;
        return result;
    }
}
*/
import "C"

func devDiagnosticsNativeStart() {
	C.btDevDiagnosticsNativeStart()
}

func devDiagnosticsNativeFootprint() (int64, bool) {
	var ok C.int
	bytes := C.btDevDiagnosticsNativeFootprint(&ok)
	return int64(bytes), ok != 0
}

func devDiagnosticsNativeBuild() (string, int, bool) {
	identity := C.btDevDiagnosticsNativeBuild()
	if identity.ok == 0 {
		return "", 0, false
	}
	return C.GoString(&identity.version[0]), int(identity.build), true
}

//export bibleTextDevDiagnosticNativeEvent
func bibleTextDevDiagnosticNativeEvent(code C.int) {
	var event string
	switch code {
	case 0:
		event = "native_will_resign"
	case 1:
		event = "native_did_background"
	case 2:
		event = "native_will_foreground"
	case 3:
		event = "native_did_active"
	case 4:
		event = "native_memory_warning"
	case 5:
		event = "native_will_terminate"
	default:
		return
	}
	devDiagnosticEvent(event)
}
