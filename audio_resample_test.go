//go:build !darwin && !android

package bibletext

// The desktop engine plays at one fixed rate, so any narration recorded at
// another must be converted rather than refused. Before this the engine took
// the first file's rate as the context rate and failed every file at a
// different one for the rest of the process — and the WEB recordings mix
// 22050 and 44100, so playing a New Testament chapter left the whole Old
// Testament silent, or the reverse. These pin the conversion the fix rests
// on: frame counts, interpolation, the OUTPUT-domain seek mapping that
// read-along position depends on, and whole-frame reads (a partial frame
// swaps the channels of everything after it).

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func resamplePCM(frames []int16) []byte {
	b := make([]byte, 0, len(frames)*4)
	for _, v := range frames {
		var f [4]byte
		binary.LittleEndian.PutUint16(f[0:2], uint16(v))
		binary.LittleEndian.PutUint16(f[2:4], uint16(v))
		b = append(b, f[:]...)
	}
	return b
}

func resampleDecode(b []byte) []int16 {
	out := make([]int16, 0, len(b)/4)
	for i := 0; i+3 < len(b); i += 4 {
		out = append(out, int16(binary.LittleEndian.Uint16(b[i:i+2])))
	}
	return out
}

func TestResampleDoublesFrameCount(t *testing.T) {
	src := resamplePCM([]int16{0, 100, 200, 300})
	r, err := newResampleSeeker(bytes.NewReader(src), 22050, 44100, int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	frames := resampleDecode(got)
	// 4 source frames at 2x -> about 8 output frames.
	if len(frames) < 7 || len(frames) > 8 {
		t.Fatalf("expected ~8 output frames, got %d: %v", len(frames), frames)
	}
	// Interpolation: the midpoints appear between the originals.
	if frames[0] != 0 || frames[2] != 100 {
		t.Errorf("source frames must land on even outputs: %v", frames)
	}
	if frames[1] != 50 {
		t.Errorf("2x upsample must average neighbours, got %d want 50: %v", frames[1], frames)
	}
	if r.Length() != int64(len(src))*2 {
		t.Errorf("Length must be the output length: got %d want %d", r.Length(), len(src)*2)
	}
}

func TestResampleHalvesFrameCount(t *testing.T) {
	src := resamplePCM([]int16{0, 100, 200, 300, 400, 500, 600, 700})
	r, _ := newResampleSeeker(bytes.NewReader(src), 44100, 22050, int64(len(src)))
	got, _ := io.ReadAll(r)
	frames := resampleDecode(got)
	if len(frames) < 4 || len(frames) > 5 {
		t.Fatalf("expected ~4 output frames, got %d: %v", len(frames), frames)
	}
	if frames[0] != 0 || frames[1] != 200 {
		t.Errorf("downsample must take every other frame: %v", frames)
	}
}

func TestResampleSeekMapsOutputToSource(t *testing.T) {
	src := resamplePCM([]int16{0, 100, 200, 300, 400, 500, 600, 700})
	r, _ := newResampleSeeker(bytes.NewReader(src), 22050, 44100, int64(len(src)))
	// Seek to output frame 4 == source frame 2 (value 200).
	pos, err := r.Seek(4*otoBytesPerFrame, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if pos != 4*otoBytesPerFrame {
		t.Fatalf("Seek must report the OUTPUT offset, got %d", pos)
	}
	buf := make([]byte, otoBytesPerFrame)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatal(err)
	}
	if v := resampleDecode(buf)[0]; v != 200 {
		t.Errorf("output frame 4 must be source frame 2 (200), got %d", v)
	}
}

func TestResampleEmitsWholeFramesOnly(t *testing.T) {
	src := resamplePCM([]int16{0, 100, 200, 300})
	r, _ := newResampleSeeker(bytes.NewReader(src), 22050, 44100, int64(len(src)))
	buf := make([]byte, 7) // not a frame multiple
	n, _ := r.Read(buf)
	if n%otoBytesPerFrame != 0 {
		t.Errorf("Read must emit whole frames only, got %d bytes", n)
	}
}

func TestResampleIdentityRate(t *testing.T) {
	src := resamplePCM([]int16{10, 20, 30, 40})
	r, _ := newResampleSeeker(bytes.NewReader(src), 44100, 44100, int64(len(src)))
	got, _ := io.ReadAll(r)
	if f := resampleDecode(got); len(f) != 4 || f[0] != 10 || f[3] != 40 {
		t.Errorf("a 1:1 conversion must pass the frames through: %v", f)
	}
}
