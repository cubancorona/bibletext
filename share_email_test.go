//go:build !ios && !android

package bibletext

// The mail decisions behind Email… on the desktop share confirmation
// (share_email.go): the mailto: link's length, which route may follow a
// failed one, what the Email portal's answer means, and when Linux offers
// the button. They are pure, so every platform's run holds them, not only
// the Linux job's.

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// mailtoBody is the body a mailto: link carries, decoded.
func mailtoBody(t *testing.T, u *url.URL) string {
	t.Helper()
	_, enc, ok := strings.Cut(u.Opaque, "&body=")
	if !ok {
		t.Fatalf("no body in %q", u.String())
	}
	body, err := url.QueryUnescape(enc)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// A LONG SHARE IS CUT TO A LINK OF 2,000 CHARACTERS, the passage at a word
// with an ellipsis inside its closing quotation mark, the citation (and a
// link share's link) kept whole; a short one is left as it is. Mutations: no
// cap; the cap cutting the citation; the cut falling mid-word.
func TestAMailtoLinkIsKeptTo2000Characters(t *testing.T) {
	passage := strings.TrimSpace(strings.Repeat("Jesus answered, “Most certainly I tell you, unless one is born anew, he can’t see God’s Kingdom.” ", 60))
	link := ShareLinkURL(defaultVersionID, "John", 3, 1, 36)
	for _, c := range []struct {
		name, body, tail string
		quoted           bool
	}{
		{"citation", composeShareText("“"+passage+"”", "John 3:1–36", "World English Bible"),
			"\n\n— John 3:1–36 (World English Bible)", true},
		{"note", passage + "\n\nJohn 3:1–36 (World English Bible)\n" + link,
			"\n\nJohn 3:1–36 (World English Bible)\n" + link, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			u := mailtoURL("John 3:1–36 (World English Bible)", c.body)
			if n := len(u.String()); n > mailtoMaxLen {
				t.Fatalf("the link is %d characters, over %d", n, mailtoMaxLen)
			}
			if n := len(u.String()); n < mailtoMaxLen-200 {
				t.Errorf("the link is %d characters: cut further than it needed to be", n)
			}
			body := strings.ReplaceAll(mailtoBody(t, u), "\r\n", "\n")
			if !strings.HasSuffix(body, c.tail) {
				t.Fatalf("the cut body does not end in the whole citation %q: …%q", c.tail, body[max(0, len(body)-120):])
			}
			head := strings.TrimSuffix(body, c.tail)
			wantEnd := "…"
			if c.quoted {
				wantEnd = "…”"
			}
			if !strings.HasSuffix(head, wantEnd) {
				t.Errorf("the passage ends %q, want %q", head[max(0, len(head)-30):], wantEnd)
			}
			kept := strings.TrimSuffix(head, wantEnd)
			if !strings.HasPrefix(c.body, kept) || !utf8.ValidString(kept) {
				t.Fatalf("the kept passage is not the start of the share")
			}
			if next := c.body[len(kept):]; next != "" && !strings.ContainsAny(next[:1], " \n,") {
				t.Errorf("the cut fell mid-word: …%q | %q…", kept[max(0, len(kept)-20):], next[:min(20, len(next))])
			}
		})
	}
	t.Run("a short share is left whole", func(t *testing.T) {
		body := composeShareText("“For God so loved the world.”", "John 3:16", "World English Bible")
		if got := strings.ReplaceAll(mailtoBody(t, mailtoURL("John 3:16 (World English Bible)", body)), "\r\n", "\n"); got != body {
			t.Errorf("a short body came back as %q", got)
		}
	})
}

// ONLY "NOTHING TO COMPOSE WITH" HANDS ON TO THE NEXT ROUTE. The Email
// portal's 2, or a route that reached nothing (no portal, no tool), tries
// the next; the reader cancelling (1), any other answer, or a portal that
// took the call and never answered ends the compose, so a late compose from
// the portal is never joined by a second. Mutations: every non-zero answer
// falling through, as it did; composeByRoutes ignoring a stop.
func TestOnlyNothingToComposeWithFallsThrough(t *testing.T) {
	if portalEmailResponse(0) != nil {
		t.Error("0 must be success")
	}
	noPortal := errors.New("no session bus")
	for _, c := range []struct {
		name      string
		first     error
		wantTried int
		wantOK    bool
	}{
		{"success", nil, 1, true},
		{"2: no mail client", portalEmailResponse(2), 3, false},
		{"no portal at all", noPortal, 3, false},
		{"1: the reader cancelled", portalEmailResponse(1), 1, false},
		{"3: an answer not in the specification", portalEmailResponse(3), 1, false},
		{"no answer after the call was taken", mailRouteStop{errors.New("no answer from the Email portal")}, 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tried := 0
			route := func(err error) mailRoute {
				return func(string, string, string) error { tried++; return err }
			}
			err := composeByRoutes([]mailRoute{route(c.first), route(errors.New("no xdg-email")), route(errors.New("no opener"))}, "s", "b", "")
			if tried != c.wantTried {
				t.Errorf("%d routes tried, want %d", tried, c.wantTried)
			}
			if (err == nil) != c.wantOK {
				t.Errorf("composeByRoutes = %v, want success %v", err, c.wantOK)
			}
			if c.first != nil && !errors.Is(err, c.first) {
				t.Errorf("the first route's error %v is not in %v", c.first, err)
			}
		})
	}
	t.Run("a later route succeeding", func(t *testing.T) {
		calls := 0
		err := composeByRoutes([]mailRoute{
			func(string, string, string) error { calls++; return portalEmailResponse(2) },
			func(string, string, string) error { calls++; return nil },
			func(string, string, string) error { calls++; return nil },
		}, "s", "b", "")
		if err != nil || calls != 2 {
			t.Errorf("composeByRoutes = %v after %d routes, want success after 2", err, calls)
		}
	})
}

// writeDesktopEntry writes applications/<id> under dir.
func writeDesktopEntry(t *testing.T, dir, id, entry string) {
	t.Helper()
	path := filepath.Join(dir, "applications", id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
}

// THE HANDLER'S DESKTOP ENTRY SAYS WHETHER IT IS A MAIL CLIENT: Categories
// naming Email and not WebBrowser, in the [Desktop Entry] group, in the
// first data directory that has the entry. Mutations: the browser test
// dropped (a browser that also lists Email passes); another group's
// Categories read; the system entry read before the user's.
func TestTheMailtoHandlerIsReadFromItsDesktopEntry(t *testing.T) {
	user, system := t.TempDir(), t.TempDir()
	dirs := []string{user, system}
	writeDesktopEntry(t, system, "fixture-browser.desktop",
		"[Desktop Entry]\nName=Fixture Browser\nCategories=GNOME;GTK;Network;WebBrowser;\nMimeType=x-scheme-handler/mailto;\n")
	writeDesktopEntry(t, system, "fixture-mail.desktop",
		"[Desktop Entry]\nName=Fixture Mail\nCategories=Network;Email;\n")
	writeDesktopEntry(t, system, "fixture-both.desktop",
		"[Desktop Entry]\nName=Fixture Suite\nCategories=Network;Email;WebBrowser;\n")
	writeDesktopEntry(t, system, "fixture-action.desktop",
		"[Desktop Action compose]\nCategories=Email;\n\n[Desktop Entry]\nName=Fixture Action\nCategories=Utility;\n")
	writeDesktopEntry(t, system, "fixture-override.desktop", "[Desktop Entry]\nCategories=Email;\n")
	writeDesktopEntry(t, user, "fixture-override.desktop", "[Desktop Entry]\nCategories=WebBrowser;\n")
	for _, c := range []struct {
		id   string
		want mailHandler
	}{
		{"", mailHandlerNone},
		{"  \n", mailHandlerNone},
		{"fixture-browser.desktop\n", mailHandlerOther},
		{"fixture-mail.desktop\n", mailHandlerMailClient},
		{"fixture-both.desktop", mailHandlerOther},
		{"fixture-action.desktop", mailHandlerOther},
		{"fixture-override.desktop", mailHandlerOther},
		{"fixture-missing.desktop", mailHandlerOther},
	} {
		if got := mailHandlerOf(c.id, dirs); got != c.want {
			t.Errorf("mailHandlerOf(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

// WHEN LINUX OFFERS EMAIL…. Text: the portal's SchemeSupported wherever it
// answers; otherwise, in a sandbox, never (xdg-mime there sees the sandbox's
// handler, not the desktop's), and outside one only when xdg-mime names a
// handler. Image: only outside a sandbox, with a route that takes a file,
// and a mail client as the handler — never a browser, which drops the
// picture. Mutations: the image offered with a browser as the handler (as
// it was); the sandbox trusting xdg-mime; a handler that could not be asked
// taken for one that exists.
func TestWhenLinuxOffersEmail(t *testing.T) {
	stockUbuntu := linuxMailFacts{portalEmail: true, xdgEmail: true, handler: mailHandlerOther}
	withClient := linuxMailFacts{portalEmail: true, xdgEmail: true, handler: mailHandlerMailClient}
	for _, c := range []struct {
		name        string
		f           linuxMailFacts
		text, image bool
	}{
		{"a stock Ubuntu desktop: portal 1.18, Firefox handles mailto:", stockUbuntu, true, false},
		{"a mail client handles mailto:", withClient, true, true},
		{"no handler named", linuxMailFacts{portalEmail: true, xdgEmail: true, handler: mailHandlerNone}, false, false},
		{"no xdg-mime to ask", linuxMailFacts{portalEmail: true, xdgEmail: false, handler: mailHandlerUnknown}, false, false},
		{"a mail client, but no route that takes a file", linuxMailFacts{handler: mailHandlerMailClient}, true, false},
		{"a mail client, xdg-email and no portal", linuxMailFacts{xdgEmail: true, handler: mailHandlerMailClient}, true, true},
		{"portal 1.19.1 says yes", linuxMailFacts{schemeKnown: true, schemeSupported: true, portalEmail: true, handler: mailHandlerNone}, true, false},
		{"portal 1.19.1 says no", linuxMailFacts{schemeKnown: true, portalEmail: true, handler: mailHandlerMailClient}, false, true},
		{"the snap on Ubuntu 24.04: no SchemeSupported", linuxMailFacts{confined: true, portalEmail: true, handler: mailHandlerMailClient}, false, false},
		{"the snap on a newer portal that says yes", linuxMailFacts{confined: true, schemeKnown: true, schemeSupported: true, portalEmail: true}, true, false},
		{"the snap on a newer portal that says no", linuxMailFacts{confined: true, schemeKnown: true, portalEmail: true}, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := linuxMailOffered(false, c.f); got != c.text {
				t.Errorf("text: offered %v, want %v (%+v)", got, c.text, c.f)
			}
			if got := linuxMailOffered(true, c.f); got != c.image {
				t.Errorf("image: offered %v, want %v (%+v)", got, c.image, c.f)
			}
		})
	}
}

func (m mailHandler) String() string {
	return [...]string{"unknown", "none", "other", "mail client"}[m] + fmt.Sprintf("(%d)", int(m))
}
