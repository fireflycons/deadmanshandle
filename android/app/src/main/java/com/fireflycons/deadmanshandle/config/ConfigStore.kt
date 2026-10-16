package com.fireflycons.deadmanshandle.config

import android.content.SharedPreferences
import androidx.core.content.edit
import java.util.Base64
import javax.crypto.Cipher

/**
 * The config, kept in app-private preferences: the endpoint and push settings
 * in clear and the API key sealed with an [ApiKeyKeystore] cipher. The app's
 * data is excluded from backups and device transfers, and Android deletes it
 * on uninstall.
 */
class ConfigStore(private val prefs: SharedPreferences) {
    /** The endpoint, readable without authenticating, or null if not configured */
    val endpoint: String?
        get() = if (isConfigured) prefs.getString(ENDPOINT, null) else null

    /** The sealed API key, for [ApiKeyKeystore.decryptCipher], or null if not configured */
    val sealedApiKey: SealedApiKey?
        get() {
            if (!isConfigured) return null
            val decoder = Base64.getDecoder()
            return SealedApiKey(
                decoder.decode(prefs.getString(IV, null)),
                decoder.decode(prefs.getString(CIPHERTEXT, null)),
            )
        }

    /** The push settings, readable without authenticating, or null if push is off or not configured */
    val pushOptions: PushOptions?
        get() {
            if (!isConfigured) return null
            val fields = PUSH_FIELDS.map { prefs.getString(it, null) }
            if (fields.any { it == null }) return null
            val (projectId, appId, apiKey, senderId) = fields.map { it!! }
            return PushOptions(projectId, appId, apiKey, senderId)
        }

    /**
     * Whether a check-in has sent this install's current push token since the
     * config was saved, so the service can push to it
     */
    val isPushRegistered: Boolean
        get() {
            if (pushOptions == null) return false
            val registered = prefs.getString(REGISTERED_TOKEN, null) ?: return false
            return prefs.getString(LATEST_TOKEN, registered) == registered
        }

    val isConfigured: Boolean
        get() = listOf(ENDPOINT, IV, CIPHERTEXT).all(prefs::contains)

    /** Saves [config], sealing the API key with [cipher] from [ApiKeyKeystore.encryptCipher] */
    fun save(config: CheckInConfig, cipher: Cipher) {
        val sealed = SealedApiKey.seal(cipher, config.apiKey)
        val encoder = Base64.getEncoder()
        prefs.edit {
            putString(ENDPOINT, config.endpoint)
            putString(IV, encoder.encodeToString(sealed.iv))
            putString(CIPHERTEXT, encoder.encodeToString(sealed.ciphertext))
            val push = config.push
            if (push == null) {
                PUSH_FIELDS.forEach(::remove)
            } else {
                PUSH_FIELDS.zip(listOf(push.projectId, push.appId, push.apiKey, push.senderId)).forEach { (key, value) ->
                    putString(key, value)
                }
            }
            // A new config may be a new deployment, which needs the token
            remove(REGISTERED_TOKEN)
        }
    }

    /** Records that a check-in has sent [token], this install's push token */
    fun markPushRegistered(token: String) = prefs.edit {
        putString(REGISTERED_TOKEN, token)
        putString(LATEST_TOKEN, token)
    }

    /** Records a new push token, which the service has until a check-in sends it */
    fun noteNewPushToken(token: String) = prefs.edit { putString(LATEST_TOKEN, token) }

    /**
     * The config, opening the API key with [cipher] from
     * [ApiKeyKeystore.decryptCipher], or null if not configured
     */
    fun load(cipher: Cipher): CheckInConfig? {
        val endpoint = endpoint ?: return null
        val sealed = sealedApiKey ?: return null
        return CheckInConfig(endpoint, sealed.open(cipher), pushOptions)
    }

    fun clear() = prefs.edit { clear() }

    companion object {
        /** The name of the preferences file */
        const val PREFS = "config"

        private const val ENDPOINT = "endpoint"
        private const val IV = "apiKeyIv"
        private const val CIPHERTEXT = "apiKeyCiphertext"
        private val PUSH_FIELDS = listOf("pushProjectId", "pushAppId", "pushApiKey", "pushSenderId")
        private const val REGISTERED_TOKEN = "pushRegisteredToken"
        private const val LATEST_TOKEN = "pushLatestToken"
    }
}
