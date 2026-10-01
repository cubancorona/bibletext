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
// file manager only when the file manager said so; and that outside a snap
// it goes where it went before. Each check is shown failing on the case it
// tells apart: run as though the app did not know it was in a snap, with no
// user-dirs.dirs, or with a file manager that answered yes.

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

// THE IMAGE LINES STILL TO BE SETTLED (share_sheet_desktop.go), held as
// literals so that a retyping fails here and not in front of a reader.
func TestTheImageLinesStillToBeSettledAreTheListedOnes(t *testing.T) {
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
// asked to show and what the sheet says.
func imageShare(h *shareSheetHarness) (src string, revealed []string, texts []string) {
	h.t.Helper()
	src = h.renderedCard()
	fallbackShareImage(src)
	return src, h.revealed, sheetTexts(h.sheet())
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
	found := false
	for _, s := range texts {
		found = found || s == line
	}
	if !found {
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
// desktop's language gave it, read from the reader's own home inside the
// snap (with $HOME meaning that home) and from XDG_CONFIG_HOME outside it.
// There is no Downloads folder in either home. Control: without
// user-dirs.dirs the picture does not reach the localised folder.
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
			return savedOnlyIn(filepath.Join(realHome, french), revealed, texts, shareLineImage)
		}
	}
	if p := runCase(t, "snap", inSnap(true)); len(p) > 0 {
		t.Errorf("in the snap: %v", p)
	}
	if p := runCase(t, "control: no user-dirs.dirs", inSnap(false)); len(p) == 0 {
		t.Error("control: with no user-dirs.dirs the picture still reached the localised folder, so the check proves nothing")
	}
	if p := runCase(t, "outside a snap", func(t *testing.T) []string {
		h := imageShareHarness(t, true)
		home := redirectHome(t)
		mkdirs(t, filepath.Join(home, russian))
		writeUserDirs(t, home, "XDG_DOWNLOAD_DIR=\"$HOME/"+russian+"\"\n")
		_, revealed, texts := imageShare(h)
		return savedOnlyIn(filepath.Join(home, russian), revealed, texts, shareLineImage)
	}); len(p) > 0 {
		t.Errorf("outside a snap: %v", p)
	}
}

// WITH NO DOWNLOADS FOLDER IN THE SNAP the picture is saved in the reader's
// own home, which the file manager can open, and the sheet says only that it
// is shown there — not in the snap's private temp folder, which nothing
// outside the snap can open. Control: with SNAP unset it stays in the temp
// folder, as it does outside a snap, and the check fails.
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
// snap without the plug. Control: with no SNAP_USER_COMMON the picture stays
// in the temp folder, and the check fails.
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
// its path — and nothing about the file manager. Control: the same share
// with a file manager that said yes, which the check must catch.
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
			redirectHome(t)
			return "", false // the renderer's own folder
		}},
	}
	for _, c := range cases {
		share := func(shown bool) func(t *testing.T) []string {
			return func(t *testing.T) []string {
				h := imageShareHarness(t, shown)
				dir, downloads := c.lay(t)
				src, _, texts := imageShare(h)
				if dir == "" {
					dir = filepath.Dir(src)
				}
				want := fmt.Sprintf(shareLineImageSavedIn, dir)
				if downloads {
					want = shareLineImageSaved
				}
				var problems []string
				if claimsShown(texts) {
					problems = append(problems, fmt.Sprintf("the sheet says the picture is shown: %v", texts))
				}
				if !sheetHas(h.sheet(), want) {
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
// did. (Without a Downloads folder it stays in the temp folder:
// TestTheImageSheetWithoutDownloadsSaysOnlyWhereItIsShown.) Control: SNAP
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

// THE SHEET WAITS FOR THE FILE MANAGER'S ANSWER, and carries the mail of
// the share it belongs to: no sheet opens before the answer, and a preview
// that sets its own mail in the meantime does not change this one's.
// Control: once the answer comes the same look finds the sheet.
func TestTheImageSheetWaitsForTheFileManagersAnswer(t *testing.T) {
	h := imageShareHarness(t, true)
	mkdirs(t, filepath.Join(redirectHome(t), "Downloads"))
	var answer func(bool)
	revealInFileManager = func(p string, report func(bool)) {
		h.revealed = append(h.revealed, p)
		answer = report
	}
	isSheet := func() bool {
		p := h.top()
		return p != nil && p.Visible() && sheetHas(p, shareSheetImageHeading)
	}
	fallbackShareImage(h.renderedCard())
	if answer == nil {
		t.Fatal("control: the file manager was not asked")
	}
	if isSheet() {
		t.Error("the sheet opened before the file manager answered")
	}
	shareImageMail.subject, shareImageMail.body = "Psalm 23:1 (Sample)", "a later preview's mail"
	answer(false)
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

// THE FOLDERS TRIED FOLLOW THE PLATFORM: user-dirs.dirs on Linux only, the
// reader's real home inside a snap only, and Windows and macOS keep
// ~/Downloads alone. A user-dirs.dirs naming the home switches the directory
// off. The Linux rows are the controls for the others: the same file read.
func TestShareImageFoldersFollowThePlatform(t *testing.T) {
	home, real, common := t.TempDir(), t.TempDir(), t.TempDir()
	writeUserDirs(t, home, "XDG_DOWNLOAD_DIR=\"$HOME/Dl\"")
	writeUserDirs(t, real, "XDG_DOWNLOAD_DIR=\"$HOME/RealDl\"")
	off := t.TempDir()
	writeUserDirs(t, off, "XDG_DOWNLOAD_DIR=\"$HOME/\"")
	dirs := func(fs []shareFolder) string {
		var out []string
		for _, f := range fs {
			rel := f.dir
			for _, base := range []string{home, real, off, common} {
				if r, err := filepath.Rel(base, f.dir); err == nil && !strings.HasPrefix(r, "..") {
					rel = map[string]string{home: "home", real: "real", off: "off", common: "common"}[base] + "/" + filepath.ToSlash(r)
				}
			}
			out = append(out, fmt.Sprintf("%s:%v", rel, f.downloads))
		}
		return strings.Join(out, " ")
	}
	for _, c := range []struct {
		name string
		p    sharePlace
		want string
	}{
		{"windows", sharePlace{goos: "windows", home: home}, "home/Downloads:true"},
		{"darwin", sharePlace{goos: "darwin", home: home}, "home/Downloads:true"},
		{"linux", sharePlace{goos: "linux", home: home}, "home/Dl:true home/Downloads:true"},
		{"linux, SNAP_REAL_HOME without SNAP", sharePlace{goos: "linux", home: home, realHome: real}, "home/Dl:true home/Downloads:true"},
		{"snap", sharePlace{goos: "linux", home: home, snap: true, realHome: real, userCommon: common, configHome: filepath.Join(home, ".config")}, "real/RealDl:true real/Downloads:true real/.:false common/.:false"},
		{"linux, SNAP_USER_COMMON without SNAP", sharePlace{goos: "linux", home: home, userCommon: common}, "home/Dl:true home/Downloads:true"},
		{"linux, XDG_CONFIG_HOME", sharePlace{goos: "linux", home: off, configHome: filepath.Join(home, ".config")}, "off/Dl:true off/Downloads:true"},
		{"linux, switched off", sharePlace{goos: "linux", home: off}, "off/Downloads:true"},
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
