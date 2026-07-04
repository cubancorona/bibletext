//go:build android

package bibletext

// Native-Android reading pane: a real android.widget.TextView (selectable)
// inside a ScrollView, floated above the Fyne GL surface — the Android twin of
// the iOS UITextView overlay (reading_ios.go). The Java half lives in
// android/BtBridge.java, shipped as classes2.dex by scripts/build-android.sh;
// this file drives it over JNI from fyne's RunNative (a JNI-attached background
// thread; BtBridge hops every UI mutation onto the Android main thread itself).
//
// The reader gets native long-press selection with the floating toolbar, plus
// BibleText's study cluster (Ask/Explain/Context/Translation, Cross-references,
// Share with citation) — the same action strings and downstream code as iOS.
// Callbacks Java→Go are in reading_android_export.go (empty cgo preamble, per
// the //export rule).

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static jclass    btaClass = NULL;   // global ref to org.bibletext.BtBridge
static jmethodID btaInitM, btaSetStyleM, btaSetHtmlM, btaArmRestoreM, btaGetFracM,
                 btaSetFrameM, btaShowM, btaHideM, btaSuppressM, btaUnsuppressM, btaShareTextM;

// Resolve BtBridge through the ACTIVITY's classloader. FindClass on a
// JNI-attached background thread uses the system classloader and cannot see
// app dex classes — the documented gomobile trap — so we go
// ctx.getClass().getClassLoader().loadClass("org.bibletext.BtBridge").
static int btaEnsureClass(JNIEnv *env, jobject ctx) {
	if (btaClass != NULL) {
		return 1;
	}
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getCl = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject cl = (*env)->CallObjectMethod(env, ctx, getCl);
	jclass clCls = (*env)->GetObjectClass(env, cl);
	jmethodID loadClass = (*env)->GetMethodID(env, clCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring name = (*env)->NewStringUTF(env, "org.bibletext.BtBridge");
	jobject cls = (*env)->CallObjectMethod(env, cl, loadClass, name);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		return 0; // classes2.dex missing (plain `fyne package` build) — overlay stays off
	}
	btaClass = (jclass)(*env)->NewGlobalRef(env, cls);

	btaInitM       = (*env)->GetStaticMethodID(env, btaClass, "init", "(Landroid/app/Activity;)V");
	btaSetStyleM   = (*env)->GetStaticMethodID(env, btaClass, "setStyle", "(IIFFIIII)V");
	btaSetHtmlM    = (*env)->GetStaticMethodID(env, btaClass, "setHtml", "(Ljava/lang/String;F)V");
	btaArmRestoreM = (*env)->GetStaticMethodID(env, btaClass, "armRestore", "(F)V");
	btaGetFracM    = (*env)->GetStaticMethodID(env, btaClass, "getScrollFrac", "()F");
	btaSetFrameM   = (*env)->GetStaticMethodID(env, btaClass, "setFrame", "(IIII)V");
	btaShowM       = (*env)->GetStaticMethodID(env, btaClass, "show", "()V");
	btaHideM       = (*env)->GetStaticMethodID(env, btaClass, "hide", "()V");
	btaSuppressM   = (*env)->GetStaticMethodID(env, btaClass, "suppress", "()V");
	btaUnsuppressM = (*env)->GetStaticMethodID(env, btaClass, "unsuppress", "()V");
	btaShareTextM  = (*env)->GetStaticMethodID(env, btaClass, "shareText", "(Ljava/lang/String;)V");
	// A missing method (a dex/JNI signature skew from editing BtBridge.java
	// without updating these descriptors) returns NULL and leaves a pending
	// NoSuchMethodError; every wrapper below guards only on btaClass==NULL, so an
	// unchecked NULL jmethodID would later SIGSEGV in CallStatic*Method. Treat any
	// skew as "bridge absent" — clear the exception, drop the class, fall back to
	// the Fyne reading pane (mirrors Fyne's own find_static_method helper).
	if ((*env)->ExceptionCheck(env) ||
	    btaInitM == NULL || btaSetStyleM == NULL || btaSetHtmlM == NULL ||
	    btaArmRestoreM == NULL || btaGetFracM == NULL || btaSetFrameM == NULL ||
	    btaShowM == NULL || btaHideM == NULL || btaSuppressM == NULL ||
	    btaUnsuppressM == NULL || btaShareTextM == NULL) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteGlobalRef(env, btaClass);
		btaClass = NULL;
		return 0;
	}
	return 1;
}

static int btaInit(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (!btaEnsureClass(env, (jobject)ctx)) {
		return 0;
	}
	(*env)->CallStaticVoidMethod(env, btaClass, btaInitM, (jobject)ctx);
	return 1;
}

static int btaReady() { return btaClass != NULL; }

static void btaSetStyle(uintptr_t jni_env, int textColor, int paperColor, float sizePx,
                        float lineMult, int padL, int padT, int padR, int padB) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetStyleM, textColor, paperColor,
	                             sizePx, lineMult, padL, padT, padR, padB);
}

static void btaSetHtml(uintptr_t jni_env, const char *html, float frac) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jstring s = (*env)->NewStringUTF(env, html);
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetHtmlM, s, frac);
	(*env)->DeleteLocalRef(env, s);
}

static void btaArmRestore(uintptr_t jni_env, float frac) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaArmRestoreM, frac);
}

static float btaGetFrac(uintptr_t jni_env) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return -1.0f;
	return (*env)->CallStaticFloatMethod(env, btaClass, btaGetFracM);
}

static void btaSetFrame(uintptr_t jni_env, int x, int y, int w, int h) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetFrameM, x, y, w, h);
}

static void btaSimple(uintptr_t jni_env, int which) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jmethodID m = which == 0 ? btaShowM : which == 1 ? btaHideM : which == 2 ? btaSuppressM : btaUnsuppressM;
	(*env)->CallStaticVoidMethod(env, btaClass, m);
}

static void btaShareText(uintptr_t jni_env, const char *body) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jstring s = (*env)->NewStringUTF(env, body);
	(*env)->CallStaticVoidMethod(env, btaClass, btaShareTextM, s);
	(*env)->DeleteLocalRef(env, s);
}
*/
import "C"

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"strings"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// btaAvailable reports whether the BtBridge dex class resolved — false in a
// plain `fyne package` build that skipped scripts/build-android.sh, in which
// case the reading pane falls back to the Fyne widget path.
var btaAvailable = false
var btaInitTried = false
var btaCtx uintptr // the activity jobject the bridge was last (re-)initialised for

// runBta wraps driver.RunNative: hands the callback an attached JNIEnv, after
// making sure the bridge class + activity are initialised. RunNative surfaces
// pending Java exceptions as errors — log them (Go's log reaches logcat under
// GoLog), or a broken bridge would silently no-op every overlay call.
func runBta(fn func(env uintptr)) {
	err := driver.RunNative(func(c any) error {
		ac, ok := c.(*driver.AndroidContext)
		if !ok {
			return nil
		}
		// (Re-)init when first seen OR when the activity changed. Android
		// recreates the activity (rotation, background→foreground), handing a
		// new Ctx; BtBridge.init then rebuilds its Dialog against the live
		// activity — a Dialog cached against a dead activity would throw
		// BadTokenException on show(). The C class + method IDs are cached, so
		// re-init is just a BtBridge.init(newActivity) call.
		if !btaInitTried || ac.Ctx != btaCtx {
			btaInitTried = true
			btaCtx = ac.Ctx
			btaAvailable = C.btaInit(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
			if !btaAvailable {
				// Bridge dex missing (plain `fyne package` build) — the reading
				// pane fell back to the Fyne widget path; note it once.
				log.Printf("bibletext: BtBridge dex not present; using Fyne reading fallback")
			}
		}
		if btaAvailable {
			fn(ac.Env)
		}
		return nil
	})
	if err != nil {
		log.Printf("bibletext: BtBridge call failed: %v", err)
	}
}

// --- The Fyne-side host widget (twin of iOS nativeReadingHost) ---------------

type nativeReadingHost struct {
	widget.BaseWidget
	state *AppState
}

var currentHost *nativeReadingHost

func newNativeReadingHost(state *AppState, verses []Verse) *nativeReadingHost {
	h := &nativeReadingHost{state: state}
	h.ExtendBaseWidget(h)
	currentHost = h
	pushChapterHTML(state, verses)
	return h
}

func (h *nativeReadingHost) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func (h *nativeReadingHost) Resize(size fyne.Size) {
	h.BaseWidget.Resize(size)
	h.pushFrame()
}

func (h *nativeReadingHost) Move(p fyne.Position) {
	h.BaseWidget.Move(p)
	h.pushFrame()
}

// pushFrame projects the host's canvas rect to the native overlay, immediately
// and again once the layout settles (Resize/Move fire mid-layout). The
// currentHost guard stops a swapped-out host's deferred push from clobbering
// the live one.
func (h *nativeReadingHost) pushFrame() {
	setFrameFromObject(h)
	time.AfterFunc(60*time.Millisecond, func() {
		fyne.Do(func() {
			if currentHost == h {
				setFrameFromObject(h)
			}
		})
	})
}

func setFrameFromObject(h *nativeReadingHost) {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(h)
	sz := h.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	scale := float32(1)
	if c := fyne.CurrentApp().Driver().CanvasForObject(h); c != nil {
		scale = c.Scale()
	}
	x := int(math.Round(float64(pos.X * scale)))
	y := int(math.Round(float64(pos.Y * scale)))
	w := int(math.Round(float64(sz.Width * scale)))
	hg := int(math.Round(float64(sz.Height * scale)))
	runBta(func(env uintptr) {
		C.btaSetFrame(C.uintptr_t(env), C.int(x), C.int(y), C.int(w), C.int(hg))
	})
}

// --- Overlay visibility (contract shared with ui_mobile.go) ------------------

func showNativeReadingOverlay() { runBta(func(env uintptr) { C.btaSimple(C.uintptr_t(env), 0) }) }
func hideNativeReadingOverlay() { runBta(func(env uintptr) { C.btaSimple(C.uintptr_t(env), 1) }) }

// notifyReadingOverlay is the single visibility entry point the shared mobile
// UI calls; visibility truth is always overlayShouldShow(state) at the caller.
func notifyReadingOverlay(visible bool) {
	if visible {
		showNativeReadingOverlay()
	} else {
		hideNativeReadingOverlay()
	}
}

// afterRebuild re-pins the overlay after the window tree is swapped, then
// re-asserts visibility LAST so a stray async show can't leave the overlay
// floating over the Books/Search tabs (same cadence as iOS).
func afterRebuild(state *AppState) {
	time.AfterFunc(150*time.Millisecond, func() {
		fyne.Do(func() {
			if overlayShouldShow(state) && currentHost != nil {
				setFrameFromObject(currentHost)
			}
			notifyReadingOverlay(overlayShouldShow(state))
		})
	})
}

// --- Chapter rendering --------------------------------------------------------

var lastPushedBookChapter string

// pushChapterHTML mirrors the iOS twin: fingerprint-gated re-import, restore
// arming, and a same-chapter re-render (theme flip) preserving the live scroll.
func pushChapterHTML(state *AppState, verses []Verse) {
	fp := chapterRenderFingerprint(state)
	bc := fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter)

	if state.restore == nil && bc == lastPushedBookChapter && fp != lastPushedChapterFP {
		if v, d, f, ok := captureReadingAnchor(); ok && (v > 0 || f > 0) {
			state.restore = &restoreAnchor{
				Book:    state.CurrentBook,
				Chapter: state.CurrentChapter,
				Verse:   v,
				Delta:   d,
				Frac:    f,
			}
		}
	}

	armPendingRestore(state)

	if state.restore == nil && fp == lastPushedChapterFP {
		return
	}
	lastPushedChapterFP = fp
	lastPushedBookChapter = bc

	frac := float32(-1)
	if state.restore != nil && state.restore.Frac > 0 {
		frac = float32(state.restore.Frac)
	}

	html := buildChapterHTMLAndroid(state, verses)
	pal := state.pal()
	scale := float32(1)
	if state.window != nil {
		scale = state.window.Canvas().Scale()
	}
	textPx := float32(21) * float32(readingTextScale()) * scale
	padL, padT := int(10*scale), int(14*scale)
	runBta(func(env uintptr) {
		C.btaSetStyle(C.uintptr_t(env),
			C.int(argbInt(pal.Text)), C.int(argbInt(pal.Background)),
			C.float(textPx), C.float(1.7),
			C.int(padL), C.int(padT), C.int(padL), C.int(padT))
		ch := C.CString(html)
		C.btaSetHtml(C.uintptr_t(env), ch, C.float(frac))
		C.free(unsafe.Pointer(ch))
	})
}

// buildChapterHTMLAndroid emits the Html.fromHtml-safe dialect of the chapter:
// no CSS classes (fromHtml ignores <style>), so verse numbers are
// <sup><small><font color><b>, red-letter is <font color>, and the search-jump
// highlight is an inline style= span (honored on API 24+).
func buildChapterHTMLAndroid(state *AppState, verses []Verse) string {
	pal := state.pal()
	vnum := nrgbaToHex(pal.VerseNumber)
	red := nrgbaToHex(pal.RedLetter)
	hlBG := nrgbaToHex(pal.Highlight)
	hlFG := nrgbaToHex(pal.HighlightText)

	redLetter := redLetterEnabled()
	var b strings.Builder
	for _, para := range groupVersesIntoParagraphs(verses) {
		b.WriteString("<p>")
		for i, v := range para {
			if i > 0 {
				b.WriteString(" ")
			}
			fmt.Fprintf(&b, `<sup><small><font color="%s"><b>%d</b></font></small></sup>&nbsp;`, vnum, v.Verse)
			body := htmlEscape(strings.TrimSpace(strings.ReplaceAll(v.Text, "\n", " ")))
			switch {
			case isVerseHighlighted(state, v):
				// A search highlight wins visually over red-letter (as on iOS).
				fmt.Fprintf(&b, `<span style="color:%s;background-color:%s"><b>%s</b></span>`, hlFG, hlBG, body)
			case redLetter && isWordsOfChrist(v.BookName, v.Chapter, v.Verse):
				fmt.Fprintf(&b, `<font color="%s">%s</font>`, red, body)
			default:
				b.WriteString(body)
			}
		}
		b.WriteString("</p>")
	}
	return b.String()
}

// --- Reading-position persistence (frac-based; see reading_scroll_android.go) --

// captureReadingAnchor reads the live scroll as a whole-chapter fraction.
// ok=true whenever the native view is alive (even at the top — ok means
// "readable", not "non-zero"); ok=false only when the overlay is absent, so a
// late lifecycle flush preserves the previously saved anchor instead of
// wiping it.
func captureReadingAnchor() (verse int, delta, frac float64, ok bool) {
	if !btaBridgePresent() {
		return 0, 0, 0, false
	}
	f := float32(-1)
	runBta(func(env uintptr) {
		f = float32(C.btaGetFrac(C.uintptr_t(env)))
	})
	if f < 0 {
		return 0, 0, 0, false
	}
	return 0, 0, float64(f), true
}

// armReadingRestore arms the one-shot scroll target. A verse-anchored save
// from an iOS session still carries Frac (always populated on capture), so
// ignoring verse/delta here loses nothing but precision. verse<=0 && frac<=0
// disarms. Not consumed on apply; dropped on the reader's first scroll
// (BtBridge's scroll listener clears it).
func armReadingRestore(verse int, delta, frac float64) {
	if !btaBridgePresent() {
		return
	}
	f := float32(frac)
	if verse <= 0 && frac <= 0 {
		f = -1 // disarm
	}
	runBta(func(env uintptr) {
		C.btaArmRestore(C.uintptr_t(env), C.float(f))
	})
}

// --- Share (text only on Android; share-as-image is iOS/macOS for now) -------

func nativeShareText(s string) {
	runBta(func(env uintptr) {
		cs := C.CString(s)
		C.btaShareText(C.uintptr_t(env), cs)
		C.free(unsafe.Pointer(cs))
	})
}

func nativeShareImage(path string) {
	// Needs a FileProvider (manifest + res xml) — deferred. The share-image
	// menu item is not offered on Android (BtBridge.java's menu omits it).
}

// --- buildReadingViewMobile (Android) — the iOS shape, minus audio ----------

func buildReadingViewMobile(state *AppState) fyne.CanvasObject {
	pal := state.pal()

	state.hideReadingOverlay = func() {
		runBta(func(env uintptr) { C.btaSimple(C.uintptr_t(env), 2) }) // suppress
	}
	state.showReadingOverlay = func() {
		runBta(func(env uintptr) { C.btaSimple(C.uintptr_t(env), 3) }) // unsuppress
		if overlayShouldShow(state) {
			showNativeReadingOverlay()
		} else {
			hideNativeReadingOverlay()
		}
	}

	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	if !btaBridgePresent() {
		// classes2.dex missing (plain fyne package build): fall back to the
		// old Fyne reading widget so the app still works.
		return buildReadingViewMobileFyne(state)
	}

	host := newNativeReadingHost(state, verses)
	paper := canvas.NewRectangle(pal.Background)

	if state.IsFullScreen {
		exit := widget.NewButtonWithIcon("", theme.ViewRestoreIcon(), func() {
			state.IsFullScreen = false
			rebuildWindow(state)
		})
		exit.Importance = widget.LowImportance
		ref := canvas.NewText(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.TextMuted)
		ref.TextSize = 16
		refBox := container.NewVBox(layout.NewSpacer(), ref, layout.NewSpacer())
		exitRow := container.NewBorder(nil, nil, refBox, exit, nil)
		body := container.NewBorder(exitRow, nil, nil, nil, container.NewStack(paper, host))
		return container.NewPadded(body)
	}

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(container.New(layout.NewCustomPaddedLayout(6, 4, 0, 0), bar))
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeaderMobile(state, chapterNumbers))

	header := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Padding(), 0), top)
	body := container.NewBorder(header, nil, nil, nil, container.NewStack(paper, host))
	return container.New(layout.NewCustomPaddedLayout(0, theme.Padding(), 0, theme.Padding()), body)
}

// btaBridgePresent triggers a lazy init probe so the fallback decision is real.
func btaBridgePresent() bool {
	if !btaInitTried {
		runBta(func(env uintptr) {}) // side effect: btaInit
	}
	return btaAvailable
}

// argbInt packs an NRGBA palette color into Android's ARGB int.
func argbInt(c color.NRGBA) int32 {
	return int32(c.A)<<24 | int32(c.R)<<16 | int32(c.G)<<8 | int32(c.B)
}
