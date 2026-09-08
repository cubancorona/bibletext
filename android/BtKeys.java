package org.bibletext;

import android.os.Build;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyPermanentlyInvalidatedException;
import android.security.keystore.KeyProperties;
import android.util.Log;
import java.security.Key;
import java.security.KeyStore;
import javax.crypto.AEADBadTagException;
import javax.crypto.Cipher;
import javax.crypto.KeyGenerator;
import javax.crypto.SecretKey;
import javax.crypto.spec.GCMParameterSpec;

/**
 * BtKeys wraps and unwraps the reader's AI-provider keys with an AES-256-GCM
 * key held in the Android Keystore, so the ciphertext the Go side stores under
 * no_backup/ is meaningless without this phone's secure hardware.
 *
 * IT DOES ONE THING. It does not read or write files, does not know an account
 * from a blob path, and never decides that anything may be erased. The Go side
 * (ai_secure_store_blob.go) owns the envelope, the storage, the atomic write
 * and the (found, ok) answer, because all of that is then testable on a host
 * and none of it is testable here: this project has no on-device Go test
 * runner, so every line that lives in Java is a line no test executes.
 *
 * NO Context, NO Activity, NO init(). KeyStore.getInstance("AndroidKeyStore")
 * needs nothing, so unlike BtBridge and BtAudio there is no activity-recreation
 * lifecycle here and nothing to re-initialise when Android hands the app a new
 * activity. The class only has to be LOADABLE through the activity classloader.
 *
 * THREADING. Every other public static in this package hops to the main thread
 * (BtBridge's UI.post). These do not: they run synchronously on the caller's
 * JNI-attached thread, because secretStore.Read is a synchronous function that
 * returns a value, and posting-then-waiting from a thread that may itself be
 * the main thread would deadlock. Keystore work should not run on the main
 * thread in any case — it is a binder round trip plus file I/O.
 *
 * API 23. KeyGenParameterSpec, KeyProperties and KeyPermanentlyInvalidated-
 * Exception are all API 23, and minSdk is 21. javac and d8 both accept those
 * references without complaint, so the gate is entirely ours, and it is
 * STRUCTURAL rather than an `if`: every API-23 type is referenced only from
 * BtKeysV23, which is only ever reached from inside an SDK_INT check, so ART
 * never has to resolve it on 21-22.
 *
 * Ships as classes2.dex, compiled by scripts/build-android.sh.
 */
public final class BtKeys {
    private static final String TAG = "BibleText";

    private BtKeys() {}

    /**
     * available reports STRUCTURAL presence: whether this device could ever do
     * Keystore AES at all. False makes the Go side return a nil secretStore,
     * which is the documented way to say "this platform has no credential
     * store" and leaves keys exactly where they are today.
     *
     * A TRANSIENT keystore fault must NOT be reported here. That belongs in
     * unwrap's null, so the store stays live and migration retries later.
     */
    public static boolean available() {
        if (Build.VERSION.SDK_INT < 23) return false;
        try {
            KeyStore.getInstance("AndroidKeyStore");
            return true;
        } catch (Throwable t) {
            Log.w(TAG, "BtKeys: no AndroidKeyStore provider on this device", t);
            return false;
        }
    }

    /**
     * wrap returns iv(12) || ciphertext || tag(16), or null on any failure.
     *
     * aad is the account id: it binds a blob to its slot, so a blob moved
     * between slots fails authentication instead of decrypting into another
     * provider's field. RENAMING AN ACCOUNT ID IS THEREFORE A KEY-DESTROYING
     * CHANGE; the ids are the provider ids and the API.Bible id, and they are
     * frozen.
     */
    public static byte[] wrap(byte[] aad, byte[] plaintext) {
        if (Build.VERSION.SDK_INT < 23) return null;
        if (aad == null || plaintext == null || plaintext.length == 0) return null;
        try {
            return BtKeysV23.wrap(aad, plaintext);
        } catch (Throwable t) {
            // The catch-all is TRANSIENT, always. A failure to create or use a
            // key says nothing about an existing blob, so it can never be
            // evidence that one is dead.
            Log.w(TAG, "BtKeys: wrap failed", t);
            return null;
        }
    }

    /**
     * unwrap is deliberately tri-state:
     *
     *   null       the store failed, or we could not tell — the caller keeps
     *              any legacy plaintext copy and retries on the next launch
     *   length 0   this blob can never be decrypted again on this device
     *   otherwise  the plaintext
     *
     * A stored value is never empty (empty means delete, and the Go side
     * removes the file), so a zero-length array is unambiguous. An exception
     * cannot manufacture one, which is what keeps the irreversible branch on
     * the Go side unreachable from any failure mode.
     */
    public static byte[] unwrap(byte[] aad, byte[] blob) {
        if (Build.VERSION.SDK_INT < 23) return null;
        if (aad == null || blob == null || blob.length < 28) return null;
        try {
            return BtKeysV23.unwrap(aad, blob);
        } catch (Throwable t) {
            Log.w(TAG, "BtKeys: unwrap failed", t);
            return null;
        }
    }
}

/**
 * Every android.security.keystore reference in the app lives here, so ART
 * resolves this class only after an SDK_INT >= 23 test has passed.
 */
final class BtKeysV23 {
    // ONE alias for all accounts, not one per account: "is the master key
    // present?" is exactly the single question the (found, ok) contract asks,
    // and per-account aliases would allow a partial state that contract has no
    // vocabulary for. The .v1 suffix is load-bearing — Keystore key parameters
    // are immutable, so a future parameter change must mint a NEW alias rather
    // than delete and regenerate this one, which would destroy every blob.
    private static final String ALIAS = "uk.co.bibletext.ai-provider-keys.v1";
    private static final String XFORM = "AES/GCM/NoPadding";
    private static final int IV_LEN = 12;
    private static final int TAG_BITS = 128;
    private static final byte[] DEAD = new byte[0];

    private BtKeysV23() {}

    static byte[] wrap(byte[] aad, byte[] plaintext) throws Exception {
        SecretKey k = key(true);
        if (k == null) return null;
        Cipher c = Cipher.getInstance(XFORM);
        // No IV parameter: setRandomizedEncryptionRequired means a
        // caller-supplied IV is REFUSED. The cipher generates one; we read it
        // back with getIV and carry it in the blob.
        c.init(Cipher.ENCRYPT_MODE, k);
        byte[] iv = c.getIV();
        if (iv == null || iv.length != IV_LEN) return null;
        c.updateAAD(aad);
        byte[] ct = c.doFinal(plaintext);
        byte[] out = new byte[IV_LEN + ct.length];
        System.arraycopy(iv, 0, out, 0, IV_LEN);
        System.arraycopy(ct, 0, out, IV_LEN, ct.length);
        return out;
    }

    static byte[] unwrap(byte[] aad, byte[] blob) throws Exception {
        // NEVER generate on the read path. A read that regenerated the alias
        // would make every existing blob undecryptable by our own hand.
        SecretKey k = key(false);
        if (k == null) return DEAD; // containsAlias cleanly said no

        Cipher c = Cipher.getInstance(XFORM);
        try {
            c.init(Cipher.DECRYPT_MODE, k, new GCMParameterSpec(TAG_BITS, blob, 0, IV_LEN));
        } catch (KeyPermanentlyInvalidatedException e) {
            // Must precede any InvalidKeyException catch; javac enforces the
            // ordering. The class name is the whole contract.
            return DEAD;
        }
        c.updateAAD(aad);
        try {
            return c.doFinal(blob, IV_LEN, blob.length - IV_LEN);
        } catch (AEADBadTagException e) {
            // GCM authentication failed: wrong key, wrong account, truncated or
            // tampered. There is no future in which this succeeds.
            return DEAD;
        }
        // Everything else — UnrecoverableKeyException, KeyStoreException,
        // IllegalBlockSizeException (AndroidKeyStore uses it to surface
        // keystore OPERATION failures rather than a block-size problem), a bare
        // BadPaddingException, ProviderException — propagates to the caller's
        // catch-all and is reported TRANSIENT. Each of those plausibly means
        // the key is gone, but none PROVES it, and the two mistakes are not
        // symmetric: transient merely blocks a migration and leaves the reader
        // a working plaintext key, while dead erases the reader's key.
    }

    /**
     * key(false) returns null only when the alias is cleanly absent.
     * key(true) creates it.
     *
     * synchronized because two keyStore instances run their migration
     * concurrently on a device — the shared one from the background load and
     * the app-state one from the UI thread, each on its own JNI thread. Two
     * threads racing generate-then-write could otherwise leave one thread's
     * blob encrypted under a key that no longer exists.
     */
    private static synchronized SecretKey key(boolean create) throws Exception {
        KeyStore ks = KeyStore.getInstance("AndroidKeyStore");
        ks.load(null);
        if (ks.containsAlias(ALIAS)) {
            Key k = ks.getKey(ALIAS, null);
            if (k instanceof SecretKey) return (SecretKey) k;
            // An alias that is not our secret key: every existing blob is
            // already unopenable, so replacing it on the WRITE path loses
            // nothing. On the read path it is simply dead.
        }
        if (!create) return null;

        KeyGenerator g = KeyGenerator.getInstance(
                KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore");
        g.init(new KeyGenParameterSpec.Builder(ALIAS,
                        KeyProperties.PURPOSE_ENCRYPT | KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                // Said out loud rather than inherited: with this on, a
                // caller-provided IV is refused, which is what makes IV reuse
                // structurally impossible.
                .setRandomizedEncryptionRequired(true)
                // Explicitly false. The app reads keys with no prompt, at
                // startup and while backgrounded. Auth-bound keys would also
                // make KeyPermanentlyInvalidatedException an ORDINARY event —
                // any lock-screen or biometric change — and that is the
                // exception which classifies a blob as dead.
                .setUserAuthenticationRequired(false)
                // NOT setUnlockedDeviceRequired (API 28): it is the Android
                // analogue of the Keychain's WhenUnlocked, and the Apple
                // adapter chose AfterFirstUnlock precisely so a background
                // launch can read before the first unlock.
                // NOT setIsStrongBoxBacked: no benefit against a plaintext file
                // copied out of the sandbox, and StrongBoxUnavailableException
                // is an unchecked ProviderException whose catch clause would
                // have to name an API-28 type.
                .build());
        return g.generateKey();
    }
}
