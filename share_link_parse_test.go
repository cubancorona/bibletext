package bibletext

// Parsing is the deep-link half of the contract. The round-trip test is the
// important one: anything ShareLinkURL can emit, ParseShareLink must read back —
// otherwise a link the app itself produced would fail to open the app.

import "testing"

func TestParseShareLink(t *testing.T) {
	for _, tc := range []struct {
		name, url string
		want      ShareTarget
		ok        bool
	}{
		{name: "canonical single verse", url: "https://bibletext.co.uk/web/john/3/#v16",
			want: ShareTarget{"web", "John", 3, 16, 0}, ok: true},
		{name: "canonical range", url: "https://bibletext.co.uk/web/john/3/#v16-18",
			want: ShareTarget{"web", "John", 3, 16, 18}, ok: true},
		{name: "chapter only", url: "https://bibletext.co.uk/bsb/psalms/23/",
			want: ShareTarget{"bsb", "Psalms", 23, 0, 0}, ok: true},
		{name: "no trailing slash", url: "https://bibletext.co.uk/web/john/3#v16",
			want: ShareTarget{"web", "John", 3, 16, 0}, ok: true},
		{name: "query alias", url: "https://bibletext.co.uk/web/john/3/?v=16-18",
			want: ShareTarget{"web", "John", 3, 16, 18}, ok: true},
		{name: "http and www", url: "http://www.bibletext.co.uk/web/john/3/#v16",
			want: ShareTarget{"web", "John", 3, 16, 0}, ok: true},
		{name: "hand-typed capitals", url: "https://BibleText.co.uk/WEB/John/3/#v16",
			want: ShareTarget{"web", "John", 3, 16, 0}, ok: true},
		{name: "numbered book", url: "https://bibletext.co.uk/web/1-corinthians/13/#v4-7",
			want: ShareTarget{"web", "1 Corinthians", 13, 4, 7}, ok: true},
		{name: "deuterocanon", url: "https://bibletext.co.uk/webc/1-maccabees/2/#v19-22",
			want: ShareTarget{"webc", "1 Maccabees", 2, 19, 22}, ok: true},

		// Forgiving: a mangled verse payload still lands on the right chapter.
		{name: "backwards range keeps the first verse", url: "https://bibletext.co.uk/web/john/3/#v18-16",
			want: ShareTarget{"web", "John", 3, 18, 0}, ok: true},
		{name: "garbage fragment ignored", url: "https://bibletext.co.uk/web/john/3/#vfoo",
			want: ShareTarget{"web", "John", 3, 0, 0}, ok: true},
		{name: "zero verse ignored", url: "https://bibletext.co.uk/web/john/3/#v0",
			want: ShareTarget{"web", "John", 3, 0, 0}, ok: true},

		// NOT ours, or not a passage — these must stay in the browser. The
		// privacy and support pages are the URLs App Store Connect points at;
		// swallowing them into the app would be a real bug.
		{name: "privacy page", url: "https://bibletext.co.uk/privacy.html"},
		{name: "support page", url: "https://bibletext.co.uk/support.html"},
		{name: "landing page", url: "https://bibletext.co.uk/"},
		{name: "version index", url: "https://bibletext.co.uk/web/"},
		{name: "book index", url: "https://bibletext.co.uk/web/john/"},
		{name: "another site", url: "https://example.com/web/john/3/#v16"},
		{name: "licensed version id", url: "https://bibletext.co.uk/nkjv/john/3/#v16"},
		{name: "unknown book", url: "https://bibletext.co.uk/web/nowhere/3/"},
		{name: "non-numeric chapter", url: "https://bibletext.co.uk/web/john/three/"},
		{name: "empty", url: ""},
	} {
		got, ok := ParseShareLink(tc.url)
		if ok != tc.ok {
			t.Errorf("%s: ok = %v, want %v (%q)", tc.name, ok, tc.ok, tc.url)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%s:\n got %+v\nwant %+v", tc.name, got, tc.want)
		}
	}
}

// TestShareLinkRoundTrip: every link the app can produce must parse back to the
// passage it came from. A failure here means the app would share a link its own
// deep-link handler could not open.
func TestShareLinkRoundTrip(t *testing.T) {
	books := append(append([]string{}, NewBibleData().Books...), catholicBooks...)
	for _, book := range books {
		for _, v := range []string{"web", "bsb", "webc"} {
			for _, span := range [][2]int{{0, 0}, {16, 0}, {16, 18}} {
				url := ShareLinkURL(v, book, 3, span[0], span[1])
				if url == "" {
					t.Fatalf("%s/%s produced no link", v, book)
				}
				got, ok := ParseShareLink(url)
				if !ok {
					t.Errorf("%q did not parse back", url)
					continue
				}
				if got.Book != book || got.Chapter != 3 {
					t.Errorf("%q -> %+v, want book %q chapter 3", url, got, book)
				}
				if got.VerseLo != span[0] || got.VerseHi != span[1] {
					t.Errorf("%q -> verses %d-%d, want %d-%d", url, got.VerseLo, got.VerseHi, span[0], span[1])
				}
				// The version may legitimately be rewritten to webc for a
				// deuterocanonical book; otherwise it must survive.
				if got.VersionID != v && got.VersionID != "webc" {
					t.Errorf("%q -> version %q, want %q", url, got.VersionID, v)
				}
			}
		}
	}
}
