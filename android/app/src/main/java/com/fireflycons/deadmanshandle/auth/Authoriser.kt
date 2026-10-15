package com.fireflycons.deadmanshandle.auth

import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import javax.crypto.Cipher

/**
 * Authorises an [com.fireflycons.deadmanshandle.config.ApiKeyKeystore] cipher
 * with the user's fingerprint (or other strong biometric) or device
 * credential.
 *
 * Create it in onCreate, as BiometricPrompt requires. If the activity is
 * recreated while a prompt is open, the result is dropped and the user
 * starts again.
 */
class Authoriser(activity: FragmentActivity) {
    private var onAuthorised: ((Cipher) -> Unit)? = null
    private var onError: ((CharSequence) -> Unit)? = null

    private val prompt = BiometricPrompt(
        activity,
        ContextCompat.getMainExecutor(activity),
        object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                val handler = onAuthorised
                clear()
                handler?.invoke(checkNotNull(result.cryptoObject?.cipher))
            }

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                val handler = onError
                clear()
                // Cancelling is not an error worth showing
                if (errorCode !in CANCELLED) handler?.invoke(errString)
            }

            // A finger that doesn't match is retried within the prompt
        },
    )

    /**
     * Shows a prompt titled [title]; once the user authenticates, calls
     * [onAuthorised] with [cipher] ready to use. A failure other than the
     * user cancelling calls [onError] with the system's message.
     */
    fun authorise(
        cipher: Cipher,
        title: String,
        onAuthorised: (Cipher) -> Unit,
        onError: (CharSequence) -> Unit,
    ) {
        this.onAuthorised = onAuthorised
        this.onError = onError
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle(title)
            .setAllowedAuthenticators(BIOMETRIC_STRONG or DEVICE_CREDENTIAL)
            .build()
        prompt.authenticate(info, BiometricPrompt.CryptoObject(cipher))
    }

    private fun clear() {
        onAuthorised = null
        onError = null
    }

    private companion object {
        val CANCELLED = setOf(
            BiometricPrompt.ERROR_USER_CANCELED,
            BiometricPrompt.ERROR_NEGATIVE_BUTTON,
            BiometricPrompt.ERROR_CANCELED,
        )
    }
}
