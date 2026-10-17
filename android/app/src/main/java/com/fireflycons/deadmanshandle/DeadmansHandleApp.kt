package com.fireflycons.deadmanshandle

import android.app.Application
import com.fireflycons.deadmanshandle.config.ConfigStore
import com.fireflycons.deadmanshandle.push.Firebase
import com.fireflycons.deadmanshandle.push.Notifications

/**
 * Starts Firebase from the saved push settings whenever the process starts,
 * including when a push wakes it, before the messaging service runs
 */
class DeadmansHandleApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Notifications.createChannels(this)
        Firebase.configure(this, ConfigStore(getSharedPreferences(ConfigStore.PREFS, MODE_PRIVATE)).pushOptions)
    }
}
