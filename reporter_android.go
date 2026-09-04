//go:build android

package bibletext

// reporterLayoutActive reports whether the reading pane should use the
// U.S. Reports-style page layout (the reporter* constants in reading.go): an
// Android phone in landscape reading mode with its typography half on
// (phone_landscape.go, both on by default).
//
// Not tablets, unlike iOS. An Android tablet keeps the phone page in every
// orientation because nothing has measured the reporter page against a
// tablet's width here, and the landscape mode excludes tablets by construction
// anyway (deviceIsTablet, device_android.go) — so this answers for phones
// only, and says so rather than inheriting the iPad's rule untested.
//
// The three halves of the page reach the pane by different routes and all
// three ask THIS question: the paragraph grammar is markup
// (android_chapter_html.go), the measure is pushed to the bridge and centred
// there (reading_android.go, BtBridge.applyReadingPadding), and the leading is
// deliberately unchanged — see the measured note at the setStyle push.
func reporterLayoutActive() bool {
	return phoneLandscapeTypographyEnabled() && phoneLandscapeReading()
}
