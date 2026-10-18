package com.fireflycons.deadmanshandle.checkin

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Test

class CheckInBodyTest {
    @Test
    fun noTokenIsAnEmptyBody() {
        assertEquals(0, checkInBody(null).size)
    }

    @Test
    fun token() {
        // As internal/handlers.HTTPHandler reads it
        val body = JSONObject(String(checkInBody("fcm-token:abc_123")))
        assertEquals("fcm-token:abc_123", body.getString("pushToken"))
        assertEquals(1, body.length())
    }
}
