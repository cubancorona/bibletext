package bibletext

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

// seedGospelsJSON is the public-domain World English Bible Gospels (Matthew–John),
// embedded in the binary (~0.7 MB) so a first run with neither a cache nor a network
// connection still opens to readable scripture. See loadSeedGospels.
//
//go:embed assets/seed/web-gospels.json
var seedGospelsJSON []byte

// loadSeedGospels decodes the embedded WEB Gospels. It is the OFFLINE FALLBACK used
// when the first-ever load can neither read a cache nor reach the network: instead of
// a dead-end "couldn't load" screen, the app opens to Matthew–John. The seed is held
// in memory only and never written to the on-disk cache, so the next launch with a
// connection fetches and caches the complete Bible.
func loadSeedGospels() (*BibleData, error) {
	bd := &BibleData{}
	if err := json.Unmarshal(seedGospelsJSON, bd); err != nil {
		return nil, fmt.Errorf("decode embedded gospels seed: %w", err)
	}
	if len(bd.Books) == 0 {
		return nil, fmt.Errorf("embedded gospels seed is empty")
	}
	bd.PrepareSearchIndex()
	return bd, nil
}
