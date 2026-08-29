//go:build darwin

// The Apple Keychain adapter, used on iOS always and on macOS only by a build
// that carries a stable signing identity.
//
// macOS used to be excluded outright, for a good reason: release.yml ships the
// direct-download build from a bare `fyne package -os darwin` with no Developer
// ID, and an ad-hoc signature binds the login-keychain ACL to a code hash that
// changes with EVERY update. Reads then prompt alarmingly or fail with
// errSecAuthFailed, and a migration that had already erased the Preferences
// copy would take the reader's key with it.
//
// That reasoning is about the SIGNATURE, not about macOS, and the two macOS
// builds no longer share one. The Mac App Store build is signed with a real
// identity under a Team ID and sandboxed; the direct download is still ad-hoc.
// So the decision is made at runtime from the identity the running binary
// actually has (btAIKeychainUsable below), and an ad-hoc or unsigned build —
// the direct download, and any `go run` during development — keeps the old
// Preferences behaviour untouched.
//
// Falling back is safe in both directions: getAPIKey erases the Preferences
// copy only after a Write that returned true, and never after a store error.

package bibletext

/*
#cgo LDFLAGS: -framework Foundation -framework Security
#include <stdlib.h>
#include <string.h>
#include <TargetConditionals.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

#if TARGET_OS_OSX
// btAIKeychainUsable reports whether THIS binary should keep secrets in the
// login keychain. An ad-hoc signature has no team identifier, and it is exactly
// the ad-hoc case whose ACL churns on every update, so the presence of a team
// identifier is the question being asked — not "is this a Mac".
static int btAIKeychainUsable(void) {
    @autoreleasepool {
        SecCodeRef me = NULL;
        if (SecCodeCopySelf(kSecCSDefaultFlags, &me) != errSecSuccess || me == NULL) {
            return 0;
        }
        CFDictionaryRef info = NULL;
        OSStatus st = SecCodeCopySigningInformation(
            (SecStaticCodeRef)me, kSecCSSigningInformation, &info);
        CFRelease(me);
        if (st != errSecSuccess || info == NULL) return 0;
        CFStringRef team = (CFStringRef)CFDictionaryGetValue(info, kSecCodeInfoTeamIdentifier);
        int usable = (team != NULL && CFStringGetLength(team) > 0);
        CFRelease(info);
        return usable;
    }
}
#else
// Every iOS build is signed with a team identity; there is no ad-hoc case.
static int btAIKeychainUsable(void) { return 1; }
#endif

static NSString *btAIKeychainService(void) {
    NSString *bundleID = [[NSBundle mainBundle] bundleIdentifier];
    if (bundleID.length == 0) bundleID = @"uk.co.bibletext";
    return [bundleID stringByAppendingString:@".ai-provider-keys"];
}

// Returns 1 when found, 0 when absent, and -1 on a Keychain error.
static int btAIKeychainRead(const char *accountUTF8, char **valueUTF8) {
    @autoreleasepool {
        if (accountUTF8 == NULL || valueUTF8 == NULL) return -1;
        NSString *account = [NSString stringWithUTF8String:accountUTF8];
        NSDictionary *query = @{
            (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
            (__bridge id)kSecAttrService: btAIKeychainService(),
            (__bridge id)kSecAttrAccount: account,
            (__bridge id)kSecReturnData: @YES,
            (__bridge id)kSecMatchLimit: (__bridge id)kSecMatchLimitOne,
        };
        CFTypeRef result = NULL;
        OSStatus status = SecItemCopyMatching((__bridge CFDictionaryRef)query, &result);
        if (status == errSecItemNotFound) return 0;
        if (status != errSecSuccess || result == NULL) return -1;

        NSData *data = (__bridge NSData *)result;
        *valueUTF8 = malloc(data.length + 1);
        if (*valueUTF8 != NULL) {
            memcpy(*valueUTF8, data.bytes, data.length);
            (*valueUTF8)[data.length] = '\0';
        }
        CFRelease(result);
        return *valueUTF8 == NULL ? -1 : 1;
    }
}

static int btAIKeychainWrite(const char *accountUTF8, const char *valueUTF8) {
    @autoreleasepool {
        if (accountUTF8 == NULL || valueUTF8 == NULL) return 0;
        NSString *account = [NSString stringWithUTF8String:accountUTF8];
        NSString *value = [NSString stringWithUTF8String:valueUTF8];
        if (account == nil || value == nil) return 0; // invalid UTF-8 payload
        NSDictionary *query = @{
            (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
            (__bridge id)kSecAttrService: btAIKeychainService(),
            (__bridge id)kSecAttrAccount: account,
        };

        if (value.length == 0) {
            OSStatus status = SecItemDelete((__bridge CFDictionaryRef)query);
            return status == errSecSuccess || status == errSecItemNotFound;
        }

        // ACCESSIBILITY: AfterFirstUnlock, NOT ...ThisDeviceOnly. Apple excludes
        // ThisDeviceOnly items from every backup and from device migration, so
        // migrating a key out of Preferences into a ThisDeviceOnly item and then
        // erasing the Preferences copy would SILENTLY LOSE the reader's key when
        // they restore a backup or move to a new iPhone — a regression against
        // 1.1.5, where the key travelled with preferences.json.
        // AfterFirstUnlock keeps it encrypted at rest and backup-restorable, and
        // also lets a background launch read it before the first unlock.
        NSData *data = [value dataUsingEncoding:NSUTF8StringEncoding];
        OSStatus status = SecItemUpdate(
            (__bridge CFDictionaryRef)query,
            (__bridge CFDictionaryRef)@{
                (__bridge id)kSecValueData: data,
                // Re-class any item an earlier build stored as ThisDeviceOnly:
                // SecItemAdd sets accessibility once, so only an update fixes it.
                (__bridge id)kSecAttrAccessible: (__bridge id)kSecAttrAccessibleAfterFirstUnlock});
        if (status == errSecItemNotFound) {
            NSMutableDictionary *item = [NSMutableDictionary dictionaryWithDictionary:query];
            item[(__bridge id)kSecValueData] = data;
            item[(__bridge id)kSecAttrAccessible] = (__bridge id)kSecAttrAccessibleAfterFirstUnlock;
            status = SecItemAdd((__bridge CFDictionaryRef)item, NULL);
        }
        return status == errSecSuccess;
    }
}
*/
import "C"

import "unsafe"

type appleKeychainStore struct{}

func (appleKeychainStore) Name() string { return "Keychain" }

func newPlatformSecretStore() secretStore {
	// nil means "this platform has no credential store", which the keystore
	// already handles by staying on Preferences — and which keyInSecureStore
	// reports honestly, so the Settings sheet keeps saying "Saved on this
	// device" rather than claiming a Keychain that is not being used.
	if C.btAIKeychainUsable() != 1 {
		return nil
	}
	return appleKeychainStore{}
}

func (appleKeychainStore) Read(account string) (string, bool, bool) {
	ca := C.CString(account)
	defer C.free(unsafe.Pointer(ca))
	var cv *C.char
	switch C.btAIKeychainRead(ca, &cv) {
	case 1:
		if cv == nil {
			return "", false, false // defensive: found but no payload → treat as store error
		}
		defer C.free(unsafe.Pointer(cv))
		return C.GoString(cv), true, true
	case 0:
		return "", false, true // definitively absent
	default:
		return "", false, false // Keychain error — caller keeps legacy copy, retries later
	}
}

func (appleKeychainStore) Write(account, value string) bool {
	ca := C.CString(account)
	cv := C.CString(value)
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cv))
	return C.btAIKeychainWrite(ca, cv) == 1
}
