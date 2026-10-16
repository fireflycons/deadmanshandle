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
    fun validPayloadWithPush() {
        // As internal/qrconfig.Encode writes it with -firebase
        val text = """{"v":1,"endpoint":"https://example.com","apiKey":"k3y",""" +
            """"push":{"projectId":"proj","appId":"1:42:android:ab","apiKey":"AIza","senderId":"42"}}"""
        assertEquals(
            QrPayload.Valid(CheckInConfig("https://example.com", "k3y", PushOptions("proj", "1:42:android:ab", "AIza", "42"))),
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
            """{"v":1,"endpoint":"https://example.com","apiKey":"k","push":5}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com","apiKey":"k","push":null}""" to QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com","apiKey":"k","push":{"projectId":"p","appId":"a","apiKey":"k"}}""" to
                QrPayload.Invalid,
            """{"v":1,"endpoint":"https://example.com","apiKey":"k","push":{"projectId":"p","appId":"a","apiKey":"k","senderId":" "}}""" to
                QrPayload.Invalid,
        ).forEach { (text, want) ->
            assertEquals(text, want, QrPayload.parse(text))
        }
    }
}
