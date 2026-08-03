//go:build darwin

package bibletext

/*
#cgo LDFLAGS: -framework Foundation -framework Security
#include <stdlib.h>
#include <string.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

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
        NSDictionary *query = @{
            (__bridge id)kSecClass: (__bridge id)kSecClassGenericPassword,
            (__bridge id)kSecAttrService: btAIKeychainService(),
            (__bridge id)kSecAttrAccount: account,
        };

        if (value.length == 0) {
            OSStatus status = SecItemDelete((__bridge CFDictionaryRef)query);
            return status == errSecSuccess || status == errSecItemNotFound;
        }

        NSData *data = [value dataUsingEncoding:NSUTF8StringEncoding];
        OSStatus status = SecItemUpdate(
            (__bridge CFDictionaryRef)query,
            (__bridge CFDictionaryRef)@{(__bridge id)kSecValueData: data});
        if (status == errSecItemNotFound) {
            NSMutableDictionary *item = [NSMutableDictionary dictionaryWithDictionary:query];
            item[(__bridge id)kSecValueData] = data;
            item[(__bridge id)kSecAttrAccessible] = (__bridge id)kSecAttrAccessibleWhenUnlockedThisDeviceOnly;
            status = SecItemAdd((__bridge CFDictionaryRef)item, NULL);
        }
        return status == errSecSuccess;
    }
}
*/
import "C"

import "unsafe"

type appleKeychainStore struct{}

func newPlatformSecretStore() secretStore { return appleKeychainStore{} }

func (appleKeychainStore) Read(account string) (string, bool) {
	ca := C.CString(account)
	defer C.free(unsafe.Pointer(ca))
	var cv *C.char
	if C.btAIKeychainRead(ca, &cv) != 1 || cv == nil {
		return "", false
	}
	defer C.free(unsafe.Pointer(cv))
	return C.GoString(cv), true
}

func (appleKeychainStore) Write(account, value string) bool {
	ca := C.CString(account)
	cv := C.CString(value)
	defer C.free(unsafe.Pointer(ca))
	defer C.free(unsafe.Pointer(cv))
	return C.btAIKeychainWrite(ca, cv) == 1
}
