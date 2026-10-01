//go:build !ios && !android

package bibletext

// WHERE THE DESKTOP SAVES A SHARED PICTURE (fallbackShareImage): the
// reader's Downloads folder, found the way the desktop finds it, and
// otherwise a folder the file manager can open.
//
// On Linux the Downloads folder is the XDG download directory,
// XDG_DOWNLOAD_DIR in user-dirs.dirs, under whatever name the desktop's
// language gave it (Téléchargements, Загрузки), and ~/Downloads where the
// file names none, names the home itself (which is how a directory is
// switched off there), or names one that is not there. Windows and macOS
// have ~/Downloads alone.
//
// Inside the snap HOME is the snap's own ~/snap/bibletext/<revision>, which
// has no Downloads folder, and the temp folder the renderer writes to is the
// snap's private one, which nothing outside the snap can open. The reader's
// own home is in SNAP_REAL_HOME, and the snap's home plug reaches the files
// in it that are not hidden, so the Downloads folder is looked for there:
// user-dirs.dirs is read from that home's .config (the desktop plug may read
// it; XDG_CONFIG_HOME inside the snap is the snap's own), and its $HOME is
// that home. With no Downloads folder there either, the picture is saved in
// the reader's home itself, the folder the file manager opens on; and where
// the home plug is disconnected, so that the reader's home cannot be reached
// at all, in the snap's own folder that outlives its revisions
// (SNAP_USER_COMMON, ~/snap/bibletext/common), which the file manager can
// open too. It is left in the snap's temp folder only when none of those
// takes it.
//
// Outside a snap, with no Downloads folder, the picture stays where the
// renderer wrote it, in the temp folder, which the file manager can open.

import (
	"bufio"
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
	snap       bool   // $SNAP is set: the app runs inside a snap
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

// readerHome is the reader's own home: SNAP_REAL_HOME inside a snap, and
// the home everywhere else.
func (p sharePlace) readerHome() string {
	if p.snap && p.realHome != "" {
		return p.realHome
	}
	return p.home
}

// shareFolder is a folder a shared picture may be saved into.
type shareFolder struct {
	dir       string
	downloads bool // the reader's Downloads folder, which the sheet names
}

// shareImageFolders is where the picture is saved, in the order tried: the
// XDG download directory where the desktop has one, ~/Downloads, and inside
// a snap the reader's home and then the snap's own common folder. A folder
// that is not there, or takes no copy, passes to the next
// (saveSharedImage).
func shareImageFolders(p sharePlace) []shareFolder {
	home := p.readerHome()
	if home == "" {
		return nil
	}
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
	if p.goos != "windows" && p.goos != "darwin" {
		if dir := xdgDownloadDir(p); dir != "" {
			add(dir, true)
		}
	}
	add(filepath.Join(home, "Downloads"), true)
	if p.snap {
		add(home, false)
		if p.userCommon != "" {
			add(p.userCommon, false)
		}
	}
	return out
}

// xdgDownloadDir is the download directory user-dirs.dirs names, or "" where
// there is no file, it names none, or it names the reader's home, which is
// how the file switches a directory off.
func xdgDownloadDir(p sharePlace) string {
	home := p.readerHome()
	config := filepath.Join(home, ".config")
	if !p.snap && filepath.IsAbs(p.configHome) {
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
	folder    string // the folder it is in
	downloads bool   // the folder is the reader's Downloads folder
}

// saveSharedImage copies the rendered card at path, under the reader's name
// for it (shareImageName), into the first of the place's folders that is
// there and takes the copy. Where none does, the picture is the renderer's
// own file.
func saveSharedImage(path string, p sharePlace, now time.Time) savedImage {
	for _, f := range shareImageFolders(p) {
		if st, err := os.Stat(f.dir); err != nil || !st.IsDir() {
			continue
		}
		target := filepath.Join(f.dir, shareImageName(now))
		if copyFileContents(path, target) == nil {
			return savedImage{file: target, folder: f.dir, downloads: f.downloads}
		}
	}
	return savedImage{file: path, folder: filepath.Dir(path)}
}

// line is what the sheet says of the picture: where it is saved, when that
// is Downloads, and that the file manager shows it only when the file
// manager said it did (revealInFileManager). A picture saved elsewhere whose
// folder the file manager did not open is named by its folder's path, the
// one way left to find it.
func (s savedImage) line(shown bool) string {
	switch {
	case s.downloads && shown:
		return shareLineImage
	case s.downloads:
		return shareLineImageSaved
	case shown:
		return shareLineImageShown
	default:
		return fmt.Sprintf(shareLineImageSavedIn, s.folder)
	}
}
