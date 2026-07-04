// JNI thunks for BtAudio's native methods (the Android audio engine). The
// `_android.c` suffix scopes this file to GOOS=android builds. ART resolves
// BtAudio's `native` methods against these exported symbols by name
// (System.loadLibrary was already run by GoNativeActivity), so no RegisterNatives
// is needed — the same split the reading overlay uses (reading_jni_android.c).
// Each thunk hands off to the Go //export functions in audio_export_android.go.

#include <jni.h>

// Implemented in Go (audio_export_android.go).
extern void btAudioState(int code);
extern void btAudioTime(double seconds);
extern void btAudioRange(int location);

JNIEXPORT void JNICALL
Java_org_bibletext_BtAudio_nativeAudioState(JNIEnv *env, jclass clazz, jint code) {
	btAudioState((int)code);
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtAudio_nativeAudioTime(JNIEnv *env, jclass clazz, jdouble seconds) {
	btAudioTime((double)seconds);
}

JNIEXPORT void JNICALL
Java_org_bibletext_BtAudio_nativeAudioRange(JNIEnv *env, jclass clazz, jint location) {
	btAudioRange((int)location);
}
