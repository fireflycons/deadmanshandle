package com.fireflycons.deadmanshandle.push

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.fireflycons.deadmanshandle.MainActivity
import com.fireflycons.deadmanshandle.R

/** Shows pushes as notifications, in a channel per [PushMessage.Channel] */
object Notifications {
    /** Creates the channels, so they can be set in Settings before the first push */
    fun createChannels(context: Context) {
        val alerts = NotificationChannel(
            PushMessage.Channel.ALERTS.id,
            context.getString(R.string.channel_alerts),
            NotificationManager.IMPORTANCE_HIGH,
        ).apply { description = context.getString(R.string.channel_alerts_description) }
        val reminders = NotificationChannel(
            PushMessage.Channel.REMINDERS.id,
            context.getString(R.string.channel_reminders),
            NotificationManager.IMPORTANCE_DEFAULT,
        ).apply { description = context.getString(R.string.channel_reminders_description) }
        context.getSystemService(NotificationManager::class.java).createNotificationChannels(listOf(alerts, reminders))
    }

    /** Whether notifications can be shown: allowed, and not turned off in Settings */
    fun allowed(context: Context): Boolean =
        hasPermission(context) && NotificationManagerCompat.from(context).areNotificationsEnabled()

    /** Shows [push]; tapping it opens the app. Does nothing if not [allowed]. */
    fun show(context: Context, push: PushMessage) {
        val open = PendingIntent.getActivity(
            context,
            0,
            Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val notification = NotificationCompat.Builder(context, push.channel.id)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(push.title)
            .setContentText(push.body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(push.body))
            .setContentIntent(open)
            .setAutoCancel(true)
            .build()
        // Checked here, where lint can see it
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
        NotificationManagerCompat.from(context).notify(push.notificationId, notification)
    }

    // POST_NOTIFICATIONS is a runtime permission from Android 13; before, it is implied
    private fun hasPermission(context: Context) =
        Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
}
