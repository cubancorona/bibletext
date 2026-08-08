// JNI thunk for BtBridge.nativeOpenedLink — a tapped shared link (App Links).
// The `_android.c` suffix scopes this file to GOOS=android builds. ART resolves
// the Java `native` method against this exported symbol by name, the same
// split the reading overlay and audio engine use (reading_jni_android.c,
// audio_jni_android.c), so no RegisterNatives is needed.

#include <jni.h>

// Implemented in Go (share_link_export_android.go).
extern void btOpenedLink(char *url);

JNIEXPORT void JNICALL
Java_org_bibletext_BtBridge_nativeOpenedLink(JNIEnv *env, jclass clazz, jstring url) {
	if (url == NULL) {
		return;
	}
	const char *s = (*env)->GetStringUTFChars(env, url, NULL);
	if (s == NULL) {
		return;
	}
	// btOpenedLink copies the string before returning, so releasing here is safe.
	btOpenedLink((char *)s);
	(*env)->ReleaseStringUTFChars(env, url, s);
}
