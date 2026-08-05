//go:build !darwin && !android

package bibletext

// countingSeeker is the read-along position source on Windows/Linux (audible
// position = bytes read − player buffer). These run on the linux/windows CI
// jobs — the platforms whose build this file belongs to.

import (
	"bytes"
	"io"
	"testing"
)

func TestCountingSeekerTracksReadsAndSeeks(t *testing.T) {
	src := bytes.NewReader(make([]byte, 4096))
	cs := &countingSeeker{r: src}

	buf := make([]byte, 1000)
	for i := 0; i < 3; i++ {
		if _, err := io.ReadFull(cs, buf); err != nil {
			t.Fatal(err)
		}
	}
	if got := cs.pos(); got != 3000 {
		t.Fatalf("pos after 3x1000 reads = %d, want 3000", got)
	}

	// An intentional jump (the ±15s skip) re-syncs the count to the target.
	if _, err := cs.Seek(400, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if got := cs.pos(); got != 400 {
		t.Fatalf("pos after Seek(400) = %d, want 400", got)
	}
	if _, err := io.ReadFull(cs, buf[:100]); err != nil {
		t.Fatal(err)
	}
	if got := cs.pos(); got != 500 {
		t.Fatalf("pos after seek+read = %d, want 500", got)
	}

	// A failed seek must leave the count alone.
	if _, err := cs.Seek(-10, io.SeekStart); err == nil {
		t.Fatal("negative seek should error")
	}
	if got := cs.pos(); got != 500 {
		t.Fatalf("failed seek must not move pos (got %d)", got)
	}
}
