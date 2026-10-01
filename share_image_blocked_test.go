//go:build !windows && !ios && !android

package bibletext

// A SAVE THAT WAITS DOES NOT FREEZE THE WINDOW (fallbackShareImage).
//
// Inside the snap, where AppArmor prompting is on, the first write into the
// reader's home waits until the reader answers a permission prompt, and the
// share had saved on the UI goroutine. A named pipe stands in for the
// prompt: reading user-dirs.dirs, which the save does to find the Downloads
// folder, waits on a pipe until something writes to it, as a write into the
// home waits on the prompt. The share must return while it waits, and what
// it saves when the wait is over must be the card as it was at the tap, not
// the next card the renderer has written over the same file since.

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestASaveThatWaitsDoesNotHoldTheShare holds both. Its mutations each fail
// it: the save made before the share sets it aside, which holds the share
// for as long as the save waits, and the card read only when the save runs,
// which saves the later card. Control: once the pipe answers, the picture is
// in Downloads and the sheet says so.
func TestASaveThatWaitsDoesNotHoldTheShare(t *testing.T) {
	h := imageShareHarness(t, true)
	home := redirectHome(t)
	downloads := filepath.Join(home, "Downloads")
	mkdirs(t, downloads, filepath.Join(home, ".config"))
	dirsFile := filepath.Join(home, ".config", "user-dirs.dirs")
	if err := syscall.Mkfifo(dirsFile, 0o600); err != nil {
		t.Fatal(err)
	}
	// The work runs on a goroutine of its own and hands the sheet back to
	// this one, the test's UI goroutine.
	dones := make(chan func(), 1)
	shareImageAside = func(work, done func()) {
		go func() {
			work()
			dones <- done
		}()
	}
	// answer gives the waiting read its user-dirs.dirs, once something is
	// reading the pipe, within a bound.
	answer := func() error {
		deadline := time.Now().Add(5 * time.Second)
		for {
			f, err := os.OpenFile(dirsFile, os.O_WRONLY|syscall.O_NONBLOCK, 0)
			if err == nil {
				_, err = f.WriteString("XDG_DOWNLOAD_DIR=\"$HOME/Downloads\"\n")
				if cerr := f.Close(); err == nil {
					err = cerr
				}
				return err
			}
			if !errors.Is(err, syscall.ENXIO) || time.Now().After(deadline) {
				return err
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	src := h.renderedCard()
	tapped, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	returned := make(chan struct{})
	go func() {
		fallbackShareImage(src)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		aerr := answer()
		<-returned
		t.Fatalf("the share held the UI goroutine while its save waited (the pipe answered: %v)", aerr)
	}

	if err := os.WriteFile(src, []byte("the next card, rendered over the file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := answer(); err != nil {
		t.Fatalf("the save never read user-dirs.dirs: %v", err)
	}
	select {
	case done := <-dones:
		done()
	case <-time.After(5 * time.Second):
		t.Fatal("the save never finished")
	}

	got := pictures(downloads)
	if len(got) != 1 {
		t.Fatalf("%d pictures in Downloads, want 1", len(got))
	}
	if b, err := os.ReadFile(got[0]); err != nil || string(b) != string(tapped) {
		t.Errorf("the picture saved reads %q (%v), want the card as it was at the tap, %q", b, err, tapped)
	}
	if texts := imageSheetTexts(h); !hasText(texts, shareLineImage) {
		t.Errorf("control: the sheet reads %v, want %q", texts, shareLineImage)
	}
}
