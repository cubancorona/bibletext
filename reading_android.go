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
                 btaSetFrameM, btaShowM, btaHideM, btaSuppressM, btaUnsuppressM,
                 btaShareTextM, btaShareImageM, btaSetAIEnabledM,
                 btaRAHighlightM, btaRAClearM, btaRAFollowM, btaRAColorsM;

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
	btaShareImageM = (*env)->GetStaticMethodID(env, btaClass, "shareImage", "(Ljava/lang/String;)V");
	// Selection-menu AI gate — mirrors the Settings → Assistant choice; when off,
	// onCreateActionMode omits the "Study with AI" submenu (Share/Cross-refs stay).
	btaSetAIEnabledM = (*env)->GetStaticMethodID(env, btaClass, "setAIEnabled", "(Z)V");
	// Read-along (audio): highlight the narrated verse + the floating "Follow
	// narration" pill, both painted on this same overlay (reading_android.go owns
	// the BtBridge handle, so the audio read-along calls route through here).
	btaRAHighlightM = (*env)->GetStaticMethodID(env, btaClass, "readAlongHighlight", "(IZ)V");
	btaRAClearM     = (*env)->GetStaticMethodID(env, btaClass, "readAlongClear", "()V");
	btaRAFollowM    = (*env)->GetStaticMethodID(env, btaClass, "readAlongFollow", "(Z)V");
	btaRAColorsM    = (*env)->GetStaticMethodID(env, btaClass, "setReadAlongColors", "(III)V");
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
	    btaUnsuppressM == NULL || btaShareTextM == NULL || btaShareImageM == NULL ||
	    btaSetAIEnabledM == NULL ||
	    btaRAHighlightM == NULL || btaRAClearM == NULL || btaRAFollowM == NULL ||
	    btaRAColorsM == NULL) {
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

static void btaShareImage(uintptr_t jni_env, const char *path) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jstring s = (*env)->NewStringUTF(env, path);
	(*env)->CallStaticVoidMethod(env, btaClass, btaShareImageM, s);
	(*env)->DeleteLocalRef(env, s);
}

static void btaSetAIEnabled(uintptr_t jni_env, int on) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetAIEnabledM, on ? JNI_TRUE : JNI_FALSE);
}

// --- Read-along (audio) wrappers on the reading overlay ---------------------
static void btaRAHighlight(uintptr_t jni_env, int verse, int follow) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaRAHighlightM, verse, follow ? JNI_TRUE : JNI_FALSE);
}

static void btaRAClear(uintptr_t jni_env) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaRAClearM);
}

static void btaRAFollow(uintptr_t jni_env, int show) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaRAFollowM, show ? JNI_TRUE : JNI_FALSE);
}

static void btaRAColors(uintptr_t jni_env, int highlight, int followBg, int followFg) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaRAColorsM, highlight, followBg, followFg);
}
*/
import "C"

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"sync/atomic"
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

// btaRecreated is raised when runBta detects Android handed us a NEW activity
// (recreation: swipe-away-with-live-process relaunch, rotation, memory pressure).
// BtBridge.init rebuilt its Dialog with a BLANK TextView, but pushChapterHTML's
// fingerprint gate doesn't know that — so foregroundOverlayRecovery consumes this
// flag on re-entry and forces a full re-render. Atomic: set on RunNative's JNI
// goroutine, consumed on the recovery goroutine.
var btaRecreated atomic.Bool

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
			recreated := btaInitTried // a CHANGED Ctx = activity recreation, not first init
			btaInitTried = true
			btaCtx = ac.Ctx
			btaAvailable = C.btaInit(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
			if !btaAvailable {
				// Bridge dex missing (plain `fyne package` build) — the reading
				// pane fell back to the Fyne widget path; note it once.
				log.Printf("bibletext: BtBridge dex not present; using Fyne reading fallback")
			} else if recreated {
				btaRecreated.Store(true) // fresh (blank) Dialog — needs a re-render
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

// syncNativeAIMenu mirrors the Settings → Assistant choice ("None" = off) into
// BtBridge's selection-menu AI gate, so the "Study with AI" submenu appears or
// disappears with the setting. Runs through runBta, which (re-)inits the bridge
// and hands us an attached JNIEnv; a missing bridge dex makes it a silent no-op
// (the Fyne fallback pane has no AI menu to gate anyway).
func syncNativeAIMenu(state *AppState) {
	on := C.int(0)
	if aiFeaturesEnabled(state) {
		on = 1
	}
	runBta(func(env uintptr) {
		C.btaSetAIEnabled(C.uintptr_t(env), on)
	})
}

func newNativeReadingHost(state *AppState, verses []Verse) *nativeReadingHost {
	h := &nativeReadingHost{state: state}
	h.ExtendBaseWidget(h)
	currentHost = h
	syncNativeAIMenu(state) // the menu gate must match the setting before any selection
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

// foregroundOverlayRecovery runs on every return to the foreground (the
// OnEnteredForeground lifecycle hook, app.go). Android may have RECREATED the
// activity while we were away — swipe-away with the process kept alive by the
// audio service, rotation, memory pressure — in which case BtBridge rebuilt its
// Dialog with a blank TextView while the Go side's fingerprint gate still says
// the chapter is pushed. Probe (a no-op runBta forces the Ctx-change detection),
// then, if a recreation was flagged, drop the fingerprint and rebuild the
// reading view so the chapter re-renders, the frame/visibility re-pin, and a
// live read-along re-asserts (afterRebuild does all three). Without this, the
// reading pane comes back BLANK after a swipe-away relaunch — background audio
// made that path common (the foreground service keeps the process alive).
func foregroundOverlayRecovery(state *AppState) {
	if state == nil {
		return
	}
	go func() {
		runBta(func(env uintptr) {}) // ensure the new-activity detection has run
		if !btaRecreated.CompareAndSwap(true, false) {
			return // same activity as before — nothing was lost
		}
		fyne.Do(func() {
			if state.Bible == nil {
				return // still on the loading screen — rebuildWindow will render fresh anyway
			}
			// Preserve the reader's place: BtBridge.lastFrac is a static that
			// SURVIVES the Dialog rebuild, so it still holds the pre-recreation
			// scroll. Capture it as a one-shot restore anchor for the re-render
			// (else the fingerprint reset below would re-open pinned to the top).
			if v, d, f, ok := captureReadingAnchor(); ok {
				state.restore = &restoreAnchor{
					Book:    state.CurrentBook,
					Chapter: state.CurrentChapter,
					Verse:   v,
					Delta:   d,
					Frac:    f,
				}
			}
			// The Java TextView is empty; force pushChapterHTML past its gate.
			lastPushedChapterFP = ""
			lastPushedBookChapter = ""
			// Full rebuild — the same (live-verified) path a tab switch takes: it
			// drains any stranded modal, clears the suppress latch, handles the
			// full-screen reading mode, and afterRebuild re-pins the frame,
			// re-asserts visibility, and re-issues a live read-along highlight +
			// follow pill (state.refresh() would do none of that).
			rebuildWindow(state)
		})
	}()
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
			// The rebuilt overlay reset its read-along state; re-issue the live
			// highlight + follow pill so narration in progress isn't left un-tinted
			// with no way back to follow (Android activity recreation / rotation).
			gAudio.reassertReadAlong()
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

	// Restyle the read-along highlight + "Follow narration" pill for this palette,
	// so a light/dark flip mid-narration recolors them (mirrors the iOS render).
	pushFollowButtonColors(pal)
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
	runBta(func(env uintptr) {
		cs := C.CString(path)
		C.btaShareImage(C.uintptr_t(env), cs)
		C.free(unsafe.Pointer(cs))
	})
}

// --- Read-along (audio) — highlight the narrated verse + the "Follow narration"
//     pill, both on the reading overlay (audio_controller.go drives these). The
//     Java side hops to the main thread, so they're safe from the BtAudio
//     position-poll/TTS-range callbacks as well as the Fyne goroutine. -----------

// readAlongHighlight tints the verse being narrated (clearing the previous one) and,
// when follow is set, gently scrolls it into a comfortable band. verse<=0 just clears.
func readAlongHighlight(verse int, follow bool) {
	runBta(func(env uintptr) {
		f := C.int(0)
		if follow {
			f = 1
		}
		C.btaRAHighlight(C.uintptr_t(env), C.int(verse), f)
	})
}

// readAlongClear removes the read-along tint (stop/nav).
func readAlongClear() {
	runBta(func(env uintptr) { C.btaRAClear(C.uintptr_t(env)) })
}

// readAlongFollowButton shows/hides the floating "Follow narration" pill over the
// reading pane (audio_controller drives it around follow suspension).
func readAlongFollowButton(show bool) {
	runBta(func(env uintptr) {
		s := C.int(0)
		if show {
			s = 1
		}
		C.btaRAFollow(C.uintptr_t(env), s)
	})
}

// pushFollowButtonColors styles the read-along highlight + the pill from the app
// palette (fixed amber wash like iOS; accent ground, accent-text label). Called on
// every real chapter render so theme flips restyle them.
func pushFollowButtonColors(p palette) {
	// Fixed amber wash at ~0.32 alpha, matching the iOS read-along tint.
	const highlight = int32(0x52<<24 | 0xFF<<16 | 0xCC<<8 | 0x4D)
	followBg := argbAlpha(p.Accent, 0xC7) // ~0.78 alpha, semi-transparent like iOS
	followFg := argbAlpha(p.AccentText, 0xFF)
	runBta(func(env uintptr) {
		C.btaRAColors(C.uintptr_t(env), C.int(highlight), C.int(followBg), C.int(followFg))
	})
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

// argbAlpha packs an NRGBA color with an explicit alpha byte — used for the
// semi-transparent "Follow narration" pill (ignore the palette color's own alpha).
func argbAlpha(c color.NRGBA, alpha uint8) int32 {
	return int32(alpha)<<24 | int32(c.R)<<16 | int32(c.G)<<8 | int32(c.B)
}
