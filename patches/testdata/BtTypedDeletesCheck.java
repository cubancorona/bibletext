package org.golang.app;

// The check for the GoNativeActivity half of
// patches/fyne-2.7.4-mobile-typed-characters.patch. scripts/check-android-java.sh
// compiles it with the activity as the patches leave it and runs it on the
// host's JVM: it calls only the activity's static deletedCharacters, which
// needs nothing from Android.
//
// An Android keyboard edits a hidden EditText, and the activity turns each
// edit into key presses for the field the toolkit draws. onTextChanged hands
// the run an edit inserts to keyboardTyped, which types it one character
// (code point) at a time; beforeTextChanged sends a keyboardDelete for each
// character of the run the edit removes, and each deletes one character from
// the field. The run's length arrives in UTF-16 units, and a character
// outside the Basic Multilingual Plane is two of them, so counting units
// deleted characters the keyboard had not.
final class BtTypedDeletesCheck {
    private static int failures = 0;

    // before is the hidden field's text before the edit, which starts with the
    // space the activity keeps there; removed is the run the edit takes out,
    // found at its last occurrence.
    private static void expect(String name, String before, String removed, int characters) {
        int start = before.lastIndexOf(removed);
        if (start < 0) {
            throw new IllegalArgumentException(name + ": the removed run is not in the text");
        }
        int sent = GoNativeActivity.deletedCharacters(before, start, removed.length());
        if (sent != characters) {
            failures++;
            System.err.println("FAIL " + name + ": " + removed.length() + " UTF-16 units removed, "
                    + sent + " deletes sent, want " + characters);
        } else {
            System.out.println("ok   " + name + ": " + removed.length() + " units, " + sent + " deletes");
        }
    }

    public static void main(String[] args) {
        String family = "👨‍👩‍👧";
        String profession = "👩‍⚕️";
        String persian = "می‌خواهم";

        // A backspace sent as KEYCODE_DEL: the EditText removes the whole emoji.
        expect("a family emoji, removed whole", " abcdef" + family, family, 5);
        // A backspace sent as deleteSurroundingTextInCodePoints(1, 0).
        expect("the family emoji's last character", " abcdef" + family, "👧", 1);
        expect("a profession emoji, removed whole", " a" + profession, profession, 4);
        expect("an emoji added in Unicode 16.0", " abc🫩", "🫩", 1);
        expect("a letter, an emoji and a letter", " xa😀b", "a😀b", 3);
        // Characters inside the Basic Multilingual Plane are one unit each, as before.
        expect("three letters", " abcdef", "def", 3);
        expect("a Persian word with its non-joiner", " " + persian, persian, 8);
        // A keyboard replacing what it typed: the field's whole run.
        String typed = "fixture " + family + " " + profession + " 🫩 " + persian;
        expect("a note of 29 characters in 34 units", " " + typed, typed, 29);
        expect("nothing removed", " abc", "", 0);

        if (failures > 0) {
            System.err.println(failures + " of the deletion counts are wrong: an Android keyboard's "
                    + "deletion would remove a different number of characters from the field");
            System.exit(1);
        }
        System.out.println("GoNativeActivity sends one delete per character a keyboard removes");
    }
}
