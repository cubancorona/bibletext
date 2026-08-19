// JNI thunks for BtBridge's native methods (the Android reading overlay).
// The `_android.c` suffix scopes this file to GOOS=android builds. ART
// resolves BtBridge's `native` methods against these exported symbols by
// name (System.loadLibrary was already run by GoNativeActivity), so no
// RegisterNatives is needed. Each thunk copies its JNI arguments to C
// buffers and hands off to the Go //export functions declared below
// (defined in reading_android_export.go) — the same split the Fyne driver
// itself uses (android.c ↔ //export files).

#include <jni.h>
#include <stdlib.h>
#include <string.h>

// Implemented in Go (reading_android_export.go). The char* arguments are
// only valid for the duration of the call — Go copies them immediately.
// lo/hi is the selection's verse span, resolved in Java (0,0 = unresolved).
extern void btaSelectionAction(char *action, char *text, int lo, int hi);
extern void btaScrolled(float frac);
extern void btaReadAlongUserScrolled(void);
extern void btaReadAlongFollowTapped(void);
extern void btaKeyboardChanged(float overlapDp);
extern void btaNoteNextTapped(void);
extern void btaNoteHidden(void);
extern void btaNoteDeleted(void);
extern void btaNoteRestored(void);

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeSelectionAction(JNIEnv *env, jclass clazz, jstring jAction, jstring jText, jint jLo, jint jHi) {
	const char *action = (*env)->GetStringUTFChars(env, jAction, NULL);
	const char *text = (*env)->GetStringUTFChars(env, jText, NULL);
	btaSelectionAction((char *)action, (char *)text, (int)jLo, (int)jHi);
	(*env)->ReleaseStringUTFChars(env, jAction, action);
	(*env)->ReleaseStringUTFChars(env, jText, text);
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeScrolled(JNIEnv *env, jclass clazz, jfloat frac) {
	btaScrolled((float)frac);
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeReadAlongUserScrolled(JNIEnv *env, jclass clazz) {
	btaReadAlongUserScrolled();
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeReadAlongFollowTapped(JNIEnv *env, jclass clazz) {
	btaReadAlongFollowTapped();
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeKeyboardChanged(JNIEnv *env, jclass clazz, jfloat overlapDp) {
	btaKeyboardChanged((float)overlapDp);
}

// The full-screen note sticker's verbs (the implementation requirement): next-tap on the count
// region, Hide, Delete, and the pill's tap-to-restore — each dispatching to
// the same Go verb the iOS sticker calls (reading_android_export.go).
JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeNoteNextTapped(JNIEnv *env, jclass clazz) {
	btaNoteNextTapped();
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeNoteHidden(JNIEnv *env, jclass clazz) {
	btaNoteHidden();
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeNoteDeleted(JNIEnv *env, jclass clazz) {
	btaNoteDeleted();
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeNoteRestored(JNIEnv *env, jclass clazz) {
	btaNoteRestored();
}
