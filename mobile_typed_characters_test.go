package bibletext

// patches/fyne-2.7.4-mobile-typed-characters.patch makes the toolkit's mobile
// driver type every character but a control, as its desktop driver does.
// Without it a joined emoji typed into a field on Android, or into a field
// the toolkit draws on the iPhone and iPad, lost its zero-width joiners on the
// way in, and Persian and Indic text lost its non-joiners, though the app's
// own note rule (normalizeNote) keeps both. Its second half, in Android's
// GoNativeActivity, sends one delete per character a keyboard removes from
// the hidden EditText rather than one per UTF-16 unit, which deleted
// characters typed before an emoji.
//
// The patch's own tests type into a field through the driver. They live in
// patches/testdata/mobile-typed-characters_test.go, are copied into the
// generated toolkit, and run in the macOS CI job. Its Java check,
// patches/testdata/BtTypedDeletesCheck.java, counts the deletes for removed
// runs and runs in scripts/check-android-java.sh. These hold the rest:
//
//  1. the tracked patch still says what it must;
//  2. the build applies it after the patches it was made against, copies its
//     tests in, and CI runs every one of them, the Java check included;
//  3. a generated toolkit carries it, wherever one has been generated;
//  4. what the note field holds reaches the message, the link and the kept
//     record unchanged, on every composer path.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

const (
	typedCharactersPatch = "patches/fyne-2.7.4-mobile-typed-characters.patch"
	typedCharactersTests = "patches/testdata/mobile-typed-characters_test.go"
	typedCharactersCopy  = "internal/driver/mobile/bt_typed_characters_test.go"
	typedCharactersRun   = "go test -race ./internal/driver/mobile -run '^TestBTTyped' -count=1"
	typedDeletesCheck    = "patches/testdata/BtTypedDeletesCheck.java"
)

// The Android half's lines, as the patch adds and removes them.
const (
	typedDeletesPerUnit  = "for (int i = 0; i < count; i++) {"
	typedDeletesCounted  = "int characters = deletedCharacters(s, start, count);"
	typedDeletesLoop     = "for (int i = 0; i < characters; i++) {"
	typedDeletesByPoints = "return Character.codePointCount(s, start, start + count);"
)

func TestTypedCharactersPatchStillSaysWhatItMust(t *testing.T) {
	body := readRepoFile(t, typedCharactersPatch)
	for claim, want := range map[string]string{
		"targets the mobile driver":                    "+++ b/internal/driver/mobile/driver.go",
		"takes out the printable-only filter":          "-\tif strconv.IsPrint(r) {",
		"drops only controls and what is no character": "+\tif !utf8.ValidRune(r) || unicode.IsControl(r) {",
		"carries the line the build looks for":         "BibleText patch: type every character but a control",
		"targets the Android activity":                 "+++ b/internal/driver/mobile/app/GoNativeActivity.java",
		"takes out the delete per UTF-16 unit":         "-                            " + typedDeletesPerUnit,
		"counts a removed run's characters":            "+                            " + typedDeletesCounted,
		"sends a delete per character":                 "+                            " + typedDeletesLoop,
		"counts them in code points":                   "+        " + typedDeletesByPoints,
		"carries the delete line the build looks for":  "BibleText patch: delete what the keyboard deleted",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s no longer %s: %q is absent", typedCharactersPatch, claim, want)
		}
	}
}

// setupScriptVariable is the shell variable scripts/setup-fyne-patch.sh holds
// rel in.
func setupScriptVariable(t *testing.T, script, rel string) string {
	t.Helper()
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, `="`+rel+`"`) {
			return strings.TrimSpace(strings.SplitN(line, "=", 2)[0])
		}
	}
	t.Fatalf("no variable in scripts/setup-fyne-patch.sh holds %s, so a build would go without it", rel)
	return ""
}

func TestTheBuildAppliesTheTypedCharactersPatch(t *testing.T) {
	script := readRepoFile(t, "scripts/setup-fyne-patch.sh")

	apply := `patch -p1 -d "$DEST" < "$` + setupScriptVariable(t, script, typedCharactersPatch) + `"`
	at := strings.Index(script, apply)
	if at < 0 {
		t.Fatalf("%s is assigned but never applied with %s", typedCharactersPatch, apply)
	}
	// The patch was made against the driver.go the drawloop patch leaves, which
	// changes the same file above it.
	// The patch was made against the driver.go the drawloop patch leaves, and
	// the GoNativeActivity.java the newintent and night-mode patches leave,
	// each of which changes the same file above it.
	for _, before := range []string{
		"patches/fyne-2.7.4-ios-drawloop.patch",
		"patches/fyne-2.7.4-android-newintent.patch",
		"patches/fyne-2.7.4-android-night-mode.patch",
	} {
		applied := strings.Index(script, `patch -p1 -d "$DEST" < "$`+setupScriptVariable(t, script, before)+`"`)
		if applied < 0 || applied > at {
			t.Errorf("%s must be applied after %s, which it was made against", typedCharactersPatch, before)
		}
	}

	tests := `cp "$` + setupScriptVariable(t, script, typedCharactersTests) + `" "$DEST/` + typedCharactersCopy + `"`
	if !strings.Contains(script, tests) {
		t.Errorf("the patch's tests are not copied into the toolkit: %s is absent", tests)
	}
	if !strings.Contains(script, `grep -q "BibleText patch: type every character but a control"`) {
		t.Error("the script does not check that the typed-characters patch landed")
	}
	if !strings.Contains(script, `grep -q "BibleText patch: delete what the keyboard deleted"`) {
		t.Error("the script does not check that the patch's Android half landed")
	}
}

// The Java check runs where the Android Java is compiled, against the
// activity the patches generate, and CI runs that script (held by
// TestBothMobilePlatformsHaveACompileGate).
func TestTheAndroidJavaGateRunsTheTypedDeletesCheck(t *testing.T) {
	script := readRepoFile(t, "scripts/check-android-java.sh")
	generate := strings.Index(script, `BIBLETEXT_FYNE_DEST="$out/fyne" scripts/setup-fyne-patch.sh`)
	compile := strings.Index(script, `"$out/fyne/internal/driver/mobile/app/GoNativeActivity.java"`)
	withCheck := strings.Index(script, "    "+typedDeletesCheck+" ")
	run := strings.Index(script, `java -cp "$out/activity:$JAR" org.golang.app.BtTypedDeletesCheck`)
	switch {
	case generate < 0:
		t.Error("scripts/check-android-java.sh does not generate the patched toolkit")
	case compile < generate:
		t.Error("scripts/check-android-java.sh does not compile the generated GoNativeActivity.java after generating it")
	case withCheck < compile:
		t.Errorf("scripts/check-android-java.sh does not compile %s with the activity", typedDeletesCheck)
	case run < withCheck:
		t.Error("scripts/check-android-java.sh never runs the typed-deletes check")
	}

	check := readRepoFile(t, typedDeletesCheck)
	for claim, want := range map[string]string{
		"sits in the activity's package":    "package org.golang.app;",
		"is the class the script runs":      "final class BtTypedDeletesCheck {",
		"asks the activity":                 "GoNativeActivity.deletedCharacters(",
		"fails the script on a wrong count": "System.exit(1);",
	} {
		if !strings.Contains(check, want) {
			t.Errorf("%s no longer %s: %q is absent", typedDeletesCheck, claim, want)
		}
	}
}

// The CI step runs the copied tests by name, so a test added to the file
// under another name would never run.
func TestCIRunsEveryTypedCharactersRegression(t *testing.T) {
	ci := readRepoFile(t, ".github/workflows/ci.yml")
	start := strings.Index(ci, "\n  build-test-macos:")
	end := strings.Index(ci, "\n  build-test-windows:")
	if start < 0 || end < start {
		t.Fatal("the macOS job is not where this test looks for it")
	}
	job := ci[start:end]
	generate := strings.Index(job, "scripts/check-ios-pane.sh\n")
	step := strings.Index(job, "working-directory: build/ios-check/fyne\n        run: "+typedCharactersRun+"\n")
	if step < 0 {
		t.Fatalf("the macOS job has no step running %q in build/ios-check/fyne", typedCharactersRun)
	}
	if generate < 0 || generate > step {
		t.Error("the step runs before scripts/check-ios-pane.sh generates build/ios-check/fyne")
	}

	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(repoRoot(t), typedCharactersTests), nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", typedCharactersTests, err)
	}
	var names []string
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
			names = append(names, fn.Name.Name)
			if !strings.HasPrefix(fn.Name.Name, "TestBTTyped") {
				t.Errorf("%s in %s does not match the CI step's -run '^TestBTTyped', so CI never runs it",
					fn.Name.Name, typedCharactersTests)
			}
		}
	}
	if len(names) < 4 {
		t.Errorf("only %d tests found in %s (%v); the patch's proof has gone", len(names), typedCharactersTests, names)
	}
}

// The generated toolkit is regenerated from scratch by
// scripts/setup-fyne-patch.sh and is not tracked, so this runs only where it
// has been generated.
func TestPatchedToolkitTypesEveryCharacterButAControl(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "third_party", "fyne")
	driver, err := os.ReadFile(filepath.Join(dir, "internal", "driver", "mobile", "driver.go"))
	if err != nil {
		t.Skipf("third_party/fyne is absent (regenerated by scripts/setup-fyne-patch.sh, not tracked)")
	}
	src := string(driver)
	if !strings.Contains(src, "if !utf8.ValidRune(r) || unicode.IsControl(r) {") ||
		strings.Contains(src, "if strconv.IsPrint(r) {") {
		t.Error("the generated mobile driver still keeps only what strconv.IsPrint accepts; " +
			"run scripts/setup-fyne-patch.sh")
	}
	copied, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(typedCharactersCopy)))
	if err != nil || string(copied) != readRepoFile(t, typedCharactersTests) {
		t.Errorf("the generated toolkit's %s is missing or stale; run scripts/setup-fyne-patch.sh", typedCharactersCopy)
	}
	activity, err := os.ReadFile(filepath.Join(dir, "internal", "driver", "mobile", "app", "GoNativeActivity.java"))
	if err != nil {
		t.Fatalf("the generated toolkit has no GoNativeActivity.java: %v", err)
	}
	act := string(activity)
	if strings.Contains(act, typedDeletesPerUnit) || !strings.Contains(act, typedDeletesCounted) ||
		!strings.Contains(act, typedDeletesLoop) || !strings.Contains(act, typedDeletesByPoints) {
		t.Error("the generated GoNativeActivity still sends a delete per UTF-16 unit; run scripts/setup-fyne-patch.sh")
	}
}

// The app's half: on each composer path, the joined text the field holds is
// the note line of the message, the note inside the link and the kept
// record's words. The desktop card is the macOS, Windows and Linux path, the
// phone sheet's own field Android's, and the native field the iPhone's and
// iPad's, whose UITextView hands its text over unfiltered
// (note_entry_ios.go).
func TestATypedJoinedNoteIsSharedAndKeptAsTyped(t *testing.T) {
	notes := map[string]string{
		"a family emoji":                     "fixture \U0001F468‍\U0001F469‍\U0001F467 together",
		"a Persian word with its non-joiner": "می‌خواهم",
		"a Devanagari half form":             "क्‍ष",
	}
	for noteName, note := range notes {
		for _, path := range []struct {
			name          string
			phone, native bool
		}{
			{name: "desktop card"},
			{name: "phone sheet, its own field", phone: true},
			{name: "phone sheet, the native field", phone: true, native: true},
		} {
			t.Run(noteName+", "+path.name, func(t *testing.T) {
				st := sentNoteState()
				w, h := float32(900), float32(760)
				if path.phone {
					w, h = 390, 844
				}
				win := noteSheetWindow(t, st, path.phone, w, h)
				holdNoteSheetTimers(t)
				setNotesEnabled(true)
				sent := catchShares(t)
				typed := ""
				if path.native {
					nativeNoteField(t, win)
					prev := noteEntryTyped
					noteEntryTyped = func() string { return typed }
					t.Cleanup(func() { noteEntryTyped = prev })
				}

				promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
				p := topPopup(t, win)
				if path.native {
					typed = note
				} else {
					field := composerField(p)
					if field == nil {
						t.Fatalf("control: the composer has no field; texts %v", sheetTexts(p))
					}
					test.Type(field, note)
					if field.Text != note {
						t.Fatalf("control: the field holds %+q after typing %+q", field.Text, note)
					}
				}
				test.Tap(findTreeButton(p.Content, "Share"))

				if len(*sent) != 1 {
					t.Fatalf("one share must go out; got %d", len(*sent))
				}
				lines := strings.Split((*sent)[0], "\n")
				if lines[0] != note {
					t.Errorf("the message's note line is %+q, want %+q", lines[0], note)
				}
				target, ok := ParseShareLink(lines[len(lines)-1])
				if !ok || target.Note != note {
					t.Errorf("the link's note is %+q (parsed %v), want %+q", target.Note, ok, note)
				}
				mine := myNotes(t)
				if len(mine) != 1 || mine[0].Text != note {
					t.Errorf("the kept record is %+v, want one holding %+q", mine, note)
				}
			})
		}
	}
}
