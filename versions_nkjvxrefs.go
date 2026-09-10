//go:build nkjvxrefs

package bibletext

// THE NKJV'S CROSS REFERENCES, BEHIND A BUILD TAG.
//
// The feed carries the edition's cross-reference apparatus and the app has
// captured all of it (publisher_xrefs.go explains the rows). Whether a
// licensed publisher's apparatus may be DISPLAYED is a licensing question that
// has been put to API.Bible support and not yet answered, so the display is
// compiled only on request:
//
//	go build -tags nkjvxrefs ./...   (and --tags nkjvxrefs for fyne package)
//
// A build tag rather than a setting, for the same reason the NRSV and LSB sit
// behind theirs (versions_nrsv.go): a setting, even off by default, ships the
// capability the enquiry is about. On a yes this file's one line moves into
// the registry entry and the tag goes; on a no nothing needs unwinding.

func init() {
	for i := range registeredVersions {
		if registeredVersions[i].ID == "nkjv" {
			registeredVersions[i].PublisherCrossRefs = true
		}
	}
}
