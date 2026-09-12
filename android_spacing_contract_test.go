package bibletext

// The Android bridge cannot import Go, so its five spacing constants are
// copies of reading_spacing.go's. This holds them equal: change a number in
// Go and this names the Java line to change with it. Mutation guarded: any of
// the five drifting, or the declaration being reshaped so the read matches
// nothing (the control).

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

func TestAndroidSpacingConstantsEqualGo(t *testing.T) {
	src, err := os.ReadFile("android/BtBridge.java")
	if err != nil {
		t.Fatalf("cannot read the bridge: %v", err)
	}
	re := regexp.MustCompile(`PARA_GAP_EM = ([0-9.]+)f, HEAD_LEAD_EM = ([0-9.]+)f, HEAD_TAIL_EM = ([0-9.]+)f, TITLE_GAP_EM = ([0-9.]+)f, INDENT_EM = ([0-9.]+)f`)
	m := re.FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("the bridge's spacing constants are no longer declared in the shape this test reads; " +
			"re-point the test rather than dropping it")
	}
	for i, want := range []struct {
		name string
		v    float64
	}{{"PARA_GAP_EM", readingParaGapEm}, {"HEAD_LEAD_EM", readingHeadLeadEm}, {"HEAD_TAIL_EM", readingHeadTailEm}, {"TITLE_GAP_EM", readingTitleGapEm}, {"INDENT_EM", reporterIndentEm}} {
		got, err := strconv.ParseFloat(m[i+1], 64)
		if err != nil || got != want.v {
			t.Errorf("android/BtBridge.java %s = %s; reading_spacing.go says %g", want.name, m[i+1], want.v)
		}
	}
}
