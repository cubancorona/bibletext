package bibletext

// THE BRING-YOUR-OWN-KEY TRANSLATION SITS AT THE TOP WHETHER OR NOT A KEY IS
// PRESENT.
//
// The picker used to order by "is it selectable right now", which put a keyless
// NKJV in the same bucket as a translation awaiting a licence and sank it below
// the public-domain three. Those are not the same situation: one is a free key
// away and its own row says so, the other nobody can unlock. Ordering by what
// the reader can act on keeps the offering visible on a fresh install, which is
// exactly the device that has no key.

import (
	"testing"
)

func firstPickerID(t *testing.T) string {
	t.Helper()
	order := versionPickerOrder()
	if len(order) == 0 {
		t.Fatal("the picker has no versions at all")
	}
	return order[0].ID
}

func TestBYOKVersionLeadsThePickerWithoutAKey(t *testing.T) {
	// No key, no licence env: the NKJV is NOT selectable here.
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")

	nkjv, ok := versionByID("nkjv")
	if !ok {
		t.Skip("nkjv is not registered in this build")
	}
	if nkjv.canSelect() {
		t.Skip("a key is configured in this environment; the keyless case cannot be tested")
	}
	if got := firstPickerID(t); got != "nkjv" {
		t.Errorf("picker leads with %q without a key, want nkjv — a translation the "+
			"reader can unlock themselves must not sink below the public-domain ones", got)
	}
}

func TestBYOKVersionStillLeadsThePickerWithAKey(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "test-key")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "1")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "test-bible-id")

	if _, ok := versionByID("nkjv"); !ok {
		t.Skip("nkjv is not registered in this build")
	}
	if got := firstPickerID(t); got != "nkjv" {
		t.Errorf("picker leads with %q with a key, want nkjv", got)
	}
}

// A version nobody can unlock still goes last — the point is the DISTINCTION,
// not simply promoting everything licensed.
func TestUnlicensedVersionStaysAtTheBottom(t *testing.T) {
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")

	// They form a TRAILING BLOCK — not "each is last", which is only true when
	// there is exactly one of them. A build with both evaluation translations
	// registered (-tags nrsv,lsb) has two, and demanding the final index of each
	// fails on a correct layout.
	order := versionPickerOrder()
	seenDead := false
	for i, v := range order {
		unlockableByNobody := !v.canSelect() && !byokCapable(v)
		if unlockableByNobody {
			seenDead = true
			continue
		}
		if seenDead {
			t.Errorf("%q at %d is usable or unlockable, yet follows a version "+
				"nobody can unlock — those belong last", v.ID, i)
		}
	}
}
