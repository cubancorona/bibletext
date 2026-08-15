//go:build !darwin

package bibletext

// The non-Apple half of the wash model's platform question — see
// reading_tint_apple.go for the whole of it.
//
// There is no live-mutation seam here: Android hands its TextView a whole
// `Spanned` and the Windows/Linux styled pane redraws its own tint rectangles, so
// a wash change is a re-render on both, and a re-render already carries the
// scroll. Nothing on these platforms should be asked to declare a reposition it
// gets for free.
const washIsLiveMutation = false
