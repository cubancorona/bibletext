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
                 btaShareTextM, btaShareImageM, btaSetAIEnabledM, btaSetNotesEnabledM,
                 btaOpenBrowserM,
                 btaRAHighlightM, btaRAClearM, btaRAFollowM, btaRAColorsM,
                 btaScrollVerseM, btaSetNoteM;

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
	// Selection-menu notes gate — mirrors Settings → Shared notes; when off,
	// onCreateActionMode omits "Share with note".
	btaSetNotesEnabledM = (*env)->GetStaticMethodID(env, btaClass, "setNotesEnabled", "(Z)V");
	// Handing a shared link back out to the browser (notes off + the link has one).
	btaOpenBrowserM = (*env)->GetStaticMethodID(env, btaClass, "openInBrowser", "(Ljava/lang/String;)V");
	// Read-along (audio): highlight the narrated verse + the floating "Follow
	// narration" pill, both painted on this same overlay (reading_android.go owns
	// the BtBridge handle, so the audio read-along calls route through here).
	btaRAHighlightM = (*env)->GetStaticMethodID(env, btaClass, "readAlongHighlight", "(IZ)V");
	btaRAClearM     = (*env)->GetStaticMethodID(env, btaClass, "readAlongClear", "()V");
	btaRAFollowM    = (*env)->GetStaticMethodID(env, btaClass, "readAlongFollow", "(Z)V");
	btaRAColorsM    = (*env)->GetStaticMethodID(env, btaClass, "setReadAlongColors", "(III)V");
	// Arrival scroll: pin the shared link's highlighted verse near the top.
	btaScrollVerseM = (*env)->GetStaticMethodID(env, btaClass, "scrollToVerse", "(I)V");
	// The shared-note sticker (full-screen reading, the implementation requirement): text, WHO line,
	// pill/next presentation, anchor verse, then the five palette colors
	// (surface, text, muted, accent, border) as ARGB ints.
	btaSetNoteM = (*env)->GetStaticMethodID(env, btaClass, "setNote",
	                                        "([B[BZZIIIIII)V");
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
	    btaSetAIEnabledM == NULL || btaSetNotesEnabledM == NULL || btaOpenBrowserM == NULL ||
	    btaRAHighlightM == NULL || btaRAClearM == NULL || btaRAFollowM == NULL ||
	    btaRAColorsM == NULL || btaScrollVerseM == NULL || btaSetNoteM == NULL) {
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

static void btaSetNotesEnabled(uintptr_t jni_env, int on) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetNotesEnabledM, on ? JNI_TRUE : JNI_FALSE);
}

static void btaOpenBrowser(uintptr_t jni_env, char *url) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jstring s = (*env)->NewStringUTF(env, url);
	(*env)->CallStaticVoidMethod(env, btaClass, btaOpenBrowserM, s);
	(*env)->DeleteLocalRef(env, s);
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

static void btaScrollVerse(uintptr_t jni_env, int verse) {
	JNIEnv *env = (JNIEnv *)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaScrollVerseM, verse);
}

static void btaRAColors(uintptr_t jni_env, int highlight, int followBg, int followFg) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btaClass, btaRAColorsM, highlight, followBg, followFg);
}

// The shared-note sticker tuple (full-screen reading). Strings are copied by
// NewStringUTF before the call returns; the Java side hops to the UI thread
// and compares the tuple itself before touching any view.
// btaBytes: UTF-8 bytes as a jbyteArray, NULL for empty. The note body is the
// first USER-AUTHORED free text to cross this bridge, and emoji are 4-byte
// UTF-8 — invalid *modified* UTF-8, which NewStringUTF may abort on under
// CheckJNI (emulators commonly enable it). Bytes cross verbatim; Java decodes
// with the real UTF-8 charset (verification finding).
static jbyteArray btaBytes(JNIEnv *env, const char *s) {
	if (s == NULL || s[0] == '\0') return NULL;
	jsize n = (jsize)strlen(s);
	jbyteArray a = (*env)->NewByteArray(env, n);
	if (a == NULL) return NULL;
	(*env)->SetByteArrayRegion(env, a, 0, n, (const jbyte*)s);
	return a;
}

static void btaSetNote(uintptr_t jni_env, const char *text, const char *who,
                       int pill, int next, int anchorVerse,
                       int bg, int fg, int muted, int accent, int border) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btaClass == NULL) return;
	jbyteArray t = btaBytes(env, text);
	jbyteArray w = btaBytes(env, who);
	(*env)->CallStaticVoidMethod(env, btaClass, btaSetNoteM, t, w,
	                             pill ? JNI_TRUE : JNI_FALSE, next ? JNI_TRUE : JNI_FALSE,
	                             anchorVerse, bg, fg, muted, accent, border);
	if (t != NULL) (*env)->DeleteLocalRef(env, t);
	if (w != NULL) (*env)->DeleteLocalRef(env, w);
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
		notesOn := C.int(0)
		if notesFeatureOn(state) {
			notesOn = 1
		}
		C.btaSetNotesEnabled(C.uintptr_t(env), notesOn)
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

// pushNoteToOverlay hands the native sticker its tuple — the Android twin of
// iOS pushNoteToPane (reading_ios.go). The composition is androidStickerPush
// (notes_plan.go), a byte-identical alias of the Apple push, so the WHO line,
// counts, pill labels and derived suppression cannot diverge across panes.
//
// GATED TO FULL-SCREEN READING, deliberately: in normal (compact) reading the
// Fyne banner above the pane already draws the whole set (notes_banner.go,
// slotted in buildReadingViewMobile below), and a Dialog sticker there would
// show the reader the same note twice. Full-screen has no banner — before
// this push it showed a lit span with nothing to say why (the implementation requirement). Leaving
// full-screen pushes the empty tuple, which takes the sticker down.
//
// Called on EVERY chapter push, BEFORE the fingerprint skip gate — the same
// ordering as iOS: a presentation-only flip (hide/show, suppression, a
// next-tap between notes on the same verse) may change nothing the combined
// render fingerprint folds, so the Java side compares the pushed tuple itself
// and refreshes the sticker alone (BtBridge.setNote's `changed` gate, the
// bibleTextSetNote pattern).
func pushNoteToOverlay(state *AppState) {
	text, who, pill, next := "", "", false, false
	anchor := 0
	if state.IsFullScreen {
		text, who, pill, next = androidStickerPush(state, buildChapterPlan(state, appPrefs(), state.Bible))
		// The note's OWN verse, not the highlight's — minimizing clears the
		// highlight, and a marker without an anchor jumps to the top of the
		// chapter (the iOS lesson). Verse 0 (unplaced-only) parks at the top.
		anchor = state.NoteVerseLo
	}
	pal := state.pal()
	p, n := C.int(0), C.int(0)
	if pill {
		p = 1
	}
	if next {
		n = 1
	}
	ct := C.CString(text)
	cw := C.CString(who)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cw))
	runBta(func(env uintptr) {
		C.btaSetNote(C.uintptr_t(env), ct, cw, p, n, C.int(anchor),
			C.int(argbInt(pal.SurfaceAlt)), C.int(argbInt(pal.Text)),
			C.int(argbInt(pal.TextMuted)), C.int(argbInt(pal.Accent)),
			C.int(argbInt(pal.Border)))
	})
}

// pushChapterHTML mirrors the iOS twin: fingerprint-gated re-import, restore
// arming, and a same-chapter re-render (theme flip) preserving the live scroll.
func pushChapterHTML(state *AppState, verses []Verse) {
	// The sticker tuple rides every push, ahead of the skip gate (see
	// pushNoteToOverlay) — so presentation flips render even when the Spanned
	// is byte-identical and the gate below returns early.
	pushNoteToOverlay(state)

	fp := chapterRenderFingerprint(state)
	bc := fmt.Sprintf("%s|%d", state.CurrentBook, state.CurrentChapter)

	// A declared arrival (forceReposition) outranks "stay where you were": the
	// capture below exists for re-renders the reader did not ask to move on (a
	// theme flip, a verb's presentation change). The sticker's next-tap is now

	// btaNoteNextTapped declares no arrival, so its moved mark is captured
	// here and the re-render lands back where the reader was, sticker and
	// tint swapped in place. The guard itself stays for the arrivals that do
	// declare (a tapped link, a search result, a Go-to).
	if state.restore == nil && !state.forceReposition && bc == lastPushedBookChapter && fp != lastPushedChapterFP {
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

	// forceReposition defeats the skip: an explicit arrival must place the view
	// even when the render is byte-identical (see AppState.forceReposition).
	if state.restore == nil && !state.forceReposition && fp == lastPushedChapterFP {
		return
	}
	state.forceReposition = false
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
			// The pitch as a multiple of the TEXT SIZE — the Java side turns it
			// into an exact line height, so this is the same quantity CSS
			// line-height names.
			//
			// It is NOT the 2.0 the Apple pane writes into its CSS. That value
			// is nominal: the UIKit HTML importer does not honour a unitless
			// line-height, so iOS actually draws close to the font's own
			// leading. Matching the CSS number made Android markedly looser than

			// reason this is a measured constant rather than a shared one.
			//
			// It used to be 1.7 applied to the font's NATURAL line height, a
			// larger quantity again, which is where the original looseness came
			// from.
			C.float(textPx), C.float(1.35),
			C.int(padL), C.int(padT), C.int(padL), C.int(padT))
		ch := C.CString(html)
		C.btaSetHtml(C.uintptr_t(env), ch, C.float(frac))
		C.free(unsafe.Pointer(ch))
		// THE ARRIVAL SCROLL, which Android never had: iOS and macOS position
		// on the highlighted verse inside their scroll cadence, but Android's
		// only programmatic scrolls were the restore frac and read-along — a
		// shared link opened the right chapter at the WRONG place (emulator-
		// caught: John 11 v35 landed at v57). Restore outranks the highlight,
		// same ordering the other panes settled on: a pending restore means
		// the reader is coming back, not arriving.
		if sp, here := state.markHere(); state.restore == nil && here {
			C.btaScrollVerse(C.uintptr_t(env), C.int(sp.Lo))
		}
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

// openLinkInBrowser hands a shared link back out to the default browser. Used
// when the link carries a note and notes are switched off: the app cannot stop
// the OS handing it the link — the manifest's App Links filter settles that at
// build time — but it can decline it and pass it on, where the note still reads.
//
// The Java side names the browser explicitly, because a bare ACTION_VIEW would
// resolve straight back to this app.
func openLinkInBrowser(rawURL string) {
	if rawURL == "" {
		return
	}
	runBta(func(env uintptr) {
		cs := C.CString(rawURL)
		C.btaOpenBrowser(C.uintptr_t(env), cs)
		C.free(unsafe.Pointer(cs))
	})
}

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
		// The sheet the reader was inside has left the canvas: run the window
		// rebuild a background data swap deferred to spare it (no-op otherwise,
		// and non-recursive — rebuildWindow downs the flag before re-running
		// this closure).
		consumeDeferredFullRebuild(state)
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
	// The chapter's shared note (notes_banner.go). Android renders it as this
	// banner ABOVE the pane — the native overlay covers only the paper below,
	// so no BtBridge machinery is needed — where iOS draws its in-text sticker
	// instead (its own buildReadingViewMobile, untouched). platform reproduction:
	// the banner had been slotted only into the desktop buildReadingView, which
	// Android's compact layout never calls.
	if banner := buildNoteBanner(state); banner != nil {
		top.Add(banner)
	}

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
