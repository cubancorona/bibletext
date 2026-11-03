package bibletext

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Run the dependency-free Java offset mapper itself; Android's layout and UI
// lifecycle remain native-runtime checks rather than claims of this test.
func TestAndroidReflowMapsSemanticOffsetsAndSelection(t *testing.T) {
	javac := jdkTool(t)
	dir := t.TempDir()
	source, err := os.ReadFile("android/BtReadingReflow.java")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "BtReadingReflow.java"), source, 0600); err != nil {
		t.Fatal(err)
	}
	program := `package org.bibletext;
public class ReflowCheck {
    static void need(boolean ok, String message) { if (!ok) throw new AssertionError(message); }
    static String selected(String text, int[] range) {
        return range == null ? "" : text.substring(Math.min(range[0],range[1]), Math.max(range[0],range[1]));
    }
    public static void main(String[] args) {
        String oldPage = "First phrase.\n\nSecond First phrase.\n\u200eשלום 🌿 end.";
        String newPage = "First phrase.\nSecond First phrase.\nשלום 🌿 end.";
        int second = oldPage.indexOf("First", 5);
        int[] range = BtReadingReflow.selection(true, true, oldPage, oldPage.length(), second, second+5,
                newPage, newPage.length());
        need(selected(newPage,range).equals("First"), "selected text changed");
        need(range[0] == newPage.indexOf("First",5), "repeated text was matched to its first occurrence");
        // A width reimport can keep either page kind and its raw text.
        int[] samePage = BtReadingReflow.selection(true,true,oldPage,oldPage.length(),second,second+5,
                oldPage,oldPage.length());
        need(samePage != null && samePage[0] == second && samePage[1] == second+5,
                "same-kind unchanged-page reimport lost its selected repeated phrase");
        int[] reverse = BtReadingReflow.selection(true,true,oldPage,oldPage.length(),second+5,second,newPage,newPage.length());
        need(reverse[0] == range[1] && reverse[1] == range[0], "selection direction changed");
        need(BtReadingReflow.selection(false,true,oldPage,oldPage.length(),second,second+5,newPage,newPage.length()) == null,"arrival carried selection");
        need(BtReadingReflow.selection(true,false,oldPage,oldPage.length(),second,second+5,newPage,newPage.length()) == null,"new chapter carried selection");
        need(BtReadingReflow.selection(true,true,oldPage,oldPage.length(),second,second,newPage,newPage.length()) == null,"deselection resurrected");
        need(BtReadingReflow.selection(true,true,oldPage,oldPage.length(),second,second+5,newPage.replace("Second","Changed"),newPage.length()) == null,"changed wording carried selection");
        need(BtReadingReflow.selection(true,true,"a bc",4,0,1,"ab c",4) == null,"word boundaries discarded");
        String emoji="Text 🌿 שלום";
        int e=emoji.indexOf("🌿");
        need(BtReadingReflow.selection(true,true,emoji,emoji.length(),e,e+2,emoji,emoji.length()) != null,"whole emoji refused");
        need(BtReadingReflow.selection(true,true,emoji,emoji.length(),e,e+1,emoji,emoji.length()) == null,"split surrogate accepted");
        need(BtReadingReflow.selection(true,true,"Heading\u2005words\nText",18,0,7,"Heading words\nText",18) != null,"heading presentation gap not mapped");
        need(BtReadingReflow.selection(true,true,"one\u00a0two",7,4,7,"one two",7) != null,"NBSP not mapped");
        String scripture="Text 🌿 שלום";
        String a=scripture+"\nFootnote A", b=scripture+"\n\nFootnote B";
        need(BtReadingReflow.selection(true,true,a,scripture.length(),0,4,b,scripture.length()) != null,"unchanged Scripture refused beside changed apparatus");
        need(BtReadingReflow.selection(true,true,a,scripture.length(),0,a.length(),b,scripture.length()) == null,"apparatus-crossing selection carried");
        BtReadingReflow.Anchor anchor=new BtReadingReflow.Anchor(oldPage,second,.375f,false);
        need(anchor.map(newPage,oldPage.length(),newPage.length()) == newPage.indexOf("First",5),"anchor moved to other repeated phrase");
        need(anchor.lineFraction == .375f,"line fraction changed");
        need(new BtReadingReflow.Anchor(oldPage,second,-.25f,false).lineFraction == -.25f,"before-line inset distance lost");
        need(anchor.map(newPage.replace("Second","Changed"),oldPage.length(),newPage.length()) == -1,"anchor accepted changed text");
        need(new BtReadingReflow.Anchor(oldPage,0,0,true).top,"top anchor lost");
    }
}`
	if err := os.WriteFile(filepath.Join(dir, "ReflowCheck.java"), []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(javac, "-encoding", "UTF-8", "--release", "8", "-d", dir, filepath.Join(dir, "BtReadingReflow.java"), filepath.Join(dir, "ReflowCheck.java")).CombinedOutput(); err != nil {
		t.Fatalf("compile Java reflow mapper: %v\n%s", err, out)
	}
	java := filepath.Join(filepath.Dir(javac), "java")
	if out, err := exec.Command(java, "-cp", dir, "org.bibletext.ReflowCheck").CombinedOutput(); err != nil {
		t.Fatalf("Java reflow mapper: %v\n%s", err, out)
	}
}
