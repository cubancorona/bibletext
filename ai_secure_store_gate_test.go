//go:build darwin

package bibletext

import "testing"

// The macOS gate exists because an AD-HOC signature binds the login-keychain
// ACL to a code hash that changes with every update: reads then prompt or fail
// with errSecAuthFailed. The direct-download build is ad-hoc, so it must keep
// using Preferences; only a build carrying a team identifier may use the
// Keychain.
//
// A `go test` binary is itself ad-hoc, which makes it the ad-hoc case: this
// asserts the gate is closed for it. Remove or invert the gate and macOS starts
// using the Keychain from unsigned builds, which is the regression that costs a
// reader their key rather than merely inconveniencing them.
//
// The other half — that a SIGNED build opens the gate — cannot be asserted
// here, because it needs a signing identity that CI does not have. Verify it by
// hand with:
//
//	go test -c -o /tmp/kc.test .
//	codesign -f -s "<Apple Development id>" /tmp/kc.test
//
// and confirm newPlatformSecretStore no longer returns nil.
func TestAdHocBuildsDoNotUseTheKeychain(t *testing.T) {
	if newPlatformSecretStore() != nil {
		t.Fatal("an ad-hoc-signed binary opened the Keychain: the signing-identity " +
			"gate is not being consulted. On the direct-download build that ACL is " +
			"rebound by every update, and a migration that erased the Preferences " +
			"copy would take the reader's key with it.")
	}
}
