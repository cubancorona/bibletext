//go:build !darwin && !android

package bibletext

// Desktop audio engine for Windows and Linux (and any other plain-Fyne host):
// recorded narration playback via oto (WASAPI on Windows, ALSA loaded at runtime
// through purego on Linux — no cgo at build time), decoding the narration MP3s
// with go-mp3. The native engines remain audio_ios.go / audio_macos.go /
// audio_android.go; this one speaks the same nativeAudio* shim + posts the same
// applyNativeState transitions, so audio_controller.go and the whole audio UI
// (button, source menu, continuous chapter advance) work unchanged.
//
// Scope vs the native engines — deliberate, documented in README/ARCHITECTURE:
//   - Recorded narration: play/pause, ±15s seek, natural-end detection (feeds
//     the controller's continuous chapter advance) — full support.
//   - Read-aloud TTS: NOT offered here (no cross-platform speech engine worth
//     shipping); ttsSupported() gates every surface that would expose it, and
//     chapters with no recording show no audio control at all.
//   - Media keys / lock-screen (MPRIS, SMTC), artwork: none yet — the in-app
//     button is the transport.
//   - Read-along highlight: readalong_stub.go stays no-op — the Fyne reading
//     pane has no per-verse highlight hook.
//
// Threading: everything runs on background goroutines; applyNativeState is
// designed to be called off-main (the cgo engines call it from native threads).
// A generation counter guards against every stale-callback hazard: Start/Stop
// bump it, and a goroutine belonging to an older generation never touches the
// player or posts state.

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	oto "github.com/ebitengine/oto/v3"
	mp3 "github.com/hajimehoshi/go-mp3"
)

const otoBytesPerFrame = 4 // go-mp3 always emits 16-bit stereo

type otoEngine struct {
	mu      sync.Mutex
	gen     int // bumped by start/stop; stale goroutines compare and bail
	ctx     *oto.Context
	ctxRate int
	player  *oto.Player
	dec     *mp3.Decoder
	paused  bool
}

var gOto otoEngine

// httpAudio is the client for narration downloads. Chapters are a few MB; the
// generous timeout covers slow links without hanging forever.
var httpAudio = &http.Client{Timeout: 120 * time.Second}

// post delivers a controller state transition. Direct call — applyNativeState
// is the same entry the native engines invoke from their own threads.
func (e *otoEngine) post(gen int, s audioPlayState) {
	e.mu.Lock()
	stale := gen != e.gen
	e.mu.Unlock()
	if !stale {
		gAudio.applyNativeState(s)
	}
}

// ensureContext creates the process-wide oto context on first use. oto allows
// exactly one context per process at one sample rate; the narration files all
// come from the same pipelines, so the first file's rate is the rate.
func (e *otoEngine) ensureContext(rate int) error {
	if e.ctx != nil {
		if e.ctxRate != rate {
			return fmt.Errorf("audio: context is %d Hz, file is %d Hz", e.ctxRate, rate)
		}
		return nil
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   rate,
		ChannelCount: 2,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return err
	}
	select {
	case <-ready:
	case <-time.After(10 * time.Second):
		return errors.New("audio: device not ready")
	}
	e.ctx = ctx
	e.ctxRate = rate
	return nil
}

// teardownLocked closes the current player. Callers hold e.mu.
func (e *otoEngine) teardownLocked() {
	if e.player != nil {
		_ = e.player.Close()
		e.player = nil
	}
	e.dec = nil
	e.paused = false
}

func nativeAudioStartURL(url, title, artist string) {
	e := &gOto
	e.mu.Lock()
	e.gen++
	gen := e.gen
	e.teardownLocked()
	e.mu.Unlock()

	go func() {
		e.post(gen, audioBuffering)

		resp, err := httpAudio.Get(url)
		if err != nil {
			e.post(gen, audioFailed)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			e.post(gen, audioFailed)
			return
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			e.post(gen, audioFailed)
			return
		}

		dec, err := mp3.NewDecoder(newSeekableBytes(data))
		if err != nil {
			e.post(gen, audioFailed)
			return
		}

		e.mu.Lock()
		if gen != e.gen { // superseded while downloading
			e.mu.Unlock()
			return
		}
		if err := e.ensureContext(dec.SampleRate()); err != nil {
			e.mu.Unlock()
			e.post(gen, audioFailed)
			return
		}
		p := e.ctx.NewPlayer(dec)
		e.player = p
		e.dec = dec
		e.paused = false
		p.Play()
		e.mu.Unlock()
		e.post(gen, audioPlaying)

		// Watcher: detect natural end (buffer drained while not paused) and
		// player errors. It belongs to this generation only.
		tick := time.NewTicker(500 * time.Millisecond)
		defer tick.Stop()
		for range tick.C {
			e.mu.Lock()
			if gen != e.gen {
				e.mu.Unlock()
				return
			}
			player, paused := e.player, e.paused
			e.mu.Unlock()
			if player == nil {
				return
			}
			if err := player.Err(); err != nil {
				e.mu.Lock()
				if gen == e.gen {
					e.teardownLocked()
				}
				e.mu.Unlock()
				e.post(gen, audioFailed)
				return
			}
			if !paused && !player.IsPlaying() {
				// Drained to the end: tear down and report ENDED so the
				// controller's continuous playback rolls to the next chapter.
				e.mu.Lock()
				if gen == e.gen {
					e.teardownLocked()
				}
				e.mu.Unlock()
				e.post(gen, audioEnded)
				return
			}
		}
	}()
}

// nativeAudioStartTTS is unreachable in practice: ttsSupported() is false here,
// which hides the read-aloud source and the whole button for recording-less
// chapters. Kept as a safe no-op for defense in depth.
func nativeAudioStartTTS(text, title, artist string) {}

func nativeAudioToggle() {
	e := &gOto
	e.mu.Lock()
	gen := e.gen
	p := e.player
	if p == nil {
		e.mu.Unlock()
		return
	}
	var next audioPlayState
	if e.paused {
		e.paused = false
		p.Play()
		next = audioPlaying
	} else {
		e.paused = true
		p.Pause()
		next = audioPaused
	}
	e.mu.Unlock()
	e.post(gen, next)
}

// nativeAudioStop tears down without posting state: the controller updates its
// own state on explicit stops, and a late "idle" from here could clobber a
// chapter that was started immediately after (the classic stale-callback bug
// the cgo engines guard with native mode checks — here the generation bump
// suffices).
func nativeAudioStop() {
	e := &gOto
	e.mu.Lock()
	e.gen++
	e.teardownLocked()
	e.mu.Unlock()
}

func nativeAudioSkip(seconds float64) {
	e := &gOto
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.player == nil || e.dec == nil {
		return
	}
	cur, err := e.player.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	total := e.dec.Length()
	next := cur + int64(seconds*float64(e.ctxRate))*otoBytesPerFrame
	next -= next % otoBytesPerFrame // frame-align, or the channels swap
	if next < 0 {
		next = 0
	}
	if total > 0 && next > total-otoBytesPerFrame {
		next = total - otoBytesPerFrame
	}
	_, _ = e.player.Seek(next, io.SeekStart)
}

// nativeAudioSetArtwork: no lock-screen / Now Playing surface on these hosts.
func nativeAudioSetArtwork(path string) {}

// seekableBytes adapts a byte slice as the io.ReadSeeker go-mp3 wants (and that
// oto's Player.Seek requires end-to-end).
type seekableBytes struct {
	b   []byte
	off int64
}

func newSeekableBytes(b []byte) *seekableBytes { return &seekableBytes{b: b} }

func (s *seekableBytes) Read(p []byte) (int, error) {
	if s.off >= int64(len(s.b)) {
		return 0, io.EOF
	}
	n := copy(p, s.b[s.off:])
	s.off += int64(n)
	return n, nil
}

func (s *seekableBytes) Seek(offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = s.off
	case io.SeekEnd:
		base = int64(len(s.b))
	default:
		return 0, fmt.Errorf("bad whence %d", whence)
	}
	n := base + offset
	if n < 0 {
		return 0, errors.New("negative seek")
	}
	s.off = n
	return n, nil
}
