package com.fireflycons.deadmanshandle.config

import java.net.URI
import java.net.URISyntaxException

/**
 * What the app needs to check in: the API endpoint and the API key. The
 * rules match the Go side (internal/qrconfig and Config.Validate).
 */
data class CheckInConfig(val endpoint: String, val apiKey: String) {
    init {
        require(isValidEndpoint(endpoint)) { "The API endpoint is not an https URL" }
        require(isValidApiKey(apiKey)) { "The API key is empty" }
    }

    // The key is secret, so keep it out of logs and crash reports
    override fun toString() = "CheckInConfig(endpoint=$endpoint, apiKey=***)"

    companion object {
        /** Whether [endpoint] is an https URL with a host */
        fun isValidEndpoint(endpoint: String): Boolean = try {
            val uri = URI(endpoint)
            uri.scheme == "https" && !uri.host.isNullOrEmpty()
        } catch (_: URISyntaxException) {
            false
        }

        /** Whether [apiKey] is not blank; it is otherwise used exactly as given */
        fun isValidApiKey(apiKey: String) = apiKey.isNotBlank()
    }
}
