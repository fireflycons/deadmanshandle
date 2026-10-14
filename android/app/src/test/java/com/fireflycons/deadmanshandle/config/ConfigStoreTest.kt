package com.fireflycons.deadmanshandle.config

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertThrows
import org.junit.Assert.assertTrue
import org.junit.Test
import javax.crypto.AEADBadTagException
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

// A software AES-GCM key stands in for the Keystore key, which the JVM lacks
class ConfigStoreTest {
    private val key = newKey()
    private val config = CheckInConfig("https://example.com", "secret-key")

    @Test
    fun emptyStoreIsNotConfigured() {
        val store = ConfigStore(FakeSharedPreferences())
        assertFalse(store.isConfigured)
        assertNull(store.endpoint)
        assertNull(store.sealedApiKey)
        assertNull(store.load(encryptCipher(key)))
    }

    @Test
    fun saveAndLoad() {
        val prefs = FakeSharedPreferences()
        ConfigStore(prefs).save(config, encryptCipher(key))

        // A new store over the same preferences, as after a restart
        val store = ConfigStore(prefs)
        assertTrue(store.isConfigured)
        assertEquals(config.endpoint, store.endpoint)
        val sealed = store.sealedApiKey!!
        assertEquals(config, store.load(decryptCipher(key, sealed)))
    }

    @Test
    fun apiKeyIsNotStoredInClear() {
        val prefs = FakeSharedPreferences()
        ConfigStore(prefs).save(config, encryptCipher(key))
        prefs.all.values.forEach { value ->
            assertFalse(value.toString(), value.toString().contains("secret-key"))
        }
    }

    @Test
    fun wrongKeyCannotOpen() {
        val store = ConfigStore(FakeSharedPreferences())
        store.save(config, encryptCipher(key))
        val sealed = store.sealedApiKey!!
        assertThrows(AEADBadTagException::class.java) {
            store.load(decryptCipher(newKey(), sealed))
        }
    }

    @Test
    fun partialConfigIsNotConfigured() {
        val prefs = FakeSharedPreferences()
        ConfigStore(prefs).save(config, encryptCipher(key))
        prefs.edit().remove("apiKeyCiphertext").apply()

        val store = ConfigStore(prefs)
        assertFalse(store.isConfigured)
        assertNull(store.endpoint)
        assertNull(store.sealedApiKey)
    }

    @Test
    fun clear() {
        val store = ConfigStore(FakeSharedPreferences())
        store.save(config, encryptCipher(key))
        store.clear()
        assertFalse(store.isConfigured)
    }

    private fun newKey(): SecretKey = KeyGenerator.getInstance("AES").apply { init(256) }.generateKey()

    private fun encryptCipher(key: SecretKey): Cipher =
        Cipher.getInstance("AES/GCM/NoPadding").apply { init(Cipher.ENCRYPT_MODE, key) }

    private fun decryptCipher(key: SecretKey, sealed: SealedApiKey): Cipher =
        Cipher.getInstance("AES/GCM/NoPadding").apply {
            init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, sealed.iv))
        }
}
