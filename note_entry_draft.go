package bibletext

import "sync"

// nativeComposeDraft keeps native callbacks separate from the UI-owned sheet.
// Updating the text before posting its counter avoids losing the last edit when
// a submit tap is already waiting on the UI queue.
type nativeComposeDraft struct {
	mu         sync.Mutex
	owner      uint64
	open       bool
	text       string
	keyboardPx float32
}

func (d *nativeComposeDraft) begin(owner uint64, text string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.owner, d.open, d.text, d.keyboardPx = owner, true, text, 0
}

func (d *nativeComposeDraft) update(owner uint64, text string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.open || owner != d.owner {
		return false
	}
	d.text = text
	return true
}

func (d *nativeComposeDraft) keyboard(owner uint64, px float32) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.open || owner != d.owner {
		return false
	}
	d.keyboardPx = max(px, 0)
	return true
}

func (d *nativeComposeDraft) read(owner uint64) (string, float32, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if owner != d.owner {
		return "", 0, false
	}
	return d.text, d.keyboardPx, d.open
}

func (d *nativeComposeDraft) end(owner uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if owner == d.owner {
		d.open, d.keyboardPx = false, 0
	}
}
