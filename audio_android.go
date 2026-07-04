//go:build android

package bibletext

// Native-Android per-chapter audio: the twin of audio_ios.go. The engine (a
// MediaPlayer streaming the recorded MP3, an on-device TextToSpeech reading the
// verses aloud, AudioManager audio focus, a 200ms position poll for read-along)
// lives in android/BtAudio.java, shipped as classes2.dex by
// scripts/build-android.sh. This file drives it over JNI from fyne's RunNative (a
// JNI-attached background thread; BtAudio hops every engine call onto the Android
// main thread itself, like BtBridge).
//
// The interface it implements is the exact one audio_controller.go calls —
// nativeAudioStartURL / StartTTS / Toggle / Stop / Skip / SetArtwork — so the
// cross-platform controller drives Android identically to Apple. Callbacks
// Java→Go (playback state, position, TTS speech range) are the BtAudio `native`
// methods, their JNI thunks in audio_jni_android.c, landing in the //export
// functions in audio_export_android.go.

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static jclass    btAudioClass = NULL;   // global ref to org.bibletext.BtAudio
static jmethodID btAudioInitM, btAudioStartURLM, btAudioStartTTSM, btAudioToggleM,
                 btAudioStopM, btAudioSkipM, btAudioSetArtworkM;

// Resolve BtAudio through the ACTIVITY's classloader — the same gomobile trap the
// reading overlay hits: FindClass on a JNI-attached background thread uses the
// system classloader and can't see app dex classes, so go
// ctx.getClassLoader().loadClass("org.bibletext.BtAudio").
static int btAudioEnsureClass(JNIEnv *env, jobject ctx) {
	if (btAudioClass != NULL) {
		return 1;
	}
	jclass ctxCls = (*env)->GetObjectClass(env, ctx);
	jmethodID getCl = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject cl = (*env)->CallObjectMethod(env, ctx, getCl);
	jclass clCls = (*env)->GetObjectClass(env, cl);
	jmethodID loadClass = (*env)->GetMethodID(env, clCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring name = (*env)->NewStringUTF(env, "org.bibletext.BtAudio");
	jobject cls = (*env)->CallObjectMethod(env, cl, loadClass, name);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		return 0; // classes2.dex missing (plain `fyne package` build) — audio stays off
	}
	btAudioClass = (jclass)(*env)->NewGlobalRef(env, cls);

	btAudioInitM       = (*env)->GetStaticMethodID(env, btAudioClass, "init", "(Landroid/app/Activity;)V");
	btAudioStartURLM   = (*env)->GetStaticMethodID(env, btAudioClass, "startURL", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)V");
	btAudioStartTTSM   = (*env)->GetStaticMethodID(env, btAudioClass, "startTTS", "(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)V");
	btAudioToggleM     = (*env)->GetStaticMethodID(env, btAudioClass, "toggle", "()V");
	btAudioStopM       = (*env)->GetStaticMethodID(env, btAudioClass, "stop", "()V");
	btAudioSkipM       = (*env)->GetStaticMethodID(env, btAudioClass, "skip", "(D)V");
	btAudioSetArtworkM = (*env)->GetStaticMethodID(env, btAudioClass, "setArtwork", "(Ljava/lang/String;)V");
	// A NULL method ID (a dex/JNI signature skew from editing BtAudio.java without
	// updating these descriptors) leaves a pending NoSuchMethodError; every wrapper
	// guards only on btAudioClass==NULL, so an unchecked NULL would SIGSEGV in
	// CallStaticVoidMethod. Treat any skew as "engine absent" — clear, drop the
	// class, no audio (mirrors btaEnsureClass in reading_android.go).
	if ((*env)->ExceptionCheck(env) ||
	    btAudioInitM == NULL || btAudioStartURLM == NULL || btAudioStartTTSM == NULL ||
	    btAudioToggleM == NULL || btAudioStopM == NULL || btAudioSkipM == NULL ||
	    btAudioSetArtworkM == NULL) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteGlobalRef(env, btAudioClass);
		btAudioClass = NULL;
		return 0;
	}
	return 1;
}

static int btAudioInit(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (!btAudioEnsureClass(env, (jobject)ctx)) {
		return 0;
	}
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioInitM, (jobject)ctx);
	return 1;
}

static void btAudioStartURL(uintptr_t jni_env, const char *url, const char *title, const char *artist) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	jstring u = (*env)->NewStringUTF(env, url);
	jstring t = (*env)->NewStringUTF(env, title);
	jstring a = (*env)->NewStringUTF(env, artist);
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioStartURLM, u, t, a);
	(*env)->DeleteLocalRef(env, u);
	(*env)->DeleteLocalRef(env, t);
	(*env)->DeleteLocalRef(env, a);
}

static void btAudioStartTTS(uintptr_t jni_env, const char *text, const char *title, const char *artist) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	jstring x = (*env)->NewStringUTF(env, text);
	jstring t = (*env)->NewStringUTF(env, title);
	jstring a = (*env)->NewStringUTF(env, artist);
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioStartTTSM, x, t, a);
	(*env)->DeleteLocalRef(env, x);
	(*env)->DeleteLocalRef(env, t);
	(*env)->DeleteLocalRef(env, a);
}

static void btAudioToggle(uintptr_t jni_env) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioToggleM);
}

static void btAudioStop(uintptr_t jni_env) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioStopM);
}

static void btAudioSkip(uintptr_t jni_env, double seconds) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioSkipM, seconds);
}

static void btAudioSetArtwork(uintptr_t jni_env, const char *path) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (btAudioClass == NULL) return;
	jstring p = (*env)->NewStringUTF(env, path);
	(*env)->CallStaticVoidMethod(env, btAudioClass, btAudioSetArtworkM, p);
	(*env)->DeleteLocalRef(env, p);
}
*/
import "C"

import (
	"log"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

// btAudioAvailable reports whether the BtAudio dex class resolved. As with the
// reading overlay, it's false in a plain `fyne package` build that skipped
// scripts/build-android.sh; the wrappers then no-op and the play button is inert.
var btAudioAvailable = false
var btAudioInitTried = false
var btAudioCtx uintptr // the activity jobject the engine was last (re-)initialised for

// runBtAudio wraps driver.RunNative for the audio bridge — the audio twin of
// runBta. It (re-)initialises BtAudio when first seen or when Android hands a new
// activity (rotation, background→foreground), so the engine's Context/AudioManager
// track the live activity; the C class + method IDs are cached across re-inits.
func runBtAudio(fn func(env uintptr)) {
	err := driver.RunNative(func(c any) error {
		ac, ok := c.(*driver.AndroidContext)
		if !ok {
			return nil
		}
		if !btAudioInitTried || ac.Ctx != btAudioCtx {
			btAudioInitTried = true
			btAudioCtx = ac.Ctx
			btAudioAvailable = C.btAudioInit(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
			if !btAudioAvailable {
				log.Printf("bibletext: BtAudio dex not present; audio disabled")
			}
		}
		if btAudioAvailable {
			fn(ac.Env)
		}
		return nil
	})
	if err != nil {
		log.Printf("bibletext: BtAudio call failed: %v", err)
	}
}

// nativeAudioStartURL streams a recorded chapter MP3 (seekable — the ±15s skip),
// crediting title/artist on the (future) lock-screen card.
func nativeAudioStartURL(url, title, artist string) {
	cu, ct, ca := C.CString(url), C.CString(title), C.CString(artist)
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(ca))
	runBtAudio(func(env uintptr) {
		C.btAudioStartURL(C.uintptr_t(env), cu, ct, ca)
	})
}

// nativeAudioStartTTS reads the chapter's verses aloud on-device.
func nativeAudioStartTTS(text, title, artist string) {
	cx, ct, ca := C.CString(text), C.CString(title), C.CString(artist)
	defer C.free(unsafe.Pointer(cx))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(ca))
	runBtAudio(func(env uintptr) {
		C.btAudioStartTTS(C.uintptr_t(env), cx, ct, ca)
	})
}

func nativeAudioToggle() {
	runBtAudio(func(env uintptr) { C.btAudioToggle(C.uintptr_t(env)) })
}

func nativeAudioStop() {
	runBtAudio(func(env uintptr) { C.btAudioStop(C.uintptr_t(env)) })
}

func nativeAudioSkip(seconds float64) {
	runBtAudio(func(env uintptr) { C.btAudioSkip(C.uintptr_t(env), C.double(seconds)) })
}

func nativeAudioSetArtwork(path string) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	runBtAudio(func(env uintptr) { C.btAudioSetArtwork(C.uintptr_t(env), cp) })
}
