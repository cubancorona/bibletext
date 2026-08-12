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
extern void btaSelectionAction(char *action, char *text);
extern void btaScrolled(float frac);
extern void btaReadAlongUserScrolled(void);
extern void btaReadAlongFollowTapped(void);
extern void btaKeyboardChanged(float overlapDp);

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeSelectionAction(JNIEnv *env, jclass clazz, jstring jAction, jstring jText) {
	const char *action = (*env)->GetStringUTFChars(env, jAction, NULL);
	const char *text = (*env)->GetStringUTFChars(env, jText, NULL);
	btaSelectionAction((char *)action, (char *)text);
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
