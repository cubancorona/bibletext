package org.bibletext;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ServiceInfo;
import android.graphics.Bitmap;
import android.media.session.MediaSession;
import android.os.Build;
import android.os.IBinder;

/**
 * BtAudioService is the foreground-service anchor for background audio — the
 * Android half of what iOS gets from {@code UIBackgroundModes=audio}. While a
 * chapter plays (or is paused mid-listen) this service holds the process in the
 * foreground with a {@link Notification.MediaStyle} notification linked to
 * BtAudio's {@link MediaSession}, which is what puts the Now Playing card with
 * transport controls on the lock screen / quick-settings media carousel and
 * keeps Android from freezing the app (and the MediaPlayer/TTS in it) when the
 * screen goes off.
 *
 * Declared in the custom AndroidManifest.xml with
 * {@code android:foregroundServiceType="mediaPlayback"} — which is exactly why
 * the build had to move off Fyne's legacy binres manifest encoder onto the
 * aapt2 path (binres' baked-in API-21 attribute table can't encode that
 * API-29 attribute).
 *
 * The service owns NO audio state: BtAudio is the engine and the source of
 * truth; this class only renders its state into the notification and forwards
 * notification-action taps back into it. Pre-13 devices tap the notification's
 * own actions; Android 13+ renders transport from the MediaSession's
 * PlaybackState instead, so both paths land in BtAudio.
 *
 * Lifecycle: started (startForegroundService) by BtAudio when playback begins;
 * re-asserts startForeground on every onStartCommand (required within 5s of a
 * startForegroundService); stopped by BtAudio.stop() via stopService(). Swiping
 * the app away (onTaskRemoved) stops playback outright — matching iOS, and
 * avoiding a zombie session whose activity (and Fyne UI) is gone.
 */
public final class BtAudioService extends Service {
    static final String CHANNEL_ID = "bt.playback";
    static final int NOTIFICATION_ID = 0xB1B7;

    static final String ACT_TOGGLE = "org.bibletext.audio.TOGGLE";
    static final String ACT_REW = "org.bibletext.audio.REW";
    static final String ACT_FF = "org.bibletext.audio.FF";

    private static BtAudioService instance;

    @Override public void onCreate() {
        super.onCreate();
        instance = this;
        BtAudio.onServiceCreated();
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel ch = new NotificationChannel(
                    CHANNEL_ID, "Playback", NotificationManager.IMPORTANCE_LOW);
            ch.setShowBadge(false);
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null) nm.createNotificationChannel(ch);
        }
    }

    @Override public void onDestroy() {
        instance = null;
        super.onDestroy();
    }

    @Override public IBinder onBind(Intent intent) { return null; }

    @Override public int onStartCommand(Intent intent, int flags, int startId) {
        String a = intent == null ? null : intent.getAction();
        if (ACT_TOGGLE.equals(a)) {
            BtAudio.toggle();
        } else if (ACT_REW.equals(a)) {
            BtAudio.skip(-15);
        } else if (ACT_FF.equals(a)) {
            BtAudio.skip(15);
        }
        // Always (re)assert foreground with the current notification: required
        // promptly after a startForegroundService, and harmless on action taps
        // (the action already updated BtAudio's state, so this re-render is the
        // refresh). mediaPlayback FGS type must be passed explicitly on 29+.
        Notification n = buildNotification();
        try {
            if (Build.VERSION.SDK_INT >= 29) {
                startForeground(NOTIFICATION_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK);
            } else {
                startForeground(NOTIFICATION_ID, n);
            }
        } catch (Throwable t) {
            android.util.Log.w("BtAudioService", "startForeground failed", t);
        }
        // Zombie guard: if a stop raced our creation (BtAudio.stop() ran between
        // startForegroundService and this onStartCommand, when stopService()
        // found no instance to stop), the session is already over — honor the
        // startForeground contract above, then drop out immediately instead of
        // parking a stale non-dismissible 'playing' notification forever.
        if (!BtAudio.uiSessionLive()) {
            try { stopForeground(true); } catch (Throwable ignored) {}
            stopSelf();
        }
        return START_NOT_STICKY;
    }

    /**
     * The reader swiped the app out of recents: stop playback entirely (matches
     * iOS). Without this the FGS would keep narrating with the whole Fyne UI —
     * and the Go side's activity handle — gone.
     */
    @Override public void onTaskRemoved(Intent rootIntent) {
        BtAudio.stopFromService();
        try { stopForeground(true); } catch (Throwable ignored) {}
        stopSelf();
        super.onTaskRemoved(rootIntent);
    }

    /** refresh re-renders the notification from BtAudio's current state. */
    static void refresh() {
        BtAudioService s = instance;
        if (s == null) return;
        try {
            NotificationManager nm = (NotificationManager) s.getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null) nm.notify(NOTIFICATION_ID, s.buildNotification());
        } catch (Throwable t) {
            android.util.Log.w("BtAudioService", "refresh failed", t);
        }
    }

    static boolean isRunning() { return instance != null; }

    /** stopService drops the foreground state + notification and stops. */
    static void stopService() {
        BtAudioService s = instance;
        if (s == null) return;
        try { s.stopForeground(true); } catch (Throwable ignored) {}
        try { s.stopSelf(); } catch (Throwable ignored) {}
    }

    private PendingIntent actionIntent(String action, int code) {
        Intent i = new Intent(this, BtAudioService.class).setAction(action);
        int fl = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= 23) fl |= PendingIntent.FLAG_IMMUTABLE;
        return PendingIntent.getService(this, code, i, fl);
    }

    @SuppressWarnings("deprecation")
    private Notification buildNotification() {
        boolean playing = BtAudio.uiPlaying();
        boolean canSeek = BtAudio.uiCanSeek();
        String title = BtAudio.uiTitle();
        String artist = BtAudio.uiArtist();
        Bitmap art = BtAudio.uiArtwork();
        MediaSession.Token token = BtAudio.sessionToken();

        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= 26) {
            b = new Notification.Builder(this, CHANNEL_ID);
        } else {
            b = new Notification.Builder(this);
            b.setPriority(Notification.PRIORITY_LOW);
        }
        b.setSmallIcon(android.R.drawable.ic_media_play)
                .setContentTitle(title == null ? "BibleText" : title)
                .setContentText(artist == null ? "" : artist)
                .setVisibility(Notification.VISIBILITY_PUBLIC)
                .setOnlyAlertOnce(true)
                .setShowWhen(false)
                .setOngoing(playing);
        if (art != null) b.setLargeIcon(art);

        // Tapping the card brings the app forward (the launcher intent fronts
        // the existing task rather than spawning a second activity).
        try {
            Intent launch = getPackageManager().getLaunchIntentForPackage(getPackageName());
            if (launch != null) {
                int fl = PendingIntent.FLAG_UPDATE_CURRENT;
                if (Build.VERSION.SDK_INT >= 23) fl |= PendingIntent.FLAG_IMMUTABLE;
                b.setContentIntent(PendingIntent.getActivity(this, 0, launch, fl));
            }
        } catch (Throwable ignored) {}

        // Transport actions (framework glyphs — we ship no drawable resources).
        // ±15 only where the source can seek (recorded; TTS can't).
        int playPauseIdx = 0;
        if (canSeek) {
            b.addAction(new Notification.Action.Builder(
                    android.R.drawable.ic_media_rew, "Back 15 seconds", actionIntent(ACT_REW, 1)).build());
            playPauseIdx = 1;
        }
        b.addAction(new Notification.Action.Builder(
                playing ? android.R.drawable.ic_media_pause : android.R.drawable.ic_media_play,
                playing ? "Pause" : "Play", actionIntent(ACT_TOGGLE, 2)).build());
        if (canSeek) {
            b.addAction(new Notification.Action.Builder(
                    android.R.drawable.ic_media_ff, "Forward 15 seconds", actionIntent(ACT_FF, 3)).build());
        }

        Notification.MediaStyle style = new Notification.MediaStyle();
        if (token != null) style.setMediaSession(token);
        style.setShowActionsInCompactView(canSeek ? new int[]{0, 1, 2} : new int[]{playPauseIdx});
        b.setStyle(style);
        return b.build();
    }
}
