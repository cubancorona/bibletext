//go:build bibletextdev

package bibletext

// The dev link-testing page. DEVELOPMENT BUILDS ONLY — see dev_links_off.go for
// why this is a build tag rather than a runtime flag.
//
// WHY IT EXISTS. The one path that matters most for shared notes is the hardest
// to exercise: a universal link cannot be triggered in the simulator at all, and
// on a device it needs a tap from inside another app, which rules out any
// scripted test. So the scenarios here call HandleShareLink — the REAL entry
// point the OS calls — with URLs built by the app's own ShareLinkURLWithNote.
// That means each row exercises the genuine chain end to end: encode → parse →
// notes gate → the offer dialog → apply → store → render. Nothing is mocked, and
// a scenario that passes here is the same code that runs when a link is tapped.
//
// The deliberately-malformed URLs are written out by hand, because the builder
// cannot produce them — that is rather the point of testing them.

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

const devLinksEnabled = true

// devLinksScrollY remembers how far down the scenario list the reader had got.
// A package var rather than AppState: this page does not exist in a shipping
// build, and its state has no business widening a type the whole app shares.
var devLinksScrollY float32

type devScenario struct {
	name string
	what string // what to look for — the expected result, in one line
	url  string
}

// longNote is a note at the 280-rune cap, to see the bubble at its tallest.
func devLongNote() string {
	s := "This is what a note looks like when somebody uses every character they are " +
		"given, which is worth seeing because the bubble has to reserve its own band in " +
		"the text and the band is measured from the height of this. Read it slowly and " +
		"check nothing below is covered up. "
	r := []rune(s)
	for len(r) < NoteMaxRunes {
		r = append(r, 'x')
	}
	return string(r[:NoteMaxRunes])
}

func devScenarios() []devScenario {
	n := func(book string, ch, lo, hi int, note string) string {
		return ShareLinkURLWithNote("bsb", book, ch, lo, hi, note)
	}
	plain := func(book string, ch, lo, hi int) string {
		return ShareLinkURL("bsb", book, ch, lo, hi)
	}
	return []devScenario{
		// --- the ordinary cases -------------------------------------------------
		{"Note, mid-chapter, long paragraph", "Bubble above v34's paragraph; v35 highlighted; the note is what you land on",
			n("John", 11, 35, 35, "Read this synthetic note this morning and synthetic note.\n\nRinging on Sunday. synthetic note.")},
		{"Note on the FIRST paragraph", "Bubble above v1 — the container-inset path, not paragraphSpacingBefore",
			n("Psalms", 23, 1, 4, "This got me through last night. synthetic note both.")},
		{"Note deep in a long chapter", "Psalm 119 is 176 verses: check it lands on the note, not the top",
			n("Psalms", 119, 105, 105, "Your word is a lamp — this is the one I was trying to remember.")},
		{"Note on a verse range", "vv.1-4 all highlighted, not just the first",
			n("Romans", 8, 1, 4, "The whole passage, not just the famous line.")},
		{"Note on a whole chapter (no verse)", "No highlight at all; bubble at the top",
			n("Philippians", 4, 0, 0, "All of it. Read it twice.")},
		{"Plain link, NO note", "Straight to the verse, no bubble anywhere",
			plain("John", 3, 16, 16)},
		{"Plain link, chapter only", "Chapter opens at the top, nothing highlighted",
			plain("Genesis", 1, 0, 0)},

		// --- the note text itself ------------------------------------------------
		{"Note at the 280-rune cap", "The tallest bubble there can be; nothing below it covered",
			n("Isaiah", 40, 31, 31, devLongNote())},
		{"Note with blank lines", "Paragraph breaks kept; 3+ newlines collapsed",
			n("Matthew", 5, 3, 10, "One.\n\n\n\n\nTwo, after far too many blank lines.")},
		{"Note with emoji + accents", "Renders as text; no tofu, no mis-measured band",
			n("Psalms", 121, 1, 2, "Café ☕ — synthetic note 🙏 grüße!")},
		{"Note trying to inject markup", "Must appear LITERALLY, never rendered as markup",
			n("John", 1, 1, 1, "<b>bold?</b> <script>alert(1)</script> [link](http://example.com)")},
		{"Note with bidi control characters", "The overrides are stripped by normalizeNote; text reads normally",
			n("Proverbs", 3, 5, 6, "Trust ‮reversed?‬ and lean not ‏on‎ your own understanding.")},
		{"Note that is only whitespace", "Treated as NO note — plain link behaviour",
			n("Ruth", 1, 16, 16, "   \n\n   \t  ")},

		// --- the malformed and the hostile ---------------------------------------
		{"Unknown fragment keys", "Unknown keys are IGNORED, never rejected: v16 still highlights",
			"https://bibletext.co.uk/bsb/john/3/#v16&zz=nonsense&n2=whatever"},
		{"Verse beyond the end of the chapter", "Highlights what exists; must not error or land blank",
			plain("Jude", 1, 900, 950)},
		{"Reversed range (hi < lo)", "Must not crash or highlight backwards",
			"https://bibletext.co.uk/bsb/john/3/#v18-16"},
		{"Unknown book slug", "Declined — the app should not navigate anywhere",
			"https://bibletext.co.uk/bsb/nosuchbook/3/#v16"},
		{"Corrupt note payload", "Note unreadable → passage still opens, no bubble",
			"https://bibletext.co.uk/bsb/john/3/#v16&n=!!!!not-base64!!!!"},
		{"Wrong host", "Declined outright",
			"https://example.com/bsb/john/3/#v16"},

		// --- other translations ---------------------------------------------------
		{"Note in the WEB translation", "Switches translation as well as passage",
			ShareLinkURLWithNote("web", "John", 14, 6, 6, "The WEB wording of this one.")},
		{"Note in the Catholic canon (deuterocanon)", "Tobit only exists in WEBC — check the switch",
			ShareLinkURLWithNote("webc", "Tobit", 4, 15, 15, "A deuterocanonical note.")},
	}
}

// buildDevLinksTab is the page itself: the switches at the top, then one row per
// scenario. Each row's button is the whole point — it calls HandleShareLink
// exactly as the OS does.
func buildDevLinksTab(state *AppState, switchToRead func()) fyne.CanvasObject {
	pal := state.pal()

	title := canvas.NewText("Link scenarios (dev build)", pal.Text)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 20

	status := canvas.NewText("", pal.TextMuted)
	status.TextSize = 12
	refreshStatus := func() {
		on := "off"
		if notesEnabled() {
			on = "on"
		}
		status.Text = "Notes are " + on + " · " + itoa(len(readNotes(appPrefs()))) + " stored"
		status.Refresh()
	}
	refreshStatus()

	notesSwitch := widget.NewCheck("Shared notes on", func(b bool) {
		setNotesEnabled(b)
		refreshStatus()
	})
	notesSwitch.SetChecked(notesEnabled())

	wipe := widget.NewButton("Delete all stored notes", func() {
		deleteAllNotes(appPrefs())
		state.ActiveNote, state.NoteMinimized, state.NoteVerseLo = "", false, 0
		refreshStatus()
		state.refresh()
	})
	wipe.Importance = widget.DangerImportance

	// Wrapping must be set explicitly — widget.Label does not wrap by default, and
	// an unwrapped one reports its whole single line as its MinSize, which is how
	// this line ran off the side of the screen.
	blurb := widget.NewLabel("Each button calls HandleShareLink with a real URL — " +
		"the same entry point the OS uses when a link is tapped.")
	blurb.Wrapping = fyne.TextWrapWord

	// An Entry PRE-FILLED with emoji, to separate two very different faults when
	// someone reports "the emoji did not appear in the note box": whether a Fyne
	// Entry can DRAW emoji at all, or whether the iOS emoji keyboard never
	// delivers them. A Label renders them fine, and Entry is a different widget.
	// Type beside them to compare what arrives with what was already there.
	emojiProbe := widget.NewEntry()
	emojiProbe.SetText("prefilled 🤏 🥺 🫶 👊 ☕ — type here →")

	head := container.NewVBox(
		title, blurb,
		notesSwitch, wipe, status,
		widget.NewLabel("Emoji probe (Entry vs Label):"),
		widget.NewLabel("label 🤏 🥺 🫶 👊 ☕"),
		emojiProbe,
		widget.NewSeparator(),
	)

	column := container.NewVBox()
	for _, sc := range devScenarios() {
		sc := sc
		name := canvas.NewText(sc.name, pal.Accent)
		name.TextStyle = fyne.TextStyle{Bold: true}
		name.TextSize = 16

		expect := widget.NewLabel(sc.what)
		expect.Wrapping = fyne.TextWrapWord

		// The URL, shown so a failure can be reproduced outside the app.
		u := canvas.NewText(shortenForDev(sc.url), pal.TextMuted)
		u.TextSize = 10

		inApp := widget.NewButton("Open in app", func() {
			// Will this link raise the three-way offer instead of navigating?
			// (Notes off + the link genuinely carries a note.) The offer is a
			// modal on THIS canvas, and switchToRead's window rebuild drains
			// overlays — switching would destroy the question before a single
			// frame of it was drawn. Stay put; the offer does its own
			// navigating through whichever door the reader picks.
			t, parsed := ParseShareLink(sc.url)
			offerWillShow := parsed && t.Note != "" && !notesEnabled()
			if !HandleShareLink(state, sc.url) {
				// Declined is a legitimate outcome for the malformed cases —
				// say so rather than leaving the tap looking broken.
				status.Text = "Declined: " + sc.name
				status.Refresh()
				return
			}
			refreshStatus()
			if !offerWillShow && switchToRead != nil {
				switchToRead()
			}
		})
		inApp.Importance = widget.HighImportance

		// The same URL handed to the browser, so the two renderings of one link
		// can be compared without retyping it. This is how the notched-highlight
		// report got settled: the app and the web turned out to have the same
		// defect for the same reason, which is far easier to see side by side
		// than to reason about from one screenshot.
		inBrowser := widget.NewButton("Open in browser", func() {
			openLinkInBrowser(sc.url)
		})

		column.Add(container.NewVBox(
			name, expect, u,
			container.NewHBox(inApp, inBrowser),
			widget.NewSeparator(),
		))
	}

	// squeezeWidthLayout on both halves, for the reason sheet_fit.go documents: a
	// scroll widens its content to the content's MinSize and clips the overflow
	// sideways, and a wrapping Label's MinSize is wider than a phone. Without it
	// this page clipped its own descriptions — which would be a poor advert for a
	// page whose job is finding exactly that.
	scroll := container.NewVScroll(container.New(squeezeWidthLayout{}, container.NewPadded(column)))

	// Keep the reader's place in the list. Opening a scenario switches to the Read
	// tab, and coming back rebuilds this page from scratch (that is how the mobile
	// tab bar works), so without this every check sent you back to scenario one —
	// and the whole point is working DOWN a list of twenty-one.
	scroll.Offset = fyne.NewPos(0, devLinksScrollY)
	scroll.OnScrolled = func(p fyne.Position) { devLinksScrollY = p.Y }

	return container.NewBorder(
		container.New(squeezeWidthLayout{}, container.NewPadded(head)), nil, nil, nil,
		scroll)
}

// shortenForDev keeps a long note payload from turning the row into a wall.
func shortenForDev(u string) string {
	if i := strings.Index(u, "&n="); i >= 0 && len(u) > i+22 {
		return u[:i+14] + "…(" + itoa(len(u)-i-3) + " chars)"
	}
	return u
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}
