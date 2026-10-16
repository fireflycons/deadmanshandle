package com.fireflycons.deadmanshandle.config

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertThrows
import org.junit.Test

class CheckInConfigTest {
    @Test
    fun validEndpoints() {
        listOf(
            "https://abc123.execute-api.eu-west-1.amazonaws.com",
            "https://abc123.execute-api.eu-west-1.amazonaws.com/",
            "https://example.com:8443/prefix",
        ).forEach { endpoint ->
            assertEquals(endpoint, true, CheckInConfig.isValidEndpoint(endpoint))
        }
    }

    @Test
    fun invalidEndpoints() {
        listOf(
            "",
            "http://example.com",
            "https://",
            "https:///path",
            "example.com",
            "https://exa mple.com",
        ).forEach { endpoint ->
            assertEquals(endpoint, false, CheckInConfig.isValidEndpoint(endpoint))
            assertThrows(endpoint, IllegalArgumentException::class.java) {
                CheckInConfig(endpoint, "key")
            }
        }
    }

    @Test
    fun blankApiKeyIsRejected() {
        listOf("", " ", "\t\n").forEach { key ->
            assertThrows(IllegalArgumentException::class.java) {
                CheckInConfig("https://example.com", key)
            }
        }
    }

    @Test
    fun toStringHidesApiKey() {
        val text = CheckInConfig("https://example.com", "secret-key").toString()
        assertFalse(text, text.contains("secret-key"))
    }
}
