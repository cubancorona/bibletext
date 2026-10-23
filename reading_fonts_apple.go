//go:build darwin

package bibletext

// REGISTERING THE SHIPPED READING FACES WITH CORETEXT.
//
// The Apple panes are fed HTML, and the HTML importer will NOT resolve an
// app-private font by name: probed on macOS, a stylesheet asking for the
// shipped face returns exactly what asking for the system serif returns, and it
// fails silently. So the family is replaced AFTER the import instead, run by
// run, at each run's own point size — which is why the em-derived sizes the
// panes depend on survive untouched.
//
// That sweep needs the face to exist as a font object, which is what this file
// provides. The bytes are already compiled into the binary
// (reading_fonts_embed.go); they are handed to CoreText for the lifetime of the
// process, with no file written and no bundle resource to keep in step.
//
// The HEBREW is not swept separately and needs no detection. The font the sweep
// installs carries a CASCADE LIST naming the Hebrew face, and CoreText falls
// through to it PER GLYPH: Latin and Greek stay in the reading face, Hebrew
// resolves to the Hebrew one, and nothing has to recognise a Hebrew run.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreText -framework CoreGraphics -framework CoreFoundation

#import <CoreText/CoreText.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>

// btRegisterFontBytes hands one font to CoreText for this process only.
// A failure is not fatal anywhere: the sweep then finds no font by that name,
// leaves the imported one alone, and the pane reads in the system serif exactly
// as it did before any of this.
//
// WHY THE DEPRECATED CALL. The replacement,
// CTFontManagerRegisterFontDescriptors, cannot register a font that exists only
// in memory: descriptors built from data carry no file URL, and it answers
// error 303 for both faces, after which neither family resolves. Taking the
// modern route would mean writing the fonts to disk and registering them by URL
// — a file to create, keep in step and clean up — to replace a call that works
// and whose deprecation is not a removal. The app's floors are macOS 12 and
// iOS 15; if this is ever withdrawn, the URL route is the fallback.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static void btRegisterFontBytes(const void *bytes, int len) {
    if (bytes == NULL || len <= 0) return;
    CFDataRef data = CFDataCreate(NULL, (const UInt8 *)bytes, (CFIndex)len);
    if (data == NULL) return;
    CGDataProviderRef prov = CGDataProviderCreateWithCFData(data);
    CFRelease(data);
    if (prov == NULL) return;
    CGFontRef font = CGFontCreateWithDataProvider(prov);
    CGDataProviderRelease(prov);
    if (font == NULL) return;
    CFErrorRef err = NULL;
    CTFontManagerRegisterGraphicsFont(font, &err);
    CGFontRelease(font);
    if (err != NULL) CFRelease(err);
}
#pragma clang diagnostic pop


// btReadingFaceNamed is the face CoreText knows by the PostScript name `name`,
// at `size`, carrying a cascade list to the Hebrew face; NULL when CoreText
// does not have that face. Asked for a name it does not know,
// CTFontCreateWithFontDescriptor does not fail: it gives another font
// (Helvetica, upright and regular), which would replace every run of a page,
// the italic title and the bold headings the panes find by their traits
// among them. So what comes back is checked by its name, as btFontAvailable
// checks a family.
static CTFontRef btReadingFaceNamed(const char *name, CGFloat size) {
    if (name == NULL) return NULL;
    CFStringRef ps = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (ps == NULL) return NULL;
    CTFontDescriptorRef base = CTFontDescriptorCreateWithNameAndSize(ps, size);
    if (base == NULL) {
        CFRelease(ps);
        return NULL;
    }

    CFStringRef heb = CFStringCreateWithCString(NULL, "EzraSIL", kCFStringEncodingUTF8);
    CTFontDescriptorRef hebDesc = heb ? CTFontDescriptorCreateWithNameAndSize(heb, size) : NULL;
    if (heb != NULL) CFRelease(heb);

    CTFontRef out = NULL;
    if (hebDesc != NULL) {
        CFArrayRef cascade = CFArrayCreate(NULL, (const void **)&hebDesc, 1, &kCFTypeArrayCallBacks);
        if (cascade != NULL) {
            CFStringRef keys[1] = { kCTFontCascadeListAttribute };
            CFTypeRef vals[1] = { cascade };
            CFDictionaryRef attrs = CFDictionaryCreate(NULL, (const void **)keys, vals, 1,
                &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
            if (attrs != NULL) {
                CTFontDescriptorRef full = CTFontDescriptorCreateCopyWithAttributes(base, attrs);
                CFRelease(attrs);
                if (full != NULL) {
                    out = CTFontCreateWithFontDescriptor(full, size, NULL);
                    CFRelease(full);
                }
            }
            CFRelease(cascade);
        }
        CFRelease(hebDesc);
    }
    if (out == NULL) out = CTFontCreateWithFontDescriptor(base, size, NULL);
    CFRelease(base);
    if (out != NULL) {
        CFStringRef got = CTFontCopyPostScriptName(out);
        int same = got != NULL && CFStringCompare(got, ps, 0) == kCFCompareEqualTo;
        if (got != NULL) CFRelease(got);
        if (!same) {
            CFRelease(out);
            out = NULL;
        }
    }
    CFRelease(ps);
    return out;
}

// btReadingFontLike returns the shipped reading face at the SAME point size and
// the same bold/italic traits as src, carrying a cascade list to the Hebrew
// face. Returns NULL when the faces are not registered (btReadingFaceNamed),
// and the caller then leaves the run alone.
//
// Same size, because the panes read meaning from point size: the verse number
// is 0.66 of the body and the footnote section 0.85, and two thresholds tell
// them apart. Changing a size here would break both.
//
// The cascade is why nothing has to recognise a Hebrew run. CoreText resolves
// per GLYPH, so Latin and Greek come from the reading face and Hebrew falls
// through to the Hebrew one inside the very same run.
// NOT static: each pane's cgo preamble is its own translation unit, so the
// two reading panes declare this extern and link against the one definition.
CTFontRef btReadingFontLike(CTFontRef src) {
    if (src == NULL) return NULL;
    CGFloat size = CTFontGetSize(src);
    CTFontSymbolicTraits t = CTFontGetSymbolicTraits(src);
    const char *name = "Junicode-Regular";
    if ((t & kCTFontTraitBold) && (t & kCTFontTraitItalic)) name = "Junicode-BoldItalic";
    else if (t & kCTFontTraitBold)                          name = "Junicode-Bold";
    else if (t & kCTFontTraitItalic)                        name = "Junicode-Italic";
    return btReadingFaceNamed(name, size);
}

// btReadingFontOfSize is the reading face at a size with the bold and italic
// traits asked for: what btReadingFontLike gives a run of the stylesheet's
// family (Georgia, buildChapterHTML) set at that size and weight, and that
// run itself, Georgia with those traits, when the reading faces are not
// registered. For the page the panes set without the importer when a chapter
// will not import (reading_apple_fallback.go). Not static, like
// btReadingFontLike. The caller releases the result; NULL when no font can be
// made.
CTFontRef btReadingFontOfSize(CGFloat size, CTFontSymbolicTraits traits) {
    CTFontRef base = CTFontCreateWithName(CFSTR("Georgia"), size, NULL);
    if (base == NULL) return NULL;
    if (traits != 0) {
        CTFontRef styled = CTFontCreateCopyWithSymbolicTraits(base, size, NULL, traits, traits);
        if (styled != NULL) {
            CFRelease(base);
            base = styled;
        }
    }
    CTFontRef face = btReadingFontLike(base);
    if (face == NULL) return base;
    CFRelease(base);
    return face;
}

// btFontNameOf writes f's PostScript name into buf and releases f; 0 when f is
// NULL. It and the two below let the host tests read what the helpers above
// give.
static int btFontNameOf(CTFontRef f, char *buf, int len) {
    if (f == NULL) return 0;
    CFStringRef n = CTFontCopyPostScriptName(f);
    CFRelease(f);
    if (n == NULL) return 0;
    int ok = CFStringGetCString(n, buf, len, kCFStringEncodingUTF8) ? 1 : 0;
    CFRelease(n);
    return ok;
}

static int btReadingFaceNamedName(const char *name, double size, char *buf, int len) {
    return btFontNameOf(btReadingFaceNamed(name, (CGFloat)size), buf, len);
}

static int btReadingFontOfSizeName(double size, int bold, int italic, char *buf, int len) {
    CTFontSymbolicTraits t = 0;
    if (bold) t |= kCTFontTraitBold;
    if (italic) t |= kCTFontTraitItalic;
    return btFontNameOf(btReadingFontOfSize((CGFloat)size, t), buf, len);
}

// btFontAvailable reports whether CoreText can now resolve a family by name.
static int btFontAvailable(const char *name) {
    if (name == NULL) return 0;
    CFStringRef s = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (s == NULL) return 0;
    CTFontRef f = CTFontCreateWithName(s, 12.0, NULL);
    int ok = 0;
    if (f != NULL) {
        CFStringRef fam = CTFontCopyFamilyName(f);
        // CTFontCreateWithName never fails: an unknown name yields a default
        // face. Compare what came back, or every check would pass.
        ok = (fam != NULL && CFStringCompare(fam, s, 0) == kCFCompareEqualTo) ? 1 : 0;
        if (fam != NULL) CFRelease(fam);
        CFRelease(f);
    }
    CFRelease(s);
    return ok;
}
*/
import "C"

import (
	"sync"
	"unsafe"
)

// readingFaceFamily and hebrewFaceFamily are the family names CoreText knows
// the shipped faces by, read from their own name tables. The sweep asks for
// them by name, so they must match the files in assets/fonts/reading.
const (
	readingFaceFamily = "Junicode"
	hebrewFaceFamily  = "Ezra SIL"
)

var (
	appleFontsOnce sync.Once
	appleFontsOK   bool
)

// registerAppleReadingFonts hands the shipped faces to CoreText, once per
// process. Safe to call from anywhere and any number of times; the panes call
// it before the first import, because a font registered afterwards is of no use
// to a sweep that has already run.
func registerAppleReadingFonts() bool {
	appleFontsOnce.Do(func() {
		for _, b := range [][]byte{
			readingFontRegular, readingFontItalic,
			readingFontBold, readingFontBoldItalic,
			readingFontHebrew,
		} {
			if len(b) == 0 {
				continue
			}
			C.btRegisterFontBytes(unsafe.Pointer(&b[0]), C.int(len(b)))
		}
		// Checked by outcome, not by the call: both families must actually
		// resolve, or the sweep would install nothing and the cascade would
		// have nothing to fall through to.
		appleFontsOK = fontFamilyAvailable(readingFaceFamily) &&
			fontFamilyAvailable(hebrewFaceFamily)
	})
	return appleFontsOK
}

// fontFamilyAvailable reports whether CoreText can resolve a family by name.
func fontFamilyAvailable(name string) bool {
	c := C.CString(name)
	defer C.free(unsafe.Pointer(c))
	return C.btFontAvailable(c) == 1
}

// readingFaceName is the PostScript name of the face btReadingFaceNamed gives
// for the PostScript name name at size, or "" when it gives none.
func readingFaceName(name string, size float64) string {
	c := C.CString(name)
	defer C.free(unsafe.Pointer(c))
	var buf [256]C.char
	if C.btReadingFaceNamedName(c, C.double(size), &buf[0], C.int(len(buf))) == 0 {
		return ""
	}
	return C.GoString(&buf[0])
}

// readingFontOfSizeName is the PostScript name of the font btReadingFontOfSize
// gives at size with the traits asked for, the one the fallback page is set
// in, or "" when it gives none.
func readingFontOfSizeName(size float64, bold, italic bool) string {
	var b, i C.int
	if bold {
		b = 1
	}
	if italic {
		i = 1
	}
	var buf [256]C.char
	if C.btReadingFontOfSizeName(C.double(size), b, i, &buf[0], C.int(len(buf))) == 0 {
		return ""
	}
	return C.GoString(&buf[0])
}
