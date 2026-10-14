package com.fireflycons.deadmanshandle.config

import android.content.SharedPreferences
import androidx.core.content.edit
import java.util.Base64
import javax.crypto.Cipher

/**
 * The config, kept in app-private preferences: the endpoint in clear and the
 * API key sealed with an [ApiKeyKeystore] cipher. The app's data is excluded
 * from backups and device transfers, and Android deletes it on uninstall.
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
        }
    }

    /**
     * The config, opening the API key with [cipher] from
     * [ApiKeyKeystore.decryptCipher], or null if not configured
     */
    fun load(cipher: Cipher): CheckInConfig? {
        val endpoint = endpoint ?: return null
        val sealed = sealedApiKey ?: return null
        return CheckInConfig(endpoint, sealed.open(cipher))
    }

    fun clear() = prefs.edit { clear() }

    private companion object {
        const val ENDPOINT = "endpoint"
        const val IV = "apiKeyIv"
        const val CIPHERTEXT = "apiKeyCiphertext"
    }
}
