package com.fireflycons.deadmanshandle.config

import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyPermanentlyInvalidatedException
import android.security.keystore.KeyProperties
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * The Android Keystore key that encrypts the API key. Every use needs the
 * user's fingerprint (or other strong biometric) or device credential, so a
 * cipher from here must be authorised with a BiometricPrompt CryptoObject
 * before it can encrypt or decrypt.
 *
 * Creating the key needs a secure lock screen (KeyguardManager.isDeviceSecure);
 * without one, [encryptCipher] throws. Android invalidates the key if the
 * screen lock is removed, and deletes it when the app is uninstalled.
 * Enrolling a fingerprint does not invalidate it: that only applies to keys
 * usable with a biometric alone, and this one also accepts the device
 * credential, which enrolling needs anyway.
 */
class ApiKeyKeystore {
    private val keyStore = KeyStore.getInstance(PROVIDER).apply { load(null) }

    /** A cipher to encrypt an API key, creating the key if there is none or it was invalidated */
    fun encryptCipher(): Cipher {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        try {
            cipher.init(Cipher.ENCRYPT_MODE, existingKey() ?: createKey())
        } catch (_: KeyPermanentlyInvalidatedException) {
            keyStore.deleteEntry(ALIAS)
            cipher.init(Cipher.ENCRYPT_MODE, createKey())
        }
        return cipher
    }

    /**
     * A cipher to decrypt [sealed], or null if the key is gone or was
     * invalidated, in which case the config must be entered again
     */
    fun decryptCipher(sealed: SealedApiKey): Cipher? {
        val key = existingKey() ?: return null
        val cipher = Cipher.getInstance(TRANSFORMATION)
        return try {
            cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(TAG_BITS, sealed.iv))
            cipher
        } catch (_: KeyPermanentlyInvalidatedException) {
            keyStore.deleteEntry(ALIAS)
            null
        }
    }

    private fun existingKey() = keyStore.getKey(ALIAS, null) as SecretKey?

    private fun createKey(): SecretKey {
        val spec = KeyGenParameterSpec.Builder(
            ALIAS,
            KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
        )
            .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
            .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
            .setKeySize(256)
            .setUserAuthenticationRequired(true)
            // 0: authorise each use, not a time window
            .setUserAuthenticationParameters(
                0,
                KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL,
            )
            .build()
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, PROVIDER)
            .apply { init(spec) }
            .generateKey()
    }

    private companion object {
        const val PROVIDER = "AndroidKeyStore"
        const val ALIAS = "api-key"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val TAG_BITS = 128
    }
}
