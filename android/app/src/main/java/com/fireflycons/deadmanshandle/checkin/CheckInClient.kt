package com.fireflycons.deadmanshandle.checkin

import com.fireflycons.deadmanshandle.config.CheckInConfig
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** Checks in to the API. [checkIn] blocks, so call it off the main thread. */
class CheckInClient {
    /** Checks in, registering [pushToken] for push notifications if given */
    fun checkIn(config: CheckInConfig, pushToken: String? = null): CheckInResult {
        val connection = try {
            URL(checkInUrl(config.endpoint)).openConnection() as HttpURLConnection
        } catch (e: IOException) {
            return CheckInResult.Unreachable(e.toString())
        }
        return try {
            connection.requestMethod = "POST"
            connection.setRequestProperty("x-api-key", config.apiKey)
            connection.connectTimeout = TIMEOUT_MILLIS
            connection.readTimeout = TIMEOUT_MILLIS
            val request = checkInBody(pushToken)
            if (request.isNotEmpty()) connection.setRequestProperty("Content-Type", "application/json")
            connection.doOutput = true
            connection.setFixedLengthStreamingMode(request.size)
            connection.outputStream.use { it.write(request) }

            val status = connection.responseCode
            val stream = if (status < 400) connection.inputStream else connection.errorStream
            val body = stream?.bufferedReader()?.use { it.readText() }
            CheckInResult.from(status, body)
        } catch (e: IOException) {
            CheckInResult.Unreachable(e.toString())
        } finally {
            connection.disconnect()
        }
    }

    private companion object {
        const val TIMEOUT_MILLIS = 15_000
    }
}

/**
 * The check-in request's body: `{"pushToken":"…"}`, as
 * internal/handlers.HTTPHandler reads it, or empty without a token
 */
fun checkInBody(pushToken: String?): ByteArray =
    if (pushToken == null) ByteArray(0) else JSONObject().put("pushToken", pushToken).toString().toByteArray()
