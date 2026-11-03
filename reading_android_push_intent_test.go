package bibletext

import (
	"strings"
	"testing"
)

func TestAndroidChapterPushDecisionPreservesPlacementPriority(t *testing.T) {
	for _, tc := range []struct {
		name                string
		unchanged, explicit bool
		restore             *restoreAnchor
		push, keepRestore   bool
	}{
		{"unchanged live reflow", true, false, nil, false, false},
		{"changed render", false, false, nil, true, false},
		{"explicit verse", true, true, nil, true, false},
		{"explicit top", true, true, nil, true, false},
		{"seed data swap arrival", true, true, nil, true, false},
		{"saved position", true, false, &restoreAnchor{Book: "John", Chapter: 3, Verse: 14}, true, true},
		{"history top", true, false, &restoreAnchor{Book: "John", Chapter: 3}, true, true},
		{"transient reflow", true, false, &restoreAnchor{Book: "John", Chapter: 3, Verse: 14, Reflow: true}, true, true},
		{"stale book no-op", true, false, &restoreAnchor{Book: "Mark", Chapter: 3}, false, false},
		{"stale chapter no-op", true, false, &restoreAnchor{Book: "John", Chapter: 2}, false, false},
		{"stale restore changed render", false, false, &restoreAnchor{Book: "Mark", Chapter: 3}, true, false},
		{"stale restore explicit arrival", true, true, &restoreAnchor{Book: "Mark", Chapter: 3}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &AppState{CurrentBook: "John", CurrentChapter: 3, restore: tc.restore}
			if got := androidReadingPushWanted(state, tc.unchanged, tc.explicit); got != tc.push {
				t.Fatalf("push = %v, want %v", got, tc.push)
			}
			if tc.keepRestore {
				if state.restore != tc.restore {
					t.Fatal("the selected restore was replaced")
				}
			} else if state.restore != nil {
				t.Fatal("a restore for another chapter survived the no-op decision")
			}
		})
	}
}

func TestAndroidChapterPushNormalizesBeforeReflowCapture(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		unchanged, explicit, capture bool
	}{
		{"changed page", false, false, true},
		{"explicit arrival", false, true, false},
		{"unchanged page", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &AppState{CurrentBook: "John", CurrentChapter: 3,
				restore: &restoreAnchor{Book: "Mark", Chapter: 2}}
			wanted := androidReadingPushWanted(state, tc.unchanged, tc.explicit)
			if state.restore != nil {
				t.Fatal("stale restore still suppresses live capture")
			}
			capture := shouldCaptureScrollRestore(state, true, !tc.unchanged, tc.explicit)
			if capture != tc.capture {
				t.Fatalf("capture = %v, want %v", capture, tc.capture)
			}
			if capture && !wanted {
				t.Fatal("a needed captured restore could be lost to the no-op gate")
			}
		})
	}
}

// The decision above is executed on the host; this contract binds its no-op
// result to the Android-only caller before any native restore arm is sent.
func TestAndroidChapterPushArmsOnlyAfterTheNoOpGate(t *testing.T) {
	source := readNativeSource(t, "reading_android.go")
	start := strings.Index(source, "func pushChapterHTML(")
	if start < 0 {
		t.Fatal("Android chapter push missing")
	}
	end := strings.Index(source[start:], "\n}\n\n// --- Reading-position persistence")
	if start < 0 || end < 0 {
		t.Fatal("Android chapter push boundary missing")
	}
	push := source[start : start+end]
	decision := strings.Index(push, "pushWanted := androidReadingPushWanted(state, fp == lastPushedChapterFP, explicitArrival)")
	capture := strings.Index(push, "if shouldCaptureScrollRestore(state,")
	skip := strings.Index(push, "if !pushWanted {")
	firstArm := strings.Index(push, "armPendingRestore(state)")
	repeatedArm := strings.Index(push, "armAndroidReadingRestore(r.Verse, r.Delta, r.Frac, true)")
	sticker := strings.Index(push, "chrome := pushNoteToOverlay(state)")
	if decision < 0 || capture < 0 || skip < 0 || firstArm < 0 || repeatedArm < 0 || sticker < 0 ||
		sticker > decision || decision > capture || capture > skip || skip > firstArm || skip > repeatedArm {
		t.Fatal("the normalized decision must precede capture and the no-op gate must precede both restore arms")
	}
	arm := firstArm
	if repeatedArm < arm {
		arm = repeatedArm
	}
	if !strings.Contains(push[skip:arm], "\n\t\treturn\n") {
		t.Fatal("a no-op chapter push can still reach a native restore arm")
	}
}
