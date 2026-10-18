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
    fun pushOptions() {
        val prefs = FakeSharedPreferences()
        val push = PushOptions("proj", "1:42:android:ab", "AIza", "42")
        ConfigStore(prefs).save(config.copy(push = push), encryptCipher(key))

        // Readable without authenticating, and loaded with the config
        val store = ConfigStore(prefs)
        assertEquals(push, store.pushOptions)
        assertEquals(config.copy(push = push), store.load(decryptCipher(key, store.sealedApiKey!!)))

        // Saving a config without push removes them
        store.save(config, encryptCipher(key))
        assertNull(store.pushOptions)
        assertEquals(config, store.load(decryptCipher(key, store.sealedApiKey!!)))
    }

    @Test
    fun pushRegistration() {
        val store = ConfigStore(FakeSharedPreferences())
        val withPush = config.copy(push = PushOptions("proj", "1:42:android:ab", "AIza", "42"))
        store.save(withPush, encryptCipher(key))
        assertFalse(store.isPushRegistered)

        // A token from before the first check-in
        store.noteNewPushToken("t1")
        assertFalse(store.isPushRegistered)

        store.markPushRegistered("t1")
        assertTrue(store.isPushRegistered)
        // Reported again after the check-in sent it
        store.noteNewPushToken("t1")
        assertTrue(store.isPushRegistered)

        // A new token needs a check-in to send it
        store.noteNewPushToken("t2")
        assertFalse(store.isPushRegistered)
        store.markPushRegistered("t2")
        assertTrue(store.isPushRegistered)

        // A new config needs a new registration
        store.save(withPush, encryptCipher(key))
        assertFalse(store.isPushRegistered)

        // Without push there is nothing to register
        store.save(config, encryptCipher(key))
        store.markPushRegistered("t3")
        assertFalse(store.isPushRegistered)
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
