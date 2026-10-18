package com.fireflycons.deadmanshandle.config

import org.json.JSONObject

/**
 * The Firebase project's client settings, from the setup QR code (qrconfig
 * -firebase), with which the app registers for push notifications. Each
 * deployment has its own project. They identify it but are not secrets.
 */
data class PushOptions(val projectId: String, val appId: String, val apiKey: String, val senderId: String) {
    init {
        require(isValid(projectId, appId, apiKey, senderId)) { "The push settings are incomplete" }
    }

    companion object {
        private fun isValid(vararg fields: String?) = fields.all { !it.isNullOrBlank() }

        /** The payload's "push" object, as internal/qrconfig writes it, or null if a field is missing */
        fun fromJson(json: JSONObject): PushOptions? {
            val fields = listOf("projectId", "appId", "apiKey", "senderId").map { json.opt(it) as? String }
            if (!isValid(*fields.toTypedArray())) return null
            val (projectId, appId, apiKey, senderId) = fields.map { it!! }
            return PushOptions(projectId, appId, apiKey, senderId)
        }
    }
}
