package com.fireflycons.deadmanshandle.config

import javax.crypto.Cipher

/** An API key encrypted with AES-GCM: the IV, and the ciphertext with its tag */
class SealedApiKey(val iv: ByteArray, val ciphertext: ByteArray) {
    /** Decrypts the key with [cipher], initialised for decryption with [iv] */
    fun open(cipher: Cipher) = String(cipher.doFinal(ciphertext), Charsets.UTF_8)

    companion object {
        /** Encrypts [apiKey] with [cipher], initialised for encryption */
        fun seal(cipher: Cipher, apiKey: String): SealedApiKey {
            val ciphertext = cipher.doFinal(apiKey.toByteArray(Charsets.UTF_8))
            return SealedApiKey(cipher.iv, ciphertext)
        }
    }
}
