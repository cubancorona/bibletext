package org.bibletext;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.media.AudioAttributes;
import android.media.AudioFocusRequest;
import android.media.AudioManager;
import android.media.MediaMetadata;
import android.media.MediaPlayer;
import android.media.session.MediaSession;
import android.media.session.PlaybackState;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.PowerManager;
import android.speech.tts.TextToSpeech;
import android.speech.tts.UtteranceProgressListener;
import android.util.Log;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;

/**
 * BtAudio is BibleText's native Android audio engine — the twin of the iOS
 * BTAudioController (audio_ios.go). Two engines behind one façade, driven from Go
 * over JNI (audio_android.go):
 *   - a {@link MediaPlayer} streams a recorded MP3 chapter (HTTP-range seekable —
 *     the ±15s skip); MediaPlayer's extractor sniffs MP3 by content, so the CDN's
 *     extension-less {@code application/octet-stream} response needs no MIME hint
 *     (unlike iOS/AVPlayer).
 *   - a {@link TextToSpeech} reads the chapter's own verses aloud for every version
 *     / book without a recording, always matching the on-screen text.
 *
 * State changes post back to Go via the {@code native} methods below (→
 * audio_jni_android.c → audio_export_android.go) so the play button and read-along
 * update, exactly like the iOS {@code bibleTextAudioStateChanged} path. The
 * position poll ({@link #nativeAudioTime}) and the TTS
 * {@link UtteranceProgressListener#onRangeStart} range ({@link #nativeAudioRange})
 * drive read-along, twins of the AVPlayer time observer and
 * willSpeakRangeOfSpeechString.
 *
 * Ships as classes2.dex alongside BtBridge (scripts/build-android.sh). Every method
 * is invoked from a JNI-attached background thread (fyne RunNative); each hops its
 * work onto the main {@link Looper} through {@link #UI}, so MediaPlayer and
 * TextToSpeech (which want a Looper thread) are always touched from main.
 *
 * Background + lock-screen transport: BtAudio owns a framework
 * {@link MediaSession} (PlaybackState drives the Android 13+ system media
 * controls; REWIND/FAST_FORWARD are the ±15s, SEEK_TO the scrubber — omitted
 * for TTS, like iOS) and {@link BtAudioService} anchors the process in the
 * foreground while a listening session is live, so narration survives
 * screen-off and chapters roll over while the device sleeps.
 */
public final class BtAudio {
    private static final Handler UI = new Handler(Looper.getMainLooper());

    // State codes posted to Go — kept in sync with audioPlayState (audio_controller.go)
    // and the iOS enum. Mode selects which engine is live.
    private static final int ST_IDLE = 0, ST_PLAYING = 1, ST_PAUSED = 2,
                             ST_ENDED = 3, ST_FAILED = 4, ST_BUFFERING = 5;
    private static final int MODE_NONE = 0, MODE_URL = 1, MODE_TTS = 2;

    private static Activity activity;
    private static Context appContext;
    private static AudioManager audioManager;
    private static int mode = MODE_NONE;

    // Whether the reader (or lock screen) paused deliberately — gates auto-resume
    // after a transient audio-focus loss (a phone call), like iOS userPaused.
    private static boolean userPaused = false;
    // Set while a transient focus loss paused us, so the matching GAIN resumes.
    private static boolean focusLossPaused = false;

    private static String title, artist;
    private static Bitmap artwork;   // lock-screen / notification card (rendered by Go)

    // ---- MediaSession + foreground service (background / lock-screen) ----
    // The session is the OS-facing transport surface: Android 13+ renders the
    // lock-screen / quick-settings media controls from its PlaybackState (the
    // notification's own actions only matter pre-13). BtAudioService anchors the
    // process in the foreground so playback survives screen-off; syncService
    // starts/stops it as playback state changes.
    private static MediaSession session;
    private static PowerManager.WakeLock wakeLock;
    private static int durationMs = 0;          // recorded duration once prepared (0 = unknown)
    private static int lastUiState = ST_IDLE;   // last state posted — what the notification renders
    private static boolean svcStartRequested = false; // startForegroundService issued, onCreate pending
    private static boolean notifPermAsked = false;    // POST_NOTIFICATIONS prompt shown once

    // sessionLive spans one LISTENING SESSION: raised by startURL/startTTS,
    // dropped only by a real end (Go-driven stop, swipe-away, permanent focus
    // loss, unrecoverable TTS error). The service lifecycle keys off THIS — not
    // individual states — so it survives chapter transitions AND the
    // recorded-FAILED → TTS-fallback window (restarting a foreground service
    // from the background would throw on API 31+, so it must never go down
    // mid-session). It also lets a service whose creation raced a stop
    // recognise itself as a zombie and self-terminate (BtAudioService checks
    // uiSessionLive in onStartCommand).
    private static boolean sessionLive = false;

    // ---- recorded MP3 (MediaPlayer) ----
    private static MediaPlayer mp;
    private static boolean mpPlaying = false;
    private static boolean mpPrepared = false;  // onPrepared landed (start/seek legal)

    // ---- on-device TTS ----
    private static TextToSpeech tts;
    private static boolean ttsReady = false;
    private static String ttsText = "";
    private static int ttsGen = 0;              // bumped on every (re)start/stop; scopes callbacks
    private static boolean ttsSpeaking = false;
    private static boolean ttsPaused = false;
    private static int ttsLastGlobal = 0;       // last global char offset spoken (pause-resume anchor)
    private static int ttsPausedOffset = 0;
    private static String ttsLastUtt = "";      // id of the final chunk → ENDED
    private static final HashMap<String, Integer> ttsBase = new HashMap<>(); // utteranceId → global base offset
    private static String pendingTTSText = null; // set if startTTS lands before the engine finishes init
    private static boolean ttsPending = false;   // a start is queued waiting for the engine (toggle can cancel it)

    // ---- audio focus ----
    private static AudioFocusRequest focusRequest; // API 26+
    private static final AudioManager.OnAudioFocusChangeListener focusListener =
        new AudioManager.OnAudioFocusChangeListener() {
            @Override public void onAudioFocusChange(final int change) {
                UI.post(new Runnable() { @Override public void run() { handleFocus(change); } });
            }
        };

    // Called from native (audio_jni_android.c → audio_export_android.go).
    private static native void nativeAudioState(int code);
    private static native void nativeAudioTime(double seconds);
    private static native void nativeAudioRange(int location);

    private BtAudio() {}

    /** init stores the activity/context and prepares the TTS engine (async). */
    public static void init(final Activity act) {
        UI.post(new Runnable() {
            @Override public void run() {
                activity = act;
                if (act != null) appContext = act.getApplicationContext();
                if (audioManager == null && appContext != null) {
                    audioManager = (AudioManager) appContext.getSystemService(Context.AUDIO_SERVICE);
                }
                ensureTTS();
            }
        });
    }

    private static void ensureTTS() {
        if (tts != null || appContext == null) return;
        tts = new TextToSpeech(appContext, new TextToSpeech.OnInitListener() {
            @Override public void onInit(int status) {
                UI.post(new Runnable() { @Override public void run() {
                    if (status == TextToSpeech.SUCCESS) {
                        ttsReady = true;
                        try { tts.setLanguage(Locale.US); } catch (Throwable ignored) {}
                        tts.setOnUtteranceProgressListener(new UtteranceProgressListener() {
                            @Override public void onStart(String id) {}
                            @Override public void onDone(final String id) {
                                UI.post(new Runnable() { @Override public void run() { ttsOnDone(id); } });
                            }
                            @Override public void onError(final String id) {
                                UI.post(new Runnable() { @Override public void run() { ttsOnError(id); } });
                            }
                            // API 26+: character range about to be spoken (per-utterance offsets).
                            @Override public void onRangeStart(final String id, final int start, final int end, final int frame) {
                                UI.post(new Runnable() { @Override public void run() { ttsOnRange(id, start); } });
                            }
                        });
                        if (pendingTTSText != null && ttsPending) {
                            String t = pendingTTSText; pendingTTSText = null; ttsPending = false;
                            if (!requestFocus()) {
                                // Deferred start landed during a call — park paused.
                                ttsPaused = true;
                                ttsPausedOffset = 0;
                                post(ST_PAUSED);
                            } else {
                                speakFrom(t, 0);
                            }
                        }
                    } else {
                        Log.w("BtAudio", "TextToSpeech init failed: " + status);
                        if (mode == MODE_TTS && sessionLive) {
                            // The engine is dead with a session up (we already told Go
                            // PLAYING): end it cleanly — no wake lock / FGS / 'playing'
                            // notification parked over permanent silence.
                            hardStopLocal();
                            post(ST_IDLE);
                        }
                    }
                }});
            }
        });
    }

    // ================= recorded MP3 =================

    /** startURL streams a recorded chapter MP3 (seekable). */
    public static void startURL(final String url, final String t, final String a) {
        UI.post(new Runnable() {
            @Override public void run() {
                teardownEngines();
                mode = MODE_URL;
                sessionLive = true;
                userPaused = false;
                focusLossPaused = false;
                mpPrepared = false;
                title = t; artist = a;
                durationMs = 0;
                ensureSession();
                maybeRequestNotifPermission();
                updateMetadata();
                try {
                    mp = new MediaPlayer();
                    mp.setAudioAttributes(new AudioAttributes.Builder()
                            .setUsage(AudioAttributes.USAGE_MEDIA)
                            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build());
                    mp.setOnPreparedListener(new MediaPlayer.OnPreparedListener() {
                        @Override public void onPrepared(MediaPlayer m) {
                            if (mode != MODE_URL || mp != m) return;
                            mpPrepared = true;
                            try { durationMs = m.getDuration(); } catch (Throwable ignored) {}
                            updateMetadata();   // duration now known → lock-screen scrubber
                            if (userPaused) {
                                // Paused from the lock screen while still buffering —
                                // honor it: hold ready instead of starting out loud.
                                post(ST_PAUSED);
                                return;
                            }
                            if (!requestFocus()) {
                                // Focus denied (active phone call etc.) — hold as
                                // paused; resume works once the call ends.
                                post(ST_PAUSED);
                                return;
                            }
                            m.start();
                            mpPlaying = true;
                            post(ST_PLAYING);
                            startPoll();
                        }
                    });
                    mp.setOnCompletionListener(new MediaPlayer.OnCompletionListener() {
                        @Override public void onCompletion(MediaPlayer m) {
                            if (mode != MODE_URL || mp != m) return;
                            mpPlaying = false;
                            stopPoll();
                            post(ST_ENDED);
                        }
                    });
                    mp.setOnErrorListener(new MediaPlayer.OnErrorListener() {
                        @Override public boolean onError(MediaPlayer m, int what, int extra) {
                            if (mode != MODE_URL || mp != m) return true;
                            Log.w("BtAudio", "MediaPlayer error " + what + "/" + extra);
                            teardownEngines();
                            post(ST_FAILED);   // Go restarts the chapter as read-aloud
                            return true;
                        }
                    });
                    mp.setOnInfoListener(new MediaPlayer.OnInfoListener() {
                        @Override public boolean onInfo(MediaPlayer m, int what, int extra) {
                            if (mode != MODE_URL || mp != m) return false;
                            if (what == MediaPlayer.MEDIA_INFO_BUFFERING_START) {
                                post(ST_BUFFERING);
                            } else if (what == MediaPlayer.MEDIA_INFO_BUFFERING_END) {
                                if (mpPlaying) post(ST_PLAYING);
                            }
                            return false;
                        }
                    });
                    mp.setOnSeekCompleteListener(new MediaPlayer.OnSeekCompleteListener() {
                        @Override public void onSeekComplete(MediaPlayer m) {
                            if (mode != MODE_URL || mp != m) return;
                            updateSession();   // re-sync the lock-screen position after ±15 / scrub
                        }
                    });
                    mp.setDataSource(url);
                    post(ST_BUFFERING);        // stream coming up — spinner until prepared
                    mp.prepareAsync();
                } catch (Throwable e) {
                    Log.w("BtAudio", "startURL failed", e);
                    teardownEngines();
                    post(ST_FAILED);
                }
            }
        });
    }

    private static final Runnable poll = new Runnable() {
        @Override public void run() {
            if (mode != MODE_URL || mp == null || !mpPlaying) return;
            try { nativeAudioTime(mp.getCurrentPosition() / 1000.0); } catch (Throwable ignored) {}
            UI.postDelayed(this, 200);   // ~5×/sec, matching the iOS observer
        }
    };

    private static void startPoll() { UI.removeCallbacks(poll); UI.post(poll); }
    private static void stopPoll() { UI.removeCallbacks(poll); }

    // ================= on-device TTS =================

    /** startTTS reads the chapter's verses aloud on-device. */
    public static void startTTS(final String text, final String t, final String a) {
        UI.post(new Runnable() {
            @Override public void run() {
                teardownEngines();
                mode = MODE_TTS;
                sessionLive = true;
                userPaused = false;
                focusLossPaused = false;
                ttsPaused = false;
                title = t; artist = a;
                ttsText = text == null ? "" : text;
                durationMs = 0;   // TTS has no clock → no duration metadata, no scrubber
                ensureSession();
                maybeRequestNotifPermission();
                updateMetadata();
                if (tts == null) ensureTTS();
                if (!ttsReady || tts == null) {
                    // Engine still initialising — speak once onInit lands. ttsPending
                    // lets a pause tap in this window cancel the queued start (toggle).
                    pendingTTSText = ttsText;
                    ttsPending = true;
                    post(ST_PLAYING);   // intended playing; the button shows the wave glyph
                    return;
                }
                if (!requestFocus()) {
                    // Focus denied (active phone call etc.) — park paused at the
                    // start; play/resume retries once the call ends.
                    ttsPaused = true;
                    ttsPausedOffset = 0;
                    post(ST_PAUSED);
                    return;
                }
                speakFrom(ttsText, 0);
                post(ST_PLAYING);
            }
        });
    }

    // speakFrom splits text[fromOffset:] into ≤maxLen chunks at whitespace and queues
    // them, recording each chunk's GLOBAL base offset so onRangeStart maps to the
    // whole-chapter offset (matching speechVerseOffsets). Bumps the generation so any
    // late callback from a prior utterance is ignored.
    private static void speakFrom(String full, int fromOffset) {
        if (tts == null || full == null) return;
        ttsGen++;
        int gen = ttsGen;
        ttsBase.clear();
        ttsSpeaking = true;
        int max = TextToSpeech.getMaxSpeechInputLength();
        if (max > 3900 || max <= 0) max = 3900;   // leave margin under the engine cap
        String s = fromOffset <= 0 ? full : full.substring(Math.min(fromOffset, full.length()));
        List<int[]> chunks = new ArrayList<>();    // {localStart, localEnd}
        int i = 0, len = s.length();
        while (i < len) {
            int end = Math.min(i + max, len);
            if (end < len) {
                int sp = s.lastIndexOf(' ', end);
                if (sp > i) end = sp + 1;          // break after a space, no lost chars
            }
            chunks.add(new int[]{i, end});
            i = end;
        }
        if (chunks.isEmpty()) { post(ST_ENDED); ttsSpeaking = false; return; }
        ttsLastUtt = "u_" + gen + "_" + (chunks.size() - 1);
        for (int k = 0; k < chunks.size(); k++) {
            int[] c = chunks.get(k);
            String id = "u_" + gen + "_" + k;
            ttsBase.put(id, fromOffset + c[0]);
            int q = (k == 0) ? TextToSpeech.QUEUE_FLUSH : TextToSpeech.QUEUE_ADD;
            try { tts.speak(s.substring(c[0], c[1]), q, (Bundle) null, id); } catch (Throwable ignored) {}
        }
    }

    private static boolean currentGen(String id) {
        return id != null && id.startsWith("u_" + ttsGen + "_");
    }

    private static void ttsOnRange(String id, int start) {
        if (mode != MODE_TTS || !currentGen(id)) return;
        Integer base = ttsBase.get(id);
        int global = (base == null ? 0 : base) + start;
        ttsLastGlobal = global;
        try { nativeAudioRange(global); } catch (Throwable ignored) {}
    }

    private static void ttsOnDone(String id) {
        if (mode != MODE_TTS || !currentGen(id)) return;
        if (id.equals(ttsLastUtt)) {
            ttsSpeaking = false;
            post(ST_ENDED);
        }
    }

    private static void ttsOnError(String id) {
        if (mode != MODE_TTS || !currentGen(id)) return;
        // A non-final chunk can error while later QUEUE_ADD chunks are still queued;
        // bump the generation and flush the queue so those chunks don't keep
        // speaking (and firing read-along ranges) after we've reported stopped.
        ttsGen++;
        try { if (tts != null) tts.stop(); } catch (Throwable ignored) {}
        ttsSpeaking = false;
        // End the session outright. (An earlier version parked "paused at the
        // error point" for a lock-screen resume, but Go has already disowned the
        // chapter on IDLE — resuming a disowned session desyncs the in-app button
        // and defeats stop-on-navigation. A clean stop keeps every surface
        // consistent; the reader restarts from the app.)
        hardStopLocal();
        post(ST_IDLE);   // clean stop — NOT ENDED (which would advance) or FAILED
    }

    // ================= shared transport =================

    /** toggle flips play/pause for the live engine. */
    public static void toggle() {
        UI.post(new Runnable() {
            @Override public void run() {
                if (mode == MODE_URL && mp != null) {
                    if (mpPlaying) {
                        userPaused = true; doPause(); post(ST_PAUSED);
                    } else if (!mpPrepared) {
                        // Still buffering: flip the latched intent (a pause here is
                        // honored by onPrepared; a second tap re-arms the start).
                        userPaused = !userPaused;
                        post(userPaused ? ST_PAUSED : ST_BUFFERING);
                    } else {
                        userPaused = false; focusLossPaused = false;
                        if (doResume()) post(ST_PLAYING); else post(ST_PAUSED);
                    }
                } else if (mode == MODE_TTS) {
                    if (ttsPending) {
                        // A start is queued waiting for the engine to init; cancel it
                        // and enter the paused state so the next tap resumes from 0.
                        ttsPending = false; pendingTTSText = null;
                        userPaused = true; ttsPaused = true; ttsPausedOffset = 0;
                        post(ST_PAUSED);
                    } else if (ttsSpeaking && !ttsPaused) {
                        userPaused = true; ttsPaused = true; doPause(); post(ST_PAUSED);
                    } else if (ttsPaused) {
                        userPaused = false; focusLossPaused = false;
                        if (doResume()) {
                            post(ST_PLAYING);
                        } else {
                            ttsPaused = true;   // stay parked; retry later
                            post(ST_PAUSED);
                        }
                    }
                }
            }
        });
    }

    // doPause/doResume are the mode-agnostic mechanics, WITHOUT touching userPaused
    // (so both a manual toggle and a focus-loss can share them).
    private static void doPause() {
        if (mode == MODE_URL && mp != null) {
            try { if (mpPlaying) mp.pause(); } catch (Throwable ignored) {}
            mpPlaying = false;
            stopPoll();
        } else if (mode == MODE_TTS) {
            // TextToSpeech can't pause/resume, so remember where we were and stop;
            // resume re-speaks from that verse's offset (bump gen to drop stale cbs).
            ttsPausedOffset = ttsLastGlobal;
            ttsGen++;
            try { if (tts != null) tts.stop(); } catch (Throwable ignored) {}
            ttsSpeaking = false;
        }
    }

    // doResume returns false when playback could not restart (focus denied —
    // e.g. an active phone call — or the engine refused); callers post PAUSED
    // then, so no surface claims sound that isn't happening.
    private static boolean doResume() {
        if (!requestFocus()) return false;
        if (mode == MODE_URL && mp != null) {
            try {
                mp.start();
                mpPlaying = true;
                startPoll();
            } catch (Throwable ignored) {
                return false;
            }
            return true;
        } else if (mode == MODE_TTS && tts != null) {
            ttsPaused = false;
            if (!ttsReady) {
                // Resumed before the engine finished init (paused during cold start) —
                // re-queue from the top; onInit will pick it up.
                pendingTTSText = ttsText;
                ttsPending = true;
            } else {
                speakFrom(ttsText, ttsPausedOffset);
            }
            return true;
        }
        return false;
    }

    /** skip seeks the recorded player by ±seconds (no-op for TTS). */
    public static void skip(final double seconds) {
        UI.post(new Runnable() {
            @Override public void run() {
                if (mode != MODE_URL || mp == null) return;
                try {
                    int cur = mp.getCurrentPosition();
                    int dur = mp.getDuration();
                    int target = cur + (int) Math.round(seconds * 1000.0);
                    if (target < 0) target = 0;
                    if (dur > 0 && target > dur) target = dur;
                    if (Build.VERSION.SDK_INT >= 26) {
                        mp.seekTo((long) target, MediaPlayer.SEEK_CLOSEST);
                    } else {
                        mp.seekTo(target);
                    }
                } catch (Throwable ignored) {}
            }
        });
    }

    /** stop tears both engines down + abandons focus. No callback — Go drove the stop. */
    public static void stop() {
        UI.post(new Runnable() {
            @Override public void run() { hardStopLocal(); }
        });
    }

    /**
     * stopFromService: the reader swiped the app out of recents (service
     * onTaskRemoved) — stop everything, matching iOS. Posts ST_IDLE so the Go
     * controller (still alive in this process) clears its loaded state.
     */
    static void stopFromService() {
        UI.post(new Runnable() {
            @Override public void run() {
                hardStopLocal();
                post(ST_IDLE);
            }
        });
    }

    // hardStopLocal ends the LISTENING SESSION natively — engines, focus, wake
    // lock, MediaSession, foreground service — WITHOUT notifying Go (callers
    // post() when Go needs to know). Resetting lastUiState matters for the
    // stop-races-service-creation window: a service that comes up after this
    // renders idle state and immediately self-stops (uiSessionLive false).
    private static void hardStopLocal() {
        teardownEngines();
        abandonFocus();
        sessionLive = false;
        lastUiState = ST_IDLE;
        releaseSessionAndService();
    }

    private static void teardownEngines() {
        stopPoll();
        if (mp != null) {
            try { mp.setOnPreparedListener(null); mp.setOnCompletionListener(null);
                  mp.setOnErrorListener(null); mp.setOnInfoListener(null); } catch (Throwable ignored) {}
            try { mp.reset(); } catch (Throwable ignored) {}
            try { mp.release(); } catch (Throwable ignored) {}
            mp = null;
        }
        mpPlaying = false;
        mpPrepared = false;
        ttsGen++;   // invalidate any in-flight TTS callbacks
        if (tts != null) { try { tts.stop(); } catch (Throwable ignored) {} }
        ttsSpeaking = false;
        ttsPaused = false;
        ttsPending = false;
        pendingTTSText = null;
        releaseWake();
        mode = MODE_NONE;
    }

    /** setArtwork sets the lock-screen / notification card (Go renders the PNG async). */
    public static void setArtwork(final String path) {
        UI.post(new Runnable() {
            @Override public void run() {
                try { artwork = BitmapFactory.decodeFile(path); } catch (Throwable ignored) {}
                if (artwork != null && mode != MODE_NONE) {
                    updateMetadata();          // album art onto the session
                    BtAudioService.refresh();  // large icon onto the notification
                }
            }
        });
    }

    // ================= MediaSession + foreground service =================

    // ensureSession creates the (single, reused) MediaSession and wires the
    // OS transport callbacks — the Android twin of iOS's MPRemoteCommandCenter
    // handlers. Callbacks arrive on the main looper (session created there).
    private static void ensureSession() {
        if (session != null || appContext == null) return;
        try {
            session = new MediaSession(appContext, "BibleText");
            session.setCallback(new MediaSession.Callback() {
                @Override public void onPlay() { resumeFromRemote(); }
                @Override public void onPause() { pauseFromRemote(); }
                @Override public void onRewind() { skip(-15); }
                @Override public void onFastForward() { skip(15); }
                @Override public void onSeekTo(long pos) { seekToMs(pos); }
            });
            if (Build.VERSION.SDK_INT < 26) {
                session.setFlags(MediaSession.FLAG_HANDLES_MEDIA_BUTTONS
                        | MediaSession.FLAG_HANDLES_TRANSPORT_CONTROLS);
            }
            session.setActive(true);   // routes hardware media buttons here
        } catch (Throwable t) {
            Log.w("BtAudio", "MediaSession create failed", t);
            session = null;
        }
    }

    private static void releaseSessionAndService() {
        svcStartRequested = false;
        BtAudioService.stopService();
        if (session != null) {
            try { session.setActive(false); session.release(); } catch (Throwable ignored) {}
            session = null;
        }
    }

    // pauseFromRemote / resumeFromRemote are the PRECISE lock-screen verbs (the
    // OS sends distinct PLAY and PAUSE, unlike the notification's toggle) —
    // idempotent, so a stray duplicate command can't flip us the wrong way.
    private static void pauseFromRemote() {
        if (mode == MODE_URL && mp != null && mpPlaying) {
            userPaused = true;
            doPause();
            post(ST_PAUSED);
        } else if (mode == MODE_URL && mp != null && !mpPrepared) {
            // Still buffering: LATCH the pause — onPrepared honors userPaused and
            // holds ready instead of starting out loud against an explicit
            // lock-screen pause (phone now in a pocket / meeting).
            userPaused = true;
            post(ST_PAUSED);
        } else if (mode == MODE_TTS && ttsSpeaking && !ttsPaused) {
            userPaused = true;
            ttsPaused = true;
            doPause();
            post(ST_PAUSED);
        } else if (mode == MODE_TTS && ttsPending) {
            ttsPending = false;
            pendingTTSText = null;
            userPaused = true;
            ttsPaused = true;
            ttsPausedOffset = 0;
            post(ST_PAUSED);
        }
    }

    private static void resumeFromRemote() {
        if (mode == MODE_URL && mp != null && !mpPlaying) {
            userPaused = false;
            focusLossPaused = false;
            if (!mpPrepared) {
                post(ST_BUFFERING);   // un-latch; onPrepared starts playback
                return;
            }
            if (doResume()) post(ST_PLAYING); else post(ST_PAUSED);
        } else if (mode == MODE_TTS && (ttsPaused || ttsPending)) {
            userPaused = false;
            focusLossPaused = false;
            ttsPaused = false;
            ttsPending = false;
            if (doResume()) {
                post(ST_PLAYING);
            } else {
                ttsPaused = true;   // stay parked; retry later
                post(ST_PAUSED);
            }
        }
    }

    // seekToMs services the lock-screen scrubber (recorded only).
    private static void seekToMs(long pos) {
        if (mode != MODE_URL || mp == null) return;
        try {
            int dur = durationMs > 0 ? durationMs : mp.getDuration();
            long target = Math.max(0, dur > 0 ? Math.min(pos, dur) : pos);
            if (Build.VERSION.SDK_INT >= 26) {
                mp.seekTo(target, MediaPlayer.SEEK_CLOSEST);
            } else {
                mp.seekTo((int) target);
            }
        } catch (Throwable ignored) {}
    }

    // updateSession publishes the transport state the system media controls
    // render (Android 13+ builds the lock-screen buttons from these actions).
    private static void updateSession() {
        if (session == null) return;
        try {
            long actions = PlaybackState.ACTION_PLAY | PlaybackState.ACTION_PAUSE
                    | PlaybackState.ACTION_PLAY_PAUSE;
            if (mode == MODE_URL) {
                actions |= PlaybackState.ACTION_REWIND | PlaybackState.ACTION_FAST_FORWARD
                        | PlaybackState.ACTION_SEEK_TO;
            }
            int s;
            switch (lastUiState) {
                case ST_PLAYING:   s = PlaybackState.STATE_PLAYING; break;
                case ST_PAUSED:    s = PlaybackState.STATE_PAUSED; break;
                case ST_BUFFERING: s = PlaybackState.STATE_BUFFERING; break;
                default:           s = PlaybackState.STATE_STOPPED; break;
            }
            long pos = PlaybackState.PLAYBACK_POSITION_UNKNOWN;
            if (mode == MODE_URL && mp != null) {
                try { pos = mp.getCurrentPosition(); } catch (Throwable ignored) {}
            }
            session.setPlaybackState(new PlaybackState.Builder()
                    .setState(s, pos, lastUiState == ST_PLAYING ? 1f : 0f)
                    .setActions(actions)
                    .build());
        } catch (Throwable ignored) {}
    }

    // updateMetadata publishes title/artist/duration/art. Duration only for
    // recorded chapters — omitting it for TTS suppresses the scrubber, exactly
    // like the iOS Now Playing card.
    private static void updateMetadata() {
        if (session == null) return;
        try {
            MediaMetadata.Builder m = new MediaMetadata.Builder();
            if (title != null) m.putString(MediaMetadata.METADATA_KEY_TITLE, title);
            if (artist != null) m.putString(MediaMetadata.METADATA_KEY_ARTIST, artist);
            if (mode == MODE_URL && durationMs > 0) {
                m.putLong(MediaMetadata.METADATA_KEY_DURATION, durationMs);
            }
            if (artwork != null) m.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, artwork);
            session.setMetadata(m.build());
        } catch (Throwable ignored) {}
    }

    // syncService keeps the foreground service in step with THE SESSION (not
    // individual states): started when sound is (intended to be) coming out,
    // refreshed on every state change, stopped only when the session ends
    // (hardStopLocal → sessionLive false). Keying off sessionLive — not mode —
    // keeps the service alive across chapter transitions AND the recorded-FAILED
    // → TTS-fallback window, so it never needs a (forbidden on API 31+)
    // foreground-service restart from the background. The svcStartRequested
    // guard prevents spamming startForegroundService before onCreate lands.
    private static void syncService(int code) {
        if (appContext == null) return;
        if (!sessionLive) {
            svcStartRequested = false;
            BtAudioService.stopService();
            return;
        }
        boolean active = code == ST_PLAYING || code == ST_BUFFERING;
        if (active && !BtAudioService.isRunning()) {
            if (svcStartRequested) return;   // onCreate pending; it renders current state
            svcStartRequested = true;
            try {
                Intent i = new Intent(appContext, BtAudioService.class);
                if (Build.VERSION.SDK_INT >= 26) {
                    appContext.startForegroundService(i);
                } else {
                    appContext.startService(i);
                }
            } catch (Throwable t) {
                svcStartRequested = false;
                Log.w("BtAudio", "start service failed", t);
            }
            return;
        }
        BtAudioService.refresh();
    }

    /** Called by BtAudioService.onCreate — the start request has landed. */
    static void onServiceCreated() { svcStartRequested = false; }

    // A partial wake lock covers BOTH engines while audible (MediaPlayer's own
    // setWakeMode would cover only the recorded path; TTS chunk-queuing between
    // utterances needs the CPU too). Held during PLAYING/BUFFERING only.
    private static void acquireWake() {
        try {
            if (wakeLock == null && appContext != null) {
                PowerManager pm = (PowerManager) appContext.getSystemService(Context.POWER_SERVICE);
                if (pm == null) return;
                wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "bibletext:audio");
                wakeLock.setReferenceCounted(false);
            }
            if (wakeLock != null && !wakeLock.isHeld()) wakeLock.acquire();
        } catch (Throwable ignored) {}
    }

    private static void releaseWake() {
        try { if (wakeLock != null && wakeLock.isHeld()) wakeLock.release(); } catch (Throwable ignored) {}
    }

    // POST_NOTIFICATIONS is a runtime permission on 13+ (needed for the media
    // notification). Asked once, at first playback — contextual, and safe:
    // GoNativeActivity overrides no onRequestPermissionsResult, so the callback
    // lands in Activity's no-op.
    private static void maybeRequestNotifPermission() {
        if (Build.VERSION.SDK_INT < 33 || notifPermAsked) return;
        Activity act = activity;
        if (act == null || appContext == null) return;
        notifPermAsked = true;
        try {
            if (appContext.checkSelfPermission("android.permission.POST_NOTIFICATIONS")
                    != PackageManager.PERMISSION_GRANTED) {
                act.requestPermissions(new String[]{"android.permission.POST_NOTIFICATIONS"}, 0xB1B7);
            }
        } catch (Throwable ignored) {}
    }

    // ---- state the service pulls when rendering the notification (all set on
    //      the main thread, read on the main thread) ----
    static MediaSession.Token sessionToken() { return session == null ? null : session.getSessionToken(); }
    static String uiTitle() { return title; }
    static String uiArtist() { return artist; }
    static Bitmap uiArtwork() { return artwork; }
    static boolean uiPlaying() { return lastUiState == ST_PLAYING || lastUiState == ST_BUFFERING; }
    static boolean uiCanSeek() { return mode == MODE_URL; }
    static boolean uiSessionLive() { return sessionLive; }

    // ================= audio focus =================

    // requestFocus returns whether playback may proceed. FAILED (e.g. an active
    // phone call) → callers hold as paused instead of narrating over the call —
    // the Android analog of iOS's btAudioSetupSession bail. Fail-open on
    // exceptions (matches the previous best-effort behavior).
    private static boolean requestFocus() {
        if (audioManager == null) return true;
        try {
            int r;
            if (Build.VERSION.SDK_INT >= 26) {
                if (focusRequest == null) {
                    AudioAttributes attrs = new AudioAttributes.Builder()
                            .setUsage(AudioAttributes.USAGE_MEDIA)
                            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build();
                    focusRequest = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN)
                            .setAudioAttributes(attrs)
                            .setOnAudioFocusChangeListener(focusListener, UI)
                            .setWillPauseWhenDucked(true)
                            .build();
                }
                r = audioManager.requestAudioFocus(focusRequest);
            } else {
                r = audioManager.requestAudioFocus(focusListener,
                        AudioManager.STREAM_MUSIC, AudioManager.AUDIOFOCUS_GAIN);
            }
            return r != AudioManager.AUDIOFOCUS_REQUEST_FAILED;
        } catch (Throwable ignored) {
            return true;
        }
    }

    private static void abandonFocus() {
        if (audioManager == null) return;
        try {
            if (Build.VERSION.SDK_INT >= 26) {
                if (focusRequest != null) audioManager.abandonAudioFocusRequest(focusRequest);
            } else {
                audioManager.abandonAudioFocus(focusListener);
            }
        } catch (Throwable ignored) {}
    }

    // handleFocus mirrors the iOS interruption handler: a transient loss (phone
    // call, another app) pauses; GAIN resumes unless the reader had paused; a
    // permanent loss stops.
    private static void handleFocus(int change) {
        switch (change) {
            case AudioManager.AUDIOFOCUS_LOSS_TRANSIENT:
            case AudioManager.AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK:
                if ((mode == MODE_URL && mpPlaying) || (mode == MODE_TTS && ttsSpeaking && !ttsPaused)) {
                    focusLossPaused = true;
                    doPause();
                    if (mode == MODE_TTS) ttsPaused = true;
                    post(ST_PAUSED);
                }
                break;
            case AudioManager.AUDIOFOCUS_GAIN:
                if (focusLossPaused && !userPaused && mode != MODE_NONE) {
                    focusLossPaused = false;
                    if (doResume()) post(ST_PLAYING);
                }
                break;
            case AudioManager.AUDIOFOCUS_LOSS:
                hardStopLocal();
                post(ST_IDLE);
                break;
        }
    }

    // post reports a state change to Go AND mirrors it onto every OS surface in
    // one place: the wake lock, the MediaSession PlaybackState (lock-screen
    // controls), and the foreground service / notification. Always main-thread.
    private static void post(int code) {
        lastUiState = code;
        if (code == ST_PLAYING || code == ST_BUFFERING) {
            acquireWake();
        } else {
            releaseWake();
        }
        updateSession();
        syncService(code);
        try { nativeAudioState(code); } catch (Throwable ignored) {}
    }
}
