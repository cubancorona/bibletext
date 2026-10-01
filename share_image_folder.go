//go:build !ios && !android

package bibletext

// WHERE THE DESKTOP SAVES A SHARED PICTURE (fallbackShareImage): the
// reader's Downloads folder, found the way the desktop finds it, and
// otherwise a folder the file manager can open and that keeps the picture.
//
// On Linux the Downloads folder is the XDG download directory,
// XDG_DOWNLOAD_DIR in user-dirs.dirs, under whatever name the desktop's
// language or the reader gave it (Téléchargements, Загрузки), and
// ~/Downloads where the file names none, names the home itself (which is how
// a directory is switched off there), or names one that is not there. The
// sheet calls the folder Downloads only when that is its name; one with
// another name is named by its path, or not at all when the file manager
// shows it (savedImage.line). Windows and macOS have ~/Downloads alone.
//
// Inside the snap HOME is the snap's own ~/snap/bibletext/<revision>, which
// has no Downloads folder, and the temp folder the renderer writes to is the
// snap's private one, which nothing outside the snap can open. The reader's
// own home is in SNAP_REAL_HOME, and the snap's home plug reaches the files
// in it that are not hidden, so the Downloads folder is looked for there:
// user-dirs.dirs is read from that home's .config (the desktop plug may read
// it; XDG_CONFIG_HOME inside the snap is the snap's own), and its $HOME is
// that home. A snap is a Linux one: SNAP on Windows or the macOS mimic is
// passed over (sharePlace.inSnap).
//
// With no Downloads folder, the picture is saved on Linux in the reader's
// home itself, the folder the file manager opens on, in a snap and outside
// one alike; and inside the snap, where the home plug is disconnected, so
// that the reader's home cannot be reached at all, in the snap's own folder
// that outlives its revisions (SNAP_USER_COMMON, ~/snap/bibletext/common),
// which the file manager can open too. Windows and the macOS mimic keep the
// copy they were handed, in the temp folder.
//
// Where no folder takes the picture it is still the file the share was
// handed, in the temp folder, which keeps it only until the next card is
// rendered over it, or, inside the snap, where nothing outside the snap can
// open it. The sheet never names that folder as the place the picture is
// saved (savedImage.line).

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// shareImageGOOS is the platform whose folder rules the image share follows:
// runtime.GOOS, and a test's choice, so that the Linux and snap rules are
// held on every host the suite runs on.
var shareImageGOOS = runtime.GOOS

// sharePlace is what decides where a shared picture is saved.
type sharePlace struct {
	goos       string // shareImageGOOS
	home       string // os.UserHomeDir: inside a snap, the snap's own home
	snap       bool   // $SNAP is set (inSnap says whether that makes it a snap)
	realHome   string // $SNAP_REAL_HOME: the reader's home, inside a snap
	userCommon string // $SNAP_USER_COMMON: the snap's own folder for every revision
	configHome string // $XDG_CONFIG_HOME
}

// shareImagePlaceNow reads the place from the process's environment.
func shareImagePlaceNow() sharePlace {
	p := sharePlace{
		goos:       shareImageGOOS,
		snap:       os.Getenv("SNAP") != "",
		realHome:   os.Getenv("SNAP_REAL_HOME"),
		userCommon: os.Getenv("SNAP_USER_COMMON"),
		configHome: os.Getenv("XDG_CONFIG_HOME"),
	}
	if h, err := os.UserHomeDir(); err == nil {
		p.home = h
	}
	return p
}

// inSnap reports whether the app runs inside a snap: SNAP is set, on Linux,
// the one platform snaps run on. A SNAP variable on Windows or the macOS
// mimic is a stray, and its rules are not followed there.
func (p sharePlace) inSnap() bool {
	return p.snap && p.goos == "linux"
}

// xdgDesktop reports whether the platform follows the XDG rules: user-dirs.dirs
// for the Downloads folder, and the home itself where there is none.
func (p sharePlace) xdgDesktop() bool {
	return p.goos != "windows" && p.goos != "darwin"
}

// readerHome is the reader's own home: SNAP_REAL_HOME inside a snap, and
// the home everywhere else.
func (p sharePlace) readerHome() string {
	if p.inSnap() && p.realHome != "" {
		return p.realHome
	}
	return p.home
}

// shareFolder is a folder a shared picture may be saved into.
type shareFolder struct {
	dir       string
	downloads bool // the reader's Downloads folder, called Downloads: the sheet names it so
}

// shareImageFolders is where the picture is saved, in the order tried: the
// XDG download directory where the desktop has one, ~/Downloads, then on
// Linux the reader's home, and inside a snap the snap's own common folder. A
// folder that is not there, or takes no copy, passes to the next
// (saveSharedImage). A download directory with a name other than Downloads
// is not called Downloads on the sheet.
func shareImageFolders(p sharePlace) []shareFolder {
	var out []shareFolder
	add := func(dir string, downloads bool) {
		dir = filepath.Clean(dir)
		for _, f := range out {
			if f.dir == dir {
				return
			}
		}
		out = append(out, shareFolder{dir: dir, downloads: downloads})
	}
	if home := p.readerHome(); home != "" {
		if p.xdgDesktop() {
			if dir := xdgDownloadDir(p); dir != "" {
				add(dir, filepath.Base(dir) == "Downloads")
			}
		}
		add(filepath.Join(home, "Downloads"), true)
		if p.xdgDesktop() {
			add(home, false)
		}
	}
	if p.inSnap() && p.userCommon != "" {
		add(p.userCommon, false)
	}
	return out
}

// xdgDownloadDir is the download directory user-dirs.dirs names, or "" where
// there is no file, it names none, or it names the reader's home, which is
// how the file switches a directory off.
func xdgDownloadDir(p sharePlace) string {
	home := p.readerHome()
	config := filepath.Join(home, ".config")
	if !p.inSnap() && filepath.IsAbs(p.configHome) {
		config = p.configHome
	}
	b, err := os.ReadFile(filepath.Join(config, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	dir := userDirIn(string(b), "DOWNLOAD", home)
	if dir == "" || filepath.Clean(dir) == filepath.Clean(home) {
		return ""
	}
	return dir
}

// userDirIn is the directory a user-dirs.dirs file names for XDG_<kind>_DIR,
// read as xdg-user-dirs' own lookup reads it: a line XDG_<kind>_DIR="$HOME/x"
// or XDG_<kind>_DIR="/x", the value running to the next unescaped quote with
// each backslash taking the character after it literally, "$HOME" meaning
// home, and the last such line counting. Comments and any other form are
// passed over. "" when no line names one.
func userDirIn(file, kind, home string) string {
	key := "XDG_" + kind + "_DIR"
	found := ""
	sc := bufio.NewScanner(strings.NewReader(file))
	for sc.Scan() {
		p := strings.TrimLeft(sc.Text(), " \t")
		if !strings.HasPrefix(p, key) {
			continue
		}
		p = strings.TrimLeft(p[len(key):], " \t")
		if !strings.HasPrefix(p, "=") {
			continue
		}
		p = strings.TrimLeft(p[1:], " \t")
		if !strings.HasPrefix(p, `"`) {
			continue
		}
		p = p[1:]
		relative := false
		switch {
		case strings.HasPrefix(p, "$HOME/"):
			p, relative = p[len("$HOME/"):], true
		case strings.HasPrefix(p, `$HOME"`):
			p, relative = p[len("$HOME"):], true
		case !strings.HasPrefix(p, "/"):
			continue
		}
		var v strings.Builder
		for i := 0; i < len(p) && p[i] != '"'; i++ {
			if p[i] == '\\' && i+1 < len(p) {
				i++
			}
			v.WriteByte(p[i])
		}
		if relative {
			found = filepath.Join(home, filepath.FromSlash(v.String()))
		} else {
			found = filepath.Clean(filepath.FromSlash(v.String()))
		}
	}
	return found
}

// savedImage is where a shared picture was saved.
type savedImage struct {
	file      string // the picture
	folder    string // the folder that keeps it; "" when none took it and file is the one the share was handed
	downloads bool   // the folder is the reader's Downloads folder, called Downloads
	err       error  // why no folder took it, when none did
}

// saveSharedImage copies the rendered card at path, under the reader's name
// for it (shareImageName), into the first of the place's folders that is
// there and takes the copy. Where none does, the picture is still the file
// at path, in no folder that keeps it.
func saveSharedImage(path string, p sharePlace, now time.Time) savedImage {
	var err error
	for _, f := range shareImageFolders(p) {
		st, serr := os.Stat(f.dir)
		if serr != nil || !st.IsDir() {
			continue
		}
		target := filepath.Join(f.dir, shareImageName(now))
		if err = copyFileContents(path, target); err == nil {
			return savedImage{file: target, folder: f.dir, downloads: f.downloads}
		}
	}
	if err == nil {
		err = errors.New("no folder to save the picture in")
	}
	return savedImage{file: path, err: err}
}

// line is what the sheet says of the picture: where it is saved, when that
// is Downloads, and that the file manager shows it only when the file
// manager said it did (revealInFileManager). A picture saved elsewhere whose
// folder the file manager did not open is named by its folder's path, the
// one way left to find it. A picture no folder took is not said to be saved
// anywhere: the temp folder it is in keeps it only until the next card, and
// inside the snap is one the reader cannot open. Unless the file manager
// shows it there is nothing true to say of it, and line is "": the sheet
// does not open (fallbackShareImage).
func (s savedImage) line(shown bool) string {
	switch {
	case s.downloads && shown:
		return shareLineImage
	case s.downloads:
		return shareLineImageSaved
	case shown:
		return shareLineImageShown
	case s.folder != "":
		return fmt.Sprintf(shareLineImageSavedIn, s.folder)
	default:
		return ""
	}
}
