//go:build !ios && !android

package bibletext

// WHERE A SHARED PICTURE GOES, AND WHAT THE SHEET SAYS OF IT
// (share_image_folder.go, fallbackShareImage).
//
// Inside the snap HOME is the snap's own home, with no Downloads folder, and
// the temp folder is the snap's private one: the picture stayed there, the
// file manager opened nothing, and the sheet said it was shown. These hold,
// on every host the suite runs on (the Linux rules are chosen through
// shareImageGOOS, the snap through the environment the snap sets), that the
// picture lands in the reader's own Downloads folder — the one in
// SNAP_REAL_HOME, under the name user-dirs.dirs gives it — or in the
// reader's home where there is none; that the sheet says it is shown in the
// file manager only when the file manager said so, calls a folder Downloads
// only when that is its name, and that its line never names the temp folder
// as where the picture is saved; and that outside a snap it goes where it
// went before.
// Each check is shown failing on the case it tells apart: run as though the
// app did not know it was in a snap, with no user-dirs.dirs, as Windows, or
// with a file manager that answered yes.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

// THE IMAGE LINES APPROVED ON 1 OCTOBER 2026 (share_sheet_desktop.go), held
// as literals so that a retyping fails here and not in front of a reader.
func TestTheImageLinesAreTheApprovedOnes(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{shareLineImageShown, "The picture is shown in your file manager."},
		{shareLineImageSaved, "The picture is saved in Downloads."},
		{shareLineImageSavedIn, "The picture is saved in %s."},
	} {
		if c.got != c.want {
			t.Errorf("the sheet says %q, want %q", c.got, c.want)
		}
	}
}

// asLinux has the image share follow Linux's folder rules for the test.
func asLinux(t *testing.T) {
	t.Helper()
	prev := shareImageGOOS
	shareImageGOOS = "linux"
	t.Cleanup(func() { shareImageGOOS = prev })
}

// imageShareHarness is the share sheet harness ready for an image share
// under Linux's rules, the preview's mail set, and the file manager
// answering revealShown.
func imageShareHarness(t *testing.T, revealShown bool) *shareSheetHarness {
	t.Helper()
	h := newShareSheetHarness(t)
	h.revealShown = revealShown
	asLinux(t)
	prev := shareImageMail
	shareImageMail.subject, shareImageMail.body = sampleImageSubject, sampleImageBody
	t.Cleanup(func() { shareImageMail = prev })
	return h
}

// snapHomes lays the environment out as the snap's: HOME is the snap's own
// home (redirectHome's, with no Downloads folder, and XDG_CONFIG_HOME in
// it), and the reader's own home, a separate folder, is SNAP_REAL_HOME. SNAP
// is set only when inSnap, so that the same layout run without it shows
// what the app does when it does not know it is in a snap.
func snapHomes(t *testing.T, inSnap bool) (snapHome, realHome string) {
	t.Helper()
	snapHome = redirectHome(t)
	realHome = t.TempDir()
	t.Setenv("SNAP_REAL_HOME", realHome)
	if inSnap {
		t.Setenv("SNAP", "/snap/bibletext/x1")
	}
	return snapHome, realHome
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeUserDirs(t *testing.T, home, body string) {
	t.Helper()
	mkdirs(t, filepath.Join(home, ".config"))
	if err := os.WriteFile(filepath.Join(home, ".config", "user-dirs.dirs"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pictures is the PNGs directly in dir.
func pictures(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".png") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// claimsShown reports whether any line on the sheet says the file manager
// shows the picture.
func claimsShown(texts []string) bool {
	for _, s := range texts {
		if strings.Contains(s, "shown in your file manager") {
			return true
		}
	}
	return false
}

// imageShare runs one image share and returns what the file manager was
// asked to show and what the sheet says, nil when no sheet opened.
func imageShare(h *shareSheetHarness) (src string, revealed []string, texts []string) {
	h.t.Helper()
	src = h.renderedCard()
	fallbackShareImage(src)
	return src, h.revealed, imageSheetTexts(h)
}

// imageSheetTexts is what the image share's sheet on top of the canvas says,
// or nil when there is none. It does not stop the test, so that a control
// can run a case that opens no sheet.
func imageSheetTexts(h *shareSheetHarness) []string {
	p := h.top()
	if p == nil || !p.Visible() || !sheetHas(p, shareSheetImageHeading) {
		return nil
	}
	return sheetTexts(p)
}

func hasText(texts []string, want string) bool {
	for _, s := range texts {
		if s == want {
			return true
		}
	}
	return false
}

// savedOnlyIn checks that the share left exactly one picture, in dir, that
// the file manager was asked to show that picture, and that the sheet reads
// line; problems are returned rather than reported, so a control can run
// the same check on a case it must fail.
func savedOnlyIn(dir string, revealed, texts []string, line string) (problems []string) {
	got := pictures(dir)
	if len(got) != 1 {
		return append(problems, fmt.Sprintf("%d pictures in %s, want 1", len(got), dir))
	}
	if len(revealed) != 1 || revealed[0] != got[0] {
		problems = append(problems, fmt.Sprintf("the file manager was asked to show %v, want %s", revealed, got[0]))
	}
	if !hasText(texts, line) {
		problems = append(problems, fmt.Sprintf("the sheet reads %v, want %q", texts, line))
	}
	return problems
}

// runCase runs body in a subtest of its own (its environment and its
// harness are undone after it) and returns what it found wrong.
func runCase(t *testing.T, name string, body func(t *testing.T) []string) []string {
	t.Helper()
	var problems []string
	t.Run(name, func(t *testing.T) { problems = body(t) })
	return problems
}

// IN THE SNAP THE PICTURE LANDS IN THE READER'S OWN DOWNLOADS FOLDER, the
// one in SNAP_REAL_HOME, not the snap's home, which has none; the file
// manager is asked to show it there, and the sheet says it is saved in
// Downloads and shown. Control: the same layout with SNAP unset reads HOME
// alone, as the app did before, and the check fails.
func TestTheSnapSavesThePictureInTheReadersOwnDownloads(t *testing.T) {
	share := func(inSnap bool) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			snapHome, realHome := snapHomes(t, inSnap)
			mkdirs(t, filepath.Join(realHome, "Downloads"))
			_, revealed, texts := imageShare(h)
			problems := savedOnlyIn(filepath.Join(realHome, "Downloads"), revealed, texts, shareLineImage)
			if stray := pictures(snapHome); len(stray) > 0 {
				problems = append(problems, fmt.Sprintf("a picture was saved in the snap's own home: %v", stray))
			}
			return problems
		}
	}
	if p := runCase(t, "snap", share(true)); len(p) > 0 {
		t.Errorf("in the snap: %v", p)
	}
	if p := runCase(t, "control: SNAP unset", share(false)); len(p) == 0 {
		t.Error("control: with SNAP unset the check still passed, so it cannot tell the snap's real home from its own")
	}
}

// THE DOWNLOAD DIRECTORY IS THE ONE USER-DIRS.DIRS NAMES, under the name the
// desktop's language or the reader gave it, read from the reader's own home
// inside the snap (with $HOME meaning that home) and from XDG_CONFIG_HOME
// outside it. There is no ~/Downloads in either home. The sheet calls the
// folder Downloads only when that is its name: a localised folder the file
// manager shows is said only to be shown, and a renamed one it did not show
// is named by its path. Controls: without user-dirs.dirs the picture does
// not reach the localised folder; and a download directory elsewhere that is
// called Downloads is called so on the sheet, so the name, not the file,
// decides.
func TestTheLocalisedDownloadDirectoryIsHonoured(t *testing.T) {
	const french, russian = "Téléchargements", "Загрузки"
	inSnap := func(withFile bool) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			_, realHome := snapHomes(t, true)
			mkdirs(t, filepath.Join(realHome, french))
			if withFile {
				writeUserDirs(t, realHome, "# written by xdg-user-dirs-update\nXDG_DESKTOP_DIR=\"$HOME/Bureau\"\nXDG_DOWNLOAD_DIR=\"$HOME/"+french+"\"\n")
			}
			_, revealed, texts := imageShare(h)
			return savedOnlyIn(filepath.Join(realHome, french), revealed, texts, shareLineImageShown)
		}
	}
	if p := runCase(t, "snap", inSnap(true)); len(p) > 0 {
		t.Errorf("in the snap: %v", p)
	}
	if p := runCase(t, "control: no user-dirs.dirs", inSnap(false)); len(p) == 0 {
		t.Error("control: with no user-dirs.dirs the picture still reached the localised folder, so the check proves nothing")
	}
	outside := func(dir string, shown bool, line func(dir string) string) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, shown)
			home := redirectHome(t)
			mkdirs(t, filepath.Join(home, dir))
			writeUserDirs(t, home, "XDG_DOWNLOAD_DIR=\"$HOME/"+filepath.ToSlash(dir)+"\"\n")
			_, revealed, texts := imageShare(h)
			return savedOnlyIn(filepath.Join(home, dir), revealed, texts, line(filepath.Join(home, dir)))
		}
	}
	shownLine := func(string) string { return shareLineImageShown }
	pathLine := func(dir string) string { return fmt.Sprintf(shareLineImageSavedIn, dir) }
	if p := runCase(t, "outside a snap", outside(russian, true, shownLine)); len(p) > 0 {
		t.Errorf("outside a snap: %v", p)
	}
	if p := runCase(t, "renamed, not shown", outside("Incoming", false, pathLine)); len(p) > 0 {
		t.Errorf("a renamed download folder the file manager did not show: %v", p)
	}
	if p := runCase(t, "control: elsewhere, called Downloads", outside(filepath.Join("Files", "Downloads"), false, func(string) string { return shareLineImageSaved })); len(p) > 0 {
		t.Errorf("control: a download directory called Downloads was not called so: %v", p)
	}
}

// WITH NO DOWNLOADS FOLDER IN THE SNAP the picture is saved in the reader's
// own home, which the file manager can open, and the sheet says only that it
// is shown there — not in the snap's private temp folder, which nothing
// outside the snap can open. Control: with SNAP unset it is saved in HOME,
// the snap's own home, as it would be outside a snap, and the check fails.
func TestTheSnapWithNoDownloadsSavesInTheReadersHome(t *testing.T) {
	share := func(inSnap bool) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			_, realHome := snapHomes(t, inSnap)
			src, revealed, texts := imageShare(h)
			problems := savedOnlyIn(realHome, revealed, texts, shareLineImageShown)
			if len(revealed) == 1 && revealed[0] == src {
				problems = append(problems, "the file manager was asked to show the renderer's temp copy")
			}
			return problems
		}
	}
	if p := runCase(t, "snap", share(true)); len(p) > 0 {
		t.Errorf("in the snap: %v", p)
	}
	if p := runCase(t, "control: SNAP unset", share(false)); len(p) == 0 {
		t.Error("control: with SNAP unset the check still passed, so it cannot tell the reader's home from the temp folder")
	}
}

// WITH THE HOME PLUG DISCONNECTED the reader's home cannot be reached at all,
// and the picture is saved in the snap's own common folder,
// ~/snap/bibletext/common, which outlives the snap's revisions and which the
// file manager can open, not in the snap's private temp folder, which it
// cannot. The reader's home is a folder that is not there, as it is to a
// snap without the plug. Control: with no SNAP_USER_COMMON no folder takes
// the picture (TestAPictureNoFolderTookIsNotSaidToBeSaved), and the check
// fails.
func TestTheSnapWithoutItsHomePlugSavesInItsOwnFolder(t *testing.T) {
	share := func(withCommon bool) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			redirectHome(t)
			t.Setenv("SNAP", "/snap/bibletext/x1")
			t.Setenv("SNAP_REAL_HOME", filepath.Join(t.TempDir(), "unreachable"))
			common := t.TempDir()
			if withCommon {
				t.Setenv("SNAP_USER_COMMON", common)
			}
			_, revealed, texts := imageShare(h)
			return savedOnlyIn(common, revealed, texts, shareLineImageShown)
		}
	}
	if p := runCase(t, "snap", share(true)); len(p) > 0 {
		t.Errorf("in the snap without its home plug: %v", p)
	}
	if p := runCase(t, "control: no SNAP_USER_COMMON", share(false)); len(p) == 0 {
		t.Error("control: with no SNAP_USER_COMMON the picture still reached the common folder, so the check proves nothing")
	}
}

// A FILE MANAGER THAT DID NOT SAY IT SHOWED THE PICTURE IS NOT SAID TO HAVE:
// the sheet says where the picture is saved — Downloads, or the folder by
// its path, the reader's home where there is no Downloads folder, in the
// snap and outside it — and nothing about the file manager. Control: the
// same share with a file manager that said yes, which the check must catch.
func TestTheImageSheetDoesNotClaimAFailedReveal(t *testing.T) {
	cases := []struct {
		name string
		lay  func(t *testing.T) (dir string, downloads bool)
	}{
		{"snap with Downloads", func(t *testing.T) (string, bool) {
			_, realHome := snapHomes(t, true)
			mkdirs(t, filepath.Join(realHome, "Downloads"))
			return filepath.Join(realHome, "Downloads"), true
		}},
		{"snap without Downloads", func(t *testing.T) (string, bool) {
			_, realHome := snapHomes(t, true)
			return realHome, false
		}},
		{"outside a snap without Downloads", func(t *testing.T) (string, bool) {
			return redirectHome(t), false
		}},
	}
	for _, c := range cases {
		share := func(shown bool) func(t *testing.T) []string {
			return func(t *testing.T) []string {
				h := imageShareHarness(t, shown)
				dir, downloads := c.lay(t)
				_, _, texts := imageShare(h)
				want := fmt.Sprintf(shareLineImageSavedIn, dir)
				if downloads {
					want = shareLineImageSaved
				}
				var problems []string
				if claimsShown(texts) {
					problems = append(problems, fmt.Sprintf("the sheet says the picture is shown: %v", texts))
				}
				if !hasText(texts, want) {
					problems = append(problems, fmt.Sprintf("the sheet reads %v, want %q", texts, want))
				}
				if len(pictures(dir)) == 0 {
					problems = append(problems, "no picture in "+dir)
				}
				return problems
			}
		}
		if p := runCase(t, c.name, share(false)); len(p) > 0 {
			t.Errorf("%s: %v", c.name, p)
		}
		if p := runCase(t, c.name+", control: the file manager said yes", share(true)); len(p) == 0 {
			t.Errorf("%s: control: with the file manager answering yes the check still passed, so it cannot see a claim", c.name)
		}
	}
}

// OUTSIDE A SNAP NOTHING MOVES: the picture is saved in HOME's Downloads
// folder, as before, whatever SNAP_REAL_HOME says, and the sheet reads as it
// did. (Without a Downloads folder it is saved in the home:
// TestOutsideASnapWithNoDownloadsThePictureIsSavedInTheHome.) Control: SNAP
// set takes the picture to the other home, and the check fails.
func TestOutsideASnapThePictureGoesWhereItWent(t *testing.T) {
	share := func(inSnap bool) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			home, other := snapHomes(t, inSnap)
			mkdirs(t, filepath.Join(home, "Downloads"), filepath.Join(other, "Downloads"))
			_, revealed, texts := imageShare(h)
			return savedOnlyIn(filepath.Join(home, "Downloads"), revealed, texts, shareLineImage)
		}
	}
	if p := runCase(t, "tarball", share(false)); len(p) > 0 {
		t.Errorf("outside a snap: %v", p)
	}
	if p := runCase(t, "control: SNAP set", share(true)); len(p) == 0 {
		t.Error("control: with SNAP set the picture still went to HOME's Downloads, so the check proves nothing")
	}
}

// OUTSIDE A SNAP, ON LINUX, WITH NO DOWNLOADS FOLDER, the picture is saved
// in the reader's home, as it is in the snap, not left in the temp folder,
// where the next card is rendered over it; the file manager is asked to show
// that copy, and the sheet says it is shown. Control: Windows' rules, which
// keep the copy they were handed, leave it in the temp folder, and the check
// fails.
func TestOutsideASnapWithNoDownloadsThePictureIsSavedInTheHome(t *testing.T) {
	share := func(goos string) func(t *testing.T) []string {
		return func(t *testing.T) []string {
			h := imageShareHarness(t, true)
			shareImageGOOS = goos
			home := redirectHome(t)
			_, revealed, texts := imageShare(h)
			return savedOnlyIn(home, revealed, texts, shareLineImageShown)
		}
	}
	if p := runCase(t, "linux", share("linux")); len(p) > 0 {
		t.Errorf("outside a snap without Downloads: %v", p)
	}
	if p := runCase(t, "control: windows", share("windows")); len(p) == 0 {
		t.Error("control: under Windows' rules the picture still reached the home, so the check cannot tell the home from the temp folder")
	}
}

// A PICTURE NO FOLDER TOOK IS NOT SAID TO BE SAVED by the sheet's line. It
// is still the file the share was handed, in the temp folder, which keeps
// it only until the next card, and inside the snap is one the reader cannot
// open; so the line never names that folder or says the picture is saved,
// and, unless the file manager said it shows the picture, the sheet does
// not open at all. Shown, the sheet opens under its one heading, Picture
// saved, which every image sheet carries and which this check passes over
// (docs/BACKLOG.md lists that case for a decision). In the snap no folder
// takes it when the home plug is disconnected and SNAP_USER_COMMON takes no
// copy; outside it, when the home is not there. Control: the same shares
// with a file manager that said yes open the sheet, saying only that the
// picture is shown.
func TestAPictureNoFolderTookIsNotSaidToBeSaved(t *testing.T) {
	lays := []struct {
		name string
		lay  func(t *testing.T)
	}{
		{"snap without its home plug or common folder", func(t *testing.T) {
			redirectHome(t)
			t.Setenv("SNAP", "/snap/bibletext/x1")
			t.Setenv("SNAP_REAL_HOME", filepath.Join(t.TempDir(), "unreachable"))
		}},
		{"outside a snap, with no home", func(t *testing.T) {
			gone := filepath.Join(redirectHome(t), "gone")
			t.Setenv("HOME", gone)
			t.Setenv("USERPROFILE", gone)
		}},
	}
	for _, c := range lays {
		share := func(shown bool) func(t *testing.T) []string {
			return func(t *testing.T) []string {
				h := imageShareHarness(t, shown)
				c.lay(t)
				src, revealed, texts := imageShare(h)
				var problems []string
				if len(revealed) != 1 || revealed[0] != src {
					problems = append(problems, fmt.Sprintf("the file manager was asked to show %v, want the picture as handed, %s", revealed, src))
				}
				for _, s := range texts {
					if s != shareSheetImageHeading && (strings.Contains(s, filepath.Dir(src)) || strings.Contains(s, "saved")) {
						problems = append(problems, fmt.Sprintf("the sheet says where the picture is saved: %q", s))
					}
				}
				if shown && !hasText(texts, shareLineImageShown) {
					problems = append(problems, fmt.Sprintf("the file manager showed the picture and the sheet reads %v, want %q", texts, shareLineImageShown))
				}
				if !shown && texts != nil {
					problems = append(problems, fmt.Sprintf("a sheet opened with nothing true to say: %v", texts))
				}
				return problems
			}
		}
		if p := runCase(t, c.name, share(false)); len(p) > 0 {
			t.Errorf("%s: %v", c.name, p)
		}
		if p := runCase(t, c.name+", control: the file manager said yes", share(true)); len(p) > 0 {
			t.Errorf("%s: control: %v", c.name, p)
		}
	}
}

// THE SHEET WAITS FOR THE SAVE AND THE FILE MANAGER'S ANSWER, and carries
// the mail of the share it belongs to: the share returns with the save and
// the reveal set aside, off the UI goroutine, no sheet opens before they are
// done, and a preview that sets its own mail in the meantime does not change
// this one's. Control: once they are done the same look finds the sheet.
func TestTheImageSheetWaitsForTheFileManagersAnswer(t *testing.T) {
	h := imageShareHarness(t, false)
	mkdirs(t, filepath.Join(redirectHome(t), "Downloads"))
	var pending func()
	shareImageAside = func(work, done func()) { pending = func() { work(); done() } }
	isSheet := func() bool {
		p := h.top()
		return p != nil && p.Visible() && sheetHas(p, shareSheetImageHeading)
	}
	fallbackShareImage(h.renderedCard())
	if pending == nil {
		t.Fatal("the save and the reveal were not set aside")
	}
	if len(h.revealed) > 0 {
		t.Errorf("the file manager was asked before the share returned, on the UI goroutine: %v", h.revealed)
	}
	if isSheet() {
		t.Error("the sheet opened before the file manager answered")
	}
	shareImageMail.subject, shareImageMail.body = "Psalm 23:1 (Sample)", "a later preview's mail"
	pending()
	if len(h.revealed) != 1 {
		t.Fatalf("control: the file manager was asked %d times, want once", len(h.revealed))
	}
	if !isSheet() {
		t.Fatalf("control: after the answer the sheet is not on top; texts %v", sheetTexts(h.top()))
	}
	if !sheetHas(h.top(), shareLineImageSaved) {
		t.Errorf("the sheet reads %v, want %q", sheetTexts(h.top()), shareLineImageSaved)
	}
	test.Tap(findTreeButton(h.top().Content, shareButtonEmail))
	select {
	case c := <-h.composed:
		if c.subject != sampleImageSubject || c.body != sampleImageBody {
			t.Errorf("Email… carried %q / %q, want this share's %q / %q", c.subject, c.body, sampleImageSubject, sampleImageBody)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Email… composed nothing")
	}
}

// THE SAVE AND THE REVEAL RUN OFF THE CALLER'S GOROUTINE, and the sheet
// after them: shareImageAside returns while its work is still waiting, and
// runs done only once the work is over. Its two mutations each fail here:
// the work run in place, which holds the caller until it ends, and done run
// without waiting for it. Control: the work let go, done runs.
func TestShareImageAsideRunsTheWorkOffTheCallersGoroutine(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	release, finished, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		shareImageAside(func() { <-release }, func() { close(finished) })
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("shareImageAside held its caller while the work waited")
	}
	select {
	case <-finished:
		t.Fatal("done ran while the work was still waiting")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("control: the work was let go and done never ran")
	}
}

// USER-DIRS.DIRS IS READ AS XDG-USER-DIRS READS IT. The rows that name no
// directory are the controls for the ones that do.
func TestUserDirsFileIsReadAsXDGUserDirsReadsIt(t *testing.T) {
	home := filepath.FromSlash("/home/reader")
	for _, c := range []struct{ name, file, want string }{
		{"as written", "# comment\nXDG_DESKTOP_DIR=\"$HOME/Desktop\"\nXDG_DOWNLOAD_DIR=\"$HOME/Downloads\"\n", filepath.Join(home, "Downloads")},
		{"localised", "XDG_DOWNLOAD_DIR=\"$HOME/Téléchargements\"", filepath.Join(home, "Téléchargements")},
		{"nested", "XDG_DOWNLOAD_DIR=\"$HOME/Files/In\"", filepath.Join(home, "Files", "In")},
		{"absolute", "XDG_DOWNLOAD_DIR=\"/data/dl/\"", filepath.FromSlash("/data/dl")},
		{"spaces around", "  XDG_DOWNLOAD_DIR =\t\"$HOME/Down loads\"", filepath.Join(home, "Down loads")},
		{"escaped", `XDG_DOWNLOAD_DIR="$HOME/a\"b\\c"`, filepath.Join(home, `a"b\c`)},
		{"the last line counts", "XDG_DOWNLOAD_DIR=\"$HOME/One\"\nXDG_DOWNLOAD_DIR=\"$HOME/Two\"", filepath.Join(home, "Two")},
		{"home itself", "XDG_DOWNLOAD_DIR=\"$HOME/\"", home},
		{"bare $HOME", "XDG_DOWNLOAD_DIR=\"$HOME\"", home},
		{"another directory", "XDG_DESKTOP_DIR=\"$HOME/Desktop\"", ""},
		{"commented", "# XDG_DOWNLOAD_DIR=\"$HOME/Downloads\"", ""},
		{"unquoted", "XDG_DOWNLOAD_DIR=$HOME/Downloads", ""},
		{"relative", "XDG_DOWNLOAD_DIR=\"Downloads\"", ""},
		{"another variable", "XDG_DOWNLOAD_DIR=\"$USER/Downloads\"", ""},
		{"longer key", "XDG_DOWNLOAD_DIRS=\"$HOME/Downloads\"", ""},
	} {
		if got := userDirIn(c.file, "DOWNLOAD", home); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// THE FOLDERS TRIED FOLLOW THE PLATFORM: user-dirs.dirs and the home itself
// on Linux only, the reader's real home and the common folder inside a snap
// only, and Windows and macOS keep ~/Downloads alone, a SNAP variable there
// passed over. A user-dirs.dirs naming the home switches the directory off.
// Only a folder called Downloads is called so. The Linux and snap rows are
// the controls for the others: the same files read, the same variables set.
func TestShareImageFoldersFollowThePlatform(t *testing.T) {
	home, real, common := t.TempDir(), t.TempDir(), t.TempDir()
	writeUserDirs(t, home, "XDG_DOWNLOAD_DIR=\"$HOME/Dl\"")
	writeUserDirs(t, real, "XDG_DOWNLOAD_DIR=\"$HOME/RealDl\"")
	off := t.TempDir()
	writeUserDirs(t, off, "XDG_DOWNLOAD_DIR=\"$HOME/\"")
	named := t.TempDir()
	writeUserDirs(t, named, "XDG_DOWNLOAD_DIR=\"$HOME/Files/Downloads\"")
	bases := map[string]string{home: "home", real: "real", off: "off", common: "common", named: "named"}
	dirs := func(fs []shareFolder) string {
		var out []string
		for _, f := range fs {
			rel := f.dir
			for base, label := range bases {
				if r, err := filepath.Rel(base, f.dir); err == nil && !strings.HasPrefix(r, "..") {
					rel = label + "/" + filepath.ToSlash(r)
				}
			}
			out = append(out, fmt.Sprintf("%s:%v", rel, f.downloads))
		}
		return strings.Join(out, " ")
	}
	inSnap := sharePlace{home: home, snap: true, realHome: real, userCommon: common, configHome: filepath.Join(home, ".config")}
	as := func(goos string, p sharePlace) sharePlace { p.goos = goos; return p }
	for _, c := range []struct {
		name string
		p    sharePlace
		want string
	}{
		{"windows", sharePlace{goos: "windows", home: home}, "home/Downloads:true"},
		{"darwin", sharePlace{goos: "darwin", home: home}, "home/Downloads:true"},
		{"windows, SNAP set", as("windows", inSnap), "home/Downloads:true"},
		{"darwin, SNAP set", as("darwin", inSnap), "home/Downloads:true"},
		{"linux", sharePlace{goos: "linux", home: home}, "home/Dl:false home/Downloads:true home/.:false"},
		{"linux, SNAP_REAL_HOME without SNAP", sharePlace{goos: "linux", home: home, realHome: real}, "home/Dl:false home/Downloads:true home/.:false"},
		{"snap", as("linux", inSnap), "real/RealDl:false real/Downloads:true real/.:false common/.:false"},
		{"linux, SNAP_USER_COMMON without SNAP", sharePlace{goos: "linux", home: home, userCommon: common}, "home/Dl:false home/Downloads:true home/.:false"},
		{"linux, XDG_CONFIG_HOME", sharePlace{goos: "linux", home: off, configHome: filepath.Join(home, ".config")}, "off/Dl:false off/Downloads:true off/.:false"},
		{"linux, switched off", sharePlace{goos: "linux", home: off}, "off/Downloads:true off/.:false"},
		{"linux, a download directory called Downloads", sharePlace{goos: "linux", home: named}, "named/Files/Downloads:true named/Downloads:true named/.:false"},
	} {
		if got := dirs(shareImageFolders(c.p)); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// revealHelperEnv makes the test binary, run as a child, stand in for
// xdg-open: "ok" passes (exit 0), "fail" fails (exit 1), "hang" sleeps past
// the wait.
const revealHelperEnv = "BIBLETEXT_REVEAL_CHILD"

func TestRevealHelperChild(t *testing.T) {
	switch os.Getenv(revealHelperEnv) {
	case "ok":
	case "fail":
		t.Fail()
	case "hang":
		time.Sleep(3 * time.Second)
	default:
		t.Skip("run as a child of TestRevealByCommandBelievesOnlyASuccess")
	}
}

// THE FILE MANAGER IS SAID TO SHOW THE PICTURE ONLY ON A SUCCESS: an exit
// status of 0 within the wait. A command that fails, cannot start, or has not
// finished is not. The success is the control for the rest.
func TestRevealByCommandBelievesOnlyASuccess(t *testing.T) {
	child := func(mode string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=^TestRevealHelperChild$")
		cmd.Env = append(os.Environ(), revealHelperEnv+"="+mode)
		return cmd
	}
	if !revealByCommand(child("ok"), time.Minute) {
		t.Fatal("control: a command that succeeded was not believed")
	}
	if revealByCommand(child("fail"), time.Minute) {
		t.Error("a command that failed was believed")
	}
	if revealByCommand(exec.Command(filepath.Join(t.TempDir(), "no-such-opener")), time.Minute) {
		t.Error("a command that could not start was believed")
	}
	start := time.Now()
	if revealByCommand(child("hang"), 300*time.Millisecond) {
		t.Error("a command still running at the wait was believed")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("the wait for a hung command took %v, past its bound", waited)
	}
}
