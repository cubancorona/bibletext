//go:build !linux && !ios && !android

package bibletext

// Showing a shared picture in the file manager off Linux (revealFile, behind
// revealInFileManager in share_fallback.go): Explorer's select mode on
// Windows, where the in-app sheet is the Share sheet's fallback; Finder's
// reveal on macOS, reached only through the platform-mimic dev mode, where
// the flow still has to complete and Finder is the honest stand-in; and
// xdg-open on the folder anywhere else the desktop build compiles.

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealFile asks the file manager to show path and reports whether it said
// it did. Explorer exits with 1 whether or not it showed the file, so on
// Windows its start is all there is to go on; open and xdg-open say, within
// revealAnswerWait.
func revealFile(path string) bool {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("explorer", "/select,", path)
		if cmd.Start() != nil {
			return false
		}
		go func() { _ = cmd.Wait() }()
		return true
	case "darwin":
		return revealByCommand(exec.Command("open", "-R", path), revealAnswerWait)
	default:
		return revealByCommand(exec.Command("xdg-open", filepath.Dir(path)), revealAnswerWait)
	}
}
