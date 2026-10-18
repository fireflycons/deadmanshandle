package com.fireflycons.deadmanshandle.config

import org.json.JSONException
import org.json.JSONObject

/**
 * The text of a setup QR code, as made by the qrconfig CLI
 * (internal/qrconfig): `{"v":1,"endpoint":"https://…","apiKey":"…"}`, with
 * an optional "push" object holding the [PushOptions].
 */
sealed interface QrPayload {
    data class Valid(val config: CheckInConfig) : QrPayload

    /** Not JSON with a numeric "v": some other QR code */
    data object NotAConfig : QrPayload

    /** A version this app doesn't know, from a newer qrconfig */
    data class UnknownVersion(val version: Int) : QrPayload

    /** A known version, but the endpoint, API key or push settings are missing or invalid */
    data object Invalid : QrPayload

    companion object {
        const val VERSION = 1

        fun parse(text: String): QrPayload {
            val json = try {
                JSONObject(text)
            } catch (_: JSONException) {
                return NotAConfig
            }
            val version = json.opt("v") as? Int ?: return NotAConfig
            if (version != VERSION) return UnknownVersion(version)
            val endpoint = json.opt("endpoint") as? String
            val apiKey = json.opt("apiKey") as? String
            if (endpoint == null || apiKey == null ||
                !CheckInConfig.isValidEndpoint(endpoint) || !CheckInConfig.isValidApiKey(apiKey)
            ) {
                return Invalid
            }
            val push = when (val value = json.opt("push")) {
                null -> null
                is JSONObject -> PushOptions.fromJson(value) ?: return Invalid
                else -> return Invalid
            }
            return Valid(CheckInConfig(endpoint, apiKey, push))
        }
    }
}
