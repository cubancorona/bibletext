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
	"fmt"
	"os"
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
	s := "Fixture long-note text alpha beta gamma delta epsilon zeta eta theta iota " +
		"kappa lambda mu nu xi omicron pi rho sigma tau upsilon phi chi psi omega. " +
		"Fixture continuation alpha beta gamma delta epsilon zeta eta theta iota. "
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
			n("John", 11, 35, 35, "Fixture paragraph alpha beta gamma.\n\nFixture paragraph delta epsilon.")},
		{"A SECOND note on the same passage (S8/S9)", "Open the row above first. The bubble shows THIS note alone; the WHO line above it reads “Note from Friend · 1 of 2 on this passage” — the honest count in the app's own chrome",
			ShareLinkURLWithNote("web", "John", 11, 35, 35, "Fixture same-range message two.")},
		{"THREE notes, SAME verse range (S10)", "Open all three rows in order. One bubble at a time; the who line counts “1 of 3 on this passage ›” and tapping the count cycles them — same range, so the wash never moves",
			n("John", 3, 16, 16, "Fixture same-range message one.")},
		{"…same range №2", "Second of the trio above — open after №1",
			n("John", 3, 16, 16, "Fixture same-range message two.")},
		{"…same range №3", "Third of the trio — after this one the count reads 1 of 3",
			n("John", 3, 16, 16, "Fixture same-range message three.")},
		{"THREE notes, ONE paragraph, DIFFERENT ranges (S10)", "WEB John 3:14-17 is one paragraph. Open all three; the count-tap walks them and the wash moves WITHIN the paragraph: 14-15 → 16 → 16-17 (two overlap at v16)",
			ShareLinkURLWithNote("web", "John", 3, 14, 15, "Fixture range message 14-15.")},
		{"…same paragraph, v16 alone", "Second range in the paragraph trio",
			ShareLinkURLWithNote("web", "John", 3, 16, 16, "Fixture range message 16.")},
		{"…same paragraph, vv16-17", "Third range — overlaps the one above at v16",
			ShareLinkURLWithNote("web", "John", 3, 16, 17, "Fixture range message 16-17.")},
		{"Note on the FIRST paragraph", "Bubble above v1 — the container-inset path, not paragraphSpacingBefore",
			n("Psalms", 23, 1, 4, "Fixture message alpha beta gamma delta epsilon zeta.")},
		{"Note deep in a long chapter", "Psalm 119 is 176 verses: check it lands on the note, not the top",
			n("Psalms", 119, 105, 105, "Fixture deep-chapter message alpha.")},
		{"Note on a verse range", "vv.1-4 all highlighted, not just the first",
			n("Romans", 8, 1, 4, "Fixture verse-range message alpha.")},
		{"Note on a whole chapter (no verse)", "No highlight at all; bubble at the top",
			n("Philippians", 4, 0, 0, "Fixture chapter message alpha.")},
		{"Plain link, NO note", "Straight to the verse, no bubble anywhere",
			plain("John", 3, 16, 16)},
		{"Plain link, chapter only", "Chapter opens at the top, nothing highlighted",
			plain("Genesis", 1, 0, 0)},

		// --- the note text itself ------------------------------------------------
		{"Note at the 280-rune cap", "The tallest bubble there can be; nothing below it covered",
			n("Isaiah", 40, 31, 31, devLongNote())},
		{"Note with blank lines", "Paragraph breaks kept; 3+ newlines collapsed",
			n("Matthew", 5, 3, 10, "Fixture paragraph one.\n\n\n\n\nFixture paragraph two.")},
		{"Note with emoji + accents", "Renders as text; no tofu, no mis-measured band",
			n("Psalms", 121, 1, 2, "Fixture Unicode: café ☕ — grüße! αβγ.")},
		{"Note trying to inject markup", "Must appear LITERALLY, never rendered as markup",
			n("John", 1, 1, 1, "<b>bold?</b> <script>alert(1)</script> [link](http://example.com)")},
		{"Note with bidi control characters", "The overrides are stripped by normalizeNote; text reads normally",
			n("Proverbs", 3, 5, 6, "Fixture ‮reversed?‬ and neutral ‏segment‎.")},
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
			ShareLinkURLWithNote("web", "John", 14, 6, 6, "Fixture WEB message alpha.")},
		{"Note in the Catholic canon (deuterocanon)", "Tobit only exists in WEBC — check the switch",
			ShareLinkURLWithNote("webc", "Tobit", 4, 15, 15, "Fixture WEBC message alpha.")},

		// --- the LICENSED translation, which behaves differently per reader -------
		//
		// The only link whose outcome depends on something the recipient has
		// rather than on the link. With the NKJV available it switches and opens
		// like any other; without it — no key, no licence — the passage still
		// opens in whatever translation the reader has and a message says the
		// link was written in the New King James Version. Neither branch was
		// reachable from this page before, which is why it is here: the branch
		// that only some readers see is the one worth being able to tap.
		{"Note in the NKJV (licensed)", "Switches if you have it; otherwise opens here plus a message",
			ShareLinkURLWithNote("nkjv", "Psalms", 23, 1, 4, "Fixture version message alpha beta gamma.")},
	}
}

// devVersionCachePanel: make a translation look as though it was never
// downloaded.
//
// WHY THIS EXISTS. The interesting half of a shared link is the half that only
// SOME readers see. A /nkjv/ link switches translation and opens for anyone who
// has the NKJV, and for anyone who does not it opens the passage in whatever
// they are reading plus a message naming the translation the note was written
// in. Testing the second branch used to mean finding a simulator that had never
// fetched the text — and once any simulator HAS fetched it, that state cannot be
// got back to from inside the app.
//
// WHAT IT ACTUALLY DOES, and the part worth reading before trusting it: deleting
// a cache file changes what the NEXT LAUNCH finds, not what this process holds.
// The Bible currently open is in memory and the reading state names it, so
// clearing the cache of the translation you are reading changes nothing you can
// see until you relaunch. Saying so on the button's own status line is the
// difference between a working control and one that looks broken.
func devVersionCachePanel(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	head := canvas.NewText("Version caches", pal.Text)
	head.TextStyle = fyne.TextStyle{Bold: true}
	head.TextSize = 16

	note := widget.NewLabel("Delete a translation's cached text AND unload it, so the app " +
		"behaves as though it was never downloaded — no relaunch needed. Clearing the one " +
		"you are reading switches you to " + defaultVersionID + " first.")
	note.Wrapping = fyne.TextWrapWord

	status := canvas.NewText("", pal.TextMuted)
	status.TextSize = 12

	rows := container.NewVBox()

	// cachedBytes reports what this version occupies on disk across its current
	// epoch and every superseded one, and whether anything is there at all.
	cachedBytes := func(v BibleVersion) (int64, bool) {
		var total int64
		found := false
		paths := append([]string{cachePathForVersion(v.ID)}, supersededCachePaths(v)...)
		for _, path := range paths {
			if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
				total += fi.Size()
				found = true
			}
		}
		return total, found
	}

	// clear removes the current epoch AND every superseded one. Missing the
	// superseded files would leave the app able to open the translation offline
	// from an older decode — which looks exactly like the delete not working.
	clear := func(v BibleVersion) int {
		removed := 0
		paths := append([]string{cachePathForVersion(v.ID)}, supersededCachePaths(v)...)
		for _, path := range paths {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}
		return removed
	}

	var rebuild func()
	rebuild = func() {
		rows.Objects = rows.Objects[:0]
		anyCached := false
		for _, v := range registeredVersions {
			v := v
			size, cached := cachedBytes(v)
			if cached {
				anyCached = true
			}
			label := v.Abbrev + " — not downloaded"
			if cached {
				label = fmt.Sprintf("%s — %.1f MB cached", v.Abbrev, float64(size)/(1024*1024))
			}
			if v.ID == state.CurrentVersion {
				label += " · open now"
			}
			text := widget.NewLabel(label)
			text.Wrapping = fyne.TextWrapWord

			btn := widget.NewButton("Clear", func() {
				n := clear(v)

				// DELETING THE FILE IS NOT ENOUGH, and believing it was is what
				// made this control lie. A loaded translation is kept in memory
				// by id (AppState.loadedVersions), switchVersion consults that
				// map before it ever reaches the disk, and switchToLinkVersion
				// returns early when the link names the version already open. So
				// a cleared NKJV still opened an /nkjv/ note instantly, with no
				// download and no message — the cache was genuinely gone and
				// nothing about the running app had changed, so the symptom looks
				// exactly like a clear that silently failed.
				//
				// To make the app actually behave as though it never had the
				// text: move off it, then forget it. Only then does the next
				// link take the not-downloaded path.
				switched := false
				if v.ID == state.CurrentVersion && v.ID != defaultVersionID {
					switchVersion(state, defaultVersionID)
					switched = state.CurrentVersion != v.ID
				}
				unloaded := false
				if v.ID != state.CurrentVersion {
					delete(state.loadedVersions, v.ID)
					unloaded = true
				}

				switch {
				case n == 0 && !unloaded:
					status.Text = v.Abbrev + ": nothing cached to clear"
				case v.ID == state.CurrentVersion:
					// The open version IS the default, so there is nowhere to
					// move to. Say what is still true rather than pretend.
					status.Text = v.Abbrev + ": cache cleared — still open in memory, relaunch to see it gone"
				case switched:
					status.Text = v.Abbrev + ": cleared and unloaded — switched to " +
						defaultVersionID + "; a link to it will now behave as never downloaded"
				default:
					status.Text = v.Abbrev + ": cleared and unloaded — a link to it will now behave as never downloaded"
				}
				status.Refresh()
				rebuild()
				state.refresh()
			})
			if !cached {
				btn.Disable()
			}
			rows.Add(container.NewBorder(nil, nil, nil, btn, text))
		}

		all := widget.NewButton("Clear every version cache", func() {
			total := 0
			for _, v := range registeredVersions {
				total += clear(v)
			}
			// Same reasoning as the per-version button: forget them in memory
			// too, or the app keeps serving every translation it has already
			// loaded. The one being read has to stay — there is nothing to fall
			// back to once every cache is gone — so it is the only survivor, and
			// the message says so.
			kept := state.CurrentVersion
			for id := range state.loadedVersions {
				if id != kept {
					delete(state.loadedVersions, id)
				}
			}
			status.Text = fmt.Sprintf("cleared %d cache file(s) and unloaded all but %s "+
				"(still open) — relaunch for a true first-run app", total, kept)
			status.Refresh()
			rebuild()
			state.refresh()
		})
		all.Importance = widget.DangerImportance
		if !anyCached {
			all.Disable()
		}
		rows.Add(all)
		rows.Refresh()
	}
	rebuild()

	return container.NewVBox(head, note, rows, status, widget.NewSeparator())
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
		status.Text = "Notes are " + on + " · " + itoa(storedNoteCount(appPrefs())) + " stored"
		status.Refresh()
	}
	refreshStatus()

	notesSwitch := widget.NewCheck("Shared notes on", func(b bool) {
		// The same verbs Settings uses, both directions: each ends on the
		// projection, so the mirror the Apple sticker pushes from is right
		// before the tab-switch rebuild repaints it — a dev route to the
		// switch is still a route (X4, and the stale-sticker twin).
		if b {
			turnNotesOn(state)
		} else {
			turnNotesOff(state)
		}
		refreshStatus()
	})
	notesSwitch.SetChecked(notesEnabled())

	wipe := widget.NewButton("Delete all stored notes", func() {
		deleteAllNotes(appPrefs())
		clearLiveNote(state)
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

	// EVERYTHING SCROLLS, including what used to be pinned. The title, blurb,
	// notes switch, wipe button, status line and emoji probe were the Border's
	// fixed top slot, which on a phone is most of a screen before the first
	// scenario is reachable — and the scenarios are the point of the page.
	// Nothing here needs to stay in view while scrolling: the status line is
	// read right after tapping something, not during.
	column := container.NewVBox(head, devVersionCachePanel(state))
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
		// behavior can be compared: the app and the web once had the same defect
		// for the same reason, which is far easier to see side by side
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

	return scroll
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
