package com.fireflycons.deadmanshandle.checkin

import org.json.JSONException
import org.json.JSONObject
import java.time.Instant
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeParseException
import java.util.Locale

/** The outcome of `POST <endpoint>/checkin` */
sealed interface CheckInResult {
    data class Success(val newTimeout: Instant) : CheckInResult

    /** 401: the API key is missing or wrong */
    data object Unauthorised : CheckInResult

    /** 429: API Gateway's throttle, about one request a minute */
    data object Throttled : CheckInResult

    /** Any other response, with the API's message if it gave one */
    data class Failed(val status: Int, val message: String?) : CheckInResult

    /** No response: no network, DNS failure, timeout and so on */
    data class Unreachable(val reason: String) : CheckInResult

    companion object {
        /** The result of a response with [status] and [body] */
        fun from(status: Int, body: String?): CheckInResult {
            val json = try {
                body?.let(::JSONObject)
            } catch (_: JSONException) {
                null
            }
            return when (status) {
                200 -> json?.optString("newTimeout")?.let(::parseTimeout)?.let(::Success)
                    ?: Failed(status, "The response has no valid newTimeout")
                401 -> Unauthorised
                429 -> Throttled
                else -> Failed(status, json?.optString("message")?.ifEmpty { null })
            }
        }

        private fun parseTimeout(text: String): Instant? = try {
            OffsetDateTime.parse(text).toInstant()
        } catch (_: DateTimeParseException) {
            null
        }
    }
}

/** The check-in URL for [endpoint], which may end with a slash */
fun checkInUrl(endpoint: String) = endpoint.trimEnd('/') + "/checkin"

/** A deadline as the emails show it, `Monday 2 January 2006 at 15:04 MST`, in [zone] */
fun formatDeadline(deadline: Instant, zone: ZoneId, locale: Locale): String =
    DateTimeFormatter.ofPattern("EEEE d MMMM yyyy 'at' HH:mm z", locale).format(deadline.atZone(zone))
