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
import com.google.mlkit.common.MlKitException
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.codescanner.GmsBarcodeScannerOptions
import com.google.mlkit.vision.codescanner.GmsBarcodeScanning
import com.fireflycons.deadmanshandle.auth.Authoriser
import com.fireflycons.deadmanshandle.config.ApiKeyKeystore
import com.fireflycons.deadmanshandle.config.CheckInConfig
import com.fireflycons.deadmanshandle.config.ConfigStore
import com.fireflycons.deadmanshandle.config.QrPayload
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
                            onScan = { onScanned ->
                                message = null
                                scan(onScanned, onError = { message = it })
                            },
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

    /**
     * Scans a setup QR code with Google's code scanner, which runs in Play
     * services and needs no camera permission. Cancelling calls neither
     * callback.
     */
    private fun scan(onScanned: (CheckInConfig) -> Unit, onError: (String) -> Unit) {
        val options = GmsBarcodeScannerOptions.Builder()
            .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
            .build()
        // Not activity-scoped listeners: those are removed when the scanner's
        // own activity stops this one
        GmsBarcodeScanning.getClient(this, options).startScan()
            .addOnSuccessListener { barcode ->
                when (val payload = QrPayload.parse(barcode.rawValue.orEmpty())) {
                    is QrPayload.Valid -> onScanned(payload.config)
                    QrPayload.NotAConfig -> onError(getString(R.string.qr_not_config))
                    is QrPayload.UnknownVersion -> onError(getString(R.string.qr_unknown_version, payload.version))
                    QrPayload.Invalid -> onError(getString(R.string.qr_invalid))
                }
            }
            .addOnFailureListener {
                when ((it as? MlKitException)?.errorCode) {
                    MlKitException.CODE_SCANNER_CANCELLED -> {}
                    // The scanner is a Play services module, downloaded on first use
                    MlKitException.CODE_SCANNER_UNAVAILABLE -> onError(getString(R.string.scanner_downloading))
                    else -> onError(getString(R.string.scan_failed, it.toString()))
                }
            }
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
