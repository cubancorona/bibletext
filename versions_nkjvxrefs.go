//go:build nkjvxrefs

package bibletext

// THE NKJV'S CROSS REFERENCES, BEHIND A BUILD TAG.
//
// The feed carries the edition's cross-reference apparatus and the app has
// captured all of it (publisher_xrefs.go explains the cards). Whether a
// licensed publisher's apparatus may be DISPLAYED is a licensing question put
// to HarperCollins' permissions team on 28 August 2026 and not yet answered
// (an email to API.Bible support was approved on 6 November 2026 but had not
// been sent as of that date), so the display is compiled only on request:
//
//	go build -tags nkjvxrefs ./...   (and --tags nkjvxrefs for fyne package)
//
// A build tag rather than a setting, for the same reason the NRSV and LSB sit
// behind theirs (versions_nrsv.go): a setting, even off by default, ships the
// capability the enquiry is about; scripts/verify-not-next.sh refuses a
// release artifact built with the tag. On a yes this file's one line moves
// into the registry entry and the tag goes; on a no nothing needs unwinding.

func init() {
	for i := range registeredVersions {
		if registeredVersions[i].ID == "nkjv" {
			registeredVersions[i].PublisherCrossRefs = true
		}
	}
}
