package bibletext

// Who a note is from — the ONE place that question is answered (S9,
// [redacted-retired-private-reference] "Identity — the decision record").
//
// Today the answer is a constant: "Friend" for a received note, "you" for the
// reader's own. No name is collected, no name is displayed, and the share
// sheet has no name field. But SenderName is CARRIED — decoded off the wire
// and stored since the record codec landed — and the standing decision is to
// design and implement ahead so a name field later is a switch-flip, not a
// migration. So the display path exists NOW, built and table-tested, gated
// behind one build constant that nothing sets:
//
//	senderNamesEnabled = false
//
// Every surface that attributes a note — the Fyne banner byline, the notes
// browser byline (both via noteByline, notes_mine.go), and the Apple panes'
// native WHO line (senderByline via appleStickerPush, notes_plan.go) — routes
// through senderName below, so turning names on is that one constant and no
// surface can be forgotten.
//
// THE DISPLAY RULES (all in senderNameWithFlag, so they cannot drift from the
// flag): a claimed name is UNTRUSTED, attacker-chosen text, and it is
//
//   - ONE LINE — newlines, tabs and every control/bidi-steering character are
//     stripped the way normalizeNote strips them, then whitespace runs
//     collapse;
//   - LENGTH-CAPPED at 24 runes (the spec's cap for the reserved 'f' record (docs/NOTE_WIRE_FORMAT.md: ≤24 runes); the codec does not decode 'f' yet, so THIS function is where the cap actually lives), counted in runes so
//     the limit means the same thing in every script;
//   - BIDI-ISOLATED — wrapped in U+2066 (LRI) / U+2069 (PDI) so an RTL name
//     cannot reorder the chrome around it. The steering characters INSIDE the
//     name were already stripped; the isolates are ours, added last;
//   - REFUSED — the note falls back to "Friend" — when the name impersonates
//     the app's own chrome (case-folds to "BibleText", "Note", "Notes" or
//     "BibleText Support", or contains "bibletext" once spaces are gone: the
//     phishing case docs/SHARED_NOTES.md names), or when it carries a URL-ish
//     token (a scheme, a www. prefix, or a dotted host like "evil.com" — a
//     byline must never be a place to smuggle a link). Homoglyphs are NOT
//     filtered ("Аnna" with a Cyrillic А passes): they are also legitimate
//     names, and the isolation + quiet styling is the honest defence.

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// senderNamesEnabled is the build flag that turns names on when the name
// field ships. While false, every byline says "Friend" whatever a record carries —
// the dormant path below is compiled, tested and unreachable.
const senderNamesEnabled = false

// senderNameMaxRunes matches the wire's 'f' record cap.
const senderNameMaxRunes = 24

// senderName is who a RECEIVED note is from, ready for display: "Friend"
// today, and under the flag the note's claimed name with every display rule
// applied. Never empty.
func senderName(n StoredNote) string {
	return senderNameWithFlag(n, senderNamesEnabled)
}

// senderNameWithFlag is senderName with the gate as an argument, so the
// dormant branch is testable while the constant stays false.
func senderNameWithFlag(n StoredNote, enabled bool) string {
	if !enabled {
		return "Friend"
	}
	name := sanitizeSenderName(n.SenderName)
	if name == "" || senderNameRefused(name) {
		return "Friend"
	}
	if utf8.RuneCountInString(name) > senderNameMaxRunes {
		r := []rune(name)
		name = strings.TrimSpace(string(r[:senderNameMaxRunes]))
	}
	// The isolates go on LAST, after every judgement — they are the app's own
	// framing, not part of the name, and must never reach the refusal checks.
	return "⁦" + name + "⁩"
}

// senderByline is the Apple panes' WHO-line base — the attribution half of the
// sticker's chrome, before any counts join it (appleStickerPush).
// "Note from Friend" today, exactly the literal it replaces.
func senderByline(n StoredNote) string {
	if n.Kind == noteKindMine {
		// Own notes are never drawn in the reading text — that exclusion is
		// deliberate — so this arm serves any future surface that attributes one
		// natively.
		return "Note from you"
	}
	return "Note from " + senderName(n)
}

// sanitizeSenderName makes a claimed name one clean line: newlines and tabs
// become spaces, C0/C1/DEL and invalid UTF-8 go, EVERY Unicode format
// character (class Cf) goes, and the app's own separator grammar is not
// available to the name.
//
// Cf AS A CLASS, not an enumerated list. The first version stripped the bidi
// marks and the BOM by name, and implementation verification it with U+200B: a name
// spelled "Bible<ZWSP>Text Support" case-folds as "bible…text support",
// slipping the impersonation refusal, while RENDERING pixel-identically to
// the refused string. Zero-width and joiner characters have no legitimate
// place in a display name, and enumerating bad invisibles is a list that is
// wrong the day Unicode grows — the class is the honest predicate.
//
// The MIDDLE DOT becomes a hyphen for the same reason in the other direction:
// " · " is the who-line's own separator, so a name containing it could dress
// itself as the app's counts ("Amy · 9 not shown here"), and the truncation
// rule — which protects everything after the FIRST " · " — would then
// preserve the forged half as if it were chrome. A name may not speak in the
// chrome's grammar.
func sanitizeSenderName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(' ') // a name is ONE line, unlike a note
		case r == '\u00b7':
			b.WriteRune('-')
		case r == utf8.RuneError:
		case r < 0x20 || r == 0x7f:
		case r >= 0x80 && r <= 0x9f:
		case unicode.Is(unicode.Cf, r):
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// senderNameRefused is the impersonation and link check, on the sanitized
// name. Refusal falls back to "Friend" — the note still shows; only the name
// is declined.
func senderNameRefused(name string) bool {
	folded := strings.ToLower(name)
	switch folded {
	case "bibletext", "note", "notes", "bibletext support":
		return true
	}
	// "BibleText Security", "bible text", "[redacted-fixture-name] BibleText" — anything that
	// works the app's name into its own once spaces are gone. A blocklist can
	// honestly catch exactly this impersonation and no other; wider claims
	// (homoglyphs) are left to display, per the design doc.
	if strings.Contains(strings.ReplaceAll(folded, " ", ""), "bibletext") {
		return true
	}
	return senderNameURLish(folded)
}

// senderNameURLish reports whether any token of the (folded) name reads as a
// link: an explicit scheme, a www. prefix, or a dotted host — a '.' followed
// by two or more ASCII letters that end the token or precede a '/'
// ("evil.com", "bibletext.co.uk/web"). Initials ("J. R. Tolkien") pass: their
// dots are followed by at most one letter.
func senderNameURLish(folded string) bool {
	for _, tok := range strings.Fields(folded) {
		if strings.Contains(tok, "://") ||
			strings.HasPrefix(tok, "www.") ||
			strings.HasPrefix(tok, "http:") || strings.HasPrefix(tok, "https:") {
			return true
		}
		for i := 0; i < len(tok); i++ {
			if tok[i] != '.' {
				continue
			}
			j := i + 1
			for j < len(tok) && tok[j] >= 'a' && tok[j] <= 'z' {
				j++
			}
			if j-i-1 >= 2 && (j == len(tok) || tok[j] == '/') {
				return true
			}
		}
	}
	return false
}
