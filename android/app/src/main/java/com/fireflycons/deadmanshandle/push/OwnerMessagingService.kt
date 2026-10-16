package com.fireflycons.deadmanshandle.push

import com.fireflycons.deadmanshandle.config.ConfigStore
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * Receives the service's pushes, even while the app is closed: the
 * Application has started Firebase from the saved config by then
 */
class OwnerMessagingService : FirebaseMessagingService() {
    private val store by lazy { ConfigStore(getSharedPreferences(ConfigStore.PREFS, MODE_PRIVATE)) }

    override fun onMessageReceived(message: RemoteMessage) {
        // Push was turned off since the token was registered
        if (store.pushOptions == null) return
        val push = PushMessage.parse(message.data) ?: return
        Notifications.show(this, push)
    }

    /**
     * Only a check-in can send a new token, as that is when the API key is
     * open, so the app shows that one is needed (see [ConfigStore.isPushRegistered])
     */
    @Suppress("OVERRIDE_DEPRECATION") // see Firebase
    override fun onNewToken(token: String) {
        store.noteNewPushToken(token)
    }
}
