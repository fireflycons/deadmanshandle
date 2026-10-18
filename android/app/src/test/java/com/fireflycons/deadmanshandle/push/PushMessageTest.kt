package com.fireflycons.deadmanshandle.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PushMessageTest {
    @Test
    fun parse() {
        // As internal/adapters/sns_push_sender.go sends it
        val data = mapOf("v" to "1", "kind" to "triggered", "title" to "Deadman's Handle Triggered", "body" to "Sent to:\n  a@example.com")
        assertEquals(
            PushMessage("triggered", "Deadman's Handle Triggered", "Sent to:\n  a@example.com"),
            PushMessage.parse(data),
        )
        assertNull(PushMessage.parse(mapOf("kind" to "warning", "body" to "b")))
        assertNull(PushMessage.parse(mapOf("kind" to "warning", "title" to " ", "body" to "b")))
        assertNull(PushMessage.parse(mapOf("kind" to "warning", "title" to "t")))
        assertEquals(PushMessage("", "t", "b"), PushMessage.parse(mapOf("title" to "t", "body" to "b")))
    }

    @Test
    fun channel() {
        assertEquals(PushMessage.Channel.REMINDERS, PushMessage("warning", "t", "b").channel)
        listOf("triggered", "document-missing", "delivery-blocked", "document-changed", "delivery-held", "alarm", "").forEach {
            assertEquals(it, PushMessage.Channel.ALERTS, PushMessage(it, "t", "b").channel)
        }
    }

    @Test
    fun notificationId() {
        // A notice replaces the last of its kind
        assertEquals(PushMessage("warning", "t1", "b1").notificationId, PushMessage("warning", "t2", "b2").notificationId)
        assertNotEquals(PushMessage("warning", "t", "b").notificationId, PushMessage("triggered", "t", "b").notificationId)
        // Different alarms are kept apart
        assertNotEquals(PushMessage("alarm", "a1", "b").notificationId, PushMessage("alarm", "a2", "b").notificationId)
    }
}
