package bibletext

import (
	"regexp"
	"slices"
	"testing"
)

// The menu gate changes availability without changing fixed actions or order.
func TestAskQuestionMenuGate(t *testing.T) {
	fixed := []aiStudyVerb{{aiActionExplain, "Explain"}, {aiActionContext, "Analyze context"}, {aiActionTranslation, "Analyze translation"}, {aiActionCompare, "Compare perspectives"}}
	if got := aiStudyVerbsForAsk(false); !slices.Equal(got, fixed) {
		t.Errorf("gate off: %v", got)
	}
	all := append(slices.Clone(fixed), aiStudyVerb{aiActionAsk, "Ask a question…"})
	if got := aiStudyVerbsForAsk(true); !slices.Equal(got, all) {
		t.Errorf("gate on: %v", got)
	}
	if askQuestionEnabled != nextRelease {
		t.Error("Ask gate must follow the next-release switch")
	}
	if !slices.Equal(aiStudyVerbs, aiStudyVerbsForAsk(askQuestionEnabled)) {
		t.Error("active menu ignores the Ask gate")
	}
}

// Native menus receive the Go gate rather than retaining an independent default.
// These sources are not compiled by an ordinary host test run.
func TestNativeAskMenusMirrorTheCodeGate(t *testing.T) {
	for _, tc := range []struct {
		path     string
		patterns []string
	}{
		{"reading_ios.go", []string{
			`static int gBTAskEnabled = 0;`,
			`void btIOSSetAskEnabled\(int on\) \{ gBTAskEnabled = on; \}`,
			`if \(gBTAskEnabled\) \[aiItems addObject:make\(@"Ask a question…", @"ask"\)\];`,
			`askOn := C.int\(0\)\s+if askQuestionEnabled \{\s+askOn = 1\s+\}\s+C.btIOSSetAskEnabled\(askOn\)`,
		}},
		{"reading_macos.go", []string{
			`static int gBTMacAskEnabled = 0;`,
			`void btMacSetAskEnabled\(int on\) \{ gBTMacAskEnabled = on; \}`,
			`if \(\[pair\[1\] isEqualToString:@"ask"\] && !gBTMacAskEnabled\) continue;`,
			`askOn := C.int\(0\)\s+if askQuestionEnabled \{\s+askOn = 1\s+\}\s+C.btMacSetAskEnabled\(askOn\)`,
		}},
		{"reading_android.go", []string{
			`GetStaticMethodID\(env, btaClass, "setAskEnabled", "\(Z\)V"\)`,
			`CallStaticVoidMethod\(env, btaClass, btaSetAskEnabledM, on \? JNI_TRUE : JNI_FALSE\)`,
			`askOn := C.int\(0\)\s+if askQuestionEnabled \{\s+askOn = 1\s+\}\s+C.btaSetAskEnabled\(C.uintptr_t\(env\), askOn\)`,
		}},
		{"android/BtBridge.java", []string{
			`private static volatile boolean askEnabled = false;`,
			`public static void setAskEnabled\(boolean on\) \{ askEnabled = on; \}`,
			`if \("ask".equals\(action\) && !askEnabled\) continue;`,
		}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			src := readNativeSource(t, tc.path)
			for _, pattern := range tc.patterns {
				re := regexp.MustCompile(pattern)
				if !re.MatchString(src) {
					t.Errorf("missing native Ask gate: %s", pattern)
				}
				if stripped := re.ReplaceAllString(src, ""); re.MatchString(stripped) {
					t.Errorf("control: removed gate still matches %s", pattern)
				}
			}
		})
	}
}
