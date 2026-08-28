//go:build !darwin && !android

package bibletext

// Rate conversion for the desktop audio engine.
//
// WHY THIS EXISTS. oto allows exactly ONE context per process, fixed at one
// sample rate for the life of the app. The engine used to adopt the first
// file's rate and then refuse anything else — on the theory that all the
// narration came from one pipeline and shared a rate. It does not: the WEB
// recordings mix 22050 Hz and 44100 Hz. So the first chapter played decided
// the rate, and every chapter at the other rate failed for the rest of the
// process — play a New Testament chapter first and the whole Old Testament
// was silent, or the reverse. The button just returned to idle; nothing said
// why.
//
// The context now fixes one rate up front and anything else is converted to
// it, so no file can be locked out by whatever happened to play first.
//
// The conversion is linear interpolation between adjacent frames. For speech
// that is inaudible from a better kernel, and 22050 -> 44100 is the exact 2x
// case where interpolation reduces to averaging neighbours.
//
// POSITION IS THE SUBTLE PART. Elapsed time is derived from bytes read
// (audio_other.go: bytes / rate*otoBytesPerFrame), and read-along highlighting
// rides on that number, so the conversion must present a consistent
// OUTPUT-rate byte stream: Read emits output frames, Seek takes output offsets
// and maps them back to the source, and Length reports the output length.
// Anything less and the highlight drifts against the voice on exactly the
// chapters this fix exists to make playable.

import (
	"encoding/binary"
	"fmt"
	"io"
)

// resampleSeeker converts 16-bit stereo PCM from one sample rate to another,
// presenting the OUTPUT rate's byte stream through io.ReadSeeker.
type resampleSeeker struct {
	src      io.ReadSeeker
	from, to int
	srcLen   int64 // source PCM bytes, 0 when unknown

	step float64 // source frames advanced per output frame
	frac float64 // position between prev and cur, in [0,1)

	prev, cur [2]int16
	primed    bool
	eof       bool

	frame [otoBytesPerFrame]byte
}

// newResampleSeeker wraps src. from and to must both be positive.
func newResampleSeeker(src io.ReadSeeker, from, to int, srcLen int64) (*resampleSeeker, error) {
	if from <= 0 || to <= 0 {
		return nil, fmt.Errorf("audio: cannot convert %d Hz to %d Hz", from, to)
	}
	return &resampleSeeker{
		src:    src,
		from:   from,
		to:     to,
		srcLen: srcLen,
		step:   float64(from) / float64(to),
	}, nil
}

// Length is the output-rate byte length, frame-aligned. Zero when the source
// length is unknown, matching go-mp3's own contract.
func (r *resampleSeeker) Length() int64 {
	if r.srcLen <= 0 {
		return 0
	}
	srcFrames := r.srcLen / otoBytesPerFrame
	outFrames := int64(float64(srcFrames) * float64(r.to) / float64(r.from))
	return outFrames * otoBytesPerFrame
}

// readSourceFrame reads one stereo frame from the source.
func (r *resampleSeeker) readSourceFrame() ([2]int16, bool) {
	var f [2]int16
	if _, err := io.ReadFull(r.src, r.frame[:]); err != nil {
		return f, false
	}
	f[0] = int16(binary.LittleEndian.Uint16(r.frame[0:2]))
	f[1] = int16(binary.LittleEndian.Uint16(r.frame[2:4]))
	return f, true
}

func (r *resampleSeeker) Read(p []byte) (int, error) {
	// Whole frames only: a partial frame would swap the channels of everything
	// after it, which is the failure the engine's own frame-alignment guards
	// exist to prevent.
	n := len(p) / otoBytesPerFrame * otoBytesPerFrame
	if n == 0 {
		return 0, nil
	}
	if !r.primed {
		var ok bool
		if r.prev, ok = r.readSourceFrame(); !ok {
			return 0, io.EOF
		}
		if r.cur, ok = r.readSourceFrame(); !ok {
			r.cur, r.eof = r.prev, true
		}
		r.primed = true
	}

	written := 0
	for written < n {
		if r.eof && r.frac >= 1 {
			break
		}
		l := int16(float64(r.prev[0]) + (float64(r.cur[0])-float64(r.prev[0]))*r.frac)
		rr := int16(float64(r.prev[1]) + (float64(r.cur[1])-float64(r.prev[1]))*r.frac)
		binary.LittleEndian.PutUint16(p[written+0:], uint16(l))
		binary.LittleEndian.PutUint16(p[written+2:], uint16(rr))
		written += otoBytesPerFrame

		r.frac += r.step
		for r.frac >= 1 {
			r.frac--
			if r.eof {
				r.frac = 1 // stop emitting on the next pass
				break
			}
			r.prev = r.cur
			var ok bool
			if r.cur, ok = r.readSourceFrame(); !ok {
				r.eof = true
			}
		}
	}
	if written == 0 {
		return 0, io.EOF
	}
	return written, nil
}

// Seek takes an OUTPUT byte offset and positions the source to match. Only the
// whences the engine uses are supported; io.SeekEnd needs a known length.
func (r *resampleSeeker) Seek(offset int64, whence int) (int64, error) {
	var outAbs int64
	switch whence {
	case io.SeekStart:
		outAbs = offset
	case io.SeekEnd:
		if r.Length() == 0 {
			return 0, fmt.Errorf("audio: cannot seek from the end of an unknown length")
		}
		outAbs = r.Length() + offset
	default:
		return 0, fmt.Errorf("audio: unsupported seek whence %d", whence)
	}
	if outAbs < 0 {
		outAbs = 0
	}
	outAbs -= outAbs % otoBytesPerFrame

	outFrames := outAbs / otoBytesPerFrame
	srcFrames := int64(float64(outFrames) * r.step)
	if _, err := r.src.Seek(srcFrames*otoBytesPerFrame, io.SeekStart); err != nil {
		return 0, err
	}
	// Restart the interpolation window at the new position: the fraction is
	// carried so a seek lands mid-frame exactly where the stream would have.
	r.primed, r.eof = false, false
	r.frac = float64(outFrames)*r.step - float64(srcFrames)
	return outAbs, nil
}
