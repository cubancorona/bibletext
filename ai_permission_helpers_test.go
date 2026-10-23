package bibletext

// The tests' two ways into a provider's permission record (ai_permission.go),
// each through the key the provider has now, as the app's own checks go.

// grantAIPermission records that the reader allowed provider id with the key
// it has now, as Allow does.
func grantAIPermission(ks *keyStore, id string) {
	ks.setAIAllowed(id, providerAPIKey(ks, id))
}

// aiPermitted reports whether provider id holds the reader's permission for
// the key it has now.
func aiPermitted(ks *keyStore, id string) bool {
	return ks.aiAllowed(id, providerAPIKey(ks, id))
}
