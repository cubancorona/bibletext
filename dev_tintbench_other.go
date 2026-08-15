//go:build bibletextdev && !darwin

package bibletext

// devAutoTintBench: see dev_tintbench_apple.go. The benchmark measures the
// native overlay's two repair paths, and there is no native overlay here.
func devAutoTintBench(*AppState) {}

// devForceBodyRebuild: see dev_tintbench_apple.go. There is no body/wash split
// off the Apple panes, so there is nothing to force.
func devForceBodyRebuild(*AppState) {}
