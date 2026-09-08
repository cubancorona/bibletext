//go:build android

// The Android half of the credential store: the cipher call, and nothing else.
//
// Keys used to sit in plaintext in fyne.Preferences here, which is inside
// files/ — the directory Android Auto Backup uploads to the reader's Google
// Drive. The ciphertext now lives under no_backup/ beside the Bible caches
// (cache_path_android.go moved those for the same reason), wrapped with an
// AES-256-GCM key that never leaves this phone's secure hardware.
//
// Everything that could lose a reader's key — the envelope, the atomic write,
// the (found, ok) classification — is in ai_secure_store_blob.go, untagged, so
// `go test` executes it on every platform. This file is only the bridge,
// because nothing here can be tested without a device.

package bibletext

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static jclass    btKeysClass = NULL;   // global ref to org.bibletext.BtKeys
static jmethodID btKeysAvailM, btKeysWrapM, btKeysUnwrapM;

// Resolve BtKeys through the ACTIVITY's classloader. FindClass on a
// JNI-attached background thread uses the system classloader and cannot see app
// dex classes — the documented gomobile trap, and the same reason BtBridge is
// resolved this way in reading_android.go.
static int btKeysEnsureClass(JNIEnv *env, jobject ctx) {
	if (btKeysClass != NULL) {
		return 1;
	}
	if (ctx == NULL) {
		return 0; // GetObjectClass(env, NULL) is a CheckJNI abort, not an error
	}
	jclass    ctxCls    = (*env)->GetObjectClass(env, ctx);
	jmethodID getCl     = (*env)->GetMethodID(env, ctxCls, "getClassLoader", "()Ljava/lang/ClassLoader;");
	jobject   cl        = (*env)->CallObjectMethod(env, ctx, getCl);
	jclass    clCls     = (*env)->GetObjectClass(env, cl);
	jmethodID loadClass = (*env)->GetMethodID(env, clCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	jstring   name      = (*env)->NewStringUTF(env, "org.bibletext.BtKeys");
	jobject   cls       = (*env)->CallObjectMethod(env, cl, loadClass, name);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		return 0; // classes2.dex missing (a plain `fyne package` build)
	}
	if (cls == NULL) {
		return 0; // NewGlobalRef(NULL) then GetStaticMethodID(NULL, ..) aborts
	}
	btKeysClass   = (jclass)(*env)->NewGlobalRef(env, cls);
	btKeysAvailM  = (*env)->GetStaticMethodID(env, btKeysClass, "available", "()Z");
	btKeysWrapM   = (*env)->GetStaticMethodID(env, btKeysClass, "wrap", "([B[B)[B");
	btKeysUnwrapM = (*env)->GetStaticMethodID(env, btKeysClass, "unwrap", "([B[B)[B");
	if ((*env)->ExceptionCheck(env) ||
	    btKeysAvailM == NULL || btKeysWrapM == NULL || btKeysUnwrapM == NULL) {
		// A NULL method id leaves a pending NoSuchMethodError, and a pending
		// exception poisons every later JNI call on this thread. Drop the class
		// so the store reports itself absent rather than crashing later.
		(*env)->ExceptionClear(env);
		(*env)->DeleteGlobalRef(env, btKeysClass);
		btKeysClass = NULL;
		return 0;
	}
	return 1;
}

// 1 = usable, 0 = structurally absent (no dex, API < 23, no provider).
static int btKeysProbe(uintptr_t jni_env, uintptr_t ctx) {
	JNIEnv *env = (JNIEnv*)jni_env;
	if (!btKeysEnsureClass(env, (jobject)ctx)) {
		return 0;
	}
	jboolean ok = (*env)->CallStaticBooleanMethod(env, btKeysClass, btKeysAvailM);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		return 0;
	}
	return ok == JNI_TRUE ? 1 : 0;
}

// btKeysCall drives wrap or unwrap. Returns 1 with the result in *out
// (malloc'ed, caller frees), 0 when Java returned a zero-length array — which
// only unwrap does, meaning "permanently undecryptable" — and -1 on any
// failure, including a call that could not be made.
//
// PushLocalFrame/PopLocalFrame keeps all six exit paths clean. It matters more
// here than in the one-shot bridge setup: the migration reads one key per
// provider on every launch, and on a thread the JVM already knows, local refs
// persist across calls until the frame pops.
static int btKeysCall(uintptr_t jni_env, uintptr_t ctx, int unwrapping,
                      const void *aad, int aadLen,
                      const void *in, int inLen,
                      void **out, int *outLen) {
	JNIEnv *env = (JNIEnv*)jni_env;
	*out = NULL;
	*outLen = 0;
	if (!btKeysEnsureClass(env, (jobject)ctx)) {
		return -1;
	}
	if ((*env)->PushLocalFrame(env, 8) != 0) {
		(*env)->ExceptionClear(env);
		return -1;
	}
	jbyteArray ja = (*env)->NewByteArray(env, aadLen);
	jbyteArray jb = (*env)->NewByteArray(env, inLen);
	if (ja == NULL || jb == NULL) {
		(*env)->ExceptionClear(env);
		(*env)->PopLocalFrame(env, NULL);
		return -1;
	}
	(*env)->SetByteArrayRegion(env, ja, 0, aadLen, (const jbyte*)aad);
	(*env)->SetByteArrayRegion(env, jb, 0, inLen, (const jbyte*)in);

	jobject res = (*env)->CallStaticObjectMethod(env, btKeysClass,
		unwrapping ? btKeysUnwrapM : btKeysWrapM, ja, jb);
	// Check for an exception BEFORE looking at res: the call returns NULL both
	// when Java returned null and when it threw, and the two are
	// indistinguishable from the return value alone. They map to the same safe
	// answer here deliberately. Leaving one pending is not an option — ART
	// asserts on a pending exception at its next entry point and aborts.
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		(*env)->PopLocalFrame(env, NULL);
		return -1;
	}
	if (res == NULL) {
		(*env)->PopLocalFrame(env, NULL);
		return -1;
	}
	jsize n = (*env)->GetArrayLength(env, (jbyteArray)res);
	if (n == 0) {
		(*env)->PopLocalFrame(env, NULL);
		return 0; // unwrap's "this can never be decrypted again"
	}
	void *buf = malloc((size_t)n);
	if (buf == NULL) {
		(*env)->PopLocalFrame(env, NULL);
		return -1;
	}
	(*env)->GetByteArrayRegion(env, (jbyteArray)res, 0, n, (jbyte*)buf);
	(*env)->PopLocalFrame(env, NULL); // frees ja, jb and res together
	*out = buf;
	*outLen = (int)n;
	return 1;
}
*/
import "C"

import (
	"log"
	"os"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

// runBtKeys hands the callback an attached JNIEnv and the activity jobject.
// Unlike runBta it caches no context: BtKeys needs the classloader on first use
// and has nothing to re-initialise when the activity is recreated.
//
// driver.RunNative is synchronous — RunOnJVM blocks until the callback
// completes — so a plain captured local is safe and no atomic is needed.
func runBtKeys(fn func(env, ctx uintptr)) {
	if err := driver.RunNative(func(c any) error {
		ac, ok := c.(*driver.AndroidContext)
		if !ok || ac.Ctx == 0 {
			return nil // fn does not run; the caller's sentinel stands
		}
		fn(ac.Env, ac.Ctx)
		return nil
	}); err != nil {
		log.Printf("bibletext: BtKeys call failed: %v", err)
	}
}

type jniKeyWrapper struct{}

func (jniKeyWrapper) Wrap(aad, plaintext []byte) []byte {
	if len(aad) == 0 || len(plaintext) == 0 {
		return nil
	}
	// Allocated outside the callback: the pointers must outlive it, and the
	// free must happen on the calling goroutine.
	cAAD := C.CBytes(aad)
	defer C.free(cAAD)
	cIn := C.CBytes(plaintext)
	defer C.free(cIn)

	status := C.int(-1)
	var out unsafe.Pointer
	var outLen C.int
	runBtKeys(func(env, ctx uintptr) {
		status = C.btKeysCall(C.uintptr_t(env), C.uintptr_t(ctx), 0,
			cAAD, C.int(len(aad)), cIn, C.int(len(plaintext)), &out, &outLen)
	})
	if status != 1 || out == nil {
		return nil
	}
	defer C.free(out)
	return C.GoBytes(out, outLen)
}

func (jniKeyWrapper) Unwrap(aad, payload []byte) ([]byte, int) {
	if len(aad) == 0 || len(payload) == 0 {
		return nil, unwrapFailed
	}
	cAAD := C.CBytes(aad)
	defer C.free(cAAD)
	cIn := C.CBytes(payload)
	defer C.free(cIn)

	// THE MOST IMPORTANT LINE IN THIS FILE. runBtKeys calls fn only when a real
	// AndroidContext arrived and swallows a RunNative error, so this sentinel
	// is the answer that ships whenever the bridge is missing. unwrapFailed
	// keeps the reader's legacy copy; unwrapDead here would invite the caller
	// to erase it.
	status := C.int(-1)
	var out unsafe.Pointer
	var outLen C.int
	runBtKeys(func(env, ctx uintptr) {
		status = C.btKeysCall(C.uintptr_t(env), C.uintptr_t(ctx), 1,
			cAAD, C.int(len(aad)), cIn, C.int(len(payload)), &out, &outLen)
	})
	switch status {
	case 1:
		if out == nil {
			return nil, unwrapFailed
		}
		defer C.free(out)
		return C.GoBytes(out, outLen), unwrapOK
	case 0:
		return nil, unwrapDead
	default:
		return nil, unwrapFailed
	}
}

func newPlatformSecretStore() secretStore {
	dir, ok := secretBlobDirFrom(fyneStorageRoot())
	if !ok {
		// An unrecognised storage layout. Keeping today's behaviour is the safe
		// answer: the app still works and keys stay where they are, rather than
		// inventing a new way to lose one on a should-never-happen path.
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	probe := C.int(-1)
	runBtKeys(func(env, ctx uintptr) {
		probe = C.btKeysProbe(C.uintptr_t(env), C.uintptr_t(ctx))
	})
	if probe == 0 {
		return nil // API < 23, classes2.dex absent, or no AndroidKeyStore
	}
	// probe == -1 means the driver could not give us an AndroidContext YET.
	// The keystore is constructed as soon as the app exists and its result is
	// cached for the session, so returning nil here would strand every key in
	// plaintext preferences for the whole run. Return the store instead: each
	// call re-resolves the class and reports ok=false until it succeeds, which
	// blocks nothing and erases nothing.
	return &blobSecretStore{name: "Android Keystore", dir: dir, w: jniKeyWrapper{}}
}
