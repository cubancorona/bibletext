package bibletext

import "testing"

// The browser command Windows records for https is a template with the
// executable first and %1 for the URL; the direct launch must run exactly
// that with the URL in place.
func TestBrowserCommandLineFillsTheTemplate(t *testing.T) {
	link := "https://bibletext.co.uk/web/john/3/#v16&n=YWJj"
	for _, tc := range []struct {
		template, cmdline, exe string
	}{
		{`"C:\Program Files\Microsoft\Edge\Application\msedge.exe" --single-argument %1`,
			`"C:\Program Files\Microsoft\Edge\Application\msedge.exe" --single-argument ` + link,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`},
		{`"C:\Program Files\Google\Chrome\Application\chrome.exe" -- "%1"`,
			`"C:\Program Files\Google\Chrome\Application\chrome.exe" -- "` + link + `"`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`},
		{`"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "%1"`,
			`"C:\Program Files\Mozilla Firefox\firefox.exe" -osint -url "` + link + `"`,
			`C:\Program Files\Mozilla Firefox\firefox.exe`},
		{`C:\browsers\plain.exe %1`, `C:\browsers\plain.exe ` + link, `C:\browsers\plain.exe`},
		{`C:\browsers\noarg.exe`, `C:\browsers\noarg.exe "` + link + `"`, `C:\browsers\noarg.exe`},
	} {
		cmdline, exe, ok := browserCommandLine(tc.template, link)
		if !ok || cmdline != tc.cmdline || exe != tc.exe {
			t.Errorf("browserCommandLine(%q) = %q, %q, %v; want %q, %q", tc.template, cmdline, exe, ok, tc.cmdline, tc.exe)
		}
	}
}

func TestBrowserCommandLineRefusesWhatItCannotQuote(t *testing.T) {
	for _, tc := range []struct{ template, link string }{
		{"", "https://bibletext.co.uk/web/john/3/"},
		{`"unterminated`, "https://bibletext.co.uk/web/john/3/"},
		{`"C:\x.exe" %1`, `https://bibletext.co.uk/web/john/3/#v16" --evil`},
		{`"C:\x.exe" %1`, ""},
	} {
		if _, _, ok := browserCommandLine(tc.template, tc.link); ok {
			t.Errorf("browserCommandLine(%q, %q) accepted", tc.template, tc.link)
		}
	}
}
