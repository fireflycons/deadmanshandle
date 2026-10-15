package com.fireflycons.deadmanshandle.checkin

import org.junit.Assert.assertEquals
import org.junit.Test
import java.time.Instant
import java.time.ZoneId
import java.util.Locale

class CheckInResultTest {
    @Test
    fun from() {
        listOf(
            // As internal/handlers.HTTPHandler writes them
            Triple(
                200,
                """{"statusCode":200,"message":"Check-in successful","newTimeout":"2026-10-22T07:00:00Z"}""",
                CheckInResult.Success(Instant.parse("2026-10-22T07:00:00Z")),
            ),
            Triple(
                200,
                """{"newTimeout":"2026-10-22T08:00:00+01:00"}""",
                CheckInResult.Success(Instant.parse("2026-10-22T07:00:00Z")),
            ),
            Triple(200, """{"message":"ok"}""", CheckInResult.Failed(200, "The response has no valid newTimeout")),
            Triple(200, """{"newTimeout":"tomorrow"}""", CheckInResult.Failed(200, "The response has no valid newTimeout")),
            Triple(200, "not JSON", CheckInResult.Failed(200, "The response has no valid newTimeout")),
            Triple(401, """{"statusCode":401,"message":"Invalid API key"}""", CheckInResult.Unauthorised),
            // API Gateway's own throttle response
            Triple(429, """{"message":"Too Many Requests"}""", CheckInResult.Throttled),
            Triple(
                500,
                """{"statusCode":500,"message":"Failed to save check-in"}""",
                CheckInResult.Failed(500, "Failed to save check-in"),
            ),
            Triple(404, """{"message":"Not Found"}""", CheckInResult.Failed(404, "Not Found")),
            Triple(502, null, CheckInResult.Failed(502, null)),
            Triple(503, "<html></html>", CheckInResult.Failed(503, null)),
        ).forEach { (status, body, want) ->
            assertEquals("$status $body", want, CheckInResult.from(status, body))
        }
    }

    @Test
    fun url() {
        assertEquals("https://example.com/checkin", checkInUrl("https://example.com"))
        assertEquals("https://example.com/checkin", checkInUrl("https://example.com/"))
        assertEquals("https://example.com/prefix/checkin", checkInUrl("https://example.com/prefix"))
    }

    @Test
    fun deadline() {
        val deadline = Instant.parse("2026-10-22T07:00:00Z")
        assertEquals(
            "Thursday 22 October 2026 at 07:00 UTC",
            formatDeadline(deadline, ZoneId.of("UTC"), Locale.UK),
        )
        assertEquals(
            "Thursday 22 October 2026 at 08:00 BST",
            formatDeadline(deadline, ZoneId.of("Europe/London"), Locale.UK),
        )
    }
}
