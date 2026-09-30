//go:build !ios && !android

package bibletext

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// shareParts is a text share as a system share sheet that takes it in
// parts is handed it: the Windows Share sheet (share_windows.go), whose
// package has a title it will not open without, the text, and a web link
// for the sheet's link affordance and for the apps that take a link.
type shareParts struct {
	title string // the citation: "John 3:16 (World English Bible)"
	text  string // the message, exactly as every other platform shares it
	link  string // the link a link or note share ends in, or ""
}

// sharePartsFor splits a composed text share into its parts. The seam
// carries the message alone, on every platform, so the parts are read from
// its shape, as the desktop confirmation reads its verb (shareVerbOf): the
// title is the citation, which a citation share ends in behind its em dash
// and a link or note share carries on the line above the link; the link is
// that last line. A message with no citation to read falls back to the
// product's name, since the sheet refuses a package without a title.
func sharePartsFor(s string) shareParts {
	verb := shareVerbOf(s)
	p := shareParts{title: shareSubjectOf(s, verb), text: s}
	if verb != shareVerbCitation {
		lines := strings.Split(strings.TrimSpace(s), "\n")
		p.link = strings.TrimSpace(lines[len(lines)-1])
	}
	if p.title == "" {
		p.title = ProductName()
	}
	return p
}

// shareImageName is the file name a shared picture goes by, where the reader
// sees it: in Downloads when the desktop confirmation saves it there
// (fallbackShareImage), and in the Windows Share sheet, which shows the
// file's name in place of a title.
func shareImageName(t time.Time) string {
	return ProductName() + " verse " + t.Format("2006-01-02 15.04.05") + ".png"
}

// shareImageKeep is how long a copy made for the Windows Share sheet is kept
// (copyShareImage).
const shareImageKeep = 24 * time.Hour

// copyShareImage copies the rendered card at path to a file named for the
// reader, in a folder of the app's own under imageRenderDir, and returns the
// copy's absolute path. The renderer's file is rewritten by the next card,
// and its name is the renderer's, not the reader's; the Windows Share sheet
// shows the file's name, and an app shared to may read the file after the
// sheet has closed, so the copy is kept, and the folder is cleared of copies
// older than shareImageKeep as the next is made.
func copyShareImage(path string, now time.Time) (string, error) {
	dir, err := filepath.Abs(filepath.Join(imageRenderDir(), strings.ToLower(ProductName())+"-share"))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if info, err := e.Info(); err == nil && info.Mode().IsRegular() && now.Sub(info.ModTime()) > shareImageKeep {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}
	dst := filepath.Join(dir, shareImageName(now))
	if err := copyFileContents(path, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// sharedImage is a picture share as it stood when the reader tapped Share:
// the card copied under the reader's name and the mail the preview set for
// it. Both are taken then, because the Windows Share sheet, or its fallback,
// can take the share seconds later, and a preview opened in between
// rewrites the renderer's file (renderVerseImage) and shareImageMail for its
// own card.
type sharedImage struct {
	file string    // the copy (copyShareImage), or the renderer's file when the copy failed
	mail shareMail // shareImageMail as the tap left it
	err  error     // why the copy failed, or nil
}

// takeSharedImage takes the card at path and the mail for it.
func takeSharedImage(path string, now time.Time) sharedImage {
	img := sharedImage{file: path, mail: shareImageMail}
	if file, err := copyShareImage(path, now); err != nil {
		img.err = err
	} else {
		img.file = file
	}
	return img
}

// title is the package title the Windows Share sheet is given for the
// picture: its citation, or the product's name where the preview set none,
// since the sheet refuses a package without a title.
func (img sharedImage) title() string {
	if img.mail.subject != "" {
		return img.mail.subject
	}
	return ProductName()
}

// fallback opens the in-app sheet for the picture as it was taken. It puts
// the share's mail back first, since fallbackShareImage reads shareImageMail
// and a later preview may have replaced it; that preview sets its own again
// before it hands its card on. UI goroutine.
func (img sharedImage) fallback() {
	shareImageMail = img.mail
	fallbackShareImage(img.file)
}
