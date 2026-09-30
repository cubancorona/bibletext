//go:build !linux && !windows && !ios && !android

package bibletext

// Email… where the desktop fallback bodies compile but neither Linux nor
// Windows is the platform: macOS running the platform-mimic dev mode down
// the Windows/Linux share path (dev_mimic_on.go), and any other desktop. A
// mailto: link through the platform's own opener (external_link.go), which
// carries no file, so the image share offers no Email… here.

import "errors"

func shareEmailAvailable(withAttachment bool) bool { return !withAttachment }

func composeShareEmail(subject, body, attachment string) error {
	if attachment != "" {
		return errors.New("a mailto: link carries no attachment")
	}
	return externalOpener(mailtoURL(mailSubjectLine(subject), body))
}
