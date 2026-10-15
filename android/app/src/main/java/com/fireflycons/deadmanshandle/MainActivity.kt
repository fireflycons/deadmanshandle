package com.fireflycons.deadmanshandle

import android.app.KeyguardManager
import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.consumeWindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.fragment.app.FragmentActivity
import com.fireflycons.deadmanshandle.auth.Authoriser
import com.fireflycons.deadmanshandle.config.ApiKeyKeystore
import com.fireflycons.deadmanshandle.config.CheckInConfig
import com.fireflycons.deadmanshandle.config.ConfigStore
import com.fireflycons.deadmanshandle.ui.ConfigEntryScreen
import com.fireflycons.deadmanshandle.ui.ConfiguredScreen
import java.security.GeneralSecurityException

// A FragmentActivity, as BiometricPrompt needs
class MainActivity : FragmentActivity() {
    private lateinit var store: ConfigStore
    private lateinit var keystore: ApiKeyKeystore
    private lateinit var authoriser: Authoriser

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        store = ConfigStore(getSharedPreferences(PREFS, MODE_PRIVATE))
        keystore = ApiKeyKeystore()
        authoriser = Authoriser(this)
        val invalidated = clearIfKeyInvalidated()

        setContent {
            MaterialTheme {
                var endpoint by remember { mutableStateOf(store.endpoint) }
                var editing by rememberSaveable { mutableStateOf(false) }
                var message by remember {
                    mutableStateOf(if (invalidated) getString(R.string.config_invalidated) else null)
                }

                Scaffold { padding ->
                    val modifier = Modifier.fillMaxSize().padding(padding).consumeWindowInsets(padding).imePadding()
                    val saved = endpoint
                    if (saved == null || editing) {
                        ConfigEntryScreen(
                            initialEndpoint = saved.orEmpty(),
                            message = message,
                            onSave = { config ->
                                message = null
                                save(
                                    config,
                                    onSaved = {
                                        endpoint = config.endpoint
                                        editing = false
                                    },
                                    onError = { message = it },
                                )
                            },
                            onCancel = if (saved != null) {
                                {
                                    editing = false
                                    message = null
                                }
                            } else null,
                            modifier = modifier,
                        )
                    } else {
                        ConfiguredScreen(saved, onChange = { editing = true }, modifier = modifier)
                    }
                }
            }
        }
    }

    /**
     * Removing the screen lock invalidates the Keystore key, so the saved
     * API key can't be opened. The config is then
     * cleared, to be entered again. Returns whether it was.
     */
    private fun clearIfKeyInvalidated(): Boolean {
        val sealed = store.sealedApiKey ?: return false
        if (keystore.decryptCipher(sealed) != null) return false
        store.clear()
        return true
    }

    /** Seals the API key, once the user authorises it, and saves [config] */
    private fun save(config: CheckInConfig, onSaved: () -> Unit, onError: (String) -> Unit) {
        // Without a screen lock the Keystore key can't be created
        if (!getSystemService(KeyguardManager::class.java).isDeviceSecure) {
            onError(getString(R.string.no_screen_lock))
            return
        }
        val cipher = try {
            keystore.encryptCipher()
        } catch (e: GeneralSecurityException) {
            onError(getString(R.string.save_failed, e.toString()))
            return
        }
        authoriser.authorise(
            cipher,
            getString(R.string.save_prompt_title),
            onAuthorised = { authorised ->
                try {
                    store.save(config, authorised)
                    onSaved()
                } catch (e: GeneralSecurityException) {
                    onError(getString(R.string.save_failed, e.toString()))
                }
            },
            onError = { onError(getString(R.string.save_failed, it)) },
        )
    }

    private companion object {
        const val PREFS = "config"
    }
}
