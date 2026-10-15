package com.fireflycons.deadmanshandle.config

import org.junit.Assert.assertEquals
import org.junit.Test

class QrPayloadTest {
    @Test
    fun validPayload() {
        // As internal/qrconfig.Encode writes it
        val text = """{"v":1,"endpoint":"https://abc123.execute-api.eu-west-1.amazonaws.com","apiKey":"k3y"}"""
        assertEquals(
            QrPayload.Valid(CheckInConfig("https://abc123.execute-api.eu-west-1.amazonaws.com", "k3y")),
            QrPayload.parse(text),
        )
    }

    @Test
    fun parse() {
        listOf(
            "" to QrPayload.NotAConfig,
            "https://example.com" to QrPayload.NotAConfig,
            "[1]" to QrPayload.NotAConfig,
            """{"endpoint":"https://example.com","apiKey":"k"}""" to QrPayload.NotAConfig,
            """{"v":"1","endpoint":"https://example.com","apiKey":"k"}""" to QrPayload.NotAConfig,
            """{"v":2,"endpoint":"https://example.com","apiKey":"k"}""" to QrPayload.UnknownVersion(2),
            """{"v":1,"apiKey":"k"}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com"}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"http://example.com","apiKey":"k"}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com","apiKey":" "}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com","apiKey":5}""" to QrPayload.Invalid,
        ).forEach { (text, want) ->
            assertEquals(text, want, QrPayload.parse(text))
        }
    }
}
