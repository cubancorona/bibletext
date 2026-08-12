package org.golang.app;

// Reconstructed from the fyne CLI's embedded prebuilt classes.dex (fyne.io/tools
// v1.7.2, dexdump disassembly) — byte-for-byte behavioural twin of the receiver
// the tool ships. It exists here because scripts/build-android.sh REPLACES the
// APK's classes.dex to pick up our patched GoNativeActivity (the tool's dex is
// prebuilt, so patching the .java in third_party/fyne alone changes nothing),
// and the manifest's <receiver> entry needs this class to keep resolving.
// If the fyne CLI is upgraded, re-dump its dex and re-verify this file matches.

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.os.Build;

public class FyneNotificationReceiver extends BroadcastReceiver {
    private static final String CHANNEL_ID = "fyne-notif";
    private static final int UNKNOWN_APP_ICON = 0x010d0000;

    @Override
    public void onReceive(Context context, Intent intent) {
        String title = intent.getStringExtra("title");
        String body = intent.getStringExtra("body");
        int id = intent.getIntExtra("notif_id", 1);
        NotificationManager mgr =
                (NotificationManager) context.getSystemService("notification");
        if (mgr == null) return;
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel ch = new NotificationChannel(
                    CHANNEL_ID, "Fyne Notification",
                    NotificationManager.IMPORTANCE_HIGH);
            mgr.createNotificationChannel(ch);
            builder = new Notification.Builder(context, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(context);
        }
        if (title == null) title = "";
        if (body == null) body = "";
        builder.setContentTitle(title)
                .setContentText(body)
                .setSmallIcon(UNKNOWN_APP_ICON)
                .setAutoCancel(true);
        mgr.notify(id, builder.build());
    }
}
